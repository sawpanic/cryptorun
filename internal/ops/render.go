package ops

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// StatusRenderer handles rendering operational status to console and files
type StatusRenderer struct {
	outputDir string
}

// NewStatusRenderer creates a new status renderer
func NewStatusRenderer(outputDir string) *StatusRenderer {
	return &StatusRenderer{
		outputDir: outputDir,
	}
}

// RenderConsole renders status to console in a compact table format
func (r *StatusRenderer) RenderConsole(
	kpiMetrics KPIMetrics,
	guardResults []GuardResult,
	switchStatus SwitchStatus,
) {
	fmt.Println("=== CryptoRun Operational Status ===")
	fmt.Printf("Timestamp: %s\n\n", time.Now().Format("2006-01-02 15:04:05"))

	// KPI Section
	r.renderKPITable(kpiMetrics)
	fmt.Println()

	// Guards Section
	r.renderGuardsTable(guardResults)
	fmt.Println()

	// Emergency Status
	r.renderEmergencyStatus(switchStatus.Emergency)
	fmt.Println()

	// Provider Status
	r.renderProviderStatus(switchStatus.Providers)
	fmt.Println()

	// Venue Status
	r.renderVenueStatus(switchStatus.Venues)
}

// renderKPITable renders KPI metrics in table format
func (r *StatusRenderer) renderKPITable(metrics KPIMetrics) {
	fmt.Println("📊 KEY PERFORMANCE INDICATORS")
	fmt.Println("┌─────────────────────┬──────────┬────────────┐")
	fmt.Println("│ Metric              │ Value    │ Status     │")
	fmt.Println("├─────────────────────┼──────────┼────────────┤")

	// Requests per minute
	status := r.getKPIStatus(metrics.RequestsPerMinute, 100, 200)
	fmt.Printf("│ %-19s │ %8.1f │ %-10s │\n", "Requests/min", metrics.RequestsPerMinute, status)

	// Error rate
	status = r.getKPIStatus(metrics.ErrorRatePercent, 5, 15)
	fmt.Printf("│ %-19s │ %7.1f%% │ %-10s │\n", "Error rate", metrics.ErrorRatePercent, status)

	// Cache hit rate
	status = r.getCacheHitStatus(metrics.CacheHitRatePercent)
	fmt.Printf("│ %-19s │ %7.1f%% │ %-10s │\n", "Cache hit rate", metrics.CacheHitRatePercent, status)

	// Open breakers
	status = r.getBreakerStatus(metrics.OpenBreakerCount)
	fmt.Printf("│ %-19s │ %8d │ %-10s │\n", "Open breakers", metrics.OpenBreakerCount, status)

	// Venue health
	totalVenues := metrics.HealthyVenueCount + metrics.UnhealthyVenueCount
	venueText := fmt.Sprintf("%d/%d", metrics.HealthyVenueCount, totalVenues)
	status = r.getVenueHealthStatus(metrics.HealthyVenueCount, totalVenues)
	fmt.Printf("│ %-19s │ %8s │ %-10s │\n", "Healthy venues", venueText, status)

	fmt.Println("└─────────────────────┴──────────┴────────────┘")
}

// renderGuardsTable renders guard results in table format
func (r *StatusRenderer) renderGuardsTable(results []GuardResult) {
	if len(results) == 0 {
		fmt.Println("🛡️  OPERATIONAL GUARDS: No guards configured")
		return
	}

	fmt.Println("🛡️  OPERATIONAL GUARDS")
	fmt.Println("┌─────────────────────┬──────────┬─────────────────────────────────┐")
	fmt.Println("│ Guard               │ Status   │ Message                         │")
	fmt.Println("├─────────────────────┼──────────┼─────────────────────────────────┤")

	for _, result := range results {
		statusIcon := r.getStatusIcon(result.Status)
		message := r.truncateMessage(result.Message, 31)
		fmt.Printf("│ %-19s │ %s%-7s │ %-31s │\n",
			r.truncateText(result.Name, 19), statusIcon, result.Status.String(), message)
	}

	fmt.Println("└─────────────────────┴──────────┴─────────────────────────────────┘")
}

// renderEmergencyStatus renders emergency switch status
func (r *StatusRenderer) renderEmergencyStatus(emergency EmergencyStatus) {
	icon := "✅"
	if emergency.AnyEmergencyActive {
		icon = "🚨"
	}

	fmt.Printf("%s EMERGENCY SWITCHES\n", icon)
	fmt.Println("┌─────────────────────┬─────────┐")
	fmt.Println("│ Switch              │ Status  │")
	fmt.Println("├─────────────────────┼─────────┤")

	fmt.Printf("│ %-19s │ %s%-6s │\n", "All scanners", r.getBoolIcon(!emergency.AllScannersDisabled), r.getBoolText(!emergency.AllScannersDisabled))
	fmt.Printf("│ %-19s │ %s%-6s │\n", "Live data", r.getBoolIcon(!emergency.LiveDataDisabled), r.getBoolText(!emergency.LiveDataDisabled))
	fmt.Printf("│ %-19s │ %s%-6s │\n", "Read-only mode", r.getBoolIcon(!emergency.ReadOnlyMode), r.getReadOnlyText(!emergency.ReadOnlyMode))

	fmt.Println("└─────────────────────┴─────────┘")
}

// renderProviderStatus renders provider switch status
func (r *StatusRenderer) renderProviderStatus(providers map[string]ProviderStatus) {
	if len(providers) == 0 {
		fmt.Println("🏭 PROVIDER STATUS: No providers configured")
		return
	}

	fmt.Println("🏭 PROVIDER STATUS")
	fmt.Println("┌─────────────────────┬─────────┬────┬──────┬─────────────┐")
	fmt.Println("│ Provider            │ Enabled │ WS │ REST │ Operational │")
	fmt.Println("├─────────────────────┼─────────┼────┼──────┼─────────────┤")

	for _, provider := range providers {
		wsIcon := r.getBoolIcon(provider.WebsocketAllowed)
		restIcon := r.getBoolIcon(provider.RestAllowed)
		opIcon := r.getBoolIcon(provider.FullyOperational)

		fmt.Printf("│ %-19s │ %s%-6s │ %s%-1s │ %s%-3s │ %s%-10s │\n",
			r.truncateText(provider.Name, 19),
			r.getBoolIcon(provider.Enabled), r.getBoolText(provider.Enabled),
			wsIcon, r.getBoolTextShort(provider.WebsocketAllowed),
			restIcon, r.getBoolTextShort(provider.RestAllowed),
			opIcon, r.getBoolText(provider.FullyOperational))
	}

	fmt.Println("└─────────────────────┴─────────┴────┴──────┴─────────────┘")
}

// renderVenueStatus renders venue switch status
func (r *StatusRenderer) renderVenueStatus(venues map[string]VenueStatus) {
	if len(venues) == 0 {
		fmt.Println("🏢 VENUE STATUS: No venues configured")
		return
	}

	fmt.Println("🏢 VENUE STATUS")
	fmt.Println("┌─────────────────────┬─────────┬─────────────────────┐")
	fmt.Println("│ Venue               │ Status  │ Last Updated        │")
	fmt.Println("├─────────────────────┼─────────┼─────────────────────┤")

	for _, venue := range venues {
		lastUpdated := "Never"
		if !venue.LastUpdated.IsZero() {
			lastUpdated = venue.LastUpdated.Format("15:04:05")
		}

		fmt.Printf("│ %-19s │ %s%-6s │ %-19s │\n",
			r.truncateText(venue.Name, 19),
			r.getBoolIcon(venue.Enabled), r.getBoolText(venue.Enabled),
			lastUpdated)
	}

	fmt.Println("└─────────────────────┴─────────┴─────────────────────┘")
}

// WriteSnapshot writes status snapshot to CSV file
func (r *StatusRenderer) WriteSnapshot(
	kpiMetrics KPIMetrics,
	guardResults []GuardResult,
	switchStatus SwitchStatus,
) error {
	// Ensure output directory exists
	if err := os.MkdirAll(r.outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	timestamp := time.Now()
	filename := fmt.Sprintf("status_snapshot_%s.csv", timestamp.Format("20060102_150405"))
	filePath := filepath.Join(r.outputDir, filename)

	file, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("failed to create snapshot file: %w", err)
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// Write header
	header := []string{
		"timestamp", "category", "name", "value", "status", "message",
	}
	if err := writer.Write(header); err != nil {
		return fmt.Errorf("failed to write CSV header: %w", err)
	}

	// Write KPI data
	r.writeKPIData(writer, timestamp, kpiMetrics)

	// Write guard data
	r.writeGuardData(writer, timestamp, guardResults)

	// Write switch data
	r.writeSwitchData(writer, timestamp, switchStatus)

	// Also write to standard filename for easy access
	standardPath := filepath.Join(r.outputDir, "status_snapshot.csv")
	if err := r.copyFile(filePath, standardPath); err != nil {
		// Log error but don't fail the operation
		fmt.Printf("Warning: failed to copy to standard filename: %v\n", err)
	}

	fmt.Printf("📁 Snapshot written to: %s\n", filePath)
	return nil
}

// writeKPIData writes KPI metrics to CSV
func (r *StatusRenderer) writeKPIData(writer *csv.Writer, timestamp time.Time, metrics KPIMetrics) {
	tsStr := timestamp.Format("2006-01-02 15:04:05")

	records := [][]string{
		{tsStr, "kpi", "requests_per_minute", fmt.Sprintf("%.1f", metrics.RequestsPerMinute), r.getKPIStatus(metrics.RequestsPerMinute, 100, 200), ""},
		{tsStr, "kpi", "error_rate_percent", fmt.Sprintf("%.1f", metrics.ErrorRatePercent), r.getKPIStatus(metrics.ErrorRatePercent, 5, 15), ""},
		{tsStr, "kpi", "cache_hit_rate_percent", fmt.Sprintf("%.1f", metrics.CacheHitRatePercent), r.getCacheHitStatus(metrics.CacheHitRatePercent), ""},
		{tsStr, "kpi", "open_breaker_count", fmt.Sprintf("%d", metrics.OpenBreakerCount), r.getBreakerStatus(metrics.OpenBreakerCount), ""},
		{tsStr, "kpi", "healthy_venue_count", fmt.Sprintf("%d", metrics.HealthyVenueCount), "", ""},
		{tsStr, "kpi", "unhealthy_venue_count", fmt.Sprintf("%d", metrics.UnhealthyVenueCount), "", ""},
	}

	for _, record := range records {
		writer.Write(record)
	}
}

// writeGuardData writes guard results to CSV
func (r *StatusRenderer) writeGuardData(writer *csv.Writer, timestamp time.Time, results []GuardResult) {
	tsStr := timestamp.Format("2006-01-02 15:04:05")

	for _, result := range results {
		record := []string{
			tsStr, "guard", result.Name, "", result.Status.String(), result.Message,
		}
		writer.Write(record)
	}
}

// writeSwitchData writes switch status to CSV
func (r *StatusRenderer) writeSwitchData(writer *csv.Writer, timestamp time.Time, status SwitchStatus) {
	tsStr := timestamp.Format("2006-01-02 15:04:05")

	// Emergency switches
	records := [][]string{
		{tsStr, "switch", "emergency_scanners", r.getBoolText(!status.Emergency.AllScannersDisabled), "", ""},
		{tsStr, "switch", "emergency_live_data", r.getBoolText(!status.Emergency.LiveDataDisabled), "", ""},
		{tsStr, "switch", "emergency_readonly", r.getBoolText(!status.Emergency.ReadOnlyMode), "", ""},
	}

	// Provider switches
	for name, provider := range status.Providers {
		records = append(records, []string{
			tsStr, "switch", "provider_" + name, r.getBoolText(provider.Enabled), "", "",
		})
	}

	// Venue switches
	for name, venue := range status.Venues {
		records = append(records, []string{
			tsStr, "switch", "venue_" + name, r.getBoolText(venue.Enabled), "", "",
		})
	}

	for _, record := range records {
		writer.Write(record)
	}
}

// Helper functions for formatting

func (r *StatusRenderer) getKPIStatus(value, warn, critical float64) string {
	if value >= critical {
		return "CRITICAL"
	} else if value >= warn {
		return "WARN"
	}
	return "OK"
}

func (r *StatusRenderer) getCacheHitStatus(value float64) string {
	if value < 50 {
		return "CRITICAL"
	} else if value < 75 {
		return "WARN"
	}
	return "OK"
}

func (r *StatusRenderer) getBreakerStatus(count int) string {
	if count > 2 {
		return "CRITICAL"
	} else if count > 0 {
		return "WARN"
	}
	return "OK"
}

func (r *StatusRenderer) getVenueHealthStatus(healthy, total int) string {
	if total == 0 {
		return "UNKNOWN"
	}
	ratio := float64(healthy) / float64(total)
	if ratio < 0.5 {
		return "CRITICAL"
	} else if ratio < 0.8 {
		return "WARN"
	}
	return "OK"
}

func (r *StatusRenderer) getStatusIcon(status GuardStatus) string {
	switch status {
	case GuardStatusOK:
		return "✅"
	case GuardStatusWarn:
		return "⚠️ "
	case GuardStatusCritical:
		return "🔴"
	case GuardStatusBlock:
		return "🚫"
	default:
		return "❓"
	}
}

func (r *StatusRenderer) getBoolIcon(enabled bool) string {
	if enabled {
		return "✅"
	}
	return "❌"
}

func (r *StatusRenderer) getBoolText(enabled bool) string {
	if enabled {
		return "ON"
	}
	return "OFF"
}

func (r *StatusRenderer) getBoolTextShort(enabled bool) string {
	if enabled {
		return "Y"
	}
	return "N"
}

func (r *StatusRenderer) getReadOnlyText(notReadOnly bool) string {
	if notReadOnly {
		return "WRITE"
	}
	return "READ"
}

func (r *StatusRenderer) truncateText(text string, maxLen int) string {
	if len(text) <= maxLen {
		return text
	}
	if maxLen < 3 {
		return text[:maxLen]
	}
	return text[:maxLen-3] + "..."
}

func (r *StatusRenderer) truncateMessage(message string, maxLen int) string {
	if len(message) <= maxLen {
		return message
	}
	if maxLen < 3 {
		return message[:maxLen]
	}
	return message[:maxLen-3] + "..."
}

func (r *StatusRenderer) copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = destFile.ReadFrom(sourceFile)
	return err
}
