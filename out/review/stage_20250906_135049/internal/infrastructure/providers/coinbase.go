package providers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog/log"

	"cryptorun/internal/infrastructure/httpclient"
	"cryptorun/internal/telemetry/metrics"
)

type CoinbaseProvider struct {
	baseURL    string
	client     *httpclient.ClientPool
	health     *metrics.ProviderHealth
	mu         sync.RWMutex
	degraded   bool
	degradedReason string
}

type CoinbaseConfig struct {
	BaseURL        string
	RequestTimeout time.Duration
	MaxRetries     int
	MaxConcurrency int
}

func NewCoinbaseProvider(config CoinbaseConfig) *CoinbaseProvider {
	clientConfig := httpclient.ClientConfig{
		MaxConcurrency: config.MaxConcurrency,
		RequestTimeout: config.RequestTimeout,
		JitterRange:    [2]int{50, 150},
		MaxRetries:     config.MaxRetries,
		BackoffBase:    time.Second,
		BackoffMax:     15 * time.Second,
		UserAgent:      "CryptoRun/3.2.1 (Exchange-Native)",
	}
	
	return &CoinbaseProvider{
		baseURL: config.BaseURL,
		client:  httpclient.NewClientPool(clientConfig),
		health:  metrics.NewProviderHealth("coinbase"),
	}
}

func (p *CoinbaseProvider) GetProducts(ctx context.Context) ([]CoinbaseProduct, error) {
	url := fmt.Sprintf("%s/products", p.baseURL)
	
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	
	startTime := time.Now()
	resp, err := p.client.Do(ctx, req)
	duration := time.Since(startTime)
	
	p.health.RecordRequest(err == nil, duration)
	
	if err != nil {
		log.Error().Err(err).Str("url", url).Msg("Coinbase API request failed")
		return nil, p.handleDegradedState("api_error", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		err := fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
		return nil, p.handleDegradedState("http_error", err)
	}
	
	var products []CoinbaseProduct
	if err := json.NewDecoder(resp.Body).Decode(&products); err != nil {
		return nil, p.handleDegradedState("decode_error", err)
	}
	
	// Filter to USD pairs only
	usdProducts := make([]CoinbaseProduct, 0)
	for _, product := range products {
		if strings.HasSuffix(product.ID, "-USD") && product.Status == "online" {
			usdProducts = append(usdProducts, product)
		}
	}
	
	log.Debug().
		Int("total_products", len(products)).
		Int("usd_products", len(usdProducts)).
		Dur("duration", duration).
		Msg("Coinbase products retrieved")
	
	return usdProducts, nil
}

func (p *CoinbaseProvider) GetOrderBook(ctx context.Context, productID string, level int) (*CoinbaseOrderBook, error) {
	url := fmt.Sprintf("%s/products/%s/book?level=%d", p.baseURL, productID, level)
	
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
		return nil, p.handleDegradedState("rate_limited", fmt.Errorf("rate limited by Coinbase"))
	}
	
	if resp.StatusCode != http.StatusOK {
		err := fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
		return nil, p.handleDegradedState("http_error", err)
	}
	
	var book CoinbaseOrderBook
	if err := json.NewDecoder(resp.Body).Decode(&book); err != nil {
		return nil, p.handleDegradedState("decode_error", err)
	}
	
	book.ProductID = productID
	book.Timestamp = time.Now()
	
	log.Debug().
		Str("product_id", productID).
		Int("bids", len(book.Bids)).
		Int("asks", len(book.Asks)).
		Dur("duration", duration).
		Msg("Coinbase order book retrieved")
	
	return &book, nil
}

func (p *CoinbaseProvider) Get24HStats(ctx context.Context, productID string) (*CoinbaseStats, error) {
	url := fmt.Sprintf("%s/products/%s/stats", p.baseURL, productID)
	
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
	
	if resp.StatusCode != http.StatusOK {
		err := fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
		return nil, p.handleDegradedState("http_error", err)
	}
	
	var stats CoinbaseStats
	if err := json.NewDecoder(resp.Body).Decode(&stats); err != nil {
		return nil, p.handleDegradedState("decode_error", err)
	}
	
	stats.ProductID = productID
	stats.Timestamp = time.Now()
	
	log.Debug().
		Str("product_id", productID).
		Str("volume", stats.Volume).
		Str("last", stats.Last).
		Dur("duration", duration).
		Msg("Coinbase stats retrieved")
	
	return &stats, nil
}

func (p *CoinbaseProvider) IsHealthy() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return !p.degraded && p.health.IsHealthy()
}

func (p *CoinbaseProvider) GetHealth() *metrics.ProviderHealth {
	return p.health
}

func (p *CoinbaseProvider) handleRateLimit(resp *http.Response) {
	retryAfter := resp.Header.Get("Retry-After")
	if retryAfter == "" {
		retryAfter = "unknown"
	}
	
	log.Warn().
		Str("retry_after", retryAfter).
		Msg("Coinbase rate limit hit")
}

func (p *CoinbaseProvider) handleDegradedState(reason string, err error) error {
	p.mu.Lock()
	p.degraded = true
	p.degradedReason = reason
	p.mu.Unlock()
	
	log.Warn().
		Err(err).
		Str("reason", reason).
		Msg("Coinbase provider degraded")
	
	p.health.SetDegraded(true, reason)
	
	return fmt.Errorf("PROVIDER_DEGRADED: %s - %w", reason, err)
}

// Data structures
type CoinbaseProduct struct {
	ID                     string `json:"id"`
	BaseCurrency           string `json:"base_currency"`
	QuoteCurrency          string `json:"quote_currency"`
	BaseMinSize            string `json:"base_min_size"`
	BaseMaxSize            string `json:"base_max_size"`
	QuoteIncrement         string `json:"quote_increment"`
	BaseIncrement          string `json:"base_increment"`
	DisplayName            string `json:"display_name"`
	MinMarketFunds         string `json:"min_market_funds"`
	MaxMarketFunds         string `json:"max_market_funds"`
	MarginEnabled          bool   `json:"margin_enabled"`
	PostOnly               bool   `json:"post_only"`
	LimitOnly              bool   `json:"limit_only"`
	CancelOnly             bool   `json:"cancel_only"`
	TradingDisabled        bool   `json:"trading_disabled"`
	Status                 string `json:"status"`
	StatusMessage          string `json:"status_message"`
}

type CoinbaseOrderBook struct {
	ProductID string     `json:"product_id"`
	Sequence  int64      `json:"sequence"`
	Bids      [][]string `json:"bids"` // [price, size, num-orders]
	Asks      [][]string `json:"asks"` // [price, size, num-orders]
	Timestamp time.Time  `json:"timestamp"`
}

type CoinbaseStats struct {
	ProductID string    `json:"product_id"`
	Open      string    `json:"open"`
	High      string    `json:"high"`
	Low       string    `json:"low"`
	Volume    string    `json:"volume"`
	Last      string    `json:"last"`
	Volume30Day string  `json:"volume_30day"`
	Timestamp time.Time `json:"timestamp"`
}