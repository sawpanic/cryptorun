# CryptoRun Deployment Guide

## UX MUST — Live Progress & Explainability

Complete deployment guide for CryptoRun v3.2.1 covering containerization, Kubernetes, database setup, and production configuration.

## Environment Requirements

### Minimum System Requirements

- **CPU**: 2 cores (4 recommended)
- **Memory**: 4GB RAM (8GB recommended)
- **Storage**: 50GB available (SSD preferred)
- **Network**: Stable internet connection for exchange APIs
- **OS**: Linux (Ubuntu 20.04+), macOS, or Windows with WSL2

### Required Dependencies

- **Go**: 1.21+ for building from source
- **Docker**: 20.10+ for containerization
- **Kubernetes**: 1.25+ for orchestration (optional)
- **PostgreSQL**: 15+ with TimescaleDB extension (recommended)
- **Redis**: 6+ for caching

## Quick Start - Docker Compose

### 1. Development Environment Setup

```bash
# Clone repository
git clone https://github.com/sawpanic/cryptorun.git
cd cryptorun

# Start all services (PostgreSQL, Redis, CryptoRun)
docker-compose up -d

# Check service health
docker-compose ps
docker-compose logs cryptorun

# Access endpoints
curl http://localhost:8080/health
curl http://localhost:8081/metrics
```

### 2. Development with Database

```bash
# Start only database services
docker-compose up -d postgres redis

# Run CryptoRun locally with database connection
export PG_DSN="postgres://cryptorun:cryptorun@localhost:5432/cryptorun?sslmode=disable"
export REDIS_ADDR="localhost:6379"

# Build and run
make build
./cryptorun monitor --exchange kraken --pairs USD-only
```

## Production Deployment - Kubernetes

### Prerequisites

1. **Kubernetes Cluster**: EKS, GKE, AKS, or self-managed
2. **Ingress Controller**: NGINX Ingress Controller
3. **Cert Manager**: For TLS certificate management
4. **Monitoring**: Prometheus + Grafana (optional but recommended)

### Application Deployment

```bash
# Apply all Kubernetes manifests
kubectl apply -f deploy/k8s/

# Check deployment status
kubectl get pods -l app=cryptorun
kubectl describe deployment cryptorun

# Check logs
kubectl logs -f deployment/cryptorun
```

## Database Setup

### PostgreSQL with TimescaleDB

```bash
# Using Docker for development
docker run -d \
  --name cryptorun-postgres \
  -p 5432:5432 \
  -e POSTGRES_DB=cryptorun \
  -e POSTGRES_USER=cryptorun \
  -e POSTGRES_PASSWORD=cryptorun \
  timescale/timescaledb:latest-pg15

# Apply migrations
make dbmigrate
```

### Environment Variables

| Variable | Description | Example |
|----------|-------------|---------|
| `PG_DSN` | PostgreSQL connection string | `postgres://user:pass@host:5432/db?sslmode=require` |
| `REDIS_ADDR` | Redis server address | `localhost:6379` |
| `METRICS_ADDR` | Metrics server bind address | `:8081` |
| `LOG_LEVEL` | Logging level | `info` |

## Health Checks and Monitoring

### Health Endpoints

- **`/health`**: General health check (database, cache, exchanges)
- **`/metrics`**: Prometheus metrics
- **`/ready`**: Kubernetes readiness check

### Key Metrics

```prometheus
# Request rates and latencies
cryptorun_http_requests_total
cryptorun_http_request_duration_seconds

# Database performance
cryptorun_db_connections_active
cryptorun_db_query_duration_seconds

# Exchange connectivity
cryptorun_exchange_requests_total
cryptorun_exchange_websocket_connections
```

## Security Configuration

- Multi-stage Docker build with distroless base image
- Non-root user execution (uid: 65532)
- Read-only root filesystem
- Resource limits and security contexts
- Network policies for pod-to-pod communication
- TLS termination at ingress level

## Build Commands

```bash
# Development build
make build

# Docker build
make docker

# Database setup and testing
make dbtest

# Full quality checks
make check
```

## End-to-End Production Deployment

### 1. Infrastructure Preparation

```bash
# Create namespace
kubectl create namespace cryptorun-prod

# Create secrets
kubectl create secret generic cryptorun-secrets \
  --from-literal=pg-dsn="postgres://cryptorun:YOUR_PASSWORD@postgres:5432/cryptorun?sslmode=require" \
  --from-literal=redis-addr="redis:6379" \
  -n cryptorun-prod

# Apply ConfigMaps
kubectl apply -f deploy/k8s/configmap.yaml -n cryptorun-prod

# Deploy PostgreSQL with TimescaleDB
helm install postgres bitnami/postgresql \
  --set auth.postgresPassword=YOUR_PASSWORD \
  --set auth.database=cryptorun \
  --set auth.username=cryptorun \
  --set auth.password=YOUR_PASSWORD \
  --set image.tag=15.4.0-debian-11-r45 \
  --set persistence.size=100Gi \
  -n cryptorun-prod

# Deploy Redis
helm install redis bitnami/redis \
  --set auth.enabled=false \
  --set replica.replicaCount=1 \
  --set persistence.size=20Gi \
  -n cryptorun-prod
```

### 2. CryptoRun Application Deployment

```bash
# Build and push Docker image
docker build -t cryptorun:v3.2.1 .
docker tag cryptorun:v3.2.1 your-registry/cryptorun:v3.2.1
docker push your-registry/cryptorun:v3.2.1

# Deploy CryptoRun application
envsubst < deploy/k8s/deployment.yaml | kubectl apply -n cryptorun-prod -f -

# Expose service
kubectl apply -f deploy/k8s/service.yaml -n cryptorun-prod

# Configure ingress
kubectl apply -f deploy/k8s/ingress.yaml -n cryptorun-prod
```

### 3. Database Migration and Setup

```bash
# Wait for PostgreSQL to be ready
kubectl wait --for=condition=ready pod -l app.kubernetes.io/name=postgresql -n cryptorun-prod --timeout=300s

# Run database migrations
kubectl exec -it deployment/cryptorun -n cryptorun-prod -- /app/cryptorun migrate --up

# Verify TimescaleDB extension
kubectl exec -it deployment/cryptorun -n cryptorun-prod -- /app/cryptorun dbcheck
```

### 4. Monitoring Setup (Optional)

```bash
# Install Prometheus + Grafana
helm repo add prometheus-community https://prometheus-community.github.io/helm-charts
helm install prometheus prometheus-community/kube-prometheus-stack \
  --set grafana.adminPassword=admin123 \
  --set prometheus.prometheusSpec.retention=30d \
  -n cryptorun-prod

# Import Grafana dashboard
kubectl create configmap grafana-dashboard-cryptorun \
  --from-file=deploy/grafana/cryptorun-overview-dashboard.json \
  -n cryptorun-prod

# Apply ServiceMonitor for metrics collection
kubectl apply -f deploy/k8s/servicemonitor.yaml -n cryptorun-prod
```

### 5. Validation and Testing

```bash
# Check deployment status
kubectl get all -n cryptorun-prod

# Test health endpoints
kubectl port-forward svc/cryptorun 8080:8080 -n cryptorun-prod &
curl http://localhost:8080/health
curl http://localhost:8080/metrics

# Run load test against production
cd tests/load
k6 run scan_load_test.js --env CRYPTORUN_URL=https://your-domain.com

# Run regression tests
k6 run regression_suite.js --env CRYPTORUN_URL=https://your-domain.com
```

## Scaling Configuration

### Horizontal Pod Autoscaler

```yaml
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: cryptorun-hpa
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: cryptorun
  minReplicas: 2
  maxReplicas: 10
  metrics:
  - type: Resource
    resource:
      name: cpu
      target:
        type: Utilization
        averageUtilization: 70
  - type: Resource
    resource:
      name: memory
      target:
        type: Utilization
        averageUtilization: 80
```

### Database Scaling

```bash
# Scale PostgreSQL for high availability
helm upgrade postgres bitnami/postgresql \
  --set replication.enabled=true \
  --set replication.numSynchronousReplicas=1 \
  --set replication.synchronousCommit=on \
  --set pgpool.enabled=true \
  -n cryptorun-prod

# Scale Redis for clustering
helm upgrade redis bitnami/redis \
  --set cluster.enabled=true \
  --set cluster.nodes=6 \
  --set cluster.replicas=1 \
  -n cryptorun-prod
```

## Troubleshooting

### Common Issues

1. **Database Connection Errors**
   ```bash
   # Check PostgreSQL pod status
   kubectl logs -l app.kubernetes.io/name=postgresql -n cryptorun-prod
   
   # Test connection from CryptoRun pod
   kubectl exec -it deployment/cryptorun -n cryptorun-prod -- psql -h postgres -U cryptorun -d cryptorun -c "SELECT NOW();"
   ```

2. **High Memory Usage**
   ```bash
   # Check memory usage
   kubectl top pods -n cryptorun-prod
   
   # Adjust memory limits in deployment
   kubectl patch deployment cryptorun -n cryptorun-prod -p '{"spec":{"template":{"spec":{"containers":[{"name":"cryptorun","resources":{"limits":{"memory":"2Gi"}}}]}}}}'
   ```

3. **Exchange API Rate Limits**
   ```bash
   # Check rate limit metrics
   kubectl exec -it deployment/cryptorun -n cryptorun-prod -- curl -s localhost:8081/metrics | grep cryptorun_provider_rate_limit
   
   # Adjust rate limit configuration
   kubectl edit configmap cryptorun-config -n cryptorun-prod
   ```

### Performance Optimization

```bash
# Enable connection pooling for PostgreSQL
kubectl patch configmap cryptorun-config -n cryptorun-prod --patch='
data:
  database.yaml: |
    max_connections: 100
    max_idle_connections: 10
    max_lifetime: 1h
    connection_timeout: 30s
'

# Optimize Redis for caching workload
kubectl patch configmap redis-configuration -n cryptorun-prod --patch='
data:
  redis.conf: |
    maxmemory-policy allkeys-lru
    maxmemory 1gb
    save ""
'
```
