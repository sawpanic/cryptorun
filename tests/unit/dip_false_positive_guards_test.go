package unit

import (
	"context"
	"testing"
	"time"

	"github.com/sawpanic/cryptorun/internal/algo/dip"
)

func TestDipGuards_NewsShockGuard_SevereDropWithoutRebound(t *testing.T) {
	config := dip.GuardsConfig{
		NewsShock: dip.NewsShockConfig{
			Return24hMin: -15.0, // -15% threshold
			AccelRebound: 3.0,   // 3% rebound required
			ReboundBars:  2,     // Check next 2 bars
		},
		StairStep: dip.StairStepConfig{MaxAttempts: 2, LowerHighWindow: 5},
		TimeDecay: dip.TimeDecayConfig{BarsToLive: 2},
	}

	guards := dip.NewDipGuards(config)

	// Create data with severe 20% drop
	data := createNewsShockData(50, -20.0, false) // No rebound

	dipPoint := &dip.DipPoint{
		Index:     40,
		Price:     80.0,
		Timestamp: time.Now().Add(-2 * time.Hour),
	}

	result, err := guards.ValidateEntry(context.Background(), dipPoint, data, time.Now())
	if err != nil {
		t.Fatalf("ValidateEntry failed: %v", err)
	}

	if result.Passed {
		t.Error("News shock guard should veto severe drop without rebound")
	}

	newsCheck, exists := result.GuardChecks["news_shock"]
	if !exists {
		t.Fatal("News shock check should exist")
	}

	if newsCheck.Passed {
		t.Error("News shock check should fail")
	}

	if newsCheck.Value >= config.NewsShock.Return24hMin {
		t.Errorf("Return should be below threshold: got %.1f%%, threshold %.1f%%",
			newsCheck.Value, config.NewsShock.Return24hMin)
	}

	expectedReason := "severe shock"
	if !containsSubstring(newsCheck.Reason, expectedReason) {
		t.Errorf("Reason should mention severe shock, got: %s", newsCheck.Reason)
	}

	t.Logf("✅ News shock guard correctly vetoed: %.1f%% drop without rebound", newsCheck.Value)
}

func TestDipGuards_NewsShockGuard_SevereDropWithRebound(t *testing.T) {
	config := dip.GuardsConfig{
		NewsShock: dip.NewsShockConfig{
			Return24hMin: -15.0,
			AccelRebound: 3.0,
			ReboundBars:  2,
		},
		StairStep: dip.StairStepConfig{MaxAttempts: 2, LowerHighWindow: 5},
		TimeDecay: dip.TimeDecayConfig{BarsToLive: 2},
	}

	guards := dip.NewDipGuards(config)

	// Create data with severe drop but strong rebound
	data := createNewsShockData(50, -18.0, true) // With rebound

	dipPoint := &dip.DipPoint{
		Index:     35,
		Price:     82.0,
		Timestamp: time.Now().Add(-2 * time.Hour),
	}

	result, err := guards.ValidateEntry(context.Background(), dipPoint, data, time.Now())
	if err != nil {
		t.Fatalf("ValidateEntry failed: %v", err)
	}

	newsCheck := result.GuardChecks["news_shock"]

	if !newsCheck.Passed {
		t.Error("News shock guard should pass with acceleration rebound")
	}

	expectedReason := "acceleration rebound detected"
	if newsCheck.Reason != expectedReason {
		t.Errorf("Expected reason '%s', got '%s'", expectedReason, newsCheck.Reason)
	}

	t.Logf("✅ News shock guard passed with rebound: %.1f%% drop but recovery detected", newsCheck.Value)
}

func TestDipGuards_StairStepGuard_PersistentLowerHighs(t *testing.T) {
	config := dip.GuardsConfig{
		NewsShock: dip.NewsShockConfig{Return24hMin: -15.0, AccelRebound: 3.0, ReboundBars: 2},
		StairStep: dip.StairStepConfig{
			MaxAttempts:     2,
			LowerHighWindow: 5,
		},
		TimeDecay: dip.TimeDecayConfig{BarsToLive: 2},
	}

	guards := dip.NewDipGuards(config)

	// Create stair-step pattern with persistent lower highs
	data := createStairStepData(40, 3) // 3 attempts at recovery

	dipPoint := &dip.DipPoint{
		Index:     35,
		Price:     85.0,
		Timestamp: time.Now().Add(-1 * time.Hour),
	}

	result, err := guards.ValidateEntry(context.Background(), dipPoint, data, time.Now())
	if err != nil {
		t.Fatalf("ValidateEntry failed: %v", err)
	}

	if result.Passed {
		t.Error("Stair-step guard should veto persistent lower highs pattern")
	}

	stairCheck := result.GuardChecks["stair_step"]
	if stairCheck.Passed {
		t.Error("Stair-step check should fail")
	}

	if stairCheck.Value < float64(config.StairStep.MaxAttempts) {
		t.Errorf("Should detect at least %d attempts, got %.0f",
			config.StairStep.MaxAttempts, stairCheck.Value)
	}

	expectedReason := "stair-step attempts"
	if !containsSubstring(stairCheck.Reason, expectedReason) {
		t.Errorf("Reason should mention stair-step attempts, got: %s", stairCheck.Reason)
	}

	t.Logf("✅ Stair-step guard correctly vetoed: %.0f attempts detected", stairCheck.Value)
}

func TestDipGuards_TimeDecayGuard_ExpiredSignal(t *testing.T) {
	config := dip.GuardsConfig{
		NewsShock: dip.NewsShockConfig{Return24hMin: -15.0, AccelRebound: 3.0, ReboundBars: 2},
		StairStep: dip.StairStepConfig{MaxAttempts: 2, LowerHighWindow: 5},
		TimeDecay: dip.TimeDecayConfig{BarsToLive: 2},
	}

	guards := dip.NewDipGuards(config)

	// Create clean data (should pass other guards)
	data := createCleanDipData(30)

	// Old dip point - expired
	dipPoint := &dip.DipPoint{
		Index:     25,
		Price:     95.0,
		Timestamp: time.Now().Add(-4 * time.Hour), // 4 hours ago = 4 bars
	}

	result, err := guards.ValidateEntry(context.Background(), dipPoint, data, time.Now())
	if err != nil {
		t.Fatalf("ValidateEntry failed: %v", err)
	}

	if result.Passed {
		t.Error("Time decay guard should veto expired signal")
	}

	timeCheck := result.GuardChecks["time_decay"]
	if timeCheck.Passed {
		t.Error("Time decay check should fail for expired signal")
	}

	if timeCheck.Value <= float64(config.TimeDecay.BarsToLive) {
		t.Errorf("Signal should be expired: %d bars elapsed, limit %d",
			int(timeCheck.Value), config.TimeDecay.BarsToLive)
	}

	expectedReason := "signal expired"
	if !containsSubstring(timeCheck.Reason, expectedReason) {
		t.Errorf("Reason should mention expiration, got: %s", timeCheck.Reason)
	}

	t.Logf("✅ Time decay guard correctly vetoed: %.0f bars elapsed (limit %d)",
		timeCheck.Value, config.TimeDecay.BarsToLive)
}

func TestDipGuards_AllGuardsPass_ValidDip(t *testing.T) {
	config := dip.GuardsConfig{
		NewsShock: dip.NewsShockConfig{Return24hMin: -15.0, AccelRebound: 3.0, ReboundBars: 2},
		StairStep: dip.StairStepConfig{MaxAttempts: 2, LowerHighWindow: 5},
		TimeDecay: dip.TimeDecayConfig{BarsToLive: 2},
	}

	guards := dip.NewDipGuards(config)

	// Create good quality dip data
	data := createCleanDipData(40)

	// Fresh dip point
	dipPoint := &dip.DipPoint{
		Index:     35,
		Price:     98.0,
		Timestamp: time.Now().Add(-30 * time.Minute), // 30 minutes ago = fresh
	}

	result, err := guards.ValidateEntry(context.Background(), dipPoint, data, time.Now())
	if err != nil {
		t.Fatalf("ValidateEntry failed: %v", err)
	}

	if !result.Passed {
		t.Errorf("All guards should pass for valid dip, but got veto: %s", result.VetoReason)
	}

	// Check individual guard results
	for name, check := range result.GuardChecks {
		if !check.Passed {
			t.Errorf("Guard %s should pass, but failed: %s", name, check.Reason)
		}
	}

	if len(result.GuardChecks) != 3 {
		t.Errorf("Expected 3 guard checks, got %d", len(result.GuardChecks))
	}

	t.Logf("✅ All guards passed for valid dip candidate")
}

func TestDipGuards_ValidateEntryTiming_PriceMovement(t *testing.T) {
	config := dip.GuardsConfig{
		NewsShock: dip.NewsShockConfig{Return24hMin: -15.0, AccelRebound: 3.0, ReboundBars: 2},
		StairStep: dip.StairStepConfig{MaxAttempts: 2, LowerHighWindow: 5},
		TimeDecay: dip.TimeDecayConfig{BarsToLive: 2},
	}

	guards := dip.NewDipGuards(config)

	// Create data where price has moved significantly from dip
	data := createCleanDipData(40)

	// Add recent bar with large price movement
	lastBar := data[len(data)-1]
	data = append(data, dip.MarketData{
		Timestamp: lastBar.Timestamp.Add(time.Hour),
		Open:      lastBar.Close,
		High:      lastBar.Close * 1.08, // 8% move up
		Low:       lastBar.Close * 0.99,
		Close:     lastBar.Close * 1.07, // 7% move from dip
		Volume:    1000000,
	})

	dipPoint := &dip.DipPoint{
		Index:     35,
		Price:     100.0, // Current price is ~107, so 7% movement
		Timestamp: time.Now().Add(-1 * time.Hour),
	}

	result, err := guards.ValidateEntryTiming(context.Background(), dipPoint, data, time.Now())
	if err != nil {
		t.Fatalf("ValidateEntryTiming failed: %v", err)
	}

	// Should pass standard guards but fail on price movement
	priceCheck, exists := result.GuardChecks["price_movement"]
	if !exists {
		t.Fatal("Price movement check should exist in timing validation")
	}

	if priceCheck.Passed {
		t.Error("Price movement check should fail for large movement from dip")
	}

	if priceCheck.Value <= 5.0 { // Default 5% threshold
		t.Errorf("Should detect significant price movement, got %.1f%%", priceCheck.Value)
	}

	t.Logf("✅ Entry timing validation correctly rejected: %.1f%% price movement", priceCheck.Value)
}

func TestDipGuards_MultipleGuardFailures_FirstVetoReturned(t *testing.T) {
	config := dip.GuardsConfig{
		NewsShock: dip.NewsShockConfig{Return24hMin: -10.0, AccelRebound: 3.0, ReboundBars: 2}, // Stricter
		StairStep: dip.StairStepConfig{MaxAttempts: 1, LowerHighWindow: 3},                     // Stricter
		TimeDecay: dip.TimeDecayConfig{BarsToLive: 1},                                          // Stricter
	}

	guards := dip.NewDipGuards(config)

	// Create data that fails multiple guards
	data := createNewsShockData(40, -12.0, false) // Fails news shock
	data = createStairStepDataFrom(data, 2)       // Also create stair-step pattern

	// Old and problematic dip point
	dipPoint := &dip.DipPoint{
		Index:     30,
		Price:     88.0,
		Timestamp: time.Now().Add(-3 * time.Hour), // Also expired
	}

	result, err := guards.ValidateEntry(context.Background(), dipPoint, data, time.Now())
	if err != nil {
		t.Fatalf("ValidateEntry failed: %v", err)
	}

	if result.Passed {
		t.Error("Should fail when multiple guards are violated")
	}

	// Should have failures for multiple guards
	failureCount := 0
	for _, check := range result.GuardChecks {
		if !check.Passed {
			failureCount++
		}
	}

	if failureCount < 2 {
		t.Errorf("Expected multiple guard failures, only got %d", failureCount)
	}

	// VetoReason should mention the first failure encountered
	if result.VetoReason == "" {
		t.Error("Should provide veto reason for failed guards")
	}

	t.Logf("✅ Multiple guard failures handled: %d failures, reason: %s",
		failureCount, result.VetoReason)
}

// Helper functions for creating test data

func createNewsShockData(periods int, shockReturn float64, withRebound bool) []dip.MarketData {
	data := make([]dip.MarketData, periods)
	basePrice := 100.0
	startTime := time.Now().Add(-time.Duration(periods) * time.Hour)

	shockStartIndex := periods - 30 // Shock in last 30 bars
	shockEndIndex := periods - 5    // Recovery period

	for i := 0; i < periods; i++ {
		var price float64

		if i < shockStartIndex {
			// Normal uptrend before shock
			price = basePrice * (1 + float64(i)*0.005)
		} else if i < shockEndIndex {
			// Shock period - distribute the drop
			shockProgress := float64(i-shockStartIndex) / float64(shockEndIndex-shockStartIndex)
			shockPrice := basePrice * 1.15 // Price before shock
			price = shockPrice * (1 + shockReturn/100.0*shockProgress)
		} else {
			// Recovery period
			shockLow := basePrice * 1.15 * (1 + shockReturn/100.0)
			if withRebound {
				// Strong rebound
				recoveryProgress := float64(i-shockEndIndex) / float64(periods-shockEndIndex)
				price = shockLow * (1 + 0.05*recoveryProgress) // 5% rebound
			} else {
				// Weak/no rebound
				price = shockLow * (1 + 0.005*float64(i-shockEndIndex))
			}
		}

		data[i] = dip.MarketData{
			Timestamp: startTime.Add(time.Duration(i) * time.Hour),
			Open:      price * 1.001,
			High:      price * 1.02,
			Low:       price * 0.98,
			Close:     price,
			Volume:    1000000,
		}
	}

	return data
}

func createStairStepData(periods int, attempts int) []dip.MarketData {
	data := make([]dip.MarketData, periods)
	basePrice := 100.0
	startTime := time.Now().Add(-time.Duration(periods) * time.Hour)

	windowSize := 5
	currentHigh := basePrice * 1.2

	for i := 0; i < periods; i++ {
		windowIndex := i / windowSize

		var price float64
		if windowIndex < attempts {
			// Each window makes a lower high
			windowHigh := currentHigh * (1 - float64(windowIndex)*0.03) // 3% lower each attempt
			windowProgress := float64(i%windowSize) / float64(windowSize)

			if windowProgress < 0.6 {
				// Rise to lower high
				price = basePrice * (1 + windowProgress*0.15)
			} else {
				// Fall from lower high
				price = windowHigh * (1 - (windowProgress-0.6)*0.1)
			}
		} else {
			// After attempts, sideways/declining
			price = basePrice * 0.9
		}

		data[i] = dip.MarketData{
			Timestamp: startTime.Add(time.Duration(i) * time.Hour),
			Open:      price * 1.002,
			High:      price * 1.01,
			Low:       price * 0.99,
			Close:     price,
			Volume:    1000000,
		}
	}

	return data
}

func createStairStepDataFrom(existing []dip.MarketData, attempts int) []dip.MarketData {
	// Modify existing data to add stair-step pattern
	windowSize := 5
	startIndex := len(existing) - attempts*windowSize - 5

	if startIndex < 0 {
		startIndex = 0
	}

	basePrice := existing[startIndex].Close
	currentHigh := basePrice * 1.1

	for i := startIndex; i < len(existing); i++ {
		relativeIndex := i - startIndex
		windowIndex := relativeIndex / windowSize

		if windowIndex < attempts {
			windowHigh := currentHigh * (1 - float64(windowIndex)*0.04)
			windowProgress := float64(relativeIndex%windowSize) / float64(windowSize)

			var price float64
			if windowProgress < 0.5 {
				price = basePrice * (1 + windowProgress*0.08)
			} else {
				price = windowHigh * (1 - (windowProgress-0.5)*0.08)
			}

			existing[i].Close = price
			existing[i].High = price * 1.01
			existing[i].Low = price * 0.99
		}
	}

	return existing
}

func createCleanDipData(periods int) []dip.MarketData {
	data := make([]dip.MarketData, periods)
	basePrice := 100.0
	startTime := time.Now().Add(-time.Duration(periods) * time.Hour)

	for i := 0; i < periods; i++ {
		// Gentle uptrend with small dip near end
		var price float64
		if i < periods-5 {
			price = basePrice * (1 + float64(i)*0.002) // 0.2% per hour
		} else {
			// Small healthy pullback
			pullbackStart := basePrice * (1 + float64(periods-5)*0.002)
			pullbackProgress := float64(i-(periods-5)) / 5.0
			price = pullbackStart * (1 - pullbackProgress*0.02) // 2% pullback
		}

		data[i] = dip.MarketData{
			Timestamp: startTime.Add(time.Duration(i) * time.Hour),
			Open:      price * 1.001,
			High:      price * 1.005,
			Low:       price * 0.995,
			Close:     price,
			Volume:    1000000,
		}
	}

	return data
}

func containsSubstring(s, substr string) bool {
	return len(s) >= len(substr) &&
		func() bool {
			for i := 0; i <= len(s)-len(substr); i++ {
				if s[i:i+len(substr)] == substr {
					return true
				}
			}
			return false
		}()
}
