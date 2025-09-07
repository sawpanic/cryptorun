// Package data provides Redis-based caching with point-in-time integrity
package data

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// CacheEntry represents a cached item with metadata
type CacheEntry struct {
	Data        interface{} `json:"data"`
	Source      string      `json:"source"`
	CachedAt    time.Time   `json:"cached_at"`
	ExpiresAt   time.Time   `json:"expires_at"`
	PIT         bool        `json:"point_in_time"`
	Attribution string      `json:"attribution"`
}

// CacheManager handles cached data with PIT integrity
type CacheManager interface {
	Get(key string) (interface{}, bool)
	Set(key string, value interface{}, ttl time.Duration) error
	Delete(key string) error

	// PIT snapshots - immutable records with full attribution
	StorePITSnapshot(key string, data interface{}, source string) error
	GetPITSnapshot(key string, timestamp time.Time) (interface{}, bool)

	// Cache statistics
	Stats() CacheStats
	Health() bool
	Close() error
}

// CacheStats provides cache performance metrics
type CacheStats struct {
	HitRate      float64   `json:"hit_rate"`
	TotalHits    int64     `json:"total_hits"`
	TotalMisses  int64     `json:"total_misses"`
	TotalSets    int64     `json:"total_sets"`
	ErrorCount   int64     `json:"error_count"`
	LastError    string    `json:"last_error,omitempty"`
	Connected    bool      `json:"connected"`
	LastPing     time.Time `json:"last_ping"`
	MemoryUsedMB float64   `json:"memory_used_mb"`
}

// RedisCacheManager implements CacheManager using Redis
type RedisCacheManager struct {
	client *redis.Client
	ctx    context.Context
	stats  CacheStats

	// Configuration
	keyPrefix   string
	pitPrefix   string
	maxMemoryMB int64
}

// NewRedisCacheManager creates a new Redis cache manager
func NewRedisCacheManager(addr, password string, db int) *RedisCacheManager {
	client := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,

		// Connection pooling
		PoolSize:     10,
		MinIdleConns: 2,

		// Timeouts
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,

		// Retry settings
		MaxRetries:      3,
		MinRetryBackoff: 100 * time.Millisecond,
		MaxRetryBackoff: 500 * time.Millisecond,
	})

	return &RedisCacheManager{
		client:      client,
		ctx:         context.Background(),
		keyPrefix:   "cryptorun:",
		pitPrefix:   "pit:",
		maxMemoryMB: 512, // 512MB default
		stats: CacheStats{
			Connected: true,
		},
	}
}

// Get retrieves a value from cache
func (r *RedisCacheManager) Get(key string) (interface{}, bool) {
	fullKey := r.keyPrefix + key

	result, err := r.client.Get(r.ctx, fullKey).Result()
	if err != nil {
		if err == redis.Nil {
			r.stats.TotalMisses++
			return nil, false
		}

		r.stats.ErrorCount++
		r.stats.LastError = fmt.Sprintf("Get error: %v", err)
		r.stats.Connected = false
		return nil, false
	}

	// Deserialize the cache entry
	var entry CacheEntry
	if err := json.Unmarshal([]byte(result), &entry); err != nil {
		r.stats.ErrorCount++
		r.stats.LastError = fmt.Sprintf("Deserialize error: %v", err)
		return nil, false
	}

	// Check if expired
	if time.Now().After(entry.ExpiresAt) {
		// Clean up expired entry
		r.Delete(key)
		r.stats.TotalMisses++
		return nil, false
	}

	r.stats.TotalHits++
	r.updateHitRate()
	return entry.Data, true
}

// Set stores a value in cache with TTL
func (r *RedisCacheManager) Set(key string, value interface{}, ttl time.Duration) error {
	fullKey := r.keyPrefix + key

	entry := CacheEntry{
		Data:        value,
		Source:      "cache",
		CachedAt:    time.Now(),
		ExpiresAt:   time.Now().Add(ttl),
		PIT:         false,
		Attribution: "redis_cache",
	}

	data, err := json.Marshal(entry)
	if err != nil {
		r.stats.ErrorCount++
		r.stats.LastError = fmt.Sprintf("Serialize error: %v", err)
		return err
	}

	err = r.client.Set(r.ctx, fullKey, data, ttl).Err()
	if err != nil {
		r.stats.ErrorCount++
		r.stats.LastError = fmt.Sprintf("Set error: %v", err)
		r.stats.Connected = false
		return err
	}

	r.stats.TotalSets++
	r.stats.Connected = true
	return nil
}

// Delete removes a key from cache
func (r *RedisCacheManager) Delete(key string) error {
	fullKey := r.keyPrefix + key
	return r.client.Del(r.ctx, fullKey).Err()
}

// StorePITSnapshot stores immutable point-in-time snapshot
func (r *RedisCacheManager) StorePITSnapshot(key string, data interface{}, source string) error {
	timestamp := time.Now()
	pitKey := fmt.Sprintf("%s%s:%d", r.pitPrefix, key, timestamp.Unix())

	entry := CacheEntry{
		Data:        data,
		Source:      source,
		CachedAt:    timestamp,
		ExpiresAt:   timestamp.Add(24 * time.Hour), // PIT snapshots expire after 24h
		PIT:         true,
		Attribution: fmt.Sprintf("pit_snapshot:%s", source),
	}

	data_bytes, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("failed to serialize PIT snapshot: %w", err)
	}

	// Store with extended TTL for PIT integrity
	err = r.client.Set(r.ctx, pitKey, data_bytes, 24*time.Hour).Err()
	if err != nil {
		return fmt.Errorf("failed to store PIT snapshot: %w", err)
	}

	// Also store a reference with timestamp for easy lookup
	refKey := r.pitPrefix + "refs:" + key
	timestampStr := fmt.Sprintf("%d", timestamp.Unix())
	r.client.ZAdd(r.ctx, refKey, redis.Z{
		Score:  float64(timestamp.Unix()),
		Member: timestampStr,
	})

	// Expire refs after 7 days
	r.client.Expire(r.ctx, refKey, 7*24*time.Hour)

	return nil
}

// GetPITSnapshot retrieves point-in-time snapshot closest to timestamp
func (r *RedisCacheManager) GetPITSnapshot(key string, timestamp time.Time) (interface{}, bool) {
	refKey := r.pitPrefix + "refs:" + key

	// Find closest timestamp <= requested timestamp
	results, err := r.client.ZRevRangeByScore(r.ctx, refKey, &redis.ZRangeBy{
		Min:    "0",
		Max:    fmt.Sprintf("%d", timestamp.Unix()),
		Offset: 0,
		Count:  1,
	}).Result()

	if err != nil || len(results) == 0 {
		return nil, false
	}

	// Get the actual PIT snapshot
	snapshotTimestamp := results[0]
	pitKey := fmt.Sprintf("%s%s:%s", r.pitPrefix, key, snapshotTimestamp)

	result, err := r.client.Get(r.ctx, pitKey).Result()
	if err != nil {
		return nil, false
	}

	var entry CacheEntry
	if err := json.Unmarshal([]byte(result), &entry); err != nil {
		return nil, false
	}

	return entry.Data, true
}

// Stats returns cache performance statistics
func (r *RedisCacheManager) Stats() CacheStats {
	// Update memory usage
	_, err := r.client.Info(r.ctx, "memory").Result()
	if err == nil {
		// Parse memory usage from Redis INFO output
		// This is a simplified implementation
		r.stats.MemoryUsedMB = 0 // Would parse actual memory usage
	}

	r.updateHitRate()
	return r.stats
}

// Health checks Redis connection health
func (r *RedisCacheManager) Health() bool {
	pong, err := r.client.Ping(r.ctx).Result()
	if err != nil || pong != "PONG" {
		r.stats.Connected = false
		r.stats.ErrorCount++
		r.stats.LastError = fmt.Sprintf("Health check failed: %v", err)
		return false
	}

	r.stats.Connected = true
	r.stats.LastPing = time.Now()
	return true
}

// Close closes the Redis connection
func (r *RedisCacheManager) Close() error {
	return r.client.Close()
}

// updateHitRate calculates current hit rate
func (r *RedisCacheManager) updateHitRate() {
	total := r.stats.TotalHits + r.stats.TotalMisses
	if total > 0 {
		r.stats.HitRate = float64(r.stats.TotalHits) / float64(total)
	}
}

// InMemoryCacheManager provides a simple in-memory cache for testing
type InMemoryCacheManager struct {
	data    map[string]CacheEntry
	pitData map[string]map[int64]CacheEntry // key -> timestamp -> entry
	stats   CacheStats
}

// NewInMemoryCacheManager creates an in-memory cache for testing
func NewInMemoryCacheManager() *InMemoryCacheManager {
	return &InMemoryCacheManager{
		data:    make(map[string]CacheEntry),
		pitData: make(map[string]map[int64]CacheEntry),
		stats: CacheStats{
			Connected: true,
			LastPing:  time.Now(),
		},
	}
}

// Get retrieves value from in-memory cache
func (m *InMemoryCacheManager) Get(key string) (interface{}, bool) {
	entry, exists := m.data[key]
	if !exists {
		m.stats.TotalMisses++
		m.updateHitRate()
		return nil, false
	}

	// Check expiration
	if time.Now().After(entry.ExpiresAt) {
		delete(m.data, key)
		m.stats.TotalMisses++
		m.updateHitRate()
		return nil, false
	}

	m.stats.TotalHits++
	m.updateHitRate()
	return entry.Data, true
}

// Set stores value in in-memory cache
func (m *InMemoryCacheManager) Set(key string, value interface{}, ttl time.Duration) error {
	entry := CacheEntry{
		Data:        value,
		Source:      "memory",
		CachedAt:    time.Now(),
		ExpiresAt:   time.Now().Add(ttl),
		PIT:         false,
		Attribution: "in_memory_cache",
	}

	m.data[key] = entry
	m.stats.TotalSets++
	return nil
}

// Delete removes key from in-memory cache
func (m *InMemoryCacheManager) Delete(key string) error {
	delete(m.data, key)
	return nil
}

// StorePITSnapshot stores PIT snapshot in memory
func (m *InMemoryCacheManager) StorePITSnapshot(key string, data interface{}, source string) error {
	timestamp := time.Now()
	entry := CacheEntry{
		Data:        data,
		Source:      source,
		CachedAt:    timestamp,
		ExpiresAt:   timestamp.Add(24 * time.Hour),
		PIT:         true,
		Attribution: fmt.Sprintf("pit_snapshot:%s", source),
	}

	if m.pitData[key] == nil {
		m.pitData[key] = make(map[int64]CacheEntry)
	}

	m.pitData[key][timestamp.Unix()] = entry
	return nil
}

// GetPITSnapshot retrieves PIT snapshot from memory
func (m *InMemoryCacheManager) GetPITSnapshot(key string, timestamp time.Time) (interface{}, bool) {
	keyData, exists := m.pitData[key]
	if !exists {
		return nil, false
	}

	// Find closest timestamp <= requested
	target := timestamp.Unix()
	var closestTime int64
	var found bool

	for t := range keyData {
		if t <= target && t > closestTime {
			closestTime = t
			found = true
		}
	}

	if !found {
		return nil, false
	}

	entry := keyData[closestTime]

	// Check expiration
	if time.Now().After(entry.ExpiresAt) {
		delete(keyData, closestTime)
		return nil, false
	}

	return entry.Data, true
}

// Stats returns in-memory cache stats
func (m *InMemoryCacheManager) Stats() CacheStats {
	m.updateHitRate()

	// Estimate memory usage (rough calculation)
	m.stats.MemoryUsedMB = float64(len(m.data)+len(m.pitData)) * 0.001 // 1KB per entry estimate

	return m.stats
}

// Health always returns true for in-memory cache
func (m *InMemoryCacheManager) Health() bool {
	m.stats.LastPing = time.Now()
	return true
}

// Close is a no-op for in-memory cache
func (m *InMemoryCacheManager) Close() error {
	return nil
}

// updateHitRate calculates hit rate for in-memory cache
func (m *InMemoryCacheManager) updateHitRate() {
	total := m.stats.TotalHits + m.stats.TotalMisses
	if total > 0 {
		m.stats.HitRate = float64(m.stats.TotalHits) / float64(total)
	}
}
