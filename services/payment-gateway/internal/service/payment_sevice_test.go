// services/payment-gateway/internal/service/payment_service_test.go
package service

import (
	"testing"
)

func TestValidateLuhnChecksum(t *testing.T) {
	tests := []struct {
		name       string
		cardNumber string
		want       bool
	}{
		{
			name:       "Valid Visa",
			cardNumber: "4242424242424242",
			want:       true,
		},
		{
			name:       "Valid Mastercard",
			cardNumber: "5555555555554444",
			want:       true,
		},
		{
			name:       "Valid Amex",
			cardNumber: "378282246310005",
			want:       true,
		},
		{
			name:       "Invalid card",
			cardNumber: "1234567890123456",
			want:       false,
		},
		{
			name:       "Empty string",
			cardNumber: "",
			want:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ValidateLuhnChecksum(tt.cardNumber)
			if got != tt.want {
				t.Errorf("ValidateLuhnChecksum() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDetectCardNetwork(t *testing.T) {
	tests := []struct {
		name       string
		cardNumber string
		want       string
	}{
		{
			name:       "Visa",
			cardNumber: "4242424242424242",
			want:       "visa",
		},
		{
			name:       "Mastercard",
			cardNumber: "5555555555554444",
			want:       "mastercard",
		},
		{
			name:       "Amex",
			cardNumber: "378282246310005",
			want:       "amex",
		},
		{
			name:       "Unknown",
			cardNumber: "1234567890123456",
			want:       "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DetectCardNetwork(tt.cardNumber)
			if got != tt.want {
				t.Errorf("DetectCardNetwork() = %v, want %v", got, tt.want)
			}
		})
	}
}