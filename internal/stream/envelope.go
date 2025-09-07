package stream

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"
)

// Envelope represents a streaming message with comprehensive metadata
type Envelope struct {
	Timestamp time.Time       `json:"ts"`               // Message timestamp (required)
	Symbol    string          `json:"symbol"`           // Trading symbol (required)
	Source    string          `json:"source"`           // Venue or pipe name (required)
	Payload   json.RawMessage `json:"payload"`          // Message content (required)
	Checksum  string          `json:"checksum"`         // blake3(payload||ts||symbol||source)
	Version   int             `json:"version"`          // Message format version (start at 1)
	
	// Extended metadata for CryptoRun
	MessageID  string            `json:"message_id,omitempty"`   // Unique message identifier
	Headers    map[string]string `json:"headers,omitempty"`      // Additional metadata
	Venue      string            `json:"venue,omitempty"`        // Exchange venue
	DataType   string            `json:"data_type,omitempty"`    // "ohlcv", "depth", "trades"
	SourceTier string            `json:"source_tier,omitempty"`  // "hot", "warm", "cold"
}

// ComputeChecksum generates SHA256 checksum for message integrity
func (e *Envelope) ComputeChecksum() string {
	// Create deterministic hash input: payload||timestamp||symbol||source
	hashInput := fmt.Sprintf("%s||%d||%s||%s", 
		string(e.Payload), 
		e.Timestamp.UnixNano(), 
		e.Symbol, 
		e.Source)
	
	hash := sha256.Sum256([]byte(hashInput))
	return hex.EncodeToString(hash[:])
}

// Validate validates envelope contents and verifies checksum
func Validate(e *Envelope) error {
	// Check required fields
	if e.Symbol == "" {
		return fmt.Errorf("envelope symbol is empty")
	}
	if e.Source == "" {
		return fmt.Errorf("envelope source is empty")
	}
	if len(e.Payload) == 0 {
		return fmt.Errorf("envelope payload is empty")
	}
	if e.Timestamp.IsZero() {
		return fmt.Errorf("envelope timestamp is zero")
	}
	if e.Version <= 0 {
		return fmt.Errorf("envelope version must be positive, got %d", e.Version)
	}
	
	// Verify checksum if present
	if e.Checksum != "" {
		expected := e.ComputeChecksum()
		if e.Checksum != expected {
			return fmt.Errorf("envelope checksum mismatch: expected %s, got %s", expected, e.Checksum)
		}
	}
	
	return nil
}

// SetChecksum computes and sets the checksum for the envelope
func (e *Envelope) SetChecksum() {
	e.Checksum = e.ComputeChecksum()
}

// IsValid returns true if envelope passes validation
func (e *Envelope) IsValid() bool {
	return Validate(e) == nil
}

// GetAge returns age of message relative to current time
func (e *Envelope) GetAge() time.Duration {
	return time.Since(e.Timestamp)
}

// IsStale checks if message exceeds maximum age threshold
func (e *Envelope) IsStale(maxAge time.Duration) bool {
	return e.GetAge() > maxAge
}

// GetHeader returns header value for key, empty string if not found
func (e *Envelope) GetHeader(key string) string {
	if e.Headers == nil {
		return ""
	}
	return e.Headers[key]
}

// SetHeader sets header key-value pair
func (e *Envelope) SetHeader(key, value string) {
	if e.Headers == nil {
		e.Headers = make(map[string]string)
	}
	e.Headers[key] = value
}

// AddProvenance adds provenance tracking information to headers
func (e *Envelope) AddProvenance(originalSource, tier string, confidence float64, cacheHit bool) {
	if e.Headers == nil {
		e.Headers = make(map[string]string)
	}
	e.Headers["original_source"] = originalSource
	e.Headers["confidence"] = fmt.Sprintf("%.3f", confidence)
	e.Headers["cache_hit"] = fmt.Sprintf("%t", cacheHit)
	if tier != "" {
		e.SourceTier = tier
	}
}

// AddFallbackChain records fallback source chain in headers
func (e *Envelope) AddFallbackChain(chain []string) {
	if len(chain) == 0 {
		return
	}
	if e.Headers == nil {
		e.Headers = make(map[string]string)
	}
	
	// Join fallback chain with commas
	chainStr := ""
	for i, source := range chain {
		if i > 0 {
			chainStr += ","
		}
		chainStr += source
	}
	e.Headers["fallback_chain"] = chainStr
}

// NewEnvelope creates a new envelope with required fields and version 1
func NewEnvelope(symbol, source string, payload json.RawMessage) *Envelope {
	envelope := &Envelope{
		Timestamp: time.Now(),
		Symbol:    symbol,
		Source:    source,
		Payload:   payload,
		Version:   1, // Start with version 1
	}
	envelope.SetChecksum()
	return envelope
}

// NewEnvelopeWithTimestamp creates envelope with specific timestamp
func NewEnvelopeWithTimestamp(timestamp time.Time, symbol, source string, payload json.RawMessage) *Envelope {
	envelope := &Envelope{
		Timestamp: timestamp,
		Symbol:    symbol,
		Source:    source,
		Payload:   payload,
		Version:   1,
	}
	envelope.SetChecksum()
	return envelope
}

// ToJSON serializes envelope to JSON
func (e *Envelope) ToJSON() ([]byte, error) {
	return json.Marshal(e)
}

// FromJSON deserializes envelope from JSON and validates
func FromJSON(data []byte) (*Envelope, error) {
	var envelope Envelope
	if err := json.Unmarshal(data, &envelope); err != nil {
		return nil, fmt.Errorf("failed to unmarshal envelope: %w", err)
	}
	
	if err := Validate(&envelope); err != nil {
		return nil, fmt.Errorf("envelope validation failed: %w", err)
	}
	
	return &envelope, nil
}

// EnvelopeBuilder provides fluent interface for envelope construction
type EnvelopeBuilder struct {
	envelope *Envelope
}

// NewBuilder creates new envelope builder
func NewBuilder(symbol, source string) *EnvelopeBuilder {
	return &EnvelopeBuilder{
		envelope: &Envelope{
			Timestamp: time.Now(),
			Symbol:    symbol,
			Source:    source,
			Version:   1,
		},
	}
}

// WithPayload sets the payload
func (b *EnvelopeBuilder) WithPayload(payload json.RawMessage) *EnvelopeBuilder {
	b.envelope.Payload = payload
	return b
}

// WithTimestamp sets the timestamp
func (b *EnvelopeBuilder) WithTimestamp(ts time.Time) *EnvelopeBuilder {
	b.envelope.Timestamp = ts
	return b
}

// WithVenue sets the venue
func (b *EnvelopeBuilder) WithVenue(venue string) *EnvelopeBuilder {
	b.envelope.Venue = venue
	return b
}

// WithDataType sets the data type
func (b *EnvelopeBuilder) WithDataType(dataType string) *EnvelopeBuilder {
	b.envelope.DataType = dataType
	return b
}

// WithSourceTier sets the source tier
func (b *EnvelopeBuilder) WithSourceTier(tier string) *EnvelopeBuilder {
	b.envelope.SourceTier = tier
	return b
}

// WithHeader adds a header
func (b *EnvelopeBuilder) WithHeader(key, value string) *EnvelopeBuilder {
	b.envelope.SetHeader(key, value)
	return b
}

// WithMessageID sets the message ID
func (b *EnvelopeBuilder) WithMessageID(id string) *EnvelopeBuilder {
	b.envelope.MessageID = id
	return b
}

// Build constructs the final envelope with checksum
func (b *EnvelopeBuilder) Build() (*Envelope, error) {
	if err := Validate(b.envelope); err != nil {
		return nil, fmt.Errorf("envelope build validation failed: %w", err)
	}
	b.envelope.SetChecksum()
	return b.envelope, nil
}