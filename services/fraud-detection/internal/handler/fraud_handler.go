// services/fraud-detection/internal/handler/fraud_handler.go
package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"fraud-detection/internal/models"
	"fraud-detection/internal/service"
)

type FraudHandler struct {
	engine *service.FraudEngine
	logger *zap.Logger
}

func NewFraudHandler(engine *service.FraudEngine, logger *zap.Logger) *FraudHandler {
	return &FraudHandler{
		engine: engine,
		logger: logger,
	}
}

func (h *FraudHandler) CheckFraud(c *gin.Context) {
	var req models.FraudCheckRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	result, err := h.engine.AnalyzeTransaction(c.Request.Context(), &req)
	if err != nil {
		h.logger.Error("failed to analyze transaction", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to analyze fraud"})
		return
	}

	c.JSON(http.StatusOK, result)
}

func (h *FraudHandler) GetFraudResult(c *gin.Context) {
	transactionID := c.Param("transaction_id")
	
	// In production, fetch from database
	c.JSON(http.StatusOK, gin.H{
		"transaction_id": transactionID,
		"status":         "completed",
	})
}

func (h *FraudHandler) GetFraudStats(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"total_checks":    1000,
		"high_risk_count": 50,
		"blocked_count":   10,
	})
}