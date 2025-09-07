package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/sawpanic/cryptorun/src/infrastructure/datafacade"
	"github.com/sawpanic/cryptorun/src/infrastructure/datafacade/interfaces"
)

func main() {
	// Example 1: Basic Setup and Configuration
	basicSetupExample()
	
	// Example 2: Real-time Streaming
	streamingExample()
	
	// Example 3: Cached REST Data Access
	cachedDataExample()
	
	// Example 4: Multi-venue Aggregation
	multiVenueExample()
	
	// Example 5: Point-in-Time Snapshots
	pitSnapshotExample()
	
	// Example 6: Health Monitoring
	healthMonitoringExample()
}

func basicSetupExample() {
	fmt.Println("=== Basic Setup Example ===")
	
	// Use default configuration for demo
	config := datafacade.DefaultConfig()
	
	// Create factory and facade
	factory := datafacade.NewFactory(config)
	facade, err := factory.CreateDataFacade()
	if err != nil {
		log.Printf("Failed to create facade: %v", err)
		return
	}
	defer facade.Shutdown(context.Background())
	
	fmt.Printf("Data facade created successfully with %d venues\n", 
		len(facade.GetSupportedVenues()))
}

func streamingExample() {
	fmt.Println("\n=== Real-time Streaming Example ===")
	
	config := datafacade.DefaultConfig()
	factory := datafacade.NewFactory(config)
	facade, err := factory.CreateDataFacade()
	if err != nil {
		log.Printf("Failed to create facade: %v", err)
		return
	}
	defer facade.Shutdown(context.Background())
	
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	
	// Subscribe to trades from Binance
	tradesCh, err := facade.SubscribeToTrades(ctx, "binance", "BTC/USDT")
	if err != nil {
		log.Printf("Failed to subscribe to trades: %v", err)
		return
	}
	
	// Subscribe to order book updates
	orderBookCh, err := facade.SubscribeToOrderBook(ctx, "binance", "BTC/USDT", 10)
	if err != nil {
		log.Printf("Failed to subscribe to order book: %v", err)
		return
	}
	
	// Process events for a limited time
	timeout := time.After(5 * time.Second)
	tradeCount := 0
	bookCount := 0
	
	for {
		select {
		case trade := <-tradesCh:
			tradeCount++
			fmt.Printf("Trade #%d: %s %f @ %f on %s at %s\n", 
				tradeCount, trade.Trade.Side, trade.Trade.Quantity, 
				trade.Trade.Price, trade.Trade.Venue, trade.EventTime.Format("15:04:05"))
				
		case orderBook := <-orderBookCh:
			bookCount++
			bestBid := 0.0
			bestAsk := 0.0
			if len(orderBook.OrderBook.Bids) > 0 {
				bestBid = orderBook.OrderBook.Bids[0].Price
			}
			if len(orderBook.OrderBook.Asks) > 0 {
				bestAsk = orderBook.OrderBook.Asks[0].Price
			}
			fmt.Printf("OrderBook #%d: Best bid: %f, Best ask: %f, Spread: %f bps\n", 
				bookCount, bestBid, bestAsk, (bestAsk-bestBid)/bestBid*10000)
				
		case <-timeout:
			fmt.Printf("Streaming example completed. Received %d trades and %d order book updates\n", 
				tradeCount, bookCount)
			return
		}
	}
}

func cachedDataExample() {
	fmt.Println("\n=== Cached REST Data Example ===")
	
	config := datafacade.DefaultConfig()
	factory := datafacade.NewFactory(config)
	facade, err := factory.CreateDataFacade()
	if err != nil {
		log.Printf("Failed to create facade: %v", err)
		return
	}
	defer facade.Shutdown(context.Background())
	
	ctx := context.Background()
	
	// Get recent trades (cached for 30s by default)
	trades, err := facade.GetTrades(ctx, "binance", "BTC/USDT", 10)
	if err != nil {
		log.Printf("Failed to get trades: %v", err)
		return
	}
	
	fmt.Printf("Retrieved %d recent trades:\n", len(trades))
	for i, trade := range trades {
		fmt.Printf("  Trade %d: %s %f @ %f at %s\n", 
			i+1, trade.Side, trade.Quantity, trade.Price, 
			trade.Timestamp.Format("15:04:05"))
	}
	
	// Get klines data (cached for 60s by default)
	klines, err := facade.GetKlines(ctx, "binance", "BTC/USDT", "1h", 24)
	if err != nil {
		log.Printf("Failed to get klines: %v", err)
		return
	}
	
	fmt.Printf("\nRetrieved %d hourly klines:\n", len(klines))
	for i, kline := range klines[:5] { // Show first 5
		fmt.Printf("  Kline %d: O: %f, H: %f, L: %f, C: %f, V: %f\n", 
			i+1, kline.Open, kline.High, kline.Low, kline.Close, kline.Volume)
	}
	
	// Get order book snapshot (cached for 5s by default)
	orderBook, err := facade.GetOrderBook(ctx, "binance", "BTC/USDT", 5)
	if err != nil {
		log.Printf("Failed to get order book: %v", err)
		return
	}
	
	fmt.Printf("\nOrder Book (top 3 levels):\n")
	fmt.Printf("  Bids: ")
	for i, bid := range orderBook.Bids[:3] {
		fmt.Printf("%f@%f", bid.Price, bid.Quantity)
		if i < 2 { fmt.Printf(", ") }
	}
	fmt.Printf("\n  Asks: ")
	for i, ask := range orderBook.Asks[:3] {
		fmt.Printf("%f@%f", ask.Price, ask.Quantity)
		if i < 2 { fmt.Printf(", ") }
	}
	fmt.Println()
}

func multiVenueExample() {
	fmt.Println("\n=== Multi-venue Aggregation Example ===")
	
	config := datafacade.DefaultConfig()
	factory := datafacade.NewFactory(config)
	facade, err := factory.CreateDataFacade()
	if err != nil {
		log.Printf("Failed to create facade: %v", err)
		return
	}
	defer facade.Shutdown(context.Background())
	
	ctx := context.Background()
	venues := []string{"binance", "okx", "coinbase", "kraken"}
	symbol := "BTC/USDT"
	
	// Get trades from all venues simultaneously
	allTrades, err := facade.GetTradesMultiVenue(ctx, venues, symbol, 5)
	if err != nil {
		log.Printf("Failed to get multi-venue trades: %v", err)
		return
	}
	
	fmt.Printf("Multi-venue trades for %s:\n", symbol)
	for venue, trades := range allTrades {
		fmt.Printf("  %s: %d trades", venue, len(trades))
		if len(trades) > 0 {
			avgPrice := 0.0
			for _, trade := range trades {
				avgPrice += trade.Price
			}
			avgPrice /= float64(len(trades))
			fmt.Printf(" (avg price: %.2f)", avgPrice)
		}
		fmt.Println()
	}
	
	// Get order books from all venues simultaneously
	allOrderBooks, err := facade.GetOrderBookMultiVenue(ctx, venues, symbol, 5)
	if err != nil {
		log.Printf("Failed to get multi-venue order books: %v", err)
		return
	}
	
	fmt.Printf("\nMulti-venue order book spreads for %s:\n", symbol)
	for venue, orderBook := range allOrderBooks {
		if len(orderBook.Bids) > 0 && len(orderBook.Asks) > 0 {
			bestBid := orderBook.Bids[0].Price
			bestAsk := orderBook.Asks[0].Price
			spread := (bestAsk - bestBid) / bestBid * 10000
			fmt.Printf("  %s: spread %.2f bps (bid: %.2f, ask: %.2f)\n", 
				venue, spread, bestBid, bestAsk)
		}
	}
}

func pitSnapshotExample() {
	fmt.Println("\n=== Point-in-Time Snapshots Example ===")
	
	config := datafacade.DefaultConfig()
	factory := datafacade.NewFactory(config)
	facade, err := factory.CreateDataFacade()
	if err != nil {
		log.Printf("Failed to create facade: %v", err)
		return
	}
	defer facade.Shutdown(context.Background())
	
	ctx := context.Background()
	
	// Create a snapshot
	snapshotID := fmt.Sprintf("demo_snapshot_%d", time.Now().Unix())
	err = facade.CreateSnapshot(ctx, snapshotID)
	if err != nil {
		log.Printf("Failed to create snapshot: %v", err)
		return
	}
	
	fmt.Printf("Created snapshot: %s\n", snapshotID)
	
	// List all snapshots
	snapshots, err := facade.ListSnapshots(ctx, interfaces.SnapshotFilter{
		Limit: 10,
	})
	if err != nil {
		log.Printf("Failed to list snapshots: %v", err)
		return
	}
	
	fmt.Printf("Available snapshots (%d total):\n", len(snapshots))
	for i, snapshot := range snapshots {
		fmt.Printf("  %d. %s (created: %s, venues: %v)\n", 
			i+1, snapshot.SnapshotID, snapshot.Timestamp.Format("15:04:05"), 
			snapshot.Venues)
	}
	
	// Retrieve the snapshot we just created
	data, err := facade.GetSnapshot(ctx, snapshotID)
	if err != nil {
		log.Printf("Failed to get snapshot: %v", err)
		return
	}
	
	fmt.Printf("Snapshot data contains %d venues:\n", len(data))
	for venue := range data {
		fmt.Printf("  - %s\n", venue)
	}
}

func healthMonitoringExample() {
	fmt.Println("\n=== Health Monitoring Example ===")
	
	config := datafacade.DefaultConfig()
	factory := datafacade.NewFactory(config)
	facade, err := factory.CreateDataFacade()
	if err != nil {
		log.Printf("Failed to create facade: %v", err)
		return
	}
	defer facade.Shutdown(context.Background())
	
	ctx := context.Background()
	
	// Check overall health
	health, err := facade.GetHealth(ctx)
	if err != nil {
		log.Printf("Failed to get health: %v", err)
		return
	}
	
	fmt.Printf("Overall Status: %s\n", health.Overall)
	fmt.Printf("Health checked at: %s\n", health.Timestamp.Format("15:04:05"))
	fmt.Println("\nVenue Health:")
	for venue, venueHealth := range health.Venues {
		status := "🔴 unhealthy"
		if venueHealth.IsHealthy {
			status = "🟢 healthy"
		}
		fmt.Printf("  %s: %s (circuit: %s, last check: %s)\n", 
			venue, status, venueHealth.CircuitBreakerState, 
			venueHealth.LastCheck.Format("15:04:05"))
	}
	
	// Get detailed metrics
	metrics, err := facade.GetMetrics(ctx)
	if err != nil {
		log.Printf("Failed to get metrics: %v", err)
		return
	}
	
	fmt.Printf("\nSystem Metrics:\n")
	fmt.Printf("  Active Streams: %d\n", metrics.ActiveStreams)
	fmt.Printf("  Total Venues: %d\n", metrics.TotalVenues)
	fmt.Printf("  Enabled Venues: %d\n", metrics.EnabledVenues)
	fmt.Printf("  Cache Hit Rate: %.2f%%\n", metrics.CacheStats.HitRate*100)
	fmt.Printf("  Cache Items: %d\n", metrics.CacheStats.ItemCount)
	fmt.Printf("  Cache Size: %d bytes\n", metrics.CacheStats.Size)
	
	// Demonstrate rate limiting information
	// Note: Rate limit details are internal to the facade
	fmt.Println("\nRate limiting is handled automatically by the facade")
}

// Helper function for error handling
func handleError(operation string, err error) bool {
	if err != nil {
		log.Printf("%s failed: %v", operation, err)
		return true
	}
	return false
}