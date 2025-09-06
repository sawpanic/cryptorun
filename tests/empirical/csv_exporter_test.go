package empirical

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"testing"
)

// CSVExporter handles exporting test results to CSV format
type CSVExporter struct {
	OutputDir string
}

func NewCSVExporter(outputDir string) *CSVExporter {
	return &CSVExporter{OutputDir: outputDir}
}

func (e *CSVExporter) ExportDecileLiftAnalysis(analysis []DecileAnalysis, filename string) error {
	filePath := filepath.Join(e.OutputDir, filename)

	file, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("failed to create CSV file: %w", err)
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// Write header
	header := []string{
		"decile",
		"count",
		"avg_composite_score",
		"avg_forward_return_4h",
		"avg_forward_return_24h",
		"min_score",
		"max_score",
		"return_4h_pct",
		"return_24h_pct",
	}

	if err := writer.Write(header); err != nil {
		return fmt.Errorf("failed to write header: %w", err)
	}

	// Sort by decile for consistent output
	sort.Slice(analysis, func(i, j int) bool {
		return analysis[i].Decile < analysis[j].Decile
	})

	// Write data rows
	for _, decile := range analysis {
		row := []string{
			strconv.Itoa(decile.Decile),
			strconv.Itoa(decile.Count),
			fmt.Sprintf("%.3f", decile.AvgCompositeScore),
			fmt.Sprintf("%.6f", decile.AvgForwardReturn4h),
			fmt.Sprintf("%.6f", decile.AvgForwardReturn24h),
			fmt.Sprintf("%.3f", decile.MinScore),
			fmt.Sprintf("%.3f", decile.MaxScore),
			fmt.Sprintf("%.3f", decile.AvgForwardReturn4h*100),
			fmt.Sprintf("%.3f", decile.AvgForwardReturn24h*100),
		}

		if err := writer.Write(row); err != nil {
			return fmt.Errorf("failed to write row: %w", err)
		}
	}

	return nil
}

func (e *CSVExporter) ExportGateWinRateAnalysis(gateAnalysis []GateWinRateEntry, filename string) error {
	filePath := filepath.Join(e.OutputDir, filename)

	file, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("failed to create CSV file: %w", err)
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// Write header
	header := []string{
		"gate_config",
		"timeframe",
		"regime",
		"pass_count",
		"fail_count",
		"pass_avg_return",
		"fail_avg_return",
		"outperformance_gap",
		"pass_hit_rate",
		"fail_hit_rate",
		"hit_rate_lift",
		"pass_avg_return_pct",
		"fail_avg_return_pct",
		"outperformance_gap_pct",
	}

	if err := writer.Write(header); err != nil {
		return fmt.Errorf("failed to write header: %w", err)
	}

	// Write data rows
	for _, entry := range gateAnalysis {
		row := []string{
			entry.GateConfig,
			entry.Timeframe,
			entry.Regime,
			strconv.Itoa(entry.PassCount),
			strconv.Itoa(entry.FailCount),
			fmt.Sprintf("%.6f", entry.PassAvgReturn),
			fmt.Sprintf("%.6f", entry.FailAvgReturn),
			fmt.Sprintf("%.6f", entry.OutperformanceGap),
			fmt.Sprintf("%.4f", entry.PassHitRate),
			fmt.Sprintf("%.4f", entry.FailHitRate),
			fmt.Sprintf("%.4f", entry.HitRateLift),
			fmt.Sprintf("%.3f", entry.PassAvgReturn*100),
			fmt.Sprintf("%.3f", entry.FailAvgReturn*100),
			fmt.Sprintf("%.3f", entry.OutperformanceGap*100),
		}

		if err := writer.Write(row); err != nil {
			return fmt.Errorf("failed to write row: %w", err)
		}
	}

	return nil
}

// GateWinRateEntry represents a single gate analysis result
type GateWinRateEntry struct {
	GateConfig        string
	Timeframe         string
	Regime            string
	PassCount         int
	FailCount         int
	PassAvgReturn     float64
	FailAvgReturn     float64
	OutperformanceGap float64
	PassHitRate       float64
	FailHitRate       float64
	HitRateLift       float64
}

func TestCSVExport_DecileLiftRegime(t *testing.T) {
	// Load synthetic panel and calculate decile statistics
	panel := loadSyntheticPanel(t)
	decileStats := calculateDecileStatistics(panel)

	// Create exporter
	outputDir := "../../../artifacts"
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		t.Fatalf("failed to create output directory: %v", err)
	}

	exporter := NewCSVExporter(outputDir)

	// Export decile lift analysis
	filename := "regime_decile_lift.csv"
	if err := exporter.ExportDecileLiftAnalysis(decileStats, filename); err != nil {
		t.Fatalf("failed to export decile lift analysis: %v", err)
	}

	// Verify file was created
	filePath := filepath.Join(outputDir, filename)
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		t.Errorf("CSV file was not created: %s", filePath)
	}

	// Read and validate CSV content
	content, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("failed to read exported CSV: %v", err)
	}

	// Basic validation
	if len(content) == 0 {
		t.Error("exported CSV is empty")
	}

	// Check for expected header
	contentStr := string(content)
	expectedHeaders := []string{"decile", "avg_composite_score", "avg_forward_return_4h"}
	for _, header := range expectedHeaders {
		if !contains(contentStr, header) {
			t.Errorf("CSV missing expected header: %s", header)
		}
	}

	t.Logf("Successfully exported decile lift analysis to %s (%d bytes)", filename, len(content))
}

func TestCSVExport_GateWinRate(t *testing.T) {
	panel := loadSyntheticPanel(t)

	// Generate gate win rate analysis for different configurations
	gateConfigs := []struct {
		name  string
		gates EntryGates
	}{
		{
			name: "standard",
			gates: EntryGates{
				MinScore:                   75.0,
				MinVADR:                    1.8,
				MinFundingDivergenceZScore: 2.0,
			},
		},
		{
			name: "lenient",
			gates: EntryGates{
				MinScore:                   70.0,
				MinVADR:                    1.5,
				MinFundingDivergenceZScore: 1.5,
			},
		},
		{
			name: "strict",
			gates: EntryGates{
				MinScore:                   80.0,
				MinVADR:                    2.2,
				MinFundingDivergenceZScore: 2.5,
			},
		},
	}

	timeframes := []string{"4h", "24h"}
	regimes := getUniqueRegimes(panel)

	var gateAnalysisResults []GateWinRateEntry

	// Generate analysis for all combinations
	for _, config := range gateConfigs {
		for _, timeframe := range timeframes {
			// Overall analysis (all regimes)
			overallAnalysis := analyzeGatePerformance(panel, config.gates, timeframe)

			entry := GateWinRateEntry{
				GateConfig:        config.name,
				Timeframe:         timeframe,
				Regime:            "all",
				PassCount:         overallAnalysis.GatePassCount,
				FailCount:         overallAnalysis.GateFailCount,
				PassAvgReturn:     overallAnalysis.GatePassAvgReturn,
				FailAvgReturn:     overallAnalysis.GateFailAvgReturn,
				OutperformanceGap: overallAnalysis.OutperformanceGap,
			}

			// Calculate hit rates
			hitThreshold := getHitThreshold(timeframe)
			entry.PassHitRate = calculateHitRate(panel, config.gates, hitThreshold, true)
			entry.FailHitRate = calculateHitRate(panel, config.gates, hitThreshold, false)
			entry.HitRateLift = entry.PassHitRate - entry.FailHitRate

			gateAnalysisResults = append(gateAnalysisResults, entry)

			// Regime-specific analysis
			for _, regime := range regimes {
				regimeEntries := filterByRegime(panel, regime)
				if len(regimeEntries) < 3 {
					continue // Skip if insufficient data
				}

				regimeAnalysis := analyzeGatePerformanceForEntries(regimeEntries, config.gates, timeframe)

				regimeEntry := GateWinRateEntry{
					GateConfig:        config.name,
					Timeframe:         timeframe,
					Regime:            regime,
					PassCount:         regimeAnalysis.GatePassCount,
					FailCount:         regimeAnalysis.GateFailCount,
					PassAvgReturn:     regimeAnalysis.GatePassAvgReturn,
					FailAvgReturn:     regimeAnalysis.GateFailAvgReturn,
					OutperformanceGap: regimeAnalysis.OutperformanceGap,
				}

				// Calculate regime-specific hit rates
				regimeEntry.PassHitRate = calculateHitRateForEntries(regimeEntries, config.gates, hitThreshold, true)
				regimeEntry.FailHitRate = calculateHitRateForEntries(regimeEntries, config.gates, hitThreshold, false)
				regimeEntry.HitRateLift = regimeEntry.PassHitRate - regimeEntry.FailHitRate

				gateAnalysisResults = append(gateAnalysisResults, regimeEntry)
			}
		}
	}

	// Export gate win rate analysis
	outputDir := "../../../artifacts"
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		t.Fatalf("failed to create output directory: %v", err)
	}

	exporter := NewCSVExporter(outputDir)
	filename := "gate_winrate.csv"

	if err := exporter.ExportGateWinRateAnalysis(gateAnalysisResults, filename); err != nil {
		t.Fatalf("failed to export gate win rate analysis: %v", err)
	}

	// Verify file was created
	filePath := filepath.Join(outputDir, filename)
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		t.Errorf("CSV file was not created: %s", filePath)
	}

	// Read and validate CSV content
	content, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("failed to read exported CSV: %v", err)
	}

	// Basic validation
	if len(content) == 0 {
		t.Error("exported CSV is empty")
	}

	contentStr := string(content)
	expectedHeaders := []string{"gate_config", "pass_count", "outperformance_gap"}
	for _, header := range expectedHeaders {
		if !contains(contentStr, header) {
			t.Errorf("CSV missing expected header: %s", header)
		}
	}

	t.Logf("Successfully exported gate win rate analysis to %s (%d bytes, %d entries)",
		filename, len(content), len(gateAnalysisResults))
}

func TestCSVExport_FilePermissions(t *testing.T) {
	outputDir := "../../../artifacts"
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		t.Fatalf("failed to create output directory: %v", err)
	}

	exporter := NewCSVExporter(outputDir)

	// Test with minimal data
	minimalData := []DecileAnalysis{
		{
			Decile:              1,
			Count:               1,
			AvgCompositeScore:   50.0,
			AvgForwardReturn4h:  0.01,
			AvgForwardReturn24h: 0.02,
			MinScore:            50.0,
			MaxScore:            50.0,
		},
	}

	filename := "test_permissions.csv"
	if err := exporter.ExportDecileLiftAnalysis(minimalData, filename); err != nil {
		t.Errorf("failed to export with minimal data: %v", err)
	}

	// Verify file exists and is readable
	filePath := filepath.Join(outputDir, filename)
	if _, err := os.Stat(filePath); err != nil {
		t.Errorf("exported file not accessible: %v", err)
	}

	// Clean up test file
	os.Remove(filePath)
}

// Helper functions

func getUniqueRegimes(panel []SyntheticPanelEntry) []string {
	regimeSet := make(map[string]bool)
	for _, entry := range panel {
		regimeSet[entry.Regime] = true
	}

	var regimes []string
	for regime := range regimeSet {
		regimes = append(regimes, regime)
	}

	sort.Strings(regimes)
	return regimes
}

func filterByRegime(panel []SyntheticPanelEntry, regime string) []SyntheticPanelEntry {
	var filtered []SyntheticPanelEntry
	for _, entry := range panel {
		if entry.Regime == regime {
			filtered = append(filtered, entry)
		}
	}
	return filtered
}

func getHitThreshold(timeframe string) float64 {
	switch timeframe {
	case "4h":
		return 0.025 // 2.5%
	case "24h":
		return 0.040 // 4.0%
	default:
		return 0.025
	}
}

func calculateHitRateForEntries(entries []SyntheticPanelEntry, gates EntryGates, threshold float64, passesGatesFlag bool) float64 {
	var relevantEntries []SyntheticPanelEntry

	for _, entry := range entries {
		entryPassesGates := passesGates(entry, gates)
		if entryPassesGates == passesGatesFlag {
			relevantEntries = append(relevantEntries, entry)
		}
	}

	if len(relevantEntries) == 0 {
		return 0
	}

	hits := 0
	for _, entry := range relevantEntries {
		if entry.ForwardReturn4h >= threshold {
			hits++
		}
	}

	return float64(hits) / float64(len(relevantEntries))
}

func contains(str, substr string) bool {
	return len(str) >= len(substr) &&
		(str[:len(substr)] == substr ||
			len(str) > len(substr) &&
				(str[len(str)-len(substr):] == substr ||
					containsMiddle(str, substr)))
}

func containsMiddle(str, substr string) bool {
	for i := 0; i <= len(str)-len(substr); i++ {
		if str[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
