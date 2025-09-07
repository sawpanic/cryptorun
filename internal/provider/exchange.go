package provider

import (
	"context"
	"fmt"
	"time"
)

// ExchangeProvider defines the interface for exchange-native data providers
type ExchangeProvider interface {
	// Core identification
	GetName() string
	GetVenue() string
	GetSupportsDerivatives() bool
	
	// Market data operations (exchange-native only)
	GetOrderBook(ctx context.Context, symbol string) (*OrderBookData, error)
	GetTrades(ctx context.Context, symbol string, limit int) ([]TradeData, error)
	GetKlines(ctx context.Context, symbol string, interval string, limit int) ([]KlineData, error)
	
	// Derivatives data (if supported)
	GetFunding(ctx context.Context, symbol string) (*FundingData, error)
	GetOpenInterest(ctx context.Context, symbol string) (*OpenInterestData, error)
	
	// Health and status
	Health() ProviderHealth
	GetLimits() ProviderLimits
	
	// Lifecycle
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
}

// OrderBookData represents L1/L2 order book data (exchange-native only)
type OrderBookData struct {
	Venue     string    `json:"venue"`
	Symbol    string    `json:"symbol"`
	Timestamp time.Time `json:"timestamp"`
	
	// L1 data (required)
	BestBid       float64 `json:"best_bid"`
	BestAsk       float64 `json:"best_ask"`
	BestBidSize   float64 `json:"best_bid_size"`
	BestAskSize   float64 `json:"best_ask_size"`
	
	// Calculated fields
	MidPrice      float64 `json:"mid_price"`
	SpreadBps     float64 `json:"spread_bps"`
	
	// L2 data (optional but preferred for microstructure)
	Bids          []PriceLevel `json:"bids,omitempty"`
	Asks          []PriceLevel `json:"asks,omitempty"`
	
	// Exchange-native proof
	ProviderProof ExchangeProof `json:"provider_proof"`
}

// PriceLevel represents a single level in the order book
type PriceLevel struct {
	Price    float64 `json:"price"`
	Size     float64 `json:"size"`
	NumOrders int    `json:"num_orders,omitempty"`
}

// TradeData represents individual trade execution
type TradeData struct {
	Venue     string    `json:"venue"`
	Symbol    string    `json:"symbol"`
	Timestamp time.Time `json:"timestamp"`
	Price     float64   `json:"price"`
	Size      float64   `json:"size"`
	Side      string    `json:"side"` // "buy" or "sell"
	TradeID   string    `json:"trade_id"`
}

// KlineData represents OHLCV candle data
type KlineData struct {
	Venue      string    `json:"venue"`
	Symbol     string    `json:"symbol"`
	Interval   string    `json:"interval"`
	OpenTime   time.Time `json:"open_time"`
	CloseTime  time.Time `json:"close_time"`
	Open       float64   `json:"open"`
	High       float64   `json:"high"`
	Low        float64   `json:"low"`
	Close      float64   `json:"close"`
	Volume     float64   `json:"volume"`
	QuoteVolume float64  `json:"quote_volume,omitempty"`
	TradeCount int64     `json:"trade_count,omitempty"`
}

// FundingData represents perpetual funding information
type FundingData struct {
	Venue        string    `json:"venue"`
	Symbol       string    `json:"symbol"`
	Timestamp    time.Time `json:"timestamp"`
	FundingRate  float64   `json:"funding_rate"`
	NextFunding  time.Time `json:"next_funding"`
	MarkPrice    float64   `json:"mark_price"`
	IndexPrice   float64   `json:"index_price"`
}

// OpenInterestData represents derivatives open interest
type OpenInterestData struct {
	Venue        string    `json:"venue"`
	Symbol       string    `json:"symbol"`
	Timestamp    time.Time `json:"timestamp"`
	OpenInterest float64   `json:"open_interest"`
	USDValue     float64   `json:"usd_value,omitempty"`
}

// ExchangeProof provides proof that data came from exchange-native source
type ExchangeProof struct {
	SourceType    string            `json:"source_type"`    // "exchange_native"
	Provider      string            `json:"provider"`       // "binance", "kraken", etc.
	APIEndpoint   string            `json:"api_endpoint"`   // Actual endpoint used
	ResponseTime  time.Duration     `json:"response_time"`  // Latency measurement
	Headers       map[string]string `json:"headers,omitempty"` // Relevant response headers
	Checksum      string            `json:"checksum"`       // Data integrity hash
}

// ProviderHealth indicates provider operational status
type ProviderHealth struct {
	Healthy       bool              `json:"healthy"`
	Status        string            `json:"status"`
	Errors        []string          `json:"errors,omitempty"`
	LastCheck     time.Time         `json:"last_check"`
	ResponseTime  time.Duration     `json:"response_time"`
	CircuitState  string            `json:"circuit_state"`  // "closed", "open", "half-open"
	Metrics       ProviderMetrics   `json:"metrics"`
}

// ProviderMetrics provides operational metrics
type ProviderMetrics struct {
	RequestCount     int64   `json:"request_count"`
	ErrorCount       int64   `json:"error_count"`
	SuccessRate      float64 `json:"success_rate"`
	AvgResponseTime  float64 `json:"avg_response_time_ms"`
	RateLimitHits    int64   `json:"rate_limit_hits"`
	CircuitOpenTime  int64   `json:"circuit_open_time_seconds"`
	CacheHitRate     float64 `json:"cache_hit_rate"`
	DataFreshness    float64 `json:"data_freshness_seconds"`
}

// ProviderHealthMetrics provides health-specific metrics
type ProviderHealthMetrics struct {
	RequestCount  int64   `json:"request_count"`
	SuccessCount  int64   `json:"success_count"`
	FailureCount  int64   `json:"failure_count"`
	SuccessRate   float64 `json:"success_rate"`
}

// ProviderLimits defines operational constraints
type ProviderLimits struct {
	RequestsPerSecond int           `json:"requests_per_second"`
	BurstLimit        int           `json:"burst_limit"`
	DailyLimit        int           `json:"daily_limit,omitempty"`
	WeightLimit       int           `json:"weight_limit,omitempty"`
	Timeout           time.Duration `json:"timeout"`
	MaxRetries        int           `json:"max_retries"`
}

// ProviderConfig holds provider configuration
type ProviderConfig struct {
	Name              string            `json:"name"`
	Venue             string            `json:"venue"`
	BaseURL           string            `json:"base_url"`
	WebSocketURL      string            `json:"websocket_url,omitempty"`
	APIKey            string            `json:"api_key,omitempty"`
	APISecret         string            `json:"api_secret,omitempty"`
	Testnet           bool              `json:"testnet"`
	
	// Rate limiting
	RateLimit         ProviderLimits    `json:"rate_limit"`
	
	// Circuit breaker
	CircuitBreaker    CircuitConfig     `json:"circuit_breaker"`
	
	// Cache settings
	CacheConfig       CacheConfig       `json:"cache"`
	
	// Additional settings
	Extra             map[string]string `json:"extra,omitempty"`
}

// CircuitConfig defines circuit breaker behavior
type CircuitConfig struct {
	Enabled           bool          `json:"enabled"`
	FailureThreshold  float64       `json:"failure_threshold"`  // 0.0-1.0
	MinRequests       int           `json:"min_requests"`
	OpenTimeout       time.Duration `json:"open_timeout"`
	ProbeInterval     time.Duration `json:"probe_interval"`
	MaxFailures       int           `json:"max_failures"`
}

// CacheConfig defines caching behavior
type CacheConfig struct {
	Enabled           bool          `json:"enabled"`
	TTL               time.Duration `json:"ttl"`
	MaxEntries        int           `json:"max_entries"`
	StaleWhileRevalidate time.Duration `json:"stale_while_revalidate,omitempty"`
}

// ProviderRegistry manages multiple exchange providers
type ProviderRegistry interface {
	Register(provider ExchangeProvider) error
	Get(venue string) (ExchangeProvider, error)
	GetAll() []ExchangeProvider
	GetHealthy() []ExchangeProvider
	GetSupportsDerivatives() []ExchangeProvider
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
	Health() map[string]ProviderHealth
}

// ProviderError represents provider-specific errors
type ProviderError struct {
	Provider    string `json:"provider"`
	Code        string `json:"code"`
	Message     string `json:"message"`
	HTTPStatus  int    `json:"http_status,omitempty"`
	RateLimited bool   `json:"rate_limited"`
	Temporary   bool   `json:"temporary"`
	Cause       error  `json:"-"`
}

func (e *ProviderError) Error() string {
	return fmt.Sprintf("provider %s: %s (%s)", e.Provider, e.Message, e.Code)
}

func (e *ProviderError) Unwrap() error {
	return e.Cause
}

// Common error codes
const (
	ErrCodeRateLimit        = "RATE_LIMIT"
	ErrCodeCircuitOpen      = "CIRCUIT_OPEN"
	ErrCodeTimeout          = "TIMEOUT"
	ErrCodeInvalidSymbol    = "INVALID_SYMBOL"
	ErrCodeMaintenance      = "MAINTENANCE"
	ErrCodeAuthentication   = "AUTH_ERROR"
	ErrCodeInsufficientData = "INSUFFICIENT_DATA"
	ErrCodeAPIError         = "API_ERROR"
	ErrCodeNetworkError     = "NETWORK_ERROR"
	ErrCodeInvalidData      = "INVALID_DATA"
)

// CreateExchangeProof generates proof for exchange-native data
func CreateExchangeProof(provider, endpoint string, responseTime time.Duration, data interface{}) ExchangeProof {
	checksum := calculateDataChecksum(data)
	
	return ExchangeProof{
		SourceType:   "exchange_native",
		Provider:     provider,
		APIEndpoint:  endpoint,
		ResponseTime: responseTime,
		Headers: map[string]string{
			"proof_type": "exchange_native",
			"generated_at": time.Now().Format(time.RFC3339),
		},
		Checksum:     checksum,
	}
}

func calculateDataChecksum(data interface{}) string {
	// Simplified checksum - in real implementation would use proper hashing
	return fmt.Sprintf("checksum_%d", time.Now().UnixNano())
}