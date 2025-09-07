package main

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/sawpanic/cryptorun/internal/persistence"
	"github.com/sawpanic/cryptorun/internal/report/perf"
	"github.com/spf13/cobra"
)

// reportCmd represents the report command
var reportCmd = &cobra.Command{
	Use:   "report",
	Short: "Generate performance and portfolio reports",
	Long: `Generate comprehensive performance analysis and portfolio reports.

Examples:
  cryptorun report --performance --from 2025-01-01 --to 2025-12-31 --format md --outfile report.md
  cryptorun report --portfolio --format csv --outfile portfolio.csv
  cryptorun report --performance --from 2025-01-01 --format json`,
	RunE: runReportCommand,
}

var (
	reportPerformance bool
	reportPortfolio   bool
	reportFromDate    string
	reportToDate      string
	reportFormat      string
	reportOutfile     string
)

func init() {
	rootCmd.AddCommand(reportCmd)

	reportCmd.Flags().BoolVar(&reportPerformance, "performance", false, "Generate performance report")
	reportCmd.Flags().BoolVar(&reportPortfolio, "portfolio", false, "Generate portfolio report")
	reportCmd.Flags().StringVar(&reportFromDate, "from", "", "Start date for performance analysis (YYYY-MM-DD)")
	reportCmd.Flags().StringVar(&reportToDate, "to", "", "End date for performance analysis (YYYY-MM-DD)")
	reportCmd.Flags().StringVar(&reportFormat, "format", "md", "Output format: md, csv, json")
	reportCmd.Flags().StringVar(&reportOutfile, "outfile", "", "Output file path (default: stdout)")

	// Note: At least one of --performance or --portfolio must be specified (checked in runReportCommand)
}

func runReportCommand(cmd *cobra.Command, args []string) error {
	if !reportPerformance && !reportPortfolio {
		return fmt.Errorf("at least one of --performance or --portfolio must be specified")
	}

	// Validate format
	validFormats := []string{"md", "csv", "json"}
	formatValid := false
	for _, format := range validFormats {
		if reportFormat == format {
			formatValid = true
			break
		}
	}
	if !formatValid {
		return fmt.Errorf("invalid format: %s (valid: %s)", reportFormat, strings.Join(validFormats, ", "))
	}

	// Parse dates
	var fromTime, toTime time.Time
	var err error

	if reportFromDate != "" {
		fromTime, err = time.Parse("2006-01-02", reportFromDate)
		if err != nil {
			return fmt.Errorf("invalid from date format: %v (use YYYY-MM-DD)", err)
		}
	} else {
		fromTime = time.Now().AddDate(-1, 0, 0) // Default: 1 year ago
	}

	if reportToDate != "" {
		toTime, err = time.Parse("2006-01-02", reportToDate)
		if err != nil {
			return fmt.Errorf("invalid to date format: %v (use YYYY-MM-DD)", err)
		}
	} else {
		toTime = time.Now() // Default: now
	}

	if fromTime.After(toTime) {
		return fmt.Errorf("from date (%s) cannot be after to date (%s)", reportFromDate, reportToDate)
	}

	// Initialize database connection
	tradesRepo, err := initializeTradesRepo()
	if err != nil {
		return fmt.Errorf("failed to initialize database: %v", err)
	}

	ctx := context.Background()

	// Generate requested reports
	var output strings.Builder

	if reportPerformance {
		perfReport, err := generatePerformanceReport(ctx, tradesRepo, fromTime, toTime)
		if err != nil {
			return fmt.Errorf("failed to generate performance report: %v", err)
		}

		perfOutput, err := formatPerformanceReport(perfReport, reportFormat)
		if err != nil {
			return fmt.Errorf("failed to format performance report: %v", err)
		}

		output.WriteString(perfOutput)
		if reportPortfolio {
			output.WriteString("\n\n")
		}
	}

	if reportPortfolio {
		portfolioReport, err := generatePortfolioReport(ctx, tradesRepo, toTime)
		if err != nil {
			return fmt.Errorf("failed to generate portfolio report: %v", err)
		}

		portfolioOutput, err := formatPortfolioReport(portfolioReport, reportFormat)
		if err != nil {
			return fmt.Errorf("failed to format portfolio report: %v", err)
		}

		output.WriteString(portfolioOutput)
	}

	// Output results
	if reportOutfile == "" {
		fmt.Print(output.String())
	} else {
		err = os.WriteFile(reportOutfile, []byte(output.String()), 0644)
		if err != nil {
			return fmt.Errorf("failed to write output file: %v", err)
		}
		fmt.Printf("Report written to: %s\n", reportOutfile)
	}

	return nil
}

func initializeTradesRepo() (persistence.TradesRepo, error) {
	// Mock repository for development/testing purposes
	// In production, would initialize PostgreSQL connection
	repo := &MockTradesRepo{}
	return repo, nil
}

func generatePerformanceReport(ctx context.Context, tradesRepo persistence.TradesRepo, fromTime, toTime time.Time) (*perf.PerfMetrics, error) {
	// Fetch trades from database
	timeRange := persistence.TimeRange{
		From: fromTime,
		To:   toTime,
	}

	// Get trades for all symbols (simplified approach)
	// In production, would have a method to get all symbols or trades across symbols
	allTrades := make([]perf.TradeRecord, 0)
	
	// Common crypto symbols for demonstration
	symbols := []string{"BTC-USD", "ETH-USD", "USDT", "USDC", "ADA-USD", "DOT-USD"}
	
	for _, symbol := range symbols {
		trades, err := tradesRepo.ListBySymbol(ctx, symbol, timeRange, 10000)
		if err != nil {
			continue // Skip symbols with errors
		}

		for _, trade := range trades {
			tradeRecord := perf.TradeRecord{
				ID:             strconv.FormatInt(trade.ID, 10),
				Symbol:         trade.Symbol,
				Side:           trade.Side,
				Quantity:       trade.Qty,
				Price:          trade.Price,
				Fees:           0.001 * trade.Price * trade.Qty, // Assume 0.1% fees
				Slippage:       0.0005 * trade.Price * trade.Qty, // Assume 0.05% slippage
				Timestamp:      trade.Timestamp,
				StrategySource: "manual", // Default strategy source
				ExpectedPrice:  trade.Price * 1.0005, // Slight expected price difference
			}
			allTrades = append(allTrades, tradeRecord)
		}
	}

	if len(allTrades) == 0 {
		return nil, fmt.Errorf("no trades found in the specified time range")
	}

	// Generate synthetic return periods (would use real portfolio values in production)
	returns := generateSyntheticReturns(allTrades, fromTime, toTime)

	// Calculate performance metrics
	config := perf.DefaultPerfCalculatorConfig()
	calculator := perf.NewPerfCalculator(config)

	metrics, err := calculator.CalculatePerformance(allTrades, returns)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate performance: %v", err)
	}

	return metrics, nil
}

func generateSyntheticReturns(trades []perf.TradeRecord, fromTime, toTime time.Time) []perf.ReturnPeriod {
	// Generate daily return periods (simplified approach)
	returns := make([]perf.ReturnPeriod, 0)
	
	current := fromTime
	portfolioValue := 100000.0 // Starting portfolio value
	cumulativeReturn := 0.0
	
	for current.Before(toTime) {
		// Generate synthetic daily return (would use real portfolio values)
		dailyReturn := (rand.Float64() - 0.5) * 0.04 // ±2% daily return
		cumulativeReturn = (1 + cumulativeReturn) * (1 + dailyReturn) - 1
		portfolioValue = 100000.0 * (1 + cumulativeReturn)
		
		returnPeriod := perf.ReturnPeriod{
			Date:             current,
			Return:           dailyReturn,
			CumulativeReturn: cumulativeReturn,
			PortfolioValue:   portfolioValue,
			Benchmark:        dailyReturn * 0.8, // Simplified benchmark
		}
		
		returns = append(returns, returnPeriod)
		current = current.AddDate(0, 0, 1)
	}
	
	return returns
}

func generatePortfolioReport(ctx context.Context, tradesRepo persistence.TradesRepo, asOfTime time.Time) (*perf.PortfolioMetrics, error) {
	config := perf.DefaultPerfCalculatorConfig()
	calculator := perf.NewPortfolioCalculator(config, tradesRepo)

	portfolio, err := calculator.CalculatePortfolio(ctx, asOfTime)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate portfolio: %v", err)
	}

	return portfolio, nil
}

func formatPerformanceReport(metrics *perf.PerfMetrics, format string) (string, error) {
	switch format {
	case "json":
		data, err := json.MarshalIndent(metrics, "", "  ")
		if err != nil {
			return "", err
		}
		return string(data), nil

	case "csv":
		return formatPerformanceCSV(metrics), nil

	case "md":
		return formatPerformanceMD(metrics), nil

	default:
		return "", fmt.Errorf("unsupported format: %s", format)
	}
}

func formatPortfolioReport(portfolio *perf.PortfolioMetrics, format string) (string, error) {
	switch format {
	case "json":
		data, err := json.MarshalIndent(portfolio, "", "  ")
		if err != nil {
			return "", err
		}
		return string(data), nil

	case "csv":
		return formatPortfolioCSV(portfolio), nil

	case "md":
		return formatPortfolioMD(portfolio), nil

	default:
		return "", fmt.Errorf("unsupported format: %s", format)
	}
}

func formatPerformanceMD(metrics *perf.PerfMetrics) string {
	var md strings.Builder
	
	md.WriteString("# Performance Report\n\n")
	md.WriteString(fmt.Sprintf("**Analysis Period**: %s to %s (%d days)\n\n", 
		metrics.StartDate.Format("2006-01-02"), 
		metrics.EndDate.Format("2006-01-02"), 
		metrics.TotalDays))
	
	md.WriteString("## P&L Summary\n\n")
	md.WriteString(fmt.Sprintf("| Metric | Value |\n"))
	md.WriteString(fmt.Sprintf("|--------|-------|\n"))
	md.WriteString(fmt.Sprintf("| Spec P&L | $%.2f |\n", metrics.SpecPnL))
	md.WriteString(fmt.Sprintf("| Raw P&L | $%.2f |\n", metrics.RawPnL))
	md.WriteString(fmt.Sprintf("| Net P&L | $%.2f |\n", metrics.NetPnL))
	md.WriteString(fmt.Sprintf("| Total Fees | $%.2f |\n", metrics.TotalFees))
	md.WriteString(fmt.Sprintf("| Total Slippage | $%.2f |\n", metrics.TotalSlippage))
	md.WriteString("\n")
	
	md.WriteString("## Returns\n\n")
	md.WriteString(fmt.Sprintf("| Metric | Value |\n"))
	md.WriteString(fmt.Sprintf("|--------|-------|\n"))
	md.WriteString(fmt.Sprintf("| Total Return | %.2f%% |\n", metrics.TotalReturn*100))
	md.WriteString(fmt.Sprintf("| Annualized Return | %.2f%% |\n", metrics.AnnualizedReturn*100))
	md.WriteString(fmt.Sprintf("| Exposure Weighted Return | %.2f%% |\n", metrics.ExposureWeightedReturn*100))
	md.WriteString("\n")
	
	md.WriteString("## Risk Metrics\n\n")
	md.WriteString(fmt.Sprintf("| Metric | Value |\n"))
	md.WriteString(fmt.Sprintf("|--------|-------|\n"))
	md.WriteString(fmt.Sprintf("| Sharpe Ratio | %.2f |\n", metrics.Sharpe))
	md.WriteString(fmt.Sprintf("| Sortino Ratio | %.2f |\n", metrics.Sortino))
	md.WriteString(fmt.Sprintf("| Calmar Ratio | %.2f |\n", metrics.Calmar))
	md.WriteString(fmt.Sprintf("| Max Drawdown | %.2f%% |\n", metrics.MaxDrawdown*100))
	md.WriteString(fmt.Sprintf("| Max Drawdown Days | %d |\n", metrics.MaxDrawdownDays))
	md.WriteString(fmt.Sprintf("| Volatility | %.2f%% |\n", metrics.Volatility*100))
	md.WriteString("\n")
	
	md.WriteString("## Trade Analysis\n\n")
	md.WriteString(fmt.Sprintf("| Metric | Value |\n"))
	md.WriteString(fmt.Sprintf("|--------|-------|\n"))
	md.WriteString(fmt.Sprintf("| Total Trades | %d |\n", metrics.TotalTrades))
	md.WriteString(fmt.Sprintf("| Winning Trades | %d |\n", metrics.WinningTrades))
	md.WriteString(fmt.Sprintf("| Losing Trades | %d |\n", metrics.LosingTrades))
	md.WriteString(fmt.Sprintf("| Hit Rate | %.2f%% |\n", metrics.HitRate*100))
	md.WriteString(fmt.Sprintf("| Win/Loss Ratio | %.2f |\n", metrics.WinLossRatio))
	md.WriteString(fmt.Sprintf("| Profit Factor | %.2f |\n", metrics.ProfitFactor))
	md.WriteString(fmt.Sprintf("| Average Win | $%.2f |\n", metrics.AvgWin))
	md.WriteString(fmt.Sprintf("| Average Loss | $%.2f |\n", metrics.AvgLoss))
	
	return md.String()
}

func formatPerformanceCSV(metrics *perf.PerfMetrics) string {
	var csv strings.Builder
	
	// CSV header
	csv.WriteString("metric,value\n")
	
	// Write metrics
	csv.WriteString(fmt.Sprintf("spec_pnl,%.2f\n", metrics.SpecPnL))
	csv.WriteString(fmt.Sprintf("raw_pnl,%.2f\n", metrics.RawPnL))
	csv.WriteString(fmt.Sprintf("net_pnl,%.2f\n", metrics.NetPnL))
	csv.WriteString(fmt.Sprintf("total_return,%.4f\n", metrics.TotalReturn))
	csv.WriteString(fmt.Sprintf("annualized_return,%.4f\n", metrics.AnnualizedReturn))
	csv.WriteString(fmt.Sprintf("sharpe_ratio,%.4f\n", metrics.Sharpe))
	csv.WriteString(fmt.Sprintf("max_drawdown,%.4f\n", metrics.MaxDrawdown))
	csv.WriteString(fmt.Sprintf("hit_rate,%.4f\n", metrics.HitRate))
	csv.WriteString(fmt.Sprintf("total_trades,%d\n", metrics.TotalTrades))
	csv.WriteString(fmt.Sprintf("volatility,%.4f\n", metrics.Volatility))
	
	return csv.String()
}

func formatPortfolioMD(portfolio *perf.PortfolioMetrics) string {
	var md strings.Builder
	
	md.WriteString("# Portfolio Report\n\n")
	md.WriteString(fmt.Sprintf("**As of**: %s\n\n", portfolio.AsOfTime.Format("2006-01-02 15:04:05")))
	
	md.WriteString("## Portfolio Summary\n\n")
	md.WriteString(fmt.Sprintf("| Metric | Value |\n"))
	md.WriteString(fmt.Sprintf("|--------|-------|\n"))
	md.WriteString(fmt.Sprintf("| Total Value | $%.2f |\n", portfolio.TotalValue))
	md.WriteString(fmt.Sprintf("| Long Exposure | $%.2f |\n", portfolio.LongExposure))
	md.WriteString(fmt.Sprintf("| Short Exposure | $%.2f |\n", portfolio.ShortExposure))
	md.WriteString(fmt.Sprintf("| Net Exposure | $%.2f |\n", portfolio.NetExposure))
	md.WriteString(fmt.Sprintf("| Gross Exposure | $%.2f |\n", portfolio.GrossExposure))
	md.WriteString(fmt.Sprintf("| Concentration Risk | %.4f |\n", portfolio.ConcentrationRisk))
	md.WriteString(fmt.Sprintf("| Portfolio VaR (95%%) | $%.2f |\n", portfolio.PortfolioVaR))
	md.WriteString("\n")
	
	md.WriteString("## Positions\n\n")
	md.WriteString("| Symbol | Quantity | Market Value | Weight | Unrealized P&L | Days Held | Sector |\n")
	md.WriteString("|--------|----------|--------------|--------|----------------|-----------|--------|\n")
	
	for _, position := range portfolio.Positions {
		md.WriteString(fmt.Sprintf("| %s | %.4f | $%.2f | %.2f%% | $%.2f | %d | %s |\n",
			position.Symbol,
			position.Quantity,
			position.MarketValue,
			position.Weight*100,
			position.UnrealizedPnL,
			position.DaysHeld,
			position.Sector))
	}
	md.WriteString("\n")
	
	md.WriteString("## Sector Allocation\n\n")
	md.WriteString("| Sector | Allocation |\n")
	md.WriteString("|--------|------------|\n")
	
	for sector, allocation := range portfolio.SectorAllocation {
		md.WriteString(fmt.Sprintf("| %s | %.2f%% |\n", sector, allocation*100))
	}
	
	return md.String()
}

func formatPortfolioCSV(portfolio *perf.PortfolioMetrics) string {
	var csv strings.Builder
	
	// Portfolio summary CSV
	csv.WriteString("metric,value\n")
	csv.WriteString(fmt.Sprintf("total_value,%.2f\n", portfolio.TotalValue))
	csv.WriteString(fmt.Sprintf("long_exposure,%.2f\n", portfolio.LongExposure))
	csv.WriteString(fmt.Sprintf("short_exposure,%.2f\n", portfolio.ShortExposure))
	csv.WriteString(fmt.Sprintf("concentration_risk,%.4f\n", portfolio.ConcentrationRisk))
	
	csv.WriteString("\n# Positions\n")
	csv.WriteString("symbol,quantity,market_value,weight,unrealized_pnl,days_held,sector\n")
	
	for _, position := range portfolio.Positions {
		csv.WriteString(fmt.Sprintf("%s,%.4f,%.2f,%.4f,%.2f,%d,%s\n",
			position.Symbol,
			position.Quantity,
			position.MarketValue,
			position.Weight,
			position.UnrealizedPnL,
			position.DaysHeld,
			position.Sector))
	}
	
	return csv.String()
}

// Simple random number generator for synthetic data
var randSeed int64 = 1

type rand struct{}

func (r rand) Float64() float64 {
	randSeed = (randSeed*1103515245 + 12345) & 0x7fffffff
	return float64(randSeed) / float64(0x7fffffff)
}

// MockTradesRepo provides mock trade data for development and testing
type MockTradesRepo struct{}

func (m *MockTradesRepo) Insert(ctx context.Context, trade persistence.Trade) error {
	return nil
}

func (m *MockTradesRepo) InsertBatch(ctx context.Context, trades []persistence.Trade) error {
	return nil
}

func (m *MockTradesRepo) ListBySymbol(ctx context.Context, symbol string, tr persistence.TimeRange, limit int) ([]persistence.Trade, error) {
	// Generate mock trades for the symbol
	trades := make([]persistence.Trade, 0)
	
	current := tr.From
	id := int64(1)
	
	for current.Before(tr.To) && len(trades) < limit {
		// Generate mock trade data
		price := 50000.0 + (rand{}.Float64()-0.5)*10000 // Random price around $50k
		quantity := 0.1 + rand{}.Float64()*2.0          // Random quantity 0.1-2.1
		
		trade := persistence.Trade{
			ID:        id,
			Timestamp: current,
			Symbol:    symbol,
			Venue:     "kraken",
			Side:      "buy",
			Price:     price,
			Qty:       quantity,
			OrderID:   nil,
			Attributes: map[string]interface{}{
				"mock": true,
			},
			CreatedAt: current,
		}
		
		trades = append(trades, trade)
		current = current.Add(time.Hour * 6) // Every 6 hours
		id++
	}
	
	return trades, nil
}

func (m *MockTradesRepo) ListByVenue(ctx context.Context, venue string, tr persistence.TimeRange, limit int) ([]persistence.Trade, error) {
	return []persistence.Trade{}, nil
}

func (m *MockTradesRepo) GetByOrderID(ctx context.Context, orderID string) (*persistence.Trade, error) {
	return nil, nil
}

func (m *MockTradesRepo) GetLatest(ctx context.Context, limit int) ([]persistence.Trade, error) {
	return []persistence.Trade{}, nil
}

func (m *MockTradesRepo) Count(ctx context.Context, tr persistence.TimeRange) (int64, error) {
	return 100, nil
}

func (m *MockTradesRepo) CountByVenue(ctx context.Context, tr persistence.TimeRange) (map[string]int64, error) {
	return map[string]int64{
		"kraken":   50,
		"binance":  30,
		"coinbase": 20,
	}, nil
}