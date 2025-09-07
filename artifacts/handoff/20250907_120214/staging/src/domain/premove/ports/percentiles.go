package ports

import (
	"context"
	"time"
)

type PercentileWindow string

const (
	PctWin14d PercentileWindow = "14d"
	PctWin30d PercentileWindow = "30d"
)

// PercentilePoint represents a percentile calculation for a specific timestamp
type PercentilePoint struct {
	Timestamp time.Time
	P10       float64
	P25       float64
	P50       float64
	P75       float64
	P90       float64
	Count     int
	IsValid   bool
}

type PercentileEngine interface {
	// Winsorize at ±3σ then compute percentile p ∈ [0,100].
	// Returns NaN-safe values; if < Nmin samples, returns (value, ok=false).
	Percentile(values []float64, p float64, w PercentileWindow) (float64, bool)

	// Calculate computes percentiles for a series over the specified window
	// Returns percentiles winsorized at ±3σ with 14d/30d windows
	Calculate(ctx context.Context, data []float64, timestamps []time.Time, windowDays int) ([]PercentilePoint, error)

	// GetLatest returns the most recent percentile calculation
	GetLatest(ctx context.Context, data []float64, timestamps []time.Time, windowDays int) (*PercentilePoint, error)
}
