// services/payment-gateway/internal/models/payment.go
package models

import "time"

type PaymentStatus string

const (
	PaymentStatusPending         PaymentStatus = "pending"
	PaymentStatusRequiresAction  PaymentStatus = "requires_action"
	PaymentStatusProcessing      PaymentStatus = "processing"
	PaymentStatusSucceeded       PaymentStatus = "succeeded"
	PaymentStatusFailed          PaymentStatus = "failed"
	PaymentStatusCancelled       PaymentStatus = "cancelled"
)

type Payment struct {
	ID                     string                 `json:"id" db:"id"`
	Amount                 float64                `json:"amount" db:"amount"`
	Currency               string                 `json:"currency" db:"currency"`
	Status                 PaymentStatus          `json:"status" db:"status"`
	CardLast4              string                 `json:"card_last4" db:"card_last4"`
	CardNetwork            string                 `json:"card_network" db:"card_network"`
	CustomerEmail          string                 `json:"customer_email" db:"customer_email"`
	Description            string                 `json:"description" db:"description"`
	StripePaymentIntentID  string                 `json:"stripe_payment_intent_id,omitempty" db:"stripe_payment_intent_id"`
	ClientSecret           string                 `json:"client_secret,omitempty" db:"client_secret"`
	Requires3DS            bool                   `json:"requires_3ds" db:"requires_3ds"`
	IdempotencyKey         string                 `json:"idempotency_key,omitempty" db:"idempotency_key"`
	FailureReason          string                 `json:"failure_reason,omitempty" db:"failure_reason"`
	Metadata               map[string]interface{} `json:"metadata,omitempty" db:"metadata"`
	CreatedAt              time.Time              `json:"created_at" db:"created_at"`
	UpdatedAt              time.Time              `json:"updated_at" db:"updated_at"`
	CompletedAt            time.Time              `json:"completed_at,omitempty" db:"completed_at"`
}

type PaymentRequest struct {
	Amount          float64                `json:"amount" binding:"required,gt=0"`
	Currency        string                 `json:"currency" binding:"required,len=3"`
	CardNumber      string                 `json:"card_number" binding:"required"`
	CardExpMonth    int                    `json:"card_exp_month" binding:"required,min=1,max=12"`
	CardExpYear     int                    `json:"card_exp_year" binding:"required,min=2024"`
	CardCVC         string                 `json:"card_cvc" binding:"required,len=3"`
	CustomerEmail   string                 `json:"customer_email" binding:"required,email"`
	Description     string                 `json:"description"`
	IdempotencyKey  string                 `json:"idempotency_key"`
	Metadata        map[string]interface{} `json:"metadata"`
}

type PaymentResponse struct {
	Payment      *Payment `json:"payment"`
	NextAction   string   `json:"next_action,omitempty"`
}

// Database schema
const PaymentSchema = `
CREATE TABLE IF NOT EXISTS payments (
    id VARCHAR(36) PRIMARY KEY,
    amount DECIMAL(19, 4) NOT NULL,
    currency VARCHAR(3) NOT NULL,
    status VARCHAR(20) NOT NULL,
    card_last4 VARCHAR(4),
    card_network VARCHAR(20),
    customer_email VARCHAR(255),
    description TEXT,
    stripe_payment_intent_id VARCHAR(255),
    client_secret TEXT,
    requires_3ds BOOLEAN DEFAULT FALSE,
    idempotency_key VARCHAR(255) UNIQUE,
    failure_reason TEXT,
    metadata JSONB,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMP,
    
    INDEX idx_status (status),
    INDEX idx_customer_email (customer_email),
    INDEX idx_created_at (created_at)
);
`