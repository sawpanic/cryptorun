package datasources

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestHealthManager_GetHealthSnapshot(t *testing.T) {
	pm := NewProviderManager()
	cm := NewCacheManager()
	circm := NewCircuitManager()
	hm := NewHealthManager(pm, cm, circm)
	
	// Record some activity
	pm.RecordRequest("binance", 1)
	cm.Set("test-key", "market_data", "test data")
	circm.RecordRequest("binance", true, 100*time.Millisecond, nil)
	hm.RecordLatency("binance", 150*time.Millisecond)
	
	snapshot := hm.GetHealthSnapshot()
	
	// Verify snapshot structure
	if snapshot.Timestamp.IsZero() {
		t.Error("Expected non-zero timestamp")
	}
	
	if len(snapshot.Providers) == 0 {
		t.Error("Expected provider health data")
	}
	
	if snapshot.Cache.Status == "" {
		t.Error("Expected cache status")
	}
	
	if len(snapshot.Circuits) == 0 {
		t.Error("Expected circuit health data")
	}
	
	// Verify provider health
	if binanceHealth, exists := snapshot.Providers["binance"]; exists {
		if binanceHealth.Name != "Binance" {
			t.Errorf("Expected provider name Binance, got %s", binanceHealth.Name)
		}
		if binanceHealth.RequestsToday != 1 {
			t.Errorf("Expected 1 request today, got %d", binanceHealth.RequestsToday)
		}
		if binanceHealth.Status == "" {
			t.Error("Expected provider status")
		}
	} else {
		t.Error("Expected binance provider health")
	}
}

func TestHealthManager_RecordLatency(t *testing.T) {
	pm := NewProviderManager()
	cm := NewCacheManager()
	circm := NewCircuitManager()
	hm := NewHealthManager(pm, cm, circm)
	
	// Record latency samples
	latencies := []time.Duration{
		100 * time.Millisecond,
		200 * time.Millisecond,
		150 * time.Millisecond,
		300 * time.Millisecond,
		250 * time.Millisecond,
	}
	
	for _, latency := range latencies {
		hm.RecordLatency("binance", latency)
	}
	
	snapshot := hm.GetHealthSnapshot()
	binanceHealth := snapshot.Providers["binance"]
	
	// Verify latency metrics are populated
	if binanceHealth.Latency.Avg == 0 {
		t.Error("Expected non-zero average latency")
	}
	
	if binanceHealth.Latency.Max != 300*time.Millisecond {
		t.Errorf("Expected max latency 300ms, got %v", binanceHealth.Latency.Max)
	}
	
	if binanceHealth.Latency.P99 == 0 {
		t.Error("Expected non-zero P99 latency")
	}
}

func TestLatencyTracker_Metrics(t *testing.T) {
	tracker := &LatencyTracker{}
	
	// Add samples in known order
	samples := []time.Duration{
		100 * time.Millisecond, // P50 will be around here
		200 * time.Millisecond,
		300 * time.Millisecond,
		400 * time.Millisecond,
		500 * time.Millisecond, // P99 will be around here
	}
	
	for _, sample := range samples {
		tracker.addSample(sample)
	}
	
	metrics := tracker.getMetrics()
	
	if metrics.Max != 500*time.Millisecond {
		t.Errorf("Expected max 500ms, got %v", metrics.Max)
	}
	
	expectedAvg := 300 * time.Millisecond
	if metrics.Avg != expectedAvg {
		t.Errorf("Expected avg %v, got %v", expectedAvg, metrics.Avg)
	}
	
	// P50 should be the median (300ms)
	if metrics.P50 != 300*time.Millisecond {
		t.Errorf("Expected P50 300ms, got %v", metrics.P50)
	}
}

func TestLatencyTracker_SampleLimit(t *testing.T) {
	tracker := &LatencyTracker{}
	
	// Add more than 1000 samples
	for i := 0; i < 1200; i++ {
		tracker.addSample(time.Duration(i) * time.Millisecond)
	}
	
	tracker.mu.RLock()
	sampleCount := len(tracker.samples)
	tracker.mu.RUnlock()
	
	if sampleCount > 1000 {
		t.Errorf("Expected sample count <= 1000, got %d", sampleCount)
	}
}

func TestHealthManager_GetHealthJSON(t *testing.T) {
	pm := NewProviderManager()
	cm := NewCacheManager()
	circm := NewCircuitManager()
	hm := NewHealthManager(pm, cm, circm)
	
	jsonStr, err := hm.GetHealthJSON()
	if err != nil {
		t.Fatalf("Failed to get health JSON: %v", err)
	}
	
	// Verify it's valid JSON
	var snapshot HealthSnapshot
	err = json.Unmarshal([]byte(jsonStr), &snapshot)
	if err != nil {
		t.Fatalf("Failed to parse health JSON: %v", err)
	}
	
	// Verify structure
	if len(snapshot.Providers) == 0 {
		t.Error("Expected providers in JSON")
	}
}

func TestHealthManager_GetHealthSummary(t *testing.T) {
	pm := NewProviderManager()
	cm := NewCacheManager()
	circm := NewCircuitManager()
	hm := NewHealthManager(pm, cm, circm)
	
	summary := hm.GetHealthSummary()
	
	if summary == "" {
		t.Error("Expected non-empty health summary")
	}
	
	// Should contain key health indicators
	expectedParts := []string{"Health:", "Providers:", "Circuits:", "Cache:", "Latency P99:"}
	for _, part := range expectedParts {
		if !strings.Contains(summary, part) {
			t.Errorf("Expected summary to contain '%s', got: %s", part, summary)
		}
	}
}

func TestHealthManager_IsHealthy(t *testing.T) {
	pm := NewProviderManager()
	cm := NewCacheManager()
	circm := NewCircuitManager()
	hm := NewHealthManager(pm, cm, circm)
	
	// Should be healthy initially
	if !hm.IsHealthy() {
		t.Error("Expected system to be healthy initially")
	}
	
	// Open all circuits to make it unhealthy
	for provider := range DefaultProviders {
		circm.ForceOpen(provider)
	}
	
	// Should be unhealthy now
	if hm.IsHealthy() {
		t.Error("Expected system to be unhealthy after opening circuits")
	}
}

func TestHealthManager_CalculateOverallHealth(t *testing.T) {
	pm := NewProviderManager()
	cm := NewCacheManager()
	circm := NewCircuitManager()
	hm := NewHealthManager(pm, cm, circm)
	
	testCases := []struct {
		name     string
		summary  HealthSummary
		expected string
	}{
		{
			name: "healthy system",
			summary: HealthSummary{
				ProvidersHealthy:  4,
				ProvidersTotal:    5,
				CircuitsClosed:    4,
				CircuitsTotal:     5,
				CacheHitRate:      85.0,
				OverallLatencyP99: 2 * time.Second,
			},
			expected: "healthy",
		},
		{
			name: "degraded system",
			summary: HealthSummary{
				ProvidersHealthy:  2,
				ProvidersTotal:    5,
				CircuitsClosed:    2,
				CircuitsTotal:     5,
				CacheHitRate:      60.0,
				OverallLatencyP99: 5 * time.Second,
			},
			expected: "degraded",
		},
		{
			name: "unhealthy system",
			summary: HealthSummary{
				ProvidersHealthy:  1,
				ProvidersTotal:    5,
				CircuitsClosed:    1,
				CircuitsTotal:     5,
				CacheHitRate:      30.0,
				OverallLatencyP99: 15 * time.Second,
			},
			expected: "unhealthy",
		},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := hm.calculateOverallHealth(tc.summary)
			if result != tc.expected {
				t.Errorf("Expected %s, got %s", tc.expected, result)
			}
		})
	}
}

func TestProviderHealth_Status(t *testing.T) {
	pm := NewProviderManager()
	cm := NewCacheManager()
	circm := NewCircuitManager()
	hm := NewHealthManager(pm, cm, circm)
	
	// Test healthy provider
	snapshot := hm.GetHealthSnapshot()
	binanceHealth := snapshot.Providers["binance"]
	
	// Should be healthy initially (circuit closed, high health %)
	if binanceHealth.Status != "healthy" {
		t.Errorf("Expected healthy status, got %s", binanceHealth.Status)
	}
	
	// Open circuit to make it degraded/unhealthy
	circm.ForceOpen("binance")
	
	snapshot = hm.GetHealthSnapshot()
	binanceHealth = snapshot.Providers["binance"]
	
	// Should not be healthy now
	if binanceHealth.Status == "healthy" {
		t.Error("Expected unhealthy status after opening circuit")
	}
}

func TestHealthManager_ConcurrentAccess(t *testing.T) {
	pm := NewProviderManager()
	cm := NewCacheManager()
	circm := NewCircuitManager()
	hm := NewHealthManager(pm, cm, circm)
	
	// Test concurrent access doesn't cause panics
	done := make(chan bool)
	
	for i := 0; i < 10; i++ {
		go func(id int) {
			for j := 0; j < 50; j++ {
				hm.RecordLatency("binance", time.Duration(j)*time.Millisecond)
				hm.GetHealthSnapshot()
				hm.GetHealthSummary()
				hm.IsHealthy()
			}
			done <- true
		}(i)
	}
	
	// Wait for all goroutines to complete
	for i := 0; i < 10; i++ {
		<-done
	}
}

func TestHealthSnapshot_Serialization(t *testing.T) {
	pm := NewProviderManager()
	cm := NewCacheManager()
	circm := NewCircuitManager()
	hm := NewHealthManager(pm, cm, circm)
	
	// Add some data
	pm.RecordRequest("binance", 1)
	hm.RecordLatency("binance", 100*time.Millisecond)
	
	snapshot := hm.GetHealthSnapshot()
	
	// Serialize to JSON
	data, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatalf("Failed to marshal snapshot: %v", err)
	}
	
	// Deserialize back
	var restored HealthSnapshot
	err = json.Unmarshal(data, &restored)
	if err != nil {
		t.Fatalf("Failed to unmarshal snapshot: %v", err)
	}
	
	// Verify key fields are preserved
	if restored.OverallHealth != snapshot.OverallHealth {
		t.Errorf("Expected overall health %s, got %s", snapshot.OverallHealth, restored.OverallHealth)
	}
	
	if len(restored.Providers) != len(snapshot.Providers) {
		t.Errorf("Expected %d providers, got %d", len(snapshot.Providers), len(restored.Providers))
	}
}