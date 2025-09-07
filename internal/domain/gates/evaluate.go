package gates

import (
	"context"
	"fmt"
	"time"

	"github.com/sawpanic/cryptorun/internal/domain"
)

// GateReason represents the result and reasoning for a single gate evaluation
type GateReason struct {
	Name    string             `json:"name"`
	Passed  bool               `json:"passed"`
	Message string             `json:"message"`
	Metrics map[string]float64 `json:"metrics"`
}

// EvaluateAllGatesInputs contains all inputs needed for comprehensive gate evaluation
type EvaluateAllGatesInputs struct {
	Symbol           string
	Timestamp        time.Time
	
	// Freshness gate inputs
	BarsAge          int
	PriceChange      float64
	ATR1h            float64
	
	// Fatigue gate inputs
	Momentum24h      float64
	RSI4h            float64
	Acceleration     float64
	
	// Late-fill gate inputs
	SignalTime       time.Time
	ExecutionTime    time.Time
	
	// Optional microstructure gate inputs (if available)
	Spread           *float64 // nil if not available
	Depth            *float64 // nil if not available
	VADR             *float64 // nil if not available
}

// EvaluateAllGatesResult contains the overall result and individual gate reasons
type EvaluateAllGatesResult struct {
	Passed        bool         `json:"passed"`
	OverallReason string       `json:"overall_reason"`
	Reasons       []GateReason `json:"reasons"`
	Timestamp     time.Time    `json:"timestamp"`
	Symbol        string       `json:"symbol"`
}

// EvaluateAllGates runs all gates in sequence and returns comprehensive results
// Gates are evaluated in order: Freshness -> Fatigue -> Late-Fill -> Microstructure
// Short-circuits on hard failures but still collects all reasons for transparency
func EvaluateAllGates(ctx context.Context, inputs EvaluateAllGatesInputs) (*EvaluateAllGatesResult, error) {
	result := &EvaluateAllGatesResult{
		Passed:    true,
		Reasons:   make([]GateReason, 0),
		Timestamp: time.Now(),
		Symbol:    inputs.Symbol,
	}

	// 1. Freshness Gate
	freshnessInputs := domain.FreshnessGateInputs{
		Symbol:      inputs.Symbol,
		BarsAge:     inputs.BarsAge,
		PriceChange: inputs.PriceChange,
		ATR1h:       inputs.ATR1h,
	}
	
	freshnessEvidence := domain.EvaluateFreshnessGate(freshnessInputs)
	freshnessReason := GateReason{
		Name:    "freshness",
		Passed:  freshnessEvidence.Allow,
		Message: freshnessEvidence.Reason,
		Metrics: map[string]float64{
			"bars_age":     float64(inputs.BarsAge),
			"price_change": inputs.PriceChange,
			"atr_1h":       inputs.ATR1h,
			"atr_ratio":    inputs.PriceChange / inputs.ATR1h,
		},
	}
	result.Reasons = append(result.Reasons, freshnessReason)
	
	if !freshnessEvidence.Allow {
		result.Passed = false
		result.OverallReason = fmt.Sprintf("blocked_by_freshness: %s", freshnessEvidence.Reason)
		// Continue collecting reasons for transparency
	}

	// 2. Fatigue Gate
	fatigueInputs := domain.FatigueGateInputs{
		Symbol:       inputs.Symbol,
		Momentum24h:  inputs.Momentum24h,
		RSI4h:        inputs.RSI4h,
		Acceleration: inputs.Acceleration,
	}
	
	fatigueEvidence := domain.EvaluateFatigueGate(fatigueInputs)
	fatigueReason := GateReason{
		Name:    "fatigue",
		Passed:  fatigueEvidence.Allow,
		Message: fatigueEvidence.Reason,
		Metrics: map[string]float64{
			"momentum_24h": inputs.Momentum24h,
			"rsi_4h":       inputs.RSI4h,
			"acceleration": inputs.Acceleration,
		},
	}
	result.Reasons = append(result.Reasons, fatigueReason)
	
	if !fatigueEvidence.Allow {
		result.Passed = false
		if result.OverallReason == "" {
			result.OverallReason = fmt.Sprintf("blocked_by_fatigue: %s", fatigueEvidence.Reason)
		}
	}

	// 3. Late-Fill Gate
	lateFillInputs := domain.LateFillGateInputs{
		Symbol:        inputs.Symbol,
		SignalTime:    inputs.SignalTime,
		ExecutionTime: inputs.ExecutionTime,
	}
	
	lateFillEvidence := domain.EvaluateLateFillGate(lateFillInputs)
	executionDelay := inputs.ExecutionTime.Sub(inputs.SignalTime).Seconds()
	lateFillReason := GateReason{
		Name:    "late_fill",
		Passed:  lateFillEvidence.Allow,
		Message: lateFillEvidence.Reason,
		Metrics: map[string]float64{
			"execution_delay_seconds": executionDelay,
		},
	}
	result.Reasons = append(result.Reasons, lateFillReason)
	
	if !lateFillEvidence.Allow {
		result.Passed = false
		if result.OverallReason == "" {
			result.OverallReason = fmt.Sprintf("blocked_by_late_fill: %s", lateFillEvidence.Reason)
		}
	}

	// 4. Optional Microstructure Gate (if data available)
	if inputs.Spread != nil && inputs.Depth != nil && inputs.VADR != nil {
		microReason := GateReason{
			Name:    "microstructure",
			Metrics: map[string]float64{
				"spread_bps": *inputs.Spread,
				"depth_usd":  *inputs.Depth,
				"vadr":       *inputs.VADR,
			},
		}
		
		// Apply microstructure checks
		spreadPass := *inputs.Spread <= 50.0
		depthPass := *inputs.Depth >= 100000.0
		vadrPass := *inputs.VADR >= 1.75
		
		microReason.Passed = spreadPass && depthPass && vadrPass
		
		if !microReason.Passed {
			var failureReasons []string
			if !spreadPass {
				failureReasons = append(failureReasons, fmt.Sprintf("spread_too_wide_%.1fbps", *inputs.Spread))
			}
			if !depthPass {
				failureReasons = append(failureReasons, fmt.Sprintf("insufficient_depth_$%.0f", *inputs.Depth))
			}
			if !vadrPass {
				failureReasons = append(failureReasons, fmt.Sprintf("low_vadr_%.2fx", *inputs.VADR))
			}
			microReason.Message = fmt.Sprintf("microstructure_failure: %v", failureReasons)
			
			result.Passed = false
			if result.OverallReason == "" {
				result.OverallReason = fmt.Sprintf("blocked_by_microstructure: %s", microReason.Message)
			}
		} else {
			microReason.Message = "microstructure_pass"
		}
		
		result.Reasons = append(result.Reasons, microReason)
	}

	// Set overall success reason if all passed
	if result.Passed {
		result.OverallReason = "all_gates_passed"
	}

	return result, nil
}

// FormatGateExplanation creates a human-readable explanation of gate results
func FormatGateExplanation(result *EvaluateAllGatesResult) string {
	var explanation string
	
	if result.Passed {
		explanation = fmt.Sprintf("✅ %s: ALL GATES PASSED\n", result.Symbol)
	} else {
		explanation = fmt.Sprintf("❌ %s: ENTRY BLOCKED\n", result.Symbol)
		explanation += fmt.Sprintf("   Overall: %s\n", result.OverallReason)
	}
	
	explanation += "\n📊 Gate Details:\n"
	for _, reason := range result.Reasons {
		status := "❌"
		if reason.Passed {
			status = "✅"
		}
		
		explanation += fmt.Sprintf("   %s %s: %s\n", status, reason.Name, reason.Message)
		
		// Add key metrics
		switch reason.Name {
		case "freshness":
			if atrRatio, ok := reason.Metrics["atr_ratio"]; ok {
				explanation += fmt.Sprintf("      • Price move: %.2fx ATR (limit: 1.2x)\n", atrRatio)
			}
			if barsAge, ok := reason.Metrics["bars_age"]; ok {
				explanation += fmt.Sprintf("      • Bars age: %.0f (limit: 2)\n", barsAge)
			}
		case "fatigue":
			if momentum, ok := reason.Metrics["momentum_24h"]; ok {
				explanation += fmt.Sprintf("      • 24h momentum: %.1f%% (fatigue threshold: >12%%)\n", momentum)
			}
			if rsi, ok := reason.Metrics["rsi_4h"]; ok {
				explanation += fmt.Sprintf("      • RSI 4h: %.1f (overbought threshold: >70)\n", rsi)
			}
			if accel, ok := reason.Metrics["acceleration"]; ok {
				explanation += fmt.Sprintf("      • Acceleration: %.1f%% (override threshold: ≥2%%)\n", accel)
			}
		case "late_fill":
			if delay, ok := reason.Metrics["execution_delay_seconds"]; ok {
				explanation += fmt.Sprintf("      • Execution delay: %.1fs (limit: 30s)\n", delay)
			}
		case "microstructure":
			if spread, ok := reason.Metrics["spread_bps"]; ok {
				explanation += fmt.Sprintf("      • Spread: %.1f bps (limit: 50 bps)\n", spread)
			}
			if depth, ok := reason.Metrics["depth_usd"]; ok {
				explanation += fmt.Sprintf("      • Depth: $%.0f (limit: $100k)\n", depth)
			}
			if vadr, ok := reason.Metrics["vadr"]; ok {
				explanation += fmt.Sprintf("      • VADR: %.2fx (limit: 1.75x)\n", vadr)
			}
		}
	}
	
	explanation += fmt.Sprintf("\n🕐 Evaluation time: %s\n", result.Timestamp.Format("2006-01-02 15:04:05"))
	
	return explanation
}