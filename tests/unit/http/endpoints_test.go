package http

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	httpContracts "github.com/sawpanic/cryptorun/internal/interfaces/http"
	"github.com/sawpanic/cryptorun/internal/interfaces/http/endpoints"
	"github.com/sawpanic/cryptorun/internal/metrics"
)

func TestCandidatesHandler(t *testing.T) {
	collector := metrics.NewCollector()
	handler := endpoints.CandidatesHandler(collector)

	tests := []struct {
		name           string
		queryParams    string
		expectedStatus int
		expectedFields []string
	}{
		{
			name:           "default_request",
			queryParams:    "",
			expectedStatus: http.StatusOK,
			expectedFields: []string{"timestamp", "regime", "candidates", "summary"},
		},
		{
			name:           "with_limit_parameter",
			queryParams:    "?n=10",
			expectedStatus: http.StatusOK,
			expectedFields: []string{"timestamp", "regime", "candidates", "summary"},
		},
		{
			name:           "invalid_limit_parameter",
			queryParams:    "?n=invalid",
			expectedStatus: http.StatusBadRequest,
			expectedFields: []string{"error", "message", "timestamp"},
		},
		{
			name:           "limit_too_high",
			queryParams:    "?n=300",
			expectedStatus: http.StatusBadRequest,
			expectedFields: []string{"error", "message", "timestamp"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/candidates"+tt.queryParams, nil)
			w := httptest.NewRecorder()

			handler(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
			assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			require.NoError(t, err)

			for _, field := range tt.expectedFields {
				assert.Contains(t, response, field, "Response should contain field: %s", field)
			}

			if tt.expectedStatus == http.StatusOK {
				// Validate response structure
				validateCandidatesResponse(t, response)
			}
		})
	}
}

func TestCandidatesHandler_MethodNotAllowed(t *testing.T) {
	collector := metrics.NewCollector()
	handler := endpoints.CandidatesHandler(collector)

	req := httptest.NewRequest(http.MethodPost, "/candidates", nil)
	w := httptest.NewRecorder()

	handler(w, req)

	assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
	assert.Equal(t, "GET", w.Header().Get("Allow"))
}

func TestExplainHandler(t *testing.T) {
	handler := endpoints.ExplainHandler()

	tests := []struct {
		name           string
		path           string
		expectedStatus int
		expectedFields []string
	}{
		{
			name:           "valid_symbol",
			path:           "/explain/BTC-USD",
			expectedStatus: http.StatusOK,
			expectedFields: []string{"symbol", "score", "gates", "factors", "regime", "attribution"},
		},
		{
			name:           "another_valid_symbol",
			path:           "/explain/ETH-USD",
			expectedStatus: http.StatusOK,
			expectedFields: []string{"symbol", "score", "gates", "factors", "regime", "attribution"},
		},
		{
			name:           "invalid_symbol_format",
			path:           "/explain/INVALID",
			expectedStatus: http.StatusBadRequest,
			expectedFields: []string{"error", "message", "timestamp"},
		},
		{
			name:           "missing_symbol",
			path:           "/explain/",
			expectedStatus: http.StatusBadRequest,
			expectedFields: []string{"error", "message", "timestamp"},
		},
		{
			name:           "invalid_path",
			path:           "/wrong/path",
			expectedStatus: http.StatusBadRequest,
			expectedFields: []string{"error", "message", "timestamp"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			w := httptest.NewRecorder()

			handler(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
			assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			require.NoError(t, err)

			for _, field := range tt.expectedFields {
				assert.Contains(t, response, field, "Response should contain field: %s", field)
			}

			if tt.expectedStatus == http.StatusOK {
				// Validate response structure
				validateExplainResponse(t, response)
			}
		})
	}
}

func TestExplainHandler_MethodNotAllowed(t *testing.T) {
	handler := endpoints.ExplainHandler()

	req := httptest.NewRequest(http.MethodPost, "/explain/BTC-USD", nil)
	w := httptest.NewRecorder()

	handler(w, req)

	assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
	assert.Equal(t, "GET", w.Header().Get("Allow"))
}

func TestRegimeHandler(t *testing.T) {
	collector := metrics.NewCollector()
	handler := endpoints.RegimeHandler(collector)

	tests := []struct {
		name           string
		expectedStatus int
		expectedFields []string
	}{
		{
			name:           "valid_request",
			expectedStatus: http.StatusOK,
			expectedFields: []string{"timestamp", "current_regime", "regime_numeric", "health", "weights", "switches_today", "avg_duration_hours", "next_evaluation", "history"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/regime", nil)
			w := httptest.NewRecorder()

			handler(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
			assert.Equal(t, "application/json", w.Header().Get("Content-Type"))
			assert.Equal(t, "max-age=30", w.Header().Get("Cache-Control"))

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			require.NoError(t, err)

			for _, field := range tt.expectedFields {
				assert.Contains(t, response, field, "Response should contain field: %s", field)
			}

			// Validate response structure
			validateRegimeResponse(t, response)
		})
	}
}

func TestRegimeHandler_MethodNotAllowed(t *testing.T) {
	collector := metrics.NewCollector()
	handler := endpoints.RegimeHandler(collector)

	req := httptest.NewRequest(http.MethodPost, "/regime", nil)
	w := httptest.NewRecorder()

	handler(w, req)

	assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
	assert.Equal(t, "GET", w.Header().Get("Allow"))
}

func TestEndpointTimeouts(t *testing.T) {
	// Test that endpoints complete within reasonable time limits
	collector := metrics.NewCollector()

	tests := []struct {
		name    string
		handler http.HandlerFunc
		path    string
		timeout time.Duration
	}{
		{
			name:    "candidates_timeout",
			handler: endpoints.CandidatesHandler(collector),
			path:    "/candidates",
			timeout: 300 * time.Millisecond,
		},
		{
			name:    "explain_timeout",
			handler: endpoints.ExplainHandler(),
			path:    "/explain/BTC-USD",
			timeout: 300 * time.Millisecond,
		},
		{
			name:    "regime_timeout",
			handler: endpoints.RegimeHandler(collector),
			path:    "/regime",
			timeout: 300 * time.Millisecond,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			w := httptest.NewRecorder()

			start := time.Now()
			tt.handler(w, req)
			duration := time.Since(start)

			assert.True(t, duration < tt.timeout, "Handler took too long: %v (limit: %v)", duration, tt.timeout)
			assert.Equal(t, http.StatusOK, w.Code)
		})
	}
}

// validateCandidatesResponse validates the structure of candidates response
func validateCandidatesResponse(t *testing.T, response map[string]interface{}) {
	// Validate top-level fields
	assert.IsType(t, "", response["timestamp"])
	assert.IsType(t, "", response["regime"])
	assert.IsType(t, float64(0), response["total_count"])
	assert.IsType(t, float64(0), response["requested"])

	// Validate candidates array
	candidates, ok := response["candidates"].([]interface{})
	require.True(t, ok, "candidates should be an array")

	if len(candidates) > 0 {
		candidate := candidates[0].(map[string]interface{})

		// Validate candidate structure
		assert.Contains(t, candidate, "symbol")
		assert.Contains(t, candidate, "score")
		assert.Contains(t, candidate, "rank")
		assert.Contains(t, candidate, "gate_status")
		assert.Contains(t, candidate, "microstructure")
		assert.Contains(t, candidate, "attribution")

		// Validate gate_status structure
		gateStatus := candidate["gate_status"].(map[string]interface{})
		assert.Contains(t, gateStatus, "overall_passed")
		assert.Contains(t, gateStatus, "score_gate")
		assert.Contains(t, gateStatus, "vadr_gate")

		// Validate microstructure structure
		microstructure := candidate["microstructure"].(map[string]interface{})
		assert.Contains(t, microstructure, "spread_bps")
		assert.Contains(t, microstructure, "depth_usd")
		assert.Contains(t, microstructure, "vadr")

		// Validate attribution structure
		attribution := candidate["attribution"].(map[string]interface{})
		assert.Contains(t, attribution, "momentum_score")
		assert.Contains(t, attribution, "weight_profile")
	}

	// Validate summary structure
	summary := response["summary"].(map[string]interface{})
	assert.Contains(t, summary, "passed_all_gates")
	assert.Contains(t, summary, "avg_score")
	assert.Contains(t, summary, "gate_pass_rates")
}

// validateExplainResponse validates the structure of explain response
func validateExplainResponse(t *testing.T, response map[string]interface{}) {
	// Validate top-level fields
	assert.IsType(t, "", response["symbol"])
	assert.IsType(t, "", response["exchange"])
	assert.IsType(t, "", response["timestamp"])
	assert.IsType(t, "", response["data_source"])

	// Validate score structure
	score := response["score"].(map[string]interface{})
	assert.Contains(t, score, "final_score")
	assert.Contains(t, score, "pre_orthogonal")
	assert.Contains(t, score, "post_orthogonal")
	assert.Contains(t, score, "weighted_scores")
	assert.Contains(t, score, "calculation_steps")

	// Validate gates structure
	gates := response["gates"].(map[string]interface{})
	assert.Contains(t, gates, "overall")
	assert.Contains(t, gates, "score_gate")
	assert.Contains(t, gates, "vadr_gate")

	// Validate gate detail structure
	scoreGate := gates["score_gate"].(map[string]interface{})
	assert.Contains(t, scoreGate, "passed")
	assert.Contains(t, scoreGate, "threshold")
	assert.Contains(t, scoreGate, "actual_value")

	// Validate factors structure
	factors := response["factors"].(map[string]interface{})
	assert.Contains(t, factors, "momentum_core")
	assert.Contains(t, factors, "technical")
	assert.Contains(t, factors, "volume")
	assert.Contains(t, factors, "quality")
	assert.Contains(t, factors, "social")

	// Validate regime structure
	regime := response["regime"].(map[string]interface{})
	assert.Contains(t, regime, "current_regime")
	assert.Contains(t, regime, "regime_weights")
	assert.Contains(t, regime, "indicators")

	// Validate attribution structure
	attribution := response["attribution"].(map[string]interface{})
	assert.Contains(t, attribution, "total_contributions")
	assert.Contains(t, attribution, "step_by_step")
	assert.Contains(t, attribution, "performance_metrics")
}

// validateRegimeResponse validates the structure of regime response
func validateRegimeResponse(t *testing.T, response map[string]interface{}) {
	// Validate top-level fields
	assert.IsType(t, "", response["timestamp"])
	assert.IsType(t, "", response["current_regime"])
	assert.IsType(t, float64(0), response["regime_numeric"])
	assert.IsType(t, float64(0), response["switches_today"])
	assert.IsType(t, float64(0), response["avg_duration_hours"])
	assert.IsType(t, "", response["next_evaluation"])

	// Validate health structure
	health := response["health"].(map[string]interface{})
	assert.Contains(t, health, "volatility_7d")
	assert.Contains(t, health, "above_ma_pct")
	assert.Contains(t, health, "breadth_thrust")
	assert.Contains(t, health, "stability_score")

	// Validate weights structure
	weights := response["weights"].(map[string]interface{})
	assert.Contains(t, weights, "momentum")
	assert.Contains(t, weights, "technical")
	assert.Contains(t, weights, "volume")
	assert.Contains(t, weights, "quality")
	assert.Contains(t, weights, "catalyst")

	// Validate history array
	history, ok := response["history"].([]interface{})
	require.True(t, ok, "history should be an array")

	if len(history) > 0 {
		regimeSwitch := history[0].(map[string]interface{})
		assert.Contains(t, regimeSwitch, "timestamp")
		assert.Contains(t, regimeSwitch, "from_regime")
		assert.Contains(t, regimeSwitch, "to_regime")
		assert.Contains(t, regimeSwitch, "trigger")
		assert.Contains(t, regimeSwitch, "confidence")
	}

	// Validate regime is one of expected values
	currentRegime := response["current_regime"].(string)
	validRegimes := []string{"trending_bull", "choppy", "high_vol"}
	assert.Contains(t, validRegimes, currentRegime)

	// Validate regime_numeric matches current_regime
	regimeNumeric := response["regime_numeric"].(float64)
	expectedNumeric := map[string]float64{
		"choppy":        0.0,
		"trending_bull": 1.0,
		"high_vol":      2.0,
	}
	assert.Equal(t, expectedNumeric[currentRegime], regimeNumeric)
}

// TestGoldenJSON tests against golden JSON files for consistent responses
func TestGoldenJSON(t *testing.T) {
	// This would be implemented with golden file comparisons in production
	// For now, we test that the JSON structure is stable
	collector := metrics.NewCollector()

	tests := []struct {
		name    string
		handler http.HandlerFunc
		path    string
	}{
		{"candidates", endpoints.CandidatesHandler(collector), "/candidates?n=3"},
		{"explain", endpoints.ExplainHandler(), "/explain/BTC-USD"},
		{"regime", endpoints.RegimeHandler(collector), "/regime"},
	}

	for _, tt := range tests {
		t.Run(tt.name+"_json_stability", func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			w := httptest.NewRecorder()

			tt.handler(w, req)

			assert.Equal(t, http.StatusOK, w.Code)

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			require.NoError(t, err, "Response should be valid JSON")

			// Ensure response is not empty
			assert.NotEmpty(t, response, "Response should not be empty")

			// Ensure JSON is pretty-printed and consistent
			prettyJSON, err := json.MarshalIndent(response, "", "  ")
			require.NoError(t, err)
			assert.True(t, len(prettyJSON) > 100, "Response should be substantial")

			// Verify it can be unmarshaled again (round-trip test)
			var roundTrip map[string]interface{}
			err = json.Unmarshal(prettyJSON, &roundTrip)
			require.NoError(t, err, "Round-trip JSON should be valid")
		})
	}
}

// BenchmarkEndpoints benchmarks the performance of endpoints
func BenchmarkEndpoints(b *testing.B) {
	collector := metrics.NewCollector()

	benchmarks := []struct {
		name    string
		handler http.HandlerFunc
		path    string
	}{
		{"candidates", endpoints.CandidatesHandler(collector), "/candidates?n=20"},
		{"explain", endpoints.ExplainHandler(), "/explain/BTC-USD"},
		{"regime", endpoints.RegimeHandler(collector), "/regime"},
	}

	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			req := httptest.NewRequest(http.MethodGet, bm.path, nil)

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				w := httptest.NewRecorder()
				bm.handler(w, req)

				if w.Code != http.StatusOK {
					b.Fatalf("Expected status 200, got %d", w.Code)
				}
			}
		})
	}
}
