package datasources

import (
	"fmt"
	"testing"
	"time"
)

func TestCacheManager_SetGet(t *testing.T) {
	cm := NewCacheManager()
	
	// Set and get data
	data := "test data"
	err := cm.Set("test-key", "market_data", data)
	if err != nil {
		t.Fatalf("Failed to set cache data: %v", err)
	}
	
	retrieved, found := cm.Get("test-key")
	if !found {
		t.Error("Expected to find cached data")
	}
	
	if retrieved != data {
		t.Errorf("Expected %v, got %v", data, retrieved)
	}
}

func TestCacheManager_TTLExpiration(t *testing.T) {
	cm := NewCacheManager()
	cm.SetTTL("short_ttl", 100*time.Millisecond)
	
	// Set data with short TTL
	data := "expiring data"
	err := cm.Set("expire-key", "short_ttl", data)
	if err != nil {
		t.Fatalf("Failed to set cache data: %v", err)
	}
	
	// Should be available immediately
	_, found := cm.Get("expire-key")
	if !found {
		t.Error("Expected to find cached data immediately")
	}
	
	// Wait for expiration
	time.Sleep(150 * time.Millisecond)
	
	// Should be expired now
	_, found = cm.Get("expire-key")
	if found {
		t.Error("Expected cached data to be expired")
	}
}

func TestCacheManager_ZeroTTL(t *testing.T) {
	cm := NewCacheManager()
	
	// ws_stream has 0 TTL (should not cache)
	err := cm.Set("stream-key", "ws_stream", "stream data")
	if err != nil {
		t.Fatalf("Failed to set cache data: %v", err)
	}
	
	// Should not be cached due to 0 TTL
	_, found := cm.Get("stream-key")
	if found {
		t.Error("Expected ws_stream data not to be cached")
	}
}

func TestCacheManager_GetWithTTL(t *testing.T) {
	cm := NewCacheManager()
	cm.SetTTL("test_category", 1*time.Second)
	
	data := "ttl test data"
	cm.Set("ttl-key", "test_category", data)
	
	retrieved, remaining, found := cm.GetWithTTL("ttl-key")
	if !found {
		t.Error("Expected to find cached data")
	}
	
	if retrieved != data {
		t.Errorf("Expected %v, got %v", data, retrieved)
	}
	
	if remaining <= 0 || remaining > 1*time.Second {
		t.Errorf("Expected remaining TTL between 0 and 1s, got %v", remaining)
	}
}

func TestCacheManager_Delete(t *testing.T) {
	cm := NewCacheManager()
	
	cm.Set("delete-key", "market_data", "data to delete")
	
	// Verify it exists
	_, found := cm.Get("delete-key")
	if !found {
		t.Error("Expected to find cached data before deletion")
	}
	
	// Delete it
	cm.Delete("delete-key")
	
	// Verify it's gone
	_, found = cm.Get("delete-key")
	if found {
		t.Error("Expected cached data to be deleted")
	}
}

func TestCacheManager_Clear(t *testing.T) {
	cm := NewCacheManager()
	
	// Add multiple entries
	cm.Set("key1", "market_data", "data1")
	cm.Set("key2", "market_data", "data2")
	cm.Set("key3", "market_data", "data3")
	
	// Clear all
	cm.Clear()
	
	// Verify all are gone
	for _, key := range []string{"key1", "key2", "key3"} {
		if _, found := cm.Get(key); found {
			t.Errorf("Expected key %s to be cleared", key)
		}
	}
}

func TestCacheManager_CleanExpired(t *testing.T) {
	cm := NewCacheManager()
	cm.SetTTL("short_ttl", 50*time.Millisecond)
	cm.SetTTL("long_ttl", 1*time.Hour)
	
	// Add entries with different TTLs
	cm.Set("short-key", "short_ttl", "short data")
	cm.Set("long-key", "long_ttl", "long data")
	
	// Wait for short TTL to expire
	time.Sleep(100 * time.Millisecond)
	
	// Clean expired entries
	cleaned := cm.CleanExpired()
	if cleaned != 1 {
		t.Errorf("Expected 1 expired entry, cleaned %d", cleaned)
	}
	
	// Verify short key is gone but long key remains
	_, found := cm.Get("short-key")
	if found {
		t.Error("Expected short-key to be cleaned")
	}
	
	_, found = cm.Get("long-key")
	if !found {
		t.Error("Expected long-key to remain")
	}
}

func TestCacheManager_Stats(t *testing.T) {
	cm := NewCacheManager()
	cm.SetTTL("short_ttl", 50*time.Millisecond)
	
	// Add entries
	cm.Set("key1", "market_data", "data1")
	cm.Set("key2", "short_ttl", "data2")
	
	// Get initial stats
	stats := cm.Stats()
	if stats.TotalEntries != 2 {
		t.Errorf("Expected 2 total entries, got %d", stats.TotalEntries)
	}
	if stats.ActiveEntries != 2 {
		t.Errorf("Expected 2 active entries, got %d", stats.ActiveEntries)
	}
	
	// Wait for one to expire
	time.Sleep(100 * time.Millisecond)
	
	stats = cm.Stats()
	if stats.TotalEntries != 2 {
		t.Errorf("Expected 2 total entries, got %d", stats.TotalEntries)
	}
	if stats.ExpiredEntries != 1 {
		t.Errorf("Expected 1 expired entry, got %d", stats.ExpiredEntries)
	}
}

func TestCacheManager_BuildKey(t *testing.T) {
	cm := NewCacheManager()
	
	// Test key building without parameters
	key1 := cm.BuildKey("binance", "ticker", nil)
	expected1 := "binance:ticker"
	if key1 != expected1 {
		t.Errorf("Expected key %s, got %s", expected1, key1)
	}
	
	// Test key building with parameters
	params := map[string]string{
		"symbol": "BTCUSD",
		"limit":  "100",
	}
	key2 := cm.BuildKey("binance", "orderbook", params)
	
	// Should contain provider and endpoint
	if len(key2) <= len("binance:orderbook") {
		t.Error("Expected key with parameters to be longer")
	}
	
	// Same parameters should produce same key
	key3 := cm.BuildKey("binance", "orderbook", params)
	if key2 != key3 {
		t.Error("Expected same parameters to produce same key")
	}
}

func TestCacheManager_ProviderSpecificTTL(t *testing.T) {
	cm := NewCacheManager()
	
	// Test provider-specific override
	binanceTTL := cm.GetProviderTTL("binance", "klines")
	expectedBinanceTTL := 300 * time.Second // From ProviderCacheOverrides
	if binanceTTL != expectedBinanceTTL {
		t.Errorf("Expected Binance klines TTL %v, got %v", expectedBinanceTTL, binanceTTL)
	}
	
	// Test fallback to default
	defaultTTL := cm.GetProviderTTL("kraken", "market_data")
	expectedDefaultTTL := 120 * time.Second // From DefaultCacheConfig
	if defaultTTL != expectedDefaultTTL {
		t.Errorf("Expected default market_data TTL %v, got %v", expectedDefaultTTL, defaultTTL)
	}
}

func TestCacheManager_SetProviderData(t *testing.T) {
	cm := NewCacheManager()
	
	params := map[string]string{"symbol": "BTCUSD"}
	data := "provider data"
	
	err := cm.SetProviderData("binance", "ticker", "price_current", params, data)
	if err != nil {
		t.Fatalf("Failed to set provider data: %v", err)
	}
	
	// Build the same key and retrieve
	key := cm.BuildKey("binance", "ticker", params)
	retrieved, found := cm.Get(key)
	if !found {
		t.Error("Expected to find provider data")
	}
	
	if retrieved != data {
		t.Errorf("Expected %v, got %v", data, retrieved)
	}
}

func TestDefaultCacheConfig(t *testing.T) {
	// Verify all default configurations are valid
	for category, ttl := range DefaultCacheConfig {
		if category == "" {
			t.Error("Found empty category in default config")
		}
		if ttl < 0 {
			t.Errorf("Category %s has negative TTL: %v", category, ttl)
		}
	}
	
	// Verify WebSocket streams have 0 TTL
	if DefaultCacheConfig["ws_stream"] != 0 {
		t.Error("WebSocket streams should have 0 TTL")
	}
}

func TestCacheManager_ConcurrentAccess(t *testing.T) {
	cm := NewCacheManager()
	
	// Test concurrent access doesn't cause panics
	done := make(chan bool)
	
	for i := 0; i < 10; i++ {
		go func(id int) {
			for j := 0; j < 100; j++ {
				key := fmt.Sprintf("key-%d-%d", id, j)
				cm.Set(key, "market_data", fmt.Sprintf("data-%d-%d", id, j))
				cm.Get(key)
				cm.Delete(key)
			}
			done <- true
		}(i)
	}
	
	// Wait for all goroutines to complete
	for i := 0; i < 10; i++ {
		<-done
	}
}