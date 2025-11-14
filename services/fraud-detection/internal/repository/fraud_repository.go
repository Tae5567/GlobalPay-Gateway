// services/fraud-detection/internal/repository/fraud_repository.go
package repository

import (
	"context"
	"database/sql"
	"time"

	"github.com/lib/pq"
	"fraud-detection/internal/models"
)

type FraudRepository struct {
	db *sql.DB
}

func NewFraudRepository(db *sql.DB) *FraudRepository {
	return &FraudRepository{db: db}
}

func (r *FraudRepository) SaveFraudCheck(ctx context.Context, result *models.FraudCheckResult) error {
	query := `
		INSERT INTO fraud_check_results (id, transaction_id, score, risk_level, decision, flags, processing_ms, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`
	_, err := r.db.ExecContext(ctx, query,
		result.TransactionID,
		result.TransactionID,
		result.Score,
		result.RiskLevel,
		result.Decision,
		pq.Array(result.Flags),
		result.ProcessingMS,
		result.CreatedAt,
	)
	return err
}

func (r *FraudRepository) CountRecentTransactions(ctx context.Context, email string, duration time.Duration) (int, error) {
	query := `
		SELECT COUNT(*) FROM fraud_check_results
		WHERE transaction_id IN (
			SELECT id FROM payments WHERE customer_email = $1 AND created_at >= $2
		)
	`
	
	var count int
	since := time.Now().Add(-duration)
	err := r.db.QueryRowContext(ctx, query, email, since).Scan(&count)
	return count, err
}

func (r *FraudRepository) GetRecentLocations(ctx context.Context, email string, duration time.Duration) ([]string, error) {
	// Simplified - in production, track locations properly
	return []string{"US", "CA"}, nil
}

func (r *FraudRepository) IsBlacklisted(ctx context.Context, email, cardLast4 string) (bool, error) {
	// Simplified - in production, maintain blacklist table
	return false, nil
}

func (r *FraudRepository) IsKnownDevice(ctx context.Context, email, deviceFingerprint string) (bool, error) {
	// Simplified - in production, track device fingerprints
	return true, nil
}