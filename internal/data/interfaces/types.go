package interfaces

import (
	"context"
	"time"
)

// Data layer interfaces and types shared across packages

// Cache interfaces
type Cache interface {
	Get(key string) (interface{}, bool)
	Set(key string, value interface{}, ttl time.Duration)
	Stats() CacheStats
	Clear()
	Stop()
}

type CacheStats struct {
	PricesHot    CacheTierStats `json:"prices_hot"`
	PricesWarm   CacheTierStats `json:"prices_warm"`
	VolumesVADR  CacheTierStats `json:"volumes_vadr"`
	TokenMeta    CacheTierStats `json:"token_meta"`
	TotalEntries int64          `json:"total_entries"`
}

type CacheTierStats struct {
	TTL      time.Duration `json:"ttl"`
	Hits     int64         `json:"hits"`
	Misses   int64         `json:"misses"`
	Entries  int64         `json:"entries"`
	HitRatio float64       `json:"hit_ratio"`
}

// Exchange data types
type TradesCallback func([]Trade) error
type BookL2Callback func(*BookL2) error
type KlinesCallback func([]Kline) error

type Trade struct {
	Symbol    string    `json:"symbol"`
	Venue     string    `json:"venue"`
	Timestamp time.Time `json:"timestamp"`
	Price     float64   `json:"price"`
	Size      float64   `json:"size"`
	Side      string    `json:"side"`
	TradeID   string    `json:"trade_id"`
}

type BookLevel struct {
	Price float64 `json:"price"`
	Size  float64 `json:"size"`
}

type BookL2 struct {
	Symbol    string      `json:"symbol"`
	Venue     string      `json:"venue"`
	Timestamp time.Time   `json:"timestamp"`
	Bids      []BookLevel `json:"bids"`
	Asks      []BookLevel `json:"asks"`
	Sequence  int64       `json:"sequence"`
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
	QuoteVol  float64   `json:"quote_vol"`
}

type HealthStatus struct {
	Venue          string        `json:"venue"`
	Status         string        `json:"status"`
	LastSeen       time.Time     `json:"last_seen"`
	ErrorRate      float64       `json:"error_rate"`
	P99Latency     time.Duration `json:"p99_latency"`
	WSConnected    bool          `json:"ws_connected"`
	RESTHealthy    bool          `json:"rest_healthy"`
	Recommendation string        `json:"recommendation"`
}

// Exchange interface
type Exchange interface {
	Name() string
	ConnectWS(ctx context.Context) error
	SubscribeTrades(symbol string, callback TradesCallback) error
	SubscribeBookL2(symbol string, callback BookL2Callback) error
	StreamKlines(symbol string, interval string, callback KlinesCallback) error
	GetKlines(ctx context.Context, symbol string, interval string, limit int) ([]Kline, error)
	GetTrades(ctx context.Context, symbol string, limit int) ([]Trade, error)
	GetBookL2(ctx context.Context, symbol string) (*BookL2, error)
	NormalizeSymbol(symbol string) string
	NormalizeInterval(interval string) string
	Health() HealthStatus
}

// PIT Store interfaces
type PITStore interface {
	Snapshot(entity string, timestamp time.Time, payload interface{}, source string) error
	List(entity string, from time.Time, to time.Time) ([]PITEntry, error)
}

type PITEntry struct {
	Entity    string                 `json:"entity"`
	Timestamp time.Time              `json:"timestamp"`
	Source    string                 `json:"source"`
	Hash      string                 `json:"hash"`
	Payload   map[string]interface{} `json:"payload"`
}

// Rate limiter interface
type RateLimiter interface {
	Allow(venue string) bool
	Wait(venue string) time.Duration
	Status(venue string) RateLimitStatus
}

type RateLimitStatus struct {
	Venue            string        `json:"venue"`
	RequestsPerMin   int           `json:"requests_per_min"`
	WindowRemaining  time.Duration `json:"window_remaining"`
	RequestsInWindow int           `json:"requests_in_window"`
	Limit            int           `json:"limit"`
	ResetTime        time.Time     `json:"reset_time"`
	Throttled        bool          `json:"throttled"`
	Remaining        int64         `json:"remaining"`
	BackoffTime      time.Duration `json:"backoff_time"`
}

// Attribution for data source tracking
type Attribution struct {
	Venue       string    `json:"venue"`
	LastUpdate  time.Time `json:"last_update"`
	Sources     []string  `json:"sources"`
	CacheHits   int64     `json:"cache_hits"`
	CacheMisses int64     `json:"cache_misses"`
}