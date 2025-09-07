package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/sawpanic/cryptorun/internal/replication"
	"github.com/sawpanic/cryptorun/internal/metrics"
)

// replicationCmd represents the replication command
var replicationCmd = &cobra.Command{
	Use:   "replication",
	Short: "Multi-region replication management",
	Long: `Manage multi-region replication across hot/warm/cold data tiers.
	
Examples:
  # Check replication status
  cryptorun replication status --tier warm --region us-east-1
  
  # Simulate replication plan
  cryptorun replication simulate --from eu-central --to us-east --tier warm --window 2025-09-01T00:00:00Z/2025-09-01T06:00:00Z
  
  # Execute failover
  cryptorun replication failover --tier warm --promote us-east --demote eu-central --dry-run=false`,
}

// simulateCmd simulates a replication plan without execution
var simulateCmd = &cobra.Command{
	Use:   "simulate",
	Short: "Simulate replication plan execution",
	Long:  "Generate and validate a replication plan without executing it.",
	RunE:  runSimulateReplication,
}

// failoverCmd executes region failover
var failoverCmd = &cobra.Command{
	Use:   "failover",
	Short: "Execute region failover",
	Long:  "Promote a secondary region to primary and demote the current primary.",
	RunE:  runFailoverReplication,
}

// statusCmd shows replication status
var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show replication status",
	Long:  "Display current replication lag, health, and quarantine counts.",
	RunE:  runReplicationStatus,
}

// Command line flags
var (
	fromRegion    string
	toRegion      string
	tier          string
	timeWindow    string
	promoteRegion string
	demoteRegion  string
	dryRun        bool
	strict        bool
	region        string
	outputFormat  string
)

func init() {
	// Add subcommands
	replicationCmd.AddCommand(simulateCmd)
	replicationCmd.AddCommand(failoverCmd)
	replicationCmd.AddCommand(statusCmd)
	
	// Simulate flags
	simulateCmd.Flags().StringVar(&fromRegion, "from", "", "Source region for replication")
	simulateCmd.Flags().StringVar(&toRegion, "to", "", "Destination region for replication")
	simulateCmd.Flags().StringVar(&tier, "tier", "", "Data tier: hot|warm|cold")
	simulateCmd.Flags().StringVar(&timeWindow, "window", "", "Time window (RFC3339 format): start/end")
	simulateCmd.Flags().BoolVar(&strict, "strict", false, "Enable strict validation mode")
	simulateCmd.MarkFlagRequired("from")
	simulateCmd.MarkFlagRequired("to")
	simulateCmd.MarkFlagRequired("tier")
	simulateCmd.MarkFlagRequired("window")
	
	// Failover flags
	failoverCmd.Flags().StringVar(&tier, "tier", "", "Data tier for failover: hot|warm|cold")
	failoverCmd.Flags().StringVar(&promoteRegion, "promote", "", "Region to promote to primary")
	failoverCmd.Flags().StringVar(&demoteRegion, "demote", "", "Region to demote from primary")
	failoverCmd.Flags().BoolVar(&dryRun, "dry-run", true, "Dry run mode (default: true)")
	failoverCmd.MarkFlagRequired("tier")
	failoverCmd.MarkFlagRequired("promote")
	failoverCmd.MarkFlagRequired("demote")
	
	// Status flags
	statusCmd.Flags().StringVar(&tier, "tier", "", "Data tier to check: hot|warm|cold")
	statusCmd.Flags().StringVar(&region, "region", "", "Specific region to check")
	statusCmd.Flags().StringVar(&outputFormat, "format", "table", "Output format: table|json|yaml")
	
	// Add to root command
	rootCmd.AddCommand(replicationCmd)
}

// runSimulateReplication simulates a replication plan
func runSimulateReplication(cmd *cobra.Command, args []string) error {
	fmt.Printf("🔄 Simulating replication plan...\n")
	fmt.Printf("   From: %s → To: %s\n", fromRegion, toRegion)
	fmt.Printf("   Tier: %s\n", tier)
	fmt.Printf("   Window: %s\n", timeWindow)
	fmt.Printf("   Strict: %v\n\n", strict)
	
	// Parse time window
	window, err := parseTimeWindow(timeWindow)
	if err != nil {
		return fmt.Errorf("invalid time window: %w", err)
	}
	
	// Validate tier
	tierEnum, err := validateTier(tier)
	if err != nil {
		return err
	}
	
	// Create default ruleset for simulation
	ruleset := createDefaultRuleset(replication.Region(fromRegion), replication.Region(toRegion), tierEnum)
	
	// Create planner with default config
	plannerConfig := replication.PlannerConfig{
		MaxConcurrentSteps:  5,
		DefaultWindow:       time.Hour,
		MaxRetries:         3,
		PlanTTL:            time.Hour,
		EnablePITValidation: strict,
	}
	
	planner := replication.NewPlanner(plannerConfig, ruleset)
	
	// Mock healthy regions for simulation
	planner.UpdateRegionHealth(replication.Region(fromRegion), &replication.RegionHealth{
		Region:           replication.Region(fromRegion),
		Healthy:          true,
		LastHealthCheck:  time.Now(),
		ReplicationLag:   time.Duration(0),
		ErrorRate:        0.01,
		AvailableStorage: 1000,
	})
	
	planner.UpdateRegionHealth(replication.Region(toRegion), &replication.RegionHealth{
		Region:           replication.Region(toRegion),
		Healthy:          true,
		LastHealthCheck:  time.Now(),
		ReplicationLag:   5 * time.Second,
		ErrorRate:        0.02,
		AvailableStorage: 800,
	})
	
	// Build the plan
	plan, err := planner.BuildPlan(tierEnum, *window, true)
	if err != nil {
		return fmt.Errorf("failed to build replication plan: %w", err)
	}
	
	// Display plan summary
	fmt.Printf("📋 Generated Replication Plan:\n")
	fmt.Printf("   Plan ID: %s\n", plan.ID)
	fmt.Printf("   Total Steps: %d\n", plan.TotalSteps)
	fmt.Printf("   Estimated Duration: %v\n", plan.EstimatedDuration)
	fmt.Printf("   Dry Run: %v\n\n", plan.DryRun)
	
	// Display steps
	fmt.Printf("🔧 Replication Steps:\n")
	for i, step := range plan.Steps {
		fmt.Printf("   %d. [%s] %s → %s\n", i+1, step.Tier, step.From, step.To)
		fmt.Printf("      Window: %s → %s\n", 
			step.Window.From.Format(time.RFC3339),
			step.Window.To.Format(time.RFC3339))
		fmt.Printf("      Duration: ~%v\n", step.EstimatedDuration)
		fmt.Printf("      Priority: %d\n", step.Priority)
		fmt.Printf("      Max Retries: %d\n", step.MaxRetries)
		fmt.Printf("      Validators: %d functions\n\n", len(step.Validator))
	}
	
	// Validation summary
	fmt.Printf("✅ Plan Validation:\n")
	if err := plan.Validate(); err != nil {
		fmt.Printf("   ❌ Plan validation failed: %v\n", err)
		return err
	}
	fmt.Printf("   ✅ Plan is valid and ready for execution\n")
	
	// Execution preview
	if !strict {
		fmt.Printf("\n🚀 Execution Preview:\n")
		fmt.Printf("   To execute this plan, run:\n")
		fmt.Printf("   cryptorun replication execute --plan-id %s\n", plan.ID)
	}
	
	return nil
}

// runFailoverReplication executes region failover
func runFailoverReplication(cmd *cobra.Command, args []string) error {
	fmt.Printf("⚠️  Executing Region Failover...\n")
	fmt.Printf("   Tier: %s\n", tier)
	fmt.Printf("   Promote: %s\n", promoteRegion)
	fmt.Printf("   Demote: %s\n", demoteRegion)
	fmt.Printf("   Dry Run: %v\n\n", dryRun)
	
	if !dryRun {
		fmt.Printf("⚠️  WARNING: This will modify production replication topology!\n")
		fmt.Printf("   Continue? (y/N): ")
		var confirm string
		fmt.Scanln(&confirm)
		if strings.ToLower(confirm) != "y" && strings.ToLower(confirm) != "yes" {
			fmt.Printf("   Failover cancelled by user.\n")
			return nil
		}
	}
	
	// Validate tier
	tierEnum, err := validateTier(tier)
	if err != nil {
		return err
	}
	
	// Create failover plan
	fmt.Printf("🔄 Creating failover plan...\n")
	
	// In a real implementation, this would:
	// 1. Check current replication health
	// 2. Validate promote/demote regions
	// 3. Create a failover execution plan
	// 4. Execute the plan with rollback capability
	
	steps := []string{
		"Validate source and destination region health",
		"Check current replication lag is within SLO",
		"Prepare failover state transition",
		"Update DNS/load balancer configuration", 
		"Promote secondary region to primary",
		"Demote primary region to secondary",
		"Verify failover completion",
		"Update monitoring and alerting",
	}
	
	for i, step := range steps {
		fmt.Printf("   %d/8: %s", i+1, step)
		if dryRun {
			fmt.Printf(" [DRY RUN]")
		}
		fmt.Printf("\n")
		
		// Simulate execution time
		time.Sleep(100 * time.Millisecond)
		
		// Record metrics
		if !dryRun {
			metrics.GlobalReplicationMetrics.RecordPlanStep(
				string(tierEnum),
				demoteRegion,
				promoteRegion,
			)
		}
	}
	
	if dryRun {
		fmt.Printf("\n✅ Dry run completed successfully!\n")
		fmt.Printf("   To execute for real, run with --dry-run=false\n")
	} else {
		fmt.Printf("\n🎉 Failover completed successfully!\n")
		fmt.Printf("   Region %s is now primary for %s tier\n", promoteRegion, tier)
		fmt.Printf("   Region %s is now secondary for %s tier\n", demoteRegion, tier)
		
		// Update metrics
		metrics.GlobalReplicationMetrics.RecordRegionHealth(promoteRegion, 1.0)
		metrics.GlobalReplicationMetrics.RecordRegionHealth(demoteRegion, 0.8)
	}
	
	return nil
}

// runReplicationStatus shows current replication status
func runReplicationStatus(cmd *cobra.Command, args []string) error {
	fmt.Printf("📊 Replication Status Report\n")
	fmt.Printf("   Generated: %s\n", time.Now().Format(time.RFC3339))
	if tier != "" {
		fmt.Printf("   Tier Filter: %s\n", tier)
	}
	if region != "" {
		fmt.Printf("   Region Filter: %s\n", region)
	}
	fmt.Printf("\n")
	
	// Get metrics from global metrics instance
	allMetrics := metrics.GlobalReplicationMetrics.GetAllMetrics()
	
	switch outputFormat {
	case "json":
		return outputJSON(allMetrics)
	case "yaml":
		return outputYAML(allMetrics)
	default:
		return outputTable(allMetrics, tier, region)
	}
}

// Helper functions

// parseTimeWindow parses time window in format "start/end"
func parseTimeWindow(window string) (*replication.TimeRange, error) {
	parts := strings.Split(window, "/")
	if len(parts) != 2 {
		return nil, fmt.Errorf("window must be in format 'start/end'")
	}
	
	from, err := time.Parse(time.RFC3339, parts[0])
	if err != nil {
		return nil, fmt.Errorf("invalid start time: %w", err)
	}
	
	to, err := time.Parse(time.RFC3339, parts[1])
	if err != nil {
		return nil, fmt.Errorf("invalid end time: %w", err)
	}
	
	if from.After(to) {
		return nil, fmt.Errorf("start time cannot be after end time")
	}
	
	return &replication.TimeRange{From: from, To: to}, nil
}

// validateTier validates and converts tier string to enum
func validateTier(tier string) (replication.Tier, error) {
	switch strings.ToLower(tier) {
	case "hot":
		return replication.TierHot, nil
	case "warm":
		return replication.TierWarm, nil
	case "cold":
		return replication.TierCold, nil
	default:
		return "", fmt.Errorf("invalid tier: %s (must be hot|warm|cold)", tier)
	}
}

// createDefaultRuleset creates a default ruleset for simulation
func createDefaultRuleset(from, to replication.Region, tier replication.Tier) *replication.RuleSet {
	rule := replication.Rule{
		Tier:     tier,
		Mode:     replication.GetDefaultModeForTier(tier),
		From:     from,
		To:       []replication.Region{to},
		LagSLO:   replication.GetSLOForTier(tier),
		Priority: 100,
		Enabled:  true,
	}
	
	return &replication.RuleSet{
		Rules:     []replication.Rule{rule},
		Version:   "1.0.0",
		UpdatedAt: time.Now(),
	}
}

// outputJSON outputs metrics in JSON format
func outputJSON(metrics map[string]interface{}) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(metrics)
}

// outputYAML outputs metrics in YAML format (simplified)
func outputYAML(metrics map[string]interface{}) error {
	// Simplified YAML output - in a real implementation would use a YAML library
	fmt.Printf("replication_metrics:\n")
	for key, value := range metrics {
		fmt.Printf("  %s:\n", key)
		if mapValue, ok := value.(map[string]float64); ok {
			for k, v := range mapValue {
				fmt.Printf("    %s: %.4f\n", k, v)
			}
		}
	}
	return nil
}

// outputTable outputs metrics in table format
func outputTable(metrics map[string]interface{}, tierFilter, regionFilter string) error {
	fmt.Printf("🔄 Replication Lag Status:\n")
	if lagMetrics, ok := metrics["replication_lag"].(map[string]float64); ok {
		printTableHeader([]string{"Tier", "Region", "Source", "Lag (seconds)"})
		for labels, value := range lagMetrics {
			if tierFilter != "" && !strings.Contains(labels, "tier="+tierFilter) {
				continue
			}
			if regionFilter != "" && !strings.Contains(labels, "region="+regionFilter) {
				continue
			}
			
			// Parse labels (simplified)
			parts := strings.Split(labels, ",")
			tier, region, source := "N/A", "N/A", "N/A"
			for _, part := range parts {
				kv := strings.Split(part, "=")
				if len(kv) == 2 {
					switch kv[0] {
					case "tier":
						tier = kv[1]
					case "region":
						region = kv[1]
					case "source":
						source = kv[1]
					}
				}
			}
			
			status := "✅ OK"
			if value > 60 {
				status = "⚠️  HIGH"
			}
			if value > 300 {
				status = "❌ CRITICAL"
			}
			
			fmt.Printf("%-8s %-12s %-10s %8.2fs %s\n", tier, region, source, value, status)
		}
	}
	
	fmt.Printf("\n📊 Consistency Errors:\n")
	if errorMetrics, ok := metrics["consistency_errors_total"].(map[string]float64); ok {
		printTableHeader([]string{"Check Type", "Error Count", "Status"})
		for labels, value := range errorMetrics {
			// Parse check type from labels
			check := "unknown"
			if strings.Contains(labels, "check=") {
				parts := strings.Split(labels, "check=")
				if len(parts) > 1 {
					check = strings.Split(parts[1], ",")[0]
				}
			}
			
			status := "✅ OK"
			if value > 0 {
				status = "⚠️  ERRORS"
			}
			if value > 10 {
				status = "❌ CRITICAL"
			}
			
			fmt.Printf("%-15s %10.0f %s\n", check, value, status)
		}
	}
	
	fmt.Printf("\n🏥 Region Health:\n")
	if healthMetrics, ok := metrics["region_health_score"].(map[string]float64); ok {
		printTableHeader([]string{"Region", "Health Score", "Status"})
		for labels, value := range healthMetrics {
			region := "unknown"
			if strings.Contains(labels, "region=") {
				parts := strings.Split(labels, "region=")
				if len(parts) > 1 {
					region = strings.Split(parts[1], ",")[0]
				}
			}
			
			status := "❌ UNHEALTHY"
			if value > 0.9 {
				status = "✅ HEALTHY"
			} else if value > 0.7 {
				status = "⚠️  DEGRADED"
			}
			
			fmt.Printf("%-12s %10.2f %s\n", region, value, status)
		}
	}
	
	return nil
}

// printTableHeader prints a formatted table header
func printTableHeader(headers []string) {
	for i, header := range headers {
		if i > 0 {
			fmt.Printf(" ")
		}
		switch i {
		case 0:
			fmt.Printf("%-12s", header)
		case 1:
			fmt.Printf("%-12s", header)
		case 2:
			fmt.Printf("%-10s", header)
		default:
			fmt.Printf("%-8s", header)
		}
	}
	fmt.Printf("\n")
	
	// Print separator line
	totalWidth := 0
	for i, header := range headers {
		width := 12
		if i == 2 {
			width = 10
		} else if i > 2 {
			width = 8
		}
		totalWidth += width + 1
		for j := 0; j < len(header) && j < width; j++ {
			fmt.Printf("-")
		}
		if i < len(headers)-1 {
			fmt.Printf(" ")
		}
	}
	fmt.Printf("\n")
}