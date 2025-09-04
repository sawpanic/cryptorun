package models

import "fmt"

// CleanOrthogonalSystem separates alpha factors from gates/constraints
// Fixes: 123.9% weight sum, collinearity, role confusion
type CleanOrthogonalSystem struct {
	// ALPHA LAYER: Additive factors that sum to 100%
	AlphaFactors AlphaWeights `json:"alpha_factors"`
	
	// GATE LAYER: Multiplicative validators [0,1] - NO additive weights
	Gates GateValidators `json:"gates"`
	
	// REGIME LAYER: Selects which weight vector to use
	RegimeSelector RegimeWeightSelector `json:"regime_selector"`
}

// AlphaWeights - Clean orthogonal factors summing to 100%
type AlphaWeights struct {
	// Core 5-factor system (recommended)
	QualityResidual      float64 `json:"quality_residual"`      // Quality with Technical stripped out
	VolumeLiquidityFused float64 `json:"volume_liquidity_fused"` // Volume confirmation + depth/spread 
	TechnicalResidual    float64 `json:"technical_residual"`     // Technical after Quality overlap removed
	OnChainResidual      float64 `json:"onchain_residual"`       // OnChain after overlap removed
	SocialResidual       float64 `json:"social_residual"`        // Social after all overlaps removed
	
	// Optional 6th factor (only if orthogonal)
	DerivativesOrthogonal float64 `json:"derivatives_orthogonal"` // Only if IC stays positive after residualization
}

// GateValidators - Multiplicative gates [0,1] - NOT additive
type GateValidators struct {
	LiquidityGate    LiquidityGateConfig    `json:"liquidity_gate"`
	CrossVenueGate   CrossVenueGateConfig   `json:"cross_venue_gate"`
	VolatilityGate   VolatilityGateConfig   `json:"volatility_gate"`
}

type LiquidityGateConfig struct {
	Enabled              bool    `json:"enabled"`
	DepthThresholdMultiple float64 `json:"depth_threshold_multiple"` // vs median depth
	SpreadThresholdMultiple float64 `json:"spread_threshold_multiple"` // vs median spread
	MinGateValue         float64 `json:"min_gate_value"`              // Floor at 0.4
	MaxGateValue         float64 `json:"max_gate_value"`              // Ceiling at 1.0
}

type CrossVenueGateConfig struct {
	Enabled                bool    `json:"enabled"`
	MaxReturnDivergenceBps float64 `json:"max_return_divergence_bps"` // Binance vs Kraken divergence
	MinVenueTrustScore     float64 `json:"min_venue_trust_score"`     // Trust score threshold
	LookbackMinutes        int     `json:"lookback_minutes"`          // Rolling window
}

type VolatilityGateConfig struct {
	Enabled          bool    `json:"enabled"`
	MaxVolatilityBps float64 `json:"max_volatility_bps"` // Cap momentum in extreme vol
	LookbackHours    int     `json:"lookback_hours"`
}

// RegimeWeightSelector - Picks weight vector, doesn't add weight
type RegimeWeightSelector struct {
	CurrentRegime    string                          `json:"current_regime"`    // BULL, BEAR, CHOP
	RegimeConfidence float64                         `json:"regime_confidence"` // [0,1]
	WeightVectors    map[string]AlphaWeights        `json:"weight_vectors"`    // Regime -> Weights
	RegimeIndicators map[string]float64             `json:"regime_indicators"` // Indicators used
}

// GetCleanOrthogonalWeights5Factor - Recommended 5-factor system (100% sum)
func GetCleanOrthogonalWeights5Factor() AlphaWeights {
	return AlphaWeights{
		QualityResidual:      0.35, // 35% - Quality with Technical stripped out
		VolumeLiquidityFused: 0.26, // 26% - Volume + Liquidity combined (no double count)
		TechnicalResidual:    0.18, // 18% - Technical after Quality overlap removed
		OnChainResidual:      0.12, // 12% - OnChain after overlaps removed  
		SocialResidual:       0.09, // 9%  - Social after all overlaps removed
		DerivativesOrthogonal: 0.0, // 0%  - Not used in 5-factor system
	}
}

// GetCleanOrthogonalWeights6Factor - Optional 6-factor system (100% sum)
func GetCleanOrthogonalWeights6Factor() AlphaWeights {
	return AlphaWeights{
		QualityResidual:       0.34, // 34% - Quality with Technical stripped out
		VolumeLiquidityFused:  0.24, // 24% - Volume + Liquidity combined
		TechnicalResidual:     0.15, // 15% - Technical after Quality overlap removed
		OnChainResidual:       0.10, // 10% - OnChain after overlaps removed
		SocialResidual:        0.10, // 10% - Social after all overlaps removed
		DerivativesOrthogonal: 0.07, // 7%  - Only if IC stays positive after residualization
	}
}

// GetMomentumOrthogonalWeights - Momentum-first orthogonal system (100% sum)
// Purpose: Favor breakout acceleration and social-driven moves while maintaining
// orthogonality and institutional guardrails.
// Rationale:
// - Increase TechnicalResidual to emphasize price velocity and breakouts
// - Increase SocialResidual to capture viral momentum
// - Keep VolumeLiquidityFused meaningful for confirmation and execution quality
// - Reduce QualityResidual to avoid biasing toward low-volatility majors
// - Maintain OnChainResidual for validation without over-weighting
func GetMomentumOrthogonalWeights() AlphaWeights {
    return AlphaWeights{
        QualityResidual:       0.15, // 15%  - Reduce quality bias (was 35%)
        VolumeLiquidityFused:  0.20, // 20%  - Keep volume/liquidity confirmation meaningful
        TechnicalResidual:     0.35, // 35%  - Emphasize momentum/breakouts (largest weight)
        OnChainResidual:       0.10, // 10%  - Validation of genuine flows
        SocialResidual:        0.20, // 20%  - Capture social/viral surges
        DerivativesOrthogonal: 0.00, // 0%   - Not used in this 5-factor profile
    }
}

// GetSocialWeightedOrthogonalWeights - Social-dominant orthogonal system (100% sum)
func GetSocialWeightedOrthogonalWeights() AlphaWeights {
	return AlphaWeights{
		QualityResidual:       0.18, // 18% - Quality foundation
		VolumeLiquidityFused:  0.12, // 12% - Volume confirmation
		TechnicalResidual:     0.05, // 5%  - Minimal technical noise
		OnChainResidual:       0.15, // 15% - OnChain validation signals
		SocialResidual:        0.50, // 50% - MAXIMUM social sentiment
		DerivativesOrthogonal: 0.0,  // 0%  - Not used in social mode
	}
}

// GetRegimeWeightVectors - Three regime-specific weight sets
func GetRegimeWeightVectors() map[string]AlphaWeights {
	base := GetCleanOrthogonalWeights5Factor()
	
	return map[string]AlphaWeights{
		"BULL": {
			QualityResidual:      base.QualityResidual - 0.10,      // 25% - Reduce quality in bull
			VolumeLiquidityFused: base.VolumeLiquidityFused + 0.09, // 35% - Boost momentum/volume  
			TechnicalResidual:    base.TechnicalResidual + 0.07,    // 25% - Boost technical
			OnChainResidual:      base.OnChainResidual + 0.03,      // 15% - Moderate onchain flows
			SocialResidual:       base.SocialResidual - 0.09,       // 0%  - Eliminate social noise
			DerivativesOrthogonal: 0.0,
		},
		"BEAR": {
			QualityResidual:      base.QualityResidual + 0.10,      // 45% - Emphasize quality
			VolumeLiquidityFused: base.VolumeLiquidityFused - 0.06, // 20% - Reduce momentum focus
			TechnicalResidual:    base.TechnicalResidual - 0.08,    // 10% - Reduce technical whipsaws
			OnChainResidual:      base.OnChainResidual + 0.03,      // 15% - Boost capitulation signals
			SocialResidual:       base.SocialResidual + 0.01,       // 10% - Moderate social
			DerivativesOrthogonal: 0.0,
		},
		"CHOP": {
			QualityResidual:      base.QualityResidual - 0.05,      // 30% - Moderate quality
			VolumeLiquidityFused: base.VolumeLiquidityFused - 0.11, // 15% - Low conviction moves
			TechnicalResidual:    base.TechnicalResidual - 0.08,    // 10% - Avoid whipsaws
			OnChainResidual:      base.OnChainResidual + 0.08,      // 20% - Real moves detection
			SocialResidual:       base.SocialResidual + 0.16,       // 25% - Sentiment drives chop
			DerivativesOrthogonal: 0.0,
		},
		"SOCIAL": {
			QualityResidual:      0.18,  // 18% - Quality foundation
			VolumeLiquidityFused: 0.12,  // 12% - Volume confirmation  
			TechnicalResidual:    0.05,  // 5%  - Minimal technical
			OnChainResidual:      0.15,  // 15% - OnChain validation
			SocialResidual:       0.50,  // 50% - MAXIMUM social sentiment
			DerivativesOrthogonal: 0.0,
		},
	}
}

// GetDefaultGates - Standard gate configurations
func GetDefaultGates() GateValidators {
	return GateValidators{
		LiquidityGate: LiquidityGateConfig{
			Enabled:                 true,
			DepthThresholdMultiple:  1.0,   // vs median depth
			SpreadThresholdMultiple: 1.0,   // vs median spread
			MinGateValue:           0.4,   // Floor
			MaxGateValue:           1.0,   // Ceiling
		},
		CrossVenueGate: CrossVenueGateConfig{
			Enabled:                true,
			MaxReturnDivergenceBps: 50.0,  // 0.5% max divergence
			MinVenueTrustScore:     0.8,   // 80% trust minimum
			LookbackMinutes:        15,    // 15-minute window
		},
		VolatilityGate: VolatilityGateConfig{
			Enabled:          true,
			MaxVolatilityBps: 500.0, // 5% volatility cap
			LookbackHours:    4,     // 4-hour volatility window
		},
	}
}

// ValidateCleanOrthogonalWeights ensures proper weight distribution
func ValidateCleanOrthogonalWeights(weights AlphaWeights, configName string) error {
	// Check sum equals 100%
	total := weights.QualityResidual + weights.VolumeLiquidityFused + 
			 weights.TechnicalResidual + weights.OnChainResidual + 
			 weights.SocialResidual + weights.DerivativesOrthogonal
	
	if total < 0.999 || total > 1.001 {
		return fmt.Errorf("CleanOrthogonalWeights for %s sum to %.3f, must sum to 1.000", configName, total)
	}
	
	// Check no factor exceeds 50% (anti-concentration)
	maxWeight := 0.5
	if weights.QualityResidual > maxWeight {
		return fmt.Errorf("QualityResidual %.3f exceeds maximum %.1f%%", weights.QualityResidual, maxWeight*100)
	}
	if weights.VolumeLiquidityFused > maxWeight {
		return fmt.Errorf("VolumeLiquidityFused %.3f exceeds maximum %.1f%%", weights.VolumeLiquidityFused, maxWeight*100)
	}
	
	// Check minimum 3 factors have meaningful weight (>1%)
	meaningfulFactors := 0
	minMeaningfulWeight := 0.01
	
	if weights.QualityResidual > minMeaningfulWeight { meaningfulFactors++ }
	if weights.VolumeLiquidityFused > minMeaningfulWeight { meaningfulFactors++ }
	if weights.TechnicalResidual > minMeaningfulWeight { meaningfulFactors++ }
	if weights.OnChainResidual > minMeaningfulWeight { meaningfulFactors++ }
	if weights.SocialResidual > minMeaningfulWeight { meaningfulFactors++ }
	if weights.DerivativesOrthogonal > minMeaningfulWeight { meaningfulFactors++ }
	
	if meaningfulFactors < 3 {
		return fmt.Errorf("CleanOrthogonalWeights for %s has only %d meaningful factors, minimum 3 required", configName, meaningfulFactors)
	}
	
	return nil
}

// CalculateCleanOrthogonalScore computes final score with gates
func CalculateCleanOrthogonalScore(opp ComprehensiveOpportunity, weights AlphaWeights, gates GateValidators) float64 {
	// Step 1: Calculate alpha score (additive)
	alphaScore := calculateAlphaScore(opp, weights)
	
	// Step 2: Apply gates (multiplicative)
	liquidityGate := calculateLiquidityGate(opp, gates.LiquidityGate)
	crossVenueGate := calculateCrossVenueGate(opp, gates.CrossVenueGate)
	volatilityGate := calculateVolatilityGate(opp, gates.VolatilityGate)
	
	// Final score: Alpha × Gates
	finalScore := alphaScore * liquidityGate * crossVenueGate * volatilityGate
	
	return finalScore
}

func calculateAlphaScore(opp ComprehensiveOpportunity, weights AlphaWeights) float64 {
	score := 0.0
	
	// Quality (residualized - Technical overlap removed)
	qualityResidual := extractQualityWithoutTechnical(opp)
	score += qualityResidual * weights.QualityResidual
	
	// Volume+Liquidity (fused - no double counting)
	volumeLiquidityFused := extractVolumeLiquidityFused(opp)
	score += volumeLiquidityFused * weights.VolumeLiquidityFused
	
	// Technical (residualized - Quality overlap removed)
	technicalResidual := extractTechnicalWithoutQuality(opp, qualityResidual)
	score += technicalResidual * weights.TechnicalResidual
	
	// OnChain (residualized)
	onChainResidual := extractOnChainResidual(opp, qualityResidual, volumeLiquidityFused)
	score += onChainResidual * weights.OnChainResidual
	
	// Social (residualized - all overlaps removed)
	socialResidual := extractSocialResidual(opp, qualityResidual, volumeLiquidityFused, technicalResidual)
	score += socialResidual * weights.SocialResidual
	
	// Derivatives (only if weight > 0)
	if weights.DerivativesOrthogonal > 0 {
		derivativesOrth := extractDerivativesOrthogonal(opp)
		score += derivativesOrth * weights.DerivativesOrthogonal
	}
	
	return score
}

// Placeholder extraction functions - implement residualization logic
func extractQualityWithoutTechnical(opp ComprehensiveOpportunity) float64 {
	// Remove Technical component from Quality to eliminate overlap
	rawQuality := opp.QualityScore
	if rawQuality == 0 {
		rawQuality = opp.CompositeScore
	}
	
	// Strip out technical component (estimated 30% overlap)
	technicalContamination := opp.TechnicalScore * 0.30
	qualityResidual := rawQuality - technicalContamination
	
	return max(qualityResidual * 1.3, 0.0) // Scale up residual
}

func extractVolumeLiquidityFused(opp ComprehensiveOpportunity) float64 {
	// Combine Volume + Liquidity into single composite (no double counting)
	volumeComponent := opp.VolumeConfirmationScore
	if volumeComponent == 0 {
		volumeComponent = opp.VolumeScore
	}
	
	liquidityComponent := opp.LiquidityScore
	
	// Fused composite: 70% volume patterns, 30% liquidity depth
	return (volumeComponent * 0.70) + (liquidityComponent * 0.30)
}

func extractTechnicalWithoutQuality(opp ComprehensiveOpportunity, qualityScore float64) float64 {
	// Remove Quality overlap from Technical
	rawTechnical := opp.TechnicalScore
	
	// Remove quality contamination (estimated 25% overlap)
	qualityContamination := qualityScore * 0.25
	technicalResidual := rawTechnical - qualityContamination
	
	return max(technicalResidual * 1.4, 0.0) // Scale up residual
}

func extractOnChainResidual(opp ComprehensiveOpportunity, qualityScore, volumeLiquidityScore float64) float64 {
	// Remove overlaps from OnChain
	rawOnChain := opp.OnChainScore
	if rawOnChain == 0 {
		rawOnChain = 50.0 // Default moderate activity
	}
	
	// Remove overlaps
	qualityContamination := qualityScore * 0.15
	volumeContamination := volumeLiquidityScore * 0.20
	
	onChainResidual := rawOnChain - qualityContamination - volumeContamination
	
	return max(onChainResidual * 1.6, 0.0) // Scale up residual
}

func extractSocialResidual(opp ComprehensiveOpportunity, qualityScore, volumeLiquidityScore, technicalScore float64) float64 {
	// Remove all overlaps from Social (heavily contaminated)
	rawSocial := opp.SentimentScore
	if rawSocial == 0 {
		rawSocial = 45.0 // Default moderate sentiment
	}
	
	// Remove overlaps (social correlates with everything)
	qualityContamination := qualityScore * 0.30
	volumeContamination := volumeLiquidityScore * 0.15
	technicalContamination := technicalScore * 0.20
	
	socialResidual := rawSocial - qualityContamination - volumeContamination - technicalContamination
	
	return max(socialResidual * 2.2, 0.0) // Heavy scaling due to high contamination
}

func extractDerivativesOrthogonal(opp ComprehensiveOpportunity) float64 {
	// Derivatives component - only if truly orthogonal after residualization
	return 50.0 // Placeholder - implement based on derivatives data
}

func calculateLiquidityGate(opp ComprehensiveOpportunity, config LiquidityGateConfig) float64 {
	if !config.Enabled {
		return 1.0
	}
	
	// Simplified liquidity gate calculation
	liquidityScore := opp.LiquidityScore / 100.0 // Convert to [0,1]
	
	// Apply thresholds
	gate := max(min(liquidityScore, config.MaxGateValue), config.MinGateValue)
	
	return gate
}

func calculateCrossVenueGate(opp ComprehensiveOpportunity, config CrossVenueGateConfig) float64 {
	if !config.Enabled {
		return 1.0
	}
	
	// Simplified cross-venue validation
	// In production: check price divergence across exchanges
	return 0.95 // Default high confidence
}

func calculateVolatilityGate(opp ComprehensiveOpportunity, config VolatilityGateConfig) float64 {
	if !config.Enabled {
		return 1.0
	}
	
	// Simplified volatility gate
	// In production: check if volatility exceeds threshold
	return 1.0 // Default no penalty
}

func max(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

func min(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}
