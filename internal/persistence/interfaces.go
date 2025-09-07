package persistence

import (
	"context"
	"time"
)

// TimeRange represents a time window for data queries with PIT integrity
type TimeRange struct {
	From time.Time `json:"from"`
	To   time.Time `json:"to"`
}

// Trade represents a single trade execution with exchange-native data
type Trade struct {
	ID        int64                  `json:"id" db:"id"`
	Timestamp time.Time             `json:"ts" db:"ts"`
	Symbol    string                `json:"symbol" db:"symbol"`
	Venue     string                `json:"venue" db:"venue"`
	Side      string                `json:"side" db:"side"`
	Price     float64               `json:"price" db:"price"`
	Qty       float64               `json:"qty" db:"qty"`
	OrderID   *string               `json:"order_id,omitempty" db:"order_id"`
	Attributes map[string]interface{} `json:"attributes" db:"attributes"`
	CreatedAt time.Time             `json:"created_at" db:"created_at"`
}

// RegimeSnapshot represents a 4h regime detection result with weight profile
type RegimeSnapshot struct {
	Timestamp        time.Time             `json:"ts" db:"ts"`
	RealizedVol7d    float64               `json:"realized_vol_7d" db:"realized_vol_7d"`
	PctAbove20MA     float64               `json:"pct_above_20ma" db:"pct_above_20ma"`
	BreadthThrust    float64               `json:"breadth_thrust" db:"breadth_thrust"`
	Regime           string                `json:"regime" db:"regime"`
	Weights          map[string]float64    `json:"weights" db:"weights"`
	ConfidenceScore  float64               `json:"confidence_score" db:"confidence_score"`
	DetectionMethod  string                `json:"detection_method" db:"detection_method"`
	Metadata         map[string]interface{} `json:"metadata" db:"metadata"`
	CreatedAt        time.Time             `json:"created_at" db:"created_at"`
}

// PremoveArtifact represents entry gate results and composite scoring data
type PremoveArtifact struct {
	ID               int64                  `json:"id" db:"id"`
	Timestamp        time.Time             `json:"ts" db:"ts"`
	Symbol           string                `json:"symbol" db:"symbol"`
	Venue            string                `json:"venue" db:"venue"`
	
	// Entry Gates (hard requirements)
	GateScore        bool                  `json:"gate_score" db:"gate_score"`
	GateVADR         bool                  `json:"gate_vadr" db:"gate_vadr"`
	GateFunding      bool                  `json:"gate_funding" db:"gate_funding"`
	GateMicrostructure bool                `json:"gate_microstructure" db:"gate_microstructure"`
	GateFreshness    bool                  `json:"gate_freshness" db:"gate_freshness"`
	GateFatigue      bool                  `json:"gate_fatigue" db:"gate_fatigue"`
	
	// Composite Scoring Results
	Score            *float64              `json:"score,omitempty" db:"score"`
	MomentumCore     *float64              `json:"momentum_core,omitempty" db:"momentum_core"`
	TechnicalResidual *float64             `json:"technical_residual,omitempty" db:"technical_residual"`
	VolumeResidual   *float64              `json:"volume_residual,omitempty" db:"volume_residual"`
	QualityResidual  *float64              `json:"quality_residual,omitempty" db:"quality_residual"`
	SocialResidual   *float64              `json:"social_residual,omitempty" db:"social_residual"` // Capped at +10
	
	// Attribution and Context
	Factors          map[string]interface{} `json:"factors,omitempty" db:"factors"`
	Regime           *string               `json:"regime,omitempty" db:"regime"`
	ConfidenceScore  float64               `json:"confidence_score" db:"confidence_score"`
	ProcessingLatencyMS *int               `json:"processing_latency_ms,omitempty" db:"processing_latency_ms"`
	CreatedAt        time.Time             `json:"created_at" db:"created_at"`
}

// TradesRepo provides trade data persistence with PIT integrity
type TradesRepo interface {
	// Insert adds a new trade record with timestamp validation
	Insert(ctx context.Context, trade Trade) error
	
	// InsertBatch adds multiple trades atomically for high-throughput scenarios
	InsertBatch(ctx context.Context, trades []Trade) error
	
	// ListBySymbol retrieves trades for a symbol within time range (PIT-ordered)
	ListBySymbol(ctx context.Context, symbol string, tr TimeRange, limit int) ([]Trade, error)
	
	// ListByVenue retrieves trades for a venue within time range
	ListByVenue(ctx context.Context, venue string, tr TimeRange, limit int) ([]Trade, error)
	
	// GetByOrderID finds trade by exchange order ID for reconciliation
	GetByOrderID(ctx context.Context, orderID string) (*Trade, error)
	
	// GetLatest returns most recent trades across all symbols/venues
	GetLatest(ctx context.Context, limit int) ([]Trade, error)
	
	// Count returns total trades in time range for statistics
	Count(ctx context.Context, tr TimeRange) (int64, error)
	
	// CountByVenue returns trade counts grouped by venue
	CountByVenue(ctx context.Context, tr TimeRange) (map[string]int64, error)
}

// RegimeRepo provides regime snapshot persistence with 4h cadence
type RegimeRepo interface {
	// Upsert inserts or updates regime snapshot for timestamp
	Upsert(ctx context.Context, snapshot RegimeSnapshot) error
	
	// Latest returns the most recent regime classification
	Latest(ctx context.Context) (*RegimeSnapshot, error)
	
	// GetByTimestamp retrieves specific regime snapshot
	GetByTimestamp(ctx context.Context, ts time.Time) (*RegimeSnapshot, error)
	
	// ListRange retrieves regime history within time window
	ListRange(ctx context.Context, tr TimeRange) ([]RegimeSnapshot, error)
	
	// ListByRegime retrieves all snapshots of a specific regime type
	ListByRegime(ctx context.Context, regime string, limit int) ([]RegimeSnapshot, error)
	
	// GetRegimeStats returns regime distribution statistics
	GetRegimeStats(ctx context.Context, tr TimeRange) (map[string]int64, error)
	
	// GetWeightsHistory returns weight evolution over time for analysis
	GetWeightsHistory(ctx context.Context, tr TimeRange) ([]RegimeSnapshot, error)
}

// PremoveRepo provides premove artifact persistence with scoring history
type PremoveRepo interface {
	// Upsert inserts or updates premove artifact (unique per ts/symbol/venue)
	Upsert(ctx context.Context, artifact PremoveArtifact) error
	
	// UpsertBatch processes multiple artifacts atomically
	UpsertBatch(ctx context.Context, artifacts []PremoveArtifact) error
	
	// Window retrieves artifacts within time range for backtesting
	Window(ctx context.Context, tr TimeRange) ([]PremoveArtifact, error)
	
	// ListBySymbol retrieves artifacts for specific symbol (PIT-ordered)
	ListBySymbol(ctx context.Context, symbol string, tr TimeRange, limit int) ([]PremoveArtifact, error)
	
	// ListPassed retrieves artifacts that passed all entry gates
	ListPassed(ctx context.Context, tr TimeRange, limit int) ([]PremoveArtifact, error)
	
	// ListByScore retrieves artifacts above score threshold
	ListByScore(ctx context.Context, minScore float64, tr TimeRange, limit int) ([]PremoveArtifact, error)
	
	// ListByRegime retrieves artifacts for specific market regime
	ListByRegime(ctx context.Context, regime string, tr TimeRange, limit int) ([]PremoveArtifact, error)
	
	// GetGateStats returns entry gate pass/fail statistics
	GetGateStats(ctx context.Context, tr TimeRange) (map[string]map[string]int64, error)
	
	// GetScoreDistribution returns score histogram for performance analysis
	GetScoreDistribution(ctx context.Context, tr TimeRange, buckets int) (map[string]int64, error)
	
	// GetLatencyStats returns processing latency percentiles
	GetLatencyStats(ctx context.Context, tr TimeRange) (map[string]float64, error)
}

// Repository aggregates all persistence interfaces
type Repository struct {
	Trades  TradesRepo
	Regimes RegimeRepo
	Premove PremoveRepo
}

// HealthCheck represents repository health status
type HealthCheck struct {
	Healthy        bool              `json:"healthy"`
	Errors         []string          `json:"errors,omitempty"`
	ConnectionPool map[string]int    `json:"connection_pool"`
	LastCheck      time.Time         `json:"last_check"`
	ResponseTimeMS int64             `json:"response_time_ms"`
}

// RepositoryHealth provides health monitoring for persistence layer
type RepositoryHealth interface {
	// Health returns current repository health status
	Health(ctx context.Context) HealthCheck
	
	// Ping tests basic connectivity to database
	Ping(ctx context.Context) error
	
	// Stats returns connection pool and query statistics
	Stats(ctx context.Context) map[string]interface{}
}