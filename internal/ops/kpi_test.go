package ops

import (
	"testing"
	"time"
)

func TestKPITracker_RequestTracking(t *testing.T) {
	tracker := NewKPITracker(60*time.Second, 300*time.Second, 300*time.Second)

	// Record some requests
	for i := 0; i < 10; i++ {
		tracker.RecordRequest()
	}

	metrics := tracker.GetMetrics()

	// Should calculate requests per minute correctly
	expectedRate := float64(10) * 60.0 / 60.0 // 10 requests in 60 second window
	if metrics.RequestsPerMinute != expectedRate {
		t.Errorf("Expected requests per minute %.1f, got %.1f", expectedRate, metrics.RequestsPerMinute)
	}
}

func TestKPITracker_ErrorRateCalculation(t *testing.T) {
	tracker := NewKPITracker(60*time.Second, 300*time.Second, 300*time.Second)

	// Record 8 successful requests and 2 errors
	for i := 0; i < 8; i++ {
		tracker.RecordRequest()
	}
	for i := 0; i < 2; i++ {
		tracker.RecordError()
	}

	metrics := tracker.GetMetrics()

	// Error rate should be 20% (2 errors out of 10 total)
	expectedErrorRate := 20.0
	if metrics.ErrorRatePercent != expectedErrorRate {
		t.Errorf("Expected error rate %.1f%%, got %.1f%%", expectedErrorRate, metrics.ErrorRatePercent)
	}
}

func TestKPITracker_CacheHitRateCalculation(t *testing.T) {
	tracker := NewKPITracker(60*time.Second, 300*time.Second, 300*time.Second)

	// Record 7 cache hits and 3 misses
	for i := 0; i < 7; i++ {
		tracker.RecordCacheHit()
	}
	for i := 0; i < 3; i++ {
		tracker.RecordCacheMiss()
	}

	metrics := tracker.GetMetrics()

	// Cache hit rate should be 70% (7 hits out of 10 total)
	expectedHitRate := 70.0
	if metrics.CacheHitRatePercent != expectedHitRate {
		t.Errorf("Expected cache hit rate %.1f%%, got %.1f%%", expectedHitRate, metrics.CacheHitRatePercent)
	}
}

func TestKPITracker_BreakerTracking(t *testing.T) {
	tracker := NewKPITracker(60*time.Second, 300*time.Second, 300*time.Second)

	// Set some breakers open
	tracker.SetBreakerOpen("provider1", true)
	tracker.SetBreakerOpen("provider2", true)

	metrics := tracker.GetMetrics()

	if metrics.OpenBreakerCount != 2 {
		t.Errorf("Expected 2 open breakers, got %d", metrics.OpenBreakerCount)
	}

	// Close one breaker
	tracker.SetBreakerOpen("provider1", false)

	metrics = tracker.GetMetrics()
	if metrics.OpenBreakerCount != 1 {
		t.Errorf("Expected 1 open breaker after closing one, got %d", metrics.OpenBreakerCount)
	}
}

func TestKPITracker_VenueHealthTracking(t *testing.T) {
	tracker := NewKPITracker(60*time.Second, 300*time.Second, 300*time.Second)

	// Add healthy venues
	tracker.UpdateVenueHealth("venue1", VenueHealthStatus{
		IsHealthy:     true,
		UptimePercent: 99.5,
		LatencyMs:     100,
		DepthUSD:      50000,
		SpreadBps:     10.5,
	})

	tracker.UpdateVenueHealth("venue2", VenueHealthStatus{
		IsHealthy:     false,
		UptimePercent: 85.0,
		LatencyMs:     5000,
		DepthUSD:      20000,
		SpreadBps:     150.0,
	})

	metrics := tracker.GetMetrics()

	if metrics.HealthyVenueCount != 1 {
		t.Errorf("Expected 1 healthy venue, got %d", metrics.HealthyVenueCount)
	}

	if metrics.UnhealthyVenueCount != 1 {
		t.Errorf("Expected 1 unhealthy venue, got %d", metrics.UnhealthyVenueCount)
	}
}

func TestKPITracker_WindowCleanup(t *testing.T) {
	tracker := NewKPITracker(1*time.Second, 1*time.Second, 1*time.Second)

	// Record some requests
	tracker.RecordRequest()
	tracker.RecordRequest()

	metrics := tracker.GetMetrics()
	if metrics.RequestsPerMinute != 120.0 { // 2 requests in 1 second = 120/min
		t.Errorf("Expected 120 requests/min initially, got %.1f", metrics.RequestsPerMinute)
	}

	// Wait for window to expire
	time.Sleep(1100 * time.Millisecond)

	// Record one more request to trigger cleanup
	tracker.RecordRequest()

	metrics = tracker.GetMetrics()
	// Should only count the latest request now
	if metrics.RequestsPerMinute != 60.0 { // 1 request in 1 second = 60/min
		t.Errorf("Expected 60 requests/min after cleanup, got %.1f", metrics.RequestsPerMinute)
	}
}

func TestKPITracker_EmptyMetrics(t *testing.T) {
	tracker := NewKPITracker(60*time.Second, 300*time.Second, 300*time.Second)

	metrics := tracker.GetMetrics()

	// All metrics should be zero initially
	if metrics.RequestsPerMinute != 0.0 {
		t.Errorf("Expected 0 requests/min initially, got %.1f", metrics.RequestsPerMinute)
	}

	if metrics.ErrorRatePercent != 0.0 {
		t.Errorf("Expected 0%% error rate initially, got %.1f%%", metrics.ErrorRatePercent)
	}

	if metrics.CacheHitRatePercent != 0.0 {
		t.Errorf("Expected 0%% cache hit rate initially, got %.1f%%", metrics.CacheHitRatePercent)
	}

	if metrics.OpenBreakerCount != 0 {
		t.Errorf("Expected 0 open breakers initially, got %d", metrics.OpenBreakerCount)
	}
}

func TestKPITracker_GetOpenBreakers(t *testing.T) {
	tracker := NewKPITracker(60*time.Second, 300*time.Second, 300*time.Second)

	// Set some breakers
	tracker.SetBreakerOpen("kraken", true)
	tracker.SetBreakerOpen("binance", true)
	tracker.SetBreakerOpen("coinbase", false) // This should not appear in the list

	breakers := tracker.GetOpenBreakers()

	if len(breakers) != 2 {
		t.Errorf("Expected 2 open breakers, got %d", len(breakers))
	}

	// Check that the right providers are in the list
	breakerMap := make(map[string]bool)
	for _, provider := range breakers {
		breakerMap[provider] = true
	}

	if !breakerMap["kraken"] {
		t.Error("Expected kraken to be in open breakers list")
	}

	if !breakerMap["binance"] {
		t.Error("Expected binance to be in open breakers list")
	}

	if breakerMap["coinbase"] {
		t.Error("Did not expect coinbase to be in open breakers list")
	}
}

func TestKPITracker_GetVenueHealth(t *testing.T) {
	tracker := NewKPITracker(60*time.Second, 300*time.Second, 300*time.Second)

	expectedHealth := VenueHealthStatus{
		IsHealthy:     true,
		UptimePercent: 98.5,
		LatencyMs:     250,
		DepthUSD:      75000,
		SpreadBps:     15.2,
	}

	tracker.UpdateVenueHealth("test_venue", expectedHealth)

	healthMap := tracker.GetVenueHealth()

	actualHealth, exists := healthMap["test_venue"]
	if !exists {
		t.Fatal("Expected venue health to exist for test_venue")
	}

	if actualHealth.IsHealthy != expectedHealth.IsHealthy {
		t.Errorf("Expected IsHealthy %v, got %v", expectedHealth.IsHealthy, actualHealth.IsHealthy)
	}

	if actualHealth.UptimePercent != expectedHealth.UptimePercent {
		t.Errorf("Expected UptimePercent %.1f, got %.1f", expectedHealth.UptimePercent, actualHealth.UptimePercent)
	}

	if actualHealth.LatencyMs != expectedHealth.LatencyMs {
		t.Errorf("Expected LatencyMs %d, got %d", expectedHealth.LatencyMs, actualHealth.LatencyMs)
	}
}

func TestKPITracker_StaleVenueHealth(t *testing.T) {
	tracker := NewKPITracker(60*time.Second, 300*time.Second, 300*time.Second)

	// Create a venue health with old timestamp
	oldHealth := VenueHealthStatus{
		IsHealthy:     true,
		UptimePercent: 99.0,
		LastUpdate:    time.Now().Add(-10 * time.Minute), // Very old
	}

	// Manually set the old health (simulating stale data)
	tracker.mu.Lock()
	tracker.venueHealth["stale_venue"] = oldHealth
	tracker.mu.Unlock()

	metrics := tracker.GetMetrics()

	// Stale venue should be counted as unhealthy
	if metrics.UnhealthyVenueCount != 1 {
		t.Errorf("Expected 1 unhealthy venue (stale), got %d", metrics.UnhealthyVenueCount)
	}

	if metrics.HealthyVenueCount != 0 {
		t.Errorf("Expected 0 healthy venues (all stale), got %d", metrics.HealthyVenueCount)
	}
}

func TestKPITracker_ConcurrentAccess(t *testing.T) {
	tracker := NewKPITracker(60*time.Second, 300*time.Second, 300*time.Second)

	done := make(chan bool)

	// Start goroutines that concurrently modify the tracker
	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < 100; j++ {
				tracker.RecordRequest()
				tracker.RecordCacheHit()
				tracker.SetBreakerOpen("test_provider", j%2 == 0)
			}
			done <- true
		}()
	}

	// Wait for all goroutines to complete
	for i := 0; i < 10; i++ {
		<-done
	}

	// Should not panic and should return valid metrics
	metrics := tracker.GetMetrics()

	if metrics.RequestsPerMinute < 0 {
		t.Error("Requests per minute should not be negative")
	}

	if metrics.CacheHitRatePercent < 0 || metrics.CacheHitRatePercent > 100 {
		t.Errorf("Cache hit rate should be between 0-100%%, got %.1f%%", metrics.CacheHitRatePercent)
	}
}
