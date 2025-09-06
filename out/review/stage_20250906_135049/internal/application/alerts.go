package application

import (
	"crypto/md5"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
	"gopkg.in/yaml.v3"
)

// AlertsConfig represents the complete alerts configuration
type AlertsConfig struct {
	Alerts       AlertsSettings     `yaml:"alerts"`
	Destinations DestinationConfig  `yaml:"destinations"`
	Thresholds   ThresholdSettings  `yaml:"thresholds"`
	Throttles    ThrottleSettings   `yaml:"throttles"`
	Formats      FormatSettings     `yaml:"formats"`
	Safety       SafetySettings     `yaml:"safety"`
	Features     FeatureSettings    `yaml:"features"`
}

// AlertsSettings contains global alert controls
type AlertsSettings struct {
	Enabled        bool `yaml:"enabled"`
	DryRunDefault  bool `yaml:"dry_run_default"`
}

// DestinationConfig contains provider configurations
type DestinationConfig struct {
	Discord  DiscordConfig  `yaml:"discord"`
	Telegram TelegramConfig `yaml:"telegram"`
}

// DiscordConfig contains Discord webhook configuration
type DiscordConfig struct {
	Enabled    bool   `yaml:"enabled"`
	WebhookURL string `yaml:"webhook_url"`
	Username   string `yaml:"username"`
	AvatarURL  string `yaml:"avatar_url"`
}

// TelegramConfig contains Telegram bot configuration
type TelegramConfig struct {
	Enabled  bool   `yaml:"enabled"`
	BotToken string `yaml:"bot_token"`
	ChatID   string `yaml:"chat_id"`
}

// ThresholdSettings contains alert trigger thresholds
type ThresholdSettings struct {
	ScoreMin             float64 `yaml:"score_min"`
	FreshnessMaxBars     int     `yaml:"freshness_max_bars"`
	SpreadBpsMax         float64 `yaml:"spread_bps_max"`
	DepthUsdMin          float64 `yaml:"depth_usd_min"`
	ExitMinHoldMinutes   int     `yaml:"exit_min_hold_minutes"`
	ExitPnlThreshold     float64 `yaml:"exit_pnl_threshold"`
}

// ThrottleSettings contains throttling and deduplication rules
type ThrottleSettings struct {
	MinIntervalPerSymbol    int           `yaml:"min_interval_per_symbol"`
	GlobalRateLimit         int           `yaml:"global_rate_limit"`
	ResendCooloffAfterExit  int           `yaml:"resend_cooloff_after_exit"`
	QuietHours              QuietHours    `yaml:"quiet_hours"`
	MaxAlertsPerHour        int           `yaml:"max_alerts_per_hour"`
	MaxAlertsPerDay         int           `yaml:"max_alerts_per_day"`
}

// QuietHours defines silent periods
type QuietHours struct {
	Enabled bool   `yaml:"enabled"`
	Start   string `yaml:"start"`
	End     string `yaml:"end"`
}

// FormatSettings contains message templates
type FormatSettings struct {
	EntryTemplate string `yaml:"entry_template"`
	ExitTemplate  string `yaml:"exit_template"`
}

// SafetySettings contains safety constraints
type SafetySettings struct {
	AllowedVenues            []string `yaml:"allowed_venues"`
	BannedAggregators        []string `yaml:"banned_aggregators"`
	SocialCapMax             float64  `yaml:"social_cap_max"`
	EnforceMomentumPriority  bool     `yaml:"enforce_momentum_priority"`
	MaxDataAgeMinutes        int      `yaml:"max_data_age_minutes"`
	RequireVenueNative       bool     `yaml:"require_venue_native"`
}

// FeatureSettings contains feature flags
type FeatureSettings struct {
	CandidateAlerts         bool `yaml:"candidate_alerts"`
	ExitAlerts              bool `yaml:"exit_alerts"`
	ThrottleBypassCritical  bool `yaml:"throttle_bypass_critical"`
	IncludeDebugInfo        bool `yaml:"include_debug_info"`
}

// AlertEvent represents an alertable event
type AlertEvent struct {
	Type        AlertType              `json:"type"`
	Symbol      string                 `json:"symbol"`
	Timestamp   time.Time              `json:"timestamp"`
	Data        map[string]interface{} `json:"data"`
	Priority    AlertPriority          `json:"priority"`
	Fingerprint string                 `json:"fingerprint"`
}

// AlertType represents the type of alert event
type AlertType string

const (
	AlertTypeEntry AlertType = "entry"
	AlertTypeExit  AlertType = "exit"
)

// AlertPriority represents alert priority levels
type AlertPriority string

const (
	AlertPriorityLow      AlertPriority = "low"
	AlertPriorityNormal   AlertPriority = "normal"
	AlertPriorityHigh     AlertPriority = "high"
	AlertPriorityCritical AlertPriority = "critical"
)

// AlertState tracks alert history for throttling
type AlertState struct {
	LastAlertTime    map[string]time.Time `json:"last_alert_time"`
	AlertCounts      AlertCounts          `json:"alert_counts"`
	ExitCooldowns    map[string]time.Time `json:"exit_cooldowns"`
	GlobalLastAlert  time.Time            `json:"global_last_alert"`
}

// AlertCounts tracks alert frequency
type AlertCounts struct {
	HourlyCount int       `json:"hourly_count"`
	DailyCount  int       `json:"daily_count"`
	LastReset   time.Time `json:"last_reset"`
}

// AlertManager handles the complete alerting workflow
type AlertManager struct {
	config        *AlertsConfig
	state         *AlertState
	providers     []AlertProvider
	dryRun        bool
}

// AlertProvider interface for alert destinations
type AlertProvider interface {
	Name() string
	SendAlert(event *AlertEvent, message string) error
	IsEnabled() bool
	ValidateConfig() error
}

// NewAlertManager creates a new alert manager
func NewAlertManager(configPath string, dryRun bool) (*AlertManager, error) {
	config, err := LoadAlertsConfig(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load alerts config: %w", err)
	}

	state := &AlertState{
		LastAlertTime: make(map[string]time.Time),
		AlertCounts:   AlertCounts{LastReset: time.Now()},
		ExitCooldowns: make(map[string]time.Time),
	}

	manager := &AlertManager{
		config: config,
		state:  state,
		dryRun: dryRun,
	}

	// Initialize providers
	if err := manager.initializeProviders(); err != nil {
		return nil, fmt.Errorf("failed to initialize providers: %w", err)
	}

	return manager, nil
}

// LoadAlertsConfig loads configuration from YAML file
func LoadAlertsConfig(path string) (*AlertsConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	// Expand environment variables
	content := os.ExpandEnv(string(data))

	var config AlertsConfig
	if err := yaml.Unmarshal([]byte(content), &config); err != nil {
		return nil, err
	}

	return &config, nil
}

// initializeProviders sets up alert providers
func (am *AlertManager) initializeProviders() error {
	am.providers = []AlertProvider{}

	// Initialize Discord provider
	if am.config.Destinations.Discord.Enabled {
		discordProvider := NewDiscordProvider(&am.config.Destinations.Discord)
		if err := discordProvider.ValidateConfig(); err != nil {
			log.Warn().Err(err).Msg("Discord provider validation failed")
		} else {
			am.providers = append(am.providers, discordProvider)
		}
	}

	// Initialize Telegram provider
	if am.config.Destinations.Telegram.Enabled {
		telegramProvider := NewTelegramProvider(&am.config.Destinations.Telegram)
		if err := telegramProvider.ValidateConfig(); err != nil {
			log.Warn().Err(err).Msg("Telegram provider validation failed")
		} else {
			am.providers = append(am.providers, telegramProvider)
		}
	}

	return nil
}

// ProcessCandidateAlert processes new candidate alerts
func (am *AlertManager) ProcessCandidateAlert(candidate *ScanCandidate) error {
	if !am.config.Features.CandidateAlerts {
		return nil
	}

	// Validate safety constraints
	if err := am.validateSafetyConstraints(candidate); err != nil {
		log.Warn().Err(err).Str("symbol", candidate.Symbol).Msg("Alert safety constraint violation")
		return err
	}

	// Check thresholds
	if !am.meetsEntryThresholds(candidate) {
		return nil // Doesn't meet alert criteria
	}

	// Create alert event
	event := &AlertEvent{
		Type:      AlertTypeEntry,
		Symbol:    candidate.Symbol,
		Timestamp: time.Now(),
		Priority:  am.determinePriority(candidate),
		Data:      am.buildCandidateData(candidate),
	}
	event.Fingerprint = am.generateFingerprint(event)

	// Check throttling
	if am.shouldThrottle(event) {
		log.Debug().Str("symbol", event.Symbol).Msg("Alert throttled")
		return nil
	}

	return am.sendAlert(event)
}

// ProcessExitAlert processes exit signal alerts
func (am *AlertManager) ProcessExitAlert(exitData *ExitSignal) error {
	if !am.config.Features.ExitAlerts {
		return nil
	}

	// Check minimum hold time
	holdDuration := time.Since(exitData.EntryTime)
	minHold := time.Duration(am.config.Thresholds.ExitMinHoldMinutes) * time.Minute
	if holdDuration < minHold {
		return nil
	}

	// Create alert event
	event := &AlertEvent{
		Type:      AlertTypeExit,
		Symbol:    exitData.Symbol,
		Timestamp: time.Now(),
		Priority:  am.determineExitPriority(exitData),
		Data:      am.buildExitData(exitData),
	}
	event.Fingerprint = am.generateFingerprint(event)

	// Set exit cooldown
	am.state.ExitCooldowns[exitData.Symbol] = time.Now()

	return am.sendAlert(event)
}

// validateSafetyConstraints enforces safety rules
func (am *AlertManager) validateSafetyConstraints(candidate *ScanCandidate) error {
	// Check venue is native (not aggregator)
	if am.config.Safety.RequireVenueNative {
		venue := strings.ToLower(candidate.Microstructure.Venue)
		
		// Check against banned aggregators
		for _, banned := range am.config.Safety.BannedAggregators {
			if strings.Contains(venue, banned) {
				return fmt.Errorf("banned aggregator venue: %s", venue)
			}
		}

		// Check against allowed venues
		allowed := false
		for _, allowedVenue := range am.config.Safety.AllowedVenues {
			if strings.Contains(venue, allowedVenue) {
				allowed = true
				break
			}
		}
		if !allowed {
			return fmt.Errorf("venue not in allowed list: %s", venue)
		}
	}

	// Check social cap enforcement
	if am.config.Safety.EnforceMomentumPriority && candidate.Factors.Social > am.config.Safety.SocialCapMax {
		return fmt.Errorf("social factor exceeds cap: %.2f > %.2f", 
			candidate.Factors.Social, am.config.Safety.SocialCapMax)
	}

	// Check data freshness
	dataAge := time.Since(candidate.Timestamp)
	maxAge := time.Duration(am.config.Safety.MaxDataAgeMinutes) * time.Minute
	if dataAge > maxAge {
		return fmt.Errorf("data too old: %v > %v", dataAge, maxAge)
	}

	return nil
}

// meetsEntryThresholds checks if candidate meets alert thresholds
func (am *AlertManager) meetsEntryThresholds(candidate *ScanCandidate) bool {
	// Check composite score
	if candidate.Score.Score < am.config.Thresholds.ScoreMin {
		return false
	}

	// Check freshness
	if candidate.Freshness.BarsSinceSignal > am.config.Thresholds.FreshnessMaxBars {
		return false
	}

	// Check microstructure
	if candidate.Microstructure.SpreadBps > am.config.Thresholds.SpreadBpsMax {
		return false
	}

	if candidate.Microstructure.DepthUsd < am.config.Thresholds.DepthUsdMin {
		return false
	}

	return true
}

// shouldThrottle determines if alert should be throttled
func (am *AlertManager) shouldThrottle(event *AlertEvent) bool {
	now := time.Now()

	// Check global rate limit
	globalInterval := time.Duration(am.config.Throttles.GlobalRateLimit) * time.Second
	if now.Sub(am.state.GlobalLastAlert) < globalInterval {
		return true
	}

	// Check per-symbol throttling
	symbolInterval := time.Duration(am.config.Throttles.MinIntervalPerSymbol) * time.Second
	if lastAlert, exists := am.state.LastAlertTime[event.Symbol]; exists {
		if now.Sub(lastAlert) < symbolInterval {
			return true
		}
	}

	// Check exit cooldown
	if event.Type == AlertTypeEntry {
		cooloffDuration := time.Duration(am.config.Throttles.ResendCooloffAfterExit) * time.Second
		if exitTime, exists := am.state.ExitCooldowns[event.Symbol]; exists {
			if now.Sub(exitTime) < cooloffDuration {
				return true
			}
		}
	}

	// Check quiet hours
	if am.config.Throttles.QuietHours.Enabled && am.isQuietHour(now) {
		return true
	}

	// Check burst limits
	am.updateAlertCounts()
	if am.state.AlertCounts.HourlyCount >= am.config.Throttles.MaxAlertsPerHour {
		return true
	}
	if am.state.AlertCounts.DailyCount >= am.config.Throttles.MaxAlertsPerDay {
		return true
	}

	return false
}

// isQuietHour checks if current time is within quiet hours
func (am *AlertManager) isQuietHour(t time.Time) bool {
	utc := t.UTC()
	hour := utc.Format("15:04")
	
	start := am.config.Throttles.QuietHours.Start
	end := am.config.Throttles.QuietHours.End
	
	// Handle overnight quiet hours (e.g., 22:00 to 08:00)
	if start > end {
		return hour >= start || hour <= end
	}
	
	// Handle same-day quiet hours (e.g., 12:00 to 14:00)
	return hour >= start && hour <= end
}

// updateAlertCounts updates alert frequency counters
func (am *AlertManager) updateAlertCounts() {
	now := time.Now()
	
	// Reset hourly count if needed
	if now.Sub(am.state.AlertCounts.LastReset) > time.Hour {
		am.state.AlertCounts.HourlyCount = 0
	}
	
	// Reset daily count if needed
	if now.Sub(am.state.AlertCounts.LastReset) > 24*time.Hour {
		am.state.AlertCounts.DailyCount = 0
		am.state.AlertCounts.LastReset = now
	}
}

// sendAlert sends alert to all enabled providers
func (am *AlertManager) sendAlert(event *AlertEvent) error {
	// Format message
	message, err := am.formatMessage(event)
	if err != nil {
		return fmt.Errorf("failed to format message: %w", err)
	}

	// In dry-run mode, just log the formatted message
	if am.dryRun || !am.config.Alerts.Enabled {
		log.Info().
			Str("type", string(event.Type)).
			Str("symbol", event.Symbol).
			Str("priority", string(event.Priority)).
			Bool("dry_run", am.dryRun).
			Msg("DRY-RUN Alert (would send)")
		
		fmt.Printf("=== DRY-RUN ALERT ===\n%s\n===================\n", message)
		return nil
	}

	// Send to all providers
	var errors []string
	sent := false
	
	for _, provider := range am.providers {
		if !provider.IsEnabled() {
			continue
		}

		if err := provider.SendAlert(event, message); err != nil {
			log.Error().
				Err(err).
				Str("provider", provider.Name()).
				Str("symbol", event.Symbol).
				Msg("Failed to send alert")
			errors = append(errors, fmt.Sprintf("%s: %v", provider.Name(), err))
		} else {
			log.Info().
				Str("provider", provider.Name()).
				Str("symbol", event.Symbol).
				Str("type", string(event.Type)).
				Msg("Alert sent successfully")
			sent = true
		}
	}

	if sent {
		// Update state
		am.state.LastAlertTime[event.Symbol] = event.Timestamp
		am.state.GlobalLastAlert = event.Timestamp
		am.state.AlertCounts.HourlyCount++
		am.state.AlertCounts.DailyCount++
	}

	if len(errors) > 0 {
		return fmt.Errorf("alert sending errors: %s", strings.Join(errors, "; "))
	}

	return nil
}

// formatMessage formats alert event into message text
func (am *AlertManager) formatMessage(event *AlertEvent) (string, error) {
	var template string
	
	switch event.Type {
	case AlertTypeEntry:
		template = am.config.Formats.EntryTemplate
	case AlertTypeExit:
		template = am.config.Formats.ExitTemplate
	default:
		return "", fmt.Errorf("unknown alert type: %s", event.Type)
	}

	// Replace template variables
	message := template
	for key, value := range event.Data {
		placeholder := fmt.Sprintf("{%s}", key)
		message = strings.ReplaceAll(message, placeholder, fmt.Sprintf("%v", value))
	}

	return message, nil
}

// generateFingerprint creates unique identifier for deduplication
func (am *AlertManager) generateFingerprint(event *AlertEvent) string {
	data := fmt.Sprintf("%s:%s:%s", event.Type, event.Symbol, event.Timestamp.Format("2006-01-02T15:04"))
	hash := md5.Sum([]byte(data))
	return fmt.Sprintf("%x", hash)[:8]
}

// Helper functions for building alert data and determining priorities
func (am *AlertManager) determinePriority(candidate *ScanCandidate) AlertPriority {
	if candidate.Score.Score >= 90 {
		return AlertPriorityHigh
	}
	if candidate.Score.Score >= 80 {
		return AlertPriorityNormal
	}
	return AlertPriorityLow
}

func (am *AlertManager) determineExitPriority(exitData *ExitSignal) AlertPriority {
	if exitData.PnlPercent < -10 {
		return AlertPriorityHigh
	}
	if exitData.Cause == "hard_stop" {
		return AlertPriorityHigh
	}
	return AlertPriorityNormal
}

func (am *AlertManager) buildCandidateData(candidate *ScanCandidate) map[string]interface{} {
	return map[string]interface{}{
		"symbol":             candidate.Symbol,
		"composite_score":    fmt.Sprintf("%.1f", candidate.Score.Score),
		"top_factor":         am.getTopFactor(candidate),
		"decision":           candidate.Decision,
		"spread_bps":         fmt.Sprintf("%.1f", candidate.Microstructure.SpreadBps),
		"depth_usd":          candidate.Microstructure.DepthUsd,
		"vadr":               fmt.Sprintf("%.2f", candidate.Microstructure.VADR),
		"venue":              candidate.Microstructure.Venue,
		"freshness_badge":    am.getFreshnessBadge(candidate),
		"catalyst_bucket":    candidate.Catalyst.Bucket,
		"catalyst_multiplier": fmt.Sprintf("%.1f", candidate.Catalyst.Multiplier),
		"why_now":            am.generateWhyNow(candidate),
		"rank":               candidate.Score.Rank,
		"timestamp":          candidate.Timestamp.Format("15:04 UTC"),
	}
}

func (am *AlertManager) buildExitData(exitData *ExitSignal) map[string]interface{} {
	return map[string]interface{}{
		"symbol":        exitData.Symbol,
		"exit_cause":    exitData.Cause,
		"hold_duration": am.formatDuration(time.Since(exitData.EntryTime)),
		"pnl_percent":   fmt.Sprintf("%+.2f", exitData.PnlPercent),
		"entry_score":   fmt.Sprintf("%.1f", exitData.EntryScore),
		"exit_score":    fmt.Sprintf("%.1f", exitData.ExitScore),
		"max_drawdown":  fmt.Sprintf("%.2f", exitData.MaxDrawdown),
		"peak_gain":     fmt.Sprintf("%.2f", exitData.PeakGain),
		"final_stats":   exitData.FinalStats,
		"timestamp":     exitData.ExitTime.Format("15:04 UTC"),
	}
}

func (am *AlertManager) getTopFactor(candidate *ScanCandidate) string {
	factors := map[string]float64{
		"Momentum": candidate.Factors.MomentumCore,
		"Volume":   candidate.Factors.Volume,
		"Social":   candidate.Factors.Social,
	}

	topFactor := "Momentum"
	maxValue := candidate.Factors.MomentumCore

	for factor, value := range factors {
		if value > maxValue {
			maxValue = value
			topFactor = factor
		}
	}

	return fmt.Sprintf("%s (%.1f)", topFactor, maxValue)
}

func (am *AlertManager) getFreshnessBadge(candidate *ScanCandidate) string {
	bars := candidate.Freshness.BarsSinceSignal
	if bars == 0 {
		return "🟢 Live"
	}
	if bars == 1 {
		return "🟡 1 bar"
	}
	return fmt.Sprintf("🟠 %d bars", bars)
}

func (am *AlertManager) generateWhyNow(candidate *ScanCandidate) string {
	reasons := []string{}
	
	if candidate.Score.Score >= 90 {
		reasons = append(reasons, "exceptional composite score")
	}
	
	if candidate.Freshness.BarsSinceSignal == 0 {
		reasons = append(reasons, "live signal")
	}
	
	if candidate.Microstructure.SpreadBps < 20 {
		reasons = append(reasons, "tight spread")
	}
	
	if candidate.Catalyst.Multiplier > 1.5 {
		reasons = append(reasons, "catalyst boost")
	}
	
	if len(reasons) == 0 {
		return "meets all entry criteria"
	}
	
	return strings.Join(reasons, " + ")
}

func (am *AlertManager) formatDuration(d time.Duration) string {
	if d < time.Hour {
		return fmt.Sprintf("%.0fm", d.Minutes())
	}
	return fmt.Sprintf("%.1fh", d.Hours())
}

// Supporting types for alerts (these would normally be defined elsewhere)
type ScanCandidate struct {
	Symbol         string
	Score          struct {
		Score float64
		Rank  int
	}
	Factors struct {
		MomentumCore float64
		Volume       float64
		Social       float64
	}
	Microstructure struct {
		SpreadBps float64
		DepthUsd  float64
		VADR      float64
		Venue     string
	}
	Freshness struct {
		BarsSinceSignal int
	}
	Catalyst struct {
		Bucket     string
		Multiplier float64
	}
	Decision  string
	Timestamp time.Time
}

type ExitSignal struct {
	Symbol      string
	Cause       string
	EntryTime   time.Time
	ExitTime    time.Time
	PnlPercent  float64
	EntryScore  float64
	ExitScore   float64
	MaxDrawdown float64
	PeakGain    float64
	FinalStats  string
}