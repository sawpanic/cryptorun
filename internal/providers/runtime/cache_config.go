package runtime

import (
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

// CacheConfig defines provider-specific cache configurations
type CacheConfig struct {
	Provider           string            `yaml:"provider"`
	WarmCacheTTL       time.Duration     `yaml:"warm_cache_ttl"`
	HotCacheTTL        time.Duration     `yaml:"hot_cache_ttl"`
	ColdCacheTTL       time.Duration     `yaml:"cold_cache_ttl"`
	MaxSize            int               `yaml:"max_size"`
	PrefixMap          map[string]string `yaml:"prefix_map"`
	CompressionEnabled bool              `yaml:"compression_enabled"`
	DegradedTTL        time.Duration     `yaml:"degraded_ttl"` // Extended TTL during degradation
}

// Cache tier configurations per provider
var CacheConfigs = map[string]CacheConfig{
	"binance": {
		Provider:     "binance",
		WarmCacheTTL: time.Minute * 5, // 5-min cache for warm set
		HotCacheTTL:  time.Second * 30,
		ColdCacheTTL: time.Hour * 24,
		MaxSize:      10000,
		DegradedTTL:  time.Minute * 10, // Double cache on degradation
		PrefixMap: map[string]string{
			"ticker": "binance:ticker:",
			"depth":  "binance:depth:",
			"trades": "binance:trades:",
			"klines": "binance:klines:",
		},
		CompressionEnabled: true,
	},
	"dexscreener": {
		Provider:     "dexscreener",
		WarmCacheTTL: time.Minute * 5,
		HotCacheTTL:  time.Minute * 2,
		ColdCacheTTL: time.Hour * 12,
		MaxSize:      5000,
		DegradedTTL:  time.Minute * 10,
		PrefixMap: map[string]string{
			"pairs":  "dex:pairs:",
			"search": "dex:search:",
			"latest": "dex:latest:",
		},
		CompressionEnabled: true,
	},
	"coingecko": {
		Provider:     "coingecko",
		WarmCacheTTL: time.Minute * 5,
		HotCacheTTL:  time.Minute * 3,
		ColdCacheTTL: time.Hour * 6,
		MaxSize:      3000,
		DegradedTTL:  time.Minute * 15,
		PrefixMap: map[string]string{
			"price":    "cg:price:",
			"market":   "cg:market:",
			"trending": "cg:trending:",
		},
		CompressionEnabled: false, // JSON responses are small
	},
	"moralis": {
		Provider:     "moralis",
		WarmCacheTTL: time.Minute * 5,
		HotCacheTTL:  time.Minute * 5,
		ColdCacheTTL: time.Hour * 24,
		MaxSize:      2000,
		DegradedTTL:  time.Minute * 20,
		PrefixMap: map[string]string{
			"token": "moralis:token:",
			"nft":   "moralis:nft:",
			"defi":  "moralis:defi:",
		},
		CompressionEnabled: true,
	},
	"cmc": {
		Provider:     "cmc",
		WarmCacheTTL: time.Minute * 5,
		HotCacheTTL:  time.Minute * 5,
		ColdCacheTTL: time.Hour * 12,
		MaxSize:      3000,
		DegradedTTL:  time.Minute * 25,
		PrefixMap: map[string]string{
			"listings": "cmc:listings:",
			"quotes":   "cmc:quotes:",
			"metadata": "cmc:metadata:",
		},
		CompressionEnabled: true,
	},
	"etherscan": {
		Provider:     "etherscan",
		WarmCacheTTL: time.Minute * 5,
		HotCacheTTL:  time.Minute * 10,
		ColdCacheTTL: time.Hour * 48,
		MaxSize:      1000,
		DegradedTTL:  time.Minute * 30,
		PrefixMap: map[string]string{
			"account":  "etherscan:account:",
			"contract": "etherscan:contract:",
			"token":    "etherscan:token:",
		},
		CompressionEnabled: false, // API responses relatively small
	},
	"paprika": {
		Provider:     "paprika",
		WarmCacheTTL: time.Minute * 5,
		HotCacheTTL:  time.Minute * 2,
		ColdCacheTTL: time.Hour * 8,
		MaxSize:      4000,
		DegradedTTL:  time.Minute * 12,
		PrefixMap: map[string]string{
			"coins":     "paprika:coins:",
			"tickers":   "paprika:tickers:",
			"exchanges": "paprika:exchanges:",
		},
		CompressionEnabled: true,
	},
}

// CacheManager manages provider-specific caching with degradation support
type CacheManager struct {
	mu        sync.RWMutex
	config    CacheConfig
	degraded  bool
	cacheHits int64
	cacheMiss int64
	entries   map[string]CacheEntry
}

// CacheEntry represents a cached item
type CacheEntry struct {
	Data       []byte
	ExpiresAt  time.Time
	Tier       CacheTier
	Compressed bool
}

// CacheTier represents different cache tiers
type CacheTier int

const (
	TierHot CacheTier = iota
	TierWarm
	TierCold
)

func (t CacheTier) String() string {
	switch t {
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

// NewCacheManager creates a provider-specific cache manager
func NewCacheManager(provider string) *CacheManager {
	config, exists := CacheConfigs[provider]
	if !exists {
		log.Warn().Str("provider", provider).Msg("Unknown provider, using default cache config")
		config = CacheConfig{
			Provider:     provider,
			WarmCacheTTL: time.Minute * 5,
			HotCacheTTL:  time.Minute * 1,
			ColdCacheTTL: time.Hour * 6,
			MaxSize:      1000,
			DegradedTTL:  time.Minute * 10,
			PrefixMap: map[string]string{
				"default": provider + ":default:",
			},
		}
	}

	return &CacheManager{
		config:  config,
		entries: make(map[string]CacheEntry),
	}
}

// Get retrieves item from cache
func (cm *CacheManager) Get(key string) ([]byte, bool) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	entry, exists := cm.entries[key]
	if !exists {
		cm.cacheMiss++
		return nil, false
	}

	if time.Now().After(entry.ExpiresAt) {
		// Entry expired
		cm.cacheMiss++
		delete(cm.entries, key)
		return nil, false
	}

	cm.cacheHits++

	// Decompress if needed
	data := entry.Data
	if entry.Compressed && cm.config.CompressionEnabled {
		// Placeholder for decompression logic
		// In real implementation, use gzip or similar
		data = entry.Data
	}

	return data, true
}

// Set stores item in cache with appropriate TTL based on tier
func (cm *CacheManager) Set(key string, data []byte, tier CacheTier) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	// Check size limits
	if len(cm.entries) >= cm.config.MaxSize {
		cm.evictOldest()
	}

	// Determine TTL based on tier and degradation status
	var ttl time.Duration
	switch tier {
	case TierHot:
		ttl = cm.config.HotCacheTTL
	case TierWarm:
		ttl = cm.config.WarmCacheTTL
	case TierCold:
		ttl = cm.config.ColdCacheTTL
	}

	// Extend TTL if provider is degraded
	if cm.degraded {
		ttl = cm.config.DegradedTTL
	}

	// Compress if enabled and data is large enough
	compressed := false
	if cm.config.CompressionEnabled && len(data) > 1024 {
		// Placeholder for compression logic
		// In real implementation, use gzip
		compressed = true
	}

	entry := CacheEntry{
		Data:       data,
		ExpiresAt:  time.Now().Add(ttl),
		Tier:       tier,
		Compressed: compressed,
	}

	cm.entries[key] = entry

	log.Debug().
		Str("provider", cm.config.Provider).
		Str("key", key).
		Str("tier", tier.String()).
		Dur("ttl", ttl).
		Bool("compressed", compressed).
		Msg("Cached entry")
}

// SetDegraded enables degraded mode with extended cache TTLs
func (cm *CacheManager) SetDegraded(degraded bool) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if cm.degraded != degraded {
		cm.degraded = degraded
		status := "normal"
		if degraded {
			status = "degraded"
		}

		log.Info().
			Str("provider", cm.config.Provider).
			Str("status", status).
			Dur("degraded_ttl", cm.config.DegradedTTL).
			Msg("Cache degradation mode changed")
	}
}

// BuildKey constructs cache key with provider prefix
func (cm *CacheManager) BuildKey(keyType, identifier string) string {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	prefix, exists := cm.config.PrefixMap[keyType]
	if !exists {
		prefix = cm.config.Provider + ":default:"
	}

	return prefix + identifier
}

// GetStats returns cache statistics
func (cm *CacheManager) GetStats() map[string]interface{} {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	totalRequests := cm.cacheHits + cm.cacheMiss
	hitRate := 0.0
	if totalRequests > 0 {
		hitRate = float64(cm.cacheHits) / float64(totalRequests)
	}

	// Count entries by tier
	tierCounts := map[string]int{
		"hot":  0,
		"warm": 0,
		"cold": 0,
	}

	for _, entry := range cm.entries {
		tierCounts[entry.Tier.String()]++
	}

	return map[string]interface{}{
		"provider":    cm.config.Provider,
		"degraded":    cm.degraded,
		"entries":     len(cm.entries),
		"max_size":    cm.config.MaxSize,
		"cache_hits":  cm.cacheHits,
		"cache_miss":  cm.cacheMiss,
		"hit_rate":    hitRate,
		"tier_counts": tierCounts,
		"config": map[string]interface{}{
			"warm_ttl":     cm.config.WarmCacheTTL.String(),
			"hot_ttl":      cm.config.HotCacheTTL.String(),
			"cold_ttl":     cm.config.ColdCacheTTL.String(),
			"degraded_ttl": cm.config.DegradedTTL.String(),
		},
	}
}

// Cleanup removes expired entries
func (cm *CacheManager) Cleanup() {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	now := time.Now()
	removed := 0

	for key, entry := range cm.entries {
		if now.After(entry.ExpiresAt) {
			delete(cm.entries, key)
			removed++
		}
	}

	if removed > 0 {
		log.Debug().
			Str("provider", cm.config.Provider).
			Int("removed", removed).
			Msg("Cache cleanup completed")
	}
}

// Private helper methods
func (cm *CacheManager) evictOldest() {
	// Simple eviction: remove first expired entry or oldest entry
	var oldestKey string
	var oldestTime time.Time

	now := time.Now()
	for key, entry := range cm.entries {
		if now.After(entry.ExpiresAt) {
			// Remove expired entry immediately
			delete(cm.entries, key)
			return
		}

		if oldestKey == "" || entry.ExpiresAt.Before(oldestTime) {
			oldestKey = key
			oldestTime = entry.ExpiresAt
		}
	}

	if oldestKey != "" {
		delete(cm.entries, oldestKey)
		log.Debug().Str("key", oldestKey).Msg("Evicted oldest cache entry")
	}
}
