package guards

import (
	"context"
	"net/http"
	"testing"
	"time"
)

func TestProviderGuard_CacheHit(t *testing.T) {
	config := ProviderConfig{
		Name:          "test",
		TTLSeconds:    300,
		BurstLimit:    10,
		SustainedRate: 1.0,
		MaxRetries:    3,
		BackoffBaseMs: 100,
	}

	guard := NewProviderGuard(config)

	// Pre-populate cache
	cacheKey := "test-key"
	entry := CacheEntry{
		Data:       []byte(`{"test": "data"}`),
		StatusCode: 200,
		Headers:    make(http.Header),
		Timestamp:  time.Now(),
	}
	guard.cache.Set(cacheKey, entry)

	req := GuardedRequest{
		Method:   "GET",
		URL:      "https://api.test.com/data",
		CacheKey: cacheKey,
	}

	// Should return cached data without calling fetcher
	resp, err := guard.Execute(context.Background(), req, func(ctx context.Context, req GuardedRequest) (*GuardedResponse, error) {
		t.Fatal("Fetcher should not be called on cache hit")
		return nil, nil
	})

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if !resp.Cached {
		t.Error("Response should be marked as cached")
	}

	if string(resp.Data) != `{"test": "data"}` {
		t.Errorf("Expected cached data, got: %s", string(resp.Data))
	}
}

func TestProviderGuard_CacheMiss(t *testing.T) {
	config := ProviderConfig{
		Name:          "test",
		TTLSeconds:    300,
		BurstLimit:    10,
		SustainedRate: 10.0, // High rate to avoid rate limiting
		MaxRetries:    3,
		BackoffBaseMs: 100,
	}

	guard := NewProviderGuard(config)

	req := GuardedRequest{
		Method:   "GET",
		URL:      "https://api.test.com/data",
		CacheKey: "cache-miss-key",
	}

	fetcherCalled := false
	resp, err := guard.Execute(context.Background(), req, func(ctx context.Context, req GuardedRequest) (*GuardedResponse, error) {
		fetcherCalled = true
		return &GuardedResponse{
			Data:       []byte(`{"fresh": "data"}`),
			StatusCode: 200,
			Headers:    make(http.Header),
		}, nil
	})

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if !fetcherCalled {
		t.Error("Fetcher should be called on cache miss")
	}

	if resp.Cached {
		t.Error("Response should not be marked as cached")
	}

	if string(resp.Data) != `{"fresh": "data"}` {
		t.Errorf("Expected fresh data, got: %s", string(resp.Data))
	}
}

func TestProviderGuard_CircuitBreakerOpen(t *testing.T) {
	config := ProviderConfig{
		Name:           "test",
		TTLSeconds:     300,
		BurstLimit:     10,
		SustainedRate:  10.0,
		MaxRetries:     1,
		BackoffBaseMs:  100,
		FailureThresh:  0.5,
		WindowRequests: 2,
	}

	guard := NewProviderGuard(config)

	// Force circuit breaker open by recording failures
	guard.circuit.RecordFailure()
	guard.circuit.RecordFailure()
	guard.circuit.RecordFailure() // Should open circuit

	req := GuardedRequest{
		Method:   "GET",
		URL:      "https://api.test.com/data",
		CacheKey: "circuit-test-key",
	}

	// Should fail immediately due to open circuit
	_, err := guard.Execute(context.Background(), req, func(ctx context.Context, req GuardedRequest) (*GuardedResponse, error) {
		t.Fatal("Fetcher should not be called when circuit is open")
		return nil, nil
	})

	if err == nil {
		t.Fatal("Expected circuit breaker error")
	}

	providerErr, ok := err.(*ProviderError)
	if !ok {
		t.Fatalf("Expected ProviderError, got: %T", err)
	}

	if providerErr.Provider != "test" {
		t.Errorf("Expected provider 'test', got: %s", providerErr.Provider)
	}

	if providerErr.Retryable {
		t.Error("Circuit breaker error should not be retryable")
	}
}

func TestProviderGuard_RetryLogic(t *testing.T) {
	config := ProviderConfig{
		Name:          "test",
		TTLSeconds:    300,
		BurstLimit:    10,
		SustainedRate: 10.0,
		MaxRetries:    2,
		BackoffBaseMs: 50, // Small for fast test
	}

	guard := NewProviderGuard(config)

	req := GuardedRequest{
		Method:   "GET",
		URL:      "https://api.test.com/data",
		CacheKey: "retry-test-key",
	}

	attemptCount := 0
	_, err := guard.Execute(context.Background(), req, func(ctx context.Context, req GuardedRequest) (*GuardedResponse, error) {
		attemptCount++
		if attemptCount <= 2 {
			// Return retryable error
			return &GuardedResponse{
				StatusCode: 500,
			}, nil
		}
		// Success on third attempt
		return &GuardedResponse{
			Data:       []byte(`{"success": "after_retries"}`),
			StatusCode: 200,
			Headers:    make(http.Header),
		}, nil
	})

	if err != nil {
		t.Fatalf("Expected success after retries, got: %v", err)
	}

	if attemptCount != 3 {
		t.Errorf("Expected 3 attempts, got: %d", attemptCount)
	}
}

func TestProviderGuard_Health(t *testing.T) {
	config := ProviderConfig{
		Name:          "test",
		TTLSeconds:    300,
		BurstLimit:    10,
		SustainedRate: 10.0,
		MaxRetries:    3,
		BackoffBaseMs: 100,
	}

	guard := NewProviderGuard(config)

	// Record some metrics
	guard.telemetry.RecordCacheHit(100 * time.Millisecond)
	guard.telemetry.RecordCacheMiss()
	guard.telemetry.RecordSuccess(200 * time.Millisecond)

	health := guard.Health()

	if health.Provider != "test" {
		t.Errorf("Expected provider 'test', got: %s", health.Provider)
	}

	if health.CircuitOpen {
		t.Error("Circuit should not be open initially")
	}

	expectedHitRate := 0.5 // 1 hit, 1 miss
	if health.CacheHitRate != expectedHitRate {
		t.Errorf("Expected cache hit rate %.2f, got: %.2f", expectedHitRate, health.CacheHitRate)
	}

	if health.RequestCount != 1 {
		t.Errorf("Expected request count 1, got: %d", health.RequestCount)
	}

	if health.ErrorRate != 0.0 {
		t.Errorf("Expected error rate 0.0, got: %.2f", health.ErrorRate)
	}
}

func TestProviderGuard_BackoffCalculation(t *testing.T) {
	config := ProviderConfig{
		Name:          "test",
		BackoffBaseMs: 100,
	}

	guard := NewProviderGuard(config)

	tests := []struct {
		attempt  int
		expected time.Duration
	}{
		{1, 100 * time.Millisecond}, // base * 2^0
		{2, 200 * time.Millisecond}, // base * 2^1
		{3, 400 * time.Millisecond}, // base * 2^2
		{4, 800 * time.Millisecond}, // base * 2^3
	}

	for _, test := range tests {
		backoff := guard.calculateBackoff(test.attempt)
		// Allow for jitter variance (±25%)
		minExpected := time.Duration(float64(test.expected) * 0.75)
		maxExpected := time.Duration(float64(test.expected) * 1.25)

		if backoff < minExpected || backoff > maxExpected {
			t.Errorf("Attempt %d: expected backoff between %v and %v, got: %v",
				test.attempt, minExpected, maxExpected, backoff)
		}
	}
}

func TestProviderGuard_RetryableStatusCodes(t *testing.T) {
	config := ProviderConfig{Name: "test"}
	guard := NewProviderGuard(config)

	retryableCodes := []int{429, 500, 502, 503, 504}
	nonRetryableCodes := []int{400, 401, 403, 404, 422}

	for _, code := range retryableCodes {
		if !guard.isRetryableStatus(code) {
			t.Errorf("Status code %d should be retryable", code)
		}
	}

	for _, code := range nonRetryableCodes {
		if guard.isRetryableStatus(code) {
			t.Errorf("Status code %d should not be retryable", code)
		}
	}
}

func TestProviderGuard_ExtractRetryAfter(t *testing.T) {
	config := ProviderConfig{Name: "test"}
	guard := NewProviderGuard(config)

	tests := []struct {
		retryAfter string
		expected   time.Duration
	}{
		{"30", 30 * time.Second},
		{"120", 120 * time.Second},
		{"", 0},
		{"invalid", 0},
	}

	for _, test := range tests {
		headers := make(http.Header)
		if test.retryAfter != "" {
			headers.Set("Retry-After", test.retryAfter)
		}

		duration := guard.extractRetryAfter(headers)
		if duration != test.expected {
			t.Errorf("Retry-After %q: expected %v, got: %v",
				test.retryAfter, test.expected, duration)
		}
	}
}
