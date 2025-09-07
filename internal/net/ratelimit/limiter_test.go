package ratelimit

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestLimiter_Allow(t *testing.T) {
	limiter := NewLimiter(2.0, 2) // 2 RPS, burst of 2

	// Should allow first 2 requests immediately (burst)
	if !limiter.Allow("test.com") {
		t.Error("First request should be allowed")
	}
	if !limiter.Allow("test.com") {
		t.Error("Second request should be allowed")
	}

	// Third request should be blocked (no tokens available)
	if limiter.Allow("test.com") {
		t.Error("Third request should be blocked")
	}
}

func TestLimiter_MultipleHosts(t *testing.T) {
	limiter := NewLimiter(1.0, 1) // 1 RPS, burst of 1

	// Each host should have independent rate limiting
	if !limiter.Allow("host1.com") {
		t.Error("First request to host1 should be allowed")
	}
	if !limiter.Allow("host2.com") {
		t.Error("First request to host2 should be allowed")
	}

	// Second requests should be blocked for both
	if limiter.Allow("host1.com") {
		t.Error("Second request to host1 should be blocked")
	}
	if limiter.Allow("host2.com") {
		t.Error("Second request to host2 should be blocked")
	}
}

func TestLimiter_Wait(t *testing.T) {
	limiter := NewLimiter(10.0, 1) // 10 RPS, burst of 1

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	// First request should pass immediately
	start := time.Now()
	err := limiter.Wait(ctx, "test.com")
	elapsed := time.Since(start)

	if err != nil {
		t.Errorf("Wait should not error on first request: %v", err)
	}
	if elapsed > 10*time.Millisecond {
		t.Errorf("First request should be immediate, took %v", elapsed)
	}

	// Second request should wait approximately 100ms (1/10 second for 10 RPS)
	start = time.Now()
	err = limiter.Wait(ctx, "test.com")
	elapsed = time.Since(start)

	if err != nil {
		t.Errorf("Wait should not error: %v", err)
	}
	if elapsed < 50*time.Millisecond || elapsed > 150*time.Millisecond {
		t.Errorf("Second request should wait ~100ms, took %v", elapsed)
	}
}

func TestLimiter_WaitTimeout(t *testing.T) {
	limiter := NewLimiter(0.1, 1) // Very slow: 0.1 RPS (10 second delay)

	// Use up the burst
	limiter.Allow("test.com")

	// Context with short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	start := time.Now()
	err := limiter.Wait(ctx, "test.com")
	elapsed := time.Since(start)

	if err == nil {
		t.Error("Wait should timeout with short context")
	}
	if elapsed > 150*time.Millisecond {
		t.Errorf("Wait should timeout quickly, took %v", elapsed)
	}
}

func TestLimiter_ConcurrentAccess(t *testing.T) {
	limiter := NewLimiter(100.0, 10) // 100 RPS, burst of 10
	host := "concurrent-test.com"

	const numGoroutines = 50
	const requestsPerGoroutine = 5

	var allowed, blocked int64
	var wg sync.WaitGroup

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < requestsPerGoroutine; j++ {
				if limiter.Allow(host) {
					atomic.AddInt64(&allowed, 1)
				} else {
					atomic.AddInt64(&blocked, 1)
				}
			}
		}()
	}

	wg.Wait()

	totalRequests := allowed + blocked
	expectedTotal := int64(numGoroutines * requestsPerGoroutine)

	if totalRequests != expectedTotal {
		t.Errorf("Total requests %d != expected %d", totalRequests, expectedTotal)
	}

	// Should allow some requests (at least the burst amount)
	if allowed < 10 {
		t.Errorf("Should allow at least burst amount, allowed %d", allowed)
	}

	// Should block some requests (more than burst available)
	if blocked == 0 {
		t.Errorf("Should block some requests with this load, blocked %d", blocked)
	}
}

func TestLimiter_Stats(t *testing.T) {
	limiter := NewLimiter(5.0, 10)
	host := "stats-test.com"

	// Use some tokens
	limiter.Allow(host)
	limiter.Allow(host)

	stats := limiter.Stats()
	hostStats, exists := stats[host]

	if !exists {
		t.Error("Stats should include the host")
	}

	if hostStats.Host != host {
		t.Errorf("Host stats should be for %s, got %s", host, hostStats.Host)
	}

	if hostStats.RPS != 5.0 {
		t.Errorf("RPS should be 5.0, got %f", hostStats.RPS)
	}

	if hostStats.Burst != 10 {
		t.Errorf("Burst should be 10, got %d", hostStats.Burst)
	}

	// Tokens available should be less than burst after using some
	if hostStats.TokensAvailable >= 10 {
		t.Errorf("Tokens available should be < 10 after usage, got %f", hostStats.TokensAvailable)
	}
}

func TestLimiter_SetRPS(t *testing.T) {
	limiter := NewLimiter(1.0, 2)
	host := "rps-test.com"

	// Use up initial tokens
	limiter.Allow(host)
	limiter.Allow(host)

	// Should be throttled at 1 RPS
	if limiter.Allow(host) {
		t.Error("Should be throttled at 1 RPS")
	}

	// Increase to 10 RPS - this also increases the bucket size effectively
	limiter.SetRPS(10.0)

	// Wait briefly for tokens to accumulate at new rate
	time.Sleep(150 * time.Millisecond)

	// Should now allow more requests
	if !limiter.Allow(host) {
		t.Error("Should allow requests after increasing RPS")
	}
}

func TestLimiter_Reset(t *testing.T) {
	limiter := NewLimiter(1.0, 1)
	host := "reset-test.com"

	// Use up tokens
	limiter.Allow(host)

	// Should be throttled
	if limiter.Allow(host) {
		t.Error("Should be throttled before reset")
	}

	// Reset should clear all limiters
	limiter.Reset()

	// Should allow requests again
	if !limiter.Allow(host) {
		t.Error("Should allow requests after reset")
	}
}

func TestManager_AddProvider(t *testing.T) {
	manager := NewManager()

	manager.AddProvider("test-provider", 5.0, 10)

	limiter, exists := manager.GetLimiter("test-provider")
	if !exists {
		t.Error("Provider should exist after adding")
	}

	if limiter == nil {
		t.Error("Limiter should not be nil")
	}
}

func TestManager_Allow(t *testing.T) {
	manager := NewManager()

	// No limiter configured - should allow
	if !manager.Allow("unknown-provider", "test.com") {
		t.Error("Should allow requests for unknown provider")
	}

	// Add limiter and test
	manager.AddProvider("test-provider", 1.0, 1)

	// First request should be allowed
	if !manager.Allow("test-provider", "test.com") {
		t.Error("First request should be allowed")
	}

	// Second request should be blocked
	if manager.Allow("test-provider", "test.com") {
		t.Error("Second request should be blocked")
	}
}

func TestManager_Stats(t *testing.T) {
	manager := NewManager()

	manager.AddProvider("provider1", 5.0, 10)
	manager.AddProvider("provider2", 3.0, 5)

	// Use some tokens
	manager.Allow("provider1", "test1.com")
	manager.Allow("provider2", "test2.com")

	allStats := manager.Stats()

	if len(allStats) != 2 {
		t.Errorf("Should have stats for 2 providers, got %d", len(allStats))
	}

	provider1Stats, exists := allStats["provider1"]
	if !exists {
		t.Error("Should have stats for provider1")
	}

	if len(provider1Stats) == 0 {
		t.Error("Provider1 should have host stats")
	}
}
