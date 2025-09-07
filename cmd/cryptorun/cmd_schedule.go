//go:build ignore
// +build ignore

package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/sawpanic/cryptorun/internal/application/scheduler"
)

func init() {
	rootCmd.AddCommand(scheduleCmd)
	scheduleCmd.AddCommand(scheduleRunCmd)
	
	scheduleRunCmd.Flags().String("job", "", "Job to run: scan.hot, scan.warm, regime.refresh, premove.hourly, report.eod, report.weekly")
	scheduleRunCmd.Flags().Bool("once", false, "Run job once then exit")
	scheduleRunCmd.Flags().Bool("loop", false, "Run job in continuous loop")
	scheduleRunCmd.Flags().Bool("dry-run", false, "Validate configuration without executing")
	scheduleRunCmd.MarkFlagRequired("job")
}

var scheduleCmd = &cobra.Command{
	Use:   "schedule",
	Short: "Schedule and run automated jobs",
	Long:  `Run scheduled jobs for signals scanning, regime detection, pre-movement alerts, and reports`,
}

var scheduleRunCmd = &cobra.Command{
	Use:   "run",
	Short: "Execute a scheduled job",
	Long:  `Execute scheduled jobs with configurable cadence and output`,
	RunE:  runScheduleJob,
}

func runScheduleJob(cmd *cobra.Command, args []string) error {
	jobName, _ := cmd.Flags().GetString("job")
	once, _ := cmd.Flags().GetBool("once")
	loop, _ := cmd.Flags().GetBool("loop")
	dryRun, _ := cmd.Flags().GetBool("dry-run")
	
	if !once && !loop {
		return fmt.Errorf("must specify either --once or --loop")
	}
	
	if once && loop {
		return fmt.Errorf("cannot specify both --once and --loop")
	}
	
	scheduler := scheduler.New()
	
	if dryRun {
		fmt.Printf("🔍 DRY RUN: Job '%s' validated successfully\n", jobName)
		return nil
	}
	
	switch {
	case strings.HasPrefix(jobName, "scan."):
		return runSignalsLoop(scheduler, jobName, once, loop)
	case jobName == "regime.refresh":
		return runRegimeRefresh(scheduler, once, loop)
	case jobName == "premove.hourly":
		return runPremoveLoop(scheduler, once, loop)
	case strings.HasPrefix(jobName, "report."):
		return runReportJob(scheduler, jobName, once)
	default:
		return fmt.Errorf("unknown job: %s", jobName)
	}
}

func runSignalsLoop(sched *scheduler.Scheduler, jobName string, once, loop bool) error {
	var cadence time.Duration
	
	switch jobName {
	case "scan.hot":
		cadence = 15 * time.Minute
	case "scan.warm":
		cadence = 2 * time.Hour
	default:
		return fmt.Errorf("unknown scan job: %s", jobName)
	}
	
	fmt.Printf("📊 Starting %s signals loop (cadence: %v)\n", jobName, cadence)
	
	if once {
		return sched.RunSignalsOnce(jobName)
	}
	
	ticker := time.NewTicker(cadence)
	defer ticker.Stop()
	
	// Run immediately
	if err := sched.RunSignalsOnce(jobName); err != nil {
		fmt.Printf("❌ Initial run failed: %v\n", err)
	}
	
	for {
		select {
		case <-ticker.C:
			if err := sched.RunSignalsOnce(jobName); err != nil {
				fmt.Printf("❌ Loop iteration failed: %v\n", err)
			}
		}
	}
}

func runRegimeRefresh(sched *scheduler.Scheduler, once, loop bool) error {
	fmt.Println("🌀 Starting regime refresh job (cadence: 4h)")
	
	if once {
		return sched.RunRegimeRefresh()
	}
	
	ticker := time.NewTicker(4 * time.Hour)
	defer ticker.Stop()
	
	// Run immediately
	if err := sched.RunRegimeRefresh(); err != nil {
		fmt.Printf("❌ Initial regime refresh failed: %v\n", err)
	}
	
	for {
		select {
		case <-ticker.C:
			if err := sched.RunRegimeRefresh(); err != nil {
				fmt.Printf("❌ Regime refresh failed: %v\n", err)
			}
		}
	}
}

func runPremoveLoop(sched *scheduler.Scheduler, once, loop bool) error {
	fmt.Println("🎯 Starting pre-movement detector (cadence: 1h)")
	
	if once {
		return sched.RunPremoveOnce()
	}
	
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()
	
	// Run immediately
	if err := sched.RunPremoveOnce(); err != nil {
		fmt.Printf("❌ Initial premove run failed: %v\n", err)
	}
	
	for {
		select {
		case <-ticker.C:
			if err := sched.RunPremoveOnce(); err != nil {
				fmt.Printf("❌ Premove iteration failed: %v\n", err)
			}
		}
	}
}

func runReportJob(sched *scheduler.Scheduler, jobName string, once bool) error {
	if !once {
		return fmt.Errorf("report jobs only support --once mode")
	}
	
	fmt.Printf("📈 Running %s report\n", jobName)
	
	switch jobName {
	case "report.eod":
		return sched.RunEODReport()
	case "report.weekly":
		return sched.RunWeeklyReport()
	default:
		return fmt.Errorf("unknown report job: %s", jobName)
	}
}