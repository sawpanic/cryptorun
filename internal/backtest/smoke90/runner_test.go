package smoke90

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestRunnerCreation(t *testing.T) {
	config := &Config{
		TopN:     10,
		Stride:   4 * time.Hour,
		Hold:     24 * time.Hour,
		UseCache: true,
	}

	tempDir, err := os.MkdirTemp("", "smoke90_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	runner := NewRunner(config, tempDir)
	if runner == nil {
		t.Fatal("Runner should not be nil")
	}

	if runner.config.TopN != 10 {
		t.Errorf("Expected TopN=10, got %d", runner.config.TopN)
	}

	if runner.config.Stride != 4*time.Hour {
		t.Errorf("Expected Stride=4h, got %v", runner.config.Stride)
	}
}

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	if config.TopN <= 0 {
		t.Error("Default TopN should be positive")
	}

	if config.Stride <= 0 {
		t.Error("Default Stride should be positive")
	}

	if config.Hold <= 0 {
		t.Error("Default Hold should be positive")
	}

	if config.Horizon <= 0 {
		t.Error("Default Horizon should be positive")
	}
}

func TestRunnerWithMockClock(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "smoke90_clock_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	runner := NewRunner(nil, tempDir) // Use default config

	// Mock clock for deterministic testing
	mockTime := time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC)
	runner.SetClock(&MockClock{fixedTime: mockTime})

	// Quick test that runner works with mock clock
	if runner.clock.Now() != mockTime {
		t.Error("Mock clock should return fixed time")
	}
}

func TestSmoke90RunShortDuration(t *testing.T) {
	// Test with very short duration to avoid long test times
	config := &Config{
		TopN:     2,
		Stride:   1 * time.Hour,
		Hold:     1 * time.Hour,
		Horizon:  2 * time.Hour, // Very short for testing
		UseCache: true,
		Progress: false, // Disable progress output
	}

	tempDir, err := os.MkdirTemp("", "smoke90_short_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	runner := NewRunner(config, tempDir)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	results, err := runner.Run(ctx)
	if err != nil {
		t.Fatalf("Short run failed: %v", err)
	}

	if results == nil {
		t.Fatal("Results should not be nil")
	}

	if results.TotalWindows <= 0 {
		t.Error("Should have processed some windows")
	}

	// Check that artifacts are created
	writer := NewWriter(tempDir)
	artifacts := writer.GetArtifactPaths()

	if _, err := os.Stat(artifacts.ResultsJSONL); os.IsNotExist(err) {
		t.Error("Results JSONL should be created")
	}

	if _, err := os.Stat(artifacts.ReportMD); os.IsNotExist(err) {
		t.Error("Report MD should be created")
	}

	t.Logf("✅ Short run test passed: %d/%d windows processed",
		results.ProcessedWindows, results.TotalWindows)
}

// MockClock for deterministic testing
type MockClock struct {
	fixedTime time.Time
}

func (m *MockClock) Now() time.Time {
	return m.fixedTime
}
