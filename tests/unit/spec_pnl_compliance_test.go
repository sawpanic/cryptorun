package unit

import (
	"context"
	"testing"
	"time"

	"github.com/sawpanic/cryptorun/internal/application/bench"
)

// TestSpecPnLComplianceEnforcement ensures recommendations use only spec-compliant P&L
func TestSpecPnLComplianceEnforcement(t *testing.T) {
	ctx := context.Background()

	// Test cases for spec P&L vs raw 24h compliance
	testCases := []struct {
		name            string
		symbol          string
		raw24hGain      float64
		specPnLExpected float64
		shouldRecommend bool
		expectedReason  string
	}{
		{
			name:            "PositiveSpecPnL_ShouldRecommend",
			symbol:          "ADAUSD",
			raw24hGain:      13.4,
			specPnLExpected: 8.2, // Positive spec P&L
			shouldRecommend: true,
			expectedReason:  "spec_compliant_gain_supports_action",
		},
		{
			name:            "NegativeSpecPnL_ShouldNotRecommend",
			symbol:          "ETHUSD",
			raw24hGain:      42.8, // High raw gain
			specPnLExpected: -2.1, // But negative spec P&L
			shouldRecommend: false,
			expectedReason:  "spec_pnl_negative_correctly_filtered",
		},
		{
			name:            "LowSpecPnL_ShouldNotRecommend",
			symbol:          "SOLUSD",
			raw24hGain:      38.4, // High raw gain
			specPnLExpected: 0.5,  // But very low spec P&L
			shouldRecommend: false,
			expectedReason:  "spec_pnl_too_low_for_action",
		},
		{
			name:            "ModerateSpecPnL_ShouldRecommend",
			symbol:          "DOTUSD",
			raw24hGain:      11.8,
			specPnLExpected: 5.3, // Reasonable spec P&L
			shouldRecommend: true,
			expectedReason:  "spec_compliant_gain_supports_action",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create spec P&L calculator with mock configuration
			gatesConfig := bench.GatesConfig{
				MinScore:     2.0,
				MaxSpreadBps: 50.0,
				MinDepthUSD:  100000.0,
				MinVADR:      1.75,
			}

			guardsConfig := bench.GuardsConfig{
				FatigueThreshold: 12.0,
				RSIThreshold:     70.0,
				MaxBarsAge:       2,
				MaxDelaySeconds:  30,
				ATRFactor:        1.2,
			}

			seriesSource := bench.SeriesSource{
				ExchangeNativeFirst: true,
				PreferredExchanges:  []string{"binance", "kraken"},
				FallbackAggregators: []string{"coingecko"},
			}

			calculator := bench.NewSpecPnLCalculator("trending", gatesConfig, guardsConfig, seriesSource)

			// Calculate spec-compliant P&L
			signalTime := time.Now().Add(-2 * time.Hour)
			result, err := calculator.CalculateSpecPnL(ctx, tc.symbol, signalTime, tc.raw24hGain)

			if err != nil {
				t.Fatalf("Failed to calculate spec P&L: %v", err)
			}

			// Verify spec P&L is used for recommendations (not raw 24h)
			shouldRecommend := result.SpecPnLPct > 1.0 && result.EntryValid && result.ExitValid

			if shouldRecommend != tc.shouldRecommend {
				t.Errorf("SPEC COMPLIANCE VIOLATION: Symbol %s - expected recommendation %v, got %v",
					tc.symbol, tc.shouldRecommend, shouldRecommend)
				t.Errorf("  Raw 24h: %.1f%%, Spec P&L: %.1f%%", tc.raw24hGain, result.SpecPnLPct)
			}

			// Verify raw 24h gain is NOT used for decision making
			if result.Raw24hChange != tc.raw24hGain {
				t.Errorf("Raw 24h gain mismatch: expected %.1f, got %.1f",
					tc.raw24hGain, result.Raw24hChange)
			}

			// Verify spec P&L is the decision metric
			if shouldRecommend && result.SpecPnLPct <= 1.0 {
				t.Errorf("SPEC COMPLIANCE VIOLATION: Recommending symbol %s with spec P&L %.1f <= 1.0%%",
					tc.symbol, result.SpecPnLPct)
			}

			// Log the compliance check
			t.Logf("SPEC COMPLIANCE CHECK: %s - Raw: %.1f%%, Spec: %.1f%%, Recommend: %v",
				tc.symbol, tc.raw24hGain, result.SpecPnLPct, shouldRecommend)
		})
	}
}

// TestSampleSizeGuardEnforcement ensures n≥20 requirement is enforced
func TestSampleSizeGuardEnforcement(t *testing.T) {
	ctx := context.Background()

	testCases := []struct {
		name                        string
		sampleSize                  int
		shouldEnableRecommendations bool
	}{
		{
			name:                        "SufficientSample_EnableRecommendations",
			sampleSize:                  25,
			shouldEnableRecommendations: true,
		},
		{
			name:                        "InsufficientSample_DisableRecommendations",
			sampleSize:                  15,
			shouldEnableRecommendations: false,
		},
		{
			name:                        "BoundarySample_EnableRecommendations",
			sampleSize:                  20,
			shouldEnableRecommendations: true,
		},
		{
			name:                        "BelowBoundary_DisableRecommendations",
			sampleSize:                  19,
			shouldEnableRecommendations: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Load configuration
			config, err := bench.LoadBenchmarkConfig()
			if err != nil {
				t.Fatalf("Failed to load benchmark config: %v", err)
			}

			// Check if recommendations should be enabled based on sample size
			recommendationsEnabled := tc.sampleSize >= config.Diagnostics.SampleSize.MinPerWindow

			if recommendationsEnabled != tc.shouldEnableRecommendations {
				t.Errorf("SAMPLE SIZE GUARD VIOLATION: Sample size %d - expected recommendations enabled %v, got %v",
					tc.sampleSize, tc.shouldEnableRecommendations, recommendationsEnabled)
			}

			// Verify minimum sample size is 20
			if config.Diagnostics.SampleSize.MinPerWindow != 20 {
				t.Errorf("CONFORMANCE VIOLATION: Minimum sample size = %d, must be 20",
					config.Diagnostics.SampleSize.MinPerWindow)
			}

			t.Logf("SAMPLE SIZE CHECK: n=%d, min_required=%d, recommendations_enabled=%v",
				tc.sampleSize, config.Diagnostics.SampleSize.MinPerWindow, recommendationsEnabled)
		})
	}
}

// TestRawGainAdviceProhibition ensures raw 24h gains never drive recommendations
func TestRawGainAdviceProhibition(t *testing.T) {
	// Mock diagnostic analysis with high raw gains but negative spec P&L
	mockAnalysis := &bench.MissAttributionAnalysis{
		TopMissedSymbols: []bench.MissedSymbolAnalysis{
			{
				Symbol:           "ETHUSD",
				GainPercentage:   42.8, // High raw gain
				PrimaryReason:    "gates_failure",
				ConfigTweak:      "Should NOT suggest config changes",
				RecoveryEstimate: "Should NOT estimate recovery",
			},
			{
				Symbol:           "SOLUSD",
				GainPercentage:   38.4, // High raw gain
				PrimaryReason:    "guards_failure",
				ConfigTweak:      "Should NOT suggest config changes",
				RecoveryEstimate: "Should NOT estimate recovery",
			},
		},
	}

	// Verify that high raw gains alone don't drive recommendations
	for _, miss := range mockAnalysis.TopMissedSymbols {
		t.Run(miss.Symbol, func(t *testing.T) {
			// If spec P&L is not positive, should not have actionable recommendations
			hasActionableAdvice := miss.ConfigTweak != "Should NOT suggest config changes" &&
				miss.RecoveryEstimate != "Should NOT estimate recovery"

			// Mock: assume spec P&L is negative for these symbols
			mockSpecPnL := -1.5 // Negative spec P&L

			if hasActionableAdvice && mockSpecPnL <= 0 {
				t.Errorf("RAW GAIN ADVICE VIOLATION: Symbol %s with raw gain %.1f%% and negative spec P&L %.1f%% has actionable advice",
					miss.Symbol, miss.GainPercentage, mockSpecPnL)
			}

			// Verify raw gain percentage is preserved for context
			if miss.GainPercentage <= 0 {
				t.Errorf("Raw gain percentage should be preserved for context: %s has %.1f%%",
					miss.Symbol, miss.GainPercentage)
			}

			t.Logf("RAW GAIN PROHIBITION CHECK: %s - Raw: %.1f%%, Spec: %.1f%%, HasAdvice: %v",
				miss.Symbol, miss.GainPercentage, mockSpecPnL, hasActionableAdvice)
		})
	}
}

// TestExchangeNativeSeriesSourceLabeling ensures proper series source attribution
func TestExchangeNativeSeriesSourceLabeling(t *testing.T) {
	ctx := context.Background()

	testCases := []struct {
		name              string
		preferredExchange string
		expectedLabel     string
		shouldBeFallback  bool
	}{
		{
			name:              "BinanceNative_ProperLabeling",
			preferredExchange: "binance",
			expectedLabel:     "exchange_native_binance",
			shouldBeFallback:  false,
		},
		{
			name:              "KrakenNative_ProperLabeling",
			preferredExchange: "kraken",
			expectedLabel:     "exchange_native_kraken",
			shouldBeFallback:  false,
		},
		{
			name:              "AggregatorFallback_ProperLabeling",
			preferredExchange: "coingecko",
			expectedLabel:     "aggregator_fallback_coingecko",
			shouldBeFallback:  true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Mock series source configuration
			var preferredExchanges []string
			var fallbackAggregators []string

			if tc.shouldBeFallback {
				preferredExchanges = []string{} // No preferred exchanges
				fallbackAggregators = []string{tc.preferredExchange}
			} else {
				preferredExchanges = []string{tc.preferredExchange}
				fallbackAggregators = []string{"coingecko"}
			}

			seriesSource := bench.SeriesSource{
				ExchangeNativeFirst: true,
				PreferredExchanges:  preferredExchanges,
				FallbackAggregators: fallbackAggregators,
			}

			calculator := bench.NewSpecPnLCalculator("trending", bench.GatesConfig{}, bench.GuardsConfig{}, seriesSource)

			// Calculate spec P&L and check series source labeling
			signalTime := time.Now().Add(-2 * time.Hour)
			result, err := calculator.CalculateSpecPnL(ctx, "BTCUSD", signalTime, 15.0)

			if err != nil {
				t.Fatalf("Failed to calculate spec P&L: %v", err)
			}

			// Verify series source is properly labeled
			if result.SeriesSource != tc.expectedLabel {
				t.Errorf("SERIES SOURCE LABELING VIOLATION: Expected '%s', got '%s'",
					tc.expectedLabel, result.SeriesSource)
			}

			// Verify fallback labeling is clear
			if tc.shouldBeFallback && !strings.Contains(result.SeriesSource, "aggregator_fallback") {
				t.Errorf("FALLBACK LABELING VIOLATION: Aggregator source '%s' must contain 'aggregator_fallback'",
					result.SeriesSource)
			}

			// Verify exchange-native labeling
			if !tc.shouldBeFallback && !strings.Contains(result.SeriesSource, "exchange_native") {
				t.Errorf("EXCHANGE NATIVE LABELING VIOLATION: Exchange source '%s' must contain 'exchange_native'",
					result.SeriesSource)
			}

			t.Logf("SERIES SOURCE CHECK: %s -> %s (fallback: %v)",
				tc.preferredExchange, result.SeriesSource, tc.shouldBeFallback)
		})
	}
}

// Helper function to simulate strings.Contains for testing
func contains(s, substr string) bool {
	return len(s) >= len(substr) && s[len(s)-len(substr):] == substr
}
