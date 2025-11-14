// services/fraud-detection/internal/models/fraud.go
package models

import "time"

type RiskLevel string
type Decision string

const (
	RiskLevelLow    RiskLevel = "low"
	RiskLevelMedium RiskLevel = "medium"
	RiskLevelHigh   RiskLevel = "high"

	DecisionApprove Decision = "approve"
	DecisionReview  Decision = "review"
	DecisionBlock   Decision = "block"
)

type FraudCheckRequest struct {
	TransactionID     string  `json:"transaction_id" binding:"required"`
	Amount            float64 `json:"amount" binding:"required,gt=0"`
	Currency          string  `json:"currency" binding:"required,len=3"`
	CustomerEmail     string  `json:"customer_email" binding:"required,email"`
	Country           string  `json:"country" binding:"required"`
	CardLast4         string  `json:"card_last4"`
	DeviceFingerprint string  `json:"device_fingerprint"`
	IPAddress         string  `json:"ip_address"`
}

type FraudCheckResponse struct {
	TransactionID string       `json:"transaction_id"`
	Score         int          `json:"score"`
	RiskLevel     RiskLevel    `json:"risk_level"`
	Decision      Decision     `json:"decision"`
	Flags         []string     `json:"flags"`
	Rules         []RuleResult `json:"rules"`
	Timestamp     time.Time    `json:"timestamp"`
}

type RuleResult struct {
	RuleName    string `json:"rule_name"`
	Triggered   bool   `json:"triggered"`
	Score       int    `json:"score"`
	Description string `json:"description"`
}

type FraudCheckResult struct {
	ID           string    `json:"id" db:"id"`
	TransactionID string    `json:"transaction_id" db:"transaction_id"`
	Score         int       `json:"score" db:"score"`
	RiskLevel     string    `json:"risk_level" db:"risk_level"`
	Decision      string    `json:"decision" db:"decision"`
	Flags         []string  `json:"flags" db:"flags"`
	ProcessingMS  int64     `json:"processing_ms" db:"processing_ms"`
	CreatedAt     time.Time `json:"created_at" db:"created_at"`
}