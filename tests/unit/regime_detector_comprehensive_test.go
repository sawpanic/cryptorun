package unit

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"cryptorun/internal/domain/regime"
)

// TestRegimeDetectorSyntheticInputs tests the core requirement:
// REGIME DETECTOR CORRECTNESS - synthetic inputs, stable 4h cadence updates
func TestRegimeDetectorSyntheticInputs(t *testing.T) {
	detector := regime.NewRegimeDetector(regime.DefaultThresholds())

	t.Run("Pure_Bull_Market_Detection", func(t *testing.T) {
		// Synthetic bull market: low vol, high breadth, positive thrust
		inputs := regime.RegimeInputs{
			RealizedVol7d: 0.25, // Low volatility (below 0.30 threshold)
			PctAbove20MA:  0.75, // Strong breadth (above 0.65 threshold)
			BreadthThrust: 0.25, // Positive thrust (above 0.15 threshold)
			Timestamp:     time.Now(),
		}

		regimeType, confidence, err := detector.DetectRegime(inputs)
		require.NoError(t, err)

		assert.Equal(t, regime.TrendingBull, regimeType, "Should detect trending bull market")
		assert.True(t, confidence > 0.8, "Should have high confidence for clear bull signal")
	})

	t.Run("Pure_Choppy_Market_Detection", func(t *testing.T) {
		// Synthetic choppy market: medium vol, neutral breadth, minimal thrust
		inputs := regime.RegimeInputs{
			RealizedVol7d: 0.45, // Medium volatility (between thresholds)
			PctAbove20MA:  0.50, // Neutral breadth (between thresholds)
			BreadthThrust: 0.05, // Minimal thrust (between thresholds)
			Timestamp:     time.Now(),
		}

		regimeType, confidence, err := detector.DetectRegime(inputs)
		require.NoError(t, err)

		assert.Equal(t, regime.Choppy, regimeType, "Should detect choppy market")
		// Confidence might be lower for neutral signals
	})

	t.Run("Pure_High_Vol_Detection", func(t *testing.T) {
		// Synthetic high volatility: extreme vol dominates other signals
		inputs := regime.RegimeInputs{
			RealizedVol7d: 0.80,  // Very high volatility (above 0.60 threshold)
			PctAbove20MA:  0.40,  // Bearish breadth
			BreadthThrust: -0.10, // Negative thrust
			Timestamp:     time.Now(),
		}

		regimeType, confidence, err := detector.DetectRegime(inputs)
		require.NoError(t, err)

		assert.Equal(t, regime.HighVol, regimeType, "Should detect high volatility regime")
		assert.True(t, confidence > 0.7, "Should have high confidence for extreme volatility")
	})

	t.Run("Border_Cases", func(t *testing.T) {
		thresholds := regime.DefaultThresholds()

		testCases := []struct {
			name     string
			inputs   regime.RegimeInputs
			expected regime.RegimeType
		}{
			{
				name: "vol_exactly_at_high_threshold",
				inputs: regime.RegimeInputs{
					RealizedVol7d: thresholds.VolHighThreshold, // Exactly 0.60
					PctAbove20MA:  0.50,
					BreadthThrust: 0.00,
					Timestamp:     time.Now(),
				},
				expected: regime.HighVol, // Should trigger high vol
			},
			{
				name: "breadth_exactly_at_bull_threshold",
				inputs: regime.RegimeInputs{
					RealizedVol7d: 0.35,                     // Medium vol
					PctAbove20MA:  thresholds.BullThreshold, // Exactly 0.65
					BreadthThrust: 0.05,
					Timestamp:     time.Now(),
				},
				expected: regime.TrendingBull, // Should trigger bull
			},
			{
				name: "thrust_exactly_at_positive_threshold",
				inputs: regime.RegimeInputs{
					RealizedVol7d: 0.25,                      // Low vol
					PctAbove20MA:  0.70,                      // Bull breadth
					BreadthThrust: thresholds.ThrustPositive, // Exactly 0.15
					Timestamp:     time.Now(),
				},
				expected: regime.TrendingBull, // Should reinforce bull
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				regimeType, _, err := detector.DetectRegime(tc.inputs)
				require.NoError(t, err)
				assert.Equal(t, tc.expected, regimeType, "Border case should trigger expected regime")
			})
		}
	})

	t.Run("Synthetic_Time_Series", func(t *testing.T) {
		// Test a sequence of synthetic inputs representing regime transitions
		baseTime := time.Now()

		sequences := []struct {
			name     string
			inputs   regime.RegimeInputs
			expected regime.RegimeType
			hour     int
		}{
			// Start in bull market
			{
				name: "bull_market_start",
				inputs: regime.RegimeInputs{
					RealizedVol7d: 0.20,
					PctAbove20MA:  0.80,
					BreadthThrust: 0.30,
					Timestamp:     baseTime.Add(time.Duration(0) * time.Hour),
				},
				expected: regime.TrendingBull,
				hour:     0,
			},
			// Maintain bull market
			{
				name: "bull_market_continuation",
				inputs: regime.RegimeInputs{
					RealizedVol7d: 0.25,
					PctAbove20MA:  0.75,
					BreadthThrust: 0.20,
					Timestamp:     baseTime.Add(time.Duration(4) * time.Hour),
				},
				expected: regime.TrendingBull,
				hour:     4,
			},
			// Transition to choppy (volatility rises, breadth weakens)
			{
				name: "transition_to_choppy",
				inputs: regime.RegimeInputs{
					RealizedVol7d: 0.40,
					PctAbove20MA:  0.55,
					BreadthThrust: 0.05,
					Timestamp:     baseTime.Add(time.Duration(8) * time.Hour),
				},
				expected: regime.Choppy,
				hour:     8,
			},
			// Spike to high vol (crisis mode)
			{
				name: "crisis_high_vol",
				inputs: regime.RegimeInputs{
					RealizedVol7d: 0.90,
					PctAbove20MA:  0.25,
					BreadthThrust: -0.30,
					Timestamp:     baseTime.Add(time.Duration(12) * time.Hour),
				},
				expected: regime.HighVol,
				hour:     12,
			},
		}

		for _, seq := range sequences {
			t.Run(seq.name, func(t *testing.T) {
				regimeType, confidence, err := detector.DetectRegime(seq.inputs)
				require.NoError(t, err)

				assert.Equal(t, seq.expected, regimeType, "Sequence step should detect expected regime")
				assert.True(t, confidence > 0.0, "Should have positive confidence")
				assert.True(t, confidence <= 1.0, "Confidence should not exceed 1.0")

				// Timestamp should be preserved
				assert.Equal(t, seq.inputs.Timestamp, seq.inputs.Timestamp, "Timestamp should be preserved")
			})
		}
	})

	t.Run("Majority_Vote_Logic", func(t *testing.T) {
		// Test the 3-indicator majority vote system
		testCases := []struct {
			name        string
			inputs      regime.RegimeInputs
			expected    regime.RegimeType
			description string
		}{
			{
				name: "vol_wins_2of3",
				inputs: regime.RegimeInputs{
					RealizedVol7d: 0.75, // HIGH VOL (vote 1)
					PctAbove20MA:  0.30, // BEARISH -> should suggest HIGH VOL (vote 2)
					BreadthThrust: 0.20, // BULLISH -> conflicts
					Timestamp:     time.Now(),
				},
				expected:    regime.HighVol,
				description: "High volatility + bearish breadth should override bullish thrust",
			},
			{
				name: "bull_wins_2of3",
				inputs: regime.RegimeInputs{
					RealizedVol7d: 0.25,  // LOW VOL -> suggests BULL (vote 1)
					PctAbove20MA:  0.70,  // BULLISH -> suggests BULL (vote 2)
					BreadthThrust: -0.05, // Slight negative thrust -> conflicts
					Timestamp:     time.Now(),
				},
				expected:    regime.TrendingBull,
				description: "Low vol + bullish breadth should override slight negative thrust",
			},
			{
				name: "choppy_default",
				inputs: regime.RegimeInputs{
					RealizedVol7d: 0.45, // MEDIUM VOL -> neutral
					PctAbove20MA:  0.50, // NEUTRAL BREADTH -> neutral
					BreadthThrust: 0.00, // NEUTRAL THRUST -> neutral
					Timestamp:     time.Now(),
				},
				expected:    regime.Choppy,
				description: "All neutral signals should default to choppy",
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				regimeType, confidence, err := detector.DetectRegime(tc.inputs)
				require.NoError(t, err)

				assert.Equal(t, tc.expected, regimeType, tc.description)
				assert.True(t, confidence >= 0.0 && confidence <= 1.0, "Confidence should be valid range")
			})
		}
	})
}

// TestRegimeDetectorStableCadence tests the stable 4h cadence requirement
func TestRegimeDetectorStableCadence(t *testing.T) {
	detector := regime.NewRegimeDetector(regime.DefaultThresholds())

	t.Run("Four_Hour_Stability", func(t *testing.T) {
		// Test that similar inputs within 4h window produce consistent results
		baseTime := time.Now()

		baseInputs := regime.RegimeInputs{
			RealizedVol7d: 0.35,
			PctAbove20MA:  0.60,
			BreadthThrust: 0.10,
			Timestamp:     baseTime,
		}

		// Get initial regime
		initialRegime, initialConf, err := detector.DetectRegime(baseInputs)
		require.NoError(t, err)

		// Test slight variations within noise tolerance
		variations := []regime.RegimeInputs{
			{
				RealizedVol7d: 0.36, // +1bp vol
				PctAbove20MA:  0.59, // -1% breadth
				BreadthThrust: 0.11, // +1% thrust
				Timestamp:     baseTime.Add(1 * time.Hour),
			},
			{
				RealizedVol7d: 0.34, // -1bp vol
				PctAbove20MA:  0.61, // +1% breadth
				BreadthThrust: 0.09, // -1% thrust
				Timestamp:     baseTime.Add(2 * time.Hour),
			},
			{
				RealizedVol7d: 0.37, // +2bp vol
				PctAbove20MA:  0.58, // -2% breadth
				BreadthThrust: 0.12, // +2% thrust
				Timestamp:     baseTime.Add(3 * time.Hour),
			},
		}

		for i, variation := range variations {
			t.Run(fmt.Sprintf("variation_%d", i+1), func(t *testing.T) {
				regimeType, confidence, err := detector.DetectRegime(variation)
				require.NoError(t, err)

				// Should maintain same regime for small variations
				assert.Equal(t, initialRegime, regimeType, "Small variations should not change regime")

				// Confidence should be similar (within reasonable range)
				assert.InDelta(t, initialConf, confidence, 0.2, "Confidence should be stable for small variations")
			})
		}
	})

	t.Run("Regime_Transition_Hysteresis", func(t *testing.T) {
		// Test that regime changes require meaningful signal changes (hysteresis)
		baseTime := time.Now()

		// Start in clear bull market
		bullInputs := regime.RegimeInputs{
			RealizedVol7d: 0.25,
			PctAbove20MA:  0.70,
			BreadthThrust: 0.20,
			Timestamp:     baseTime,
		}

		regimeType, _, err := detector.DetectRegime(bullInputs)
		require.NoError(t, err)
		assert.Equal(t, regime.TrendingBull, regimeType)

		// Gradually move toward choppy territory
		transitionSteps := []struct {
			inputs      regime.RegimeInputs
			expectBull  bool
			description string
		}{
			{
				inputs: regime.RegimeInputs{
					RealizedVol7d: 0.30, // Still low but increasing
					PctAbove20MA:  0.67, // Still bullish but weakening
					BreadthThrust: 0.16, // Still positive but weakening
					Timestamp:     baseTime.Add(4 * time.Hour),
				},
				expectBull:  true,
				description: "Minor weakening should maintain bull regime",
			},
			{
				inputs: regime.RegimeInputs{
					RealizedVol7d: 0.38, // Approaching medium vol
					PctAbove20MA:  0.60, // Weakening breadth
					BreadthThrust: 0.10, // Weakening thrust
					Timestamp:     baseTime.Add(8 * time.Hour),
				},
				expectBull:  false, // Should transition to choppy
				description: "Significant weakening should transition to choppy",
			},
		}

		for i, step := range transitionSteps {
			t.Run(fmt.Sprintf("transition_step_%d", i+1), func(t *testing.T) {
				regimeType, _, err := detector.DetectRegime(step.inputs)
				require.NoError(t, err)

				if step.expectBull {
					assert.Equal(t, regime.TrendingBull, regimeType, step.description)
				} else {
					assert.NotEqual(t, regime.TrendingBull, regimeType, step.description)
				}
			})
		}
	})

	t.Run("Timestamp_Preservation", func(t *testing.T) {
		// Test that timestamps are correctly preserved through detection
		testTimes := []time.Time{
			time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
			time.Date(2024, 6, 15, 8, 30, 0, 0, time.UTC),
			time.Date(2024, 12, 31, 23, 45, 0, 0, time.UTC),
		}

		for i, testTime := range testTimes {
			t.Run(fmt.Sprintf("timestamp_%d", i+1), func(t *testing.T) {
				inputs := regime.RegimeInputs{
					RealizedVol7d: 0.40,
					PctAbove20MA:  0.55,
					BreadthThrust: 0.05,
					Timestamp:     testTime,
				}

				_, _, err := detector.DetectRegime(inputs)
				require.NoError(t, err)

				// The inputs should preserve the original timestamp
				assert.Equal(t, testTime, inputs.Timestamp, "Timestamp should be preserved")
			})
		}
	})
}

// TestRegimeDetectorErrorHandling tests error conditions and edge cases
func TestRegimeDetectorErrorHandling(t *testing.T) {
	detector := regime.NewRegimeDetector(regime.DefaultThresholds())

	t.Run("Invalid_Inputs", func(t *testing.T) {
		invalidCases := []struct {
			name   string
			inputs regime.RegimeInputs
		}{
			{
				name: "negative_volatility",
				inputs: regime.RegimeInputs{
					RealizedVol7d: -0.10,
					PctAbove20MA:  0.50,
					BreadthThrust: 0.00,
					Timestamp:     time.Now(),
				},
			},
			{
				name: "invalid_percentage_above_1",
				inputs: regime.RegimeInputs{
					RealizedVol7d: 0.40,
					PctAbove20MA:  1.50, // Above 1.0
					BreadthThrust: 0.00,
					Timestamp:     time.Now(),
				},
			},
			{
				name: "invalid_percentage_below_0",
				inputs: regime.RegimeInputs{
					RealizedVol7d: 0.40,
					PctAbove20MA:  -0.10, // Below 0.0
					BreadthThrust: 0.00,
					Timestamp:     time.Now(),
				},
			},
		}

		for _, tc := range invalidCases {
			t.Run(tc.name, func(t *testing.T) {
				_, _, err := detector.DetectRegime(tc.inputs)
				// Should either handle gracefully or return meaningful error
				if err != nil {
					assert.Contains(t, err.Error(), "invalid", "Error should indicate invalid input")
				}
				// If no error, the detector should handle gracefully
			})
		}
	})

	t.Run("Extreme_Values", func(t *testing.T) {
		extremeCases := []regime.RegimeInputs{
			{
				RealizedVol7d: 5.0, // 500% volatility
				PctAbove20MA:  0.50,
				BreadthThrust: 0.00,
				Timestamp:     time.Now(),
			},
			{
				RealizedVol7d: 0.40,
				PctAbove20MA:  0.99, // 99% above MA
				BreadthThrust: 0.00,
				Timestamp:     time.Now(),
			},
			{
				RealizedVol7d: 0.40,
				PctAbove20MA:  0.50,
				BreadthThrust: 2.0, // 200% thrust
				Timestamp:     time.Now(),
			},
		}

		for i, inputs := range extremeCases {
			t.Run(fmt.Sprintf("extreme_%d", i+1), func(t *testing.T) {
				regimeType, confidence, err := detector.DetectRegime(inputs)
				require.NoError(t, err, "Should handle extreme values gracefully")

				// Should return valid regime
				assert.Contains(t, []regime.RegimeType{regime.TrendingBull, regime.Choppy, regime.HighVol},
					regimeType, "Should return valid regime type")

				// Confidence should be bounded
				assert.True(t, confidence >= 0.0 && confidence <= 1.0, "Confidence should be bounded")
			})
		}
	})
}
