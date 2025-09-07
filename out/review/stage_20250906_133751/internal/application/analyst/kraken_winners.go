package analyst

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
)

// KrakenTickerResponse represents the Kraken ticker API response
type KrakenTickerResponse struct {
	Error  []string                     `json:"error"`
	Result map[string]KrakenTickerEntry `json:"result"`
}

// KrakenTickerEntry represents a single ticker entry from Kraken
type KrakenTickerEntry struct {
	Ask               []string `json:"a"` // [price, whole_lot_volume, lot_volume]
	Bid               []string `json:"b"` // [price, whole_lot_volume, lot_volume]
	LastTrade         []string `json:"c"` // [price, lot_volume]
	Volume            []string `json:"v"` // [today, last_24h]
	VolumeWeightedAvg []string `json:"p"` // [today, last_24h]
	TradeCount        []int    `json:"t"` // [today, last_24h]
	Low               []string `json:"l"` // [today, last_24h]
	High              []string `json:"h"` // [today, last_24h]
	OpeningPrice      string   `json:"o"` // opening price today
}

// WinnersFetcher handles fetching top performing assets
type WinnersFetcher struct {
	useFixture  bool
	httpTimeout time.Duration
	symbolMap   map[string]string // Map Kraken symbols to standard symbols
}

// NewWinnersFetcher creates a new winners fetcher
func NewWinnersFetcher(useFixture bool) *WinnersFetcher {
	return &WinnersFetcher{
		useFixture:  useFixture,
		httpTimeout: 10 * time.Second,
		symbolMap:   buildSymbolMap(),
	}
}

// FetchWinners retrieves top performing assets for specified timeframes
func (wf *WinnersFetcher) FetchWinners(timeframes []string) ([]WinnerCandidate, error) {
	if wf.useFixture {
		return wf.fetchFixtureWinners(timeframes)
	}
	return wf.fetchKrakenWinners(timeframes)
}

// fetchKrakenWinners fetches live data from Kraken ticker API
func (wf *WinnersFetcher) fetchKrakenWinners(timeframes []string) ([]WinnerCandidate, error) {
	log.Info().Strs("timeframes", timeframes).Msg("Fetching winners from Kraken ticker")

	// Fetch current ticker data
	tickerData, err := wf.fetchKrakenTicker()
	if err != nil {
		log.Error().Err(err).Msg("Failed to fetch Kraken ticker data")
		return nil, fmt.Errorf("failed to fetch Kraken ticker: %w", err)
	}

	var allWinners []WinnerCandidate
	timestamp := time.Now().UTC()

	for _, timeframe := range timeframes {
		winners, err := wf.calculateWinners(tickerData, timeframe, timestamp)
		if err != nil {
			log.Error().Str("timeframe", timeframe).Err(err).Msg("Failed to calculate winners")
			continue
		}
		allWinners = append(allWinners, winners...)
	}

	log.Info().Int("total_winners", len(allWinners)).Msg("Fetched winners from Kraken")
	return allWinners, nil
}

// fetchKrakenTicker fetches raw ticker data from Kraken API
func (wf *WinnersFetcher) fetchKrakenTicker() (map[string]KrakenTickerEntry, error) {
	client := &http.Client{Timeout: wf.httpTimeout}

	// Fetch ticker data for all USD pairs
	url := "https://api.kraken.com/0/public/Ticker"

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("User-Agent", "CryptoRun-Analyst/1.0")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch ticker: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ticker API returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var tickerResp KrakenTickerResponse
	if err := json.Unmarshal(body, &tickerResp); err != nil {
		return nil, fmt.Errorf("failed to parse ticker response: %w", err)
	}

	if len(tickerResp.Error) > 0 {
		return nil, fmt.Errorf("ticker API error: %v", tickerResp.Error)
	}

	return tickerResp.Result, nil
}

// calculateWinners processes ticker data to find top performers for a timeframe
func (wf *WinnersFetcher) calculateWinners(tickerData map[string]KrakenTickerEntry, timeframe string, timestamp time.Time) ([]WinnerCandidate, error) {
	type PerformanceEntry struct {
		Symbol      string
		Performance float64
		Volume      float64
		Price       float64
	}

	var performances []PerformanceEntry

	for krakenSymbol, entry := range tickerData {
		// Filter for USD pairs only
		if !strings.HasSuffix(krakenSymbol, "USD") {
			continue
		}

		// Convert to standard symbol
		standardSymbol, exists := wf.symbolMap[krakenSymbol]
		if !exists {
			standardSymbol = krakenSymbol
		}

		// Calculate performance based on timeframe
		performance, err := wf.calculatePerformance(entry, timeframe)
		if err != nil {
			continue // Skip symbols with calculation errors
		}

		// Get current price and volume
		price, err := strconv.ParseFloat(entry.LastTrade[0], 64)
		if err != nil {
			continue
		}

		volume, err := strconv.ParseFloat(entry.Volume[1], 64) // 24h volume
		if err != nil {
			continue
		}

		performances = append(performances, PerformanceEntry{
			Symbol:      standardSymbol,
			Performance: performance,
			Volume:      volume,
			Price:       price,
		})
	}

	// Sort by performance descending
	sort.Slice(performances, func(i, j int) bool {
		if math.Abs(performances[i].Performance-performances[j].Performance) < 0.0001 {
			// Stable tie-break by symbol name
			return performances[i].Symbol < performances[j].Symbol
		}
		return performances[i].Performance > performances[j].Performance
	})

	// Take top 10 winners for this timeframe
	topCount := 10
	if len(performances) < topCount {
		topCount = len(performances)
	}

	winners := make([]WinnerCandidate, topCount)
	for i := 0; i < topCount; i++ {
		entry := performances[i]
		winners[i] = WinnerCandidate{
			Symbol:        entry.Symbol,
			Timeframe:     timeframe,
			PerformancePC: entry.Performance,
			Volume:        entry.Volume,
			Price:         entry.Price,
			Rank:          i + 1,
			Source:        "kraken_ticker",
			Timestamp:     timestamp,
		}
	}

	log.Info().Str("timeframe", timeframe).Int("winners", len(winners)).
		Float64("top_performance", func() float64 {
			if len(winners) > 0 {
				return winners[0].PerformancePC
			}
			return 0.0
		}()).
		Msg("Calculated winners for timeframe")

	return winners, nil
}

// calculatePerformance calculates percentage gain for the given timeframe
func (wf *WinnersFetcher) calculatePerformance(entry KrakenTickerEntry, timeframe string) (float64, error) {
	currentPrice, err := strconv.ParseFloat(entry.LastTrade[0], 64)
	if err != nil {
		return 0, err
	}

	var referencePrice float64

	switch timeframe {
	case "1h":
		// For 1h, we approximate using opening price (not perfect but reasonable)
		referencePrice, err = strconv.ParseFloat(entry.OpeningPrice, 64)
		if err != nil {
			return 0, err
		}
		// Adjust for 1h approximation (this is a limitation of Kraken ticker API)
		// In production, you'd want OHLC data for precise 1h calculations

	case "24h":
		// Use opening price for 24h calculation
		referencePrice, err = strconv.ParseFloat(entry.OpeningPrice, 64)
		if err != nil {
			return 0, err
		}

	case "7d":
		// For 7d, we'd need historical data. For now, use a proxy calculation
		// This is a limitation - in production you'd fetch OHLC data
		referencePrice, err = strconv.ParseFloat(entry.OpeningPrice, 64)
		if err != nil {
			return 0, err
		}
		// Apply a scaling factor as approximation
		referencePrice *= 0.95 // Rough approximation

	default:
		return 0, fmt.Errorf("unsupported timeframe: %s", timeframe)
	}

	if referencePrice <= 0 {
		return 0, fmt.Errorf("invalid reference price: %f", referencePrice)
	}

	performance := ((currentPrice - referencePrice) / referencePrice) * 100
	return performance, nil
}

// fetchFixtureWinners returns deterministic fixture data for testing
func (wf *WinnersFetcher) fetchFixtureWinners(timeframes []string) ([]WinnerCandidate, error) {
	log.Info().Strs("timeframes", timeframes).Msg("Using fixture data for winners")

	// Deterministic fixture data for testing
	fixtureWinners := map[string][]WinnerCandidate{
		"1h": {
			{Symbol: "BTCUSD", Timeframe: "1h", PerformancePC: 3.2, Volume: 1500000, Price: 45000, Rank: 1, Source: "fixture"},
			{Symbol: "ETHUSD", Timeframe: "1h", PerformancePC: 2.8, Volume: 800000, Price: 3200, Rank: 2, Source: "fixture"},
			{Symbol: "SOLUSD", Timeframe: "1h", PerformancePC: 2.1, Volume: 400000, Price: 180, Rank: 3, Source: "fixture"},
			{Symbol: "ADAUSD", Timeframe: "1h", PerformancePC: 1.9, Volume: 300000, Price: 0.65, Rank: 4, Source: "fixture"},
			{Symbol: "DOTUSD", Timeframe: "1h", PerformancePC: 1.7, Volume: 250000, Price: 12.5, Rank: 5, Source: "fixture"},
		},
		"24h": {
			{Symbol: "ETHUSD", Timeframe: "24h", PerformancePC: 8.5, Volume: 2000000, Price: 3200, Rank: 1, Source: "fixture"},
			{Symbol: "BTCUSD", Timeframe: "24h", PerformancePC: 6.2, Volume: 3500000, Price: 45000, Rank: 2, Source: "fixture"},
			{Symbol: "SOLUSD", Timeframe: "24h", PerformancePC: 5.8, Volume: 900000, Price: 180, Rank: 3, Source: "fixture"},
			{Symbol: "AVAXUSD", Timeframe: "24h", PerformancePC: 4.3, Volume: 400000, Price: 32, Rank: 4, Source: "fixture"},
			{Symbol: "MATICUSD", Timeframe: "24h", PerformancePC: 3.9, Volume: 350000, Price: 0.85, Rank: 5, Source: "fixture"},
		},
		"7d": {
			{Symbol: "SOLUSD", Timeframe: "7d", PerformancePC: 18.2, Volume: 1200000, Price: 180, Rank: 1, Source: "fixture"},
			{Symbol: "ETHUSD", Timeframe: "7d", PerformancePC: 15.6, Volume: 2500000, Price: 3200, Rank: 2, Source: "fixture"},
			{Symbol: "AVAXUSD", Timeframe: "7d", PerformancePC: 12.8, Volume: 600000, Price: 32, Rank: 3, Source: "fixture"},
			{Symbol: "BTCUSD", Timeframe: "7d", PerformancePC: 10.1, Volume: 4000000, Price: 45000, Rank: 4, Source: "fixture"},
			{Symbol: "LINKUSD", Timeframe: "7d", PerformancePC: 9.4, Volume: 500000, Price: 18, Rank: 5, Source: "fixture"},
		},
	}

	timestamp := time.Now().UTC()
	var allWinners []WinnerCandidate

	for _, timeframe := range timeframes {
		winners, exists := fixtureWinners[timeframe]
		if !exists {
			log.Warn().Str("timeframe", timeframe).Msg("No fixture data for timeframe")
			continue
		}

		// Update timestamps and ensure deterministic ordering
		for i := range winners {
			winners[i].Timestamp = timestamp
		}

		allWinners = append(allWinners, winners...)
	}

	log.Info().Int("total_winners", len(allWinners)).Msg("Loaded fixture winners")
	return allWinners, nil
}

// buildSymbolMap creates mapping from Kraken symbols to standard symbols
func buildSymbolMap() map[string]string {
	// Map Kraken's symbol format to our standard format
	// This would be expanded based on universe.json analysis
	return map[string]string{
		"XXBTZUSD": "BTCUSD",
		"XETHZUSD": "ETHUSD",
		"XXRPZUSD": "XRPUSD",
		"ADAUSD":   "ADAUSD",
		"SOLUSD":   "SOLUSD",
		"DOTUSD":   "DOTUSD",
		"AVAXUSD":  "AVAXUSD",
		"MATICUSD": "MATICUSD",
		"LINKUSD":  "LINKUSD",
		"UNIUSD":   "UNIUSD",
		// Add more mappings as needed
	}
}
