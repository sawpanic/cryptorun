package unit

import (
	"context"
	"testing"

	"cryptorun/internal/bench/diagnostics"
)

func TestDiagnosticsUsesSpecCompliantPnL(t *testing.T) {
	config := diagnostics.DiagnosticsConfig{
		MinSampleSize:      20,
		SeriesSource:       "exchange_native_first",
		OutputDir:          "out/test/diagnostics",
		ExchangeNativeOnly: false,
		RegimeAware:        true,
	}

	analyzer := diagnostics.NewDiagnosticsAnalyzer(config)

	// Create test data with known raw 24h gains
	topGainers := map[string][]diagnostics.TopGainer{
		"24h": {
			{Symbol: "ETH", PercentageFloat: 42.8}, // Raw market gain
			{Symbol: "SOL", PercentageFloat: 38.4}, // Raw market gain
			{Symbol: "BTC", PercentageFloat: 25.0}, // Raw market gain
		},
	}

	scanResults := map[string][]string{
		"24h": {"BTC"}, // Only BTC in our scan results
	}

	ctx := context.Background()
	report, err := analyzer.AnalyzeBenchmarkResults(ctx, topGainers, scanResults)
	if err != nil {
		t.Fatalf("Analysis failed: %v", err)
	}

	// Test 1: ETH and SOL should be misses with spec-compliant P&L
	window24h := report.WindowAnalysis["24h"]

	if len(window24h.Misses) < 2 {
		t.Errorf("Expected at least 2 misses (ETH, SOL), got %d", len(window24h.Misses))
	}

	// Test 2: Spec-compliant P&L should be lower than raw gains
	totalSpecGains := window24h.SpecCompliantGains
	totalRawGains := window24h.RawMarketGains

	if totalSpecGains >= totalRawGains {
		t.Errorf("Spec-compliant gains (%.2f) should be less than raw market gains (%.2f)", totalSpecGains, totalRawGains)
	}

	// Test 3: Performance summary should use spec-compliant P&L for missed opportunities
	if report.PerformanceSummary.TotalSpecCompliantGainsMissed == report.PerformanceSummary.TotalRawMarketGainsMissed {
		t.Error("Performance summary should distinguish between spec-compliant and raw market gains")
	}

	// Test 4: Each diagnostic result should have spec-compliant P&L calculated
	for _, miss := range window24h.Misses {
		if miss.SpecCompliantPnL == nil {
			t.Errorf("Miss for %s should have spec-compliant P&L calculated", miss.Symbol)
		}

		// Spec-compliant P&L should be lower than raw gain (due to entry/exit timing)
		if miss.SpecCompliantPnL != nil && *miss.SpecCompliantPnL >= miss.RawGainPercent {
			t.Errorf("Spec-compliant P&L (%.2f) should be less than raw gain (%.2f) for %s",
				*miss.SpecCompliantPnL, miss.RawGainPercent, miss.Symbol)
		}
	}

	// Test 5: CRITICAL - Should not use raw 24h change for tuning decisions
	if report.ActionableInsights != nil {
		for _, improvement := range report.ActionableInsights.TopImprovements {
			// Check that improvement doesn't mention raw percentage values like 42.8% or 38.4%
			if containsRawPercentages(improvement.Impact) {
				t.Errorf("Actionable insight should not reference raw 24h percentages: %s", improvement.Impact)
			}
		}
	}
}

func TestSampleSizeRequirementEnforcement(t *testing.T) {
	config := diagnostics.DiagnosticsConfig{
		MinSampleSize: 20,
	}

	analyzer := diagnostics.NewDiagnosticsAnalyzer(config)

	// Test with insufficient sample size (n<20)
	smallSample := map[string][]diagnostics.TopGainer{
		"1h": make([]diagnostics.TopGainer, 15), // Only 15 samples
	}

	scanResults := map[string][]string{
		"1h": {"BTC", "ETH"},
	}

	ctx := context.Background()
	report, err := analyzer.AnalyzeBenchmarkResults(ctx, smallSample, scanResults)
	if err != nil {
		t.Fatalf("Analysis failed: %v", err)
	}

	// Should not generate recommendations with insufficient sample size
	if report.ActionableInsights != nil && report.SampleSizeValidation.RecommendationsEnabled {
		t.Error("Should not generate actionable insights when sample size < 20")
	}

	if !contains(report.SampleSizeValidation.InsufficientWindows, "1h") {
		t.Error("1h window should be marked as insufficient sample size")
	}

	// Window should have recommendation note about insufficient sample
	window1h := report.WindowAnalysis["1h"]
	if window1h.RecommendationNote == nil || !containsText(*window1h.RecommendationNote, "Sample size") {
		t.Error("Window with insufficient sample should have recommendation note")
	}
}

func TestExchangeNativeSeriesWithFallbackLabeling(t *testing.T) {
	config := diagnostics.DiagnosticsConfig{
		SeriesSource:       "exchange_native_first",
		ExchangeNativeOnly: false, // Allow fallback for testing
	}

	analyzer := diagnostics.NewDiagnosticsAnalyzer(config)

	topGainers := map[string][]diagnostics.TopGainer{
		"24h": {{Symbol: "TESTCOIN", PercentageFloat: 15.0}},
	}

	scanResults := map[string][]string{
		"24h": {"TESTCOIN"},
	}

	ctx := context.Background()
	report, err := analyzer.AnalyzeBenchmarkResults(ctx, topGainers, scanResults)
	if err != nil {
		t.Fatalf("Analysis failed: %v", err)
	}

	// Check that series source is properly labeled
	for _, hit := range report.WindowAnalysis["24h"].Hits {
		if hit.SeriesSource == "" {
			t.Error("Hit should have series source labeled")
		}

		// Should be either exchange_native or aggregator_fallback
		if hit.SeriesSource != "exchange_native" && hit.SeriesSource != "aggregator_fallback" {
			t.Errorf("Invalid series source: %s", hit.SeriesSource)
		}
	}
}

func TestRegimeAwareThresholdEnforcement(t *testing.T) {
	config := diagnostics.DiagnosticsConfig{
		RegimeAware: true,
	}

	analyzer := diagnostics.NewDiagnosticsAnalyzer(config)

	topGainers := map[string][]diagnostics.TopGainer{
		"24h": {{Symbol: "BTC", PercentageFloat: 20.0}},
	}

	scanResults := map[string][]string{
		"24h": {"BTC"},
	}

	ctx := context.Background()
	report, err := analyzer.AnalyzeBenchmarkResults(ctx, topGainers, scanResults)
	if err != nil {
		t.Fatalf("Analysis failed: %v", err)
	}

	// Check that regime context is properly set
	if report.RegimeContext.CurrentRegime == "" {
		t.Error("Regime context should have current regime set")
	}

	// Check that diagnostic results include regime information
	for _, hit := range report.WindowAnalysis["24h"].Hits {
		if hit.RegimedUsed == "" {
			t.Error("Hit should have regime used for analysis")
		}
	}
}

func TestOutputFormatWithRawVsSpecCompliantColumns(t *testing.T) {
	config := diagnostics.DiagnosticsConfig{
		MinSampleSize: 1, // Low for testing
	}

	analyzer := diagnostics.NewDiagnosticsAnalyzer(config)

	topGainers := map[string][]diagnostics.TopGainer{
		"24h": {
			{Symbol: "ETH", PercentageFloat: 42.8},
			{Symbol: "BTC", PercentageFloat: 25.0},
		},
	}

	scanResults := map[string][]string{
		"24h": {"BTC"},
	}

	ctx := context.Background()
	report, err := analyzer.AnalyzeBenchmarkResults(ctx, topGainers, scanResults)
	if err != nil {
		t.Fatalf("Analysis failed: %v", err)
	}

	// Every diagnostic result should have both raw and spec-compliant data
	allResults := append(report.WindowAnalysis["24h"].Hits, report.WindowAnalysis["24h"].Misses...)

	for _, result := range allResults {
		// Should have raw gain percentage
		if result.RawGainPercent == 0 {
			t.Errorf("Result for %s should have raw gain percentage", result.Symbol)
		}

		// Should have entry/exit details for compliant analysis
		if result.Status == "HIT" {
			if result.EntryPrice == nil || result.ExitPrice == nil {
				t.Errorf("Hit for %s should have entry/exit price details", result.Symbol)
			}

			if result.EntryReason == nil || result.ExitReason == nil {
				t.Errorf("Hit for %s should have entry/exit reason details", result.Symbol)
			}
		}
	}

	// Performance summary should clearly distinguish the two metrics
	if report.PerformanceSummary.TotalRawMarketGainsMissed <= 0 {
		t.Error("Should track raw market gains missed for comparison")
	}

	if report.PerformanceSummary.TotalSpecCompliantGainsMissed <= 0 {
		t.Error("Should track spec-compliant gains missed for decisions")
	}
}

// Helper functions
func containsRawPercentages(text string) bool {
	// Check for specific raw percentages that should not be used in decisions
	rawPercentages := []string{"42.8", "38.4", "25.0"}
	for _, percentage := range rawPercentages {
		if containsText(text, percentage) {
			return true
		}
	}
	return false
}

func containsText(text, substring string) bool {
	return len(text) >= len(substring) &&
		text != substring &&
		(text[:len(substring)] == substring ||
			text[len(text)-len(substring):] == substring ||
			indexOfSubstring(text, substring) != -1)
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

func indexOfSubstring(text, substring string) int {
	for i := 0; i <= len(text)-len(substring); i++ {
		if text[i:i+len(substring)] == substring {
			return i
		}
	}
	return -1
}
