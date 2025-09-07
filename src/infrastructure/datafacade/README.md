# CryptoRun Data Facade

## UX MUST — Live Progress & Explainability

The Data Facade provides real-time transparency through comprehensive monitoring and attribution tracking for all market data operations.

## Overview

The Data Facade is CryptoRun's unified data access layer implementing HOT (WebSocket streaming) and WARM (REST + cache) access patterns with comprehensive rate limiting, circuit breaker protection, and point-in-time immutable snapshots.

## Quick Start

```go
import (
    "cryptorun/src/infrastructure/datafacade"
    "cryptorun/src/infrastructure/datafacade/config"
)

// Load configuration
config, err := config.LoadConfig("./config")
if err != nil {
    log.Fatal(err)
}

// Create factory and facade
factory := datafacade.NewFactory(config)
facade, err := factory.CreateDataFacade()
if err != nil {
    log.Fatal(err)
}
defer facade.Shutdown(context.Background())

// Subscribe to real-time trades
ctx := context.Background()
tradesCh, err := facade.SubscribeToTrades(ctx, "binance", "BTC/USDT")
if err != nil {
    log.Fatal(err)
}

// Process trade events
for trade := range tradesCh {
    fmt.Printf("Trade: %s %f @ %f on %s\n", 
        trade.Trade.Side, trade.Trade.Quantity, 
        trade.Trade.Price, trade.Trade.Venue)
}
```

## Architecture

```
Data Facade
├── HOT (WebSocket)     - Real-time streaming data
├── WARM (REST+Cache)   - On-demand cached data  
├── PIT Snapshots       - Immutable historical data
├── Rate Limiting       - Token bucket + budget tracking
├── Circuit Breakers    - Venue health monitoring
└── Multi-venue         - Binance, OKX, Coinbase, Kraken
```

## Key Features

- **Exchange-Native Only**: No aggregators, direct venue APIs only
- **Dual Access Patterns**: HOT streaming + WARM cached REST
- **Provider-Aware Protection**: Rate limiting + circuit breakers
- **Point-in-Time Immutability**: Compressed snapshots with retention
- **Comprehensive Testing**: Unit, integration, and performance tests

## Configuration Files

- `config/cache.yaml` - Redis and TTL configurations
- `config/rate_limits.yaml` - Per-venue rate limiting rules  
- `config/circuits.yaml` - Circuit breaker thresholds
- `config/pit.yaml` - PIT snapshot settings
- `config/venues.yaml` - Exchange connection details

## Testing

```bash
# Run all tests
go test ./... -v

# Run with race detection
go test ./... -race

# Run benchmarks
go test ./... -bench=.
```

## Documentation

See [DATA_FACADE.md](../../docs/DATA_FACADE.md) for comprehensive documentation including:

- Configuration examples
- API reference
- Performance characteristics
- Error handling patterns
- Monitoring and observability
- Best practices

## License

Part of the CryptoRun project.