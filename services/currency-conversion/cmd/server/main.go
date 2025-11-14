// services/currency-conversion/cmd/server/main.go
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

	"currency-conversion/internal/handler"
	"currency-conversion/internal/repository"
	"currency-conversion/internal/service"
	"shared/pkg/database"
	"shared/pkg/logger"
	"shared/pkg/middleware"
	"shared/pkg/redis"
)

func main() {
	log := logger.NewLogger("currency-conversion")
	defer log.Sync()

	cfg := loadConfig()

	// Initialize database
	db, err := database.NewPostgresDB(cfg.DatabaseURL)
	if err != nil {
		log.Fatal("failed to connect to database", zap.Error(err))
	}

	// Initialize Redis
	redisClient := redis.NewRedisClient(cfg.RedisURL)

	// Initialize repositories
	rateRepo := repository.NewRateRepository(db)

	// Initialize services
	exchangeService := service.NewExchangeService(rateRepo, redisClient, cfg.ExchangeAPIKey, log)

	// Initialize handlers
	currencyHandler := handler.NewCurrencyHandler(exchangeService, log)

	// Setup router
	router := setupRouter(currencyHandler, log)

	srv := &http.Server{
		Addr:         fmt.Sprintf(":%s", cfg.Port),
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		log.Info("starting currency conversion service", zap.String("port", cfg.Port))
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

func setupRouter(handler *handler.CurrencyHandler, log *zap.Logger) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()

	router.Use(middleware.RequestID())
	router.Use(middleware.Logger(log))
	router.Use(middleware.Recovery(log))
	router.Use(middleware.CORS())

	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "healthy"})
	})
	router.GET("/ready", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ready"})
	})

	router.GET("/metrics", gin.WrapH(promhttp.Handler()))

	v1 := router.Group("/api/v1")
	{
		currency := v1.Group("/currency")
		{
			currency.POST("/convert", handler.ConvertCurrency)
			currency.GET("/rates/:from/:to", handler.GetRate)
			currency.GET("/rates/history/:from/:to", handler.GetRateHistory)
			currency.GET("/supported", handler.GetSupportedCurrencies)
		}
	}

	return router
}

type Config struct {
	Port            string
	DatabaseURL     string
	RedisURL        string
	ExchangeAPIKey  string
	Environment     string
}

func loadConfig() *Config {
	return &Config{
		Port:            getEnv("PORT", "8081"),
		DatabaseURL:     getEnv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/globalpay?sslmode=disable"),
		RedisURL:        getEnv("REDIS_URL", "localhost:6379"),
		ExchangeAPIKey:  getEnv("EXCHANGE_RATE_API_KEY", ""),
		Environment:     getEnv("ENVIRONMENT", "development"),
	}
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}