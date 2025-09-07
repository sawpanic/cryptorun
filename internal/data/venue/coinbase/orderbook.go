package coinbase

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/rs/zerolog/log"

	"cryptorun/internal/data/venue/types"
)

// OrderBookClient fetches L1/L2 data from Coinbase exchange-native API
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

// NewOrderBookClient creates a Coinbase orderbook client
func NewOrderBookClient() *OrderBookClient {
	return &OrderBookClient{
		baseURL:    "https://api.exchange.coinbase.com",
		httpClient: &http.Client{Timeout: 10 * time.Second},
		cache: &OrderBookCache{
			data: make(map[string]*types.CachedOrderBook),
			ttl:  5 * time.Minute, // 300s TTL as required
		},
	}
}

// FetchOrderBook retrieves L1/L2 data for a symbol from Coinbase
func (c *OrderBookClient) FetchOrderBook(ctx context.Context, symbol string) (*types.OrderBook, error) {
	// Check cache first
	if cached := c.cache.Get(symbol); cached != nil {
		log.Debug().
			Str("symbol", symbol).
			Str("venue", "coinbase").
			Time("cached_at", cached.Timestamp).
			Msg("Using cached orderbook data")
		return cached.OrderBook, nil
	}

	// Fetch from Coinbase API
	orderBook, err := c.fetchFromAPI(ctx, symbol)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch from Coinbase API: %w", err)
	}

	// Cache the result
	c.cache.Set(symbol, orderBook)

	log.Info().
		Str("symbol", symbol).
		Str("venue", "coinbase").
		Float64("spread_bps", orderBook.SpreadBPS).
		Float64("depth_usd", orderBook.DepthUSDPlusMinus2Pct).
		Msg("Fetched fresh orderbook from Coinbase")

	return orderBook, nil
}

// fetchFromAPI makes the actual HTTP request to Coinbase
func (c *OrderBookClient) fetchFromAPI(ctx context.Context, symbol string) (*types.OrderBook, error) {
	// Get current time for monotonic timestamp
	fetchTime := time.Now()

	// Coinbase book endpoint - level=2 gives full depth
	url := fmt.Sprintf("%s/products/%s/book?level=2", c.baseURL, symbol)

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
		return nil, fmt.Errorf("Coinbase API error: %d %s", resp.StatusCode, string(body))
	}

	var coinbaseBook CoinbaseBookResponse
	if err := json.NewDecoder(resp.Body).Decode(&coinbaseBook); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Convert to normalized OrderBook format
	orderBook, err := c.convertToOrderBook(symbol, &coinbaseBook, fetchTime)
	if err != nil {
		return nil, fmt.Errorf("failed to convert orderbook: %w", err)
	}

	return orderBook, nil
}

// convertToOrderBook converts Coinbase format to normalized format
func (c *OrderBookClient) convertToOrderBook(symbol string, book *CoinbaseBookResponse, fetchTime time.Time) (*types.OrderBook, error) {
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

	// Parse sequence number
	sequenceNum := int64(book.Sequence)

	return &types.OrderBook{
		Symbol:                symbol,
		Venue:                 "coinbase",
		TimestampMono:         fetchTime,
		SequenceNum:           sequenceNum,
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
func (c *OrderBookClient) calculateDepthPlusMinus2Pct(book *CoinbaseBookResponse, midPrice float64) float64 {
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

// CoinbaseBookResponse represents Coinbase API response format
type CoinbaseBookResponse struct {
	Sequence int        `json:"sequence"`
	Bids     [][]string `json:"bids"` // [price, size, num-orders]
	Asks     [][]string `json:"asks"` // [price, size, num-orders]
}
