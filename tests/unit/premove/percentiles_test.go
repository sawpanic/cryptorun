package premove

import (
	"testing"
	"time"

	"cryptorun/src/application/premove"
)

func TestPercentileEngine_IsotonicCalibration(t *testing.T) {
	t.Run("pool_adjacent_violators", func(t *testing.T) {
		// This test expects an IsotonicCalibrator that doesn't exist yet
		calibrator := premove.NewIsotonicCalibrator(premove.CalibrationConfig{
			MinObservations: 10,
			Smoothing:      true,
			ConfidenceLevel: 0.95,
		})
		
		// Create score-outcome pairs for calibration
		observations := []premove.CalibrationObservation{
			{Score: 60.0, Outcome: false}, // Low score, no movement
			{Score: 65.0, Outcome: false},
			{Score: 70.0, Outcome: true},  // Movement occurred
			{Score: 75.0, Outcome: false},
			{Score: 75.0, Outcome: true},
			{Score: 80.0, Outcome: true},
			{Score: 85.0, Outcome: true},
			{Score: 90.0, Outcome: true},
			{Score: 95.0, Outcome: true},
			{Score: 100.0, Outcome: true},
		}
		
		curve, err := calibrator.FitIsotonicCurve(observations)
		if err != nil {
			t.Errorf("Isotonic calibration failed: %v", err)
		}
		
		// Check monotonicity property
		for i := 1; i < len(curve.Points); i++ {
			prev := curve.Points[i-1]
			curr := curve.Points[i]
			
			if curr.Score > prev.Score && curr.Probability < prev.Probability {
				t.Errorf("Curve not monotonic: score %.1f->%.1f, prob %.3f->%.3f",
					prev.Score, curr.Score, prev.Probability, curr.Probability)
			}
		}
		
		// Check probability bounds
		for _, point := range curve.Points {
			if point.Probability < 0.0 || point.Probability > 1.0 {
				t.Errorf("Invalid probability %.3f at score %.1f", point.Probability, point.Score)
			}
		}
	})

	t.Run("binomial_confidence_intervals", func(t *testing.T) {
		calculator := premove.NewBinomialConfidenceCalculator(premove.ConfidenceConfig{
			Method:          "wilson",
			ConfidenceLevel: 0.95,
			ContinuityCorrection: true,
		})
		
		// Test various sample sizes and success rates
		testCases := []struct {
			successes int
			trials    int
			expected  float64
		}{
			{5, 10, 0.5},   // 50% success rate
			{8, 10, 0.8},   // 80% success rate
			{1, 10, 0.1},   // 10% success rate
			{25, 50, 0.5},  // Larger sample
		}
		
		for _, tc := range testCases {
			interval, err := calculator.CalculateInterval(tc.successes, tc.trials)
			if err != nil {
				t.Errorf("Confidence interval calculation failed for %d/%d: %v",
					tc.successes, tc.trials, err)
			}
			
			// Check bounds
			if interval.Lower < 0.0 || interval.Upper > 1.0 {
				t.Errorf("Invalid interval bounds [%.3f, %.3f] for %d/%d",
					interval.Lower, interval.Upper, tc.successes, tc.trials)
			}
			
			if interval.Lower > interval.Upper {
				t.Errorf("Invalid interval: lower %.3f > upper %.3f",
					interval.Lower, interval.Upper)
			}
			
			// Point estimate should be within interval
			pointEst := float64(tc.successes) / float64(tc.trials)
			if pointEst < interval.Lower || pointEst > interval.Upper {
				t.Errorf("Point estimate %.3f outside interval [%.3f, %.3f]",
					pointEst, interval.Lower, interval.Upper)
			}
		}
	})

	t.Run("regime_aware_calibration", func(t *testing.T) {
		regimeCalibrator := premove.NewRegimeAwareCalibrator(premove.RegimeCalibrationConfig{
			Regimes:             []string{"trending_bull", "choppy", "high_vol"},
			MinObservationsPerRegime: 20,
			RegimeDetectionWindow:    4 * time.Hour,
		})
		
		// Create regime-specific observations
		observations := map[string][]premove.RegimeObservation{
			"trending_bull": {
				{Score: 80.0, Outcome: true, Regime: "trending_bull", Timestamp: time.Now()},
				{Score: 85.0, Outcome: true, Regime: "trending_bull", Timestamp: time.Now()},
				{Score: 75.0, Outcome: false, Regime: "trending_bull", Timestamp: time.Now()},
			},
			"choppy": {
				{Score: 90.0, Outcome: false, Regime: "choppy", Timestamp: time.Now()},
				{Score: 95.0, Outcome: true, Regime: "choppy", Timestamp: time.Now()},
				{Score: 85.0, Outcome: false, Regime: "choppy", Timestamp: time.Now()},
			},
		}
		
		curves, err := regimeCalibrator.FitRegimeCurves(observations)
		if err != nil {
			t.Errorf("Regime calibration failed: %v", err)
		}
		
		if len(curves) == 0 {
			t.Error("Expected calibration curves for regimes")
		}
		
		// Check that different regimes have different curves
		bullCurve := curves["trending_bull"]
		choppyCurve := curves["choppy"]
		
		if bullCurve == nil || choppyCurve == nil {
			t.Error("Missing calibration curves for regimes")
		}
		
		// Compare probabilities at same score
		bullProb := regimeCalibrator.GetProbability(85.0, "trending_bull")
		choppyProb := regimeCalibrator.GetProbability(85.0, "choppy")
		
		if bullProb == choppyProb {
			t.Error("Different regimes should potentially have different probabilities")
		}
	})

	t.Run("temporal_decay_weighting", func(t *testing.T) {
		decayCalibrator := premove.NewTemporalDecayCalibrator(premove.DecayConfig{
			HalfLife:        30 * 24 * time.Hour, // 30 days
			MinWeight:       0.01,
			MaxObservations: 1000,
		})
		
		// Create observations with timestamps
		now := time.Now()
		observations := []premove.TimedObservation{
			{Score: 80.0, Outcome: true, Timestamp: now.Add(-10 * 24 * time.Hour)}, // Recent
			{Score: 80.0, Outcome: false, Timestamp: now.Add(-60 * 24 * time.Hour)}, // Old
			{Score: 85.0, Outcome: true, Timestamp: now.Add(-5 * 24 * time.Hour)},  // Very recent
		}
		
		curve, err := decayCalibrator.FitWithDecay(observations)
		if err != nil {
			t.Errorf("Temporal decay calibration failed: %v", err)
		}
		
		// Recent observations should have higher effective weight
		weights := decayCalibrator.GetObservationWeights(observations)
		if len(weights) != len(observations) {
			t.Errorf("Expected %d weights, got %d", len(observations), len(weights))
		}
		
		// Most recent should have highest weight
		if weights[2] <= weights[1] || weights[2] <= weights[0] {
			t.Error("Most recent observation should have highest weight")
		}
		
		// Oldest should have lowest weight
		if weights[1] >= weights[0] {
			t.Error("Older observation should have lower weight")
		}
		
		// Check curve quality metrics
		if curve.RSquared <= 0 {
			t.Errorf("Expected positive R-squared, got %.3f", curve.RSquared)
		}
	})
}

func TestPercentileEngine_HitRateAnalysis(t *testing.T) {
	t.Run("state_based_hit_rates", func(t *testing.T) {
		analyzer := premove.NewHitRateAnalyzer(premove.AnalyzerConfig{
			States:           []string{"WATCH", "PREPARE", "PRIME", "EXECUTE"},
			MovementThreshold: 0.05, // 5% movement
			TimeHorizon:      48 * time.Hour,
		})
		
		// Create state-based observations
		records := []premove.StateRecord{
			{State: "WATCH", Score: 65.0, Symbol: "BTCUSD", Timestamp: time.Now(), MovementOccurred: false},
			{State: "PREPARE", Score: 75.0, Symbol: "ETHUSD", Timestamp: time.Now(), MovementOccurred: true},
			{State: "PRIME", Score: 85.0, Symbol: "SOLUSD", Timestamp: time.Now(), MovementOccurred: true},
			{State: "EXECUTE", Score: 95.0, Symbol: "ADAUSD", Timestamp: time.Now(), MovementOccurred: true},
		}
		
		hitRates, err := analyzer.CalculateHitRates(records)
		if err != nil {
			t.Errorf("Hit rate calculation failed: %v", err)
		}
		
		if len(hitRates) == 0 {
			t.Error("Expected hit rates for states")
		}
		
		// Higher states should generally have higher hit rates
		watchRate := hitRates["WATCH"]
		executeRate := hitRates["EXECUTE"]
		
		if executeRate.Rate <= watchRate.Rate {
			t.Error("EXECUTE state should have higher hit rate than WATCH")
		}
		
		// Check confidence intervals
		for state, rate := range hitRates {
			if rate.ConfidenceInterval.Lower > rate.Rate || rate.Rate > rate.ConfidenceInterval.Upper {
				t.Errorf("Hit rate %.3f outside confidence interval [%.3f, %.3f] for state %s",
					rate.Rate, rate.ConfidenceInterval.Lower, rate.ConfidenceInterval.Upper, state)
			}
		}
	})

	t.Run("stratified_analysis", func(t *testing.T) {
		stratifier := premove.NewStratifiedAnalyzer(premove.StratificationConfig{
			Dimensions: []string{"sector", "market_cap", "volatility_quartile"},
			MinSampleSize: 5,
		})
		
		// Create stratified observations
		observations := []premove.StratifiedObservation{
			{
				Score:     80.0,
				Outcome:   true,
				Strata:    map[string]string{"sector": "L1", "market_cap": "large", "volatility_quartile": "Q2"},
				Timestamp: time.Now(),
			},
			{
				Score:     75.0,
				Outcome:   false,
				Strata:    map[string]string{"sector": "DeFi", "market_cap": "medium", "volatility_quartile": "Q3"},
				Timestamp: time.Now(),
			},
		}
		
		analysis, err := stratifier.AnalyzeByStrata(observations)
		if err != nil {
			t.Errorf("Stratified analysis failed: %v", err)
		}
		
		if len(analysis.StrataResults) == 0 {
			t.Error("Expected stratified results")
		}
		
		// Check for significant differences between strata
		significantDiffs := stratifier.FindSignificantDifferences(analysis)
		if len(significantDiffs) == 0 {
			t.Log("No significant differences found between strata (may be expected with small sample)")
		}
	})

	t.Run("rolling_window_analysis", func(t *testing.T) {
		rollingAnalyzer := premove.NewRollingWindowAnalyzer(premove.RollingConfig{
			WindowSize:   7 * 24 * time.Hour, // 7 days
			StepSize:     24 * time.Hour,     // 1 day
			MinSampleSize: 10,
		})
		
		// Create time series of observations
		baseTime := time.Now().Add(-30 * 24 * time.Hour) // 30 days ago
		observations := []premove.TimedObservation{}
		
		for i := 0; i < 30; i++ {
			obs := premove.TimedObservation{
				Score:     70.0 + float64(i%20), // Varying scores
				Outcome:   i%3 == 0,             // 33% hit rate
				Timestamp: baseTime.Add(time.Duration(i) * 24 * time.Hour),
			}
			observations = append(observations, obs)
		}
		
		windows, err := rollingAnalyzer.AnalyzeRollingWindows(observations)
		if err != nil {
			t.Errorf("Rolling window analysis failed: %v", err)
		}
		
		if len(windows) == 0 {
			t.Error("Expected rolling window results")
		}
		
		// Check window progression
		for i, window := range windows {
			if window.StartTime.After(window.EndTime) {
				t.Errorf("Window %d: start time after end time", i)
			}
			
			if window.SampleSize < 1 {
				t.Errorf("Window %d: invalid sample size %d", i, window.SampleSize)
			}
		}
		
		// Check for performance degradation over time
		degradation := rollingAnalyzer.DetectPerformanceDegradation(windows)
		if degradation.IsSignificant {
			t.Logf("Performance degradation detected: %.1f%% decline", degradation.DeclinePercent)
		}
	})
}

func TestPercentileEngine_DistributionFitting(t *testing.T) {
	t.Run("score_distribution_modeling", func(t *testing.T) {
		modeler := premove.NewScoreDistributionModeler(premove.ModelingConfig{
			Distributions: []string{"beta", "gamma", "lognormal"},
			FitMethod:    "maximum_likelihood",
			GoodnessTests: []string{"kolmogorov_smirnov", "anderson_darling"},
		})
		
		// Generate synthetic score data
		scores := []float64{
			65.0, 70.0, 72.0, 75.0, 78.0, 80.0, 82.0, 85.0, 88.0, 90.0,
			75.0, 77.0, 79.0, 81.0, 83.0, 86.0, 89.0, 91.0, 94.0, 96.0,
		}
		
		models, err := modeler.FitDistributions(scores)
		if err != nil {
			t.Errorf("Distribution fitting failed: %v", err)
		}
		
		if len(models) == 0 {
			t.Error("Expected fitted distribution models")
		}
		
		// Find best fitting distribution
		bestModel := modeler.SelectBestModel(models)
		if bestModel == nil {
			t.Error("Expected best model selection")
		}
		
		if bestModel.GoodnessOfFit <= 0 {
			t.Errorf("Expected positive goodness of fit, got %.3f", bestModel.GoodnessOfFit)
		}
		
		// Test distribution properties
		percentiles := []float64{0.25, 0.5, 0.75, 0.95}
		for _, p := range percentiles {
			value := bestModel.Quantile(p)
			if value <= 0 {
				t.Errorf("Invalid quantile value %.2f for percentile %.2f", value, p)
			}
		}
	})

	t.Run("tail_behavior_analysis", func(t *testing.T) {
		tailAnalyzer := premove.NewTailBehaviorAnalyzer(premove.TailConfig{
			UpperTailThreshold: 0.95,
			LowerTailThreshold: 0.05,
			ExtremeValueMethod: "peaks_over_threshold",
		})
		
		// Create data with extreme values
		data := []float64{
			50.0, 55.0, 60.0, 65.0, 70.0, 75.0, 80.0, 85.0, 90.0, 95.0,
			98.0, 99.0, 100.0, 105.0, // High tail
			45.0, 40.0, 35.0, 30.0,   // Low tail
		}
		
		analysis, err := tailAnalyzer.AnalyzeTails(data)
		if err != nil {
			t.Errorf("Tail analysis failed: %v", err)
		}
		
		if analysis.UpperTailIndex <= 0 {
			t.Errorf("Expected positive upper tail index, got %.3f", analysis.UpperTailIndex)
		}
		
		if len(analysis.ExtremeValues) == 0 {
			t.Error("Expected extreme values to be identified")
		}
		
		// Check tail risk estimates
		var95 := analysis.ValueAtRisk(0.95)
		var99 := analysis.ValueAtRisk(0.99)
		
		if var99 <= var95 {
			t.Error("VaR(99%) should be greater than VaR(95%)")
		}
	})

	t.Run("mixture_model_fitting", func(t *testing.T) {
		mixtureModeler := premove.NewMixtureModeler(premove.MixtureConfig{
			MaxComponents:    3,
			ComponentType:    "gaussian",
			InitMethod:      "kmeans",
			ConvergenceTol:  1e-6,
			MaxIterations:   100,
		})
		
		// Create bimodal data (two score clusters)
		lowScores := []float64{60.0, 62.0, 65.0, 67.0, 70.0}
		highScores := []float64{85.0, 87.0, 90.0, 92.0, 95.0}
		
		data := append(lowScores, highScores...)
		
		mixture, err := mixtureModeler.FitMixture(data)
		if err != nil {
			t.Errorf("Mixture model fitting failed: %v", err)
		}
		
		if len(mixture.Components) < 2 {
			t.Error("Expected at least 2 mixture components for bimodal data")
		}
		
		// Check component weights sum to 1
		totalWeight := 0.0
		for _, component := range mixture.Components {
			totalWeight += component.Weight
			
			if component.Weight <= 0 {
				t.Error("Component weights should be positive")
			}
		}
		
		if abs(totalWeight-1.0) > 1e-6 {
			t.Errorf("Component weights should sum to 1, got %.6f", totalWeight)
		}
		
		// Test probability density calculation
		testScore := 80.0
		density := mixture.Density(testScore)
		if density <= 0 {
			t.Errorf("Expected positive density for score %.1f", testScore)
		}
	})

	t.Run("bayesian_updating", func(t *testing.T) {
		bayesianUpdater := premove.NewBayesianUpdater(premove.BayesianConfig{
			PriorDistribution: "beta",
			PriorParameters:   []float64{2.0, 5.0}, // Beta(2,5) - skeptical prior
			UpdateMethod:     "conjugate",
		})
		
		// Start with prior belief
		prior := bayesianUpdater.GetPriorBelief()
		if prior.Mean >= 0.5 {
			t.Error("Skeptical prior should have mean < 0.5")
		}
		
		// Update with positive evidence
		evidence := []premove.Evidence{
			{Outcome: true, Weight: 1.0},
			{Outcome: true, Weight: 1.0},
			{Outcome: false, Weight: 1.0},
			{Outcome: true, Weight: 1.0},
		}
		
		posterior, err := bayesianUpdater.UpdateBelief(evidence)
		if err != nil {
			t.Errorf("Bayesian update failed: %v", err)
		}
		
		// Posterior mean should be higher than prior (3 successes out of 4)
		if posterior.Mean <= prior.Mean {
			t.Error("Posterior mean should increase with positive evidence")
		}
		
		// Check credible interval
		if posterior.CredibleInterval.Lower >= posterior.CredibleInterval.Upper {
			t.Error("Invalid credible interval bounds")
		}
		
		if posterior.CredibleInterval.Lower > posterior.Mean || posterior.Mean > posterior.CredibleInterval.Upper {
			t.Error("Mean should be within credible interval")
		}
	})
}