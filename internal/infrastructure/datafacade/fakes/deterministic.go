package fakes

import (
	"fmt"
	"hash/fnv"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/sawpanic/cryptorun/internal/domain/microstructure"
	"github.com/sawpanic/cryptorun/internal/domain/regime"
)

// DeterministicFakeProvider generates consistent fake data for testing
type DeterministicFakeProvider struct {
	baseTime time.Time
	symbols  []string
	seed     uint32
}

// NewDeterministicFakeProvider creates a new deterministic fake provider
func NewDeterministicFakeProvider(baseTime time.Time, symbols []string) *DeterministicFakeProvider {
	// Sort symbols for consistent ordering
	sortedSymbols := make([]string, len(symbols))
	copy(sortedSymbols, symbols)
	sort.Strings(sortedSymbols)
	
	return &DeterministicFakeProvider{
		baseTime: baseTime,
		symbols:  sortedSymbols,
		seed:     42, // Fixed seed for deterministic results
	}
}

// GetMicrostructureData generates deterministic microstructure data
func (dfp *DeterministicFakeProvider) GetMicrostructureData(symbol string, timestamp time.Time) microstructure.MicrostructureData {
	// Create deterministic hash based on symbol and timestamp
	hasher := fnv.New32a()
	hasher.Write([]byte(fmt.Sprintf("%s-%d", symbol, timestamp.Unix())))
	hash := hasher.Sum32()
	
	// Use hash to generate consistent but varied data
	rng := &deterministicRNG{seed: hash}
	
	// Base price varies by symbol
	basePrice := dfp.getBasePrice(symbol)
	
	// Add deterministic price variation based on timestamp
	timeFactor := math.Sin(float64(timestamp.Unix()) / 3600.0) // Hourly cycle
	priceVariation := timeFactor * 0.05 * basePrice // ±5% variation
	currentPrice := basePrice + priceVariation
	
	// Generate bid/ask with realistic spread
	spreadBps := 20 + rng.Float64()*30 // 20-50 bps spread
	spreadAbsolute := currentPrice * (spreadBps / 10000.0)
	
	bestBid := currentPrice - (spreadAbsolute / 2)
	bestAsk := currentPrice + (spreadAbsolute / 2)
	
	// Generate order book levels
	bidLevels := dfp.generateOrderLevels(bestBid, false, rng)
	askLevels := dfp.generateOrderLevels(bestAsk, true, rng)
	
	// Generate recent trades
	trades := dfp.generateRecentTrades(currentPrice, timestamp, rng)
	
	// Calculate volume and market data
	volume24h := 10000000 + rng.Float64()*90000000 // $10M-$100M daily volume
	marketCap := dfp.getMarketCap(symbol)
	circulatingSupply := dfp.getCirculatingSupply(symbol)
	
	return microstructure.MicrostructureData{
		Symbol:            symbol,
		Exchange:          dfp.getPreferredExchange(symbol),
		Timestamp:         timestamp,
		BestBid:           bestBid,
		BestAsk:           bestAsk,
		BidSize:           bidLevels[0].Size,
		AskSize:           askLevels[0].Size,
		OrderBook: microstructure.OrderBook{
			Bids:      bidLevels,
			Asks:      askLevels,
			Timestamp: timestamp,
			Sequence:  hash % 1000000,
		},
		RecentTrades:      trades,
		Volume24h:         volume24h,
		MarketCap:         marketCap,
		CirculatingSupply: circulatingSupply,
		Metadata: microstructure.MicrostructureMetadata{
			DataSource:       fmt.Sprintf("%s_fake_api", dfp.getPreferredExchange(symbol)),
			LastUpdate:       timestamp,
			Staleness:        rng.Float64() * 10.0, // 0-10 seconds staleness
			IsExchangeNative: true,
			APIEndpoint:      fmt.Sprintf("https://api.%s.com/v1/orderbook", dfp.getPreferredExchange(symbol)),
			RateLimit: microstructure.RateLimit{
				RequestsUsed:      int(hash % 50),
				RequestsRemaining: 150 + int(hash%50),
				ResetTimestamp:    timestamp.Add(time.Minute).Unix(),
			},
		},
	}
}

// GetRegimeData generates deterministic market regime data
func (dfp *DeterministicFakeProvider) GetRegimeData(timestamp time.Time) regime.MarketData {
	hasher := fnv.New32a()
	hasher.Write([]byte(fmt.Sprintf("regime-%d", timestamp.Unix())))
	hash := hasher.Sum32()
	rng := &deterministicRNG{seed: hash}
	
	// Generate regime indicators with daily cycles
	dayOfYear := timestamp.YearDay()
	hourOfDay := timestamp.Hour()
	
	// Realized volatility varies seasonally
	baseVol := 0.25 // 25% base annualized volatility
	seasonalFactor := math.Sin(float64(dayOfYear) / 365.0 * 2 * math.Pi) * 0.1
	hourlyFactor := math.Sin(float64(hourOfDay) / 24.0 * 2 * math.Pi) * 0.05
	realizedVol7d := baseVol + seasonalFactor + hourlyFactor
	
	// Moving average and current price
	basePrice := 50000.0 // BTC base price
	ma20 := basePrice * (1 + seasonalFactor*0.1)
	currentPrice := ma20 * (1 + (rng.Float64()-0.5)*0.1) // ±5% from MA
	
	// Market breadth indicators
	breadthBase := 0.5
	breadthVariation := math.Cos(float64(timestamp.Unix()) / 7200.0) * 0.3 // 2-hour cycle
	
	return regime.MarketData{
		Symbol:        "BTC-USD", // Primary symbol for regime detection
		Prices:        dfp.generatePriceSeries(currentPrice, timestamp, rng),
		Volumes:       dfp.generateVolumeSeries(rng),
		RealizedVol7d: realizedVol7d,
		MA20:          ma20,
		CurrentPrice:  currentPrice,
		BreadthData: regime.BreadthData{
			AdvanceDeclineRatio: math.Max(0, math.Min(1, breadthBase+breadthVariation)),
			NewHighsNewLows:     math.Max(0, math.Min(1, breadthBase+breadthVariation*0.8)),
			VolumeRatio:        math.Max(0, math.Min(1, breadthBase+breadthVariation*0.6)),
			Timestamp:          timestamp,
		},
		Timestamp: timestamp,
	}
}

// deterministicRNG provides deterministic random number generation
type deterministicRNG struct {
	seed uint32
}

func (r *deterministicRNG) Float64() float64 {
	r.seed = r.seed*1103515245 + 12345
	return float64(r.seed%10000) / 10000.0
}

func (r *deterministicRNG) Int(max int) int {
	r.seed = r.seed*1103515245 + 12345
	return int(r.seed) % max
}

// Helper methods for generating consistent data

func (dfp *DeterministicFakeProvider) getBasePrice(symbol string) float64 {
	prices := map[string]float64{
		"BTC-USD": 50000.0,
		"ETH-USD": 3000.0,
		"ADA-USD": 1.20,
		"DOT-USD": 25.0,
		"LINK-USD": 15.0,
		"LTC-USD": 150.0,
		"XLM-USD": 0.30,
		"XRP-USD": 0.60,
		"SOL-USD": 100.0,
		"MATIC-USD": 1.50,
	}
	
	if price, exists := prices[symbol]; exists {
		return price
	}
	
	// Generate deterministic price for unknown symbols
	hasher := fnv.New32a()
	hasher.Write([]byte(symbol))
	hash := hasher.Sum32()
	
	return float64(hash%10000) / 100.0 // $0.01 to $99.99
}

func (dfp *DeterministicFakeProvider) getMarketCap(symbol string) float64 {
	marketCaps := map[string]float64{
		"BTC-USD": 1000000000000.0, // $1T
		"ETH-USD": 400000000000.0,  // $400B
		"ADA-USD": 50000000000.0,   // $50B
		"DOT-USD": 30000000000.0,   // $30B
		"LINK-USD": 8000000000.0,   // $8B
		"LTC-USD": 12000000000.0,   // $12B
		"XLM-USD": 6000000000.0,    // $6B
		"XRP-USD": 35000000000.0,   // $35B
		"SOL-USD": 45000000000.0,   // $45B
		"MATIC-USD": 15000000000.0, // $15B
	}
	
	if cap, exists := marketCaps[symbol]; exists {
		return cap
	}
	
	return 1000000000.0 // $1B default
}

func (dfp *DeterministicFakeProvider) getCirculatingSupply(symbol string) float64 {
	supplies := map[string]float64{
		"BTC-USD": 19500000.0,     // ~19.5M BTC
		"ETH-USD": 120000000.0,    // ~120M ETH
		"ADA-USD": 35000000000.0,  // ~35B ADA
		"DOT-USD": 1200000000.0,   // ~1.2B DOT
		"LINK-USD": 500000000.0,   // ~500M LINK
		"LTC-USD": 75000000.0,     // ~75M LTC
		"XLM-USD": 25000000000.0,  // ~25B XLM
		"XRP-USD": 50000000000.0,  // ~50B XRP
		"SOL-USD": 400000000.0,    // ~400M SOL
		"MATIC-USD": 10000000000.0, // ~10B MATIC
	}
	
	if supply, exists := supplies[symbol]; exists {
		return supply
	}
	
	return 1000000000.0 // 1B default
}

func (dfp *DeterministicFakeProvider) getPreferredExchange(symbol string) string {
	// CryptoRun prefers Kraken for USD pairs
	if strings.HasSuffix(symbol, "-USD") {
		return "kraken"
	}
	return "binance"
}

func (dfp *DeterministicFakeProvider) generateOrderLevels(startPrice float64, isAsk bool, rng *deterministicRNG) []microstructure.OrderLevel {
	levels := make([]microstructure.OrderLevel, 5)
	
	for i := 0; i < 5; i++ {
		var price float64
		if isAsk {
			// Ask prices go up
			price = startPrice + (float64(i) * startPrice * 0.0005) // 5 bps increments
		} else {
			// Bid prices go down
			price = startPrice - (float64(i) * startPrice * 0.0005)
		}
		
		// Size decreases with distance from best price
		baseSize := 1.0 + rng.Float64()*2.0 // 1-3 base size
		sizeMultiplier := 1.0 / (1.0 + float64(i)*0.3) // Decreasing multiplier
		size := baseSize * sizeMultiplier
		
		sizeUSD := size * price
		orderCount := 1 + rng.Int(5) // 1-5 orders per level
		
		levels[i] = microstructure.OrderLevel{
			Price:      price,
			Size:       size,
			SizeUSD:    sizeUSD,
			OrderCount: orderCount,
		}
	}
	
	return levels
}

func (dfp *DeterministicFakeProvider) generateRecentTrades(price float64, timestamp time.Time, rng *deterministicRNG) []microstructure.Trade {
	trades := make([]microstructure.Trade, 3)
	
	for i := 0; i < 3; i++ {
		// Trades within last few minutes
		tradeTime := timestamp.Add(-time.Duration(rng.Int(300)) * time.Second)
		
		// Price varies slightly around current price
		tradePrice := price * (1 + (rng.Float64()-0.5)*0.002) // ±0.2% variation
		
		// Random size
		tradeSize := 0.1 + rng.Float64()*2.0 // 0.1-2.1 size
		
		// Random side
		side := "buy"
		if rng.Float64() > 0.5 {
			side = "sell"
		}
		
		trades[i] = microstructure.Trade{
			Price:     tradePrice,
			Size:      tradeSize,
			Side:      side,
			Timestamp: tradeTime,
			TradeID:   fmt.Sprintf("t%d", timestamp.Unix()+int64(i)),
		}
	}
	
	return trades
}

func (dfp *DeterministicFakeProvider) generatePriceSeries(currentPrice float64, timestamp time.Time, rng *deterministicRNG) []float64 {
	// Generate 24 hourly prices leading up to current time
	prices := make([]float64, 24)
	
	for i := 0; i < 24; i++ {
		// Simple random walk with mean reversion
		hours := 23 - i
		timeFactor := math.Exp(-float64(hours) * 0.1) // Exponential decay
		variation := (rng.Float64() - 0.5) * 0.02 * timeFactor // ±1% max variation
		prices[i] = currentPrice * (1 + variation)
	}
	
	return prices
}

func (dfp *DeterministicFakeProvider) generateVolumeSeries(rng *deterministicRNG) []float64 {
	// Generate 24 hourly volumes
	volumes := make([]float64, 24)
	
	for i := 0; i < 24; i++ {
		// Higher volume during "market hours" (UTC)
		hour := i
		volumeMultiplier := 1.0
		if hour >= 8 && hour <= 16 { // Business hours
			volumeMultiplier = 1.5
		}
		
		baseVolume := 1000000 + rng.Float64()*9000000 // $1M-$10M per hour
		volumes[i] = baseVolume * volumeMultiplier
	}
	
	return volumes
}

// GetSupportedSymbols returns the list of supported symbols
func (dfp *DeterministicFakeProvider) GetSupportedSymbols() []string {
	return dfp.symbols
}

// SetSeed allows changing the randomization seed for testing
func (dfp *DeterministicFakeProvider) SetSeed(seed uint32) {
	dfp.seed = seed
}