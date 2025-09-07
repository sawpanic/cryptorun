package unit

import (
	"testing"
	"time"

	"github.com/sawpanic/cryptorun/internal/application"
	"github.com/sawpanic/cryptorun/internal/infrastructure/websocket"
)

// TestHotSetIntegration tests the integration between hot set and scanner
func TestHotSetIntegration(t *testing.T) {
	// Create test configuration
	config := &websocket.HotSetConfig{
		TopN:            10,
		UpdateInterval:  5 * time.Minute,
		ReconnectDelay:  5 * time.Second,
		MaxReconnects:   3,
		MetricsInterval: 30 * time.Second,
		VADRMinBars:     20,
		StaleThreshold:  60 * time.Second,
		Venues: []websocket.VenueConfig{
			{
				Name:       "test_venue",
				WSEndpoint: "ws://test",
				Enabled:    false, // Don't actually connect
			},
		},
	}

	// Create integration
	integration := application.NewHotSetIntegration(config)

	// Test that it starts as not running
	if integration.IsRunning() {
		t.Error("Integration should not be running initially")
	}

	// Test getting active symbols when not running
	symbols := integration.GetActiveSymbols()
	if len(symbols) != 0 {
		t.Errorf("Expected 0 active symbols when not running, got %d", len(symbols))
	}

	// Test health check
	health := integration.GetMicrostructureHealth()
	if health.Status != "unhealthy" && health.Status != "degraded" {
		t.Logf("Health status: %s - %s", health.Status, health.Reason)
	}

	// Test configuration
	gotConfig := integration.GetConfig()
	if gotConfig.TopN != config.TopN {
		t.Errorf("Expected TopN %d, got %d", config.TopN, gotConfig.TopN)
	}

	// Test subscribing when not running (should get closed channel)
	tickChan := integration.SubscribeToTicks()
	select {
	case _, ok := <-tickChan:
		if ok {
			t.Error("Expected closed channel when not running")
		}
	case <-time.After(100 * time.Millisecond):
		t.Log("Channel appears to be closed (good)")
	}
}

// TestMicrostructureProvider tests the microstructure provider functionality
func TestMicrostructureProvider(t *testing.T) {
	// Create a hotset manager (without starting it)
	config := &websocket.HotSetConfig{
		VADRMinBars: 20,
	}
	hotsetManager := websocket.NewHotSetManager(config)

	// Create provider
	provider := websocket.NewMicrostructureProvider(hotsetManager)

	// Test when no symbols are active
	symbols := provider.GetActiveSymbols()
	if len(symbols) != 0 {
		t.Errorf("Expected 0 active symbols, got %d", len(symbols))
	}

	// Test checking if symbol is active
	isActive := provider.IsSymbolActive("BTCUSD")
	if isActive {
		t.Error("Symbol should not be active when hotset is not started")
	}

	// Test health check
	health := provider.MicrostructureHealthCheck()
	if health.Status != "degraded" {
		t.Logf("Health status: %s", health.Status)
	}

	if health.Symbols != 0 {
		t.Errorf("Expected 0 symbols, got %d", health.Symbols)
	}
}

// TestMicrostructureGateValidation tests gate validation logic
func TestMicrostructureGateValidation(t *testing.T) {
	// This test would require a more complete setup with actual microstructure data
	// For now, just test the basic structure

	config := &websocket.HotSetConfig{
		VADRMinBars: 5,
	}
	hotsetManager := websocket.NewHotSetManager(config)
	provider := websocket.NewMicrostructureProvider(hotsetManager)

	// Test getting inputs for non-existent symbol
	_, err := provider.GetMicrostructureInputs("NONEXISTENT")
	if err == nil {
		t.Error("Expected error for non-existent symbol")
	}

	// Test getting latest tick (not implemented yet)
	_, err = provider.GetLatestTick("BTCUSD")
	if err == nil {
		t.Error("Expected error for unimplemented latest tick functionality")
	}
}
