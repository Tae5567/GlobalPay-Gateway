// services/currency-conversion/internal/repository/rate_repository.go
package repository

import (
	"context"
	"database/sql"
	"time"

	"currency-conversion/internal/models"
)

type RateRepository struct {
	db *sql.DB
}

func NewRateRepository(db *sql.DB) *RateRepository {
	return &RateRepository{db: db}
}

func (r *RateRepository) SaveRate(ctx context.Context, rate *models.ExchangeRate) error {
	query := `
		INSERT INTO exchange_rates (from_currency, to_currency, rate, source, timestamp)
		VALUES ($1, $2, $3, $4, $5)
	`
	_, err := r.db.ExecContext(ctx, query,
		rate.FromCurrency,
		rate.ToCurrency,
		rate.Rate,
		rate.Source,
		rate.Timestamp,
	)
	return err
}

func (r *RateRepository) GetLatestRate(ctx context.Context, from, to string) (*models.ExchangeRate, error) {
	query := `
		SELECT from_currency, to_currency, rate, source, timestamp
		FROM exchange_rates
		WHERE from_currency = $1 AND to_currency = $2
		ORDER BY timestamp DESC
		LIMIT 1
	`
	
	rate := &models.ExchangeRate{}
	err := r.db.QueryRowContext(ctx, query, from, to).Scan(
		&rate.FromCurrency,
		&rate.ToCurrency,
		&rate.Rate,
		&rate.Source,
		&rate.Timestamp,
	)
	
	if err == sql.ErrNoRows {
		return nil, nil
	}
	
	return rate, err
}

func (r *RateRepository) GetRateHistory(ctx context.Context, from, to string, since time.Time) ([]*models.ExchangeRate, error) {
	query := `
		SELECT from_currency, to_currency, rate, source, timestamp
		FROM exchange_rates
		WHERE from_currency = $1 AND to_currency = $2 AND timestamp >= $3
		ORDER BY timestamp ASC
	`
	
	rows, err := r.db.QueryContext(ctx, query, from, to, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var rates []*models.ExchangeRate
	for rows.Next() {
		rate := &models.ExchangeRate{}
		if err := rows.Scan(&rate.FromCurrency, &rate.ToCurrency, &rate.Rate, &rate.Source, &rate.Timestamp); err != nil {
			return nil, err
		}
		rates = append(rates, rate)
	}

	return rates, nil
}

func (r *RateRepository) SaveConversion(ctx context.Context, conversion *models.Conversion) error {
	query := `
		INSERT INTO conversions (id, from_currency, to_currency, original_amount, converted_amount, exchange_rate, fee, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`
	_, err := r.db.ExecContext(ctx, query,
		conversion.ID,
		conversion.FromCurrency,
		conversion.ToCurrency,
		conversion.OriginalAmount,
		conversion.ConvertedAmount,
		conversion.ExchangeRate,
		conversion.Fee,
		conversion.CreatedAt,
	)
	return err
}