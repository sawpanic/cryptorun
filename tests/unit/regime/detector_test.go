package regime

import (
	"context"
	"testing"
	"time"

	"github.com/sawpanic/cryptorun/internal/regime"
)

// MockDetectorInputs provides test data for regime detection
type MockDetectorInputs struct {
	realizedVol   float64
	breadth       float64
	breadthThrust float64
	timestamp     time.Time
}

func (m *MockDetectorInputs) GetRealizedVolatility7d(ctx context.Context) (float64, error) {
	return m.realizedVol, nil
}

func (m *MockDetectorInputs) GetBreadthAbove20MA(ctx context.Context) (float64, error) {
	return m.breadth, nil
}

func (m *MockDetectorInputs) GetBreadthThrustADXProxy(ctx context.Context) (float64, error) {
	return m.breadthThrust, nil
}

func (m *MockDetectorInputs) GetTimestamp(ctx context.Context) (time.Time, error) {
	return m.timestamp, nil
}

func TestDetector_VotingLogic(t *testing.T) {
	baseTime := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)

	testCases := []struct {
		name           string
		realizedVol    float64 // Threshold: 0.25
		breadth        float64 // Threshold: 0.60
		breadthThrust  float64 // Threshold: 0.70
		expectedRegime regime.Regime
	}{
		{
			name:           "trending_bull_clear",
			realizedVol:    0.15,                // Low vol
			breadth:        0.75,                // High breadth -> trending
			breadthThrust:  0.80,                // High thrust -> trending
			expectedRegime: regime.TrendingBull, // 2/3 trending votes
		},
		{
			name:           "high_vol_clear",
			realizedVol:    0.35,           // High vol
			breadth:        0.40,           // Low breadth -> choppy
			breadthThrust:  0.50,           // Low thrust -> choppy
			expectedRegime: regime.HighVol, // High vol vote wins
		},
		{
			name:           "choppy_default",
			realizedVol:    0.20,          // Low vol
			breadth:        0.50,          // Low breadth -> choppy
			breadthThrust:  0.60,          // Low thrust -> choppy
			expectedRegime: regime.Choppy, // 2/3 choppy votes
		},
		{
			name:           "boundary_conditions_vol",
			realizedVol:    0.25,                // Exactly at threshold
			breadth:        0.60,                // Exactly at threshold
			breadthThrust:  0.70,                // Exactly at threshold
			expectedRegime: regime.TrendingBull, // At threshold = trending
		},
		{
			name:           "mixed_signals",
			realizedVol:    0.30,           // High vol
			breadth:        0.70,           // High breadth -> trending
			breadthThrust:  0.40,           // Low thrust -> choppy
			expectedRegime: regime.HighVol, // High vol overrides
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			inputs := &MockDetectorInputs{
				realizedVol:   tc.realizedVol,
				breadth:       tc.breadth,
				breadthThrust: tc.breadthThrust,
				timestamp:     baseTime,
			}

			detector := regime.NewDetector(inputs)
			ctx := context.Background()

			result, err := detector.DetectRegime(ctx)
			if err != nil {
				t.Fatalf("DetectRegime failed: %v", err)
			}

			if result.Regime != tc.expectedRegime {
				t.Errorf("Expected regime %s, got %s", tc.expectedRegime, result.Regime)
			}

			// Validate signals are captured
			if result.Signals["realized_vol_7d"] != tc.realizedVol {
				t.Errorf("Realized vol signal mismatch: expected %.3f, got %.3f",
					tc.realizedVol, result.Signals["realized_vol_7d"])
			}

			// Validate confidence is reasonable
			if result.Confidence < 0.33 || result.Confidence > 1.0 {
				t.Errorf("Confidence out of range: %.3f", result.Confidence)
			}

			// Validate voting breakdown exists
			if len(result.VotingBreakdown) != 3 {
				t.Errorf("Expected 3 votes, got %d", len(result.VotingBreakdown))
			}
		})
	}
}

func TestDetector_StabilityDetection(t *testing.T) {
	baseTime := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)

	inputs := &MockDetectorInputs{
		realizedVol:   0.15,
		breadth:       0.75,
		breadthThrust: 0.80,
		timestamp:     baseTime,
	}

	detector := regime.NewDetector(inputs)
	ctx := context.Background()

	// First detection - should be stable (no history)
	result1, err := detector.DetectRegime(ctx)
	if err != nil {
		t.Fatalf("First detection failed: %v", err)
	}

	if !result1.IsStable {
		t.Error("First detection should be stable (no history)")
	}

	if result1.ChangesSinceStart != 0 {
		t.Errorf("Expected 0 changes, got %d", result1.ChangesSinceStart)
	}

	// Change regime and detect again
	inputs.realizedVol = 0.35 // Switch to high vol
	inputs.breadth = 0.40
	inputs.timestamp = baseTime.Add(4 * time.Hour)

	result2, err := detector.DetectRegime(ctx)
	if err != nil {
		t.Fatalf("Second detection failed: %v", err)
	}

	if result2.IsStable {
		t.Error("Second detection should not be stable (recent change)")
	}

	if result2.ChangesSinceStart != 1 {
		t.Errorf("Expected 1 change, got %d", result2.ChangesSinceStart)
	}

	// Verify regime change was recorded
	history := detector.GetDetectionHistory()
	if len(history) != 1 {
		t.Fatalf("Expected 1 regime change, got %d", len(history))
	}

	change := history[0]
	if change.FromRegime != regime.TrendingBull || change.ToRegime != regime.HighVol {
		t.Errorf("Regime change mismatch: %s -> %s", change.FromRegime, change.ToRegime)
	}
}

func TestDetector_UpdateInterval(t *testing.T) {
	baseTime := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)

	inputs := &MockDetectorInputs{
		realizedVol:   0.15,
		breadth:       0.75,
		breadthThrust: 0.80,
		timestamp:     baseTime,
	}

	detector := regime.NewDetector(inputs)
	ctx := context.Background()

	// First detection
	_, err := detector.DetectRegime(ctx)
	if err != nil {
		t.Fatalf("First detection failed: %v", err)
	}

	// Immediate second detection should return cached result
	shouldUpdate, err := detector.ShouldUpdate(ctx)
	if err != nil {
		t.Fatalf("ShouldUpdate failed: %v", err)
	}

	if shouldUpdate {
		t.Error("Should not update immediately after first detection")
	}

	// After 4 hours, should update
	inputs.timestamp = baseTime.Add(4 * time.Hour)

	shouldUpdate, err = detector.ShouldUpdate(ctx)
	if err != nil {
		t.Fatalf("ShouldUpdate failed: %v", err)
	}

	if !shouldUpdate {
		t.Error("Should update after 4 hours")
	}
}

func TestDetector_BoundaryConditions(t *testing.T) {
	baseTime := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)

	testCases := []struct {
		name        string
		realizedVol float64
		expected    string // Expected vote for realized vol
	}{
		{"exactly_at_threshold", 0.25, "high_vol"}, // At threshold should trigger
		{"just_below", 0.249, "low_vol"},
		{"just_above", 0.251, "high_vol"},
		{"zero", 0.0, "low_vol"},
		{"very_high", 1.0, "high_vol"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			inputs := &MockDetectorInputs{
				realizedVol:   tc.realizedVol,
				breadth:       0.50, // Below threshold
				breadthThrust: 0.60, // Below threshold
				timestamp:     baseTime,
			}

			detector := regime.NewDetector(inputs)
			ctx := context.Background()

			result, err := detector.DetectRegime(ctx)
			if err != nil {
				t.Fatalf("DetectRegime failed: %v", err)
			}

			realizedVolVote := result.VotingBreakdown["realized_vol"]
			if realizedVolVote != tc.expected {
				t.Errorf("Expected realized vol vote %s, got %s", tc.expected, realizedVolVote)
			}
		})
	}
}

func TestDetector_ErrorHandling(t *testing.T) {
	// Test with nil inputs (should cause errors)
	detector := regime.NewDetector(nil)
	ctx := context.Background()

	_, err := detector.DetectRegime(ctx)
	if err == nil {
		t.Error("Expected error with nil inputs")
	}
}

func TestWeightManager_Validation(t *testing.T) {
	wm := regime.NewWeightManager()

	// Test all presets validate
	for regimeType := range wm.GetAllPresets() {
		err := wm.ValidateWeights(regimeType)
		if err != nil {
			t.Errorf("Weight validation failed for regime %s: %v", regimeType, err)
		}
	}

	// Test getting weights for each regime
	trendingPreset, err := wm.GetWeightsForRegime(regime.TrendingBull)
	if err != nil {
		t.Fatalf("Failed to get trending bull weights: %v", err)
	}

	// Trending should have weekly carry
	if trendingPreset.Weights["weekly_7d_carry"] == 0.0 {
		t.Error("Trending bull should have non-zero weekly carry")
	}

	choppyPreset, err := wm.GetWeightsForRegime(regime.Choppy)
	if err != nil {
		t.Fatalf("Failed to get choppy weights: %v", err)
	}

	// Choppy should have no weekly carry
	if choppyPreset.Weights["weekly_7d_carry"] != 0.0 {
		t.Error("Choppy should have zero weekly carry")
	}

	highVolPreset, err := wm.GetWeightsForRegime(regime.HighVol)
	if err != nil {
		t.Fatalf("Failed to get high vol weights: %v", err)
	}

	// High vol should have tightened gates
	if !highVolPreset.MovementGate.TightenedThresholds {
		t.Error("High vol should have tightened thresholds")
	}

	if highVolPreset.MovementGate.MinMovementPercent <= 5.0 {
		t.Error("High vol should have higher movement threshold than standard")
	}
}
