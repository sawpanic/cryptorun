package providers

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type ProviderBanner struct {
	rateLimiter *RateLimiter
	circuitMgr  *CircuitBreakerManager
	budgetGuard *BudgetGuard
}

type ProviderHealth struct {
	Timestamp  time.Time                    `json:"timestamp"`
	Summary    HealthSummary                `json:"summary"`
	RateStatus map[string]*RLBudget         `json:"rate_limits"`
	CBStatus   map[string]*BreakerStatus    `json:"circuit_breakers"`
	Budgets    map[string]*BudgetStatus     `json:"budgets"`
}

type HealthSummary struct {
	TotalProviders  int     `json:"total_providers"`
	ActiveProviders int     `json:"active_providers"`
	WarningCount    int     `json:"warning_count"`
	ErrorCount      int     `json:"error_count"`
	OverallHealth   string  `json:"overall_health"`
	CacheHitRate    float64 `json:"cache_hit_rate"`
}

func NewProviderBanner(rl *RateLimiter, cbm *CircuitBreakerManager, bg *BudgetGuard) *ProviderBanner {
	return &ProviderBanner{
		rateLimiter: rl,
		circuitMgr:  cbm,
		budgetGuard: bg,
	}
}

func (pb *ProviderBanner) DisplayStartupBanner() {
	health := pb.gatherProviderHealth()
	
	fmt.Println()
	fmt.Println("┌─────────────────────────────────────────────────────────────────────┐")
	fmt.Println("│                    🚀 CryptoRun Provider Health                     │")
	fmt.Println("└─────────────────────────────────────────────────────────────────────┘")
	fmt.Println()
	
	// Overall health summary
	fmt.Printf("📊 System Overview: %s (%d/%d providers active)\n",
		health.Summary.OverallHealth,
		health.Summary.ActiveProviders,
		health.Summary.TotalProviders)
	
	if health.Summary.CacheHitRate > 0 {
		fmt.Printf("📈 Cache Hit Rate: %.1f%%\n", health.Summary.CacheHitRate)
	}
	
	fmt.Println()
	
	// Provider status table
	fmt.Println("Provider Status:")
	fmt.Println("─────────────────────────────────────────────────────────────────────")
	fmt.Printf("%-12s │ %-8s │ %-12s │ %-15s │ %s\n", 
		"Provider", "Circuit", "Rate Limit", "Budget", "Status")
	fmt.Println("─────────────────────────────────────────────────────────────────────")
	
	providers := []string{"binance", "kraken", "coingecko", "moralis"}
	for _, provider := range providers {
		pb.displayProviderRow(provider, health)
	}
	
	fmt.Println("─────────────────────────────────────────────────────────────────────")
	fmt.Println()
	
	// Warnings and errors
	if health.Summary.WarningCount > 0 || health.Summary.ErrorCount > 0 {
		fmt.Printf("⚠️  %d warnings, ❌ %d errors detected\n", 
			health.Summary.WarningCount, health.Summary.ErrorCount)
		pb.displayAlerts(health)
		fmt.Println()
	}
	
	fmt.Printf("🕐 Status as of: %s\n", health.Timestamp.Format("15:04:05 MST"))
	fmt.Println()
}

func (pb *ProviderBanner) displayProviderRow(provider string, health *ProviderHealth) {
	// Get status for each component
	cbStatus := "N/A"
	if cb, exists := health.CBStatus[provider]; exists {
		switch cb.State {
		case "CLOSED":
			cbStatus = "🟢 OK"
		case "HALF_OPEN":
			cbStatus = "🟡 PROBE"
		case "OPEN":
			cbStatus = "🔴 OPEN"
		}
	}
	
	rlStatus := "N/A"
	if rl, exists := health.RateStatus[provider]; exists {
		utilization := float64(rl.Current) / float64(rl.Limit) * 100
		switch {
		case utilization < 50:
			rlStatus = "🟢 LOW"
		case utilization < 80:
			rlStatus = "🟡 MED"
		default:
			rlStatus = "🔴 HIGH"
		}
	}
	
	budgetStatus := "N/A"
	if budget, exists := health.Budgets[provider]; exists {
		switch budget.Status {
		case "ACTIVE":
			budgetStatus = "🟢 OK"
		case "WARNING":
			budgetStatus = "🟡 WARN"
		case "LIMIT_REACHED":
			budgetStatus = "🔴 LIMIT"
		}
	}
	
	// Overall provider status
	overallStatus := "🟢 HEALTHY"
	if cbStatus == "🔴 OPEN" || budgetStatus == "🔴 LIMIT" {
		overallStatus = "🔴 DOWN"
	} else if cbStatus == "🟡 PROBE" || rlStatus == "🔴 HIGH" || budgetStatus == "🟡 WARN" {
		overallStatus = "🟡 DEGRADED"
	}
	
	fmt.Printf("%-12s │ %-8s │ %-12s │ %-15s │ %s\n",
		provider, cbStatus, rlStatus, budgetStatus, overallStatus)
}

func (pb *ProviderBanner) displayAlerts(health *ProviderHealth) {
	for provider, cb := range health.CBStatus {
		if cb.State == "OPEN" {
			fmt.Printf("❌ %s circuit breaker OPEN (%.1f%% error rate)\n", 
				provider, cb.ErrorRate)
		}
	}
	
	for provider, budget := range health.Budgets {
		if budget.Status == "LIMIT_REACHED" {
			fmt.Printf("❌ %s budget limit reached (%d remaining calls)\n", 
				provider, budget.RemainingCalls)
		} else if budget.Status == "WARNING" {
			fmt.Printf("⚠️  %s budget warning (%.1f%% monthly utilization)\n", 
				provider, budget.MonthlyUtilization)
		}
	}
}

func (pb *ProviderBanner) WriteHealthJSON(artifactPath string) error {
	health := pb.gatherProviderHealth()
	
	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(artifactPath), 0755); err != nil {
		return fmt.Errorf("failed to create artifact directory: %w", err)
	}
	
	file, err := os.Create(artifactPath)
	if err != nil {
		return fmt.Errorf("failed to create health JSON file: %w", err)
	}
	defer file.Close()
	
	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(health); err != nil {
		return fmt.Errorf("failed to encode health JSON: %w", err)
	}
	
	return nil
}

func (pb *ProviderBanner) WriteBannerText(artifactPath string) error {
	// Capture banner text output
	health := pb.gatherProviderHealth()
	
	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(artifactPath), 0755); err != nil {
		return fmt.Errorf("failed to create artifact directory: %w", err)
	}
	
	file, err := os.Create(artifactPath)
	if err != nil {
		return fmt.Errorf("failed to create banner file: %w", err)
	}
	defer file.Close()
	
	// Write simplified banner to file
	fmt.Fprintf(file, "CryptoRun Provider Health - %s\n", health.Timestamp.Format("2006-01-02 15:04:05"))
	fmt.Fprintf(file, "Overall Health: %s\n", health.Summary.OverallHealth)
	fmt.Fprintf(file, "Active Providers: %d/%d\n", health.Summary.ActiveProviders, health.Summary.TotalProviders)
	fmt.Fprintf(file, "Warnings: %d, Errors: %d\n", health.Summary.WarningCount, health.Summary.ErrorCount)
	
	return nil
}

func (pb *ProviderBanner) gatherProviderHealth() *ProviderHealth {
	timestamp := time.Now()
	
	// Collect status from all components
	var rateStatus map[string]*RLBudget
	if pb.rateLimiter != nil {
		rateStatus = make(map[string]*RLBudget)
		providers := []string{"binance", "kraken", "coingecko", "moralis"}
		for _, provider := range providers {
			if budget := pb.rateLimiter.GetBudgetStatus(provider); budget != nil {
				rateStatus[provider] = budget
			}
		}
	}
	
	var cbStatus map[string]*BreakerStatus
	if pb.circuitMgr != nil {
		cbStatus = make(map[string]*BreakerStatus)
		providers := []string{"binance", "kraken", "coingecko", "moralis"}
		for _, provider := range providers {
			if status := pb.circuitMgr.GetStatus(provider); status != nil {
				cbStatus[provider] = status
			}
		}
	}
	
	var budgetStatus map[string]*BudgetStatus
	if pb.budgetGuard != nil {
		budgetStatus = pb.budgetGuard.GetAllStatuses()
	}
	
	// Calculate summary
	summary := pb.calculateHealthSummary(rateStatus, cbStatus, budgetStatus)
	
	return &ProviderHealth{
		Timestamp:  timestamp,
		Summary:    summary,
		RateStatus: rateStatus,
		CBStatus:   cbStatus,
		Budgets:    budgetStatus,
	}
}

func (pb *ProviderBanner) calculateHealthSummary(
	rateStatus map[string]*RLBudget,
	cbStatus map[string]*BreakerStatus,
	budgetStatus map[string]*BudgetStatus,
) HealthSummary {
	totalProviders := 4 // binance, kraken, coingecko, moralis
	activeProviders := 0
	warningCount := 0
	errorCount := 0
	
	providers := []string{"binance", "kraken", "coingecko", "moralis"}
	for _, provider := range providers {
		isActive := true
		hasWarning := false
		hasError := false
		
		// Check circuit breaker status
		if cb, exists := cbStatus[provider]; exists {
			if cb.State == "OPEN" {
				isActive = false
				hasError = true
			} else if cb.State == "HALF_OPEN" {
				hasWarning = true
			}
		}
		
		// Check budget status
		if budget, exists := budgetStatus[provider]; exists {
			if budget.Status == "LIMIT_REACHED" {
				isActive = false
				hasError = true
			} else if budget.Status == "WARNING" {
				hasWarning = true
			}
		}
		
		if isActive {
			activeProviders++
		}
		if hasWarning {
			warningCount++
		}
		if hasError {
			errorCount++
		}
	}
	
	// Determine overall health
	var overallHealth string
	healthPercentage := float64(activeProviders) / float64(totalProviders) * 100
	switch {
	case healthPercentage >= 100:
		overallHealth = "🟢 EXCELLENT"
	case healthPercentage >= 75:
		overallHealth = "🟡 GOOD"
	case healthPercentage >= 50:
		overallHealth = "🟠 DEGRADED"
	default:
		overallHealth = "🔴 CRITICAL"
	}
	
	// Mock cache hit rate (would integrate with actual cache layer)
	cacheHitRate := 87.5
	
	return HealthSummary{
		TotalProviders:  totalProviders,
		ActiveProviders: activeProviders,
		WarningCount:    warningCount,
		ErrorCount:      errorCount,
		OverallHealth:   overallHealth,
		CacheHitRate:    cacheHitRate,
	}
}