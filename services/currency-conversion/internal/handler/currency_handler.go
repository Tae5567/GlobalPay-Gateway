// services/currency-conversion/internal/handler/currency_handler.go
package handler

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"currency-conversion/internal/models"
	"currency-conversion/internal/service"
)

type CurrencyHandler struct {
	service *service.ExchangeService
	logger  *zap.Logger
}

func NewCurrencyHandler(service *service.ExchangeService, logger *zap.Logger) *CurrencyHandler {
	return &CurrencyHandler{
		service: service,
		logger:  logger,
	}
}

func (h *CurrencyHandler) ConvertCurrency(c *gin.Context) {
	var req models.ConversionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	result, err := h.service.Convert(c.Request.Context(), &req)
	if err != nil {
		h.logger.Error("failed to convert currency", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to convert currency"})
		return
	}

	c.JSON(http.StatusOK, result)
}

func (h *CurrencyHandler) GetRate(c *gin.Context) {
	from := c.Param("from")
	to := c.Param("to")

	rate, err := h.service.GetRate(c.Request.Context(), from, to)
	if err != nil {
		h.logger.Error("failed to get rate", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get exchange rate"})
		return
	}

	c.JSON(http.StatusOK, rate)
}

func (h *CurrencyHandler) GetRateHistory(c *gin.Context) {
	from := c.Param("from")
	to := c.Param("to")
	
	daysStr := c.DefaultQuery("days", "30")
	days, err := strconv.Atoi(daysStr)
	if err != nil || days < 1 || days > 365 {
		days = 30
	}

	rates, err := h.service.GetHistoricalRates(c.Request.Context(), from, to, days)
	if err != nil {
		h.logger.Error("failed to get historical rates", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get rate history"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"rates": rates})
}

func (h *CurrencyHandler) GetSupportedCurrencies(c *gin.Context) {
	currencies := h.service.GetSupportedCurrencies()
	c.JSON(http.StatusOK, gin.H{"currencies": currencies})
}