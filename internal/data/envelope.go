package data

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"
)

// SourceTier identifies the data layer source
type SourceTier string

const (
	TierHot  SourceTier = "hot"  // WebSocket real-time
	TierWarm SourceTier = "warm" // REST API + cache
	TierCold SourceTier = "cold" // Historical files
)

// Envelope wraps all data records with provenance and freshness tracking
type Envelope struct {
	// Core metadata
	Timestamp   time.Time  `json:"timestamp"`
	Venue       string     `json:"venue"`
	Symbol      string     `json:"symbol"`
	SourceTier  SourceTier `json:"source_tier"`
	FreshnessMS int64      `json:"freshness_ms"` // Age in milliseconds

	// Provenance tracking
	Provenance ProvenanceInfo `json:"provenance"`
	Checksum   string         `json:"checksum"` // SHA256 of (venue,symbol,ts,value,unit)

	// Payload (one of these will be populated)
	OrderBook   interface{} `json:"order_book,omitempty"`
	PriceData   interface{} `json:"price_data,omitempty"`
	VolumeData  interface{} `json:"volume_data,omitempty"`
	GenericData interface{} `json:"generic_data,omitempty"`
}

// ProvenanceInfo tracks data lineage and quality
type ProvenanceInfo struct {
	OriginalSource string    `json:"original_source"` // "binance_ws", "kraken_rest", etc.
	CacheHit       bool      `json:"cache_hit"`
	FallbackChain  []string  `json:"fallback_chain,omitempty"` // If failed over
	RetrievedAt    time.Time `json:"retrieved_at"`
	TTLExpires     time.Time `json:"ttl_expires,omitempty"`

	// Quality metrics
	LatencyMS       int64   `json:"latency_ms"`
	RetryCount      int     `json:"retry_count"`
	CircuitState    string  `json:"circuit_state,omitempty"` // "open", "closed", "half-open"
	ConfidenceScore float64 `json:"confidence_score"`        // 0.0-1.0
}

// GenerateChecksum creates stable checksum for provenance tracking
func (e *Envelope) GenerateChecksum(value interface{}, unit string) string {
	// Create stable hash from core fields
	hashInput := fmt.Sprintf("%s|%s|%d|%v|%s",
		e.Venue,
		e.Symbol,
		e.Timestamp.UnixNano(),
		value,
		unit,
	)

	hash := sha256.Sum256([]byte(hashInput))
	return hex.EncodeToString(hash[:])
}

// IsStale checks if data exceeds maximum age for given use case
func (e *Envelope) IsStale(maxAgeMS int64) bool {
	return e.FreshnessMS > maxAgeMS
}

// CalculateFreshness updates freshness based on current time
func (e *Envelope) CalculateFreshness() {
	e.FreshnessMS = time.Since(e.Timestamp).Milliseconds()
}

// IsFallback returns true if this data came from fallback sources
func (e *Envelope) IsFallback() bool {
	return len(e.Provenance.FallbackChain) > 0
}

// GetSourceAuthority returns authority level: Hot=3, Warm=2, Cold=1
func (e *Envelope) GetSourceAuthority() int {
	switch e.SourceTier {
	case TierHot:
		return 3
	case TierWarm:
		return 2
	case TierCold:
		return 1
	default:
		return 0
	}
}

// EnvelopeOption allows functional configuration
type EnvelopeOption func(*Envelope)

// WithFallbackChain records the fallback sequence used
func WithFallbackChain(chain []string) EnvelopeOption {
	return func(e *Envelope) {
		e.Provenance.FallbackChain = chain
	}
}

// WithCacheHit marks data as cache-sourced
func WithCacheHit(hit bool) EnvelopeOption {
	return func(e *Envelope) {
		e.Provenance.CacheHit = hit
	}
}

// WithConfidenceScore sets quality confidence
func WithConfidenceScore(score float64) EnvelopeOption {
	return func(e *Envelope) {
		e.Provenance.ConfidenceScore = score
	}
}

// NewEnvelope creates envelope with basic metadata
func NewEnvelope(venue, symbol string, tier SourceTier, opts ...EnvelopeOption) *Envelope {
	now := time.Now()

	e := &Envelope{
		Timestamp:  now,
		Venue:      venue,
		Symbol:     symbol,
		SourceTier: tier,
		Provenance: ProvenanceInfo{
			RetrievedAt:     now,
			ConfidenceScore: 1.0, // Default to full confidence
		},
	}

	// Apply options
	for _, opt := range opts {
		opt(e)
	}

	e.CalculateFreshness()
	return e
}
