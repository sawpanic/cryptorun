package march_aug

import (
	"fmt"
	"math"
	"math/rand"
	"time"
)

// MockDataSource implements DataSource interface with realistic market data for March-Aug 2025
type MockDataSource struct {
	universe []string
	seed     int64
}

// NewMockDataSource creates a new mock data source for backtesting
func NewMockDataSource(universe []string) *MockDataSource {
	return &MockDataSource{
		universe: universe,
		seed:     1234567890, // Fixed seed for reproducible results
	}
}

// GetMarketData generates realistic OHLCV data for the period
func (m *MockDataSource) GetMarketData(symbol string, start, end time.Time) ([]MarketData, error) {
	rand.Seed(m.seed + int64(len(symbol))) // Symbol-specific seed

	var data []MarketData
	current := start

	// Base prices for different symbols
	basePrice := m.getBasePrice(symbol)
	price := basePrice

	for current.Before(end) {
		// Generate realistic price movement with volatility clustering
		volatility := m.getVolatility(symbol, current)
		priceChange := rand.NormFloat64() * volatility * price
		price = math.Max(price+priceChange, price*0.01) // Prevent negative prices

		// Generate OHLC from close price
		spread := volatility * price * 0.3
		open := price * (1 + (rand.Float64()-0.5)*0.02)
		high := math.Max(open, price) + rand.Float64()*spread
		low := math.Min(open, price) - rand.Float64()*spread

		// Generate volume with mean reversion
		baseVolume := m.getBaseVolume(symbol)
		volumeMultiplier := 0.5 + rand.Float64()*2.0 // 0.5x to 2.5x base
		if math.Abs(priceChange/price) > 0.05 {      // High volatility = high volume
			volumeMultiplier *= 2.0
		}
		volume := baseVolume * volumeMultiplier

		venue := m.getVenue(symbol)

		data = append(data, MarketData{
			Symbol:    symbol,
			Timestamp: current,
			Open:      open,
			High:      high,
			Low:       low,
			Close:     price,
			Volume:    volume,
			Venue:     venue,
		})

		current = current.Add(time.Hour) // Hourly data
	}

	return data, nil
}

// GetFundingData generates venue-native funding rate data with median calculation
func (m *MockDataSource) GetFundingData(symbol string, start, end time.Time) ([]FundingData, error) {
	rand.Seed(m.seed + int64(len(symbol)*2))

	var data []FundingData
	current := start

	for current.Before(end) {
		// Generate funding rates for each venue (8h intervals)
		if current.Hour()%8 == 0 {
			binanceFR := (rand.NormFloat64() * 0.0005) + 0.0001 // Mean 0.01%, std 0.05%
			okxFR := (rand.NormFloat64() * 0.0004) + 0.0001
			bybitFR := (rand.NormFloat64() * 0.0006) + 0.0001

			// Calculate median
			rates := []float64{binanceFR, okxFR, bybitFR}
			medianFR := m.median(rates)

			// Calculate divergence in standard deviations
			stdDev := m.stdDev(rates)
			divergence := 0.0
			if stdDev > 0 {
				divergence = math.Abs(binanceFR-medianFR) / stdDev
			}

			data = append(data, FundingData{
				Symbol:     symbol,
				Timestamp:  current,
				BinanceFR:  binanceFR,
				OKXFR:      okxFR,
				BybitFR:    bybitFR,
				MedianFR:   medianFR,
				Divergence: divergence,
			})
		}
		current = current.Add(time.Hour)
	}

	return data, nil
}

// GetOpenInterestData generates OI data with residuals
func (m *MockDataSource) GetOpenInterestData(symbol string, start, end time.Time) ([]OpenInterestData, error) {
	rand.Seed(m.seed + int64(len(symbol)*3))

	var data []OpenInterestData
	current := start
	baseOI := m.getBaseOI(symbol)
	currentOI := baseOI

	for current.Before(end) {
		// OI changes with market activity
		oiChange := rand.NormFloat64() * 0.1 * currentOI     // 10% std dev
		currentOI = math.Max(currentOI+oiChange, baseOI*0.1) // Minimum 10% of base

		// Calculate 24h change
		oi24hAgo := currentOI * (0.8 + rand.Float64()*0.4) // Mock 24h ago value
		oiChange24h := (currentOI - oi24hAgo) / oi24hAgo

		// OI residual: excess OI change vs expected from price movement
		expectedOIChange := rand.Float64() * 0.05 // Mock expected change
		oiResidual := oiChange24h - expectedOIChange

		data = append(data, OpenInterestData{
			Symbol:       symbol,
			Timestamp:    current,
			OpenInterest: currentOI,
			OIChange24h:  oiChange24h,
			OIResidual:   oiResidual,
			Venue:        m.getVenue(symbol),
		})

		current = current.Add(4 * time.Hour) // 4-hour intervals
	}

	return data, nil
}

// GetReservesData generates exchange reserves data (BTC/ETH from Glassnode)
func (m *MockDataSource) GetReservesData(symbol string, start, end time.Time) ([]ReservesData, error) {
	rand.Seed(m.seed + int64(len(symbol)*4))

	var data []ReservesData
	current := start

	// Only BTC and ETH have robust reserves data
	available := symbol == "BTC-USD" || symbol == "ETH-USD"
	baseReserves := m.getBaseReserves(symbol)
	currentReserves := baseReserves

	for current.Before(end) {
		if available {
			// Reserves change slowly over time
			reservesChange := rand.NormFloat64() * 0.02 * currentReserves // 2% std dev
			currentReserves = math.Max(currentReserves+reservesChange, baseReserves*0.5)

			// Calculate percentage change
			reservesPct := reservesChange / currentReserves

			data = append(data, ReservesData{
				Symbol:      symbol,
				Timestamp:   current,
				Reserves:    currentReserves,
				ReservesPct: reservesPct,
				Available:   true,
			})
		} else {
			// Mark as N/A for alts
			data = append(data, ReservesData{
				Symbol:      symbol,
				Timestamp:   current,
				Reserves:    0,
				ReservesPct: 0,
				Available:   false,
			})
		}

		current = current.Add(24 * time.Hour) // Daily data
	}

	return data, nil
}

// GetCatalystData generates dated catalyst events with timing multipliers
func (m *MockDataSource) GetCatalystData(symbol string, start, end time.Time) ([]CatalystData, error) {
	rand.Seed(m.seed + int64(len(symbol)*5))

	var data []CatalystData
	current := start

	// Generate catalyst events throughout the period
	eventTypes := []string{"SEC_settlement", "hard_fork", "ETF_flow", "regulatory", "partnership"}

	for current.Before(end) {
		// Random event probability (roughly 1 event per month per symbol)
		if rand.Float64() < 0.001 { // 0.1% chance per hour
			eventType := eventTypes[rand.Intn(len(eventTypes))]
			impact := rand.Float64()*20 + 5 // 5-25 base impact

			// Calculate timing multiplier based on weeks until event
			weeksFromNow := float64(time.Until(current).Hours()) / (7 * 24)
			timingMult := m.getTimingMultiplier(weeksFromNow)

			data = append(data, CatalystData{
				Symbol:      symbol,
				Timestamp:   current,
				EventType:   eventType,
				Description: fmt.Sprintf("%s event for %s", eventType, symbol),
				Impact:      impact,
				TimingMult:  timingMult,
				HeatScore:   impact * timingMult,
			})
		}
		current = current.Add(6 * time.Hour) // Check every 6 hours
	}

	return data, nil
}

// GetSocialData generates Fear & Greed and search spike data
func (m *MockDataSource) GetSocialData(symbol string, start, end time.Time) ([]SocialData, error) {
	rand.Seed(m.seed + int64(len(symbol)*6))

	var data []SocialData
	current := start

	for current.Before(end) {
		// Fear & Greed index (0-100)
		fearGreed := math.Max(0, math.Min(100, rand.NormFloat64()*20+50))

		// Search spikes (relative to baseline)
		searchSpikes := math.Max(0, rand.NormFloat64()*0.5+1.0)

		// Combined social score
		socialScore := (fearGreed/100)*50 + (searchSpikes-1)*25 // 0-75 range

		data = append(data, SocialData{
			Symbol:       symbol,
			Timestamp:    current,
			FearGreed:    fearGreed,
			SearchSpikes: searchSpikes,
			SocialScore:  socialScore,
		})

		current = current.Add(4 * time.Hour) // 4-hour intervals
	}

	return data, nil
}

// Helper methods for realistic data generation

func (m *MockDataSource) getBasePrice(symbol string) float64 {
	prices := map[string]float64{
		"BTC-USD":   67000,
		"ETH-USD":   3200,
		"SOL-USD":   140,
		"ADA-USD":   1.2,
		"DOT-USD":   12,
		"AVAX-USD":  35,
		"LINK-USD":  18,
		"UNI-USD":   8.5,
		"AAVE-USD":  150,
		"MATIC-USD": 0.85,
	}
	if price, exists := prices[symbol]; exists {
		return price
	}
	return 5.0 // Default for unknown symbols
}

func (m *MockDataSource) getVolatility(symbol string, t time.Time) float64 {
	// Base volatility varies by asset
	baseVol := map[string]float64{
		"BTC-USD": 0.04, "ETH-USD": 0.05, "SOL-USD": 0.08,
		"ADA-USD": 0.07, "DOT-USD": 0.06, "AVAX-USD": 0.09,
	}

	vol := 0.06 // Default volatility
	if v, exists := baseVol[symbol]; exists {
		vol = v
	}

	// Volatility clustering - higher vol during market stress
	hour := t.Hour()
	if hour >= 13 && hour <= 16 { // US market hours = higher vol
		vol *= 1.3
	}

	return vol
}

func (m *MockDataSource) getBaseVolume(symbol string) float64 {
	volumes := map[string]float64{
		"BTC-USD": 25000, "ETH-USD": 45000, "SOL-USD": 8000,
		"ADA-USD": 15000, "DOT-USD": 5000, "AVAX-USD": 3000,
	}
	if vol, exists := volumes[symbol]; exists {
		return vol
	}
	return 2000 // Default volume
}

func (m *MockDataSource) getBaseOI(symbol string) float64 {
	// Open Interest in USD
	ois := map[string]float64{
		"BTC-USD": 15000000000, "ETH-USD": 8000000000, "SOL-USD": 1500000000,
		"ADA-USD": 500000000, "DOT-USD": 300000000, "AVAX-USD": 200000000,
	}
	if oi, exists := ois[symbol]; exists {
		return oi
	}
	return 100000000 // Default 100M USD
}

func (m *MockDataSource) getBaseReserves(symbol string) float64 {
	// Exchange reserves in native units
	reserves := map[string]float64{
		"BTC-USD": 2500000,  // ~2.5M BTC on exchanges
		"ETH-USD": 25000000, // ~25M ETH on exchanges
	}
	if res, exists := reserves[symbol]; exists {
		return res
	}
	return 0 // No reserves data for other assets
}

func (m *MockDataSource) getVenue(symbol string) string {
	venues := []string{"binance", "kraken", "coinbase"}
	// Deterministic venue selection based on symbol
	index := int(symbol[0]) % len(venues)
	return venues[index]
}

func (m *MockDataSource) getTimingMultiplier(weeksFromNow float64) float64 {
	absWeeks := math.Abs(weeksFromNow)
	switch {
	case absWeeks <= 4:
		return 1.2 // 0-4 weeks: 1.2x
	case absWeeks <= 8:
		return 1.0 // 4-8 weeks: 1.0x
	case absWeeks <= 12:
		return 0.8 // 8-12 weeks: 0.8x
	default:
		return 0.6 // 12+ weeks: 0.6x
	}
}

func (m *MockDataSource) median(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}

	// Simple median for 3 values
	if len(values) == 3 {
		if values[0] <= values[1] && values[0] <= values[2] {
			if values[1] <= values[2] {
				return values[1] // values[0] <= values[1] <= values[2]
			}
			return values[2] // values[0] <= values[2] < values[1]
		} else if values[1] <= values[0] && values[1] <= values[2] {
			if values[0] <= values[2] {
				return values[0] // values[1] <= values[0] <= values[2]
			}
			return values[2] // values[1] <= values[2] < values[0]
		} else {
			if values[0] <= values[1] {
				return values[0] // values[2] <= values[0] <= values[1]
			}
			return values[1] // values[2] <= values[1] < values[0]
		}
	}

	return values[0] // Fallback
}

func (m *MockDataSource) stdDev(values []float64) float64 {
	if len(values) <= 1 {
		return 0
	}

	// Calculate mean
	sum := 0.0
	for _, v := range values {
		sum += v
	}
	mean := sum / float64(len(values))

	// Calculate variance
	variance := 0.0
	for _, v := range values {
		variance += math.Pow(v-mean, 2)
	}
	variance /= float64(len(values) - 1)

	return math.Sqrt(variance)
}
