package http

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/sawpanic/cryptorun/internal/provider"
)

func TestHealthHandler(t *testing.T) {
	// Create mock registry
	registry := provider.NewProviderRegistry()
	
	// Create health handler
	handler := NewHealthHandler(registry, "v1.0.0", "test-build")
	
	// Create test request
	req, err := http.NewRequest("GET", "/health", nil)
	if err != nil {
		t.Fatal(err)
	}
	
	// Create response recorder
	rr := httptest.NewRecorder()
	
	// Execute request
	handler.ServeHTTP(rr, req)
	
	// Check status code
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("Handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}
	
	// Check content type
	expected := "application/json"
	if ctype := rr.Header().Get("Content-Type"); ctype != expected {
		t.Errorf("Handler returned wrong content type: got %v want %v", ctype, expected)
	}
	
	// Parse response body
	var response HealthResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Errorf("Failed to parse JSON response: %v", err)
	}
	
	// Check response fields
	if response.Version != "v1.0.0" {
		t.Errorf("Expected version v1.0.0, got %s", response.Version)
	}
	
	if response.BuildStamp != "test-build" {
		t.Errorf("Expected build stamp test-build, got %s", response.BuildStamp)
	}
	
	if response.Status == "" {
		t.Error("Expected status field to be populated")
	}
	
	// Check that we have system information
	if response.System.GoVersion == "" {
		t.Error("Expected Go version to be populated")
	}
	
	if response.System.NumGoroutines <= 0 {
		t.Error("Expected positive number of goroutines")
	}
}

func TestMetricsHandler(t *testing.T) {
	// Create mock registry
	registry := provider.NewProviderRegistry()
	
	// Create metrics handler
	handler := NewMetricsHandler(registry)
	
	// Create test request
	req, err := http.NewRequest("GET", "/metrics", nil)
	if err != nil {
		t.Fatal(err)
	}
	
	// Create response recorder
	rr := httptest.NewRecorder()
	
	// Execute request
	handler.ServeHTTP(rr, req)
	
	// Check status code
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("Handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}
	
	// Check content type
	expected := "text/plain; version=0.0.4; charset=utf-8"
	if ctype := rr.Header().Get("Content-Type"); ctype != expected {
		t.Errorf("Handler returned wrong content type: got %v want %v", ctype, expected)
	}
	
	// Check that response contains Prometheus metrics
	body := rr.Body.String()
	
	expectedMetrics := []string{
		"# HELP go_info",
		"# TYPE go_info gauge",
		"go_goroutines",
		"go_memstats_alloc_bytes",
		"process_uptime_seconds",
		"cryptorun_last_update_timestamp",
	}
	
	for _, metric := range expectedMetrics {
		if !contains(body, metric) {
			t.Errorf("Expected to find metric %s in response", metric)
		}
	}
}

func TestMetricsHandlerWithProvider(t *testing.T) {
	// Create registry with mock provider
	registry := provider.NewProviderRegistry()
	
	// Create mock provider
	mockProvider := &provider.MockExchangeProvider{}
	err := registry.Register(mockProvider)
	if err != nil {
		t.Fatalf("Failed to register mock provider: %v", err)
	}
	
	// Start registry
	err = registry.Start(nil)
	if err != nil {
		t.Fatalf("Failed to start registry: %v", err)
	}
	defer registry.Stop(nil)
	
	// Create metrics handler
	handler := NewMetricsHandler(registry)
	
	// Create test request
	req, err := http.NewRequest("GET", "/metrics", nil)
	if err != nil {
		t.Fatal(err)
	}
	
	// Create response recorder
	rr := httptest.NewRecorder()
	
	// Execute request
	handler.ServeHTTP(rr, req)
	
	// Check status code
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("Handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}
	
	// Check that response contains provider metrics
	body := rr.Body.String()
	
	expectedProviderMetrics := []string{
		"cryptorun_provider_healthy",
		"cryptorun_provider_requests_total",
		"cryptorun_provider_errors_total", 
		"cryptorun_provider_success_rate",
		"cryptorun_provider_response_time_seconds",
	}
	
	for _, metric := range expectedProviderMetrics {
		if !contains(body, metric) {
			t.Errorf("Expected to find provider metric %s in response", metric)
		}
	}
}

func TestHealthHandlerWithProviders(t *testing.T) {
	// Create registry with providers
	registry := provider.NewProviderRegistry()
	
	// Create mock provider
	mockProvider := &provider.MockExchangeProvider{}
	err := registry.Register(mockProvider)
	if err != nil {
		t.Fatalf("Failed to register mock provider: %v", err)
	}
	
	// Start registry
	err = registry.Start(nil)
	if err != nil {
		t.Fatalf("Failed to start registry: %v", err)
	}
	defer registry.Stop(nil)
	
	// Create health handler
	handler := NewHealthHandler(registry, "v1.0.0", "test-build")
	
	// Create test request
	req, err := http.NewRequest("GET", "/health", nil)
	if err != nil {
		t.Fatal(err)
	}
	
	// Create response recorder
	rr := httptest.NewRecorder()
	
	// Execute request
	handler.ServeHTTP(rr, req)
	
	// Check status code
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("Handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}
	
	// Parse response body
	var response HealthResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Errorf("Failed to parse JSON response: %v", err)
	}
	
	// Check that we have provider information
	if len(response.Providers) == 0 {
		t.Error("Expected provider information to be present")
	}
	
	// Check provider summary
	if response.Summary.Total == 0 {
		t.Error("Expected provider summary to show registered providers")
	}
	
	// Check for health checks
	if len(response.Checks) == 0 {
		t.Error("Expected health checks to be present")
	}
}

func TestCustomMetrics(t *testing.T) {
	// Create metrics handler
	handler := NewMetricsHandler(nil)
	
	// Set custom metric
	handler.SetCustomMetric("test_metric", 42.0)
	
	// Create test request
	req, err := http.NewRequest("GET", "/metrics", nil)
	if err != nil {
		t.Fatal(err)
	}
	
	// Create response recorder
	rr := httptest.NewRecorder()
	
	// Execute request
	handler.ServeHTTP(rr, req)
	
	// Check that custom metric appears in response
	body := rr.Body.String()
	if !contains(body, "test_metric 42") {
		t.Error("Expected to find custom metric in response")
	}
}

func TestHealthStatuses(t *testing.T) {
	testCases := []struct {
		name           string
		providerCount  int
		healthyCount   int
		expectedStatus string
		expectedHTTP   int
	}{
		{
			name:           "No providers",
			providerCount:  0,
			healthyCount:   0,
			expectedStatus: "degraded",
			expectedHTTP:   http.StatusOK,
		},
		{
			name:           "All healthy",
			providerCount:  2,
			healthyCount:   2,
			expectedStatus: "healthy",
			expectedHTTP:   http.StatusOK,
		},
		{
			name:           "Partially healthy",
			providerCount:  4,
			healthyCount:   2,
			expectedStatus: "degraded", // 50% healthy
			expectedHTTP:   http.StatusOK,
		},
		{
			name:           "All unhealthy",
			providerCount:  3,
			healthyCount:   0,
			expectedStatus: "unhealthy",
			expectedHTTP:   http.StatusServiceUnavailable,
		},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create registry
			registry := provider.NewProviderRegistry()
			
			// Add mock providers
			for i := 0; i < tc.providerCount; i++ {
				mockProvider := &provider.MockExchangeProvider{}
				// TODO: Set health status based on test case
				registry.Register(mockProvider)
			}
			
			// Create handler
			handler := NewHealthHandler(registry, "v1.0.0", "test-build")
			
			// Create request
			req, err := http.NewRequest("GET", "/health", nil)
			if err != nil {
				t.Fatal(err)
			}
			
			// Execute request
			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)
			
			// Check HTTP status
			if rr.Code != tc.expectedHTTP {
				t.Errorf("Expected HTTP status %d, got %d", tc.expectedHTTP, rr.Code)
			}
			
			// Parse response
			var response HealthResponse
			if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
				t.Errorf("Failed to parse JSON response: %v", err)
				return
			}
			
			// Check status
			if response.Status != tc.expectedStatus {
				t.Errorf("Expected status %s, got %s", tc.expectedStatus, response.Status)
			}
		})
	}
}

// Helper function to check if string contains substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && 
		   (s == substr || 
		   (len(s) > len(substr) && s[:len(substr)] == substr) ||
		   findSubstring(s, substr))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}