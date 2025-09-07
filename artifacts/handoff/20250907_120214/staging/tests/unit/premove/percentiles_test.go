package premove

import (
	"context"
	"math"
	"testing"
	"time"

	"github.com/sawpanic/cryptorun/src/domain/premove/ports"
	"github.com/sawpanic/cryptorun/src/infrastructure/percentiles"
)

func TestPercentileEngine_Calculate(t *testing.T) {
	engine := percentiles.NewPercentileEngine()

	// Test data with linear progression
	data := []float64{1.0, 2.0, 3.0, 4.0, 5.0, 6.0, 7.0, 8.0, 9.0, 10.0, 11.0, 12.0, 13.0, 14.0, 15.0, 16.0, 17.0, 18.0, 19.0, 20.0, 21.0, 22.0, 23.0, 24.0, 25.0}

	// Generate timestamps for 25 hours
	baseTime := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	timestamps := make([]time.Time, len(data))
	for i := range timestamps {
		timestamps[i] = baseTime.Add(time.Duration(i) * time.Hour)
	}

	ctx := context.Background()
	results, err := engine.Calculate(ctx, data, timestamps, 1) // 1 day window

	if err != nil {
		t.Fatalf("Calculate failed: %v", err)
	}

	if len(results) != len(data) {
		t.Fatalf("Expected %d results, got %d", len(data), len(results))
	}

	// Check last result with full window
	lastResult := results[len(results)-1]
	if !lastResult.IsValid {
		t.Error("Last result should be valid")
	}

	if lastResult.Count != 25 {
		t.Errorf("Expected count 25, got %d", lastResult.Count)
	}

	// Check percentile values make sense
	if lastResult.P50 != 13.0 { // Median of 1-25 should be 13
		t.Errorf("Expected P50=13.0, got %f", lastResult.P50)
	}
}

func TestPercentileEngine_InsufficientSamples(t *testing.T) {
	engine := percentiles.NewPercentileEngine()

	// 14d window needs at least 10 samples
	shortValues := []float64{1.0, 2.0, 3.0}
	_, ok := engine.Percentile(shortValues, 50.0, ports.PctWin14d)
	if ok {
		t.Error("Should fail with insufficient samples for 14d window")
	}

	// 30d window needs at least 20 samples
	mediumValues := make([]float64, 15)
	for i := range mediumValues {
		mediumValues[i] = float64(i + 1)
	}
	_, ok = engine.Percentile(mediumValues, 50.0, ports.PctWin30d)
	if ok {
		t.Error("Should fail with insufficient samples for 30d window")
	}
}

func TestPercentileEngine_Winsorization(t *testing.T) {
	engine := percentiles.NewPercentileEngine()

	// Values with extreme outliers
	values := []float64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 1000, 2000, 3000}

	// Calculate p80 - should be stable due to winsorization
	p80, ok := engine.Percentile(values, 80.0, ports.PctWin14d)
	if !ok {
		t.Error("P80 calculation should succeed")
	}

	// With winsorization, p80 should not be heavily influenced by outliers
	if p80 > 50 {
		t.Errorf("P80 too high (%.1f), winsorization may not be working", p80)
	}
}

func TestPercentileEngine_NaNHandling(t *testing.T) {
	engine := percentiles.NewPercentileEngine()

	values := []float64{1, 2, 3, math.NaN(), 4, 5, 6, 7, 8, 9, 10, math.Inf(1)}

	p50, ok := engine.Percentile(values, 50.0, ports.PctWin14d)
	if !ok {
		t.Error("Should handle NaN and Inf values")
	}

	if math.IsNaN(p50) || math.IsInf(p50, 0) {
		t.Error("Result should not be NaN or Inf")
	}
}
