package calibration

import (
	"fmt"
	"math"
	"sort"
	"time"
)

// IsotonicCalibrator implements monotone score-to-probability mapping using isotonic regression
type IsotonicCalibrator struct {
	// Calibration curve data
	scores       []float64 // Monotone increasing scores
	probabilities []float64 // Corresponding probabilities
	
	// Metadata
	fittedAt     time.Time // When this calibration was fitted
	sampleCount  int       // Number of training samples used
	regime       string    // Market regime during fitting ("bull", "bear", "choppy")
	
	// Performance metrics
	reliability  float64   // Hosmer-Lemeshow goodness-of-fit
	resolution   float64   // Discrimination ability
	sharpness    float64   // Average probability spread
	
	// Configuration
	minSamples   int       // Minimum samples required for fitting
	smoothing    float64   // Smoothing parameter for noisy data
}

// CalibrationConfig holds configuration for isotonic calibration
type CalibrationConfig struct {
	MinSamples        int           `yaml:"min_samples"`        // Minimum samples for fitting (default: 100)
	RefreshInterval   time.Duration `yaml:"refresh_interval"`   // How often to refit (default: 30 days)
	SmoothingFactor   float64       `yaml:"smoothing_factor"`   // Smoothing for noisy data (default: 0.01)
	MaxAge            time.Duration `yaml:"max_age"`            // Max age before forced refresh (default: 90 days)
	RegimeAware       bool          `yaml:"regime_aware"`       // Separate calibrations per regime (default: true)
	ValidationSplit   float64       `yaml:"validation_split"`   // Fraction for holdout validation (default: 0.2)
}

// DefaultCalibrationConfig returns sensible defaults for calibration
func DefaultCalibrationConfig() CalibrationConfig {
	return CalibrationConfig{
		MinSamples:      100,
		RefreshInterval: 30 * 24 * time.Hour, // 30 days
		SmoothingFactor: 0.01,
		MaxAge:          90 * 24 * time.Hour, // 90 days
		RegimeAware:     true,
		ValidationSplit: 0.2,
	}
}

// CalibrationSample represents a training sample for calibration
type CalibrationSample struct {
	Score     float64   `json:"score"`     // Composite score from unified scorer
	Outcome   bool      `json:"outcome"`   // True if target event occurred
	Timestamp time.Time `json:"timestamp"` // When this sample was observed
	Symbol    string    `json:"symbol"`    // Asset symbol for this sample
	Regime    string    `json:"regime"`    // Market regime during observation
	
	// Additional metadata for validation
	HoldingPeriod time.Duration `json:"holding_period"` // How long position was held
	MaxMove       float64       `json:"max_move"`       // Maximum price movement observed
	FinalMove     float64       `json:"final_move"`     // Final price movement
}

// NewIsotonicCalibrator creates a new isotonic calibrator
func NewIsotonicCalibrator(config CalibrationConfig) *IsotonicCalibrator {
	return &IsotonicCalibrator{
		scores:        make([]float64, 0),
		probabilities: make([]float64, 0),
		minSamples:    config.MinSamples,
		smoothing:     config.SmoothingFactor,
		sampleCount:   0,
		reliability:   0.0,
		resolution:    0.0,
		sharpness:     0.0,
	}
}

// Fit performs isotonic regression on calibration samples
func (ic *IsotonicCalibrator) Fit(samples []CalibrationSample) error {
	if len(samples) < ic.minSamples {
		return fmt.Errorf("insufficient samples for calibration: need %d, got %d", ic.minSamples, len(samples))
	}

	// Sort samples by score
	sort.Slice(samples, func(i, j int) bool {
		return samples[i].Score < samples[j].Score
	})

	// Group samples into bins for probability estimation
	bins := ic.createBins(samples)
	
	// Perform isotonic regression to ensure monotonicity
	scores, probs := ic.isotonicRegression(bins)
	
	// Store calibration curve
	ic.scores = scores
	ic.probabilities = probs
	ic.fittedAt = time.Now()
	ic.sampleCount = len(samples)
	
	// Calculate performance metrics
	ic.calculateMetrics(samples)
	
	return nil
}

// Predict returns calibrated probability for a given score
func (ic *IsotonicCalibrator) Predict(score float64) float64 {
	if len(ic.scores) == 0 {
		// No calibration fitted - return uncalibrated score
		return math.Max(0, math.Min(1, score/100.0))
	}

	// Handle edge cases
	if score <= ic.scores[0] {
		return ic.probabilities[0]
	}
	if score >= ic.scores[len(ic.scores)-1] {
		return ic.probabilities[len(ic.probabilities)-1]
	}

	// Linear interpolation between calibration points
	for i := 1; i < len(ic.scores); i++ {
		if score <= ic.scores[i] {
			// Interpolate between points i-1 and i
			x0, x1 := ic.scores[i-1], ic.scores[i]
			y0, y1 := ic.probabilities[i-1], ic.probabilities[i]
			
			// Linear interpolation
			weight := (score - x0) / (x1 - x0)
			return y0 + weight*(y1-y0)
		}
	}

	// Fallback (should not reach here)
	return ic.probabilities[len(ic.probabilities)-1]
}

// createBins groups samples into bins for probability estimation
func (ic *IsotonicCalibrator) createBins(samples []CalibrationSample) []CalibrationBin {
	// Adaptive binning based on sample count
	numBins := ic.calculateOptimalBins(len(samples))
	binSize := len(samples) / numBins
	
	bins := make([]CalibrationBin, 0, numBins)
	
	for i := 0; i < len(samples); i += binSize {
		end := i + binSize
		if end > len(samples) {
			end = len(samples)
		}
		if end <= i {
			break
		}
		
		binSamples := samples[i:end]
		bin := ic.createBin(binSamples)
		bins = append(bins, bin)
	}
	
	return bins
}

// CalibrationBin represents a bin of samples for probability estimation
type CalibrationBin struct {
	MeanScore   float64 // Average score in this bin
	Probability float64 // Observed probability of positive outcome
	Count       int     // Number of samples in bin
	Confidence  float64 // Confidence interval width (±)
}

// createBin creates a calibration bin from samples
func (ic *IsotonicCalibrator) createBin(samples []CalibrationSample) CalibrationBin {
	if len(samples) == 0 {
		return CalibrationBin{}
	}

	// Calculate mean score
	scoreSum := 0.0
	positiveCount := 0
	
	for _, sample := range samples {
		scoreSum += sample.Score
		if sample.Outcome {
			positiveCount++
		}
	}
	
	meanScore := scoreSum / float64(len(samples))
	probability := float64(positiveCount) / float64(len(samples))
	
	// Calculate confidence interval (Wilson score interval)
	confidence := ic.calculateConfidenceInterval(positiveCount, len(samples))
	
	return CalibrationBin{
		MeanScore:   meanScore,
		Probability: probability,
		Count:       len(samples),
		Confidence:  confidence,
	}
}

// isotonicRegression performs pool-adjacent-violators algorithm
func (ic *IsotonicCalibrator) isotonicRegression(bins []CalibrationBin) ([]float64, []float64) {
	if len(bins) == 0 {
		return []float64{}, []float64{}
	}
	
	scores := make([]float64, len(bins))
	probs := make([]float64, len(bins))
	weights := make([]float64, len(bins))
	
	// Initialize with bin data
	for i, bin := range bins {
		scores[i] = bin.MeanScore
		probs[i] = bin.Probability
		weights[i] = float64(bin.Count)
	}
	
	// Pool-Adjacent-Violators Algorithm
	// Ensures monotone increasing probabilities
	for i := 1; i < len(probs); i++ {
		if probs[i] < probs[i-1] {
			// Pool adjacent violators
			ic.poolViolators(probs, weights, scores, i)
			
			// Start over from beginning to check for new violations
			i = 0
		}
	}
	
	return scores, probs
}

// poolViolators merges violating adjacent points
func (ic *IsotonicCalibrator) poolViolators(probs, weights, scores []float64, violatorIndex int) {
	// Find the range of points to pool
	start := violatorIndex - 1
	end := violatorIndex
	
	// Extend backwards while violations exist
	for start > 0 && probs[start] > probs[start-1] {
		start--
	}
	
	// Extend forwards while violations exist  
	for end < len(probs)-1 && probs[end] > probs[end+1] {
		end++
	}
	
	// Calculate pooled values
	totalWeight := 0.0
	weightedProbSum := 0.0
	weightedScoreSum := 0.0
	
	for i := start; i <= end; i++ {
		totalWeight += weights[i]
		weightedProbSum += weights[i] * probs[i]
		weightedScoreSum += weights[i] * scores[i]
	}
	
	pooledProb := weightedProbSum / totalWeight
	pooledScore := weightedScoreSum / totalWeight
	
	// Replace all points in range with pooled values
	for i := start; i <= end; i++ {
		probs[i] = pooledProb
		scores[i] = pooledScore
	}
}

// calculateOptimalBins determines optimal number of bins based on sample size
func (ic *IsotonicCalibrator) calculateOptimalBins(sampleCount int) int {
	// Sturges' rule with modifications for calibration
	baseRule := int(math.Ceil(math.Log2(float64(sampleCount)))) + 1
	
	// Constraints based on sample size
	minBins := 5   // At least 5 bins for meaningful calibration
	maxBins := 50  // At most 50 bins to avoid overfitting
	
	// Ensure at least 10 samples per bin on average
	maxBinsBySample := sampleCount / 10
	
	optimalBins := baseRule
	if optimalBins < minBins {
		optimalBins = minBins
	}
	if optimalBins > maxBins {
		optimalBins = maxBins
	}
	if optimalBins > maxBinsBySample && maxBinsBySample >= minBins {
		optimalBins = maxBinsBySample
	}
	
	return optimalBins
}

// calculateConfidenceInterval computes Wilson score confidence interval
func (ic *IsotonicCalibrator) calculateConfidenceInterval(successes, trials int) float64 {
	if trials == 0 {
		return 0.0
	}
	
	p := float64(successes) / float64(trials)
	n := float64(trials)
	z := 1.96 // 95% confidence interval
	
	// Wilson score interval half-width
	term2 := z * math.Sqrt((p*(1-p) + z*z/(4*n)) / n)
	denominator := 1 + z*z/n
	
	halfWidth := term2 / denominator
	return halfWidth
}

// calculateMetrics computes calibration performance metrics
func (ic *IsotonicCalibrator) calculateMetrics(samples []CalibrationSample) {
	if len(samples) == 0 || len(ic.scores) == 0 {
		return
	}
	
	// Calculate reliability (calibration error)
	ic.reliability = ic.calculateReliability(samples)
	
	// Calculate resolution (discrimination ability)
	ic.resolution = ic.calculateResolution(samples)
	
	// Calculate sharpness (average probability spread)
	ic.sharpness = ic.calculateSharpness()
}

// calculateReliability computes Hosmer-Lemeshow-style reliability
func (ic *IsotonicCalibrator) calculateReliability(samples []CalibrationSample) float64 {
	// Create bins and calculate observed vs predicted frequencies
	bins := ic.createBins(samples)
	
	chiSquare := 0.0
	for _, bin := range bins {
		if bin.Count == 0 {
			continue
		}
		
		observed := bin.Probability * float64(bin.Count)
		expected := ic.Predict(bin.MeanScore) * float64(bin.Count)
		
		if expected > 0 {
			chiSquare += math.Pow(observed-expected, 2) / expected
		}
	}
	
	// Return normalized reliability metric (lower is better)
	return chiSquare / float64(len(bins))
}

// calculateResolution measures discrimination ability
func (ic *IsotonicCalibrator) calculateResolution(samples []CalibrationSample) float64 {
	if len(samples) == 0 {
		return 0.0
	}
	
	// Calculate base rate
	positiveCount := 0
	for _, sample := range samples {
		if sample.Outcome {
			positiveCount++
		}
	}
	baseRate := float64(positiveCount) / float64(len(samples))
	
	// Calculate weighted variance of predicted probabilities
	meanPrediction := 0.0
	for _, sample := range samples {
		prob := ic.Predict(sample.Score)
		meanPrediction += prob
	}
	meanPrediction /= float64(len(samples))
	
	variance := 0.0
	for _, sample := range samples {
		prob := ic.Predict(sample.Score)
		variance += math.Pow(prob-meanPrediction, 2)
	}
	variance /= float64(len(samples))
	
	// Resolution relative to maximum possible
	maxResolution := baseRate * (1 - baseRate)
	if maxResolution > 0 {
		return variance / maxResolution
	}
	
	return 0.0
}

// calculateSharpness measures average probability spread
func (ic *IsotonicCalibrator) calculateSharpness() float64 {
	if len(ic.probabilities) < 2 {
		return 0.0
	}
	
	minProb := ic.probabilities[0]
	maxProb := ic.probabilities[len(ic.probabilities)-1]
	
	return maxProb - minProb
}

// GetCalibrationInfo returns metadata about the current calibration
type CalibrationInfo struct {
	FittedAt     time.Time `json:"fitted_at"`
	SampleCount  int       `json:"sample_count"`
	Regime       string    `json:"regime"`
	Reliability  float64   `json:"reliability"`  // Lower is better (calibration error)
	Resolution   float64   `json:"resolution"`   // Higher is better (discrimination)
	Sharpness    float64   `json:"sharpness"`    // Probability range spread
	PointCount   int       `json:"point_count"`  // Number of calibration points
	ScoreRange   [2]float64 `json:"score_range"` // Min/max scores in calibration
}

// GetInfo returns information about the current calibration
func (ic *IsotonicCalibrator) GetInfo() CalibrationInfo {
	info := CalibrationInfo{
		FittedAt:    ic.fittedAt,
		SampleCount: ic.sampleCount,
		Regime:      ic.regime,
		Reliability: ic.reliability,
		Resolution:  ic.resolution,
		Sharpness:   ic.sharpness,
		PointCount:  len(ic.scores),
	}
	
	if len(ic.scores) > 0 {
		info.ScoreRange = [2]float64{ic.scores[0], ic.scores[len(ic.scores)-1]}
	}
	
	return info
}

// IsValid checks if the calibration is ready for use
func (ic *IsotonicCalibrator) IsValid() bool {
	return len(ic.scores) > 0 && 
		   len(ic.probabilities) > 0 && 
		   len(ic.scores) == len(ic.probabilities) &&
		   ic.sampleCount >= ic.minSamples
}

// GetAge returns how old the current calibration is
func (ic *IsotonicCalibrator) GetAge() time.Duration {
	if ic.fittedAt.IsZero() {
		return time.Duration(0)
	}
	return time.Since(ic.fittedAt)
}

// NeedsRefresh checks if calibration needs to be refreshed
func (ic *IsotonicCalibrator) NeedsRefresh(config CalibrationConfig) bool {
	if !ic.IsValid() {
		return true
	}
	
	age := ic.GetAge()
	return age > config.RefreshInterval
}