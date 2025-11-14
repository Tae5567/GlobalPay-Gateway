// services/transaction-ledger/internal/models/ledger.go
package models

import "time"

type EntryType string
type TransactionStatus string

const (
	EntryTypeDebit  EntryType = "debit"
	EntryTypeCredit EntryType = "credit"

	TxnStatusPending   TransactionStatus = "pending"
	TxnStatusCompleted TransactionStatus = "completed"
	TxnStatusFailed    TransactionStatus = "failed"
	TxnStatusReversed  TransactionStatus = "reversed"
)

// LedgerTransaction represents a financial transaction
type LedgerTransaction struct {
	ID          string            `json:"id" db:"id"`
	Description string            `json:"description" db:"description"`
	PaymentID   string            `json:"payment_id" db:"payment_id"`
	Status      TransactionStatus `json:"status" db:"status"`
	CreatedAt   time.Time         `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at" db:"updated_at"`
	Entries     []*LedgerEntry    `json:"entries,omitempty"`
}

// LedgerEntry represents a single entry in double-entry bookkeeping
type LedgerEntry struct {
	ID            string    `json:"id" db:"id"`
	TransactionID string    `json:"transaction_id" db:"transaction_id"`
	AccountID     string    `json:"account_id" db:"account_id"`
	Type          EntryType `json:"type" db:"type"`
	Amount        float64   `json:"amount" db:"amount"`
	Currency      string    `json:"currency" db:"currency"`
	Description   string    `json:"description" db:"description"`
	CreatedAt     time.Time `json:"created_at" db:"created_at"`
}

// LedgerEntryRequest for creating ledger entries
type LedgerEntryRequest struct {
	Description string         `json:"description" binding:"required"`
	PaymentID   string         `json:"payment_id"`
	Entries     []EntryRequest `json:"entries" binding:"required,min=2"`
}

type EntryRequest struct {
	AccountID   string    `json:"account_id" binding:"required"`
	Type        EntryType `json:"type" binding:"required,oneof=debit credit"`
	Amount      float64   `json:"amount" binding:"required,gt=0"`
	Currency    string    `json:"currency" binding:"required,len=3"`
	Description string    `json:"description"`
}

// AccountBalance represents account balance
type AccountBalance struct {
	AccountID string    `json:"account_id"`
	Balance   float64   `json:"balance"`
	Currency  string    `json:"currency"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ReconciliationReport for period reconciliation
type ReconciliationReport struct {
	ID                string    `json:"id" db:"id"`
	StartDate         time.Time `json:"start_date" db:"start_date"`
	EndDate           time.Time `json:"end_date" db:"end_date"`
	TotalTransactions int       `json:"total_transactions" db:"total_transactions"`
	TotalDebits       float64   `json:"total_debits" db:"total_debits"`
	TotalCredits      float64   `json:"total_credits" db:"total_credits"`
	IsBalanced        bool      `json:"is_balanced" db:"is_balanced"`
	Discrepancies     []string  `json:"discrepancies" db:"discrepancies"`
	CreatedAt         time.Time `json:"created_at" db:"created_at"`
}

// Database schema
const LedgerSchema = `
CREATE TABLE IF NOT EXISTS ledger_transactions (
    id VARCHAR(36) PRIMARY KEY,
    description TEXT NOT NULL,
    payment_id VARCHAR(36),
    status VARCHAR(20) NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    
    INDEX idx_payment_id (payment_id),
    INDEX idx_status (status),
    INDEX idx_created_at (created_at)
);

CREATE TABLE IF NOT EXISTS ledger_entries (
    id VARCHAR(36) PRIMARY KEY,
    transaction_id VARCHAR(36) NOT NULL,
    account_id VARCHAR(100) NOT NULL,
    type VARCHAR(10) NOT NULL,
    amount DECIMAL(19, 4) NOT NULL,
    currency VARCHAR(3) NOT NULL,
    description TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    
    FOREIGN KEY (transaction_id) REFERENCES ledger_transactions(id),
    INDEX idx_transaction_id (transaction_id),
    INDEX idx_account_id (account_id),
    INDEX idx_created_at (created_at)
);

CREATE TABLE IF NOT EXISTS reconciliation_reports (
    id VARCHAR(36) PRIMARY KEY,
    start_date TIMESTAMP NOT NULL,
    end_date TIMESTAMP NOT NULL,
    total_transactions INT NOT NULL,
    total_debits DECIMAL(19, 4) NOT NULL,
    total_credits DECIMAL(19, 4) NOT NULL,
    is_balanced BOOLEAN NOT NULL,
    discrepancies TEXT[],
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    
    INDEX idx_dates (start_date, end_date)
);
`