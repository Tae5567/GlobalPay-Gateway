// services/transaction-ledger/internal/handler/ledger_handler.go
package handler

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"transaction-ledger/internal/models"
	"transaction-ledger/internal/service"
)

type LedgerHandler struct {
	service *service.LedgerService
	logger  *zap.Logger
}

func NewLedgerHandler(service *service.LedgerService, logger *zap.Logger) *LedgerHandler {
	return &LedgerHandler{
		service: service,
		logger:  logger,
	}
}

func (h *LedgerHandler) CreateEntry(c *gin.Context) {
	var req models.LedgerEntryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	transaction, err := h.service.CreateDoubleEntry(c.Request.Context(), &req)
	if err != nil {
		h.logger.Error("failed to create ledger entry", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, transaction)
}

func (h *LedgerHandler) GetEntry(c *gin.Context) {
	entryID := c.Param("id")
	c.JSON(http.StatusOK, gin.H{"id": entryID})
}

func (h *LedgerHandler) ListEntries(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"entries": []interface{}{}})
}

func (h *LedgerHandler) GetBalance(c *gin.Context) {
	accountID := c.Param("account")
	
	balance, err := h.service.GetBalance(c.Request.Context(), accountID)
	if err != nil {
		h.logger.Error("failed to get balance", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get balance"})
		return
	}

	c.JSON(http.StatusOK, balance)
}

func (h *LedgerHandler) Reconcile(c *gin.Context) {
	var req struct {
		StartDate string `json:"start_date" binding:"required"`
		EndDate   string `json:"end_date" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	startDate, _ := time.Parse("2006-01-02", req.StartDate)
	endDate, _ := time.Parse("2006-01-02", req.EndDate)

	report, err := h.service.Reconcile(c.Request.Context(), startDate, endDate)
	if err != nil {
		h.logger.Error("reconciliation failed", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Reconciliation failed"})
		return
	}

	c.JSON(http.StatusOK, report)
}

func (h *LedgerHandler) GetTransactionEntries(c *gin.Context) {
	transactionID := c.Param("id")
	c.JSON(http.StatusOK, gin.H{"transaction_id": transactionID})
}

func (h *LedgerHandler) ListTransactions(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"transactions": []interface{}{}})
}