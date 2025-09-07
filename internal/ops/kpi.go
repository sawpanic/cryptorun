package ops

import (
	"sync"
	"time"
)

// KPITracker tracks rolling operational KPIs
type KPITracker struct {
	mu sync.RWMutex

	// Request tracking
	requestTimes  []time.Time
	requestWindow time.Duration

	// Error tracking
	errorTimes    []time.Time
	errorWindow   time.Duration
	totalRequests int

	// Cache tracking
	cacheHits   []time.Time
	cacheMisses []time.Time
	cacheWindow time.Duration

	// Provider breaker tracking
	openBreakers map[string]bool

	// Venue health tracking
	venueHealth map[string]VenueHealthStatus
}

// VenueHealthStatus represents health metrics for a venue
type VenueHealthStatus struct {
	IsHealthy     bool
	UptimePercent float64
	LatencyMs     int64
	DepthUSD      float64
	SpreadBps     float64
	LastUpdate    time.Time
}

// KPIMetrics represents current KPI values
type KPIMetrics struct {
	RequestsPerMinute   float64
	ErrorRatePercent    float64
	CacheHitRatePercent float64
	OpenBreakerCount    int
	UnhealthyVenueCount int
	HealthyVenueCount   int
}

// NewKPITracker creates a new KPI tracker
func NewKPITracker(requestWindow, errorWindow, cacheWindow time.Duration) *KPITracker {
	return &KPITracker{
		requestWindow: requestWindow,
		errorWindow:   errorWindow,
		cacheWindow:   cacheWindow,
		openBreakers:  make(map[string]bool),
		venueHealth:   make(map[string]VenueHealthStatus),
	}
}

// RecordRequest records a successful request
func (k *KPITracker) RecordRequest() {
	k.mu.Lock()
	defer k.mu.Unlock()

	now := time.Now()
	k.requestTimes = append(k.requestTimes, now)
	k.totalRequests++

	// Clean old entries
	k.cleanOldRequests(now)
}

// RecordError records a failed request
func (k *KPITracker) RecordError() {
	k.mu.Lock()
	defer k.mu.Unlock()

	now := time.Now()
	k.errorTimes = append(k.errorTimes, now)
	k.totalRequests++

	// Clean old entries
	k.cleanOldErrors(now)
}

// RecordCacheHit records a cache hit
func (k *KPITracker) RecordCacheHit() {
	k.mu.Lock()
	defer k.mu.Unlock()

	now := time.Now()
	k.cacheHits = append(k.cacheHits, now)

	// Clean old entries
	k.cleanOldCacheEntries(now)
}

// RecordCacheMiss records a cache miss
func (k *KPITracker) RecordCacheMiss() {
	k.mu.Lock()
	defer k.mu.Unlock()

	now := time.Now()
	k.cacheMisses = append(k.cacheMisses, now)

	// Clean old entries
	k.cleanOldCacheEntries(now)
}

// SetBreakerOpen marks a circuit breaker as open
func (k *KPITracker) SetBreakerOpen(provider string, isOpen bool) {
	k.mu.Lock()
	defer k.mu.Unlock()

	if isOpen {
		k.openBreakers[provider] = true
	} else {
		delete(k.openBreakers, provider)
	}
}

// UpdateVenueHealth updates health status for a venue
func (k *KPITracker) UpdateVenueHealth(venue string, health VenueHealthStatus) {
	k.mu.Lock()
	defer k.mu.Unlock()

	health.LastUpdate = time.Now()
	k.venueHealth[venue] = health
}

// GetMetrics returns current KPI metrics
func (k *KPITracker) GetMetrics() KPIMetrics {
	k.mu.RLock()
	defer k.mu.RUnlock()

	now := time.Now()

	// Calculate requests per minute
	requestsPerMinute := float64(len(k.requestTimes)) * 60.0 / k.requestWindow.Seconds()

	// Calculate error rate
	totalErrorWindow := len(k.errorTimes) + len(k.requestTimes)
	errorRate := float64(0)
	if totalErrorWindow > 0 {
		errorRate = float64(len(k.errorTimes)) / float64(totalErrorWindow) * 100.0
	}

	// Calculate cache hit rate
	totalCacheOps := len(k.cacheHits) + len(k.cacheMisses)
	cacheHitRate := float64(0)
	if totalCacheOps > 0 {
		cacheHitRate = float64(len(k.cacheHits)) / float64(totalCacheOps) * 100.0
	}

	// Count venue health
	healthyVenues := 0
	unhealthyVenues := 0
	for _, health := range k.venueHealth {
		// Consider venue stale if no update in last 5 minutes
		if now.Sub(health.LastUpdate) > 5*time.Minute {
			unhealthyVenues++
			continue
		}

		if health.IsHealthy {
			healthyVenues++
		} else {
			unhealthyVenues++
		}
	}

	return KPIMetrics{
		RequestsPerMinute:   requestsPerMinute,
		ErrorRatePercent:    errorRate,
		CacheHitRatePercent: cacheHitRate,
		OpenBreakerCount:    len(k.openBreakers),
		UnhealthyVenueCount: unhealthyVenues,
		HealthyVenueCount:   healthyVenues,
	}
}

// GetOpenBreakers returns list of open circuit breakers
func (k *KPITracker) GetOpenBreakers() []string {
	k.mu.RLock()
	defer k.mu.RUnlock()

	breakers := make([]string, 0, len(k.openBreakers))
	for provider := range k.openBreakers {
		breakers = append(breakers, provider)
	}
	return breakers
}

// GetVenueHealth returns venue health status
func (k *KPITracker) GetVenueHealth() map[string]VenueHealthStatus {
	k.mu.RLock()
	defer k.mu.RUnlock()

	// Return copy to avoid race conditions
	health := make(map[string]VenueHealthStatus)
	for venue, status := range k.venueHealth {
		health[venue] = status
	}
	return health
}

// cleanOldRequests removes request times outside the window
func (k *KPITracker) cleanOldRequests(now time.Time) {
	cutoff := now.Add(-k.requestWindow)
	newTimes := k.requestTimes[:0]
	for _, t := range k.requestTimes {
		if t.After(cutoff) {
			newTimes = append(newTimes, t)
		}
	}
	k.requestTimes = newTimes
}

// cleanOldErrors removes error times outside the window
func (k *KPITracker) cleanOldErrors(now time.Time) {
	cutoff := now.Add(-k.errorWindow)
	newTimes := k.errorTimes[:0]
	for _, t := range k.errorTimes {
		if t.After(cutoff) {
			newTimes = append(newTimes, t)
		}
	}
	k.errorTimes = newTimes
}

// cleanOldCacheEntries removes cache entries outside the window
func (k *KPITracker) cleanOldCacheEntries(now time.Time) {
	cutoff := now.Add(-k.cacheWindow)

	// Clean hits
	newHits := k.cacheHits[:0]
	for _, t := range k.cacheHits {
		if t.After(cutoff) {
			newHits = append(newHits, t)
		}
	}
	k.cacheHits = newHits

	// Clean misses
	newMisses := k.cacheMisses[:0]
	for _, t := range k.cacheMisses {
		if t.After(cutoff) {
			newMisses = append(newMisses, t)
		}
	}
	k.cacheMisses = newMisses
}
