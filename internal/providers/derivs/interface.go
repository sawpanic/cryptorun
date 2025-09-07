package derivs

import (
	"context"
	"time"
)

// DerivMetrics represents derivatives data from exchange-native sources
type DerivMetrics struct {
	Timestamp        time.Time `json:"timestamp"`
	Symbol           string    `json:"symbol"`
	Venue            string    `json:"venue"`
	
	// Funding Rate Data
	Funding          float64   `json:"funding"`          // Current funding rate
	FundingZScore    float64   `json:"funding_z_score"`  // Z-score vs 30-period lookback
	NextFundingTime  time.Time `json:"next_funding_time"`
	
	// Open Interest Data
	OpenInterest     float64   `json:"open_interest"`     // Current OI in contracts
	OpenInterestUSD  float64   `json:"open_interest_usd"` // Current OI in USD
	OIResidual       float64   `json:"oi_residual"`       // OI residual after trend removal
	
	// Basis and Calendar Spread
	Basis            float64   `json:"basis"`             // (futures - spot)/spot over tenor
	BasisPercent     float64   `json:"basis_percent"`     // Basis as percentage
	CalendarSpread   float64   `json:"calendar_spread,omitempty"` // Near vs far contract spread
	
	// Volume Analysis
	Volume24h        float64   `json:"volume_24h"`        // 24h trading volume
	VolumeUSD24h     float64   `json:"volume_usd_24h"`    // 24h volume in USD
	VolumeRatio      float64   `json:"volume_ratio"`      // Perp volume / spot volume
	
	// Price Reference
	MarkPrice        float64   `json:"mark_price"`        // Exchange mark price
	IndexPrice       float64   `json:"index_price"`       // Index price (spot reference)
	LastPrice        float64   `json:"last_price"`        // Last traded price
	
	// Data Quality
	DataSource       string    `json:"data_source"`       // Exchange-native source identifier
	ConfidenceScore  float64   `json:"confidence_score"`  // Quality score 0.0-1.0
	PITShift         int       `json:"pit_shift"`         // Point-in-time shift applied
}

// DerivProvider defines the interface for derivatives data providers
type DerivProvider interface {
	// GetFundingWindow retrieves funding rate history within time range
	GetFundingWindow(ctx context.Context, symbol string, tr TimeRange) ([]DerivMetrics, error)
	
	// GetLatest retrieves latest derivatives metrics for a symbol
	GetLatest(ctx context.Context, symbol string) (*DerivMetrics, error)
	
	// GetMultipleLatest retrieves latest metrics for multiple symbols
	GetMultipleLatest(ctx context.Context, symbols []string) (map[string]*DerivMetrics, error)
	
	// CalculateFundingZScore calculates z-score for funding rates using historical data
	CalculateFundingZScore(ctx context.Context, symbol string, lookbackPeriods int) (float64, error)
	
	// GetOpenInterestHistory retrieves OI history for trend analysis
	GetOpenInterestHistory(ctx context.Context, symbol string, tr TimeRange) ([]DerivMetrics, error)
	
	// Health returns provider health and connectivity status
	Health(ctx context.Context) (*ProviderHealth, error)
	
	// GetSupportedSymbols returns list of supported derivative symbols (USD pairs only)
	GetSupportedSymbols(ctx context.Context) ([]string, error)
}

// TimeRange represents a time window for data queries
type TimeRange struct {
	From time.Time `json:"from"`
	To   time.Time `json:"to"`
}

// ProviderHealth represents derivatives provider health status
type ProviderHealth struct {
	Healthy          bool              `json:"healthy"`
	Venue            string            `json:"venue"`
	LastUpdate       time.Time         `json:"last_update"`
	LatencyMS        float64           `json:"latency_ms"`
	ErrorRate        float64           `json:"error_rate"`
	SupportedSymbols int               `json:"supported_symbols"`
	DataFreshness    map[string]time.Duration `json:"data_freshness"`
	Errors           []string          `json:"errors,omitempty"`
}

// DerivProviderConfig holds common configuration for derivatives providers
type DerivProviderConfig struct {
	Venue            string        `json:"venue"`
	BaseURL          string        `json:"base_url"`
	RequestTimeout   time.Duration `json:"request_timeout"`
	RateLimitRPS     float64       `json:"rate_limit_rps"`
	MaxRetries       int           `json:"max_retries"`
	RetryBackoff     time.Duration `json:"retry_backoff"`
	PITShiftPeriods  int           `json:"pit_shift_periods"`   // PIT protection shift
	EnableMetrics    bool          `json:"enable_metrics"`
	UserAgent        string        `json:"user_agent"`
}

// DerivProviderFactory creates derivatives providers for different exchanges
type DerivProviderFactory interface {
	// CreateBinanceProvider creates Binance derivatives provider
	CreateBinanceProvider(config DerivProviderConfig) (DerivProvider, error)
	
	// CreateOKXProvider creates OKX derivatives provider
	CreateOKXProvider(config DerivProviderConfig) (DerivProvider, error)
	
	// CreateKrakenProvider creates Kraken derivatives provider (limited support)
	CreateKrakenProvider(config DerivProviderConfig) (DerivProvider, error)
	
	// GetAvailableProviders returns list of available exchange providers
	GetAvailableProviders() []string
}

// DerivAggregator combines data from multiple exchange-native providers
type DerivAggregator interface {
	// AggregateLatest combines latest metrics from multiple exchanges
	AggregateLatest(ctx context.Context, symbol string, venues []string) (*AggregatedDerivMetrics, error)
	
	// CalculateCrossVenueFundingZScore calculates funding z-score across venues
	CalculateCrossVenueFundingZScore(ctx context.Context, symbol string, venues []string) (float64, error)
	
	// GetConsensusMetrics provides consensus view across exchanges
	GetConsensusMetrics(ctx context.Context, symbol string, venues []string) (*ConsensusDerivMetrics, error)
	
	// ValidateConsistency checks for outliers across venues
	ValidateConsistency(ctx context.Context, metrics map[string]*DerivMetrics) (*ConsistencyReport, error)
}

// AggregatedDerivMetrics represents combined metrics from multiple venues
type AggregatedDerivMetrics struct {
	Symbol               string                    `json:"symbol"`
	Timestamp            time.Time                 `json:"timestamp"`
	VenueCount           int                       `json:"venue_count"`
	VenueMetrics         map[string]*DerivMetrics  `json:"venue_metrics"`
	
	// Aggregated Values
	WeightedFunding      float64                   `json:"weighted_funding"`      // Volume-weighted funding
	MedianFunding        float64                   `json:"median_funding"`        // Median funding across venues
	FundingSpread        float64                   `json:"funding_spread"`        // Max - Min funding
	
	TotalOpenInterest    float64                   `json:"total_open_interest"`   // Sum across venues
	WeightedBasis        float64                   `json:"weighted_basis"`        // Volume-weighted basis
	
	// Consensus Indicators
	FundingConsensus     float64                   `json:"funding_consensus"`     // 0.0-1.0 agreement score
	VenueAgreement       float64                   `json:"venue_agreement"`       // Overall agreement score
	OutlierVenues        []string                  `json:"outlier_venues"`        // Venues with outlier data
	
	DataQuality          float64                   `json:"data_quality"`          // Composite quality score
}

// ConsensusDerivMetrics represents consensus view across exchanges
type ConsensusDerivMetrics struct {
	Symbol                string    `json:"symbol"`
	Timestamp             time.Time `json:"timestamp"`
	
	// Consensus Values (outlier-adjusted)
	ConsensusFunding      float64   `json:"consensus_funding"`
	ConsensusOI           float64   `json:"consensus_oi"`
	ConsensusBasis        float64   `json:"consensus_basis"`
	
	// Confidence Metrics
	ConsensusConfidence   float64   `json:"consensus_confidence"`   // 0.0-1.0
	DataReliability       float64   `json:"data_reliability"`       // 0.0-1.0
	VenueParticipation    float64   `json:"venue_participation"`    // % of venues contributing
	
	// Outlier Analysis
	OutlierCount          int       `json:"outlier_count"`
	MaxDeviation          float64   `json:"max_deviation"`
	
	ContributingVenues    []string  `json:"contributing_venues"`
}

// ConsistencyReport analyzes data consistency across venues
type ConsistencyReport struct {
	Symbol               string                 `json:"symbol"`
	Timestamp            time.Time              `json:"timestamp"`
	VenueCount           int                    `json:"venue_count"`
	
	// Consistency Scores (0.0-1.0)
	FundingConsistency   float64                `json:"funding_consistency"`
	OIConsistency        float64                `json:"oi_consistency"`
	BasisConsistency     float64                `json:"basis_consistency"`
	OverallConsistency   float64                `json:"overall_consistency"`
	
	// Outlier Detection
	Outliers             map[string]OutlierInfo `json:"outliers"`
	OutlierThreshold     float64                `json:"outlier_threshold"`
	
	// Data Quality Flags
	InsufficientData     bool                   `json:"insufficient_data"`
	StaleDataDetected    bool                   `json:"stale_data_detected"`
	HighVarianceWarning  bool                   `json:"high_variance_warning"`
	
	Recommendations      []string               `json:"recommendations"`
}

// OutlierInfo describes an outlier venue and metric
type OutlierInfo struct {
	Venue        string  `json:"venue"`
	Metric       string  `json:"metric"`       // "funding", "oi", "basis"
	Value        float64 `json:"value"`
	Deviation    float64 `json:"deviation"`    // Standard deviations from mean
	Confidence   float64 `json:"confidence"`   // Outlier confidence 0.0-1.0
}

// ZScoreCalculator provides funding rate z-score calculations
type ZScoreCalculator interface {
	// CalculateZScore calculates z-score for current funding vs historical
	CalculateZScore(current float64, historical []float64) (float64, error)
	
	// CalculateRollingZScore calculates rolling z-score over time series
	CalculateRollingZScore(values []float64, windowSize int) ([]float64, error)
	
	// GetZScoreStats returns z-score statistics for analysis
	GetZScoreStats(zScores []float64) (*ZScoreStats, error)
}

// ZScoreStats provides statistical analysis of z-scores
type ZScoreStats struct {
	Mean           float64   `json:"mean"`
	StdDev         float64   `json:"std_dev"`
	Min            float64   `json:"min"`
	Max            float64   `json:"max"`
	Percentile95   float64   `json:"percentile_95"`
	Percentile05   float64   `json:"percentile_05"`
	ExtremeCounts  int       `json:"extreme_counts"`  // |z| > 2.0
	SampleSize     int       `json:"sample_size"`
}

// MetricsCallback for derivatives provider metrics collection
type MetricsCallback func(metric string, value float64, tags map[string]string)