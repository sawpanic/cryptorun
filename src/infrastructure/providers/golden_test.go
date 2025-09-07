package providers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"
	
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestBinanceProvider_GetFundingHistory_GoldenFile tests Binance funding history with recorded fixtures
func TestBinanceProvider_GetFundingHistory_GoldenFile(t *testing.T) {
	// Load golden file
	goldenData, err := os.ReadFile("testdata/binance_funding_history.json")
	require.NoError(t, err)
	
	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/fapi/v1/fundingHistory", r.URL.Path)
		assert.Equal(t, "BTCUSDT", r.URL.Query().Get("symbol"))
		assert.Equal(t, "3", r.URL.Query().Get("limit"))
		
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(goldenData)
	}))
	defer server.Close()
	
	// Create provider with test server URL
	provider := &BinanceProvider{
		name:       "binance",
		baseURL:    server.URL,
		futuresURL: server.URL,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
		rateLimiter: NewRateLimiter(1200, 20),
	}
	
	// Test the request
	ctx := context.Background()
	req := &FundingRequest{
		Symbol: "BTCUSDT",
		Limit:  3,
	}
	
	resp, err := provider.GetFundingHistory(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, resp)
	
	// Verify response structure
	assert.Equal(t, 3, len(resp.Data))
	assert.Equal(t, "binance", resp.Provenance.Venue)
	assert.Equal(t, "/fapi/v1/fundingHistory", resp.Provenance.Endpoint)
	assert.Equal(t, 3, resp.Provenance.Window)
	assert.Greater(t, resp.Provenance.LatencyMs, 0)
	
	// Verify first funding rate
	funding := resp.Data[0]
	assert.Equal(t, "BTCUSDT", funding.Symbol)
	assert.Equal(t, 0.0001, funding.Rate)
	assert.Equal(t, 43500.5, funding.MarkPrice)
	assert.Equal(t, time.Unix(1693958400, 0), funding.Timestamp)
}

// TestBinanceProvider_GetSpotTrades_GoldenFile tests Binance spot trades with recorded fixtures
func TestBinanceProvider_GetSpotTrades_GoldenFile(t *testing.T) {
	// Load golden file
	goldenData, err := os.ReadFile("testdata/binance_spot_trades.json")
	require.NoError(t, err)
	
	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v3/trades", r.URL.Path)
		assert.Equal(t, "BTCUSDT", r.URL.Query().Get("symbol"))
		assert.Equal(t, "3", r.URL.Query().Get("limit"))
		
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(goldenData)
	}))
	defer server.Close()
	
	// Create provider with test server URL
	provider := &BinanceProvider{
		name:    "binance",
		baseURL: server.URL,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
		rateLimiter: NewRateLimiter(1200, 20),
	}
	
	// Test the request
	ctx := context.Background()
	req := &SpotTradesRequest{
		Symbol: "BTCUSDT",
		Limit:  3,
	}
	
	resp, err := provider.GetSpotTrades(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, resp)
	
	// Verify response structure
	assert.Equal(t, 3, len(resp.Data))
	assert.Equal(t, "binance", resp.Provenance.Venue)
	assert.Equal(t, "/api/v3/trades", resp.Provenance.Endpoint)
	
	// Verify first trade
	trade := resp.Data[0]
	assert.Equal(t, "BTCUSDT", trade.Symbol)
	assert.Equal(t, 43500.5, trade.Price)
	assert.Equal(t, 0.12345678, trade.Volume)
	assert.Equal(t, "sell", trade.Side) // isBuyerMaker: true means it's a sell
	assert.Equal(t, "28457", trade.TradeID)
	assert.Equal(t, time.Unix(1693958400, 0), trade.Timestamp)
}

// TestOKXProvider_GetFundingHistory_GoldenFile tests OKX funding history with recorded fixtures
func TestOKXProvider_GetFundingHistory_GoldenFile(t *testing.T) {
	// Load golden file
	goldenData, err := os.ReadFile("testdata/okx_funding_history.json")
	require.NoError(t, err)
	
	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v5/public/funding-history", r.URL.Path)
		assert.Equal(t, "BTC-USDT-SWAP", r.URL.Query().Get("instId"))
		assert.Equal(t, "3", r.URL.Query().Get("limit"))
		
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(goldenData)
	}))
	defer server.Close()
	
	// Create provider with test server URL
	provider := &OKXProvider{
		name:    "okx",
		baseURL: server.URL,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
		rateLimiter: NewRateLimiter(60, 10),
	}
	
	// Test the request
	ctx := context.Background()
	req := &FundingRequest{
		Symbol: "BTCUSDT",
		Limit:  3,
	}
	
	resp, err := provider.GetFundingHistory(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, resp)
	
	// Verify response structure
	assert.Equal(t, 3, len(resp.Data))
	assert.Equal(t, "okx", resp.Provenance.Venue)
	assert.Equal(t, "/api/v5/public/funding-history", resp.Provenance.Endpoint)
	
	// Verify first funding rate
	funding := resp.Data[0]
	assert.Equal(t, "BTCUSDT", funding.Symbol) // Converted from BTC-USDT-SWAP
	assert.Equal(t, 0.0001, funding.Rate)
	assert.Equal(t, 43500.5, funding.MarkPrice)
	assert.Equal(t, time.Unix(1693958400, 0), funding.Timestamp)
}

// TestCoingeckoProvider_GetSupplyReserves_GoldenFile tests CoinGecko supply data with recorded fixtures
func TestCoingeckoProvider_GetSupplyReserves_GoldenFile(t *testing.T) {
	// Load golden file
	goldenData, err := os.ReadFile("testdata/coingecko_bitcoin.json")
	require.NoError(t, err)
	
	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v3/coins/bitcoin", r.URL.Path)
		assert.Equal(t, "false", r.URL.Query().Get("localization"))
		assert.Equal(t, "true", r.URL.Query().Get("market_data"))
		
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(goldenData)
	}))
	defer server.Close()
	
	// Create provider with test server URL
	provider := &CoingeckoProvider{
		name:    "coingecko",
		baseURL: server.URL + "/api/v3",
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
		rateLimiter: NewRateLimiter(60, 1),
	}
	
	// Test the request
	ctx := context.Background()
	req := &SupplyRequest{
		Symbol: "BTC",
	}
	
	resp, err := provider.GetSupplyReserves(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, resp)
	
	// Verify response structure
	assert.Equal(t, "coingecko", resp.Provenance.Venue)
	assert.Equal(t, "/coins/bitcoin", resp.Provenance.Endpoint)
	
	// Verify supply data
	supply := resp.Data
	assert.Equal(t, "BTC", supply.Symbol)
	assert.Equal(t, 19750000.0, supply.CirculatingSupply)
	assert.Equal(t, 19750000.0, supply.TotalSupply)
	assert.Equal(t, 21000000.0, supply.MaxSupply)
}

// TestProviderRegistry_ProbeCapabilities_Integration tests the full provider registry
func TestProviderRegistry_ProbeCapabilities_Integration(t *testing.T) {
	registry := NewProviderRegistry()
	
	// Add test providers
	mockProviders := []Provider{
		&MockProvider{
			name: "test-binance",
			capabilities: map[Capability]bool{
				CapabilityFunding:    true,
				CapabilitySpotTrades: true,
				CapabilityOrderBookL2: true,
				CapabilityKlineData: true,
			},
			probeLatency: 100 * time.Millisecond,
		},
		&MockProvider{
			name: "test-coingecko",
			capabilities: map[Capability]bool{
				CapabilitySupplyReserves: true,
			},
			probeLatency: 200 * time.Millisecond,
		},
	}
	
	for _, provider := range mockProviders {
		err := registry.RegisterProvider(provider)
		require.NoError(t, err)
	}
	
	// Probe capabilities
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	report, err := registry.ProbeCapabilities(ctx)
	require.NoError(t, err)
	require.NotNil(t, report)
	
	// Verify report structure
	assert.Equal(t, 2, len(report.Providers))
	
	// Verify each provider report
	for _, providerReport := range report.Providers {
		assert.NotEmpty(t, providerReport.Name)
		assert.NotEmpty(t, providerReport.Capabilities)
		
		// Count supported capabilities
		supportedCount := 0
		for _, status := range providerReport.Capabilities {
			if status.Supported && status.Available {
				supportedCount++
				assert.Greater(t, status.LatencyMs, 0)
			}
		}
		
		if providerReport.Name == "test-binance" {
			assert.Equal(t, 4, supportedCount)
		} else if providerReport.Name == "test-coingecko" {
			assert.Equal(t, 1, supportedCount)
		}
	}
}

// TestCapabilityReport_JSON tests that capability reports can be marshaled to JSON
func TestCapabilityReport_JSON(t *testing.T) {
	report := &CapabilityReport{
		Timestamp: time.Now(),
		Providers: []ProviderReport{
			{
				Name: "test-provider",
				Capabilities: map[string]CapabilityStatus{
					"funding": {
						Supported: true,
						Available: true,
						LatencyMs: 150,
					},
					"spot_trades": {
						Supported: true,
						Available: false,
						LatencyMs: 0,
						Error:     "connection timeout",
					},
				},
			},
		},
	}
	
	// Marshal to JSON
	data, err := json.MarshalIndent(report, "", "  ")
	require.NoError(t, err)
	
	// Verify JSON contains expected fields
	jsonStr := string(data)
	assert.Contains(t, jsonStr, "test-provider")
	assert.Contains(t, jsonStr, "funding")
	assert.Contains(t, jsonStr, "spot_trades")
	assert.Contains(t, jsonStr, "connection timeout")
	assert.Contains(t, jsonStr, "150")
	
	// Verify we can unmarshal back
	var unmarshaled CapabilityReport
	err = json.Unmarshal(data, &unmarshaled)
	require.NoError(t, err)
	assert.Equal(t, report.Providers[0].Name, unmarshaled.Providers[0].Name)
}