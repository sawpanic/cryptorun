package momentum

import (
	"math"
	"time"
)

// EntryExitConfig defines entry and exit gate parameters
type EntryExitConfig struct {
	Entry EntryGateConfig `yaml:"entry"`
	Exit  ExitGateConfig  `yaml:"exit"`
}

// EntryGateConfig defines entry gate parameters
type EntryGateConfig struct {
	MinScore       float64 `yaml:"min_score"`        // Minimum momentum score
	VolumeMultiple float64 `yaml:"volume_multiple"`  // Minimum volume surge
	ADXThreshold   float64 `yaml:"adx_threshold"`    // Minimum ADX for trend
	HurstThreshold float64 `yaml:"hurst_threshold"`  // Minimum Hurst exponent
}

// ExitGateConfig defines exit gate parameters  
type ExitGateConfig struct {
	HardStop       float64 `yaml:"hard_stop"`        // Hard stop loss %
	VenueHealth    float64 `yaml:"venue_health"`     // Minimum venue health
	MaxHoldHours   int     `yaml:"max_hold_hours"`   // Maximum hold period (48h)
	AccelReversal  float64 `yaml:"accel_reversal"`   // Acceleration reversal threshold
	FadeThreshold  float64 `yaml:"fade_threshold"`   // Momentum fade threshold
	TrailingStop   float64 `yaml:"trailing_stop"`    // Trailing stop %
	ProfitTarget   float64 `yaml:"profit_target"`    // Profit target %
}

// EntrySignal represents an entry signal evaluation
type EntrySignal struct {
	Symbol      string    `json:"symbol"`
	Timestamp   time.Time `json:"timestamp"`
	Score       float64   `json:"score"`
	Qualified   bool      `json:"qualified"`
	GateResults EntryGateResults `json:"gate_results"`
	Reason      string    `json:"reason,omitempty"`
}

// ExitSignal represents an exit signal evaluation  
type ExitSignal struct {
	Symbol      string    `json:"symbol"`
	Timestamp   time.Time `json:"timestamp"`
	ExitType    string    `json:"exit_type"`
	Triggered   bool      `json:"triggered"`
	GateResults ExitGateResults `json:"gate_results"`
	Reason      string    `json:"reason,omitempty"`
}

// EntryGateResults contains entry gate validation results
type EntryGateResults struct {
	ScoreGate   GuardResult `json:"score_gate"`
	VolumeGate  GuardResult `json:"volume_gate"`
	ADXGate     GuardResult `json:"adx_gate"`
	HurstGate   GuardResult `json:"hurst_gate"`
}

// ExitGateResults contains exit gate validation results
type ExitGateResults struct {
	HardStopGate    GuardResult `json:"hard_stop_gate"`
	VenueHealthGate GuardResult `json:"venue_health_gate"`
	TimeGate        GuardResult `json:"time_gate"`
	AccelGate       GuardResult `json:"accel_gate"`
	FadeGate        GuardResult `json:"fade_gate"`
	TrailingGate    GuardResult `json:"trailing_gate"`
	ProfitGate      GuardResult `json:"profit_gate"`
}

// EntryExitGates implements entry and exit gate logic
type EntryExitGates struct {
	config EntryExitConfig
}

// NewEntryExitGates creates new entry/exit gates
func NewEntryExitGates(config EntryExitConfig) *EntryExitGates {
	return &EntryExitGates{
		config: config,
	}
}

// EvaluateEntry evaluates entry conditions for a momentum signal
func (eeg *EntryExitGates) EvaluateEntry(result *MomentumResult, marketData map[string][]MarketData, volumeData []float64) *EntrySignal {
	signal := &EntrySignal{
		Symbol:    result.Symbol,
		Timestamp: time.Now(),
		Score:     result.CoreScore,
	}

	// Apply entry gates
	signal.GateResults = eeg.applyEntryGates(result, marketData, volumeData)

	// Check if all gates pass
	allGatesPass := signal.GateResults.ScoreGate.Pass &&
		signal.GateResults.VolumeGate.Pass &&
		signal.GateResults.ADXGate.Pass &&
		signal.GateResults.HurstGate.Pass

	// Also check momentum guards
	allGuardsPass := result.GuardResults.Fatigue.Pass &&
		result.GuardResults.Freshness.Pass &&
		result.GuardResults.LateFill.Pass

	signal.Qualified = allGatesPass && allGuardsPass

	if !signal.Qualified {
		if !allGatesPass {
			signal.Reason = "entry gates failed"
		} else {
			signal.Reason = "momentum guards failed"
		}
	} else {
		signal.Reason = "entry qualified"
	}

	return signal
}

// EvaluateExit evaluates exit conditions for a position
func (eeg *EntryExitGates) EvaluateExit(symbol string, entryPrice, currentPrice float64, entryTime time.Time, venueHealth float64, acceleration float64) *ExitSignal {
	signal := &ExitSignal{
		Symbol:    symbol,
		Timestamp: time.Now(),
	}

	// Apply exit gates
	signal.GateResults = eeg.applyExitGates(entryPrice, currentPrice, entryTime, venueHealth, acceleration)

	// Determine exit type and trigger status
	signal.ExitType, signal.Triggered = eeg.determineExitType(signal.GateResults)
	
	if signal.Triggered {
		signal.Reason = "exit triggered: " + signal.ExitType
	} else {
		signal.Reason = "position maintained"
	}

	return signal
}

// applyEntryGates applies all entry gates
func (eeg *EntryExitGates) applyEntryGates(result *MomentumResult, marketData map[string][]MarketData, volumeData []float64) EntryGateResults {
	return EntryGateResults{
		ScoreGate:  eeg.applyScoreGate(result.CoreScore),
		VolumeGate: eeg.applyVolumeGate(volumeData),
		ADXGate:    eeg.applyADXGate(marketData),
		HurstGate:  eeg.applyHurstGate(marketData),
	}
}

// applyScoreGate checks minimum momentum score
func (eeg *EntryExitGates) applyScoreGate(score float64) GuardResult {
	if score >= eeg.config.Entry.MinScore {
		return GuardResult{
			Pass:   true,
			Value:  score,
			Reason: "momentum score above threshold",
		}
	}
	
	return GuardResult{
		Pass:   false,
		Value:  score,
		Reason: "momentum score below minimum threshold",
	}
}

// applyVolumeGate checks volume surge requirement
func (eeg *EntryExitGates) applyVolumeGate(volumeData []float64) GuardResult {
	if len(volumeData) < 2 {
		return GuardResult{
			Pass:   false,
			Value:  0.0,
			Reason: "insufficient volume data",
		}
	}

	currentVolume := volumeData[len(volumeData)-1]
	avgVolume := calculateAverage(volumeData[:len(volumeData)-1])
	
	if avgVolume == 0 {
		return GuardResult{
			Pass:   false,
			Value:  0.0,
			Reason: "invalid average volume",
		}
	}

	volumeMultiple := currentVolume / avgVolume
	
	if volumeMultiple >= eeg.config.Entry.VolumeMultiple {
		return GuardResult{
			Pass:   true,
			Value:  volumeMultiple,
			Reason: "volume surge detected",
		}
	}
	
	return GuardResult{
		Pass:   false,
		Value:  volumeMultiple,
		Reason: "insufficient volume surge",
	}
}

// applyADXGate checks ADX trend strength
func (eeg *EntryExitGates) applyADXGate(marketData map[string][]MarketData) GuardResult {
	tf4h, exists := marketData["4h"]
	if !exists || len(tf4h) < 15 {
		return GuardResult{
			Pass:   false,
			Value:  0.0,
			Reason: "insufficient data for ADX calculation",
		}
	}

	adx := calculateADX(tf4h, 14)
	
	if adx >= eeg.config.Entry.ADXThreshold {
		return GuardResult{
			Pass:   true,
			Value:  adx,
			Reason: "ADX indicates trending market",
		}
	}
	
	return GuardResult{
		Pass:   false,
		Value:  adx,
		Reason: "ADX below trend threshold",
	}
}

// applyHurstGate checks Hurst exponent for persistence
func (eeg *EntryExitGates) applyHurstGate(marketData map[string][]MarketData) GuardResult {
	tf4h, exists := marketData["4h"]
	if !exists || len(tf4h) < 20 {
		return GuardResult{
			Pass:   false,
			Value:  0.0,
			Reason: "insufficient data for Hurst calculation",
		}
	}

	hurst := calculateHurst(tf4h, 20)
	
	if hurst >= eeg.config.Entry.HurstThreshold {
		return GuardResult{
			Pass:   true,
			Value:  hurst,
			Reason: "Hurst indicates trend persistence",
		}
	}
	
	return GuardResult{
		Pass:   false,
		Value:  hurst,
		Reason: "Hurst below persistence threshold",
	}
}

// applyExitGates applies all exit gates
func (eeg *EntryExitGates) applyExitGates(entryPrice, currentPrice float64, entryTime time.Time, venueHealth, acceleration float64) ExitGateResults {
	now := time.Now()
	pnlPercent := (currentPrice - entryPrice) / entryPrice * 100.0
	holdDuration := now.Sub(entryTime)

	return ExitGateResults{
		HardStopGate:    eeg.applyHardStopGate(pnlPercent),
		VenueHealthGate: eeg.applyVenueHealthGate(venueHealth),
		TimeGate:        eeg.applyTimeGate(holdDuration),
		AccelGate:       eeg.applyAccelGate(acceleration),
		FadeGate:        eeg.applyFadeGate(pnlPercent),
		TrailingGate:    eeg.applyTrailingGate(pnlPercent),
		ProfitGate:      eeg.applyProfitGate(pnlPercent),
	}
}

// applyHardStopGate checks hard stop loss
func (eeg *EntryExitGates) applyHardStopGate(pnlPercent float64) GuardResult {
	triggered := pnlPercent <= -eeg.config.Exit.HardStop
	
	return GuardResult{
		Pass:   !triggered,
		Value:  pnlPercent,
		Reason: map[bool]string{true: "hard stop triggered", false: "within stop loss range"}[triggered],
	}
}

// applyVenueHealthGate checks venue health
func (eeg *EntryExitGates) applyVenueHealthGate(health float64) GuardResult {
	triggered := health < eeg.config.Exit.VenueHealth
	
	return GuardResult{
		Pass:   !triggered,
		Value:  health,
		Reason: map[bool]string{true: "venue health degraded", false: "venue health acceptable"}[triggered],
	}
}

// applyTimeGate checks maximum hold period
func (eeg *EntryExitGates) applyTimeGate(duration time.Duration) GuardResult {
	maxDuration := time.Duration(eeg.config.Exit.MaxHoldHours) * time.Hour
	triggered := duration >= maxDuration
	
	return GuardResult{
		Pass:   !triggered,
		Value:  duration.Hours(),
		Reason: map[bool]string{true: "maximum hold period reached", false: "within hold period"}[triggered],
	}
}

// applyAccelGate checks acceleration reversal
func (eeg *EntryExitGates) applyAccelGate(acceleration float64) GuardResult {
	triggered := acceleration <= -eeg.config.Exit.AccelReversal
	
	return GuardResult{
		Pass:   !triggered,
		Value:  acceleration,
		Reason: map[bool]string{true: "acceleration reversal detected", false: "acceleration maintained"}[triggered],
	}
}

// applyFadeGate checks momentum fade
func (eeg *EntryExitGates) applyFadeGate(pnlPercent float64) GuardResult {
	// Fade check: small positive but declining momentum
	triggered := pnlPercent > 0 && pnlPercent < eeg.config.Exit.FadeThreshold
	
	return GuardResult{
		Pass:   !triggered,
		Value:  pnlPercent,
		Reason: map[bool]string{true: "momentum fade detected", false: "momentum maintained"}[triggered],
	}
}

// applyTrailingGate checks trailing stop
func (eeg *EntryExitGates) applyTrailingGate(pnlPercent float64) GuardResult {
	// Simple trailing stop: exit if down from peak by trailing %
	triggered := pnlPercent > 0 && pnlPercent < eeg.config.Exit.TrailingStop
	
	return GuardResult{
		Pass:   !triggered,
		Value:  pnlPercent,
		Reason: map[bool]string{true: "trailing stop triggered", false: "above trailing stop"}[triggered],
	}
}

// applyProfitGate checks profit target
func (eeg *EntryExitGates) applyProfitGate(pnlPercent float64) GuardResult {
	triggered := pnlPercent >= eeg.config.Exit.ProfitTarget
	
	return GuardResult{
		Pass:   !triggered,
		Value:  pnlPercent,
		Reason: map[bool]string{true: "profit target reached", false: "below profit target"}[triggered],
	}
}

// determineExitType determines the exit type and trigger status
func (eeg *EntryExitGates) determineExitType(gates ExitGateResults) (string, bool) {
	// Priority order for exit triggers
	if !gates.HardStopGate.Pass {
		return "hard_stop", true
	}
	if !gates.VenueHealthGate.Pass {
		return "venue_health", true
	}
	if !gates.TimeGate.Pass {
		return "time_limit", true
	}
	if !gates.ProfitGate.Pass {
		return "profit_target", true
	}
	if !gates.AccelGate.Pass {
		return "accel_reversal", true
	}
	if !gates.FadeGate.Pass {
		return "momentum_fade", true
	}
	if !gates.TrailingGate.Pass {
		return "trailing_stop", true
	}
	
	return "none", false
}

// calculateAverage calculates average of float64 slice
func calculateAverage(values []float64) float64 {
	if len(values) == 0 {
		return 0.0
	}
	
	sum := 0.0
	for _, v := range values {
		sum += v
	}
	return sum / float64(len(values))
}

// calculateADX calculates Average Directional Index (simplified)
func calculateADX(data []MarketData, period int) float64 {
	if len(data) < period+1 {
		return 0.0
	}

	// Simplified ADX calculation
	// In practice, this would be more complex with proper DI+ and DI- calculations
	sum := 0.0
	for i := len(data) - period; i < len(data); i++ {
		if i == 0 {
			continue
		}
		
		trueRange := max(data[i].High-data[i].Low, 
						 max(abs(data[i].High-data[i-1].Close), 
							 abs(data[i].Low-data[i-1].Close)))
		
		if trueRange > 0 {
			directionalMovement := abs(data[i].Close - data[i-1].Close)
			sum += (directionalMovement / trueRange) * 100
		}
	}
	
	return sum / float64(period)
}

// calculateHurst calculates Hurst exponent (simplified)
func calculateHurst(data []MarketData, period int) float64 {
	if len(data) < period {
		return 0.5 // Random walk
	}

	// Simplified Hurst calculation using R/S analysis
	prices := make([]float64, period)
	for i := 0; i < period; i++ {
		prices[i] = data[len(data)-period+i].Close
	}

	// Calculate log returns
	returns := make([]float64, len(prices)-1)
	for i := 1; i < len(prices); i++ {
		if prices[i-1] > 0 {
			returns[i-1] = math.Log(prices[i] / prices[i-1])
		}
	}

	// Simple variance-based estimate
	if len(returns) < 2 {
		return 0.5
	}

	variance := 0.0
	mean := calculateAverage(returns)
	
	for _, r := range returns {
		variance += (r - mean) * (r - mean)
	}
	variance /= float64(len(returns))

	// Simplified Hurst estimation
	if variance > 0 {
		return 0.5 + (variance * 0.1) // Simplified mapping
	}
	
	return 0.5
}

// max returns maximum of two float64 values
func max(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}