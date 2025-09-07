package gates

import (
	"context"
	"fmt"
	"time"
)

// GuardMetrics evaluates timing and fatigue guards for entry signals
type GuardMetrics struct {
	config *GuardConfig
}

// GuardConfig contains thresholds for signal freshness, fatigue, and timing guards
type GuardConfig struct {
	// Freshness guard: signal must be recent
	MaxBarsAge int `yaml:"max_bars_age"` // ≤2 bars old

	// Fatigue guard: prevent overextended entries
	FatiguePrice24hThreshold float64 `yaml:"fatigue_price_24h_threshold"` // >12% = fatigued
	FatigueRSI4hThreshold    float64 `yaml:"fatigue_rsi_4h_threshold"`    // >70 = overbought

	// Proximity guard: price must be close to trigger
	ProximityATRMultiple float64 `yaml:"proximity_atr_multiple"` // 1.2× ATR maximum

	// Late-fill guard: prevent stale execution
	MaxSecondsSinceTrigger int64 `yaml:"max_seconds_since_trigger"` // <30s
}

// DefaultGuardConfig returns production-ready guard configuration
func DefaultGuardConfig() *GuardConfig {
	return &GuardConfig{
		MaxBarsAge:               2,    // ≤2 bars old
		FatiguePrice24hThreshold: 12.0, // >12% in 24h
		FatigueRSI4hThreshold:    70.0, // RSI >70
		ProximityATRMultiple:     1.2,  // 1.2× ATR
		MaxSecondsSinceTrigger:   30,   // <30 seconds
	}
}

// NewGuardMetrics creates a new guard metrics evaluator
func NewGuardMetrics(config *GuardConfig) *GuardMetrics {
	if config == nil {
		config = DefaultGuardConfig()
	}
	return &GuardMetrics{config: config}
}

// GuardInputs contains all data needed for guard evaluation
type GuardInputs struct {
	Symbol              string    `json:"symbol"`
	BarsSinceSignal     int       `json:"bars_since_signal"`     // How many bars ago was signal
	PriceChange24h      float64   `json:"price_change_24h"`      // 24h price change %
	RSI4h               float64   `json:"rsi_4h"`                // 4-hour RSI
	DistanceFromTrigger float64   `json:"distance_from_trigger"` // Distance from trigger price
	ATR1h               float64   `json:"atr_1h"`                // 1-hour ATR for proximity
	SecondsSinceTrigger int64     `json:"seconds_since_trigger"` // Time since trigger bar close
	HasPullback         bool      `json:"has_pullback"`          // Recent pullback detected
	HasAcceleration     bool      `json:"has_acceleration"`      // Renewed acceleration
	Timestamp           time.Time `json:"timestamp"`
}

// GuardResult contains the evaluation results for all guards
type GuardResult struct {
	Symbol         string                `json:"symbol"`
	Timestamp      time.Time             `json:"timestamp"`
	AllPassed      bool                  `json:"all_passed"`
	GuardChecks    map[string]*GateCheck `json:"guard_checks"`
	FailureReasons []string              `json:"failure_reasons"`
	PassedGuards   []string              `json:"passed_guards"`
}

// EvaluateGuards performs comprehensive guard evaluation
func (gm *GuardMetrics) EvaluateGuards(ctx context.Context, inputs GuardInputs) (*GuardResult, error) {
	result := &GuardResult{
		Symbol:         inputs.Symbol,
		Timestamp:      inputs.Timestamp,
		GuardChecks:    make(map[string]*GateCheck),
		FailureReasons: []string{},
		PassedGuards:   []string{},
	}

	// Guard 1: Freshness - signal must be ≤2 bars old
	freshnessCheck := gm.evaluateFreshnessGuard(inputs)
	result.GuardChecks["freshness"] = freshnessCheck
	if freshnessCheck.Passed {
		result.PassedGuards = append(result.PassedGuards, "freshness")
	} else {
		result.FailureReasons = append(result.FailureReasons,
			fmt.Sprintf("Stale signal: %d bars old (max %d)", inputs.BarsSinceSignal, gm.config.MaxBarsAge))
	}

	// Guard 2: Fatigue - prevent overextended entries
	fatigueCheck := gm.evaluateFatigueGuard(inputs)
	result.GuardChecks["fatigue"] = fatigueCheck
	if fatigueCheck.Passed {
		result.PassedGuards = append(result.PassedGuards, "fatigue")
	} else {
		result.FailureReasons = append(result.FailureReasons,
			fmt.Sprintf("Fatigue detected: 24h %.1f%% + RSI %.1f (no pullback/accel)",
				inputs.PriceChange24h, inputs.RSI4h))
	}

	// Guard 3: Proximity - price must be close to trigger
	proximityCheck := gm.evaluateProximityGuard(inputs)
	result.GuardChecks["proximity"] = proximityCheck
	if proximityCheck.Passed {
		result.PassedGuards = append(result.PassedGuards, "proximity")
	} else {
		maxDist := inputs.ATR1h * gm.config.ProximityATRMultiple
		result.FailureReasons = append(result.FailureReasons,
			fmt.Sprintf("Price too far: %.4f > %.4f (%.1fx ATR)",
				inputs.DistanceFromTrigger, maxDist, gm.config.ProximityATRMultiple))
	}

	// Guard 4: Late-fill - execution must be timely
	lateFillCheck := gm.evaluateLateFillGuard(inputs)
	result.GuardChecks["late_fill"] = lateFillCheck
	if lateFillCheck.Passed {
		result.PassedGuards = append(result.PassedGuards, "late_fill")
	} else {
		result.FailureReasons = append(result.FailureReasons,
			fmt.Sprintf("Late fill: %ds since trigger (max %ds)",
				inputs.SecondsSinceTrigger, gm.config.MaxSecondsSinceTrigger))
	}

	// Overall result
	result.AllPassed = len(result.FailureReasons) == 0

	return result, nil
}

// evaluateFreshnessGuard ensures signal is recent (≤2 bars old)
func (gm *GuardMetrics) evaluateFreshnessGuard(inputs GuardInputs) *GateCheck {
	passed := inputs.BarsSinceSignal <= gm.config.MaxBarsAge

	return &GateCheck{
		Name:        "freshness",
		Passed:      passed,
		Value:       inputs.BarsSinceSignal,
		Threshold:   gm.config.MaxBarsAge,
		Description: fmt.Sprintf("Signal age %d bars ≤ %d bars", inputs.BarsSinceSignal, gm.config.MaxBarsAge),
	}
}

// evaluateFatigueGuard checks for overextension (24h >12% AND RSI4h >70 UNLESS pullback/acceleration)
func (gm *GuardMetrics) evaluateFatigueGuard(inputs GuardInputs) *GateCheck {
	// Check if conditions indicate fatigue
	isOverextended := inputs.PriceChange24h > gm.config.FatiguePrice24hThreshold &&
		inputs.RSI4h > gm.config.FatigueRSI4hThreshold

	// Check for exceptions that override fatigue
	hasException := inputs.HasPullback || inputs.HasAcceleration

	passed := !isOverextended || hasException

	description := fmt.Sprintf("24h %.1f%% (thresh %.1f%%), RSI %.1f (thresh %.1f)",
		inputs.PriceChange24h, gm.config.FatiguePrice24hThreshold,
		inputs.RSI4h, gm.config.FatigueRSI4hThreshold)

	if isOverextended && hasException {
		description += " - EXCEPTION: pullback/acceleration detected"
	}

	return &GateCheck{
		Name:        "fatigue",
		Passed:      passed,
		Value:       inputs.PriceChange24h,
		Threshold:   gm.config.FatiguePrice24hThreshold,
		Description: description,
	}
}

// evaluateProximityGuard ensures price is close to trigger (≤1.2× ATR)
func (gm *GuardMetrics) evaluateProximityGuard(inputs GuardInputs) *GateCheck {
	maxDistance := inputs.ATR1h * gm.config.ProximityATRMultiple
	passed := inputs.DistanceFromTrigger <= maxDistance

	return &GateCheck{
		Name:      "proximity",
		Passed:    passed,
		Value:     inputs.DistanceFromTrigger,
		Threshold: maxDistance,
		Description: fmt.Sprintf("Distance %.4f ≤ %.4f (%.1fx ATR)",
			inputs.DistanceFromTrigger, maxDistance, gm.config.ProximityATRMultiple),
	}
}

// evaluateLateFillGuard prevents late fills (<30s since trigger)
func (gm *GuardMetrics) evaluateLateFillGuard(inputs GuardInputs) *GateCheck {
	passed := inputs.SecondsSinceTrigger < gm.config.MaxSecondsSinceTrigger

	return &GateCheck{
		Name:        "late_fill",
		Passed:      passed,
		Value:       inputs.SecondsSinceTrigger,
		Threshold:   gm.config.MaxSecondsSinceTrigger,
		Description: fmt.Sprintf("Fill timing %ds < %ds", inputs.SecondsSinceTrigger, gm.config.MaxSecondsSinceTrigger),
	}
}

// GetGuardSummary returns a concise guard evaluation summary
func (gr *GuardResult) GetGuardSummary() string {
	if gr.AllPassed {
		return fmt.Sprintf("✅ GUARDS CLEARED — %s (%d/%d passed)",
			gr.Symbol, len(gr.PassedGuards), len(gr.GuardChecks))
	} else {
		return fmt.Sprintf("❌ GUARD BLOCKED — %s (%d failures)",
			gr.Symbol, len(gr.FailureReasons))
	}
}

// GetDetailedGuardReport returns comprehensive guard analysis
func (gr *GuardResult) GetDetailedGuardReport() string {
	report := fmt.Sprintf("Guard Evaluation: %s\n", gr.Symbol)
	report += fmt.Sprintf("Overall: %s\n\n",
		map[bool]string{true: "PASS ✅", false: "FAIL ❌"}[gr.AllPassed])

	// Show all guard results
	guardOrder := []string{"freshness", "fatigue", "proximity", "late_fill"}

	for _, guardName := range guardOrder {
		if check, exists := gr.GuardChecks[guardName]; exists {
			status := map[bool]string{true: "✅", false: "❌"}[check.Passed]
			report += fmt.Sprintf("%s %s: %s\n", status, check.Name, check.Description)
		}
	}

	if len(gr.FailureReasons) > 0 {
		report += fmt.Sprintf("\nFailure Details:\n")
		for i, reason := range gr.FailureReasons {
			report += fmt.Sprintf("  %d. %s\n", i+1, reason)
		}
	}

	return report
}
