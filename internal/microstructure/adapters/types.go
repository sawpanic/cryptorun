package adapters

import (
	"context"
	"fmt"
	"time"
)

// L1Data represents Level 1 market data (best bid/ask)
type L1Data struct {
	Symbol    string    `json:"symbol"`    // BTC/USD
	Venue     string    `json:"venue"`     // binance/okx/coinbase
	Timestamp time.Time `json:"timestamp"` // Exchange timestamp

	// L1 pricing
	BidPrice float64 `json:"bid_price"` // Best bid price
	BidSize  float64 `json:"bid_size"`  // Best bid size
	AskPrice float64 `json:"ask_price"` // Best ask price
	AskSize  float64 `json:"ask_size"`  // Best ask size

	// Derived metrics
	SpreadBps float64 `json:"spread_bps"` // Spread in basis points
	MidPrice  float64 `json:"mid_price"`  // (bid + ask) / 2

	// Attribution
	Quality string        `json:"quality"`  // excellent/good/degraded
	DataAge time.Duration `json:"data_age"` // Age when retrieved
}

// L2Data represents Level 2 market data (depth within ±2%)
type L2Data struct {
	Symbol    string    `json:"symbol"`
	Venue     string    `json:"venue"`
	Timestamp time.Time `json:"timestamp"`

	// Depth measurements (USD equivalent)
	BidDepthUSD   float64 `json:"bid_depth_usd"`   // Bids within -2%
	AskDepthUSD   float64 `json:"ask_depth_usd"`   // Asks within +2%
	TotalDepthUSD float64 `json:"total_depth_usd"` // Combined depth
	BidLevels     int     `json:"bid_levels"`      // # bid levels
	AskLevels     int     `json:"ask_levels"`      // # ask levels

	// Liquidity gradient (concentration metric)
	LiquidityGradient float64 `json:"liquidity_gradient"` // depth@0.5% / depth@2%

	// VADR input feed (not VADR calculation itself)
	VADRInputVolume float64 `json:"vadr_input_volume"` // Volume estimate
	VADRInputRange  float64 `json:"vadr_input_range"`  // Range estimate

	// Attribution
	Quality    string `json:"quality"`      // Data quality
	IsUSDQuote bool   `json:"is_usd_quote"` // USD pair validation
}

// DataQuality represents the quality level of market data
type DataQuality string

const (
	DataQualityExcellent DataQuality = "excellent"
	DataQualityGood      DataQuality = "good"
	DataQualityDegraded  DataQuality = "degraded"
)

// OrderBookSnapshot represents a complete order book from an exchange
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

// MicrostructureAdapter defines the interface for exchange-native microstructure adapters
type MicrostructureAdapter interface {
	// GetL1Data fetches best bid/ask data
	GetL1Data(ctx context.Context, symbol string) (*L1Data, error)

	// GetL2Data fetches order book depth data
	GetL2Data(ctx context.Context, symbol string) (*L2Data, error)

	// GetOrderBookSnapshot fetches complete order book snapshot
	GetOrderBookSnapshot(ctx context.Context, symbol string) (*OrderBookSnapshot, error)
}

// AdapterConfig holds configuration for microstructure adapters
type AdapterConfig struct {
	Venue                 string  `yaml:"venue"`                   // binance/okx/coinbase
	Enabled               bool    `yaml:"enabled"`                 // Whether adapter is enabled
	RequestTimeoutSec     int     `yaml:"request_timeout_sec"`     // HTTP request timeout
	MaxRetries            int     `yaml:"max_retries"`             // Max retry attempts
	RateLimitRPS          float64 `yaml:"rate_limit_rps"`          // Rate limit (requests per second)
	CircuitBreakerEnabled bool    `yaml:"circuit_breaker_enabled"` // Enable circuit breaker

	// Health thresholds
	HealthCheckInterval time.Duration `yaml:"health_check_interval"` // Health check frequency
	MaxErrorRate        float64       `yaml:"max_error_rate"`        // Max error rate before degraded
	MaxLatencyMs        int64         `yaml:"max_latency_ms"`        // Max latency before degraded
}

// DefaultAdapterConfig returns default configuration for an adapter
func DefaultAdapterConfig(venue string) *AdapterConfig {
	return &AdapterConfig{
		Venue:                 venue,
		Enabled:               true,
		RequestTimeoutSec:     10,
		MaxRetries:            3,
		RateLimitRPS:          10.0, // Conservative default
		CircuitBreakerEnabled: true,
		HealthCheckInterval:   30 * time.Second,
		MaxErrorRate:          0.05, // 5%
		MaxLatencyMs:          2000, // 2 seconds
	}
}

// AdapterFactory creates microstructure adapters
type AdapterFactory struct {
	configs map[string]*AdapterConfig
}

// NewAdapterFactory creates a new adapter factory
func NewAdapterFactory() *AdapterFactory {
	return &AdapterFactory{
		configs: map[string]*AdapterConfig{
			"binance":  DefaultAdapterConfig("binance"),
			"okx":      DefaultAdapterConfig("okx"),
			"coinbase": DefaultAdapterConfig("coinbase"),
		},
	}
}

// CreateAdapter creates an adapter for the specified venue
func (f *AdapterFactory) CreateAdapter(venue string) (MicrostructureAdapter, error) {
	config, exists := f.configs[venue]
	if !exists || !config.Enabled {
		return nil, fmt.Errorf("venue %s not supported or disabled", venue)
	}

	switch venue {
	case "binance":
		return NewBinanceMicrostructureAdapter(), nil
	case "okx":
		return NewOKXMicrostructureAdapter(), nil
	case "coinbase":
		return NewCoinbaseMicrostructureAdapter(), nil
	default:
		return nil, fmt.Errorf("unknown venue: %s", venue)
	}
}

// GetSupportedVenues returns list of supported venues
func (f *AdapterFactory) GetSupportedVenues() []string {
	var venues []string
	for venue, config := range f.configs {
		if config.Enabled {
			venues = append(venues, venue)
		}
	}
	return venues
}

// SetVenueConfig updates configuration for a venue
func (f *AdapterFactory) SetVenueConfig(venue string, config *AdapterConfig) {
	f.configs[venue] = config
}

// GetVenueConfig returns configuration for a venue
func (f *AdapterFactory) GetVenueConfig(venue string) *AdapterConfig {
	return f.configs[venue]
}
