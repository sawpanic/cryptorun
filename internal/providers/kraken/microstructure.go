package kraken

import (
	"context"
	"fmt"
	"math"
	"time"
	
	"github.com/sawpanic/cryptorun/internal/data/facade"
	"github.com/sawpanic/cryptorun/internal/metrics"
	"github.com/sawpanic/cryptorun/internal/providers"
)

// MicrostructureData represents exchange-native L1/L2 order book analysis
type MicrostructureData struct {
	Pair           string    `json:"pair"`
	Timestamp      time.Time `json:"timestamp"`
	Venue          string    `json:"venue"`
	
	// L1 Data (Best Bid/Offer)
	BestBidPrice   float64   `json:"best_bid_price"`
	BestAskPrice   float64   `json:"best_ask_price"`
	BestBidVolume  float64   `json:"best_bid_volume"`
	BestAskVolume  float64   `json:"best_ask_volume"`
	
	// Calculated Metrics
	MidPrice       float64   `json:"mid_price"`
	SpreadBps      float64   `json:"spread_bps"`
	SpreadPercent  float64   `json:"spread_percent"`
	
	// L2 Depth Analysis (±2% around mid)
	BidDepthUSD2Pct float64  `json:"bid_depth_usd_2pct"`
	AskDepthUSD2Pct float64  `json:"ask_depth_usd_2pct"`
	TotalDepthUSD2Pct float64 `json:"total_depth_usd_2pct"`
	
	// VADR (Volume-Adjusted Daily Range) - requires historical data
	VADR           float64   `json:"vadr,omitempty"`
	
	// Health Signals
	Staleness      time.Duration `json:"staleness"`
	SequenceGap    bool      `json:"sequence_gap,omitempty"`
	DataQuality    float64   `json:"data_quality"` // 0.0-1.0
}

// MicrostructureExtractor handles L1/L2 analysis for exchange-native data
type MicrostructureExtractor struct {
	client         *Client
	lastSequenceID int64
	healthWindow   []MicrostructureData
	maxHealthWindow int
	vadrCalculator *metrics.VADRCalculator
	dataFacade     facade.DataFacade  // For historical data access
}

// NewMicrostructureExtractor creates a new microstructure analyzer
func NewMicrostructureExtractor(client *Client) *MicrostructureExtractor {
	return &MicrostructureExtractor{
		client:          client,
		healthWindow:    make([]MicrostructureData, 0),
		maxHealthWindow: 100, // Keep last 100 samples for health analysis
		vadrCalculator:  metrics.NewVADRCalculator(),
	}
}

// NewMicrostructureExtractorWithFacade creates a new extractor with data facade for VADR
func NewMicrostructureExtractorWithFacade(client *Client, dataFacade facade.DataFacade) *MicrostructureExtractor {
	return &MicrostructureExtractor{
		client:          client,
		healthWindow:    make([]MicrostructureData, 0),
		maxHealthWindow: 100,
		vadrCalculator:  metrics.NewVADRCalculator(),
		dataFacade:      dataFacade,
	}
}

// ExtractMicrostructure performs comprehensive L1/L2 analysis for a USD pair
func (me *MicrostructureExtractor) ExtractMicrostructure(ctx context.Context, pair string) (*MicrostructureData, error) {
	if !providers.IsUSDPair(pair) {
		return nil, fmt.Errorf("non-USD pair rejected: %s - exchange-native USD pairs only", pair)
	}
	
	start := time.Now()
	
	// Get L2 order book data (exchange-native)
	orderBook, err := me.client.GetOrderBook(ctx, pair, 50) // Top 50 levels
	if err != nil {
		return nil, fmt.Errorf("failed to get order book: %w", err)
	}
	
	// Get ticker for additional validation
	tickers, err := me.client.GetTicker(ctx, []string{pair})
	if err != nil {
		return nil, fmt.Errorf("failed to get ticker: %w", err)
	}
	
	ticker, exists := tickers[normalizePairName(pair)]
	if !exists {
		return nil, fmt.Errorf("ticker not found for pair: %s", pair)
	}
	
	// Extract L1 data
	bestBid, err := orderBook.Data.GetBestBid()
	if err != nil {
		return nil, fmt.Errorf("failed to get best bid: %w", err)
	}
	
	bestAsk, err := orderBook.Data.GetBestAsk()
	if err != nil {
		return nil, fmt.Errorf("failed to get best ask: %w", err)
	}
	
	// Calculate basic metrics
	midPrice := (bestBid.Price + bestAsk.Price) / 2.0
	spread := bestAsk.Price - bestBid.Price
	spreadBps := (spread / midPrice) * 10000
	spreadPercent := (spread / midPrice) * 100
	
	// Validate spread sanity (exchange-native requirement)
	if spreadBps > 1000 { // > 10% spread seems suspicious
		return nil, fmt.Errorf("suspicious spread detected: %.2f bps", spreadBps)
	}
	
	// Calculate L2 depth within ±2% (microstructure requirement)
	bidDepth, askDepth, err := orderBook.Data.CalculateDepthUSD(midPrice, 2.0)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate depth: %w", err)
	}
	
	totalDepth := bidDepth + askDepth
	
	// Create microstructure data
	microData := &MicrostructureData{
		Pair:              normalizePairName(pair),
		Timestamp:         time.Now(),
		Venue:             "kraken", // Exchange-native source
		BestBidPrice:      bestBid.Price,
		BestAskPrice:      bestAsk.Price,
		BestBidVolume:     bestBid.Volume,
		BestAskVolume:     bestAsk.Volume,
		MidPrice:          midPrice,
		SpreadBps:         spreadBps,
		SpreadPercent:     spreadPercent,
		BidDepthUSD2Pct:   bidDepth,
		AskDepthUSD2Pct:   askDepth,
		TotalDepthUSD2Pct: totalDepth,
		Staleness:         time.Since(start),
		DataQuality:       me.calculateDataQuality(ticker, bestBid, bestAsk),
	}
	
	// Calculate VADR if data facade is available
	if me.dataFacade != nil && me.vadrCalculator != nil {
		vadr, err := me.calculateVADR(ctx, pair)
		if err == nil && vadr > 0 {
			microData.VADR = vadr
		}
	}
	
	// Add to health window for monitoring
	me.addToHealthWindow(*microData)
	
	return microData, nil
}

// ValidateMicrostructureGates validates against CryptoRun v3.2.1 requirements
func (me *MicrostructureExtractor) ValidateMicrostructureGates(data *MicrostructureData) (*GateValidation, error) {
	if data == nil {
		return nil, fmt.Errorf("microstructure data is nil")
	}
	
	validation := &GateValidation{
		Timestamp: time.Now(),
		Pair:      data.Pair,
		Venue:     data.Venue,
	}
	
	// Gate 1: Spread < 50 bps (CryptoRun requirement)
	validation.SpreadGate.Pass = data.SpreadBps < 50.0
	validation.SpreadGate.Value = data.SpreadBps
	validation.SpreadGate.Threshold = 50.0
	validation.SpreadGate.Reason = fmt.Sprintf("Spread %.2f bps", data.SpreadBps)
	
	// Gate 2: Depth ≥ $100k within ±2% (CryptoRun requirement)
	validation.DepthGate.Pass = data.TotalDepthUSD2Pct >= 100000.0
	validation.DepthGate.Value = data.TotalDepthUSD2Pct
	validation.DepthGate.Threshold = 100000.0
	validation.DepthGate.Reason = fmt.Sprintf("Total depth $%.0f", data.TotalDepthUSD2Pct)
	
	// Gate 3: VADR ≥ 1.75 (if available)
	if data.VADR > 0 {
		validation.VADRGate.Pass = data.VADR >= 1.75
		validation.VADRGate.Value = data.VADR
		validation.VADRGate.Threshold = 1.75
		validation.VADRGate.Reason = fmt.Sprintf("VADR %.2f", data.VADR)
	} else {
		validation.VADRGate.Pass = false // Require VADR data
		validation.VADRGate.Reason = "VADR data not available"
	}
	
	// Gate 4: Data Quality ≥ 0.8 (80%)
	validation.QualityGate.Pass = data.DataQuality >= 0.8
	validation.QualityGate.Value = data.DataQuality
	validation.QualityGate.Threshold = 0.8
	validation.QualityGate.Reason = fmt.Sprintf("Data quality %.1f%%", data.DataQuality*100)
	
	// Gate 5: Staleness < 30 seconds
	stalenessSeconds := data.Staleness.Seconds()
	validation.StalenessGate.Pass = stalenessSeconds < 30.0
	validation.StalenessGate.Value = stalenessSeconds
	validation.StalenessGate.Threshold = 30.0
	validation.StalenessGate.Reason = fmt.Sprintf("Staleness %.1fs", stalenessSeconds)
	
	// Overall pass requires all gates to pass
	validation.OverallPass = validation.SpreadGate.Pass &&
							validation.DepthGate.Pass &&
							validation.VADRGate.Pass &&
							validation.QualityGate.Pass &&
							validation.StalenessGate.Pass
	
	return validation, nil
}

// GetHealthSignals returns current microstructure health metrics
func (me *MicrostructureExtractor) GetHealthSignals() *HealthSignals {
	if len(me.healthWindow) == 0 {
		return &HealthSignals{
			WindowSize: 0,
			AvgSpreadBps: 0,
			AvgDepthUSD: 0,
			DataQualityScore: 0,
		}
	}
	
	var totalSpread, totalDepth, totalQuality float64
	var staleSamples, gapSamples int
	
	for _, sample := range me.healthWindow {
		totalSpread += sample.SpreadBps
		totalDepth += sample.TotalDepthUSD2Pct
		totalQuality += sample.DataQuality
		
		if sample.Staleness > 10*time.Second {
			staleSamples++
		}
		if sample.SequenceGap {
			gapSamples++
		}
	}
	
	windowSize := len(me.healthWindow)
	return &HealthSignals{
		WindowSize:       windowSize,
		AvgSpreadBps:     totalSpread / float64(windowSize),
		AvgDepthUSD:      totalDepth / float64(windowSize),
		DataQualityScore: totalQuality / float64(windowSize),
		StalenessRate:    float64(staleSamples) / float64(windowSize),
		SequenceGapRate:  float64(gapSamples) / float64(windowSize),
		LastUpdate:       time.Now(),
	}
}

// Helper methods

func (me *MicrostructureExtractor) calculateDataQuality(ticker *TickerInfo, bestBid, bestAsk *OrderBookLevel) float64 {
	quality := 1.0
	
	// Check if ticker and order book prices are consistent
	tickerBid, err := ticker.GetBidPrice()
	if err != nil {
		quality *= 0.8
	} else if math.Abs(tickerBid-bestBid.Price)/bestBid.Price > 0.001 { // > 0.1% difference
		quality *= 0.9
	}
	
	tickerAsk, err := ticker.GetAskPrice()
	if err != nil {
		quality *= 0.8
	} else if math.Abs(tickerAsk-bestAsk.Price)/bestAsk.Price > 0.001 { // > 0.1% difference
		quality *= 0.9
	}
	
	// Check for reasonable spread
	spread := (bestAsk.Price - bestBid.Price) / bestBid.Price
	if spread < 0.0001 { // < 1 bps seems too tight
		quality *= 0.7
	} else if spread > 0.01 { // > 1% seems too wide for major pairs
		quality *= 0.8
	}
	
	// Check for reasonable volumes
	if bestBid.Volume <= 0 || bestAsk.Volume <= 0 {
		quality *= 0.5
	}
	
	return quality
}

func (me *MicrostructureExtractor) addToHealthWindow(data MicrostructureData) {
	me.healthWindow = append(me.healthWindow, data)
	
	// Maintain window size
	if len(me.healthWindow) > me.maxHealthWindow {
		me.healthWindow = me.healthWindow[1:]
	}
}

// Types for gate validation and health monitoring

type GateValidation struct {
	Timestamp     time.Time    `json:"timestamp"`
	Pair          string       `json:"pair"`
	Venue         string       `json:"venue"`
	OverallPass   bool         `json:"overall_pass"`
	SpreadGate    GateResult   `json:"spread_gate"`
	DepthGate     GateResult   `json:"depth_gate"`
	VADRGate      GateResult   `json:"vadr_gate"`
	QualityGate   GateResult   `json:"quality_gate"`
	StalenessGate GateResult   `json:"staleness_gate"`
}

type GateResult struct {
	Pass      bool    `json:"pass"`
	Value     float64 `json:"value"`
	Threshold float64 `json:"threshold"`
	Reason    string  `json:"reason"`
}

type HealthSignals struct {
	WindowSize       int       `json:"window_size"`
	AvgSpreadBps     float64   `json:"avg_spread_bps"`
	AvgDepthUSD      float64   `json:"avg_depth_usd"`
	DataQualityScore float64   `json:"data_quality_score"`
	StalenessRate    float64   `json:"staleness_rate"`
	SequenceGapRate  float64   `json:"sequence_gap_rate"`
	LastUpdate       time.Time `json:"last_update"`
}

// calculateVADR computes VADR for the given pair using historical data
func (me *MicrostructureExtractor) calculateVADR(ctx context.Context, pair string) (float64, error) {
	if me.dataFacade == nil || me.vadrCalculator == nil {
		return 0, fmt.Errorf("data facade or VADR calculator not available")
	}
	
	// Get 24h historical OHLCV data for VADR calculation
	endTime := time.Now()
	startTime := endTime.Add(-24 * time.Hour)
	
	// Fetch historical klines from data facade
	klines, err := me.dataFacade.GetKlines(ctx, pair, "1h", startTime, endTime)
	if err != nil {
		return 0, fmt.Errorf("failed to get historical klines for VADR: %w", err)
	}
	
	if len(klines) < 20 {
		return 0, fmt.Errorf("insufficient historical data for VADR: got %d bars, need 20+", len(klines))
	}
	
	// Convert to facade.Kline format (assuming GetKlines returns this format)
	
	// Calculate average daily volume for tier determination
	var totalVolume float64
	for _, kline := range klines {
		totalVolume += kline.VolumeUSD
	}
	avgVolume := totalVolume / float64(len(klines)) * 24 // Scale to daily
	
	// Calculate VADR with tier precedence
	vadr, frozen, err := me.vadrCalculator.CalculateWithPrecedence(klines, 1.75) // Default tier min
	if err != nil {
		return 0, fmt.Errorf("VADR calculation failed: %w", err)
	}
	
	if frozen {
		return 0, fmt.Errorf("VADR calculation frozen due to insufficient data")
	}
	
	// Validate VADR against tier requirements
	passes, tier, reason := metrics.ValidateVADR(vadr, frozen, avgVolume)
	if !passes {
		return 0, fmt.Errorf("VADR validation failed: %s (tier: %s)", reason, tier.Name)
	}
	
	return vadr, nil
}

// GetVADRMetrics returns comprehensive VADR analysis for the pair
func (me *MicrostructureExtractor) GetVADRMetrics(ctx context.Context, pair string) (*metrics.VADRMetrics, error) {
	if me.dataFacade == nil || me.vadrCalculator == nil {
		return nil, fmt.Errorf("data facade or VADR calculator not available")
	}
	
	// Get 24h historical data
	endTime := time.Now()
	startTime := endTime.Add(-24 * time.Hour)
	
	klines, err := me.dataFacade.GetKlines(ctx, pair, "1h", startTime, endTime)
	if err != nil {
		return nil, fmt.Errorf("failed to get historical klines: %w", err)
	}
	
	// Calculate average daily volume
	var totalVolume float64
	for _, kline := range klines {
		totalVolume += kline.VolumeUSD
	}
	avgVolume := totalVolume / float64(len(klines)) * 24
	
	// Get comprehensive VADR metrics
	vadrMetrics := me.vadrCalculator.GetVADRMetrics(klines, avgVolume, 24*time.Hour)
	return &vadrMetrics, nil
}