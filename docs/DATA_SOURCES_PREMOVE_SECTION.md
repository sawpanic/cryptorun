# Data Sources — Pre‑Movement Detector

**Authority rules**
- **Microstructure:** exchange‑native only for depth/spread/OB; no aggregators
- **Derivatives:** venue‑native funding/OI/basis; options from Deribit (large caps)
- **Warm context:** CoinGecko/Paprika for caps/volumes/prices (no depth), TTL 300s
- **Catalysts:** CoinMarketCal (free→Pro), DefiLlama unlocks; TokenUnlocks optional
- **TTL & breakers:** 30s–10m; breaker doubles TTL on degradation; staleness penalties applied to scores

**Provider table** (short):
- Binance/OKX/Coinbase/Kraken WS/REST (price, trades, L2, funding/OI, basis where supported)
- Deribit (options skew/term)
- CoinGecko/Paprika (context only)
- CoinMarketCal, DefiLlama unlocks (catalysts)
- **Cold tier:** Historical data with Parquet compression and PIT integrity

