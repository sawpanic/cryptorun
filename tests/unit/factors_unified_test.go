package unit

import (
	"math"
	"testing"
	"time"

	"github.com/sawpanic/cryptorun/internal/domain/factors"
)

func TestUnifiedFactorEngine_WeightNormalization(t *testing.T) {
	testCases := []struct {
		name        string
		weights     factors.RegimeWeights
		expectValid bool
	}{
		{
			name: "valid_bull_weights",
			weights: factors.RegimeWeights{
				MomentumCore:      0.50,
				TechnicalResidual: 0.20,
				VolumeResidual:    0.20,
				QualityResidual:   0.05,
				SocialResidual:    0.05,
			},
			expectValid: true,
		},
		{
			name: "invalid_weights_sum_too_high",
			weights: factors.RegimeWeights{
				MomentumCore:      0.60,
				TechnicalResidual: 0.30,
				VolumeResidual:    0.20,
				QualityResidual:   0.05,
				SocialResidual:    0.05,
			},
			expectValid: false,
		},
		{
			name: "invalid_negative_weight",
			weights: factors.RegimeWeights{
				MomentumCore:      0.50,
				TechnicalResidual: -0.10,
				VolumeResidual:    0.35,
				QualityResidual:   0.15,
				SocialResidual:    0.10,
			},
			expectValid: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := factors.NewUnifiedFactorEngine("test", tc.weights)

			if tc.expectValid && err != nil {
				t.Errorf("Expected valid weights but got error: %v", err)
			}
			if !tc.expectValid && err == nil {
				t.Errorf("Expected invalid weights but got no error")
			}

			// Test weight sum
			sum := tc.weights.Sum()
			t.Logf("Weight sum: %.6f", sum)
		})
	}
}

func TestUnifiedFactorEngine_OrthogonalizationOrder(t *testing.T) {
	// Valid bull regime weights that sum to 1.0
	weights := factors.RegimeWeights{
		MomentumCore:      0.50,
		TechnicalResidual: 0.20,
		VolumeResidual:    0.20,
		QualityResidual:   0.05,
		SocialResidual:    0.05,
	}

	engine, err := factors.NewUnifiedFactorEngine("bull", weights)
	if err != nil {
		t.Fatalf("Failed to create factor engine: %v", err)
	}

	// Create test factor rows with known correlations
	factorRows := []factors.FactorRow{
		{
			Symbol:          "BTCUSD",
			Timestamp:       time.Now(),
			MomentumCore:    10.0, // Strong positive momentum
			TechnicalFactor: 8.5,  // Correlated with momentum
			VolumeFactor:    5.0,  // Moderate volume
			QualityFactor:   7.0,  // Good quality
			SocialFactor:    12.0, // Above +10 cap (should be capped)
		},
		{
			Symbol:          "ETHUSD",
			Timestamp:       time.Now(),
			MomentumCore:    -5.0, // Negative momentum
			TechnicalFactor: -3.0, // Correlated with momentum
			VolumeFactor:    2.0,  // Low volume
			QualityFactor:   6.5,  // Decent quality
			SocialFactor:    3.0,  // Moderate social
		},
		{
			Symbol:          "ADAUSD",
			Timestamp:       time.Now(),
			MomentumCore:    2.0,  // Weak positive momentum
			TechnicalFactor: 1.5,  // Weak technical
			VolumeFactor:    8.0,  // High volume
			QualityFactor:   4.0,  // Lower quality
			SocialFactor:    -8.0, // Negative social
		},
	}

	processed, err := engine.ProcessFactors(factorRows)
	if err != nil {
		t.Fatalf("ProcessFactors failed: %v", err)
	}

	if len(processed) != len(factorRows) {
		t.Errorf("Expected %d processed rows, got %d", len(factorRows), len(processed))
	}

	// Test MomentumCore protection (should remain unchanged)
	for i, row := range processed {
		if row.MomentumCore != factorRows[i].MomentumCore {
			t.Errorf("MomentumCore changed during processing: %.2f -> %.2f",
				factorRows[i].MomentumCore, row.MomentumCore)
		}
	}

	// Test social cap application (should cap at ±10)
	for _, row := range processed {
		if row.SocialResidual > 10.0 {
			t.Errorf("Social residual exceeds +10 cap: %.2f", row.SocialResidual)
		}
		if row.SocialResidual < -10.0 {
			t.Errorf("Social residual below -10 cap: %.2f", row.SocialResidual)
		}
	}

	// Test ranking (scores should be ordered)
	for i := 1; i < len(processed); i++ {
		if processed[i-1].CompositeScore < processed[i].CompositeScore {
			t.Errorf("Scores not properly ranked: %.2f < %.2f",
				processed[i-1].CompositeScore, processed[i].CompositeScore)
		}
	}

	// Test ranks assigned correctly
	for i, row := range processed {
		expectedRank := i + 1
		if row.Rank != expectedRank {
			t.Errorf("Incorrect rank: expected %d, got %d", expectedRank, row.Rank)
		}
	}

	t.Logf("Processed %d factor rows successfully", len(processed))
	for i, row := range processed {
		t.Logf("  %d. %s: Score=%.2f, MomentumCore=%.2f, SocialResidual=%.2f",
			row.Rank, row.Symbol, row.CompositeScore, row.MomentumCore, row.SocialResidual)
	}
}

func TestUnifiedFactorEngine_CorrelationMatrix(t *testing.T) {
	weights := factors.RegimeWeights{
		MomentumCore:      0.50,
		TechnicalResidual: 0.20,
		VolumeResidual:    0.20,
		QualityResidual:   0.05,
		SocialResidual:    0.05,
	}

	engine, err := factors.NewUnifiedFactorEngine("bull", weights)
	if err != nil {
		t.Fatalf("Failed to create factor engine: %v", err)
	}

	// Generate larger sample for correlation testing
	numSamples := 50
	factorRows := make([]factors.FactorRow, numSamples)

	for i := 0; i < numSamples; i++ {
		// Create factors with known correlations
		momentum := float64(i-25) * 0.5            // -12.5 to +12.0
		technical := momentum*0.8 + float64(i%7-3) // Correlated with momentum + noise
		volume := float64(i%10) * 0.3              // Independent volume pattern
		quality := float64(i%8) + 2.0              // Independent quality pattern
		social := momentum*0.3 + float64(i%5-2)    // Weakly correlated with momentum

		factorRows[i] = factors.FactorRow{
			Symbol:          fmt.Sprintf("SYM%02d", i),
			Timestamp:       time.Now(),
			MomentumCore:    momentum,
			TechnicalFactor: technical,
			VolumeFactor:    volume,
			QualityFactor:   quality,
			SocialFactor:    social,
		}
	}

	processed, err := engine.ProcessFactors(factorRows)
	if err != nil {
		t.Fatalf("ProcessFactors failed: %v", err)
	}

	// Get correlation matrix
	corrMatrix := engine.GetCorrelationMatrix(processed)

	// Test correlations between MomentumCore and residuals (should be near zero)
	momentumCorrelations := []string{"TechnicalResidual", "VolumeResidual", "QualityResidual", "SocialResidual"}

	for _, factor := range momentumCorrelations {
		corr := corrMatrix["MomentumCore"][factor]
		if math.Abs(corr) > 0.3 { // Allow some tolerance for small samples
			t.Errorf("High correlation between MomentumCore and %s: %.3f", factor, corr)
		}
		t.Logf("MomentumCore vs %s correlation: %.3f", factor, corr)
	}

	// Test correlations between residual factors (should be lower than original)
	residualPairs := [][]string{
		{"TechnicalResidual", "VolumeResidual"},
		{"TechnicalResidual", "QualityResidual"},
		{"TechnicalResidual", "SocialResidual"},
		{"VolumeResidual", "QualityResidual"},
		{"VolumeResidual", "SocialResidual"},
		{"QualityResidual", "SocialResidual"},
	}

	for _, pair := range residualPairs {
		corr := corrMatrix[pair[0]][pair[1]]
		if math.Abs(corr) > 0.6 { // Residuals should have |ρ| < 0.6
			t.Errorf("High correlation between residuals %s and %s: %.3f", pair[0], pair[1], corr)
		}
		t.Logf("%s vs %s correlation: %.3f", pair[0], pair[1], corr)
	}
}

func TestUnifiedFactorEngine_SocialCapEnforcement(t *testing.T) {
	weights := factors.RegimeWeights{
		MomentumCore:      0.50,
		TechnicalResidual: 0.20,
		VolumeResidual:    0.20,
		QualityResidual:   0.05,
		SocialResidual:    0.05,
	}

	engine, err := factors.NewUnifiedFactorEngine("bull", weights)
	if err != nil {
		t.Fatalf("Failed to create factor engine: %v", err)
	}

	// Test extreme social values
	testCases := []struct {
		socialInput    float64
		expectedCapped float64
	}{
		{15.0, 10.0},   // Should be capped to +10
		{-15.0, -10.0}, // Should be capped to -10
		{8.0, 8.0},     // Should remain unchanged
		{-5.0, -5.0},   // Should remain unchanged
		{10.0, 10.0},   // At the cap, should remain unchanged
		{-10.0, -10.0}, // At the cap, should remain unchanged
	}

	for i, tc := range testCases {
		factorRows := []factors.FactorRow{
			{
				Symbol:          fmt.Sprintf("TEST%d", i),
				Timestamp:       time.Now(),
				MomentumCore:    5.0,
				TechnicalFactor: 3.0,
				VolumeFactor:    2.0,
				QualityFactor:   4.0,
				SocialFactor:    tc.socialInput,
			},
		}

		processed, err := engine.ProcessFactors(factorRows)
		if err != nil {
			t.Fatalf("ProcessFactors failed: %v", err)
		}

		actualCapped := processed[0].SocialResidual

		// Allow some tolerance for residualization effects
		if math.Abs(actualCapped) > 10.0+1e-10 {
			t.Errorf("Social cap not enforced: input=%.2f, expected_capped=%.2f, actual=%.2f",
				tc.socialInput, tc.expectedCapped, actualCapped)
		}

		t.Logf("Social input %.2f -> residual %.2f (capped within ±10)",
			tc.socialInput, actualCapped)
	}
}

func TestUnifiedFactorEngine_RegimeSwitching(t *testing.T) {
	// Test different regime weight profiles
	regimeTests := []struct {
		regime  string
		weights factors.RegimeWeights
	}{
		{
			regime: "bull",
			weights: factors.RegimeWeights{
				MomentumCore:      0.50,
				TechnicalResidual: 0.20,
				VolumeResidual:    0.20,
				QualityResidual:   0.05,
				SocialResidual:    0.05,
			},
		},
		{
			regime: "choppy",
			weights: factors.RegimeWeights{
				MomentumCore:      0.40,
				TechnicalResidual: 0.25,
				VolumeResidual:    0.15,
				QualityResidual:   0.15,
				SocialResidual:    0.05,
			},
		},
		{
			regime: "high_vol",
			weights: factors.RegimeWeights{
				MomentumCore:      0.45,
				TechnicalResidual: 0.15,
				VolumeResidual:    0.25,
				QualityResidual:   0.10,
				SocialResidual:    0.05,
			},
		},
	}

	for _, rt := range regimeTests {
		t.Run(rt.regime, func(t *testing.T) {
			engine, err := factors.NewUnifiedFactorEngine(rt.regime, rt.weights)
			if err != nil {
				t.Fatalf("Failed to create engine for regime %s: %v", rt.regime, err)
			}

			// Verify weight sum = 1.0
			sum := rt.weights.Sum()
			if math.Abs(sum-1.0) > 0.001 {
				t.Errorf("Regime %s weights sum to %.6f, expected 1.0", rt.regime, sum)
			}

			// Verify regime switching
			if engine.GetCurrentRegime() != rt.regime {
				t.Errorf("Expected regime %s, got %s", rt.regime, engine.GetCurrentRegime())
			}

			currentWeights := engine.GetCurrentWeights()
			if currentWeights.MomentumCore != rt.weights.MomentumCore {
				t.Errorf("Weight mismatch after regime switch")
			}

			t.Logf("Regime %s: MomentumCore=%.2f, Sum=%.3f",
				rt.regime, rt.weights.MomentumCore, sum)
		})
	}
}

func TestUnifiedFactorEngine_CompositeScoring(t *testing.T) {
	weights := factors.RegimeWeights{
		MomentumCore:      0.50,
		TechnicalResidual: 0.20,
		VolumeResidual:    0.20,
		QualityResidual:   0.05,
		SocialResidual:    0.05,
	}

	engine, err := factors.NewUnifiedFactorEngine("bull", weights)
	if err != nil {
		t.Fatalf("Failed to create factor engine: %v", err)
	}

	// Test scoring with known values
	factorRows := []factors.FactorRow{
		{
			Symbol:          "HIGH_SCORER",
			MomentumCore:    10.0, // High momentum
			TechnicalFactor: 8.0,
			VolumeFactor:    6.0,
			QualityFactor:   7.0,
			SocialFactor:    5.0,
		},
		{
			Symbol:          "LOW_SCORER",
			MomentumCore:    -5.0, // Negative momentum
			TechnicalFactor: 2.0,
			VolumeFactor:    1.0,
			QualityFactor:   3.0,
			SocialFactor:    -2.0,
		},
	}

	processed, err := engine.ProcessFactors(factorRows)
	if err != nil {
		t.Fatalf("ProcessFactors failed: %v", err)
	}

	// HIGH_SCORER should rank higher than LOW_SCORER
	if processed[0].Symbol != "HIGH_SCORER" {
		t.Errorf("Expected HIGH_SCORER to rank first, got %s", processed[0].Symbol)
	}

	if processed[1].Symbol != "LOW_SCORER" {
		t.Errorf("Expected LOW_SCORER to rank second, got %s", processed[1].Symbol)
	}

	highScore := processed[0].CompositeScore
	lowScore := processed[1].CompositeScore

	if highScore <= lowScore {
		t.Errorf("HIGH_SCORER should have higher composite score: %.2f vs %.2f",
			highScore, lowScore)
	}

	t.Logf("HIGH_SCORER: %.2f, LOW_SCORER: %.2f", highScore, lowScore)
}

// Helper function to format symbol names
func formatSymbol(format string, args ...interface{}) string {
	// This is a placeholder - in real code this would import "fmt"
	if format == "SYM%02d" {
		i := args[0].(int)
		if i < 10 {
			return "SYM0" + string(rune('0'+i))
		}
		return "SYM" + string(rune('0'+i/10)) + string(rune('0'+i%10))
	}
	if format == "TEST%d" {
		i := args[0].(int)
		return "TEST" + string(rune('0'+i))
	}
	return format
}
