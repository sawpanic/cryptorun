package optimization

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	
	"github.com/rs/zerolog/log"
)

// ReportGenerator creates optimization reports and outputs
type ReportGenerator struct {
	outputDir string
}

// NewReportGenerator creates a new report generator
func NewReportGenerator(outputDir string) *ReportGenerator {
	return &ReportGenerator{
		outputDir: outputDir,
	}
}

// GenerateOptimizationReport generates a complete optimization report
func (rg *ReportGenerator) GenerateOptimizationReport(result *OptimizationResult) error {
	timestamp := result.StartTime.Format("20060102_150405")
	targetDir := filepath.Join(rg.outputDir, strings.ToLower(string(result.Target)), timestamp)
	
	// Create output directory
	err := os.MkdirAll(targetDir, 0755)
	if err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}
	
	log.Info().Str("dir", targetDir).Msg("Generating optimization report")
	
	// Generate params.json
	err = rg.generateParamsJSON(result, filepath.Join(targetDir, "params.json"))
	if err != nil {
		return fmt.Errorf("failed to generate params.json: %w", err)
	}
	
	// Generate report.md
	err = rg.generateReportMarkdown(result, filepath.Join(targetDir, "report.md"))
	if err != nil {
		return fmt.Errorf("failed to generate report.md: %w", err)
	}
	
	// Generate cv_curves.json
	err = rg.generateCVCurvesJSON(result, filepath.Join(targetDir, "cv_curves.json"))
	if err != nil {
		return fmt.Errorf("failed to generate cv_curves.json: %w", err)
	}
	
	log.Info().Str("dir", targetDir).Msg("Optimization report generated successfully")
	return nil
}

// generateParamsJSON generates the parameters JSON file
func (rg *ReportGenerator) generateParamsJSON(result *OptimizationResult, filePath string) error {
	// Create a clean parameter export
	paramExport := struct {
		ID         string                 `json:"id"`
		Target     string                 `json:"target"`
		Timestamp  time.Time              `json:"timestamp"`
		Parameters map[string]interface{} `json:"parameters"`
		Objective  float64                `json:"objective_score"`
		Metrics    EvaluationMetrics      `json:"performance_metrics"`
	}{
		ID:         result.ID,
		Target:     string(result.Target),
		Timestamp:  result.Parameters.Timestamp,
		Parameters: make(map[string]interface{}),
		Objective:  result.AggregateMetrics.ObjectiveScore,
		Metrics:    result.AggregateMetrics,
	}
	
	// Extract parameter values
	for name, param := range result.Parameters.Parameters {
		paramExport.Parameters[name] = param.Value
	}
	
	data, err := json.MarshalIndent(paramExport, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal parameters: %w", err)
	}
	
	return os.WriteFile(filePath, data, 0644)
}

// generateReportMarkdown generates the main report markdown file
func (rg *ReportGenerator) generateReportMarkdown(result *OptimizationResult, filePath string) error {
	var report strings.Builder
	
	// Header
	report.WriteString(fmt.Sprintf("# %s Optimization Report\n\n", strings.Title(string(result.Target))))
	report.WriteString(fmt.Sprintf("**Generated:** %s  \n", result.EndTime.Format("2006-01-02 15:04:05 UTC")))
	report.WriteString(fmt.Sprintf("**Duration:** %s  \n", result.Duration.String()))
	report.WriteString(fmt.Sprintf("**Optimization ID:** `%s`  \n\n", result.ID))
	
	// Executive Summary
	report.WriteString("## Executive Summary\n\n")
	report.WriteString(fmt.Sprintf("**Objective Score:** %.4f  \n", result.AggregateMetrics.ObjectiveScore))
	
	if result.Target == TargetMomentum {
		report.WriteString(fmt.Sprintf("**Precision@20 (24h):** %.2f%%  \n", result.AggregateMetrics.Precision20_24h*100))
		report.WriteString(fmt.Sprintf("**Precision@20 (48h):** %.2f%%  \n", result.AggregateMetrics.Precision20_48h*100))
	} else if result.Target == TargetDip {
		report.WriteString(fmt.Sprintf("**Precision@20 (12h):** %.2f%%  \n", result.AggregateMetrics.Precision20_24h*100))
		report.WriteString(fmt.Sprintf("**Precision@20 (24h):** %.2f%%  \n", result.AggregateMetrics.Precision20_48h*100))
	}
	
	report.WriteString(fmt.Sprintf("**False Positive Rate:** %.2f%%  \n", result.AggregateMetrics.FalsePositiveRate*100))
	report.WriteString(fmt.Sprintf("**Max Drawdown Penalty:** %.4f  \n", result.AggregateMetrics.MaxDrawdownPenalty))
	report.WriteString(fmt.Sprintf("**Total Predictions:** %d  \n", result.AggregateMetrics.TotalPredictions))
	report.WriteString(fmt.Sprintf("**Valid Predictions:** %d (%.1f%%)  \n\n", 
		result.AggregateMetrics.ValidPredictions,
		float64(result.AggregateMetrics.ValidPredictions)/float64(result.AggregateMetrics.TotalPredictions)*100))
	
	// Performance Metrics
	report.WriteString("## Performance Metrics\n\n")
	report.WriteString("| Metric | 24h | 48h |\n")
	report.WriteString("|--------|-----|-----|\n")
	report.WriteString(fmt.Sprintf("| Precision@10 | %.2f%% | %.2f%% |\n", 
		result.AggregateMetrics.Precision10_24h*100, result.AggregateMetrics.Precision10_48h*100))
	report.WriteString(fmt.Sprintf("| Precision@20 | %.2f%% | %.2f%% |\n", 
		result.AggregateMetrics.Precision20_24h*100, result.AggregateMetrics.Precision20_48h*100))
	report.WriteString(fmt.Sprintf("| Precision@50 | %.2f%% | %.2f%% |\n", 
		result.AggregateMetrics.Precision50_24h*100, result.AggregateMetrics.Precision50_48h*100))
	report.WriteString(fmt.Sprintf("| Win Rate | %.2f%% | %.2f%% |\n\n", 
		result.AggregateMetrics.WinRate24h*100, result.AggregateMetrics.WinRate48h*100))
	
	// Regime Analysis
	if len(result.RegimeMetrics) > 0 {
		report.WriteString("## Regime Analysis\n\n")
		
		for regime, metrics := range result.RegimeMetrics {
			report.WriteString(fmt.Sprintf("### %s Regime\n\n", strings.Title(regime)))
			report.WriteString(fmt.Sprintf("- **Precision@20 (24h):** %.2f%%\n", metrics.Precision20_24h*100))
			report.WriteString(fmt.Sprintf("- **Precision@20 (48h):** %.2f%%\n", metrics.Precision20_48h*100))
			report.WriteString(fmt.Sprintf("- **Win Rate (24h):** %.2f%%\n", metrics.WinRate24h*100))
			report.WriteString(fmt.Sprintf("- **False Positive Rate:** %.2f%%\n", metrics.FalsePositiveRate*100))
			report.WriteString(fmt.Sprintf("- **Total Predictions:** %d\n\n", metrics.TotalPredictions))
		}
	}
	
	// Stability Analysis
	report.WriteString("## Stability Analysis\n\n")
	report.WriteString(fmt.Sprintf("**Precision Std Dev:** %.4f  \n", result.Stability.PrecisionStdDev))
	report.WriteString(fmt.Sprintf("**Objective Std Dev:** %.4f  \n", result.Stability.ObjectiveStdDev))
	report.WriteString(fmt.Sprintf("**Fold Consistency:** %.2f%% (higher is better)  \n", result.Stability.FoldConsistency*100))
	report.WriteString(fmt.Sprintf("**Regime Consistency:** %.2f%%  \n\n", result.Stability.RegimeConsistency*100))
	
	// Parameter Configuration
	report.WriteString("## Optimized Parameters\n\n")
	
	if result.Target == TargetMomentum {
		rg.writeMomentumParameters(&report, result)
	} else if result.Target == TargetDip {
		rg.writeDipParameters(&report, result)
	}
	
	// Cross-Validation Results
	report.WriteString("## Cross-Validation Results\n\n")
	report.WriteString(fmt.Sprintf("**Total Folds:** %d  \n", len(result.CVResults)))
	
	validFolds := 0
	for _, fold := range result.CVResults {
		if fold.Error == "" {
			validFolds++
		}
	}
	report.WriteString(fmt.Sprintf("**Valid Folds:** %d  \n", validFolds))
	
	report.WriteString("\n### Fold Performance\n\n")
	report.WriteString("| Fold | Period | Precision@20 (24h) | Precision@20 (48h) | Objective |\n")
	report.WriteString("|------|--------|-------------------|-------------------|----------|\n")
	
	for _, fold := range result.CVResults {
		if fold.Error == "" {
			objective := CalculateObjective(fold.Metrics)
			report.WriteString(fmt.Sprintf("| %d | %s to %s | %.2f%% | %.2f%% | %.4f |\n",
				fold.Fold,
				fold.TestPeriod.Start.Format("01-02"),
				fold.TestPeriod.End.Format("01-02"),
				fold.Metrics.Precision20_24h*100,
				fold.Metrics.Precision20_48h*100,
				objective))
		}
	}
	report.WriteString("\n")
	
	// Add true positive examples for dip optimization
	if result.Target == TargetDip {
		report.WriteString("## True Positive Examples\n\n")
		rg.writeTruePositiveExamples(&report, result)
	}
	
	// Footer
	report.WriteString("---\n")
	report.WriteString(fmt.Sprintf("*Report generated by CryptoRun Optimization Engine at %s*\n", 
		time.Now().Format("2006-01-02 15:04:05 UTC")))
	
	return os.WriteFile(filePath, []byte(report.String()), 0644)
}

// writeMomentumParameters writes momentum-specific parameters
func (rg *ReportGenerator) writeMomentumParameters(report *strings.Builder, result *OptimizationResult) {
	regimes := []string{"bull", "choppy", "high_vol"}
	timeframes := []string{"1h", "4h", "12h", "24h", "7d"}
	
	report.WriteString("### Regime Weights\n\n")
	
	for _, regime := range regimes {
		report.WriteString(fmt.Sprintf("**%s Regime:**\n", strings.Title(regime)))
		
		for _, tf := range timeframes {
			paramName := fmt.Sprintf("%s_weight_%s", regime, tf)
			if param, exists := result.Parameters.Parameters[paramName]; exists {
				if weight, ok := param.Value.(float64); ok {
					report.WriteString(fmt.Sprintf("- %s: %.1f%%\n", strings.ToUpper(tf), weight*100))
				}
			}
		}
		report.WriteString("\n")
	}
	
	// Other momentum parameters
	report.WriteString("### Technical Parameters\n\n")
	
	if param, exists := result.Parameters.Parameters["accel_ema_span"]; exists {
		report.WriteString(fmt.Sprintf("**Acceleration EMA Span:** %v  \n", param.Value))
	}
	
	if param, exists := result.Parameters.Parameters["robust_smoothing"]; exists {
		report.WriteString(fmt.Sprintf("**Robust Smoothing:** %v  \n", param.Value))
	}
	
	if param, exists := result.Parameters.Parameters["atr_lookback"]; exists {
		report.WriteString(fmt.Sprintf("**ATR Lookback:** %v periods  \n", param.Value))
	}
	
	if param, exists := result.Parameters.Parameters["volume_confirm"]; exists {
		report.WriteString(fmt.Sprintf("**Volume Confirmation:** %v  \n", param.Value))
	}
	
	report.WriteString("\n### Movement Thresholds\n\n")
	
	if param, exists := result.Parameters.Parameters["bull_threshold"]; exists {
		report.WriteString(fmt.Sprintf("**Bull Market:** %.2f%%  \n", param.Value))
	}
	
	if param, exists := result.Parameters.Parameters["choppy_threshold"]; exists {
		report.WriteString(fmt.Sprintf("**Choppy Market:** %.2f%%  \n", param.Value))
	}
	
	if param, exists := result.Parameters.Parameters["bear_threshold"]; exists {
		report.WriteString(fmt.Sprintf("**Bear Market:** %.2f%%  \n", param.Value))
	}
	
	report.WriteString("\n")
}

// writeDipParameters writes dip-specific parameters  
func (rg *ReportGenerator) writeDipParameters(report *strings.Builder, result *OptimizationResult) {
	report.WriteString("### Signal Parameters\n\n")
	
	if param, exists := result.Parameters.Parameters["rsi_trigger_1h"]; exists {
		report.WriteString(fmt.Sprintf("**RSI(1h) Trigger:** %.1f  \n", param.Value))
	}
	
	if param, exists := result.Parameters.Parameters["dip_depth_min"]; exists {
		report.WriteString(fmt.Sprintf("**Dip Depth Range:** %.1f%% to -6.0%%  \n", param.Value))
	}
	
	if param, exists := result.Parameters.Parameters["volume_flush_min"]; exists {
		report.WriteString(fmt.Sprintf("**Volume Flush:** %.2fx minimum  \n", param.Value))
	}
	
	report.WriteString("\n### Confirmation Methods\n\n")
	
	if param, exists := result.Parameters.Parameters["confirm_rsi_4h_rising"]; exists {
		report.WriteString(fmt.Sprintf("**RSI 4h Rising:** %v  \n", param.Value))
	}
	
	if param, exists := result.Parameters.Parameters["confirm_momentum_1h_cross"]; exists {
		report.WriteString(fmt.Sprintf("**Momentum 1h Cross:** %v  \n", param.Value))
	}
	
	if param, exists := result.Parameters.Parameters["enable_divergence"]; exists {
		report.WriteString(fmt.Sprintf("**Divergence Detection:** %v  \n", param.Value))
	}
	
	report.WriteString("\n### Quality Filters\n\n")
	
	if param, exists := result.Parameters.Parameters["ma20_proximity_max"]; exists {
		report.WriteString(fmt.Sprintf("**20MA Proximity:** %.1fx ATR maximum  \n", param.Value))
	}
	
	// Fixed constraints
	report.WriteString("\n### Fixed Constraints\n\n")
	report.WriteString("- **Minimum ADX:** 25.0\n")
	report.WriteString("- **Minimum Hurst:** 0.55\n")
	report.WriteString("- **Minimum VADR:** 1.75x\n")
	report.WriteString("- **Maximum Spread:** 50 bps\n")
	report.WriteString("- **Minimum Depth:** $100k @ ±2%\n")
	report.WriteString("- **Maximum Freshness:** 2 bars, 1.2x ATR\n")
	report.WriteString("- **Maximum Late Fill:** 30 seconds\n\n")
}

// writeTruePositiveExamples writes example true positive cases for dip optimization
func (rg *ReportGenerator) writeTruePositiveExamples(report *strings.Builder, result *OptimizationResult) {
	// Find the best performing fold
	bestFold := -1
	bestObjective := -1.0
	
	for i, fold := range result.CVResults {
		if fold.Error == "" {
			objective := CalculateObjective(fold.Metrics)
			if objective > bestObjective {
				bestObjective = objective
				bestFold = i
			}
		}
	}
	
	if bestFold == -1 {
		report.WriteString("*No valid folds available for examples*\n\n")
		return
	}
	
	// Get successful predictions from best fold
	fold := result.CVResults[bestFold]
	successfulPredictions := []Prediction{}
	
	for _, pred := range fold.Predictions {
		if pred.Success24h && pred.Gates.AllPass {
			successfulPredictions = append(successfulPredictions, pred)
		}
	}
	
	// Sort by performance and take top 5
	sort.Slice(successfulPredictions, func(i, j int) bool {
		return successfulPredictions[i].Actual24h > successfulPredictions[j].Actual24h
	})
	
	examples := successfulPredictions
	if len(examples) > 5 {
		examples = examples[:5]
	}
	
	report.WriteString(fmt.Sprintf("*Top %d true positive examples from best performing fold (%d):*\n\n", len(examples), bestFold))
	
	for i, example := range examples {
		report.WriteString(fmt.Sprintf("**Example %d:**\n", i+1))
		report.WriteString(fmt.Sprintf("- Symbol: %s\n", example.Symbol))
		report.WriteString(fmt.Sprintf("- Timestamp: %s\n", example.Timestamp.Format("2006-01-02 15:04")))
		report.WriteString(fmt.Sprintf("- Composite Score: %.1f\n", example.CompositeScore))
		report.WriteString(fmt.Sprintf("- 12h Return: %.2f%%\n", example.Actual24h))
		report.WriteString(fmt.Sprintf("- 24h Return: %.2f%%\n", example.Actual48h))
		report.WriteString("\n")
	}
}

// generateCVCurvesJSON generates cross-validation curves data
func (rg *ReportGenerator) generateCVCurvesJSON(result *OptimizationResult, filePath string) error {
	// Create CV curves data structure
	cvCurves := struct {
		Target    string        `json:"target"`
		Timestamp time.Time     `json:"timestamp"`
		Folds     []FoldCurve   `json:"folds"`
		Summary   CurvesSummary `json:"summary"`
	}{
		Target:    string(result.Target),
		Timestamp: result.StartTime,
		Folds:     make([]FoldCurve, 0),
		Summary:   CurvesSummary{},
	}
	
	// Process each fold
	objectives := []float64{}
	precision24h := []float64{}
	precision48h := []float64{}
	
	for _, fold := range result.CVResults {
		if fold.Error != "" {
			continue
		}
		
		objective := CalculateObjective(fold.Metrics)
		objectives = append(objectives, objective)
		precision24h = append(precision24h, fold.Metrics.Precision20_24h)
		precision48h = append(precision48h, fold.Metrics.Precision20_48h)
		
		foldCurve := FoldCurve{
			Fold:          fold.Fold,
			TrainStart:    fold.TrainPeriod.Start,
			TrainEnd:      fold.TrainPeriod.End,
			TestStart:     fold.TestPeriod.Start,
			TestEnd:       fold.TestPeriod.End,
			Objective:     objective,
			Precision24h:  fold.Metrics.Precision20_24h,
			Precision48h:  fold.Metrics.Precision20_48h,
			FPRate:        fold.Metrics.FalsePositiveRate,
			Predictions:   len(fold.Predictions),
		}
		
		cvCurves.Folds = append(cvCurves.Folds, foldCurve)
	}
	
	// Calculate summary statistics
	if len(objectives) > 0 {
		cvCurves.Summary = CurvesSummary{
			ValidFolds:       len(objectives),
			MeanObjective:    mean(objectives),
			StdObjective:     stdDev(objectives),
			MinObjective:     minFloat64(objectives),
			MaxObjective:     maxFloat64(objectives),
			MeanPrecision24h: mean(precision24h),
			StdPrecision24h:  stdDev(precision24h),
			MeanPrecision48h: mean(precision48h),
			StdPrecision48h:  stdDev(precision48h),
		}
	}
	
	data, err := json.MarshalIndent(cvCurves, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal CV curves: %w", err)
	}
	
	return os.WriteFile(filePath, data, 0644)
}

// FoldCurve represents performance data for a single CV fold
type FoldCurve struct {
	Fold         int       `json:"fold"`
	TrainStart   time.Time `json:"train_start"`
	TrainEnd     time.Time `json:"train_end"`
	TestStart    time.Time `json:"test_start"`
	TestEnd      time.Time `json:"test_end"`
	Objective    float64   `json:"objective"`
	Precision24h float64   `json:"precision_24h"`
	Precision48h float64   `json:"precision_48h"`
	FPRate       float64   `json:"false_positive_rate"`
	Predictions  int       `json:"predictions"`
}

// CurvesSummary provides summary statistics for CV curves
type CurvesSummary struct {
	ValidFolds       int     `json:"valid_folds"`
	MeanObjective    float64 `json:"mean_objective"`
	StdObjective     float64 `json:"std_objective"`
	MinObjective     float64 `json:"min_objective"`
	MaxObjective     float64 `json:"max_objective"`
	MeanPrecision24h float64 `json:"mean_precision_24h"`
	StdPrecision24h  float64 `json:"std_precision_24h"`
	MeanPrecision48h float64 `json:"mean_precision_48h"`
	StdPrecision48h  float64 `json:"std_precision_48h"`
}

// Statistical helper functions
func mean(values []float64) float64 {
	if len(values) == 0 {
		return 0.0
	}
	sum := 0.0
	for _, v := range values {
		sum += v
	}
	return sum / float64(len(values))
}

func stdDev(values []float64) float64 {
	if len(values) < 2 {
		return 0.0
	}
	m := mean(values)
	variance := 0.0
	for _, v := range values {
		diff := v - m
		variance += diff * diff
	}
	variance /= float64(len(values) - 1)
	return variance
}

func minFloat64(values []float64) float64 {
	if len(values) == 0 {
		return 0.0
	}
	minVal := values[0]
	for _, v := range values[1:] {
		if v < minVal {
			minVal = v
		}
	}
	return minVal
}

func maxFloat64(values []float64) float64 {
	if len(values) == 0 {
		return 0.0
	}
	maxVal := values[0]
	for _, v := range values[1:] {
		if v > maxVal {
			maxVal = v
		}
	}
	return maxVal
}