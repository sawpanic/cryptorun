package unit

import (
	"context"
	"testing"
	"time"

	"github.com/sawpanic/cryptorun/internal/regime"
)

func TestRegimeDetectorThresholds(t *testing.T) {
	tests := []struct {
		name             string
		realizedVol      float64
		breadthAbove20MA float64
		breadthThrust    float64
		expectedRegime   regime.Regime
		minConfidence    float64
	}{
		{
			name:             "trending_bull_clear_signals",
			realizedVol:      0.18, // Low vol (below 25% threshold)
			breadthAbove20MA: 0.75, // Strong breadth (above 60%)
			breadthThrust:    0.80, // Strong thrust (above 70%)
			expectedRegime:   regime.TrendingBull,
			minConfidence:    0.66, // Should get at least 2/3 votes
		},
		{
			name:             "choppy_mixed_signals",
			realizedVol:      0.22, // Moderate vol (below threshold)
			breadthAbove20MA: 0.45, // Weak breadth (below 60%)
			breadthThrust:    0.55, // Weak thrust (below 70%)
			expectedRegime:   regime.Choppy,
			minConfidence:    0.66, // Should get at least 2/3 votes
		},
		{
			name:             "high_vol_clear",
			realizedVol:      0.35,          // High vol (above 25% threshold)
			breadthAbove20MA: 0.40,          // Weak breadth due to volatility
			breadthThrust:    0.65,          // Moderate thrust
			expectedRegime:   regime.Choppy, // High vol gets 1 vote, others vote choppy (2/3 wins)
			minConfidence:    0.66,          // Should get 2/3 votes for choppy
		},
		{
			name:             "boundary_case_vol_threshold",
			realizedVol:      0.25,          // Exactly at threshold
			breadthAbove20MA: 0.50,          // Neutral breadth
			breadthThrust:    0.60,          // Below thrust threshold
			expectedRegime:   regime.Choppy, // Should lean choppy
			minConfidence:    0.33,
		},
		{
			name:             "boundary_case_breadth_threshold",
			realizedVol:      0.20,                // Low vol (votes "low_vol", ignored in majority)
			breadthAbove20MA: 0.65,                // Above breadth threshold (votes "trending_bull")
			breadthThrust:    0.75,                // Above thrust threshold (votes "trending_bull")
			expectedRegime:   regime.TrendingBull, // 2/3 trending votes win
			minConfidence:    0.66,                // Should get 2/3 votes for trending
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock inputs with specific values
			inputs := &MockInputs{
				realizedVol:   tt.realizedVol,
				breadth:       tt.breadthAbove20MA,
				breadthThrust: tt.breadthThrust,
			}

			detector := regime.NewDetector(inputs)

			result, err := detector.DetectRegime(context.Background())
			if err != nil {
				t.Fatalf("DetectRegime failed: %v", err)
			}

			if result.Regime != tt.expectedRegime {
				t.Errorf("Expected regime %s, got %s",
					tt.expectedRegime.String(), result.Regime.String())
			}

			if result.Confidence < tt.minConfidence {
				t.Errorf("Expected confidence >= %.2f, got %.2f",
					tt.minConfidence, result.Confidence)
			}

			// Verify signals are populated
			if len(result.Signals) == 0 {
				t.Error("Expected signals to be populated")
			}

			// Verify voting breakdown
			if len(result.VotingBreakdown) == 0 {
				t.Error("Expected voting breakdown to be populated")
			}
		})
	}
}

func TestRegimeDetectorCaching(t *testing.T) {
	inputs := &MockInputs{
		realizedVol:   0.20,
		breadth:       0.70,
		breadthThrust: 0.75,
		currentTime:   time.Now(),
	}

	detector := regime.NewDetector(inputs)

	// First detection
	result1, err := detector.DetectRegime(context.Background())
	if err != nil {
		t.Fatalf("First DetectRegime failed: %v", err)
	}

	// Immediate second detection (should use cache)
	result2, err := detector.DetectRegime(context.Background())
	if err != nil {
		t.Fatalf("Second DetectRegime failed: %v", err)
	}

	// Results should be identical (same timestamp indicates caching)
	if result1.LastUpdate != result2.LastUpdate {
		t.Error("Expected cached result with same timestamp")
	}

	if result1.Regime != result2.Regime {
		t.Error("Cached result should have same regime")
	}

	// Advance time by 4+ hours to trigger update
	inputs.currentTime = inputs.currentTime.Add(5 * time.Hour)

	result3, err := detector.DetectRegime(context.Background())
	if err != nil {
		t.Fatalf("Third DetectRegime failed: %v", err)
	}

	// Should have new timestamp
	if result3.LastUpdate == result1.LastUpdate {
		t.Error("Expected new detection after time advancement")
	}
}

func TestWeightBlendSelection(t *testing.T) {
	weightManager := regime.NewWeightManager()

	tests := []struct {
		regime          regime.Regime
		expectedWeights map[string]float64
		weeklyCarry     bool
		description     string
	}{
		{
			regime: regime.TrendingBull,
			expectedWeights: map[string]float64{
				"momentum_1h":     0.25,
				"weekly_7d_carry": 0.10, // Only in trending
			},
			weeklyCarry: true,
			description: "trending bull should include weekly carry",
		},
		{
			regime: regime.Choppy,
			expectedWeights: map[string]float64{
				"momentum_1h":     0.20, // Reduced in chop
				"weekly_7d_carry": 0.00, // No carry in chop
				"volume_surge":    0.12, // Higher volume emphasis
			},
			weeklyCarry: false,
			description: "choppy should exclude weekly carry",
		},
		{
			regime: regime.HighVol,
			expectedWeights: map[string]float64{
				"momentum_1h":     0.15, // Reduced short-term in volatility
				"momentum_24h":    0.15, // Favor longer timeframes
				"weekly_7d_carry": 0.00, // No carry in volatility
				"quality_score":   0.12, // Higher quality emphasis
			},
			weeklyCarry: false,
			description: "high vol should emphasize quality and longer timeframes",
		},
	}

	for _, tt := range tests {
		t.Run(tt.regime.String(), func(t *testing.T) {
			preset, err := weightManager.GetWeightsForRegime(tt.regime)
			if err != nil {
				t.Fatalf("Failed to get weights for regime %s: %v", tt.regime.String(), err)
			}

			// Check specific expected weights
			for factor, expectedWeight := range tt.expectedWeights {
				actualWeight, exists := preset.Weights[factor]
				if !exists {
					t.Errorf("Expected factor %s not found in %s regime", factor, tt.regime.String())
					continue
				}
				if actualWeight != expectedWeight {
					t.Errorf("Factor %s: expected weight %.2f, got %.2f",
						factor, expectedWeight, actualWeight)
				}
			}

			// Check weekly carry presence
			weeklyCarryWeight := preset.Weights["weekly_7d_carry"]
			hasWeeklyCarry := weeklyCarryWeight > 0
			if hasWeeklyCarry != tt.weeklyCarry {
				t.Errorf("Weekly carry expectation mismatch: expected %v, got weight %.2f",
					tt.weeklyCarry, weeklyCarryWeight)
			}

			// Validate weight sum is close to 1.0
			err = weightManager.ValidateWeights(tt.regime)
			if err != nil {
				t.Errorf("Weight validation failed for %s: %v", tt.regime.String(), err)
			}

			// Check metadata
			if preset.Description == "" {
				t.Error("Expected non-empty description")
			}
		})
	}
}

func TestRegimeStability(t *testing.T) {
	inputs := &MockInputs{
		realizedVol:   0.70, // Start trending
		breadth:       0.70,
		breadthThrust: 0.75,
		currentTime:   time.Now(),
	}

	detector := regime.NewDetector(inputs)

	// Initial detection
	result1, err := detector.DetectRegime(context.Background())
	if err != nil {
		t.Fatalf("Initial detection failed: %v", err)
	}

	initialRegime := result1.Regime

	// Simulate regime changes over time
	changes := 0
	for i := 0; i < 5; i++ {
		// Advance time by 4 hours
		inputs.currentTime = inputs.currentTime.Add(4 * time.Hour)

		// Slightly modify inputs to simulate market evolution
		inputs.realizedVol += (float64(i) - 2) * 0.02 // Some variation
		inputs.breadth += (float64(i) - 2) * 0.05

		result, err := detector.DetectRegime(context.Background())
		if err != nil {
			t.Fatalf("Detection %d failed: %v", i+1, err)
		}

		if result.Regime != initialRegime {
			changes++
			initialRegime = result.Regime
		}

		// Check stability flag
		history := detector.GetDetectionHistory()
		if len(history) > 2 {
			// After several detections, should have some stability assessment
			t.Logf("Detection %d: regime=%s, stable=%v, changes=%d",
				i+1, result.Regime.String(), result.IsStable, len(history))
		}
	}

	// Verify we tracked regime changes
	history := detector.GetDetectionHistory()
	if len(history) != changes {
		t.Errorf("Expected %d regime changes in history, got %d", changes, len(history))
	}

	// Each change should have proper metadata
	for _, change := range history {
		if change.FromRegime == change.ToRegime {
			t.Error("Invalid regime change: from and to are the same")
		}
		if change.Confidence <= 0 || change.Confidence > 1 {
			t.Errorf("Invalid confidence in regime change: %.2f", change.Confidence)
		}
		if change.TriggerHour < 0 || change.TriggerHour > 23 {
			t.Errorf("Invalid trigger hour: %d", change.TriggerHour)
		}
	}
}

// MockInputs implements DetectorInputs for testing
type MockInputs struct {
	realizedVol   float64
	breadth       float64
	breadthThrust float64
	currentTime   time.Time
}

func (m *MockInputs) GetRealizedVolatility7d(ctx context.Context) (float64, error) {
	return m.realizedVol, nil
}

func (m *MockInputs) GetBreadthAbove20MA(ctx context.Context) (float64, error) {
	return m.breadth, nil
}

func (m *MockInputs) GetBreadthThrustADXProxy(ctx context.Context) (float64, error) {
	return m.breadthThrust, nil
}

func (m *MockInputs) GetTimestamp(ctx context.Context) (time.Time, error) {
	if m.currentTime.IsZero() {
		return time.Now(), nil
	}
	return m.currentTime, nil
}
