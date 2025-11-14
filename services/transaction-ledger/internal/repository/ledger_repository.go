// services/transaction-ledger/internal/repository/ledger_repository.go
package repository

import (
	"context"
	"database/sql"
	"time"

	"github.com/lib/pq"
	"transaction-ledger/internal/models"
)

type LedgerRepository struct {
	db *sql.DB
}

func NewLedgerRepository(db *sql.DB) *LedgerRepository {
	return &LedgerRepository{db: db}
}

func (r *LedgerRepository) CreateTransaction(ctx context.Context, txn *models.LedgerTransaction, entries []*models.LedgerEntry) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Insert transaction
	txnQuery := `
		INSERT INTO ledger_transactions (id, description, payment_id, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`
	_, err = tx.ExecContext(ctx, txnQuery,
		txn.ID,
		txn.Description,
		txn.PaymentID,
		txn.Status,
		txn.CreatedAt,
		txn.UpdatedAt,
	)
	if err != nil {
		return err
	}

	// Insert entries
	entryQuery := `
		INSERT INTO ledger_entries (id, transaction_id, account_id, type, amount, currency, description, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`
	
	for _, entry := range entries {
		_, err = tx.ExecContext(ctx, entryQuery,
			entry.ID,
			entry.TransactionID,
			entry.AccountID,
			entry.Type,
			entry.Amount,
			entry.Currency,
			entry.Description,
			entry.CreatedAt,
		)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (r *LedgerRepository) UpdateTransactionStatus(ctx context.Context, txnID string, status models.TransactionStatus) error {
	query := `
		UPDATE ledger_transactions
		SET status = $1, updated_at = $2
		WHERE id = $3
	`
	_, err := r.db.ExecContext(ctx, query, status, time.Now(), txnID)
	return err
}

func (r *LedgerRepository) GetEntriesByAccount(ctx context.Context, accountID string) ([]*models.LedgerEntry, error) {
	query := `
		SELECT id, transaction_id, account_id, type, amount, currency, description, created_at
		FROM ledger_entries
		WHERE account_id = $1
		ORDER BY created_at DESC
	`
	
	rows, err := r.db.QueryContext(ctx, query, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []*models.LedgerEntry
	for rows.Next() {
		entry := &models.LedgerEntry{}
		err := rows.Scan(
			&entry.ID,
			&entry.TransactionID,
			&entry.AccountID,
			&entry.Type,
			&entry.Amount,
			&entry.Currency,
			&entry.Description,
			&entry.CreatedAt,
		)
		if err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}

	return entries, nil
}

func (r *LedgerRepository) GetEntriesByTransaction(ctx context.Context, txnID string) ([]*models.LedgerEntry, error) {
	query := `
		SELECT id, transaction_id, account_id, type, amount, currency, description, created_at
		FROM ledger_entries
		WHERE transaction_id = $1
	`
	
	rows, err := r.db.QueryContext(ctx, query, txnID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []*models.LedgerEntry
	for rows.Next() {
		entry := &models.LedgerEntry{}
		err := rows.Scan(
			&entry.ID,
			&entry.TransactionID,
			&entry.AccountID,
			&entry.Type,
			&entry.Amount,
			&entry.Currency,
			&entry.Description,
			&entry.CreatedAt,
		)
		if err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}

	return entries, nil
}

func (r *LedgerRepository) GetTransactionsByDateRange(ctx context.Context, start, end time.Time) ([]*models.LedgerTransaction, error) {
	query := `
		SELECT id, description, payment_id, status, created_at, updated_at
		FROM ledger_transactions
		WHERE created_at BETWEEN $1 AND $2
		ORDER BY created_at ASC
	`
	
	rows, err := r.db.QueryContext(ctx, query, start, end)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var transactions []*models.LedgerTransaction
	for rows.Next() {
		txn := &models.LedgerTransaction{}
		err := rows.Scan(
			&txn.ID,
			&txn.Description,
			&txn.PaymentID,
			&txn.Status,
			&txn.CreatedAt,
			&txn.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		transactions = append(transactions, txn)
	}

	return transactions, nil
}

func (r *LedgerRepository) SaveReconciliationReport(ctx context.Context, report *models.ReconciliationReport) error {
	query := `
		INSERT INTO reconciliation_reports 
		(id, start_date, end_date, total_transactions, total_debits, total_credits, is_balanced, discrepancies, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`
	_, err := r.db.ExecContext(ctx, query,
		report.ID,
		report.StartDate,
		report.EndDate,
		report.TotalTransactions,
		report.TotalDebits,
		report.TotalCredits,
		report.IsBalanced,
		pq.Array(report.Discrepancies),
		report.CreatedAt,
	)
	return err
}