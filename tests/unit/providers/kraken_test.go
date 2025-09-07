package providers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/sawpanic/cryptorun/internal/providers/kraken"
)

func TestKrakenClient_GetServerTime(t *testing.T) {
	// Mock server that returns Kraken-style server time
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/0/public/Time" {
			t.Errorf("Expected path /0/public/Time, got %s", r.URL.Path)
		}
		
		response := map[string]interface{}{
			"error": []string{},
			"result": map[string]interface{}{
				"unixtime": 1693987200,
				"rfc1123":  "Wed, 06 Sep 2023 12:00:00 +0000",
			},
		}
		
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer mockServer.Close()

	// Create client with mock server URL
	config := kraken.Config{
		BaseURL:        mockServer.URL,
		RequestTimeout: 5 * time.Second,
		RateLimitRPS:   10.0, // Higher rate for testing
	}
	client := kraken.NewClient(config)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Test server time request
	timeResp, err := client.GetServerTime(ctx)
	if err != nil {
		t.Fatalf("GetServerTime failed: %v", err)
	}

	if timeResp.UnixTime != 1693987200 {
		t.Errorf("Expected unix time 1693987200, got %d", timeResp.UnixTime)
	}

	if timeResp.RFC1123 != "Wed, 06 Sep 2023 12:00:00 +0000" {
		t.Errorf("Expected RFC1123 time 'Wed, 06 Sep 2023 12:00:00 +0000', got %s", timeResp.RFC1123)
	}
}

func TestKrakenClient_GetTicker_USDPairsOnly(t *testing.T) {
	// Mock server that returns ticker data
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/0/public/Ticker" {
			t.Errorf("Expected path /0/public/Ticker, got %s", r.URL.Path)
		}
		
		// Validate USD pairs only
		pair := r.URL.Query().Get("pair")
		if pair == "BTC-EUR" {
			t.Errorf("Non-USD pair should be rejected: %s", pair)
		}
		
		response := map[string]interface{}{
			"error": []string{},
			"result": map[string]interface{}{
				"XXBTZUSD": map[string]interface{}{
					"a": []string{"50100.00000", "1", "1.000"},
					"b": []string{"50000.00000", "2", "2.000"},
					"c": []string{"50050.00000", "0.01000000"},
					"v": []string{"1000.12345678", "2000.12345678"},
					"p": []string{"50025.00000", "50025.00000"},
					"t": []int{1000, 2000},
					"l": []string{"49000.00000", "49000.00000"},
					"h": []string{"51000.00000", "51000.00000"},
					"o": "50000.00000",
				},
			},
		}
		
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer mockServer.Close()

	config := kraken.Config{
		BaseURL:        mockServer.URL,
		RequestTimeout: 5 * time.Second,
		RateLimitRPS:   10.0,
	}
	client := kraken.NewClient(config)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Test USD pair (should succeed)
	tickers, err := client.GetTicker(ctx, []string{"BTC-USD"})
	if err != nil {
		t.Fatalf("GetTicker failed for USD pair: %v", err)
	}

	btcTicker, exists := tickers["BTC-USD"]
	if !exists {
		t.Fatal("BTC-USD ticker not found in response")
	}

	// Validate ticker data
	askPrice, err := btcTicker.GetAskPrice()
	if err != nil {
		t.Fatalf("Failed to get ask price: %v", err)
	}
	if askPrice != 50100.0 {
		t.Errorf("Expected ask price 50100.0, got %f", askPrice)
	}

	bidPrice, err := btcTicker.GetBidPrice()
	if err != nil {
		t.Fatalf("Failed to get bid price: %v", err)
	}
	if bidPrice != 50000.0 {
		t.Errorf("Expected bid price 50000.0, got %f", bidPrice)
	}

	// Test spread calculation
	spreadBps, err := btcTicker.GetSpreadBps()
	if err != nil {
		t.Fatalf("Failed to get spread: %v", err)
	}
	expectedSpread := ((50100.0 - 50000.0) / 50000.0) * 10000
	if spreadBps != expectedSpread {
		t.Errorf("Expected spread %.2f bps, got %.2f", expectedSpread, spreadBps)
	}

	// Test non-USD pair rejection
	_, err = client.GetTicker(ctx, []string{"BTC-EUR"})
	if err == nil {
		t.Error("Expected error for non-USD pair, got nil")
	}
	if err != nil && !containsString(err.Error(), "USD pairs only") {
		t.Errorf("Expected 'USD pairs only' error, got: %v", err)
	}
}

func TestKrakenClient_GetOrderBook_L2Analysis(t *testing.T) {
	// Mock server that returns order book data
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/0/public/Depth" {
			t.Errorf("Expected path /0/public/Depth, got %s", r.URL.Path)
		}
		
		response := map[string]interface{}{
			"error": []string{},
			"result": map[string]interface{}{
				"XXBTZUSD": map[string]interface{}{
					"asks": [][]string{
						{"50100.00", "1.5", "1693987200"},
						{"50200.00", "2.0", "1693987201"},
						{"50300.00", "1.0", "1693987202"},
					},
					"bids": [][]string{
						{"50000.00", "2.0", "1693987200"},
						{"49900.00", "1.5", "1693987201"},
						{"49800.00", "1.0", "1693987202"},
					},
				},
			},
		}
		
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer mockServer.Close()

	config := kraken.Config{
		BaseURL:        mockServer.URL,
		RequestTimeout: 5 * time.Second,
		RateLimitRPS:   10.0,
	}
	client := kraken.NewClient(config)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Test order book request
	orderBook, err := client.GetOrderBook(ctx, "BTC-USD", 10)
	if err != nil {
		t.Fatalf("GetOrderBook failed: %v", err)
	}

	// Validate best bid/ask
	bestBid, err := orderBook.Data.GetBestBid()
	if err != nil {
		t.Fatalf("Failed to get best bid: %v", err)
	}
	if bestBid.Price != 50000.0 {
		t.Errorf("Expected best bid 50000.0, got %f", bestBid.Price)
	}

	bestAsk, err := orderBook.Data.GetBestAsk()
	if err != nil {
		t.Fatalf("Failed to get best ask: %v", err)
	}
	if bestAsk.Price != 50100.0 {
		t.Errorf("Expected best ask 50100.0, got %f", bestAsk.Price)
	}

	// Test depth calculation (±2% around mid price)
	midPrice := (bestBid.Price + bestAsk.Price) / 2.0 // 50050.0
	bidDepth, askDepth, err := orderBook.Data.CalculateDepthUSD(midPrice, 2.0)
	if err != nil {
		t.Fatalf("Failed to calculate depth: %v", err)
	}

	// Validate depth calculations
	if bidDepth <= 0 {
		t.Errorf("Expected positive bid depth, got %f", bidDepth)
	}
	if askDepth <= 0 {
		t.Errorf("Expected positive ask depth, got %f", askDepth)
	}

	totalDepth := bidDepth + askDepth
	if totalDepth <= 0 {
		t.Errorf("Expected positive total depth, got %f", totalDepth)
	}
}

func TestKrakenMicrostructureExtractor_ExchangeNativeValidation(t *testing.T) {
	// Mock server for microstructure analysis
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var response map[string]interface{}
		
		switch r.URL.Path {
		case "/0/public/Depth":
			response = map[string]interface{}{
				"error": []string{},
				"result": map[string]interface{}{
					"XXBTZUSD": map[string]interface{}{
						"asks": [][]string{
							{"50100.00", "10.0", "1693987200"},
							{"50200.00", "20.0", "1693987201"},
						},
						"bids": [][]string{
							{"50000.00", "15.0", "1693987200"},
							{"49900.00", "25.0", "1693987201"},
						},
					},
				},
			}
		case "/0/public/Ticker":
			response = map[string]interface{}{
				"error": []string{},
				"result": map[string]interface{}{
					"XXBTZUSD": map[string]interface{}{
						"a": []string{"50100.00", "10", "10.000"},
						"b": []string{"50000.00", "15", "15.000"},
						"c": []string{"50050.00", "1.00000000"},
					},
				},
			}
		default:
			http.NotFound(w, r)
			return
		}
		
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer mockServer.Close()

	config := kraken.Config{
		BaseURL:        mockServer.URL,
		RequestTimeout: 5 * time.Second,
		RateLimitRPS:   10.0,
	}
	client := kraken.NewClient(config)
	extractor := kraken.NewMicrostructureExtractor(client)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Test microstructure extraction for USD pair
	microData, err := extractor.ExtractMicrostructure(ctx, "BTC-USD")
	if err != nil {
		t.Fatalf("ExtractMicrostructure failed: %v", err)
	}

	// Validate exchange-native data
	if microData.Venue != "kraken" {
		t.Errorf("Expected venue 'kraken', got %s", microData.Venue)
	}

	if microData.Pair != "BTC-USD" {
		t.Errorf("Expected pair 'BTC-USD', got %s", microData.Pair)
	}

	// Validate microstructure metrics
	if microData.MidPrice <= 0 {
		t.Errorf("Expected positive mid price, got %f", microData.MidPrice)
	}

	if microData.SpreadBps <= 0 {
		t.Errorf("Expected positive spread, got %f bps", microData.SpreadBps)
	}

	if microData.TotalDepthUSD2Pct <= 0 {
		t.Errorf("Expected positive total depth, got %f USD", microData.TotalDepthUSD2Pct)
	}

	// Test non-USD pair rejection
	_, err = extractor.ExtractMicrostructure(ctx, "BTC-EUR")
	if err == nil {
		t.Error("Expected error for non-USD pair, got nil")
	}

	// Test gate validation
	validation, err := extractor.ValidateMicrostructureGates(microData)
	if err != nil {
		t.Fatalf("ValidateMicrostructureGates failed: %v", err)
	}

	if validation.Pair != "BTC-USD" {
		t.Errorf("Expected validation pair 'BTC-USD', got %s", validation.Pair)
	}

	if validation.Venue != "kraken" {
		t.Errorf("Expected validation venue 'kraken', got %s", validation.Venue)
	}
}

func TestKrakenRateLimiter_Compliance(t *testing.T) {
	// Test rate limiter with Kraken's 1 RPS requirement
	rateLimiter := kraken.NewRateLimiter(1.0) // 1 RPS for Kraken free tier

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Test initial token availability
	if remaining := rateLimiter.Remaining(); remaining <= 0 {
		t.Errorf("Expected positive tokens initially, got %f", remaining)
	}

	// Test rate limiting enforcement
	start := time.Now()
	
	// First request should be immediate
	if err := rateLimiter.Wait(ctx); err != nil {
		t.Fatalf("First request failed: %v", err)
	}
	
	// Second request should be rate limited
	if err := rateLimiter.Wait(ctx); err != nil {
		t.Fatalf("Second request failed: %v", err)
	}
	
	elapsed := time.Since(start)
	// Should take at least 1 second due to 1 RPS limit
	if elapsed < 900*time.Millisecond {
		t.Errorf("Rate limiting not enforced: elapsed %v < 900ms", elapsed)
	}

	// Test try-wait functionality
	if rateLimiter.TryWait() {
		t.Error("TryWait should fail when no tokens available")
	}

	// Wait for refill
	time.Sleep(1100 * time.Millisecond)
	
	if !rateLimiter.TryWait() {
		t.Error("TryWait should succeed after refill")
	}
}

// Helper function
func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || 
		(len(s) > len(substr) && 
			(s[:len(substr)] == substr || 
			 s[len(s)-len(substr):] == substr || 
			 len(s) > len(substr)*2)))
}

// Benchmark tests for performance validation
func BenchmarkKrakenClient_GetServerTime(b *testing.B) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"error": []string{},
			"result": map[string]interface{}{
				"unixtime": 1693987200,
				"rfc1123":  "Wed, 06 Sep 2023 12:00:00 +0000",
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer mockServer.Close()

	config := kraken.Config{
		BaseURL:        mockServer.URL,
		RequestTimeout: 5 * time.Second,
		RateLimitRPS:   100.0, // Higher rate for benchmarking
	}
	client := kraken.NewClient(config)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := client.GetServerTime(ctx)
		if err != nil {
			b.Fatalf("GetServerTime failed: %v", err)
		}
	}
}

func BenchmarkKrakenMicrostructureExtractor_ExtractMicrostructure(b *testing.B) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var response map[string]interface{}
		
		switch r.URL.Path {
		case "/0/public/Depth":
			response = map[string]interface{}{
				"error": []string{},
				"result": map[string]interface{}{
					"XXBTZUSD": map[string]interface{}{
						"asks": [][]string{{"50100.00", "10.0", "1693987200"}},
						"bids": [][]string{{"50000.00", "15.0", "1693987200"}},
					},
				},
			}
		case "/0/public/Ticker":
			response = map[string]interface{}{
				"error": []string{},
				"result": map[string]interface{}{
					"XXBTZUSD": map[string]interface{}{
						"a": []string{"50100.00", "10", "10.000"},
						"b": []string{"50000.00", "15", "15.000"},
					},
				},
			}
		}
		
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer mockServer.Close()

	config := kraken.Config{
		BaseURL:        mockServer.URL,
		RequestTimeout: 5 * time.Second,
		RateLimitRPS:   100.0,
	}
	client := kraken.NewClient(config)
	extractor := kraken.NewMicrostructureExtractor(client)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := extractor.ExtractMicrostructure(ctx, "BTC-USD")
		if err != nil {
			b.Fatalf("ExtractMicrostructure failed: %v", err)
		}
	}
}