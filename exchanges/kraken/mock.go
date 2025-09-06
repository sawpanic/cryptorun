package kraken

import (
	"net/http"
	"net/http/httptest"
)

// NewTimeoutMockServer creates a mock server that times out first N requests
func NewTimeoutMockServer(timeoutCount int) *httptest.Server {
	requestCount := 0
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		if requestCount <= timeoutCount {
			// Simulate timeout by not responding
			select {}
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"result": {"BTCUSD": {"bid": "100.0", "ask": "101.0"}}}`))
	}))
}
