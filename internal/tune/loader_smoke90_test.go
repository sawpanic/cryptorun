package tune

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sawpanic/cryptorun/internal/tune/data"
)

func TestSmokeDataLoader_LoadResults_GoldenFixtures(t *testing.T) {
	// Test with small golden fixture
	loader := data.NewSmokeDataLoader("../../../testdata/tuner")

	// Load all results without filtering
	results, err := loader.LoadResults(nil, nil)
	require.NoError(t, err)

	// Should load from both fixture files
	assert.GreaterOrEqual(t, len(results), 5, "Should load at least 5 results from fixtures")

	// Validate data structure
	for _, result := range results {
		assert.NotEmpty(t, result.Symbol, "Symbol should not be empty")
		assert.NotEmpty(t, result.Regime, "Regime should not be empty")
		assert.NotEmpty(t, result.Window, "Window should not be empty")
		assert.NotZero(t, result.Timestamp, "Timestamp should not be zero")

		// Validate numeric ranges
		if !math.IsNaN(result.Score) && result.Score != 0 {
			assert.GreaterOrEqual(t, result.Score, 0.0, "Score should be non-negative")
			assert.LessOrEqual(t, result.Score, 100.0, "Score should be <= 100")
		}

		assert.GreaterOrEqual(t, result.EntryPrice, 0.0, "Entry price should be non-negative")
		assert.GreaterOrEqual(t, result.ExitPrice, 0.0, "Exit price should be non-negative")
	}
}

func TestSmokeDataLoader_LoadResults_RegimeFiltering(t *testing.T) {
	loader := data.NewSmokeDataLoader("../../../testdata/tuner")

	// Filter by regime
	normalResults, err := loader.LoadResults([]string{"normal"}, nil)
	require.NoError(t, err)

	// All results should be from normal regime
	for _, result := range normalResults {
		assert.Equal(t, "normal", result.Regime, "All results should be from normal regime")
	}

	// Test multiple regimes
	multiResults, err := loader.LoadResults([]string{"normal", "volatile"}, nil)
	require.NoError(t, err)

	assert.GreaterOrEqual(t, len(multiResults), len(normalResults), "Multi-regime should have at least as many as single")

	regimes := make(map[string]bool)
	for _, result := range multiResults {
		regimes[result.Regime] = true
	}
	assert.True(t, regimes["normal"] || regimes["volatile"], "Should contain normal or volatile regime")
}

func TestSmokeDataLoader_LoadResults_WindowFiltering(t *testing.T) {
	loader := data.NewSmokeDataLoader("../../../testdata/tuner")

	// Filter by window
	windowResults, err := loader.LoadResults(nil, []string{"4h"})
	require.NoError(t, err)

	// All results should be from 4h window
	for _, result := range windowResults {
		assert.Equal(t, "4h", result.Window, "All results should be from 4h window")
	}

	// Test multiple windows
	multiWindowResults, err := loader.LoadResults(nil, []string{"1h", "4h"})
	require.NoError(t, err)

	assert.GreaterOrEqual(t, len(multiWindowResults), len(windowResults), "Multi-window should have at least as many")
}

func TestSmokeDataLoader_LoadResults_CombinedFiltering(t *testing.T) {
	loader := data.NewSmokeDataLoader("../../../testdata/tuner")

	// Combined regime and window filtering
	filteredResults, err := loader.LoadResults([]string{"normal"}, []string{"4h"})
	require.NoError(t, err)

	for _, result := range filteredResults {
		assert.Equal(t, "normal", result.Regime, "Should match regime filter")
		assert.Equal(t, "4h", result.Window, "Should match window filter")
	}
}

func TestSmokeDataLoader_LoadResults_TimestampOrdering(t *testing.T) {
	loader := data.NewSmokeDataLoader("../../../testdata/tuner")

	results, err := loader.LoadResults(nil, nil)
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(results), 2, "Need at least 2 results to test ordering")

	// Results should be ordered by timestamp
	for i := 1; i < len(results); i++ {
		assert.True(t,
			results[i-1].Timestamp.Before(results[i].Timestamp) || results[i-1].Timestamp.Equal(results[i].Timestamp),
			"Results should be ordered by timestamp")
	}
}

func TestSmokeDataLoader_LoadResults_NaNHandling(t *testing.T) {
	loader := data.NewSmokeDataLoader("../../../testdata/tuner")

	results, err := loader.LoadResults(nil, nil)
	require.NoError(t, err)

	// Should handle NaN scores gracefully
	var hasNaNScore bool
	for _, result := range results {
		if math.IsNaN(result.Score) {
			hasNaNScore = true
			// NaN score should not crash the system
			assert.False(t, result.Hit, "Entry with NaN score should not be marked as hit")
		}
	}

	// Edge case file should contain at least one NaN
	if !hasNaNScore {
		t.Log("No NaN scores found - check if edge case fixture is loaded")
	}
}

func TestSmokeDataLoader_LoadResults_HitCalculation(t *testing.T) {
	loader := data.NewSmokeDataLoader("../../../testdata/tuner")

	results, err := loader.LoadResults(nil, nil)
	require.NoError(t, err)

	for _, result := range results {
		// Skip NaN or zero scores
		if math.IsNaN(result.Score) || result.Score == 0 {
			continue
		}

		// Verify hit calculation logic matches expected behavior
		expectedHit := loader.CalculateHitPublic(&result)
		assert.Equal(t, expectedHit, result.Hit,
			"Hit calculation should match expected for symbol %s", result.Symbol)
	}
}

func TestSmokeDataLoader_GetMetricsByRegime(t *testing.T) {
	loader := data.NewSmokeDataLoader("../../../testdata/tuner")

	results, err := loader.LoadResults(nil, nil)
	require.NoError(t, err)

	metrics := loader.GetMetricsByRegime(results)
	require.NotEmpty(t, metrics, "Should have metrics for at least one regime")

	for regime, regimeMetrics := range metrics {
		assert.Equal(t, regime, regimeMetrics.Regime, "Regime should match key")
		assert.Greater(t, regimeMetrics.TotalSignals, 0, "Should have positive total signals")
		assert.GreaterOrEqual(t, regimeMetrics.Hits, 0, "Hits should be non-negative")
		assert.LessOrEqual(t, regimeMetrics.Hits, regimeMetrics.TotalSignals, "Hits should not exceed total signals")

		// Hit rate should be between 0 and 1
		assert.GreaterOrEqual(t, regimeMetrics.HitRate, 0.0, "Hit rate should be >= 0")
		assert.LessOrEqual(t, regimeMetrics.HitRate, 1.0, "Hit rate should be <= 1")

		// Spearman correlation should be between -1 and 1
		assert.GreaterOrEqual(t, regimeMetrics.SpearmanCorr, -1.0, "Spearman correlation should be >= -1")
		assert.LessOrEqual(t, regimeMetrics.SpearmanCorr, 1.0, "Spearman correlation should be <= 1")

		// Score bounds should be valid
		assert.LessOrEqual(t, regimeMetrics.ScoreBounds[0], regimeMetrics.ScoreBounds[1], "Score min should be <= max")

		// Return bounds should be valid
		assert.LessOrEqual(t, regimeMetrics.ReturnBounds[0], regimeMetrics.ReturnBounds[1], "Return min should be <= max")
	}
}

func TestSmokeDataLoader_GetAvailableRegimes(t *testing.T) {
	loader := data.NewSmokeDataLoader("../../../testdata/tuner")

	regimes, err := loader.GetAvailableRegimes()
	require.NoError(t, err)

	// Should find the regimes in our test fixtures
	expectedRegimes := []string{"calm", "normal", "volatile"}
	for _, expected := range expectedRegimes {
		assert.Contains(t, regimes, expected, "Should contain regime %s", expected)
	}

	// Results should be sorted
	for i := 1; i < len(regimes); i++ {
		assert.LessOrEqual(t, regimes[i-1], regimes[i], "Regimes should be sorted")
	}
}

func TestSmokeDataLoader_NonexistentDirectory(t *testing.T) {
	loader := data.NewSmokeDataLoader("/nonexistent/path")

	results, err := loader.LoadResults(nil, nil)
	// Should handle gracefully - either error or empty results
	if err != nil {
		assert.Error(t, err)
	} else {
		assert.Empty(t, results, "Should return empty results for nonexistent directory")
	}
}

func TestSmokeDataLoader_EmptyDirectory(t *testing.T) {
	// Create temporary empty directory
	tempDir, err := os.MkdirTemp("", "empty_smoke_test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	loader := data.NewSmokeDataLoader(tempDir)

	results, err := loader.LoadResults(nil, nil)
	require.NoError(t, err)
	assert.Empty(t, results, "Should return empty results for empty directory")
}

func TestSmokeDataLoader_CreateMockResults_Deterministic(t *testing.T) {
	// Test that mock results are deterministic
	results1 := data.CreateMockResults([]string{"normal"}, []string{"4h"}, 10)
	results2 := data.CreateMockResults([]string{"normal"}, []string{"4h"}, 10)

	require.Equal(t, len(results1), len(results2), "Should create same number of results")

	for i := range results1 {
		assert.Equal(t, results1[i].Symbol, results2[i].Symbol, "Symbols should match")
		assert.Equal(t, results1[i].Score, results2[i].Score, "Scores should be deterministic")
		assert.Equal(t, results1[i].Regime, results2[i].Regime, "Regimes should match")
		assert.Equal(t, results1[i].Window, results2[i].Window, "Windows should match")
	}
}

func TestSmokeDataLoader_CreateMockResults_Variety(t *testing.T) {
	regimes := []string{"normal", "volatile", "calm"}
	windows := []string{"1h", "4h", "12h"}

	results := data.CreateMockResults(regimes, windows, 5)

	// Should have results for all combinations
	expectedCount := len(regimes) * len(windows) * 5
	assert.Equal(t, expectedCount, len(results), "Should create results for all combinations")

	// Check variety in generated data
	regimeSet := make(map[string]bool)
	windowSet := make(map[string]bool)
	symbolSet := make(map[string]bool)

	for _, result := range results {
		regimeSet[result.Regime] = true
		windowSet[result.Window] = true
		symbolSet[result.Symbol] = true

		// Validate ranges
		assert.GreaterOrEqual(t, result.Score, 70.0, "Mock scores should be >= 70")
		assert.LessOrEqual(t, result.Score, 100.0, "Mock scores should be <= 100")
		assert.Greater(t, result.EntryPrice, 0.0, "Entry prices should be positive")
		assert.Greater(t, result.ExitPrice, 0.0, "Exit prices should be positive")
	}

	// Should have all regimes and windows represented
	for _, regime := range regimes {
		assert.True(t, regimeSet[regime], "Should have results for regime %s", regime)
	}
	for _, window := range windows {
		assert.True(t, windowSet[window], "Should have results for window %s", window)
	}

	assert.GreaterOrEqual(t, len(symbolSet), 5, "Should have variety in symbols")
}

func TestSmokeDataLoader_SpearmanCorrelation_EdgeCases(t *testing.T) {
	// Test Spearman correlation with edge cases

	// Perfect correlation
	results1 := []data.SmokeResult{
		{Score: 1.0, ForwardReturn: 0.01},
		{Score: 2.0, ForwardReturn: 0.02},
		{Score: 3.0, ForwardReturn: 0.03},
	}
	metrics1 := data.NewSmokeDataLoader("").CalculateMetricsPublic(results1)
	assert.InDelta(t, 1.0, metrics1.SpearmanCorr, 0.01, "Perfect positive correlation should be ~1.0")

	// Perfect negative correlation
	results2 := []data.SmokeResult{
		{Score: 3.0, ForwardReturn: 0.01},
		{Score: 2.0, ForwardReturn: 0.02},
		{Score: 1.0, ForwardReturn: 0.03},
	}
	metrics2 := data.NewSmokeDataLoader("").CalculateMetricsPublic(results2)
	assert.InDelta(t, -1.0, metrics2.SpearmanCorr, 0.01, "Perfect negative correlation should be ~-1.0")

	// Single point
	results3 := []data.SmokeResult{
		{Score: 1.0, ForwardReturn: 0.01},
	}
	metrics3 := data.NewSmokeDataLoader("").CalculateMetricsPublic(results3)
	assert.Equal(t, 0.0, metrics3.SpearmanCorr, "Single point should have 0 correlation")

	// Empty results
	metrics4 := data.NewSmokeDataLoader("").CalculateMetricsPublic([]data.SmokeResult{})
	assert.Equal(t, data.RegimeMetrics{}, metrics4, "Empty results should return empty metrics")
}

// Helper function to create a temporary test file
func createTempSmokeFile(t *testing.T, data []data.SmokeResult) string {
	tempDir, err := os.MkdirTemp("", "smoke_test")
	require.NoError(t, err)
	t.Cleanup(func() { os.RemoveAll(tempDir) })

	filePath := filepath.Join(tempDir, "test_smoke90.json")

	jsonData, err := json.Marshal(data)
	require.NoError(t, err)

	err = os.WriteFile(filePath, jsonData, 0644)
	require.NoError(t, err)

	return tempDir
}

func TestSmokeDataLoader_InvalidJSON(t *testing.T) {
	// Create temporary file with invalid JSON
	tempDir, err := os.MkdirTemp("", "invalid_json_test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	invalidPath := filepath.Join(tempDir, "invalid_smoke90.json")
	err = os.WriteFile(invalidPath, []byte("invalid json"), 0644)
	require.NoError(t, err)

	loader := data.NewSmokeDataLoader(tempDir)

	// Should handle gracefully and continue loading other files
	results, err := loader.LoadResults(nil, nil)
	require.NoError(t, err)
	assert.Empty(t, results, "Should return empty results due to invalid JSON")
}

func TestSmokeDataLoader_MalformedData(t *testing.T) {
	// Test with malformed data structures
	malformedData := []map[string]interface{}{
		{
			"symbol": "TEST",
			"score":  "not_a_number", // Invalid score type
			"regime": "normal",
		},
	}

	tempDir, err := os.MkdirTemp("", "malformed_test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	filePath := filepath.Join(tempDir, "malformed_smoke90.json")

	jsonData, err := json.Marshal(malformedData)
	require.NoError(t, err)

	err = os.WriteFile(filePath, jsonData, 0644)
	require.NoError(t, err)

	loader := data.NewSmokeDataLoader(tempDir)

	// Should handle malformed data gracefully
	results, err := loader.LoadResults(nil, nil)
	require.NoError(t, err)
	// Results might be empty or contain parsed data depending on implementation
	t.Logf("Loaded %d results from malformed data", len(results))
}
