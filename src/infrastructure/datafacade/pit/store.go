package pit

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/sawpanic/cryptorun/src/infrastructure/datafacade/interfaces"
)

// FileBasedPITStore implements PITStore using filesystem
type FileBasedPITStore struct {
	basePath    string
	snapshots   map[string]*snapshotData
	mu          sync.RWMutex
	compression bool
}

type snapshotData struct {
	Info     interfaces.SnapshotInfo    `json:"info"`
	Data     map[string]interface{}     `json:"data"`
	Created  time.Time                  `json:"created"`
	FilePath string                     `json:"file_path"`
}

// NewFileBasedPITStore creates a new file-based PIT store
func NewFileBasedPITStore(basePath string, enableCompression bool) *FileBasedPITStore {
	return &FileBasedPITStore{
		basePath:    basePath,
		snapshots:   make(map[string]*snapshotData),
		compression: enableCompression,
	}
}

// CreateSnapshot creates a new point-in-time snapshot
func (s *FileBasedPITStore) CreateSnapshot(ctx context.Context, snapshotID string, data map[string]interface{}) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check if snapshot already exists
	if _, exists := s.snapshots[snapshotID]; exists {
		return fmt.Errorf("snapshot %s already exists", snapshotID)
	}

	now := time.Now()
	
	// Analyze data to build metadata
	venues := make(map[string]bool)
	symbols := make(map[string]bool)
	dataTypes := make(map[interfaces.DataType]bool)
	
	for key := range data {
		parts := strings.Split(key, ":")
		if len(parts) >= 3 {
			venue := parts[0]
			symbol := parts[1]
			dataType := interfaces.DataType(parts[2])
			
			venues[venue] = true
			symbols[symbol] = true
			dataTypes[dataType] = true
		}
	}
	
	// Convert maps to slices
	venueList := make([]string, 0, len(venues))
	for venue := range venues {
		venueList = append(venueList, venue)
	}
	
	symbolList := make([]string, 0, len(symbols))
	for symbol := range symbols {
		symbolList = append(symbolList, symbol)
	}
	
	dataTypeList := make([]interfaces.DataType, 0, len(dataTypes))
	for dataType := range dataTypes {
		dataTypeList = append(dataTypeList, dataType)
	}

	// Create snapshot info
	info := interfaces.SnapshotInfo{
		SnapshotID: snapshotID,
		Timestamp:  now,
		Venues:     venueList,
		Symbols:    symbolList,
		DataTypes:  dataTypeList,
		Size:       0, // Will be calculated after serialization
		Metadata: map[string]interface{}{
			"created_by":    "data_facade",
			"total_entries": len(data),
			"compression":   s.compression,
		},
	}

	// Create snapshot data
	snapshot := &snapshotData{
		Info:    info,
		Data:    data,
		Created: now,
	}

	// Ensure directory exists
	snapshotDir := filepath.Join(s.basePath, "snapshots", now.Format("2006-01-02"))
	if err := os.MkdirAll(snapshotDir, 0755); err != nil {
		return fmt.Errorf("create snapshot directory: %w", err)
	}

	// Write to file
	filename := fmt.Sprintf("%s_%s.json", snapshotID, now.Format("150405"))
	filePath := filepath.Join(snapshotDir, filename)
	
	if err := s.writeSnapshotToFile(snapshot, filePath); err != nil {
		return fmt.Errorf("write snapshot to file: %w", err)
	}

	// Calculate actual file size
	if fileInfo, err := os.Stat(filePath); err == nil {
		snapshot.Info.Size = fileInfo.Size()
	}
	
	snapshot.FilePath = filePath
	s.snapshots[snapshotID] = snapshot

	return nil
}

// GetSnapshot retrieves a snapshot by ID
func (s *FileBasedPITStore) GetSnapshot(ctx context.Context, snapshotID string) (map[string]interface{}, error) {
	s.mu.RLock()
	snapshot, exists := s.snapshots[snapshotID]
	s.mu.RUnlock()

	if !exists {
		// Try to load from file
		if err := s.loadSnapshotFromFile(snapshotID); err != nil {
			return nil, fmt.Errorf("snapshot %s not found: %w", snapshotID, err)
		}
		
		s.mu.RLock()
		snapshot = s.snapshots[snapshotID]
		s.mu.RUnlock()
	}

	// Return a copy to ensure immutability
	dataCopy := make(map[string]interface{})
	for k, v := range snapshot.Data {
		dataCopy[k] = v
	}

	return dataCopy, nil
}

// ListSnapshots returns snapshots matching the filter
func (s *FileBasedPITStore) ListSnapshots(ctx context.Context, filter interfaces.SnapshotFilter) ([]interfaces.SnapshotInfo, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var results []interfaces.SnapshotInfo
	
	for _, snapshot := range s.snapshots {
		if s.matchesFilter(snapshot.Info, filter) {
			results = append(results, snapshot.Info)
		}
	}

	// Sort by timestamp (newest first)
	for i := 0; i < len(results)-1; i++ {
		for j := i + 1; j < len(results); j++ {
			if results[i].Timestamp.Before(results[j].Timestamp) {
				results[i], results[j] = results[j], results[i]
			}
		}
	}

	// Apply limit
	if filter.Limit > 0 && len(results) > filter.Limit {
		results = results[:filter.Limit]
	}

	return results, nil
}

// DeleteSnapshot removes a snapshot
func (s *FileBasedPITStore) DeleteSnapshot(ctx context.Context, snapshotID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	snapshot, exists := s.snapshots[snapshotID]
	if !exists {
		return fmt.Errorf("snapshot %s not found", snapshotID)
	}

	// Remove file
	if snapshot.FilePath != "" {
		if err := os.Remove(snapshot.FilePath); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove snapshot file: %w", err)
		}
	}

	// Remove from memory
	delete(s.snapshots, snapshotID)

	return nil
}

// LoadExistingSnapshots loads snapshots from filesystem on startup
func (s *FileBasedPITStore) LoadExistingSnapshots() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	snapshotsDir := filepath.Join(s.basePath, "snapshots")
	
	return filepath.Walk(snapshotsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip errors, continue walking
		}

		if info.IsDir() {
			return nil
		}

		if !strings.HasSuffix(info.Name(), ".json") {
			return nil
		}

		// Try to load the snapshot
		if err := s.loadSnapshotFromFilePath(path); err != nil {
			// Log error but continue
			return nil
		}

		return nil
	})
}

// Helper methods

func (s *FileBasedPITStore) writeSnapshotToFile(snapshot *snapshotData, filePath string) error {
	data, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal snapshot: %w", err)
	}

	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return fmt.Errorf("write file: %w", err)
	}

	return nil
}

func (s *FileBasedPITStore) loadSnapshotFromFile(snapshotID string) error {
	// Search for the snapshot file
	snapshotsDir := filepath.Join(s.basePath, "snapshots")
	
	var foundPath string
	_ = filepath.Walk(snapshotsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}

		if info.IsDir() {
			return nil
		}

		if strings.HasPrefix(info.Name(), snapshotID+"_") && strings.HasSuffix(info.Name(), ".json") {
			foundPath = path
			return fmt.Errorf("found") // Use error to break out of walk
		}

		return nil
	})

	if foundPath == "" {
		return fmt.Errorf("snapshot file not found")
	}

	return s.loadSnapshotFromFilePath(foundPath)
}

func (s *FileBasedPITStore) loadSnapshotFromFilePath(filePath string) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("read file: %w", err)
	}

	var snapshot snapshotData
	if err := json.Unmarshal(data, &snapshot); err != nil {
		return fmt.Errorf("unmarshal snapshot: %w", err)
	}

	snapshot.FilePath = filePath
	s.snapshots[snapshot.Info.SnapshotID] = &snapshot

	return nil
}

func (s *FileBasedPITStore) matchesFilter(info interfaces.SnapshotInfo, filter interfaces.SnapshotFilter) bool {
	// Time filter
	if filter.FromTime != nil && info.Timestamp.Before(*filter.FromTime) {
		return false
	}
	if filter.ToTime != nil && info.Timestamp.After(*filter.ToTime) {
		return false
	}

	// Venues filter
	if len(filter.Venues) > 0 {
		found := false
		for _, filterVenue := range filter.Venues {
			for _, infoVenue := range info.Venues {
				if filterVenue == infoVenue {
					found = true
					break
				}
			}
			if found {
				break
			}
		}
		if !found {
			return false
		}
	}

	// Symbols filter
	if len(filter.Symbols) > 0 {
		found := false
		for _, filterSymbol := range filter.Symbols {
			for _, infoSymbol := range info.Symbols {
				if filterSymbol == infoSymbol {
					found = true
					break
				}
			}
			if found {
				break
			}
		}
		if !found {
			return false
		}
	}

	// DataTypes filter
	if len(filter.DataTypes) > 0 {
		found := false
		for _, filterDataType := range filter.DataTypes {
			for _, infoDataType := range info.DataTypes {
				if filterDataType == infoDataType {
					found = true
					break
				}
			}
			if found {
				break
			}
		}
		if !found {
			return false
		}
	}

	return true
}

// Cleanup removes old snapshots based on retention policy
func (s *FileBasedPITStore) Cleanup(ctx context.Context, retentionDays int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	cutoffTime := time.Now().AddDate(0, 0, -retentionDays)
	
	var toDelete []string
	
	for snapshotID, snapshot := range s.snapshots {
		if snapshot.Created.Before(cutoffTime) {
			toDelete = append(toDelete, snapshotID)
		}
	}

	for _, snapshotID := range toDelete {
		if err := s.DeleteSnapshot(ctx, snapshotID); err != nil {
			// Log error but continue cleanup
			continue
		}
	}

	return nil
}

// GetStorageStats returns storage statistics
func (s *FileBasedPITStore) GetStorageStats() (*PITStorageStats, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stats := &PITStorageStats{
		TotalSnapshots: len(s.snapshots),
		TotalSize:      0,
		OldestSnapshot: time.Now(),
		NewestSnapshot: time.Time{},
	}

	for _, snapshot := range s.snapshots {
		stats.TotalSize += snapshot.Info.Size
		
		if snapshot.Created.Before(stats.OldestSnapshot) {
			stats.OldestSnapshot = snapshot.Created
		}
		
		if snapshot.Created.After(stats.NewestSnapshot) {
			stats.NewestSnapshot = snapshot.Created
		}
	}

	return stats, nil
}

// PITStorageStats contains PIT storage statistics
type PITStorageStats struct {
	TotalSnapshots int       `json:"total_snapshots"`
	TotalSize      int64     `json:"total_size_bytes"`
	OldestSnapshot time.Time `json:"oldest_snapshot"`
	NewestSnapshot time.Time `json:"newest_snapshot"`
}