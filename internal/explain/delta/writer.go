package delta

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
)

// Writer handles artifact generation for delta results
type Writer struct {
	outputDir string
}

// NewWriter creates a new delta results writer
func NewWriter(outputDir string) *Writer {
	return &Writer{
		outputDir: outputDir,
	}
}

// WriteJSONL writes detailed results in JSONL format
func (w *Writer) WriteJSONL(results *Results) error {
	resultsPath := filepath.Join(w.outputDir, "results.jsonl")

	file, err := os.Create(resultsPath)
	if err != nil {
		return fmt.Errorf("failed to create results JSONL: %w", err)
	}
	defer file.Close()

	// Write header with metadata
	header := map[string]interface{}{
		"type":               "explain_delta_header",
		"timestamp":          time.Now().Format(time.RFC3339),
		"universe":           results.Universe,
		"regime":             results.Regime,
		"baseline_timestamp": results.BaselineTimestamp.Format(time.RFC3339),
		"current_timestamp":  results.CurrentTimestamp.Format(time.RFC3339),
		"total_assets":       results.TotalAssets,
		"fail_count":         results.FailCount,
		"warn_count":         results.WarnCount,
		"ok_count":           results.OKCount,
	}

	headerBytes, _ := json.Marshal(header)
	if _, err := file.WriteString(string(headerBytes) + "\n"); err != nil {
		return fmt.Errorf("failed to write JSONL header: %w", err)
	}

	// Write each asset delta
	for _, asset := range results.Assets {
		assetLine := map[string]interface{}{
			"type":             "asset_delta",
			"symbol":           asset.Symbol,
			"regime":           asset.Regime,
			"status":           asset.Status,
			"baseline_factors": asset.BaselineFactors,
			"current_factors":  asset.CurrentFactors,
			"deltas":           asset.Deltas,
			"tolerance_check":  asset.ToleranceCheck,
		}

		if asset.WorstViolation != nil {
			assetLine["worst_violation"] = asset.WorstViolation
		}

		assetBytes, _ := json.Marshal(assetLine)
		if _, err := file.WriteString(string(assetBytes) + "\n"); err != nil {
			return fmt.Errorf("failed to write asset delta: %w", err)
		}
	}

	// Write worst offenders summary
	if len(results.WorstOffenders) > 0 {
		offendersLine := map[string]interface{}{
			"type":            "worst_offenders",
			"worst_offenders": results.WorstOffenders,
		}

		offendersBytes, _ := json.Marshal(offendersLine)
		if _, err := file.WriteString(string(offendersBytes) + "\n"); err != nil {
			return fmt.Errorf("failed to write worst offenders: %w", err)
		}
	}

	log.Info().Str("path", resultsPath).Msg("Delta results JSONL written")
	return nil
}

// WriteMarkdown writes comprehensive markdown summary
func (w *Writer) WriteMarkdown(results *Results) error {
	summaryPath := filepath.Join(w.outputDir, "summary.md")

	file, err := os.Create(summaryPath)
	if err != nil {
		return fmt.Errorf("failed to create markdown summary: %w", err)
	}
	defer file.Close()

	md := w.generateMarkdown(results)

	if _, err := file.WriteString(md); err != nil {
		return fmt.Errorf("failed to write markdown summary: %w", err)
	}

	log.Info().Str("path", summaryPath).Msg("Delta summary markdown written")
	return nil
}

// generateMarkdown creates comprehensive markdown report
func (w *Writer) generateMarkdown(results *Results) string {
	var md strings.Builder

	// Header
	md.WriteString("# Explain Delta Analysis Report\n\n")
	md.WriteString("## UX MUST — Live Progress & Explainability\n\n")
	md.WriteString("Real-time forensic analysis of factor contribution shifts with tolerance-based validation and regime-aware thresholds.\n\n")

	// Executive Summary
	md.WriteString("## Executive Summary\n\n")
	md.WriteString(fmt.Sprintf("- **Universe**: %s\n", results.Universe))
	md.WriteString(fmt.Sprintf("- **Regime**: %s\n", results.Regime))
	md.WriteString(fmt.Sprintf("- **Baseline**: %s\n", results.BaselineTimestamp.Format("2006-01-02 15:04 UTC")))
	md.WriteString(fmt.Sprintf("- **Current**: %s\n", results.CurrentTimestamp.Format("2006-01-02 15:04 UTC")))
	md.WriteString(fmt.Sprintf("- **Total Assets**: %d\n", results.TotalAssets))
	md.WriteString(fmt.Sprintf("- **Status**: FAIL(%d) WARN(%d) OK(%d)\n\n",
		results.FailCount, results.WarnCount, results.OKCount))

	passRate := float64(results.OKCount) / float64(results.TotalAssets) * 100
	md.WriteString(fmt.Sprintf("**Pass Rate**: %.1f%% (%d/%d assets within tolerance)\n\n",
		passRate, results.OKCount, results.TotalAssets))

	// Status indicator
	if results.FailCount > 0 {
		md.WriteString("🔴 **CRITICAL**: Factor shifts detected beyond failure thresholds\n\n")
	} else if results.WarnCount > 0 {
		md.WriteString("🟡 **WARNING**: Factor shifts detected beyond warning thresholds\n\n")
	} else {
		md.WriteString("✅ **HEALTHY**: All factor contributions within acceptable tolerance\n\n")
	}

	// Worst Offenders
	if len(results.WorstOffenders) > 0 {
		md.WriteString("## Worst Violations\n\n")
		md.WriteString("| Rank | Symbol | Factor | Delta | Tolerance | Severity | Hint |\n")
		md.WriteString("|------|--------|--------|-------|-----------|----------|------|\n")

		for i, offender := range results.WorstOffenders {
			sign := "+"
			if offender.Delta < 0 {
				sign = ""
			}

			md.WriteString(fmt.Sprintf("| %d | %s | %s | %s%.1f | ±%.1f | %s | %s |\n",
				i+1,
				offender.Symbol,
				offender.Factor,
				sign,
				offender.Delta,
				offender.Tolerance,
				offender.Severity,
				offender.Hint))
		}
		md.WriteString("\n")
	}

	// Factor Distribution Analysis
	md.WriteString("## Factor Distribution Analysis\n\n")

	factorStats := w.calculateFactorStats(results)

	md.WriteString("### Violations by Factor\n\n")
	md.WriteString("| Factor | Fail | Warn | OK | Fail Rate |\n")
	md.WriteString("|--------|------|------|----|-----------|\n")

	for factor, stats := range factorStats {
		failRate := float64(stats.Fails) / float64(stats.Total) * 100
		md.WriteString(fmt.Sprintf("| %s | %d | %d | %d | %.1f%% |\n",
			factor, stats.Fails, stats.Warns, stats.OKs, failRate))
	}
	md.WriteString("\n")

	// Regime Tolerance Settings
	md.WriteString("## Regime Tolerance Configuration\n\n")
	if results.ToleranceConfig != nil {
		if regimeTol, exists := results.ToleranceConfig.Regimes[results.Regime]; exists {
			md.WriteString(fmt.Sprintf("**Regime**: %s\n\n", regimeTol.Name))
			md.WriteString("| Factor | Warn Threshold | Fail Threshold | Direction |\n")
			md.WriteString("|--------|----------------|----------------|----------|\n")

			for _, factorTol := range regimeTol.FactorTolerances {
				md.WriteString(fmt.Sprintf("| %s | ±%.1f | ±%.1f | %s |\n",
					factorTol.Factor,
					factorTol.WarnAt,
					factorTol.FailAt,
					factorTol.Direction))
			}
			md.WriteString("\n")
		}
	}

	// Detailed Asset Analysis
	md.WriteString("## Detailed Asset Analysis\n\n")

	// Group assets by status
	failAssets := make([]*AssetDelta, 0)
	warnAssets := make([]*AssetDelta, 0)
	okAssets := make([]*AssetDelta, 0)

	for _, asset := range results.Assets {
		switch asset.Status {
		case "FAIL":
			failAssets = append(failAssets, asset)
		case "WARN":
			warnAssets = append(warnAssets, asset)
		case "OK":
			okAssets = append(okAssets, asset)
		}
	}

	// Failed assets (detailed breakdown)
	if len(failAssets) > 0 {
		md.WriteString("### Failed Assets (Detailed)\n\n")
		for _, asset := range failAssets {
			w.writeAssetDetails(&md, asset, true)
		}
	}

	// Warning assets (summary)
	if len(warnAssets) > 0 {
		md.WriteString("### Warning Assets (Summary)\n\n")
		for _, asset := range warnAssets {
			w.writeAssetDetails(&md, asset, false)
		}
	}

	// Methodology
	md.WriteString("## Methodology\n\n")
	md.WriteString("### Delta Calculation\n")
	md.WriteString("- **Delta**: `current_factor - baseline_factor`\n")
	md.WriteString("- **Tolerance Check**: `|delta| >= threshold`\n")
	md.WriteString("- **Direction Filter**: Applied per factor configuration\n\n")

	md.WriteString("### Severity Levels\n")
	md.WriteString("- **OK**: All factors within acceptable tolerance\n")
	md.WriteString("- **WARN**: One or more factors exceed warning threshold\n")
	md.WriteString("- **FAIL**: One or more factors exceed failure threshold\n\n")

	md.WriteString("### Regime Adaptation\n")
	md.WriteString("- Tolerance thresholds adapt to market regime\n")
	md.WriteString("- Bull markets use tighter tolerances\n")
	md.WriteString("- Volatile markets use relaxed tolerances\n\n")

	// Artifacts
	md.WriteString("## Generated Artifacts\n\n")
	md.WriteString("- **results.jsonl**: Complete delta analysis in JSONL format\n")
	md.WriteString("- **summary.md**: This comprehensive markdown report\n\n")

	// Footer
	md.WriteString("---\n")
	md.WriteString(fmt.Sprintf("*Generated on %s by CryptoRun explain delta*\n",
		time.Now().Format("2006-01-02 15:04:05 UTC")))

	return md.String()
}

// writeAssetDetails writes detailed analysis for a single asset
func (w *Writer) writeAssetDetails(md *strings.Builder, asset *AssetDelta, detailed bool) {
	md.WriteString(fmt.Sprintf("#### %s (%s)\n\n", asset.Symbol, asset.Status))

	if detailed && asset.WorstViolation != nil {
		md.WriteString(fmt.Sprintf("**Worst Violation**: %s (δ=%.1f, threshold=±%.1f) - %s\n\n",
			asset.WorstViolation.Factor,
			asset.WorstViolation.Delta,
			asset.WorstViolation.Tolerance,
			asset.WorstViolation.Hint))
	}

	// Factor table
	md.WriteString("| Factor | Baseline | Current | Delta | Status |\n")
	md.WriteString("|--------|----------|---------|-------|--------|\n")

	for factor, baseline := range asset.BaselineFactors {
		current := asset.CurrentFactors[factor]
		delta := asset.Deltas[factor]

		var status string
		if check, exists := asset.ToleranceCheck[factor]; exists {
			status = check.Severity
		} else {
			status = "OK"
		}

		sign := "+"
		if delta < 0 {
			sign = ""
		}

		md.WriteString(fmt.Sprintf("| %s | %.1f | %.1f | %s%.1f | %s |\n",
			factor, baseline, current, sign, delta, status))
	}

	md.WriteString("\n")
}

// calculateFactorStats computes violation statistics by factor
func (w *Writer) calculateFactorStats(results *Results) map[string]struct{ Fails, Warns, OKs, Total int } {
	stats := make(map[string]struct{ Fails, Warns, OKs, Total int })

	for _, asset := range results.Assets {
		for factor, check := range asset.ToleranceCheck {
			s := stats[factor]
			s.Total++

			switch check.Severity {
			case "FAIL":
				s.Fails++
			case "WARN":
				s.Warns++
			case "OK":
				s.OKs++
			}

			stats[factor] = s
		}
	}

	return stats
}

// GetArtifactPaths returns the paths to generated artifacts
func (w *Writer) GetArtifactPaths() *ArtifactPaths {
	return &ArtifactPaths{
		ResultsJSONL: filepath.Join(w.outputDir, "results.jsonl"),
		SummaryMD:    filepath.Join(w.outputDir, "summary.md"),
		OutputDir:    w.outputDir,
	}
}
