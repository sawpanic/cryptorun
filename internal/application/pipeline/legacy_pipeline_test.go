package pipeline

import (
	"context"
	"testing"
)

// TestLegacyScanPipelineInterface verifies compile-time interface satisfaction
func TestLegacyScanPipelineInterface(t *testing.T) {
	// This test ensures the interface is satisfied at compile-time
	var _ ScanPipelineInterface = (*LegacyScanPipeline)(nil)

	// Create instance to verify constructor
	pipeline := NewLegacyScanPipeline("/tmp/snapshots")
	if pipeline == nil {
		t.Fatal("NewLegacyScanPipeline returned nil")
	}

	// Verify default regime
	if pipeline.regime != "trending_bull" {
		t.Errorf("Expected default regime 'trending_bull', got '%s'", pipeline.regime)
	}

	// Test regime setting
	pipeline.SetRegime("choppy")
	if pipeline.regime != "choppy" {
		t.Errorf("Expected regime 'choppy' after SetRegime, got '%s'", pipeline.regime)
	}
}

// TestScanUniverseNotSupported verifies ScanUniverse returns appropriate error
func TestScanUniverseNotSupported(t *testing.T) {
	pipeline := NewLegacyScanPipeline("/tmp/snapshots")

	ctx := context.Background()
	candidates, err := pipeline.ScanUniverse(ctx)

	// Should return empty slice and NotSupported error
	if len(candidates) != 0 {
		t.Errorf("Expected empty candidates slice, got %d candidates", len(candidates))
	}

	if err == nil {
		t.Error("Expected NotSupported error, got nil")
	}

	expectedErrMsg := "LegacyScanPipeline: ScanUniverse not supported, use composite pipeline"
	if err.Error() != expectedErrMsg {
		t.Errorf("Expected error message '%s', got '%s'", expectedErrMsg, err.Error())
	}
}

// TestWriteMethods verifies WriteJSONL and WriteLedger stubs
func TestWriteMethods(t *testing.T) {
	pipeline := NewLegacyScanPipeline("/tmp/snapshots")

	// Test WriteJSONL - should not error (stub implementation)
	candidates := []CandidateResult{}
	err := pipeline.WriteJSONL(candidates, "/tmp/output")
	if err != nil {
		t.Errorf("WriteJSONL should not error (stub), got: %v", err)
	}

	// Test WriteLedger - should not error (stub implementation)
	err = pipeline.WriteLedger(candidates)
	if err != nil {
		t.Errorf("WriteLedger should not error (stub), got: %v", err)
	}
}
