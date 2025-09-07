package providers

import (
	"context"
	"errors"
	"sync"
	"time"
)

// Capability represents different data provider capabilities
type Capability string

const (
	CapabilityFunding       Capability = "funding"        // Perpetuals funding history/current
	CapabilitySpotTrades    Capability = "spot_trades"    // Spot market trades
	CapabilityOrderBookL2   Capability = "orderbook_l2"   // Order book L2 data
	CapabilityKlineData     Capability = "kline_data"     // Kline/candle data
	CapabilitySupplyReserves Capability = "supply_reserves" // Supply/reserves (on-chain proxy)
	CapabilityWhaleDetection Capability = "whale_detection" // Large print detection
	CapabilityCVD           Capability = "cvd"            // Cumulative volume delta
)

// Common errors
var (
	ErrCapabilityNotSupported = errors.New("capability not supported by provider")
	ErrProviderNotFound      = errors.New("no provider found for capability")
	ErrProviderUnavailable   = errors.New("provider currently unavailable")
)

// Provenance tracks data source information for transparency
type Provenance struct {
	Venue     string `json:"venue"`      // Provider name (binance, okx, etc.)
	Endpoint  string `json:"endpoint"`   // API endpoint used
	Window    int    `json:"window"`     // Data window/limit if applicable
	LatencyMs int    `json:"latency_ms"` // Request latency in milliseconds
	Timestamp time.Time `json:"timestamp"` // When data was fetched
}

// Provider interface defines the contract all providers must implement
type Provider interface {
	Name() string
	HasCapability(cap Capability) bool
	Probe(ctx context.Context) (*ProbeResult, error)
	
	// Funding capability
	GetFundingHistory(ctx context.Context, req *FundingRequest) (*FundingResponse, error)
	
	// Spot trades capability  
	GetSpotTrades(ctx context.Context, req *SpotTradesRequest) (*SpotTradesResponse, error)
	
	// Order book L2 capability
	GetOrderBookL2(ctx context.Context, req *OrderBookRequest) (*OrderBookResponse, error)
	
	// Kline data capability
	GetKlineData(ctx context.Context, req *KlineRequest) (*KlineResponse, error)
	
	// Supply/reserves capability (on-chain proxy)
	GetSupplyReserves(ctx context.Context, req *SupplyRequest) (*SupplyResponse, error)
	
	// Whale detection capability
	GetWhaleDetection(ctx context.Context, req *WhaleRequest) (*WhaleResponse, error)
	
	// CVD capability
	GetCVD(ctx context.Context, req *CVDRequest) (*CVDResponse, error)
}

// Request/Response types for each capability

// Funding types
type FundingRequest struct {
	Symbol string `json:"symbol"`
	Limit  int    `json:"limit"`
}

type FundingRate struct {
	Symbol    string    `json:"symbol"`
	Rate      float64   `json:"rate"`
	Timestamp time.Time `json:"timestamp"`
	MarkPrice float64   `json:"mark_price,omitempty"`
}

type FundingResponse struct {
	Data       []FundingRate `json:"data"`
	Provenance Provenance    `json:"provenance"`
}

// Spot trades types
type SpotTradesRequest struct {
	Symbol string `json:"symbol"`
	Limit  int    `json:"limit"`
}

type SpotTrade struct {
	Symbol    string    `json:"symbol"`
	Price     float64   `json:"price"`
	Volume    float64   `json:"volume"`
	Side      string    `json:"side"` // "buy" or "sell"
	Timestamp time.Time `json:"timestamp"`
	TradeID   string    `json:"trade_id,omitempty"`
}

type SpotTradesResponse struct {
	Data       []SpotTrade `json:"data"`
	Provenance Provenance  `json:"provenance"`
}

// Order book types
type OrderBookRequest struct {
	Symbol string `json:"symbol"`
	Limit  int    `json:"limit"`
}

type OrderBookEntry struct {
	Price float64 `json:"price"`
	Size  float64 `json:"size"`
}

type OrderBookL2 struct {
	Symbol    string           `json:"symbol"`
	Bids      []OrderBookEntry `json:"bids"`
	Asks      []OrderBookEntry `json:"asks"`
	Timestamp time.Time        `json:"timestamp"`
}

type OrderBookResponse struct {
	Data       *OrderBookL2 `json:"data"`
	Provenance Provenance   `json:"provenance"`
}

// Kline types
type KlineRequest struct {
	Symbol   string `json:"symbol"`
	Interval string `json:"interval"` // "1m", "5m", "1h", "1d", etc.
	Limit    int    `json:"limit"`
}

type Kline struct {
	Symbol    string    `json:"symbol"`
	Interval  string    `json:"interval"`
	OpenTime  time.Time `json:"open_time"`
	CloseTime time.Time `json:"close_time"`
	Open      float64   `json:"open"`
	High      float64   `json:"high"`
	Low       float64   `json:"low"`
	Close     float64   `json:"close"`
	Volume    float64   `json:"volume"`
	QuoteVolume float64 `json:"quote_volume,omitempty"`
}

type KlineResponse struct {
	Data       []Kline    `json:"data"`
	Provenance Provenance `json:"provenance"`
}

// Supply/reserves types
type SupplyRequest struct {
	Symbol string `json:"symbol"`
}

type SupplyData struct {
	Symbol         string    `json:"symbol"`
	CirculatingSupply float64 `json:"circulating_supply"`
	TotalSupply    float64   `json:"total_supply,omitempty"`
	MaxSupply      float64   `json:"max_supply,omitempty"`
	Timestamp      time.Time `json:"timestamp"`
}

type SupplyResponse struct {
	Data       *SupplyData `json:"data"`
	Provenance Provenance  `json:"provenance"`
}

// Whale detection types
type WhaleRequest struct {
	Symbol    string `json:"symbol"`
	Threshold float64 `json:"threshold"` // Minimum USD value to consider "whale"
	Limit     int     `json:"limit"`
}

type WhaleEvent struct {
	Symbol    string    `json:"symbol"`
	Price     float64   `json:"price"`
	Volume    float64   `json:"volume"`
	Value     float64   `json:"value"` // USD value
	Side      string    `json:"side"`  // "buy" or "sell"
	Timestamp time.Time `json:"timestamp"`
}

type WhaleResponse struct {
	Data       []WhaleEvent `json:"data"`
	Provenance Provenance   `json:"provenance"`
}

// CVD (Cumulative Volume Delta) types
type CVDRequest struct {
	Symbol   string `json:"symbol"`
	Interval string `json:"interval"` // Time interval for CVD calculation
	Limit    int    `json:"limit"`
}

type CVDData struct {
	Symbol     string    `json:"symbol"`
	Interval   string    `json:"interval"`
	CVD        float64   `json:"cvd"`        // Cumulative volume delta
	BuyVolume  float64   `json:"buy_volume"` // Total buy volume
	SellVolume float64   `json:"sell_volume"` // Total sell volume
	Timestamp  time.Time `json:"timestamp"`
}

type CVDResponse struct {
	Data       []CVDData  `json:"data"`
	Provenance Provenance `json:"provenance"`
}

// Probe result for health checks
type ProbeResult struct {
	Success   bool      `json:"success"`
	LatencyMs int       `json:"latency_ms"`
	Error     string    `json:"error,omitempty"`
	Timestamp time.Time `json:"timestamp"`
}

// ProviderRegistry manages multiple providers with capabilities and fallback chains
type ProviderRegistry struct {
	providers    []Provider
	capabilities map[Capability][]Provider // Ordered list of providers per capability
	mu           sync.RWMutex
}

// NewProviderRegistry creates a new provider registry
func NewProviderRegistry() *ProviderRegistry {
	return &ProviderRegistry{
		providers:    make([]Provider, 0),
		capabilities: make(map[Capability][]Provider),
	}
}

// RegisterProvider adds a provider to the registry
func (r *ProviderRegistry) RegisterProvider(provider Provider) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	// Add to providers list
	r.providers = append(r.providers, provider)
	
	// Add to capability mappings
	capabilities := []Capability{
		CapabilityFunding, CapabilitySpotTrades, CapabilityOrderBookL2,
		CapabilityKlineData, CapabilitySupplyReserves, CapabilityWhaleDetection,
		CapabilityCVD,
	}
	
	for _, cap := range capabilities {
		if provider.HasCapability(cap) {
			r.capabilities[cap] = append(r.capabilities[cap], provider)
		}
	}
	
	return nil
}

// GetProviders returns providers for a specific capability in fallback order
func (r *ProviderRegistry) GetProviders(cap Capability) []Provider {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	providers := r.capabilities[cap]
	if len(providers) == 0 {
		return []Provider{}
	}
	
	// Return a copy to avoid concurrent modifications
	result := make([]Provider, len(providers))
	copy(result, providers)
	return result
}

// GetProvider returns the first available provider for a capability
func (r *ProviderRegistry) GetProvider(cap Capability) (Provider, error) {
	providers := r.GetProviders(cap)
	if len(providers) == 0 {
		return nil, ErrProviderNotFound
	}
	
	return providers[0], nil
}

// ProbeCapabilities tests all providers and returns a capability report
func (r *ProviderRegistry) ProbeCapabilities(ctx context.Context) (*CapabilityReport, error) {
	r.mu.RLock()
	providers := make([]Provider, len(r.providers))
	copy(providers, r.providers)
	r.mu.RUnlock()
	
	report := &CapabilityReport{
		Timestamp: time.Now(),
		Providers: make([]ProviderReport, len(providers)),
	}
	
	for i, provider := range providers {
		providerReport := ProviderReport{
			Name:         provider.Name(),
			Capabilities: make(map[string]CapabilityStatus),
		}
		
		// Test each capability
		capabilities := []Capability{
			CapabilityFunding, CapabilitySpotTrades, CapabilityOrderBookL2,
			CapabilityKlineData, CapabilitySupplyReserves, CapabilityWhaleDetection,
			CapabilityCVD,
		}
		
		for _, cap := range capabilities {
			status := CapabilityStatus{
				Supported: provider.HasCapability(cap),
			}
			
			if status.Supported {
				// Probe the capability
				probeStart := time.Now()
				result, err := provider.Probe(ctx)
				status.LatencyMs = int(time.Since(probeStart).Milliseconds())
				
				if err != nil {
					status.Available = false
					status.Error = err.Error()
				} else {
					status.Available = result.Success
					if !result.Success {
						status.Error = result.Error
					}
				}
			}
			
			providerReport.Capabilities[string(cap)] = status
		}
		
		report.Providers[i] = providerReport
	}
	
	return report, nil
}

// CapabilityReport contains the results of probing all providers
type CapabilityReport struct {
	Timestamp time.Time        `json:"timestamp"`
	Providers []ProviderReport `json:"providers"`
}

// ProviderReport contains capability status for a single provider
type ProviderReport struct {
	Name         string                       `json:"name"`
	Capabilities map[string]CapabilityStatus `json:"capabilities"`
}

// CapabilityStatus indicates whether a capability is supported and available
type CapabilityStatus struct {
	Supported bool   `json:"supported"`
	Available bool   `json:"available"`
	LatencyMs int    `json:"latency_ms"`
	Error     string `json:"error,omitempty"`
}