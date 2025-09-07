package guards

import (
	"time"
)

// Regime types for guard configuration
type Regime string

const (
	RegimeTrending Regime = "trending"
	RegimeChoppy   Regime = "choppy"
	RegimeHighVol  Regime = "high_volatility"
	RegimeUnknown  Regime = "unknown"
)

// GuardConfig holds regime-aware guard thresholds
type GuardConfig struct {
	RegimeAware bool `yaml:"regime_aware"`

	Fatigue   FatigueConfig   `yaml:"fatigue"`
	LateFill  LateFillConfig  `yaml:"late_fill"`
	Freshness FreshnessConfig `yaml:"freshness"`
}

// FatigueConfig controls overextension protection
type FatigueConfig struct {
	Baseline        FatigueThresholds `yaml:"baseline"`
	TrendingProfile FatigueThresholds `yaml:"trending_profile"`
	MaxMomentum     float64           `yaml:"max_momentum_threshold"`
	MaxRSI          float64           `yaml:"max_rsi_threshold"`
}

type FatigueThresholds struct {
	Momentum24hThreshold float64 `yaml:"momentum_24h_threshold"`
	RSI4hThreshold       float64 `yaml:"rsi_4h_threshold"`
	AccelerationOverride float64 `yaml:"acceleration_override"`
	RequiresAccelRenewal bool    `yaml:"requires_accel_renewal"`
}

// LateFillConfig controls execution timing constraints
type LateFillConfig struct {
	Baseline           LateFillThresholds `yaml:"baseline"`
	TrendingProfile    LateFillThresholds `yaml:"trending_profile"`
	MaxDelaySecondsAbs int                `yaml:"max_delay_seconds_absolute"`
	MinDelaySeconds    int                `yaml:"min_delay_seconds"`
}

type LateFillThresholds struct {
	MaxDelaySeconds      int     `yaml:"max_delay_seconds"`
	RequiresInfraHealth  bool    `yaml:"requires_infra_health"`
	RequiresATRProximity bool    `yaml:"requires_atr_proximity"`
	ATRFactor            float64 `yaml:"atr_factor"`
}

// FreshnessConfig controls signal staleness protection
type FreshnessConfig struct {
	Baseline        FreshnessThresholds `yaml:"baseline"`
	TrendingProfile FreshnessThresholds `yaml:"trending_profile"`
	MaxBarsAgeAbs   int                 `yaml:"max_bars_age_absolute"`
	MinATRFactor    float64             `yaml:"min_atr_factor"`
}

type FreshnessThresholds struct {
	MaxBarsAge          int     `yaml:"max_bars_age"`
	ATRFactor           float64 `yaml:"atr_factor"`
	RequiresVADR        float64 `yaml:"requires_vadr"`
	RequiresTightSpread bool    `yaml:"requires_tight_spread"`
	SpreadThresholdBps  float64 `yaml:"spread_threshold_bps"`
}

// Guard evaluation inputs
type FatigueInputs struct {
	Symbol       string
	Momentum24h  float64
	RSI4h        float64
	Acceleration float64
	AccelRenewal bool // From acceleration detection
	Regime       Regime
}

type LateFillInputs struct {
	Symbol        string
	SignalTime    time.Time
	ExecutionTime time.Time
	InfraP99MS    float64 // Infrastructure P99 latency
	ATRDistance   float64 // Price distance from trigger in ATR units
	Regime        Regime
}

type FreshnessInputs struct {
	Symbol      string
	BarsAge     int
	PriceChange float64
	ATR1h       float64
	VADR        float64 // Volume-adjusted daily range
	SpreadBps   float64 // Spread in basis points
	Regime      Regime
}

// Guard evaluation results
type GuardResult struct {
	Allow   bool
	Reason  string
	Profile string // "baseline" or "trending"
	Regime  Regime
	Details map[string]interface{}
}

// Combined guard evaluation
type AllGuardsInputs struct {
	Fatigue   FatigueInputs
	LateFill  LateFillInputs
	Freshness FreshnessInputs
}

type AllGuardsResult struct {
	AllowEntry   bool
	BlockReason  string
	BlockedBy    string // Which guard blocked
	Profile      string // Which profile was used
	Regime       Regime
	GuardResults map[string]GuardResult
}
