package adapters

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/sawpanic/cryptorun/src/infrastructure/datafacade/http"
	"github.com/sawpanic/cryptorun/src/infrastructure/datafacade/interfaces"
	
	"github.com/gorilla/websocket"
)

// BinanceAdapter implements VenueAdapter for Binance
type BinanceAdapter struct {
	venue       string
	baseURL     string
	wsURL       string
	httpClient  *http.Client
	wsConns     map[string]*websocket.Conn
	wsConnsMux  sync.RWMutex
	
	// Rate limiting
	rateLimiter interfaces.RateLimiter
	
	// Circuit breaker
	circuitBreaker interfaces.CircuitBreaker
}

// NewBinanceAdapter creates a new Binance adapter
func NewBinanceAdapter(rateLimiter interfaces.RateLimiter, circuitBreaker interfaces.CircuitBreaker) *BinanceAdapter {
	return &BinanceAdapter{
		venue:          "binance",
		baseURL:        "https://api.binance.com",
		wsURL:          "wss://stream.binance.com:9443/ws",
		httpClient:     http.NewClient("https://api.binance.com", 30*time.Second),
		wsConns:        make(map[string]*websocket.Conn),
		rateLimiter:    rateLimiter,
		circuitBreaker: circuitBreaker,
	}
}

// GetVenue returns the venue name
func (b *BinanceAdapter) GetVenue() string {
	return b.venue
}

// IsSupported checks if a data type is supported
func (b *BinanceAdapter) IsSupported(dataType interfaces.DataType) bool {
	supported := map[interfaces.DataType]bool{
		interfaces.DataTypeTrades:       true,
		interfaces.DataTypeKlines:       true,
		interfaces.DataTypeOrderBook:    true,
		interfaces.DataTypeFunding:      true,  // For futures
		interfaces.DataTypeOpenInterest: true,  // For futures
	}
	return supported[dataType]
}

// StreamTrades implements hot trade streaming
func (b *BinanceAdapter) StreamTrades(ctx context.Context, symbol string) (<-chan interfaces.TradeEvent, error) {
	stream := fmt.Sprintf("%s@trade", strings.ToLower(b.normalizeSymbol(symbol)))
	wsURL := fmt.Sprintf("%s/%s", b.wsURL, stream)
	
	tradeChan := make(chan interfaces.TradeEvent, 100)
	
	go func() {
		defer close(tradeChan)
		
		for {
			select {
			case <-ctx.Done():
				return
			default:
				if err := b.connectAndStream(ctx, wsURL, stream, tradeChan, b.parseTradeEvent); err != nil {
					// Log error and attempt reconnection after delay
					select {
					case <-ctx.Done():
						return
					case <-time.After(5 * time.Second):
						continue
					}
				}
			}
		}
	}()
	
	return tradeChan, nil
}

// StreamKlines implements hot kline streaming  
func (b *BinanceAdapter) StreamKlines(ctx context.Context, symbol string, interval string) (<-chan interfaces.KlineEvent, error) {
	stream := fmt.Sprintf("%s@kline_%s", strings.ToLower(b.normalizeSymbol(symbol)), interval)
	wsURL := fmt.Sprintf("%s/%s", b.wsURL, stream)
	
	klineChan := make(chan interfaces.KlineEvent, 100)
	
	go func() {
		defer close(klineChan)
		
		for {
			select {
			case <-ctx.Done():
				return
			default:
				if err := b.connectAndStream(ctx, wsURL, stream, klineChan, b.parseKlineEvent); err != nil {
					select {
					case <-ctx.Done():
						return  
					case <-time.After(5 * time.Second):
						continue
					}
				}
			}
		}
	}()
	
	return klineChan, nil
}

// StreamOrderBook implements hot order book streaming
func (b *BinanceAdapter) StreamOrderBook(ctx context.Context, symbol string, depth int) (<-chan interfaces.OrderBookEvent, error) {
	// Use depth stream for real-time updates
	stream := fmt.Sprintf("%s@depth", strings.ToLower(b.normalizeSymbol(symbol)))
	wsURL := fmt.Sprintf("%s/%s", b.wsURL, stream)
	
	orderBookChan := make(chan interfaces.OrderBookEvent, 100)
	
	go func() {
		defer close(orderBookChan)
		
		for {
			select {
			case <-ctx.Done():
				return
			default:
				if err := b.connectAndStream(ctx, wsURL, stream, orderBookChan, b.parseOrderBookEvent); err != nil {
					select {
					case <-ctx.Done():
						return
					case <-time.After(5 * time.Second):
						continue
					}
				}
			}
		}
	}()
	
	return orderBookChan, nil
}

// StreamFunding implements funding rate streaming (for futures)
func (b *BinanceAdapter) StreamFunding(ctx context.Context, symbol string) (<-chan interfaces.FundingEvent, error) {
	// This would typically be for Binance Futures
	stream := fmt.Sprintf("%s@markPrice", strings.ToLower(b.normalizeSymbol(symbol)))
	wsURL := fmt.Sprintf("wss://fstream.binance.com/ws/%s", stream)
	
	fundingChan := make(chan interfaces.FundingEvent, 100)
	
	go func() {
		defer close(fundingChan)
		
		for {
			select {
			case <-ctx.Done():
				return
			default:
				if err := b.connectAndStream(ctx, wsURL, stream, fundingChan, b.parseFundingEvent); err != nil {
					select {
					case <-ctx.Done():
						return
					case <-time.After(10 * time.Second): // Longer delay for funding
						continue
					}
				}
			}
		}
	}()
	
	return fundingChan, nil
}

// FetchTrades implements warm trade fetching with rate limiting
func (b *BinanceAdapter) FetchTrades(ctx context.Context, symbol string, limit int) ([]interfaces.Trade, error) {
	var trades []interfaces.Trade
	err := b.circuitBreaker.Call(ctx, "fetch_trades", func() error {
		// Check rate limits
		if err := b.rateLimiter.Allow(ctx, b.venue, "/api/v3/trades"); err != nil {
			return fmt.Errorf("rate limited: %w", err)
		}
		
		endpoint := fmt.Sprintf("/api/v3/trades?symbol=%s&limit=%d", 
			b.normalizeSymbol(symbol), limit)
		
		resp, rateLimitHeaders, err := b.httpClient.GetWithRateLimitHeaders(ctx, endpoint)
		if err != nil {
			return fmt.Errorf("http request: %w", err)
		}
		
		// Process rate limit headers
		if err := b.processRateLimitHeaders(rateLimitHeaders); err != nil {
			return fmt.Errorf("process rate limit headers: %w", err)
		}
		
		if resp.StatusCode != 200 {
			return fmt.Errorf("http error: %d - %s", resp.StatusCode, string(resp.Body))
		}
		
		var rawTrades []binanceTradeResponse
		if err := json.Unmarshal(resp.Body, &rawTrades); err != nil {
			return fmt.Errorf("decode response: %w", err)
		}
		
		// Convert to interface format
		trades = make([]interfaces.Trade, len(rawTrades))
		for i, raw := range rawTrades {
			trades[i] = b.convertTrade(symbol, raw)
		}
		
		return nil
	})
	
	return trades, err
}

// FetchKlines implements warm kline fetching
func (b *BinanceAdapter) FetchKlines(ctx context.Context, symbol string, interval string, limit int) ([]interfaces.Kline, error) {
	var klines []interfaces.Kline
	
	err := b.circuitBreaker.Call(ctx, "fetch_klines", func() error {
		if err := b.rateLimiter.Allow(ctx, b.venue, "/api/v3/klines"); err != nil {
			return fmt.Errorf("rate limited: %w", err)
		}
		
		url := fmt.Sprintf("%s/api/v3/klines?symbol=%s&interval=%s&limit=%d", 
			b.baseURL, b.normalizeSymbol(symbol), interval, limit)
		
		req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			return fmt.Errorf("create request: %w", err)
		}
		
		resp, err := b.httpClient.Do(req)
		if err != nil {
			return fmt.Errorf("http request: %w", err)
		}
		defer resp.Body.Close()
		
		if err := b.processRateLimitHeaders(resp.Header); err != nil {
			return fmt.Errorf("process rate limit headers: %w", err)
		}
		
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("http error: %d", resp.StatusCode)
		}
		
		var rawKlines [][]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&rawKlines); err != nil {
			return fmt.Errorf("decode response: %w", err)
		}
		
		klines = make([]interfaces.Kline, len(rawKlines))
		for i, raw := range rawKlines {
			klines[i] = b.convertKline(symbol, interval, raw)
		}
		
		return nil
	})
	
	return klines, err
}

// FetchOrderBook implements warm order book fetching
func (b *BinanceAdapter) FetchOrderBook(ctx context.Context, symbol string) (*interfaces.OrderBookSnapshot, error) {
	var orderBook *interfaces.OrderBookSnapshot
	
	err := b.circuitBreaker.Call(ctx, "fetch_orderbook", func() error {
		if err := b.rateLimiter.Allow(ctx, b.venue, "/api/v3/depth"); err != nil {
			return fmt.Errorf("rate limited: %w", err)
		}
		
		url := fmt.Sprintf("%s/api/v3/depth?symbol=%s&limit=100", 
			b.baseURL, b.normalizeSymbol(symbol))
		
		req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			return fmt.Errorf("create request: %w", err)
		}
		
		resp, err := b.httpClient.Do(req)
		if err != nil {
			return fmt.Errorf("http request: %w", err)
		}
		defer resp.Body.Close()
		
		if err := b.processRateLimitHeaders(resp.Header); err != nil {
			return fmt.Errorf("process rate limit headers: %w", err)
		}
		
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("http error: %d", resp.StatusCode)
		}
		
		var raw binanceOrderBookResponse
		if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
			return fmt.Errorf("decode response: %w", err)
		}
		
		orderBook = b.convertOrderBook(symbol, raw)
		return nil
	})
	
	return orderBook, err
}

// FetchFundingRate implements funding rate fetching
func (b *BinanceAdapter) FetchFundingRate(ctx context.Context, symbol string) (*interfaces.FundingRate, error) {
	var fundingRate *interfaces.FundingRate
	
	err := b.circuitBreaker.Call(ctx, "fetch_funding", func() error {
		if err := b.rateLimiter.Allow(ctx, b.venue, "/fapi/v1/premiumIndex"); err != nil {
			return fmt.Errorf("rate limited: %w", err)
		}
		
		// Use futures API
		url := fmt.Sprintf("https://fapi.binance.com/fapi/v1/premiumIndex?symbol=%s", 
			b.normalizeSymbol(symbol))
		
		req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			return fmt.Errorf("create request: %w", err)
		}
		
		resp, err := b.httpClient.Do(req)
		if err != nil {
			return fmt.Errorf("http request: %w", err)
		}
		defer resp.Body.Close()
		
		if err := b.processRateLimitHeaders(resp.Header); err != nil {
			return fmt.Errorf("process rate limit headers: %w", err)
		}
		
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("http error: %d", resp.StatusCode)
		}
		
		var raw binanceFundingResponse
		if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
			return fmt.Errorf("decode response: %w", err)
		}
		
		fundingRate = b.convertFundingRate(symbol, raw)
		return nil
	})
	
	return fundingRate, err
}

// FetchOpenInterest implements open interest fetching
func (b *BinanceAdapter) FetchOpenInterest(ctx context.Context, symbol string) (*interfaces.OpenInterest, error) {
	var openInterest *interfaces.OpenInterest
	
	err := b.circuitBreaker.Call(ctx, "fetch_openinterest", func() error {
		if err := b.rateLimiter.Allow(ctx, b.venue, "/fapi/v1/openInterest"); err != nil {
			return fmt.Errorf("rate limited: %w", err)
		}
		
		url := fmt.Sprintf("https://fapi.binance.com/fapi/v1/openInterest?symbol=%s", 
			b.normalizeSymbol(symbol))
		
		req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			return fmt.Errorf("create request: %w", err)
		}
		
		resp, err := b.httpClient.Do(req)
		if err != nil {
			return fmt.Errorf("http request: %w", err)
		}
		defer resp.Body.Close()
		
		if err := b.processRateLimitHeaders(resp.Header); err != nil {
			return fmt.Errorf("process rate limit headers: %w", err)
		}
		
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("http error: %d", resp.StatusCode)
		}
		
		var raw binanceOpenInterestResponse
		if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
			return fmt.Errorf("decode response: %w", err)
		}
		
		openInterest = b.convertOpenInterest(symbol, raw)
		return nil
	})
	
	return openInterest, err
}

// HealthCheck performs venue health check
func (b *BinanceAdapter) HealthCheck(ctx context.Context) error {
	return b.circuitBreaker.Call(ctx, "health_check", func() error {
		if err := b.rateLimiter.Allow(ctx, b.venue, "/api/v3/ping"); err != nil {
			return fmt.Errorf("rate limited: %w", err)
		}
		
		url := fmt.Sprintf("%s/api/v3/ping", b.baseURL)
		
		req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			return fmt.Errorf("create request: %w", err)
		}
		
		resp, err := b.httpClient.Do(req)
		if err != nil {
			return fmt.Errorf("http request: %w", err)
		}
		defer resp.Body.Close()
		
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("health check failed: %d", resp.StatusCode)
		}
		
		return nil
	})
}

// Helper methods

func (b *BinanceAdapter) normalizeSymbol(symbol string) string {
	// Convert BTC/USDT to BTCUSDT
	return strings.ReplaceAll(strings.ToUpper(symbol), "/", "")
}

func (b *BinanceAdapter) connectAndStream(ctx context.Context, wsURL, stream string, outputChan interface{}, parser func([]byte, interface{}) error) error {
	b.wsConnsMux.Lock()
	conn, exists := b.wsConns[stream]
	b.wsConnsMux.Unlock()
	
	if !exists {
		var err error
		conn, _, err = websocket.DefaultDialer.Dial(wsURL, nil)
		if err != nil {
			return fmt.Errorf("websocket dial: %w", err)
		}
		
		b.wsConnsMux.Lock()
		b.wsConns[stream] = conn
		b.wsConnsMux.Unlock()
	}
	
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			_, message, err := conn.ReadMessage()
			if err != nil {
				// Connection error, clean up and return to trigger reconnect
				b.wsConnsMux.Lock()
				delete(b.wsConns, stream)
				b.wsConnsMux.Unlock()
				conn.Close()
				return fmt.Errorf("websocket read: %w", err)
			}
			
			if err := parser(message, outputChan); err != nil {
				// Log parse error but continue
				continue
			}
		}
	}
}

func (b *BinanceAdapter) parseTradeEvent(data []byte, outputChan interface{}) error {
	var raw binanceTradeEvent
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	
	tradeChan := outputChan.(chan interfaces.TradeEvent)
	
	price, _ := strconv.ParseFloat(raw.Price, 64)
	quantity, _ := strconv.ParseFloat(raw.Quantity, 64)
	
	event := interfaces.TradeEvent{
		Trade: interfaces.Trade{
			Symbol:    raw.Symbol,
			Venue:     b.venue,
			Price:     price,
			Quantity:  quantity,
			Side:      map[bool]string{true: "buy", false: "sell"}[raw.IsBuyerMaker],
			Timestamp: time.Unix(0, raw.TradeTime*1000000),
			TradeID:   strconv.FormatInt(raw.TradeID, 10),
		},
		EventTime: time.Unix(0, raw.EventTime*1000000),
		IsMaker:   raw.IsBuyerMaker,
	}
	
	select {
	case tradeChan <- event:
	default: // Channel full, drop event
	}
	
	return nil
}

func (b *BinanceAdapter) parseKlineEvent(data []byte, outputChan interface{}) error {
	var raw binanceKlineEvent  
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	
	klineChan := outputChan.(chan interfaces.KlineEvent)
	
	open, _ := strconv.ParseFloat(raw.Kline.Open, 64)
	high, _ := strconv.ParseFloat(raw.Kline.High, 64)
	low, _ := strconv.ParseFloat(raw.Kline.Low, 64)
	close, _ := strconv.ParseFloat(raw.Kline.Close, 64)
	volume, _ := strconv.ParseFloat(raw.Kline.Volume, 64)
	quoteVolume, _ := strconv.ParseFloat(raw.Kline.QuoteAssetVolume, 64)
	
	event := interfaces.KlineEvent{
		Kline: interfaces.Kline{
			Symbol:       raw.Kline.Symbol,
			Venue:        b.venue,
			Interval:     raw.Kline.Interval,
			OpenTime:     time.Unix(0, raw.Kline.OpenTime*1000000),
			CloseTime:    time.Unix(0, raw.Kline.CloseTime*1000000),
			Open:         open,
			High:         high,
			Low:          low,
			Close:        close,
			Volume:       volume,
			QuoteVolume:  quoteVolume,
			TradeCount:   raw.Kline.TradeCount,
		},
		EventTime: time.Unix(0, raw.EventTime*1000000),
		IsClosed:  raw.Kline.IsClosed,
	}
	
	select {
	case klineChan <- event:
	default:
	}
	
	return nil
}

func (b *BinanceAdapter) parseOrderBookEvent(data []byte, outputChan interface{}) error {
	var raw binanceDepthEvent
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	
	orderBookChan := outputChan.(chan interfaces.OrderBookEvent)
	
	// Convert bid/ask arrays
	bids := make([]interfaces.PriceLevel, len(raw.Bids))
	for i, bid := range raw.Bids {
		price, _ := strconv.ParseFloat(bid[0], 64)
		quantity, _ := strconv.ParseFloat(bid[1], 64)
		bids[i] = interfaces.PriceLevel{Price: price, Quantity: quantity}
	}
	
	asks := make([]interfaces.PriceLevel, len(raw.Asks))
	for i, ask := range raw.Asks {
		price, _ := strconv.ParseFloat(ask[0], 64)
		quantity, _ := strconv.ParseFloat(ask[1], 64)
		asks[i] = interfaces.PriceLevel{Price: price, Quantity: quantity}
	}
	
	event := interfaces.OrderBookEvent{
		OrderBookSnapshot: interfaces.OrderBookSnapshot{
			Symbol:       raw.Symbol,
			Venue:        b.venue,
			Timestamp:    time.Now(),
			Bids:         bids,
			Asks:         asks,
			LastUpdateID: raw.FinalUpdateID,
			IsL2:         true,
		},
		EventTime:   time.Unix(0, raw.EventTime*1000000),
		FirstUpdate: raw.FirstUpdateID,
		FinalUpdate: raw.FinalUpdateID,
	}
	
	select {
	case orderBookChan <- event:
	default:
	}
	
	return nil
}

func (b *BinanceAdapter) parseFundingEvent(data []byte, outputChan interface{}) error {
	var raw binanceMarkPriceEvent
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	
	fundingChan := outputChan.(chan interfaces.FundingEvent)
	
	fundingRate, _ := strconv.ParseFloat(raw.FundingRate, 64)
	
	event := interfaces.FundingEvent{
		FundingRate: interfaces.FundingRate{
			Symbol:          raw.Symbol,
			Venue:           b.venue,
			FundingRate:     fundingRate,
			NextFundingTime: time.Unix(0, raw.NextFundingTime*1000000),
			Timestamp:       time.Unix(0, raw.EventTime*1000000),
		},
		EventTime: time.Unix(0, raw.EventTime*1000000),
	}
	
	select {
	case fundingChan <- event:
	default:
	}
	
	return nil
}

func (b *BinanceAdapter) processRateLimitHeaders(headers map[string]string) error {
	return b.rateLimiter.ProcessRateLimitHeaders(b.venue, headers)
}

func (b *BinanceAdapter) convertTrade(symbol string, raw binanceTradeResponse) interfaces.Trade {
	price, _ := strconv.ParseFloat(raw.Price, 64)
	qty, _ := strconv.ParseFloat(raw.Qty, 64)
	
	return interfaces.Trade{
		Symbol:    symbol,
		Venue:     b.venue,
		Price:     price,
		Quantity:  qty,
		Side:      map[bool]string{true: "sell", false: "buy"}[raw.IsBuyerMaker],
		Timestamp: time.Unix(0, raw.Time*1000000),
		TradeID:   strconv.FormatInt(raw.ID, 10),
	}
}

func (b *BinanceAdapter) convertKline(symbol, interval string, raw []interface{}) interfaces.Kline {
	open, _ := strconv.ParseFloat(raw[1].(string), 64)
	high, _ := strconv.ParseFloat(raw[2].(string), 64)
	low, _ := strconv.ParseFloat(raw[3].(string), 64)
	close, _ := strconv.ParseFloat(raw[4].(string), 64)
	volume, _ := strconv.ParseFloat(raw[5].(string), 64)
	quoteVolume, _ := strconv.ParseFloat(raw[7].(string), 64)
	
	openTime := int64(raw[0].(float64))
	closeTime := int64(raw[6].(float64))
	tradeCount := int64(raw[8].(float64))
	
	return interfaces.Kline{
		Symbol:      symbol,
		Venue:       b.venue,
		Interval:    interval,
		OpenTime:    time.Unix(0, openTime*1000000),
		CloseTime:   time.Unix(0, closeTime*1000000),
		Open:        open,
		High:        high,
		Low:         low,
		Close:       close,
		Volume:      volume,
		QuoteVolume: quoteVolume,
		TradeCount:  tradeCount,
	}
}

func (b *BinanceAdapter) convertOrderBook(symbol string, raw binanceOrderBookResponse) *interfaces.OrderBookSnapshot {
	bids := make([]interfaces.PriceLevel, len(raw.Bids))
	for i, bid := range raw.Bids {
		price, _ := strconv.ParseFloat(bid[0], 64)
		quantity, _ := strconv.ParseFloat(bid[1], 64)
		bids[i] = interfaces.PriceLevel{Price: price, Quantity: quantity}
	}
	
	asks := make([]interfaces.PriceLevel, len(raw.Asks))
	for i, ask := range raw.Asks {
		price, _ := strconv.ParseFloat(ask[0], 64)
		quantity, _ := strconv.ParseFloat(ask[1], 64)
		asks[i] = interfaces.PriceLevel{Price: price, Quantity: quantity}
	}
	
	return &interfaces.OrderBookSnapshot{
		Symbol:       symbol,
		Venue:        b.venue,
		Timestamp:    time.Now(),
		Bids:         bids,
		Asks:         asks,
		LastUpdateID: raw.LastUpdateID,
		IsL2:         true,
	}
}

func (b *BinanceAdapter) convertFundingRate(symbol string, raw binanceFundingResponse) *interfaces.FundingRate {
	fundingRate, _ := strconv.ParseFloat(raw.LastFundingRate, 64)
	nextFundingTime := raw.NextFundingTime
	
	return &interfaces.FundingRate{
		Symbol:          symbol,
		Venue:           b.venue,
		FundingRate:     fundingRate,
		NextFundingTime: time.Unix(0, nextFundingTime*1000000),
		Timestamp:       time.Now(),
	}
}

func (b *BinanceAdapter) convertOpenInterest(symbol string, raw binanceOpenInterestResponse) *interfaces.OpenInterest {
	openInterest, _ := strconv.ParseFloat(raw.OpenInterest, 64)
	
	return &interfaces.OpenInterest{
		Symbol:       symbol,
		Venue:        b.venue,
		OpenInterest: openInterest,
		Timestamp:    time.Now(),
	}
}

// Binance API response types

type binanceTradeResponse struct {
	ID           int64  `json:"id"`
	Price        string `json:"price"`
	Qty          string `json:"qty"`
	Time         int64  `json:"time"`
	IsBuyerMaker bool   `json:"isBuyerMaker"`
}

type binanceTradeEvent struct {
	EventType     string `json:"e"`
	EventTime     int64  `json:"E"`
	Symbol        string `json:"s"`
	TradeID       int64  `json:"t"`
	Price         string `json:"p"`
	Quantity      string `json:"q"`
	TradeTime     int64  `json:"T"`
	IsBuyerMaker  bool   `json:"m"`
}

type binanceKlineEvent struct {
	EventType string `json:"e"`
	EventTime int64  `json:"E"`
	Symbol    string `json:"s"`
	Kline     struct {
		Symbol            string `json:"s"`
		OpenTime          int64  `json:"t"`
		CloseTime         int64  `json:"T"`
		Interval          string `json:"i"`
		Open              string `json:"o"`
		Close             string `json:"c"`
		High              string `json:"h"`
		Low               string `json:"l"`
		Volume            string `json:"v"`
		TradeCount        int64  `json:"n"`
		IsClosed          bool   `json:"x"`
		QuoteAssetVolume  string `json:"q"`
		TakerBuyBaseAsset string `json:"V"`
		TakerBuyQuoteAsset string `json:"Q"`
	} `json:"k"`
}

type binanceDepthEvent struct {
	EventType       string     `json:"e"`
	EventTime       int64      `json:"E"`
	Symbol          string     `json:"s"`
	FirstUpdateID   int64      `json:"U"`
	FinalUpdateID   int64      `json:"u"`
	Bids            [][]string `json:"b"`
	Asks            [][]string `json:"a"`
}

type binanceMarkPriceEvent struct {
	EventType       string `json:"e"`
	EventTime       int64  `json:"E"`
	Symbol          string `json:"s"`
	MarkPrice       string `json:"p"`
	FundingRate     string `json:"r"`
	NextFundingTime int64  `json:"T"`
}

type binanceOrderBookResponse struct {
	LastUpdateID int64      `json:"lastUpdateId"`
	Bids         [][]string `json:"bids"`
	Asks         [][]string `json:"asks"`
}

type binanceFundingResponse struct {
	Symbol           string `json:"symbol"`
	LastFundingRate  string `json:"lastFundingRate"`
	NextFundingTime  int64  `json:"nextFundingTime"`
}

type binanceOpenInterestResponse struct {
	Symbol        string `json:"symbol"`
	OpenInterest  string `json:"openInterest"`
	Time          int64  `json:"time"`
}