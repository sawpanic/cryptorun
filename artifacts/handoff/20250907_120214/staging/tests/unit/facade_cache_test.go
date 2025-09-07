package unit

import (
	"testing"
	"time"

	"cryptorun/internal/data/cache"
)

func TestTTLCacheBasicOperations(t *testing.T) {
	cache := cache.NewTTLCache(100)
	defer cache.Stop()
	
	// Test Set and Get
	key := "test_key"
	value := "test_value"
	ttl := 1 * time.Second
	
	cache.Set(key, value, ttl)
	
	// Should retrieve the value immediately
	retrieved, found := cache.Get(key)
	if !found {
		t.Fatal("Expected to find cached value")
	}
	
	if retrieved != value {
		t.Fatalf("Expected %v, got %v", value, retrieved)
	}
}

func TestTTLCacheExpiration(t *testing.T) {
	cache := cache.NewTTLCache(100)
	defer cache.Stop()
	
	key := "expire_test"
	value := "will_expire"
	ttl := 50 * time.Millisecond
	
	cache.Set(key, value, ttl)
	
	// Should be available immediately
	_, found := cache.Get(key)
	if !found {
		t.Fatal("Expected to find value before expiration")
	}
	
	// Wait for expiration
	time.Sleep(100 * time.Millisecond)
	
	// Should be expired now
	_, found = cache.Get(key)
	if found {
		t.Fatal("Expected value to be expired")
	}
}

func TestTTLCacheStats(t *testing.T) {
	cache := cache.NewTTLCache(100)
	defer cache.Stop()
	
	// Generate some hits and misses
	cache.Set("key1", "value1", 1*time.Minute)
	cache.Set("key2", "value2", 1*time.Minute)
	
	// Generate hits
	cache.Get("key1")
	cache.Get("key1")
	cache.Get("key2")
	
	// Generate misses
	cache.Get("nonexistent1")
	cache.Get("nonexistent2")
	
	stats := cache.Stats()
	
	if stats.TotalEntries != 2 {
		t.Fatalf("Expected 2 total entries, got %d", stats.TotalEntries)
	}
	
	// Should have positive hit ratio
	if stats.PricesHot.HitRatio <= 0 {
		t.Fatalf("Expected positive hit ratio, got %f", stats.PricesHot.HitRatio)
	}
}

func TestTTLCacheEviction(t *testing.T) {
	// Small cache for testing eviction
	cache := cache.NewTTLCache(2)
	defer cache.Stop()
	
	cache.Set("key1", "value1", 1*time.Minute)
	cache.Set("key2", "value2", 1*time.Minute)
	
	// Cache should be full
	stats := cache.Stats()
	if stats.TotalEntries != 2 {
		t.Fatalf("Expected 2 entries, got %d", stats.TotalEntries)
	}
	
	// Adding third item should trigger eviction
	time.Sleep(10 * time.Millisecond) // Ensure different timestamps
	cache.Set("key3", "value3", 1*time.Minute)
	
	// Should still have 2 entries (oldest evicted)
	stats = cache.Stats()
	if stats.TotalEntries > 2 {
		t.Fatalf("Expected max 2 entries after eviction, got %d", stats.TotalEntries)
	}
	
	// key3 should exist, key1 might be evicted (LRU)
	_, found := cache.Get("key3")
	if !found {
		t.Fatal("Expected newest key to exist after eviction")
	}
}

func TestTTLCacheClear(t *testing.T) {
	cache := cache.NewTTLCache(100)
	defer cache.Stop()
	
	cache.Set("key1", "value1", 1*time.Minute)
	cache.Set("key2", "value2", 1*time.Minute)
	
	stats := cache.Stats()
	if stats.TotalEntries == 0 {
		t.Fatal("Expected entries before clear")
	}
	
	cache.Clear()
	
	stats = cache.Stats()
	if stats.TotalEntries != 0 {
		t.Fatalf("Expected 0 entries after clear, got %d", stats.TotalEntries)
	}
	
	// Verify entries are actually gone
	_, found := cache.Get("key1")
	if found {
		t.Fatal("Expected key1 to be cleared")
	}
}

func TestTTLCacheConcurrency(t *testing.T) {
	cache := cache.NewTTLCache(1000)
	defer cache.Stop()
	
	// Test concurrent reads and writes
	done := make(chan bool)
	
	// Writer goroutine
	go func() {
		for i := 0; i < 100; i++ {
			key := fmt.Sprintf("key_%d", i)
			value := fmt.Sprintf("value_%d", i)
			cache.Set(key, value, 1*time.Minute)
		}
		done <- true
	}()
	
	// Reader goroutine
	go func() {
		for i := 0; i < 100; i++ {
			key := fmt.Sprintf("key_%d", i%10)
			cache.Get(key)
		}
		done <- true
	}()
	
	// Wait for both goroutines
	<-done
	<-done
	
	// Should not panic and should have some entries
	stats := cache.Stats()
	if stats.TotalEntries == 0 {
		t.Fatal("Expected some entries after concurrent operations")
	}
}

// Need to add fmt import
import "fmt"