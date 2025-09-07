package db

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/sawpanic/cryptorun/internal/data/facade"
	"github.com/sawpanic/cryptorun/internal/persistence"
)

// PITStore implements facade.PITStore with dual persistence (file + database)
type PITStore struct {
	manager  *Manager
	fileBase string
	enabled  bool
}

// NewPITStore creates a new Point-in-Time store with optional database persistence
func NewPITStore(manager *Manager, fileBasePath string) *PITStore {
	return &PITStore{
		manager:  manager,
		fileBase: fileBasePath,
		enabled:  manager != nil && manager.IsEnabled(),
	}
}

// Snapshot stores a point-in-time record in both file system and database (if enabled)
func (s *PITStore) Snapshot(entity string, timestamp time.Time, payload interface{}, source string) error {
	// Always store to file system for backwards compatibility
	if err := s.storeToFile(entity, timestamp, payload, source); err != nil {
		log.Warn().Err(err).
			Str("entity", entity).
			Str("source", source).
			Time("timestamp", timestamp).
			Msg("Failed to store PIT snapshot to file")
	}

	// Store to database if enabled
	if s.enabled && s.manager.Repository() != nil {
		if err := s.storeToDatabase(entity, timestamp, payload, source); err != nil {
			log.Warn().Err(err).
				Str("entity", entity).
				Str("source", source).
				Time("timestamp", timestamp).
				Msg("Failed to store PIT snapshot to database")
			// Don't fail the entire operation if database storage fails
		}
	}

	return nil
}

// Read retrieves a point-in-time record (tries database first, then file fallback)
func (s *PITStore) Read(entity string, timestamp time.Time) (interface{}, error) {
	// Try database first if enabled
	if s.enabled && s.manager.Repository() != nil {
		if data, err := s.readFromDatabase(entity, timestamp); err == nil {
			return data, nil
		}
		// Fall back to file if database read fails
	}

	return s.readFromFile(entity, timestamp)
}

// List retrieves multiple point-in-time records within a time range
func (s *PITStore) List(entity string, from time.Time, to time.Time) ([]facade.PITEntry, error) {
	// Try database first if enabled
	if s.enabled && s.manager.Repository() != nil {
		if entries, err := s.listFromDatabase(entity, from, to); err == nil && len(entries) > 0 {
			return entries, nil
		}
		// Fall back to file if database query fails or returns no results
	}

	return s.listFromFile(entity, from, to)
}

// Database storage methods

func (s *PITStore) storeToDatabase(entity string, timestamp time.Time, payload interface{}, source string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	repos := s.manager.Repository()
	if repos == nil {
		return fmt.Errorf("repository not available")
	}

	// Route to appropriate repository based on entity type
	switch entity {
	case "trades":
		if trade, ok := s.convertToTrade(payload, source); ok {
			return repos.Trades.Insert(ctx, trade)
		}
	case "premove_artifacts":
		if artifact, ok := s.convertToPremoveArtifact(payload, source); ok {
			return repos.Premove.Upsert(ctx, artifact)
		}
	case "regime_snapshots":
		if snapshot, ok := s.convertToRegimeSnapshot(payload); ok {
			return repos.Regimes.Upsert(ctx, snapshot)
		}
	}

	// For unrecognized entity types, we skip database storage
	log.Debug().
		Str("entity", entity).
		Msg("Skipping database storage for unknown entity type")
	return nil
}

func (s *PITStore) readFromDatabase(entity string, timestamp time.Time) (interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	repos := s.manager.Repository()
	if repos == nil {
		return nil, fmt.Errorf("repository not available")
	}

	switch entity {
	case "regime_snapshots":
		return repos.Regimes.GetByTimestamp(ctx, timestamp)
	}

	return nil, fmt.Errorf("read not supported for entity type: %s", entity)
}

func (s *PITStore) listFromDatabase(entity string, from time.Time, to time.Time) ([]facade.PITEntry, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	repos := s.manager.Repository()
	if repos == nil {
		return nil, fmt.Errorf("repository not available")
	}

	timeRange := persistence.TimeRange{From: from, To: to}

	switch entity {
	case "trades":
		// We don't have a symbol filter here, so this would return too much data
		// Skip database query for trades without symbol filter
		return nil, fmt.Errorf("trades listing requires symbol filter")
	case "premove_artifacts":
		artifacts, err := repos.Premove.Window(ctx, timeRange)
		if err != nil {
			return nil, err
		}
		
		entries := make([]facade.PITEntry, len(artifacts))
		for i, artifact := range artifacts {
			entries[i] = facade.PITEntry{
				Entity:    entity,
				Timestamp: artifact.Timestamp,
				Payload:   artifact,
				Source:    artifact.Venue,
			}
		}
		return entries, nil
	case "regime_snapshots":
		snapshots, err := repos.Regimes.ListRange(ctx, timeRange)
		if err != nil {
			return nil, err
		}
		
		entries := make([]facade.PITEntry, len(snapshots))
		for i, snapshot := range snapshots {
			entries[i] = facade.PITEntry{
				Entity:    entity,
				Timestamp: snapshot.Timestamp,
				Payload:   snapshot,
				Source:    "regime_detector",
			}
		}
		return entries, nil
	}

	return nil, fmt.Errorf("list not supported for entity type: %s", entity)
}

// Conversion helpers

func (s *PITStore) convertToTrade(payload interface{}, source string) (persistence.Trade, bool) {
	// Try to convert from facade.Trade first
	if facadeTrade, ok := payload.(facade.Trade); ok {
		var orderID *string
		if facadeTrade.TradeID != "" {
			orderID = &facadeTrade.TradeID
		}
		
		return persistence.Trade{
			Timestamp:  facadeTrade.Timestamp,
			Symbol:     facadeTrade.Symbol,
			Venue:      facadeTrade.Venue,
			Side:       facadeTrade.Side,
			Price:      facadeTrade.Price,
			Qty:        facadeTrade.Size,
			OrderID:    orderID,
			Attributes: make(map[string]interface{}),
		}, true
	}
	
	// Try to convert from map[string]interface{} (JSON-like structure)
	if m, ok := payload.(map[string]interface{}); ok {
		trade := persistence.Trade{
			Venue:      source,
			Attributes: make(map[string]interface{}),
		}
		
		if symbol, ok := m["symbol"].(string); ok {
			trade.Symbol = symbol
		}
		if price, ok := m["price"].(float64); ok {
			trade.Price = price
		}
		if size, ok := m["size"].(float64); ok {
			trade.Qty = size
		}
		if side, ok := m["side"].(string); ok {
			trade.Side = side
		}
		if tsStr, ok := m["timestamp"].(string); ok {
			if ts, err := time.Parse(time.RFC3339, tsStr); err == nil {
				trade.Timestamp = ts
			}
		}
		if tradeID, ok := m["trade_id"].(string); ok {
			trade.OrderID = &tradeID
		}
		
		return trade, trade.Symbol != "" && trade.Price > 0 && trade.Qty > 0
	}
	
	return persistence.Trade{}, false
}

func (s *PITStore) convertToPremoveArtifact(payload interface{}, source string) (persistence.PremoveArtifact, bool) {
	// This would need specific logic based on how premove artifacts are structured
	// For now, we'll skip this complex conversion
	return persistence.PremoveArtifact{}, false
}

func (s *PITStore) convertToRegimeSnapshot(payload interface{}) (persistence.RegimeSnapshot, bool) {
	// This would need specific logic based on how regime snapshots are structured
	// For now, we'll skip this complex conversion
	return persistence.RegimeSnapshot{}, false
}

// File storage methods (existing functionality)

func (s *PITStore) storeToFile(entity string, timestamp time.Time, payload interface{}, source string) error {
	if s.fileBase == "" {
		return nil // File storage disabled
	}

	// Create directory structure: fileBase/entity/YYYY/MM/DD/
	dir := filepath.Join(s.fileBase, entity, timestamp.Format("2006"), timestamp.Format("01"), timestamp.Format("02"))
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	// File name: HH-MM-SS-source.json
	filename := fmt.Sprintf("%s-%s.json", timestamp.Format("15-04-05"), source)
	filepath := filepath.Join(dir, filename)

	// Marshal payload to JSON
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	// Write to file
	if err := os.WriteFile(filepath, data, 0644); err != nil {
		return fmt.Errorf("failed to write file %s: %w", filepath, err)
	}

	return nil
}

func (s *PITStore) readFromFile(entity string, timestamp time.Time) (interface{}, error) {
	if s.fileBase == "" {
		return nil, fmt.Errorf("file storage not configured")
	}

	// This is a simplified implementation - in reality, you'd need to handle
	// multiple files for the same timestamp, different sources, etc.
	pattern := filepath.Join(s.fileBase, entity, timestamp.Format("2006"), timestamp.Format("01"), timestamp.Format("02"), timestamp.Format("15-04-05")+"*.json")
	
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil, fmt.Errorf("failed to glob pattern %s: %w", pattern, err)
	}
	
	if len(matches) == 0 {
		return nil, fmt.Errorf("no files found for pattern %s", pattern)
	}

	// Read the first matching file
	data, err := os.ReadFile(matches[0])
	if err != nil {
		return nil, fmt.Errorf("failed to read file %s: %w", matches[0], err)
	}

	var payload interface{}
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, fmt.Errorf("failed to unmarshal data: %w", err)
	}

	return payload, nil
}

func (s *PITStore) listFromFile(entity string, from time.Time, to time.Time) ([]facade.PITEntry, error) {
	if s.fileBase == "" {
		return nil, fmt.Errorf("file storage not configured")
	}

	var entries []facade.PITEntry
	
	// This is a simplified implementation - you'd need to traverse the date hierarchy
	// and collect all files within the time range
	baseDir := filepath.Join(s.fileBase, entity)
	
	err := filepath.Walk(baseDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		
		if !info.IsDir() && filepath.Ext(path) == ".json" {
			// Extract timestamp from file path and name
			// This is a simplified approach - you'd need proper timestamp parsing
			if info.ModTime().After(from) && info.ModTime().Before(to) {
				data, readErr := os.ReadFile(path)
				if readErr != nil {
					return readErr
				}
				
				var payload interface{}
				if unmarshalErr := json.Unmarshal(data, &payload); unmarshalErr != nil {
					return unmarshalErr
				}
				
				entries = append(entries, facade.PITEntry{
					Entity:    entity,
					Timestamp: info.ModTime(),
					Payload:   payload,
					Source:    "file",
				})
			}
		}
		
		return nil
	})
	
	return entries, err
}