package pit

import (
	"compress/gzip"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/sawpanic/cryptorun/internal/data/facade"
)

// Store implements facade.PITStore for point-in-time data snapshots
// Provides immutable, append-only storage for audit and backtest purposes
type Store struct {
	baseDir string
}

// NewStore creates a new PIT store with the specified base directory
func NewStore(baseDir string) *Store {
	return &Store{
		baseDir: baseDir,
	}
}

// Snapshot stores a point-in-time data snapshot
// Path: artifacts/pit/{entity}/{date}/{timestamp}.json.gz
func (s *Store) Snapshot(entity string, timestamp time.Time, payload interface{}, source string) error {
	// Create directory structure
	dateDir := timestamp.Format("2006-01-02")
	entityDir := filepath.Join(s.baseDir, entity, dateDir)
	
	if err := os.MkdirAll(entityDir, 0755); err != nil {
		return fmt.Errorf("failed to create PIT directory %s: %w", entityDir, err)
	}
	
	// Create filename with microsecond precision
	filename := fmt.Sprintf("%s.json.gz", timestamp.Format("20060102_150405.000000"))
	filePath := filepath.Join(entityDir, filename)
	
	// Check if file already exists (append-only, no overwrites)
	if _, err := os.Stat(filePath); err == nil {
		log.Debug().Str("file", filePath).Msg("PIT snapshot already exists, skipping")
		return nil
	}
	
	// Create snapshot structure
	snapshot := PITSnapshot{
		Entity:    entity,
		Timestamp: timestamp,
		Source:    source,
		Payload:   payload,
		CreatedAt: time.Now(),
	}
	
	// Write compressed JSON
	file, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("failed to create PIT file %s: %w", filePath, err)
	}
	defer file.Close()
	
	gzWriter := gzip.NewWriter(file)
	defer gzWriter.Close()
	
	encoder := json.NewEncoder(gzWriter)
	encoder.SetIndent("", "  ")
	
	if err := encoder.Encode(snapshot); err != nil {
		return fmt.Errorf("failed to encode PIT snapshot: %w", err)
	}
	
	log.Debug().Str("entity", entity).Str("source", source).
		Str("file", filePath).Time("timestamp", timestamp).
		Msg("PIT snapshot stored")
	
	return nil
}

// Read retrieves a specific point-in-time snapshot
func (s *Store) Read(entity string, timestamp time.Time) (interface{}, error) {
	dateDir := timestamp.Format("2006-01-02")
	filename := fmt.Sprintf("%s.json.gz", timestamp.Format("20060102_150405.000000"))
	filePath := filepath.Join(s.baseDir, entity, dateDir, filename)
	
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("PIT snapshot not found: %w", err)
	}
	defer file.Close()
	
	gzReader, err := gzip.NewReader(file)
	if err != nil {
		return nil, fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer gzReader.Close()
	
	var snapshot PITSnapshot
	decoder := json.NewDecoder(gzReader)
	
	if err := decoder.Decode(&snapshot); err != nil {
		return nil, fmt.Errorf("failed to decode PIT snapshot: %w", err)
	}
	
	log.Debug().Str("entity", entity).Time("timestamp", timestamp).
		Str("source", snapshot.Source).Msg("PIT snapshot read")
	
	return snapshot.Payload, nil
}

// List returns all snapshots for an entity within a time range
func (s *Store) List(entity string, from time.Time, to time.Time) ([]facade.PITEntry, error) {
	entityDir := filepath.Join(s.baseDir, entity)
	
	// Check if entity directory exists
	if _, err := os.Stat(entityDir); os.IsNotExist(err) {
		return []facade.PITEntry{}, nil
	}
	
	var entries []facade.PITEntry
	
	// Walk through date directories
	err := filepath.Walk(entityDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		
		// Skip directories and non-gzip files
		if info.IsDir() || filepath.Ext(path) != ".gz" {
			return nil
		}
		
		// Parse timestamp from filename
		basename := info.Name()
		timestampStr := basename[:len(basename)-8] // Remove .json.gz
		timestamp, err := time.Parse("20060102_150405.000000", timestampStr)
		if err != nil {
			log.Warn().Str("file", path).Str("timestamp", timestampStr).
				Err(err).Msg("Failed to parse PIT timestamp")
			return nil
		}
		
		// Check if timestamp is within range
		if timestamp.Before(from) || timestamp.After(to) {
			return nil
		}
		
		// Read snapshot to get source
		payload, err := s.Read(entity, timestamp)
		if err != nil {
			log.Warn().Str("file", path).Err(err).Msg("Failed to read PIT snapshot")
			return nil
		}
		
		entries = append(entries, facade.PITEntry{
			Entity:    entity,
			Timestamp: timestamp,
			Payload:   payload,
			Source:    extractSourceFromPayload(payload),
		})
		
		return nil
	})
	
	if err != nil {
		return nil, fmt.Errorf("failed to walk PIT directory: %w", err)
	}
	
	// Sort entries by timestamp
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Timestamp.Before(entries[j].Timestamp)
	})
	
	log.Debug().Str("entity", entity).Time("from", from).Time("to", to).
		Int("count", len(entries)).Msg("Listed PIT snapshots")
	
	return entries, nil
}

// ListEntities returns all available entities in the PIT store
func (s *Store) ListEntities() ([]string, error) {
	entries, err := os.ReadDir(s.baseDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, fmt.Errorf("failed to read PIT base directory: %w", err)
	}
	
	var entities []string
	for _, entry := range entries {
		if entry.IsDir() {
			entities = append(entities, entry.Name())
		}
	}
	
	return entities, nil
}

// Cleanup removes PIT snapshots older than the specified retention period
func (s *Store) Cleanup(retentionDays int) error {
	cutoff := time.Now().AddDate(0, 0, -retentionDays)
	
	entities, err := s.ListEntities()
	if err != nil {
		return fmt.Errorf("failed to list entities for cleanup: %w", err)
	}
	
	var totalDeleted int
	
	for _, entity := range entities {
		entityDir := filepath.Join(s.baseDir, entity)
		
		err := filepath.Walk(entityDir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			
			// Skip directories
			if info.IsDir() {
				return nil
			}
			
			// Check file modification time
			if info.ModTime().Before(cutoff) {
				if err := os.Remove(path); err != nil {
					log.Warn().Str("file", path).Err(err).Msg("Failed to delete old PIT file")
				} else {
					totalDeleted++
				}
			}
			
			return nil
		})
		
		if err != nil {
			log.Warn().Str("entity", entity).Err(err).Msg("Failed to cleanup entity directory")
		}
	}
	
	log.Info().Int("retention_days", retentionDays).Int("deleted", totalDeleted).
		Msg("PIT cleanup completed")
	
	return nil
}

// PITSnapshot represents the stored snapshot structure
type PITSnapshot struct {
	Entity    string      `json:"entity"`
	Timestamp time.Time   `json:"timestamp"`
	Source    string      `json:"source"`
	Payload   interface{} `json:"payload"`
	CreatedAt time.Time   `json:"created_at"`
}

// extractSourceFromPayload attempts to extract source information from payload
func extractSourceFromPayload(payload interface{}) string {
	// Try to extract venue information from common data structures
	if payloadMap, ok := payload.(map[string]interface{}); ok {
		if venue, exists := payloadMap["venue"]; exists {
			if venueStr, ok := venue.(string); ok {
				return venueStr
			}
		}
	}
	
	// Try type assertion for our data structures
	switch p := payload.(type) {
	case facade.Trade:
		return p.Venue
	case facade.BookL2:
		return p.Venue
	case facade.Kline:
		return p.Venue
	case *facade.Trade:
		return p.Venue
	case *facade.BookL2:
		return p.Venue
	case *facade.Kline:
		return p.Venue
	}
	
	return "unknown"
}