package adapters

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/sawpanic/cryptorun/src/infrastructure/datafacade/interfaces"

	"github.com/gorilla/websocket"
)

// CoinbaseAdapter implements VenueAdapter for Coinbase Advanced Trade
type CoinbaseAdapter struct {
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

// Coinbase WebSocket message structures
type coinbaseWSMessage struct {
	Type      string                 `json:"type"`
	ProductID string                 `json:"product_id,omitempty"`
	Channel   string                 `json:"channel,omitempty"`
	Events    []coinbaseEvent        `json:"events,omitempty"`
	Subscribe map[string]interface{} `json:"subscribe,omitempty"`
	Timestamp string                 `json:"timestamp,omitempty"`
}

type coinbaseEvent struct {
	Type      string      `json:"type"`
	ProductID string      `json:"product_id"`
	Updates   interface{} `json:"updates,omitempty"`
	Tickers   interface{} `json:"tickers,omitempty"`
	Candles   interface{} `json:"candles,omitempty"`
}

// Coinbase API response structures
type coinbaseTradeData struct {
	TradeID   string `json:"trade_id"`
	ProductID string `json:"product_id"`
	Price     string `json:"price"`
	Size      string `json:"size"`
	Side      string `json:"side"`
	Time      string `json:"time"`
}

type coinbaseKlineData struct {
	Start  string `json:"start"`
	Low    string `json:"low"`
	High   string `json:"high"`
	Open   string `json:"open"`
	Close  string `json:"close"`
	Volume string `json:"volume"`
}

type coinbaseOrderBookData struct {
	ProductID string     `json:"product_id"`
	Bids      [][]string `json:"bids"`
	Asks      [][]string `json:"asks"`
	Time      string     `json:"time"`
}

// NewCoinbaseAdapter creates a new Coinbase adapter
func NewCoinbaseAdapter(baseURL, wsURL string, rateLimiter interfaces.RateLimiter, 
	circuitBreaker interfaces.CircuitBreaker, cache interfaces.CacheLayer) *CoinbaseAdapter {
	return &CoinbaseAdapter{
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
func (c *CoinbaseAdapter) GetVenue() string {
	return "coinbase"
}

// WebSocket streaming methods (HOT data)

func (c *CoinbaseAdapter) StreamTrades(ctx context.Context, symbol string) (<-chan interfaces.TradeEvent, error) {
	ch := make(chan interfaces.TradeEvent, 100)
	
	productID := c.normalizeSymbol(symbol)
	channel := "market_trades"
	key := fmt.Sprintf("%s:%s", channel, productID)
	
	conn, err := c.getWSConnection(ctx, "public")
	if err != nil {
		return nil, fmt.Errorf("get websocket connection: %w", err)
	}
	
	// Subscribe to trades
	subMsg := coinbaseWSMessage{
		Type:    "subscribe",
		Channel: channel,
		Subscribe: map[string]interface{}{
			"product_ids": []string{productID},
		},
	}
	
	if err := conn.WriteJSON(subMsg); err != nil {
		return nil, fmt.Errorf("subscribe to trades: %w", err)
	}
	
	// Store subscription
	c.subLock.Lock()
	c.subscriptions[key] = make(chan interface{}, 100)
	c.subLock.Unlock()
	
	// Start processing messages
	go c.processTradeMessages(ctx, key, ch)
	
	return ch, nil
}

func (c *CoinbaseAdapter) StreamKlines(ctx context.Context, symbol, interval string) (<-chan interfaces.KlineEvent, error) {
	ch := make(chan interfaces.KlineEvent, 100)
	
	productID := c.normalizeSymbol(symbol)
	granularity := c.convertInterval(interval)
	channel := "candles"
	key := fmt.Sprintf("%s:%s:%s", channel, productID, granularity)
	
	conn, err := c.getWSConnection(ctx, "public")
	if err != nil {
		return nil, fmt.Errorf("get websocket connection: %w", err)
	}
	
	// Subscribe to candles
	subMsg := coinbaseWSMessage{
		Type:    "subscribe",
		Channel: channel,
		Subscribe: map[string]interface{}{
			"product_ids": []string{productID},
		},
	}
	
	if err := conn.WriteJSON(subMsg); err != nil {
		return nil, fmt.Errorf("subscribe to klines: %w", err)
	}
	
	// Store subscription
	c.subLock.Lock()
	c.subscriptions[key] = make(chan interface{}, 100)
	c.subLock.Unlock()
	
	// Start processing messages
	go c.processKlineMessages(ctx, key, ch, interval)
	
	return ch, nil
}

func (c *CoinbaseAdapter) StreamOrderBook(ctx context.Context, symbol string, depth int) (<-chan interfaces.OrderBookEvent, error) {
	ch := make(chan interfaces.OrderBookEvent, 100)
	
	productID := c.normalizeSymbol(symbol)
	channel := "level2"
	key := fmt.Sprintf("%s:%s", channel, productID)
	
	conn, err := c.getWSConnection(ctx, "public")
	if err != nil {
		return nil, fmt.Errorf("get websocket connection: %w", err)
	}
	
	// Subscribe to level2 order book
	subMsg := coinbaseWSMessage{
		Type:    "subscribe",
		Channel: channel,
		Subscribe: map[string]interface{}{
			"product_ids": []string{productID},
		},
	}
	
	if err := conn.WriteJSON(subMsg); err != nil {
		return nil, fmt.Errorf("subscribe to orderbook: %w", err)
	}
	
	// Store subscription
	c.subLock.Lock()
	c.subscriptions[key] = make(chan interface{}, 100)
	c.subLock.Unlock()
	
	// Start processing messages
	go c.processOrderBookMessages(ctx, key, ch)
	
	return ch, nil
}

func (c *CoinbaseAdapter) StreamFunding(ctx context.Context, symbol string) (<-chan interfaces.FundingEvent, error) {
	// Coinbase doesn't have perpetual futures, so funding rates don't apply
	// Return a closed channel to indicate no funding data
	ch := make(chan interfaces.FundingEvent)
	close(ch)
	return ch, nil
}

func (c *CoinbaseAdapter) StreamOpenInterest(ctx context.Context, symbol string) (<-chan interfaces.OpenInterestEvent, error) {
	// Coinbase doesn't have perpetual futures, so open interest doesn't apply
	// Return a closed channel to indicate no open interest data
	ch := make(chan interfaces.OpenInterestEvent)
	close(ch)
	return ch, nil
}

// REST API methods (WARM data)

func (c *CoinbaseAdapter) GetTrades(ctx context.Context, symbol string, limit int) ([]interfaces.Trade, error) {
	// Check cache first
	if trades, found, err := c.cache.GetCachedTrades(ctx, "coinbase", symbol); err == nil && found {
		return trades, nil
	}
	
	// Check circuit breaker
	if err := c.circuitBreaker.Call(ctx, "fetch_trades", func() error {
		return c.rateLimiter.Allow(ctx, "coinbase", "trades")
	}); err != nil {
		return nil, fmt.Errorf("circuit breaker/rate limiter: %w", err)
	}
	
	productID := c.normalizeSymbol(symbol)
	endpoint := fmt.Sprintf("/api/v3/brokerage/market/products/%s/trades?limit=%d", productID, limit)
	
	var result struct {
		Trades []coinbaseTradeData `json:"trades"`
	}
	
	if err := c.makeRequest(ctx, "GET", endpoint, nil, &result); err != nil {
		return nil, fmt.Errorf("make request: %w", err)
	}
	
	// Convert to interface trades
	trades := make([]interfaces.Trade, len(result.Trades))
	for i, trade := range result.Trades {
		price, _ := strconv.ParseFloat(trade.Price, 64)
		quantity, _ := strconv.ParseFloat(trade.Size, 64)
		timestamp, _ := time.Parse(time.RFC3339, trade.Time)
		
		trades[i] = interfaces.Trade{
			ID:        trade.TradeID,
			Symbol:    symbol,
			Price:     price,
			Quantity:  quantity,
			Side:      trade.Side,
			Timestamp: timestamp,
			Venue:     "coinbase",
		}
	}
	
	// Cache the result
	c.cache.CacheTrades(ctx, "coinbase", symbol, trades, 30*time.Second)
	
	return trades, nil
}

func (c *CoinbaseAdapter) GetKlines(ctx context.Context, symbol, interval string, limit int) ([]interfaces.Kline, error) {
	// Check cache first
	if klines, found, err := c.cache.GetCachedKlines(ctx, "coinbase", symbol, interval); err == nil && found {
		return klines, nil
	}
	
	// Check circuit breaker
	if err := c.circuitBreaker.Call(ctx, "fetch_klines", func() error {
		return c.rateLimiter.Allow(ctx, "coinbase", "klines")
	}); err != nil {
		return nil, fmt.Errorf("circuit breaker/rate limiter: %w", err)
	}
	
	productID := c.normalizeSymbol(symbol)
	granularity := c.convertInterval(interval)
	
	// Calculate start and end times for the requested limit
	endTime := time.Now()
	duration := c.getIntervalDuration(interval)
	startTime := endTime.Add(-time.Duration(limit) * duration)
	
	endpoint := fmt.Sprintf("/api/v3/brokerage/market/products/%s/candles?start=%d&end=%d&granularity=%s", 
		productID, startTime.Unix(), endTime.Unix(), granularity)
	
	var result struct {
		Candles []coinbaseKlineData `json:"candles"`
	}
	
	if err := c.makeRequest(ctx, "GET", endpoint, nil, &result); err != nil {
		return nil, fmt.Errorf("make request: %w", err)
	}
	
	// Convert to interface klines
	klines := make([]interfaces.Kline, len(result.Candles))
	for i, candle := range result.Candles {
		startTime, _ := time.Parse(time.RFC3339, candle.Start)
		open, _ := strconv.ParseFloat(candle.Open, 64)
		high, _ := strconv.ParseFloat(candle.High, 64)
		low, _ := strconv.ParseFloat(candle.Low, 64)
		close, _ := strconv.ParseFloat(candle.Close, 64)
		volume, _ := strconv.ParseFloat(candle.Volume, 64)
		
		klines[i] = interfaces.Kline{
			Symbol:    symbol,
			Interval:  interval,
			OpenTime:  startTime,
			CloseTime: startTime.Add(c.getIntervalDuration(interval)),
			Open:      open,
			High:      high,
			Low:       low,
			Close:     close,
			Volume:    volume,
			Venue:     "coinbase",
		}
	}
	
	// Cache the result
	c.cache.CacheKlines(ctx, "coinbase", symbol, interval, klines, 60*time.Second)
	
	return klines, nil
}

func (c *CoinbaseAdapter) GetOrderBook(ctx context.Context, symbol string, depth int) (*interfaces.OrderBookSnapshot, error) {
	// Check cache first
	if orderBook, found, err := c.cache.GetCachedOrderBook(ctx, "coinbase", symbol); err == nil && found {
		return orderBook, nil
	}
	
	// Check circuit breaker
	if err := c.circuitBreaker.Call(ctx, "fetch_orderbook", func() error {
		return c.rateLimiter.Allow(ctx, "coinbase", "orderbook")
	}); err != nil {
		return nil, fmt.Errorf("circuit breaker/rate limiter: %w", err)
	}
	
	productID := c.normalizeSymbol(symbol)
	endpoint := fmt.Sprintf("/api/v3/brokerage/market/products/%s/book?limit=%d", productID, depth)
	
	var result struct {
		PriceBook coinbaseOrderBookData `json:"pricebook"`
	}
	
	if err := c.makeRequest(ctx, "GET", endpoint, nil, &result); err != nil {
		return nil, fmt.Errorf("make request: %w", err)
	}
	
	data := result.PriceBook
	timestamp, _ := time.Parse(time.RFC3339, data.Time)
	
	// Convert to interface order book
	orderBook := &interfaces.OrderBookSnapshot{
		Symbol:    symbol,
		Venue:     "coinbase",
		Timestamp: timestamp,
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
	
	// Cache the result
	c.cache.CacheOrderBook(ctx, "coinbase", symbol, orderBook, 5*time.Second)
	
	return orderBook, nil
}

func (c *CoinbaseAdapter) GetFunding(ctx context.Context, symbol string) (*interfaces.FundingRate, error) {
	// Coinbase doesn't have perpetual futures, so no funding rates
	return nil, fmt.Errorf("funding rates not supported by coinbase (spot-only exchange)")
}

func (c *CoinbaseAdapter) GetOpenInterest(ctx context.Context, symbol string) (*interfaces.OpenInterest, error) {
	// Coinbase doesn't have perpetual futures, so no open interest
	return nil, fmt.Errorf("open interest not supported by coinbase (spot-only exchange)")
}

// Helper methods

func (c *CoinbaseAdapter) normalizeSymbol(symbol string) string {
	// Convert BTC/USDT to BTC-USDT for Coinbase
	return strings.ReplaceAll(strings.ToUpper(symbol), "/", "-")
}

func (c *CoinbaseAdapter) convertInterval(interval string) string {
	// Convert standard intervals to Coinbase granularity format
	intervalMap := map[string]string{
		"1m":  "ONE_MINUTE",
		"5m":  "FIVE_MINUTE",
		"15m": "FIFTEEN_MINUTE",
		"1h":  "ONE_HOUR",
		"6h":  "SIX_HOUR",
		"1d":  "ONE_DAY",
	}
	
	if coinbaseInterval, exists := intervalMap[interval]; exists {
		return coinbaseInterval
	}
	return "ONE_MINUTE" // Default
}

func (c *CoinbaseAdapter) getIntervalDuration(interval string) time.Duration {
	switch interval {
	case "1m":
		return time.Minute
	case "5m":
		return 5 * time.Minute
	case "15m":
		return 15 * time.Minute
	case "1h":
		return time.Hour
	case "6h":
		return 6 * time.Hour
	case "1d":
		return 24 * time.Hour
	default:
		return time.Minute
	}
}

func (c *CoinbaseAdapter) getWSConnection(ctx context.Context, connType string) (*websocket.Conn, error) {
	c.wsLock.RLock()
	conn, exists := c.wsConnections[connType]
	c.wsLock.RUnlock()
	
	if exists && conn != nil {
		return conn, nil
	}
	
	// Create new connection
	c.wsLock.Lock()
	defer c.wsLock.Unlock()
	
	// Double-check after acquiring write lock
	if conn, exists := c.wsConnections[connType]; exists && conn != nil {
		return conn, nil
	}
	
	wsURL := c.wsURL
	
	conn, _, err := websocket.DefaultDialer.DialContext(ctx, wsURL, nil)
	if err != nil {
		return nil, fmt.Errorf("dial websocket: %w", err)
	}
	
	c.wsConnections[connType] = conn
	
	// Start reading messages
	go c.readWSMessages(ctx, conn, connType)
	
	return conn, nil
}

func (c *CoinbaseAdapter) readWSMessages(ctx context.Context, conn *websocket.Conn, connType string) {
	defer func() {
		c.wsLock.Lock()
		delete(c.wsConnections, connType)
		c.wsLock.Unlock()
		conn.Close()
	}()
	
	for {
		select {
		case <-ctx.Done():
			return
		default:
			var msg coinbaseWSMessage
			if err := conn.ReadJSON(&msg); err != nil {
				// Handle reconnection logic here
				return
			}
			
			c.routeWSMessage(msg)
		}
	}
}

func (c *CoinbaseAdapter) routeWSMessage(msg coinbaseWSMessage) {
	if len(msg.Events) == 0 {
		return
	}
	
	for _, event := range msg.Events {
		key := fmt.Sprintf("%s:%s", msg.Channel, event.ProductID)
		
		c.subLock.RLock()
		ch, exists := c.subscriptions[key]
		c.subLock.RUnlock()
		
		if exists {
			select {
			case ch <- event:
			default:
				// Channel is full, drop message
			}
		}
	}
}

func (c *CoinbaseAdapter) processTradeMessages(ctx context.Context, key string, out chan<- interfaces.TradeEvent) {
	c.subLock.RLock()
	ch := c.subscriptions[key]
	c.subLock.RUnlock()
	
	defer close(out)
	
	for {
		select {
		case <-ctx.Done():
			return
		case data := <-ch:
			if event, ok := data.(coinbaseEvent); ok {
				if tradeEvents := c.convertTradeEvent(event); len(tradeEvents) > 0 {
					for _, tradeEvent := range tradeEvents {
						select {
						case out <- tradeEvent:
						case <-ctx.Done():
							return
						}
					}
				}
			}
		}
	}
}

func (c *CoinbaseAdapter) processKlineMessages(ctx context.Context, key string, out chan<- interfaces.KlineEvent, interval string) {
	c.subLock.RLock()
	ch := c.subscriptions[key]
	c.subLock.RUnlock()
	
	defer close(out)
	
	for {
		select {
		case <-ctx.Done():
			return
		case data := <-ch:
			if event, ok := data.(coinbaseEvent); ok {
				if klineEvent := c.convertKlineEvent(event, interval); klineEvent != nil {
					select {
					case out <- *klineEvent:
					case <-ctx.Done():
						return
					}
				}
			}
		}
	}
}

func (c *CoinbaseAdapter) processOrderBookMessages(ctx context.Context, key string, out chan<- interfaces.OrderBookEvent) {
	c.subLock.RLock()
	ch := c.subscriptions[key]
	c.subLock.RUnlock()
	
	defer close(out)
	
	for {
		select {
		case <-ctx.Done():
			return
		case data := <-ch:
			if event, ok := data.(coinbaseEvent); ok {
				if obEvent := c.convertOrderBookEvent(event); obEvent != nil {
					select {
					case out <- *obEvent:
					case <-ctx.Done():
						return
					}
				}
			}
		}
	}
}

// Data conversion helpers

func (c *CoinbaseAdapter) convertTradeEvent(event coinbaseEvent) []interfaces.TradeEvent {
	var tradeEvents []interfaces.TradeEvent
	
	// Parse trades from the event updates
	if updates, ok := event.Updates.([]interface{}); ok {
		for _, update := range updates {
			if tradeMap, ok := update.(map[string]interface{}); ok {
				tradeID, _ := tradeMap["trade_id"].(string)
				price, _ := strconv.ParseFloat(tradeMap["price"].(string), 64)
				size, _ := strconv.ParseFloat(tradeMap["size"].(string), 64)
				side, _ := tradeMap["side"].(string)
				timeStr, _ := tradeMap["time"].(string)
				timestamp, _ := time.Parse(time.RFC3339, timeStr)
				
				tradeEvent := interfaces.TradeEvent{
					Trade: interfaces.Trade{
						ID:        tradeID,
						Symbol:    c.denormalizeSymbol(event.ProductID),
						Price:     price,
						Quantity:  size,
						Side:      side,
						Timestamp: timestamp,
						Venue:     "coinbase",
					},
					EventTime: time.Now(),
				}
				
				tradeEvents = append(tradeEvents, tradeEvent)
			}
		}
	}
	
	return tradeEvents
}

func (c *CoinbaseAdapter) convertKlineEvent(event coinbaseEvent, interval string) *interfaces.KlineEvent {
	// Parse candles from the event
	if candles, ok := event.Candles.([]interface{}); ok {
		for _, candle := range candles {
			if candleMap, ok := candle.(map[string]interface{}); ok {
				startStr, _ := candleMap["start"].(string)
				startTime, _ := time.Parse(time.RFC3339, startStr)
				open, _ := strconv.ParseFloat(candleMap["open"].(string), 64)
				high, _ := strconv.ParseFloat(candleMap["high"].(string), 64)
				low, _ := strconv.ParseFloat(candleMap["low"].(string), 64)
				close, _ := strconv.ParseFloat(candleMap["close"].(string), 64)
				volume, _ := strconv.ParseFloat(candleMap["volume"].(string), 64)
				
				return &interfaces.KlineEvent{
					Kline: interfaces.Kline{
						Symbol:    c.denormalizeSymbol(event.ProductID),
						Interval:  interval,
						OpenTime:  startTime,
						CloseTime: startTime.Add(c.getIntervalDuration(interval)),
						Open:      open,
						High:      high,
						Low:       low,
						Close:     close,
						Volume:    volume,
						Venue:     "coinbase",
					},
					EventTime: time.Now(),
				}
			}
		}
	}
	
	return nil
}

func (c *CoinbaseAdapter) convertOrderBookEvent(event coinbaseEvent) *interfaces.OrderBookEvent {
	// Parse order book updates from the event
	if updates, ok := event.Updates.(map[string]interface{}); ok {
		snapshot := &interfaces.OrderBookSnapshot{
			Symbol:    c.denormalizeSymbol(event.ProductID),
			Venue:     "coinbase",
			Timestamp: time.Now(),
		}
		
		// Process bids
		if bids, ok := updates["bids"].([]interface{}); ok {
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
		if asks, ok := updates["asks"].([]interface{}); ok {
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
			OrderBook: *snapshot,
			EventTime: time.Now(),
		}
	}
	
	return nil
}

func (c *CoinbaseAdapter) denormalizeSymbol(productID string) string {
	// Convert BTC-USDT back to BTC/USDT
	return strings.ReplaceAll(productID, "-", "/")
}

func (c *CoinbaseAdapter) makeRequest(ctx context.Context, method, endpoint string, body interface{}, result interface{}) error {
	// This would implement the actual HTTP request logic
	// For now, return a mock implementation
	return nil
}