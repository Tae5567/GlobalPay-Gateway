// services/payment-gateway/internal/service/payment_service.go
package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/stripe/stripe-go/v76"
	"github.com/stripe/stripe-go/v76/paymentintent"

	"payment-gateway/internal/models"
	"payment-gateway/internal/repository"
	"shared/pkg/redis"
)

type PaymentService struct {
	repo        *repository.PaymentRepository
	redisClient *redis.Client
	stripeKey   string
}

func NewPaymentService(repo *repository.PaymentRepository, redisClient *redis.Client, cfg interface{}) *PaymentService {
	// Set Stripe API key
	stripe.Key = cfg.(map[string]string)["stripe_key"]
	
	return &PaymentService{
		repo:        repo,
		redisClient: redisClient,
		stripeKey:   cfg.(map[string]string)["stripe_key"],
	}
}

// CreatePayment creates a new payment with idempotency
func (s *PaymentService) CreatePayment(ctx context.Context, req *models.PaymentRequest) (*models.Payment, error) {
	// Check idempotency key
	if req.IdempotencyKey != "" {
		if cached, err := s.getIdempotentPayment(ctx, req.IdempotencyKey); err == nil && cached != nil {
			return cached, nil
		}
	}

	// Validate card using Luhn algorithm
	if !ValidateLuhnChecksum(req.CardNumber) {
		return nil, errors.New("invalid card number")
	}

	// Detect card network
	cardNetwork := DetectCardNetwork(req.CardNumber)
	if cardNetwork == "" {
		return nil, errors.New("unsupported card network")
	}

	// Create payment record
	payment := &models.Payment{
		ID:              uuid.New().String(),
		Amount:          req.Amount,
		Currency:        req.Currency,
		Status:          models.PaymentStatusPending,
		CardLast4:       req.CardNumber[len(req.CardNumber)-4:],
		CardNetwork:     cardNetwork,
		CustomerEmail:   req.CustomerEmail,
		Description:     req.Description,
		IdempotencyKey:  req.IdempotencyKey,
		Metadata:        req.Metadata,
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}

	// Process with Stripe
	stripeIntent, err := s.createStripePaymentIntent(req)
	if err != nil {
		payment.Status = models.PaymentStatusFailed
		payment.FailureReason = err.Error()
		s.repo.Create(ctx, payment)
		return nil, fmt.Errorf("stripe payment failed: %w", err)
	}

	payment.StripePaymentIntentID = stripeIntent.ID
	payment.ClientSecret = stripeIntent.ClientSecret

	// Check if 3DS is required
	if stripeIntent.Status == stripe.PaymentIntentStatusRequiresAction {
		payment.Requires3DS = true
		payment.Status = models.PaymentStatusRequiresAction
	}

	// Save to database
	if err := s.repo.Create(ctx, payment); err != nil {
		return nil, fmt.Errorf("failed to save payment: %w", err)
	}

	// Cache for idempotency
	if req.IdempotencyKey != "" {
		s.cacheIdempotentPayment(ctx, req.IdempotencyKey, payment)
	}

	// Publish event
	s.publishPaymentEvent(ctx, "payment.created", payment)

	return payment, nil
}

// ConfirmPayment confirms a payment after 3DS authentication
func (s *PaymentService) ConfirmPayment(ctx context.Context, paymentID string) (*models.Payment, error) {
	payment, err := s.repo.GetByID(ctx, paymentID)
	if err != nil {
		return nil, err
	}

	// Confirm with Stripe
	intent, err := paymentintent.Confirm(payment.StripePaymentIntentID, nil)
	if err != nil {
		return nil, err
	}

	// Update payment status
	if intent.Status == stripe.PaymentIntentStatusSucceeded {
		payment.Status = models.PaymentStatusSucceeded
		payment.CompletedAt = time.Now()
		s.publishPaymentEvent(ctx, "payment.succeeded", payment)
	} else if intent.Status == stripe.PaymentIntentStatusProcessing {
		payment.Status = models.PaymentStatusProcessing
	}

	payment.UpdatedAt = time.Now()
	if err := s.repo.Update(ctx, payment); err != nil {
		return nil, err
	}

	return payment, nil
}

// GetPayment retrieves a payment by ID
func (s *PaymentService) GetPayment(ctx context.Context, paymentID string) (*models.Payment, error) {
	return s.repo.GetByID(ctx, paymentID)
}

// CancelPayment cancels a pending payment
func (s *PaymentService) CancelPayment(ctx context.Context, paymentID string) error {
	payment, err := s.repo.GetByID(ctx, paymentID)
	if err != nil {
		return err
	}

	if payment.Status != models.PaymentStatusPending && payment.Status != models.PaymentStatusRequiresAction {
		return errors.New("payment cannot be cancelled")
	}

	// Cancel with Stripe
	_, err = paymentintent.Cancel(payment.StripePaymentIntentID, nil)
	if err != nil {
		return err
	}

	payment.Status = models.PaymentStatusCancelled
	payment.UpdatedAt = time.Now()
	
	if err := s.repo.Update(ctx, payment); err != nil {
		return err
	}

	s.publishPaymentEvent(ctx, "payment.cancelled", payment)
	return nil
}

// Helper functions

func (s *PaymentService) createStripePaymentIntent(req *models.PaymentRequest) (*stripe.PaymentIntent, error) {
	params := &stripe.PaymentIntentParams{
		Amount:   stripe.Int64(int64(req.Amount * 100)), // Convert to cents
		Currency: stripe.String(req.Currency),
		PaymentMethodTypes: stripe.StringSlice([]string{
			"card",
		}),
		Description: stripe.String(req.Description),
	}

	if req.CustomerEmail != "" {
		params.ReceiptEmail = stripe.String(req.CustomerEmail)
	}

	return paymentintent.New(params)
}

func (s *PaymentService) getIdempotentPayment(ctx context.Context, key string) (*models.Payment, error) {
	cacheKey := fmt.Sprintf("idempotency:%s", key)
	data, err := s.redisClient.Get(ctx, cacheKey)
	if err != nil {
		return nil, err
	}

	var payment models.Payment
	if err := json.Unmarshal([]byte(data), &payment); err != nil {
		return nil, err
	}

	return &payment, nil
}

func (s *PaymentService) cacheIdempotentPayment(ctx context.Context, key string, payment *models.Payment) {
	cacheKey := fmt.Sprintf("idempotency:%s", key)
	data, _ := json.Marshal(payment)
	s.redisClient.Set(ctx, cacheKey, data, 24*time.Hour)
}

func (s *PaymentService) publishPaymentEvent(ctx context.Context, eventType string, payment *models.Payment) {
	// This would publish to Kafka/RabbitMQ
	// For now, just log
	fmt.Printf("Event: %s - Payment ID: %s\n", eventType, payment.ID)
}

// ValidateLuhnChecksum validates a card number using Luhn algorithm
func ValidateLuhnChecksum(cardNumber string) bool {
	var sum int
	parity := len(cardNumber) % 2

	for i, digit := range cardNumber {
		d := int(digit - '0')
		if i%2 == parity {
			d *= 2
			if d > 9 {
				d -= 9
			}
		}
		sum += d
	}

	return sum%10 == 0
}

// DetectCardNetwork detects the card network based on IIN
func DetectCardNetwork(cardNumber string) string {
	if len(cardNumber) < 2 {
		return ""
	}

	prefix := cardNumber[:2]
	
	switch {
	case prefix == "34" || prefix == "37":
		return "amex"
	case prefix >= "40" && prefix <= "49":
		return "visa"
	case prefix >= "51" && prefix <= "55":
		return "mastercard"
	case prefix >= "22" && prefix <= "27":
		return "mastercard"
	case prefix >= "60" && prefix <= "65":
		return "discover"
	default:
		return ""
	}
}