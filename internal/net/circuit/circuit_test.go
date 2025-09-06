package circuit

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestBreaker_ClosedState(t *testing.T) {
	config := Config{
		FailureThreshold: 3,
		SuccessThreshold: 2,
		Timeout:          100 * time.Millisecond,
		RequestTimeout:   50 * time.Millisecond,
	}
	breaker := NewBreaker(config)

	// Should start in closed state
	if breaker.State() != StateClosed {
		t.Errorf("Breaker should start in closed state, got %s", breaker.State())
	}

	// Successful requests should keep it closed
	err := breaker.Call(context.Background(), func(ctx context.Context) error {
		return nil
	})
	if err != nil {
		t.Errorf("Successful call should not error: %v", err)
	}

	if breaker.State() != StateClosed {
		t.Errorf("Breaker should remain closed after success, got %s", breaker.State())
	}
}

func TestBreaker_OpenOnFailures(t *testing.T) {
	config := Config{
		FailureThreshold: 3,
		SuccessThreshold: 2,
		Timeout:          100 * time.Millisecond,
		RequestTimeout:   50 * time.Millisecond,
	}
	breaker := NewBreaker(config)

	// Fail multiple times to open circuit
	for i := 0; i < 3; i++ {
		err := breaker.Call(context.Background(), func(ctx context.Context) error {
			return errors.New("test failure")
		})
		if err == nil {
			t.Error("Failed call should return error")
		}
	}

	// Should now be in open state
	if breaker.State() != StateOpen {
		t.Errorf("Breaker should be open after failures, got %s", breaker.State())
	}

	// Further requests should be blocked with ErrCircuitOpen
	err := breaker.Call(context.Background(), func(ctx context.Context) error {
		return nil
	})
	if err != ErrCircuitOpen {
		t.Errorf("Open breaker should return ErrCircuitOpen, got %v", err)
	}
}

func TestBreaker_HalfOpenRecovery(t *testing.T) {
	config := Config{
		FailureThreshold: 2,
		SuccessThreshold: 2,
		Timeout:          50 * time.Millisecond, // Short timeout for testing
		RequestTimeout:   100 * time.Millisecond,
	}
	breaker := NewBreaker(config)

	// Open the circuit with failures
	for i := 0; i < 2; i++ {
		breaker.Call(context.Background(), func(ctx context.Context) error {
			return errors.New("failure")
		})
	}

	if breaker.State() != StateOpen {
		t.Error("Breaker should be open")
	}

	// Wait for timeout to allow recovery attempt
	time.Sleep(60 * time.Millisecond)

	// First call after timeout should be allowed (transitions to half-open)
	err := breaker.Call(context.Background(), func(ctx context.Context) error {
		return nil
	})
	if err != nil {
		t.Errorf("First call after timeout should succeed: %v", err)
	}

	// Should be in half-open state after first success
	if breaker.State() != StateHalfOpen {
		t.Errorf("Breaker should be half-open, got %s", breaker.State())
	}

	// Need one more success to close
	err = breaker.Call(context.Background(), func(ctx context.Context) error {
		return nil
	})
	if err != nil {
		t.Errorf("Second success should not error: %v", err)
	}

	// Should now be closed
	if breaker.State() != StateClosed {
		t.Errorf("Breaker should be closed after success threshold, got %s", breaker.State())
	}
}

func TestBreaker_HalfOpenFailure(t *testing.T) {
	config := Config{
		FailureThreshold: 1,
		SuccessThreshold: 1,
		Timeout:          50 * time.Millisecond,
		RequestTimeout:   100 * time.Millisecond,
	}
	breaker := NewBreaker(config)

	// Open the circuit
	breaker.Call(context.Background(), func(ctx context.Context) error {
		return errors.New("failure")
	})

	if breaker.State() != StateOpen {
		t.Error("Breaker should be open")
	}

	// Wait for timeout
	time.Sleep(60 * time.Millisecond)

	// Fail in half-open state should return to open
	err := breaker.Call(context.Background(), func(ctx context.Context) error {
		return errors.New("half-open failure")
	})
	if err == nil {
		t.Error("Failed call should return error")
	}

	// Should be open again
	if breaker.State() != StateOpen {
		t.Errorf("Breaker should be open after half-open failure, got %s", breaker.State())
	}
}

func TestBreaker_Timeout(t *testing.T) {
	config := Config{
		FailureThreshold: 2,
		SuccessThreshold: 1,
		Timeout:          100 * time.Millisecond,
		RequestTimeout:   50 * time.Millisecond, // Short timeout
	}
	breaker := NewBreaker(config)

	// Call that takes longer than timeout
	err := breaker.Call(context.Background(), func(ctx context.Context) error {
		time.Sleep(100 * time.Millisecond) // Longer than request timeout
		return nil
	})

	if err != ErrRequestTimeout {
		t.Errorf("Should return timeout error, got %v", err)
	}

	// Timeouts should count as failures
	stats := breaker.Stats()
	if stats.TotalTimeouts == 0 {
		t.Error("Should record timeout")
	}
}

func TestBreaker_Stats(t *testing.T) {
	config := Config{
		FailureThreshold: 5,
		SuccessThreshold: 2,
		Timeout:          100 * time.Millisecond,
		RequestTimeout:   50 * time.Millisecond,
	}
	breaker := NewBreaker(config)

	// Mix of successes and failures
	breaker.Call(context.Background(), func(ctx context.Context) error { return nil })
	breaker.Call(context.Background(), func(ctx context.Context) error { return errors.New("fail") })
	breaker.Call(context.Background(), func(ctx context.Context) error { return nil })

	stats := breaker.Stats()

	if stats.TotalRequests != 3 {
		t.Errorf("Should have 3 total requests, got %d", stats.TotalRequests)
	}

	if stats.TotalSuccesses != 2 {
		t.Errorf("Should have 2 successes, got %d", stats.TotalSuccesses)
	}

	if stats.TotalFailures != 1 {
		t.Errorf("Should have 1 failure, got %d", stats.TotalFailures)
	}

	expectedSuccessRate := 2.0 / 3.0
	if abs(stats.SuccessRate-expectedSuccessRate) > 0.01 {
		t.Errorf("Success rate should be %.2f, got %.2f", expectedSuccessRate, stats.SuccessRate)
	}

	if stats.State != StateClosed {
		t.Errorf("Should be closed, got %s", stats.State)
	}

	if !stats.IsHealthy() {
		t.Error("Should be healthy with >90% success rate")
	}
}

func TestBreaker_Reset(t *testing.T) {
	config := Config{
		FailureThreshold: 2,
		SuccessThreshold: 1,
		Timeout:          100 * time.Millisecond,
		RequestTimeout:   50 * time.Millisecond,
	}
	breaker := NewBreaker(config)

	// Open the circuit
	breaker.Call(context.Background(), func(ctx context.Context) error { return errors.New("fail") })
	breaker.Call(context.Background(), func(ctx context.Context) error { return errors.New("fail") })

	if breaker.State() != StateOpen {
		t.Error("Breaker should be open")
	}

	// Reset should return to closed state and clear stats
	breaker.Reset()

	if breaker.State() != StateClosed {
		t.Errorf("Breaker should be closed after reset, got %s", breaker.State())
	}

	stats := breaker.Stats()
	if stats.TotalRequests != 0 {
		t.Errorf("Total requests should be 0 after reset, got %d", stats.TotalRequests)
	}
}

func TestBreaker_ForceStates(t *testing.T) {
	breaker := NewBreaker(Config{})

	// Force open
	breaker.ForceOpen()
	if breaker.State() != StateOpen {
		t.Error("ForceOpen should set state to open")
	}

	// Force half-open
	breaker.ForceHalfOpen()
	if breaker.State() != StateHalfOpen {
		t.Error("ForceHalfOpen should set state to half-open")
	}

	// Force closed
	breaker.ForceClosed()
	if breaker.State() != StateClosed {
		t.Error("ForceClosed should set state to closed")
	}
}

func TestManager_AddProvider(t *testing.T) {
	manager := NewManager()
	config := Config{
		FailureThreshold: 3,
		SuccessThreshold: 2,
		Timeout:          100 * time.Millisecond,
		RequestTimeout:   50 * time.Millisecond,
	}

	manager.AddProvider("test-provider", config)

	breaker, exists := manager.GetBreaker("test-provider")
	if !exists {
		t.Error("Provider should exist after adding")
	}

	if breaker == nil {
		t.Error("Breaker should not be nil")
	}

	if breaker.State() != StateClosed {
		t.Error("New breaker should be closed")
	}
}

func TestManager_Call(t *testing.T) {
	manager := NewManager()

	// No breaker configured - should execute directly
	err := manager.Call(context.Background(), "unknown-provider", func(ctx context.Context) error {
		return nil
	})
	if err != nil {
		t.Errorf("Should execute directly for unknown provider: %v", err)
	}

	// Add breaker and test
	config := Config{
		FailureThreshold: 1,
		SuccessThreshold: 1,
		Timeout:          100 * time.Millisecond,
		RequestTimeout:   50 * time.Millisecond,
	}
	manager.AddProvider("test-provider", config)

	// Successful call
	err = manager.Call(context.Background(), "test-provider", func(ctx context.Context) error {
		return nil
	})
	if err != nil {
		t.Errorf("Successful call should not error: %v", err)
	}

	// Failed call to open circuit
	err = manager.Call(context.Background(), "test-provider", func(ctx context.Context) error {
		return errors.New("failure")
	})
	if err == nil {
		t.Error("Failed call should return error")
	}

	// Next call should be blocked
	err = manager.Call(context.Background(), "test-provider", func(ctx context.Context) error {
		return nil
	})
	if err != ErrCircuitOpen {
		t.Errorf("Should return ErrCircuitOpen, got %v", err)
	}
}

func TestManager_Stats(t *testing.T) {
	manager := NewManager()

	config1 := Config{FailureThreshold: 3, SuccessThreshold: 2, Timeout: 100 * time.Millisecond, RequestTimeout: 50 * time.Millisecond}
	config2 := Config{FailureThreshold: 5, SuccessThreshold: 3, Timeout: 200 * time.Millisecond, RequestTimeout: 100 * time.Millisecond}

	manager.AddProvider("provider1", config1)
	manager.AddProvider("provider2", config2)

	// Make some calls
	manager.Call(context.Background(), "provider1", func(ctx context.Context) error { return nil })
	manager.Call(context.Background(), "provider2", func(ctx context.Context) error { return errors.New("fail") })

	allStats := manager.Stats()

	if len(allStats) != 2 {
		t.Errorf("Should have stats for 2 providers, got %d", len(allStats))
	}

	provider1Stats, exists := allStats["provider1"]
	if !exists {
		t.Error("Should have stats for provider1")
	}
	if provider1Stats.TotalRequests != 1 {
		t.Errorf("Provider1 should have 1 request, got %d", provider1Stats.TotalRequests)
	}

	provider2Stats, exists := allStats["provider2"]
	if !exists {
		t.Error("Should have stats for provider2")
	}
	if provider2Stats.TotalFailures != 1 {
		t.Errorf("Provider2 should have 1 failure, got %d", provider2Stats.TotalFailures)
	}
}

func TestManager_IsHealthy(t *testing.T) {
	manager := NewManager()

	// No providers - should be healthy
	if !manager.IsHealthy() {
		t.Error("Manager with no providers should be healthy")
	}

	// Add healthy provider
	config := Config{FailureThreshold: 5, SuccessThreshold: 2, Timeout: 100 * time.Millisecond, RequestTimeout: 50 * time.Millisecond}
	manager.AddProvider("healthy-provider", config)

	// Make successful calls
	for i := 0; i < 10; i++ {
		manager.Call(context.Background(), "healthy-provider", func(ctx context.Context) error { return nil })
	}

	if !manager.IsHealthy() {
		t.Error("Manager should be healthy with successful requests")
	}

	// Add unhealthy provider (open circuit)
	manager.AddProvider("unhealthy-provider", config)
	for i := 0; i < 5; i++ {
		manager.Call(context.Background(), "unhealthy-provider", func(ctx context.Context) error { return errors.New("fail") })
	}

	if manager.IsHealthy() {
		t.Error("Manager should be unhealthy with open circuit")
	}
}

func TestManager_GetUnhealthyProviders(t *testing.T) {
	manager := NewManager()

	config := Config{FailureThreshold: 2, SuccessThreshold: 1, Timeout: 100 * time.Millisecond, RequestTimeout: 50 * time.Millisecond}

	manager.AddProvider("healthy", config)
	manager.AddProvider("unhealthy", config)

	// Keep healthy provider healthy
	manager.Call(context.Background(), "healthy", func(ctx context.Context) error { return nil })

	// Make unhealthy provider unhealthy
	manager.Call(context.Background(), "unhealthy", func(ctx context.Context) error { return errors.New("fail") })
	manager.Call(context.Background(), "unhealthy", func(ctx context.Context) error { return errors.New("fail") })

	unhealthy := manager.GetUnhealthyProviders()

	if len(unhealthy) != 1 {
		t.Errorf("Should have 1 unhealthy provider, got %d", len(unhealthy))
	}

	if len(unhealthy) > 0 && !contains(unhealthy[0], "unhealthy") {
		t.Errorf("Unhealthy list should contain 'unhealthy' provider, got %v", unhealthy)
	}
}

// Helper function to check if string contains substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && s[:len(substr)] == substr ||
		len(s) > len(substr) && s[len(s)-len(substr):] == substr ||
		(len(s) > len(substr) && findInString(s, substr))
}

func findInString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// Helper function for floating point comparison
func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}
