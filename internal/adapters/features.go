package adapters

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"strings"
	"time"
)

// FeaturesAdapter provides shared feature extraction for both scoring systems
type FeaturesAdapter struct {
	httpClient *http.Client
	cache      map[string]*CachedFeatures
}

// NewFeaturesAdapter creates a features adapter
func NewFeaturesAdapter() *FeaturesAdapter {
	return &FeaturesAdapter{
		httpClient: &http.Client{Timeout: 30 * time.Second},
		cache:      make(map[string]*CachedFeatures),
	}
}

// Features contains all features needed for both unified and legacy scoring
type Features struct {
	Symbol    string    `json:"symbol"`
	Timestamp time.Time `json:"timestamp"`

	// Price data
	Returns1h  float64 `json:"returns_1h"`
	Returns4h  float64 `json:"returns_4h"`
	Returns12h float64 `json:"returns_12h"`
	Returns24h float64 `json:"returns_24h"`

	// Volume data
	Volume24h   float64 `json:"volume_24h"`
	VolumeAvg   float64 `json:"volume_avg"`
	VolumeRatio float64 `json:"volume_ratio"`

	// Technical indicators
	RSI        float64 `json:"rsi"`
	MACD       float64 `json:"macd"`
	Volatility float64 `json:"volatility"`

	// Social/sentiment
	SocialSentiment float64 `json:"social_sentiment"` // -1 to +1

	// Attribution
	DataSources map[string]string `json:"data_sources"`
	CacheHit    bool              `json:"cache_hit"`
}

// CachedFeatures wraps features with cache metadata
type CachedFeatures struct {
	Features *Features     `json:"features"`
	CachedAt time.Time     `json:"cached_at"`
	TTL      time.Duration `json:"ttl"`
}

// GetFeatures retrieves all features for an asset at a specific time
func (fa *FeaturesAdapter) GetFeatures(ctx context.Context, symbol string, timestamp time.Time) (*Features, error) {
	cacheKey := fmt.Sprintf("%s_%d", symbol, timestamp.Unix())

	// Check cache first
	if cached, exists := fa.cache[cacheKey]; exists {
		if time.Since(cached.CachedAt) < cached.TTL {
			cached.Features.CacheHit = true
			return cached.Features, nil
		}
		// Expired - remove from cache
		delete(fa.cache, cacheKey)
	}

	// Fetch fresh data
	features, err := fa.fetchFeatures(ctx, symbol, timestamp)
	if err != nil {
		return nil, err
	}

	// Cache the result
	fa.cache[cacheKey] = &CachedFeatures{
		Features: features,
		CachedAt: time.Now(),
		TTL:      300 * time.Second, // 5 minutes
	}

	features.CacheHit = false
	return features, nil
}

// fetchFeatures retrieves features from external APIs
func (fa *FeaturesAdapter) fetchFeatures(ctx context.Context, symbol string, timestamp time.Time) (*Features, error) {
	features := &Features{
		Symbol:      symbol,
		Timestamp:   timestamp,
		DataSources: make(map[string]string),
	}

	// Fetch price data from CoinGecko (free API)
	priceData, err := fa.fetchCoinGeckoPriceData(ctx, symbol)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch price data: %w", err)
	}

	// Extract returns
	features.Returns1h = priceData.PriceChangePercentage1h / 100.0
	features.Returns4h = priceData.PriceChangePercentage4h / 100.0
	features.Returns12h = priceData.PriceChangePercentage12h / 100.0
	features.Returns24h = priceData.PriceChangePercentage24h / 100.0

	// Extract volume data
	features.Volume24h = priceData.TotalVolume
	features.VolumeAvg = priceData.TotalVolume * 0.8 // Mock average (80% of current)
	features.VolumeRatio = features.Volume24h / features.VolumeAvg

	// Mock technical indicators (in practice, these would be calculated)
	features.RSI = 50 + (features.Returns24h * 200)          // Mock RSI
	features.MACD = features.Returns4h - features.Returns12h // Mock MACD
	features.Volatility = math.Abs(features.Returns24h)      // Mock volatility

	// Mock social sentiment (would come from social APIs)
	features.SocialSentiment = math.Tanh(features.Returns24h * 5) // Mock sentiment

	features.DataSources["prices"] = "coingecko"
	features.DataSources["volume"] = "coingecko"
	features.DataSources["technical"] = "computed"
	features.DataSources["social"] = "mock"

	return features, nil
}

// fetchCoinGeckoPriceData fetches price/volume data from CoinGecko
func (fa *FeaturesAdapter) fetchCoinGeckoPriceData(ctx context.Context, symbol string) (*CoinGeckoData, error) {
	// Map common symbols to CoinGecko IDs
	geckoID := fa.symbolToGeckoID(symbol)

	url := fmt.Sprintf("https://api.coingecko.com/api/v3/coins/%s?localization=false&tickers=false&market_data=true&community_data=false&developer_data=false&sparkline=false", geckoID)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := fa.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("CoinGecko request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("CoinGecko API error: %d %s", resp.StatusCode, string(body))
	}

	var coinData CoinGeckoCoinData
	if err := json.NewDecoder(resp.Body).Decode(&coinData); err != nil {
		return nil, fmt.Errorf("failed to decode CoinGecko response: %w", err)
	}

	// Extract the data we need
	data := &CoinGeckoData{
		CurrentPrice:             coinData.MarketData.CurrentPrice.USD,
		TotalVolume:              coinData.MarketData.TotalVolume.USD,
		PriceChangePercentage1h:  coinData.MarketData.PriceChangePercentage1hInCurrency.USD,
		PriceChangePercentage4h:  coinData.MarketData.PriceChangePercentage1hInCurrency.USD * 2, // Mock 4h
		PriceChangePercentage12h: coinData.MarketData.PriceChangePercentage1hInCurrency.USD * 3, // Mock 12h
		PriceChangePercentage24h: coinData.MarketData.PriceChangePercentage24hInCurrency.USD,
	}

	return data, nil
}

// symbolToGeckoID maps trading symbols to CoinGecko coin IDs
func (fa *FeaturesAdapter) symbolToGeckoID(symbol string) string {
	// Common mappings
	mappings := map[string]string{
		"BTC":   "bitcoin",
		"ETH":   "ethereum",
		"BNB":   "binancecoin",
		"SOL":   "solana",
		"ADA":   "cardano",
		"AVAX":  "avalanche-2",
		"DOT":   "polkadot",
		"LINK":  "chainlink",
		"MATIC": "matic-network",
		"UNI":   "uniswap",
		"LTC":   "litecoin",
		"BCH":   "bitcoin-cash",
		"XRP":   "ripple",
		"DOGE":  "dogecoin",
	}

	if geckoID, exists := mappings[symbol]; exists {
		return geckoID
	}

	// Default: lowercase symbol
	return fmt.Sprintf("%s", strings.ToLower(symbol))
}

// CoinGeckoData contains extracted price/volume data
type CoinGeckoData struct {
	CurrentPrice             float64 `json:"current_price"`
	TotalVolume              float64 `json:"total_volume"`
	PriceChangePercentage1h  float64 `json:"price_change_percentage_1h"`
	PriceChangePercentage4h  float64 `json:"price_change_percentage_4h"`
	PriceChangePercentage12h float64 `json:"price_change_percentage_12h"`
	PriceChangePercentage24h float64 `json:"price_change_percentage_24h"`
}

// CoinGeckoCoinData represents the full CoinGecko coin API response
type CoinGeckoCoinData struct {
	MarketData CoinGeckoMarketData `json:"market_data"`
}

// CoinGeckoMarketData contains market data from CoinGecko
type CoinGeckoMarketData struct {
	CurrentPrice                       CurrencyValue `json:"current_price"`
	TotalVolume                        CurrencyValue `json:"total_volume"`
	PriceChangePercentage1hInCurrency  CurrencyValue `json:"price_change_percentage_1h_in_currency"`
	PriceChangePercentage24hInCurrency CurrencyValue `json:"price_change_percentage_24h_in_currency"`
}

// CurrencyValue represents a value in multiple currencies
type CurrencyValue struct {
	USD float64 `json:"usd"`
}
