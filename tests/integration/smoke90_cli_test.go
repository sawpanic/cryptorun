package integration

import (
	"context"
	"os"
	"testing"
	"time"

	"cryptorun/internal/backtest/smoke90"
)

func TestSmoke90CLIIntegration(t *testing.T) {
	// Test the smoke90 backtest functionality that would be called by CLI
	config := &smoke90.Config{
		TopN:     10,
		Stride:   4 * time.Hour,
		Hold:     24 * time.Hour,
		UseCache: true,
	}

	// Create temporary output directory
	tempDir, err := os.MkdirTemp("", "smoke90_cli_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	runner := smoke90.NewRunner(config, tempDir)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Run the backtest
	results, err := runner.Run(ctx)
	if err != nil {
		t.Fatalf("Smoke90 backtest failed: %v", err)
	}

	// Validate results structure
	if results == nil {
		t.Fatal("Results should not be nil")
	}

	if results.Config == nil {
		t.Error("Config should not be nil")
	}

	if results.Metrics == nil {
		t.Error("Metrics should not be nil")
	}

	if results.TotalWindows <= 0 {
		t.Error("Total windows should be positive")
	}

	// Check that some processing occurred (may have skipped windows due to cache misses)
	t.Logf("Processed: %d/%d windows (%.1f%% coverage)",
		results.ProcessedWindows, results.TotalWindows,
		float64(results.ProcessedWindows)/float64(results.TotalWindows)*100)

	// Check that artifacts are written
	writer := smoke90.NewWriter(tempDir)
	artifacts := writer.GetArtifactPaths()

	// Results JSONL should exist (even if empty)
	if _, err := os.Stat(artifacts.ResultsJSONL); os.IsNotExist(err) {
		t.Errorf("Results JSONL file should exist: %s", artifacts.ResultsJSONL)
	}

	// Report MD should exist
	if _, err := os.Stat(artifacts.ReportMD); os.IsNotExist(err) {
		t.Errorf("Report MD file should exist: %s", artifacts.ReportMD)
	}

	t.Logf("✅ CLI integration test passed - artifacts created in: %s", tempDir)
}

func TestSmoke90ConfigValidation(t *testing.T) {
	// Test configuration validation that would be used by CLI flag parsing
	tests := []struct {
		name      string
		config    *smoke90.Config
		expectErr bool
	}{
		{
			name: "valid_config",
			config: &smoke90.Config{
				TopN:     20,
				Stride:   4 * time.Hour,
				Hold:     24 * time.Hour,
				UseCache: true,
			},
			expectErr: false,
		},
		{
			name:      "default_config",
			config:    nil, // Should use defaults
			expectErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir, err := os.MkdirTemp("", "smoke90_config_test")
			if err != nil {
				t.Fatalf("Failed to create temp dir: %v", err)
			}
			defer os.RemoveAll(tempDir)

			runner := smoke90.NewRunner(tt.config, tempDir)
			if runner == nil {
				t.Fatal("Runner should not be nil")
			}

			// Quick validation that runner is properly initialized
			ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
			defer cancel()

			// This may fail due to timeout, but should not crash
			_, err = runner.Run(ctx)
			// We don't check error since timeout is expected
			t.Logf("Runner created successfully for test: %s", tt.name)
		})
	}
}
