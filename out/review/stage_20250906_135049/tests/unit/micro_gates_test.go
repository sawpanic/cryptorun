package unit

import (
	"math"
	"testing"

	"cryptorun/internal/domain"
)

func TestSpreadGateBorderlineCases(t *testing.T) {
	testCases := []struct {
		name          string
		bid           float64
		ask           float64
		maxSpreadBps  float64
		expectedOK    bool
		expectedValue float64
		expectedName  string
	}{
		{
			name:          "Exactly at threshold",
			bid:           100.0,
			ask:           100.50,  // 50 bps spread
			maxSpreadBps:  50.0,
			expectedOK:    true,
			expectedValue: 50.0,
			expectedName:  "spread",
		},
		{
			name:          "Just above threshold by 0.01 bps",
			bid:           100.0,
			ask:           100.5001, // 50.01 bps spread -> rounds to 50
			maxSpreadBps:  50.0,
			expectedOK:    true,  // 50.01 rounds to 50, which passes
			expectedValue: 50.0,
			expectedName:  "spread",
		},
		{
			name:          "Just below threshold",
			bid:           100.0,
			ask:           100.4999, // 49.99 bps spread -> rounds to 50
			maxSpreadBps:  50.0,
			expectedOK:    true,
			expectedValue: 50.0,
			expectedName:  "spread",
		},
		{
			name:          "Very tight spread",
			bid:           50000.0,
			ask:           50000.05, // 1 bps spread -> rounds to 0
			maxSpreadBps:  50.0,
			expectedOK:    true,
			expectedValue: 0.0,
			expectedName:  "spread",
		},
		{
			name:          "Wide spread",
			bid:           100.0,
			ask:           105.0,    // 476.19 bps spread -> rounds to 488
			maxSpreadBps:  50.0,
			expectedOK:    false,
			expectedValue: 488.0,
			expectedName:  "spread",
		},
		{
			name:          "Invalid bid/ask - bid > ask",
			bid:           100.0,
			ask:           99.0,
			maxSpreadBps:  50.0,
			expectedOK:    false,
			expectedValue: 9999.0, // Invalid cases return 9999
			expectedName:  "spread_invalid",
		},
		{
			name:          "Invalid bid/ask - equal",
			bid:           100.0,
			ask:           100.0,
			maxSpreadBps:  50.0,
			expectedOK:    false,
			expectedValue: 9999.0,
			expectedName:  "spread_invalid",
		},
		{
			name:          "Invalid bid/ask - zero bid",
			bid:           0.0,
			ask:           100.0,
			maxSpreadBps:  50.0,
			expectedOK:    false,
			expectedValue: 9999.0,
			expectedName:  "spread_invalid",
		},
		{
			name:          "Invalid bid/ask - negative ask",
			bid:           100.0,
			ask:           -1.0,
			maxSpreadBps:  50.0,
			expectedOK:    false,
			expectedValue: 9999.0,
			expectedName:  "spread_invalid",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			thresholds := domain.MicroGateThresholds{MaxSpreadBps: tc.maxSpreadBps}
			inputs := domain.MicroGateInputs{Bid: tc.bid, Ask: tc.ask}
			results := domain.EvaluateMicroGates(inputs, thresholds)
			result := results.Spread

			if result.OK != tc.expectedOK {
				t.Errorf("OK mismatch: got %v, want %v", result.OK, tc.expectedOK)
			}

			if math.Abs(result.Value-tc.expectedValue) > 0.01 {
				t.Errorf("Value mismatch: got %f, want %f", result.Value, tc.expectedValue)
			}

			if result.Threshold != tc.maxSpreadBps {
				t.Errorf("Threshold mismatch: got %f, want %f", result.Threshold, tc.maxSpreadBps)
			}

			if result.Name != tc.expectedName {
				t.Errorf("Name mismatch: got %s, want %s", result.Name, tc.expectedName)
			}
		})
	}
}

func TestDepthGateBorderlineCases(t *testing.T) {
	testCases := []struct {
		name          string
		depth2PcUSD   float64
		minDepthUSD   float64
		expectedOK    bool
		expectedValue float64
	}{
		{
			name:          "Exactly at threshold",
			depth2PcUSD:   100000.0,
			minDepthUSD:   100000.0,
			expectedOK:    true,
			expectedValue: 100000.0,
		},
		{
			name:          "Just above threshold",
			depth2PcUSD:   100000.01,
			minDepthUSD:   100000.0,
			expectedOK:    true,
			expectedValue: 100000.0, // Rounded to nearest USD
		},
		{
			name:          "Just below threshold",
			depth2PcUSD:   99999.99,
			minDepthUSD:   100000.0,
			expectedOK:    true,      // 99999.99 rounds to 100000
			expectedValue: 100000.0,  // Rounded to nearest USD
		},
		{
			name:          "Zero depth",
			depth2PcUSD:   0.0,
			minDepthUSD:   100000.0,
			expectedOK:    false,
			expectedValue: 0.0,
		},
		{
			name:          "Negative depth",
			depth2PcUSD:   -1000.0,
			minDepthUSD:   100000.0,
			expectedOK:    false,
			expectedValue: 0.0, // Negative values guarded to 0
		},
		{
			name:          "Very high depth",
			depth2PcUSD:   10000000.0,
			minDepthUSD:   100000.0,
			expectedOK:    true,
			expectedValue: 10000000.0,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			thresholds := domain.MicroGateThresholds{MinDepthUSD: tc.minDepthUSD}
			inputs := domain.MicroGateInputs{Depth2PcUSD: tc.depth2PcUSD}
			results := domain.EvaluateMicroGates(inputs, thresholds)
			result := results.Depth

			if result.OK != tc.expectedOK {
				t.Errorf("OK mismatch: got %v, want %v", result.OK, tc.expectedOK)
			}

			if math.Abs(result.Value-tc.expectedValue) > 0.01 {
				t.Errorf("Value mismatch: got %f, want %f", result.Value, tc.expectedValue)
			}

			if result.Threshold != tc.minDepthUSD {
				t.Errorf("Threshold mismatch: got %f, want %f", result.Threshold, tc.minDepthUSD)
			}

			if result.Name != "depth" {
				t.Errorf("Name mismatch: got %s, want depth", result.Name)
			}
		})
	}
}

func TestVADRGateBorderlineCases(t *testing.T) {
	testCases := []struct {
		name        string
		vadr        float64
		minVADR     float64
		expectedOK  bool
	}{
		{
			name:       "Exactly at threshold",
			vadr:       1.75,
			minVADR:    1.75,
			expectedOK: true,
		},
		{
			name:       "Just above threshold",
			vadr:       1.751,
			minVADR:    1.75,
			expectedOK: true,
		},
		{
			name:       "Just below threshold",
			vadr:       1.749,
			minVADR:    1.75,
			expectedOK: false,
		},
		{
			name:       "Zero VADR",
			vadr:       0.0,
			minVADR:    1.75,
			expectedOK: false,
		},
		{
			name:       "Negative VADR",
			vadr:       -0.5,
			minVADR:    1.75,
			expectedOK: false,
		},
		{
			name:       "Very high VADR",
			vadr:       10.0,
			minVADR:    1.75,
			expectedOK: true,
		},
		{
			name:       "Fractional precision",
			vadr:       1.7505,
			minVADR:    1.75,
			expectedOK: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			thresholds := domain.MicroGateThresholds{MinVADR: tc.minVADR}
		inputs := domain.MicroGateInputs{VADR: tc.vadr}
		results := domain.EvaluateMicroGates(inputs, thresholds)
		result := results.VADR

			if result.OK != tc.expectedOK {
				t.Errorf("OK mismatch: got %v, want %v", result.OK, tc.expectedOK)
			}

			if math.Abs(result.Value-tc.vadr) > 0.001 {
				t.Errorf("Value mismatch: got %f, want %f", result.Value, tc.vadr)
			}

			if result.Threshold != tc.minVADR {
				t.Errorf("Threshold mismatch: got %f, want %f", result.Threshold, tc.minVADR)
			}

			if result.Name != "vadr" {
				t.Errorf("Name mismatch: got %s, want vadr", result.Name)
			}
		})
	}
}

func TestADVGateBorderlineCases(t *testing.T) {
	testCases := []struct {
		name        string
		advUSD      int64
		minADVUSD   int64
		expectedOK  bool
	}{
		{
			name:       "Exactly at threshold",
			advUSD:     100000,
			minADVUSD:  100000,
			expectedOK: true,
		},
		{
			name:       "Just above threshold",
			advUSD:     100001,
			minADVUSD:  100000,
			expectedOK: true,
		},
		{
			name:       "Just below threshold",
			advUSD:     99999,
			minADVUSD:  100000,
			expectedOK: false,
		},
		{
			name:       "Zero ADV",
			advUSD:     0,
			minADVUSD:  100000,
			expectedOK: false,
		},
		{
			name:       "Negative ADV",
			advUSD:     -1000,
			minADVUSD:  100000,
			expectedOK: false,
		},
		{
			name:       "Very high ADV",
			advUSD:     1000000000,
			minADVUSD:  100000,
			expectedOK: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			thresholds := domain.MicroGateThresholds{MinADVUSD: tc.minADVUSD}
		inputs := domain.MicroGateInputs{ADVUSD: tc.advUSD}
		results := domain.EvaluateMicroGates(inputs, thresholds)
		result := results.ADV

			if result.OK != tc.expectedOK {
				t.Errorf("OK mismatch: got %v, want %v", result.OK, tc.expectedOK)
			}

			if int64(result.Value) != tc.advUSD {
				t.Errorf("Value mismatch: got %f, want %d", result.Value, tc.advUSD)
			}

			if int64(result.Threshold) != tc.minADVUSD {
				t.Errorf("Threshold mismatch: got %f, want %d", result.Threshold, tc.minADVUSD)
			}

			if result.Name != "adv" {
				t.Errorf("Name mismatch: got %s, want adv", result.Name)
			}
		})
	}
}

func TestMicroGateResultsCombined(t *testing.T) {
	thresholds := domain.MicroGateThresholds{
		MaxSpreadBps: 50.0,
		MinDepthUSD:  100000.0,
		MinVADR:      1.75,
		MinADVUSD:    100000,
	}

	testCases := []struct {
		name              string
		inputs            domain.MicroGateInputs
		expectedAllPass   bool
		expectedReason    string
	}{
		{
			name: "All gates pass",
			inputs: domain.MicroGateInputs{
				Symbol:      "BTCUSD",
				Bid:         50000.0,
				Ask:         50025.0, // 50 bps spread
				Depth2PcUSD: 150000.0,
				VADR:        2.0,
				ADVUSD:      200000,
			},
			expectedAllPass: true,
			expectedReason:  "",
		},
		{
			name: "Spread gate fails",
			inputs: domain.MicroGateInputs{
				Symbol:      "ETHUSD",
				Bid:         3000.0,
				Ask:         3020.0, // 66.44 bps spread
				Depth2PcUSD: 150000.0,
				VADR:        2.0,
				ADVUSD:      200000,
			},
			expectedAllPass: false,
			expectedReason:  "spread 66.45 bps > 50.00 bps",
		},
		{
			name: "Depth gate fails",
			inputs: domain.MicroGateInputs{
				Symbol:      "ADAUSD",
				Bid:         1.0,
				Ask:         1.005, // 50 bps spread
				Depth2PcUSD: 50000.0,
				VADR:        2.0,
				ADVUSD:      200000,
			},
			expectedAllPass: false,
			expectedReason:  "depth $50000 < $100000",
		},
		{
			name: "VADR gate fails",
			inputs: domain.MicroGateInputs{
				Symbol:      "SOLUSD",
				Bid:         100.0,
				Ask:         100.5, // 50 bps spread
				Depth2PcUSD: 150000.0,
				VADR:        1.5,
				ADVUSD:      200000,
			},
			expectedAllPass: false,
			expectedReason:  "VADR 1.500 < 1.750",
		},
		{
			name: "ADV gate fails",
			inputs: domain.MicroGateInputs{
				Symbol:      "DOTUSD",
				Bid:         10.0,
				Ask:         10.05, // 50 bps spread
				Depth2PcUSD: 150000.0,
				VADR:        2.0,
				ADVUSD:      50000,
			},
			expectedAllPass: false,
			expectedReason:  "ADV $50000 < $100000",
		},
		{
			name: "Multiple gates fail - spread reported first",
			inputs: domain.MicroGateInputs{
				Symbol:      "LOWUSD",
				Bid:         1.0,
				Ask:         1.1,   // 952.38 bps spread
				Depth2PcUSD: 10000.0,
				VADR:        1.0,
				ADVUSD:      10000,
			},
			expectedAllPass: false,
			expectedReason:  "spread 952.38 bps > 50.00 bps",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			results := domain.EvaluateMicroGates(tc.inputs, thresholds)

			if results.AllPass != tc.expectedAllPass {
				t.Errorf("AllPass mismatch: got %v, want %v", results.AllPass, tc.expectedAllPass)
			}

			if results.Symbol != tc.inputs.Symbol {
				t.Errorf("Symbol mismatch: got %s, want %s", results.Symbol, tc.inputs.Symbol)
			}

			if tc.expectedReason != "" {
				if results.Reason != tc.expectedReason {
					t.Errorf("Reason mismatch: got '%s', want '%s'", results.Reason, tc.expectedReason)
				}
			}

			// Verify individual gate results are present
			if results.Spread.Name != "spread" {
				t.Error("Spread gate result missing or invalid")
			}
			if results.Depth.Name != "depth" {
				t.Error("Depth gate result missing or invalid")
			}
			if results.VADR.Name != "vadr" {
				t.Error("VADR gate result missing or invalid")
			}
			if results.ADV.Name != "adv" {
				t.Error("ADV gate result missing or invalid")
			}
		})
	}
}

func TestCalculateSpreadBpsEdgeCases(t *testing.T) {
	testCases := []struct {
		name     string
		bid      float64
		ask      float64
		expected float64
	}{
		{
			name:     "Normal spread",
			bid:      100.0,
			ask:      100.5,
			expected: 49.88, // (0.5 / 100.25) * 10000
		},
		{
			name:     "Very tight spread",
			bid:      50000.0,
			ask:      50000.5,
			expected: 0.10, // (0.5 / 50000.25) * 10000
		},
		{
			name:     "Wide spread",
			bid:      100.0,
			ask:      110.0,
			expected: 952.38,
		},
		{
			name:     "Bid equals ask",
			bid:      100.0,
			ask:      100.0,
			expected: math.NaN(),
		},
		{
			name:     "Bid greater than ask",
			bid:      100.0,
			ask:      99.0,
			expected: math.NaN(),
		},
		{
			name:     "Zero bid",
			bid:      0.0,
			ask:      100.0,
			expected: math.NaN(),
		},
		{
			name:     "Negative ask",
			bid:      100.0,
			ask:      -10.0,
			expected: math.NaN(),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := domain.CalculateSpreadBps(tc.bid, tc.ask)

			if !math.IsNaN(tc.expected) {
				if math.Abs(result-tc.expected) > 0.01 {
					t.Errorf("SpreadBps mismatch: got %f, want %f", result, tc.expected)
				}
			} else {
				if !math.IsNaN(result) {
					t.Errorf("Expected NaN, got %f", result)
				}
			}
		})
	}
}

