package unit

import (
	"math"
	"testing"

	"github.com/sawpanic/cryptorun/internal/domain"
)

func TestRoundBps_HalfUpSemantics(t *testing.T) {
	testCases := []struct {
		name     string
		input    float64
		expected int
	}{
		// Core HALF-UP cases
		{"Exactly 49.5 rounds UP", 49.5, 50},
		{"Exactly 50.5 rounds UP", 50.5, 51},
		{"49.4 rounds DOWN", 49.4, 49},
		{"49.6 rounds UP", 49.6, 50},
		{"Zero", 0.0, 0},
		{"Negative -49.5 rounds DOWN", -49.5, -50}, // HALF-UP for negative
		{"Negative -49.4 rounds UP", -49.4, -49},

		// Edge cases
		{"Very small positive", 0.5, 1},
		{"Very small negative", -0.5, -1},
		{"Large number", 12345.6, 12346},

		// Pathological inputs
		{"NaN input", math.NaN(), 0},
		{"Positive Infinity", math.Inf(1), 0},
		{"Negative Infinity", math.Inf(-1), 0},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := domain.RoundBps(tc.input)
			if result != tc.expected {
				t.Errorf("RoundBps(%.1f) = %d, want %d", tc.input, result, tc.expected)
			}
		})
	}
}

func TestComputeSpreadBps_WithRounding(t *testing.T) {
	testCases := []struct {
		name        string
		bid         float64
		ask         float64
		expectedBps int
	}{
		// Borderline cases for 50 bps threshold
		{"Exactly 50 bps", 100.0, 100.50, 50},
		{"Just under 50 bps", 100.0, 100.4999, 50}, // 49.99 rounds to 50
		{"Just over 50 bps", 100.0, 100.5001, 50},  // 50.01 rounds to 50
		{"Clearly over 50 bps", 100.0, 100.51, 51}, // 51.0 rounds to 51

		// Half-up rounding verification
		{"Calculated 49.38 rounds to 49", 100.0, 100.495, 49}, // (0.495/100.2475)*10000 = 49.38
		{"Calculated 50.37 rounds to 50", 100.0, 100.505, 50}, // (0.505/100.2525)*10000 = 50.37
		{"Tight 5 bps spread", 100.0, 100.05, 5},              // (0.05/100.025)*10000 = 4.998 rounds to 5

		// Invalid inputs return 9999
		{"Bid >= Ask", 100.0, 100.0, 9999},
		{"Negative bid", -100.0, 100.0, 9999},
		{"Zero ask", 100.0, 0.0, 9999},
		{"NaN bid", math.NaN(), 100.0, 9999},
		{"Inf ask", 100.0, math.Inf(1), 9999},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := domain.ComputeSpreadBps(tc.bid, tc.ask)
			if result != tc.expectedBps {
				t.Errorf("ComputeSpreadBps(%.4f, %.4f) = %d, want %d",
					tc.bid, tc.ask, result, tc.expectedBps)
			}
		})
	}
}

func TestSpreadGate_InclusiveThreshold(t *testing.T) {
	thresholds := domain.DefaultMicroGateThresholds()

	testCases := []struct {
		name       string
		bid        float64
		ask        float64
		shouldPass bool
		reason     string
	}{
		// Inclusive threshold testing (50 bps max)
		{"Exactly at 50 bps threshold - PASS", 100.0, 100.50, true, "inclusive boundary"},
		{"49 bps - PASS", 100.0, 100.49, true, "under threshold"},
		{"51 bps - FAIL", 100.0, 100.51, false, "over threshold"},

		// Borderline with rounding
		{"49.38 bps rounds to 49 - PASS", 100.0, 100.495, true, "under boundary after rounding"},
		{"50.37 bps rounds to 50 - PASS", 100.0, 100.505, true, "at boundary after rounding"},

		// Invalid cases
		{"Invalid bid/ask - FAIL", 100.0, 100.0, false, "invalid prices"},
		{"NaN inputs - FAIL", math.NaN(), 100.0, false, "pathological input"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			inputs := domain.MicroGateInputs{
				Symbol:      "TEST",
				Bid:         tc.bid,
				Ask:         tc.ask,
				Depth2PcUSD: 200000, // Above threshold
				VADR:        2.0,    // Above threshold
				ADVUSD:      200000, // Above threshold
			}

			results := domain.EvaluateMicroGates(inputs, thresholds)

			if results.Spread.OK != tc.shouldPass {
				t.Errorf("Spread gate for %s: got OK=%v, want %v (value=%.1f, threshold=%.1f)",
					tc.name, results.Spread.OK, tc.shouldPass,
					results.Spread.Value, results.Spread.Threshold)
			}

			// Verify threshold is exactly 50
			if results.Spread.Threshold != 50.0 {
				t.Errorf("Spread threshold should be 50.0, got %.1f", results.Spread.Threshold)
			}
		})
	}
}

func TestDepthGate_InclusiveThreshold(t *testing.T) {
	thresholds := domain.DefaultMicroGateThresholds() // $100k min

	testCases := []struct {
		name       string
		depth      float64
		shouldPass bool
		reason     string
	}{
		// Inclusive threshold testing ($100k min)
		{"Exactly at $100k threshold - PASS", 100000.0, true, "inclusive boundary"},
		{"$99999 - FAIL", 99999.0, false, "under threshold by $1"},
		{"$100001 - PASS", 100001.0, true, "over threshold"},
		{"$200k - PASS", 200000.0, true, "well over threshold"},

		// Rounding cases
		{"$99999.9 rounds to $100000 - PASS", 99999.9, true, "rounds up to threshold"},
		{"$99999.4 rounds to $99999 - FAIL", 99999.4, false, "rounds down under threshold"},

		// Edge cases
		{"Zero depth - FAIL", 0.0, false, "no liquidity"},
		{"Negative depth - FAIL", -1000.0, false, "invalid depth"},

		// Pathological inputs
		{"NaN depth - FAIL", math.NaN(), false, "pathological input"},
		{"Infinite depth - FAIL", math.Inf(1), false, "pathological input"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			inputs := domain.MicroGateInputs{
				Symbol:      "TEST",
				Bid:         100.0,
				Ask:         100.25, // 25 bps spread, under threshold
				Depth2PcUSD: tc.depth,
				VADR:        2.0,    // Above threshold
				ADVUSD:      200000, // Above threshold
			}

			results := domain.EvaluateMicroGates(inputs, thresholds)

			if results.Depth.OK != tc.shouldPass {
				t.Errorf("Depth gate for %s: got OK=%v, want %v (value=%.0f, threshold=%.0f)",
					tc.name, results.Depth.OK, tc.shouldPass,
					results.Depth.Value, results.Depth.Threshold)
			}

			// Verify rounding to nearest USD
			expectedValue := math.Round(tc.depth)
			if !math.IsNaN(tc.depth) && !math.IsInf(tc.depth, 0) {
				if tc.depth < 0 {
					expectedValue = 0 // Negative values guarded to 0
				}
				if results.Depth.Value != expectedValue {
					t.Errorf("Depth value should be rounded to %.0f, got %.0f", expectedValue, results.Depth.Value)
				}
			}
		})
	}
}

func TestDepth2pcUSD_PrecisionSemantics(t *testing.T) {
	testCases := []struct {
		name        string
		bid         float64
		ask         float64
		bidSizes    []float64
		askSizes    []float64
		bidPrices   []float64
		askPrices   []float64
		expected    float64
		description string
	}{
		{
			name:        "Simple case within ±2%",
			bid:         100.0,
			ask:         100.20,
			bidSizes:    []float64{1000.0, 500.0},
			bidPrices:   []float64{100.0, 99.5}, // 99.5 is within ±2% of mid=100.1
			askSizes:    []float64{800.0, 300.0},
			askPrices:   []float64{100.20, 100.8},                                   // 100.8 is within ±2% of mid=100.1
			expected:    math.Round(1000*100.0 + 500*99.5 + 800*100.20 + 300*100.8), // All within bounds
			description: "All levels within ±2%",
		},
		{
			name:        "Some levels outside ±2%",
			bid:         100.0,
			ask:         100.20, // mid = 100.1, bounds = [98.098, 102.102]
			bidSizes:    []float64{1000.0, 500.0},
			bidPrices:   []float64{100.0, 97.0}, // 97.0 is outside -2%
			askSizes:    []float64{800.0, 300.0},
			askPrices:   []float64{100.20, 103.0},            // 103.0 is outside +2%
			expected:    math.Round(1000*100.0 + 800*100.20), // Only levels within bounds
			description: "Excludes levels outside ±2%",
		},
		{
			name:        "Rounding precision test",
			bid:         100.0,
			ask:         100.20,
			bidSizes:    []float64{0.333},
			bidPrices:   []float64{100.0},
			askSizes:    []float64{0.0},
			askPrices:   []float64{},
			expected:    math.Round(math.Round(0.333*100.0*100) / 100), // Round to cent, then to USD
			description: "Precise cent rounding",
		},
		{
			name:        "Invalid inputs return zero",
			bid:         math.NaN(),
			ask:         100.0,
			bidSizes:    []float64{1000.0},
			bidPrices:   []float64{100.0},
			askSizes:    []float64{},
			askPrices:   []float64{},
			expected:    0.0,
			description: "NaN inputs fail safe",
		},
		{
			name:        "Mismatched arrays return zero",
			bid:         100.0,
			ask:         100.20,
			bidSizes:    []float64{1000.0, 500.0}, // 2 elements
			bidPrices:   []float64{100.0},         // 1 element - mismatch
			askSizes:    []float64{},
			askPrices:   []float64{},
			expected:    0.0,
			description: "Array length mismatch fail safe",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := domain.Depth2pcUSD(tc.bid, tc.ask, tc.bidSizes, tc.askSizes, tc.bidPrices, tc.askPrices)

			if math.IsNaN(tc.expected) {
				if !math.IsNaN(result) {
					t.Errorf("Expected NaN, got %.2f", result)
				}
			} else if math.Abs(result-tc.expected) > 0.01 { // Allow small rounding differences
				t.Errorf("Depth2pcUSD = %.2f, want %.2f (%s)", result, tc.expected, tc.description)
			}
		})
	}
}

func TestVADRGate_NaNGuarding(t *testing.T) {
	thresholds := domain.DefaultMicroGateThresholds() // 1.75 min VADR

	testCases := []struct {
		name       string
		vadr       float64
		shouldPass bool
	}{
		{"Valid VADR above threshold", 2.0, true},
		{"Valid VADR at threshold", 1.75, true},
		{"Valid VADR below threshold", 1.0, false},
		{"NaN VADR fails safe", math.NaN(), false},
		{"Positive Inf VADR fails safe", math.Inf(1), false},
		{"Negative Inf VADR fails safe", math.Inf(-1), false},
		{"Zero VADR", 0.0, false},
		{"Negative VADR", -1.0, false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			inputs := domain.MicroGateInputs{
				Symbol:      "TEST",
				Bid:         100.0,
				Ask:         100.25, // 25 bps spread
				Depth2PcUSD: 200000, // Above threshold
				VADR:        tc.vadr,
				ADVUSD:      200000, // Above threshold
			}

			results := domain.EvaluateMicroGates(inputs, thresholds)

			if results.VADR.OK != tc.shouldPass {
				t.Errorf("VADR gate for %s: got OK=%v, want %v (value=%.3f)",
					tc.name, results.VADR.OK, tc.shouldPass, results.VADR.Value)
			}

			// Verify NaN/Inf inputs are converted to 0.0
			if math.IsNaN(tc.vadr) || math.IsInf(tc.vadr, 0) {
				if results.VADR.Value != 0.0 {
					t.Errorf("Pathological VADR input should result in 0.0 value, got %.3f", results.VADR.Value)
				}
			}
		})
	}
}

func TestGuardFinite_FailSafe(t *testing.T) {
	testCases := []struct {
		name     string
		value    float64
		fallback float64
		expected float64
	}{
		{"Normal value passes through", 42.5, 0.0, 42.5},
		{"NaN uses fallback", math.NaN(), -1.0, -1.0},
		{"Positive Inf uses fallback", math.Inf(1), 100.0, 100.0},
		{"Negative Inf uses fallback", math.Inf(-1), 200.0, 200.0},
		{"Zero value passes through", 0.0, 999.0, 0.0},
		{"Negative value passes through", -42.5, 0.0, -42.5},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := domain.GuardFinite(tc.value, tc.fallback)

			if math.IsNaN(tc.expected) {
				if !math.IsNaN(result) {
					t.Errorf("Expected NaN, got %.2f", result)
				}
			} else if result != tc.expected {
				t.Errorf("GuardFinite(%.2f, %.2f) = %.2f, want %.2f",
					tc.value, tc.fallback, result, tc.expected)
			}
		})
	}
}

func TestGuardPositive_FailSafe(t *testing.T) {
	testCases := []struct {
		name     string
		value    float64
		fallback float64
		expected float64
	}{
		{"Positive value passes through", 42.5, 0.0, 42.5},
		{"Zero uses fallback", 0.0, 100.0, 100.0},
		{"Negative uses fallback", -10.0, 50.0, 50.0},
		{"NaN uses fallback", math.NaN(), 25.0, 25.0},
		{"Positive Inf uses fallback", math.Inf(1), 75.0, 75.0},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := domain.GuardPositive(tc.value, tc.fallback)

			if result != tc.expected {
				t.Errorf("GuardPositive(%.2f, %.2f) = %.2f, want %.2f",
					tc.value, tc.fallback, result, tc.expected)
			}
		})
	}
}
