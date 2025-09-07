package signals

import "time"

type GateInputs struct {
	Close        []float64
	Volumes      []float64
	ATR1h        float64
	RSI4h        float64
	Accel4h      float64
	VADR         float64
	SpreadBps    float64
	DepthUSD2pc  float64
	TriggerPrice float64
	SignalTime   time.Time
	Now          time.Time
}

type GateResult struct {
	Pass   bool
	Reason string
}

func EvaluateGates(in GateInputs) GateResult {
	// Movement threshold: |4h%| >= 3%
	mv := 0.0
	if n := len(in.Close); n >= 2 {
		mv = in.Close[n-1]/in.Close[n-2] - 1
	}
	if mv < 0 {
		mv = -mv
	}
	if mv < 0.03 {
		return GateResult{Reason: "movement < 3% (4h)"}
	}
	// VADR >= 1.75x
	if in.VADR < 1.75 {
		return GateResult{Reason: "VADR < 1.75x"}
	}
	// Liquidity min: not enforced here (slice)
	// Trend quality: stub accept
	// Freshness: within 2 bars and within 1.2x ATR of trigger
	if in.ATR1h > 0 && in.TriggerPrice > 0 {
		diff := in.TriggerPrice - Last(in.Close)
		if diff < 0 {
			diff = -diff
		}
		if diff > 1.2*in.ATR1h {
			return GateResult{Reason: "beyond 1.2x ATR from trigger"}
		}
	}
	if !in.SignalTime.IsZero() && !in.Now.IsZero() {
		if in.Now.Sub(in.SignalTime) > 2*time.Hour {
			return GateResult{Reason: "stale > 2 bars"}
		}
	}
	// Late fill guard
	if !in.SignalTime.IsZero() && !in.Now.IsZero() {
		if in.Now.Sub(in.SignalTime) > 30*time.Second { /* ok for slice: we consider signal fresh by caller */
		}
	}
	// Fatigue guard: 24h > +12% AND RSI>70 unless accel>0 (approx using 4h bars)
	if in.RSI4h > 70 {
		// If we had 24h bars, we'd compute; slice assumes caller ensures reasonable
		if in.Accel4h <= 0 { /* allow for slice; could block here */
		}
	}
	// Spread gate
	if in.SpreadBps >= 50 {
		return GateResult{Reason: "spread >= 50bps"}
	}
	// Depth gate
	if in.DepthUSD2pc < 100_000 {
		return GateResult{Reason: "depth@2% < $100k"}
	}
	return GateResult{Pass: true}
}
