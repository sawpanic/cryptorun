package data

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sawpanic/cryptorun/internal/data"
)

// Helper function to safely extract float64 values from envelope data
func extractFloat(envelope *data.Envelope, dataField string, key string, defaultValue float64) float64 {
	var dataMap map[string]interface{}
	var ok bool

	switch dataField {
	case "price":
		dataMap, ok = envelope.PriceData.(map[string]interface{})
	case "volume":
		dataMap, ok = envelope.VolumeData.(map[string]interface{})
	default:
		return defaultValue
	}

	if !ok {
		return defaultValue
	}

	if val, exists := dataMap[key]; exists {
		if floatVal, ok := val.(float64); ok {
			return floatVal
		}
	}
	return defaultValue
}

func TestColdDataParquetSupport(t *testing.T) {
	// Create temporary directory for test data
	tempDir := t.TempDir()

	config := data.ColdDataConfig{
		EnableParquet: true,
		EnableCSV:     true,
		DefaultFormat: "parquet",
		BasePath:      tempDir,
		CacheExpiry:   "5m",
		EnableCache:   true,
	}

	coldData, err := data.NewColdData(config)
	require.NoError(t, err)

	t.Run("config_flags_respected", func(t *testing.T) {
		stats := coldData.GetStats()
		assert.Equal(t, true, stats["enable_parquet"])
		assert.Equal(t, true, stats["enable_csv"])
		assert.Equal(t, "parquet", stats["default_format"])
		assert.Equal(t, tempDir, stats["base_path"])
	})

	t.Run("parquet_format_reader_selection", func(t *testing.T) {
		// Test Parquet reader selection
		parquetReader := coldData.GetFormatReader("/path/to/file.parquet")
		assert.IsType(t, &data.ParquetReader{}, parquetReader)

		// Test CSV reader selection
		csvReader := coldData.GetFormatReader("/path/to/file.csv")
		assert.IsType(t, &data.CSVReader{}, csvReader)

		// Test default reader (should be Parquet based on config)
		defaultReader := coldData.GetFormatReader("/path/to/file.unknown")
		assert.IsType(t, &data.ParquetReader{}, defaultReader)
	})

	t.Run("disable_parquet_fallback", func(t *testing.T) {
		configNoParquet := data.ColdDataConfig{
			EnableParquet: false,
			EnableCSV:     true,
			DefaultFormat: "csv",
			BasePath:      tempDir,
			CacheExpiry:   "5m",
			EnableCache:   true,
		}

		coldDataNoParquet, err := data.NewColdData(configNoParquet)
		require.NoError(t, err)

		// Should fallback to CSV when Parquet disabled
		reader := coldDataNoParquet.GetFormatReader("/path/to/file.parquet")
		assert.IsType(t, &data.CSVReader{}, reader)
	})
}

func TestCSVReaderOHLCVSchema(t *testing.T) {
	tempDir := t.TempDir()
	reader := &data.CSVReader{}

	t.Run("load_valid_csv_with_header", func(t *testing.T) {
		// Create test CSV with OHLCV schema
		csvContent := `timestamp,open,high,low,close,volume,venue,tier,provenance
2025-09-07T12:00:00Z,100.0,105.0,99.0,103.0,1000.0,kraken,cold,historical
2025-09-07T13:00:00Z,103.0,108.0,102.0,106.0,1200.0,kraken,cold,historical`

		csvPath := filepath.Join(tempDir, "test.csv")
		err := os.WriteFile(csvPath, []byte(csvContent), 0644)
		require.NoError(t, err)

		envelopes, err := reader.LoadFile(csvPath, "kraken", "BTCUSD")
		require.NoError(t, err)
		assert.Len(t, envelopes, 2)

		// Verify first record
		first := envelopes[0]
		assert.Equal(t, "BTCUSD", first.Symbol)
		assert.Equal(t, "kraken", first.Venue)
		assert.Equal(t, 100.0, extractFloat(first, "price", "open", 0))
		assert.Equal(t, 105.0, extractFloat(first, "price", "high", 0))
		assert.Equal(t, 99.0, extractFloat(first, "price", "low", 0))
		assert.Equal(t, 103.0, extractFloat(first, "price", "close", 0))
		assert.Equal(t, 1000.0, extractFloat(first, "volume", "volume", 0))
		assert.Equal(t, data.TierCold, first.SourceTier)
		assert.Equal(t, "historical", first.Provenance.OriginalSource)
	})

	t.Run("load_csv_without_header", func(t *testing.T) {
		// CSV without header (numeric data in first row)
		csvContent := `2025-09-07T12:00:00Z,100.0,105.0,99.0,103.0,1000.0,kraken,cold,historical`

		csvPath := filepath.Join(tempDir, "no_header.csv")
		err := os.WriteFile(csvPath, []byte(csvContent), 0644)
		require.NoError(t, err)

		envelopes, err := reader.LoadFile(csvPath, "kraken", "BTCUSD")
		require.NoError(t, err)
		assert.Len(t, envelopes, 1)
	})

	t.Run("time_filtered_loading", func(t *testing.T) {
		// Create CSV with multiple time periods
		csvContent := `timestamp,open,high,low,close,volume
2025-09-07T10:00:00Z,100.0,105.0,99.0,103.0,1000.0
2025-09-07T12:00:00Z,103.0,108.0,102.0,106.0,1200.0
2025-09-07T14:00:00Z,106.0,110.0,105.0,109.0,1300.0`

		csvPath := filepath.Join(tempDir, "time_filter.csv")
		err := os.WriteFile(csvPath, []byte(csvContent), 0644)
		require.NoError(t, err)

		from := time.Date(2025, 9, 7, 11, 0, 0, 0, time.UTC)
		until := time.Date(2025, 9, 7, 13, 0, 0, 0, time.UTC)

		envelopes, err := reader.LoadFileWithTimeFilter(csvPath, "kraken", "BTCUSD", from, until)
		require.NoError(t, err)
		assert.Len(t, envelopes, 1) // Only 12:00 record should match

		filtered := envelopes[0]
		assert.Equal(t, 103.0, extractFloat(filtered, "price", "open", 0))
	})

	t.Run("write_and_read_roundtrip", func(t *testing.T) {
		// Create test envelopes
		testData := []*data.Envelope{
			{
				Symbol:     "BTCUSD",
				Venue:      "kraken",
				Timestamp:  time.Date(2025, 9, 7, 12, 0, 0, 0, time.UTC),
				SourceTier: data.TierCold,
				PriceData: map[string]interface{}{
					"open":  100.0,
					"high":  105.0,
					"low":   99.0,
					"close": 103.0,
				},
				VolumeData: map[string]interface{}{
					"volume": 1000.0,
				},
				Provenance: data.ProvenanceInfo{
					OriginalSource:  "test",
					RetrievedAt: time.Date(2025, 9, 7, 12, 0, 0, 0, time.UTC),
					ConfidenceScore: 0.8,
				},
			},
		}

		csvPath := filepath.Join(tempDir, "roundtrip.csv")

		// Write data
		err := reader.WriteFile(csvPath, testData)
		require.NoError(t, err)

		// Read data back
		envelopes, err := reader.LoadFile(csvPath, "kraken", "BTCUSD")
		require.NoError(t, err)
		require.Len(t, envelopes, 1)

		// Verify roundtrip accuracy
		roundtrip := envelopes[0]
		assert.Equal(t, testData[0].Symbol, roundtrip.Symbol)
		assert.Equal(t, extractFloat(testData[0], "price", "open", 0), extractFloat(roundtrip, "price", "open", 0))
		assert.Equal(t, extractFloat(testData[0], "volume", "volume", 0), extractFloat(roundtrip, "volume", "volume", 0))
	})

	t.Run("validate_csv_file", func(t *testing.T) {
		// Valid CSV
		validCSV := `timestamp,open,high,low,close,volume
2025-09-07T12:00:00Z,100.0,105.0,99.0,103.0,1000.0`
		
		validPath := filepath.Join(tempDir, "valid.csv")
		err := os.WriteFile(validPath, []byte(validCSV), 0644)
		require.NoError(t, err)

		err = reader.ValidateFile(validPath)
		assert.NoError(t, err)

		// Invalid CSV (insufficient columns)
		invalidCSV := `timestamp,open,high
2025-09-07T12:00:00Z,100.0,105.0`
		
		invalidPath := filepath.Join(tempDir, "invalid.csv")
		err = os.WriteFile(invalidPath, []byte(invalidCSV), 0644)
		require.NoError(t, err)

		err = reader.ValidateFile(invalidPath)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "insufficient columns")
	})
}

func TestParquetReaderMockImplementation(t *testing.T) {
	reader := &data.ParquetReader{}
	tempDir := t.TempDir()

	t.Run("load_parquet_deterministic_fake", func(t *testing.T) {
		// Mock implementation should return deterministic test data
		envelopes, err := reader.LoadFile("/fake/path.parquet", "kraken", "BTCUSD")
		require.NoError(t, err)
		require.Len(t, envelopes, 1)

		envelope := envelopes[0]
		assert.Equal(t, "BTCUSD", envelope.Symbol)
		assert.Equal(t, "kraken", envelope.Venue)
		assert.Equal(t, data.TierCold, envelope.SourceTier)
		assert.Equal(t, 100.0, extractFloat(envelope, "price", "open", 0))
		assert.Equal(t, "parquet_historical", envelope.Provenance.OriginalSource)
	})

	t.Run("time_filtered_loading", func(t *testing.T) {
		from := time.Now().Add(-2 * time.Hour)
		until := time.Now()

		envelopes, err := reader.LoadFileWithTimeFilter("/fake/path.parquet", "kraken", "BTCUSD", from, until)
		require.NoError(t, err)
		assert.Len(t, envelopes, 1) // Mock should return 1 record within time range
	})

	t.Run("write_parquet_mock", func(t *testing.T) {
		testData := []*data.Envelope{
			{
				Symbol:     "BTCUSD",
				Venue:      "kraken",
				Timestamp:  time.Now(),
				SourceTier: data.TierCold,
				PriceData:  map[string]interface{}{"open": 100.0, "high": 105.0, "low": 99.0, "close": 103.0},
				VolumeData: map[string]interface{}{"volume": 1000.0},
				Provenance: data.ProvenanceInfo{OriginalSource: "test", ConfidenceScore: 0.8},
			},
		}

		parquetPath := filepath.Join(tempDir, "test.parquet")
		err := reader.WriteFile(parquetPath, testData)
		require.NoError(t, err)

		// Verify file was created
		_, err = os.Stat(parquetPath)
		assert.NoError(t, err)
	})

	t.Run("validate_parquet_file", func(t *testing.T) {
		// Create mock Parquet file
		parquetPath := filepath.Join(tempDir, "test.parquet")
		err := os.WriteFile(parquetPath, []byte("PARQUET MOCK DATA"), 0644)
		require.NoError(t, err)

		err = reader.ValidateFile(parquetPath)
		assert.NoError(t, err)

		// Test empty file validation
		emptyPath := filepath.Join(tempDir, "empty.parquet")
		err = os.WriteFile(emptyPath, []byte(""), 0644)
		require.NoError(t, err)

		err = reader.ValidateFile(emptyPath)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "empty")
	})
}

func TestColdDataHistoricalSlice(t *testing.T) {
	tempDir := t.TempDir()

	config := data.ColdDataConfig{
		EnableParquet: true,
		EnableCSV:     true,
		DefaultFormat: "csv",
		BasePath:      tempDir,
		CacheExpiry:   "1m",
		EnableCache:   true,
	}

	coldData, err := data.NewColdData(config)
	require.NoError(t, err)

	t.Run("query_time_range_csv", func(t *testing.T) {
		// Create venue directory and test CSV file
		venueDir := filepath.Join(tempDir, "kraken")
		err := os.MkdirAll(venueDir, 0755)
		require.NoError(t, err)

		csvContent := `timestamp,open,high,low,close,volume
2025-09-07T10:00:00Z,100.0,105.0,99.0,103.0,1000.0
2025-09-07T12:00:00Z,103.0,108.0,102.0,106.0,1200.0
2025-09-07T14:00:00Z,106.0,110.0,105.0,109.0,1300.0`

		csvPath := filepath.Join(venueDir, "BTCUSD_2025-09-07.csv")
		err = os.WriteFile(csvPath, []byte(csvContent), 0644)
		require.NoError(t, err)

		// Query specific time range
		from := time.Date(2025, 9, 7, 11, 0, 0, 0, time.UTC)
		until := time.Date(2025, 9, 7, 13, 0, 0, 0, time.UTC)

		envelopes, err := coldData.GetHistoricalSlice(context.Background(), "kraken", "BTCUSD", from, until)
		require.NoError(t, err)
		assert.Len(t, envelopes, 1) // Only 12:00 record should match

		// Verify data properties
		envelope := envelopes[0]
		assert.Equal(t, "BTCUSD", envelope.Symbol)
		assert.Equal(t, "kraken", envelope.Venue)
		assert.Equal(t, data.TierCold, envelope.SourceTier)
		assert.Equal(t, 103.0, extractFloat(envelope, "price", "open", 0))
		assert.Contains(t, envelope.Provenance.OriginalSource, "historical")
	})

	t.Run("cache_functionality", func(t *testing.T) {
		// Query same data twice to test caching
		from := time.Date(2025, 9, 7, 11, 0, 0, 0, time.UTC)
		until := time.Date(2025, 9, 7, 13, 0, 0, 0, time.UTC)

		// First query (should cache)
		envelopes1, err := coldData.GetHistoricalSlice(context.Background(), "kraken", "BTCUSD", from, until)
		require.NoError(t, err)

		// Second query (should use cache)
		envelopes2, err := coldData.GetHistoricalSlice(context.Background(), "kraken", "BTCUSD", from, until)
		require.NoError(t, err)

		assert.Equal(t, len(envelopes1), len(envelopes2))
		
		// Verify cache is being used (stats should show cached queries)
		stats := coldData.GetStats()
		cachedQueries, ok := stats["cached_queries"].(int)
		assert.True(t, ok)
		assert.Greater(t, cachedQueries, 0)
	})
}

func TestColdDataWriteAndRetrieve(t *testing.T) {
	tempDir := t.TempDir()

	config := data.ColdDataConfig{
		EnableParquet: true,
		EnableCSV:     true,
		DefaultFormat: "csv",
		BasePath:      tempDir,
		CacheExpiry:   "1m",
		EnableCache:   true,
	}

	coldData, err := data.NewColdData(config)
	require.NoError(t, err)

	t.Run("write_and_retrieve_data", func(t *testing.T) {
		// Create test data
		testEnvelopes := []*data.Envelope{
			{
				Symbol:     "ETHUSD",
				Venue:      "kraken",
				Timestamp:  time.Date(2025, 9, 7, 12, 0, 0, 0, time.UTC),
				SourceTier: data.TierCold,
				PriceData: map[string]interface{}{
					"open":  3000.0,
					"high":  3100.0,
					"low":   2950.0,
					"close": 3050.0,
				},
				VolumeData: map[string]interface{}{
					"volume": 500.0,
				},
				Provenance: data.ProvenanceInfo{
					OriginalSource:  "test_write",
					RetrievedAt: time.Date(2025, 9, 7, 12, 0, 0, 0, time.UTC),
					ConfidenceScore: 0.9,
				},
			},
		}

		// Write data to cold storage
		err := coldData.WriteData("kraken", "ETHUSD", testEnvelopes)
		require.NoError(t, err)

		// Verify file was created in correct location
		venueDir := filepath.Join(tempDir, "kraken")
		entries, err := os.ReadDir(venueDir)
		require.NoError(t, err)
		assert.Greater(t, len(entries), 0)

		// Find the created file
		var createdFile string
		for _, entry := range entries {
			if strings.Contains(entry.Name(), "ETHUSD") && strings.HasSuffix(entry.Name(), ".csv") {
				createdFile = filepath.Join(venueDir, entry.Name())
				break
			}
		}
		assert.NotEmpty(t, createdFile, "Expected to find created ETHUSD file")

		// Verify file content by reading it back
		reader := &data.CSVReader{}
		readEnvelopes, err := reader.LoadFile(createdFile, "kraken", "ETHUSD")
		require.NoError(t, err)
		require.Len(t, readEnvelopes, 1)

		readEnvelope := readEnvelopes[0]
		assert.Equal(t, "ETHUSD", readEnvelope.Symbol)
		assert.Equal(t, 3000.0, extractFloat(readEnvelope, "price", "open", 0))
		assert.Equal(t, 3100.0, extractFloat(readEnvelope, "price", "high", 0))
	})

	t.Run("config_toggle_format", func(t *testing.T) {
		// Test Parquet format selection
		configParquet := data.ColdDataConfig{
			EnableParquet: true,
			EnableCSV:     true,
			DefaultFormat: "parquet",
			BasePath:      tempDir,
			CacheExpiry:   "1m",
			EnableCache:   true,
		}

		coldDataParquet, err := data.NewColdData(configParquet)
		require.NoError(t, err)

		testData := []*data.Envelope{
			{
				Symbol:     "ADAUSD",
				Venue:      "kraken",
				Timestamp:  time.Now(),
				SourceTier: data.TierCold,
				PriceData:  map[string]interface{}{"open": 1.0, "high": 1.1, "low": 0.9, "close": 1.05},
			VolumeData: map[string]interface{}{"volume": 10000.0},
				Provenance: data.ProvenanceInfo{OriginalSource: "test", ConfidenceScore: 0.8},
			},
		}

		err = coldDataParquet.WriteData("kraken", "ADAUSD", testData)
		require.NoError(t, err)

		// Verify Parquet file was created
		venueDir := filepath.Join(tempDir, "kraken")
		entries, err := os.ReadDir(venueDir)
		require.NoError(t, err)

		found := false
		for _, entry := range entries {
			if strings.Contains(entry.Name(), "ADAUSD") && strings.HasSuffix(entry.Name(), ".parquet") {
				found = true
				break
			}
		}
		assert.True(t, found, "Expected to find created Parquet file")
	})
}

// Benchmark cold tier operations
func BenchmarkColdDataCSVLoad(b *testing.B) {
	tempDir := b.TempDir()
	
	// Create large CSV file for benchmarking
	csvContent := `timestamp,open,high,low,close,volume
`
	baseTime := time.Date(2025, 9, 7, 12, 0, 0, 0, time.UTC)
	for i := 0; i < 1000; i++ {
		timestamp := baseTime.Add(time.Duration(i) * time.Minute)
		csvContent += fmt.Sprintf("%s,%.2f,%.2f,%.2f,%.2f,%.2f\n", 
			timestamp.Format(time.RFC3339), 
			100.0+float64(i)*0.01, 
			105.0+float64(i)*0.01, 
			99.0+float64(i)*0.01, 
			103.0+float64(i)*0.01, 
			1000.0+float64(i)*10.0)
	}

	csvPath := filepath.Join(tempDir, "benchmark.csv")
	err := os.WriteFile(csvPath, []byte(csvContent), 0644)
	require.NoError(b, err)

	reader := &data.CSVReader{}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		envelopes, err := reader.LoadFile(csvPath, "kraken", "BTCUSD")
		require.NoError(b, err)
		require.Len(b, envelopes, 1000)
	}
}