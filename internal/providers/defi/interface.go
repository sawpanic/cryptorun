package defi

import (
	"context"
	"time"
)

// DeFiMetrics represents DeFi protocol metrics from free APIs
type DeFiMetrics struct {
	Timestamp        time.Time `json:"timestamp"`
	Protocol         string    `json:"protocol"`         // Protocol name (e.g., "uniswap-v3", "aave")
	TokenSymbol      string    `json:"token_symbol"`     // Token symbol (USD pairs only)
	
	// TVL (Total Value Locked) Data  
	TVL              float64   `json:"tvl"`              // Current TVL in USD
	TVLChange24h     float64   `json:"tvl_change_24h"`   // 24h TVL change percentage
	TVLChange7d      float64   `json:"tvl_change_7d"`    // 7d TVL change percentage
	
	// AMM Pool Metrics (Uniswap/SushiSwap/etc.)
	PoolVolume24h    float64   `json:"pool_volume_24h"`  // 24h trading volume in USD
	PoolLiquidity    float64   `json:"pool_liquidity"`   // Current liquidity in USD
	PoolFees24h      float64   `json:"pool_fees_24h"`    // 24h fees collected
	
	// Lending Metrics (Aave/Compound/etc.)
	BorrowAPY        float64   `json:"borrow_apy,omitempty"`        // Current borrow APY
	SupplyAPY        float64   `json:"supply_apy,omitempty"`        // Current supply APY
	UtilizationRate  float64   `json:"utilization_rate,omitempty"`  // Utilization rate (0.0-1.0)
	
	// Data Quality
	DataSource       string    `json:"data_source"`      // Source API (thegraph, etc.)
	ConfidenceScore  float64   `json:"confidence_score"` // Quality score 0.0-1.0
	PITShift         int       `json:"pit_shift"`        // Point-in-time shift applied
	TVLRank          int       `json:"tvl_rank"`         // TVL ranking position
}

// DeFiProvider defines the interface for DeFi metrics providers
type DeFiProvider interface {
	// GetProtocolTVL retrieves TVL metrics for a protocol/token
	GetProtocolTVL(ctx context.Context, protocol string, tokenSymbol string) (*DeFiMetrics, error)
	
	// GetPoolMetrics retrieves AMM pool metrics for a token pair
	GetPoolMetrics(ctx context.Context, protocol string, tokenA, tokenB string) (*DeFiMetrics, error)
	
	// GetLendingMetrics retrieves lending protocol metrics
	GetLendingMetrics(ctx context.Context, protocol string, tokenSymbol string) (*DeFiMetrics, error)
	
	// GetTopTVLTokens returns tokens by TVL (USD pairs only)
	GetTopTVLTokens(ctx context.Context, limit int) ([]DeFiMetrics, error)
	
	// Health returns provider health and connectivity status
	Health(ctx context.Context) (*ProviderHealth, error)
	
	// GetSupportedProtocols returns list of supported DeFi protocols
	GetSupportedProtocols(ctx context.Context) ([]string, error)
}

// ProviderHealth represents DeFi provider health status
type ProviderHealth struct {
	Healthy            bool              `json:"healthy"`
	DataSource         string            `json:"data_source"`
	LastUpdate         time.Time         `json:"last_update"`
	LatencyMS          float64           `json:"latency_ms"`
	ErrorRate          float64           `json:"error_rate"`
	SupportedProtocols int               `json:"supported_protocols"`
	DataFreshness      map[string]time.Duration `json:"data_freshness"`
	Errors             []string          `json:"errors,omitempty"`
}

// DeFiProviderConfig holds configuration for DeFi metrics providers
type DeFiProviderConfig struct {
	DataSource       string        `json:"data_source"`       // "thegraph", "defillama", etc.
	BaseURL          string        `json:"base_url"`
	RequestTimeout   time.Duration `json:"request_timeout"`
	RateLimitRPS     float64       `json:"rate_limit_rps"`
	MaxRetries       int           `json:"max_retries"`
	RetryBackoff     time.Duration `json:"retry_backoff"`
	PITShiftPeriods  int           `json:"pit_shift_periods"` // PIT protection shift
	EnableMetrics    bool          `json:"enable_metrics"`
	UserAgent        string        `json:"user_agent"`
	USDPairsOnly     bool          `json:"usd_pairs_only"`    // Enforce USD pairs constraint
}

// DeFiProviderFactory creates DeFi providers for different data sources
type DeFiProviderFactory interface {
	// CreateTheGraphProvider creates The Graph subgraph provider (free tier)
	CreateTheGraphProvider(config DeFiProviderConfig) (DeFiProvider, error)
	
	// CreateDeFiLlamaProvider creates DeFiLlama API provider (free tier)
	CreateDeFiLlamaProvider(config DeFiProviderConfig) (DeFiProvider, error)
	
	// GetAvailableProviders returns list of available DeFi data sources
	GetAvailableProviders() []string
}

// DeFiAggregator combines metrics from multiple DeFi data sources
type DeFiAggregator interface {
	// AggregateProtocolMetrics combines metrics across protocols
	AggregateProtocolMetrics(ctx context.Context, tokenSymbol string, protocols []string) (*AggregatedDeFiMetrics, error)
	
	// GetCrossProtocolTVL calculates total TVL across protocols for a token
	GetCrossProtocolTVL(ctx context.Context, tokenSymbol string) (float64, error)
	
	// ValidateConsistency checks for outliers across data sources
	ValidateConsistency(ctx context.Context, metrics map[string]*DeFiMetrics) (*ConsistencyReport, error)
}

// AggregatedDeFiMetrics represents combined metrics from multiple protocols
type AggregatedDeFiMetrics struct {
	TokenSymbol          string                    `json:"token_symbol"`
	Timestamp            time.Time                 `json:"timestamp"`
	ProtocolCount        int                       `json:"protocol_count"`
	ProtocolMetrics      map[string]*DeFiMetrics   `json:"protocol_metrics"`
	
	// Aggregated Values
	TotalTVL             float64                   `json:"total_tvl"`             // Sum across protocols
	WeightedTVLChange24h float64                   `json:"weighted_tvl_change"`   // TVL-weighted change
	TotalVolume24h       float64                   `json:"total_volume_24h"`      // Sum of pool volumes
	
	// Consensus Indicators
	TVLConsensus         float64                   `json:"tvl_consensus"`         // 0.0-1.0 agreement score
	DataQuality          float64                   `json:"data_quality"`          // Composite quality score
	OutlierProtocols     []string                  `json:"outlier_protocols"`     // Protocols with outlier data
	VenueCount           int                       `json:"venue_count"`           // Number of venues
	VenueMetrics         map[string]*DeFiMetrics   `json:"venue_metrics"`         // Per-venue metrics
	OutlierVenues        []string                  `json:"outlier_venues"`        // Venues with outlier data
}

// ConsistencyReport analyzes data consistency across DeFi sources
type ConsistencyReport struct {
	TokenSymbol          string                 `json:"token_symbol"`
	Timestamp            time.Time              `json:"timestamp"`
	ProtocolCount        int                    `json:"protocol_count"`
	
	// Consistency Scores (0.0-1.0)
	TVLConsistency       float64                `json:"tvl_consistency"`
	VolumeConsistency    float64                `json:"volume_consistency"`
	OverallConsistency   float64                `json:"overall_consistency"`
	
	// Outlier Detection
	Outliers             map[string]OutlierInfo `json:"outliers"`
	OutlierThreshold     float64                `json:"outlier_threshold"`
	
	// Data Quality Flags
	InsufficientData     bool                   `json:"insufficient_data"`
	StaleDataDetected    bool                   `json:"stale_data_detected"`
	HighVarianceWarning  bool                   `json:"high_variance_warning"`
	VenueCount           int                    `json:"venue_count"`
	
	Recommendations      []string               `json:"recommendations"`
}

// OutlierInfo describes an outlier protocol and metric
type OutlierInfo struct {
	Protocol     string  `json:"protocol"`
	Metric       string  `json:"metric"`       // "tvl", "volume", "apy"
	Value        float64 `json:"value"`
	Deviation    float64 `json:"deviation"`    // Standard deviations from mean
	Confidence   float64 `json:"confidence"`   // Outlier confidence 0.0-1.0
}

// MetricsCallback for DeFi provider metrics collection
type MetricsCallback func(metric string, value float64, tags map[string]string)