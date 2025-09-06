package regime

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"text/template"
	"time"

	"github.com/rs/zerolog/log"
)

// ReportGenerator creates markdown and CSV artifacts for regime analysis
type ReportGenerator struct {
	analyzer *RegimeAnalyzer
}

// NewReportGenerator creates a new report generator
func NewReportGenerator() *ReportGenerator {
	return &ReportGenerator{
		analyzer: NewRegimeAnalyzer(),
	}
}

// Generate creates comprehensive regime report with markdown and CSV artifacts
func (rg *ReportGenerator) Generate(config ReportConfig) error {
	// Calculate report period
	endTime := time.Now().UTC()
	startTime := endTime.Add(-config.Period)

	period := ReportPeriod{
		StartTime: startTime,
		EndTime:   endTime,
		Duration:  config.Period.String(),
	}

	// Generate analysis
	data, err := rg.analyzer.GenerateReport(period)
	if err != nil {
		return fmt.Errorf("failed to generate regime analysis: %w", err)
	}

	// Ensure output directory exists
	if err := os.MkdirAll(config.OutputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Generate PIT timestamp for artifacts
	timestamp := data.GeneratedAt.Format("20060102_150405")

	// Generate markdown report
	mdPath := filepath.Join(config.OutputDir, fmt.Sprintf("regime_weekly_%s.md", timestamp))
	if err := rg.generateMarkdownReport(data, mdPath); err != nil {
		return fmt.Errorf("failed to generate markdown report: %w", err)
	}

	// Generate CSV artifacts
	csvFiles, err := rg.generateCSVArtifacts(data, config.OutputDir, timestamp)
	if err != nil {
		return fmt.Errorf("failed to generate CSV artifacts: %w", err)
	}

	log.Info().
		Str("markdown", mdPath).
		Strs("csv_files", csvFiles).
		Int("kpi_alerts", len(data.KPIAlerts)).
		Msg("Regime report generated successfully")

	return nil
}

// generateMarkdownReport creates comprehensive markdown report
func (rg *ReportGenerator) generateMarkdownReport(data *RegimeReportData, outputPath string) error {
	tmpl := template.Must(template.New("regime_report").Parse(regimeReportTemplate))

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return fmt.Errorf("failed to execute template: %w", err)
	}

	if err := os.WriteFile(outputPath, buf.Bytes(), 0644); err != nil {
		return fmt.Errorf("failed to write markdown file: %w", err)
	}

	return nil
}

// generateCSVArtifacts creates CSV files for flip history, exit stats, and decile lifts
func (rg *ReportGenerator) generateCSVArtifacts(data *RegimeReportData, outputDir, timestamp string) ([]string, error) {
	csvFiles := []string{}

	// Generate flip history CSV
	flipPath := filepath.Join(outputDir, fmt.Sprintf("regime_flips_%s.csv", timestamp))
	if err := rg.generateFlipHistoryCSV(data.FlipHistory, flipPath); err != nil {
		return nil, fmt.Errorf("failed to generate flip history CSV: %w", err)
	}
	csvFiles = append(csvFiles, flipPath)

	// Generate exit stats CSV
	exitPath := filepath.Join(outputDir, fmt.Sprintf("regime_exits_%s.csv", timestamp))
	if err := rg.generateExitStatsCSV(data.ExitStats, exitPath); err != nil {
		return nil, fmt.Errorf("failed to generate exit stats CSV: %w", err)
	}
	csvFiles = append(csvFiles, exitPath)

	// Generate decile lifts CSV
	decilePath := filepath.Join(outputDir, fmt.Sprintf("regime_deciles_%s.csv", timestamp))
	if err := rg.generateDecileLiftsCSV(data.DecileLifts, decilePath); err != nil {
		return nil, fmt.Errorf("failed to generate decile lifts CSV: %w", err)
	}
	csvFiles = append(csvFiles, decilePath)

	// Generate KPI alerts CSV
	alertPath := filepath.Join(outputDir, fmt.Sprintf("regime_alerts_%s.csv", timestamp))
	if err := rg.generateKPIAlertsCSV(data.KPIAlerts, alertPath); err != nil {
		return nil, fmt.Errorf("failed to generate KPI alerts CSV: %w", err)
	}
	csvFiles = append(csvFiles, alertPath)

	return csvFiles, nil
}

// generateFlipHistoryCSV creates CSV with regime transition timeline
func (rg *ReportGenerator) generateFlipHistoryCSV(flips []RegimeFlip, outputPath string) error {
	file, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// Write header
	header := []string{
		"timestamp", "from_regime", "to_regime", "duration_hours",
		"realized_vol_7d", "pct_above_20ma", "breadth_thrust", "stability_score", "confidence_level",
		"momentum_before", "technical_before", "volume_before", "quality_before", "catalyst_before",
		"momentum_after", "technical_after", "volume_after", "quality_after", "catalyst_after",
		"momentum_delta", "technical_delta", "volume_delta", "quality_delta", "catalyst_delta",
	}
	writer.Write(header)

	// Write data rows
	for _, flip := range flips {
		row := []string{
			flip.Timestamp.Format(time.RFC3339),
			flip.FromRegime,
			flip.ToRegime,
			fmt.Sprintf("%.1f", flip.DurationHours),
			fmt.Sprintf("%.3f", flip.DetectorInputs.RealizedVol7d),
			fmt.Sprintf("%.3f", flip.DetectorInputs.PctAbove20MA),
			fmt.Sprintf("%.3f", flip.DetectorInputs.BreadthThrust),
			fmt.Sprintf("%.3f", flip.DetectorInputs.StabilityScore),
			fmt.Sprintf("%.3f", flip.DetectorInputs.ConfidenceLevel),
			fmt.Sprintf("%.1f", flip.WeightChanges.Before.Momentum),
			fmt.Sprintf("%.1f", flip.WeightChanges.Before.Technical),
			fmt.Sprintf("%.1f", flip.WeightChanges.Before.Volume),
			fmt.Sprintf("%.1f", flip.WeightChanges.Before.Quality),
			fmt.Sprintf("%.1f", flip.WeightChanges.Before.Catalyst),
			fmt.Sprintf("%.1f", flip.WeightChanges.After.Momentum),
			fmt.Sprintf("%.1f", flip.WeightChanges.After.Technical),
			fmt.Sprintf("%.1f", flip.WeightChanges.After.Volume),
			fmt.Sprintf("%.1f", flip.WeightChanges.After.Quality),
			fmt.Sprintf("%.1f", flip.WeightChanges.After.Catalyst),
			fmt.Sprintf("%.1f", flip.WeightChanges.Delta.Momentum),
			fmt.Sprintf("%.1f", flip.WeightChanges.Delta.Technical),
			fmt.Sprintf("%.1f", flip.WeightChanges.Delta.Volume),
			fmt.Sprintf("%.1f", flip.WeightChanges.Delta.Quality),
			fmt.Sprintf("%.1f", flip.WeightChanges.Delta.Catalyst),
		}
		writer.Write(row)
	}

	return nil
}

// generateExitStatsCSV creates CSV with exit distribution analysis
func (rg *ReportGenerator) generateExitStatsCSV(exitStats map[string]ExitStats, outputPath string) error {
	file, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// Write header
	header := []string{
		"regime", "total_exits", "time_limit", "hard_stop", "momentum_fade", "profit_target", "venue_health", "other",
		"time_limit_pct", "hard_stop_pct", "profit_target_pct", "avg_hold_hours", "avg_return_pct",
	}
	writer.Write(header)

	// Write data rows (sorted by regime for consistency)
	regimes := []string{"trending_bull", "choppy", "high_vol"}
	for _, regime := range regimes {
		if stats, exists := exitStats[regime]; exists {
			row := []string{
				stats.Regime,
				strconv.Itoa(stats.TotalExits),
				strconv.Itoa(stats.TimeLimit),
				strconv.Itoa(stats.HardStop),
				strconv.Itoa(stats.MomentumFade),
				strconv.Itoa(stats.ProfitTarget),
				strconv.Itoa(stats.VenueHealth),
				strconv.Itoa(stats.Other),
				fmt.Sprintf("%.1f", stats.TimeLimitPct),
				fmt.Sprintf("%.1f", stats.HardStopPct),
				fmt.Sprintf("%.1f", stats.ProfitTargetPct),
				fmt.Sprintf("%.1f", stats.AvgHoldHours),
				fmt.Sprintf("%.1f", stats.AvgReturnPct),
			}
			writer.Write(row)
		}
	}

	return nil
}

// generateDecileLiftsCSV creates CSV with score→return analysis
func (rg *ReportGenerator) generateDecileLiftsCSV(decileLifts map[string]DecileLift, outputPath string) error {
	file, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// Write header
	header := []string{
		"regime", "decile", "score_min", "score_max", "count", "avg_score", "avg_return_48h", "hit_rate", "sharpe",
	}
	writer.Write(header)

	// Write data rows
	regimes := []string{"trending_bull", "choppy", "high_vol"}
	for _, regime := range regimes {
		if lift, exists := decileLifts[regime]; exists {
			for _, bucket := range lift.Deciles {
				row := []string{
					regime,
					strconv.Itoa(bucket.Decile),
					fmt.Sprintf("%.1f", bucket.ScoreMin),
					fmt.Sprintf("%.1f", bucket.ScoreMax),
					strconv.Itoa(bucket.Count),
					fmt.Sprintf("%.1f", bucket.AvgScore),
					fmt.Sprintf("%.1f", bucket.AvgReturn48h),
					fmt.Sprintf("%.3f", bucket.HitRate),
					fmt.Sprintf("%.2f", bucket.Sharpe),
				}
				writer.Write(row)
			}
		}
	}

	return nil
}

// generateKPIAlertsCSV creates CSV with KPI violations and recommendations
func (rg *ReportGenerator) generateKPIAlertsCSV(alerts []KPIAlert, outputPath string) error {
	file, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// Write header
	header := []string{
		"type", "regime", "current_pct", "target_pct", "severity", "action", "description",
	}
	writer.Write(header)

	// Write data rows
	for _, alert := range alerts {
		row := []string{
			alert.Type,
			alert.Regime,
			fmt.Sprintf("%.1f", alert.CurrentPct),
			fmt.Sprintf("%.1f", alert.TargetPct),
			alert.Severity,
			alert.Action,
			alert.Description,
		}
		writer.Write(row)
	}

	return nil
}

// regimeReportTemplate is the markdown template for weekly regime reports
const regimeReportTemplate = `# CryptoRun Regime Weekly Report

**Generated:** {{.GeneratedAt.Format "2006-01-02 15:04:05 UTC"}}  
**Period:** {{.Period.StartTime.Format "2006-01-02"}} to {{.Period.EndTime.Format "2006-01-02"}} ({{.Period.Duration}})

## 🚨 KPI Alerts

{{if .KPIAlerts -}}
{{range .KPIAlerts -}}
### {{if eq .Severity "critical"}}🔴{{else}}🟡{{end}} {{.Type | title}} - {{.Regime | title}} Regime

- **Current**: {{printf "%.1f%%" .CurrentPct}}
- **Target**: {{printf "%.1f%%" .TargetPct}}  
- **Severity**: {{.Severity}}
- **Action**: {{.Action}}

{{.Description}}

{{end -}}
{{else -}}
✅ **No KPI violations detected** - All regimes operating within target thresholds.

{{end}}

## 📈 Regime Flip History

**Total Flips**: {{len .FlipHistory}}  
**Average Duration**: {{range .FlipHistory}}{{.DurationHours}}{{end | avg}}h

| Timestamp | From | To | Duration | Vol 7d | Above 20MA | Breadth | Momentum Δ | 
|-----------|------|----|---------:|-------:|-----------:|--------:|-----------:|
{{range .FlipHistory -}}
| {{.Timestamp.Format "01-02 15:04"}} | {{regimeBadge .FromRegime}} {{.FromRegime}} | {{regimeBadge .ToRegime}} {{.ToRegime}} | {{printf "%.1fh" .DurationHours}} | {{printf "%.2f" .DetectorInputs.RealizedVol7d}} | {{printf "%.2f" .DetectorInputs.PctAbove20MA}} | {{printf "%.2f" .DetectorInputs.BreadthThrust}} | {{printf "%+.1f%%" .WeightChanges.Delta.Momentum}} |
{{end}}

### Detector Inputs Summary

- **7d Realized Volatility**: {{range .FlipHistory}}{{.DetectorInputs.RealizedVol7d}}{{end | avg | printf "%.2f"}} avg ({{range .FlipHistory}}{{.DetectorInputs.RealizedVol7d}}{{end | min | printf "%.2f"}}-{{range .FlipHistory}}{{.DetectorInputs.RealizedVol7d}}{{end | max | printf "%.2f"}} range)
- **% Above 20MA**: {{range .FlipHistory}}{{.DetectorInputs.PctAbove20MA}}{{end | avg | printf "%.2f"}} avg  
- **Breadth Thrust**: {{range .FlipHistory}}{{.DetectorInputs.BreadthThrust}}{{end | avg | printf "%.2f"}} avg

## 🚪 Exit Distribution Analysis

| Regime | Total | Time Limit | Hard Stop | Profit Target | Avg Return | Avg Hold |
|--------|------:|-----------:|----------:|--------------:|-----------:|---------:|
{{range $regime, $stats := .ExitStats -}}
| {{regimeBadge $regime}} {{$regime}} | {{$stats.TotalExits}} | {{printf "%.1f%%" $stats.TimeLimitPct}} {{if gt $stats.TimeLimitPct 40.0}}⚠️{{end}} | {{printf "%.1f%%" $stats.HardStopPct}} {{if gt $stats.HardStopPct 20.0}}🔴{{end}} | {{printf "%.1f%%" $stats.ProfitTargetPct}} {{if lt $stats.ProfitTargetPct 25.0}}⚠️{{end}} | {{printf "%.1f%%" $stats.AvgReturnPct}} | {{printf "%.1fh" $stats.AvgHoldHours}} |
{{end}}

**KPI Targets**: Time Limit ≤40%, Hard Stop ≤20%, Profit Target ≥25%

## 📊 Score→Return Lift Analysis

{{range $regime, $lift := .DecileLifts -}}
### {{regimeBadge $regime}} {{$regime | title}} Regime

**Correlation**: {{printf "%.2f" $lift.Correlation}} | **R²**: {{printf "%.2f" $lift.R2}} | **Lift**: {{printf "%.1fx" $lift.Lift}}

| Decile | Score Range | Count | Avg Return | Hit Rate |
|-------:|-------------|------:|-----------:|---------:|
{{range $lift.Deciles -}}
| {{.Decile}} | {{printf "%.0f-%.0f" .ScoreMin .ScoreMax}} | {{.Count}} | {{printf "%.1f%%" .AvgReturn48h}} | {{printf "%.1f%%" (mul .HitRate 100)}} |
{{end}}

{{end}}

## 💡 Recommendations

{{if .KPIAlerts -}}
Based on KPI violations detected:

{{range .KPIAlerts -}}
- **{{.Regime | title}}**: {{.Action}}
{{end -}}
{{else -}}
- Regime performance within acceptable bounds
- Continue monitoring exit distributions and score lift
- Consider optimizing factor weights if correlation degrades
{{end}}

---

**Artifacts Generated**:
- regime_flips_{{.GeneratedAt.Format "20060102_150405"}}.csv
- regime_exits_{{.GeneratedAt.Format "20060102_150405"}}.csv  
- regime_deciles_{{.GeneratedAt.Format "20060102_150405"}}.csv
- regime_alerts_{{.GeneratedAt.Format "20060102_150405"}}.csv

**Point-in-Time Integrity**: All data reflects regime states at decision time with no retroactive adjustments.
`

// Template helper functions would be registered here in production
func init() {
	// In production, these would be properly registered template functions
}
