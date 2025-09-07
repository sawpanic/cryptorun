// Package microstructure provides execution feasibility gates and venue health checks
// using exchange-native L1/L2 data (Binance/OKX/Coinbase). No aggregators allowed.
package microstructure

import (
	"context"
	"time"
)

// GateReport provides comprehensive microstructure gate evaluation results
type GateReport struct {
	Symbol    string    `json:"symbol"`
	Venue     string    `json:"venue"`
	Timestamp time.Time `json:"timestamp"`

	// Gate results
	DepthOK  bool `json:"depth_ok"`  // Depth within ±2% meets tier requirements
	SpreadOK bool `json:"spread_ok"` // Spread ≤ tier cap (25-80 bps)
	VadrOK   bool `json:"vadr_ok"`   // VADR ≥ tier minimum (1.75-1.85×)

	// Detailed metrics
	Details GateDetails `json:"details"`

	// Overall assessment
	ExecutionFeasible bool     `json:"execution_feasible"` // All gates passed
	RecommendedAction string   `json:"recommended_action"` // "proceed", "halve_size", "defer"
	FailureReasons    []string `json:"failure_reasons,omitempty"`
}

// GateDetails contains the raw measurements and calculations
type GateDetails struct {
	// Depth measurements (USD)
	BidDepthUSD      float64 `json:"bid_depth_usd"`      // Total bids within -2%
	AskDepthUSD      float64 `json:"ask_depth_usd"`      // Total asks within +2%
	TotalDepthUSD    float64 `json:"total_depth_usd"`    // Combined depth
	DepthRequiredUSD float64 `json:"depth_required_usd"` // Tier requirement

	// Spread measurements
	SpreadBps    float64 `json:"spread_bps"`     // Current spread in basis points
	SpreadCapBps float64 `json:"spread_cap_bps"` // Tier cap in basis points

	// VADR measurements
	VADRCurrent float64 `json:"vadr_current"` // Current VADR multiple
	VADRMinimum float64 `json:"vadr_minimum"` // Tier minimum required

	// Venue health
	VenueHealth VenueHealthStatus `json:"venue_health"`

	// Liquidity tier
	LiquidityTier string  `json:"liquidity_tier"` // "tier1", "tier2", "tier3"
	ADV           float64 `json:"adv"`            // Average Daily Volume (USD)

	// Processing metadata
	DataAge      time.Duration `json:"data_age"`      // Age of L1/L2 data
	ProcessingMs int64         `json:"processing_ms"` // Gate evaluation time
	DataQuality  string        `json:"data_quality"`  // "excellent", "good", "degraded"
}

// VenueHealthStatus tracks venue operational metrics
type VenueHealthStatus struct {
	Healthy        bool      `json:"healthy"`
	RejectRate     float64   `json:"reject_rate"`    // % orders rejected (last 15min)
	LatencyP99Ms   int64     `json:"latency_p99_ms"` // 99th percentile latency
	ErrorRate      float64   `json:"error_rate"`     // % API errors (last 15min)
	LastUpdate     time.Time `json:"last_update"`
	Recommendation string    `json:"recommendation"` // "full_size", "halve_size", "avoid"
	UptimePercent  float64   `json:"uptime_percent"` // 24h uptime percentage
}

// OrderBookSnapshot represents L1/L2 order book data from exchange
type OrderBookSnapshot struct {
	Symbol    string           `json:"symbol"`
	Venue     string           `json:"venue"`
	Timestamp time.Time        `json:"timestamp"`
	Bids      []PriceLevel     `json:"bids"` // Descending by price
	Asks      []PriceLevel     `json:"asks"` // Ascending by price
	LastPrice float64          `json:"last_price"`
	Metadata  SnapshotMetadata `json:"metadata"`
}

// PriceLevel represents a single order book level
type PriceLevel struct {
	Price float64 `json:"price"` // Price per unit
	Size  float64 `json:"size"`  // Quantity available
}

// SnapshotMetadata contains snapshot quality information
type SnapshotMetadata struct {
	Source      string        `json:"source"`       // "binance", "okx", "coinbase"
	Sequence    int64         `json:"sequence"`     // Exchange sequence number
	IsStale     bool          `json:"is_stale"`     // Data older than 5s
	UpdateAge   time.Duration `json:"update_age"`   // Time since last update
	BookQuality string        `json:"book_quality"` // "full", "partial", "degraded"
}

// LiquidityTier defines execution size and quality requirements by ADV
type LiquidityTier struct {
	Name         string  `json:"name"`           // "tier1", "tier2", "tier3"
	ADVMin       float64 `json:"adv_min"`        // Minimum ADV (USD) for tier
	ADVMax       float64 `json:"adv_max"`        // Maximum ADV (USD) for tier
	DepthMinUSD  float64 `json:"depth_min_usd"`  // Minimum depth within ±2%
	SpreadCapBps float64 `json:"spread_cap_bps"` // Maximum spread (basis points)
	VADRMinimum  float64 `json:"vadr_minimum"`   // Minimum VADR multiple
	Description  string  `json:"description"`    // Human readable description
}

// SnapshotResult contains microstructure data for entry gate evaluation
type SnapshotResult struct {
	// Basic microstructure metrics
	VADR           float64 `json:"vadr"`             // Volume-Adjusted Daily Range
	SpreadBps      float64 `json:"spread_bps"`       // Bid-ask spread in basis points
	DepthUSD       float64 `json:"depth_usd"`        // Total depth within ±2%
	DailyVolumeUSD float64 `json:"daily_volume_usd"` // 24h volume in USD

	// Bar and timing data
	BarCount        int           `json:"bar_count"`         // Number of bars in VADR calculation
	BarsFromTrigger int           `json:"bars_from_trigger"` // Bars since signal trigger
	LateFillDelay   time.Duration `json:"late_fill_delay"`   // Time delay from signal bar close

	// Trend quality indicators
	ADX   float64 `json:"adx"`   // Average Directional Index
	Hurst float64 `json:"hurst"` // Hurst exponent

	// Metadata
	Timestamp time.Time     `json:"timestamp"` // When data was captured
	DataAge   time.Duration `json:"data_age"`  // Age of underlying data
}

// Evaluator is the main interface for microstructure gate evaluation
type Evaluator interface {
	// EvaluateSnapshot performs basic evaluation returning core metrics
	EvaluateSnapshot(symbol string) (EvaluationResult, error)
}

// FullEvaluator extends Evaluator with comprehensive evaluation capabilities
type FullEvaluator interface {
	Evaluator

	// EvaluateGates performs comprehensive gate evaluation for a symbol/venue
	EvaluateGates(ctx context.Context, symbol, venue string, orderbook *OrderBookSnapshot, adv float64) (*GateReport, error)

	// GetLiquidityTier determines tier based on ADV
	GetLiquidityTier(adv float64) *LiquidityTier

	// UpdateVenueHealth updates venue health metrics
	UpdateVenueHealth(venue string, health VenueHealthStatus) error

	// GetVenueHealth retrieves current venue health status
	GetVenueHealth(venue string) (*VenueHealthStatus, error)
}

// Config holds microstructure evaluator configuration
type Config struct {
	// Rolling averages
	SpreadWindowSeconds int `yaml:"spread_window_seconds"` // Default: 60
	DepthWindowSeconds  int `yaml:"depth_window_seconds"`  // Default: 60

	// Data quality thresholds
	MaxDataAgeSeconds int `yaml:"max_data_age_seconds"` // Default: 5
	MinBookLevels     int `yaml:"min_book_levels"`      // Default: 5

	// Venue health thresholds
	RejectRateThreshold float64 `yaml:"reject_rate_threshold"` // Default: 5.0%
	LatencyThresholdMs  int64   `yaml:"latency_threshold_ms"`  // Default: 2000ms
	ErrorRateThreshold  float64 `yaml:"error_rate_threshold"`  // Default: 3.0%

	// Supported venues (USD pairs only)
	SupportedVenues []string `yaml:"supported_venues"` // ["binance", "okx", "coinbase"]

	// Liquidity tiers configuration
	LiquidityTiers []LiquidityTier `yaml:"liquidity_tiers"`
}

// DefaultConfig returns production-ready configuration
func DefaultConfig() *Config {
	return &Config{
		SpreadWindowSeconds: 60,
		DepthWindowSeconds:  60,
		MaxDataAgeSeconds:   5,
		MinBookLevels:       5,
		RejectRateThreshold: 5.0,
		LatencyThresholdMs:  2000,
		ErrorRateThreshold:  3.0,
		SupportedVenues:     []string{"binance", "okx", "coinbase"},
		LiquidityTiers: []LiquidityTier{
			{
				Name:         "tier1",
				ADVMin:       5000000, // $5M+ ADV
				ADVMax:       1e12,    // No upper limit
				DepthMinUSD:  150000,  // $150k depth
				SpreadCapBps: 25,      // 25 bps max spread
				VADRMinimum:  1.85,    // 1.85× minimum VADR
				Description:  "High liquidity: Large caps, stablecoins",
			},
			{
				Name:         "tier2",
				ADVMin:       1000000, // $1M-5M ADV
				ADVMax:       5000000,
				DepthMinUSD:  75000, // $75k depth
				SpreadCapBps: 50,    // 50 bps max spread
				VADRMinimum:  1.80,  // 1.80× minimum VADR
				Description:  "Medium liquidity: Mid caps",
			},
			{
				Name:         "tier3",
				ADVMin:       100000, // $100k-1M ADV
				ADVMax:       1000000,
				DepthMinUSD:  25000, // $25k depth
				SpreadCapBps: 80,    // 80 bps max spread
				VADRMinimum:  1.75,  // 1.75× minimum VADR
				Description:  "Lower liquidity: Small caps",
			},
		},
	}
}

// VenueMetrics tracks venue operational performance
type VenueMetrics struct {
	Venue           string        `json:"venue"`
	RecentRequests  []RequestLog  `json:"recent_requests"` // Last 100 requests
	RecentErrors    []ErrorLog    `json:"recent_errors"`   // Last 50 errors
	HealthHistory   []HealthPoint `json:"health_history"`  // Last 24h health
	LastHealthCheck time.Time     `json:"last_health_check"`
}

// RequestLog tracks individual API requests
type RequestLog struct {
	Timestamp  time.Time `json:"timestamp"`
	Endpoint   string    `json:"endpoint"`
	LatencyMs  int64     `json:"latency_ms"`
	Success    bool      `json:"success"`
	StatusCode int       `json:"status_code"`
	ErrorCode  string    `json:"error_code,omitempty"`
}

// ErrorLog tracks API errors
type ErrorLog struct {
	Timestamp   time.Time `json:"timestamp"`
	Endpoint    string    `json:"endpoint"`
	ErrorType   string    `json:"error_type"`
	ErrorCode   string    `json:"error_code"`
	Message     string    `json:"message"`
	Recoverable bool      `json:"recoverable"`
}

// HealthPoint tracks venue health over time
type HealthPoint struct {
	Timestamp    time.Time `json:"timestamp"`
	Healthy      bool      `json:"healthy"`
	RejectRate   float64   `json:"reject_rate"`
	LatencyP99Ms int64     `json:"latency_p99_ms"`
	ErrorRate    float64   `json:"error_rate"`
}
