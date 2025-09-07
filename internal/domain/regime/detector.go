package regime

import (
	"fmt"
	"math"
	"time"

	regimeconfig "github.com/sawpanic/cryptorun/internal/config/regime"
)

// Use types from config package to avoid duplication and import cycles
type RegimeType = regimeconfig.RegimeType
type MarketData = regimeconfig.MarketData
type RegimeDetection = regimeconfig.RegimeDetection
type RegimeIndicator = regimeconfig.RegimeIndicator

// Constants for regime types
const (
	RegimeCalm     = regimeconfig.RegimeCalm
	RegimeNormal   = regimeconfig.RegimeNormal
	RegimeVolatile = regimeconfig.RegimeVolatile
)

// RegimeDetector implements the 4-hour regime detection system
type RegimeDetector struct {
	config          regimeconfig.WeightsConfig
	detectionWindow time.Duration
	lastDetection   *RegimeDetection
	
	// Thresholds for regime classification
	volatilityThresholds struct {
		calmHigh     float64  // Below this = calm
		normalHigh   float64  // Between calm and this = normal
		// Above normalHigh = volatile
	}
	
	breadthThresholds struct {
		strongThrust  float64  // Strong breadth thrust
		weakThrust    float64  // Weak breadth thrust
	}
}

// NewRegimeDetector creates a new regime detector
func NewRegimeDetector(config regimeconfig.WeightsConfig) *RegimeDetector {
	return &RegimeDetector{
		config:          config,
		detectionWindow: 4 * time.Hour, // 4h refresh cycle
		volatilityThresholds: struct {
			calmHigh   float64
			normalHigh float64
		}{
			calmHigh:   0.15,  // 15% annualized vol
			normalHigh: 0.35,  // 35% annualized vol
		},
		breadthThresholds: struct {
			strongThrust float64
			weakThrust   float64
		}{
			strongThrust: 0.7,   // 70% thrust
			weakThrust:   0.3,   // 30% thrust
		},
	}
}

// DetectRegime performs regime detection using multiple indicators
func (rd *RegimeDetector) DetectRegime(data MarketData) (*RegimeDetection, error) {
	now := time.Now()
	
	// Check if we need a fresh detection (4h cycle)
	if rd.lastDetection != nil && now.Before(rd.lastDetection.ValidUntil) {
		// Return cached result if still valid
		return rd.lastDetection, nil
	}

	// Collect regime indicators
	indicators := []RegimeIndicator{}
	
	// Indicator 1: Realized Volatility (7-day)
	volIndicator, err := rd.analyzeVolatility(data)
	if err != nil {
		return nil, fmt.Errorf("volatility analysis failed: %w", err)
	}
	indicators = append(indicators, volIndicator)
	
	// Indicator 2: Moving Average Position
	maIndicator, err := rd.analyzeMovingAveragePosition(data)
	if err != nil {
		return nil, fmt.Errorf("moving average analysis failed: %w", err)
	}
	indicators = append(indicators, maIndicator)
	
	// Indicator 3: Breadth Thrust
	breadthIndicator, err := rd.analyzeBreadthThrust(data)
	if err != nil {
		return nil, fmt.Errorf("breadth analysis failed: %w", err)
	}
	indicators = append(indicators, breadthIndicator)
	
	// Apply majority vote with weights
	regime, confidence := rd.calculateMajorityVote(indicators)
	
	// Track regime changes
	var regimeChangedAt *time.Time
	previousRegime := RegimeCalm // Default
	if rd.lastDetection != nil {
		previousRegime = rd.lastDetection.CurrentRegime
		if previousRegime != regime {
			regimeChangedAt = &now
		}
	}
	
	detection := &RegimeDetection{
		CurrentRegime:   regime,
		Confidence:      confidence,
		Indicators:      indicators,
		DetectionTime:   now,
		ValidUntil:      now.Add(rd.detectionWindow),
		PreviousRegime:  previousRegime,
		RegimeChangedAt: regimeChangedAt,
	}
	
	rd.lastDetection = detection
	return detection, nil
}

// analyzeVolatility determines regime based on 7-day realized volatility
func (rd *RegimeDetector) analyzeVolatility(data MarketData) (RegimeIndicator, error) {
	realizedVol := data.RealizedVol7d
	
	var vote RegimeType
	if realizedVol < rd.volatilityThresholds.calmHigh {
		vote = RegimeCalm
	} else if realizedVol < rd.volatilityThresholds.normalHigh {
		vote = RegimeNormal
	} else {
		vote = RegimeVolatile
	}
	
	return RegimeIndicator{
		Name:      "RealizedVol7d",
		Value:     realizedVol,
		Threshold: rd.volatilityThresholds.normalHigh,
		Vote:      vote,
		Weight:    0.4, // 40% weight for volatility
	}, nil
}

// analyzeMovingAveragePosition determines regime based on price vs 20MA
func (rd *RegimeDetector) analyzeMovingAveragePosition(data MarketData) (RegimeIndicator, error) {
	if data.MA20 <= 0 {
		return RegimeIndicator{}, fmt.Errorf("invalid moving average: %f", data.MA20)
	}
	
	// Calculate percentage above/below 20MA
	pctAboveMA := (data.CurrentPrice - data.MA20) / data.MA20 * 100
	
	var vote RegimeType
	if math.Abs(pctAboveMA) < 2.0 {
		// Within 2% of MA = choppy/volatile
		vote = RegimeVolatile
	} else if pctAboveMA > 5.0 || pctAboveMA < -5.0 {
		// Strong trend = calm
		vote = RegimeCalm  
	} else {
		// Moderate trend = normal
		vote = RegimeNormal
	}
	
	return RegimeIndicator{
		Name:      "MA20Position",
		Value:     pctAboveMA,
		Threshold: 5.0, // 5% threshold for strong trend
		Vote:      vote,
		Weight:    0.3, // 30% weight for trend
	}, nil
}

// analyzeBreadthThrust determines regime based on market breadth
func (rd *RegimeDetector) analyzeBreadthThrust(data MarketData) (RegimeIndicator, error) {
	// Composite breadth score (0-1 scale)
	breadthScore := (data.BreadthData.AdvanceDeclineRatio + 
					 data.BreadthData.VolumeRatio + 
					 data.BreadthData.NewHighsNewLows) / 3.0
	
	// Clamp to [0, 1] range
	if breadthScore > 1.0 {
		breadthScore = 1.0
	} else if breadthScore < 0.0 {
		breadthScore = 0.0
	}
	
	var vote RegimeType
	if breadthScore > rd.breadthThresholds.strongThrust {
		// Strong breadth = trending/calm
		vote = RegimeCalm
	} else if breadthScore > rd.breadthThresholds.weakThrust {
		// Moderate breadth = normal
		vote = RegimeNormal
	} else {
		// Weak breadth = volatile/choppy
		vote = RegimeVolatile
	}
	
	return RegimeIndicator{
		Name:      "BreadthThrust",
		Value:     breadthScore,
		Threshold: rd.breadthThresholds.strongThrust,
		Vote:      vote,
		Weight:    0.3, // 30% weight for breadth
	}, nil
}

// calculateMajorityVote applies weighted majority voting to determine regime
func (rd *RegimeDetector) calculateMajorityVote(indicators []RegimeIndicator) (RegimeType, float64) {
	votes := map[RegimeType]float64{
		RegimeCalm:     0.0,
		RegimeNormal:   0.0,
		RegimeVolatile: 0.0,
	}
	
	totalWeight := 0.0
	for _, indicator := range indicators {
		votes[indicator.Vote] += indicator.Weight
		totalWeight += indicator.Weight
	}
	
	// Normalize weights
	if totalWeight > 0 {
		for regime := range votes {
			votes[regime] /= totalWeight
		}
	}
	
	// Find winner
	winningRegime := RegimeNormal // Default fallback
	maxVote := 0.0
	
	for regime, vote := range votes {
		if vote > maxVote {
			maxVote = vote
			winningRegime = regime
		}
	}
	
	confidence := maxVote * 100.0 // Convert to percentage
	return winningRegime, confidence
}

// GetCurrentRegime returns the current regime (may trigger detection)
func (rd *RegimeDetector) GetCurrentRegime(data MarketData) (RegimeType, error) {
	detection, err := rd.DetectRegime(data)
	if err != nil {
		return RegimeNormal, err
	}
	return detection.CurrentRegime, nil
}

// GetWeightsForRegime returns the weight configuration for a given regime
func (rd *RegimeDetector) GetWeightsForRegime(regime RegimeType) (regimeconfig.DomainRegimeWeights, error) {
	regimeStr := string(regime)
	weights, exists := rd.config.Regimes[regimeStr]
	if !exists {
		// Fall back to default regime 
		defaultRegime := "normal" // Default regime
		weights, exists = rd.config.Regimes[defaultRegime]
		if !exists {
			return regimeconfig.DomainRegimeWeights{}, fmt.Errorf("no weights found for regime %s or default", regimeStr)
		}
	}
	// Convert from config RegimeWeights to DomainRegimeWeights
	domainWeights := regimeconfig.DomainRegimeWeights{
		MomentumCore: weights.MomentumCore * 100, // Convert from 0-1 to 0-100
		Technical:    weights.TechnicalResid * 100,
		Volume:       weights.SupplyDemandBlock * 0.55 * 100, // 55% of supply/demand goes to volume
		Quality:      weights.SupplyDemandBlock * 0.45 * 100, // 45% of supply/demand goes to quality
		Social:       0, // Social is always applied separately outside the 100% allocation
	}
	
	return domainWeights, nil
}

// ValidateDomainRegimeWeights ensures weight configuration is valid (renamed to avoid conflict)
func ValidateDomainRegimeWeights(weights regimeconfig.DomainRegimeWeights, config regimeconfig.WeightsConfig) error {
	// Calculate total weight (excluding social which is capped separately)
	total := weights.MomentumCore + weights.Technical + weights.Volume + weights.Quality
	
	// Check weight sum tolerance
	tolerance := config.Validation.WeightSumTolerance
	if math.Abs(total-1.0) > tolerance {
		return fmt.Errorf("weight sum %.3f outside tolerance %.3f of 1.0", total, tolerance)
	}
	
	// Check minimum momentum weight (hardcoded minimum)
	if weights.MomentumCore < 0.20 {
		return fmt.Errorf("momentum weight %.3f below minimum 0.20", weights.MomentumCore)
	}
	
	// Check maximum social weight (use SocialHardCap from validation)
	if weights.Social > config.Validation.SocialHardCap {
		return fmt.Errorf("social weight %.3f above maximum %.3f", weights.Social, config.Validation.SocialHardCap)
	}
	
	// Ensure all weights are non-negative
	allWeights := []struct {
		name   string
		weight float64
	}{
		{"momentum_core", weights.MomentumCore},
		{"technical", weights.Technical},
		{"volume", weights.Volume},
		{"quality", weights.Quality},
		{"social", weights.Social},
	}
	
	for _, w := range allWeights {
		if w.weight < 0 {
			return fmt.Errorf("%s weight cannot be negative: %.3f", w.name, w.weight)
		}
	}
	
	return nil
}

// FormatRegimeReport creates a human-readable regime report
func FormatRegimeReport(detection *RegimeDetection) string {
	if detection == nil {
		return "No regime detection available"
	}
	
	report := fmt.Sprintf("Regime: %s (%.1f%% confidence)\n", 
		detection.CurrentRegime, detection.Confidence)
	
	report += fmt.Sprintf("Detected: %s (valid until %s)\n",
		detection.DetectionTime.Format("15:04:05"),
		detection.ValidUntil.Format("15:04:05"))
	
	if detection.RegimeChangedAt != nil {
		report += fmt.Sprintf("Changed from %s at %s\n",
			detection.PreviousRegime,
			detection.RegimeChangedAt.Format("15:04:05"))
	}
	
	report += "\nIndicator Breakdown:\n"
	for _, indicator := range detection.Indicators {
		report += fmt.Sprintf("  %s: %.3f → %s (weight: %.1f%%)\n",
			indicator.Name, indicator.Value, indicator.Vote, indicator.Weight*100)
	}
	
	return report
}

// GetLastUpdate returns when the detector last ran
func (rd *RegimeDetector) GetLastUpdate() time.Time {
	if rd.lastDetection == nil {
		return time.Time{}
	}
	return rd.lastDetection.DetectionTime
}

// GetDetectorStatus returns the current status of the detector
func (rd *RegimeDetector) GetDetectorStatus() map[string]interface{} {
	if rd.lastDetection == nil {
		return map[string]interface{}{
			"status": "Not initialized",
			"last_detection": nil,
			"cache_valid": false,
		}
	}
	
	now := time.Now()
	cacheValid := now.Before(rd.lastDetection.ValidUntil)
	status := "Active (cached result)"
	if !cacheValid {
		status = "Stale (needs refresh)"
	}
	
	return map[string]interface{}{
		"status": status,
		"last_detection": rd.lastDetection.DetectionTime.Format(time.RFC3339),
		"current_regime": rd.lastDetection.CurrentRegime.String(),
		"confidence": rd.lastDetection.Confidence,
		"cache_valid": cacheValid,
		"valid_until": rd.lastDetection.ValidUntil.Format(time.RFC3339),
	}
}

// GetRegimeHistory returns the recent regime detection history
func (rd *RegimeDetector) GetRegimeHistory(limit int) []RegimeDetection {
	// For now, just return the current detection
	// In a full implementation, this would maintain a history buffer
	if rd.lastDetection == nil {
		return []RegimeDetection{}
	}
	return []RegimeDetection{*rd.lastDetection}
}

// ValidateInputs checks if the market data is valid for regime detection
func (rd *RegimeDetector) ValidateInputs(data MarketData) error {
	if len(data.Prices) == 0 {
		return fmt.Errorf("no price data provided")
	}
	if data.CurrentPrice <= 0 {
		return fmt.Errorf("invalid current price: %f", data.CurrentPrice)
	}
	if data.MA20 <= 0 {
		return fmt.Errorf("invalid MA20: %f", data.MA20)
	}
	if data.RealizedVol7d < 0 {
		return fmt.Errorf("invalid realized volatility: %f", data.RealizedVol7d)
	}
	return nil
}