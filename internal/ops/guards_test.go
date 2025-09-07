package ops

import (
	"testing"
	"time"
)

func TestGuardManager_BudgetGuard(t *testing.T) {
	config := GuardConfig{
		Budget: BudgetGuardConfig{
			Enabled:         true,
			HourlyLimit:     100,
			SoftWarnPercent: 0.8,
			HardStopPercent: 0.95,
		},
	}

	manager := NewGuardManager(config)

	// Record 85 API calls (85% of limit)
	for i := 0; i < 85; i++ {
		manager.RecordAPICall("test_provider")
	}

	results := manager.CheckAllGuards()
	budgetResult := findGuardResult(results, "budget")

	if budgetResult == nil {
		t.Fatal("Budget guard result not found")
	}

	// Should be warn or block status since we're at 85%
	if budgetResult.Status == GuardStatusOK {
		t.Errorf("Expected WARN or BLOCK status at high usage, got %s", budgetResult.Status.String())
	}

	// Add many more calls to definitely exceed hard stop
	for i := 0; i < 20; i++ {
		manager.RecordAPICall("test_provider")
	}

	results = manager.CheckAllGuards()
	budgetResult = findGuardResult(results, "budget")

	// With 105 total calls (105% of limit), should definitely be blocked
	if budgetResult.Status != GuardStatusBlock {
		// Debug info to see what percentage we actually have
		if metadata, ok := budgetResult.Metadata["usage_percent"]; ok {
			t.Logf("Actual usage: %.1f%%", metadata.(float64))
		}
		if metadata, ok := budgetResult.Metadata["current_calls"]; ok {
			t.Logf("Current calls: %v", metadata)
		}
		t.Errorf("Expected BLOCK status when clearly over limit, got %s", budgetResult.Status.String())
	}
}

func TestGuardManager_CallQuotaGuard(t *testing.T) {
	config := GuardConfig{
		CallQuota: CallQuotaGuardConfig{
			Enabled: true,
			Providers: map[string]ProviderQuotaConfig{
				"kraken": {
					CallsPerMinute: 60,
					BurstLimit:     10,
				},
			},
		},
	}

	manager := NewGuardManager(config)

	// Record calls within burst limit
	for i := 0; i < 8; i++ {
		manager.RecordAPICall("kraken")
	}

	results := manager.CheckAllGuards()
	quotaResult := findGuardResult(results, "kraken")

	if quotaResult == nil {
		t.Fatal("Quota guard result not found for kraken")
	}

	if quotaResult.Status != GuardStatusOK {
		t.Errorf("Expected OK status within limits, got %s", quotaResult.Status.String())
	}

	// Make many rapid calls to exceed burst limit
	for i := 0; i < 15; i++ {
		manager.RecordAPICall("kraken")
	}

	results = manager.CheckAllGuards()
	quotaResult = findGuardResult(results, "kraken")

	// With 23 total calls (8 + 15), should exceed both minute and burst limits
	if quotaResult.Status == GuardStatusOK {
		// Debug info
		if metadata, ok := quotaResult.Metadata["burst_calls"]; ok {
			t.Logf("Burst calls: %v", metadata)
		}
		if metadata, ok := quotaResult.Metadata["burst_limit"]; ok {
			t.Logf("Burst limit: %v", metadata)
		}
		if metadata, ok := quotaResult.Metadata["calls_per_minute"]; ok {
			t.Logf("Calls per minute: %v", metadata)
		}
		t.Errorf("Expected non-OK status when clearly over limits, got %s", quotaResult.Status.String())
	}
}

func TestGuardManager_CorrelationGuard(t *testing.T) {
	config := GuardConfig{
		Correlation: CorrelationGuardConfig{
			Enabled:         true,
			MaxCorrelation:  0.8,
			TopNSignals:     3,
			LookbackPeriods: 24,
		},
	}

	manager := NewGuardManager(config)

	// Record signals with similar scores (high correlation)
	signals := []SignalData{
		{Symbol: "BTC-USD", Score: 80.0, Timestamp: time.Now()},
		{Symbol: "ETH-USD", Score: 82.0, Timestamp: time.Now()},
		{Symbol: "SOL-USD", Score: 81.0, Timestamp: time.Now()},
	}

	for _, signal := range signals {
		manager.RecordSignal(signal)
	}

	results := manager.CheckAllGuards()
	corrResult := findGuardResult(results, "correlation")

	if corrResult == nil {
		t.Fatal("Correlation guard result not found")
	}

	// With similar scores, correlation should be high and potentially blocked
	if corrResult.Status == GuardStatusOK {
		// Check metadata for correlation value
		if metadata, ok := corrResult.Metadata["max_correlation"]; ok {
			if corr, ok := metadata.(float64); ok && corr > 0.8 {
				t.Errorf("Expected correlation guard to block with correlation %.3f > 0.8", corr)
			}
		}
	}
}

func TestGuardManager_VenueHealthGuard(t *testing.T) {
	config := GuardConfig{
		VenueHealth: VenueHealthGuardConfig{
			Enabled:          true,
			MinUptimePercent: 0.95,
			MaxLatencyMs:     5000,
			MinDepthUSD:      50000,
			MaxSpreadBps:     100,
		},
	}

	manager := NewGuardManager(config)

	results := manager.CheckAllGuards()
	venueResult := findGuardResult(results, "venue_health")

	if venueResult == nil {
		t.Fatal("Venue health guard result not found")
	}

	// Should be OK (placeholder implementation)
	if venueResult.Status != GuardStatusOK {
		t.Errorf("Expected OK status for venue health (placeholder), got %s", venueResult.Status.String())
	}
}

func TestGuardManager_DisabledGuards(t *testing.T) {
	config := GuardConfig{
		Budget: BudgetGuardConfig{
			Enabled: false, // Disabled
		},
		CallQuota: CallQuotaGuardConfig{
			Enabled: false, // Disabled
		},
	}

	manager := NewGuardManager(config)

	// Record some activity
	manager.RecordAPICall("test_provider")

	results := manager.CheckAllGuards()

	// Should have no results since guards are disabled
	if len(results) != 0 {
		t.Errorf("Expected no guard results when all guards disabled, got %d", len(results))
	}
}

func TestGuardManager_ResultCaching(t *testing.T) {
	config := GuardConfig{
		Budget: BudgetGuardConfig{
			Enabled:     true,
			HourlyLimit: 100,
		},
	}

	manager := NewGuardManager(config)

	// First check
	results1 := manager.CheckAllGuards()

	// Second check immediately after (should use cache)
	results2 := manager.CheckAllGuards()

	if len(results1) != len(results2) {
		t.Errorf("Expected same number of results from cache, got %d vs %d", len(results1), len(results2))
	}

	// Results should be identical
	for i, result1 := range results1 {
		result2 := results2[i]
		if result1.Name != result2.Name {
			t.Errorf("Expected same guard name, got %s vs %s", result1.Name, result2.Name)
		}
		if result1.Status != result2.Status {
			t.Errorf("Expected same guard status, got %s vs %s", result1.Status, result2.Status)
		}
	}
}

func TestGuardManager_APICallTracking(t *testing.T) {
	config := GuardConfig{
		Budget: BudgetGuardConfig{
			Enabled:     true,
			HourlyLimit: 100,
		},
	}

	manager := NewGuardManager(config)

	// Record calls for different providers
	manager.RecordAPICall("kraken")
	manager.RecordAPICall("binance")
	manager.RecordAPICall("kraken") // Another kraken call

	// Check that calls are tracked separately
	manager.mu.RLock()

	// Should have calls recorded for both providers
	if len(manager.providerCallTimes["kraken"]) != 2 {
		t.Errorf("Expected 2 calls for kraken, got %d", len(manager.providerCallTimes["kraken"]))
	}

	if len(manager.providerCallTimes["binance"]) != 1 {
		t.Errorf("Expected 1 call for binance, got %d", len(manager.providerCallTimes["binance"]))
	}

	// Should have total calls tracked for budget
	currentHour := time.Now().Truncate(time.Hour)
	if manager.hourlyCallCounts[currentHour] != 3 {
		t.Errorf("Expected 3 total calls this hour, got %d", manager.hourlyCallCounts[currentHour])
	}

	manager.mu.RUnlock()
}

func TestGuardManager_SignalTracking(t *testing.T) {
	config := GuardConfig{
		Correlation: CorrelationGuardConfig{
			Enabled:         true,
			TopNSignals:     5,
			LookbackPeriods: 24,
		},
	}

	manager := NewGuardManager(config)

	// Record multiple signals
	signals := []SignalData{
		{Symbol: "BTC-USD", Score: 75.0, Timestamp: time.Now()},
		{Symbol: "ETH-USD", Score: 68.0, Timestamp: time.Now()},
		{Symbol: "SOL-USD", Score: 82.0, Timestamp: time.Now()},
	}

	for _, signal := range signals {
		manager.RecordSignal(signal)
	}

	manager.mu.RLock()
	if len(manager.signalHistory) != 3 {
		t.Errorf("Expected 3 signals in history, got %d", len(manager.signalHistory))
	}
	manager.mu.RUnlock()

	// Record many more signals to test cleanup
	for i := 0; i < 100; i++ {
		manager.RecordSignal(SignalData{
			Symbol:    "TEST-USD",
			Score:     float64(i),
			Timestamp: time.Now(),
		})
	}

	manager.mu.RLock()
	maxHistory := config.Correlation.LookbackPeriods * config.Correlation.TopNSignals * 2
	if len(manager.signalHistory) > maxHistory {
		t.Errorf("Expected signal history to be capped at %d, got %d", maxHistory, len(manager.signalHistory))
	}
	manager.mu.RUnlock()
}

func TestGuardStatus_String(t *testing.T) {
	tests := []struct {
		status   GuardStatus
		expected string
	}{
		{GuardStatusOK, "OK"},
		{GuardStatusWarn, "WARN"},
		{GuardStatusCritical, "CRITICAL"},
		{GuardStatusBlock, "BLOCK"},
		{GuardStatus(999), "UNKNOWN"}, // Invalid status
	}

	for _, test := range tests {
		if test.status.String() != test.expected {
			t.Errorf("Expected %s for status %d, got %s", test.expected, int(test.status), test.status.String())
		}
	}
}

func TestGuardManager_CorrelationCalculation(t *testing.T) {
	config := GuardConfig{
		Correlation: CorrelationGuardConfig{
			Enabled:         true,
			MaxCorrelation:  0.5,
			TopNSignals:     2,
			LookbackPeriods: 24,
		},
	}

	manager := NewGuardManager(config)

	// Test with very different scores (low correlation)
	signals := []SignalData{
		{Symbol: "BTC-USD", Score: 10.0, Timestamp: time.Now()},
		{Symbol: "ETH-USD", Score: 90.0, Timestamp: time.Now()},
	}

	for _, signal := range signals {
		manager.RecordSignal(signal)
	}

	topSignals := manager.getTopNRecentSignals(2)
	correlation := manager.calculateMaxCorrelation(topSignals)

	// With scores 10 and 90, correlation should be low
	// (90-10)/90 = 0.89, so correlation = 1-0.89 = 0.11
	expectedCorr := 0.11
	tolerance := 0.02
	if correlation < expectedCorr-tolerance || correlation > expectedCorr+tolerance {
		t.Errorf("Expected correlation around %.2f, got %.2f", expectedCorr, correlation)
	}
}

func TestGuardManager_EmptySignalHistory(t *testing.T) {
	config := GuardConfig{
		Correlation: CorrelationGuardConfig{
			Enabled:         true,
			MaxCorrelation:  0.8,
			TopNSignals:     5,
			LookbackPeriods: 24,
		},
	}

	manager := NewGuardManager(config)

	// No signals recorded
	results := manager.CheckAllGuards()
	corrResult := findGuardResult(results, "correlation")

	if corrResult == nil {
		t.Fatal("Correlation guard result not found")
	}

	if corrResult.Status != GuardStatusOK {
		t.Errorf("Expected OK status with no signal history, got %s", corrResult.Status.String())
	}

	if !contains(corrResult.Message, "Insufficient signal history") {
		t.Errorf("Expected message about insufficient history, got: %s", corrResult.Message)
	}
}

// Helper function to find a guard result by name
func findGuardResult(results []GuardResult, name string) *GuardResult {
	for _, result := range results {
		if result.Name == name {
			return &result
		}
	}
	return nil
}

// Helper function to check if string contains substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && s[:len(substr)] == substr ||
		(len(s) > len(substr) && contains(s[1:], substr))
}
