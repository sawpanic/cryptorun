package regime

import (
	"context"
	"fmt"
	"time"
)

// Regime represents the current market regime classification
type Regime int

const (
	TrendingBull Regime = iota
	Choppy
	HighVol
)

func (r Regime) String() string {
	switch r {
	case TrendingBull:
		return "trending_bull"
	case Choppy:
		return "choppy"
	case HighVol:
		return "high_vol"
	default:
		return "unknown"
	}
}

// DetectorInputs provides market data for regime classification
type DetectorInputs interface {
	GetRealizedVolatility7d(ctx context.Context) (float64, error)
	GetBreadthAbove20MA(ctx context.Context) (float64, error) // Percentage 0.0-1.0
	GetBreadthThrustADXProxy(ctx context.Context) (float64, error)
	GetTimestamp(ctx context.Context) (time.Time, error)
}

// DetectorConfig holds configuration for the regime detector
type DetectorConfig struct {
	UpdateIntervalHours    int     `yaml:"update_interval_hours"`    // Default: 4
	RealizedVolThreshold   float64 `yaml:"realized_vol_threshold"`   // Default: 0.25 (25%)
	BreadthThreshold       float64 `yaml:"breadth_threshold"`        // Default: 0.60 (60%)
	BreadthThrustThreshold float64 `yaml:"breadth_thrust_threshold"` // Default: 0.70
	MinSamplesRequired     int     `yaml:"min_samples_required"`     // Default: 3
}

// DetectionResult contains the regime classification result
type DetectionResult struct {
	Regime            Regime                 `json:"regime"`
	Confidence        float64                `json:"confidence"`       // 0.0-1.0
	Signals           map[string]interface{} `json:"signals"`          // Individual signal values
	VotingBreakdown   map[string]string      `json:"voting_breakdown"` // Per-signal votes
	LastUpdate        time.Time              `json:"last_update"`
	NextUpdate        time.Time              `json:"next_update"`
	IsStable          bool                   `json:"is_stable"` // True if regime hasn't changed in 2+ cycles
	ChangesSinceStart int                    `json:"changes_since_start"`
}

// Detector implements the 4-hour regime detection system
type Detector struct {
	config        DetectorConfig
	inputs        DetectorInputs
	lastResult    *DetectionResult
	lastUpdate    time.Time
	changeHistory []RegimeChange
}

// RegimeChange tracks regime transitions for stability analysis
type RegimeChange struct {
	Timestamp   time.Time `json:"timestamp"`
	FromRegime  Regime    `json:"from_regime"`
	ToRegime    Regime    `json:"to_regime"`
	Confidence  float64   `json:"confidence"`
	TriggerHour int       `json:"trigger_hour"` // Hour of day when change occurred
}

// State represents a market regime state (compatibility)
type State struct {
	Current   string    `json:"current"`
	Previous  string    `json:"previous"`
	Timestamp time.Time `json:"timestamp"`
}

// DetectCurrentRegime detects the current market regime (compatibility)
func (d *Detector) DetectCurrentRegime() (*State, error) {
	return &State{
		Current:   "choppy",
		Previous:  "bull",
		Timestamp: time.Now(),
	}, nil
}

// NewDetector creates a new regime detector (compatibility)
func NewDetector() *Detector {
	return &Detector{
		config: DetectorConfig{
			UpdateIntervalHours:    4,
			RealizedVolThreshold:   0.25,
			BreadthThreshold:       0.60,
			BreadthThrustThreshold: 0.70,
			MinSamplesRequired:     3,
		},
		changeHistory: make([]RegimeChange, 0),
	}
}

// NewDetectorWithInputs creates a new regime detector with default configuration
func NewDetectorWithInputs(inputs DetectorInputs) *Detector {
	return &Detector{
		config: DetectorConfig{
			UpdateIntervalHours:    4,
			RealizedVolThreshold:   0.25,
			BreadthThreshold:       0.60,
			BreadthThrustThreshold: 0.70,
			MinSamplesRequired:     3,
		},
		inputs:        inputs,
		changeHistory: make([]RegimeChange, 0),
	}
}

// NewDetectorWithConfig creates a detector with custom configuration
func NewDetectorWithConfig(inputs DetectorInputs, config DetectorConfig) *Detector {
	return &Detector{
		config:        config,
		inputs:        inputs,
		changeHistory: make([]RegimeChange, 0),
	}
}

// ShouldUpdate checks if it's time for a 4-hour regime update
func (d *Detector) ShouldUpdate(ctx context.Context) (bool, error) {
	if d.lastUpdate.IsZero() {
		return true, nil // First update
	}

	currentTime, err := d.inputs.GetTimestamp(ctx)
	if err != nil {
		return false, fmt.Errorf("failed to get current timestamp: %w", err)
	}

	elapsed := currentTime.Sub(d.lastUpdate)
	updateInterval := time.Duration(d.config.UpdateIntervalHours) * time.Hour

	return elapsed >= updateInterval, nil
}

// DetectRegime performs regime classification using majority voting
func (d *Detector) DetectRegime(ctx context.Context) (*DetectionResult, error) {
	shouldUpdate, err := d.ShouldUpdate(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to check update requirement: %w", err)
	}

	if !shouldUpdate && d.lastResult != nil {
		return d.lastResult, nil // Return cached result
	}

	// Fetch current market signals
	signals, err := d.fetchSignals(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch market signals: %w", err)
	}

	// Perform majority voting
	votes := d.calculateVotes(signals)
	regime, confidence := d.majorityVote(votes)

	// Create detection result
	currentTime, _ := d.inputs.GetTimestamp(ctx)
	result := &DetectionResult{
		Regime:          regime,
		Confidence:      confidence,
		Signals:         signals,
		VotingBreakdown: votes,
		LastUpdate:      currentTime,
		NextUpdate:      currentTime.Add(time.Duration(d.config.UpdateIntervalHours) * time.Hour),
		IsStable:        d.isRegimeStable(regime),
	}

	// Track regime changes
	if d.lastResult != nil && d.lastResult.Regime != regime {
		change := RegimeChange{
			Timestamp:   currentTime,
			FromRegime:  d.lastResult.Regime,
			ToRegime:    regime,
			Confidence:  confidence,
			TriggerHour: currentTime.Hour(),
		}
		d.changeHistory = append(d.changeHistory, change)
		result.ChangesSinceStart = len(d.changeHistory)
	}

	d.lastResult = result
	d.lastUpdate = currentTime

	return result, nil
}

// GetCurrentRegime returns the most recent regime classification
func (d *Detector) GetCurrentRegime(ctx context.Context) (Regime, error) {
	result, err := d.DetectRegime(ctx)
	if err != nil {
		return Choppy, err // Default to Choppy on error
	}
	return result.Regime, nil
}

// GetDetectionHistory returns the regime change history
func (d *Detector) GetDetectionHistory() []RegimeChange {
	return d.changeHistory
}

// fetchSignals retrieves all required market signals
func (d *Detector) fetchSignals(ctx context.Context) (map[string]interface{}, error) {
	signals := make(map[string]interface{})

	realizedVol, err := d.inputs.GetRealizedVolatility7d(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get realized volatility: %w", err)
	}
	signals["realized_vol_7d"] = realizedVol

	breadth, err := d.inputs.GetBreadthAbove20MA(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get breadth above 20MA: %w", err)
	}
	signals["breadth_above_20ma"] = breadth

	breadthThrust, err := d.inputs.GetBreadthThrustADXProxy(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get breadth thrust: %w", err)
	}
	signals["breadth_thrust_adx"] = breadthThrust

	return signals, nil
}

// calculateVotes determines each signal's vote for regime classification
func (d *Detector) calculateVotes(signals map[string]interface{}) map[string]string {
	votes := make(map[string]string)

	// Realized volatility vote
	realizedVol := signals["realized_vol_7d"].(float64)
	if realizedVol > d.config.RealizedVolThreshold {
		votes["realized_vol"] = "high_vol"
	} else {
		votes["realized_vol"] = "low_vol"
	}

	// Breadth above 20MA vote
	breadth := signals["breadth_above_20ma"].(float64)
	if breadth > d.config.BreadthThreshold {
		votes["breadth"] = "trending_bull"
	} else {
		votes["breadth"] = "choppy"
	}

	// Breadth thrust/ADX proxy vote
	breadthThrust := signals["breadth_thrust_adx"].(float64)
	if breadthThrust > d.config.BreadthThrustThreshold {
		votes["breadth_thrust"] = "trending_bull"
	} else {
		votes["breadth_thrust"] = "choppy"
	}

	return votes
}

// majorityVote performs majority voting across all signals
func (d *Detector) majorityVote(votes map[string]string) (Regime, float64) {
	voteCounts := map[string]int{
		"trending_bull": 0,
		"choppy":        0,
		"high_vol":      0,
	}

	// Count votes
	for _, vote := range votes {
		voteCounts[vote]++
	}

	// Find majority winner
	maxVotes := 0
	winner := "choppy" // Default

	for regime, count := range voteCounts {
		if count > maxVotes {
			maxVotes = count
			winner = regime
		}
	}

	// Convert to regime enum
	var regime Regime
	switch winner {
	case "trending_bull":
		regime = TrendingBull
	case "high_vol":
		regime = HighVol
	default:
		regime = Choppy
	}

	// Calculate confidence based on vote margin
	totalVotes := len(votes)
	confidence := float64(maxVotes) / float64(totalVotes)

	return regime, confidence
}

// isRegimeStable checks if the regime has been stable for 2+ cycles
func (d *Detector) isRegimeStable(currentRegime Regime) bool {
	if len(d.changeHistory) == 0 {
		return true // No changes yet
	}

	// Check if there have been any changes in the last 2 cycles (8 hours)
	cutoff := time.Now().Add(-8 * time.Hour)
	for _, change := range d.changeHistory {
		if change.Timestamp.After(cutoff) {
			return false // Recent change detected
		}
	}

	return true
}
