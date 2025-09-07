package data

import (
	"bytes"
	"compress/gzip"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sawpanic/cryptorun/internal/data"
)

// Helper function to extract float values from envelope data
func extractFloat(envelope *data.Envelope, dataType, field string, defaultValue float64) float64 {
	var dataMap map[string]interface{}
	var ok bool
	
	switch dataType {
	case "price":
		if envelope.PriceData != nil {
			dataMap, ok = envelope.PriceData.(map[string]interface{})
			if !ok {
				return defaultValue
			}
		}
	case "volume":
		if envelope.VolumeData != nil {
			dataMap, ok = envelope.VolumeData.(map[string]interface{})
			if !ok {
				return defaultValue
			}
		}
	default:
		return defaultValue
	}
	
	if dataMap == nil {
		return defaultValue
	}
	
	if value, exists := dataMap[field]; exists {
		if floatValue, ok := value.(float64); ok {
			return floatValue
		}
	}
	
	return defaultValue
}

func TestColdTierCompressionSupport(t *testing.T) {
	// Create temporary directory for test data
	tempDir := t.TempDir()

	t.Run("gzip_compression_config", func(t *testing.T) {
		config := data.ColdDataConfig{
			EnableParquet: false,
			EnableCSV:     true,
			DefaultFormat: "csv",
			BasePath:      tempDir,
			CacheExpiry:   "5m",
			EnableCache:   true,
			Compression: data.CompressionConfig{
				Enable:     true,
				Algorithm:  "gzip",
				Level:      6,
				AutoDetect: true,
				Extensions: map[string][]string{
					"gzip": {".gz", ".gzip"},
					"lz4":  {".lz4"},
				},
			},
		}

		coldData, err := data.NewColdData(config)
		require.NoError(t, err)
		require.NotNil(t, coldData)

		// Verify compression is properly configured
		assert.True(t, config.Compression.Enable)
		assert.Equal(t, "gzip", config.Compression.Algorithm)
		assert.Equal(t, 6, config.Compression.Level)
	})

	t.Run("compression_auto_detection", func(t *testing.T) {
		config := data.ColdDataConfig{
			EnableCSV: true,
			Compression: data.CompressionConfig{
				Enable:     true,
				Algorithm:  "gzip", // Default
				AutoDetect: true,
				Extensions: map[string][]string{
					"gzip": {".gz", ".gzip"},
					"lz4":  {".lz4"},
				},
			},
		}

		// Test different file extensions
		testCases := []struct {
			filename           string
			expectedCompression string
		}{
			{"data.csv.gz", "gzip"},
			{"data.csv.gzip", "gzip"},
			{"data.csv.lz4", "lz4"},
			{"data.csv", "none"}, // No compression for plain CSV
		}

		for _, tc := range testCases {
			t.Run(tc.filename, func(t *testing.T) {
				// This would be tested via the internal detectCompressionFromPath function
				// For now, we test the behavior indirectly through file operations
				csvReader := &data.CSVReader{}
				csvReader.SetCompressionConfig(config.Compression)

				// Test data
				testData := []*data.Envelope{
					{
						Symbol:     "BTCUSD",
						Venue:      "test",
						Timestamp:  time.Now(),
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
							ConfidenceScore: 0.9,
						},
					},
				}

				// Write compressed file
				filePath := filepath.Join(tempDir, tc.filename)
				err := csvReader.WriteFile(filePath, testData)
				require.NoError(t, err)

				// Verify file exists
				_, err = os.Stat(filePath)
				require.NoError(t, err)

				// For gzip files, verify they are actually compressed
				if strings.Contains(tc.filename, ".gz") {
					// Read raw file and verify it starts with gzip magic number
					content, err := os.ReadFile(filePath)
					require.NoError(t, err)
					assert.True(t, len(content) > 2)
					assert.Equal(t, byte(0x1f), content[0]) // Gzip magic number
					assert.Equal(t, byte(0x8b), content[1])
				}

				// Read back and verify
				envelopes, err := csvReader.LoadFile(filePath, "test", "BTCUSD")
				require.NoError(t, err)
				require.Len(t, envelopes, 1)

				envelope := envelopes[0]
				assert.Equal(t, "BTCUSD", envelope.Symbol)
				assert.Equal(t, 100.0, extractFloat(envelope, "price", "open", 0))
				assert.Equal(t, 103.0, extractFloat(envelope, "price", "close", 0))
			})
		}
	})

	t.Run("compression_disabled", func(t *testing.T) {
		config := data.ColdDataConfig{
			EnableCSV: true,
			Compression: data.CompressionConfig{
				Enable: false, // Disabled
			},
		}

		csvReader := &data.CSVReader{}
		csvReader.SetCompressionConfig(config.Compression)

		testData := []*data.Envelope{
			{
				Symbol:     "ETHUSD",
				Venue:      "test",
				Timestamp:  time.Now(),
				SourceTier: data.TierCold,
				PriceData: map[string]interface{}{
					"open": 200.0, "high": 210.0, "low": 195.0, "close": 205.0,
				},
				VolumeData: map[string]interface{}{"volume": 500.0},
				Provenance: data.ProvenanceInfo{OriginalSource: "test", ConfidenceScore: 0.8},
			},
		}

		// Even with .gz extension, should not compress when disabled
		filePath := filepath.Join(tempDir, "disabled_compression.csv.gz")
		err := csvReader.WriteFile(filePath, testData)
		require.NoError(t, err)

		// File should not be gzip compressed (no gzip magic number)
		content, err := os.ReadFile(filePath)
		require.NoError(t, err)
		
		// Should start with CSV header, not gzip magic
		contentStr := string(content)
		assert.True(t, strings.HasPrefix(contentStr, "timestamp,"))

		// Should still be readable
		envelopes, err := csvReader.LoadFile(filePath, "test", "ETHUSD")
		require.NoError(t, err)
		require.Len(t, envelopes, 1)
	})

	t.Run("gzip_compression_roundtrip", func(t *testing.T) {
		config := data.ColdDataConfig{
			EnableCSV: true,
			Compression: data.CompressionConfig{
				Enable:    true,
				Algorithm: "gzip",
				Level:     9, // Maximum compression
				Extensions: map[string][]string{
					"gzip": {".gz"},
				},
			},
		}

		csvReader := &data.CSVReader{}
		csvReader.SetCompressionConfig(config.Compression)

		// Create larger dataset for better compression testing
		var testData []*data.Envelope
		baseTime := time.Date(2025, 9, 7, 12, 0, 0, 0, time.UTC)
		
		for i := 0; i < 100; i++ {
			envelope := &data.Envelope{
				Symbol:     "BTCUSD",
				Venue:      "kraken",
				Timestamp:  baseTime.Add(time.Duration(i) * time.Minute),
				SourceTier: data.TierCold,
				PriceData: map[string]interface{}{
					"open":  50000.0 + float64(i),
					"high":  50100.0 + float64(i),
					"low":   49900.0 + float64(i),
					"close": 50050.0 + float64(i),
				},
				VolumeData: map[string]interface{}{
					"volume": 1000.0 + float64(i*10),
				},
				Provenance: data.ProvenanceInfo{
					OriginalSource:  "historical_test",
					ConfidenceScore: 0.95,
				},
			}
			testData = append(testData, envelope)
		}

		// Write compressed file
		gzipPath := filepath.Join(tempDir, "large_data.csv.gz")
		err := csvReader.WriteFile(gzipPath, testData)
		require.NoError(t, err)

		// Verify compression actually occurred
		compressedSize, err := getFileSize(gzipPath)
		require.NoError(t, err)

		// Write uncompressed version for comparison
		uncompressedReader := &data.CSVReader{}
		uncompressedReader.SetCompressionConfig(data.CompressionConfig{Enable: false})
		
		uncompressedPath := filepath.Join(tempDir, "large_data.csv")
		err = uncompressedReader.WriteFile(uncompressedPath, testData)
		require.NoError(t, err)

		uncompressedSize, err := getFileSize(uncompressedPath)
		require.NoError(t, err)

		// Compressed file should be significantly smaller
		compressionRatio := float64(compressedSize) / float64(uncompressedSize)
		assert.True(t, compressionRatio < 0.5, "Compression ratio should be < 50%%, got %.2f", compressionRatio)

		// Read back compressed data and verify integrity
		readEnvelopes, err := csvReader.LoadFile(gzipPath, "kraken", "BTCUSD")
		require.NoError(t, err)
		require.Len(t, readEnvelopes, 100)

		// Verify first and last records
		first := readEnvelopes[0]
		assert.Equal(t, "BTCUSD", first.Symbol)
		assert.Equal(t, 50000.0, extractFloat(first, "price", "open", 0))
		assert.Equal(t, 1000.0, extractFloat(first, "volume", "volume", 0))

		last := readEnvelopes[99]
		assert.Equal(t, 50099.0, extractFloat(last, "price", "open", 0))
		assert.Equal(t, 1990.0, extractFloat(last, "volume", "volume", 0))
	})

	t.Run("lz4_compression_mock", func(t *testing.T) {
		// Since we don't have LZ4 library, test the mock implementation
		config := data.ColdDataConfig{
			EnableCSV: true,
			Compression: data.CompressionConfig{
				Enable:    true,
				Algorithm: "lz4",
				Level:     4,
				Extensions: map[string][]string{
					"lz4": {".lz4"},
				},
			},
		}

		csvReader := &data.CSVReader{}
		csvReader.SetCompressionConfig(config.Compression)

		testData := []*data.Envelope{
			{
				Symbol:     "SOLUSD",
				Venue:      "test",
				Timestamp:  time.Now(),
				SourceTier: data.TierCold,
				PriceData:  map[string]interface{}{"open": 150.0, "high": 155.0, "low": 148.0, "close": 152.0},
				VolumeData: map[string]interface{}{"volume": 750.0},
				Provenance: data.ProvenanceInfo{OriginalSource: "test_lz4", ConfidenceScore: 0.85},
			},
		}

		// Write with LZ4 extension (mock implementation should handle gracefully)
		lz4Path := filepath.Join(tempDir, "test_data.csv.lz4")
		err := csvReader.WriteFile(lz4Path, testData)
		require.NoError(t, err)

		// Read back and verify (mock should work transparently)
		envelopes, err := csvReader.LoadFile(lz4Path, "test", "SOLUSD")
		require.NoError(t, err)
		require.Len(t, envelopes, 1)

		envelope := envelopes[0]
		assert.Equal(t, "SOLUSD", envelope.Symbol)
		assert.Equal(t, 150.0, extractFloat(envelope, "price", "open", 0))
	})
}

func TestCompressionBenchmarks(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping benchmark tests in short mode")
	}

	tempDir := t.TempDir()

	// Create test dataset
	var testData []*data.Envelope
	baseTime := time.Date(2025, 9, 7, 12, 0, 0, 0, time.UTC)
	
	for i := 0; i < 1000; i++ {
		envelope := &data.Envelope{
			Symbol:     "BTCUSD",
			Venue:      "kraken",
			Timestamp:  baseTime.Add(time.Duration(i) * time.Second),
			SourceTier: data.TierCold,
			PriceData: map[string]interface{}{
				"open":  50000.0 + float64(i%100),
				"high":  50100.0 + float64(i%100),
				"low":   49900.0 + float64(i%100),
				"close": 50050.0 + float64(i%100),
			},
			VolumeData: map[string]interface{}{
				"volume": 1000.0 + float64(i),
			},
			Provenance: data.ProvenanceInfo{
				OriginalSource:  "benchmark_test",
				ConfidenceScore: 0.95,
			},
		}
		testData = append(testData, envelope)
	}

	benchmarks := []struct {
		name        string
		compression data.CompressionConfig
		filename    string
	}{
		{
			name:        "no_compression",
			compression: data.CompressionConfig{Enable: false},
			filename:    "benchmark_none.csv",
		},
		{
			name: "gzip_level_1",
			compression: data.CompressionConfig{
				Enable: true, Algorithm: "gzip", Level: 1,
				Extensions: map[string][]string{"gzip": {".gz"}},
			},
			filename: "benchmark_gzip1.csv.gz",
		},
		{
			name: "gzip_level_6",
			compression: data.CompressionConfig{
				Enable: true, Algorithm: "gzip", Level: 6,
				Extensions: map[string][]string{"gzip": {".gz"}},
			},
			filename: "benchmark_gzip6.csv.gz",
		},
		{
			name: "gzip_level_9",
			compression: data.CompressionConfig{
				Enable: true, Algorithm: "gzip", Level: 9,
				Extensions: map[string][]string{"gzip": {".gz"}},
			},
			filename: "benchmark_gzip9.csv.gz",
		},
	}

	results := make(map[string]benchmarkResult)

	for _, bm := range benchmarks {
		t.Run(bm.name, func(t *testing.T) {
			csvReader := &data.CSVReader{}
			csvReader.SetCompressionConfig(bm.compression)

			filePath := filepath.Join(tempDir, bm.filename)

			// Measure write time
			writeStart := time.Now()
			err := csvReader.WriteFile(filePath, testData)
			writeTime := time.Since(writeStart)
			require.NoError(t, err)

			// Get file size
			fileSize, err := getFileSize(filePath)
			require.NoError(t, err)

			// Measure read time
			readStart := time.Now()
			envelopes, err := csvReader.LoadFile(filePath, "kraken", "BTCUSD")
			readTime := time.Since(readStart)
			require.NoError(t, err)
			require.Len(t, envelopes, 1000)

			results[bm.name] = benchmarkResult{
				fileSize:  fileSize,
				writeTime: writeTime,
				readTime:  readTime,
			}

			t.Logf("%s: Size=%d bytes, Write=%v, Read=%v", 
				bm.name, fileSize, writeTime, readTime)
		})
	}

	// Compare results
	if uncompressed, ok := results["no_compression"]; ok {
		for name, result := range results {
			if name == "no_compression" {
				continue
			}
			
			compressionRatio := float64(result.fileSize) / float64(uncompressed.fileSize)
			t.Logf("%s vs uncompressed: Size ratio=%.2f, Write ratio=%.2f, Read ratio=%.2f",
				name, compressionRatio,
				float64(result.writeTime)/float64(uncompressed.writeTime),
				float64(result.readTime)/float64(uncompressed.readTime))
		}
	}
}

// Helper types and functions

type benchmarkResult struct {
	fileSize  int64
	writeTime time.Duration
	readTime  time.Duration
}

func getFileSize(filePath string) (int64, error) {
	info, err := os.Stat(filePath)
	if err != nil {
		return 0, err
	}
	return info.Size(), nil
}

// Helper function to test gzip compression manually
func TestGzipCompressionDetails(t *testing.T) {
	// Use larger data for meaningful compression
	var originalBuilder strings.Builder
	originalBuilder.WriteString("timestamp,open,high,low,close,volume\n")
	
	// Add multiple rows to ensure compression benefit
	for i := 0; i < 100; i++ {
		originalBuilder.WriteString("2025-09-07T12:00:00Z,100.0,105.0,99.0,103.0,1000.0\n")
	}
	original := originalBuilder.String()
	
	// Compress with gzip
	var compressed bytes.Buffer
	gzWriter := gzip.NewWriter(&compressed)
	_, err := gzWriter.Write([]byte(original))
	require.NoError(t, err)
	err = gzWriter.Close()
	require.NoError(t, err)
	
	// For larger repetitive data, compression should occur
	t.Logf("Original: %d bytes, Compressed: %d bytes (%.1f%% ratio)", 
		len(original), compressed.Len(), 
		float64(compressed.Len())/float64(len(original))*100)
	
	// Decompress and verify
	gzReader, err := gzip.NewReader(&compressed)
	require.NoError(t, err)
	defer gzReader.Close()
	
	var decompressed bytes.Buffer
	_, err = decompressed.ReadFrom(gzReader)
	require.NoError(t, err)
	
	assert.Equal(t, original, decompressed.String())
}