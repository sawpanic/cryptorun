package weights

import (
	"math"
	"testing"
	"time"

	"github.com/sawpanic/cryptorun/internal/tune/data"
)

func TestObjectiveFunction_Evaluate(t *testing.T) {
	// Setup test data
	smokeData, benchData := createTestDataSets()
	baseWeights := createBaseWeights()
	config := DefaultObjectiveConfig()

	objective := NewObjectiveFunction(config, smokeData, benchData, baseWeights, "normal")

	testWeights := RegimeWeights{
		MomentumCore:      0.425,
		TechnicalResidual: 0.20,
		VolumeResidual:    0.15,
		QualityResidual:   0.225,
	}

	result, err := objective.Evaluate(testWeights)
	if err != nil {
		t.Fatalf("evaluation failed: %v", err)
	}

	// Verify result structure
	if result.TotalScore < 0 {
		t.Errorf("total score should be non-negative, got %f", result.TotalScore)
	}

	if result.HitRateScore < 0 || result.HitRateScore > 1 {
		t.Errorf("hit rate score should be in [0,1], got %f", result.HitRateScore)
	}

	if result.SpearmanScore < -1 || result.SpearmanScore > 1 {
		t.Errorf("Spearman score should be in [-1,1], got %f", result.SpearmanScore)
	}

	if result.RegularizationPenalty < 0 {
		t.Errorf("regularization penalty should be non-negative, got %f", result.RegularizationPenalty)
	}

	// Verify sample counts
	if result.SmokeCount == 0 && result.BenchCount == 0 {
		t.Error("should have at least some sample data")
	}

	t.Logf("Objective result: total=%.4f, hit_rate=%.4f, spearman=%.4f, l2=%.4f",
		result.TotalScore, result.HitRateScore, result.SpearmanScore, result.RegularizationPenalty)
}

func TestObjectiveFunction_EvaluateEmptyData(t *testing.T) {
	config := DefaultObjectiveConfig()
	baseWeights := createBaseWeights()

	// Test with no data
	objective := NewObjectiveFunction(config, []data.SmokeResult{}, []data.BenchResult{}, baseWeights, "normal")

	testWeights := RegimeWeights{
		MomentumCore:      0.425,
		TechnicalResidual: 0.20,
		VolumeResidual:    0.15,
		QualityResidual:   0.225,
	}

	_, err := objective.Evaluate(testWeights)
	if err == nil {
		t.Error("expected error with no data, got none")
	}

	if !containsSubstring(err.Error(), "no data available") {
		t.Errorf("expected 'no data available' error, got: %v", err)
	}
}

func TestObjectiveFunction_EvaluateSmokeOnly(t *testing.T) {
	smokeData, _ := createTestDataSets()
	baseWeights := createBaseWeights()
	config := DefaultObjectiveConfig()

	// Test with smoke data only
	objective := NewObjectiveFunction(config, smokeData, []data.BenchResult{}, baseWeights, "normal")

	testWeights := RegimeWeights{
		MomentumCore:      0.425,
		TechnicalResidual: 0.20,
		VolumeResidual:    0.15,
		QualityResidual:   0.225,
	}

	result, err := objective.Evaluate(testWeights)
	if err != nil {
		t.Fatalf("evaluation failed: %v", err)
	}

	if result.SmokeCount == 0 {
		t.Error("expected smoke count > 0")
	}
	if result.BenchCount != 0 {
		t.Error("expected bench count = 0")
	}

	// Should use smoke metrics directly
	if result.HitRateScore != result.SmokeHitRate {
		t.Errorf("hit rate should equal smoke hit rate: %f vs %f", result.HitRateScore, result.SmokeHitRate)
	}
}

func TestObjectiveFunction_EvaluateBenchOnly(t *testing.T) {
	_, benchData := createTestDataSets()
	baseWeights := createBaseWeights()
	config := DefaultObjectiveConfig()

	// Test with bench data only
	objective := NewObjectiveFunction(config, []data.SmokeResult{}, benchData, baseWeights, "normal")

	testWeights := RegimeWeights{
		MomentumCore:      0.425,
		TechnicalResidual: 0.20,
		VolumeResidual:    0.15,
		QualityResidual:   0.225,
	}

	result, err := objective.Evaluate(testWeights)
	if err != nil {
		t.Fatalf("evaluation failed: %v", err)
	}

	if result.BenchCount == 0 {
		t.Error("expected bench count > 0")
	}
	if result.SmokeCount != 0 {
		t.Error("expected smoke count = 0")
	}

	// Should use bench metrics directly
	if result.HitRateScore != result.BenchHitRate {
		t.Errorf("hit rate should equal bench hit rate: %f vs %f", result.HitRateScore, result.BenchHitRate)
	}
}

func TestObjectiveFunction_EvaluateBothDataSources(t *testing.T) {
	smokeData, benchData := createTestDataSets()
	baseWeights := createBaseWeights()
	config := DefaultObjectiveConfig()

	objective := NewObjectiveFunction(config, smokeData, benchData, baseWeights, "normal")

	testWeights := RegimeWeights{
		MomentumCore:      0.425,
		TechnicalResidual: 0.20,
		VolumeResidual:    0.15,
		QualityResidual:   0.225,
	}

	result, err := objective.Evaluate(testWeights)
	if err != nil {
		t.Fatalf("evaluation failed: %v", err)
	}

	if result.SmokeCount == 0 || result.BenchCount == 0 {
		t.Error("expected both smoke and bench counts > 0")
	}

	// Combined metrics should be weighted average (smoke 60%, bench 40%)
	expectedHitRate := 0.6*result.SmokeHitRate + 0.4*result.BenchHitRate
	if math.Abs(result.HitRateScore-expectedHitRate) > 0.001 {
		t.Errorf("hit rate not properly combined: got %f, expected %f", result.HitRateScore, expectedHitRate)
	}

	expectedSpearman := 0.6*result.SmokeSpearman + 0.4*result.BenchSpearman
	if math.Abs(result.SpearmanScore-expectedSpearman) > 0.001 {
		t.Errorf("Spearman not properly combined: got %f, expected %f", result.SpearmanScore, expectedSpearman)
	}
}

func TestObjectiveFunction_PrimaryMetricHitRate(t *testing.T) {
	smokeData, benchData := createTestDataSets()
	baseWeights := createBaseWeights()

	config := ObjectiveConfig{
		HitRateWeight:    0.8,
		SpearmanWeight:   0.2,
		RegularizationL2: 0.005,
		PrimaryMetric:    "hitrate",
	}

	objective := NewObjectiveFunction(config, smokeData, benchData, baseWeights, "normal")

	testWeights := RegimeWeights{
		MomentumCore:      0.425,
		TechnicalResidual: 0.20,
		VolumeResidual:    0.15,
		QualityResidual:   0.225,
	}

	result, err := objective.Evaluate(testWeights)
	if err != nil {
		t.Fatalf("evaluation failed: %v", err)
	}

	// With hitrate primary, hit rate should dominate the score
	expectedScore := config.HitRateWeight*result.HitRateScore +
		config.SpearmanWeight*result.SpearmanScore -
		config.RegularizationL2*result.RegularizationPenalty

	if math.Abs(result.TotalScore-expectedScore) > 0.001 {
		t.Errorf("total score mismatch: got %f, expected %f", result.TotalScore, expectedScore)
	}
}

func TestObjectiveFunction_PrimaryMetricSpearman(t *testing.T) {
	smokeData, benchData := createTestDataSets()
	baseWeights := createBaseWeights()

	config := ObjectiveConfig{
		HitRateWeight:    0.3,
		SpearmanWeight:   0.7,
		RegularizationL2: 0.005,
		PrimaryMetric:    "spearman",
	}

	objective := NewObjectiveFunction(config, smokeData, benchData, baseWeights, "normal")

	testWeights := RegimeWeights{
		MomentumCore:      0.425,
		TechnicalResidual: 0.20,
		VolumeResidual:    0.15,
		QualityResidual:   0.225,
	}

	result, err := objective.Evaluate(testWeights)
	if err != nil {
		t.Fatalf("evaluation failed: %v", err)
	}

	// With spearman primary, weights are flipped
	expectedScore := config.SpearmanWeight*result.SpearmanScore +
		config.HitRateWeight*result.HitRateScore -
		config.RegularizationL2*result.RegularizationPenalty

	if math.Abs(result.TotalScore-expectedScore) > 0.001 {
		t.Errorf("total score mismatch with Spearman primary: got %f, expected %f", result.TotalScore, expectedScore)
	}
}

func TestObjectiveFunction_RegularizationPenalty(t *testing.T) {
	smokeData, benchData := createTestDataSets()
	baseWeights := createBaseWeights()

	// Test with high regularization
	config := ObjectiveConfig{
		HitRateWeight:    0.7,
		SpearmanWeight:   0.3,
		RegularizationL2: 0.1, // High penalty
		PrimaryMetric:    "hitrate",
	}

	objective := NewObjectiveFunction(config, smokeData, benchData, baseWeights, "normal")

	// Test weights very different from base
	farWeights := RegimeWeights{
		MomentumCore:      0.50, // Far from base 0.425
		TechnicalResidual: 0.15, // Far from base 0.20
		VolumeResidual:    0.20, // Far from base 0.15
		QualityResidual:   0.15, // Far from base 0.225
	}

	// Test weights close to base
	closeWeights := RegimeWeights{
		MomentumCore:      0.430, // Close to base 0.425
		TechnicalResidual: 0.205, // Close to base 0.20
		VolumeResidual:    0.155, // Close to base 0.15
		QualityResidual:   0.210, // Close to base 0.225
	}

	farResult, err := objective.Evaluate(farWeights)
	if err != nil {
		t.Fatalf("evaluation failed for far weights: %v", err)
	}

	closeResult, err := objective.Evaluate(closeWeights)
	if err != nil {
		t.Fatalf("evaluation failed for close weights: %v", err)
	}

	// Far weights should have higher penalty
	if farResult.RegularizationPenalty <= closeResult.RegularizationPenalty {
		t.Errorf("far weights should have higher penalty: %f vs %f",
			farResult.RegularizationPenalty, closeResult.RegularizationPenalty)
	}

	t.Logf("Regularization test: far=%.4f, close=%.4f",
		farResult.RegularizationPenalty, closeResult.RegularizationPenalty)
}

func TestObjectiveFunction_SimulateAdjustedScore(t *testing.T) {
	smokeData, benchData := createTestDataSets()
	baseWeights := createBaseWeights()
	config := DefaultObjectiveConfig()

	objective := NewObjectiveFunction(config, smokeData, benchData, baseWeights, "normal")

	originalScore := 80.0

	// Test weights same as base (no adjustment expected)
	sameWeights := baseWeights["normal"]
	adjustedSame := objective.simulateAdjustedScore(originalScore, sameWeights)
	if math.Abs(adjustedSame-originalScore) > 1.0 { // Allow small numerical differences
		t.Errorf("same weights should produce similar score: %f -> %f", originalScore, adjustedSame)
	}

	// Test weights different from base
	differentWeights := RegimeWeights{
		MomentumCore:      0.50,
		TechnicalResidual: 0.15,
		VolumeResidual:    0.20,
		QualityResidual:   0.15,
	}

	adjustedDiff := objective.simulateAdjustedScore(originalScore, differentWeights)
	if adjustedDiff == originalScore {
		t.Error("different weights should produce different adjusted score")
	}

	// Adjusted score should stay in valid range [0, 100]
	if adjustedDiff < 0 || adjustedDiff > 100 {
		t.Errorf("adjusted score out of range [0, 100]: %f", adjustedDiff)
	}

	t.Logf("Score adjustment test: %f -> same=%f, diff=%f", originalScore, adjustedSame, adjustedDiff)
}

func TestObjectiveFunction_CalculateConfidenceInterval(t *testing.T) {
	objective := NewObjectiveFunction(DefaultObjectiveConfig(), []data.SmokeResult{}, []data.BenchResult{}, nil, "normal")

	tests := []struct {
		name       string
		values     []float64
		confidence float64
		expectErr  bool
	}{
		{
			name:       "normal distribution",
			values:     []float64{0.5, 0.6, 0.4, 0.55, 0.45, 0.65, 0.35, 0.58, 0.42, 0.52},
			confidence: 0.95,
			expectErr:  false,
		},
		{
			name:       "single value",
			values:     []float64{0.5},
			confidence: 0.95,
			expectErr:  true,
		},
		{
			name:       "empty values",
			values:     []float64{},
			confidence: 0.95,
			expectErr:  true,
		},
		{
			name:       "two values",
			values:     []float64{0.4, 0.6},
			confidence: 0.95,
			expectErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lower, upper, err := objective.CalculateConfidenceInterval(tt.values, tt.confidence)

			if tt.expectErr && err == nil {
				t.Error("expected error, got none")
			}
			if !tt.expectErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			if !tt.expectErr {
				if lower > upper {
					t.Errorf("lower bound should be <= upper bound: %f > %f", lower, upper)
				}

				// Mean should be within the confidence interval
				if len(tt.values) > 0 {
					sum := 0.0
					for _, v := range tt.values {
						sum += v
					}
					mean := sum / float64(len(tt.values))

					if mean < lower || mean > upper {
						t.Errorf("mean %f should be within CI [%f, %f]", mean, lower, upper)
					}
				}

				t.Logf("CI for %s: [%f, %f]", tt.name, lower, upper)
			}
		})
	}
}

func TestObjectiveFunction_NonexistentRegime(t *testing.T) {
	smokeData, benchData := createTestDataSets()
	baseWeights := createBaseWeights()
	config := DefaultObjectiveConfig()

	// Test with regime not present in data
	objective := NewObjectiveFunction(config, smokeData, benchData, baseWeights, "nonexistent")

	testWeights := RegimeWeights{
		MomentumCore:      0.425,
		TechnicalResidual: 0.20,
		VolumeResidual:    0.15,
		QualityResidual:   0.225,
	}

	_, err := objective.Evaluate(testWeights)
	if err == nil {
		t.Error("expected error for nonexistent regime")
	}
}

func TestObjectiveFunction_SingleAssetPanel(t *testing.T) {
	// Create single-asset data
	smokeData := []data.SmokeResult{
		{
			Symbol:        "BTC",
			Timestamp:     time.Now(),
			Score:         85.0,
			Regime:        "normal",
			ForwardReturn: 0.030,
			Window:        "4h",
		},
	}

	benchData := []data.BenchResult{
		{
			Symbol:     "BTC",
			Timestamp:  time.Now(),
			Score:      85.0,
			Regime:     "normal",
			ActualGain: 0.030,
			Window:     "24h",
		},
	}

	baseWeights := createBaseWeights()
	config := DefaultObjectiveConfig()

	objective := NewObjectiveFunction(config, smokeData, benchData, baseWeights, "normal")

	testWeights := RegimeWeights{
		MomentumCore:      0.425,
		TechnicalResidual: 0.20,
		VolumeResidual:    0.15,
		QualityResidual:   0.225,
	}

	result, err := objective.Evaluate(testWeights)
	if err != nil {
		t.Fatalf("single asset evaluation failed: %v", err)
	}

	// With single asset, correlation should be 0 (no variance)
	if result.SpearmanScore != 0 {
		t.Errorf("expected zero Spearman correlation for single asset, got %f", result.SpearmanScore)
	}

	if result.SmokeCount != 1 || result.BenchCount != 1 {
		t.Errorf("expected counts of 1, got smoke=%d, bench=%d", result.SmokeCount, result.BenchCount)
	}
}

func TestObjectiveFunction_DegenerateVariance(t *testing.T) {
	// Create data with identical scores (zero variance)
	smokeData := []data.SmokeResult{
		{Symbol: "BTC", Score: 75.0, Regime: "normal", ForwardReturn: 0.020, Window: "4h"},
		{Symbol: "ETH", Score: 75.0, Regime: "normal", ForwardReturn: 0.030, Window: "4h"},
		{Symbol: "ADA", Score: 75.0, Regime: "normal", ForwardReturn: 0.025, Window: "4h"},
	}

	baseWeights := createBaseWeights()
	config := DefaultObjectiveConfig()

	objective := NewObjectiveFunction(config, smokeData, []data.BenchResult{}, baseWeights, "normal")

	testWeights := RegimeWeights{
		MomentumCore:      0.425,
		TechnicalResidual: 0.20,
		VolumeResidual:    0.15,
		QualityResidual:   0.225,
	}

	result, err := objective.Evaluate(testWeights)
	if err != nil {
		t.Fatalf("degenerate variance evaluation failed: %v", err)
	}

	// Zero score variance should result in zero or undefined correlation
	// Note: Spearman correlation with identical scores is undefined, implementation may vary
	if math.IsInf(result.SpearmanScore, 0) {
		t.Errorf("Spearman correlation should not be infinite with identical scores, got %f", result.SpearmanScore)
	}

	t.Logf("Degenerate variance test passed: Spearman=%f", result.SpearmanScore)
}

// Helper functions for creating test data
func createTestDataSets() ([]data.SmokeResult, []data.BenchResult) {
	baseTime := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)

	smokeData := []data.SmokeResult{
		{
			Symbol:        "BTC",
			Timestamp:     baseTime,
			Score:         85.5,
			Regime:        "normal",
			ForwardReturn: 0.0325,
			Hit:           true,
			Window:        "4h",
		},
		{
			Symbol:        "ETH",
			Timestamp:     baseTime,
			Score:         72.3,
			Regime:        "normal",
			ForwardReturn: 0.018,
			Hit:           false,
			Window:        "4h",
		},
		{
			Symbol:        "ADA",
			Timestamp:     baseTime,
			Score:         68.1,
			Regime:        "volatile",
			ForwardReturn: 0.012,
			Hit:           false,
			Window:        "4h",
		},
	}

	benchData := []data.BenchResult{
		{
			Symbol:       "BTC",
			Timestamp:    baseTime,
			Score:        89.5,
			Regime:       "normal",
			Rank:         1,
			BatchSize:    3,
			ActualGain:   0.0425,
			BenchmarkHit: true,
			Window:       "24h",
		},
		{
			Symbol:       "ETH",
			Timestamp:    baseTime,
			Score:        76.2,
			Regime:       "normal",
			Rank:         2,
			BatchSize:    3,
			ActualGain:   0.0315,
			BenchmarkHit: false,
			Window:       "24h",
		},
		{
			Symbol:       "ADA",
			Timestamp:    baseTime,
			Score:        65.3,
			Regime:       "calm",
			Rank:         3,
			BatchSize:    3,
			ActualGain:   0.008,
			BenchmarkHit: false,
			Window:       "24h",
		},
	}

	return smokeData, benchData
}

func createBaseWeights() map[string]RegimeWeights {
	return map[string]RegimeWeights{
		"normal": {
			MomentumCore:      0.425,
			TechnicalResidual: 0.20,
			VolumeResidual:    0.15,
			QualityResidual:   0.225,
		},
		"calm": {
			MomentumCore:      0.45,
			TechnicalResidual: 0.215,
			VolumeResidual:    0.125,
			QualityResidual:   0.21,
		},
		"volatile": {
			MomentumCore:      0.44,
			TechnicalResidual: 0.18,
			VolumeResidual:    0.185,
			QualityResidual:   0.195,
		},
	}
}
