package domain

import (
	"time"
)

type GateInputs struct { DailyVolUSD float64; VADR float64; SpreadBps float64 }

type EntryDecision struct { Allow bool; Reason string }

func EntryGates(inp GateInputs) EntryDecision {
	if inp.DailyVolUSD < 200000 { return EntryDecision{false, "kraken volume < $200k"} }
	// additional 6 gates: placeholders pass-through
	return EntryDecision{true, "ok"}
}

// FatigueGate enforces: block if 24h > +12% and RSI4h > 70 unless renewed acceleration
type FatigueGate struct {
	MaxMomentum24h      float64 `json:"max_momentum_24h"`      // Maximum 24h momentum (default: 12.0%)
	MaxRSI4h            float64 `json:"max_rsi_4h"`            // Maximum 4h RSI (default: 70.0)
	MinAcceleration     float64 `json:"min_acceleration"`      // Minimum acceleration to override (default: 2.0%)
	Description         string  `json:"description"`
}

// FatigueInput contains data needed for fatigue evaluation
type FatigueInput struct {
	Symbol       string    `json:"symbol"`
	Momentum24h  float64   `json:"momentum_24h"`   // 24h momentum percentage
	RSI4h        float64   `json:"rsi_4h"`         // 4h RSI value  
	Acceleration float64   `json:"acceleration"`   // Recent acceleration
	Timestamp    time.Time `json:"timestamp"`
}

// DefaultFatigueGate returns the default fatigue gate configuration
func DefaultFatigueGate() FatigueGate {
	return FatigueGate{
		MaxMomentum24h:  12.0, // 12%
		MaxRSI4h:        70.0, // RSI 70
		MinAcceleration: 2.0,  // 2% acceleration to override
		Description:     "Fatigue gate: block if 24h > +12% and RSI4h > 70 unless renewed acceleration",
	}
}

// EvaluateFatigueGate checks if the signal shows fatigue
func EvaluateFatigueGate(input FatigueInput, gate FatigueGate) GateEvidence {
	// Check if 24h momentum exceeds threshold
	if input.Momentum24h <= gate.MaxMomentum24h {
		// Momentum is within acceptable range
		return GateEvidence{
			OK:        true,
			Value:     input.Momentum24h,
			Threshold: gate.MaxMomentum24h,
			Name:      "fatigue_momentum_ok",
		}
	}

	// High momentum - check RSI
	if input.RSI4h <= gate.MaxRSI4h {
		// RSI is acceptable even with high momentum
		return GateEvidence{
			OK:        true,
			Value:     input.RSI4h,
			Threshold: gate.MaxRSI4h,
			Name:      "fatigue_rsi_ok",
		}
	}

	// Both momentum and RSI are high - check for renewed acceleration
	if input.Acceleration >= gate.MinAcceleration {
		// Renewed acceleration overrides fatigue concerns
		return GateEvidence{
			OK:        true,
			Value:     input.Acceleration,
			Threshold: gate.MinAcceleration,
			Name:      "fatigue_acceleration_override",
		}
	}

	// Fatigued: high momentum + high RSI + no renewed acceleration
	return GateEvidence{
		OK:        false,
		Value:     input.Momentum24h,
		Threshold: gate.MaxMomentum24h,
		Name:      "fatigue_excessive",
	}
}

// GetFatigueStatus returns a human-readable fatigue status
func GetFatigueStatus(input FatigueInput, gate FatigueGate) string {
	evidence := EvaluateFatigueGate(input, gate)
	
	if evidence.OK {
		switch evidence.Name {
		case "fatigue_momentum_ok":
			return "MOMENTUM_OK"
		case "fatigue_rsi_ok":
			return "RSI_OK"
		case "fatigue_acceleration_override":
			return "ACCELERATION_OVERRIDE"
		default:
			return "OK"
		}
	}

	return "FATIGUED"
}

// BuildFatigueInput creates a FatigueInput from available data
func BuildFatigueInput(symbol string, momentum24h, rsi4h, acceleration float64) FatigueInput {
	return FatigueInput{
		Symbol:       symbol,
		Momentum24h:  momentum24h,
		RSI4h:        rsi4h,
		Acceleration: acceleration,
		Timestamp:    time.Now().UTC(),
	}
}
