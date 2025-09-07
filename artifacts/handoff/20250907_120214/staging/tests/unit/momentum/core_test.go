package unit

import (
	"math"
	"testing"

	"cryptorun/internal/domain/regime"
	"github.com/sawpanic/cryptorun/src/domain/momentum"
)

func TestMTW_Normalization(t *testing.T) {
	testCases := []struct {
		name     string
		input    momentum.MTW
		expected float64 // Expected sum after normalization
	}{
		{
			name: "already normalized",
			input: momentum.MTW{
				W1h: 0.20, W4h: 0.35, W12h: 0.30, W24h: 0.15, W7d: 0.00,
			},
			expected: 1.0,
		},
		{
			name: "needs normalization",
			input: momentum.MTW{
				W1h: 20.0, W4h: 35.0, W12h: 30.0, W24h: 15.0, W7d: 0.0,
			},
			expected: 1.0,
		},
		{
			name: "with 7d carry",
			input: momentum.MTW{
				W1h: 0.15, W4h: 0.30, W12h: 0.35, W24h: 0.15, W7d: 0.05,
			},
			expected: 1.0,
		},
		{
			name: "zero weights",
			input: momentum.MTW{
				W1h: 0.0, W4h: 0.0, W12h: 0.0, W24h: 0.0, W7d: 0.0,
			},
			expected: 0.0, // Degenerate case
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			normalized := tc.input.Normalize()
			sum := normalized.Sum()

			if math.Abs(sum-tc.expected) > 1e-10 {
				t.Errorf("expected sum %.10f, got %.10f", tc.expected, sum)
			}

			// All individual weights should be non-negative
			if normalized.W1h < 0 || normalized.W4h < 0 || normalized.W12h < 0 ||
				normalized.W24h < 0 || normalized.W7d < 0 {
				t.Errorf("normalized weights should be non-negative: %+v", normalized)
			}
		})
	}
}

func TestWeightsForRegime(t *testing.T) {
	testCases := []struct {
		regime   regime.RegimeType
		expected struct {
			hasCarry bool
			w4hMin   float64 // 4h should be primary signal
		}
	}{
		{
			regime: regime.TrendingBull,
			expected: struct {
				hasCarry bool
				w4hMin   float64
			}{hasCarry: true, w4hMin: 0.25}, // Should have 7d carry in bull
		},
		{
			regime: regime.Choppy,
			expected: struct {
				hasCarry bool
				w4hMin   float64
			}{hasCarry: false, w4hMin: 0.30}, // No carry in choppy, higher 4h
		},
		{
			regime: regime.HighVol,
			expected: struct {
				hasCarry bool
				w4hMin   float64
			}{hasCarry: false, w4hMin: 0.30}, // No carry in volatility
		},
	}

	for _, tc := range testCases {
		t.Run(string(tc.regime), func(t *testing.T) {
			weights := momentum.WeightsForRegime(tc.regime)

			// Verify normalization
			sum := weights.Sum()
			if math.Abs(sum-1.0) > 1e-10 {
				t.Errorf("weights should sum to 1.0, got %.10f", sum)
			}

			// Verify 7d carry presence/absence
			if tc.expected.hasCarry && weights.W7d == 0 {
				t.Errorf("expected 7d carry for %s regime", tc.regime)
			}
			if !tc.expected.hasCarry && weights.W7d != 0 {
				t.Errorf("expected no 7d carry for %s regime, got %.6f", tc.regime, weights.W7d)
			}

			// Verify 4h is primary signal timeframe
			if weights.W4h < tc.expected.w4hMin {
				t.Errorf("4h weight %.6f below minimum %.6f for %s",
					weights.W4h, tc.expected.w4hMin, tc.regime)
			}

			// All weights should be non-negative
			if weights.W1h < 0 || weights.W4h < 0 || weights.W12h < 0 ||
				weights.W24h < 0 || weights.W7d < 0 {
				t.Errorf("all weights should be non-negative for %s: %+v", tc.regime, weights)
			}
		})
	}
}

func TestComputeCore_ATRNormalization(t *testing.T) {
	inputs := momentum.CoreInputs{
		R1h:     0.02,  // 2% return
		R4h:     0.05,  // 5% return
		R12h:    0.08,  // 8% return
		R24h:    0.12,  // 12% return
		R7d:     0.20,  // 20% return
		ATR1h:   0.01,  // 1% ATR
		ATR4h:   0.02,  // 2% ATR
		Accel4h: 0.001, // Small positive acceleration
	}

	weights := momentum.MTW{W1h: 0.2, W4h: 0.35, W12h: 0.3, W24h: 0.15, W7d: 0.0}

	// Test with ATR normalization
	resultWithATR := momentum.ComputeCore(inputs, weights, false, 0.1, true)

	// Test without ATR normalization
	resultWithoutATR := momentum.ComputeCore(inputs, weights, false, 0.1, false)

	// With ATR normalization, the impact should be different
	if resultWithATR.Score == resultWithoutATR.Score {
		t.Errorf("ATR normalization should affect the result")
	}

	// Both scores should be in valid range
	if resultWithATR.Score < 0 || resultWithATR.Score > 100 {
		t.Errorf("score with ATR norm out of range: %.2f", resultWithATR.Score)
	}
	if resultWithoutATR.Score < 0 || resultWithoutATR.Score > 100 {
		t.Errorf("score without ATR norm out of range: %.2f", resultWithoutATR.Score)
	}
}

func TestComputeCore_AccelerationBoost(t *testing.T) {
	baseInputs := momentum.CoreInputs{
		R1h:   0.01,
		R4h:   0.03, // Positive 4h return
		R12h:  0.05,
		R24h:  0.08,
		R7d:   0.00,
		ATR1h: 0.01,
		ATR4h: 0.02,
	}

	weights := momentum.MTW{W1h: 0.2, W4h: 0.35, W12h: 0.3, W24h: 0.15, W7d: 0.0}

	testCases := []struct {
		name        string
		accel4h     float64
		accelBoost  float64
		expectBoost bool
	}{
		{
			name:        "positive aligned acceleration",
			accel4h:     0.001, // Positive acceleration, aligns with positive R4h
			accelBoost:  0.1,
			expectBoost: true,
		},
		{
			name:        "negative aligned acceleration",
			accel4h:     -0.001, // Negative acceleration, doesn't align with positive R4h
			accelBoost:  0.1,
			expectBoost: false,
		},
		{
			name:        "zero acceleration",
			accel4h:     0.0, // No acceleration
			accelBoost:  0.1,
			expectBoost: false,
		},
		{
			name:        "no acceleration boost",
			accel4h:     0.001, // Positive acceleration
			accelBoost:  0.0,   // But no boost configured
			expectBoost: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			inputs := baseInputs
			inputs.Accel4h = tc.accel4h

			result := momentum.ComputeCore(inputs, weights, false, tc.accelBoost, true)

			if tc.expectBoost {
				if result.Parts.Accel == 0 {
					t.Errorf("expected acceleration boost but got zero")
				}
				// Boost should be positive for positive R4h + positive aligned accel
				if inputs.R4h > 0 && tc.accel4h > 0 && result.Parts.Accel <= 0 {
					t.Errorf("expected positive acceleration boost for aligned signals")
				}
			} else {
				if result.Parts.Accel != 0 {
					t.Errorf("expected no acceleration boost but got %.6f", result.Parts.Accel)
				}
			}
		})
	}
}

func TestComputeCore_ScoreMonotonicity(t *testing.T) {
	baseInputs := momentum.CoreInputs{
		R1h:     0.01,
		R4h:     0.02,
		R12h:    0.03,
		R24h:    0.04,
		R7d:     0.00,
		ATR1h:   0.01,
		ATR4h:   0.02,
		Accel4h: 0.0,
	}

	weights := momentum.MTW{W1h: 0.2, W4h: 0.35, W12h: 0.3, W24h: 0.15, W7d: 0.0}

	// Test monotonicity: increasing each return component should increase score
	testCases := []struct {
		name     string
		modifier func(momentum.CoreInputs) momentum.CoreInputs
	}{
		{
			name: "increase R1h",
			modifier: func(in momentum.CoreInputs) momentum.CoreInputs {
				in.R1h = in.R1h + 0.01
				return in
			},
		},
		{
			name: "increase R4h",
			modifier: func(in momentum.CoreInputs) momentum.CoreInputs {
				in.R4h = in.R4h + 0.01
				return in
			},
		},
		{
			name: "increase R12h",
			modifier: func(in momentum.CoreInputs) momentum.CoreInputs {
				in.R12h = in.R12h + 0.01
				return in
			},
		},
		{
			name: "increase R24h",
			modifier: func(in momentum.CoreInputs) momentum.CoreInputs {
				in.R24h = in.R24h + 0.01
				return in
			},
		},
	}

	baseResult := momentum.ComputeCore(baseInputs, weights, false, 0.0, true)

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			modifiedInputs := tc.modifier(baseInputs)
			modifiedResult := momentum.ComputeCore(modifiedInputs, weights, false, 0.0, true)

			if modifiedResult.Score <= baseResult.Score {
				t.Errorf("increasing %s should increase score: base=%.6f, modified=%.6f",
					tc.name, baseResult.Score, modifiedResult.Score)
			}
		})
	}
}

func TestComputeCore_SevenDayCarry(t *testing.T) {
	inputs := momentum.CoreInputs{
		R1h:     0.01,
		R4h:     0.02,
		R12h:    0.03,
		R24h:    0.04,
		R7d:     0.10, // Significant 7d return
		ATR1h:   0.01,
		ATR4h:   0.02,
		Accel4h: 0.0,
	}

	weights := momentum.MTW{W1h: 0.15, W4h: 0.30, W12h: 0.35, W24h: 0.15, W7d: 0.05}

	// Test with 7d carry enabled (bull regime)
	resultWithCarry := momentum.ComputeCore(inputs, weights, true, 0.0, true)

	// Test with 7d carry disabled (choppy/volatile regime)
	resultWithoutCarry := momentum.ComputeCore(inputs, weights, false, 0.0, true)

	// With carry should have higher score due to positive 7d return
	if resultWithCarry.Score <= resultWithoutCarry.Score {
		t.Errorf("with 7d carry should have higher score: with=%.6f, without=%.6f",
			resultWithCarry.Score, resultWithoutCarry.Score)
	}

	// With carry should have non-zero 7d contribution
	if resultWithCarry.Parts.R7d == 0 {
		t.Errorf("expected non-zero 7d contribution with carry enabled")
	}

	// Without carry should have zero 7d contribution
	if resultWithoutCarry.Parts.R7d != 0 {
		t.Errorf("expected zero 7d contribution with carry disabled, got %.6f",
			resultWithoutCarry.Parts.R7d)
	}
}

func TestValidateInputs(t *testing.T) {
	validInputs := momentum.CoreInputs{
		R1h:     0.01,
		R4h:     0.02,
		R12h:    0.03,
		R24h:    0.04,
		R7d:     0.05,
		ATR1h:   0.01,
		ATR4h:   0.02,
		Accel4h: 0.001,
	}

	// Test valid inputs
	if err := momentum.ValidateInputs(validInputs); err != nil {
		t.Errorf("valid inputs should not produce error: %v", err)
	}

	invalidCases := []struct {
		name        string
		modifier    func(momentum.CoreInputs) momentum.CoreInputs
		expectError bool
	}{
		{
			name: "NaN R1h",
			modifier: func(in momentum.CoreInputs) momentum.CoreInputs {
				in.R1h = math.NaN()
				return in
			},
			expectError: true,
		},
		{
			name: "infinite R4h",
			modifier: func(in momentum.CoreInputs) momentum.CoreInputs {
				in.R4h = math.Inf(1)
				return in
			},
			expectError: true,
		},
		{
			name: "negative ATR1h",
			modifier: func(in momentum.CoreInputs) momentum.CoreInputs {
				in.ATR1h = -0.01
				return in
			},
			expectError: true,
		},
		{
			name: "negative ATR4h",
			modifier: func(in momentum.CoreInputs) momentum.CoreInputs {
				in.ATR4h = -0.02
				return in
			},
			expectError: true,
		},
	}

	for _, tc := range invalidCases {
		t.Run(tc.name, func(t *testing.T) {
			invalidInputs := tc.modifier(validInputs)
			err := momentum.ValidateInputs(invalidInputs)

			if tc.expectError && err == nil {
				t.Errorf("expected error for %s but got none", tc.name)
			}
			if !tc.expectError && err != nil {
				t.Errorf("expected no error for %s but got: %v", tc.name, err)
			}
		})
	}
}

func TestGetMomentumBreakdown(t *testing.T) {
	inputs := momentum.CoreInputs{
		R1h:     0.01,
		R4h:     0.02,
		R12h:    0.03,
		R24h:    0.04,
		R7d:     0.05,
		ATR1h:   0.01,
		ATR4h:   0.02,
		Accel4h: 0.001,
	}

	weights := momentum.MTW{W1h: 0.2, W4h: 0.35, W12h: 0.3, W24h: 0.15, W7d: 0.0}
	result := momentum.ComputeCore(inputs, weights, false, 0.1, true)

	breakdown := momentum.GetMomentumBreakdown(result)

	// Verify breakdown structure
	if _, exists := breakdown["final_score"]; !exists {
		t.Errorf("breakdown missing final_score")
	}

	components, ok := breakdown["components"].(map[string]float64)
	if !ok {
		t.Errorf("breakdown components should be map[string]float64")
	}

	expectedComponents := []string{
		"r1h_contribution", "r4h_contribution", "r12h_contribution",
		"r24h_contribution", "r7d_contribution", "accel_contribution",
	}

	for _, comp := range expectedComponents {
		if _, exists := components[comp]; !exists {
			t.Errorf("breakdown missing component: %s", comp)
		}
	}

	// Verify final score matches
	if finalScore, ok := breakdown["final_score"].(float64); ok {
		if math.Abs(finalScore-result.Score) > 1e-10 {
			t.Errorf("breakdown final_score %.6f != result.Score %.6f", finalScore, result.Score)
		}
	}
}

func TestComputeCore_BoundedOutput(t *testing.T) {
	// Test extreme inputs to verify score bounding
	extremeCases := []struct {
		name   string
		inputs momentum.CoreInputs
	}{
		{
			name: "extreme positive returns",
			inputs: momentum.CoreInputs{
				R1h: 5.0, R4h: 5.0, R12h: 5.0, R24h: 5.0, R7d: 5.0,
				ATR1h: 0.01, ATR4h: 0.02, Accel4h: 1.0,
			},
		},
		{
			name: "extreme negative returns",
			inputs: momentum.CoreInputs{
				R1h: -5.0, R4h: -5.0, R12h: -5.0, R24h: -5.0, R7d: -5.0,
				ATR1h: 0.01, ATR4h: 0.02, Accel4h: -1.0,
			},
		},
		{
			name: "mixed extreme returns",
			inputs: momentum.CoreInputs{
				R1h: 3.0, R4h: -2.0, R12h: 1.5, R24h: -1.0, R7d: 0.5,
				ATR1h: 0.01, ATR4h: 0.02, Accel4h: 0.5,
			},
		},
	}

	weights := momentum.MTW{W1h: 0.2, W4h: 0.35, W12h: 0.3, W24h: 0.15, W7d: 0.0}

	for _, tc := range extremeCases {
		t.Run(tc.name, func(t *testing.T) {
			result := momentum.ComputeCore(tc.inputs, weights, false, 0.1, true)

			// Score must be bounded between 0 and 100
			if result.Score < 0 || result.Score > 100 {
				t.Errorf("score out of bounds [0,100]: %.6f", result.Score)
			}

			// Score should not be NaN or infinite
			if math.IsNaN(result.Score) || math.IsInf(result.Score, 0) {
				t.Errorf("score should not be NaN or infinite: %.6f", result.Score)
			}
		})
	}
}
