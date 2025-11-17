// services/transaction-ledger/internal/service/reconciliation.go
//Recconciliation service

package service

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"transaction-ledger/internal/models"
	"transaction-ledger/internal/repository"
)

// ReconciliationService handles financial reconciliation
type ReconciliationService struct {
	repo   *repository.LedgerRepository
	logger *zap.Logger
}

// NewReconciliationService creates a new reconciliation service
func NewReconciliationService(repo *repository.LedgerRepository, logger *zap.Logger) *ReconciliationService {
	return &ReconciliationService{
		repo:   repo,
		logger: logger,
	}
}

// ReconcileDaily performs daily reconciliation
func (s *ReconciliationService) ReconcileDaily(ctx context.Context, date time.Time) (*models.ReconciliationReport, error) {
	startOfDay := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, date.Location())
	endOfDay := startOfDay.Add(24 * time.Hour)

	return s.ReconcilePeriod(ctx, startOfDay, endOfDay)
}

// ReconcilePeriod reconciles transactions for a specific period
func (s *ReconciliationService) ReconcilePeriod(ctx context.Context, startDate, endDate time.Time) (*models.ReconciliationReport, error) {
	s.logger.Info("starting reconciliation",
		zap.Time("start_date", startDate),
		zap.Time("end_date", endDate))

	report := &models.ReconciliationReport{
		ID:           uuid.New().String(),
		StartDate:    startDate,
		EndDate:      endDate,
		CreatedAt:    time.Now(),
		IsBalanced:   true,
		Discrepancies: []string{},
	}

	// Get all transactions in the period
	transactions, err := s.repo.GetTransactionsByDateRange(ctx, startDate, endDate)
	if err != nil {
		return nil, fmt.Errorf("failed to get transactions: %w", err)
	}

	report.TotalTransactions = len(transactions)
	s.logger.Info("transactions found", zap.Int("count", len(transactions)))

	var totalDebits, totalCredits float64
	var unbalancedTransactions []string

	// Check each transaction
	for _, txn := range transactions {
		entries, err := s.repo.GetEntriesByTransaction(ctx, txn.ID)
		if err != nil {
			s.logger.Error("failed to get entries", zap.String("txn_id", txn.ID), zap.Error(err))
			continue
		}

		// Calculate debits and credits for this transaction
		var txnDebits, txnCredits float64
		for _, entry := range entries {
			if entry.Type == models.EntryTypeDebit {
				txnDebits += entry.Amount
				totalDebits += entry.Amount
			} else {
				txnCredits += entry.Amount
				totalCredits += entry.Amount
			}
		}

		// Check if transaction is balanced
		if !isBalanced(txnDebits, txnCredits) {
			discrepancy := fmt.Sprintf("Transaction %s: debits=%.2f, credits=%.2f (diff=%.2f)",
				txn.ID, txnDebits, txnCredits, txnDebits-txnCredits)
			report.Discrepancies = append(report.Discrepancies, discrepancy)
			unbalancedTransactions = append(unbalancedTransactions, txn.ID)
			report.IsBalanced = false
		}
	}

	report.TotalDebits = totalDebits
	report.TotalCredits = totalCredits

	// Overall balance check
	if !isBalanced(totalDebits, totalCredits) {
		report.IsBalanced = false
		report.Discrepancies = append(report.Discrepancies,
			fmt.Sprintf("Overall imbalance: debits=%.2f, credits=%.2f (diff=%.2f)",
				totalDebits, totalCredits, totalDebits-totalCredits))
	}

	// Save report
	if err := s.repo.SaveReconciliationReport(ctx, report); err != nil {
		s.logger.Error("failed to save reconciliation report", zap.Error(err))
	}

	// Log results
	if report.IsBalanced {
		s.logger.Info("reconciliation complete - BALANCED",
			zap.Int("transactions", report.TotalTransactions),
			zap.Float64("total_debits", report.TotalDebits),
			zap.Float64("total_credits", report.TotalCredits))
	} else {
		s.logger.Warn("reconciliation complete - UNBALANCED",
			zap.Int("transactions", report.TotalTransactions),
			zap.Int("discrepancies", len(report.Discrepancies)),
			zap.Strings("unbalanced_txns", unbalancedTransactions))
	}

	return report, nil
}

// ReconcileAccount reconciles a specific account
func (s *ReconciliationService) ReconcileAccount(ctx context.Context, accountID string, startDate, endDate time.Time) (*models.AccountReconciliation, error) {
	entries, err := s.repo.GetEntriesByAccount(ctx, accountID)
	if err != nil {
		return nil, err
	}

	reconciliation := &models.AccountReconciliation{
		AccountID:   accountID,
		StartDate:   startDate,
		EndDate:     endDate,
		OpeningBalance: 0, // Get from previous period
		ClosingBalance: 0,
		CreatedAt:   time.Now(),
	}

	var totalDebits, totalCredits float64
	var periodDebits, periodCredits float64

	for _, entry := range entries {
		if entry.Type == models.EntryTypeDebit {
			totalDebits += entry.Amount
			if entry.CreatedAt.After(startDate) && entry.CreatedAt.Before(endDate) {
				periodDebits += entry.Amount
			}
		} else {
			totalCredits += entry.Amount
			if entry.CreatedAt.After(startDate) && entry.CreatedAt.Before(endDate) {
				periodCredits += entry.Amount
			}
		}
	}

	reconciliation.TotalDebits = periodDebits
	reconciliation.TotalCredits = periodCredits
	reconciliation.ClosingBalance = totalDebits - totalCredits

	return reconciliation, nil
}

// FindDiscrepancies finds all discrepancies in the ledger
func (s *ReconciliationService) FindDiscrepancies(ctx context.Context) ([]models.Discrepancy, error) {
	var discrepancies []models.Discrepancy

	// Get all transactions
	transactions, err := s.repo.GetTransactionsByDateRange(ctx, 
		time.Now().AddDate(0, -1, 0), // Last month
		time.Now())
	if err != nil {
		return nil, err
	}

	for _, txn := range transactions {
		entries, err := s.repo.GetEntriesByTransaction(ctx, txn.ID)
		if err != nil {
			continue
		}

		var debits, credits float64
		for _, entry := range entries {
			if entry.Type == models.EntryTypeDebit {
				debits += entry.Amount
			} else {
				credits += entry.Amount
			}
		}

		if !isBalanced(debits, credits) {
			discrepancies = append(discrepancies, models.Discrepancy{
				TransactionID: txn.ID,
				Type:          "unbalanced_transaction",
				Description:   fmt.Sprintf("Debits: %.2f, Credits: %.2f", debits, credits),
				Amount:        debits - credits,
				DetectedAt:    time.Now(),
			})
		}
	}

	return discrepancies, nil
}

// AutoCorrectDiscrepancies attempts to automatically fix simple discrepancies
func (s *ReconciliationService) AutoCorrectDiscrepancies(ctx context.Context, discrepancies []models.Discrepancy) error {
	for _, disc := range discrepancies {
		s.logger.Info("attempting to correct discrepancy",
			zap.String("transaction_id", disc.TransactionID),
			zap.String("type", disc.Type))

		// In production, implement correction logic based on discrepancy type
		// For now, just log
		s.logger.Warn("auto-correction not implemented for this discrepancy type",
			zap.String("type", disc.Type))
	}

	return nil
}

// GenerateSettlementReport generates a settlement report for payment processors
func (s *ReconciliationService) GenerateSettlementReport(ctx context.Context, startDate, endDate time.Time, processor string) (*models.SettlementReport, error) {
	report := &models.SettlementReport{
		ID:              uuid.New().String(),
		Processor:       processor,
		StartDate:       startDate,
		EndDate:         endDate,
		CreatedAt:       time.Now(),
	}

	// Get all successful payments in period
	// This would query payments service or database
	report.TotalTransactions = 0
	report.TotalAmount = 0.0
	report.TotalFees = 0.0

	return report, nil
}

// Helper functions

func isBalanced(debits, credits float64) bool {
	// Allow for small floating point differences
	tolerance := 0.01
	diff := debits - credits
	return diff >= -tolerance && diff <= tolerance
}

// Additional models for reconciliation

type AccountReconciliation struct {
	AccountID      string
	StartDate      time.Time
	EndDate        time.Time
	OpeningBalance float64
	ClosingBalance float64
	TotalDebits    float64
	TotalCredits   float64
	CreatedAt      time.Time
}

type Discrepancy struct {
	TransactionID string
	Type          string
	Description   string
	Amount        float64
	DetectedAt    time.Time
}

type SettlementReport struct {
	ID                string
	Processor         string
	StartDate         time.Time
	EndDate           time.Time
	TotalTransactions int
	TotalAmount       float64
	TotalFees         float64
	CreatedAt         time.Time
}