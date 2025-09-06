package derivs

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"time"
)

// Provider represents a derivatives data provider
type Provider interface {
	GetFundingHistory(ctx context.Context, symbol string, limit int) ([]FundingRate, error)
	GetOpenInterest(ctx context.Context, symbol string) (*OpenInterest, error)
	GetTickerData(ctx context.Context, symbol string) (*TickerData, error)
	Name() string
}

// FundingRate represents a funding rate data point
type FundingRate struct {
	Symbol    string    `json:"symbol"`
	Rate      float64   `json:"fundingRate,string"`
	Timestamp time.Time `json:"fundingTime"`
	MarkPrice float64   `json:"markPrice,string"`
}

// OpenInterest represents open interest data
type OpenInterest struct {
	Symbol    string    `json:"symbol"`
	Value     float64   `json:"openInterest,string"`
	Timestamp time.Time `json:"time"`
}

// TickerData represents 24hr ticker statistics
type TickerData struct {
	Symbol           string    `json:"symbol"`
	LastPrice        float64   `json:"lastPrice,string"`
	Volume           float64   `json:"volume,string"`
	QuoteVolume      float64   `json:"quoteVolume,string"`
	WeightedAvgPrice float64   `json:"weightedAvgPrice,string"`
	Timestamp        time.Time `json:"closeTime"`
}

// RateLimiter implements token bucket rate limiting
type RateLimiter struct {
	tokens     int
	maxTokens  int
	refillRate int
	mu         sync.Mutex
	lastRefill time.Time
}

func NewRateLimiter(maxTokens, refillRatePerSecond int) *RateLimiter {
	return &RateLimiter{
		tokens:     maxTokens,
		maxTokens:  maxTokens,
		refillRate: refillRatePerSecond,
		lastRefill: time.Now(),
	}
}

func (rl *RateLimiter) Allow() bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	// Refill tokens based on time elapsed
	now := time.Now()
	elapsed := now.Sub(rl.lastRefill)
	tokensToAdd := int(elapsed.Seconds()) * rl.refillRate

	if tokensToAdd > 0 {
		rl.tokens = min(rl.maxTokens, rl.tokens+tokensToAdd)
		rl.lastRefill = now
	}

	if rl.tokens > 0 {
		rl.tokens--
		return true
	}

	return false
}

// BinanceProvider implements Binance derivatives API
type BinanceProvider struct {
	baseURL     string
	client      *http.Client
	rateLimiter *RateLimiter
}

func NewBinanceProvider(baseURL string, rpsLimit int) *BinanceProvider {
	return &BinanceProvider{
		baseURL: baseURL,
		client: &http.Client{
			Timeout: 5 * time.Second,
		},
		rateLimiter: NewRateLimiter(rpsLimit*2, rpsLimit), // 2x burst capacity
	}
}

func (b *BinanceProvider) Name() string {
	return "binance"
}

func (b *BinanceProvider) GetFundingHistory(ctx context.Context, symbol string, limit int) ([]FundingRate, error) {
	if !b.rateLimiter.Allow() {
		return nil, fmt.Errorf("rate limit exceeded for binance")
	}

	endpoint := "/fapi/v1/fundingHistory"
	params := url.Values{}
	params.Set("symbol", symbol)
	params.Set("limit", strconv.Itoa(limit))

	url := fmt.Sprintf("%s%s?%s", b.baseURL, endpoint, params.Encode())

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := b.client.Do(req)
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

	var result []struct {
		Symbol      string `json:"symbol"`
		FundingRate string `json:"fundingRate"`
		FundingTime int64  `json:"fundingTime"`
		MarkPrice   string `json:"markPrice"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	rates := make([]FundingRate, len(result))
	for i, r := range result {
		rate, _ := strconv.ParseFloat(r.FundingRate, 64)
		markPrice, _ := strconv.ParseFloat(r.MarkPrice, 64)

		rates[i] = FundingRate{
			Symbol:    r.Symbol,
			Rate:      rate,
			Timestamp: time.Unix(r.FundingTime/1000, 0),
			MarkPrice: markPrice,
		}
	}

	return rates, nil
}

func (b *BinanceProvider) GetOpenInterest(ctx context.Context, symbol string) (*OpenInterest, error) {
	if !b.rateLimiter.Allow() {
		return nil, fmt.Errorf("rate limit exceeded for binance")
	}

	endpoint := "/fapi/v1/openInterest"
	params := url.Values{}
	params.Set("symbol", symbol)

	url := fmt.Sprintf("%s%s?%s", b.baseURL, endpoint, params.Encode())

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := b.client.Do(req)
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
		Symbol       string `json:"symbol"`
		OpenInterest string `json:"openInterest"`
		Time         int64  `json:"time"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	oi, _ := strconv.ParseFloat(result.OpenInterest, 64)

	return &OpenInterest{
		Symbol:    result.Symbol,
		Value:     oi,
		Timestamp: time.Unix(result.Time/1000, 0),
	}, nil
}

func (b *BinanceProvider) GetTickerData(ctx context.Context, symbol string) (*TickerData, error) {
	if !b.rateLimiter.Allow() {
		return nil, fmt.Errorf("rate limit exceeded for binance")
	}

	endpoint := "/fapi/v1/ticker/24hr"
	params := url.Values{}
	params.Set("symbol", symbol)

	url := fmt.Sprintf("%s%s?%s", b.baseURL, endpoint, params.Encode())

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := b.client.Do(req)
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
		Symbol           string `json:"symbol"`
		LastPrice        string `json:"lastPrice"`
		Volume           string `json:"volume"`
		QuoteVolume      string `json:"quoteVolume"`
		WeightedAvgPrice string `json:"weightedAvgPrice"`
		CloseTime        int64  `json:"closeTime"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	lastPrice, _ := strconv.ParseFloat(result.LastPrice, 64)
	volume, _ := strconv.ParseFloat(result.Volume, 64)
	quoteVolume, _ := strconv.ParseFloat(result.QuoteVolume, 64)
	weightedAvgPrice, _ := strconv.ParseFloat(result.WeightedAvgPrice, 64)

	return &TickerData{
		Symbol:           result.Symbol,
		LastPrice:        lastPrice,
		Volume:           volume,
		QuoteVolume:      quoteVolume,
		WeightedAvgPrice: weightedAvgPrice,
		Timestamp:        time.Unix(result.CloseTime/1000, 0),
	}, nil
}

// OKXProvider implements OKX derivatives API
type OKXProvider struct {
	baseURL     string
	client      *http.Client
	rateLimiter *RateLimiter
}

func NewOKXProvider(baseURL string, rpsLimit int) *OKXProvider {
	return &OKXProvider{
		baseURL: baseURL,
		client: &http.Client{
			Timeout: 5 * time.Second,
		},
		rateLimiter: NewRateLimiter(rpsLimit*2, rpsLimit),
	}
}

func (o *OKXProvider) Name() string {
	return "okx"
}

func (o *OKXProvider) GetFundingHistory(ctx context.Context, symbol string, limit int) ([]FundingRate, error) {
	if !o.rateLimiter.Allow() {
		return nil, fmt.Errorf("rate limit exceeded for okx")
	}

	// OKX uses different symbol format - convert if needed
	instId := convertSymbolToOKX(symbol)

	endpoint := "/api/v5/public/funding-history"
	params := url.Values{}
	params.Set("instId", instId)
	params.Set("limit", strconv.Itoa(limit))

	url := fmt.Sprintf("%s%s?%s", o.baseURL, endpoint, params.Encode())

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := o.client.Do(req)
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
		Code string `json:"code"`
		Data []struct {
			InstId      string `json:"instId"`
			FundingRate string `json:"fundingRate"`
			FundingTime string `json:"fundingTime"`
			MarkPx      string `json:"markPx"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	if result.Code != "0" {
		return nil, fmt.Errorf("OKX API error: code %s", result.Code)
	}

	rates := make([]FundingRate, len(result.Data))
	for i, r := range result.Data {
		rate, _ := strconv.ParseFloat(r.FundingRate, 64)
		markPrice, _ := strconv.ParseFloat(r.MarkPx, 64)

		timestamp, _ := strconv.ParseInt(r.FundingTime, 10, 64)

		rates[i] = FundingRate{
			Symbol:    convertSymbolFromOKX(r.InstId),
			Rate:      rate,
			Timestamp: time.Unix(timestamp/1000, 0),
			MarkPrice: markPrice,
		}
	}

	return rates, nil
}

func (o *OKXProvider) GetOpenInterest(ctx context.Context, symbol string) (*OpenInterest, error) {
	if !o.rateLimiter.Allow() {
		return nil, fmt.Errorf("rate limit exceeded for okx")
	}

	instId := convertSymbolToOKX(symbol)

	endpoint := "/api/v5/public/open-interest"
	params := url.Values{}
	params.Set("instId", instId)

	url := fmt.Sprintf("%s%s?%s", o.baseURL, endpoint, params.Encode())

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := o.client.Do(req)
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
		Code string `json:"code"`
		Data []struct {
			InstId string `json:"instId"`
			Oi     string `json:"oi"`
			OiCcy  string `json:"oiCcy"`
			Ts     string `json:"ts"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	if result.Code != "0" || len(result.Data) == 0 {
		return nil, fmt.Errorf("OKX API error or no data")
	}

	data := result.Data[0]
	oi, _ := strconv.ParseFloat(data.Oi, 64)
	timestamp, _ := strconv.ParseInt(data.Ts, 10, 64)

	return &OpenInterest{
		Symbol:    convertSymbolFromOKX(data.InstId),
		Value:     oi,
		Timestamp: time.Unix(timestamp/1000, 0),
	}, nil
}

func (o *OKXProvider) GetTickerData(ctx context.Context, symbol string) (*TickerData, error) {
	// OKX implementation - simplified for now
	return nil, fmt.Errorf("GetTickerData not implemented for OKX yet")
}

// Helper functions for OKX symbol conversion
func convertSymbolToOKX(symbol string) string {
	// Convert BTCUSDT -> BTC-USDT-SWAP
	if len(symbol) > 4 && symbol[len(symbol)-4:] == "USDT" {
		base := symbol[:len(symbol)-4]
		return fmt.Sprintf("%s-USDT-SWAP", base)
	}
	return symbol
}

func convertSymbolFromOKX(instId string) string {
	// Convert BTC-USDT-SWAP -> BTCUSDT
	if len(instId) > 10 && instId[len(instId)-5:] == "-SWAP" {
		parts := instId[:len(instId)-5] // Remove -SWAP
		// Remove dash: BTC-USDT -> BTCUSDT
		return parts[0:3] + parts[4:]
	}
	return instId
}

// ProviderManager manages multiple derivatives providers
type ProviderManager struct {
	providers []Provider
	mu        sync.RWMutex
}

func NewProviderManager() *ProviderManager {
	return &ProviderManager{
		providers: make([]Provider, 0),
	}
}

func (pm *ProviderManager) AddProvider(provider Provider) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	pm.providers = append(pm.providers, provider)
}

func (pm *ProviderManager) GetProviders() []Provider {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	result := make([]Provider, len(pm.providers))
	copy(result, pm.providers)
	return result
}

// Utility function
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
