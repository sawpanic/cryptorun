package guards

import (
	"sync"
	"time"
)

// RateLimiter implements a token bucket algorithm for rate limiting
type RateLimiter struct {
	tokens        float64
	maxTokens     float64
	refillRate    float64 // tokens per second
	lastRefill    time.Time
	mutex         sync.Mutex
	burstLimit    int
	sustainedRate float64
}

// NewRateLimiter creates a new rate limiter with token bucket algorithm
func NewRateLimiter(config ProviderConfig) *RateLimiter {
	burstLimit := config.BurstLimit
	if burstLimit <= 0 {
		burstLimit = 10 // Default burst of 10 requests
	}

	sustainedRate := config.SustainedRate
	if sustainedRate <= 0 {
		sustainedRate = 1.0 // Default 1 request per second
	}

	return &RateLimiter{
		tokens:        float64(burstLimit),
		maxTokens:     float64(burstLimit),
		refillRate:    sustainedRate,
		lastRefill:    time.Now(),
		burstLimit:    burstLimit,
		sustainedRate: sustainedRate,
	}
}

// Allow checks if a request can proceed based on available tokens
func (rl *RateLimiter) Allow() bool {
	rl.mutex.Lock()
	defer rl.mutex.Unlock()

	now := time.Now()
	rl.refillTokens(now)

	if rl.tokens >= 1.0 {
		rl.tokens -= 1.0
		return true
	}

	return false
}

// AllowN checks if N requests can proceed
func (rl *RateLimiter) AllowN(n int) bool {
	if n <= 0 {
		return true
	}

	rl.mutex.Lock()
	defer rl.mutex.Unlock()

	now := time.Now()
	rl.refillTokens(now)

	tokensNeeded := float64(n)
	if rl.tokens >= tokensNeeded {
		rl.tokens -= tokensNeeded
		return true
	}

	return false
}

// Reserve reserves a token and returns the time to wait if not immediately available
func (rl *RateLimiter) Reserve() time.Duration {
	rl.mutex.Lock()
	defer rl.mutex.Unlock()

	now := time.Now()
	rl.refillTokens(now)

	if rl.tokens >= 1.0 {
		rl.tokens -= 1.0
		return 0
	}

	// Calculate wait time for next token
	tokensNeeded := 1.0 - rl.tokens
	waitTime := time.Duration(tokensNeeded/rl.refillRate) * time.Second

	// Consume the future token
	rl.tokens = 0

	return waitTime
}

// refillTokens adds tokens based on elapsed time (must be called with lock held)
func (rl *RateLimiter) refillTokens(now time.Time) {
	elapsed := now.Sub(rl.lastRefill)
	if elapsed <= 0 {
		return
	}

	tokensToAdd := elapsed.Seconds() * rl.refillRate
	rl.tokens += tokensToAdd

	if rl.tokens > rl.maxTokens {
		rl.tokens = rl.maxTokens
	}

	rl.lastRefill = now
}

// Stats returns current rate limiter statistics
func (rl *RateLimiter) Stats() RateLimiterStats {
	rl.mutex.Lock()
	defer rl.mutex.Unlock()

	now := time.Now()
	rl.refillTokens(now)

	return RateLimiterStats{
		CurrentTokens: rl.tokens,
		MaxTokens:     rl.maxTokens,
		RefillRate:    rl.refillRate,
		BurstLimit:    rl.burstLimit,
		SustainedRate: rl.sustainedRate,
		LastRefill:    rl.lastRefill,
	}
}

// RateLimiterStats represents rate limiter statistics
type RateLimiterStats struct {
	CurrentTokens float64   `json:"current_tokens"`
	MaxTokens     float64   `json:"max_tokens"`
	RefillRate    float64   `json:"refill_rate"`
	BurstLimit    int       `json:"burst_limit"`
	SustainedRate float64   `json:"sustained_rate"`
	LastRefill    time.Time `json:"last_refill"`
}

// Reset resets the rate limiter to full token capacity
func (rl *RateLimiter) Reset() {
	rl.mutex.Lock()
	defer rl.mutex.Unlock()

	rl.tokens = rl.maxTokens
	rl.lastRefill = time.Now()
}

// SetRate updates the sustained rate (tokens per second)
func (rl *RateLimiter) SetRate(rate float64) {
	if rate <= 0 {
		return
	}

	rl.mutex.Lock()
	defer rl.mutex.Unlock()

	now := time.Now()
	rl.refillTokens(now)
	rl.refillRate = rate
	rl.sustainedRate = rate
}

// SetBurst updates the burst capacity
func (rl *RateLimiter) SetBurst(burst int) {
	if burst <= 0 {
		return
	}

	rl.mutex.Lock()
	defer rl.mutex.Unlock()

	rl.maxTokens = float64(burst)
	rl.burstLimit = burst

	// Adjust current tokens if over new limit
	if rl.tokens > rl.maxTokens {
		rl.tokens = rl.maxTokens
	}
}

// AvailableTokens returns the number of immediately available tokens
func (rl *RateLimiter) AvailableTokens() float64 {
	rl.mutex.Lock()
	defer rl.mutex.Unlock()

	now := time.Now()
	rl.refillTokens(now)

	return rl.tokens
}

// TimeToNext returns the duration until the next token is available
func (rl *RateLimiter) TimeToNext() time.Duration {
	rl.mutex.Lock()
	defer rl.mutex.Unlock()

	now := time.Now()
	rl.refillTokens(now)

	if rl.tokens >= 1.0 {
		return 0
	}

	tokensNeeded := 1.0 - rl.tokens
	return time.Duration(tokensNeeded/rl.refillRate) * time.Second
}

// MultiProviderRateLimiter manages rate limiters for multiple providers
type MultiProviderRateLimiter struct {
	limiters map[string]*RateLimiter
	mutex    sync.RWMutex
}

// NewMultiProviderRateLimiter creates a rate limiter manager
func NewMultiProviderRateLimiter() *MultiProviderRateLimiter {
	return &MultiProviderRateLimiter{
		limiters: make(map[string]*RateLimiter),
	}
}

// AddProvider adds a rate limiter for a specific provider
func (mrl *MultiProviderRateLimiter) AddProvider(name string, config ProviderConfig) {
	mrl.mutex.Lock()
	defer mrl.mutex.Unlock()

	mrl.limiters[name] = NewRateLimiter(config)
}

// Allow checks if a request can proceed for the given provider
func (mrl *MultiProviderRateLimiter) Allow(provider string) bool {
	mrl.mutex.RLock()
	limiter, exists := mrl.limiters[provider]
	mrl.mutex.RUnlock()

	if !exists {
		return true // No limits configured
	}

	return limiter.Allow()
}

// GetStats returns statistics for all providers
func (mrl *MultiProviderRateLimiter) GetStats() map[string]RateLimiterStats {
	mrl.mutex.RLock()
	defer mrl.mutex.RUnlock()

	stats := make(map[string]RateLimiterStats)
	for name, limiter := range mrl.limiters {
		stats[name] = limiter.Stats()
	}

	return stats
}
