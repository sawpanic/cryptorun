package datasources

import (
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"
)

// ProviderLimits defines rate limit configuration for each data provider
type ProviderLimits struct {
	Name           string
	RequestsPerSec int
	BurstLimit     int
	MonthlyQuota   int64
	DailyQuota     int64
	WeightBased    bool // for Binance-style weight system
}

// RateLimiter tracks usage and enforces limits for a provider
type RateLimiter struct {
	provider       ProviderLimits
	tokens         int
	lastRefill     time.Time
	requestsToday  int64
	requestsMonth  int64
	weightUsed     int // for Binance weight tracking
	mu             sync.RWMutex
}

// ProviderManager manages rate limiters for all data providers
type ProviderManager struct {
	limiters map[string]*RateLimiter
	mu       sync.RWMutex
}

// Define provider configurations
var DefaultProviders = map[string]ProviderLimits{
	"binance": {
		Name:           "Binance",
		RequestsPerSec: 20,
		BurstLimit:     40,
		MonthlyQuota:   0, // no monthly limit
		DailyQuota:     0, // no daily limit
		WeightBased:    true,
	},
	"coingecko": {
		Name:           "CoinGecko",
		RequestsPerSec: 10,
		BurstLimit:     20,
		MonthlyQuota:   10000,
		DailyQuota:     0,
		WeightBased:    false,
	},
	"moralis": {
		Name:           "Moralis",
		RequestsPerSec: 25,
		BurstLimit:     50,
		MonthlyQuota:   0,
		DailyQuota:     2000000, // 2M CU per day
		WeightBased:    false,
	},
	"dexscreener": {
		Name:           "DEXScreener",
		RequestsPerSec: 30,
		BurstLimit:     60,
		MonthlyQuota:   0,
		DailyQuota:     0,
		WeightBased:    false,
	},
	"kraken": {
		Name:           "Kraken",
		RequestsPerSec: 1,
		BurstLimit:     2,
		MonthlyQuota:   0,
		DailyQuota:     0,
		WeightBased:    false,
	},
}

// NewProviderManager creates a new provider manager with default configurations
func NewProviderManager() *ProviderManager {
	pm := &ProviderManager{
		limiters: make(map[string]*RateLimiter),
	}
	
	for name, limits := range DefaultProviders {
		pm.limiters[name] = &RateLimiter{
			provider:   limits,
			tokens:     limits.BurstLimit,
			lastRefill: time.Now(),
		}
	}
	
	return pm
}

// CanMakeRequest checks if a request can be made to the specified provider
func (pm *ProviderManager) CanMakeRequest(providerName string) bool {
	pm.mu.RLock()
	limiter, exists := pm.limiters[providerName]
	pm.mu.RUnlock()
	
	if !exists {
		return false
	}
	
	return limiter.canMakeRequest()
}

// RecordRequest records a request made to the specified provider
func (pm *ProviderManager) RecordRequest(providerName string, weight int) error {
	pm.mu.RLock()
	limiter, exists := pm.limiters[providerName]
	pm.mu.RUnlock()
	
	if !exists {
		return fmt.Errorf("unknown provider: %s", providerName)
	}
	
	return limiter.recordRequest(weight)
}

// ProcessResponseHeaders updates rate limit state from response headers
func (pm *ProviderManager) ProcessResponseHeaders(providerName string, headers http.Header) {
	pm.mu.RLock()
	limiter, exists := pm.limiters[providerName]
	pm.mu.RUnlock()
	
	if !exists {
		return
	}
	
	limiter.processHeaders(headers)
}

// GetUsageStats returns current usage statistics for a provider
func (pm *ProviderManager) GetUsageStats(providerName string) (UsageStats, error) {
	pm.mu.RLock()
	limiter, exists := pm.limiters[providerName]
	pm.mu.RUnlock()
	
	if !exists {
		return UsageStats{}, fmt.Errorf("unknown provider: %s", providerName)
	}
	
	return limiter.getUsageStats(), nil
}

// UsageStats represents current usage statistics for a provider
type UsageStats struct {
	Provider       string    `json:"provider"`
	RequestsToday  int64     `json:"requests_today"`
	RequestsMonth  int64     `json:"requests_month"`
	WeightUsed     int       `json:"weight_used"`
	TokensLeft     int       `json:"tokens_left"`
	LastRequest    time.Time `json:"last_request"`
	DailyQuota     int64     `json:"daily_quota"`
	MonthlyQuota   int64     `json:"monthly_quota"`
	HealthPercent  float64   `json:"health_percent"`
}

func (rl *RateLimiter) canMakeRequest() bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	
	rl.refillTokens()
	
	// Check token bucket
	if rl.tokens <= 0 {
		return false
	}
	
	// Check daily quota
	if rl.provider.DailyQuota > 0 && rl.requestsToday >= rl.provider.DailyQuota {
		return false
	}
	
	// Check monthly quota
	if rl.provider.MonthlyQuota > 0 && rl.requestsMonth >= rl.provider.MonthlyQuota {
		return false
	}
	
	return true
}

func (rl *RateLimiter) recordRequest(weight int) error {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	
	if !rl.canMakeRequestUnsafe() {
		return fmt.Errorf("rate limit exceeded for provider %s", rl.provider.Name)
	}
	
	rl.tokens--
	rl.requestsToday++
	rl.requestsMonth++
	
	if rl.provider.WeightBased {
		rl.weightUsed += weight
	}
	
	return nil
}

func (rl *RateLimiter) canMakeRequestUnsafe() bool {
	rl.refillTokens()
	
	if rl.tokens <= 0 {
		return false
	}
	
	if rl.provider.DailyQuota > 0 && rl.requestsToday >= rl.provider.DailyQuota {
		return false
	}
	
	if rl.provider.MonthlyQuota > 0 && rl.requestsMonth >= rl.provider.MonthlyQuota {
		return false
	}
	
	return true
}

func (rl *RateLimiter) refillTokens() {
	now := time.Now()
	elapsed := now.Sub(rl.lastRefill)
	
	if elapsed > time.Second {
		tokensToAdd := int(elapsed.Seconds()) * rl.provider.RequestsPerSec
		rl.tokens = minInt(rl.provider.BurstLimit, rl.tokens+tokensToAdd)
		rl.lastRefill = now
	}
}

func (rl *RateLimiter) processHeaders(headers http.Header) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	
	// Process Binance weight headers
	if rl.provider.WeightBased {
		if weight := headers.Get("X-MBX-USED-WEIGHT-1M"); weight != "" {
			if w, err := strconv.Atoi(weight); err == nil {
				rl.weightUsed = w
			}
		}
	}
}

func (rl *RateLimiter) getUsageStats() UsageStats {
	rl.mu.RLock()
	defer rl.mu.RUnlock()
	
	healthPercent := 100.0
	
	// Calculate health based on quotas
	if rl.provider.DailyQuota > 0 {
		dailyUsage := float64(rl.requestsToday) / float64(rl.provider.DailyQuota) * 100
		healthPercent = minFloat64(healthPercent, 100-dailyUsage)
	}
	
	if rl.provider.MonthlyQuota > 0 {
		monthlyUsage := float64(rl.requestsMonth) / float64(rl.provider.MonthlyQuota) * 100
		healthPercent = minFloat64(healthPercent, 100-monthlyUsage)
	}
	
	return UsageStats{
		Provider:      rl.provider.Name,
		RequestsToday: rl.requestsToday,
		RequestsMonth: rl.requestsMonth,
		WeightUsed:    rl.weightUsed,
		TokensLeft:    rl.tokens,
		LastRequest:   rl.lastRefill,
		DailyQuota:    rl.provider.DailyQuota,
		MonthlyQuota:  rl.provider.MonthlyQuota,
		HealthPercent: maxFloat64(0, healthPercent),
	}
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func minFloat64(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

func maxFloat64(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}