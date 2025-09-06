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

// TelegramProvider implements AlertProvider for Telegram Bot API
type TelegramProvider struct {
	config *TelegramConfig
	client *http.Client
	apiURL string
}

// TelegramMessage represents Telegram bot message structure
type TelegramMessage struct {
	ChatID                string `json:"chat_id"`
	Text                  string `json:"text"`
	ParseMode             string `json:"parse_mode,omitempty"`
	DisableWebPagePreview bool   `json:"disable_web_page_preview,omitempty"`
}

// TelegramResponse represents Telegram API response
type TelegramResponse struct {
	OK     bool                   `json:"ok"`
	Result map[string]interface{} `json:"result,omitempty"`
	Error  *TelegramError         `json:"error,omitempty"`
}

// TelegramError represents Telegram API error
type TelegramError struct {
	Code        int    `json:"error_code"`
	Description string `json:"description"`
}

// NewTelegramProvider creates a new Telegram alert provider
func NewTelegramProvider(config *TelegramConfig) *TelegramProvider {
	apiURL := fmt.Sprintf("https://api.telegram.org/bot%s", config.BotToken)
	
	return &TelegramProvider{
		config: config,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		apiURL: apiURL,
	}
}

// Name returns the provider name
func (tp *TelegramProvider) Name() string {
	return "telegram"
}

// IsEnabled returns whether the provider is enabled
func (tp *TelegramProvider) IsEnabled() bool {
	return tp.config.Enabled
}

// ValidateConfig validates the Telegram configuration
func (tp *TelegramProvider) ValidateConfig() error {
	if tp.config.BotToken == "" {
		return fmt.Errorf("telegram bot token is required")
	}
	
	if tp.config.ChatID == "" {
		return fmt.Errorf("telegram chat ID is required")
	}

	// Validate bot token format (should start with digits followed by colon)
	parts := strings.Split(tp.config.BotToken, ":")
	if len(parts) != 2 || len(parts[0]) < 8 {
		return fmt.Errorf("invalid telegram bot token format")
	}

	return nil
}

// SendAlert sends an alert to Telegram
func (tp *TelegramProvider) SendAlert(event *AlertEvent, message string) error {
	// Format message for Telegram (convert to MarkdownV2)
	telegramMessage := tp.formatForTelegram(event, message)
	
	payload := TelegramMessage{
		ChatID:                tp.config.ChatID,
		Text:                  telegramMessage,
		ParseMode:             "MarkdownV2",
		DisableWebPagePreview: true,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal Telegram payload: %w", err)
	}

	url := fmt.Sprintf("%s/sendMessage", tp.apiURL)
	resp, err := tp.client.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to send Telegram message: %w", err)
	}
	defer resp.Body.Close()

	var telegramResp TelegramResponse
	if err := json.NewDecoder(resp.Body).Decode(&telegramResp); err != nil {
		return fmt.Errorf("failed to decode Telegram response: %w", err)
	}

	if !telegramResp.OK {
		errorDesc := "unknown error"
		if telegramResp.Error != nil {
			errorDesc = telegramResp.Error.Description
		}
		return fmt.Errorf("telegram API error: %s", errorDesc)
	}

	log.Debug().
		Str("symbol", event.Symbol).
		Str("type", string(event.Type)).
		Str("chat_id", tp.config.ChatID).
		Msg("Telegram alert sent successfully")

	return nil
}

// formatForTelegram converts message to Telegram MarkdownV2 format
func (tp *TelegramProvider) formatForTelegram(event *AlertEvent, message string) string {
	// Convert basic markdown to Telegram MarkdownV2
	telegramMsg := message

	// Escape special characters for MarkdownV2
	telegramMsg = tp.escapeMarkdownV2(telegramMsg)

	// Add emoji-based priority indicator
	priority := tp.getPriorityEmoji(event.Priority)
	
	// Add header with priority
	header := fmt.Sprintf("%s *CryptoRun Alert \\- %s*\n\n", priority, tp.escapeText(string(event.Type)))
	
	return header + telegramMsg
}

// escapeMarkdownV2 escapes special characters for Telegram MarkdownV2
func (tp *TelegramProvider) escapeMarkdownV2(text string) string {
	// Characters that need escaping in MarkdownV2: _*[]()~`>#+-=|{}.!
	specialChars := []string{
		"_", "\\_", "*", "\\*", "[", "\\[", "]", "\\]", 
		"(", "\\(", ")", "\\)", "~", "\\~", "`", "\\`", 
		">", "\\>", "#", "\\#", "+", "\\+", "-", "\\-", 
		"=", "\\=", "|", "\\|", "{", "\\{", "}", "\\}", 
		".", "\\.", "!", "\\!",
	}
	
	replacer := strings.NewReplacer(specialChars...)
	return replacer.Replace(text)
}

// escapeText escapes text for safe inclusion in MarkdownV2
func (tp *TelegramProvider) escapeText(text string) string {
	return tp.escapeMarkdownV2(text)
}

// getPriorityEmoji returns emoji for alert priority
func (tp *TelegramProvider) getPriorityEmoji(priority AlertPriority) string {
	switch priority {
	case AlertPriorityCritical:
		return "🚨🚨🚨"
	case AlertPriorityHigh:
		return "🚨🚨"
	case AlertPriorityNormal:
		return "🚨"
	default:
		return "ℹ️"
	}
}

// GetBotInfo retrieves bot information for validation
func (tp *TelegramProvider) GetBotInfo() (map[string]interface{}, error) {
	url := fmt.Sprintf("%s/getMe", tp.apiURL)
	
	resp, err := tp.client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to get bot info: %w", err)
	}
	defer resp.Body.Close()

	var telegramResp TelegramResponse
	if err := json.NewDecoder(resp.Body).Decode(&telegramResp); err != nil {
		return nil, fmt.Errorf("failed to decode bot info response: %w", err)
	}

	if !telegramResp.OK {
		return nil, fmt.Errorf("telegram API error getting bot info")
	}

	return telegramResp.Result, nil
}

// TestConnection tests the Telegram bot connection
func (tp *TelegramProvider) TestConnection() error {
	botInfo, err := tp.GetBotInfo()
	if err != nil {
		return err
	}

	botUsername, ok := botInfo["username"].(string)
	if !ok {
		return fmt.Errorf("unable to get bot username")
	}

	log.Info().
		Str("bot_username", botUsername).
		Str("chat_id", tp.config.ChatID).
		Msg("Telegram bot connection verified")

	return nil
}