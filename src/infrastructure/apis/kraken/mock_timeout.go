package kraken

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"time"
)

// TimeoutMockServer simulates API timeout scenarios for resilience testing
type TimeoutMockServer struct {
	server       *httptest.Server
	timeoutCount int
	timeoutAfter int // Number of requests before timeout stops
}

// NewTimeoutMockServer creates a mock server that simulates timeouts
// timeoutAfter: number of requests that will timeout before returning to normal
func NewTimeoutMockServer(timeoutAfter int) *TimeoutMockServer {
	mock := &TimeoutMockServer{
		timeoutAfter: timeoutAfter,
	}
	
	mock.server = httptest.NewServer(http.HandlerFunc(mock.handleRequest))
	return mock
}

// URL returns the mock server URL
func (m *TimeoutMockServer) URL() string {
	return m.server.URL
}

// Close shuts down the mock server
func (m *TimeoutMockServer) Close() {
	m.server.Close()
}

// GetTimeoutCount returns how many requests have timed out
func (m *TimeoutMockServer) GetTimeoutCount() int {
	return m.timeoutCount
}

// ResetTimeoutCount resets the timeout counter
func (m *TimeoutMockServer) ResetTimeoutCount() {
	m.timeoutCount = 0
}

// handleRequest handles incoming HTTP requests
func (m *TimeoutMockServer) handleRequest(w http.ResponseWriter, r *http.Request) {
	// Simulate timeout for the first N requests
	if m.timeoutCount < m.timeoutAfter {
		m.timeoutCount++
		
		// Sleep longer than typical client timeout to trigger timeout
		select {
		case <-time.After(35 * time.Second): // Longer than typical 30s timeout
			// This should not be reached in normal testing
			w.WriteHeader(http.StatusRequestTimeout)
			fmt.Fprintf(w, `{"error": ["Service:Timeout"]}`)
		case <-r.Context().Done():
			// Client cancelled - this is what we expect
			return
		}
		return
	}
	
	// After timeout period, return normal response
	switch r.URL.Path {
	case "/0/public/Ticker":
		m.handleTickerRequest(w, r)
	case "/0/public/AssetPairs":
		m.handleAssetPairsRequest(w, r)
	default:
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprintf(w, `{"error": ["Unknown:Path not found"]}`)
	}
}

// handleTickerRequest returns a normal ticker response
func (m *TimeoutMockServer) handleTickerRequest(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	
	response := `{
		"error": [],
		"result": {
			"XXBTZUSD": {
				"a": ["50000.00000", "1", "1.000"],
				"b": ["49975.00000", "2", "2.000"],
				"c": ["49990.00000", "0.10000000"],
				"v": ["100.00000000", "200.00000000"],
				"p": ["49950.00000", "49960.00000"],
				"t": [150, 300],
				"l": ["49900.00000", "49850.00000"],
				"h": ["50100.00000", "50200.00000"],
				"o": "49925.00000"
			},
			"XETHZUSD": {
				"a": ["3000.00000", "5", "5.000"],
				"b": ["2995.00000", "3", "3.000"],
				"c": ["2998.00000", "0.50000000"],
				"v": ["500.00000000", "800.00000000"],
				"p": ["2990.00000", "2995.00000"],
				"t": [80, 150],
				"l": ["2980.00000", "2970.00000"],
				"h": ["3020.00000", "3050.00000"],
				"o": "2985.00000"
			}
		}
	}`
	
	fmt.Fprint(w, response)
}

// handleAssetPairsRequest returns a normal asset pairs response
func (m *TimeoutMockServer) handleAssetPairsRequest(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	
	response := `{
		"error": [],
		"result": {
			"XXBTZUSD": {
				"altname": "XBTUSD",
				"wsname": "XBT/USD",
				"aclass_base": "currency",
				"base": "XXBT",
				"aclass_quote": "currency", 
				"quote": "ZUSD",
				"lot": "unit",
				"pair_decimals": 1,
				"lot_decimals": 8,
				"lot_multiplier": 1,
				"leverage_buy": [2, 3, 4, 5],
				"leverage_sell": [2, 3, 4, 5],
				"fees": [[0, 0.26], [50000, 0.24], [100000, 0.22]],
				"fees_maker": [[0, 0.16], [50000, 0.14], [100000, 0.12]],
				"fee_volume_currency": "ZUSD",
				"margin_call": 80,
				"margin_stop": 40,
				"ordermin": "0.0001"
			}
		}
	}`
	
	fmt.Fprint(w, response)
}

// SimulateCircuitBreakerScenario creates a scenario to test circuit breaker
// Returns a server that will timeout for specified number of requests
func SimulateCircuitBreakerScenario(ctx context.Context, timeoutRequests int) (*TimeoutMockServer, error) {
	server := NewTimeoutMockServer(timeoutRequests)
	
	// Verify server is running
	select {
	case <-time.After(100 * time.Millisecond):
		return server, nil
	case <-ctx.Done():
		server.Close()
		return nil, ctx.Err()
	}
}