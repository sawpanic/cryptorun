package data

import (
	"context"
	"fmt"
	"log"
	"time"
)

// Bridge orchestrates data retrieval across Hot/Warm/Cold tiers
type Bridge struct {
	hot  HotTier
	warm WarmTier
	cold ColdTier

	// Configuration
	maxAgeHotMS    int64 // Maximum age for hot data (default: 5000ms)
	maxAgeWarmMS   int64 // Maximum age for warm data (default: 60000ms)
	enableFallback bool  // Enable tier fallback (default: true)
}

// TierInterface defines common interface for all data tiers
type TierInterface interface {
	GetOrderBook(ctx context.Context, venue, symbol string) (*Envelope, error)
	GetPriceData(ctx context.Context, venue, symbol string) (*Envelope, error)
	IsAvailable(ctx context.Context, venue string) bool
}

// HotTier interface for WebSocket real-time data
type HotTier interface {
	TierInterface
	Subscribe(venue, symbol string) error
	Unsubscribe(venue, symbol string) error
	GetLatestTick(venue, symbol string) (*Envelope, error)
}

// WarmTier interface for REST API + cache data
type WarmTier interface {
	TierInterface
	SetCacheTTL(venue string, ttlSeconds int)
	InvalidateCache(venue, symbol string) error
	GetCacheStats() CacheStats
}

// ColdTier interface for historical file data
type ColdTier interface {
	TierInterface
	GetHistoricalSlice(ctx context.Context, venue, symbol string, start, end time.Time) ([]*Envelope, error)
	LoadFromFile(filePath string) error
}

// CacheStats provides cache performance metrics
type CacheStats struct {
	HitRate     float64   `json:"hit_rate"`
	MissCount   int64     `json:"miss_count"`
	ErrorCount  int64     `json:"error_count"`
	LastUpdated time.Time `json:"last_updated"`
}

// BridgeConfig configures the bridge behavior
type BridgeConfig struct {
	MaxAgeHotMS    int64 `json:"max_age_hot_ms"`
	MaxAgeWarmMS   int64 `json:"max_age_warm_ms"`
	EnableFallback bool  `json:"enable_fallback"`
}

// DefaultBridgeConfig returns sensible defaults
func DefaultBridgeConfig() BridgeConfig {
	return BridgeConfig{
		MaxAgeHotMS:    5000,  // 5 seconds
		MaxAgeWarmMS:   60000, // 60 seconds
		EnableFallback: true,
	}
}

// NewBridge creates a new data bridge orchestrator
func NewBridge(hot HotTier, warm WarmTier, cold ColdTier, config BridgeConfig) *Bridge {
	return &Bridge{
		hot:            hot,
		warm:           warm,
		cold:           cold,
		maxAgeHotMS:    config.MaxAgeHotMS,
		maxAgeWarmMS:   config.MaxAgeWarmMS,
		enableFallback: config.EnableFallback,
	}
}

// GetOrderBook retrieves order book with tier cascade
func (b *Bridge) GetOrderBook(ctx context.Context, venue, symbol string) (*Envelope, error) {
	return b.cascadeGet(ctx, venue, symbol, func(tier TierInterface) (*Envelope, error) {
		return tier.GetOrderBook(ctx, venue, symbol)
	})
}

// GetPriceData retrieves price data with tier cascade
func (b *Bridge) GetPriceData(ctx context.Context, venue, symbol string) (*Envelope, error) {
	return b.cascadeGet(ctx, venue, symbol, func(tier TierInterface) (*Envelope, error) {
		return tier.GetPriceData(ctx, venue, symbol)
	})
}

// cascadeGet implements the "worst feed wins" cascade logic
func (b *Bridge) cascadeGet(ctx context.Context, venue, symbol string, getter func(TierInterface) (*Envelope, error)) (*Envelope, error) {
	var fallbackChain []string
	var lastErr error

	// Try Hot tier first
	if b.hot != nil && b.hot.IsAvailable(ctx, venue) {
		envelope, err := getter(b.hot)
		if err == nil && !envelope.IsStale(b.maxAgeHotMS) {
			log.Printf("[BRIDGE] Hot tier success: %s %s (freshness: %dms)", venue, symbol, envelope.FreshnessMS)
			return envelope, nil
		}
		if err != nil {
			lastErr = err
			fallbackChain = append(fallbackChain, fmt.Sprintf("hot_failed:%s", err.Error()))
		} else {
			fallbackChain = append(fallbackChain, fmt.Sprintf("hot_stale:%dms", envelope.FreshnessMS))
		}
	}

	// Fallback to Warm tier
	if b.enableFallback && b.warm != nil && b.warm.IsAvailable(ctx, venue) {
		envelope, err := getter(b.warm)
		if err == nil && !envelope.IsStale(b.maxAgeWarmMS) {
			envelope.Provenance.FallbackChain = fallbackChain
			envelope.SourceTier = TierWarm // Ensure correct tier marking
			log.Printf("[BRIDGE] Warm tier fallback success: %s %s (freshness: %dms)", venue, symbol, envelope.FreshnessMS)
			return envelope, nil
		}
		if err != nil {
			lastErr = err
			fallbackChain = append(fallbackChain, fmt.Sprintf("warm_failed:%s", err.Error()))
		} else {
			fallbackChain = append(fallbackChain, fmt.Sprintf("warm_stale:%dms", envelope.FreshnessMS))
		}
	}

	// Final fallback to Cold tier (no freshness check - historical data)
	if b.enableFallback && b.cold != nil && b.cold.IsAvailable(ctx, venue) {
		envelope, err := getter(b.cold)
		if err == nil {
			envelope.Provenance.FallbackChain = fallbackChain
			envelope.SourceTier = TierCold // Ensure correct tier marking
			log.Printf("[BRIDGE] Cold tier fallback success: %s %s", venue, symbol)
			return envelope, nil
		}
		lastErr = err
		fallbackChain = append(fallbackChain, fmt.Sprintf("cold_failed:%s", err.Error()))
	}

	// All tiers failed
	log.Printf("[BRIDGE] All tiers failed for %s %s: %v (chain: %v)", venue, symbol, lastErr, fallbackChain)
	return nil, fmt.Errorf("all data tiers failed for %s %s: %w (fallback_chain: %v)", venue, symbol, lastErr, fallbackChain)
}

// GetBestAvailableSource returns the highest authority source currently available
func (b *Bridge) GetBestAvailableSource(ctx context.Context, venue string) SourceTier {
	if b.hot != nil && b.hot.IsAvailable(ctx, venue) {
		return TierHot
	}
	if b.warm != nil && b.warm.IsAvailable(ctx, venue) {
		return TierWarm
	}
	if b.cold != nil && b.cold.IsAvailable(ctx, venue) {
		return TierCold
	}
	return ""
}

// ValidateSourceAuthority ensures higher authority sources aren't overwritten by lower ones
func (b *Bridge) ValidateSourceAuthority(existing, incoming *Envelope) bool {
	if existing == nil {
		return true // No existing data, accept incoming
	}

	// "Worst feed wins" - only accept if incoming has equal or higher authority
	return incoming.GetSourceAuthority() >= existing.GetSourceAuthority()
}

// GetHealthStatus returns health status of all tiers
func (b *Bridge) GetHealthStatus(ctx context.Context) map[string]interface{} {
	status := make(map[string]interface{})

	// Check Hot tier
	if b.hot != nil {
		status["hot"] = map[string]interface{}{
			"available": b.hot.IsAvailable(ctx, "binance"), // Test with common venue
			"tier":      "hot",
		}
	}

	// Check Warm tier
	if b.warm != nil {
		status["warm"] = map[string]interface{}{
			"available":   b.warm.IsAvailable(ctx, "binance"),
			"tier":        "warm",
			"cache_stats": b.warm.GetCacheStats(),
		}
	}

	// Check Cold tier
	if b.cold != nil {
		status["cold"] = map[string]interface{}{
			"available": b.cold.IsAvailable(ctx, "binance"),
			"tier":      "cold",
		}
	}

	status["config"] = map[string]interface{}{
		"max_age_hot_ms":  b.maxAgeHotMS,
		"max_age_warm_ms": b.maxAgeWarmMS,
		"enable_fallback": b.enableFallback,
	}

	return status
}
