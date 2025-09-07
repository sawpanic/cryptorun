package data

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestEnvelope_GenerateChecksum(t *testing.T) {
	// Create envelope
	envelope := NewEnvelope("binance", "BTCUSD", TierHot)

	// Test checksum generation
	value := map[string]interface{}{
		"price": 50000.0,
		"qty":   1.5,
	}

	checksum1 := envelope.GenerateChecksum(value, "order_book")
	checksum2 := envelope.GenerateChecksum(value, "order_book")

	// Same input should produce same checksum
	assert.Equal(t, checksum1, checksum2)
	assert.Len(t, checksum1, 64) // SHA256 hex length

	// Different values should produce different checksums
	value2 := map[string]interface{}{
		"price": 50001.0,
		"qty":   1.5,
	}
	checksum3 := envelope.GenerateChecksum(value2, "order_book")
	assert.NotEqual(t, checksum1, checksum3)

	// Different units should produce different checksums
	checksum4 := envelope.GenerateChecksum(value, "price_tick")
	assert.NotEqual(t, checksum1, checksum4)
}

func TestEnvelope_IsStale(t *testing.T) {
	envelope := NewEnvelope("binance", "BTCUSD", TierHot)

	// Fresh data
	envelope.FreshnessMS = 1000             // 1 second
	assert.False(t, envelope.IsStale(5000)) // 5 second limit

	// Stale data
	envelope.FreshnessMS = 10000           // 10 seconds
	assert.True(t, envelope.IsStale(5000)) // 5 second limit
}

func TestEnvelope_CalculateFreshness(t *testing.T) {
	// Create envelope with timestamp 5 seconds ago
	envelope := NewEnvelope("binance", "BTCUSD", TierHot)
	envelope.Timestamp = time.Now().Add(-5 * time.Second)

	// Calculate freshness
	envelope.CalculateFreshness()

	// Should be approximately 5000ms (allow some tolerance)
	assert.InDelta(t, 5000, envelope.FreshnessMS, 100) // 100ms tolerance
}

func TestEnvelope_IsFallback(t *testing.T) {
	envelope := NewEnvelope("binance", "BTCUSD", TierHot)

	// No fallback initially
	assert.False(t, envelope.IsFallback())

	// Add fallback chain
	envelope.Provenance.FallbackChain = []string{"hot_failed:error"}
	assert.True(t, envelope.IsFallback())
}

func TestEnvelope_GetSourceAuthority(t *testing.T) {
	hotEnvelope := NewEnvelope("binance", "BTCUSD", TierHot)
	warmEnvelope := NewEnvelope("binance", "BTCUSD", TierWarm)
	coldEnvelope := NewEnvelope("binance", "BTCUSD", TierCold)

	assert.Equal(t, 3, hotEnvelope.GetSourceAuthority())
	assert.Equal(t, 2, warmEnvelope.GetSourceAuthority())
	assert.Equal(t, 1, coldEnvelope.GetSourceAuthority())

	// Test invalid tier
	invalidEnvelope := &Envelope{SourceTier: SourceTier("invalid")}
	assert.Equal(t, 0, invalidEnvelope.GetSourceAuthority())
}

func TestEnvelopeOptions(t *testing.T) {
	// Test with options
	fallbackChain := []string{"hot_failed", "warm_stale"}
	envelope := NewEnvelope("binance", "BTCUSD", TierCold,
		WithFallbackChain(fallbackChain),
		WithCacheHit(true),
		WithConfidenceScore(0.8),
	)

	assert.Equal(t, fallbackChain, envelope.Provenance.FallbackChain)
	assert.True(t, envelope.Provenance.CacheHit)
	assert.Equal(t, 0.8, envelope.Provenance.ConfidenceScore)
}

func TestProvenanceInfo(t *testing.T) {
	envelope := NewEnvelope("binance", "BTCUSD", TierWarm)

	// Test provenance fields
	assert.NotZero(t, envelope.Provenance.RetrievedAt)
	assert.Equal(t, 1.0, envelope.Provenance.ConfidenceScore) // Default confidence
	assert.Zero(t, envelope.Provenance.LatencyMS)
	assert.Zero(t, envelope.Provenance.RetryCount)
	assert.Empty(t, envelope.Provenance.FallbackChain)
}

func TestSourceTier(t *testing.T) {
	// Test tier constants
	assert.Equal(t, SourceTier("hot"), TierHot)
	assert.Equal(t, SourceTier("warm"), TierWarm)
	assert.Equal(t, SourceTier("cold"), TierCold)

	// Test tier assignment
	envelope := NewEnvelope("binance", "BTCUSD", TierHot)
	assert.Equal(t, TierHot, envelope.SourceTier)
}
