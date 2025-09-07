package facade

import (
	"context"
	"fmt"
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
	
	// PostgreSQL persistence for PIT reads and dual-path storage
	repository Repository
	dbEnabled  bool
	
	// Metrics
	attribution map[string]*Attribution
	healthStats map[string]*HealthStatus
	cacheStats  *CacheStats
}

// Repository interface for PostgreSQL persistence integration
type Repository interface {
	// Trades persistence
	InsertTrade(ctx context.Context, trade Trade) error
	ReadTrades(ctx context.Context, symbol string, from time.Time, to time.Time, limit int) ([]Trade, error)
	
	// Regime snapshots
	UpsertRegime(ctx context.Context, snapshot RegimeSnapshot) error
	ReadRegimes(ctx context.Context, from time.Time, to time.Time) ([]RegimeSnapshot, error)
	
	// Premove artifacts
	UpsertArtifact(ctx context.Context, artifact PremoveArtifact) error
	ReadArtifacts(ctx context.Context, symbol string, from time.Time, to time.Time, limit int) ([]PremoveArtifact, error)
	
	// Health check
	Health(ctx context.Context) RepositoryHealth
}

// RepositoryHealth for database monitoring
type RepositoryHealth struct {
	Healthy        bool              `json:"healthy"`
	Errors         []string          `json:"errors,omitempty"`
	ConnectionPool map[string]int    `json:"connection_pool"`
	LastCheck      time.Time         `json:"last_check"`
	ResponseTimeMS int64             `json:"response_time_ms"`
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

// PITStore interface for point-in-time snapshots with PostgreSQL backend
type PITStore interface {
	Snapshot(entity string, timestamp time.Time, payload interface{}, source string) error
	Read(entity string, timestamp time.Time) (interface{}, error)
	List(entity string, from time.Time, to time.Time) ([]PITEntry, error)
	
	// PostgreSQL-backed queries for calibration and backtesting
	ReadTrades(ctx context.Context, symbol string, from time.Time, to time.Time) ([]Trade, error)
	ReadRegimes(ctx context.Context, from time.Time, to time.Time) ([]RegimeSnapshot, error)
	ReadArtifacts(ctx context.Context, symbol string, from time.Time, to time.Time) ([]PremoveArtifact, error)
}

type PITEntry struct {
	Entity    string      `json:"entity"`
	Timestamp time.Time   `json:"timestamp"`
	Source    string      `json:"source"`
	Payload   interface{} `json:"payload"`
}

// Persistence layer types for PIT queries (matching internal/persistence)
type RegimeSnapshot struct {
	Timestamp        time.Time             `json:"ts"`
	RealizedVol7d    float64               `json:"realized_vol_7d"`
	PctAbove20MA     float64               `json:"pct_above_20ma"`
	BreadthThrust    float64               `json:"breadth_thrust"`
	Regime           string                `json:"regime"`
	Weights          map[string]float64    `json:"weights"`
	ConfidenceScore  float64               `json:"confidence_score"`
	DetectionMethod  string                `json:"detection_method"`
	Metadata         map[string]interface{} `json:"metadata"`
	CreatedAt        time.Time             `json:"created_at"`
}

type PremoveArtifact struct {
	ID               int64                  `json:"id"`
	Timestamp        time.Time             `json:"ts"`
	Symbol           string                `json:"symbol"`
	Venue            string                `json:"venue"`
	GateScore        bool                  `json:"gate_score"`
	GateVADR         bool                  `json:"gate_vadr"`
	GateFunding      bool                  `json:"gate_funding"`
	GateMicrostructure bool                `json:"gate_microstructure"`
	GateFreshness    bool                  `json:"gate_freshness"`
	GateFatigue      bool                  `json:"gate_fatigue"`
	Score            *float64              `json:"score,omitempty"`
	MomentumCore     *float64              `json:"momentum_core,omitempty"`
	TechnicalResidual *float64             `json:"technical_residual,omitempty"`
	VolumeResidual   *float64              `json:"volume_residual,omitempty"`
	QualityResidual  *float64              `json:"quality_residual,omitempty"`
	SocialResidual   *float64              `json:"social_residual,omitempty"` // Capped at +10
	Factors          map[string]interface{} `json:"factors,omitempty"`
	Regime           *string               `json:"regime,omitempty"`
	ConfidenceScore  float64               `json:"confidence_score"`
	ProcessingLatencyMS *int               `json:"processing_latency_ms,omitempty"`
	CreatedAt        time.Time             `json:"created_at"`
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

// New creates a new data facade instance with optional PostgreSQL repository
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
		dbEnabled:   false, // Will be set via SetRepository
	}
}

// SetRepository enables PostgreSQL persistence for dual-path storage and PIT reads
func (f *Facade) SetRepository(repo Repository) {
	f.repository = repo
	f.dbEnabled = true
}

// PITReads provides point-in-time data access for calibration and backtesting
func (f *Facade) PITReads() PITReader {
	return &pitReader{
		repository: f.repository,
		dbEnabled:  f.dbEnabled,
	}
}

// PITReader provides point-in-time queries for historical analysis
type PITReader interface {
	Trades(ctx context.Context, symbol string, from time.Time, to time.Time) ([]Trade, error)
	Regimes(ctx context.Context, from time.Time, to time.Time) ([]RegimeSnapshot, error)
	Artifacts(ctx context.Context, symbol string, from time.Time, to time.Time) ([]PremoveArtifact, error)
}

// pitReader implements PITReader interface
type pitReader struct {
	repository Repository
	dbEnabled  bool
}

func (pr *pitReader) Trades(ctx context.Context, symbol string, from time.Time, to time.Time) ([]Trade, error) {
	if !pr.dbEnabled || pr.repository == nil {
		return nil, fmt.Errorf("database not enabled - use file artifacts for PIT reads")
	}
	return pr.repository.ReadTrades(ctx, symbol, from, to, 1000)
}

func (pr *pitReader) Regimes(ctx context.Context, from time.Time, to time.Time) ([]RegimeSnapshot, error) {
	if !pr.dbEnabled || pr.repository == nil {
		return nil, fmt.Errorf("database not enabled - use file artifacts for PIT reads")
	}
	return pr.repository.ReadRegimes(ctx, from, to)
}

func (pr *pitReader) Artifacts(ctx context.Context, symbol string, from time.Time, to time.Time) ([]PremoveArtifact, error) {
	if !pr.dbEnabled || pr.repository == nil {
		return nil, fmt.Errorf("database not enabled - use file artifacts for PIT reads")
	}
	return pr.repository.ReadArtifacts(ctx, symbol, from, to, 500)
}