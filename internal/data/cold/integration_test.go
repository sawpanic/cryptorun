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

// TestParquetStoreIntegration tests the full round-trip of writing and reading Parquet files
func TestParquetStoreIntegration(t *testing.T) {
	tempDir := t.TempDir()
	
	// Create schema registry
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
	
	// Test with different compression types
	compressionTypes := []CompressionType{
		CompressionNone,
		CompressionGzip,
		CompressionLZ4,
	}
	
	for _, compression := range compressionTypes {
		t.Run(string(compression), func(t *testing.T) {
			config := ParquetStoreConfig{
				Compression:    compression,
				BatchSize:      100,
				ValidateSchema: true,
				SchemaVersion:  "1.0.0",
				MemoryLimit:    256,
			}
			
			store := NewParquetStore(config, registry)
			
			// Set up metrics collection
			metricsCollected := make(map[string]int64)
			store.SetMetricsCallback(func(metric string, value int64) {
				metricsCollected[metric]++
			})
			
			// Create test data (500 rows)
			envelopes := createLargeTestDataset(500)
			filePath := filepath.Join(tempDir, "test_"+string(compression)+".parquet")
			
			// Write data
			ctx := context.Background()
			err := store.WriteBatch(ctx, filePath, envelopes)
			if err != nil {
				t.Fatalf("WriteBatch failed with %s compression: %v", compression, err)
			}
			
			// Verify file exists and has content
			info, err := os.Stat(filePath)
			if err != nil {
				t.Fatalf("File does not exist after write: %v", err)
			}
			if info.Size() == 0 {
				t.Fatalf("File is empty after write")
			}
			
			// Read back the data
			readEnvelopes, err := store.ReadBatch(ctx, filePath, 0)
			if err != nil {
				t.Fatalf("ReadBatch failed: %v", err)
			}
			
			// Verify data integrity (at least some data returned)
			if len(readEnvelopes) == 0 {
				t.Fatalf("No data returned from read")
			}
			
			// Test PIT validation
			err = store.ValidatePIT(filePath)
			if err != nil {
				t.Fatalf("PIT validation failed: %v", err)
			}
			
			// Test file stats
			stats, err := store.GetFileStats(filePath)
			if err != nil {
				t.Fatalf("GetFileStats failed: %v", err)
			}
			
			// Verify stats content
			if stats["file_path"] != filePath {
				t.Errorf("Wrong file path in stats: %v", stats["file_path"])
			}
			if stats["compression"] != string(compression) {
				t.Errorf("Wrong compression in stats: %v", stats["compression"])
			}
			
			// Test time range scanning
			// Use a wide time range to ensure we catch some data
			startTime := envelopes[0].Timestamp.Add(-1 * 60 * 60) // 1 hour before first
			endTime := envelopes[len(envelopes)-1].Timestamp.Add(60 * 60) // 1 hour after last
			
			scannedEnvelopes, err := store.ScanRange(ctx, filePath, startTime, endTime)
			if err != nil {
				t.Fatalf("ScanRange failed: %v", err)
			}
			
			// Should return some data within the range
			if len(scannedEnvelopes) == 0 {
				t.Errorf("ScanRange returned no data for wide time range")
			}
			
			// Verify metrics were collected
			if metricsCollected["cold_parquet_write_total"] == 0 {
				t.Error("Write metrics not collected")
			}
			if metricsCollected["cold_parquet_read_total"] == 0 {
				t.Error("Read metrics not collected")
			}
		})
	}
}

// TestSchemaValidation tests schema validation functionality
func TestSchemaValidation(t *testing.T) {
	tempDir := t.TempDir()
	
	// Create schema registry
	schemaDir := filepath.Join(tempDir, "schemas")
	registry := schema.NewSchemaRegistry(schemaDir)
	if err := registry.CreateDefaultSchemas(); err != nil {
		t.Fatalf("Failed to create schemas: %v", err)
	}
	if err := registry.LoadSchemas(); err != nil {
		t.Fatalf("Failed to load schemas: %v", err)
	}
	
	config := DefaultParquetStoreConfig()
	config.ValidateSchema = true
	store := NewParquetStore(config, registry)
	
	// Test with valid data
	validEnvelopes := createLargeTestDataset(10)
	filePath := filepath.Join(tempDir, "valid.parquet")
	
	err := store.WriteBatch(context.Background(), filePath, validEnvelopes)
	if err != nil {
		t.Fatalf("Valid data should not fail validation: %v", err)
	}
	
	// Test with schema validation disabled
	config.ValidateSchema = false
	storeNoValidation := NewParquetStore(config, registry)
	
	err = storeNoValidation.WriteBatch(context.Background(), filePath+"_no_validation", validEnvelopes)
	if err != nil {
		t.Fatalf("Data should write without validation: %v", err)
	}
}

// createLargeTestDataset creates a larger dataset for integration testing
func createLargeTestDataset(count int) []*data.Envelope {
	var envelopes []*data.Envelope
	baseTime := time.Now().Add(-24 * time.Hour)
	
	venues := []string{"kraken", "coinbase", "binance"}
	symbols := []string{"BTC-USD", "ETH-USD", "SOL-USD"}
	
	for i := 0; i < count; i++ {
		venue := venues[i%len(venues)]
		symbol := symbols[i%len(symbols)]
		
		timestamp := baseTime.Add(time.Duration(i) * time.Minute)
		envelope := data.NewEnvelope(venue, symbol, data.TierCold,
			data.WithConfidenceScore(0.8+float64(i%20)*0.01),
		)
		envelope.Timestamp = timestamp
		envelope.Provenance.OriginalSource = "integration_test_source"
		envelope.Provenance.LatencyMS = int64(10 + i%50)
		
		// Create realistic order book data
		basePrice := 50000.0 + float64(i*10)
		spread := 20.0 + float64(i%10)
		
		orderBookData := map[string]interface{}{
			"venue":          venue,
			"symbol":         symbol,
			"timestamp":      timestamp,
			"best_bid_price": basePrice - spread/2,
			"best_ask_price": basePrice + spread/2,
			"best_bid_qty":   1.5 + float64(i%100)*0.01,
			"best_ask_qty":   2.0 + float64(i%100)*0.01,
			"mid_price":      basePrice,
			"spread_bps":     spread,
			"data_source":    "integration_test",
		}
		
		envelope.OrderBook = orderBookData
		envelope.Checksum = envelope.GenerateChecksum(orderBookData, "integration")
		
		envelopes = append(envelopes, envelope)
	}
	
	return envelopes
}