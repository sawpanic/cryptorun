package kraken

import (
	"fmt"
	"net/http"
	"net/http/httptest"
)

// EmptyBookMockServer simulates empty order book scenarios for resilience testing
type EmptyBookMockServer struct {
	server      *httptest.Server
	emptyCount  int
	emptyAfter  int // Number of requests that return empty books
	scenarioType string
}

// EmptyBookScenario defines types of empty book responses
type EmptyBookScenario string

const (
	EmptyBookNoLevels     EmptyBookScenario = "no_levels"      // No bid/ask levels
	EmptyBookZeroVolume   EmptyBookScenario = "zero_volume"    // Levels with zero volume
	EmptyBookZeroPrices   EmptyBookScenario = "zero_prices"    // Levels with zero prices
	EmptyBookResultEmpty  EmptyBookScenario = "result_empty"   // Empty result object
	EmptyBookNoTickers    EmptyBookScenario = "no_tickers"     // No ticker pairs in response
)

// NewEmptyBookMockServer creates a mock server that returns empty order books
func NewEmptyBookMockServer(emptyAfter int, scenario EmptyBookScenario) *EmptyBookMockServer {
	mock := &EmptyBookMockServer{
		emptyAfter:   emptyAfter,
		scenarioType: string(scenario),
	}
	
	mock.server = httptest.NewServer(http.HandlerFunc(mock.handleRequest))
	return mock
}

// URL returns the mock server URL
func (m *EmptyBookMockServer) URL() string {
	return m.server.URL
}

// Close shuts down the mock server
func (m *EmptyBookMockServer) Close() {
	m.server.Close()
}

// GetEmptyResponseCount returns how many empty responses have been sent
func (m *EmptyBookMockServer) GetEmptyResponseCount() int {
	return m.emptyCount
}

// handleRequest handles incoming HTTP requests
func (m *EmptyBookMockServer) handleRequest(w http.ResponseWriter, r *http.Request) {
	// Return empty book for the first N requests
	if m.emptyCount < m.emptyAfter {
		m.emptyCount++
		m.sendEmptyBookResponse(w, r)
		return
	}
	
	// After empty period, return normal response
	switch r.URL.Path {
	case "/0/public/Ticker":
		m.handleNormalTickerRequest(w, r)
	case "/0/public/Depth":
		m.handleNormalDepthRequest(w, r)
	case "/0/public/AssetPairs":
		m.handleNormalAssetPairsRequest(w, r)
	default:
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprintf(w, `{"error": ["Unknown:Path not found"]}`)
	}
}

// sendEmptyBookResponse sends an empty book response based on scenario
func (m *EmptyBookMockServer) sendEmptyBookResponse(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	
	switch r.URL.Path {
	case "/0/public/Ticker":
		m.sendEmptyTickerResponse(w)
	case "/0/public/Depth":
		m.sendEmptyDepthResponse(w)
	case "/0/public/AssetPairs":
		m.sendEmptyAssetPairsResponse(w)
	default:
		fmt.Fprintf(w, `{"error": [], "result": {}}`)
	}
}

// sendEmptyTickerResponse sends empty ticker based on scenario
func (m *EmptyBookMockServer) sendEmptyTickerResponse(w http.ResponseWriter) {
	switch EmptyBookScenario(m.scenarioType) {
	case EmptyBookNoTickers:
		// No ticker pairs
		fmt.Fprint(w, `{"error": [], "result": {}}`)
		
	case EmptyBookZeroVolume:
		// Tickers with zero volume
		response := `{
			"error": [],
			"result": {
				"XXBTZUSD": {
					"a": ["50000.00000", "0", "0.000"],
					"b": ["49975.00000", "0", "0.000"],
					"c": ["49990.00000", "0.00000000"],
					"v": ["0.00000000", "0.00000000"],
					"p": ["0.00000", "0.00000"],
					"t": [0, 0],
					"l": ["0.00000", "0.00000"],
					"h": ["0.00000", "0.00000"],
					"o": "0.00000"
				}
			}
		}`
		fmt.Fprint(w, response)
		
	case EmptyBookZeroPrices:
		// Tickers with zero prices
		response := `{
			"error": [],
			"result": {
				"XXBTZUSD": {
					"a": ["0.00000", "1", "1.000"],
					"b": ["0.00000", "2", "2.000"],
					"c": ["0.00000", "0.10000000"],
					"v": ["100.00000000", "200.00000000"],
					"p": ["0.00000", "0.00000"],
					"t": [150, 300],
					"l": ["0.00000", "0.00000"],
					"h": ["0.00000", "0.00000"],
					"o": "0.00000"
				}
			}
		}`
		fmt.Fprint(w, response)
		
	default:
		// Default: no tickers
		fmt.Fprint(w, `{"error": [], "result": {}}`)
	}
}

// sendEmptyDepthResponse sends empty depth based on scenario  
func (m *EmptyBookMockServer) sendEmptyDepthResponse(w http.ResponseWriter) {
	switch EmptyBookScenario(m.scenarioType) {
	case EmptyBookNoLevels:
		// No bid/ask levels
		response := `{
			"error": [],
			"result": {
				"XXBTZUSD": {
					"bids": [],
					"asks": []
				}
			}
		}`
		fmt.Fprint(w, response)
		
	case EmptyBookZeroVolume:
		// Levels with zero volume
		response := `{
			"error": [],
			"result": {
				"XXBTZUSD": {
					"bids": [
						["49975.00000", "0.00000000", 1693363200]
					],
					"asks": [
						["50000.00000", "0.00000000", 1693363200]
					]
				}
			}
		}`
		fmt.Fprint(w, response)
		
	case EmptyBookZeroPrices:
		// Levels with zero prices
		response := `{
			"error": [],
			"result": {
				"XXBTZUSD": {
					"bids": [
						["0.00000", "1.00000000", 1693363200]
					],
					"asks": [
						["0.00000", "1.00000000", 1693363200]
					]
				}
			}
		}`
		fmt.Fprint(w, response)
		
	default:
		// Default: empty result
		fmt.Fprint(w, `{"error": [], "result": {}}`)
	}
}

// sendEmptyAssetPairsResponse sends empty asset pairs response
func (m *EmptyBookMockServer) sendEmptyAssetPairsResponse(w http.ResponseWriter) {
	fmt.Fprint(w, `{"error": [], "result": {}}`)
}

// handleNormalTickerRequest returns a normal ticker response
func (m *EmptyBookMockServer) handleNormalTickerRequest(w http.ResponseWriter, r *http.Request) {
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

// handleNormalDepthRequest returns a normal depth response
func (m *EmptyBookMockServer) handleNormalDepthRequest(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	
	response := `{
		"error": [],
		"result": {
			"XXBTZUSD": {
				"bids": [
					["49975.00000", "1.50000000", 1693363200],
					["49970.00000", "2.00000000", 1693363190],
					["49965.00000", "0.75000000", 1693363180]
				],
				"asks": [
					["50000.00000", "1.20000000", 1693363200],
					["50005.00000", "1.80000000", 1693363190],
					["50010.00000", "0.95000000", 1693363180]
				]
			}
		}
	}`
	
	fmt.Fprint(w, response)
}

// handleNormalAssetPairsRequest returns a normal asset pairs response
func (m *EmptyBookMockServer) handleNormalAssetPairsRequest(w http.ResponseWriter, r *http.Request) {
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

// CreateEmptyBookTestSuite creates multiple empty book scenarios
func CreateEmptyBookTestSuite() map[string]*EmptyBookMockServer {
	return map[string]*EmptyBookMockServer{
		"no_levels":     NewEmptyBookMockServer(2, EmptyBookNoLevels),
		"zero_volume":   NewEmptyBookMockServer(2, EmptyBookZeroVolume),
		"zero_prices":   NewEmptyBookMockServer(2, EmptyBookZeroPrices),
		"result_empty":  NewEmptyBookMockServer(1, EmptyBookResultEmpty),
		"no_tickers":    NewEmptyBookMockServer(1, EmptyBookNoTickers),
	}
}

// ExpectedGateFailures returns the expected gate failures for empty book scenarios
func ExpectedGateFailures(scenario EmptyBookScenario) []string {
	switch scenario {
	case EmptyBookNoLevels:
		return []string{"DEPTH_THIN", "SPREAD_WIDE"}
	case EmptyBookZeroVolume:
		return []string{"DEPTH_THIN"}
	case EmptyBookZeroPrices:
		return []string{"SPREAD_WIDE", "DEPTH_THIN"}
	case EmptyBookResultEmpty:
		return []string{"DEPTH_THIN", "SPREAD_WIDE"}
	case EmptyBookNoTickers:
		return []string{"DATA_UNAVAILABLE"}
	default:
		return []string{"UNKNOWN_FAILURE"}
	}
}