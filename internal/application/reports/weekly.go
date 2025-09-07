package reports

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

type WeeklyReporter struct{}

type WeeklyReport struct {
	WeekStart      time.Time               `json:"week_start"`
	WeekEnd        time.Time               `json:"week_end"`
	Performance    WeeklyPerformance       `json:"performance"`
	SystemHealth   WeeklySystemHealth      `json:"system_health"`
	MarketAnalysis WeeklyMarketAnalysis    `json:"market_analysis"`
	Recommendations []string               `json:"recommendations"`
	KPIs           WeeklyKPIs             `json:"kpis"`
}

type WeeklyPerformance struct {
	TotalSignals      int     `json:"total_signals"`
	WinRate           float64 `json:"win_rate_pct"`
	AvgReturn         float64 `json:"avg_return_pct"`
	BestDay           string  `json:"best_day"`
	WorstDay          string  `json:"worst_day"`
	CumulativeReturn  float64 `json:"cumulative_return_pct"`
	MaxDrawdown       float64 `json:"max_drawdown_pct"`
	SharpeRatio       float64 `json:"sharpe_ratio"`
	SortinoRatio      float64 `json:"sortino_ratio"`
	CalmarRatio       float64 `json:"calmar_ratio"`
	VaR95             float64 `json:"var_95_pct"`
}

type WeeklySystemHealth struct {
	AvgUptime         float64            `json:"avg_uptime_pct"`
	CacheHitRate      float64            `json:"cache_hit_rate_pct"`
	APIReliability    float64            `json:"api_reliability_pct"`
	LatencyP99        int                `json:"latency_p99_ms"`
	ProviderIncidents map[string]int     `json:"provider_incidents"`
	CircuitTripCount  int                `json:"circuit_trip_count"`
	BudgetViolations  int                `json:"budget_violations"`
}

type WeeklyMarketAnalysis struct {
	DominantRegime     string               `json:"dominant_regime"`
	RegimeSwitches     int                  `json:"regime_switches"`
	VolatilityProfile  string               `json:"volatility_profile"`
	MarketBreadth      float64              `json:"market_breadth_pct"`
	TrendStrength      float64              `json:"trend_strength"`
	CorrelationMatrix  map[string]float64   `json:"correlation_matrix"`
	SectorPerformance  map[string]float64   `json:"sector_performance"`
}

type WeeklyKPIs struct {
	SignalGeneration  KPIMetric  `json:"signal_generation"`
	ExitEfficiency    KPIMetric  `json:"exit_efficiency"`
	RiskManagement    KPIMetric  `json:"risk_management"`
	SystemReliability KPIMetric  `json:"system_reliability"`
	DataQuality       KPIMetric  `json:"data_quality"`
}

type KPIMetric struct {
	Current float64 `json:"current"`
	Target  float64 `json:"target"`
	Status  string  `json:"status"` // EXCELLENT, GOOD, NEEDS_IMPROVEMENT, CRITICAL
	Trend   string  `json:"trend"`  // IMPROVING, STABLE, DECLINING
}

func NewWeeklyReporter() *WeeklyReporter {
	return &WeeklyReporter{}
}

func (wr *WeeklyReporter) GenerateReport() (*WeeklyReport, error) {
	now := time.Now()
	weekStart := now.AddDate(0, 0, -7)
	
	fmt.Println("📊 Generating weekly operational report...")
	
	report := &WeeklyReport{
		WeekStart:       weekStart,
		WeekEnd:         now,
		Performance:     wr.analyzeWeeklyPerformance(),
		SystemHealth:    wr.assessWeeklySystemHealth(),
		MarketAnalysis:  wr.conductMarketAnalysis(),
		KPIs:           wr.calculateWeeklyKPIs(),
	}
	
	// Generate recommendations based on analysis
	report.Recommendations = wr.generateRecommendations(report)
	
	return report, nil
}

func (wr *WeeklyReporter) WriteReportMarkdown(filePath string, report *WeeklyReport) error {
	file, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("failed to create weekly report file: %w", err)
	}
	defer file.Close()
	
	// Write comprehensive weekly report
	fmt.Fprintf(file, "# CryptoRun Weekly Report\n")
	fmt.Fprintf(file, "**Period**: %s to %s\n\n", 
		report.WeekStart.Format("2006-01-02"), 
		report.WeekEnd.Format("2006-01-02"))
	
	// Executive Summary
	fmt.Fprintf(file, "## Executive Summary\n\n")
	fmt.Fprintf(file, "- **Total Signals**: %d\n", report.Performance.TotalSignals)
	fmt.Fprintf(file, "- **Win Rate**: %.1f%%\n", report.Performance.WinRate)
	fmt.Fprintf(file, "- **Cumulative Return**: %.2f%%\n", report.Performance.CumulativeReturn)
	fmt.Fprintf(file, "- **Sharpe Ratio**: %.2f\n", report.Performance.SharpeRatio)
	fmt.Fprintf(file, "- **System Uptime**: %.2f%%\n\n", report.SystemHealth.AvgUptime)
	
	// Performance Analysis
	fmt.Fprintf(file, "## Performance Analysis\n\n")
	fmt.Fprintf(file, "### Trading Metrics\n")
	fmt.Fprintf(file, "- **Average Return**: %.2f%%\n", report.Performance.AvgReturn)
	fmt.Fprintf(file, "- **Best Day**: %s\n", report.Performance.BestDay)
	fmt.Fprintf(file, "- **Worst Day**: %s\n", report.Performance.WorstDay)
	fmt.Fprintf(file, "- **Maximum Drawdown**: %.2f%%\n", report.Performance.MaxDrawdown)
	fmt.Fprintf(file, "- **VaR (95%%)**: %.2f%%\n\n", report.Performance.VaR95)
	
	fmt.Fprintf(file, "### Risk-Adjusted Returns\n")
	fmt.Fprintf(file, "- **Sharpe Ratio**: %.2f\n", report.Performance.SharpeRatio)
	fmt.Fprintf(file, "- **Sortino Ratio**: %.2f\n", report.Performance.SortinoRatio)
	fmt.Fprintf(file, "- **Calmar Ratio**: %.2f\n\n", report.Performance.CalmarRatio)
	
	// System Health
	fmt.Fprintf(file, "## System Health\n\n")
	fmt.Fprintf(file, "### Infrastructure Metrics\n")
	fmt.Fprintf(file, "- **Average Uptime**: %.2f%%\n", report.SystemHealth.AvgUptime)
	fmt.Fprintf(file, "- **Cache Hit Rate**: %.1f%% (Target: >85%%)\n", report.SystemHealth.CacheHitRate)
	fmt.Fprintf(file, "- **API Reliability**: %.1f%%\n", report.SystemHealth.APIReliability)
	fmt.Fprintf(file, "- **P99 Latency**: %dms (Target: <300ms)\n\n", report.SystemHealth.LatencyP99)
	
	fmt.Fprintf(file, "### Provider Incidents\n")
	for provider, incidents := range report.SystemHealth.ProviderIncidents {
		fmt.Fprintf(file, "- **%s**: %d incidents\n", provider, incidents)
	}
	fmt.Fprintf(file, "- **Circuit Breaker Trips**: %d\n", report.SystemHealth.CircuitTripCount)
	fmt.Fprintf(file, "- **Budget Violations**: %d\n\n", report.SystemHealth.BudgetViolations)
	
	// Market Analysis
	fmt.Fprintf(file, "## Market Analysis\n\n")
	fmt.Fprintf(file, "### Regime & Volatility\n")
	fmt.Fprintf(file, "- **Dominant Regime**: %s\n", report.MarketAnalysis.DominantRegime)
	fmt.Fprintf(file, "- **Regime Switches**: %d\n", report.MarketAnalysis.RegimeSwitches)
	fmt.Fprintf(file, "- **Volatility Profile**: %s\n", report.MarketAnalysis.VolatilityProfile)
	fmt.Fprintf(file, "- **Market Breadth**: %.1f%%\n", report.MarketAnalysis.MarketBreadth)
	fmt.Fprintf(file, "- **Trend Strength**: %.1f\n\n", report.MarketAnalysis.TrendStrength)
	
	fmt.Fprintf(file, "### Sector Performance\n")
	for sector, performance := range report.MarketAnalysis.SectorPerformance {
		fmt.Fprintf(file, "- **%s**: %.2f%%\n", sector, performance)
	}
	fmt.Fprintf(file, "\n")
	
	// KPIs Dashboard
	fmt.Fprintf(file, "## KPI Dashboard\n\n")
	fmt.Fprintf(file, "| KPI Category | Current | Target | Status | Trend |\n")
	fmt.Fprintf(file, "|--------------|---------|--------|--------|-------|\n")
	
	kpiCategories := map[string]KPIMetric{
		"Signal Generation":  report.KPIs.SignalGeneration,
		"Exit Efficiency":    report.KPIs.ExitEfficiency,
		"Risk Management":    report.KPIs.RiskManagement,
		"System Reliability": report.KPIs.SystemReliability,
		"Data Quality":       report.KPIs.DataQuality,
	}
	
	for category, kpi := range kpiCategories {
		status := wr.formatKPIStatus(kpi.Status)
		trend := wr.formatTrend(kpi.Trend)
		fmt.Fprintf(file, "| %s | %.1f | %.1f | %s | %s |\n", 
			category, kpi.Current, kpi.Target, status, trend)
	}
	fmt.Fprintf(file, "\n")
	
	// Recommendations
	fmt.Fprintf(file, "## Recommendations\n\n")
	for i, rec := range report.Recommendations {
		fmt.Fprintf(file, "%d. %s\n", i+1, rec)
	}
	fmt.Fprintf(file, "\n")
	
	fmt.Fprintf(file, "---\n")
	fmt.Fprintf(file, "*Generated at %s*\n", time.Now().Format("2006-01-02 15:04:05"))
	
	return nil
}

func (wr *WeeklyReporter) WriteCSVReports(baseDir string, report *WeeklyReport) error {
	// Write performance summary CSV
	perfFile := filepath.Join(baseDir, "weekly_performance.csv")
	if err := wr.writePerformanceCSV(perfFile, &report.Performance); err != nil {
		return err
	}
	
	// Write KPI tracking CSV
	kpiFile := filepath.Join(baseDir, "kpi_tracking.csv")
	if err := wr.writeKPITrackingCSV(kpiFile, &report.KPIs); err != nil {
		return err
	}
	
	// Write market analysis CSV
	marketFile := filepath.Join(baseDir, "market_analysis.csv")
	if err := wr.writeMarketAnalysisCSV(marketFile, &report.MarketAnalysis); err != nil {
		return err
	}
	
	return nil
}

func (wr *WeeklyReporter) analyzeWeeklyPerformance() WeeklyPerformance {
	// Mock weekly performance analysis
	return WeeklyPerformance{
		TotalSignals:     287,
		WinRate:          64.8,
		AvgReturn:        5.7,
		BestDay:          "2025-09-05 (+18.3%)",
		WorstDay:         "2025-09-03 (-8.1%)",
		CumulativeReturn: 23.4,
		MaxDrawdown:      -12.7,
		SharpeRatio:      1.83,
		SortinoRatio:     2.41,
		CalmarRatio:      1.84,
		VaR95:            -8.3,
	}
}

func (wr *WeeklyReporter) assessWeeklySystemHealth() WeeklySystemHealth {
	return WeeklySystemHealth{
		AvgUptime:      98.7,
		CacheHitRate:   89.2,
		APIReliability: 97.3,
		LatencyP99:     267,
		ProviderIncidents: map[string]int{
			"binance":   2,
			"kraken":    1,
			"coingecko": 4,
			"moralis":   3,
		},
		CircuitTripCount: 8,
		BudgetViolations: 2,
	}
}

func (wr *WeeklyReporter) conductMarketAnalysis() WeeklyMarketAnalysis {
	return WeeklyMarketAnalysis{
		DominantRegime:    "TRENDING",
		RegimeSwitches:    12,
		VolatilityProfile: "ELEVATED",
		MarketBreadth:     67.3,
		TrendStrength:     0.72,
		CorrelationMatrix: map[string]float64{
			"BTC-ETH":   0.89,
			"BTC-ALT":   0.74,
			"ETH-ALT":   0.82,
			"USD-STBL":  0.95,
		},
		SectorPerformance: map[string]float64{
			"Layer1":       12.8,
			"DeFi":         -3.2,
			"Infrastructure": 8.9,
			"Meme":         -15.7,
			"AI":           21.3,
			"Gaming":       -1.8,
		},
	}
}

func (wr *WeeklyReporter) calculateWeeklyKPIs() WeeklyKPIs {
	return WeeklyKPIs{
		SignalGeneration: KPIMetric{
			Current: 287,
			Target:  250,
			Status:  "EXCELLENT",
			Trend:   "IMPROVING",
		},
		ExitEfficiency: KPIMetric{
			Current: 78.3, // % of optimal exits
			Target:  75.0,
			Status:  "GOOD",
			Trend:   "STABLE",
		},
		RiskManagement: KPIMetric{
			Current: 12.7, // Max drawdown %
			Target:  15.0,
			Status:  "GOOD",
			Trend:   "IMPROVING",
		},
		SystemReliability: KPIMetric{
			Current: 98.7, // Uptime %
			Target:  99.0,
			Status:  "GOOD",
			Trend:   "STABLE",
		},
		DataQuality: KPIMetric{
			Current: 89.2, // Cache hit rate %
			Target:  85.0,
			Status:  "EXCELLENT",
			Trend:   "IMPROVING",
		},
	}
}

func (wr *WeeklyReporter) generateRecommendations(report *WeeklyReport) []string {
	recommendations := []string{}
	
	// Performance-based recommendations
	if report.Performance.WinRate < 60.0 {
		recommendations = append(recommendations, "Consider tightening entry criteria - win rate below 60% target")
	}
	
	if report.Performance.MaxDrawdown < -15.0 {
		recommendations = append(recommendations, "Review position sizing - drawdown exceeds -15% threshold")
	}
	
	// System health recommendations
	if report.SystemHealth.CacheHitRate < 85.0 {
		recommendations = append(recommendations, "Optimize caching strategy - hit rate below 85% target")
	}
	
	if report.SystemHealth.LatencyP99 > 300 {
		recommendations = append(recommendations, "Investigate P99 latency - exceeds 300ms target")
	}
	
	// Market-based recommendations
	if report.MarketAnalysis.RegimeSwitches > 20 {
		recommendations = append(recommendations, "High regime instability detected - consider adaptive position sizing")
	}
	
	// Provider-based recommendations
	totalIncidents := 0
	worstProvider := ""
	maxIncidents := 0
	for provider, incidents := range report.SystemHealth.ProviderIncidents {
		totalIncidents += incidents
		if incidents > maxIncidents {
			maxIncidents = incidents
			worstProvider = provider
		}
	}
	
	if totalIncidents > 15 {
		recommendations = append(recommendations, fmt.Sprintf("Review %s provider reliability - highest incident count", worstProvider))
	}
	
	// Default recommendation if all good
	if len(recommendations) == 0 {
		recommendations = append(recommendations, "System performing within all target parameters - maintain current configuration")
	}
	
	return recommendations
}

func (wr *WeeklyReporter) formatKPIStatus(status string) string {
	switch status {
	case "EXCELLENT":
		return "🟢 EXCELLENT"
	case "GOOD":
		return "🟡 GOOD"
	case "NEEDS_IMPROVEMENT":
		return "🟠 NEEDS IMPROVEMENT"
	case "CRITICAL":
		return "🔴 CRITICAL"
	default:
		return status
	}
}

func (wr *WeeklyReporter) formatTrend(trend string) string {
	switch trend {
	case "IMPROVING":
		return "📈 IMPROVING"
	case "STABLE":
		return "➡️ STABLE"
	case "DECLINING":
		return "📉 DECLINING"
	default:
		return trend
	}
}

// CSV helper functions
func (wr *WeeklyReporter) writePerformanceCSV(filePath string, perf *WeeklyPerformance) error {
	file, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer file.Close()
	
	writer := csv.NewWriter(file)
	defer writer.Flush()
	
	// Write performance metrics
	rows := [][]string{
		{"Metric", "Value"},
		{"TotalSignals", strconv.Itoa(perf.TotalSignals)},
		{"WinRate", fmt.Sprintf("%.1f", perf.WinRate)},
		{"AvgReturn", fmt.Sprintf("%.2f", perf.AvgReturn)},
		{"CumulativeReturn", fmt.Sprintf("%.2f", perf.CumulativeReturn)},
		{"MaxDrawdown", fmt.Sprintf("%.2f", perf.MaxDrawdown)},
		{"SharpeRatio", fmt.Sprintf("%.2f", perf.SharpeRatio)},
		{"SortinoRatio", fmt.Sprintf("%.2f", perf.SortinoRatio)},
		{"CalmarRatio", fmt.Sprintf("%.2f", perf.CalmarRatio)},
		{"VaR95", fmt.Sprintf("%.2f", perf.VaR95)},
	}
	
	for _, row := range rows {
		writer.Write(row)
	}
	
	return nil
}

func (wr *WeeklyReporter) writeKPITrackingCSV(filePath string, kpis *WeeklyKPIs) error {
	file, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer file.Close()
	
	writer := csv.NewWriter(file)
	defer writer.Flush()
	
	header := []string{"KPI", "Current", "Target", "Status", "Trend"}
	writer.Write(header)
	
	kpiData := [][]string{
		{"SignalGeneration", fmt.Sprintf("%.1f", kpis.SignalGeneration.Current), fmt.Sprintf("%.1f", kpis.SignalGeneration.Target), kpis.SignalGeneration.Status, kpis.SignalGeneration.Trend},
		{"ExitEfficiency", fmt.Sprintf("%.1f", kpis.ExitEfficiency.Current), fmt.Sprintf("%.1f", kpis.ExitEfficiency.Target), kpis.ExitEfficiency.Status, kpis.ExitEfficiency.Trend},
		{"RiskManagement", fmt.Sprintf("%.1f", kpis.RiskManagement.Current), fmt.Sprintf("%.1f", kpis.RiskManagement.Target), kpis.RiskManagement.Status, kpis.RiskManagement.Trend},
		{"SystemReliability", fmt.Sprintf("%.1f", kpis.SystemReliability.Current), fmt.Sprintf("%.1f", kpis.SystemReliability.Target), kpis.SystemReliability.Status, kpis.SystemReliability.Trend},
		{"DataQuality", fmt.Sprintf("%.1f", kpis.DataQuality.Current), fmt.Sprintf("%.1f", kpis.DataQuality.Target), kpis.DataQuality.Status, kpis.DataQuality.Trend},
	}
	
	for _, row := range kpiData {
		writer.Write(row)
	}
	
	return nil
}

func (wr *WeeklyReporter) writeMarketAnalysisCSV(filePath string, market *WeeklyMarketAnalysis) error {
	file, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer file.Close()
	
	writer := csv.NewWriter(file)
	defer writer.Flush()
	
	// Write market analysis metrics
	rows := [][]string{
		{"Metric", "Value"},
		{"DominantRegime", market.DominantRegime},
		{"RegimeSwitches", strconv.Itoa(market.RegimeSwitches)},
		{"VolatilityProfile", market.VolatilityProfile},
		{"MarketBreadth", fmt.Sprintf("%.1f", market.MarketBreadth)},
		{"TrendStrength", fmt.Sprintf("%.2f", market.TrendStrength)},
	}
	
	for _, row := range rows {
		writer.Write(row)
	}
	
	// Add sector performance
	writer.Write([]string{"", ""}) // Empty row
	writer.Write([]string{"Sector", "Performance"})
	
	for sector, perf := range market.SectorPerformance {
		writer.Write([]string{sector, fmt.Sprintf("%.2f", perf)})
	}
	
	return nil
}