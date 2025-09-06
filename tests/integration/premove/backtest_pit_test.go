package premove

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"cryptorun/src/application/premove"
)

func TestBacktestPITLoader_DeterministicFixtures(t *testing.T) {
	t.Run("load_pit_records_from_jsonl", func(t *testing.T) {
		// This test expects a PIT loader that doesn't exist yet
		loader := premove.NewPITRecordLoader(premove.PITLoaderConfig{
			ArtifactsPath:    "internal/testdata/premove",
			ValidateSchema:   true,
			DeduplicateRecords: true,
		})
		
		// Load PIT records from fixture file
		records, err := loader.LoadFromFile("pit_sample.jsonl")
		if err != nil {
			t.Errorf("Failed to load PIT records: %v", err)
		}
		
		if len(records) == 0 {
			t.Error("Expected PIT records from fixture file")
		}
		
		// Validate record structure
		for i, record := range records {
			if record.Timestamp.IsZero() {
				t.Errorf("Record %d missing timestamp", i)
			}
			
			if record.Symbol == "" {
				t.Errorf("Record %d missing symbol", i)
			}
			
			if record.Score < 0 || record.Score > 120 {
				t.Errorf("Record %d invalid score: %.2f", i, record.Score)
			}
			
			if record.State == "" {
				t.Errorf("Record %d missing state", i)
			}
			
			if record.Regime == "" {
				t.Errorf("Record %d missing regime", i)
			}
		}
		
		// Check temporal ordering
		for i := 1; i < len(records); i++ {
			if records[i].Timestamp.Before(records[i-1].Timestamp) {
				t.Errorf("Records not in temporal order at index %d", i)
			}
		}
	})

	t.Run("validate_movement_outcomes", func(t *testing.T) {
		validator := premove.NewOutcomeValidator(premove.ValidationConfig{
			MovementThreshold:   0.05, // 5%
			TimeHorizon:        48 * time.Hour,
			RequiredDataPoints: 10,
		})
		
		// Load sample records
		loader := premove.NewPITRecordLoader(premove.PITLoaderConfig{
			ArtifactsPath: "internal/testdata/premove",
		})
		
		records, err := loader.LoadFromFile("pit_sample.jsonl")
		if err != nil {
			t.Skipf("Skipping test - fixture file not available: %v", err)
		}
		
		// Validate outcomes against price data
		validationResults, err := validator.ValidateOutcomes(records)
		if err != nil {
			t.Errorf("Outcome validation failed: %v", err)
		}
		
		if len(validationResults.ValidRecords) == 0 {
			t.Error("Expected some valid records from fixture")
		}
		
		// Check validation metrics
		totalRecords := len(validationResults.ValidRecords) + len(validationResults.InvalidRecords)
		validationRate := float64(len(validationResults.ValidRecords)) / float64(totalRecords)
		
		if validationRate < 0.8 {
			t.Errorf("Low validation rate: %.2f%% valid", validationRate*100)
		}
		
		// Check for common validation errors
		errorCounts := make(map[string]int)
		for _, invalid := range validationResults.InvalidRecords {
			errorCounts[invalid.Reason]++
		}
		
		t.Logf("Validation error breakdown: %+v", errorCounts)
	})

	t.Run("temporal_consistency_check", func(t *testing.T) {
		consistencyChecker := premove.NewTemporalConsistencyChecker(premove.ConsistencyConfig{
			MaxGapMinutes:    30, // No gaps >30 minutes
			MinRecordSpacing: time.Minute,
			DeduplicationWindow: 5 * time.Minute,
		})
		
		// Create sample records with various temporal issues
		testRecords := []premove.PITRecord{
			{
				Timestamp: time.Date(2025, 9, 1, 10, 0, 0, 0, time.UTC),
				Symbol:    "BTCUSD",
				Score:     80.0,
				State:     "PRIME",
				Regime:    "trending_bull",
			},
			{
				Timestamp: time.Date(2025, 9, 1, 10, 1, 0, 0, time.UTC),
				Symbol:    "BTCUSD", 
				Score:     81.0,
				State:     "PRIME",
				Regime:    "trending_bull",
			},
			// Large gap
			{
				Timestamp: time.Date(2025, 9, 1, 11, 0, 0, 0, time.UTC),
				Symbol:    "BTCUSD",
				Score:     75.0,
				State:     "PREPARE",
				Regime:    "choppy",
			},
			// Duplicate (within deduplication window)
			{
				Timestamp: time.Date(2025, 9, 1, 11, 2, 0, 0, time.UTC),
				Symbol:    "BTCUSD",
				Score:     75.0,
				State:     "PREPARE", 
				Regime:    "choppy",
			},
		}
		
		analysis, err := consistencyChecker.CheckConsistency(testRecords)
		if err != nil {
			t.Errorf("Consistency check failed: %v", err)
		}
		
		if len(analysis.Gaps) == 0 {
			t.Error("Should detect temporal gaps in test data")
		}
		
		if len(analysis.Duplicates) == 0 {
			t.Error("Should detect potential duplicates in test data")
		}
		
		// Check gap detection accuracy
		for _, gap := range analysis.Gaps {
			if gap.Duration < 30*time.Minute {
				t.Errorf("Should not flag gaps < 30 minutes, found %.1f minutes", 
					gap.Duration.Minutes())
			}
		}
	})

	t.Run("regime_transition_tracking", func(t *testing.T) {
		transitionTracker := premove.NewRegimeTransitionTracker(premove.TransitionConfig{
			MinRegimeDuration: 2 * time.Hour,
			TransitionWindow:  30 * time.Minute,
			ValidRegimes:     []string{"trending_bull", "choppy", "high_vol", "risk_off"},
		})
		
		// Create records showing regime transitions
		transitionRecords := []premove.PITRecord{
			// Bull regime
			{Timestamp: time.Date(2025, 9, 1, 9, 0, 0, 0, time.UTC), Regime: "trending_bull", Symbol: "BTCUSD", Score: 85.0},
			{Timestamp: time.Date(2025, 9, 1, 10, 0, 0, 0, time.UTC), Regime: "trending_bull", Symbol: "BTCUSD", Score: 87.0},
			{Timestamp: time.Date(2025, 9, 1, 11, 0, 0, 0, time.UTC), Regime: "trending_bull", Symbol: "BTCUSD", Score: 83.0},
			
			// Transition to choppy
			{Timestamp: time.Date(2025, 9, 1, 12, 0, 0, 0, time.UTC), Regime: "choppy", Symbol: "BTCUSD", Score: 70.0},
			{Timestamp: time.Date(2025, 9, 1, 13, 0, 0, 0, time.UTC), Regime: "choppy", Symbol: "BTCUSD", Score: 68.0},
			{Timestamp: time.Date(2025, 9, 1, 14, 0, 0, 0, time.UTC), Regime: "choppy", Symbol: "BTCUSD", Score: 72.0},
		}
		
		transitions, err := transitionTracker.TrackTransitions(transitionRecords)
		if err != nil {
			t.Errorf("Regime transition tracking failed: %v", err)
		}
		
		if len(transitions) == 0 {
			t.Error("Should detect regime transitions in test data")
		}
		
		// Validate transition structure
		for i, transition := range transitions {
			if transition.FromRegime == transition.ToRegime {
				t.Errorf("Transition %d: from and to regime are the same", i)
			}
			
			if transition.TransitionTime.IsZero() {
				t.Errorf("Transition %d: missing transition time", i)
			}
			
			if transition.Duration <= 0 {
				t.Errorf("Transition %d: invalid duration", i)
			}
		}
		
		// Check for expected transition
		foundBullToChoppy := false
		for _, transition := range transitions {
			if transition.FromRegime == "trending_bull" && transition.ToRegime == "choppy" {
				foundBullToChoppy = true
				break
			}
		}
		
		if !foundBullToChoppy {
			t.Error("Should detect trending_bull -> choppy transition")
		}
	})

	t.Run("deterministic_replay_execution", func(t *testing.T) {
		replayEngine := premove.NewDeterministicReplayEngine(premove.ReplayConfig{
			StrictTiming:      true,
			ValidateSignatures: true,
			ReproducibilityMode: true,
		})
		
		// Load fixture data
		pitFile := filepath.Join("internal", "testdata", "premove", "pit_sample.jsonl")
		if _, err := os.Stat(pitFile); os.IsNotExist(err) {
			t.Skip("PIT sample fixture file not available")
		}
		
		// Execute deterministic replay
		replay, err := replayEngine.ExecuteReplay(pitFile)
		if err != nil {
			t.Errorf("Deterministic replay failed: %v", err)
		}
		
		if len(replay.ProcessedRecords) == 0 {
			t.Error("Expected processed records from replay")
		}
		
		// Verify deterministic properties
		if replay.ProcessingHash == "" {
			t.Error("Missing processing hash for deterministic verification")
		}
		
		if replay.RecordCount != len(replay.ProcessedRecords) {
			t.Errorf("Record count mismatch: expected %d, got %d", 
				len(replay.ProcessedRecords), replay.RecordCount)
		}
		
		// Run replay again - should get identical results
		replay2, err := replayEngine.ExecuteReplay(pitFile)
		if err != nil {
			t.Errorf("Second replay failed: %v", err)
		}
		
		if replay.ProcessingHash != replay2.ProcessingHash {
			t.Error("Replay should be deterministic - different hashes")
		}
		
		if replay.RecordCount != replay2.RecordCount {
			t.Error("Replay should be deterministic - different record counts")
		}
	})

	t.Run("parallel_batch_processing", func(t *testing.T) {
		batchProcessor := premove.NewBatchProcessor(premove.BatchConfig{
			BatchSize:       100,
			MaxConcurrency:  4,
			ErrorTolerance:  0.05, // 5% error tolerance
		})
		
		// Create large dataset for batch processing
		largeDataset := make([]premove.PITRecord, 1000)
		baseTime := time.Date(2025, 9, 1, 0, 0, 0, 0, time.UTC)
		
		for i := 0; i < 1000; i++ {
			largeDataset[i] = premove.PITRecord{
				Timestamp: baseTime.Add(time.Duration(i) * time.Minute),
				Symbol:    "BTCUSD",
				Score:     60.0 + float64(i%40),
				State:     []string{"WATCH", "PREPARE", "PRIME"}[i%3],
				Regime:    []string{"trending_bull", "choppy", "high_vol"}[i%3],
				Movement:  i%5 == 0, // 20% movement rate
			}
		}
		
		// Process in batches
		result, err := batchProcessor.ProcessBatches(largeDataset)
		if err != nil {
			t.Errorf("Batch processing failed: %v", err)
		}
		
		if result.TotalProcessed != 1000 {
			t.Errorf("Expected 1000 processed records, got %d", result.TotalProcessed)
		}
		
		// Check error rate is within tolerance
		errorRate := float64(result.ErrorCount) / float64(result.TotalProcessed)
		if errorRate > 0.05 {
			t.Errorf("Error rate %.2f%% exceeds 5%% tolerance", errorRate*100)
		}
		
		// Verify batch statistics
		if len(result.BatchStats) == 0 {
			t.Error("Expected batch statistics")
		}
		
		totalBatches := (1000 + 100 - 1) / 100 // Ceiling division
		if len(result.BatchStats) != totalBatches {
			t.Errorf("Expected %d batches, got %d", totalBatches, len(result.BatchStats))
		}
		
		// Check processing time is reasonable
		if result.TotalProcessingTime > 10*time.Second {
			t.Errorf("Processing took too long: %v", result.TotalProcessingTime)
		}
	})
}

func TestBacktestPITLoader_FixtureValidation(t *testing.T) {
	t.Run("validate_required_fixtures", func(t *testing.T) {
		requiredFiles := []string{
			"pit_sample.jsonl",
			"bars.csv", 
			"trades.csv",
		}
		
		fixtureDir := filepath.Join("internal", "testdata", "premove")
		
		for _, filename := range requiredFiles {
			fullPath := filepath.Join(fixtureDir, filename)
			if _, err := os.Stat(fullPath); os.IsNotExist(err) {
				t.Errorf("Required fixture file missing: %s", fullPath)
			}
		}
	})

	t.Run("validate_jsonl_format", func(t *testing.T) {
		pitFile := filepath.Join("internal", "testdata", "premove", "pit_sample.jsonl")
		if _, err := os.Stat(pitFile); os.IsNotExist(err) {
			t.Skip("PIT sample fixture not available")
		}
		
		file, err := os.Open(pitFile)
		if err != nil {
			t.Fatalf("Failed to open fixture file: %v", err)
		}
		defer file.Close()
		
		scanner := bufio.NewScanner(file)
		lineNum := 0
		
		for scanner.Scan() {
			lineNum++
			line := strings.TrimSpace(scanner.Text())
			
			// Skip empty lines and comments
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			
			// Validate JSON structure
			var record premove.PITRecord
			if err := json.Unmarshal([]byte(line), &record); err != nil {
				t.Errorf("Invalid JSON on line %d: %v", lineNum, err)
			}
			
			// Validate required fields
			if record.Timestamp.IsZero() {
				t.Errorf("Line %d: missing timestamp", lineNum)
			}
			
			if record.Symbol == "" {
				t.Errorf("Line %d: missing symbol", lineNum)
			}
		}
		
		if err := scanner.Err(); err != nil {
			t.Errorf("Error reading fixture file: %v", err)
		}
		
		if lineNum == 0 {
			t.Error("Fixture file appears to be empty")
		}
	})

	t.Run("validate_csv_format", func(t *testing.T) {
		csvFiles := []string{"bars.csv", "trades.csv"}
		
		for _, filename := range csvFiles {
			csvFile := filepath.Join("internal", "testdata", "premove", filename)
			if _, err := os.Stat(csvFile); os.IsNotExist(err) {
				t.Errorf("CSV fixture missing: %s", filename)
				continue
			}
			
			file, err := os.Open(csvFile)
			if err != nil {
				t.Errorf("Failed to open %s: %v", filename, err)
				continue
			}
			defer file.Close()
			
			scanner := bufio.NewScanner(file)
			lineNum := 0
			
			for scanner.Scan() {
				lineNum++
				line := strings.TrimSpace(scanner.Text())
				
				if line == "" {
					continue
				}
				
				// Check CSV format (basic validation)
				fields := strings.Split(line, ",")
				if len(fields) < 3 {
					t.Errorf("%s line %d: insufficient fields", filename, lineNum)
				}
				
				// First line should be header
				if lineNum == 1 {
					expectedHeaders := map[string][]string{
						"bars.csv":   {"timestamp", "open", "high", "low", "close", "volume"},
						"trades.csv": {"timestamp", "price", "size", "side"},
					}
					
					if expected, exists := expectedHeaders[filename]; exists {
						for i, expectedHeader := range expected {
							if i < len(fields) && !strings.Contains(strings.ToLower(fields[i]), expectedHeader) {
								t.Errorf("%s missing expected header '%s'", filename, expectedHeader)
							}
						}
					}
				}
			}
			
			if lineNum <= 1 {
				t.Errorf("%s appears to have no data rows", filename)
			}
		}
	})

	t.Run("cross_validate_timestamps", func(t *testing.T) {
		// This test would cross-validate timestamps across PIT records and price data
		// to ensure temporal consistency between fixtures
		
		validator := premove.NewCrossTimestampValidator(premove.CrossValidationConfig{
			ToleranceMinutes: 1, // 1 minute tolerance
			RequireExactMatch: false,
		})
		
		pitFile := filepath.Join("internal", "testdata", "premove", "pit_sample.jsonl")
		barsFile := filepath.Join("internal", "testdata", "premove", "bars.csv")
		
		// Skip if fixtures not available
		if _, err := os.Stat(pitFile); os.IsNotExist(err) {
			t.Skip("PIT fixture not available")
		}
		
		if _, err := os.Stat(barsFile); os.IsNotExist(err) {
			t.Skip("Bars fixture not available")
		}
		
		validation, err := validator.CrossValidate(pitFile, barsFile)
		if err != nil {
			t.Errorf("Cross validation failed: %v", err)
		}
		
		// Check alignment metrics
		if validation.AlignmentRate < 0.8 {
			t.Errorf("Low timestamp alignment rate: %.2f%%", validation.AlignmentRate*100)
		}
		
		if len(validation.OrphanedRecords) > len(validation.MatchedRecords)/10 {
			t.Error("Too many orphaned records - poor timestamp alignment")
		}
	})
}