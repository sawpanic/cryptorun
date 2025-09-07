package unit

import (
	"fmt"
	"math"
	"testing"

	"cryptorun/internal/domain/scoring"
)

func TestNormalizeVolatilityScore(t *testing.T) {
	testCases := []struct {
		name            string
		volatility      float64
		expectedMin     float64 // Minimum expected score
		expectedMax     float64 // Maximum expected score
		expectCapped    bool
	}{
		{
			name:        "Optimal volatility low",
			volatility:  15.0,
			expectedMin: 99.0,
			expectedMax: 100.0,
			expectCapped: false,
		},
		{
			name:        "Optimal volatility mid",
			volatility:  20.0,
			expectedMin: 99.0,
			expectedMax: 100.0,
			expectCapped: false,
		},
		{
			name:        "Optimal volatility high",
			volatility:  25.0,
			expectedMin: 99.0,
			expectedMax: 100.0,
			expectCapped: false,
		},
		{
			name:        "Low volatility",
			volatility:  5.0,
			expectedMin: 30.0,
			expectedMax: 40.0,
			expectCapped: false,
		},
		{
			name:        "High volatility", // This is the failing test case
			volatility:  50.0,
			expectedMin: 0.0,   // Should get low score
			expectedMax: 40.0,  // Should be well below 50
			expectCapped: false,
		},
		{
			name:        "Very high volatility",
			volatility:  100.0,
			expectedMin: 0.0,
			expectedMax: 30.0,
			expectCapped: true, // Should be capped
		},
		{
			name:        "Zero volatility",
			volatility:  0.0,
			expectedMin: 0.0,
			expectedMax: 10.0,
			expectCapped: false,
		},
		{
			name:        "NaN volatility",
			volatility:  math.NaN(),
			expectedMin: 49.0,
			expectedMax: 51.0,
			expectCapped: false,
		},
		{
			name:        "Positive infinity",
			volatility:  math.Inf(1),
			expectedMin: 49.0,
			expectedMax: 51.0,
			expectCapped: false,
		},
		{
			name:        "Negative volatility",
			volatility:  -20.0, // Should use absolute value
			expectedMin: 99.0,
			expectedMax: 100.0,
			expectCapped: false,
		},
		{
			name:        "Extremely high volatility",
			volatility:  500.0,
			expectedMin: 0.0,
			expectedMax: 26.0, // Slightly more tolerance
			expectCapped: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := scoring.NormalizeVolatilityScore(tc.volatility)

			// Check score is in expected range
			if result.Score < tc.expectedMin || result.Score > tc.expectedMax {
				t.Errorf("Score out of expected range: got %f, want %f-%f", 
					result.Score, tc.expectedMin, tc.expectedMax)
			}

			// Check capping flag
			if result.Capped != tc.expectCapped {
				t.Errorf("Capping flag mismatch: got %v, want %v", 
					result.Capped, tc.expectCapped)
			}

			// Check original value is preserved
			if !math.IsNaN(tc.volatility) && !math.IsInf(tc.volatility, 0) {
				if result.OriginalValue != tc.volatility {
					t.Errorf("Original value not preserved: got %f, want %f", 
						result.OriginalValue, tc.volatility)
				}
			}

			// Score should always be in valid range
			if result.Score < 0.0 || result.Score > 100.0 {
				t.Errorf("Score out of range: %f", result.Score)
			}
		})
	}
}

func TestVolatilityScoreRange(t *testing.T) {
	// Test a range of volatilities to ensure no score explosions
	volatilities := []float64{0.0, 1.0, 5.0, 10.0, 15.0, 20.0, 25.0, 30.0, 40.0, 50.0, 75.0, 100.0, 200.0, 500.0}
	
	for _, vol := range volatilities {
		t.Run("Volatility_"+fmt.Sprintf("%.1f", vol), func(t *testing.T) {
			result := scoring.NormalizeVolatilityScore(vol)
			
			if result.Score < 0.0 || result.Score > 100.0 {
				t.Errorf("Volatility %f produced out-of-range score: %f", vol, result.Score)
			}
		})
	}
}

func TestVolatilityOptimalRange(t *testing.T) {
	// Test that optimal range (15-25) gets high scores
	optimalVolatilities := []float64{15.0, 17.5, 20.0, 22.5, 25.0}
	
	for _, vol := range optimalVolatilities {
		t.Run("Optimal_"+fmt.Sprintf("%.1f", vol), func(t *testing.T) {
			result := scoring.NormalizeVolatilityScore(vol)
			
			if result.Score < 95.0 {
				t.Errorf("Optimal volatility %f should get high score, got %f", vol, result.Score)
			}
		})
	}
}

func TestVolatilityHighPenalty(t *testing.T) {
	// Test that high volatilities get significantly penalized
	highVolatilities := []float64{40.0, 50.0, 75.0, 100.0}
	
	for _, vol := range highVolatilities {
		t.Run("High_"+fmt.Sprintf("%.1f", vol), func(t *testing.T) {
			result := scoring.NormalizeVolatilityScore(vol)
			
			// High volatility should get low scores
			if result.Score > 40.0 {
				t.Errorf("High volatility %f should get low score, got %f", vol, result.Score)
			}
		})
	}
}

func TestVolatilityCappingBehavior(t *testing.T) {
	// Test that extreme values are capped but still produce valid scores
	extremeVols := []float64{150.0, 300.0, 1000.0, 10000.0}
	
	for _, vol := range extremeVols {
		t.Run("Extreme_"+fmt.Sprintf("%.0f", vol), func(t *testing.T) {
			result := scoring.NormalizeVolatilityScore(vol)
			
			if !result.Capped {
				t.Errorf("Extreme volatility %f should be capped", vol)
			}
			
			if result.Score < 0.0 || result.Score > 100.0 {
				t.Errorf("Capped volatility %f should still produce valid score, got %f", vol, result.Score)
			}
			
			// Extreme values should get very low scores
			if result.Score > 30.0 {
				t.Errorf("Extreme volatility %f should get very low score, got %f", vol, result.Score)
			}
		})
	}
}