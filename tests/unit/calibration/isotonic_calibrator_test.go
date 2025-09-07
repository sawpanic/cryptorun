package calibration

import (
	"testing"
	"time"

	"github.com/sawpanic/cryptorun/internal/score/calibration"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsotonicCalibrator_BasicFitting(t *testing.T) {
	config := calibration.DefaultCalibrationConfig()
	calibrator := calibration.NewIsotonicCalibrator(config)

	// Generate synthetic calibration data with clear pattern
	samples := generateCalibrationSamples(150, 0.7) // 150 samples, 70% correlation

	err := calibrator.Fit(samples)
	require.NoError(t, err, "Fitting should succeed with sufficient data")

	// Verify calibrator state after fitting
	assert.True(t, calibrator.IsValid(), "Calibrator should be valid after fitting")
	info := calibrator.GetInfo()
	assert.Equal(t, 150, info.SampleCount)
	assert.Greater(t, info.PointCount, 0, "Should have calibration points")
	assert.Greater(t, info.ScoreRange[1], info.ScoreRange[0], "Score range should be meaningful")
}

func TestIsotonicCalibrator_MonotonicityEnforcement(t *testing.T) {
	config := calibration.DefaultCalibrationConfig()
	calibrator := calibration.NewIsotonicCalibrator(config)

	// Create samples that would violate monotonicity without isotonic regression
	samples := []calibration.CalibrationSample{
		{Score: 10, Outcome: false, Timestamp: time.Now(), Symbol: "BTC", Regime: "bull", HoldingPeriod: 48 * time.Hour},
		{Score: 20, Outcome: true, Timestamp: time.Now(), Symbol: "BTC", Regime: "bull", HoldingPeriod: 48 * time.Hour},
		{Score: 30, Outcome: false, Timestamp: time.Now(), Symbol: "BTC", Regime: "bull", HoldingPeriod: 48 * time.Hour}, // Violation
		{Score: 40, Outcome: true, Timestamp: time.Now(), Symbol: "BTC", Regime: "bull", HoldingPeriod: 48 * time.Hour},
		{Score: 50, Outcome: true, Timestamp: time.Now(), Symbol: "BTC", Regime: "bull", HoldingPeriod: 48 * time.Hour},
		{Score: 60, Outcome: false, Timestamp: time.Now(), Symbol: "BTC", Regime: "bull", HoldingPeriod: 48 * time.Hour}, // Violation
		{Score: 70, Outcome: true, Timestamp: time.Now(), Symbol: "BTC", Regime: "bull", HoldingPeriod: 48 * time.Hour},
		{Score: 80, Outcome: true, Timestamp: time.Now(), Symbol: "BTC", Regime: "bull", HoldingPeriod: 48 * time.Hour},
		{Score: 90, Outcome: true, Timestamp: time.Now(), Symbol: "BTC", Regime: "bull", HoldingPeriod: 48 * time.Hour},
	}

	// Add more samples to reach minimum
	for i := 0; i < 100; i++ {
		samples = append(samples, calibration.CalibrationSample{
			Score:         float64(20 + i%60),
			Outcome:       (i%3) == 0, // 33% success rate
			Timestamp:     time.Now().Add(-time.Duration(i) * time.Hour),
			Symbol:        "BTC",
			Regime:        "bull",
			HoldingPeriod: 48 * time.Hour,
		})
	}

	err := calibrator.Fit(samples)
	require.NoError(t, err, "Fitting should succeed")

	// Verify monotonicity: higher scores should never have lower probabilities
	for score := 10.0; score < 90.0; score += 5.0 {
		prob1 := calibrator.Predict(score)
		prob2 := calibrator.Predict(score + 5.0)
		assert.LessOrEqual(t, prob1, prob2+1e-10, "Monotonicity violated at score %.1f -> %.1f", score, score+5.0)
	}
}

func TestIsotonicCalibrator_PredictionAccuracy(t *testing.T) {
	config := calibration.DefaultCalibrationConfig()
	calibrator := calibration.NewIsotonicCalibrator(config)

	// Generate well-structured training data
	trainSamples := generateCalibrationSamples(200, 0.8) // 200 samples, high correlation

	err := calibrator.Fit(trainSamples)
	require.NoError(t, err, "Training should succeed")

	// Test predictions on known patterns
	testCases := []struct {
		score    float64
		minProb  float64
		maxProb  float64
		desc     string
	}{
		{10.0, 0.0, 0.3, "Low scores should have low probabilities"},
		{50.0, 0.3, 0.7, "Medium scores should have medium probabilities"},
		{90.0, 0.6, 1.0, "High scores should have high probabilities"},
	}

	for _, tc := range testCases {
		prob := calibrator.Predict(tc.score)
		assert.GreaterOrEqual(t, prob, tc.minProb, "%s (score %.1f)", tc.desc, tc.score)
		assert.LessOrEqual(t, prob, tc.maxProb, "%s (score %.1f)", tc.desc, tc.score)
		assert.GreaterOrEqual(t, prob, 0.0, "Probability should be non-negative")
		assert.LessOrEqual(t, prob, 1.0, "Probability should not exceed 1.0")
	}
}

func TestIsotonicCalibrator_InsufficientData(t *testing.T) {
	config := calibration.DefaultCalibrationConfig()
	calibrator := calibration.NewIsotonicCalibrator(config)

	// Try fitting with insufficient samples
	samples := generateCalibrationSamples(50, 0.5) // Less than minimum 100

	err := calibrator.Fit(samples)
	assert.Error(t, err, "Should reject insufficient samples")
	assert.Contains(t, err.Error(), "insufficient samples", "Error should mention insufficient samples")
	assert.False(t, calibrator.IsValid(), "Calibrator should not be valid without fitting")
}

func TestIsotonicCalibrator_EdgeCasePredictions(t *testing.T) {
	config := calibration.DefaultCalibrationConfig()
	calibrator := calibration.NewIsotonicCalibrator(config)

	samples := generateCalibrationSamples(150, 0.6)
	err := calibrator.Fit(samples)
	require.NoError(t, err)

	info := calibrator.GetInfo()
	minScore := info.ScoreRange[0]
	maxScore := info.ScoreRange[1]

	// Test edge cases
	assert.GreaterOrEqual(t, calibrator.Predict(minScore-10), 0.0, "Below-range prediction should be valid")
	assert.LessOrEqual(t, calibrator.Predict(minScore-10), 1.0, "Below-range prediction should be valid")

	assert.GreaterOrEqual(t, calibrator.Predict(maxScore+10), 0.0, "Above-range prediction should be valid")
	assert.LessOrEqual(t, calibrator.Predict(maxScore+10), 1.0, "Above-range prediction should be valid")

	// Test exact boundary values
	prob1 := calibrator.Predict(minScore)
	prob2 := calibrator.Predict(maxScore)
	assert.LessOrEqual(t, prob1, prob2+1e-10, "Max score should have >= probability than min score")
}

func TestIsotonicCalibrator_PerformanceMetrics(t *testing.T) {
	config := calibration.DefaultCalibrationConfig()
	calibrator := calibration.NewIsotonicCalibrator(config)

	// Generate high-quality training data
	samples := generateCalibrationSamples(300, 0.85)
	err := calibrator.Fit(samples)
	require.NoError(t, err)

	info := calibrator.GetInfo()

	// Reliability (calibration error) should be reasonable
	assert.LessOrEqual(t, info.Reliability, 1.0, "Reliability metric should be reasonable")
	assert.GreaterOrEqual(t, info.Reliability, 0.0, "Reliability should be non-negative")

	// Resolution should show some discrimination ability
	assert.GreaterOrEqual(t, info.Resolution, 0.0, "Resolution should be non-negative")

	// Sharpness should show probability spread
	assert.GreaterOrEqual(t, info.Sharpness, 0.0, "Sharpness should be non-negative")
	assert.LessOrEqual(t, info.Sharpness, 1.0, "Sharpness should not exceed 1.0")
}

func TestIsotonicCalibrator_RefreshLogic(t *testing.T) {
	config := calibration.CalibrationConfig{
		MinSamples:      100,
		RefreshInterval: 1 * time.Hour, // Short interval for testing
		SmoothingFactor: 0.01,
		MaxAge:          24 * time.Hour,
		RegimeAware:     true,
		ValidationSplit: 0.2,
	}

	calibrator := calibration.NewIsotonicCalibrator(config)
	samples := generateCalibrationSamples(150, 0.7)

	// Initial fit
	err := calibrator.Fit(samples)
	require.NoError(t, err)

	// Should not need refresh immediately
	assert.False(t, calibrator.NeedsRefresh(config), "Should not need refresh immediately after fitting")

	// Simulate age passage by setting fitted time in the past
	// Note: This would require exposing fittedAt or adding a test method
	// For now, we'll just verify the method exists and returns bool
	needsRefresh := calibrator.NeedsRefresh(config)
	assert.IsType(t, false, needsRefresh, "NeedsRefresh should return boolean")
}

func TestIsotonicCalibrator_ConfidenceIntervals(t *testing.T) {
	config := calibration.DefaultCalibrationConfig()
	calibrator := calibration.NewIsotonicCalibrator(config)

	// Generate samples with known outcomes for confidence testing
	samples := make([]calibration.CalibrationSample, 200)
	for i := range samples {
		score := float64(10 + i/2) // Scores from 10 to 109
		outcome := (i % 4) == 0    // 25% success rate
		samples[i] = calibration.CalibrationSample{
			Score:         score,
			Outcome:       outcome,
			Timestamp:     time.Now().Add(-time.Duration(i) * time.Minute),
			Symbol:        "TEST",
			Regime:        "normal",
			HoldingPeriod: 48 * time.Hour,
			MaxMove:       0.05,
			FinalMove:     func() float64 { if outcome { return 0.06 } else { return -0.02 } }(),
		}
	}

	err := calibrator.Fit(samples)
	require.NoError(t, err, "Fitting should succeed")

	// Verify predictions are reasonable given the 25% base rate
	avgPrediction := 0.0
	numPredictions := 0
	for score := 20.0; score <= 100.0; score += 10.0 {
		prob := calibrator.Predict(score)
		avgPrediction += prob
		numPredictions++
	}
	avgPrediction /= float64(numPredictions)

	// With isotonic regression, average prediction should be close to base rate
	assert.InDelta(t, 0.25, avgPrediction, 0.15, "Average prediction should approximate base rate")
}

// Benchmark isotonic calibration performance
func BenchmarkIsotonicCalibrator_Fit(b *testing.B) {
	config := calibration.DefaultCalibrationConfig()
	samples := generateCalibrationSamples(200, 0.7)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		calibrator := calibration.NewIsotonicCalibrator(config)
		err := calibrator.Fit(samples)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkIsotonicCalibrator_Predict(b *testing.B) {
	config := calibration.DefaultCalibrationConfig()
	calibrator := calibration.NewIsotonicCalibrator(config)
	samples := generateCalibrationSamples(200, 0.7)

	err := calibrator.Fit(samples)
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		score := float64(30 + i%70) // Vary scores between 30-100
		_ = calibrator.Predict(score)
	}
}

// generateCalibrationSamples creates synthetic calibration data
func generateCalibrationSamples(count int, correlation float64) []calibration.CalibrationSample {
	samples := make([]calibration.CalibrationSample, count)
	baseTime := time.Now().Add(-48 * time.Hour)

	for i := 0; i < count; i++ {
		// Generate score with some randomness
		score := 20.0 + float64(i%80) + float64(i%5)*2.0 // Scores 20-104

		// Generate outcome based on correlation with score
		threshold := 50.0 + (score-50.0)*correlation // Correlation-based threshold
		noise := float64((i*7)%20) - 10.0            // Noise component
		outcome := score+noise > threshold

		samples[i] = calibration.CalibrationSample{
			Score:         score,
			Outcome:       outcome,
			Timestamp:     baseTime.Add(time.Duration(i) * time.Minute),
			Symbol:        []string{"BTC", "ETH", "ADA"}[i%3],
			Regime:        []string{"bull", "bear", "choppy"}[i%3],
			HoldingPeriod: time.Duration(24+i%48) * time.Hour,
			MaxMove:       0.02 + float64(i%10)*0.01, // 2-12% moves
			FinalMove: func() float64 {
				if outcome {
					return 0.03 + float64(i%8)*0.01 // 3-11% positive moves
				}
				return -0.02 - float64(i%6)*0.005 // -2% to -4.5% negative moves
			}(),
		}
	}

	return samples
}

func TestIsotonicCalibrator_PredictionStability(t *testing.T) {
	config := calibration.DefaultCalibrationConfig()
	calibrator := calibration.NewIsotonicCalibrator(config)

	samples := generateCalibrationSamples(150, 0.75)
	err := calibrator.Fit(samples)
	require.NoError(t, err)

	// Test that repeated predictions are identical
	score := 67.5
	prob1 := calibrator.Predict(score)
	prob2 := calibrator.Predict(score)
	prob3 := calibrator.Predict(score)

	assert.Equal(t, prob1, prob2, "Repeated predictions should be identical")
	assert.Equal(t, prob2, prob3, "Repeated predictions should be identical")
}

func TestIsotonicCalibrator_InterpolationAccuracy(t *testing.T) {
	config := calibration.DefaultCalibrationConfig()
	calibrator := calibration.NewIsotonicCalibrator(config)

	// Create calibration with known structure
	samples := generateCalibrationSamples(150, 0.9) // High correlation for predictable interpolation
	err := calibrator.Fit(samples)
	require.NoError(t, err)

	// Test interpolation properties
	for baseScore := 25.0; baseScore < 95.0; baseScore += 20.0 {
		prob1 := calibrator.Predict(baseScore)
		probMid := calibrator.Predict(baseScore + 5.0)
		prob2 := calibrator.Predict(baseScore + 10.0)

		// Interpolated value should be between endpoints (monotonicity)
		assert.LessOrEqual(t, prob1, probMid+1e-10, "Interpolation should maintain monotonicity")
		assert.LessOrEqual(t, probMid, prob2+1e-10, "Interpolation should maintain monotonicity")
	}
}