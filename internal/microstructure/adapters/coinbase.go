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

// CoinbaseMicrostructureAdapter provides exchange-native L1/L2 data from Coinbase
type CoinbaseMicrostructureAdapter struct {
	baseURL    string
	httpClient *http.Client
}

// NewCoinbaseMicrostructureAdapter creates a new Coinbase microstructure adapter
func NewCoinbaseMicrostructureAdapter() *CoinbaseMicrostructureAdapter {
	return &CoinbaseMicrostructureAdapter{
		baseURL: "https://api.pro.coinbase.com",
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// GetL1Data fetches best bid/ask data from Coinbase
func (c *CoinbaseMicrostructureAdapter) GetL1Data(ctx context.Context, symbol string) (*L1Data, error) {
	// VALIDATE EXCHANGE-NATIVE SOURCE
	if err := ValidateL1DataSource("coinbase"); err != nil {
		return nil, fmt.Errorf("source validation failed: %w", err)
	}
	// Convert symbol format: BTC/USD -> BTC-USD
	coinbaseSymbol := convertToCoinbaseSymbol(symbol)

	url := fmt.Sprintf("%s/products/%s/ticker", c.baseURL, coinbaseSymbol)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("coinbase API error: %d", resp.StatusCode)
	}

	var ticker CoinbaseTicker
	if err := json.NewDecoder(resp.Body).Decode(&ticker); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Convert to L1Data
	bidPrice, _ := strconv.ParseFloat(ticker.Bid, 64)
	bidQty, _ := strconv.ParseFloat(ticker.BidSize, 64)
	askPrice, _ := strconv.ParseFloat(ticker.Ask, 64)
	askQty, _ := strconv.ParseFloat(ticker.AskSize, 64)
	_, _ = strconv.ParseFloat(ticker.Price, 64) // lastPrice not used in L1 data

	midPrice := (bidPrice + askPrice) / 2
	spreadBps := calculateSpreadBps(bidPrice, askPrice)

	return &L1Data{
		Symbol:    symbol,
		Venue:     "coinbase",
		Timestamp: time.Now(),
		BidPrice:  bidPrice,
		BidSize:   bidQty,
		AskPrice:  askPrice,
		AskSize:   askQty,
		SpreadBps: spreadBps,
		MidPrice:  midPrice,
		Quality:   "excellent",
		DataAge:   0,
	}, nil
}

// GetL2Data fetches order book depth data from Coinbase
func (c *CoinbaseMicrostructureAdapter) GetL2Data(ctx context.Context, symbol string) (*L2Data, error) {
	// VALIDATE EXCHANGE-NATIVE SOURCE
	if err := ValidateL2DataSource("coinbase"); err != nil {
		return nil, fmt.Errorf("source validation failed: %w", err)
	}
	coinbaseSymbol := convertToCoinbaseSymbol(symbol)

	url := fmt.Sprintf("%s/products/%s/book?level=2", c.baseURL, coinbaseSymbol)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("coinbase API error: %d", resp.StatusCode)
	}

	var orderBook CoinbaseOrderBook
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
	bidDepthUSD, bidLevels := calculateDepthUSDFromCoinbase(orderBook.Bids, midPrice, -0.02)
	askDepthUSD, askLevels := calculateDepthUSDFromCoinbase(orderBook.Asks, midPrice, 0.02)
	totalDepthUSD := bidDepthUSD + askDepthUSD

	// Calculate liquidity gradient (0.5% vs 2%)
	bidDepth05, _ := calculateDepthUSDFromCoinbase(orderBook.Bids, midPrice, -0.005)
	askDepth05, _ := calculateDepthUSDFromCoinbase(orderBook.Asks, midPrice, 0.005)
	totalDepth05 := bidDepth05 + askDepth05

	liquidityGradient := 0.0
	if totalDepthUSD > 0 {
		liquidityGradient = totalDepth05 / totalDepthUSD
	}

	return &L2Data{
		Symbol:            symbol,
		Venue:             "coinbase",
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
func (c *CoinbaseMicrostructureAdapter) GetOrderBookSnapshot(ctx context.Context, symbol string) (*OrderBookSnapshot, error) {
	coinbaseSymbol := convertToCoinbaseSymbol(symbol)

	url := fmt.Sprintf("%s/products/%s/book?level=2", c.baseURL, coinbaseSymbol)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("coinbase API error: %d", resp.StatusCode)
	}

	var orderBook CoinbaseOrderBook
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

	// Get last price
	lastPrice := 0.0
	if len(bids) > 0 && len(asks) > 0 {
		lastPrice = (bids[0].Price + asks[0].Price) / 2
	}

	sequence, _ := strconv.ParseInt(fmt.Sprintf("%d", orderBook.Sequence), 10, 64)

	return &OrderBookSnapshot{
		Symbol:    symbol,
		Venue:     "coinbase",
		Timestamp: time.Now(),
		Bids:      bids,
		Asks:      asks,
		LastPrice: lastPrice,
		Metadata: SnapshotMetadata{
			Source:      "coinbase",
			Sequence:    sequence,
			IsStale:     false,
			UpdateAge:   0,
			BookQuality: "full",
		},
	}, nil
}

// Coinbase API response types
type CoinbaseTicker struct {
	TradeID int64  `json:"trade_id"`
	Price   string `json:"price"`
	Size    string `json:"size"`
	Time    string `json:"time"`
	Bid     string `json:"bid"`
	Ask     string `json:"ask"`
	BidSize string `json:"bid_size"`
	AskSize string `json:"ask_size"`
	Volume  string `json:"volume"`
}

type CoinbaseOrderBook struct {
	Sequence int64      `json:"sequence"`
	Bids     [][]string `json:"bids"`
	Asks     [][]string `json:"asks"`
}

// Helper functions
func convertToCoinbaseSymbol(symbol string) string {
	// Convert BTC/USD to BTC-USD, ETH/USD to ETH-USD
	return strings.ReplaceAll(strings.ToUpper(symbol), "/", "-")
}

func calculateDepthUSDFromCoinbase(levels [][]string, midPrice, pctRange float64) (float64, int) {
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
