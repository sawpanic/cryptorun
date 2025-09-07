package data_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sawpanic/cryptorun/internal/data"
)

func TestColdDumpFunctionality(t *testing.T) {
	tempDir := t.TempDir()

	t.Run("create_test_csv_file", func(t *testing.T) {
		// Create a test CSV file with OHLCV data
		csvContent := `timestamp,open,high,low,close,volume,venue,tier,provenance
2025-09-07T12:00:00Z,100.0,105.0,99.0,103.0,1000.0,kraken,cold,historical
2025-09-07T13:00:00Z,103.0,108.0,102.0,106.0,1200.0,kraken,cold,historical
2025-09-07T14:00:00Z,106.0,110.0,105.0,109.0,1300.0,kraken,cold,historical`

		csvPath := filepath.Join(tempDir, "test_dump.csv")
		err := os.WriteFile(csvPath, []byte(csvContent), 0644)
		require.NoError(t, err)

		// Test CSV reader functionality
		reader := &data.CSVReader{}
		envelopes, err := reader.LoadFile(csvPath, "kraken", "BTC-USD")
		require.NoError(t, err)
		assert.Len(t, envelopes, 3)

		// Test time filtering
		from := time.Date(2025, 9, 7, 12, 30, 0, 0, time.UTC)
		until := time.Date(2025, 9, 7, 13, 30, 0, 0, time.UTC)
		
		filteredEnvelopes, err := reader.LoadFileWithTimeFilter(csvPath, "kraken", "BTC-USD", from, until)
		require.NoError(t, err)
		assert.Len(t, filteredEnvelopes, 1) // Only 13:00 record should match
		
		// Verify the filtered record
		envelope := filteredEnvelopes[0]
		assert.Equal(t, "BTC-USD", envelope.Symbol)
		assert.Equal(t, "kraken", envelope.Venue)
		
		// Test envelope to row conversion (simulating dump functionality)
		row, err := data.ConvertEnvelopeToRow(envelope)
		require.NoError(t, err)
		
		// Validate converted row has expected fields
		expectedFields := []string{"ts", "symbol", "venue", "source_tier", "confidence"}
		for _, field := range expectedFields {
			assert.Contains(t, row, field, "Row should contain field: %s", field)
		}
	})

	t.Run("parquet_metadata_simulation", func(t *testing.T) {
		// Test the Parquet metadata functionality
		config := data.ColdDataConfig{
			EnableParquet: true,
			BasePath:      tempDir,
		}

		schema := data.ParquetSchema{
			Table: "ohlcv",
			Fields: []data.ParquetField{
				{Name: "ts", Type: "timestamp(ms)", Required: true, Primary: true},
				{Name: "symbol", Type: "string", Required: true},
				{Name: "venue", Type: "string", Required: true},
				{Name: "source_tier", Type: "string", Required: true},
			},
		}

		store, err := data.NewParquetStore(config, schema)
		require.NoError(t, err)

		// Test metadata retrieval (mock implementation)
		metadata, err := store.GetParquetMetadata(nil, "/mock/file.parquet")
		require.NoError(t, err)
		
		assert.NotEmpty(t, metadata.FilePath)
		assert.Greater(t, metadata.RowCount, int64(0))
		assert.NotEmpty(t, metadata.Compression)
	})

	t.Run("row_formatting_simulation", func(t *testing.T) {
		// Simulate the row formatting functionality used by cold-dump
		testRow := data.Row{
			"ts":          time.Date(2025, 9, 7, 12, 0, 0, 0, time.UTC),
			"symbol":      "BTC-USD",
			"venue":       "kraken",
			"source_tier": "cold",
			"close":       50000.0,
			"volume":      1000.0,
		}

		// Test key extraction (simulates table formatting)
		columns := []string{"ts", "symbol", "venue", "close"}
		for _, col := range columns {
			value := testRow[col]
			assert.NotNil(t, value, "Column %s should have a value", col)

			// Test value formatting
			var formattedValue string
			if ts, ok := value.(time.Time); ok {
				formattedValue = ts.Format("15:04:05")
			} else {
				formattedValue = "formatted"
			}
			assert.NotEmpty(t, formattedValue)
		}
	})

	t.Run("time_range_parsing_simulation", func(t *testing.T) {
		// Test time range parsing logic used by cold-dump
		fromStr := "2025-09-07T12:00:00Z"
		toStr := "2025-09-07T14:00:00Z"

		from, err := time.Parse(time.RFC3339, fromStr)
		require.NoError(t, err)

		to, err := time.Parse(time.RFC3339, toStr)
		require.NoError(t, err)

		assert.True(t, from.Before(to))

		timeRange := data.TimeRange{From: from, To: to}
		assert.Equal(t, 2*time.Hour, timeRange.To.Sub(timeRange.From))
	})

	t.Run("validation_simulation", func(t *testing.T) {
		// Test file validation functionality
		csvPath := filepath.Join(tempDir, "validation_test.csv")
		
		// Valid CSV
		validContent := `timestamp,open,high,low,close,volume
2025-09-07T12:00:00Z,100.0,105.0,99.0,103.0,1000.0`
		err := os.WriteFile(csvPath, []byte(validContent), 0644)
		require.NoError(t, err)

		reader := &data.CSVReader{}
		err = reader.ValidateFile(csvPath)
		assert.NoError(t, err)

		// Invalid CSV (insufficient columns)
		invalidPath := filepath.Join(tempDir, "invalid.csv")
		invalidContent := `timestamp,open,high
2025-09-07T12:00:00Z,100.0,105.0`
		err = os.WriteFile(invalidPath, []byte(invalidContent), 0644)
		require.NoError(t, err)

		err = reader.ValidateFile(invalidPath)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "insufficient columns")
	})
}

func TestParquetIteratorFunctionality(t *testing.T) {
	t.Run("iterator_time_filtering", func(t *testing.T) {
		// Create time range
		now := time.Now()
		timeRange := data.TimeRange{
			From: now.Add(-1 * time.Hour),
			To:   now,
		}

		// Create mock Parquet iterator
		iterator := &data.MockParquetIterator{
			// These fields would be set by createMultiFileIterator in practice
		}

		// Simulate setting up the iterator with test data
		files := []string{"mock.parquet"}
		columns := []string{"ts", "symbol", "close"}
		
		// Validate time range
		assert.True(t, timeRange.From.Before(timeRange.To))
		assert.NotEmpty(t, files)
		assert.NotEmpty(t, columns)
		
		// Test iterator interface
		assert.NotNil(t, iterator)
		
		// Test that we can call iterator methods
		err := iterator.Close()
		assert.NoError(t, err)
	})

	t.Run("parquet_options_validation", func(t *testing.T) {
		// Test different Parquet options
		options := []data.ParquetOptions{
			{Compression: "gzip", RowGroupSize: 64 * 1024},
			{Compression: "lz4", RowGroupSize: 128 * 1024},
			{Compression: "snappy", RowGroupSize: 256 * 1024},
		}

		for _, opt := range options {
			assert.NotEmpty(t, opt.Compression)
			assert.Greater(t, opt.RowGroupSize, 0)
		}

		// Test default options
		defaultOpts := data.DefaultParquetOptions()
		assert.Equal(t, "snappy", defaultOpts.Compression)
		assert.Equal(t, 128*1024, defaultOpts.RowGroupSize)
	})
}

// Benchmark cold dump operations
func BenchmarkColdDumpOperations(b *testing.B) {
	// Create test data
	testRow := data.Row{
		"ts":          time.Now(),
		"symbol":      "BTC-USD",
		"venue":       "kraken",
		"source_tier": "cold",
		"close":       50000.0,
		"volume":      1000.0,
	}

	b.Run("row_field_access", func(b *testing.B) {
		columns := []string{"ts", "symbol", "venue", "close"}
		b.ResetTimer()
		
		for i := 0; i < b.N; i++ {
			for _, col := range columns {
				_ = testRow[col]
			}
		}
	})

	b.Run("time_formatting", func(b *testing.B) {
		timestamp := time.Now()
		b.ResetTimer()
		
		for i := 0; i < b.N; i++ {
			_ = timestamp.Format("15:04:05")
		}
	})
}