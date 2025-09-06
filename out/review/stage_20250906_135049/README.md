# CryptoRun

## 🏃‍♂️ Real-time Cryptocurrency Momentum Scanner

**CryptoRun** is a real-time **6–48h cryptocurrency momentum scanner** powered by free, keyless exchange-native APIs. Designed to deliver **explainable trading signals** with strong safeguards: freshness, fatigue, and late-fill guards, microstructure validation, regime awareness, and strict conformance tests.

### Promise

* *Never chase late entries* → freshness & late-fill guards.
* *Never size what can't be exited* → depth/spread/VADR enforcement.
* *Never let hype outrank price/volume* → capped social factor.
* *Never break under load* → provider-aware rate limits + circuit breakers.
* *Always transparent* → attribution fields in outputs.

### Quick Start

```bash
# Build
go build ./src/cmd/cryptorun

# Scan for momentum opportunities
./cryptorun scan --exchange kraken --pairs USD-only --dry-run

# Monitor system health
./cryptorun monitor

# Run self-diagnostics
./cryptorun selftest

# Generate performance digest
./cryptorun digest --date 2025-09-01
```

### Architecture

Built in Go with a clean layered architecture:

- **`domain/`**: Business logic (scoring, gates, orthogonalization, regime detection)
- **`application/`**: Use cases (universe builders, factor builders, snapshot store)
- **`infrastructure/`**: External integrations (Kraken APIs, circuit breakers, rate limiting)
- **`interfaces/`**: HTTP endpoints (`/health`, `/metrics`, `/decile`)
- **`cmd/cryptorun/`**: CLI entry point with commands: scan, backtest, monitor, health

### Key Features

- **6-48 hour momentum scanner**: Not HFT, not buy-and-hold
- **Exchange-native only**: Never use aggregators for depth/spread data
- **Kraken USD pairs only**: Primary data source with rate limiting
- **Regime-adaptive**: Weights change based on market conditions
- **Orthogonal factors**: Gram-Schmidt orthogonalization to avoid correlation
- **Circuit breakers**: Provider-aware fallbacks and rate limit handling

### Documentation

- [Build Instructions](docs/BUILD.md)
- [API Integration](docs/API_INTEGRATION.md) 
- [Deployment Guide](docs/DEPLOYMENT.md)
- [Monitoring Setup](docs/MONITORING.md)
- [Usage Examples](docs/USAGE.md)

### Development

See [CLAUDE.md](CLAUDE.md) for detailed development guidelines and commands.

## Naming History

This project was previously known as "CProtocol" in older documentation and changelog entries dated before 2025-09-01. Historical references to "CProtocol" in pre-2025-09-01 changelog entries and legacy documents are preserved for historical accuracy.

---

**CryptoRun** - Real-time cryptocurrency momentum scanning with explainable signals.