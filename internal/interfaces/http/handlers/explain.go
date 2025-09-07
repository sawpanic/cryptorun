package handlers

import (
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/mux"

	httpContracts "github.com/sawpanic/cryptorun/internal/http"
)

// Explain handles GET /explain/{symbol} endpoint
func (h *Handlers) Explain(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	symbol := strings.ToUpper(vars["symbol"])

	// Validate symbol format (XXX-USD)
	if !isValidSymbol(symbol) {
		h.writeError(w, r, http.StatusBadRequest, "invalid_symbol",
			"Symbol must be in format XXX-USD (e.g., BTC-USD)")
		return
	}

	// Mock explanation data - in real implementation, this would:
	// 1. Check artifacts directory first
	// 2. Fall back to live calculation
	// 3. Use cache when available
	explanation := generateMockExplanation(symbol)

	response := httpContracts.ExplainResponse{
		Symbol:      symbol,
		Explanation: explanation,
		Source:      "artifacts", // mock: could be "live" or "cache"
		Timestamp:   time.Now().UTC(),
		CacheHit:    false,
	}

	h.writeJSON(w, http.StatusOK, response)
}

// isValidSymbol validates symbol format (XXX-USD)
func isValidSymbol(symbol string) bool {
	if len(symbol) < 6 {
		return false
	}
	return strings.HasSuffix(symbol, "-USD") || strings.HasSuffix(symbol, "USD")
}

// generateMockExplanation creates mock explainability data
func generateMockExplanation(symbol string) map[string]interface{} {
	return map[string]interface{}{
		"composite_score": 87.5,
		"rank":            3,
		"factors": map[string]interface{}{
			"momentum_core": map[string]interface{}{
				"value":        35.0,
				"weight":       0.40,
				"contribution": 14.0,
				"timeframes": map[string]interface{}{
					"1h":  8.2,
					"4h":  12.1,
					"12h": 9.8,
					"24h": 4.9,
				},
				"protected":   true,
				"explanation": "Strong momentum across multiple timeframes, ATR-normalized",
			},
			"technical_residual": map[string]interface{}{
				"value":                  11.25,
				"weight":                 0.30,
				"contribution":           3.375,
				"orthogonalized_against": []string{"momentum_core"},
				"explanation":            "Technical indicators residual after momentum orthogonalization",
			},
			"volume_residual": map[string]interface{}{
				"value":                  7.5,
				"weight":                 0.20,
				"contribution":           1.5,
				"orthogonalized_against": []string{"momentum_core", "technical_residual"},
				"explanation":            "Volume factors residual after prior orthogonalization",
			},
			"quality_residual": map[string]interface{}{
				"value":                  3.75,
				"weight":                 0.08,
				"contribution":           0.3,
				"orthogonalized_against": []string{"momentum_core", "technical_residual", "volume_residual"},
				"explanation":            "Quality metrics residual after prior orthogonalization",
			},
			"social_residual": map[string]interface{}{
				"value":                  8.5,
				"weight":                 0.02,
				"contribution":           0.17,
				"capped_at":              10.0,
				"orthogonalized_against": []string{"momentum_core", "technical_residual", "volume_residual", "quality_residual"},
				"explanation":            "Social/brand factor residual, capped at ±10 points",
			},
		},
		"gates": map[string]interface{}{
			"entry_gates": map[string]interface{}{
				"score_gate": map[string]interface{}{
					"passed":      true,
					"threshold":   75.0,
					"actual":      87.5,
					"explanation": "Composite score meets minimum threshold",
				},
				"vadr_gate": map[string]interface{}{
					"passed":      true,
					"threshold":   1.8,
					"actual":      2.3,
					"explanation": "Volume-Adjusted Daily Range indicates sufficient liquidity",
				},
				"funding_gate": map[string]interface{}{
					"passed":      true,
					"threshold":   2.0,
					"actual":      2.7,
					"explanation": "Cross-venue funding divergence indicates opportunity",
				},
			},
			"guard_gates": map[string]interface{}{
				"freshness_gate": map[string]interface{}{
					"passed":       true,
					"max_bars":     2,
					"actual_bars":  1,
					"atr_multiple": 1.2,
					"explanation":  "Signal is fresh and within ATR bounds",
				},
				"fatigue_gate": map[string]interface{}{
					"passed":      true,
					"explanation": "No fatigue detected (24h move < 12% OR RSI < 70)",
				},
				"late_fill_gate": map[string]interface{}{
					"passed":      true,
					"max_delay":   "30s",
					"explanation": "No late fill risk detected",
				},
			},
		},
		"regime": map[string]interface{}{
			"current":    "trending_bull",
			"confidence": 0.85,
			"weights_applied": map[string]float64{
				"momentum_core": 0.40,
				"technical":     0.30,
				"volume":        0.20,
				"quality":       0.08,
				"social":        0.02,
			},
		},
		"microstructure": map[string]interface{}{
			"spread_bps":      12.5,
			"depth_usd":       145000,
			"vadr":            2.3,
			"venue":           "kraken",
			"exchange_native": true,
		},
		"attribution": map[string]interface{}{
			"calculation_time": "2024-09-06T23:45:12Z",
			"data_sources":     []string{"kraken_rest", "kraken_ws"},
			"cache_layers":     []string{"redis_hot", "file_warm"},
			"regime_detection": "4h_majority_vote",
		},
	}
}
