package providers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/sawpanic/cryptorun/internal/infrastructure/httpclient"
	"github.com/sawpanic/cryptorun/internal/telemetry/metrics"
)

type OKXProvider struct {
	baseURL        string
	client         *httpclient.ClientPool
	health         *metrics.ProviderHealth
	mu             sync.RWMutex
	degraded       bool
	degradedReason string
}

type OKXConfig struct {
	BaseURL        string
	RequestTimeout time.Duration
	MaxRetries     int
	MaxConcurrency int
}

func NewOKXProvider(config OKXConfig) *OKXProvider {
	clientConfig := httpclient.ClientConfig{
		MaxConcurrency: config.MaxConcurrency,
		RequestTimeout: config.RequestTimeout,
		JitterRange:    [2]int{50, 150},
		MaxRetries:     config.MaxRetries,
		BackoffBase:    time.Second,
		BackoffMax:     15 * time.Second,
		UserAgent:      "github.com/sawpanic/cryptorun/3.2.1 (Exchange-Native)",
	}

	return &OKXProvider{
		baseURL: config.BaseURL,
		client:  httpclient.NewClientPool(clientConfig),
		health:  metrics.NewProviderHealth("okx"),
	}
}

func (p *OKXProvider) GetInstruments(ctx context.Context, instType string) ([]OKXInstrument, error) {
	url := fmt.Sprintf("%s/api/v5/public/instruments?instType=%s", p.baseURL, instType)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	startTime := time.Now()
	resp, err := p.client.Do(ctx, req)
	duration := time.Since(startTime)

	p.health.RecordRequest(err == nil, duration)

	if err != nil {
		log.Error().Err(err).Str("url", url).Msg("OKX API request failed")
		return nil, p.handleDegradedState("api_error", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		err := fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
		return nil, p.handleDegradedState("http_error", err)
	}

	var response OKXResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, p.handleDegradedState("decode_error", err)
	}

	if response.Code != "0" {
		err := fmt.Errorf("OKX API error: %s - %s", response.Code, response.Msg)
		return nil, p.handleDegradedState("api_error", err)
	}

	var instruments []OKXInstrument
	if err := json.Unmarshal(response.Data, &instruments); err != nil {
		return nil, p.handleDegradedState("decode_error", err)
	}

	log.Debug().
		Int("instruments_count", len(instruments)).
		Str("inst_type", instType).
		Dur("duration", duration).
		Msg("OKX instruments retrieved")

	return instruments, nil
}

func (p *OKXProvider) GetOrderBook(ctx context.Context, instID string, depth int) (*OKXOrderBook, error) {
	url := fmt.Sprintf("%s/api/v5/market/books?instId=%s&sz=%d", p.baseURL, instID, depth)

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
		return nil, p.handleDegradedState("rate_limited", fmt.Errorf("rate limited by OKX"))
	}

	if resp.StatusCode != http.StatusOK {
		err := fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
		return nil, p.handleDegradedState("http_error", err)
	}

	var response OKXResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, p.handleDegradedState("decode_error", err)
	}

	if response.Code != "0" {
		err := fmt.Errorf("OKX API error: %s - %s", response.Code, response.Msg)
		return nil, p.handleDegradedState("api_error", err)
	}

	var books []OKXOrderBook
	if err := json.Unmarshal(response.Data, &books); err != nil {
		return nil, p.handleDegradedState("decode_error", err)
	}

	if len(books) == 0 {
		return nil, fmt.Errorf("no order book data for instrument %s", instID)
	}

	book := &books[0]
	book.Timestamp = time.Now()

	log.Debug().
		Str("inst_id", instID).
		Int("bids", len(book.Bids)).
		Int("asks", len(book.Asks)).
		Dur("duration", duration).
		Msg("OKX order book retrieved")

	return book, nil
}

func (p *OKXProvider) Get24HTicker(ctx context.Context, instID string) (*OKXTicker, error) {
	url := fmt.Sprintf("%s/api/v5/market/ticker?instId=%s", p.baseURL, instID)

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

	var response OKXResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, p.handleDegradedState("decode_error", err)
	}

	if response.Code != "0" {
		err := fmt.Errorf("OKX API error: %s - %s", response.Code, response.Msg)
		return nil, p.handleDegradedState("api_error", err)
	}

	var tickers []OKXTicker
	if err := json.Unmarshal(response.Data, &tickers); err != nil {
		return nil, p.handleDegradedState("decode_error", err)
	}

	if len(tickers) == 0 {
		return nil, fmt.Errorf("no ticker data for instrument %s", instID)
	}

	ticker := &tickers[0]
	ticker.Timestamp = time.Now()

	log.Debug().
		Str("inst_id", instID).
		Str("last_price", ticker.Last).
		Str("volume_24h", ticker.Vol24h).
		Dur("duration", duration).
		Msg("OKX ticker retrieved")

	return ticker, nil
}

func (p *OKXProvider) IsHealthy() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return !p.degraded && p.health.IsHealthy()
}

func (p *OKXProvider) GetHealth() *metrics.ProviderHealth {
	return p.health
}

func (p *OKXProvider) handleRateLimit(resp *http.Response) {
	retryAfter := resp.Header.Get("Retry-After")
	if retryAfter == "" {
		retryAfter = "unknown"
	}

	log.Warn().
		Str("retry_after", retryAfter).
		Msg("OKX rate limit hit")
}

func (p *OKXProvider) handleDegradedState(reason string, err error) error {
	p.mu.Lock()
	p.degraded = true
	p.degradedReason = reason
	p.mu.Unlock()

	log.Warn().
		Err(err).
		Str("reason", reason).
		Msg("OKX provider degraded")

	p.health.SetDegraded(true, reason)

	return fmt.Errorf("PROVIDER_DEGRADED: %s - %w", reason, err)
}

// Data structures
type OKXResponse struct {
	Code string          `json:"code"`
	Msg  string          `json:"msg"`
	Data json.RawMessage `json:"data"`
}

type OKXInstrument struct {
	InstType  string `json:"instType"`
	InstID    string `json:"instId"`
	Uly       string `json:"uly"`
	Category  string `json:"category"`
	BaseCcy   string `json:"baseCcy"`
	QuoteCcy  string `json:"quoteCcy"`
	SettleCcy string `json:"settleCcy"`
	CtVal     string `json:"ctVal"`
	CtMult    string `json:"ctMult"`
	CtValCcy  string `json:"ctValCcy"`
	OptType   string `json:"optType"`
	Stk       string `json:"stk"`
	ListTime  string `json:"listTime"`
	ExpTime   string `json:"expTime"`
	Lever     string `json:"lever"`
	TickSz    string `json:"tickSz"`
	LotSz     string `json:"lotSz"`
	MinSz     string `json:"minSz"`
	CtType    string `json:"ctType"`
	Alias     string `json:"alias"`
	State     string `json:"state"`
}

type OKXOrderBook struct {
	Asks      [][]string `json:"asks"` // [price, size, liquidated_orders, num_orders]
	Bids      [][]string `json:"bids"` // [price, size, liquidated_orders, num_orders]
	Timestamp time.Time  `json:"ts"`
}

type OKXTicker struct {
	InstType  string    `json:"instType"`
	InstID    string    `json:"instId"`
	Last      string    `json:"last"`
	LastSz    string    `json:"lastSz"`
	AskPx     string    `json:"askPx"`
	AskSz     string    `json:"askSz"`
	BidPx     string    `json:"bidPx"`
	BidSz     string    `json:"bidSz"`
	Open24h   string    `json:"open24h"`
	High24h   string    `json:"high24h"`
	Low24h    string    `json:"low24h"`
	Vol24h    string    `json:"vol24h"`
	VolCcy24h string    `json:"volCcy24h"`
	SodUtc0   string    `json:"sodUtc0"`
	SodUtc8   string    `json:"sodUtc8"`
	TS        string    `json:"ts"`
	Timestamp time.Time `json:"timestamp"`
}
