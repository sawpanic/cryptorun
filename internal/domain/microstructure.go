package domain

import "time"

// MicrostructureMetrics represents real-time market microstructure data
type MicrostructureMetrics struct {
	Symbol           string    `json:"symbol"`
	SpreadBps        float64   `json:"spread_bps"`
	DepthUSD2Pct     float64   `json:"depth_usd_2pct"`
	VADR             float64   `json:"vadr"`
	VenueHealth      string    `json:"venue_health"`
	LastUpdate       time.Time `json:"last_update"`
	IsExchangeNative bool      `json:"is_exchange_native"`
	TickCount        int64     `json:"tick_count"`

	// Gate validation results
	SpreadOK         bool `json:"spread_ok"`
	DepthOK          bool `json:"depth_ok"`
	VADROK           bool `json:"vadr_ok"`
	VenueHealthOK    bool `json:"venue_health_ok"`
	MicrostructureOK bool `json:"microstructure_ok"`
}
