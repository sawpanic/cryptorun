package domain

import (
	"fmt"
	"math"
)

type GateEvidence struct {
	OK        bool    `json:"ok"`
	Value     float64 `json:"value"`
	Threshold float64 `json:"threshold"`
	Name      string  `json:"name"`
}

type MicroGateInputs struct {
	Symbol      string
	Bid         float64
	Ask         float64
	Depth2PcUSD float64
	VADR        float64
	ADVUSD      int64
}

type MicroGateResults struct {
	Symbol    string        `json:"symbol"`
	AllPass   bool          `json:"all_pass"`
	Spread    GateEvidence  `json:"spread"`
	Depth     GateEvidence  `json:"depth"`
	VADR      GateEvidence  `json:"vadr"`
	ADV       GateEvidence  `json:"adv"`
	Reason    string        `json:"reason,omitempty"`
}

func EvaluateMicroGates(inputs MicroGateInputs, thresholds MicroGateThresholds) MicroGateResults {
	spread := evaluateSpreadGate(inputs.Bid, inputs.Ask, thresholds.MaxSpreadBps)
	depth := evaluateDepthGate(inputs.Depth2PcUSD, thresholds.MinDepthUSD)
	vadr := evaluateVADRGate(inputs.VADR, thresholds.MinVADR)
	adv := evaluateADVGate(inputs.ADVUSD, thresholds.MinADVUSD)

	allPass := spread.OK && depth.OK && vadr.OK && adv.OK

	reason := ""
	if !allPass {
		if !spread.OK {
			reason = fmt.Sprintf("spread %.0f bps > %.0f bps threshold", spread.Value, spread.Threshold)
		} else if !depth.OK {
			reason = fmt.Sprintf("depth $%.0f < $%.0f threshold", depth.Value, depth.Threshold)
		} else if !vadr.OK {
			reason = fmt.Sprintf("VADR %.3f < %.3f threshold", vadr.Value, vadr.Threshold)
		} else if !adv.OK {
			reason = fmt.Sprintf("ADV $%.0f < $%.0f threshold", adv.Value, adv.Threshold)
		}
	}

	return MicroGateResults{
		Symbol:  inputs.Symbol,
		AllPass: allPass,
		Spread:  spread,
		Depth:   depth,
		VADR:    vadr,
		ADV:     adv,
		Reason:  reason,
	}
}

type MicroGateThresholds struct {
	MaxSpreadBps float64
	MinDepthUSD  float64
	MinVADR      float64
	MinADVUSD    int64
}

func DefaultMicroGateThresholds() MicroGateThresholds {
	return MicroGateThresholds{
		MaxSpreadBps: 50.0,    // 50 basis points maximum
		MinDepthUSD:  100000,  // $100k minimum depth within ±2%
		MinVADR:      1.75,    // 1.75x minimum VADR
		MinADVUSD:    100000,  // $100k minimum ADV
	}
}

func evaluateSpreadGate(bid, ask, maxSpreadBps float64) GateEvidence {
	// Use precision helper for consistent calculation
	spreadBpsInt := ComputeSpreadBps(bid, ask)
	spreadBps := float64(spreadBpsInt)
	
	// Handle pathological cases (ComputeSpreadBps returns 9999 for invalid inputs)
	if spreadBpsInt == 9999 {
		return GateEvidence{
			OK:        false,
			Value:     spreadBps,
			Threshold: maxSpreadBps,
			Name:      "spread_invalid",
		}
	}

	// Inclusive threshold check: spread_bps <= threshold_bps
	return GateEvidence{
		OK:        spreadBps <= maxSpreadBps,
		Value:     spreadBps,
		Threshold: maxSpreadBps,
		Name:      "spread",
	}
}

func evaluateDepthGate(depth2PcUSD, minDepthUSD float64) GateEvidence {
	// Guard against NaN/Inf inputs
	guardedDepth := GuardFinite(depth2PcUSD, 0.0)
	
	// Inclusive threshold check: depth2pc_usd >= threshold_usd
	return GateEvidence{
		OK:        guardedDepth >= minDepthUSD,
		Value:     math.Round(guardedDepth), // Round to nearest USD as per spec
		Threshold: minDepthUSD,
		Name:      "depth",
	}
}

func evaluateVADRGate(vadr, minVADR float64) GateEvidence {
	// Guard against NaN/Inf inputs - fail safe
	guardedVADR := GuardFinite(vadr, 0.0)
	
	return GateEvidence{
		OK:        guardedVADR >= minVADR,
		Value:     roundToDecimals(guardedVADR, 3),
		Threshold: minVADR,
		Name:      "vadr",
	}
}

func evaluateADVGate(advUSD int64, minADVUSD int64) GateEvidence {
	return GateEvidence{
		OK:        advUSD >= minADVUSD,
		Value:     float64(advUSD),
		Threshold: float64(minADVUSD),
		Name:      "adv",
	}
}

func CalculateSpreadBps(bid, ask float64) float64 {
	if bid <= 0 || ask <= 0 || bid >= ask {
		return math.NaN()
	}
	
	mid := (bid + ask) / 2.0
	spread := ask - bid
	return (spread / mid) * 10000.0
}

func roundToDecimals(value float64, decimals int) float64 {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return value
	}
	
	multiplier := math.Pow(10, float64(decimals))
	return math.Round(value*multiplier) / multiplier
}