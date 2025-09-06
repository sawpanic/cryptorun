package premove

import (
	"testing"
	"time"

	"cryptorun/src/application/premove"
)

func TestCVDResidualTracker_QualityMetrics(t *testing.T) {
	t.Run("r_squared_calculation", func(t *testing.T) {
		// This test expects a CVDResidualTracker that doesn't exist yet
		tracker := premove.NewCVDResidualTracker(premove.CVDConfig{
			WindowSize:        24 * time.Hour,
			MinObservations:   50,
			QualityThreshold:  0.7, // R² threshold
		})
		
		// Create synthetic CVD and price data with correlation
		dataPoints := []premove.CVDPricePoint{
			{Timestamp: time.Now().Add(-23 * time.Hour), CVD: 100000, Price: 45000.0},
			{Timestamp: time.Now().Add(-22 * time.Hour), CVD: 105000, Price: 45100.0},
			{Timestamp: time.Now().Add(-21 * time.Hour), CVD: 95000, Price: 44900.0},
			{Timestamp: time.Now().Add(-20 * time.Hour), CVD: 110000, Price: 45200.0},
			{Timestamp: time.Now().Add(-19 * time.Hour), CVD: 115000, Price: 45300.0},
		}
		
		rsquared, err := tracker.CalculateRSquared("BTCUSD", dataPoints)
		if err != nil {
			t.Errorf("R-squared calculation failed: %v", err)
		}
		
		if rsquared < 0.0 || rsquared > 1.0 {
			t.Errorf("Invalid R-squared value: %.3f", rsquared)
		}
		
		// With correlated data, should have reasonable R²
		if rsquared < 0.3 {
			t.Errorf("Expected higher R-squared for correlated data, got %.3f", rsquared)
		}
	})

	t.Run("daily_quality_tracking", func(t *testing.T) {
		qualityTracker := premove.NewDailyQualityTracker(premove.QualityTrackingConfig{
			DegradationThreshold: 0.2, // 20% R² drop
			AlertThreshold:       0.5, // Alert if R² < 0.5
			MovingAverageWindow:  7,   // 7-day MA
		})
		
		// Simulate declining quality over days
		baseTime := time.Now().Add(-10 * 24 * time.Hour)
		for day := 0; day < 10; day++ {
			date := baseTime.Add(time.Duration(day) * 24 * time.Hour)
			
			// Simulate declining R² over time
			rsquared := 0.8 - float64(day)*0.05 // Starts at 0.8, declines by 0.05/day
			
			measurement := premove.DailyQualityMeasurement{
				Date:      date,
				Symbol:    "ETHUSD",
				RSquared:  rsquared,
				SampleSize: 100 + day*10,
			}
			
			alert, err := qualityTracker.RecordDailyQuality(measurement)
			if err != nil {
				t.Errorf("Quality tracking failed for day %d: %v", day, err)
			}
			
			// Should alert when R² drops below threshold
			if rsquared < 0.5 && alert == nil {
				t.Errorf("Expected quality alert on day %d (R²=%.3f)", day, rsquared)
			}
		}
		
		// Get quality trend
		trend := qualityTracker.GetQualityTrend("ETHUSD", 7*24*time.Hour)
		if trend.Slope >= 0 {
			t.Error("Expected negative quality trend (declining R²)")
		}
		
		if trend.SignificanceLevel > 0.05 {
			t.Errorf("Quality decline should be significant, p-value: %.4f", trend.SignificanceLevel)
		}
	})

	t.Run("regime_specific_quality", func(t *testing.T) {
		regimeTracker := premove.NewRegimeSpecificQualityTracker(premove.RegimeQualityConfig{
			Regimes:           []string{"trending_bull", "choppy", "high_vol"},
			MinSamplePerRegime: 30,
			ComparisonMethod:  "anova",
		})
		
		// Record quality measurements across different regimes
		regimeData := map[string][]float64{
			"trending_bull": {0.85, 0.83, 0.87, 0.82, 0.86},
			"choppy":        {0.65, 0.60, 0.68, 0.62, 0.66},
			"high_vol":      {0.45, 0.48, 0.42, 0.50, 0.46},
		}
		
		for regime, rsquaredValues := range regimeData {
			for i, rsquared := range rsquaredValues {
				measurement := premove.RegimeQualityMeasurement{
					Symbol:     "SOLUSD",
					Regime:     regime,
					RSquared:   rsquared,
					Timestamp:  time.Now().Add(-time.Duration(i) * time.Hour),
					SampleSize: 50,
				}
				
				regimeTracker.RecordMeasurement(measurement)
			}
		}
		
		// Analyze differences between regimes
		analysis, err := regimeTracker.AnalyzeRegimeDifferences("SOLUSD")
		if err != nil {
			t.Errorf("Regime analysis failed: %v", err)
		}
		
		if len(analysis.RegimeAverages) != 3 {
			t.Errorf("Expected 3 regime averages, got %d", len(analysis.RegimeAverages))
		}
		
		// Bull regime should have highest quality
		bullQuality := analysis.RegimeAverages["trending_bull"]
		choppyQuality := analysis.RegimeAverages["choppy"]
		
		if bullQuality <= choppyQuality {
			t.Error("Trending bull regime should have higher CVD quality than choppy")
		}
		
		// Check statistical significance
		if analysis.SignificantDifference && analysis.PValue > 0.05 {
			t.Error("Significant difference flag inconsistent with p-value")
		}
	})

	t.Run("residual_autocorrelation", func(t *testing.T) {
		autocorrAnalyzer := premove.NewResidualAutocorrelationAnalyzer(premove.AutocorrelationConfig{
			MaxLag:          24, // 24 periods
			SignificanceLevel: 0.05,
			TestType:         "ljung_box",
		})
		
		// Generate residuals with some autocorrelation
		residuals := []float64{
			0.1, 0.05, 0.08, -0.02, 0.03, 0.12, 0.07, -0.01,
			0.09, 0.04, 0.11, -0.03, 0.06, 0.13, 0.08, -0.02,
			0.1, 0.05, 0.09, -0.01, 0.04, 0.14, 0.06, 0.02,
		}
		
		analysis, err := autocorrAnalyzer.AnalyzeAutocorrelation("ADAUSD", residuals)
		if err != nil {
			t.Errorf("Autocorrelation analysis failed: %v", err)
		}
		
		if len(analysis.Autocorrelations) == 0 {
			t.Error("Expected autocorrelation coefficients")
		}
		
		// Check bounds
		for lag, acf := range analysis.Autocorrelations {
			if acf < -1.0 || acf > 1.0 {
				t.Errorf("Invalid autocorrelation %.3f at lag %d", acf, lag)
			}
		}
		
		// Check for significant autocorrelation
		if analysis.HasSignificantAutocorrelation {
			t.Logf("Significant autocorrelation detected (p=%.4f)", analysis.LjungBoxPValue)
		}
		
		// Confidence bounds should be symmetric around 0
		if analysis.ConfidenceBounds.Upper <= 0 || analysis.ConfidenceBounds.Lower >= 0 {
			t.Error("Confidence bounds should straddle zero")
		}
	})
}

func TestCVDResidualTracker_SignalDegradation(t *testing.T) {
	t.Run("degradation_detection", func(t *testing.T) {
		detector := premove.NewSignalDegradationDetector(premove.DegradationConfig{
			LookbackPeriod:     30 * 24 * time.Hour, // 30 days
			BaselinePeriod:     7 * 24 * time.Hour,  // 7 days baseline
			DegradationThreshold: 0.15,             // 15% R² drop
			MinConfidence:       0.90,
		})
		
		// Create baseline period with good quality
		baselineStart := time.Now().Add(-37 * 24 * time.Hour)
		for day := 0; day < 7; day++ {
			measurement := premove.QualityMeasurement{
				Symbol:    "BTCUSD",
				Timestamp: baselineStart.Add(time.Duration(day) * 24 * time.Hour),
				RSquared:  0.80 + float64(day%3)*0.02, // Stable around 0.8
				SampleSize: 100,
			}
			
			detector.RecordMeasurement(measurement)
		}
		
		// Create recent period with degraded quality
		recentStart := time.Now().Add(-7 * 24 * time.Hour)
		for day := 0; day < 7; day++ {
			measurement := premove.QualityMeasurement{
				Symbol:    "BTCUSD",
				Timestamp: recentStart.Add(time.Duration(day) * 24 * time.Hour),
				RSquared:  0.60 - float64(day)*0.02, // Declining from 0.6
				SampleSize: 100,
			}
			
			detector.RecordMeasurement(measurement)
		}
		
		// Check for degradation
		degradation, err := detector.DetectDegradation("BTCUSD")
		if err != nil {
			t.Errorf("Degradation detection failed: %v", err)
		}
		
		if !degradation.IsSignificant {
			t.Error("Should detect significant degradation")
		}
		
		if degradation.PercentDecline <= 15.0 {
			t.Errorf("Expected >15%% decline, got %.1f%%", degradation.PercentDecline)
		}
		
		if degradation.Confidence < 0.90 {
			t.Errorf("Expected high confidence, got %.2f", degradation.Confidence)
		}
	})

	t.Run("recovery_detection", func(t *testing.T) {
		recoveryDetector := premove.NewRecoveryDetector(premove.RecoveryConfig{
			RecoveryThreshold:    0.10, // 10% improvement
			ConsecutivePeriods:   3,    // 3 consecutive improvements
			MinRecoveryDuration:  24 * time.Hour,
		})
		
		// Simulate degraded period followed by recovery
		baseTime := time.Now().Add(-10 * 24 * time.Hour)
		
		// Degraded period
		for day := 0; day < 5; day++ {
			measurement := premove.QualityMeasurement{
				Symbol:    "ETHUSD",
				Timestamp: baseTime.Add(time.Duration(day) * 24 * time.Hour),
				RSquared:  0.45 + float64(day%2)*0.02, // Low quality
				SampleSize: 80,
			}
			
			recoveryDetector.RecordMeasurement(measurement)
		}
		
		// Recovery period
		for day := 5; day < 10; day++ {
			measurement := premove.QualityMeasurement{
				Symbol:    "ETHUSD",
				Timestamp: baseTime.Add(time.Duration(day) * 24 * time.Hour),
				RSquared:  0.45 + float64(day-4)*0.08, // Improving quality
				SampleSize: 80,
			}
			
			recoveryDetector.RecordMeasurement(measurement)
		}
		
		// Check for recovery
		recovery, err := recoveryDetector.DetectRecovery("ETHUSD")
		if err != nil {
			t.Errorf("Recovery detection failed: %v", err)
		}
		
		if !recovery.IsRecovering {
			t.Error("Should detect quality recovery")
		}
		
		if recovery.ImprovementPercent <= 10.0 {
			t.Errorf("Expected >10%% improvement, got %.1f%%", recovery.ImprovementPercent)
		}
		
		if recovery.ConsecutivePeriods < 3 {
			t.Errorf("Expected >=3 consecutive improvements, got %d", recovery.ConsecutivePeriods)
		}
	})

	t.Run("pattern_exhaustion_monitoring", func(t *testing.T) {
		exhaustionMonitor := premove.NewPatternExhaustionMonitor(premove.ExhaustionConfig{
			ShortWindow:  7 * 24 * time.Hour,
			LongWindow:   30 * 24 * time.Hour,
			ThresholdRatio: 0.7, // Short window should be >70% of long window
			AlertThreshold: 0.6, // Alert if ratio drops below 60%
		})
		
		// Create long-term baseline
		longBaseTime := time.Now().Add(-35 * 24 * time.Hour)
		for day := 0; day < 30; day++ {
			measurement := premove.PatternMeasurement{
				Symbol:       "SOLUSD",
				Timestamp:    longBaseTime.Add(time.Duration(day) * 24 * time.Hour),
				PatternStrength: 0.75 + float64(day%5)*0.01, // Stable pattern strength
			}
			
			exhaustionMonitor.RecordMeasurement(measurement)
		}
		
		// Create recent period with weakening patterns
		recentBaseTime := time.Now().Add(-7 * 24 * time.Hour)
		for day := 0; day < 7; day++ {
			measurement := premove.PatternMeasurement{
				Symbol:       "SOLUSD",
				Timestamp:    recentBaseTime.Add(time.Duration(day) * 24 * time.Hour),
				PatternStrength: 0.50 - float64(day)*0.02, // Weakening patterns
			}
			
			exhaustionMonitor.RecordMeasurement(measurement)
		}
		
		// Check for pattern exhaustion
		exhaustion, err := exhaustionMonitor.CheckExhaustion("SOLUSD")
		if err != nil {
			t.Errorf("Pattern exhaustion check failed: %v", err)
		}
		
		if !exhaustion.IsExhausted {
			t.Error("Should detect pattern exhaustion")
		}
		
		if exhaustion.RatioToBaseline > 0.6 {
			t.Errorf("Expected ratio <0.6, got %.2f", exhaustion.RatioToBaseline)
		}
		
		if exhaustion.DegradationConfidence < 0.8 {
			t.Errorf("Expected high degradation confidence, got %.2f", exhaustion.DegradationConfidence)
		}
	})
}

func TestCVDResidualTracker_MarketMicrostructure(t *testing.T) {
	t.Run("order_flow_imbalance_correlation", func(t *testing.T) {
		correlator := premove.NewOrderFlowCorrelator(premove.OrderFlowConfig{
			ImbalanceThreshold: 0.6, // 60% buy/sell imbalance
			CorrelationWindow:  time.Hour,
			MinTradesRequired:  50,
		})
		
		// Create order flow and CVD data
		flows := []premove.OrderFlow{
			{Timestamp: time.Now().Add(-55 * time.Minute), BuyVolume: 600, SellVolume: 400, CVD: 100000},
			{Timestamp: time.Now().Add(-50 * time.Minute), BuyVolume: 700, SellVolume: 300, CVD: 105000},
			{Timestamp: time.Now().Add(-45 * time.Minute), BuyVolume: 550, SellVolume: 450, CVD: 103000},
			{Timestamp: time.Now().Add(-40 * time.Minute), BuyVolume: 800, SellVolume: 200, CVD: 108000},
		}
		
		correlation, err := correlator.CalculateFlowCorrelation("BTCUSD", flows)
		if err != nil {
			t.Errorf("Flow correlation calculation failed: %v", err)
		}
		
		if correlation.Coefficient < -1.0 || correlation.Coefficient > 1.0 {
			t.Errorf("Invalid correlation coefficient: %.3f", correlation.Coefficient)
		}
		
		// With buy-heavy flows and increasing CVD, should have positive correlation
		if correlation.Coefficient <= 0.3 {
			t.Errorf("Expected positive correlation between buy imbalance and CVD, got %.3f", 
				correlation.Coefficient)
		}
		
		if correlation.PValue > 0.05 {
			t.Logf("Correlation not statistically significant (p=%.4f)", correlation.PValue)
		}
	})

	t.Run("tick_level_analysis", func(t *testing.T) {
		tickAnalyzer := premove.NewTickLevelAnalyzer(premove.TickConfig{
			AggregationPeriod: 5 * time.Minute,
			MinTickCount:      100,
			VolumeWeighting:   true,
		})
		
		// Create tick-level trade data
		ticks := []premove.TickData{
			{Timestamp: time.Now().Add(-4*time.Minute), Price: 45000.0, Size: 0.5, Side: "buy"},
			{Timestamp: time.Now().Add(-4*time.Minute), Price: 45001.0, Size: 0.3, Side: "buy"},
			{Timestamp: time.Now().Add(-3*time.Minute), Price: 44999.0, Size: 0.7, Side: "sell"},
			{Timestamp: time.Now().Add(-2*time.Minute), Price: 45002.0, Size: 0.4, Side: "buy"},
		}
		
		analysis, err := tickAnalyzer.AnalyzeTicks("ETHUSD", ticks)
		if err != nil {
			t.Errorf("Tick analysis failed: %v", err)
		}
		
		if analysis.NetCVD == 0 {
			t.Error("Expected non-zero net CVD from tick analysis")
		}
		
		if analysis.VolumeWeightedPrice <= 0 {
			t.Error("Expected positive volume weighted price")
		}
		
		// Check buy/sell pressure metrics
		if analysis.BuyPressure+analysis.SellPressure != 1.0 {
			t.Errorf("Buy and sell pressure should sum to 1.0, got %.3f", 
				analysis.BuyPressure+analysis.SellPressure)
		}
		
		if len(analysis.PriceImpactBySize) == 0 {
			t.Error("Expected price impact analysis by trade size")
		}
	})

	t.Run("market_maker_detection", func(t *testing.T) {
		mmDetector := premove.NewMarketMakerDetector(premove.MarketMakerConfig{
			PassiveRatioThreshold: 0.7, // 70% passive fills
			SpreadCaptureRatio:    0.5, // 50% of spread captured
			MinVolumeThreshold:    1000.0,
		})
		
		// Simulate market maker activity patterns
		activities := []premove.TradingActivity{
			{
				Timestamp:    time.Now().Add(-30 * time.Minute),
				PassiveFills: 70,
				AggressiveFills: 30,
				SpreadCapture: 0.6,
				Volume:       1500.0,
			},
			{
				Timestamp:    time.Now().Add(-25 * time.Minute),
				PassiveFills: 80,
				AggressiveFills: 20,
				SpreadCapture: 0.7,
				Volume:       2000.0,
			},
		}
		
		detection, err := mmDetector.DetectMarketMaking("ADAUSD", activities)
		if err != nil {
			t.Errorf("Market maker detection failed: %v", err)
		}
		
		if !detection.IsLikelyMarketMaker {
			t.Error("Should detect market maker activity with high passive ratio")
		}
		
		if detection.Confidence < 0.6 {
			t.Errorf("Expected high confidence in MM detection, got %.2f", detection.Confidence)
		}
		
		if len(detection.IdentifiedPatterns) == 0 {
			t.Error("Expected identified market making patterns")
		}
		
		// Check specific patterns
		for _, pattern := range detection.IdentifiedPatterns {
			if pattern == "high_passive_ratio" {
				break
			}
		}
	})

	t.Run("latent_liquidity_estimation", func(t *testing.T) {
		liquidityEstimator := premove.NewLatentLiquidityEstimator(premove.LiquidityConfig{
			DepthLevels:       []float64{0.1, 0.5, 1.0, 2.0}, // % from mid
			EstimationMethod: "hawkes_process",
			HalfLifeMinutes:   30,
		})
		
		// Create order book snapshots
		snapshots := []premove.BookSnapshot{
			{
				Timestamp: time.Now().Add(-25 * time.Minute),
				Bids: []premove.OrderLevel{
					{Price: 45000.0, Size: 1.0},
					{Price: 44999.0, Size: 2.0},
					{Price: 44995.0, Size: 5.0},
				},
				Asks: []premove.OrderLevel{
					{Price: 45001.0, Size: 1.0},
					{Price: 45002.0, Size: 2.0},
					{Price: 45005.0, Size: 5.0},
				},
			},
		}
		
		estimation, err := liquidityEstimator.EstimateLatentLiquidity("BTCUSD", snapshots)
		if err != nil {
			t.Errorf("Latent liquidity estimation failed: %v", err)
		}
		
		if len(estimation.DepthEstimates) == 0 {
			t.Error("Expected depth estimates at different levels")
		}
		
		for level, estimate := range estimation.DepthEstimates {
			if estimate.VisibleLiquidity <= 0 {
				t.Errorf("Expected positive visible liquidity at level %.1f%%", level*100)
			}
			
			if estimate.HiddenLiquidityEst < 0 {
				t.Errorf("Hidden liquidity estimate should be non-negative at level %.1f%%", level*100)
			}
		}
		
		// Total liquidity should be sum of visible + hidden
		for _, estimate := range estimation.DepthEstimates {
			expectedTotal := estimate.VisibleLiquidity + estimate.HiddenLiquidityEst
			if abs(estimate.TotalLiquidityEst-expectedTotal) > 0.01 {
				t.Error("Total liquidity should equal visible + hidden")
			}
		}
	})
}