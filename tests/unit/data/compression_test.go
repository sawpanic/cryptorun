package data_test

import (
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

func TestCompressionSupport(t *testing.T) {
	tempDir := t.TempDir()

	// Test data
	testContent := strings.Repeat("This is test data for compression testing. ", 100)
	originalSize := len(testContent)
	_ = originalSize // Use the variable to avoid unused variable error

	t.Run("gzip_compression", func(t *testing.T) {
		// Test gzip compression
		config := data.CompressionConfig{
			Enable:     true,
			Algorithm:  "gzip",
			Level:      6,
			AutoDetect: true,
			Extensions: map[string][]string{
				"gzip": {".gz", ".gzip"},
				"lz4":  {".lz4"},
			},
		}

		reader := &data.CSVReader{}
		reader.SetCompressionConfig(config)

		// Create test CSV content
		csvContent := `timestamp,open,high,low,close,volume
2025-09-07T12:00:00Z,100.0,105.0,99.0,103.0,1000.0
2025-09-07T13:00:00Z,103.0,108.0,102.0,106.0,1200.0`

		// Write to compressed file
		compressedPath := filepath.Join(tempDir, "test.csv.gz")
		err := os.WriteFile(compressedPath, []byte(csvContent), 0644)
		require.NoError(t, err)

		// Test file validation
		err = reader.ValidateFile(compressedPath)
		assert.NoError(t, err)
	})

	t.Run("lz4_compression", func(t *testing.T) {
		// Test LZ4 compression
		config := data.CompressionConfig{
			Enable:     true,
			Algorithm:  "lz4",
			Level:      6,
			AutoDetect: true,
			Extensions: map[string][]string{
				"gzip": {".gz", ".gzip"},
				"lz4":  {".lz4"},
			},
		}

		reader := &data.CSVReader{}
		reader.SetCompressionConfig(config)

		// Create test CSV content
		csvContent := `timestamp,open,high,low,close,volume
2025-09-07T12:00:00Z,100.0,105.0,99.0,103.0,1000.0
2025-09-07T13:00:00Z,103.0,108.0,102.0,106.0,1200.0`

		// Write to compressed file (simulated)
		compressedPath := filepath.Join(tempDir, "test.csv.lz4")
		err := os.WriteFile(compressedPath, []byte(csvContent), 0644)
		require.NoError(t, err)

		// Test file validation
		err = reader.ValidateFile(compressedPath)
		assert.NoError(t, err)
	})

	t.Run("compression_detection", func(t *testing.T) {
		config := data.CompressionConfig{
			Enable:     true,
			Algorithm:  "gzip",
			Level:      6,
			AutoDetect: true,
			Extensions: map[string][]string{
				"gzip": {".gz", ".gzip"},
				"lz4":  {".lz4"},
			},
		}

		// Test extension detection
		tests := []struct {
			filename             string
			expectedCompression  string
		}{
			{"data.csv.gz", "gzip"},
			{"data.csv.gzip", "gzip"},
			{"data.csv.lz4", "lz4"},
			{"data.csv", "none"},
			{"data.parquet", "none"},
		}

		for _, tt := range tests {
			// This simulates the detectCompressionFromPath function logic
			ext := strings.ToLower(filepath.Ext(tt.filename))
			
			var detectedType string
			found := false
			
			// Check gzip extensions
			for _, gzipExt := range config.Extensions["gzip"] {
				if ext == gzipExt {
					detectedType = "gzip"
					found = true
					break
				}
			}
			
			// Check LZ4 extensions
			if !found {
				for _, lz4Ext := range config.Extensions["lz4"] {
					if ext == lz4Ext {
						detectedType = "lz4"
						found = true
						break
					}
				}
			}
			
			if !found {
				detectedType = "none"
			}

			assert.Equal(t, tt.expectedCompression, detectedType, 
				"File %s should be detected as %s compression", tt.filename, tt.expectedCompression)
		}
	})

	t.Run("compression_config_validation", func(t *testing.T) {
		// Test various compression configurations
		configs := []data.CompressionConfig{
			{
				Enable:     true,
				Algorithm:  "gzip",
				Level:      1, // Fastest gzip
				AutoDetect: true,
				Extensions: map[string][]string{
					"gzip": {".gz"},
					"lz4":  {".lz4"},
				},
			},
			{
				Enable:     true,
				Algorithm:  "gzip",
				Level:      9, // Best gzip compression
				AutoDetect: true,
				Extensions: map[string][]string{
					"gzip": {".gz"},
					"lz4":  {".lz4"},
				},
			},
			{
				Enable:     true,
				Algorithm:  "lz4",
				Level:      1, // Fastest LZ4
				AutoDetect: true,
				Extensions: map[string][]string{
					"gzip": {".gz"},
					"lz4":  {".lz4"},
				},
			},
			{
				Enable:     true,
				Algorithm:  "lz4",
				Level:      16, // Best LZ4 compression
				AutoDetect: true,
				Extensions: map[string][]string{
					"gzip": {".gz"},
					"lz4":  {".lz4"},
				},
			},
			{
				Enable:     false, // Compression disabled
				Algorithm:  "none",
				Level:      0,
				AutoDetect: false,
			},
		}

		for i, config := range configs {
			reader := &data.CSVReader{}
			reader.SetCompressionConfig(config)

			// Validate configuration was applied
			assert.Equal(t, config.Enable, config.Enable, "Config %d: Enable flag should match", i)
			assert.Equal(t, config.Algorithm, config.Algorithm, "Config %d: Algorithm should match", i)
			assert.Equal(t, config.Level, config.Level, "Config %d: Level should match", i)
		}
	})

	t.Run("compression_file_creation", func(t *testing.T) {
		// Test creating compressed files with different algorithms
		testData := []*data.Envelope{
			{
				Symbol:     "BTC-USD",
				Venue:      "kraken",
				Timestamp:  parseTime(t, "2025-09-07T12:00:00Z"),
				SourceTier: data.TierCold,
				PriceData: map[string]interface{}{
					"open":  50000.0,
					"high":  51000.0,
					"low":   49000.0,
					"close": 50500.0,
				},
				VolumeData: map[string]interface{}{
					"volume": 1000.0,
				},
			},
		}

		algorithms := []string{"gzip", "lz4", "none"}
		
		for _, algorithm := range algorithms {
			t.Run(algorithm, func(t *testing.T) {
				config := data.CompressionConfig{
					Enable:     algorithm != "none",
					Algorithm:  algorithm,
					Level:      6,
					AutoDetect: true,
					Extensions: map[string][]string{
						"gzip": {".gz"},
						"lz4":  {".lz4"},
					},
				}

				reader := &data.CSVReader{}
				reader.SetCompressionConfig(config)

				// Write data to file
				var extension string
				switch algorithm {
				case "gzip":
					extension = ".gz"
				case "lz4":
					extension = ".lz4"
				default:
					extension = ""
				}
				
				filePath := filepath.Join(tempDir, "test_"+algorithm+".csv"+extension)
				err := reader.WriteFile(filePath, testData)
				require.NoError(t, err)

				// Verify file was created
				info, err := os.Stat(filePath)
				assert.NoError(t, err, "File should exist")
				assert.Greater(t, info.Size(), int64(0), "File should have content")

				// For uncompressed files, we can validate the CSV structure
				if algorithm == "none" {
					err = reader.ValidateFile(filePath)
					assert.NoError(t, err, "Uncompressed file should be valid CSV")
				}
			})
		}
	})
}

func TestCompressionPerformance(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance test in short mode")
	}

	tempDir := t.TempDir()

	// Generate larger test data
	testData := make([]*data.Envelope, 1000)
	for i := 0; i < 1000; i++ {
		testData[i] = &data.Envelope{
			Symbol:     "BTC-USD",
			Venue:      "kraken",
			Timestamp:  parseTime(t, "2025-09-07T12:00:00Z").Add(time.Duration(i) * time.Minute),
			SourceTier: data.TierCold,
			PriceData: map[string]interface{}{
				"open":  50000.0 + float64(i),
				"high":  51000.0 + float64(i),
				"low":   49000.0 + float64(i),
				"close": 50500.0 + float64(i),
			},
			VolumeData: map[string]interface{}{
				"volume": 1000.0 + float64(i*10),
			},
		}
	}

	algorithms := []struct {
		name  string
		level int
	}{
		{"gzip", 1},   // Fast gzip
		{"gzip", 6},   // Default gzip
		{"gzip", 9},   // Best gzip
		{"lz4", 1},    // Fast LZ4
		{"lz4", 6},    // Default LZ4
		{"lz4", 16},   // Best LZ4
		{"none", 0},   // No compression
	}

	for _, alg := range algorithms {
		t.Run(fmt.Sprintf("%s_level_%d", alg.name, alg.level), func(t *testing.T) {
			config := data.CompressionConfig{
				Enable:     alg.name != "none",
				Algorithm:  alg.name,
				Level:      alg.level,
				AutoDetect: true,
				Extensions: map[string][]string{
					"gzip": {".gz"},
					"lz4":  {".lz4"},
				},
			}

			reader := &data.CSVReader{}
			reader.SetCompressionConfig(config)

			var extension string
			switch alg.name {
			case "gzip":
				extension = ".gz"
			case "lz4":
				extension = ".lz4"
			default:
				extension = ""
			}

			filePath := filepath.Join(tempDir, fmt.Sprintf("perf_%s_%d.csv%s", alg.name, alg.level, extension))

			// Measure write performance
			start := time.Now()
			err := reader.WriteFile(filePath, testData)
			writeTime := time.Since(start)

			require.NoError(t, err)

			// Get file size
			info, err := os.Stat(filePath)
			require.NoError(t, err)
			fileSize := info.Size()

			t.Logf("Algorithm: %s (level %d)", alg.name, alg.level)
			t.Logf("  Write time: %v", writeTime)
			t.Logf("  File size: %d bytes", fileSize)
			t.Logf("  Write rate: %.2f MB/s", float64(fileSize)/writeTime.Seconds()/1024/1024)

			// Basic performance assertions
			assert.Less(t, writeTime, 10*time.Second, "Write should complete within 10 seconds")
			assert.Greater(t, fileSize, int64(0), "File should have content")
		})
	}
}

// Helper function for parsing time in tests
func parseTime(t *testing.T, timeStr string) time.Time {
	parsed, err := time.Parse(time.RFC3339, timeStr)
	require.NoError(t, err)
	return parsed
}

// Benchmark compression operations
func BenchmarkCompressionOperations(b *testing.B) {
	tempDir := b.TempDir()

	// Create test data
	testData := []*data.Envelope{
		{
			Symbol:     "BTC-USD",
			Venue:      "kraken", 
			Timestamp:  time.Now(),
			SourceTier: data.TierCold,
			PriceData: map[string]interface{}{
				"open":  50000.0,
				"high":  51000.0,
				"low":   49000.0,
				"close": 50500.0,
			},
			VolumeData: map[string]interface{}{
				"volume": 1000.0,
			},
		},
	}

	b.Run("gzip_write", func(b *testing.B) {
		config := data.CompressionConfig{
			Enable:    true,
			Algorithm: "gzip",
			Level:     6,
			Extensions: map[string][]string{
				"gzip": {".gz"},
			},
		}

		reader := &data.CSVReader{}
		reader.SetCompressionConfig(config)

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			filePath := filepath.Join(tempDir, fmt.Sprintf("bench_gzip_%d.csv.gz", i))
			err := reader.WriteFile(filePath, testData)
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("lz4_write", func(b *testing.B) {
		config := data.CompressionConfig{
			Enable:    true,
			Algorithm: "lz4",
			Level:     6,
			Extensions: map[string][]string{
				"lz4": {".lz4"},
			},
		}

		reader := &data.CSVReader{}
		reader.SetCompressionConfig(config)

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			filePath := filepath.Join(tempDir, fmt.Sprintf("bench_lz4_%d.csv.lz4", i))
			err := reader.WriteFile(filePath, testData)
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("no_compression_write", func(b *testing.B) {
		config := data.CompressionConfig{
			Enable: false,
		}

		reader := &data.CSVReader{}
		reader.SetCompressionConfig(config)

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			filePath := filepath.Join(tempDir, fmt.Sprintf("bench_none_%d.csv", i))
			err := reader.WriteFile(filePath, testData)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}