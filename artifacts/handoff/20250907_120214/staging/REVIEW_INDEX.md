# CryptoRun Code Review Package

**Generated:** 2025-09-07 12:02:14 UTC  
**Package ID:** 20250907_120214

## What to Read First

**Start here for orientation:**
1. **CLAUDE.md** - Core development guide, commands, architecture overview
2. **docs/DATA_FACADE.md** - Data architecture (hot/warm/cold layers) 
3. **docs/MICROSTRUCTURE.md** - Exchange-native L1/L2 validation
4. **docs/REGIME.md** - Market regime detection and adaptive weights
5. **docs/PREMOVE_VALIDATION.md** - Entry gate validation system

## Build/Test Status

**Build Status: ❌ FAILED**
- Syntax errors in datafacade middleware (imports placement)
- Undefined interfaces (OpenInterestEvent) in adapter implementations
- Import path resolution issues (cryptorun/internal/gates/entry)
- Multiple compilation errors prevent successful build

**Test Status: ⚠️ MIXED**  
- Some test packages pass successfully
- Weight constraint validation failures in tests/unit/tune
- TestRandomWeightGeneration: failed after 100 attempts
- TestEdgeCases: weights validation failed for regime bounds

## Key Design Documents

| Document | Purpose |
|----------|---------|
| **docs/SCHEDULER.md** | Task scheduling and execution framework |
| **docs/GATES.md** | Entry gate validation (Score≥75 + VADR≥1.8) |
| **docs/PREMOVE_VALIDATION.md** | Pre-movement validation with 2-of-3 gates |
| **docs/DATA_FACADE.md** | Data layer architecture (hot/warm/cold) |
| **docs/MICROSTRUCTURE.md** | L1/L2 order book validation system |
| **docs/REGIME.md** | Market regime detection and weight adaptation |
| **docs/OPERATIONS.md** | Operational procedures and monitoring |

## Runbook (Smoke Test Commands)

```bash
# Build from src directory
cd C:\CryptoRun\src
go build ./cmd/cryptorun

# Hot scan dry-run (Kraken USD pairs)
./cryptorun scan --exchange kraken --pairs USD-only --dry-run

# Premove dry-run validation
./cryptorun premove --dry-run

# Health check
./cryptorun health

# Monitor mode (serves /health, /metrics, /decile)
./cryptorun monitor
```

## Microstructure & Safety Disclaimers

⚠️ **Critical Safety Requirements:**

- **Exchange-Native L1/L2 Only**: Never use aggregators (DEXScreener, CoinGecko) for depth/spread data
- **Rate Limits & Circuit Breakers**: All outbound API calls protected with provider-aware limits
- **Social Factor Cap**: Strictly limited to +10 points, applied OUTSIDE 100% weight allocation  
- **Regime-Aware Gating**: Three weight profiles (calm/normal/volatile) with 4h automatic switching
- **Entry Gates**: Hard requirements - Score≥75 + VADR≥1.8 + funding divergence≥2σ
- **Premove Gates**: 2-of-3 validation (A: freshness, B: fatigue, C: late-fill) with documented precedence

## Reviewer Checklist

```
□ Can I build the CLI from src without fiddling?
□ Do tests pass locally? Any flakes?
□ Is factor ordering (MomentumCore protected) and social cap present?
□ Are microstructure gates (spread, depth, VADR) enforced pre-entry?
□ Are RL/CB and budget guards wired on every outbound call?
□ Are premove gates A/B/C implemented per spec with precedence?
□ Is docs ↔ code consistent? Any drift?
```

## Known Limitations/TODOs

**Immediate Issues:**
- **Build Compilation**: Fix syntax errors in datafacade middleware imports
- **Interface Definitions**: Define missing OpenInterestEvent interface  
- **Import Paths**: Resolve Go module path configuration issues
- **Weight Constraints**: Fix random weight generation algorithm in tune package

**Architecture Status:**
- ✅ Unified composite scoring system with protected MomentumCore
- ✅ Gram-Schmidt orthogonalization implemented  
- ✅ Regime-adaptive weight system with 3 profiles
- ✅ Hard entry gates (Score≥75 + VADR≥1.8)
- ❌ Live data connections (uses mocks for testing)
- ❌ Regime detector implementation (manual regime setting)
- ❌ Production deployment configuration

**Completion Estimate:** ~85% core system, ~60% overall project

---

**Review Focus Areas:**
1. Fix compilation errors before functional review
2. Validate weight constraint logic in tune package
3. Verify single pipeline architecture compliance
4. Check microstructure gate implementation completeness
5. Ensure all security redactions are properly applied