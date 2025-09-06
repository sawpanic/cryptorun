package gates

import (
	"context"
	"fmt"
	"time"

	"cryptorun/internal/data/derivs"
	"cryptorun/internal/microstructure"
)

// EntryGateEvaluator enforces hard entry requirements
type EntryGateEvaluator struct {
	microEvaluator  *microstructure.Evaluator
	fundingProvider *derivs.FundingProvider
	oiProvider      *derivs.OpenInterestProvider
	etfProvider     *derivs.ETFProvider
	config          *EntryGateConfig
}

// NewEntryGateEvaluator creates an entry gate evaluator
func NewEntryGateEvaluator(
	microEvaluator *microstructure.Evaluator,
	fundingProvider *derivs.FundingProvider,
	oiProvider *derivs.OpenInterestProvider,
	etfProvider *derivs.ETFProvider,
) *EntryGateEvaluator {
	return &EntryGateEvaluator{
		microEvaluator:  microEvaluator,
		fundingProvider: fundingProvider,
		oiProvider:      oiProvider,
		etfProvider:     etfProvider,
		config:          DefaultEntryGateConfig(),
	}
}

// EntryGateConfig contains hard thresholds for entry gates
type EntryGateConfig struct {
	// Score gate
	MinCompositeScore float64 `yaml:"min_composite_score"` // ≥75

	// Microstructure gates
	MinVADR       float64 `yaml:"min_vadr"`        // ≥1.8×
	MaxSpreadBps  float64 `yaml:"max_spread_bps"`  // ≤50bps
	MinDepthUSD   float64 `yaml:"min_depth_usd"`   // ≥$100k within ±2%
	DepthRangePct float64 `yaml:"depth_range_pct"` // ±2%

	// Funding divergence gate
	MinFundingZScore         float64 `yaml:"min_funding_z_score"`        // ≥2.0 standard deviations
	RequireFundingDivergence bool    `yaml:"require_funding_divergence"` // Must have divergence

	// Optional: OI and ETF gates (can be disabled)
	EnableOIGate   bool    `yaml:"enable_oi_gate"`    // Enable OI residual check
	MinOIResidual  float64 `yaml:"min_oi_residual"`   // ≥$1M OI residual
	EnableETFGate  bool    `yaml:"enable_etf_gate"`   // Enable ETF flow check
	MinETFFlowTint float64 `yaml:"min_etf_flow_tint"` // ≥0.3 tint (positive flows)
}

// DefaultEntryGateConfig returns production-ready gate configuration
func DefaultEntryGateConfig() *EntryGateConfig {
	return &EntryGateConfig{
		// Core gates (always enforced)
		MinCompositeScore: 75.0,
		MinVADR:           1.8,
		MaxSpreadBps:      50.0,
		MinDepthUSD:       100000.0, // $100k
		DepthRangePct:     2.0,      // ±2%

		// Funding divergence (always enforced)
		MinFundingZScore:         2.0,
		RequireFundingDivergence: true,

		// Optional gates (can be disabled for symbols without data)
		EnableOIGate:   true,
		MinOIResidual:  1000000.0, // $1M
		EnableETFGate:  true,
		MinETFFlowTint: 0.3, // 30% net inflow tint
	}
}

// EntryGateResult contains the evaluation result and detailed reasoning
type EntryGateResult struct {
	Symbol           string                `json:"symbol"`
	Timestamp        time.Time             `json:"timestamp"`
	Passed           bool                  `json:"passed"`
	CompositeScore   float64               `json:"composite_score"`
	GateResults      map[string]*GateCheck `json:"gate_results"`    // gate_name -> result
	FailureReasons   []string              `json:"failure_reasons"` // List of failed gate descriptions
	PassedGates      []string              `json:"passed_gates"`    // List of passed gate names
	EvaluationTimeMs int64                 `json:"evaluation_time_ms"`
}

// GateCheck represents the result of a single gate evaluation
type GateCheck struct {
	Name        string      `json:"name"`
	Passed      bool        `json:"passed"`
	Value       interface{} `json:"value"`       // Actual measured value
	Threshold   interface{} `json:"threshold"`   // Required threshold
	Description string      `json:"description"` // Human-readable description
}

// EvaluateEntry performs comprehensive entry gate evaluation
func (ege *EntryGateEvaluator) EvaluateEntry(ctx context.Context, symbol string, compositeScore float64, priceChange24h float64) (*EntryGateResult, error) {
	startTime := time.Now()

	result := &EntryGateResult{
		Symbol:         symbol,
		Timestamp:      time.Now(),
		CompositeScore: compositeScore,
		GateResults:    make(map[string]*GateCheck),
		FailureReasons: []string{},
		PassedGates:    []string{},
	}

	// Gate 1: Composite Score ≥ 75
	scoreCheck := &GateCheck{
		Name:        "composite_score",
		Value:       compositeScore,
		Threshold:   ege.config.MinCompositeScore,
		Description: fmt.Sprintf("Composite score %.1f ≥ %.1f", compositeScore, ege.config.MinCompositeScore),
	}
	scoreCheck.Passed = compositeScore >= ege.config.MinCompositeScore
	result.GateResults["composite_score"] = scoreCheck

	if scoreCheck.Passed {
		result.PassedGates = append(result.PassedGates, "composite_score")
	} else {
		result.FailureReasons = append(result.FailureReasons, fmt.Sprintf("Score %.1f below threshold %.1f", compositeScore, ege.config.MinCompositeScore))
	}

	// Gate 2: Microstructure Gates (VADR ≥ 1.8×, spread, depth)
	microResult, err := ege.microEvaluator.EvaluateSnapshot(ctx, symbol)
	if err != nil {
		return nil, fmt.Errorf("microstructure evaluation failed: %w", err)
	}

	// VADR check
	vadrCheck := &GateCheck{
		Name:        "vadr",
		Value:       microResult.VADR,
		Threshold:   ege.config.MinVADR,
		Description: fmt.Sprintf("VADR %.2f× ≥ %.2f×", microResult.VADR, ege.config.MinVADR),
	}
	vadrCheck.Passed = microResult.VADR >= ege.config.MinVADR
	result.GateResults["vadr"] = vadrCheck

	// Spread check
	spreadCheck := &GateCheck{
		Name:        "spread",
		Value:       microResult.SpreadBps,
		Threshold:   ege.config.MaxSpreadBps,
		Description: fmt.Sprintf("Spread %.1f bps ≤ %.1f bps", microResult.SpreadBps, ege.config.MaxSpreadBps),
	}
	spreadCheck.Passed = microResult.SpreadBps <= ege.config.MaxSpreadBps
	result.GateResults["spread"] = spreadCheck

	// Depth check
	depthCheck := &GateCheck{
		Name:        "depth",
		Value:       microResult.DepthUSD,
		Threshold:   ege.config.MinDepthUSD,
		Description: fmt.Sprintf("Depth $%.0f ≥ $%.0f within ±%.1f%%", microResult.DepthUSD, ege.config.MinDepthUSD, ege.config.DepthRangePct),
	}
	depthCheck.Passed = microResult.DepthUSD >= ege.config.MinDepthUSD
	result.GateResults["depth"] = depthCheck

	// Update passed/failed lists
	for _, check := range []*GateCheck{vadrCheck, spreadCheck, depthCheck} {
		if check.Passed {
			result.PassedGates = append(result.PassedGates, check.Name)
		} else {
			result.FailureReasons = append(result.FailureReasons, check.Description+" FAILED")
		}
	}

	// Gate 3: Funding Divergence Present
	if ege.config.RequireFundingDivergence {
		fundingSnapshot, err := ege.fundingProvider.GetFundingSnapshot(ctx, symbol)
		if err != nil {
			// Funding data unavailable - this is a hard failure
			fundingCheck := &GateCheck{
				Name:        "funding_divergence",
				Value:       "unavailable",
				Threshold:   ege.config.MinFundingZScore,
				Description: "Funding divergence data unavailable",
				Passed:      false,
			}
			result.GateResults["funding_divergence"] = fundingCheck
			result.FailureReasons = append(result.FailureReasons, "Funding divergence data unavailable")
		} else {
			fundingCheck := &GateCheck{
				Name:        "funding_divergence",
				Value:       fundingSnapshot.MaxDivergence,
				Threshold:   ege.config.MinFundingZScore,
				Description: fmt.Sprintf("Funding z-score %.2f ≥ %.2f", fundingSnapshot.MaxDivergence, ege.config.MinFundingZScore),
			}
			fundingCheck.Passed = fundingSnapshot.HasSignificantDivergence(ege.config.MinFundingZScore)
			result.GateResults["funding_divergence"] = fundingCheck

			if fundingCheck.Passed {
				result.PassedGates = append(result.PassedGates, "funding_divergence")
			} else {
				venue, zScore := fundingSnapshot.GetDivergentVenue()
				result.FailureReasons = append(result.FailureReasons,
					fmt.Sprintf("Insufficient funding divergence (max %.2f at %s, need ≥%.2f)",
						zScore, venue, ege.config.MinFundingZScore))
			}
		}
	}

	// Gate 4: Optional OI Gate
	if ege.config.EnableOIGate {
		oiSnapshot, err := ege.oiProvider.GetOpenInterestSnapshot(ctx, symbol, priceChange24h)
		if err != nil {
			// OI data unavailable - log but don't fail (optional gate)
			oiCheck := &GateCheck{
				Name:        "oi_residual",
				Value:       "unavailable",
				Threshold:   ege.config.MinOIResidual,
				Description: "OI data unavailable (optional)",
				Passed:      true, // Don't fail on missing optional data
			}
			result.GateResults["oi_residual"] = oiCheck
			result.PassedGates = append(result.PassedGates, "oi_residual")
		} else {
			oiCheck := &GateCheck{
				Name:        "oi_residual",
				Value:       oiSnapshot.OIResidual,
				Threshold:   ege.config.MinOIResidual,
				Description: fmt.Sprintf("OI residual $%.0f ≥ $%.0f", oiSnapshot.OIResidual, ege.config.MinOIResidual),
			}
			oiCheck.Passed = oiSnapshot.OIResidual >= ege.config.MinOIResidual
			result.GateResults["oi_residual"] = oiCheck

			if oiCheck.Passed {
				result.PassedGates = append(result.PassedGates, "oi_residual")
			} else {
				result.FailureReasons = append(result.FailureReasons,
					fmt.Sprintf("OI residual $%.0f below threshold $%.0f",
						oiSnapshot.OIResidual, ege.config.MinOIResidual))
			}
		}
	}

	// Gate 5: Optional ETF Gate
	if ege.config.EnableETFGate {
		etfSnapshot, err := ege.etfProvider.GetETFFlowSnapshot(ctx, symbol)
		if err != nil || len(etfSnapshot.ETFList) == 0 {
			// ETF data unavailable - pass by default (not all assets have ETFs)
			etfCheck := &GateCheck{
				Name:        "etf_flows",
				Value:       "unavailable",
				Threshold:   ege.config.MinETFFlowTint,
				Description: "ETF data unavailable (optional)",
				Passed:      true,
			}
			result.GateResults["etf_flows"] = etfCheck
			result.PassedGates = append(result.PassedGates, "etf_flows")
		} else {
			etfCheck := &GateCheck{
				Name:        "etf_flows",
				Value:       etfSnapshot.FlowTint,
				Threshold:   ege.config.MinETFFlowTint,
				Description: fmt.Sprintf("ETF tint %.2f ≥ %.2f", etfSnapshot.FlowTint, ege.config.MinETFFlowTint),
			}
			etfCheck.Passed = etfSnapshot.IsFlowTintBullish(ege.config.MinETFFlowTint)
			result.GateResults["etf_flows"] = etfCheck

			if etfCheck.Passed {
				result.PassedGates = append(result.PassedGates, "etf_flows")
			} else {
				result.FailureReasons = append(result.FailureReasons,
					fmt.Sprintf("ETF tint %.2f below threshold %.2f",
						etfSnapshot.FlowTint, ege.config.MinETFFlowTint))
			}
		}
	}

	// Overall pass/fail determination
	result.Passed = len(result.FailureReasons) == 0

	result.EvaluationTimeMs = time.Since(startTime).Milliseconds()

	return result, nil
}

// GetGateSummary returns a concise summary of gate evaluation
func (egr *EntryGateResult) GetGateSummary() string {
	if egr.Passed {
		return fmt.Sprintf("✅ ENTRY CLEARED — %s (score: %.1f, %d/%d gates passed)",
			egr.Symbol, egr.CompositeScore, len(egr.PassedGates), len(egr.GateResults))
	} else {
		return fmt.Sprintf("❌ ENTRY BLOCKED — %s (%d failures: %s)",
			egr.Symbol, len(egr.FailureReasons), egr.FailureReasons[0])
	}
}

// GetDetailedReport returns a comprehensive gate evaluation report
func (egr *EntryGateResult) GetDetailedReport() string {
	report := fmt.Sprintf("Entry Gate Evaluation: %s (%.1f score)\n", egr.Symbol, egr.CompositeScore)
	report += fmt.Sprintf("Overall: %s | Evaluation: %dms\n\n",
		map[bool]string{true: "PASS ✅", false: "FAIL ❌"}[egr.Passed],
		egr.EvaluationTimeMs)

	// List all gate results
	gateOrder := []string{"composite_score", "vadr", "spread", "depth", "funding_divergence", "oi_residual", "etf_flows"}

	for _, gateName := range gateOrder {
		if check, exists := egr.GateResults[gateName]; exists {
			status := map[bool]string{true: "✅", false: "❌"}[check.Passed]
			report += fmt.Sprintf("%s %s: %s\n", status, check.Name, check.Description)
		}
	}

	if len(egr.FailureReasons) > 0 {
		report += fmt.Sprintf("\nFailure Details:\n")
		for i, reason := range egr.FailureReasons {
			report += fmt.Sprintf("  %d. %s\n", i+1, reason)
		}
	}

	return report
}
