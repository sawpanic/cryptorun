package runtime

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

// RateLimitConfig defines provider-specific rate limits per v3.2.1
type RateLimitConfig struct {
	Provider        string        `yaml:"provider"`
	RequestsPerMin  int           `yaml:"requests_per_min"`
	RequestsPerHour int           `yaml:"requests_per_hour"`
	DailyRequests   int           `yaml:"daily_requests"`
	MonthlyBudget   int           `yaml:"monthly_budget"`
	WeightLimit     int           `yaml:"weight_limit"` // Binance weight system
	BurstSize       int           `yaml:"burst_size"`
	BackoffBase     time.Duration `yaml:"backoff_base"`
	BackoffMax      time.Duration `yaml:"backoff_max"`
	Headers         []string      `yaml:"headers"` // Headers to monitor
}

// ProviderLimits defines all provider configurations
var ProviderLimits = map[string]RateLimitConfig{
	"binance": {
		Provider:        "binance",
		RequestsPerMin:  1200,
		RequestsPerHour: 7200,
		DailyRequests:   100000,
		MonthlyBudget:   2000000,
		WeightLimit:     1200,
		BurstSize:       10,
		BackoffBase:     time.Second,
		BackoffMax:      time.Minute * 5,
		Headers:         []string{"X-MBX-USED-WEIGHT", "X-MBX-USED-WEIGHT-1M"},
	},
	"dexscreener": {
		Provider:        "dexscreener",
		RequestsPerMin:  300,
		RequestsPerHour: 1800,
		DailyRequests:   25000,
		MonthlyBudget:   500000,
		BurstSize:       5,
		BackoffBase:     time.Second * 2,
		BackoffMax:      time.Minute * 10,
	},
	"coingecko": {
		Provider:        "coingecko",
		RequestsPerMin:  50, // Free tier
		RequestsPerHour: 3000,
		DailyRequests:   10000,
		MonthlyBudget:   300000,
		BurstSize:       3,
		BackoffBase:     time.Second * 3,
		BackoffMax:      time.Minute * 15,
	},
	"moralis": {
		Provider:        "moralis",
		RequestsPerMin:  25, // Free tier
		RequestsPerHour: 1500,
		DailyRequests:   40000,
		MonthlyBudget:   1000000,
		BurstSize:       2,
		BackoffBase:     time.Second * 5,
		BackoffMax:      time.Minute * 20,
	},
	"cmc": {
		Provider:        "cmc",
		RequestsPerMin:  30, // Basic plan
		RequestsPerHour: 1800,
		DailyRequests:   10000,
		MonthlyBudget:   333, // Credit-based
		BurstSize:       2,
		BackoffBase:     time.Second * 4,
		BackoffMax:      time.Minute * 30,
	},
	"etherscan": {
		Provider:        "etherscan",
		RequestsPerMin:  5, // Free tier
		RequestsPerHour: 300,
		DailyRequests:   100000,
		MonthlyBudget:   3000000,
		BurstSize:       1,
		BackoffBase:     time.Second * 10,
		BackoffMax:      time.Minute * 60,
	},
	"paprika": {
		Provider:        "paprika",
		RequestsPerMin:  100,
		RequestsPerHour: 6000,
		DailyRequests:   25000,
		MonthlyBudget:   750000,
		BurstSize:       5,
		BackoffBase:     time.Second * 2,
		BackoffMax:      time.Minute * 8,
	},
}

// RateLimiter manages provider-aware rate limiting with budget tracking
type RateLimiter struct {
	mu              sync.RWMutex
	config          RateLimitConfig
	requestTimes    []time.Time
	hourlyRequests  []time.Time
	dailyCount      int
	monthlyCount    int
	lastDayReset    time.Time
	lastMonthReset  time.Time
	currentWeight   int // Binance weight tracking
	backoffUntil    time.Time
	backoffAttempts int
}

// NewRateLimiter creates a provider-specific rate limiter
func NewRateLimiter(provider string) *RateLimiter {
	config, exists := ProviderLimits[provider]
	if !exists {
		log.Warn().Str("provider", provider).Msg("Unknown provider, using default limits")
		config = RateLimitConfig{
			Provider:        provider,
			RequestsPerMin:  60,
			RequestsPerHour: 3600,
			DailyRequests:   50000,
			MonthlyBudget:   1000000,
			BurstSize:       3,
			BackoffBase:     time.Second * 2,
			BackoffMax:      time.Minute * 5,
		}
	}

	now := time.Now()
	return &RateLimiter{
		config:         config,
		requestTimes:   make([]time.Time, 0, config.BurstSize*2),
		hourlyRequests: make([]time.Time, 0, 100),
		lastDayReset:   now,
		lastMonthReset: now,
	}
}

// CheckLimit verifies if request can proceed under current limits
func (rl *RateLimiter) CheckLimit(ctx context.Context) error {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()

	// Check if in backoff period
	if now.Before(rl.backoffUntil) {
		return fmt.Errorf("rate limited: backoff until %s", rl.backoffUntil.Format(time.RFC3339))
	}

	// Reset counters if needed
	rl.resetCountersIfNeeded(now)

	// Check per-minute limit
	rl.cleanExpiredRequests(now, time.Minute)
	if len(rl.requestTimes) >= rl.config.RequestsPerMin {
		return fmt.Errorf("rate limited: %d requests per minute exceeded", rl.config.RequestsPerMin)
	}

	// Check hourly limit
	rl.cleanExpiredHourlyRequests(now)
	if len(rl.hourlyRequests) >= rl.config.RequestsPerHour {
		return fmt.Errorf("rate limited: %d requests per hour exceeded", rl.config.RequestsPerHour)
	}

	// Check daily budget
	if rl.dailyCount >= rl.config.DailyRequests {
		return fmt.Errorf("daily budget exhausted: %d/%d requests", rl.dailyCount, rl.config.DailyRequests)
	}

	// Check monthly budget
	if rl.monthlyCount >= rl.config.MonthlyBudget {
		return fmt.Errorf("monthly budget exhausted: %d/%d requests", rl.monthlyCount, rl.config.MonthlyBudget)
	}

	// Check weight limit (Binance)
	if rl.config.WeightLimit > 0 && rl.currentWeight >= rl.config.WeightLimit {
		return fmt.Errorf("weight limit exceeded: %d/%d", rl.currentWeight, rl.config.WeightLimit)
	}

	return nil
}

// RecordRequest tracks successful request and updates counters
func (rl *RateLimiter) RecordRequest(weight int) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	rl.requestTimes = append(rl.requestTimes, now)
	rl.hourlyRequests = append(rl.hourlyRequests, now)
	rl.dailyCount++
	rl.monthlyCount++
	rl.currentWeight += weight

	// Reset backoff on successful request
	rl.backoffAttempts = 0
}

// HandleRateLimit processes rate limit responses (429, 418)
func (rl *RateLimiter) HandleRateLimit(statusCode int, headers map[string]string) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	rl.backoffAttempts++

	// Parse Retry-After header if present
	var retryAfter time.Duration = rl.calculateBackoff()
	if retryAfterStr, exists := headers["Retry-After"]; exists {
		if seconds, err := strconv.Atoi(retryAfterStr); err == nil {
			retryAfter = time.Duration(seconds) * time.Second
		}
	}

	// Update weight from Binance headers
	rl.updateWeightFromHeaders(headers)

	rl.backoffUntil = time.Now().Add(retryAfter)

	log.Warn().
		Str("provider", rl.config.Provider).
		Int("status", statusCode).
		Dur("backoff", retryAfter).
		Int("attempts", rl.backoffAttempts).
		Msg("Rate limit triggered, applying backoff")
}

// GetStatus returns current limiter status
func (rl *RateLimiter) GetStatus() map[string]interface{} {
	rl.mu.RLock()
	defer rl.mu.RUnlock()

	now := time.Now()
	rl.cleanExpiredRequests(now, time.Minute)
	rl.cleanExpiredHourlyRequests(now)

	status := map[string]interface{}{
		"provider":          rl.config.Provider,
		"requests_per_min":  fmt.Sprintf("%d/%d", len(rl.requestTimes), rl.config.RequestsPerMin),
		"requests_per_hour": fmt.Sprintf("%d/%d", len(rl.hourlyRequests), rl.config.RequestsPerHour),
		"daily_requests":    fmt.Sprintf("%d/%d", rl.dailyCount, rl.config.DailyRequests),
		"monthly_budget":    fmt.Sprintf("%d/%d", rl.monthlyCount, rl.config.MonthlyBudget),
		"backoff_until":     rl.backoffUntil,
		"is_throttled":      time.Now().Before(rl.backoffUntil),
		"backoff_attempts":  rl.backoffAttempts,
	}

	if rl.config.WeightLimit > 0 {
		status["weight"] = fmt.Sprintf("%d/%d", rl.currentWeight, rl.config.WeightLimit)
	}

	return status
}

// Helper methods
func (rl *RateLimiter) resetCountersIfNeeded(now time.Time) {
	// Reset daily counter
	if now.Sub(rl.lastDayReset) >= 24*time.Hour {
		rl.dailyCount = 0
		rl.lastDayReset = now
		log.Debug().Str("provider", rl.config.Provider).Msg("Reset daily request counter")
	}

	// Reset monthly counter
	if now.Sub(rl.lastMonthReset) >= 30*24*time.Hour {
		rl.monthlyCount = 0
		rl.lastMonthReset = now
		log.Debug().Str("provider", rl.config.Provider).Msg("Reset monthly request counter")
	}

	// Reset weight (for providers with weight systems)
	if rl.config.WeightLimit > 0 && len(rl.requestTimes) == 0 {
		rl.currentWeight = 0
	}
}

func (rl *RateLimiter) cleanExpiredRequests(now time.Time, window time.Duration) {
	cutoff := now.Add(-window)
	i := 0
	for i < len(rl.requestTimes) && rl.requestTimes[i].Before(cutoff) {
		i++
	}
	if i > 0 {
		rl.requestTimes = rl.requestTimes[i:]
	}
}

func (rl *RateLimiter) cleanExpiredHourlyRequests(now time.Time) {
	cutoff := now.Add(-time.Hour)
	i := 0
	for i < len(rl.hourlyRequests) && rl.hourlyRequests[i].Before(cutoff) {
		i++
	}
	if i > 0 {
		rl.hourlyRequests = rl.hourlyRequests[i:]
	}
}

func (rl *RateLimiter) calculateBackoff() time.Duration {
	backoff := rl.config.BackoffBase
	for i := 0; i < rl.backoffAttempts-1; i++ {
		backoff *= 2
		if backoff > rl.config.BackoffMax {
			backoff = rl.config.BackoffMax
			break
		}
	}
	return backoff
}

func (rl *RateLimiter) updateWeightFromHeaders(headers map[string]string) {
	for _, header := range rl.config.Headers {
		if weightStr, exists := headers[header]; exists {
			if weight, err := strconv.Atoi(weightStr); err == nil {
				rl.currentWeight = weight
				log.Debug().
					Str("provider", rl.config.Provider).
					Str("header", header).
					Int("weight", weight).
					Msg("Updated weight from header")
			}
		}
	}
}
