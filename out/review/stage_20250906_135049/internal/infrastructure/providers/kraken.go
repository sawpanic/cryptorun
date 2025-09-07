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

type KrakenProvider struct {
	baseURL    string
	client     *httpclient.ClientPool
	health     *metrics.ProviderHealth
	mu         sync.RWMutex
	degraded   bool
	degradedReason string
}

type KrakenConfig struct {
	BaseURL        string
	RequestTimeout time.Duration
	MaxRetries     int
	MaxConcurrency int
}

func NewKrakenProvider(config KrakenConfig) *KrakenProvider {
	clientConfig := httpclient.ClientConfig{
		MaxConcurrency: config.MaxConcurrency,
		RequestTimeout: config.RequestTimeout,
		JitterRange:    [2]int{50, 150},
		MaxRetries:     config.MaxRetries,
		BackoffBase:    time.Second,
		BackoffMax:     15 * time.Second,
		UserAgent:      "CryptoRun/3.2.1 (Exchange-Native)",
	}
	
	return &KrakenProvider{
		baseURL: config.BaseURL,
		client:  httpclient.NewClientPool(clientConfig),
		health:  metrics.NewProviderHealth("kraken"),
	}
}

func (p *KrakenProvider) GetAssetPairs(ctx context.Context) (map[string]AssetPair, error) {
	url := fmt.Sprintf("%s/0/public/AssetPairs", p.baseURL)
	
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	
	startTime := time.Now()
	resp, err := p.client.Do(ctx, req)
	duration := time.Since(startTime)
	
	p.health.RecordRequest(err == nil, duration)
	
	if err != nil {
		log.Error().Err(err).Str("url", url).Msg("Kraken API request failed")
		return nil, p.handleDegradedState("api_error", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		err := fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
		return nil, p.handleDegradedState("http_error", err)
	}
	
	var response KrakenResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, p.handleDegradedState("decode_error", err)
	}
	
	if len(response.Error) > 0 {
		err := fmt.Errorf("Kraken API error: %v", response.Error)
		return nil, p.handleDegradedState("api_error", err)
	}
	
	pairs := make(map[string]AssetPair)
	if err := json.Unmarshal(response.Result, &pairs); err != nil {
		return nil, p.handleDegradedState("decode_error", err)
	}
	
	// Filter to USD pairs only
	usdPairs := make(map[string]AssetPair)
	for name, pair := range pairs {
		if strings.HasSuffix(pair.Quote, "USD") || strings.HasSuffix(pair.Quote, "ZUSD") {
			usdPairs[name] = pair
		}
	}
	
	log.Debug().
		Int("total_pairs", len(pairs)).
		Int("usd_pairs", len(usdPairs)).
		Dur("duration", duration).
		Msg("Kraken asset pairs retrieved")
	
	return usdPairs, nil
}

func (p *KrakenProvider) GetOrderBook(ctx context.Context, pair string, depth int) (*OrderBook, error) {
	url := fmt.Sprintf("%s/0/public/Depth?pair=%s&count=%d", p.baseURL, pair, depth)
	
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
		return nil, p.handleDegradedState("rate_limited", fmt.Errorf("rate limited by Kraken"))
	}
	
	if resp.StatusCode != http.StatusOK {
		err := fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
		return nil, p.handleDegradedState("http_error", err)
	}
	
	var response KrakenResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, p.handleDegradedState("decode_error", err)
	}
	
	if len(response.Error) > 0 {
		err := fmt.Errorf("Kraken API error: %v", response.Error)
		return nil, p.handleDegradedState("api_error", err)
	}
	
	// Parse order book data
	var bookData map[string]OrderBookData
	if err := json.Unmarshal(response.Result, &bookData); err != nil {
		return nil, p.handleDegradedState("decode_error", err)
	}
	
	// Get the first (and should be only) pair data
	for pairName, data := range bookData {
		orderBook := &OrderBook{
			Pair:      pairName,
			Bids:      p.parseOrders(data.Bids),
			Asks:      p.parseOrders(data.Asks),
			Timestamp: time.Now(),
		}
		
		log.Debug().
			Str("pair", pair).
			Int("bids", len(orderBook.Bids)).
			Int("asks", len(orderBook.Asks)).
			Dur("duration", duration).
			Msg("Kraken order book retrieved")
		
		return orderBook, nil
	}
	
	return nil, fmt.Errorf("no order book data for pair %s", pair)
}

func (p *KrakenProvider) Get24HVolume(ctx context.Context, pair string) (*VolumeData, error) {
	url := fmt.Sprintf("%s/0/public/Ticker?pair=%s", p.baseURL, pair)
	
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
	
	var response KrakenResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, p.handleDegradedState("decode_error", err)
	}
	
	if len(response.Error) > 0 {
		err := fmt.Errorf("Kraken API error: %v", response.Error)
		return nil, p.handleDegradedState("api_error", err)
	}
	
	var tickerData map[string]TickerData
	if err := json.Unmarshal(response.Result, &tickerData); err != nil {
		return nil, p.handleDegradedState("decode_error", err)
	}
	
	for pairName, ticker := range tickerData {
		volumeData := &VolumeData{
			Pair:           pairName,
			Volume24h:      parseFloat(ticker.Volume[1]), // 24h volume
			VolumeToday:    parseFloat(ticker.Volume[0]), // Today volume
			VWAP24h:        parseFloat(ticker.VWAP[1]),   // 24h VWAP
			Timestamp:      time.Now(),
		}
		
		log.Debug().
			Str("pair", pair).
			Float64("volume_24h", volumeData.Volume24h).
			Dur("duration", duration).
			Msg("Kraken volume data retrieved")
		
		return volumeData, nil
	}
	
	return nil, fmt.Errorf("no ticker data for pair %s", pair)
}

func (p *KrakenProvider) IsHealthy() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return !p.degraded && p.health.IsHealthy()
}

func (p *KrakenProvider) GetHealth() *metrics.ProviderHealth {
	return p.health
}

func (p *KrakenProvider) parseOrders(orders [][]interface{}) []Order {
	result := make([]Order, len(orders))
	for i, order := range orders {
		if len(order) >= 3 {
			result[i] = Order{
				Price:     parseFloat(order[0]),
				Volume:    parseFloat(order[1]),
				Timestamp: parseInt64(order[2]),
			}
		}
	}
	return result
}

func (p *KrakenProvider) handleRateLimit(resp *http.Response) {
	retryAfter := resp.Header.Get("Retry-After")
	if retryAfter == "" {
		retryAfter = "unknown"
	}
	
	log.Warn().
		Str("retry_after", retryAfter).
		Msg("Kraken rate limit hit")
}

func (p *KrakenProvider) handleDegradedState(reason string, err error) error {
	p.mu.Lock()
	p.degraded = true
	p.degradedReason = reason
	p.mu.Unlock()
	
	log.Warn().
		Err(err).
		Str("reason", reason).
		Msg("Kraken provider degraded")
	
	p.health.SetDegraded(true, reason)
	
	return fmt.Errorf("PROVIDER_DEGRADED: %s - %w", reason, err)
}

// Data structures
type KrakenResponse struct {
	Error  []string        `json:"error"`
	Result json.RawMessage `json:"result"`
}

type AssetPair struct {
	AltName           string `json:"altname"`
	WSName            string `json:"wsname"`
	ClassBase         string `json:"aclass_base"`
	Base              string `json:"base"`
	ClassQuote        string `json:"aclass_quote"`
	Quote             string `json:"quote"`
	Lot               string `json:"lot"`
	PairDecimals      int    `json:"pair_decimals"`
	LotDecimals       int    `json:"lot_decimals"`
	LotMultiplier     int    `json:"lot_multiplier"`
	LeverageBuy       []int  `json:"leverage_buy"`
	LeverageSell      []int  `json:"leverage_sell"`
	Fees              [][]float64 `json:"fees"`
	FeesMaker         [][]float64 `json:"fees_maker"`
	FeeVolumeCurrency string `json:"fee_volume_currency"`
	MarginCall        int    `json:"margin_call"`
	MarginStop        int    `json:"margin_stop"`
	OrderMin          string `json:"ordermin"`
}

type OrderBookData struct {
	Bids [][]interface{} `json:"bids"`
	Asks [][]interface{} `json:"asks"`
}

type OrderBook struct {
	Pair      string    `json:"pair"`
	Bids      []Order   `json:"bids"`
	Asks      []Order   `json:"asks"`
	Timestamp time.Time `json:"timestamp"`
}

type Order struct {
	Price     float64 `json:"price"`
	Volume    float64 `json:"volume"`
	Timestamp int64   `json:"timestamp"`
}

type TickerData struct {
	Ask                     []string `json:"a"` // [price, whole lot volume, lot volume]
	Bid                     []string `json:"b"` // [price, whole lot volume, lot volume]
	LastTradeClosed         []string `json:"c"` // [price, lot volume]
	Volume                  []string `json:"v"` // [today, last 24 hours]
	VWAP                    []string `json:"p"` // [today, last 24 hours]
	NumberOfTrades          []int    `json:"t"` // [today, last 24 hours]
	Low                     []string `json:"l"` // [today, last 24 hours]
	High                    []string `json:"h"` // [today, last 24 hours]
	TodaysOpeningPrice      string   `json:"o"`
}

type VolumeData struct {
	Pair        string    `json:"pair"`
	Volume24h   float64   `json:"volume_24h"`
	VolumeToday float64   `json:"volume_today"`
	VWAP24h     float64   `json:"vwap_24h"`
	Timestamp   time.Time `json:"timestamp"`
}

// Helper functions
func parseFloat(v interface{}) float64 {
	switch val := v.(type) {
	case string:
		if val == "" {
			return 0.0
		}
		// strconv.ParseFloat would be used here
		// For now, handle common decimal patterns
		result := 0.0
		dotPos := -1
		negative := false
		start := 0
		
		if len(val) > 0 && val[0] == '-' {
			negative = true
			start = 1
		}
		
		for i := start; i < len(val); i++ {
			char := val[i]
			if char >= '0' && char <= '9' {
				digit := float64(char - '0')
				if dotPos == -1 {
					result = result*10 + digit
				} else {
					divisor := 1.0
					for j := 0; j < i-dotPos; j++ {
						divisor *= 10
					}
					result += digit / divisor
				}
			} else if char == '.' && dotPos == -1 {
				dotPos = i
			} else {
				break // Invalid character, stop parsing
			}
		}
		
		if negative {
			result = -result
		}
		return result
	case float64:
		return val
	case int:
		return float64(val)
	case int64:
		return float64(val)
	default:
		return 0.0
	}
}

func parseInt64(v interface{}) int64 {
	switch val := v.(type) {
	case string:
		if val == "" {
			return 0
		}
		// strconv.ParseInt would be used here
		// For now, handle simple integer patterns
		result := int64(0)
		negative := false
		start := 0
		
		if len(val) > 0 && val[0] == '-' {
			negative = true
			start = 1
		}
		
		for i := start; i < len(val); i++ {
			char := val[i]
			if char >= '0' && char <= '9' {
				digit := int64(char - '0')
				result = result*10 + digit
			} else {
				break // Invalid character or decimal point
			}
		}
		
		if negative {
			result = -result
		}
		return result
	case float64:
		return int64(val)
	case int64:
		return val
	case int:
		return int64(val)
	default:
		return 0
	}
}