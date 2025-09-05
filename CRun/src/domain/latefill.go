package domain

import (
	"time"
)

// LateFillGate enforces reject fills >30s after signal bar close
type LateFillGate struct {
	MaxFillDelay time.Duration `json:"max_fill_delay"`
	Description  string        `json:"description"`
}

// LateFillInput contains data needed for late fill evaluation
type LateFillInput struct {
	Symbol          string    `json:"symbol"`
	SignalBarClose  time.Time `json:"signal_bar_close"`
	FillTime        time.Time `json:"fill_time"`
	OrderPlaceTime  time.Time `json:"order_place_time"`
	FillDelay       time.Duration `json:"fill_delay"`
}

// DefaultLateFillGate returns the default late fill gate configuration
func DefaultLateFillGate() LateFillGate {
	return LateFillGate{
		MaxFillDelay: 30 * time.Second,
		Description:  "Late-fill gate: reject fills >30s after signal bar close",
	}
}

// EvaluateLateFillGate checks if the fill timing meets requirements
func EvaluateLateFillGate(input LateFillInput, gate LateFillGate) GateEvidence {
	// Calculate actual delay from signal bar close to fill
	fillDelay := input.FillTime.Sub(input.SignalBarClose)
	
	// Negative delay means fill happened before bar close (invalid)
	if fillDelay < 0 {
		return GateEvidence{
			OK:        false,
			Value:     fillDelay.Seconds(),
			Threshold: 0.0,
			Name:      "latefill_early",
		}
	}

	// Check if fill delay exceeds threshold
	if fillDelay > gate.MaxFillDelay {
		return GateEvidence{
			OK:        false,
			Value:     fillDelay.Seconds(),
			Threshold: gate.MaxFillDelay.Seconds(),
			Name:      "latefill_late",
		}
	}

	// Fill timing is acceptable
	return GateEvidence{
		OK:        true,
		Value:     fillDelay.Seconds(),
		Threshold: gate.MaxFillDelay.Seconds(),
		Name:      "latefill",
	}
}

// CalculateFillDelay computes the delay between signal and fill
func CalculateFillDelay(signalTime, fillTime time.Time) time.Duration {
	return fillTime.Sub(signalTime)
}

// GetLateFillStatus returns a human-readable late fill status
func GetLateFillStatus(input LateFillInput, gate LateFillGate) string {
	evidence := EvaluateLateFillGate(input, gate)
	
	if evidence.OK {
		return "ON_TIME"
	}

	switch evidence.Name {
	case "latefill_early":
		return "EARLY_FILL"
	case "latefill_late":
		return "LATE_FILL"
	default:
		return "TIMING_ERROR"
	}
}

// BuildLateFillInput creates a LateFillInput from timing data
func BuildLateFillInput(symbol string, signalBarClose, fillTime, orderPlaceTime time.Time) LateFillInput {
	return LateFillInput{
		Symbol:         symbol,
		SignalBarClose: signalBarClose,
		FillTime:       fillTime,
		OrderPlaceTime: orderPlaceTime,
		FillDelay:      CalculateFillDelay(signalBarClose, fillTime),
	}
}

// IsWithinFillWindow checks if current time is within acceptable fill window
func IsWithinFillWindow(signalBarClose time.Time, maxDelay time.Duration) bool {
	currentTime := time.Now().UTC()
	timeSinceSignal := currentTime.Sub(signalBarClose)
	
	return timeSinceSignal >= 0 && timeSinceSignal <= maxDelay
}

// GetRemainingFillTime returns how much time is left in the fill window
func GetRemainingFillTime(signalBarClose time.Time, maxDelay time.Duration) time.Duration {
	currentTime := time.Now().UTC()
	windowCloseTime := signalBarClose.Add(maxDelay)
	
	remaining := windowCloseTime.Sub(currentTime)
	if remaining < 0 {
		return 0
	}
	
	return remaining
}