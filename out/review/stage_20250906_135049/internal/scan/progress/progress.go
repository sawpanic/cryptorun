package progress

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// ScanEvent represents a progress event during scanning
type ScanEvent struct {
	Timestamp time.Time              `json:"timestamp"`
	Phase     string                 `json:"phase"`     // init, fetch, analyze, orthogonalize, filter, complete
	Symbol    string                 `json:"symbol,omitempty"`
	Status    string                 `json:"status"`    // start, progress, success, error
	Progress  int                    `json:"progress"`  // 0-100
	Total     int                    `json:"total"`     // total items to process
	Current   int                    `json:"current"`   // current item being processed
	Message   string                 `json:"message,omitempty"`
	Error     string                 `json:"error,omitempty"`
	Metrics   map[string]interface{} `json:"metrics,omitempty"`
}

// ScanProgressPrinter handles different output formats for scan progress
type ScanProgressPrinter interface {
	ScanStart(pipeline string, symbols []string)
	ScanEvent(event ScanEvent)
	ScanComplete(candidates int, outputPath string)
}

// JSONScanPrinter outputs structured JSON progress
type JSONScanPrinter struct {
	startTime time.Time
}

func NewJSONScanPrinter() ScanProgressPrinter {
	return &JSONScanPrinter{}
}

func (p *JSONScanPrinter) ScanStart(pipeline string, symbols []string) {
	p.startTime = time.Now()
	output := map[string]interface{}{
		"event":     "scan_start",
		"timestamp": p.startTime.Format(time.RFC3339),
		"pipeline":  pipeline,
		"symbols":   symbols,
		"count":     len(symbols),
	}
	json.NewEncoder(os.Stdout).Encode(output)
}

func (p *JSONScanPrinter) ScanEvent(event ScanEvent) {
	json.NewEncoder(os.Stdout).Encode(event)
}

func (p *JSONScanPrinter) ScanComplete(candidates int, outputPath string) {
	output := map[string]interface{}{
		"event":      "scan_complete",
		"timestamp":  time.Now().Format(time.RFC3339),
		"candidates": candidates,
		"output":     outputPath,
		"duration":   time.Since(p.startTime).Milliseconds(),
	}
	json.NewEncoder(os.Stdout).Encode(output)
}

// PlainScanPrinter outputs human-readable progress
type PlainScanPrinter struct {
	startTime    time.Time
	currentPhase string
	symbolCount  int
	phaseCount   map[string]int
}

func NewPlainScanPrinter() ScanProgressPrinter {
	return &PlainScanPrinter{
		phaseCount: make(map[string]int),
	}
}

func (p *PlainScanPrinter) ScanStart(pipeline string, symbols []string) {
	p.startTime = time.Now()
	p.symbolCount = len(symbols)
	fmt.Printf("🔍 Starting %s scan (%d symbols)\n", pipeline, len(symbols))
	fmt.Printf("Symbols: %v\n", symbols)
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
}

func (p *PlainScanPrinter) ScanEvent(event ScanEvent) {
	// Track phase transitions
	if p.currentPhase != event.Phase {
		p.currentPhase = event.Phase
		p.phaseCount[event.Phase]++
		
		switch event.Phase {
		case "init":
			fmt.Printf("📋 Initializing pipeline...\n")
		case "fetch":
			fmt.Printf("📊 Fetching market data...\n")
		case "analyze":
			fmt.Printf("🧮 Analyzing momentum signals...\n")
		case "orthogonalize":
			fmt.Printf("📐 Applying orthogonalization...\n")
		case "filter":
			fmt.Printf("🔍 Filtering candidates...\n")
		}
	}
	
	// Show progress for specific events
	if event.Status == "progress" && event.Total > 0 {
		if event.Symbol != "" {
			progressPct := int((float64(event.Current) / float64(event.Total)) * 100)
			fmt.Printf("  [%d%%] %s: %s\n", progressPct, event.Phase, event.Symbol)
		} else {
			progressPct := int((float64(event.Current) / float64(event.Total)) * 100)
			fmt.Printf("  [%d%%] %s (%d/%d)\n", progressPct, event.Phase, event.Current, event.Total)
		}
	}
	
	// Show errors
	if event.Status == "error" {
		fmt.Printf("  ❌ Error in %s: %s\n", event.Phase, event.Error)
	}
	
	// Show metrics for important phases
	if event.Status == "success" && len(event.Metrics) > 0 {
		if event.Phase == "analyze" {
			if score, ok := event.Metrics["momentum_score"]; ok {
				fmt.Printf("  ✅ %s: score=%.2f\n", event.Symbol, score)
			}
		}
	}
}

func (p *PlainScanPrinter) ScanComplete(candidates int, outputPath string) {
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	duration := time.Since(p.startTime)
	fmt.Printf("✅ Scan completed: %d candidates found (%v)\n", candidates, duration.Round(time.Second))
	fmt.Printf("📁 Results: %s\n", outputPath)
}

// AutoScanPrinter adapts output based on environment
type AutoScanPrinter struct {
	delegate ScanProgressPrinter
}

func NewAutoScanPrinter() ScanProgressPrinter {
	// Use plain output unless in CI or when output is redirected
	if os.Getenv("CI") != "" || !isTerminal() {
		return &AutoScanPrinter{delegate: NewJSONScanPrinter()}
	}
	return &AutoScanPrinter{delegate: NewPlainScanPrinter()}
}

func (p *AutoScanPrinter) ScanStart(pipeline string, symbols []string) {
	p.delegate.ScanStart(pipeline, symbols)
}

func (p *AutoScanPrinter) ScanEvent(event ScanEvent) {
	p.delegate.ScanEvent(event)
}

func (p *AutoScanPrinter) ScanComplete(candidates int, outputPath string) {
	p.delegate.ScanComplete(candidates, outputPath)
}

// ScanProgressTracker records progress to JSONL file
type ScanProgressTracker struct {
	progressFile string
}

func NewScanProgressTracker(auditDir string) *ScanProgressTracker {
	return &ScanProgressTracker{
		progressFile: filepath.Join(auditDir, "progress_trace.jsonl"),
	}
}

func (st *ScanProgressTracker) RecordEvent(event ScanEvent) error {
	// Ensure audit directory exists
	if err := os.MkdirAll(filepath.Dir(st.progressFile), 0755); err != nil {
		return err
	}
	
	// Append to JSONL file
	file, err := os.OpenFile(st.progressFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer file.Close()
	
	encoder := json.NewEncoder(file)
	return encoder.Encode(event)
}

// Helper function
func isTerminal() bool {
	stat, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return (stat.Mode() & os.ModeCharDevice) != 0
}

// ScanProgressBus coordinates progress streaming and file writing
type ScanProgressBus struct {
	printer ScanProgressPrinter
	tracker *ScanProgressTracker
}

func NewScanProgressBus(progressMode string, auditDir string) *ScanProgressBus {
	var printer ScanProgressPrinter
	switch progressMode {
	case "json":
		printer = NewJSONScanPrinter()
	case "plain":
		printer = NewPlainScanPrinter()
	default: // auto
		printer = NewAutoScanPrinter()
	}
	
	tracker := NewScanProgressTracker(auditDir)
	
	return &ScanProgressBus{
		printer: printer,
		tracker: tracker,
	}
}

func (spb *ScanProgressBus) ScanStart(pipeline string, symbols []string) {
	spb.printer.ScanStart(pipeline, symbols)
	
	event := ScanEvent{
		Timestamp: time.Now(),
		Phase:     "init",
		Status:    "start",
		Message:   fmt.Sprintf("Starting %s scan with %d symbols", pipeline, len(symbols)),
		Metrics: map[string]interface{}{
			"pipeline":    pipeline,
			"symbol_count": len(symbols),
		},
	}
	spb.tracker.RecordEvent(event)
}

func (spb *ScanProgressBus) ScanEvent(event ScanEvent) {
	event.Timestamp = time.Now()
	spb.printer.ScanEvent(event)
	spb.tracker.RecordEvent(event)
}

func (spb *ScanProgressBus) ScanComplete(candidates int, outputPath string) {
	spb.printer.ScanComplete(candidates, outputPath)
	
	event := ScanEvent{
		Timestamp: time.Now(),
		Phase:     "complete",
		Status:    "success",
		Message:   fmt.Sprintf("Scan completed with %d candidates", candidates),
		Metrics: map[string]interface{}{
			"candidates":  candidates,
			"output_path": outputPath,
		},
	}
	spb.tracker.RecordEvent(event)
}