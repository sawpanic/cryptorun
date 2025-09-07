package unit

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/sawpanic/cryptorun/internal/data/rl"
)

func TestRateLimiterHeaders(t *testing.T) {
	limiter := rl.NewRateLimiter()
	
	// Test header parsing
	testCases := []struct {
		name           string
		headers        map[string]string
		expectRemaining int64
		expectReset     time.Time
		expectError    bool
	}{
		{
			name: "valid_headers",
			headers: map[string]string{
				"X-Ratelimit-Remaining": "1000",
				"X-Ratelimit-Reset":     "1609459200", // Unix timestamp
			},
			expectRemaining: 1000,
			expectReset:     time.Unix(1609459200, 0),
			expectError:     false,
		},
		{
			name: "missing_headers",
			headers: map[string]string{},
			expectError: true,
		},
		{
			name: "invalid_remaining",
			headers: map[string]string{
				"X-Ratelimit-Remaining": "invalid",
				"X-Ratelimit-Reset":     "1609459200",
			},
			expectError: true,
		},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create HTTP response with headers
			resp := &http.Response{
				Header: make(http.Header),
			}
			for k, v := range tc.headers {
				resp.Header.Set(k, v)
			}
			
			remaining, reset, err := limiter.ParseHeaders(resp)
			
			if tc.expectError {
				if err == nil {
					t.Fatal("Expected error but got none")
				}
				return
			}
			
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
			
			if remaining != tc.expectRemaining {
				t.Errorf("Expected remaining %d, got %d", tc.expectRemaining, remaining)
			}
			
			if !reset.Equal(tc.expectReset) {
				t.Errorf("Expected reset %v, got %v", tc.expectReset, reset)
			}
		})
	}
}

func TestBudgetGuard(t *testing.T) {
	limiter := rl.NewRateLimiter()
	
	// Test budget allocation
	venue := "kraken"
	budget := int64(1000)
	
	err := limiter.SetBudget(venue, budget)
	if err != nil {
		t.Fatalf("Failed to set budget: %v", err)
	}
	
	// Test consumption
	testCases := []struct {
		name         string
		cost         int64
		expectAllow  bool
		expectRemain int64
	}{
		{"normal_request", 10, true, 990},
		{"large_request", 500, true, 490},
		{"exceeding_request", 600, false, 490}, // Should fail
		{"small_after_fail", 5, true, 485},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			allowed, remaining := limiter.CheckBudget(venue, tc.cost)
			
			if allowed != tc.expectAllow {
				t.Errorf("Expected allowed=%v, got %v", tc.expectAllow, allowed)
			}
			
			if allowed && remaining != tc.expectRemain {
				t.Errorf("Expected remaining=%d, got %d", tc.expectRemain, remaining)
			}
		})
	}
}

func TestBackoffStrategy(t *testing.T) {
	limiter := rl.NewRateLimiter()
	venue := "kraken"
	
	// Test exponential backoff
	delays := make([]time.Duration, 5)
	for i := 0; i < 5; i++ {
		delay := limiter.GetBackoff(venue, i)
		delays[i] = delay
		
		// Each delay should be roughly 2x the previous
		if i > 0 {
			ratio := float64(delay) / float64(delays[i-1])
			if ratio < 1.5 || ratio > 2.5 {
				t.Errorf("Backoff ratio %f not exponential at attempt %d", ratio, i)
			}
		}
	}
	
	// First delay should be reasonable (1-2 seconds)
	if delays[0] < 500*time.Millisecond || delays[0] > 2*time.Second {
		t.Errorf("First delay %v not in expected range", delays[0])
	}
	
	// Reset backoff
	limiter.ResetBackoff(venue)
	resetDelay := limiter.GetBackoff(venue, 0)
	if resetDelay != delays[0] {
		t.Errorf("Backoff not reset properly: expected %v, got %v", delays[0], resetDelay)
	}
}

func TestVenueIsolation(t *testing.T) {
	limiter := rl.NewRateLimiter()
	
	venues := []string{"kraken", "binance", "okx"}
	budgets := []int64{1000, 2000, 1500}
	
	// Set different budgets for each venue
	for i, venue := range venues {
		err := limiter.SetBudget(venue, budgets[i])
		if err != nil {
			t.Fatalf("Failed to set budget for %s: %v", venue, err)
		}
	}
	
	// Consume from one venue shouldn't affect others
	allowed, remaining := limiter.CheckBudget("kraken", 500)
	if !allowed || remaining != 500 {
		t.Errorf("Kraken consumption failed: allowed=%v, remaining=%d", allowed, remaining)
	}
	
	// Other venues should be unaffected
	allowed, remaining = limiter.CheckBudget("binance", 100)
	if !allowed || remaining != 1900 {
		t.Errorf("Binance unaffected check failed: allowed=%v, remaining=%d", allowed, remaining)
	}
	
	// Backoff isolation
	limiter.GetBackoff("kraken", 3) // High attempt count for kraken
	binanceDelay := limiter.GetBackoff("binance", 0) // First attempt for binance
	krakenDelay := limiter.GetBackoff("kraken", 3)
	
	if binanceDelay >= krakenDelay {
		t.Errorf("Backoff not isolated: binance=%v, kraken=%v", binanceDelay, krakenDelay)
	}
}

func TestRateLimiterStats(t *testing.T) {
	limiter := rl.NewRateLimiter()
	venue := "kraken"
	
	limiter.SetBudget(venue, 1000)
	
	// Generate some activity
	limiter.CheckBudget(venue, 100) // Allowed
	limiter.CheckBudget(venue, 200) // Allowed  
	limiter.CheckBudget(venue, 800) // Should be blocked
	
	stats := limiter.GetStats(venue)
	
	if stats.TotalRequests != 3 {
		t.Errorf("Expected 3 total requests, got %d", stats.TotalRequests)
	}
	
	if stats.AllowedRequests != 2 {
		t.Errorf("Expected 2 allowed requests, got %d", stats.AllowedRequests)
	}
	
	if stats.BlockedRequests != 1 {
		t.Errorf("Expected 1 blocked request, got %d", stats.BlockedRequests)
	}
	
	expectedRatio := float64(2) / float64(3)
	if abs(stats.AllowRatio-expectedRatio) > 0.01 {
		t.Errorf("Expected allow ratio %.2f, got %.2f", expectedRatio, stats.AllowRatio)
	}
}

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}