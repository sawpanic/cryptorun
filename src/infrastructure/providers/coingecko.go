package providers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// CoingeckoProvider implements the Provider interface for CoinGecko
type CoingeckoProvider struct {
	name        string
	baseURL     string
	client      *http.Client
	rateLimiter *RateLimiter
}

// NewCoingeckoProvider creates a new CoinGecko provider with free/keyless endpoints
func NewCoingeckoProvider() *CoingeckoProvider {
	return &CoingeckoProvider{
		name:    "coingecko",
		baseURL: "https://api.coingecko.com/api/v3",
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
		rateLimiter: NewRateLimiter(60, 1), // 10-50 calls per minute for free tier
	}
}

func (c *CoingeckoProvider) Name() string {
	return c.name
}

func (c *CoingeckoProvider) HasCapability(cap Capability) bool {
	switch cap {
	case CapabilitySupplyReserves: // CoinGecko provides supply data
		return true
	case CapabilityFunding, CapabilitySpotTrades, CapabilityOrderBookL2, CapabilityKlineData:
		return false // Not available via CoinGecko
	case CapabilityWhaleDetection, CapabilityCVD:
		return false // Not available via free APIs
	}
	return false
}

func (c *CoingeckoProvider) Probe(ctx context.Context) (*ProbeResult, error) {
	start := time.Now()
	
	// Use ping endpoint as a lightweight health check
	endpoint := "/ping"
	url := c.baseURL + endpoint
	
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return &ProbeResult{
			Success:   false,
			Error:     err.Error(),
			Timestamp: time.Now(),
		}, nil
	}
	
	resp, err := c.client.Do(req)
	if err != nil {
		return &ProbeResult{
			Success:   false,
			Error:     err.Error(),
			LatencyMs: int(time.Since(start).Milliseconds()),
			Timestamp: time.Now(),
		}, nil
	}
	defer resp.Body.Close()
	
	success := resp.StatusCode == http.StatusOK
	errorMsg := ""
	if !success {
		errorMsg = fmt.Sprintf("HTTP %d", resp.StatusCode)
	}
	
	return &ProbeResult{
		Success:   success,
		Error:     errorMsg,
		LatencyMs: int(time.Since(start).Milliseconds()),
		Timestamp: time.Now(),
	}, nil
}

func (c *CoingeckoProvider) GetSupplyReserves(ctx context.Context, req *SupplyRequest) (*SupplyResponse, error) {
	start := time.Now()
	
	if !c.rateLimiter.Allow() {
		return nil, fmt.Errorf("rate limit exceeded for %s", c.name)
	}
	
	// Convert symbol to CoinGecko ID (simplified mapping)
	coinId := c.convertSymbolToCoinGeckoId(req.Symbol)
	
	endpoint := "/coins/" + coinId
	params := url.Values{}
	params.Set("localization", "false")
	params.Set("tickers", "false")
	params.Set("market_data", "true")
	params.Set("community_data", "false")
	params.Set("developer_data", "false")
	params.Set("sparkline", "false")
	
	fullURL := fmt.Sprintf("%s%s?%s", c.baseURL, endpoint, params.Encode())
	
	httpReq, err := http.NewRequestWithContext(ctx, "GET", fullURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	
	resp, err := c.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API error: status %d", resp.StatusCode)
	}
	
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}
	
	var result struct {
		Id         string `json:"id"`
		Symbol     string `json:"symbol"`
		Name       string `json:"name"`
		MarketData struct {
			CirculatingSupply float64 `json:"circulating_supply"`
			TotalSupply       float64 `json:"total_supply"`
			MaxSupply         float64 `json:"max_supply"`
		} `json:"market_data"`
	}
	
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}
	
	return &SupplyResponse{
		Data: &SupplyData{
			Symbol:            strings.ToUpper(result.Symbol),
			CirculatingSupply: result.MarketData.CirculatingSupply,
			TotalSupply:       result.MarketData.TotalSupply,
			MaxSupply:         result.MarketData.MaxSupply,
			Timestamp:         time.Now(),
		},
		Provenance: Provenance{
			Venue:     c.name,
			Endpoint:  endpoint,
			LatencyMs: int(time.Since(start).Milliseconds()),
			Timestamp: time.Now(),
		},
	}, nil
}

// Helper function to convert symbols to CoinGecko IDs
func (c *CoingeckoProvider) convertSymbolToCoinGeckoId(symbol string) string {
	// Basic mapping - this could be expanded with a full mapping table
	switch strings.ToUpper(symbol) {
	case "BTC", "BITCOIN":
		return "bitcoin"
	case "ETH", "ETHEREUM":
		return "ethereum"
	case "ADA", "CARDANO":
		return "cardano"
	case "DOT", "POLKADOT":
		return "polkadot"
	case "LINK", "CHAINLINK":
		return "chainlink"
	case "LTC", "LITECOIN":
		return "litecoin"
	case "XRP", "RIPPLE":
		return "ripple"
	case "SOL", "SOLANA":
		return "solana"
	case "AVAX", "AVALANCHE":
		return "avalanche-2"
	case "MATIC", "POLYGON":
		return "matic-network"
	default:
		// Fallback: try lowercase symbol as ID
		return strings.ToLower(symbol)
	}
}

// CoinGecko doesn't support these capabilities
func (c *CoingeckoProvider) GetFundingHistory(ctx context.Context, req *FundingRequest) (*FundingResponse, error) {
	return nil, ErrCapabilityNotSupported
}

func (c *CoingeckoProvider) GetSpotTrades(ctx context.Context, req *SpotTradesRequest) (*SpotTradesResponse, error) {
	return nil, ErrCapabilityNotSupported
}

func (c *CoingeckoProvider) GetOrderBookL2(ctx context.Context, req *OrderBookRequest) (*OrderBookResponse, error) {
	return nil, ErrCapabilityNotSupported
}

func (c *CoingeckoProvider) GetKlineData(ctx context.Context, req *KlineRequest) (*KlineResponse, error) {
	return nil, ErrCapabilityNotSupported
}

func (c *CoingeckoProvider) GetWhaleDetection(ctx context.Context, req *WhaleRequest) (*WhaleResponse, error) {
	return nil, ErrCapabilityNotSupported
}

func (c *CoingeckoProvider) GetCVD(ctx context.Context, req *CVDRequest) (*CVDResponse, error) {
	return nil, ErrCapabilityNotSupported
}