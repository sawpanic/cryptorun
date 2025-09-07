package application

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
)

// DiscordProvider implements AlertProvider for Discord webhooks
type DiscordProvider struct {
	config *DiscordConfig
	client *http.Client
}

// DiscordWebhookPayload represents Discord webhook message structure
type DiscordWebhookPayload struct {
	Username  string         `json:"username,omitempty"`
	AvatarURL string         `json:"avatar_url,omitempty"`
	Content   string         `json:"content,omitempty"`
	Embeds    []DiscordEmbed `json:"embeds,omitempty"`
}

// DiscordEmbed represents Discord embed structure
type DiscordEmbed struct {
	Title       string              `json:"title,omitempty"`
	Description string              `json:"description,omitempty"`
	Color       int                 `json:"color,omitempty"`
	Fields      []DiscordEmbedField `json:"fields,omitempty"`
	Footer      *DiscordEmbedFooter `json:"footer,omitempty"`
	Timestamp   string              `json:"timestamp,omitempty"`
}

// DiscordEmbedField represents Discord embed field
type DiscordEmbedField struct {
	Name   string `json:"name"`
	Value  string `json:"value"`
	Inline bool   `json:"inline"`
}

// DiscordEmbedFooter represents Discord embed footer
type DiscordEmbedFooter struct {
	Text    string `json:"text,omitempty"`
	IconURL string `json:"icon_url,omitempty"`
}

// NewDiscordProvider creates a new Discord alert provider
func NewDiscordProvider(config *DiscordConfig) *DiscordProvider {
	return &DiscordProvider{
		config: config,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Name returns the provider name
func (dp *DiscordProvider) Name() string {
	return "discord"
}

// IsEnabled returns whether the provider is enabled
func (dp *DiscordProvider) IsEnabled() bool {
	return dp.config.Enabled
}

// ValidateConfig validates the Discord configuration
func (dp *DiscordProvider) ValidateConfig() error {
	if dp.config.WebhookURL == "" {
		return fmt.Errorf("discord webhook URL is required")
	}
	
	if !strings.HasPrefix(dp.config.WebhookURL, "https://discord.com/api/webhooks/") &&
	   !strings.HasPrefix(dp.config.WebhookURL, "https://discordapp.com/api/webhooks/") {
		return fmt.Errorf("invalid discord webhook URL format")
	}

	return nil
}

// SendAlert sends an alert to Discord
func (dp *DiscordProvider) SendAlert(event *AlertEvent, message string) error {
	embed := dp.createEmbed(event, message)
	
	payload := DiscordWebhookPayload{
		Username:  dp.config.Username,
		AvatarURL: dp.config.AvatarURL,
		Embeds:    []DiscordEmbed{embed},
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal Discord payload: %w", err)
	}

	resp, err := dp.client.Post(dp.config.WebhookURL, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to send Discord webhook: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("discord webhook returned status %d", resp.StatusCode)
	}

	log.Debug().
		Str("symbol", event.Symbol).
		Str("type", string(event.Type)).
		Int("status", resp.StatusCode).
		Msg("Discord alert sent successfully")

	return nil
}

// createEmbed creates a Discord embed from alert event
func (dp *DiscordProvider) createEmbed(event *AlertEvent, message string) DiscordEmbed {
	embed := DiscordEmbed{
		Timestamp: event.Timestamp.Format(time.RFC3339),
		Footer: &DiscordEmbedFooter{
			Text: "CryptoRun v3.2.1 Scanner",
		},
	}

	// Set color based on alert type and priority
	embed.Color = dp.getEmbedColor(event)

	switch event.Type {
	case AlertTypeEntry:
		embed.Title = fmt.Sprintf("🚨 NEW ENTRY SIGNAL: %s", event.Symbol)
		embed = dp.populateEntryEmbed(embed, event, message)
	case AlertTypeExit:
		embed.Title = fmt.Sprintf("⛔ EXIT SIGNAL: %s", event.Symbol)
		embed = dp.populateExitEmbed(embed, event, message)
	}

	return embed
}

// populateEntryEmbed populates entry-specific embed fields
func (dp *DiscordProvider) populateEntryEmbed(embed DiscordEmbed, event *AlertEvent, message string) DiscordEmbed {
	data := event.Data

	// Parse microstructure data from message for clean formatting
	microLines := dp.extractCodeBlock(message)
	
	embed.Fields = []DiscordEmbedField{
		{
			Name:   "📊 Score & Rank",
			Value:  fmt.Sprintf("**Score:** %s\n**Rank:** #%v", data["composite_score"], data["rank"]),
			Inline: true,
		},
		{
			Name:   "⚡ Freshness",
			Value:  fmt.Sprintf("%s", data["freshness_badge"]),
			Inline: true,
		},
		{
			Name:   "🎯 Top Factor",
			Value:  fmt.Sprintf("%s", data["top_factor"]),
			Inline: true,
		},
		{
			Name:   "📈 Microstructure",
			Value:  fmt.Sprintf("```\n%s\n```", microLines),
			Inline: false,
		},
		{
			Name:   "🔥 Catalyst",
			Value:  fmt.Sprintf("%s (x%s)", data["catalyst_bucket"], data["catalyst_multiplier"]),
			Inline: true,
		},
		{
			Name:   "💡 Why Now",
			Value:  fmt.Sprintf("%s", data["why_now"]),
			Inline: false,
		},
	}

	return embed
}

// populateExitEmbed populates exit-specific embed fields
func (dp *DiscordProvider) populateExitEmbed(embed DiscordEmbed, event *AlertEvent, message string) DiscordEmbed {
	data := event.Data
	
	pnlColor := "🔴" // Default red for losses
	if pnl, ok := data["pnl_percent"].(string); ok && strings.HasPrefix(pnl, "+") {
		pnlColor = "🟢"
	}

	embed.Fields = []DiscordEmbedField{
		{
			Name:   fmt.Sprintf("%s P&L", pnlColor),
			Value:  fmt.Sprintf("**%s%%**", data["pnl_percent"]),
			Inline: true,
		},
		{
			Name:   "⏱️ Hold Time",
			Value:  fmt.Sprintf("%s", data["hold_duration"]),
			Inline: true,
		},
		{
			Name:   "🚪 Exit Cause",
			Value:  fmt.Sprintf("%s", data["exit_cause"]),
			Inline: true,
		},
		{
			Name:   "📊 Performance Stats",
			Value:  fmt.Sprintf("**Entry:** %s\n**Exit:** %s\n**Peak:** +%s%%\n**Drawdown:** -%s%%", 
				data["entry_score"], data["exit_score"], data["peak_gain"], data["max_drawdown"]),
			Inline: false,
		},
	}

	return embed
}

// getEmbedColor returns appropriate color based on alert type and priority
func (dp *DiscordProvider) getEmbedColor(event *AlertEvent) int {
	switch event.Type {
	case AlertTypeEntry:
		switch event.Priority {
		case AlertPriorityCritical:
			return 0xFF0000 // Red
		case AlertPriorityHigh:
			return 0xFF6600 // Orange
		case AlertPriorityNormal:
			return 0x00FF00 // Green
		default:
			return 0x0099FF // Blue
		}
	case AlertTypeExit:
		// Check P&L for exit color
		if pnl, ok := event.Data["pnl_percent"].(string); ok {
			if strings.HasPrefix(pnl, "+") {
				return 0x00FF00 // Green for profit
			}
		}
		return 0xFF0000 // Red for loss
	default:
		return 0x808080 // Gray
	}
}

// extractCodeBlock extracts content between ``` markers
func (dp *DiscordProvider) extractCodeBlock(message string) string {
	lines := strings.Split(message, "\n")
	inCodeBlock := false
	var codeLines []string

	for _, line := range lines {
		if strings.HasPrefix(line, "```") {
			inCodeBlock = !inCodeBlock
			continue
		}
		if inCodeBlock {
			codeLines = append(codeLines, line)
		}
	}

	if len(codeLines) == 0 {
		return "No microstructure data"
	}

	return strings.Join(codeLines, "\n")
}