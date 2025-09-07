package cache

import (
	"sync"
	"time"

	"github.com/sawpanic/cryptorun/internal/data/interfaces"
)

// TTLCache implements Cache with time-based expiration
type TTLCache struct {
	mu         sync.RWMutex
	entries    map[string]*cacheEntry
	maxEntries int64
	stats      cacheStats
	
	// Cleanup
	stopCh chan struct{}
}

type cacheEntry struct {
	value     interface{}
	expires   time.Time
	accessed  time.Time
	hits      int64
}

type cacheStats struct {
	hits         int64
	misses       int64
	evictions    int64
	cleanupRuns  int64
}

// NewTTLCache creates a new TTL cache with specified maximum entries
func NewTTLCache(maxEntries int64) *TTLCache {
	cache := &TTLCache{
		entries:    make(map[string]*cacheEntry),
		maxEntries: maxEntries,
		stopCh:     make(chan struct{}),
	}
	
	// Start cleanup goroutine
	go cache.cleanup()
	
	return cache
}

// Get retrieves a value from cache if not expired
func (c *TTLCache) Get(key string) (interface{}, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	entry, exists := c.entries[key]
	if !exists {
		c.stats.misses++
		return nil, false
	}
	
	// Check expiration
	if time.Now().After(entry.expires) {
		c.stats.misses++
		// Don't delete here to avoid write lock upgrade
		return nil, false
	}
	
	// Update access tracking
	entry.accessed = time.Now()
	entry.hits++
	c.stats.hits++
	
	return entry.value, true
}

// Set stores a value in cache with TTL
func (c *TTLCache) Set(key string, value interface{}, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	// Check if we need to evict entries
	if int64(len(c.entries)) >= c.maxEntries {
		c.evictLRU()
	}
	
	expires := time.Now().Add(ttl)
	c.entries[key] = &cacheEntry{
		value:    value,
		expires:  expires,
		accessed: time.Now(),
		hits:     0,
	}
}

// Stats returns cache performance statistics
func (c *TTLCache) Stats() interfaces.CacheStats {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	totalRequests := c.stats.hits + c.stats.misses
	hitRatio := 0.0
	if totalRequests > 0 {
		hitRatio = float64(c.stats.hits) / float64(totalRequests)
	}
	
	// Count entries by TTL tiers (approximated)
	var pricesHot, pricesWarm, volumesVADR, tokenMeta int64
	now := time.Now()
	
	for _, entry := range c.entries {
		if time.Now().After(entry.expires) {
			continue // Skip expired entries
		}
		
		ttl := entry.expires.Sub(now)
		switch {
		case ttl <= 10*time.Second: // Hot prices
			pricesHot++
		case ttl <= 60*time.Second: // Warm prices
			pricesWarm++
		case ttl <= 300*time.Second: // VADR volumes
			volumesVADR++
		default: // Token metadata
			tokenMeta++
		}
	}
	
	return interfaces.CacheStats{
		PricesHot: interfaces.CacheTierStats{
			TTL:      5 * time.Second,
			Hits:     c.stats.hits / 4, // Approximate distribution
			Misses:   c.stats.misses / 4,
			Entries:  pricesHot,
			HitRatio: hitRatio,
		},
		PricesWarm: interfaces.CacheTierStats{
			TTL:      30 * time.Second,
			Hits:     c.stats.hits / 4,
			Misses:   c.stats.misses / 4,
			Entries:  pricesWarm,
			HitRatio: hitRatio,
		},
		VolumesVADR: interfaces.CacheTierStats{
			TTL:      120 * time.Second,
			Hits:     c.stats.hits / 4,
			Misses:   c.stats.misses / 4,
			Entries:  volumesVADR,
			HitRatio: hitRatio,
		},
		TokenMeta: interfaces.CacheTierStats{
			TTL:      24 * time.Hour,
			Hits:     c.stats.hits / 4,
			Misses:   c.stats.misses / 4,
			Entries:  tokenMeta,
			HitRatio: hitRatio,
		},
		TotalEntries: int64(len(c.entries)),
	}
}

// Clear removes all entries from cache
func (c *TTLCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	c.entries = make(map[string]*cacheEntry)
	c.stats = cacheStats{}
}

// Stop shuts down the cleanup goroutine
func (c *TTLCache) Stop() {
	close(c.stopCh)
}

// evictLRU removes the least recently used entry (caller must hold write lock)
func (c *TTLCache) evictLRU() {
	if len(c.entries) == 0 {
		return
	}
	
	var oldestKey string
	var oldestTime time.Time = time.Now()
	
	// Find least recently accessed entry
	for key, entry := range c.entries {
		if entry.accessed.Before(oldestTime) {
			oldestTime = entry.accessed
			oldestKey = key
		}
	}
	
	if oldestKey != "" {
		delete(c.entries, oldestKey)
		c.stats.evictions++
	}
}

// cleanup runs periodically to remove expired entries
func (c *TTLCache) cleanup() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()
	
	for {
		select {
		case <-c.stopCh:
			return
		case <-ticker.C:
			c.removeExpired()
		}
	}
}

// removeExpired removes all expired entries
func (c *TTLCache) removeExpired() {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	now := time.Now()
	expiredKeys := make([]string, 0)
	
	// Collect expired keys
	for key, entry := range c.entries {
		if now.After(entry.expires) {
			expiredKeys = append(expiredKeys, key)
		}
	}
	
	// Remove expired entries
	for _, key := range expiredKeys {
		delete(c.entries, key)
	}
	
	c.stats.cleanupRuns++
}