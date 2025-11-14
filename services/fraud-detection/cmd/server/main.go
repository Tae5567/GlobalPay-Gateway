// services/fraud-detection/cmd/server/main.go
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

	"fraud-detection/internal/handler"
	"fraud-detection/internal/repository"
	"fraud-detection/internal/service"
	"shared/pkg/database"
	"shared/pkg/logger"
	"shared/pkg/middleware"
)

func main() {
	log := logger.NewLogger("fraud-detection")
	defer log.Sync()

	cfg := loadConfig()

	// Initialize database
	db, err := database.NewPostgresDB(cfg.DatabaseURL)
	if err != nil {
		log.Fatal("failed to connect to database", zap.Error(err))
	}

	// Initialize repositories
	fraudRepo := repository.NewFraudRepository(db)

	// Initialize services
	fraudEngine := service.NewFraudEngine(fraudRepo, log)

	// Initialize handlers
	fraudHandler := handler.NewFraudHandler(fraudEngine, log)

	// Setup router
	router := setupRouter(fraudHandler, log)

	srv := &http.Server{
		Addr:         fmt.Sprintf(":%s", cfg.Port),
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		log.Info("starting fraud detection service", zap.String("port", cfg.Port))
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

func setupRouter(handler *handler.FraudHandler, log *zap.Logger) *gin.Engine {
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
		fraud := v1.Group("/fraud")
		{
			fraud.POST("/check", handler.CheckFraud)
			fraud.GET("/results/:transaction_id", handler.GetFraudResult)
			fraud.GET("/stats", handler.GetFraudStats)
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
		Port:        getEnv("PORT", "8082"),
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