package premove

import (
	"context"
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCVDResidualAnalyzer_AnalyzeCVDResidual_RegressionSuccess(t *testing.T) {
	analyzer := NewCVDResidualAnalyzer(nil) // Use default config

	// Generate synthetic data with clear price-CVD relationship
	dataPoints := generateSyntheticCVDData(100, 0.8) // 100 points, 80% R²

	result, err := analyzer.AnalyzeCVDResidual(context.Background(), "BTC-USD", dataPoints)
	require.NoError(t, err)
	assert.NotNil(t, result)

	// Should use regression method successfully
	assert.Equal(t, "regression", result.Method)
	assert.Empty(t, result.FallbackReason, "Should not fallback with good data")
	assert.NotNil(t, result.Model)

	// Model should meet quality thresholds
	assert.GreaterOrEqual(t, result.Model.RSquared, 0.30, "R² should meet minimum threshold")
	assert.True(t, result.Model.IsValid, "Model should be valid")
	assert.Equal(t, 100, result.Model.DataPoints)

	// Should calculate residual metrics
	assert.NotEqual(t, 0.0, result.RawResidual, "Should have non-zero residual")
	assert.GreaterOrEqual(t, result.PercentileRank, 0.0)
	assert.LessOrEqual(t, result.PercentileRank, 100.0)
	assert.GreaterOrEqual(t, result.SignificanceScore, 0.0)
	assert.LessOrEqual(t, result.SignificanceScore, 1.0)

	// Should assess data quality
	assert.NotNil(t, result.DataQuality)
	assert.Equal(t, 100, result.DataQuality.PointsAvailable)
	assert.GreaterOrEqual(t, result.DataQuality.PointsUsed, 90) // Should retain most points
}

func TestCVDResidualAnalyzer_AnalyzeCVDResidual_LowRSquaredFallback(t *testing.T) {
	analyzer := NewCVDResidualAnalyzer(nil)

	// Generate noisy data with low correlation (should trigger fallback)
	dataPoints := generateNoisyCVDData(60) // Random relationship

	result, err := analyzer.AnalyzeCVDResidual(context.Background(), "ETH-USD", dataPoints)
	require.NoError(t, err)

	// Should fallback due to low R²
	assert.Equal(t, "percentile", result.Method)
	assert.Contains(t, result.FallbackReason, "Low R²")
	assert.Nil(t, result.Model) // No valid regression model

	// Should still calculate percentile-based significance
	assert.GreaterOrEqual(t, result.PercentileRank, 0.0)
	assert.LessOrEqual(t, result.PercentileRank, 100.0)
	assert.GreaterOrEqual(t, result.SignificanceScore, 0.0)
	assert.LessOrEqual(t, result.SignificanceScore, 1.0)
}

func TestCVDResidualAnalyzer_AnalyzeCVDResidual_InsufficientData(t *testing.T) {
	analyzer := NewCVDResidualAnalyzer(nil)

	// Generate insufficient data points (below minimum threshold)
	dataPoints := generateSyntheticCVDData(20, 0.7) // Only 20 points

	result, err := analyzer.AnalyzeCVDResidual(context.Background(), "SOL-USD", dataPoints)
	require.NoError(t, err)

	// Should fallback due to insufficient data
	assert.NotEqual(t, "regression", result.Method)
	assert.Contains(t, result.FallbackReason, "Insufficient data points")

	// Should still provide fallback analysis
	assert.GreaterOrEqual(t, result.SignificanceScore, 0.0)
	assert.LessOrEqual(t, result.SignificanceScore, 1.0)
}

func TestCVDResidualAnalyzer_WinsorizeData_OutlierRemoval(t *testing.T) {
	analyzer := NewCVDResidualAnalyzer(nil)

	// Generate data with extreme outliers
	dataPoints := make([]*CVDDataPoint, 0, 100)
	baseTime := time.Now()

	// Normal data points
	for i := 0; i < 90; i++ {
		dp := &CVDDataPoint{
			Timestamp:   baseTime.Add(time.Duration(i) * time.Minute),
			Price:       100.0 + float64(i)*0.1,
			CVD:         1000.0 + float64(i)*10.0,
			PriceChange: 0.1,
		}
		dataPoints = append(dataPoints, dp)
	}

	// Add extreme outliers
	for i := 90; i < 100; i++ {
		dp := &CVDDataPoint{
			Timestamp:   baseTime.Add(time.Duration(i) * time.Minute),
			Price:       1000.0,   // Extreme price
			CVD:         100000.0, // Extreme CVD
			PriceChange: 50.0,     // Extreme change
		}
		dataPoints = append(dataPoints, dp)
	}

	// Test winsorization
	filtered, outlierCount := analyzer.winsorizeData(dataPoints)

	assert.Greater(t, outlierCount, 0, "Should remove some outliers")
	assert.Less(t, len(filtered), len(dataPoints), "Should filter out outliers")
	assert.GreaterOrEqual(t, len(filtered), 80, "Should retain most normal data")

	// Test with small dataset (should not winsorize)
	smallData := dataPoints[:5]
	filteredSmall, outlierCountSmall := analyzer.winsorizeData(smallData)
	assert.Equal(t, 0, outlierCountSmall, "Should not winsorize small datasets")
	assert.Equal(t, len(smallData), len(filteredSmall), "Should retain all small dataset points")
}

func TestCVDResidualAnalyzer_FitRegressionModel(t *testing.T) {
	analyzer := NewCVDResidualAnalyzer(nil)

	// Generate perfect linear relationship: CVD = 1000 + 100 * PriceChange
	dataPoints := make([]*CVDDataPoint, 0, 50)
	baseTime := time.Now()

	for i := 0; i < 50; i++ {
		priceChange := float64(i-25) * 0.1 // Price changes from -2.5 to +2.4
		cvd := 1000.0 + 100.0*priceChange  // Perfect linear relationship

		dp := &CVDDataPoint{
			Timestamp:   baseTime.Add(time.Duration(i) * time.Minute),
			PriceChange: priceChange,
			CVD:         cvd,
		}
		dataPoints = append(dataPoints, dp)
	}

	model, err := analyzer.fitRegressionModel("TEST-USD", dataPoints)
	require.NoError(t, err)
	assert.NotNil(t, model)

	// Should recover correct coefficients (approximately)
	assert.InDelta(t, 1000.0, model.Intercept, 1.0, "Should recover correct intercept")
	assert.InDelta(t, 100.0, model.PriceCoefficient, 1.0, "Should recover correct slope")
	assert.Greater(t, model.RSquared, 0.99, "Perfect linear relationship should have R² > 0.99")
	assert.True(t, model.IsValid)
}

func TestCVDResidualAnalyzer_PerformFallbackAnalysis_Percentile(t *testing.T) {
	config := DefaultCVDConfig()
	config.FallbackMethod = "percentile"
	config.FallbackLookback = 10
	config.FallbackThreshold = 80.0

	analyzer := NewCVDResidualAnalyzer(config)

	// Generate data where latest point is extreme
	dataPoints := make([]*CVDDataPoint, 0, 15)
	baseTime := time.Now()

	// Normal CVD values
	for i := 0; i < 14; i++ {
		dp := &CVDDataPoint{
			Timestamp: baseTime.Add(time.Duration(i) * time.Minute),
			CVD:       1000.0 + float64(i)*10.0, // Values 1000-1130
		}
		dataPoints = append(dataPoints, dp)
	}

	// Extreme latest value
	extremeDP := &CVDDataPoint{
		Timestamp: baseTime.Add(15 * time.Minute),
		CVD:       2000.0, // Much higher than others
	}
	dataPoints = append(dataPoints, extremeDP)

	result := &CVDResidualResult{
		Symbol:    "TEST-USD",
		Timestamp: time.Now(),
	}

	finalResult := analyzer.performFallbackAnalysis(dataPoints, result)

	assert.Equal(t, "percentile", finalResult.Method)
	assert.Equal(t, 2000.0, finalResult.RawResidual) // Should use raw CVD
	assert.GreaterOrEqual(t, finalResult.PercentileRank, 90.0, "Extreme value should have high percentile rank")
	assert.True(t, finalResult.IsSignificant, "Should be significant above threshold")
}

func TestCVDResidualAnalyzer_PerformFallbackAnalysis_ZScore(t *testing.T) {
	config := DefaultCVDConfig()
	config.FallbackMethod = "zscore"
	config.FallbackLookback = 10
	config.ResidualMinStdDev = 2.0

	analyzer := NewCVDResidualAnalyzer(config)

	// Generate data with known mean and std dev
	dataPoints := make([]*CVDDataPoint, 0, 15)
	baseTime := time.Now()

	// CVD values with mean=1000, std dev ≈ 10
	cvdValues := []float64{980, 985, 990, 995, 1000, 1005, 1010, 1015, 1020}
	for i, cvd := range cvdValues {
		dp := &CVDDataPoint{
			Timestamp: baseTime.Add(time.Duration(i) * time.Minute),
			CVD:       cvd,
		}
		dataPoints = append(dataPoints, dp)
	}

	// Add extreme latest value (3 std devs from mean)
	extremeDP := &CVDDataPoint{
		Timestamp: baseTime.Add(10 * time.Minute),
		CVD:       1030.0, // ~3 std devs above mean
	}
	dataPoints = append(dataPoints, extremeDP)

	result := &CVDResidualResult{
		Symbol:    "TEST-USD",
		Timestamp: time.Now(),
	}

	finalResult := analyzer.performFallbackAnalysis(dataPoints, result)

	assert.Equal(t, "zscore", finalResult.Method)
	assert.Greater(t, math.Abs(finalResult.NormalizedResidual), 2.0, "Should have >2σ z-score")
	assert.True(t, finalResult.IsSignificant, "Should be significant above threshold")
}

func TestCVDResidualAnalyzer_CalculateSignificanceScore(t *testing.T) {
	analyzer := NewCVDResidualAnalyzer(nil)

	// Test extreme significance (high z-score and percentile)
	score1 := analyzer.calculateSignificanceScore(3.0, 95.0) // 3σ, 95th percentile
	assert.GreaterOrEqual(t, score1, 0.8, "Extreme values should have high significance")

	// Test moderate significance
	score2 := analyzer.calculateSignificanceScore(1.5, 70.0) // 1.5σ, 70th percentile
	assert.GreaterOrEqual(t, score2, 0.3)
	assert.LessOrEqual(t, score2, 0.7)

	// Test low significance
	score3 := analyzer.calculateSignificanceScore(0.5, 55.0) // 0.5σ, near median
	assert.LessOrEqual(t, score3, 0.4, "Low values should have low significance")

	// All scores should be in valid range
	scores := []float64{score1, score2, score3}
	for _, score := range scores {
		assert.GreaterOrEqual(t, score, 0.0, "Significance score should be ≥ 0")
		assert.LessOrEqual(t, score, 1.0, "Significance score should be ≤ 1")
	}
}

func TestCVDResidualAnalyzer_PerformanceRequirements(t *testing.T) {
	config := DefaultCVDConfig()
	config.MaxComputeTimeMs = 100 // 100ms limit for testing

	analyzer := NewCVDResidualAnalyzer(config)
	dataPoints := generateSyntheticCVDData(200, 0.7) // Larger dataset

	start := time.Now()
	result, err := analyzer.AnalyzeCVDResidual(context.Background(), "PERF-TEST", dataPoints)
	duration := time.Since(start)

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Less(t, duration.Milliseconds(), int64(500), "Analysis should complete in <500ms")
	assert.Greater(t, result.ComputeTimeMs, int64(0), "Should report compute time")
}

func TestCVDResidualAnalyzer_GetResidualSummary(t *testing.T) {
	result := &CVDResidualResult{
		Symbol:         "BTC-USD",
		RawResidual:    1234.5,
		PercentileRank: 87.3,
		Method:         "regression",
		ComputeTimeMs:  45,
		IsSignificant:  true,
	}

	summary := result.GetResidualSummary()
	assert.Contains(t, summary, "⚠️  SIGNIFICANT")
	assert.Contains(t, summary, "BTC-USD")
	assert.Contains(t, summary, "1234.5")
	assert.Contains(t, summary, "87.3%")
	assert.Contains(t, summary, "regression")
	assert.Contains(t, summary, "45ms")
}

func TestCVDResidualAnalyzer_GetDetailedAnalysis(t *testing.T) {
	result := &CVDResidualResult{
		Symbol:             "ETH-USD",
		Method:             "regression",
		IsSignificant:      true,
		ComputeTimeMs:      67,
		RawResidual:        -567.8,
		NormalizedResidual: -2.3,
		PercentileRank:     15.2,
		SignificanceScore:  0.82,
		Model: &CVDRegressionModel{
			RSquared:         0.78,
			StandardError:    245.6,
			DataPoints:       89,
			Intercept:        1234.5,
			PriceCoefficient: 123.4,
		},
		DataQuality: &CVDDataQuality{
			PointsAvailable:  100,
			PointsUsed:       89,
			WinsorizedPoints: 11,
			OutliersPct:      11.0,
			DataSpanHours:    6.5,
		},
	}

	analysis := result.GetDetailedAnalysis()
	assert.Contains(t, analysis, "ETH-USD")
	assert.Contains(t, analysis, "regression")
	assert.Contains(t, analysis, "true")
	assert.Contains(t, analysis, "-567.8")
	assert.Contains(t, analysis, "-2.3")
	assert.Contains(t, analysis, "15.2%")
	assert.Contains(t, analysis, "R²: 0.78")
	assert.Contains(t, analysis, "CVD = 1234.5 + 123.4 × PriceChange")
	assert.Contains(t, analysis, "100 available, 89 used")
	assert.Contains(t, analysis, "Winsorized: 11 outliers")
}

func TestDefaultCVDConfig(t *testing.T) {
	config := DefaultCVDConfig()
	require.NotNil(t, config)

	// Check regression parameters
	assert.Equal(t, 50, config.MinDataPoints)
	assert.Equal(t, 5.0, config.WinsorizePctLower)
	assert.Equal(t, 95.0, config.WinsorizePctUpper)
	assert.Equal(t, 0.30, config.MinRSquared)
	assert.True(t, config.DailyRefitEnabled)

	// Check fallback parameters
	assert.Equal(t, "percentile", config.FallbackMethod)
	assert.Equal(t, 20, config.FallbackLookback)
	assert.Equal(t, 80.0, config.FallbackThreshold)

	// Check residual analysis parameters
	assert.Equal(t, 2.0, config.ResidualMinStdDev)
	assert.Equal(t, int64(3600), config.ResidualMaxAge)

	// Check performance limits
	assert.Equal(t, int64(200), config.MaxComputeTimeMs)
}

// Helper functions for test data generation

func generateSyntheticCVDData(count int, rSquared float64) []*CVDDataPoint {
	dataPoints := make([]*CVDDataPoint, 0, count)
	baseTime := time.Now()

	// Generate data with controlled R²
	noiseLevel := math.Sqrt(1.0 - rSquared) // Adjust noise to achieve target R²

	for i := 0; i < count; i++ {
		priceChange := (float64(i) - float64(count)/2.0) / 10.0 // Price changes around 0

		// True relationship: CVD = 1000 + 50 * priceChange
		trueCVD := 1000.0 + 50.0*priceChange

		// Add noise to achieve target R²
		noise := (rand.Float64() - 0.5) * noiseLevel * 200.0 // ±100 noise range
		observedCVD := trueCVD + noise

		dp := &CVDDataPoint{
			Timestamp:   baseTime.Add(time.Duration(i) * time.Minute),
			Price:       100.0 + priceChange,
			CVD:         observedCVD,
			PriceChange: priceChange,
			Volume:      1000000.0, // 1M volume baseline
		}
		dataPoints = append(dataPoints, dp)
	}

	return dataPoints
}

func generateNoisyCVDData(count int) []*CVDDataPoint {
	dataPoints := make([]*CVDDataPoint, 0, count)
	baseTime := time.Now()

	for i := 0; i < count; i++ {
		// Completely random relationship (no correlation)
		priceChange := (rand.Float64() - 0.5) * 4.0 // ±2.0 price change
		cvd := 800.0 + rand.Float64()*400.0         // Random CVD 800-1200

		dp := &CVDDataPoint{
			Timestamp:   baseTime.Add(time.Duration(i) * time.Minute),
			Price:       100.0 + priceChange,
			CVD:         cvd,
			PriceChange: priceChange,
			Volume:      1000000.0,
		}
		dataPoints = append(dataPoints, dp)
	}

	return dataPoints
}

// Simple random number generator for reproducible tests
var rand = &simpleRand{seed: 12345}

type simpleRand struct {
	seed int64
}

func (r *simpleRand) Float64() float64 {
	r.seed = (r.seed*1103515245 + 12345) & 0x7fffffff
	return float64(r.seed) / float64(0x7fffffff)
}
