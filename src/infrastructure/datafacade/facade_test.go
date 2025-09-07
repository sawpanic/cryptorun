package datafacade

import (
	"context"
	"testing"
	"time"

	"github.com/sawpanic/cryptorun/src/infrastructure/datafacade/interfaces"
)

// MockVenueAdapter implements VenueAdapter for testing
type MockVenueAdapter struct {
	venue string
}

func (m *MockVenueAdapter) GetVenue() string {
	return m.venue
}

func (m *MockVenueAdapter) StreamTrades(ctx context.Context, symbol string) (<-chan interfaces.TradeEvent, error) {
	ch := make(chan interfaces.TradeEvent)
	go func() {
		defer close(ch)
		select {
		case ch <- interfaces.TradeEvent{
			Trade: interfaces.Trade{
				ID:       "mock_1",
				Symbol:   symbol,
				Price:    50000.0,
				Quantity: 1.0,
				Side:     "buy",
				Venue:    m.venue,
			},
			EventTime: time.Now(),
		}:
		case <-ctx.Done():
		}
	}()
	return ch, nil
}

func (m *MockVenueAdapter) StreamKlines(ctx context.Context, symbol, interval string) (<-chan interfaces.KlineEvent, error) {
	ch := make(chan interfaces.KlineEvent)
	go func() {
		defer close(ch)
		select {
		case ch <- interfaces.KlineEvent{
			Kline: interfaces.Kline{
				Symbol:   symbol,
				Interval: interval,
				Open:     50000.0,
				High:     51000.0,
				Low:      49000.0,
				Close:    50500.0,
				Volume:   100.0,
				Venue:    m.venue,
			},
			EventTime: time.Now(),
		}:
		case <-ctx.Done():
		}
	}()
	return ch, nil
}

func (m *MockVenueAdapter) StreamOrderBook(ctx context.Context, symbol string, depth int) (<-chan interfaces.OrderBookEvent, error) {
	ch := make(chan interfaces.OrderBookEvent)
	go func() {
		defer close(ch)
		select {
		case ch <- interfaces.OrderBookEvent{
			OrderBook: interfaces.OrderBookSnapshot{
				Symbol:    symbol,
				Venue:     m.venue,
				Timestamp: time.Now(),
				Bids: []interfaces.OrderBookLevel{
					{Price: 50000.0, Quantity: 1.0},
				},
				Asks: []interfaces.OrderBookLevel{
					{Price: 50001.0, Quantity: 1.0},
				},
			},
			EventTime: time.Now(),
		}:
		case <-ctx.Done():
		}
	}()
	return ch, nil
}

func (m *MockVenueAdapter) StreamFunding(ctx context.Context, symbol string) (<-chan interfaces.FundingEvent, error) {
	ch := make(chan interfaces.FundingEvent)
	close(ch) // Mock venue doesn't support funding
	return ch, nil
}

func (m *MockVenueAdapter) StreamOpenInterest(ctx context.Context, symbol string) (<-chan interfaces.OpenInterestEvent, error) {
	ch := make(chan interfaces.OpenInterestEvent)
	close(ch) // Mock venue doesn't support open interest
	return ch, nil
}

func (m *MockVenueAdapter) GetTrades(ctx context.Context, symbol string, limit int) ([]interfaces.Trade, error) {
	return []interfaces.Trade{
		{
			ID:       "rest_1",
			Symbol:   symbol,
			Price:    50000.0,
			Quantity: 1.0,
			Side:     "buy",
			Venue:    m.venue,
		},
	}, nil
}

func (m *MockVenueAdapter) GetKlines(ctx context.Context, symbol, interval string, limit int) ([]interfaces.Kline, error) {
	return []interfaces.Kline{
		{
			Symbol:   symbol,
			Interval: interval,
			Open:     50000.0,
			High:     51000.0,
			Low:      49000.0,
			Close:    50500.0,
			Volume:   100.0,
			Venue:    m.venue,
		},
	}, nil
}

func (m *MockVenueAdapter) GetOrderBook(ctx context.Context, symbol string, depth int) (*interfaces.OrderBookSnapshot, error) {
	return &interfaces.OrderBookSnapshot{
		Symbol:    symbol,
		Venue:     m.venue,
		Timestamp: time.Now(),
		Bids: []interfaces.OrderBookLevel{
			{Price: 50000.0, Quantity: 1.0},
		},
		Asks: []interfaces.OrderBookLevel{
			{Price: 50001.0, Quantity: 1.0},
		},
	}, nil
}

func (m *MockVenueAdapter) GetFunding(ctx context.Context, symbol string) (*interfaces.FundingRate, error) {
	return nil, interfaces.ErrNotSupported
}

func (m *MockVenueAdapter) GetOpenInterest(ctx context.Context, symbol string) (*interfaces.OpenInterest, error) {
	return nil, interfaces.ErrNotSupported
}

// MockCacheLayer implements CacheLayer for testing
type MockCacheLayer struct {
	data map[string][]byte
}

func NewMockCacheLayer() *MockCacheLayer {
	return &MockCacheLayer{
		data: make(map[string][]byte),
	}
}

func (m *MockCacheLayer) Get(ctx context.Context, key string) ([]byte, bool, error) {
	value, exists := m.data[key]
	return value, exists, nil
}

func (m *MockCacheLayer) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	m.data[key] = value
	return nil
}

func (m *MockCacheLayer) Delete(ctx context.Context, key string) error {
	delete(m.data, key)
	return nil
}

func (m *MockCacheLayer) Clear(ctx context.Context, pattern string) error {
	// Simple implementation: clear all keys (pattern matching not implemented)
	m.data = make(map[string][]byte)
	return nil
}

func (m *MockCacheLayer) GetStats(ctx context.Context) (*interfaces.CacheStats, error) {
	return &interfaces.CacheStats{
		Hits:      10,
		Misses:    2,
		Sets:      12,
		Deletes:   1,
		HitRate:   0.83,
		Size:      1024,
		ItemCount: len(m.data),
		AvgTTL:    300,
	}, nil
}

func (m *MockCacheLayer) GetHitRate(ctx context.Context) float64 {
	return 0.83
}

func (m *MockCacheLayer) BuildKey(dataType, venue, symbol string, params ...string) string {
	key := venue + ":" + symbol + ":" + dataType
	for _, param := range params {
		key += ":" + param
	}
	return key
}

func (m *MockCacheLayer) CacheTrades(ctx context.Context, venue, symbol string, trades []interfaces.Trade, ttl time.Duration) error {
	return nil
}

func (m *MockCacheLayer) GetCachedTrades(ctx context.Context, venue, symbol string) ([]interfaces.Trade, bool, error) {
	return nil, false, nil
}

func (m *MockCacheLayer) CacheKlines(ctx context.Context, venue, symbol, interval string, klines []interfaces.Kline, ttl time.Duration) error {
	return nil
}

func (m *MockCacheLayer) GetCachedKlines(ctx context.Context, venue, symbol, interval string) ([]interfaces.Kline, bool, error) {
	return nil, false, nil
}

func (m *MockCacheLayer) CacheOrderBook(ctx context.Context, venue, symbol string, orderBook *interfaces.OrderBookSnapshot, ttl time.Duration) error {
	return nil
}

func (m *MockCacheLayer) GetCachedOrderBook(ctx context.Context, venue, symbol string) (*interfaces.OrderBookSnapshot, bool, error) {
	return nil, false, nil
}

func (m *MockCacheLayer) CacheFunding(ctx context.Context, venue, symbol string, funding *interfaces.FundingRate, ttl time.Duration) error {
	return nil
}

func (m *MockCacheLayer) GetCachedFunding(ctx context.Context, venue, symbol string) (*interfaces.FundingRate, bool, error) {
	return nil, false, nil
}

func (m *MockCacheLayer) CacheOpenInterest(ctx context.Context, venue, symbol string, oi *interfaces.OpenInterest, ttl time.Duration) error {
	return nil
}

func (m *MockCacheLayer) GetCachedOpenInterest(ctx context.Context, venue, symbol string) (*interfaces.OpenInterest, bool, error) {
	return nil, false, nil
}

func (m *MockCacheLayer) Close() error {
	return nil
}

// MockRateLimiter implements RateLimiter for testing
type MockRateLimiter struct{}

func (m *MockRateLimiter) Allow(ctx context.Context, venue, endpoint string) error {
	return nil // Always allow for testing
}

func (m *MockRateLimiter) GetLimits(ctx context.Context, venue string) (*interfaces.RateLimits, error) {
	return &interfaces.RateLimits{
		RequestsPerSecond: 10,
		BurstAllowance:    5,
	}, nil
}

func (m *MockRateLimiter) UpdateLimits(ctx context.Context, venue string, limits *interfaces.RateLimits) error {
	return nil
}

func (m *MockRateLimiter) ProcessRateLimitHeaders(venue string, headers map[string]string) error {
	return nil
}

// MockCircuitBreaker implements CircuitBreaker for testing
type MockCircuitBreaker struct{}

func (m *MockCircuitBreaker) Call(ctx context.Context, operation string, fn func() error) error {
	return fn() // Always execute for testing
}

func (m *MockCircuitBreaker) GetState(ctx context.Context, operation string) (*interfaces.CircuitState, error) {
	return &interfaces.CircuitState{
		State:        "closed",
		FailureCount: 0,
		SuccessCount: 10,
		ErrorRate:    0.0,
	}, nil
}

func (m *MockCircuitBreaker) ForceOpen(ctx context.Context, operation string) error {
	return nil
}

func (m *MockCircuitBreaker) ForceClose(ctx context.Context, operation string) error {
	return nil
}

func (m *MockCircuitBreaker) ConfigureBreaker(operation string, config interface{}) {
	// No-op for testing
}

// MockPITStore implements PITStore for testing
type MockPITStore struct {
	snapshots map[string]map[string]interface{}
}

func NewMockPITStore() *MockPITStore {
	return &MockPITStore{
		snapshots: make(map[string]map[string]interface{}),
	}
}

func (m *MockPITStore) CreateSnapshot(ctx context.Context, snapshotID string, data map[string]interface{}) error {
	m.snapshots[snapshotID] = data
	return nil
}

func (m *MockPITStore) GetSnapshot(ctx context.Context, snapshotID string) (map[string]interface{}, error) {
	data, exists := m.snapshots[snapshotID]
	if !exists {
		return nil, interfaces.ErrSnapshotNotFound
	}
	return data, nil
}

func (m *MockPITStore) ListSnapshots(ctx context.Context, filter interfaces.SnapshotFilter) ([]interfaces.SnapshotInfo, error) {
	var infos []interfaces.SnapshotInfo
	for id := range m.snapshots {
		infos = append(infos, interfaces.SnapshotInfo{
			SnapshotID: id,
			Timestamp:  time.Now(),
		})
	}
	return infos, nil
}

func (m *MockPITStore) DeleteSnapshot(ctx context.Context, snapshotID string) error {
	delete(m.snapshots, snapshotID)
	return nil
}

func (m *MockPITStore) LoadExistingSnapshots() error {
	return nil
}

func (m *MockPITStore) Cleanup(ctx context.Context, retentionDays int) error {
	return nil
}

func createTestFacade() *DataFacadeImpl {
	return &DataFacadeImpl{
		venues: map[string]interfaces.VenueAdapter{
			"mock_exchange": &MockVenueAdapter{venue: "mock_exchange"},
		},
		cache:          NewMockCacheLayer(),
		rateLimiter:    &MockRateLimiter{},
		circuitBreaker: &MockCircuitBreaker{},
		pitStore:       NewMockPITStore(),
		subscriptions:  make(map[string]subscription),
		config: &Config{
			Venues: map[string]VenueConfig{
				"mock_exchange": {
					BaseURL: "https://mock.example.com",
					WSURL:   "wss://mock.example.com/ws",
					Enabled: true,
				},
			},
		},
	}
}

func TestDataFacadeImpl_SubscribeToTrades(t *testing.T) {
	facade := createTestFacade()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	t.Run("subscribes to trades successfully", func(t *testing.T) {
		ch, err := facade.SubscribeToTrades(ctx, "mock_exchange", "BTCUSDT")
		if err != nil {
			t.Fatalf("SubscribeToTrades failed: %v", err)
		}
		
		// Wait for a trade event
		select {
		case trade := <-ch:
			if trade.Trade.Symbol != "BTCUSDT" {
				t.Errorf("Expected symbol BTCUSDT, got %s", trade.Trade.Symbol)
			}
			if trade.Trade.Venue != "mock_exchange" {
				t.Errorf("Expected venue mock_exchange, got %s", trade.Trade.Venue)
			}
		case <-time.After(1 * time.Second):
			t.Error("Timeout waiting for trade event")
		}
	})
	
	t.Run("returns error for unknown venue", func(t *testing.T) {
		_, err := facade.SubscribeToTrades(ctx, "unknown_venue", "BTCUSDT")
		if err == nil {
			t.Error("Expected error for unknown venue")
		}
	})
	
	t.Run("reuses existing subscription", func(t *testing.T) {
		// First subscription
		ch1, err := facade.SubscribeToTrades(ctx, "mock_exchange", "ETHUSDT")
		if err != nil {
			t.Fatalf("First SubscribeToTrades failed: %v", err)
		}
		
		// Second subscription should return the same channel
		ch2, err := facade.SubscribeToTrades(ctx, "mock_exchange", "ETHUSDT")
		if err != nil {
			t.Fatalf("Second SubscribeToTrades failed: %v", err)
		}
		
		if ch1 != ch2 {
			t.Error("Expected same channel for duplicate subscription")
		}
	})
}

func TestDataFacadeImpl_GetTrades(t *testing.T) {
	facade := createTestFacade()
	ctx := context.Background()
	
	t.Run("gets trades successfully", func(t *testing.T) {
		trades, err := facade.GetTrades(ctx, "mock_exchange", "BTCUSDT", 10)
		if err != nil {
			t.Fatalf("GetTrades failed: %v", err)
		}
		
		if len(trades) != 1 {
			t.Errorf("Expected 1 trade, got %d", len(trades))
		}
		
		if trades[0].Symbol != "BTCUSDT" {
			t.Errorf("Expected symbol BTCUSDT, got %s", trades[0].Symbol)
		}
	})
	
	t.Run("returns error for unknown venue", func(t *testing.T) {
		_, err := facade.GetTrades(ctx, "unknown_venue", "BTCUSDT", 10)
		if err == nil {
			t.Error("Expected error for unknown venue")
		}
	})
}

func TestDataFacadeImpl_GetTradesMultiVenue(t *testing.T) {
	facade := createTestFacade()
	ctx := context.Background()
	
	t.Run("gets trades from multiple venues", func(t *testing.T) {
		venues := []string{"mock_exchange"}
		result, err := facade.GetTradesMultiVenue(ctx, venues, "BTCUSDT", 10)
		if err != nil {
			t.Fatalf("GetTradesMultiVenue failed: %v", err)
		}
		
		if len(result) != 1 {
			t.Errorf("Expected 1 venue result, got %d", len(result))
		}
		
		trades, exists := result["mock_exchange"]
		if !exists {
			t.Error("Expected trades for mock_exchange")
		}
		
		if len(trades) != 1 {
			t.Errorf("Expected 1 trade, got %d", len(trades))
		}
	})
	
	t.Run("handles mixed valid and invalid venues", func(t *testing.T) {
		venues := []string{"mock_exchange", "invalid_venue"}
		result, err := facade.GetTradesMultiVenue(ctx, venues, "BTCUSDT", 10)
		if err != nil {
			t.Fatalf("GetTradesMultiVenue failed: %v", err)
		}
		
		// Should only have results for the valid venue
		if len(result) != 1 {
			t.Errorf("Expected 1 venue result, got %d", len(result))
		}
		
		_, exists := result["mock_exchange"]
		if !exists {
			t.Error("Expected trades for mock_exchange")
		}
		
		_, exists = result["invalid_venue"]
		if exists {
			t.Error("Should not have trades for invalid_venue")
		}
	})
}

func TestDataFacadeImpl_CreateSnapshot(t *testing.T) {
	facade := createTestFacade()
	ctx := context.Background()
	
	t.Run("creates snapshot successfully", func(t *testing.T) {
		snapshotID := "test_snapshot"
		
		err := facade.CreateSnapshot(ctx, snapshotID)
		if err != nil {
			t.Fatalf("CreateSnapshot failed: %v", err)
		}
		
		// Verify snapshot was created
		data, err := facade.GetSnapshot(ctx, snapshotID)
		if err != nil {
			t.Fatalf("GetSnapshot failed: %v", err)
		}
		
		if data == nil {
			t.Error("Expected snapshot data")
		}
		
		// Should have data for mock_exchange
		_, exists := data["mock_exchange"]
		if !exists {
			t.Error("Expected data for mock_exchange")
		}
	})
}

func TestDataFacadeImpl_GetHealth(t *testing.T) {
	facade := createTestFacade()
	ctx := context.Background()
	
	t.Run("returns health status", func(t *testing.T) {
		health, err := facade.GetHealth(ctx)
		if err != nil {
			t.Fatalf("GetHealth failed: %v", err)
		}
		
		if health.Overall != "healthy" {
			t.Errorf("Expected overall status 'healthy', got %s", health.Overall)
		}
		
		venueHealth, exists := health.Venues["mock_exchange"]
		if !exists {
			t.Error("Expected health for mock_exchange")
		}
		
		if !venueHealth.Healthy {
			t.Error("Expected mock_exchange to be healthy")
		}
	})
}

func TestDataFacadeImpl_GetMetrics(t *testing.T) {
	facade := createTestFacade()
	ctx := context.Background()
	
	t.Run("returns metrics", func(t *testing.T) {
		metrics, err := facade.GetMetrics(ctx)
		if err != nil {
			t.Fatalf("GetMetrics failed: %v", err)
		}
		
		if metrics.TotalVenues != 1 {
			t.Errorf("Expected 1 total venue, got %d", metrics.TotalVenues)
		}
		
		if metrics.EnabledVenues != 1 {
			t.Errorf("Expected 1 enabled venue, got %d", metrics.EnabledVenues)
		}
		
		// Should have cache stats
		if metrics.CacheStats.HitRate == 0.0 {
			t.Error("Expected non-zero hit rate from mock cache")
		}
	})
}

func TestDataFacadeImpl_Unsubscribe(t *testing.T) {
	facade := createTestFacade()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	t.Run("unsubscribes successfully", func(t *testing.T) {
		// First, create a subscription
		_, err := facade.SubscribeToTrades(ctx, "mock_exchange", "BTCUSDT")
		if err != nil {
			t.Fatalf("SubscribeToTrades failed: %v", err)
		}
		
		// Unsubscribe
		err = facade.Unsubscribe(ctx, "mock_exchange", "trades", "BTCUSDT")
		if err != nil {
			t.Fatalf("Unsubscribe failed: %v", err)
		}
		
		// Subscription should be removed
		if len(facade.subscriptions) != 0 {
			t.Errorf("Expected 0 subscriptions after unsubscribe, got %d", len(facade.subscriptions))
		}
	})
}

func TestDataFacadeImpl_GetSupportedVenues(t *testing.T) {
	facade := createTestFacade()
	
	t.Run("returns supported venues", func(t *testing.T) {
		venues := facade.GetSupportedVenues()
		if len(venues) != 1 {
			t.Errorf("Expected 1 supported venue, got %d", len(venues))
		}
		
		if venues[0] != "mock_exchange" {
			t.Errorf("Expected mock_exchange, got %s", venues[0])
		}
	})
}

func TestDataFacadeImpl_Shutdown(t *testing.T) {
	facade := createTestFacade()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	t.Run("shuts down cleanly", func(t *testing.T) {
		// Create some subscriptions first
		facade.SubscribeToTrades(ctx, "mock_exchange", "BTCUSDT")
		facade.SubscribeToKlines(ctx, "mock_exchange", "ETHUSDT", "1h")
		
		err := facade.Shutdown(ctx)
		if err != nil {
			t.Fatalf("Shutdown failed: %v", err)
		}
		
		// All subscriptions should be cancelled
		if len(facade.subscriptions) != 0 {
			t.Errorf("Expected 0 subscriptions after shutdown, got %d", len(facade.subscriptions))
		}
	})
}