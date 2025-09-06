package data

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSmokeDataLoader_LoadResults(t *testing.T) {
	tests := []struct {
		name          string
		regimes       []string
		windows       []string
		expectResults int
		expectError   bool
	}{
		{
			name:          "load all regimes and windows",
			regimes:       []string{},
			windows:       []string{},
			expectResults: 4, // All results from tiny fixture
			expectError:   false,
		},
		{
			name:          "filter by normal regime",
			regimes:       []string{"normal"},
			windows:       []string{},
			expectResults: 2, // BTC and ETH
			expectError:   false,
		},
		{
			name:          "filter by volatile regime",
			regimes:       []string{"volatile"},
			windows:       []string{},
			expectResults: 1, // SOL
			expectError:   false,
		},
		{
			name:          "filter by 4h window",
			regimes:       []string{},
			windows:       []string{"4h"},
			expectResults: 4, // All have 4h window
			expectError:   false,
		},
		{
			name:          "filter by nonexistent regime",
			regimes:       []string{"nonexistent"},
			windows:       []string{},
			expectResults: 0,
			expectError:   false,
		},
	}

	// Setup test data directory
	testDir := setupTestDataDirectory(t)
	defer os.RemoveAll(testDir)

	loader := NewSmokeDataLoader(testDir)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results, err := loader.LoadResults(tt.regimes, tt.windows)

			if tt.expectError && err == nil {
				t.Errorf("expected error, got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			if len(results) != tt.expectResults {
				t.Errorf("expected %d results, got %d", tt.expectResults, len(results))
			}

			// Verify sorting by timestamp
			for i := 1; i < len(results); i++ {
				if results[i-1].Timestamp.After(results[i].Timestamp) {
					t.Errorf("results not sorted by timestamp: %v > %v", results[i-1].Timestamp, results[i].Timestamp)
				}
			}
		})
	}
}

func TestSmokeDataLoader_ValidateShape(t *testing.T) {
	testDir := setupTestDataDirectory(t)
	defer os.RemoveAll(testDir)

	loader := NewSmokeDataLoader(testDir)
	results, err := loader.LoadResults([]string{}, []string{})

	if err != nil {
		t.Fatalf("failed to load results: %v", err)
	}

	if len(results) == 0 {
		t.Fatal("no results loaded")
	}

	// Validate structure of first result
	result := results[0]

	if result.Symbol == "" {
		t.Error("symbol is empty")
	}
	if result.Timestamp.IsZero() {
		t.Error("timestamp is zero")
	}
	if result.Score < 0 || result.Score > 100 {
		t.Errorf("score %f out of valid range [0, 100]", result.Score)
	}
	if result.Regime == "" {
		t.Error("regime is empty")
	}
	if result.Window == "" {
		t.Error("window is empty")
	}
	if result.EntryPrice <= 0 {
		t.Errorf("entry price %f should be positive", result.EntryPrice)
	}
}

func TestSmokeDataLoader_CalculateHit(t *testing.T) {
	loader := NewSmokeDataLoader("")

	tests := []struct {
		name     string
		result   SmokeResult
		expected bool
	}{
		{
			name: "4h window hit",
			result: SmokeResult{
				Window:        "4h",
				ForwardReturn: 0.030, // Above 2.5% threshold
			},
			expected: true,
		},
		{
			name: "4h window miss",
			result: SmokeResult{
				Window:        "4h",
				ForwardReturn: 0.020, // Below 2.5% threshold
			},
			expected: false,
		},
		{
			name: "1h window hit",
			result: SmokeResult{
				Window:        "1h",
				ForwardReturn: 0.018, // Above 1.5% threshold
			},
			expected: true,
		},
		{
			name: "unknown window defaults to 2.5%",
			result: SmokeResult{
				Window:        "unknown",
				ForwardReturn: 0.030, // Above default 2.5%
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hit := loader.CalculateHitPublic(&tt.result)
			if hit != tt.expected {
				t.Errorf("expected %v, got %v for return %f in window %s", tt.expected, hit, tt.result.ForwardReturn, tt.result.Window)
			}
		})
	}
}

func TestSmokeDataLoader_CalculateMetrics(t *testing.T) {
	loader := NewSmokeDataLoader("")

	// Create test data with known properties
	results := []SmokeResult{
		{
			Symbol:        "BTC",
			Score:         85.0,
			ForwardReturn: 0.030,
			Window:        "4h",
			Regime:        "normal",
		},
		{
			Symbol:        "ETH",
			Score:         75.0,
			ForwardReturn: 0.020, // Below threshold
			Window:        "4h",
			Regime:        "normal",
		},
		{
			Symbol:        "SOL",
			Score:         90.0,
			ForwardReturn: 0.035,
			Window:        "4h",
			Regime:        "normal",
		},
	}

	// Update hit status based on threshold
	for i := range results {
		results[i].Hit = loader.CalculateHitPublic(&results[i])
	}

	metrics := loader.CalculateMetricsPublic(results)

	// Verify basic metrics
	if metrics.TotalSignals != 3 {
		t.Errorf("expected 3 total signals, got %d", metrics.TotalSignals)
	}

	expectedHits := 2 // BTC and SOL should hit
	if metrics.Hits != expectedHits {
		t.Errorf("expected %d hits, got %d", expectedHits, metrics.Hits)
	}

	expectedHitRate := float64(expectedHits) / float64(len(results))
	if math.Abs(metrics.HitRate-expectedHitRate) > 0.001 {
		t.Errorf("expected hit rate %.3f, got %.3f", expectedHitRate, metrics.HitRate)
	}

	// Verify score bounds
	if metrics.ScoreBounds[0] != 75.0 || metrics.ScoreBounds[1] != 90.0 {
		t.Errorf("expected score bounds [75.0, 90.0], got [%f, %f]", metrics.ScoreBounds[0], metrics.ScoreBounds[1])
	}

	// Spearman correlation should be positive (higher score -> higher return)
	if metrics.SpearmanCorr < 0 {
		t.Errorf("expected positive Spearman correlation, got %f", metrics.SpearmanCorr)
	}
}

func TestSmokeDataLoader_EmptyResults(t *testing.T) {
	loader := NewSmokeDataLoader("")

	metrics := loader.CalculateMetricsPublic([]SmokeResult{})

	if metrics.TotalSignals != 0 {
		t.Errorf("expected 0 signals for empty input, got %d", metrics.TotalSignals)
	}
	if metrics.HitRate != 0 {
		t.Errorf("expected 0 hit rate for empty input, got %f", metrics.HitRate)
	}
	if metrics.SpearmanCorr != 0 {
		t.Errorf("expected 0 correlation for empty input, got %f", metrics.SpearmanCorr)
	}
}

func TestSmokeDataLoader_NaNHandling(t *testing.T) {
	loader := NewSmokeDataLoader("")

	// Test with NaN values
	results := []SmokeResult{
		{
			Symbol:        "NAN_TEST",
			Score:         math.NaN(),
			ForwardReturn: math.NaN(),
			Window:        "4h",
			Regime:        "normal",
		},
		{
			Symbol:        "NORMAL",
			Score:         80.0,
			ForwardReturn: 0.025,
			Window:        "4h",
			Regime:        "normal",
		},
	}

	// The function should handle NaN gracefully without panicking
	metrics := loader.CalculateMetricsPublic(results)

	// Verify it doesn't panic and produces reasonable results
	if metrics.TotalSignals != 2 {
		t.Errorf("expected 2 signals, got %d", metrics.TotalSignals)
	}

	// NaN values should result in some defined correlation (0, 1, or NaN)
	// The important thing is that it doesn't panic or return infinite values
	if math.IsInf(metrics.SpearmanCorr, 0) {
		t.Errorf("correlation should not be infinite with NaN input, got %f", metrics.SpearmanCorr)
	}

	// Log the actual result for debugging
	t.Logf("Correlation with NaN input: %f", metrics.SpearmanCorr)
}

func TestSmokeDataLoader_SingleResult(t *testing.T) {
	loader := NewSmokeDataLoader("")

	result := SmokeResult{
		Symbol:        "SINGLE",
		Score:         75.0,
		ForwardReturn: 0.030,
		Window:        "4h",
		Regime:        "normal",
	}
	result.Hit = loader.CalculateHitPublic(&result)

	metrics := loader.CalculateMetricsPublic([]SmokeResult{result})

	if metrics.TotalSignals != 1 {
		t.Errorf("expected 1 signal, got %d", metrics.TotalSignals)
	}
	if metrics.Hits != 1 {
		t.Errorf("expected 1 hit, got %d", metrics.Hits)
	}
	if metrics.HitRate != 1.0 {
		t.Errorf("expected hit rate 1.0, got %f", metrics.HitRate)
	}
	// Single point should have zero correlation
	if metrics.SpearmanCorr != 0 {
		t.Errorf("expected zero correlation for single point, got %f", metrics.SpearmanCorr)
	}
}

func TestCreateMockResults(t *testing.T) {
	regimes := []string{"normal", "volatile"}
	windows := []string{"4h", "24h"}
	count := 3

	results := CreateMockResults(regimes, windows, count)

	expectedCount := len(regimes) * len(windows) * count
	if len(results) != expectedCount {
		t.Errorf("expected %d results, got %d", expectedCount, len(results))
	}

	// Verify structure
	for i, result := range results {
		if result.Symbol == "" {
			t.Errorf("result %d has empty symbol", i)
		}
		if result.Regime == "" {
			t.Errorf("result %d has empty regime", i)
		}
		if result.Window == "" {
			t.Errorf("result %d has empty window", i)
		}
		if result.Score < 70.0 || result.Score > 100.0 {
			t.Errorf("result %d has invalid score %f", i, result.Score)
		}
		if result.EntryPrice <= 0 {
			t.Errorf("result %d has invalid entry price %f", i, result.EntryPrice)
		}
	}

	// Verify deterministic behavior
	results2 := CreateMockResults(regimes, windows, count)
	if len(results2) != len(results) {
		t.Error("mock results should be deterministic")
	}

	for i := range results {
		if results[i].Score != results2[i].Score {
			t.Errorf("mock results not deterministic at index %d: %f vs %f", i, results[i].Score, results2[i].Score)
		}
	}
}

// Helper function to setup test data directory
func setupTestDataDirectory(t *testing.T) string {
	tempDir, err := os.MkdirTemp("", "smoke_test_")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	// Create test data that matches our fixture
	testResults := []SmokeResult{
		{
			Symbol:        "BTC",
			Timestamp:     time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC),
			Score:         85.5,
			Regime:        "normal",
			ForwardReturn: 0.0325,
			Hit:           true,
			Window:        "4h",
			EntryPrice:    50000,
			ExitPrice:     51625,
		},
		{
			Symbol:        "ETH",
			Timestamp:     time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC),
			Score:         72.3,
			Regime:        "normal",
			ForwardReturn: 0.018,
			Hit:           false,
			Window:        "4h",
			EntryPrice:    3000,
			ExitPrice:     3054,
		},
		{
			Symbol:        "SOL",
			Timestamp:     time.Date(2024, 1, 1, 11, 0, 0, 0, time.UTC),
			Score:         91.2,
			Regime:        "volatile",
			ForwardReturn: 0.045,
			Hit:           true,
			Window:        "4h",
			EntryPrice:    100,
			ExitPrice:     104.5,
		},
		{
			Symbol:        "ADA",
			Timestamp:     time.Date(2024, 1, 1, 11, 0, 0, 0, time.UTC),
			Score:         68.1,
			Regime:        "calm",
			ForwardReturn: 0.012,
			Hit:           false,
			Window:        "4h",
			EntryPrice:    0.5,
			ExitPrice:     0.506,
		},
	}

	// Write test file
	testFile := filepath.Join(tempDir, "test_smoke90.json")
	data, err := json.Marshal(testResults)
	if err != nil {
		t.Fatalf("failed to marshal test data: %v", err)
	}

	if err := os.WriteFile(testFile, data, 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	return tempDir
}

// Golden test for regression detection
func TestSmokeDataLoader_GoldenRegression(t *testing.T) {
	// Use the actual testdata fixtures for golden testing
	loader := NewSmokeDataLoader("../../../testdata/tuner")
	results, err := loader.LoadResults([]string{"normal"}, []string{"4h"})

	if err != nil {
		t.Fatalf("failed to load golden test data: %v", err)
	}

	if len(results) == 0 {
		t.Skip("No golden test data found, skipping regression test")
	}

	metrics := loader.CalculateMetricsPublic(results)

	// These values should remain stable across refactoring
	golden := map[string]interface{}{
		"total_signals": 2,    // Normal regime results in fixture
		"hit_rate":      0.5,  // 1 out of 2 hits (BTC hits, ETH misses)
		"min_score":     72.3, // ETH score
		"max_score":     85.5, // BTC score
		"regime":        "normal",
	}

	// Validate golden values
	if metrics.TotalSignals != golden["total_signals"].(int) {
		t.Errorf("golden regression: expected total_signals %d, got %d", golden["total_signals"].(int), metrics.TotalSignals)
	}

	expectedHitRate := golden["hit_rate"].(float64)
	if math.Abs(metrics.HitRate-expectedHitRate) > 0.001 {
		t.Errorf("golden regression: expected hit_rate %.3f, got %.3f", expectedHitRate, metrics.HitRate)
	}

	if math.Abs(metrics.ScoreBounds[0]-golden["min_score"].(float64)) > 0.001 {
		t.Errorf("golden regression: expected min_score %.1f, got %.1f", golden["min_score"].(float64), metrics.ScoreBounds[0])
	}

	if math.Abs(metrics.ScoreBounds[1]-golden["max_score"].(float64)) > 0.001 {
		t.Errorf("golden regression: expected max_score %.1f, got %.1f", golden["max_score"].(float64), metrics.ScoreBounds[1])
	}

	t.Logf("Golden test passed: signals=%d, hit_rate=%.3f, spearman=%.3f",
		metrics.TotalSignals, metrics.HitRate, metrics.SpearmanCorr)
}
