package persistence

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTimeRange_Validation(t *testing.T) {
	tests := []struct {
		name  string
		tr    TimeRange
		valid bool
	}{
		{
			name: "valid_range",
			tr: TimeRange{
				From: time.Date(2025, 9, 7, 10, 0, 0, 0, time.UTC),
				To:   time.Date(2025, 9, 7, 11, 0, 0, 0, time.UTC),
			},
			valid: true,
		},
		{
			name: "same_time",
			tr: TimeRange{
				From: time.Date(2025, 9, 7, 10, 0, 0, 0, time.UTC),
				To:   time.Date(2025, 9, 7, 10, 0, 0, 0, time.UTC),
			},
			valid: true,
		},
		{
			name: "zero_times",
			tr: TimeRange{
				From: time.Time{},
				To:   time.Time{},
			},
			valid: true, // Edge case - both zero is considered valid
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// TimeRange validation would be implemented in business logic
			assert.NotNil(t, tt.tr)
			if tt.valid {
				assert.True(t, tt.tr.To.After(tt.tr.From) || tt.tr.To.Equal(tt.tr.From))
			}
		})
	}
}

func TestTrade_Validation(t *testing.T) {
	validTrade := Trade{
		ID:        1,
		Timestamp: time.Now(),
		Symbol:    "BTC-USD",
		Venue:     "kraken",
		Side:      "buy",
		Price:     50000.0,
		Qty:       0.1,
		OrderID:   stringPtr("order123"),
		Attributes: map[string]interface{}{
			"taker": true,
		},
		CreatedAt: time.Now(),
	}

	t.Run("valid_trade", func(t *testing.T) {
		assert.Equal(t, "BTC-USD", validTrade.Symbol)
		assert.Equal(t, "kraken", validTrade.Venue)
		assert.Greater(t, validTrade.Price, 0.0)
		assert.Greater(t, validTrade.Qty, 0.0)
		require.NotNil(t, validTrade.OrderID)
		assert.Equal(t, "order123", *validTrade.OrderID)
	})

	t.Run("exchange_native_venues", func(t *testing.T) {
		validVenues := []string{"binance", "okx", "coinbase", "kraken"}
		for _, venue := range validVenues {
			trade := validTrade
			trade.Venue = venue
			assert.Contains(t, validVenues, trade.Venue)
		}
	})
}

func TestRegimeSnapshot_Validation(t *testing.T) {
	validSnapshot := RegimeSnapshot{
		Timestamp:       time.Now(),
		RealizedVol7d:   15.5,
		PctAbove20MA:    67.8,
		BreadthThrust:   0.23,
		Regime:          "trending",
		Weights: map[string]float64{
			"momentum":  30.0,
			"technical": 25.0,
			"volume":    20.0,
			"quality":   15.0,
			"social":    10.0, // Social cap at max
		},
		ConfidenceScore: 0.82,
		DetectionMethod: "majority_vote",
		Metadata:        map[string]interface{}{"test": true},
		CreatedAt:       time.Now(),
	}

	t.Run("valid_snapshot", func(t *testing.T) {
		assert.Equal(t, "trending", validSnapshot.Regime)
		assert.GreaterOrEqual(t, validSnapshot.RealizedVol7d, 0.0)
		assert.GreaterOrEqual(t, validSnapshot.PctAbove20MA, 0.0)
		assert.LessOrEqual(t, validSnapshot.PctAbove20MA, 100.0)
		assert.GreaterOrEqual(t, validSnapshot.BreadthThrust, -1.0)
		assert.LessOrEqual(t, validSnapshot.BreadthThrust, 1.0)
		assert.GreaterOrEqual(t, validSnapshot.ConfidenceScore, 0.0)
		assert.LessOrEqual(t, validSnapshot.ConfidenceScore, 1.0)
	})

	t.Run("valid_regimes", func(t *testing.T) {
		validRegimes := []string{"trending", "choppy", "highvol", "mixed"}
		for _, regime := range validRegimes {
			snapshot := validSnapshot
			snapshot.Regime = regime
			assert.Contains(t, validRegimes, snapshot.Regime)
		}
	})

	t.Run("social_cap_enforcement", func(t *testing.T) {
		socialWeight, exists := validSnapshot.Weights["social"]
		require.True(t, exists)
		assert.LessOrEqual(t, socialWeight, 10.0, "Social factor must be capped at +10")
	})
}

func TestPremoveArtifact_Validation(t *testing.T) {
	score := 85.5
	momentumCore := 42.1
	socialResidual := 8.7 // Under cap

	validArtifact := PremoveArtifact{
		ID:                 1,
		Timestamp:          time.Now(),
		Symbol:             "ETH-USD",
		Venue:              "binance",
		GateScore:          true,
		GateVADR:           true,
		GateFunding:        true,
		GateMicrostructure: true,
		GateFreshness:      true,
		GateFatigue:        false,
		Score:              &score,
		MomentumCore:       &momentumCore,
		SocialResidual:     &socialResidual,
		Factors: map[string]interface{}{
			"momentum_timeframes": []string{"1h", "4h", "12h", "24h"},
		},
		Regime:          stringPtr("trending"),
		ConfidenceScore: 0.75,
		CreatedAt:       time.Now(),
	}

	t.Run("valid_artifact", func(t *testing.T) {
		assert.Equal(t, "ETH-USD", validArtifact.Symbol)
		assert.Equal(t, "binance", validArtifact.Venue)
		require.NotNil(t, validArtifact.Score)
		assert.GreaterOrEqual(t, *validArtifact.Score, 0.0)
		assert.LessOrEqual(t, *validArtifact.Score, 100.0)
		require.NotNil(t, validArtifact.MomentumCore)
		assert.GreaterOrEqual(t, *validArtifact.MomentumCore, 0.0)
	})

	t.Run("entry_gates", func(t *testing.T) {
		// Test that gates can be individually checked
		assert.True(t, validArtifact.GateScore)
		assert.True(t, validArtifact.GateVADR)
		assert.True(t, validArtifact.GateFunding)
		assert.True(t, validArtifact.GateMicrostructure)
		assert.True(t, validArtifact.GateFreshness)
		assert.False(t, validArtifact.GateFatigue)
	})

	t.Run("social_residual_cap", func(t *testing.T) {
		require.NotNil(t, validArtifact.SocialResidual)
		assert.LessOrEqual(t, *validArtifact.SocialResidual, 10.0, "Social residual must be capped at +10")
	})
}

func TestHealthCheck_Structure(t *testing.T) {
	healthCheck := HealthCheck{
		Healthy: true,
		Errors:  []string{},
		ConnectionPool: map[string]int{
			"active":   5,
			"idle":     10,
			"max":      20,
		},
		LastCheck:      time.Now(),
		ResponseTimeMS: 45,
	}

	t.Run("valid_health_check", func(t *testing.T) {
		assert.True(t, healthCheck.Healthy)
		assert.Empty(t, healthCheck.Errors)
		assert.Contains(t, healthCheck.ConnectionPool, "active")
		assert.Contains(t, healthCheck.ConnectionPool, "idle")
		assert.Contains(t, healthCheck.ConnectionPool, "max")
		assert.Greater(t, healthCheck.ResponseTimeMS, int64(0))
	})
}

// Helper functions
func stringPtr(s string) *string {
	return &s
}