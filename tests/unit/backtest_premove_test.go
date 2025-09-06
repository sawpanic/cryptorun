package unit

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"cryptorun/src/application/premove"
)

func TestBacktestHarness_EmptyArtifacts(t *testing.T) {
	// Create temporary directories
	tempDir := t.TempDir()
	artifactsDir := filepath.Join(tempDir, "artifacts")
	outputDir := filepath.Join(tempDir, "output")

	if err := os.MkdirAll(filepath.Join(artifactsDir, "premove"), 0755); err != nil {
		t.Fatal(err)
	}

	// Run backtest with no artifacts
	harness := premove.NewBacktestHarness(artifactsDir, outputDir)
	err := harness.RunBacktest()

	if err != nil {
		t.Fatalf("Expected no error for empty artifacts, got: %v", err)
	}

	// Verify empty outputs were created
	checkFileExists(t, filepath.Join(outputDir, "hit_rates_by_state_and_regime.json"))
	checkFileExists(t, filepath.Join(outputDir, "isotonic_calibration_curve.json"))
	checkFileExists(t, filepath.Join(outputDir, "cvd_resid_r2_daily.csv"))
}

func TestBacktestHarness_WithFixtures(t *testing.T) {
	// Create temporary directories
	tempDir := t.TempDir()
	artifactsDir := filepath.Join(tempDir, "artifacts")
	outputDir := filepath.Join(tempDir, "output")

	if err := os.MkdirAll(filepath.Join(artifactsDir, "premove"), 0755); err != nil {
		t.Fatal(err)
	}

	// Create test fixture JSONL file
	fixtures := createDeterministicFixtures()
	fixtureFile := filepath.Join(artifactsDir, "premove", "test_fixture.jsonl")

	if err := writeJSONLFixtures(fixtureFile, fixtures); err != nil {
		t.Fatal(err)
	}

	// Run backtest
	harness := premove.NewBacktestHarness(artifactsDir, outputDir)
	err := harness.RunBacktest()

	if err != nil {
		t.Fatalf("Backtest failed: %v", err)
	}

	// Verify outputs
	validateHitRatesOutput(t, filepath.Join(outputDir, "hit_rates_by_state_and_regime.json"))
	validateCalibrationOutput(t, filepath.Join(outputDir, "isotonic_calibration_curve.json"))
	validateCVDOutput(t, filepath.Join(outputDir, "cvd_resid_r2_daily.csv"))
}

func TestIsotonicCalibration_Monotonicity(t *testing.T) {
	// Create test data with non-monotonic probabilities
	tempDir := t.TempDir()
	artifactsDir := filepath.Join(tempDir, "artifacts")
	outputDir := filepath.Join(tempDir, "output")

	if err := os.MkdirAll(filepath.Join(artifactsDir, "premove"), 0755); err != nil {
		t.Fatal(err)
	}

	// Create fixtures that would violate monotonicity if not corrected
	fixtures := []premove.PITRecord{
		{Symbol: "BTCUSD", Timestamp: time.Now().Add(-24 * time.Hour), Score: 10, State: "WATCH",
			Regime: "risk_on", ActualMove: floatPtr(0.02)}, // Low score, small move
		{Symbol: "BTCUSD", Timestamp: time.Now().Add(-23 * time.Hour), Score: 50, State: "PREPARE",
			Regime: "risk_on", ActualMove: floatPtr(0.08)}, // Mid score, big move
		{Symbol: "BTCUSD", Timestamp: time.Now().Add(-22 * time.Hour), Score: 80, State: "PRIME",
			Regime: "risk_on", ActualMove: floatPtr(0.01)}, // High score, small move
		{Symbol: "BTCUSD", Timestamp: time.Now().Add(-21 * time.Hour), Score: 120, State: "EXECUTE",
			Regime: "risk_on", ActualMove: floatPtr(0.10)}, // Highest score, big move
	}

	fixtureFile := filepath.Join(artifactsDir, "premove", "monotonic_test.jsonl")
	if err := writeJSONLFixtures(fixtureFile, fixtures); err != nil {
		t.Fatal(err)
	}

	// Run backtest and verify monotonic calibration
	harness := premove.NewBacktestHarness(artifactsDir, outputDir)
	err := harness.RunBacktest()
	if err != nil {
		t.Fatalf("Backtest failed: %v", err)
	}

	// Load and validate calibration curve monotonicity
	curveFile := filepath.Join(outputDir, "isotonic_calibration_curve.json")
	curveData, err := os.ReadFile(curveFile)
	if err != nil {
		t.Fatal(err)
	}

	var curve premove.CalibrationCurve
	if err := json.Unmarshal(curveData, &curve); err != nil {
		t.Fatal(err)
	}

	// Verify monotonicity
	for i := 1; i < len(curve.Points); i++ {
		if curve.Points[i].Probability < curve.Points[i-1].Probability {
			t.Errorf("Calibration curve violates monotonicity at index %d: %.3f < %.3f",
				i, curve.Points[i].Probability, curve.Points[i-1].Probability)
		}
	}
}

func TestHitRateComputation_ByStateAndRegime(t *testing.T) {
	tempDir := t.TempDir()
	artifactsDir := filepath.Join(tempDir, "artifacts")
	outputDir := filepath.Join(tempDir, "output")

	if err := os.MkdirAll(filepath.Join(artifactsDir, "premove"), 0755); err != nil {
		t.Fatal(err)
	}

	// Create fixtures with known hit rates
	fixtures := []premove.PITRecord{
		// WATCH state, risk_on regime: 2/3 success rate
		{Symbol: "BTCUSD", Score: 65, State: "WATCH", Regime: "risk_on", ActualMove: floatPtr(0.06)}, // Hit
		{Symbol: "ETHUSD", Score: 70, State: "WATCH", Regime: "risk_on", ActualMove: floatPtr(0.08)}, // Hit
		{Symbol: "ADAUSD", Score: 68, State: "WATCH", Regime: "risk_on", ActualMove: floatPtr(0.02)}, // Miss

		// EXECUTE state, risk_off regime: 3/3 success rate
		{Symbol: "BTCUSD", Score: 125, State: "EXECUTE", Regime: "risk_off", ActualMove: floatPtr(0.12)}, // Hit
		{Symbol: "ETHUSD", Score: 130, State: "EXECUTE", Regime: "risk_off", ActualMove: floatPtr(0.09)}, // Hit
		{Symbol: "SOLUSD", Score: 135, State: "EXECUTE", Regime: "risk_off", ActualMove: floatPtr(0.15)}, // Hit
	}

	fixtureFile := filepath.Join(artifactsDir, "premove", "hit_rate_test.jsonl")
	if err := writeJSONLFixtures(fixtureFile, fixtures); err != nil {
		t.Fatal(err)
	}

	// Run backtest
	harness := premove.NewBacktestHarness(artifactsDir, outputDir)
	err := harness.RunBacktest()
	if err != nil {
		t.Fatalf("Backtest failed: %v", err)
	}

	// Validate specific hit rates
	hitRatesFile := filepath.Join(outputDir, "hit_rates_by_state_and_regime.json")
	hitRatesData, err := os.ReadFile(hitRatesFile)
	if err != nil {
		t.Fatal(err)
	}

	var hitRates []premove.HitRate
	if err := json.Unmarshal(hitRatesData, &hitRates); err != nil {
		t.Fatal(err)
	}

	// Verify expected hit rates
	expectedRates := map[string]float64{
		"WATCH_risk_on":    2.0 / 3.0,
		"EXECUTE_risk_off": 1.0, // 3/3
	}

	for _, hr := range hitRates {
		key := hr.State + "_" + hr.Regime
		if expected, exists := expectedRates[key]; exists {
			if abs(hr.HitRate-expected) > 0.001 {
				t.Errorf("Hit rate for %s: expected %.3f, got %.3f", key, expected, hr.HitRate)
			}
		}
	}
}

func TestCVDResidualTracking(t *testing.T) {
	tempDir := t.TempDir()
	artifactsDir := filepath.Join(tempDir, "artifacts")
	outputDir := filepath.Join(tempDir, "output")

	if err := os.MkdirAll(filepath.Join(artifactsDir, "premove"), 0755); err != nil {
		t.Fatal(err)
	}

	// Create fixtures with CVD residual data
	today := time.Now().Truncate(24 * time.Hour)
	fixtures := []premove.PITRecord{
		{
			Symbol:    "BTCUSD",
			Timestamp: today.Add(1 * time.Hour),
			Score:     75,
			State:     "PREPARE",
			Regime:    "risk_on",
			SubScores: map[string]float64{"cvd_residual": 0.5, "price": 45000},
		},
		{
			Symbol:    "BTCUSD",
			Timestamp: today.Add(2 * time.Hour),
			Score:     80,
			State:     "PREPARE",
			Regime:    "risk_on",
			SubScores: map[string]float64{"cvd_residual": 0.8, "price": 46000},
		},
		{
			Symbol:    "BTCUSD",
			Timestamp: today.Add(3 * time.Hour),
			Score:     85,
			State:     "PRIME",
			Regime:    "risk_on",
			SubScores: map[string]float64{"cvd_residual": 1.2, "price": 47000},
		},
	}

	fixtureFile := filepath.Join(artifactsDir, "premove", "cvd_test.jsonl")
	if err := writeJSONLFixtures(fixtureFile, fixtures); err != nil {
		t.Fatal(err)
	}

	// Run backtest
	harness := premove.NewBacktestHarness(artifactsDir, outputDir)
	err := harness.RunBacktest()
	if err != nil {
		t.Fatalf("Backtest failed: %v", err)
	}

	// Verify CVD R² output exists and has expected content
	cvdFile := filepath.Join(outputDir, "cvd_resid_r2_daily.csv")
	checkFileExists(t, cvdFile)

	content, err := os.ReadFile(cvdFile)
	if err != nil {
		t.Fatal(err)
	}

	contentStr := string(content)
	if !contains(contentStr, "BTCUSD") {
		t.Error("CVD output should contain BTCUSD symbol")
	}
	if !contains(contentStr, today.Format("2006-01-02")) {
		t.Error("CVD output should contain today's date")
	}
}

func TestRunBacktestInternal_CLIFree(t *testing.T) {
	// Test the CLI-free interface
	tempDir := t.TempDir()
	artifactsDir := filepath.Join(tempDir, "artifacts")
	outputDir := filepath.Join(tempDir, "output")

	if err := os.MkdirAll(filepath.Join(artifactsDir, "premove"), 0755); err != nil {
		t.Fatal(err)
	}

	// Use CLI-free interface
	err := premove.RunBacktestInternal(artifactsDir, outputDir)
	if err != nil {
		t.Fatalf("CLI-free backtest failed: %v", err)
	}

	// Verify outputs exist
	checkFileExists(t, filepath.Join(outputDir, "hit_rates_by_state_and_regime.json"))
	checkFileExists(t, filepath.Join(outputDir, "isotonic_calibration_curve.json"))
	checkFileExists(t, filepath.Join(outputDir, "cvd_resid_r2_daily.csv"))
}

// Helper functions

func createDeterministicFixtures() []premove.PITRecord {
	baseTime := time.Date(2024, 9, 1, 10, 0, 0, 0, time.UTC)

	return []premove.PITRecord{
		{
			Symbol:      "BTCUSD",
			Timestamp:   baseTime,
			Score:       75.5,
			State:       "PREPARE",
			SubScores:   map[string]float64{"momentum": 30, "depth": 20, "cvd_residual": 0.8, "price": 45000},
			PassedGates: 2,
			Penalties:   map[string]float64{"freshness": 0.1},
			TopReasons:  []string{"strong_momentum", "adequate_depth"},
			Sources:     map[string]interface{}{"kraken": "live", "binance": "cached"},
			Regime:      "risk_on",
			ActualMove:  floatPtr(0.07), // 7% move - above 5% threshold
		},
		{
			Symbol:      "ETHUSD",
			Timestamp:   baseTime.Add(1 * time.Hour),
			Score:       62.3,
			State:       "WATCH",
			SubScores:   map[string]float64{"momentum": 25, "depth": 15, "cvd_residual": 0.3, "price": 2800},
			PassedGates: 1,
			Penalties:   map[string]float64{"freshness": 0.2},
			TopReasons:  []string{"moderate_momentum"},
			Sources:     map[string]interface{}{"kraken": "live"},
			Regime:      "risk_off",
			ActualMove:  floatPtr(0.03), // 3% move - below 5% threshold
		},
		{
			Symbol:      "SOLUSD",
			Timestamp:   baseTime.Add(2 * time.Hour),
			Score:       95.8,
			State:       "PRIME",
			SubScores:   map[string]float64{"momentum": 40, "depth": 25, "cvd_residual": 1.5, "price": 150},
			PassedGates: 3,
			Penalties:   map[string]float64{},
			TopReasons:  []string{"exceptional_momentum", "deep_liquidity", "whale_activity"},
			Sources:     map[string]interface{}{"kraken": "live", "binance": "live"},
			Regime:      "risk_on",
			ActualMove:  floatPtr(0.12), // 12% move - well above threshold
		},
	}
}

func writeJSONLFixtures(filename string, fixtures []premove.PITRecord) error {
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	for _, fixture := range fixtures {
		if err := encoder.Encode(fixture); err != nil {
			return err
		}
	}

	return nil
}

func checkFileExists(t *testing.T, filepath string) {
	if _, err := os.Stat(filepath); os.IsNotExist(err) {
		t.Errorf("Expected file does not exist: %s", filepath)
	}
}

func validateHitRatesOutput(t *testing.T, filepath string) {
	data, err := os.ReadFile(filepath)
	if err != nil {
		t.Fatal(err)
	}

	var hitRates []premove.HitRate
	if err := json.Unmarshal(data, &hitRates); err != nil {
		t.Fatal(err)
	}

	// Should have hit rates for each state-regime combination
	if len(hitRates) == 0 {
		t.Error("Expected non-empty hit rates")
	}

	// Validate structure
	for _, hr := range hitRates {
		if hr.State == "" {
			t.Error("Hit rate missing state")
		}
		if hr.HitRate < 0 || hr.HitRate > 1 {
			t.Errorf("Invalid hit rate: %f", hr.HitRate)
		}
		if hr.ConfidenceCI[0] > hr.ConfidenceCI[1] {
			t.Error("Invalid confidence interval")
		}
	}
}

func validateCalibrationOutput(t *testing.T, filepath string) {
	data, err := os.ReadFile(filepath)
	if err != nil {
		t.Fatal(err)
	}

	var curve premove.CalibrationCurve
	if err := json.Unmarshal(data, &curve); err != nil {
		t.Fatal(err)
	}

	if len(curve.Points) == 0 {
		t.Error("Calibration curve should have points")
	}

	if curve.GeneratedAt.IsZero() {
		t.Error("Calibration curve should have generation timestamp")
	}

	// Verify monotonicity
	for i := 1; i < len(curve.Points); i++ {
		if curve.Points[i].Score < curve.Points[i-1].Score {
			t.Error("Calibration points should be sorted by score")
		}
		if curve.Points[i].Probability < curve.Points[i-1].Probability {
			t.Error("Calibration curve should be monotonic")
		}
	}
}

func validateCVDOutput(t *testing.T, filepath string) {
	data, err := os.ReadFile(filepath)
	if err != nil {
		t.Fatal(err)
	}

	content := string(data)
	if !contains(content, "date,symbol,r2_score,samples") {
		t.Error("CVD output should have proper CSV header")
	}
}

func floatPtr(f float64) *float64 {
	return &f
}

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || (len(s) > len(substr) &&
		(s[:len(substr)] == substr || s[len(s)-len(substr):] == substr ||
			indexOf(s, substr) != -1)))
}

func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
