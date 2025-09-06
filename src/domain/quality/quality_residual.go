package quality

import (
	"fmt"
	"math"

	"cryptorun/src/domain/derivs"
)

// QualityResidualCalculator combines derivatives metrics into quality signals
type QualityResidualCalculator struct {
	config        QualityConfig
	derivsMetrics *derivs.DerivativesMetrics
}

// QualityConfig holds quality residual calculation parameters
type QualityConfig struct {
	// Weight configuration for quality blend
	FundingZScoreWeight   float64 `yaml:"funding_zscore"`    // w1
	DeltaOIResidualWeight float64 `yaml:"delta_oi_residual"` // w2
	BasisDispersionWeight float64 `yaml:"basis_dispersion"`  // w3

	// Clipping bounds for funding z-score
	FundingZClipMax float64 `yaml:"funding_z_clip_max"` // Upper clip bound
	FundingZClipMin float64 `yaml:"funding_z_clip_min"` // Lower clip bound (typically 0)

	// Normalization parameters
	OIResidualScale      float64 `yaml:"oi_residual_scale"`      // Scale factor for OI residual
	BasisDispersionScale float64 `yaml:"basis_dispersion_scale"` // Scale factor for basis signal
}

// QualitySignal represents the final quality residual output
type QualitySignal struct {
	Score         float64            `json:"score"`          // Final 0-100 score
	Components    QualityComponents  `json:"components"`     // Individual component scores
	SignalQuality string             `json:"signal_quality"` // Overall quality assessment
	Attribution   map[string]float64 `json:"attribution"`    // Component contributions
	Timestamp     int64              `json:"timestamp"`      // Unix timestamp
}

// QualityComponents breaks down individual quality factors
type QualityComponents struct {
	FundingStress      float64 `json:"funding_stress"`       // Clipped funding z-score component
	OIResidual         float64 `json:"oi_residual"`          // Delta OI residual component
	BasisDispersion    float64 `json:"basis_dispersion"`     // Basis dispersion component
	FundingZRaw        float64 `json:"funding_z_raw"`        // Raw funding z-score (pre-clip)
	OIResidualRaw      float64 `json:"oi_residual_raw"`      // Raw OI residual (pre-scale)
	BasisDispersionRaw float64 `json:"basis_dispersion_raw"` // Raw basis dispersion (pre-scale)
}

func NewQualityResidualCalculator(config QualityConfig, derivsMetrics *derivs.DerivativesMetrics) *QualityResidualCalculator {
	return &QualityResidualCalculator{
		config:        config,
		derivsMetrics: derivsMetrics,
	}
}

// Calculate computes the quality residual score from derivatives data
func (qrc *QualityResidualCalculator) Calculate(venueData []derivs.VenueData, oiData []derivs.OIPoint) (*QualitySignal, error) {
	if len(venueData) == 0 {
		return nil, fmt.Errorf("no venue data provided for quality calculation")
	}

	// Calculate funding z-score
	fundingResult, err := qrc.derivsMetrics.FundingZ(venueData)
	if err != nil {
		return nil, fmt.Errorf("funding z-score calculation failed: %w", err)
	}

	// Calculate delta OI residual
	var oiResult *derivs.DeltaOIResult
	if len(oiData) >= 10 { // Minimum observations for meaningful OI analysis
		oiResult, err = qrc.derivsMetrics.DeltaOIResidual(oiData)
		if err != nil {
			// OI analysis is optional - continue with warning
			oiResult = &derivs.DeltaOIResult{
				Residual:      0,
				SignalQuality: "insufficient_data",
			}
		}
	} else {
		oiResult = &derivs.DeltaOIResult{
			Residual:      0,
			SignalQuality: "insufficient_data",
		}
	}

	// Calculate basis dispersion
	basisResult, err := qrc.derivsMetrics.BasisDispersion(venueData)
	if err != nil {
		return nil, fmt.Errorf("basis dispersion calculation failed: %w", err)
	}

	// Process components
	components := qrc.processComponents(fundingResult, oiResult, basisResult)

	// Combine into final score
	score := qrc.combineScore(components)

	// Determine overall signal quality
	signalQuality := qrc.assessSignalQuality(fundingResult, oiResult, basisResult)

	// Calculate attribution
	attribution := qrc.calculateAttribution(components)

	return &QualitySignal{
		Score:         score,
		Components:    components,
		SignalQuality: signalQuality,
		Attribution:   attribution,
		Timestamp:     now(),
	}, nil
}

// processComponents transforms raw metrics into normalized components
func (qrc *QualityResidualCalculator) processComponents(
	funding *derivs.FundingZResult,
	oi *derivs.DeltaOIResult,
	basis *derivs.BasisDispersionResult) QualityComponents {

	// Process funding z-score (clip negative values, cap positive values)
	fundingZRaw := funding.ZScore
	fundingStress := math.Max(qrc.config.FundingZClipMin,
		math.Min(qrc.config.FundingZClipMax, -fundingZRaw)) // Negative because stress = negative funding

	// Process OI residual (scale and normalize)
	oiResidualRaw := oi.Residual
	oiResidual := math.Abs(oiResidualRaw) * qrc.config.OIResidualScale

	// Process basis dispersion (scale and normalize)
	basisDispersionRaw := basis.Dispersion
	basisDispersion := basisDispersionRaw * qrc.config.BasisDispersionScale

	return QualityComponents{
		FundingStress:      fundingStress,
		OIResidual:         oiResidual,
		BasisDispersion:    basisDispersion,
		FundingZRaw:        fundingZRaw,
		OIResidualRaw:      oiResidualRaw,
		BasisDispersionRaw: basisDispersionRaw,
	}
}

// combineScore blends components using configured weights
func (qrc *QualityResidualCalculator) combineScore(components QualityComponents) float64 {
	// Weighted combination: quality = w1*funding_stress + w2*oi_residual + w3*basis_dispersion
	rawScore := qrc.config.FundingZScoreWeight*components.FundingStress +
		qrc.config.DeltaOIResidualWeight*components.OIResidual +
		qrc.config.BasisDispersionWeight*components.BasisDispersion

	// Normalize to 0-100 range
	// Apply sigmoid-like transformation to handle extreme values gracefully
	normalizedScore := 100 * (1 - math.Exp(-rawScore/2))

	// Ensure bounds
	return math.Max(0, math.Min(100, normalizedScore))
}

// assessSignalQuality determines overall signal reliability
func (qrc *QualityResidualCalculator) assessSignalQuality(
	funding *derivs.FundingZResult,
	oi *derivs.DeltaOIResult,
	basis *derivs.BasisDispersionResult) string {

	qualityScore := 0

	// Funding quality assessment
	switch funding.DataQuality {
	case "high":
		qualityScore += 3
	case "medium":
		qualityScore += 2
	case "low_variance", "insufficient_history":
		qualityScore += 1
	}

	// OI quality assessment
	switch oi.SignalQuality {
	case "high":
		qualityScore += 2
	case "medium":
		qualityScore += 1
	case "low", "insufficient_data":
		qualityScore += 0
	}

	// Basis quality (simplified - based on venue count)
	if len(basis.VenueBasis) >= 3 {
		qualityScore += 2
	} else if len(basis.VenueBasis) >= 2 {
		qualityScore += 1
	}

	// Map quality score to categories
	switch {
	case qualityScore >= 6:
		return "high"
	case qualityScore >= 4:
		return "medium"
	case qualityScore >= 2:
		return "low"
	default:
		return "poor"
	}
}

// calculateAttribution shows contribution of each component to final score
func (qrc *QualityResidualCalculator) calculateAttribution(components QualityComponents) map[string]float64 {
	// Calculate raw contributions
	fundingContrib := qrc.config.FundingZScoreWeight * components.FundingStress
	oiContrib := qrc.config.DeltaOIResidualWeight * components.OIResidual
	basisContrib := qrc.config.BasisDispersionWeight * components.BasisDispersion

	total := fundingContrib + oiContrib + basisContrib

	attribution := make(map[string]float64)

	if total > 0 {
		attribution["funding_stress"] = fundingContrib / total
		attribution["oi_residual"] = oiContrib / total
		attribution["basis_dispersion"] = basisContrib / total
	} else {
		// Equal attribution if no signal
		attribution["funding_stress"] = 0.33
		attribution["oi_residual"] = 0.33
		attribution["basis_dispersion"] = 0.34
	}

	return attribution
}

// QualityResidualEngine orchestrates quality residual calculation with caching
type QualityResidualEngine struct {
	calculator *QualityResidualCalculator
	cache      QualityCache
}

// QualityCache interface for caching quality signals
type QualityCache interface {
	Get(symbol string) (*QualitySignal, bool)
	Set(symbol string, signal *QualitySignal, ttl int64)
	Invalidate(symbol string)
}

func NewQualityResidualEngine(calculator *QualityResidualCalculator, cache QualityCache) *QualityResidualEngine {
	return &QualityResidualEngine{
		calculator: calculator,
		cache:      cache,
	}
}

// GetQualitySignal retrieves quality signal with caching
func (qre *QualityResidualEngine) GetQualitySignal(symbol string, venueData []derivs.VenueData, oiData []derivs.OIPoint) (*QualitySignal, error) {
	// Check cache first
	if cached, found := qre.cache.Get(symbol); found {
		return cached, nil
	}

	// Calculate fresh signal
	signal, err := qre.calculator.Calculate(venueData, oiData)
	if err != nil {
		return nil, err
	}

	// Cache result (5 minutes TTL)
	qre.cache.Set(symbol, signal, 300)

	return signal, nil
}

// Helper functions

func now() int64 {
	return int64(1000) // Placeholder - should use time.Now().Unix()
}

// DefaultQualityConfig returns sensible defaults for quality residual calculation
func DefaultQualityConfig() QualityConfig {
	return QualityConfig{
		FundingZScoreWeight:   0.4,   // 40% weight to funding stress
		DeltaOIResidualWeight: 0.35,  // 35% weight to OI dynamics
		BasisDispersionWeight: 0.25,  // 25% weight to basis signals
		FundingZClipMax:       3.0,   // Cap z-scores at +3
		FundingZClipMin:       0.0,   // Only positive funding stress matters
		OIResidualScale:       100.0, // Scale OI residuals to 0-100 range
		BasisDispersionScale:  200.0, // Scale basis dispersion to 0-100 range
	}
}
