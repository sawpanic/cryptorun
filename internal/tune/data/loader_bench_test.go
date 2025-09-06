package data

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestBenchDataLoader_LoadResults(t *testing.T) {
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
			expectResults: 5, // All results from tiny fixture
			expectError:   false,
		},
		{
			name:          "filter by normal regime",
			regimes:       []string{"normal"},
			windows:       []string{},
			expectResults: 5, // All are normal regime
			expectError:   false,
		},
		{
			name:          "filter by 24h window",
			regimes:       []string{},
			windows:       []string{"24h"},
			expectResults: 5, // All have 24h window
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
	testDir := setupBenchTestDataDirectory(t)
	defer os.RemoveAll(testDir)

	loader := NewBenchDataLoader(testDir)

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

func TestBenchDataLoader_ValidateShape(t *testing.T) {
	testDir := setupBenchTestDataDirectory(t)
	defer os.RemoveAll(testDir)

	loader := NewBenchDataLoader(testDir)
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
	if result.Rank <= 0 {
		t.Errorf("rank %d should be positive", result.Rank)
	}
	if result.BatchSize <= 0 {
		t.Errorf("batch size %d should be positive", result.BatchSize)
	}
}

func TestBenchDataLoader_CalculateBenchmarkHits(t *testing.T) {
	loader := NewBenchDataLoader("")

	// Create test batch with known ordering by actual gain
	results := []BenchResult{
		{Symbol: "HIGH", Timestamp: time.Now(), ActualGain: 0.08, BenchmarkHit: false}, // Should be hit
		{Symbol: "MED", Timestamp: time.Now(), ActualGain: 0.04, BenchmarkHit: false},  // Should NOT be hit
		{Symbol: "LOW", Timestamp: time.Now(), ActualGain: 0.01, BenchmarkHit: false},  // Should NOT be hit
		{Symbol: "NEG", Timestamp: time.Now(), ActualGain: -0.02, BenchmarkHit: false}, // Should NOT be hit
		{Symbol: "ZERO", Timestamp: time.Now(), ActualGain: 0.00, BenchmarkHit: false}, // Should NOT be hit
	}

	loader.calculateBenchmarkHits(results)

	// Top 20% (1 out of 5) should be marked as hits
	hitCount := 0
	for _, result := range results {
		if result.BenchmarkHit {
			hitCount++
		}
	}

	if hitCount != 1 {
		t.Errorf("expected 1 benchmark hit out of 5, got %d", hitCount)
	}

	// The highest gain should be the hit
	if !results[0].BenchmarkHit {
		t.Error("highest actual gain should be benchmark hit")
	}

	// Others should not be hits
	for i := 1; i < len(results); i++ {
		if results[i].BenchmarkHit {
			t.Errorf("result %d with gain %f should not be benchmark hit", i, results[i].ActualGain)
		}
	}
}

func TestBenchDataLoader_CalculateBenchMetrics(t *testing.T) {
	loader := NewBenchDataLoader("")

	// Create test data with known properties
	results := []BenchResult{
		{
			Symbol:       "BTC",
			Score:        90.0,
			Rank:         1,
			ActualGain:   0.05,
			BenchmarkHit: true,
			Regime:       "normal",
			BatchSize:    5,
		},
		{
			Symbol:       "ETH",
			Score:        80.0,
			Rank:         2,
			ActualGain:   0.03,
			BenchmarkHit: false,
			Regime:       "normal",
			BatchSize:    5,
		},
		{
			Symbol:       "ADA",
			Score:        70.0,
			Rank:         3,
			ActualGain:   0.02,
			BenchmarkHit: false,
			Regime:       "normal",
			BatchSize:    5,
		},
		{
			Symbol:       "DOT",
			Score:        60.0,
			Rank:         4,
			ActualGain:   0.01,
			BenchmarkHit: false,
			Regime:       "normal",
			BatchSize:    5,
		},
		{
			Symbol:       "SOL",
			Score:        50.0,
			Rank:         5,
			ActualGain:   -0.01,
			BenchmarkHit: false,
			Regime:       "normal",
			BatchSize:    5,
		},
	}

	metrics := loader.calculateBenchMetrics(results)

	// Verify basic metrics
	if metrics.TotalSymbols != 5 {
		t.Errorf("expected 5 total symbols, got %d", metrics.TotalSymbols)
	}

	if metrics.BenchmarkHits != 1 {
		t.Errorf("expected 1 benchmark hit, got %d", metrics.BenchmarkHits)
	}

	expectedHitRate := 0.2 // 1 out of 5
	if math.Abs(metrics.HitRate-expectedHitRate) > 0.001 {
		t.Errorf("expected hit rate %.3f, got %.3f", expectedHitRate, metrics.HitRate)
	}

	// Average rank should be 3.0 (1+2+3+4+5)/5
	expectedAvgRank := 3.0
	if math.Abs(metrics.AvgRank-expectedAvgRank) > 0.001 {
		t.Errorf("expected avg rank %.1f, got %.1f", expectedAvgRank, metrics.AvgRank)
	}

	// Average actual gain
	expectedAvgGain := (0.05 + 0.03 + 0.02 + 0.01 - 0.01) / 5.0
	if math.Abs(metrics.AvgActualGain-expectedAvgGain) > 0.001 {
		t.Errorf("expected avg gain %.3f, got %.3f", expectedAvgGain, metrics.AvgActualGain)
	}

	// Rank correlation should be positive (lower rank -> higher gain)
	if metrics.RankCorr <= 0 {
		t.Errorf("expected positive rank correlation, got %f", metrics.RankCorr)
	}

	// Precision@5 should be 1.0 (1 hit in top 5, and all 5 are in top 5)
	if math.Abs(metrics.PrecisionAt5-0.2) > 0.001 {
		t.Errorf("expected precision@5 0.2, got %f", metrics.PrecisionAt5)
	}
}

func TestBenchDataLoader_EmptyResults(t *testing.T) {
	loader := NewBenchDataLoader("")

	metrics := loader.calculateBenchMetrics([]BenchResult{})

	if metrics.TotalSymbols != 0 {
		t.Errorf("expected 0 symbols for empty input, got %d", metrics.TotalSymbols)
	}
	if metrics.HitRate != 0 {
		t.Errorf("expected 0 hit rate for empty input, got %f", metrics.HitRate)
	}
	if metrics.RankCorr != 0 {
		t.Errorf("expected 0 correlation for empty input, got %f", metrics.RankCorr)
	}
}

func TestBenchDataLoader_GetBenchMetricsByRegime(t *testing.T) {
	loader := NewBenchDataLoader("")

	results := []BenchResult{
		{Symbol: "BTC", Regime: "normal", ActualGain: 0.05, BenchmarkHit: true},
		{Symbol: "ETH", Regime: "normal", ActualGain: 0.02, BenchmarkHit: false},
		{Symbol: "ADA", Regime: "volatile", ActualGain: 0.08, BenchmarkHit: true},
	}

	metricsByRegime := loader.GetBenchMetricsByRegime(results)

	if len(metricsByRegime) != 2 {
		t.Errorf("expected 2 regimes, got %d", len(metricsByRegime))
	}

	// Check normal regime
	normalMetrics, exists := metricsByRegime["normal"]
	if !exists {
		t.Error("normal regime metrics missing")
	} else {
		if normalMetrics.TotalSymbols != 2 {
			t.Errorf("normal regime: expected 2 symbols, got %d", normalMetrics.TotalSymbols)
		}
		if normalMetrics.BenchmarkHits != 1 {
			t.Errorf("normal regime: expected 1 hit, got %d", normalMetrics.BenchmarkHits)
		}
	}

	// Check volatile regime
	volatileMetrics, exists := metricsByRegime["volatile"]
	if !exists {
		t.Error("volatile regime metrics missing")
	} else {
		if volatileMetrics.TotalSymbols != 1 {
			t.Errorf("volatile regime: expected 1 symbol, got %d", volatileMetrics.TotalSymbols)
		}
		if volatileMetrics.BenchmarkHits != 1 {
			t.Errorf("volatile regime: expected 1 hit, got %d", volatileMetrics.BenchmarkHits)
		}
	}
}

func TestCreateMockBenchResults(t *testing.T) {
	regimes := []string{"normal", "volatile"}
	windows := []string{"24h"}
	batchSize := 3
	batches := 2

	results := CreateMockBenchResults(regimes, windows, batchSize, batches)

	expectedCount := len(regimes) * len(windows) * batchSize * batches
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
		if result.Score < 60.0 || result.Score > 100.0 {
			t.Errorf("result %d has invalid score %f", i, result.Score)
		}
		if result.Rank <= 0 || result.Rank > batchSize {
			t.Errorf("result %d has invalid rank %d (batch size %d)", i, result.Rank, batchSize)
		}
		if result.BatchSize != batchSize {
			t.Errorf("result %d has wrong batch size %d, expected %d", i, result.BatchSize, batchSize)
		}
	}

	// Check that benchmarks hits are calculated (top 20% per batch)
	totalHits := 0
	for _, result := range results {
		if result.BenchmarkHit {
			totalHits++
		}
	}

	// Each batch should have at least 1 hit (20% of 3 = at least 1)
	expectedMinHits := batches * len(regimes) * len(windows) // 1 hit per batch
	if totalHits < expectedMinHits {
		t.Errorf("expected at least %d benchmark hits, got %d", expectedMinHits, totalHits)
	}

	// Verify deterministic behavior
	results2 := CreateMockBenchResults(regimes, windows, batchSize, batches)
	if len(results2) != len(results) {
		t.Error("mock results should be deterministic")
	}

	for i := range results {
		if results[i].Score != results2[i].Score {
			t.Errorf("mock results not deterministic at index %d: %f vs %f", i, results[i].Score, results2[i].Score)
		}
	}
}

func TestBenchDataLoader_MultipleBatches(t *testing.T) {
	loader := NewBenchDataLoader("")

	// Create two batches with different timestamps
	time1 := time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC)
	time2 := time.Date(2024, 1, 1, 11, 0, 0, 0, time.UTC)

	results := []BenchResult{
		// Batch 1
		{Symbol: "B1_HIGH", Timestamp: time1, ActualGain: 0.06, BenchmarkHit: false},
		{Symbol: "B1_LOW", Timestamp: time1, ActualGain: 0.01, BenchmarkHit: false},
		// Batch 2
		{Symbol: "B2_HIGH", Timestamp: time2, ActualGain: 0.08, BenchmarkHit: false},
		{Symbol: "B2_LOW", Timestamp: time2, ActualGain: 0.02, BenchmarkHit: false},
	}

	loader.calculateBenchmarkHits(results)

	// Each batch should have its own top 20% (1 out of 2 = 1 hit per batch)
	batch1Hits := 0
	batch2Hits := 0

	for _, result := range results {
		if result.BenchmarkHit {
			if result.Timestamp.Equal(time1) {
				batch1Hits++
			} else if result.Timestamp.Equal(time2) {
				batch2Hits++
			}
		}
	}

	if batch1Hits != 1 {
		t.Errorf("batch 1 should have 1 hit, got %d", batch1Hits)
	}
	if batch2Hits != 1 {
		t.Errorf("batch 2 should have 1 hit, got %d", batch2Hits)
	}

	// Verify the correct symbols are marked as hits
	for _, result := range results {
		if result.Symbol == "B1_HIGH" && !result.BenchmarkHit {
			t.Error("B1_HIGH should be benchmark hit")
		}
		if result.Symbol == "B1_LOW" && result.BenchmarkHit {
			t.Error("B1_LOW should not be benchmark hit")
		}
		if result.Symbol == "B2_HIGH" && !result.BenchmarkHit {
			t.Error("B2_HIGH should be benchmark hit")
		}
		if result.Symbol == "B2_LOW" && result.BenchmarkHit {
			t.Error("B2_LOW should not be benchmark hit")
		}
	}
}

// Helper function to setup bench test data directory
func setupBenchTestDataDirectory(t *testing.T) string {
	tempDir, err := os.MkdirTemp("", "bench_test_")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	// Create test data that matches our fixture
	testResults := []BenchResult{
		{
			Symbol:       "BTC",
			Timestamp:    time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
			Score:        89.5,
			Regime:       "normal",
			Rank:         1,
			BatchSize:    5,
			ActualGain:   0.0425,
			BenchmarkHit: true,
			Window:       "24h",
		},
		{
			Symbol:       "ETH",
			Timestamp:    time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
			Score:        76.2,
			Regime:       "normal",
			Rank:         2,
			BatchSize:    5,
			ActualGain:   0.0315,
			BenchmarkHit: true,
			Window:       "24h",
		},
		{
			Symbol:       "SOL",
			Timestamp:    time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
			Score:        71.8,
			Regime:       "normal",
			Rank:         3,
			BatchSize:    5,
			ActualGain:   0.0125,
			BenchmarkHit: false,
			Window:       "24h",
		},
		{
			Symbol:       "ADA",
			Timestamp:    time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
			Score:        65.3,
			Regime:       "normal",
			Rank:         4,
			BatchSize:    5,
			ActualGain:   0.008,
			BenchmarkHit: false,
			Window:       "24h",
		},
		{
			Symbol:       "DOT",
			Timestamp:    time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
			Score:        62.1,
			Regime:       "normal",
			Rank:         5,
			BatchSize:    5,
			ActualGain:   -0.005,
			BenchmarkHit: false,
			Window:       "24h",
		},
	}

	// Write test file
	testFile := filepath.Join(tempDir, "test_topgainers.json")
	data, err := json.Marshal(testResults)
	if err != nil {
		t.Fatalf("failed to marshal test data: %v", err)
	}

	if err := os.WriteFile(testFile, data, 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	return tempDir
}

// Golden test for bench loader regression detection
func TestBenchDataLoader_GoldenRegression(t *testing.T) {
	// Use the actual testdata fixtures for golden testing
	loader := NewBenchDataLoader("../../../testdata/tuner")
	results, err := loader.LoadResults([]string{"normal"}, []string{"24h"})

	if err != nil {
		t.Fatalf("failed to load golden test data: %v", err)
	}

	if len(results) == 0 {
		t.Skip("No golden bench test data found, skipping regression test")
	}

	metrics := loader.calculateBenchMetrics(results)

	// These values should remain stable across refactoring
	golden := map[string]interface{}{
		"total_symbols": 5,   // Normal regime results in fixture
		"hit_rate":      0.4, // 2 out of 5 hits (top 20%)
		"avg_rank":      3.0, // (1+2+3+4+5)/5
		"regime":        "normal",
	}

	// Validate golden values
	if metrics.TotalSymbols != golden["total_symbols"].(int) {
		t.Errorf("golden regression: expected total_symbols %d, got %d", golden["total_symbols"].(int), metrics.TotalSymbols)
	}

	expectedHitRate := golden["hit_rate"].(float64)
	if math.Abs(metrics.HitRate-expectedHitRate) > 0.001 {
		t.Errorf("golden regression: expected hit_rate %.3f, got %.3f", expectedHitRate, metrics.HitRate)
	}

	expectedAvgRank := golden["avg_rank"].(float64)
	if math.Abs(metrics.AvgRank-expectedAvgRank) > 0.001 {
		t.Errorf("golden regression: expected avg_rank %.1f, got %.1f", expectedAvgRank, metrics.AvgRank)
	}

	t.Logf("Golden bench test passed: symbols=%d, hit_rate=%.3f, avg_rank=%.1f, rank_corr=%.3f",
		metrics.TotalSymbols, metrics.HitRate, metrics.AvgRank, metrics.RankCorr)
}
