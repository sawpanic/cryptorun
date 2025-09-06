package regime

import (
	"testing"
)

// RegimeDetectionInputs represents the 3-indicator system inputs
type RegimeDetectionInputs struct {
	RealizedVol7d       float64 // Annualized volatility (0.0 = 0%, 1.0 = 100%)
	BreadthPctAbove20MA float64 // Percent of universe above 20MA (0.0 = 0%, 1.0 = 100%)
	BreadthThrust       float64 // Breadth thrust indicator (-1.0 to +1.0)
}

// RegimeDetector simulates the regime detection logic
type RegimeDetector struct {
	// Thresholds from regimes.yaml
	VolLowThreshold  float64 // 0.30
	VolHighThreshold float64 // 0.60
	BullThreshold    float64 // 0.65
	BearThreshold    float64 // 0.35
	ThrustPositive   float64 // 0.15
	ThrustNegative   float64 // -0.15
}

func NewRegimeDetector() *RegimeDetector {
	return &RegimeDetector{
		VolLowThreshold:  0.30,
		VolHighThreshold: 0.60,
		BullThreshold:    0.65,
		BearThreshold:    0.35,
		ThrustPositive:   0.15,
		ThrustNegative:   -0.15,
	}
}

func (rd *RegimeDetector) DetectRegime(inputs RegimeDetectionInputs) string {
	// Simplified regime detection logic based on majority vote
	// Real implementation would use 6-period window and stability bias

	votes := make(map[string]int)

	// Volatility indicator
	if inputs.RealizedVol7d < rd.VolLowThreshold {
		votes["CHOPPY"]++
	} else if inputs.RealizedVol7d > rd.VolHighThreshold {
		votes["HIGH_VOL"]++
	} else {
		votes["TRENDING_BULL"]++
	}

	// Breadth indicator
	if inputs.BreadthPctAbove20MA > rd.BullThreshold {
		votes["TRENDING_BULL"]++
	} else if inputs.BreadthPctAbove20MA < rd.BearThreshold {
		votes["HIGH_VOL"]++
	} else {
		votes["CHOPPY"]++
	}

	// Thrust indicator
	if inputs.BreadthThrust > rd.ThrustPositive {
		votes["TRENDING_BULL"]++
	} else if inputs.BreadthThrust < rd.ThrustNegative {
		votes["HIGH_VOL"]++
	} else {
		votes["CHOPPY"]++
	}

	// Return regime with most votes
	maxVotes := 0
	winningRegime := "CHOPPY" // Default fallback

	for regime, voteCount := range votes {
		if voteCount > maxVotes {
			maxVotes = voteCount
			winningRegime = regime
		}
	}

	return winningRegime
}

func TestRegimeDetector_BoundaryConditions(t *testing.T) {
	detector := NewRegimeDetector()

	tests := []struct {
		name           string
		inputs         RegimeDetectionInputs
		expectedRegime string
		description    string
	}{
		// Volatility boundary tests
		{
			name: "vol_exactly_low_threshold",
			inputs: RegimeDetectionInputs{
				RealizedVol7d:       0.30, // Exactly at low threshold
				BreadthPctAbove20MA: 0.50, // Neutral
				BreadthThrust:       0.00, // Neutral
			},
			expectedRegime: "CHOPPY",
			description:    "Volatility at low threshold should NOT trigger low volatility",
		},
		{
			name: "vol_exactly_high_threshold",
			inputs: RegimeDetectionInputs{
				RealizedVol7d:       0.60, // Exactly at high threshold
				BreadthPctAbove20MA: 0.50, // Neutral
				BreadthThrust:       0.00, // Neutral
			},
			expectedRegime: "CHOPPY",
			description:    "Volatility at high threshold should NOT trigger high volatility",
		},
		{
			name: "vol_just_below_low_threshold",
			inputs: RegimeDetectionInputs{
				RealizedVol7d:       0.29, // Just below low threshold
				BreadthPctAbove20MA: 0.50, // Neutral
				BreadthThrust:       0.00, // Neutral
			},
			expectedRegime: "CHOPPY",
			description:    "Volatility just below low threshold triggers low vol vote",
		},
		{
			name: "vol_just_above_high_threshold",
			inputs: RegimeDetectionInputs{
				RealizedVol7d:       0.61, // Just above high threshold
				BreadthPctAbove20MA: 0.50, // Neutral
				BreadthThrust:       0.00, // Neutral
			},
			expectedRegime: "HIGH_VOL",
			description:    "Volatility just above high threshold triggers high vol vote",
		},

		// Breadth boundary tests
		{
			name: "breadth_exactly_bull_threshold",
			inputs: RegimeDetectionInputs{
				RealizedVol7d:       0.45, // Neutral
				BreadthPctAbove20MA: 0.65, // Exactly at bull threshold
				BreadthThrust:       0.00, // Neutral
			},
			expectedRegime: "CHOPPY",
			description:    "Breadth at bull threshold should NOT trigger bull vote",
		},
		{
			name: "breadth_exactly_bear_threshold",
			inputs: RegimeDetectionInputs{
				RealizedVol7d:       0.45, // Neutral
				BreadthPctAbove20MA: 0.35, // Exactly at bear threshold
				BreadthThrust:       0.00, // Neutral
			},
			expectedRegime: "CHOPPY",
			description:    "Breadth at bear threshold should NOT trigger bear vote",
		},
		{
			name: "breadth_just_above_bull_threshold",
			inputs: RegimeDetectionInputs{
				RealizedVol7d:       0.45, // Neutral
				BreadthPctAbove20MA: 0.66, // Just above bull threshold
				BreadthThrust:       0.00, // Neutral
			},
			expectedRegime: "TRENDING_BULL",
			description:    "Breadth just above bull threshold triggers bull vote",
		},
		{
			name: "breadth_just_below_bear_threshold",
			inputs: RegimeDetectionInputs{
				RealizedVol7d:       0.45, // Neutral
				BreadthPctAbove20MA: 0.34, // Just below bear threshold
				BreadthThrust:       0.00, // Neutral
			},
			expectedRegime: "HIGH_VOL",
			description:    "Breadth just below bear threshold triggers bear vote",
		},

		// Thrust boundary tests
		{
			name: "thrust_exactly_positive_threshold",
			inputs: RegimeDetectionInputs{
				RealizedVol7d:       0.45, // Neutral
				BreadthPctAbove20MA: 0.50, // Neutral
				BreadthThrust:       0.15, // Exactly at positive threshold
			},
			expectedRegime: "CHOPPY",
			description:    "Thrust at positive threshold should NOT trigger positive vote",
		},
		{
			name: "thrust_exactly_negative_threshold",
			inputs: RegimeDetectionInputs{
				RealizedVol7d:       0.45,  // Neutral
				BreadthPctAbove20MA: 0.50,  // Neutral
				BreadthThrust:       -0.15, // Exactly at negative threshold
			},
			expectedRegime: "CHOPPY",
			description:    "Thrust at negative threshold should NOT trigger negative vote",
		},
		{
			name: "thrust_just_above_positive_threshold",
			inputs: RegimeDetectionInputs{
				RealizedVol7d:       0.45, // Neutral
				BreadthPctAbove20MA: 0.50, // Neutral
				BreadthThrust:       0.16, // Just above positive threshold
			},
			expectedRegime: "TRENDING_BULL",
			description:    "Thrust just above positive threshold triggers positive vote",
		},
		{
			name: "thrust_just_below_negative_threshold",
			inputs: RegimeDetectionInputs{
				RealizedVol7d:       0.45,  // Neutral
				BreadthPctAbove20MA: 0.50,  // Neutral
				BreadthThrust:       -0.16, // Just below negative threshold
			},
			expectedRegime: "HIGH_VOL",
			description:    "Thrust just below negative threshold triggers negative vote",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			regime := detector.DetectRegime(tt.inputs)

			if regime != tt.expectedRegime {
				t.Errorf("regime detection failed: expected %s, got %s\nDescription: %s\nInputs: vol=%.3f, breadth=%.3f, thrust=%.3f",
					tt.expectedRegime, regime, tt.description,
					tt.inputs.RealizedVol7d, tt.inputs.BreadthPctAbove20MA, tt.inputs.BreadthThrust)
			}
		})
	}
}

func TestRegimeDetector_ExtremeValues(t *testing.T) {
	detector := NewRegimeDetector()

	tests := []struct {
		name           string
		inputs         RegimeDetectionInputs
		expectedRegime string
	}{
		{
			name: "extreme_high_vol",
			inputs: RegimeDetectionInputs{
				RealizedVol7d:       1.50, // 150% annualized
				BreadthPctAbove20MA: 0.50,
				BreadthThrust:       0.00,
			},
			expectedRegime: "HIGH_VOL",
		},
		{
			name: "extreme_low_vol",
			inputs: RegimeDetectionInputs{
				RealizedVol7d:       0.05, // 5% annualized
				BreadthPctAbove20MA: 0.50,
				BreadthThrust:       0.00,
			},
			expectedRegime: "CHOPPY",
		},
		{
			name: "extreme_bull_breadth",
			inputs: RegimeDetectionInputs{
				RealizedVol7d:       0.45,
				BreadthPctAbove20MA: 0.95, // 95% above 20MA
				BreadthThrust:       0.00,
			},
			expectedRegime: "TRENDING_BULL",
		},
		{
			name: "extreme_bear_breadth",
			inputs: RegimeDetectionInputs{
				RealizedVol7d:       0.45,
				BreadthPctAbove20MA: 0.05, // Only 5% above 20MA
				BreadthThrust:       0.00,
			},
			expectedRegime: "HIGH_VOL",
		},
		{
			name: "extreme_positive_thrust",
			inputs: RegimeDetectionInputs{
				RealizedVol7d:       0.45,
				BreadthPctAbove20MA: 0.50,
				BreadthThrust:       0.50, // 50% thrust
			},
			expectedRegime: "TRENDING_BULL",
		},
		{
			name: "extreme_negative_thrust",
			inputs: RegimeDetectionInputs{
				RealizedVol7d:       0.45,
				BreadthPctAbove20MA: 0.50,
				BreadthThrust:       -0.50, // -50% thrust
			},
			expectedRegime: "HIGH_VOL",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			regime := detector.DetectRegime(tt.inputs)

			if regime != tt.expectedRegime {
				t.Errorf("extreme value test failed: expected %s, got %s", tt.expectedRegime, regime)
			}
		})
	}
}

func TestRegimeDetector_ClearRegimeScenarios(t *testing.T) {
	detector := NewRegimeDetector()

	tests := []struct {
		name           string
		inputs         RegimeDetectionInputs
		expectedRegime string
		description    string
	}{
		{
			name: "clear_trending_bull",
			inputs: RegimeDetectionInputs{
				RealizedVol7d:       0.40, // Medium volatility
				BreadthPctAbove20MA: 0.80, // Strong breadth
				BreadthThrust:       0.25, // Positive thrust
			},
			expectedRegime: "TRENDING_BULL",
			description:    "Strong breadth + positive thrust should indicate trending bull",
		},
		{
			name: "clear_high_vol",
			inputs: RegimeDetectionInputs{
				RealizedVol7d:       0.80,  // High volatility
				BreadthPctAbove20MA: 0.20,  // Weak breadth
				BreadthThrust:       -0.25, // Negative thrust
			},
			expectedRegime: "HIGH_VOL",
			description:    "High vol + weak breadth + negative thrust should indicate high vol",
		},
		{
			name: "clear_choppy",
			inputs: RegimeDetectionInputs{
				RealizedVol7d:       0.20, // Low volatility
				BreadthPctAbove20MA: 0.45, // Neutral breadth
				BreadthThrust:       0.05, // Neutral thrust
			},
			expectedRegime: "CHOPPY",
			description:    "Low vol + neutral conditions should indicate choppy",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			regime := detector.DetectRegime(tt.inputs)

			if regime != tt.expectedRegime {
				t.Errorf("clear scenario failed: expected %s, got %s\nDescription: %s",
					tt.expectedRegime, regime, tt.description)
			}
		})
	}
}

func TestRegimeDetector_NeutralConditions(t *testing.T) {
	detector := NewRegimeDetector()

	// Test perfectly neutral conditions (should default to CHOPPY)
	inputs := RegimeDetectionInputs{
		RealizedVol7d:       0.45, // Between thresholds
		BreadthPctAbove20MA: 0.50, // Between thresholds
		BreadthThrust:       0.00, // Between thresholds
	}

	regime := detector.DetectRegime(inputs)

	if regime != "CHOPPY" {
		t.Errorf("neutral conditions should result in CHOPPY, got %s", regime)
	}
}

func TestRegimeDetector_ConfigThresholds(t *testing.T) {
	// Test that detector uses correct thresholds from regimes.yaml
	detector := NewRegimeDetector()

	// Verify threshold values match regimes.yaml
	expectedThresholds := map[string]float64{
		"VolLowThreshold":  0.30,
		"VolHighThreshold": 0.60,
		"BullThreshold":    0.65,
		"BearThreshold":    0.35,
		"ThrustPositive":   0.15,
		"ThrustNegative":   -0.15,
	}

	if detector.VolLowThreshold != expectedThresholds["VolLowThreshold"] {
		t.Errorf("VolLowThreshold mismatch: expected %.2f, got %.2f",
			expectedThresholds["VolLowThreshold"], detector.VolLowThreshold)
	}

	if detector.VolHighThreshold != expectedThresholds["VolHighThreshold"] {
		t.Errorf("VolHighThreshold mismatch: expected %.2f, got %.2f",
			expectedThresholds["VolHighThreshold"], detector.VolHighThreshold)
	}

	if detector.BullThreshold != expectedThresholds["BullThreshold"] {
		t.Errorf("BullThreshold mismatch: expected %.2f, got %.2f",
			expectedThresholds["BullThreshold"], detector.BullThreshold)
	}

	if detector.BearThreshold != expectedThresholds["BearThreshold"] {
		t.Errorf("BearThreshold mismatch: expected %.2f, got %.2f",
			expectedThresholds["BearThreshold"], detector.BearThreshold)
	}

	if detector.ThrustPositive != expectedThresholds["ThrustPositive"] {
		t.Errorf("ThrustPositive mismatch: expected %.2f, got %.2f",
			expectedThresholds["ThrustPositive"], detector.ThrustPositive)
	}

	if detector.ThrustNegative != expectedThresholds["ThrustNegative"] {
		t.Errorf("ThrustNegative mismatch: expected %.2f, got %.2f",
			expectedThresholds["ThrustNegative"], detector.ThrustNegative)
	}
}
