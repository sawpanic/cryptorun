package domain

import (
	"math"
	"time"
)

// FreshnessGate enforces ≤2 bars old & within 1.2×ATR(1h) constraint
type FreshnessGate struct {
	MaxBarAge    int     `json:"max_bar_age"`     // Maximum bars old (default: 2)
	ATRMultiple  float64 `json:"atr_multiple"`    // ATR multiple for freshness (default: 1.2)
	Description  string  `json:"description"`
}

// FreshnessInput contains data needed for freshness evaluation
type FreshnessInput struct {
	Symbol         string    `json:"symbol"`
	CurrentPrice   float64   `json:"current_price"`
	LastBarPrice   float64   `json:"last_bar_price"`
	LastBarTime    time.Time `json:"last_bar_time"`
	ATR1h          float64   `json:"atr_1h"`
	BarsAge        int       `json:"bars_age"`
	CurrentTime    time.Time `json:"current_time"`
}

// DefaultFreshnessGate returns the default freshness gate configuration
func DefaultFreshnessGate() FreshnessGate {
	return FreshnessGate{
		MaxBarAge:   2,
		ATRMultiple: 1.2,
		Description: "Freshness gate: ≤2 bars old & within 1.2×ATR(1h)",
	}
}

// EvaluateFreshnessGate checks if the signal meets freshness requirements
func EvaluateFreshnessGate(input FreshnessInput, gate FreshnessGate) GateEvidence {
	// Check age constraint
	if input.BarsAge > gate.MaxBarAge {
		return GateEvidence{
			OK:        false,
			Value:     float64(input.BarsAge),
			Threshold: float64(gate.MaxBarAge),
			Name:      "freshness_age",
		}
	}

	// Check price movement constraint (within ATR multiple)
	if math.IsNaN(input.ATR1h) || input.ATR1h <= 0 {
		// If ATR is not available, only check age constraint
		return GateEvidence{
			OK:        true,
			Value:     float64(input.BarsAge),
			Threshold: float64(gate.MaxBarAge),
			Name:      "freshness_age_only",
		}
	}

	// Calculate price movement from last bar
	priceMove := math.Abs(input.CurrentPrice - input.LastBarPrice)
	maxAllowedMove := input.ATR1h * gate.ATRMultiple

	if priceMove > maxAllowedMove {
		return GateEvidence{
			OK:        false,
			Value:     priceMove,
			Threshold: maxAllowedMove,
			Name:      "freshness_atr",
		}
	}

	// Both age and ATR constraints satisfied
	return GateEvidence{
		OK:        true,
		Value:     math.Max(float64(input.BarsAge)/float64(gate.MaxBarAge), priceMove/maxAllowedMove),
		Threshold: 1.0,
		Name:      "freshness",
	}
}

// CalculateBarsAge determines how many bars old the signal is
func CalculateBarsAge(signalTime, currentTime time.Time, barDuration time.Duration) int {
	if barDuration <= 0 {
		return 0
	}

	timeDiff := currentTime.Sub(signalTime)
	if timeDiff < 0 {
		return 0 // Future signal
	}

	barsAge := int(timeDiff / barDuration)
	return barsAge
}

// GetFreshnessStatus returns a human-readable freshness status
func GetFreshnessStatus(input FreshnessInput, gate FreshnessGate) string {
	evidence := EvaluateFreshnessGate(input, gate)
	
	if evidence.OK {
		return "FRESH"
	}

	switch evidence.Name {
	case "freshness_age":
		return "STALE_AGE"
	case "freshness_atr":
		return "STALE_PRICE"
	default:
		return "STALE"
	}
}

// BuildFreshnessInput creates a FreshnessInput from available market data
func BuildFreshnessInput(symbol string, currentPrice, lastBarPrice float64, lastBarTime time.Time, atr1h float64, barsAge int) FreshnessInput {
	return FreshnessInput{
		Symbol:       symbol,
		CurrentPrice: currentPrice,
		LastBarPrice: lastBarPrice,
		LastBarTime:  lastBarTime,
		ATR1h:        atr1h,
		BarsAge:      barsAge,
		CurrentTime:  time.Now().UTC(),
	}
}