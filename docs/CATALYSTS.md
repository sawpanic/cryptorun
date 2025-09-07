# Catalyst Compression Factor

## UX MUST — Live Progress & Explainability

Real-time catalyst compression detection combining Bollinger Band width analysis with time-decayed catalyst event scoring: mathematical precision for squeeze detection, exponential time-decay multipliers, and comprehensive PIT integrity for deterministic catalyst-driven momentum signals.

**Last Updated:** 2025-09-07  
**Version:** v1.0.0 CatalystCompression Integration  
**Breaking Changes:** New 6th factor added to unified composite scoring pipeline

The CatalystCompression factor integrates market compression analysis (Bollinger Band width) with time-decayed catalyst events to detect accumulation phases that precede momentum breakouts in the 6-48 hour trading horizon.

## Mathematical Foundation

### Bollinger Band Compression

**BB Width Calculation:**
```
BB_Width = (BB_Upper - BB_Lower) / BB_Middle

Where:
BB_Upper = SMA(20) + 2.0 × StdDev(20)
BB_Lower = SMA(20) - 2.0 × StdDev(20) 
BB_Middle = SMA(20)
```

**Compression Score (Inverted Z-Score):**
```
Compression_Score = (Mean_Width - Current_Width) / StdDev_Width
Normalized = (clamp(Z_Score, -3, 3) + 3) / 6

High compression → Low BB width → High compression score (0-1)
```

### Keltner Channel Squeeze Detection

**Keltner Channels:**
```
Keltner_Upper = EMA(20) + 1.5 × ATR(14)
Keltner_Lower = EMA(20) - 1.5 × ATR(14)
Keltner_Middle = EMA(20)
```

**Squeeze State:**
```
In_Squeeze = (BB_Upper ≤ Keltner_Upper) AND (BB_Lower ≥ Keltner_Lower)
```

### Time-Decay Catalyst Events

**Exponential Time Decay:**
```
Weight = BaseWeight × e^(-ln(2) × |t| / HalfLife) × Confidence

Where:
BaseWeight = Tier multiplier (Imminent: 1.2, NearTerm: 1.0, Medium: 0.8, Distant: 0.6)
HalfLife = Tier-specific decay period
Confidence = Event confidence score (0.0-1.0)
```

**Tier Half-Lives:**
- **Imminent**: 2 days (events 0-7 days away)
- **Near-term**: 7 days (events 7-30 days away)  
- **Medium**: 14 days (events 30-90 days away)
- **Distant**: 30 days (events 90+ days away)

### Final Score Combination

**Weighted Combination:**
```
CatalystScore = 0.6 × CompressionScore + 0.4 × CatalystEventSignal

Where:
CompressionScore ∈ [0, 1] (BB width compression)
CatalystEventSignal ∈ [0, 1] (time-decayed events)
```

## Event Registry Architecture

### Event Storage Structure

```go
type CatalystEvent struct {
    ID          string    // Unique identifier
    Symbol      string    // Asset symbol (e.g., "BTCUSD")
    Title       string    // Human-readable title
    Description string    // Detailed description
    EventTime   time.Time // When the event occurs/occurred
    Tier        EventTier // Impact tier (Imminent/NearTerm/Medium/Distant)
    Source      string    // Data source (must respect robots.txt)
    Confidence  float64   // 0.0-1.0 confidence in event
    CreatedAt   time.Time // When event was added to registry
    UpdatedAt   time.Time // Last update time
    Tags        []string  // Optional tags for categorization
}
```

### Event Aggregation with Logarithmic Scaling

**Signal Calculation:**
```
TotalWeight = Σ(EventWeight_i)
MaxPossibleWeight = ImmientBase × EventCount

Signal = log(1 + TotalWeight) / log(1 + MaxPossibleWeight)
Final_Signal = clamp(Signal, 0, 1)
```

**Logarithmic scaling prevents single events from dominating the signal while preserving relative importance.**

### PIT (Point-in-Time) Integrity

**Temporal Consistency:**
- Events only contribute to signals after their `CreatedAt` time
- Historical queries return different results based on query time
- No retroactive event modifications affect past calculations

**Data Cleanup:**
- Events outside relevance windows are automatically purged
- Maximum events per symbol: 50 (configurable)
- Relevance windows: 90 days future, 30 days past

## Regime Integration

### Catalyst Weight Allocation by Regime

**Trending Bull (Default):**
- Catalyst Block: 10% (focus on momentum continuation)
- Lower catalyst weight as momentum already established

**Choppy Markets:**  
- Catalyst Block: 15% (higher compression value)
- Catalyst events more predictive in uncertain conditions

**High Volatility:**
- Catalyst Block: 11% (reduced reliability) 
- Compression less meaningful in volatile conditions

### Orthogonalization Position

**5th Factor in Gram-Schmidt Sequence:**
```
CatalystResidual = Catalyst - proj(Catalyst onto [MomentumCore, Technical, Volume, Quality])
```

**Independence Verification:**
- |ρ(CatalystResidual, MomentumCore)| < 0.1 (momentum protection)
- |ρ(CatalystResidual, OtherResiduals)| < 0.6 (factor independence)

## Configuration Parameters

### Bollinger Band Settings
```yaml
bb_period: 20           # SMA and StdDev period
bb_std_dev: 2.0        # Standard deviation multiplier
```

### Keltner Channel Settings
```yaml
keltner_period: 20      # EMA period for middle line
keltner_multiplier: 1.5 # ATR multiplier for channel width
atr_period: 14         # ATR calculation period
```

### Compression Analysis
```yaml
compression_lookback: 50  # Historical periods for z-score normalization
compression_clamp: 3.0    # Maximum z-score for normalization
```

### Event Registry Configuration
```yaml
decay_half_life: 168h     # 7 days default half-life
max_look_ahead: 2160h     # 90 days future event horizon
max_look_behind: 720h     # 30 days past event relevance
max_sources_per_symbol: 50 # Event count limit per symbol
respect_robots_txt: true   # Honor robots.txt (always true)
```

## Data Sources & Compliance

### Approved Sources for Catalyst Events
- **CoinGecko** (calendar, major events)
- **Exchange native calendars** (Binance, OKX, Kraken announcements)
- **GitHub** (protocol repositories for technical events)
- **Official project announcements** (Twitter, Medium, official sites)

### Prohibited Sources
- **Paid APIs** (must use free tier only)
- **Aggregator data** (DEXScreener, DeFiPulse, etc.)
- **Social scraping** (beyond official announcements)
- **Insider information** (non-public data)

### robots.txt Compliance
- All HTTP requests respect robots.txt
- Rate limiting per domain: 1 request/second default
- User-Agent identification: "CryptoRun/1.0 (+https://github.com/sawpanic/cryptorun)"

## Performance Characteristics

### Computational Complexity
- **BB Calculation**: O(n) for n price points
- **Compression Score**: O(k) for k historical periods  
- **Event Registry**: O(m) for m events per symbol
- **Total**: O(n + k + m) per symbol

### Benchmark Performance
```
CatalystCompressionCalculation: ~877ns per operation
CatalystEventRegistry lookup:   ~8μs per operation (100 events)
Memory usage:                  <1MB for 1000 symbols × 50 events
```

### Cache Strategy
- **Event Registry**: 10-minute TTL (events change infrequently)
- **Compression Calculations**: No caching (real-time price dependent)
- **Historical BB Widths**: 1-hour TTL for z-score normalization

## Quality Assurance

### Mathematical Validation
- **BB Width Non-negative**: Always (Upper - Lower) ≥ 0
- **Compression Score Bounds**: Always ∈ [0, 1] 
- **Time Decay Monotonicity**: Weight decreases as |time_delta| increases
- **Signal Aggregation Bounds**: Final signal ∈ [0, 1]

### PIT Testing Requirements
- **Deterministic Results**: Same inputs → identical outputs (±1e-10 tolerance)
- **Temporal Consistency**: Historical queries return period-appropriate data
- **Event Ordering**: Events only visible after creation time

### Integration Testing
- **Orthogonalization**: Verify factor independence after Gram-Schmidt
- **Weight Allocation**: Confirm catalyst_block weights applied correctly
- **Score Contribution**: Validate catalyst contribution to composite score

## Implementation Examples

### Basic Usage
```go
// Initialize calculator
config := factors.DefaultCatalystCompressionConfig()
calculator := factors.NewCatalystCompressionCalculator(config)

// Prepare input data
input := factors.CatalystCompressionInput{
    Close:        closePrices,    // 60+ points for proper analysis
    TypicalPrice: typicalPrices,  // (H+L+C)/3
    High:         highPrices,     // For ATR calculation
    Low:          lowPrices,      // For ATR calculation
    Volume:       volumeData,     // Optional for future enhancements
    Timestamp:    []int64{now},   // Current time for catalyst lookup
}

// Calculate catalyst compression
result, err := calculator.Calculate(input)
```

### Event Registry Usage
```go
// Initialize registry
config := catalyst.DefaultRegistryConfig()
registry := catalyst.NewEventRegistry(config)

// Add catalyst event
event := catalyst.CatalystEvent{
    ID:          "btc_halving_2024",
    Symbol:      "BTCUSD", 
    Title:       "Bitcoin Halving",
    EventTime:   halvingDate,
    Tier:        catalyst.TierImminent,
    Confidence:  0.95,
}
registry.AddEvent(event)

// Get aggregated signal
signal := registry.GetCatalystSignal("BTCUSD", time.Now())
```

### Integration with Unified Scoring
```go
// In UnifiedScorer.calculateCatalystFactors()
compressionInput := factors.CatalystCompressionInput{
    Close:        input.PriceClose,
    TypicalPrice: input.PriceTypical,
    High:         input.PriceHigh,
    Low:          input.PriceLow,
    Volume:       input.Volume,
    Timestamp:    []int64{input.Timestamp.Unix()},
}

result, err := us.catalystCalculator.Calculate(compressionInput)
catalystSignal := us.catalystRegistry.GetCatalystSignal(input.Symbol, input.Timestamp)

// Return factors for orthogonalization
return []float64{
    result.CompressionScore,  // BB width compression (0-1)
    squeezeFloat,            // Squeeze state (0 or 1)
    catalystSignal.Signal,   // Time-decayed catalyst signal (0-1)
}
```

## Troubleshooting

### Common Issues

**Insufficient Data Points:**
- Error: "insufficient data for compression analysis (need 50, got X)"
- Solution: Ensure at least 60 price points for proper BB width analysis

**Zero Compression Score:**
- Cause: Insufficient historical data or constant price series  
- Solution: Verify price data quality and historical depth

**High Correlation with Other Factors:**
- Check: Correlation matrix after orthogonalization
- Expected: |ρ(CatalystResidual, others)| < 0.6

**Event Registry Empty Results:**
- Verify: Event creation time vs query time
- Check: Event relevance windows (90d future, 30d past)

### Performance Optimization

**Memory Usage:**
- Limit events per symbol to 50
- Use cleanup routines to purge expired events
- Cache historical BB width calculations

**Computation Speed:**
- Pre-compute ATR and EMA for multiple timeframes
- Batch process multiple symbols together
- Use efficient z-score normalization with rolling statistics

This catalyst compression system provides mathematically sound, PIT-compliant catalyst event analysis integrated seamlessly into the unified composite scoring pipeline while maintaining strict independence through orthogonalization.