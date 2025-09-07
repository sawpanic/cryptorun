package microstructure

import "time"

// EvaluationResult contains microstructure evaluation metrics
type EvaluationResult struct {
	// Core microstructure metrics
	SpreadBps float64 `json:"spread_bps"` // Bid-ask spread in basis points
	DepthUSD  float64 `json:"depth_usd"`  // Total depth within ±2%
	VADR      float64 `json:"vadr"`       // Volume-Adjusted Daily Range

	// Volume and bar metrics for gates
	BarCount       int     `json:"bar_count"`        // Number of bars for volume surge validation
	DailyVolumeUSD float64 `json:"daily_volume_usd"` // Average daily volume in USD

	// Technical indicators for trend quality gates
	ADX   float64 `json:"adx"`   // Average Directional Index
	Hurst float64 `json:"hurst"` // Hurst exponent for trend persistence

	// Freshness/timing metrics for guards
	BarsFromTrigger int           `json:"bars_from_trigger"` // Number of bars since signal trigger
	LateFillDelay   time.Duration `json:"late_fill_delay"`   // Time delay since bar close

	// Overall health indicator
	Healthy bool `json:"healthy"` // All gates pass and venue is healthy
}
