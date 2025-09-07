package ws

import (
	"fmt"
	"sync"
	"time"

	"github.com/sawpanic/cryptorun/internal/data"
)

// BinanceWSClient implements WebSocket client for Binance
type BinanceWSClient struct {
	connected   bool
	subscribed  map[string]bool // symbol -> subscribed
	latestTicks map[string]*data.Envelope
	mutex       sync.RWMutex
}

// BinanceBookTicker represents Binance WebSocket book ticker data
type BinanceBookTicker struct {
	Symbol     string `json:"s"`
	BidPrice   string `json:"b"`
	BidQty     string `json:"B"`
	AskPrice   string `json:"a"`
	AskQty     string `json:"A"`
	UpdateTime int64  `json:"u"`
}

// NewBinanceWSClient creates a new Binance WebSocket client
func NewBinanceWSClient() *BinanceWSClient {
	return &BinanceWSClient{
		subscribed:  make(map[string]bool),
		latestTicks: make(map[string]*data.Envelope),
	}
}

// Connect establishes WebSocket connection
func (b *BinanceWSClient) Connect() error {
	// TODO: Implement actual WebSocket connection
	// For now, simulate connection
	b.mutex.Lock()
	b.connected = true
	b.mutex.Unlock()

	// Start mock data generator for testing
	go b.generateMockTicks()

	return nil
}

// Disconnect closes WebSocket connection
func (b *BinanceWSClient) Disconnect() error {
	b.mutex.Lock()
	b.connected = false
	b.mutex.Unlock()

	return nil
}

// Subscribe to symbol updates
func (b *BinanceWSClient) Subscribe(symbol string) error {
	if !b.IsConnected() {
		return fmt.Errorf("client not connected")
	}

	b.mutex.Lock()
	b.subscribed[symbol] = true
	b.mutex.Unlock()

	// TODO: Send actual subscription message
	return nil
}

// Unsubscribe from symbol updates
func (b *BinanceWSClient) Unsubscribe(symbol string) error {
	b.mutex.Lock()
	delete(b.subscribed, symbol)
	b.mutex.Unlock()

	// TODO: Send actual unsubscription message
	return nil
}

// IsConnected returns connection status
func (b *BinanceWSClient) IsConnected() bool {
	b.mutex.RLock()
	defer b.mutex.RUnlock()
	return b.connected
}

// GetLastTick retrieves the most recent tick for symbol
func (b *BinanceWSClient) GetLastTick(symbol string) (*data.Envelope, error) {
	b.mutex.RLock()
	envelope, exists := b.latestTicks[symbol]
	b.mutex.RUnlock()

	if !exists {
		return nil, fmt.Errorf("no tick data for symbol: %s", symbol)
	}

	// Update freshness
	envelope.CalculateFreshness()

	return envelope, nil
}

// generateMockTicks generates fake tick data for testing
func (b *BinanceWSClient) generateMockTicks() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		if !b.IsConnected() {
			return
		}

		select {
		case <-ticker.C:
			b.mutex.RLock()
			symbols := make([]string, 0, len(b.subscribed))
			for symbol := range b.subscribed {
				symbols = append(symbols, symbol)
			}
			b.mutex.RUnlock()

			// Generate mock data for subscribed symbols
			for _, symbol := range symbols {
				mockTick := b.generateMockBookTicker(symbol)
				envelope := b.convertToEnvelope(mockTick)

				b.mutex.Lock()
				b.latestTicks[symbol] = envelope
				b.mutex.Unlock()
			}
		}
	}
}

// generateMockBookTicker creates fake book ticker data
func (b *BinanceWSClient) generateMockBookTicker(symbol string) *BinanceBookTicker {
	// Generate realistic-looking mock data
	basePrice := 50000.0 + float64(time.Now().Unix()%1000) // Simulate price movement
	spread := 0.01 * basePrice                             // 1% spread

	return &BinanceBookTicker{
		Symbol:     symbol,
		BidPrice:   fmt.Sprintf("%.2f", basePrice),
		BidQty:     "1.5",
		AskPrice:   fmt.Sprintf("%.2f", basePrice+spread),
		AskQty:     "2.3",
		UpdateTime: time.Now().UnixMilli(),
	}
}

// convertToEnvelope converts Binance data to standard envelope
func (b *BinanceWSClient) convertToEnvelope(tick *BinanceBookTicker) *data.Envelope {
	now := time.Now()

	envelope := data.NewEnvelope("binance", tick.Symbol, data.TierHot,
		data.WithConfidenceScore(0.95),
	)

	// Set provenance info
	envelope.Provenance.OriginalSource = "binance_ws"
	envelope.Provenance.LatencyMS = 50 // Mock latency

	// Mock order book data
	orderBookData := map[string]interface{}{
		"symbol":         tick.Symbol,
		"venue":          "binance",
		"timestamp":      now,
		"best_bid_price": tick.BidPrice,
		"best_bid_qty":   tick.BidQty,
		"best_ask_price": tick.AskPrice,
		"best_ask_qty":   tick.AskQty,
		"sequence_num":   tick.UpdateTime,
	}

	envelope.OrderBook = orderBookData
	envelope.Checksum = envelope.GenerateChecksum(orderBookData, "book_ticker")

	return envelope
}
