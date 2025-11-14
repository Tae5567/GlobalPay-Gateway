// services/payment-gateway/internal/handler/payment_handler.go
package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"payment-gateway/internal/models"
	"payment-gateway/internal/service"
)

type PaymentHandler struct {
	service *service.PaymentService
	logger  *zap.Logger
}

func NewPaymentHandler(service *service.PaymentService, logger *zap.Logger) *PaymentHandler {
	return &PaymentHandler{
		service: service,
		logger:  logger,
	}
}

// CreatePayment handles POST /api/v1/payments
func (h *PaymentHandler) CreatePayment(c *gin.Context) {
	var req models.PaymentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	payment, err := h.service.CreatePayment(c.Request.Context(), &req)
	if err != nil {
		h.logger.Error("failed to create payment", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to process payment"})
		return
	}

	response := models.PaymentResponse{
		Payment: payment,
	}

	if payment.Requires3DS {
		response.NextAction = "complete_3ds_authentication"
	}

	c.JSON(http.StatusCreated, response)
}

// GetPayment handles GET /api/v1/payments/:id
func (h *PaymentHandler) GetPayment(c *gin.Context) {
	paymentID := c.Param("id")

	payment, err := h.service.GetPayment(c.Request.Context(), paymentID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Payment not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"payment": payment})
}

// ConfirmPayment handles POST /api/v1/payments/:id/confirm
func (h *PaymentHandler) ConfirmPayment(c *gin.Context) {
	paymentID := c.Param("id")

	payment, err := h.service.ConfirmPayment(c.Request.Context(), paymentID)
	if err != nil {
		h.logger.Error("failed to confirm payment", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to confirm payment"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"payment": payment})
}

// CancelPayment handles POST /api/v1/payments/:id/cancel
func (h *PaymentHandler) CancelPayment(c *gin.Context) {
	paymentID := c.Param("id")

	if err := h.service.CancelPayment(c.Request.Context(), paymentID); err != nil {
		h.logger.Error("failed to cancel payment", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to cancel payment"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Payment cancelled successfully"})
}

// ListPayments handles GET /api/v1/payments
func (h *PaymentHandler) ListPayments(c *gin.Context) {
	// In production, add pagination
	c.JSON(http.StatusOK, gin.H{"payments": []interface{}{}})
}

// StripeWebhook handles POST /api/v1/webhooks/stripe
func (h *PaymentHandler) StripeWebhook(c *gin.Context) {
	// Handle Stripe webhook events
	// Verify signature, process events
	c.JSON(http.StatusOK, gin.H{"received": true})
}