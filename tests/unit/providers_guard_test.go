package unit

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/sawpanic/cryptorun/internal/infrastructure/httpclient"
	"github.com/sawpanic/cryptorun/internal/infrastructure/providers"
)

func TestCoinGeckoProvider_BudgetEnforcement(t *testing.T) {
	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`[{"id": "bitcoin", "symbol": "btc", "name": "Bitcoin"}]`))
	}))
	defer server.Close()

	config := providers.CoinGeckoConfig{
		BaseURL:        server.URL,
		RPMLimit:       2, // Very low limit for testing
		MonthlyLimit:   10,
		RequestTimeout: 5 * time.Second,
		MaxRetries:     1,
		TTL:            300 * time.Second,
	}

	provider := providers.NewCoinGeckoProvider(config)

	ctx := context.Background()

	// First request should succeed
	_, err := provider.GetCoinsList(ctx)
	if err != nil {
		t.Fatalf("First request should succeed: %v", err)
	}

	// Second request should succeed
	_, err = provider.GetCoinsList(ctx)
	if err != nil {
		t.Fatalf("Second request should succeed: %v", err)
	}

	// Third request should fail due to RPM limit
	_, err = provider.GetCoinsList(ctx)
	if err == nil {
		t.Error("Third request should fail due to RPM budget")
	}

	if !strings.Contains(err.Error(), "PROVIDER_DEGRADED") {
		t.Errorf("Error should contain PROVIDER_DEGRADED, got: %v", err)
	}

	if !strings.Contains(err.Error(), "budget_exceeded") {
		t.Errorf("Error should contain budget_exceeded, got: %v", err)
	}
}

func TestCoinGeckoProvider_RateLimitHandling(t *testing.T) {
	// Create mock server that returns 429
	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		if requestCount == 1 {
			w.Header().Set("Retry-After", "60")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`[]`))
	}))
	defer server.Close()

	config := providers.CoinGeckoConfig{
		BaseURL:        server.URL,
		RPMLimit:       100,
		MonthlyLimit:   1000,
		RequestTimeout: 5 * time.Second,
		MaxRetries:     1,
		TTL:            300 * time.Second,
	}

	provider := providers.NewCoinGeckoProvider(config)

	ctx := context.Background()

	// Request should fail with rate limit error
	_, err := provider.GetCoinsMarkets(ctx, "usd", 1, 50)
	if err == nil {
		t.Error("Request should fail due to rate limit")
	}

	if !strings.Contains(err.Error(), "rate_limited") {
		t.Errorf("Error should contain rate_limited, got: %v", err)
	}
}

func TestKrakenProvider_ExchangeNativeEnforcement(t *testing.T) {
	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		if strings.Contains(r.URL.Path, "AssetPairs") {
			w.Write([]byte(`{"error":[], "result":{"XBTUSD":{"altname":"XBTUSD","base":"XXBT","quote":"ZUSD","pair_decimals":1,"lot_decimals":8}}}`))
		} else if strings.Contains(r.URL.Path, "Depth") {
			w.Write([]byte(`{"error":[], "result":{"XBTUSD":{"bids":[["50000.0","1.0",1234567890]],"asks":[["50001.0","1.0",1234567890]]}}}`))
		} else if strings.Contains(r.URL.Path, "Ticker") {
			w.Write([]byte(`{"error":[], "result":{"XBTUSD":{"v":["100.0","200.0"],"p":["50000.0","49999.0"]}}}`))
		}
	}))
	defer server.Close()

	config := providers.KrakenConfig{
		BaseURL:        server.URL,
		RequestTimeout: 5 * time.Second,
		MaxRetries:     1,
		MaxConcurrency: 2,
	}

	provider := providers.NewKrakenProvider(config)

	ctx := context.Background()

	// Test getting asset pairs (should filter to USD only)
	pairs, err := provider.GetAssetPairs(ctx)
	if err != nil {
		t.Fatalf("GetAssetPairs failed: %v", err)
	}

	// Should only return USD pairs
	for name, pair := range pairs {
		if !strings.HasSuffix(pair.Quote, "USD") {
			t.Errorf("Non-USD pair returned: %s with quote %s", name, pair.Quote)
		}
	}
}

func TestHTTPClientPool_ConcurrencyLimit(t *testing.T) {
	// Create slow mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond) // Simulate slow response
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))
	defer server.Close()

	config := httpclient.ClientConfig{
		MaxConcurrency: 2, // Allow only 2 concurrent requests
		RequestTimeout: 1 * time.Second,
		JitterRange:    [2]int{10, 20},
		MaxRetries:     0,
		BackoffBase:    100 * time.Millisecond,
		BackoffMax:     1 * time.Second,
	}

	pool := httpclient.NewClientPool(config)

	// Start 4 requests concurrently
	results := make(chan error, 4)
	start := time.Now()

	for i := 0; i < 4; i++ {
		go func() {
			req, _ := http.NewRequest("GET", server.URL, nil)
			ctx := context.Background()
			_, err := pool.Do(ctx, req)
			results <- err
		}()
	}

	// Collect results
	for i := 0; i < 4; i++ {
		err := <-results
		if err != nil {
			t.Errorf("Request %d failed: %v", i, err)
		}
	}

	duration := time.Since(start)

	// With concurrency limit of 2 and 4 requests taking ~100ms each,
	// should take at least 200ms (2 batches of 2)
	if duration < 180*time.Millisecond {
		t.Errorf("Requests completed too quickly (%v), concurrency limit may not be working", duration)
	}
}

func TestHTTPClientPool_JitterApplication(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))
	defer server.Close()

	config := httpclient.ClientConfig{
		MaxConcurrency: 10,
		RequestTimeout: 1 * time.Second,
		JitterRange:    [2]int{100, 200}, // 100-200ms jitter
		MaxRetries:     0,
		BackoffBase:    100 * time.Millisecond,
		BackoffMax:     1 * time.Second,
	}

	pool := httpclient.NewClientPool(config)

	req, _ := http.NewRequest("GET", server.URL, nil)
	ctx := context.Background()

	start := time.Now()
	_, err := pool.Do(ctx, req)
	duration := time.Since(start)

	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}

	// Should have applied jitter (100-200ms) plus request time
	if duration < 90*time.Millisecond {
		t.Errorf("Request completed too quickly (%v), jitter may not be applied", duration)
	}
}

func TestHTTPClientPool_ExponentialBackoff(t *testing.T) {
	failCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		failCount++
		if failCount <= 2 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))
	defer server.Close()

	config := httpclient.ClientConfig{
		MaxConcurrency: 1,
		RequestTimeout: 10 * time.Second,
		JitterRange:    [2]int{1, 2}, // Minimal jitter for timing precision
		MaxRetries:     3,
		BackoffBase:    100 * time.Millisecond,
		BackoffMax:     2 * time.Second,
	}

	pool := httpclient.NewClientPool(config)

	req, _ := http.NewRequest("GET", server.URL, nil)
	ctx := context.Background()

	start := time.Now()
	resp, err := pool.Do(ctx, req)
	duration := time.Since(start)

	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	// Should have taken at least backoff time for retries
	// First retry: ~100ms, Second retry: ~200ms = ~300ms minimum
	if duration < 250*time.Millisecond {
		t.Errorf("Request completed too quickly (%v), backoff may not be working", duration)
	}

	if failCount != 3 {
		t.Errorf("Expected 3 attempts, got %d", failCount)
	}
}

func TestProviderHealthMetrics_SuccessRateTracking(t *testing.T) {
	// This will be implemented when we have access to the metrics package
	// For now, create a placeholder test

	t.Log("Provider health metrics tracking test - placeholder")
	// TODO: Implement when metrics package is available
}
