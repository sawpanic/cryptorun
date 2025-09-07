package ops

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/sawpanic/cryptorun/internal/ops"
)

func TestOpsIntegration_ProviderBreakerScenario(t *testing.T) {
	// Initialize operational components
	kpiTracker := ops.NewKPITracker(
		60*time.Second,  // request window
		300*time.Second, // error window
		300*time.Second, // cache window
	)

	guardConfig := ops.GuardConfig{
		Budget: ops.BudgetGuardConfig{
			Enabled:         true,
			HourlyLimit:     100,
			SoftWarnPercent: 0.8,
			HardStopPercent: 0.95,
		},
		CallQuota: ops.CallQuotaGuardConfig{
			Enabled: true,
			Providers: map[string]ops.ProviderQuotaConfig{
				"kraken": {
					CallsPerMinute: 30,
					BurstLimit:     5,
				},
			},
		},
	}

	guardManager := ops.NewGuardManager(guardConfig)

	switchConfig := ops.SwitchConfig{
		Emergency: ops.EmergencySwitchConfig{
			DisableAllScanners: false,
			DisableLiveData:    false,
			ReadOnlyMode:       false,
		},
		Providers: map[string]ops.ProviderSwitchConfig{
			"kraken": {
				Enabled:        true,
				AllowWebsocket: true,
				AllowRest:      true,
			},
		},
		Venues: map[string]bool{
			"kraken_usd": true,
		},
	}

	switchManager := ops.NewSwitchManager(switchConfig)

	// Test Scenario: Provider breaker opens due to errors
	t.Run("ProviderBreakerOpens", func(t *testing.T) {
		// Simulate successful operations initially
		for i := 0; i < 10; i++ {
			guardManager.RecordAPICall("kraken")
			kpiTracker.RecordRequest()
			time.Sleep(10 * time.Millisecond) // Small delay to spread calls
		}

		// Set breaker open (simulating circuit breaker activation)
		kpiTracker.SetBreakerOpen("kraken", true)

		// Check that KPI reflects the open breaker
		metrics := kpiTracker.GetMetrics()
		if metrics.OpenBreakerCount != 1 {
			t.Errorf("Expected 1 open breaker, got %d", metrics.OpenBreakerCount)
		}

		// Check that the specific provider is in the open breakers list
		openBreakers := kpiTracker.GetOpenBreakers()
		if len(openBreakers) != 1 || openBreakers[0] != "kraken" {
			t.Errorf("Expected kraken in open breakers list, got %v", openBreakers)
		}
	})

	// Test Scenario: Emergency switches activated
	t.Run("EmergencySwitchesActivated", func(t *testing.T) {
		// Initially all systems should be operational
		if !switchManager.IsScannersEnabled() {
			t.Error("Expected scanners to be enabled initially")
		}

		// Activate emergency switch
		switchManager.SetEmergencySwitch("disable_all_scanners", true)

		// Check that scanners are now disabled
		if switchManager.IsScannersEnabled() {
			t.Error("Expected scanners to be disabled after emergency switch")
		}

		// Check status reflects emergency state
		status := switchManager.GetStatus()
		if !status.Emergency.AnyEmergencyActive {
			t.Error("Expected emergency state to be active")
		}

		if !status.Emergency.AllScannersDisabled {
			t.Error("Expected all scanners to be marked as disabled")
		}
	})

	// Test Scenario: Provider quota exceeded
	t.Run("ProviderQuotaExceeded", func(t *testing.T) {
		// Reset guard manager for clean test
		guardManager := ops.NewGuardManager(guardConfig)

		// Rapidly make API calls to exceed burst limit
		for i := 0; i < 7; i++ { // Exceeds burst limit of 5
			guardManager.RecordAPICall("kraken")
		}

		// Check guard results
		results := guardManager.CheckAllGuards()

		// Find kraken quota result
		var quotaResult *ops.GuardResult
		for _, result := range results {
			if result.Name == "kraken" {
				quotaResult = &result
				break
			}
		}

		if quotaResult == nil {
			t.Fatal("Expected kraken quota guard result")
		}

		if quotaResult.Status != ops.GuardStatusBlock {
			t.Errorf("Expected BLOCK status for quota exceeded, got %s", quotaResult.Status.String())
		}
	})

	// Test Scenario: Venue health degradation
	t.Run("VenueHealthDegradation", func(t *testing.T) {
		// Add healthy venue
		kpiTracker.UpdateVenueHealth("kraken_usd", ops.VenueHealthStatus{
			IsHealthy:     true,
			UptimePercent: 99.5,
			LatencyMs:     100,
			DepthUSD:      100000,
			SpreadBps:     10,
		})

		metrics := kpiTracker.GetMetrics()
		if metrics.HealthyVenueCount != 1 {
			t.Errorf("Expected 1 healthy venue, got %d", metrics.HealthyVenueCount)
		}

		// Degrade venue health
		kpiTracker.UpdateVenueHealth("kraken_usd", ops.VenueHealthStatus{
			IsHealthy:     false,
			UptimePercent: 85.0,
			LatencyMs:     8000,
			DepthUSD:      20000,
			SpreadBps:     150,
		})

		metrics = kpiTracker.GetMetrics()
		if metrics.UnhealthyVenueCount != 1 {
			t.Errorf("Expected 1 unhealthy venue after degradation, got %d", metrics.UnhealthyVenueCount)
		}

		if metrics.HealthyVenueCount != 0 {
			t.Errorf("Expected 0 healthy venues after degradation, got %d", metrics.HealthyVenueCount)
		}
	})
}

func TestOpsIntegration_StatusReporting(t *testing.T) {
	// Create temporary output directory
	tempDir := t.TempDir()

	// Initialize components with sample data
	kpiTracker := ops.NewKPITracker(60*time.Second, 300*time.Second, 300*time.Second)
	guardManager := ops.NewGuardManager(ops.GuardConfig{
		Budget: ops.BudgetGuardConfig{
			Enabled:     true,
			HourlyLimit: 1000,
		},
	})
	switchManager := ops.NewSwitchManager(ops.SwitchConfig{
		Emergency: ops.EmergencySwitchConfig{},
		Providers: map[string]ops.ProviderSwitchConfig{
			"kraken": {
				Enabled:        true,
				AllowWebsocket: true,
				AllowRest:      true,
			},
		},
		Venues: map[string]bool{
			"kraken_usd": true,
		},
	})

	renderer := ops.NewStatusRenderer(tempDir)

	// Add sample data
	for i := 0; i < 25; i++ {
		kpiTracker.RecordRequest()
		guardManager.RecordAPICall("kraken")
	}

	for i := 0; i < 3; i++ {
		kpiTracker.RecordError()
	}

	for i := 0; i < 15; i++ {
		kpiTracker.RecordCacheHit()
	}

	for i := 0; i < 5; i++ {
		kpiTracker.RecordCacheMiss()
	}

	kpiTracker.UpdateVenueHealth("kraken_usd", ops.VenueHealthStatus{
		IsHealthy:     true,
		UptimePercent: 99.2,
		LatencyMs:     200,
		DepthUSD:      75000,
		SpreadBps:     12.5,
	})

	// Get status data
	kpiMetrics := kpiTracker.GetMetrics()
	guardResults := guardManager.CheckAllGuards()
	switchStatus := switchManager.GetStatus()

	// Test snapshot writing
	t.Run("SnapshotWriting", func(t *testing.T) {
		err := renderer.WriteSnapshot(kpiMetrics, guardResults, switchStatus)
		if err != nil {
			t.Fatalf("Failed to write snapshot: %v", err)
		}

		// Check that standard snapshot file was created
		standardPath := filepath.Join(tempDir, "status_snapshot.csv")
		if _, err := os.Stat(standardPath); os.IsNotExist(err) {
			t.Error("Standard snapshot file was not created")
		}

		// Check that timestamped file exists
		entries, err := os.ReadDir(tempDir)
		if err != nil {
			t.Fatalf("Failed to read temp directory: %v", err)
		}

		timestampedFound := false
		for _, entry := range entries {
			if entry.Name() != "status_snapshot.csv" &&
				filepath.Ext(entry.Name()) == ".csv" {
				timestampedFound = true
				break
			}
		}

		if !timestampedFound {
			t.Error("Timestamped snapshot file was not created")
		}
	})

	// Test CSV content
	t.Run("CSVContent", func(t *testing.T) {
		standardPath := filepath.Join(tempDir, "status_snapshot.csv")
		content, err := os.ReadFile(standardPath)
		if err != nil {
			t.Fatalf("Failed to read snapshot file: %v", err)
		}

		contentStr := string(content)

		// Check for header
		if !contains(contentStr, "timestamp,category,name,value,status,message") {
			t.Error("CSV header not found in snapshot")
		}

		// Check for KPI data
		if !contains(contentStr, "kpi,requests_per_minute") {
			t.Error("KPI data not found in snapshot")
		}

		// Check for switch data
		if !contains(contentStr, "switch,emergency_scanners") {
			t.Error("Switch data not found in snapshot")
		}
	})
}

func TestOpsIntegration_FullWorkflow(t *testing.T) {
	// This test simulates a complete operational workflow

	// Initialize all components
	kpiTracker := ops.NewKPITracker(60*time.Second, 300*time.Second, 300*time.Second)

	guardConfig := ops.GuardConfig{
		Budget: ops.BudgetGuardConfig{
			Enabled:         true,
			HourlyLimit:     50, // Low limit for testing
			SoftWarnPercent: 0.8,
			HardStopPercent: 0.95,
		},
		CallQuota: ops.CallQuotaGuardConfig{
			Enabled: true,
			Providers: map[string]ops.ProviderQuotaConfig{
				"kraken": {
					CallsPerMinute: 20,
					BurstLimit:     3,
				},
			},
		},
		Correlation: ops.CorrelationGuardConfig{
			Enabled:         true,
			MaxCorrelation:  0.7,
			TopNSignals:     3,
			LookbackPeriods: 1, // Short for testing
		},
	}

	guardManager := ops.NewGuardManager(guardConfig)

	switchConfig := ops.SwitchConfig{
		Emergency: ops.EmergencySwitchConfig{},
		Providers: map[string]ops.ProviderSwitchConfig{
			"kraken": {Enabled: true, AllowWebsocket: true, AllowRest: true},
		},
		Venues: map[string]bool{
			"kraken_usd": true,
		},
	}

	switchManager := ops.NewSwitchManager(switchConfig)

	// Phase 1: Normal operation
	t.Run("NormalOperation", func(t *testing.T) {
		// Record normal activity
		for i := 0; i < 10; i++ {
			guardManager.RecordAPICall("kraken")
			kpiTracker.RecordRequest()
		}

		// Check that everything is OK
		guards := guardManager.CheckAllGuards()
		for _, guard := range guards {
			if guard.Status == ops.GuardStatusBlock || guard.Status == ops.GuardStatusCritical {
				t.Errorf("Unexpected guard status during normal operation: %s - %s", guard.Name, guard.Status)
			}
		}

		metrics := kpiTracker.GetMetrics()
		if metrics.OpenBreakerCount > 0 {
			t.Error("Unexpected open breakers during normal operation")
		}
	})

	// Phase 2: Gradual degradation
	t.Run("GradualDegradation", func(t *testing.T) {
		// Add more API calls to approach limits
		for i := 0; i < 35; i++ { // Total now 45/50 = 90%
			guardManager.RecordAPICall("kraken")
			kpiTracker.RecordRequest()
		}

		// Should trigger warning
		guards := guardManager.CheckAllGuards()
		budgetGuard := findGuardByName(guards, "budget")
		if budgetGuard == nil {
			t.Fatal("Budget guard not found")
		}

		if budgetGuard.Status != ops.GuardStatusWarn {
			t.Errorf("Expected WARN status at 90%% budget, got %s", budgetGuard.Status)
		}
	})

	// Phase 3: Emergency response
	t.Run("EmergencyResponse", func(t *testing.T) {
		// Simulate emergency: disable scanners
		switchManager.SetEmergencySwitch("disable_all_scanners", true)
		switchManager.SetEmergencySwitch("read_only_mode", true)

		// Verify emergency state
		if switchManager.IsScannersEnabled() {
			t.Error("Scanners should be disabled in emergency")
		}

		if !switchManager.IsReadOnlyMode() {
			t.Error("Should be in read-only mode during emergency")
		}

		// Check that any emergency is active
		if !switchManager.HasAnyEmergencyActive() {
			t.Error("Emergency state should be active")
		}
	})

	// Phase 4: Recovery
	t.Run("Recovery", func(t *testing.T) {
		// Clear emergency switches
		switchManager.EnableAllSystems()

		// Verify systems are back online
		if !switchManager.IsScannersEnabled() {
			t.Error("Scanners should be enabled after recovery")
		}

		if switchManager.IsReadOnlyMode() {
			t.Error("Should not be in read-only mode after recovery")
		}

		if switchManager.HasAnyEmergencyActive() {
			t.Error("No emergency should be active after recovery")
		}
	})
}

// Helper function to find guard by name
func findGuardByName(guards []ops.GuardResult, name string) *ops.GuardResult {
	for _, guard := range guards {
		if guard.Name == name {
			return &guard
		}
	}
	return nil
}

// Helper function to check string contains substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) &&
		(s[:len(substr)] == substr ||
			(len(s) > len(substr) && contains(s[1:], substr)))
}
