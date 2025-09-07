package unit

import (
	"math"
	"testing"

	"cryptorun/internal/application/pipeline"
)

func TestComputeScores(t *testing.T) {
	scorer := pipeline.NewScorer()
	scorer.SetRegime("bull")

	// Create test factor sets
	factorSets := []pipeline.FactorSet{
		{
			Symbol:       "HIGHSCORE",
			MomentumCore: 15.0, // High momentum
			Volume:       2.0,  // High volume
			Social:       5.0,  // Positive social
			Volatility:   20.0, // Moderate volatility
		},
		{
			Symbol:       "LOWSCORE",
			MomentumCore: -5.0, // Negative momentum
			Volume:       0.5,  // Low volume
			Social:       -3.0, // Negative social
			Volatility:   50.0, // High volatility
		},
		{
			Symbol:       "MEDSCORE",
			MomentumCore: 8.0,  // Medium momentum
			Volume:       1.0,  // Neutral volume
			Social:       2.0,  // Slight positive social
			Volatility:   15.0, // Good volatility
		},
	}

	scores, err := scorer.ComputeScores(factorSets)
	if err != nil {
		t.Fatalf("Failed to compute scores: %v", err)
	}

	if len(scores) != 3 {
		t.Errorf("Expected 3 scores, got %d", len(scores))
	}

	// Check that scores are ranked correctly (highest first)
	if len(scores) >= 2 {
		if scores[0].Score < scores[1].Score {
			t.Error("Scores should be ranked from highest to lowest")
		}
	}

	// Check that highest momentum gets highest score
	highScore := findScoreBySymbol(scores, "HIGHSCORE")
	lowScore := findScoreBySymbol(scores, "LOWSCORE")

	if highScore == nil || lowScore == nil {
		t.Fatal("Could not find expected scores")
	}

	if highScore.Score <= lowScore.Score {
		t.Errorf("High momentum should score higher than low: high=%f, low=%f", highScore.Score, lowScore.Score)
	}

	// Check rank assignment
	if highScore.Rank == 0 || lowScore.Rank == 0 {
		t.Error("Ranks should be assigned (not zero)")
	}

	if highScore.Rank > lowScore.Rank {
		t.Errorf("Lower rank number should indicate higher score: high_rank=%d, low_rank=%d", highScore.Rank, lowScore.Rank)
	}
}

func TestNormalizeMomentumScore(t *testing.T) {
	scorer := pipeline.NewScorer()

	testCases := []struct {
		name     string
		momentum float64
		expected float64 // Approximate expected score
		tolerance float64
	}{
		{"Zero momentum", 0.0, 50.0, 5.0},
		{"Positive momentum", 20.0, 80.0, 10.0},
		{"Negative momentum", -20.0, 10.0, 10.0},
		{"High positive momentum", 30.0, 95.0, 10.0},
		{"High negative momentum", -30.0, 0.0, 10.0},
		{"NaN momentum", math.NaN(), 0.0, 1.0},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// We need to access the private method indirectly through scoring
			factorSet := pipeline.FactorSet{
				Symbol:       "TEST",
				MomentumCore: tc.momentum,
				Volume:       1.0,
				Social:       0.0,
				Volatility:   15.0,
			}

			scores, err := scorer.ComputeScores([]pipeline.FactorSet{factorSet})
			if err != nil {
				t.Fatalf("Failed to compute score: %v", err)
			}

			if len(scores) != 1 {
				t.Fatalf("Expected 1 score, got %d", len(scores))
			}

			momentumScore := scores[0].Components.MomentumScore

			if math.IsNaN(tc.momentum) && momentumScore != 0.0 {
				t.Errorf("NaN momentum should result in 0 score, got %f", momentumScore)
			} else if !math.IsNaN(tc.momentum) {
				if math.Abs(momentumScore-tc.expected) > tc.tolerance {
					t.Errorf("Momentum score out of expected range: got %f, want %f ±%f", momentumScore, tc.expected, tc.tolerance)
				}
			}
		})
	}
}

func TestSelectTopN(t *testing.T) {
	scorer := pipeline.NewScorer()

	// Create scores with known ranking
	scores := []pipeline.CompositeScore{
		{Symbol: "FIRST", Score: 95.0, Rank: 1},
		{Symbol: "SECOND", Score: 85.0, Rank: 2},
		{Symbol: "THIRD", Score: 75.0, Rank: 3},
		{Symbol: "FOURTH", Score: 65.0, Rank: 4},
		{Symbol: "FIFTH", Score: 55.0, Rank: 5},
	}

	testCases := []struct {
		name     string
		n        int
		expected int
	}{
		{"Top 3", 3, 3},
		{"Top 2", 2, 2},
		{"Top 10 (more than available)", 10, 5},
		{"Top 0", 0, 0},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			selected := scorer.SelectTopN(scores, tc.n)

			if len(selected) != tc.expected {
				t.Errorf("Expected %d selected, got %d", tc.expected, len(selected))
			}

			// Check that all selected candidates are marked as selected
			for i, candidate := range selected {
				if !candidate.Selected {
					t.Errorf("Candidate %d should be marked as selected", i)
				}

				if candidate.Rank != i+1 {
					t.Errorf("Candidate %d should have rank %d, got %d", i, i+1, candidate.Rank)
				}
			}

			// Check that they're still in score order
			for i := 1; i < len(selected); i++ {
				if selected[i-1].Score < selected[i].Score {
					t.Errorf("Selected candidates should be in descending score order")
				}
			}
		})
	}
}

func TestRegimeAdjustments(t *testing.T) {
	scorer := pipeline.NewScorer()

	factorSet := pipeline.FactorSet{
		Symbol:       "TEST",
		MomentumCore: 15.0, // High momentum 
		Volume:       2.5,  // High volume
		Social:       0.0,
		Volatility:   35.0, // High volatility
	}

	testRegimes := []struct {
		regime        string
		expectBoost   bool
		expectPenalty bool
	}{
		{"bull", true, false},    // Should boost high momentum
		{"choppy", false, true},  // Should penalize high volatility
		{"high_vol", false, false}, // Mixed effects
	}

	for _, tc := range testRegimes {
		t.Run(tc.regime, func(t *testing.T) {
			scorer.SetRegime(tc.regime)

			scores, err := scorer.ComputeScores([]pipeline.FactorSet{factorSet})
			if err != nil {
				t.Fatalf("Failed to compute scores for regime %s: %v", tc.regime, err)
			}

			if len(scores) != 1 {
				t.Fatalf("Expected 1 score, got %d", len(scores))
			}

			score := scores[0]

			// Check that regime is recorded
			if score.Meta.Regime != tc.regime {
				t.Errorf("Wrong regime in meta: got %s, want %s", score.Meta.Regime, tc.regime)
			}

			// Check that score is reasonable (should be above 0 and below 200)
			if score.Score <= 0 || score.Score >= 200 {
				t.Errorf("Score seems unreasonable for regime %s: %f", tc.regime, score.Score)
			}
		})
	}
}

func TestVolumeScoring(t *testing.T) {
	scorer := pipeline.NewScorer()

	testCases := []struct {
		name     string
		volume   float64
		expected float64 // Approximate expected score
		tolerance float64
	}{
		{"Unit volume", 1.0, 50.0, 5.0},      // Log10(1) = 0, so 50 + 0*25 = 50
		{"High volume", 10.0, 75.0, 5.0},    // Log10(10) = 1, so 50 + 1*25 = 75
		{"Low volume", 0.1, 25.0, 5.0},      // Log10(0.1) = -1, so 50 + (-1)*25 = 25
		{"Zero volume", 0.0, 50.0, 5.0},     // Should default to neutral
		{"NaN volume", math.NaN(), 50.0, 1.0}, // Should default to neutral
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			factorSet := pipeline.FactorSet{
				Symbol:       "TEST",
				MomentumCore: 0.0, // Neutral momentum
				Volume:       tc.volume,
				Social:       0.0, // Neutral social
				Volatility:   15.0, // Optimal volatility
			}

			scores, err := scorer.ComputeScores([]pipeline.FactorSet{factorSet})
			if err != nil {
				t.Fatalf("Failed to compute score: %v", err)
			}

			volumeScore := scores[0].Components.VolumeScore

			if math.Abs(volumeScore-tc.expected) > tc.tolerance {
				t.Errorf("Volume score out of expected range: got %f, want %f ±%f", volumeScore, tc.expected, tc.tolerance)
			}
		})
	}
}

func TestSocialScoring(t *testing.T) {
	scorer := pipeline.NewScorer()

	testCases := []struct {
		name     string
		social   float64
		expected float64
		tolerance float64
	}{
		{"Neutral social", 0.0, 50.0, 1.0},
		{"Positive social", 5.0, 62.5, 2.0},   // 50 + 5*2.5 = 62.5
		{"Negative social", -5.0, 37.5, 2.0},  // 50 - 5*2.5 = 37.5
		{"Max positive", 10.0, 75.0, 2.0},     // 50 + 10*2.5 = 75
		{"Max negative", -10.0, 25.0, 2.0},    // 50 - 10*2.5 = 25
		{"NaN social", math.NaN(), 50.0, 1.0}, // Should default to neutral
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			factorSet := pipeline.FactorSet{
				Symbol:       "TEST",
				MomentumCore: 0.0, // Neutral momentum
				Volume:       1.0, // Neutral volume
				Social:       tc.social,
				Volatility:   15.0, // Optimal volatility
			}

			scores, err := scorer.ComputeScores([]pipeline.FactorSet{factorSet})
			if err != nil {
				t.Fatalf("Failed to compute score: %v", err)
			}

			socialScore := scores[0].Components.SocialScore

			if math.Abs(socialScore-tc.expected) > tc.tolerance {
				t.Errorf("Social score out of expected range: got %f, want %f ±%f", socialScore, tc.expected, tc.tolerance)
			}
		})
	}
}

func TestVolatilityScoring(t *testing.T) {
	scorer := pipeline.NewScorer()

	testCases := []struct {
		name        string
		volatility  float64
		expectHigh  bool // Should get high score (near 100)
		expectLow   bool // Should get low score (well below 50)
	}{
		{"Optimal volatility", 20.0, true, false},   // In 15-25 range
		{"Low volatility", 5.0, false, false},       // Below optimal but not terrible
		{"High volatility", 50.0, false, true},      // Too high, should be penalized
		{"Very high volatility", 100.0, false, true}, // Very high, heavy penalty
		{"Zero volatility", 0.0, false, true},       // No movement, poor
		{"NaN volatility", math.NaN(), false, false}, // Neutral default
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			factorSet := pipeline.FactorSet{
				Symbol:       "TEST",
				MomentumCore: 0.0, // Neutral momentum
				Volume:       1.0, // Neutral volume
				Social:       0.0, // Neutral social
				Volatility:   tc.volatility,
			}

			scores, err := scorer.ComputeScores([]pipeline.FactorSet{factorSet})
			if err != nil {
				t.Fatalf("Failed to compute score: %v", err)
			}

			volScore := scores[0].Components.VolatilityScore

			if tc.expectHigh && volScore < 80.0 {
				t.Errorf("Expected high volatility score for %f, got %f", tc.volatility, volScore)
			}

			if tc.expectLow && volScore > 40.0 {
				t.Errorf("Expected low volatility score for %f, got %f", tc.volatility, volScore)
			}

			// All scores should be in 0-100 range
			if volScore < 0 || volScore > 100 {
				t.Errorf("Volatility score out of valid range: %f", volScore)
			}
		})
	}
}

func TestScoreBreakdown(t *testing.T) {
	scorer := pipeline.NewScorer()

	factorSet := pipeline.FactorSet{
		Symbol:       "TEST",
		MomentumCore: 10.0,
		Volume:       1.5,
		Social:       3.0,
		Volatility:   18.0,
	}

	scores, err := scorer.ComputeScores([]pipeline.FactorSet{factorSet})
	if err != nil {
		t.Fatalf("Failed to compute score: %v", err)
	}

	breakdown := scorer.GetScoreBreakdown(scores[0])

	// Check that breakdown contains all expected fields
	expectedFields := []string{
		"symbol", "final_score", "rank", "selected",
		"momentum_score", "volume_score", "social_score", "volatility_score",
		"weighted_sum", "regime", "factors_used", "validation_pass", "weights",
	}

	for _, field := range expectedFields {
		if _, exists := breakdown[field]; !exists {
			t.Errorf("Missing field in breakdown: %s", field)
		}
	}

	// Check that symbol matches
	if breakdown["symbol"] != "TEST" {
		t.Errorf("Wrong symbol in breakdown: got %v, want TEST", breakdown["symbol"])
	}

	// Check that weights are included
	weights, ok := breakdown["weights"].(map[string]float64)
	if !ok {
		t.Error("Weights should be a map[string]float64")
	} else {
		expectedWeightFields := []string{"momentum", "volume", "social", "volatility"}
		for _, field := range expectedWeightFields {
			if _, exists := weights[field]; !exists {
				t.Errorf("Missing weight field: %s", field)
			}
		}
	}
}

// Helper function to find a score by symbol
func findScoreBySymbol(scores []pipeline.CompositeScore, symbol string) *pipeline.CompositeScore {
	for i := range scores {
		if scores[i].Symbol == symbol {
			return &scores[i]
		}
	}
	return nil
}