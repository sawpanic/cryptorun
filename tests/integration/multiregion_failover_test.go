package integration

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/cryptorun/internal/data/validate"
	"github.com/cryptorun/internal/metrics"
	"github.com/cryptorun/internal/replication"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMultiRegionFailover tests the complete failover workflow
func TestMultiRegionFailover(t *testing.T) {
	// Setup test environment
	tempDir := setupTestRegions(t)
	defer os.RemoveAll(tempDir)
	
	// Reset metrics for clean test
	metrics.GlobalDataMetrics.Reset()
	
	// Test case: Warm tier failover
	t.Run("WarmTierFailover", func(t *testing.T) {
		testWarmTierFailover(t, tempDir)
	})
	
	// Test case: Cold tier failover
	t.Run("ColdTierFailover", func(t *testing.T) {
		testColdTierFailover(t, tempDir)
	})
	
	// Test case: Hot tier active-active scenario
	t.Run("HotTierActiveActive", func(t *testing.T) {
		testHotTierActiveActive(t, tempDir)
	})
	
	// Test case: Validation error handling
	t.Run("ValidationErrorHandling", func(t *testing.T) {
		testValidationErrorHandling(t, tempDir)
	})
}

func setupTestRegions(t *testing.T) string {
	tempDir, err := ioutil.TempDir("", "cryptorun_multiregion_test")
	require.NoError(t, err)
	
	// Create region directories
	regions := []string{"us-east-1", "us-west-2", "eu-west-1"}
	for _, region := range regions {
		regionDir := filepath.Join(tempDir, region)
		
		// Create tier directories
		for _, tier := range []string{"hot", "warm", "cold"} {
			tierDir := filepath.Join(regionDir, tier)
			require.NoError(t, os.MkdirAll(tierDir, 0755))
		}
		
		// Initialize region health
		metrics.GlobalDataMetrics.SetRegionHealth(region, 1.0)
	}
	
	return tempDir
}

func testWarmTierFailover(t *testing.T, tempDir string) {
	// Setup: Seed us-east-1 with data, us-west-2 is lagging
	primaryRegion := "us-east-1"
	secondaryRegion := "us-west-2"
	tier := replication.TierWarm
	
	// Create test data in primary region
	primaryData := createTestWarmData(t, tempDir, primaryRegion)
	
	// Set initial lag metrics
	metrics.GlobalDataMetrics.RecordReplicationLag(string(tier), primaryRegion, "kraken", 0.0)
	metrics.GlobalDataMetrics.RecordReplicationLag(string(tier), secondaryRegion, "kraken", 45.0) // 45s lag
	
	// Step 1: Verify initial state
	primaryLag := metrics.GlobalDataMetrics.GetReplicationLag(string(tier), primaryRegion, "kraken")
	secondaryLag := metrics.GlobalDataMetrics.GetReplicationLag(string(tier), secondaryRegion, "kraken")
	
	assert.Equal(t, 0.0, primaryLag, "Primary region should have no lag")
	assert.Equal(t, 45.0, secondaryLag, "Secondary region should have 45s lag")
	
	// Step 2: Create and execute replication plan
	planner := replication.NewPlanner()
	planner.AddHealthCheck(replication.Region(primaryRegion), 1.0)
	planner.AddHealthCheck(replication.Region(secondaryRegion), 0.9)
	
	window := replication.TimeRange{
		From: time.Now().Add(-1 * time.Hour),
		To:   time.Now(),
	}
	
	plan, err := planner.BuildPlan(tier, window, false)
	require.NoError(t, err)
	assert.True(t, len(plan.Steps) > 0, "Plan should have replication steps")
	
	// Step 3: Execute plan (simulate)
	executor := &MockWarmColdExecutor{
		tempDir: tempDir,
	}
	
	for _, step := range plan.Steps {
		if string(step.From) == primaryRegion && string(step.To) == secondaryRegion {
			err := executor.ExecuteStep(step, primaryData)
			require.NoError(t, err)
			
			// Record successful step execution
			metrics.GlobalDataMetrics.IncrementPlanSteps(string(step.Tier), string(step.From), string(step.To))
		}
	}
	
	// Step 4: Verify replication lag decreased
	updatedLag := metrics.GlobalDataMetrics.GetReplicationLag(string(tier), secondaryRegion, "kraken")
	assert.True(t, updatedLag < secondaryLag, "Replication lag should decrease after sync")
	
	// Step 5: Simulate primary region failure
	metrics.GlobalDataMetrics.SetRegionHealth(primaryRegion, 0.3) // Critical health
	
	// Step 6: Promote secondary region
	metrics.GlobalDataMetrics.SetRegionHealth(secondaryRegion, 1.0)
	metrics.GlobalDataMetrics.RecordReplicationLag(string(tier), secondaryRegion, "promoted", 0.0)
	
	// Step 7: Write new data to promoted region
	newData := createTestWarmData(t, tempDir, secondaryRegion)
	assert.NotNil(t, newData)
	
	// Step 8: Simulate primary region recovery
	metrics.GlobalDataMetrics.SetRegionHealth(primaryRegion, 0.9)
	
	// Step 9: Delta reconciliation back to primary
	reconciliationPlan, err := planner.BuildPlan(tier, replication.TimeRange{
		From: time.Now().Add(-30 * time.Minute),
		To:   time.Now(),
	}, false)
	require.NoError(t, err)
	
	// Execute delta sync
	for _, step := range reconciliationPlan.Steps {
		if string(step.From) == secondaryRegion && string(step.To) == primaryRegion {
			err := executor.ExecuteStep(step, newData)
			require.NoError(t, err)
		}
	}
	
	// Step 10: Verify final state meets SLO
	finalLag := metrics.GlobalDataMetrics.GetReplicationLag(string(tier), primaryRegion, "kraken")
	sloThreshold := 60.0 // 60 seconds for warm tier
	assert.True(t, finalLag <= sloThreshold, "Final replication lag should be within SLO")
	
	// Verify metrics were recorded
	stepCount := metrics.GlobalDataMetrics.GetPlanStepsCount(string(tier), primaryRegion, secondaryRegion)
	assert.True(t, stepCount > 0, "Plan steps should be recorded in metrics")
}

func testColdTierFailover(t *testing.T, tempDir string) {
	// Test cold tier with larger files and longer sync times
	primaryRegion := "us-east-1"
	backupRegion := "eu-west-1"
	tier := replication.TierCold
	
	// Create historical parquet files in primary
	coldFiles := createTestColdFiles(t, tempDir, primaryRegion)
	assert.True(t, len(coldFiles) > 0)
	
	// Set initial state - primary healthy, backup lagging
	metrics.GlobalDataMetrics.SetRegionHealth(primaryRegion, 1.0)
	metrics.GlobalDataMetrics.SetRegionHealth(backupRegion, 1.0)
	metrics.GlobalDataMetrics.RecordReplicationLag(string(tier), backupRegion, "primary", 180.0) // 3 minutes lag
	
	// Execute backfill
	planner := replication.NewPlanner()
	window := replication.TimeRange{
		From: time.Now().Add(-24 * time.Hour),
		To:   time.Now(),
	}
	
	plan, err := planner.BuildPlan(tier, window, false)
	require.NoError(t, err)
	
	executor := &MockWarmColdExecutor{tempDir: tempDir}
	
	for _, step := range plan.Steps {
		if string(step.From) == primaryRegion && string(step.To) == backupRegion {
			err := executor.ExecuteStep(step, coldFiles)
			require.NoError(t, err)
		}
	}
	
	// Verify files were replicated
	backupDir := filepath.Join(tempDir, backupRegion, "cold")
	files, err := ioutil.ReadDir(backupDir)
	require.NoError(t, err)
	assert.True(t, len(files) > 0, "Files should be replicated to backup region")
	
	// Simulate disaster - primary region data loss
	primaryDir := filepath.Join(tempDir, primaryRegion, "cold")
	os.RemoveAll(primaryDir) // Simulate data loss
	metrics.GlobalDataMetrics.SetRegionHealth(primaryRegion, 0.0)
	
	// Promote backup region
	metrics.GlobalDataMetrics.SetRegionHealth(backupRegion, 1.0)
	metrics.GlobalDataMetrics.RecordReplicationLag(string(tier), backupRegion, "promoted", 0.0)
	
	// Verify backup region has the data
	files, err = ioutil.ReadDir(backupDir)
	require.NoError(t, err)
	assert.True(t, len(files) > 0, "Backup region should have all data after promotion")
	
	// Recovery: Restore primary region from backup
	require.NoError(t, os.MkdirAll(primaryDir, 0755))
	
	recoveryPlan, err := planner.BuildPlan(tier, window, false)
	require.NoError(t, err)
	
	for _, step := range recoveryPlan.Steps {
		if string(step.From) == backupRegion && string(step.To) == primaryRegion {
			err := executor.ExecuteStep(step, coldFiles)
			require.NoError(t, err)
		}
	}
	
	// Verify recovery
	recoveredFiles, err := ioutil.ReadDir(primaryDir)
	require.NoError(t, err)
	assert.Equal(t, len(files), len(recoveredFiles), "All files should be recovered to primary region")
}

func testHotTierActiveActive(t *testing.T, tempDir string) {
	// Test hot tier active-active scenario with WebSocket simulation
	region1 := "us-east-1"
	region2 := "us-west-2"
	tier := replication.TierHot
	
	// Both regions are active and healthy
	metrics.GlobalDataMetrics.SetRegionHealth(region1, 1.0)
	metrics.GlobalDataMetrics.SetRegionHealth(region2, 1.0)
	
	// Initial lag - both should be very low for hot tier
	metrics.GlobalDataMetrics.RecordReplicationLag(string(tier), region1, "websocket", 0.1)
	metrics.GlobalDataMetrics.RecordReplicationLag(string(tier), region2, "websocket", 0.2)
	
	// Simulate WebSocket data streaming
	hotExecutor := &MockHotExecutor{
		regions: []string{region1, region2},
		tempDir: tempDir,
	}
	
	// Simulate sequence gap in one region
	err := hotExecutor.SimulateSequenceGap(region1, 1000, 1010) // Gap from seq 1000-1010
	require.NoError(t, err)
	
	// Anti-entropy reconciliation should detect and fix the gap
	err = hotExecutor.RunAntiEntropyReconciliation()
	require.NoError(t, err)
	
	// Verify gap was filled
	assert.False(t, hotExecutor.HasSequenceGap(region1), "Sequence gap should be resolved")
	
	// Check that replication lag stayed within hot tier SLO
	lag1 := metrics.GlobalDataMetrics.GetReplicationLag(string(tier), region1, "websocket")
	lag2 := metrics.GlobalDataMetrics.GetReplicationLag(string(tier), region2, "websocket")
	
	hotSLO := 0.5 // 500ms
	assert.True(t, lag1 <= hotSLO, "Region 1 should meet hot tier SLO")
	assert.True(t, lag2 <= hotSLO, "Region 2 should meet hot tier SLO")
}

func testValidationErrorHandling(t *testing.T, tempDir string) {
	// Test validation layer integration with replication
	
	// Create schema validator
	schemaConfig := validate.SchemaConfig{
		RequiredFields: map[string]string{
			"timestamp": "int64",
			"price":     "float64",
			"volume":    "float64",
		},
		Strict: true,
	}
	schemaValidator := validate.NewSchemaValidator(schemaConfig)
	
	// Create staleness validator
	stalenessConfig := validate.StalenessConfig{
		MaxAge: map[string]time.Duration{
			"hot":  5 * time.Second,
			"warm": 60 * time.Second,
			"cold": 5 * time.Minute,
		},
		TimestampField: "timestamp",
		TimestampFormat: "unix",
	}
	stalenessValidator := validate.NewStalenessValidator(stalenessConfig)
	
	// Create anomaly detector
	anomalyConfig := validate.AnomalyConfig{
		MADThreshold:   3.0,
		SpikeThreshold: 5.0,
		WindowSize:     50,
		MinDataPoints:  10,
		PriceFields:    []string{"price"},
		VolumeFields:   []string{"volume"},
		EnableQuarantine: true,
	}
	anomalyChecker := validate.NewAnomalyChecker(anomalyConfig)
	
	// Test data with various issues
	testCases := []struct {
		name           string
		data           map[string]interface{}
		expectSchema   bool
		expectStale    bool
		expectAnomaly  bool
	}{
		{
			name: "valid_data",
			data: map[string]interface{}{
				"timestamp": int64(time.Now().Unix()),
				"price":     100.50,
				"volume":    1000.0,
			},
			expectSchema:  false,
			expectStale:   false,
			expectAnomaly: false,
		},
		{
			name: "missing_required_field",
			data: map[string]interface{}{
				"timestamp": int64(time.Now().Unix()),
				"price":     100.50,
				// Missing volume
			},
			expectSchema:  true,
			expectStale:   false,
			expectAnomaly: false,
		},
		{
			name: "stale_data",
			data: map[string]interface{}{
				"timestamp": int64(time.Now().Add(-10 * time.Minute).Unix()),
				"price":     100.50,
				"volume":    1000.0,
			},
			expectSchema:  false,
			expectStale:   true,
			expectAnomaly: false,
		},
		{
			name: "price_anomaly",
			data: map[string]interface{}{
				"timestamp": int64(time.Now().Unix()),
				"price":     -50.0, // Negative price is invalid
				"volume":    1000.0,
			},
			expectSchema:  false,
			expectStale:   false,
			expectAnomaly: true,
		},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Schema validation
			schemaResult := schemaValidator.ValidateRecord(tc.data)
			if tc.expectSchema {
				assert.True(t, len(schemaResult.Errors) > 0, "Should have schema validation errors")
				metrics.GlobalDataMetrics.IncrementConsistencyErrors("schema")
			} else {
				assert.True(t, len(schemaResult.Errors) == 0, "Should not have schema validation errors")
			}
			
			// Staleness validation
			stalenessResult := stalenessValidator.CheckStaleness(tc.data, "hot")
			if tc.expectStale {
				assert.True(t, stalenessResult.IsStale, "Should be marked as stale")
				metrics.GlobalDataMetrics.IncrementConsistencyErrors("staleness")
			} else {
				assert.False(t, stalenessResult.IsStale, "Should not be marked as stale")
			}
			
			// Anomaly detection
			anomalyResult := anomalyChecker.CheckAnomaly(tc.data, "hot")
			if tc.expectAnomaly {
				assert.True(t, anomalyResult.IsAnomaly, "Should be marked as anomalous")
				metrics.GlobalDataMetrics.IncrementConsistencyErrors("anomaly")
				if anomalyResult.ShouldQuarantine {
					metrics.GlobalDataMetrics.IncrementQuarantine("hot", "us-east-1", string(anomalyResult.AnomalyType))
				}
			} else {
				assert.False(t, anomalyResult.IsAnomaly, "Should not be marked as anomalous")
			}
		})
	}
	
	// Verify metrics were properly recorded
	schemaErrors := metrics.GlobalDataMetrics.GetConsistencyErrorsCount("schema")
	stalenessErrors := metrics.GlobalDataMetrics.GetConsistencyErrorsCount("staleness")  
	anomalyErrors := metrics.GlobalDataMetrics.GetConsistencyErrorsCount("anomaly")
	
	assert.True(t, schemaErrors >= 1, "Schema errors should be recorded")
	assert.True(t, stalenessErrors >= 1, "Staleness errors should be recorded")
	assert.True(t, anomalyErrors >= 1, "Anomaly errors should be recorded")
	
	quarantineCount := metrics.GlobalDataMetrics.GetQuarantineCount("hot", "us-east-1", "corruption")
	assert.True(t, quarantineCount >= 1, "Quarantine count should be recorded")
}

// Mock implementations for testing

type MockWarmColdExecutor struct {
	tempDir string
}

func (e *MockWarmColdExecutor) ExecuteStep(step replication.Step, data interface{}) error {
	// Simulate file copying with delay
	time.Sleep(100 * time.Millisecond)
	
	// Create target directory
	targetDir := filepath.Join(e.tempDir, string(step.To), string(step.Tier))
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return err
	}
	
	// Simulate file replication
	if files, ok := data.([]string); ok {
		for _, file := range files {
			targetFile := filepath.Join(targetDir, filepath.Base(file))
			if err := copyFile(file, targetFile); err != nil {
				metrics.GlobalDataMetrics.IncrementStepFailures(string(step.Tier), string(step.From), string(step.To), "copy_error")
				return err
			}
		}
	}
	
	// Update replication lag to simulate successful sync
	metrics.GlobalDataMetrics.RecordReplicationLag(string(step.Tier), string(step.To), "primary", 0.5)
	
	// Record duration
	metrics.GlobalDataMetrics.RecordStepDuration(string(step.Tier), step.EstimatedDuration.Seconds())
	
	return nil
}

type MockHotExecutor struct {
	regions       []string
	tempDir       string
	sequenceGaps  map[string][][2]int64 // region -> list of gaps [start, end]
}

func (e *MockHotExecutor) SimulateSequenceGap(region string, start, end int64) error {
	if e.sequenceGaps == nil {
		e.sequenceGaps = make(map[string][][2]int64)
	}
	
	e.sequenceGaps[region] = append(e.sequenceGaps[region], [2]int64{start, end})
	
	// Increase replication lag due to gap
	metrics.GlobalDataMetrics.RecordReplicationLag("hot", region, "websocket", 2.0)
	
	return nil
}

func (e *MockHotExecutor) RunAntiEntropyReconciliation() error {
	// Simulate anti-entropy process fixing gaps
	time.Sleep(500 * time.Millisecond)
	
	for region := range e.sequenceGaps {
		// Clear gaps
		delete(e.sequenceGaps, region)
		
		// Restore low lag
		metrics.GlobalDataMetrics.RecordReplicationLag("hot", region, "websocket", 0.1)
	}
	
	return nil
}

func (e *MockHotExecutor) HasSequenceGap(region string) bool {
	gaps, exists := e.sequenceGaps[region]
	return exists && len(gaps) > 0
}

// Helper functions

func createTestWarmData(t *testing.T, tempDir, region string) []string {
	warmDir := filepath.Join(tempDir, region, "warm")
	
	// Create sample data files
	files := []string{
		filepath.Join(warmDir, "cache_snapshot_2025090701.json"),
		filepath.Join(warmDir, "aggregated_metrics_2025090701.json"),
	}
	
	for _, file := range files {
		data := map[string]interface{}{
			"timestamp": time.Now().Unix(),
			"region":    region,
			"data":      "sample warm tier data",
		}
		
		jsonData, err := json.Marshal(data)
		require.NoError(t, err)
		
		err = ioutil.WriteFile(file, jsonData, 0644)
		require.NoError(t, err)
	}
	
	return files
}

func createTestColdFiles(t *testing.T, tempDir, region string) []string {
	coldDir := filepath.Join(tempDir, region, "cold")
	
	// Create sample parquet-like files (simulated with JSON)
	files := []string{
		filepath.Join(coldDir, "historical_20250906.parquet"),
		filepath.Join(coldDir, "backtest_data_20250906.parquet"),
		filepath.Join(coldDir, "regime_history_20250906.parquet"),
	}
	
	for _, file := range files {
		// Simulate large historical data file
		data := make([]map[string]interface{}, 1000)
		for i := 0; i < 1000; i++ {
			data[i] = map[string]interface{}{
				"timestamp": time.Now().Add(time.Duration(-i) * time.Minute).Unix(),
				"symbol":    "BTC-USD",
				"price":     50000.0 + float64(i%1000),
				"volume":    1000.0 + float64(i%500),
			}
		}
		
		jsonData, err := json.Marshal(data)
		require.NoError(t, err)
		
		err = ioutil.WriteFile(file, jsonData, 0644)
		require.NoError(t, err)
	}
	
	return files
}

func copyFile(src, dst string) error {
	input, err := ioutil.ReadFile(src)
	if err != nil {
		return err
	}
	
	return ioutil.WriteFile(dst, input, 0644)
}