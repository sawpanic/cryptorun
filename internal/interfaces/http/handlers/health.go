package handlers

import (
	"net/http"
	"time"

	httpContracts "github.com/sawpanic/cryptorun/internal/http"
)

// Health handles GET /health endpoint
func (h *Handlers) Health(w http.ResponseWriter, r *http.Request) {
	// Mock health data - in real implementation, this would query actual services
	response := httpContracts.HealthResponse{
		Status:    "healthy",
		Timestamp: time.Now().UTC(),
		Providers: map[string]httpContracts.ProviderHealth{
			"kraken": {
				Name:   "kraken",
				Status: "healthy",
				RateLimit: httpContracts.RateLimit{
					Current:   145,
					Limit:     1000,
					Remaining: 855,
					ResetTime: int(time.Now().Add(time.Minute).Unix()),
				},
				LastResponse: time.Now().Add(-2 * time.Second),
				ErrorRate:    0.02,
			},
			"okx": {
				Name:   "okx",
				Status: "degraded",
				RateLimit: httpContracts.RateLimit{
					Current:   890,
					Limit:     1000,
					Remaining: 110,
					ResetTime: int(time.Now().Add(30 * time.Second).Unix()),
				},
				LastResponse: time.Now().Add(-15 * time.Second),
				ErrorRate:    0.08,
			},
		},
		Circuits: map[string]httpContracts.CircuitHealth{
			"kraken_rest": {
				Name:        "kraken_rest",
				State:       "closed",
				FailureRate: 0.01,
				Requests:    1425,
				Failures:    14,
			},
			"okx_ws": {
				Name:        "okx_ws",
				State:       "half-open",
				FailureRate: 0.12,
				Requests:    892,
				Failures:    107,
			},
		},
		Latencies: httpContracts.LatencyMetrics{
			P95Handler: 125 * time.Millisecond,
			P99Handler: 280 * time.Millisecond,
			AvgHandler: 45 * time.Millisecond,
			Target:     300 * time.Millisecond, // P99 target from requirements
		},
	}

	h.writeJSON(w, http.StatusOK, response)
}
