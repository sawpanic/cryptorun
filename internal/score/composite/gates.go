package composite

import (
	"fmt"
	"math"
	"time"
)

// HardEntryGates implements the unified hard entry gates for composite scoring
type HardEntryGates struct {
	// Thresholds
	minScore float64 // 75 - minimum composite score (0-100, pre-social)
	minVADR  float64 // 1.8 - minimum VADR threshold

	// Keep existing gates
	maxFreshnessAge int     // ≤2 bars
	maxATRDistance  float64 // ≤1.2×ATR
	maxLateFillMs   float64 // <30s (with p99 relaxation)
	maxSpreadBps    float64 // <50bps
	minDepthUSD     float64 // ≥$100k @ ±2%
	minMicroVADR    float64 // ≥1.75× (microstructure)
}

// NewHardEntryGates creates hard entry gates with unified composite thresholds
func NewHardEntryGates() *HardEntryGates {
	return &HardEntryGates{
		// NEW unified composite thresholds
		minScore: 75.0, // Score≥75 (0-100 scale, before social)
		minVADR:  1.8,  // VADR≥1.8× (increased from 1.75×)

		// EXISTING gate thresholds (unchanged)
		maxFreshnessAge: 2,      // ≤2 bars
		maxATRDistance:  1.2,    // ≤1.2×ATR
		maxLateFillMs:   30000,  // <30s (30000ms)
		maxSpreadBps:    50.0,   // <50bps
		minDepthUSD:     100000, // ≥$100k
		minMicroVADR:    1.75,   // ≥1.75× (microstructure validation)
	}
}

// GateInput contains all data needed for entry gate evaluation
type GateInput struct {
	Symbol    string
	Timestamp time.Time

	// NEW: Composite scoring requirements
	CompositeScore float64 // 0-100 internal score (before social)
	VADR           float64 // Volume-adjusted daily range
	FundingZScore  float64 // Venue-median funding z-score

	// EXISTING: Freshness/timing gates
	BarAge        int     // Age in bars
	ATRDistance   float64 // Distance from trigger (×ATR)
	ATRCurrent    float64 // Current ATR value
	SignalTime    time.Time
	ExecutionTime time.Time

	// EXISTING: Microstructure gates
	SpreadBps float64 // Bid-ask spread (basis points)
	DepthUSD  float64 // Order book depth (USD @ ±2%)
	MicroVADR float64 // Microstructure VADR

	// EXISTING: Fatigue gates
	Momentum24h float64 // 24h momentum (%)
	RSI4h       float64 // 4h RSI
}

// GateResult holds the complete gate evaluation outcome
type GateResult struct {
	Allowed     bool              `json:"allowed"`
	Reason      string            `json:"reason"`
	GatesPassed map[string]bool   `json:"gates_passed"`
	GateReasons map[string]string `json:"gate_reasons"`
	Timestamp   time.Time         `json:"timestamp"`

	// Hard gate categories
	CompositePass bool `json:"composite_pass"`
	FreshnessPass bool `json:"freshness_pass"`
	MicroPass     bool `json:"micro_pass"`
	FatiguePass   bool `json:"fatigue_pass"`
	PolicyPass    bool `json:"policy_pass"`
}

// EvaluateAll runs all hard entry gates in sequence
func (g *HardEntryGates) EvaluateAll(input GateInput) GateResult {
	result := GateResult{
		GatesPassed: make(map[string]bool),
		GateReasons: make(map[string]string),
		Timestamp:   time.Now(),
	}

	// NEW: Composite scoring gates (checked FIRST, before social)
	g.evaluateCompositeGates(input, &result)

	// EXISTING: Freshness gates
	g.evaluateFreshnessGates(input, &result)

	// EXISTING: Microstructure gates
	g.evaluateMicrostructureGates(input, &result)

	// EXISTING: Fatigue gates
	g.evaluateFatigueGates(input, &result)

	// EXISTING: Policy gates (late-fill, etc.)
	g.evaluatePolicyGates(input, &result)

	// Overall pass/fail determination
	result.Allowed = result.CompositePass && result.FreshnessPass &&
		result.MicroPass && result.FatiguePass && result.PolicyPass

	if !result.Allowed {
		result.Reason = g.buildFailureReason(&result)
	} else {
		result.Reason = "All entry gates passed"
	}

	return result
}

// evaluateCompositeGates checks NEW unified composite requirements
func (g *HardEntryGates) evaluateCompositeGates(input GateInput, result *GateResult) {
	// Gate 1: Composite Score ≥75
	scorePass := input.CompositeScore >= g.minScore
	result.GatesPassed["composite_score"] = scorePass
	if !scorePass {
		result.GateReasons["composite_score"] = fmt.Sprintf("score %.1f < %.1f minimum",
			input.CompositeScore, g.minScore)
	}

	// Gate 2: VADR ≥1.8×
	vadrPass := input.VADR >= g.minVADR
	result.GatesPassed["vadr"] = vadrPass
	if !vadrPass {
		result.GateReasons["vadr"] = fmt.Sprintf("VADR %.2f× < %.1f× minimum",
			input.VADR, g.minVADR)
	}

	// Gate 3: Funding divergence present (venue-median funding z ≤ 0 with price holding)
	fundingPass := input.FundingZScore <= 0.0
	result.GatesPassed["funding_divergence"] = fundingPass
	if !fundingPass {
		result.GateReasons["funding_divergence"] = fmt.Sprintf("funding z-score %.2f > 0.0 (no divergence)",
			input.FundingZScore)
	}

	// Composite gates overall pass
	result.CompositePass = scorePass && vadrPass && fundingPass
}

// evaluateFreshnessGates checks data freshness requirements
func (g *HardEntryGates) evaluateFreshnessGates(input GateInput, result *GateResult) {
	// Bar age ≤2 bars
	agePass := input.BarAge <= g.maxFreshnessAge
	result.GatesPassed["bar_age"] = agePass
	if !agePass {
		result.GateReasons["bar_age"] = fmt.Sprintf("bar age %d > %d bars maximum",
			input.BarAge, g.maxFreshnessAge)
	}

	// ATR distance ≤1.2×
	atrPass := true
	if input.ATRCurrent > 0 {
		atrPass = input.ATRDistance <= g.maxATRDistance
		result.GatesPassed["atr_distance"] = atrPass
		if !atrPass {
			result.GateReasons["atr_distance"] = fmt.Sprintf("ATR distance %.2f× > %.1f× maximum",
				input.ATRDistance, g.maxATRDistance)
		}
	}

	result.FreshnessPass = agePass && atrPass
}

// evaluateMicrostructureGates checks microstructure requirements
func (g *HardEntryGates) evaluateMicrostructureGates(input GateInput, result *GateResult) {
	// Spread <50bps
	spreadPass := input.SpreadBps < g.maxSpreadBps
	result.GatesPassed["spread"] = spreadPass
	if !spreadPass {
		result.GateReasons["spread"] = fmt.Sprintf("spread %.1f bps ≥ %.1f bps maximum",
			input.SpreadBps, g.maxSpreadBps)
	}

	// Depth ≥$100k @ ±2%
	depthPass := input.DepthUSD >= g.minDepthUSD
	result.GatesPassed["depth"] = depthPass
	if !depthPass {
		result.GateReasons["depth"] = fmt.Sprintf("depth $%.0f < $%.0f minimum",
			input.DepthUSD, g.minDepthUSD)
	}

	// Microstructure VADR ≥1.75×
	microVadrPass := input.MicroVADR >= g.minMicroVADR
	result.GatesPassed["micro_vadr"] = microVadrPass
	if !microVadrPass {
		result.GateReasons["micro_vadr"] = fmt.Sprintf("micro VADR %.2f× < %.2f× minimum",
			input.MicroVADR, g.minMicroVADR)
	}

	result.MicroPass = spreadPass && depthPass && microVadrPass
}

// evaluateFatigueGates checks overextension protection
func (g *HardEntryGates) evaluateFatigueGates(input GateInput, result *GateResult) {
	// Fatigue protection: 24h >12% AND RSI4h >70
	momentumHigh := math.Abs(input.Momentum24h) > 12.0
	rsiHigh := input.RSI4h > 70.0

	fatigueTriggered := momentumHigh && rsiHigh
	result.GatesPassed["fatigue"] = !fatigueTriggered

	if fatigueTriggered {
		result.GateReasons["fatigue"] = fmt.Sprintf("fatigue: 24h momentum %.1f%% > 12%% AND RSI4h %.1f > 70",
			input.Momentum24h, input.RSI4h)
	}

	result.FatiguePass = !fatigueTriggered
}

// evaluatePolicyGates checks timing and administrative policies
func (g *HardEntryGates) evaluatePolicyGates(input GateInput, result *GateResult) {
	// Late-fill protection: <30s delay (with p99 relaxation handled elsewhere)
	delay := input.ExecutionTime.Sub(input.SignalTime)
	delayMs := float64(delay.Nanoseconds()) / 1e6

	lateFillPass := delayMs <= g.maxLateFillMs
	result.GatesPassed["late_fill"] = lateFillPass
	if !lateFillPass {
		result.GateReasons["late_fill"] = fmt.Sprintf("late fill: %.0fms > %.0fms maximum",
			delayMs, g.maxLateFillMs)
	}

	// Note: Budget gates would be checked here if implemented

	result.PolicyPass = lateFillPass
}

// buildFailureReason constructs a comprehensive failure explanation
func (g *HardEntryGates) buildFailureReason(result *GateResult) string {
	var failures []string

	// Collect all failure reasons
	for gate, passed := range result.GatesPassed {
		if !passed {
			if reason, exists := result.GateReasons[gate]; exists {
				failures = append(failures, reason)
			}
		}
	}

	if len(failures) == 0 {
		return "Gate evaluation failed (no specific reason recorded)"
	}

	if len(failures) == 1 {
		return failures[0]
	}

	// Multiple failures
	primaryFailure := failures[0]
	additionalCount := len(failures) - 1
	return fmt.Sprintf("%s (+ %d other failures)", primaryFailure, additionalCount)
}

// GetThresholds returns current gate threshold configuration
func (g *HardEntryGates) GetThresholds() map[string]interface{} {
	return map[string]interface{}{
		// NEW composite thresholds
		"min_composite_score": g.minScore,
		"min_vadr":            g.minVADR,

		// EXISTING thresholds
		"max_freshness_age": g.maxFreshnessAge,
		"max_atr_distance":  g.maxATRDistance,
		"max_late_fill_ms":  g.maxLateFillMs,
		"max_spread_bps":    g.maxSpreadBps,
		"min_depth_usd":     g.minDepthUSD,
		"min_micro_vadr":    g.minMicroVADR,
	}
}
