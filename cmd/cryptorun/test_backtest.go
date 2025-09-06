//go:build test
// +build test

package main

import (
	"context"
	"fmt"
	"testing"
	"time"

	"cryptorun/internal/backtest/smoke90"
)

// TestBacktestSmoke90Integration tests the smoke90 integration without requiring the full CLI
func TestBacktestSmoke90Integration(t *testing.T) {
	config := &smoke90.Config{
		TopN:     5, // Small number for testing
		Stride:   4 * time.Hour,
		Hold:     24 * time.Hour,
		UseCache: true,
	}

	runner := smoke90.NewRunner(config, "test-output")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// This should compile and run the core smoke90 logic
	results, err := runner.Run(ctx)
	if err != nil {
		t.Fatalf("Smoke90 backtest failed: %v", err)
	}

	if results == nil {
		t.Fatal("Results should not be nil")
	}

	// Basic validation
	if results.Config == nil {
		t.Error("Results config should not be nil")
	}

	if results.Metrics == nil {
		t.Error("Results metrics should not be nil")
	}

	fmt.Printf("✅ Integration test passed: %d windows processed\n", results.ProcessedWindows)
}
