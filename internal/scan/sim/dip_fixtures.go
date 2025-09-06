package sim

import (
	"context"
	"fmt"
	"math"
	"time"

	"cryptorun/internal/algo/dip"
	"cryptorun/internal/domain"
)

// FixtureDataProvider provides deterministic test data for dip pipeline testing
type FixtureDataProvider struct {
	marketData       map[string]map[string][]dip.MarketData
	microstructure   map[string]*domain.MicroGateInputs
	socialData       map[string]*dip.SocialData
	failureSimulator *FailureSimulator
}

// FailureSimulator controls test failure scenarios
type FailureSimulator struct {
	MarketDataError     bool
	MicrostructureError bool
	SocialDataError     bool
	ErrorMessage        string
}

// NewFixtureDataProvider creates a new fixture data provider
func NewFixtureDataProvider() *FixtureDataProvider {
	return &FixtureDataProvider{
		marketData:       make(map[string]map[string][]dip.MarketData),
		microstructure:   make(map[string]*domain.MicroGateInputs),
		socialData:       make(map[string]*dip.SocialData),
		failureSimulator: &FailureSimulator{},
	}
}

// SetMarketData sets deterministic OHLCV data for testing
func (fdp *FixtureDataProvider) SetMarketData(symbol, timeframe string, data []dip.MarketData) {
	if fdp.marketData[symbol] == nil {
		fdp.marketData[symbol] = make(map[string][]dip.MarketData)
	}
	fdp.marketData[symbol][timeframe] = data
}

// SetMicrostructureData sets L1/L2 order book data for testing
func (fdp *FixtureDataProvider) SetMicrostructureData(symbol string, data *domain.MicroGateInputs) {
	fdp.microstructure[symbol] = data
}

// SetSocialData sets social/brand data for testing
func (fdp *FixtureDataProvider) SetSocialData(symbol string, data *dip.SocialData) {
	fdp.socialData[symbol] = data
}

// SetFailureSimulator configures error simulation
func (fdp *FixtureDataProvider) SetFailureSimulator(simulator *FailureSimulator) {
	fdp.failureSimulator = simulator
}

// GetAllMarketData returns all market data for testing access
func (fdp *FixtureDataProvider) GetAllMarketData(symbol string) map[string][]dip.MarketData {
	if data, exists := fdp.marketData[symbol]; exists {
		return data
	}
	return make(map[string][]dip.MarketData)
}

// GetStoredMicrostructureData returns stored microstructure data
func (fdp *FixtureDataProvider) GetStoredMicrostructureData(symbol string) *domain.MicroGateInputs {
	return fdp.microstructure[symbol]
}

// GetStoredSocialData returns stored social data
func (fdp *FixtureDataProvider) GetStoredSocialData(symbol string) *dip.SocialData {
	return fdp.socialData[symbol]
}

// GetMarketData implements DataProvider interface
func (fdp *FixtureDataProvider) GetMarketData(ctx context.Context, symbol string, timeframe string, periods int) ([]dip.MarketData, error) {
	if fdp.failureSimulator.MarketDataError {
		return nil, fmt.Errorf("simulated market data error: %s", fdp.failureSimulator.ErrorMessage)
	}

	if symbolData, exists := fdp.marketData[symbol]; exists {
		if tfData, exists := symbolData[timeframe]; exists {
			// Return requested number of periods, or all data if less available
			if len(tfData) <= periods {
				return tfData, nil
			}
			return tfData[len(tfData)-periods:], nil
		}
	}

	// Generate default data if not explicitly set
	return fdp.generateDefaultMarketData(symbol, timeframe, periods), nil
}

// GetMicrostructureData implements DataProvider interface
func (fdp *FixtureDataProvider) GetMicrostructureData(ctx context.Context, symbol string) (*domain.MicroGateInputs, error) {
	if fdp.failureSimulator.MicrostructureError {
		return nil, fmt.Errorf("simulated microstructure error: %s", fdp.failureSimulator.ErrorMessage)
	}

	if data, exists := fdp.microstructure[symbol]; exists {
		return data, nil
	}

	// Generate default microstructure data
	return fdp.generateDefaultMicrostructure(symbol), nil
}

// GetSocialData implements DataProvider interface
func (fdp *FixtureDataProvider) GetSocialData(ctx context.Context, symbol string) (*dip.SocialData, error) {
	if fdp.failureSimulator.SocialDataError {
		return nil, fmt.Errorf("simulated social data error: %s", fdp.failureSimulator.ErrorMessage)
	}

	if data, exists := fdp.socialData[symbol]; exists {
		return data, nil
	}

	// Social data is optional - return nil without error
	return nil, nil
}

// generateDefaultMarketData creates realistic OHLCV data for testing
func (fdp *FixtureDataProvider) generateDefaultMarketData(symbol, timeframe string, periods int) []dip.MarketData {
	data := make([]dip.MarketData, periods)

	// Base parameters based on symbol
	basePrice := 100.0
	if symbol == "BTCUSD" {
		basePrice = 45000.0
	} else if symbol == "ETHUSD" {
		basePrice = 3000.0
	}

	baseVolume := 1000000.0
	volatility := 0.02 // 2% typical move

	// Time interval based on timeframe
	var interval time.Duration
	switch timeframe {
	case "1h":
		interval = time.Hour
	case "4h":
		interval = 4 * time.Hour
	case "12h":
		interval = 12 * time.Hour
	case "24h":
		interval = 24 * time.Hour
	default:
		interval = time.Hour
	}

	startTime := time.Now().Add(-time.Duration(periods) * interval)
	currentPrice := basePrice

	for i := 0; i < periods; i++ {
		// Generate realistic OHLC with trend and noise
		trendFactor := 1.0 + (float64(i)/float64(periods))*0.1 // 10% uptrend over period
		noise := (math.Sin(float64(i)*0.3) + math.Cos(float64(i)*0.7)) * volatility

		targetPrice := basePrice * trendFactor * (1 + noise)

		// Smooth price movement
		priceChange := (targetPrice - currentPrice) * 0.3 // 30% of way to target
		currentPrice += priceChange

		// Generate OHLC
		open := currentPrice
		close := currentPrice * (1 + (math.Sin(float64(i)*0.5) * volatility * 0.5))

		high := math.Max(open, close) * (1 + math.Abs(math.Sin(float64(i)*0.2))*volatility*0.3)
		low := math.Min(open, close) * (1 - math.Abs(math.Cos(float64(i)*0.4))*volatility*0.3)

		// Volume with some variation
		volume := baseVolume * (1 + math.Sin(float64(i)*0.1)*0.3)

		data[i] = dip.MarketData{
			Timestamp: startTime.Add(time.Duration(i) * interval),
			Open:      open,
			High:      high,
			Low:       low,
			Close:     close,
			Volume:    volume,
		}

		currentPrice = close
	}

	return data
}

// generateDefaultMicrostructure creates realistic L1/L2 data for testing
func (fdp *FixtureDataProvider) generateDefaultMicrostructure(symbol string) *domain.MicroGateInputs {
	// Base parameters by symbol
	basePrice := 100.0
	if symbol == "BTCUSD" {
		basePrice = 45000.0
	} else if symbol == "ETHUSD" {
		basePrice = 3000.0
	}

	// Tight spread for good liquidity
	spreadBps := 25.0 // 25 basis points
	spreadAmount := basePrice * (spreadBps / 10000.0)

	bid := basePrice - spreadAmount/2
	ask := basePrice + spreadAmount/2

	return &domain.MicroGateInputs{
		Symbol:      symbol,
		Bid:         bid,
		Ask:         ask,
		Depth2PcUSD: 150000.0, // $150k depth
		VADR:        2.1,      // Good VADR
		ADVUSD:      5000000,  // $5M ADV
	}
}

// DipScenario represents a test scenario for dip detection
type DipScenario struct {
	Name             string
	Symbol           string
	ShouldQualify    bool
	ShouldDetectDip  bool
	ShouldPassGuards bool
	ExpectedScore    float64
	Description      string
}

// CreateUptrendScenario creates data showing strong uptrend with quality dip
func CreateUptrendScenario(symbol string) *FixtureDataProvider {
	fdp := NewFixtureDataProvider()

	// Create strong uptrend data for 12h timeframe (MA qualification)
	data12h := make([]dip.MarketData, 60) // 60 periods
	basePrice := 100.0
	startTime := time.Now().Add(-60 * 12 * time.Hour)

	for i := 0; i < 60; i++ {
		// Strong uptrend with 30% gain over period
		price := basePrice * (1 + float64(i)*0.005) // 0.5% per period

		data12h[i] = dip.MarketData{
			Timestamp: startTime.Add(time.Duration(i) * 12 * time.Hour),
			Open:      price * 0.99,
			High:      price * 1.02,
			Low:       price * 0.98,
			Close:     price,
			Volume:    1000000,
		}
	}
	fdp.SetMarketData(symbol, "12h", data12h)

	// Similar for 24h - need at least 50 bars for MA calculation
	data24h := make([]dip.MarketData, 60)
	for i := 0; i < 60; i++ {
		price := basePrice * (1 + float64(i)*0.01) // 1% per period

		data24h[i] = dip.MarketData{
			Timestamp: startTime.Add(time.Duration(i) * 24 * time.Hour),
			Open:      price * 0.99,
			High:      price * 1.03,
			Low:       price * 0.97,
			Close:     price,
			Volume:    1000000,
		}
	}
	fdp.SetMarketData(symbol, "24h", data24h)

	// 4h data for ADX calculation - need at least 28 bars for 14-period ADX
	data4h := make([]dip.MarketData, 60)
	for i := 0; i < 60; i++ {
		price := basePrice * (1 + float64(i)*0.002) // 0.2% per period

		data4h[i] = dip.MarketData{
			Timestamp: startTime.Add(time.Duration(i) * 4 * time.Hour),
			Open:      price * 0.995,
			High:      price * 1.01,
			Low:       price * 0.99,
			Close:     price,
			Volume:    1000000,
		}
	}
	fdp.SetMarketData(symbol, "4h", data4h)

	// 1h data with clear dip pattern
	data1h := make([]dip.MarketData, 100)

	// Create uptrend with clear prior swing low at the beginning
	priorSwingLow := basePrice * 0.95 // 5% below base price
	currentPrice := basePrice * 1.25  // End of uptrend

	for i := 0; i < 90; i++ {
		// Uptrend portion - start from prior swing low
		progress := float64(i) / 89.0
		price := priorSwingLow + (currentPrice-priorSwingLow)*progress

		data1h[i] = dip.MarketData{
			Timestamp: startTime.Add(time.Duration(i) * time.Hour),
			Open:      price * 0.998,
			High:      price * 1.005,
			Low:       price * 0.995,
			Close:     price,
			Volume:    1000000,
		}
	}

	// Create dip pattern in last 10 bars
	dipStartPrice := currentPrice
	for i := 90; i < 100; i++ {
		var price float64
		if i < 95 {
			// 5 red bars down - moderate dip for good Fib levels
			dropFactor := 1.0 - float64(i-89)*0.025 // 2.5% per bar for moderate dip
			price = dipStartPrice * dropFactor

			data1h[i] = dip.MarketData{
				Timestamp: startTime.Add(time.Duration(i) * time.Hour),
				Open:      price * 1.04, // Higher open for red bar
				High:      price * 1.045,
				Low:       price * 0.96, // Lower low
				Close:     price,        // Lower close = red bar
				Volume:    1500000,      // Higher volume
			}
		} else if i == 95 {
			// Reversal bar - bullish engulfing potential at moderate dip
			price = dipStartPrice * 0.875 // 12.5% total dip

			data1h[i] = dip.MarketData{
				Timestamp: startTime.Add(time.Duration(i) * time.Hour),
				Open:      price * 0.99,
				High:      price * 1.04, // Strong recovery
				Low:       price * 0.98,
				Close:     price * 1.02, // Green bar
				Volume:    2000000,      // High volume
			}
		} else {
			// Recovery from moderate dip
			price = dipStartPrice * (0.88 + float64(i-95)*0.015)

			data1h[i] = dip.MarketData{
				Timestamp: startTime.Add(time.Duration(i) * time.Hour),
				Open:      price * 0.99,
				High:      price * 1.02,
				Low:       price * 0.98,
				Close:     price,
				Volume:    1200000,
			}
		}
	}
	fdp.SetMarketData(symbol, "1h", data1h)

	// Good microstructure
	fdp.SetMicrostructureData(symbol, &domain.MicroGateInputs{
		Symbol:      symbol,
		Bid:         currentPrice * 0.9998,
		Ask:         currentPrice * 1.0002, // 4 bps spread
		Depth2PcUSD: 200000.0,              // Good depth
		VADR:        2.2,                   // Strong VADR
		ADVUSD:      8000000,               // High ADV
	})

	// Positive social data
	fdp.SetSocialData(symbol, &dip.SocialData{
		SentimentScore:   0.6, // Positive
		VolumeMultiplier: 1.4, // Above average
		BrandRecognition: 0.8, // Well known
		TrustScore:       0.9, // Trusted
		LastUpdated:      time.Now().Add(-5 * time.Minute),
	})

	return fdp
}

// CreateChoppyMarketScenario creates sideways market that should be rejected
func CreateChoppyMarketScenario(symbol string) *FixtureDataProvider {
	fdp := NewFixtureDataProvider()

	basePrice := 100.0
	startTime := time.Now().Add(-60 * 12 * time.Hour)

	// Choppy sideways market for all timeframes
	for _, tf := range []struct {
		name     string
		periods  int
		interval time.Duration
	}{
		{"1h", 100, time.Hour},
		{"4h", 50, 4 * time.Hour},
		{"12h", 60, 12 * time.Hour},
		{"24h", 30, 24 * time.Hour},
	} {
		data := make([]dip.MarketData, tf.periods)

		for i := 0; i < tf.periods; i++ {
			// Sideways with noise - no trend
			noise := math.Sin(float64(i)*0.4) * 0.05 // 5% noise
			price := basePrice * (1 + noise)

			data[i] = dip.MarketData{
				Timestamp: startTime.Add(time.Duration(i) * tf.interval),
				Open:      price * 0.998,
				High:      price * 1.02,
				Low:       price * 0.98,
				Close:     price,
				Volume:    1000000,
			}
		}

		fdp.SetMarketData(symbol, tf.name, data)
	}

	// Poor microstructure
	fdp.SetMicrostructureData(symbol, &domain.MicroGateInputs{
		Symbol:      symbol,
		Bid:         basePrice * 0.997,
		Ask:         basePrice * 1.003, // 60 bps spread (too wide)
		Depth2PcUSD: 50000.0,           // Low depth
		VADR:        1.2,               // Poor VADR
		ADVUSD:      500000,            // Low ADV
	})

	return fdp
}

// CreateNewsShockScenario creates scenario with severe drop (should be vetoed)
func CreateNewsShockScenario(symbol string) *FixtureDataProvider {
	fdp := NewFixtureDataProvider()

	// Start with good uptrend
	uptrend := CreateUptrendScenario(symbol)

	// Override 1h data with shock pattern
	data1h := make([]dip.MarketData, 100)
	basePrice := 100.0
	startTime := time.Now().Add(-100 * time.Hour)

	// Normal uptrend for first 75 bars
	for i := 0; i < 75; i++ {
		price := basePrice * (1 + float64(i)*0.002)

		data1h[i] = dip.MarketData{
			Timestamp: startTime.Add(time.Duration(i) * time.Hour),
			Open:      price * 0.999,
			High:      price * 1.01,
			Low:       price * 0.995,
			Close:     price,
			Volume:    1000000,
		}
	}

	// Severe shock drop in next bars
	shockPrice := basePrice * 1.15
	for i := 75; i < 85; i++ {
		// 20% drop in 10 hours
		dropFactor := 1.0 - (float64(i-74) * 0.02)
		price := shockPrice * dropFactor

		data1h[i] = dip.MarketData{
			Timestamp: startTime.Add(time.Duration(i) * time.Hour),
			Open:      price * 1.02,
			High:      price * 1.03,
			Low:       price * 0.95, // Large range
			Close:     price,        // Down close
			Volume:    5000000,      // Panic volume
		}
	}

	// Weak recovery (no acceleration rebound)
	for i := 85; i < 100; i++ {
		price := shockPrice * 0.82 * (1 + float64(i-85)*0.005) // Slow recovery

		data1h[i] = dip.MarketData{
			Timestamp: startTime.Add(time.Duration(i) * time.Hour),
			Open:      price * 0.999,
			High:      price * 1.01,
			Low:       price * 0.995,
			Close:     price,
			Volume:    2000000,
		}
	}

	fdp.SetMarketData(symbol, "1h", data1h)

	// Copy other timeframe data from uptrend scenario
	for _, tf := range []string{"4h", "12h", "24h"} {
		if data, exists := uptrend.marketData[symbol][tf]; exists {
			fdp.SetMarketData(symbol, tf, data)
		}
	}

	// Good microstructure (but should still be vetoed by guards)
	if micro, exists := uptrend.microstructure[symbol]; exists {
		fdp.SetMicrostructureData(symbol, micro)
	}

	return fdp
}

// GetKnownScenarios returns predefined test scenarios
func GetKnownScenarios() []*DipScenario {
	return []*DipScenario{
		{
			Name:             "Strong Uptrend with Quality Dip",
			Symbol:           "BTCUSD",
			ShouldQualify:    true,
			ShouldDetectDip:  true,
			ShouldPassGuards: true,
			ExpectedScore:    65.0,
			Description:      "Clear uptrend, RSI divergence, good liquidity, no guards triggered",
		},
		{
			Name:             "Choppy Market",
			Symbol:           "ETHUSD",
			ShouldQualify:    false,
			ShouldDetectDip:  false,
			ShouldPassGuards: false,
			ExpectedScore:    0.0,
			Description:      "Sideways market, poor trend qualification, should be rejected early",
		},
		{
			Name:             "News Shock Pattern",
			Symbol:           "BTCUSD",
			ShouldQualify:    true,
			ShouldDetectDip:  true,
			ShouldPassGuards: false,
			ExpectedScore:    0.0,
			Description:      "Good technical setup but severe drop without rebound, news shock guard should veto",
		},
	}
}
