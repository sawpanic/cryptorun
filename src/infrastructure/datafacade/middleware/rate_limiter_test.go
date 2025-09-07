package middleware

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/sawpanic/cryptorun/src/infrastructure/datafacade/interfaces"
)

func TestTokenBucketRateLimiter_Allow(t *testing.T) {
	rl := NewTokenBucketRateLimiter()
	
	// Configure rate limits for test venue
	limits := &interfaces.RateLimits{
		RequestsPerSecond: 10,
		BurstAllowance:    5,
		WeightLimits: map[string]int{
			"trades": 1,
			"heavy":  5,
		},
		DailyLimit:   intPtr(1000),
		MonthlyLimit: intPtr(30000),
	}
	
	err := rl.UpdateLimits(context.Background(), "test", limits)
	if err != nil {
		t.Fatalf("UpdateLimits failed: %v", err)
	}
	
	ctx := context.Background()
	
	t.Run("allows requests within limits", func(t *testing.T) {
		// Should allow requests within burst allowance
		for i := 0; i < 5; i++ {
			err := rl.Allow(ctx, "test", "trades")
			if err != nil {
				t.Errorf("Request %d should be allowed: %v", i, err)
			}
		}
	})
	
	t.Run("rejects requests over burst limit", func(t *testing.T) {
		// Configure a very restrictive rate limiter
		restrictiveLimits := &interfaces.RateLimits{
			RequestsPerSecond: 1,
			BurstAllowance:    1,
			WeightLimits:      map[string]int{},
			DailyLimit:        intPtr(100),
			MonthlyLimit:      intPtr(3000),
		}
		
		restrictiveRL := NewTokenBucketRateLimiter()
		restrictiveRL.UpdateLimits(ctx, "restrictive", restrictiveLimits)
		
		// First request should pass
		err := restrictiveRL.Allow(ctx, "restrictive", "test")
		if err != nil {
			t.Errorf("First request should be allowed: %v", err)
		}
		
		// Second request should be rejected
		err = restrictiveRL.Allow(ctx, "restrictive", "test")
		if err == nil {
			t.Error("Second request should be rejected")
		}
	})
	
	t.Run("handles weight limits", func(t *testing.T) {
		// Create a fresh rate limiter for weight testing
		weightRL := NewTokenBucketRateLimiter()
		weightLimits := &interfaces.RateLimits{
			RequestsPerSecond: 100, // High RPS to focus on weight limits
			BurstAllowance:    50,
			WeightLimits: map[string]int{
				"light":  1,
				"heavy":  10,
			},
			DailyLimit:   intPtr(10000),
			MonthlyLimit: intPtr(300000),
		}
		
		weightRL.UpdateLimits(ctx, "weight_test", weightLimits)
		
		// Should allow multiple light requests
		for i := 0; i < 5; i++ {
			err := weightRL.Allow(ctx, "weight_test", "light")
			if err != nil {
				t.Errorf("Light request %d should be allowed: %v", i, err)
			}
		}
		
		// Heavy request should work initially
		err := weightRL.Allow(ctx, "weight_test", "heavy")
		if err != nil {
			t.Errorf("Heavy request should be allowed: %v", err)
		}
	})
	
	t.Run("rejects unknown venue", func(t *testing.T) {
		err := rl.Allow(ctx, "unknown_venue", "trades")
		if err == nil {
			t.Error("Request to unknown venue should be rejected")
		}
	})
}

func TestTokenBucketRateLimiter_GetLimits(t *testing.T) {
	rl := NewTokenBucketRateLimiter()
	
	originalLimits := &interfaces.RateLimits{
		RequestsPerSecond: 15,
		BurstAllowance:    8,
		WeightLimits:      map[string]int{"test": 2},
		DailyLimit:        intPtr(2000),
		MonthlyLimit:      intPtr(60000),
	}
	
	err := rl.UpdateLimits(context.Background(), "test", originalLimits)
	if err != nil {
		t.Fatalf("UpdateLimits failed: %v", err)
	}
	
	retrievedLimits, err := rl.GetLimits(context.Background(), "test")
	if err != nil {
		t.Fatalf("GetLimits failed: %v", err)
	}
	
	if retrievedLimits.RequestsPerSecond != originalLimits.RequestsPerSecond {
		t.Errorf("RequestsPerSecond mismatch: got %d, want %d", 
			retrievedLimits.RequestsPerSecond, originalLimits.RequestsPerSecond)
	}
	
	if retrievedLimits.BurstAllowance != originalLimits.BurstAllowance {
		t.Errorf("BurstAllowance mismatch: got %d, want %d", 
			retrievedLimits.BurstAllowance, originalLimits.BurstAllowance)
	}
	
	if *retrievedLimits.DailyLimit != *originalLimits.DailyLimit {
		t.Errorf("DailyLimit mismatch: got %d, want %d", 
			*retrievedLimits.DailyLimit, *originalLimits.DailyLimit)
	}
}

func TestTokenBucketRateLimiter_ProcessRateLimitHeaders(t *testing.T) {
	rl := NewTokenBucketRateLimiter()
	
	// Configure rate limits first
	limits := &interfaces.RateLimits{
		RequestsPerSecond: 10,
		BurstAllowance:    5,
		WeightLimits:      map[string]int{"test": 1},
		DailyLimit:        intPtr(1000),
		MonthlyLimit:      intPtr(30000),
	}
	
	rl.UpdateLimits(context.Background(), "binance", limits)
	
	t.Run("processes Binance headers", func(t *testing.T) {
		headers := map[string]string{
			"X-MBX-USED-WEIGHT": "100",
			"Retry-After":       "5",
		}
		
		err := rl.ProcessRateLimitHeaders("binance", headers)
		if err != nil {
			t.Errorf("ProcessRateLimitHeaders failed: %v", err)
		}
		
		// Verify that requests are blocked due to retry-after
		err = rl.Allow(context.Background(), "binance", "test")
		if err == nil {
			t.Error("Request should be blocked due to Retry-After header")
		}
	})
	
	t.Run("handles unknown venue", func(t *testing.T) {
		headers := map[string]string{
			"X-RateLimit-Remaining": "0",
		}
		
		err := rl.ProcessRateLimitHeaders("unknown", headers)
		if err == nil {
			t.Error("ProcessRateLimitHeaders should fail for unknown venue")
		}
	})
}

func TestBudgetTracker(t *testing.T) {
	bt := NewBudgetTracker()
	
	// Update limits for test venue
	bt.UpdateLimits("test", intPtr(10), intPtr(100))
	
	t.Run("allows requests within budget", func(t *testing.T) {
		// Should allow requests within daily limit
		for i := 0; i < 5; i++ {
			err := bt.CheckBudget("test")
			if err != nil {
				t.Errorf("Request %d should be within budget: %v", i, err)
			}
			bt.IncrementUsage("test")
		}
	})
	
	t.Run("rejects requests over daily budget", func(t *testing.T) {
		// Set a very low daily limit
		bt.UpdateLimits("budget_test", intPtr(2), intPtr(100))
		
		// Use up the daily budget
		for i := 0; i < 2; i++ {
			bt.CheckBudget("budget_test")
			bt.IncrementUsage("budget_test")
		}
		
		// Next request should be rejected
		err := bt.CheckBudget("budget_test")
		if err == nil {
			t.Error("Request should be rejected due to daily budget limit")
		}
	})
	
	t.Run("allows unlimited requests for unconfigured venue", func(t *testing.T) {
		err := bt.CheckBudget("unlimited")
		if err != nil {
			t.Errorf("Unconfigured venue should allow unlimited requests: %v", err)
		}
	})
}

func TestUsageCounter(t *testing.T) {
	t.Run("increments count correctly", func(t *testing.T) {
		counter := NewUsageCounter(time.Hour)
		
		if counter.GetCount() != 0 {
			t.Errorf("Initial count should be 0, got %d", counter.GetCount())
		}
		
		counter.Increment()
		if counter.GetCount() != 1 {
			t.Errorf("Count after increment should be 1, got %d", counter.GetCount())
		}
		
		counter.Increment()
		if counter.GetCount() != 2 {
			t.Errorf("Count after second increment should be 2, got %d", counter.GetCount())
		}
	})
	
	t.Run("resets after window expires", func(t *testing.T) {
		// Use a very short window for testing
		counter := NewUsageCounter(time.Millisecond)
		
		counter.Increment()
		if counter.GetCount() != 1 {
			t.Errorf("Count should be 1, got %d", counter.GetCount())
		}
		
		// Wait for window to expire
		time.Sleep(2 * time.Millisecond)
		
		if counter.GetCount() != 0 {
			t.Errorf("Count should reset to 0 after window expires, got %d", counter.GetCount())
		}
	})
}

func TestBackoffCalculator(t *testing.T) {
	t.Run("calculates exponential backoff", func(t *testing.T) {
		bc := NewBackoffCalculator(time.Second, time.Minute, 2.0)
		bc.jitterEnabled = false // Disable jitter for predictable tests
		
		delay1 := bc.NextDelay()
		if delay1 != time.Second {
			t.Errorf("First delay should be %v, got %v", time.Second, delay1)
		}
		
		delay2 := bc.NextDelay()
		if delay2 != 2*time.Second {
			t.Errorf("Second delay should be %v, got %v", 2*time.Second, delay2)
		}
		
		delay3 := bc.NextDelay()
		if delay3 != 4*time.Second {
			t.Errorf("Third delay should be %v, got %v", 4*time.Second, delay3)
		}
	})
	
	t.Run("respects maximum delay", func(t *testing.T) {
		bc := NewBackoffCalculator(time.Second, 3*time.Second, 2.0)
		bc.jitterEnabled = false // Disable jitter for predictable tests
		
		// Skip to high retry count
		for i := 0; i < 10; i++ {
			bc.NextDelay()
		}
		
		delay := bc.NextDelay()
		if delay > 3*time.Second {
			t.Errorf("Delay should not exceed max delay %v, got %v", 3*time.Second, delay)
		}
	})
	
	t.Run("resets retry count", func(t *testing.T) {
		bc := NewBackoffCalculator(time.Second, time.Minute, 2.0)
		bc.jitterEnabled = false
		
		// Generate some delays
		bc.NextDelay()
		bc.NextDelay()
		
		// Reset
		bc.Reset()
		
		delay := bc.NextDelay()
		if delay != time.Second {
			t.Errorf("Delay after reset should be initial delay %v, got %v", time.Second, delay)
		}
	})
}

// Test weighted rate limiting with bursts and throttling
func TestWeightedRateLimiting_BurstSimulation(t *testing.T) {
	rl := NewTokenBucketRateLimiter()
	
	// Configure realistic Binance-like limits
	limits := &interfaces.RateLimits{
		RequestsPerSecond: 10,
		BurstAllowance:    20,
		WeightLimits: map[string]int{
			"orderbook":      1,
			"trades":         1,
			"klines":         1,
			"account_info":   10,
			"all_tickers":    40,
		},
		DailyLimit:   intPtr(100000),
		MonthlyLimit: intPtr(3000000),
	}
	
	err := rl.UpdateLimits(context.Background(), "binance", limits)
	if err != nil {
		t.Fatalf("UpdateLimits failed: %v", err)
	}
	
	ctx := context.Background()
	
	t.Run("simulate burst requests", func(t *testing.T) {
		var requestsBlocked int
		var requestsAllowed int
		
		// Simulate a burst of 50 requests
		for i := 0; i < 50; i++ {
			err := rl.Allow(ctx, "binance", "orderbook")
			if err != nil {
				requestsBlocked++
			} else {
				requestsAllowed++
			}
		}
		
		if requestsAllowed == 0 {
			t.Error("At least some requests should be allowed during burst")
		}
		
		if requestsBlocked == 0 {
			t.Error("Some requests should be blocked after burst capacity")
		}
		
		t.Logf("Burst test: %d allowed, %d blocked", requestsAllowed, requestsBlocked)
	})
	
	t.Run("verify weight accumulation", func(t *testing.T) {
		// Reset with fresh limiter for clean test
		rlWeight := NewTokenBucketRateLimiter()
		rlWeight.UpdateLimits(ctx, "binance_weight", limits)
		
		// Should allow lightweight requests
		for i := 0; i < 10; i++ {
			err := rlWeight.Allow(ctx, "binance_weight", "orderbook")
			if err != nil {
				t.Errorf("Lightweight request %d should be allowed: %v", i, err)
			}
		}
		
		// Heavy request should eventually be throttled
		var heavyBlocked bool
		for i := 0; i < 5; i++ {
			err := rlWeight.Allow(ctx, "binance_weight", "all_tickers")
			if err != nil {
				heavyBlocked = true
				break
			}
		}
		
		if !heavyBlocked {
			t.Error("Heavy requests should eventually be throttled")
		}
	})
}

// Test sliding window counters for budget tracking
func TestSlidingWindowBudgetTracking(t *testing.T) {
	bt := NewBudgetTracker()
	
	// Set tight daily limits for testing
	dailyLimit := 5
	monthlyLimit := 100
	bt.UpdateLimits("test_venue", &dailyLimit, &monthlyLimit)
	
	t.Run("daily budget enforcement", func(t *testing.T) {
		// Use up the daily budget
		for i := 0; i < dailyLimit; i++ {
			err := bt.CheckBudget("test_venue")
			if err != nil {
				t.Errorf("Request %d should be within daily budget: %v", i, err)
			}
			bt.IncrementUsage("test_venue")
		}
		
		// Next request should be rejected
		err := bt.CheckBudget("test_venue")
		if err == nil {
			t.Error("Request should be rejected due to daily budget exhaustion")
		}
	})
	
	t.Run("budget window reset", func(t *testing.T) {
		// Create counter with very short window
		counter := NewUsageCounter(100 * time.Millisecond)
		
		// Fill the counter
		for i := 0; i < 3; i++ {
			counter.Increment()
		}
		
		if counter.GetCount() != 3 {
			t.Errorf("Counter should have 3, got %d", counter.GetCount())
		}
		
		// Wait for window to expire
		time.Sleep(150 * time.Millisecond)
		
		if counter.GetCount() != 0 {
			t.Errorf("Counter should reset to 0, got %d", counter.GetCount())
		}
		
		// Should accept new increments
		counter.Increment()
		if counter.GetCount() != 1 {
			t.Errorf("Counter should accept new increments after reset, got %d", counter.GetCount())
		}
	})
}

// Test provider-specific header processing
func TestProviderSpecificHeaders(t *testing.T) {
	rl := NewTokenBucketRateLimiter()
	
	limits := &interfaces.RateLimits{
		RequestsPerSecond: 10,
		BurstAllowance:    5,
		WeightLimits:      make(map[string]int),
		DailyLimit:        intPtr(1000),
		MonthlyLimit:      intPtr(30000),
	}
	
	ctx := context.Background()
	
	vendors := []string{"binance", "okx", "coinbase", "kraken"}
	for _, venue := range vendors {
		rl.UpdateLimits(ctx, venue, limits)
	}
	
	t.Run("binance weight headers", func(t *testing.T) {
		headers := map[string]string{
			"X-MBX-USED-WEIGHT": "800",
			"X-MBX-ORDER-COUNT": "50",
		}
		
		err := rl.ProcessRateLimitHeaders("binance", headers)
		if err != nil {
			t.Errorf("Processing Binance headers failed: %v", err)
		}
	})
	
	t.Run("okx rate limit headers", func(t *testing.T) {
		headers := map[string]string{
			"ratelimit-remaining": "0",
			"ratelimit-reset":     fmt.Sprintf("%d", time.Now().Add(30*time.Second).UnixMilli()),
		}
		
		err := rl.ProcessRateLimitHeaders("okx", headers)
		if err != nil {
			t.Errorf("Processing OKX headers failed: %v", err)
		}
		
		// Request should be blocked due to rate limit
		err = rl.Allow(ctx, "okx", "test")
		if err == nil {
			t.Error("Request should be blocked when ratelimit-remaining is 0")
		}
	})
	
	t.Run("generic retry-after header", func(t *testing.T) {
		headers := map[string]string{
			"Retry-After": "2", // 2 seconds
		}
		
		err := rl.ProcessRateLimitHeaders("coinbase", headers)
		if err != nil {
			t.Errorf("Processing Coinbase headers failed: %v", err)
		}
		
		// Request should be blocked immediately
		err = rl.Allow(ctx, "coinbase", "test")
		if err == nil {
			t.Error("Request should be blocked due to Retry-After header")
		}
	})
}

// Test concurrent access to rate limiter
func TestRateLimiterConcurrency(t *testing.T) {
	rl := NewTokenBucketRateLimiter()
	
	limits := &interfaces.RateLimits{
		RequestsPerSecond: 100,
		BurstAllowance:    50,
		WeightLimits: map[string]int{
			"light": 1,
			"heavy": 5,
		},
		DailyLimit:   intPtr(10000),
		MonthlyLimit: intPtr(300000),
	}
	
	rl.UpdateLimits(context.Background(), "concurrent", limits)
	
	t.Run("concurrent requests", func(t *testing.T) {
		var wg sync.WaitGroup
		var allowedCount int64
		var blockedCount int64
		var mu sync.Mutex
		
		// Launch 100 concurrent requests
		for i := 0; i < 100; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				
				ctx := context.Background()
				err := rl.Allow(ctx, "concurrent", "light")
				
				mu.Lock()
				if err != nil {
					blockedCount++
				} else {
					allowedCount++
				}
				mu.Unlock()
			}(i)
		}
		
		wg.Wait()
		
		t.Logf("Concurrent test: %d allowed, %d blocked", allowedCount, blockedCount)
		
		if allowedCount == 0 {
			t.Error("Some requests should be allowed in concurrent test")
		}
	})
}

// Test metrics collection and reporting
func TestRateLimiterMetrics(t *testing.T) {
	rl := NewTokenBucketRateLimiter()
	
	limits := &interfaces.RateLimits{
		RequestsPerSecond: 5,
		BurstAllowance:    3,
		WeightLimits:      make(map[string]int),
		DailyLimit:        intPtr(100),
		MonthlyLimit:      intPtr(3000),
	}
	
	ctx := context.Background()
	rl.UpdateLimits(ctx, "metrics_test", limits)
	
	t.Run("collect blocking metrics", func(t *testing.T) {
		var blockedRequests int
		var allowedRequests int
		
		// Generate requests to trigger blocking
		for i := 0; i < 20; i++ {
			err := rl.Allow(ctx, "metrics_test", "test")
			if err != nil {
				blockedRequests++
			} else {
				allowedRequests++
			}
		}
		
		t.Logf("Metrics collection: %d allowed, %d blocked", allowedRequests, blockedRequests)
		
		// Verify we collected meaningful metrics
		if blockedRequests == 0 {
			t.Error("Should have blocked some requests to test metrics collection")
		}
		
		if allowedRequests == 0 {
			t.Error("Should have allowed some requests to test metrics collection")
		}
	})
	
	t.Run("budget usage metrics", func(t *testing.T) {
		bt := NewBudgetTracker()
		dailyLimit := 10
		bt.UpdateLimits("budget_metrics", &dailyLimit, nil)
		
		// Use some budget
		for i := 0; i < 5; i++ {
			bt.IncrementUsage("budget_metrics")
		}
		
		// Check remaining budget
		err := bt.CheckBudget("budget_metrics")
		if err != nil {
			t.Errorf("Budget check failed: %v", err)
		}
		
		// Should still have budget available
		if daily, exists := bt.dailyUsage["budget_metrics"]; exists {
			usage := daily.GetCount()
			if usage != 5 {
				t.Errorf("Expected usage of 5, got %d", usage)
			}
		}
	})
}

// Helper function to create int pointer
func intPtr(i int) *int {
	return &i
}