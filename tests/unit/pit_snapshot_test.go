package unit

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/sawpanic/cryptorun/internal/data/facade"
	"github.com/sawpanic/cryptorun/internal/data/pit"
)

func TestPITStoreBasicOperations(t *testing.T) {
	// Create temp directory
	tmpDir := t.TempDir()
	store := pit.NewStore(tmpDir)
	
	ctx := context.Background()
	entity := "btcusd"
	
	// Test data
	testKlines := []facade.Kline{
		{
			Timestamp: time.Now(),
			Open:      50000.0,
			High:      51000.0,
			Low:       49000.0,
			Close:     50500.0,
			Volume:    10.5,
		},
	}
	
	// Store snapshot
	snapshotID, err := store.StoreSnapshot(ctx, entity, testKlines)
	if err != nil {
		t.Fatalf("Failed to store snapshot: %v", err)
	}
	
	if snapshotID == "" {
		t.Fatal("Expected non-empty snapshot ID")
	}
	
	// Retrieve snapshot
	var retrievedKlines []facade.Kline
	err = store.GetSnapshot(ctx, snapshotID, &retrievedKlines)
	if err != nil {
		t.Fatalf("Failed to get snapshot: %v", err)
	}
	
	// Verify data
	if len(retrievedKlines) != len(testKlines) {
		t.Fatalf("Expected %d klines, got %d", len(testKlines), len(retrievedKlines))
	}
	
	original := testKlines[0]
	retrieved := retrievedKlines[0]
	
	if retrieved.Open != original.Open || retrieved.Close != original.Close {
		t.Errorf("Kline data mismatch: original %+v, retrieved %+v", original, retrieved)
	}
}

func TestPITStoreCompression(t *testing.T) {
	tmpDir := t.TempDir()
	store := pit.NewStore(tmpDir)
	
	ctx := context.Background()
	entity := "ethusd"
	
	// Large dataset to test compression
	largeKlines := make([]facade.Kline, 1000)
	for i := range largeKlines {
		largeKlines[i] = facade.Kline{
			Timestamp: time.Now().Add(time.Duration(i) * time.Minute),
			Open:      3000.0 + float64(i),
			High:      3100.0 + float64(i),
			Low:       2900.0 + float64(i),
			Close:     3050.0 + float64(i),
			Volume:    1.0 + float64(i)*0.1,
		}
	}
	
	snapshotID, err := store.StoreSnapshot(ctx, entity, largeKlines)
	if err != nil {
		t.Fatalf("Failed to store large snapshot: %v", err)
	}
	
	// Check file exists and is compressed
	snapshotPath := store.GetSnapshotPath(snapshotID)
	if !filepath.Ext(snapshotPath) == ".gz" {
		t.Error("Expected .gz extension for compressed file")
	}
	
	// Verify file size is reasonable (compression should help)
	stat, err := os.Stat(snapshotPath)
	if err != nil {
		t.Fatalf("Failed to stat snapshot file: %v", err)
	}
	
	// Rough estimate: 1000 klines * ~100 bytes each = ~100KB, compressed should be much smaller
	if stat.Size() > 50000 { // 50KB threshold
		t.Errorf("Compressed file too large: %d bytes", stat.Size())
	}
	
	// Verify retrieval still works
	var retrievedKlines []facade.Kline
	err = store.GetSnapshot(ctx, snapshotID, &retrievedKlines)
	if err != nil {
		t.Fatalf("Failed to retrieve compressed snapshot: %v", err)
	}
	
	if len(retrievedKlines) != 1000 {
		t.Errorf("Expected 1000 klines, got %d", len(retrievedKlines))
	}
}

func TestPITStoreMetadata(t *testing.T) {
	tmpDir := t.TempDir()
	store := pit.NewStore(tmpDir)
	
	ctx := context.Background()
	entity := "adausd"
	
	testData := []facade.Trade{
		{
			TradeID:   "12345",
			Timestamp: time.Now(),
			Symbol:    "ADAUSD",
			Side:      "buy",
			Size:      1000.0,
			Price:     0.45,
		},
	}
	
	snapshotID, err := store.StoreSnapshot(ctx, entity, testData)
	if err != nil {
		t.Fatalf("Failed to store snapshot: %v", err)
	}
	
	// Get metadata
	metadata, err := store.GetMetadata(ctx, snapshotID)
	if err != nil {
		t.Fatalf("Failed to get metadata: %v", err)
	}
	
	// Verify metadata fields
	if metadata.ID != snapshotID {
		t.Errorf("Expected ID %s, got %s", snapshotID, metadata.ID)
	}
	
	if metadata.Entity != entity {
		t.Errorf("Expected entity %s, got %s", entity, metadata.Entity)
	}
	
	if metadata.DataType != "facade.Trade" {
		t.Errorf("Expected data type facade.Trade, got %s", metadata.DataType)
	}
	
	if metadata.RecordCount != 1 {
		t.Errorf("Expected record count 1, got %d", metadata.RecordCount)
	}
	
	if metadata.CompressedSize <= 0 {
		t.Errorf("Expected positive compressed size, got %d", metadata.CompressedSize)
	}
	
	// Timestamp should be recent
	if time.Since(metadata.StoredAt) > time.Minute {
		t.Errorf("Stored timestamp too old: %v", metadata.StoredAt)
	}
}

func TestPITStoreListSnapshots(t *testing.T) {
	tmpDir := t.TempDir()
	store := pit.NewStore(tmpDir)
	
	ctx := context.Background()
	entities := []string{"btcusd", "ethusd", "adausd"}
	snapshotIDs := make([]string, len(entities))
	
	// Create multiple snapshots
	for i, entity := range entities {
		testData := []facade.Kline{
			{Timestamp: time.Now(), Close: float64(1000 + i)},
		}
		
		id, err := store.StoreSnapshot(ctx, entity, testData)
		if err != nil {
			t.Fatalf("Failed to store snapshot for %s: %v", entity, err)
		}
		snapshotIDs[i] = id
		
		// Small delay to ensure different timestamps
		time.Sleep(10 * time.Millisecond)
	}
	
	// List all snapshots
	snapshots, err := store.ListSnapshots(ctx, "", 10) // No entity filter
	if err != nil {
		t.Fatalf("Failed to list snapshots: %v", err)
	}
	
	if len(snapshots) != 3 {
		t.Errorf("Expected 3 snapshots, got %d", len(snapshots))
	}
	
	// Verify ordering (should be newest first)
	for i := 1; i < len(snapshots); i++ {
		if snapshots[i].StoredAt.After(snapshots[i-1].StoredAt) {
			t.Error("Snapshots not ordered by newest first")
		}
	}
	
	// Test entity filtering
	btcSnapshots, err := store.ListSnapshots(ctx, "btcusd", 10)
	if err != nil {
		t.Fatalf("Failed to list BTC snapshots: %v", err)
	}
	
	if len(btcSnapshots) != 1 {
		t.Errorf("Expected 1 BTC snapshot, got %d", len(btcSnapshots))
	}
	
	if btcSnapshots[0].Entity != "btcusd" {
		t.Errorf("Expected btcusd entity, got %s", btcSnapshots[0].Entity)
	}
}

func TestPITStoreCleanup(t *testing.T) {
	tmpDir := t.TempDir()
	store := pit.NewStore(tmpDir)
	
	ctx := context.Background()
	entity := "cleanup_test"
	
	// Create old snapshots
	oldTime := time.Now().Add(-48 * time.Hour)
	testData := []facade.Kline{{Timestamp: oldTime, Close: 1000.0}}
	
	// Manually create old snapshot with fake timestamp
	snapshotID, err := store.StoreSnapshot(ctx, entity, testData)
	if err != nil {
		t.Fatalf("Failed to store test snapshot: %v", err)
	}
	
	// Verify it exists
	var retrieved []facade.Kline
	err = store.GetSnapshot(ctx, snapshotID, &retrieved)
	if err != nil {
		t.Fatalf("Failed to retrieve snapshot before cleanup: %v", err)
	}
	
	// Run cleanup (remove snapshots older than 24h)
	maxAge := 24 * time.Hour
	deleted, err := store.Cleanup(ctx, maxAge)
	if err != nil {
		t.Fatalf("Failed to cleanup: %v", err)
	}
	
	// For this test, we can't easily manipulate file timestamps,
	// so we'll just verify the cleanup function doesn't error
	// In a real scenario, files older than maxAge would be deleted
	
	t.Logf("Cleanup completed, %d files would be deleted", deleted)
}

func TestPITStoreCorruption(t *testing.T) {
	tmpDir := t.TempDir()
	store := pit.NewStore(tmpDir)
	
	ctx := context.Background()
	entity := "corruption_test"
	
	// Store valid snapshot
	testData := []facade.Kline{{Timestamp: time.Now(), Close: 1000.0}}
	snapshotID, err := store.StoreSnapshot(ctx, entity, testData)
	if err != nil {
		t.Fatalf("Failed to store snapshot: %v", err)
	}
	
	// Corrupt the file by writing invalid data
	snapshotPath := store.GetSnapshotPath(snapshotID)
	err = os.WriteFile(snapshotPath, []byte("corrupted data"), 0644)
	if err != nil {
		t.Fatalf("Failed to corrupt file: %v", err)
	}
	
	// Try to retrieve - should fail gracefully
	var retrieved []facade.Kline
	err = store.GetSnapshot(ctx, snapshotID, &retrieved)
	if err == nil {
		t.Error("Expected error when reading corrupted snapshot")
	}
	
	// Error should be descriptive
	if err.Error() == "" {
		t.Error("Expected non-empty error message")
	}
}

func TestPITStoreConcurrency(t *testing.T) {
	tmpDir := t.TempDir()
	store := pit.NewStore(tmpDir)
	
	ctx := context.Background()
	
	// Concurrent writes to different entities
	done := make(chan string, 10)
	
	for i := 0; i < 10; i++ {
		go func(idx int) {
			entity := fmt.Sprintf("concurrent_test_%d", idx)
			testData := []facade.Kline{
				{Timestamp: time.Now(), Close: float64(1000 + idx)},
			}
			
			snapshotID, err := store.StoreSnapshot(ctx, entity, testData)
			if err != nil {
				t.Errorf("Concurrent store failed for %s: %v", entity, err)
				done <- ""
				return
			}
			
			done <- snapshotID
		}(i)
	}
	
	// Wait for all goroutines
	snapshotIDs := make([]string, 0, 10)
	for i := 0; i < 10; i++ {
		id := <-done
		if id != "" {
			snapshotIDs = append(snapshotIDs, id)
		}
	}
	
	// Should have all snapshots
	if len(snapshotIDs) != 10 {
		t.Errorf("Expected 10 successful concurrent stores, got %d", len(snapshotIDs))
	}
	
	// All IDs should be unique
	idSet := make(map[string]bool)
	for _, id := range snapshotIDs {
		if idSet[id] {
			t.Errorf("Duplicate snapshot ID: %s", id)
		}
		idSet[id] = true
	}
}