package kraken

import (
	"context"
	"sync"
	"time"
)

// RateLimiter implements token bucket rate limiting for Kraken API
type RateLimiter struct {
	rps          float64
	tokens       float64
	maxTokens    float64
	lastRefill   time.Time
	mu           sync.Mutex
	lastRequest  time.Time
}

// NewRateLimiter creates a new rate limiter with specified requests per second
func NewRateLimiter(rps float64) *RateLimiter {
	if rps <= 0 {
		rps = 1.0 // Default to 1 RPS for safety
	}
	
	return &RateLimiter{
		rps:        rps,
		tokens:     rps,        // Start with full bucket
		maxTokens:  rps * 2,    // Allow burst up to 2x RPS
		lastRefill: time.Now(),
	}
}

// Wait blocks until a token is available, respecting rate limits
func (rl *RateLimiter) Wait(ctx context.Context) error {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	
	// Refill tokens based on elapsed time
	now := time.Now()
	elapsed := now.Sub(rl.lastRefill).Seconds()
	
	// Add tokens based on elapsed time and RPS
	tokensToAdd := elapsed * rl.rps
	rl.tokens = min(rl.tokens+tokensToAdd, rl.maxTokens)
	rl.lastRefill = now
	
	// If we have tokens available, consume one and return
	if rl.tokens >= 1.0 {
		rl.tokens -= 1.0
		rl.lastRequest = now
		return nil
	}
	
	// Calculate wait time needed
	waitTime := time.Duration((1.0-rl.tokens)/rl.rps*1000) * time.Millisecond
	
	// Release lock during wait
	rl.mu.Unlock()
	
	// Wait for either timeout or required duration
	select {
	case <-ctx.Done():
		rl.mu.Lock() // Re-acquire lock before returning
		return ctx.Err()
	case <-time.After(waitTime):
		// Continue after wait
	}
	
	// Re-acquire lock and consume token
	rl.mu.Lock()
	rl.tokens = max(0, rl.tokens-1.0)
	rl.lastRequest = time.Now()
	
	return nil
}

// TryWait attempts to acquire a token without blocking
func (rl *RateLimiter) TryWait() bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	
	// Refill tokens
	now := time.Now()
	elapsed := now.Sub(rl.lastRefill).Seconds()
	tokensToAdd := elapsed * rl.rps
	rl.tokens = min(rl.tokens+tokensToAdd, rl.maxTokens)
	rl.lastRefill = now
	
	// Check if token is available
	if rl.tokens >= 1.0 {
		rl.tokens -= 1.0
		rl.lastRequest = now
		return true
	}
	
	return false
}

// Remaining returns the number of tokens currently available
func (rl *RateLimiter) Remaining() float64 {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	
	// Refill tokens before checking
	now := time.Now()
	elapsed := now.Sub(rl.lastRefill).Seconds()
	tokensToAdd := elapsed * rl.rps
	rl.tokens = min(rl.tokens+tokensToAdd, rl.maxTokens)
	rl.lastRefill = now
	
	return rl.tokens
}

// LastRequest returns the time of the last successful request
func (rl *RateLimiter) LastRequest() time.Time {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	return rl.lastRequest
}

// SetRPS updates the rate limit (useful for dynamic adjustment)
func (rl *RateLimiter) SetRPS(rps float64) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	
	if rps <= 0 {
		return // Invalid RPS
	}
	
	rl.rps = rps
	rl.maxTokens = rps * 2 // Update burst capacity
	
	// Ensure current tokens don't exceed new max
	rl.tokens = min(rl.tokens, rl.maxTokens)
}

// RPS returns the current rate limit
func (rl *RateLimiter) RPS() float64 {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	return rl.rps
}

// Reset resets the rate limiter state
func (rl *RateLimiter) Reset() {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	
	rl.tokens = rl.rps
	rl.lastRefill = time.Now()
	rl.lastRequest = time.Time{}
}

// Helper functions for min/max operations
func min(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

func max(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}