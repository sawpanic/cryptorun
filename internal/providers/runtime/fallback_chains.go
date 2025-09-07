package runtime

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

// FallbackConfig defines fallback chain configuration
type FallbackConfig struct {
	DataType           string        `yaml:"data_type"`
	PrimaryProvider    string        `yaml:"primary_provider"`
	Fallbacks          []string      `yaml:"fallbacks"`
	MaxRetries         int           `yaml:"max_retries"`
	RetryDelay         time.Duration `yaml:"retry_delay"`
	HealthCheckEnabled bool          `yaml:"health_check_enabled"`
}

// API fallback chains per v3.2.1
var FallbackChains = map[string]FallbackConfig{
	"price_data": {
		DataType:           "price_data",
		PrimaryProvider:    "binance",
		Fallbacks:          []string{"coingecko", "cmc", "paprika"},
		MaxRetries:         3,
		RetryDelay:         time.Second * 2,
		HealthCheckEnabled: true,
	},
	"market_data": {
		DataType:           "market_data",
		PrimaryProvider:    "coingecko",
		Fallbacks:          []string{"cmc", "paprika", "dexscreener"},
		MaxRetries:         2,
		RetryDelay:         time.Second * 3,
		HealthCheckEnabled: true,
	},
	"social_data": {
		DataType:           "social_data",
		PrimaryProvider:    "dexscreener",
		Fallbacks:          []string{"coingecko", "cmc"},
		MaxRetries:         2,
		RetryDelay:         time.Second * 5,
		HealthCheckEnabled: false, // Social data less critical
	},
	"defi_data": {
		DataType:           "defi_data",
		PrimaryProvider:    "moralis",
		Fallbacks:          []string{"dexscreener", "etherscan"},
		MaxRetries:         2,
		RetryDelay:         time.Second * 10,
		HealthCheckEnabled: true,
	},
	"ethereum_data": {
		DataType:           "ethereum_data",
		PrimaryProvider:    "etherscan",
		Fallbacks:          []string{"moralis"},
		MaxRetries:         1,
		RetryDelay:         time.Second * 15,
		HealthCheckEnabled: true,
	},
	"exchange_data": {
		DataType:           "exchange_data",
		PrimaryProvider:    "binance",
		Fallbacks:          []string{"paprika", "cmc"},
		MaxRetries:         2,
		RetryDelay:         time.Second * 3,
		HealthCheckEnabled: true,
	},
}

// FallbackManager manages API fallback chains with provider health awareness
type FallbackManager struct {
	mu              sync.RWMutex
	rateLimiters    map[string]*RateLimiter
	circuitBreakers map[string]*CircuitBreaker
	cacheManagers   map[string]*CacheManager
	fallbackStats   map[string]FallbackStats
}

// FallbackStats tracks fallback usage statistics
type FallbackStats struct {
	DataType        string           `json:"data_type"`
	TotalRequests   int64            `json:"total_requests"`
	PrimarySuccess  int64            `json:"primary_success"`
	FallbackUsed    int64            `json:"fallback_used"`
	FallbackSuccess map[string]int64 `json:"fallback_success"`
	TotalFailures   int64            `json:"total_failures"`
	AvgLatency      time.Duration    `json:"avg_latency"`
}

// NewFallbackManager creates a new fallback manager
func NewFallbackManager() *FallbackManager {
	fm := &FallbackManager{
		rateLimiters:    make(map[string]*RateLimiter),
		circuitBreakers: make(map[string]*CircuitBreaker),
		cacheManagers:   make(map[string]*CacheManager),
		fallbackStats:   make(map[string]FallbackStats),
	}

	// Initialize providers
	providers := []string{"binance", "dexscreener", "coingecko", "moralis", "cmc", "etherscan", "paprika"}
	for _, provider := range providers {
		fm.rateLimiters[provider] = NewRateLimiter(provider)
		fm.circuitBreakers[provider] = NewCircuitBreaker(provider)
		fm.cacheManagers[provider] = NewCacheManager(provider)
	}

	// Initialize stats
	for dataType := range FallbackChains {
		fm.fallbackStats[dataType] = FallbackStats{
			DataType:        dataType,
			FallbackSuccess: make(map[string]int64),
		}
	}

	return fm
}

// FetchWithFallback attempts to fetch data with fallback chain
func (fm *FallbackManager) FetchWithFallback(
	ctx context.Context,
	dataType, key string,
	fetchFn func(provider string) ([]byte, error),
) ([]byte, error) {

	config, exists := FallbackChains[dataType]
	if !exists {
		return nil, fmt.Errorf("no fallback chain configured for data type: %s", dataType)
	}

	startTime := time.Now()

	fm.mu.Lock()
	stats := fm.fallbackStats[dataType]
	stats.TotalRequests++
	fm.fallbackStats[dataType] = stats
	fm.mu.Unlock()

	// Try cache first
	if cacheData := fm.tryCache(config.PrimaryProvider, key); cacheData != nil {
		fm.updateLatencyStats(dataType, time.Since(startTime))
		return cacheData, nil
	}

	// Try primary provider
	if data, err := fm.tryProvider(ctx, config.PrimaryProvider, key, fetchFn); err == nil {
		fm.recordSuccess(dataType, config.PrimaryProvider, true)
		fm.cacheResult(config.PrimaryProvider, key, data, TierWarm)
		fm.updateLatencyStats(dataType, time.Since(startTime))
		return data, nil
	} else {
		log.Warn().
			Str("provider", config.PrimaryProvider).
			Str("data_type", dataType).
			Err(err).
			Msg("Primary provider failed, trying fallbacks")
	}

	// Try fallback providers
	for _, fallbackProvider := range config.Fallbacks {
		// Check cache for fallback provider
		if cacheData := fm.tryCache(fallbackProvider, key); cacheData != nil {
			fm.recordSuccess(dataType, fallbackProvider, false)
			fm.updateLatencyStats(dataType, time.Since(startTime))
			return cacheData, nil
		}

		// Try fallback provider
		if data, err := fm.tryProvider(ctx, fallbackProvider, key, fetchFn); err == nil {
			fm.recordSuccess(dataType, fallbackProvider, false)
			fm.cacheResult(fallbackProvider, key, data, TierWarm)
			fm.updateLatencyStats(dataType, time.Since(startTime))

			log.Info().
				Str("fallback_provider", fallbackProvider).
				Str("data_type", dataType).
				Msg("Fallback provider succeeded")
			return data, nil
		} else {
			log.Warn().
				Str("provider", fallbackProvider).
				Str("data_type", dataType).
				Err(err).
				Msg("Fallback provider failed")
		}

		// Add retry delay between fallback attempts
		time.Sleep(config.RetryDelay)
	}

	// All providers failed
	fm.recordFailure(dataType)
	return nil, fmt.Errorf("all providers failed for data type: %s", dataType)
}

// tryProvider attempts to fetch from a specific provider with circuit breaker and rate limiting
func (fm *FallbackManager) tryProvider(
	ctx context.Context,
	provider, key string,
	fetchFn func(provider string) ([]byte, error),
) ([]byte, error) {

	// Check rate limits
	if rl := fm.rateLimiters[provider]; rl != nil {
		if err := rl.CheckLimit(ctx); err != nil {
			// Set cache to degraded mode
			if cm := fm.cacheManagers[provider]; cm != nil {
				cm.SetDegraded(true)
			}
			return nil, fmt.Errorf("rate limited: %w", err)
		}
	}

	// Use circuit breaker
	cb := fm.circuitBreakers[provider]
	if cb == nil {
		return nil, fmt.Errorf("no circuit breaker for provider: %s", provider)
	}

	var result []byte
	var fetchErr error

	err := cb.Call(ctx, func() error {
		result, fetchErr = fetchFn(provider)
		return fetchErr
	})

	if err != nil {
		// Circuit breaker error
		return nil, err
	}

	if fetchErr != nil {
		// Handle rate limit responses
		if rl := fm.rateLimiters[provider]; rl != nil && isRateLimitError(fetchErr) {
			rl.HandleRateLimit(429, nil) // Simplified header handling
		}
		return nil, fetchErr
	}

	// Record successful request
	if rl := fm.rateLimiters[provider]; rl != nil {
		rl.RecordRequest(1) // Simplified weight

		// Restore normal cache mode on success
		if cm := fm.cacheManagers[provider]; cm != nil {
			cm.SetDegraded(false)
		}
	}

	return result, nil
}

// tryCache attempts to get data from cache
func (fm *FallbackManager) tryCache(provider, key string) []byte {
	cm := fm.cacheManagers[provider]
	if cm == nil {
		return nil
	}

	cacheKey := cm.BuildKey("default", key)
	if data, found := cm.Get(cacheKey); found {
		log.Debug().
			Str("provider", provider).
			Str("key", key).
			Msg("Cache hit")
		return data
	}

	return nil
}

// cacheResult stores successful result in cache
func (fm *FallbackManager) cacheResult(provider, key string, data []byte, tier CacheTier) {
	cm := fm.cacheManagers[provider]
	if cm == nil {
		return
	}

	cacheKey := cm.BuildKey("default", key)
	cm.Set(cacheKey, data, tier)
}

// GetProviderHealth returns health status of all providers
func (fm *FallbackManager) GetProviderHealth() map[string]interface{} {
	fm.mu.RLock()
	defer fm.mu.RUnlock()

	health := make(map[string]interface{})

	for provider, cb := range fm.circuitBreakers {
		rl := fm.rateLimiters[provider]
		cm := fm.cacheManagers[provider]

		providerHealth := map[string]interface{}{
			"circuit_breaker": cb.GetStatus(),
			"rate_limiter":    rl.GetStatus(),
			"cache":           cm.GetStats(),
			"healthy":         cb.IsHealthy() && !isRateLimited(rl),
		}

		health[provider] = providerHealth
	}

	return health
}

// GetFallbackStats returns fallback usage statistics
func (fm *FallbackManager) GetFallbackStats() map[string]FallbackStats {
	fm.mu.RLock()
	defer fm.mu.RUnlock()

	// Return copy of stats
	stats := make(map[string]FallbackStats)
	for k, v := range fm.fallbackStats {
		stats[k] = v
	}
	return stats
}

// RunHealthChecks performs health checks on all providers
func (fm *FallbackManager) RunHealthChecks(ctx context.Context) {
	for provider, cb := range fm.circuitBreakers {
		cb.RunHealthCheck(ctx, func() error {
			// Simple health check - could be expanded
			return fm.performHealthCheck(ctx, provider)
		})
	}
}

// Helper methods
func (fm *FallbackManager) recordSuccess(dataType, provider string, isPrimary bool) {
	fm.mu.Lock()
	defer fm.mu.Unlock()

	stats := fm.fallbackStats[dataType]
	if isPrimary {
		stats.PrimarySuccess++
	} else {
		stats.FallbackUsed++
		if stats.FallbackSuccess == nil {
			stats.FallbackSuccess = make(map[string]int64)
		}
		stats.FallbackSuccess[provider]++
	}
	fm.fallbackStats[dataType] = stats
}

func (fm *FallbackManager) recordFailure(dataType string) {
	fm.mu.Lock()
	defer fm.mu.Unlock()

	stats := fm.fallbackStats[dataType]
	stats.TotalFailures++
	fm.fallbackStats[dataType] = stats
}

func (fm *FallbackManager) updateLatencyStats(dataType string, latency time.Duration) {
	fm.mu.Lock()
	defer fm.mu.Unlock()

	stats := fm.fallbackStats[dataType]
	// Simple moving average
	if stats.AvgLatency == 0 {
		stats.AvgLatency = latency
	} else {
		stats.AvgLatency = (stats.AvgLatency + latency) / 2
	}
	fm.fallbackStats[dataType] = stats
}

func (fm *FallbackManager) performHealthCheck(ctx context.Context, provider string) error {
	// Placeholder health check implementation
	// In real implementation, this would make a simple API call
	return nil
}

func isRateLimitError(err error) bool {
	// Check if error indicates rate limiting
	errStr := err.Error()
	return containsAny(errStr, []string{"429", "rate limit", "too many requests", "quota exceeded"})
}

func isRateLimited(rl *RateLimiter) bool {
	if rl == nil {
		return false
	}
	status := rl.GetStatus()
	throttled, ok := status["is_throttled"].(bool)
	return ok && throttled
}

func containsAny(str string, substrs []string) bool {
	for _, substr := range substrs {
		if len(str) >= len(substr) {
			for i := 0; i <= len(str)-len(substr); i++ {
				if str[i:i+len(substr)] == substr {
					return true
				}
			}
		}
	}
	return false
}
