package cold

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/sawpanic/cryptorun/internal/data"
	"github.com/sawpanic/cryptorun/internal/data/schema"
)

func TestParquetStore_WriteBatch(t *testing.T) {
	tempDir := t.TempDir()
	schemaRegistry := createTestSchemaRegistryForTesting(t, tempDir)
	
	config := DefaultParquetStoreConfig()
	config.ValidateSchema = true
	store := NewParquetStore(config, schemaRegistry)
	
	// Create test data
	envelopes := createTestEnvelopes(5)
	
	// Test writing
	filePath := filepath.Join(tempDir, "test.parquet")
	err := store.WriteBatch(context.Background(), filePath, envelopes)
	if err != nil {
		t.Fatalf("WriteBatch failed: %v", err)
	}
	
	// Verify file was created
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		t.Fatalf("Expected file %s was not created", filePath)
	}
}

func TestParquetStore_ReadBatch(t *testing.T) {
	tempDir := t.TempDir()
	schemaRegistry := createTestSchemaRegistryForTesting(t, tempDir)
	
	config := DefaultParquetStoreConfig()
	store := NewParquetStore(config, schemaRegistry)
	
	// Create test file
	envelopes := createTestEnvelopes(3)
	filePath := filepath.Join(tempDir, "test.parquet")
	err := store.WriteBatch(context.Background(), filePath, envelopes)
	if err != nil {
		t.Fatalf("WriteBatch failed: %v", err)
	}
	
	// Test reading
	readEnvelopes, err := store.ReadBatch(context.Background(), filePath, 0)
	if err != nil {
		t.Fatalf("ReadBatch failed: %v", err)
	}
	
	// Verify data (mock implementation returns fixed data)
	if len(readEnvelopes) == 0 {
		t.Fatalf("Expected data, got empty result")
	}
	
	for _, envelope := range readEnvelopes {
		if envelope.Venue != "kraken" {
			t.Errorf("Expected venue 'kraken', got '%s'", envelope.Venue)
		}
		if envelope.SourceTier != data.TierCold {
			t.Errorf("Expected tier 'cold', got '%s'", envelope.SourceTier)
		}
	}
}

func TestParquetStore_ScanRange(t *testing.T) {
	tempDir := t.TempDir()
	schemaRegistry := createTestSchemaRegistryForTesting(t, tempDir)
	
	config := DefaultParquetStoreConfig()
	store := NewParquetStore(config, schemaRegistry)
	
	// Create test file
	envelopes := createTestEnvelopes(10)
	filePath := filepath.Join(tempDir, "test.parquet")
	err := store.WriteBatch(context.Background(), filePath, envelopes)
	if err != nil {
		t.Fatalf("WriteBatch failed: %v", err)
	}
	
	// Test scanning with time range
	startTime := time.Now().Add(-25 * time.Hour)
	endTime := time.Now().Add(-20 * time.Hour)
	
	scannedEnvelopes, err := store.ScanRange(context.Background(), filePath, startTime, endTime)
	if err != nil {
		t.Fatalf("ScanRange failed: %v", err)
	}
	
	// Verify results are within time range
	for _, envelope := range scannedEnvelopes {
		if envelope.Timestamp.Before(startTime) || envelope.Timestamp.After(endTime) {
			t.Errorf("Envelope timestamp %v is outside range [%v, %v]", 
				envelope.Timestamp, startTime, endTime)
		}
	}
}

func TestParquetStore_ValidatePIT(t *testing.T) {
	tempDir := t.TempDir()
	schemaRegistry := createTestSchemaRegistryForTesting(t, tempDir)
	
	config := DefaultParquetStoreConfig()
	store := NewParquetStore(config, schemaRegistry)
	
	// Create test file
	envelopes := createTestEnvelopes(5)
	filePath := filepath.Join(tempDir, "test.parquet")
	err := store.WriteBatch(context.Background(), filePath, envelopes)
	if err != nil {
		t.Fatalf("WriteBatch failed: %v", err)
	}
	
	// Test PIT validation
	err = store.ValidatePIT(filePath)
	if err != nil {
		t.Fatalf("ValidatePIT failed: %v", err)
	}
}

func TestParquetStore_GetFileStats(t *testing.T) {
	tempDir := t.TempDir()
	schemaRegistry := createTestSchemaRegistryForTesting(t, tempDir)
	
	config := DefaultParquetStoreConfig()
	store := NewParquetStore(config, schemaRegistry)
	
	// Create test file
	envelopes := createTestEnvelopes(7)
	filePath := filepath.Join(tempDir, "test.parquet")
	err := store.WriteBatch(context.Background(), filePath, envelopes)
	if err != nil {
		t.Fatalf("WriteBatch failed: %v", err)
	}
	
	// Test getting stats
	stats, err := store.GetFileStats(filePath)
	if err != nil {
		t.Fatalf("GetFileStats failed: %v", err)
	}
	
	// Verify stats
	if stats["file_path"] != filePath {
		t.Errorf("Expected file_path '%s', got '%v'", filePath, stats["file_path"])
	}
	
	if rowCount, ok := stats["row_count"].(int); !ok || rowCount <= 0 {
		t.Errorf("Expected positive row_count, got %v", stats["row_count"])
	}
	
	if compression, ok := stats["compression"].(string); !ok || compression == "" {
		t.Errorf("Expected compression info, got %v", stats["compression"])
	}
}

func TestParquetStore_CompressionTypes(t *testing.T) {
	compressionTypes := []CompressionType{
		CompressionNone,
		CompressionGzip,
		CompressionLZ4,
		CompressionZSTD,
		CompressionSnappy,
	}
	
	for _, compression := range compressionTypes {
		t.Run(string(compression), func(t *testing.T) {
			tempDir := t.TempDir()
			schemaRegistry := createTestSchemaRegistryForTesting(t, tempDir)
			
			config := DefaultParquetStoreConfig()
			config.Compression = compression
			store := NewParquetStore(config, schemaRegistry)
			
			// Test with different compression types
			envelopes := createTestEnvelopes(3)
			filePath := filepath.Join(tempDir, "test_"+string(compression)+".parquet")
			
			err := store.WriteBatch(context.Background(), filePath, envelopes)
			if err != nil {
				t.Fatalf("WriteBatch with %s compression failed: %v", compression, err)
			}
			
			// Verify file was created
			if _, err := os.Stat(filePath); os.IsNotExist(err) {
				t.Fatalf("File with %s compression was not created", compression)
			}
		})
	}
}

func TestParquetStore_MetricsCallback(t *testing.T) {
	tempDir := t.TempDir()
	schemaRegistry := createTestSchemaRegistryForTesting(t, tempDir)
	
	config := DefaultParquetStoreConfig()
	store := NewParquetStore(config, schemaRegistry)
	
	// Set up metrics collection
	metricsCollected := make(map[string]int64)
	store.SetMetricsCallback(func(metric string, value int64) {
		metricsCollected[metric] += value
	})
	
	// Perform operations that should generate metrics
	envelopes := createTestEnvelopes(5)
	filePath := filepath.Join(tempDir, "test.parquet")
	
	err := store.WriteBatch(context.Background(), filePath, envelopes)
	if err != nil {
		t.Fatalf("WriteBatch failed: %v", err)
	}
	
	_, err = store.ReadBatch(context.Background(), filePath, 0)
	if err != nil {
		t.Fatalf("ReadBatch failed: %v", err)
	}
	
	err = store.ValidatePIT(filePath)
	if err != nil {
		t.Fatalf("ValidatePIT failed: %v", err)
	}
	
	// Verify metrics were collected
	expectedMetrics := []string{
		"cold_parquet_write_total",
		"cold_parquet_read_total",
		"cold_pit_validate_total",
	}
	
	for _, metric := range expectedMetrics {
		if count, exists := metricsCollected[metric]; !exists || count <= 0 {
			t.Errorf("Expected metric '%s' to be collected with positive value, got %d", metric, count)
		}
	}
}

func TestDefaultParquetStoreConfig(t *testing.T) {
	config := DefaultParquetStoreConfig()
	
	if config.Compression != CompressionGzip {
		t.Errorf("Expected default compression to be gzip, got %s", config.Compression)
	}
	
	if config.BatchSize != 1000 {
		t.Errorf("Expected default batch size to be 1000, got %d", config.BatchSize)
	}
	
	if !config.ValidateSchema {
		t.Error("Expected default ValidateSchema to be true")
	}
	
	if config.SchemaVersion != "1.0.0" {
		t.Errorf("Expected default schema version to be '1.0.0', got '%s'", config.SchemaVersion)
	}
	
	if config.MemoryLimit != 512 {
		t.Errorf("Expected default memory limit to be 512, got %d", config.MemoryLimit)
	}
}

// Helper functions for tests

func createTestSchemaRegistryForTesting(t testing.TB, tempDir string) *schema.SchemaRegistry {
	schemaDir := filepath.Join(tempDir, "schemas")
	if err := os.MkdirAll(schemaDir, 0755); err != nil {
		t.Fatalf("Failed to create schema directory: %v", err)
	}
	
	registry := schema.NewSchemaRegistry(schemaDir)
	if err := registry.CreateDefaultSchemas(); err != nil {
		t.Fatalf("Failed to create default schemas: %v", err)
	}
	
	if err := registry.LoadSchemas(); err != nil {
		t.Fatalf("Failed to load schemas: %v", err)
	}
	
	return registry
}

func createTestEnvelopes(count int) []*data.Envelope {
	var envelopes []*data.Envelope
	baseTime := time.Now().Add(-24 * time.Hour)
	
	for i := 0; i < count; i++ {
		timestamp := baseTime.Add(time.Duration(i) * time.Hour)
		envelope := data.NewEnvelope("kraken", "BTC-USD", data.TierCold,
			data.WithConfidenceScore(0.9),
		)
		envelope.Timestamp = timestamp
		envelope.Provenance.OriginalSource = "test_source"
		
		orderBookData := map[string]interface{}{
			"venue":          "kraken",
			"symbol":         "BTC-USD",
			"timestamp":      timestamp,
			"best_bid_price": 50000.0 + float64(i*100),
			"best_ask_price": 50010.0 + float64(i*100),
			"best_bid_qty":   1.5,
			"best_ask_qty":   2.0,
			"mid_price":      50005.0 + float64(i*100),
			"spread_bps":     20.0,
			"data_source":    "test",
		}
		
		envelope.OrderBook = orderBookData
		envelope.Checksum = envelope.GenerateChecksum(orderBookData, "test")
		
		envelopes = append(envelopes, envelope)
	}
	
	return envelopes
}

// Benchmarks

func BenchmarkParquetStore_WriteBatch(b *testing.B) {
	tempDir := b.TempDir()
	schemaRegistry := createTestSchemaRegistryForTesting(b, tempDir)
	
	config := DefaultParquetStoreConfig()
	config.ValidateSchema = false // Disable for benchmark
	store := NewParquetStore(config, schemaRegistry)
	
	envelopes := createBenchmarkEnvelopes(1000)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		filePath := filepath.Join(tempDir, "bench_"+string(rune(i))+".parquet")
		err := store.WriteBatch(context.Background(), filePath, envelopes)
		if err != nil {
			b.Fatalf("WriteBatch failed: %v", err)
		}
	}
}

func BenchmarkParquetStore_ReadBatch(b *testing.B) {
	tempDir := b.TempDir()
	schemaRegistry := createTestSchemaRegistryForTesting(b, tempDir)
	
	config := DefaultParquetStoreConfig()
	store := NewParquetStore(config, schemaRegistry)
	
	// Create test file
	envelopes := createBenchmarkEnvelopes(1000)
	filePath := filepath.Join(tempDir, "bench.parquet")
	err := store.WriteBatch(context.Background(), filePath, envelopes)
	if err != nil {
		b.Fatalf("WriteBatch setup failed: %v", err)
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := store.ReadBatch(context.Background(), filePath, 0)
		if err != nil {
			b.Fatalf("ReadBatch failed: %v", err)
		}
	}
}

func createBenchmarkEnvelopes(count int) []*data.Envelope {
	var envelopes []*data.Envelope
	baseTime := time.Now().Add(-24 * time.Hour)
	
	for i := 0; i < count; i++ {
		timestamp := baseTime.Add(time.Duration(i) * time.Minute)
		envelope := data.NewEnvelope("kraken", "BTC-USD", data.TierCold,
			data.WithConfidenceScore(0.85),
		)
		envelope.Timestamp = timestamp
		envelope.Provenance.OriginalSource = "benchmark_source"
		
		orderBookData := map[string]interface{}{
			"venue":          "kraken",
			"symbol":         "BTC-USD",
			"timestamp":      timestamp,
			"best_bid_price": 50000.0 + float64(i),
			"best_ask_price": 50010.0 + float64(i),
			"best_bid_qty":   1.5 + float64(i)*0.001,
			"best_ask_qty":   2.0 + float64(i)*0.001,
			"mid_price":      50005.0 + float64(i),
			"spread_bps":     20.0 + float64(i)*0.01,
			"data_source":    "benchmark",
		}
		
		envelope.OrderBook = orderBookData
		envelope.Checksum = envelope.GenerateChecksum(orderBookData, "benchmark")
		
		envelopes = append(envelopes, envelope)
	}
	
	return envelopes
}