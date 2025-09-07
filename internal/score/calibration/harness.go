package calibration

import (
	"context"
	"fmt"
	"math"
	"sort"
	"sync"
	"time"
)

// CalibrationHarness manages multiple isotonic calibrators for different regimes
type CalibrationHarness struct {
	// Regime-specific calibrators
	calibrators map[string]*IsotonicCalibrator
	
	// Data management
	sampleBuffer   []CalibrationSample
	maxBufferSize  int
	
	// Configuration
	config CalibrationConfig
	
	// Thread safety
	mutex sync.RWMutex
	
	// Status tracking
	lastRefresh   time.Time
	refreshCount  int
	totalSamples  int
}

// NewCalibrationHarness creates a new calibration harness
func NewCalibrationHarness(config CalibrationConfig) *CalibrationHarness {
	return &CalibrationHarness{
		calibrators:   make(map[string]*IsotonicCalibrator),
		sampleBuffer:  make([]CalibrationSample, 0),
		maxBufferSize: config.MinSamples * 10, // Buffer 10x minimum for efficient batching
		config:        config,
		lastRefresh:   time.Now(),
		refreshCount:  0,
		totalSamples:  0,
	}
}

// AddSample adds a new calibration sample to the buffer
func (ch *CalibrationHarness) AddSample(sample CalibrationSample) error {
	ch.mutex.Lock()
	defer ch.mutex.Unlock()
	
	// Validate sample
	if err := ch.validateSample(sample); err != nil {
		return fmt.Errorf("invalid calibration sample: %w", err)
	}
	
	// Add to buffer
	ch.sampleBuffer = append(ch.sampleBuffer, sample)
	ch.totalSamples++
	
	// Trim buffer if it exceeds maximum size
	if len(ch.sampleBuffer) > ch.maxBufferSize {
		// Keep most recent samples
		excess := len(ch.sampleBuffer) - ch.maxBufferSize
		ch.sampleBuffer = ch.sampleBuffer[excess:]
	}
	
	return nil
}

// PredictProbability returns calibrated probability for a score in a given regime
func (ch *CalibrationHarness) PredictProbability(score float64, regime string) (float64, error) {
	ch.mutex.RLock()
	defer ch.mutex.RUnlock()
	
	// Get regime-specific calibrator
	calibrator, exists := ch.calibrators[regime]
	if !exists || !calibrator.IsValid() {
		// Fall back to general calibrator if regime-specific not available
		if general, exists := ch.calibrators["general"]; exists && general.IsValid() {
			return general.Predict(score), nil
		}
		
		// No calibration available - return uncalibrated probability
		return ch.uncalibratedProbability(score), nil
	}
	
	return calibrator.Predict(score), nil
}

// RefreshCalibration refits all calibrators with recent data
func (ch *CalibrationHarness) RefreshCalibration(ctx context.Context) error {
	ch.mutex.Lock()
	defer ch.mutex.Unlock()
	
	if len(ch.sampleBuffer) < ch.config.MinSamples {
		return fmt.Errorf("insufficient samples for calibration refresh: need %d, have %d", 
			ch.config.MinSamples, len(ch.sampleBuffer))
	}
	
	// Group samples by regime if regime-aware mode is enabled
	var sampleGroups map[string][]CalibrationSample
	
	if ch.config.RegimeAware {
		sampleGroups = ch.groupSamplesByRegime()
	} else {
		// Single calibrator for all regimes
		sampleGroups = map[string][]CalibrationSample{
			"general": ch.sampleBuffer,
		}
	}
	
	// Fit calibrators for each regime
	newCalibrators := make(map[string]*IsotonicCalibrator)
	
	for regime, samples := range sampleGroups {
		if len(samples) < ch.config.MinSamples {
			// Skip regimes with insufficient data
			continue
		}
		
		// Create new calibrator
		calibrator := NewIsotonicCalibrator(ch.config)
		calibrator.regime = regime
		
		// Split into training and validation sets
		trainingSamples, validationSamples := ch.splitSamples(samples)
		
		// Fit calibrator
		if err := calibrator.Fit(trainingSamples); err != nil {
			return fmt.Errorf("failed to fit calibrator for regime %s: %w", regime, err)
		}
		
		// Validate calibrator performance
		if err := ch.validateCalibrator(calibrator, validationSamples); err != nil {
			return fmt.Errorf("calibrator validation failed for regime %s: %w", regime, err)
		}
		
		newCalibrators[regime] = calibrator
	}
	
	// Replace old calibrators with new ones
	ch.calibrators = newCalibrators
	ch.lastRefresh = time.Now()
	ch.refreshCount++
	
	return nil
}

// groupSamplesByRegime groups samples by market regime
func (ch *CalibrationHarness) groupSamplesByRegime() map[string][]CalibrationSample {
	groups := make(map[string][]CalibrationSample)
	
	for _, sample := range ch.sampleBuffer {
		regime := sample.Regime
		if regime == "" {
			regime = "general"
		}
		
		if groups[regime] == nil {
			groups[regime] = make([]CalibrationSample, 0)
		}
		groups[regime] = append(groups[regime], sample)
	}
	
	return groups
}

// splitSamples splits samples into training and validation sets
func (ch *CalibrationHarness) splitSamples(samples []CalibrationSample) ([]CalibrationSample, []CalibrationSample) {
	splitIndex := int(float64(len(samples)) * (1.0 - ch.config.ValidationSplit))
	
	// Ensure minimum training size
	if splitIndex < ch.config.MinSamples {
		splitIndex = len(samples) // Use all samples for training
	}
	
	training := samples[:splitIndex]
	validation := samples[splitIndex:]
	
	return training, validation
}

// validateSample checks if a calibration sample is valid
func (ch *CalibrationHarness) validateSample(sample CalibrationSample) error {
	if sample.Score < 0 || sample.Score > 110 {
		return fmt.Errorf("score %.2f outside valid range [0, 110]", sample.Score)
	}
	
	if sample.Timestamp.IsZero() {
		return fmt.Errorf("timestamp cannot be zero")
	}
	
	if sample.Symbol == "" {
		return fmt.Errorf("symbol cannot be empty")
	}
	
	if sample.HoldingPeriod <= 0 {
		return fmt.Errorf("holding period must be positive")
	}
	
	return nil
}

// validateCalibrator checks if a calibrator meets quality standards
func (ch *CalibrationHarness) validateCalibrator(calibrator *IsotonicCalibrator, validationSamples []CalibrationSample) error {
	if !calibrator.IsValid() {
		return fmt.Errorf("calibrator is not valid")
	}
	
	// Skip validation if no validation samples
	if len(validationSamples) == 0 {
		return nil
	}
	
	// Calculate calibration error on validation set
	calibrationError := ch.calculateCalibrationError(calibrator, validationSamples)
	
	// Check if calibration error is acceptable
	maxAllowedError := 0.1 // 10% maximum calibration error
	if calibrationError > maxAllowedError {
		return fmt.Errorf("calibration error %.3f exceeds maximum allowed %.3f", 
			calibrationError, maxAllowedError)
	}
	
	// Check if calibrator provides sufficient discrimination
	auc := ch.calculateAUC(calibrator, validationSamples)
	minAUC := 0.55 // Must be better than random (0.5) by at least 5%
	if auc < minAUC {
		return fmt.Errorf("AUC %.3f below minimum required %.3f", auc, minAUC)
	}
	
	return nil
}

// calculateCalibrationError computes mean absolute calibration error
func (ch *CalibrationHarness) calculateCalibrationError(calibrator *IsotonicCalibrator, samples []CalibrationSample) float64 {
	if len(samples) == 0 {
		return 0.0
	}
	
	totalError := 0.0
	
	for _, sample := range samples {
		predicted := calibrator.Predict(sample.Score)
		actual := 0.0
		if sample.Outcome {
			actual = 1.0
		}
		totalError += math.Abs(predicted - actual)
	}
	
	return totalError / float64(len(samples))
}

// calculateAUC computes Area Under the ROC Curve
func (ch *CalibrationHarness) calculateAUC(calibrator *IsotonicCalibrator, samples []CalibrationSample) float64 {
	if len(samples) < 2 {
		return 0.5 // Random performance
	}
	
	// Get predicted probabilities and actual outcomes
	type scoredSample struct {
		probability float64
		outcome     bool
	}
	
	scoredSamples := make([]scoredSample, len(samples))
	for i, sample := range samples {
		scoredSamples[i] = scoredSample{
			probability: calibrator.Predict(sample.Score),
			outcome:     sample.Outcome,
		}
	}
	
	// Sort by predicted probability (descending)
	sort.Slice(scoredSamples, func(i, j int) bool {
		return scoredSamples[i].probability > scoredSamples[j].probability
	})
	
	// Count positives and negatives
	positives := 0
	negatives := 0
	for _, sample := range scoredSamples {
		if sample.outcome {
			positives++
		} else {
			negatives++
		}
	}
	
	if positives == 0 || negatives == 0 {
		return 0.5 // Can't calculate AUC with only one class
	}
	
	// Calculate AUC using trapezoidal rule
	tpr := 0.0  // True Positive Rate
	fpr := 0.0  // False Positive Rate
	auc := 0.0
	
	currentPositives := 0
	currentNegatives := 0
	
	for _, sample := range scoredSamples {
		if sample.outcome {
			currentPositives++
		} else {
			currentNegatives++
		}
		
		newTPR := float64(currentPositives) / float64(positives)
		newFPR := float64(currentNegatives) / float64(negatives)
		
		// Add trapezoidal area
		auc += (newFPR - fpr) * (tpr + newTPR) / 2.0
		
		tpr = newTPR
		fpr = newFPR
	}
	
	return auc
}

// uncalibratedProbability returns a simple score-to-probability mapping
func (ch *CalibrationHarness) uncalibratedProbability(score float64) float64 {
	// Simple sigmoid-like mapping for scores 0-110
	// Maps score 75 to ~0.5 probability, 100 to ~0.8 probability
	normalized := math.Max(0, math.Min(110, score)) / 110.0
	
	// Sigmoid transformation centered around score 75
	centerPoint := 75.0 / 110.0
	steepness := 8.0
	
	return 1.0 / (1.0 + math.Exp(-steepness*(normalized-centerPoint)))
}

// GetStatus returns current calibration harness status
type CalibrationStatus struct {
	TotalSamples    int                          `json:"total_samples"`
	BufferSize      int                          `json:"buffer_size"`
	LastRefresh     time.Time                    `json:"last_refresh"`
	RefreshCount    int                          `json:"refresh_count"`
	Calibrators     map[string]CalibrationInfo   `json:"calibrators"`
	NextRefreshDue  time.Time                    `json:"next_refresh_due"`
	RefreshNeeded   bool                         `json:"refresh_needed"`
}

// GetStatus returns current status of the calibration harness
func (ch *CalibrationHarness) GetStatus() CalibrationStatus {
	ch.mutex.RLock()
	defer ch.mutex.RUnlock()
	
	status := CalibrationStatus{
		TotalSamples: ch.totalSamples,
		BufferSize:   len(ch.sampleBuffer),
		LastRefresh:  ch.lastRefresh,
		RefreshCount: ch.refreshCount,
		Calibrators:  make(map[string]CalibrationInfo),
		NextRefreshDue: ch.lastRefresh.Add(ch.config.RefreshInterval),
	}
	
	// Get info for each calibrator
	for regime, calibrator := range ch.calibrators {
		status.Calibrators[regime] = calibrator.GetInfo()
	}
	
	// Check if refresh is needed
	status.RefreshNeeded = ch.needsRefresh()
	
	return status
}

// needsRefresh checks if any calibrator needs refreshing
func (ch *CalibrationHarness) needsRefresh() bool {
	// Check if we have enough samples and time has passed
	if len(ch.sampleBuffer) < ch.config.MinSamples {
		return false
	}
	
	timeSinceRefresh := time.Since(ch.lastRefresh)
	if timeSinceRefresh > ch.config.RefreshInterval {
		return true
	}
	
	// Check if any calibrator is invalid or too old
	for _, calibrator := range ch.calibrators {
		if !calibrator.IsValid() || calibrator.NeedsRefresh(ch.config) {
			return true
		}
	}
	
	return false
}

// ScheduledRefresh performs calibration refresh if needed
func (ch *CalibrationHarness) ScheduledRefresh(ctx context.Context) error {
	if !ch.needsRefresh() {
		return nil // No refresh needed
	}
	
	return ch.RefreshCalibration(ctx)
}

// ClearOldSamples removes samples older than the specified duration
func (ch *CalibrationHarness) ClearOldSamples(maxAge time.Duration) int {
	ch.mutex.Lock()
	defer ch.mutex.Unlock()
	
	cutoff := time.Now().Add(-maxAge)
	originalCount := len(ch.sampleBuffer)
	
	// Filter out old samples
	filtered := make([]CalibrationSample, 0, len(ch.sampleBuffer))
	for _, sample := range ch.sampleBuffer {
		if sample.Timestamp.After(cutoff) {
			filtered = append(filtered, sample)
		}
	}
	
	ch.sampleBuffer = filtered
	removed := originalCount - len(filtered)
	
	return removed
}

// ExportCalibrationData exports calibration curves for analysis
type CalibrationExport struct {
	Regime       string                 `json:"regime"`
	Info         CalibrationInfo        `json:"info"`
	Scores       []float64              `json:"scores"`
	Probabilities []float64             `json:"probabilities"`
	ExportedAt   time.Time              `json:"exported_at"`
}

// ExportCalibrationData exports all calibration curves
func (ch *CalibrationHarness) ExportCalibrationData() []CalibrationExport {
	ch.mutex.RLock()
	defer ch.mutex.RUnlock()
	
	exports := make([]CalibrationExport, 0, len(ch.calibrators))
	
	for regime, calibrator := range ch.calibrators {
		if !calibrator.IsValid() {
			continue
		}
		
		export := CalibrationExport{
			Regime:        regime,
			Info:          calibrator.GetInfo(),
			Scores:        make([]float64, len(calibrator.scores)),
			Probabilities: make([]float64, len(calibrator.probabilities)),
			ExportedAt:    time.Now(),
		}
		
		copy(export.Scores, calibrator.scores)
		copy(export.Probabilities, calibrator.probabilities)
		
		exports = append(exports, export)
	}
	
	return exports
}