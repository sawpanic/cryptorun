package proxy

import (
	"context"

	"github.com/sawpanic/cryptorun/src/domain/premove/ports"
)

// Evaluator implements SupplyProxy with gates A-C evaluation
type Evaluator struct {
	// Gate weights for scoring
	gateAWeight float64 // Funding + Spot positioning gates
	gateBWeight float64 // Reserve + Accumulation gates
	gateCWeight float64 // Volume confirmation gate
}

// NewSupplyProxy creates a new supply-squeeze proxy evaluator
func NewSupplyProxy() *Evaluator {
	return &Evaluator{
		gateAWeight: 40.0, // Gate A: Funding + Spot (higher weight)
		gateBWeight: 35.0, // Gate B: Reserves + Whales
		gateCWeight: 25.0, // Gate C: Volume confirmation
	}
}

// Evaluate implements the legacy interface for backward compatibility
func (e *Evaluator) Evaluate(pi ports.ProxyInputs) (gatesPassed int, requireVolumeConfirm bool) {
	gatesPassed = 0
	requireVolumeConfirm = false

	// Gate A: Funding + Spot positioning
	if e.evaluateGateA(pi) {
		gatesPassed++
	}

	// Gate B: Reserves + Whale accumulation
	if e.evaluateGateB(pi) {
		gatesPassed++
	}

	// Gate C: Volume confirmation
	if e.evaluateGateC(pi) {
		gatesPassed++
	}

	// Regime-based volume confirmation requirement
	requireVolumeConfirm = e.requiresVolumeConfirm(pi.Regime)

	return gatesPassed, requireVolumeConfirm
}

// EvaluateDetailed returns comprehensive proxy analysis with gates A-C evaluation
func (e *Evaluator) EvaluateDetailed(ctx context.Context, inputs ports.ProxyInputs) (*ports.ProxyResult, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	result := &ports.ProxyResult{
		GateDetails: make(map[string]bool),
	}

	// Evaluate individual gates
	gateA := e.evaluateGateA(inputs)
	gateB := e.evaluateGateB(inputs)
	gateC := e.evaluateGateC(inputs)

	// Record gate details
	result.GateDetails["gate_a_funding_spot"] = gateA
	result.GateDetails["gate_b_reserves_whales"] = gateB
	result.GateDetails["gate_c_volume"] = gateC
	result.GateDetails["funding_z_below_neg15"] = inputs.FundingZBelowNeg15
	result.GateDetails["spot_above_vwap24h"] = inputs.SpotAboveVWAP24h
	result.GateDetails["exchange_reserves_7d_down"] = inputs.ExchangeReserves7dDown
	result.GateDetails["whale_accum_2of3"] = inputs.WhaleAccum2of3
	result.GateDetails["volume_first_bar_p80"] = inputs.VolumeFirstBarP80

	// Count passed gates
	result.GatesPassed = 0
	if gateA {
		result.GatesPassed++
	}
	if gateB {
		result.GatesPassed++
	}
	if gateC {
		result.GatesPassed++
	}

	// Calculate weighted score
	score := 0.0
	if gateA {
		score += e.gateAWeight
	}
	if gateB {
		score += e.gateBWeight
	}
	if gateC {
		score += e.gateCWeight
	}
	result.Score = score

	// Regime-based volume confirmation requirement
	result.RequireVolumeConfirm = e.requiresVolumeConfirm(inputs.Regime)

	return result, nil
}

// evaluateGateA checks funding and spot positioning conditions
// Gate A: Both funding_z < -1.5 AND spot > vwap_24h must be true
func (e *Evaluator) evaluateGateA(inputs ports.ProxyInputs) bool {
	return inputs.FundingZBelowNeg15 && inputs.SpotAboveVWAP24h
}

// evaluateGateB checks reserves and whale accumulation conditions
// Gate B: At least ONE of exchange_reserves_7d_down OR whale_accum_2of3 must be true
func (e *Evaluator) evaluateGateB(inputs ports.ProxyInputs) bool {
	return inputs.ExchangeReserves7dDown || inputs.WhaleAccum2of3
}

// evaluateGateC checks volume confirmation conditions
// Gate C: volume_first_bar_p80 must be true (high initial volume)
func (e *Evaluator) evaluateGateC(inputs ports.ProxyInputs) bool {
	return inputs.VolumeFirstBarP80
}

// requiresVolumeConfirm determines if volume confirmation is required based on regime
func (e *Evaluator) requiresVolumeConfirm(regime string) bool {
	switch regime {
	case "risk_off", "btc_driven":
		return true // Require volume confirmation in these regimes
	case "risk_on", "selective":
		return false // No volume confirmation required
	default:
		return true // Conservative default: require volume confirmation
	}
}

// GetGateRequirements returns the requirements for each gate (for testing/documentation)
func (e *Evaluator) GetGateRequirements() map[string]string {
	return map[string]string{
		"gate_a":                    "funding_z < -1.5 AND spot > vwap_24h (both required)",
		"gate_b":                    "exchange_reserves_7d_down OR whale_accum_2of3 (at least one required)",
		"gate_c":                    "volume_first_bar_p80 (required)",
		"volume_confirm_regimes":    "risk_off, btc_driven (require volume confirmation)",
		"no_volume_confirm_regimes": "risk_on, selective (no volume confirmation required)",
	}
}
