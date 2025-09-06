package budget

import (
	"testing"
	"time"
)

func TestTracker_Allow(t *testing.T) {
	tracker := NewTracker(100, 0, 0.8) // 100 daily limit, reset at midnight, 80% warning

	// Consume up to warning threshold
	for i := 0; i < 80; i++ {
		tracker.Consume()
	}

	// Should warn at 80%
	err := tracker.Allow()
	if err == nil {
		t.Error("Should return warning at 80% threshold")
	}
	if _, isWarning := err.(*BudgetWarningError); !isWarning {
		t.Errorf("Should return BudgetWarningError, got %T: %v", err, err)
	}

	// Consume to limit
	for i := 80; i < 100; i++ {
		tracker.Consume()
	}

	// Should block at limit
	err = tracker.Allow()
	if err == nil {
		t.Error("Should block at 100% limit")
	}
	if _, isExhausted := err.(*BudgetExhaustedError); !isExhausted {
		t.Errorf("Should return BudgetExhaustedError, got %T: %v", err, err)
	}
}

func TestTracker_Consume(t *testing.T) {
	tracker := NewTracker(10, 0, 0.8)

	// Consume under warning threshold
	for i := 0; i < 7; i++ {
		if err := tracker.Consume(); err != nil {
			t.Errorf("Should consume request %d: %v", i, err)
		}
	}

	// Should warn at 80%
	err := tracker.Consume() // 8th request = 80%
	if err == nil {
		t.Error("Should warn at 80% threshold")
	}
	if _, isWarning := err.(*BudgetWarningError); !isWarning {
		t.Errorf("Should return BudgetWarningError, got %T: %v", err, err)
	}

	// Consume remaining without error
	tracker.Consume() // 9th
	tracker.Consume() // 10th (at limit)

	// Should block further consumption
	err = tracker.Consume()
	if err == nil {
		t.Error("Should block consumption over limit")
	}
	if _, isExhausted := err.(*BudgetExhaustedError); !isExhausted {
		t.Errorf("Should return BudgetExhaustedError, got %T: %v", err, err)
	}

	// Usage count should not increment when blocked
	stats := tracker.Stats()
	if stats.Used != 10 {
		t.Errorf("Usage should be 10 after blocked attempt, got %d", stats.Used)
	}
}

func TestTracker_Stats(t *testing.T) {
	tracker := NewTracker(100, 12, 0.75) // Reset at noon

	// Consume some requests
	for i := 0; i < 30; i++ {
		tracker.Consume()
	}

	stats := tracker.Stats()

	if stats.Limit != 100 {
		t.Errorf("Limit should be 100, got %d", stats.Limit)
	}

	if stats.Used != 30 {
		t.Errorf("Used should be 30, got %d", stats.Used)
	}

	if stats.Remaining != 70 {
		t.Errorf("Remaining should be 70, got %d", stats.Remaining)
	}

	expectedUtilization := 0.30 // 30/100
	if abs64(stats.UtilizationRate-expectedUtilization) > 0.01 {
		t.Errorf("Utilization should be %.2f, got %.2f", expectedUtilization, stats.UtilizationRate)
	}

	if stats.WarnThreshold != 0.75 {
		t.Errorf("Warn threshold should be 0.75, got %.2f", stats.WarnThreshold)
	}

	if stats.ResetHour != 12 {
		t.Errorf("Reset hour should be 12, got %d", stats.ResetHour)
	}

	if stats.IsWarning {
		t.Error("Should not be warning at 30% utilization")
	}

	if stats.IsExhausted {
		t.Error("Should not be exhausted at 30% utilization")
	}

	// Check time calculations
	if stats.TimeToReset() <= 0 {
		t.Error("Time to reset should be positive")
	}
}

func TestTracker_Reset(t *testing.T) {
	tracker := NewTracker(50, 0, 0.8)

	// Use up budget
	for i := 0; i < 50; i++ {
		tracker.Consume()
	}

	// Should be exhausted
	stats := tracker.Stats()
	if !stats.IsExhausted {
		t.Error("Should be exhausted after consuming full budget")
	}

	// Reset manually
	tracker.Reset()

	// Should allow requests again
	if err := tracker.Allow(); err != nil {
		t.Errorf("Should allow requests after reset: %v", err)
	}

	stats = tracker.Stats()
	if stats.Used != 0 {
		t.Errorf("Used should be 0 after reset, got %d", stats.Used)
	}
}

func TestTracker_SetLimit(t *testing.T) {
	tracker := NewTracker(100, 0, 0.8)

	// Consume some requests
	for i := 0; i < 50; i++ {
		tracker.Consume()
	}

	// Reduce limit below current usage
	tracker.SetLimit(30)

	// Should be over limit now
	err := tracker.Allow()
	if err == nil {
		t.Error("Should block when current usage exceeds new limit")
	}

	// Increase limit above current usage
	tracker.SetLimit(60)

	// Should allow again
	if err := tracker.Allow(); err != nil {
		t.Errorf("Should allow when limit increased above usage: %v", err)
	}
}

func TestTracker_AutoReset(t *testing.T) {
	// This test is tricky because it involves time manipulation
	// For now, we'll test the logic without actually waiting
	now := time.Now().UTC()

	// Create tracker that should have reset "yesterday"
	tracker := NewTracker(100, now.Hour(), 0.8)

	// Manually set last reset to yesterday
	tracker.mu.Lock()
	tracker.lastReset = now.Add(-25 * time.Hour) // 25 hours ago
	tracker.mu.Unlock()

	// Use some budget
	for i := 0; i < 50; i++ {
		tracker.Consume()
	}

	// The checkAndResetIfNeeded should reset when we check
	// This happens automatically in Allow/Consume calls
	err := tracker.Allow()
	if err != nil {
		t.Errorf("Should allow after auto-reset: %v", err)
	}

	stats := tracker.Stats()
	if stats.Used >= 50 {
		t.Errorf("Usage should be reset, got %d", stats.Used)
	}
}

func TestManager_AddProvider(t *testing.T) {
	manager := NewManager()

	manager.AddProvider("test-provider", 1000, 0, 0.8)

	tracker, exists := manager.GetTracker("test-provider")
	if !exists {
		t.Error("Provider should exist after adding")
	}

	if tracker == nil {
		t.Error("Tracker should not be nil")
	}
}

func TestManager_Allow(t *testing.T) {
	manager := NewManager()

	// No tracker configured - should allow
	if err := manager.Allow("unknown-provider"); err != nil {
		t.Errorf("Should allow for unknown provider: %v", err)
	}

	// Add tracker and test
	manager.AddProvider("test-provider", 10, 0, 0.8)

	// Should allow under threshold
	for i := 0; i < 7; i++ {
		if err := manager.Allow("test-provider"); err != nil {
			t.Errorf("Should allow request %d: %v", i, err)
		}
	}

	// Should warn at threshold
	err := manager.Allow("test-provider")
	if err == nil {
		t.Error("Should warn at 80% threshold")
	}
}

func TestManager_Consume(t *testing.T) {
	manager := NewManager()

	// No tracker configured - should succeed
	if err := manager.Consume("unknown-provider"); err != nil {
		t.Errorf("Should consume for unknown provider: %v", err)
	}

	// Add tracker and test
	manager.AddProvider("test-provider", 5, 0, 0.8)

	// Consume to limit
	for i := 0; i < 5; i++ {
		manager.Consume("test-provider")
	}

	// Should block further consumption
	err := manager.Consume("test-provider")
	if err == nil {
		t.Error("Should block consumption at limit")
	}
}

func TestManager_Stats(t *testing.T) {
	manager := NewManager()

	manager.AddProvider("provider1", 100, 0, 0.8)
	manager.AddProvider("provider2", 200, 6, 0.9)

	// Use some budget
	for i := 0; i < 50; i++ {
		manager.Consume("provider1")
	}
	for i := 0; i < 30; i++ {
		manager.Consume("provider2")
	}

	allStats := manager.Stats()

	if len(allStats) != 2 {
		t.Errorf("Should have stats for 2 providers, got %d", len(allStats))
	}

	provider1Stats, exists := allStats["provider1"]
	if !exists {
		t.Error("Should have stats for provider1")
	}
	if provider1Stats.Used != 50 {
		t.Errorf("Provider1 should have used 50, got %d", provider1Stats.Used)
	}

	provider2Stats, exists := allStats["provider2"]
	if !exists {
		t.Error("Should have stats for provider2")
	}
	if provider2Stats.Used != 30 {
		t.Errorf("Provider2 should have used 30, got %d", provider2Stats.Used)
	}
}

func TestManager_GetWarnings(t *testing.T) {
	manager := NewManager()

	manager.AddProvider("low-usage", 100, 0, 0.8)
	manager.AddProvider("high-usage", 100, 0, 0.8)

	// Low usage - no warning
	for i := 0; i < 50; i++ {
		manager.Consume("low-usage")
	}

	// High usage - should warn
	for i := 0; i < 90; i++ {
		manager.Consume("high-usage")
	}

	warnings := manager.GetWarnings()

	if len(warnings) != 1 {
		t.Errorf("Should have 1 warning, got %d", len(warnings))
	}

	if len(warnings) > 0 && !containsSubstring(warnings[0], "high-usage") {
		t.Errorf("Warning should mention high-usage provider, got %s", warnings[0])
	}
}

func TestManager_GetExhausted(t *testing.T) {
	manager := NewManager()

	manager.AddProvider("normal", 100, 0, 0.8)
	manager.AddProvider("exhausted", 50, 0, 0.8)

	// Normal usage
	for i := 0; i < 30; i++ {
		manager.Consume("normal")
	}

	// Exhaust budget
	for i := 0; i < 50; i++ {
		manager.Consume("exhausted")
	}

	exhausted := manager.GetExhausted()

	if len(exhausted) != 1 {
		t.Errorf("Should have 1 exhausted provider, got %d", len(exhausted))
	}

	if len(exhausted) > 0 && !containsSubstring(exhausted[0], "exhausted") {
		t.Errorf("Exhausted list should mention exhausted provider, got %s", exhausted[0])
	}
}

func TestBudgetExhaustedError(t *testing.T) {
	eta := time.Now().Add(2 * time.Hour)
	err := &BudgetExhaustedError{
		Provider: "test-provider",
		Used:     100,
		Limit:    100,
		ETA:      eta,
	}

	msg := err.Error()
	if !containsSubstring(msg, "test-provider") {
		t.Errorf("Error message should contain provider name: %s", msg)
	}
	if !containsSubstring(msg, "100/100") {
		t.Errorf("Error message should contain usage: %s", msg)
	}
}

func TestBudgetWarningError(t *testing.T) {
	err := &BudgetWarningError{
		Provider:  "test-provider",
		Used:      80,
		Limit:     100,
		Threshold: 0.8,
	}

	msg := err.Error()
	if !containsSubstring(msg, "test-provider") {
		t.Errorf("Error message should contain provider name: %s", msg)
	}
	if !containsSubstring(msg, "80.0%") {
		t.Errorf("Error message should contain utilization percentage: %s", msg)
	}
}

// Helper functions
func abs64(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}

func containsSubstring(s, substr string) bool {
	if len(substr) > len(s) {
		return false
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
