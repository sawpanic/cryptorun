package kraken

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
)

// BadJSONMockServer simulates malformed JSON responses for resilience testing
type BadJSONMockServer struct {
	server     *httptest.Server
	badCount   int
	badAfter   int // Number of requests that return bad JSON
	responseType string // Type of bad response to simulate
}

// BadJSONResponseType defines types of malformed responses
type BadJSONResponseType string

const (
	BadJSONMalformed     BadJSONResponseType = "malformed"     // Invalid JSON syntax
	BadJSONTruncated     BadJSONResponseType = "truncated"     // Incomplete JSON
	BadJSONWrongSchema   BadJSONResponseType = "wrong_schema"  // Valid JSON, wrong structure
	BadJSONEmptyResponse BadJSONResponseType = "empty"         // Empty response body
	BadJSONNullResponse  BadJSONResponseType = "null"          // Null response
)

// NewBadJSONMockServer creates a mock server that returns malformed JSON
func NewBadJSONMockServer(badAfter int, responseType BadJSONResponseType) *BadJSONMockServer {
	mock := &BadJSONMockServer{
		badAfter:     badAfter,
		responseType: string(responseType),
	}
	
	mock.server = httptest.NewServer(http.HandlerFunc(mock.handleRequest))
	return mock
}

// URL returns the mock server URL
func (m *BadJSONMockServer) URL() string {
	return m.server.URL
}

// Close shuts down the mock server
func (m *BadJSONMockServer) Close() {
	m.server.Close()
}

// GetBadResponseCount returns how many bad responses have been sent
func (m *BadJSONMockServer) GetBadResponseCount() int {
	return m.badCount
}

// handleRequest handles incoming HTTP requests
func (m *BadJSONMockServer) handleRequest(w http.ResponseWriter, r *http.Request) {
	// Return bad JSON for the first N requests
	if m.badCount < m.badAfter {
		m.badCount++
		m.sendBadResponse(w, r)
		return
	}
	
	// After bad period, return normal response
	switch r.URL.Path {
	case "/0/public/Ticker":
		m.handleNormalTickerRequest(w, r)
	case "/0/public/AssetPairs":
		m.handleNormalAssetPairsRequest(w, r)
	default:
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprintf(w, `{"error": ["Unknown:Path not found"]}`)
	}
}

// sendBadResponse sends a malformed response based on responseType
func (m *BadJSONMockServer) sendBadResponse(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	
	switch BadJSONResponseType(m.responseType) {
	case BadJSONMalformed:
		// Invalid JSON syntax
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"error": [], "result": {"XXBTZUSD": {"a": [invalid json}`)
		
	case BadJSONTruncated:
		// Incomplete JSON (cut off mid-response)
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"error": [], "result": {"XXBTZUSD": {"a": ["50000.00000", "1", "1.000"], "b": ["49975.00000"`)
		
	case BadJSONWrongSchema:
		// Valid JSON but wrong structure
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"status": "ok", "data": ["this", "is", "not", "kraken", "format"]}`)
		
	case BadJSONEmptyResponse:
		// Empty response body
		w.WriteHeader(http.StatusOK)
		// Don't write anything
		
	case BadJSONNullResponse:
		// Null response
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `null`)
		
	default:
		// Default to malformed JSON
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{malformed json response}`)
	}
}

// handleNormalTickerRequest returns a valid ticker response
func (m *BadJSONMockServer) handleNormalTickerRequest(w http.ResponseWriter, r *http.Request) {
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
			}
		}
	}`
	
	fmt.Fprint(w, response)
}

// handleNormalAssetPairsRequest returns a valid asset pairs response
func (m *BadJSONMockServer) handleNormalAssetPairsRequest(w http.ResponseWriter, r *http.Request) {
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

// CreateBadJSONTestSuite creates multiple bad JSON scenarios for comprehensive testing
func CreateBadJSONTestSuite() map[string]*BadJSONMockServer {
	return map[string]*BadJSONMockServer{
		"malformed":    NewBadJSONMockServer(3, BadJSONMalformed),
		"truncated":    NewBadJSONMockServer(2, BadJSONTruncated),
		"wrong_schema": NewBadJSONMockServer(2, BadJSONWrongSchema),
		"empty":        NewBadJSONMockServer(1, BadJSONEmptyResponse),
		"null":         NewBadJSONMockServer(1, BadJSONNullResponse),
	}
}

// ValidateErrorHandling checks if an error message indicates proper JSON error handling
func ValidateErrorHandling(err error) bool {
	if err == nil {
		return false
	}
	
	errStr := strings.ToLower(err.Error())
	
	// Check for common JSON parsing error indicators
	jsonErrorIndicators := []string{
		"json",
		"parse",
		"unmarshal",
		"syntax",
		"invalid character",
		"unexpected end",
		"malformed",
	}
	
	for _, indicator := range jsonErrorIndicators {
		if strings.Contains(errStr, indicator) {
			return true
		}
	}
	
	return false
}