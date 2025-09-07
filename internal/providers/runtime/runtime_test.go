package runtime

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRateLimiter_CheckLimit(t *testing.T) {
	tests := []struct {
		name           string
		provider       string
		requests       int
		expectError    bool
		errorContains  string
	}{
		{
			name:        "binance_normal_load",
			provider:    "binance",
			requests:    10,
			expectError: false,
		},
		{
			name:          "binance_rate_limit_exceeded",
			provider:      "binance", 
			requests:      1300, // Exceeds 1200/min limit
			expectError:   true,
			errorContains: "rate limited",
		},
		{
			name:        "coingecko_free_tier",
			provider:    "coingecko",
			requests:    5,
			expectError: false,
		},
		{
			name:          "coingecko_quota_exceeded",
			provider:      "coingecko",
			requests:      60, // Exceeds 50/min free tier
			expectError:   true,
			errorContains: "rate limited",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rl := NewRateLimiter(tt.provider)
			ctx := context.Background()

			var lastErr error
			for i := 0; i < tt.requests; i++ {
				err := rl.CheckLimit(ctx)
				if err != nil {
					lastErr = err
					break
				}
				rl.RecordRequest(1)
			}

			if tt.expectError {
				require.Error(t, lastErr)
				assert.Contains(t, lastErr.Error(), tt.errorContains)
			} else {
				assert.NoError(t, lastErr)
			}
		})
	}
}

func TestRateLimiter_BackoffBehavior(t *testing.T) {
	rl := NewRateLimiter("binance")
	
	// Simulate rate limit response
	headers := map[string]string{
		"Retry-After": "5",
		"X-MBX-USED-WEIGHT": "1200",
	}
	
	rl.HandleRateLimit(429, headers)
	
	// Check that we're in backoff
	ctx := context.Background()
	err := rl.CheckLimit(ctx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "backoff until")
	
	status := rl.GetStatus()
	assert.True(t, status["is_throttled"].(bool))
	assert.Equal(t, 1, status["backoff_attempts"])
}

func TestCircuitBreaker_StateTransitions(t *testing.T) {
	cb := NewCircuitBreaker("binance")
	ctx := context.Background()
	
	// Initial state should be closed
	assert.True(t, cb.IsHealthy())
	status := cb.GetStatus()
	assert.Equal(t, "closed", status["state"])
	
	// Simulate failures to trip circuit
	failingFn := func() error {
		return fmt.Errorf("simulated API failure")
	}
	
	// Trip the circuit with failures
	for i := 0; i < 5; i++ { // Binance threshold is 5
		err := cb.Call(ctx, failingFn)
		assert.Error(t, err)
	}
	
	// Circuit should now be open
	assert.False(t, cb.IsHealthy())
	status = cb.GetStatus()
	assert.Equal(t, "open", status["state"])
	
	// Calls should fail immediately
	err := cb.Call(ctx, func() error { return nil })
	require.Error(t, err)
	assert.Contains(t, err.Error(), "circuit breaker open")
}

func TestCircuitBreaker_RecoveryFlow(t *testing.T) {
	cb := NewCircuitBreaker("coingecko")
	ctx := context.Background()
	
	// Trip circuit
	for i := 0; i < 4; i++ { // CoinGecko threshold is 4
		cb.Call(ctx, func() error { return fmt.Errorf("failure") })
	}
	
	// Force to half-open by manipulating time
	cb.mu.Lock()
	cb.nextRetryTime = time.Now().Add(-time.Second) // Past retry time
	cb.mu.Unlock()
	
	// Successful calls should close circuit
	successFn := func() error { return nil }
	
	for i := 0; i < 2; i++ { // CoinGecko success threshold is 2
		err := cb.Call(ctx, successFn)
		assert.NoError(t, err)
	}
	
	// Circuit should be closed now
	assert.True(t, cb.IsHealthy())
	status := cb.GetStatus()
	assert.Equal(t, "closed", status["state"])
}

func TestCacheManager_TierBehavior(t *testing.T) {
	cm := NewCacheManager("binance")
	
	testData := []byte("test data")
	key := "test_key"
	
	// Test different cache tiers
	tiers := []CacheTier{TierHot, TierWarm, TierCold}
	
	for _, tier := range tiers {
		cacheKey := cm.BuildKey("default", fmt.Sprintf("%s_%s", key, tier.String()))
		
		// Set with tier
		cm.Set(cacheKey, testData, tier)
		
		// Should be retrievable
		data, found := cm.Get(cacheKey)
		assert.True(t, found)
		assert.Equal(t, testData, data)
	}
	
	stats := cm.GetStats()
	assert.Equal(t, 3, stats["entries"])
	assert.Greater(t, stats["hit_rate"].(float64), 0.0)
}

func TestCacheManager_DegradationMode(t *testing.T) {
	cm := NewCacheManager("binance")
	
	// Normal mode
	testData := []byte("test data")
	key := cm.BuildKey("default", "test")
	
	cm.Set(key, testData, TierWarm)
	
	// Enable degraded mode
	cm.SetDegraded(true)
	
	// New entries should use extended TTL
	degradedData := []byte("degraded data")
	degradedKey := cm.BuildKey("default", "degraded")
	
	cm.Set(degradedKey, degradedData, TierWarm)
	
	stats := cm.GetStats()
	assert.True(t, stats["degraded"].(bool))
	assert.Equal(t, 2, stats["entries"])
}

func TestFallbackManager_ProviderFallback(t *testing.T) {
	fm := NewFallbackManager()
	ctx := context.Background()
	
	callCount := make(map[string]int)
	
	// Mock fetch function that fails for primary but succeeds for fallback
	fetchFn := func(provider string) ([]byte, error) {
		callCount[provider]++
		
		if provider == "binance" {
			return nil, fmt.Errorf("binance API unavailable")
		}
		
		if provider == "coingecko" {
			return []byte(fmt.Sprintf("data from %s", provider)), nil
		}
		
		return nil, fmt.Errorf("provider %s failed", provider)
	}
	
	// Try to fetch price data (primary: binance, fallbacks: coingecko, cmc, paprika)
	data, err := fm.FetchWithFallback(ctx, "price_data", "BTCUSD", fetchFn)
	
	require.NoError(t, err)
	assert.Equal(t, []byte("data from coingecko"), data)
	
	// Verify call pattern
	assert.Equal(t, 1, callCount["binance"])    // Primary failed
	assert.Equal(t, 1, callCount["coingecko"])  // First fallback succeeded
	assert.Equal(t, 0, callCount["cmc"])        // Not called
	assert.Equal(t, 0, callCount["paprika"])    // Not called
}

func TestFallbackManager_AllProvidersFail(t *testing.T) {
	fm := NewFallbackManager()
	ctx := context.Background()
	
	// Mock fetch function that always fails
	fetchFn := func(provider string) ([]byte, error) {
		return nil, fmt.Errorf("provider %s unavailable", provider)
	}
	
	// All providers should fail
	data, err := fm.FetchWithFallback(ctx, "price_data", "BTCUSD", fetchFn)
	
	require.Error(t, err)
	assert.Nil(t, data)
	assert.Contains(t, err.Error(), "all providers failed")
	
	// Check stats
	stats := fm.GetFallbackStats()
	priceStats := stats["price_data"]
	assert.Equal(t, int64(1), priceStats.TotalRequests)
	assert.Equal(t, int64(1), priceStats.TotalFailures)
}

func TestFallbackManager_CacheHit(t *testing.T) {
	fm := NewFallbackManager()
	ctx := context.Background()
	
	// Pre-populate cache
	binanceCache := fm.cacheManagers["binance"]
	cacheKey := binanceCache.BuildKey("default", "BTCUSD")
	cachedData := []byte("cached price data")
	binanceCache.Set(cacheKey, cachedData, TierWarm)
	
	callCount := 0
	fetchFn := func(provider string) ([]byte, error) {
		callCount++
		return nil, fmt.Errorf("should not be called")
	}
	
	// Should return cached data without calling any provider
	data, err := fm.FetchWithFallback(ctx, "price_data", "BTCUSD", fetchFn)
	
	require.NoError(t, err)
	assert.Equal(t, cachedData, data)
	assert.Equal(t, 0, callCount) // No provider calls should be made
}

func TestProviderLimits_Configurations(t *testing.T) {
	// Test that all providers have valid configurations
	providers := []string{"binance", "dexscreener", "coingecko", "moralis", "cmc", "etherscan", "paprika"}
	
	for _, provider := range providers {
		t.Run(provider, func(t *testing.T) {
			config, exists := ProviderLimits[provider]
			require.True(t, exists)
			
			assert.Equal(t, provider, config.Provider)
			assert.Greater(t, config.RequestsPerMin, 0)
			assert.Greater(t, config.RequestsPerHour, 0)
			assert.Greater(t, config.DailyRequests, 0)
			assert.Greater(t, config.MonthlyBudget, 0)
			assert.Greater(t, config.BurstSize, 0)
			assert.Greater(t, config.BackoffBase, time.Duration(0))
			assert.Greater(t, config.BackoffMax, config.BackoffBase)
		})
	}
}

func TestFallbackChains_Configurations(t *testing.T) {
	// Test that all fallback chains have valid configurations
	dataTypes := []string{"price_data", "market_data", "social_data", "defi_data", "ethereum_data", "exchange_data"}
	
	for _, dataType := range dataTypes {
		t.Run(dataType, func(t *testing.T) {
			config, exists := FallbackChains[dataType]
			require.True(t, exists)
			
			assert.Equal(t, dataType, config.DataType)
			assert.NotEmpty(t, config.PrimaryProvider)
			assert.Greater(t, len(config.Fallbacks), 0)
			assert.Greater(t, config.MaxRetries, 0)
			assert.Greater(t, config.RetryDelay, time.Duration(0))
			
			// Verify primary provider exists in ProviderLimits
			_, exists = ProviderLimits[config.PrimaryProvider]
			assert.True(t, exists, "Primary provider %s not found in ProviderLimits", config.PrimaryProvider)
			
			// Verify all fallback providers exist
			for _, fallback := range config.Fallbacks {
				_, exists = ProviderLimits[fallback]
				assert.True(t, exists, "Fallback provider %s not found in ProviderLimits", fallback)
			}
		})
	}
}

// Benchmark tests
func BenchmarkRateLimiter_CheckLimit(b *testing.B) {
	rl := NewRateLimiter("binance")
	ctx := context.Background()
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = rl.CheckLimit(ctx)
		rl.RecordRequest(1)
	}
}

func BenchmarkCircuitBreaker_Call(b *testing.B) {
	cb := NewCircuitBreaker("binance")
	ctx := context.Background()
	
	successFn := func() error { return nil }
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = cb.Call(ctx, successFn)
	}
}

func BenchmarkCacheManager_GetSet(b *testing.B) {
	cm := NewCacheManager("binance")
	testData := []byte("benchmark test data")
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := cm.BuildKey("default", fmt.Sprintf("key_%d", i))
		cm.Set(key, testData, TierWarm)
		_, _ = cm.Get(key)
	}
}