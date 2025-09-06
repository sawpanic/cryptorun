package premove

import (
	"testing"
	"time"

	"cryptorun/src/application/premove"
)

func TestExecutionTracker_QualityScoring(t *testing.T) {
	t.Run("comprehensive_quality_metrics", func(t *testing.T) {
		// This test expects an ExecutionQualityTracker that doesn't exist yet
		tracker := premove.NewExecutionQualityTracker(premove.QualityConfig{
			SlippageBpsWeight:    0.4,
			FillTimeWeight:      0.3,
			SizeDeviationWeight: 0.2,
			RejectRateWeight:    0.1,
		})
		
		execution := premove.ExecutionRecord{
			ID:              "exec-001",
			Symbol:          "BTCUSD",
			IntendedPrice:   45000.0,
			ActualPrice:     45015.0, // 3.3 bps slippage
			IntendedSize:    1.0,
			ActualSize:      0.98,    // 2% partial fill
			TimeToFillMs:    2500,    // 2.5 second fill
			Status:          "filled",
			Timestamp:       time.Now(),
			PreMoveScore:    85.0,
			TriggerReason:   "pre_movement_detected",
		}
		
		quality, err := tracker.CalculateQualityScore(execution)
		if err != nil {
			t.Errorf("Quality calculation failed: %v", err)
		}
		
		// Should be good quality (low slippage, reasonable fill time)
		if quality.OverallScore < 80.0 {
			t.Errorf("Expected high quality score, got %.2f", quality.OverallScore)
		}
		
		if quality.SlippageComponent <= 0 {
			t.Error("Expected positive slippage component score")
		}
		
		if len(quality.ComponentBreakdown) != 4 {
			t.Errorf("Expected 4 quality components, got %d", len(quality.ComponentBreakdown))
		}
	})

	t.Run("slippage_tolerance_adaptation", func(t *testing.T) {
		adapter := premove.NewSlippageToleranceAdapter(premove.AdapterConfig{
			BaseToleranceBps:   30.0,
			VolatilityMultiplier: 2.0,
			ScoreBonus:         0.1, // 10 bps bonus per score point above 80
		})
		
		// High score candidate in volatile conditions
		candidate := premove.ExecutionCandidate{
			Symbol:           "ETHUSD",
			Score:           95.0,
			ExpectedSlippage: 25.0, // bps
			MarketConditions: premove.MarketConditions{
				Volatility:    0.08, // High volatility
				SpreadBps:     15.0,
				DepthRatio:    0.85,
			},
		}
		
		tolerance := adapter.CalculateTolerance(candidate)
		
		// Should get bonus for high score and volatility adjustment
		expectedMin := 30.0 + (95.0-80.0)*0.1 + 30.0*1.0 // base + score bonus + volatility
		if tolerance.MaxSlippageBps < expectedMin {
			t.Errorf("Expected tolerance >= %.1f, got %.1f", expectedMin, tolerance.MaxSlippageBps)
		}
		
		if !tolerance.AdaptiveMode {
			t.Error("Should be in adaptive mode for high volatility")
		}
	})

	t.Run("fill_time_optimization", func(t *testing.T) {
		optimizer := premove.NewFillTimeOptimizer(premove.OptimizerConfig{
			TargetFillTimeMs:     3000,
			AggressivenessLevels: 5,
			LearningRate:        0.1,
		})
		
		// Record historical executions
		executions := []premove.ExecutionRecord{
			{Symbol: "BTCUSD", TimeToFillMs: 5000, OrderType: "limit", Status: "filled"},
			{Symbol: "BTCUSD", TimeToFillMs: 2000, OrderType: "market", Status: "filled"},
			{Symbol: "BTCUSD", TimeToFillMs: 8000, OrderType: "limit", Status: "partial"},
		}
		
		for _, exec := range executions {
			err := optimizer.RecordExecution(exec)
			if err != nil {
				t.Errorf("Failed to record execution: %v", err)
			}
		}
		
		// Get optimized order parameters
		candidate := premove.ExecutionCandidate{
			Symbol: "BTCUSD",
			Score:  88.0,
			Size:   1.5,
		}
		
		params, err := optimizer.OptimizeOrderParams(candidate)
		if err != nil {
			t.Errorf("Order optimization failed: %v", err)
		}
		
		if params.OrderType == "" {
			t.Error("Expected order type recommendation")
		}
		
		if params.AggressivenessLevel < 1 || params.AggressivenessLevel > 5 {
			t.Errorf("Invalid aggressiveness level: %d", params.AggressivenessLevel)
		}
	})

	t.Run("market_impact_modeling", func(t *testing.T) {
		modeler := premove.NewMarketImpactModeler(premove.ImpactModelConfig{
			Model:              "almgren_chriss",
			TemporaryImpactRate: 0.5,
			PermanentImpactRate: 0.1,
			VolatilityHalfLife:  30 * time.Minute,
		})
		
		trade := premove.PlannedTrade{
			Symbol:         "ETHUSD",
			Size:           5.0,
			Price:          3000.0,
			ADV:            1000000.0,
			Volatility:     0.05,
			SpreadBps:      12.0,
		}
		
		impact, err := modeler.EstimateImpact(trade)
		if err != nil {
			t.Errorf("Impact estimation failed: %v", err)
		}
		
		if impact.TotalImpactBps <= 0 {
			t.Errorf("Expected positive market impact, got %.2f", impact.TotalImpactBps)
		}
		
		if impact.TemporaryImpactBps <= 0 {
			t.Error("Expected positive temporary impact")
		}
		
		if impact.PermanentImpactBps < 0 {
			t.Error("Permanent impact should be non-negative")
		}
		
		// Total should be sum of components
		expectedTotal := impact.TemporaryImpactBps + impact.PermanentImpactBps
		if abs(impact.TotalImpactBps-expectedTotal) > 0.01 {
			t.Error("Impact components don't sum to total")
		}
	})
}

func TestExecutionTracker_RecoveryMode(t *testing.T) {
	t.Run("failure_pattern_detection", func(t *testing.T) {
		detector := premove.NewFailurePatternDetector(premove.PatternConfig{
			MinSampleSize:      10,
			FailureThreshold:   0.3, // 30% failure rate
			PatternLookback:    24 * time.Hour,
		})
		
		// Simulate pattern of failures
		now := time.Now()
		for i := 0; i < 15; i++ {
			execution := premove.ExecutionRecord{
				Symbol:    "SOLUSD",
				Status:    "rejected",
				Timestamp: now.Add(-time.Duration(i) * time.Hour),
				Reason:    "insufficient_liquidity",
			}
			
			detector.RecordExecution(execution)
		}
		
		patterns := detector.DetectPatterns("SOLUSD")
		if len(patterns) == 0 {
			t.Error("Expected failure patterns to be detected")
		}
		
		pattern := patterns[0]
		if pattern.FailureRate < 0.8 {
			t.Errorf("Expected high failure rate, got %.2f", pattern.FailureRate)
		}
		
		if pattern.DominantReason != "insufficient_liquidity" {
			t.Errorf("Expected 'insufficient_liquidity' as dominant reason, got %s", pattern.DominantReason)
		}
	})

	t.Run("recovery_strategy_selection", func(t *testing.T) {
		selector := premove.NewRecoveryStrategySelector(premove.RecoveryConfig{
			Strategies: []premove.RecoveryStrategy{
				{Name: "reduce_size", Trigger: "size_rejection", Effectiveness: 0.8},
				{Name: "increase_patience", Trigger: "fill_timeout", Effectiveness: 0.7},
				{Name: "switch_venue", Trigger: "liquidity_shortage", Effectiveness: 0.9},
			},
		})
		
		failure := premove.ExecutionFailure{
			Symbol:          "ADAUSD",
			FailureType:     "size_rejection",
			RecentAttempts:  3,
			AvgFailureRate:  0.6,
			Context:         premove.ExecutionContext{Venue: "kraken", Session: "london"},
		}
		
		strategy, err := selector.SelectStrategy(failure)
		if err != nil {
			t.Errorf("Strategy selection failed: %v", err)
		}
		
		if strategy.Name != "reduce_size" {
			t.Errorf("Expected 'reduce_size' strategy for size_rejection, got %s", strategy.Name)
		}
		
		if strategy.Confidence <= 0.5 {
			t.Errorf("Expected high confidence in strategy, got %.2f", strategy.Confidence)
		}
	})

	t.Run("adaptive_recovery_cooldown", func(t *testing.T) {
		cooldown := premove.NewAdaptiveCooldown(premove.CooldownConfig{
			BaseCooldown:      5 * time.Minute,
			MaxCooldown:       30 * time.Minute,
			BackoffMultiplier: 1.5,
			SuccessDecayRate:  0.8,
		})
		
		// Simulate consecutive failures
		for i := 0; i < 4; i++ {
			cooldown.RecordFailure("BTCUSD", "execution_timeout")
		}
		
		duration := cooldown.GetCooldownDuration("BTCUSD")
		
		// Should be longer than base cooldown due to consecutive failures
		if duration <= 5*time.Minute {
			t.Error("Cooldown should increase after consecutive failures")
		}
		
		// Record successful execution
		cooldown.RecordSuccess("BTCUSD")
		
		newDuration := cooldown.GetCooldownDuration("BTCUSD")
		if newDuration >= duration {
			t.Error("Cooldown should decrease after successful execution")
		}
		
		// Check if ready for retry
		ready := cooldown.IsReadyForRetry("BTCUSD")
		if !ready && newDuration == 0 {
			t.Error("Should be ready for retry if cooldown is zero")
		}
	})

	t.Run("quality_degradation_alerts", func(t *testing.T) {
		monitor := premove.NewQualityDegradationMonitor(premove.MonitorConfig{
			QualityThreshold:    70.0,
			DegradationWindow:   time.Hour,
			MinSampleSize:      5,
			AlertThreshold:     0.2, // 20% degradation
		})
		
		// Record declining quality scores
		baseTime := time.Now()
		scores := []float64{90.0, 85.0, 80.0, 70.0, 60.0, 55.0}
		
		for i, score := range scores {
			execution := premove.ExecutionRecord{
				Symbol:        "ETHUSD",
				Timestamp:     baseTime.Add(time.Duration(i) * 10 * time.Minute),
				QualityScore:  score,
				Status:        "filled",
			}
			
			alert, err := monitor.CheckDegradation(execution)
			if err != nil {
				t.Errorf("Degradation check failed: %v", err)
			}
			
			if i >= 4 && alert == nil {
				t.Error("Expected degradation alert for declining quality")
			}
		}
		
		// Get degradation summary
		summary := monitor.GetDegradationSummary("ETHUSD")
		if summary.RecentAvgQuality >= summary.BaselineQuality {
			t.Error("Recent quality should be lower than baseline")
		}
		
		if summary.DegradationPercent <= 0 {
			t.Errorf("Expected positive degradation percentage, got %.2f", summary.DegradationPercent)
		}
	})
}

func TestExecutionTracker_PerformanceAnalytics(t *testing.T) {
	t.Run("p99_latency_tracking", func(t *testing.T) {
		tracker := premove.NewLatencyTracker(premove.LatencyConfig{
			Percentiles:     []float64{0.5, 0.95, 0.99},
			SlidingWindow:   time.Hour,
			MinSampleSize:  50,
		})
		
		// Simulate execution latencies
		latencies := []time.Duration{
			100 * time.Millisecond,
			150 * time.Millisecond,
			200 * time.Millisecond,
			500 * time.Millisecond, // Outlier
			80 * time.Millisecond,
			120 * time.Millisecond,
		}
		
		for _, latency := range latencies {
			execution := premove.ExecutionRecord{
				Symbol:       "BTCUSD",
				TimeToFillMs: int64(latency / time.Millisecond),
				Timestamp:    time.Now(),
			}
			
			tracker.RecordLatency(execution)
		}
		
		stats := tracker.GetLatencyStats("BTCUSD")
		if stats.P50 <= 0 {
			t.Error("Expected positive P50 latency")
		}
		
		if stats.P99 <= stats.P95 {
			t.Error("P99 should be greater than P95")
		}
		
		if stats.P99 > 400*time.Millisecond {
			t.Errorf("P99 latency too high: %v", stats.P99)
		}
	})

	t.Run("venue_performance_comparison", func(t *testing.T) {
		comparator := premove.NewVenuePerformanceComparator(premove.ComparatorConfig{
			Metrics: []string{"slippage", "fill_rate", "latency", "rejection_rate"},
			Window:  24 * time.Hour,
		})
		
		// Record executions across venues
		venues := []string{"kraken", "coinbase", "binance"}
		for _, venue := range venues {
			for i := 0; i < 10; i++ {
				execution := premove.ExecutionRecord{
					Symbol:       "ETHUSD",
					Venue:        venue,
					SlippageBps:  5.0 + float64(i%3), // Vary slippage
					Status:       "filled",
					TimeToFillMs: 2000 + int64(i*100),
					Timestamp:    time.Now(),
				}
				
				comparator.RecordExecution(execution)
			}
		}
		
		comparison := comparator.CompareVenues("ETHUSD")
		if len(comparison.VenueRankings) != 3 {
			t.Errorf("Expected 3 venue rankings, got %d", len(comparison.VenueRankings))
		}
		
		// Check ranking consistency
		bestVenue := comparison.VenueRankings[0]
		if bestVenue.OverallScore <= 0 {
			t.Error("Best venue should have positive score")
		}
		
		// Should have detailed metrics per venue
		for _, ranking := range comparison.VenueRankings {
			if len(ranking.MetricScores) == 0 {
				t.Errorf("Venue %s missing metric scores", ranking.VenueName)
			}
		}
	})

	t.Run("profitability_attribution", func(t *testing.T) {
		attributor := premove.NewProfitabilityAttributor(premove.AttributionConfig{
			HoldingPeriods: []time.Duration{1 * time.Hour, 4 * time.Hour, 24 * time.Hour},
			BenchmarkType:  "market_neutral",
		})
		
		// Record execution with subsequent price movement
		execution := premove.ExecutionRecord{
			ID:           "exec-prof-001",
			Symbol:       "BTCUSD",
			ActualPrice:  45000.0,
			ActualSize:   1.0,
			Status:       "filled",
			Timestamp:    time.Now().Add(-2 * time.Hour),
			PreMoveScore: 88.0,
		}
		
		// Simulate price movements
		priceUpdates := []premove.PriceUpdate{
			{Symbol: "BTCUSD", Price: 45100.0, Timestamp: time.Now().Add(-1 * time.Hour)}, // +100 after 1h
			{Symbol: "BTCUSD", Price: 45200.0, Timestamp: time.Now()},                    // +200 after 2h
		}
		
		attribution, err := attributor.AttributeProfitability(execution, priceUpdates)
		if err != nil {
			t.Errorf("Profitability attribution failed: %v", err)
		}
		
		if attribution.TotalPnL <= 0 {
			t.Errorf("Expected positive PnL, got %.2f", attribution.TotalPnL)
		}
		
		// Should have attribution for each holding period
		if len(attribution.HoldingPeriodPnL) == 0 {
			t.Error("Expected holding period PnL breakdown")
		}
		
		// Check score correlation
		if attribution.ScoreCorrelation == 0 {
			t.Error("Expected non-zero correlation between score and profitability")
		}
	})
}

// Helper function for absolute value
func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}