package exits

import (
	"context"
	"fmt"
	"time"
)

// ExitReason represents the reason for exit with precedence
type ExitReason int

const (
	NoExit               ExitReason = iota
	HardStop                        // Highest precedence: hard stop loss hit
	VenueHealthCut                  // Venue degradation (P99 latency, error rates)
	TimeLimit                       // 48-hour time limit reached
	AccelerationReversal            // Momentum acceleration has reversed
	MomentumFade                    // Momentum factor has faded significantly
	TrailingStop                    // Trailing stop activated
	ProfitTarget                    // Profit target achieved (lowest precedence)
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
	TriggeredBy      string     `json:"triggered_by"` // Specific trigger description
	CurrentPrice     float64    `json:"current_price"`
	EntryPrice       float64    `json:"entry_price"`
	UnrealizedPnL    float64    `json:"unrealized_pnl"` // % return
	HoursHeld        float64    `json:"hours_held"`
	EvaluationTimeMs int64      `json:"evaluation_time_ms"`
}

// ExitInputs contains all data required for exit evaluation
type ExitInputs struct {
	Symbol       string    `json:"symbol"`
	EntryPrice   float64   `json:"entry_price"`
	CurrentPrice float64   `json:"current_price"`
	EntryTime    time.Time `json:"entry_time"`
	CurrentTime  time.Time `json:"current_time"`

	// ATR for stop calculations
	ATR1h float64 `json:"atr_1h"` // 1-hour ATR for stop calculations

	// Hard stop (calculated as entryPrice - ATR*multiplier)
	HardStopATRMultiplier float64 `json:"hard_stop_atr_multiplier"` // Default 1.5

	// Venue health metrics
	VenueP99LatencyMs   int64   `json:"venue_p99_latency_ms"`  // P99 latency in ms
	VenueErrorRate      float64 `json:"venue_error_rate"`      // Error rate %
	VenueRejectRate     float64 `json:"venue_reject_rate"`     // Order reject rate %
	VenueHealthDegraded bool    `json:"venue_health_degraded"` // Whether venue is degraded

	// Time-based exit
	MaxHoldHours float64 `json:"max_hold_hours"` // Maximum hold time (default 48h)

	// Momentum metrics (1h & 4h required for fade detection)
	Momentum1h      float64 `json:"momentum_1h"`       // Current 1h momentum
	Momentum4h      float64 `json:"momentum_4h"`       // Current 4h momentum
	MomentumAccel4h float64 `json:"momentum_accel_4h"` // 4h acceleration (d²)
	IsAccelerating  bool    `json:"is_accelerating"`   // Whether still accelerating

	// Trailing stop (ATR-based after 12h)
	HighWaterMark         float64 `json:"high_water_mark"`         // Highest price since entry
	TrailingATRMultiplier float64 `json:"trailing_atr_multiplier"` // Default 1.8

	// Profit targets (+8% / +15% / +25%)
	ProfitTarget1      float64 `json:"profit_target_1"`       // First target %
	ProfitTarget2      float64 `json:"profit_target_2"`       // Second target %
	ProfitTarget3      float64 `json:"profit_target_3"`       // Third target %
	ProfitTargetPrice1 float64 `json:"profit_target_price_1"` // Calculated price 1
	ProfitTargetPrice2 float64 `json:"profit_target_price_2"` // Calculated price 2
	ProfitTargetPrice3 float64 `json:"profit_target_price_3"` // Calculated price 3
}

// ExitEvaluator evaluates exit conditions with proper precedence
type ExitEvaluator struct {
	config *ExitConfig
}

// ExitConfig contains exit rule configuration
type ExitConfig struct {
	// Hard stop configuration (-1.5× ATR)
	EnableHardStop bool    `yaml:"enable_hard_stop"`
	HardStopATRx   float64 `yaml:"hard_stop_atr_multiplier"` // -1.5 default

	// Venue health thresholds (tighten +0.3× ATR if venue degrades)
	MaxVenueP99LatencyMs    int64   `yaml:"max_venue_p99_latency_ms"`   // 2000ms default
	MaxVenueErrorRate       float64 `yaml:"max_venue_error_rate"`       // 3% default
	MaxVenueRejectRate      float64 `yaml:"max_venue_reject_rate"`      // 5% default
	VenueHealthATRTightener float64 `yaml:"venue_health_atr_tightener"` // +0.3 ATR tightener

	// Time limit
	DefaultMaxHoldHours float64 `yaml:"default_max_hold_hours"` // 48 hours default

	// Momentum fade thresholds (1h & 4h negative)
	MomentumFadeThreshold  float64 `yaml:"momentum_fade_threshold"`  // Both 1h & 4h must be negative
	AccelReversalThreshold float64 `yaml:"accel_reversal_threshold"` // 4h d²<0

	// Trailing stop configuration (ATR×1.8 unless accelerating)
	EnableTrailingStop    bool    `yaml:"enable_trailing_stop"`
	TrailingATRMultiplier float64 `yaml:"trailing_atr_multiplier"` // 1.8 default
	MinHoursForTrailing   float64 `yaml:"min_hours_for_trailing"`  // 12 hours default

	// Profit targets (+8% / +15% / +25%)
	EnableProfitTargets  bool    `yaml:"enable_profit_targets"`
	DefaultProfitTarget1 float64 `yaml:"default_profit_target_1"` // 8% default
	DefaultProfitTarget2 float64 `yaml:"default_profit_target_2"` // 15% default
	DefaultProfitTarget3 float64 `yaml:"default_profit_target_3"` // 25% default
}

// DefaultExitConfig returns production-ready exit configuration
func DefaultExitConfig() *ExitConfig {
	return &ExitConfig{
		// Hard stop: -1.5× ATR
		EnableHardStop: true,
		HardStopATRx:   1.5,

		// Venue health: tighten +0.3× ATR if venue degrades
		MaxVenueP99LatencyMs:    2000, // 2 seconds
		MaxVenueErrorRate:       3.0,  // 3%
		MaxVenueRejectRate:      5.0,  // 5%
		VenueHealthATRTightener: 0.3,  // +0.3 ATR tightener

		// Time limit: 48h max
		DefaultMaxHoldHours: 48.0,

		// Momentum: 1h & 4h negative for fade, 4h d²<0 for accel reversal
		MomentumFadeThreshold:  0.0, // Both 1h & 4h must be negative
		AccelReversalThreshold: 0.0, // 4h d²<0

		// Trailing: ATR×1.8 after 12h unless accelerating
		EnableTrailingStop:    true,
		TrailingATRMultiplier: 1.8,
		MinHoursForTrailing:   12.0,

		// Profit targets: +8% / +15% / +25%
		EnableProfitTargets:  true,
		DefaultProfitTarget1: 8.0,  // 8%
		DefaultProfitTarget2: 15.0, // 15%
		DefaultProfitTarget3: 25.0, // 25%
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

	// Evaluate exits in precedence order (first trigger wins)

	// 1. Hard Stop: -1.5× ATR (highest precedence)
	if ee.config.EnableHardStop && ee.evaluateHardStop(inputs) {
		result.ShouldExit = true
		result.ExitReason = HardStop
		result.ReasonString = "hard_stop"
		hardStopPrice := inputs.EntryPrice - (inputs.ATR1h * ee.config.HardStopATRx)
		result.TriggeredBy = fmt.Sprintf("Hard stop: %.4f ≤ %.4f (-%.1f×ATR)",
			inputs.CurrentPrice, hardStopPrice, ee.config.HardStopATRx)
	}

	// 2. Venue Health Cut: tighten +0.3× ATR if venue degrades
	if !result.ShouldExit && ee.evaluateVenueHealth(inputs) {
		result.ShouldExit = true
		result.ExitReason = VenueHealthCut
		result.ReasonString = "venue_health_cut"
		tightenerPrice := inputs.EntryPrice - (inputs.ATR1h * ee.config.VenueHealthATRTightener)
		result.TriggeredBy = fmt.Sprintf("Venue degraded, tightened stop: %.4f ≤ %.4f (+%.1f×ATR tightener)",
			inputs.CurrentPrice, tightenerPrice, ee.config.VenueHealthATRTightener)
	}

	// 3. Time Limit: 48h max
	if !result.ShouldExit && ee.evaluateTimeLimit(inputs) {
		result.ShouldExit = true
		result.ExitReason = TimeLimit
		result.ReasonString = "time_limit"
		maxHours := inputs.MaxHoldHours
		if maxHours <= 0 {
			maxHours = ee.config.DefaultMaxHoldHours
		}
		result.TriggeredBy = fmt.Sprintf("Time limit: %.1f hours ≥ %.1f hour max",
			result.HoursHeld, maxHours)
	}

	// 4. Acceleration Reversal: 4h d²<0
	if !result.ShouldExit && ee.evaluateAccelerationReversal(inputs) {
		result.ShouldExit = true
		result.ExitReason = AccelerationReversal
		result.ReasonString = "acceleration_reversal"
		result.TriggeredBy = fmt.Sprintf("Acceleration reversal: 4h d² = %.3f < 0",
			inputs.MomentumAccel4h)
	}

	// 5. Momentum Fade: 1h & 4h negative
	if !result.ShouldExit && ee.evaluateMomentumFade(inputs) {
		result.ShouldExit = true
		result.ExitReason = MomentumFade
		result.ReasonString = "momentum_fade"
		result.TriggeredBy = fmt.Sprintf("Momentum fade: 1h=%.2f<0 & 4h=%.2f<0",
			inputs.Momentum1h, inputs.Momentum4h)
	}

	// 6. Trailing Stop: ATR×1.8 after 12h unless accelerating
	if !result.ShouldExit && ee.config.EnableTrailingStop && ee.evaluateTrailingStop(inputs) {
		result.ShouldExit = true
		result.ExitReason = TrailingStop
		result.ReasonString = "trailing_stop"
		stopPrice := inputs.HighWaterMark - (inputs.ATR1h * inputs.TrailingATRMultiplier)
		result.TriggeredBy = fmt.Sprintf("Trailing stop: %.4f ≤ %.4f (HWM %.4f - %.1f×ATR)",
			inputs.CurrentPrice, stopPrice, inputs.HighWaterMark, inputs.TrailingATRMultiplier)
	}

	// 7. Profit Targets: +8% / +15% / +25% (lowest precedence)
	if !result.ShouldExit && ee.config.EnableProfitTargets && ee.evaluateProfitTargets(inputs) {
		result.ShouldExit = true
		result.ExitReason = ProfitTarget
		result.ReasonString = "profit_target"

		if inputs.ProfitTargetPrice3 > 0 && inputs.CurrentPrice >= inputs.ProfitTargetPrice3 {
			result.TriggeredBy = fmt.Sprintf("Profit target 3: %.4f ≥ %.4f (+%.1f%%)",
				inputs.CurrentPrice, inputs.ProfitTargetPrice3, inputs.ProfitTarget3)
		} else if inputs.ProfitTargetPrice2 > 0 && inputs.CurrentPrice >= inputs.ProfitTargetPrice2 {
			result.TriggeredBy = fmt.Sprintf("Profit target 2: %.4f ≥ %.4f (+%.1f%%)",
				inputs.CurrentPrice, inputs.ProfitTargetPrice2, inputs.ProfitTarget2)
		} else if inputs.ProfitTargetPrice1 > 0 && inputs.CurrentPrice >= inputs.ProfitTargetPrice1 {
			result.TriggeredBy = fmt.Sprintf("Profit target 1: %.4f ≥ %.4f (+%.1f%%)",
				inputs.CurrentPrice, inputs.ProfitTargetPrice1, inputs.ProfitTarget1)
		}
	}

	result.EvaluationTimeMs = time.Since(startTime).Milliseconds()
	return result, nil
}

// evaluateHardStop checks if hard stop loss is triggered (-1.5× ATR)
func (ee *ExitEvaluator) evaluateHardStop(inputs ExitInputs) bool {
	if inputs.ATR1h <= 0 {
		return false // Can't calculate ATR-based stop without ATR
	}

	hardStopATRx := inputs.HardStopATRMultiplier
	if hardStopATRx <= 0 {
		hardStopATRx = ee.config.HardStopATRx
	}

	hardStopPrice := inputs.EntryPrice - (inputs.ATR1h * hardStopATRx)
	return inputs.CurrentPrice <= hardStopPrice
}

// evaluateVenueHealth checks if venue performance is degraded (tighten +0.3× ATR)
func (ee *ExitEvaluator) evaluateVenueHealth(inputs ExitInputs) bool {
	// Check if venue is degraded by any metric
	isDegraded := inputs.VenueP99LatencyMs > ee.config.MaxVenueP99LatencyMs ||
		inputs.VenueErrorRate > ee.config.MaxVenueErrorRate ||
		inputs.VenueRejectRate > ee.config.MaxVenueRejectRate ||
		inputs.VenueHealthDegraded

	if !isDegraded || inputs.ATR1h <= 0 {
		return false
	}

	// Calculate tightened stop price
	tightenedStopPrice := inputs.EntryPrice - (inputs.ATR1h * ee.config.VenueHealthATRTightener)
	return inputs.CurrentPrice <= tightenedStopPrice
}

// evaluateTimeLimit checks if maximum hold time is reached (48h)
func (ee *ExitEvaluator) evaluateTimeLimit(inputs ExitInputs) bool {
	maxHours := inputs.MaxHoldHours
	if maxHours <= 0 {
		maxHours = ee.config.DefaultMaxHoldHours
	}

	hoursHeld := inputs.CurrentTime.Sub(inputs.EntryTime).Hours()
	return hoursHeld >= maxHours
}

// evaluateAccelerationReversal checks if 4h acceleration has turned negative (d²<0)
func (ee *ExitEvaluator) evaluateAccelerationReversal(inputs ExitInputs) bool {
	return inputs.MomentumAccel4h < 0
}

// evaluateMomentumFade checks if both 1h & 4h momentum are negative
func (ee *ExitEvaluator) evaluateMomentumFade(inputs ExitInputs) bool {
	return inputs.Momentum1h < 0 && inputs.Momentum4h < 0
}

// evaluateTrailingStop checks if trailing stop is triggered (ATR×1.8 after 12h unless accelerating)
func (ee *ExitEvaluator) evaluateTrailingStop(inputs ExitInputs) bool {
	if inputs.HighWaterMark <= inputs.EntryPrice {
		return false // No profits to protect yet
	}

	// Only apply trailing stop after minimum hours
	hoursHeld := inputs.CurrentTime.Sub(inputs.EntryTime).Hours()
	if hoursHeld < ee.config.MinHoursForTrailing {
		return false
	}

	// Don't apply trailing stop if still accelerating
	if inputs.IsAccelerating {
		return false
	}

	if inputs.ATR1h <= 0 {
		return false // Need ATR for calculation
	}

	trailingMultiplier := inputs.TrailingATRMultiplier
	if trailingMultiplier <= 0 {
		trailingMultiplier = ee.config.TrailingATRMultiplier
	}

	stopPrice := inputs.HighWaterMark - (inputs.ATR1h * trailingMultiplier)
	return inputs.CurrentPrice <= stopPrice
}

// evaluateProfitTargets checks if profit targets are hit (+8% / +15% / +25%)
func (ee *ExitEvaluator) evaluateProfitTargets(inputs ExitInputs) bool {
	// Check targets in ascending order (highest first)
	if inputs.ProfitTargetPrice3 > 0 && inputs.CurrentPrice >= inputs.ProfitTargetPrice3 {
		return true
	}

	if inputs.ProfitTargetPrice2 > 0 && inputs.CurrentPrice >= inputs.ProfitTargetPrice2 {
		return true
	}

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
