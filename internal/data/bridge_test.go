package data

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockHotTier for testing
type MockHotTier struct {
	mock.Mock
}

func (m *MockHotTier) GetOrderBook(ctx context.Context, venue, symbol string) (*Envelope, error) {
	args := m.Called(ctx, venue, symbol)
	return args.Get(0).(*Envelope), args.Error(1)
}

func (m *MockHotTier) GetPriceData(ctx context.Context, venue, symbol string) (*Envelope, error) {
	args := m.Called(ctx, venue, symbol)
	return args.Get(0).(*Envelope), args.Error(1)
}

func (m *MockHotTier) IsAvailable(ctx context.Context, venue string) bool {
	args := m.Called(ctx, venue)
	return args.Bool(0)
}

func (m *MockHotTier) Subscribe(venue, symbol string) error {
	args := m.Called(venue, symbol)
	return args.Error(0)
}

func (m *MockHotTier) Unsubscribe(venue, symbol string) error {
	args := m.Called(venue, symbol)
	return args.Error(0)
}

func (m *MockHotTier) GetLatestTick(venue, symbol string) (*Envelope, error) {
	args := m.Called(venue, symbol)
	return args.Get(0).(*Envelope), args.Error(1)
}

// MockWarmTier for testing
type MockWarmTier struct {
	mock.Mock
}

func (m *MockWarmTier) GetOrderBook(ctx context.Context, venue, symbol string) (*Envelope, error) {
	args := m.Called(ctx, venue, symbol)
	return args.Get(0).(*Envelope), args.Error(1)
}

func (m *MockWarmTier) GetPriceData(ctx context.Context, venue, symbol string) (*Envelope, error) {
	args := m.Called(ctx, venue, symbol)
	return args.Get(0).(*Envelope), args.Error(1)
}

func (m *MockWarmTier) IsAvailable(ctx context.Context, venue string) bool {
	args := m.Called(ctx, venue)
	return args.Bool(0)
}

func (m *MockWarmTier) SetCacheTTL(venue string, ttlSeconds int) {
	m.Called(venue, ttlSeconds)
}

func (m *MockWarmTier) InvalidateCache(venue, symbol string) error {
	args := m.Called(venue, symbol)
	return args.Error(0)
}

func (m *MockWarmTier) GetCacheStats() CacheStats {
	args := m.Called()
	return args.Get(0).(CacheStats)
}

// MockColdTier for testing
type MockColdTier struct {
	mock.Mock
}

func (m *MockColdTier) GetOrderBook(ctx context.Context, venue, symbol string) (*Envelope, error) {
	args := m.Called(ctx, venue, symbol)
	return args.Get(0).(*Envelope), args.Error(1)
}

func (m *MockColdTier) GetPriceData(ctx context.Context, venue, symbol string) (*Envelope, error) {
	args := m.Called(ctx, venue, symbol)
	return args.Get(0).(*Envelope), args.Error(1)
}

func (m *MockColdTier) IsAvailable(ctx context.Context, venue string) bool {
	args := m.Called(ctx, venue)
	return args.Bool(0)
}

func (m *MockColdTier) GetHistoricalSlice(ctx context.Context, venue, symbol string, start, end time.Time) ([]*Envelope, error) {
	args := m.Called(ctx, venue, symbol, start, end)
	return args.Get(0).([]*Envelope), args.Error(1)
}

func (m *MockColdTier) LoadFromFile(filePath string) error {
	args := m.Called(filePath)
	return args.Error(0)
}

func TestBridgeCascade_HotTierSuccess(t *testing.T) {
	// Setup
	mockHot := new(MockHotTier)
	mockWarm := new(MockWarmTier)
	mockCold := new(MockColdTier)

	config := DefaultBridgeConfig()
	bridge := NewBridge(mockHot, mockWarm, mockCold, config)

	ctx := context.Background()
	venue := "binance"
	symbol := "BTCUSD"

	// Create fresh envelope (not stale)
	envelope := NewEnvelope(venue, symbol, TierHot)
	envelope.FreshnessMS = 1000 // 1 second, under 5 second limit

	// Setup expectations
	mockHot.On("IsAvailable", ctx, venue).Return(true)
	mockHot.On("GetOrderBook", ctx, venue, symbol).Return(envelope, nil)

	// Execute
	result, err := bridge.GetOrderBook(ctx, venue, symbol)

	// Assert
	assert.NoError(t, err)
	assert.Equal(t, TierHot, result.SourceTier)
	assert.Empty(t, result.Provenance.FallbackChain)
	mockHot.AssertExpectations(t)
}

func TestBridgeCascade_HotFailsWarmSucceeds(t *testing.T) {
	// Setup
	mockHot := new(MockHotTier)
	mockWarm := new(MockWarmTier)
	mockCold := new(MockColdTier)

	config := DefaultBridgeConfig()
	bridge := NewBridge(mockHot, mockWarm, mockCold, config)

	ctx := context.Background()
	venue := "binance"
	symbol := "BTCUSD"

	// Create warm envelope
	envelope := NewEnvelope(venue, symbol, TierWarm)
	envelope.FreshnessMS = 30000 // 30 seconds, under 60 second limit for warm

	// Setup expectations - hot fails, warm succeeds
	mockHot.On("IsAvailable", ctx, venue).Return(true)
	mockHot.On("GetOrderBook", ctx, venue, symbol).Return((*Envelope)(nil), errors.New("connection failed"))
	mockWarm.On("IsAvailable", ctx, venue).Return(true)
	mockWarm.On("GetOrderBook", ctx, venue, symbol).Return(envelope, nil)

	// Execute
	result, err := bridge.GetOrderBook(ctx, venue, symbol)

	// Assert
	assert.NoError(t, err)
	assert.Equal(t, TierWarm, result.SourceTier)
	assert.Contains(t, result.Provenance.FallbackChain, "hot_failed:connection failed")
	mockHot.AssertExpectations(t)
	mockWarm.AssertExpectations(t)
}

func TestBridgeCascade_HotStaleWarmSucceeds(t *testing.T) {
	// Setup
	mockHot := new(MockHotTier)
	mockWarm := new(MockWarmTier)
	mockCold := new(MockColdTier)

	config := DefaultBridgeConfig()
	bridge := NewBridge(mockHot, mockWarm, mockCold, config)

	ctx := context.Background()
	venue := "binance"
	symbol := "BTCUSD"

	// Create stale hot envelope
	staleEnvelope := NewEnvelope(venue, symbol, TierHot)
	staleEnvelope.FreshnessMS = 10000 // 10 seconds, over 5 second limit

	// Create fresh warm envelope
	warmEnvelope := NewEnvelope(venue, symbol, TierWarm)
	warmEnvelope.FreshnessMS = 30000 // 30 seconds, under 60 second limit

	// Setup expectations
	mockHot.On("IsAvailable", ctx, venue).Return(true)
	mockHot.On("GetOrderBook", ctx, venue, symbol).Return(staleEnvelope, nil)
	mockWarm.On("IsAvailable", ctx, venue).Return(true)
	mockWarm.On("GetOrderBook", ctx, venue, symbol).Return(warmEnvelope, nil)

	// Execute
	result, err := bridge.GetOrderBook(ctx, venue, symbol)

	// Assert
	assert.NoError(t, err)
	assert.Equal(t, TierWarm, result.SourceTier)
	assert.Contains(t, result.Provenance.FallbackChain, "hot_stale:10000ms")
	mockHot.AssertExpectations(t)
	mockWarm.AssertExpectations(t)
}

func TestBridgeCascade_AllTiersFallbackToCold(t *testing.T) {
	// Setup
	mockHot := new(MockHotTier)
	mockWarm := new(MockWarmTier)
	mockCold := new(MockColdTier)

	config := DefaultBridgeConfig()
	bridge := NewBridge(mockHot, mockWarm, mockCold, config)

	ctx := context.Background()
	venue := "binance"
	symbol := "BTCUSD"

	// Create cold envelope (no freshness check for cold)
	coldEnvelope := NewEnvelope(venue, symbol, TierCold)
	coldEnvelope.FreshnessMS = 3600000 // 1 hour - doesn't matter for cold tier

	// Setup expectations - hot and warm fail, cold succeeds
	mockHot.On("IsAvailable", ctx, venue).Return(true)
	mockHot.On("GetOrderBook", ctx, venue, symbol).Return((*Envelope)(nil), errors.New("hot failed"))
	mockWarm.On("IsAvailable", ctx, venue).Return(true)
	mockWarm.On("GetOrderBook", ctx, venue, symbol).Return((*Envelope)(nil), errors.New("warm failed"))
	mockCold.On("IsAvailable", ctx, venue).Return(true)
	mockCold.On("GetOrderBook", ctx, venue, symbol).Return(coldEnvelope, nil)

	// Execute
	result, err := bridge.GetOrderBook(ctx, venue, symbol)

	// Assert
	assert.NoError(t, err)
	assert.Equal(t, TierCold, result.SourceTier)
	assert.Len(t, result.Provenance.FallbackChain, 2) // hot and warm failures recorded
	assert.Contains(t, result.Provenance.FallbackChain, "hot_failed:hot failed")
	assert.Contains(t, result.Provenance.FallbackChain, "warm_failed:warm failed")

	mockHot.AssertExpectations(t)
	mockWarm.AssertExpectations(t)
	mockCold.AssertExpectations(t)
}

func TestBridgeCascade_AllTiersFail(t *testing.T) {
	// Setup
	mockHot := new(MockHotTier)
	mockWarm := new(MockWarmTier)
	mockCold := new(MockColdTier)

	config := DefaultBridgeConfig()
	bridge := NewBridge(mockHot, mockWarm, mockCold, config)

	ctx := context.Background()
	venue := "binance"
	symbol := "BTCUSD"

	// Setup expectations - all tiers fail
	mockHot.On("IsAvailable", ctx, venue).Return(true)
	mockHot.On("GetOrderBook", ctx, venue, symbol).Return((*Envelope)(nil), errors.New("hot failed"))
	mockWarm.On("IsAvailable", ctx, venue).Return(true)
	mockWarm.On("GetOrderBook", ctx, venue, symbol).Return((*Envelope)(nil), errors.New("warm failed"))
	mockCold.On("IsAvailable", ctx, venue).Return(true)
	mockCold.On("GetOrderBook", ctx, venue, symbol).Return((*Envelope)(nil), errors.New("cold failed"))

	// Execute
	result, err := bridge.GetOrderBook(ctx, venue, symbol)

	// Assert
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "all data tiers failed")
	assert.Contains(t, err.Error(), "fallback_chain")

	mockHot.AssertExpectations(t)
	mockWarm.AssertExpectations(t)
	mockCold.AssertExpectations(t)
}

func TestValidateSourceAuthority(t *testing.T) {
	bridge := &Bridge{}

	hotEnvelope := NewEnvelope("binance", "BTCUSD", TierHot)
	warmEnvelope := NewEnvelope("binance", "BTCUSD", TierWarm)
	coldEnvelope := NewEnvelope("binance", "BTCUSD", TierCold)

	// Test authority levels
	assert.Equal(t, 3, hotEnvelope.GetSourceAuthority())
	assert.Equal(t, 2, warmEnvelope.GetSourceAuthority())
	assert.Equal(t, 1, coldEnvelope.GetSourceAuthority())

	// Test authority validation
	assert.True(t, bridge.ValidateSourceAuthority(nil, hotEnvelope))           // No existing data
	assert.True(t, bridge.ValidateSourceAuthority(coldEnvelope, warmEnvelope)) // Higher authority
	assert.True(t, bridge.ValidateSourceAuthority(warmEnvelope, warmEnvelope)) // Same authority
	assert.False(t, bridge.ValidateSourceAuthority(hotEnvelope, warmEnvelope)) // Lower authority
}

func TestGetBestAvailableSource(t *testing.T) {
	mockHot := new(MockHotTier)
	mockWarm := new(MockWarmTier)
	mockCold := new(MockColdTier)

	config := DefaultBridgeConfig()
	bridge := NewBridge(mockHot, mockWarm, mockCold, config)

	ctx := context.Background()
	venue := "binance"

	// Test all available - should return hot
	mockHot.On("IsAvailable", ctx, venue).Return(true)
	result := bridge.GetBestAvailableSource(ctx, venue)
	assert.Equal(t, TierHot, result)

	// Test hot unavailable - should return warm
	mockHot2 := new(MockHotTier)
	mockHot2.On("IsAvailable", ctx, venue).Return(false)
	mockWarm.On("IsAvailable", ctx, venue).Return(true)
	bridge2 := NewBridge(mockHot2, mockWarm, mockCold, config)
	result2 := bridge2.GetBestAvailableSource(ctx, venue)
	assert.Equal(t, TierWarm, result2)
}
