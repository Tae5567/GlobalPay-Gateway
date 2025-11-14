// services/currency-conversion/internal/service/rate_cache.go
package service

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"

	"currency-conversion/internal/models"
	"shared/pkg/redis"
)

// RateCache manages exchange rate caching with multiple layers
type RateCache struct {
	redis      *redis.Client
	logger     *zap.Logger
	memCache   *MemoryCache
	ttl        time.Duration
}

// MemoryCache provides in-memory caching for ultra-fast lookups
type MemoryCache struct {
	mu     sync.RWMutex
	data   map[string]*CacheEntry
	maxAge time.Duration
}

// CacheEntry represents a cached rate with timestamp
type CacheEntry struct {
	Rate      *models.ExchangeRate
	CachedAt  time.Time
}

// NewRateCache creates a new rate cache instance
func NewRateCache(redisClient *redis.Client, logger *zap.Logger) *RateCache {
	return &RateCache{
		redis:    redisClient,
		logger:   logger,
		memCache: NewMemoryCache(5 * time.Minute),
		ttl:      5 * time.Minute,
	}
}

// NewMemoryCache creates a new in-memory cache
func NewMemoryCache(maxAge time.Duration) *MemoryCache {
	cache := &MemoryCache{
		data:   make(map[string]*CacheEntry),
		maxAge: maxAge,
	}
	
	// Start cleanup goroutine
	go cache.cleanup()
	
	return cache
}

// Get retrieves a rate from cache (checks memory first, then Redis)
func (rc *RateCache) Get(ctx context.Context, from, to string) (*models.ExchangeRate, error) {
	key := rc.cacheKey(from, to)

	// Try memory cache first (fastest)
	if rate := rc.memCache.Get(key); rate != nil {
		rc.logger.Debug("cache hit (memory)", 
			zap.String("from", from), 
			zap.String("to", to))
		return rate, nil
	}

	// Try Redis cache (fast)
	data, err := rc.redis.Get(ctx, key)
	if err == nil {
		var rate models.ExchangeRate
		if err := json.Unmarshal([]byte(data), &rate); err == nil {
			rc.logger.Debug("cache hit (redis)", 
				zap.String("from", from), 
				zap.String("to", to))
			
			// Store in memory cache for next time
			rc.memCache.Set(key, &rate)
			return &rate, nil
		}
	}

	// Cache miss
	rc.logger.Debug("cache miss", 
		zap.String("from", from), 
		zap.String("to", to))
	return nil, fmt.Errorf("cache miss")
}

// Set stores a rate in both memory and Redis cache
func (rc *RateCache) Set(ctx context.Context, from, to string, rate *models.ExchangeRate) error {
	key := rc.cacheKey(from, to)

	// Store in memory cache
	rc.memCache.Set(key, rate)

	// Store in Redis
	data, err := json.Marshal(rate)
	if err != nil {
		return fmt.Errorf("failed to marshal rate: %w", err)
	}

	if err := rc.redis.Set(ctx, key, data, rc.ttl); err != nil {
		rc.logger.Error("failed to cache rate in redis", 
			zap.Error(err),
			zap.String("key", key))
		return err
	}

	rc.logger.Debug("rate cached", 
		zap.String("from", from), 
		zap.String("to", to),
		zap.Float64("rate", rate.Rate))

	return nil
}

// Delete removes a rate from cache
func (rc *RateCache) Delete(ctx context.Context, from, to string) error {
	key := rc.cacheKey(from, to)
	
	// Remove from memory cache
	rc.memCache.Delete(key)
	
	// Remove from Redis
	return rc.redis.Delete(ctx, key)
}

// Invalidate removes all cached rates for a currency
func (rc *RateCache) Invalidate(ctx context.Context, currency string) error {
	// This is a simplified implementation
	// In production, use Redis SCAN to find and delete all keys with pattern
	rc.logger.Info("invalidating cache for currency", zap.String("currency", currency))
	
	// Clear memory cache entries containing this currency
	rc.memCache.mu.Lock()
	defer rc.memCache.mu.Unlock()
	
	for key := range rc.memCache.data {
		if containsCurrency(key, currency) {
			delete(rc.memCache.data, key)
		}
	}
	
	return nil
}

// GetStats returns cache statistics
func (rc *RateCache) GetStats() map[string]interface{} {
	rc.memCache.mu.RLock()
	defer rc.memCache.mu.RUnlock()

	return map[string]interface{}{
		"memory_cache_size": len(rc.memCache.data),
		"memory_cache_ttl":  rc.memCache.maxAge.String(),
		"redis_ttl":         rc.ttl.String(),
	}
}

// cacheKey generates a cache key for a currency pair
func (rc *RateCache) cacheKey(from, to string) string {
	return fmt.Sprintf("rate:%s:%s", from, to)
}

// MemoryCache methods

// Get retrieves from memory cache
func (mc *MemoryCache) Get(key string) *models.ExchangeRate {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	entry, exists := mc.data[key]
	if !exists {
		return nil
	}

	// Check if entry is still valid
	if time.Since(entry.CachedAt) > mc.maxAge {
		return nil
	}

	return entry.Rate
}

// Set stores in memory cache
func (mc *MemoryCache) Set(key string, rate *models.ExchangeRate) {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	mc.data[key] = &CacheEntry{
		Rate:     rate,
		CachedAt: time.Now(),
	}
}

// Delete removes from memory cache
func (mc *MemoryCache) Delete(key string) {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	delete(mc.data, key)
}

// cleanup periodically removes expired entries
func (mc *MemoryCache) cleanup() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		mc.mu.Lock()
		now := time.Now()
		for key, entry := range mc.data {
			if now.Sub(entry.CachedAt) > mc.maxAge {
				delete(mc.data, key)
			}
		}
		mc.mu.Unlock()
	}
}

// Helper functions

func containsCurrency(key, currency string) bool {
	// Simple check if currency is in the key
	// Keys are in format "rate:USD:EUR"
	return len(key) > len(currency) && 
		(key[5:5+len(currency)] == currency || key[len(key)-len(currency):] == currency)
}

// WarmupCache pre-loads common currency pairs
func (rc *RateCache) WarmupCache(ctx context.Context, pairs []struct{ From, To string }, fetchFunc func(string, string) (*models.ExchangeRate, error)) error {
	rc.logger.Info("warming up cache", zap.Int("pairs", len(pairs)))

	for _, pair := range pairs {
		rate, err := fetchFunc(pair.From, pair.To)
		if err != nil {
			rc.logger.Warn("failed to warm up cache for pair",
				zap.String("from", pair.From),
				zap.String("to", pair.To),
				zap.Error(err))
			continue
		}

		if err := rc.Set(ctx, pair.From, pair.To, rate); err != nil {
			rc.logger.Error("failed to cache rate during warmup",
				zap.Error(err))
		}
	}

	rc.logger.Info("cache warmup complete")
	return nil
}