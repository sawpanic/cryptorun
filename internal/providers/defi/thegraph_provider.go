package defi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
	
	"github.com/sawpanic/cryptorun/internal/providers"
)

// TheGraphProvider implements DeFi metrics using The Graph Protocol (free tier)
type TheGraphProvider struct {
	config     DeFiProviderConfig
	client     *http.Client
	rateLimiter providers.RateLimiter
	guard      *providers.ExchangeNativeGuard
}

// NewTheGraphProvider creates a new The Graph provider
func NewTheGraphProvider(config DeFiProviderConfig) (*TheGraphProvider, error) {
	if config.BaseURL == "" {
		config.BaseURL = "https://api.thegraph.com/subgraphs/name"
	}
	
	if config.RequestTimeout == 0 {
		config.RequestTimeout = 30 * time.Second
	}
	
	if config.RateLimitRPS == 0 {
		config.RateLimitRPS = 5.0 // Conservative free tier limit
	}
	
	if config.UserAgent == "" {
		config.UserAgent = "CryptoRun/1.0 (DeFi-metrics)"
	}
	
	// Enforce USD pairs only constraint
	config.USDPairsOnly = true
	
	client := &http.Client{
		Timeout: config.RequestTimeout,
	}
	
	rateLimiter, err := providers.NewTokenBucketLimiter(config.RateLimitRPS, 10)
	if err != nil {
		return nil, fmt.Errorf("failed to create rate limiter: %w", err)
	}
	
	return &TheGraphProvider{
		config:      config,
		client:      client,
		rateLimiter: rateLimiter,
		guard:       providers.NewExchangeNativeGuard(),
	}, nil
}

// GetProtocolTVL retrieves TVL metrics for a protocol/token
func (p *TheGraphProvider) GetProtocolTVL(ctx context.Context, protocol string, tokenSymbol string) (*DeFiMetrics, error) {
	// Enforce USD pairs only
	if !isUSDToken(tokenSymbol) {
		return nil, fmt.Errorf("non-USD token not allowed: %s - USD pairs only", tokenSymbol)
	}
	
	// Apply rate limiting
	if err := p.rateLimiter.Allow(); err != nil {
		return nil, fmt.Errorf("rate limit exceeded: %w", err)
	}
	
	// Build GraphQL query based on protocol
	query, err := p.buildTVLQuery(protocol, tokenSymbol)
	if err != nil {
		return nil, fmt.Errorf("failed to build TVL query: %w", err)
	}
	
	// Execute GraphQL request
	resp, err := p.executeGraphQLQuery(ctx, protocol, query)
	if err != nil {
		return nil, fmt.Errorf("GraphQL query failed: %w", err)
	}
	
	// Parse response and build metrics
	metrics, err := p.parseTVLResponse(resp, protocol, tokenSymbol)
	if err != nil {
		return nil, fmt.Errorf("failed to parse TVL response: %w", err)
	}
	
	// Apply PIT shift if configured
	if p.config.PITShiftPeriods > 0 {
		metrics.PITShift = p.config.PITShiftPeriods
		metrics.Timestamp = metrics.Timestamp.Add(-time.Duration(p.config.PITShiftPeriods) * time.Hour)
	}
	
	return metrics, nil
}

// GetPoolMetrics retrieves AMM pool metrics for a token pair
func (p *TheGraphProvider) GetPoolMetrics(ctx context.Context, protocol string, tokenA, tokenB string) (*DeFiMetrics, error) {
	// Enforce USD pairs only - at least one token must be USD
	if !isUSDToken(tokenA) && !isUSDToken(tokenB) {
		return nil, fmt.Errorf("non-USD pair not allowed: %s/%s - USD pairs only", tokenA, tokenB)
	}
	
	// Apply rate limiting
	if err := p.rateLimiter.Allow(); err != nil {
		return nil, fmt.Errorf("rate limit exceeded: %w", err)
	}
	
	// Build pool metrics query
	query, err := p.buildPoolQuery(protocol, tokenA, tokenB)
	if err != nil {
		return nil, fmt.Errorf("failed to build pool query: %w", err)
	}
	
	// Execute GraphQL request
	resp, err := p.executeGraphQLQuery(ctx, protocol, query)
	if err != nil {
		return nil, fmt.Errorf("GraphQL query failed: %w", err)
	}
	
	// Parse response
	tokenSymbol := tokenA
	if isUSDToken(tokenB) {
		tokenSymbol = tokenB
	}
	
	metrics, err := p.parsePoolResponse(resp, protocol, tokenSymbol)
	if err != nil {
		return nil, fmt.Errorf("failed to parse pool response: %w", err)
	}
	
	return metrics, nil
}

// GetLendingMetrics retrieves lending protocol metrics
func (p *TheGraphProvider) GetLendingMetrics(ctx context.Context, protocol string, tokenSymbol string) (*DeFiMetrics, error) {
	// Enforce USD pairs only
	if !isUSDToken(tokenSymbol) {
		return nil, fmt.Errorf("non-USD token not allowed: %s - USD pairs only", tokenSymbol)
	}
	
	// Apply rate limiting
	if err := p.rateLimiter.Allow(); err != nil {
		return nil, fmt.Errorf("rate limit exceeded: %w", err)
	}
	
	// Build lending query
	query, err := p.buildLendingQuery(protocol, tokenSymbol)
	if err != nil {
		return nil, fmt.Errorf("failed to build lending query: %w", err)
	}
	
	// Execute GraphQL request
	resp, err := p.executeGraphQLQuery(ctx, protocol, query)
	if err != nil {
		return nil, fmt.Errorf("GraphQL query failed: %w", err)
	}
	
	// Parse response
	metrics, err := p.parseLendingResponse(resp, protocol, tokenSymbol)
	if err != nil {
		return nil, fmt.Errorf("failed to parse lending response: %w", err)
	}
	
	return metrics, nil
}

// GetTopTVLTokens returns tokens by TVL (USD pairs only)
func (p *TheGraphProvider) GetTopTVLTokens(ctx context.Context, limit int) ([]DeFiMetrics, error) {
	// Apply rate limiting
	if err := p.rateLimiter.Allow(); err != nil {
		return nil, fmt.Errorf("rate limit exceeded: %w", err)
	}
	
	// Build top TVL query
	query := fmt.Sprintf(`{
		tokens(
			first: %d,
			orderBy: totalValueLockedUSD,
			orderDirection: desc,
			where: { symbol_contains: "USD" }
		) {
			id
			symbol
			name
			totalValueLockedUSD
			volume24USD
		}
	}`, limit)
	
	// Execute against Uniswap V3 subgraph (most comprehensive)
	resp, err := p.executeGraphQLQuery(ctx, "uniswap-v3", query)
	if err != nil {
		return nil, fmt.Errorf("GraphQL query failed: %w", err)
	}
	
	// Parse response
	metrics, err := p.parseTopTokensResponse(resp)
	if err != nil {
		return nil, fmt.Errorf("failed to parse top tokens response: %w", err)
	}
	
	return metrics, nil
}

// Health returns provider health and connectivity status
func (p *TheGraphProvider) Health(ctx context.Context) (*ProviderHealth, error) {
	start := time.Now()
	
	// Test connectivity with a simple query
	query := `{ _meta { block { number } } }`
	_, err := p.executeGraphQLQuery(ctx, "uniswap-v3", query)
	
	latency := time.Since(start).Seconds() * 1000
	
	health := &ProviderHealth{
		Healthy:            err == nil,
		DataSource:         "thegraph",
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
func (p *TheGraphProvider) GetSupportedProtocols(ctx context.Context) ([]string, error) {
	return p.getSupportedProtocols(), nil
}

// Private helper methods

func (p *TheGraphProvider) getSupportedProtocols() []string {
	return []string{
		"uniswap-v2",
		"uniswap-v3", 
		"sushiswap",
		"aave-v2",
		"aave-v3",
		"compound-v2",
		"curve",
		"balancer-v2",
	}
}

func (p *TheGraphProvider) executeGraphQLQuery(ctx context.Context, protocol string, query string) (map[string]interface{}, error) {
	// Build subgraph URL
	subgraphURL := fmt.Sprintf("%s/%s", p.config.BaseURL, p.getSubgraphName(protocol))
	
	// Prepare GraphQL request
	payload := map[string]interface{}{
		"query": query,
	}
	
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal query: %w", err)
	}
	
	req, err := http.NewRequestWithContext(ctx, "POST", subgraphURL, strings.NewReader(string(jsonData)))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", p.config.UserAgent)
	
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
	
	// Check for GraphQL errors
	if errors, ok := result["errors"]; ok && errors != nil {
		return nil, fmt.Errorf("GraphQL errors: %v", errors)
	}
	
	return result, nil
}

func (p *TheGraphProvider) getSubgraphName(protocol string) string {
	// Map protocol names to The Graph subgraph names
	mapping := map[string]string{
		"uniswap-v2":  "uniswap/uniswap-v2",
		"uniswap-v3":  "uniswap/uniswap-v3",
		"sushiswap":   "sushiswap/exchange",
		"aave-v2":     "aave/protocol-v2",
		"aave-v3":     "aave/protocol-v3-ethereum",
		"compound-v2": "graphprotocol/compound-v2",
		"curve":       "convex-community/curve-pools",
		"balancer-v2": "balancer-labs/balancer-v2",
	}
	
	if subgraph, ok := mapping[protocol]; ok {
		return subgraph
	}
	
	return protocol // Fallback
}

func (p *TheGraphProvider) buildTVLQuery(protocol string, tokenSymbol string) (string, error) {
	// Build protocol-specific TVL queries
	switch protocol {
	case "uniswap-v2", "uniswap-v3":
		return fmt.Sprintf(`{
			tokens(where: { symbol: "%s" }) {
				id
				symbol
				name
				totalValueLockedUSD
				volume24USD
				txCount
			}
		}`, tokenSymbol), nil
		
	case "aave-v2", "aave-v3":
		return fmt.Sprintf(`{
			reserves(where: { symbol: "%s" }) {
				id
				symbol
				name
				totalLiquidityUSD
				totalBorrowsUSD
				utilizationRate
				liquidityRate
				borrowRate
			}
		}`, tokenSymbol), nil
		
	default:
		return "", fmt.Errorf("unsupported protocol for TVL query: %s", protocol)
	}
}

func (p *TheGraphProvider) buildPoolQuery(protocol string, tokenA, tokenB string) (string, error) {
	switch protocol {
	case "uniswap-v2", "uniswap-v3":
		return fmt.Sprintf(`{
			pools(where: {
				or: [
					{ token0_: { symbol: "%s" }, token1_: { symbol: "%s" } },
					{ token0_: { symbol: "%s" }, token1_: { symbol: "%s" } }
				]
			}) {
				id
				token0 { symbol }
				token1 { symbol }
				totalValueLockedUSD
				volume24USD
				feesUSD
				liquidity
			}
		}`, tokenA, tokenB, tokenB, tokenA), nil
		
	default:
		return "", fmt.Errorf("unsupported protocol for pool query: %s", protocol)
	}
}

func (p *TheGraphProvider) buildLendingQuery(protocol string, tokenSymbol string) (string, error) {
	switch protocol {
	case "aave-v2", "aave-v3":
		return fmt.Sprintf(`{
			reserves(where: { symbol: "%s" }) {
				id
				symbol
				totalLiquidityUSD
				totalBorrowsUSD
				utilizationRate
				liquidityRate
				borrowRate
				aToken { id }
			}
		}`, tokenSymbol), nil
		
	case "compound-v2":
		return fmt.Sprintf(`{
			markets(where: { underlyingSymbol: "%s" }) {
				id
				symbol
				underlyingSymbol
				totalSupplyUSD
				totalBorrowsUSD
				supplyRate
				borrowRate
				utilizationRate
			}
		}`, tokenSymbol), nil
		
	default:
		return "", fmt.Errorf("unsupported protocol for lending query: %s", protocol)
	}
}

func (p *TheGraphProvider) parseTVLResponse(resp map[string]interface{}, protocol string, tokenSymbol string) (*DeFiMetrics, error) {
	data, ok := resp["data"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid response format: missing data")
	}
	
	metrics := &DeFiMetrics{
		Timestamp:       time.Now(),
		Protocol:        protocol,
		TokenSymbol:     tokenSymbol,
		DataSource:      "thegraph",
		ConfidenceScore: 0.9, // High confidence for The Graph data
	}
	
	// Parse based on protocol
	switch protocol {
	case "uniswap-v2", "uniswap-v3":
		if err := p.parseUniswapTVL(data, metrics); err != nil {
			return nil, err
		}
	case "aave-v2", "aave-v3":
		if err := p.parseAaveTVL(data, metrics); err != nil {
			return nil, err
		}
	}
	
	return metrics, nil
}

func (p *TheGraphProvider) parseUniswapTVL(data map[string]interface{}, metrics *DeFiMetrics) error {
	tokens, ok := data["tokens"].([]interface{})
	if !ok || len(tokens) == 0 {
		return fmt.Errorf("no token data found")
	}
	
	token := tokens[0].(map[string]interface{})
	
	if tvl, ok := token["totalValueLockedUSD"].(string); ok {
		if tvlFloat, err := parseFloat64(tvl); err == nil {
			metrics.TVL = tvlFloat
		}
	}
	
	if volume, ok := token["volume24USD"].(string); ok {
		if volumeFloat, err := parseFloat64(volume); err == nil {
			metrics.PoolVolume24h = volumeFloat
		}
	}
	
	return nil
}

func (p *TheGraphProvider) parseAaveTVL(data map[string]interface{}, metrics *DeFiMetrics) error {
	reserves, ok := data["reserves"].([]interface{})
	if !ok || len(reserves) == 0 {
		return fmt.Errorf("no reserve data found")
	}
	
	reserve := reserves[0].(map[string]interface{})
	
	if liquidity, ok := reserve["totalLiquidityUSD"].(string); ok {
		if liquidityFloat, err := parseFloat64(liquidity); err == nil {
			metrics.TVL = liquidityFloat
		}
	}
	
	if supplyRate, ok := reserve["liquidityRate"].(string); ok {
		if rateFloat, err := parseFloat64(supplyRate); err == nil {
			metrics.SupplyAPY = rateFloat / 1e27 * 100 // Convert from Ray to percentage
		}
	}
	
	if borrowRate, ok := reserve["borrowRate"].(string); ok {
		if rateFloat, err := parseFloat64(borrowRate); err == nil {
			metrics.BorrowAPY = rateFloat / 1e27 * 100 // Convert from Ray to percentage
		}
	}
	
	if utilRate, ok := reserve["utilizationRate"].(string); ok {
		if utilFloat, err := parseFloat64(utilRate); err == nil {
			metrics.UtilizationRate = utilFloat
		}
	}
	
	return nil
}

func (p *TheGraphProvider) parsePoolResponse(resp map[string]interface{}, protocol string, tokenSymbol string) (*DeFiMetrics, error) {
	// Implementation similar to parseTVLResponse but for pool-specific data
	return p.parseTVLResponse(resp, protocol, tokenSymbol)
}

func (p *TheGraphProvider) parseLendingResponse(resp map[string]interface{}, protocol string, tokenSymbol string) (*DeFiMetrics, error) {
	// Implementation similar to parseTVLResponse but for lending-specific data
	return p.parseTVLResponse(resp, protocol, tokenSymbol)
}

func (p *TheGraphProvider) parseTopTokensResponse(resp map[string]interface{}) ([]DeFiMetrics, error) {
	data, ok := resp["data"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid response format: missing data")
	}
	
	tokens, ok := data["tokens"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid tokens data")
	}
	
	var metrics []DeFiMetrics
	for _, tokenData := range tokens {
		token := tokenData.(map[string]interface{})
		
		symbol, ok := token["symbol"].(string)
		if !ok || !isUSDToken(symbol) {
			continue // Skip non-USD tokens
		}
		
		metric := DeFiMetrics{
			Timestamp:       time.Now(),
			Protocol:        "uniswap-v3",
			TokenSymbol:     symbol,
			DataSource:      "thegraph",
			ConfidenceScore: 0.9,
		}
		
		if tvl, ok := token["totalValueLockedUSD"].(string); ok {
			if tvlFloat, err := parseFloat64(tvl); err == nil {
				metric.TVL = tvlFloat
			}
		}
		
		if volume, ok := token["volume24USD"].(string); ok {
			if volumeFloat, err := parseFloat64(volume); err == nil {
				metric.PoolVolume24h = volumeFloat
			}
		}
		
		metrics = append(metrics, metric)
	}
	
	return metrics, nil
}

// Helper functions

func isUSDToken(symbol string) bool {
	usdTokens := []string{"USDT", "USDC", "BUSD", "DAI", "TUSD", "USDP", "FRAX"}
	symbolUpper := strings.ToUpper(symbol)
	
	for _, usd := range usdTokens {
		if symbolUpper == usd || strings.HasSuffix(symbolUpper, usd) {
			return true
		}
	}
	
	return false
}

func parseFloat64(s string) (float64, error) {
	// Handle large numbers that might be in scientific notation
	var f float64
	_, err := fmt.Sscanf(s, "%f", &f)
	return f, err
}