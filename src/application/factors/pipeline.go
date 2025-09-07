package factors

import (
	"fmt"

	"github.com/sawpanic/cryptorun/internal/domain/factors"
	"github.com/sawpanic/cryptorun/internal/domain/regime"
	"github.com/sawpanic/cryptorun/src/domain/momentum"
)

// MomentumCoreIntegration handles integration of MomentumCore with the unified factor pipeline
type MomentumCoreIntegration struct {
	factorEngine *factors.UnifiedFactorEngine
}

// NewMomentumCoreIntegration creates a new momentum core integration
func NewMomentumCoreIntegration(engine *factors.UnifiedFactorEngine) *MomentumCoreIntegration {
	return &MomentumCoreIntegration{
		factorEngine: engine,
	}
}

// ProcessWithMomentumCore processes factor rows ensuring MomentumCore is protected and populated first
func (mci *MomentumCoreIntegration) ProcessWithMomentumCore(
	factorRows []factors.FactorRow,
	momentumInputs []momentum.CoreInputs,
	currentRegime regime.RegimeType,
	useCarry bool,
	accelBoost float64,
) ([]factors.FactorRow, error) {

	if len(factorRows) != len(momentumInputs) {
		return nil, fmt.Errorf("mismatch between factor rows (%d) and momentum inputs (%d)",
			len(factorRows), len(momentumInputs))
	}

	// Step 1: Populate MomentumCore as the protected base vector
	processedRows := make([]factors.FactorRow, len(factorRows))
	weights := momentum.WeightsForRegime(currentRegime)

	for i, row := range factorRows {
		processedRows[i] = row

		// Calculate MomentumCore score for this symbol
		result := momentum.ComputeCore(momentumInputs[i], weights, useCarry, accelBoost, true)

		// Set MomentumCore as the protected factor (never residualized)
		processedRows[i].MomentumCore = result.Score

		// Validate MomentumCore is properly set
		if processedRows[i].MomentumCore < 0 || processedRows[i].MomentumCore > 100 {
			return nil, fmt.Errorf("invalid MomentumCore score for %s: %.2f (must be 0-100)",
				row.Symbol, processedRows[i].MomentumCore)
		}
	}

	// Step 2: Process through unified factor engine (MomentumCore protected by design)
	finalRows, err := mci.factorEngine.ProcessFactors(processedRows)
	if err != nil {
		return nil, fmt.Errorf("factor engine processing failed: %w", err)
	}

	// Step 3: Verify MomentumCore protection was maintained
	for i, row := range finalRows {
		if row.MomentumCore != processedRows[i].MomentumCore {
			return nil, fmt.Errorf("MomentumCore protection violated for %s: expected %.6f, got %.6f",
				row.Symbol, processedRows[i].MomentumCore, row.MomentumCore)
		}
	}

	return finalRows, nil
}

// ValidateFactorPipeline ensures the factor pipeline maintains MomentumCore protection
func ValidateFactorPipeline(engine *factors.UnifiedFactorEngine) error {
	// Check that MomentumCore is listed as protected in the orthogonalization order
	order := factors.DefaultOrthogonalizationOrder

	// Verify MomentumCore is in protected list
	momentumProtected := false
	for _, protected := range order.Protected {
		if protected == "MomentumCore" {
			momentumProtected = true
			break
		}
	}

	if !momentumProtected {
		return fmt.Errorf("MomentumCore must be protected in orthogonalization order")
	}

	// Verify MomentumCore is NOT in the residualization sequence
	for _, factor := range order.Sequence {
		if factor == "MomentumCore" {
			return fmt.Errorf("MomentumCore must not be in residualization sequence")
		}
	}

	// Verify proper factor ordering in sequence
	expectedSequence := []string{"TechnicalFactor", "VolumeFactor", "QualityFactor", "SocialFactor"}
	if len(order.Sequence) != len(expectedSequence) {
		return fmt.Errorf("expected %d factors in sequence, got %d",
			len(expectedSequence), len(order.Sequence))
	}

	for i, expected := range expectedSequence {
		if order.Sequence[i] != expected {
			return fmt.Errorf("factor sequence mismatch at position %d: expected %s, got %s",
				i, expected, order.Sequence[i])
		}
	}

	return nil
}

// GetFactorPipelineStatus returns the current status of factor pipeline integration
func GetFactorPipelineStatus(engine *factors.UnifiedFactorEngine) map[string]interface{} {
	order := factors.DefaultOrthogonalizationOrder

	status := map[string]interface{}{
		"momentum_core": map[string]interface{}{
			"protected":          true,
			"position":           "first",
			"never_residualized": true,
		},
		"orthogonalization_order": map[string]interface{}{
			"protected_factors": order.Protected,
			"sequence":          order.Sequence,
		},
		"factor_hierarchy": []string{
			"MomentumCore (protected)",
			"TechnicalResidual (vs MomentumCore)",
			"VolumeResidual (vs MomentumCore, Technical)",
			"QualityResidual (vs MomentumCore, Technical, Volume)",
			"SocialResidual (vs all previous, capped at ±10)",
		},
		"integration": map[string]interface{}{
			"regime_adaptive": true,
			"multi_timeframe": true,
			"atr_normalized":  true,
			"accel_boost":     true,
		},
	}

	return status
}

// FactorRowBuilder helps build factor rows with proper MomentumCore integration
type FactorRowBuilder struct {
	regime     regime.RegimeType
	useCarry   bool
	accelBoost float64
	atrNorm    bool
}

// NewFactorRowBuilder creates a new factor row builder
func NewFactorRowBuilder(regime regime.RegimeType, useCarry bool, accelBoost float64) *FactorRowBuilder {
	return &FactorRowBuilder{
		regime:     regime,
		useCarry:   useCarry,
		accelBoost: accelBoost,
		atrNorm:    true, // Always use ATR normalization
	}
}

// BuildFactorRow creates a factor row with MomentumCore properly calculated
func (frb *FactorRowBuilder) BuildFactorRow(
	symbol string,
	momentumInputs momentum.CoreInputs,
	technicalScore, volumeScore, qualityScore, socialScore float64,
) (factors.FactorRow, error) {

	// Validate momentum inputs
	if err := momentum.ValidateInputs(momentumInputs); err != nil {
		return factors.FactorRow{}, fmt.Errorf("invalid momentum inputs for %s: %w", symbol, err)
	}

	// Calculate MomentumCore
	weights := momentum.WeightsForRegime(frb.regime)
	result := momentum.ComputeCore(momentumInputs, weights, frb.useCarry, frb.accelBoost, frb.atrNorm)

	// Create factor row with MomentumCore set first
	row := factors.FactorRow{
		Symbol:          symbol,
		Timestamp:       momentumInputs.Timestamp,
		MomentumCore:    result.Score, // Protected factor - never residualized
		TechnicalFactor: technicalScore,
		VolumeFactor:    volumeScore,
		QualityFactor:   qualityScore,
		SocialFactor:    socialScore,

		// Residuals will be calculated by UnifiedFactorEngine
		TechnicalResidual: 0.0,
		VolumeResidual:    0.0,
		QualityResidual:   0.0,
		SocialResidual:    0.0,

		// Composite score calculated later
		CompositeScore: 0.0,
		Rank:           0,
		Selected:       false,
	}

	return row, nil
}
