package cache

import (
	"fmt"
	"sync"
	"time"
)

// TTLCache implements a thread-safe cache with time-to-live expiration
type TTLCache struct {
	mu      sync.RWMutex
	items   map[string]*cacheItem
	janitor *janitor
}

// cacheItem represents a single cache entry with expiration
type cacheItem struct {
	value      interface{}
	expiration int64
}

// janitor handles automatic cleanup of expired items
type janitor struct {
	interval time.Duration
	stop     chan bool
}

// CacheStats provides cache performance metrics
type CacheStats struct {
	Hits        int64
	Misses      int64
	Sets        int64
	Deletes     int64
	Evictions   int64
	ItemCount   int
	HitRate     float64
}

// NewTTLCache creates a new TTL cache with specified cleanup interval
func NewTTLCache(cleanupInterval time.Duration) *TTLCache {
	cache := &TTLCache{
		items: make(map[string]*cacheItem),
	}
	
	cache.janitor = &janitor{
		interval: cleanupInterval,
		stop:     make(chan bool),
	}
	
	go cache.janitor.run(cache)
	return cache
}

// Set stores a value with the specified TTL
func (c *TTLCache) Set(key string, value interface{}, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	expiration := time.Now().Add(ttl).UnixNano()
	c.items[key] = &cacheItem{
		value:      value,
		expiration: expiration,
	}
}

// Get retrieves a value from the cache
func (c *TTLCache) Get(key string) (interface{}, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	item, found := c.items[key]
	if !found {
		return nil, false
	}
	
	// Check if item has expired
	if item.expiration > 0 && time.Now().UnixNano() > item.expiration {
		return nil, false
	}
	
	return item.value, true
}

// GetWithTTL retrieves a value and its remaining TTL
func (c *TTLCache) GetWithTTL(key string) (interface{}, time.Duration, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	item, found := c.items[key]
	if !found {
		return nil, 0, false
	}
	
	now := time.Now().UnixNano()
	if item.expiration > 0 && now > item.expiration {
		return nil, 0, false
	}
	
	var remainingTTL time.Duration
	if item.expiration > 0 {
		remainingTTL = time.Duration(item.expiration - now)
	}
	
	return item.value, remainingTTL, true
}

// Delete removes an item from the cache
func (c *TTLCache) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.items, key)
}

// Clear removes all items from the cache
func (c *TTLCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items = make(map[string]*cacheItem)
}

// ItemCount returns the number of items in the cache
func (c *TTLCache) ItemCount() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.items)
}

// Keys returns all non-expired keys in the cache
func (c *TTLCache) Keys() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	now := time.Now().UnixNano()
	keys := make([]string, 0, len(c.items))
	
	for key, item := range c.items {
		if item.expiration == 0 || now <= item.expiration {
			keys = append(keys, key)
		}
	}
	
	return keys
}

// Flush removes all expired items from the cache
func (c *TTLCache) Flush() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	now := time.Now().UnixNano()
	removed := 0
	
	for key, item := range c.items {
		if item.expiration > 0 && now > item.expiration {
			delete(c.items, key)
			removed++
		}
	}
	
	return removed
}

// Close stops the cache janitor and clears all items
func (c *TTLCache) Close() {
	if c.janitor != nil {
		c.janitor.stop <- true
	}
	c.Clear()
}

// GetStats returns cache performance statistics
func (c *TTLCache) GetStats() CacheStats {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	// This is a simplified stats implementation
	// In a production system, you'd track hits/misses/etc.
	itemCount := len(c.items)
	
	return CacheStats{
		ItemCount: itemCount,
		HitRate:   0.0, // Would be calculated from actual hit/miss tracking
	}
}

// janitor cleanup goroutine
func (j *janitor) run(cache *TTLCache) {
	ticker := time.NewTicker(j.interval)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			cache.Flush()
		case <-j.stop:
			return
		}
	}
}

// LayeredCache implements hot/warm/cold cache tiers
type LayeredCache struct {
	hot  *TTLCache  // Fast, short TTL
	warm *TTLCache  // Medium speed, medium TTL  
	cold *TTLCache  // Slower, long TTL
	
	// Configuration
	hotTTL  time.Duration
	warmTTL time.Duration
	coldTTL time.Duration
	
	// Stats tracking
	mu    sync.RWMutex
	stats LayeredStats
}

// LayeredStats tracks performance across cache tiers
type LayeredStats struct {
	HotHits   int64
	WarmHits  int64
	ColdHits  int64
	Misses    int64
	Sets      int64
	Promotes  int64
	HitRate   float64
}

// NewLayeredCache creates a new multi-tier cache
func NewLayeredCache(hotTTL, warmTTL, coldTTL time.Duration) *LayeredCache {
	return &LayeredCache{
		hot:     NewTTLCache(time.Minute),      // Clean every minute
		warm:    NewTTLCache(5 * time.Minute),  // Clean every 5 minutes
		cold:    NewTTLCache(15 * time.Minute), // Clean every 15 minutes
		hotTTL:  hotTTL,
		warmTTL: warmTTL,
		coldTTL: coldTTL,
	}
}

// Set stores a value in the appropriate cache tier
func (lc *LayeredCache) Set(key string, value interface{}, tier CacheTier) {
	lc.mu.Lock()
	lc.stats.Sets++
	lc.mu.Unlock()
	
	switch tier {
	case TierHot:
		lc.hot.Set(key, value, lc.hotTTL)
	case TierWarm:
		lc.warm.Set(key, value, lc.warmTTL)
	case TierCold:
		lc.cold.Set(key, value, lc.coldTTL)
	default:
		lc.hot.Set(key, value, lc.hotTTL)
	}
}

// Get retrieves a value from any cache tier, promoting on hit
func (lc *LayeredCache) Get(key string) (interface{}, CacheTier, bool) {
	// Try hot cache first
	if value, found := lc.hot.Get(key); found {
		lc.mu.Lock()
		lc.stats.HotHits++
		lc.mu.Unlock()
		return value, TierHot, true
	}
	
	// Try warm cache
	if value, found := lc.warm.Get(key); found {
		lc.mu.Lock()
		lc.stats.WarmHits++
		lc.stats.Promotes++
		lc.mu.Unlock()
		
		// Promote to hot
		lc.hot.Set(key, value, lc.hotTTL)
		return value, TierWarm, true
	}
	
	// Try cold cache
	if value, found := lc.cold.Get(key); found {
		lc.mu.Lock()
		lc.stats.ColdHits++
		lc.stats.Promotes++
		lc.mu.Unlock()
		
		// Promote to warm (not hot to avoid polluting hot cache)
		lc.warm.Set(key, value, lc.warmTTL)
		return value, TierCold, true
	}
	
	// Cache miss
	lc.mu.Lock()
	lc.stats.Misses++
	lc.mu.Unlock()
	
	return nil, TierHot, false
}

// GetStats returns layered cache statistics
func (lc *LayeredCache) GetStats() LayeredStats {
	lc.mu.RLock()
	defer lc.mu.RUnlock()
	
	stats := lc.stats
	totalRequests := stats.HotHits + stats.WarmHits + stats.ColdHits + stats.Misses
	if totalRequests > 0 {
		stats.HitRate = float64(stats.HotHits+stats.WarmHits+stats.ColdHits) / float64(totalRequests) * 100.0
	}
	
	return stats
}

// Clear removes all items from all cache tiers
func (lc *LayeredCache) Clear() {
	lc.hot.Clear()
	lc.warm.Clear()
	lc.cold.Clear()
}

// Close shuts down all cache tiers
func (lc *LayeredCache) Close() {
	lc.hot.Close()
	lc.warm.Close()
	lc.cold.Close()
}

// CacheTier represents different cache performance tiers
type CacheTier int

const (
	TierHot CacheTier = iota
	TierWarm
	TierCold
)

// String returns the string representation of a cache tier
func (ct CacheTier) String() string {
	switch ct {
	case TierHot:
		return "hot"
	case TierWarm:
		return "warm"
	case TierCold:
		return "cold"
	default:
		return "unknown"
	}
}

// CacheKey generates a deterministic cache key from components
func CacheKey(components ...string) string {
	if len(components) == 0 {
		return ""
	}
	if len(components) == 1 {
		return components[0]
	}
	
	// Simple key concatenation with separator
	result := components[0]
	for i := 1; i < len(components); i++ {
		result += ":" + components[i]
	}
	return result
}

// CacheKeyWithTimestamp creates a time-bucketed cache key
func CacheKeyWithTimestamp(bucketSize time.Duration, components ...string) string {
	// Round timestamp down to bucket boundary
	now := time.Now()
	bucket := now.Truncate(bucketSize).Unix()
	
	// Add timestamp bucket to key components
	timestampComponent := fmt.Sprintf("t%d", bucket)
	allComponents := append([]string{timestampComponent}, components...)
	
	return CacheKey(allComponents...)
}