package unit

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"cryptorun/internal/config/regime"
	"cryptorun/internal/data/derivs"
	"cryptorun/internal/explain"
	"cryptorun/internal/gates"
	"cryptorun/internal/score/composite"
)

func TestCompositeUnifiedSystem(t *testing.T) {
	tests := []struct {
		name        string
		rawFactors  *composite.RawFactors
		regime      string
		expectPass  bool
		expectScore float64
	}{
		{
			name: "strong_momentum_signal",
			rawFactors: &composite.RawFactors{
				MomentumCore: 85.0,
				Technical:    65.0,
				Volume:       75.0,
				Quality:      60.0,
				Social:       45.0,
			},
			regime:      "normal",
			expectPass:  true,
			expectScore: 80.0, // Approximate expected final score
		},
		{
			name: "weak_signal_below_threshold",
			rawFactors: &composite.RawFactors{
				MomentumCore: 45.0,
				Technical:    35.0,
				Volume:       25.0,
				Quality:      30.0,
				Social:       15.0,
			},
			regime:      "normal",
			expectPass:  false,
			expectScore: 40.0, // Below 75 threshold
		},
		{
			name: "protected_momentum_core",
			rawFactors: &composite.RawFactors{
				MomentumCore: 90.0, // High momentum should be preserved
				Technical:    20.0,
				Volume:       10.0,
				Quality:      15.0,
				Social:       5.0,
			},
			regime:      "calm",
			expectPass:  true,
			expectScore: 75.0, // Momentum-driven
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create unified composite scorer
			scorer := composite.NewCompositeScorer()

			// Load regime weights
			weightsLoader := regime.NewWeightsLoader("../../config/regime_weights.yaml")
			weights, err := weightsLoader.GetWeightsForRegime(tt.regime)
			require.NoError(t, err)

			// Score the asset
			result, err := scorer.ScoreAsset(context.Background(), tt.rawFactors, weights)
			require.NoError(t, err)

			// Validate MomentumCore protection (no orthogonalization)
			assert.Equal(t, tt.rawFactors.MomentumCore, result.MomentumCore,
				"MomentumCore should be protected from orthogonalization")

			// Validate social cap at +10
			assert.LessOrEqual(t, result.SocialResidCapped, 10.0,
				"Social should be capped at +10")

			// Validate score threshold
			passesScore := result.FinalScoreWithSocial >= 75.0
			assert.Equal(t, tt.expectPass, passesScore,
				"Score threshold check should match expected result")

			// Validate approximate score (within tolerance)
			scoreDiff := abs(result.FinalScoreWithSocial - tt.expectScore)
			assert.LessOrEqual(t, scoreDiff, 10.0,
				"Final score should be within 10 points of expected")
		})
	}
}

func TestEntryGatesIntegration(t *testing.T) {
	// Create entry gate evaluator with mock providers
	microEvaluator := createMockMicrostructureEvaluator()
	fundingProvider := derivs.NewFundingProvider()
	oiProvider := derivs.NewOpenInterestProvider()
	etfProvider := derivs.NewETFProvider()

	gateEvaluator := gates.NewEntryGateEvaluator(
		microEvaluator, fundingProvider, oiProvider, etfProvider)

	tests := []struct {
		name        string
		symbol      string
		score       float64
		priceChange float64
		expectPass  bool
		expectGates []string
	}{
		{
			name:        "all_gates_pass",
			symbol:      "BTCUSD",
			score:       85.0,
			priceChange: 0.05, // 5% price change
			expectPass:  true,
			expectGates: []string{"composite_score", "vadr", "spread", "depth", "funding_divergence"},
		},
		{
			name:        "score_too_low",
			symbol:      "ALTCOIN",
			score:       65.0, // Below 75 threshold
			priceChange: 0.03,
			expectPass:  false,
			expectGates: []string{}, // Should fail on first gate
		},
		{
			name:        "funding_insufficient",
			symbol:      "LOWFUNDING",
			score:       80.0,
			priceChange: 0.02,
			expectPass:  false,
			expectGates: []string{"composite_score", "vadr", "spread", "depth"}, // Fails on funding
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			result, err := gateEvaluator.EvaluateEntry(ctx, tt.symbol, tt.score, tt.priceChange)
			require.NoError(t, err)

			assert.Equal(t, tt.expectPass, result.Passed,
				"Entry gate result should match expected")

			if tt.expectPass {
				assert.Empty(t, result.FailureReasons,
					"Passing result should have no failure reasons")
				assert.GreaterOrEqual(t, len(result.PassedGates), len(tt.expectGates),
					"Should have at least expected number of passed gates")
			} else {
				assert.NotEmpty(t, result.FailureReasons,
					"Failing result should have failure reasons")
			}

			// Validate evaluation performance (should be fast)
			assert.LessOrEqual(t, result.EvaluationTimeMs, int64(1000),
				"Entry gate evaluation should complete within 1 second")
		})
	}
}

func TestOrthogonalizationMaintainsProtection(t *testing.T) {
	orthogonalizer := composite.NewOrthogonalizer()

	rawFactors := &composite.RawFactors{
		MomentumCore: 75.0,
		Technical:    50.0,
		Volume:       60.0,
		Quality:      45.0,
		Social:       30.0,
	}

	result, err := orthogonalizer.Orthogonalize(rawFactors)
	require.NoError(t, err)

	// Critical test: MomentumCore must be unchanged
	assert.Equal(t, rawFactors.MomentumCore, result.MomentumCore,
		"MomentumCore must be protected from orthogonalization")

	// Residualized factors should be different from raw
	assert.NotEqual(t, rawFactors.Technical, result.TechnicalResid,
		"Technical should be residualized")
	assert.NotEqual(t, rawFactors.Volume, result.VolumeResid,
		"Volume should be residualized")
	assert.NotEqual(t, rawFactors.Quality, result.QualityResid,
		"Quality should be residualized")

	// Residuals should be bounded
	assert.LessOrEqual(t, abs(result.TechnicalResid), 150.0,
		"Technical residual should be bounded")
	assert.LessOrEqual(t, abs(result.VolumeResid), 150.0,
		"Volume residual should be bounded")
	assert.LessOrEqual(t, abs(result.QualityResid), 150.0,
		"Quality residual should be bounded")
	assert.LessOrEqual(t, abs(result.SocialResid), 150.0,
		"Social residual should be bounded")

	// Test orthogonality validation
	err = orthogonalizer.ValidateOrthogonality(result, 0.1)
	assert.NoError(t, err, "Orthogonalized factors should pass orthogonality validation")
}

func TestExplainerIntegration(t *testing.T) {
	// Create explainer with mock providers
	weightsLoader := regime.NewWeightsLoader("../../config/regime_weights.yaml")
	fundingProvider := derivs.NewFundingProvider()
	oiProvider := derivs.NewOpenInterestProvider()
	etfProvider := derivs.NewETFProvider()

	explainer := explain.NewExplainer(weightsLoader, fundingProvider, oiProvider, etfProvider)

	// Create mock composite score result
	compositeScore := &composite.CompositeScore{
		MomentumCore:         80.0,
		TechnicalResid:       15.0,
		VolumeResid:          20.0,
		QualityResid:         10.0,
		SocialResidCapped:    8.5,
		InternalTotal100:     76.5,
		FinalScoreWithSocial: 85.0,
	}

	rawFactors := &composite.RawFactors{
		MomentumCore: 80.0,
		Technical:    55.0,
		Volume:       65.0,
		Quality:      45.0,
		Social:       35.0,
	}

	microResult := createMockMicroResult()

	ctx := context.Background()
	explanation, err := explainer.ExplainScoring(ctx, "BTCUSD", compositeScore, rawFactors, "normal", microResult)
	require.NoError(t, err)

	// Validate explanation completeness
	assert.Equal(t, "BTCUSD", explanation.Symbol)
	assert.Equal(t, 85.0, explanation.FinalScore)
	assert.Equal(t, "normal", explanation.Regime)

	assert.NotNil(t, explanation.CompositeBreakdown)
	assert.NotEmpty(t, explanation.FactorContributions)
	assert.NotNil(t, explanation.WeightExplanation)
	assert.NotNil(t, explanation.MicrostructureExplanation)

	// Validate factor contributions
	assert.Contains(t, explanation.FactorContributions, "momentum_core")
	assert.Contains(t, explanation.FactorContributions, "social")

	momentumContrib := explanation.FactorContributions["momentum_core"]
	assert.Equal(t, 80.0, momentumContrib.RawValue, "Should preserve raw momentum value")
	assert.Equal(t, 80.0, momentumContrib.OrthogonalValue, "Should preserve orthogonal momentum value")

	// Validate key insights generation
	assert.NotEmpty(t, explanation.KeyInsights, "Should generate key insights")

	// High scores should have positive insights
	hasPositiveInsight := false
	for _, insight := range explanation.KeyInsights {
		if containsAny(insight, []string{"Strong", "Excellent", "✅"}) {
			hasPositiveInsight = true
			break
		}
	}
	assert.True(t, hasPositiveInsight, "High score should generate positive insights")
}

func TestRegimeWeightAdaptation(t *testing.T) {
	weightsLoader := regime.NewWeightsLoader("../../config/regime_weights.yaml")

	regimes := []string{"calm", "normal", "volatile"}

	for _, regimeName := range regimes {
		t.Run("regime_"+regimeName, func(t *testing.T) {
			weights, err := weightsLoader.GetWeightsForRegime(regimeName)
			require.NoError(t, err)

			// All weights should sum to approximately 1.0 (before social addition)
			totalWeight := weights.MomentumCore + weights.TechnicalResid +
				weights.SupplyDemandBlock + weights.CatalystBlock
			assert.InDelta(t, 1.0, totalWeight, 0.01,
				"Regime weights should sum to 1.0")

			// Each weight should be positive and reasonable
			assert.Greater(t, weights.MomentumCore, 0.0, "MomentumCore weight should be positive")
			assert.Greater(t, weights.TechnicalResid, 0.0, "TechnicalResid weight should be positive")
			assert.Greater(t, weights.SupplyDemandBlock, 0.0, "SupplyDemandBlock weight should be positive")
			assert.Greater(t, weights.CatalystBlock, 0.0, "CatalystBlock weight should be positive")

			assert.LessOrEqual(t, weights.MomentumCore, 0.5, "MomentumCore weight should be reasonable")
			assert.LessOrEqual(t, weights.TechnicalResid, 0.4, "TechnicalResid weight should be reasonable")
			assert.LessOrEqual(t, weights.SupplyDemandBlock, 0.4, "SupplyDemandBlock weight should be reasonable")
			assert.LessOrEqual(t, weights.CatalystBlock, 0.2, "CatalystBlock weight should be reasonable")
		})
	}
}

// Helper functions

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}

func containsAny(s string, substrs []string) bool {
	for _, substr := range substrs {
		if contains(s, substr) {
			return true
		}
	}
	return false
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && s[:len(substr)] == substr
}

// Mock helpers

func createMockMicrostructureEvaluator() *MockMicrostructureEvaluator {
	return &MockMicrostructureEvaluator{}
}

type MockMicrostructureEvaluator struct{}

func (m *MockMicrostructureEvaluator) EvaluateSnapshot(ctx context.Context, symbol string) (*MockMicroResult, error) {
	// Return different results based on symbol for testing
	switch symbol {
	case "BTCUSD":
		return &MockMicroResult{
			VADR:      2.1,
			SpreadBps: 25.0,
			DepthUSD:  150000.0,
		}, nil
	case "ALTCOIN":
		return &MockMicroResult{
			VADR:      1.9,
			SpreadBps: 45.0,
			DepthUSD:  120000.0,
		}, nil
	case "LOWFUNDING":
		return &MockMicroResult{
			VADR:      2.0,
			SpreadBps: 35.0,
			DepthUSD:  110000.0,
		}, nil
	default:
		return &MockMicroResult{
			VADR:      1.8,
			SpreadBps: 50.0,
			DepthUSD:  100000.0,
		}, nil
	}
}

type MockMicroResult struct {
	VADR      float64
	SpreadBps float64
	DepthUSD  float64
}

func createMockMicroResult() *MockMicroResult {
	return &MockMicroResult{
		VADR:      2.1,
		SpreadBps: 25.0,
		DepthUSD:  150000.0,
	}
}
