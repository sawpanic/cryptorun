package ports

import "context"

type ProxyInputs struct {
	// Booleans precomputed upstream in scan step (from free sources).
	FundingZBelowNeg15     bool // funding_z < -1.5
	SpotAboveVWAP24h       bool
	ExchangeReserves7dDown bool   // reserves_7d <= -5% across >=3 venues (if available)
	WhaleAccum2of3         bool   // composite
	VolumeFirstBarP80      bool   // first 15m bar >= p80(vadr_24h)
	Regime                 string // "risk_on" | "risk_off" | "btc_driven" | "selective"
}

// ProxyResult represents the output of supply-squeeze proxy evaluation
type ProxyResult struct {
	GatesPassed          int
	RequireVolumeConfirm bool
	GateDetails          map[string]bool // Which gates passed/failed
	Score                float64         // Aggregated score from gates
}

type SupplyProxy interface {
	// Returns gatesPassed: count of gates satisfied; and requireVolumeConfirm bool for risk_off/btc_driven.
	Evaluate(pi ProxyInputs) (gatesPassed int, requireVolumeConfirm bool)

	// EvaluateDetailed returns comprehensive proxy analysis with gates A-C evaluation
	EvaluateDetailed(ctx context.Context, inputs ProxyInputs) (*ProxyResult, error)
}
