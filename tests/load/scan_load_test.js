// K6 Load Test for CryptoRun Scan Operations
// Tests high-throughput scanning via CLI and HTTP endpoints
// Target: P99 latency < 300ms under load

import http from 'k6/http';
import { check, sleep } from 'k6';
import { Rate, Trend } from 'k6/metrics';
import exec from 'k6/execution';

// Custom metrics
const scanLatency = new Trend('scan_latency_ms');
const scanErrorRate = new Rate('scan_error_rate');
const healthCheckLatency = new Trend('health_check_latency_ms');

// Configuration
const BASE_URL = __ENV.CRYPTORUN_URL || 'http://localhost:8080';
const CLI_COMMAND = __ENV.CLI_PATH || './cryptorun';

// Test configuration options
export let options = {
  stages: [
    // Ramp up
    { duration: '2m', target: 10 },   // Ramp to 10 users over 2 minutes
    { duration: '5m', target: 50 },   // Ramp to 50 users over 5 minutes
    { duration: '10m', target: 100 }, // Ramp to 100 users over 10 minutes
    
    // Sustained load
    { duration: '15m', target: 100 }, // Stay at 100 users for 15 minutes
    { duration: '5m', target: 200 },  // Spike to 200 users for 5 minutes
    { duration: '10m', target: 100 }, // Back to 100 users for 10 minutes
    
    // Ramp down
    { duration: '5m', target: 0 },    // Ramp down to 0 users
  ],
  thresholds: {
    // P99 latency target: < 300ms
    'scan_latency_ms': ['p(99)<300'],
    
    // Error rate target: < 1%
    'scan_error_rate': ['rate<0.01'],
    
    // Health check latency: < 100ms
    'health_check_latency_ms': ['p(95)<100'],
    
    // HTTP duration thresholds
    'http_req_duration': [
      'p(95)<500',   // 95% of requests under 500ms
      'p(99)<1000',  // 99% of requests under 1s
    ],
    
    // HTTP failure rate: < 1%
    'http_req_failed': ['rate<0.01'],
  },
  ext: {
    loadimpact: {
      distribution: {
        'amazon:us:ashburn': { loadZone: 'amazon:us:ashburn', percent: 50 },
        'amazon:ie:dublin': { loadZone: 'amazon:ie:dublin', percent: 25 },
        'amazon:sg:singapore': { loadZone: 'amazon:sg:singapore', percent: 25 },
      }
    }
  }
};

// Test data
const TEST_SYMBOLS = ['BTC-USD', 'ETH-USD', 'USDT', 'USDC', 'ADA-USD', 'DOT-USD'];
const EXCHANGES = ['kraken', 'binance', 'coinbase'];
const SCAN_TYPES = ['momentum', 'regime', 'portfolio'];

export function setup() {
  console.log('🚀 Starting CryptoRun Load Test');
  console.log(`Target: ${BASE_URL}`);
  console.log(`Test duration: ~52 minutes`);
  console.log(`Peak load: 200 concurrent users`);
  
  // Verify target is healthy before starting
  let healthResp = http.get(`${BASE_URL}/health`);
  if (healthResp.status !== 200) {
    throw new Error(`Target unhealthy: ${healthResp.status}`);
  }
  
  return {
    startTime: new Date().toISOString(),
    baseUrl: BASE_URL,
  };
}

export default function (data) {
  const testType = Math.random();
  
  if (testType < 0.6) {
    // 60% - HTTP API scan requests
    testHTTPScan(data);
  } else if (testType < 0.9) {
    // 30% - Health checks and metrics
    testHealthAndMetrics(data);
  } else {
    // 10% - CLI scan simulation (via HTTP proxy if available)
    testCLIScanSimulation(data);
  }
  
  // Random sleep between 1-5 seconds to simulate realistic user behavior
  sleep(Math.random() * 4 + 1);
}

function testHTTPScan(data) {
  const scanStart = Date.now();
  
  // Select random test parameters
  const symbol = TEST_SYMBOLS[Math.floor(Math.random() * TEST_SYMBOLS.length)];
  const exchange = EXCHANGES[Math.floor(Math.random() * EXCHANGES.length)];
  const scanType = SCAN_TYPES[Math.floor(Math.random() * SCAN_TYPES.length)];
  
  // Build scan request URL
  const scanUrl = `${data.baseUrl}/api/scan`;
  
  const payload = {
    symbol: symbol,
    exchange: exchange,
    scan_type: scanType,
    dry_run: true,
    limit: 10,
  };
  
  const params = {
    headers: {
      'Content-Type': 'application/json',
      'User-Agent': `K6-LoadTest/${exec.vu.idInTest}`,
    },
    timeout: '30s',
  };
  
  // Execute scan request
  const response = http.post(scanUrl, JSON.stringify(payload), params);
  
  const scanLatencyMs = Date.now() - scanStart;
  scanLatency.add(scanLatencyMs);
  
  // Validate response
  const success = check(response, {
    'scan status is 200 or 202': (r) => r.status === 200 || r.status === 202,
    'scan response has data': (r) => {
      if (r.body) {
        try {
          const body = JSON.parse(r.body);
          return body.results || body.status;
        } catch {
          return false;
        }
      }
      return false;
    },
    'scan latency under 1s': () => scanLatencyMs < 1000,
  });
  
  if (!success) {
    scanErrorRate.add(1);
    console.warn(`Scan failed: ${response.status} - ${symbol}@${exchange}`);
  } else {
    scanErrorRate.add(0);
  }
}

function testHealthAndMetrics(data) {
  const healthStart = Date.now();
  
  // Health check
  const healthResp = http.get(`${data.baseUrl}/health`, {
    timeout: '10s',
    tags: { endpoint: 'health' },
  });
  
  const healthLatencyMs = Date.now() - healthStart;
  healthCheckLatency.add(healthLatencyMs);
  
  check(healthResp, {
    'health check status 200': (r) => r.status === 200,
    'health check latency < 100ms': () => healthLatencyMs < 100,
    'health response valid': (r) => {
      try {
        const body = JSON.parse(r.body);
        return body.overall && body.timestamp;
      } catch {
        return false;
      }
    },
  });
  
  // Metrics endpoint check (20% of the time)
  if (Math.random() < 0.2) {
    const metricsResp = http.get(`${data.baseUrl}/metrics`, {
      timeout: '10s',
      tags: { endpoint: 'metrics' },
    });
    
    check(metricsResp, {
      'metrics status 200': (r) => r.status === 200,
      'metrics content valid': (r) => r.body && r.body.includes('cryptorun_'),
    });
  }
}

function testCLIScanSimulation(data) {
  // Simulate CLI scan via HTTP API (since direct CLI execution in K6 is limited)
  const cliStart = Date.now();
  
  const symbol = TEST_SYMBOLS[Math.floor(Math.random() * TEST_SYMBOLS.length)];
  const exchange = EXCHANGES[Math.floor(Math.random() * EXCHANGES.length)];
  
  // Simulate CLI scan with API equivalent
  const cliUrl = `${data.baseUrl}/api/cli/scan`;
  
  const payload = {
    args: [
      'scan',
      '--exchange', exchange,
      '--symbol', symbol,
      '--dry-run',
      '--format', 'json',
      '--limit', '5'
    ],
  };
  
  const response = http.post(cliUrl, JSON.stringify(payload), {
    headers: {
      'Content-Type': 'application/json',
      'X-CLI-Simulation': 'true',
    },
    timeout: '45s', // CLI operations might take longer
  });
  
  const cliLatencyMs = Date.now() - cliStart;
  scanLatency.add(cliLatencyMs); // Use same metric as HTTP scans
  
  check(response, {
    'CLI scan status success': (r) => r.status === 200 || r.status === 202,
    'CLI scan returns results': (r) => {
      try {
        const body = JSON.parse(r.body);
        return body.results || body.output;
      } catch {
        return false;
      }
    },
    'CLI scan latency reasonable': () => cliLatencyMs < 5000,
  });
}

export function handleSummary(data) {
  const summary = {
    test_start: data.setup_data?.startTime,
    test_end: new Date().toISOString(),
    total_requests: data.metrics.http_reqs?.count || 0,
    avg_response_time: data.metrics.http_req_duration?.avg || 0,
    p95_response_time: data.metrics.http_req_duration?.['p(95)'] || 0,
    p99_response_time: data.metrics.http_req_duration?.['p(99)'] || 0,
    error_rate: (data.metrics.http_req_failed?.rate || 0) * 100,
    scan_p99_latency: data.metrics.scan_latency_ms?.['p(99)'] || 0,
    scan_error_rate: (data.metrics.scan_error_rate?.rate || 0) * 100,
    health_check_p95: data.metrics.health_check_latency_ms?.['p(95)'] || 0,
    thresholds_passed: data.thresholds || {},
  };
  
  console.log('\n📊 Load Test Summary:');
  console.log('═══════════════════════');
  console.log(`Total Requests: ${summary.total_requests}`);
  console.log(`Average Response Time: ${summary.avg_response_time.toFixed(2)}ms`);
  console.log(`P95 Response Time: ${summary.p95_response_time.toFixed(2)}ms`);
  console.log(`P99 Response Time: ${summary.p99_response_time.toFixed(2)}ms`);
  console.log(`Error Rate: ${summary.error_rate.toFixed(2)}%`);
  console.log(`Scan P99 Latency: ${summary.scan_p99_latency.toFixed(2)}ms`);
  console.log(`Scan Error Rate: ${summary.scan_error_rate.toFixed(2)}%`);
  console.log(`Health Check P95: ${summary.health_check_p95.toFixed(2)}ms`);
  
  // Check if P99 target was met
  const p99Target = 300; // 300ms target
  const p99Met = summary.scan_p99_latency <= p99Target;
  console.log(`\n🎯 P99 Target (${p99Target}ms): ${p99Met ? '✅ MET' : '❌ MISSED'}`);
  
  if (!p99Met) {
    console.log(`   Actual P99: ${summary.scan_p99_latency.toFixed(2)}ms`);
    console.log(`   Over target by: ${(summary.scan_p99_latency - p99Target).toFixed(2)}ms`);
  }
  
  // Return both text and JSON summary
  return {
    'stdout': '\nLoad test completed. See artifacts/load-test-results.json for detailed results.\n',
    'artifacts/load-test-results.json': JSON.stringify(summary, null, 2),
    'artifacts/load-test-full.json': JSON.stringify(data, null, 2),
  };
}

export function teardown(data) {
  console.log('\n🏁 Load test teardown completed');
  console.log(`Test started: ${data.startTime}`);
  console.log(`Test ended: ${new Date().toISOString()}`);
}