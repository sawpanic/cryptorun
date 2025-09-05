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
			reason = fmt.Sprintf("spread %.2f bps > %.2f bps", spread.Value, spread.Threshold)
		} else if !depth.OK {
			reason = fmt.Sprintf("depth $%.0f < $%.0f", depth.Value, depth.Threshold)
		} else if !vadr.OK {
			reason = fmt.Sprintf("VADR %.3f < %.3f", vadr.Value, vadr.Threshold)
		} else if !adv.OK {
			reason = fmt.Sprintf("ADV $%.0f < $%.0f", adv.Value, adv.Threshold)
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
	if bid <= 0 || ask <= 0 || bid >= ask {
		return GateEvidence{
			OK:        false,
			Value:     math.NaN(),
			Threshold: maxSpreadBps,
			Name:      "spread",
		}
	}

	mid := (bid + ask) / 2.0
	spread := ask - bid
	spreadBps := (spread / mid) * 10000.0

	return GateEvidence{
		OK:        spreadBps <= maxSpreadBps,
		Value:     roundToDecimals(spreadBps, 2),
		Threshold: maxSpreadBps,
		Name:      "spread",
	}
}

func evaluateDepthGate(depth2PcUSD, minDepthUSD float64) GateEvidence {
	return GateEvidence{
		OK:        depth2PcUSD >= minDepthUSD,
		Value:     roundToDecimals(depth2PcUSD, 0),
		Threshold: minDepthUSD,
		Name:      "depth",
	}
}

func evaluateVADRGate(vadr, minVADR float64) GateEvidence {
	return GateEvidence{
		OK:        vadr >= minVADR,
		Value:     roundToDecimals(vadr, 3),
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