package provider

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func TestCircuitBreaker_BasicOperation(t *testing.T) {
	config := CircuitConfig{
		Enabled:          true,
		FailureThreshold: 0.5,
		MinRequests:      3,
		OpenTimeout:      100 * time.Millisecond,
		ProbeInterval:    50 * time.Millisecond,
	}
	
	cb := NewCircuitBreaker("test", config)
	
	// Initially closed
	if cb.GetState() != CircuitClosed {
		t.Errorf("Expected circuit to be closed, got %s", cb.GetState())
	}
	
	// Successful calls should keep circuit closed
	for i := 0; i < 5; i++ {
		err := cb.Call(func() error {
			return nil
		})
		if err != nil {
			t.Errorf("Expected successful call, got error: %v", err)
		}
	}
	
	if cb.GetState() != CircuitClosed {
		t.Errorf("Expected circuit to remain closed after successes, got %s", cb.GetState())
	}
}

func TestCircuitBreaker_OpenOnFailures(t *testing.T) {
	config := CircuitConfig{
		Enabled:          true,
		FailureThreshold: 0.5,
		MinRequests:      2,
		OpenTimeout:      100 * time.Millisecond,
		MaxFailures:      3,
	}
	
	cb := NewCircuitBreaker("test", config)
	
	// Generate failures to open circuit
	for i := 0; i < 4; i++ {
		cb.Call(func() error {
			return &ProviderError{Code: "TEST_ERROR", Message: "test failure"}
		})
	}
	
	// Circuit should be open now
	if cb.GetState() != CircuitOpen {
		t.Errorf("Expected circuit to be open, got %s", cb.GetState())
	}
	
	// Calls should be rejected
	err := cb.Call(func() error {
		return nil
	})
	
	if err == nil {
		t.Error("Expected call to be rejected when circuit is open")
	}
	
	providerErr, ok := err.(*ProviderError)
	if !ok {
		t.Errorf("Expected ProviderError, got %T", err)
	} else if providerErr.Code != ErrCodeCircuitOpen {
		t.Errorf("Expected circuit open error, got %s", providerErr.Code)
	}
}

func TestCircuitBreaker_HalfOpenTransition(t *testing.T) {
	config := CircuitConfig{
		Enabled:          true,
		FailureThreshold: 0.5,
		MinRequests:      2,
		OpenTimeout:      50 * time.Millisecond,
		MaxFailures:      2,
	}
	
	cb := NewCircuitBreaker("test", config)
	
	// Open circuit with failures
	cb.Call(func() error { return &ProviderError{} })
	cb.Call(func() error { return &ProviderError{} })
	cb.Call(func() error { return &ProviderError{} })
	
	if cb.GetState() != CircuitOpen {
		t.Error("Expected circuit to be open")
	}
	
	// Wait for timeout
	time.Sleep(60 * time.Millisecond)
	
	// Next call should transition to half-open
	err := cb.Call(func() error {
		return nil // Success
	})
	
	if err != nil {
		t.Errorf("Expected successful call in half-open state, got: %v", err)
	}
	
	// Should be closed again after success
	if cb.GetState() != CircuitClosed {
		t.Errorf("Expected circuit to be closed after success, got %s", cb.GetState())
	}
}

func TestRateLimiter_TokenBucket(t *testing.T) {
	limits := ProviderLimits{
		RequestsPerSecond: 2,
		BurstLimit:        3,
		Timeout:          time.Second,
	}
	
	rl := NewRateLimiter(limits)
	ctx := context.Background()
	
	// Should allow burst requests
	for i := 0; i < 3; i++ {
		if err := rl.Wait(ctx); err != nil {
			t.Errorf("Expected burst request %d to succeed, got: %v", i, err)
		}
	}
	
	// Next request should be rate limited
	start := time.Now()
	err := rl.Wait(ctx)
	duration := time.Since(start)
	
	if err != nil {
		t.Errorf("Expected rate limited request to eventually succeed, got: %v", err)
	}
	
	// Should have waited approximately 500ms (1/2 requests per second)
	expectedDelay := 450 * time.Millisecond
	if duration < expectedDelay {
		t.Errorf("Expected delay of at least %v, got %v", expectedDelay, duration)
	}
}

func TestProviderCache_BasicOperations(t *testing.T) {
	config := CacheConfig{
		Enabled:    true,
		TTL:        100 * time.Millisecond,
		MaxEntries: 3,
	}
	
	cache := NewProviderCache(config)
	
	// Test set and get
	cache.Set("key1", "value1", config.TTL)
	
	value := cache.Get("key1")
	if value == nil {
		t.Error("Expected cached value, got nil")
	}
	
	if strValue, ok := value.(string); !ok || strValue != "value1" {
		t.Errorf("Expected 'value1', got %v", value)
	}
	
	// Test cache miss
	missValue := cache.Get("nonexistent")
	if missValue != nil {
		t.Errorf("Expected cache miss, got %v", missValue)
	}
	
	// Test expiration
	time.Sleep(150 * time.Millisecond)
	expiredValue := cache.Get("key1")
	if expiredValue != nil {
		t.Errorf("Expected expired value to be nil, got %v", expiredValue)
	}
}

func TestProviderCache_Eviction(t *testing.T) {
	config := CacheConfig{
		Enabled:    true,
		TTL:        time.Second,
		MaxEntries: 2,
	}
	
	cache := NewProviderCache(config)
	
	// Fill cache to capacity
	cache.Set("key1", "value1", config.TTL)
	cache.Set("key2", "value2", config.TTL)
	
	// Adding third item should evict oldest
	cache.Set("key3", "value3", config.TTL)
	
	// key1 should be evicted
	if cache.Get("key1") != nil {
		t.Error("Expected key1 to be evicted")
	}
	
	// key2 and key3 should still be there
	if cache.Get("key2") == nil {
		t.Error("Expected key2 to still be cached")
	}
	if cache.Get("key3") == nil {
		t.Error("Expected key3 to still be cached")
	}
}

func TestKrakenProvider_Integration(t *testing.T) {
	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/SystemStatus" {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"error":[],"result":{"status":"online","timestamp":"2023-09-07T12:00:00Z"}}`))
			return
		}
		
		if r.URL.Path == "/Depth" {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{
				"error": [],
				"result": {
					"XXBTZUSD": {
						"bids": [["50000.0", "1.5", "1693000000"], ["49990.0", "2.0", "1693000001"]],
						"asks": [["50010.0", "1.2", "1693000000"], ["50020.0", "1.8", "1693000001"]]
					}
				}
			}`))
			return
		}
		
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()
	
	// Configure provider with mock server
	config := ProviderConfig{
		Name:    "test-kraken",
		Venue:   "kraken",
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
	
	// Create provider
	provider, err := NewKrakenProvider(config, nil)
	if err != nil {
		t.Fatalf("Failed to create Kraken provider: %v", err)
	}
	
	// Start provider
	ctx := context.Background()
	if err := provider.Start(ctx); err != nil {
		t.Fatalf("Failed to start provider: %v", err)
	}
	defer provider.Stop(ctx)
	
	// Test order book retrieval
	orderBook, err := provider.GetOrderBook(ctx, "BTC-USD")
	if err != nil {
		t.Fatalf("Failed to get order book: %v", err)
	}
	
	if orderBook.Venue != "kraken" {
		t.Errorf("Expected venue 'kraken', got '%s'", orderBook.Venue)
	}
	
	if orderBook.Symbol != "BTC-USD" {
		t.Errorf("Expected symbol 'BTC-USD', got '%s'", orderBook.Symbol)
	}
	
	if orderBook.BestBid != 50000.0 {
		t.Errorf("Expected best bid 50000.0, got %f", orderBook.BestBid)
	}
	
	if orderBook.BestAsk != 50010.0 {
		t.Errorf("Expected best ask 50010.0, got %f", orderBook.BestAsk)
	}
	
	// Verify exchange proof
	if orderBook.ProviderProof.SourceType != "exchange_native" {
		t.Errorf("Expected exchange_native proof, got %s", orderBook.ProviderProof.SourceType)
	}
	
	if orderBook.ProviderProof.Provider != "kraken" {
		t.Errorf("Expected kraken provider proof, got %s", orderBook.ProviderProof.Provider)
	}
}

func TestProviderRegistry_Operations(t *testing.T) {
	registry := NewProviderRegistry()
	
	// Create mock provider
	mockProvider := &MockExchangeProvider{
		name:                "mock",
		venue:              "mock-exchange",
		supportsDerivatives: false,
		healthy:            true,
	}
	
	// Test registration
	err := registry.Register(mockProvider)
	if err != nil {
		t.Fatalf("Failed to register provider: %v", err)
	}
	
	// Test duplicate registration
	err = registry.Register(mockProvider)
	if err == nil {
		t.Error("Expected error when registering duplicate provider")
	}
	
	// Test retrieval
	retrieved, err := registry.Get("mock-exchange")
	if err != nil {
		t.Fatalf("Failed to get provider: %v", err)
	}
	
	if retrieved != mockProvider {
		t.Error("Retrieved provider doesn't match registered provider")
	}
	
	// Test GetAll
	allProviders := registry.GetAll()
	if len(allProviders) != 1 {
		t.Errorf("Expected 1 provider, got %d", len(allProviders))
	}
	
	// Test GetHealthy
	healthyProviders := registry.GetHealthy()
	if len(healthyProviders) != 1 {
		t.Errorf("Expected 1 healthy provider, got %d", len(healthyProviders))
	}
	
	// Test lifecycle
	ctx := context.Background()
	if err := registry.Start(ctx); err != nil {
		t.Fatalf("Failed to start registry: %v", err)
	}
	
	if err := registry.Stop(ctx); err != nil {
		t.Fatalf("Failed to stop registry: %v", err)
	}
}

func TestConcurrentAccess(t *testing.T) {
	// Test concurrent access to circuit breaker
	cb := NewCircuitBreaker("concurrent-test", DefaultCircuitConfig())
	
	var wg sync.WaitGroup
	numGoroutines := 10
	callsPerGoroutine := 100
	
	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < callsPerGoroutine; j++ {
				cb.Call(func() error {
					if j%10 == 0 { // Fail 10% of calls
						return &ProviderError{}
					}
					return nil
				})
			}
		}()
	}
	
	wg.Wait()
	
	stats := cb.GetStats()
	if stats.RequestCount != int64(numGoroutines*callsPerGoroutine) {
		t.Errorf("Expected %d total requests, got %d", numGoroutines*callsPerGoroutine, stats.RequestCount)
	}
}

// Mock provider for testing
type MockExchangeProvider struct {
	name                string
	venue               string
	supportsDerivatives bool
	healthy             bool
	started             bool
}

func (m *MockExchangeProvider) GetName() string {
	return m.name
}

func (m *MockExchangeProvider) GetVenue() string {
	return m.venue
}

func (m *MockExchangeProvider) GetSupportsDerivatives() bool {
	return m.supportsDerivatives
}

func (m *MockExchangeProvider) GetOrderBook(ctx context.Context, symbol string) (*OrderBookData, error) {
	return &OrderBookData{
		Venue:  m.venue,
		Symbol: symbol,
	}, nil
}

func (m *MockExchangeProvider) GetTrades(ctx context.Context, symbol string, limit int) ([]TradeData, error) {
	return []TradeData{}, nil
}

func (m *MockExchangeProvider) GetKlines(ctx context.Context, symbol string, interval string, limit int) ([]KlineData, error) {
	return []KlineData{}, nil
}

func (m *MockExchangeProvider) GetFunding(ctx context.Context, symbol string) (*FundingData, error) {
	return nil, &ProviderError{Code: ErrCodeInsufficientData}
}

func (m *MockExchangeProvider) GetOpenInterest(ctx context.Context, symbol string) (*OpenInterestData, error) {
	return nil, &ProviderError{Code: ErrCodeInsufficientData}
}

func (m *MockExchangeProvider) Health() ProviderHealth {
	return ProviderHealth{
		Healthy: m.healthy,
		Status:  "mock",
	}
}

func (m *MockExchangeProvider) GetLimits() ProviderLimits {
	return ProviderLimits{
		RequestsPerSecond: 10,
		BurstLimit:        5,
		Timeout:          time.Second,
	}
}

func (m *MockExchangeProvider) Start(ctx context.Context) error {
	m.started = true
	return nil
}

func (m *MockExchangeProvider) Stop(ctx context.Context) error {
	m.started = false
	return nil
}