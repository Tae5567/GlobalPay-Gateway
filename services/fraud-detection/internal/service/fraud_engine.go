// services/fraud-detection/internal/service/fraud_engine.go
// Fraud checks
package service

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/zap"

	"fraud-detection/internal/models"
	"fraud-detection/internal/repository"
)

type FraudEngine struct {
	repo   *repository.FraudRepository
	logger *zap.Logger
}

func NewFraudEngine(repo *repository.FraudRepository, logger *zap.Logger) *FraudEngine {
	return &FraudEngine{
		repo:   repo,
		logger: logger,
	}
}

// AnalyzeTransaction performs fraud analysis on a transaction
func (s *FraudEngine) AnalyzeTransaction(ctx context.Context, req *models.FraudCheckRequest) (*models.FraudCheckResponse, error) {
	startTime := time.Now()
	
	// Initialize response
	response := &models.FraudCheckResponse{
		TransactionID: req.TransactionID,
		Score:         0,
		RiskLevel:     models.RiskLevelLow,
		Flags:         []string{},
		Rules:         []models.RuleResult{},
		Timestamp:     time.Now(),
	}

	// Run all fraud detection rules
	rules := []func(context.Context, *models.FraudCheckRequest, *models.FraudCheckResponse) error{
		s.checkVelocity,
		s.checkAmountThreshold,
		s.checkGeolocation,
		s.checkBlacklist,
		s.checkTimePattern,
		s.checkDeviceFingerprint,
	}

	for _, rule := range rules {
		if err := rule(ctx, req, response); err != nil {
			s.logger.Error("fraud rule execution failed", 
				zap.Error(err),
				zap.String("transaction_id", req.TransactionID))
		}
	}

	// Calculate final risk level
	response.RiskLevel = s.calculateRiskLevel(response.Score)
	response.Decision = s.makeDecision(response.RiskLevel, response.Score)
	
	// Save fraud check result
	result := &models.FraudCheckResult{
		TransactionID: req.TransactionID,
		Score:         response.Score,
		RiskLevel:     string(response.RiskLevel),
		Decision:      string(response.Decision),
		Flags:         response.Flags,
		ProcessingMS:  time.Since(startTime).Milliseconds(),
		CreatedAt:     time.Now(),
	}

	if err := s.repo.SaveFraudCheck(ctx, result); err != nil {
		s.logger.Error("failed to save fraud check", zap.Error(err))
	}

	// Send webhook if high risk
	if response.RiskLevel == models.RiskLevelHigh {
		s.sendFraudAlert(ctx, response)
	}

	return response, nil
}

// checkVelocity checks transaction velocity (transactions per time window)
func (s *FraudEngine) checkVelocity(ctx context.Context, req *models.FraudCheckRequest, resp *models.FraudCheckResponse) error {
	// Check transactions in last hour
	count, err := s.repo.CountRecentTransactions(ctx, req.CustomerEmail, 1*time.Hour)
	if err != nil {
		return err
	}

	ruleResult := models.RuleResult{
		RuleName:    "velocity_check",
		Triggered:   false,
		Score:       0,
		Description: fmt.Sprintf("Transaction count in last hour: %d", count),
	}

	// Thresholds
	if count > 10 {
		ruleResult.Triggered = true
		ruleResult.Score = 40
		resp.Flags = append(resp.Flags, "high_velocity")
		resp.Score += 40
	} else if count > 5 {
		ruleResult.Triggered = true
		ruleResult.Score = 20
		resp.Flags = append(resp.Flags, "moderate_velocity")
		resp.Score += 20
	}

	resp.Rules = append(resp.Rules, ruleResult)
	return nil
}

// checkAmountThreshold checks for unusually large amounts
func (s *FraudEngine) checkAmountThreshold(ctx context.Context, req *models.FraudCheckRequest, resp *models.FraudCheckResponse) error {
	ruleResult := models.RuleResult{
		RuleName:    "amount_threshold",
		Triggered:   false,
		Score:       0,
		Description: fmt.Sprintf("Transaction amount: %.2f %s", req.Amount, req.Currency),
	}

	// Convert to USD for consistent checking
	amountUSD := req.Amount
	if req.Currency != "USD" {
		// In production, convert using currency service
		amountUSD = req.Amount * 1.0 // Placeholder
	}

	if amountUSD > 10000 {
		ruleResult.Triggered = true
		ruleResult.Score = 30
		resp.Flags = append(resp.Flags, "large_amount")
		resp.Score += 30
	} else if amountUSD > 5000 {
		ruleResult.Triggered = true
		ruleResult.Score = 15
		resp.Flags = append(resp.Flags, "elevated_amount")
		resp.Score += 15
	}

	resp.Rules = append(resp.Rules, ruleResult)
	return nil
}

// checkGeolocation checks for suspicious location patterns
func (s *FraudEngine) checkGeolocation(ctx context.Context, req *models.FraudCheckRequest, resp *models.FraudCheckResponse) error {
	ruleResult := models.RuleResult{
		RuleName:    "geolocation_check",
		Triggered:   false,
		Score:       0,
		Description: fmt.Sprintf("Country: %s", req.Country),
	}

	// Get customer's usual locations
	recentLocations, err := s.repo.GetRecentLocations(ctx, req.CustomerEmail, 30*24*time.Hour)
	if err != nil {
		return err
	}

	// Check if current location is unusual
	if len(recentLocations) > 0 {
		isNewLocation := true
		for _, loc := range recentLocations {
			if loc == req.Country {
				isNewLocation = false
				break
			}
		}

		if isNewLocation {
			ruleResult.Triggered = true
			ruleResult.Score = 25
			resp.Flags = append(resp.Flags, "new_location")
			resp.Score += 25
		}
	}

	// Check high-risk countries (example list)
	highRiskCountries := map[string]bool{
		"XX": true, // Example codes
		"YY": true,
	}

	if highRiskCountries[req.Country] {
		ruleResult.Triggered = true
		ruleResult.Score = 35
		resp.Flags = append(resp.Flags, "high_risk_country")
		resp.Score += 35
	}

	resp.Rules = append(resp.Rules, ruleResult)
	return nil
}

// checkBlacklist checks if customer/card is blacklisted
func (s *FraudEngine) checkBlacklist(ctx context.Context, req *models.FraudCheckRequest, resp *models.FraudCheckResponse) error {
	ruleResult := models.RuleResult{
		RuleName:    "blacklist_check",
		Triggered:   false,
		Score:       0,
		Description: "Checking blacklist status",
	}

	isBlacklisted, err := s.repo.IsBlacklisted(ctx, req.CustomerEmail, req.CardLast4)
	if err != nil {
		return err
	}

	if isBlacklisted {
		ruleResult.Triggered = true
		ruleResult.Score = 100 // Automatic block
		resp.Flags = append(resp.Flags, "blacklisted")
		resp.Score = 100
	}

	resp.Rules = append(resp.Rules, ruleResult)
	return nil
}

// checkTimePattern checks for unusual transaction timing
func (s *FraudEngine) checkTimePattern(ctx context.Context, req *models.FraudCheckRequest, resp *models.FraudCheckResponse) error {
	ruleResult := models.RuleResult{
		RuleName:    "time_pattern",
		Triggered:   false,
		Score:       0,
		Description: fmt.Sprintf("Transaction hour: %d", time.Now().Hour()),
	}

	hour := time.Now().Hour()
	
	// Transactions between 2 AM and 5 AM are more suspicious
	if hour >= 2 && hour <= 5 {
		ruleResult.Triggered = true
		ruleResult.Score = 10
		resp.Flags = append(resp.Flags, "unusual_hour")
		resp.Score += 10
	}

	resp.Rules = append(resp.Rules, ruleResult)
	return nil
}

// checkDeviceFingerprint checks device consistency
func (s *FraudEngine) checkDeviceFingerprint(ctx context.Context, req *models.FraudCheckRequest, resp *models.FraudCheckResponse) error {
	ruleResult := models.RuleResult{
		RuleName:    "device_fingerprint",
		Triggered:   false,
		Score:       0,
		Description: "Device fingerprint analysis",
	}

	if req.DeviceFingerprint != "" {
		isKnownDevice, err := s.repo.IsKnownDevice(ctx, req.CustomerEmail, req.DeviceFingerprint)
		if err != nil {
			return err
		}

		if !isKnownDevice {
			ruleResult.Triggered = true
			ruleResult.Score = 15
			resp.Flags = append(resp.Flags, "new_device")
			resp.Score += 15
		}
	}

	resp.Rules = append(resp.Rules, ruleResult)
	return nil
}

// calculateRiskLevel determines risk level based on score
func (s *FraudEngine) calculateRiskLevel(score int) models.RiskLevel {
	switch {
	case score >= 70:
		return models.RiskLevelHigh
	case score >= 40:
		return models.RiskLevelMedium
	default:
		return models.RiskLevelLow
	}
}

// makeDecision decides whether to approve, review, or block
func (s *FraudEngine) makeDecision(riskLevel models.RiskLevel, score int) models.Decision {
	switch riskLevel {
	case models.RiskLevelHigh:
		if score >= 90 {
			return models.DecisionBlock
		}
		return models.DecisionReview
	case models.RiskLevelMedium:
		return models.DecisionReview
	default:
		return models.DecisionApprove
	}
}

// sendFraudAlert sends webhook notification for high-risk transactions
func (s *FraudEngine) sendFraudAlert(ctx context.Context, response *models.FraudCheckResponse) {
	// In production, send to webhook endpoint
	s.logger.Warn("high-risk transaction detected",
		zap.String("transaction_id", response.TransactionID),
		zap.Int("score", response.Score),
		zap.Strings("flags", response.Flags))
}