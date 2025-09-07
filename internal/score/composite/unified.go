package composite

import (
	"context"
	"fmt"
	"math"
	"time"

	"cryptorun/internal/data/derivs"
	"cryptorun/internal/data/etf"
)

// CompositeScore represents the unified scoring result
type CompositeScore struct {
	// Raw components
	MomentumCore   float64           `json:"momentum_core"`
	TechnicalResid float64           `json:"technical_resid"`
	VolumeResid    VolumeComponents  `json:"volume_resid"`
	QualityResid   QualityComponents `json:"quality_resid"`
	SocialResid    float64           `json:"social_resid_capped"`

	// Final scores
	Internal0to100  float64 `json:"internal_0_100"`    // Normalized before social
	FinalWithSocial float64 `json:"final_with_social"` // Clamped [0,110]

	// Metadata
	Regime    string    `json:"regime"`
	Timestamp time.Time `json:"timestamp"`
	Symbol    string    `json:"symbol"`
}

// VolumeComponents breaks down volume-related scoring
type VolumeComponents struct {
	Volume   float64 `json:"volume"`   // Volume surge factor
	DeltaOI  float64 `json:"delta_oi"` // Open interest change
	Combined float64 `json:"combined"` // Weighted combination
}

// QualityComponents breaks down quality-related scoring
type QualityComponents struct {
	OIResid     float64 `json:"oi_resid"`     // OI residual after volume
	Reserves    float64 `json:"reserves"`     // Reserve health
	ETFTint     float64 `json:"etf_tint"`     // ETF flow tint
	VenueHealth float64 `json:"venue_health"` // Venue-specific health
	Combined    float64 `json:"combined"`     // Weighted combination
}

// ScoringInput contains all raw factors for scoring
type ScoringInput struct {
	Symbol    string
	Timestamp time.Time

	// Momentum factors (multi-timeframe)
	Momentum1h  float64
	Momentum4h  float64
	Momentum12h float64
	Momentum24h float64
	Momentum7d  float64

	// Technical factors
	RSI4h    float64
	ADX1h    float64
	HurstExp float64

	// Volume factors
	VolumeSurge float64
	DeltaOI     float64

	// Quality factors
	OIAbsolute   float64
	ReserveRatio float64
	ETFFlows     float64
	VenueHealth  float64

	// Social factors (applied after normalization)
	SocialScore float64
	BrandScore  float64

	// Market regime
	Regime string

	// Meta
	DataSources map[string]string
}


// UnifiedScorer implements the simplified unified composite scoring model
type UnifiedScorer struct {
	orthogonalizer  *Orthogonalizer
	normalizer      *Normalizer
	fundingProvider *derivs.FundingProvider
	oiProvider      *derivs.OpenInterestProvider
	etfProvider     *etf.ETFFlowProvider
}

// NewUnifiedScorer creates a new unified composite scorer
func NewUnifiedScorer() *UnifiedScorer {
	return &UnifiedScorer{
		orthogonalizer:  NewOrthogonalizer(),
		normalizer:      NewNormalizer(),
		fundingProvider: derivs.NewFundingProvider(),
		oiProvider:      derivs.NewOpenInterestProvider(),
		etfProvider:     etf.NewETFFlowProvider(),
	}
}

// Score calculates the unified composite score using the new model
func (us *UnifiedScorer) Score(input ScoringInput) CompositeScore {
	// Step 1: Calculate MomentumCore (protected factor)
	momentumCore := us.calculateMomentumCore(input)

	// Step 2: Build factors for orthogonalization
	factors := []Factor{
		{Name: "momentum_core", Values: []float64{momentumCore}, Protected: true},
		{Name: "technical", Values: us.calculateTechnicalFactors(input)},
		{Name: "volume", Values: us.calculateVolumeFactors(input)},
		{Name: "quality", Values: us.calculateQualityFactors(input)},
	}

	// Step 3: Orthogonalize using Gram-Schmidt
	orthogonalFactors, err := us.orthogonalizer.Orthogonalize(factors)
	if err != nil {
		// Fallback to non-orthogonalized scoring
		return us.fallbackScore(input)
	}

	// Step 4: Extract residuals and create components
	technicalResid := us.extractScalar(orthogonalFactors.TechnicalResid)
	volumeResid := us.extractVolumeComponents(orthogonalFactors.VolumeResid, input)
	qualityResid := us.extractQualityComponents(orthogonalFactors.QualityResid, input)

	// Step 5: Apply regime weights to compute 0-100 score
	weights, _ := us.normalizer.GetRegimeWeights(input.Regime)

	internal0to100 := weights["momentum_core"]*momentumCore +
		weights["technical_residual"]*technicalResid +
		(weights["supply_demand_block"]*weights["volume_weight"])*volumeResid.Combined +
		(weights["supply_demand_block"]*weights["quality_weight"])*qualityResid.Combined

	// Normalize to 0-100 scale
	internal0to100 = math.Max(0, math.Min(100, internal0to100))

	// Step 6: Apply social component AFTER normalization
	socialCombined := (input.SocialScore + input.BrandScore) / 2
	socialCapped := math.Min(10, math.Max(0, socialCombined)) // Cap at +10

	// Step 7: Final score with social, clamped to [0,110]
	finalWithSocial := math.Max(0, math.Min(110, internal0to100+socialCapped))

	return CompositeScore{
		MomentumCore:    momentumCore,
		TechnicalResid:  technicalResid,
		VolumeResid:     volumeResid,
		QualityResid:    qualityResid,
		SocialResid:     socialCapped,
		Internal0to100:  internal0to100,
		FinalWithSocial: finalWithSocial,
		Regime:          input.Regime,
		Timestamp:       input.Timestamp,
		Symbol:          input.Symbol,
	}
}

// calculateMomentumCore computes the protected momentum factor
func (us *UnifiedScorer) calculateMomentumCore(input ScoringInput) float64 {
	// Multi-timeframe momentum with regime-specific weights
	var weights map[string]float64

	switch input.Regime {
	case "trending_bull":
		// Bull: 24h 10-15%, 7d 5-10%
		weights = map[string]float64{"1h": 0.20, "4h": 0.35, "12h": 0.30, "24h": 0.12, "7d": 0.08}
	case "choppy":
		// Choppy: 24h 5-8%, 7d ≤2%
		weights = map[string]float64{"1h": 0.15, "4h": 0.30, "12h": 0.40, "24h": 0.07, "7d": 0.02}
	case "high_vol":
		// High vol: tighter focus on shorter timeframes
		weights = map[string]float64{"1h": 0.25, "4h": 0.40, "12h": 0.25, "24h": 0.10, "7d": 0.00}
	// Legacy regime mappings
	case "calm":
		weights = map[string]float64{"1h": 0.15, "4h": 0.30, "12h": 0.40, "24h": 0.07, "7d": 0.02}
	case "volatile":
		weights = map[string]float64{"1h": 0.25, "4h": 0.40, "12h": 0.25, "24h": 0.10, "7d": 0.00}
	default: // normal
		weights = map[string]float64{"1h": 0.20, "4h": 0.35, "12h": 0.30, "24h": 0.12, "7d": 0.08}
	}

	return weights["1h"]*input.Momentum1h +
		weights["4h"]*input.Momentum4h +
		weights["12h"]*input.Momentum12h +
		weights["24h"]*input.Momentum24h +
		weights["7d"]*input.Momentum7d
}

// calculateTechnicalFactors computes technical analysis components
func (us *UnifiedScorer) calculateTechnicalFactors(input ScoringInput) []float64 {
	return []float64{
		input.RSI4h / 100.0, // Normalize RSI to 0-1
		input.ADX1h / 100.0, // Normalize ADX to 0-1
		input.HurstExp,      // Already 0-1
	}
}

// calculateVolumeFactors computes volume-related components
func (us *UnifiedScorer) calculateVolumeFactors(input ScoringInput) []float64 {
	return []float64{
		input.VolumeSurge / 5.0, // Normalize to ~0-1 range
		input.DeltaOI,           // Already normalized
	}
}

// calculateQualityFactors computes quality/fundamental components
func (us *UnifiedScorer) calculateQualityFactors(input ScoringInput) []float64 {
	return []float64{
		input.OIAbsolute / 1000000.0, // Scale OI
		input.ReserveRatio,           // Already 0-1
		input.ETFFlows / 100000.0,    // Scale ETF flows
		input.VenueHealth,            // Already 0-1
	}
}

// extractScalar extracts a single scalar from a factor
func (us *UnifiedScorer) extractScalar(factor Factor) float64 {
	if len(factor.Values) == 0 {
		return 0.0
	}

	if len(factor.Values) == 1 {
		return factor.Values[0]
	}

	// Average multiple values
	sum := 0.0
	for _, v := range factor.Values {
		sum += v
	}
	return sum / float64(len(factor.Values))
}

// extractVolumeComponents creates volume breakdown from orthogonalized factor
func (us *UnifiedScorer) extractVolumeComponents(factor Factor, input ScoringInput) VolumeComponents {
	if len(factor.Values) < 2 {
		return VolumeComponents{Combined: us.extractScalar(factor)}
	}

	// Weight: 70% volume surge, 30% delta OI
	combined := 0.7*factor.Values[0] + 0.3*factor.Values[1]

	return VolumeComponents{
		Volume:   factor.Values[0],
		DeltaOI:  factor.Values[1],
		Combined: combined,
	}
}

// extractQualityComponents creates quality breakdown from orthogonalized factor
func (us *UnifiedScorer) extractQualityComponents(factor Factor, input ScoringInput) QualityComponents {
	if len(factor.Values) < 4 {
		return QualityComponents{Combined: us.extractScalar(factor)}
	}

	// Equal weights for quality components
	combined := (factor.Values[0] + factor.Values[1] + factor.Values[2] + factor.Values[3]) / 4.0

	return QualityComponents{
		OIResid:     factor.Values[0],
		Reserves:    factor.Values[1],
		ETFTint:     factor.Values[2],
		VenueHealth: factor.Values[3],
		Combined:    combined,
	}
}

// fallbackScore provides scoring when orthogonalization fails
func (us *UnifiedScorer) fallbackScore(input ScoringInput) CompositeScore {
	// Simple fallback scoring without orthogonalization
	momentum := us.calculateMomentumCore(input)
	social := math.Min(10, (input.SocialScore+input.BrandScore)/2)

	return CompositeScore{
		MomentumCore:    momentum,
		TechnicalResid:  0,
		VolumeResid:     VolumeComponents{Combined: 0},
		QualityResid:    QualityComponents{Combined: 0},
		SocialResid:     social,
		Internal0to100:  momentum * 100, // Simple fallback
		FinalWithSocial: math.Min(110, momentum*100+social),
		Regime:          input.Regime,
		Timestamp:       input.Timestamp,
		Symbol:          input.Symbol,
	}
}

// Validate ensures the composite score meets all requirements
func (score *CompositeScore) Validate() error {
	if score.Internal0to100 < 0 || score.Internal0to100 > 100 {
		return fmt.Errorf("internal score %.2f outside [0,100] range", score.Internal0to100)
	}

	if score.FinalWithSocial < 0 || score.FinalWithSocial > 110 {
		return fmt.Errorf("final score %.2f outside [0,110] range", score.FinalWithSocial)
	}

	if score.SocialResid < 0 || score.SocialResid > 10 {
		return fmt.Errorf("social residual %.2f outside [0,10] cap", score.SocialResid)
	}

	return nil
}

// ScoreWithMeasurements calculates unified score enhanced with new measurement data
func (us *UnifiedScorer) ScoreWithMeasurements(ctx context.Context, input ScoringInput) (*EnhancedCompositeResult, error) {
	// First get base composite score
	baseScore := us.Score(input)

	// Gather measurement data
	fundingSnapshot, fundingErr := us.fundingProvider.GetFundingSnapshot(ctx, input.Symbol)
	oiSnapshot, oiErr := us.oiProvider.GetOpenInterestSnapshot(ctx, input.Symbol, 0.0) // Price change from input if available
	etfSnapshot, etfErr := us.etfProvider.GetETFFlowSnapshot(ctx, input.Symbol)

	// Create enhanced result
	result := &EnhancedCompositeResult{
		CompositeResult: CompositeResult{
			MomentumCore:         baseScore.MomentumCore,
			TechnicalResid:       baseScore.TechnicalResid,
			VolumeResid:          baseScore.VolumeResid.Combined,
			QualityResid:         baseScore.QualityResid.Combined,
			SocialResidCapped:    baseScore.SocialResid,
			FinalScore:           baseScore.Internal0to100,
			FinalScoreWithSocial: baseScore.FinalWithSocial,
			Regime:               baseScore.Regime,
			Timestamp:            baseScore.Timestamp,
		},
		MeasurementsBoost: 0.0,
		DataQuality:       "unknown",
	}

	// Apply funding measurements
	if fundingErr == nil {
		result.FundingInsight = us.generateFundingInsight(fundingSnapshot)
		if fundingSnapshot.FundingDivergencePresent {
			zAbs := math.Abs(fundingSnapshot.FundingZ)
			if zAbs >= 2.5 {
				result.MeasurementsBoost += 2.0 // Strong funding divergence
			} else if zAbs >= 2.0 {
				result.MeasurementsBoost += 1.0 // Moderate funding divergence
			}
		}
	} else {
		result.FundingInsight = "Funding data unavailable"
	}

	// Apply OI measurements
	if oiErr == nil {
		result.OIInsight = us.generateOIInsight(oiSnapshot)
		absResidual := math.Abs(oiSnapshot.OIResidual)
		if absResidual >= 2_000_000 {
			result.MeasurementsBoost += 1.5 // Strong OI activity
		} else if absResidual >= 1_000_000 {
			result.MeasurementsBoost += 0.5 // Moderate OI activity
		}
	} else {
		result.OIInsight = "OI data unavailable"
	}

	// Apply ETF measurements
	if etfErr == nil {
		result.ETFInsight = us.generateETFInsight(etfSnapshot)
		absTint := math.Abs(etfSnapshot.FlowTint)
		if absTint >= 0.015 {
			result.MeasurementsBoost += 1.0 // Strong ETF flow
		} else if absTint >= 0.01 {
			result.MeasurementsBoost += 0.5 // Moderate ETF flow
		}
	} else {
		result.ETFInsight = "ETF data unavailable"
	}

	// Cap total measurements boost at +4
	if result.MeasurementsBoost > 4.0 {
		result.MeasurementsBoost = 4.0
	}

	// Apply boost to final scores
	result.FinalScore += result.MeasurementsBoost
	result.FinalScoreWithSocial += result.MeasurementsBoost

	// Ensure scores stay within bounds
	result.FinalScore = math.Max(0, math.Min(100, result.FinalScore))
	result.FinalScoreWithSocial = math.Max(0, math.Min(114, result.FinalScoreWithSocial)) // 110 + 4 max boost

	// Assess data quality
	result.DataQuality = us.assessDataQuality(fundingErr, oiErr, etfErr)

	return result, nil
}

// generateFundingInsight creates funding insight from snapshot
func (us *UnifiedScorer) generateFundingInsight(snapshot *derivs.FundingSnapshot) string {
	if !snapshot.FundingDivergencePresent {
		return "Funding rates normal"
	}

	zAbs := math.Abs(snapshot.FundingZ)
	direction := "premium"
	if snapshot.FundingZ < 0 {
		direction = "discount"
	}

	if zAbs >= 3.0 {
		return fmt.Sprintf("Very strong funding %s (%.1fσ)", direction, zAbs)
	} else if zAbs >= 2.5 {
		return fmt.Sprintf("Strong funding %s (%.1fσ)", direction, zAbs)
	} else if zAbs >= 2.0 {
		return fmt.Sprintf("Moderate funding %s (%.1fσ)", direction, zAbs)
	}

	return "Funding rates normal"
}

// generateOIInsight creates OI insight from snapshot
func (us *UnifiedScorer) generateOIInsight(snapshot *derivs.OpenInterestSnapshot) string {
	residualAbs := math.Abs(snapshot.OIResidual)
	direction := "buildup"
	if snapshot.OIResidual < 0 {
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

// generateETFInsight creates ETF insight from snapshot
func (us *UnifiedScorer) generateETFInsight(snapshot *etf.ETFSnapshot) string {
	tintAbs := math.Abs(snapshot.FlowTint)
	direction := "inflow"
	if snapshot.FlowTint < 0 {
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

// assessDataQuality evaluates overall data completeness from errors
func (us *UnifiedScorer) assessDataQuality(fundingErr, oiErr, etfErr error) string {
	available := 0
	if fundingErr == nil {
		available++
	}
	if oiErr == nil {
		available++
	}
	if etfErr == nil {
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
