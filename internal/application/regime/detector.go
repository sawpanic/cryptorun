package regime

import (
	"time"
)

// DetectorInputs holds market data for regime detection
type DetectorInputs struct {
	RealizedVol7d float64 // 7-day realized volatility
	PctAbove20MA  float64 // Percentage of assets above 20-day MA
	BreadthThrust float64 // Breadth thrust indicator
	Timestamp     time.Time
}

// DetectionResult holds regime detection output
type DetectionResult struct {
	Regime     string             `json:"regime"`
	Confidence float64            `json:"confidence"`
	Timestamp  time.Time          `json:"timestamp"`
	Inputs     DetectorInputs     `json:"inputs"`
	Thresholds map[string]float64 `json:"thresholds"`
}

// RegimeDetector detects current market regime
type RegimeDetector struct {
	// Thresholds for regime classification
	TrendingThresholds struct {
		VolMax     float64 // Maximum volatility for trending
		AboveMAMin float64 // Minimum % above 20MA
		ThrustMin  float64 // Minimum breadth thrust
	}

	ChoppyThresholds struct {
		VolMin     float64 // Minimum volatility for choppy
		VolMax     float64 // Maximum volatility for choppy
		AboveMAMin float64 // Minimum % above 20MA
		AboveMAMax float64 // Maximum % above 20MA
		ThrustMin  float64 // Minimum breadth thrust
		ThrustMax  float64 // Maximum breadth thrust
	}

	HighVolThresholds struct {
		VolMin float64 // Minimum volatility for high-vol
	}

	// Cache for regime decisions
	lastDetection *DetectionResult
	cacheExpiry   time.Duration
}

// NewRegimeDetector creates a new regime detector with default thresholds
func NewRegimeDetector() *RegimeDetector {
	rd := &RegimeDetector{
		cacheExpiry: 15 * time.Minute, // Cache regime for 15 minutes
	}

	// Set default thresholds from config
	rd.TrendingThresholds.VolMax = 0.3
	rd.TrendingThresholds.AboveMAMin = 0.6
	rd.TrendingThresholds.ThrustMin = 0.4

	rd.ChoppyThresholds.VolMin = 0.3
	rd.ChoppyThresholds.VolMax = 0.5
	rd.ChoppyThresholds.AboveMAMin = 0.4
	rd.ChoppyThresholds.AboveMAMax = 0.6
	rd.ChoppyThresholds.ThrustMin = 0.2
	rd.ChoppyThresholds.ThrustMax = 0.4

	rd.HighVolThresholds.VolMin = 0.5

	return rd
}

// DetectRegime determines current market regime
func (rd *RegimeDetector) DetectRegime(inputs DetectorInputs) DetectionResult {
	// Check cache first
	if rd.lastDetection != nil &&
		time.Since(rd.lastDetection.Timestamp) < rd.cacheExpiry {
		return *rd.lastDetection
	}

	// Classify regime based on thresholds
	regime, confidence := rd.classifyRegime(inputs)

	// Create result
	result := DetectionResult{
		Regime:     regime,
		Confidence: confidence,
		Timestamp:  inputs.Timestamp,
		Inputs:     inputs,
		Thresholds: rd.getThresholdsMap(),
	}

	// Cache result
	rd.lastDetection = &result

	return result
}

// classifyRegime applies threshold logic to determine regime
func (rd *RegimeDetector) classifyRegime(inputs DetectorInputs) (string, float64) {
	// Check for trending regime
	if inputs.RealizedVol7d <= rd.TrendingThresholds.VolMax &&
		inputs.PctAbove20MA >= rd.TrendingThresholds.AboveMAMin &&
		inputs.BreadthThrust >= rd.TrendingThresholds.ThrustMin {

		// Calculate confidence based on how strongly conditions are met
		volConfidence := 1.0 - (inputs.RealizedVol7d / rd.TrendingThresholds.VolMax)
		maConfidence := inputs.PctAbove20MA / rd.TrendingThresholds.AboveMAMin
		thrustConfidence := inputs.BreadthThrust / rd.TrendingThresholds.ThrustMin

		confidence := (volConfidence + maConfidence + thrustConfidence) / 3.0
		if confidence > 1.0 {
			confidence = 1.0
		}

		return "TRENDING", confidence
	}

	// Check for high volatility regime
	if inputs.RealizedVol7d >= rd.HighVolThresholds.VolMin {
		confidence := inputs.RealizedVol7d / rd.HighVolThresholds.VolMin
		if confidence > 1.0 {
			confidence = 1.0
		}
		return "HIGH_VOL", confidence
	}

	// Check for choppy regime
	if inputs.RealizedVol7d >= rd.ChoppyThresholds.VolMin &&
		inputs.RealizedVol7d <= rd.ChoppyThresholds.VolMax &&
		inputs.PctAbove20MA >= rd.ChoppyThresholds.AboveMAMin &&
		inputs.PctAbove20MA <= rd.ChoppyThresholds.AboveMAMax &&
		inputs.BreadthThrust >= rd.ChoppyThresholds.ThrustMin &&
		inputs.BreadthThrust <= rd.ChoppyThresholds.ThrustMax {

		// Choppy regime confidence based on how well conditions are met
		confidence := 0.7 // Base confidence for choppy (harder to detect)
		return "CHOP", confidence
	}

	// Default to unknown regime with low confidence
	return "UNKNOWN", 0.3
}

// getThresholdsMap returns all thresholds for debugging/logging
func (rd *RegimeDetector) getThresholdsMap() map[string]float64 {
	return map[string]float64{
		"trending_vol_max":      rd.TrendingThresholds.VolMax,
		"trending_above_ma_min": rd.TrendingThresholds.AboveMAMin,
		"trending_thrust_min":   rd.TrendingThresholds.ThrustMin,
		"choppy_vol_min":        rd.ChoppyThresholds.VolMin,
		"choppy_vol_max":        rd.ChoppyThresholds.VolMax,
		"choppy_above_ma_min":   rd.ChoppyThresholds.AboveMAMin,
		"choppy_above_ma_max":   rd.ChoppyThresholds.AboveMAMax,
		"choppy_thrust_min":     rd.ChoppyThresholds.ThrustMin,
		"choppy_thrust_max":     rd.ChoppyThresholds.ThrustMax,
		"high_vol_min":          rd.HighVolThresholds.VolMin,
	}
}

// GetLastDetection returns the last cached regime detection
func (rd *RegimeDetector) GetLastDetection() *DetectionResult {
	return rd.lastDetection
}

// ClearCache clears the regime detection cache
func (rd *RegimeDetector) ClearCache() {
	rd.lastDetection = nil
}
