package data

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// FileReader interface for different file format readers
type FileReader interface {
	LoadFile(filePath, venue, symbol string) ([]*Envelope, error)
	ValidateFile(filePath string) error
}

// CSVReader handles CSV file reading
type CSVReader struct{}

// ParquetReader handles Parquet file reading
type ParquetReader struct{}

// ColdData implements historical file data tier
type ColdData struct {
	basePath      string
	csvReader     FileReader
	parquetReader FileReader

	// Cache for loaded data
	cache       map[string][]*Envelope
	cacheExpiry time.Duration
}

// ColdConfig holds configuration for cold data tier
type ColdConfig struct {
	BasePath    string `json:"base_path"`
	CacheExpiry string `json:"cache_expiry"` // Duration string like "1h"
	EnableCache bool   `json:"enable_cache"`
}

// NewColdData creates a new cold data tier
func NewColdData(config ColdConfig) (*ColdData, error) {
	expiry, err := time.ParseDuration(config.CacheExpiry)
	if err != nil {
		expiry = time.Hour // Default 1 hour
	}

	return &ColdData{
		basePath:      config.BasePath,
		csvReader:     &CSVReader{},
		parquetReader: &ParquetReader{},
		cache:         make(map[string][]*Envelope),
		cacheExpiry:   expiry,
	}, nil
}

// GetOrderBook retrieves historical order book data
func (c *ColdData) GetOrderBook(ctx context.Context, venue, symbol string) (*Envelope, error) {
	// For cold tier, return most recent available data
	data, err := c.GetHistoricalSlice(ctx, venue, symbol,
		time.Now().Add(-24*time.Hour), // Look back 24 hours
		time.Now())
	if err != nil {
		return nil, err
	}

	if len(data) == 0 {
		return nil, fmt.Errorf("no historical data found for %s %s", venue, symbol)
	}

	// Return most recent entry
	latest := data[len(data)-1]
	latest.SourceTier = TierCold
	latest.CalculateFreshness()

	return latest, nil
}

// GetPriceData retrieves historical price data
func (c *ColdData) GetPriceData(ctx context.Context, venue, symbol string) (*Envelope, error) {
	return c.GetOrderBook(ctx, venue, symbol) // Same for cold tier
}

// IsAvailable checks if cold data files exist for venue
func (c *ColdData) IsAvailable(ctx context.Context, venue string) bool {
	venuePath := filepath.Join(c.basePath, venue)
	info, err := os.Stat(venuePath)
	return err == nil && info.IsDir()
}

// GetHistoricalSlice retrieves data within time bounds
func (c *ColdData) GetHistoricalSlice(ctx context.Context, venue, symbol string, start, end time.Time) ([]*Envelope, error) {
	// Check cache first
	cacheKey := fmt.Sprintf("%s:%s:%d:%d", venue, symbol, start.Unix(), end.Unix())
	if cached, exists := c.cache[cacheKey]; exists {
		return cached, nil
	}

	// Find relevant files in date range
	files, err := c.findFilesInRange(venue, symbol, start, end)
	if err != nil {
		return nil, fmt.Errorf("failed to find files for %s %s: %w", venue, symbol, err)
	}

	var allData []*Envelope

	// Load data from each file
	for _, file := range files {
		var fileData []*Envelope
		var loadErr error

		if strings.HasSuffix(file, ".csv") {
			fileData, loadErr = c.csvReader.LoadFile(file, venue, symbol)
		} else if strings.HasSuffix(file, ".parquet") {
			fileData, loadErr = c.parquetReader.LoadFile(file, venue, symbol)
		} else {
			continue // Skip unsupported file types
		}

		if loadErr != nil {
			return nil, fmt.Errorf("failed to load file %s: %w", file, loadErr)
		}

		// Filter by time bounds
		for _, envelope := range fileData {
			if envelope.Timestamp.After(start) && envelope.Timestamp.Before(end) {
				envelope.SourceTier = TierCold
				envelope.Provenance.OriginalSource = fmt.Sprintf("%s_historical", venue)
				envelope.Provenance.ConfidenceScore = 0.7 // Lower confidence for historical
				allData = append(allData, envelope)
			}
		}
	}

	// Sort by timestamp
	sort.Slice(allData, func(i, j int) bool {
		return allData[i].Timestamp.Before(allData[j].Timestamp)
	})

	// Cache results
	c.cache[cacheKey] = allData

	return allData, nil
}

// LoadFromFile loads data from a specific file path
func (c *ColdData) LoadFromFile(filePath string) error {
	if strings.HasSuffix(filePath, ".csv") {
		return c.csvReader.ValidateFile(filePath)
	} else if strings.HasSuffix(filePath, ".parquet") {
		return c.parquetReader.ValidateFile(filePath)
	}

	return fmt.Errorf("unsupported file type: %s", filePath)
}

// findFilesInRange discovers files that might contain data in the time range
func (c *ColdData) findFilesInRange(venue, symbol string, start, end time.Time) ([]string, error) {
	venuePath := filepath.Join(c.basePath, venue)
	if _, err := os.Stat(venuePath); os.IsNotExist(err) {
		return nil, fmt.Errorf("venue directory not found: %s", venuePath)
	}

	var files []string

	// Walk through venue directory
	err := filepath.Walk(venuePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		// Check if file might contain the symbol and be in date range
		fileName := info.Name()
		if strings.Contains(fileName, symbol) || strings.Contains(fileName, "all") {
			// Simple heuristic: if file was modified within extended range, include it
			fileTime := info.ModTime()
			extendedStart := start.Add(-24 * time.Hour) // Look back extra day
			extendedEnd := end.Add(24 * time.Hour)      // Look ahead extra day

			if fileTime.After(extendedStart) && fileTime.Before(extendedEnd) {
				files = append(files, path)
			}
		}

		return nil
	})

	return files, err
}

// CleanupCache removes expired cache entries
func (c *ColdData) CleanupCache() {
	// Simple cleanup - in production would track cache timestamps
	if len(c.cache) > 100 { // Arbitrary limit
		c.cache = make(map[string][]*Envelope)
	}
}

// GetStats returns cold tier statistics
func (c *ColdData) GetStats() map[string]interface{} {
	stats := map[string]interface{}{
		"base_path":      c.basePath,
		"cached_queries": len(c.cache),
		"cache_expiry":   c.cacheExpiry.String(),
	}

	// Count available venues
	if info, err := os.Stat(c.basePath); err == nil && info.IsDir() {
		if entries, err := os.ReadDir(c.basePath); err == nil {
			venueCount := 0
			for _, entry := range entries {
				if entry.IsDir() {
					venueCount++
				}
			}
			stats["available_venues"] = venueCount
		}
	}

	return stats
}

// LoadFile implements FileReader for CSVReader
func (r *CSVReader) LoadFile(filePath, venue, symbol string) ([]*Envelope, error) {
	// Simplified CSV reading - would use full implementation from cold/csv.go
	return nil, fmt.Errorf("CSV reading not implemented in cold.go - use cold/csv.go")
}

// ValidateFile implements FileReader for CSVReader
func (r *CSVReader) ValidateFile(filePath string) error {
	return fmt.Errorf("CSV validation not implemented in cold.go")
}

// LoadFile implements FileReader for ParquetReader
func (r *ParquetReader) LoadFile(filePath, venue, symbol string) ([]*Envelope, error) {
	// Mock implementation for now
	return nil, fmt.Errorf("Parquet reading not implemented in cold.go - use cold/parquet.go")
}

// ValidateFile implements FileReader for ParquetReader
func (r *ParquetReader) ValidateFile(filePath string) error {
	return fmt.Errorf("Parquet validation not implemented in cold.go")
}
