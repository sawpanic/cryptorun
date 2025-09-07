package db

import (
	"context"
	"fmt"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/sawpanic/cryptorun/internal/data/facade"
	"github.com/sawpanic/cryptorun/internal/persistence"
)

// Integration provides database integration for the CryptoRun application
type Integration struct {
	config  *AppConfig
	manager *Manager
	pitStore facade.PITStore
}

// NewIntegration creates a new database integration with the given configuration
func NewIntegration(config *AppConfig) (*Integration, error) {
	if config == nil {
		config = DefaultAppConfig()
	}

	// Validate configuration
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	// Create database manager
	manager, err := NewManager(config.Database)
	if err != nil {
		return nil, fmt.Errorf("failed to create database manager: %w", err)
	}

	// Create PIT store with file fallback
	pitStorePath := "data/pit" // Default file storage path
	pitStore := NewPITStore(manager, pitStorePath)

	integration := &Integration{
		config:   config,
		manager:  manager,
		pitStore: pitStore,
	}

	log.Info().
		Bool("db_enabled", config.Database.Enabled).
		Str("primary_exchange", config.APIs.PrimaryExchange).
		Msg("Database integration initialized")

	return integration, nil
}

// Manager returns the database manager for direct repository access
func (i *Integration) Manager() *Manager {
	return i.manager
}

// Repository returns the repository collection (nil if database disabled)
func (i *Integration) Repository() *persistence.Repository {
	if i.manager == nil {
		return nil
	}
	return i.manager.Repository()
}

// PITStore returns the Point-in-Time store for data facade integration
func (i *Integration) PITStore() facade.PITStore {
	return i.pitStore
}

// Health returns the database health status
func (i *Integration) Health(ctx context.Context) persistence.HealthCheck {
	if i.manager == nil {
		return persistence.HealthCheck{
			Healthy:        true,
			Errors:         []string{"Database integration disabled"},
			ConnectionPool: map[string]int{"status": 0},
			LastCheck:      time.Now(),
			ResponseTimeMS: 0,
		}
	}

	return i.manager.Health().Health(ctx)
}

// IsEnabled returns whether database persistence is enabled
func (i *Integration) IsEnabled() bool {
	return i.config.Database.Enabled && i.manager != nil && i.manager.IsEnabled()
}

// Config returns the application configuration
func (i *Integration) Config() *AppConfig {
	return i.config
}

// Close gracefully shuts down the database integration
func (i *Integration) Close() error {
	if i.manager == nil {
		return nil
	}

	log.Info().Msg("Closing database integration")
	return i.manager.Close()
}

// RunMigrations executes database migrations (requires enabled database with valid DSN)
func (i *Integration) RunMigrations() error {
	if !i.IsEnabled() {
		return fmt.Errorf("database is not enabled - cannot run migrations")
	}

	// This would integrate with Goose migrations
	// For now, we'll just log that migrations would be run
	log.Info().Msg("Database migrations would be executed here")
	log.Info().Msg("Use 'goose -dir db/migrations postgres \"$PG_DSN\" up' to run migrations manually")
	
	return nil
}

// SetupDevelopment sets up a development database with test data (for local development only)
func (i *Integration) SetupDevelopment() error {
	if !i.IsEnabled() {
		return fmt.Errorf("database is not enabled")
	}

	repos := i.Repository()
	if repos == nil {
		return fmt.Errorf("repository not available")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Insert sample regime snapshot
	sampleRegime := persistence.RegimeSnapshot{
		Timestamp:       time.Now(),
		RealizedVol7d:   0.35,
		PctAbove20MA:    65.0,
		BreadthThrust:   0.15,
		Regime:          "trending",
		Weights: map[string]float64{
			"momentum":  40.0,
			"technical": 25.0,
			"volume":    20.0,
			"quality":   10.0,
			"social":    5.0,
		},
		ConfidenceScore: 0.78,
		DetectionMethod: "majority_vote",
		Metadata:        map[string]interface{}{"sample": true},
	}

	if err := repos.Regimes.Upsert(ctx, sampleRegime); err != nil {
		return fmt.Errorf("failed to insert sample regime: %w", err)
	}

	// Insert sample trade
	sampleTrade := persistence.Trade{
		Timestamp:  time.Now(),
		Symbol:     "BTC/USD",
		Venue:      "kraken",
		Side:       "buy",
		Price:      50000.0,
		Qty:        0.1,
		Attributes: map[string]interface{}{"sample": true},
	}

	if err := repos.Trades.Insert(ctx, sampleTrade); err != nil {
		return fmt.Errorf("failed to insert sample trade: %w", err)
	}

	log.Info().Msg("Development database setup completed with sample data")
	return nil
}

// Statistics returns database usage statistics
func (i *Integration) Statistics(ctx context.Context) map[string]interface{} {
	if !i.IsEnabled() {
		return map[string]interface{}{
			"enabled": false,
			"status":  "disabled",
		}
	}

	health := i.manager.Health()
	stats := health.Stats(ctx)
	
	// Add additional statistics
	repos := i.Repository()
	if repos != nil {
		timeRange := persistence.TimeRange{
			From: time.Now().Add(-24 * time.Hour),
			To:   time.Now(),
		}
		
		if tradeCount, err := repos.Trades.Count(ctx, timeRange); err == nil {
			stats["trades_24h"] = tradeCount
		}
		
		if venueStats, err := repos.Trades.CountByVenue(ctx, timeRange); err == nil {
			stats["trades_by_venue_24h"] = venueStats
		}
		
		if regimeStats, err := repos.Regimes.GetRegimeStats(ctx, timeRange); err == nil {
			stats["regime_distribution_24h"] = regimeStats
		}
	}

	return stats
}

// BackupConfig creates a timestamped backup of the current configuration
func (i *Integration) BackupConfig(backupDir string) error {
	timestamp := time.Now().Format("20060102_150405")
	backupPath := fmt.Sprintf("%s/config_backup_%s.yaml", backupDir, timestamp)
	
	return SaveAppConfig(i.config, backupPath)
}