package middleware

import (
	"context"
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

// Helper function to create int pointer
func intPtr(i int) *int {
	return &i
}