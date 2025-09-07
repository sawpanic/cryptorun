# CryptoRun Security Policy

## UX MUST — Live Progress & Explainability

Comprehensive security policies and procedures for CryptoRun v3.2.1 covering threat model, access controls, data protection, and incident response.

## Security Philosophy

CryptoRun is designed with security-by-default principles:

- **Zero-Trust Architecture**: Never trust, always verify
- **Defense in Depth**: Multiple security layers at every level
- **Principle of Least Privilege**: Minimal permissions by design  
- **Data Minimization**: Only collect and store what's necessary
- **Audit Everything**: Comprehensive logging and monitoring
- **Fail Securely**: Graceful degradation without data exposure

## Threat Model

### Primary Threats

1. **API Key Exposure**: Accidental commit or logging of exchange credentials
2. **Data Exfiltration**: Unauthorized access to trading strategies or performance data
3. **Service Denial**: Resource exhaustion attacks on scanning infrastructure
4. **Man-in-the-Middle**: Interception of exchange API communications
5. **Privilege Escalation**: Container breakout or unauthorized access elevation
6. **Supply Chain**: Compromised dependencies or build process

### Attack Vectors

- Container runtime vulnerabilities
- Kubernetes RBAC misconfigurations  
- Exposed management endpoints
- Unsecured database connections
- Weak TLS configurations
- Third-party dependency vulnerabilities

## Access Controls

### Authentication & Authorization

```yaml
# Kubernetes RBAC for CryptoRun service account
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: cryptorun-role
rules:
- apiGroups: [""]
  resources: ["configmaps", "secrets"]
  verbs: ["get", "list"]
- apiGroups: [""]
  resources: ["pods"]
  verbs: ["get", "list", "watch"]

# No write permissions to cluster resources
# No access to other namespaces
# No elevated privileges
```

### Network Security

```yaml
# Network policies for pod-to-pod communication
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: cryptorun-netpol
spec:
  podSelector:
    matchLabels:
      app: cryptorun
  policyTypes:
  - Ingress
  - Egress
  ingress:
  - from:
    - podSelector:
        matchLabels:
          app: ingress-nginx
    ports:
    - protocol: TCP
      port: 8080
  egress:
  - to: []  # Allow all egress (required for exchange APIs)
    ports:
    - protocol: TCP
      port: 443  # HTTPS only
    - protocol: TCP  
      port: 5432  # PostgreSQL
    - protocol: TCP
      port: 6379  # Redis
```

### Container Security

```dockerfile
# Multi-stage build with minimal attack surface
FROM golang:1.21-alpine AS builder
RUN adduser -D -g '' appuser
COPY . /src
WORKDIR /src
RUN CGO_ENABLED=0 GOOS=linux go build -o cryptorun ./cmd/cryptorun

FROM scratch
COPY --from=builder /etc/passwd /etc/passwd
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /src/cryptorun /app/cryptorun
USER appuser
EXPOSE 8080 8081
ENTRYPOINT ["/app/cryptorun"]
```

```yaml
# Pod security context
securityContext:
  runAsNonRoot: true
  runAsUser: 65532
  runAsGroup: 65532  
  readOnlyRootFilesystem: true
  allowPrivilegeEscalation: false
  capabilities:
    drop:
    - ALL
  seccompProfile:
    type: RuntimeDefault
```

## Data Protection

### Secrets Management

```yaml
# External secrets operator for secure credential management
apiVersion: external-secrets.io/v1beta1
kind: SecretStore
metadata:
  name: cryptorun-secret-store
spec:
  provider:
    vault:
      server: "https://vault.company.com"
      path: "secret"
      version: "v2"
      auth:
        kubernetes:
          mountPath: "kubernetes"
          role: "cryptorun-role"

---
apiVersion: external-secrets.io/v1beta1
kind: ExternalSecret
metadata:
  name: cryptorun-credentials
spec:
  secretStoreRef:
    name: cryptorun-secret-store
    kind: SecretStore
  target:
    name: cryptorun-secrets
    creationPolicy: Owner
  data:
  - secretKey: pg-dsn
    remoteRef:
      key: cryptorun/database
      property: dsn
  - secretKey: redis-addr  
    remoteRef:
      key: cryptorun/cache
      property: address
```

### Data Encryption

```yaml
# Database encryption at rest (PostgreSQL)
encryption:
  at_rest:
    enabled: true
    algorithm: AES-256-GCM
    key_rotation: 90d
  in_transit:
    tls_version: "1.3"
    certificate_validation: strict
    cipher_suites:
      - "TLS_AES_256_GCM_SHA384"
      - "TLS_CHACHA20_POLY1305_SHA256"

# Redis encryption
redis:
  tls:
    enabled: true
    cert_file: /etc/ssl/certs/redis-client.crt
    key_file: /etc/ssl/private/redis-client.key
    ca_file: /etc/ssl/certs/ca.crt
```

### Data Retention

```yaml
# Automated data retention policies
data_retention:
  scan_results:
    retention_period: 90d
    cleanup_schedule: "0 2 * * *"  # Daily at 2 AM
  performance_metrics:
    retention_period: 1y
    cleanup_schedule: "0 3 1 * *"  # Monthly at 3 AM
  audit_logs:
    retention_period: 2y
    cleanup_schedule: "0 4 1 1 *"  # Yearly at 4 AM
```

## Secure Development

### Code Security

```yaml
# Pre-commit hooks for security scanning
repos:
- repo: https://github.com/Yelp/detect-secrets
  rev: v1.4.0
  hooks:
  - id: detect-secrets
    args: ['--baseline', '.secrets.baseline']

- repo: https://github.com/securecodewarrior/github-action-add-sarif
  rev: v1
  hooks:
  - id: gosec
    args: ['-fmt', 'sarif', '-out', 'gosec.sarif', './...']

- repo: local
  hooks:
  - id: go-mod-verify
    name: verify go modules
    entry: go mod verify
    language: system
    pass_filenames: false
```

### Dependency Management

```yaml
# Dependabot configuration for security updates
version: 2
updates:
  - package-ecosystem: "gomod"
    directory: "/"
    schedule:
      interval: "weekly"
    reviewers:
      - "security-team"
    assignees:
      - "maintainer"
    commit-message:
      prefix: "security"
      include: "scope"

# Vulnerability scanning
security:
  advisories:
    - GHSA-xxxx-xxxx-xxxx  # Known false positive
  ignore_conditions:
    - dependency-name: "example/test-pkg"
      versions: ["< 2.0.0"]
      reason: "Dev dependency only"
```

### Build Security

```yaml
# Signed container images with provenance
name: Secure Build
on:
  push:
    branches: [main]

jobs:
  build:
    runs-on: ubuntu-latest
    permissions:
      contents: read
      id-token: write
      packages: write
    steps:
    - uses: actions/checkout@v4
    
    - name: Install cosign
      uses: sigstore/cosign-installer@v3

    - name: Build and push image
      uses: docker/build-push-action@v5
      with:
        push: true
        tags: ${{ env.REGISTRY }}/cryptorun:${{ github.sha }}
        
    - name: Sign image
      run: |
        cosign sign --yes ${{ env.REGISTRY }}/cryptorun:${{ github.sha }}
        
    - name: Generate provenance
      uses: slsa-framework/slsa-github-generator/.github/workflows/generator_container_slsa3.yml@v1.9.0
      with:
        image: ${{ env.REGISTRY }}/cryptorun
        digest: ${{ steps.build.outputs.digest }}
```

## Runtime Security

### Monitoring & Detection

```yaml
# Falco rules for runtime threat detection
- rule: Unexpected Network Connection from CryptoRun
  desc: Detect unexpected outbound connections
  condition: >
    outbound and container.name contains "cryptorun" and
    not fd.sip in (allowed_exchanges_ips)
  output: >
    Unexpected network connection from CryptoRun
    (container=%container.name dest=%fd.rip:%fd.rport)
  priority: HIGH

- rule: File Access from CryptoRun Container  
  desc: Detect unexpected file access
  condition: >
    open_read and container.name contains "cryptorun" and
    not fd.name in (/app/cryptorun, /etc/ssl/certs, /tmp)
  output: >
    Unexpected file access from CryptoRun
    (container=%container.name file=%fd.name)
  priority: MEDIUM

- rule: Process Execution in CryptoRun
  desc: Detect unexpected process execution
  condition: >
    spawned_process and container.name contains "cryptorun" and
    not proc.name in (cryptorun)
  output: >
    Unexpected process in CryptoRun container
    (container=%container.name process=%proc.name cmdline=%proc.cmdline)
  priority: HIGH
```

### Security Metrics

```prometheus
# Security-related metrics
cryptorun_security_events_total{type="tls_error"} counter
cryptorun_security_events_total{type="auth_failure"} counter
cryptorun_security_events_total{type="rate_limit_exceeded"} counter
cryptorun_security_certificate_expiry_days{cert="api_client"} gauge
cryptorun_security_dependency_vulnerabilities{severity="high"} gauge
```

### Incident Response Automation

```yaml
# Automated incident response
security_automation:
  triggers:
    - metric: cryptorun_security_events_total
      threshold: 10
      window: 5m
      actions:
        - type: alert
          destination: security-team
        - type: rate_limit
          provider: all
          duration: 1h

    - metric: cryptorun_security_certificate_expiry_days  
      threshold: 7
      actions:
        - type: alert
          severity: critical
          destination: ops-team
        - type: certificate_renewal
          auto_approve: false

    - event: pod_security_violation
      actions:
        - type: pod_isolation
          duration: 1h
        - type: forensic_snapshot
          storage: secure-bucket
```

## Compliance & Auditing

### Audit Logging

```go
// Structured audit logging
type AuditEvent struct {
    Timestamp   time.Time `json:"timestamp"`
    EventType   string    `json:"event_type"`
    UserID      string    `json:"user_id,omitempty"`
    Resource    string    `json:"resource"`
    Action      string    `json:"action"`
    Result      string    `json:"result"`
    IPAddress   string    `json:"ip_address,omitempty"`
    UserAgent   string    `json:"user_agent,omitempty"`
    RequestID   string    `json:"request_id"`
    Details     any       `json:"details,omitempty"`
}

// Critical events that must be audited
var AuditableEvents = []string{
    "scan.execute",
    "config.change",
    "credentials.access",
    "database.query",
    "api.rate_limit_exceeded",
    "security.violation",
    "performance.threshold_exceeded",
}
```

### Compliance Requirements

```yaml
# SOC 2 Type II compliance measures
soc2_controls:
  cc6.1:  # Logical access controls
    - kubernetes_rbac: enabled
    - pod_security_standards: restricted
    - network_policies: enforced
    
  cc6.2:  # Transmission and disposal of data
    - encryption_in_transit: tls_1.3
    - encryption_at_rest: aes_256_gcm
    - data_retention_policy: automated
    
  cc6.3:  # Access control management
    - secret_rotation: 90d
    - certificate_rotation: 1y
    - audit_log_retention: 2y

# PCI DSS (if processing payment data)
pci_dss:
  requirement_1:  # Firewall configuration
    - network_segmentation: enforced
    - default_deny: enabled
  requirement_2:  # Secure configurations  
    - container_hardening: enabled
    - minimal_attack_surface: verified
  requirement_3:  # Data protection
    - tokenization: preferred
    - encryption: required
```

## Security Procedures

### Vulnerability Management

```bash
#!/bin/bash
# Automated vulnerability scanning

# Scan container images
trivy image cryptorun:latest --exit-code 1 --severity HIGH,CRITICAL

# Scan dependencies
nancy sleuth --db-url https://ossi.sonatype.org

# Scan infrastructure
kube-score score deploy/k8s/*.yaml
kubesec scan deploy/k8s/deployment.yaml

# Generate security report
cat > security_report.md << EOF
# Security Scan Report - $(date)

## Container Vulnerabilities
$(trivy image cryptorun:latest --format table)

## Dependency Vulnerabilities  
$(nancy sleuth)

## Kubernetes Security Score
$(kube-score score deploy/k8s/*.yaml)
EOF
```

### Incident Response Plan

1. **Detection Phase**
   - Automated alerts via Prometheus/Grafana
   - Falco runtime threat detection
   - Log analysis and correlation

2. **Containment Phase**
   - Automatic pod isolation for security violations
   - Rate limiting for suspicious traffic
   - Circuit breaker activation

3. **Investigation Phase**
   - Forensic container snapshots
   - Audit log analysis
   - Root cause determination

4. **Recovery Phase**
   - Patch deployment via GitOps
   - Certificate rotation if needed
   - Service restoration verification

5. **Post-Incident Phase**
   - Security review and lessons learned
   - Process improvements
   - Documentation updates

### Emergency Procedures

```bash
# Emergency shutdown procedure
kubectl scale deployment cryptorun --replicas=0 -n cryptorun-prod

# Rotate compromised secrets
kubectl delete secret cryptorun-secrets -n cryptorun-prod
kubectl apply -f emergency-secrets.yaml -n cryptorun-prod

# Network isolation
kubectl apply -f - <<EOF
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: cryptorun-emergency-isolation
spec:
  podSelector:
    matchLabels:
      app: cryptorun
  policyTypes:
  - Ingress
  - Egress
  # No ingress/egress rules = deny all traffic
EOF

# Forensic data collection
kubectl exec deployment/cryptorun -- tar czf /tmp/forensics.tar.gz /app/logs /tmp
kubectl cp cryptorun-prod/cryptorun-pod:/tmp/forensics.tar.gz ./incident-$(date +%Y%m%d-%H%M%S).tar.gz
```

## Security Contacts

- **Security Team**: security@yourdomain.com
- **Incident Response**: incident-response@yourdomain.com  
- **Emergency Hotline**: +1-555-SECURITY
- **PGP Key**: [Public key for encrypted communications]

## Reporting Security Issues

1. **Email**: security@yourdomain.com (encrypted with PGP preferred)
2. **Severity**: Classify as Low/Medium/High/Critical
3. **Include**: Detailed description, reproduction steps, impact assessment
4. **Response Time**: 
   - Critical: 2 hours
   - High: 24 hours
   - Medium: 72 hours
   - Low: 1 week

## Security Updates

This policy is reviewed quarterly and updated as needed. Version history:

- v1.0 (2025-09-07): Initial security policy
- Latest updates tracked in git commit history