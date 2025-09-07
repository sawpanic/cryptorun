package adapters

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// BinanceMicrostructureAdapter provides exchange-native L1/L2 data from Binance
type BinanceMicrostructureAdapter struct {
	baseURL    string
	httpClient *http.Client
}

// NewBinanceMicrostructureAdapter creates a new Binance microstructure adapter
func NewBinanceMicrostructureAdapter() *BinanceMicrostructureAdapter {
	return &BinanceMicrostructureAdapter{
		baseURL: "https://api.binance.com/api/v3",
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// GetL1Data fetches best bid/ask data from Binance
func (b *BinanceMicrostructureAdapter) GetL1Data(ctx context.Context, symbol string) (*L1Data, error) {
	// VALIDATE EXCHANGE-NATIVE SOURCE
	if err := ValidateL1DataSource("binance"); err != nil {
		return nil, fmt.Errorf("source validation failed: %w", err)
	}
	// Convert symbol format: BTC/USD -> BTCUSDT
	binanceSymbol := convertToBinanceSymbol(symbol)

	url := fmt.Sprintf("%s/ticker/bookTicker?symbol=%s", b.baseURL, binanceSymbol)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := b.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("binance API error: %d", resp.StatusCode)
	}

	var bookTicker BinanceBookTicker
	if err := json.NewDecoder(resp.Body).Decode(&bookTicker); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Convert to L1Data
	bidPrice, _ := strconv.ParseFloat(bookTicker.BidPrice, 64)
	bidQty, _ := strconv.ParseFloat(bookTicker.BidQty, 64)
	askPrice, _ := strconv.ParseFloat(bookTicker.AskPrice, 64)
	askQty, _ := strconv.ParseFloat(bookTicker.AskQty, 64)

	midPrice := (bidPrice + askPrice) / 2
	spreadBps := calculateSpreadBps(bidPrice, askPrice)

	return &L1Data{
		Symbol:    symbol,
		Venue:     "binance",
		Timestamp: time.Now(),
		BidPrice:  bidPrice,
		BidSize:   bidQty,
		AskPrice:  askPrice,
		AskSize:   askQty,
		SpreadBps: spreadBps,
		MidPrice:  midPrice,
		Quality:   "excellent", // Real-time data
		DataAge:   0,
	}, nil
}

// GetL2Data fetches order book depth data from Binance
func (b *BinanceMicrostructureAdapter) GetL2Data(ctx context.Context, symbol string) (*L2Data, error) {
	// VALIDATE EXCHANGE-NATIVE SOURCE
	if err := ValidateL2DataSource("binance"); err != nil {
		return nil, fmt.Errorf("source validation failed: %w", err)
	}
	binanceSymbol := convertToBinanceSymbol(symbol)

	url := fmt.Sprintf("%s/depth?symbol=%s&limit=100", b.baseURL, binanceSymbol)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := b.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("binance API error: %d", resp.StatusCode)
	}

	var orderBook BinanceOrderBook
	if err := json.NewDecoder(resp.Body).Decode(&orderBook); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Calculate mid price from best bid/ask
	if len(orderBook.Bids) == 0 || len(orderBook.Asks) == 0 {
		return nil, fmt.Errorf("empty order book")
	}

	bestBid, _ := strconv.ParseFloat(orderBook.Bids[0][0], 64)
	bestAsk, _ := strconv.ParseFloat(orderBook.Asks[0][0], 64)
	midPrice := (bestBid + bestAsk) / 2

	// Calculate depth within ±2%
	bidDepthUSD, bidLevels := calculateDepthUSD(orderBook.Bids, midPrice, -0.02)
	askDepthUSD, askLevels := calculateDepthUSD(orderBook.Asks, midPrice, 0.02)
	totalDepthUSD := bidDepthUSD + askDepthUSD

	// Calculate liquidity gradient (0.5% vs 2%)
	bidDepth05, _ := calculateDepthUSD(orderBook.Bids, midPrice, -0.005)
	askDepth05, _ := calculateDepthUSD(orderBook.Asks, midPrice, 0.005)
	totalDepth05 := bidDepth05 + askDepth05

	liquidityGradient := 0.0
	if totalDepthUSD > 0 {
		liquidityGradient = totalDepth05 / totalDepthUSD
	}

	return &L2Data{
		Symbol:            symbol,
		Venue:             "binance",
		Timestamp:         time.Now(),
		BidDepthUSD:       bidDepthUSD,
		AskDepthUSD:       askDepthUSD,
		TotalDepthUSD:     totalDepthUSD,
		BidLevels:         bidLevels,
		AskLevels:         askLevels,
		LiquidityGradient: liquidityGradient,
		VADRInputVolume:   0, // Would need additional data
		VADRInputRange:    0, // Would need additional data
		Quality:           "excellent",
		IsUSDQuote:        strings.HasSuffix(strings.ToUpper(symbol), "/USD"),
	}, nil
}

// GetOrderBookSnapshot fetches complete order book snapshot
func (b *BinanceMicrostructureAdapter) GetOrderBookSnapshot(ctx context.Context, symbol string) (*OrderBookSnapshot, error) {
	binanceSymbol := convertToBinanceSymbol(symbol)

	url := fmt.Sprintf("%s/depth?symbol=%s&limit=100", b.baseURL, binanceSymbol)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := b.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("binance API error: %d", resp.StatusCode)
	}

	var orderBook BinanceOrderBook
	if err := json.NewDecoder(resp.Body).Decode(&orderBook); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Convert to adapter format
	bids := make([]PriceLevel, len(orderBook.Bids))
	for i, bid := range orderBook.Bids {
		price, _ := strconv.ParseFloat(bid[0], 64)
		size, _ := strconv.ParseFloat(bid[1], 64)
		bids[i] = PriceLevel{Price: price, Size: size}
	}

	asks := make([]PriceLevel, len(orderBook.Asks))
	for i, ask := range orderBook.Asks {
		price, _ := strconv.ParseFloat(ask[0], 64)
		size, _ := strconv.ParseFloat(ask[1], 64)
		asks[i] = PriceLevel{Price: price, Size: size}
	}

	// Get last price from ticker
	lastPrice := 0.0
	if len(bids) > 0 && len(asks) > 0 {
		lastPrice = (bids[0].Price + asks[0].Price) / 2
	}

	return &OrderBookSnapshot{
		Symbol:    symbol,
		Venue:     "binance",
		Timestamp: time.Now(),
		Bids:      bids,
		Asks:      asks,
		LastPrice: lastPrice,
		Metadata: SnapshotMetadata{
			Source:      "binance",
			Sequence:    orderBook.LastUpdateID,
			IsStale:     false,
			UpdateAge:   0,
			BookQuality: "full",
		},
	}, nil
}

// Binance API response types
type BinanceBookTicker struct {
	Symbol   string `json:"symbol"`
	BidPrice string `json:"bidPrice"`
	BidQty   string `json:"bidQty"`
	AskPrice string `json:"askPrice"`
	AskQty   string `json:"askQty"`
}

type BinanceOrderBook struct {
	LastUpdateID int64       `json:"lastUpdateId"`
	Bids         [][2]string `json:"bids"`
	Asks         [][2]string `json:"asks"`
}

// Helper functions
func convertToBinanceSymbol(symbol string) string {
	// Convert BTC/USD to BTCUSDT, ETH/USD to ETHUSDT
	parts := strings.Split(symbol, "/")
	if len(parts) != 2 {
		return symbol // Return as-is if not in expected format
	}

	base := strings.ToUpper(parts[0])
	quote := strings.ToUpper(parts[1])

	// Binance uses USDT for USD pairs
	if quote == "USD" {
		quote = "USDT"
	}

	return base + quote
}

func calculateSpreadBps(bidPrice, askPrice float64) float64 {
	if bidPrice <= 0 || askPrice <= 0 || askPrice <= bidPrice {
		return 0
	}

	midPrice := (bidPrice + askPrice) / 2
	spread := askPrice - bidPrice
	return (spread / midPrice) * 10000 // Convert to basis points
}

func calculateDepthUSD(levels [][2]string, midPrice, pctRange float64) (float64, int) {
	targetPrice := midPrice * (1 + pctRange)
	totalDepth := 0.0
	levelCount := 0

	for _, level := range levels {
		price, _ := strconv.ParseFloat(level[0], 64)
		size, _ := strconv.ParseFloat(level[1], 64)

		var withinRange bool
		if pctRange < 0 { // Bids (price should be >= target)
			withinRange = price >= targetPrice
		} else { // Asks (price should be <= target)
			withinRange = price <= targetPrice
		}

		if withinRange {
			totalDepth += price * size // USD value
			levelCount++
		} else {
			break // Levels are sorted
		}
	}

	return totalDepth, levelCount
}
