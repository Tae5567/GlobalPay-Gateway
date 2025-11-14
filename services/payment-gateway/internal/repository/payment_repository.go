// services/payment-gateway/internal/repository/payment_repository.go
package repository

import (
	"context"
	"database/sql"

	"payment-gateway/internal/models"
)

type PaymentRepository struct {
	db *sql.DB
}

func NewPaymentRepository(db *sql.DB) *PaymentRepository {
	return &PaymentRepository{db: db}
}

func (r *PaymentRepository) Create(ctx context.Context, payment *models.Payment) error {
	query := `
		INSERT INTO payments (
			id, amount, currency, status, card_last4, card_network,
			customer_email, description, stripe_payment_intent_id,
			client_secret, requires_3ds, idempotency_key, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
	`

	_, err := r.db.ExecContext(ctx, query,
		payment.ID,
		payment.Amount,
		payment.Currency,
		payment.Status,
		payment.CardLast4,
		payment.CardNetwork,
		payment.CustomerEmail,
		payment.Description,
		payment.StripePaymentIntentID,
		payment.ClientSecret,
		payment.Requires3DS,
		payment.IdempotencyKey,
		payment.CreatedAt,
		payment.UpdatedAt,
	)

	return err
}

func (r *PaymentRepository) GetByID(ctx context.Context, id string) (*models.Payment, error) {
	query := `
		SELECT id, amount, currency, status, card_last4, card_network,
			   customer_email, description, stripe_payment_intent_id,
			   client_secret, requires_3ds, created_at, updated_at
		FROM payments WHERE id = $1
	`

	payment := &models.Payment{}
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&payment.ID,
		&payment.Amount,
		&payment.Currency,
		&payment.Status,
		&payment.CardLast4,
		&payment.CardNetwork,
		&payment.CustomerEmail,
		&payment.Description,
		&payment.StripePaymentIntentID,
		&payment.ClientSecret,
		&payment.Requires3DS,
		&payment.CreatedAt,
		&payment.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}

	return payment, err
}

func (r *PaymentRepository) Update(ctx context.Context, payment *models.Payment) error {
	query := `
		UPDATE payments
		SET status = $1, updated_at = $2, completed_at = $3
		WHERE id = $4
	`

	_, err := r.db.ExecContext(ctx, query,
		payment.Status,
		payment.UpdatedAt,
		payment.CompletedAt,
		payment.ID,
	)

	return err
}