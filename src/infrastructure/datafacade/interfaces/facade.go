package interfaces

import (
	"context"
	"errors"
	"time"
)

// DataFacade provides unified access to HOT (WebSocket) and WARM (REST) market data
type DataFacade interface {
	// HOT stream operations
	SubscribeToTrades(ctx context.Context, venue, symbol string) (<-chan TradeEvent, error)
	SubscribeToKlines(ctx context.Context, venue, symbol string, interval string) (<-chan KlineEvent, error)
	SubscribeToOrderBook(ctx context.Context, venue, symbol string, depth int) (<-chan OrderBookEvent, error)
	SubscribeToFunding(ctx context.Context, venue, symbol string) (<-chan FundingEvent, error)

	// WARM pull operations with TTL caching
	GetTrades(ctx context.Context, venue, symbol string, limit int) ([]Trade, error)
	GetKlines(ctx context.Context, venue, symbol string, interval string, limit int) ([]Kline, error)
	GetOrderBookSnapshot(ctx context.Context, venue, symbol string) (*OrderBookSnapshot, error)
	GetFundingRate(ctx context.Context, venue, symbol string) (*FundingRate, error)
	GetOpenInterest(ctx context.Context, venue, symbol string) (*OpenInterest, error)

	// PIT snapshot operations
	CreateSnapshot(ctx context.Context, snapshotID string) error
	GetSnapshotData(ctx context.Context, snapshotID, venue, symbol, dataType string) (interface{}, error)
	ListSnapshots(ctx context.Context, venue string) ([]SnapshotInfo, error)

	// Health and metrics
	GetVenueHealth(ctx context.Context, venue string) (*VenueHealth, error)
	GetMetrics(ctx context.Context) (*FacadeMetrics, error)
}

// VenueAdapter provides venue-specific data access
type VenueAdapter interface {
	GetVenue() string
	IsSupported(dataType DataType) bool
	
	// Hot stream methods
	StreamTrades(ctx context.Context, symbol string) (<-chan TradeEvent, error)
	StreamKlines(ctx context.Context, symbol string, interval string) (<-chan KlineEvent, error)
	StreamOrderBook(ctx context.Context, symbol string, depth int) (<-chan OrderBookEvent, error)
	StreamFunding(ctx context.Context, symbol string) (<-chan FundingEvent, error)

	// Warm pull methods
	FetchTrades(ctx context.Context, symbol string, limit int) ([]Trade, error)
	FetchKlines(ctx context.Context, symbol string, interval string, limit int) ([]Kline, error)
	FetchOrderBook(ctx context.Context, symbol string) (*OrderBookSnapshot, error)
	FetchFundingRate(ctx context.Context, symbol string) (*FundingRate, error)
	FetchOpenInterest(ctx context.Context, symbol string) (*OpenInterest, error)

	// Health check
	HealthCheck(ctx context.Context) error
}

// CacheLayer provides TTL-based caching for WARM data
type CacheLayer interface {
	Get(ctx context.Context, key string) ([]byte, bool, error)
	Set(ctx context.Context, key string, value []byte, ttl time.Duration) error
	Delete(ctx context.Context, key string) error
	Clear(ctx context.Context, pattern string) error
	
	// Cache stats
	GetStats(ctx context.Context) (*CacheStats, error)
	GetHitRate(ctx context.Context) float64
}

// PITStore provides point-in-time immutable snapshots
type PITStore interface {
	CreateSnapshot(ctx context.Context, snapshotID string, data map[string]interface{}) error
	GetSnapshot(ctx context.Context, snapshotID string) (map[string]interface{}, error)
	ListSnapshots(ctx context.Context, filter SnapshotFilter) ([]SnapshotInfo, error)
	DeleteSnapshot(ctx context.Context, snapshotID string) error
}

// RateLimiter handles provider-aware rate limiting
type RateLimiter interface {
	Allow(ctx context.Context, venue, endpoint string) error
	GetLimits(ctx context.Context, venue string) (*RateLimits, error)
	UpdateLimits(ctx context.Context, venue string, limits *RateLimits) error
	
	// Respect exchange headers
	ProcessRateLimitHeaders(venue string, headers map[string]string) error
}

// CircuitBreaker provides fault tolerance
type CircuitBreaker interface {
	Call(ctx context.Context, operation string, fn func() error) error
	GetState(ctx context.Context, operation string) (*CircuitState, error)
	ForceOpen(ctx context.Context, operation string) error
	ForceClose(ctx context.Context, operation string) error
}

// DataType represents the type of market data
type DataType string

const (
	DataTypeTrades       DataType = "trades"
	DataTypeKlines      DataType = "klines" 
	DataTypeOrderBook   DataType = "orderbook"
	DataTypeFunding     DataType = "funding"
	DataTypeOpenInterest DataType = "openinterest"
)

// Trade represents a single trade execution
type Trade struct {
	Symbol    string    `json:"symbol"`
	Venue     string    `json:"venue"`
	Price     float64   `json:"price"`
	Quantity  float64   `json:"quantity"`
	Side      string    `json:"side"` // "buy" or "sell"
	Timestamp time.Time `json:"timestamp"`
	TradeID   string    `json:"trade_id"`
}

// TradeEvent represents a streaming trade event
type TradeEvent struct {
	Trade
	EventTime   time.Time `json:"event_time"`
	IsMaker     bool      `json:"is_maker"`
	PITSnapshot string    `json:"pit_snapshot,omitempty"`
}

// Kline represents OHLCV candlestick data
type Kline struct {
	Symbol       string    `json:"symbol"`
	Venue        string    `json:"venue"`
	Interval     string    `json:"interval"`
	OpenTime     time.Time `json:"open_time"`
	CloseTime    time.Time `json:"close_time"`
	Open         float64   `json:"open"`
	High         float64   `json:"high"`
	Low          float64   `json:"low"`
	Close        float64   `json:"close"`
	Volume       float64   `json:"volume"`
	QuoteVolume  float64   `json:"quote_volume"`
	TradeCount   int64     `json:"trade_count"`
	TakerBuyBase float64   `json:"taker_buy_base"`
	TakerBuyQuote float64  `json:"taker_buy_quote"`
}

// KlineEvent represents a streaming kline event
type KlineEvent struct {
	Kline
	EventTime time.Time `json:"event_time"`
	IsClosed  bool      `json:"is_closed"`
}

// OrderBookSnapshot represents L1/L2 order book data
type OrderBookSnapshot struct {
	Symbol       string      `json:"symbol"`
	Venue        string      `json:"venue"`
	Timestamp    time.Time   `json:"timestamp"`
	Bids         []PriceLevel `json:"bids"`
	Asks         []PriceLevel `json:"asks"`
	LastUpdateID int64       `json:"last_update_id"`
	IsL2         bool        `json:"is_l2"`
}

// OrderBookEvent represents streaming order book updates
type OrderBookEvent struct {
	OrderBookSnapshot
	EventTime   time.Time `json:"event_time"`
	FirstUpdate int64     `json:"first_update_id"`
	FinalUpdate int64     `json:"final_update_id"`
}

// PriceLevel represents a single order book level
type PriceLevel struct {
	Price    float64 `json:"price"`
	Quantity float64 `json:"quantity"`
}

// FundingRate represents perpetual funding rate
type FundingRate struct {
	Symbol          string    `json:"symbol"`
	Venue           string    `json:"venue"`
	FundingRate     float64   `json:"funding_rate"`
	NextFundingTime time.Time `json:"next_funding_time"`
	Timestamp       time.Time `json:"timestamp"`
}

// FundingEvent represents streaming funding rate updates
type FundingEvent struct {
	FundingRate
	EventTime time.Time `json:"event_time"`
}

// OpenInterest represents derivatives open interest
type OpenInterest struct {
	Symbol        string    `json:"symbol"`
	Venue         string    `json:"venue"`
	OpenInterest  float64   `json:"open_interest"`
	NotionalValue float64   `json:"notional_value,omitempty"`
	Timestamp     time.Time `json:"timestamp"`
}

// SnapshotInfo contains metadata about a PIT snapshot
type SnapshotInfo struct {
	SnapshotID string                 `json:"snapshot_id"`
	Timestamp  time.Time             `json:"timestamp"`
	Venues     []string              `json:"venues"`
	Symbols    []string              `json:"symbols"`
	DataTypes  []DataType            `json:"data_types"`
	Size       int64                 `json:"size_bytes"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
}

// SnapshotFilter for querying snapshots
type SnapshotFilter struct {
	FromTime  *time.Time  `json:"from_time,omitempty"`
	ToTime    *time.Time  `json:"to_time,omitempty"`
	Venues    []string    `json:"venues,omitempty"`
	Symbols   []string    `json:"symbols,omitempty"`
	DataTypes []DataType  `json:"data_types,omitempty"`
	Limit     int         `json:"limit,omitempty"`
}

// VenueHealth represents venue operational status
type VenueHealth struct {
	Venue             string        `json:"venue"`
	IsHealthy         bool          `json:"is_healthy"`
	LastCheck         time.Time     `json:"last_check"`
	ResponseTime      time.Duration `json:"response_time"`
	ErrorRate         float64       `json:"error_rate"`
	RejectRate        float64       `json:"reject_rate"`
	WebSocketConnected bool         `json:"websocket_connected"`
	CircuitBreakerState string      `json:"circuit_breaker_state"`
	RateLimitUsage    float64       `json:"rate_limit_usage"`
}

// FacadeMetrics contains overall facade performance metrics
type FacadeMetrics struct {
	Timestamp         time.Time                    `json:"timestamp"`
	TotalRequests     int64                        `json:"total_requests"`
	SuccessfulRequests int64                       `json:"successful_requests"`
	FailedRequests    int64                        `json:"failed_requests"`
	CacheHitRate      float64                      `json:"cache_hit_rate"`
	AvgResponseTime   time.Duration                `json:"avg_response_time"`
	P99ResponseTime   time.Duration                `json:"p99_response_time"`
	VenueMetrics      map[string]*VenueMetrics     `json:"venue_metrics"`
	StreamMetrics     *StreamMetrics               `json:"stream_metrics"`
}

// VenueMetrics contains per-venue metrics
type VenueMetrics struct {
	Venue           string        `json:"venue"`
	Requests        int64         `json:"requests"`
	Successes       int64         `json:"successes"`
	Failures        int64         `json:"failures"`
	AvgLatency      time.Duration `json:"avg_latency"`
	RateLimitHits   int64         `json:"rate_limit_hits"`
	CircuitBreaks   int64         `json:"circuit_breaks"`
	LastSuccessTime time.Time     `json:"last_success_time"`
}

// StreamMetrics contains WebSocket streaming metrics
type StreamMetrics struct {
	ActiveConnections int64             `json:"active_connections"`
	TotalMessages     int64             `json:"total_messages"`
	MessagesByVenue   map[string]int64  `json:"messages_by_venue"`
	MessagesByType    map[string]int64  `json:"messages_by_type"`
	ReconnectCount    int64             `json:"reconnect_count"`
	LastReconnect     time.Time         `json:"last_reconnect"`
}

// CacheStats contains cache performance statistics
type CacheStats struct {
	Hits         int64         `json:"hits"`
	Misses       int64         `json:"misses"`
	Sets         int64         `json:"sets"`
	Deletes      int64         `json:"deletes"`
	HitRate      float64       `json:"hit_rate"`
	Size         int64         `json:"size_bytes"`
	ItemCount    int64         `json:"item_count"`
	AvgTTL       time.Duration `json:"avg_ttl"`
}

// RateLimits contains venue-specific rate limiting configuration
type RateLimits struct {
	Venue           string            `json:"venue"`
	RequestsPerMinute int             `json:"requests_per_minute"`
	RequestsPerSecond int             `json:"requests_per_second"`
	BurstAllowance   int             `json:"burst_allowance"`
	WeightLimits     map[string]int   `json:"weight_limits,omitempty"`
	MonthlyLimit     *int             `json:"monthly_limit,omitempty"`
	DailyLimit       *int             `json:"daily_limit,omitempty"`
	ResetTimes       map[string]time.Time `json:"reset_times,omitempty"`
}

// CircuitState represents circuit breaker state
type CircuitState struct {
	State           string    `json:"state"` // "closed", "open", "half_open"
	FailureCount    int       `json:"failure_count"`
	SuccessCount    int       `json:"success_count"`
	LastFailureTime time.Time `json:"last_failure_time"`
	NextRetryTime   time.Time `json:"next_retry_time,omitempty"`
	ErrorRate       float64   `json:"error_rate"`
}

// Error types
var (
	ErrVenueNotSupported    = errors.New("venue not supported")
	ErrSymbolNotSupported   = errors.New("symbol not supported") 
	ErrDataTypeNotSupported = errors.New("data type not supported")
	ErrRateLimitExceeded   = errors.New("rate limit exceeded")
	ErrCircuitBreakerOpen  = errors.New("circuit breaker is open")
	ErrCacheTimeout        = errors.New("cache operation timeout")
	ErrSnapshotNotFound    = errors.New("snapshot not found")
	ErrInvalidSnapshot     = errors.New("invalid snapshot format")
	ErrNotSupported        = errors.New("operation not supported")
)

// HealthStatus represents overall facade health
type HealthStatus struct {
	Timestamp time.Time                  `json:"timestamp"`
	Overall   string                     `json:"overall"`
	Venues    map[string]VenueHealth     `json:"venues"`
}