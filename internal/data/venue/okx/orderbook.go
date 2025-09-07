package okx

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/sawpanic/cryptorun/internal/data/venue/types"
)

// OrderBookClient fetches L1/L2 data from OKX exchange-native API
type OrderBookClient struct {
	baseURL    string
	httpClient *http.Client
	cache      *OrderBookCache
}

// OrderBookCache provides TTL caching for orderbook data
type OrderBookCache struct {
	data map[string]*types.CachedOrderBook
	ttl  time.Duration
}

// NewOrderBookClient creates an OKX orderbook client
func NewOrderBookClient() *OrderBookClient {
	return &OrderBookClient{
		baseURL:    "https://www.okx.com",
		httpClient: &http.Client{Timeout: 10 * time.Second},
		cache: &OrderBookCache{
			data: make(map[string]*types.CachedOrderBook),
			ttl:  5 * time.Minute, // 300s TTL as required
		},
	}
}

// FetchOrderBook retrieves L1/L2 data for a symbol from OKX
func (c *OrderBookClient) FetchOrderBook(ctx context.Context, symbol string) (*types.OrderBook, error) {
	// Check cache first
	if cached := c.cache.Get(symbol); cached != nil {
		log.Debug().
			Str("symbol", symbol).
			Str("venue", "okx").
			Time("cached_at", cached.Timestamp).
			Msg("Using cached orderbook data")
		return cached.OrderBook, nil
	}

	// Fetch from OKX API
	orderBook, err := c.fetchFromAPI(ctx, symbol)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch from OKX API: %w", err)
	}

	// Cache the result
	c.cache.Set(symbol, orderBook)

	log.Info().
		Str("symbol", symbol).
		Str("venue", "okx").
		Float64("spread_bps", orderBook.SpreadBPS).
		Float64("depth_usd", orderBook.DepthUSDPlusMinus2Pct).
		Msg("Fetched fresh orderbook from OKX")

	return orderBook, nil
}

// fetchFromAPI makes the actual HTTP request to OKX
func (c *OrderBookClient) fetchFromAPI(ctx context.Context, symbol string) (*types.OrderBook, error) {
	// Get current time for monotonic timestamp
	fetchTime := time.Now()

	// OKX books endpoint - fetch full depth for ±2% calculation
	url := fmt.Sprintf("%s/api/v5/market/books?instId=%s&sz=400", c.baseURL, symbol)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("OKX API error: %d %s", resp.StatusCode, string(body))
	}

	var okxBooks OKXBooksResponse
	if err := json.NewDecoder(resp.Body).Decode(&okxBooks); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if len(okxBooks.Data) == 0 {
		return nil, fmt.Errorf("empty response from OKX for %s", symbol)
	}

	// Convert to normalized OrderBook format
	orderBook, err := c.convertToOrderBook(symbol, &okxBooks.Data[0], fetchTime)
	if err != nil {
		return nil, fmt.Errorf("failed to convert orderbook: %w", err)
	}

	return orderBook, nil
}

// convertToOrderBook converts OKX format to normalized format
func (c *OrderBookClient) convertToOrderBook(symbol string, book *OKXBookData, fetchTime time.Time) (*types.OrderBook, error) {
	if len(book.Bids) == 0 || len(book.Asks) == 0 {
		return nil, fmt.Errorf("empty orderbook for %s", symbol)
	}

	// Parse best bid/ask (L1)
	bestBidPrice, err := strconv.ParseFloat(book.Bids[0][0], 64)
	if err != nil {
		return nil, fmt.Errorf("invalid best bid price: %w", err)
	}

	bestBidQty, err := strconv.ParseFloat(book.Bids[0][1], 64)
	if err != nil {
		return nil, fmt.Errorf("invalid best bid quantity: %w", err)
	}

	bestAskPrice, err := strconv.ParseFloat(book.Asks[0][0], 64)
	if err != nil {
		return nil, fmt.Errorf("invalid best ask price: %w", err)
	}

	bestAskQty, err := strconv.ParseFloat(book.Asks[0][1], 64)
	if err != nil {
		return nil, fmt.Errorf("invalid best ask quantity: %w", err)
	}

	// Calculate mid price and spread
	midPrice := (bestBidPrice + bestAskPrice) / 2.0
	spread := bestAskPrice - bestBidPrice
	spreadBPS := (spread / midPrice) * 10000 // Convert to basis points

	// Calculate depth within ±2% of mid
	depthUSD := c.calculateDepthPlusMinus2Pct(book, midPrice)

	// Convert bids/asks to normalized format
	bids := make([]types.Level, 0, len(book.Bids))
	for _, bid := range book.Bids {
		price, _ := strconv.ParseFloat(bid[0], 64)
		qty, _ := strconv.ParseFloat(bid[1], 64)
		bids = append(bids, types.Level{
			Price:    price,
			Quantity: qty,
			ValueUSD: price * qty,
		})
	}

	asks := make([]types.Level, 0, len(book.Asks))
	for _, ask := range book.Asks {
		price, _ := strconv.ParseFloat(ask[0], 64)
		qty, _ := strconv.ParseFloat(ask[1], 64)
		asks = append(asks, types.Level{
			Price:    price,
			Quantity: qty,
			ValueUSD: price * qty,
		})
	}

	// Parse timestamp from OKX (milliseconds)
	timestampMs, _ := strconv.ParseInt(book.Ts, 10, 64)

	return &types.OrderBook{
		Symbol:                symbol,
		Venue:                 "okx",
		TimestampMono:         fetchTime,
		SequenceNum:           timestampMs, // Use timestamp as sequence for OKX
		BestBidPrice:          bestBidPrice,
		BestBidQty:            bestBidQty,
		BestAskPrice:          bestAskPrice,
		BestAskQty:            bestAskQty,
		MidPrice:              midPrice,
		SpreadBPS:             spreadBPS,
		DepthUSDPlusMinus2Pct: depthUSD,
		Bids:                  bids,
		Asks:                  asks,
	}, nil
}

// calculateDepthPlusMinus2Pct sums USD value within ±2% of mid price
func (c *OrderBookClient) calculateDepthPlusMinus2Pct(book *OKXBookData, midPrice float64) float64 {
	lowerBound := midPrice * 0.98 // -2%
	upperBound := midPrice * 1.02 // +2%

	totalDepthUSD := 0.0

	// Sum bid depth within range
	for _, bid := range book.Bids {
		price, _ := strconv.ParseFloat(bid[0], 64)
		if price < lowerBound {
			break // Bids are sorted descending
		}
		qty, _ := strconv.ParseFloat(bid[1], 64)
		totalDepthUSD += price * qty
	}

	// Sum ask depth within range
	for _, ask := range book.Asks {
		price, _ := strconv.ParseFloat(ask[0], 64)
		if price > upperBound {
			break // Asks are sorted ascending
		}
		qty, _ := strconv.ParseFloat(ask[1], 64)
		totalDepthUSD += price * qty
	}

	return totalDepthUSD
}

// Cache implementation
func (c *OrderBookCache) Get(symbol string) *types.CachedOrderBook {
	cached, exists := c.data[symbol]
	if !exists {
		return nil
	}

	if time.Since(cached.Timestamp) > c.ttl {
		delete(c.data, symbol)
		return nil
	}

	return cached
}

func (c *OrderBookCache) Set(symbol string, orderBook *types.OrderBook) {
	c.data[symbol] = &types.CachedOrderBook{
		OrderBook: orderBook,
		Timestamp: time.Now(),
	}
}

// OKXBooksResponse represents OKX API response format
type OKXBooksResponse struct {
	Code string        `json:"code"`
	Msg  string        `json:"msg"`
	Data []OKXBookData `json:"data"`
}

// OKXBookData represents individual book data from OKX
type OKXBookData struct {
	Asks [][]string `json:"asks"` // [price, quantity, liquidated_orders, num_orders]
	Bids [][]string `json:"bids"` // [price, quantity, liquidated_orders, num_orders]
	Ts   string     `json:"ts"`   // Timestamp in milliseconds
}
