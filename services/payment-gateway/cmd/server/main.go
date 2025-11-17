// services/payment-gateway/cmd/server/main.go
// HTTP Server
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

	"payment-gateway/internal/handler"
	"payment-gateway/internal/repository"
	"payment-gateway/internal/service"
	"shared/pkg/database"
	"shared/pkg/logger"
	"shared/pkg/middleware"
	"shared/pkg/redis"
	"shared/pkg/tracing"
)

func main() {
	// Initialize logger
	log := logger.NewLogger("payment-gateway")
	defer log.Sync()

	// Load configuration
	cfg := loadConfig()

	// Initialize tracing
	shutdown, err := tracing.InitTracer("payment-gateway", cfg.JaegerEndpoint)
	if err != nil {
		log.Fatal("failed to initialize tracer", zap.Error(err))
	}
	defer shutdown(context.Background())

	// Initialize database
	db, err := database.NewPostgresDB(cfg.DatabaseURL)
	if err != nil {
		log.Fatal("failed to connect to database", zap.Error(err))
	}

	// Initialize Redis
	redisClient := redis.NewRedisClient(cfg.RedisURL)

	// Initialize repositories
	paymentRepo := repository.NewPaymentRepository(db)

	// Initialize services
	paymentService := service.NewPaymentService(paymentRepo, redisClient, cfg)

	// Initialize handlers
	paymentHandler := handler.NewPaymentHandler(paymentService, log)

	// Setup router
	router := setupRouter(paymentHandler, log)

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
		log.Info("starting server", zap.String("port", cfg.Port))
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

func setupRouter(handler *handler.PaymentHandler, log *zap.Logger) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()

	// Middleware
	router.Use(middleware.RequestID())
	router.Use(middleware.Logger(log))
	router.Use(middleware.Recovery(log))
	router.Use(middleware.CORS())
	router.Use(middleware.RateLimiter())

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
		payments := v1.Group("/payments")
		{
			payments.POST("", handler.CreatePayment)
			payments.GET("/:id", handler.GetPayment)
			payments.POST("/:id/confirm", handler.ConfirmPayment)
			payments.POST("/:id/cancel", handler.CancelPayment)
			payments.GET("", handler.ListPayments)
		}

		// Webhook for Stripe
		v1.POST("/webhooks/stripe", handler.StripeWebhook)
	}

	return router
}

type Config struct {
	Port           string
	DatabaseURL    string
	RedisURL       string
	JaegerEndpoint string
	StripeKey      string
	Environment    string
}

func loadConfig() *Config {
	return &Config{
		Port:           getEnv("PORT", "8080"),
		DatabaseURL:    getEnv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/globalpay?sslmode=disable"),
		RedisURL:       getEnv("REDIS_URL", "localhost:6379"),
		JaegerEndpoint: getEnv("JAEGER_ENDPOINT", "http://localhost:14268/api/traces"),
		StripeKey:      getEnv("STRIPE_SECRET_KEY", ""),
		Environment:    getEnv("ENVIRONMENT", "development"),
	}
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}