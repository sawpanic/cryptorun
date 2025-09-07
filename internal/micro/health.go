// Health monitoring system for microstructure collectors
// Provides venue health tracking with red/yellow/green badges
package micro

import (
	"context"
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// HealthMonitor manages venue health tracking and CSV artifact generation
type HealthMonitor struct {
	collectors []Collector
	config     *HealthMonitorConfig

	// Health tracking
	healthMutex   sync.RWMutex
	healthHistory map[string][]*VenueHealth // venue -> health history

	// CSV writers for artifacts
	csvMutex   sync.Mutex
	csvWriters map[string]*csv.Writer // venue -> CSV writer
	csvFiles   map[string]*os.File    // venue -> CSV file

	// Context and control
	ctx     context.Context
	cancel  context.CancelFunc
	wg      sync.WaitGroup
	running bool

	// Tickers
	healthCheckTicker *time.Ticker
	csvFlushTicker    *time.Ticker
}

// HealthMonitorConfig configures the health monitoring system
type HealthMonitorConfig struct {
	// Monitoring intervals
	HealthCheckIntervalSec int `yaml:"health_check_interval_sec"` // Default: 5
	CSVFlushIntervalSec    int `yaml:"csv_flush_interval_sec"`    // Default: 30

	// History retention
	MaxHistoryPoints int `yaml:"max_history_points"` // Default: 720 (1 hour at 5s intervals)

	// Health thresholds (override collector defaults)
	MaxHeartbeatAgeMs       int64   `yaml:"max_heartbeat_age_ms"`       // Default: 10000
	MaxMessageGapRate       float64 `yaml:"max_message_gap_rate"`       // Default: 0.05
	MaxErrorRate            float64 `yaml:"max_error_rate"`             // Default: 0.03
	MaxLatencyP99Ms         int64   `yaml:"max_latency_p99_ms"`         // Default: 2000
	MinDataCompletenessRate float64 `yaml:"min_data_completeness_rate"` // Default: 0.95

	// CSV output settings
	ArtifactsDir       string `yaml:"artifacts_dir"`        // Default: "./artifacts/micro"
	EnableCSV          bool   `yaml:"enable_csv"`           // Default: true
	CSVTimestampFormat string `yaml:"csv_timestamp_format"` // Default: RFC3339
}

// DefaultHealthMonitorConfig returns default configuration
func DefaultHealthMonitorConfig() *HealthMonitorConfig {
	return &HealthMonitorConfig{
		HealthCheckIntervalSec:  5,
		CSVFlushIntervalSec:     30,
		MaxHistoryPoints:        720, // 1 hour at 5s intervals
		MaxHeartbeatAgeMs:       10000,
		MaxMessageGapRate:       0.05,
		MaxErrorRate:            0.03,
		MaxLatencyP99Ms:         2000,
		MinDataCompletenessRate: 0.95,
		ArtifactsDir:            "./artifacts/micro",
		EnableCSV:               true,
		CSVTimestampFormat:      time.RFC3339,
	}
}

// NewHealthMonitor creates a new health monitor
func NewHealthMonitor(collectors []Collector, config *HealthMonitorConfig) *HealthMonitor {
	if config == nil {
		config = DefaultHealthMonitorConfig()
	}

	return &HealthMonitor{
		collectors:    collectors,
		config:        config,
		healthHistory: make(map[string][]*VenueHealth),
		csvWriters:    make(map[string]*csv.Writer),
		csvFiles:      make(map[string]*os.File),
	}
}

// Start begins health monitoring
func (hm *HealthMonitor) Start(ctx context.Context) error {
	hm.ctx, hm.cancel = context.WithCancel(ctx)
	hm.running = true

	// Initialize CSV files if enabled
	if hm.config.EnableCSV {
		if err := hm.initializeCSVFiles(); err != nil {
			return fmt.Errorf("failed to initialize CSV files: %w", err)
		}
	}

	// Start tickers
	hm.healthCheckTicker = time.NewTicker(time.Duration(hm.config.HealthCheckIntervalSec) * time.Second)
	hm.csvFlushTicker = time.NewTicker(time.Duration(hm.config.CSVFlushIntervalSec) * time.Second)

	// Start background workers
	hm.wg.Add(2)
	go hm.healthCheckWorker()
	go hm.csvFlushWorker()

	return nil
}

// Stop gracefully shuts down the health monitor
func (hm *HealthMonitor) Stop(ctx context.Context) error {
	if !hm.running {
		return nil
	}

	// Signal shutdown
	hm.cancel()

	// Stop tickers
	if hm.healthCheckTicker != nil {
		hm.healthCheckTicker.Stop()
	}
	if hm.csvFlushTicker != nil {
		hm.csvFlushTicker.Stop()
	}

	// Wait for workers to finish
	hm.wg.Wait()

	// Close CSV files
	if err := hm.closeCSVFiles(); err != nil {
		return fmt.Errorf("failed to close CSV files: %w", err)
	}

	hm.running = false
	return nil
}

// GetVenueHealth returns current health for a venue
func (hm *HealthMonitor) GetVenueHealth(venue string) (*VenueHealth, error) {
	// Find the collector for this venue
	for _, collector := range hm.collectors {
		if collector.Venue() == venue {
			return collector.GetVenueHealth()
		}
	}

	return nil, fmt.Errorf("venue %s not found", venue)
}

// GetHealthHistory returns health history for a venue
func (hm *HealthMonitor) GetHealthHistory(venue string) ([]*VenueHealth, error) {
	hm.healthMutex.RLock()
	defer hm.healthMutex.RUnlock()

	history, exists := hm.healthHistory[venue]
	if !exists {
		return nil, fmt.Errorf("no health history available for venue %s", venue)
	}

	// Return a copy to prevent external modification
	historyCopy := make([]*VenueHealth, len(history))
	for i, health := range history {
		healthCopy := *health
		historyCopy[i] = &healthCopy
	}

	return historyCopy, nil
}

// GetAllVenueHealth returns current health for all venues
func (hm *HealthMonitor) GetAllVenueHealth() (map[string]*VenueHealth, error) {
	allHealth := make(map[string]*VenueHealth)

	for _, collector := range hm.collectors {
		venue := collector.Venue()
		if health, err := collector.GetVenueHealth(); err == nil {
			allHealth[venue] = health
		} else {
			return nil, fmt.Errorf("failed to get health for venue %s: %w", venue, err)
		}
	}

	return allHealth, nil
}

// GetHealthSummaryReport returns a comprehensive health summary
func (hm *HealthMonitor) GetHealthSummaryReport() *HealthSummaryReport {
	allHealth, _ := hm.GetAllVenueHealth()

	report := &HealthSummaryReport{
		Timestamp:     time.Now(),
		TotalVenues:   len(hm.collectors),
		VenueHealth:   allHealth,
		HealthCounts:  make(map[HealthStatus]int),
		AlertVenues:   make([]string, 0),
		HealthyVenues: make([]string, 0),
	}

	// Count health statuses and categorize venues
	for venue, health := range allHealth {
		report.HealthCounts[health.Status]++

		if health.Healthy {
			report.HealthyVenues = append(report.HealthyVenues, venue)
		} else {
			report.AlertVenues = append(report.AlertVenues, venue)
		}
	}

	// Calculate overall health rate
	if report.TotalVenues > 0 {
		report.OverallHealthRate = float64(len(report.HealthyVenues)) / float64(report.TotalVenues)
	}

	return report
}

// HealthSummaryReport provides a comprehensive health overview
type HealthSummaryReport struct {
	Timestamp         time.Time               `json:"timestamp"`
	TotalVenues       int                     `json:"total_venues"`
	HealthyVenues     []string                `json:"healthy_venues"`
	AlertVenues       []string                `json:"alert_venues"`
	OverallHealthRate float64                 `json:"overall_health_rate"`
	HealthCounts      map[HealthStatus]int    `json:"health_counts"`
	VenueHealth       map[string]*VenueHealth `json:"venue_health"`
}

// healthCheckWorker performs health checks on all collectors
func (hm *HealthMonitor) healthCheckWorker() {
	defer hm.wg.Done()

	for {
		select {
		case <-hm.ctx.Done():
			return
		case <-hm.healthCheckTicker.C:
			hm.performHealthChecks()
		}
	}
}

// csvFlushWorker flushes CSV data periodically
func (hm *HealthMonitor) csvFlushWorker() {
	defer hm.wg.Done()

	for {
		select {
		case <-hm.ctx.Done():
			return
		case <-hm.csvFlushTicker.C:
			hm.flushCSVData()
		}
	}
}

// performHealthChecks checks health of all collectors and updates history
func (hm *HealthMonitor) performHealthChecks() {
	hm.healthMutex.Lock()
	defer hm.healthMutex.Unlock()

	for _, collector := range hm.collectors {
		venue := collector.Venue()

		if health, err := collector.GetVenueHealth(); err == nil {
			// Enhance health data with monitor-level analysis
			enhancedHealth := hm.enhanceHealthData(health)

			// Add to history
			if hm.healthHistory[venue] == nil {
				hm.healthHistory[venue] = make([]*VenueHealth, 0)
			}

			hm.healthHistory[venue] = append(hm.healthHistory[venue], enhancedHealth)

			// Trim history to max points
			if len(hm.healthHistory[venue]) > hm.config.MaxHistoryPoints {
				hm.healthHistory[venue] = hm.healthHistory[venue][1:]
			}

			// Write to CSV if enabled
			if hm.config.EnableCSV {
				hm.writeHealthToCSV(venue, enhancedHealth)
			}
		}
	}
}

// enhanceHealthData applies monitor-level health analysis
func (hm *HealthMonitor) enhanceHealthData(health *VenueHealth) *VenueHealth {
	// Create a copy to avoid modifying the original
	enhanced := *health

	// Apply monitor-level thresholds (may be stricter than collector defaults)
	if enhanced.HeartbeatAgeMs > hm.config.MaxHeartbeatAgeMs {
		enhanced.Status = HealthRed
		enhanced.Healthy = false
		enhanced.Recommendation = "avoid"
	} else if enhanced.MessageGapRate > hm.config.MaxMessageGapRate {
		if enhanced.Status == HealthGreen {
			enhanced.Status = HealthYellow
			enhanced.Recommendation = "halve_size"
		}
	}

	if enhanced.ErrorRate > hm.config.MaxErrorRate {
		enhanced.Status = HealthRed
		enhanced.Healthy = false
		enhanced.Recommendation = "avoid"
	}

	if enhanced.LatencyP99Ms > hm.config.MaxLatencyP99Ms {
		if enhanced.Status == HealthGreen {
			enhanced.Status = HealthYellow
			enhanced.Recommendation = "halve_size"
		}
	}

	if enhanced.DataCompleteness < hm.config.MinDataCompletenessRate {
		if enhanced.Status == HealthGreen {
			enhanced.Status = HealthYellow
			enhanced.Recommendation = "halve_size"
		}
	}

	return &enhanced
}

// initializeCSVFiles sets up CSV files for health artifacts
func (hm *HealthMonitor) initializeCSVFiles() error {
	// Create artifacts directory
	if err := os.MkdirAll(hm.config.ArtifactsDir, 0755); err != nil {
		return fmt.Errorf("failed to create artifacts directory: %w", err)
	}

	hm.csvMutex.Lock()
	defer hm.csvMutex.Unlock()

	for _, collector := range hm.collectors {
		venue := collector.Venue()
		filename := fmt.Sprintf("health_%s.csv", venue)
		filepath := filepath.Join(hm.config.ArtifactsDir, filename)

		// Open file for append
		file, err := os.OpenFile(filepath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			return fmt.Errorf("failed to open CSV file for venue %s: %w", venue, err)
		}

		hm.csvFiles[venue] = file
		hm.csvWriters[venue] = csv.NewWriter(file)

		// Write header if file is empty
		fileInfo, err := file.Stat()
		if err != nil {
			return fmt.Errorf("failed to stat CSV file: %w", err)
		}

		if fileInfo.Size() == 0 {
			header := []string{
				"timestamp", "venue", "status", "healthy", "uptime",
				"heartbeat_age_ms", "message_gap_rate", "ws_reconnect_count",
				"latency_p50_ms", "latency_p99_ms", "error_rate",
				"data_freshness_ms", "data_completeness", "recommendation",
			}
			if err := hm.csvWriters[venue].Write(header); err != nil {
				return fmt.Errorf("failed to write CSV header for venue %s: %w", venue, err)
			}
		}
	}

	return nil
}

// writeHealthToCSV writes a health record to the venue's CSV file
func (hm *HealthMonitor) writeHealthToCSV(venue string, health *VenueHealth) {
	hm.csvMutex.Lock()
	defer hm.csvMutex.Unlock()

	writer, exists := hm.csvWriters[venue]
	if !exists {
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

	if err := writer.Write(row); err != nil {
		// Log error but don't fail the monitor
		fmt.Printf("Failed to write health CSV record for %s: %v\n", venue, err)
	}
}

// flushCSVData flushes all CSV writers
func (hm *HealthMonitor) flushCSVData() {
	hm.csvMutex.Lock()
	defer hm.csvMutex.Unlock()

	for venue, writer := range hm.csvWriters {
		writer.Flush()
		if err := writer.Error(); err != nil {
			fmt.Printf("CSV flush error for venue %s: %v\n", venue, err)
		}
	}
}

// closeCSVFiles closes all CSV files
func (hm *HealthMonitor) closeCSVFiles() error {
	hm.csvMutex.Lock()
	defer hm.csvMutex.Unlock()

	// Flush all writers first
	for _, writer := range hm.csvWriters {
		writer.Flush()
	}

	// Close all files
	var lastErr error
	for venue, file := range hm.csvFiles {
		if err := file.Close(); err != nil {
			lastErr = fmt.Errorf("failed to close CSV file for venue %s: %w", venue, err)
		}
	}

	return lastErr
}

// GetCSVFilePaths returns the paths to all health CSV files
func (hm *HealthMonitor) GetCSVFilePaths() map[string]string {
	paths := make(map[string]string)

	for _, collector := range hm.collectors {
		venue := collector.Venue()
		filename := fmt.Sprintf("health_%s.csv", venue)
		paths[venue] = filepath.Join(hm.config.ArtifactsDir, filename)
	}

	return paths
}
