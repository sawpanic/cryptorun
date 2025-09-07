package main

import (
	"context"
	"fmt"
	_ "os" // Unused in current build
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/rs/zerolog/log"

	"github.com/sawpanic/cryptorun/internal/data/facade"
	"github.com/sawpanic/cryptorun/internal/data/cache"
	"github.com/sawpanic/cryptorun/internal/data/pit"
	"github.com/sawpanic/cryptorun/internal/data/rl"
	"github.com/sawpanic/cryptorun/internal/data/exchanges/kraken"
	"github.com/sawpanic/cryptorun/internal/metrics"
)

// runProbeData implements the data probe command
func runProbeData(cmd *cobra.Command, args []string) error {
	pair, _ := cmd.Flags().GetString("pair")
	venue, _ := cmd.Flags().GetString("venue")
	mins, _ := cmd.Flags().GetInt("mins")
	stream, _ := cmd.Flags().GetBool("stream")
	
	log.Info().Str("pair", pair).Str("venue", venue).Int("mins", mins).
		Bool("stream", stream).Msg("Starting data probe")
	
	// Initialize data facade
	dataFacade, err := initializeDataFacade()
	if err != nil {
		return fmt.Errorf("failed to initialize data facade: %w", err)
	}
	
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(mins)*time.Minute)
	defer cancel()
	
	// Start facade
	if err := dataFacade.Start(ctx); err != nil {
		return fmt.Errorf("failed to start data facade: %w", err)
	}
	defer dataFacade.Stop()
	
	if stream {
		return runStreamingProbe(ctx, dataFacade, venue, pair)
	} else {
		return runStaticProbe(ctx, dataFacade, venue, pair)
	}
}

// initializeDataFacade creates and configures the data facade
func initializeDataFacade() (facade.DataFacade, error) {
	// Configuration
	hotCfg := facade.HotConfig{
		Venues:       []string{"kraken", "binance", "okx", "coinbase"},
		MaxPairs:     30,
		ReconnectSec: 5,
		BufferSize:   1000,
		Timeout:      10 * time.Second,
	}
	
	warmCfg := facade.WarmConfig{
		Venues:       []string{"kraken", "binance", "okx", "coinbase"},
		DefaultTTL:   30 * time.Second,
		MaxRetries:   3,
		BackoffBase:  1 * time.Second,
		RequestLimit: 100,
	}
	
	cacheCfg := facade.CacheConfig{
		PricesHot:   5 * time.Second,
		PricesWarm:  30 * time.Second,
		VolumesVADR: 120 * time.Second,
		TokenMeta:   24 * time.Hour,
		MaxEntries:  10000,
	}
	
	// Create components
	_ = cache.NewTTLCache(cacheCfg.MaxEntries) // ttlCache unused in stub
	rateLimiter := rl.NewRateLimiter()
	_ = pit.NewStore("artifacts/pit") // pitStore unused in stub
	
	// Create facade
	df := facade.New(hotCfg, warmCfg, cacheCfg, rateLimiter)
	
	// Register exchange adapters
	_ = kraken.NewAdapter() // krakenAdapter unused in stub
	// Note: In full implementation, would register all exchange adapters
	
	log.Info().Msg("Data facade initialized")
	return df, nil
}

// runStaticProbe performs a one-time data fetch and analysis
func runStaticProbe(ctx context.Context, df facade.DataFacade, venue, pair string) error {
	fmt.Printf("🔍 CryptoRun Data Probe - Static Analysis\n")
	fmt.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n\n")
	
	// Display venue health status
	fmt.Printf("📊 Venue Health Status\n")
	fmt.Printf("%-12s %-10s %-12s %-10s %-12s %-s\n", 
		"VENUE", "STATUS", "WS", "REST", "LATENCY", "RECOMMENDATION")
	fmt.Printf("%-12s %-10s %-12s %-10s %-12s %-s\n",
		"-----", "------", "--", "----", "-------", "--------------")
	
	venues := []string{"kraken", "binance", "okx", "coinbase"}
	for _, v := range venues {
		health := df.VenueHealth(v)
		wsStatus := "❌"
		if health.WSConnected {
			wsStatus = "✅"
		}
		restStatus := "❌"
		if health.RESTHealthy {
			restStatus = "✅"
		}
		
		latencyStr := fmt.Sprintf("%dms", health.P99Latency/time.Millisecond)
		
		fmt.Printf("%-12s %-10s %-12s %-10s %-12s %-s\n",
			v, health.Status, wsStatus, restStatus, latencyStr, health.Recommendation)
	}
	
	// Display cache statistics
	fmt.Printf("\n💾 Cache Performance\n")
	cacheStats := df.CacheStats()
	fmt.Printf("%-15s %-8s %-8s %-8s %-10s %-s\n",
		"TIER", "TTL", "HITS", "MISSES", "ENTRIES", "HIT_RATIO")
	fmt.Printf("%-15s %-8s %-8s %-8s %-10s %-s\n",
		"----", "---", "----", "------", "-------", "---------")
	
	tiers := map[string]facade.CacheTierStats{
		"prices_hot":    cacheStats.PricesHot,
		"prices_warm":   cacheStats.PricesWarm,
		"volumes_vadr":  cacheStats.VolumesVADR,
		"token_meta":    cacheStats.TokenMeta,
	}
	
	for name, stats := range tiers {
		ttlStr := fmt.Sprintf("%ds", int(stats.TTL.Seconds()))
		hitRatioStr := fmt.Sprintf("%.1f%%", stats.HitRatio*100)
		
		fmt.Printf("%-15s %-8s %-8d %-8d %-10d %-s\n",
			name, ttlStr, stats.Hits, stats.Misses, stats.Entries, hitRatioStr)
	}
	
	// Test data fetching for specific pair
	if pair != "" && venue != "" {
		fmt.Printf("\n📈 Data Fetch Test: %s on %s\n", strings.ToUpper(pair), strings.ToUpper(venue))
		
		// Fetch klines
		klines, err := df.GetKlines(ctx, venue, pair, "1h", 24)
		if err != nil {
			fmt.Printf("❌ Klines fetch failed: %v\n", err)
		} else {
			fmt.Printf("✅ Klines: %d bars fetched\n", len(klines))
			if len(klines) > 0 {
				latest := klines[len(klines)-1]
				fmt.Printf("   Latest: %s O:%.4f H:%.4f L:%.4f C:%.4f V:%.2f\n",
					latest.Timestamp.Format("15:04"), latest.Open, latest.High, 
					latest.Low, latest.Close, latest.Volume)
			}
		}
		
		// Fetch recent trades
		trades, err := df.GetTrades(ctx, venue, pair, 10)
		if err != nil {
			fmt.Printf("❌ Trades fetch failed: %v\n", err)
		} else {
			fmt.Printf("✅ Trades: %d recent trades fetched\n", len(trades))
			if len(trades) > 0 {
				latest := trades[len(trades)-1]
				fmt.Printf("   Latest: %s %s %.4f @ %.4f\n",
					latest.Timestamp.Format("15:04:05"), latest.Side, latest.Size, latest.Price)
			}
		}
		
		// Fetch orderbook
		book, err := df.GetBookL2(ctx, venue, pair)
		if err != nil {
			fmt.Printf("❌ Orderbook fetch failed: %v\n", err)
		} else {
			fmt.Printf("✅ Orderbook: %d bids, %d asks\n", len(book.Bids), len(book.Asks))
			if len(book.Bids) > 0 && len(book.Asks) > 0 {
				bestBid := book.Bids[0]
				bestAsk := book.Asks[0]
				spread := ((bestAsk.Price - bestBid.Price) / bestBid.Price) * 10000 // bps
				fmt.Printf("   Spread: %.4f - %.4f = %.1f bps\n", 
					bestBid.Price, bestAsk.Price, spread)
			}
		}
		
		// Calculate VADR if we have klines
		if len(klines) > 0 {
			fmt.Printf("\n📊 VADR Analysis\n")
			vadrCalc := metrics.NewVADRCalculator()
			vadrMetrics := vadrCalc.GetVADRMetrics(klines, 5000000, 24*time.Hour) // Assume $5M ADV
			
			fmt.Printf("   VADR Value: %.2f\n", vadrMetrics.Value)
			fmt.Printf("   Frozen: %v\n", vadrMetrics.Frozen)
			fmt.Printf("   Tier: %s (min: %.2f)\n", vadrMetrics.Tier.Name, vadrMetrics.Tier.MinVADR)
			fmt.Printf("   Valid: %v\n", vadrMetrics.Valid)
			if vadrMetrics.Reason != "" {
				fmt.Printf("   Reason: %s\n", vadrMetrics.Reason)
			}
		}
	}
	
	// Display source attribution
	fmt.Printf("\n🔍 Source Attribution\n")
	for _, v := range venues {
		attr := df.SourceAttribution(v)
		if !attr.LastUpdate.IsZero() {
			fmt.Printf("%-12s: %d sources, %d hits, %d misses, latency %dms\n",
				v, len(attr.Sources), attr.CacheHits, attr.CacheMisses,
				attr.Latency/time.Millisecond)
		}
	}
	
	fmt.Printf("\n✅ Static probe completed\n")
	return nil
}

// runStreamingProbe performs continuous monitoring of streaming data
func runStreamingProbe(ctx context.Context, df facade.DataFacade, venue, pair string) error {
	fmt.Printf("📡 CryptoRun Data Probe - Streaming Mode\n")
	fmt.Printf("Venue: %s, Pair: %s\n", strings.ToUpper(venue), strings.ToUpper(pair))
	fmt.Printf("Press Ctrl+C to stop...\n\n")
	
	// Subscribe to trades
	tradeCount := 0
	tradesCallback := func(trades []facade.Trade) error {
		for _, trade := range trades {
			tradeCount++
			fmt.Printf("[%s] 📈 %s %s %.6f @ %.4f (ID: %s)\n",
				trade.Timestamp.Format("15:04:05"),
				strings.ToUpper(trade.Side),
				trade.Symbol,
				trade.Size,
				trade.Price,
				trade.TradeID)
		}
		return nil
	}
	
	if err := df.SubscribeTrades(ctx, venue, pair, tradesCallback); err != nil {
		log.Warn().Err(err).Msg("Failed to subscribe to trades")
	}
	
	// Subscribe to orderbook updates
	bookUpdateCount := 0
	bookCallback := func(book *facade.BookL2) error {
		bookUpdateCount++
		if len(book.Bids) > 0 && len(book.Asks) > 0 {
			bestBid := book.Bids[0]
			bestAsk := book.Asks[0]
			spread := ((bestAsk.Price - bestBid.Price) / bestBid.Price) * 10000
			
			fmt.Printf("[%s] 📊 Book: %.4f x %.2f | %.2f x %.4f (spread: %.1f bps)\n",
				book.Timestamp.Format("15:04:05"),
				bestBid.Price, bestBid.Size,
				bestAsk.Size, bestAsk.Price,
				spread)
		}
		return nil
	}
	
	if err := df.SubscribeBookL2(ctx, venue, pair, bookCallback); err != nil {
		log.Warn().Err(err).Msg("Failed to subscribe to orderbook")
	}
	
	// Periodic status updates
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	
	startTime := time.Now()
	
	for {
		select {
		case <-ctx.Done():
			duration := time.Since(startTime)
			fmt.Printf("\n📊 Streaming Summary\n")
			fmt.Printf("Duration: %s\n", duration)
			fmt.Printf("Trades processed: %d\n", tradeCount)
			fmt.Printf("Book updates: %d\n", bookUpdateCount)
			fmt.Printf("Avg trades/min: %.1f\n", float64(tradeCount)/duration.Minutes())
			return nil
			
		case <-ticker.C:
			// Display periodic health status
			health := df.VenueHealth(venue)
			cacheStats := df.CacheStats()
			
			fmt.Printf("\n[%s] 🔍 Status Check\n", time.Now().Format("15:04:05"))
			fmt.Printf("  Venue: %s (%s)\n", health.Venue, health.Status)
			fmt.Printf("  WS Connected: %v, REST Healthy: %v\n", health.WSConnected, health.RESTHealthy)
			fmt.Printf("  Cache Entries: %d\n", cacheStats.TotalEntries)
			fmt.Printf("  Trades: %d, Books: %d\n", tradeCount, bookUpdateCount)
		}
	}
}