// Package micro provides hardened exchange-native microstructure collectors
// for L1 (bid/ask/spread) and L2 depth (±2%) with per-venue health telemetry.
// NO AGGREGATORS - exchange-native only.
package micro

import (
	"context"
	"time"
)

// L1Data represents Level 1 order book data (best bid/ask)
type L1Data struct {
	Symbol    string    `json:"symbol"`
	Venue     string    `json:"venue"`
	Timestamp time.Time `json:"timestamp"`

	// L1 pricing
	BidPrice  float64 `json:"bid_price"`
	BidSize   float64 `json:"bid_size"`
	AskPrice  float64 `json:"ask_price"`
	AskSize   float64 `json:"ask_size"`
	LastPrice float64 `json:"last_price"`

	// Derived metrics
	SpreadBps float64 `json:"spread_bps"` // Bid-ask spread in basis points
	MidPrice  float64 `json:"mid_price"`  // (bid + ask) / 2

	// Metadata
	Sequence int64         `json:"sequence"` // Exchange sequence number
	DataAge  time.Duration `json:"data_age"` // Age when processed
	Quality  DataQuality   `json:"quality"`  // excellent/good/degraded
}

// L2Data represents Level 2 order book data (depth within ±2%)
type L2Data struct {
	Symbol    string    `json:"symbol"`
	Venue     string    `json:"venue"`
	Timestamp time.Time `json:"timestamp"`

	// Depth measurements (USD equivalent)
	BidDepthUSD   float64 `json:"bid_depth_usd"`   // Total bids within -2%
	AskDepthUSD   float64 `json:"ask_depth_usd"`   // Total asks within +2%
	TotalDepthUSD float64 `json:"total_depth_usd"` // Combined depth
	BidLevels     int     `json:"bid_levels"`      // Number of bid levels
	AskLevels     int     `json:"ask_levels"`      // Number of ask levels

	// Liquidity gradient (depth@0.5% to depth@2% ratio)
	LiquidityGradient float64 `json:"liquidity_gradient"` // Higher = more concentrated

	// VADR input feed (not VADR calculation itself)
	VADRInputVolume float64 `json:"vadr_input_volume"` // Volume for VADR calc
	VADRInputRange  float64 `json:"vadr_input_range"`  // Price range for VADR calc

	// Metadata
	Sequence   int64         `json:"sequence"`     // Exchange sequence number
	DataAge    time.Duration `json:"data_age"`     // Age when processed
	Quality    DataQuality   `json:"quality"`      // excellent/good/degraded
	IsUSDQuote bool          `json:"is_usd_quote"` // True if USD quote, false if flagged/skipped
}

// VenueHealth tracks operational metrics for exchange venues
type VenueHealth struct {
	Venue     string    `json:"venue"`
	Timestamp time.Time `json:"timestamp"`

	// Health status
	Status  HealthStatus `json:"status"`  // red/yellow/green
	Healthy bool         `json:"healthy"` // Overall health flag

	// Operational metrics (60s rolling stats)
	Uptime           float64 `json:"uptime"`             // % uptime in last 60s
	HeartbeatAgeMs   int64   `json:"heartbeat_age_ms"`   // Age of last heartbeat
	MessageGapRate   float64 `json:"message_gap_rate"`   // % messages with gaps
	WSReconnectCount int     `json:"ws_reconnect_count"` // Reconnects in 60s

	// Performance metrics
	LatencyP50Ms int64   `json:"latency_p50_ms"` // 50th percentile
	LatencyP99Ms int64   `json:"latency_p99_ms"` // 99th percentile
	ErrorRate    float64 `json:"error_rate"`     // % errors in 60s

	// Data quality
	DataFreshness    time.Duration `json:"data_freshness"`    // Age of newest data
	DataCompleteness float64       `json:"data_completeness"` // % complete messages

	// Recommendation
	Recommendation string `json:"recommendation"` // proceed/halve_size/avoid
}

// CollectorMetrics tracks collector performance over 1s aggregation windows
type CollectorMetrics struct {
	Venue       string    `json:"venue"`
	WindowStart time.Time `json:"window_start"`
	WindowEnd   time.Time `json:"window_end"`

	// Message counts
	L1Messages    int64 `json:"l1_messages"`    // L1 updates in window
	L2Messages    int64 `json:"l2_messages"`    // L2 updates in window
	ErrorMessages int64 `json:"error_messages"` // Errors in window

	// Processing stats
	ProcessingTimeMs int64   `json:"processing_time_ms"` // Total processing time
	AvgLatencyMs     float64 `json:"avg_latency_ms"`     // Average message latency
	MaxLatencyMs     int64   `json:"max_latency_ms"`     // Maximum message latency

	// Quality stats
	StaleDataCount  int64   `json:"stale_data_count"` // Messages older than 5s
	IncompleteCount int64   `json:"incomplete_count"` // Incomplete messages
	QualityScore    float64 `json:"quality_score"`    // 0-100 quality score
}

// DataQuality represents data quality assessment
type DataQuality string

const (
	QualityExcellent DataQuality = "excellent"
	QualityGood      DataQuality = "good"
	QualityDegraded  DataQuality = "degraded"
)

// HealthStatus represents venue health status
type HealthStatus string

const (
	HealthRed    HealthStatus = "red"
	HealthYellow HealthStatus = "yellow"
	HealthGreen  HealthStatus = "green"
)

// Collector interface for exchange-native L1/L2 data collection
type Collector interface {
	// Start begins data collection for the venue
	Start(ctx context.Context) error

	// Stop gracefully shuts down the collector
	Stop(ctx context.Context) error

	// GetL1Data returns the latest L1 data for a symbol
	GetL1Data(symbol string) (*L1Data, error)

	// GetL2Data returns the latest L2 data for a symbol
	GetL2Data(symbol string) (*L2Data, error)

	// GetVenueHealth returns current venue health status
	GetVenueHealth() (*VenueHealth, error)

	// GetMetrics returns collector performance metrics
	GetMetrics() (*CollectorMetrics, error)

	// Subscribe to symbol updates (USD pairs only)
	Subscribe(symbols []string) error

	// Unsubscribe from symbol updates
	Unsubscribe(symbols []string) error

	// Venue returns the venue name
	Venue() string

	// IsHealthy returns true if venue is currently healthy
	IsHealthy() bool
}

// CollectorConfig holds configuration for venue collectors
type CollectorConfig struct {
	// Venue identification
	Venue   string `yaml:"venue"`    // binance/okx/coinbase
	BaseURL string `yaml:"base_url"` // REST API base URL
	WSURL   string `yaml:"ws_url"`   // WebSocket URL

	// Sampling configuration
	AggregationWindowMs  int `yaml:"aggregation_window_ms"`   // Default: 1000 (1s)
	RollingStatsWindowMs int `yaml:"rolling_stats_window_ms"` // Default: 60000 (60s)

	// Health thresholds
	MaxHeartbeatAgeMs       int64   `yaml:"max_heartbeat_age_ms"`       // Default: 10000
	MaxMessageGapRate       float64 `yaml:"max_message_gap_rate"`       // Default: 0.05 (5%)
	MaxErrorRate            float64 `yaml:"max_error_rate"`             // Default: 0.03 (3%)
	MaxLatencyP99Ms         int64   `yaml:"max_latency_p99_ms"`         // Default: 2000
	MinDataCompletenessRate float64 `yaml:"min_data_completeness_rate"` // Default: 0.95 (95%)

	// Data quality thresholds
	MaxDataAgeMs         int64   `yaml:"max_data_age_ms"`        // Default: 5000
	MinLiquidityGradient float64 `yaml:"min_liquidity_gradient"` // Default: 0.1

	// Rate limiting
	MaxRequestsPerSecond   int `yaml:"max_requests_per_second"`    // Default: 10
	MaxWSConnectionsPerMin int `yaml:"max_ws_connections_per_min"` // Default: 5

	// Artifacts
	HealthCSVPath   string `yaml:"health_csv_path"`   // Health CSV file path
	EnableHealthCSV bool   `yaml:"enable_health_csv"` // Default: true
}

// DefaultConfig returns production-ready collector configuration
func DefaultConfig(venue string) *CollectorConfig {
	baseConfig := &CollectorConfig{
		Venue:                   venue,
		AggregationWindowMs:     1000,  // 1s windows
		RollingStatsWindowMs:    60000, // 60s rolling stats
		MaxHeartbeatAgeMs:       10000, // 10s max heartbeat age
		MaxMessageGapRate:       0.05,  // 5% max gap rate
		MaxErrorRate:            0.03,  // 3% max error rate
		MaxLatencyP99Ms:         2000,  // 2s max P99 latency
		MinDataCompletenessRate: 0.95,  // 95% min completeness
		MaxDataAgeMs:            5000,  // 5s max data age
		MinLiquidityGradient:    0.1,   // 0.1 min gradient
		MaxRequestsPerSecond:    10,    // 10 RPS max
		MaxWSConnectionsPerMin:  5,     // 5 WS connections/min
		EnableHealthCSV:         true,
	}

	// Venue-specific URLs
	switch venue {
	case "binance":
		baseConfig.BaseURL = "https://api.binance.com"
		baseConfig.WSURL = "wss://stream.binance.com:9443/ws"
		baseConfig.HealthCSVPath = "./artifacts/micro/health_binance.csv"
	case "okx":
		baseConfig.BaseURL = "https://www.okx.com"
		baseConfig.WSURL = "wss://ws.okx.com:8443/ws/v5/public"
		baseConfig.HealthCSVPath = "./artifacts/micro/health_okx.csv"
	case "coinbase":
		baseConfig.BaseURL = "https://api.pro.coinbase.com"
		baseConfig.WSURL = "wss://ws-feed.pro.coinbase.com"
		baseConfig.HealthCSVPath = "./artifacts/micro/health_coinbase.csv"
	default:
		// Use generic paths for unknown venues
		baseConfig.BaseURL = "https://api.unknown.com"
		baseConfig.WSURL = "wss://ws.unknown.com"
		baseConfig.HealthCSVPath = "./artifacts/micro/health_" + venue + ".csv"
	}

	return baseConfig
}

// HealthCSVRecord represents a single health record for CSV export
type HealthCSVRecord struct {
	Timestamp        string  `csv:"timestamp"`
	Venue            string  `csv:"venue"`
	Status           string  `csv:"status"`
	Healthy          string  `csv:"healthy"`
	Uptime           float64 `csv:"uptime"`
	HeartbeatAgeMs   int64   `csv:"heartbeat_age_ms"`
	MessageGapRate   float64 `csv:"message_gap_rate"`
	WSReconnectCount int     `csv:"ws_reconnect_count"`
	LatencyP50Ms     int64   `csv:"latency_p50_ms"`
	LatencyP99Ms     int64   `csv:"latency_p99_ms"`
	ErrorRate        float64 `csv:"error_rate"`
	DataFreshnessMs  int64   `csv:"data_freshness_ms"`
	DataCompleteness float64 `csv:"data_completeness"`
	Recommendation   string  `csv:"recommendation"`
}

// ToCSVRecord converts VenueHealth to CSV record
func (vh *VenueHealth) ToCSVRecord() *HealthCSVRecord {
	return &HealthCSVRecord{
		Timestamp:        vh.Timestamp.Format(time.RFC3339),
		Venue:            vh.Venue,
		Status:           string(vh.Status),
		Healthy:          boolToString(vh.Healthy),
		Uptime:           vh.Uptime,
		HeartbeatAgeMs:   vh.HeartbeatAgeMs,
		MessageGapRate:   vh.MessageGapRate,
		WSReconnectCount: vh.WSReconnectCount,
		LatencyP50Ms:     vh.LatencyP50Ms,
		LatencyP99Ms:     vh.LatencyP99Ms,
		ErrorRate:        vh.ErrorRate,
		DataFreshnessMs:  int64(vh.DataFreshness / time.Millisecond),
		DataCompleteness: vh.DataCompleteness,
		Recommendation:   vh.Recommendation,
	}
}

// boolToString converts boolean to string for CSV
func boolToString(b bool) string {
	if b {
		return "true"
	}
	return "false"
}
