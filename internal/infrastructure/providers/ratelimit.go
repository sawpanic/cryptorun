package providers

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

type RateLimiter struct {
	limiters map[string]*rate.Limiter
	budgets  map[string]*RLBudget
	mutex    sync.RWMutex
}

type RLBudget struct {
	Name          string
	Current       int
	Limit         int
	ResetTime     time.Time
	LastUpdate    time.Time
	WindowMinutes int
}

type WeightHeaders struct {
	Used      int
	Limit     int
	ResetTime time.Time
}

func NewRateLimiter() *RateLimiter {
	return &RateLimiter{
		limiters: make(map[string]*rate.Limiter),
		budgets:  make(map[string]*RLBudget),
	}
}

func (rl *RateLimiter) InitializeProvider(provider string, rps float64, burst int) {
	rl.mutex.Lock()
	defer rl.mutex.Unlock()
	
	rl.limiters[provider] = rate.NewLimiter(rate.Limit(rps), burst)
	rl.budgets[provider] = &RLBudget{
		Name:          provider,
		Current:       0,
		Limit:         int(rps * 60), // Convert to per-minute
		ResetTime:     time.Now().Add(time.Minute),
		LastUpdate:    time.Now(),
		WindowMinutes: 1,
	}
}

func (rl *RateLimiter) Allow(ctx context.Context, provider string) error {
	rl.mutex.RLock()
	limiter, exists := rl.limiters[provider]
	rl.mutex.RUnlock()
	
	if !exists {
		return fmt.Errorf("rate limiter not initialized for provider: %s", provider)
	}
	
	// Check token bucket
	if !limiter.Allow() {
		// Calculate backoff time
		backoffDuration := rl.calculateBackoff(provider)
		select {
		case <-time.After(backoffDuration):
			// Retry after backoff
			if !limiter.Allow() {
				return fmt.Errorf("rate limit exceeded for %s after backoff", provider)
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	
	// Update budget
	rl.updateBudget(provider)
	
	return nil
}

func (rl *RateLimiter) UpdateFromHeaders(provider string, headers map[string]string) {
	rl.mutex.Lock()
	defer rl.mutex.Unlock()
	
	budget := rl.budgets[provider]
	if budget == nil {
		return
	}
	
	// Parse common header formats
	if used, exists := headers["X-RateLimit-Used"]; exists {
		if val, err := strconv.Atoi(used); err == nil {
			budget.Current = val
		}
	}
	
	if limit, exists := headers["X-RateLimit-Limit"]; exists {
		if val, err := strconv.Atoi(limit); err == nil {
			budget.Limit = val
		}
	}
	
	if reset, exists := headers["X-RateLimit-Reset"]; exists {
		if val, err := strconv.ParseInt(reset, 10, 64); err == nil {
			budget.ResetTime = time.Unix(val, 0)
		}
	}
	
	// Handle Retry-After header (429 responses)
	if retryAfter, exists := headers["Retry-After"]; exists {
		if val, err := strconv.Atoi(retryAfter); err == nil {
			// Temporary rate limit adjustment
			newRate := rate.Limit(0.5) // Slow down significantly
			rl.limiters[provider].SetLimit(newRate)
			
			// Schedule reset after retry period
			go func() {
				time.Sleep(time.Duration(val) * time.Second)
				rl.resetProviderRate(provider)
			}()
		}
	}
	
	budget.LastUpdate = time.Now()
}

func (rl *RateLimiter) GetBudgetStatus(provider string) *RLBudget {
	rl.mutex.RLock()
	defer rl.mutex.RUnlock()
	
	budget := rl.budgets[provider]
	if budget == nil {
		return nil
	}
	
	// Return copy to avoid race conditions
	return &RLBudget{
		Name:          budget.Name,
		Current:       budget.Current,
		Limit:         budget.Limit,
		ResetTime:     budget.ResetTime,
		LastUpdate:    budget.LastUpdate,
		WindowMinutes: budget.WindowMinutes,
	}
}

func (rl *RateLimiter) calculateBackoff(provider string) time.Duration {
	budget := rl.budgets[provider]
	if budget == nil {
		return time.Second
	}
	
	utilizationPct := float64(budget.Current) / float64(budget.Limit) * 100
	
	switch {
	case utilizationPct > 90:
		return 30 * time.Second // Heavy backoff near limit
	case utilizationPct > 75:
		return 10 * time.Second // Moderate backoff
	case utilizationPct > 50:
		return 3 * time.Second  // Light backoff
	default:
		return time.Second      // Minimal backoff
	}
}

func (rl *RateLimiter) updateBudget(provider string) {
	budget := rl.budgets[provider]
	if budget == nil {
		return
	}
	
	// Check if budget window has reset
	if time.Now().After(budget.ResetTime) {
		budget.Current = 0
		budget.ResetTime = time.Now().Add(time.Duration(budget.WindowMinutes) * time.Minute)
	}
	
	budget.Current++
	budget.LastUpdate = time.Now()
}

func (rl *RateLimiter) resetProviderRate(provider string) {
	rl.mutex.Lock()
	defer rl.mutex.Unlock()
	
	// Restore original rate limit after temporary throttling
	if limiter, exists := rl.limiters[provider]; exists {
		// Default rates by provider
		defaultRates := map[string]float64{
			"binance":   10.0,
			"kraken":    5.0,
			"coingecko": 3.0,
			"moralis":   2.0,
		}
		
		if defaultRate, exists := defaultRates[provider]; exists {
			limiter.SetLimit(rate.Limit(defaultRate))
		}
	}
}