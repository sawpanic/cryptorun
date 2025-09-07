package perf

import (
	"fmt"
	"time"
)

// Alert represents a performance or portfolio alert
type Alert struct {
	Type        string                 `json:"type"`         // Alert type (performance, correlation, drawdown, etc.)
	Severity    string                 `json:"severity"`     // CRITICAL, WARNING, INFO
	Message     string                 `json:"message"`      // Human-readable alert message
	Timestamp   time.Time              `json:"timestamp"`    // Alert generation time
	Metric      string                 `json:"metric"`       // Metric that triggered alert
	Value       float64                `json:"value"`        // Actual metric value
	Threshold   float64                `json:"threshold"`    // Threshold that was breached
	Symbol      string                 `json:"symbol,omitempty"` // Symbol if position-specific
	Context     map[string]interface{} `json:"context"`      // Additional context data
}

// AlertManager manages performance and portfolio alerts
type AlertManager struct {
	config   PerfCalculatorConfig
	handlers []AlertHandler
}

// AlertHandler defines the interface for alert notification handlers
type AlertHandler interface {
	// SendAlert sends an alert notification
	SendAlert(alert Alert) error
	
	// GetHandlerType returns the handler type (slack, email, webhook, etc.)
	GetHandlerType() string
}

// NewAlertManager creates a new alert manager
func NewAlertManager(config PerfCalculatorConfig) *AlertManager {
	return &AlertManager{
		config:   config,
		handlers: make([]AlertHandler, 0),
	}
}

// AddHandler adds an alert handler
func (am *AlertManager) AddHandler(handler AlertHandler) {
	am.handlers = append(am.handlers, handler)
}

// CheckPerformanceAlerts checks performance metrics against thresholds
func (am *AlertManager) CheckPerformanceAlerts(metrics *PerfMetrics) []Alert {
	alerts := make([]Alert, 0)
	
	// Check Sharpe ratio threshold
	if metrics.Sharpe < am.config.MinSharpeRatio {
		alert := Alert{
			Type:      "performance",
			Severity:  "WARNING",
			Message:   fmt.Sprintf("Sharpe ratio %.2f is below minimum threshold of %.2f", metrics.Sharpe, am.config.MinSharpeRatio),
			Timestamp: time.Now(),
			Metric:    "sharpe_ratio",
			Value:     metrics.Sharpe,
			Threshold: am.config.MinSharpeRatio,
			Context: map[string]interface{}{
				"analysis_period": fmt.Sprintf("%s to %s", metrics.StartDate.Format("2006-01-02"), metrics.EndDate.Format("2006-01-02")),
				"total_trades":    metrics.TotalTrades,
			},
		}
		alerts = append(alerts, alert)
	}
	
	// Check maximum drawdown threshold
	if metrics.MaxDrawdown > am.config.MaxDrawdown {
		severity := "WARNING"
		if metrics.MaxDrawdown > am.config.MaxDrawdown*1.5 { // 1.5x threshold = critical
			severity = "CRITICAL"
		}
		
		alert := Alert{
			Type:      "drawdown",
			Severity:  severity,
			Message:   fmt.Sprintf("Maximum drawdown %.2f%% exceeds threshold of %.2f%%", metrics.MaxDrawdown*100, am.config.MaxDrawdown*100),
			Timestamp: time.Now(),
			Metric:    "max_drawdown",
			Value:     metrics.MaxDrawdown,
			Threshold: am.config.MaxDrawdown,
			Context: map[string]interface{}{
				"drawdown_days": metrics.MaxDrawdownDays,
				"volatility":    metrics.Volatility,
			},
		}
		alerts = append(alerts, alert)
	}
	
	// Check hit rate degradation
	if metrics.HitRate < 0.40 {
		alert := Alert{
			Type:      "hit_rate",
			Severity:  "WARNING",
			Message:   fmt.Sprintf("Hit rate %.2f%% is critically low", metrics.HitRate*100),
			Timestamp: time.Now(),
			Metric:    "hit_rate",
			Value:     metrics.HitRate,
			Threshold: 0.40,
			Context: map[string]interface{}{
				"winning_trades": metrics.WinningTrades,
				"losing_trades":  metrics.LosingTrades,
				"total_trades":   metrics.TotalTrades,
			},
		}
		alerts = append(alerts, alert)
	}
	
	// Check profit factor
	if metrics.ProfitFactor < 1.0 && metrics.ProfitFactor > 0 {
		alert := Alert{
			Type:      "profit_factor",
			Severity:  "CRITICAL",
			Message:   fmt.Sprintf("Profit factor %.2f indicates net losses", metrics.ProfitFactor),
			Timestamp: time.Now(),
			Metric:    "profit_factor",
			Value:     metrics.ProfitFactor,
			Threshold: 1.0,
			Context: map[string]interface{}{
				"avg_win":  metrics.AvgWin,
				"avg_loss": metrics.AvgLoss,
			},
		}
		alerts = append(alerts, alert)
	}
	
	// Check excessive volatility
	if metrics.Volatility > 0.80 { // 80% annual volatility threshold
		alert := Alert{
			Type:      "volatility",
			Severity:  "WARNING",
			Message:   fmt.Sprintf("Portfolio volatility %.2f%% is excessive", metrics.Volatility*100),
			Timestamp: time.Now(),
			Metric:    "volatility",
			Value:     metrics.Volatility,
			Threshold: 0.80,
			Context: map[string]interface{}{
				"sharpe_ratio":   metrics.Sharpe,
				"downside_vol":   metrics.DownsideVol,
			},
		}
		alerts = append(alerts, alert)
	}
	
	return alerts
}

// CheckPortfolioAlerts checks portfolio metrics against thresholds
func (am *AlertManager) CheckPortfolioAlerts(portfolio *PortfolioMetrics) []Alert {
	alerts := make([]Alert, 0)
	
	// Check concentration risk
	if portfolio.ConcentrationRisk > 0.50 {
		severity := "WARNING"
		if portfolio.ConcentrationRisk > 0.70 {
			severity = "CRITICAL"
		}
		
		alert := Alert{
			Type:      "concentration",
			Severity:  severity,
			Message:   fmt.Sprintf("Portfolio concentration risk %.3f is high (threshold: 0.50)", portfolio.ConcentrationRisk),
			Timestamp: time.Now(),
			Metric:    "concentration_risk",
			Value:     portfolio.ConcentrationRisk,
			Threshold: 0.50,
			Context: map[string]interface{}{
				"total_positions": len(portfolio.Positions),
				"total_value":     portfolio.TotalValue,
			},
		}
		alerts = append(alerts, alert)
	}
	
	// Check pairwise correlations
	for symbol1, correlations := range portfolio.CorrelationMatrix {
		for symbol2, correlation := range correlations {
			if symbol1 != symbol2 && correlation > am.config.MaxCorrelation {
				alert := Alert{
					Type:      "correlation",
					Severity:  "WARNING",
					Message:   fmt.Sprintf("High correlation %.3f between %s and %s (threshold: %.3f)", correlation, symbol1, symbol2, am.config.MaxCorrelation),
					Timestamp: time.Now(),
					Metric:    "pairwise_correlation",
					Value:     correlation,
					Threshold: am.config.MaxCorrelation,
					Context: map[string]interface{}{
						"symbol1": symbol1,
						"symbol2": symbol2,
					},
				}
				alerts = append(alerts, alert)
			}
		}
	}
	
	// Check sector concentration
	for sector, allocation := range portfolio.SectorAllocation {
		if allocation > 0.40 { // 40% sector limit
			alert := Alert{
				Type:      "sector_concentration",
				Severity:  "WARNING",
				Message:   fmt.Sprintf("High sector concentration: %s %.1f%% (limit: 40%%)", sector, allocation*100),
				Timestamp: time.Now(),
				Metric:    "sector_allocation",
				Value:     allocation,
				Threshold: 0.40,
				Symbol:    sector, // Using Symbol field for sector name
				Context: map[string]interface{}{
					"sector":         sector,
					"recommendation": "Diversify into other sectors",
				},
			}
			alerts = append(alerts, alert)
		}
	}
	
	// Check VaR relative to portfolio size
	varPercent := portfolio.PortfolioVaR / portfolio.TotalValue
	if varPercent > 0.15 { // 15% VaR threshold
		alert := Alert{
			Type:      "var_risk",
			Severity:  "WARNING",
			Message:   fmt.Sprintf("Portfolio VaR %.1f%% exceeds 15%% of portfolio value", varPercent*100),
			Timestamp: time.Now(),
			Metric:    "portfolio_var_percent",
			Value:     varPercent,
			Threshold: 0.15,
			Context: map[string]interface{}{
				"var_absolute":   portfolio.PortfolioVaR,
				"portfolio_size": portfolio.TotalValue,
			},
		}
		alerts = append(alerts, alert)
	}
	
	// Check exposure ratios
	grossExposureRatio := portfolio.GrossExposure / portfolio.TotalValue
	if grossExposureRatio > 2.0 { // 200% gross exposure limit
		alert := Alert{
			Type:      "exposure",
			Severity:  "WARNING",
			Message:   fmt.Sprintf("Gross exposure %.1fx exceeds 2.0x leverage limit", grossExposureRatio),
			Timestamp: time.Now(),
			Metric:    "gross_exposure_ratio",
			Value:     grossExposureRatio,
			Threshold: 2.0,
			Context: map[string]interface{}{
				"long_exposure":  portfolio.LongExposure,
				"short_exposure": portfolio.ShortExposure,
				"net_exposure":   portfolio.NetExposure,
			},
		}
		alerts = append(alerts, alert)
	}
	
	return alerts
}

// SendAlerts sends alerts through all configured handlers
func (am *AlertManager) SendAlerts(alerts []Alert) error {
	if len(alerts) == 0 {
		return nil
	}
	
	var errors []error
	
	for _, alert := range alerts {
		for _, handler := range am.handlers {
			if err := handler.SendAlert(alert); err != nil {
				errors = append(errors, fmt.Errorf("handler %s failed: %v", handler.GetHandlerType(), err))
			}
		}
	}
	
	if len(errors) > 0 {
		return fmt.Errorf("alert delivery errors: %v", errors)
	}
	
	return nil
}

// LogHandler logs alerts to stdout (default handler)
type LogHandler struct{}

// SendAlert logs the alert
func (h *LogHandler) SendAlert(alert Alert) error {
	fmt.Printf("[%s] %s ALERT: %s\n", alert.Timestamp.Format("2006-01-02 15:04:05"), alert.Severity, alert.Message)
	return nil
}

// GetHandlerType returns handler type
func (h *LogHandler) GetHandlerType() string {
	return "log"
}

// WebhookHandler sends alerts via webhook
type WebhookHandler struct {
	WebhookURL string
	Timeout    time.Duration
}

// SendAlert sends alert via webhook
func (h *WebhookHandler) SendAlert(alert Alert) error {
	// Implementation would send HTTP POST to webhook URL
	// Simplified for now
	fmt.Printf("WEBHOOK [%s]: %s\n", h.WebhookURL, alert.Message)
	return nil
}

// GetHandlerType returns handler type
func (h *WebhookHandler) GetHandlerType() string {
	return "webhook"
}

// SlackHandler sends alerts to Slack
type SlackHandler struct {
	WebhookURL string
	Channel    string
	Username   string
}

// SendAlert sends alert to Slack
func (h *SlackHandler) SendAlert(alert Alert) error {
	// Implementation would format and send to Slack webhook
	// Simplified for now
	emoji := "⚠️"
	if alert.Severity == "CRITICAL" {
		emoji = "🚨"
	}
	
	fmt.Printf("SLACK [%s]: %s %s - %s\n", h.Channel, emoji, alert.Severity, alert.Message)
	return nil
}

// GetHandlerType returns handler type
func (h *SlackHandler) GetHandlerType() string {
	return "slack"
}

// EmailHandler sends alerts via email
type EmailHandler struct {
	SMTPServer   string
	SMTPPort     int
	Username     string
	Password     string
	FromAddress  string
	ToAddresses  []string
}

// SendAlert sends alert via email
func (h *EmailHandler) SendAlert(alert Alert) error {
	// Implementation would send email via SMTP
	// Simplified for now
	fmt.Printf("EMAIL [%s]: %s Alert - %s\n", h.ToAddresses[0], alert.Severity, alert.Message)
	return nil
}

// GetHandlerType returns handler type
func (h *EmailHandler) GetHandlerType() string {
	return "email"
}

// AlertSummary contains summary statistics for alerts
type AlertSummary struct {
	TotalAlerts     int                   `json:"total_alerts"`
	BySeverity      map[string]int        `json:"by_severity"`
	ByType          map[string]int        `json:"by_type"`
	RecentAlerts    []Alert              `json:"recent_alerts"`
	TopAlerts       []Alert              `json:"top_alerts"`        // Most critical alerts
	GeneratedAt     time.Time            `json:"generated_at"`
}

// SummarizeAlerts creates an alert summary
func SummarizeAlerts(alerts []Alert) AlertSummary {
	summary := AlertSummary{
		TotalAlerts:  len(alerts),
		BySeverity:   make(map[string]int),
		ByType:       make(map[string]int),
		RecentAlerts: make([]Alert, 0),
		TopAlerts:    make([]Alert, 0),
		GeneratedAt:  time.Now(),
	}
	
	// Count by severity and type
	for _, alert := range alerts {
		summary.BySeverity[alert.Severity]++
		summary.ByType[alert.Type]++
	}
	
	// Get recent alerts (last 10)
	if len(alerts) > 10 {
		summary.RecentAlerts = alerts[len(alerts)-10:]
	} else {
		summary.RecentAlerts = alerts
	}
	
	// Get top alerts (critical first, then warnings)
	criticalAlerts := make([]Alert, 0)
	warningAlerts := make([]Alert, 0)
	
	for _, alert := range alerts {
		if alert.Severity == "CRITICAL" {
			criticalAlerts = append(criticalAlerts, alert)
		} else if alert.Severity == "WARNING" {
			warningAlerts = append(warningAlerts, alert)
		}
	}
	
	// Add critical alerts first
	for _, alert := range criticalAlerts {
		if len(summary.TopAlerts) < 5 {
			summary.TopAlerts = append(summary.TopAlerts, alert)
		}
	}
	
	// Fill with warnings if needed
	for _, alert := range warningAlerts {
		if len(summary.TopAlerts) < 5 {
			summary.TopAlerts = append(summary.TopAlerts, alert)
		}
	}
	
	return summary
}