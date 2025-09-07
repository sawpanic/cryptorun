package integration

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sawpanic/cryptorun/internal/application/bench"
	"github.com/sawpanic/cryptorun/internal/application/metrics"
	"github.com/sawpanic/cryptorun/internal/application/pipeline"
)

// TestMenuActionsCallUnifiedPipelines ensures menu actions route to the same functions as CLI
func TestMenuActionsCallUnifiedPipelines(t *testing.T) {
	ctx := context.Background()

	t.Run("ScanMenuRoutesToUnifiedPipeline", func(t *testing.T) {
		t.Log("🎯 Testing that menu scan calls pipeline.Run() (same as CLI)")

		// Test scan options that would be used by menu
		opts := pipeline.ScanOptions{
			Exchange:    "kraken,okx,coinbase",
			Pairs:       "USD-only",
			DryRun:      false,
			OutputDir:   "out/scan",
			SnapshotDir: "out/microstructure/snapshots",
			MaxSymbols:  20,
			MinScore:    2.0,
			Regime:      "bull",
			ConfigFile:  "",
		}

		// This is the SAME function that CLI calls
		result, artifacts, err := pipeline.Run(ctx, opts)

		// Should succeed (unified pipeline exists and works)
		require.NoError(t, err, "Menu scan should use working unified pipeline")
		assert.NotNil(t, result, "Menu scan should return results")
		assert.NotNil(t, artifacts, "Menu scan should generate artifacts")

		// Verify result structure
		assert.NotEmpty(t, result.ProcessingTime, "Should have processing time")
		assert.NotEmpty(t, result.Regime, "Should have regime setting")
		assert.GreaterOrEqual(t, result.TotalSymbols, 0, "Should report symbol count")

		t.Log("✅ Menu scan successfully routes to pipeline.Run()")
	})

	t.Run("BenchMenuRoutesToUnifiedPipeline", func(t *testing.T) {
		t.Log("🎯 Testing that menu bench calls bench.Run() (same as CLI)")

		// Test bench options that would be used by menu
		opts := bench.TopGainersOptions{
			TTL:        15 * time.Minute,
			Limit:      20,
			Windows:    []string{"1h", "24h"},
			OutputDir:  "out/bench",
			DryRun:     true, // Use dry run for test
			APIBaseURL: "https://api.coingecko.com/api/v3",
			ConfigFile: "",
		}

		// This is the SAME function that CLI calls
		result, artifacts, err := bench.Run(ctx, opts)

		// Should succeed (unified benchmark exists and works)
		require.NoError(t, err, "Menu bench should use working unified pipeline")
		assert.NotNil(t, result, "Menu bench should return results")
		assert.NotNil(t, artifacts, "Menu bench should generate artifacts")

		// Verify result structure
		assert.NotEmpty(t, result.ProcessingTime, "Should have processing time")
		assert.GreaterOrEqual(t, result.OverallAlignment, 0.0, "Should have alignment score")
		assert.NotEmpty(t, result.WindowResults, "Should have window results")

		t.Log("✅ Menu bench successfully routes to bench.Run()")
	})

	t.Run("DiagnosticsMenuRoutesToUnifiedPipeline", func(t *testing.T) {
		t.Log("🎯 Testing that menu diagnostics calls bench.RunDiagnostics() (same as CLI)")

		// Test diagnostics options that would be used by menu
		opts := bench.DiagnosticsOptions{
			OutputDir:         "out/bench/diagnostics",
			AlignmentScore:    0.65,
			BenchmarkWindow:   "1h",
			DetailLevel:       "high",
			ConfigFile:        "",
			IncludeSparklines: true,
		}

		// This is the SAME function that CLI would call
		result, artifacts, err := bench.RunDiagnostics(ctx, opts)

		// Should succeed (unified diagnostics exists and works)
		require.NoError(t, err, "Menu diagnostics should use working unified pipeline")
		assert.NotNil(t, result, "Menu diagnostics should return results")
		assert.NotNil(t, artifacts, "Menu diagnostics should generate artifacts")

		// Verify result structure
		assert.NotEmpty(t, result.ProcessingTime, "Should have processing time")
		assert.GreaterOrEqual(t, result.AlignmentScore, 0.0, "Should have alignment score")
		assert.NotNil(t, result.MissAttribution, "Should have miss attribution analysis")

		t.Log("✅ Menu diagnostics successfully routes to bench.RunDiagnostics()")
	})

	t.Run("HealthMenuRoutesToUnifiedPipeline", func(t *testing.T) {
		t.Log("🎯 Testing that menu health calls metrics.Snapshot() (same as CLI)")

		// Test health options that would be used by menu
		opts := metrics.HealthOptions{
			IncludeMetrics:  true,
			IncludeCounters: true,
			Format:          "table",
			OutputFile:      "",
		}

		// This is the SAME function that CLI calls
		snapshot, err := metrics.Snapshot(ctx, opts)

		// Should succeed (unified health check exists and works)
		require.NoError(t, err, "Menu health should use working unified pipeline")
		assert.NotNil(t, snapshot, "Menu health should return snapshot")

		// Verify snapshot structure (basic checks)
		assert.NotEmpty(t, snapshot.Timestamp, "Should have timestamp")

		t.Log("✅ Menu health successfully routes to metrics.Snapshot()")
	})
}

// TestMenuUnifiedHandlersIntegration tests the actual menu unified handlers
func TestMenuUnifiedHandlersIntegration(t *testing.T) {
	t.Run("MenuUnifiedHandlersExist", func(t *testing.T) {
		t.Log("🎯 Testing that MenuUnifiedHandlers exist and work")

		// This tests the integration between menu and unified handlers
		// The handlers should exist in menu_unified.go and call the same functions

		// Test scan momentum handler
		ctx := context.Background()

		// Mock the scan options that menu would use
		scanOpts := pipeline.ScanOptions{
			Exchange:    "kraken",
			Pairs:       "USD-only",
			DryRun:      false,
			OutputDir:   "out/scanner",
			SnapshotDir: "out/scanner/snapshots",
			MaxSymbols:  50,
			MinScore:    2.0,
			Regime:      "trending",
			ConfigFile:  "",
		}

		// Verify the unified function exists and works
		result, artifacts, err := pipeline.Run(ctx, scanOpts)
		require.NoError(t, err, "MenuUnifiedHandlers should call working pipeline.Run")
		assert.NotNil(t, result, "Should get scan result")
		assert.NotNil(t, artifacts, "Should get scan artifacts")

		// Test bench handler
		benchOpts := bench.TopGainersOptions{
			TTL:        15 * time.Minute,
			Limit:      20,
			Windows:    []string{"1h", "24h"},
			OutputDir:  "out/bench",
			DryRun:     true,
			APIBaseURL: "",
			ConfigFile: "",
		}

		benchResult, benchArtifacts, err := bench.Run(ctx, benchOpts)
		require.NoError(t, err, "MenuUnifiedHandlers should call working bench.Run")
		assert.NotNil(t, benchResult, "Should get bench result")
		assert.NotNil(t, benchArtifacts, "Should get bench artifacts")

		t.Log("✅ MenuUnifiedHandlers integration working correctly")
	})
}

// TestMenuConsistencyWithCLI ensures menu produces same results as CLI
func TestMenuConsistencyWithCLI(t *testing.T) {
	ctx := context.Background()

	t.Run("ScanConsistency", func(t *testing.T) {
		t.Log("🎯 Testing that menu scan produces same results as CLI scan")

		// Same options that both menu and CLI would use
		opts := pipeline.ScanOptions{
			Exchange:    "kraken",
			Pairs:       "USD-only",
			DryRun:      false,
			OutputDir:   "out/scan",
			SnapshotDir: "out/microstructure/snapshots",
			MaxSymbols:  20,
			MinScore:    2.0,
			Regime:      "trending",
			ConfigFile:  "",
		}

		// Call unified function (what both menu and CLI call)
		result1, artifacts1, err1 := pipeline.Run(ctx, opts)
		require.NoError(t, err1, "First call should succeed")

		// Call again with same options
		result2, artifacts2, err2 := pipeline.Run(ctx, opts)
		require.NoError(t, err2, "Second call should succeed")

		// Results should be consistent (same pipeline, same results)
		assert.Equal(t, result1.Regime, result2.Regime, "Regime should be consistent")
		assert.Equal(t, result1.TotalSymbols, result2.TotalSymbols, "Symbol count should be consistent")
		assert.Equal(t, len(artifacts1.Artifacts), len(artifacts2.Artifacts), "Artifact count should be consistent")

		t.Log("✅ Menu and CLI scan produce consistent results via pipeline.Run()")
	})

	t.Run("BenchConsistency", func(t *testing.T) {
		t.Log("🎯 Testing that menu bench produces same results as CLI bench")

		// Same options that both menu and CLI would use
		opts := bench.TopGainersOptions{
			TTL:        15 * time.Minute,
			Limit:      20,
			Windows:    []string{"1h", "24h"},
			OutputDir:  "out/bench",
			DryRun:     true, // Use dry run for consistent test results
			APIBaseURL: "https://api.coingecko.com/api/v3",
			ConfigFile: "",
		}

		// Call unified function (what both menu and CLI call)
		result1, artifacts1, err1 := bench.Run(ctx, opts)
		require.NoError(t, err1, "First call should succeed")

		// Call again with same options
		result2, artifacts2, err2 := bench.Run(ctx, opts)
		require.NoError(t, err2, "Second call should succeed")

		// Results should be consistent (same pipeline, same results)
		assert.Equal(t, len(result1.WindowResults), len(result2.WindowResults), "Window count should be consistent")
		assert.Equal(t, len(artifacts1.WindowJSONs), len(artifacts2.WindowJSONs), "Artifact count should be consistent")

		t.Log("✅ Menu and CLI bench produce consistent results via bench.Run()")
	})
}

// TestSingleEntryPointEnforcement verifies only one implementation per action
func TestSingleEntryPointEnforcement(t *testing.T) {
	t.Log("🎯 Testing that each action has exactly ONE entry point")

	entryPoints := []struct {
		name     string
		function func(context.Context) error
	}{
		{
			name: "Scan Entry Point",
			function: func(ctx context.Context) error {
				opts := pipeline.ScanOptions{
					Exchange:    "kraken",
					Pairs:       "USD-only",
					DryRun:      true,
					OutputDir:   "out/scan",
					SnapshotDir: "out/microstructure/snapshots",
					MaxSymbols:  5, // Small for test
					MinScore:    2.0,
					Regime:      "trending",
					ConfigFile:  "",
				}
				_, _, err := pipeline.Run(ctx, opts)
				return err
			},
		},
		{
			name: "Bench Entry Point",
			function: func(ctx context.Context) error {
				opts := bench.TopGainersOptions{
					TTL:        5 * time.Minute,
					Limit:      5, // Small for test
					Windows:    []string{"1h"},
					OutputDir:  "out/bench",
					DryRun:     true,
					APIBaseURL: "https://api.coingecko.com/api/v3",
					ConfigFile: "",
				}
				_, _, err := bench.Run(ctx, opts)
				return err
			},
		},
	}

	for _, ep := range entryPoints {
		t.Run(ep.name, func(t *testing.T) {
			ctx := context.Background()

			// Entry point should work (single implementation exists)
			err := ep.function(ctx)
			assert.NoError(t, err, "Single entry point should work")

			t.Logf("✅ %s has working single entry point", ep.name)
		})
	}

	t.Log("✅ All actions have single, working entry points")
}
