package qa

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Printer interface for different output formats
type Printer interface {
	Start(totalPhases int)
	Phase(result PhaseResult)
	Complete(result *RunResult)
}

// ProgressTracker handles checkpoint persistence
type ProgressTracker interface {
	RecordPhase(phase int, result PhaseResult) error
	GetLastCompletedPhase() (int, error)
}

// JSONPrinter outputs structured JSON progress
type JSONPrinter struct {
	startTime time.Time
}

func NewJSONPrinter() Printer {
	return &JSONPrinter{}
}

func (p *JSONPrinter) Start(totalPhases int) {
	p.startTime = time.Now()
	output := map[string]interface{}{
		"event":        "qa_start",
		"timestamp":    p.startTime.Format(time.RFC3339),
		"total_phases": totalPhases,
	}
	p.printJSON(output)
}

func (p *JSONPrinter) Phase(result PhaseResult) {
	output := map[string]interface{}{
		"event":     "qa_phase",
		"timestamp": time.Now().Format(time.RFC3339),
		"phase":     result.Phase,
		"name":      result.Name,
		"status":    result.Status,
		"duration":  result.Duration.Milliseconds(),
		"error":     result.Error,
		"metrics":   result.Metrics,
	}
	p.printJSON(output)
}

func (p *JSONPrinter) Complete(result *RunResult) {
	output := map[string]interface{}{
		"event":             "qa_complete",
		"timestamp":         time.Now().Format(time.RFC3339),
		"success":           result.Success,
		"failure_reason":    result.FailureReason,
		"passed_phases":     result.PassedPhases,
		"total_phases":      result.TotalPhases,
		"healthy_providers": result.HealthyProviders,
		"total_duration":    result.TotalDuration.Milliseconds(),
		"artifacts_path":    result.ArtifactsPath,
	}
	p.printJSON(output)
}

func (p *JSONPrinter) printJSON(data map[string]interface{}) {
	json.NewEncoder(os.Stdout).Encode(data)
}

// PlainPrinter outputs human-readable text
type PlainPrinter struct {
	startTime time.Time
}

func NewPlainPrinter() Printer {
	return &PlainPrinter{}
}

func (p *PlainPrinter) Start(totalPhases int) {
	p.startTime = time.Now()
	fmt.Printf("🚀 Starting QA suite (%d phases)\n", totalPhases)
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
}

func (p *PlainPrinter) Phase(result PhaseResult) {
	status := "✅"
	if result.Status == "fail" {
		status = "❌"
	} else if result.Status == "skip" {
		status = "⏭️"
	}

	fmt.Printf("[%d] %s %s (%v)\n",
		result.Phase, status, result.Name, result.Duration.Round(time.Millisecond))

	if result.Error != "" {
		fmt.Printf("    Error: %s\n", result.Error)
	}

	// Print key metrics
	if len(result.Metrics) > 0 {
		fmt.Printf("    Metrics: ")
		first := true
		for key, value := range result.Metrics {
			if !first {
				fmt.Printf(", ")
			}
			fmt.Printf("%s=%v", key, formatMetricValue(value))
			first = false
		}
		fmt.Println()
	}
}

func (p *PlainPrinter) Complete(result *RunResult) {
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	if result.Success {
		fmt.Printf("✅ QA PASSED (%d/%d phases, %v total)\n",
			result.PassedPhases, result.TotalPhases, result.TotalDuration.Round(time.Second))

		// Show acceptance verification status if present
		if len(result.PhaseResults) > 0 {
			lastPhase := result.PhaseResults[len(result.PhaseResults)-1]
			if lastPhase.Phase == 7 && lastPhase.Name == "Acceptance Verification" {
				fmt.Printf("🔍 Acceptance: %d files validated, %s\n",
					getMetricInt(lastPhase.Metrics, "validated_files"),
					getMetricString(lastPhase.Metrics, "metrics_status"))
			}
		}
	} else {
		fmt.Printf("❌ QA FAILED (%d/%d phases, %v total)\n",
			result.PassedPhases, result.TotalPhases, result.TotalDuration.Round(time.Second))
		if result.FailureReason != "" {
			fmt.Printf("   Reason: %s\n", result.FailureReason)
		}
	}

	if result.ArtifactsPath != "" {
		fmt.Printf("📁 Artifacts: %s\n", result.ArtifactsPath)
	}
}

// AutoPrinter adapts output based on environment
type AutoPrinter struct {
	delegate Printer
}

func NewAutoPrinter() Printer {
	// Use plain output unless in CI or when output is redirected
	if os.Getenv("CI") != "" || !isTerminal() {
		return &AutoPrinter{delegate: NewJSONPrinter()}
	}
	return &AutoPrinter{delegate: NewPlainPrinter()}
}

func (p *AutoPrinter) Start(totalPhases int) {
	p.delegate.Start(totalPhases)
}

func (p *AutoPrinter) Phase(result PhaseResult) {
	p.delegate.Phase(result)
}

func (p *AutoPrinter) Complete(result *RunResult) {
	p.delegate.Complete(result)
}

// FileProgressTracker persists progress to disk
type FileProgressTracker struct {
	progressFile string
}

func NewProgressTracker(auditDir string) ProgressTracker {
	return &FileProgressTracker{
		progressFile: filepath.Join(auditDir, "progress_trace.jsonl"),
	}
}

func (pt *FileProgressTracker) RecordPhase(phase int, result PhaseResult) error {
	// Ensure audit directory exists
	if err := os.MkdirAll(filepath.Dir(pt.progressFile), 0755); err != nil {
		return err
	}

	// Create progress entry
	entry := map[string]interface{}{
		"timestamp": time.Now().Format(time.RFC3339),
		"phase":     phase,
		"name":      result.Name,
		"status":    result.Status,
		"duration":  result.Duration.Milliseconds(),
		"error":     result.Error,
	}

	// Append to JSONL file
	file, err := os.OpenFile(pt.progressFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	return encoder.Encode(entry)
}

func (pt *FileProgressTracker) GetLastCompletedPhase() (int, error) {
	file, err := os.Open(pt.progressFile)
	if err != nil {
		if os.IsNotExist(err) {
			return -1, nil // No previous progress
		}
		return -1, err
	}
	defer file.Close()

	// Read all lines and find the last completed phase
	decoder := json.NewDecoder(file)
	lastPhase := -1

	for decoder.More() {
		var entry map[string]interface{}
		if err := decoder.Decode(&entry); err != nil {
			break // Skip malformed entries
		}

		if status, ok := entry["status"].(string); ok && status == "pass" {
			if phase, ok := entry["phase"].(float64); ok {
				if int(phase) > lastPhase {
					lastPhase = int(phase)
				}
			}
		}
	}

	return lastPhase, nil
}

// Helper functions
func formatMetricValue(value interface{}) string {
	switch v := value.(type) {
	case map[string]interface{}:
		return fmt.Sprintf("(%d items)", len(v))
	case []interface{}:
		return fmt.Sprintf("(%d items)", len(v))
	case string:
		if len(v) > 20 {
			return v[:20] + "..."
		}
		return v
	default:
		return fmt.Sprintf("%v", v)
	}
}

func isTerminal() bool {
	// Simple check if stdout is connected to a terminal
	stat, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return (stat.Mode() & os.ModeCharDevice) != 0
}

func getMetricInt(metrics map[string]interface{}, key string) int {
	if val, ok := metrics[key]; ok {
		if intVal, ok := val.(int); ok {
			return intVal
		}
	}
	return 0
}

func getMetricString(metrics map[string]interface{}, key string) string {
	if val, ok := metrics[key]; ok {
		if strVal, ok := val.(string); ok {
			return strVal
		}
	}
	return "unknown"
}
