package market

import "time"

// Snapshot represents market microstructure data
type Snapshot struct {
	Symbol      string    `json:"symbol"`
	Timestamp   time.Time `json:"timestamp"`
	Bid         float64   `json:"bid"`
	Ask         float64   `json:"ask"`
	SpreadBps   float64   `json:"spread_bps"`
	Depth2PcUSD float64   `json:"depth_2pc_usd"`
	VADR        float64   `json:"vadr"`
	ADVUSD      float64   `json:"adv_usd"`
	GatesPass   bool      `json:"gates_pass"`
}

// NewSnapshot creates a new market snapshot
func NewSnapshot(symbol string, bid, ask, spreadBps, depth2PcUSD, vadr, advUSD float64) *Snapshot {
	return &Snapshot{
		Symbol:      symbol,
		Timestamp:   time.Now().UTC(),
		Bid:         bid,
		Ask:         ask,
		SpreadBps:   spreadBps,
		Depth2PcUSD: depth2PcUSD,
		VADR:        vadr,
		ADVUSD:      advUSD,
		GatesPass:   spreadBps < 50.0 && depth2PcUSD >= 100000 && vadr >= 1.75,
	}
}
