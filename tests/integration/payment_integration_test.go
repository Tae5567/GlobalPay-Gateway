// tests/integration/payment_integration_test.go
//go:build integration
// +build integration

package integration

import (
	"context"
	"database/sql"
	"testing"
	"time"

	_ "github.com/lib/pq"
)

func TestPaymentFlow(t *testing.T) {
	// Setup test database
	db, err := sql.Open("postgres", "postgres://postgres:postgres@localhost:5432/globalpay_test?sslmode=disable")
	if err != nil {
		t.Fatalf("Failed to connect to test database: %v", err)
	}
	defer db.Close()

	// Test payment creation
	ctx := context.Background()
	
	// Create test payment
	query := `
		INSERT INTO payments (id, amount, currency, status, card_last4, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`
	
	_, err = db.ExecContext(ctx, query, 
		"test-payment-1",
		100.00,
		"USD",
		"pending",
		"4242",
		time.Now(),
		time.Now(),
	)
	
	if err != nil {
		t.Fatalf("Failed to create payment: %v", err)
	}

	// Verify payment was created
	var count int
	err = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM payments WHERE id = $1", "test-payment-1").Scan(&count)
	if err != nil {
		t.Fatalf("Failed to query payment: %v", err)
	}

	if count != 1 {
		t.Errorf("Expected 1 payment, got %d", count)
	}

	// Cleanup
	_, err = db.ExecContext(ctx, "DELETE FROM payments WHERE id = $1", "test-payment-1")
	if err != nil {
		t.Logf("Failed to cleanup: %v", err)
	}
}