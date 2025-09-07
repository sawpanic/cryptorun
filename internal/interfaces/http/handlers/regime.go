package handlers

import (
	"net/http"
	"time"

	httpContracts "github.com/sawpanic/cryptorun/internal/http"
)

// Regime handles GET /regime endpoint
func (h *Handlers) Regime(w http.ResponseWriter, r *http.Request) {
	// Mock regime data - in real implementation, this would query the regime detector
	response := httpContracts.RegimeResponse{
		ActiveRegime: "trending_bull",
		Confidence:   0.85,
		Weights: map[string]float64{
			"momentum_core":      0.40,
			"technical_residual": 0.30,
			"volume_residual":    0.20,
			"quality_residual":   0.08,
			"social_residual":    0.02,
			"weekly_7d_carry":    0.10, // Special for trending regime
		},
		LastDetection: time.Now().Add(-15 * time.Minute).UTC(),
		NextUpdate:    time.Now().Add(45 * time.Minute).UTC(), // 4h cadence
		Signals: map[string]float64{
			"realized_vol_7d": 0.23, // Below threshold, indicating calm
			"pct_above_20ma":  0.67, // 67% above 20MA (bullish breadth)
			"breadth_thrust":  0.78, // Strong breadth momentum (ADX proxy)
		},
		RegimeHistory: []httpContracts.RegimeChange{
			{
				FromRegime: "choppy",
				ToRegime:   "trending_bull",
				Timestamp:  time.Now().Add(-6 * time.Hour).UTC(),
				Confidence: 0.85,
			},
			{
				FromRegime: "high_volatility",
				ToRegime:   "choppy",
				Timestamp:  time.Now().Add(-18 * time.Hour).UTC(),
				Confidence: 0.72,
			},
			{
				FromRegime: "trending_bull",
				ToRegime:   "high_volatility",
				Timestamp:  time.Now().Add(-32 * time.Hour).UTC(),
				Confidence: 0.91,
			},
		},
	}

	h.writeJSON(w, http.StatusOK, response)
}
