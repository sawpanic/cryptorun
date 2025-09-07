package unit

import (
	"context"
	"testing"
	"time"

	"github.com/sawpanic/cryptorun/internal/domain/microstructure"
	"github.com/sawpanic/cryptorun/internal/domain/regime"
	"github.com/sawpanic/cryptorun/internal/infrastructure/datafacade"
	"github.com/sawpanic/cryptorun/internal/infrastructure/datafacade/cache"
	"github.com/sawpanic/cryptorun/internal/infrastructure/datafacade/fakes"
)

func TestTTLCache(t *testing.T) {
	ttlCache := cache.NewTTLCache(time.Second)
	defer ttlCache.Close()

	// Test basic set/get
	ttlCache.Set("key1", "value1", time.Minute)
	
	value, found := ttlCache.Get("key1")
	if !found {
		t.Error("Expected to find key1")
	}
	
	if value != "value1" {
		t.Errorf("Expected 'value1', got %v", value)
	}

	// Test expiration
	ttlCache.Set("key2", "value2", 10*time.Millisecond)
	time.Sleep(20*time.Millisecond)
	
	_, found = ttlCache.Get("key2")
	if found {
		t.Error("Expected key2 to be expired")
	}

	// Test TTL retrieval
	ttlCache.Set("key3", "value3", time.Hour)
	value, ttl, found := ttlCache.GetWithTTL("key3")
	if !found {
		t.Error("Expected to find key3")
	}
	
	if ttl <= 0 || ttl > time.Hour {
		t.Errorf("Expected positive TTL less than 1 hour, got %v", ttl)
	}

	// Test item count
	count := ttlCache.ItemCount()
	if count < 1 {
		t.Errorf("Expected at least 1 item, got %d", count)
	}

	// Test flush
	removedCount := ttlCache.Flush()
	t.Logf("Removed %d expired items", removedCount)
}

func TestLayeredCache(t *testing.T) {
	layeredCache := cache.NewLayeredCache(
		10*time.Millisecond, // Hot
		100*time.Millisecond, // Warm
		time.Second,         // Cold
	)
	defer layeredCache.Close()

	// Test setting in different tiers
	layeredCache.Set("hot_key", "hot_value", cache.TierHot)
	layeredCache.Set("warm_key", "warm_value", cache.TierWarm)
	layeredCache.Set("cold_key", "cold_value", cache.TierCold)

	// Test retrieval and tier identification
	value, tier, found := layeredCache.Get("hot_key")
	if !found {
		t.Error("Expected to find hot_key")
	}
	if tier != cache.TierHot {
		t.Errorf("Expected hot tier, got %v", tier)
	}
	if value != "hot_value" {
		t.Errorf("Expected 'hot_value', got %v", value)
	}

	// Test promotion from warm to hot
	value, tier, found = layeredCache.Get("warm_key")
	if !found {
		t.Error("Expected to find warm_key")
	}
	if tier != cache.TierWarm {
		t.Errorf("Expected warm tier, got %v", tier)
	}

	// Verify promotion occurred - warm key should now also be in hot
	time.Sleep(1 * time.Millisecond) // Small delay
	value, tier, found = layeredCache.Get("warm_key")
	if !found {
		t.Error("Expected to find promoted warm_key")
	}
	// Should still report original tier found
	if tier != cache.TierHot {
		t.Errorf("Expected hot tier after promotion, got %v", tier)
	}

	// Test cache stats
	stats := layeredCache.GetStats()
	if stats.HotHits == 0 && stats.WarmHits == 0 && stats.ColdHits == 0 {
		t.Error("Expected some cache hits")
	}
}

func TestCacheKeyGeneration(t *testing.T) {
	// Test basic key generation
	key1 := cache.CacheKey("component1", "component2", "component3")
	expectedKey1 := "component1:component2:component3"
	if key1 != expectedKey1 {
		t.Errorf("Expected '%s', got '%s'", expectedKey1, key1)
	}

	// Test single component
	key2 := cache.CacheKey("single")
	if key2 != "single" {
		t.Errorf("Expected 'single', got '%s'", key2)
	}

	// Test empty key
	key3 := cache.CacheKey()
	if key3 != "" {
		t.Errorf("Expected empty string, got '%s'", key3)
	}

	// Test time-bucketed key
	key4 := cache.CacheKeyWithTimestamp(time.Hour, "regime", "market")
	if len(key4) == 0 {
		t.Error("Expected non-empty time-bucketed key")
	}
	
	// Keys should be consistent within the same time bucket
	key5 := cache.CacheKeyWithTimestamp(time.Hour, "regime", "market")
	if key4 != key5 {
		t.Error("Expected consistent time-bucketed keys within same hour")
	}
}

func TestDeterministicFakeProvider(t *testing.T) {
	symbols := []string{"BTC-USD", "ETH-USD", "ADA-USD"}
	baseTime := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	
	provider := fakes.NewDeterministicFakeProvider(baseTime, symbols)

	// Test microstructure data generation
	data1 := provider.GetMicrostructureData("BTC-USD", baseTime)
	data2 := provider.GetMicrostructureData("BTC-USD", baseTime)

	// Should be deterministic - same inputs produce same outputs
	if data1.BestBid != data2.BestBid {
		t.Errorf("Expected deterministic data: %f != %f", data1.BestBid, data2.BestBid)
	}

	if data1.Symbol != "BTC-USD" {
		t.Errorf("Expected symbol 'BTC-USD', got '%s'", data1.Symbol)
	}

	// Verify realistic data properties
	if data1.BestBid >= data1.BestAsk {
		t.Error("Best bid should be less than best ask")
	}

	if len(data1.OrderBook.Bids) != 5 || len(data1.OrderBook.Asks) != 5 {
		t.Errorf("Expected 5 bid and ask levels, got %d bids, %d asks",
			len(data1.OrderBook.Bids), len(data1.OrderBook.Asks))
	}

	if data1.Volume24h <= 0 {
		t.Error("Expected positive 24h volume")
	}

	// Test regime data generation
	regimeData1 := provider.GetRegimeData(baseTime)
	regimeData2 := provider.GetRegimeData(baseTime)

	// Should be deterministic
	if regimeData1.RealizedVol7d != regimeData2.RealizedVol7d {
		t.Errorf("Expected deterministic regime data: %f != %f",
			regimeData1.RealizedVol7d, regimeData2.RealizedVol7d)
	}

	if len(regimeData1.Prices) != 24 {
		t.Errorf("Expected 24 hourly prices, got %d", len(regimeData1.Prices))
	}

	// Test supported symbols
	supportedSymbols := provider.GetSupportedSymbols()
	if len(supportedSymbols) != len(symbols) {
		t.Errorf("Expected %d symbols, got %d", len(symbols), len(supportedSymbols))
	}
}

func TestDataFacadeWithFakes(t *testing.T) {
	config := datafacade.DefaultFacadeConfig()
	config.UseFakesForTesting = true
	config.HotTTL = 100 * time.Millisecond
	config.WarmTTL = 500 * time.Millisecond

	facade := datafacade.NewDataFacade(config)
	defer facade.Close()

	ctx := context.Background()

	// Test microstructure data retrieval
	data, err := facade.GetMicrostructureData(ctx, "BTC-USD")
	if err != nil {
		t.Fatalf("Failed to get microstructure data: %v", err)
	}

	if data.Symbol != "BTC-USD" {
		t.Errorf("Expected symbol 'BTC-USD', got '%s'", data.Symbol)
	}

	// Test caching - second request should be faster and from cache
	start := time.Now()
	data2, err := facade.GetMicrostructureData(ctx, "BTC-USD")
	duration := time.Since(start)

	if err != nil {
		t.Fatalf("Failed to get cached microstructure data: %v", err)
	}

	if duration > 10*time.Millisecond {
		t.Errorf("Cached request took too long: %v", duration)
	}

	// Data should be identical (from cache)
	if data.Timestamp != data2.Timestamp {
		t.Error("Expected identical cached data")
	}

	// Test regime data retrieval
	regimeData, err := facade.GetRegimeData(ctx)
	if err != nil {
		t.Fatalf("Failed to get regime data: %v", err)
	}

	if regimeData.Symbol == "" {
		t.Error("Expected non-empty regime data symbol")
	}

	// Test batch retrieval
	symbols := []string{"BTC-USD", "ETH-USD", "ADA-USD"}
	batchData, batchErrors := facade.GetBatchMicrostructureData(ctx, symbols)

	if len(batchErrors) > 0 {
		t.Errorf("Expected no batch errors, got %d errors", len(batchErrors))
	}

	if len(batchData) != len(symbols) {
		t.Errorf("Expected %d batch results, got %d", len(symbols), len(batchData))
	}

	for _, symbol := range symbols {
		if _, exists := batchData[symbol]; !exists {
			t.Errorf("Missing batch data for symbol %s", symbol)
		}
	}
}

func TestDataFacadeCacheExpiration(t *testing.T) {
	config := datafacade.DefaultFacadeConfig()
	config.UseFakesForTesting = true
	config.HotTTL = 50 * time.Millisecond  // Very short TTL for testing

	facade := datafacade.NewDataFacade(config)
	defer facade.Close()

	ctx := context.Background()

	// Get initial data
	data1, err := facade.GetMicrostructureData(ctx, "BTC-USD")
	if err != nil {
		t.Fatalf("Failed to get initial data: %v", err)
	}

	// Wait for cache expiration
	time.Sleep(100 * time.Millisecond)

	// Get data again - should be fresh (different timestamp)
	data2, err := facade.GetMicrostructureData(ctx, "BTC-USD")
	if err != nil {
		t.Fatalf("Failed to get expired data: %v", err)
	}

	// Timestamps should be different (fresh data generated)
	if data1.Timestamp.Equal(data2.Timestamp) {
		t.Error("Expected fresh data after cache expiration")
	}
}

func TestDataFacadeStats(t *testing.T) {
	config := datafacade.DefaultFacadeConfig()
	config.UseFakesForTesting = true

	facade := datafacade.NewDataFacade(config)
	defer facade.Close()

	ctx := context.Background()

	// Generate some cache activity
	_, _ = facade.GetMicrostructureData(ctx, "BTC-USD")
	_, _ = facade.GetMicrostructureData(ctx, "BTC-USD") // Cache hit
	_, _ = facade.GetMicrostructureData(ctx, "ETH-USD")
	_, _ = facade.GetRegimeData(ctx)

	// Test cache stats
	cacheStats := facade.GetCacheStats()
	if cacheStats.HotHits == 0 && cacheStats.WarmHits == 0 && cacheStats.ColdHits == 0 {
		t.Error("Expected some cache hits")
	}

	// Test circuit states (should be empty since we're using fakes)
	circuitStates := facade.GetCircuitStates()
	if len(circuitStates) > 0 {
		t.Errorf("Expected no circuit states with fakes, got %d", len(circuitStates))
	}

	// Test metrics
	metrics := facade.GetMetrics()
	if metrics.LastUpdated.IsZero() {
		t.Error("Expected non-zero metrics timestamp")
	}

	// Test supported symbols
	symbols := facade.GetSupportedSymbols()
	if len(symbols) == 0 {
		t.Error("Expected non-empty supported symbols list")
	}
}

func TestCacheKeyConsistency(t *testing.T) {
	// Test that cache keys are consistent
	key1 := cache.CacheKey("microstructure", "BTC-USD")
	key2 := cache.CacheKey("microstructure", "BTC-USD")
	
	if key1 != key2 {
		t.Error("Cache keys should be consistent")
	}

	// Test different keys are different
	key3 := cache.CacheKey("microstructure", "ETH-USD")
	if key1 == key3 {
		t.Error("Different cache keys should not be equal")
	}

	// Test time-bucketed keys consistency within same bucket
	now := time.Now()
	bucketKey1 := cache.CacheKeyWithTimestamp(time.Hour, "regime")
	time.Sleep(1 * time.Millisecond) // Small delay within same hour
	bucketKey2 := cache.CacheKeyWithTimestamp(time.Hour, "regime")
	
	if bucketKey1 != bucketKey2 {
		t.Error("Time-bucketed keys should be consistent within same time bucket")
	}
}