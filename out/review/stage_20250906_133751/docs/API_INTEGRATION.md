# API Integration

Providers: Kraken (primary), DEXScreener, Binance, CoinGecko, Moralis, CMC, CoinPaprika.
- DEXScreener: 60 rpm per endpoint limiter
- Binance: reference-only weight throttler via X-MBX-USED-WEIGHT
- Monthly budget guards: switch providers at $1k remaining
- Redis caching with tiered TTLs
