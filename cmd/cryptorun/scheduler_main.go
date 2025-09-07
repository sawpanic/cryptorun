package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/rs/zerolog/log"

	"cryptorun/internal/application"
	"cryptorun/internal/scheduler"
)

// runScheduleList lists all configured scheduled jobs
func runScheduleList(cmd *cobra.Command, args []string) error {
	sched, err := scheduler.NewScheduler("config/scheduler.yaml")
	if err != nil {
		return fmt.Errorf("failed to initialize scheduler: %w", err)
	}

	jobs, err := sched.ListJobs()
	if err != nil {
		return fmt.Errorf("failed to list jobs: %w", err)
	}

	fmt.Printf("📋 Scheduled Jobs (%d)\n", len(jobs))
	fmt.Printf("%-20s %-15s %-8s %-s\n", "JOB NAME", "SCHEDULE", "STATUS", "DESCRIPTION")
	fmt.Printf("%-20s %-15s %-8s %-s\n", "--------", "--------", "------", "-----------")

	for _, job := range jobs {
		status := "enabled"
		if !job.Enabled {
			status = "disabled"
		}
		fmt.Printf("%-20s %-15s %-8s %-s\n", job.Name, job.Schedule, status, job.Description)
	}

	return nil
}

// runScheduleStart starts the scheduler daemon
func runScheduleStart(cmd *cobra.Command, args []string) error {
	log.Info().Msg("Starting CryptoRun scheduler daemon")

	sched, err := scheduler.NewScheduler("config/scheduler.yaml")
	if err != nil {
		return fmt.Errorf("failed to initialize scheduler: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start scheduler in background
	go func() {
		if err := sched.Start(ctx); err != nil {
			log.Error().Err(err).Msg("scheduler failed")
		}
	}()

	log.Info().Msg("Scheduler daemon running. Press Ctrl+C to stop.")

	// Wait for interrupt signal
	select {
	case <-ctx.Done():
		log.Info().Msg("Scheduler daemon stopped")
	}

	return nil
}

// runScheduleStatus shows current scheduler status
func runScheduleStatus(cmd *cobra.Command, args []string) error {
	sched, err := scheduler.NewScheduler("config/scheduler.yaml")
	if err != nil {
		return fmt.Errorf("failed to initialize scheduler: %w", err)
	}

	status, err := sched.GetStatus()
	if err != nil {
		return fmt.Errorf("failed to get status: %w", err)
	}

	fmt.Printf("🕐 Scheduler Status\n")
	fmt.Printf("Running: %v\n", status.Running)
	fmt.Printf("Jobs Enabled: %d\n", status.EnabledJobs)
	fmt.Printf("Jobs Disabled: %d\n", status.DisabledJobs)
	fmt.Printf("Next Run: %s\n", status.NextRun.Format(time.RFC3339))
	fmt.Printf("Last Run: %s\n", status.LastRun.Format(time.RFC3339))
	fmt.Printf("Uptime: %s\n", status.Uptime)

	return nil
}

// runScheduleRun executes a specific scheduled job immediately
func runScheduleRun(cmd *cobra.Command, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("job name required")
	}

	jobName := args[0]
	dryRun, _ := cmd.Flags().GetBool("dry-run")

	log.Info().Str("job", jobName).Bool("dry_run", dryRun).Msg("Running scheduled job")

	sched, err := scheduler.NewScheduler("config/scheduler.yaml")
	if err != nil {
		return fmt.Errorf("failed to initialize scheduler: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	result, err := sched.RunJob(ctx, jobName, dryRun)
	if err != nil {
		return fmt.Errorf("job execution failed: %w", err)
	}

	fmt.Printf("✅ Job '%s' completed successfully\n", jobName)
	fmt.Printf("Duration: %s\n", result.Duration)
	fmt.Printf("Artifacts: %d files\n", len(result.Artifacts))
	
	if len(result.Artifacts) > 0 {
		fmt.Printf("Generated artifacts:\n")
		for _, artifact := range result.Artifacts {
			fmt.Printf("  %s\n", artifact)
		}
	}

	return nil
}