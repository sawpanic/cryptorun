package datasources

import (
	"errors"
	"testing"
	"time"
)

func TestCircuitManager_CanMakeRequest(t *testing.T) {
	cm := NewCircuitManager()

	// Should be able to make requests initially (closed state)
	if !cm.CanMakeRequest("binance") {
		t.Error("Should be able to make request when circuit is closed")
	}

	// Unknown provider should return false
	if cm.CanMakeRequest("unknown") {
		t.Error("Should not be able to make request to unknown provider")
	}
}

func TestCircuitManager_RecordSuccess(t *testing.T) {
	cm := NewCircuitManager()

	// Record successful requests
	cm.RecordRequest("binance", true, 100*time.Millisecond, nil)
	cm.RecordRequest("binance", true, 200*time.Millisecond, nil)

	// Circuit should remain closed
	if cm.GetCircuitState("binance") != CircuitClosed {
		t.Error("Circuit should remain closed after successful requests")
	}
}

func TestCircuitManager_RecordFailures(t *testing.T) {
	cm := NewCircuitManager()

	// Record enough failures to open the circuit
	config := DefaultCircuitConfigs["binance"]

	// Need minimum requests before circuit can open
	for i := 0; i < config.MinRequestsInWindow; i++ {
		cm.RecordRequest("binance", false, 0, errors.New("test error"))
	}

	// Circuit should now be open
	if cm.GetCircuitState("binance") != CircuitOpen {
		t.Error("Circuit should be open after enough failures")
	}

	// Should not be able to make requests
	if cm.CanMakeRequest("binance") {
		t.Error("Should not be able to make request when circuit is open")
	}
}

func TestCircuitManager_LatencyThreshold(t *testing.T) {
	cm := NewCircuitManager()

	config := DefaultCircuitConfigs["binance"]
	slowLatency := config.LatencyThreshold + time.Second

	// Record slow requests (should be treated as failures)
	for i := 0; i < config.MinRequestsInWindow; i++ {
		cm.RecordRequest("binance", true, slowLatency, nil)
	}

	// Circuit should open due to slow requests
	if cm.GetCircuitState("binance") != CircuitOpen {
		t.Error("Circuit should be open after slow requests")
	}
}

func TestCircuitManager_HalfOpenTransition(t *testing.T) {
	cm := NewCircuitManager()

	// Open the circuit first
	cm.ForceOpen("binance")

	if cm.GetCircuitState("binance") != CircuitOpen {
		t.Error("Circuit should be open after ForceOpen")
	}

	// Modify timeout for testing (make it very short)
	circuit := cm.circuits["binance"]
	circuit.mu.Lock()
	circuit.config.Timeout = 10 * time.Millisecond
	circuit.lastFailTime = time.Now().Add(-20 * time.Millisecond) // Set in the past
	circuit.mu.Unlock()

	// Should transition to half-open after timeout
	if !cm.CanMakeRequest("binance") {
		t.Error("Should be able to make request after timeout (half-open state)")
	}

	if cm.GetCircuitState("binance") != CircuitHalfOpen {
		t.Error("Circuit should be half-open after timeout")
	}
}

func TestCircuitManager_HalfOpenToClosedTransition(t *testing.T) {
	cm := NewCircuitManager()

	// Set circuit to half-open
	circuit := cm.circuits["binance"]
	circuit.mu.Lock()
	circuit.state = CircuitHalfOpen
	circuit.successCount = 0
	circuit.mu.Unlock()

	config := DefaultCircuitConfigs["binance"]

	// Record successful requests to close the circuit
	for i := 0; i < config.SuccessThreshold; i++ {
		cm.RecordRequest("binance", true, 100*time.Millisecond, nil)
	}

	// Circuit should now be closed
	if cm.GetCircuitState("binance") != CircuitClosed {
		t.Error("Circuit should be closed after enough successes from half-open")
	}
}

func TestCircuitManager_FallbackProviders(t *testing.T) {
	cm := NewCircuitManager()

	// Open binance circuit
	cm.ForceOpen("binance")

	// Should get fallback provider
	activeProvider := cm.GetActiveProvider("binance")
	if activeProvider == "binance" {
		t.Error("Should return fallback provider when primary is down")
	}

	// Should be one of the fallback providers
	fallbacks := DefaultFallbackChains["binance"]
	found := false
	for _, fallback := range fallbacks {
		if activeProvider == fallback {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Active provider %s should be in fallback chain %v", activeProvider, fallbacks)
	}
}

func TestCircuitManager_BudgetThreshold(t *testing.T) {
	cm := NewCircuitManager()

	// Circuit should open when budget is too low
	lowHealthPercent := 5.0 // Below 10% threshold for binance
	cm.CheckBudgetThreshold("binance", lowHealthPercent)

	if cm.GetCircuitState("binance") != CircuitOpen {
		t.Error("Circuit should be open when budget is below threshold")
	}
}

func TestCircuitManager_ForceOperations(t *testing.T) {
	cm := NewCircuitManager()

	// Test force open
	cm.ForceOpen("binance")
	if cm.GetCircuitState("binance") != CircuitOpen {
		t.Error("Circuit should be open after ForceOpen")
	}

	// Test force close
	cm.ForceClose("binance")
	if cm.GetCircuitState("binance") != CircuitClosed {
		t.Error("Circuit should be closed after ForceClose")
	}
}

func TestCircuitBreaker_Stats(t *testing.T) {
	cm := NewCircuitManager()

	// Record some requests
	cm.RecordRequest("binance", true, 100*time.Millisecond, nil)
	cm.RecordRequest("binance", false, 200*time.Millisecond, errors.New("test error"))
	cm.RecordRequest("binance", true, 150*time.Millisecond, nil)

	circuit := cm.circuits["binance"]
	stats := circuit.GetStats()

	if stats.Provider != "binance" {
		t.Errorf("Expected provider binance, got %s", stats.Provider)
	}

	if stats.RequestCount != 3 {
		t.Errorf("Expected 3 requests, got %d", stats.RequestCount)
	}

	if stats.ErrorRate <= 0 {
		t.Error("Expected non-zero error rate")
	}

	if stats.AvgLatency <= 0 {
		t.Error("Expected positive average latency")
	}

	if stats.MaxLatency != 200*time.Millisecond {
		t.Errorf("Expected max latency 200ms, got %v", stats.MaxLatency)
	}
}

func TestCircuitManager_GetAllStats(t *testing.T) {
	cm := NewCircuitManager()

	// Record requests for multiple providers
	cm.RecordRequest("binance", true, 100*time.Millisecond, nil)
	cm.RecordRequest("coingecko", false, 200*time.Millisecond, errors.New("test"))

	allStats := cm.GetAllStats()

	if len(allStats) != len(DefaultCircuitConfigs) {
		t.Errorf("Expected %d provider stats, got %d", len(DefaultCircuitConfigs), len(allStats))
	}

	if _, exists := allStats["binance"]; !exists {
		t.Error("Expected binance stats to exist")
	}

	if _, exists := allStats["coingecko"]; !exists {
		t.Error("Expected coingecko stats to exist")
	}
}

func TestCircuitState_String(t *testing.T) {
	tests := []struct {
		state    CircuitState
		expected string
	}{
		{CircuitClosed, "closed"},
		{CircuitOpen, "open"},
		{CircuitHalfOpen, "half-open"},
		{CircuitState(999), "unknown"},
	}

	for _, test := range tests {
		if test.state.String() != test.expected {
			t.Errorf("Expected %s, got %s", test.expected, test.state.String())
		}
	}
}

func TestDefaultCircuitConfigs(t *testing.T) {
	// Verify all default configurations are valid
	for provider, config := range DefaultCircuitConfigs {
		if config.Provider != provider {
			t.Errorf("Provider %s config has mismatched name %s", provider, config.Provider)
		}
		if config.ErrorThreshold <= 0 {
			t.Errorf("Provider %s has invalid ErrorThreshold: %d", provider, config.ErrorThreshold)
		}
		if config.SuccessThreshold <= 0 {
			t.Errorf("Provider %s has invalid SuccessThreshold: %d", provider, config.SuccessThreshold)
		}
		if config.Timeout <= 0 {
			t.Errorf("Provider %s has invalid Timeout: %v", provider, config.Timeout)
		}
		if config.WindowSize <= 0 {
			t.Errorf("Provider %s has invalid WindowSize: %d", provider, config.WindowSize)
		}
		if config.MinRequestsInWindow <= 0 {
			t.Errorf("Provider %s has invalid MinRequestsInWindow: %d", provider, config.MinRequestsInWindow)
		}
	}
}

func TestDefaultFallbackChains(t *testing.T) {
	// Verify fallback chains are valid
	for provider, fallbacks := range DefaultFallbackChains {
		if len(fallbacks) == 0 {
			t.Errorf("Provider %s has no fallback providers", provider)
		}

		// Ensure fallback providers exist in configs
		for _, fallback := range fallbacks {
			if _, exists := DefaultCircuitConfigs[fallback]; !exists {
				t.Errorf("Provider %s has invalid fallback %s", provider, fallback)
			}
		}

		// Ensure provider doesn't include itself in fallbacks
		for _, fallback := range fallbacks {
			if fallback == provider {
				t.Errorf("Provider %s includes itself in fallback chain", provider)
			}
		}
	}
}

func TestCircuitManager_ConcurrentAccess(t *testing.T) {
	cm := NewCircuitManager()

	// Test concurrent access doesn't cause panics
	done := make(chan bool)

	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < 100; j++ {
				cm.CanMakeRequest("binance")
				cm.RecordRequest("binance", j%2 == 0, time.Duration(j)*time.Millisecond, nil)
				cm.GetCircuitState("binance")
				cm.GetActiveProvider("binance")
			}
			done <- true
		}()
	}

	// Wait for all goroutines to complete
	for i := 0; i < 10; i++ {
		<-done
	}
}
