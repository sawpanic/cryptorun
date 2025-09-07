package provider

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestMultiProviderIntegration(t *testing.T) {
	// Create registry
	registry := NewProviderRegistry()
	
	// Test with multiple providers
	providers := []struct {
		name     string
		venue    string
		factory  func(config ProviderConfig, callback func(string, interface{})) (ExchangeProvider, error)
	}{
		{"test-kraken", "kraken", NewKrakenProvider},
		{"test-binance", "binance", NewBinanceProvider},
		{"test-coinbase", "coinbase", NewCoinbaseProvider},
		{"test-okx", "okx", NewOKXProvider},
	}
	
	// Mock server for all exchanges
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Handle different provider endpoints
		switch {
		case r.URL.Path == "/SystemStatus" || r.URL.Path == "/api/v3/ping" || 
			 r.URL.Path == "/api/v3/brokerage/time" || r.URL.Path == "/api/v5/system/time":
			// Health check endpoints
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"status":"ok"}`))
			
		case r.URL.Path == "/Depth":
			// Kraken order book
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{
				"error": [],
				"result": {
					"XXBTZUSD": {
						"bids": [["50000.0", "1.5"], ["49990.0", "2.0"]],
						"asks": [["50010.0", "1.2"], ["50020.0", "1.8"]]
					}
				}
			}`))
			
		case r.URL.Path == "/api/v3/depth":
			// Binance order book
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{
				"lastUpdateId": 123456,
				"bids": [["50000.0", "1.5"], ["49990.0", "2.0"]],
				"asks": [["50010.0", "1.2"], ["50020.0", "1.8"]]
			}`))
			
		case r.URL.Path == "/api/v3/brokerage/product_book":
			// Coinbase order book
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{
				"pricebook": {
					"product_id": "BTC-USD",
					"bids": [{"price": "50000.0", "size": "1.5"}],
					"asks": [{"price": "50010.0", "size": "1.2"}],
					"time": "2023-09-07T12:00:00Z"
				}
			}`))
			
		case r.URL.Path == "/api/v5/market/books":
			// OKX order book
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{
				"code": "0",
				"msg": "",
				"data": [{
					"instId": "BTC-USDT",
					"bids": [["50000.0", "1.5"]],
					"asks": [["50010.0", "1.2"]],
					"ts": "1693832400000"
				}]
			}`))
			
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	
	// Create and register all providers
	for _, providerInfo := range providers {
		config := ProviderConfig{
			Name:    providerInfo.name,
			Venue:   providerInfo.venue,
			BaseURL: server.URL,
			RateLimit: ProviderLimits{
				RequestsPerSecond: 10,
				BurstLimit:        5,
				Timeout:          5 * time.Second,
			},
			CircuitBreaker: DefaultCircuitConfig(),
			CacheConfig: CacheConfig{
				Enabled:    true,
				TTL:        5 * time.Second,
				MaxEntries: 100,
			},
		}
		
		provider, err := providerInfo.factory(config, nil)
		if err != nil {
			t.Fatalf("Failed to create %s provider: %v", providerInfo.venue, err)
		}
		
		if err := registry.Register(provider); err != nil {
			t.Fatalf("Failed to register %s provider: %v", providerInfo.venue, err)
		}
	}
	
	// Start all providers
	ctx := context.Background()
	if err := registry.Start(ctx); err != nil {
		t.Fatalf("Failed to start provider registry: %v", err)
	}
	defer registry.Stop(ctx)
	
	// Test that all providers are registered and healthy
	allProviders := registry.GetAll()
	if len(allProviders) != len(providers) {
		t.Errorf("Expected %d providers, got %d", len(providers), len(allProviders))
	}
	
	healthyProviders := registry.GetHealthy()
	if len(healthyProviders) != len(providers) {
		t.Errorf("Expected %d healthy providers, got %d", len(providers), len(healthyProviders))
	}
	
	// Test order book retrieval from each provider
	for _, providerInfo := range providers {
		provider, err := registry.Get(providerInfo.venue)
		if err != nil {
			t.Errorf("Failed to get %s provider: %v", providerInfo.venue, err)
			continue
		}
		
		orderBook, err := provider.GetOrderBook(ctx, "BTC-USD")
		if err != nil {
			t.Errorf("Failed to get order book from %s: %v", providerInfo.venue, err)
			continue
		}
		
		if orderBook.Venue != providerInfo.venue {
			t.Errorf("Expected venue %s, got %s", providerInfo.venue, orderBook.Venue)
		}
		
		if orderBook.Symbol != "BTC-USD" {
			t.Errorf("Expected symbol BTC-USD, got %s", orderBook.Symbol)
		}
		
		if len(orderBook.Bids) == 0 {
			t.Errorf("No bids returned from %s", providerInfo.venue)
		}
		
		if len(orderBook.Asks) == 0 {
			t.Errorf("No asks returned from %s", providerInfo.venue)
		}
		
		// Verify exchange-native proof
		if orderBook.ProviderProof.SourceType != "exchange_native" {
			t.Errorf("Expected exchange_native proof from %s, got %s", 
				providerInfo.venue, orderBook.ProviderProof.SourceType)
		}
		
		if orderBook.ProviderProof.Provider != providerInfo.venue {
			t.Errorf("Expected provider proof %s, got %s", 
				providerInfo.venue, orderBook.ProviderProof.Provider)
		}
	}
	
	// Test circuit breaker statistics
	stats := registry.GetCircuitBreakerStats()
	if len(stats) == 0 {
		t.Error("No circuit breaker stats returned")
	}
	
	// Test health status
	health := registry.Health()
	if len(health) != len(providers) {
		t.Errorf("Expected health for %d providers, got %d", len(providers), len(health))
	}
	
	for venue, healthStatus := range health {
		if !healthStatus.Healthy {
			t.Errorf("Provider %s is not healthy: %s", venue, healthStatus.Status)
		}
	}
}

func TestProviderFailover(t *testing.T) {
	registry := NewProviderRegistry()
	
	// Create two mock servers - one healthy, one failing
	healthyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"error": [],
			"result": {
				"XXBTZUSD": {
					"bids": [["50000.0", "1.5"]],
					"asks": [["50010.0", "1.2"]]
				}
			}
		}`))
	}))
	defer healthyServer.Close()
	
	failingServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer failingServer.Close()
	
	// Create two Kraken providers with different endpoints
	healthyProvider, err := NewKrakenProvider(ProviderConfig{
		Name:    "kraken-primary",
		Venue:   "kraken-primary",
		BaseURL: healthyServer.URL,
		RateLimit: ProviderLimits{
			RequestsPerSecond: 10,
			BurstLimit:        5,
			Timeout:          time.Second,
		},
		CircuitBreaker: CircuitConfig{
			Enabled:          true,
			FailureThreshold: 0.5,
			MinRequests:      2,
			OpenTimeout:      100 * time.Millisecond,
			MaxFailures:      2,
		},
		CacheConfig: CacheConfig{
			Enabled:    true,
			TTL:        time.Second,
			MaxEntries: 10,
		},
	}, nil)
	if err != nil {
		t.Fatalf("Failed to create healthy provider: %v", err)
	}
	
	failingProvider, err := NewKrakenProvider(ProviderConfig{
		Name:    "kraken-backup",
		Venue:   "kraken-backup",
		BaseURL: failingServer.URL,
		RateLimit: ProviderLimits{
			RequestsPerSecond: 10,
			BurstLimit:        5,
			Timeout:          time.Second,
		},
		CircuitBreaker: CircuitConfig{
			Enabled:          true,
			FailureThreshold: 0.5,
			MinRequests:      2,
			OpenTimeout:      100 * time.Millisecond,
			MaxFailures:      2,
		},
		CacheConfig: CacheConfig{
			Enabled:    true,
			TTL:        time.Second,
			MaxEntries: 10,
		},
	}, nil)
	if err != nil {
		t.Fatalf("Failed to create failing provider: %v", err)
	}
	
	// Register both providers
	registry.Register(healthyProvider)
	registry.Register(failingProvider)
	
	ctx := context.Background()
	registry.Start(ctx)
	defer registry.Stop(ctx)
	
	// Test healthy provider works
	_, err = healthyProvider.GetOrderBook(ctx, "BTC-USD")
	if err != nil {
		t.Errorf("Healthy provider failed: %v", err)
	}
	
	// Test failing provider triggers circuit breaker
	for i := 0; i < 5; i++ {
		failingProvider.GetOrderBook(ctx, "BTC-USD")
	}
	
	// Check that only healthy provider is returned
	healthyProviders := registry.GetHealthy()
	if len(healthyProviders) != 1 {
		t.Errorf("Expected 1 healthy provider after failures, got %d", len(healthyProviders))
	}
	
	if healthyProviders[0].GetVenue() != "kraken-primary" {
		t.Errorf("Expected kraken-primary to be healthy, got %s", healthyProviders[0].GetVenue())
	}
}

func TestProviderCacheEffectiveness(t *testing.T) {
	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"error": [],
			"result": {
				"XXBTZUSD": {
					"bids": [["50000.0", "1.5"]],
					"asks": [["50010.0", "1.2"]]
				}
			}
		}`))
	}))
	defer server.Close()
	
	provider, err := NewKrakenProvider(ProviderConfig{
		Name:    "kraken-cache-test",
		Venue:   "kraken",
		BaseURL: server.URL,
		RateLimit: ProviderLimits{
			RequestsPerSecond: 100,
			BurstLimit:        50,
			Timeout:          time.Second,
		},
		CircuitBreaker: DefaultCircuitConfig(),
		CacheConfig: CacheConfig{
			Enabled:    true,
			TTL:        time.Second,
			MaxEntries: 100,
		},
	}, nil)
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}
	
	ctx := context.Background()
	provider.Start(ctx)
	defer provider.Stop(ctx)
	
	// Make multiple requests quickly
	for i := 0; i < 10; i++ {
		_, err := provider.GetOrderBook(ctx, "BTC-USD")
		if err != nil {
			t.Errorf("Request %d failed: %v", i, err)
		}
	}
	
	// Should have made only 1 actual HTTP request due to caching
	if requestCount > 2 { // Allow for 1 startup + 1 data request
		t.Errorf("Expected at most 2 HTTP requests, got %d (cache not working)", requestCount)
	}
	
	// Wait for cache to expire
	time.Sleep(1100 * time.Millisecond)
	
	// Make another request - should hit the server again
	initialCount := requestCount
	_, err = provider.GetOrderBook(ctx, "BTC-USD")
	if err != nil {
		t.Errorf("Post-expiration request failed: %v", err)
	}
	
	if requestCount <= initialCount {
		t.Error("Cache did not expire - no new request made")
	}
}