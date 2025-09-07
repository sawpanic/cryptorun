package kraken

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/sawpanic/cryptorun/internal/data/interfaces"
	"github.com/sawpanic/cryptorun/internal/providers/guards"
)

// Adapter implements the Exchange interface for Kraken
type Adapter struct {
	name       string
	baseURL    string
	httpClient *http.Client
	guard      *guards.ProviderGuard
	lastSeen   time.Time
	healthStats HealthStats
}

type HealthStats struct {
	RequestCount  int64
	ErrorCount    int64
	LastErrorTime time.Time
	AvgLatency    time.Duration
}

// KrakenResponse represents the standard Kraken API response wrapper
type KrakenResponse struct {
	Error  []string        `json:"error"`
	Result json.RawMessage `json:"result"`
}

// NewAdapter creates a new Kraken exchange adapter
func NewAdapter(name string) *Adapter {
	config := guards.ProviderConfig{
		Name:          name,
		SustainedRate: 1.0,  // Conservative 1 RPS for Kraken
		BurstLimit:    3,    // Small burst limit for Kraken
		TTLSeconds:    10,   // 10 seconds cache for Kraken's slower updates
		MaxRetries:    2,
		FailureThresh: 0.2,  // 20% failure rate threshold
		WindowRequests: 50,
		ProbeInterval:  60,
	}

	return &Adapter{
		name:    name,
		baseURL: "https://api.kraken.com/0/public",
		httpClient: &http.Client{
			Timeout: 15 * time.Second, // Longer timeout for Kraken
		},
		guard:    guards.NewProviderGuard(config),
		lastSeen: time.Now(),
		healthStats: HealthStats{},
	}
}

// Name returns the exchange name
func (a *Adapter) Name() string {
	return a.name
}

// ConnectWS is not implemented for REST-only adapter
func (a *Adapter) ConnectWS(ctx context.Context) error {
	return fmt.Errorf("WebSocket not supported in REST-only mode")
}

// SubscribeTrades is not implemented for REST-only adapter
func (a *Adapter) SubscribeTrades(symbol string, callback interfaces.TradesCallback) error {
	return fmt.Errorf("WebSocket subscriptions not supported in REST-only mode")
}

// SubscribeBookL2 is not implemented for REST-only adapter
func (a *Adapter) SubscribeBookL2(symbol string, callback interfaces.BookL2Callback) error {
	return fmt.Errorf("WebSocket subscriptions not supported in REST-only mode")
}

// StreamKlines is not implemented for REST-only adapter
func (a *Adapter) StreamKlines(symbol string, interval string, callback interfaces.KlinesCallback) error {
	return fmt.Errorf("WebSocket subscriptions not supported in REST-only mode")
}

// NormalizeSymbol converts symbols to Kraken format (e.g., BTCUSD -> XXBTZUSD)
func (a *Adapter) NormalizeSymbol(symbol string) string {
	symbol = strings.ToUpper(symbol)
	
	// Kraken symbol mapping
	symbolMap := map[string]string{
		"BTCUSD":  "XXBTZUSD",
		"ETHUSD":  "XETHZUSD", 
		"SOLUSD":  "SOLUSD",
		"ADAUSD":  "ADAUSD",
		"LINKUSD": "LINKUSD",
		"DOTUSD":  "DOTUSD",
		"MATICUSD": "MATICUSD",
		"AVAXUSD": "AVAXUSD",
		"UNIUSD":  "UNIUSD",
		"LTCUSD":  "XLTCZUSD",
		"XRPUSD":  "XXRPZUSD",
	}
	
	if krakenSymbol, exists := symbolMap[symbol]; exists {
		return krakenSymbol
	}
	
	return symbol
}

// NormalizeInterval converts time intervals to Kraken format (in minutes)
func (a *Adapter) NormalizeInterval(interval string) string {
	// Kraken intervals: 1, 5, 15, 30, 60, 240, 1440, 10080, 21600 (in minutes)
	switch strings.ToLower(interval) {
	case "1min", "1m":
		return "1"
	case "5min", "5m":
		return "5"
	case "15min", "15m":
		return "15"
	case "30min", "30m":
		return "30"
	case "1hour", "1h", "60m":
		return "60"
	case "4hour", "4h":
		return "240"
	case "1day", "1d", "24h":
		return "1440"
	default:
		return "60" // Default to 1 hour
	}
}

// Health returns the current health status
func (a *Adapter) Health() interfaces.HealthStatus {
	status := "healthy"
	recommendation := "use_primary"
	
	// Calculate error rate
	errorRate := float64(0)
	if a.healthStats.RequestCount > 0 {
		errorRate = float64(a.healthStats.ErrorCount) / float64(a.healthStats.RequestCount)
	}
	
	// Check if unhealthy
	if errorRate > 0.1 || time.Since(a.lastSeen) > 10*time.Minute {
		status = "degraded"
		recommendation = "use_fallback"
	}
	
	if errorRate > 0.5 || time.Since(a.lastSeen) > 30*time.Minute {
		status = "unhealthy"
		recommendation = "avoid"
	}
	
	return interfaces.HealthStatus{
		Venue:          a.name,
		Status:         status,
		LastSeen:       a.lastSeen,
		ErrorRate:      errorRate,
		P99Latency:     a.healthStats.AvgLatency,
		WSConnected:    false,
		RESTHealthy:    status == "healthy",
		Recommendation: recommendation,
	}
}

// GetKlines, GetTrades, GetBookL2 - Stub implementations for now
func (a *Adapter) GetKlines(ctx context.Context, symbol string, interval string, limit int) ([]interfaces.Kline, error) {
	return nil, fmt.Errorf("GetKlines not yet implemented for Kraken")
}

func (a *Adapter) GetTrades(ctx context.Context, symbol string, limit int) ([]interfaces.Trade, error) {
	return nil, fmt.Errorf("GetTrades not implemented for Kraken REST adapter")
}

func (a *Adapter) GetBookL2(ctx context.Context, symbol string) (*interfaces.BookL2, error) {
	return nil, fmt.Errorf("GetBookL2 not yet implemented for Kraken")
}
