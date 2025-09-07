package unit

import (
	"math"
	"testing"
	"time"

	"github.com/sawpanic/cryptorun/internal/domain/regime"
)

func TestRegimeDetector_DefaultThresholds(t *testing.T) {
	thresholds := regime.DefaultThresholds()

	// Validate threshold ranges are sensible
	if thresholds.VolLowThreshold >= thresholds.VolHighThreshold {
		t.Errorf("vol low threshold %f should be less than high threshold %f",
			thresholds.VolLowThreshold, thresholds.VolHighThreshold)
	}

	if thresholds.BearThreshold >= thresholds.BullThreshold {
		t.Errorf("bear threshold %f should be less than bull threshold %f",
			thresholds.BearThreshold, thresholds.BullThreshold)
	}

	if thresholds.ThrustNegative >= thresholds.ThrustPositive {
		t.Errorf("negative thrust %f should be less than positive thrust %f",
			thresholds.ThrustNegative, thresholds.ThrustPositive)
	}
}

func TestRegimeDetector_BasicDetection(t *testing.T) {
	detector := regime.NewRegimeDetector(regime.DefaultThresholds())

	testCases := []struct {
		name     string
		inputs   regime.RegimeInputs
		expected regime.RegimeType
	}{
		{
			name: "high volatility detection",
			inputs: regime.RegimeInputs{
				RealizedVol7d: 0.70, // Above high threshold (0.60)
				PctAbove20MA:  0.50, // Neutral
				BreadthThrust: 0.10, // Neutral
				Timestamp:     time.Now(),
			},
			expected: regime.HighVol,
		},
		{
			name: "trending bull detection",
			inputs: regime.RegimeInputs{
				RealizedVol7d: 0.25, // Low volatility
				PctAbove20MA:  0.70, // Strong bullish breadth
				BreadthThrust: 0.20, // Positive thrust
				Timestamp:     time.Now(),
			},
			expected: regime.TrendingBull,
		},
		{
			name: "choppy market detection",
			inputs: regime.RegimeInputs{
				RealizedVol7d: 0.40, // Medium volatility
				PctAbove20MA:  0.50, // Neutral breadth
				BreadthThrust: 0.05, // Neutral thrust
				Timestamp:     time.Now(),
			},
			expected: regime.Choppy,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := detector.DetectRegime(tc.inputs)
			if result != tc.expected {
				t.Errorf("expected regime %s, got %s", tc.expected, result)
			}
		})
	}
}

func TestRegimeDetector_MajorityVoting(t *testing.T) {
	detector := regime.NewRegimeDetector(regime.DefaultThresholds())
	baseTime := time.Now()

	// Feed conflicting signals over time
	conflictingInputs := []regime.RegimeInputs{
		{
			RealizedVol7d: 0.70, // High vol signal
			PctAbove20MA:  0.70, // Bull signal
			BreadthThrust: 0.20, // Bull signal
			Timestamp:     baseTime,
		},
		{
			RealizedVol7d: 0.25, // Bull signal
			PctAbove20MA:  0.70, // Bull signal
			BreadthThrust: 0.20, // Bull signal
			Timestamp:     baseTime.Add(4 * time.Hour),
		},
		{
			RealizedVol7d: 0.25, // Bull signal
			PctAbove20MA:  0.70, // Bull signal
			BreadthThrust: 0.20, // Bull signal
			Timestamp:     baseTime.Add(8 * time.Hour),
		},
	}

	var finalRegime regime.RegimeType
	for _, inputs := range conflictingInputs {
		finalRegime = detector.DetectRegime(inputs)
	}

	// Should stabilize on TrendingBull due to majority voting
	if finalRegime != regime.TrendingBull {
		t.Errorf("expected majority voting to result in TrendingBull, got %s", finalRegime)
	}
}

func TestRegimeDetector_UpdateCadence(t *testing.T) {
	detector := regime.NewRegimeDetector(regime.DefaultThresholds())
	baseTime := time.Now()

	// First detection
	inputs1 := regime.RegimeInputs{
		RealizedVol7d: 0.70,
		PctAbove20MA:  0.50,
		BreadthThrust: 0.10,
		Timestamp:     baseTime,
	}
	regime1 := detector.DetectRegime(inputs1)

	// Second detection within 4h (should not update regime)
	inputs2 := regime.RegimeInputs{
		RealizedVol7d: 0.25, // Changed to bull signals
		PctAbove20MA:  0.70,
		BreadthThrust: 0.20,
		Timestamp:     baseTime.Add(2 * time.Hour), // Only 2h later
	}
	regime2 := detector.DetectRegime(inputs2)

	if regime1 != regime2 {
		t.Errorf("regime should not change within 4h cadence: %s vs %s", regime1, regime2)
	}

	// Third detection after 4h (should allow update)
	inputs3 := regime.RegimeInputs{
		RealizedVol7d: 0.25,
		PctAbove20MA:  0.70,
		BreadthThrust: 0.20,
		Timestamp:     baseTime.Add(5 * time.Hour), // 5h later
	}
	regime3 := detector.DetectRegime(inputs3)

	// Should allow regime change after cadence period
	if regime3 == regime1 && regime3 != regime.TrendingBull {
		t.Errorf("regime should be able to change after 4h cadence")
	}
}

func TestRegimeDetector_StabilityBias(t *testing.T) {
	detector := regime.NewRegimeDetector(regime.DefaultThresholds())
	baseTime := time.Now()

	// Establish initial regime
	initialInputs := regime.RegimeInputs{
		RealizedVol7d: 0.25,
		PctAbove20MA:  0.70,
		BreadthThrust: 0.20,
		Timestamp:     baseTime,
	}
	detector.DetectRegime(initialInputs)

	// Slightly conflicting signal (should not cause regime change due to stability bias)
	marginalInputs := regime.RegimeInputs{
		RealizedVol7d: 0.45, // Slightly higher vol
		PctAbove20MA:  0.55, // Slightly lower breadth
		BreadthThrust: 0.10, // Slightly lower thrust
		Timestamp:     baseTime.Add(5 * time.Hour),
	}

	regime := detector.DetectRegime(marginalInputs)

	// Should remain in original regime due to stability bias
	if regime != regime.TrendingBull {
		t.Errorf("expected stability bias to prevent regime change, got %s", regime)
	}
}

func TestRegimeDetector_InputValidation(t *testing.T) {
	detector := regime.NewRegimeDetector(regime.DefaultThresholds())

	invalidCases := []struct {
		name   string
		inputs regime.RegimeInputs
	}{
		{
			name: "negative volatility",
			inputs: regime.RegimeInputs{
				RealizedVol7d: -0.1,
				PctAbove20MA:  0.5,
				BreadthThrust: 0.1,
				Timestamp:     time.Now(),
			},
		},
		{
			name: "excessive volatility",
			inputs: regime.RegimeInputs{
				RealizedVol7d: 2.5, // Over 250% vol
				PctAbove20MA:  0.5,
				BreadthThrust: 0.1,
				Timestamp:     time.Now(),
			},
		},
		{
			name: "invalid percent above MA",
			inputs: regime.RegimeInputs{
				RealizedVol7d: 0.5,
				PctAbove20MA:  1.5, // Over 100%
				BreadthThrust: 0.1,
				Timestamp:     time.Now(),
			},
		},
		{
			name: "excessive breadth thrust",
			inputs: regime.RegimeInputs{
				RealizedVol7d: 0.5,
				PctAbove20MA:  0.5,
				BreadthThrust: 1.5, // Over ±1.0 range
				Timestamp:     time.Now(),
			},
		},
		{
			name: "zero timestamp",
			inputs: regime.RegimeInputs{
				RealizedVol7d: 0.5,
				PctAbove20MA:  0.5,
				BreadthThrust: 0.1,
				Timestamp:     time.Time{}, // Zero timestamp
			},
		},
	}

	for _, tc := range invalidCases {
		t.Run(tc.name, func(t *testing.T) {
			err := detector.ValidateInputs(tc.inputs)
			if err == nil {
				t.Errorf("expected validation error for %s", tc.name)
			}
		})
	}
}

func TestRegimeDetector_History(t *testing.T) {
	detector := regime.NewRegimeDetector(regime.DefaultThresholds())
	baseTime := time.Now()

	// Add several inputs
	inputs := []regime.RegimeInputs{
		{
			RealizedVol7d: 0.25,
			PctAbove20MA:  0.70,
			BreadthThrust: 0.20,
			Timestamp:     baseTime,
		},
		{
			RealizedVol7d: 0.30,
			PctAbove20MA:  0.65,
			BreadthThrust: 0.15,
			Timestamp:     baseTime.Add(time.Hour),
		},
		{
			RealizedVol7d: 0.35,
			PctAbove20MA:  0.60,
			BreadthThrust: 0.10,
			Timestamp:     baseTime.Add(2 * time.Hour),
		},
	}

	for _, input := range inputs {
		detector.DetectRegime(input)
	}

	history := detector.GetRegimeHistory()

	if len(history) != len(inputs) {
		t.Errorf("expected history length %d, got %d", len(inputs), len(history))
	}

	// Verify history is in correct order (most recent first or chronological)
	for i, historicalInput := range history {
		if !historicalInput.Timestamp.Equal(inputs[i].Timestamp) {
			t.Errorf("history order incorrect at position %d", i)
		}
	}
}

func TestRegimeDetector_StatusReporting(t *testing.T) {
	detector := regime.NewRegimeDetector(regime.DefaultThresholds())

	inputs := regime.RegimeInputs{
		RealizedVol7d: 0.25,
		PctAbove20MA:  0.70,
		BreadthThrust: 0.20,
		Timestamp:     time.Now(),
	}

	detector.DetectRegime(inputs)
	status := detector.GetDetectorStatus()

	// Verify status contains expected fields
	expectedFields := []string{"current_regime", "last_update", "update_cadence", "history_length", "thresholds"}
	for _, field := range expectedFields {
		if _, exists := status[field]; !exists {
			t.Errorf("status missing expected field: %s", field)
		}
	}

	// Verify current regime is set
	if regime, ok := status["current_regime"].(string); !ok || regime == "" {
		t.Errorf("current_regime should be a non-empty string")
	}
}

// Benchmark regime detection performance
func BenchmarkRegimeDetector(b *testing.B) {
	detector := regime.NewRegimeDetector(regime.DefaultThresholds())
	baseTime := time.Now()

	inputs := regime.RegimeInputs{
		RealizedVol7d: 0.45,
		PctAbove20MA:  0.55,
		BreadthThrust: 0.12,
		Timestamp:     baseTime,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		inputs.Timestamp = baseTime.Add(time.Duration(i) * time.Hour)
		detector.DetectRegime(inputs)
	}
}
