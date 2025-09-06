package exits

import (
	"context"
	"fmt"
	"time"
)

// ExitReason represents the reason for exit with precedence
type ExitReason int

const (
	NoExit ExitReason = iota
	HardStop           // Highest precedence: hard stop loss hit
	VenueHealthCut     // Venue degradation (P99 latency, error rates)
	TimeLimit          // 48-hour time limit reached
	AccelerationReversal // Momentum acceleration has reversed
	MomentumFade       // Momentum factor has faded significantly
	TrailingStop       // Trailing stop activated
	ProfitTarget       // Profit target achieved (lowest precedence)
)

func (er ExitReason) String() string {
	switch er {
	case NoExit:
		return "no_exit"
	case HardStop:
		return "hard_stop"
	case VenueHealthCut:
		return "venue_health_cut"
	case TimeLimit:
		return "time_limit"
	case AccelerationReversal:
		return "acceleration_reversal"
	case MomentumFade:
		return "momentum_fade"
	case TrailingStop:
		return "trailing_stop"
	case ProfitTarget:
		return "profit_target"
	default:
		return "unknown"
	}
}

// ExitResult contains the exit evaluation outcome
type ExitResult struct {
	Symbol           string     `json:"symbol"`
	Timestamp        time.Time  `json:"timestamp"`
	ShouldExit       bool       `json:"should_exit"`
	ExitReason       ExitReason `json:"exit_reason"`
	ReasonString     string     `json:"reason_string"`
	TriggeredBy      string     `json:"triggered_by"`      // Specific trigger description
	CurrentPrice     float64    `json:"current_price"`
	EntryPrice       float64    `json:"entry_price"`
	UnrealizedPnL    float64    `json:"unrealized_pnl"`    // % return
	HoursHeld        float64    `json:"hours_held"`
	EvaluationTimeMs int64      `json:"evaluation_time_ms"`
}

// ExitInputs contains all data required for exit evaluation
type ExitInputs struct {
	Symbol              string    `json:"symbol"`
	EntryPrice          float64   `json:"entry_price"`
	CurrentPrice        float64   `json:"current_price"`
	EntryTime           time.Time `json:"entry_time"`
	CurrentTime         time.Time `json:"current_time"`
	
	// Hard stop
	HardStopPrice       float64   `json:"hard_stop_price"`       // Absolute stop loss price
	
	// Venue health metrics
	VenueP99LatencyMs   int64     `json:"venue_p99_latency_ms"`  // P99 latency in ms
	VenueErrorRate      float64   `json:"venue_error_rate"`      // Error rate %
	VenueRejectRate     float64   `json:"venue_reject_rate"`     // Order reject rate %
	
	// Time-based exit
	MaxHoldHours        float64   `json:"max_hold_hours"`        // Maximum hold time (default 48h)
	
	// Momentum metrics
	MomentumScore       float64   `json:"momentum_score"`        // Current momentum score
	EntryMomentumScore  float64   `json:"entry_momentum_score"`  // Momentum at entry
	MomentumAccel       float64   `json:"momentum_accel"`        // Acceleration metric
	EntryAccel          float64   `json:"entry_accel"`           // Acceleration at entry
	
	// Trailing stop
	HighWaterMark       float64   `json:"high_water_mark"`       // Highest price since entry
	TrailingStopPct     float64   `json:"trailing_stop_pct"`     // Trailing stop percentage
	
	// Profit targets
	ProfitTarget1       float64   `json:"profit_target_1"`       // First profit target %
	ProfitTarget2       float64   `json:"profit_target_2"`       // Second profit target %
	ProfitTargetPrice1  float64   `json:"profit_target_price_1"` // Calculated price for target 1
	ProfitTargetPrice2  float64   `json:"profit_target_price_2"` // Calculated price for target 2
}

// ExitEvaluator evaluates exit conditions with proper precedence
type ExitEvaluator struct {
	config *ExitConfig
}

// ExitConfig contains exit rule configuration
type ExitConfig struct {
	// Hard stop configuration
	EnableHardStop bool `yaml:"enable_hard_stop"`

	// Venue health thresholds
	MaxVenueP99LatencyMs int64   `yaml:"max_venue_p99_latency_ms"` // 2000ms default
	MaxVenueErrorRate    float64 `yaml:"max_venue_error_rate"`     // 3% default
	MaxVenueRejectRate   float64 `yaml:"max_venue_reject_rate"`    // 5% default

	// Time limit
	DefaultMaxHoldHours float64 `yaml:"default_max_hold_hours"` // 48 hours default

	// Momentum fade thresholds
	MomentumFadeThreshold   float64 `yaml:"momentum_fade_threshold"`   // 30% fade from entry
	AccelReversalThreshold  float64 `yaml:"accel_reversal_threshold"`  // 50% accel loss

	// Trailing stop configuration
	EnableTrailingStop   bool    `yaml:"enable_trailing_stop"`
	DefaultTrailingPct   float64 `yaml:"default_trailing_pct"` // 5% default

	// Profit targets
	EnableProfitTargets  bool    `yaml:"enable_profit_targets"`
	DefaultProfitTarget1 float64 `yaml:"default_profit_target_1"` // 15% default
	DefaultProfitTarget2 float64 `yaml:"default_profit_target_2"` // 30% default
}

// DefaultExitConfig returns production-ready exit configuration
func DefaultExitConfig() *ExitConfig {
	return &ExitConfig{
		EnableHardStop:          true,
		MaxVenueP99LatencyMs:    2000,  // 2 seconds
		MaxVenueErrorRate:       3.0,   // 3%
		MaxVenueRejectRate:      5.0,   // 5%
		DefaultMaxHoldHours:     48.0,  // 48 hours
		MomentumFadeThreshold:   30.0,  // 30% fade
		AccelReversalThreshold:  50.0,  // 50% accel loss
		EnableTrailingStop:      true,
		DefaultTrailingPct:      5.0,   // 5%
		EnableProfitTargets:     true,
		DefaultProfitTarget1:    15.0,  // 15%
		DefaultProfitTarget2:    30.0,  // 30%
	}
}

// NewExitEvaluator creates a new exit evaluator
func NewExitEvaluator(config *ExitConfig) *ExitEvaluator {
	if config == nil {
		config = DefaultExitConfig()
	}
	return &ExitEvaluator{config: config}
}

// EvaluateExit performs exit evaluation with proper precedence
func (ee *ExitEvaluator) EvaluateExit(ctx context.Context, inputs ExitInputs) (*ExitResult, error) {
	startTime := time.Now()
	
	result := &ExitResult{
		Symbol:        inputs.Symbol,
		Timestamp:     inputs.CurrentTime,
		ShouldExit:    false,
		ExitReason:    NoExit,
		CurrentPrice:  inputs.CurrentPrice,
		EntryPrice:    inputs.EntryPrice,
		UnrealizedPnL: ((inputs.CurrentPrice / inputs.EntryPrice) - 1.0) * 100,
		HoursHeld:     inputs.CurrentTime.Sub(inputs.EntryTime).Hours(),
	}

	// Evaluate exits in precedence order (highest to lowest)
	
	// 1. Hard Stop (highest precedence)
	if ee.config.EnableHardStop && ee.evaluateHardStop(inputs) {
		result.ShouldExit = true
		result.ExitReason = HardStop
		result.ReasonString = "hard_stop"
		result.TriggeredBy = fmt.Sprintf("Price %.4f ≤ stop %.4f", inputs.CurrentPrice, inputs.HardStopPrice)
	}
	
	// 2. Venue Health Cut
	if !result.ShouldExit && ee.evaluateVenueHealth(inputs) {
		result.ShouldExit = true
		result.ExitReason = VenueHealthCut
		result.ReasonString = "venue_health_cut"
		result.TriggeredBy = fmt.Sprintf("Venue degraded: P99=%dms, err=%.1f%%, rej=%.1f%%", 
			inputs.VenueP99LatencyMs, inputs.VenueErrorRate, inputs.VenueRejectRate)
	}
	
	// 3. Time Limit (48 hours)
	if !result.ShouldExit && ee.evaluateTimeLimit(inputs) {
		result.ShouldExit = true
		result.ExitReason = TimeLimit
		result.ReasonString = "time_limit"
		result.TriggeredBy = fmt.Sprintf("Held %.1f hours ≥ %.1f hour limit", 
			result.HoursHeld, inputs.MaxHoldHours)
	}
	
	// 4. Acceleration Reversal
	if !result.ShouldExit && ee.evaluateAccelerationReversal(inputs) {
		result.ShouldExit = true
		result.ExitReason = AccelerationReversal
		result.ReasonString = "acceleration_reversal"
		accelChange := ((inputs.MomentumAccel / inputs.EntryAccel) - 1.0) * 100
		result.TriggeredBy = fmt.Sprintf("Acceleration reversed: %.1f%% change", accelChange)
	}
	
	// 5. Momentum Fade
	if !result.ShouldExit && ee.evaluateMomentumFade(inputs) {
		result.ShouldExit = true
		result.ExitReason = MomentumFade
		result.ReasonString = "momentum_fade"
		momentumChange := ((inputs.MomentumScore / inputs.EntryMomentumScore) - 1.0) * 100
		result.TriggeredBy = fmt.Sprintf("Momentum faded: %.1f%% decline", momentumChange)
	}
	
	// 6. Trailing Stop
	if !result.ShouldExit && ee.config.EnableTrailingStop && ee.evaluateTrailingStop(inputs) {
		result.ShouldExit = true
		result.ExitReason = TrailingStop
		result.ReasonString = "trailing_stop"
		stopPrice := inputs.HighWaterMark * (1.0 - inputs.TrailingStopPct/100.0)
		result.TriggeredBy = fmt.Sprintf("Trailing stop: %.4f < %.4f (%.1f%% from HWM %.4f)", 
			inputs.CurrentPrice, stopPrice, inputs.TrailingStopPct, inputs.HighWaterMark)
	}
	
	// 7. Profit Targets (lowest precedence)
	if !result.ShouldExit && ee.config.EnableProfitTargets && ee.evaluateProfitTargets(inputs) {
		result.ShouldExit = true
		result.ExitReason = ProfitTarget
		result.ReasonString = "profit_target"
		
		if inputs.CurrentPrice >= inputs.ProfitTargetPrice2 {
			result.TriggeredBy = fmt.Sprintf("Profit target 2 hit: %.4f ≥ %.4f (+%.1f%%)", 
				inputs.CurrentPrice, inputs.ProfitTargetPrice2, inputs.ProfitTarget2)
		} else if inputs.CurrentPrice >= inputs.ProfitTargetPrice1 {
			result.TriggeredBy = fmt.Sprintf("Profit target 1 hit: %.4f ≥ %.4f (+%.1f%%)", 
				inputs.CurrentPrice, inputs.ProfitTargetPrice1, inputs.ProfitTarget1)
		}
	}

	result.EvaluationTimeMs = time.Since(startTime).Milliseconds()
	return result, nil
}

// evaluateHardStop checks if hard stop loss is triggered
func (ee *ExitEvaluator) evaluateHardStop(inputs ExitInputs) bool {
	return inputs.HardStopPrice > 0 && inputs.CurrentPrice <= inputs.HardStopPrice
}

// evaluateVenueHealth checks if venue performance is degraded
func (ee *ExitEvaluator) evaluateVenueHealth(inputs ExitInputs) bool {
	return inputs.VenueP99LatencyMs > ee.config.MaxVenueP99LatencyMs ||
		   inputs.VenueErrorRate > ee.config.MaxVenueErrorRate ||
		   inputs.VenueRejectRate > ee.config.MaxVenueRejectRate
}

// evaluateTimeLimit checks if maximum hold time is reached
func (ee *ExitEvaluator) evaluateTimeLimit(inputs ExitInputs) bool {
	maxHours := inputs.MaxHoldHours
	if maxHours <= 0 {
		maxHours = ee.config.DefaultMaxHoldHours
	}
	
	hoursHeld := inputs.CurrentTime.Sub(inputs.EntryTime).Hours()
	return hoursHeld >= maxHours
}

// evaluateAccelerationReversal checks if momentum acceleration has reversed significantly
func (ee *ExitEvaluator) evaluateAccelerationReversal(inputs ExitInputs) bool {
	if inputs.EntryAccel <= 0 {
		return false // Can't evaluate without entry acceleration
	}
	
	accelChange := ((inputs.MomentumAccel / inputs.EntryAccel) - 1.0) * 100
	return accelChange <= -ee.config.AccelReversalThreshold
}

// evaluateMomentumFade checks if momentum has faded significantly from entry
func (ee *ExitEvaluator) evaluateMomentumFade(inputs ExitInputs) bool {
	if inputs.EntryMomentumScore <= 0 {
		return false // Can't evaluate without entry momentum
	}
	
	momentumChange := ((inputs.MomentumScore / inputs.EntryMomentumScore) - 1.0) * 100
	return momentumChange <= -ee.config.MomentumFadeThreshold
}

// evaluateTrailingStop checks if trailing stop is triggered
func (ee *ExitEvaluator) evaluateTrailingStop(inputs ExitInputs) bool {
	if inputs.HighWaterMark <= inputs.EntryPrice {
		return false // No profits to protect yet
	}
	
	trailingPct := inputs.TrailingStopPct
	if trailingPct <= 0 {
		trailingPct = ee.config.DefaultTrailingPct
	}
	
	stopPrice := inputs.HighWaterMark * (1.0 - trailingPct/100.0)
	return inputs.CurrentPrice <= stopPrice
}

// evaluateProfitTargets checks if profit targets are hit
func (ee *ExitEvaluator) evaluateProfitTargets(inputs ExitInputs) bool {
	// Check target 2 first (higher target)
	if inputs.ProfitTargetPrice2 > 0 && inputs.CurrentPrice >= inputs.ProfitTargetPrice2 {
		return true
	}
	
	// Check target 1
	if inputs.ProfitTargetPrice1 > 0 && inputs.CurrentPrice >= inputs.ProfitTargetPrice1 {
		return true
	}
	
	return false
}

// GetExitSummary returns a concise exit evaluation summary
func (er *ExitResult) GetExitSummary() string {
	if er.ShouldExit {
		return fmt.Sprintf("🚪 EXIT TRIGGERED — %s: %s (%.1f%% PnL after %.1fh)", 
			er.Symbol, er.ExitReason.String(), er.UnrealizedPnL, er.HoursHeld)
	} else {
		return fmt.Sprintf("✅ HOLD POSITION — %s: %.1f%% PnL after %.1fh", 
			er.Symbol, er.UnrealizedPnL, er.HoursHeld)
	}
}

// GetDetailedExitReport returns comprehensive exit analysis
func (er *ExitResult) GetDetailedExitReport() string {
	report := fmt.Sprintf("Exit Evaluation: %s\n", er.Symbol)
	report += fmt.Sprintf("Decision: %s | PnL: %.1f%% | Held: %.1fh\n", 
		map[bool]string{true: "EXIT 🚪", false: "HOLD ✅"}[er.ShouldExit],
		er.UnrealizedPnL, er.HoursHeld)
	report += fmt.Sprintf("Price: %.4f (entry: %.4f)\n\n", er.CurrentPrice, er.EntryPrice)

	if er.ShouldExit {
		report += fmt.Sprintf("Exit Reason: %s\n", er.ExitReason.String())
		report += fmt.Sprintf("Trigger: %s\n", er.TriggeredBy)
	} else {
		report += "Position remains open - no exit conditions met\n"
	}

	report += fmt.Sprintf("\nEvaluation completed in %dms", er.EvaluationTimeMs)
	return report
}