package providers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/rs/zerolog/log"

	"cryptorun/internal/infrastructure/httpclient"
	"cryptorun/internal/telemetry/metrics"
)

type CoinGeckoProvider struct {
	baseURL        string
	client         *httpclient.ClientPool
	budget         *Budget
	health         *metrics.ProviderHealth
	mu             sync.RWMutex
	degraded       bool
	degradedReason string
}

type Budget struct {
	RPMLimit     int
	MonthlyLimit int
	RPMUsed      int
	MonthlyUsed  int
	lastReset    time.Time
	mu           sync.RWMutex
}

type CoinGeckoConfig struct {
	BaseURL        string
	RPMLimit       int
	MonthlyLimit   int
	RequestTimeout time.Duration
	MaxRetries     int
	TTL            time.Duration
}

func NewCoinGeckoProvider(config CoinGeckoConfig) *CoinGeckoProvider {
	clientConfig := httpclient.ClientConfig{
		MaxConcurrency: 2, // Conservative for free tier
		RequestTimeout: config.RequestTimeout,
		JitterRange:    [2]int{50, 150},
		MaxRetries:     config.MaxRetries,
		BackoffBase:    time.Second,
		BackoffMax:     30 * time.Second,
		UserAgent:      "CryptoRun/3.2.1 (Free Tier)",
	}

	provider := &CoinGeckoProvider{
		baseURL: config.BaseURL,
		client:  httpclient.NewClientPool(clientConfig),
		budget: &Budget{
			RPMLimit:     config.RPMLimit,
			MonthlyLimit: config.MonthlyLimit,
			lastReset:    time.Now(),
		},
		health: metrics.NewProviderHealth("coingecko"),
	}

	// Start budget reset goroutine
	go provider.budgetResetLoop()

	return provider
}

func (p *CoinGeckoProvider) GetCoinsList(ctx context.Context) ([]CoinInfo, error) {
	if err := p.checkBudget("rpm"); err != nil {
		return nil, p.handleDegradedState("budget_exceeded", err)
	}

	url := fmt.Sprintf("%s/coins/list", p.baseURL)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	startTime := time.Now()
	resp, err := p.client.Do(ctx, req)
	duration := time.Since(startTime)

	p.health.RecordRequest(err == nil, duration)

	if err != nil {
		log.Error().Err(err).Str("url", url).Msg("CoinGecko API request failed")
		return nil, p.handleDegradedState("api_error", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		err := fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
		return nil, p.handleDegradedState("http_error", err)
	}

	var coins []CoinInfo
	if err := json.NewDecoder(resp.Body).Decode(&coins); err != nil {
		return nil, p.handleDegradedState("decode_error", err)
	}

	p.consumeBudget("rpm", 1)
	p.consumeBudget("monthly", 1)

	log.Debug().
		Int("coins_count", len(coins)).
		Dur("duration", duration).
		Msg("CoinGecko coins list retrieved")

	return coins, nil
}

func (p *CoinGeckoProvider) GetCoinsMarkets(ctx context.Context, vsCurrency string, page int, perPage int) ([]MarketData, error) {
	if err := p.checkBudget("rpm"); err != nil {
		return nil, p.handleDegradedState("budget_exceeded", err)
	}

	url := fmt.Sprintf("%s/coins/markets?vs_currency=%s&order=market_cap_desc&per_page=%d&page=%d&sparkline=false",
		p.baseURL, vsCurrency, perPage, page)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	startTime := time.Now()
	resp, err := p.client.Do(ctx, req)
	duration := time.Since(startTime)

	p.health.RecordRequest(err == nil, duration)

	if err != nil {
		return nil, p.handleDegradedState("api_error", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusTooManyRequests {
		p.handleRateLimit(resp)
		return nil, p.handleDegradedState("rate_limited", fmt.Errorf("rate limited by CoinGecko"))
	}

	if resp.StatusCode != http.StatusOK {
		err := fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
		return nil, p.handleDegradedState("http_error", err)
	}

	var markets []MarketData
	if err := json.NewDecoder(resp.Body).Decode(&markets); err != nil {
		return nil, p.handleDegradedState("decode_error", err)
	}

	p.consumeBudget("rpm", 1)
	p.consumeBudget("monthly", 1)

	log.Debug().
		Int("markets_count", len(markets)).
		Str("vs_currency", vsCurrency).
		Dur("duration", duration).
		Msg("CoinGecko markets data retrieved")

	return markets, nil
}

func (p *CoinGeckoProvider) IsHealthy() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return !p.degraded && p.health.IsHealthy()
}

func (p *CoinGeckoProvider) GetHealth() *metrics.ProviderHealth {
	return p.health
}

func (p *CoinGeckoProvider) checkBudget(budgetType string) error {
	p.budget.mu.RLock()
	defer p.budget.mu.RUnlock()

	switch budgetType {
	case "rpm":
		if p.budget.RPMUsed >= p.budget.RPMLimit {
			return fmt.Errorf("RPM budget exceeded: %d/%d", p.budget.RPMUsed, p.budget.RPMLimit)
		}
	case "monthly":
		if p.budget.MonthlyUsed >= p.budget.MonthlyLimit {
			return fmt.Errorf("monthly budget exceeded: %d/%d", p.budget.MonthlyUsed, p.budget.MonthlyLimit)
		}
	}

	return nil
}

func (p *CoinGeckoProvider) consumeBudget(budgetType string, amount int) {
	p.budget.mu.Lock()
	defer p.budget.mu.Unlock()

	switch budgetType {
	case "rpm":
		p.budget.RPMUsed += amount
	case "monthly":
		p.budget.MonthlyUsed += amount
	}
}

func (p *CoinGeckoProvider) budgetResetLoop() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		p.budget.mu.Lock()
		p.budget.RPMUsed = 0 // Reset RPM every minute
		p.budget.mu.Unlock()

		// Reset monthly budget on first day of month
		now := time.Now()
		if now.Day() == 1 && now.Hour() == 0 && now.Minute() == 0 {
			p.budget.mu.Lock()
			p.budget.MonthlyUsed = 0
			p.budget.lastReset = now
			p.budget.mu.Unlock()

			log.Info().Msg("CoinGecko monthly budget reset")
		}
	}
}

func (p *CoinGeckoProvider) handleRateLimit(resp *http.Response) {
	retryAfter := resp.Header.Get("Retry-After")
	if retryAfter != "" {
		log.Warn().
			Str("retry_after", retryAfter).
			Msg("CoinGecko rate limit hit")
	}
}

func (p *CoinGeckoProvider) handleDegradedState(reason string, err error) error {
	p.mu.Lock()
	p.degraded = true
	p.degradedReason = reason
	p.mu.Unlock()

	log.Warn().
		Err(err).
		Str("reason", reason).
		Msg("CoinGecko provider degraded")

	p.health.SetDegraded(true, reason)

	return fmt.Errorf("PROVIDER_DEGRADED: %s - %w", reason, err)
}

// Data structures
type CoinInfo struct {
	ID     string `json:"id"`
	Symbol string `json:"symbol"`
	Name   string `json:"name"`
}

type MarketData struct {
	ID                 string  `json:"id"`
	Symbol             string  `json:"symbol"`
	Name               string  `json:"name"`
	CurrentPrice       float64 `json:"current_price"`
	MarketCap          float64 `json:"market_cap"`
	MarketCapRank      int     `json:"market_cap_rank"`
	TotalVolume        float64 `json:"total_volume"`
	PriceChange24h     float64 `json:"price_change_24h"`
	PriceChangePerc24h float64 `json:"price_change_percentage_24h"`
	LastUpdated        string  `json:"last_updated"`
}
