package regime

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// API provides the main interface for regime detection and weight management
type API struct {
	detector      *Detector
	weightManager *WeightManager
	mu            sync.RWMutex
	lastCheck     time.Time
	updateTimer   *time.Timer
	isRunning     bool
}

// NewAPI creates a new regime detection API
func NewAPI(inputs DetectorInputs) *API {
	detector := NewDetectorWithInputs(inputs)
	weightManager := NewWeightManager()

	return &API{
		detector:      detector,
		weightManager: weightManager,
	}
}

// NewAPIWithConfig creates an API with custom configuration
func NewAPIWithConfig(inputs DetectorInputs, config DetectorConfig) *API {
	detector := NewDetectorWithConfig(inputs, config)
	weightManager := NewWeightManager()

	return &API{
		detector:      detector,
		weightManager: weightManager,
	}
}

// Start begins the 4-hour regime detection cycle
func (api *API) Start(ctx context.Context) error {
	api.mu.Lock()
	defer api.mu.Unlock()

	if api.isRunning {
		return fmt.Errorf("regime API is already running")
	}

	// Initial detection
	result, err := api.detector.DetectRegime(ctx)
	if err != nil {
		return fmt.Errorf("initial regime detection failed: %w", err)
	}

	// Update weights based on initial detection
	api.weightManager.UpdateCurrentRegime(result)
	api.lastCheck = time.Now()
	api.isRunning = true

	// Schedule next update
	api.scheduleNextUpdate(ctx)

	return nil
}

// Stop halts the regime detection cycle
func (api *API) Stop() {
	api.mu.Lock()
	defer api.mu.Unlock()

	if api.updateTimer != nil {
		api.updateTimer.Stop()
		api.updateTimer = nil
	}

	api.isRunning = false
}

// GetActiveWeights returns the current regime's factor weights
func (api *API) GetActiveWeights() *WeightPreset {
	api.mu.RLock()
	defer api.mu.RUnlock()

	return api.weightManager.GetActiveWeights()
}

// GetCurrentRegime returns the currently detected regime
func (api *API) GetCurrentRegime(ctx context.Context) (Regime, error) {
	api.mu.RLock()
	defer api.mu.RUnlock()

	return api.detector.GetCurrentRegime(ctx)
}

// ForceUpdate manually triggers regime detection (ignores 4h interval)
func (api *API) ForceUpdate(ctx context.Context) (*DetectionResult, error) {
	api.mu.Lock()
	defer api.mu.Unlock()

	// Temporarily bypass interval check
	api.detector.lastUpdate = time.Time{}

	result, err := api.detector.DetectRegime(ctx)
	if err != nil {
		return nil, fmt.Errorf("forced regime update failed: %w", err)
	}

	// Update weights if regime changed
	if api.weightManager.GetCurrentRegime() != result.Regime {
		api.weightManager.UpdateCurrentRegime(result)
	}

	api.lastCheck = time.Now()

	// Reschedule next regular update
	api.scheduleNextUpdate(ctx)

	return result, nil
}

// GetDetectionResult returns the most recent detection result
func (api *API) GetDetectionResult() *DetectionResult {
	api.mu.RLock()
	defer api.mu.RUnlock()

	return api.weightManager.GetLastUpdate()
}

// GetAllWeightPresets returns all available regime weight presets
func (api *API) GetAllWeightPresets() map[Regime]*WeightPreset {
	return api.weightManager.GetAllPresets()
}

// GetRegimeHistory returns the regime change history
func (api *API) GetRegimeHistory() []RegimeChange {
	api.mu.RLock()
	defer api.mu.RUnlock()

	return api.detector.GetDetectionHistory()
}

// GetAPIStatus returns the current status of the regime API
func (api *API) GetAPIStatus() map[string]interface{} {
	api.mu.RLock()
	defer api.mu.RUnlock()

	status := map[string]interface{}{
		"is_running":     api.isRunning,
		"last_check":     api.lastCheck.Format(time.RFC3339),
		"current_regime": api.weightManager.GetCurrentRegime().String(),
	}

	if api.weightManager.GetLastUpdate() != nil {
		result := api.weightManager.GetLastUpdate()
		status["last_detection"] = map[string]interface{}{
			"regime":        result.Regime.String(),
			"confidence":    result.Confidence,
			"is_stable":     result.IsStable,
			"changes_count": result.ChangesSinceStart,
			"last_update":   result.LastUpdate.Format(time.RFC3339),
			"next_update":   result.NextUpdate.Format(time.RFC3339),
		}
	}

	return status
}

// GetFactorWeightTable returns a formatted table of all regime weights
func (api *API) GetFactorWeightTable() map[string]interface{} {
	api.mu.RLock()
	defer api.mu.RUnlock()

	presets := api.weightManager.GetAllPresets()

	// Get all unique factors
	allFactors := make(map[string]bool)
	for _, preset := range presets {
		for factor := range preset.Weights {
			allFactors[factor] = true
		}
	}

	// Build comparison table
	factorList := make([]string, 0, len(allFactors))
	for factor := range allFactors {
		factorList = append(factorList, factor)
	}

	table := map[string]interface{}{
		"factors": factorList,
		"regimes": map[string]map[string]float64{},
		"metadata": map[string]interface{}{
			"active_regime": api.weightManager.GetCurrentRegime().String(),
			"last_update":   api.lastCheck.Format(time.RFC3339),
		},
	}

	// Populate weight data for each regime
	for regime, preset := range presets {
		regimeWeights := make(map[string]float64)
		for _, factor := range factorList {
			if weight, exists := preset.Weights[factor]; exists {
				regimeWeights[factor] = weight
			} else {
				regimeWeights[factor] = 0.0
			}
		}
		table["regimes"].(map[string]map[string]float64)[regime.String()] = regimeWeights
	}

	return table
}

// ValidateConfiguration checks that all regime presets have valid weights
func (api *API) ValidateConfiguration() error {
	api.mu.RLock()
	defer api.mu.RUnlock()

	for regime := range api.weightManager.GetAllPresets() {
		if err := api.weightManager.ValidateWeights(regime); err != nil {
			return fmt.Errorf("validation failed for regime %s: %w", regime.String(), err)
		}
	}

	return nil
}

// GetMovementGates returns movement gate configuration for current regime
func (api *API) GetMovementGates() MovementGateConfig {
	api.mu.RLock()
	defer api.mu.RUnlock()

	activeWeights := api.weightManager.GetActiveWeights()
	return activeWeights.MovementGate
}

// GetRegimeTransitions returns regime transition analysis
func (api *API) GetRegimeTransitions() map[string]interface{} {
	api.mu.RLock()
	defer api.mu.RUnlock()

	return api.weightManager.GetRegimeTransitionMatrix()
}

// scheduleNextUpdate sets up the next 4-hour update cycle
func (api *API) scheduleNextUpdate(ctx context.Context) {
	if api.updateTimer != nil {
		api.updateTimer.Stop()
	}

	updateInterval := 4 * time.Hour
	api.updateTimer = time.AfterFunc(updateInterval, func() {
		api.performScheduledUpdate(ctx)
	})
}

// performScheduledUpdate executes the scheduled regime detection
func (api *API) performScheduledUpdate(ctx context.Context) {
	api.mu.Lock()
	defer api.mu.Unlock()

	if !api.isRunning {
		return // API was stopped
	}

	result, err := api.detector.DetectRegime(ctx)
	if err != nil {
		// Log error but continue running
		// In production, this would use proper logging
		return
	}

	// Update weights if regime changed
	previousRegime := api.weightManager.GetCurrentRegime()
	if previousRegime != result.Regime {
		api.weightManager.UpdateCurrentRegime(result)
	}

	api.lastCheck = time.Now()

	// Schedule next update
	api.scheduleNextUpdate(ctx)
}

// IsRunning returns whether the regime detection cycle is active
func (api *API) IsRunning() bool {
	api.mu.RLock()
	defer api.mu.RUnlock()

	return api.isRunning
}

// GetTimeSinceLastUpdate returns duration since last regime check
func (api *API) GetTimeSinceLastUpdate() time.Duration {
	api.mu.RLock()
	defer api.mu.RUnlock()

	if api.lastCheck.IsZero() {
		return 0
	}

	return time.Since(api.lastCheck)
}
