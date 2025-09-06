package integration

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
	"cryptorun/internal/microstructure"
	"cryptorun/internal/score/composite"
)

// TestUnifiedSystemEndToEnd tests the complete unified scoring pipeline
func TestUnifiedSystemEndToEnd(t *testing.T) {
	ctx := context.Background()

	// Initialize all components of the unified system
	scorer := composite.NewCompositeScorer()
	weightsLoader := regime.NewWeightsLoader("../../config/regime_weights.yaml")

	// Mock providers for testing
	fundingProvider := createMockFundingProvider()
	oiProvider := createMockOIProvider()
	etfProvider := createMockETFProvider()
	microEvaluator := createMockMicroEvaluator()

	gateEvaluator := gates.NewEntryGateEvaluator(
		microEvaluator, fundingProvider, oiProvider, etfProvider)

	explainer := explain.NewExplainer(
		weightsLoader, fundingProvider, oiProvider, etfProvider)

	// Test scenarios
	testCases := []struct {
		name        string
		symbol      string
		rawFactors  *composite.RawFactors
		regime      string
		expectEntry bool
		expectScore float64
		priceChange float64
	}{
		{
			name:   "high_quality_btc_signal",
			symbol: "BTCUSD",
			rawFactors: &composite.RawFactors{
				MomentumCore: 90.0,
				Technical:    70.0,
				Volume:       80.0,
				Quality:      65.0,
				Social:       55.0,
			},
			regime:      "normal",
			expectEntry: true,
			expectScore: 85.0,
			priceChange: 0.05,
		},
		{
			name:   "marginal_eth_signal",
			symbol: "ETHUSD",
			rawFactors: &composite.RawFactors{
				MomentumCore: 78.0,
				Technical:    45.0,
				Volume:       50.0,
				Quality:      55.0,
				Social:       30.0,
			},
			regime:      "volatile",
			expectEntry: true, // Should pass with different weight profile
			expectScore: 76.0,
			priceChange: 0.03,
		},
		{
			name:   "weak_altcoin_signal",
			symbol: "ALTCOIN",
			rawFactors: &composite.RawFactors{
				MomentumCore: 45.0,
				Technical:    35.0,
				Volume:       25.0,
				Quality:      40.0,
				Social:       20.0,
			},
			regime:      "calm",
			expectEntry: false, // Should fail score threshold
			expectScore: 50.0,
			priceChange: 0.02,
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			// Step 1: Score the asset using unified composite system
			weights, err := weightsLoader.GetWeightsForRegime(tt.regime)
			require.NoError(t, err)

			compositeResult, err := scorer.ScoreAsset(ctx, tt.rawFactors, weights)
			require.NoError(t, err)

			// Validate scoring system properties
			assert.Equal(t, tt.rawFactors.MomentumCore, compositeResult.MomentumCore,
				"MomentumCore should be protected")
			assert.LessOrEqual(t, compositeResult.SocialResidCapped, 10.0,
				"Social should be capped at +10")

			// Step 2: Evaluate entry gates
			gateResult, err := gateEvaluator.EvaluateEntry(
				ctx, tt.symbol, compositeResult.FinalScoreWithSocial, tt.priceChange)
			require.NoError(t, err)

			assert.Equal(t, tt.expectEntry, gateResult.Passed,
				"Entry gate result should match expectation")

			// Step 3: Generate explanation
			microResult := &microstructure.EvaluationResult{
				VADR:      2.1,
				SpreadBps: 25.0,
				DepthUSD:  150000.0,
			}

			explanation, err := explainer.ExplainScoring(
				ctx, tt.symbol, compositeResult, tt.rawFactors, tt.regime, microResult)
			require.NoError(t, err)

			// Validate explanation completeness
			assert.Equal(t, tt.symbol, explanation.Symbol)
			assert.Equal(t, compositeResult.FinalScoreWithSocial, explanation.FinalScore)
			assert.NotEmpty(t, explanation.KeyInsights)

			// Step 4: Validate end-to-end consistency
			if gateResult.Passed {
				// Score should be ≥75 if gates passed
				assert.GreaterOrEqual(t, compositeResult.FinalScoreWithSocial, 75.0,
					"Passing gates implies score ≥75")

				// Should have positive key insights
				hasPositiveInsight := false
				for _, insight := range explanation.KeyInsights {
					if contains(insight, "✅") || contains(insight, "Strong") {
						hasPositiveInsight = true
						break
					}
				}
				assert.True(t, hasPositiveInsight,
					"Passing assets should have positive insights")
			}

			// Step 5: Performance validation
			assert.LessOrEqual(t, gateResult.EvaluationTimeMs, int64(2000),
				"Complete evaluation should finish within 2 seconds")

			t.Logf("End-to-end test completed for %s: Score=%.1f, Entry=%t, Gates=%d/%d",
				tt.symbol, compositeResult.FinalScoreWithSocial, gateResult.Passed,
				len(gateResult.PassedGates), len(gateResult.GateResults))
		})
	}
}

// TestRegimeSensitivity validates that regime changes affect scoring appropriately
func TestRegimeSensitivity(t *testing.T) {
	ctx := context.Background()
	scorer := composite.NewCompositeScorer()
	weightsLoader := regime.NewWeightsLoader("../../config/regime_weights.yaml")

	// Fixed raw factors to test regime impact
	rawFactors := &composite.RawFactors{
		MomentumCore: 70.0,
		Technical:    60.0,
		Volume:       55.0,
		Quality:      50.0,
		Social:       40.0,
	}

	regimes := []string{"calm", "normal", "volatile"}
	scores := make(map[string]float64)

	// Score the same asset under different regimes
	for _, regime := range regimes {
		weights, err := weightsLoader.GetWeightsForRegime(regime)
		require.NoError(t, err)

		result, err := scorer.ScoreAsset(ctx, rawFactors, weights)
		require.NoError(t, err)

		scores[regime] = result.FinalScoreWithSocial

		// MomentumCore should always be protected regardless of regime
		assert.Equal(t, rawFactors.MomentumCore, result.MomentumCore,
			"MomentumCore should be protected in %s regime", regime)
	}

	// Scores should be different across regimes (regime sensitivity)
	assert.NotEqual(t, scores["calm"], scores["normal"],
		"Calm and normal regimes should produce different scores")
	assert.NotEqual(t, scores["normal"], scores["volatile"],
		"Normal and volatile regimes should produce different scores")

	// Log regime impact for debugging
	t.Logf("Regime score sensitivity: Calm=%.1f, Normal=%.1f, Volatile=%.1f",
		scores["calm"], scores["normal"], scores["volatile"])
}

// TestDataProviderResilience tests system behavior with provider failures
func TestDataProviderResilience(t *testing.T) {
	ctx := context.Background()

	// Create providers with controlled failures
	fundingProvider := createFailingFundingProvider() // Will return errors
	oiProvider := createMockOIProvider()              // Will work normally
	etfProvider := createMockETFProvider()            // Will work normally
	microEvaluator := createMockMicroEvaluator()

	gateEvaluator := gates.NewEntryGateEvaluator(
		microEvaluator, fundingProvider, oiProvider, etfProvider)

	// Test with funding provider failure
	result, err := gateEvaluator.EvaluateEntry(ctx, "BTCUSD", 80.0, 0.05)
	require.NoError(t, err, "System should handle funding provider failures gracefully")

	// Should fail due to funding requirement, but not due to system error
	assert.False(t, result.Passed, "Should fail when required funding data unavailable")
	assert.Contains(t, result.FailureReasons, "Funding divergence data unavailable")

	// Performance should still be acceptable even with failures
	assert.LessOrEqual(t, result.EvaluationTimeMs, int64(5000),
		"Even with provider failures, evaluation should complete within 5 seconds")
}

// TestCacheEfficiency validates caching behavior across components
func TestCacheEfficiency(t *testing.T) {
	ctx := context.Background()

	fundingProvider := derivs.NewFundingProvider()
	symbol := "BTCUSD"

	// First call should miss cache
	start1 := time.Now()
	snapshot1, err := fundingProvider.GetFundingSnapshot(ctx, symbol)
	require.NoError(t, err)
	duration1 := time.Since(start1)
	assert.False(t, snapshot1.CacheHit, "First call should miss cache")

	// Second call should hit cache
	start2 := time.Now()
	snapshot2, err := fundingProvider.GetFundingSnapshot(ctx, symbol)
	require.NoError(t, err)
	duration2 := time.Since(start2)
	assert.True(t, snapshot2.CacheHit, "Second call should hit cache")

	// Cache hit should be significantly faster
	assert.Less(t, duration2, duration1/2, "Cache hit should be at least 2x faster")

	// Data should be consistent
	assert.Equal(t, snapshot1.Symbol, snapshot2.Symbol)
	assert.Equal(t, snapshot1.MaxDivergence, snapshot2.MaxDivergence)
}

// TestSystemIntegrationUnderLoad simulates system behavior under load
func TestSystemIntegrationUnderLoad(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping load test in short mode")
	}

	ctx := context.Background()
	scorer := composite.NewCompositeScorer()
	weightsLoader := regime.NewWeightsLoader("../../config/regime_weights.yaml")

	// Test concurrent scoring requests
	numGoroutines := 10
	numRequests := 5
	results := make(chan time.Duration, numGoroutines*numRequests)

	for i := 0; i < numGoroutines; i++ {
		go func(goroutineID int) {
			for j := 0; j < numRequests; j++ {
				start := time.Now()

				rawFactors := &composite.RawFactors{
					MomentumCore: 75.0 + float64(goroutineID*j),
					Technical:    60.0,
					Volume:       55.0,
					Quality:      50.0,
					Social:       45.0,
				}

				weights, err := weightsLoader.GetWeightsForRegime("normal")
				require.NoError(t, err)

				_, err = scorer.ScoreAsset(ctx, rawFactors, weights)
				require.NoError(t, err)

				results <- time.Since(start)
			}
		}(i)
	}

	// Collect all results
	var durations []time.Duration
	for i := 0; i < numGoroutines*numRequests; i++ {
		durations = append(durations, <-results)
	}

	// Validate performance under load
	totalRequests := len(durations)
	var totalTime time.Duration
	maxTime := time.Duration(0)

	for _, duration := range durations {
		totalTime += duration
		if duration > maxTime {
			maxTime = duration
		}
	}

	avgTime := totalTime / time.Duration(totalRequests)

	// Performance requirements
	assert.Less(t, avgTime, 100*time.Millisecond,
		"Average scoring time should be under 100ms")
	assert.Less(t, maxTime, 500*time.Millisecond,
		"Max scoring time should be under 500ms")

	t.Logf("Load test completed: %d requests, avg: %v, max: %v",
		totalRequests, avgTime, maxTime)
}

// Helper functions and mocks

func contains(s, substr string) bool {
	return len(s) >= len(substr) &&
		findSubstring(s, substr) != -1
}

func findSubstring(s, substr string) int {
	if len(substr) == 0 {
		return 0
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		match := true
		for j := 0; j < len(substr); j++ {
			if s[i+j] != substr[j] {
				match = false
				break
			}
		}
		if match {
			return i
		}
	}
	return -1
}

func createMockFundingProvider() *MockFundingProvider {
	return &MockFundingProvider{}
}

type MockFundingProvider struct{}

func (m *MockFundingProvider) GetFundingSnapshot(ctx context.Context, symbol string) (*MockFundingSnapshot, error) {
	return &MockFundingSnapshot{
		Symbol:        symbol,
		MaxDivergence: 2.5, // Above 2.0 threshold
		VenueRates: map[string]float64{
			"binance": 0.01,
			"okx":     0.008,
			"bybit":   0.015,
		},
		CacheHit: false,
	}, nil
}

type MockFundingSnapshot struct {
	Symbol        string
	MaxDivergence float64
	VenueRates    map[string]float64
	CacheHit      bool
}

func (m *MockFundingSnapshot) HasSignificantDivergence(threshold float64) bool {
	return m.MaxDivergence >= threshold
}

func createFailingFundingProvider() *FailingFundingProvider {
	return &FailingFundingProvider{}
}

type FailingFundingProvider struct{}

func (f *FailingFundingProvider) GetFundingSnapshot(ctx context.Context, symbol string) (*MockFundingSnapshot, error) {
	return nil, assert.AnError // Always returns error
}

func createMockOIProvider() *MockOIProvider {
	return &MockOIProvider{}
}

type MockOIProvider struct{}

func (m *MockOIProvider) GetOpenInterestSnapshot(ctx context.Context, symbol string, priceChange float64) (*MockOISnapshot, error) {
	return &MockOISnapshot{
		OIResidual: 1500000.0, // Above $1M threshold
		CacheHit:   false,
	}, nil
}

type MockOISnapshot struct {
	OIResidual float64
	CacheHit   bool
}

func createMockETFProvider() *MockETFProvider {
	return &MockETFProvider{}
}

type MockETFProvider struct{}

func (m *MockETFProvider) GetETFFlowSnapshot(ctx context.Context, symbol string) (*MockETFSnapshot, error) {
	return &MockETFSnapshot{
		FlowTint: 0.4, // Above 0.3 threshold
		ETFList:  []string{"GBTC", "IBIT"},
		CacheHit: false,
	}, nil
}

type MockETFSnapshot struct {
	FlowTint float64
	ETFList  []string
	CacheHit bool
}

func (m *MockETFSnapshot) IsFlowTintBullish(threshold float64) bool {
	return m.FlowTint >= threshold
}

func createMockMicroEvaluator() *MockMicroEvaluator {
	return &MockMicroEvaluator{}
}

type MockMicroEvaluator struct{}

func (m *MockMicroEvaluator) EvaluateSnapshot(ctx context.Context, symbol string) (*MockMicroResult, error) {
	return &MockMicroResult{
		VADR:      2.1,      // Above 1.8 threshold
		SpreadBps: 25.0,     // Below 50 threshold
		DepthUSD:  150000.0, // Above $100k threshold
	}, nil
}

type MockMicroResult struct {
	VADR      float64
	SpreadBps float64
	DepthUSD  float64
}
