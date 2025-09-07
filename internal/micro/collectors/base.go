// Package collectors provides exchange-native L1/L2 microstructure data collectors
// with hardened health monitoring and liquidity gradient calculations.
package collectors

import (
	"context"
	"encoding/csv"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/sawpanic/cryptorun/internal/micro"
)

// BaseCollector provides common functionality for all venue collectors
type BaseCollector struct {
	config *micro.CollectorConfig
	venue  string

	// Data storage
	l1DataMutex sync.RWMutex
	l1Data      map[string]*micro.L1Data // symbol -> latest L1 data

	l2DataMutex sync.RWMutex
	l2Data      map[string]*micro.L2Data // symbol -> latest L2 data

	// Health monitoring
	healthMutex   sync.RWMutex
	venueHealth   *micro.VenueHealth
	healthHistory []micro.VenueHealth // Last 100 health points

	// Metrics tracking
	metricsMutex   sync.RWMutex
	metrics        *micro.CollectorMetrics
	metricsHistory []micro.CollectorMetrics // Last 100 metric windows

	// CSV writer for health artifacts
	healthCSVMutex  sync.Mutex
	healthCSVFile   *os.File
	healthCSVWriter *csv.Writer

	// Context and control
	ctx     context.Context
	cancel  context.CancelFunc
	wg      sync.WaitGroup
	running bool

	// Subscriptions
	subscriptionsMutex sync.RWMutex
	subscriptions      map[string]bool // symbol -> subscribed

	// Timing and windows
	lastAggregation    time.Time
	aggregationTicker  *time.Ticker
	rollingStatsTicker *time.Ticker
}

// NewBaseCollector creates a new base collector with common functionality
func NewBaseCollector(config *micro.CollectorConfig) *BaseCollector {
	return &BaseCollector{
		config:        config,
		venue:         config.Venue,
		l1Data:        make(map[string]*micro.L1Data),
		l2Data:        make(map[string]*micro.L2Data),
		subscriptions: make(map[string]bool),
		venueHealth: &micro.VenueHealth{
			Venue:          config.Venue,
			Timestamp:      time.Now(),
			Status:         micro.HealthGreen,
			Healthy:        true,
			Recommendation: "proceed",
		},
		metrics: &micro.CollectorMetrics{
			Venue:       config.Venue,
			WindowStart: time.Now(),
			WindowEnd:   time.Now().Add(time.Duration(config.AggregationWindowMs) * time.Millisecond),
		},
		lastAggregation: time.Now(),
	}
}

// Start begins the base collector operations
func (bc *BaseCollector) Start(ctx context.Context) error {
	bc.ctx, bc.cancel = context.WithCancel(ctx)
	bc.running = true

	// Initialize health CSV if enabled
	if err := bc.initHealthCSV(); err != nil {
		return fmt.Errorf("failed to initialize health CSV: %w", err)
	}

	// Start aggregation ticker (1s windows)
	bc.aggregationTicker = time.NewTicker(time.Duration(bc.config.AggregationWindowMs) * time.Millisecond)

	// Start rolling stats ticker (60s windows)
	bc.rollingStatsTicker = time.NewTicker(time.Duration(bc.config.RollingStatsWindowMs) * time.Millisecond)

	// Start background goroutines
	bc.wg.Add(2)
	go bc.aggregationWorker()
	go bc.rollingStatsWorker()

	return nil
}

// Stop gracefully shuts down the base collector
func (bc *BaseCollector) Stop(ctx context.Context) error {
	if !bc.running {
		return nil
	}

	// Signal shutdown
	bc.cancel()

	// Stop tickers
	if bc.aggregationTicker != nil {
		bc.aggregationTicker.Stop()
	}
	if bc.rollingStatsTicker != nil {
		bc.rollingStatsTicker.Stop()
	}

	// Wait for workers to finish
	bc.wg.Wait()

	// Close health CSV
	if err := bc.closeHealthCSV(); err != nil {
		return fmt.Errorf("failed to close health CSV: %w", err)
	}

	bc.running = false
	return nil
}

// GetL1Data returns the latest L1 data for a symbol
func (bc *BaseCollector) GetL1Data(symbol string) (*micro.L1Data, error) {
	bc.l1DataMutex.RLock()
	defer bc.l1DataMutex.RUnlock()

	data, exists := bc.l1Data[symbol]
	if !exists {
		return nil, fmt.Errorf("no L1 data available for symbol %s", symbol)
	}

	// Update data age
	dataCopy := *data
	dataCopy.DataAge = time.Since(data.Timestamp)

	return &dataCopy, nil
}

// GetL2Data returns the latest L2 data for a symbol
func (bc *BaseCollector) GetL2Data(symbol string) (*micro.L2Data, error) {
	bc.l2DataMutex.RLock()
	defer bc.l2DataMutex.RUnlock()

	data, exists := bc.l2Data[symbol]
	if !exists {
		return nil, fmt.Errorf("no L2 data available for symbol %s", symbol)
	}

	// Update data age
	dataCopy := *data
	dataCopy.DataAge = time.Since(data.Timestamp)

	return &dataCopy, nil
}

// GetVenueHealth returns current venue health status
func (bc *BaseCollector) GetVenueHealth() (*micro.VenueHealth, error) {
	bc.healthMutex.RLock()
	defer bc.healthMutex.RUnlock()

	// Return a copy to prevent external modification
	healthCopy := *bc.venueHealth
	return &healthCopy, nil
}

// GetMetrics returns collector performance metrics
func (bc *BaseCollector) GetMetrics() (*micro.CollectorMetrics, error) {
	bc.metricsMutex.RLock()
	defer bc.metricsMutex.RUnlock()

	// Return a copy to prevent external modification
	metricsCopy := *bc.metrics
	return &metricsCopy, nil
}

// Venue returns the venue name
func (bc *BaseCollector) Venue() string {
	return bc.venue
}

// IsHealthy returns true if venue is currently healthy
func (bc *BaseCollector) IsHealthy() bool {
	bc.healthMutex.RLock()
	defer bc.healthMutex.RUnlock()
	return bc.venueHealth.Healthy
}

// updateL1Data safely updates L1 data for a symbol
func (bc *BaseCollector) updateL1Data(symbol string, data *micro.L1Data) {
	bc.l1DataMutex.Lock()
	defer bc.l1DataMutex.Unlock()

	// Set venue and ensure data age is current
	data.Venue = bc.venue
	data.DataAge = time.Since(data.Timestamp)

	bc.l1Data[symbol] = data
}

// updateL2Data safely updates L2 data for a symbol
func (bc *BaseCollector) updateL2Data(symbol string, data *micro.L2Data) {
	bc.l2DataMutex.Lock()
	defer bc.l2DataMutex.Unlock()

	// Set venue and ensure data age is current
	data.Venue = bc.venue
	data.DataAge = time.Since(data.Timestamp)

	bc.l2Data[symbol] = data
}

// updateVenueHealth safely updates venue health status
func (bc *BaseCollector) updateVenueHealth(health *micro.VenueHealth) {
	bc.healthMutex.Lock()
	defer bc.healthMutex.Unlock()

	// Ensure venue name is set
	health.Venue = bc.venue
	bc.venueHealth = health

	// Add to history (keep last 100)
	bc.healthHistory = append(bc.healthHistory, *health)
	if len(bc.healthHistory) > 100 {
		bc.healthHistory = bc.healthHistory[1:]
	}

	// Write to CSV if enabled
	if bc.config.EnableHealthCSV {
		bc.writeHealthCSV(health)
	}
}

// updateMetrics safely updates collector metrics
func (bc *BaseCollector) updateMetrics(metrics *micro.CollectorMetrics) {
	bc.metricsMutex.Lock()
	defer bc.metricsMutex.Unlock()

	// Ensure venue name is set
	metrics.Venue = bc.venue
	bc.metrics = metrics

	// Add to history (keep last 100)
	bc.metricsHistory = append(bc.metricsHistory, *metrics)
	if len(bc.metricsHistory) > 100 {
		bc.metricsHistory = bc.metricsHistory[1:]
	}
}

// calculateSpreadBps calculates spread in basis points
func (bc *BaseCollector) calculateSpreadBps(bidPrice, askPrice float64) float64 {
	if bidPrice <= 0 || askPrice <= 0 || askPrice <= bidPrice {
		return 0
	}

	midPrice := (bidPrice + askPrice) / 2
	spread := askPrice - bidPrice
	return (spread / midPrice) * 10000 // Convert to basis points
}

// calculateLiquidityGradient calculates the liquidity gradient (depth@0.5% to depth@2% ratio)
func (bc *BaseCollector) calculateLiquidityGradient(depth05Pct, depth2Pct float64) float64 {
	if depth2Pct <= 0 {
		return 0
	}
	return depth05Pct / depth2Pct
}

// assessDataQuality determines data quality based on age and completeness
func (bc *BaseCollector) assessDataQuality(dataAge time.Duration, hasCompleteData bool, sequenceGap bool) micro.DataQuality {
	score := 0

	// Age scoring
	if dataAge < 2*time.Second {
		score += 2
	} else if dataAge < 5*time.Second {
		score += 1
	}

	// Completeness scoring
	if hasCompleteData {
		score += 2
	}

	// Sequence continuity scoring
	if !sequenceGap {
		score += 1
	}

	switch {
	case score >= 4:
		return micro.QualityExcellent
	case score >= 3:
		return micro.QualityGood
	default:
		return micro.QualityDegraded
	}
}

// aggregationWorker runs the 1s aggregation window processing
func (bc *BaseCollector) aggregationWorker() {
	defer bc.wg.Done()

	for {
		select {
		case <-bc.ctx.Done():
			return
		case <-bc.aggregationTicker.C:
			bc.processAggregationWindow()
		}
	}
}

// rollingStatsWorker runs the 60s rolling statistics processing
func (bc *BaseCollector) rollingStatsWorker() {
	defer bc.wg.Done()

	for {
		select {
		case <-bc.ctx.Done():
			return
		case <-bc.rollingStatsTicker.C:
			bc.processRollingStats()
		}
	}
}

// processAggregationWindow processes the current 1s aggregation window
func (bc *BaseCollector) processAggregationWindow() {
	now := time.Now()
	windowStart := bc.lastAggregation
	windowEnd := now

	// Update metrics for this window (to be implemented by specific collectors)
	bc.updateMetricsWindow(windowStart, windowEnd)

	bc.lastAggregation = now
}

// processRollingStats processes 60s rolling statistics
func (bc *BaseCollector) processRollingStats() {
	// Calculate rolling health metrics (to be implemented by specific collectors)
	bc.updateRollingHealthStats()
}

// updateMetricsWindow updates metrics for the current window (base implementation)
func (bc *BaseCollector) updateMetricsWindow(windowStart, windowEnd time.Time) {
	// This is a base implementation - specific collectors should override
	metrics := &micro.CollectorMetrics{
		Venue:       bc.venue,
		WindowStart: windowStart,
		WindowEnd:   windowEnd,
		// Default values - specific collectors will provide real data
		L1Messages:       0,
		L2Messages:       0,
		ErrorMessages:    0,
		ProcessingTimeMs: 0,
		AvgLatencyMs:     0,
		MaxLatencyMs:     0,
		StaleDataCount:   0,
		IncompleteCount:  0,
		QualityScore:     100.0,
	}

	bc.updateMetrics(metrics)
}

// updateRollingHealthStats updates 60s rolling health statistics (base implementation)
func (bc *BaseCollector) updateRollingHealthStats() {
	// This is a base implementation - specific collectors should override
	health := &micro.VenueHealth{
		Venue:            bc.venue,
		Timestamp:        time.Now(),
		Status:           micro.HealthGreen,
		Healthy:          true,
		Uptime:           100.0,
		HeartbeatAgeMs:   1000,
		MessageGapRate:   0.0,
		WSReconnectCount: 0,
		LatencyP50Ms:     50,
		LatencyP99Ms:     100,
		ErrorRate:        0.0,
		DataFreshness:    1 * time.Second,
		DataCompleteness: 100.0,
		Recommendation:   "proceed",
	}

	bc.updateVenueHealth(health)
}

// initHealthCSV initializes the health CSV file and writer
func (bc *BaseCollector) initHealthCSV() error {
	if !bc.config.EnableHealthCSV {
		return nil
	}

	bc.healthCSVMutex.Lock()
	defer bc.healthCSVMutex.Unlock()

	// Create the file (or append if exists)
	file, err := os.OpenFile(bc.config.HealthCSVPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("failed to open health CSV file: %w", err)
	}

	bc.healthCSVFile = file
	bc.healthCSVWriter = csv.NewWriter(file)

	// Write CSV header if file is empty
	fileInfo, err := file.Stat()
	if err != nil {
		return fmt.Errorf("failed to stat health CSV file: %w", err)
	}

	if fileInfo.Size() == 0 {
		header := []string{
			"timestamp", "venue", "status", "healthy", "uptime",
			"heartbeat_age_ms", "message_gap_rate", "ws_reconnect_count",
			"latency_p50_ms", "latency_p99_ms", "error_rate",
			"data_freshness_ms", "data_completeness", "recommendation",
		}
		if err := bc.healthCSVWriter.Write(header); err != nil {
			return fmt.Errorf("failed to write CSV header: %w", err)
		}
		bc.healthCSVWriter.Flush()
	}

	return nil
}

// closeHealthCSV closes the health CSV file and writer
func (bc *BaseCollector) closeHealthCSV() error {
	if !bc.config.EnableHealthCSV {
		return nil
	}

	bc.healthCSVMutex.Lock()
	defer bc.healthCSVMutex.Unlock()

	if bc.healthCSVWriter != nil {
		bc.healthCSVWriter.Flush()
	}

	if bc.healthCSVFile != nil {
		return bc.healthCSVFile.Close()
	}

	return nil
}

// writeHealthCSV writes a health record to the CSV file
func (bc *BaseCollector) writeHealthCSV(health *micro.VenueHealth) {
	if !bc.config.EnableHealthCSV {
		return
	}

	bc.healthCSVMutex.Lock()
	defer bc.healthCSVMutex.Unlock()

	if bc.healthCSVWriter == nil {
		return
	}

	record := health.ToCSVRecord()
	row := []string{
		record.Timestamp,
		record.Venue,
		record.Status,
		record.Healthy,
		fmt.Sprintf("%.2f", record.Uptime),
		fmt.Sprintf("%d", record.HeartbeatAgeMs),
		fmt.Sprintf("%.4f", record.MessageGapRate),
		fmt.Sprintf("%d", record.WSReconnectCount),
		fmt.Sprintf("%d", record.LatencyP50Ms),
		fmt.Sprintf("%d", record.LatencyP99Ms),
		fmt.Sprintf("%.4f", record.ErrorRate),
		fmt.Sprintf("%d", record.DataFreshnessMs),
		fmt.Sprintf("%.2f", record.DataCompleteness),
		record.Recommendation,
	}

	if err := bc.healthCSVWriter.Write(row); err != nil {
		// Log error but don't fail the collector
		fmt.Printf("Failed to write health CSV record: %v\n", err)
	}

	bc.healthCSVWriter.Flush()
}
