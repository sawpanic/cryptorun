package domain

// FreshnessGate validates signal freshness
type FreshnessGate struct{}

// EvaluateFreshnessGate checks if signal is fresh (≤2 bars & ≤1.2×ATR)
func EvaluateFreshnessGate(input FreshnessGateInputs) GateEvidence {
	barsAge, priceMove, atr := input.BarsAge, input.PriceChange, input.ATR1h
	if barsAge > 2 {
		return GateEvidence{Allow: false, Reason: "stale_bars"}
	}
	if priceMove > 1.2*atr {
		return GateEvidence{Allow: false, Reason: "excessive_move"}
	}
	return GateEvidence{Allow: true, Reason: "fresh"}
}
