package common

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"
)

// UniverseBuilder constructs trading universes for benchmarking
type UniverseBuilder struct {
	httpClient *http.Client
}

// NewUniverseBuilder creates a universe builder
func NewUniverseBuilder() *UniverseBuilder {
	return &UniverseBuilder{
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

// UniverseSpec defines how to build a trading universe
type UniverseSpec struct {
	Type      string  `json:"type"`        // "topN" or "path"
	Value     string  `json:"value"`       // "30" for topN, file path for path
	MinUSDAdv float64 `json:"min_usd_adv"` // Minimum USD average daily volume
}

// Asset represents a trading asset in the universe
type Asset struct {
	Symbol       string  `json:"symbol"`
	Name         string  `json:"name"`
	MarketCapUSD float64 `json:"market_cap_usd"`
	VolumeUSD24h float64 `json:"volume_usd_24h"`
	PriceUSD     float64 `json:"price_usd"`
	HasUSDPair   bool    `json:"has_usd_pair"`
	FXRate       float64 `json:"fx_rate,omitempty"` // If converted via FX
	FXNote       string  `json:"fx_note,omitempty"` // Conversion note
	DataProvider string  `json:"data_provider"`
}

// BuildUniverse constructs a trading universe based on the spec
func (ub *UniverseBuilder) BuildUniverse(ctx context.Context, spec UniverseSpec) ([]Asset, error) {
	switch spec.Type {
	case "topN":
		return ub.buildTopNUniverse(ctx, spec)
	case "path":
		return ub.loadUniverseFromPath(spec.Value)
	default:
		return nil, fmt.Errorf("unsupported universe type: %s", spec.Type)
	}
}

// buildTopNUniverse fetches top N assets from CoinGecko by market cap
func (ub *UniverseBuilder) buildTopNUniverse(ctx context.Context, spec UniverseSpec) ([]Asset, error) {
	// Parse topN value
	var topN int
	if _, err := fmt.Sscanf(spec.Value, "%d", &topN); err != nil {
		return nil, fmt.Errorf("invalid topN value: %s", spec.Value)
	}

	// Fetch from CoinGecko API (free tier)
	url := fmt.Sprintf("https://api.coingecko.com/api/v3/coins/markets?vs_currency=usd&order=market_cap_desc&per_page=%d&page=1&sparkline=false", topN)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := ub.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("CoinGecko request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("CoinGecko API error: %d %s", resp.StatusCode, string(body))
	}

	var geckoData []CoinGeckoMarket
	if err := json.NewDecoder(resp.Body).Decode(&geckoData); err != nil {
		return nil, fmt.Errorf("failed to decode CoinGecko response: %w", err)
	}

	// Convert to our Asset format
	assets := make([]Asset, 0, len(geckoData))
	for _, coin := range geckoData {
		// Skip if volume too low
		if coin.TotalVolume < spec.MinUSDAdv {
			continue
		}

		asset := Asset{
			Symbol:       strings.ToUpper(coin.Symbol),
			Name:         coin.Name,
			MarketCapUSD: coin.MarketCap,
			VolumeUSD24h: coin.TotalVolume,
			PriceUSD:     coin.CurrentPrice,
			HasUSDPair:   ub.hasUSDPair(coin.Symbol),
			DataProvider: "coingecko",
		}

		// Handle FX conversion if no direct USD pair
		if !asset.HasUSDPair {
			asset.FXRate = 1.0 // Simplified - use CoinGecko USD price directly
			asset.FXNote = "converted_via_coingecko_usd_price"
		}

		assets = append(assets, asset)
	}

	// Sort by market cap descending
	sort.Slice(assets, func(i, j int) bool {
		return assets[i].MarketCapUSD > assets[j].MarketCapUSD
	})

	return assets, nil
}

// hasUSDPair checks if an asset has a direct USD trading pair
func (ub *UniverseBuilder) hasUSDPair(symbol string) bool {
	// Common USD pairs (simplified check)
	usdPairs := map[string]bool{
		"BTC": true, "ETH": true, "USDT": true, "USDC": true,
		"BNB": true, "SOL": true, "ADA": true, "AVAX": true,
		"DOT": true, "LINK": true, "MATIC": true, "UNI": true,
		"LTC": true, "BCH": true, "XRP": true, "DOGE": true,
	}
	return usdPairs[strings.ToUpper(symbol)]
}

// loadUniverseFromPath loads a universe from a file path
func (ub *UniverseBuilder) loadUniverseFromPath(path string) ([]Asset, error) {
	// For now, return error - file-based universe loading can be added later
	return nil, fmt.Errorf("file-based universe loading not implemented: %s", path)
}

// FilterUSDOnly removes non-USD pairs from the universe
func FilterUSDOnly(assets []Asset) []Asset {
	filtered := make([]Asset, 0, len(assets))
	for _, asset := range assets {
		if asset.HasUSDPair || asset.FXRate > 0 {
			filtered = append(filtered, asset)
		}
	}
	return filtered
}

// CoinGeckoMarket represents a coin from CoinGecko markets API
type CoinGeckoMarket struct {
	ID                 string  `json:"id"`
	Symbol             string  `json:"symbol"`
	Name               string  `json:"name"`
	CurrentPrice       float64 `json:"current_price"`
	MarketCap          float64 `json:"market_cap"`
	TotalVolume        float64 `json:"total_volume"`
	PriceChangePerc24h float64 `json:"price_change_percentage_24h"`
}
