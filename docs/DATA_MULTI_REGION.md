# CryptoRun Multi-Region Replication Architecture

## Overview

CryptoRun implements multi-region replication across hot/warm/cold data tiers to ensure high availability, disaster recovery, and geographic distribution for cryptocurrency momentum scanning operations.

## Topology & Network Assumptions

### Regions
- **Primary**: `us-east-1` (North Virginia) - Primary ingestion and processing
- **Secondary**: `us-west-2` (Oregon) - West coast failover and load distribution  
- **Tertiary**: `eu-west-1` (Ireland) - European operations and compliance

### Network Assumptions
- **Inter-region RTT**: 50-150ms typical, 300ms worst case
- **Clock Synchronization**: NTP with ±100ms tolerance
- **Bandwidth**: 10Gbps dedicated inter-region links
- **Partition Tolerance**: Designed for temporary split-brain scenarios

## Tier-Specific Replication Strategies

### Hot Tier (Real-time WebSocket Data)
**Strategy**: Active-Active with Local Authority

```yaml
Mode: active-active
Authority: local-ingestion-wins
Reconciliation: anti-entropy-every-4h
Conflict Resolution: timestamp_wins
SLO: <500ms replication lag
```

**Architecture**:
- Each region maintains independent WebSocket connections to exchanges
- Local ingestion is authoritative for locally received data
- Anti-entropy reconcilers sync deltas every 4 hours
- Sequence gap detection triggers immediate backfill

**Failure Modes**:
- Regional WebSocket failure → failover to nearest healthy region
- Split-brain → continue local operation, reconcile on reconnect
- Clock drift → timestamp conflict resolution with NTP sync alerts

### Warm Tier (Cached/Aggregated Data)  
**Strategy**: Active-Passive with Planned Failover

```yaml
Mode: active-passive  
Authority: primary-region-wins
Replication: async-batch-60s
SLO: <60s replication lag
```

**Architecture**:
- Primary region (`us-east-1`) processes all warm tier computations
- Secondary regions receive read-only replicas via batch sync
- Promotion requires operator approval for data consistency
- Cache invalidation propagated to all regions

**Failure Modes**:
- Primary failure → promote secondary with data validation
- Replication lag → alert at >90s, emergency promote at >300s
- Partial failure → degraded read-only mode until recovery

### Cold Tier (Historical Files)
**Strategy**: Active-Passive with Automated Backfill

```yaml
Mode: active-passive
Authority: primary-region-wins  
Replication: async-file-sync-5m
SLO: <5m replication lag for new files
Backfill: automated-missing-detection
```

**Architecture**:
- Primary region writes all historical Parquet/CSV files
- File-level replication with integrity checksums
- Missing file detection triggers automatic backfill
- Point-in-time integrity maintained across regions

## Source Authority & Cascades

### Authority Hierarchy
1. **Exchange-Native APIs** (Kraken, Binance, OKX, Coinbase)
2. **Local Regional Ingestion** (for hot tier)
3. **Primary Region Processing** (for warm/cold tiers)
4. **Cross-Region Backfill** (for disaster recovery)

### Cascade Rules
- **Best-Feed-Wins**: Lowest latency, highest confidence score
- **Freshness Penalty**: Stale data (>60s) marked with provenance warning
- **Venue Fallback**: Kraken → OKX → Binance → Coinbase (USD pairs only)

## Failure Classes & Recovery

### Regional Loss (Complete)
**Detection**: Health checks fail for >60s across all services
**Response**: 
1. Promote healthy region to primary 
2. Redirect traffic via DNS/load balancer
3. Preserve unflushed data in persistent queues
4. Begin cross-region delta reconciliation

**Recovery**:
1. Validate system clock synchronization
2. Compare data checksums and sequence numbers
3. Replay missing events from healthy regions  
4. Verify point-in-time integrity before rejoining

### Partial Provider Outage
**Detection**: Specific exchange API failures or degraded performance
**Response**:
1. Activate backup exchange connections in same region
2. Fall back to cross-region exchange feeds if necessary
3. Mark data with provenance tags for quality tracking
4. Maintain separate lag metrics per provider

### Split-Brain Scenario
**Detection**: Network partition prevents cross-region communication
**Response**:
1. Continue local operations with conflict markers
2. Queue all changes for post-partition reconciliation  
3. Expose "partition mode" status via health endpoints
4. Alert operators for manual intervention if >30min

**Reconciliation**:
1. Timestamp-based conflict resolution (NTP synchronized)
2. Last-writer-wins for configuration changes
3. Merge-based resolution for analytical data
4. Manual review queue for critical conflicts

### Clock Drift
**Detection**: NTP sync alerts or timestamp anomaly detection  
**Response**:
1. Mark affected data with timing uncertainty flags
2. Use sequence numbers as secondary ordering mechanism
3. Trigger NTP re-sync and drift measurement
4. Quarantine data if drift >10s until clock stabilizes

## SLO & Metrics Table

| Tier | Metric | Target | Warning | Critical | Recovery |
|------|--------|---------|---------|-----------|----------|
| Hot | Replication Lag | <500ms | >1s | >5s | <30s |
| Hot | WebSocket Gaps | 0% | >0.1% | >1% | <60s |
| Warm | Batch Sync Lag | <60s | >90s | >300s | <120s |  
| Warm | Cache Hit Rate | >90% | <85% | <70% | <300s |
| Cold | File Sync Lag | <5m | >10m | >30m | <60m |
| Cold | Integrity Errors | 0% | >0.01% | >0.1% | <24h |
| All | Cross-Region RTT | <150ms | >200ms | >500ms | N/A |
| All | Clock Skew | <100ms | >500ms | >1s | <60s |

## Prometheus Metrics

```prometheus
# Replication lag by tier and region
cryptorun_replication_lag_seconds{tier="hot|warm|cold",region="us-east-1",source="kraken"}

# Replication plan execution
cryptorun_replication_plan_steps_total{tier="warm",from="us-east-1",to="us-west-2"}
cryptorun_replication_step_failures_total{tier="cold",from="us-east-1",to="eu-west-1",reason="network"}

# Data consistency monitoring  
cryptorun_data_consistency_errors_total{check="schema|staleness|anomaly|corrupt"}
cryptorun_quarantine_total{tier="hot",region="us-west-2",kind="timestamp_skew"}

# Cross-region health
cryptorun_region_health_score{region="us-east-1"} # 0.0-1.0
cryptorun_cross_region_rtt_seconds{from="us-east-1",to="eu-west-1"}
```

## Recovery Playbook

### Hot Tier Failover (WebSocket)
1. **Detection**: WebSocket connection failures >30s
2. **Response**: Activate standby connections in backup region
3. **Validation**: Verify sequence continuity and gap detection
4. **Rollback**: Original region healthy + gap backfill complete

### Warm Tier Promotion  
1. **Detection**: Primary region unhealthy >60s
2. **Preparation**: Flush in-flight batches and sync lag <90s
3. **Promotion**: Update DNS and activate secondary processing  
4. **Validation**: Verify cache consistency and computation accuracy
5. **Demotion**: Original region passes health checks + data validation

### Cold Tier Disaster Recovery
1. **Detection**: Primary region data loss or >30m file sync lag
2. **Assessment**: Identify missing files and integrity gaps
3. **Backfill**: Automated recovery from secondary regions  
4. **Verification**: Checksum validation and point-in-time integrity
5. **Reconciliation**: Merge any new files created during outage

### Split-Brain Resolution
1. **Detection**: Cross-region communication loss >30min
2. **Inventory**: Queue all conflicting changes by region and timestamp
3. **Resolution**: Apply timestamp-wins policy with manual review queue
4. **Validation**: Run integrity checks and anomaly detection post-merge
5. **Monitoring**: Enhanced alerting until normal operation confirmed

## Configuration Reference

Multi-region settings are configured in `config/data_sources.yaml`:

```yaml
cold:
  streaming:
    replication:
      enable: true
      primary_region: "us-east-1"
      secondary_regions: ["us-west-2", "eu-west-1"]
      conflict_resolution: "timestamp_wins"
      policies:
        active_active:
          lag_threshold_ms: 500
        active_passive:  
          lag_threshold_ms: 5000
      failover:
        unhealthy_timeout: "60s"
        error_rate_threshold: 0.05
```

## Operational Commands

```bash
# Check replication status
cryptorun replication status --tier warm --region us-east-1

# Simulate failover without execution  
cryptorun replication simulate --from eu-central --to us-east --tier warm --window 2025-01-01T00:00:00Z/2025-01-01T06:00:00Z

# Execute planned failover
cryptorun replication failover --tier warm --promote us-east --demote eu-central --dry-run=false
```

## Security & Compliance

- **Data Residency**: EU data remains in `eu-west-1` for GDPR compliance
- **Encryption**: TLS 1.3 for all cross-region traffic  
- **Authentication**: mTLS certificates for inter-region API calls
- **Audit**: All failover operations logged with operator attribution
- **Retention**: Disaster recovery logs kept for 2 years minimum

## Performance Considerations

- **Bandwidth Usage**: ~100MB/hour per region for typical operations
- **CPU Impact**: <5% overhead for replication monitoring and sync
- **Storage**: 3x replication factor increases storage requirements  
- **Latency**: Cross-region reads add 50-150ms vs local reads

## Validation & Data Quality

CryptoRun includes comprehensive validation layers to ensure data integrity during replication:

### Schema Validation
- **Required Fields**: Validates presence and types of critical fields (timestamp, price, volume)
- **Field Patterns**: Regex validation for structured fields (e.g., symbol format: BTC-USD)
- **Range Validation**: Ensures numeric fields fall within expected ranges
- **Configuration**: See `internal/data/validate/schema.go`

### Staleness Detection
- **Tier-Specific Thresholds**: Hot (5s), Warm (60s), Cold (5m)
- **Clock Skew Tolerance**: Configurable tolerance for distributed systems
- **Multiple Timestamp Support**: Fallback to alternative timestamp fields
- **Configuration**: See `internal/data/validate/staleness.go`

### Anomaly Detection
- **MAD-based Scoring**: Uses Median Absolute Deviation for robust outlier detection
- **Spike Detection**: Volume surge detection with configurable thresholds
- **Corruption Detection**: Identifies NaN, infinite, or invalid values
- **Quarantine System**: Automatic quarantine of critical anomalies
- **Configuration**: See `internal/data/validate/anomaly.go`

## Troubleshooting Guide

### Common Issues

#### High Replication Lag
```bash
# Check current lag
cryptorun replication status --tier warm --all

# Identify bottlenecks
cryptorun replication status --format prometheus | grep lag

# Simulate recovery plan
cryptorun replication simulate --from primary --to lagging-region --tier warm
```

**Root Causes:**
- Network congestion between regions
- Target region resource constraints
- Large data volumes during peak hours
- Failed validation checks blocking replication

**Resolution:**
1. Check network connectivity between regions
2. Verify target region disk space and CPU
3. Review validation error counts: `cryptorun replication status --format json`
4. Consider temporary tier-specific threshold adjustments

#### Split-Brain Detection
**Symptoms:**
- Cross-region RTT metrics showing timeouts
- Conflicting data between regions
- Health checks failing inconsistently

**Response:**
1. **Immediate**: Continue local operations with conflict markers
2. **Assessment**: `cryptorun replication status --all` to assess scope
3. **Resolution**: Manual conflict resolution after network recovery
4. **Recovery**: Run anti-entropy reconciliation

#### Validation Errors Spike
```bash
# Check validation error breakdown
cryptorun replication status --format json | jq '.consistency_errors'

# Monitor quarantine status
cryptorun replication status --all | grep -i quarantine
```

**Common Validation Issues:**
- **Schema Errors**: Missing required fields, type mismatches
- **Staleness**: Clock drift, ingestion delays
- **Anomalies**: Market volatility, data source issues

**Resolution Steps:**
1. Identify error category (schema/staleness/anomaly)
2. Review data source health and configuration
3. Consider temporary validation threshold adjustments
4. Investigate quarantined data for systematic issues

### Emergency Procedures

#### Regional Outage Response
1. **Assessment** (0-5 minutes):
   ```bash
   cryptorun replication status --region us-east-1 --all
   ```

2. **Failover Decision** (5-15 minutes):
   ```bash
   # Dry run first
   cryptorun replication failover --tier warm --promote us-west-2 --dry-run
   
   # Execute if dry run passes
   cryptorun replication failover --tier warm --promote us-west-2 --validate
   ```

3. **Validation** (15-30 minutes):
   - Verify replication lag drops below SLO
   - Check data consistency metrics
   - Monitor application health endpoints

4. **Communication**:
   - Update incident status
   - Notify downstream systems
   - Document timeline and decisions

#### Data Corruption Response
1. **Immediate Isolation**:
   ```bash
   # Check quarantine status
   cryptorun replication status --all | grep quarantine
   
   # Review corruption scope
   cryptorun replication status --format json | jq '.consistency_errors.corrupt'
   ```

2. **Source Investigation**:
   - Identify affected time windows
   - Check exchange API health
   - Review ingestion pipeline logs

3. **Recovery**:
   ```bash
   # Simulate clean data backfill
   cryptorun replication simulate --from clean-region --to affected-region --window TIME_RANGE
   
   # Execute recovery
   cryptorun replication failover --tier affected --promote clean-region --force
   ```

## Testing & Validation

### Integration Test Suite
Run comprehensive failover testing:
```bash
go test ./tests/integration/multiregion_failover_test.go -v
```

**Test Coverage:**
- Warm tier failover with lag recovery
- Cold tier disaster recovery
- Hot tier active-active scenarios
- Validation layer integration
- Metrics collection accuracy

### Unit Test Coverage
```bash
go test ./tests/unit/validate_*_test.go -v
```

**Validation Components:**
- Schema validation: Field types, patterns, ranges
- Staleness detection: Multi-format timestamps, tier thresholds
- Anomaly detection: MAD scoring, spike detection, corruption handling

### Performance Benchmarks
```bash
# Validation performance
go test ./tests/unit/validate_*_test.go -bench=BenchmarkValidation -benchmem

# Replication simulation
go test ./tests/integration/ -bench=BenchmarkReplication -benchmem
```

**Performance Targets:**
- Schema validation: <100µs per record
- Staleness check: <10µs per record  
- Anomaly detection: <100µs per record
- Replication planning: <1s for 1000 steps

## Monitoring & Alerting

### Key Metrics Dashboard
```prometheus
# Replication lag by tier
cryptorun_replication_lag_seconds{tier="warm", region="us-east-1"} > 90

# Data consistency errors
rate(cryptorun_data_consistency_errors_total[5m]) > 0.1

# Quarantine rate
rate(cryptorun_quarantine_total[5m]) > 0.01

# Regional health
cryptorun_region_health_score < 0.7
```

### Alert Thresholds

| Metric | Warning | Critical | Action |
|--------|---------|----------|--------|
| Replication Lag (Hot) | >1s | >5s | Check WebSocket health |
| Replication Lag (Warm) | >90s | >300s | Consider failover |
| Replication Lag (Cold) | >10m | >30m | Investigate storage |
| Consistency Errors | >5/min | >20/min | Review validation config |
| Quarantine Rate | >1% | >5% | Investigate data sources |
| Region Health | <0.8 | <0.5 | Prepare failover |

### SRE Runbook Integration

This multi-region system integrates with CryptoRun's broader operational framework:

- **Health Checks**: `/health` endpoint includes replication status
- **Metrics Export**: `/metrics` exposes all replication and validation metrics
- **Logging**: Structured logs with correlation IDs for incident tracking
- **Circuit Breakers**: Integration with existing provider health checks

## Related Documentation

- [Data Facade Architecture](DATA_FACADE.md) - Core data layer design
- [CLAUDE.md](../CLAUDE.md) - Development commands and architecture
- [Performance Testing](../tests/load/) - Load testing procedures
- [Circuit Breaker Configuration](../config/circuits.yaml) - Provider reliability

This architecture ensures CryptoRun maintains high availability and data integrity across geographic regions while respecting regulatory requirements and operational constraints.