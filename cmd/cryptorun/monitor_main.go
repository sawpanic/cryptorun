package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"cryptorun/internal/application"
	"cryptorun/internal/interfaces/http/endpoints"
	"cryptorun/internal/metrics"
)

// runMonitor starts the monitoring HTTP server
func runMonitor(cmd *cobra.Command, args []string) error {
	port, _ := cmd.Flags().GetString("port")
	host, _ := cmd.Flags().GetString("host")

	// Validate port
	if _, err := strconv.Atoi(port); err != nil {
		return fmt.Errorf("invalid port: %s", port)
	}

	addr := fmt.Sprintf("%s:%s", host, port)

	log.Info().Str("addr", addr).Msg("Starting CryptoRun monitoring server")

	// Initialize metrics collector and risk monitor
	metricsCollector := metrics.NewCollector()
	riskMonitor := application.NewRiskMonitor()

	// Create HTTP server with endpoints
	mux := http.NewServeMux()

	// Health endpoint
	mux.HandleFunc("/health", endpoints.HealthHandler(metricsCollector))

	// Metrics endpoint - API health, circuit breakers, cache hit rates, latencies, risk envelope
	mux.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		endpoints.MetricsHandlerWithRisk(metricsCollector, riskMonitor)(w, r)
	})

	// Decile endpoint - score vs forward returns analysis
	mux.HandleFunc("/decile", endpoints.DecileHandler(metricsCollector))

	// Risk envelope endpoint - dedicated risk management dashboard
	mux.HandleFunc("/risk", func(w http.ResponseWriter, r *http.Request) {
		endpoints.RiskEnvelopeHandler(riskMonitor)(w, r)
	})

	// New read-only analytical endpoints
	// Candidates endpoint - top composite candidates with gate status
	mux.HandleFunc("/candidates", endpoints.CandidatesHandler(metricsCollector))

	// Explain endpoint - explainability for specific symbols
	mux.HandleFunc("/explain/", endpoints.ExplainHandler())

	// Regime endpoint - current regime information and weights
	mux.HandleFunc("/regime", endpoints.RegimeHandler(metricsCollector))

	server := &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// Start background metrics collection
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go metricsCollector.StartCollection(ctx)

	// Start server in goroutine
	serverErr := make(chan error, 1)
	go func() {
		log.Info().
			Str("health", fmt.Sprintf("http://%s/health", addr)).
			Str("metrics", fmt.Sprintf("http://%s/metrics", addr)).
			Str("decile", fmt.Sprintf("http://%s/decile", addr)).
			Str("candidates", fmt.Sprintf("http://%s/candidates", addr)).
			Str("explain", fmt.Sprintf("http://%s/explain/{symbol}", addr)).
			Str("regime", fmt.Sprintf("http://%s/regime", addr)).
			Msg("Monitor endpoints available")

		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			serverErr <- err
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case <-quit:
		log.Info().Msg("Shutdown signal received")
	case err := <-serverErr:
		return fmt.Errorf("server error: %w", err)
	}

	// Graceful shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Error().Err(err).Msg("Server shutdown error")
		return err
	}

	log.Info().Msg("Monitor server shutdown complete")
	return nil
}
