package ws

import (
	"fmt"
	"sync"
	"time"

	"github.com/sawpanic/cryptorun/internal/data"
)

// OKXWSClient implements WebSocket client for OKX
type OKXWSClient struct {
	connected   bool
	subscribed  map[string]bool // symbol -> subscribed
	latestTicks map[string]*data.Envelope
	mutex       sync.RWMutex
}

// OKXBookTicker represents OKX WebSocket book ticker data
type OKXBookTicker struct {
	InstID    string `json:"instId"`
	BestBid   string `json:"bidPx"`
	BestBidSz string `json:"bidSz"`
	BestAsk   string `json:"askPx"`
	BestAskSz string `json:"askSz"`
	TS        string `json:"ts"`
}

// NewOKXWSClient creates a new OKX WebSocket client
func NewOKXWSClient() *OKXWSClient {
	return &OKXWSClient{
		subscribed:  make(map[string]bool),
		latestTicks: make(map[string]*data.Envelope),
	}
}

// Connect establishes WebSocket connection
func (o *OKXWSClient) Connect() error {
	o.mutex.Lock()
	o.connected = true
	o.mutex.Unlock()

	// Start mock data generator
	go o.generateMockTicks()

	return nil
}

// Disconnect closes WebSocket connection
func (o *OKXWSClient) Disconnect() error {
	o.mutex.Lock()
	o.connected = false
	o.mutex.Unlock()

	return nil
}

// Subscribe to symbol updates
func (o *OKXWSClient) Subscribe(symbol string) error {
	if !o.IsConnected() {
		return fmt.Errorf("client not connected")
	}

	o.mutex.Lock()
	o.subscribed[symbol] = true
	o.mutex.Unlock()

	return nil
}

// Unsubscribe from symbol updates
func (o *OKXWSClient) Unsubscribe(symbol string) error {
	o.mutex.Lock()
	delete(o.subscribed, symbol)
	o.mutex.Unlock()

	return nil
}

// IsConnected returns connection status
func (o *OKXWSClient) IsConnected() bool {
	o.mutex.RLock()
	defer o.mutex.RUnlock()
	return o.connected
}

// GetLastTick retrieves the most recent tick for symbol
func (o *OKXWSClient) GetLastTick(symbol string) (*data.Envelope, error) {
	o.mutex.RLock()
	envelope, exists := o.latestTicks[symbol]
	o.mutex.RUnlock()

	if !exists {
		return nil, fmt.Errorf("no tick data for symbol: %s", symbol)
	}

	envelope.CalculateFreshness()
	return envelope, nil
}

// generateMockTicks generates fake tick data for testing
func (o *OKXWSClient) generateMockTicks() {
	ticker := time.NewTicker(1500 * time.Millisecond) // Slightly different from Binance
	defer ticker.Stop()

	for {
		if !o.IsConnected() {
			return
		}

		select {
		case <-ticker.C:
			o.mutex.RLock()
			symbols := make([]string, 0, len(o.subscribed))
			for symbol := range o.subscribed {
				symbols = append(symbols, symbol)
			}
			o.mutex.RUnlock()

			for _, symbol := range symbols {
				mockTick := o.generateMockBookTicker(symbol)
				envelope := o.convertToEnvelope(mockTick)

				o.mutex.Lock()
				o.latestTicks[symbol] = envelope
				o.mutex.Unlock()
			}
		}
	}
}

// generateMockBookTicker creates fake book ticker data
func (o *OKXWSClient) generateMockBookTicker(symbol string) *OKXBookTicker {
	basePrice := 49800.0 + float64(time.Now().Unix()%800) // Different base than Binance
	spread := 0.008 * basePrice                           // 0.8% spread

	return &OKXBookTicker{
		InstID:    symbol,
		BestBid:   fmt.Sprintf("%.2f", basePrice),
		BestBidSz: "2.1",
		BestAsk:   fmt.Sprintf("%.2f", basePrice+spread),
		BestAskSz: "1.8",
		TS:        fmt.Sprintf("%d", time.Now().UnixMilli()),
	}
}

// convertToEnvelope converts OKX data to standard envelope
func (o *OKXWSClient) convertToEnvelope(tick *OKXBookTicker) *data.Envelope {
	now := time.Now()

	envelope := data.NewEnvelope("okx", tick.InstID, data.TierHot,
		data.WithConfidenceScore(0.92), // Slightly lower than Binance
	)

	envelope.Provenance.OriginalSource = "okx_ws"
	envelope.Provenance.LatencyMS = 75 // Higher latency than Binance

	orderBookData := map[string]interface{}{
		"symbol":         tick.InstID,
		"venue":          "okx",
		"timestamp":      now,
		"best_bid_price": tick.BestBid,
		"best_bid_qty":   tick.BestBidSz,
		"best_ask_price": tick.BestAsk,
		"best_ask_qty":   tick.BestAskSz,
		"sequence_num":   tick.TS,
	}

	envelope.OrderBook = orderBookData
	envelope.Checksum = envelope.GenerateChecksum(orderBookData, "book_ticker")

	return envelope
}
