package facade

import (
	"context"
	"time"
)

// DataFacade provides a unified interface for accessing market data across exchanges
// with hot (WebSocket) and warm (REST+cache) tiers
type DataFacade interface {
	// Hot tier - WebSocket subscriptions for top pairs
	SubscribeTrades(ctx context.Context, venue string, symbol string, callback TradesCallback) error
	SubscribeBookL2(ctx context.Context, venue string, symbol string, callback BookL2Callback) error
	StreamKlines(ctx context.Context, venue string, symbol string, interval string, callback KlinesCallback) error

	// Warm tier - REST API with caching for remaining universe
	GetKlines(ctx context.Context, venue string, symbol string, interval string, limit int) ([]Kline, error)
	GetTrades(ctx context.Context, venue string, symbol string, limit int) ([]Trade, error)
	GetBookL2(ctx context.Context, venue string, symbol string) (*BookL2, error)

	// Attribution and health
	SourceAttribution(venue string) Attribution
	VenueHealth(venue string) HealthStatus
	CacheStats() CacheStats

	// Lifecycle
	Start(ctx context.Context) error
	Stop() error
}

// Normalized data structures
type Trade struct {
	Symbol    string    `json:"symbol"`
	Venue     string    `json:"venue"`
	Timestamp time.Time `json:"timestamp"`
	Price     float64   `json:"price"`
	Size      float64   `json:"size"`
	Side      string    `json:"side"` // "buy" or "sell"
	TradeID   string    `json:"trade_id"`
}

type BookL2 struct {
	Symbol    string        `json:"symbol"`
	Venue     string        `json:"venue"`
	Timestamp time.Time     `json:"timestamp"`
	Bids      []BookLevel   `json:"bids"`
	Asks      []BookLevel   `json:"asks"`
	Sequence  int64         `json:"sequence"`
}

type BookLevel struct {
	Price float64 `json:"price"`
	Size  float64 `json:"size"`
}

type Kline struct {
	Symbol    string    `json:"symbol"`
	Venue     string    `json:"venue"`
	Timestamp time.Time `json:"timestamp"`
	Interval  string    `json:"interval"`
	Open      float64   `json:"open"`
	High      float64   `json:"high"`
	Low       float64   `json:"low"`
	Close     float64   `json:"close"`
	Volume    float64   `json:"volume"`
	QuoteVol  float64   `json:"quote_volume"`
}

// Callbacks for streaming data
type TradesCallback func([]Trade) error
type BookL2Callback func(*BookL2) error
type KlinesCallback func([]Kline) error

// Attribution tracks data sources and freshness
type Attribution struct {
	Venue       string        `json:"venue"`
	LastUpdate  time.Time     `json:"last_update"`
	Sources     []string      `json:"sources"`
	CacheHits   int64         `json:"cache_hits"`
	CacheMisses int64         `json:"cache_misses"`
	Latency     time.Duration `json:"latency"`
}

// HealthStatus tracks venue connectivity and performance
type HealthStatus struct {
	Venue        string        `json:"venue"`
	Status       string        `json:"status"` // "healthy", "degraded", "offline"
	LastSeen     time.Time     `json:"last_seen"`
	ErrorRate    float64       `json:"error_rate"`
	P99Latency   time.Duration `json:"p99_latency"`
	WSConnected  bool          `json:"ws_connected"`
	RESTHealthy  bool          `json:"rest_healthy"`
	Recommendation string      `json:"recommendation,omitempty"`
}

// CacheStats provides cache performance metrics
type CacheStats struct {
	PricesHot    CacheTierStats `json:"prices_hot"`
	PricesWarm   CacheTierStats `json:"prices_warm"`
	VolumesVADR  CacheTierStats `json:"volumes_vadr"`
	TokenMeta    CacheTierStats `json:"token_metadata"`
	TotalEntries int64          `json:"total_entries"`
}

type CacheTierStats struct {
	TTL      time.Duration `json:"ttl"`
	Hits     int64         `json:"hits"`
	Misses   int64         `json:"misses"`
	Entries  int64         `json:"entries"`
	HitRatio float64       `json:"hit_ratio"`
}

// Configuration structures
type HotConfig struct {
	Venues       []string      `yaml:"venues"`
	MaxPairs     int           `yaml:"max_pairs"`
	ReconnectSec int           `yaml:"reconnect_sec"`
	BufferSize   int           `yaml:"buffer_size"`
	Timeout      time.Duration `yaml:"timeout"`
}

type WarmConfig struct {
	Venues       []string      `yaml:"venues"`
	DefaultTTL   time.Duration `yaml:"default_ttl"`
	MaxRetries   int           `yaml:"max_retries"`
	BackoffBase  time.Duration `yaml:"backoff_base"`
	RequestLimit int           `yaml:"request_limit"`
}

type CacheConfig struct {
	PricesHot   time.Duration `yaml:"prices_hot"`   // 5s
	PricesWarm  time.Duration `yaml:"prices_warm"`  // 30s
	VolumesVADR time.Duration `yaml:"volumes_vadr"` // 120s
	TokenMeta   time.Duration `yaml:"token_metadata"` // 24h
	MaxEntries  int64         `yaml:"max_entries"`
}

// Facade implementation
type Facade struct {
	hotConfig   HotConfig
	warmConfig  WarmConfig
	cacheConfig CacheConfig
	
	// Internal components (will be injected)
	cache      Cache
	rateLimit  RateLimiter
	pitStore   PITStore
	exchanges  map[string]Exchange
	
	// Metrics
	attribution map[string]*Attribution
	healthStats map[string]*HealthStatus
	cacheStats  *CacheStats
}

// Cache interface for TTL caching
type Cache interface {
	Get(key string) (interface{}, bool)
	Set(key string, value interface{}, ttl time.Duration)
	Stats() CacheStats
	Clear()
}

// RateLimiter interface for exchange-specific rate limiting
type RateLimiter interface {
	Allow(venue string) bool
	Wait(venue string) time.Duration
	UpdateBudget(venue string, remaining int64)
	Status(venue string) RateLimitStatus
}

type RateLimitStatus struct {
	Venue       string `json:"venue"`
	Remaining   int64  `json:"remaining"`
	ResetTime   time.Time `json:"reset_time"`
	Throttled   bool   `json:"throttled"`
	BackoffTime time.Duration `json:"backoff_time,omitempty"`
}

// PITStore interface for point-in-time snapshots
type PITStore interface {
	Snapshot(entity string, timestamp time.Time, payload interface{}, source string) error
	Read(entity string, timestamp time.Time) (interface{}, error)
	List(entity string, from time.Time, to time.Time) ([]PITEntry, error)
}

type PITEntry struct {
	Entity    string      `json:"entity"`
	Timestamp time.Time   `json:"timestamp"`
	Source    string      `json:"source"`
	Payload   interface{} `json:"payload"`
}

// Exchange interface for venue-specific implementations
type Exchange interface {
	Name() string
	
	// WebSocket streams
	ConnectWS(ctx context.Context) error
	SubscribeTrades(symbol string, callback TradesCallback) error
	SubscribeBookL2(symbol string, callback BookL2Callback) error
	StreamKlines(symbol string, interval string, callback KlinesCallback) error
	
	// REST API
	GetKlines(ctx context.Context, symbol string, interval string, limit int) ([]Kline, error)
	GetTrades(ctx context.Context, symbol string, limit int) ([]Trade, error)
	GetBookL2(ctx context.Context, symbol string) (*BookL2, error)
	
	// Normalization
	NormalizeSymbol(symbol string) string
	NormalizeInterval(interval string) string
	
	// Health
	Health() HealthStatus
}

// New creates a new data facade instance
func New(hotCfg HotConfig, warmCfg WarmConfig, cacheCfg CacheConfig, rl RateLimiter) *Facade {
	return &Facade{
		hotConfig:   hotCfg,
		warmConfig:  warmCfg,
		cacheConfig: cacheCfg,
		rateLimit:   rl,
		exchanges:   make(map[string]Exchange),
		attribution: make(map[string]*Attribution),
		healthStats: make(map[string]*HealthStatus),
		cacheStats:  &CacheStats{},
	}
}