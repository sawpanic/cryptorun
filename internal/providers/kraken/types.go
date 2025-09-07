package kraken

import (
	"encoding/json"
	"fmt"
	"strconv"
	"time"
)

// KrakenResponse represents the standard Kraken API response wrapper
type KrakenResponse struct {
	Error  []string        `json:"error"`
	Result json.RawMessage `json:"result"`
}

// ServerTimeResponse holds Kraken server time information
type ServerTimeResponse struct {
	UnixTime int64  `json:"unixtime"`
	RFC1123  string `json:"rfc1123"`
}

// TickerInfo represents ticker data for a trading pair
type TickerInfo struct {
	Ask                 []string `json:"a"` // [price, whole_lot_volume, lot_volume]
	Bid                 []string `json:"b"` // [price, whole_lot_volume, lot_volume]
	LastTradeClosed     []string `json:"c"` // [price, lot_volume]
	Volume              []string `json:"v"` // [today, last_24h]
	VolumeWeightedPrice []string `json:"p"` // [today, last_24h]
	NumberOfTrades      []int    `json:"t"` // [today, last_24h]
	Low                 []string `json:"l"` // [today, last_24h]
	High                []string `json:"h"` // [today, last_24h]
	OpeningPrice        string   `json:"o"` // today's opening price
}

// GetAskPrice returns the best ask price as float64
func (t *TickerInfo) GetAskPrice() (float64, error) {
	if len(t.Ask) == 0 {
		return 0, fmt.Errorf("no ask price available")
	}
	return strconv.ParseFloat(t.Ask[0], 64)
}

// GetBidPrice returns the best bid price as float64
func (t *TickerInfo) GetBidPrice() (float64, error) {
	if len(t.Bid) == 0 {
		return 0, fmt.Errorf("no bid price available")
	}
	return strconv.ParseFloat(t.Bid[0], 64)
}

// GetSpreadBps calculates spread in basis points
func (t *TickerInfo) GetSpreadBps() (float64, error) {
	askPrice, err := t.GetAskPrice()
	if err != nil {
		return 0, err
	}
	
	bidPrice, err := t.GetBidPrice()
	if err != nil {
		return 0, err
	}
	
	if bidPrice <= 0 {
		return 0, fmt.Errorf("invalid bid price: %f", bidPrice)
	}
	
	spread := (askPrice - bidPrice) / bidPrice
	return spread * 10000, nil // Convert to basis points
}

// GetMidPrice calculates the mid price
func (t *TickerInfo) GetMidPrice() (float64, error) {
	askPrice, err := t.GetAskPrice()
	if err != nil {
		return 0, err
	}
	
	bidPrice, err := t.GetBidPrice()
	if err != nil {
		return 0, err
	}
	
	return (askPrice + bidPrice) / 2.0, nil
}

// Get24hVolume returns 24h volume as float64
func (t *TickerInfo) Get24hVolume() (float64, error) {
	if len(t.Volume) < 2 {
		return 0, fmt.Errorf("no 24h volume available")
	}
	return strconv.ParseFloat(t.Volume[1], 64)
}

// OrderBookResponse represents the order book response
type OrderBookResponse struct {
	Pair string         `json:"pair"`
	Data *OrderBookData `json:"data"`
}

// OrderBookData contains L2 order book information
type OrderBookData struct {
	Asks [][]string `json:"asks"` // [price, volume, timestamp]
	Bids [][]string `json:"bids"` // [price, volume, timestamp]
}

// OrderBookLevel represents a single price level in the order book
type OrderBookLevel struct {
	Price     float64   `json:"price"`
	Volume    float64   `json:"volume"`
	Timestamp time.Time `json:"timestamp"`
}

// GetBestAsk returns the best ask price and volume
func (ob *OrderBookData) GetBestAsk() (*OrderBookLevel, error) {
	if len(ob.Asks) == 0 {
		return nil, fmt.Errorf("no ask levels available")
	}
	
	return parseOrderLevel(ob.Asks[0])
}

// GetBestBid returns the best bid price and volume
func (ob *OrderBookData) GetBestBid() (*OrderBookLevel, error) {
	if len(ob.Bids) == 0 {
		return nil, fmt.Errorf("no bid levels available")
	}
	
	return parseOrderLevel(ob.Bids[0])
}

// CalculateDepthUSD calculates depth within percentage range in USD
func (ob *OrderBookData) CalculateDepthUSD(midPrice float64, percentRange float64) (bidDepth, askDepth float64, err error) {
	if midPrice <= 0 || percentRange <= 0 {
		return 0, 0, fmt.Errorf("invalid parameters: midPrice=%f, percentRange=%f", midPrice, percentRange)
	}
	
	lowerBound := midPrice * (1 - percentRange/100)
	upperBound := midPrice * (1 + percentRange/100)
	
	// Calculate bid depth (within range above lowerBound)
	for _, bidLevel := range ob.Bids {
		if len(bidLevel) < 2 {
			continue
		}
		
		price, err := strconv.ParseFloat(bidLevel[0], 64)
		if err != nil {
			continue
		}
		
		if price >= lowerBound {
			volume, err := strconv.ParseFloat(bidLevel[1], 64)
			if err != nil {
				continue
			}
			bidDepth += price * volume // USD value
		}
	}
	
	// Calculate ask depth (within range below upperBound)
	for _, askLevel := range ob.Asks {
		if len(askLevel) < 2 {
			continue
		}
		
		price, err := strconv.ParseFloat(askLevel[0], 64)
		if err != nil {
			continue
		}
		
		if price <= upperBound {
			volume, err := strconv.ParseFloat(askLevel[1], 64)
			if err != nil {
				continue
			}
			askDepth += price * volume // USD value
		}
	}
	
	return bidDepth, askDepth, nil
}

// TradeResponse represents recent trades data
type TradeResponse struct {
	Pair   string       `json:"pair"`
	Trades []TradeEntry `json:"trades"`
	Last   string       `json:"last"`
}

// TradeEntry represents a single trade
type TradeEntry struct {
	Price      string    `json:"price"`
	Volume     string    `json:"volume"`
	Time       float64   `json:"time"`
	BuyOrSell  string    `json:"buy_or_sell"` // "b" or "s"
	MarketOrLimit string `json:"market_or_limit"` // "m" or "l"
	Timestamp  time.Time `json:"-"` // Calculated from Time
}

// GetTimestamp returns the trade timestamp
func (te *TradeEntry) GetTimestamp() time.Time {
	return time.Unix(int64(te.Time), int64((te.Time-float64(int64(te.Time)))*1e9))
}

// GetPriceFloat returns price as float64
func (te *TradeEntry) GetPriceFloat() (float64, error) {
	return strconv.ParseFloat(te.Price, 64)
}

// GetVolumeFloat returns volume as float64
func (te *TradeEntry) GetVolumeFloat() (float64, error) {
	return strconv.ParseFloat(te.Volume, 64)
}

// IsBuy returns true if this is a buy trade
func (te *TradeEntry) IsBuy() bool {
	return te.BuyOrSell == "b"
}

// HealthStatus represents the health status of the Kraken client
type HealthStatus struct {
	Healthy bool          `json:"healthy"`
	Status  string        `json:"status"`
	Errors  []string      `json:"errors,omitempty"`
	Metrics HealthMetrics `json:"metrics"`
}

// HealthMetrics provides operational metrics for health monitoring
type HealthMetrics struct {
	ServerTime         int64     `json:"server_time"`
	RateLimitRemaining float64   `json:"rate_limit_remaining"`
	LastRequestTime    time.Time `json:"last_request_time"`
	WebSocketHealthy   bool      `json:"websocket_healthy"`
	RequestLatencyMS   float64   `json:"request_latency_ms,omitempty"`
}

// WebSocketMessage represents a generic WebSocket message
type WebSocketMessage struct {
	Event       string                 `json:"event,omitempty"`
	Pair        []string              `json:"pair,omitempty"`
	Subscription map[string]interface{} `json:"subscription,omitempty"`
	ChannelID   int                    `json:"channelID,omitempty"`
	ChannelName string                 `json:"channelName,omitempty"`
	Data        json.RawMessage        `json:"data,omitempty"`
}

// SubscriptionRequest represents a WebSocket subscription request
type SubscriptionRequest struct {
	Event        string                 `json:"event"`
	Pair         []string              `json:"pair,omitempty"`
	Subscription map[string]interface{} `json:"subscription"`
}

// Helper functions

func parseOrderLevel(level []string) (*OrderBookLevel, error) {
	if len(level) < 2 {
		return nil, fmt.Errorf("invalid order level format")
	}
	
	price, err := strconv.ParseFloat(level[0], 64)
	if err != nil {
		return nil, fmt.Errorf("invalid price: %w", err)
	}
	
	volume, err := strconv.ParseFloat(level[1], 64)
	if err != nil {
		return nil, fmt.Errorf("invalid volume: %w", err)
	}
	
	var timestamp time.Time
	if len(level) >= 3 {
		if ts, err := strconv.ParseFloat(level[2], 64); err == nil {
			timestamp = time.Unix(int64(ts), int64((ts-float64(int64(ts)))*1e9))
		}
	}
	
	return &OrderBookLevel{
		Price:     price,
		Volume:    volume,
		Timestamp: timestamp,
	}, nil
}

