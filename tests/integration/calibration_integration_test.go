package integration

import (
	"context"
	"math"
	"testing"
	"time"

	"github.com/sawpanic/cryptorun/internal/score/calibration"
	"github.com/sawpanic/cryptorun/internal/score/composite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCalibrationEndToEnd tests the complete calibration workflow
func TestCalibrationEndToEnd(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Setup calibration system
	config := calibration.DefaultCalibrationConfig()
	config.MinSamples = 50
	config.RefreshInterval = 100 * time.Millisecond // Fast refresh for testing
	
	harness := calibration.NewCalibrationHarness(config)
	collector := calibration.NewCalibrationCollector(harness)

	ctx := context.Background()
	err := collector.StartTracking(ctx)
	require.NoError(t, err)
	defer collector.StopTracking()

	// Phase 1: Collect calibration data through live tracking
	t.Log("Phase 1: Collecting calibration samples...")

	// Simulate live trading over time with different regimes
	regimes := []string{"bull", "bear", "choppy"}
	symbols := []string{"BTCUSD", "ETHUSD", "ADAUSD", "DOTUSD"}
	
	positionCount := 0
	for _, regime := range regimes {
		for i := 0; i < 30; i++ { // 30 positions per regime
			symbol := symbols[i%len(symbols)]
			score := 40.0 + float64(i)*2.0 // Scores from 40-98
			entryPrice := 1000.0 + float64(i)*10.0
			
			compositeResult := &composite.CompositeScore{
				FinalWithSocial:   score,
				MomentumCore: score * 0.35, // Approximate momentum contribution
			}

			err := collector.TrackNewPosition(symbol, score, compositeResult, entryPrice, regime)
			require.NoError(t, err, "Failed to track position %d", positionCount)
			positionCount++

			// Simulate price movements over time
			time.Sleep(1 * time.Millisecond) // Small delay to create temporal spread
			
			// Higher scores should have higher success probability
			successProb := (score - 30.0) / 70.0 // Convert score 30-100 to prob 0-1
			successProb = math.Max(0.1, math.Min(0.9, successProb)) // Clamp to 10-90%
			
			// Generate realistic price movement
			moveSize := 0.02 + (successProb * 0.08) // 2-10% potential moves
			if float64(i%100)/100.0 < successProb {
				// Successful move
				newPrice := entryPrice * (1.0 + moveSize)
				err = collector.UpdatePosition(symbol, newPrice)
			} else {
				// Unsuccessful move (small gain/loss)
				newPrice := entryPrice * (1.0 + (moveSize * 0.3)) // Smaller move
				err = collector.UpdatePosition(symbol, newPrice)
				
				// Let position timeout by waiting for target period
				time.Sleep(2 * time.Millisecond)
				err = collector.UpdatePosition(symbol, newPrice) // Trigger timeout check
			}
			require.NoError(t, err, "Failed to update position %d", positionCount)
		}
	}

	// Check data collection results
	collectorStatus := collector.GetStatus()
	t.Logf("Collected %d total positions, %d successful", 
		collectorStatus.TotalPositions, collectorStatus.SuccessfulMoves)
	
	assert.Equal(t, 90, collectorStatus.TotalPositions, "Should have tracked 90 positions")
	assert.Greater(t, collectorStatus.SuccessfulMoves, 0, "Should have some successful moves")

	// Phase 2: Calibration training and validation
	t.Log("Phase 2: Training calibration models...")

	// Refresh calibration with collected data
	err = harness.RefreshCalibration(ctx)
	require.NoError(t, err, "Calibration refresh should succeed")

	harnessStatus := harness.GetStatus()
	t.Logf("Calibration harness: %d total samples, %d calibrators", 
		harnessStatus.TotalSamples, len(harnessStatus.Calibrators))

	assert.Greater(t, harnessStatus.TotalSamples, 50, "Should have sufficient samples")
	assert.Greater(t, len(harnessStatus.Calibrators), 0, "Should have created calibrators")

	// Verify regime-specific calibrators were created
	if config.RegimeAware {
		for _, regime := range regimes {
			if calibratorInfo, exists := harnessStatus.Calibrators[regime]; exists {
				t.Logf("Regime %s: %d samples, reliability %.3f", 
					regime, calibratorInfo.SampleCount, calibratorInfo.Reliability)
				assert.Greater(t, calibratorInfo.SampleCount, 10, 
					"Regime %s should have samples", regime)
			}
		}
	}

	// Phase 3: Calibration quality validation
	t.Log("Phase 3: Validating calibration quality...")

	// Test calibration accuracy across score ranges
	testScores := []float64{45, 55, 65, 75, 85, 95}
	for _, regime := range regimes {
		if _, exists := harnessStatus.Calibrators[regime]; exists {
			var prevProb float64 = -1
			for _, score := range testScores {
				prob, err := harness.PredictProbability(score, regime)
				require.NoError(t, err, "Prediction should succeed for score %.1f in regime %s", score, regime)
				
				assert.GreaterOrEqual(t, prob, 0.0, "Probability should be non-negative")
				assert.LessOrEqual(t, prob, 1.0, "Probability should not exceed 1.0")
				
				// Check monotonicity
				if prevProb >= 0 {
					assert.GreaterOrEqual(t, prob, prevProb-1e-6, 
						"Probability should increase with score (regime %s, score %.1f)", regime, score)
				}
				prevProb = prob
				
				t.Logf("Regime %s, Score %.1f → Probability %.3f", regime, score, prob)
			}
		}
	}

	// Phase 4: Calibration stability testing  
	t.Log("Phase 4: Testing calibration stability...")

	// Multiple predictions of the same score should be identical
	testScore := 70.0
	testRegime := "bull"
	if _, exists := harnessStatus.Calibrators[testRegime]; exists {
		prob1, err1 := harness.PredictProbability(testScore, testRegime)
		prob2, err2 := harness.PredictProbability(testScore, testRegime)
		prob3, err3 := harness.PredictProbability(testScore, testRegime)
		
		require.NoError(t, err1)
		require.NoError(t, err2)
		require.NoError(t, err3)
		
		assert.Equal(t, prob1, prob2, "Repeated predictions should be identical")
		assert.Equal(t, prob2, prob3, "Repeated predictions should be identical")
	}

	// Phase 5: Export and analysis
	t.Log("Phase 5: Exporting calibration data...")

	exports := harness.ExportCalibrationData()
	assert.Greater(t, len(exports), 0, "Should have calibration data to export")

	for _, export := range exports {
		t.Logf("Exported calibration for regime %s: %d points", 
			export.Regime, len(export.Scores))
		
		assert.Greater(t, len(export.Scores), 5, "Should have multiple calibration points")
		assert.Equal(t, len(export.Scores), len(export.Probabilities), 
			"Scores and probabilities should match")
		assert.False(t, export.ExportedAt.IsZero(), "Export timestamp should be set")
		
		// Verify exported curve is monotonic
		for i := 1; i < len(export.Probabilities); i++ {
			assert.LessOrEqual(t, export.Probabilities[i-1], export.Probabilities[i]+1e-10,
				"Exported calibration curve should be monotonic")
		}
	}

	// Phase 6: Historical data export
	t.Log("Phase 6: Exporting position history...")

	history := collector.ExportPositionHistory()
	assert.Equal(t, 90, history.TotalPositions, "History should match total positions")
	assert.Greater(t, len(history.ClosedPositions), 0, "Should have closed positions in history")
	assert.Greater(t, history.AvgHoldingPeriod, time.Duration(0), "Should have positive average holding period")

	t.Logf("Position history: %.1f%% success rate, avg holding period %v",
		history.SuccessRate*100, history.AvgHoldingPeriod)
}

// TestCalibrationPerformanceUnderLoad tests calibration system performance
func TestCalibrationPerformanceUnderLoad(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance test in short mode")
	}

	config := calibration.DefaultCalibrationConfig()
	config.MinSamples = 100
	
	harness := calibration.NewCalibrationHarness(config)
	collector := calibration.NewCalibrationCollector(harness)

	ctx := context.Background()
	err := collector.StartTracking(ctx)
	require.NoError(t, err)
	defer collector.StopTracking()

	startTime := time.Now()

	// Simulate high-frequency position tracking
	numPositions := 500
	for i := 0; i < numPositions; i++ {
		symbol := []string{"BTCUSD", "ETHUSD", "ADAUSD", "DOTUSD", "SOLUSD"}[i%5]
		score := 50.0 + float64(i%50)
		regime := []string{"bull", "bear", "choppy"}[i%3]
		
		compositeResult := &composite.CompositeScore{FinalWithSocial: score}
		
		err := collector.TrackNewPosition(symbol, score, compositeResult, 1000.0+float64(i), regime)
		require.NoError(t, err, "Failed to track position %d", i)

		// Simulate rapid price updates
		if i%10 == 0 { // Update every 10th position
			newPrice := 1000.0 + float64(i) * (1.0 + 0.1) // 10% move
			err = collector.UpdatePosition(symbol, newPrice)
			require.NoError(t, err, "Failed to update position %d", i)
		}
	}

	trackingTime := time.Since(startTime)
	t.Logf("Tracked %d positions in %v (%.2fms per position)", 
		numPositions, trackingTime, float64(trackingTime.Nanoseconds())/float64(numPositions)/1e6)

	// Test calibration refresh performance
	startTime = time.Now()
	err = harness.RefreshCalibration(ctx)
	require.NoError(t, err)
	calibrationTime := time.Since(startTime)

	t.Logf("Calibration refresh took %v", calibrationTime)

	// Performance assertions
	avgTrackingTime := trackingTime / time.Duration(numPositions)
	assert.Less(t, avgTrackingTime, 5*time.Millisecond, 
		"Position tracking should be fast (<%dms per position)", 5)

	assert.Less(t, calibrationTime, 1*time.Second, 
		"Calibration refresh should complete in reasonable time")

	// Test prediction performance
	harnessStatus := harness.GetStatus()
	if len(harnessStatus.Calibrators) > 0 {
		regime := "bull"
		if _, exists := harnessStatus.Calibrators[regime]; exists {
			startTime = time.Now()
			numPredictions := 1000
			
			for i := 0; i < numPredictions; i++ {
				score := 40.0 + float64(i%60)
				_, err := harness.PredictProbability(score, regime)
				require.NoError(t, err)
			}
			
			predictionTime := time.Since(startTime)
			avgPredictionTime := predictionTime / time.Duration(numPredictions)
			
			t.Logf("Made %d predictions in %v (%.2fμs per prediction)", 
				numPredictions, predictionTime, float64(avgPredictionTime.Nanoseconds())/1000.0)
			
			assert.Less(t, avgPredictionTime, 100*time.Microsecond, 
				"Predictions should be fast (<%dμs per prediction)", 100)
		}
	}
}

// TestCalibrationMemoryUsage tests memory efficiency
func TestCalibrationMemoryUsage(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping memory test in short mode")
	}

	config := calibration.DefaultCalibrationConfig()
	config.MinSamples = 50
	
	harness := calibration.NewCalibrationHarness(config)
	_ = calibration.NewCalibrationCollector(harness) // collector unused in this test

	// Add many samples to test buffer management
	numSamples := config.MinSamples * 15 // Exceed buffer size
	
	for i := 0; i < numSamples; i++ {
		sample := calibration.CalibrationSample{
			Score:         40.0 + float64(i%60),
			Outcome:       (i%3) == 0,
			Timestamp:     time.Now().Add(-time.Duration(i) * time.Minute),
			Symbol:        []string{"BTC", "ETH", "ADA", "DOT", "SOL"}[i%5],
			Regime:        []string{"bull", "bear", "choppy"}[i%3],
			HoldingPeriod: 48 * time.Hour,
			MaxMove:       0.05,
			FinalMove:     func() float64 { if (i%3) == 0 { return 0.06 } else { return -0.02 } }(),
		}
		
		err := harness.AddSample(sample)
		require.NoError(t, err, "Failed to add sample %d", i)
	}

	status := harness.GetStatus()
	t.Logf("Added %d samples, buffer size %d", status.TotalSamples, status.BufferSize)

	// Buffer should not grow unbounded
	maxBuffer := config.MinSamples * 10 // From implementation
	assert.LessOrEqual(t, status.BufferSize, maxBuffer, 
		"Buffer should not exceed maximum size")
	assert.Equal(t, numSamples, status.TotalSamples, 
		"Should track all samples added")

	// Test cleanup functionality
	oldSampleCount := harness.ClearOldSamples(1 * time.Hour) // Clear samples older than 1h
	t.Logf("Cleaned up %d old samples", oldSampleCount)
	
	newStatus := harness.GetStatus()
	assert.Equal(t, status.BufferSize-oldSampleCount, newStatus.BufferSize,
		"Buffer size should decrease by cleanup count")
}

// TestCalibrationErrorHandling tests error conditions
func TestCalibrationErrorHandling(t *testing.T) {
	config := calibration.DefaultCalibrationConfig()
	config.MinSamples = 100 // High minimum for testing insufficient data

	harness := calibration.NewCalibrationHarness(config)
	collector := calibration.NewCalibrationCollector(harness)

	// Test insufficient data for calibration
	samples := make([]calibration.CalibrationSample, 50) // Below minimum
	for i := range samples {
		samples[i] = calibration.CalibrationSample{
			Score:         float64(40 + i),
			Outcome:       (i%2) == 0,
			Timestamp:     time.Now().Add(-time.Duration(i) * time.Minute),
			Symbol:        "BTC",
			Regime:        "bull",
			HoldingPeriod: 48 * time.Hour,
		}
		err := harness.AddSample(samples[i])
		require.NoError(t, err)
	}

	// Should fail due to insufficient samples
	err := harness.RefreshCalibration(context.Background())
	assert.Error(t, err, "Should fail with insufficient samples")
	assert.Contains(t, err.Error(), "insufficient", "Error should mention insufficient samples")

	// Test prediction without calibration
	_, err = harness.PredictProbability(75.0, "bull")
	assert.NoError(t, err, "Should return uncalibrated probability")

	// Test invalid sample data
	invalidSample := calibration.CalibrationSample{
		Score:         -10.0, // Invalid score
		Outcome:       true,
		Timestamp:     time.Now(),
		Symbol:        "BTC",
		Regime:        "bull",
		HoldingPeriod: 48 * time.Hour,
	}
	
	err = harness.AddSample(invalidSample)
	assert.Error(t, err, "Should reject invalid sample")

	// Test collector error conditions
	ctx := context.Background()
	err = collector.StartTracking(ctx)
	require.NoError(t, err)

	// Double start should fail
	err = collector.StartTracking(ctx)
	assert.Error(t, err, "Should not allow double start")

	collector.StopTracking()

	// Operations on non-existent positions should fail gracefully
	err = collector.ForceClosePosition("nonexistent", 1000.0)
	assert.Error(t, err, "Should error for non-existent position")
}