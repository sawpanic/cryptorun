// CryptoRun Regression Test Suite
// Compares current performance against baseline metrics
// Focus: hit-rate, correlation, latency regression detection

import { check } from 'k6';
import http from 'k6/http';
import { Trend, Rate, Counter } from 'k6/metrics';
import encoding from 'k6/encoding';

// Custom metrics for regression tracking
const hitRateRegression = new Trend('hit_rate_regression');
const latencyRegression = new Trend('latency_regression_ms');  
const correlationRegression = new Trend('correlation_regression');
const baselineComparison = new Rate('baseline_comparison_passed');
const regressionAlerts = new Counter('regression_alerts_generated');

// Configuration
const BASE_URL = __ENV.CRYPTORUN_URL || 'http://localhost:8080';
const BASELINE_PATH = __ENV.BASELINE_PATH || 'artifacts/baselines/baseline.json';

// Test stages for regression testing
export let options = {
  stages: [
    { duration: '1m', target: 5 },   // Light load for regression testing
    { duration: '10m', target: 10 }, // Sustained moderate load
    { duration: '5m', target: 20 },  // Brief spike to test stability
    { duration: '5m', target: 10 },  // Back to moderate
    { duration: '1m', target: 0 },   // Ramp down
  ],
  thresholds: {
    // Regression-specific thresholds (stricter than load test)
    'hit_rate_regression': ['p(50)>-5'], // Hit rate shouldn't drop >5%
    'latency_regression_ms': ['p(95)<50'], // Latency shouldn't increase >50ms
    'correlation_regression': ['p(50)>-0.1'], // Correlation shouldn't increase >0.1
    'baseline_comparison_passed': ['rate>0.8'], // 80% of comparisons should pass
    'http_req_duration': ['p(95)<400'], // Slightly more lenient than load test
    'http_req_failed': ['rate<0.005'], // Stricter error rate for regression
  },
};

// Load baseline data (simulated for now)
const BASELINE_DATA = {
  timestamp: '2025-09-06T12:00:00Z',
  performance: {
    hit_rate: 0.62,           // 62% hit rate
    sharpe_ratio: 1.35,       // Sharpe ratio baseline
    max_drawdown: 0.08,       // 8% max drawdown
    correlation_avg: 0.45,     // Average pairwise correlation
    avg_latency_ms: 125.5,    // Average request latency
  },
  system: {
    cache_hit_rate: 0.87,     // 87% cache hit rate
    error_rate: 0.003,        // 0.3% error rate
    throughput_rps: 85.2,     // Requests per second
    memory_usage_mb: 512.8,   // Memory usage baseline
    cpu_usage_pct: 15.2,      // CPU usage percentage
  },
  data_quality: {
    consensus_score: 0.91,    // Data consensus score
    freshness_avg_sec: 45,    // Average data freshness
    outlier_rate: 0.02,       // 2% outlier detection rate
  },
  version: '3.2.0',
  test_duration_minutes: 30,
  sample_size: 1250,
};

export function setup() {
  console.log('🔍 Starting CryptoRun Regression Test Suite');
  console.log(`Baseline Version: ${BASELINE_DATA.version}`);
  console.log(`Baseline Date: ${BASELINE_DATA.timestamp}`);
  console.log(`Testing against: ${BASE_URL}`);
  
  // Verify target is healthy
  let healthResp = http.get(`${BASE_URL}/health`);
  if (healthResp.status !== 200) {
    throw new Error(`Target unhealthy: ${healthResp.status}`);
  }
  
  // Get current version
  let versionResp = http.get(`${BASE_URL}/version`);
  let currentVersion = 'unknown';
  if (versionResp.status === 200) {
    try {
      currentVersion = JSON.parse(versionResp.body).version;
    } catch (e) {
      console.warn('Could not parse version response');
    }
  }
  
  return {
    baselineData: BASELINE_DATA,
    currentVersion: currentVersion,
    testStartTime: new Date().toISOString(),
  };
}

export default function (data) {
  const testType = Math.random();
  
  if (testType < 0.4) {
    // 40% - Performance regression tests
    testPerformanceRegression(data);
  } else if (testType < 0.7) {
    // 30% - System metrics regression  
    testSystemRegression(data);
  } else if (testType < 0.9) {
    // 20% - Data quality regression
    testDataQualityRegression(data);
  } else {
    // 10% - Comprehensive scan comparison
    testScanRegression(data);
  }
}

function testPerformanceRegression(data) {
  const perfStart = Date.now();
  
  // Get current performance metrics
  const perfResp = http.get(`${BASE_URL}/api/performance/current`, {
    timeout: '15s',
    tags: { test_type: 'performance_regression' },
  });
  
  const latencyMs = Date.now() - perfStart;
  latencyRegression.add(latencyMs);
  
  if (check(perfResp, { 'performance response valid': (r) => r.status === 200 })) {
    try {
      const currentPerf = JSON.parse(perfResp.body);
      const baseline = data.baselineData.performance;
      
      // Compare hit rate
      const hitRateDiff = ((currentPerf.hit_rate - baseline.hit_rate) / baseline.hit_rate) * 100;
      hitRateRegression.add(hitRateDiff);
      
      // Compare correlation
      const corrDiff = currentPerf.correlation_avg - baseline.correlation_avg;
      correlationRegression.add(corrDiff);
      
      // Latency comparison
      const latencyDiff = latencyMs - baseline.avg_latency_ms;
      latencyRegression.add(latencyDiff);
      
      // Overall comparison
      const regressionDetected = (
        hitRateDiff < -10 || // Hit rate dropped >10%
        corrDiff > 0.15 ||   // Correlation increased >0.15
        latencyDiff > 100    // Latency increased >100ms
      );
      
      baselineComparison.add(regressionDetected ? 0 : 1);
      
      if (regressionDetected) {
        regressionAlerts.add(1);
        console.warn(`⚠️ Performance regression detected:`);
        console.warn(`  Hit rate: ${hitRateDiff.toFixed(1)}% change`);
        console.warn(`  Correlation: ${corrDiff.toFixed(3)} increase`);
        console.warn(`  Latency: +${latencyDiff.toFixed(1)}ms`);
      }
      
    } catch (e) {
      console.error(`Performance regression test failed: ${e.message}`);
      baselineComparison.add(0);
    }
  }
}

function testSystemRegression(data) {
  const sysStart = Date.now();
  
  // Get system metrics
  const metricsResp = http.get(`${BASE_URL}/metrics`, {
    timeout: '10s',
    tags: { test_type: 'system_regression' },
  });
  
  if (check(metricsResp, { 'metrics response valid': (r) => r.status === 200 })) {
    // Parse Prometheus metrics (simplified)
    const metricsText = metricsResp.body;
    const baseline = data.baselineData.system;
    
    // Extract current metrics (simplified parsing)
    const currentMetrics = parsePrometheusMetrics(metricsText);
    
    // Cache hit rate comparison
    const cacheHitRate = currentMetrics.cache_hit_ratio || 0.85;
    const cacheRateDiff = ((cacheHitRate - baseline.cache_hit_rate) / baseline.cache_hit_rate) * 100;
    
    // Error rate comparison  
    const errorRate = currentMetrics.error_rate || 0.005;
    const errorRateDiff = ((errorRate - baseline.error_rate) / baseline.error_rate) * 100;
    
    // System regression detection
    const systemRegression = (
      cacheRateDiff < -10 ||  // Cache hit rate dropped >10%
      errorRateDiff > 50      // Error rate increased >50%
    );
    
    baselineComparison.add(systemRegression ? 0 : 1);
    
    if (systemRegression) {
      regressionAlerts.add(1);
      console.warn(`⚠️ System regression detected:`);
      console.warn(`  Cache hit rate: ${cacheRateDiff.toFixed(1)}% change`);  
      console.warn(`  Error rate: ${errorRateDiff.toFixed(1)}% change`);
    }
  }
}

function testDataQualityRegression(data) {
  const qualityStart = Date.now();
  
  // Test data quality endpoints
  const qualityResp = http.get(`${BASE_URL}/api/data/quality`, {
    timeout: '10s',
    tags: { test_type: 'quality_regression' },
  });
  
  if (check(qualityResp, { 'quality response valid': (r) => r.status === 200 })) {
    try {
      const currentQuality = JSON.parse(qualityResp.body);
      const baseline = data.baselineData.data_quality;
      
      // Consensus score comparison
      const consensusDiff = ((currentQuality.consensus_score - baseline.consensus_score) / baseline.consensus_score) * 100;
      
      // Freshness comparison
      const freshnessDiff = currentQuality.freshness_avg_sec - baseline.freshness_avg_sec;
      
      // Outlier rate comparison
      const outlierDiff = currentQuality.outlier_rate - baseline.outlier_rate;
      
      // Data quality regression detection
      const qualityRegression = (
        consensusDiff < -5 ||      // Consensus dropped >5%
        freshnessDiff > 30 ||      // Freshness increased >30s
        outlierDiff > 0.01         // Outlier rate increased >1%
      );
      
      baselineComparison.add(qualityRegression ? 0 : 1);
      
      if (qualityRegression) {
        regressionAlerts.add(1);
        console.warn(`⚠️ Data quality regression detected:`);
        console.warn(`  Consensus: ${consensusDiff.toFixed(1)}% change`);
        console.warn(`  Freshness: +${freshnessDiff.toFixed(1)}s`);
        console.warn(`  Outliers: +${(outlierDiff * 100).toFixed(2)}%`);
      }
      
    } catch (e) {
      console.error(`Data quality regression test failed: ${e.message}`);
      baselineComparison.add(0);
    }
  }
}

function testScanRegression(data) {
  const scanStart = Date.now();
  
  // Perform comprehensive scan and compare results
  const scanPayload = {
    exchange: 'kraken',
    pairs: 'USD-only',
    limit: 10,
    dry_run: true,
    include_regime: true,
    include_factors: true,
  };
  
  const scanResp = http.post(`${BASE_URL}/api/scan/comprehensive`, JSON.stringify(scanPayload), {
    headers: { 'Content-Type': 'application/json' },
    timeout: '30s',
    tags: { test_type: 'scan_regression' },
  });
  
  const scanLatency = Date.now() - scanStart;
  latencyRegression.add(scanLatency);
  
  if (check(scanResp, { 'scan response valid': (r) => r.status === 200 || r.status === 202 })) {
    try {
      const scanResults = JSON.parse(scanResp.body);
      
      // Compare scan characteristics against baseline
      const resultCount = scanResults.results ? scanResults.results.length : 0;
      const avgScore = scanResults.avg_score || 0;
      const regimeStability = scanResults.regime_confidence || 0;
      
      // Scan regression detection
      const scanRegression = (
        resultCount < 5 ||        // Too few results
        avgScore < 0.3 ||         // Poor average scores  
        regimeStability < 0.7 ||  // Unstable regime detection
        scanLatency > 5000        // Excessive scan time
      );
      
      baselineComparison.add(scanRegression ? 0 : 1);
      
      if (scanRegression) {
        regressionAlerts.add(1);
        console.warn(`⚠️ Scan regression detected:`);
        console.warn(`  Result count: ${resultCount}`);
        console.warn(`  Average score: ${avgScore.toFixed(3)}`);
        console.warn(`  Regime confidence: ${regimeStability.toFixed(3)}`);
        console.warn(`  Scan latency: ${scanLatency}ms`);
      }
      
    } catch (e) {
      console.error(`Scan regression test failed: ${e.message}`);
      baselineComparison.add(0);
    }
  }
}

// Helper function to parse Prometheus metrics (simplified)
function parsePrometheusMetrics(metricsText) {
  const metrics = {};
  
  // Extract cache hit ratio
  const cacheHitMatch = metricsText.match(/cryptorun_cache_hit_ratio\s+([\d.]+)/);
  if (cacheHitMatch) {
    metrics.cache_hit_ratio = parseFloat(cacheHitMatch[1]);
  }
  
  // Extract error rate (simplified)
  const errorRateMatch = metricsText.match(/cryptorun_pipeline_errors_total\s+([\d.]+)/);
  if (errorRateMatch) {
    metrics.error_rate = parseFloat(errorRateMatch[1]) / 1000; // Normalize
  }
  
  return metrics;
}

export function handleSummary(data) {
  const regressionSummary = {
    test_info: {
      start_time: data.setup_data?.testStartTime,
      end_time: new Date().toISOString(),
      baseline_version: data.setup_data?.baselineData?.version,
      current_version: data.setup_data?.currentVersion,
      duration_minutes: Math.round((Date.now() - new Date(data.setup_data?.testStartTime)) / 60000),
    },
    regression_metrics: {
      hit_rate_change_pct: data.metrics.hit_rate_regression?.avg || 0,
      latency_change_ms: data.metrics.latency_regression_ms?.avg || 0,
      correlation_change: data.metrics.correlation_regression?.avg || 0,
      baseline_comparisons_passed: (data.metrics.baseline_comparison_passed?.rate || 0) * 100,
      total_regression_alerts: data.metrics.regression_alerts_generated?.count || 0,
    },
    performance_summary: {
      total_requests: data.metrics.http_reqs?.count || 0,
      p95_latency: data.metrics.http_req_duration?.['p(95)'] || 0,
      error_rate_pct: (data.metrics.http_req_failed?.rate || 0) * 100,
      passed_thresholds: Object.keys(data.thresholds || {}).filter(key => 
        data.thresholds[key]?.ok === true
      ).length,
      total_thresholds: Object.keys(data.thresholds || {}).length,
    },
    regression_status: {
      overall_passed: (data.metrics.baseline_comparison_passed?.rate || 0) >= 0.8,
      critical_regressions: (data.metrics.regression_alerts_generated?.count || 0) > 5,
      performance_stable: (data.metrics.http_req_duration?.['p(95)'] || 0) < 400,
    }
  };
  
  // Determine overall regression status
  const regressionDetected = !regressionSummary.regression_status.overall_passed ||
                            regressionSummary.regression_status.critical_regressions;
  
  console.log('\n📊 Regression Test Summary:');
  console.log('════════════════════════════');
  console.log(`Baseline Version: ${regressionSummary.test_info.baseline_version}`);
  console.log(`Current Version: ${regressionSummary.test_info.current_version}`);
  console.log(`Test Duration: ${regressionSummary.test_info.duration_minutes} minutes`);
  console.log(`Total Requests: ${regressionSummary.performance_summary.total_requests}`);
  console.log(`Baseline Comparisons Passed: ${regressionSummary.regression_metrics.baseline_comparisons_passed.toFixed(1)}%`);
  console.log(`Regression Alerts Generated: ${regressionSummary.regression_metrics.total_regression_alerts}`);
  
  console.log('\n📈 Performance Changes:');
  console.log(`Hit Rate Change: ${regressionSummary.regression_metrics.hit_rate_change_pct.toFixed(1)}%`);
  console.log(`Latency Change: ${regressionSummary.regression_metrics.latency_change_ms.toFixed(1)}ms`);
  console.log(`Correlation Change: ${regressionSummary.regression_metrics.correlation_change.toFixed(3)}`);
  
  console.log(`\n🎯 Regression Status: ${regressionDetected ? '❌ REGRESSION DETECTED' : '✅ NO REGRESSION'}`);
  
  if (regressionDetected) {
    console.log('\n⚠️ Regression Details:');
    if (!regressionSummary.regression_status.overall_passed) {
      console.log(`- Only ${regressionSummary.regression_metrics.baseline_comparisons_passed.toFixed(1)}% of baseline comparisons passed (need ≥80%)`);
    }
    if (regressionSummary.regression_status.critical_regressions) {
      console.log(`- ${regressionSummary.regression_metrics.total_regression_alerts} regression alerts generated (threshold: 5)`);
    }
  }
  
  // Update baseline if current performance is better
  const shouldUpdateBaseline = (
    !regressionDetected &&
    regressionSummary.regression_metrics.hit_rate_change_pct > 5 && // Hit rate improved >5%
    regressionSummary.regression_metrics.latency_change_ms < -20    // Latency improved >20ms
  );
  
  if (shouldUpdateBaseline) {
    console.log('\n✨ Performance improved! Consider updating baseline.');
  }
  
  return {
    'stdout': `\nRegression test completed. ${regressionDetected ? 'REGRESSIONS DETECTED' : 'No regressions found'}.\n`,
    'artifacts/baselines/regression-results.json': JSON.stringify(regressionSummary, null, 2),
    'artifacts/baselines/regression-full.json': JSON.stringify(data, null, 2),
  };
}

export function teardown(data) {
  console.log('\n🏁 Regression test suite completed');
  const duration = Math.round((Date.now() - new Date(data.testStartTime)) / 60000);
  console.log(`Total duration: ${duration} minutes`);
}