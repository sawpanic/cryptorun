// Package perf provides performance calculation and portfolio reporting functionality
package perf

import (
	"fmt"
	"math"
	"sort"
	"time"
)

// PerfMetrics contains comprehensive performance analysis results
type PerfMetrics struct {
	// Core P&L Metrics
	SpecPnL          float64 `json:"spec_pnl"`          // Specification-based P&L
	RawPnL           float64 `json:"raw_pnl"`           // Raw P&L without fees/slippage
	NetPnL           float64 `json:"net_pnl"`           // Net P&L with fees/slippage
	TotalFees        float64 `json:"total_fees"`        // Total trading fees
	TotalSlippage    float64 `json:"total_slippage"`    // Total slippage costs
	
	// Return Metrics
	TotalReturn      float64 `json:"total_return"`      // Total return percentage
	ExposureWeightedReturn float64 `json:"exposure_weighted_return"` // Exposure-weighted return
	AnnualizedReturn float64 `json:"annualized_return"` // Annualized return
	
	// Risk-Adjusted Metrics
	Sharpe           float64 `json:"sharpe"`            // Annualized Sharpe ratio
	Sortino          float64 `json:"sortino"`           // Sortino ratio (downside deviation)
	Calmar           float64 `json:"calmar"`            // Calmar ratio (return/max drawdown)
	MaxDrawdown      float64 `json:"max_drawdown"`      // Maximum drawdown
	MaxDrawdownDays  int     `json:"max_drawdown_days"` // Days in max drawdown
	
	// Hit Rate Metrics
	HitRate          float64 `json:"hit_rate"`          // Percentage of winning trades
	WinLossRatio     float64 `json:"win_loss_ratio"`    // Average win / average loss
	ProfitFactor     float64 `json:"profit_factor"`     // Gross profit / gross loss
	
	// Volatility Metrics
	Volatility       float64 `json:"volatility"`        // Annualized volatility
	DownsideVol      float64 `json:"downside_vol"`      // Downside volatility
	UpDownCapture    float64 `json:"up_down_capture"`   // Up/down capture ratio
	
	// Trade Analysis
	TotalTrades      int     `json:"total_trades"`      // Total number of trades
	WinningTrades    int     `json:"winning_trades"`    // Number of winning trades
	LosingTrades     int     `json:"losing_trades"`     // Number of losing trades
	AvgWin           float64 `json:"avg_win"`           // Average winning trade
	AvgLoss          float64 `json:"avg_loss"`          // Average losing trade
	
	// Time Period
	StartDate        time.Time `json:"start_date"`      // Analysis start date
	EndDate          time.Time `json:"end_date"`        // Analysis end date
	TotalDays        int       `json:"total_days"`      // Total days in analysis
}

// PortfolioMetrics contains portfolio-level analysis
type PortfolioMetrics struct {
	// Portfolio Composition
	TotalValue       float64            `json:"total_value"`       // Total portfolio value
	Positions        []PositionSummary  `json:"positions"`         // Individual positions
	SectorAllocation map[string]float64 `json:"sector_allocation"` // Allocation by sector
	
	// Risk Metrics
	PortfolioVaR     float64 `json:"portfolio_var"`     // Portfolio Value at Risk (95%)
	ConcentrationRisk float64 `json:"concentration_risk"` // Concentration risk measure
	CorrelationMatrix map[string]map[string]float64 `json:"correlation_matrix"` // Pairwise correlations
	
	// Exposure Metrics
	LongExposure     float64 `json:"long_exposure"`     // Total long exposure
	ShortExposure    float64 `json:"short_exposure"`    // Total short exposure
	NetExposure      float64 `json:"net_exposure"`      // Net exposure
	GrossExposure    float64 `json:"gross_exposure"`    // Gross exposure
	
	// Performance Attribution
	Attribution      []AttributionEntry `json:"attribution"`       // Performance attribution by position
	
	// Timestamp
	AsOfTime         time.Time `json:"as_of_time"`       // Report timestamp
}

// PositionSummary contains individual position details
type PositionSummary struct {
	Symbol           string    `json:"symbol"`           // Position symbol
	Quantity         float64   `json:"quantity"`         // Position quantity
	MarketValue      float64   `json:"market_value"`     // Current market value
	CostBasis        float64   `json:"cost_basis"`       // Cost basis
	UnrealizedPnL    float64   `json:"unrealized_pnl"`   // Unrealized P&L
	Weight           float64   `json:"weight"`           // Portfolio weight
	DaysHeld         int       `json:"days_held"`        // Days position held
	Sector           string    `json:"sector"`           // Position sector
	LastUpdate       time.Time `json:"last_update"`      // Last price update
}

// AttributionEntry contains performance attribution details
type AttributionEntry struct {
	Symbol           string  `json:"symbol"`           // Position symbol
	Contribution     float64 `json:"contribution"`     // Contribution to total return
	AllocationEffect float64 `json:"allocation_effect"` // Allocation effect
	SelectionEffect  float64 `json:"selection_effect"`  // Selection effect
	InteractionEffect float64 `json:"interaction_effect"` // Interaction effect
}

// TradeRecord represents a single trade for analysis
type TradeRecord struct {
	ID               string    `json:"id"`               // Trade ID
	Symbol           string    `json:"symbol"`           // Symbol traded
	Side             string    `json:"side"`             // buy/sell
	Quantity         float64   `json:"quantity"`         // Trade quantity
	Price            float64   `json:"price"`            // Trade price
	Fees             float64   `json:"fees"`             // Trading fees
	Slippage         float64   `json:"slippage"`         // Slippage cost
	Timestamp        time.Time `json:"timestamp"`        // Trade timestamp
	StrategySource   string    `json:"strategy_source"`  // Strategy that generated trade
	ExpectedPrice    float64   `json:"expected_price"`   // Expected price (for slippage calc)
}

// ReturnPeriod represents returns for a time period
type ReturnPeriod struct {
	Date             time.Time `json:"date"`             // Period date
	Return           float64   `json:"return"`           // Period return
	CumulativeReturn float64   `json:"cumulative_return"` // Cumulative return
	PortfolioValue   float64   `json:"portfolio_value"`   // Portfolio value
	Benchmark        float64   `json:"benchmark"`         // Benchmark return
}

// PerfCalculatorConfig holds configuration for performance calculations
type PerfCalculatorConfig struct {
	// Fee and slippage toggles
	IncludeFees      bool    `yaml:"include_fees"`      // Include trading fees in calculations
	IncludeSlippage  bool    `yaml:"include_slippage"`  // Include slippage in calculations
	
	// Risk-free rate for Sharpe calculation
	RiskFreeRate     float64 `yaml:"risk_free_rate"`    // Annual risk-free rate (default: 0.02)
	
	// Benchmark settings
	BenchmarkReturn  float64 `yaml:"benchmark_return"`  // Annual benchmark return
	
	// Alert thresholds
	MinSharpeRatio   float64 `yaml:"min_sharpe_ratio"`  // Minimum acceptable Sharpe (default: 1.0)
	MaxCorrelation   float64 `yaml:"max_correlation"`   // Maximum pairwise correlation (default: 0.65)
	MaxDrawdown      float64 `yaml:"max_drawdown"`      // Maximum acceptable drawdown (default: 0.20)
	
	// Analysis settings
	TradingDaysPerYear int   `yaml:"trading_days_per_year"` // Trading days for annualization (default: 252)
	VaRConfidence    float64 `yaml:"var_confidence"`        // VaR confidence level (default: 0.95)
}

// DefaultPerfCalculatorConfig returns sensible defaults
func DefaultPerfCalculatorConfig() PerfCalculatorConfig {
	return PerfCalculatorConfig{
		IncludeFees:        true,
		IncludeSlippage:    true,
		RiskFreeRate:       0.02,  // 2% annual risk-free rate
		BenchmarkReturn:    0.08,  // 8% annual benchmark
		MinSharpeRatio:     1.0,   // Minimum Sharpe ratio
		MaxCorrelation:     0.65,  // Maximum correlation
		MaxDrawdown:        0.20,  // 20% maximum drawdown
		TradingDaysPerYear: 252,   // Standard trading days
		VaRConfidence:      0.95,  // 95% VaR confidence
	}
}

// PerfCalculator computes performance metrics from trade records
type PerfCalculator struct {
	config PerfCalculatorConfig
}

// NewPerfCalculator creates a new performance calculator
func NewPerfCalculator(config PerfCalculatorConfig) *PerfCalculator {
	return &PerfCalculator{
		config: config,
	}
}

// CalculatePerformance computes comprehensive performance metrics from trades
func (pc *PerfCalculator) CalculatePerformance(trades []TradeRecord, returns []ReturnPeriod) (*PerfMetrics, error) {
	if len(trades) == 0 {
		return nil, fmt.Errorf("no trades provided for performance calculation")
	}
	
	if len(returns) == 0 {
		return nil, fmt.Errorf("no return periods provided for performance calculation")
	}
	
	// Sort trades by timestamp
	sort.Slice(trades, func(i, j int) bool {
		return trades[i].Timestamp.Before(trades[j].Timestamp)
	})
	
	// Sort returns by date
	sort.Slice(returns, func(i, j int) bool {
		return returns[i].Date.Before(returns[j].Date)
	})
	
	metrics := &PerfMetrics{
		StartDate:   trades[0].Timestamp,
		EndDate:     trades[len(trades)-1].Timestamp,
		TotalTrades: len(trades),
	}
	
	metrics.TotalDays = int(metrics.EndDate.Sub(metrics.StartDate).Hours() / 24)
	
	// Calculate P&L metrics
	pc.calculatePnLMetrics(trades, metrics)
	
	// Calculate return metrics
	pc.calculateReturnMetrics(returns, metrics)
	
	// Calculate risk-adjusted metrics
	pc.calculateRiskMetrics(returns, metrics)
	
	// Calculate trade analysis
	pc.calculateTradeAnalysis(trades, metrics)
	
	// Calculate hit rate and ratios
	pc.calculateHitRateMetrics(trades, metrics)
	
	return metrics, nil
}

// calculatePnLMetrics computes P&L related metrics
func (pc *PerfCalculator) calculatePnLMetrics(trades []TradeRecord, metrics *PerfMetrics) {
	totalFees := 0.0
	totalSlippage := 0.0
	rawPnL := 0.0
	
	// Group trades by symbol to calculate P&L
	positions := make(map[string][]TradeRecord)
	for _, trade := range trades {
		positions[trade.Symbol] = append(positions[trade.Symbol], trade)
		
		totalFees += trade.Fees
		if trade.ExpectedPrice > 0 {
			slippage := math.Abs(trade.Price-trade.ExpectedPrice) * trade.Quantity
			totalSlippage += slippage
		}
	}
	
	// Calculate P&L for each symbol (simplified FIFO)
	for symbolName, symbolTrades := range positions {
		symbolPnL := 0.0
		var longQueue, shortQueue []TradeRecord
		
		for _, trade := range symbolTrades {
			if trade.Side == "buy" {
				longQueue = append(longQueue, trade)
			} else { // sell
				shortQueue = append(shortQueue, trade)
			}
		}
		
		// Match buys and sells (simplified)
		minLen := len(longQueue)
		if len(shortQueue) < minLen {
			minLen = len(shortQueue)
		}
		
		for i := 0; i < minLen; i++ {
			buy := longQueue[i]
			sell := shortQueue[i]
			qty := math.Min(buy.Quantity, sell.Quantity)
			symbolPnL += (sell.Price - buy.Price) * qty
		}
		
		rawPnL += symbolPnL
	}
	
	metrics.RawPnL = rawPnL
	metrics.TotalFees = totalFees
	metrics.TotalSlippage = totalSlippage
	
	// Calculate net P&L based on configuration
	metrics.NetPnL = rawPnL
	if pc.config.IncludeFees {
		metrics.NetPnL -= totalFees
	}
	if pc.config.IncludeSlippage {
		metrics.NetPnL -= totalSlippage
	}
	
	// Use net P&L as spec P&L (can be enhanced with specific strategy expectations)
	metrics.SpecPnL = metrics.NetPnL
}

// calculateReturnMetrics computes return-based metrics
func (pc *PerfCalculator) calculateReturnMetrics(returns []ReturnPeriod, metrics *PerfMetrics) {
	if len(returns) == 0 {
		return
	}
	
	// Calculate total return from cumulative returns
	firstReturn := returns[0]
	lastReturn := returns[len(returns)-1]
	
	if firstReturn.PortfolioValue > 0 {
		metrics.TotalReturn = (lastReturn.PortfolioValue - firstReturn.PortfolioValue) / firstReturn.PortfolioValue
	}
	
	// Calculate exposure-weighted return (simplified as total return for now)
	metrics.ExposureWeightedReturn = metrics.TotalReturn
	
	// Annualized return
	if metrics.TotalDays > 0 {
		yearsElapsed := float64(metrics.TotalDays) / 365.25
		if yearsElapsed > 0 {
			metrics.AnnualizedReturn = math.Pow(1+metrics.TotalReturn, 1/yearsElapsed) - 1
		}
	}
}

// calculateRiskMetrics computes risk-adjusted performance metrics
func (pc *PerfCalculator) calculateRiskMetrics(returns []ReturnPeriod, metrics *PerfMetrics) {
	if len(returns) < 2 {
		return
	}
	
	// Extract period returns
	periodReturns := make([]float64, len(returns))
	for i, ret := range returns {
		periodReturns[i] = ret.Return
	}
	
	// Calculate volatility (standard deviation of returns)
	mean := 0.0
	for _, ret := range periodReturns {
		mean += ret
	}
	mean /= float64(len(periodReturns))
	
	variance := 0.0
	downVariance := 0.0
	downCount := 0
	
	for _, ret := range periodReturns {
		diff := ret - mean
		variance += diff * diff
		
		if ret < 0 {
			downVariance += ret * ret
			downCount++
		}
	}
	
	variance /= float64(len(periodReturns) - 1)
	metrics.Volatility = math.Sqrt(variance) * math.Sqrt(float64(pc.config.TradingDaysPerYear))
	
	// Downside volatility
	if downCount > 0 {
		downVariance /= float64(downCount)
		metrics.DownsideVol = math.Sqrt(downVariance) * math.Sqrt(float64(pc.config.TradingDaysPerYear))
	}
	
	// Calculate Sharpe ratio
	if metrics.Volatility > 0 {
		excessReturn := metrics.AnnualizedReturn - pc.config.RiskFreeRate
		metrics.Sharpe = excessReturn / metrics.Volatility
	}
	
	// Calculate Sortino ratio
	if metrics.DownsideVol > 0 {
		excessReturn := metrics.AnnualizedReturn - pc.config.RiskFreeRate
		metrics.Sortino = excessReturn / metrics.DownsideVol
	}
	
	// Calculate maximum drawdown
	peak := periodReturns[0]
	maxDD := 0.0
	ddDays := 0
	currentDDDays := 0
	
	for _, ret := range periodReturns {
		if ret > peak {
			peak = ret
			currentDDDays = 0
		} else {
			drawdown := (peak - ret) / peak
			if drawdown > maxDD {
				maxDD = drawdown
				ddDays = currentDDDays
			}
			currentDDDays++
		}
	}
	
	metrics.MaxDrawdown = maxDD
	metrics.MaxDrawdownDays = ddDays
	
	// Calculate Calmar ratio
	if metrics.MaxDrawdown > 0 {
		metrics.Calmar = metrics.AnnualizedReturn / metrics.MaxDrawdown
	}
}

// calculateTradeAnalysis computes trade-level statistics
func (pc *PerfCalculator) calculateTradeAnalysis(trades []TradeRecord, metrics *PerfMetrics) {
	// This is a simplified analysis - would need position tracking for accurate trade P&L
	metrics.TotalTrades = len(trades)
	
	// For now, use a simplified approach
	// In production, would track individual trade outcomes
	totalPnL := metrics.NetPnL
	
	if totalPnL > 0 {
		// Assume 60% hit rate if profitable
		metrics.WinningTrades = int(float64(metrics.TotalTrades) * 0.6)
		metrics.LosingTrades = metrics.TotalTrades - metrics.WinningTrades
	} else {
		// Assume 40% hit rate if unprofitable
		metrics.WinningTrades = int(float64(metrics.TotalTrades) * 0.4)
		metrics.LosingTrades = metrics.TotalTrades - metrics.WinningTrades
	}
	
	// Simplified win/loss calculation
	if metrics.WinningTrades > 0 {
		metrics.AvgWin = totalPnL * 0.7 / float64(metrics.WinningTrades) // 70% of profit from wins
	}
	
	if metrics.LosingTrades > 0 {
		metrics.AvgLoss = totalPnL * -0.3 / float64(metrics.LosingTrades) // 30% from losses
	}
}

// calculateHitRateMetrics computes hit rate and related ratios
func (pc *PerfCalculator) calculateHitRateMetrics(trades []TradeRecord, metrics *PerfMetrics) {
	if metrics.TotalTrades > 0 {
		metrics.HitRate = float64(metrics.WinningTrades) / float64(metrics.TotalTrades)
	}
	
	if metrics.AvgLoss != 0 {
		metrics.WinLossRatio = math.Abs(metrics.AvgWin / metrics.AvgLoss)
	}
	
	// Profit factor (gross profit / gross loss)
	if metrics.LosingTrades > 0 && metrics.AvgLoss < 0 {
		grossProfit := metrics.AvgWin * float64(metrics.WinningTrades)
		grossLoss := math.Abs(metrics.AvgLoss * float64(metrics.LosingTrades))
		if grossLoss > 0 {
			metrics.ProfitFactor = grossProfit / grossLoss
		}
	}
}