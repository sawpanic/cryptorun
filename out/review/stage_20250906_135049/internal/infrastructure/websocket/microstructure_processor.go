package websocket

import (
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/rs/zerolog/log"

	"cryptorun/internal/domain"
)

// MicrostructureProcessor calculates real-time microstructure metrics
type MicrostructureProcessor struct {
	symbols     map[string]*SymbolMicrostructure
	vadrMinBars int
	mu          sync.RWMutex
}

// SymbolMicrostructure holds microstructure data for a single symbol
type SymbolMicrostructure struct {
	Symbol        string                    `json:"symbol"`
	LastTick      *TickUpdate              `json:"last_tick"`
	Metrics       *domain.MicrostructureMetrics `json:"metrics"`
	PriceHistory  *RollingWindow           `json:"price_history"`
	VolumeHistory *RollingWindow           `json:"volume_history"`
	TickCount     int64                    `json:"tick_count"`
	LastUpdate    time.Time                `json:"last_update"`
	mu            sync.RWMutex
}

// RollingWindow maintains a rolling window of values for calculations
type RollingWindow struct {
	values []float64
	index  int
	size   int
	filled bool
	mu     sync.RWMutex
}

// NewMicrostructureProcessor creates a new microstructure processor
func NewMicrostructureProcessor(vadrMinBars int) *MicrostructureProcessor {
	return &MicrostructureProcessor{
		symbols:     make(map[string]*SymbolMicrostructure),
		vadrMinBars: vadrMinBars,
	}
}

// Initialize sets up tracking for the given symbols
func (mp *MicrostructureProcessor) Initialize(symbols []string) error {
	mp.mu.Lock()
	defer mp.mu.Unlock()
	
	for _, symbol := range symbols {
		mp.symbols[symbol] = &SymbolMicrostructure{
			Symbol: symbol,
			Metrics: &domain.MicrostructureMetrics{
				Symbol:           symbol,
				SpreadBps:        math.NaN(),
				DepthUSD2Pct:     math.NaN(),
				VADR:             math.NaN(),
				VenueHealth:      "unknown",
				LastUpdate:       time.Now(),
				IsExchangeNative: true, // WebSocket is always exchange-native
			},
			PriceHistory:  NewRollingWindow(mp.vadrMinBars * 2), // Extra buffer for VADR calculation
			VolumeHistory: NewRollingWindow(mp.vadrMinBars * 2),
		}
	}
	
	log.Info().Int("symbols", len(symbols)).Int("vadr_bars", mp.vadrMinBars).Msg("Initialized microstructure processor")
	
	return nil
}

// ProcessTick updates microstructure metrics with a new tick
func (mp *MicrostructureProcessor) ProcessTick(tick *TickUpdate) {
	mp.mu.RLock()
	symbolData, exists := mp.symbols[tick.Symbol]
	mp.mu.RUnlock()
	
	if !exists {
		return
	}
	
	symbolData.mu.Lock()
	defer symbolData.mu.Unlock()
	
	// Update last tick and counters
	symbolData.LastTick = tick
	symbolData.TickCount++
	symbolData.LastUpdate = time.Now()
	
	// Calculate spread in basis points
	if tick.Bid > 0 && tick.Ask > 0 && tick.Ask > tick.Bid {
		midPrice := (tick.Bid + tick.Ask) / 2
		spread := tick.Ask - tick.Bid
		symbolData.Metrics.SpreadBps = (spread / midPrice) * 10000
	} else {
		symbolData.Metrics.SpreadBps = math.NaN()
	}
	
	// Calculate depth at ±2% (estimate from bid/ask sizes)
	if tick.BidSize > 0 && tick.AskSize > 0 && tick.Bid > 0 && tick.Ask > 0 {
		// Estimate depth using bid/ask sizes and current prices
		bidDepth := tick.BidSize * tick.Bid
		askDepth := tick.AskSize * tick.Ask
		symbolData.Metrics.DepthUSD2Pct = math.Min(bidDepth, askDepth)
	} else {
		symbolData.Metrics.DepthUSD2Pct = math.NaN()
	}
	
	// Add price to history for VADR calculation
	if tick.LastPrice > 0 {
		symbolData.PriceHistory.Add(tick.LastPrice)
	}
	
	// Add volume to history
	if tick.Volume24h > 0 {
		symbolData.VolumeHistory.Add(tick.Volume24h)
	}
	
	// Calculate VADR if we have enough bars
	if symbolData.PriceHistory.IsFilled() && symbolData.PriceHistory.Count() >= mp.vadrMinBars {
		vadr := mp.calculateVADR(symbolData.PriceHistory, symbolData.VolumeHistory)
		symbolData.Metrics.VADR = vadr
	} else {
		symbolData.Metrics.VADR = math.NaN()
	}
	
	// Update venue health based on tick freshness and quality
	symbolData.Metrics.VenueHealth = mp.assessVenueHealth(tick, symbolData)
	
	// Update metadata
	symbolData.Metrics.LastUpdate = time.Now()
	symbolData.Metrics.TickCount = symbolData.TickCount
	
	// Validate metrics against gates
	mp.validateMicrostructureGates(symbolData.Metrics)
}

// GetMetrics returns current microstructure metrics for a symbol
func (mp *MicrostructureProcessor) GetMetrics(symbol string) (*domain.MicrostructureMetrics, error) {
	mp.mu.RLock()
	symbolData, exists := mp.symbols[symbol]
	mp.mu.RUnlock()
	
	if !exists {
		return nil, fmt.Errorf("symbol %s not tracked", symbol)
	}
	
	symbolData.mu.RLock()
	defer symbolData.mu.RUnlock()
	
	// Return a copy to avoid race conditions
	metrics := *symbolData.Metrics
	return &metrics, nil
}

// GetAllMetrics returns microstructure metrics for all symbols
func (mp *MicrostructureProcessor) GetAllMetrics() map[string]*domain.MicrostructureMetrics {
	mp.mu.RLock()
	defer mp.mu.RUnlock()
	
	result := make(map[string]*domain.MicrostructureMetrics)
	
	for symbol, symbolData := range mp.symbols {
		symbolData.mu.RLock()
		metrics := *symbolData.Metrics // Copy
		symbolData.mu.RUnlock()
		result[symbol] = &metrics
	}
	
	return result
}

// calculateVADR computes Volume-Adjusted Daily Range
func (mp *MicrostructureProcessor) calculateVADR(priceHistory, volumeHistory *RollingWindow) float64 {
	prices := priceHistory.GetValues()
	volumes := volumeHistory.GetValues()
	
	if len(prices) < mp.vadrMinBars || len(volumes) < mp.vadrMinBars {
		return math.NaN()
	}
	
	// Calculate price range over the window
	minPrice := prices[0]
	maxPrice := prices[0]
	totalVolume := 0.0
	
	for i, price := range prices {
		if price < minPrice {
			minPrice = price
		}
		if price > maxPrice {
			maxPrice = price
		}
		
		if i < len(volumes) {
			totalVolume += volumes[i]
		}
	}
	
	if minPrice <= 0 || totalVolume <= 0 {
		return math.NaN()
	}
	
	// Price range as percentage
	priceRange := (maxPrice - minPrice) / minPrice
	
	// Volume-adjusted: higher volume should increase VADR
	// Use log to prevent extreme values
	avgVolume := totalVolume / float64(len(volumes))
	volumeAdjustment := math.Log(1 + avgVolume/1000000) // Normalize by 1M USD
	
	vadr := priceRange * (1 + volumeAdjustment)
	
	return vadr
}

// assessVenueHealth determines venue health based on tick quality
func (mp *MicrostructureProcessor) assessVenueHealth(tick *TickUpdate, symbolData *SymbolMicrostructure) string {
	now := time.Now()
	
	// Check tick freshness
	tickAge := now.Sub(tick.Timestamp)
	if tickAge > 30*time.Second {
		return "stale"
	}
	
	// Check data quality
	if tick.Bid <= 0 || tick.Ask <= 0 || tick.Ask <= tick.Bid {
		return "degraded"
	}
	
	// Check if we're receiving regular updates
	if symbolData.LastUpdate.IsZero() {
		return "initializing"
	}
	
	timeSinceLastUpdate := now.Sub(symbolData.LastUpdate)
	if timeSinceLastUpdate > 10*time.Second {
		return "slow"
	}
	
	// Check processing latency
	if tick.ProcessingLatency > 100*time.Millisecond {
		return "high_latency"
	}
	
	return "healthy"
}

// validateMicrostructureGates checks if metrics meet gate requirements
func (mp *MicrostructureProcessor) validateMicrostructureGates(metrics *domain.MicrostructureMetrics) {
	// Spread gate: < 50 bps
	if !math.IsNaN(metrics.SpreadBps) {
		metrics.SpreadOK = metrics.SpreadBps < 50.0
	} else {
		metrics.SpreadOK = false
	}
	
	// Depth gate: >= $100k within ±2%
	if !math.IsNaN(metrics.DepthUSD2Pct) {
		metrics.DepthOK = metrics.DepthUSD2Pct >= 100000.0
	} else {
		metrics.DepthOK = false
	}
	
	// VADR gate: >= 1.75x
	if !math.IsNaN(metrics.VADR) {
		metrics.VADROK = metrics.VADR >= 1.75
	} else {
		metrics.VADROK = false
	}
	
	// Venue health gate: must be healthy
	metrics.VenueHealthOK = metrics.VenueHealth == "healthy"
	
	// Overall microstructure OK
	metrics.MicrostructureOK = metrics.SpreadOK && metrics.DepthOK && 
							   metrics.VADROK && metrics.VenueHealthOK && 
							   metrics.IsExchangeNative
}

// NewRollingWindow creates a new rolling window
func NewRollingWindow(size int) *RollingWindow {
	return &RollingWindow{
		values: make([]float64, size),
		size:   size,
	}
}

// Add adds a value to the rolling window
func (rw *RollingWindow) Add(value float64) {
	rw.mu.Lock()
	defer rw.mu.Unlock()
	
	rw.values[rw.index] = value
	rw.index = (rw.index + 1) % rw.size
	
	if !rw.filled && rw.index == 0 {
		rw.filled = true
	}
}

// GetValues returns a copy of current values
func (rw *RollingWindow) GetValues() []float64 {
	rw.mu.RLock()
	defer rw.mu.RUnlock()
	
	if !rw.filled {
		// Return only the filled portion
		result := make([]float64, rw.index)
		copy(result, rw.values[:rw.index])
		return result
	}
	
	// Return values in chronological order
	result := make([]float64, rw.size)
	copy(result[:rw.size-rw.index], rw.values[rw.index:])
	copy(result[rw.size-rw.index:], rw.values[:rw.index])
	return result
}

// IsFilled returns true if the window is completely filled
func (rw *RollingWindow) IsFilled() bool {
	rw.mu.RLock()
	defer rw.mu.RUnlock()
	return rw.filled
}

// Count returns the number of values currently in the window
func (rw *RollingWindow) Count() int {
	rw.mu.RLock()
	defer rw.mu.RUnlock()
	
	if rw.filled {
		return rw.size
	}
	return rw.index
}