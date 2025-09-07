package composite

import (
	"context"
	"fmt"
	"time"

	"cryptorun/internal/application/pipeline"
	"cryptorun/internal/data/derivs"
	"cryptorun/internal/data/etf"
)

// Type aliases for pipeline integration
type RawFactors = pipeline.FactorSet
type RegimeWeights = pipeline.RegimeWeights

// CompositeScorer handles composite scoring with enhanced measurements
type CompositeScorer struct {
	DataMeasurements *DataMeasurements
}

// ScoreAsset performs basic composite scoring (stub for compilation)
func (cs *CompositeScorer) ScoreAsset(ctx context.Context, factors *RawFactors, weights *RegimeWeights) (*CompositeResult, error) {
	// Basic scoring stub - return simple result
	return &CompositeResult{
		FinalScore:    50.0,
		MomentumCore:  factors.MomentumCore,
		Regime:        "default",
		Timestamp:     time.Now(),
	}, nil
}


// DataMeasurements holds all new measurement data sources
type DataMeasurements struct {
	FundingProvider *derivs.FundingProvider
	OIProvider      *derivs.OpenInterestProvider
	ETFProvider     *etf.ETFFlowProvider
}

// NewDataMeasurements creates a new measurements container
func NewDataMeasurements() *DataMeasurements {
	return &DataMeasurements{
		FundingProvider: derivs.NewFundingProvider(),
		OIProvider:      derivs.NewOpenInterestProvider(),
		ETFProvider:     etf.NewETFFlowProvider(),
	}
}

// EnhancedRawFactors extends RawFactors with new measurement data
type EnhancedRawFactors struct {
	// Original factors
	MomentumCore float64 `json:"momentum_core"`
	Technical    float64 `json:"technical"`
	Volume       float64 `json:"volume"`
	Quality      float64 `json:"quality"`
	Social       float64 `json:"social"`

	// New measurement factors
	FundingDivergence bool    `json:"funding_divergence"` // Entry gate requirement
	FundingZ          float64 `json:"funding_z"`          // Z-score for attribution
	OIResidual        float64 `json:"oi_residual"`        // OI residual magnitude
	ETFTint           float64 `json:"etf_tint"`           // ETF flow tint

	// Attribution metadata
	HasFundingData bool   `json:"has_funding_data"`
	HasOIData      bool   `json:"has_oi_data"`
	HasETFData     bool   `json:"has_etf_data"`
	Source         string `json:"source"`
}

// GatherEnhancedFactors collects all factor data including new measurements
func (dm *DataMeasurements) GatherEnhancedFactors(ctx context.Context, symbol string, priceChange float64, rawFactors *RawFactors) (*EnhancedRawFactors, error) {
	enhanced := &EnhancedRawFactors{
		// Copy original factors
		MomentumCore: rawFactors.MomentumCore,
		Technical:    rawFactors.Technical,
		Volume:       rawFactors.Volume,
		Quality:      rawFactors.Quality,
		Social:       rawFactors.Social,
		Source:       "enhanced-pipeline",
	}

	// Gather funding data
	if fundingSnapshot, err := dm.FundingProvider.GetFundingSnapshot(ctx, symbol); err == nil {
		enhanced.FundingDivergence = fundingSnapshot.FundingDivergencePresent
		enhanced.FundingZ = fundingSnapshot.FundingZ
		enhanced.HasFundingData = true
	} else {
		// Log but don't fail - funding data is optional enhancement
		fmt.Printf("Warning: failed to get funding data for %s: %v\n", symbol, err)
		enhanced.HasFundingData = false
	}

	// Gather OI data
	if oiSnapshot, err := dm.OIProvider.GetOpenInterestSnapshot(ctx, symbol, priceChange); err == nil {
		enhanced.OIResidual = oiSnapshot.OIResidual
		enhanced.HasOIData = true
	} else {
		fmt.Printf("Warning: failed to get OI data for %s: %v\n", symbol, err)
		enhanced.HasOIData = false
	}

	// Gather ETF data
	if etfSnapshot, err := dm.ETFProvider.GetETFFlowSnapshot(ctx, symbol); err == nil {
		enhanced.ETFTint = etfSnapshot.FlowTint
		enhanced.HasETFData = true
	} else {
		fmt.Printf("Warning: failed to get ETF data for %s: %v\n", symbol, err)
		enhanced.HasETFData = false
	}

	return enhanced, nil
}

// CompositeResult represents the core scoring result structure
type CompositeResult struct {
	MomentumCore         float64   `json:"momentum_core"`
	TechnicalResid       float64   `json:"technical_resid"`
	VolumeResid          float64   `json:"volume_resid"`
	QualityResid         float64   `json:"quality_resid"`
	SocialResidCapped    float64   `json:"social_resid_capped"`
	FinalScore           float64   `json:"final_score"`
	FinalScoreWithSocial float64   `json:"final_score_with_social"`
	Regime               string    `json:"regime"`
	Timestamp            time.Time `json:"timestamp"`
}

// EnhancedCompositeResult extends CompositeResult with new measurement insights
type EnhancedCompositeResult struct {
	// Original composite result fields
	CompositeResult

	// New measurement insights
	FundingInsight string `json:"funding_insight"`
	OIInsight      string `json:"oi_insight"`
	ETFInsight     string `json:"etf_insight"`

	// Combined assessment
	MeasurementsBoost float64 `json:"measurements_boost"` // Additional boost from measurements
	DataQuality       string  `json:"data_quality"`       // Overall data completeness
}

// ScoreAssetWithMeasurements performs composite scoring enhanced with new measurements
func (cs *CompositeScorer) ScoreAssetWithMeasurements(ctx context.Context, enhanced *EnhancedRawFactors, weights *RegimeWeights, measurements *DataMeasurements) (*EnhancedCompositeResult, error) {
	// First perform standard composite scoring
	rawFactors := &RawFactors{
		MomentumCore: enhanced.MomentumCore,
		Technical:    enhanced.Technical,
		Volume:       enhanced.Volume,
		Quality:      enhanced.Quality,
		Social:       enhanced.Social,
	}

	baseResult, err := cs.ScoreAsset(ctx, rawFactors, weights)
	if err != nil {
		return nil, fmt.Errorf("base scoring failed: %w", err)
	}

	// Create enhanced result
	result := &EnhancedCompositeResult{
		CompositeResult: *baseResult,
	}

	// Apply measurement enhancements
	result.MeasurementsBoost = cs.calculateMeasurementsBoost(enhanced)
	result.FinalScoreWithSocial += result.MeasurementsBoost

	// Generate insights
	result.FundingInsight = cs.generateFundingInsight(enhanced)
	result.OIInsight = cs.generateOIInsight(enhanced)
	result.ETFInsight = cs.generateETFInsight(enhanced)
	result.DataQuality = cs.assessDataQuality(enhanced)

	return result, nil
}

// calculateMeasurementsBoost computes additional score boost from new measurements
func (cs *CompositeScorer) calculateMeasurementsBoost(enhanced *EnhancedRawFactors) float64 {
	var boost float64

	// Funding divergence boost: +2 if present with strong Z-score
	if enhanced.HasFundingData && enhanced.FundingDivergence {
		if abs(enhanced.FundingZ) >= 2.5 {
			boost += 2.0 // Strong divergence
		} else if abs(enhanced.FundingZ) >= 2.0 {
			boost += 1.0 // Moderate divergence
		}
	}

	// OI residual boost: +1.5 if significant residual activity
	if enhanced.HasOIData {
		absResidual := abs(enhanced.OIResidual)
		if absResidual >= 2_000_000 { // $2M+ residual
			boost += 1.5
		} else if absResidual >= 1_000_000 { // $1M+ residual
			boost += 0.5
		}
	}

	// ETF tint boost: +1 for strong directional flow
	if enhanced.HasETFData {
		absTint := abs(enhanced.ETFTint)
		if absTint >= 0.015 { // ±1.5% of ADV
			boost += 1.0
		} else if absTint >= 0.01 { // ±1% of ADV
			boost += 0.5
		}
	}

	// Cap total measurements boost at +4
	if boost > 4.0 {
		boost = 4.0
	}

	return boost
}

// generateFundingInsight creates human-readable funding insight
func (cs *CompositeScorer) generateFundingInsight(enhanced *EnhancedRawFactors) string {
	if !enhanced.HasFundingData {
		return "Funding data unavailable"
	}

	if enhanced.FundingDivergence {
		zAbs := abs(enhanced.FundingZ)
		direction := "neutral"
		if enhanced.FundingZ > 0 {
			direction = "premium"
		} else if enhanced.FundingZ < 0 {
			direction = "discount"
		}

		if zAbs >= 3.0 {
			return fmt.Sprintf("Very strong funding %s (%.1fσ)", direction, zAbs)
		} else if zAbs >= 2.5 {
			return fmt.Sprintf("Strong funding %s (%.1fσ)", direction, zAbs)
		} else if zAbs >= 2.0 {
			return fmt.Sprintf("Moderate funding %s (%.1fσ)", direction, zAbs)
		}
	}

	return "Funding rates normal"
}

// generateOIInsight creates human-readable OI insight
func (cs *CompositeScorer) generateOIInsight(enhanced *EnhancedRawFactors) string {
	if !enhanced.HasOIData {
		return "Open interest data unavailable"
	}

	residualAbs := abs(enhanced.OIResidual)
	direction := "neutral"
	if enhanced.OIResidual > 0 {
		direction = "buildup"
	} else if enhanced.OIResidual < 0 {
		direction = "reduction"
	}

	if residualAbs >= 5_000_000 {
		return fmt.Sprintf("Major OI %s ($%.1fM residual)", direction, residualAbs/1_000_000)
	} else if residualAbs >= 2_000_000 {
		return fmt.Sprintf("Significant OI %s ($%.1fM residual)", direction, residualAbs/1_000_000)
	} else if residualAbs >= 1_000_000 {
		return fmt.Sprintf("Moderate OI %s ($%.1fM residual)", direction, residualAbs/1_000_000)
	}

	return "OI activity normal"
}

// generateETFInsight creates human-readable ETF insight
func (cs *CompositeScorer) generateETFInsight(enhanced *EnhancedRawFactors) string {
	if !enhanced.HasETFData {
		return "ETF flow data unavailable"
	}

	tintAbs := abs(enhanced.ETFTint)
	direction := "neutral"
	if enhanced.ETFTint > 0 {
		direction = "inflow"
	} else if enhanced.ETFTint < 0 {
		direction = "outflow"
	}

	if tintAbs >= 0.018 {
		return fmt.Sprintf("Very strong ETF %s (%.1f%% of ADV)", direction, tintAbs*100)
	} else if tintAbs >= 0.015 {
		return fmt.Sprintf("Strong ETF %s (%.1f%% of ADV)", direction, tintAbs*100)
	} else if tintAbs >= 0.01 {
		return fmt.Sprintf("Moderate ETF %s (%.1f%% of ADV)", direction, tintAbs*100)
	}

	return "ETF flows balanced"
}

// assessDataQuality evaluates overall data completeness
func (cs *CompositeScorer) assessDataQuality(enhanced *EnhancedRawFactors) string {
	available := 0

	if enhanced.HasFundingData {
		available++
	}
	if enhanced.HasOIData {
		available++
	}
	if enhanced.HasETFData {
		available++
	}

	switch available {
	case 3:
		return "Complete (3/3 sources)"
	case 2:
		return "Good (2/3 sources)"
	case 1:
		return "Limited (1/3 sources)"
	default:
		return "Incomplete (0/3 sources)"
	}
}

// Helper function for absolute value
func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}
