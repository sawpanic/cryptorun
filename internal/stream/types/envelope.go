package types

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"
)

// Envelope represents a message envelope for streaming data
type Envelope struct {
	Timestamp time.Time       `json:"ts"`
	Symbol    string          `json:"symbol"`
	Source    string          `json:"source"`
	Payload   json.RawMessage `json:"payload"`
	Checksum  string          `json:"checksum"`
	Version   int             `json:"version"`
}

// CalculateChecksum calculates and sets the SHA256 checksum for the envelope
func (e *Envelope) CalculateChecksum() error {
	// Create a copy without the checksum for calculation
	temp := *e
	temp.Checksum = ""
	
	data, err := json.Marshal(temp)
	if err != nil {
		return fmt.Errorf("failed to marshal envelope for checksum: %w", err)
	}
	
	hash := sha256.Sum256(data)
	e.Checksum = hex.EncodeToString(hash[:])
	
	return nil
}

// VerifyChecksum verifies the envelope's checksum
func (e *Envelope) VerifyChecksum() bool {
	originalChecksum := e.Checksum
	
	// Calculate current checksum
	temp := *e
	temp.Checksum = ""
	
	data, err := json.Marshal(temp)
	if err != nil {
		return false
	}
	
	hash := sha256.Sum256(data)
	currentChecksum := hex.EncodeToString(hash[:])
	
	return originalChecksum == currentChecksum
}

// Validate validates the envelope structure and content
func Validate(envelope *Envelope) error {
	if envelope == nil {
		return fmt.Errorf("envelope cannot be nil")
	}
	
	if envelope.Timestamp.IsZero() {
		return fmt.Errorf("timestamp cannot be zero")
	}
	
	if envelope.Symbol == "" {
		return fmt.Errorf("symbol cannot be empty")
	}
	
	if envelope.Source == "" {
		return fmt.Errorf("source cannot be empty")
	}
	
	if len(envelope.Payload) == 0 {
		return fmt.Errorf("payload cannot be empty")
	}
	
	if envelope.Version <= 0 {
		return fmt.Errorf("version must be positive, got %d", envelope.Version)
	}
	
	// Validate JSON payload
	var temp interface{}
	if err := json.Unmarshal(envelope.Payload, &temp); err != nil {
		return fmt.Errorf("payload must be valid JSON: %w", err)
	}
	
	return nil
}