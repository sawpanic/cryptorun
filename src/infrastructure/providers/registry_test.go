package providers

import (
	"context"
	"testing"
	"time"
)

func TestProviderRegistry_RegisterProvider(t *testing.T) {
	registry := NewProviderRegistry()
	
	mockProvider := &MockProvider{
		name: "test-provider",
		capabilities: map[Capability]bool{
			CapabilityFunding:   true,
			CapabilitySpotTrades: true,
		},
	}
	
	err := registry.RegisterProvider(mockProvider)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	
	providers := registry.GetProviders(CapabilityFunding)
	if len(providers) != 1 {
		t.Errorf("Expected 1 provider, got %d", len(providers))
	}
	
	if providers[0].Name() != "test-provider" {
		t.Errorf("Expected 'test-provider', got %s", providers[0].Name())
	}
}

func TestProviderRegistry_GetProviders_WithFallback(t *testing.T) {
	registry := NewProviderRegistry()
	
	// Register primary provider
	primary := &MockProvider{
		name: "primary",
		capabilities: map[Capability]bool{CapabilitySpotTrades: true},
	}
	
	// Register fallback provider
	fallback := &MockProvider{
		name: "fallback", 
		capabilities: map[Capability]bool{CapabilitySpotTrades: true},
	}
	
	registry.RegisterProvider(primary)
	registry.RegisterProvider(fallback)
	
	providers := registry.GetProviders(CapabilitySpotTrades)
	if len(providers) != 2 {
		t.Errorf("Expected 2 providers, got %d", len(providers))
	}
}

func TestProviderRegistry_ProbeCapabilities(t *testing.T) {
	registry := NewProviderRegistry()
	
	provider := &MockProvider{
		name: "test",
		capabilities: map[Capability]bool{
			CapabilityFunding:    true,
			CapabilityOrderBookL2: true,
		},
		probeLatency: 50 * time.Millisecond,
	}
	
	registry.RegisterProvider(provider)
	
	ctx := context.Background()
	report, err := registry.ProbeCapabilities(ctx)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	
	if len(report.Providers) != 1 {
		t.Errorf("Expected 1 provider in report, got %d", len(report.Providers))
	}
	
	providerReport := report.Providers[0]
	if providerReport.Name != "test" {
		t.Errorf("Expected provider name 'test', got %s", providerReport.Name)
	}
	
	// Check that only supported capabilities are marked as available
	supportedCount := 0
	for _, status := range providerReport.Capabilities {
		if status.Supported {
			supportedCount++
		}
	}
	
	if supportedCount != 2 {
		t.Errorf("Expected 2 supported capabilities, got %d", supportedCount)
	}
}

// MockProvider for testing
type MockProvider struct {
	name         string
	capabilities map[Capability]bool
	probeLatency time.Duration
}

func (m *MockProvider) Name() string {
	return m.name
}

func (m *MockProvider) HasCapability(cap Capability) bool {
	return m.capabilities[cap]
}

func (m *MockProvider) GetFundingHistory(ctx context.Context, req *FundingRequest) (*FundingResponse, error) {
	if m.probeLatency > 0 {
		time.Sleep(m.probeLatency)
	}
	return &FundingResponse{
		Data: []FundingRate{{
			Symbol:    req.Symbol,
			Rate:      0.0001,
			Timestamp: time.Now(),
		}},
		Provenance: Provenance{
			Venue:     m.name,
			Endpoint:  "/test/funding",
			Window:    req.Limit,
			LatencyMs: int(m.probeLatency.Milliseconds()),
		},
	}, nil
}

func (m *MockProvider) GetSpotTrades(ctx context.Context, req *SpotTradesRequest) (*SpotTradesResponse, error) {
	if m.probeLatency > 0 {
		time.Sleep(m.probeLatency)
	}
	return &SpotTradesResponse{
		Data: []SpotTrade{{
			Symbol:    req.Symbol,
			Price:     50000.0,
			Volume:    1.5,
			Timestamp: time.Now(),
		}},
		Provenance: Provenance{
			Venue:     m.name,
			Endpoint:  "/test/trades",
			Window:    req.Limit,
			LatencyMs: int(m.probeLatency.Milliseconds()),
		},
	}, nil
}

func (m *MockProvider) GetOrderBookL2(ctx context.Context, req *OrderBookRequest) (*OrderBookResponse, error) {
	if m.probeLatency > 0 {
		time.Sleep(m.probeLatency)
	}
	return &OrderBookResponse{
		Data: &OrderBookL2{
			Symbol: req.Symbol,
			Bids: []OrderBookEntry{
				{Price: 49950.0, Size: 0.5},
				{Price: 49900.0, Size: 1.0},
			},
			Asks: []OrderBookEntry{
				{Price: 50050.0, Size: 0.3},
				{Price: 50100.0, Size: 0.8},
			},
			Timestamp: time.Now(),
		},
		Provenance: Provenance{
			Venue:     m.name,
			Endpoint:  "/test/depth",
			LatencyMs: int(m.probeLatency.Milliseconds()),
		},
	}, nil
}

func (m *MockProvider) GetKlineData(ctx context.Context, req *KlineRequest) (*KlineResponse, error) {
	if m.probeLatency > 0 {
		time.Sleep(m.probeLatency)
	}
	return &KlineResponse{
		Data: []Kline{{
			Symbol:    req.Symbol,
			Interval:  req.Interval,
			OpenTime:  time.Now().Add(-time.Hour),
			CloseTime: time.Now(),
			Open:      50000.0,
			High:      51000.0,
			Low:       49000.0,
			Close:     50500.0,
			Volume:    100.0,
		}},
		Provenance: Provenance{
			Venue:     m.name,
			Endpoint:  "/test/klines",
			Window:    req.Limit,
			LatencyMs: int(m.probeLatency.Milliseconds()),
		},
	}, nil
}

func (m *MockProvider) GetSupplyReserves(ctx context.Context, req *SupplyRequest) (*SupplyResponse, error) {
	return nil, ErrCapabilityNotSupported
}

func (m *MockProvider) GetWhaleDetection(ctx context.Context, req *WhaleRequest) (*WhaleResponse, error) {
	return nil, ErrCapabilityNotSupported
}

func (m *MockProvider) GetCVD(ctx context.Context, req *CVDRequest) (*CVDResponse, error) {
	return nil, ErrCapabilityNotSupported
}

func (m *MockProvider) Probe(ctx context.Context) (*ProbeResult, error) {
	if m.probeLatency > 0 {
		time.Sleep(m.probeLatency)
	}
	
	return &ProbeResult{
		Success:   true,
		LatencyMs: int(m.probeLatency.Milliseconds()),
		Timestamp: time.Now(),
	}, nil
}