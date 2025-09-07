package middleware

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/sawpanic/cryptorun/src/infrastructure/datafacade/interfaces"
)

func TestCircuitBreakerImpl_Call(t *testing.T) {
	cb := NewCircuitBreakerImpl()
	
	// Configure circuit breaker
	config := CircuitBreakerConfig{
		FailureThreshold: 3,
		SuccessThreshold: 2,
		Timeout:          100 * time.Millisecond,
		MaxRequests:      2,
	}
	cb.ConfigureBreaker("test_operation", config)
	
	ctx := context.Background()
	
	t.Run("allows successful calls in closed state", func(t *testing.T) {
		successFn := func() error { return nil }
		
		for i := 0; i < 5; i++ {
			err := cb.Call(ctx, "test_operation", successFn)
			if err != nil {
				t.Errorf("Successful call %d should not fail: %v", i, err)
			}
		}
		
		// Verify state is still closed
		state, err := cb.GetState(ctx, "test_operation")
		if err != nil {
			t.Fatalf("GetState failed: %v", err)
		}
		if state.State != "closed" {
			t.Errorf("Circuit should be closed, got %s", state.State)
		}
	})
	
	t.Run("opens circuit after failure threshold", func(t *testing.T) {
		// Configure a new circuit breaker for this test
		cb.ConfigureBreaker("failure_test", config)
		
		failureFn := func() error { return errors.New("operation failed") }
		
		// Generate failures to trip the circuit breaker
		for i := 0; i < 3; i++ {
			cb.Call(ctx, "failure_test", failureFn)
		}
		
		// Circuit should now be open
		state, err := cb.GetState(ctx, "failure_test")
		if err != nil {
			t.Fatalf("GetState failed: %v", err)
		}
		if state.State != "open" {
			t.Errorf("Circuit should be open, got %s", state.State)
		}
		
		// Subsequent calls should be rejected immediately
		err = cb.Call(ctx, "failure_test", func() error { return nil })
		if err == nil {
			t.Error("Call should be rejected when circuit is open")
		}
	})
	
	t.Run("transitions to half-open after timeout", func(t *testing.T) {
		// Configure circuit with very short timeout for testing
		shortConfig := CircuitBreakerConfig{
			FailureThreshold: 2,
			SuccessThreshold: 1,
			Timeout:          10 * time.Millisecond,
			MaxRequests:      1,
		}
		cb.ConfigureBreaker("timeout_test", shortConfig)
		
		failureFn := func() error { return errors.New("operation failed") }
		
		// Trip the circuit breaker
		cb.Call(ctx, "timeout_test", failureFn)
		cb.Call(ctx, "timeout_test", failureFn)
		
		// Verify circuit is open
		state, _ := cb.GetState(ctx, "timeout_test")
		if state.State != "open" {
			t.Errorf("Circuit should be open, got %s", state.State)
		}
		
		// Wait for timeout
		time.Sleep(15 * time.Millisecond)
		
		// Next call should be allowed (circuit transitions to half-open)
		successFn := func() error { return nil }
		err := cb.Call(ctx, "timeout_test", successFn)
		if err != nil {
			t.Errorf("Call should be allowed after timeout: %v", err)
		}
	})
	
	t.Run("closes circuit after success threshold in half-open", func(t *testing.T) {
		// Configure circuit for half-open testing
		halfOpenConfig := CircuitBreakerConfig{
			FailureThreshold: 2,
			SuccessThreshold: 2,
			Timeout:          10 * time.Millisecond,
			MaxRequests:      3,
		}
		cb.ConfigureBreaker("half_open_test", halfOpenConfig)
		
		// Trip the circuit breaker
		failureFn := func() error { return errors.New("operation failed") }
		cb.Call(ctx, "half_open_test", failureFn)
		cb.Call(ctx, "half_open_test", failureFn)
		
		// Wait for timeout to allow transition to half-open
		time.Sleep(15 * time.Millisecond)
		
		// Make successful calls to close the circuit
		successFn := func() error { return nil }
		cb.Call(ctx, "half_open_test", successFn)
		cb.Call(ctx, "half_open_test", successFn)
		
		// Circuit should now be closed
		state, _ := cb.GetState(ctx, "half_open_test")
		if state.State != "closed" {
			t.Errorf("Circuit should be closed after success threshold, got %s", state.State)
		}
	})
	
	t.Run("respects max requests in half-open state", func(t *testing.T) {
		// Configure circuit with max requests = 1
		limitedConfig := CircuitBreakerConfig{
			FailureThreshold: 2,
			SuccessThreshold: 1,
			Timeout:          10 * time.Millisecond,
			MaxRequests:      1,
		}
		cb.ConfigureBreaker("limited_test", limitedConfig)
		
		// Trip the circuit breaker
		failureFn := func() error { return errors.New("operation failed") }
		cb.Call(ctx, "limited_test", failureFn)
		cb.Call(ctx, "limited_test", failureFn)
		
		// Wait for timeout
		time.Sleep(15 * time.Millisecond)
		
		// First call should be allowed
		successFn := func() error { return nil }
		err := cb.Call(ctx, "limited_test", successFn)
		if err != nil {
			t.Errorf("First call in half-open should be allowed: %v", err)
		}
		
		// Second call should be rejected due to max requests limit
		err = cb.Call(ctx, "limited_test", successFn)
		if err == nil {
			t.Error("Second call should be rejected due to max requests limit")
		}
	})
}

func TestCircuitBreakerImpl_GetState(t *testing.T) {
	cb := NewCircuitBreakerImpl()
	
	t.Run("returns error for non-existent operation", func(t *testing.T) {
		_, err := cb.GetState(context.Background(), "non_existent")
		if err == nil {
			t.Error("GetState should return error for non-existent operation")
		}
	})
	
	t.Run("returns correct state information", func(t *testing.T) {
		config := CircuitBreakerConfig{
			FailureThreshold: 3,
			SuccessThreshold: 2,
			Timeout:          time.Minute,
			MaxRequests:      2,
		}
		cb.ConfigureBreaker("state_test", config)
		
		// Initial state should be closed
		state, err := cb.GetState(context.Background(), "state_test")
		if err != nil {
			t.Fatalf("GetState failed: %v", err)
		}
		
		if state.State != "closed" {
			t.Errorf("Initial state should be closed, got %s", state.State)
		}
		if state.FailureCount != 0 {
			t.Errorf("Initial failure count should be 0, got %d", state.FailureCount)
		}
		if state.SuccessCount != 0 {
			t.Errorf("Initial success count should be 0, got %d", state.SuccessCount)
		}
		if state.ErrorRate != 0.0 {
			t.Errorf("Initial error rate should be 0.0, got %f", state.ErrorRate)
		}
		
		// Make some calls to generate state
		failureFn := func() error { return errors.New("test failure") }
		successFn := func() error { return nil }
		
		cb.Call(context.Background(), "state_test", successFn)
		cb.Call(context.Background(), "state_test", failureFn)
		
		// Check updated state
		state, _ = cb.GetState(context.Background(), "state_test")
		if state.FailureCount != 1 {
			t.Errorf("Failure count should be 1, got %d", state.FailureCount)
		}
		if state.SuccessCount != 1 {
			t.Errorf("Success count should be 1, got %d", state.SuccessCount)
		}
		if state.ErrorRate != 0.5 {
			t.Errorf("Error rate should be 0.5, got %f", state.ErrorRate)
		}
	})
}

func TestCircuitBreakerImpl_ForceOpen(t *testing.T) {
	cb := NewCircuitBreakerImpl()
	
	config := CircuitBreakerConfig{
		FailureThreshold: 5,
		SuccessThreshold: 2,
		Timeout:          time.Minute,
		MaxRequests:      2,
	}
	cb.ConfigureBreaker("force_test", config)
	
	// Force circuit open
	err := cb.ForceOpen(context.Background(), "force_test")
	if err != nil {
		t.Fatalf("ForceOpen failed: %v", err)
	}
	
	// Verify circuit is open
	state, _ := cb.GetState(context.Background(), "force_test")
	if state.State != "open" {
		t.Errorf("Circuit should be open after ForceOpen, got %s", state.State)
	}
	
	// Calls should be rejected
	err = cb.Call(context.Background(), "force_test", func() error { return nil })
	if err == nil {
		t.Error("Call should be rejected when circuit is forced open")
	}
	
	// Test with non-existent operation
	err = cb.ForceOpen(context.Background(), "non_existent")
	if err == nil {
		t.Error("ForceOpen should fail for non-existent operation")
	}
}

func TestCircuitBreakerImpl_ForceClose(t *testing.T) {
	cb := NewCircuitBreakerImpl()
	
	config := CircuitBreakerConfig{
		FailureThreshold: 2,
		SuccessThreshold: 2,
		Timeout:          time.Hour, // Long timeout to keep circuit open
		MaxRequests:      2,
	}
	cb.ConfigureBreaker("force_close_test", config)
	
	// Trip the circuit breaker
	failureFn := func() error { return errors.New("test failure") }
	cb.Call(context.Background(), "force_close_test", failureFn)
	cb.Call(context.Background(), "force_close_test", failureFn)
	
	// Verify circuit is open
	state, _ := cb.GetState(context.Background(), "force_close_test")
	if state.State != "open" {
		t.Errorf("Circuit should be open, got %s", state.State)
	}
	
	// Force circuit closed
	err := cb.ForceClose(context.Background(), "force_close_test")
	if err != nil {
		t.Fatalf("ForceClose failed: %v", err)
	}
	
	// Verify circuit is closed
	state, _ = cb.GetState(context.Background(), "force_close_test")
	if state.State != "closed" {
		t.Errorf("Circuit should be closed after ForceClose, got %s", state.State)
	}
	
	// Calls should be allowed
	err = cb.Call(context.Background(), "force_close_test", func() error { return nil })
	if err != nil {
		t.Errorf("Call should be allowed when circuit is forced closed: %v", err)
	}
	
	// Test with non-existent operation
	err = cb.ForceClose(context.Background(), "non_existent")
	if err == nil {
		t.Error("ForceClose should fail for non-existent operation")
	}
}

func TestVenueCircuitBreaker(t *testing.T) {
	config := VenueConfig{
		HTTP: struct {
			FailureThreshold int
			SuccessThreshold int
			Timeout          time.Duration
			MaxRequests      int
		}{
			FailureThreshold: 3,
			SuccessThreshold: 2,
			Timeout:          100 * time.Millisecond,
			MaxRequests:      2,
		},
		WebSocket: struct {
			FailureThreshold int
			SuccessThreshold int
			Timeout          time.Duration
			MaxRequests      int
		}{
			FailureThreshold: 2,
			SuccessThreshold: 1,
			Timeout:          50 * time.Millisecond,
			MaxRequests:      1,
		},
		FallbackEnabled: true,
		FallbackVenues:  []string{"backup1", "backup2"},
	}
	
	vcb := NewVenueCircuitBreaker("test_venue", config)
	
	t.Run("tracks HTTP health correctly", func(t *testing.T) {
		// Initially should be healthy
		if !vcb.IsHTTPHealthy() {
			t.Error("HTTP should be healthy initially")
		}
		
		// Generate failures to trip HTTP circuit breaker
		for i := 0; i < 3; i++ {
			vcb.RecordHTTPFailure()
		}
		
		// Should no longer be healthy
		if vcb.IsHTTPHealthy() {
			t.Error("HTTP should not be healthy after failures")
		}
		
		// Wait for timeout and record success
		time.Sleep(110 * time.Millisecond)
		vcb.RecordHTTPSuccess()
		vcb.RecordHTTPSuccess()
		
		// Should be healthy again
		if !vcb.IsHTTPHealthy() {
			t.Error("HTTP should be healthy after recovery")
		}
	})
	
	t.Run("tracks WebSocket health correctly", func(t *testing.T) {
		// Initially should be healthy
		if !vcb.IsWSHealthy() {
			t.Error("WebSocket should be healthy initially")
		}
		
		// Generate failures to trip WebSocket circuit breaker
		vcb.RecordWSFailure()
		vcb.RecordWSFailure()
		
		// Should no longer be healthy
		if vcb.IsWSHealthy() {
			t.Error("WebSocket should not be healthy after failures")
		}
	})
	
	t.Run("returns correct overall status", func(t *testing.T) {
		// Create a fresh venue circuit breaker
		freshVCB := NewVenueCircuitBreaker("fresh_venue", config)
		
		status := freshVCB.GetStatus()
		if status.Venue != "fresh_venue" {
			t.Errorf("Expected venue 'fresh_venue', got %s", status.Venue)
		}
		if status.HTTPState != "closed" {
			t.Errorf("Expected HTTP state 'closed', got %s", status.HTTPState)
		}
		if status.WSState != "closed" {
			t.Errorf("Expected WS state 'closed', got %s", status.WSState)
		}
		if !status.IsHealthy {
			t.Error("Overall status should be healthy initially")
		}
	})
}

func TestHealthProbe(t *testing.T) {
	probe := NewHealthProbe("test_venue", "http://example.com/health", 5*time.Second)
	
	if probe.venue != "test_venue" {
		t.Errorf("Expected venue 'test_venue', got %s", probe.venue)
	}
	if probe.endpoint != "http://example.com/health" {
		t.Errorf("Expected endpoint 'http://example.com/health', got %s", probe.endpoint)
	}
	if probe.timeout != 5*time.Second {
		t.Errorf("Expected timeout 5s, got %v", probe.timeout)
	}
	
	// Test health check (this would normally make an HTTP request)
	err := probe.Check(context.Background())
	if err != nil {
		t.Errorf("Health check should succeed (mock implementation): %v", err)
	}
}

func TestMaxTime(t *testing.T) {
	now := time.Now()
	later := now.Add(time.Hour)
	
	max := maxTime(now, later)
	if !max.Equal(later) {
		t.Errorf("maxTime should return later time, got %v, expected %v", max, later)
	}
	
	max = maxTime(later, now)
	if !max.Equal(later) {
		t.Errorf("maxTime should return later time regardless of order, got %v, expected %v", max, later)
	}
}