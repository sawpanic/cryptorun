package middleware

import (
	"context"
	"fmt"
	"math/rand"
	"strconv"
	"sync"
	"time"

	"github.com/sawpanic/cryptorun/src/infrastructure/datafacade/interfaces"
	
	"golang.org/x/time/rate"
)

// TokenBucketRateLimiter implements rate limiting using token bucket algorithm with sliding window counters
type TokenBucketRateLimiter struct {
	limiters      map[string]*venueRateLimiter
	limits        map[string]*interfaces.RateLimits
	mu            sync.RWMutex
	budgetTracker *BudgetTracker
	metrics       *RateLimiterMetrics
}

type venueRateLimiter struct {
	venue            string
	globalLimiter    *rate.Limiter
	endpointLimiters map[string]*rate.Limiter
	weights          map[string]int
	
	// Enhanced weight tracking with sliding windows
	weightWindow     *SlidingWindow
	maxWeight        int
	windowDuration   time.Duration
	
	// Header tracking
	lastUsedWeight   int
	lastResetTime    time.Time
	retryAfter       time.Time
	
	// Metrics
	requestsAllowed  int64
	requestsBlocked  int64
	
	mu sync.RWMutex
}

// BudgetTracker tracks daily/monthly API usage budgets
type BudgetTracker struct {
	dailyUsage   map[string]*UsageCounter
	monthlyUsage map[string]*UsageCounter
	limits       map[string]*BudgetLimits
	mu           sync.RWMutex
}

type UsageCounter struct {
	count       int64
	windowStart time.Time
	windowSize  time.Duration
}

type BudgetLimits struct {
	dailyLimit   int64
	monthlyLimit int64
}

// NewTokenBucketRateLimiter creates a new enhanced rate limiter
func NewTokenBucketRateLimiter() *TokenBucketRateLimiter {
	return &TokenBucketRateLimiter{
		limiters:      make(map[string]*venueRateLimiter),
		limits:        make(map[string]*interfaces.RateLimits),
		budgetTracker: NewBudgetTracker(),
		metrics:       NewRateLimiterMetrics(),
	}
}

// Allow checks if a request should be allowed
func (rl *TokenBucketRateLimiter) Allow(ctx context.Context, venue, endpoint string) error {
	// Check budget limits first
	if err := rl.budgetTracker.CheckBudget(venue); err != nil {
		return fmt.Errorf("budget exceeded: %w", err)
	}
	
	rl.mu.RLock()
	venueLimiter, exists := rl.limiters[venue]
	rl.mu.RUnlock()
	
	if !exists {
		return fmt.Errorf("no rate limiter configured for venue: %s", venue)
	}
	
	venueLimiter.mu.Lock()
	defer venueLimiter.mu.Unlock()
	
	// Check if we're in a retry-after period
	if time.Now().Before(venueLimiter.retryAfter) {
		return fmt.Errorf("rate limited until %v", venueLimiter.retryAfter)
	}
	
	// Check endpoint-specific limits
	if endpointLimiter, exists := venueLimiter.endpointLimiters[endpoint]; exists {
		if !endpointLimiter.Allow() {
			return fmt.Errorf("endpoint rate limit exceeded for %s", endpoint)
		}
	}
	
	// Check global venue limit
	if !venueLimiter.globalLimiter.Allow() {
		return fmt.Errorf("venue rate limit exceeded for %s", venue)
	}
	
	// Check weight limits (for venues like Binance)
	if weight, exists := venueLimiter.weights[endpoint]; exists {
		if venueLimiter.lastUsedWeight+weight > rl.getWeightLimit(venue) {
			return fmt.Errorf("weight limit exceeded for %s", venue)
		}
		venueLimiter.lastUsedWeight += weight
	}
	
	// Update budget tracker
	rl.budgetTracker.IncrementUsage(venue)
	
	return nil
}

// GetLimits returns current rate limits for a venue
func (rl *TokenBucketRateLimiter) GetLimits(ctx context.Context, venue string) (*interfaces.RateLimits, error) {
	rl.mu.RLock()
	defer rl.mu.RUnlock()
	
	limits, exists := rl.limits[venue]
	if !exists {
		return nil, fmt.Errorf("no limits configured for venue: %s", venue)
	}
	
	// Return a copy
	limitsCopy := *limits
	return &limitsCopy, nil
}

// UpdateLimits updates rate limits for a venue
func (rl *TokenBucketRateLimiter) UpdateLimits(ctx context.Context, venue string, limits *interfaces.RateLimits) error {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	
	rl.limits[venue] = limits
	
	// Update or create venue limiter
	venueLimiter := &venueRateLimiter{
		venue:            venue,
		globalLimiter:    rate.NewLimiter(rate.Limit(limits.RequestsPerSecond), limits.BurstAllowance),
		endpointLimiters: make(map[string]*rate.Limiter),
		weights:          limits.WeightLimits,
	}
	
	// Set up endpoint limiters if specified
	for endpoint, weight := range limits.WeightLimits {
		// Convert weight to requests per second (simplified)
		rps := float64(limits.RequestsPerSecond) / float64(weight)
		venueLimiter.endpointLimiters[endpoint] = rate.NewLimiter(rate.Limit(rps), 1)
	}
	
	rl.limiters[venue] = venueLimiter
	
	// Update budget limits
	rl.budgetTracker.UpdateLimits(venue, limits.DailyLimit, limits.MonthlyLimit)
	
	return nil
}

// ProcessRateLimitHeaders processes exchange-specific rate limit headers
func (rl *TokenBucketRateLimiter) ProcessRateLimitHeaders(venue string, headers map[string]string) error {
	rl.mu.RLock()
	venueLimiter, exists := rl.limiters[venue]
	rl.mu.RUnlock()
	
	if !exists {
		return fmt.Errorf("no rate limiter configured for venue: %s", venue)
	}
	
	venueLimiter.mu.Lock()
	defer venueLimiter.mu.Unlock()
	
	switch venue {
	case "binance":
		return rl.processBinanceHeaders(venueLimiter, headers)
	case "okx":
		return rl.processOKXHeaders(venueLimiter, headers)
	case "coinbase":
		return rl.processCoinbaseHeaders(venueLimiter, headers)
	case "kraken":
		return rl.processKrakenHeaders(venueLimiter, headers)
	default:
		return rl.processGenericHeaders(venueLimiter, headers)
	}
}

// Venue-specific header processing

func (rl *TokenBucketRateLimiter) processBinanceHeaders(vl *venueRateLimiter, headers map[string]string) error {
	// Process X-MBX-USED-WEIGHT header
	if usedWeight, exists := headers["X-MBX-USED-WEIGHT"]; exists {
		if weight, err := strconv.Atoi(usedWeight); err == nil {
			vl.lastUsedWeight = weight
		}
	}
	
	// Process Retry-After header
	if retryAfter, exists := headers["Retry-After"]; exists {
		if seconds, err := strconv.Atoi(retryAfter); err == nil {
			vl.retryAfter = time.Now().Add(time.Duration(seconds) * time.Second)
		}
	}
	
	return nil
}

func (rl *TokenBucketRateLimiter) processOKXHeaders(vl *venueRateLimiter, headers map[string]string) error {
	// Process ratelimit-remaining header
	if remaining, exists := headers["ratelimit-remaining"]; exists {
		if rem, err := strconv.Atoi(remaining); err == nil && rem == 0 {
			// If no requests remaining, check reset time
			if resetTime, exists := headers["ratelimit-reset"]; exists {
				if resetMs, err := strconv.ParseInt(resetTime, 10, 64); err == nil {
					vl.retryAfter = time.Unix(0, resetMs*1000000) // Convert ms to nanoseconds
				}
			}
		}
	}
	
	// Process Retry-After header
	if retryAfter, exists := headers["Retry-After"]; exists {
		if seconds, err := strconv.Atoi(retryAfter); err == nil {
			vl.retryAfter = time.Now().Add(time.Duration(seconds) * time.Second)
		}
	}
	
	return nil
}

func (rl *TokenBucketRateLimiter) processCoinbaseHeaders(vl *venueRateLimiter, headers map[string]string) error {
	// Process Retry-After header
	if retryAfter, exists := headers["Retry-After"]; exists {
		if seconds, err := strconv.Atoi(retryAfter); err == nil {
			vl.retryAfter = time.Now().Add(time.Duration(seconds) * time.Second)
		}
	}
	
	return nil
}

func (rl *TokenBucketRateLimiter) processKrakenHeaders(vl *venueRateLimiter, headers map[string]string) error {
	// Kraken uses API counter system
	// Process Retry-After header
	if retryAfter, exists := headers["Retry-After"]; exists {
		if seconds, err := strconv.Atoi(retryAfter); err == nil {
			vl.retryAfter = time.Now().Add(time.Duration(seconds) * time.Second)
		}
	}
	
	return nil
}

func (rl *TokenBucketRateLimiter) processGenericHeaders(vl *venueRateLimiter, headers map[string]string) error {
	// Process standard rate limit headers
	if remaining, exists := headers["X-RateLimit-Remaining"]; exists {
		if rem, err := strconv.Atoi(remaining); err == nil && rem == 0 {
			if resetTime, exists := headers["X-RateLimit-Reset"]; exists {
				if reset, err := strconv.ParseInt(resetTime, 10, 64); err == nil {
					vl.retryAfter = time.Unix(reset, 0)
				}
			}
		}
	}
	
	if retryAfter, exists := headers["Retry-After"]; exists {
		if seconds, err := strconv.Atoi(retryAfter); err == nil {
			vl.retryAfter = time.Now().Add(time.Duration(seconds) * time.Second)
		}
	}
	
	return nil
}

// Helper methods

func (rl *TokenBucketRateLimiter) getWeightLimit(venue string) int {
	// Default weight limits per venue
	limits := map[string]int{
		"binance":  1200, // Binance weight limit per minute
		"okx":      600,
		"coinbase": 300,
		"kraken":   15,
	}
	
	if limit, exists := limits[venue]; exists {
		return limit
	}
	
	return 100 // Default
}

// BudgetTracker implementation

func NewBudgetTracker() *BudgetTracker {
	return &BudgetTracker{
		dailyUsage:   make(map[string]*UsageCounter),
		monthlyUsage: make(map[string]*UsageCounter),
		limits:       make(map[string]*BudgetLimits),
	}
}

func (bt *BudgetTracker) CheckBudget(venue string) error {
	bt.mu.RLock()
	defer bt.mu.RUnlock()
	
	limits, exists := bt.limits[venue]
	if !exists {
		return nil // No limits configured
	}
	
	// Check daily budget
	if daily, exists := bt.dailyUsage[venue]; exists {
		if daily.GetCount() >= limits.dailyLimit {
			return fmt.Errorf("daily budget exceeded for %s: %d/%d", venue, daily.GetCount(), limits.dailyLimit)
		}
	}
	
	// Check monthly budget
	if monthly, exists := bt.monthlyUsage[venue]; exists {
		if monthly.GetCount() >= limits.monthlyLimit {
			return fmt.Errorf("monthly budget exceeded for %s: %d/%d", venue, monthly.GetCount(), limits.monthlyLimit)
		}
	}
	
	return nil
}

func (bt *BudgetTracker) IncrementUsage(venue string) {
	bt.mu.Lock()
	defer bt.mu.Unlock()
	
	// Increment daily counter
	if daily, exists := bt.dailyUsage[venue]; exists {
		daily.Increment()
	} else {
		bt.dailyUsage[venue] = NewUsageCounter(24 * time.Hour)
		bt.dailyUsage[venue].Increment()
	}
	
	// Increment monthly counter
	if monthly, exists := bt.monthlyUsage[venue]; exists {
		monthly.Increment()
	} else {
		bt.monthlyUsage[venue] = NewUsageCounter(30 * 24 * time.Hour)
		bt.monthlyUsage[venue].Increment()
	}
}

func (bt *BudgetTracker) UpdateLimits(venue string, dailyLimit, monthlyLimit *int) {
	bt.mu.Lock()
	defer bt.mu.Unlock()
	
	limits := &BudgetLimits{}
	
	if dailyLimit != nil {
		limits.dailyLimit = int64(*dailyLimit)
	}
	
	if monthlyLimit != nil {
		limits.monthlyLimit = int64(*monthlyLimit)
	}
	
	bt.limits[venue] = limits
}

// UsageCounter implementation

func NewUsageCounter(windowSize time.Duration) *UsageCounter {
	return &UsageCounter{
		count:       0,
		windowStart: time.Now(),
		windowSize:  windowSize,
	}
}

func (uc *UsageCounter) Increment() {
	// Check if window has expired
	if time.Since(uc.windowStart) > uc.windowSize {
		uc.count = 0
		uc.windowStart = time.Now()
	}
	
	uc.count++
}

func (uc *UsageCounter) GetCount() int64 {
	// Check if window has expired
	if time.Since(uc.windowStart) > uc.windowSize {
		return 0
	}
	
	return uc.count
}

// BackoffCalculator implements exponential backoff
type BackoffCalculator struct {
	initialDelay   time.Duration
	maxDelay       time.Duration
	multiplier     float64
	jitterEnabled  bool
	retryCount     int
}

func NewBackoffCalculator(initial, max time.Duration, multiplier float64) *BackoffCalculator {
	return &BackoffCalculator{
		initialDelay:  initial,
		maxDelay:      max,
		multiplier:    multiplier,
		jitterEnabled: true,
		retryCount:    0,
	}
}

func (bc *BackoffCalculator) NextDelay() time.Duration {
	delay := time.Duration(float64(bc.initialDelay) * pow(bc.multiplier, float64(bc.retryCount)))
	
	if delay > bc.maxDelay {
		delay = bc.maxDelay
	}
	
	if bc.jitterEnabled {
		// Add up to 25% jitter
		jitter := time.Duration(float64(delay) * 0.25 * rand.Float64())
		delay += jitter
	}
	
	bc.retryCount++
	return delay
}

func (bc *BackoffCalculator) Reset() {
	bc.retryCount = 0
}

// Simple power function
func pow(base, exp float64) float64 {
	if exp == 0 {
		return 1
	}
	result := base
	for i := 1; i < int(exp); i++ {
		result *= base
	}
	return result
}

func init() {
	rand.Seed(time.Now().UnixNano())
}