// tests/e2e/payment_flow_test.go
//go:build e2e
// +build e2e

package e2e

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"
	"time"
)

func TestCreatePaymentE2E(t *testing.T) {
	baseURL := "http://localhost:8080"

	// Create payment request
	payload := map[string]interface{}{
		"amount":          100.00,
		"currency":        "USD",
		"card_number":     "4242424242424242",
		"card_exp_month":  12,
		"card_exp_year":   2025,
		"card_cvc":        "123",
		"customer_email":  "test@example.com",
		"description":     "E2E Test Payment",
		"idempotency_key": "e2e-test-" + time.Now().Format("20060102150405"),
	}

	jsonData, _ := json.Marshal(payload)
	
	// Send request
	resp, err := http.Post(
		baseURL+"/api/v1/payments",
		"application/json",
		bytes.NewBuffer(jsonData),
	)
	
	if err != nil {
		t.Fatalf("Failed to create payment: %v", err)
	}
	defer resp.Body.Close()

	// Check response
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		t.Errorf("Expected status 200 or 201, got %d", resp.StatusCode)
	}

	// Parse response
	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	// Verify response contains payment ID
	payment, ok := result["payment"].(map[string]interface{})
	if !ok {
		t.Fatal("Response doesn't contain payment object")
	}

	if payment["id"] == nil {
		t.Error("Payment ID is missing")
	}

	t.Logf("Payment created successfully: %v", payment["id"])
}