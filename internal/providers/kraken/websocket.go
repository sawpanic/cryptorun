package kraken

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/rs/zerolog/log"
	"github.com/sawpanic/cryptorun/internal/providers"
)

// WebSocketClient handles Kraken WebSocket connections for L1/L2 streaming
type WebSocketClient struct {
	baseURL      string
	conn         *websocket.Conn
	subscriptions map[string]*Subscription
	mu           sync.RWMutex
	handlers     map[string]MessageHandler
	reconnectCh  chan struct{}
	closeCh      chan struct{}
	isConnected  bool
	metrics      MetricsCallback
}

// Subscription represents an active WebSocket subscription
type Subscription struct {
	ChannelID   int                    `json:"channelID"`
	ChannelName string                 `json:"channelName"`
	Pair        string                 `json:"pair"`
	SubType     string                 `json:"subscription_type"`
	Config      map[string]interface{} `json:"config"`
	LastUpdate  time.Time              `json:"last_update"`
}

// MessageHandler processes incoming WebSocket messages
type MessageHandler func(data []byte, sub *Subscription) error

// OrderBookUpdate represents L2 order book updates
type OrderBookUpdate struct {
	ChannelID   int                    `json:"channelID"`
	ChannelName string                 `json:"channelName"`
	Data        map[string]interface{} `json:"data"`
	Timestamp   time.Time              `json:"timestamp"`
}

// TradeUpdate represents trade stream updates
type TradeUpdate struct {
	ChannelID   int     `json:"channelID"`
	ChannelName string  `json:"channelName"`
	Trades      []Trade `json:"trades"`
	Timestamp   time.Time `json:"timestamp"`
}

// Trade represents a single trade from WebSocket
type Trade struct {
	Price     string `json:"price"`
	Volume    string `json:"volume"`
	Time      string `json:"time"`
	Side      string `json:"side"`  // "b" for buy, "s" for sell
	OrderType string `json:"order_type"` // "m" for market, "l" for limit
}

// NewWebSocketClient creates a new Kraken WebSocket client
func NewWebSocketClient(baseURL string) *WebSocketClient {
	if baseURL == "" {
		baseURL = "wss://ws.kraken.com"
	}
	
	return &WebSocketClient{
		baseURL:       baseURL,
		subscriptions: make(map[string]*Subscription),
		handlers:      make(map[string]MessageHandler),
		reconnectCh:   make(chan struct{}, 1),
		closeCh:       make(chan struct{}),
	}
}

// SetMetricsCallback sets the metrics collection callback
func (ws *WebSocketClient) SetMetricsCallback(callback MetricsCallback) {
	ws.metrics = callback
}

// Connect establishes WebSocket connection with authentication handling
func (ws *WebSocketClient) Connect(ctx context.Context) error {
	ws.mu.Lock()
	defer ws.mu.Unlock()
	
	if ws.isConnected {
		return fmt.Errorf("already connected")
	}
	
	u, err := url.Parse(ws.baseURL)
	if err != nil {
		return fmt.Errorf("invalid WebSocket URL: %w", err)
	}
	
	log.Info().Str("url", ws.baseURL).Msg("Connecting to Kraken WebSocket")
	
	// Set up dialer with proper headers
	dialer := websocket.DefaultDialer
	dialer.HandshakeTimeout = 30 * time.Second
	
	headers := make(map[string][]string)
	headers["User-Agent"] = []string{"CryptoRun/3.2.1 (Exchange-Native WebSocket)"}
	
	conn, _, err := dialer.DialContext(ctx, u.String(), headers)
	if err != nil {
		if ws.metrics != nil {
			ws.metrics("kraken_ws_connect_failures_total", 1, 
				map[string]string{"provider": "kraken"})
		}
		return fmt.Errorf("WebSocket connection failed: %w", err)
	}
	
	ws.conn = conn
	ws.isConnected = true
	
	// Start message processing goroutine
	go ws.messageLoop(ctx)
	
	// Start ping handler for connection health
	go ws.pingLoop(ctx)
	
	if ws.metrics != nil {
		ws.metrics("kraken_ws_connections_total", 1, 
			map[string]string{"provider": "kraken"})
	}
	
	log.Info().Msg("Kraken WebSocket connected successfully")
	return nil
}

// SubscribeOrderBook subscribes to L2 order book updates for USD pairs
func (ws *WebSocketClient) SubscribeOrderBook(ctx context.Context, pairs []string, depth int) error {
	// Validate USD pairs only
	var validPairs []string
	for _, pair := range pairs {
		if !providers.IsUSDPair(pair) {
			log.Warn().Str("pair", pair).Msg("Skipping non-USD pair")
			continue
		}
		validPairs = append(validPairs, pair)
	}
	
	if len(validPairs) == 0 {
		return fmt.Errorf("no valid USD pairs to subscribe")
	}
	
	subscription := SubscriptionRequest{
		Event: "subscribe",
		Pair:  validPairs,
		Subscription: map[string]interface{}{
			"name":  "book",
			"depth": depth,
		},
	}
	
	// Register handler for order book updates
	ws.RegisterHandler("book", ws.handleOrderBookUpdate)
	
	return ws.sendSubscription(subscription)
}

// SubscribeTrades subscribes to trade stream for USD pairs
func (ws *WebSocketClient) SubscribeTrades(ctx context.Context, pairs []string) error {
	// Validate USD pairs only
	var validPairs []string
	for _, pair := range pairs {
		if !providers.IsUSDPair(pair) {
			log.Warn().Str("pair", pair).Msg("Skipping non-USD pair")
			continue
		}
		validPairs = append(validPairs, pair)
	}
	
	if len(validPairs) == 0 {
		return fmt.Errorf("no valid USD pairs to subscribe")
	}
	
	subscription := SubscriptionRequest{
		Event: "subscribe",
		Pair:  validPairs,
		Subscription: map[string]interface{}{
			"name": "trade",
		},
	}
	
	// Register handler for trade updates
	ws.RegisterHandler("trade", ws.handleTradeUpdate)
	
	return ws.sendSubscription(subscription)
}

// RegisterHandler registers a message handler for a specific channel type
func (ws *WebSocketClient) RegisterHandler(channelType string, handler MessageHandler) {
	ws.mu.Lock()
	defer ws.mu.Unlock()
	ws.handlers[channelType] = handler
}

// GetSubscriptions returns current active subscriptions
func (ws *WebSocketClient) GetSubscriptions() map[string]*Subscription {
	ws.mu.RLock()
	defer ws.mu.RUnlock()
	
	subs := make(map[string]*Subscription)
	for k, v := range ws.subscriptions {
		subs[k] = v
	}
	return subs
}

// IsConnected returns true if WebSocket is connected
func (ws *WebSocketClient) IsConnected() bool {
	ws.mu.RLock()
	defer ws.mu.RUnlock()
	return ws.isConnected
}

// Close closes the WebSocket connection and stops all goroutines
func (ws *WebSocketClient) Close() error {
	ws.mu.Lock()
	defer ws.mu.Unlock()
	
	if !ws.isConnected {
		return nil
	}
	
	// Signal close to all goroutines
	close(ws.closeCh)
	
	// Close connection
	err := ws.conn.Close()
	ws.conn = nil
	ws.isConnected = false
	
	log.Info().Msg("Kraken WebSocket connection closed")
	return err
}

// Private methods

func (ws *WebSocketClient) sendSubscription(sub SubscriptionRequest) error {
	ws.mu.Lock()
	defer ws.mu.Unlock()
	
	if !ws.isConnected {
		return fmt.Errorf("not connected")
	}
	
	data, err := json.Marshal(sub)
	if err != nil {
		return fmt.Errorf("failed to marshal subscription: %w", err)
	}
	
	log.Debug().RawJSON("subscription", data).Msg("Sending WebSocket subscription")
	
	if err := ws.conn.WriteMessage(websocket.TextMessage, data); err != nil {
		if ws.metrics != nil {
			ws.metrics("kraken_ws_send_errors_total", 1, 
				map[string]string{"provider": "kraken", "type": "subscription"})
		}
		return fmt.Errorf("failed to send subscription: %w", err)
	}
	
	return nil
}

func (ws *WebSocketClient) messageLoop(ctx context.Context) {
	defer func() {
		if r := recover(); r != nil {
			log.Error().Interface("panic", r).Msg("WebSocket message loop panic")
		}
	}()
	
	for {
		select {
		case <-ctx.Done():
			return
		case <-ws.closeCh:
			return
		default:
			// Read message with timeout
			ws.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
			
			messageType, data, err := ws.conn.ReadMessage()
			if err != nil {
				if websocket.IsCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
					log.Warn().Err(err).Msg("WebSocket closed unexpectedly")
					ws.triggerReconnect()
					return
				}
				
				if ws.metrics != nil {
					ws.metrics("kraken_ws_read_errors_total", 1, 
						map[string]string{"provider": "kraken"})
				}
				
				log.Error().Err(err).Msg("WebSocket read error")
				continue
			}
			
			if messageType != websocket.TextMessage {
				continue
			}
			
			// Process message
			if err := ws.processMessage(data); err != nil {
				log.Error().Err(err).Msg("Failed to process WebSocket message")
			}
		}
	}
}

func (ws *WebSocketClient) processMessage(data []byte) error {
	// Parse basic message structure
	var msg WebSocketMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		return fmt.Errorf("failed to parse message: %w", err)
	}
	
	// Handle subscription confirmations
	if msg.Event == "subscriptionStatus" {
		return ws.handleSubscriptionStatus(data)
	}
	
	// Handle channel messages (array format)
	var arrayMsg []interface{}
	if err := json.Unmarshal(data, &arrayMsg); err == nil && len(arrayMsg) > 1 {
		return ws.handleChannelMessage(arrayMsg)
	}
	
	log.Debug().RawJSON("message", data).Msg("Received WebSocket message")
	return nil
}

func (ws *WebSocketClient) handleSubscriptionStatus(data []byte) error {
	var status struct {
		ChannelID   int    `json:"channelID"`
		ChannelName string `json:"channelName"`
		Event       string `json:"event"`
		Status      string `json:"status"`
		Pair        string `json:"pair"`
		Subscription map[string]interface{} `json:"subscription"`
	}
	
	if err := json.Unmarshal(data, &status); err != nil {
		return fmt.Errorf("failed to parse subscription status: %w", err)
	}
	
	if status.Status == "subscribed" {
		ws.mu.Lock()
		ws.subscriptions[status.ChannelName] = &Subscription{
			ChannelID:   status.ChannelID,
			ChannelName: status.ChannelName,
			Pair:        status.Pair,
			SubType:     status.ChannelName,
			Config:      status.Subscription,
			LastUpdate:  time.Now(),
		}
		ws.mu.Unlock()
		
		log.Info().
			Int("channel_id", status.ChannelID).
			Str("channel", status.ChannelName).
			Str("pair", status.Pair).
			Msg("WebSocket subscription confirmed")
		
		if ws.metrics != nil {
			ws.metrics("kraken_ws_subscriptions_total", 1, 
				map[string]string{"provider": "kraken", "channel": status.ChannelName})
		}
	}
	
	return nil
}

func (ws *WebSocketClient) handleChannelMessage(arrayMsg []interface{}) error {
	if len(arrayMsg) < 3 {
		return fmt.Errorf("invalid channel message format")
	}
	
	// Channel ID is first element
	channelIDFloat, ok := arrayMsg[0].(float64)
	if !ok {
		return fmt.Errorf("invalid channel ID format")
	}
	channelID := int(channelIDFloat)
	
	// Find subscription by channel ID
	ws.mu.RLock()
	var sub *Subscription
	for _, s := range ws.subscriptions {
		if s.ChannelID == channelID {
			sub = s
			break
		}
	}
	ws.mu.RUnlock()
	
	if sub == nil {
		return fmt.Errorf("no subscription found for channel ID: %d", channelID)
	}
	
	// Route to appropriate handler
	ws.mu.RLock()
	handler, exists := ws.handlers[sub.ChannelName]
	ws.mu.RUnlock()
	
	if !exists {
		return fmt.Errorf("no handler for channel: %s", sub.ChannelName)
	}
	
	// Convert back to JSON for handler
	data, err := json.Marshal(arrayMsg)
	if err != nil {
		return fmt.Errorf("failed to marshal channel message: %w", err)
	}
	
	// Update subscription timestamp
	ws.mu.Lock()
	sub.LastUpdate = time.Now()
	ws.mu.Unlock()
	
	return handler(data, sub)
}

func (ws *WebSocketClient) handleOrderBookUpdate(data []byte, sub *Subscription) error {
	log.Debug().RawJSON("orderbook", data).Str("pair", sub.Pair).Msg("Order book update")
	
	if ws.metrics != nil {
		ws.metrics("kraken_ws_orderbook_updates_total", 1, 
			map[string]string{"provider": "kraken", "pair": sub.Pair})
	}
	
	// Here you would parse the order book data and update internal state
	// For now, just log the receipt
	return nil
}

func (ws *WebSocketClient) handleTradeUpdate(data []byte, sub *Subscription) error {
	log.Debug().RawJSON("trades", data).Str("pair", sub.Pair).Msg("Trade update")
	
	if ws.metrics != nil {
		ws.metrics("kraken_ws_trade_updates_total", 1, 
			map[string]string{"provider": "kraken", "pair": sub.Pair})
	}
	
	// Here you would parse the trade data and update internal state
	// For now, just log the receipt
	return nil
}

func (ws *WebSocketClient) pingLoop(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-ctx.Done():
			return
		case <-ws.closeCh:
			return
		case <-ticker.C:
			if err := ws.ping(); err != nil {
				log.Error().Err(err).Msg("WebSocket ping failed")
				ws.triggerReconnect()
				return
			}
		}
	}
}

func (ws *WebSocketClient) ping() error {
	ws.mu.Lock()
	defer ws.mu.Unlock()
	
	if !ws.isConnected {
		return fmt.Errorf("not connected")
	}
	
	ws.conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	return ws.conn.WriteMessage(websocket.PingMessage, nil)
}

func (ws *WebSocketClient) triggerReconnect() {
	select {
	case ws.reconnectCh <- struct{}{}:
	default:
		// Channel full, reconnect already triggered
	}
}

// GetReconnectChannel returns channel that signals when reconnection is needed
func (ws *WebSocketClient) GetReconnectChannel() <-chan struct{} {
	return ws.reconnectCh
}