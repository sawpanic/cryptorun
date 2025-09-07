package adapters

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	microadapters "github.com/sawpanic/cryptorun/internal/microstructure/adapters"
	"github.com/sawpanic/cryptorun/internal/providers/guards"
)

// CoinGeckoAdapter wraps CoinGecko API calls with provider guards
type CoinGeckoAdapter struct {
	guard      *guards.ProviderGuard
	baseURL    string
	httpClient *http.Client
}

// NewCoinGeckoAdapter creates a new CoinGecko adapter with guards
func NewCoinGeckoAdapter(config guards.ProviderConfig) *CoinGeckoAdapter {
	if config.Name == "" {
		config.Name = "coingecko"
	}

	return &CoinGeckoAdapter{
		guard:   guards.NewProviderGuard(config),
		baseURL: "https://api.coingecko.com/api/v3",
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// GetPrices fetches current prices for specified coins
// WARNING: This method is BANNED for microstructure data per v3.2.1 constraints
func (c *CoinGeckoAdapter) GetPrices(ctx context.Context, coins []string, vsCurrencies []string) (*PriceResponse, error) {
	// COMPILE-TIME AGGREGATOR BAN ENFORCEMENT
	if err := microadapters.GuardAgainstAggregator("coingecko"); err != nil {
		return nil, fmt.Errorf("AGGREGATOR BAN: %w", err)
	}
	params := fmt.Sprintf("ids=%s&vs_currencies=%s",
		joinStrings(coins, ","),
		joinStrings(vsCurrencies, ","))

	req := guards.GuardedRequest{
		Method:   "GET",
		URL:      fmt.Sprintf("%s/simple/price?%s", c.baseURL, params),
		Headers:  map[string]string{"Accept": "application/json"},
		CacheKey: c.guard.Cache().GenerateCacheKey("GET", fmt.Sprintf("/simple/price?%s", params), nil, nil),
	}

	resp, err := c.guard.Execute(ctx, req, c.httpFetcher)
	if err != nil {
		return nil, err
	}

	var priceResp PriceResponse
	if err := json.Unmarshal(resp.Data, &priceResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal price response: %w", err)
	}

	return &priceResp, nil
}

// GetMarketData fetches market data for specified coins
// WARNING: This method is BANNED for microstructure data per v3.2.1 constraints
func (c *CoinGeckoAdapter) GetMarketData(ctx context.Context, vsCurrency string, limit int) (*MarketDataResponse, error) {
	// COMPILE-TIME AGGREGATOR BAN ENFORCEMENT
	if err := microadapters.GuardAgainstAggregator("coingecko"); err != nil {
		return nil, fmt.Errorf("AGGREGATOR BAN: %w", err)
	}
	params := fmt.Sprintf("vs_currency=%s&order=market_cap_desc&per_page=%d&page=1", vsCurrency, limit)

	req := guards.GuardedRequest{
		Method:   "GET",
		URL:      fmt.Sprintf("%s/coins/markets?%s", c.baseURL, params),
		Headers:  map[string]string{"Accept": "application/json"},
		CacheKey: c.guard.Cache().GenerateCacheKey("GET", fmt.Sprintf("/coins/markets?%s", params), nil, nil),
	}

	resp, err := c.guard.Execute(ctx, req, c.httpFetcher)
	if err != nil {
		return nil, err
	}

	var marketResp MarketDataResponse
	if err := json.Unmarshal(resp.Data, &marketResp.Data); err != nil {
		return nil, fmt.Errorf("failed to unmarshal market data response: %w", err)
	}

	marketResp.Cached = resp.Cached
	marketResp.Age = resp.Age

	return &marketResp, nil
}

// GetGlobalData fetches global cryptocurrency statistics
func (c *CoinGeckoAdapter) GetGlobalData(ctx context.Context) (*GlobalDataResponse, error) {
	req := guards.GuardedRequest{
		Method:   "GET",
		URL:      fmt.Sprintf("%s/global", c.baseURL),
		Headers:  map[string]string{"Accept": "application/json"},
		CacheKey: c.guard.Cache().GenerateCacheKey("GET", "/global", nil, nil),
	}

	resp, err := c.guard.Execute(ctx, req, c.httpFetcher)
	if err != nil {
		return nil, err
	}

	var globalResp GlobalDataResponse
	if err := json.Unmarshal(resp.Data, &globalResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal global data response: %w", err)
	}

	return &globalResp, nil
}

// httpFetcher performs the actual HTTP request
func (c *CoinGeckoAdapter) httpFetcher(ctx context.Context, req guards.GuardedRequest) (*guards.GuardedResponse, error) {
	httpReq, err := http.NewRequestWithContext(ctx, req.Method, req.URL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}

	// Add headers
	for key, value := range req.Headers {
		httpReq.Header.Set(key, value)
	}

	// Add point-in-time headers if available
	c.guard.Cache().AddPITHeaders(req.CacheKey, req.Headers)
	for key, value := range req.Headers {
		httpReq.Header.Set(key, value)
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	return &guards.GuardedResponse{
		Data:       body,
		StatusCode: resp.StatusCode,
		Headers:    resp.Header,
		Cached:     false,
	}, nil
}

// Health returns the health status of the CoinGecko provider
func (c *CoinGeckoAdapter) Health() guards.ProviderHealth {
	return c.guard.Health()
}

// Response types for CoinGecko API

type PriceResponse map[string]map[string]float64

type MarketDataResponse struct {
	Data   []MarketData  `json:"data"`
	Cached bool          `json:"cached"`
	Age    time.Duration `json:"age"`
}

type MarketData struct {
	ID                    string  `json:"id"`
	Symbol                string  `json:"symbol"`
	Name                  string  `json:"name"`
	CurrentPrice          float64 `json:"current_price"`
	MarketCap             int64   `json:"market_cap"`
	MarketCapRank         int     `json:"market_cap_rank"`
	TotalVolume           int64   `json:"total_volume"`
	PriceChangePercent24h float64 `json:"price_change_percentage_24h"`
	CirculatingSupply     float64 `json:"circulating_supply"`
	TotalSupply           float64 `json:"total_supply,omitempty"`
	MaxSupply             float64 `json:"max_supply,omitempty"`
}

type GlobalDataResponse struct {
	Data struct {
		ActiveCryptocurrencies    int                `json:"active_cryptocurrencies"`
		MarketCapPercents         map[string]float64 `json:"market_cap_percentage"`
		TotalMarketCap            map[string]int64   `json:"total_market_cap"`
		TotalVolume               map[string]int64   `json:"total_volume_24h"`
		MarketCapChangePercent24h float64            `json:"market_cap_change_percentage_24h_usd"`
	} `json:"data"`
}

// Utility function to join strings
func joinStrings(slice []string, sep string) string {
	if len(slice) == 0 {
		return ""
	}
	if len(slice) == 1 {
		return slice[0]
	}

	result := slice[0]
	for i := 1; i < len(slice); i++ {
		result += sep + slice[i]
	}
	return result
}
