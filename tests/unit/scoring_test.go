package unit

import (
	"math"
	"testing"
	"time"

	"github.com/sawpanic/cryptorun/internal/application"
	"github.com/sawpanic/cryptorun/internal/domain/factors"
	"github.com/sawpanic/cryptorun/internal/domain/regime"
	"github.com/sawpanic/cryptorun/internal/domain/scoring"
)

func TestCompositeScoring(t *testing.T) {
	config := createTestWeightsConfig()
	regimeDetector := regime.NewRegimeDetector(config)
	scorer := scoring.NewCompositeScorer(config, regimeDetector)

	// Create test data
	rawFactors := factors.RawFactorRow{
		Symbol:          "BTC-USD",
		MomentumCore:    75.0,  // Strong momentum (protected)
		TechnicalFactor: 60.0,  // Good technical setup
		VolumeFactor:    45.0,  // Moderate volume
		QualityFactor:   80.0,  // High quality
		SocialFactor:    12.0,  // Over cap - should be clamped to 10
		Timestamp:       time.Now(),
	}

	regimeData := regime.MarketData{
		Symbol:        "BTC-USD",
		RealizedVol7d: 0.20,  // Normal regime
		MA20:          50000.0,
		CurrentPrice:  51500.0,
		BreadthData: regime.BreadthData{
			AdvanceDeclineRatio: 0.6,
			VolumeRatio:         0.5,
			NewHighsNewLows:     0.4,
		},
		Timestamp: time.Now(),
	}

	// Calculate composite score
	score, err := scorer.CalculateCompositeScore(rawFactors, regimeData)
	if err != nil {
		t.Fatalf("Composite scoring failed: %v", err)
	}

	// Verify basic properties
	if score.Symbol != rawFactors.Symbol {
		t.Errorf("Expected symbol %s, got %s", rawFactors.Symbol, score.Symbol)
	}

	if score.Regime != regime.RegimeNormal {
		t.Errorf("Expected normal regime, got %s", score.Regime)
	}

	// Verify momentum preservation (should be unchanged)
	if math.Abs(score.MomentumCore-rawFactors.MomentumCore) > 0.001 {
		t.Errorf("Momentum core not preserved: %.3f != %.3f", 
			score.MomentumCore, rawFactors.MomentumCore)
	}

	// Verify social cap enforcement (12.0 -> 10.0)
	expectedSocialCap := config.Validation.SocialHardCap
	if math.Abs(score.SocialCapped) > expectedSocialCap+0.001 {
		t.Errorf("Social cap not enforced: |%.3f| > %.1f", 
			score.SocialCapped, expectedSocialCap)
	}

	// Verify final score is reasonable
	if score.FinalScore <= 0 {
		t.Errorf("Final score should be positive, got %.2f", score.FinalScore)
	}

	// Verify weight allocation is correct
	coreWeightSum := score.Weights.MomentumCore + score.Weights.Technical + 
		score.Weights.Volume + score.Weights.Quality
	if math.Abs(coreWeightSum-1.0) > 0.01 {
		t.Errorf("Core weights should sum to 1.0, got %.3f", coreWeightSum)
	}
}

func TestCompositeScoringValidation(t *testing.T) {
	config := createTestWeightsConfig()
	regimeDetector := regime.NewRegimeDetector(config)
	scorer := scoring.NewCompositeScorer(config, regimeDetector)

	rawFactors := factors.RawFactorRow{
		Symbol:          "BTC-USD",
		MomentumCore:    75.0,
		TechnicalFactor: 60.0,
		VolumeFactor:    45.0,
		QualityFactor:   80.0,
		SocialFactor:    8.0,  // Within cap
		Timestamp:       time.Now(),
	}

	regimeData := createNormalRegimeData()

	score, err := scorer.CalculateCompositeScore(rawFactors, regimeData)
	if err != nil {
		t.Fatalf("Composite scoring failed: %v", err)
	}

	// Validate the score
	err = scoring.ValidateScore(score, config)
	if err != nil {
		t.Errorf("Score validation failed: %v", err)
	}

	// Test validation with nil score
	err = scoring.ValidateScore(nil, config)
	if err == nil {
		t.Error("Nil score should fail validation")
	}

	// Test validation with NaN final score
	invalidScore := *score
	invalidScore.FinalScore = math.NaN()
	err = scoring.ValidateScore(&invalidScore, config)
	if err == nil {
		t.Error("NaN final score should fail validation")
	}
}

func TestBatchCompositeScoring(t *testing.T) {
	config := createTestWeightsConfig()
	regimeDetector := regime.NewRegimeDetector(config)
	scorer := scoring.NewCompositeScorer(config, regimeDetector)

	// Create batch of raw factors
	rawFactorsMap := map[string]factors.RawFactorRow{
		"BTC-USD": {
			Symbol:          "BTC-USD",
			MomentumCore:    80.0,
			TechnicalFactor: 70.0,
			VolumeFactor:    60.0,
			QualityFactor:   85.0,
			SocialFactor:    8.0,
			Timestamp:       time.Now(),
		},
		"ETH-USD": {
			Symbol:          "ETH-USD",
			MomentumCore:    65.0,
			TechnicalFactor: 55.0,
			VolumeFactor:    50.0,
			QualityFactor:   70.0,
			SocialFactor:    12.0,  // Over cap
			Timestamp:       time.Now(),
		},
		"ADA-USD": {
			Symbol:          "ADA-USD",
			MomentumCore:    45.0,
			TechnicalFactor: 40.0,
			VolumeFactor:    35.0,
			QualityFactor:   60.0,
			SocialFactor:    5.0,
			Timestamp:       time.Now(),
		},
	}

	regimeData := createNormalRegimeData()

	// Calculate batch scores
	scores, errors := scorer.CalculateBatchScores(rawFactorsMap, regimeData)

	if len(errors) > 0 {
		t.Fatalf("Batch scoring had errors: %v", errors)
	}

	if len(scores) != len(rawFactorsMap) {
		t.Errorf("Expected %d scores, got %d", len(rawFactorsMap), len(scores))
	}

	// Verify each score
	for symbol, score := range scores {
		if score.Symbol != symbol {
			t.Errorf("Score symbol mismatch: expected %s, got %s", symbol, score.Symbol)
		}

		if score.Regime != regime.RegimeNormal {
			t.Errorf("All scores should be in normal regime, %s got %s", symbol, score.Regime)
		}

		// Validate each score
		err := scoring.ValidateScore(score, config)
		if err != nil {
			t.Errorf("Score validation failed for %s: %v", symbol, err)
		}
	}

	// Verify ETH social cap was applied
	ethScore := scores["ETH-USD"]
	if math.Abs(ethScore.SocialCapped) > config.Validation.SocialHardCap+0.001 {
		t.Errorf("ETH social cap not applied: |%.3f| > %.1f", 
			ethScore.SocialCapped, config.Validation.SocialHardCap)
	}
}

func TestScoreRanking(t *testing.T) {
	// Create test scores with different final scores
	scores := map[string]*scoring.CompositeScore{
		"BTC-USD": {Symbol: "BTC-USD", FinalScore: 85.5},
		"ETH-USD": {Symbol: "ETH-USD", FinalScore: 92.1},
		"ADA-USD": {Symbol: "ADA-USD", FinalScore: 78.3},
		"DOT-USD": {Symbol: "DOT-USD", FinalScore: 88.7},
	}

	ranking := scoring.RankScores(scores)

	expectedOrder := []string{"ETH-USD", "DOT-USD", "BTC-USD", "ADA-USD"}
	if len(ranking) != len(expectedOrder) {
		t.Errorf("Expected %d ranked symbols, got %d", len(expectedOrder), len(ranking))
	}

	for i, expected := range expectedOrder {
		if i >= len(ranking) || ranking[i] != expected {
			t.Errorf("Expected position %d to be %s, got %s", i, expected, ranking[i])
		}
	}

	// Test with nil scores (should be excluded)
	scoresWithNil := map[string]*scoring.CompositeScore{
		"BTC-USD": {Symbol: "BTC-USD", FinalScore: 85.5},
		"ETH-USD": nil,  // Should be excluded
		"ADA-USD": {Symbol: "ADA-USD", FinalScore: 78.3},
	}

	ranking = scoring.RankScores(scoresWithNil)
	if len(ranking) != 2 {
		t.Errorf("Expected 2 non-nil scores in ranking, got %d", len(ranking))
	}
}

func TestScoreExplanation(t *testing.T) {
	config := createTestWeightsConfig()
	regimeDetector := regime.NewRegimeDetector(config)
	scorer := scoring.NewCompositeScorer(config, regimeDetector)

	rawFactors := factors.RawFactorRow{
		Symbol:          "BTC-USD",
		MomentumCore:    75.0,
		TechnicalFactor: 60.0,
		VolumeFactor:    45.0,
		QualityFactor:   80.0,
		SocialFactor:    8.0,
		Timestamp:       time.Now(),
	}

	regimeData := createNormalRegimeData()

	score, err := scorer.CalculateCompositeScore(rawFactors, regimeData)
	if err != nil {
		t.Fatalf("Composite scoring failed: %v", err)
	}

	explanation := scoring.GetScoreExplanation(score)

	if len(explanation) == 0 {
		t.Error("Explanation should not be empty")
	}

	// Check that explanation contains key information
	expectedContents := []string{
		"Composite Score",
		"Component Breakdown",
		"Momentum Core",
		"Technical (residual)",
		"Volume (residual)", 
		"Quality (residual)",
		"Social (capped)",
		"Final Score",
		"Orthogonalization Quality",
	}

	for _, content := range expectedContents {
		if !containsSubstring(explanation, content) {
			t.Errorf("Explanation should contain '%s'", content)
		}
	}

	// Test nil score explanation
	nilExplanation := scoring.GetScoreExplanation(nil)
	if nilExplanation != "No score available" {
		t.Error("Nil score should return standard message")
	}
}

func TestRegimeAdaptiveWeights(t *testing.T) {
	config := createTestWeightsConfig()
	regimeDetector := regime.NewRegimeDetector(config)
	scorer := scoring.NewCompositeScorer(config, regimeDetector)

	rawFactors := factors.RawFactorRow{
		Symbol:          "BTC-USD",
		MomentumCore:    75.0,
		TechnicalFactor: 60.0,
		VolumeFactor:    45.0,
		QualityFactor:   80.0,
		SocialFactor:    8.0,
		Timestamp:       time.Now(),
	}

	// Test calm regime (high momentum weight)
	calmRegimeData := regime.MarketData{
		Symbol:        "BTC-USD",
		RealizedVol7d: 0.10,  // Low vol -> calm
		MA20:          50000.0,
		CurrentPrice:  52500.0,  // Strong trend
		BreadthData: regime.BreadthData{
			AdvanceDeclineRatio: 0.8,
			VolumeRatio:         0.7,
			NewHighsNewLows:     0.6,
		},
		Timestamp: time.Now(),
	}

	calmScore, err := scorer.CalculateCompositeScore(rawFactors, calmRegimeData)
	if err != nil {
		t.Fatalf("Calm regime scoring failed: %v", err)
	}

	// Test volatile regime (even higher momentum weight)
	volatileRegimeData := regime.MarketData{
		Symbol:        "BTC-USD",
		RealizedVol7d: 0.50,  // High vol -> volatile
		MA20:          50000.0,
		CurrentPrice:  50000.0,  // No trend
		BreadthData: regime.BreadthData{
			AdvanceDeclineRatio: 0.1,
			VolumeRatio:         0.1,
			NewHighsNewLows:     0.0,
		},
		Timestamp: time.Now(),
	}

	volatileScore, err := scorer.CalculateCompositeScore(rawFactors, volatileRegimeData)
	if err != nil {
		t.Fatalf("Volatile regime scoring failed: %v", err)
	}

	// Verify regime detection worked
	if calmScore.Regime != regime.RegimeCalm {
		t.Errorf("Expected calm regime, got %s", calmScore.Regime)
	}

	if volatileScore.Regime != regime.RegimeVolatile {
		t.Errorf("Expected volatile regime, got %s", volatileScore.Regime)
	}

	// Verify different weights were applied
	if calmScore.Weights.MomentumCore == volatileScore.Weights.MomentumCore {
		t.Error("Calm and volatile regimes should have different momentum weights")
	}

	// Volatile should have higher momentum weight than calm
	if volatileScore.Weights.MomentumCore <= calmScore.Weights.MomentumCore {
		t.Errorf("Volatile momentum weight %.3f should be > calm momentum weight %.3f",
			volatileScore.Weights.MomentumCore, calmScore.Weights.MomentumCore)
	}
}

// Helper functions
func createNormalRegimeData() regime.MarketData {
	return regime.MarketData{
		Symbol:        "BTC-USD",
		RealizedVol7d: 0.25,  // Normal volatility
		MA20:          50000.0,
		CurrentPrice:  51500.0,  // Moderate trend
		BreadthData: regime.BreadthData{
			AdvanceDeclineRatio: 0.5,
			VolumeRatio:         0.4,
			NewHighsNewLows:     0.3,
		},
		Timestamp: time.Now(),
	}
}

func containsSubstring(s, substr string) bool {
	if len(substr) == 0 {
		return true
	}
	if len(s) < len(substr) {
		return false
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
			return true
		}
	}
	return false
}