// services/currency-conversion/internal/service/exchange_service.go
package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"go.uber.org/zap"

	"currency-conversion/internal/models"
	"currency-conversion/internal/repository"
	"shared/pkg/redis"
)

type ExchangeService struct {
	repo        *repository.RateRepository
	redisClient *redis.Client
	apiKey      string
	apiURL      string
	logger      *zap.Logger
}

func NewExchangeService(repo *repository.RateRepository, redisClient *redis.Client, apiKey string, logger *zap.Logger) *ExchangeService {
	return &ExchangeService{
		repo:        repo,
		redisClient: redisClient,
		apiKey:      apiKey,
		apiURL:      "https://v6.exchangerate-api.com/v6",
		logger:      logger,
	}
}

// Convert converts an amount from one currency to another
func (s *ExchangeService) Convert(ctx context.Context, req *models.ConversionRequest) (*models.ConversionResponse, error) {
	// Get exchange rate
	rate, err := s.GetRate(ctx, req.FromCurrency, req.ToCurrency)
	if err != nil {
		return nil, fmt.Errorf("failed to get exchange rate: %w", err)
	}

	// Calculate converted amount
	convertedAmount := req.Amount * rate.Rate

	// Calculate fee (0.5% for example)
	feePercentage := 0.005
	fee := convertedAmount * feePercentage
	finalAmount := convertedAmount - fee

	response := &models.ConversionResponse{
		OriginalAmount:   req.Amount,
		ConvertedAmount:  finalAmount,
		FromCurrency:     req.FromCurrency,
		ToCurrency:       req.ToCurrency,
		ExchangeRate:     rate.Rate,
		Fee:              fee,
		FeePercentage:    feePercentage,
		RateTimestamp:    rate.Timestamp,
		ConversionID:     generateConversionID(),
	}

	// Save conversion history
	conversion := &models.Conversion{
		ID:              response.ConversionID,
		FromCurrency:    req.FromCurrency,
		ToCurrency:      req.ToCurrency,
		OriginalAmount:  req.Amount,
		ConvertedAmount: finalAmount,
		ExchangeRate:    rate.Rate,
		Fee:             fee,
		CreatedAt:       time.Now(),
	}
	
	if err := s.repo.SaveConversion(ctx, conversion); err != nil {
		s.logger.Error("failed to save conversion", zap.Error(err))
	}

	return response, nil
}

// GetRate retrieves the exchange rate with caching
func (s *ExchangeService) GetRate(ctx context.Context, from, to string) (*models.ExchangeRate, error) {
	// Check cache first
	cacheKey := fmt.Sprintf("rate:%s:%s", from, to)
	
	if cached, err := s.getCachedRate(ctx, cacheKey); err == nil && cached != nil {
		s.logger.Debug("cache hit for exchange rate", 
			zap.String("from", from), 
			zap.String("to", to))
		return cached, nil
	}

	// Fetch from API
	rate, err := s.fetchRateFromAPI(from, to)
	if err != nil {
		// Try to get from database as fallback
		if dbRate, dbErr := s.repo.GetLatestRate(ctx, from, to); dbErr == nil {
			s.logger.Warn("using database fallback for exchange rate", 
				zap.String("from", from), 
				zap.String("to", to))
			return dbRate, nil
		}
		return nil, err
	}

	// Cache the rate (5 minutes TTL)
	s.cacheRate(ctx, cacheKey, rate, 5*time.Minute)

	// Save to database for historical tracking
	if err := s.repo.SaveRate(ctx, rate); err != nil {
		s.logger.Error("failed to save rate to database", zap.Error(err))
	}

	return rate, nil
}

// fetchRateFromAPI fetches exchange rate from external API
func (s *ExchangeService) fetchRateFromAPI(from, to string) (*models.ExchangeRate, error) {
	url := fmt.Sprintf("%s/%s/pair/%s/%s", s.apiURL, s.apiKey, from, to)
	
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("API request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var apiResp struct {
		Result          string  `json:"result"`
		ConversionRate  float64 `json:"conversion_rate"`
		TimeLastUpdate  int64   `json:"time_last_update_unix"`
	}

	if err := json.Unmarshal(body, &apiResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if apiResp.Result != "success" {
		return nil, fmt.Errorf("API returned error result")
	}

	rate := &models.ExchangeRate{
		FromCurrency: from,
		ToCurrency:   to,
		Rate:         apiResp.ConversionRate,
		Timestamp:    time.Unix(apiResp.TimeLastUpdate, 0),
		Source:       "exchangerate-api.com",
	}

	return rate, nil
}

// GetHistoricalRates retrieves historical rates for a currency pair
func (s *ExchangeService) GetHistoricalRates(ctx context.Context, from, to string, days int) ([]*models.ExchangeRate, error) {
	startDate := time.Now().AddDate(0, 0, -days)
	return s.repo.GetRateHistory(ctx, from, to, startDate)
}

// GetSupportedCurrencies returns list of supported currencies
func (s *ExchangeService) GetSupportedCurrencies() []string {
	return []string{
		"USD", "EUR", "GBP", "JPY", "AUD", "CAD", "CHF", "CNY",
		"SEK", "NZD", "MXN", "SGD", "HKD", "NOK", "KRW", "TRY",
		"INR", "RUB", "BRL", "ZAR", "DKK", "PLN", "THB", "IDR",
		"HUF", "CZK", "ILS", "CLP", "PHP", "AED", "SAR", "MYR",
	}
}

// Cache helpers

func (s *ExchangeService) getCachedRate(ctx context.Context, key string) (*models.ExchangeRate, error) {
	data, err := s.redisClient.Get(ctx, key)
	if err != nil {
		return nil, err
	}

	var rate models.ExchangeRate
	if err := json.Unmarshal([]byte(data), &rate); err != nil {
		return nil, err
	}

	return &rate, nil
}

func (s *ExchangeService) cacheRate(ctx context.Context, key string, rate *models.ExchangeRate, ttl time.Duration) {
	data, _ := json.Marshal(rate)
	s.redisClient.Set(ctx, key, data, ttl)
}

func generateConversionID() string {
	return fmt.Sprintf("conv_%d", time.Now().UnixNano())
}