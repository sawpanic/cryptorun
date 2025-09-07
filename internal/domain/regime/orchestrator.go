package regime

import (
	"fmt"
	"time"

	"github.com/sawpanic/cryptorun/internal/domain/factors"
)

// RegimeOrchestrator coordinates regime detection with factor processing
type RegimeOrchestrator struct {
	detector       *RegimeDetector
	weightResolver *WeightResolver
	factorEngine   *factors.UnifiedFactorEngine
	lastUpdate     time.Time
}

// NewRegimeOrchestrator creates a new regime-aware factor processing orchestrator
func NewRegimeOrchestrator(detector *RegimeDetector, weightMap RegimeWeightMap) (*RegimeOrchestrator, error) {
	weightResolver := NewWeightResolver(weightMap, detector)

	// Initialize factor engine with current regime weights
	currentWeights, err := convertToFactorWeights(weightResolver.GetWeights())
	if err != nil {
		return nil, fmt.Errorf("failed to convert regime weights: %w", err)
	}

	// Use default market data for initialization
	defaultData := MarketData{Timestamp: time.Now()}
	currentRegime, err := detector.GetCurrentRegime(defaultData)
	if err != nil {
		return nil, fmt.Errorf("failed to get current regime: %w", err)
	}
	
	factorEngine, err := factors.NewUnifiedFactorEngine(currentRegime.String(), currentWeights)
	if err != nil {
		return nil, fmt.Errorf("failed to create factor engine: %w", err)
	}

	return &RegimeOrchestrator{
		detector:       detector,
		weightResolver: weightResolver,
		factorEngine:   factorEngine,
		lastUpdate:     time.Now(),
	}, nil
}

// ProcessFactorsWithRegimeAdaptation performs regime detection and factor processing
func (ro *RegimeOrchestrator) ProcessFactorsWithRegimeAdaptation(
	factorRows []factors.FactorRow,
	marketInputs RegimeInputs,
) ([]factors.FactorRow, error) {

	// Step 1: Update regime detection
	// Convert RegimeInputs to MarketData
	marketData := MarketData{
		Timestamp: marketInputs.Timestamp,
		// Add other fields as needed based on available data
	}
	
	previousRegime, err := ro.detector.GetCurrentRegime(marketData)
	if err != nil {
		return nil, fmt.Errorf("failed to get current regime: %w", err)
	}
	
	detection, err := ro.detector.DetectRegime(marketData)
	if err != nil {
		return nil, fmt.Errorf("failed to detect regime: %w", err)
	}
	currentRegime := detection.CurrentRegime

	// Step 2: Check if regime changed and update weights if necessary
	if currentRegime != previousRegime || time.Since(ro.lastUpdate) > time.Hour {
		if err := ro.updateFactorEngine(currentRegime); err != nil {
			return nil, fmt.Errorf("failed to update factor engine for regime %s: %w", currentRegime, err)
		}
		ro.lastUpdate = time.Now()
	}

	// Step 3: Process factors with current regime weights
	processedRows, err := ro.factorEngine.ProcessFactors(factorRows)
	if err != nil {
		return nil, fmt.Errorf("factor processing failed: %w", err)
	}

	return processedRows, nil
}

// updateFactorEngine updates the factor engine with new regime weights
func (ro *RegimeOrchestrator) updateFactorEngine(regime RegimeType) error {
	// Get weights for current regime
	regimeWeights := ro.weightResolver.GetWeightsForRegime(regime)

	// Convert to factor engine format
	factorWeights, err := convertToFactorWeights(regimeWeights)
	if err != nil {
		return fmt.Errorf("failed to convert regime weights: %w", err)
	}

	// Update factor engine
	if err := ro.factorEngine.SetRegime(regime.String(), factorWeights); err != nil {
		return fmt.Errorf("failed to set regime weights: %w", err)
	}

	return nil
}

// convertToFactorWeights converts regime factor weights to unified factor engine weights
func convertToFactorWeights(regimeWeights FactorWeights) (factors.RegimeWeights, error) {
	// Ensure weights sum to 100
	if err := ValidateFactorWeights(regimeWeights); err != nil {
		return factors.RegimeWeights{}, err
	}

	// Convert from 100-based to 1.0-based weights (excluding social cap)
	// Social factor is handled separately with +10 cap
	totalBase := regimeWeights.Momentum + regimeWeights.Technical +
		regimeWeights.Volume + regimeWeights.Quality + regimeWeights.Catalyst

	if totalBase == 0 {
		return factors.RegimeWeights{}, fmt.Errorf("total base weights cannot be zero")
	}

	// Normalize to 1.0 (social handled separately in factor engine with hard cap)
	return factors.RegimeWeights{
		MomentumCore:      regimeWeights.Momentum / 100.0,  // Protected momentum
		TechnicalResidual: regimeWeights.Technical / 100.0, // Technical residual
		VolumeResidual:    regimeWeights.Volume / 100.0,    // Volume residual
		QualityResidual:   regimeWeights.Quality / 100.0,   // Quality residual
		SocialResidual:    regimeWeights.Catalyst / 100.0,  // Map catalyst to social residual slot
	}, nil
}

// GetCurrentRegimeStatus returns current regime and factor processing status
func (ro *RegimeOrchestrator) GetCurrentRegimeStatus() map[string]interface{} {
	currentRegime := ro.detector.GetCurrentRegime()
	currentWeights := ro.weightResolver.GetWeights()
	factorWeights := ro.factorEngine.GetCurrentWeights()

	return map[string]interface{}{
		"regime": map[string]interface{}{
			"current":         currentRegime.String(),
			"last_detection":  ro.detector.GetLastUpdate().Format(time.RFC3339),
			"detector_status": ro.detector.GetDetectorStatus(),
		},
		"weights": map[string]interface{}{
			"regime_weights":     GetWeightAllocationSummary(currentRegime, currentWeights),
			"factor_weights":     factorWeights,
			"momentum_protected": GetMomentumProtectionStatus(currentWeights),
		},
		"factor_engine": map[string]interface{}{
			"current_regime": ro.factorEngine.GetCurrentRegime(),
			"social_cap":     10.0,
			"last_updated":   ro.lastUpdate.Format(time.RFC3339),
		},
	}
}

// GetRegimeHistory returns recent regime detection and weight changes
func (ro *RegimeOrchestrator) GetRegimeHistory() []map[string]interface{} {
	regimeHistory := ro.detector.GetRegimeHistory()
	history := make([]map[string]interface{}, len(regimeHistory))

	for i, inputs := range regimeHistory {
		// Simulate regime detection for historical data
		detector := NewRegimeDetector(ro.detector.thresholds)
		detectedRegime := detector.DetectRegime(inputs)
		weights := ro.weightResolver.GetWeightsForRegime(detectedRegime)

		history[i] = map[string]interface{}{
			"timestamp":       inputs.Timestamp.Format(time.RFC3339),
			"inputs":          inputs,
			"detected_regime": detectedRegime.String(),
			"weights":         weights,
		}
	}

	return history
}

// ValidateMarketInputs validates regime detection inputs
func (ro *RegimeOrchestrator) ValidateMarketInputs(inputs RegimeInputs) error {
	return ro.detector.ValidateInputs(inputs)
}

// GetOrthogonalityReport generates correlation matrix for factor orthogonality checking
func (ro *RegimeOrchestrator) GetOrthogonalityReport(factorRows []factors.FactorRow) map[string]interface{} {
	correlationMatrix := ro.factorEngine.GetCorrelationMatrix(factorRows)

	// Check orthogonality quality
	maxOffDiagonalCorr := 0.0
	for factor1, row := range correlationMatrix {
		for factor2, corr := range row {
			if factor1 != factor2 && corr > maxOffDiagonalCorr {
				maxOffDiagonalCorr = corr
			}
		}
	}

	return map[string]interface{}{
		"correlation_matrix":    correlationMatrix,
		"max_correlation":       maxOffDiagonalCorr,
		"orthogonality_quality": classifyOrthogonalityQuality(maxOffDiagonalCorr),
		"momentum_protected":    true, // MomentumCore always protected
		"social_capped":         true, // Social always capped at +10
	}
}

// classifyOrthogonalityQuality provides quality assessment for orthogonality
func classifyOrthogonalityQuality(maxCorr float64) string {
	switch {
	case maxCorr <= 0.1:
		return "Excellent"
	case maxCorr <= 0.2:
		return "Good"
	case maxCorr <= 0.3:
		return "Fair"
	case maxCorr <= 0.5:
		return "Poor"
	default:
		return "Unacceptable"
	}
}

// UpdateWeightMap updates the regime weight configuration
func (ro *RegimeOrchestrator) UpdateWeightMap(newWeightMap RegimeWeightMap) error {
	if err := ro.weightResolver.UpdateWeightMap(newWeightMap); err != nil {
		return fmt.Errorf("failed to update weight map: %w", err)
	}

	// Force update factor engine with current regime weights
	currentRegime := ro.detector.GetCurrentRegime()
	if err := ro.updateFactorEngine(currentRegime); err != nil {
		return fmt.Errorf("failed to update factor engine after weight map change: %w", err)
	}

	return nil
}

// GetWeightSensitivityAnalysis shows impact of regime changes on scoring
func (ro *RegimeOrchestrator) GetWeightSensitivityAnalysis() map[string]interface{} {
	allWeights := ro.weightResolver.GetAllWeights()

	analysis := map[string]interface{}{
		"regime_weight_differences": map[string]interface{}{
			"trending_vs_choppy": map[string]float64{
				"momentum_diff":  allWeights.TrendingBull.Momentum - allWeights.Choppy.Momentum,
				"technical_diff": allWeights.TrendingBull.Technical - allWeights.Choppy.Technical,
				"volume_diff":    allWeights.TrendingBull.Volume - allWeights.Choppy.Volume,
				"quality_diff":   allWeights.TrendingBull.Quality - allWeights.Choppy.Quality,
				"catalyst_diff":  allWeights.TrendingBull.Catalyst - allWeights.Choppy.Catalyst,
			},
			"high_vol_vs_choppy": map[string]float64{
				"momentum_diff":  allWeights.HighVol.Momentum - allWeights.Choppy.Momentum,
				"technical_diff": allWeights.HighVol.Technical - allWeights.Choppy.Technical,
				"volume_diff":    allWeights.HighVol.Volume - allWeights.Choppy.Volume,
				"quality_diff":   allWeights.HighVol.Quality - allWeights.Choppy.Quality,
				"catalyst_diff":  allWeights.HighVol.Catalyst - allWeights.Choppy.Catalyst,
			},
		},
		"momentum_protection": map[string]interface{}{
			"trending_bull": GetMomentumProtectionStatus(allWeights.TrendingBull),
			"choppy":        GetMomentumProtectionStatus(allWeights.Choppy),
			"high_vol":      GetMomentumProtectionStatus(allWeights.HighVol),
		},
		"social_cap_info": map[string]interface{}{
			"cap_value":            10.0,
			"applied_outside":      true,
			"never_orthogonalized": true,
		},
	}

	return analysis
}
