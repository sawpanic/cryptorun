package march_aug

import (
	"time"
)

// BacktestPeriod defines the March-August 2025 backtest configuration
type BacktestPeriod struct {
	StartDate time.Time `json:"start_date"`
	EndDate   time.Time `json:"end_date"`
	Name      string    `json:"name"`
	Universe  []string  `json:"universe"`
}

// MarketData represents OHLCV data with venue-native sourcing
type MarketData struct {
	Symbol    string    `json:"symbol"`
	Timestamp time.Time `json:"timestamp"`
	Open      float64   `json:"open"`
	High      float64   `json:"high"`
	Low       float64   `json:"low"`
	Close     float64   `json:"close"`
	Volume    float64   `json:"volume"`
	Venue     string    `json:"venue"` // binance, kraken, coinbase
}

// FundingData represents perpetual funding rates with venue median
type FundingData struct {
	Symbol     string    `json:"symbol"`
	Timestamp  time.Time `json:"timestamp"`
	BinanceFR  float64   `json:"binance_fr"`
	OKXFR      float64   `json:"okx_fr"`
	BybitFR    float64   `json:"bybit_fr"`
	MedianFR   float64   `json:"median_fr"`
	Divergence float64   `json:"divergence"` // Standard deviations from median
}

// OpenInterestData represents venue-native OI data
type OpenInterestData struct {
	Symbol       string    `json:"symbol"`
	Timestamp    time.Time `json:"timestamp"`
	OpenInterest float64   `json:"open_interest"`
	OIChange24h  float64   `json:"oi_change_24h"`
	OIResidual   float64   `json:"oi_residual"` // OI change vs price movement
	Venue        string    `json:"venue"`
}

// ReservesData represents exchange reserves from Glassnode
type ReservesData struct {
	Symbol      string    `json:"symbol"`
	Timestamp   time.Time `json:"timestamp"`
	Reserves    float64   `json:"reserves"`     // BTC/ETH from Glassnode
	ReservesPct float64   `json:"reserves_pct"` // % change
	Available   bool      `json:"available"`    // false for alts if not robust
}

// CatalystData represents dated catalyst events with timing multipliers
type CatalystData struct {
	Symbol      string    `json:"symbol"`
	Timestamp   time.Time `json:"timestamp"`
	EventType   string    `json:"event_type"` // SEC, hard_fork, ETF_flow
	Description string    `json:"description"`
	Impact      float64   `json:"impact"`      // Base impact score
	TimingMult  float64   `json:"timing_mult"` // 0-4w: 1.2x, 4-8w: 1.0x, etc.
	HeatScore   float64   `json:"heat_score"`  // impact * timing_mult
}

// SocialData represents Fear & Greed and search metrics
type SocialData struct {
	Symbol       string    `json:"symbol"`
	Timestamp    time.Time `json:"timestamp"`
	FearGreed    float64   `json:"fear_greed"`    // 0-100 index
	SearchSpikes float64   `json:"search_spikes"` // Google Trends relative
	SocialScore  float64   `json:"social_score"`  // Combined social signal
}

// MomentumFactors represents protected momentum calculations
type MomentumFactors struct {
	Symbol      string    `json:"symbol"`
	Timestamp   time.Time `json:"timestamp"`
	Momentum1h  float64   `json:"momentum_1h"`  // Weight: 20%
	Momentum4h  float64   `json:"momentum_4h"`  // Weight: 35%
	Momentum12h float64   `json:"momentum_12h"` // Weight: 30%
	Momentum24h float64   `json:"momentum_24h"` // Weight: 15%
	Composite   float64   `json:"composite"`    // Weighted average (protected)
	Protected   bool      `json:"protected"`    // Always true - not orthogonalized
}

// SupplyDemandFactors represents supply/demand analysis
type SupplyDemandFactors struct {
	Symbol        string    `json:"symbol"`
	Timestamp     time.Time `json:"timestamp"`
	OIADV         float64   `json:"oi_adv"`          // OI/ADV ratio
	VADR          float64   `json:"vadr"`            // Volume-Adjusted Daily Range
	ReservesFlow  float64   `json:"reserves_flow"`   // Exchange reserves change
	FundingDiv    float64   `json:"funding_div"`     // Funding divergence (σ)
	SmartMoneyDiv float64   `json:"smart_money_div"` // funding≤0 & price hold & OI residual
	Composite     float64   `json:"composite"`
}

// RegimeData represents market regime detection
type RegimeData struct {
	Timestamp     time.Time `json:"timestamp"`
	BreadthThrust float64   `json:"breadth_thrust"`  // % advancing vs declining
	RealizedVol7d float64   `json:"realized_vol_7d"` // 7-day realized volatility
	AboveMA20Pct  float64   `json:"above_ma20_pct"`  // % symbols above 20MA
	Regime        string    `json:"regime"`          // trending_bull, choppy, high_vol
	RegimeNumeric float64   `json:"regime_numeric"`  // 0=choppy, 1=bull, 2=highvol
	Confidence    float64   `json:"confidence"`
}

// CompositeScores represents final scoring with protected momentum
type CompositeScores struct {
	Symbol        string             `json:"symbol"`
	Timestamp     time.Time          `json:"timestamp"`
	MomentumScore float64            `json:"momentum_score"` // Protected from orthogonalization
	SupplyDemand  float64            `json:"supply_demand"`  // Post-orthogonalization
	CatalystHeat  float64            `json:"catalyst_heat"`  // Post-orthogonalization
	SocialSignal  float64            `json:"social_signal"`  // Capped at +10, post-orth
	FinalScore    float64            `json:"final_score"`    // Regime-weighted composite
	Regime        string             `json:"regime"`
	Attribution   map[string]float64 `json:"attribution"` // Factor contributions
}

// EntryGates represents gate evaluation results
type EntryGates struct {
	Symbol          string             `json:"symbol"`
	Timestamp       time.Time          `json:"timestamp"`
	CompositeGate   bool               `json:"composite_gate"`    // ≥75
	MovementGate    bool               `json:"movement_gate"`     // ≥2.5% (4h) or 24h fallback
	VolumeSurgeGate bool               `json:"volume_surge_gate"` // ≥1.8× average
	LiquidityGate   bool               `json:"liquidity_gate"`    // ≥$500k 24h vol
	TrendGate       bool               `json:"trend_gate"`        // ADX≥25 OR Hurst>0.55
	FatigueGate     bool               `json:"fatigue_gate"`      // Block if 24h>+12% & RSI4h>70 unless accel>0
	FreshnessGate   bool               `json:"freshness_gate"`    // ≤2 bars & late-fill <30s
	OverallPass     bool               `json:"overall_pass"`      // All gates passed
	FailReasons     []string           `json:"fail_reasons"`
	GateScores      map[string]float64 `json:"gate_scores"` // Individual gate values
}

// BacktestSignal represents a generated trading signal
type BacktestSignal struct {
	Symbol      string             `json:"symbol"`
	Timestamp   time.Time          `json:"timestamp"`
	Score       float64            `json:"score"`
	Gates       EntryGates         `json:"gates"`
	Factors     CompositeScores    `json:"factors"`
	MarketData  MarketData         `json:"market_data"`
	SignalType  string             `json:"signal_type"` // entry, exit
	Confidence  float64            `json:"confidence"`
	Attribution map[string]float64 `json:"attribution"`
}

// BacktestResult represents outcome tracking for signals
type BacktestResult struct {
	Signal      BacktestSignal `json:"signal"`
	EntryPrice  float64        `json:"entry_price"`
	ExitPrice   float64        `json:"exit_price"`
	Return48h   float64        `json:"return_48h"` // Realized 48h return
	HoldingTime time.Duration  `json:"holding_time"`
	Outcome     string         `json:"outcome"` // win, loss, timeout
	PnLPct      float64        `json:"pnl_pct"`
	MaxDrawdown float64        `json:"max_drawdown"`
	HitTarget   bool           `json:"hit_target"`
	StoppedOut  bool           `json:"stopped_out"`
}

// DecileAnalysis represents score vs return decile breakdown
type DecileAnalysis struct {
	Decile        int     `json:"decile"`      // 1-10 (10 = highest scores)
	ScoreRange    string  `json:"score_range"` // "75.0-82.5"
	Count         int     `json:"count"`       // Number of signals
	AvgScore      float64 `json:"avg_score"`
	AvgReturn48h  float64 `json:"avg_return_48h"`
	WinRate       float64 `json:"win_rate"` // % positive returns
	MedianReturn  float64 `json:"median_return"`
	StdDev        float64 `json:"std_dev"`
	Sharpe        float64 `json:"sharpe"`
	MaxDrawdown   float64 `json:"max_drawdown"`
	LiftVsDecile1 float64 `json:"lift_vs_decile1"` // Performance lift vs lowest decile
}

// AttributionAnalysis represents factor contribution analysis
type AttributionAnalysis struct {
	Factor        string  `json:"factor"`      // momentum, supply_demand, catalyst, social
	AvgContrib    float64 `json:"avg_contrib"` // Average contribution to final score
	ContribStdDev float64 `json:"contrib_std_dev"`
	ReturnCorr    float64 `json:"return_corr"`    // Correlation with 48h returns
	SignalCount   int     `json:"signal_count"`   // Signals where factor was material
	PositiveRate  float64 `json:"positive_rate"`  // % of signals with positive contribution
	TopDecileAvg  float64 `json:"top_decile_avg"` // Avg contribution in top decile
}

// BacktestSummary represents overall backtest performance
type BacktestSummary struct {
	Period          BacktestPeriod             `json:"period"`
	TotalSignals    int                        `json:"total_signals"`
	PassedGates     int                        `json:"passed_gates"`
	GatePassRate    float64                    `json:"gate_pass_rate"`
	WinRate         float64                    `json:"win_rate"` // % profitable signals
	AvgReturn48h    float64                    `json:"avg_return_48h"`
	MedianReturn    float64                    `json:"median_return"`
	Sharpe          float64                    `json:"sharpe"`
	MaxDrawdown     float64                    `json:"max_drawdown"`
	FalsePositives  int                        `json:"false_positives"` // High score, negative return
	DecileStats     []DecileAnalysis           `json:"decile_stats"`
	Attribution     []AttributionAnalysis      `json:"attribution"`
	RegimeBreakdown map[string]BacktestSummary `json:"regime_breakdown"`
}

// DataSource defines data ingestion interface
type DataSource interface {
	GetMarketData(symbol string, start, end time.Time) ([]MarketData, error)
	GetFundingData(symbol string, start, end time.Time) ([]FundingData, error)
	GetOpenInterestData(symbol string, start, end time.Time) ([]OpenInterestData, error)
	GetReservesData(symbol string, start, end time.Time) ([]ReservesData, error)
	GetCatalystData(symbol string, start, end time.Time) ([]CatalystData, error)
	GetSocialData(symbol string, start, end time.Time) ([]SocialData, error)
}

// FactorCalculator defines factor computation interface
type FactorCalculator interface {
	CalculateMomentumFactors(data []MarketData) ([]MomentumFactors, error)
	CalculateSupplyDemandFactors(market []MarketData, funding []FundingData,
		oi []OpenInterestData, reserves []ReservesData) ([]SupplyDemandFactors, error)
	CalculateCompositeScores(momentum []MomentumFactors, supply []SupplyDemandFactors,
		catalyst []CatalystData, social []SocialData, regime []RegimeData) ([]CompositeScores, error)
}

// GateEvaluator defines gate checking interface
type GateEvaluator interface {
	EvaluateGates(scores CompositeScores, market MarketData) (EntryGates, error)
}

// BacktestEngine defines main backtesting interface
type BacktestEngine interface {
	RunBacktest(period BacktestPeriod, universe []string) (*BacktestSummary, error)
	GenerateDecileAnalysis(results []BacktestResult) ([]DecileAnalysis, error)
	GenerateAttributionAnalysis(results []BacktestResult) ([]AttributionAnalysis, error)
}
