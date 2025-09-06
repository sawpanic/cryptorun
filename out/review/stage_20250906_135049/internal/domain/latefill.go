package domain

import "time"

// LateFillGate validates execution timing
type LateFillGate struct{}

// EvaluateLateFillGate checks if fill timing is acceptable (<30s)
func EvaluateLateFillGate(input LateFillGateInputs) GateEvidence {
	delay := input.ExecutionTime.Sub(input.SignalTime)
	if delay < 0 || delay > 30*time.Second {
		return GateEvidence{Allow: false, Reason: "late_fill"}
	}
	return GateEvidence{Allow: true, Reason: "timely_fill"}
}