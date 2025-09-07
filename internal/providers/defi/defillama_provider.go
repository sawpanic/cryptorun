package defi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
	
	"github.com/sawpanic/cryptorun/internal/providers"
	"github.com/sawpanic/cryptorun/internal/providers/derivs"
)

// DeFiLlamaProvider implements DeFi metrics using DeFiLlama API (free tier)
type DeFiLlamaProvider struct {
	config      DeFiProviderConfig
	client      *http.Client
	rateLimiter derivs.RateLimiter
	guard       *providers.ExchangeNativeGuard
}

// NewDeFiLlamaProvider creates a new DeFiLlama provider
func NewDeFiLlamaProvider(config DeFiProviderConfig) (*DeFiLlamaProvider, error) {
	if config.BaseURL == "" {
		config.BaseURL = "https://api.llama.fi"
	}
	
	if config.RequestTimeout == 0 {
		config.RequestTimeout = 30 * time.Second
	}
	
	if config.RateLimitRPS == 0 {
		config.RateLimitRPS = 3.0 // Conservative free tier limit
	}
	
	if config.UserAgent == "" {
		config.UserAgent = "CryptoRun/1.0 (DeFi-metrics)"
	}
	
	// Enforce USD pairs only constraint
	config.USDPairsOnly = true
	
	client := &http.Client{
		Timeout: config.RequestTimeout,
	}
	
	rateLimiter := derivs.NewTokenBucketLimiter(config.RateLimitRPS)
	
	return &DeFiLlamaProvider{
		config:      config,
		client:      client,
		rateLimiter: rateLimiter,
		guard:       providers.NewExchangeNativeGuard(),
	}, nil
}

// GetProtocolTVL retrieves TVL metrics for a protocol/token
func (p *DeFiLlamaProvider) GetProtocolTVL(ctx context.Context, protocol string, tokenSymbol string) (*DeFiMetrics, error) {
	// Enforce USD pairs only
	if !isUSDToken(tokenSymbol) {
		return nil, fmt.Errorf("non-USD token not allowed: %s - USD pairs only", tokenSymbol)
	}
	
	// Apply rate limiting
	if err := p.rateLimiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("rate limit exceeded: %w", err)
	}
	
	// Get protocol TVL data
	endpoint := fmt.Sprintf("/protocol/%s", protocol)
	resp, err := p.makeRequest(ctx, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch protocol TVL: %w", err)
	}
	
	// Parse response
	metrics, err := p.parseProtocolTVLResponse(resp, protocol, tokenSymbol)
	if err != nil {
		return nil, fmt.Errorf("failed to parse protocol TVL response: %w", err)
	}
	
	// Apply PIT shift if configured
	if p.config.PITShiftPeriods > 0 {
		metrics.PITShift = p.config.PITShiftPeriods
		metrics.Timestamp = metrics.Timestamp.Add(-time.Duration(p.config.PITShiftPeriods) * time.Hour)
	}
	
	return metrics, nil
}

// GetPoolMetrics retrieves AMM pool metrics for a token pair
func (p *DeFiLlamaProvider) GetPoolMetrics(ctx context.Context, protocol string, tokenA, tokenB string) (*DeFiMetrics, error) {
	// Enforce USD pairs only - at least one token must be USD
	if !isUSDToken(tokenA) && !isUSDToken(tokenB) {
		return nil, fmt.Errorf("non-USD pair not allowed: %s/%s - USD pairs only", tokenA, tokenB)
	}
	
	// Apply rate limiting
	if err := p.rateLimiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("rate limit exceeded: %w", err)
	}
	
	// DeFiLlama doesn't have direct pool metrics, so we use protocol TVL as proxy
	tokenSymbol := tokenA
	if isUSDToken(tokenB) {
		tokenSymbol = tokenB
	}
	
	return p.GetProtocolTVL(ctx, protocol, tokenSymbol)
}

// GetLendingMetrics retrieves lending protocol metrics
func (p *DeFiLlamaProvider) GetLendingMetrics(ctx context.Context, protocol string, tokenSymbol string) (*DeFiMetrics, error) {
	// Enforce USD pairs only
	if !isUSDToken(tokenSymbol) {
		return nil, fmt.Errorf("non-USD token not allowed: %s - USD pairs only", tokenSymbol)
	}
	
	// Apply rate limiting
	if err := p.rateLimiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("rate limit exceeded: %w", err)
	}
	
	// Get lending protocol data
	return p.GetProtocolTVL(ctx, protocol, tokenSymbol)
}

// GetTopTVLTokens returns tokens by TVL (USD pairs only)
func (p *DeFiLlamaProvider) GetTopTVLTokens(ctx context.Context, limit int) ([]DeFiMetrics, error) {
	// Apply rate limiting
	if err := p.rateLimiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("rate limit exceeded: %w", err)
	}
	
	// Get all protocols and filter by TVL
	endpoint := "/protocols"
	resp, err := p.makeRequest(ctx, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch protocols: %w", err)
	}
	
	// Parse and filter protocols
	metrics, err := p.parseTopProtocolsResponse(resp, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to parse protocols response: %w", err)
	}
	
	return metrics, nil
}

// Health returns provider health and connectivity status
func (p *DeFiLlamaProvider) Health(ctx context.Context) (*ProviderHealth, error) {
	start := time.Now()
	
	// Test connectivity with protocols endpoint
	endpoint := "/protocols"
	_, err := p.makeRequest(ctx, endpoint, nil)
	
	latency := time.Since(start).Seconds() * 1000
	
	health := &ProviderHealth{
		Healthy:            err == nil,
		DataSource:         "defillama",
		LastUpdate:         time.Now(),
		LatencyMS:          latency,
		ErrorRate:          0.0, // Would track over time in production
		SupportedProtocols: len(p.getSupportedProtocols()),
		DataFreshness:      make(map[string]time.Duration),
	}
	
	if err != nil {
		health.Errors = []string{err.Error()}
	}
	
	return health, nil
}

// GetSupportedProtocols returns list of supported DeFi protocols
func (p *DeFiLlamaProvider) GetSupportedProtocols(ctx context.Context) ([]string, error) {
	return p.getSupportedProtocols(), nil
}

// Private helper methods

func (p *DeFiLlamaProvider) getSupportedProtocols() []string {
	return []string{
		"uniswap",
		"aave",
		"compound",
		"makerdao",
		"curve",
		"sushiswap",
		"balancer",
		"yearn-finance",
		"convex-finance",
		"pancakeswap",
	}
}

func (p *DeFiLlamaProvider) makeRequest(ctx context.Context, endpoint string, params map[string]string) (map[string]interface{}, error) {
	// Build URL with parameters
	reqURL := p.config.BaseURL + endpoint
	if len(params) > 0 {
		values := url.Values{}
		for k, v := range params {
			values.Set(k, v)
		}
		reqURL += "?" + values.Encode()
	}
	
	// Create request
	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	
	req.Header.Set("User-Agent", p.config.UserAgent)
	req.Header.Set("Accept", "application/json")
	
	// Execute request
	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}
	
	// Parse response
	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	
	return result, nil
}

func (p *DeFiLlamaProvider) parseProtocolTVLResponse(resp map[string]interface{}, protocol string, tokenSymbol string) (*DeFiMetrics, error) {
	metrics := &DeFiMetrics{
		Timestamp:       time.Now(),
		Protocol:        protocol,
		TokenSymbol:     tokenSymbol,
		DataSource:      "defillama",
		ConfidenceScore: 0.85, // Good confidence for DeFiLlama data
	}
	
	// Parse TVL from response
	if tvl, ok := resp["tvl"].(float64); ok {
		metrics.TVL = tvl
	}
	
	// Parse change data if available
	if change, ok := resp["change_1d"].(float64); ok {
		metrics.TVLChange24h = change
	}
	
	if change, ok := resp["change_7d"].(float64); ok {
		metrics.TVLChange7d = change
	}
	
	// Parse chain TVLs if available for more detailed metrics
	if chainTvls, ok := resp["chainTvls"].(map[string]interface{}); ok {
		if ethTvl, ok := chainTvls["Ethereum"].(float64); ok && ethTvl > 0 {
			// Use Ethereum TVL as primary for USD token analysis
			metrics.TVL = ethTvl
		}
	}
	
	// Parse additional metadata
	if name, ok := resp["name"].(string); ok {
		// Use name to enhance protocol identification
		if strings.Contains(strings.ToLower(name), "lending") {
			// Set lending-specific fields to zero values to indicate availability
			metrics.BorrowAPY = 0.0
			metrics.SupplyAPY = 0.0
			metrics.UtilizationRate = 0.0
		}
	}
	
	return metrics, nil
}

func (p *DeFiLlamaProvider) parseTopProtocolsResponse(resp map[string]interface{}, limit int) ([]DeFiMetrics, error) {
	// DeFiLlama returns an array of protocols in the response
	protocols, ok := resp["protocols"].([]interface{})
	if !ok {
		// Try direct array format as fallback
		protocolsArr, ok := resp["data"].([]interface{})
		if !ok {
			return nil, fmt.Errorf("invalid protocols response format - expected protocols or data array")
		}
		protocols = protocolsArr
	}
	
	var metrics []DeFiMetrics
	count := 0
	
	for _, protocolData := range protocols {
		if count >= limit {
			break
		}
		
		protocol := protocolData.(map[string]interface{})
		
		// Extract protocol information
		name, ok := protocol["name"].(string)
		if !ok {
			continue
		}
		
		tvl, ok := protocol["tvl"].(float64)
		if !ok || tvl <= 0 {
			continue
		}
		
		// Create metrics for USD-focused protocols
		metric := DeFiMetrics{
			Timestamp:       time.Now(),
			Protocol:        strings.ToLower(strings.ReplaceAll(name, " ", "-")),
			TokenSymbol:     "USDT", // Default to USDT for top protocols
			TVL:             tvl,
			DataSource:      "defillama",
			ConfidenceScore: 0.85,
		}
		
		// Parse change data if available
		if change, ok := protocol["change_1d"].(float64); ok {
			metric.TVLChange24h = change
		}
		
		if change, ok := protocol["change_7d"].(float64); ok {
			metric.TVLChange7d = change
		}
		
		// Parse category to identify lending protocols
		if category, ok := protocol["category"].(string); ok {
			if strings.Contains(strings.ToLower(category), "lending") {
				metric.BorrowAPY = 0.0
				metric.SupplyAPY = 0.0
				metric.UtilizationRate = 0.0
			}
		}
		
		metrics = append(metrics, metric)
		count++
	}
	
	return metrics, nil
}

// Protocol name mapping for DeFiLlama API
func (p *DeFiLlamaProvider) mapProtocolName(protocol string) string {
	mapping := map[string]string{
		"uniswap-v2": "uniswap",
		"uniswap-v3": "uniswap",
		"aave-v2":    "aave",
		"aave-v3":    "aave",
		"compound-v2": "compound",
	}
	
	if mapped, ok := mapping[protocol]; ok {
		return mapped
	}
	
	return protocol
}

// Helper function for USD token validation (shared with thegraph_provider.go)
func isUSDTokenLlama(symbol string) bool {
	return isUSDToken(symbol) // Reuse from thegraph_provider.go
}