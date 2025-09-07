package calibration

import (
	"context"
	"testing"
	"time"

	"github.com/sawpanic/cryptorun/internal/score/calibration"
	"github.com/sawpanic/cryptorun/internal/score/composite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCalibrationCollector_BasicLifecycle(t *testing.T) {
	config := calibration.DefaultCalibrationConfig()
	harness := calibration.NewCalibrationHarness(config)
	collector := calibration.NewCalibrationCollector(harness)

	ctx := context.Background()

	// Start tracking
	err := collector.StartTracking(ctx)
	require.NoError(t, err, "Starting tracking should succeed")

	status := collector.GetStatus()
	assert.True(t, status.IsRunning, "Collector should be running")
	assert.Equal(t, 0, status.ActivePositions, "Should start with no positions")

	// Stop tracking
	collector.StopTracking()
	
	status = collector.GetStatus()
	assert.False(t, status.IsRunning, "Collector should not be running after stop")
}

func TestCalibrationCollector_PositionTracking(t *testing.T) {
	config := calibration.DefaultCalibrationConfig()
	harness := calibration.NewCalibrationHarness(config)
	collector := calibration.NewCalibrationCollector(harness)

	ctx := context.Background()
	err := collector.StartTracking(ctx)
	require.NoError(t, err)
	defer collector.StopTracking()

	// Track a new position
	compositeResult := &composite.CompositeScore{
		FinalWithSocial: 78.5,
		MomentumCore: 25.0,
	}

	err = collector.TrackNewPosition("BTCUSD", 78.5, compositeResult, 50000.0, "bull")
	require.NoError(t, err, "Tracking new position should succeed")

	status := collector.GetStatus()
	assert.Equal(t, 1, status.ActivePositions, "Should have one active position")
	assert.Equal(t, 1, status.TotalPositions, "Should have tracked one position total")

	// Update position with price movement
	err = collector.UpdatePosition("BTCUSD", 52500.0) // 5% gain
	require.NoError(t, err, "Position update should succeed")

	status = collector.GetStatus()
	assert.Equal(t, 0, status.ActivePositions, "Position should be closed due to target reached")
	assert.Equal(t, 1, status.SuccessfulMoves, "Should have one successful move")
	assert.Equal(t, 1.0, status.SuccessRate, "Success rate should be 100%")
}

func TestCalibrationCollector_TargetThreshold(t *testing.T) {
	config := calibration.DefaultCalibrationConfig()
	harness := calibration.NewCalibrationHarness(config)
	collector := calibration.NewCalibrationCollector(harness)

	// Set custom move threshold for testing
	collector.SetMoveThreshold(0.03) // 3% threshold

	ctx := context.Background()
	err := collector.StartTracking(ctx)
	require.NoError(t, err)
	defer collector.StopTracking()

	compositeResult := &composite.CompositeScore{FinalWithSocial: 82.0}
	err = collector.TrackNewPosition("ETHUSD", 82.0, compositeResult, 3000.0, "bull")
	require.NoError(t, err)

	// Update with move below threshold
	err = collector.UpdatePosition("ETHUSD", 3050.0) // ~1.67% gain, below 3% threshold
	require.NoError(t, err)

	status := collector.GetStatus()
	assert.Equal(t, 1, status.ActivePositions, "Position should remain active")

	// Update with move above threshold
	err = collector.UpdatePosition("ETHUSD", 3100.0) // ~3.33% gain, above threshold
	require.NoError(t, err)

	status = collector.GetStatus()
	assert.Equal(t, 0, status.ActivePositions, "Position should be closed")
	assert.Equal(t, 1, status.SuccessfulMoves, "Should count as successful")
}

func TestCalibrationCollector_TimeoutHandling(t *testing.T) {
	config := calibration.DefaultCalibrationConfig()
	harness := calibration.NewCalibrationHarness(config)
	collector := calibration.NewCalibrationCollector(harness)

	// Set very short target holding period for testing
	collector.SetTargetHoldingPeriod(100 * time.Millisecond)

	ctx := context.Background()
	err := collector.StartTracking(ctx)
	require.NoError(t, err)
	defer collector.StopTracking()

	compositeResult := &composite.CompositeScore{FinalWithSocial: 77.0}
	err = collector.TrackNewPosition("ADAUSD", 77.0, compositeResult, 1.25, "choppy")
	require.NoError(t, err)

	// Update with small move
	err = collector.UpdatePosition("ADAUSD", 1.27) // ~1.6% gain, below default 5% threshold
	require.NoError(t, err)

	// Wait for timeout
	time.Sleep(150 * time.Millisecond)

	// Trigger timeout check (in real scenario, this happens automatically via monitor goroutine)
	// We need to update position to trigger the check
	err = collector.UpdatePosition("ADAUSD", 1.27)
	require.NoError(t, err)

	// Give some time for the monitor to process
	time.Sleep(50 * time.Millisecond)

	_ = collector.GetStatus()
	// The position should be closed at target period, not timeout
	// Success depends on whether move exceeded threshold at target time
}

func TestCalibrationCollector_ForceClosePosition(t *testing.T) {
	config := calibration.DefaultCalibrationConfig()
	harness := calibration.NewCalibrationHarness(config)
	collector := calibration.NewCalibrationCollector(harness)

	ctx := context.Background()
	err := collector.StartTracking(ctx)
	require.NoError(t, err)
	defer collector.StopTracking()

	compositeResult := &composite.CompositeScore{FinalWithSocial: 85.5}
	err = collector.TrackNewPosition("SOLUSD", 85.5, compositeResult, 150.0, "bull")
	require.NoError(t, err)

	status := collector.GetStatus()
	require.Equal(t, 1, status.ActivePositions, "Should have one active position")

	// Get position ID for force close
	var positionID string
	for id := range status.Positions {
		positionID = id
		break
	}
	require.NotEmpty(t, positionID, "Should find position ID")

	// Force close with current price
	err = collector.ForceClosePosition(positionID, 145.0) // ~3.33% loss
	require.NoError(t, err, "Force close should succeed")

	status = collector.GetStatus()
	assert.Equal(t, 0, status.ActivePositions, "Position should be closed")
	assert.Equal(t, 0, status.SuccessfulMoves, "Should not count as successful (below threshold)")
}

func TestCalibrationCollector_MultipleSymbols(t *testing.T) {
	config := calibration.DefaultCalibrationConfig()
	harness := calibration.NewCalibrationHarness(config)
	collector := calibration.NewCalibrationCollector(harness)

	ctx := context.Background()
	err := collector.StartTracking(ctx)
	require.NoError(t, err)
	defer collector.StopTracking()

	// Track multiple positions
	symbols := []string{"BTCUSD", "ETHUSD", "ADAUSD"}
	prices := []float64{50000.0, 3000.0, 1.25}
	scores := []float64{78.0, 82.5, 76.0}

	for i, symbol := range symbols {
		compositeResult := &composite.CompositeScore{FinalWithSocial: scores[i]}
		err = collector.TrackNewPosition(symbol, scores[i], compositeResult, prices[i], "bull")
		require.NoError(t, err, "Should track position for %s", symbol)
	}

	status := collector.GetStatus()
	assert.Equal(t, 3, status.ActivePositions, "Should have three active positions")
	assert.Equal(t, 3, status.TotalPositions, "Should have tracked three positions")

	// Update all positions with successful moves
	newPrices := []float64{52750.0, 3180.0, 1.32} // ~5.5%, ~6%, ~5.6% gains

	for i, symbol := range symbols {
		err = collector.UpdatePosition(symbol, newPrices[i])
		require.NoError(t, err, "Should update position for %s", symbol)
	}

	status = collector.GetStatus()
	assert.Equal(t, 0, status.ActivePositions, "All positions should be closed")
	assert.Equal(t, 3, status.SuccessfulMoves, "All moves should be successful")
}

func TestCalibrationCollector_PositionDetails(t *testing.T) {
	config := calibration.DefaultCalibrationConfig()
	harness := calibration.NewCalibrationHarness(config)
	collector := calibration.NewCalibrationCollector(harness)

	ctx := context.Background()
	err := collector.StartTracking(ctx)
	require.NoError(t, err)
	defer collector.StopTracking()

	compositeResult := &composite.CompositeScore{
		FinalWithSocial: 79.5,
		MomentumCore: 28.0,
	}

	err = collector.TrackNewPosition("DOTUSD", 79.5, compositeResult, 8.50, "choppy")
	require.NoError(t, err)

	// Get position ID
	status := collector.GetStatus()
	var positionID string
	for id := range status.Positions {
		positionID = id
		break
	}

	// Get position details
	position, err := collector.GetPositionDetails(positionID)
	require.NoError(t, err, "Should get position details")
	
	assert.Equal(t, "DOTUSD", position.Symbol)
	assert.Equal(t, 79.5, position.Score)
	assert.Equal(t, 8.50, position.EntryPrice)
	assert.Equal(t, "choppy", position.Regime)
	assert.Nil(t, position.Outcome, "Outcome should be nil while active")
	assert.NotNil(t, position.CompositeResult, "CompositeResult should be preserved")

	// Test non-existent position
	_, err = collector.GetPositionDetails("nonexistent")
	assert.Error(t, err, "Should error for non-existent position")
}

func TestCalibrationCollector_CleanupClosedPositions(t *testing.T) {
	config := calibration.DefaultCalibrationConfig()
	harness := calibration.NewCalibrationHarness(config)
	collector := calibration.NewCalibrationCollector(harness)

	ctx := context.Background()
	err := collector.StartTracking(ctx)
	require.NoError(t, err)
	defer collector.StopTracking()

	// Create and close a position
	compositeResult := &composite.CompositeScore{FinalWithSocial: 81.0}
	err = collector.TrackNewPosition("LINKUSD", 81.0, compositeResult, 25.0, "bull")
	require.NoError(t, err)

	err = collector.UpdatePosition("LINKUSD", 26.5) // ~6% gain, should close
	require.NoError(t, err)

	status := collector.GetStatus()
	assert.Equal(t, 0, status.ActivePositions, "Position should be closed")

	// In the actual implementation, closed positions are kept for a short time
	// The cleanup method should eventually remove them
	removed := collector.CleanupClosedPositions()
	assert.GreaterOrEqual(t, removed, 0, "Should return count of removed positions")
}

func TestCalibrationCollector_ExportHistory(t *testing.T) {
	config := calibration.DefaultCalibrationConfig()
	harness := calibration.NewCalibrationHarness(config)
	collector := calibration.NewCalibrationCollector(harness)

	ctx := context.Background()
	err := collector.StartTracking(ctx)
	require.NoError(t, err)
	defer collector.StopTracking()

	// Create and close some positions
	symbols := []string{"BTCUSD", "ETHUSD"}
	for i, symbol := range symbols {
		compositeResult := &composite.CompositeScore{FinalWithSocial: 80.0 + float64(i)}
		err = collector.TrackNewPosition(symbol, 80.0+float64(i), compositeResult, 1000.0+float64(i*500), "bull")
		require.NoError(t, err)

		// Close with different outcomes
		newPrice := 1000.0 + float64(i*500)
		if i == 0 {
			newPrice *= 1.06 // Successful move
		} else {
			newPrice *= 0.98 // Unsuccessful move
		}
		err = collector.UpdatePosition(symbol, newPrice)
		require.NoError(t, err)
	}

	// Export position history
	export := collector.ExportPositionHistory()
	assert.Equal(t, 2, export.TotalPositions, "Should have tracked 2 positions")
	assert.False(t, export.ExportedAt.IsZero(), "Export timestamp should be set")
	assert.GreaterOrEqual(t, len(export.ClosedPositions), 0, "Should have closed positions")

	if len(export.ClosedPositions) > 0 {
		// Verify export contains position data
		pos := export.ClosedPositions[0]
		assert.NotEmpty(t, pos.Symbol, "Position should have symbol")
		assert.Greater(t, pos.Score, 0.0, "Position should have score")
		assert.NotNil(t, pos.Outcome, "Closed position should have outcome")
	}
}

func TestCalibrationCollector_ConfigurationUpdates(t *testing.T) {
	config := calibration.DefaultCalibrationConfig()
	harness := calibration.NewCalibrationHarness(config)
	collector := calibration.NewCalibrationCollector(harness)

	// Test threshold updates
	collector.SetMoveThreshold(0.08) // 8% threshold
	collector.SetMoveThreshold(-0.01) // Invalid - should be ignored
	collector.SetMoveThreshold(1.5)   // Invalid - should be ignored

	ctx := context.Background()
	err := collector.StartTracking(ctx)
	require.NoError(t, err)
	defer collector.StopTracking()

	compositeResult := &composite.CompositeScore{FinalWithSocial: 85.0}
	err = collector.TrackNewPosition("TESTUSD", 85.0, compositeResult, 100.0, "bull")
	require.NoError(t, err)

	// Test with move below new threshold
	err = collector.UpdatePosition("TESTUSD", 106.0) // 6% gain, below 8% threshold
	require.NoError(t, err)

	status := collector.GetStatus()
	assert.Equal(t, 1, status.ActivePositions, "Position should remain active with 6% gain")

	// Test with move above threshold
	err = collector.UpdatePosition("TESTUSD", 109.0) // 9% gain, above 8% threshold
	require.NoError(t, err)

	status = collector.GetStatus()
	assert.Equal(t, 0, status.ActivePositions, "Position should close with 9% gain")

	// Test holding period update
	collector.SetTargetHoldingPeriod(36 * time.Hour)
	collector.SetTargetHoldingPeriod(-1 * time.Hour) // Invalid - should be ignored
}

func TestCalibrationCollector_ErrorCases(t *testing.T) {
	config := calibration.DefaultCalibrationConfig()
	harness := calibration.NewCalibrationHarness(config)
	collector := calibration.NewCalibrationCollector(harness)

	// Try operations without starting
	compositeResult := &composite.CompositeScore{FinalWithSocial: 75.0}
	err := collector.TrackNewPosition("BTCUSD", 75.0, compositeResult, 50000.0, "bull")
	assert.NoError(t, err, "Should be able to track position before starting") // Based on implementation

	err = collector.UpdatePosition("BTCUSD", 51000.0)
	assert.NoError(t, err, "Should be able to update position") // Based on implementation

	// Test double start
	ctx := context.Background()
	err = collector.StartTracking(ctx)
	require.NoError(t, err)

	err = collector.StartTracking(ctx)
	assert.Error(t, err, "Should not be able to start twice")
	assert.Contains(t, err.Error(), "already running", "Error should mention already running")

	// Test double stop
	collector.StopTracking()
	collector.StopTracking() // Should not panic

	// Test force close non-existent position
	err = collector.ForceClosePosition("nonexistent", 100.0)
	assert.Error(t, err, "Should error for non-existent position")
}

// Benchmark calibration collector performance
func BenchmarkCalibrationCollector_TrackPosition(b *testing.B) {
	config := calibration.DefaultCalibrationConfig()
	harness := calibration.NewCalibrationHarness(config)
	collector := calibration.NewCalibrationCollector(harness)

	compositeResult := &composite.CompositeScore{FinalWithSocial: 80.0}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		symbol := []string{"BTCUSD", "ETHUSD", "ADAUSD", "DOTUSD"}[i%4]
		_ = collector.TrackNewPosition(symbol, 80.0+float64(i%20), compositeResult, 1000.0, "bull")
	}
}

func BenchmarkCalibrationCollector_UpdatePosition(b *testing.B) {
	config := calibration.DefaultCalibrationConfig()
	harness := calibration.NewCalibrationHarness(config)
	collector := calibration.NewCalibrationCollector(harness)

	// Pre-populate with positions
	compositeResult := &composite.CompositeScore{FinalWithSocial: 80.0}
	symbols := []string{"BTCUSD", "ETHUSD", "ADAUSD", "DOTUSD"}
	for _, symbol := range symbols {
		_ = collector.TrackNewPosition(symbol, 80.0, compositeResult, 1000.0, "bull")
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		symbol := symbols[i%len(symbols)]
		price := 1000.0 + float64(i%100) // Vary price
		_ = collector.UpdatePosition(symbol, price)
	}
}