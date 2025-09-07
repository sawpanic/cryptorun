package optimization

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"sort"
	"time"

	"github.com/rs/zerolog/log"
)

// FileDataProvider implements DataProvider using file-based data
type FileDataProvider struct {
	ledgerPath     string
	snapshotPath   string
	marketDataPath string
	cache          map[string][]LedgerEntry
}

// NewFileDataProvider creates a new file-based data provider
func NewFileDataProvider(ledgerPath, snapshotPath, marketDataPath string) *FileDataProvider {
	return &FileDataProvider{
		ledgerPath:     ledgerPath,
		snapshotPath:   snapshotPath,
		marketDataPath: marketDataPath,
		cache:          make(map[string][]LedgerEntry),
	}
}

// GetLedgerData retrieves ledger entries for the specified time range
func (fdp *FileDataProvider) GetLedgerData(ctx context.Context, start, end time.Time) ([]LedgerEntry, error) {
	// Check cache first
	cacheKey := fmt.Sprintf("%d_%d", start.Unix(), end.Unix())
	if cached, exists := fdp.cache[cacheKey]; exists {
		log.Debug().Str("key", cacheKey).Int("entries", len(cached)).Msg("Using cached ledger data")
		return cached, nil
	}

	// Load from file
	entries, err := fdp.loadLedgerFile()
	if err != nil {
		return nil, fmt.Errorf("failed to load ledger file: %w", err)
	}

	// Filter by time range
	filtered := []LedgerEntry{}
	for _, entry := range entries {
		if !start.IsZero() && entry.TsScan.Before(start) {
			continue
		}
		if !end.IsZero() && entry.TsScan.After(end) {
			continue
		}
		filtered = append(filtered, entry)
	}

	// Sort by timestamp
	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].TsScan.Before(filtered[j].TsScan)
	})

	// Cache the result
	fdp.cache[cacheKey] = filtered

	log.Info().
		Time("start", start).
		Time("end", end).
		Int("total_entries", len(entries)).
		Int("filtered_entries", len(filtered)).
		Msg("Loaded ledger data")

	return filtered, nil
}

// GetMarketData retrieves market data for a symbol and time range
func (fdp *FileDataProvider) GetMarketData(ctx context.Context, symbol string, start, end time.Time) ([]MarketDataPoint, error) {
	// This would typically load from a separate market data file
	// For now, return synthetic data based on ledger entries

	ledgerData, err := fdp.GetLedgerData(ctx, start, end)
	if err != nil {
		return nil, fmt.Errorf("failed to get ledger data for market data: %w", err)
	}

	marketData := []MarketDataPoint{}

	for _, entry := range ledgerData {
		if entry.Symbol == symbol {
			// Create synthetic market data from ledger entry
			// In practice, this would come from actual OHLCV data
			point := MarketDataPoint{
				Timestamp: entry.TsScan,
				Symbol:    symbol,
				Price:     100.0 * (1.0 + entry.Realized.H24/100.0), // Synthetic price
				Volume:    1000000.0,                                // Synthetic volume
				High:      100.0 * (1.0 + entry.Realized.H24/100.0 + 0.01),
				Low:       100.0 * (1.0 + entry.Realized.H24/100.0 - 0.01),
			}
			marketData = append(marketData, point)
		}
	}

	return marketData, nil
}

// loadLedgerFile loads all entries from the ledger file
func (fdp *FileDataProvider) loadLedgerFile() ([]LedgerEntry, error) {
	file, err := os.Open(fdp.ledgerPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open ledger file %s: %w", fdp.ledgerPath, err)
	}
	defer file.Close()

	entries := []LedgerEntry{}
	scanner := bufio.NewScanner(file)
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := scanner.Text()

		// Skip empty lines
		if len(line) == 0 {
			continue
		}

		var entry LedgerEntry
		err := json.Unmarshal([]byte(line), &entry)
		if err != nil {
			log.Warn().
				Err(err).
				Int("line", lineNum).
				Str("content", line[:min(50, len(line))]).
				Msg("Failed to parse ledger entry, skipping")
			continue
		}

		entries = append(entries, entry)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading ledger file: %w", err)
	}

	log.Info().
		Str("file", fdp.ledgerPath).
		Int("entries", len(entries)).
		Int("lines", lineNum).
		Msg("Loaded ledger file")

	return entries, nil
}

// ValidateDataAvailability checks if required data files exist and are accessible
func (fdp *FileDataProvider) ValidateDataAvailability() error {
	// Check ledger file
	if _, err := os.Stat(fdp.ledgerPath); os.IsNotExist(err) {
		return fmt.Errorf("ledger file does not exist: %s", fdp.ledgerPath)
	}

	// Try to read a few lines to validate format
	entries, err := fdp.GetLedgerData(context.Background(), time.Time{}, time.Time{})
	if err != nil {
		return fmt.Errorf("failed to load ledger data: %w", err)
	}

	if len(entries) == 0 {
		return fmt.Errorf("ledger file is empty: %s", fdp.ledgerPath)
	}

	log.Info().
		Int("total_entries", len(entries)).
		Time("earliest", entries[0].TsScan).
		Time("latest", entries[len(entries)-1].TsScan).
		Msg("Data availability validated")

	return nil
}

// GetDataSummary returns a summary of available data
func (fdp *FileDataProvider) GetDataSummary(ctx context.Context) (*DataSummary, error) {
	entries, err := fdp.GetLedgerData(ctx, time.Time{}, time.Time{})
	if err != nil {
		return nil, fmt.Errorf("failed to load data for summary: %w", err)
	}

	if len(entries) == 0 {
		return &DataSummary{}, nil
	}

	// Count unique symbols
	symbols := make(map[string]bool)
	regimes := make(map[string]int)
	gatePassCount := 0

	for _, entry := range entries {
		symbols[entry.Symbol] = true
		if entry.GatesPass {
			gatePassCount++
		}

		// Simple regime classification based on composite score patterns
		regime := classifyRegime(entry)
		regimes[regime]++
	}

	uniqueSymbols := make([]string, 0, len(symbols))
	for symbol := range symbols {
		uniqueSymbols = append(uniqueSymbols, symbol)
	}

	return &DataSummary{
		TotalEntries:    len(entries),
		UniqueSymbols:   len(symbols),
		Symbols:         uniqueSymbols,
		StartTime:       entries[0].TsScan,
		EndTime:         entries[len(entries)-1].TsScan,
		GatePassRate:    float64(gatePassCount) / float64(len(entries)),
		RegimeBreakdown: regimes,
	}, nil
}

// classifyRegime classifies regime based on entry characteristics (simplified)
func classifyRegime(entry LedgerEntry) string {
	composite := entry.Composite
	realized24h := entry.Realized.H24

	// Simple classification logic
	if composite > 80 && realized24h > 5.0 {
		return "bull"
	} else if composite < 40 || (realized24h > -2.0 && realized24h < 2.0) {
		return "choppy"
	} else if math.Abs(realized24h) > 10.0 {
		return "high_vol"
	}

	return "neutral"
}

// ClearCache clears the data cache
func (fdp *FileDataProvider) ClearCache() {
	fdp.cache = make(map[string][]LedgerEntry)
	log.Info().Msg("Data cache cleared")
}

// GetCacheStats returns cache statistics
func (fdp *FileDataProvider) GetCacheStats() map[string]int {
	stats := make(map[string]int)
	totalEntries := 0

	for key, entries := range fdp.cache {
		stats[key] = len(entries)
		totalEntries += len(entries)
	}

	stats["total_cached_entries"] = totalEntries
	stats["cache_keys"] = len(fdp.cache)

	return stats
}

// Helper function for min
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
