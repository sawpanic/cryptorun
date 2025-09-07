package portfolio

import (
	"context"
	"fmt"

	"github.com/sawpanic/cryptorun/internal/score/composite"
)

// RiskEnvelopeInterface defines the interface for risk envelope operations
type RiskEnvelopeInterface interface {
	GetMaxPositions() int
	GetMaxSectorWeight() float64
	GetMaxBetaBudget() float64
	GetMaxDrawdown() float64
	GetVolatilityTarget() float64
}

// CompositeScorerInterface defines the interface for composite scoring
type CompositeScorerInterface interface {
	Score(input composite.ScoringInput) composite.CompositeScore
}

// PortfolioAwareScorer wraps the unified composite scorer with portfolio-aware constraints
type PortfolioAwareScorer struct {
	compositeScorer  CompositeScorerInterface
	riskEnvelope     RiskEnvelopeInterface
	constraintConfig *ConstraintConfig
}

// PortfolioScoringResult contains portfolio-aware scoring results
type PortfolioScoringResult struct {
	BaseScore     float64            `json:"base_score"`
	AdjustedScore float64            `json:"adjusted_score"`
	Adjustments   map[string]float64 `json:"adjustments"`
	SuggestedSize float64            `json:"suggested_size"`
	Approved      bool               `json:"approved"`
	Reasons       []string           `json:"reasons"`
}

// NewPortfolioAwareScorer creates a new portfolio-aware scorer
func NewPortfolioAwareScorer(compositeScorer CompositeScorerInterface, riskEnvelope RiskEnvelopeInterface, config *ConstraintConfig) (*PortfolioAwareScorer, error) {
	if compositeScorer == nil {
		return nil, fmt.Errorf("composite scorer is required")
	}
	
	if config == nil {
		config = &ConstraintConfig{
			MaxPositionSize:   0.10,
			MaxSectorConc:     0.25,
			MaxCorrelation:    0.70,
			MinLiquidity:      100000,
			DrawdownThreshold: 0.12,
			BetaBudgetLimit:   2.5,
		}
	}

	return &PortfolioAwareScorer{
		compositeScorer:  compositeScorer,
		riskEnvelope:     riskEnvelope,
		constraintConfig: config,
	}, nil
}

// ScoreWithConstraints scores a position with portfolio-aware constraints
func (pas *PortfolioAwareScorer) ScoreWithConstraints(ctx context.Context, input composite.ScoringInput, portfolioState *PortfolioState) (*PortfolioScoringResult, error) {
	// Get base score from composite scorer
	baseResult := pas.compositeScorer.Score(input)
	baseScore := baseResult.FinalWithSocial
	adjustments := make(map[string]float64)
	
	// Apply portfolio constraints
	adjustedScore := baseScore
	
	// Position count penalty
	if len(portfolioState.Positions) >= 8 {
		penalty := -5.0 * float64(len(portfolioState.Positions)-7) // Penalize starting at 8 positions
		adjustments["position_count_penalty"] = penalty
		adjustedScore += penalty
	}
	
	// Correlation penalty
	if len(portfolioState.Positions) > 0 {
		avgCorrelation := 0.0
		for _, pos := range portfolioState.Positions {
			avgCorrelation += pos.Correlation
		}
		avgCorrelation /= float64(len(portfolioState.Positions))
		
		if avgCorrelation > pas.constraintConfig.MaxCorrelation {
			penalty := -20.0 * (avgCorrelation - pas.constraintConfig.MaxCorrelation)
			adjustments["correlation_penalty"] = penalty
			adjustedScore += penalty
		}
	}
	
	// Sector concentration check
	totalSectorWeight := 0.0
	for _, weight := range portfolioState.SectorWeights {
		totalSectorWeight += weight
	}
	if totalSectorWeight > pas.constraintConfig.MaxSectorConc {
		penalty := -15.0 * (totalSectorWeight - pas.constraintConfig.MaxSectorConc)
		adjustments["sector_concentration"] = penalty
		adjustedScore += penalty
	}
	
	// Drawdown penalty
	if portfolioState.Drawdown > pas.constraintConfig.DrawdownThreshold {
		penalty := -50.0 * (portfolioState.Drawdown - pas.constraintConfig.DrawdownThreshold)
		adjustments["drawdown_penalty"] = penalty
		adjustedScore += penalty
	}
	
	// Beta budget check
	if portfolioState.BetaExposure > pas.constraintConfig.BetaBudgetLimit {
		penalty := -10.0 * (portfolioState.BetaExposure - pas.constraintConfig.BetaBudgetLimit)
		adjustments["beta_penalty"] = penalty  
		adjustedScore += penalty
	}
	
	// Liquidity check (using volume surge as proxy - scale appropriately)
	volumeSurgeEstimate := input.VolumeSurge * 50000 // Rough estimate: 2.5 surge = ~125k liquidity
	if volumeSurgeEstimate < pas.constraintConfig.MinLiquidity {
		penalty := -25.0 * (1.0 - volumeSurgeEstimate/pas.constraintConfig.MinLiquidity)
		adjustments["liquidity_penalty"] = penalty
		adjustedScore += penalty
	}
	
	// Calculate suggested position size (simple Kelly approximation)
	suggestedSize := 0.05 // Default 5%
	if adjustedScore > 75 {
		suggestedSize = 0.08
	}
	if adjustedScore > 85 {
		suggestedSize = 0.10
	}
	
	// Reduce size based on negative factors
	if portfolioState.Drawdown > 0.10 {
		suggestedSize *= 0.5
	}
	if len(portfolioState.Positions) >= 8 {
		suggestedSize *= 0.7
	}
	
	// Cap at max position size
	if suggestedSize > pas.constraintConfig.MaxPositionSize {
		suggestedSize = pas.constraintConfig.MaxPositionSize
	}

	return &PortfolioScoringResult{
		BaseScore:     baseScore,
		AdjustedScore: adjustedScore,
		Adjustments:   adjustments,
		SuggestedSize: suggestedSize,
		Approved:      adjustedScore >= 65.0 && suggestedSize > 0.01,
		Reasons:       []string{},
	}, nil
}