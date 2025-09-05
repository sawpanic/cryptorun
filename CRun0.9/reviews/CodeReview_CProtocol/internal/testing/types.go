package testing

import (
	"github.com/shopspring/decimal"
	"time"
)

// Legacy BacktestProtocolConfig - keeping minimal version for compatibility

// ComprehensiveMetrics holds comprehensive backtesting metrics
type ComprehensiveMetrics struct {
	TotalReturn     decimal.Decimal `json:"total_return"`
	AnnualizedReturn decimal.Decimal `json:"annualized_return"`
	Volatility      decimal.Decimal `json:"volatility"`
	SharpeRatio     decimal.Decimal `json:"sharpe_ratio"`
	MaxDrawdown     decimal.Decimal `json:"max_drawdown"`
	WinRate         float64         `json:"win_rate"`
	ProfitFactor    decimal.Decimal `json:"profit_factor"`
	CalmarRatio     decimal.Decimal `json:"calmar_ratio"`
	IC              decimal.Decimal `json:"ic"`              // Information Coefficient
	ICIR            decimal.Decimal `json:"icir"`            // Information Coefficient IR
	HitRate         float64         `json:"hit_rate"`        // Hit rate
}

// EmbargoConfig defines embargo configuration for time series validation
type EmbargoConfig struct {
	Enabled         bool          `json:"enabled"`
	EmbargoLength   time.Duration `json:"embargo_length"`
	MinGapLength    time.Duration `json:"min_gap_length"`
	BaseEmbargo     time.Duration `json:"base_embargo"`
	EventEmbargo    time.Duration `json:"event_embargo"`
	LowFloatEmbargo time.Duration `json:"low_float_embargo"`
}

// ExecutionCostConfig defines configuration for execution cost modeling
type ExecutionCostConfig struct {
	SlippageModel   string          `json:"slippage_model"`
	CommissionRate  decimal.Decimal `json:"commission_rate"`
	MarketImpact    decimal.Decimal `json:"market_impact"`
	MinCommission   decimal.Decimal `json:"min_commission"`
}

// TimeRange defines a time range for validation
type TimeRange struct {
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
}

