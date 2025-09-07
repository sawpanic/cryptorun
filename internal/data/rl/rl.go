package rl

import (
	"strconv"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
	"cryptorun/internal/data/facade"
)

// RateLimiter implements facade.RateLimiter with venue-specific rate limiting
// and budget guards for free-tier API usage
type RateLimiter struct {
	mu       sync.RWMutex
	venues   map[string]*venueState
	budgets  map[string]*budgetGuard
}

type venueState struct {
	lastRequest   time.Time
	requestCount  int64
	resetTime     time.Time
	throttled     bool
	backoffUntil  time.Time
	weightUsed    int64
	weightLimit   int64
	
	// Configuration
	maxRPS        int64
	burstSize     int64
	backoffBase   time.Duration
	backoffMultiplier float64
}

type budgetGuard struct {
	venue         string
	monthlyLimit  int64
	used          int64
	resetDate     time.Time
	warningThreshold int64
	fallbackMode  bool
}

// NewRateLimiter creates a new rate limiter with default configurations
func NewRateLimiter() *RateLimiter {
	rl := &RateLimiter{
		venues:  make(map[string]*venueState),
		budgets: make(map[string]*budgetGuard),
	}
	
	// Initialize default venue configurations
	rl.initializeDefaults()
	
	return rl
}

// Allow checks if a request to the venue is allowed
func (rl *RateLimiter) Allow(venue string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	
	state := rl.getVenueState(venue)
	budget := rl.getBudgetGuard(venue)
	
	now := time.Now()
	
	// Check if we're in backoff period
	if state.throttled && now.Before(state.backoffUntil) {
		log.Debug().Str("venue", venue).Time("backoff_until", state.backoffUntil).
			Msg("Request blocked - in backoff period")
		return false
	}
	
	// Check budget guard
	if budget.fallbackMode {
		log.Debug().Str("venue", venue).Msg("Request blocked - budget guard fallback mode")
		return false
	}
	
	// Check rate limit
	if rl.checkRateLimit(state, now) {
		state.lastRequest = now
		state.requestCount++
		budget.used++
		
		// Clear throttling if we were previously throttled
		if state.throttled && now.After(state.backoffUntil) {
			state.throttled = false
			log.Info().Str("venue", venue).Msg("Rate limiting backoff cleared")
		}
		
		return true
	}
	
	log.Debug().Str("venue", venue).Msg("Request blocked - rate limit exceeded")
	return false
}

// Wait returns the duration to wait before the next request is allowed
func (rl *RateLimiter) Wait(venue string) time.Duration {
	rl.mu.RLock()
	defer rl.mu.RUnlock()
	
	state := rl.getVenueState(venue)
	now := time.Now()
	
	// If in backoff, return backoff time
	if state.throttled && now.Before(state.backoffUntil) {
		return state.backoffUntil.Sub(now)
	}
	
	// Calculate wait time based on rate limit
	timeSinceLastRequest := now.Sub(state.lastRequest)
	minInterval := time.Second / time.Duration(state.maxRPS)
	
	if timeSinceLastRequest < minInterval {
		return minInterval - timeSinceLastRequest
	}
	
	return 0
}

// UpdateBudget updates the remaining budget for a venue
// Used when parsing response headers like X-MBX-USED-WEIGHT
func (rl *RateLimiter) UpdateBudget(venue string, remaining int64) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	
	state := rl.getVenueState(venue)
	budget := rl.getBudgetGuard(venue)
	
	// Update weight tracking
	if remaining > 0 {
		state.weightUsed = state.weightLimit - remaining
	}
	
	// Check if we're approaching limits
	usageRatio := float64(budget.used) / float64(budget.monthlyLimit)
	
	if usageRatio > 0.9 && !budget.fallbackMode {
		budget.fallbackMode = true
		log.Warn().Str("venue", venue).Float64("usage_ratio", usageRatio).
			Msg("Budget guard activated - entering fallback mode")
	} else if usageRatio > 0.8 {
		log.Warn().Str("venue", venue).Float64("usage_ratio", usageRatio).
			Msg("Budget warning - approaching monthly limit")
	}
}

// HandleRateLimitResponse processes rate limit response headers and triggers backoff
func (rl *RateLimiter) HandleRateLimitResponse(venue string, headers map[string]string) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	
	state := rl.getVenueState(venue)
	
	// Parse Binance-style headers
	if weightUsedStr, exists := headers["X-MBX-USED-WEIGHT"]; exists {
		if weightUsed, err := strconv.ParseInt(weightUsedStr, 10, 64); err == nil {
			state.weightUsed = weightUsed
		}
	}
	
	if weightLimitStr, exists := headers["X-MBX-ORDER-COUNT-1M"]; exists {
		if weightLimit, err := strconv.ParseInt(weightLimitStr, 10, 64); err == nil {
			state.weightLimit = weightLimit
		}
	}
	
	// Parse retry-after header
	if retryAfterStr, exists := headers["Retry-After"]; exists {
		if retryAfter, err := strconv.ParseInt(retryAfterStr, 10, 64); err == nil {
			rl.triggerBackoff(state, time.Duration(retryAfter)*time.Second)
		}
	}
}

// Handle429Response triggers exponential backoff for 429 responses
func (rl *RateLimiter) Handle429Response(venue string) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	
	state := rl.getVenueState(venue)
	
	// Calculate exponential backoff
	backoffDuration := state.backoffBase
	if state.throttled {
		backoffDuration = time.Duration(float64(backoffDuration) * state.backoffMultiplier)
		if backoffDuration > 5*time.Minute {
			backoffDuration = 5 * time.Minute // Cap at 5 minutes
		}
	}
	
	rl.triggerBackoff(state, backoffDuration)
	
	log.Warn().Str("venue", venue).Dur("backoff", backoffDuration).
		Msg("429 response - exponential backoff triggered")
}

// Status returns the current rate limit status for a venue
func (rl *RateLimiter) Status(venue string) facade.RateLimitStatus {
	rl.mu.RLock()
	defer rl.mu.RUnlock()
	
	state := rl.getVenueState(venue)
	budget := rl.getBudgetGuard(venue)
	
	backoffTime := time.Duration(0)
	if state.throttled && time.Now().Before(state.backoffUntil) {
		backoffTime = state.backoffUntil.Sub(time.Now())
	}
	
	return facade.RateLimitStatus{
		Venue:       venue,
		Remaining:   budget.monthlyLimit - budget.used,
		ResetTime:   budget.resetDate,
		Throttled:   state.throttled,
		BackoffTime: backoffTime,
	}
}

// Helper methods

func (rl *RateLimiter) initializeDefaults() {
	// Binance free tier: 1200 requests per minute, 10M monthly
	rl.venues["binance"] = &venueState{
		maxRPS:            20,  // 1200/60
		burstSize:         100,
		backoffBase:       5 * time.Second,
		backoffMultiplier: 2.0,
		weightLimit:       1200,
	}
	
	rl.budgets["binance"] = &budgetGuard{
		venue:            "binance",
		monthlyLimit:     10000000,
		warningThreshold: 8000000,
		resetDate:        getNextMonthReset(),
	}
	
	// OKX: 20 requests per 2 seconds per endpoint
	rl.venues["okx"] = &venueState{
		maxRPS:            10,
		burstSize:         20,
		backoffBase:       3 * time.Second,
		backoffMultiplier: 1.5,
		weightLimit:       100,
	}
	
	rl.budgets["okx"] = &budgetGuard{
		venue:            "okx",
		monthlyLimit:     5000000,
		warningThreshold: 4000000,
		resetDate:        getNextMonthReset(),
	}
	
	// Coinbase Pro: 10 requests per second
	rl.venues["coinbase"] = &venueState{
		maxRPS:            10,
		burstSize:         50,
		backoffBase:       2 * time.Second,
		backoffMultiplier: 1.8,
		weightLimit:       1000,
	}
	
	rl.budgets["coinbase"] = &budgetGuard{
		venue:            "coinbase",
		monthlyLimit:     3000000,
		warningThreshold: 2400000,
		resetDate:        getNextMonthReset(),
	}
	
	// Kraken: 1 request per second (conservative)
	rl.venues["kraken"] = &venueState{
		maxRPS:            1,
		burstSize:         5,
		backoffBase:       10 * time.Second,
		backoffMultiplier: 2.5,
		weightLimit:       60, // 1 per second * 60 seconds
	}
	
	rl.budgets["kraken"] = &budgetGuard{
		venue:            "kraken",
		monthlyLimit:     1000000,
		warningThreshold: 800000,
		resetDate:        getNextMonthReset(),
	}
}

func (rl *RateLimiter) getVenueState(venue string) *venueState {
	state, exists := rl.venues[venue]
	if !exists {
		// Create default state for unknown venues
		state = &venueState{
			maxRPS:            5,
			burstSize:         10,
			backoffBase:       5 * time.Second,
			backoffMultiplier: 2.0,
			weightLimit:       1000,
		}
		rl.venues[venue] = state
	}
	return state
}

func (rl *RateLimiter) getBudgetGuard(venue string) *budgetGuard {
	budget, exists := rl.budgets[venue]
	if !exists {
		budget = &budgetGuard{
			venue:            venue,
			monthlyLimit:     100000,
			warningThreshold: 80000,
			resetDate:        getNextMonthReset(),
		}
		rl.budgets[venue] = budget
	}
	return budget
}

func (rl *RateLimiter) checkRateLimit(state *venueState, now time.Time) bool {
	// Reset window if needed
	if now.Sub(state.lastRequest) > time.Minute {
		state.requestCount = 0
		state.resetTime = now.Add(time.Minute)
	}
	
	// Check if we're within rate limit
	allowedRequests := state.maxRPS * 60 // per minute
	return state.requestCount < allowedRequests
}

func (rl *RateLimiter) triggerBackoff(state *venueState, duration time.Duration) {
	state.throttled = true
	state.backoffUntil = time.Now().Add(duration)
	state.backoffBase = duration
}

func getNextMonthReset() time.Time {
	now := time.Now()
	year, month, _ := now.Date()
	if month == 12 {
		return time.Date(year+1, 1, 1, 0, 0, 0, 0, now.Location())
	}
	return time.Date(year, month+1, 1, 0, 0, 0, 0, now.Location())
}