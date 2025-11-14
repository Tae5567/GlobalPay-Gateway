package models

import "time"

type ConversionRequest struct {
	Amount       float64 `json:"amount" binding:"required,gt=0"`
	FromCurrency string  `json:"from_currency" binding:"required,len=3"`
	ToCurrency   string  `json:"to_currency" binding:"required,len=3"`
}

type ConversionResponse struct {
	OriginalAmount   float64   `json:"original_amount"`
	ConvertedAmount  float64   `json:"converted_amount"`
	FromCurrency     string    `json:"from_currency"`
	ToCurrency       string    `json:"to_currency"`
	ExchangeRate     float64   `json:"exchange_rate"`
	Fee              float64   `json:"fee"`
	FeePercentage    float64   `json:"fee_percentage"`
	RateTimestamp    time.Time `json:"rate_timestamp"`
	ConversionID     string    `json:"conversion_id"`
}

type ExchangeRate struct {
	FromCurrency string    `json:"from_currency" db:"from_currency"`
	ToCurrency   string    `json:"to_currency" db:"to_currency"`
	Rate         float64   `json:"rate" db:"rate"`
	Timestamp    time.Time `json:"timestamp" db:"timestamp"`
	Source       string    `json:"source" db:"source"`
}

type Conversion struct {
	ID              string    `json:"id" db:"id"`
	FromCurrency    string    `json:"from_currency" db:"from_currency"`
	ToCurrency      string    `json:"to_currency" db:"to_currency"`
	OriginalAmount  float64   `json:"original_amount" db:"original_amount"`
	ConvertedAmount float64   `json:"converted_amount" db:"converted_amount"`
	ExchangeRate    float64   `json:"exchange_rate" db:"exchange_rate"`
	Fee             float64   `json:"fee" db:"fee"`
	CreatedAt       time.Time `json:"created_at" db:"created_at"`
}