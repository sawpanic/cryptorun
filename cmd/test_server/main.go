package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/sawpanic/cryptorun/internal/interfaces/http/endpoints"
	"github.com/sawpanic/cryptorun/internal/metrics"
)

func main() {
	// Configure logging
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	// Create metrics collector
	metricsCollector := metrics.NewCollector()

	// Create HTTP server with endpoints
	mux := http.NewServeMux()

	// New read-only analytical endpoints
	mux.HandleFunc("/candidates", endpoints.CandidatesHandler(metricsCollector))
	mux.HandleFunc("/explain/", endpoints.ExplainHandler())
	mux.HandleFunc("/regime", endpoints.RegimeHandler(metricsCollector))

	// Simple health check
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"healthy","service":"test_server"}`))
	})

	server := &http.Server{
		Addr:         "127.0.0.1:8080",
		Handler:      mux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// Start server in goroutine
	serverErr := make(chan error, 1)
	go func() {
		log.Info().
			Str("candidates", "http://127.0.0.1:8080/candidates").
			Str("explain", "http://127.0.0.1:8080/explain/BTC-USD").
			Str("regime", "http://127.0.0.1:8080/regime").
			Str("health", "http://127.0.0.1:8080/health").
			Msg("Test server endpoints available")

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
		log.Error().Err(err).Msg("Server error")
		return
	}

	// Graceful shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Error().Err(err).Msg("Server shutdown error")
		return
	}

	log.Info().Msg("Test server shutdown complete")
}
