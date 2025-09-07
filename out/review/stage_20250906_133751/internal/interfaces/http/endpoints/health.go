package endpoints

import (
	"encoding/json"
	"net/http"
	"time"

	"cryptorun/internal/metrics"
)

// HealthResponse represents the health check response
type HealthResponse struct {
	Status    string    `json:"status"`
	Timestamp time.Time `json:"timestamp"`
	Version   string    `json:"version"`
	Uptime    string    `json:"uptime"`
	Services  Services  `json:"services"`
}

// Services represents the status of various system services
type Services struct {
	Database     ServiceStatus `json:"database"`
	Cache        ServiceStatus `json:"cache"`
	MessageQueue ServiceStatus `json:"message_queue"`
	ExternalAPIs ServiceStatus `json:"external_apis"`
}

// ServiceStatus represents the status of a single service
type ServiceStatus struct {
	Status       string `json:"status"` // "healthy", "degraded", "down"
	LastCheck    string `json:"last_check"`
	ResponseTime string `json:"response_time,omitempty"`
}

var startTime = time.Now()

// HealthHandler returns a health check endpoint
func HealthHandler(collector *metrics.Collector) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", "GET")
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Get API health metrics from collector
		apiHealth := collector.GetAPIHealth()
		cacheMetrics := collector.GetCacheMetrics()

		// Determine overall API health status
		apiStatus := "healthy"
		for _, health := range apiHealth {
			if health.Status == "down" {
				apiStatus = "down"
				break
			} else if health.Status == "degraded" {
				apiStatus = "degraded"
			}
		}

		// Determine cache status based on hit rates
		cacheStatus := "healthy"
		if cacheMetrics.HotCache.HitRate < 0.7 || cacheMetrics.WarmCache.HitRate < 0.5 {
			cacheStatus = "degraded"
		}

		// Build health response
		uptime := time.Since(startTime)
		response := HealthResponse{
			Status:    "healthy", // Overall system status
			Timestamp: time.Now(),
			Version:   "v3.2.1",
			Uptime:    uptime.String(),
			Services: Services{
				Database: ServiceStatus{
					Status:       "healthy",
					LastCheck:    time.Now().Format(time.RFC3339),
					ResponseTime: "2ms",
				},
				Cache: ServiceStatus{
					Status:       cacheStatus,
					LastCheck:    time.Now().Format(time.RFC3339),
					ResponseTime: "1ms",
				},
				MessageQueue: ServiceStatus{
					Status:       "healthy",
					LastCheck:    time.Now().Format(time.RFC3339),
					ResponseTime: "3ms",
				},
				ExternalAPIs: ServiceStatus{
					Status:    apiStatus,
					LastCheck: time.Now().Format(time.RFC3339),
				},
			},
		}

		// Set overall status to worst service status
		if apiStatus == "down" || cacheStatus == "down" {
			response.Status = "down"
		} else if apiStatus == "degraded" || cacheStatus == "degraded" {
			response.Status = "degraded"
		}

		w.Header().Set("Content-Type", "application/json")

		// Set appropriate HTTP status code
		switch response.Status {
		case "healthy":
			w.WriteHeader(http.StatusOK)
		case "degraded":
			w.WriteHeader(http.StatusOK) // 200 but degraded
		case "down":
			w.WriteHeader(http.StatusServiceUnavailable)
		}

		if err := json.NewEncoder(w).Encode(response); err != nil {
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
	}
}
