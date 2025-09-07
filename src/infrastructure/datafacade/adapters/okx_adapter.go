package adapters

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/sawpanic/cryptorun/src/infrastructure/datafacade/interfaces"

	"github.com/gorilla/websocket"
)

// OKXAdapter implements VenueAdapter for OKX exchange
type OKXAdapter struct {
	baseURL     string
	wsURL       string
	rateLimiter interfaces.RateLimiter
	circuitBreaker interfaces.CircuitBreaker
	cache       interfaces.CacheLayer
	
	// WebSocket connections
	wsConnections map[string]*websocket.Conn
	wsLock        sync.RWMutex
	
	// Subscription management
	subscriptions map[string]chan interface{}
	subLock       sync.RWMutex
}

// OKX WebSocket message structures
type okxWSMessage struct {
	Op   string      `json:"op,omitempty"`
	Args []okxWSArg  `json:"args,omitempty"`
	Data interface{} `json:"data,omitempty"`
	Event string     `json:"event,omitempty"`
	Code  string     `json:"code,omitempty"`
	Msg   string     `json:"msg,omitempty"`
}

type okxWSArg struct {
	Channel string `json:"channel"`
	InstID  string `json:"instId"`
}

// OKX API response structures
type okxTradeData struct {
	InstID  string `json:"instId"`
	TradeID string `json:"tradeId"`
	Price   string `json:"px"`
	Size    string `json:"sz"`
	Side    string `json:"side"`
	Ts      string `json:"ts"`
}

type okxKlineData struct {
	InstID string   `json:"instId"`
	Data   []string `json:"data"` // [timestamp, open, high, low, close, volume, volCcy, volCcyQuote, confirm]
	Ts     string   `json:"ts"`
}

type okxOrderBookData struct {
	InstID   string     `json:"instId"`
	Asks     [][]string `json:"asks"`
	Bids     [][]string `json:"bids"`
	Ts       string     `json:"ts"`
	Checksum int        `json:"checksum"`
}

type okxFundingData struct {
	InstID        string `json:"instId"`
	FundingRate   string `json:"fundingRate"`
	NextFundingTime string `json:"nextFundingTime"`
	Ts           string `json:"ts"`
}

type okxOpenInterestData struct {
	InstID string `json:"instId"`
	Oi     string `json:"oi"`
	OiCcy  string `json:"oiCcy"`
	Ts     string `json:"ts"`
}

// NewOKXAdapter creates a new OKX adapter
func NewOKXAdapter(baseURL, wsURL string, rateLimiter interfaces.RateLimiter, 
	circuitBreaker interfaces.CircuitBreaker, cache interfaces.CacheLayer) *OKXAdapter {
	return &OKXAdapter{
		baseURL:        baseURL,
		wsURL:         wsURL,
		rateLimiter:   rateLimiter,
		circuitBreaker: circuitBreaker,
		cache:         cache,
		wsConnections: make(map[string]*websocket.Conn),
		subscriptions: make(map[string]chan interface{}),
	}
}

// GetVenue returns the venue identifier
func (o *OKXAdapter) GetVenue() string {
	return "okx"
}

// WebSocket streaming methods (HOT data)

func (o *OKXAdapter) StreamTrades(ctx context.Context, symbol string) (<-chan interfaces.TradeEvent, error) {
	ch := make(chan interfaces.TradeEvent, 100)
	
	instID := o.normalizeSymbol(symbol)
	channel := "trades"
	key := fmt.Sprintf("%s:%s", channel, instID)
	
	conn, err := o.getWSConnection(ctx, "public")
	if err != nil {
		return nil, fmt.Errorf("get websocket connection: %w", err)
	}
	
	// Subscribe to trades
	subMsg := okxWSMessage{
		Op: "subscribe",
		Args: []okxWSArg{
			{Channel: channel, InstID: instID},
		},
	}
	
	if err := conn.WriteJSON(subMsg); err != nil {
		return nil, fmt.Errorf("subscribe to trades: %w", err)
	}
	
	// Store subscription
	o.subLock.Lock()
	o.subscriptions[key] = make(chan interface{}, 100)
	o.subLock.Unlock()
	
	// Start processing messages
	go o.processTradeMessages(ctx, key, ch)
	
	return ch, nil
}

func (o *OKXAdapter) StreamKlines(ctx context.Context, symbol, interval string) (<-chan interfaces.KlineEvent, error) {
	ch := make(chan interfaces.KlineEvent, 100)
	
	instID := o.normalizeSymbol(symbol)
	okxInterval := o.convertInterval(interval)
	channel := fmt.Sprintf("candle%s", okxInterval)
	key := fmt.Sprintf("%s:%s", channel, instID)
	
	conn, err := o.getWSConnection(ctx, "public")
	if err != nil {
		return nil, fmt.Errorf("get websocket connection: %w", err)
	}
	
	// Subscribe to klines
	subMsg := okxWSMessage{
		Op: "subscribe",
		Args: []okxWSArg{
			{Channel: channel, InstID: instID},
		},
	}
	
	if err := conn.WriteJSON(subMsg); err != nil {
		return nil, fmt.Errorf("subscribe to klines: %w", err)
	}
	
	// Store subscription
	o.subLock.Lock()
	o.subscriptions[key] = make(chan interface{}, 100)
	o.subLock.Unlock()
	
	// Start processing messages
	go o.processKlineMessages(ctx, key, ch, interval)
	
	return ch, nil
}

func (o *OKXAdapter) StreamOrderBook(ctx context.Context, symbol string, depth int) (<-chan interfaces.OrderBookEvent, error) {
	ch := make(chan interfaces.OrderBookEvent, 100)
	
	instID := o.normalizeSymbol(symbol)
	channel := "books" // OKX uses "books" for order book
	key := fmt.Sprintf("%s:%s", channel, instID)
	
	conn, err := o.getWSConnection(ctx, "public")
	if err != nil {
		return nil, fmt.Errorf("get websocket connection: %w", err)
	}
	
	// Subscribe to order book
	subMsg := okxWSMessage{
		Op: "subscribe",
		Args: []okxWSArg{
			{Channel: channel, InstID: instID},
		},
	}
	
	if err := conn.WriteJSON(subMsg); err != nil {
		return nil, fmt.Errorf("subscribe to orderbook: %w", err)
	}
	
	// Store subscription
	o.subLock.Lock()
	o.subscriptions[key] = make(chan interface{}, 100)
	o.subLock.Unlock()
	
	// Start processing messages
	go o.processOrderBookMessages(ctx, key, ch)
	
	return ch, nil
}

func (o *OKXAdapter) StreamFunding(ctx context.Context, symbol string) (<-chan interfaces.FundingEvent, error) {
	ch := make(chan interfaces.FundingEvent, 100)
	
	instID := o.normalizeSymbol(symbol)
	channel := "funding-rate"
	key := fmt.Sprintf("%s:%s", channel, instID)
	
	conn, err := o.getWSConnection(ctx, "public")
	if err != nil {
		return nil, fmt.Errorf("get websocket connection: %w", err)
	}
	
	// Subscribe to funding rates
	subMsg := okxWSMessage{
		Op: "subscribe",
		Args: []okxWSArg{
			{Channel: channel, InstID: instID},
		},
	}
	
	if err := conn.WriteJSON(subMsg); err != nil {
		return nil, fmt.Errorf("subscribe to funding: %w", err)
	}
	
	// Store subscription
	o.subLock.Lock()
	o.subscriptions[key] = make(chan interface{}, 100)
	o.subLock.Unlock()
	
	// Start processing messages
	go o.processFundingMessages(ctx, key, ch)
	
	return ch, nil
}

func (o *OKXAdapter) StreamOpenInterest(ctx context.Context, symbol string) (<-chan interfaces.OpenInterestEvent, error) {
	ch := make(chan interfaces.OpenInterestEvent, 100)
	
	instID := o.normalizeSymbol(symbol)
	channel := "open-interest"
	key := fmt.Sprintf("%s:%s", channel, instID)
	
	conn, err := o.getWSConnection(ctx, "public")
	if err != nil {
		return nil, fmt.Errorf("get websocket connection: %w", err)
	}
	
	// Subscribe to open interest
	subMsg := okxWSMessage{
		Op: "subscribe",
		Args: []okxWSArg{
			{Channel: channel, InstID: instID},
		},
	}
	
	if err := conn.WriteJSON(subMsg); err != nil {
		return nil, fmt.Errorf("subscribe to open interest: %w", err)
	}
	
	// Store subscription
	o.subLock.Lock()
	o.subscriptions[key] = make(chan interface{}, 100)
	o.subLock.Unlock()
	
	// Start processing messages
	go o.processOpenInterestMessages(ctx, key, ch)
	
	return ch, nil
}

// REST API methods (WARM data)

func (o *OKXAdapter) GetTrades(ctx context.Context, symbol string, limit int) ([]interfaces.Trade, error) {
	// TODO: Implement proper caching using CacheLayer interface
	
	// Check circuit breaker
	if err := o.circuitBreaker.Call(ctx, "fetch_trades", func() error {
		return o.rateLimiter.Allow(ctx, "okx", "trades")
	}); err != nil {
		return nil, fmt.Errorf("circuit breaker/rate limiter: %w", err)
	}
	
	instID := o.normalizeSymbol(symbol)
	endpoint := fmt.Sprintf("/api/v5/market/trades?instId=%s&limit=%d", instID, limit)
	
	var result struct {
		Code string        `json:"code"`
		Msg  string        `json:"msg"`
		Data []okxTradeData `json:"data"`
	}
	
	if err := o.makeRequest(ctx, "GET", endpoint, nil, &result); err != nil {
		return nil, fmt.Errorf("make request: %w", err)
	}
	
	if result.Code != "0" {
		return nil, fmt.Errorf("OKX API error: %s - %s", result.Code, result.Msg)
	}
	
	// Convert to interface trades
	trades := make([]interfaces.Trade, len(result.Data))
	for i, trade := range result.Data {
		price, _ := strconv.ParseFloat(trade.Price, 64)
		quantity, _ := strconv.ParseFloat(trade.Size, 64)
		timestamp, _ := strconv.ParseInt(trade.Ts, 10, 64)
		
		trades[i] = interfaces.Trade{
			TradeID:   trade.TradeID,
			Symbol:    symbol,
			Price:     price,
			Quantity:  quantity,
			Side:      trade.Side,
			Timestamp: time.Unix(0, timestamp*1000000), // Convert ms to nanoseconds
			Venue:     "okx",
		}
	}
	
	// TODO: Implement proper caching using CacheLayer interface
	
	return trades, nil
}

func (o *OKXAdapter) GetKlines(ctx context.Context, symbol, interval string, limit int) ([]interfaces.Kline, error) {
	// TODO: Implement proper caching using CacheLayer interface
	
	// Check circuit breaker
	if err := o.circuitBreaker.Call(ctx, "fetch_klines", func() error {
		return o.rateLimiter.Allow(ctx, "okx", "klines")
	}); err != nil {
		return nil, fmt.Errorf("circuit breaker/rate limiter: %w", err)
	}
	
	instID := o.normalizeSymbol(symbol)
	okxInterval := o.convertInterval(interval)
	endpoint := fmt.Sprintf("/api/v5/market/candles?instId=%s&bar=%s&limit=%d", instID, okxInterval, limit)
	
	var result struct {
		Code string     `json:"code"`
		Msg  string     `json:"msg"`
		Data [][]string `json:"data"`
	}
	
	if err := o.makeRequest(ctx, "GET", endpoint, nil, &result); err != nil {
		return nil, fmt.Errorf("make request: %w", err)
	}
	
	if result.Code != "0" {
		return nil, fmt.Errorf("OKX API error: %s - %s", result.Code, result.Msg)
	}
	
	// Convert to interface klines
	klines := make([]interfaces.Kline, len(result.Data))
	for i, data := range result.Data {
		if len(data) < 6 {
			continue
		}
		
		timestamp, _ := strconv.ParseInt(data[0], 10, 64)
		open, _ := strconv.ParseFloat(data[1], 64)
		high, _ := strconv.ParseFloat(data[2], 64)
		low, _ := strconv.ParseFloat(data[3], 64)
		close, _ := strconv.ParseFloat(data[4], 64)
		volume, _ := strconv.ParseFloat(data[5], 64)
		
		klines[i] = interfaces.Kline{
			Symbol:    symbol,
			Interval:  interval,
			OpenTime:  time.Unix(0, timestamp*1000000), // Convert ms to nanoseconds
			CloseTime: time.Unix(0, timestamp*1000000).Add(o.getIntervalDuration(interval)),
			Open:      open,
			High:      high,
			Low:       low,
			Close:     close,
			Volume:    volume,
			Venue:     "okx",
		}
	}
	
	// TODO: Implement proper caching using CacheLayer interface
	
	return klines, nil
}

func (o *OKXAdapter) GetOrderBook(ctx context.Context, symbol string, depth int) (*interfaces.OrderBookSnapshot, error) {
	// TODO: Implement proper caching using CacheLayer interface
	
	// Check circuit breaker
	if err := o.circuitBreaker.Call(ctx, "fetch_orderbook", func() error {
		return o.rateLimiter.Allow(ctx, "okx", "orderbook")
	}); err != nil {
		return nil, fmt.Errorf("circuit breaker/rate limiter: %w", err)
	}
	
	instID := o.normalizeSymbol(symbol)
	endpoint := fmt.Sprintf("/api/v5/market/books?instId=%s&sz=%d", instID, depth)
	
	var result struct {
		Code string            `json:"code"`
		Msg  string            `json:"msg"`
		Data []okxOrderBookData `json:"data"`
	}
	
	if err := o.makeRequest(ctx, "GET", endpoint, nil, &result); err != nil {
		return nil, fmt.Errorf("make request: %w", err)
	}
	
	if result.Code != "0" || len(result.Data) == 0 {
		return nil, fmt.Errorf("OKX API error: %s - %s", result.Code, result.Msg)
	}
	
	data := result.Data[0]
	timestamp, _ := strconv.ParseInt(data.Ts, 10, 64)
	
	// Convert to interface order book
	orderBook := &interfaces.OrderBookSnapshot{
		Symbol:    symbol,
		Venue:     "okx",
		Timestamp: time.Unix(0, timestamp*1000000), // Convert ms to nanoseconds
		Bids:      make([]interfaces.OrderBookLevel, 0, len(data.Bids)),
		Asks:      make([]interfaces.OrderBookLevel, 0, len(data.Asks)),
	}
	
	// Process bids
	for _, bid := range data.Bids {
		if len(bid) >= 2 {
			price, _ := strconv.ParseFloat(bid[0], 64)
			quantity, _ := strconv.ParseFloat(bid[1], 64)
			orderBook.Bids = append(orderBook.Bids, interfaces.OrderBookLevel{
				Price:    price,
				Quantity: quantity,
			})
		}
	}
	
	// Process asks
	for _, ask := range data.Asks {
		if len(ask) >= 2 {
			price, _ := strconv.ParseFloat(ask[0], 64)
			quantity, _ := strconv.ParseFloat(ask[1], 64)
			orderBook.Asks = append(orderBook.Asks, interfaces.OrderBookLevel{
				Price:    price,
				Quantity: quantity,
			})
		}
	}
	
	// TODO: Implement proper caching using CacheLayer interface
	
	return orderBook, nil
}

func (o *OKXAdapter) GetFunding(ctx context.Context, symbol string) (*interfaces.FundingRate, error) {
	// TODO: Implement proper caching using CacheLayer interface
	
	// Check circuit breaker
	if err := o.circuitBreaker.Call(ctx, "fetch_funding", func() error {
		return o.rateLimiter.Allow(ctx, "okx", "funding")
	}); err != nil {
		return nil, fmt.Errorf("circuit breaker/rate limiter: %w", err)
	}
	
	instID := o.normalizeSymbol(symbol)
	endpoint := fmt.Sprintf("/api/v5/public/funding-rate?instId=%s", instID)
	
	var result struct {
		Code string           `json:"code"`
		Msg  string           `json:"msg"`
		Data []okxFundingData `json:"data"`
	}
	
	if err := o.makeRequest(ctx, "GET", endpoint, nil, &result); err != nil {
		return nil, fmt.Errorf("make request: %w", err)
	}
	
	if result.Code != "0" || len(result.Data) == 0 {
		return nil, fmt.Errorf("OKX API error: %s - %s", result.Code, result.Msg)
	}
	
	data := result.Data[0]
	fundingRate, _ := strconv.ParseFloat(data.FundingRate, 64)
	timestamp, _ := strconv.ParseInt(data.Ts, 10, 64)
	nextFundingTime, _ := strconv.ParseInt(data.NextFundingTime, 10, 64)
	
	funding := &interfaces.FundingRate{
		Symbol:          symbol,
		Venue:           "okx",
		FundingRate:     fundingRate,
		Timestamp:       time.Unix(0, timestamp*1000000), // Convert ms to nanoseconds
		NextFundingTime: time.Unix(0, nextFundingTime*1000000),
	}
	
	// TODO: Implement proper caching using CacheLayer interface
	
	return funding, nil
}

func (o *OKXAdapter) GetOpenInterest(ctx context.Context, symbol string) (*interfaces.OpenInterest, error) {
	// TODO: Implement proper caching using CacheLayer interface
	
	// Check circuit breaker
	if err := o.circuitBreaker.Call(ctx, "fetch_openinterest", func() error {
		return o.rateLimiter.Allow(ctx, "okx", "openinterest")
	}); err != nil {
		return nil, fmt.Errorf("circuit breaker/rate limiter: %w", err)
	}
	
	instID := o.normalizeSymbol(symbol)
	endpoint := fmt.Sprintf("/api/v5/public/open-interest?instId=%s", instID)
	
	var result struct {
		Code string                `json:"code"`
		Msg  string                `json:"msg"`
		Data []okxOpenInterestData `json:"data"`
	}
	
	if err := o.makeRequest(ctx, "GET", endpoint, nil, &result); err != nil {
		return nil, fmt.Errorf("make request: %w", err)
	}
	
	if result.Code != "0" || len(result.Data) == 0 {
		return nil, fmt.Errorf("OKX API error: %s - %s", result.Code, result.Msg)
	}
	
	data := result.Data[0]
	openInterest, _ := strconv.ParseFloat(data.Oi, 64)
	timestamp, _ := strconv.ParseInt(data.Ts, 10, 64)
	
	oi := &interfaces.OpenInterest{
		Symbol:       symbol,
		Venue:        "okx",
		OpenInterest: openInterest,
		Timestamp:    time.Unix(0, timestamp*1000000), // Convert ms to nanoseconds
	}
	
	// TODO: Implement proper caching using CacheLayer interface
	
	return oi, nil
}

// Helper methods

func (o *OKXAdapter) normalizeSymbol(symbol string) string {
	// Convert BTC/USDT to BTC-USDT-SWAP for perpetual futures
	if strings.Contains(symbol, "/") {
		parts := strings.Split(symbol, "/")
		if len(parts) == 2 {
			return fmt.Sprintf("%s-%s-SWAP", parts[0], parts[1])
		}
	}
	return strings.ToUpper(symbol)
}

func (o *OKXAdapter) convertInterval(interval string) string {
	// Convert standard intervals to OKX format
	intervalMap := map[string]string{
		"1m":  "1m",
		"5m":  "5m",
		"15m": "15m",
		"1h":  "1H",
		"4h":  "4H",
		"1d":  "1D",
	}
	
	if okxInterval, exists := intervalMap[interval]; exists {
		return okxInterval
	}
	return interval
}

func (o *OKXAdapter) getIntervalDuration(interval string) time.Duration {
	switch interval {
	case "1m":
		return time.Minute
	case "5m":
		return 5 * time.Minute
	case "15m":
		return 15 * time.Minute
	case "1h":
		return time.Hour
	case "4h":
		return 4 * time.Hour
	case "1d":
		return 24 * time.Hour
	default:
		return time.Minute
	}
}

func (o *OKXAdapter) getWSConnection(ctx context.Context, connType string) (*websocket.Conn, error) {
	o.wsLock.RLock()
	conn, exists := o.wsConnections[connType]
	o.wsLock.RUnlock()
	
	if exists && conn != nil {
		return conn, nil
	}
	
	// Create new connection
	o.wsLock.Lock()
	defer o.wsLock.Unlock()
	
	// Double-check after acquiring write lock
	if conn, exists := o.wsConnections[connType]; exists && conn != nil {
		return conn, nil
	}
	
	wsURL := o.wsURL + "/ws/v5/public"
	
	conn, _, err := websocket.DefaultDialer.DialContext(ctx, wsURL, nil)
	if err != nil {
		return nil, fmt.Errorf("dial websocket: %w", err)
	}
	
	o.wsConnections[connType] = conn
	
	// Start reading messages
	go o.readWSMessages(ctx, conn, connType)
	
	return conn, nil
}

func (o *OKXAdapter) readWSMessages(ctx context.Context, conn *websocket.Conn, connType string) {
	defer func() {
		o.wsLock.Lock()
		delete(o.wsConnections, connType)
		o.wsLock.Unlock()
		conn.Close()
	}()
	
	for {
		select {
		case <-ctx.Done():
			return
		default:
			var msg okxWSMessage
			if err := conn.ReadJSON(&msg); err != nil {
				// Handle reconnection logic here
				return
			}
			
			o.routeWSMessage(msg)
		}
	}
}

func (o *OKXAdapter) routeWSMessage(msg okxWSMessage) {
	if len(msg.Args) == 0 {
		return
	}
	
	arg := msg.Args[0]
	key := fmt.Sprintf("%s:%s", arg.Channel, arg.InstID)
	
	o.subLock.RLock()
	ch, exists := o.subscriptions[key]
	o.subLock.RUnlock()
	
	if exists {
		select {
		case ch <- msg.Data:
		default:
			// Channel is full, drop message
		}
	}
}

func (o *OKXAdapter) processTradeMessages(ctx context.Context, key string, out chan<- interfaces.TradeEvent) {
	o.subLock.RLock()
	ch := o.subscriptions[key]
	o.subLock.RUnlock()
	
	defer close(out)
	
	for {
		select {
		case <-ctx.Done():
			return
		case data := <-ch:
			if trades, ok := data.([]interface{}); ok {
				for _, tradeData := range trades {
					if tradeMap, ok := tradeData.(map[string]interface{}); ok {
						event := o.convertTradeData(tradeMap)
						if event != nil {
							select {
							case out <- *event:
							case <-ctx.Done():
								return
							}
						}
					}
				}
			}
		}
	}
}

func (o *OKXAdapter) processKlineMessages(ctx context.Context, key string, out chan<- interfaces.KlineEvent, interval string) {
	o.subLock.RLock()
	ch := o.subscriptions[key]
	o.subLock.RUnlock()
	
	defer close(out)
	
	for {
		select {
		case <-ctx.Done():
			return
		case data := <-ch:
			if klines, ok := data.([]interface{}); ok {
				for _, klineData := range klines {
					if klineSlice, ok := klineData.([]interface{}); ok {
						event := o.convertKlineData(klineSlice, interval)
						if event != nil {
							select {
							case out <- *event:
							case <-ctx.Done():
								return
							}
						}
					}
				}
			}
		}
	}
}

func (o *OKXAdapter) processOrderBookMessages(ctx context.Context, key string, out chan<- interfaces.OrderBookEvent) {
	o.subLock.RLock()
	ch := o.subscriptions[key]
	o.subLock.RUnlock()
	
	defer close(out)
	
	for {
		select {
		case <-ctx.Done():
			return
		case data := <-ch:
			if books, ok := data.([]interface{}); ok {
				for _, bookData := range books {
					if bookMap, ok := bookData.(map[string]interface{}); ok {
						event := o.convertOrderBookData(bookMap)
						if event != nil {
							select {
							case out <- *event:
							case <-ctx.Done():
								return
							}
						}
					}
				}
			}
		}
	}
}

func (o *OKXAdapter) processFundingMessages(ctx context.Context, key string, out chan<- interfaces.FundingEvent) {
	o.subLock.RLock()
	ch := o.subscriptions[key]
	o.subLock.RUnlock()
	
	defer close(out)
	
	for {
		select {
		case <-ctx.Done():
			return
		case data := <-ch:
			if fundings, ok := data.([]interface{}); ok {
				for _, fundingData := range fundings {
					if fundingMap, ok := fundingData.(map[string]interface{}); ok {
						event := o.convertFundingData(fundingMap)
						if event != nil {
							select {
							case out <- *event:
							case <-ctx.Done():
								return
							}
						}
					}
				}
			}
		}
	}
}

func (o *OKXAdapter) processOpenInterestMessages(ctx context.Context, key string, out chan<- interfaces.OpenInterestEvent) {
	o.subLock.RLock()
	ch := o.subscriptions[key]
	o.subLock.RUnlock()
	
	defer close(out)
	
	for {
		select {
		case <-ctx.Done():
			return
		case data := <-ch:
			if ois, ok := data.([]interface{}); ok {
				for _, oiData := range ois {
					if oiMap, ok := oiData.(map[string]interface{}); ok {
						event := o.convertOpenInterestData(oiMap)
						if event != nil {
							select {
							case out <- *event:
							case <-ctx.Done():
								return
							}
						}
					}
				}
			}
		}
	}
}

// Data conversion helpers

func (o *OKXAdapter) convertTradeData(data map[string]interface{}) *interfaces.TradeEvent {
	instID, _ := data["instId"].(string)
	tradeID, _ := data["tradeId"].(string)
	price, _ := strconv.ParseFloat(data["px"].(string), 64)
	quantity, _ := strconv.ParseFloat(data["sz"].(string), 64)
	side, _ := data["side"].(string)
	ts, _ := strconv.ParseInt(data["ts"].(string), 10, 64)
	
	return &interfaces.TradeEvent{
		Trade: interfaces.Trade{
			TradeID:   tradeID,
			Symbol:    o.denormalizeSymbol(instID),
			Price:     price,
			Quantity:  quantity,
			Side:      side,
			Timestamp: time.Unix(0, ts*1000000), // Convert ms to nanoseconds
			Venue:     "okx",
		},
		EventTime: time.Now(),
	}
}

func (o *OKXAdapter) convertKlineData(data []interface{}, interval string) *interfaces.KlineEvent {
	if len(data) < 6 {
		return nil
	}
	
	timestamp, _ := strconv.ParseInt(data[0].(string), 10, 64)
	open, _ := strconv.ParseFloat(data[1].(string), 64)
	high, _ := strconv.ParseFloat(data[2].(string), 64)
	low, _ := strconv.ParseFloat(data[3].(string), 64)
	close, _ := strconv.ParseFloat(data[4].(string), 64)
	volume, _ := strconv.ParseFloat(data[5].(string), 64)
	
	return &interfaces.KlineEvent{
		Kline: interfaces.Kline{
			Symbol:    o.denormalizeSymbol(data[0].(string)), // This would need the instId
			Interval:  interval,
			OpenTime:  time.Unix(0, timestamp*1000000), // Convert ms to nanoseconds
			CloseTime: time.Unix(0, timestamp*1000000).Add(o.getIntervalDuration(interval)),
			Open:      open,
			High:      high,
			Low:       low,
			Close:     close,
			Volume:    volume,
			Venue:     "okx",
		},
		EventTime: time.Now(),
	}
}

func (o *OKXAdapter) convertOrderBookData(data map[string]interface{}) *interfaces.OrderBookEvent {
	instID, _ := data["instId"].(string)
	ts, _ := strconv.ParseInt(data["ts"].(string), 10, 64)
	
	snapshot := &interfaces.OrderBookSnapshot{
		Symbol:    o.denormalizeSymbol(instID),
		Venue:     "okx",
		Timestamp: time.Unix(0, ts*1000000), // Convert ms to nanoseconds
	}
	
	// Process bids
	if bids, ok := data["bids"].([]interface{}); ok {
		for _, bid := range bids {
			if bidSlice, ok := bid.([]interface{}); ok && len(bidSlice) >= 2 {
				price, _ := strconv.ParseFloat(bidSlice[0].(string), 64)
				quantity, _ := strconv.ParseFloat(bidSlice[1].(string), 64)
				snapshot.Bids = append(snapshot.Bids, interfaces.OrderBookLevel{
					Price:    price,
					Quantity: quantity,
				})
			}
		}
	}
	
	// Process asks
	if asks, ok := data["asks"].([]interface{}); ok {
		for _, ask := range asks {
			if askSlice, ok := ask.([]interface{}); ok && len(askSlice) >= 2 {
				price, _ := strconv.ParseFloat(askSlice[0].(string), 64)
				quantity, _ := strconv.ParseFloat(askSlice[1].(string), 64)
				snapshot.Asks = append(snapshot.Asks, interfaces.OrderBookLevel{
					Price:    price,
					Quantity: quantity,
				})
			}
		}
	}
	
	return &interfaces.OrderBookEvent{
		OrderBookSnapshot: *snapshot,
		EventTime: time.Now(),
	}
}

func (o *OKXAdapter) convertFundingData(data map[string]interface{}) *interfaces.FundingEvent {
	instID, _ := data["instId"].(string)
	fundingRate, _ := strconv.ParseFloat(data["fundingRate"].(string), 64)
	ts, _ := strconv.ParseInt(data["ts"].(string), 10, 64)
	nextFundingTime, _ := strconv.ParseInt(data["nextFundingTime"].(string), 10, 64)
	
	return &interfaces.FundingEvent{
		FundingRate: interfaces.FundingRate{
			Symbol:          o.denormalizeSymbol(instID),
			Venue:           "okx",
			FundingRate:     fundingRate,
			Timestamp:       time.Unix(0, ts*1000000), // Convert ms to nanoseconds
			NextFundingTime: time.Unix(0, nextFundingTime*1000000),
		},
		EventTime: time.Now(),
	}
}

func (o *OKXAdapter) convertOpenInterestData(data map[string]interface{}) *interfaces.OpenInterestEvent {
	instID, _ := data["instId"].(string)
	openInterest, _ := strconv.ParseFloat(data["oi"].(string), 64)
	ts, _ := strconv.ParseInt(data["ts"].(string), 10, 64)
	
	return &interfaces.OpenInterestEvent{
		OpenInterest: interfaces.OpenInterest{
			Symbol:       o.denormalizeSymbol(instID),
			Venue:        "okx",
			OpenInterest: openInterest,
			Timestamp:    time.Unix(0, ts*1000000), // Convert ms to nanoseconds
		},
		EventTime: time.Now(),
	}
}

func (o *OKXAdapter) denormalizeSymbol(instID string) string {
	// Convert BTC-USDT-SWAP back to BTC/USDT
	if strings.HasSuffix(instID, "-SWAP") {
		parts := strings.Split(instID, "-")
		if len(parts) >= 2 {
			return fmt.Sprintf("%s/%s", parts[0], parts[1])
		}
	}
	return instID
}

func (o *OKXAdapter) makeRequest(ctx context.Context, method, endpoint string, body interface{}, result interface{}) error {
	// This would implement the actual HTTP request logic
	// For now, return a mock implementation
	return nil
}

// HealthCheck performs venue health check
func (o *OKXAdapter) HealthCheck(ctx context.Context) error {
	// Check circuit breaker
	return o.circuitBreaker.Call(ctx, "health_check", func() error {
		if err := o.rateLimiter.Allow(ctx, "okx", "ping"); err != nil {
			return fmt.Errorf("rate limited: %w", err)
		}
		
		// Simple health check - could call /api/v5/system/status
		return nil
	})
}

// IsSupported checks if a data type is supported
func (o *OKXAdapter) IsSupported(dataType interfaces.DataType) bool {
	supported := map[interfaces.DataType]bool{
		interfaces.DataTypeTrades:       true,
		interfaces.DataTypeKlines:       true,
		interfaces.DataTypeOrderBook:    true,
		interfaces.DataTypeFunding:      true,
		interfaces.DataTypeOpenInterest: true,
	}
	return supported[dataType]
}