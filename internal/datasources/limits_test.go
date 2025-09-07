package datasources

import (
	"net/http"
	"testing"
	"time"
)

func TestProviderManager_CanMakeRequest(t *testing.T) {
	pm := NewProviderManager()

	// Should be able to make requests initially
	if !pm.CanMakeRequest("binance") {
		t.Error("Should be able to make request to binance initially")
	}

	// Unknown provider should return false
	if pm.CanMakeRequest("unknown") {
		t.Error("Should not be able to make request to unknown provider")
	}
}

func TestProviderManager_RecordRequest(t *testing.T) {
	pm := NewProviderManager()

	// Should be able to record requests
	err := pm.RecordRequest("binance", 1)
	if err != nil {
		t.Errorf("Should be able to record request: %v", err)
	}

	// Unknown provider should return error
	err = pm.RecordRequest("unknown", 1)
	if err == nil {
		t.Error("Should return error for unknown provider")
	}
}

func TestProviderManager_UsageStats(t *testing.T) {
	pm := NewProviderManager()

	// Record some requests
	pm.RecordRequest("binance", 1)
	pm.RecordRequest("binance", 2)

	stats, err := pm.GetUsageStats("binance")
	if err != nil {
		t.Fatalf("Failed to get usage stats: %v", err)
	}

	if stats.RequestsToday != 2 {
		t.Errorf("Expected 2 requests today, got %d", stats.RequestsToday)
	}

	if stats.WeightUsed != 3 {
		t.Errorf("Expected weight used 3, got %d", stats.WeightUsed)
	}
}

func TestRateLimiter_TokenBucket(t *testing.T) {
	pm := NewProviderManager()

	// Exhaust tokens for binance (burst limit = 40)
	for i := 0; i < 40; i++ {
		if !pm.CanMakeRequest("binance") {
			t.Fatalf("Should be able to make request %d", i+1)
		}
		err := pm.RecordRequest("binance", 1)
		if err != nil {
			t.Fatalf("Failed to record request %d: %v", i+1, err)
		}
	}

	// Should not be able to make more requests
	if pm.CanMakeRequest("binance") {
		t.Error("Should not be able to make request after exhausting tokens")
	}
}

func TestRateLimiter_TokenRefill(t *testing.T) {
	pm := NewProviderManager()

	// Get binance limiter and exhaust tokens
	binanceLimiter := pm.limiters["binance"]
	binanceLimiter.mu.Lock()
	binanceLimiter.tokens = 0
	binanceLimiter.lastRefill = time.Now().Add(-2 * time.Second) // 2 seconds ago
	binanceLimiter.mu.Unlock()

	// Should be able to make requests again after time passes
	if !pm.CanMakeRequest("binance") {
		t.Error("Should be able to make request after token refill")
	}
}

func TestRateLimiter_BinanceWeightHeaders(t *testing.T) {
	pm := NewProviderManager()

	// Create mock response headers
	headers := http.Header{}
	headers.Set("X-MBX-USED-WEIGHT-1M", "150")

	pm.ProcessResponseHeaders("binance", headers)

	stats, _ := pm.GetUsageStats("binance")
	if stats.WeightUsed != 150 {
		t.Errorf("Expected weight used 150, got %d", stats.WeightUsed)
	}
}

func TestRateLimiter_QuotaEnforcement(t *testing.T) {
	pm := NewProviderManager()

	// Set a low daily quota for testing
	coingeckoLimiter := pm.limiters["coingecko"]
	coingeckoLimiter.mu.Lock()
	coingeckoLimiter.provider.DailyQuota = 5
	coingeckoLimiter.mu.Unlock()

	// Make requests up to quota
	for i := 0; i < 5; i++ {
		if !pm.CanMakeRequest("coingecko") {
			t.Fatalf("Should be able to make request %d", i+1)
		}
		pm.RecordRequest("coingecko", 1)
	}

	// Should not be able to make more requests
	if pm.CanMakeRequest("coingecko") {
		t.Error("Should not be able to make request after exceeding daily quota")
	}
}

func TestUsageStats_HealthPercent(t *testing.T) {
	pm := NewProviderManager()

	// Set quotas for testing
	coingeckoLimiter := pm.limiters["coingecko"]
	coingeckoLimiter.mu.Lock()
	coingeckoLimiter.provider.DailyQuota = 100
	coingeckoLimiter.requestsToday = 50 // 50% used
	coingeckoLimiter.mu.Unlock()

	stats, _ := pm.GetUsageStats("coingecko")
	expectedHealth := 50.0 // 100 - 50% used
	if stats.HealthPercent != expectedHealth {
		t.Errorf("Expected health percent %.1f, got %.1f", expectedHealth, stats.HealthPercent)
	}
}

func TestProviderLimits_Configuration(t *testing.T) {
	// Test that all providers have valid configurations
	for name, limits := range DefaultProviders {
		if limits.Name == "" {
			t.Errorf("Provider %s has empty name", name)
		}
		if limits.RequestsPerSec <= 0 {
			t.Errorf("Provider %s has invalid RequestsPerSec: %d", name, limits.RequestsPerSec)
		}
		if limits.BurstLimit <= 0 {
			t.Errorf("Provider %s has invalid BurstLimit: %d", name, limits.BurstLimit)
		}
	}
}

func TestProviderManager_ConcurrentAccess(t *testing.T) {
	pm := NewProviderManager()

	// Test concurrent access doesn't cause panics
	done := make(chan bool)

	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < 100; j++ {
				pm.CanMakeRequest("binance")
				pm.RecordRequest("binance", 1)
				pm.GetUsageStats("binance")
			}
			done <- true
		}()
	}

	// Wait for all goroutines to complete
	for i := 0; i < 10; i++ {
		<-done
	}
}
