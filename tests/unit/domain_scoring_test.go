package unit

import (
	"testing"
	"time"

	"github.com/sawpanic/cryptorun/internal/config/regime"
	"github.com/sawpanic/cryptorun/internal/domain/factors"
	"github.com/sawpanic/cryptorun/internal/domain/scoring"
)

// TestUnifiedCompositeScoring tests the single authoritative composite scorer
func TestUnifiedCompositeScoring(t *testing.T) {
	// Create test configuration
	config := regime.WeightsConfig{
		Validation: struct {
			WeightSumTolerance    float64 `yaml:"weight_sum_tolerance"`
			CorrelationThreshold  float64 `yaml:"correlation_threshold"`
			MinMomentumWeight     float64 `yaml:"min_momentum_weight"`
			MaxSocialWeight       float64 `yaml:"max_social_weight"`
			SocialHardCap         float64 `yaml:"social_hard_cap"`
		}{
			WeightSumTolerance:   0.01,
			CorrelationThreshold: 0.7,
			MinMomentumWeight:    0.3,
			MaxSocialWeight:      0.1,
			SocialHardCap:        10.0,
		},
	}

	// Mock regime detector
	regimeDetector := &MockRegimeDetector{}
	
	// Create unified composite scorer
	scorer := scoring.NewCompositeScorer(config, regimeDetector)
	
	t.Run("MomentumCore Protected", func(t *testing.T) {
		// Create test factor row
		rawFactors := factors.RawFactorRow{
			Symbol:          "BTC-USD",
			MomentumCore:    15.5, // Strong momentum
			TechnicalFactor: 8.2,
			VolumeFactor:    12.1,
			QualityFactor:   5.8,
			SocialFactor:    7.3,
			Timestamp:       time.Now(),
		}

		// Create test market data
		marketData := regime.MarketData{
			RealizedVol7d:    0.45,
			BreadthAbove20MA: 0.62,
			BreadthThrust:    0.08,
		}

		// Calculate composite score
		score, err := scorer.CalculateCompositeScore(rawFactors, marketData)
		if err != nil {
			t.Fatalf("Failed to calculate composite score: %v", err)
		}

		// Verify MomentumCore is preserved (protected)
		if score.MomentumCore != rawFactors.MomentumCore {
			t.Errorf("MomentumCore not protected: expected %.2f, got %.2f", 
				rawFactors.MomentumCore, score.MomentumCore)
		}

		// Verify final score is calculated
		if score.FinalScore <= 0 {
			t.Errorf("Final score should be positive, got %.2f", score.FinalScore)
		}

		// Verify score has all components
		if score.WeightedMomentum == 0 {
			t.Error("Weighted momentum should be non-zero")
		}
	})

	t.Run("Social Hard Cap Enforced", func(t *testing.T) {
		// Create test with excessive social factor
		rawFactors := factors.RawFactorRow{
			Symbol:          "ETH-USD",
			MomentumCore:    10.0,
			TechnicalFactor: 5.0,
			VolumeFactor:    8.0,
			QualityFactor:   3.0,
			SocialFactor:    25.0, // Excessive social factor
			Timestamp:       time.Now(),
		}

		marketData := regime.MarketData{
			RealizedVol7d:    0.35,
			BreadthAbove20MA: 0.55,
			BreadthThrust:    0.05,
		}

		score, err := scorer.CalculateCompositeScore(rawFactors, marketData)
		if err != nil {
			t.Fatalf("Failed to calculate composite score: %v", err)
		}

		// Verify social cap is enforced
		if score.SocialCapped > 10.0 {
			t.Errorf("Social cap violated: expected ≤ 10.0, got %.2f", score.SocialCapped)
		}

		// Verify social is applied last (outside 100% allocation)
		expectedCore := score.WeightedMomentum + score.WeightedTechnical + 
		                score.WeightedVolume + score.WeightedQuality
		expectedFinal := expectedCore + score.WeightedSocial

		if abs(score.FinalScore - expectedFinal) > 0.01 {
			t.Errorf("Final score calculation incorrect: expected %.2f, got %.2f", 
				expectedFinal, score.FinalScore)
		}
	})

	t.Run("Weight Normalization", func(t *testing.T) {
		// Test that weights sum to 100%
		rawFactors := factors.RawFactorRow{
			Symbol:          "ADA-USD", 
			MomentumCore:    8.5,
			TechnicalFactor: 4.2,
			VolumeFactor:    6.8,
			QualityFactor:   2.1,
			SocialFactor:    3.5,
			Timestamp:       time.Now(),
		}

		marketData := regime.MarketData{
			RealizedVol7d:    0.25,
			BreadthAbove20MA: 0.70,
			BreadthThrust:    0.12,
		}

		score, err := scorer.CalculateCompositeScore(rawFactors, marketData)
		if err != nil {
			t.Fatalf("Failed to calculate composite score: %v", err)
		}

		// Verify weight sum (excluding social which is additive)
		coreWeightSum := score.Weights.MomentumCore + score.Weights.Technical + 
		                score.Weights.Volume + score.Weights.Quality
		
		if abs(coreWeightSum - 1.0) > 0.01 {
			t.Errorf("Core weights don't sum to 1.0: got %.3f", coreWeightSum)
		}
	})
}

// TestBatchCompositeScoring tests batch scoring efficiency
func TestBatchCompositeScoring(t *testing.T) {
	config := regime.WeightsConfig{
		Validation: struct {
			WeightSumTolerance    float64 `yaml:"weight_sum_tolerance"`
			CorrelationThreshold  float64 `yaml:"correlation_threshold"`
			MinMomentumWeight     float64 `yaml:"min_momentum_weight"`
			MaxSocialWeight       float64 `yaml:"max_social_weight"`
			SocialHardCap         float64 `yaml:"social_hard_cap"`
		}{
			SocialHardCap: 10.0,
		},
	}

	regimeDetector := &MockRegimeDetector{}
	scorer := scoring.NewCompositeScorer(config, regimeDetector)

	// Create batch of factor rows
	rawFactorsMap := map[string]factors.RawFactorRow{
		"BTC-USD": {
			Symbol: "BTC-USD", MomentumCore: 15.2, TechnicalFactor: 8.1,
			VolumeFactor: 12.5, QualityFactor: 6.2, SocialFactor: 4.8,
			Timestamp: time.Now(),
		},
		"ETH-USD": {
			Symbol: "ETH-USD", MomentumCore: 11.8, TechnicalFactor: 6.9,
			VolumeFactor: 9.3, QualityFactor: 4.1, SocialFactor: 6.2,
			Timestamp: time.Now(),
		},
		"ADA-USD": {
			Symbol: "ADA-USD", MomentumCore: 7.4, TechnicalFactor: 3.8,
			VolumeFactor: 5.7, QualityFactor: 2.3, SocialFactor: 8.9,
			Timestamp: time.Now(),
		},
	}

	marketData := regime.MarketData{
		RealizedVol7d:    0.40,
		BreadthAbove20MA: 0.58,
		BreadthThrust:    0.06,
	}

	// Calculate batch scores
	scores, errors := scorer.CalculateBatchScores(rawFactorsMap, marketData)

	// Verify no errors
	if len(errors) > 0 {
		t.Fatalf("Batch scoring failed with %d errors: %v", len(errors), errors[0])
	}

	// Verify all symbols scored
	if len(scores) != 3 {
		t.Errorf("Expected 3 scores, got %d", len(scores))
	}

	// Verify each score
	for symbol, score := range scores {
		if score.Symbol != symbol {
			t.Errorf("Symbol mismatch: expected %s, got %s", symbol, score.Symbol)
		}

		if score.FinalScore <= 0 {
			t.Errorf("Symbol %s has non-positive final score: %.2f", symbol, score.FinalScore)
		}

		// Verify momentum protection
		originalMomentum := rawFactorsMap[symbol].MomentumCore
		if score.MomentumCore != originalMomentum {
			t.Errorf("Symbol %s momentum not protected: expected %.2f, got %.2f",
				symbol, originalMomentum, score.MomentumCore)
		}

		// Verify social cap
		if score.SocialCapped > 10.0 {
			t.Errorf("Symbol %s social cap violated: %.2f", symbol, score.SocialCapped)
		}
	}
}

// TestScoreValidation tests the validation function
func TestScoreValidation(t *testing.T) {
	config := regime.WeightsConfig{
		Validation: struct {
			WeightSumTolerance    float64 `yaml:"weight_sum_tolerance"`
			CorrelationThreshold  float64 `yaml:"correlation_threshold"`
			MinMomentumWeight     float64 `yaml:"min_momentum_weight"`
			MaxSocialWeight       float64 `yaml:"max_social_weight"`
			SocialHardCap         float64 `yaml:"social_hard_cap"`
		}{
			WeightSumTolerance: 0.01,
			SocialHardCap:      10.0,
		},
		QARequirements: struct {
			CorrelationThreshold float64 `yaml:"correlation_threshold"`
		}{
			CorrelationThreshold: 0.7,
		},
	}

	t.Run("Valid Score Passes", func(t *testing.T) {
		validScore := &scoring.CompositeScore{
			Symbol:      "BTC-USD",
			FinalScore:  75.5,
			SocialCapped: 8.2,
			Weights: regime.DomainRegimeWeights{
				MomentumCore: 0.40,
				Technical:    0.25,
				Volume:       0.20,
				Quality:      0.15,
			},
		}

		err := scoring.ValidateScore(validScore, config)
		if err != nil {
			t.Errorf("Valid score should pass validation: %v", err)
		}
	})

	t.Run("Invalid Social Cap Fails", func(t *testing.T) {
		invalidScore := &scoring.CompositeScore{
			Symbol:      "ETH-USD",
			FinalScore:  85.2,
			SocialCapped: 15.0, // Exceeds cap
			Weights: regime.DomainRegimeWeights{
				MomentumCore: 0.40,
				Technical:    0.25,
				Volume:       0.20,
				Quality:      0.15,
			},
		}

		err := scoring.ValidateScore(invalidScore, config)
		if err == nil {
			t.Error("Invalid social cap should fail validation")
		}
	})
}

// MockRegimeDetector for testing
type MockRegimeDetector struct{}

func (m *MockRegimeDetector) DetectRegime(data regime.MarketData) (*regime.RegimeDetectionResult, error) {
	return &regime.RegimeDetectionResult{
		CurrentRegime: regime.TRENDING,
		Confidence:    0.85,
	}, nil
}

func (m *MockRegimeDetector) GetWeightsForRegime(regimeType regime.RegimeType) (regime.DomainRegimeWeights, error) {
	return regime.DomainRegimeWeights{
		MomentumCore: 0.40,
		Technical:    0.25,
		Volume:       0.20,
		Quality:      0.15,
		Social:       0.05, // Applied outside main allocation
	}, nil
}

// Helper function for floating point comparison
func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}