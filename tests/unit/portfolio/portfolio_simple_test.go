package portfolio_test

import (
	"context"  
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sawpanic/cryptorun/internal/score/composite"
	"github.com/sawpanic/cryptorun/internal/score/portfolio"
)

// MockUnifiedScorer for testing
type MockUnifiedScorer struct{}

func (m *MockUnifiedScorer) Score(input composite.ScoringInput) composite.CompositeScore {
	// Return a mock score based on some simple logic
	baseScore := 75.0
	if input.Momentum1h > 0.05 {
		baseScore += 10.0
	}
	if input.VolumeSurge > 2.0 {
		baseScore += 5.0
	}
	
	return composite.CompositeScore{
		FinalWithSocial: baseScore,
		Symbol:          input.Symbol,
		Timestamp:       input.Timestamp,
		Regime:          input.Regime,
	}
}

// MockRiskEnvelope for testing
type MockRiskEnvelope struct {
	MaxPositions     int
	MaxSectorWeight  float64
	MaxBetaBudget    float64
	MaxDrawdown      float64
	VolatilityTarget float64
}

func (m *MockRiskEnvelope) GetMaxPositions() int        { return m.MaxPositions }
func (m *MockRiskEnvelope) GetMaxSectorWeight() float64 { return m.MaxSectorWeight }
func (m *MockRiskEnvelope) GetMaxBetaBudget() float64   { return m.MaxBetaBudget }
func (m *MockRiskEnvelope) GetMaxDrawdown() float64     { return m.MaxDrawdown }
func (m *MockRiskEnvelope) GetVolatilityTarget() float64 { return m.VolatilityTarget }

func TestPortfolioAwareScorer_Creation(t *testing.T) {
	compositeScorer := &MockUnifiedScorer{}
	riskEnvelope := &MockRiskEnvelope{
		MaxPositions:     10,
		MaxSectorWeight:  0.30,
		MaxBetaBudget:    2.5,
		MaxDrawdown:      0.15,
		VolatilityTarget: 0.20,
	}

	config := &portfolio.ConstraintConfig{
		MaxPositionSize:   0.10,
		MaxSectorConc:     0.25,
		MaxCorrelation:    0.70,
		MinLiquidity:      100000,
		DrawdownThreshold: 0.12,
		BetaBudgetLimit:   2.0,
	}

	scorer, err := portfolio.NewPortfolioAwareScorer(compositeScorer, riskEnvelope, config)
	require.NoError(t, err)
	assert.NotNil(t, scorer)
}

func TestPortfolioAwareScorer_CleanPortfolioScoring(t *testing.T) {
	compositeScorer := &MockUnifiedScorer{}
	riskEnvelope := &MockRiskEnvelope{
		MaxPositions:     10,
		MaxSectorWeight:  0.30,
		MaxBetaBudget:    2.5,
		MaxDrawdown:      0.15,
		VolatilityTarget: 0.20,
	}

	config := &portfolio.ConstraintConfig{
		MaxPositionSize:   0.10,
		MaxSectorConc:     0.25,
		MaxCorrelation:    0.70,
		MinLiquidity:      100000,
		DrawdownThreshold: 0.12,
		BetaBudgetLimit:   2.0,
	}

	scorer, err := portfolio.NewPortfolioAwareScorer(compositeScorer, riskEnvelope, config)
	require.NoError(t, err)

	// Good scoring input
	input := composite.ScoringInput{
		Symbol:      "BTC-USD",
		Timestamp:   time.Now(),
		Momentum1h:  0.08, // Good momentum
		VolumeSurge: 2.5,  // Good volume
		Regime:      "normal",
	}

	// Clean portfolio
	portfolioState := &portfolio.PortfolioState{
		Positions:     []portfolio.Position{},
		TotalValue:    1000000,
		SectorWeights: map[string]float64{},
		BetaExposure:  1.0,
		Drawdown:      0.02,
		Volatility:    0.14,
	}

	ctx := context.Background()
	result, err := scorer.ScoreWithConstraints(ctx, input, portfolioState)
	require.NoError(t, err)
	assert.NotNil(t, result)

	assert.True(t, result.BaseScore >= 75.0, "Base score should be good with strong input: %f", result.BaseScore)
	// With clean portfolio, score should not be heavily penalized
	assert.True(t, result.AdjustedScore >= result.BaseScore*0.9, "Score should not be heavily penalized with clean portfolio")
	assert.True(t, result.SuggestedSize >= 0.05, "Should suggest reasonable position size")
	assert.True(t, result.Approved, "Should be approved")
}

func TestPortfolioAwareScorer_PositionCountPenalty(t *testing.T) {
	compositeScorer := &MockUnifiedScorer{}
	riskEnvelope := &MockRiskEnvelope{MaxPositions: 10}
	config := &portfolio.ConstraintConfig{MaxPositionSize: 0.10}

	scorer, err := portfolio.NewPortfolioAwareScorer(compositeScorer, riskEnvelope, config)
	require.NoError(t, err)

	input := composite.ScoringInput{
		Symbol:      "ETH-USD",
		Momentum1h:  0.06,
		VolumeSurge: 2.2,
		Regime:      "normal",
	}

	// Portfolio with many positions (8 positions - should trigger penalty)
	positions := make([]portfolio.Position, 8)
	for i := range positions {
		positions[i] = portfolio.Position{
			Symbol:      "TEST-USD",
			Size:        0.08,
			Sector:      "Layer1",
			Beta:        1.0,
			Correlation: 0.3,
			EntryTime:   time.Now().Add(-time.Duration(i*6) * time.Hour),
		}
	}

	portfolioState := &portfolio.PortfolioState{
		Positions:     positions,
		TotalValue:    1000000,
		SectorWeights: map[string]float64{"Layer1": 0.64},
		BetaExposure:  8.0,
		Drawdown:      0.06,
		Volatility:    0.18,
	}

	ctx := context.Background()
	result, err := scorer.ScoreWithConstraints(ctx, input, portfolioState)
	require.NoError(t, err)

	assert.True(t, result.AdjustedScore < result.BaseScore, "Score should be penalized")
	assert.Contains(t, result.Adjustments, "position_count_penalty", "Should have position count penalty")
	// Position count penalty only applies when >= 8 positions, so check if penalty exists
	if penalty, exists := result.Adjustments["position_count_penalty"]; exists {
		assert.True(t, penalty < 0, "Position count penalty should be negative if applied: %f", penalty)
	}
}

func TestPortfolioAwareScorer_DrawdownPenalty(t *testing.T) {
	compositeScorer := &MockUnifiedScorer{}
	riskEnvelope := &MockRiskEnvelope{MaxDrawdown: 0.12}
	config := &portfolio.ConstraintConfig{DrawdownThreshold: 0.12}

	scorer, err := portfolio.NewPortfolioAwareScorer(compositeScorer, riskEnvelope, config)
	require.NoError(t, err)

	input := composite.ScoringInput{
		Symbol:      "RISKY-USD",
		Momentum1h:  0.04,
		VolumeSurge: 1.8,
		Regime:      "normal",
	}

	// Portfolio in drawdown
	portfolioState := &portfolio.PortfolioState{
		Positions: []portfolio.Position{
			{Symbol: "DOWN1-USD", Size: 0.10, Sector: "Volatile", Beta: 1.5, EntryTime: time.Now().Add(-48 * time.Hour)},
		},
		TotalValue:    850000, // Down 15%
		SectorWeights: map[string]float64{"Volatile": 0.10},
		BetaExposure:  1.5,
		Drawdown:      0.15, // 15% drawdown - exceeds threshold of 12%
		Volatility:    0.28,
	}

	ctx := context.Background()
	result, err := scorer.ScoreWithConstraints(ctx, input, portfolioState)
	require.NoError(t, err)

	assert.True(t, result.AdjustedScore < result.BaseScore, "Score should be penalized for drawdown")
	assert.Contains(t, result.Adjustments, "drawdown_penalty", "Should have drawdown penalty")
	assert.True(t, result.Adjustments["drawdown_penalty"] < 0, "Drawdown penalty should be negative")
	assert.True(t, result.SuggestedSize < 0.05, "Position size should be reduced in drawdown")
}

func TestPortfolioAwareScorer_LiquidityPenalty(t *testing.T) {
	compositeScorer := &MockUnifiedScorer{}
	riskEnvelope := &MockRiskEnvelope{}
	config := &portfolio.ConstraintConfig{MinLiquidity: 100000}

	scorer, err := portfolio.NewPortfolioAwareScorer(compositeScorer, riskEnvelope, config)
	require.NoError(t, err)

	// Low volume surge = low liquidity
	input := composite.ScoringInput{
		Symbol:      "ILLIQUID-USD",
		Momentum1h:  0.06,
		VolumeSurge: 0.8, // Very low volume surge
		Regime:      "normal",
	}

	portfolioState := &portfolio.PortfolioState{
		Positions:     []portfolio.Position{},
		TotalValue:    1000000,
		SectorWeights: map[string]float64{},
		BetaExposure:  1.0,
		Drawdown:      0.03,
		Volatility:    0.16,
	}

	ctx := context.Background()
	result, err := scorer.ScoreWithConstraints(ctx, input, portfolioState)
	require.NoError(t, err)

	assert.True(t, result.AdjustedScore < result.BaseScore, "Score should be penalized for low liquidity")
	assert.Contains(t, result.Adjustments, "liquidity_penalty", "Should have liquidity penalty")
	assert.True(t, result.Adjustments["liquidity_penalty"] < 0, "Liquidity penalty should be negative")
}

func TestPortfolioAwareScorer_CorrelationPenalty(t *testing.T) {
	compositeScorer := &MockUnifiedScorer{}
	riskEnvelope := &MockRiskEnvelope{}
	config := &portfolio.ConstraintConfig{MaxCorrelation: 0.70}

	scorer, err := portfolio.NewPortfolioAwareScorer(compositeScorer, riskEnvelope, config)
	require.NoError(t, err)

	input := composite.ScoringInput{
		Symbol:      "CORR-USD",
		Momentum1h:  0.07,
		VolumeSurge: 2.0,
		Regime:      "normal",
	}

	// Portfolio with high-correlation positions
	portfolioState := &portfolio.PortfolioState{
		Positions: []portfolio.Position{
			{Symbol: "BTC-USD", Size: 0.10, Sector: "Layer1", Beta: 1.0, Correlation: 0.85, EntryTime: time.Now().Add(-24 * time.Hour)},
			{Symbol: "ETH-USD", Size: 0.08, Sector: "Layer1", Beta: 1.2, Correlation: 0.75, EntryTime: time.Now().Add(-48 * time.Hour)},
		},
		TotalValue:    1000000,
		SectorWeights: map[string]float64{"Layer1": 0.18},
		BetaExposure:  2.2,
		Drawdown:      0.05,
		Volatility:    0.16,
	}

	ctx := context.Background()
	result, err := scorer.ScoreWithConstraints(ctx, input, portfolioState)
	require.NoError(t, err)

	assert.True(t, result.AdjustedScore < result.BaseScore, "Score should be penalized for high correlation")
	assert.Contains(t, result.Adjustments, "correlation_penalty", "Should have correlation penalty")
	assert.True(t, result.Adjustments["correlation_penalty"] < 0, "Correlation penalty should be negative")
}