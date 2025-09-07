package pipeline

import (
	"context"
	"fmt"
	"time"

	"github.com/sawpanic/cryptorun/internal/regime"
	"github.com/rs/zerolog/log"
)

// RegimeDetectorService handles regime detection and weight adaptation
type RegimeDetectorService struct {
	detector      *regime.Detector
	weightManager *regime.WeightManager
	lastUpdate    time.Time
	currentRegime regime.Regime
	lastResult    *regime.DetectionResult
}

// NewRegimeDetectorService creates a new regime detection service with mock inputs
func NewRegimeDetectorService() *RegimeDetectorService {
	// Initialize with mock inputs - in production this would use real market data
	inputs := regime.NewMockDetectorInputs()
	detector := regime.NewDetectorWithInputs(inputs)
	weightManager := regime.NewWeightManager()

	return &RegimeDetectorService{
		detector:      detector,
		weightManager: weightManager,
		currentRegime: regime.Choppy, // Safe default
	}
}

// NewRegimeDetectorServiceWithInputs creates a service with custom inputs (for testing)
func NewRegimeDetectorServiceWithInputs(inputs regime.DetectorInputs) *RegimeDetectorService {
	detector := regime.NewDetectorWithInputs(inputs)
	weightManager := regime.NewWeightManager()

	return &RegimeDetectorService{
		detector:      detector,
		weightManager: weightManager,
		currentRegime: regime.Choppy,
	}
}

// DetectAndUpdateRegime performs regime detection and updates weights if needed
func (rds *RegimeDetectorService) DetectAndUpdateRegime(ctx context.Context) (*regime.DetectionResult, error) {
	log.Debug().Msg("Starting regime detection")

	// Detect current regime
	result, err := rds.detector.DetectRegime(ctx)
	if err != nil {
		return nil, fmt.Errorf("regime detection failed: %w", err)
	}

	rds.lastResult = result
	rds.lastUpdate = time.Now()

	// Check if regime has changed
	if result.Regime != rds.currentRegime {
		previousRegime := rds.currentRegime
		rds.currentRegime = result.Regime

		// Update weight manager
		rds.weightManager.UpdateCurrentRegime(result)

		log.Info().
			Str("previous_regime", previousRegime.String()).
			Str("new_regime", result.Regime.String()).
			Float64("confidence", result.Confidence).
			Time("next_update", result.NextUpdate).
			Msg("Regime transition detected")
	} else {
		log.Debug().
			Str("regime", result.Regime.String()).
			Float64("confidence", result.Confidence).
			Bool("stable", result.IsStable).
			Msg("Regime unchanged")
	}

	return result, nil
}

// GetCurrentRegime returns the most recent regime classification
func (rds *RegimeDetectorService) GetCurrentRegime() regime.Regime {
	return rds.currentRegime
}

// GetCurrentRegimeString returns the regime as a string
func (rds *RegimeDetectorService) GetCurrentRegimeString() string {
	return rds.currentRegime.String()
}

// GetActiveWeights returns the current regime's factor weights
func (rds *RegimeDetectorService) GetActiveWeights() *regime.WeightPreset {
	return rds.weightManager.GetActiveWeights()
}

// GetDetectionResult returns the last detection result with full details
func (rds *RegimeDetectorService) GetDetectionResult() *regime.DetectionResult {
	return rds.lastResult
}

// GetRegimeConfidence returns the confidence level of current regime detection
func (rds *RegimeDetectorService) GetRegimeConfidence() float64 {
	if rds.lastResult != nil {
		return rds.lastResult.Confidence
	}
	return 0.0
}

// ShouldUpdate checks if a regime update is due based on 4h cadence
func (rds *RegimeDetectorService) ShouldUpdate(ctx context.Context) (bool, error) {
	return rds.detector.ShouldUpdate(ctx)
}

// GetRegimeWeightMapping returns the current regime's factor weight mapping
func (rds *RegimeDetectorService) GetRegimeWeightMapping() map[string]float64 {
	preset := rds.GetActiveWeights()
	if preset == nil {
		// Fallback weights if no preset available
		return map[string]float64{
			"momentum":  0.40,
			"technical": 0.25,
			"volume":    0.20,
			"quality":   0.10,
			"social":    0.05,
		}
	}

	// Convert regime weights to standard scoring weights
	return map[string]float64{
		"momentum":  preset.Weights["momentum_1h"] + preset.Weights["momentum_4h"] + preset.Weights["momentum_12h"] + preset.Weights["momentum_24h"] + preset.Weights["weekly_7d_carry"],
		"technical": preset.Weights["volatility_score"],
		"volume":    preset.Weights["volume_surge"],
		"quality":   preset.Weights["quality_score"],
		"social":    preset.Weights["social_sentiment"],
	}
}

// GetMovementGateConfig returns regime-specific movement detection settings
func (rds *RegimeDetectorService) GetMovementGateConfig() *regime.MovementGateConfig {
	preset := rds.GetActiveWeights()
	if preset == nil {
		// Default movement gate config
		return &regime.MovementGateConfig{
			MinMovementPercent:  5.0,
			TimeWindowHours:     48,
			VolumeSurgeRequired: true,
			TightenedThresholds: false,
		}
	}
	return &preset.MovementGate
}

// GetRegimeStatus returns comprehensive regime status for monitoring/debugging
func (rds *RegimeDetectorService) GetRegimeStatus() map[string]interface{} {
	status := map[string]interface{}{
		"current_regime":      rds.currentRegime.String(),
		"last_update":         rds.lastUpdate,
		"detection_available": rds.lastResult != nil,
	}

	if rds.lastResult != nil {
		status["confidence"] = rds.lastResult.Confidence
		status["is_stable"] = rds.lastResult.IsStable
		status["next_update"] = rds.lastResult.NextUpdate
		status["signals"] = rds.lastResult.Signals
		status["voting_breakdown"] = rds.lastResult.VotingBreakdown
		status["changes_since_start"] = rds.lastResult.ChangesSinceStart
	}

	// Include active weights
	preset := rds.GetActiveWeights()
	if preset != nil {
		status["active_weights"] = preset.Weights
		status["movement_gate"] = preset.MovementGate
		status["preset_name"] = preset.Name
		status["preset_description"] = preset.Description
	}

	return status
}

// ForceRegimeUpdate forces an immediate regime detection (bypasses 4h cadence)
func (rds *RegimeDetectorService) ForceRegimeUpdate(ctx context.Context) (*regime.DetectionResult, error) {
	log.Info().Msg("Forcing regime detection update")

	// Temporarily bypass the should update check by creating a new detector
	// In production, this might involve resetting the last update time
	result, err := rds.detector.DetectRegime(ctx)
	if err != nil {
		return nil, fmt.Errorf("forced regime detection failed: %w", err)
	}

	rds.lastResult = result
	rds.lastUpdate = time.Now()

	// Update regime if changed
	if result.Regime != rds.currentRegime {
		previousRegime := rds.currentRegime
		rds.currentRegime = result.Regime
		rds.weightManager.UpdateCurrentRegime(result)

		log.Info().
			Str("previous_regime", previousRegime.String()).
			Str("new_regime", result.Regime.String()).
			Float64("confidence", result.Confidence).
			Msg("Forced regime update completed")
	}

	return result, nil
}

// CreateRegimeServiceForTesting creates a service with predetermined regime for testing
func CreateRegimeServiceForTesting(targetRegime regime.Regime) *RegimeDetectorService {
	inputs := regime.NewMockDetectorInputsForRegime(targetRegime)
	return NewRegimeDetectorServiceWithInputs(inputs)
}
