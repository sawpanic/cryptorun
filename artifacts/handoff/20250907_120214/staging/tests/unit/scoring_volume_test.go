package unit

import (
	"fmt"
	"math"
	"testing"

	"cryptorun/internal/domain/scoring"
)

func TestNormalizeVolumeScore(t *testing.T) {
	testCases := []struct {
		name           string
		volume         float64
		expectedScore  float64
		scoreTolerance float64
		expectIlliquid bool
		expectValid    bool
	}{
		{
			name:           "Unit volume",
			volume:         1.0,
			expectedScore:  50.0,
			scoreTolerance: 0.1,
			expectIlliquid: false,
			expectValid:    true,
		},
		{
			name:           "High volume",
			volume:         10.0,
			expectedScore:  75.0,
			scoreTolerance: 0.1,
			expectIlliquid: false,
			expectValid:    true,
		},
		{
			name:           "Low volume",
			volume:         0.1,
			expectedScore:  25.0,
			scoreTolerance: 0.1,
			expectIlliquid: false,
			expectValid:    true,
		},
		{
			name:           "Zero volume",
			volume:         0.0,
			expectedScore:  50.0, // Policy: zero volume → 50.0 component-neutral score
			scoreTolerance: 0.1,
			expectIlliquid: true, // Should flag as illiquid
			expectValid:    true,
		},
		{
			name:           "Negative volume",
			volume:         -1.0,
			expectedScore:  0.0, // Clamp to 0
			scoreTolerance: 0.1,
			expectIlliquid: true,  // Should flag as illiquid
			expectValid:    false, // Invalid negative
		},
		{
			name:           "NaN volume",
			volume:         math.NaN(),
			expectedScore:  50.0, // Component-neutral for NaN
			scoreTolerance: 0.1,
			expectIlliquid: true,  // Should flag as illiquid
			expectValid:    false, // Invalid NaN
		},
		{
			name:           "Positive infinity",
			volume:         math.Inf(1),
			expectedScore:  50.0, // Component-neutral for Inf
			scoreTolerance: 0.1,
			expectIlliquid: true,  // Should flag as illiquid
			expectValid:    false, // Invalid Inf
		},
		{
			name:           "Negative infinity",
			volume:         math.Inf(-1),
			expectedScore:  50.0, // Component-neutral for Inf
			scoreTolerance: 0.1,
			expectIlliquid: true,  // Should flag as illiquid
			expectValid:    false, // Invalid Inf
		},
		{
			name:           "Very high volume",
			volume:         100.0,
			expectedScore:  100.0, // Should be capped at 100
			scoreTolerance: 0.1,
			expectIlliquid: false,
			expectValid:    true,
		},
		{
			name:           "Very low positive volume",
			volume:         0.01,
			expectedScore:  0.0, // Log10(0.01) = -2, so 50 + (-2)*25 = 0
			scoreTolerance: 0.1,
			expectIlliquid: false,
			expectValid:    true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := scoring.NormalizeVolumeScore(tc.volume)

			// Check score
			if math.Abs(result.Score-tc.expectedScore) > tc.scoreTolerance {
				t.Errorf("Score mismatch: got %f, want %f ±%f",
					result.Score, tc.expectedScore, tc.scoreTolerance)
			}

			// Check illiquidity flag
			if result.Illiquidity != tc.expectIlliquid {
				t.Errorf("Illiquidity flag mismatch: got %v, want %v",
					result.Illiquidity, tc.expectIlliquid)
			}

			// Check validity flag
			if result.VolumeValid != tc.expectValid {
				t.Errorf("Validity flag mismatch: got %v, want %v",
					result.VolumeValid, tc.expectValid)
			}

			// Score should always be in valid range
			if result.Score < 0.0 || result.Score > 100.0 {
				t.Errorf("Score out of range: %f", result.Score)
			}
		})
	}
}

func TestVolumeScoreRange(t *testing.T) {
	// Test a range of volumes to ensure no score explosions
	volumes := []float64{0.001, 0.01, 0.1, 0.5, 1.0, 2.0, 5.0, 10.0, 50.0, 100.0, 1000.0}

	for _, vol := range volumes {
		t.Run("Volume_"+fmt.Sprintf("%.3f", vol), func(t *testing.T) {
			result := scoring.NormalizeVolumeScore(vol)

			if result.Score < 0.0 || result.Score > 100.0 {
				t.Errorf("Volume %f produced out-of-range score: %f", vol, result.Score)
			}

			if vol > 0 && result.Illiquidity {
				t.Errorf("Positive volume %f should not be flagged as illiquid", vol)
			}
		})
	}
}

func TestVolumeScoreMonotonicity(t *testing.T) {
	// Volume scores should generally increase with higher volume (except zero case)
	volumes := []float64{0.1, 0.5, 1.0, 2.0, 5.0, 10.0}
	scores := make([]float64, len(volumes))

	for i, vol := range volumes {
		result := scoring.NormalizeVolumeScore(vol)
		scores[i] = result.Score
	}

	// Check monotonic increase
	for i := 1; i < len(scores); i++ {
		if scores[i] <= scores[i-1] {
			t.Errorf("Volume scores not monotonically increasing: %f (vol=%f) <= %f (vol=%f)",
				scores[i], volumes[i], scores[i-1], volumes[i-1])
		}
	}
}
