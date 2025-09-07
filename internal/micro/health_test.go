package micro

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// MockCollector implements the Collector interface for testing
type MockCollector struct {
	venue       string
	healthy     bool
	l1Data      *L1Data
	l2Data      *L2Data
	venueHealth *VenueHealth
	metrics     *CollectorMetrics
}

func NewMockCollector(venue string, healthy bool) *MockCollector {
	now := time.Now()

	status := HealthGreen
	recommendation := "proceed"
	if !healthy {
		status = HealthRed
		recommendation = "avoid"
	}

	return &MockCollector{
		venue:   venue,
		healthy: healthy,
		l1Data: &L1Data{
			Symbol:    "BTC/USD",
			Venue:     venue,
			Timestamp: now,
			BidPrice:  50000.0,
			AskPrice:  50010.0,
			LastPrice: 50005.0,
			SpreadBps: 2.0,
		},
		l2Data: &L2Data{
			Symbol:        "BTC/USD",
			Venue:         venue,
			Timestamp:     now,
			BidDepthUSD:   100000.0,
			AskDepthUSD:   95000.0,
			TotalDepthUSD: 195000.0,
			IsUSDQuote:    true,
		},
		venueHealth: &VenueHealth{
			Venue:            venue,
			Timestamp:        now,
			Status:           status,
			Healthy:          healthy,
			Uptime:           95.0,
			LatencyP50Ms:     45,
			LatencyP99Ms:     120,
			ErrorRate:        0.02,
			DataFreshness:    2 * time.Second,
			DataCompleteness: 98.0,
			Recommendation:   recommendation,
		},
		metrics: &CollectorMetrics{
			Venue:         venue,
			WindowStart:   now.Add(-time.Second),
			WindowEnd:     now,
			L1Messages:    100,
			L2Messages:    100,
			ErrorMessages: 2,
			QualityScore:  85.0,
		},
	}
}

func (m *MockCollector) Start(ctx context.Context) error { return nil }
func (m *MockCollector) Stop(ctx context.Context) error  { return nil }

func (m *MockCollector) GetL1Data(symbol string) (*L1Data, error) {
	return m.l1Data, nil
}

func (m *MockCollector) GetL2Data(symbol string) (*L2Data, error) {
	return m.l2Data, nil
}

func (m *MockCollector) GetVenueHealth() (*VenueHealth, error) {
	return m.venueHealth, nil
}

func (m *MockCollector) GetMetrics() (*CollectorMetrics, error) {
	return m.metrics, nil
}

func (m *MockCollector) Subscribe(symbols []string) error   { return nil }
func (m *MockCollector) Unsubscribe(symbols []string) error { return nil }
func (m *MockCollector) Venue() string                      { return m.venue }
func (m *MockCollector) IsHealthy() bool                    { return m.healthy }

// SetHealth allows updating the mock collector's health for testing
func (m *MockCollector) SetHealth(healthy bool, status HealthStatus, errorRate float64) {
	m.healthy = healthy
	m.venueHealth.Healthy = healthy
	m.venueHealth.Status = status
	m.venueHealth.ErrorRate = errorRate
	m.venueHealth.Timestamp = time.Now()

	if healthy {
		m.venueHealth.Recommendation = "proceed"
	} else {
		m.venueHealth.Recommendation = "avoid"
	}
}

func TestHealthMonitorLifecycle(t *testing.T) {
	// Create temporary directory for CSV files
	tempDir, err := os.MkdirTemp("", "health_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create mock collectors
	collectors := []Collector{
		NewMockCollector("binance", true),
		NewMockCollector("okx", true),
		NewMockCollector("coinbase", false), // One unhealthy collector
	}

	config := &HealthMonitorConfig{
		HealthCheckIntervalSec: 1, // Fast interval for testing
		CSVFlushIntervalSec:    2,
		MaxHistoryPoints:       10,
		ArtifactsDir:           tempDir,
		EnableCSV:              true,
		CSVTimestampFormat:     time.RFC3339,
	}

	monitor := NewHealthMonitor(collectors, config)

	// Test start
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := monitor.Start(ctx); err != nil {
		t.Fatalf("Failed to start health monitor: %v", err)
	}

	// Wait for a few health checks
	time.Sleep(3 * time.Second)

	// Test getting venue health
	health, err := monitor.GetVenueHealth("binance")
	if err != nil {
		t.Errorf("Failed to get binance health: %v", err)
	}
	if health.Venue != "binance" {
		t.Errorf("Expected venue binance, got %s", health.Venue)
	}
	if !health.Healthy {
		t.Error("Expected binance to be healthy")
	}

	// Test getting all venue health
	allHealth, err := monitor.GetAllVenueHealth()
	if err != nil {
		t.Errorf("Failed to get all venue health: %v", err)
	}
	if len(allHealth) != 3 {
		t.Errorf("Expected 3 venues, got %d", len(allHealth))
	}

	// Test health summary report
	summary := monitor.GetHealthSummaryReport()
	if summary.TotalVenues != 3 {
		t.Errorf("Expected 3 total venues, got %d", summary.TotalVenues)
	}
	if len(summary.HealthyVenues) != 2 {
		t.Errorf("Expected 2 healthy venues, got %d", len(summary.HealthyVenues))
	}
	if len(summary.AlertVenues) != 1 {
		t.Errorf("Expected 1 alert venue, got %d", len(summary.AlertVenues))
	}
	if summary.OverallHealthRate < 0.6 || summary.OverallHealthRate > 0.7 {
		t.Errorf("Expected health rate around 0.67, got %f", summary.OverallHealthRate)
	}

	// Test CSV file creation
	csvPaths := monitor.GetCSVFilePaths()
	if len(csvPaths) != 3 {
		t.Errorf("Expected 3 CSV files, got %d", len(csvPaths))
	}

	for venue, path := range csvPaths {
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("CSV file for venue %s does not exist at %s", venue, path)
		}
	}

	// Test stop
	if err := monitor.Stop(ctx); err != nil {
		t.Errorf("Failed to stop health monitor: %v", err)
	}
}

func TestHealthMonitorThresholds(t *testing.T) {
	// Create mock collector that we can manipulate
	mockCollector := NewMockCollector("test", true)
	collectors := []Collector{mockCollector}

	config := &HealthMonitorConfig{
		HealthCheckIntervalSec:  1,
		CSVFlushIntervalSec:     2,
		MaxHistoryPoints:        10,
		MaxErrorRate:            0.03, // 3% threshold
		MaxLatencyP99Ms:         1000, // 1s threshold
		MinDataCompletenessRate: 0.95, // 95% threshold
		ArtifactsDir:            "./test_temp",
		EnableCSV:               false, // Disable CSV for this test
		CSVTimestampFormat:      time.RFC3339,
	}

	monitor := NewHealthMonitor(collectors, config)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if err := monitor.Start(ctx); err != nil {
		t.Fatalf("Failed to start monitor: %v", err)
	}
	defer monitor.Stop(ctx)

	t.Run("error rate threshold", func(t *testing.T) {
		// Set error rate above threshold
		mockCollector.SetHealth(true, HealthGreen, 0.05) // 5% > 3% threshold

		time.Sleep(1500 * time.Millisecond) // Wait for health check

		health, _ := monitor.GetVenueHealth("test")
		if health.Status != HealthRed {
			t.Errorf("Expected RED status for high error rate, got %v", health.Status)
		}
		if health.Healthy {
			t.Error("Expected unhealthy status for high error rate")
		}
		if health.Recommendation != "avoid" {
			t.Errorf("Expected 'avoid' recommendation, got %s", health.Recommendation)
		}
	})

	t.Run("recovery from error", func(t *testing.T) {
		// Set error rate back to normal
		mockCollector.SetHealth(true, HealthGreen, 0.01) // 1% < 3% threshold

		time.Sleep(1500 * time.Millisecond) // Wait for health check

		health, _ := monitor.GetVenueHealth("test")
		if health.Status != HealthGreen {
			t.Errorf("Expected GREEN status after recovery, got %v", health.Status)
		}
		if !health.Healthy {
			t.Error("Expected healthy status after recovery")
		}
	})
}

func TestMetricsAggregatorIntegration(t *testing.T) {
	// Create mock collectors with different health states
	collectors := []Collector{
		NewMockCollector("binance", true),
		NewMockCollector("okx", true),
		NewMockCollector("coinbase", false),
	}

	aggregator := NewMetricsAggregator(collectors)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if err := aggregator.Start(ctx); err != nil {
		t.Fatalf("Failed to start metrics aggregator: %v", err)
	}
	defer aggregator.Stop(ctx)

	// Wait for metrics collection
	time.Sleep(1500 * time.Millisecond)

	t.Run("aggregated report", func(t *testing.T) {
		report := aggregator.GetAggregatedReport()

		if report.TotalVenues != 3 {
			t.Errorf("Expected 3 venues, got %d", report.TotalVenues)
		}

		if report.HealthyVenues != 2 {
			t.Errorf("Expected 2 healthy venues, got %d", report.HealthyVenues)
		}

		if len(report.VenueHealth) != 3 {
			t.Errorf("Expected 3 venue health entries, got %d", len(report.VenueHealth))
		}

		if len(report.VenueMetrics) != 3 {
			t.Errorf("Expected 3 venue metrics entries, got %d", len(report.VenueMetrics))
		}

		// Check specific venue health
		binanceHealth, exists := report.VenueHealth["binance"]
		if !exists {
			t.Error("Expected binance health in report")
		}
		if binanceHealth != nil && !binanceHealth.Healthy {
			t.Error("Expected binance to be healthy")
		}

		coinbaseHealth, exists := report.VenueHealth["coinbase"]
		if !exists {
			t.Error("Expected coinbase health in report")
		}
		if coinbaseHealth != nil && coinbaseHealth.Healthy {
			t.Error("Expected coinbase to be unhealthy")
		}
	})

	t.Run("health badges", func(t *testing.T) {
		badges := aggregator.GetHealthBadges()

		if len(badges) != 3 {
			t.Errorf("Expected 3 health badges, got %d", len(badges))
		}

		if badges["binance"] != HealthGreen {
			t.Errorf("Expected binance to be GREEN, got %v", badges["binance"])
		}

		if badges["coinbase"] != HealthRed {
			t.Errorf("Expected coinbase to be RED, got %v", badges["coinbase"])
		}
	})

	t.Run("venue metrics", func(t *testing.T) {
		metrics, err := aggregator.GetVenueMetrics("binance")
		if err != nil {
			t.Errorf("Failed to get binance metrics: %v", err)
		}

		if metrics.Venue != "binance" {
			t.Errorf("Expected venue binance, got %s", metrics.Venue)
		}

		if metrics.L1Messages == 0 {
			t.Error("Expected non-zero L1 messages")
		}
	})

	t.Run("health summary formatting", func(t *testing.T) {
		summary := aggregator.GetHealthSummary()

		// Should contain venue counts and emoji indicators
		if summary == "" {
			t.Error("Expected non-empty health summary")
		}

		// Should contain total count
		if len(summary) < 10 {
			t.Errorf("Expected detailed summary, got: %s", summary)
		}
	})
}

func TestHealthMonitorCSVOutput(t *testing.T) {
	// Create temporary directory for CSV files
	tempDir, err := os.MkdirTemp("", "csv_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	collectors := []Collector{
		NewMockCollector("test", true),
	}

	config := &HealthMonitorConfig{
		HealthCheckIntervalSec: 1,
		CSVFlushIntervalSec:    1,
		ArtifactsDir:           tempDir,
		EnableCSV:              true,
	}

	monitor := NewHealthMonitor(collectors, config)

	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()

	if err := monitor.Start(ctx); err != nil {
		t.Fatalf("Failed to start monitor: %v", err)
	}

	// Wait for health checks and CSV writes
	time.Sleep(2500 * time.Millisecond)

	if err := monitor.Stop(ctx); err != nil {
		t.Errorf("Failed to stop monitor: %v", err)
	}

	// Check CSV file content
	csvPath := filepath.Join(tempDir, "health_test.csv")
	content, err := os.ReadFile(csvPath)
	if err != nil {
		t.Fatalf("Failed to read CSV file: %v", err)
	}

	contentStr := string(content)

	// Check for header
	if !contains(contentStr, "timestamp,venue,status") {
		t.Error("CSV should contain header")
	}

	// Check for data rows
	if !contains(contentStr, "test,green,true") && !contains(contentStr, "test,GREEN,true") {
		t.Error("CSV should contain test venue data")
	}

	// Count lines (header + at least one data row)
	lines := len(strings.Split(contentStr, "\n"))
	if lines < 3 { // header + data + final newline
		t.Errorf("Expected at least 3 lines in CSV, got %d", lines)
	}
}

func TestHealthMonitorHistoryTracking(t *testing.T) {
	mockCollector := NewMockCollector("test", true)
	collectors := []Collector{mockCollector}

	config := &HealthMonitorConfig{
		HealthCheckIntervalSec: 1,
		MaxHistoryPoints:       5, // Small history for testing
		EnableCSV:              false,
	}

	monitor := NewHealthMonitor(collectors, config)

	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
	defer cancel()

	if err := monitor.Start(ctx); err != nil {
		t.Fatalf("Failed to start monitor: %v", err)
	}
	defer monitor.Stop(ctx)

	// Change health state a few times
	time.Sleep(1500 * time.Millisecond)
	mockCollector.SetHealth(false, HealthRed, 0.1)

	time.Sleep(1500 * time.Millisecond)
	mockCollector.SetHealth(true, HealthYellow, 0.02)

	time.Sleep(1500 * time.Millisecond)
	mockCollector.SetHealth(true, HealthGreen, 0.01)

	time.Sleep(1500 * time.Millisecond)

	// Check history
	history, err := monitor.GetHealthHistory("test")
	if err != nil {
		t.Fatalf("Failed to get health history: %v", err)
	}

	if len(history) == 0 {
		t.Error("Expected non-empty health history")
	}

	// History should not exceed max points
	if len(history) > 5 {
		t.Errorf("Expected history length <= 5, got %d", len(history))
	}

	// Check that we have different health states in history
	statusSeen := make(map[HealthStatus]bool)
	for _, h := range history {
		statusSeen[h.Status] = true
	}

	if len(statusSeen) < 2 {
		t.Error("Expected to see multiple health states in history")
	}
}

// Helper function for string containment check
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr ||
		(len(s) > len(substr) && (s[:len(substr)] == substr ||
			s[len(s)-len(substr):] == substr ||
			strings.Contains(s, substr))))
}

// Helper to split strings (using standard library)
