package premove

import (
	"context"
	"fmt"
	"math"
	"sort"
	"time"
)

// CVDResidualAnalyzer implements robust CVD residualization with R² fallback
type CVDResidualAnalyzer struct {
	config *CVDConfig
}

// NewCVDResidualAnalyzer creates a CVD residual analyzer with robust regression
func NewCVDResidualAnalyzer(config *CVDConfig) *CVDResidualAnalyzer {
	if config == nil {
		config = DefaultCVDConfig()
	}
	return &CVDResidualAnalyzer{config: config}
}

// CVDConfig contains parameters for CVD residual analysis
type CVDConfig struct {
	// Regression parameters
	MinDataPoints     int     `yaml:"min_data_points"`     // 50 minimum data points
	WinsorizePctLower float64 `yaml:"winsorize_pct_lower"` // 5% lower winsorization
	WinsorizePctUpper float64 `yaml:"winsorize_pct_upper"` // 95% upper winsorization
	MinRSquared       float64 `yaml:"min_r_squared"`       // 0.30 minimum R² for regression
	DailyRefitEnabled bool    `yaml:"daily_refit_enabled"` // true - refit model daily

	// Fallback parameters
	FallbackMethod    string  `yaml:"fallback_method"`    // "percentile" or "zscore"
	FallbackLookback  int     `yaml:"fallback_lookback"`  // 20 periods for percentile rank
	FallbackThreshold float64 `yaml:"fallback_threshold"` // 80% percentile threshold

	// Residual analysis
	ResidualMinStdDev float64 `yaml:"residual_min_std_dev"` // 2.0 std dev significance
	ResidualMaxAge    int64   `yaml:"residual_max_age"`     // 3600 seconds (1 hour) max age

	// Performance limits
	MaxComputeTimeMs int64 `yaml:"max_compute_time_ms"` // 200ms compute timeout
}

// DefaultCVDConfig returns production CVD configuration
func DefaultCVDConfig() *CVDConfig {
	return &CVDConfig{
		// Regression parameters
		MinDataPoints:     50,
		WinsorizePctLower: 5.0,
		WinsorizePctUpper: 95.0,
		MinRSquared:       0.30,
		DailyRefitEnabled: true,

		// Fallback parameters
		FallbackMethod:    "percentile",
		FallbackLookback:  20,
		FallbackThreshold: 80.0,

		// Residual analysis
		ResidualMinStdDev: 2.0,
		ResidualMaxAge:    3600, // 1 hour

		// Performance
		MaxComputeTimeMs: 200,
	}
}

// CVDDataPoint represents a single CVD observation
type CVDDataPoint struct {
	Timestamp   time.Time `json:"timestamp"`
	Price       float64   `json:"price"`        // Price at timestamp
	CVD         float64   `json:"cvd"`          // Cumulative volume delta
	Volume      float64   `json:"volume"`       // Volume at timestamp
	PriceChange float64   `json:"price_change"` // Price change since previous
}

// CVDRegressionModel contains fitted regression coefficients and statistics
type CVDRegressionModel struct {
	Symbol           string    `json:"symbol"`
	FitTimestamp     time.Time `json:"fit_timestamp"`
	Intercept        float64   `json:"intercept"`         // β₀
	PriceCoefficient float64   `json:"price_coefficient"` // β₁ - price change coefficient
	RSquared         float64   `json:"r_squared"`         // Model R²
	StandardError    float64   `json:"standard_error"`    // Residual standard error
	DataPoints       int       `json:"data_points"`       // Number of points used in fit
	IsValid          bool      `json:"is_valid"`          // Whether model meets quality thresholds
	LastRefit        time.Time `json:"last_refit"`        // When model was last refit
}

// CVDResidualResult contains residual analysis output
type CVDResidualResult struct {
	Symbol             string              `json:"symbol"`
	Timestamp          time.Time           `json:"timestamp"`
	RawResidual        float64             `json:"raw_residual"`        // Raw CVD residual
	NormalizedResidual float64             `json:"normalized_residual"` // Z-score normalized residual
	PercentileRank     float64             `json:"percentile_rank"`     // 0-100 percentile rank
	SignificanceScore  float64             `json:"significance_score"`  // 0-1 significance (for scoring)
	Method             string              `json:"method"`              // "regression" or fallback method
	Model              *CVDRegressionModel `json:"model"`               // Regression model used (if applicable)
	FallbackReason     string              `json:"fallback_reason"`     // Why fallback was used
	ComputeTimeMs      int64               `json:"compute_time_ms"`     // Analysis compute time
	DataQuality        *CVDDataQuality     `json:"data_quality"`        // Data quality assessment
	IsSignificant      bool                `json:"is_significant"`      // Whether residual is significant
	Warnings           []string            `json:"warnings"`            // Analysis warnings
}

// CVDDataQuality contains data quality metrics
type CVDDataQuality struct {
	PointsAvailable  int     `json:"points_available"`  // Total data points available
	PointsUsed       int     `json:"points_used"`       // Points used after winsorization
	WinsorizedPoints int     `json:"winsorized_points"` // Points removed by winsorization
	DataSpanHours    float64 `json:"data_span_hours"`   // Time span of data
	MissingDataPct   float64 `json:"missing_data_pct"`  // % missing data points
	OutliersPct      float64 `json:"outliers_pct"`      // % outliers detected
}

// AnalyzeCVDResidual performs comprehensive CVD residual analysis with fallbacks
func (cvda *CVDResidualAnalyzer) AnalyzeCVDResidual(ctx context.Context, symbol string, dataPoints []*CVDDataPoint) (*CVDResidualResult, error) {
	startTime := time.Now()

	result := &CVDResidualResult{
		Symbol:    symbol,
		Timestamp: time.Now(),
		Method:    "regression", // Default to regression, may fallback
		Warnings:  []string{},
		DataQuality: &CVDDataQuality{
			PointsAvailable: len(dataPoints),
		},
	}

	// Data quality assessment
	if len(dataPoints) == 0 {
		return nil, fmt.Errorf("no CVD data points provided for %s", symbol)
	}

	// Calculate data span and quality metrics
	cvda.assessDataQuality(dataPoints, result.DataQuality)

	// Check minimum data requirements
	if len(dataPoints) < cvda.config.MinDataPoints {
		result.FallbackReason = fmt.Sprintf("Insufficient data points (%d < %d)", len(dataPoints), cvda.config.MinDataPoints)
		return cvda.performFallbackAnalysis(dataPoints, result), nil
	}

	// Winsorize data to remove extreme outliers
	winsorizedData, outlierCount := cvda.winsorizeData(dataPoints)
	result.DataQuality.PointsUsed = len(winsorizedData)
	result.DataQuality.WinsorizedPoints = outlierCount
	result.DataQuality.OutliersPct = float64(outlierCount) / float64(len(dataPoints)) * 100.0

	if len(winsorizedData) < cvda.config.MinDataPoints {
		result.FallbackReason = fmt.Sprintf("Too many outliers removed (%d < %d after winsorization)", len(winsorizedData), cvda.config.MinDataPoints)
		return cvda.performFallbackAnalysis(dataPoints, result), nil
	}

	// Fit regression model: CVD = β₀ + β₁ * PriceChange + ε
	model, err := cvda.fitRegressionModel(symbol, winsorizedData)
	if err != nil {
		result.FallbackReason = fmt.Sprintf("Regression fitting failed: %v", err)
		return cvda.performFallbackAnalysis(dataPoints, result), nil
	}

	result.Model = model

	// Check model quality (R² threshold)
	if model.RSquared < cvda.config.MinRSquared {
		result.FallbackReason = fmt.Sprintf("Low R² (%.3f < %.3f)", model.RSquared, cvda.config.MinRSquared)
		return cvda.performFallbackAnalysis(dataPoints, result), nil
	}

	// Calculate residual for most recent data point
	latestPoint := dataPoints[len(dataPoints)-1]
	predicted := model.Intercept + model.PriceCoefficient*latestPoint.PriceChange
	result.RawResidual = latestPoint.CVD - predicted

	// Normalize residual using model standard error
	if model.StandardError > 0 {
		result.NormalizedResidual = result.RawResidual / model.StandardError
	}

	// Calculate percentile rank using historical residuals
	result.PercentileRank = cvda.calculatePercentileRank(result.RawResidual, winsorizedData, model)

	// Calculate significance score (0-1) for use in Pre-Movement scoring
	result.SignificanceScore = cvda.calculateSignificanceScore(result.NormalizedResidual, result.PercentileRank)

	// Determine significance based on threshold
	result.IsSignificant = math.Abs(result.NormalizedResidual) >= cvda.config.ResidualMinStdDev

	result.ComputeTimeMs = time.Since(startTime).Milliseconds()

	// Performance warning
	if result.ComputeTimeMs > cvda.config.MaxComputeTimeMs {
		result.Warnings = append(result.Warnings,
			fmt.Sprintf("CVD analysis took %dms (>%dms threshold)", result.ComputeTimeMs, cvda.config.MaxComputeTimeMs))
	}

	return result, nil
}

// assessDataQuality evaluates input data quality
func (cvda *CVDResidualAnalyzer) assessDataQuality(dataPoints []*CVDDataPoint, quality *CVDDataQuality) {
	if len(dataPoints) < 2 {
		return
	}

	// Calculate data span
	firstTime := dataPoints[0].Timestamp
	lastTime := dataPoints[len(dataPoints)-1].Timestamp
	quality.DataSpanHours = lastTime.Sub(firstTime).Hours()

	// Detect missing data (gaps > 2x median interval)
	intervals := make([]float64, 0, len(dataPoints)-1)
	for i := 1; i < len(dataPoints); i++ {
		interval := dataPoints[i].Timestamp.Sub(dataPoints[i-1].Timestamp).Minutes()
		intervals = append(intervals, interval)
	}

	sort.Float64s(intervals)
	medianInterval := intervals[len(intervals)/2]

	missingCount := 0
	for _, interval := range intervals {
		if interval > 2*medianInterval {
			missingCount++
		}
	}
	quality.MissingDataPct = float64(missingCount) / float64(len(intervals)) * 100.0
}

// winsorizeData removes extreme outliers using percentile-based winsorization
func (cvda *CVDResidualAnalyzer) winsorizeData(dataPoints []*CVDDataPoint) ([]*CVDDataPoint, int) {
	if len(dataPoints) <= 10 {
		return dataPoints, 0 // Don't winsorize small datasets
	}

	// Extract CVD and price change values for percentile calculation
	cvdValues := make([]float64, len(dataPoints))
	priceChanges := make([]float64, len(dataPoints))

	for i, dp := range dataPoints {
		cvdValues[i] = dp.CVD
		priceChanges[i] = dp.PriceChange
	}

	// Calculate winsorization bounds
	cvdLower, cvdUpper := calculatePercentileBounds(cvdValues, cvda.config.WinsorizePctLower, cvda.config.WinsorizePctUpper)
	priceLower, priceUpper := calculatePercentileBounds(priceChanges, cvda.config.WinsorizePctLower, cvda.config.WinsorizePctUpper)

	// Filter out extreme outliers
	filtered := make([]*CVDDataPoint, 0, len(dataPoints))
	outlierCount := 0

	for _, dp := range dataPoints {
		if dp.CVD >= cvdLower && dp.CVD <= cvdUpper &&
			dp.PriceChange >= priceLower && dp.PriceChange <= priceUpper {
			filtered = append(filtered, dp)
		} else {
			outlierCount++
		}
	}

	return filtered, outlierCount
}

// fitRegressionModel fits CVD = β₀ + β₁ * PriceChange + ε using least squares
func (cvda *CVDResidualAnalyzer) fitRegressionModel(symbol string, dataPoints []*CVDDataPoint) (*CVDRegressionModel, error) {
	n := len(dataPoints)
	if n < 3 {
		return nil, fmt.Errorf("insufficient data for regression (need ≥3, got %d)", n)
	}

	// Extract X (price changes) and Y (CVD) vectors
	var sumX, sumY, sumXY, sumXX, sumYY float64

	for _, dp := range dataPoints {
		x := dp.PriceChange
		y := dp.CVD

		sumX += x
		sumY += y
		sumXY += x * y
		sumXX += x * x
		sumYY += y * y
	}

	// Calculate means
	meanX := sumX / float64(n)
	meanY := sumY / float64(n)

	// Calculate regression coefficients
	// β₁ = (n∑XY - ∑X∑Y) / (n∑X² - (∑X)²)
	// β₀ = ȳ - β₁x̄

	numerator := float64(n)*sumXY - sumX*sumY
	denominator := float64(n)*sumXX - sumX*sumX

	if math.Abs(denominator) < 1e-10 {
		return nil, fmt.Errorf("singular matrix - no price variation")
	}

	beta1 := numerator / denominator
	beta0 := meanY - beta1*meanX

	// Calculate R² and standard error
	var ssRes, ssTot float64
	for _, dp := range dataPoints {
		predicted := beta0 + beta1*dp.PriceChange
		residual := dp.CVD - predicted

		ssRes += residual * residual
		ssTot += (dp.CVD - meanY) * (dp.CVD - meanY)
	}

	var rSquared float64
	if ssTot > 0 {
		rSquared = 1.0 - ssRes/ssTot
	}

	standardError := math.Sqrt(ssRes / float64(n-2))

	model := &CVDRegressionModel{
		Symbol:           symbol,
		FitTimestamp:     time.Now(),
		Intercept:        beta0,
		PriceCoefficient: beta1,
		RSquared:         rSquared,
		StandardError:    standardError,
		DataPoints:       n,
		IsValid:          rSquared >= cvda.config.MinRSquared,
		LastRefit:        time.Now(),
	}

	return model, nil
}

// performFallbackAnalysis uses percentile ranking when regression fails
func (cvda *CVDResidualAnalyzer) performFallbackAnalysis(dataPoints []*CVDDataPoint, result *CVDResidualResult) *CVDResidualResult {
	result.Method = cvda.config.FallbackMethod

	if len(dataPoints) == 0 {
		result.Warnings = append(result.Warnings, "No data available for fallback analysis")
		return result
	}

	latestPoint := dataPoints[len(dataPoints)-1]
	result.RawResidual = latestPoint.CVD // Use raw CVD as "residual"

	// Percentile-based fallback
	if cvda.config.FallbackMethod == "percentile" {
		lookback := cvda.config.FallbackLookback
		if lookback > len(dataPoints) {
			lookback = len(dataPoints)
		}

		// Get recent CVD values for percentile ranking
		recentValues := make([]float64, 0, lookback)
		startIdx := len(dataPoints) - lookback

		for i := startIdx; i < len(dataPoints); i++ {
			recentValues = append(recentValues, dataPoints[i].CVD)
		}

		result.PercentileRank = calculatePercentile(latestPoint.CVD, recentValues)
		result.SignificanceScore = result.PercentileRank / 100.0

		// Significance based on percentile threshold
		result.IsSignificant = result.PercentileRank >= cvda.config.FallbackThreshold

	} else if cvda.config.FallbackMethod == "zscore" {
		// Z-score based fallback
		lookback := cvda.config.FallbackLookback
		if lookback > len(dataPoints) {
			lookback = len(dataPoints)
		}

		// Calculate mean and std dev of recent CVD values
		var sum, sumSq float64
		startIdx := len(dataPoints) - lookback

		for i := startIdx; i < len(dataPoints); i++ {
			val := dataPoints[i].CVD
			sum += val
			sumSq += val * val
		}

		mean := sum / float64(lookback)
		variance := sumSq/float64(lookback) - mean*mean
		stddev := math.Sqrt(variance)

		if stddev > 0 {
			result.NormalizedResidual = (latestPoint.CVD - mean) / stddev
		}

		result.SignificanceScore = math.Min(1.0, math.Abs(result.NormalizedResidual)/cvda.config.ResidualMinStdDev)
		result.IsSignificant = math.Abs(result.NormalizedResidual) >= cvda.config.ResidualMinStdDev
	}

	return result
}

// calculatePercentileRank computes percentile rank of residual using regression model
func (cvda *CVDResidualAnalyzer) calculatePercentileRank(rawResidual float64, dataPoints []*CVDDataPoint, model *CVDRegressionModel) float64 {
	if len(dataPoints) < 2 {
		return 50.0 // Default to median
	}

	// Calculate residuals for all data points
	residuals := make([]float64, 0, len(dataPoints))
	for _, dp := range dataPoints {
		predicted := model.Intercept + model.PriceCoefficient*dp.PriceChange
		residual := dp.CVD - predicted
		residuals = append(residuals, residual)
	}

	return calculatePercentile(rawResidual, residuals)
}

// calculateSignificanceScore converts residual statistics to 0-1 score for Pre-Movement
func (cvda *CVDResidualAnalyzer) calculateSignificanceScore(normalizedResidual, percentileRank float64) float64 {
	// Combine z-score and percentile rank into unified significance score

	// Z-score component (0-1, capped at 3σ)
	zScore := math.Min(1.0, math.Abs(normalizedResidual)/3.0)

	// Percentile component (0-1, distance from median)
	percentileScore := math.Abs(percentileRank-50.0) / 50.0

	// Weighted average (favor z-score for significance)
	significance := 0.7*zScore + 0.3*percentileScore

	return math.Min(1.0, significance)
}

// Helper function to calculate percentile bounds for winsorization
func calculatePercentileBounds(values []float64, lowerPct, upperPct float64) (float64, float64) {
	if len(values) == 0 {
		return 0, 0
	}

	sorted := make([]float64, len(values))
	copy(sorted, values)
	sort.Float64s(sorted)

	lowerIdx := int(float64(len(sorted)) * lowerPct / 100.0)
	upperIdx := int(float64(len(sorted)) * upperPct / 100.0)

	if lowerIdx >= len(sorted) {
		lowerIdx = len(sorted) - 1
	}
	if upperIdx >= len(sorted) {
		upperIdx = len(sorted) - 1
	}

	return sorted[lowerIdx], sorted[upperIdx]
}

// Helper function to calculate percentile rank of value in dataset
func calculatePercentile(value float64, dataset []float64) float64 {
	if len(dataset) == 0 {
		return 50.0
	}

	count := 0
	for _, v := range dataset {
		if v <= value {
			count++
		}
	}

	return float64(count) / float64(len(dataset)) * 100.0
}

// GetResidualSummary returns a concise summary of CVD residual analysis
func (cvdr *CVDResidualResult) GetResidualSummary() string {
	method := cvdr.Method
	if cvdr.FallbackReason != "" {
		method = fmt.Sprintf("%s (fallback)", method)
	}

	significance := "📊 NORMAL"
	if cvdr.IsSignificant {
		significance = "⚠️  SIGNIFICANT"
	}

	return fmt.Sprintf("%s — %s CVD residual: %.3f (%.1f%%, %s, %dms)",
		significance, cvdr.Symbol, cvdr.RawResidual, cvdr.PercentileRank, method, cvdr.ComputeTimeMs)
}

// GetDetailedAnalysis returns comprehensive CVD residual analysis
func (cvdr *CVDResidualResult) GetDetailedAnalysis() string {
	report := fmt.Sprintf("CVD Residual Analysis: %s\n", cvdr.Symbol)
	report += fmt.Sprintf("Method: %s | Significant: %t | Time: %dms\n\n", cvdr.Method, cvdr.IsSignificant, cvdr.ComputeTimeMs)

	// Residual metrics
	report += fmt.Sprintf("Raw Residual: %.3f\n", cvdr.RawResidual)
	if cvdr.NormalizedResidual != 0 {
		report += fmt.Sprintf("Z-Score: %.2f\n", cvdr.NormalizedResidual)
	}
	report += fmt.Sprintf("Percentile Rank: %.1f%%\n", cvdr.PercentileRank)
	report += fmt.Sprintf("Significance Score: %.3f\n\n", cvdr.SignificanceScore)

	// Model information
	if cvdr.Model != nil {
		report += fmt.Sprintf("Regression Model:\n")
		report += fmt.Sprintf("  R²: %.3f | Std Error: %.3f | Data Points: %d\n",
			cvdr.Model.RSquared, cvdr.Model.StandardError, cvdr.Model.DataPoints)
		report += fmt.Sprintf("  Equation: CVD = %.3f + %.3f × PriceChange\n\n",
			cvdr.Model.Intercept, cvdr.Model.PriceCoefficient)
	}

	// Fallback information
	if cvdr.FallbackReason != "" {
		report += fmt.Sprintf("Fallback Reason: %s\n\n", cvdr.FallbackReason)
	}

	// Data quality
	if cvdr.DataQuality != nil {
		report += fmt.Sprintf("Data Quality:\n")
		report += fmt.Sprintf("  Points: %d available, %d used\n",
			cvdr.DataQuality.PointsAvailable, cvdr.DataQuality.PointsUsed)
		if cvdr.DataQuality.WinsorizedPoints > 0 {
			report += fmt.Sprintf("  Winsorized: %d outliers (%.1f%%)\n",
				cvdr.DataQuality.WinsorizedPoints, cvdr.DataQuality.OutliersPct)
		}
		report += fmt.Sprintf("  Data Span: %.1f hours\n", cvdr.DataQuality.DataSpanHours)
	}

	// Warnings
	if len(cvdr.Warnings) > 0 {
		report += fmt.Sprintf("\nWarnings:\n")
		for i, warning := range cvdr.Warnings {
			report += fmt.Sprintf("  %d. %s\n", i+1, warning)
		}
	}

	return report
}
