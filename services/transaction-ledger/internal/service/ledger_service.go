// services/transaction-ledger/internal/service/ledger_service.go
package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"transaction-ledger/internal/models"
	"transaction-ledger/internal/repository"
)

type LedgerService struct {
	repo   *repository.LedgerRepository
	logger *zap.Logger
}

func NewLedgerService(repo *repository.LedgerRepository, logger *zap.Logger) *LedgerService {
	return &LedgerService{
		repo:   repo,
		logger: logger,
	}
}

// CreateDoubleEntry creates a double-entry ledger transaction
func (s *LedgerService) CreateDoubleEntry(ctx context.Context, req *models.LedgerEntryRequest) (*models.LedgerTransaction, error) {
	// Validate that debits equal credits
	var totalDebits, totalCredits float64
	for _, entry := range req.Entries {
		if entry.Type == models.EntryTypeDebit {
			totalDebits += entry.Amount
		} else {
			totalCredits += entry.Amount
		}
	}

	if totalDebits != totalCredits {
		return nil, errors.New("debits must equal credits in double-entry bookkeeping")
	}

	// Create transaction
	txnID := uuid.New().String()
	transaction := &models.LedgerTransaction{
		ID:          txnID,
		Description: req.Description,
		PaymentID:   req.PaymentID,
		Status:      models.TxnStatusPending,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	// Create entries
	var entries []*models.LedgerEntry
	for _, entryReq := range req.Entries {
		entry := &models.LedgerEntry{
			ID:            uuid.New().String(),
			TransactionID: txnID,
			AccountID:     entryReq.AccountID,
			Type:          entryReq.Type,
			Amount:        entryReq.Amount,
			Currency:      entryReq.Currency,
			Description:   entryReq.Description,
			CreatedAt:     time.Now(),
		}
		entries = append(entries, entry)
	}

	// Save to database (transactional)
	if err := s.repo.CreateTransaction(ctx, transaction, entries); err != nil {
		return nil, fmt.Errorf("failed to create ledger transaction: %w", err)
	}

	transaction.Entries = entries
	transaction.Status = models.TxnStatusCompleted
	transaction.UpdatedAt = time.Now()

	// Update transaction status
	if err := s.repo.UpdateTransactionStatus(ctx, txnID, models.TxnStatusCompleted); err != nil {
		s.logger.Error("failed to update transaction status", zap.Error(err))
	}

	s.logger.Info("double-entry transaction created",
		zap.String("transaction_id", txnID),
		zap.String("payment_id", req.PaymentID))

	return transaction, nil
}

// RecordPayment records a payment in the ledger with double-entry
func (s *LedgerService) RecordPayment(ctx context.Context, paymentID string, amount float64, currency string) error {
	// Double-entry for payment:
	// Debit: Customer's Account (Asset increases)
	// Credit: Payment Gateway Account (Liability increases)

	req := &models.LedgerEntryRequest{
		Description: fmt.Sprintf("Payment %s", paymentID),
		PaymentID:   paymentID,
		Entries: []models.EntryRequest{
			{
				AccountID:   "customer_receivables",
				Type:        models.EntryTypeDebit,
				Amount:      amount,
				Currency:    currency,
				Description: "Customer payment received",
			},
			{
				AccountID:   "payment_gateway_liability",
				Type:        models.EntryTypeCredit,
				Amount:      amount,
				Currency:    currency,
				Description: "Payment gateway liability",
			},
		},
	}

	_, err := s.CreateDoubleEntry(ctx, req)
	return err
}

// GetBalance calculates the current balance for an account
func (s *LedgerService) GetBalance(ctx context.Context, accountID string) (*models.AccountBalance, error) {
	entries, err := s.repo.GetEntriesByAccount(ctx, accountID)
	if err != nil {
		return nil, err
	}

	balance := &models.AccountBalance{
		AccountID: accountID,
		Currency:  "USD", // Default
		UpdatedAt: time.Now(),
	}

	for _, entry := range entries {
		if entry.Type == models.EntryTypeDebit {
			balance.Balance += entry.Amount
		} else {
			balance.Balance -= entry.Amount
		}
	}

	return balance, nil
}

// Reconcile performs reconciliation for a time period
func (s *LedgerService) Reconcile(ctx context.Context, startDate, endDate time.Time) (*models.ReconciliationReport, error) {
	transactions, err := s.repo.GetTransactionsByDateRange(ctx, startDate, endDate)
	if err != nil {
		return nil, err
	}

	report := &models.ReconciliationReport{
		ID:               uuid.New().String(),
		StartDate:        startDate,
		EndDate:          endDate,
		TotalTransactions: len(transactions),
		CreatedAt:        time.Now(),
	}

	var totalDebits, totalCredits float64
	var discrepancies []string

	for _, txn := range transactions {
		entries, err := s.repo.GetEntriesByTransaction(ctx, txn.ID)
		if err != nil {
			continue
		}

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
		if txnDebits != txnCredits {
			discrepancies = append(discrepancies, 
				fmt.Sprintf("Transaction %s: debits %.2f != credits %.2f", 
					txn.ID, txnDebits, txnCredits))
		}
	}

	report.TotalDebits = totalDebits
	report.TotalCredits = totalCredits
	report.Discrepancies = discrepancies
	report.IsBalanced = len(discrepancies) == 0 && totalDebits == totalCredits

	// Save report
	if err := s.repo.SaveReconciliationReport(ctx, report); err != nil {
		s.logger.Error("failed to save reconciliation report", zap.Error(err))
	}

	return report, nil
}

// GetTransactionHistory gets transaction history
func (s *LedgerService) GetTransactionHistory(ctx context.Context, accountID string, limit int) ([]*models.LedgerEntry, error) {
	return s.repo.GetEntriesByAccount(ctx, accountID)
}