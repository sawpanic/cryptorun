//go:build legacy
// +build legacy

package unit

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"

	"cryptorun/infrastructure/market"
)

func TestSnapshotJSONSchema(t *testing.T) {
	snapshot := market.NewSnapshot(
		"BTCUSD",
		49995.1234,    // bid
		50004.5678,    // ask  
		18.755,        // spread_bps
		125000.789,    // depth2pc_usd
		2.456789,      // vadr
		5000000,       // adv_usd
	)

	// Test JSON marshaling
	data, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		t.Fatalf("Failed to marshal snapshot: %v", err)
	}

	// Test JSON unmarshaling
	var unmarshaled market.Snapshot
	if err := json.Unmarshal(data, &unmarshaled); err != nil {
		t.Fatalf("Failed to unmarshal snapshot: %v", err)
	}

	// Verify required fields are present
	expectedFields := []string{"symbol", "ts", "bid", "ask", "spread_bps", "depth2pc_usd", "vadr", "adv_usd"}
	var jsonMap map[string]interface{}
	if err := json.Unmarshal(data, &jsonMap); err != nil {
		t.Fatalf("Failed to unmarshal to map: %v", err)
	}

	for _, field := range expectedFields {
		if _, exists := jsonMap[field]; !exists {
			t.Errorf("Required field '%s' missing from JSON schema", field)
		}
	}

	// Verify data types and values
	if unmarshaled.Symbol != "BTCUSD" {
		t.Errorf("Symbol mismatch: got %s, want BTCUSD", unmarshaled.Symbol)
	}

	if unmarshaled.Bid != 49995.1234 {
		t.Errorf("Bid mismatch: got %f, want 49995.1234", unmarshaled.Bid)
	}

	if unmarshaled.Ask != 50004.5678 {
		t.Errorf("Ask mismatch: got %f, want 50004.5678", unmarshaled.Ask)
	}

	if unmarshaled.ADVUSD != 5000000 {
		t.Errorf("ADVUSD mismatch: got %d, want 5000000", unmarshaled.ADVUSD)
	}

	// Verify timestamp is present and reasonable
	if unmarshaled.Timestamp.IsZero() {
		t.Error("Timestamp should not be zero")
	}

	// Verify timestamp is recent (within last minute)
	if time.Since(unmarshaled.Timestamp) > time.Minute {
		t.Error("Timestamp should be recent")
	}
}

func TestSnapshotRounding(t *testing.T) {
	testCases := []struct {
		name             string
		bid              float64
		ask              float64
		spreadBps        float64
		depth2PcUSD      float64
		vadr             float64
		expectedBid      float64
		expectedAsk      float64
		expectedSpread   float64
		expectedDepth    float64
		expectedVADR     float64
	}{
		{
			name:           "Standard rounding",
			bid:            49999.12345,
			ask:            50000.98765,
			spreadBps:      18.7567,
			depth2PcUSD:    125500.789,
			vadr:           2.4567,
			expectedBid:    49999.1235,
			expectedAsk:    50000.9877,
			expectedSpread: 18.76,
			expectedDepth:  125501,
			expectedVADR:   2.457,
		},
		{
			name:           "Edge case rounding - 0.5",
			bid:            100.00005,
			ask:            100.10005,
			spreadBps:      25.505,
			depth2PcUSD:    100000.5,
			vadr:           1.7505,
			expectedBid:    100.0001,
			expectedAsk:    100.1001,
			expectedSpread: 25.51,
			expectedDepth:  100001,
			expectedVADR:   1.751,
		},
		{
			name:           "Very small values",
			bid:            0.00012345,
			ask:            0.00012678,
			spreadBps:      0.0056,
			depth2PcUSD:    50.789,
			vadr:           0.0012,
			expectedBid:    0.0001,
			expectedAsk:    0.0001,
			expectedSpread: 0.01,
			expectedDepth:  51,
			expectedVADR:   0.001,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			snapshot := market.NewSnapshot(
				"TESTUSD",
				tc.bid,
				tc.ask,
				tc.spreadBps,
				tc.depth2PcUSD,
				tc.vadr,
				1000000,
			)

			if snapshot.Bid != tc.expectedBid {
				t.Errorf("Bid rounding: got %f, want %f", snapshot.Bid, tc.expectedBid)
			}

			if snapshot.Ask != tc.expectedAsk {
				t.Errorf("Ask rounding: got %f, want %f", snapshot.Ask, tc.expectedAsk)
			}

			if snapshot.SpreadBps != tc.expectedSpread {
				t.Errorf("SpreadBps rounding: got %f, want %f", snapshot.SpreadBps, tc.expectedSpread)
			}

			if snapshot.Depth2PcUSD != tc.expectedDepth {
				t.Errorf("Depth2PcUSD rounding: got %f, want %f", snapshot.Depth2PcUSD, tc.expectedDepth)
			}

			if snapshot.VADR != tc.expectedVADR {
				t.Errorf("VADR rounding: got %f, want %f", snapshot.VADR, tc.expectedVADR)
			}
		})
	}
}

func TestSnapshotSaveLoad(t *testing.T) {
	// Create temporary directory
	tmpDir := filepath.Join(os.TempDir(), "test_snapshots")
	defer os.RemoveAll(tmpDir)

	writer := market.NewSnapshotWriter(tmpDir)

	// Create test snapshot
	originalSnapshot := market.NewSnapshot(
		"ETHUSD",
		3000.1234,
		3005.5678,
		18.456,
		200000.0,
		2.125,
		8000000,
	)

	// Save snapshot
	if err := writer.SaveSnapshot(originalSnapshot); err != nil {
		t.Fatalf("Failed to save snapshot: %v", err)
	}

	// List snapshots to find the file
	files, err := market.ListSnapshots(tmpDir, "ETHUSD")
	if err != nil {
		t.Fatalf("Failed to list snapshots: %v", err)
	}

	if len(files) != 1 {
		t.Fatalf("Expected 1 snapshot file, got %d", len(files))
	}

	// Load snapshot back
	loadedSnapshot, err := market.LoadSnapshot(files[0])
	if err != nil {
		t.Fatalf("Failed to load snapshot: %v", err)
	}

	// Verify all fields match
	if loadedSnapshot.Symbol != originalSnapshot.Symbol {
		t.Errorf("Symbol mismatch: got %s, want %s", loadedSnapshot.Symbol, originalSnapshot.Symbol)
	}

	if loadedSnapshot.Bid != originalSnapshot.Bid {
		t.Errorf("Bid mismatch: got %f, want %f", loadedSnapshot.Bid, originalSnapshot.Bid)
	}

	if loadedSnapshot.Ask != originalSnapshot.Ask {
		t.Errorf("Ask mismatch: got %f, want %f", loadedSnapshot.Ask, originalSnapshot.Ask)
	}

	if loadedSnapshot.ADVUSD != originalSnapshot.ADVUSD {
		t.Errorf("ADVUSD mismatch: got %d, want %d", loadedSnapshot.ADVUSD, originalSnapshot.ADVUSD)
	}

	// Verify timestamp precision (should be within 1 second)
	timeDiff := loadedSnapshot.Timestamp.Sub(originalSnapshot.Timestamp).Abs()
	if timeDiff > time.Second {
		t.Errorf("Timestamp precision loss: diff %v", timeDiff)
	}
}

func TestSnapshotFileNaming(t *testing.T) {
	tmpDir := filepath.Join(os.TempDir(), "test_snapshots_naming")
	defer os.RemoveAll(tmpDir)

	writer := market.NewSnapshotWriter(tmpDir)

	// Create snapshot with known timestamp
	snapshot := market.Snapshot{
		Symbol:      "BTCUSD",
		Timestamp:   time.Unix(1609459200, 0), // 2021-01-01 00:00:00 UTC
		Bid:         50000.0,
		Ask:         50050.0,
		SpreadBps:   100.0,
		Depth2PcUSD: 150000.0,
		VADR:        2.0,
		ADVUSD:      5000000,
	}

	if err := writer.SaveSnapshot(snapshot); err != nil {
		t.Fatalf("Failed to save snapshot: %v", err)
	}

	// Check file was created with expected name format
	expectedFile := filepath.Join(tmpDir, "BTCUSD-1609459200.json")
	if _, err := os.Stat(expectedFile); os.IsNotExist(err) {
		t.Errorf("Expected file %s does not exist", expectedFile)
	}
}

func TestSnapshotAtomicWrite(t *testing.T) {
	tmpDir := filepath.Join(os.TempDir(), "test_snapshots_atomic")
	defer os.RemoveAll(tmpDir)

	writer := market.NewSnapshotWriter(tmpDir)
	snapshot := market.NewSnapshot("TESTUSD", 100.0, 101.0, 100.0, 50000.0, 1.5, 100000)

	// Save snapshot
	if err := writer.SaveSnapshot(snapshot); err != nil {
		t.Fatalf("Failed to save snapshot: %v", err)
	}

	// Verify no .tmp file remains
	files, err := filepath.Glob(filepath.Join(tmpDir, "*.tmp"))
	if err != nil {
		t.Fatalf("Failed to glob tmp files: %v", err)
	}

	if len(files) > 0 {
		t.Errorf("Found %d temporary files, expected 0: %v", len(files), files)
	}
}

func TestSnapshotInvalidValues(t *testing.T) {
	// Test that snapshot creation handles invalid values gracefully
	snapshot := market.NewSnapshot(
		"INVALIDUSD",
		math.NaN(),
		math.Inf(1),
		-10.0,
		math.Inf(-1),
		math.NaN(),
		-1000,
	)

	// Should not panic and should create valid JSON
	data, err := json.Marshal(snapshot)
	if err != nil {
		t.Errorf("Failed to marshal snapshot with invalid values: %v", err)
	}

	// Should be able to unmarshal back
	var unmarshaled market.Snapshot
	if err := json.Unmarshal(data, &unmarshaled); err != nil {
		t.Errorf("Failed to unmarshal snapshot with invalid values: %v", err)
	}
}

func TestMultipleSnapshots(t *testing.T) {
	tmpDir := filepath.Join(os.TempDir(), "test_snapshots_multiple")
	defer os.RemoveAll(tmpDir)

	writer := market.NewSnapshotWriter(tmpDir)

	symbols := []string{"BTCUSD", "ETHUSD", "ADAUSD"}
	
	// Create multiple snapshots
	for i, symbol := range symbols {
		snapshot := market.NewSnapshot(
			symbol,
			float64(1000+i*100),
			float64(1005+i*100),
			50.0,
			100000.0,
			2.0,
			1000000,
		)

		if err := writer.SaveSnapshot(snapshot); err != nil {
			t.Fatalf("Failed to save snapshot for %s: %v", symbol, err)
		}
	}

	// Verify all snapshots were created
	for _, symbol := range symbols {
		files, err := market.ListSnapshots(tmpDir, symbol)
		if err != nil {
			t.Fatalf("Failed to list snapshots for %s: %v", symbol, err)
		}

		if len(files) != 1 {
			t.Errorf("Expected 1 snapshot for %s, got %d", symbol, len(files))
		}
	}
}