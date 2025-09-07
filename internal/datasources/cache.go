package datasources

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// CacheConfig defines TTL settings for different data categories
type CacheConfig struct {
	Category string        `json:"category"`
	TTL      time.Duration `json:"ttl"`
}

// CacheEntry represents a cached data entry
type CacheEntry struct {
	Data      interface{} `json:"data"`
	Timestamp time.Time   `json:"timestamp"`
	TTL       time.Duration
}

// CacheManager manages in-memory cache with configurable TTLs
type CacheManager struct {
	entries map[string]*CacheEntry
	config  map[string]time.Duration
	mu      sync.RWMutex
}

// Default cache configurations based on playbook specifications
var DefaultCacheConfig = map[string]time.Duration{
	// Hot data - real-time streams
	"ws_stream":  0 * time.Second,  // Never cache WebSocket streams
	"order_book": 5 * time.Second,  // Very short for orderbook
	"trades":     10 * time.Second, // Short for trade data

	// Warm data - REST APIs with moderate freshness needs
	"price_current": 30 * time.Second,  // Current price data
	"volume_24h":    60 * time.Second,  // Volume data
	"market_data":   120 * time.Second, // General market data
	"pair_info":     300 * time.Second, // Trading pair information

	// Cold data - slower changing information
	"exchange_info": 1800 * time.Second,  // 30 minutes for exchange info
	"asset_info":    3600 * time.Second,  // 1 hour for asset information
	"historical":    7200 * time.Second,  // 2 hours for historical data
	"metadata":      21600 * time.Second, // 6 hours for metadata
}

// Provider-specific cache overrides
var ProviderCacheOverrides = map[string]map[string]time.Duration{
	"binance": {
		"exchange_info": 3600 * time.Second, // Binance exchange info changes less frequently
		"klines":        300 * time.Second,  // Kline data
	},
	"coingecko": {
		"coin_list":   7200 * time.Second, // CoinGecko coin list
		"market_data": 180 * time.Second,  // Market data from CoinGecko
	},
	"kraken": {
		"asset_pairs": 1800 * time.Second, // Kraken asset pairs
		"server_time": 60 * time.Second,   // Server time
	},
	"dexscreener": {
		"token_info": 600 * time.Second, // DEXScreener token info
		"pool_data":  120 * time.Second, // Pool data
	},
}

// NewCacheManager creates a new cache manager with default configuration
func NewCacheManager() *CacheManager {
	cm := &CacheManager{
		entries: make(map[string]*CacheEntry),
		config:  make(map[string]time.Duration),
	}

	// Load default configuration
	for category, ttl := range DefaultCacheConfig {
		cm.config[category] = ttl
	}

	return cm
}

// SetTTL updates the TTL for a specific cache category
func (cm *CacheManager) SetTTL(category string, ttl time.Duration) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.config[category] = ttl
}

// GetTTL returns the TTL for a specific cache category
func (cm *CacheManager) GetTTL(category string) time.Duration {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	if ttl, exists := cm.config[category]; exists {
		return ttl
	}

	// Default TTL if category not found
	return 300 * time.Second
}

// Set stores data in cache with the specified category's TTL
func (cm *CacheManager) Set(key, category string, data interface{}) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	ttl := cm.getTTLUnsafe(category)

	// Don't cache if TTL is 0 (like WebSocket streams)
	if ttl == 0 {
		return nil
	}

	cm.entries[key] = &CacheEntry{
		Data:      data,
		Timestamp: time.Now(),
		TTL:       ttl,
	}

	return nil
}

// Get retrieves data from cache if not expired
func (cm *CacheManager) Get(key string) (interface{}, bool) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	entry, exists := cm.entries[key]
	if !exists {
		return nil, false
	}

	// Check if entry has expired
	if time.Since(entry.Timestamp) > entry.TTL {
		// Entry expired, clean it up
		go cm.Delete(key)
		return nil, false
	}

	return entry.Data, true
}

// GetWithTTL retrieves data from cache along with remaining TTL
func (cm *CacheManager) GetWithTTL(key string) (interface{}, time.Duration, bool) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	entry, exists := cm.entries[key]
	if !exists {
		return nil, 0, false
	}

	elapsed := time.Since(entry.Timestamp)
	if elapsed > entry.TTL {
		go cm.Delete(key)
		return nil, 0, false
	}

	remaining := entry.TTL - elapsed
	return entry.Data, remaining, true
}

// Delete removes an entry from cache
func (cm *CacheManager) Delete(key string) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	delete(cm.entries, key)
}

// Clear removes all entries from cache
func (cm *CacheManager) Clear() {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.entries = make(map[string]*CacheEntry)
}

// CleanExpired removes all expired entries from cache
func (cm *CacheManager) CleanExpired() int {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	cleaned := 0
	now := time.Now()

	for key, entry := range cm.entries {
		if now.Sub(entry.Timestamp) > entry.TTL {
			delete(cm.entries, key)
			cleaned++
		}
	}

	return cleaned
}

// Stats returns cache statistics
func (cm *CacheManager) Stats() CacheStats {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	stats := CacheStats{
		TotalEntries: len(cm.entries),
		Categories:   make(map[string]int),
	}

	now := time.Now()
	for _, entry := range cm.entries {
		if now.Sub(entry.Timestamp) <= entry.TTL {
			stats.ActiveEntries++
		} else {
			stats.ExpiredEntries++
		}
	}

	return stats
}

// CacheStats represents cache statistics
type CacheStats struct {
	TotalEntries   int            `json:"total_entries"`
	ActiveEntries  int            `json:"active_entries"`
	ExpiredEntries int            `json:"expired_entries"`
	Categories     map[string]int `json:"categories"`
}

// StartCleanupWorker starts a background goroutine to periodically clean expired entries
func (cm *CacheManager) StartCleanupWorker(interval time.Duration) {
	ticker := time.NewTicker(interval)
	go func() {
		for range ticker.C {
			cm.CleanExpired()
		}
	}()
}

// BuildKey creates a cache key from provider, endpoint, and parameters
func (cm *CacheManager) BuildKey(provider, endpoint string, params map[string]string) string {
	key := fmt.Sprintf("%s:%s", provider, endpoint)

	if len(params) > 0 {
		// Sort parameters for consistent keys
		paramBytes, _ := json.Marshal(params)
		key += ":" + string(paramBytes)
	}

	return key
}

// GetProviderTTL returns TTL for a specific provider and category combination
func (cm *CacheManager) GetProviderTTL(provider, category string) time.Duration {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	// Check for provider-specific override first
	if providerOverrides, exists := ProviderCacheOverrides[provider]; exists {
		if ttl, exists := providerOverrides[category]; exists {
			return ttl
		}
	}

	// Fall back to default category TTL
	return cm.getTTLUnsafe(category)
}

// SetProviderData stores data with provider-specific TTL
func (cm *CacheManager) SetProviderData(provider, endpoint, category string, params map[string]string, data interface{}) error {
	key := cm.BuildKey(provider, endpoint, params)
	ttl := cm.GetProviderTTL(provider, category)

	cm.mu.Lock()
	defer cm.mu.Unlock()

	// Don't cache if TTL is 0
	if ttl == 0 {
		return nil
	}

	cm.entries[key] = &CacheEntry{
		Data:      data,
		Timestamp: time.Now(),
		TTL:       ttl,
	}

	return nil
}

func (cm *CacheManager) getTTLUnsafe(category string) time.Duration {
	if ttl, exists := cm.config[category]; exists {
		return ttl
	}
	return 300 * time.Second // 5 minute default
}
