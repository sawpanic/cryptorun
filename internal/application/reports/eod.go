package reports

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

type EODReporter struct{}

type EODReport struct {
	Date           time.Time              `json:"date"`
	DecileLift     []DecileLiftMetric     `json:"decile_lift"`
	ExitDistrib    ExitDistribution       `json:"exit_distribution"`
	APIUsage       APIUsageSummary        `json:"api_usage"`
	CacheMetrics   CacheMetrics           `json:"cache_metrics"`
	VenueHealth    []VenueHealthMetric    `json:"venue_health"`
	RegimeAccuracy RegimeAccuracyMetric   `json:"regime_accuracy"`
	Summary        EODSummary             `json:"summary"`
}

type DecileLiftMetric struct {
	Decile       int     `json:"decile"`
	Count        int     `json:"count"`
	AvgScore     float64 `json:"avg_score"`
	AvgReturn    float64 `json:"avg_return_pct"`
	HitRate      float64 `json:"hit_rate_pct"`
	AvgWin       float64 `json:"avg_win_pct"`
	AvgLoss      float64 `json:"avg_loss_pct"`
	SharpeRatio  float64 `json:"sharpe_ratio"`
}

type ExitDistribution struct {
	TotalExits    int     `json:"total_exits"`
	TimeLimit     int     `json:"time_limit"`        // ≤40% target
	StopLoss      int     `json:"stop_loss"`         // ≤20% target  
	TakeProfit    int     `json:"take_profit"`
	VenueHealth   int     `json:"venue_health"`
	AccelReversal int     `json:"accel_reversal"`
	Fade          int     `json:"fade"`
	TimeLimitPct  float64 `json:"time_limit_pct"`
	StopLossPct   float64 `json:"stop_loss_pct"`
}

type APIUsageSummary struct {
	TotalRequests int                     `json:"total_requests"`
	ProviderUsage map[string]ProviderUsage `json:"provider_usage"`
	RateLimitHits int                     `json:"rate_limit_hits"`
	CircuitTrips  int                     `json:"circuit_trips"`
	BudgetAlerts  int                     `json:"budget_alerts"`
}

type ProviderUsage struct {
	Requests      int     `json:"requests"`
	BudgetUsedPct float64 `json:"budget_used_pct"`
	AvgLatencyMs  int     `json:"avg_latency_ms"`
	ErrorRate     float64 `json:"error_rate_pct"`
}

type CacheMetrics struct {
	HitRate       float64 `json:"hit_rate_pct"`
	MissCount     int     `json:"miss_count"`
	Evictions     int     `json:"evictions"`
	AvgTTL        int     `json:"avg_ttl_seconds"`
}

type VenueHealthMetric struct {
	Venue          string  `json:"venue"`
	UptimePct      float64 `json:"uptime_pct"`
	LatencyP99Ms   int     `json:"latency_p99_ms"`
	DataFreshness  int     `json:"data_freshness_seconds"`
	Status         string  `json:"status"`
}

type RegimeAccuracyMetric struct {
	Switches        int     `json:"regime_switches"`
	Accuracy        float64 `json:"accuracy_pct"`
	FalsePositives  int     `json:"false_positives"`
	VolatilityError float64 `json:"volatility_error_pct"`
}

type EODSummary struct {
	SignalsGenerated int     `json:"signals_generated"`
	TopDecileHitRate float64 `json:"top_decile_hit_rate"`
	SystemUptimePct  float64 `json:"system_uptime_pct"`
	OverallHealth    string  `json:"overall_health"`
}

func NewEODReporter() *EODReporter {
	return &EODReporter{}
}

func (eod *EODReporter) GenerateReport() (*EODReport, error) {
	date := time.Now().Truncate(24 * time.Hour)
	
	fmt.Println("📈 Generating EOD operational report...")
	
	report := &EODReport{
		Date:           date,
		DecileLift:     eod.calculateDecileLift(),
		ExitDistrib:    eod.analyzeExitDistribution(),
		APIUsage:       eod.summarizeAPIUsage(),
		CacheMetrics:   eod.collectCacheMetrics(),
		VenueHealth:    eod.assessVenueHealth(),
		RegimeAccuracy: eod.evaluateRegimeAccuracy(),
	}
	
	// Calculate summary metrics
	report.Summary = eod.calculateSummary(report)
	
	return report, nil
}

func (eod *EODReporter) WriteReportMarkdown(filePath string, report *EODReport) error {
	file, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("failed to create report file: %w", err)
	}
	defer file.Close()
	
	// Write markdown report
	fmt.Fprintf(file, "# CryptoRun EOD Report - %s\n\n", report.Date.Format("2006-01-02"))
	
	// Executive Summary
	fmt.Fprintf(file, "## Executive Summary\n\n")
	fmt.Fprintf(file, "- **Signals Generated**: %d\n", report.Summary.SignalsGenerated)
	fmt.Fprintf(file, "- **Top Decile Hit Rate**: %.1f%%\n", report.Summary.TopDecileHitRate)
	fmt.Fprintf(file, "- **System Uptime**: %.2f%%\n", report.Summary.SystemUptimePct)
	fmt.Fprintf(file, "- **Overall Health**: %s\n\n", report.Summary.OverallHealth)
	
	// Decile Performance
	fmt.Fprintf(file, "## Decile Lift Analysis\n\n")
	fmt.Fprintf(file, "| Decile | Count | Avg Score | Avg Return | Hit Rate | Sharpe |\n")
	fmt.Fprintf(file, "|--------|-------|-----------|------------|----------|--------|\n")
	for _, decile := range report.DecileLift {
		fmt.Fprintf(file, "| %d | %d | %.1f | %.2f%% | %.1f%% | %.2f |\n",
			decile.Decile, decile.Count, decile.AvgScore, decile.AvgReturn,
			decile.HitRate, decile.SharpeRatio)
	}
	fmt.Fprintf(file, "\n")
	
	// Exit Distribution
	fmt.Fprintf(file, "## Exit Distribution\n\n")
	fmt.Fprintf(file, "- **Time Limit**: %d (%.1f%%) - Target: ≤40%%\n", 
		report.ExitDistrib.TimeLimit, report.ExitDistrib.TimeLimitPct)
	fmt.Fprintf(file, "- **Stop Loss**: %d (%.1f%%) - Target: ≤20%%\n", 
		report.ExitDistrib.StopLoss, report.ExitDistrib.StopLossPct)
	fmt.Fprintf(file, "- **Take Profit**: %d\n", report.ExitDistrib.TakeProfit)
	fmt.Fprintf(file, "- **Venue Health**: %d\n", report.ExitDistrib.VenueHealth)
	fmt.Fprintf(file, "- **Other**: %d\n\n", report.ExitDistrib.AccelReversal+report.ExitDistrib.Fade)
	
	// API Usage
	fmt.Fprintf(file, "## API Usage Summary\n\n")
	fmt.Fprintf(file, "- **Total Requests**: %d\n", report.APIUsage.TotalRequests)
	fmt.Fprintf(file, "- **Rate Limit Hits**: %d\n", report.APIUsage.RateLimitHits)
	fmt.Fprintf(file, "- **Circuit Breaker Trips**: %d\n", report.APIUsage.CircuitTrips)
	fmt.Fprintf(file, "- **Budget Alerts**: %d\n\n", report.APIUsage.BudgetAlerts)
	
	// Provider Details
	fmt.Fprintf(file, "### Provider Usage\n\n")
	fmt.Fprintf(file, "| Provider | Requests | Budget Used | Avg Latency | Error Rate |\n")
	fmt.Fprintf(file, "|----------|----------|-------------|-------------|------------|\n")
	for provider, usage := range report.APIUsage.ProviderUsage {
		fmt.Fprintf(file, "| %s | %d | %.1f%% | %dms | %.2f%% |\n",
			provider, usage.Requests, usage.BudgetUsedPct, usage.AvgLatencyMs, usage.ErrorRate)
	}
	fmt.Fprintf(file, "\n")
	
	// System Metrics
	fmt.Fprintf(file, "## System Performance\n\n")
	fmt.Fprintf(file, "### Cache Metrics\n")
	fmt.Fprintf(file, "- **Hit Rate**: %.1f%% (Target: >85%%)\n", report.CacheMetrics.HitRate)
	fmt.Fprintf(file, "- **Misses**: %d\n", report.CacheMetrics.MissCount)
	fmt.Fprintf(file, "- **Evictions**: %d\n", report.CacheMetrics.Evictions)
	fmt.Fprintf(file, "- **Avg TTL**: %ds\n\n", report.CacheMetrics.AvgTTL)
	
	// Venue Health
	fmt.Fprintf(file, "### Venue Health\n\n")
	fmt.Fprintf(file, "| Venue | Uptime | P99 Latency | Freshness | Status |\n")
	fmt.Fprintf(file, "|-------|--------|-------------|-----------|--------|\n")
	for _, venue := range report.VenueHealth {
		fmt.Fprintf(file, "| %s | %.2f%% | %dms | %ds | %s |\n",
			venue.Venue, venue.UptimePct, venue.LatencyP99Ms, venue.DataFreshness, venue.Status)
	}
	fmt.Fprintf(file, "\n")
	
	// Regime Analysis
	fmt.Fprintf(file, "## Regime Detection Performance\n\n")
	fmt.Fprintf(file, "- **Switches**: %d\n", report.RegimeAccuracy.Switches)
	fmt.Fprintf(file, "- **Accuracy**: %.1f%%\n", report.RegimeAccuracy.Accuracy)
	fmt.Fprintf(file, "- **False Positives**: %d (Target: <20%%)\n", report.RegimeAccuracy.FalsePositives)
	fmt.Fprintf(file, "- **Volatility Error**: %.2f%%\n\n", report.RegimeAccuracy.VolatilityError)
	
	fmt.Fprintf(file, "---\n")
	fmt.Fprintf(file, "*Generated at %s*\n", time.Now().Format("2006-01-02 15:04:05"))
	
	return nil
}

func (eod *EODReporter) WriteCSVReports(baseDir string, report *EODReport) error {
	// Write decile lift CSV
	decileFile := filepath.Join(baseDir, "decile_lift.csv")
	if err := eod.writeDecileLiftCSV(decileFile, report.DecileLift); err != nil {
		return err
	}
	
	// Write exit distribution CSV
	exitFile := filepath.Join(baseDir, "exits.csv")
	if err := eod.writeExitDistributionCSV(exitFile, &report.ExitDistrib); err != nil {
		return err
	}
	
	// Write API usage CSV
	apiFile := filepath.Join(baseDir, "api_usage.csv")
	if err := eod.writeAPIUsageCSV(apiFile, &report.APIUsage); err != nil {
		return err
	}
	
	return nil
}

func (eod *EODReporter) calculateDecileLift() []DecileLiftMetric {
	// Mock decile analysis - in production would analyze actual signal performance
	deciles := make([]DecileLiftMetric, 10)
	for i := 0; i < 10; i++ {
		decile := i + 1
		deciles[i] = DecileLiftMetric{
			Decile:      decile,
			Count:       50 - i*3,                    // Fewer signals in lower deciles
			AvgScore:    95.0 - float64(i*5),         // Descending scores
			AvgReturn:   8.0 - float64(i)*0.8,        // Descending returns
			HitRate:     85.0 - float64(i)*8,         // Descending hit rates
			AvgWin:      12.0 - float64(i)*0.5,       // Descending avg wins
			AvgLoss:     -4.5 - float64(i)*0.2,       // Worsening avg losses
			SharpeRatio: 2.1 - float64(i)*0.2,        // Descending Sharpe ratios
		}
	}
	return deciles
}

func (eod *EODReporter) analyzeExitDistribution() ExitDistribution {
	// Mock exit analysis - in production would analyze actual exit reasons
	totalExits := 120
	timeLimit := 45    // 37.5% - within 40% target
	stopLoss := 18     // 15% - within 20% target
	takeProfit := 38
	venueHealth := 12
	accelReversal := 4
	fade := 3
	
	return ExitDistribution{
		TotalExits:    totalExits,
		TimeLimit:     timeLimit,
		StopLoss:      stopLoss,
		TakeProfit:    takeProfit,
		VenueHealth:   venueHealth,
		AccelReversal: accelReversal,
		Fade:          fade,
		TimeLimitPct:  float64(timeLimit) / float64(totalExits) * 100,
		StopLossPct:   float64(stopLoss) / float64(totalExits) * 100,
	}
}

func (eod *EODReporter) summarizeAPIUsage() APIUsageSummary {
	// Mock API usage analysis
	return APIUsageSummary{
		TotalRequests: 8742,
		ProviderUsage: map[string]ProviderUsage{
			"binance": {
				Requests:      3240,
				BudgetUsedPct: 32.4,
				AvgLatencyMs:  145,
				ErrorRate:     1.2,
			},
			"kraken": {
				Requests:      2180,
				BudgetUsedPct: 43.6,
				AvgLatencyMs:  234,
				ErrorRate:     0.8,
			},
			"coingecko": {
				Requests:      1890,
				BudgetUsedPct: 18.9,
				AvgLatencyMs:  312,
				ErrorRate:     2.1,
			},
			"moralis": {
				Requests:      1432,
				BudgetUsedPct: 35.8,
				AvgLatencyMs:  189,
				ErrorRate:     1.5,
			},
		},
		RateLimitHits: 12,
		CircuitTrips:  2,
		BudgetAlerts:  1,
	}
}

func (eod *EODReporter) collectCacheMetrics() CacheMetrics {
	return CacheMetrics{
		HitRate:   87.3,
		MissCount: 1108,
		Evictions: 234,
		AvgTTL:    127,
	}
}

func (eod *EODReporter) assessVenueHealth() []VenueHealthMetric {
	return []VenueHealthMetric{
		{
			Venue:          "kraken",
			UptimePct:      99.2,
			LatencyP99Ms:   245,
			DataFreshness:  12,
			Status:         "HEALTHY",
		},
		{
			Venue:          "binance",
			UptimePct:      98.8,
			LatencyP99Ms:   156,
			DataFreshness:  8,
			Status:         "HEALTHY",
		},
		{
			Venue:          "coingecko",
			UptimePct:      97.5,
			LatencyP99Ms:   387,
			DataFreshness:  45,
			Status:         "DEGRADED",
		},
		{
			Venue:          "moralis",
			UptimePct:      98.1,
			LatencyP99Ms:   203,
			DataFreshness:  23,
			Status:         "HEALTHY",
		},
	}
}

func (eod *EODReporter) evaluateRegimeAccuracy() RegimeAccuracyMetric {
	return RegimeAccuracyMetric{
		Switches:        6,
		Accuracy:        78.3,
		FalsePositives:  8,  // <20% target
		VolatilityError: 12.7,
	}
}

func (eod *EODReporter) calculateSummary(report *EODReport) EODSummary {
	totalSignals := 0
	for _, decile := range report.DecileLift {
		totalSignals += decile.Count
	}
	
	topDecileHitRate := 0.0
	if len(report.DecileLift) > 0 {
		topDecileHitRate = report.DecileLift[0].HitRate
	}
	
	// Calculate system uptime from venue health
	avgUptime := 0.0
	for _, venue := range report.VenueHealth {
		avgUptime += venue.UptimePct
	}
	avgUptime /= float64(len(report.VenueHealth))
	
	overallHealth := "🟢 EXCELLENT"
	if avgUptime < 95.0 || report.CacheMetrics.HitRate < 80.0 {
		overallHealth = "🟡 GOOD"
	}
	if avgUptime < 90.0 || report.APIUsage.CircuitTrips > 5 {
		overallHealth = "🔴 DEGRADED"
	}
	
	return EODSummary{
		SignalsGenerated: totalSignals,
		TopDecileHitRate: topDecileHitRate,
		SystemUptimePct:  avgUptime,
		OverallHealth:    overallHealth,
	}
}

// CSV writing helper functions
func (eod *EODReporter) writeDecileLiftCSV(filePath string, deciles []DecileLiftMetric) error {
	file, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer file.Close()
	
	writer := csv.NewWriter(file)
	defer writer.Flush()
	
	// Header
	header := []string{"Decile", "Count", "AvgScore", "AvgReturn", "HitRate", "AvgWin", "AvgLoss", "SharpeRatio"}
	writer.Write(header)
	
	// Data rows
	for _, decile := range deciles {
		row := []string{
			strconv.Itoa(decile.Decile),
			strconv.Itoa(decile.Count),
			fmt.Sprintf("%.1f", decile.AvgScore),
			fmt.Sprintf("%.2f", decile.AvgReturn),
			fmt.Sprintf("%.1f", decile.HitRate),
			fmt.Sprintf("%.2f", decile.AvgWin),
			fmt.Sprintf("%.2f", decile.AvgLoss),
			fmt.Sprintf("%.2f", decile.SharpeRatio),
		}
		writer.Write(row)
	}
	
	return nil
}

func (eod *EODReporter) writeExitDistributionCSV(filePath string, exits *ExitDistribution) error {
	file, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer file.Close()
	
	writer := csv.NewWriter(file)
	defer writer.Flush()
	
	header := []string{"ExitType", "Count", "Percentage"}
	writer.Write(header)
	
	exitTypes := [][]string{
		{"TimeLimit", strconv.Itoa(exits.TimeLimit), fmt.Sprintf("%.1f", exits.TimeLimitPct)},
		{"StopLoss", strconv.Itoa(exits.StopLoss), fmt.Sprintf("%.1f", exits.StopLossPct)},
		{"TakeProfit", strconv.Itoa(exits.TakeProfit), fmt.Sprintf("%.1f", float64(exits.TakeProfit)/float64(exits.TotalExits)*100)},
		{"VenueHealth", strconv.Itoa(exits.VenueHealth), fmt.Sprintf("%.1f", float64(exits.VenueHealth)/float64(exits.TotalExits)*100)},
		{"AccelReversal", strconv.Itoa(exits.AccelReversal), fmt.Sprintf("%.1f", float64(exits.AccelReversal)/float64(exits.TotalExits)*100)},
		{"Fade", strconv.Itoa(exits.Fade), fmt.Sprintf("%.1f", float64(exits.Fade)/float64(exits.TotalExits)*100)},
	}
	
	for _, row := range exitTypes {
		writer.Write(row)
	}
	
	return nil
}

func (eod *EODReporter) writeAPIUsageCSV(filePath string, api *APIUsageSummary) error {
	file, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer file.Close()
	
	writer := csv.NewWriter(file)
	defer writer.Flush()
	
	header := []string{"Provider", "Requests", "BudgetUsedPct", "AvgLatencyMs", "ErrorRate"}
	writer.Write(header)
	
	for provider, usage := range api.ProviderUsage {
		row := []string{
			provider,
			strconv.Itoa(usage.Requests),
			fmt.Sprintf("%.1f", usage.BudgetUsedPct),
			strconv.Itoa(usage.AvgLatencyMs),
			fmt.Sprintf("%.2f", usage.ErrorRate),
		}
		writer.Write(row)
	}
	
	return nil
}