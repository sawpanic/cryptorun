# Data Persistence Layer

## Overview

CryptoRun implements a **dual-tier persistence system** with optional PostgreSQL database storage alongside file-based storage. This design ensures backwards compatibility while providing structured query capabilities for production deployments.

## Architecture

```mermaid
graph TB
    A[Data Facade] --> B[PIT Store]
    B --> C[File Storage]
    B --> D[PostgreSQL Database]
    D --> E[Trades Repository]
    D --> F[Regime Repository]  
    D --> G[Premove Repository]
    H[Database Manager] --> D
    I[Configuration] --> H
```

### Key Components

- **Database Manager**: Connection pool management and health monitoring
- **Repository Interfaces**: Type-safe data access layer with PostgreSQL implementations
- **PIT Store**: Point-in-Time storage with dual file/database persistence
- **Integration Layer**: Unified configuration and lifecycle management

## Database Schema

### Tables

#### `trades`
Exchange-native trade execution log with Point-in-Time integrity.

```sql
CREATE TABLE trades (
    id BIGSERIAL PRIMARY KEY,
    ts TIMESTAMPTZ NOT NULL,           -- Point-in-time from exchange
    symbol TEXT NOT NULL,
    venue TEXT NOT NULL CHECK (venue IN ('binance','okx','coinbase','kraken')),
    side TEXT CHECK (side IN ('buy','sell')),
    price DOUBLE PRECISION NOT NULL CHECK (price > 0),
    qty DOUBLE PRECISION NOT NULL CHECK (qty > 0),
    order_id TEXT,
    attributes JSONB DEFAULT '{}',      -- Exchange-specific data
    created_at TIMESTAMPTZ DEFAULT NOW()
);
```

**Indexes:**
- `trades_symbol_ts_idx`: Symbol + timestamp for PIT queries
- `trades_venue_ts_idx`: Venue + timestamp for performance analysis
- `trades_order_id_idx`: Order ID for reconciliation (partial index)

#### `regime_snapshots`
Market regime detection results with 4-hour cadence.

```sql
CREATE TABLE regime_snapshots (
    ts TIMESTAMPTZ PRIMARY KEY,        -- 4h boundary timestamp
    realized_vol_7d DOUBLE PRECISION NOT NULL CHECK (realized_vol_7d >= 0),
    pct_above_20ma DOUBLE PRECISION NOT NULL CHECK (pct_above_20ma >= 0 AND pct_above_20ma <= 100),
    breadth_thrust DOUBLE PRECISION NOT NULL CHECK (breadth_thrust >= -1 AND breadth_thrust <= 1),
    regime TEXT NOT NULL CHECK (regime IN ('trending','choppy','highvol','mixed')),
    weights JSONB NOT NULL,            -- Factor weights for this regime
    confidence_score DOUBLE PRECISION DEFAULT 0.5 CHECK (confidence_score >= 0 AND confidence_score <= 1),
    detection_method TEXT DEFAULT 'majority_vote',
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMPTZ DEFAULT NOW()
);
```

**Constraints:**
- Weights JSON validation ensures proper structure and social cap (≤10)
- Regime type validation against allowed values

#### `premove_artifacts`
Pre-movement detection artifacts with entry gates and composite scoring.

```sql
CREATE TABLE premove_artifacts (
    id BIGSERIAL PRIMARY KEY,
    ts TIMESTAMPTZ NOT NULL,
    symbol TEXT NOT NULL,
    venue TEXT NOT NULL,
    
    -- Entry Gates (hard requirements)
    gate_score BOOLEAN DEFAULT FALSE,         -- Score ≥ 75
    gate_vadr BOOLEAN DEFAULT FALSE,          -- VADR ≥ 1.8
    gate_funding BOOLEAN DEFAULT FALSE,       -- Funding divergence ≥ 2σ
    gate_microstructure BOOLEAN DEFAULT FALSE,
    gate_freshness BOOLEAN DEFAULT FALSE,
    gate_fatigue BOOLEAN DEFAULT FALSE,
    
    -- Composite Scoring Results
    score DOUBLE PRECISION CHECK (score >= 0 AND score <= 100),
    momentum_core DOUBLE PRECISION CHECK (momentum_core >= 0 AND momentum_core <= 100),
    social_residual DOUBLE PRECISION CHECK (social_residual IS NULL OR social_residual <= 10),
    
    -- Attribution and Context
    factors JSONB,
    regime TEXT CHECK (regime IN ('trending','choppy','highvol','mixed')),
    processing_latency_ms INTEGER,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    
    UNIQUE (ts, symbol, venue)
);
```

## Configuration

### YAML Configuration

```yaml
database:
  enabled: false  # Enable database persistence
  dsn: ""         # PostgreSQL connection string
  max_open_conns: 10
  max_idle_conns: 5
  conn_max_lifetime: "30m"
  conn_max_idle_time: "5m"
  query_timeout: "30s"
```

### Environment Variables

Database configuration can be overridden via environment variables:

```bash
export PG_ENABLED=true
export PG_DSN="postgres://user:pass@localhost:5432/cryptorun?sslmode=disable"
export PG_MAX_OPEN_CONNS=20
export PG_MAX_IDLE_CONNS=10
export PG_CONN_MAX_LIFETIME=1h
export PG_CONN_MAX_IDLE_TIME=10m
export PG_QUERY_TIMEOUT=1m
```

### Connection String Examples

```bash
# Development (local PostgreSQL)
PG_DSN="postgres://cryptorun:password@localhost:5432/cryptorun?sslmode=disable"

# Production (managed PostgreSQL with SSL)
PG_DSN="postgres://cryptorun:password@db.example.com:5432/cryptorun?sslmode=require"

# TimescaleDB (time-series optimized)
PG_DSN="postgres://cryptorun:password@timescale.example.com:5432/cryptorun?sslmode=require"
```

## Database Migrations

### Prerequisites

Install Goose migration tool:

```bash
go install github.com/pressly/goose/v3/cmd/goose@latest
```

### Running Migrations

```bash
# Apply all pending migrations
goose -dir db/migrations postgres "$PG_DSN" up

# Check migration status
goose -dir db/migrations postgres "$PG_DSN" status

# Rollback last migration
goose -dir db/migrations postgres "$PG_DSN" down
```

### Migration Files

Located in `db/migrations/`:
- `0001_create_trades.sql` - Trade execution logging
- `0002_create_regime_snapshots.sql` - Market regime detection
- `0003_create_premove_artifacts.sql` - Entry gate results and scoring

## Usage Examples

### Basic Setup

```go
package main

import (
    "context"
    "github.com/sawpanic/cryptorun/internal/infrastructure/db"
)

func main() {
    // Load configuration
    config, err := db.LoadAppConfig("config/database.yaml")
    if err != nil {
        panic(err)
    }
    
    // Create database integration
    integration, err := db.NewIntegration(config)
    if err != nil {
        panic(err)
    }
    defer integration.Close()
    
    // Check if database is enabled
    if integration.IsEnabled() {
        // Run migrations (in production)
        if err := integration.RunMigrations(); err != nil {
            log.Warn().Err(err).Msg("Migration check failed")
        }
        
        // Get repository access
        repos := integration.Repository()
        
        // Use repositories...
        trades, err := repos.Trades.GetLatest(context.Background(), 10)
        if err != nil {
            log.Error().Err(err).Msg("Failed to query latest trades")
        }
    }
}
```

### Data Facade Integration

```go
import (
    "github.com/sawpanic/cryptorun/internal/data/facade"
    "github.com/sawpanic/cryptorun/internal/infrastructure/db"
)

func setupDataFacade() facade.DataFacade {
    // Create database integration
    integration, _ := db.NewIntegration(config)
    
    // Create facade with database-aware PIT store
    facadeConfig := facade.Config{
        PITStore: integration.PITStore(), // Dual file/database persistence
        // ... other config
    }
    
    return facade.New(facadeConfig)
}
```

### Repository Usage

```go
func tradeAnalysis(repos *persistence.Repository) error {
    ctx := context.Background()
    
    // Query trades by symbol
    timeRange := persistence.TimeRange{
        From: time.Now().Add(-24 * time.Hour),
        To:   time.Now(),
    }
    
    trades, err := repos.Trades.ListBySymbol(ctx, "BTC/USD", timeRange, 100)
    if err != nil {
        return err
    }
    
    // Query regime snapshots
    regimes, err := repos.Regimes.ListRange(ctx, timeRange)
    if err != nil {
        return err
    }
    
    // Query passing entry gates
    artifacts, err := repos.Premove.ListPassed(ctx, timeRange, 50)
    if err != nil {
        return err
    }
    
    // Process data...
    return nil
}
```

## Health Monitoring

### Health Check Endpoint

```go
func healthHandler(integration *db.Integration) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
        defer cancel()
        
        health := integration.Health(ctx)
        
        if health.Healthy {
            w.WriteHeader(http.StatusOK)
        } else {
            w.WriteHeader(http.StatusServiceUnavailable)
        }
        
        json.NewEncoder(w).Encode(health)
    }
}
```

### Statistics Collection

```go
func collectDatabaseStats(integration *db.Integration) {
    ctx := context.Background()
    stats := integration.Statistics(ctx)
    
    // Export to metrics system
    for key, value := range stats {
        prometheus.GaugeWithLabels("db_stat", map[string]string{"metric": key}).Set(float64(value))
    }
}
```

## File Storage Fallback

The PIT store automatically falls back to file storage when:

1. Database is disabled (`enabled: false`)
2. Database connection fails
3. Database queries encounter errors

### File Structure

```
data/pit/
├── trades/
│   └── 2025/01/07/
│       ├── 14-30-25-kraken.json
│       └── 14-30-26-binance.json
├── premove_artifacts/
│   └── 2025/01/07/
│       └── 14-35-10-scanner.json
└── regime_snapshots/
    └── 2025/01/07/
        └── 16-00-00-regime_detector.json
```

## Performance Considerations

### Connection Pooling

- **Max Open Connections**: 10 (default) - Balance between concurrency and resource usage
- **Max Idle Connections**: 5 (default) - Reduce idle connection overhead  
- **Connection Lifetime**: 30m (default) - Prevent stale connections
- **Query Timeout**: 30s (default) - Prevent runaway queries

### Indexing Strategy

- **PIT Queries**: Symbol + timestamp DESC for latest-first retrieval
- **Venue Analysis**: Venue + timestamp for exchange-specific queries
- **Entry Gates**: Composite index on gate conditions for performance filtering
- **Partial Indexes**: Order ID index only where NOT NULL

### Database Sizing

**Estimated Storage (per 1M records):**

- Trades: ~150MB (with indexes ~300MB)
- Regime Snapshots: ~5MB (4h cadence, minimal data)
- Premove Artifacts: ~200MB (rich attribution data)

**Recommended Hardware:**
- Development: 2 CPU, 4GB RAM, 10GB storage
- Production: 4+ CPU, 8GB+ RAM, 100GB+ storage (SSD recommended)

## Production Deployment

### Database Setup

1. **PostgreSQL 12+** or **TimescaleDB 2.0+** (recommended for time-series)
2. Create database and user:

```sql
CREATE DATABASE cryptorun;
CREATE USER cryptorun WITH PASSWORD 'secure_password';
GRANT ALL PRIVILEGES ON DATABASE cryptorun TO cryptorun;
```

3. Run migrations:

```bash
goose -dir db/migrations postgres "postgres://cryptorun:password@db:5432/cryptorun?sslmode=require" up
```

### Monitoring

- **Connection Pool**: Monitor open/idle connections via health endpoint
- **Query Performance**: Track P99 latency < 100ms for CRUD operations
- **Storage Growth**: Monitor table sizes and index usage
- **Error Rates**: Alert on database connection failures

### Backup Strategy

- **Point-in-Time Recovery**: PostgreSQL WAL-E or pgBackRest
- **Daily Dumps**: `pg_dump` with retention policy
- **Configuration Backup**: Version-controlled YAML configs

## Troubleshooting

### Common Issues

**Connection Refused**
```
failed to ping database: dial tcp: connection refused
```
- Check PostgreSQL is running
- Verify connection string (host, port, credentials)
- Test network connectivity

**Permission Denied**
```
pq: permission denied for table trades
```
- Grant proper privileges to database user
- Check table ownership and permissions

**Migration Failures**
```
goose: no migration files found
```
- Verify migration directory path
- Check file naming convention (NNNN_*.sql)

### Debug Mode

Enable debug logging for database operations:

```go
import "github.com/rs/zerolog/log"

// Set debug level
log.Logger = log.Logger.Level(zerolog.DebugLevel)
```

### Manual Queries

```sql
-- Check recent trades
SELECT symbol, venue, price, qty, ts 
FROM trades 
ORDER BY ts DESC 
LIMIT 10;

-- Current regime
SELECT regime, confidence_score, weights, ts 
FROM regime_snapshots 
ORDER BY ts DESC 
LIMIT 1;

-- Entry gate statistics (last 24h)
SELECT 
    gate_score, gate_vadr, gate_funding,
    COUNT(*) as count
FROM premove_artifacts 
WHERE ts >= NOW() - INTERVAL '24 hours'
GROUP BY gate_score, gate_vadr, gate_funding;
```

## UX MUST — Live Progress & Explainability

The persistence layer provides comprehensive observability:

- **Real-time Health**: `/health` endpoint with connection pool status
- **Live Metrics**: Connection counts, query latencies, error rates
- **Data Attribution**: Every record includes source venue and timestamp  
- **Query Tracing**: Structured logging for all database operations
- **Graceful Degradation**: Automatic fallback to file storage on database issues

All database operations are instrumented with structured logging for full visibility into system behavior and performance characteristics.