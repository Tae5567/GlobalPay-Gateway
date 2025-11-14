// services/transaction-ledger/cmd/server/main.go
package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"

	"transaction-ledger/internal/handler"
	"transaction-ledger/internal/repository"
	"transaction-ledger/internal/service"
	"shared/pkg/database"
	"shared/pkg/logger"
	"shared/pkg/middleware"
)

func main() {
	// Initialize logger
	log := logger.NewLogger("transaction-ledger")
	defer log.Sync()

	// Load configuration
	cfg := loadConfig()

	// Initialize database
	db, err := database.NewPostgresDB(cfg.DatabaseURL)
	if err != nil {
		log.Fatal("failed to connect to database", zap.Error(err))
	}

	// Initialize repositories
	ledgerRepo := repository.NewLedgerRepository(db)

	// Initialize services
	ledgerService := service.NewLedgerService(ledgerRepo, log)

	// Initialize handlers
	ledgerHandler := handler.NewLedgerHandler(ledgerService, log)

	// Setup router
	router := setupRouter(ledgerHandler, log)

	// Start server
	srv := &http.Server{
		Addr:         fmt.Sprintf(":%s", cfg.Port),
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Graceful shutdown
	go func() {
		log.Info("starting transaction ledger service", zap.String("port", cfg.Port))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("failed to start server", zap.Error(err))
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info("shutting down server...")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatal("server forced to shutdown", zap.Error(err))
	}

	log.Info("server exited")
}

func setupRouter(handler *handler.LedgerHandler, log *zap.Logger) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()

	// Middleware
	router.Use(middleware.RequestID())
	router.Use(middleware.Logger(log))
	router.Use(middleware.Recovery(log))
	router.Use(middleware.CORS())

	// Health checks
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "healthy"})
	})
	router.GET("/ready", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ready"})
	})

	// Metrics endpoint
	router.GET("/metrics", gin.WrapH(promhttp.Handler()))

	// API routes
	v1 := router.Group("/api/v1")
	{
		ledger := v1.Group("/ledger")
		{
			ledger.POST("/entries", handler.CreateEntry)
			ledger.GET("/entries/:id", handler.GetEntry)
			ledger.GET("/entries", handler.ListEntries)
			ledger.GET("/balance/:account", handler.GetBalance)
			ledger.POST("/reconcile", handler.Reconcile)
		}

		transactions := v1.Group("/transactions")
		{
			transactions.GET("/:id/entries", handler.GetTransactionEntries)
			transactions.GET("", handler.ListTransactions)
		}
	}

	return router
}

type Config struct {
	Port        string
	DatabaseURL string
	Environment string
}

func loadConfig() *Config {
	return &Config{
		Port:        getEnv("PORT", "8083"),
		DatabaseURL: getEnv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/globalpay?sslmode=disable"),
		Environment: getEnv("ENVIRONMENT", "development"),
	}
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}