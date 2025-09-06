# Menu Documentation

## UX MUST — Live Progress & Explainability

CryptoRun's interactive Menu is the canonical interface for all functionality, providing guided workflows with comprehensive visual progress indicators and explanatory output.

## 🎯 GOVERNANCE: Menu is Canon

**THE INTERACTIVE MENU IS THE PRIMARY INTERFACE**

Every CryptoRun capability must be accessible through the interactive Menu system. This policy enforces:

### Development Requirements
- **Menu-First Delivery**: Any new feature MUST ship with menu screen/panel
- **Unified Routing**: CLI subcommands MUST call same functions as menu actions  
- **Parameter Precedence**: Profile defaults → Menu selections → CLI flags (non-TTY)
- **CI Enforcement**: PRs lacking menu integration are rejected

### TTY Detection & Routing
- **Interactive Terminal**: `cryptorun` → Menu opens automatically
- **Non-interactive Environment**: `cryptorun` → Guidance message + exit(2)
- **Explicit Menu**: `cryptorun menu` → Force menu in any environment

### Quality Assurance
- **Conformance Tests**: Validate CLI/Menu use identical function pointers
- **TTY Tests**: Verify proper routing based on terminal detection
- **Documentation Parity**: Both CLI.md and MENU.md updated with new features

CLI flags and subcommands serve as automation shims for non-interactive environments (CI, scripts, cron) but the Menu remains the primary user experience.

## Menu Structure

```
CryptoRun v3.2.1 - Interactive Menu
=====================================

🎯 SCANNING
   1. Momentum Scanner    - Multi-timeframe momentum with regime adaptation
   2. Quality-Dip Scanner - High-probability pullback identification

📊 BENCHMARKING  
   3. Top Gainers        - Compare results against CoinGecko top gainers
   4. Diagnostics        - P&L simulation with spec-compliant gates/guards

🔧 DATA MANAGEMENT
   5. Universe Builder   - Rebuild USD-only trading universe with ADV filtering
   6. Pairs Sync        - Discover and sync exchange pairs with volume filters

⚗️  QUALITY ASSURANCE
   7. QA Suite          - Comprehensive testing with provider health checks
   8. Self-Test         - Offline resilience validation
   9. Spec Compliance   - Product requirement drift detection

📈 MONITORING & ANALYSIS
   10. HTTP Monitor     - Start /health, /metrics, /decile endpoints
   11. Nightly Digest   - Performance analysis from ledger and summaries
   12. Alert System     - Discord/Telegram notifications with filtering

🚀 RELEASE & PACKAGING
   13. Ship Release     - Validate results quality and prepare PR
   
⚙️  SYSTEM
   14. Settings         - Configure progress modes, venues, cache TTL
   15. Help & Docs      - Access documentation and examples
   16. Exit

=====================================
Select option [1-16]: 
```

## Benchmark and Diagnostics Viewers

### Bench Results Viewer

The Bench menu includes comprehensive viewers for analyzing benchmark artifacts:

**📊 View Benchmark Results**
- **Alignment Overview**: Overall alignment percentage and per-window scores
- **Correlation Metrics**: Kendall's τ, Spearman ρ, and Mean Absolute Error (MAE)
- **Per-Symbol Analysis**: Detailed hit/miss rationale table with rank comparisons
- **File Actions**: Open MD reports, JSON data, and detailed window breakdowns

```
╔═══════════════ BENCHMARK RESULTS ═══════════════╗

📊 Overall Alignment: 67.3%
🕒 Last Updated: 2025-09-06T13:32:48

┌─ 1H Window ─────────────────────────────────┐
│ Alignment: 62.1% (8/20 matches)            │
│ Kendall τ: 0.428                           │
│ Spearman ρ: 0.385                          │
│ MAE: 4.2 positions                         │
└─────────────────────────────────────────────┘
```

### Diagnostics Viewer

**🔍 View Diagnostics** displays comprehensive gate/guard analysis:

- **Guards Breakdown**: Top blocking guards (fatigue, freshness, late-fill)
- **Gates Breakdown**: Top blocking gates (volume, spread, depth, ADX)  
- **Hit/Miss Analysis**: Per-symbol rationale with raw vs spec-compliant P&L
- **File Actions**: Access bench_diag.md, gate_breakdown.json, and detailed analysis

```
╔═══════════════ DIAGNOSTIC ANALYSIS ═══════════════╗

📊 Overall Alignment: 60.0%
🕒 Analysis Time: 2025-09-06T13:35:00

🛡️  Top Guard Blockers:
   • fatigue_guard: 12 blocked
   • freshness_guard: 8 blocked
   • late_fill_guard: 5 blocked

🚪 Top Gate Blockers:
   • volume_surge: 15 blocked
   • spread_check: 10 blocked
   • depth_check: 7 blocked
```

### Per-Symbol Hit/Miss Table

Both viewers provide detailed per-symbol analysis showing:

- **Symbol & Rank**: Asset symbol with gainer rank vs scanner rank
- **Raw vs Spec P&L**: Market percentage vs realistic spec-compliant P&L
- **Status Rationale**: Specific reason for hit/miss classification
- **Actionable Insights**: Configuration recommendations based on missed opportunities

```
✅ HITS (8):
   BTC: Rank 1 → 1 (15.00% gain / 8.2% spec P&L)
      Perfect rank match - top gainer and top scan result
   
❌ MISSES (12):
   ETH: Rank 2 (14.20% gain / 6.1% spec P&L)
      Blocked by fatigue guard - 24h return 18.5% > 18% threshold
```

### File Integration

All viewers include file action buttons:

- **[Open MD Report]**: Opens human-readable markdown analysis
- **[Open JSON Data]**: Opens structured data for automation
- **[View Details]**: In-menu detailed breakdown without external files

Cross-platform file opening support:
- **Windows**: Uses `start` command
- **macOS**: Uses `open` command  
- **Linux**: Uses `xdg-open` command

## Visual Progress Examples

### Scanning: Momentum Pipeline

```
⚡ Momentum Pipeline [████████████░░░░░░░░] 6/8 (75.0%) ETA: 8s
  ✅ Universe: 50 symbols (125ms)
  ✅ Data Fetch: 50/50 symbols, 85% cache hit (2.1s)  
  ✅ Guards: 37/50 passed fatigue+freshness+late-fill (234ms)
  ✅ Factors: 4h momentum computed across timeframes (847ms)
  ✅ Orthogonalize: Gram-Schmidt applied, MomentumCore protected (89ms)
  🔄 Score: Composite scoring with bull regime weights [██████░░░░] 30/37 ETA: 6s

Pipeline Status: Processing candidates with regime-adaptive weights...
Cache Performance: 42 hits, 8 misses (84% hit rate)
Next: Entry gates (volume, spread, depth, ADX validation)
```

### Benchmarking: Top Gainers Analysis

```
🔄 Top Gainers Benchmark [████████████████░░░░] 4/5 (80.0%) ETA: 30s
  ✅ Init: Configuration validated (12ms)
  ✅ Fetch: 1h window (25 gainers, cache hit, 234ms)  
  ✅ Fetch: 24h window (25 gainers, API call, 1.2s)
  📊 Analyze: Computing alignment [████████████░░░░░░░░] 3/5 windows ETA: 25s

Alignment Analysis:
  Symbol Overlap: 68% (17/25 scanner candidates in top gainers)
  Rank Correlation: τ=0.42 (moderate agreement), ρ=0.38 
  Mean Absolute Error: 4.2 positions average deviation
  Composite Score: 0.61 (good alignment)

Cache: TTL 300s remaining, Rate limit: 8/10 rpm available
Source: CoinGecko Free API with exchange-native pricing references
```

### Diagnostics: P&L Simulation

```
🔬 Diagnostic Analysis [██████████████████░░] 18/20 (90.0%) ETA: 12s
  ✅ Price Series: Exchange-native bars retrieved (1.8s)
  ✅ Entry Gates: Applied venue health + volume surge filters (145ms)
  ✅ Exit Logic: Simulated stop-loss, trailing, profit targets (234ms)
  🔄 P&L Calculation: Spec-compliant simulation [████████████████░░░░] 18/20 ETA: 10s

Results Preview:
  Raw Market Gains: ETH +42.8%, SOL +38.4%, ADA +24.1%
  Spec-Compliant P&L: ETH +8.2%, SOL +11.7%, ADA +5.9%
  Missed Opportunities: 3 symbols failed entry gates (spread >50bps)
  Realistic Expectations: Gates/guards reduce gains by ~65% (safety first)

Sample Size: n=18 (meets n≥20 threshold for statistical validity)
```

## Menu Navigation

### Standard Commands
- **Number Selection**: Enter option number (1-16) and press Enter
- **Quick Access**: Type first letter of section (S=Scanning, B=Benchmarking, Q=QA)
- **Back/Exit**: Press 'b' to go back, 'q' or Ctrl+C to quit
- **Help**: Press 'h' for contextual help on current menu level

### Progress Control  
- **Progress Modes**: Settings → Progress Mode → Auto/Plain/JSON/None
- **Venue Configuration**: Settings → Data Sources → Kraken/OKX/Coinbase
- **Cache Control**: Settings → Cache TTL → 300s (default) to 3600s

### Integration Features
- **Copy Commands**: Menu shows equivalent CLI commands for automation
- **Export Results**: All operations save structured output to `out/` directories
- **Log Streaming**: Real-time logs available at `out/logs/menu_session.log`

## Technical Implementation

### Menu State Management
- **Session Persistence**: Menu state saved between operations
- **Context Switching**: Maintains current selections and progress
- **Error Recovery**: Graceful handling of failed operations with retry options

### Progress Integration
- **Unified Progress System**: All menu operations use the same progress indicators
- **Metrics Collection**: Menu actions contribute to Prometheus metrics
- **Performance Tracking**: Response times and success rates logged per feature

### Accessibility
- **Terminal Detection**: Automatic fallback to CLI help in non-TTY environments  
- **Color Support**: Adapts to terminal capabilities (full color, basic, none)
- **Screen Reader**: Plain text mode available for accessibility tools

This Menu system ensures that all CryptoRun functionality remains discoverable, accessible, and consistently presented with comprehensive progress feedback.