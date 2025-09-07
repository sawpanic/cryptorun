package websocket

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// NormalizeTick converts venue-specific tick data to our standard format (exported for testing)
func (hsm *HotSetManager) NormalizeTick(venue string, message []byte) (*TickUpdate, error) {
	return hsm.normalizeTick(venue, message)
}

// normalizeTick converts venue-specific tick data to our standard format
func (hsm *HotSetManager) normalizeTick(venue string, message []byte) (*TickUpdate, error) {
	switch strings.ToLower(venue) {
	case "kraken":
		return hsm.normalizeKrakenTick(message)
	case "binance":
		return hsm.normalizeBinanceTick(message)
	case "coinbase":
		return hsm.normalizeCoinbaseTick(message)
	case "okx":
		return hsm.normalizeOKXTick(message)
	default:
		return nil, fmt.Errorf("unsupported venue: %s", venue)
	}
}

// Kraken WebSocket message structures
type KrakenTickerUpdate struct {
	ChannelID   int          `json:"channelID"`
	ChannelName string       `json:"channelName"`
	Pair        string       `json:"pair"`
	Data        KrakenTicker `json:"data"`
}

type KrakenTicker struct {
	Ask    []string `json:"a"` // [price, wholeLotVolume, lotVolume]
	Bid    []string `json:"b"` // [price, wholeLotVolume, lotVolume]
	Close  []string `json:"c"` // [price, lotVolume]
	Volume []string `json:"v"` // [today, last24h]
	High   []string `json:"h"` // [today, last24h]
	Low    []string `json:"l"` // [today, last24h]
}

// normalizeKrakenTick converts Kraken WebSocket messages to TickUpdate
func (hsm *HotSetManager) normalizeKrakenTick(message []byte) (*TickUpdate, error) {
	// Kraken sends array messages: [channelID, data, channelName, pair]
	var rawMessage []interface{}
	if err := json.Unmarshal(message, &rawMessage); err != nil {
		return nil, fmt.Errorf("failed to unmarshal Kraken message: %w", err)
	}

	// Check if this is a ticker update
	if len(rawMessage) < 4 {
		return nil, nil // Not a ticker message
	}

	channelName, ok := rawMessage[2].(string)
	if !ok || channelName != "ticker" {
		return nil, nil // Not a ticker message
	}

	pair, ok := rawMessage[3].(string)
	if !ok {
		return nil, fmt.Errorf("invalid pair in Kraken message")
	}

	// Parse ticker data
	dataMap, ok := rawMessage[1].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid ticker data in Kraken message")
	}

	tick := &TickUpdate{
		Venue:     "kraken",
		Symbol:    hsm.normalizeKrakenSymbol(pair),
		Timestamp: time.Now().UTC(), // Kraken doesn't provide timestamp in ticker
	}

	// Parse ask data [price, wholeLotVolume, lotVolume]
	if askData, exists := dataMap["a"].([]interface{}); exists && len(askData) >= 3 {
		if askPrice, err := strconv.ParseFloat(askData[0].(string), 64); err == nil {
			tick.Ask = askPrice
		}
		if askSize, err := strconv.ParseFloat(askData[2].(string), 64); err == nil {
			tick.AskSize = askSize
		}
	}

	// Parse bid data [price, wholeLotVolume, lotVolume]
	if bidData, exists := dataMap["b"].([]interface{}); exists && len(bidData) >= 3 {
		if bidPrice, err := strconv.ParseFloat(bidData[0].(string), 64); err == nil {
			tick.Bid = bidPrice
		}
		if bidSize, err := strconv.ParseFloat(bidData[2].(string), 64); err == nil {
			tick.BidSize = bidSize
		}
	}

	// Parse last price from close data
	if closeData, exists := dataMap["c"].([]interface{}); exists && len(closeData) >= 1 {
		if lastPrice, err := strconv.ParseFloat(closeData[0].(string), 64); err == nil {
			tick.LastPrice = lastPrice
		}
	}

	// Parse 24h volume
	if volumeData, exists := dataMap["v"].([]interface{}); exists && len(volumeData) >= 2 {
		if volume24h, err := strconv.ParseFloat(volumeData[1].(string), 64); err == nil {
			// Convert to USD volume estimate (volume * last price)
			if tick.LastPrice > 0 {
				tick.Volume24h = volume24h * tick.LastPrice
			}
		}
	}

	return tick, nil
}

// normalizeKrakenSymbol converts Kraken pair names to standard format
func (hsm *HotSetManager) normalizeKrakenSymbol(krakenPair string) string {
	// Kraken uses formats like XXBTZUSD, XETHZUSD
	// Convert to BTCUSD, ETHUSD format

	// Remove X prefix and Z suffix for major currencies
	normalized := strings.ToUpper(krakenPair)

	// Handle common Kraken mappings
	replacements := map[string]string{
		"XXBTZUSD": "BTCUSD",
		"XETHZUSD": "ETHUSD",
		"XLTCZUSD": "LTCUSD",
		"XBCHZUSD": "BCHUSD",
		"XEOSZUSD": "EOSUSD",
		"XLMZUSD":  "XLMUSD",
		"XTZTZUSD": "XTZUSD",
		"ZUSD":     "USD",
	}

	// Apply replacements
	for old, new := range replacements {
		normalized = strings.Replace(normalized, old, new, -1)
	}

	// If still contains Z or X prefixes, try to clean up
	if strings.Contains(normalized, "Z") || strings.HasPrefix(normalized, "X") {
		// Remove X prefix
		if strings.HasPrefix(normalized, "X") && len(normalized) > 1 {
			normalized = normalized[1:]
		}
		// Replace ZUSD with USD
		normalized = strings.Replace(normalized, "ZUSD", "USD", -1)
	}

	return normalized
}

// Binance WebSocket structures
type BinanceTickerUpdate struct {
	Stream string            `json:"stream"`
	Data   BinanceTicker24hr `json:"data"`
}

type BinanceTicker24hr struct {
	Symbol             string `json:"s"`
	PriceChange        string `json:"P"`
	PriceChangePercent string `json:"p"`
	WeightedAvgPrice   string `json:"w"`
	LastPrice          string `json:"c"`
	LastQty            string `json:"Q"`
	BidPrice           string `json:"b"`
	BidQty             string `json:"B"`
	AskPrice           string `json:"a"`
	AskQty             string `json:"A"`
	OpenPrice          string `json:"o"`
	HighPrice          string `json:"h"`
	LowPrice           string `json:"l"`
	Volume             string `json:"v"`
	QuoteVolume        string `json:"q"`
	Count              int64  `json:"c"`
}

// normalizeBinanceTick converts Binance WebSocket messages to TickUpdate
func (hsm *HotSetManager) normalizeBinanceTick(message []byte) (*TickUpdate, error) {
	var binanceMsg BinanceTickerUpdate
	if err := json.Unmarshal(message, &binanceMsg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal Binance message: %w", err)
	}

	data := binanceMsg.Data

	tick := &TickUpdate{
		Venue:     "binance",
		Symbol:    data.Symbol, // Already in BTCUSDT format
		Timestamp: time.Now().UTC(),
	}

	// Parse numeric fields
	if bid, err := strconv.ParseFloat(data.BidPrice, 64); err == nil {
		tick.Bid = bid
	}

	if ask, err := strconv.ParseFloat(data.AskPrice, 64); err == nil {
		tick.Ask = ask
	}

	if bidSize, err := strconv.ParseFloat(data.BidQty, 64); err == nil {
		tick.BidSize = bidSize
	}

	if askSize, err := strconv.ParseFloat(data.AskQty, 64); err == nil {
		tick.AskSize = askSize
	}

	if lastPrice, err := strconv.ParseFloat(data.LastPrice, 64); err == nil {
		tick.LastPrice = lastPrice
	}

	if quoteVolume, err := strconv.ParseFloat(data.QuoteVolume, 64); err == nil {
		tick.Volume24h = quoteVolume // Already in quote currency (USD)
	}

	return tick, nil
}

// Coinbase WebSocket structures
type CoinbaseTickerUpdate struct {
	Type        string `json:"type"`
	Sequence    int64  `json:"sequence"`
	ProductID   string `json:"product_id"`
	Price       string `json:"price"`
	Open24h     string `json:"open_24h"`
	Volume24h   string `json:"volume_24h"`
	Low24h      string `json:"low_24h"`
	High24h     string `json:"high_24h"`
	Volume30d   string `json:"volume_30d"`
	BestBid     string `json:"best_bid"`
	BestAsk     string `json:"best_ask"`
	BestBidSize string `json:"best_bid_size"`
	BestAskSize string `json:"best_ask_size"`
	Time        string `json:"time"`
}

// normalizeCoinbaseTick converts Coinbase WebSocket messages to TickUpdate
func (hsm *HotSetManager) normalizeCoinbaseTick(message []byte) (*TickUpdate, error) {
	var coinbaseMsg CoinbaseTickerUpdate
	if err := json.Unmarshal(message, &coinbaseMsg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal Coinbase message: %w", err)
	}

	if coinbaseMsg.Type != "ticker" {
		return nil, nil // Not a ticker message
	}

	// Parse timestamp
	timestamp, err := time.Parse(time.RFC3339, coinbaseMsg.Time)
	if err != nil {
		timestamp = time.Now().UTC()
	}

	tick := &TickUpdate{
		Venue:     "coinbase",
		Symbol:    strings.Replace(coinbaseMsg.ProductID, "-", "", 1), // BTC-USD -> BTCUSD
		Timestamp: timestamp,
	}

	// Parse numeric fields
	if bid, err := strconv.ParseFloat(coinbaseMsg.BestBid, 64); err == nil {
		tick.Bid = bid
	}

	if ask, err := strconv.ParseFloat(coinbaseMsg.BestAsk, 64); err == nil {
		tick.Ask = ask
	}

	if bidSize, err := strconv.ParseFloat(coinbaseMsg.BestBidSize, 64); err == nil {
		tick.BidSize = bidSize
	}

	if askSize, err := strconv.ParseFloat(coinbaseMsg.BestAskSize, 64); err == nil {
		tick.AskSize = askSize
	}

	if lastPrice, err := strconv.ParseFloat(coinbaseMsg.Price, 64); err == nil {
		tick.LastPrice = lastPrice
	}

	if volume24h, err := strconv.ParseFloat(coinbaseMsg.Volume24h, 64); err == nil {
		// Convert base volume to USD volume
		if tick.LastPrice > 0 {
			tick.Volume24h = volume24h * tick.LastPrice
		}
	}

	return tick, nil
}

// OKX WebSocket structures
type OKXTickerUpdate struct {
	Arg  OKXArg      `json:"arg"`
	Data []OKXTicker `json:"data"`
}

type OKXArg struct {
	Channel string `json:"channel"`
	InstID  string `json:"instId"`
}

type OKXTicker struct {
	InstType  string `json:"instType"`
	InstID    string `json:"instId"`
	Last      string `json:"last"`
	LastSz    string `json:"lastSz"`
	AskPx     string `json:"askPx"`
	AskSz     string `json:"askSz"`
	BidPx     string `json:"bidPx"`
	BidSz     string `json:"bidSz"`
	Open24h   string `json:"open24h"`
	High24h   string `json:"high24h"`
	Low24h    string `json:"low24h"`
	VolCcy24h string `json:"volCcy24h"`
	Vol24h    string `json:"vol24h"`
	Ts        string `json:"ts"`
}

// normalizeOKXTick converts OKX WebSocket messages to TickUpdate
func (hsm *HotSetManager) normalizeOKXTick(message []byte) (*TickUpdate, error) {
	var okxMsg OKXTickerUpdate
	if err := json.Unmarshal(message, &okxMsg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal OKX message: %w", err)
	}

	if okxMsg.Arg.Channel != "tickers" || len(okxMsg.Data) == 0 {
		return nil, nil // Not a ticker message
	}

	data := okxMsg.Data[0]

	// Parse timestamp
	var timestamp time.Time
	if ts, err := strconv.ParseInt(data.Ts, 10, 64); err == nil {
		timestamp = time.Unix(0, ts*1e6).UTC()
	} else {
		timestamp = time.Now().UTC()
	}

	tick := &TickUpdate{
		Venue:     "okx",
		Symbol:    strings.Replace(data.InstID, "-", "", 1), // BTC-USD -> BTCUSD
		Timestamp: timestamp,
	}

	// Parse numeric fields
	if bid, err := strconv.ParseFloat(data.BidPx, 64); err == nil {
		tick.Bid = bid
	}

	if ask, err := strconv.ParseFloat(data.AskPx, 64); err == nil {
		tick.Ask = ask
	}

	if bidSize, err := strconv.ParseFloat(data.BidSz, 64); err == nil {
		tick.BidSize = bidSize
	}

	if askSize, err := strconv.ParseFloat(data.AskSz, 64); err == nil {
		tick.AskSize = askSize
	}

	if lastPrice, err := strconv.ParseFloat(data.Last, 64); err == nil {
		tick.LastPrice = lastPrice
	}

	if volume24h, err := strconv.ParseFloat(data.VolCcy24h, 64); err == nil {
		tick.Volume24h = volume24h // Already in quote currency (USD)
	}

	return tick, nil
}
