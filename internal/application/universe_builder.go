package application

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/sawpanic/cryptorun/internal/atomicio"
	"github.com/rs/zerolog/log"
)

// UniverseCriteria defines the requirements for universe construction
type UniverseCriteria struct {
	Quote     string `json:"quote"`
	MinADVUSD int64  `json:"min_adv_usd"`
	Venue     string `json:"venue"`
}

// UniverseSnapshot represents a point-in-time universe
type UniverseSnapshot struct {
	Metadata UniverseMetadata `json:"_metadata"`
	Universe []string         `json:"universe"`
}

// UniverseMetadata contains generation metadata and integrity hash
type UniverseMetadata struct {
	Generated time.Time        `json:"generated"`
	Source    string           `json:"source"`
	Criteria  UniverseCriteria `json:"criteria"`
	Hash      string           `json:"hash"`
	Count     int              `json:"count"`
}

// UniverseBuilder handles deterministic universe construction
type UniverseBuilder struct {
	pairsSync *PairsSync
	criteria  UniverseCriteria
}

// UniverseBuildResult contains the build outcome
type UniverseBuildResult struct {
	Snapshot    *UniverseSnapshot `json:"snapshot"`
	HashChanged bool              `json:"hash_changed"`
	PrevHash    string            `json:"prev_hash"`
	NewHash     string            `json:"new_hash"`
	Added       []string          `json:"added"`
	Removed     []string          `json:"removed"`
}

// NewUniverseBuilder creates a new universe builder
func NewUniverseBuilder(criteria UniverseCriteria) *UniverseBuilder {
	config := PairsSyncConfig{
		Venue:  criteria.Venue,
		Quote:  criteria.Quote,
		MinADV: criteria.MinADVUSD,
	}

	return &UniverseBuilder{
		pairsSync: NewPairsSync(config),
		criteria:  criteria,
	}
}

// BuildUniverse creates a deterministic universe snapshot
func (ub *UniverseBuilder) BuildUniverse(ctx context.Context) (*UniverseBuildResult, error) {
	log.Info().
		Str("venue", ub.criteria.Venue).
		Str("quote", ub.criteria.Quote).
		Int64("min_adv", ub.criteria.MinADVUSD).
		Msg("Building universe snapshot")

	// Sync pairs from exchange
	_, err := ub.pairsSync.SyncPairs(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to sync pairs: %w", err)
	}

	// Load current universe to detect changes
	currentUniverse := ub.loadCurrentUniverse()
	prevHash := ""
	if currentUniverse != nil {
		prevHash = currentUniverse.Metadata.Hash
	}

	// Get pairs that meet criteria
	validPairs, err := ub.getValidPairs(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get valid pairs: %w", err)
	}

	// Apply symbol normalization (XBT→BTC, etc.)
	normalizedPairs := ub.normalizeSymbols(validPairs)

	// Create deterministic snapshot
	snapshot := &UniverseSnapshot{
		Metadata: UniverseMetadata{
			Generated: time.Now().UTC(),
			Source:    ub.criteria.Venue,
			Criteria:  ub.criteria,
			Count:     len(normalizedPairs),
		},
		Universe: normalizedPairs,
	}

	// Generate integrity hash
	snapshot.Metadata.Hash = ub.generateHash(snapshot)

	// Determine changes
	added, removed := ub.computeChanges(currentUniverse, normalizedPairs)
	hashChanged := prevHash != snapshot.Metadata.Hash

	result := &UniverseBuildResult{
		Snapshot:    snapshot,
		HashChanged: hashChanged,
		PrevHash:    prevHash,
		NewHash:     snapshot.Metadata.Hash,
		Added:       added,
		Removed:     removed,
	}

	log.Info().
		Int("count", len(normalizedPairs)).
		Bool("hash_changed", hashChanged).
		Str("hash", snapshot.Metadata.Hash[:8]).
		Int("added", len(added)).
		Int("removed", len(removed)).
		Msg("Universe build completed")

	return result, nil
}

// getValidPairs retrieves pairs meeting ADV criteria
func (ub *UniverseBuilder) getValidPairs(ctx context.Context) ([]string, error) {
	// For now, use the existing pairs sync logic
	// In a real implementation, this would validate ADV against exchange data

	// Load current universe pairs as a baseline
	currentUniverse := ub.loadCurrentUniverse()
	if currentUniverse == nil {
		// Return a default set for bootstrap
		return []string{"BTCUSD", "ETHUSD", "ADAUSD", "SOLUSD", "DOTUSD"}, nil
	}

	// Return current universe (in practice, this would fetch real ADV data)
	return currentUniverse.Universe, nil
}

// normalizeSymbols applies standard symbol transformations
func (ub *UniverseBuilder) normalizeSymbols(symbols []string) []string {
	normalized := make([]string, 0, len(symbols))

	for _, symbol := range symbols {
		// Apply XBT→BTC normalization
		if strings.HasPrefix(symbol, "XBT") {
			symbol = strings.Replace(symbol, "XBT", "BTC", 1)
		}

		// Ensure USD-only and valid format
		if strings.HasSuffix(symbol, "USD") && ub.isValidSymbol(symbol) {
			normalized = append(normalized, symbol)
		}
	}

	// Sort deterministically
	sort.Strings(normalized)
	return normalized
}

// isValidSymbol validates symbol format
func (ub *UniverseBuilder) isValidSymbol(symbol string) bool {
	// USD-only regex: ^[A-Z0-9]+USD$
	if !strings.HasSuffix(symbol, "USD") {
		return false
	}

	base := strings.TrimSuffix(symbol, "USD")
	if len(base) == 0 {
		return false
	}

	// Check base contains only uppercase letters and numbers
	for _, char := range base {
		if !((char >= 'A' && char <= 'Z') || (char >= '0' && char <= '9')) {
			return false
		}
	}

	return true
}

// generateHash creates a 64-character hex hash over {criteria, symbols[]}
func (ub *UniverseBuilder) generateHash(snapshot *UniverseSnapshot) string {
	// Create deterministic hash input
	hashData := struct {
		Criteria UniverseCriteria `json:"criteria"`
		Symbols  []string         `json:"symbols"`
	}{
		Criteria: snapshot.Metadata.Criteria,
		Symbols:  snapshot.Universe, // Already sorted
	}

	jsonData, _ := json.Marshal(hashData)
	hash := sha256.Sum256(jsonData)
	return hex.EncodeToString(hash[:])
}

// computeChanges determines added/removed symbols
func (ub *UniverseBuilder) computeChanges(current *UniverseSnapshot, newSymbols []string) ([]string, []string) {
	if current == nil {
		return newSymbols, []string{}
	}

	currentMap := make(map[string]bool)
	for _, symbol := range current.Universe {
		currentMap[symbol] = true
	}

	newMap := make(map[string]bool)
	for _, symbol := range newSymbols {
		newMap[symbol] = true
	}

	var added, removed []string

	// Find added symbols
	for _, symbol := range newSymbols {
		if !currentMap[symbol] {
			added = append(added, symbol)
		}
	}

	// Find removed symbols
	for _, symbol := range current.Universe {
		if !newMap[symbol] {
			removed = append(removed, symbol)
		}
	}

	sort.Strings(added)
	sort.Strings(removed)

	return added, removed
}

// loadCurrentUniverse loads the current universe configuration
func (ub *UniverseBuilder) loadCurrentUniverse() *UniverseSnapshot {
	data, err := os.ReadFile("config/universe.json")
	if err != nil {
		return nil
	}

	var snapshot UniverseSnapshot
	if err := json.Unmarshal(data, &snapshot); err != nil {
		return nil
	}

	return &snapshot
}

// WriteSnapshot writes universe snapshot to timestamped directory and updates config
func (ub *UniverseBuilder) WriteSnapshot(snapshot *UniverseSnapshot) error {
	timestamp := snapshot.Metadata.Generated.Format("2006-01-02")
	snapshotDir := filepath.Join("out", "universe", timestamp)

	// Create snapshot directory
	if err := os.MkdirAll(snapshotDir, 0755); err != nil {
		return fmt.Errorf("failed to create snapshot directory: %w", err)
	}

	// Write timestamped snapshot
	snapshotPath := filepath.Join(snapshotDir, "universe.json")
	snapshotData, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal snapshot: %w", err)
	}

	if err := atomicio.WriteFile(snapshotPath, snapshotData, 0644); err != nil {
		return fmt.Errorf("failed to write snapshot: %w", err)
	}

	// Atomic update of config/universe.json
	configPath := "config/universe.json"
	if err := atomicio.WriteFile(configPath, snapshotData, 0644); err != nil {
		return fmt.Errorf("failed to update config universe: %w", err)
	}

	log.Info().
		Str("snapshot_path", snapshotPath).
		Str("config_path", configPath).
		Str("hash", snapshot.Metadata.Hash[:8]).
		Msg("Universe snapshot written")

	return nil
}

// UniverseRebuildJob represents a scheduled universe rebuild
type UniverseRebuildJob struct {
	builder  *UniverseBuilder
	schedule string // "daily" for now
}

// NewUniverseRebuildJob creates a daily universe rebuild job
func NewUniverseRebuildJob() *UniverseRebuildJob {
	criteria := UniverseCriteria{
		Quote:     "USD",
		MinADVUSD: 100000,
		Venue:     "kraken",
	}

	return &UniverseRebuildJob{
		builder:  NewUniverseBuilder(criteria),
		schedule: "daily",
	}
}

// Execute runs the universe rebuild job
func (job *UniverseRebuildJob) Execute(ctx context.Context) (*UniverseBuildResult, error) {
	log.Info().Str("schedule", job.schedule).Msg("Executing universe rebuild job")

	// Build new universe
	result, err := job.builder.BuildUniverse(ctx)
	if err != nil {
		return nil, fmt.Errorf("universe build failed: %w", err)
	}

	// Write snapshot if hash changed or forced
	if result.HashChanged || result.PrevHash == "" {
		if err := job.builder.WriteSnapshot(result.Snapshot); err != nil {
			return nil, fmt.Errorf("failed to write snapshot: %w", err)
		}
	}

	return result, nil
}

// NormalizeSymbols exposes symbol normalization for testing
func (ub *UniverseBuilder) NormalizeSymbols(symbols []string) []string {
	return ub.normalizeSymbols(symbols)
}

// GenerateHash exposes hash generation for testing
func (ub *UniverseBuilder) GenerateHash(snapshot *UniverseSnapshot) string {
	return ub.generateHash(snapshot)
}
