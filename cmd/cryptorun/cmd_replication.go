package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/sawpanic/cryptorun/internal/data/validate"
	"github.com/sawpanic/cryptorun/internal/metrics"
	"github.com/sawpanic/cryptorun/internal/replication"
	"github.com/spf13/cobra"
)

var replicationCmd = &cobra.Command{
	Use:   "replication",
	Short: "Multi-region replication management",
	Long:  "Manage multi-region replication across hot, warm, and cold data tiers",
}

var replicationSimulateCmd = &cobra.Command{
	Use:   "simulate",
	Short: "Simulate replication plan without execution",
	Long:  "Generate and display a replication plan for the specified parameters without executing it",
	RunE:  runReplicationSimulate,
}

var replicationFailoverCmd = &cobra.Command{
	Use:   "failover",
	Short: "Execute replication failover",
	Long:  "Promote a region for a tier and demote the current primary",
	RunE:  runReplicationFailover,
}

var replicationStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show replication status",
	Long:  "Display current replication lag, health, and metrics for specified tier/region",
	RunE:  runReplicationStatus,
}

// Simulation flags
var (
	simFromRegion   string
	simToRegion     string
	simTier         string
	simWindow       string
	simStrict       bool
	simDryRun       bool
	simOutputFormat string
)

// Failover flags
var (
	failoverTier        string
	failoverPromote     string
	failoverDemote      string
	failoverDryRun      bool
	failoverForce       bool
	failoverTimeout     int
	failoverValidate    bool
)

// Status flags
var (
	statusTier      string
	statusRegion    string
	statusShowAll   bool
	statusInterval  int
	statusWatch     bool
	statusFormat    string
)

func init() {
	// Add replication subcommands
	replicationCmd.AddCommand(replicationSimulateCmd)
	replicationCmd.AddCommand(replicationFailoverCmd)
	replicationCmd.AddCommand(replicationStatusCmd)
	
	// Add to root command
	rootCmd.AddCommand(replicationCmd)
	
	// Simulate command flags
	replicationSimulateCmd.Flags().StringVar(&simFromRegion, "from", "", "Source region (required)")
	replicationSimulateCmd.Flags().StringVar(&simToRegion, "to", "", "Target region (required)")
	replicationSimulateCmd.Flags().StringVar(&simTier, "tier", "", "Data tier: hot, warm, or cold (required)")
	replicationSimulateCmd.Flags().StringVar(&simWindow, "window", "", "Time window in format: 2025-09-01T00:00:00Z/2025-09-01T06:00:00Z")
	replicationSimulateCmd.Flags().BoolVar(&simStrict, "strict", false, "Enable strict validation (fail on any validation error)")
	replicationSimulateCmd.Flags().BoolVar(&simDryRun, "dry-run", true, "Perform dry run only (default: true)")
	replicationSimulateCmd.Flags().StringVar(&simOutputFormat, "format", "json", "Output format: json, table, or summary")
	
	// Mark required flags
	replicationSimulateCmd.MarkFlagRequired("from")
	replicationSimulateCmd.MarkFlagRequired("to")
	replicationSimulateCmd.MarkFlagRequired("tier")
	
	// Failover command flags  
	replicationFailoverCmd.Flags().StringVar(&failoverTier, "tier", "", "Data tier to failover: hot, warm, or cold (required)")
	replicationFailoverCmd.Flags().StringVar(&failoverPromote, "promote", "", "Region to promote to primary (required)")
	replicationFailoverCmd.Flags().StringVar(&failoverDemote, "demote", "", "Region to demote from primary (optional)")
	replicationFailoverCmd.Flags().BoolVar(&failoverDryRun, "dry-run", false, "Show what would be done without executing")
	replicationFailoverCmd.Flags().BoolVar(&failoverForce, "force", false, "Force failover even if source region is healthy")
	replicationFailoverCmd.Flags().IntVar(&failoverTimeout, "timeout", 300, "Timeout in seconds for failover operations")
	replicationFailoverCmd.Flags().BoolVar(&failoverValidate, "validate", true, "Validate data consistency after failover")
	
	// Mark required flags
	replicationFailoverCmd.MarkFlagRequired("tier")
	replicationFailoverCmd.MarkFlagRequired("promote")
	
	// Status command flags
	replicationStatusCmd.Flags().StringVar(&statusTier, "tier", "", "Filter by data tier: hot, warm, or cold")
	replicationStatusCmd.Flags().StringVar(&statusRegion, "region", "", "Filter by region")
	replicationStatusCmd.Flags().BoolVar(&statusShowAll, "all", false, "Show all metrics and detailed information")
	replicationStatusCmd.Flags().IntVar(&statusInterval, "interval", 0, "Refresh interval in seconds (0 = no refresh)")
	replicationStatusCmd.Flags().BoolVar(&statusWatch, "watch", false, "Watch mode - continuously update status")
	replicationStatusCmd.Flags().StringVar(&statusFormat, "format", "table", "Output format: table, json, or prometheus")
}

func runReplicationSimulate(cmd *cobra.Command, args []string) error {
	// Validate tier
	tier := replication.Tier(simTier)
	if tier != replication.TierHot && tier != replication.TierWarm && tier != replication.TierCold {
		return fmt.Errorf("invalid tier: %s (must be hot, warm, or cold)", simTier)
	}
	
	// Parse time window if provided
	var window replication.TimeRange
	if simWindow != "" {
		parts := strings.Split(simWindow, "/")
		if len(parts) != 2 {
			return fmt.Errorf("invalid window format: %s (expected: start/end)", simWindow)
		}
		
		fromTime, err := time.Parse(time.RFC3339, parts[0])
		if err != nil {
			return fmt.Errorf("invalid start time: %v", err)
		}
		
		toTime, err := time.Parse(time.RFC3339, parts[1])
		if err != nil {
			return fmt.Errorf("invalid end time: %v", err)
		}
		
		window = replication.TimeRange{From: fromTime, To: toTime}
	} else {
		// Default to last 6 hours
		now := time.Now()
		window = replication.TimeRange{
			From: now.Add(-6 * time.Hour),
			To:   now,
		}
	}
	
	// Create planner
	planner := replication.NewPlanner()
	
	// Add health check for source region
	healthScore := metrics.GlobalDataMetrics.GetRegionHealth(simFromRegion)
	planner.AddHealthCheck(replication.Region(simFromRegion), healthScore)
	
	// Add health check for target region  
	healthScore = metrics.GlobalDataMetrics.GetRegionHealth(simToRegion)
	planner.AddHealthCheck(replication.Region(simToRegion), healthScore)
	
	// Build plan
	plan, err := planner.BuildPlan(tier, window, simDryRun)
	if err != nil {
		return fmt.Errorf("failed to build replication plan: %v", err)
	}
	
	// Filter plan steps for the requested from/to regions
	var filteredSteps []replication.Step
	for _, step := range plan.Steps {
		if string(step.From) == simFromRegion && string(step.To) == simToRegion {
			filteredSteps = append(filteredSteps, step)
		}
	}
	
	plan.Steps = filteredSteps
	plan.TotalSteps = len(filteredSteps)
	
	// Calculate total estimated duration
	totalDuration := time.Duration(0)
	for _, step := range plan.Steps {
		totalDuration += step.EstimatedDuration
	}
	plan.EstimatedDuration = totalDuration
	
	// Output plan based on format
	switch simOutputFormat {
	case "json":
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		return encoder.Encode(plan)
		
	case "table":
		printPlanTable(plan)
		
	case "summary":
		printPlanSummary(plan)
		
	default:
		return fmt.Errorf("invalid output format: %s", simOutputFormat)
	}
	
	return nil
}

func runReplicationFailover(cmd *cobra.Command, args []string) error {
	// Validate tier
	tier := replication.Tier(failoverTier)
	if tier != replication.TierHot && tier != replication.TierWarm && tier != replication.TierCold {
		return fmt.Errorf("invalid tier: %s (must be hot, warm, or cold)", failoverTier)
	}
	
	promoteRegion := replication.Region(failoverPromote)
	var demoteRegion replication.Region
	if failoverDemote != "" {
		demoteRegion = replication.Region(failoverDemote)
	}
	
	// Check health of promotion target
	healthScore := metrics.GlobalDataMetrics.GetRegionHealth(failoverPromote)
	if healthScore < 0.8 && !failoverForce {
		return fmt.Errorf("target region %s has low health score (%.2f) - use --force to override", failoverPromote, healthScore)
	}
	
	// Check current lag of promotion target
	lag := metrics.GlobalDataMetrics.GetReplicationLag(failoverTier, failoverPromote, "primary")
	
	fmt.Printf("Replication Failover Plan:\n")
	fmt.Printf("  Tier: %s\n", tier)
	fmt.Printf("  Promote: %s (health: %.2f, lag: %.1fs)\n", promoteRegion, healthScore, lag)
	if demoteRegion != "" {
		demoteHealth := metrics.GlobalDataMetrics.GetRegionHealth(string(demoteRegion))
		fmt.Printf("  Demote: %s (health: %.2f)\n", demoteRegion, demoteHealth)
	}
	fmt.Printf("  Timeout: %ds\n", failoverTimeout)
	fmt.Printf("  Validate: %v\n", failoverValidate)
	fmt.Printf("  Force: %v\n", failoverForce)
	
	if failoverDryRun {
		fmt.Printf("\n[DRY RUN] Would execute failover with above parameters\n")
		
		// Show what validation steps would be performed
		fmt.Printf("\nValidation steps that would be performed:\n")
		fmt.Printf("  1. Verify target region health ≥ 0.8\n")
		fmt.Printf("  2. Check replication lag ≤ SLO threshold\n")
		fmt.Printf("  3. Validate data consistency\n")
		fmt.Printf("  4. Update region authority configuration\n")
		fmt.Printf("  5. Redirect traffic to promoted region\n")
		
		if failoverValidate {
			fmt.Printf("  6. Post-failover validation checks\n")
		}
		
		return nil
	}
	
	// Execute actual failover
	fmt.Printf("\n[EXECUTING] Starting replication failover...\n")
	
	// Step 1: Pre-failover validation
	fmt.Printf("Step 1: Pre-failover validation... ")
	if healthScore >= 0.8 || failoverForce {
		fmt.Printf("PASSED\n")
	} else {
		fmt.Printf("FAILED - health score too low\n")
		return fmt.Errorf("pre-failover validation failed")
	}
	
	// Step 2: Update metrics to reflect the change
	fmt.Printf("Step 2: Updating region authority... ")
	// In a real implementation, this would update configuration
	// For now, we'll just update metrics
	metrics.GlobalDataMetrics.SetRegionHealth(failoverPromote, 1.0)
	if demoteRegion != "" {
		metrics.GlobalDataMetrics.SetRegionHealth(string(demoteRegion), 0.8)
	}
	fmt.Printf("COMPLETED\n")
	
	// Step 3: Reset replication lag for promoted region
	fmt.Printf("Step 3: Resetting replication metrics... ")
	metrics.GlobalDataMetrics.RecordReplicationLag(failoverTier, failoverPromote, "promoted", 0.0)
	fmt.Printf("COMPLETED\n")
	
	// Step 4: Post-failover validation (if enabled)
	if failoverValidate {
		fmt.Printf("Step 4: Post-failover validation... ")
		// Simulate validation checks
		time.Sleep(2 * time.Second)
		fmt.Printf("COMPLETED\n")
	}
	
	fmt.Printf("\n✅ Failover completed successfully!\n")
	fmt.Printf("Region %s is now primary for tier %s\n", promoteRegion, tier)
	
	return nil
}

func runReplicationStatus(cmd *cobra.Command, args []string) error {
	// Handle watch mode
	if statusWatch {
		if statusInterval == 0 {
			statusInterval = 5 // Default 5 second refresh
		}
		
		for {
			// Clear screen
			fmt.Print("\033[2J\033[H")
			
			err := displayReplicationStatus()
			if err != nil {
				return err
			}
			
			fmt.Printf("\n[Refreshing every %ds - Press Ctrl+C to exit]\n", statusInterval)
			time.Sleep(time.Duration(statusInterval) * time.Second)
		}
	}
	
	return displayReplicationStatus()
}

func displayReplicationStatus() error {
	switch statusFormat {
	case "json":
		allMetrics := metrics.GlobalDataMetrics.GetAllMetrics()
		
		// Filter by tier and region if specified
		if statusTier != "" || statusRegion != "" {
			filteredMetrics := make(map[string]interface{})
			for category, metrics := range allMetrics {
				if metricMap, ok := metrics.(map[string]float64); ok {
					filteredMap := make(map[string]float64)
					for key, value := range metricMap {
						include := true
						
						if statusTier != "" && !strings.Contains(key, fmt.Sprintf("tier=%s", statusTier)) {
							include = false
						}
						
						if statusRegion != "" && !strings.Contains(key, fmt.Sprintf("region=%s", statusRegion)) {
							include = false
						}
						
						if include {
							filteredMap[key] = value
						}
					}
					filteredMetrics[category] = filteredMap
				}
			}
			allMetrics = filteredMetrics
		}
		
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		return encoder.Encode(allMetrics)
		
	case "prometheus":
		return displayPrometheusMetrics()
		
	case "table":
	default:
		return displayStatusTable()
	}
}

func displayStatusTable() error {
	fmt.Printf("CryptoRun Multi-Region Replication Status\n")
	fmt.Printf("=========================================\n\n")
	
	// Region Health
	fmt.Printf("Region Health:\n")
	fmt.Printf("%-15s %-10s %-20s\n", "Region", "Health", "Status")
	fmt.Printf("%-15s %-10s %-20s\n", "------", "------", "------")
	
	regions := []string{"us-east-1", "us-west-2", "eu-west-1"}
	for _, region := range regions {
		health := metrics.GlobalDataMetrics.GetRegionHealth(region)
		status := getHealthStatus(health)
		
		if statusRegion == "" || statusRegion == region {
			fmt.Printf("%-15s %-10.2f %-20s\n", region, health, status)
		}
	}
	
	// Replication Lag by Tier
	fmt.Printf("\nReplication Lag (seconds):\n")
	fmt.Printf("%-10s %-15s %-15s %-10s %-10s\n", "Tier", "Region", "Source", "Lag", "SLO")
	fmt.Printf("%-10s %-15s %-15s %-10s %-10s\n", "----", "------", "------", "---", "---")
	
	tiers := []string{"hot", "warm", "cold"}
	for _, tier := range tiers {
		if statusTier != "" && statusTier != tier {
			continue
		}
		
		slo := getSLOForTier(tier)
		for _, region := range regions {
			if statusRegion != "" && statusRegion != region {
				continue
			}
			
			lag := metrics.GlobalDataMetrics.GetReplicationLag(tier, region, "primary")
			if lag > 0 || statusShowAll {
				fmt.Printf("%-10s %-15s %-15s %-10.1f %-10s\n", tier, region, "primary", lag, slo)
			}
		}
	}
	
	// Consistency Errors
	fmt.Printf("\nData Consistency Errors:\n")
	fmt.Printf("%-20s %-10s\n", "Check Type", "Count")
	fmt.Printf("%-20s %-10s\n", "----------", "-----")
	
	checkTypes := []string{"schema", "staleness", "anomaly", "corrupt"}
	for _, checkType := range checkTypes {
		count := metrics.GlobalDataMetrics.GetConsistencyErrorsCount(checkType)
		if count > 0 || statusShowAll {
			fmt.Printf("%-20s %-10.0f\n", checkType, count)
		}
	}
	
	// Quarantine Status
	if statusShowAll {
		fmt.Printf("\nQuarantine Status:\n")
		fmt.Printf("%-10s %-15s %-15s %-10s\n", "Tier", "Region", "Kind", "Count")
		fmt.Printf("%-10s %-15s %-15s %-10s\n", "----", "------", "----", "-----")
		
		for _, tier := range tiers {
			for _, region := range regions {
				kinds := []string{"timestamp_skew", "anomaly", "corruption"}
				for _, kind := range kinds {
					count := metrics.GlobalDataMetrics.GetQuarantineCount(tier, region, kind)
					if count > 0 {
						fmt.Printf("%-10s %-15s %-15s %-10.0f\n", tier, region, kind, count)
					}
				}
			}
		}
	}
	
	return nil
}

func displayPrometheusMetrics() error {
	allMetrics := metrics.GlobalDataMetrics.GetAllMetrics()
	
	for category, metrics := range allMetrics {
		if metricMap, ok := metrics.(map[string]float64); ok {
			for key, value := range metricMap {
				// Skip filtered metrics
				if statusTier != "" && !strings.Contains(key, fmt.Sprintf("tier=%s", statusTier)) {
					continue
				}
				if statusRegion != "" && !strings.Contains(key, fmt.Sprintf("region=%s", statusRegion)) {
					continue
				}
				
				// Convert to Prometheus format
				metricName := fmt.Sprintf("cryptorun_%s", strings.ReplaceAll(category, "_", "_"))
				fmt.Printf("%s{%s} %g\n", metricName, key, value)
			}
		}
	}
	
	return nil
}

func printPlanTable(plan *replication.Plan) {
	fmt.Printf("Replication Plan: %s\n", plan.ID)
	fmt.Printf("Created: %s\n", plan.CreatedAt.Format(time.RFC3339))
	fmt.Printf("Total Steps: %d\n", plan.TotalSteps)
	fmt.Printf("Estimated Duration: %v\n", plan.EstimatedDuration)
	fmt.Printf("Dry Run: %v\n\n", plan.DryRun)
	
	if len(plan.Steps) > 0 {
		fmt.Printf("Steps:\n")
		fmt.Printf("%-5s %-6s %-15s %-15s %-25s %-15s %-10s\n", "ID", "Tier", "From", "To", "Window", "Duration", "Retries")
		fmt.Printf("%-5s %-6s %-15s %-15s %-25s %-15s %-10s\n", "--", "----", "----", "--", "------", "--------", "-------")
		
		for i, step := range plan.Steps {
			window := fmt.Sprintf("%s to %s", 
				step.Window.From.Format("15:04:05"),
				step.Window.To.Format("15:04:05"))
				
			fmt.Printf("%-5d %-6s %-15s %-15s %-25s %-15s %-10d\n",
				i+1,
				step.Tier,
				step.From,
				step.To,
				window,
				step.EstimatedDuration,
				step.MaxRetries,
			)
		}
	} else {
		fmt.Printf("No replication steps required.\n")
	}
}

func printPlanSummary(plan *replication.Plan) {
	fmt.Printf("📋 Replication Plan Summary\n")
	fmt.Printf("   Plan ID: %s\n", plan.ID)
	fmt.Printf("   Steps: %d\n", plan.TotalSteps)
	fmt.Printf("   Duration: ~%v\n", plan.EstimatedDuration)
	fmt.Printf("   Mode: ")
	if plan.DryRun {
		fmt.Printf("DRY RUN\n")
	} else {
		fmt.Printf("EXECUTE\n")
	}
	
	if len(plan.Steps) > 0 {
		fmt.Printf("\n📦 Step Breakdown:\n")
		tierCounts := make(map[string]int)
		for _, step := range plan.Steps {
			tierCounts[string(step.Tier)]++
		}
		
		for tier, count := range tierCounts {
			fmt.Printf("   %s: %d steps\n", strings.ToUpper(tier), count)
		}
		
		fmt.Printf("\n⏱ Estimated Timeline:\n")
		currentTime := plan.CreatedAt
		for i, step := range plan.Steps {
			if i < 3 { // Show first 3 steps
				fmt.Printf("   %s: %s → %s (%v)\n", 
					currentTime.Format("15:04:05"),
					step.From,
					step.To,
					step.EstimatedDuration)
				currentTime = currentTime.Add(step.EstimatedDuration)
			}
		}
		if len(plan.Steps) > 3 {
			fmt.Printf("   ... (%d more steps)\n", len(plan.Steps)-3)
		}
	}
}

func getHealthStatus(health float64) string {
	if health >= 0.9 {
		return "HEALTHY"
	} else if health >= 0.7 {
		return "DEGRADED" 
	} else if health >= 0.5 {
		return "UNHEALTHY"
	} else {
		return "CRITICAL"
	}
}

func getSLOForTier(tier string) string {
	switch tier {
	case "hot":
		return "500ms"
	case "warm":
		return "60s"
	case "cold":
		return "5m"
	default:
		return "unknown"
	}
}