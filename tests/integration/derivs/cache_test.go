package derivs

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"cryptorun/src/infrastructure/derivs"
)

// MockProvider implements Provider interface for testing
type MockProvider struct {
	name         string
	callCount    map[string]int
	mutex        sync.Mutex
	shouldFail   bool
	responseTime time.Duration
}

func NewMockProvider(name string) *MockProvider {
	return &MockProvider{
		name:      name,
		callCount: make(map[string]int),
	}
}

func (m *MockProvider) Name() string {
	return m.name
}

func (m *MockProvider) SetShouldFail(fail bool) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.shouldFail = fail
}

func (m *MockProvider) SetResponseTime(duration time.Duration) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.responseTime = duration
}

func (m *MockProvider) GetCallCount(method string) int {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	return m.callCount[method]
}

func (m *MockProvider) GetFundingHistory(ctx context.Context, symbol string, limit int) ([]derivs.FundingRate, error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	m.callCount["GetFundingHistory"]++

	// Simulate response time
	if m.responseTime > 0 {
		time.Sleep(m.responseTime)
	}

	if m.shouldFail {
		return nil, fmt.Errorf("mock provider %s intentionally failing", m.name)
	}

	// Generate mock data
	rates := make([]derivs.FundingRate, limit)
	baseTime := time.Now().Add(-8 * time.Hour)

	for i := 0; i < limit; i++ {
		rates[i] = derivs.FundingRate{
			Symbol:    symbol,
			Rate:      0.001 + float64(i)*0.0001, // Vary rates slightly
			Timestamp: baseTime.Add(time.Duration(i) * time.Hour),
			MarkPrice: 50000 + float64(i)*100,
		}
	}

	return rates, nil
}

func (m *MockProvider) GetOpenInterest(ctx context.Context, symbol string) (*derivs.OpenInterest, error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	m.callCount["GetOpenInterest"]++

	if m.responseTime > 0 {
		time.Sleep(m.responseTime)
	}

	if m.shouldFail {
		return nil, fmt.Errorf("mock provider %s intentionally failing", m.name)
	}

	return &derivs.OpenInterest{
		Symbol:    symbol,
		Value:     1000000,
		Timestamp: time.Now(),
	}, nil
}

func (m *MockProvider) GetTickerData(ctx context.Context, symbol string) (*derivs.TickerData, error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	m.callCount["GetTickerData"]++

	if m.responseTime > 0 {
		time.Sleep(m.responseTime)
	}

	if m.shouldFail {
		return nil, fmt.Errorf("mock provider %s intentionally failing", m.name)
	}

	return &derivs.TickerData{
		Symbol:           symbol,
		LastPrice:        50000,
		Volume:           10000,
		QuoteVolume:      500000000,
		WeightedAvgPrice: 49950,
		Timestamp:        time.Now(),
	}, nil
}

// SimpleCache implements basic in-memory caching for testing
type SimpleCache struct {
	data   map[string]CacheEntry
	mutex  sync.RWMutex
	hits   int
	misses int
}

type CacheEntry struct {
	Value     interface{}
	ExpiresAt time.Time
}

func NewSimpleCache() *SimpleCache {
	return &SimpleCache{
		data: make(map[string]CacheEntry),
	}
}

func (c *SimpleCache) Get(key string) (interface{}, bool) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	entry, exists := c.data[key]
	if !exists {
		c.misses++
		return nil, false
	}

	if time.Now().After(entry.ExpiresAt) {
		c.misses++
		// Clean up expired entry
		delete(c.data, key)
		return nil, false
	}

	c.hits++
	return entry.Value, true
}

func (c *SimpleCache) Set(key string, value interface{}, ttlSeconds int) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.data[key] = CacheEntry{
		Value:     value,
		ExpiresAt: time.Now().Add(time.Duration(ttlSeconds) * time.Second),
	}
}

func (c *SimpleCache) GetHitRate() float64 {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	total := c.hits + c.misses
	if total == 0 {
		return 0
	}
	return float64(c.hits) / float64(total)
}

func (c *SimpleCache) GetStats() (int, int) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	return c.hits, c.misses
}

// CachedProvider wraps a provider with caching functionality
type CachedProvider struct {
	provider derivs.Provider
	cache    *SimpleCache
	ttl      int // TTL in seconds
}

func NewCachedProvider(provider derivs.Provider, cache *SimpleCache, ttlSeconds int) *CachedProvider {
	return &CachedProvider{
		provider: provider,
		cache:    cache,
		ttl:      ttlSeconds,
	}
}

func (cp *CachedProvider) Name() string {
	return cp.provider.Name() + "_cached"
}

func (cp *CachedProvider) GetFundingHistory(ctx context.Context, symbol string, limit int) ([]derivs.FundingRate, error) {
	key := fmt.Sprintf("funding_%s_%s_%d", cp.provider.Name(), symbol, limit)

	// Check cache first
	if cached, found := cp.cache.Get(key); found {
		return cached.([]derivs.FundingRate), nil
	}

	// Cache miss - fetch from provider
	result, err := cp.provider.GetFundingHistory(ctx, symbol, limit)
	if err != nil {
		return nil, err
	}

	// Store in cache
	cp.cache.Set(key, result, cp.ttl)

	return result, nil
}

func (cp *CachedProvider) GetOpenInterest(ctx context.Context, symbol string) (*derivs.OpenInterest, error) {
	key := fmt.Sprintf("oi_%s_%s", cp.provider.Name(), symbol)

	if cached, found := cp.cache.Get(key); found {
		return cached.(*derivs.OpenInterest), nil
	}

	result, err := cp.provider.GetOpenInterest(ctx, symbol)
	if err != nil {
		return nil, err
	}

	cp.cache.Set(key, result, cp.ttl)
	return result, nil
}

func (cp *CachedProvider) GetTickerData(ctx context.Context, symbol string) (*derivs.TickerData, error) {
	key := fmt.Sprintf("ticker_%s_%s", cp.provider.Name(), symbol)

	if cached, found := cp.cache.Get(key); found {
		return cached.(*derivs.TickerData), nil
	}

	result, err := cp.provider.GetTickerData(ctx, symbol)
	if err != nil {
		return nil, err
	}

	cp.cache.Set(key, result, cp.ttl)
	return result, nil
}

func TestProviderCaching_TTLRespect(t *testing.T) {
	mockProvider := NewMockProvider("test_provider")
	cache := NewSimpleCache()
	cachedProvider := NewCachedProvider(mockProvider, cache, 2) // 2 second TTL

	ctx := context.Background()
	symbol := "BTCUSDT"

	// First call - should hit provider
	_, err := cachedProvider.GetFundingHistory(ctx, symbol, 10)
	if err != nil {
		t.Fatalf("First call failed: %v", err)
	}

	if mockProvider.GetCallCount("GetFundingHistory") != 1 {
		t.Errorf("Expected 1 provider call, got %d", mockProvider.GetCallCount("GetFundingHistory"))
	}

	// Second call immediately - should hit cache
	_, err = cachedProvider.GetFundingHistory(ctx, symbol, 10)
	if err != nil {
		t.Fatalf("Second call failed: %v", err)
	}

	if mockProvider.GetCallCount("GetFundingHistory") != 1 {
		t.Errorf("Expected 1 provider call (cached), got %d", mockProvider.GetCallCount("GetFundingHistory"))
	}

	hits, misses := cache.GetStats()
	if hits != 1 {
		t.Errorf("Expected 1 cache hit, got %d", hits)
	}

	// Wait for TTL to expire
	time.Sleep(3 * time.Second)

	// Third call after TTL - should hit provider again
	_, err = cachedProvider.GetFundingHistory(ctx, symbol, 10)
	if err != nil {
		t.Fatalf("Third call failed: %v", err)
	}

	if mockProvider.GetCallCount("GetFundingHistory") != 2 {
		t.Errorf("Expected 2 provider calls (cache expired), got %d", mockProvider.GetCallCount("GetFundingHistory"))
	}

	hits, misses = cache.GetStats()
	if misses != 2 { // Initial miss + expired miss
		t.Errorf("Expected 2 cache misses, got %d", misses)
	}

	t.Logf("Cache hit rate: %.2f", cache.GetHitRate())
}

func TestProviderCaching_NoOverQuerying(t *testing.T) {
	mockProvider := NewMockProvider("rate_limited_provider")
	cache := NewSimpleCache()
	cachedProvider := NewCachedProvider(mockProvider, cache, 60) // 1 minute TTL

	ctx := context.Background()
	symbol := "ETHUSDT"

	// Simulate multiple concurrent requests
	const numRequests = 10
	results := make(chan error, numRequests)

	for i := 0; i < numRequests; i++ {
		go func() {
			_, err := cachedProvider.GetTickerData(ctx, symbol)
			results <- err
		}()
	}

	// Wait for all requests to complete
	for i := 0; i < numRequests; i++ {
		err := <-results
		if err != nil {
			t.Errorf("Request %d failed: %v", i, err)
		}
	}

	// Should only have called provider once due to caching
	providerCalls := mockProvider.GetCallCount("GetTickerData")
	if providerCalls > 2 { // Allow some race condition tolerance
		t.Errorf("Expected ≤2 provider calls due to caching, got %d", providerCalls)
	}

	hits, misses := cache.GetStats()
	t.Logf("Cache performance: %d hits, %d misses (%.1f%% hit rate)",
		hits, misses, cache.GetHitRate()*100)
}

func TestProviderCaching_FailureHandling(t *testing.T) {
	mockProvider := NewMockProvider("failing_provider")
	cache := NewSimpleCache()
	cachedProvider := NewCachedProvider(mockProvider, cache, 30)

	ctx := context.Background()
	symbol := "ADAUSDT"

	// First call succeeds
	mockProvider.SetShouldFail(false)
	result1, err := cachedProvider.GetOpenInterest(ctx, symbol)
	if err != nil {
		t.Fatalf("First call should succeed: %v", err)
	}

	// Provider starts failing
	mockProvider.SetShouldFail(true)

	// Second call should return cached result (not fail)
	result2, err := cachedProvider.GetOpenInterest(ctx, symbol)
	if err != nil {
		t.Fatalf("Cached call should not fail: %v", err)
	}

	// Results should be identical (from cache)
	if result1.Value != result2.Value {
		t.Errorf("Cached result differs: %.0f vs %.0f", result1.Value, result2.Value)
	}

	// Provider should only have been called once (second was cached)
	if mockProvider.GetCallCount("GetOpenInterest") != 1 {
		t.Errorf("Expected 1 provider call, got %d", mockProvider.GetCallCount("GetOpenInterest"))
	}
}

func TestProviderCaching_PerformanceImprovement(t *testing.T) {
	mockProvider := NewMockProvider("slow_provider")
	cache := NewSimpleCache()
	cachedProvider := NewCachedProvider(mockProvider, cache, 30)

	// Set mock provider to be slow
	mockProvider.SetResponseTime(100 * time.Millisecond)

	ctx := context.Background()
	symbol := "SOLUSDT"

	// Time first call (should be slow)
	start1 := time.Now()
	_, err := cachedProvider.GetFundingHistory(ctx, symbol, 5)
	if err != nil {
		t.Fatalf("First call failed: %v", err)
	}
	duration1 := time.Since(start1)

	// Time second call (should be fast due to cache)
	start2 := time.Now()
	_, err = cachedProvider.GetFundingHistory(ctx, symbol, 5)
	if err != nil {
		t.Fatalf("Second call failed: %v", err)
	}
	duration2 := time.Since(start2)

	// Cached call should be significantly faster
	if duration2 >= duration1/2 {
		t.Errorf("Cache didn't improve performance: first=%v, second=%v", duration1, duration2)
	}

	t.Logf("Performance improvement: %v -> %v (%.1fx faster)",
		duration1, duration2, float64(duration1)/float64(duration2))
}

func TestProviderCaching_DifferentSymbols(t *testing.T) {
	mockProvider := NewMockProvider("multi_symbol_provider")
	cache := NewSimpleCache()
	cachedProvider := NewCachedProvider(mockProvider, cache, 60)

	ctx := context.Background()
	symbols := []string{"BTCUSDT", "ETHUSDT", "ADAUSDT"}

	// Call each symbol once - should result in 3 provider calls
	for _, symbol := range symbols {
		_, err := cachedProvider.GetTickerData(ctx, symbol)
		if err != nil {
			t.Fatalf("Call for %s failed: %v", symbol, err)
		}
	}

	if mockProvider.GetCallCount("GetTickerData") != 3 {
		t.Errorf("Expected 3 provider calls for different symbols, got %d",
			mockProvider.GetCallCount("GetTickerData"))
	}

	// Call each symbol again - should all hit cache
	for _, symbol := range symbols {
		_, err := cachedProvider.GetTickerData(ctx, symbol)
		if err != nil {
			t.Fatalf("Cached call for %s failed: %v", symbol, err)
		}
	}

	// Should still be 3 provider calls (second round was cached)
	if mockProvider.GetCallCount("GetTickerData") != 3 {
		t.Errorf("Expected 3 provider calls after caching, got %d",
			mockProvider.GetCallCount("GetTickerData"))
	}

	hits, misses := cache.GetStats()
	if hits != 3 {
		t.Errorf("Expected 3 cache hits, got %d", hits)
	}
	if misses != 3 {
		t.Errorf("Expected 3 cache misses, got %d", misses)
	}
}

func TestProviderCaching_BudgetCompliance(t *testing.T) {
	mockProvider := NewMockProvider("budget_test_provider")
	cache := NewSimpleCache()
	cachedProvider := NewCachedProvider(mockProvider, cache, 5) // Short TTL for testing

	ctx := context.Background()
	symbol := "LINKUSDT"

	// Budget: Allow max 5 calls per provider per minute
	maxCallsPerMinute := 5
	testDuration := 10 * time.Second // Shorter test duration

	callCount := 0
	startTime := time.Now()

	// Make calls for the test duration
	for time.Since(startTime) < testDuration {
		_, err := cachedProvider.GetOpenInterest(ctx, symbol)
		if err != nil {
			t.Fatalf("Call failed: %v", err)
		}
		callCount++
		time.Sleep(100 * time.Millisecond) // Spread calls over time
	}

	providerCalls := mockProvider.GetCallCount("GetOpenInterest")

	// With caching, provider calls should be much less than total calls
	if providerCalls >= callCount {
		t.Errorf("Cache not working: provider calls (%d) >= total calls (%d)",
			providerCalls, callCount)
	}

	// Provider calls should respect the budget
	expectedMaxProviderCalls := int(float64(maxCallsPerMinute) * testDuration.Seconds() / 60.0 * 2.0) // 2x margin for test variance
	if providerCalls > expectedMaxProviderCalls {
		t.Errorf("Provider calls (%d) exceed budget-compliant maximum (%d)",
			providerCalls, expectedMaxProviderCalls)
	}

	t.Logf("Budget test results: %d total calls, %d provider calls, %.1f%% cache hit rate",
		callCount, providerCalls, cache.GetHitRate()*100)
}
