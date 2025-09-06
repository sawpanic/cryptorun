package percentiles

import (
	"context"
	"fmt"
	"math"
	"sort"
	"time"

	"cryptorun/src/domain/premove/ports"
)

// Engine implements PercentileEngine with winsorized ±3σ percentiles
type Engine struct {
	minSamples int
}

// NewPercentileEngine creates a new percentile engine with default parameters
func NewPercentileEngine() *Engine {
	return &Engine{
		minSamples: 20, // Minimum samples required for valid percentile calculation
	}
}

// Percentile implements the legacy interface for backward compatibility
func (e *Engine) Percentile(values []float64, p float64, w ports.PercentileWindow) (float64, bool) {
	minSamples := getMinSamples(w)
	if len(values) < minSamples {
		return math.NaN(), false
	}

	// Winsorize at ±3σ
	winsorized := winsorize3Sigma(values)
	if len(winsorized) < minSamples {
		return math.NaN(), false
	}

	// Sort and compute percentile
	sort.Float64s(winsorized)
	return interpolatePercentile(winsorized, p), true
}

func getMinSamples(w ports.PercentileWindow) int {
	switch w {
	case ports.PctWin14d:
		return 10 // Minimum for 14d window
	case ports.PctWin30d:
		return 20 // Minimum for 30d window
	default:
		return 10
	}
}

func winsorize3Sigma(values []float64) []float64 {
	if len(values) < 3 {
		return append([]float64(nil), values...)
	}

	// Calculate mean and std dev
	var sum, sumSq float64
	validCount := 0
	for _, v := range values {
		if !math.IsNaN(v) && !math.IsInf(v, 0) {
			sum += v
			sumSq += v * v
			validCount++
		}
	}

	if validCount < 3 {
		return filterValidValues(values)
	}

	mean := sum / float64(validCount)
	variance := (sumSq - sum*sum/float64(validCount)) / float64(validCount-1)
	if variance <= 0 {
		return filterValidValues(values)
	}

	stdDev := math.Sqrt(variance)
	lowerBound := mean - 3*stdDev
	upperBound := mean + 3*stdDev

	// Winsorize outliers
	result := make([]float64, 0, len(values))
	for _, v := range values {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			continue
		}
		if v < lowerBound {
			result = append(result, lowerBound)
		} else if v > upperBound {
			result = append(result, upperBound)
		} else {
			result = append(result, v)
		}
	}

	return result
}

func filterValidValues(values []float64) []float64 {
	result := make([]float64, 0, len(values))
	for _, v := range values {
		if !math.IsNaN(v) && !math.IsInf(v, 0) {
			result = append(result, v)
		}
	}
	return result
}

// Calculate computes percentiles for a series over the specified window
func (e *Engine) Calculate(ctx context.Context, data []float64, timestamps []time.Time, windowDays int) ([]ports.PercentilePoint, error) {
	if len(data) != len(timestamps) {
		return nil, fmt.Errorf("data and timestamps must have same length: %d vs %d", len(data), len(timestamps))
	}

	if len(data) == 0 {
		return []ports.PercentilePoint{}, nil
	}

	windowDuration := time.Duration(windowDays) * 24 * time.Hour
	var results []ports.PercentilePoint

	for i := range data {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		windowStart := timestamps[i].Add(-windowDuration)
		windowData := e.getWindowData(data, timestamps, windowStart, timestamps[i])

		point := ports.PercentilePoint{
			Timestamp: timestamps[i],
			Count:     len(windowData),
			IsValid:   len(windowData) >= e.minSamples,
		}

		if point.IsValid {
			winsorized := winsorize3Sigma(windowData)
			if len(winsorized) > 0 {
				sort.Float64s(winsorized)
				point.P10 = interpolatePercentile(winsorized, 10)
				point.P25 = interpolatePercentile(winsorized, 25)
				point.P50 = interpolatePercentile(winsorized, 50)
				point.P75 = interpolatePercentile(winsorized, 75)
				point.P90 = interpolatePercentile(winsorized, 90)
			} else {
				point.IsValid = false
			}
		}

		results = append(results, point)
	}

	return results, nil
}

// GetLatest returns the most recent percentile calculation
func (e *Engine) GetLatest(ctx context.Context, data []float64, timestamps []time.Time, windowDays int) (*ports.PercentilePoint, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("no data provided")
	}

	results, err := e.Calculate(ctx, data, timestamps, windowDays)
	if err != nil {
		return nil, err
	}

	if len(results) == 0 {
		return nil, fmt.Errorf("no results calculated")
	}

	return &results[len(results)-1], nil
}

// getWindowData extracts data points within the specified time window
func (e *Engine) getWindowData(data []float64, timestamps []time.Time, windowStart, windowEnd time.Time) []float64 {
	var windowData []float64

	for i, ts := range timestamps {
		if (ts.Equal(windowStart) || ts.After(windowStart)) &&
			(ts.Equal(windowEnd) || ts.Before(windowEnd)) {
			windowData = append(windowData, data[i])
		}
	}

	return windowData
}

func interpolatePercentile(sortedValues []float64, p float64) float64 {
	if len(sortedValues) == 0 {
		return math.NaN()
	}
	if len(sortedValues) == 1 {
		return sortedValues[0]
	}

	// Linear interpolation percentile calculation
	index := (p / 100.0) * float64(len(sortedValues)-1)
	lower := int(math.Floor(index))
	upper := int(math.Ceil(index))

	if lower == upper {
		return sortedValues[lower]
	}

	weight := index - float64(lower)
	return sortedValues[lower]*(1-weight) + sortedValues[upper]*weight
}
