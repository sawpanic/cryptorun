package http

import (
	"encoding/json"
	"fmt"
	"net/http"
	"runtime"
	"time"

	"github.com/sawpanic/cryptorun/internal/provider"
)

// HealthHandler provides system health status endpoint
type HealthHandler struct {
	registry     provider.ProviderRegistry
	startTime    time.Time
	version      string
	buildStamp   string
}

// NewHealthHandler creates a new health handler
func NewHealthHandler(registry provider.ProviderRegistry, version, buildStamp string) *HealthHandler {
	return &HealthHandler{
		registry:   registry,
		startTime:  time.Now(),
		version:    version,
		buildStamp: buildStamp,
	}
}

// HealthResponse represents the health check response
type HealthResponse struct {
	Status      string                        `json:"status"`      // "healthy", "degraded", "unhealthy"
	Timestamp   time.Time                     `json:"timestamp"`
	Uptime      string                        `json:"uptime"`
	Version     string                        `json:"version"`
	BuildStamp  string                        `json:"build_stamp"`
	
	// System info
	System      SystemInfo                    `json:"system"`
	
	// Provider status
	Providers   map[string]provider.ProviderHealth `json:"providers"`
	Summary     ProviderSummary               `json:"provider_summary"`
	
	// Service checks
	Checks      map[string]CheckResult        `json:"checks"`
}

// SystemInfo provides system-level information
type SystemInfo struct {
	GoVersion    string  `json:"go_version"`
	NumGoroutines int    `json:"num_goroutines"`
	MemAlloc     uint64  `json:"mem_alloc_bytes"`
	MemSys       uint64  `json:"mem_sys_bytes"`
	NumGC        uint32  `json:"num_gc"`
}

// ProviderSummary provides aggregate provider status
type ProviderSummary struct {
	Total    int `json:"total"`
	Healthy  int `json:"healthy"`
	Degraded int `json:"degraded"`
	Failed   int `json:"failed"`
}

// CheckResult represents individual health check results
type CheckResult struct {
	Status    string        `json:"status"`    // "pass", "warn", "fail"
	Message   string        `json:"message"`
	Duration  time.Duration `json:"duration"`
	Timestamp time.Time     `json:"timestamp"`
}

// ServeHTTP implements the health check endpoint
func (h *HealthHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	
	start := time.Now()
	
	// Gather all health information
	response := h.gatherHealthInfo()
	
	// Set response headers
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	
	// Set HTTP status based on overall health
	switch response.Status {
	case "healthy":
		w.WriteHeader(http.StatusOK)
	case "degraded":
		w.WriteHeader(http.StatusOK) // Still return 200 for degraded
	case "unhealthy":
		w.WriteHeader(http.StatusServiceUnavailable)
	default:
		w.WriteHeader(http.StatusInternalServerError)
	}
	
	// Add processing time
	response.Checks["health_endpoint"] = CheckResult{
		Status:    "pass",
		Message:   "Health endpoint responding",
		Duration:  time.Since(start),
		Timestamp: time.Now(),
	}
	
	// Encode and send response
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}
}

// gatherHealthInfo collects all health information
func (h *HealthHandler) gatherHealthInfo() HealthResponse {
	now := time.Now()
	
	response := HealthResponse{
		Timestamp:  now,
		Uptime:     time.Since(h.startTime).String(),
		Version:    h.version,
		BuildStamp: h.buildStamp,
		System:     h.getSystemInfo(),
		Providers:  make(map[string]provider.ProviderHealth),
		Checks:     make(map[string]CheckResult),
	}
	
	// Get provider health if registry is available
	if h.registry != nil {
		response.Providers = h.registry.Health()
		response.Summary = h.calculateProviderSummary(response.Providers)
		
		// Add provider-specific checks
		h.addProviderChecks(&response)
	}
	
	// Add system-level checks
	h.addSystemChecks(&response)
	
	// Determine overall status
	response.Status = h.calculateOverallStatus(response.Providers, response.Checks)
	
	return response
}

// getSystemInfo collects system runtime information
func (h *HealthHandler) getSystemInfo() SystemInfo {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	
	return SystemInfo{
		GoVersion:     runtime.Version(),
		NumGoroutines: runtime.NumGoroutine(),
		MemAlloc:      memStats.Alloc,
		MemSys:        memStats.Sys,
		NumGC:         memStats.NumGC,
	}
}

// calculateProviderSummary aggregates provider health status
func (h *HealthHandler) calculateProviderSummary(providers map[string]provider.ProviderHealth) ProviderSummary {
	summary := ProviderSummary{}
	
	for _, health := range providers {
		summary.Total++
		
		if health.Healthy {
			// Check if degraded (healthy but with issues)
			if health.Metrics.SuccessRate < 0.95 || health.ResponseTime > time.Second {
				summary.Degraded++
			} else {
				summary.Healthy++
			}
		} else {
			summary.Failed++
		}
	}
	
	return summary
}

// addProviderChecks adds provider-specific health checks
func (h *HealthHandler) addProviderChecks(response *HealthResponse) {
	// Check if we have any providers
	if len(response.Providers) == 0 {
		response.Checks["providers"] = CheckResult{
			Status:    "warn",
			Message:   "No providers registered",
			Duration:  0,
			Timestamp: time.Now(),
		}
		return
	}
	
	// Check critical provider availability
	criticalProviders := []string{"kraken", "binance"} // Define critical providers
	criticalHealthy := 0
	
	for _, providerName := range criticalProviders {
		if health, exists := response.Providers[providerName]; exists && health.Healthy {
			criticalHealthy++
		}
	}
	
	if criticalHealthy == 0 {
		response.Checks["critical_providers"] = CheckResult{
			Status:    "fail",
			Message:   "No critical providers available",
			Duration:  0,
			Timestamp: time.Now(),
		}
	} else if criticalHealthy < len(criticalProviders) {
		response.Checks["critical_providers"] = CheckResult{
			Status:    "warn",
			Message:   fmt.Sprintf("Only %d/%d critical providers healthy", criticalHealthy, len(criticalProviders)),
			Duration:  0,
			Timestamp: time.Now(),
		}
	} else {
		response.Checks["critical_providers"] = CheckResult{
			Status:    "pass",
			Message:   "All critical providers healthy",
			Duration:  0,
			Timestamp: time.Now(),
		}
	}
	
	// Check overall provider success rate
	if response.Summary.Total > 0 {
		healthyRate := float64(response.Summary.Healthy) / float64(response.Summary.Total)
		
		if healthyRate < 0.5 {
			response.Checks["provider_availability"] = CheckResult{
				Status:    "fail",
				Message:   fmt.Sprintf("Provider availability too low: %.1f%%", healthyRate*100),
				Duration:  0,
				Timestamp: time.Now(),
			}
		} else if healthyRate < 0.8 {
			response.Checks["provider_availability"] = CheckResult{
				Status:    "warn",
				Message:   fmt.Sprintf("Provider availability degraded: %.1f%%", healthyRate*100),
				Duration:  0,
				Timestamp: time.Now(),
			}
		} else {
			response.Checks["provider_availability"] = CheckResult{
				Status:    "pass",
				Message:   fmt.Sprintf("Provider availability good: %.1f%%", healthyRate*100),
				Duration:  0,
				Timestamp: time.Now(),
			}
		}
	}
}

// addSystemChecks adds system-level health checks
func (h *HealthHandler) addSystemChecks(response *HealthResponse) {
	// Memory usage check
	memUsagePercent := float64(response.System.MemAlloc) / float64(response.System.MemSys) * 100
	
	if memUsagePercent > 90 {
		response.Checks["memory"] = CheckResult{
			Status:    "fail",
			Message:   fmt.Sprintf("Memory usage critical: %.1f%%", memUsagePercent),
			Duration:  0,
			Timestamp: time.Now(),
		}
	} else if memUsagePercent > 75 {
		response.Checks["memory"] = CheckResult{
			Status:    "warn",
			Message:   fmt.Sprintf("Memory usage high: %.1f%%", memUsagePercent),
			Duration:  0,
			Timestamp: time.Now(),
		}
	} else {
		response.Checks["memory"] = CheckResult{
			Status:    "pass",
			Message:   fmt.Sprintf("Memory usage normal: %.1f%%", memUsagePercent),
			Duration:  0,
			Timestamp: time.Now(),
		}
	}
	
	// Goroutine count check
	if response.System.NumGoroutines > 1000 {
		response.Checks["goroutines"] = CheckResult{
			Status:    "warn",
			Message:   fmt.Sprintf("High goroutine count: %d", response.System.NumGoroutines),
			Duration:  0,
			Timestamp: time.Now(),
		}
	} else {
		response.Checks["goroutines"] = CheckResult{
			Status:    "pass",
			Message:   fmt.Sprintf("Goroutine count normal: %d", response.System.NumGoroutines),
			Duration:  0,
			Timestamp: time.Now(),
		}
	}
	
	// Uptime check
	uptime := time.Since(h.startTime)
	if uptime < time.Minute {
		response.Checks["uptime"] = CheckResult{
			Status:    "warn",
			Message:   "Service recently started",
			Duration:  0,
			Timestamp: time.Now(),
		}
	} else {
		response.Checks["uptime"] = CheckResult{
			Status:    "pass",
			Message:   fmt.Sprintf("Service uptime: %s", uptime.String()),
			Duration:  0,
			Timestamp: time.Now(),
		}
	}
}

// calculateOverallStatus determines overall service health
func (h *HealthHandler) calculateOverallStatus(providers map[string]provider.ProviderHealth, checks map[string]CheckResult) string {
	// Check for any failing checks
	for _, check := range checks {
		if check.Status == "fail" {
			return "unhealthy"
		}
	}
	
	// Check provider status
	if len(providers) == 0 {
		return "degraded" // No providers is concerning but not critical
	}
	
	healthyProviders := 0
	for _, health := range providers {
		if health.Healthy {
			healthyProviders++
		}
	}
	
	if healthyProviders == 0 {
		return "unhealthy"
	}
	
	healthyRate := float64(healthyProviders) / float64(len(providers))
	if healthyRate < 0.5 {
		return "unhealthy"
	} else if healthyRate < 0.8 {
		return "degraded"
	}
	
	// Check for any warning conditions
	for _, check := range checks {
		if check.Status == "warn" {
			return "degraded"
		}
	}
	
	return "healthy"
}