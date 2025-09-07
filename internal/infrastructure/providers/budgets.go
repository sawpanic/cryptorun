package providers

import (
	"fmt"
	"sync"
	"time"
)

type BudgetGuard struct {
	providers map[string]*ProviderBudget
	mutex     sync.RWMutex
}

type ProviderBudget struct {
	Name               string
	MonthlyLimit       int
	MonthlyUsed        int
	DailyLimit         int
	DailyUsed          int
	HourlyLimit        int
	HourlyUsed         int
	LastReset          time.Time
	MonthlyResetTime   time.Time
	DailyResetTime     time.Time
	HourlyResetTime    time.Time
	CostPerCall        float64
	EstimatedMonthlyCU int
}

type BudgetStatus struct {
	Provider           string
	MonthlyUtilization float64
	DailyUtilization   float64
	HourlyUtilization  float64
	RemainingCalls     int
	NextReset          time.Time
	Status             string // "ACTIVE", "WARNING", "LIMIT_REACHED"
}

func NewBudgetGuard() *BudgetGuard {
	return &BudgetGuard{
		providers: make(map[string]*ProviderBudget),
	}
}

func (bg *BudgetGuard) InitializeProvider(name string, monthlyLimit, dailyLimit, hourlyLimit int, costPerCall float64) {
	bg.mutex.Lock()
	defer bg.mutex.Unlock()
	
	now := time.Now()
	budget := &ProviderBudget{
		Name:             name,
		MonthlyLimit:     monthlyLimit,
		DailyLimit:       dailyLimit,
		HourlyLimit:      hourlyLimit,
		MonthlyUsed:      0,
		DailyUsed:        0,
		HourlyUsed:       0,
		LastReset:        now,
		MonthlyResetTime: getNextMonthReset(now),
		DailyResetTime:   getNextDayReset(now),
		HourlyResetTime:  getNextHourReset(now),
		CostPerCall:      costPerCall,
	}
	
	bg.providers[name] = budget
}

func (bg *BudgetGuard) CheckAndConsume(provider string, calls int) error {
	bg.mutex.Lock()
	defer bg.mutex.Unlock()
	
	budget, exists := bg.providers[provider]
	if !exists {
		return fmt.Errorf("budget not configured for provider: %s", provider)
	}
	
	// Reset counters if windows have expired
	bg.resetExpiredWindows(budget)
	
	// Check limits before consuming
	if budget.MonthlyUsed+calls > budget.MonthlyLimit {
		return fmt.Errorf("monthly budget exceeded for %s: %d/%d calls", 
			provider, budget.MonthlyUsed, budget.MonthlyLimit)
	}
	
	if budget.DailyUsed+calls > budget.DailyLimit {
		return fmt.Errorf("daily budget exceeded for %s: %d/%d calls", 
			provider, budget.DailyUsed, budget.DailyLimit)
	}
	
	if budget.HourlyUsed+calls > budget.HourlyLimit {
		return fmt.Errorf("hourly budget exceeded for %s: %d/%d calls", 
			provider, budget.HourlyUsed, budget.HourlyLimit)
	}
	
	// Consume budget
	budget.MonthlyUsed += calls
	budget.DailyUsed += calls
	budget.HourlyUsed += calls
	budget.LastReset = time.Now()
	
	return nil
}

func (bg *BudgetGuard) GetBudgetStatus(provider string) *BudgetStatus {
	bg.mutex.RLock()
	defer bg.mutex.RUnlock()
	
	budget, exists := bg.providers[provider]
	if !exists {
		return nil
	}
	
	// Calculate utilization percentages
	monthlyUtil := float64(budget.MonthlyUsed) / float64(budget.MonthlyLimit) * 100
	dailyUtil := float64(budget.DailyUsed) / float64(budget.DailyLimit) * 100
	hourlyUtil := float64(budget.HourlyUsed) / float64(budget.HourlyLimit) * 100
	
	// Determine status
	var status string
	maxUtil := max(monthlyUtil, dailyUtil, hourlyUtil)
	switch {
	case maxUtil >= 100:
		status = "LIMIT_REACHED"
	case maxUtil >= 80:
		status = "WARNING"
	default:
		status = "ACTIVE"
	}
	
	// Calculate remaining calls (most restrictive limit)
	remainingMonthly := budget.MonthlyLimit - budget.MonthlyUsed
	remainingDaily := budget.DailyLimit - budget.DailyUsed
	remainingHourly := budget.HourlyLimit - budget.HourlyUsed
	remainingCalls := min(remainingMonthly, remainingDaily, remainingHourly)
	
	// Next reset is the earliest of all reset times
	nextReset := budget.HourlyResetTime
	if budget.DailyResetTime.Before(nextReset) {
		nextReset = budget.DailyResetTime
	}
	if budget.MonthlyResetTime.Before(nextReset) {
		nextReset = budget.MonthlyResetTime
	}
	
	return &BudgetStatus{
		Provider:           provider,
		MonthlyUtilization: monthlyUtil,
		DailyUtilization:   dailyUtil,
		HourlyUtilization:  hourlyUtil,
		RemainingCalls:     remainingCalls,
		NextReset:          nextReset,
		Status:             status,
	}
}

func (bg *BudgetGuard) GetAllStatuses() map[string]*BudgetStatus {
	bg.mutex.RLock()
	defer bg.mutex.RUnlock()
	
	statuses := make(map[string]*BudgetStatus)
	for provider := range bg.providers {
		statuses[provider] = bg.GetBudgetStatus(provider)
	}
	
	return statuses
}

func (bg *BudgetGuard) resetExpiredWindows(budget *ProviderBudget) {
	now := time.Now()
	
	// Reset hourly window
	if now.After(budget.HourlyResetTime) {
		budget.HourlyUsed = 0
		budget.HourlyResetTime = getNextHourReset(now)
	}
	
	// Reset daily window
	if now.After(budget.DailyResetTime) {
		budget.DailyUsed = 0
		budget.DailyResetTime = getNextDayReset(now)
	}
	
	// Reset monthly window
	if now.After(budget.MonthlyResetTime) {
		budget.MonthlyUsed = 0
		budget.MonthlyResetTime = getNextMonthReset(now)
	}
}

func getNextHourReset(t time.Time) time.Time {
	return t.Truncate(time.Hour).Add(time.Hour)
}

func getNextDayReset(t time.Time) time.Time {
	year, month, day := t.Date()
	return time.Date(year, month, day+1, 0, 0, 0, 0, t.Location())
}

func getNextMonthReset(t time.Time) time.Time {
	year, month, _ := t.Date()
	return time.Date(year, month+1, 1, 0, 0, 0, 0, t.Location())
}

func max(a, b, c float64) float64 {
	if a > b && a > c {
		return a
	}
	if b > c {
		return b
	}
	return c
}

func min(a, b, c int) int {
	if a < b && a < c {
		return a
	}
	if b < c {
		return b
	}
	return c
}

// Default budget configurations for free tier providers
func GetDefaultBudgets() map[string]*ProviderBudget {
	now := time.Now()
	
	return map[string]*ProviderBudget{
		"coingecko": {
			Name:             "CoinGecko",
			MonthlyLimit:     10000, // 10k calls/month free tier
			DailyLimit:       500,   // Conservative daily limit
			HourlyLimit:      50,    // Conservative hourly limit
			MonthlyUsed:      0,
			DailyUsed:        0,
			HourlyUsed:       0,
			LastReset:        now,
			MonthlyResetTime: getNextMonthReset(now),
			DailyResetTime:   getNextDayReset(now),
			HourlyResetTime:  getNextHourReset(now),
			CostPerCall:      0.0, // Free tier
		},
		"moralis": {
			Name:             "Moralis",
			MonthlyLimit:     40000, // 40k CU/month free tier
			DailyLimit:       2000,  // Conservative daily CU limit
			HourlyLimit:      200,   // Conservative hourly CU limit
			MonthlyUsed:      0,
			DailyUsed:        0,
			HourlyUsed:       0,
			LastReset:        now,
			MonthlyResetTime: getNextMonthReset(now),
			DailyResetTime:   getNextDayReset(now),
			HourlyResetTime:  getNextHourReset(now),
			CostPerCall:      0.0, // Free tier
		},
		"binance": {
			Name:             "Binance",
			MonthlyLimit:     1000000, // Very high - no explicit monthly limit
			DailyLimit:       20000,   // Conservative based on weight system
			HourlyLimit:      2000,    // Conservative hourly limit
			MonthlyUsed:      0,
			DailyUsed:        0,
			HourlyUsed:       0,
			LastReset:        now,
			MonthlyResetTime: getNextMonthReset(now),
			DailyResetTime:   getNextDayReset(now),
			HourlyResetTime:  getNextHourReset(now),
			CostPerCall:      0.0, // Free tier
		},
		"kraken": {
			Name:             "Kraken",
			MonthlyLimit:     100000, // Conservative estimate
			DailyLimit:       5000,   // Conservative daily limit
			HourlyLimit:      500,    // Conservative hourly limit
			MonthlyUsed:      0,
			DailyUsed:        0,
			HourlyUsed:       0,
			LastReset:        now,
			MonthlyResetTime: getNextMonthReset(now),
			DailyResetTime:   getNextDayReset(now),
			HourlyResetTime:  getNextHourReset(now),
			CostPerCall:      0.0, // Free tier
		},
	}
}