package domain

import "time"

// FatigueGateInputs represents inputs for fatigue gate validation
type FatigueGateInputs struct {
	Symbol          string  // Trading symbol
	Momentum24h     float64 // 24-hour momentum percentage  
	RSI4h           float64 // 4-hour RSI value
	Acceleration    float64 // 4-hour acceleration (optional override)
}

// GateEvidence represents gate validation result
type GateEvidence struct {
	Allow  bool   `json:"allow"`
	Reason string `json:"reason"`
}

// Gate input types for compatibility
type FreshnessGateInputs struct {
	Symbol      string
	BarsAge     int
	PriceChange float64
	ATR1h       float64
}

type LateFillGateInputs struct {
	Symbol        string
	SignalTime    time.Time
	ExecutionTime time.Time
}

type GateInputs struct {
	Symbol         string
	Spread         float64
	Depth          float64
	VADR           float64
	Momentum24h    float64
	RSI4h          float64
	Acceleration   float64
	BarsAge        int
	PriceChange    float64
	ATR1h          float64
	ExecutionDelay float64
}

type GateResults struct {
	AllowEntry  bool
	BlockReason string
}

// EvaluateFatigueGate validates fatigue gate logic
func EvaluateFatigueGate(inputs FatigueGateInputs) GateEvidence {
	// Block if 24h > +12% AND RSI4h > 70 UNLESS acceleration ≥ 2%
	if inputs.Momentum24h > 12.0 && inputs.RSI4h > 70.0 {
		if inputs.Acceleration < 2.0 {
			return GateEvidence{Allow: false, Reason: "fatigue_block"}
		}
	}
	return GateEvidence{Allow: true, Reason: "fatigue_pass"}
}

func EvaluateAllGates(inputs GateInputs) GateResults {
	return GateResults{AllowEntry: true, BlockReason: ""}
}

// Fatigue gate should block if 24h > +12% and RSI4h > 70 unless acceleration up