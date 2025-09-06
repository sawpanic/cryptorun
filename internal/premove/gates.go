package premove

import (
	"context"
	"fmt"
	"math"
	"sort"
	"time"

	"cryptorun/internal/microstructure"
)

// GateEvaluator implements Pre-Movement v3.3 2-of-3 confirmation logic
type GateEvaluator struct {
	microEvaluator *microstructure.Evaluator
	config         *GateConfig
}

// NewGateEvaluator creates a Pre-Movement v3.3 gate evaluator
func NewGateEvaluator(microEvaluator *microstructure.Evaluator, config *GateConfig) *GateEvaluator {
	if config == nil {
		config = DefaultGateConfig()
	}
	return &GateEvaluator{
		microEvaluator: microEvaluator,
		config:         config,
	}
}

// GateConfig contains thresholds for Pre-Movement v3.3 gate evaluation
type GateConfig struct {
	// Core 2-of-3 confirmation gates
	FundingDivergenceThreshold float64 `yaml:"funding_divergence_threshold"` // ≥2.0σ z-score
	SupplySqueezeThreshold     float64 `yaml:"supply_squeeze_threshold"`     // ≥0.6 proxy score
	WhaleCompositeThreshold    float64 `yaml:"whale_composite_threshold"`    // ≥0.7 composite

	// Supply squeeze proxy components (2-of-4 required)
	ReserveDepletionThreshold  float64 `yaml:"reserve_depletion_threshold"`  // ≤-5% cross-venue
	LargeWithdrawalsThreshold  float64 `yaml:"large_withdrawals_threshold"`  // ≥$50M/24h
	StakingInflowThreshold     float64 `yaml:"staking_inflow_threshold"`     // ≥$10M/24h
	DerivativesLeverageThreshold float64 `yaml:"derivatives_leverage_threshold"` // ≥15% OI increase

	// Volume confirmation additive (risk_off/btc_driven regimes)
	VolumeConfirmationEnabled  bool    `yaml:"volume_confirmation_enabled"`  // Enable additive volume gate
	VolumeConfirmationThreshold float64 `yaml:"volume_confirmation_threshold"` // ≥2.5× average

	// Precedence weights (for tie-breaking when >2 gates pass)
	FundingPrecedence float64 `yaml:"funding_precedence"` // 3.0 (highest priority)
	WhalePrecedence   float64 `yaml:"whale_precedence"`   // 2.0 (medium priority)
	SupplyPrecedence  float64 `yaml:"supply_precedence"`  // 1.0 (lowest priority)

	// Gate evaluation timeouts
	MaxEvaluationTimeMs int64 `yaml:"max_evaluation_time_ms"` // 500ms timeout
}

// DefaultGateConfig returns Pre-Movement v3.3 production gate configuration
func DefaultGateConfig() *GateConfig {
	return &GateConfig{
		// Core 2-of-3 thresholds
		FundingDivergenceThreshold: 2.0, // 2.0σ
		SupplySqueezeThreshold:     0.6, // 60% proxy confidence
		WhaleCompositeThreshold:    0.7, // 70% whale activity

		// Supply squeeze components (2-of-4)
		ReserveDepletionThreshold:    -5.0, // -5% reserves
		LargeWithdrawalsThreshold:    50e6, // $50M withdrawals
		StakingInflowThreshold:       10e6, // $10M staking
		DerivativesLeverageThreshold: 15.0, // 15% OI increase

		// Volume confirmation
		VolumeConfirmationEnabled:   true,
		VolumeConfirmationThreshold: 2.5, // 2.5× volume

		// Precedence weights
		FundingPrecedence: 3.0, // Funding has highest precedence
		WhalePrecedence:   2.0, // Whale activity medium precedence  
		SupplyPrecedence:  1.0, // Supply squeeze lowest precedence

		// Performance limits
		MaxEvaluationTimeMs: 500, // 500ms timeout
	}
}

// ConfirmationData contains all inputs for Pre-Movement v3.3 gate evaluation
type ConfirmationData struct {
	Symbol    string    `json:"symbol"`
	Timestamp time.Time `json:"timestamp"`

	// Core confirmation signals
	FundingZScore    float64 `json:"funding_z_score"`    // Cross-venue funding z-score
	WhaleComposite   float64 `json:"whale_composite"`    // Whale activity composite 0-1
	SupplyProxyScore float64 `json:"supply_proxy_score"` // Supply squeeze proxy 0-1

	// Supply squeeze components
	ReserveChange7d      float64 `json:"reserve_change_7d"`      // % change in exchange reserves
	LargeWithdrawals24h  float64 `json:"large_withdrawals_24h"`  // $ large withdrawals 24h
	StakingInflow24h     float64 `json:"staking_inflow_24h"`     // $ staking inflows 24h  
	DerivativesOIChange  float64 `json:"derivatives_oi_change"`  // % OI change 24h

	// Volume confirmation (regime-dependent)
	VolumeRatio24h float64 `json:"volume_ratio_24h"` // Current/average volume ratio
	CurrentRegime  string  `json:"current_regime"`   // "risk_off", "btc_driven", "normal"

	// Microstructure context
	SpreadBps   float64 `json:"spread_bps"`   // Current spread basis points
	DepthUSD    float64 `json:"depth_usd"`    // Available depth USD
	VADR        float64 `json:"vadr"`         // Volume-adjusted daily range
}

// ConfirmationResult contains the complete gate evaluation result
type ConfirmationResult struct {
	Symbol           string                 `json:"symbol"`
	Timestamp        time.Time              `json:"timestamp"`
	Passed           bool                   `json:"passed"`           // 2-of-3 + microstructure passed
	ConfirmationCount int                   `json:"confirmation_count"` // Number of core gates passed
	RequiredCount     int                   `json:"required_count"`   // Required gates (2 or 3)
	PassedGates       []string              `json:"passed_gates"`     // Names of passed gates
	FailedGates       []string              `json:"failed_gates"`     // Names of failed gates
	GateResults       map[string]*GateCheck `json:"gate_results"`     // Detailed gate results
	PrecedenceScore   float64               `json:"precedence_score"` // Weighted precedence for ranking
	VolumeBoost       bool                  `json:"volume_boost"`     // Volume confirmation applied
	SupplyBreakdown   *SupplySqueezeBreakdown `json:"supply_breakdown"` // Supply proxy component details
	MicroReport       *microstructure.EvaluationResult `json:"micro_report"` // Microstructure evaluation
	EvaluationTimeMs  int64                 `json:"evaluation_time_ms"`
	Warnings          []string              `json:"warnings"`
}

// SupplySqueezeBreakdown shows which supply components triggered
type SupplySqueezeBreakdown struct {
	ProxyScore         float64             `json:"proxy_score"`         // Final 0-1 proxy score
	ComponentResults   map[string]*GateCheck `json:"component_results"`   // Individual 2-of-4 results
	PassedComponents   []string            `json:"passed_components"`   // Names of passed components
	ComponentCount     int                 `json:"component_count"`     // Number passed (need ≥2)
	RequiredComponents int                 `json:"required_components"` // Required components (2)
}

// GateCheck represents individual gate evaluation result (reused from existing gates package)
type GateCheck struct {
	Name        string      `json:"name"`
	Passed      bool        `json:"passed"`
	Value       interface{} `json:"value"`
	Threshold   interface{} `json:"threshold"`
	Description string      `json:"description"`
}

// EvaluateConfirmation performs comprehensive Pre-Movement v3.3 gate evaluation
func (ge *GateEvaluator) EvaluateConfirmation(ctx context.Context, data *ConfirmationData) (*ConfirmationResult, error) {
	startTime := time.Now()

	result := &ConfirmationResult{
		Symbol:          data.Symbol,
		Timestamp:       time.Now(),
		RequiredCount:   2, // Base requirement: 2-of-3 
		GateResults:     make(map[string]*GateCheck),
		PassedGates:     []string{},
		FailedGates:     []string{},
		Warnings:        []string{},
	}

	// Evaluate core 2-of-3 confirmation gates
	ge.evaluateFundingDivergence(data, result)
	ge.evaluateWhaleComposite(data, result)
	ge.evaluateSupplySqueezeProxy(data, result)

	// Check volume confirmation boost for specific regimes
	if ge.config.VolumeConfirmationEnabled {
		ge.evaluateVolumeConfirmation(data, result)
	}

	// Calculate final confirmation status
	result.ConfirmationCount = len(result.PassedGates)
	
	// Adjust required count for volume boost in specific regimes
	requiredCount := 2
	if result.VolumeBoost && (data.CurrentRegime == "risk_off" || data.CurrentRegime == "btc_driven") {
		requiredCount = 1 // Volume boost reduces requirement to 1-of-3 + volume
		result.RequiredCount = requiredCount
	}

	coreConfirmationPassed := result.ConfirmationCount >= requiredCount

	// Evaluate microstructure as consultation (not blocking for Pre-Movement)
	microResult, err := ge.microEvaluator.EvaluateSnapshot(ctx, data.Symbol)
	if err != nil {
		result.Warnings = append(result.Warnings, fmt.Sprintf("Microstructure evaluation failed: %v", err))
	} else {
		result.MicroReport = microResult
	}

	// Overall pass/fail: 2-of-3 confirmations (microstructure is consultative)
	result.Passed = coreConfirmationPassed
	
	// Calculate precedence score for ranking multiple candidates
	result.PrecedenceScore = ge.calculatePrecedenceScore(result)

	result.EvaluationTimeMs = time.Since(startTime).Milliseconds()

	// Performance warning if evaluation took too long
	if result.EvaluationTimeMs > ge.config.MaxEvaluationTimeMs {
		result.Warnings = append(result.Warnings, 
			fmt.Sprintf("Gate evaluation took %dms (>%dms threshold)", 
				result.EvaluationTimeMs, ge.config.MaxEvaluationTimeMs))
	}

	return result, nil
}

// evaluateFundingDivergence checks cross-venue funding rate divergence
func (ge *GateEvaluator) evaluateFundingDivergence(data *ConfirmationData, result *ConfirmationResult) {
	gate := &GateCheck{
		Name:        "funding_divergence",
		Value:       data.FundingZScore,
		Threshold:   ge.config.FundingDivergenceThreshold,
		Description: fmt.Sprintf("Funding z-score %.2f ≥ %.2f", data.FundingZScore, ge.config.FundingDivergenceThreshold),
	}
	
	gate.Passed = data.FundingZScore >= ge.config.FundingDivergenceThreshold
	result.GateResults["funding_divergence"] = gate

	if gate.Passed {
		result.PassedGates = append(result.PassedGates, "funding_divergence")
	} else {
		result.FailedGates = append(result.FailedGates, "funding_divergence")
	}
}

// evaluateWhaleComposite checks whale activity patterns
func (ge *GateEvaluator) evaluateWhaleComposite(data *ConfirmationData, result *ConfirmationResult) {
	gate := &GateCheck{
		Name:        "whale_composite",
		Value:       data.WhaleComposite,
		Threshold:   ge.config.WhaleCompositeThreshold,
		Description: fmt.Sprintf("Whale composite %.2f ≥ %.2f", data.WhaleComposite, ge.config.WhaleCompositeThreshold),
	}
	
	gate.Passed = data.WhaleComposite >= ge.config.WhaleCompositeThreshold
	result.GateResults["whale_composite"] = gate

	if gate.Passed {
		result.PassedGates = append(result.PassedGates, "whale_composite")
	} else {
		result.FailedGates = append(result.FailedGates, "whale_composite")
	}
}

// evaluateSupplySqueezeProxy checks supply squeeze using 2-of-4 component logic
func (ge *GateEvaluator) evaluateSupplySqueezeProxy(data *ConfirmationData, result *ConfirmationResult) {
	breakdown := &SupplySqueezeBreakdown{
		ComponentResults:   make(map[string]*GateCheck),
		PassedComponents:   []string{},
		RequiredComponents: 2, // Need 2-of-4 components
	}

	// Component 1: Reserve depletion
	reserveGate := &GateCheck{
		Name:        "reserve_depletion",
		Value:       data.ReserveChange7d,
		Threshold:   ge.config.ReserveDepletionThreshold,
		Description: fmt.Sprintf("Reserve change %.1f%% ≤ %.1f%%", data.ReserveChange7d, ge.config.ReserveDepletionThreshold),
		Passed:      data.ReserveChange7d <= ge.config.ReserveDepletionThreshold,
	}
	breakdown.ComponentResults["reserve_depletion"] = reserveGate

	// Component 2: Large withdrawals
	withdrawalGate := &GateCheck{
		Name:        "large_withdrawals",
		Value:       data.LargeWithdrawals24h,
		Threshold:   ge.config.LargeWithdrawalsThreshold,
		Description: fmt.Sprintf("Withdrawals $%.1fM ≥ $%.1fM", data.LargeWithdrawals24h/1e6, ge.config.LargeWithdrawalsThreshold/1e6),
		Passed:      data.LargeWithdrawals24h >= ge.config.LargeWithdrawalsThreshold,
	}
	breakdown.ComponentResults["large_withdrawals"] = withdrawalGate

	// Component 3: Staking inflows
	stakingGate := &GateCheck{
		Name:        "staking_inflow",
		Value:       data.StakingInflow24h,
		Threshold:   ge.config.StakingInflowThreshold,
		Description: fmt.Sprintf("Staking $%.1fM ≥ $%.1fM", data.StakingInflow24h/1e6, ge.config.StakingInflowThreshold/1e6),
		Passed:      data.StakingInflow24h >= ge.config.StakingInflowThreshold,
	}
	breakdown.ComponentResults["staking_inflow"] = stakingGate

	// Component 4: Derivatives leverage
	derivsGate := &GateCheck{
		Name:        "derivatives_oi",
		Value:       data.DerivativesOIChange,
		Threshold:   ge.config.DerivativesLeverageThreshold,
		Description: fmt.Sprintf("OI change %.1f%% ≥ %.1f%%", data.DerivativesOIChange, ge.config.DerivativesLeverageThreshold),
		Passed:      data.DerivativesOIChange >= ge.config.DerivativesLeverageThreshold,
	}
	breakdown.ComponentResults["derivatives_oi"] = derivsGate

	// Count passed components
	for name, gate := range breakdown.ComponentResults {
		if gate.Passed {
			breakdown.PassedComponents = append(breakdown.PassedComponents, name)
		}
	}
	breakdown.ComponentCount = len(breakdown.PassedComponents)

	// Calculate proxy score based on component strength
	breakdown.ProxyScore = ge.calculateSupplyProxyScore(data, breakdown)

	// Main supply squeeze gate passes if proxy score meets threshold
	supplyGate := &GateCheck{
		Name:        "supply_squeeze",
		Value:       breakdown.ProxyScore,
		Threshold:   ge.config.SupplySqueezeThreshold,
		Description: fmt.Sprintf("Supply proxy %.2f ≥ %.2f (%d/4 components)", 
			breakdown.ProxyScore, ge.config.SupplySqueezeThreshold, breakdown.ComponentCount),
		Passed:      breakdown.ProxyScore >= ge.config.SupplySqueezeThreshold,
	}

	result.GateResults["supply_squeeze"] = supplyGate
	result.SupplyBreakdown = breakdown

	if supplyGate.Passed {
		result.PassedGates = append(result.PassedGates, "supply_squeeze")
	} else {
		result.FailedGates = append(result.FailedGates, "supply_squeeze")
	}
}

// evaluateVolumeConfirmation checks additive volume confirmation for specific regimes
func (ge *GateEvaluator) evaluateVolumeConfirmation(data *ConfirmationData, result *ConfirmationResult) {
	// Only apply volume boost in risk_off or btc_driven regimes
	if data.CurrentRegime != "risk_off" && data.CurrentRegime != "btc_driven" {
		return
	}

	volumeGate := &GateCheck{
		Name:        "volume_confirmation",
		Value:       data.VolumeRatio24h,
		Threshold:   ge.config.VolumeConfirmationThreshold,
		Description: fmt.Sprintf("Volume ratio %.2f× ≥ %.2f× (regime: %s)", 
			data.VolumeRatio24h, ge.config.VolumeConfirmationThreshold, data.CurrentRegime),
		Passed:      data.VolumeRatio24h >= ge.config.VolumeConfirmationThreshold,
	}

	result.GateResults["volume_confirmation"] = volumeGate
	result.VolumeBoost = volumeGate.Passed

	if volumeGate.Passed {
		result.PassedGates = append(result.PassedGates, "volume_confirmation")
	}
}

// calculateSupplyProxyScore computes weighted supply squeeze proxy score from components
func (ge *GateEvaluator) calculateSupplyProxyScore(data *ConfirmationData, breakdown *SupplySqueezeBreakdown) float64 {
	var score float64
	components := 0

	// Reserve depletion contribution (0-0.3)
	if breakdown.ComponentResults["reserve_depletion"].Passed {
		reserveStrength := math.Min(1.0, math.Abs(data.ReserveChange7d)/20.0) // -20% = max strength
		score += reserveStrength * 0.3
		components++
	}

	// Large withdrawals contribution (0-0.25)
	if breakdown.ComponentResults["large_withdrawals"].Passed {
		withdrawalStrength := math.Min(1.0, data.LargeWithdrawals24h/100e6) // $100M = max strength
		score += withdrawalStrength * 0.25
		components++
	}

	// Staking inflows contribution (0-0.2)
	if breakdown.ComponentResults["staking_inflow"].Passed {
		stakingStrength := math.Min(1.0, data.StakingInflow24h/25e6) // $25M = max strength
		score += stakingStrength * 0.2
		components++
	}

	// Derivatives leverage contribution (0-0.25)
	if breakdown.ComponentResults["derivatives_oi"].Passed {
		oiStrength := math.Min(1.0, data.DerivativesOIChange/50.0) // 50% = max strength
		score += oiStrength * 0.25
		components++
	}

	// Require at least 2 components for valid proxy score
	if components < 2 {
		return 0.0
	}

	return score
}

// calculatePrecedenceScore computes weighted precedence for ranking candidates
func (ge *GateEvaluator) calculatePrecedenceScore(result *ConfirmationResult) float64 {
	var score float64

	// Weight passed gates by precedence
	for _, gateName := range result.PassedGates {
		switch gateName {
		case "funding_divergence":
			score += ge.config.FundingPrecedence
		case "whale_composite":
			score += ge.config.WhalePrecedence  
		case "supply_squeeze":
			score += ge.config.SupplyPrecedence
		case "volume_confirmation":
			score += 0.5 // Additive boost precedence
		}
	}

	return score
}

// GetConfirmationSummary returns a concise summary of gate evaluation
func (cr *ConfirmationResult) GetConfirmationSummary() string {
	status := "❌ BLOCKED"
	if cr.Passed {
		status = "✅ CONFIRMED"
	}

	volumeNote := ""
	if cr.VolumeBoost {
		volumeNote = " +VOL"
	}

	return fmt.Sprintf("%s — %s (%d/%d gates%s, %.1f precedence, %dms)",
		status, cr.Symbol, cr.ConfirmationCount, cr.RequiredCount, 
		volumeNote, cr.PrecedenceScore, cr.EvaluationTimeMs)
}

// GetDetailedReport returns comprehensive confirmation gate report
func (cr *ConfirmationResult) GetDetailedReport() string {
	report := fmt.Sprintf("Pre-Movement v3.3 Confirmation: %s\n", cr.Symbol)
	report += fmt.Sprintf("Status: %s | Gates: %d/%d | Precedence: %.1f | Time: %dms\n\n",
		map[bool]string{true: "CONFIRMED ✅", false: "BLOCKED ❌"}[cr.Passed],
		cr.ConfirmationCount, cr.RequiredCount, cr.PrecedenceScore, cr.EvaluationTimeMs)

	// Core gate results
	report += "Core Gates (2-of-3 required):\n"
	coreGates := []string{"funding_divergence", "whale_composite", "supply_squeeze"}
	
	for _, gateName := range coreGates {
		if gate, exists := cr.GateResults[gateName]; exists {
			status := map[bool]string{true: "✅", false: "❌"}[gate.Passed]
			report += fmt.Sprintf("  %s %s: %s\n", status, gateName, gate.Description)
		}
	}

	// Supply squeeze breakdown
	if cr.SupplyBreakdown != nil {
		report += fmt.Sprintf("\nSupply Squeeze Components (%d/4 passed, need ≥2):\n", cr.SupplyBreakdown.ComponentCount)
		componentOrder := []string{"reserve_depletion", "large_withdrawals", "staking_inflow", "derivatives_oi"}
		
		for _, compName := range componentOrder {
			if comp, exists := cr.SupplyBreakdown.ComponentResults[compName]; exists {
				status := map[bool]string{true: "✅", false: "❌"}[comp.Passed]
				report += fmt.Sprintf("    %s %s: %s\n", status, compName, comp.Description)
			}
		}
	}

	// Volume confirmation
	if cr.VolumeBoost {
		if gate, exists := cr.GateResults["volume_confirmation"]; exists {
			report += fmt.Sprintf("\nVolume Confirmation: ✅ %s\n", gate.Description)
		}
	}

	// Microstructure consultation
	if cr.MicroReport != nil {
		report += fmt.Sprintf("\nMicrostructure Consultation:\n")
		report += fmt.Sprintf("  Spread: %.1f bps | Depth: $%.0f | VADR: %.2f×\n",
			cr.MicroReport.SpreadBps, cr.MicroReport.DepthUSD, cr.MicroReport.VADR)
	}

	// Warnings
	if len(cr.Warnings) > 0 {
		report += fmt.Sprintf("\nWarnings:\n")
		for i, warning := range cr.Warnings {
			report += fmt.Sprintf("  %d. %s\n", i+1, warning)
		}
	}

	return report
}

// RankCandidates sorts confirmation results by precedence score (highest first)
func RankCandidates(results []*ConfirmationResult) []*ConfirmationResult {
	ranked := make([]*ConfirmationResult, len(results))
	copy(ranked, results)
	
	sort.Slice(ranked, func(i, j int) bool {
		// First sort by pass/fail status
		if ranked[i].Passed != ranked[j].Passed {
			return ranked[i].Passed // Passed results come first
		}
		// Then sort by precedence score (higher first)
		return ranked[i].PrecedenceScore > ranked[j].PrecedenceScore
	})
	
	return ranked
}