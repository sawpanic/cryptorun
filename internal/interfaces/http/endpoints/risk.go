package endpoints

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/sawpanic/cryptorun/internal/application"
)

// RiskEnvelopeHandler returns the risk envelope monitoring endpoint
func RiskEnvelopeHandler(monitor *application.RiskMonitor) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", "GET")
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Check if requesting Prometheus format
		if r.URL.Query().Get("format") == "prometheus" || r.Header.Get("Accept") == "text/plain" {
			w.Header().Set("Content-Type", "text/plain")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(monitor.GetPrometheusMetrics()))
			return
		}

		// Default to JSON format with optional HTML view
		if strings.Contains(r.Header.Get("Accept"), "text/html") {
			renderRiskHTML(w, monitor)
			return
		}

		// JSON response
		jsonData, err := monitor.GetRiskEnvelopeJSON()
		if err != nil {
			http.Error(w, "Failed to generate risk metrics", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(jsonData)
	}
}

// MetricsHandlerWithRisk extends the metrics handler to include risk envelope data
func MetricsHandlerWithRisk(collector interface{}, monitor *application.RiskMonitor) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", "GET")
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Check if requesting Prometheus format
		if r.URL.Query().Get("format") == "prometheus" || r.Header.Get("Accept") == "text/plain" {
			w.Header().Set("Content-Type", "text/plain")
			w.WriteHeader(http.StatusOK)

			// Combine standard metrics with risk envelope metrics
			output := "# CryptoRun Metrics\n\n"
			output += monitor.GetPrometheusMetrics()

			w.Write([]byte(output))
			return
		}

		// Default to HTML view with risk envelope summary
		if strings.Contains(r.Header.Get("Accept"), "text/html") || r.URL.Query().Get("format") == "" {
			renderMetricsWithRiskHTML(w, monitor)
			return
		}

		// JSON response with risk envelope included
		riskData, err := monitor.GetRiskEnvelopeJSON()
		if err != nil {
			http.Error(w, "Failed to generate metrics", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(riskData)
	}
}

// renderRiskHTML renders a human-readable HTML view of risk envelope status
func renderRiskHTML(w http.ResponseWriter, monitor *application.RiskMonitor) {
	summary := monitor.GetRiskEnvelopeSummary()

	html := `<!DOCTYPE html>
<html>
<head>
    <title>CryptoRun - Risk Envelope</title>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <style>
        body { font-family: 'Courier New', monospace; margin: 40px; background-color: #f5f5f5; }
        .container { background: white; padding: 30px; border-radius: 8px; box-shadow: 0 2px 10px rgba(0,0,0,0.1); }
        h1 { color: #333; border-bottom: 2px solid #007acc; padding-bottom: 10px; }
        h2 { color: #555; margin-top: 30px; }
        .status { font-weight: bold; padding: 5px 10px; border-radius: 4px; }
        .healthy { background-color: #d4edda; color: #155724; }
        .degraded { background-color: #fff3cd; color: #856404; }
        .paused { background-color: #f8d7da; color: #721c24; }
        .metric { margin: 10px 0; }
        .metric-name { display: inline-block; width: 200px; font-weight: bold; }
        .metric-value { color: #007acc; }
        .section { margin: 20px 0; padding: 15px; border-left: 4px solid #007acc; background: #f8f9fa; }
        .violation { color: #dc3545; font-weight: bold; }
        .warning { color: #ffc107; font-weight: bold; }
        .timestamp { color: #6c757d; font-size: 0.9em; float: right; }
    </style>
</head>
<body>
    <div class="container">
        <h1>🛡️ Risk Envelope Status <span class="timestamp">%s</span></h1>
        
        <div class="section">
            <h2>Overall Status</h2>
            <div class="metric">
                <span class="metric-name">Health Status:</span>
                <span class="status %s">%s</span>
            </div>
            <div class="metric">
                <span class="metric-name">Active Violations:</span>
                <span class="metric-value">%d</span>
            </div>
            <div class="metric">
                <span class="metric-name">Breaches:</span>
                <span class="metric-value">%d</span>
            </div>
        </div>

        <div class="section">
            <h2>🌌 Universe Status</h2>
            <div class="metric">
                <span class="metric-name">Symbol Count:</span>
                <span class="metric-value">%s</span>
            </div>
            <div class="metric">
                <span class="metric-name">Hash:</span>
                <span class="metric-value">%s</span>
            </div>
            <div class="metric">
                <span class="metric-name">USD Compliance:</span>
                <span class="metric-value">%s</span>
            </div>
            <div class="metric">
                <span class="metric-name">ADV Compliance:</span>
                <span class="metric-value">%s</span>
            </div>
            <div class="metric">
                <span class="metric-name">Integrity Check:</span>
                <span class="metric-value">%s</span>
            </div>
        </div>

        <div class="section">
            <h2>📊 Position Management</h2>
            <div class="metric">
                <span class="metric-name">Active Positions:</span>
                <span class="metric-value">%s</span>
            </div>
            <div class="metric">
                <span class="metric-name">Utilization:</span>
                <span class="metric-value">%s</span>
            </div>
            <div class="metric">
                <span class="metric-name">Total Exposure:</span>
                <span class="metric-value">%s</span>
            </div>
        </div>

        <div class="section">
            <h2>⚠️ Risk Limits</h2>
            <div class="metric">
                <span class="metric-name">Drawdown:</span>
                <span class="metric-value">%s</span>
            </div>
            <div class="metric">
                <span class="metric-name">Max Single Asset:</span>
                <span class="metric-value">%s</span>
            </div>
            <div class="metric">
                <span class="metric-name">Max Correlation:</span>
                <span class="metric-value">%s</span>
            </div>
            <div class="metric">
                <span class="metric-name">Sector Breaches:</span>
                <span class="metric-value">%d</span>
            </div>
        </div>

        <div class="section">
            <h2>🚨 Emergency Controls</h2>
            <div class="metric">
                <span class="metric-name">Global Pause:</span>
                <span class="metric-value %s">%t</span>
            </div>
            <div class="metric">
                <span class="metric-name">Blacklisted Symbols:</span>
                <span class="metric-value">%d</span>
            </div>
            <div class="metric">
                <span class="metric-name">Degraded Mode:</span>
                <span class="metric-value">%t</span>
            </div>
        </div>

        <div style="margin-top: 30px; padding: 15px; background: #e9ecef; border-radius: 4px;">
            <strong>Endpoints:</strong><br>
            <a href="/risk?format=prometheus">/risk?format=prometheus</a> - Prometheus metrics<br>
            <a href="/metrics">/metrics</a> - Combined system metrics<br>
            <a href="/health">/health</a> - Health check
        </div>
    </div>
</body>
</html>`

	// Extract values from summary
	status := summary["status"].(string)
	statusClass := strings.ToLower(status)
	violations := summary["violations"].(int)
	breaches := summary["breaches"].(int)
	timestamp := summary["timestamp"].(string)

	universe := summary["universe"].(map[string]interface{})
	positions := summary["positions"].(map[string]interface{})
	riskLimits := summary["risk_limits"].(map[string]interface{})
	emergency := summary["emergency"].(map[string]interface{})

	pauseClass := ""
	if emergency["global_pause"].(bool) {
		pauseClass = "violation"
	}

	formattedHTML := fmt.Sprintf(html,
		timestamp,
		statusClass, status,
		violations, breaches,
		fmt.Sprintf("%v", universe["symbols"]),
		universe["hash"].(string),
		universe["usd_compliance"].(string),
		universe["adv_compliance"].(string),
		universe["integrity"].(string),
		positions["active"].(string),
		positions["utilization"].(string),
		positions["total_exposure"].(string),
		riskLimits["drawdown"].(string),
		riskLimits["max_single_asset"].(string),
		riskLimits["max_correlation"].(string),
		riskLimits["sector_breaches"].(int),
		pauseClass, emergency["global_pause"].(bool),
		emergency["blacklisted_symbols"].(int),
		emergency["degraded_mode"].(bool),
	)

	w.Header().Set("Content-Type", "text/html")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(formattedHTML))
}

// renderMetricsWithRiskHTML renders the standard metrics page with risk envelope summary
func renderMetricsWithRiskHTML(w http.ResponseWriter, monitor *application.RiskMonitor) {
	riskSummary := monitor.GetRiskEnvelopeSummary()

	html := `<!DOCTYPE html>
<html>
<head>
    <title>CryptoRun - System Metrics</title>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <style>
        body { font-family: 'Courier New', monospace; margin: 40px; background-color: #f5f5f5; }
        .container { background: white; padding: 30px; border-radius: 8px; box-shadow: 0 2px 10px rgba(0,0,0,0.1); }
        h1 { color: #333; border-bottom: 2px solid #007acc; padding-bottom: 10px; }
        h2 { color: #555; margin-top: 30px; }
        .status { font-weight: bold; padding: 5px 10px; border-radius: 4px; }
        .healthy { background-color: #d4edda; color: #155724; }
        .degraded { background-color: #fff3cd; color: #856404; }
        .paused { background-color: #f8d7da; color: #721c24; }
        .metric { margin: 10px 0; }
        .metric-name { display: inline-block; width: 200px; font-weight: bold; }
        .metric-value { color: #007acc; }
        .section { margin: 20px 0; padding: 15px; border-left: 4px solid #007acc; background: #f8f9fa; }
        .risk-summary { border-left-color: #ffc107; }
        .timestamp { color: #6c757d; font-size: 0.9em; float: right; }
    </style>
</head>
<body>
    <div class="container">
        <h1>📊 CryptoRun System Metrics <span class="timestamp">%s</span></h1>
        
        <div class="section risk-summary">
            <h2>🛡️ Risk Envelope Summary</h2>
            <div class="metric">
                <span class="metric-name">Status:</span>
                <span class="status %s">%s</span>
            </div>
            <div class="metric">
                <span class="metric-name">Active Positions:</span>
                <span class="metric-value">%s</span>
            </div>
            <div class="metric">
                <span class="metric-name">Current Drawdown:</span>
                <span class="metric-value">%s</span>
            </div>
            <div class="metric">
                <span class="metric-name">Universe Symbols:</span>
                <span class="metric-value">%v</span>
            </div>
            <div class="metric">
                <span class="metric-name">Violations/Breaches:</span>
                <span class="metric-value">%d / %d</span>
            </div>
        </div>

        <div class="section">
            <h2>🔗 Available Endpoints</h2>
            <div style="font-family: monospace;">
                <div><a href="/health">/health</a> - System health check</div>
                <div><a href="/metrics">/metrics</a> - Complete system metrics</div>
                <div><a href="/metrics?format=prometheus">/metrics?format=prometheus</a> - Prometheus format</div>
                <div><a href="/decile">/decile</a> - Decile analysis</div>
                <div><a href="/risk">/risk</a> - Risk envelope dashboard</div>
                <div><a href="/risk?format=prometheus">/risk?format=prometheus</a> - Risk metrics (Prometheus)</div>
            </div>
        </div>

        <div style="margin-top: 30px; padding: 15px; background: #e9ecef; border-radius: 4px; font-size: 0.9em;">
            <strong>Note:</strong> This is a simplified view. For detailed metrics, use the JSON endpoints or Prometheus format.
            Risk envelope includes position limits (ATR-based), portfolio concentration caps, correlation clustering detection,
            emergency controls (8%% drawdown pause), and symbol blacklisting.
        </div>
    </div>
</body>
</html>`

	status := riskSummary["status"].(string)
	statusClass := strings.ToLower(status)
	timestamp := riskSummary["timestamp"].(string)

	positions := riskSummary["positions"].(map[string]interface{})
	riskLimits := riskSummary["risk_limits"].(map[string]interface{})
	universe := riskSummary["universe"].(map[string]interface{})
	violations := riskSummary["violations"].(int)
	breaches := riskSummary["breaches"].(int)

	formattedHTML := fmt.Sprintf(html,
		timestamp,
		statusClass, status,
		positions["active"].(string),
		riskLimits["drawdown"].(string),
		universe["symbols"],
		violations, breaches,
	)

	w.Header().Set("Content-Type", "text/html")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(formattedHTML))
}
