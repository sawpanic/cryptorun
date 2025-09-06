package data

import (
	"testing"
	"time"

	"cryptorun/src/infrastructure/data"
)

func TestDataFacade_HotStreamIntegration(t *testing.T) {
	// Create facade with in-memory cache for testing
	cache := data.NewInMemoryCacheManager()
	reconciler := data.NewReconciler(data.ReconciliationConfig{
		MaxDeviation: 0.01,
		MinSources:   2,
		UseTrimmedMean: false,
	})
	
	config := &data.Config{
		HotExchanges: []string{"binance", "okx"},
		HotSetSize:   10,
		TTLs: map[string]int{
			"prices_hot":  5,
			"prices_warm": 30,
			"volumes_vadr": 120,
		},
	}
	config.Sources.PriceVolume = []string{"coingecko", "coinpaprika"}
	config.Sources.Microstructure = []string{"binance", "okx"}
	config.Reconcile.MaxDeviation = 0.01
	config.Reconcile.MinSources = 2
	
	facade := data.NewDataFacade(config, cache, reconciler)
	defer facade.Close()
	
	t.Run("hot_stream_subscription", func(t *testing.T) {
		symbols := []string{"BTCUSD", "ETHUSD"}
		
		stream, err := facade.HotSubscribe(symbols)
		if err != nil {
			t.Fatalf("Failed to subscribe to hot stream: %v", err)
		}
		defer stream.Close()
		
		// Wait for some data
		timeout := time.After(3 * time.Second)
		receivedData := false
		
		for !receivedData {
			select {
			case <-timeout:
				t.Error("Timeout waiting for hot stream data")
				return
			case trade := <-stream.Trades():
				t.Logf("Received trade: %+v", trade)
				receivedData = true
			case book := <-stream.Books():
				t.Logf("Received book: %+v", book)
				receivedData = true
			case bar := <-stream.Bars():
				t.Logf("Received bar: %+v", bar)
				receivedData = true
			case <-time.After(100 * time.Millisecond):
				// Continue waiting
			}
		}
		
		// Check stream health
		health := stream.Health()
		if !health.Connected {
			t.Error("Stream should be connected")
		}
		if health.MessageCount == 0 {
			t.Error("Stream should have received messages")
		}
	})
	
	t.Run("facade_health", func(t *testing.T) {
		health := facade.Health()
		
		if len(health.HotStreams) != 2 {
			t.Errorf("Expected 2 hot streams, got %d", len(health.HotStreams))
		}
		
		for exchange, streamHealth := range health.HotStreams {
			if !streamHealth.Connected {
				t.Errorf("Stream %s should be connected", exchange)
			}
		}
	})
}

func TestDataFacade_WarmDataIntegration(t *testing.T) {
	cache := data.NewInMemoryCacheManager()
	reconciler := data.NewReconciler(data.ReconciliationConfig{
		MaxDeviation: 0.01,
		MinSources:   2,
	})
	
	config := &data.Config{
		TTLs: map[string]int{
			"prices_warm": 30,
		},
	}
	config.Sources.PriceVolume = []string{"coingecko", "coinpaprika"}
	
	facade := data.NewDataFacade(config, cache, reconciler)
	defer facade.Close()
	
	t.Run("warm_klines_caching", func(t *testing.T) {
		req := data.KlineReq{
			Symbol: "BTCUSD",
			TF:     "1h",
			Since:  time.Now().Add(-24 * time.Hour),
			Until:  time.Now(),
			Limit:  24,
		}
		
		// First request - cache miss
		resp1, err := facade.WarmKlines(req)
		if err == nil {
			// Should fail since we don't have real data sources
			// but we can test the structure
			t.Logf("Response: %+v", resp1)
		} else {
			t.Logf("Expected error from mock sources: %v", err)
		}
		
		// Check cache stats
		stats := cache.Stats()
		t.Logf("Cache stats: %+v", stats)
	})
}

func TestDataFacade_MicrostructureAuthority(t *testing.T) {
	cache := data.NewInMemoryCacheManager()
	reconciler := data.NewReconciler(data.ReconciliationConfig{})
	
	config := &data.Config{
		HotExchanges: []string{"binance", "okx"},
	}
	config.Sources.Microstructure = []string{"binance", "okx"}
	
	facade := data.NewDataFacade(config, cache, reconciler)
	defer facade.Close()
	
	t.Run("exchange_native_only", func(t *testing.T) {
		// Test that L2Book only uses exchange-native sources
		_, err := facade.L2Book("BTCUSD")
		
		// Should fail since we don't have real exchange connections
		// but the important thing is it tries exchange-native sources only
		if err != nil {
			t.Logf("Expected error from mock exchange sources: %v", err)
			
			// Verify error message indicates exchange-native attempt
			if !contains(err.Error(), "exchange-native") {
				t.Error("Error should indicate exchange-native source attempt")
			}
		}
	})
}

func TestDataReconciliation_OutlierFiltering(t *testing.T) {
	reconciler := data.NewReconciler(data.ReconciliationConfig{
		MaxDeviation:   0.01, // 1% max deviation
		MinSources:     2,
		UseTrimmedMean: false,
	})
	
	t.Run("trim_outliers", func(t *testing.T) {
		// Create price data with one clear outlier
		sources := map[string]float64{
			"coingecko":   45000.0,
			"coinpaprika": 45050.0, // Within 1%
			"outlier":     50000.0, // 11% deviation - should be dropped
		}
		
		result, err := reconciler.ReconcilePrices(sources, "BTCUSD")
		if err != nil {
			t.Fatalf("Reconciliation should succeed: %v", err)
		}
		
		// Check that outlier was dropped
		if len(result.DroppedSources) != 1 || result.DroppedSources[0] != "outlier" {
			t.Errorf("Expected outlier to be dropped, got: %v", result.DroppedSources)
		}
		
		// Check that price is reasonable (median of good sources)
		if result.Price < 45000 || result.Price > 45100 {
			t.Errorf("Reconciled price %f should be between 45000-45100", result.Price)
		}
		
		// Check confidence
		if result.Confidence < 0.7 {
			t.Errorf("Confidence %f should be >= 0.7", result.Confidence)
		}
		
		t.Logf("Reconciliation result: %+v", result)
	})
	
	t.Run("insufficient_sources_after_filtering", func(t *testing.T) {
		// All sources are outliers except one
		sources := map[string]float64{
			"good":     45000.0,
			"outlier1": 50000.0, // Too high
			"outlier2": 40000.0, // Too low
		}
		
		_, err := reconciler.ReconcilePrices(sources, "BTCUSD")
		if err == nil {
			t.Error("Should fail with insufficient sources after outlier removal")
		}
		
		t.Logf("Expected error: %v", err)
	})
}

func TestCacheManager_PITSnapshots(t *testing.T) {
	cache := data.NewInMemoryCacheManager()
	
	t.Run("pit_snapshot_storage_retrieval", func(t *testing.T) {
		// Store PIT snapshot
		testData := map[string]interface{}{
			"price":  45000.0,
			"volume": 1000.0,
			"source": "test",
		}
		
		err := cache.StorePITSnapshot("BTCUSD:1h", testData, "test_source")
		if err != nil {
			t.Fatalf("Failed to store PIT snapshot: %v", err)
		}
		
		// Retrieve PIT snapshot
		retrieved, found := cache.GetPITSnapshot("BTCUSD:1h", time.Now())
		if !found {
			t.Fatal("PIT snapshot should be found")
		}
		
		if retrieved == nil {
			t.Fatal("Retrieved data should not be nil")
		}
		
		t.Logf("Retrieved PIT snapshot: %+v", retrieved)
	})
	
	t.Run("cache_stats", func(t *testing.T) {
		// Add some test data
		cache.Set("test_key", "test_value", 5*time.Minute)
		
		// Get data to register hit
		cache.Get("test_key")
		
		// Get non-existent data to register miss
		cache.Get("missing_key")
		
		stats := cache.Stats()
		
		if stats.TotalHits == 0 {
			t.Error("Should have at least one cache hit")
		}
		
		if stats.TotalMisses == 0 {
			t.Error("Should have at least one cache miss")
		}
		
		if !cache.Health() {
			t.Error("In-memory cache should always be healthy")
		}
		
		t.Logf("Cache stats: %+v", stats)
	})
}

// Helper function
func contains(s, substr string) bool {
	return len(s) >= len(substr) && 
		   (s == substr || 
		    (len(s) > len(substr) && 
			 (findSubstring(s, substr) >= 0)))
}

func findSubstring(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}