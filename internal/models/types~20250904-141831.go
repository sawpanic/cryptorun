package models

import (
	"time"
	"github.com/shopspring/decimal"
)

// DipOpportunity represents a potential dip buying opportunity
type DipOpportunity struct {
	Symbol       string          `json:"symbol"`
	PairCode     string          `json:"pair_code"`
	Price        decimal.Decimal `json:"price"`
	VolumeUSD    decimal.Decimal `json:"volume_usd"`
	Change24h    float64         `json:"change_24h"`
	Change7d     float64         `json:"change_7d"`
	Change30d    float64         `json:"change_30d"`
	RSI          float64         `json:"rsi"`
	QualityScore float64         `json:"quality_score"`
	
	// Crypto-native signals
	CVDData          CVDData          `json:"cvd_data"`
	LiquidationData  LiquidationData  `json:"liquidation_data"`
	StructureData    StructureData    `json:"structure_data"`
	DerivativesData  DerivativesData  `json:"derivatives_data"`
	
	// Analysis results
	GatesPassed   []string  `json:"gates_passed"`
	ProReasons    []string  `json:"pro_reasons"`
	EntryTargets  Targets   `json:"entry_targets"`
	Timestamp     time.Time `json:"timestamp"`
}

// CVDData represents Cumulative Volume Delta analysis
type CVDData struct {
	CVDValue      decimal.Decimal `json:"cvd_value"`
	CVDZScore     float64         `json:"cvd_zscore"`
	IsAbsorption  bool            `json:"is_absorption"`
	AbsorptionStr float64         `json:"absorption_strength"`
}

// LiquidationData represents liquidation sweep analysis
type LiquidationData struct {
	HasSweep        bool            `json:"has_sweep"`
	SweepLow        decimal.Decimal `json:"sweep_low"`
	ReclaimPrice    decimal.Decimal `json:"reclaim_price"`
	ReclaimConfirmed bool           `json:"reclaim_confirmed"`
	SweepStrength   float64         `json:"sweep_strength"`
	// Missing fields identified in QA
	SweepLevel      float64         `json:"sweep_level"`
	Reclaim         bool            `json:"reclaim"`
}

// StructureData represents key structural levels
type StructureData struct {
	AVWAP           decimal.Decimal `json:"avwap"`
	AVWAPDistance   float64         `json:"avwap_distance"`
	POC             decimal.Decimal `json:"poc"`
	POCDistance     float64         `json:"poc_distance"`
	NearSupport     bool            `json:"near_support"`
	SupportLevel    decimal.Decimal `json:"support_level"`
	ResistanceLevel decimal.Decimal `json:"resistance_level"`
}

// DerivativesData represents futures/options metrics
type DerivativesData struct {
	FundingRate       float64         `json:"funding_rate"`
	OIChange          float64         `json:"oi_change"`
	OptionSkew        float64         `json:"option_skew"`
	LastUpdated       time.Time       `json:"last_updated"`
	// Missing fields identified in QA
	LeverageRatio     float64         `json:"leverage_ratio"`
	LiquidationVol    decimal.Decimal `json:"liquidation_vol"`
	OpenInterest      decimal.Decimal `json:"open_interest"`
}

// Targets represents entry, stop, and profit targets
type Targets struct {
	Entry       decimal.Decimal `json:"entry"`
	StopLoss    decimal.Decimal `json:"stop_loss"`
	TakeProfit  decimal.Decimal `json:"take_profit"`
	RiskReward  float64         `json:"risk_reward"`
}

// PaperTrade represents a paper trading position
type PaperTrade struct {
	ID           string          `json:"id"`
	Symbol       string          `json:"symbol"`
	PairCode     string          `json:"pair_code"`
	EntryPrice   decimal.Decimal `json:"entry_price"`
	CurrentPrice decimal.Decimal `json:"current_price"`
	ExitPrice    decimal.Decimal `json:"exit_price,omitempty"`
	
	QualityScore float64   `json:"quality_score"`
	EntryTime    time.Time `json:"entry_time"`
	ExitTime     time.Time `json:"exit_time,omitempty"`
	
	Status       string  `json:"status"` // OPEN, WIN, LOSS, TIME
	PnLPercent   float64 `json:"pnl_percent"`
	MaxGain      float64 `json:"max_gain"`
	MaxLoss      float64 `json:"max_loss"`
	HoursHeld    int     `json:"hours_held"`
	
	Targets      Targets   `json:"targets"`
	ExitReason   string    `json:"exit_reason,omitempty"`
}

// BacktestResult represents comprehensive backtest analysis
type BacktestResult struct {
	Period        Period              `json:"period"`
	Overview      BacktestOverview    `json:"overview"`
	Performance   PerformanceMetrics  `json:"performance"`
	Breakdown     DimensionalBreakdown `json:"breakdown"`
	MarketRegime  string              `json:"market_regime"`
	Trades        []PaperTrade        `json:"trades"`
	Recommendations []string          `json:"recommendations"`
}

// Period represents the backtest time period
type Period struct {
	StartDate time.Time `json:"start_date"`
	EndDate   time.Time `json:"end_date"`
	Days      int       `json:"days"`
}

// BacktestOverview represents high-level backtest results
type BacktestOverview struct {
	TotalDipsFound   int `json:"total_dips_found"`
	TradesExecuted   int `json:"trades_executed"`
	PairsAnalyzed    int `json:"pairs_analyzed"`
	FiltersApplied   int `json:"filters_applied"`
}

// PerformanceMetrics represents trading performance
type PerformanceMetrics struct {
	WinRate        float64 `json:"win_rate"`
	AvgReturn      float64 `json:"avg_return"`
	AvgWin         float64 `json:"avg_win"`
	AvgLoss        float64 `json:"avg_loss"`
	ProfitFactor   float64 `json:"profit_factor"`
	MaxDrawdown    float64 `json:"max_drawdown"`
	TotalReturn    float64 `json:"total_return"`
	SharpeRatio    float64 `json:"sharpe_ratio"`
	TotalTrades    int     `json:"total_trades"`  // Total number of trades
}

// DimensionalBreakdown represents performance by different factors
type DimensionalBreakdown struct {
	ByRSI      map[string]PerformanceMetrics `json:"by_rsi"`
	ByDrop     map[string]PerformanceMetrics `json:"by_drop"`
	ByVolume   map[string]PerformanceMetrics `json:"by_volume"`
	ByGates    map[string]PerformanceMetrics `json:"by_gates"`
	ByTimeHeld map[string]PerformanceMetrics `json:"by_time_held"`
}

// Configuration structs
type ScannerConfig struct {
	MinDrop24h      float64 `json:"min_drop_24h"`
	MaxDrop24h      float64 `json:"max_drop_24h"`
	MaxRSI          float64 `json:"max_rsi"`
	MinQuality      float64 `json:"min_quality"`
	StopLoss        float64 `json:"stop_loss"`
	TakeProfit      float64 `json:"take_profit"`
	MaxHoldHours    int     `json:"max_hold_hours"`
	MinVolume       int64   `json:"min_volume"`
	Max7dGain       float64 `json:"max_7d_gain"`
	RequiredGates   int     `json:"required_gates"`
}

type BacktestConfig struct {
	Days         int     `json:"days"`
	MinVolume    int64   `json:"min_volume"`
	MaxPairs     int     `json:"max_pairs"`
	Commission   float64 `json:"commission"`
}

type PaperConfig struct {
	ScanInterval   time.Duration `json:"scan_interval"`
	MaxPositions   int           `json:"max_positions"`
	AutoTrade      bool          `json:"auto_trade"`
	NotifyResults  bool          `json:"notify_results"`
}

// MomentumCandidate represents a momentum buying opportunity
type MomentumCandidate struct {
	Symbol           string  `json:"symbol"`
	PairCode         string  `json:"pair_code"`
	Price            float64 `json:"price"`
	VolumeUSD        float64 `json:"volume_usd"`
	Change24h        float64 `json:"change_24h"`
	Change7d         float64 `json:"change_7d"`
	RSI              float64 `json:"rsi"`
	QualityScore     float64 `json:"quality_score"`
	MomentumScore    float64 `json:"momentum_score"`
	
	// Momentum-specific signals
	OrderFlowScore   float64 `json:"order_flow_score"`
	TimeframeAlign   float64 `json:"timeframe_align"`
	VolumeProfile    float64 `json:"volume_profile"`
	LiquidityGrab    float64 `json:"liquidity_grab"`
	SmartMoneyDiv    float64 `json:"smart_money_div"`
	RelativeStrength float64 `json:"relative_strength"`
	DeltaVADR        float64 `json:"delta_vadr"`
	SqueezeScore     float64 `json:"squeeze_score"`
	BreakoutProx     float64 `json:"breakout_prox"`
	PAQScore         float64 `json:"paq_score"`
	
	// Analysis results
	Reasons      []string    `json:"reasons"`
	EntryTargets EntryTargets `json:"entry_targets"`
	Timestamp    time.Time   `json:"timestamp"`
}

// EntryTargets represents momentum entry and exit targets
type EntryTargets struct {
	Entry      float64 `json:"entry"`
	StopLoss   float64 `json:"stop_loss"`
	TakeProfit float64 `json:"take_profit"`
	RiskReward float64 `json:"risk_reward"`
}

// OHLCData represents OHLC market data
type OHLCData struct {
	Open   []float64   `json:"open"`
	High   []float64   `json:"high"`
	Low    []float64   `json:"low"`
	Close  []float64   `json:"close"`
	Volume []float64   `json:"volume"`
	Time   []time.Time `json:"time"`
}

// Momentum Scanner Types

// MomentumSignal represents a momentum trading opportunity
type MomentumSignal struct {
	Token          TokenInfo        `json:"token"`
	SafetyScore    SafetyScore      `json:"safety_score"`
	KOLActivity    []WalletActivity `json:"kol_activity"`
	MomentumScore  float64          `json:"momentum_score"`
	VelocityScore  float64          `json:"velocity_score"`
	SocialScore    float64          `json:"social_score"`
	Timestamp      time.Time        `json:"timestamp"`
	Venue          string           `json:"venue"` // pump.fun, raydium, etc
}

// TokenInfo represents basic token information
type TokenInfo struct {
	Address     string    `json:"address"`
	Symbol      string    `json:"symbol"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Image       string    `json:"image"`
	Website     string    `json:"website"`
	Twitter     string    `json:"twitter"`
	Telegram    string    `json:"telegram"`
	CreatedAt   time.Time `json:"created_at"`
	MarketCap   float64   `json:"market_cap"`
	Volume24h   float64   `json:"volume_24h"`
	Price       float64   `json:"price"`
}

// SafetyScore represents a comprehensive safety analysis (0-100)
type SafetyScore struct {
	Total               int      `json:"total"`
	HolderDistribution int      `json:"holder_distribution"`  // 40 points max
	LiquidityScore     int      `json:"liquidity_score"`     // 30 points max
	CreatorHistory     int      `json:"creator_history"`     // 20 points max
	MetadataQuality    int      `json:"metadata_quality"`    // 10 points max
	Warnings           []string `json:"warnings"`
	RiskLevel          string   `json:"risk_level"` // LOW, MEDIUM, HIGH, CRITICAL
}

// WalletActivity represents KOL wallet trading activity
type WalletActivity struct {
	WalletAddress string    `json:"wallet_address"`
	WalletName    string    `json:"wallet_name"`
	Action        string    `json:"action"` // BUY, SELL
	TokenAddress  string    `json:"token_address"`
	Amount        float64   `json:"amount"`
	Price         float64   `json:"price"`
	Timestamp     time.Time `json:"timestamp"`
	Signature     string    `json:"signature"`
}

// KOLProfile represents a Key Opinion Leader profile
type KOLProfile struct {
	WalletAddress string    `json:"wallet_address"`
	Name          string    `json:"name"`
	Description   string    `json:"description"`
	TwitterHandle string    `json:"twitter_handle"`
	Reputation    float64   `json:"reputation"`     // 0-100 based on success rate
	TotalTrades   int       `json:"total_trades"`
	WinRate       float64   `json:"win_rate"`
	AvgReturn     float64   `json:"avg_return"`
	LastActive    time.Time `json:"last_active"`
	IsActive      bool      `json:"is_active"`
}

// Transaction represents a Solana transaction
type Transaction struct {
	Signature string    `json:"signature"`
	Slot      int64     `json:"slot"`
	BlockTime time.Time `json:"block_time"`
	Success   bool      `json:"success"`
	TokenMint string    `json:"token_mint,omitempty"`
	Amount    float64   `json:"amount,omitempty"`
	Type      string    `json:"type"` // SWAP, TRANSFER, etc
}

// TokenHolder represents a token holder
type TokenHolder struct {
	Address string `json:"address"`
	Amount  string `json:"amount"`
	Percent float64 `json:"percent"`
}

// MomentumConfig represents momentum scanner configuration
type MomentumConfig struct {
	SolanaRPC         string        `json:"solana_rpc"`
	PumpFunAPI        string        `json:"pump_fun_api"`
	MinSafetyScore    int           `json:"min_safety_score"`
	MaxPositionSize   float64       `json:"max_position_size"`
	StopLossPercent   float64       `json:"stop_loss_percent"`
	TakeProfitLevels  []float64     `json:"take_profit_levels"`
	KOLWallets        []string      `json:"kol_wallets"`
	ScanInterval      time.Duration `json:"scan_interval"`
	EnableAutoTrading bool          `json:"enable_auto_trading"`
	EnableAlerts      bool          `json:"enable_alerts"`
	DiscordWebhook    string        `json:"discord_webhook"`
	TelegramBot       string        `json:"telegram_bot"`
	TelegramChannel   string        `json:"telegram_channel"`
}

// MomentumResult represents the result of momentum analysis
type MomentumResult struct {
	Signals     []MomentumSignal `json:"signals"`
	KOLActivity []WalletActivity `json:"kol_activity"`
	TopTokens   []TokenInfo      `json:"top_tokens"`
	Stats       MomentumStats    `json:"stats"`
}

// MomentumStats represents momentum scanner statistics
type MomentumStats struct {
	TokensScanned    int     `json:"tokens_scanned"`
	SignalsFound     int     `json:"signals_found"`
	KOLWallets       int     `json:"kol_wallets"`
	ActiveSignals    int     `json:"active_signals"`
	AvgSafetyScore   float64 `json:"avg_safety_score"`
	TopPerformer     string  `json:"top_performer"`
	ScanDuration     string  `json:"scan_duration"`
}

// Hybrid Trading System Types

// HybridOpportunity represents a multi-signal trading opportunity
type HybridOpportunity struct {
	Symbol           string          `json:"symbol"`
	PairCode         string          `json:"pair_code"`
	Price            decimal.Decimal `json:"price"`
	VolumeUSD        decimal.Decimal `json:"volume_usd"`
	Change24h        float64         `json:"change_24h"`
	
	// Multi-signal scores (0-100 each)
	DipScore         float64         `json:"dip_score"`
	MomentumScore    float64         `json:"momentum_score"`
	FearGreedScore   float64         `json:"fear_greed_score"`
	CompositeScore   float64         `json:"composite_score"`
	
	// Additional fields for compatibility
	OpportunityType  string          `json:"opportunity_type"` // "DIP", "MOMENTUM", "BALANCED"
	PriceChange24h   float64         `json:"price_change_24h"` // Alias for Change24h
	
	// Signal details
	DipData          *DipOpportunity  `json:"dip_data,omitempty"`
	MomentumData     *MomentumCandidate `json:"momentum_data,omitempty"`
	SentimentData    SentimentData    `json:"sentiment_data"`
	RegimeData       MarketRegimeData `json:"regime_data"`
	
	// Hybrid analysis
	SignalType       string          `json:"signal_type"` // DIP_PRIMARY, MOMENTUM_PRIMARY, BALANCED
	ConfidenceLevel  string          `json:"confidence_level"` // LOW, MEDIUM, HIGH, EXTREME
	RecommendedSize  float64         `json:"recommended_size"` // 0.0-1.0 of max position
	Reasons          []string        `json:"reasons"`
	Warnings         []string        `json:"warnings"`
	
	EntryTargets     Targets         `json:"entry_targets"`
	Timestamp        time.Time       `json:"timestamp"`
}

// SentimentData represents fear & greed composite analysis
type SentimentData struct {
	CompositeScore   float64         `json:"composite_score"` // 0-100
	FearGreedIndex   float64         `json:"fear_greed_index"`
	SocialSentiment  float64         `json:"social_sentiment"`
	OptionsSkew      float64         `json:"options_skew"`
	FundingRates     float64         `json:"funding_rates"`
	OnChainFlow      float64         `json:"onchain_flow"`
	VIXEquivalent    float64         `json:"vix_equivalent"`
	
	Interpretation   string          `json:"interpretation"` // EXTREME_FEAR, FEAR, NEUTRAL, GREED, EXTREME_GREED
	Divergences      []string        `json:"divergences"`
	LastUpdated      time.Time       `json:"last_updated"`
}

// MarketRegimeData represents current market regime analysis
type MarketRegimeData struct {
	CurrentRegime    string          `json:"current_regime"` // BULL, BEAR, NEUTRAL, TRANSITION
	BTCTrend         string          `json:"btc_trend"` // STRONG_UP, UP, SIDEWAYS, DOWN, STRONG_DOWN
	RegimeStrength   float64         `json:"regime_strength"` // 0-100
	RegimeConfidence float64         `json:"regime_confidence"` // 0-100
	
	BTCDominance     float64         `json:"btc_dominance"`
	AltSeasonIndex   float64         `json:"alt_season_index"`
	StablecoinFlow   float64         `json:"stablecoin_flow"`
	ExchangeReserves float64         `json:"exchange_reserves"`
	
	RecommendedStrategy string       `json:"recommended_strategy"` // DIP_FOCUS, MOMENTUM_FOCUS, BALANCED, CASH
	RiskMultiplier   float64         `json:"risk_multiplier"` // 0.0-2.0
	LastUpdated      time.Time       `json:"last_updated"`
}

// HybridConfig represents hybrid scanner configuration
type HybridConfig struct {
	// Signal Weights (should sum to 1.0)
	DipWeight          float64 `json:"dip_weight"`
	MomentumWeight     float64 `json:"momentum_weight"`
	SentimentWeight    float64 `json:"sentiment_weight"`
	VolumeWeight       float64 `json:"volume_weight"`
	
	// Thresholds
	MinCompositeScore  float64 `json:"min_composite_score"`
	MinConfidenceLevel string  `json:"min_confidence_level"`
	
	// Risk Management
	MaxPositions       int     `json:"max_positions"`
	MaxSectorExposure  float64 `json:"max_sector_exposure"`
	MaxCorrelation     float64 `json:"max_correlation"`
	
	// Data Sources
	EnableSentimentAPI bool    `json:"enable_sentiment_api"`
	EnableOnChainData  bool    `json:"enable_onchain_data"`
	EnableSocialData   bool    `json:"enable_social_data"`
	
	// Update Intervals
	FastUpdateInterval time.Duration `json:"fast_update_interval"` // Price, volume
	SlowUpdateInterval time.Duration `json:"slow_update_interval"` // Sentiment, regime
}

// SignalBreakdown represents detailed signal analysis
type SignalBreakdown struct {
	DipSignals       map[string]float64 `json:"dip_signals"`
	MomentumSignals  map[string]float64 `json:"momentum_signals"`
	SentimentSignals map[string]float64 `json:"sentiment_signals"`
	WeightedScores   map[string]float64 `json:"weighted_scores"`
}

// Combined Analysis System Types

// CombinedSignal represents an advanced multi-strategy signal
type CombinedSignal struct {
	Symbol           string          `json:"symbol"`
	Strategy         string          `json:"strategy"` // momentum, dip, fear_greed
	Direction        string          `json:"direction"` // long, short
	Score            float64         `json:"score"`     // 0-100
	CompositeScore   float64         `json:"composite_score"`
	EntryPrice       decimal.Decimal `json:"entry_price"`
	StopLoss         decimal.Decimal `json:"stop_loss"`
	TakeProfit       []decimal.Decimal `json:"take_profit"` // Multiple targets
	PositionSize     float64         `json:"position_size"` // 0.0-1.0
	Timeframe        string          `json:"timeframe"`
	Triggers         []string        `json:"triggers"`
	Reasoning        string          `json:"reasoning"`
	RiskWarnings     []string        `json:"risk_warnings"`
	Metadata         map[string]interface{} `json:"metadata"`
	ConfidenceLevel  string          `json:"confidence_level"`
	Timestamp        time.Time       `json:"timestamp"`
	
	// Additional fields for compatibility
	FinalScore       float64         `json:"final_score"`       // Alias for CompositeScore
	SignalType       string          `json:"signal_type"`       // Alias for Strategy
	PriceChange24h   float64         `json:"price_change_24h"`  // 24h price change
}

// Position represents a portfolio position
type Position struct {
	Symbol       string          `json:"symbol"`
	Size         float64         `json:"size"`         // % of portfolio
	EntryPrice   decimal.Decimal `json:"entry_price"`
	CurrentPrice decimal.Decimal `json:"current_price"`
	PnL          float64         `json:"pnl"`
	EntryTime    time.Time       `json:"entry_time"`
	Strategy     string          `json:"strategy"`
	Sector       string          `json:"sector"`
	Status       string          `json:"status"` // OPEN, CLOSED, STOPPED
}

// RiskReport represents comprehensive portfolio risk assessment
type RiskReport struct {
	Timestamp       time.Time `json:"timestamp"`
	TotalExposure   float64   `json:"total_exposure"`
	RiskScore       float64   `json:"risk_score"`
	MaxDrawdown     float64   `json:"max_drawdown"`
	Positions       int       `json:"positions"`
	SectorCount     int       `json:"sector_count"`
	DailyTrades     int       `json:"daily_trades"`
	Recommendations []string  `json:"recommendations"`
	Alerts          []string  `json:"alerts"`
}

// MarketBreadthData represents market breadth metrics
type MarketBreadthData struct {
	PercentPositive24h  float64   `json:"percent_positive_24h"`
	PercentPositive7d   float64   `json:"percent_positive_7d"`
	NewHighs           int       `json:"new_highs"`
	NewLows            int       `json:"new_lows"`
	AltPerformance     float64   `json:"alt_performance"`
	VolumeDistribution map[string]float64 `json:"volume_distribution"`
	SectorPerformance  map[string]float64 `json:"sector_performance"`
	LastUpdated        time.Time `json:"last_updated"`
}

// PerformanceStats represents trading performance metrics
type PerformanceStats struct {
	TotalTrades     int     `json:"total_trades"`
	WinningTrades   int     `json:"winning_trades"`
	LosingTrades    int     `json:"losing_trades"`
	WinRate         float64 `json:"win_rate"`
	AverageWin      float64 `json:"average_win"`
	AverageLoss     float64 `json:"average_loss"`
	TotalReturn     float64 `json:"total_return"`
	SharpeRatio     float64 `json:"sharpe_ratio"`
	MaxDrawdown     float64 `json:"max_drawdown"`
	ProfitFactor    float64 `json:"profit_factor"`
	LastUpdated     time.Time `json:"last_updated"`
}

// CombinedConfig represents configuration for combined analysis system
type CombinedConfig struct {
	// Strategy Weights by Regime
	BullWeights     StrategyWeights `json:"bull_weights"`
	NeutralWeights  StrategyWeights `json:"neutral_weights"`
	BearWeights     StrategyWeights `json:"bear_weights"`
	
	// Risk Management
	MaxPortfolioExposure float64 `json:"max_portfolio_exposure"`
	MaxCorrelation       float64 `json:"max_correlation"`
	MaxSectorExposure    float64 `json:"max_sector_exposure"`
	MaxDailyTrades       int     `json:"max_daily_trades"`
	MaxPositions         int     `json:"max_positions"`
	MaxDrawdown          float64 `json:"max_drawdown"`
	
	// Alert Thresholds
	HighPriorityScore    float64 `json:"high_priority_score"`
	CriticalRiskScore    float64 `json:"critical_risk_score"`
	MaxDrawdownAlert     float64 `json:"max_drawdown_alert"`
	
	// Data Sources
	EnableMultipleFearGreed bool `json:"enable_multiple_fear_greed"`
	EnableMarketBreadth     bool `json:"enable_market_breadth"`
	EnablePerformanceDB     bool `json:"enable_performance_db"`
	
	// Timing
	ScanInterval        time.Duration `json:"scan_interval"`
	RiskCheckInterval   time.Duration `json:"risk_check_interval"`
	PerformanceInterval time.Duration `json:"performance_interval"`
}

// StrategyWeights represents weights for different strategies
type StrategyWeights struct {
	Momentum  float64 `json:"momentum"`
	Dip       float64 `json:"dip"`
	Sentiment float64 `json:"sentiment"`
}

// AlertConfig represents alert system configuration
type AlertConfig struct {
	DiscordWebhook  string `json:"discord_webhook"`
	TelegramToken   string `json:"telegram_token"`
	TelegramChatID  string `json:"telegram_chat_id"`
	EnableEmail     bool   `json:"enable_email"`
	EmailSMTP       string `json:"email_smtp"`
	EmailFrom       string `json:"email_from"`
	EmailTo         []string `json:"email_to"`
}

// ComprehensiveOpportunity represents a complete multi-dimensional opportunity analysis
type ComprehensiveOpportunity struct {
	// Basic Info
	Symbol       string          `json:"symbol"`
	PairCode     string          `json:"pair_code"`
	Price        decimal.Decimal `json:"price"`
	MarketCap    decimal.Decimal `json:"market_cap"`
	VolumeUSD    decimal.Decimal `json:"volume_usd"`
    Change24h    float64         `json:"change_24h"`
    Change7d     float64         `json:"change_7d"`
    OpportunityType string       `json:"opportunity_type"` // "DIP", "MOMENTUM", "BREAKOUT", "REVERSAL"
	
	// Multi-Dimensional Scores (0-100)
	RegimeScore      float64 `json:"regime_score"`
	DerivativesScore float64 `json:"derivatives_score"`
	OnChainScore     float64 `json:"on_chain_score"`
	WhaleScore       float64 `json:"whale_score"`       // Whale activity impact score (0-100)
	TechnicalScore   float64 `json:"technical_score"`
	VolumeScore      float64 `json:"volume_score"`
	LiquidityScore   float64 `json:"liquidity_score"`
    SentimentScore   float64 `json:"sentiment_score"`   // Multi-platform sentiment analysis score (0-100)
	
	// Hybrid Factor System Scores (from FactorWeights research)
	QualityScore             float64 `json:"quality_score"`              // Proven 0.847 correlation factor
	VolumeConfirmationScore  float64 `json:"volume_confirmation_score"`  // Proven 0.782 correlation factor
	ExitTimingScore          float64 `json:"exit_timing_score"`          // 28.6% correlation composite
	SetupScore               float64 `json:"setup_score"`                // 28.2% correlation composite
	MarketCapTier            string  `json:"market_cap_tier"`            // "LARGE", "MID", "SMALL", "MICRO", "NANO"
	
    // Composite Scores
    CompositeScore   float64 `json:"composite_score"`    // Weighted combination of all scores
    ConfidenceLevel  float64 `json:"confidence_level"`   // Statistical confidence (0-1)
    RiskScore        float64 `json:"risk_score"`         // Overall risk assessment (0-100)
	
	// Detailed Analysis Components
	RegimeAnalysis      RegimeAnalysis      `json:"regime_analysis"`
	DerivativesAnalysis DerivativesAnalysis `json:"derivatives_analysis"`
	OnChainAnalysis     OnChainAnalysis     `json:"on_chain_analysis"`
	WhaleAnalysis       WhaleActivityData   `json:"whale_analysis"`
	TechnicalAnalysis   TechnicalAnalysis   `json:"technical_analysis"`
	
	// Trading Information
	EntryPrice       decimal.Decimal   `json:"entry_price"`
	StopLoss         decimal.Decimal   `json:"stop_loss"`
	TakeProfit       []decimal.Decimal `json:"take_profit"`
	PositionSize     float64           `json:"position_size"`    // Recommended position size %
	ExpectedReturn   float64           `json:"expected_return"`  // Expected R:R ratio
	TimeHorizon      string            `json:"time_horizon"`     // "SHORT", "MEDIUM", "LONG"
	
	// Meta Information
	Strengths        []string  `json:"strengths"`
	Weaknesses       []string  `json:"weaknesses"`
	CatalystEvents   []string  `json:"catalyst_events"`
    RiskFactors      []string  `json:"risk_factors"`
    Timestamp        time.Time `json:"timestamp"`

    // Multi-timeframe returns (optional; % returns as 0-100 scaled or raw %)
    Return1h         float64 `json:"return_1h"`
    Return4h         float64 `json:"return_4h"`
    PrevReturn4h     float64 `json:"prev_return_4h"`
    Return12h        float64 `json:"return_12h"`
    Return24h        float64 `json:"return_24h"`
    Return7d         float64 `json:"return_7d"`

    // Volatility and microstructure (optional)
    ATR24h           float64 `json:"atr_24h"`
    ATR1h            float64 `json:"atr_1h"`
    ADX4h            float64 `json:"adx_4h"`
    Hurst            float64 `json:"hurst"`
    BidAskSpreadPct  float64 `json:"bid_ask_spread_pct"`
    Depth2PctUSD     float64 `json:"depth_2pct_usd"`
    Volume1hUSD      float64 `json:"volume_1h_usd"`
    AvgVolume7dUSD   float64 `json:"avg_volume_7d_usd"`

    // Entry freshness controls
    EntryTriggerPrice float64   `json:"entry_trigger_price"`
    SignalAgeBars1h   int       `json:"signal_age_bars_1h"`
    SignalAgeBars4h   int       `json:"signal_age_bars_4h"`
    LastExitTime      time.Time `json:"last_exit_time"`

    // Catalyst & Brand Power
    CatalystEvents   []CatalystEvent `json:"catalyst_events"`
    BrandPowerScore  float64         `json:"brand_power_score"` // 0-10 narrative strength
}

// CatalystEvent represents a known upcoming/ongoing catalyst
type CatalystEvent struct {
    Type       string    `json:"type"`        // listing, unlock, upgrade, partnership, governance
    Timestamp  time.Time `json:"timestamp"`   // event time
    Confidence float64   `json:"confidence"`  // 0-1 confidence of event
}

// TechnicalAnalysis represents comprehensive technical analysis
type TechnicalAnalysis struct {
	RSI              float64 `json:"rsi"`
	MACD             float64 `json:"macd"`
	BollingerBands   float64 `json:"bollinger_bands"`
	VolumeProfile    float64 `json:"volume_profile"`
	SupportLevel     decimal.Decimal `json:"support_level"`
	ResistanceLevel  decimal.Decimal `json:"resistance_level"`
	TrendStrength    float64 `json:"trend_strength"`
	PatternQuality   float64 `json:"pattern_quality"`
}

// ComprehensiveScanResult represents the result of a complete market scan
type ComprehensiveScanResult struct {
	TotalScanned     int                         `json:"total_scanned"`
	OpportunitiesFound int                       `json:"opportunities_found"`
	TopOpportunities []ComprehensiveOpportunity  `json:"top_opportunities"`
	MarketSummary    MarketSummary               `json:"market_summary"`
	ScanDuration     time.Duration               `json:"scan_duration"`
	Timestamp        time.Time                   `json:"timestamp"`
}

// MarketSummary provides overall market conditions context
type MarketSummary struct {
	OverallRegime      string  `json:"overall_regime"`      // "BULL", "BEAR", "NEUTRAL", "TRANSITION"
	MarketSentiment    float64 `json:"market_sentiment"`    // -100 to 100
	VolatilityLevel    string  `json:"volatility_level"`    // "LOW", "MEDIUM", "HIGH", "EXTREME"
	LiquidityHealth    float64 `json:"liquidity_health"`    // 0-100
	DerivativesBias    string  `json:"derivatives_bias"`    // "BULLISH", "BEARISH", "NEUTRAL"
	OnChainTrend       string  `json:"on_chain_trend"`      // "ACCUMULATION", "DISTRIBUTION", "NEUTRAL"
	RecommendedAction  string  `json:"recommended_action"`  // "BUY_DIPS", "SELL_RIPS", "WAIT", "RISK_OFF"
}

// ScoringWeights represents the weighting system for different analysis components
type ScoringWeights struct {
	RegimeWeight      float64 `json:"regime_weight"`
	DerivativesWeight float64 `json:"derivatives_weight"`
	OnChainWeight     float64 `json:"on_chain_weight"`
	WhaleWeight       float64 `json:"whale_weight"`
	TechnicalWeight   float64 `json:"technical_weight"`
	VolumeWeight      float64 `json:"volume_weight"`
	LiquidityWeight   float64 `json:"liquidity_weight"`
	SentimentWeight   float64 `json:"sentiment_weight"`
}

// RegimeAnalysis represents market regime analysis
type RegimeAnalysis struct {
	OverallRegime        string  `json:"overall_regime"`
	CompositeScore       float64 `json:"composite_score"`
	BTCRegimeStrength    float64 `json:"btc_regime_strength"`
	SectorRotationScore  float64 `json:"sector_rotation_score"`
	CurrentRegime        string  `json:"current_regime"`
	RegimeStrength       float64 `json:"regime_strength"`
	RegimeConfidence     float64 `json:"regime_confidence"`
	BTCTrend             string  `json:"btc_trend"`
	Strategy             string  `json:"recommended_strategy"`
	RiskMultiplier       float64 `json:"risk_multiplier"`
}

// DerivativesAnalysis represents derivatives market analysis  
type DerivativesAnalysis struct {
	OpenInterestTrend string    `json:"open_interest_trend"`
	FundingBias       string    `json:"funding_bias"`
	LeverageRatio     float64   `json:"leverage_ratio"`
	FundingRate       float64   `json:"funding_rate"`
	OpenInterest      float64   `json:"open_interest"`
	OIChange          float64   `json:"oi_change"`
	LiquidationRisk   float64   `json:"liquidation_risk"`
	OptionFlow        string    `json:"option_flow"`
	DerivativesBias   string    `json:"derivatives_bias"`
}

// OnChainAnalysis represents on-chain analysis
type OnChainAnalysis struct {
	ExchangeNetflow      float64 `json:"exchange_netflow"`
	WhaleAccumulation    float64 `json:"whale_accumulation"`
	WhaleDistribution    float64 `json:"whale_distribution"`
	StablecoinInflow     float64 `json:"stablecoin_inflow"`
	StablecoinOutflow    float64 `json:"stablecoin_outflow"`
	TrendDirection       string  `json:"trend_direction"`
	WalletFlows          string  `json:"wallet_flows"`
	WhaleActivity        float64 `json:"whale_activity"`
	StablecoinFlows      float64 `json:"stablecoin_flows"`
	NetworkMetrics       float64 `json:"network_metrics"`
	OnChainSentiment     string  `json:"on_chain_sentiment"`
	AccumDistribution    string  `json:"accumulation_distribution"`
}

// Ultra-Alpha Scanner Types

// SolanaOpportunity represents a Solana DEX opportunity
type SolanaOpportunity struct {
	TokenAddress         string    `json:"token_address"`
	Symbol              string    `json:"symbol"`
	Name                string    `json:"name"`
	Price               float64   `json:"price"`
	PriceChange24h      float64   `json:"price_change_24h"`
	VolumeUSD           float64   `json:"volume_usd"`
	MarketCap           float64   `json:"market_cap"`
	Liquidity           float64   `json:"liquidity"`
	
	// Solana-specific metrics
	DEXVenue            string    `json:"dex_venue"`        // "pump.fun", "raydium", "orca"
	SocialScore         float64   `json:"social_score"`
	KOLMentions         int       `json:"kol_mentions"`
	TwitterBuzz         float64   `json:"twitter_buzz"`
	TelegramActivity    float64   `json:"telegram_activity"`
	
	// Unified scoring
	UltraScore          float64   `json:"ultra_score"`
	RiskLevel           string    `json:"risk_level"`
	Confidence          float64   `json:"confidence"`
	Timestamp           time.Time `json:"timestamp"`
}

// ArbitrageOpportunity represents cross-chain arbitrage opportunity
type ArbitrageOpportunity struct {
	Symbol              string    `json:"symbol"`
	SolanaPrice         float64   `json:"solana_price"`
	CEXPrice            float64   `json:"cex_price"`
	SpreadPercent       float64   `json:"spread_percent"`
	ProfitPotential     float64   `json:"profit_potential"`
	
	// Execution details
	SolanaVenue         string    `json:"solana_venue"`
	CEXVenue            string    `json:"cex_venue"`
	MinTradeSize        float64   `json:"min_trade_size"`
	MaxTradeSize        float64   `json:"max_trade_size"`
	ExecutionRisk       string    `json:"execution_risk"`
	
	// Scoring
	ArbitrageScore      float64   `json:"arbitrage_score"`
	Timestamp           time.Time `json:"timestamp"`
}

// UnifiedOpportunity represents a cross-chain unified opportunity
type UnifiedOpportunity struct {
	Symbol              string    `json:"symbol"`
	Type                string    `json:"type"`           // "SOLANA", "CEX", "ARBITRAGE", "CROSS_CHAIN"
	Venue               string    `json:"venue"`
	
	// Unified metrics
	Price               float64   `json:"price"`
	PriceChange24h      float64   `json:"price_change_24h"`
	VolumeUSD           float64   `json:"volume_usd"`
	
	// Multi-dimensional scores
	TechnicalScore      float64   `json:"technical_score"`
	SocialScore         float64   `json:"social_score"`
	RiskScore           float64   `json:"risk_score"`
	LiquidityScore      float64   `json:"liquidity_score"`
	UltraScore          float64   `json:"ultra_score"`
	
	// Execution details
	RecommendedAction   string    `json:"recommended_action"`
	PositionSize        float64   `json:"position_size"`
	StopLoss            float64   `json:"stop_loss"`
	TakeProfit          float64   `json:"take_profit"`
	RiskReward          float64   `json:"risk_reward"`
	
	// Metadata
	Confidence          float64   `json:"confidence"`
	AlphaLevel          string    `json:"alpha_level"`    // "LOW", "MEDIUM", "HIGH", "EXTREME"
	Timestamp           time.Time `json:"timestamp"`
}

// RiskAssessment represents portfolio-level risk analysis
type RiskAssessment struct {
	OverallRiskScore    float64              `json:"overall_risk_score"`
	MaxDrawdownRisk     float64              `json:"max_drawdown_risk"`
	ConcentrationRisk   float64              `json:"concentration_risk"`
	CorrelationRisk     float64              `json:"correlation_risk"`
	LiquidityRisk       float64              `json:"liquidity_risk"`
	
	// Sector analysis
	SectorExposures     map[string]float64   `json:"sector_exposures"`
	TopCorrelations     []CorrelationPair    `json:"top_correlations"`
	RiskFactors         []string             `json:"risk_factors"`
	
	// Recommendations
	RecommendedActions  []string             `json:"recommended_actions"`
	MaxPositionSize     float64              `json:"max_position_size"`
	SuggestedHedges     []string             `json:"suggested_hedges"`
}

// AllocationRecommendation represents position sizing recommendation
type AllocationRecommendation struct {
	Symbol              string    `json:"symbol"`
	RecommendedPercent  float64   `json:"recommended_percent"`
	MaxPercent          float64   `json:"max_percent"`
	RiskAdjustedSize    float64   `json:"risk_adjusted_size"`
	Rationale           string    `json:"rationale"`
	Priority            string    `json:"priority"`       // "HIGH", "MEDIUM", "LOW"
}

// SectorAnalysis represents sector rotation analysis
type SectorAnalysis struct {
	Sector              string    `json:"sector"`
	MomentumScore       float64   `json:"momentum_score"`
	RelativeStrength    float64   `json:"relative_strength"`
	FlowDirection       string    `json:"flow_direction"` // "INFLOW", "OUTFLOW", "NEUTRAL"
	TopOpportunities    []string  `json:"top_opportunities"`
	RecommendedWeight   float64   `json:"recommended_weight"`
}

// SocialSentimentAnalysis represents social media sentiment analysis
type SocialSentimentAnalysis struct {
	OverallScore        float64            `json:"overall_score"`
	FearGreedIndex      int                `json:"fear_greed_index"`
	TwitterBuzz         float64            `json:"twitter_buzz"`
	TelegramActivity    float64            `json:"telegram_activity"`
	RedditSentiment     float64            `json:"reddit_sentiment"`
	KOLSentiment        float64            `json:"kol_sentiment"`
	
	// Trending topics
	TrendingTokens      []string           `json:"trending_tokens"`
	EmergingNarratives  []string           `json:"emerging_narratives"`
	SentimentShifts     []SentimentShift   `json:"sentiment_shifts"`
}

// CorrelationPair represents correlation between two assets
type CorrelationPair struct {
	Asset1              string    `json:"asset1"`
	Asset2              string    `json:"asset2"`
	Correlation         float64   `json:"correlation"`
	Timeframe           string    `json:"timeframe"`
}

// SentimentShift represents a change in market sentiment
type SentimentShift struct {
	Topic               string    `json:"topic"`
	PreviousScore       float64   `json:"previous_score"`
	CurrentScore        float64   `json:"current_score"`
	ChangePercent       float64   `json:"change_percent"`
	Significance        string    `json:"significance"`
}

// Whale Activity Analysis Types

// WhaleTransaction represents a large-value on-chain transaction
type WhaleTransaction struct {
	Hash            string          `json:"hash"`
	Blockchain      string          `json:"blockchain"`      // "bitcoin", "ethereum", "bsc", etc.
	Symbol          string          `json:"symbol"`          // Token symbol
	FromAddress     string          `json:"from_address"`
	ToAddress       string          `json:"to_address"`
	FromOwner       string          `json:"from_owner"`      // Exchange name or "unknown"
	ToOwner         string          `json:"to_owner"`        // Exchange name or "unknown"
	Amount          decimal.Decimal `json:"amount"`          // Token amount
	AmountUSD       decimal.Decimal `json:"amount_usd"`      // USD value
	TransactionType string          `json:"transaction_type"` // "transfer", "exchange_deposit", "exchange_withdrawal"
	Timestamp       time.Time       `json:"timestamp"`
	Confirmations   int             `json:"confirmations"`
	BlockHeight     int64           `json:"block_height"`
	GasUsed         decimal.Decimal `json:"gas_used,omitempty"`
	GasPrice        decimal.Decimal `json:"gas_price,omitempty"`
}

// WhaleActivityData represents aggregated whale activity metrics
type WhaleActivityData struct {
	Symbol          string            `json:"symbol"`
	Timeframe       string            `json:"timeframe"`        // "1h", "4h", "24h", "7d"
	TotalTxCount    int               `json:"total_tx_count"`
	TotalVolumeUSD  decimal.Decimal   `json:"total_volume_usd"`
	LargestTxUSD    decimal.Decimal   `json:"largest_tx_usd"`
	AverageTxUSD    decimal.Decimal   `json:"average_tx_usd"`

	// Exchange flow analysis
	ExchangeInflows  ExchangeFlowData  `json:"exchange_inflows"`
	ExchangeOutflows ExchangeFlowData  `json:"exchange_outflows"`
	NetExchangeFlow  decimal.Decimal   `json:"net_exchange_flow"` // Positive = net inflow, Negative = net outflow

	// Whale wallet clustering
	WhaleWallets     []WhaleWallet     `json:"whale_wallets"`
	ClusterActivity  ClusterActivity   `json:"cluster_activity"`

	// Significance scoring
	ActivityScore    float64           `json:"activity_score"`    // 0-100 based on volume and frequency
	ImpactScore      float64           `json:"impact_score"`      // 0-100 based on market impact potential
	Anomaly          bool              `json:"anomaly"`           // True if unusual activity detected
	AnomalyStrength  float64           `json:"anomaly_strength"`  // 0-100 strength of anomaly

	// Time-based metrics
	Activity1h       ActivityPeriod    `json:"activity_1h"`
	Activity4h       ActivityPeriod    `json:"activity_4h"`
	Activity24h      ActivityPeriod    `json:"activity_24h"`
	Activity7d       ActivityPeriod    `json:"activity_7d"`

	LastUpdated      time.Time         `json:"last_updated"`
	DataSource       string            `json:"data_source"`      // "whale_alert", "on_chain_api", etc.
}

// ExchangeFlowData represents exchange inflow/outflow analysis
type ExchangeFlowData struct {
	TotalVolumeUSD   decimal.Decimal            `json:"total_volume_usd"`
	TransactionCount int                        `json:"transaction_count"`
	TopExchanges     map[string]decimal.Decimal `json:"top_exchanges"`    // Exchange name -> volume
	FlowDirection    string                     `json:"flow_direction"`   // "INFLOW", "OUTFLOW", "BALANCED"
	FlowStrength     float64                    `json:"flow_strength"`    // 0-100
	DominantExchange string                     `json:"dominant_exchange"` // Exchange with highest flow
}

// WhaleWallet represents a known whale wallet with activity
type WhaleWallet struct {
	Address          string          `json:"address"`
	Label            string          `json:"label"`            // "Binance Hot Wallet", "Unknown Whale", etc.
	WalletType       string          `json:"wallet_type"`      // "exchange", "whale", "institutional", "unknown"
	Balance          decimal.Decimal `json:"balance"`          // Current balance
	BalanceUSD       decimal.Decimal `json:"balance_usd"`      // USD value of balance
	TransactionCount int             `json:"transaction_count"`
	TotalVolumeUSD   decimal.Decimal `json:"total_volume_usd"`
	LastActivity     time.Time       `json:"last_activity"`
	IsActive         bool            `json:"is_active"`        // Active in current timeframe
}

// ClusterActivity represents grouped whale activity patterns
type ClusterActivity struct {
	AccumulationWallets   []string        `json:"accumulation_wallets"`   // Wallets showing accumulation
	DistributionWallets   []string        `json:"distribution_wallets"`   // Wallets showing distribution
	ActiveExchanges       []string        `json:"active_exchanges"`       // Exchanges with high activity
	ClusterScore          float64         `json:"cluster_score"`          // 0-100 coordination score
	CoordinationDetected  bool            `json:"coordination_detected"`  // Coordinated activity detected
	CoordinationStrength  float64         `json:"coordination_strength"`  // 0-100
	DominantPattern       string          `json:"dominant_pattern"`       // "accumulation", "distribution", "mixed"
	PatternConfidence     float64         `json:"pattern_confidence"`     // 0-100
}

// ActivityPeriod represents whale activity within a specific time period
type ActivityPeriod struct {
	TransactionCount int             `json:"transaction_count"`
	VolumeUSD        decimal.Decimal `json:"volume_usd"`
	LargestTxUSD     decimal.Decimal `json:"largest_tx_usd"`
	NetFlow          decimal.Decimal `json:"net_flow"`          // Net exchange flow for this period
	ActivityScore    float64         `json:"activity_score"`    // 0-100
	Trend            string          `json:"trend"`             // "increasing", "decreasing", "stable"
	VsAverage        float64         `json:"vs_average"`        // % vs historical average
}

// WhaleAlert represents a real-time whale activity alert
type WhaleAlert struct {
	ID               string            `json:"id"`
	Symbol           string            `json:"symbol"`
	AmountUSD        decimal.Decimal   `json:"amount_usd"`
	TransactionType  string            `json:"transaction_type"`
	FromExchange     string            `json:"from_exchange,omitempty"`
	ToExchange       string            `json:"to_exchange,omitempty"`
	Significance     string            `json:"significance"`     // "LOW", "MEDIUM", "HIGH", "EXTREME"
	ImpactPrediction string            `json:"impact_prediction"` // "BULLISH", "BEARISH", "NEUTRAL"
	ConfidenceLevel  float64           `json:"confidence_level"` // 0-100
	Metadata         map[string]string `json:"metadata"`
	Timestamp        time.Time         `json:"timestamp"`
	Processed        bool              `json:"processed"`
}

// ADDITIONAL MISSING TYPES IDENTIFIED IN QA REVIEW - CRITICAL FIXES

// Extended BaseOpportunity with missing fields
type EnhancedBaseOpportunity struct {
	Symbol          string          `json:"symbol"`
	Price           decimal.Decimal `json:"price"`
	Change24h       float64         `json:"change_24h"`
	VolumeUSD       decimal.Decimal `json:"volume_usd"`
	QualityScore    float64         `json:"quality_score"`
	Timestamp       time.Time       `json:"timestamp"`
	// Missing fields identified in QA
	Pair            string          `json:"pair"`
	MarketStructure *MarketStructure `json:"market_structure"`
}

// Extended BacktestConfig with hour-level support
type ExtendedBacktestConfig struct {
	*BacktestConfig
	// Extended timeframe support (120d to 24h windows)
	Hours          int     `json:"hours"`           // Support for hour-level backtests
	Minutes        int     `json:"minutes"`         // Support for minute-level backtests
	Granularity    string  `json:"granularity"`     // "daily", "hourly", "minute"
	MaxTimeframe   int     `json:"max_timeframe"`   // Maximum days to test (up to 120d)
	MinTimeframe   int     `json:"min_timeframe"`   // Minimum hours to test (down to 24h)
}

// ExtendedTimeframe represents the enhanced timeframe system
type ExtendedTimeframe struct {
	Name        string        `json:"name"`         // "120d", "90d", "72h", "24h", etc.
	Duration    time.Duration `json:"duration"`     // Actual time duration
	Days        int           `json:"days"`         // Days component
	Hours       int           `json:"hours"`        // Hours component
	Granularity string        `json:"granularity"`  // "daily", "hourly"
	Priority    int           `json:"priority"`     // Testing priority (1=highest)
}

// ExtendedTimeframeSuite defines the complete testing suite
type ExtendedTimeframeSuite struct {
	Timeframes []ExtendedTimeframe `json:"timeframes"`
	TotalTests int                 `json:"total_tests"`
	Enabled    bool                `json:"enabled"`
}

// Advanced Performance Metrics with additional ratios
type AdvancedPerformanceMetrics struct {
	*PerformanceMetrics
	// Additional metrics requested in QA
	SortinoRatio           float64 `json:"sortino_ratio"`
	CalmarRatio           float64 `json:"calmar_ratio"`
	UlcerIndex            float64 `json:"ulcer_index"`
	MaxConsecutiveWins    int     `json:"max_consecutive_wins"`
	MaxConsecutiveLosses  int     `json:"max_consecutive_losses"`
	ProfitFactorByPeriod  map[string]float64 `json:"profit_factor_by_period"`
	DistributionalStats   DistributionalStats `json:"distributional_stats"`
}

// DistributionalStats for better evaluation
type DistributionalStats struct {
	Skewness    float64 `json:"skewness"`
	Kurtosis    float64 `json:"kurtosis"`
	VaR95       float64 `json:"var_95"`
	VaR99       float64 `json:"var_99"`
	CVaR95      float64 `json:"cvar_95"`
	CVaR99      float64 `json:"cvar_99"`
}

// Multi-Exchange Support
type ExchangeConfig struct {
	Name        string            `json:"name"`
	APIKey      string            `json:"api_key"`
	APISecret   string            `json:"api_secret"`
	BaseURL     string            `json:"base_url"`
	RateLimit   int               `json:"rate_limit"`
	Enabled     bool              `json:"enabled"`
	Features    []string          `json:"features"` // ["spot", "futures", "options"]
	Endpoints   map[string]string `json:"endpoints"`
}

// Enhanced Sentiment Data Integration
type EnhancedSentimentData struct {
	Symbol          string    `json:"symbol"`
	OverallScore    float64   `json:"overall_score"`
	FearGreedIndex  int       `json:"fear_greed_index"`
	SocialSentiment float64   `json:"social_sentiment"`
	NewsSentiment   float64   `json:"news_sentiment"`
	RedditScore     float64   `json:"reddit_score"`
	TwitterScore    float64   `json:"twitter_score"`
	LastUpdated     time.Time `json:"last_updated"`
	// Additional sentiment sources
	DiscordScore    float64   `json:"discord_score"`
	TelegramScore   float64   `json:"telegram_score"`
	YouTubeScore    float64   `json:"youtube_score"`
	NewsVolume      int       `json:"news_volume"`
	SocialVolume    int       `json:"social_volume"`
}

// MultiPlatformSentiment represents comprehensive social sentiment analysis across platforms
type MultiPlatformSentiment struct {
	Symbol              string            `json:"symbol"`
	Timestamp           time.Time         `json:"timestamp"`
	
	// Platform-specific sentiment scores (0-100 scale)
	TwitterSentiment    float64           `json:"twitter_sentiment"`
	RedditSentiment     float64           `json:"reddit_sentiment"`
	DiscordSentiment    float64           `json:"discord_sentiment"`
	TelegramSentiment   float64           `json:"telegram_sentiment"`
	
	// Aggregated sentiment metrics
	OverallSentiment    float64           `json:"overall_sentiment"`
	SentimentTrend      string            `json:"sentiment_trend"`     // "IMPROVING", "DECLINING", "STABLE"
	SentimentStrength   float64           `json:"sentiment_strength"`  // Confidence in sentiment reading (0-100)
	
	// Volume and buzz metrics
	SocialVolume        float64           `json:"social_volume"`
	VolumeChange24h     float64           `json:"volume_change_24h"`
	VolumeSurgeDetected bool              `json:"volume_surge_detected"`
	VolumeSurgeStrength float64           `json:"volume_surge_strength"`
	
	// KOL (Key Opinion Leader) influence data
	KOLInfluenceScore   float64           `json:"kol_influence_score"`
	KOLMentions         int               `json:"kol_mentions"`
	TopKOLs             []string          `json:"top_kols"`
	KOLSentimentBias    string            `json:"kol_sentiment_bias"`  // "BULLISH", "BEARISH", "NEUTRAL"
	
	// Platform-specific volume breakdown
	PlatformVolumes     map[string]float64 `json:"platform_volumes"`
	
	// Trending and narrative data
	TrendingTopics      []string          `json:"trending_topics"`
	EmergingNarratives  []string          `json:"emerging_narratives"`
	SentimentCatalysts  []string          `json:"sentiment_catalysts"`
	
	// Risk indicators
	ManipulationRisk    float64           `json:"manipulation_risk"`   // Risk of sentiment manipulation (0-100)
	BotActivityScore    float64           `json:"bot_activity_score"`  // Detected bot activity (0-100)
	DataQuality         float64           `json:"data_quality"`        // Quality of sentiment data (0-100)
}

// Enhanced On-Chain Data Integration
type EnhancedOnChainData struct {
	Symbol              string          `json:"symbol"`
	WhaleActivityScore  float64         `json:"whale_activity_score"`
	LargeTransactions   int             `json:"large_transactions"`
	StablecoinFlows     decimal.Decimal `json:"stablecoin_flows"`
	ExchangeInflows     decimal.Decimal `json:"exchange_inflows"`
	ExchangeOutflows    decimal.Decimal `json:"exchange_outflows"`
	HODLerScore         float64         `json:"hodler_score"`
	NetworkActivity     float64         `json:"network_activity"`
	Timestamp           time.Time       `json:"timestamp"`
	// Additional on-chain metrics
	ActiveAddresses     int             `json:"active_addresses"`
	NetworkHashRate     decimal.Decimal `json:"network_hash_rate"`
	MinerFlows          decimal.Decimal `json:"miner_flows"`
	StakingRatio        float64         `json:"staking_ratio"`
	YieldRates          map[string]float64 `json:"yield_rates"`
}

// Risk Management Enhancements
type EnhancedRiskConfig struct {
	UseATRStops         bool    `json:"use_atr_stops"`
	ATRPeriod          int     `json:"atr_period"`
	ATRMultiplier      float64 `json:"atr_multiplier"`
	UseTrailingStops   bool    `json:"use_trailing_stops"`
	TrailingPercent    float64 `json:"trailing_percent"`
	VolatilityTargeting bool   `json:"volatility_targeting"`
	TargetVolatility   float64 `json:"target_volatility"`
	MaxPositionSize    float64 `json:"max_position_size"`
	MaxCorrelation     float64 `json:"max_correlation"`
	KellyCriterion     bool    `json:"kelly_criterion"`
	// Additional risk management features
	DynamicPositionSizing bool     `json:"dynamic_position_sizing"`
	VaRLimit             float64   `json:"var_limit"`
	StressTestScenarios  []string  `json:"stress_test_scenarios"`
	HedgingEnabled       bool      `json:"hedging_enabled"`
	MaxLeverage          float64   `json:"max_leverage"`
	EmergencyStopLoss    float64   `json:"emergency_stop_loss"`
}

// Factor Sweep Engine Types
type FactorSweepConfig struct {
	// Factor ranges to test
	RSIPeriods         []int     `json:"rsi_periods"`
	RSIThresholds      []float64 `json:"rsi_thresholds"`
	MACDSettings       []MACDParams `json:"macd_settings"`
	VolumeThresholds   []float64 `json:"volume_thresholds"`
	DropPercentages    []float64 `json:"drop_percentages"`
	
	// Stop loss and take profit ranges
	StopLossRange      []float64 `json:"stop_loss_range"`
	TakeProfitRange    []float64 `json:"take_profit_range"`
	
	// Position sizing methods
	PositionSizingMethods []string `json:"position_sizing_methods"`
	
	// Timeframes to test
	TestTimeframes     []int     `json:"test_timeframes"` // in hours
	
	// Minimum sample sizes
	MinTrades          int       `json:"min_trades"`
	MinDays            int       `json:"min_days"`
}

type MACDParams struct {
	FastPeriod   int `json:"fast_period"`
	SlowPeriod   int `json:"slow_period"`
	SignalPeriod int `json:"signal_period"`
}

type FactorCombinationResult struct {
	Parameters      map[string]interface{} `json:"parameters"`
	Performance     AdvancedPerformanceMetrics `json:"performance"`
	Rank           int                    `json:"rank"`
	StatisticalSig bool                   `json:"statistical_significance"`
	OverfitRisk    float64                `json:"overfit_risk"`
	Robustness     float64                `json:"robustness"`
}

// Error Handling and Concurrency Types
type APIError struct {
	Exchange    string    `json:"exchange"`
	Endpoint    string    `json:"endpoint"`
	StatusCode  int       `json:"status_code"`
	Message     string    `json:"message"`
	RetryAfter  int       `json:"retry_after"`
	Timestamp   time.Time `json:"timestamp"`
}

type RateLimiter struct {
	RequestsPerMinute int           `json:"requests_per_minute"`
	BurstLimit        int           `json:"burst_limit"`
	BackoffStrategy   string        `json:"backoff_strategy"` // "exponential", "linear", "fixed"
	MaxRetries        int           `json:"max_retries"`
	RetryDelay        time.Duration `json:"retry_delay"`
}

// Enhanced Configuration System
type SystemConfig struct {
	// Basic system info  
	Name            string `json:"name"`
	Version         string `json:"version"`
	BuildTime       time.Time `json:"build_time"`
	Environment     string `json:"environment"` // "development", "testing", "production"
	
	// Core settings (compatible with main.go)
	ScanIntervalSec int    `json:"scan_interval_seconds"`
	MaxDailyTrades  int    `json:"max_daily_trades"`
	DatabasePath    string `json:"database_path"`
	
	// Logging and monitoring
	LogLevel        string    `json:"log_level"`
	LogFormat       string    `json:"log_format"`
	MetricsEnabled  bool      `json:"metrics_enabled"`
	TracingEnabled  bool      `json:"tracing_enabled"`
	
	// Performance settings
	MaxGoroutines   int       `json:"max_goroutines"`
	WorkerPoolSize  int       `json:"worker_pool_size"`
	CacheSize       int       `json:"cache_size"`
	CacheTTL        time.Duration `json:"cache_ttl"`
	
	// Database settings
	DatabaseURL     string    `json:"database_url"`
	RedisURL        string    `json:"redis_url"`
	BackupEnabled   bool      `json:"backup_enabled"`
}

// Testing and Validation Types
type TestSuite struct {
	Name            string        `json:"name"`
	Tests           []TestCase    `json:"tests"`
	SetupFunc       string        `json:"setup_func"`
	TeardownFunc    string        `json:"teardown_func"`
	Timeout         time.Duration `json:"timeout"`
	Parallel        bool          `json:"parallel"`
}

type TestCase struct {
	Name            string                 `json:"name"`
	Description     string                 `json:"description"`
	Algorithm       string                 `json:"algorithm"`
	Parameters      map[string]interface{} `json:"parameters"`
	ExpectedResults ExpectedResults        `json:"expected_results"`
	Tolerance       float64                `json:"tolerance"`
}

type ExpectedResults struct {
	MinWinRate      float64 `json:"min_win_rate"`
	MinProfitFactor float64 `json:"min_profit_factor"`
	MaxDrawdown     float64 `json:"max_drawdown"`
	MinSharpeRatio  float64 `json:"min_sharpe_ratio"`
	MinTrades       int     `json:"min_trades"`
}

// UI and Progress Types
type ProgressUpdate struct {
	Stage           string    `json:"stage"`
	Percent         float64   `json:"percent"`
	Message         string    `json:"message"`
	ETA             time.Duration `json:"eta"`
	ItemsProcessed  int       `json:"items_processed"`
	TotalItems      int       `json:"total_items"`
	Timestamp       time.Time `json:"timestamp"`
}

type KeyboardShortcut struct {
	Key         string `json:"key"`
	Description string `json:"description"`
	Action      string `json:"action"`
	Enabled     bool   `json:"enabled"`
}

// Missing types identified in QA Review
type MarketData struct {
	Symbol           string          `json:"symbol"`
	Price            float64         `json:"price"`
	Volume24h        float64         `json:"volume_24h"`
	Change24h        float64         `json:"change_24h"`
	PriceHigh        float64         `json:"price_high"`
	PriceLow         float64         `json:"price_low"`
	High24h          float64         `json:"high_24h"`
	Low24h           float64         `json:"low_24h"`
	MarketCap        decimal.Decimal `json:"market_cap"`
	CirculatingSupply decimal.Decimal `json:"circulating_supply"`
	TotalSupply      decimal.Decimal `json:"total_supply"`
	BidPrice         float64         `json:"bid_price"`
	AskPrice         float64         `json:"ask_price"`
	Spread           float64         `json:"spread"`
	LastUpdateID     int64           `json:"last_update_id"`
	Timestamp        time.Time       `json:"timestamp"`
}

type FundingData struct {
	Symbol       string          `json:"symbol"`
	FundingRate  decimal.Decimal `json:"funding_rate"`
	NextFunding  time.Time       `json:"next_funding"`
	MarkPrice    decimal.Decimal `json:"mark_price"`
	IndexPrice   decimal.Decimal `json:"index_price"`
	Timestamp    time.Time       `json:"timestamp"`
}

type OpenInterestData struct {
	Symbol         string          `json:"symbol"`
	OpenInterest   decimal.Decimal `json:"open_interest"`
	Volume24h      decimal.Decimal `json:"volume_24h"`
	Change24h      float64         `json:"change_24h"`
	Timestamp      time.Time       `json:"timestamp"`
}

type MarketStructure struct {
	Symbol        string    `json:"symbol"`
	Phase         string    `json:"phase"`      // "accumulation", "markup", "distribution", "markdown"
	Trend         string    `json:"trend"`      // "bullish", "bearish", "neutral"
	Strength      float64   `json:"strength"`   // 0-100
	Volume        float64   `json:"volume"`
	Volatility    float64   `json:"volatility"`
	Timestamp     time.Time `json:"timestamp"`
}

func NewRateLimiter(rps int, burst int, timeout time.Duration) *RateLimiter {
	return &RateLimiter{
		RequestsPerMinute: rps * 60, // Convert to requests per minute
		BurstLimit:        burst,
		MaxRetries:        3,
		BackoffStrategy:   "exponential",
	}
}

// Configuration Validation Types
type ValidationRule struct {
	Field       string      `json:"field"`
	Rule        string      `json:"rule"`        // "required", "range", "enum", "regex"
	Parameters  interface{} `json:"parameters"`
	Message     string      `json:"message"`
	Severity    string      `json:"severity"`    // "error", "warning", "info"
}

type ValidationResult struct {
	Valid       bool             `json:"valid"`
	Errors      []ValidationError `json:"errors"`
	Warnings    []ValidationError `json:"warnings"`
	Suggestions []string         `json:"suggestions"`
}

type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
	Value   interface{} `json:"value"`
}

// Optimized Scanner Configuration based on backtest analysis
type OptimizedScannerConfig struct {
	Name                string        `json:"name"`
	TimeframeDays       int           `json:"timeframe_days"`
	FactorWeights       FactorWeights `json:"factor_weights"`
	MinCompositeScore   float64       `json:"min_composite_score"`
	MaxPositions        int           `json:"max_positions"`
	RiskPerTrade        float64       `json:"risk_per_trade"`
}

// FactorWeights represents the proven factor weights from backtest analysis
type FactorWeights struct {
	QualityScore             float64 `json:"quality_score"`
	VolumeConfirmation       float64 `json:"volume_confirmation"`
	SocialSentiment          float64 `json:"social_sentiment"`
	CrossMarketCorr          float64 `json:"cross_market_correlation"`
	TechnicalIndicators      float64 `json:"technical_indicators"`
	RiskManagement           float64 `json:"risk_management"`
	PortfolioDiversification float64 `json:"portfolio_diversification"`
	DerivativesWeight        float64 `json:"derivatives_weight"`
	OnChainWeight            float64 `json:"on_chain_weight"`
	SentimentWeight          float64 `json:"sentiment_weight"`
	WhaleWeight              float64 `json:"whale_weight"`
}

// OptimizedScanResult represents the result of an optimized scanner run
type OptimizedScanResult struct {
	Config               OptimizedScannerConfig       `json:"config"`
	Opportunities        []ComprehensiveOpportunity   `json:"opportunities"`
	TotalScanned         int                          `json:"total_scanned"`
	OpportunitiesFound   int                          `json:"opportunities_found"`
	AverageComposite     float64                      `json:"average_composite"`
	TopOpportunities     []ComprehensiveOpportunity   `json:"top_opportunities"`
	MarketSummary        MarketSummary                `json:"market_summary"`
	ScanDuration         time.Duration                `json:"scan_duration"`
	Timestamp            time.Time                    `json:"timestamp"`
	
	// Transparency fields for full visibility into filtering
	TotalAnalyzed        int                          `json:"total_analyzed"`
	FilteredOut          int                          `json:"filtered_out"`
	TrimmedOut           int                          `json:"trimmed_out"`
	AllOpportunities     []ComprehensiveOpportunity   `json:"all_opportunities"`
}

// BTCData represents Bitcoin market data and analysis
type BTCData struct {
	Symbol         string          `json:"symbol"`
	Price          decimal.Decimal `json:"price"`
	Change24h      float64         `json:"change_24h"`
	Volume24h      decimal.Decimal `json:"volume_24h"`
	MarketCap      decimal.Decimal `json:"market_cap"`
	Dominance      float64         `json:"dominance"`
	FearGreedIndex int             `json:"fear_greed_index"`
	RSI            float64         `json:"rsi"`
	MA200          decimal.Decimal `json:"ma_200"`
	Timestamp      time.Time       `json:"timestamp"`
}

// ETHData represents Ethereum market data and analysis
type ETHData struct {
	Symbol         string          `json:"symbol"`
	Price          decimal.Decimal `json:"price"`
	Change24h      float64         `json:"change_24h"`
	Volume24h      decimal.Decimal `json:"volume_24h"`
	MarketCap      decimal.Decimal `json:"market_cap"`
	GasPrice       decimal.Decimal `json:"gas_price"`
	NetworkActivity float64        `json:"network_activity"`
	DeFiTVL        decimal.Decimal `json:"defi_tvl"`
	RSI            float64         `json:"rsi"`
	Timestamp      time.Time       `json:"timestamp"`
}

// MarketDataPoint represents a single point in time market data
type MarketDataPoint struct {
	Symbol    string          `json:"symbol"`
	Timestamp time.Time       `json:"timestamp"`
	Price     decimal.Decimal `json:"price"`
	Volume    decimal.Decimal `json:"volume"`
	High      decimal.Decimal `json:"high"`
	Low       decimal.Decimal `json:"low"`
	Open      decimal.Decimal `json:"open"`
	Close     decimal.Decimal `json:"close"`
}
