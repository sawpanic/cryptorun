package gates

import (
	"context"
	"testing"
	"time"

	"github.com/sawpanic/cryptorun/internal/data/derivs"
	"github.com/sawpanic/cryptorun/internal/gates"
	"github.com/sawpanic/cryptorun/internal/microstructure"
)

// Mock providers for testing
type mockFundingProvider struct {
	snapshot *derivs.FundingSnapshot
	err      error
}

func (m *mockFundingProvider) GetFundingSnapshot(ctx context.Context, symbol string) (*derivs.FundingSnapshot, error) {
	return m.snapshot, m.err
}

type mockOIProvider struct {
	snapshot *derivs.OpenInterestSnapshot
	err      error
}

func (m *mockOIProvider) GetOpenInterestSnapshot(ctx context.Context, symbol string, priceChange float64) (*derivs.OpenInterestSnapshot, error) {
	return m.snapshot, m.err
}

type mockETFProvider struct {
	snapshot *derivs.ETFFlowSnapshot
	err      error
}

func (m *mockETFProvider) GetETFFlowSnapshot(ctx context.Context, symbol string) (*derivs.ETFFlowSnapshot, error) {
	return m.snapshot, m.err
}

type mockMicroEvaluator struct {
	result *microstructure.EvaluationResult
	err    error
}

func (m *mockMicroEvaluator) EvaluateSnapshot(symbol string) (microstructure.EvaluationResult, error) {
	if m.result == nil {
		return microstructure.EvaluationResult{}, m.err
	}
	return *m.result, m.err
}

func TestEntryGateEvaluator_ComprehensiveGateValidation(t *testing.T) {
	tests := []struct {
		name           string
		compositeScore float64
		priceChange24h float64
		regime         string
		adv            float64
		fundingZScore  float64
		expectedPass   bool
		expectedReason string
	}{
		{
			name:           "all_gates_pass_trending",
			compositeScore: 85.0, // Above 75
			priceChange24h: 3.0,  // Above trending threshold (2.5%)
			regime:         "TRENDING",
			adv:            1000000.0, // $1M ADV
			fundingZScore:  2.5,       // Above 2.0 threshold
			expectedPass:   true,
			expectedReason: "",
		},
		{
			name:           "score_gate_fails",
			compositeScore: 65.0, // Below 75 threshold
			priceChange24h: 3.0,  // Good movement
			regime:         "TRENDING",
			adv:            1000000.0,
			fundingZScore:  2.5,
			expectedPass:   false,
			expectedReason: "Score 65.0 below threshold 75.0",
		},
		{
			name:           "regime_specific_movement_choppy",
			compositeScore: 85.0,
			priceChange24h: 2.8, // Below choppy threshold (3.0%)
			regime:         "CHOP",
			adv:            1000000.0,
			fundingZScore:  2.5,
			expectedPass:   false,
			expectedReason: "", // Would be set by microstructure gates
		},
		{
			name:           "regime_specific_movement_high_vol",
			compositeScore: 85.0,
			priceChange24h: 3.5, // Below high vol threshold (4.0%)
			regime:         "HIGH_VOL",
			adv:            1000000.0,
			fundingZScore:  2.5,
			expectedPass:   false,
			expectedReason: "", // Would be set by microstructure gates
		},
		{
			name:           "insufficient_liquidity",
			compositeScore: 85.0,
			priceChange24h: 3.0,
			regime:         "TRENDING",
			adv:            100000.0, // Below $500k threshold
			fundingZScore:  2.5,
			expectedPass:   false,
			expectedReason: "", // Would be set by liquidity gate
		},
		{
			name:           "funding_divergence_too_low",
			compositeScore: 85.0,
			priceChange24h: 3.0,
			regime:         "TRENDING",
			adv:            1000000.0,
			fundingZScore:  1.5, // Below 2.0 threshold
			expectedPass:   false,
			expectedReason: "", // Would be set by funding gate
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup mock providers
			fundingProvider := &mockFundingProvider{
				snapshot: &derivs.FundingSnapshot{
					Symbol:                   "BTCUSD",
					FundingZ:                 tt.fundingZScore,
					MaxVenueDivergence:       tt.fundingZScore, // Use same value for divergence
					FundingDivergencePresent: tt.fundingZScore >= 2.0,
					Timestamp:                time.Now(),
				},
			}

			oiProvider := &mockOIProvider{
				snapshot: &derivs.OpenInterestSnapshot{
					Symbol:     "BTCUSD",
					OIResidual: 2000000.0, // $2M residual
					Timestamp:  time.Now(),
				},
			}

			etfProvider := &mockETFProvider{
				snapshot: &derivs.ETFFlowSnapshot{
					Symbol:    "BTCUSD",
					FlowTint:  0.4, // 40% positive tint
					Timestamp: time.Now(),
				},
			}

			microEvaluator := &mockMicroEvaluator{
				result: &microstructure.EvaluationResult{
					SpreadBps:       45.0,             // Below 50bps threshold
					DepthUSD:        150000.0,         // Above $100k threshold
					VADR:            2.1,              // Above 1.75 threshold
					BarCount:        25,               // Above 20 bars requirement
					DailyVolumeUSD:  1500000.0,        // Above $500k threshold
					ADX:             30.0,             // Above 25 threshold
					Hurst:           0.60,             // Above 0.55 threshold
					BarsFromTrigger: 1,                // Below 2 bars threshold
					LateFillDelay:   time.Second * 15, // Below 30s threshold
					Healthy:         true,
				},
			}

			// Create evaluator
			evaluator := gates.NewEntryGateEvaluator(
				microEvaluator,
				fundingProvider,
				oiProvider,
				etfProvider,
			)

			// Run evaluation
			result, err := evaluator.EvaluateEntry(
				context.Background(),
				"BTCUSD",
				tt.compositeScore,
				tt.priceChange24h,
				tt.regime,
				tt.adv,
			)

			if err != nil {
				t.Fatalf("EvaluateEntry failed: %v", err)
			}

			if result == nil {
				t.Fatal("Expected result, got nil")
			}

			// Validate composite score is recorded
			if result.CompositeScore != tt.compositeScore {
				t.Errorf("Expected composite score %.1f, got %.1f",
					tt.compositeScore, result.CompositeScore)
			}

			// Check if basic score gate is properly evaluated
			scoreCheck, exists := result.GateResults["composite_score"]
			if !exists {
				t.Error("Expected composite_score gate check")
			} else {
				expectedScorePass := tt.compositeScore >= 75.0
				if scoreCheck.Passed != expectedScorePass {
					t.Errorf("Score gate: expected pass=%v, got pass=%v",
						expectedScorePass, scoreCheck.Passed)
				}
			}

			// Validate failure reasons are populated when gates fail
			if !result.Passed && len(result.FailureReasons) == 0 {
				t.Error("Expected failure reasons when gates fail")
			}

			// Validate passed gates are recorded when gates pass
			if result.Passed && len(result.PassedGates) == 0 {
				t.Error("Expected passed gates when evaluation passes")
			}

			t.Logf("Result: passed=%v, reasons=%v, passed_gates=%v",
				result.Passed, result.FailureReasons, result.PassedGates)
		})
	}
}

func TestEntryGateEvaluator_RegimeSpecificThresholds(t *testing.T) {
	tests := []struct {
		name                      string
		regime                    string
		expectedMovementThreshold float64
	}{
		{
			name:                      "trending_regime_threshold",
			regime:                    "TRENDING",
			expectedMovementThreshold: 2.5,
		},
		{
			name:                      "choppy_regime_threshold",
			regime:                    "CHOP",
			expectedMovementThreshold: 3.0,
		},
		{
			name:                      "high_vol_regime_threshold",
			regime:                    "HIGH_VOL",
			expectedMovementThreshold: 4.0,
		},
		{
			name:                      "unknown_regime_default",
			regime:                    "unknown",
			expectedMovementThreshold: 4.0, // Should default to high_vol
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := gates.DefaultEntryGateConfig()

			var expectedThreshold float64
			switch tt.regime {
			case "TRENDING":
				expectedThreshold = config.MovementThresholds.Trending
			case "CHOP":
				expectedThreshold = config.MovementThresholds.Choppy
			case "HIGH_VOL":
				expectedThreshold = config.MovementThresholds.HighVol
			default:
				expectedThreshold = config.MovementThresholds.HighVol // Default
			}

			if expectedThreshold != tt.expectedMovementThreshold {
				t.Errorf("Regime %s: expected threshold %.1f%%, got %.1f%%",
					tt.regime, tt.expectedMovementThreshold, expectedThreshold)
			}

			t.Logf("Regime %s uses %.1f%% movement threshold",
				tt.regime, expectedThreshold)
		})
	}
}

func TestEntryGateEvaluator_TieredMicrostructure(t *testing.T) {
	// Test that tiered microstructure results are properly integrated
	microEvaluator := &mockMicroEvaluator{
		result: &microstructure.EvaluationResult{
			SpreadBps:       35.0,             // Below 50bps threshold
			DepthUSD:        250000.0,         // Above $100k threshold
			VADR:            1.9,              // Above 1.75 threshold
			BarCount:        30,               // Above 20 bars requirement
			DailyVolumeUSD:  2000000.0,        // Above $500k threshold
			ADX:             28.0,             // Above 25 threshold
			Hurst:           0.58,             // Above 0.55 threshold
			BarsFromTrigger: 1,                // Below 2 bars threshold
			LateFillDelay:   time.Second * 20, // Below 30s threshold
			Healthy:         true,
		},
	}

	fundingProvider := &mockFundingProvider{
		snapshot: &derivs.FundingSnapshot{
			Symbol:                   "BTCUSD",
			FundingZ:                 2.2,
			MaxVenueDivergence:       2.2,
			FundingDivergencePresent: true,
			Timestamp:                time.Now(),
		},
	}

	evaluator := gates.NewEntryGateEvaluator(
		microEvaluator,
		fundingProvider,
		&mockOIProvider{snapshot: &derivs.OpenInterestSnapshot{Symbol: "BTCUSD", OIResidual: 1800000.0}},
		&mockETFProvider{snapshot: &derivs.ETFFlowSnapshot{Symbol: "BTCUSD", FlowTint: 0.42}},
	)

	result, err := evaluator.EvaluateEntry(
		context.Background(),
		"BTCUSD",
		80.0, // Good composite score
		3.5,  // Good movement
		"TRENDING",
		750000.0, // Good liquidity
	)

	if err != nil {
		t.Fatalf("EvaluateEntry failed: %v", err)
	}

	// Check that tiered gate results are included
	if result.TieredGateResult == nil {
		t.Error("Expected tiered gate result to be populated")
	}

	// Check for traditional microstructure gate checks
	depthCheck, hasDepth := result.GateResults["depth_tiered"]
	spreadCheck, hasSpread := result.GateResults["spread_tiered"]

	if !hasDepth {
		t.Error("Expected depth_tiered gate check")
	}

	if !hasSpread {
		t.Error("Expected spread_tiered gate check")
	}

	if hasDepth && depthCheck != nil {
		t.Logf("Depth check: %.0f ≥ %.0f (passed: %v)",
			depthCheck.Value, depthCheck.Threshold, depthCheck.Passed)
	}

	if hasSpread && spreadCheck != nil {
		t.Logf("Spread check: %.1f bps ≤ %.1f bps (passed: %v)",
			spreadCheck.Value, spreadCheck.Threshold, spreadCheck.Passed)
	}
}

func TestEntryGateEvaluator_Attribution(t *testing.T) {
	// Test that comprehensive attribution is generated
	evaluator := gates.NewEntryGateEvaluator(
		&mockMicroEvaluator{
			result: &microstructure.EvaluationResult{
				SpreadBps: 40.0, DepthUSD: 200000.0, VADR: 2.0, BarCount: 28, DailyVolumeUSD: 800000.0, ADX: 27.0, Hurst: 0.57, BarsFromTrigger: 1, LateFillDelay: time.Second * 10, Healthy: true,
			},
		},
		&mockFundingProvider{snapshot: &derivs.FundingSnapshot{Symbol: "BTCUSD", FundingZ: 2.3, MaxVenueDivergence: 2.3, FundingDivergencePresent: true}},
		&mockOIProvider{snapshot: &derivs.OpenInterestSnapshot{Symbol: "BTCUSD", OIResidual: 1500000.0}},
		&mockETFProvider{snapshot: &derivs.ETFFlowSnapshot{Symbol: "BTCUSD", FlowTint: 0.35}},
	)

	result, err := evaluator.EvaluateEntry(
		context.Background(),
		"BTCUSD",
		85.0,
		4.0,
		"TRENDING",
		900000.0,
	)

	if err != nil {
		t.Fatalf("EvaluateEntry failed: %v", err)
	}

	// Validate comprehensive result structure
	if result.Symbol != "BTCUSD" {
		t.Errorf("Expected symbol BTCUSD, got %s", result.Symbol)
	}

	if result.Timestamp.IsZero() {
		t.Error("Expected non-zero timestamp")
	}

	if result.EvaluationTimeMs <= 0 {
		t.Error("Expected positive evaluation time")
	}

	// Check gate result structure
	if len(result.GateResults) == 0 {
		t.Error("Expected gate results to be populated")
	}

	for gateName, gateCheck := range result.GateResults {
		if gateCheck.Name == "" {
			t.Errorf("Gate %s missing name", gateName)
		}
		if gateCheck.Description == "" {
			t.Errorf("Gate %s missing description", gateName)
		}
		if gateCheck.Value == nil {
			t.Errorf("Gate %s missing value", gateName)
		}
		if gateCheck.Threshold == nil {
			t.Errorf("Gate %s missing threshold", gateName)
		}
	}

	t.Logf("Generated %d gate results with comprehensive attribution",
		len(result.GateResults))
}
