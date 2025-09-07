package regime

import (
	"fmt"
	"math"
	"time"
)

// RegimeType represents the current market regime
type RegimeType string

const (
	TrendingBull RegimeType = "TRENDING_BULL" // Trending bull market
	Choppy       RegimeType = "CHOPPY"        // Sideways/choppy market
	HighVol      RegimeType = "HIGH_VOL"      // High volatility market
)

// String returns the string representation of the regime
func (r RegimeType) String() string {
	return string(r)
}

// RegimeInputs holds the three indicators for regime detection
type RegimeInputs struct {
	RealizedVol7d float64   `json:"realized_vol_7d"` // 7-day realized volatility
	PctAbove20MA  float64   `json:"pct_above_20ma"`  // % of universe above 20MA
	BreadthThrust float64   `json:"breadth_thrust"`  // Breadth thrust indicator
	Timestamp     time.Time `json:"timestamp"`       // Time of measurement
}

// RegimeThresholds defines the threshold values for regime classification
type RegimeThresholds struct {
	// Realized volatility thresholds (annualized)
	VolLowThreshold  float64 `yaml:"vol_low_threshold"`  // Below this = low vol
	VolHighThreshold float64 `yaml:"vol_high_threshold"` // Above this = high vol

	// % above 20MA thresholds
	BullThreshold float64 `yaml:"bull_threshold"` // Above this = bullish
	BearThreshold float64 `yaml:"bear_threshold"` // Below this = bearish

	// Breadth thrust thresholds
	ThrustPositive float64 `yaml:"thrust_positive"` // Above this = positive thrust
	ThrustNegative float64 `yaml:"thrust_negative"` // Below this = negative thrust
}

// DefaultThresholds returns the default regime detection thresholds
func DefaultThresholds() RegimeThresholds {
	return RegimeThresholds{
		VolLowThreshold:  0.30,  // 30% annualized vol
		VolHighThreshold: 0.60,  // 60% annualized vol
		BullThreshold:    0.65,  // 65% above 20MA = bullish
		BearThreshold:    0.35,  // 35% above 20MA = bearish
		ThrustPositive:   0.15,  // 15% positive thrust
		ThrustNegative:   -0.15, // -15% negative thrust
	}
}

// RegimeDetector implements the 3-indicator regime detection logic
type RegimeDetector struct {
	thresholds    RegimeThresholds
	updateCadence time.Duration
	lastUpdate    time.Time
	currentRegime RegimeType
	history       []RegimeInputs // For majority voting
	maxHistory    int            // Maximum history to keep
}

// NewRegimeDetector creates a new regime detector with default configuration
func NewRegimeDetector(thresholds RegimeThresholds) *RegimeDetector {
	return &RegimeDetector{
		thresholds:    thresholds,
		updateCadence: 4 * time.Hour,              // 4h update cadence
		currentRegime: Choppy,                     // Safe default
		history:       make([]RegimeInputs, 0, 6), // Keep 24h of 4h updates
		maxHistory:    6,
	}
}

// DetectRegime performs regime detection using the 3-indicator system
func (rd *RegimeDetector) DetectRegime(inputs RegimeInputs) RegimeType {
	// Always add to history for majority voting
	rd.addToHistory(inputs)

	// Check if it's time to update (4h cadence)
	if !rd.shouldUpdate(inputs.Timestamp) {
		return rd.currentRegime
	}

	// Calculate regime signals from current inputs
	regimeVotes := rd.calculateRegimeVotes(inputs)

	// Apply majority voting across recent history
	newRegime := rd.majorityVote(regimeVotes)

	// Update current regime and timestamp
	rd.currentRegime = newRegime
	rd.lastUpdate = inputs.Timestamp

	return newRegime
}

// GetCurrentRegime returns the current regime without updating
func (rd *RegimeDetector) GetCurrentRegime() RegimeType {
	return rd.currentRegime
}

// GetLastUpdate returns the timestamp of the last regime update
func (rd *RegimeDetector) GetLastUpdate() time.Time {
	return rd.lastUpdate
}

// addToHistory adds inputs to history, maintaining max size
func (rd *RegimeDetector) addToHistory(inputs RegimeInputs) {
	rd.history = append(rd.history, inputs)

	// Trim history to max size
	if len(rd.history) > rd.maxHistory {
		rd.history = rd.history[1:]
	}
}

// shouldUpdate determines if it's time for a regime update based on cadence
func (rd *RegimeDetector) shouldUpdate(timestamp time.Time) bool {
	if rd.lastUpdate.IsZero() {
		return true // First update
	}

	return timestamp.Sub(rd.lastUpdate) >= rd.updateCadence
}

// calculateRegimeVotes computes regime signals for each indicator
func (rd *RegimeDetector) calculateRegimeVotes(inputs RegimeInputs) map[RegimeType]int {
	votes := make(map[RegimeType]int)

	// Indicator 1: Realized Volatility (7d)
	if inputs.RealizedVol7d > rd.thresholds.VolHighThreshold {
		votes[HighVol]++ // High volatility regime
	} else if inputs.RealizedVol7d < rd.thresholds.VolLowThreshold {
		// Low volatility - check breadth for trending vs choppy
		if inputs.BreadthThrust > rd.thresholds.ThrustPositive {
			votes[TrendingBull]++
		} else {
			votes[Choppy]++
		}
	} else {
		// Medium volatility - use breadth and % above MA
		if inputs.PctAbove20MA > rd.thresholds.BullThreshold {
			votes[TrendingBull]++
		} else {
			votes[Choppy]++
		}
	}

	// Indicator 2: % Above 20MA (universe breadth)
	if inputs.PctAbove20MA > rd.thresholds.BullThreshold {
		votes[TrendingBull]++ // Strong breadth = trending
	} else if inputs.PctAbove20MA < rd.thresholds.BearThreshold {
		votes[HighVol]++ // Weak breadth often coincides with high vol
	} else {
		votes[Choppy]++ // Neutral breadth = choppy
	}

	// Indicator 3: Breadth Thrust
	if inputs.BreadthThrust > rd.thresholds.ThrustPositive {
		votes[TrendingBull]++ // Positive thrust = trending
	} else if inputs.BreadthThrust < rd.thresholds.ThrustNegative {
		votes[HighVol]++ // Negative thrust often with volatility
	} else {
		votes[Choppy]++ // Neutral thrust = choppy
	}

	return votes
}

// majorityVote determines the final regime based on voting across indicators
func (rd *RegimeDetector) majorityVote(currentVotes map[RegimeType]int) RegimeType {
	// If we have insufficient history, use current votes only
	if len(rd.history) < 2 {
		return rd.findMaxVote(currentVotes)
	}

	// Aggregate votes across recent history (weighted toward recent)
	aggregateVotes := make(map[RegimeType]float64)

	// Process historical votes with decay weights
	for i, historicalInputs := range rd.history {
		weight := math.Pow(0.8, float64(len(rd.history)-i-1)) // Recent data weighted higher
		historicalVotes := rd.calculateRegimeVotes(historicalInputs)

		for regime, votes := range historicalVotes {
			aggregateVotes[regime] += float64(votes) * weight
		}
	}

	// Find regime with highest aggregated score
	var maxRegime RegimeType = Choppy // Default fallback
	var maxScore float64 = -1

	for regime, score := range aggregateVotes {
		if score > maxScore {
			maxScore = score
			maxRegime = regime
		}
	}

	// Apply stability bias - require significant change to switch regimes
	if maxRegime != rd.currentRegime {
		// Require at least 20% higher score to switch
		currentScore := aggregateVotes[rd.currentRegime]
		if maxScore < currentScore*1.2 {
			return rd.currentRegime // Stay in current regime
		}
	}

	return maxRegime
}

// findMaxVote finds the regime with the most votes
func (rd *RegimeDetector) findMaxVote(votes map[RegimeType]int) RegimeType {
	maxVotes := -1
	var result RegimeType = Choppy // Safe default

	for regime, count := range votes {
		if count > maxVotes {
			maxVotes = count
			result = regime
		}
	}

	return result
}

// GetRegimeHistory returns the recent regime detection history
func (rd *RegimeDetector) GetRegimeHistory() []RegimeInputs {
	return append([]RegimeInputs(nil), rd.history...) // Return copy
}

// ValidateInputs checks if regime inputs are within reasonable ranges
func (rd *RegimeDetector) ValidateInputs(inputs RegimeInputs) error {
	if inputs.RealizedVol7d < 0 || inputs.RealizedVol7d > 2.0 {
		return fmt.Errorf("realized volatility out of range: %f (expected 0-2.0)", inputs.RealizedVol7d)
	}

	if inputs.PctAbove20MA < 0 || inputs.PctAbove20MA > 1.0 {
		return fmt.Errorf("percent above 20MA out of range: %f (expected 0-1.0)", inputs.PctAbove20MA)
	}

	if inputs.BreadthThrust < -1.0 || inputs.BreadthThrust > 1.0 {
		return fmt.Errorf("breadth thrust out of range: %f (expected -1.0 to 1.0)", inputs.BreadthThrust)
	}

	if inputs.Timestamp.IsZero() {
		return fmt.Errorf("timestamp is required")
	}

	return nil
}

// GetDetectorStatus returns current detector configuration and status
func (rd *RegimeDetector) GetDetectorStatus() map[string]interface{} {
	return map[string]interface{}{
		"current_regime": rd.currentRegime.String(),
		"last_update":    rd.lastUpdate.Format(time.RFC3339),
		"update_cadence": rd.updateCadence.String(),
		"history_length": len(rd.history),
		"max_history":    rd.maxHistory,
		"thresholds":     rd.thresholds,
	}
}
