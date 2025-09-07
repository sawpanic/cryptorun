// Metrics aggregation system for microstructure collectors
// Provides 1s aggregation windows and 60s rolling statistics
package micro

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// MetricsAggregator handles metric collection and aggregation across all venues
type MetricsAggregator struct {
	collectors []Collector

	// Aggregated metrics
	metricsMutex sync.RWMutex
	venueMetrics map[string]*CollectorMetrics // venue -> latest metrics

	// Health badges for CLI
	healthMutex  sync.RWMutex
	healthBadges map[string]HealthStatus // venue -> health status

	// Rolling statistics (60s windows)
	rollingMutex sync.RWMutex
	rollingStats map[string]*RollingStats // venue -> 60s stats

	// Context and control
	ctx     context.Context
	cancel  context.CancelFunc
	wg      sync.WaitGroup
	running bool

	// Tickers for aggregation
	metricsUpdateTicker *time.Ticker
	healthUpdateTicker  *time.Ticker
	rollingStatsTicker  *time.Ticker
}

// RollingStats tracks 60-second rolling statistics per venue
type RollingStats struct {
	Venue       string    `json:"venue"`
	WindowStart time.Time `json:"window_start"`
	WindowEnd   time.Time `json:"window_end"`

	// Message throughput
	TotalL1Messages    int64   `json:"total_l1_messages"`
	TotalL2Messages    int64   `json:"total_l2_messages"`
	TotalErrorMessages int64   `json:"total_error_messages"`
	MessagesPerSecond  float64 `json:"messages_per_second"`

	// Latency statistics
	AvgLatencyMs float64 `json:"avg_latency_ms"`
	P50LatencyMs int64   `json:"p50_latency_ms"`
	P95LatencyMs int64   `json:"p95_latency_ms"`
	P99LatencyMs int64   `json:"p99_latency_ms"`

	// Error rates
	ErrorRate      float64 `json:"error_rate"`      // % of messages that were errors
	StaleDataRate  float64 `json:"stale_data_rate"` // % of messages with stale data
	IncompleteRate float64 `json:"incomplete_rate"` // % of incomplete messages

	// Quality metrics
	AvgQualityScore float64 `json:"avg_quality_score"` // Average quality score
	HealthScore     float64 `json:"health_score"`      // Overall health score (0-100)
	DataFreshnessMs int64   `json:"data_freshness_ms"` // Average data freshness

	// Sequence analysis
	SequenceGapRate float64 `json:"sequence_gap_rate"` // Rate of sequence gaps
	WSReconnects    int     `json:"ws_reconnects"`     // WebSocket reconnects in window
}

// AggregatedHealthReport provides cross-venue health summary
type AggregatedHealthReport struct {
	Timestamp         time.Time                    `json:"timestamp"`
	TotalVenues       int                          `json:"total_venues"`
	HealthyVenues     int                          `json:"healthy_venues"`
	OverallHealthRate float64                      `json:"overall_health_rate"`
	VenueHealth       map[string]*VenueHealth      `json:"venue_health"`
	VenueMetrics      map[string]*CollectorMetrics `json:"venue_metrics"`
	RollingStats      map[string]*RollingStats     `json:"rolling_stats"`
	HealthBadges      map[string]HealthStatus      `json:"health_badges"`
}

// NewMetricsAggregator creates a new metrics aggregator
func NewMetricsAggregator(collectors []Collector) *MetricsAggregator {
	return &MetricsAggregator{
		collectors:   collectors,
		venueMetrics: make(map[string]*CollectorMetrics),
		healthBadges: make(map[string]HealthStatus),
		rollingStats: make(map[string]*RollingStats),
	}
}

// Start begins metrics aggregation
func (ma *MetricsAggregator) Start(ctx context.Context) error {
	ma.ctx, ma.cancel = context.WithCancel(ctx)
	ma.running = true

	// Initialize health badges
	ma.healthMutex.Lock()
	for _, collector := range ma.collectors {
		ma.healthBadges[collector.Venue()] = HealthGreen
	}
	ma.healthMutex.Unlock()

	// Start tickers
	ma.metricsUpdateTicker = time.NewTicker(1 * time.Second) // Update metrics every 1s
	ma.healthUpdateTicker = time.NewTicker(5 * time.Second)  // Update health every 5s
	ma.rollingStatsTicker = time.NewTicker(60 * time.Second) // Update rolling stats every 60s

	// Start background workers
	ma.wg.Add(3)
	go ma.metricsUpdateWorker()
	go ma.healthUpdateWorker()
	go ma.rollingStatsWorker()

	return nil
}

// Stop gracefully shuts down the metrics aggregator
func (ma *MetricsAggregator) Stop(ctx context.Context) error {
	if !ma.running {
		return nil
	}

	// Signal shutdown
	ma.cancel()

	// Stop tickers
	if ma.metricsUpdateTicker != nil {
		ma.metricsUpdateTicker.Stop()
	}
	if ma.healthUpdateTicker != nil {
		ma.healthUpdateTicker.Stop()
	}
	if ma.rollingStatsTicker != nil {
		ma.rollingStatsTicker.Stop()
	}

	// Wait for workers to finish
	ma.wg.Wait()

	ma.running = false
	return nil
}

// GetAggregatedReport returns a comprehensive health and metrics report
func (ma *MetricsAggregator) GetAggregatedReport() *AggregatedHealthReport {
	ma.metricsMutex.RLock()
	ma.healthMutex.RLock()
	ma.rollingMutex.RLock()
	defer ma.metricsMutex.RUnlock()
	defer ma.healthMutex.RUnlock()
	defer ma.rollingMutex.RUnlock()

	// Collect venue health
	venueHealth := make(map[string]*VenueHealth)
	healthyVenues := 0

	for _, collector := range ma.collectors {
		venue := collector.Venue()
		if health, err := collector.GetVenueHealth(); err == nil {
			venueHealth[venue] = health
			if health.Healthy {
				healthyVenues++
			}
		}
	}

	// Copy current metrics
	venueMetrics := make(map[string]*CollectorMetrics)
	for venue, metrics := range ma.venueMetrics {
		metricsCopy := *metrics
		venueMetrics[venue] = &metricsCopy
	}

	// Copy rolling stats
	rollingStats := make(map[string]*RollingStats)
	for venue, stats := range ma.rollingStats {
		statsCopy := *stats
		rollingStats[venue] = &statsCopy
	}

	// Copy health badges
	healthBadges := make(map[string]HealthStatus)
	for venue, badge := range ma.healthBadges {
		healthBadges[venue] = badge
	}

	// Calculate overall health rate
	overallHealthRate := 0.0
	if len(ma.collectors) > 0 {
		overallHealthRate = float64(healthyVenues) / float64(len(ma.collectors))
	}

	return &AggregatedHealthReport{
		Timestamp:         time.Now(),
		TotalVenues:       len(ma.collectors),
		HealthyVenues:     healthyVenues,
		OverallHealthRate: overallHealthRate,
		VenueHealth:       venueHealth,
		VenueMetrics:      venueMetrics,
		RollingStats:      rollingStats,
		HealthBadges:      healthBadges,
	}
}

// GetHealthBadges returns current health badges for CLI display
func (ma *MetricsAggregator) GetHealthBadges() map[string]HealthStatus {
	ma.healthMutex.RLock()
	defer ma.healthMutex.RUnlock()

	badges := make(map[string]HealthStatus)
	for venue, badge := range ma.healthBadges {
		badges[venue] = badge
	}
	return badges
}

// GetVenueMetrics returns metrics for a specific venue
func (ma *MetricsAggregator) GetVenueMetrics(venue string) (*CollectorMetrics, error) {
	ma.metricsMutex.RLock()
	defer ma.metricsMutex.RUnlock()

	metrics, exists := ma.venueMetrics[venue]
	if !exists {
		return nil, fmt.Errorf("no metrics available for venue %s", venue)
	}

	// Return a copy
	metricsCopy := *metrics
	return &metricsCopy, nil
}

// GetRollingStats returns 60s rolling stats for a specific venue
func (ma *MetricsAggregator) GetRollingStats(venue string) (*RollingStats, error) {
	ma.rollingMutex.RLock()
	defer ma.rollingMutex.RUnlock()

	stats, exists := ma.rollingStats[venue]
	if !exists {
		return nil, fmt.Errorf("no rolling stats available for venue %s", venue)
	}

	// Return a copy
	statsCopy := *stats
	return &statsCopy, nil
}

// metricsUpdateWorker updates metrics from all collectors every 1s
func (ma *MetricsAggregator) metricsUpdateWorker() {
	defer ma.wg.Done()

	for {
		select {
		case <-ma.ctx.Done():
			return
		case <-ma.metricsUpdateTicker.C:
			ma.updateMetricsFromCollectors()
		}
	}
}

// healthUpdateWorker updates health badges every 5s
func (ma *MetricsAggregator) healthUpdateWorker() {
	defer ma.wg.Done()

	for {
		select {
		case <-ma.ctx.Done():
			return
		case <-ma.healthUpdateTicker.C:
			ma.updateHealthBadges()
		}
	}
}

// rollingStatsWorker calculates 60s rolling statistics
func (ma *MetricsAggregator) rollingStatsWorker() {
	defer ma.wg.Done()

	for {
		select {
		case <-ma.ctx.Done():
			return
		case <-ma.rollingStatsTicker.C:
			ma.calculateRollingStats()
		}
	}
}

// updateMetricsFromCollectors fetches latest metrics from all collectors
func (ma *MetricsAggregator) updateMetricsFromCollectors() {
	ma.metricsMutex.Lock()
	defer ma.metricsMutex.Unlock()

	for _, collector := range ma.collectors {
		venue := collector.Venue()
		if metrics, err := collector.GetMetrics(); err == nil {
			ma.venueMetrics[venue] = metrics
		}
	}
}

// updateHealthBadges updates health status badges based on current venue health
func (ma *MetricsAggregator) updateHealthBadges() {
	ma.healthMutex.Lock()
	defer ma.healthMutex.Unlock()

	for _, collector := range ma.collectors {
		venue := collector.Venue()
		if health, err := collector.GetVenueHealth(); err == nil {
			ma.healthBadges[venue] = health.Status
		}
	}
}

// calculateRollingStats calculates 60-second rolling statistics
func (ma *MetricsAggregator) calculateRollingStats() {
	ma.rollingMutex.Lock()
	defer ma.rollingMutex.Unlock()

	now := time.Now()
	windowStart := now.Add(-60 * time.Second)

	// Get current metrics for calculation base
	ma.metricsMutex.RLock()
	currentMetrics := make(map[string]*CollectorMetrics)
	for venue, metrics := range ma.venueMetrics {
		metricsCopy := *metrics
		currentMetrics[venue] = &metricsCopy
	}
	ma.metricsMutex.RUnlock()

	// Calculate rolling stats for each venue
	for venue, metrics := range currentMetrics {
		stats := ma.calculateVenueRollingStats(venue, windowStart, now, metrics)
		ma.rollingStats[venue] = stats
	}
}

// calculateVenueRollingStats calculates rolling stats for a single venue
func (ma *MetricsAggregator) calculateVenueRollingStats(venue string, windowStart, windowEnd time.Time, metrics *CollectorMetrics) *RollingStats {
	// This is a simplified implementation - in production, you'd aggregate
	// data from a time series database or maintain sliding windows

	windowDurationSec := windowEnd.Sub(windowStart).Seconds()

	// Estimate messages per second
	messagesPerSecond := float64(metrics.L1Messages+metrics.L2Messages) / windowDurationSec

	// Calculate error rate
	totalMessages := metrics.L1Messages + metrics.L2Messages + metrics.ErrorMessages
	errorRate := 0.0
	if totalMessages > 0 {
		errorRate = float64(metrics.ErrorMessages) / float64(totalMessages)
	}

	// Calculate stale data rate (approximated)
	staleDataRate := float64(metrics.StaleDataCount) / float64(totalMessages)
	if totalMessages == 0 {
		staleDataRate = 0
	}

	// Calculate incomplete rate
	incompleteRate := float64(metrics.IncompleteCount) / float64(totalMessages)
	if totalMessages == 0 {
		incompleteRate = 0
	}

	// Calculate health score (0-100)
	healthScore := metrics.QualityScore

	// Latency statistics (approximated from current metrics)
	p50LatencyMs := int64(metrics.AvgLatencyMs * 0.7) // Approximation
	p95LatencyMs := int64(metrics.AvgLatencyMs * 1.5) // Approximation
	p99LatencyMs := metrics.MaxLatencyMs

	return &RollingStats{
		Venue:              venue,
		WindowStart:        windowStart,
		WindowEnd:          windowEnd,
		TotalL1Messages:    metrics.L1Messages,
		TotalL2Messages:    metrics.L2Messages,
		TotalErrorMessages: metrics.ErrorMessages,
		MessagesPerSecond:  messagesPerSecond,
		AvgLatencyMs:       metrics.AvgLatencyMs,
		P50LatencyMs:       p50LatencyMs,
		P95LatencyMs:       p95LatencyMs,
		P99LatencyMs:       p99LatencyMs,
		ErrorRate:          errorRate,
		StaleDataRate:      staleDataRate,
		IncompleteRate:     incompleteRate,
		AvgQualityScore:    metrics.QualityScore,
		HealthScore:        healthScore,
		DataFreshnessMs:    2000, // Approximation - 2s average freshness
		SequenceGapRate:    0.01, // Approximation - 1% sequence gap rate
		WSReconnects:       0,    // Would need to track from collectors
	}
}

// FormatHealthBadge returns a formatted health badge for CLI display
func FormatHealthBadge(status HealthStatus) string {
	switch status {
	case HealthRed:
		return "🔴 RED"
	case HealthYellow:
		return "🟡 YELLOW"
	case HealthGreen:
		return "🟢 GREEN"
	default:
		return "⚪ UNKNOWN"
	}
}

// GetHealthSummary returns a summary string of all venue health
func (ma *MetricsAggregator) GetHealthSummary() string {
	badges := ma.GetHealthBadges()

	red, yellow, green := 0, 0, 0
	for _, badge := range badges {
		switch badge {
		case HealthRed:
			red++
		case HealthYellow:
			yellow++
		case HealthGreen:
			green++
		}
	}

	total := len(badges)
	if total == 0 {
		return "No venues monitored"
	}

	return fmt.Sprintf("Venues: %d🟢 %d🟡 %d🔴 (Total: %d)", green, yellow, red, total)
}
