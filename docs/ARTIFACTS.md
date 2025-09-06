# 🗄️ CryptoRun Artifact Management

**Artifact retention, compaction, and garbage collection system for CryptoRun verification outputs.**

## Overview

CryptoRun generates extensive verification artifacts during GREEN-WALL operations, benchmarking, and backtesting. The artifact management system provides automated retention policies, safe compaction, and garbage collection to maintain optimal disk usage while preserving critical data.

## Artifact Families

The system manages five key artifact families:

### 📊 **Proofs** (microstructure proofs)
- **Sources**: GREEN-WALL microstructure validation
- **Files**: `**/microstructure/proofs/*.jsonl`, `**/microstructure/proofs/*.md`
- **Retention**: Keep last 10 runs
- **Contains**: Spread verification, depth analysis, VADR calculations

### ⚡ **Bench** (performance benchmarks)  
- **Sources**: TopGainers benchmarks, P99 latency tests
- **Files**: `**/bench/*.jsonl`, `**/bench/*.md`, `**/bench/p99_*.json`
- **Retention**: Keep last 10 runs
- **Contains**: Performance metrics, latency distributions, throughput analysis

### 🧪 **Smoke90** (backtest results)
- **Sources**: 90-day smoke backtests
- **Files**: `**/smoke90/*.jsonl`, `**/smoke90/*.md`, `**/smoke90/backtest_*.json`
- **Retention**: Keep last 8 runs
- **Contains**: Backtest results, performance attribution, risk metrics

### 📈 **Explain** (factor explanation deltas)
- **Sources**: Factor analysis and explanation outputs
- **Files**: `**/explain_delta/*.jsonl`, `**/explain_delta/*.md`
- **Retention**: Keep last 12 runs
- **Contains**: Factor breakdowns, explanation deltas, attribution analysis

### ✅ **Greenwall** (complete verification runs)
- **Sources**: Full GREEN-WALL verification suites
- **Files**: `**/greenwall/*.jsonl`, `**/greenwall/*.md`, `**/greenwall/verification_*.json`
- **Retention**: Keep last 12 runs  
- **Contains**: Complete verification reports, all sub-component results

## Configuration

Artifact management is configured via `config/artifacts.yaml`:

```yaml
# Retention policies
retention:
  proofs:   {keep: 10, pin: []}
  bench:    {keep: 10, pin: []}  
  smoke90:  {keep: 8,  pin: []}
  explain:  {keep: 12, pin: []}
  greenwall:{keep: 12, pin: []}

# Safety rules - always kept regardless of retention count
gc:
  always_keep:
    - "last_pass"    # Most recent PASS for each family
    - "pinned"       # Manually pinned artifacts  
    - "last_run"     # Most recent run (pass or fail)
```

## CLI Commands

### List Artifacts

```bash
# List all artifacts
cryptorun artifacts list

# List specific family
cryptorun artifacts list --family proofs

# JSON output for scripts
cryptorun artifacts list --json

# Verbose output with paths
cryptorun artifacts list --verbose --family greenwall
```

**Example Output:**
```
ID           FAMILY     TIMESTAMP           SIZE     STATUS PINNED FILES
a1b2c3d4     proofs     2025-09-06 14:30:22 2.1M     pass*+ 📌     3
e5f6g7h8     bench      2025-09-06 14:25:15 856K     fail*         2  
i9j0k1l2     smoke90    2025-09-06 14:20:08 4.3M     pass          5

Legend: * = last run, + = last pass, 📌 = pinned
```

### Garbage Collection

```bash
# Dry run (default) - show what would be deleted
cryptorun artifacts gc --dry-run

# Actually perform deletions
cryptorun artifacts gc --apply

# Skip confirmation prompts
cryptorun artifacts gc --apply --force
```

**GC Plan Output:**
```
GC Plan Summary (DryRun: true)
Total Entries: 47
To Delete: 12 entries (8.4M, 23 files)
To Keep: 35 entries
Safety: 5 pinned, 5 last-pass, 5 last-run kept

By Family:
  proofs: keep 10/15, delete 2.1M
  bench: keep 8/12, delete 3.2M  
  smoke90: keep 8/8, delete 0B
  explain: keep 9/12, delete 3.1M
```

### Compaction

```bash
# Preview compaction (dry run)
cryptorun artifacts compact --family proofs

# Actually compact files
cryptorun artifacts compact --apply --verbose

# Compact all families
cryptorun artifacts compact --apply
```

**Compaction Features:**
- **JSONL**: Dictionary compression for repeated field values
- **Markdown**: Remove empty sections, canonicalize headers
- **Checksums**: Verify integrity before and after compaction
- **Schema preservation**: Always preserve first record structure

### Pin Management

```bash
# Pin artifact to prevent deletion
cryptorun artifacts pin --id a1b2c3d4 --on

# Unpin artifact  
cryptorun artifacts pin --id a1b2c3d4 --off
```

### Manifest Scanning

```bash
# Scan and rebuild manifest
cryptorun artifacts scan --verbose

# Force rescan even if manifest is recent
cryptorun artifacts scan --force
```

## Safety Features

### 🛡️ **Protected Artifacts**
The system **never** deletes:
1. **Pinned artifacts**: Manually protected via pin command
2. **Last PASS**: Most recent successful run for each family
3. **Last run**: Most recent run regardless of pass/fail status

### 🔄 **Atomic Operations**
- Files moved to trash before final deletion
- Checksums verified before/after operations  
- Transactional operations with rollback capability
- Backup manifest created before modifications

### 📋 **Audit Trail**
- Complete GC reports written to `./artifacts/.trash/`
- Operation timestamps and file counts logged
- Error details preserved for troubleshooting
- Manifest backup maintained automatically

## File Layout

```
./artifacts/
├── .manifest.json          # Artifact index and metadata
├── .manifest.backup.json   # Backup manifest
├── .trash/                 # Deleted files (30-day retention)
│   ├── gc_report_20250906_143022.md
│   └── {entry-id}/         # Organized by artifact ID
├── microstructure/
│   └── proofs/            # Microstructure proof artifacts
├── bench/                 # Performance benchmark outputs  
├── smoke90/              # 90-day backtest results
├── explain_delta/        # Factor explanation analysis
└── greenwall/           # Complete GREEN-WALL runs
```

## Integration with GREEN-WALL

The artifact management system integrates seamlessly with GREEN-WALL verification:

```bash
# Run verification and manage artifacts
cryptorun verify all --n 30 --progress
cryptorun artifacts gc --apply      # Clean up old artifacts
cryptorun artifacts compact --apply # Compress remaining files
```

This workflow ensures optimal disk usage while maintaining verification history.

## Advanced Configuration

### Custom Retention Policies

```yaml
retention:
  proofs:
    keep: 15                    # Keep more proof runs
    pin: ["a1b2c3d4", "e5f6g7h8"] # Pin specific high-value runs

  bench:
    keep: 5                     # Keep fewer bench runs  
    pin: []
```

### Compaction Tuning

```yaml
compaction:
  jsonl:
    enabled: true
    min_size_kb: 100           # Only compact files >100KB
    dict_threshold: 5          # Dictionary compression threshold
    
  markdown:
    enabled: true
    min_size_kb: 50           # Only compact files >50KB
    remove_empty_sections: true
    canonical_headers: true
```

### Performance Tuning

```yaml
indexing:
  parallel_workers: 8         # Scanner parallelism
  checksum_buffer_size_kb: 128 # Checksum computation buffer
  max_files_per_scan: 50000   # Safety limit for large repos
```

## Monitoring and Alerting

### Disk Usage Monitoring

```bash
# Check artifact disk usage
cryptorun artifacts list | grep -E "Total|MB|GB"

# Monitor manifest size
ls -lh ./artifacts/.manifest.json
```

### Integration with CI/CD

```yaml
# Example GitHub Actions workflow
- name: Cleanup Artifacts
  run: |
    ./cryptorun artifacts gc --apply --force
    ./cryptorun artifacts compact --apply
    
- name: Upload Critical Artifacts  
  if: failure()
  uses: actions/upload-artifact@v3
  with:
    name: greenwall-failure-artifacts
    path: ./artifacts/greenwall/verification_*.json
```

## UX MUST — Live Progress & Explainability

The artifact management system provides comprehensive explainability:

- **Real-time progress**: All operations show live progress and file counts
- **Clear attribution**: Every decision (keep/delete) includes detailed reasoning
- **Safety confirmation**: Dry-run by default with explicit confirmation required
- **Audit trail**: Complete operation history with rollback capabilities
- **Family breakdown**: Clear categorization and per-family statistics

Users can understand exactly what artifacts exist, why retention decisions are made, and safely manage large verification histories with confidence.

## Troubleshooting

### Common Issues

**Q: Artifact not found after GC**
A: Check if it was pinned or marked as last-pass. Review GC report in `.trash/` directory.

**Q: Compaction not reducing file size**
A: Increase `dict_threshold` for JSONL or check if files contain unique data.

**Q: Manifest out of sync**
A: Run `cryptorun artifacts scan --force` to rebuild from filesystem.

**Q: GC plan validation failed**
A: Usually indicates attempt to delete protected artifacts. Check pinned status.

### Recovery Procedures

**Restore deleted artifacts:**
```bash
# Files in trash for 30 days - manual restore possible
ls ./artifacts/.trash/{artifact-id}/
cp -r ./artifacts/.trash/{artifact-id}/* ./artifacts/
```

**Restore manifest from backup:**
```bash
cp ./artifacts/.manifest.backup.json ./artifacts/.manifest.json
cryptorun artifacts scan --force  # Rebuild if needed
```

---

*For additional support, see the [GREEN-WALL Verification Guide](VERIFY.md) and [Engineering Documentation](DOCUMENTATION_PROTOCOL.md).*