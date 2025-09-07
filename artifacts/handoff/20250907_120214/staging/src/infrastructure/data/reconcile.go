// Package data provides data reconciliation with trimmed median and outlier detection
package data

import (
	"fmt"
	"math"
	"sort"
	"time"
)

// Reconciler handles data reconciliation across multiple sources
type Reconciler interface {
	// ReconcileBars reconciles OHLCV bars from multiple sources
	ReconcileBars(sources map[string][]Bar) ([]Bar, error)

	// ReconcilePrices reconciles single price points
	ReconcilePrices(sources map[string]float64, symbol string) (ReconciledPrice, error)

	// GetConfig returns current reconciliation configuration
	GetConfig() ReconciliationConfig
}

// ReconciledPrice represents a reconciled price with attribution
type ReconciledPrice struct {
	Symbol         string             `json:"symbol"`
	Price          float64            `json:"price"`
	Timestamp      time.Time          `json:"timestamp"`
	Method         string             `json:"method"`          // "trimmed_median", "mean", etc.
	SourceCount    int                `json:"source_count"`    // Number of sources used
	DroppedSources []string           `json:"dropped_sources"` // Sources excluded as outliers
	SourcePrices   map[string]float64 `json:"source_prices"`   // All source prices for transparency
	Confidence     float64            `json:"confidence"`      // 0-1 confidence score
	Deviation      float64            `json:"deviation"`       // Standard deviation
	Attribution    string             `json:"attribution"`     // Source attribution
}

// ReconciliationConfig holds reconciliation parameters
type ReconciliationConfig struct {
	MaxDeviation        float64 `json:"max_deviation"`        // 1% = 0.01
	MinSources          int     `json:"min_sources"`          // Minimum sources required
	UseTrimmedMean      bool    `json:"use_trimmed_mean"`     // Use trimmed mean instead of median
	TrimPercent         float64 `json:"trim_percent"`         // Percentage to trim (0.1 = 10%)
	ConfidenceThreshold float64 `json:"confidence_threshold"` // Minimum confidence to accept
}

// ReconcilerImpl implements the Reconciler interface
type ReconcilerImpl struct {
	config ReconciliationConfig
}

// NewReconciler creates a new data reconciler
func NewReconciler(config ReconciliationConfig) *ReconcilerImpl {
	// Set defaults if not provided
	if config.MaxDeviation == 0 {
		config.MaxDeviation = 0.01 // 1%
	}
	if config.MinSources == 0 {
		config.MinSources = 2
	}
	if config.TrimPercent == 0 {
		config.TrimPercent = 0.1 // 10%
	}
	if config.ConfidenceThreshold == 0 {
		config.ConfidenceThreshold = 0.7 // 70%
	}

	return &ReconcilerImpl{
		config: config,
	}
}

// ReconcileBars reconciles OHLCV data from multiple sources
func (r *ReconcilerImpl) ReconcileBars(sources map[string][]Bar) ([]Bar, error) {
	if len(sources) < r.config.MinSources {
		return nil, fmt.Errorf("insufficient sources: got %d, need %d", len(sources), r.config.MinSources)
	}

	// Find common time periods across all sources
	timeIndex := r.buildTimeIndex(sources)
	if len(timeIndex) == 0 {
		return nil, fmt.Errorf("no common time periods found across sources")
	}

	var reconciledBars []Bar

	// Reconcile each time period
	for _, timestamp := range timeIndex {
		bar, err := r.reconcileSingleBar(sources, timestamp)
		if err != nil {
			continue // Skip periods that can't be reconciled
		}
		reconciledBars = append(reconciledBars, bar)
	}

	if len(reconciledBars) == 0 {
		return nil, fmt.Errorf("no bars could be reconciled")
	}

	return reconciledBars, nil
}

// buildTimeIndex finds common timestamps across all sources
func (r *ReconcilerImpl) buildTimeIndex(sources map[string][]Bar) []time.Time {
	// Count occurrences of each timestamp
	timeCount := make(map[time.Time]int)

	for _, bars := range sources {
		for _, bar := range bars {
			timeCount[bar.Timestamp]++
		}
	}

	// Keep timestamps that appear in at least MinSources
	var times []time.Time
	for timestamp, count := range timeCount {
		if count >= r.config.MinSources {
			times = append(times, timestamp)
		}
	}

	// Sort chronologically
	sort.Slice(times, func(i, j int) bool {
		return times[i].Before(times[j])
	})

	return times
}

// reconcileSingleBar reconciles OHLCV data for a single timestamp
func (r *ReconcilerImpl) reconcileSingleBar(sources map[string][]Bar, timestamp time.Time) (Bar, error) {
	// Collect bars for this timestamp from each source
	bars := make(map[string]Bar)

	for source, sourceBars := range sources {
		for _, bar := range sourceBars {
			if bar.Timestamp.Equal(timestamp) {
				bars[source] = bar
				break
			}
		}
	}

	if len(bars) < r.config.MinSources {
		return Bar{}, fmt.Errorf("insufficient data for timestamp %v", timestamp)
	}

	// Extract price data for reconciliation
	opens := make(map[string]float64)
	highs := make(map[string]float64)
	lows := make(map[string]float64)
	closes := make(map[string]float64)
	volumes := make(map[string]float64)

	var symbol string
	for source, bar := range bars {
		symbol = bar.Symbol // Assume all bars are for the same symbol
		opens[source] = bar.Open
		highs[source] = bar.High
		lows[source] = bar.Low
		closes[source] = bar.Close
		volumes[source] = bar.Volume
	}

	// Reconcile each OHLCV component
	reconciledOpen, err := r.ReconcilePrices(opens, symbol)
	if err != nil {
		return Bar{}, fmt.Errorf("failed to reconcile open prices: %w", err)
	}

	reconciledHigh, err := r.ReconcilePrices(highs, symbol)
	if err != nil {
		return Bar{}, fmt.Errorf("failed to reconcile high prices: %w", err)
	}

	reconciledLow, err := r.ReconcilePrices(lows, symbol)
	if err != nil {
		return Bar{}, fmt.Errorf("failed to reconcile low prices: %w", err)
	}

	reconciledClose, err := r.ReconcilePrices(closes, symbol)
	if err != nil {
		return Bar{}, fmt.Errorf("failed to reconcile close prices: %w", err)
	}

	reconciledVolume, err := r.ReconcilePrices(volumes, symbol)
	if err != nil {
		return Bar{}, fmt.Errorf("failed to reconcile volumes: %w", err)
	}

	// Create reconciled bar
	return Bar{
		Symbol:    symbol,
		Timestamp: timestamp,
		Open:      reconciledOpen.Price,
		High:      reconciledHigh.Price,
		Low:       reconciledLow.Price,
		Close:     reconciledClose.Price,
		Volume:    reconciledVolume.Price,
		Source:    fmt.Sprintf("reconciled_%d_sources", len(bars)),
	}, nil
}

// ReconcilePrices reconciles single price points from multiple sources
func (r *ReconcilerImpl) ReconcilePrices(sources map[string]float64, symbol string) (ReconciledPrice, error) {
	if len(sources) < r.config.MinSources {
		return ReconciledPrice{}, fmt.Errorf("insufficient sources: got %d, need %d", len(sources), r.config.MinSources)
	}

	// Convert to slice for processing
	prices := make([]float64, 0, len(sources))
	sourceNames := make([]string, 0, len(sources))
	sourcePrices := make(map[string]float64)

	for source, price := range sources {
		if price > 0 && !math.IsNaN(price) && !math.IsInf(price, 0) {
			prices = append(prices, price)
			sourceNames = append(sourceNames, source)
			sourcePrices[source] = price
		}
	}

	if len(prices) < r.config.MinSources {
		return ReconciledPrice{}, fmt.Errorf("insufficient valid prices after filtering")
	}

	// Detect and remove outliers
	filteredPrices, droppedSources := r.filterOutliers(prices, sourceNames)

	if len(filteredPrices) < r.config.MinSources {
		return ReconciledPrice{}, fmt.Errorf("too many outliers removed, insufficient data remains")
	}

	// Calculate reconciled price
	var reconciledPrice float64
	var method string

	if r.config.UseTrimmedMean {
		reconciledPrice = r.trimmedMean(filteredPrices)
		method = "trimmed_mean"
	} else {
		reconciledPrice = r.median(filteredPrices)
		method = "median"
	}

	// Calculate confidence and deviation
	deviation := r.standardDeviation(filteredPrices)
	confidence := r.calculateConfidence(filteredPrices, reconciledPrice, deviation)

	// Check confidence threshold
	if confidence < r.config.ConfidenceThreshold {
		return ReconciledPrice{}, fmt.Errorf("reconciled price confidence %.2f below threshold %.2f", confidence, r.config.ConfidenceThreshold)
	}

	return ReconciledPrice{
		Symbol:         symbol,
		Price:          reconciledPrice,
		Timestamp:      time.Now(),
		Method:         method,
		SourceCount:    len(filteredPrices),
		DroppedSources: droppedSources,
		SourcePrices:   sourcePrices,
		Confidence:     confidence,
		Deviation:      deviation,
		Attribution:    fmt.Sprintf("%s_of_%d_sources", method, len(filteredPrices)),
	}, nil
}

// filterOutliers removes prices that deviate more than MaxDeviation from the median
func (r *ReconcilerImpl) filterOutliers(prices []float64, sourceNames []string) ([]float64, []string) {
	if len(prices) <= 2 {
		return prices, nil // Can't filter with too few data points
	}

	// Calculate initial median
	sortedPrices := make([]float64, len(prices))
	copy(sortedPrices, prices)
	medianPrice := r.median(sortedPrices)

	// Filter outliers
	var filteredPrices []float64
	var droppedSources []string

	for i, price := range prices {
		deviation := math.Abs(price-medianPrice) / medianPrice
		if deviation <= r.config.MaxDeviation {
			filteredPrices = append(filteredPrices, price)
		} else {
			if i < len(sourceNames) {
				droppedSources = append(droppedSources, sourceNames[i])
			}
		}
	}

	return filteredPrices, droppedSources
}

// median calculates the median of a slice of prices
func (r *ReconcilerImpl) median(prices []float64) float64 {
	if len(prices) == 0 {
		return 0
	}

	// Sort the prices
	sorted := make([]float64, len(prices))
	copy(sorted, prices)
	sort.Float64s(sorted)

	n := len(sorted)
	if n%2 == 0 {
		// Even number of elements - average of two middle values
		return (sorted[n/2-1] + sorted[n/2]) / 2
	}
	// Odd number of elements - middle value
	return sorted[n/2]
}

// trimmedMean calculates trimmed mean by removing extreme values
func (r *ReconcilerImpl) trimmedMean(prices []float64) float64 {
	if len(prices) == 0 {
		return 0
	}

	// Sort the prices
	sorted := make([]float64, len(prices))
	copy(sorted, prices)
	sort.Float64s(sorted)

	// Calculate how many to trim from each end
	trimCount := int(float64(len(sorted)) * r.config.TrimPercent / 2)
	if trimCount >= len(sorted)/2 {
		trimCount = len(sorted)/2 - 1
	}
	if trimCount < 0 {
		trimCount = 0
	}

	// Extract middle portion
	start := trimCount
	end := len(sorted) - trimCount
	if start >= end {
		// Fall back to simple mean if trimming would remove everything
		return r.mean(prices)
	}

	// Calculate mean of trimmed data
	sum := 0.0
	count := 0
	for i := start; i < end; i++ {
		sum += sorted[i]
		count++
	}

	if count == 0 {
		return r.mean(prices)
	}

	return sum / float64(count)
}

// mean calculates simple arithmetic mean
func (r *ReconcilerImpl) mean(prices []float64) float64 {
	if len(prices) == 0 {
		return 0
	}

	sum := 0.0
	for _, price := range prices {
		sum += price
	}
	return sum / float64(len(prices))
}

// standardDeviation calculates standard deviation
func (r *ReconcilerImpl) standardDeviation(prices []float64) float64 {
	if len(prices) <= 1 {
		return 0
	}

	mean := r.mean(prices)
	sumSquaredDiffs := 0.0

	for _, price := range prices {
		diff := price - mean
		sumSquaredDiffs += diff * diff
	}

	variance := sumSquaredDiffs / float64(len(prices)-1)
	return math.Sqrt(variance)
}

// calculateConfidence calculates confidence score based on deviation and source count
func (r *ReconcilerImpl) calculateConfidence(prices []float64, reconciledPrice, deviation float64) float64 {
	if len(prices) == 0 || reconciledPrice == 0 {
		return 0
	}

	// Base confidence on relative deviation
	relativeDeviation := deviation / reconciledPrice
	deviationScore := math.Max(0, 1.0-relativeDeviation/r.config.MaxDeviation)

	// Bonus for more sources
	sourceBonus := math.Min(0.2, float64(len(prices)-r.config.MinSources)*0.05)

	// Ensure 0-1 range
	confidence := math.Min(1.0, deviationScore+sourceBonus)
	return math.Max(0.0, confidence)
}

// GetConfig returns the current reconciliation configuration
func (r *ReconcilerImpl) GetConfig() ReconciliationConfig {
	return r.config
}
