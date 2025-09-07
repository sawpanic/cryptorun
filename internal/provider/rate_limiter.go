package provider

import (
	"context"
	"sync"
	"time"
)

// RateLimiter implements token bucket rate limiting
type RateLimiter struct {
	config      ProviderLimits
	tokens      int
	maxTokens   int
	lastRefill  time.Time
	mu          sync.Mutex
	
	// Metrics
	totalRequests   int64
	blockedRequests int64
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter(limits ProviderLimits) *RateLimiter {
	maxTokens := limits.BurstLimit
	if maxTokens <= 0 {
		maxTokens = limits.RequestsPerSecond * 2 // Default burst to 2x rate
	}
	
	return &RateLimiter{
		config:     limits,
		tokens:     maxTokens,
		maxTokens:  maxTokens,
		lastRefill: time.Now(),
	}
}

// Wait blocks until a token is available or context is cancelled
func (rl *RateLimiter) Wait(ctx context.Context) error {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	
	rl.totalRequests++
	
	// Refill tokens based on time passed
	rl.refillTokens()
	
	// Check if we have tokens available
	if rl.tokens > 0 {
		rl.tokens--
		return nil
	}
	
	// No tokens available - would need to wait
	rl.blockedRequests++
	
	// Calculate wait time until next token
	tokensPerSecond := float64(rl.config.RequestsPerSecond)
	if tokensPerSecond <= 0 {
		tokensPerSecond = 1.0
	}
	
	waitDuration := time.Duration(float64(time.Second) / tokensPerSecond)
	
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(waitDuration):
		// After waiting, try to get token again
		rl.refillTokens()
		if rl.tokens > 0 {
			rl.tokens--
			return nil
		}
		
		// Still no tokens - return rate limit error
		return &ProviderError{
			Provider:    "rate_limiter",
			Code:        ErrCodeRateLimit,
			Message:     "rate limit exceeded",
			RateLimited: true,
			Temporary:   true,
		}
	}
}

// refillTokens adds tokens based on elapsed time
func (rl *RateLimiter) refillTokens() {
	now := time.Now()
	elapsed := now.Sub(rl.lastRefill)
	
	if elapsed <= 0 {
		return
	}
	
	// Calculate tokens to add based on rate
	tokensToAdd := int(float64(elapsed.Seconds()) * float64(rl.config.RequestsPerSecond))
	
	if tokensToAdd > 0 {
		rl.tokens += tokensToAdd
		if rl.tokens > rl.maxTokens {
			rl.tokens = rl.maxTokens
		}
		rl.lastRefill = now
	}
}

// GetStats returns rate limiter statistics
func (rl *RateLimiter) GetStats() RateLimiterStats {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	
	var blockRate float64
	if rl.totalRequests > 0 {
		blockRate = float64(rl.blockedRequests) / float64(rl.totalRequests)
	}
	
	return RateLimiterStats{
		TotalRequests:   rl.totalRequests,
		BlockedRequests: rl.blockedRequests,
		BlockRate:       blockRate,
		AvailableTokens: rl.tokens,
		MaxTokens:       rl.maxTokens,
		RequestsPerSec:  rl.config.RequestsPerSecond,
	}
}

// Reset clears the rate limiter statistics
func (rl *RateLimiter) Reset() {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	
	rl.tokens = rl.maxTokens
	rl.lastRefill = time.Now()
	rl.totalRequests = 0
	rl.blockedRequests = 0
}

// RateLimiterStats provides rate limiter statistics
type RateLimiterStats struct {
	TotalRequests   int64   `json:"total_requests"`
	BlockedRequests int64   `json:"blocked_requests"`
	BlockRate       float64 `json:"block_rate"`
	AvailableTokens int     `json:"available_tokens"`
	MaxTokens       int     `json:"max_tokens"`
	RequestsPerSec  int     `json:"requests_per_sec"`
}

// ProviderCache implements a simple TTL cache
type ProviderCache struct {
	config    CacheConfig
	data      map[string]*cacheEntry
	mu        sync.RWMutex
	
	// Statistics
	hits   int64
	misses int64
}

type cacheEntry struct {
	value     interface{}
	expiresAt time.Time
}

// NewProviderCache creates a new cache instance
func NewProviderCache(config CacheConfig) *ProviderCache {
	cache := &ProviderCache{
		config: config,
		data:   make(map[string]*cacheEntry),
	}
	
	// Start cleanup goroutine if enabled
	if config.Enabled {
		go cache.cleanupExpired()
	}
	
	return cache
}

// Enabled returns whether caching is enabled
func (pc *ProviderCache) Enabled() bool {
	return pc.config.Enabled
}

// Get retrieves a value from the cache
func (pc *ProviderCache) Get(key string) interface{} {
	if !pc.config.Enabled {
		return nil
	}
	
	pc.mu.RLock()
	defer pc.mu.RUnlock()
	
	entry, exists := pc.data[key]
	if !exists {
		pc.misses++
		return nil
	}
	
	// Check expiration
	if time.Now().After(entry.expiresAt) {
		pc.misses++
		delete(pc.data, key)
		return nil
	}
	
	pc.hits++
	return entry.value
}

// Set stores a value in the cache
func (pc *ProviderCache) Set(key string, value interface{}, ttl time.Duration) {
	if !pc.config.Enabled {
		return
	}
	
	pc.mu.Lock()
	defer pc.mu.Unlock()
	
	// Check if we've exceeded max entries
	if len(pc.data) >= pc.config.MaxEntries {
		// Simple eviction: remove oldest entry
		pc.evictOldest()
	}
	
	pc.data[key] = &cacheEntry{
		value:     value,
		expiresAt: time.Now().Add(ttl),
	}
}

// GetHitRate returns cache hit rate
func (pc *ProviderCache) GetHitRate() float64 {
	pc.mu.RLock()
	defer pc.mu.RUnlock()
	
	total := pc.hits + pc.misses
	if total == 0 {
		return 0.0
	}
	
	return float64(pc.hits) / float64(total)
}

// Clear removes all cached entries
func (pc *ProviderCache) Clear() {
	pc.mu.Lock()
	defer pc.mu.Unlock()
	
	pc.data = make(map[string]*cacheEntry)
	pc.hits = 0
	pc.misses = 0
}

// GetStats returns cache statistics
func (pc *ProviderCache) GetStats() CacheStats {
	pc.mu.RLock()
	defer pc.mu.RUnlock()
	
	return CacheStats{
		Enabled:     pc.config.Enabled,
		Hits:        pc.hits,
		Misses:      pc.misses,
		HitRate:     pc.GetHitRate(),
		EntryCount:  len(pc.data),
		MaxEntries:  pc.config.MaxEntries,
		TTL:         pc.config.TTL,
	}
}

// evictOldest removes the oldest cache entry
func (pc *ProviderCache) evictOldest() {
	var oldestKey string
	var oldestTime time.Time
	
	for key, entry := range pc.data {
		if oldestKey == "" || entry.expiresAt.Before(oldestTime) {
			oldestKey = key
			oldestTime = entry.expiresAt
		}
	}
	
	if oldestKey != "" {
		delete(pc.data, oldestKey)
	}
}

// cleanupExpired periodically removes expired entries
func (pc *ProviderCache) cleanupExpired() {
	ticker := time.NewTicker(pc.config.TTL / 2) // Cleanup every half TTL
	defer ticker.Stop()
	
	for range ticker.C {
		pc.mu.Lock()
		now := time.Now()
		
		for key, entry := range pc.data {
			if now.After(entry.expiresAt) {
				delete(pc.data, key)
			}
		}
		
		pc.mu.Unlock()
	}
}

// CacheStats provides cache statistics
type CacheStats struct {
	Enabled    bool          `json:"enabled"`
	Hits       int64         `json:"hits"`
	Misses     int64         `json:"misses"`
	HitRate    float64       `json:"hit_rate"`
	EntryCount int           `json:"entry_count"`
	MaxEntries int           `json:"max_entries"`
	TTL        time.Duration `json:"ttl"`
}