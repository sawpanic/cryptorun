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

// OKXMicrostructureAdapter provides exchange-native L1/L2 data from OKX
type OKXMicrostructureAdapter struct {
	baseURL    string
	httpClient *http.Client
}

// NewOKXMicrostructureAdapter creates a new OKX microstructure adapter
func NewOKXMicrostructureAdapter() *OKXMicrostructureAdapter {
	return &OKXMicrostructureAdapter{
		baseURL: "https://www.okx.com/api/v5",
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// GetL1Data fetches best bid/ask data from OKX
func (o *OKXMicrostructureAdapter) GetL1Data(ctx context.Context, symbol string) (*L1Data, error) {
	// VALIDATE EXCHANGE-NATIVE SOURCE
	if err := ValidateL1DataSource("okx"); err != nil {
		return nil, fmt.Errorf("source validation failed: %w", err)
	}
	// Convert symbol format: BTC/USD -> BTC-USDT
	okxSymbol := convertToOKXSymbol(symbol)

	url := fmt.Sprintf("%s/market/ticker?instId=%s", o.baseURL, okxSymbol)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := o.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("okx API error: %d", resp.StatusCode)
	}

	var response OKXTickerResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if response.Code != "0" || len(response.Data) == 0 {
		return nil, fmt.Errorf("okx API returned error or no data")
	}

	ticker := response.Data[0]

	// Convert to L1Data
	bidPrice, _ := strconv.ParseFloat(ticker.BidPx, 64)
	bidQty, _ := strconv.ParseFloat(ticker.BidSz, 64)
	askPrice, _ := strconv.ParseFloat(ticker.AskPx, 64)
	askQty, _ := strconv.ParseFloat(ticker.AskSz, 64)
	_, _ = strconv.ParseFloat(ticker.Last, 64) // lastPrice not used in L1 data

	midPrice := (bidPrice + askPrice) / 2
	spreadBps := calculateSpreadBps(bidPrice, askPrice)

	return &L1Data{
		Symbol:    symbol,
		Venue:     "okx",
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

// GetL2Data fetches order book depth data from OKX
func (o *OKXMicrostructureAdapter) GetL2Data(ctx context.Context, symbol string) (*L2Data, error) {
	// VALIDATE EXCHANGE-NATIVE SOURCE
	if err := ValidateL2DataSource("okx"); err != nil {
		return nil, fmt.Errorf("source validation failed: %w", err)
	}
	okxSymbol := convertToOKXSymbol(symbol)

	url := fmt.Sprintf("%s/market/books?instId=%s&sz=100", o.baseURL, okxSymbol)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := o.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("okx API error: %d", resp.StatusCode)
	}

	var response OKXOrderBookResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if response.Code != "0" || len(response.Data) == 0 {
		return nil, fmt.Errorf("okx API returned error or no data")
	}

	orderBook := response.Data[0]

	// Calculate mid price from best bid/ask
	if len(orderBook.Bids) == 0 || len(orderBook.Asks) == 0 {
		return nil, fmt.Errorf("empty order book")
	}

	bestBid, _ := strconv.ParseFloat(orderBook.Bids[0][0], 64)
	bestAsk, _ := strconv.ParseFloat(orderBook.Asks[0][0], 64)
	midPrice := (bestBid + bestAsk) / 2

	// Calculate depth within ±2%
	bidDepthUSD, bidLevels := calculateDepthUSDFromOKX(orderBook.Bids, midPrice, -0.02)
	askDepthUSD, askLevels := calculateDepthUSDFromOKX(orderBook.Asks, midPrice, 0.02)
	totalDepthUSD := bidDepthUSD + askDepthUSD

	// Calculate liquidity gradient (0.5% vs 2%)
	bidDepth05, _ := calculateDepthUSDFromOKX(orderBook.Bids, midPrice, -0.005)
	askDepth05, _ := calculateDepthUSDFromOKX(orderBook.Asks, midPrice, 0.005)
	totalDepth05 := bidDepth05 + askDepth05

	liquidityGradient := 0.0
	if totalDepthUSD > 0 {
		liquidityGradient = totalDepth05 / totalDepthUSD
	}

	return &L2Data{
		Symbol:            symbol,
		Venue:             "okx",
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
func (o *OKXMicrostructureAdapter) GetOrderBookSnapshot(ctx context.Context, symbol string) (*OrderBookSnapshot, error) {
	okxSymbol := convertToOKXSymbol(symbol)

	url := fmt.Sprintf("%s/market/books?instId=%s&sz=100", o.baseURL, okxSymbol)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := o.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("okx API error: %d", resp.StatusCode)
	}

	var response OKXOrderBookResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if response.Code != "0" || len(response.Data) == 0 {
		return nil, fmt.Errorf("okx API returned error or no data")
	}

	orderBook := response.Data[0]

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

	timestamp, _ := strconv.ParseInt(orderBook.Ts, 10, 64)

	return &OrderBookSnapshot{
		Symbol:    symbol,
		Venue:     "okx",
		Timestamp: time.Unix(timestamp/1000, 0), // OKX uses milliseconds
		Bids:      bids,
		Asks:      asks,
		LastPrice: lastPrice,
		Metadata: SnapshotMetadata{
			Source:      "okx",
			Sequence:    0, // OKX doesn't provide sequence in this endpoint
			IsStale:     false,
			UpdateAge:   0,
			BookQuality: "full",
		},
	}, nil
}

// OKX API response types
type OKXTickerResponse struct {
	Code string      `json:"code"`
	Data []OKXTicker `json:"data"`
}

type OKXTicker struct {
	InstId string `json:"instId"`
	Last   string `json:"last"`
	BidPx  string `json:"bidPx"`
	BidSz  string `json:"bidSz"`
	AskPx  string `json:"askPx"`
	AskSz  string `json:"askSz"`
	Ts     string `json:"ts"`
}

type OKXOrderBookResponse struct {
	Code string         `json:"code"`
	Data []OKXOrderBook `json:"data"`
}

type OKXOrderBook struct {
	Asks [][]string `json:"asks"`
	Bids [][]string `json:"bids"`
	Ts   string     `json:"ts"`
}

// Helper functions
func convertToOKXSymbol(symbol string) string {
	// Convert BTC/USD to BTC-USDT, ETH/USD to ETH-USDT
	parts := strings.Split(symbol, "/")
	if len(parts) != 2 {
		return symbol
	}

	base := strings.ToUpper(parts[0])
	quote := strings.ToUpper(parts[1])

	// OKX uses USDT for USD pairs
	if quote == "USD" {
		quote = "USDT"
	}

	return base + "-" + quote
}

func calculateDepthUSDFromOKX(levels [][]string, midPrice, pctRange float64) (float64, int) {
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
