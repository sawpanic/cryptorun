package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"cryptorun/internal/ops"
)

var opsStatusCmd = &cobra.Command{
	Use:    "ops",
	Hidden: true, // Hidden command as requested
	Short:  "Operational status and controls",
	Long: `Operational status command provides real-time visibility into:
- KPI metrics (requests/min, error rates, cache hits)
- Guard status (budget, quotas, correlation caps)
- Emergency switches and circuit breaker states
- Provider and venue health monitoring`,
}

var opsStatusShowCmd = &cobra.Command{
	Use:   "status",
	Short: "Show operational status",
	Long: `Display comprehensive operational status including:
- Rolling KPI metrics
- Guard results and thresholds
- Emergency switch states
- Provider and venue availability
- Circuit breaker status

Outputs both console table and CSV snapshot.`,
	RunE: runOpsStatus,
}

// Command line flags
var (
	opsConfigPath string
	outputDir     string
)

func init() {
	// Add ops command to root
	rootCmd.AddCommand(opsStatusCmd)

	// Add status subcommand
	opsStatusCmd.AddCommand(opsStatusShowCmd)

	// Flags
	opsStatusShowCmd.Flags().StringVar(&opsConfigPath, "config", "config/ops.yaml", "Path to ops configuration file")
	opsStatusShowCmd.Flags().StringVar(&outputDir, "output", "./artifacts/ops", "Output directory for snapshots")
}

// runOpsStatus executes the ops status command
func runOpsStatus(cmd *cobra.Command, args []string) error {
	// Load configuration
	config, err := loadOpsConfig(opsConfigPath)
	if err != nil {
		return fmt.Errorf("failed to load ops config: %w", err)
	}

	// Initialize components
	kpiTracker := ops.NewKPITracker(
		time.Duration(config.KPI.Windows.RequestsPerMin)*time.Second,
		time.Duration(config.KPI.Windows.ErrorRate)*time.Second,
		time.Duration(config.KPI.Windows.CacheHitRate)*time.Second,
	)

	guardManager := ops.NewGuardManager(config.Guards)
	switchManager := ops.NewSwitchManager(config.Switches)
	renderer := ops.NewStatusRenderer(config.Artifacts.OutputDir)

	// Override output dir if specified
	if outputDir != "./artifacts/ops" {
		renderer = ops.NewStatusRenderer(outputDir)
	}

	// Collect current status
	kpiMetrics := kpiTracker.GetMetrics()
	guardResults := guardManager.CheckAllGuards()
	switchStatus := switchManager.GetStatus()

	// Add some sample data for demonstration
	populateSampleData(kpiTracker, guardManager)
	kpiMetrics = kpiTracker.GetMetrics()
	guardResults = guardManager.CheckAllGuards()

	// Render to console
	renderer.RenderConsole(kpiMetrics, guardResults, switchStatus)

	// Write snapshot
	if err := renderer.WriteSnapshot(kpiMetrics, guardResults, switchStatus); err != nil {
		log.Printf("Warning: failed to write snapshot: %v", err)
	}

	return nil
}

// OpsConfig represents the ops.yaml configuration structure
type OpsConfig struct {
	KPI struct {
		Windows struct {
			RequestsPerMin int `yaml:"requests_per_min"`
			ErrorRate      int `yaml:"error_rate"`
			CacheHitRate   int `yaml:"cache_hit_rate"`
		} `yaml:"windows"`
		Thresholds struct {
			ErrorRateWarn          float64 `yaml:"error_rate_warn"`
			ErrorRateCritical      float64 `yaml:"error_rate_critical"`
			CacheHitRateWarn       float64 `yaml:"cache_hit_rate_warn"`
			CacheHitRateCritical   float64 `yaml:"cache_hit_rate_critical"`
			RequestsPerMinWarn     float64 `yaml:"requests_per_min_warn"`
			RequestsPerMinCritical float64 `yaml:"requests_per_min_critical"`
		} `yaml:"thresholds"`
	} `yaml:"kpi"`

	Guards ops.GuardConfig `yaml:"guards"`

	Switches ops.SwitchConfig `yaml:"switches"`

	Artifacts struct {
		OutputDir        string `yaml:"output_dir"`
		SnapshotFilename string `yaml:"snapshot_filename"`
		RetentionDays    int    `yaml:"retention_days"`
	} `yaml:"artifacts"`
}

// loadOpsConfig loads operational configuration from YAML
func loadOpsConfig(path string) (*OpsConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file %s: %w", path, err)
	}

	var config OpsConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config YAML: %w", err)
	}

	return &config, nil
}

// populateSampleData adds sample data for demonstration
func populateSampleData(kpiTracker *ops.KPITracker, guardManager *ops.GuardManager) {
	// Simulate some KPI data
	for i := 0; i < 45; i++ {
		kpiTracker.RecordRequest()
	}

	for i := 0; i < 3; i++ {
		kpiTracker.RecordError()
	}

	for i := 0; i < 20; i++ {
		kpiTracker.RecordCacheHit()
	}

	for i := 0; i < 5; i++ {
		kpiTracker.RecordCacheMiss()
	}

	// Simulate provider status
	kpiTracker.SetBreakerOpen("kraken", false)
	kpiTracker.SetBreakerOpen("binance", false)

	// Update venue health
	kpiTracker.UpdateVenueHealth("kraken_usd", ops.VenueHealthStatus{
		IsHealthy:     true,
		UptimePercent: 99.5,
		LatencyMs:     250,
		DepthUSD:      125000,
		SpreadBps:     8.5,
	})

	kpiTracker.UpdateVenueHealth("binance_usd", ops.VenueHealthStatus{
		IsHealthy:     true,
		UptimePercent: 98.2,
		LatencyMs:     180,
		DepthUSD:      85000,
		SpreadBps:     12.3,
	})

	// Simulate API calls for guard testing
	for i := 0; i < 25; i++ {
		guardManager.RecordAPICall("kraken")
	}

	for i := 0; i < 45; i++ {
		guardManager.RecordAPICall("binance")
	}

	// Simulate some signals for correlation analysis
	signals := []ops.SignalData{
		{Symbol: "BTC-USD", Score: 78.5, Timestamp: time.Now().Add(-5 * time.Minute)},
		{Symbol: "ETH-USD", Score: 72.1, Timestamp: time.Now().Add(-3 * time.Minute)},
		{Symbol: "SOL-USD", Score: 69.8, Timestamp: time.Now().Add(-1 * time.Minute)},
		{Symbol: "ADA-USD", Score: 65.2, Timestamp: time.Now()},
	}

	for _, signal := range signals {
		guardManager.RecordSignal(signal)
	}
}
