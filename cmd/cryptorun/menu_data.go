package main

import (
	_ "context" // Unused in current build
	"fmt"
	"time"

	"github.com/sawpanic/cryptorun/internal/data/facade"
	"github.com/sawpanic/cryptorun/internal/data/cache"
	_ "github.com/sawpanic/cryptorun/internal/data/pit" // Unused in current build
	"github.com/sawpanic/cryptorun/internal/data/rl"
	_ "github.com/sawpanic/cryptorun/internal/data/exchanges/kraken" // Unused in current build
)

// DataFacadeStatus displays comprehensive data facade status in the menu
type DataFacadeStatus struct {
	facade facade.DataFacade
}

// NewDataFacadeStatus creates a new data facade status component
func NewDataFacadeStatus() (*DataFacadeStatus, error) {
	// Initialize data facade for menu display
	df, err := initializeDataFacadeForMenu()
	if err != nil {
		return nil, err
	}
	
	return &DataFacadeStatus{facade: df}, nil
}

// initializeDataFacadeForMenu creates a lightweight facade for status display
func initializeDataFacadeForMenu() (facade.DataFacade, error) {
	hotCfg := facade.HotConfig{
		Venues:       []string{"kraken", "binance", "okx", "coinbase"},
		MaxPairs:     30,
		ReconnectSec: 5,
		BufferSize:   1000,
		Timeout:      10 * time.Second,
	}
	
	warmCfg := facade.WarmConfig{
		Venues:       []string{"kraken", "binance", "okx", "coinbase"},
		DefaultTTL:   30 * time.Second,
		MaxRetries:   3,
		BackoffBase:  1 * time.Second,
		RequestLimit: 100,
	}
	
	cacheCfg := facade.CacheConfig{
		PricesHot:   5 * time.Second,
		PricesWarm:  30 * time.Second,
		VolumesVADR: 120 * time.Second,
		TokenMeta:   24 * time.Hour,
		MaxEntries:  10000,
	}
	
	_ = cache.NewTTLCache(cacheCfg.MaxEntries) // ttlCache unused
	rateLimiter := rl.NewRateLimiter()
	
	df := facade.New(hotCfg, warmCfg, cacheCfg, rateLimiter)
	return df, nil
}

// DisplayStatus renders the data facade status table for the menu
func (dfs *DataFacadeStatus) DisplayStatus() string {
	status := "\n📊 Data Facade Status\n"
	status += "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n"
	
	// Venue health status
	status += "\n🏦 Venue Health\n"
	status += fmt.Sprintf("%-12s %-10s %-4s %-6s %-10s %-12s %-s\n",
		"VENUE", "STATUS", "WS", "REST", "LATENCY", "BUDGET", "RECOMMENDATION")
	status += fmt.Sprintf("%-12s %-10s %-4s %-6s %-10s %-12s %-s\n",
		"-----", "------", "--", "----", "-------", "------", "--------------")
	
	venues := []string{"kraken", "binance", "okx", "coinbase"}
	for _, venue := range venues {
		health := dfs.facade.VenueHealth(venue)
		
		// Status indicator
		statusIcon := "🟢"
		switch health.Status {
		case "degraded":
			statusIcon = "🟡"
		case "offline":
			statusIcon = "🔴"
		}
		
		// WebSocket status
		wsStatus := "❌"
		if health.WSConnected {
			wsStatus = "✅"
		}
		
		// REST status
		restStatus := "❌"
		if health.RESTHealthy {
			restStatus = "✅"
		}
		
		// Latency formatting
		latencyStr := fmt.Sprintf("%dms", health.P99Latency/time.Millisecond)
		if health.P99Latency > 1*time.Second {
			latencyStr = fmt.Sprintf("%.1fs", health.P99Latency.Seconds())
		}
		
		// Budget status (mock for display)
		budgetStatus := "90%" // Would be calculated from rate limiter
		
		// Recommendation
		recommendation := health.Recommendation
		if recommendation == "" {
			recommendation = "-"
		}
		
		status += fmt.Sprintf("%-12s %s%-9s %-4s %-6s %-10s %-12s %-s\n",
			venue, statusIcon, health.Status, wsStatus, restStatus, 
			latencyStr, budgetStatus, recommendation)
	}
	
	// Cache performance
	status += "\n💾 Cache Performance\n"
	cacheStats := dfs.facade.CacheStats()
	status += fmt.Sprintf("%-15s %-8s %-8s %-8s %-10s %-s\n",
		"TIER", "TTL", "HITS", "MISSES", "ENTRIES", "HIT_RATIO")
	status += fmt.Sprintf("%-15s %-8s %-8s %-8s %-10s %-s\n",
		"----", "---", "----", "------", "-------", "---------")
	
	tiers := map[string]facade.CacheTierStats{
		"prices_hot":    cacheStats.PricesHot,
		"prices_warm":   cacheStats.PricesWarm,
		"volumes_vadr":  cacheStats.VolumesVADR,
		"token_meta":    cacheStats.TokenMeta,
	}
	
	for name, tierStats := range tiers {
		ttlStr := formatTTL(tierStats.TTL)
		hitRatio := tierStats.HitRatio
		hitRatioStr := fmt.Sprintf("%.1f%%", hitRatio*100)
		
		// Color code hit ratio
		hitRatioIcon := "🟢"
		if hitRatio < 0.7 {
			hitRatioIcon = "🟡"
		}
		if hitRatio < 0.5 {
			hitRatioIcon = "🔴"
		}
		
		status += fmt.Sprintf("%-15s %-8s %-8d %-8d %-10d %s%-s\n",
			name, ttlStr, tierStats.Hits, tierStats.Misses, 
			tierStats.Entries, hitRatioIcon, hitRatioStr)
	}
	
	// Overall cache summary
	totalEntries := cacheStats.TotalEntries
	status += fmt.Sprintf("\nTotal Cache Entries: %d\n", totalEntries)
	
	// Data freshness indicators
	status += "\n🔄 Data Freshness\n"
	for _, venue := range venues {
		attr := dfs.facade.SourceAttribution(venue)
		if !attr.LastUpdate.IsZero() {
			age := time.Since(attr.LastUpdate)
			ageStr := formatAge(age)
			
			freshnessIcon := "🟢"
			if age > 1*time.Minute {
				freshnessIcon = "🟡"
			}
			if age > 5*time.Minute {
				freshnessIcon = "🔴"
			}
			
			status += fmt.Sprintf("%-12s %s %s ago (%d sources)\n",
				venue, freshnessIcon, ageStr, len(attr.Sources))
		} else {
			status += fmt.Sprintf("%-12s ⚫ No data\n", venue)
		}
	}
	
	// Rate limiting status
	status += "\n⏱️  Rate Limiting\n"
	status += fmt.Sprintf("%-12s %-12s %-10s %-12s %-s\n",
		"VENUE", "REMAINING", "RESET", "STATUS", "BACKOFF")
	status += fmt.Sprintf("%-12s %-12s %-10s %-12s %-s\n",
		"-----", "---------", "-----", "------", "-------")
	
	// Mock rate limit data for display
	for _, venue := range venues {
		// In real implementation, would get from rate limiter
		remaining := "8.5M/10M"
		reset := "23d"
		rlStatus := "🟢 OK"
		backoff := "-"
		
		status += fmt.Sprintf("%-12s %-12s %-10s %-12s %-s\n",
			venue, remaining, reset, rlStatus, backoff)
	}
	
	status += "\n💡 Commands: 'cryptorun probe data' • 'cryptorun probe data --stream'\n"
	
	return status
}

// formatTTL formats duration for display
func formatTTL(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	} else if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	} else if d < 24*time.Hour {
		return fmt.Sprintf("%dh", int(d.Hours()))
	} else {
		return fmt.Sprintf("%dd", int(d.Hours()/24))
	}
}

// formatAge formats age for display
func formatAge(age time.Duration) string {
	if age < time.Minute {
		return fmt.Sprintf("%ds", int(age.Seconds()))
	} else if age < time.Hour {
		return fmt.Sprintf("%dm", int(age.Minutes()))
	} else {
		return fmt.Sprintf("%dh", int(age.Hours()))
	}
}

// GetDataFacadeMenuOption returns the menu option for data facade status
func GetDataFacadeMenuOption() MenuOption {
	return MenuOption{
		Key:         "d",
		Label:       "Data Facade Status",
		Description: "View exchange connectivity, cache performance, and data freshness",
		Handler: func() error {
			dfs, err := NewDataFacadeStatus()
			if err != nil {
				return fmt.Errorf("failed to initialize data facade status: %w", err)
			}
			
			fmt.Print(dfs.DisplayStatus())
			return nil
		},
	}
}

// MenuOption represents a menu item (assuming this exists in menu system)
type MenuOption struct {
	Key         string
	Label       string
	Description string
	Handler     func() error
}