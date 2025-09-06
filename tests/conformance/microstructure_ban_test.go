package conformance

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestAggregatorBanEnforcement ensures depth/spread data MUST be exchange-native
func TestAggregatorBanEnforcement(t *testing.T) {
	// Check all microstructure-related files for aggregator usage
	microstructurePaths := []string{
		filepath.Join("..", "..", "internal", "domain", "gates.go"),
		filepath.Join("..", "..", "internal", "infrastructure", "apis", "kraken", "client.go"),
		filepath.Join("..", "..", "internal", "infrastructure", "apis", "manager.go"),
	}

	for _, path := range microstructurePaths {
		if !fileExists(path) {
			continue // Skip missing files
		}

		content, err := readFileContent(path)
		if err != nil {
			t.Errorf("CONFORMANCE VIOLATION: Cannot read %s: %v", path, err)
			continue
		}

		// Forbidden aggregator APIs for microstructure data
		forbiddenAggregators := []string{
			"coingecko", "CoinGecko", "COINGECKO",
			"dexscreener", "DexScreener", "DEXSCREENER",
			"coinmarketcap", "CoinMarketCap", "COINMARKETCAP",
			"nomics", "Nomics", "NOMICS",
			"messari", "Messari", "MESSARI",
		}

		// Context patterns that indicate microstructure usage
		microstructurePatterns := []string{
			"depth", "Depth", "spread", "Spread", "bid", "ask", "orderbook", "OrderBook",
			"VADR", "liquidity", "Liquidity",
		}

		lines := strings.Split(content, "\n")
		for lineNum, line := range lines {
			lineUpper := strings.ToUpper(line)

			// Check if line contains microstructure context
			hasMicrostructureContext := false
			for _, pattern := range microstructurePatterns {
				if strings.Contains(line, pattern) {
					hasMicrostructureContext = true
					break
				}
			}

			if hasMicrostructureContext {
				// Now check for forbidden aggregators in this context
				for _, aggregator := range forbiddenAggregators {
					if strings.Contains(lineUpper, strings.ToUpper(aggregator)) {
						t.Errorf("CONFORMANCE VIOLATION: %s:%d uses aggregator '%s' for microstructure data: %s",
							filepath.Base(path), lineNum+1, aggregator, strings.TrimSpace(line))
					}
				}
			}
		}
	}
}

// TestExchangeNativeOnlyEnforcement verifies only approved exchanges for microstructure
func TestExchangeNativeOnlyEnforcement(t *testing.T) {
	microstructurePaths := []string{
		filepath.Join("..", "..", "internal", "domain", "gates.go"),
		filepath.Join("..", "..", "internal", "infrastructure", "apis"),
	}

	// Approved exchange-native sources for microstructure
	approvedExchanges := []string{
		"binance", "Binance", "BINANCE",
		"kraken", "Kraken", "KRAKEN",
		"coinbase", "Coinbase", "COINBASE",
		"okx", "OKX", "okx",
	}

	for _, basePath := range microstructurePaths {
		if strings.HasSuffix(basePath, "apis") {
			// Check API directory for exchange implementations
			checkExchangeAPIsDirectory(t, basePath, approvedExchanges)
		} else if fileExists(basePath) {
			content, err := readFileContent(basePath)
			if err != nil {
				t.Errorf("CONFORMANCE VIOLATION: Cannot read %s: %v", basePath, err)
				continue
			}

			validateExchangeNativeUsage(t, filepath.Base(basePath), content, approvedExchanges)
		}
	}
}

// TestDepthSpreadSourceValidation ensures depth/spread only from exchange APIs
func TestDepthSpreadSourceValidation(t *testing.T) {
	gatesPath := filepath.Join("..", "..", "internal", "domain", "gates.go")
	if !fileExists(gatesPath) {
		t.Skip("CONFORMANCE SKIP: gates.go not found")
		return
	}

	content, err := readFileContent(gatesPath)
	if err != nil {
		t.Fatalf("CONFORMANCE VIOLATION: Cannot read gates.go: %v", err)
	}

	// Look for depth/spread calculation functions
	lines := strings.Split(content, "\n")
	for lineNum, line := range lines {
		if strings.Contains(line, "func") &&
			(strings.Contains(strings.ToLower(line), "depth") || strings.Contains(strings.ToLower(line), "spread")) {

			// Found depth/spread function - check for proper source validation
			functionBlock := extractFunctionBlock(lines, lineNum)

			// Must validate exchange-native source
			requiredChecks := []string{
				"exchange", "Exchange", "native", "Native",
				"source", "Source",
			}

			foundValidation := false
			for _, check := range requiredChecks {
				if strings.Contains(functionBlock, check) {
					foundValidation = true
					break
				}
			}

			if !foundValidation {
				t.Errorf("CONFORMANCE VIOLATION: gates.go:%d depth/spread function lacks exchange-native source validation", lineNum+1)
			}

			// Must not accept aggregator data
			forbiddenInFunction := []string{
				"aggregator", "Aggregator",
				"coingecko", "dexscreener",
			}

			for _, forbidden := range forbiddenInFunction {
				if strings.Contains(strings.ToLower(functionBlock), strings.ToLower(forbidden)) {
					t.Errorf("CONFORMANCE VIOLATION: gates.go:%d depth/spread function accepts aggregator data", lineNum+1)
				}
			}
		}
	}
}

// TestVADRCalculationCompliance ensures VADR uses only exchange orderbook data
func TestVADRCalculationCompliance(t *testing.T) {
	vadrPath := filepath.Join("..", "..", "internal", "domain", "vadr.go")
	if !fileExists(vadrPath) {
		t.Skip("CONFORMANCE SKIP: vadr.go not found")
		return
	}

	content, err := readFileContent(vadrPath)
	if err != nil {
		t.Fatalf("CONFORMANCE VIOLATION: Cannot read vadr.go: %v", err)
	}

	// VADR must use orderbook data, not aggregated volume
	requiredPatterns := []string{
		"orderbook", "OrderBook", "order_book",
		"bid", "ask", "depth",
	}

	foundOrderBookUsage := false
	for _, pattern := range requiredPatterns {
		if strings.Contains(content, pattern) {
			foundOrderBookUsage = true
			break
		}
	}

	if !foundOrderBookUsage {
		t.Error("CONFORMANCE VIOLATION: vadr.go does not reference orderbook data for VADR calculation")
	}

	// Must not use aggregated volume APIs
	forbiddenVolumeAPIs := []string{
		"coingecko.*volume", "dexscreener.*volume",
		"24h.*volume", "daily.*volume",
	}

	for _, forbidden := range forbiddenVolumeAPIs {
		if containsPattern(content, forbidden) {
			t.Errorf("CONFORMANCE VIOLATION: vadr.go uses aggregated volume API pattern '%s'", forbidden)
		}
	}
}

// TestMicrostructureConfigValidation ensures config enforces exchange-native requirement
func TestMicrostructureConfigValidation(t *testing.T) {
	configPaths := []string{
		filepath.Join("..", "..", "config", "apis.yaml"),
		filepath.Join("..", "..", "config", "microstructure.yaml"),
	}

	for _, configPath := range configPaths {
		if !fileExists(configPath) {
			continue
		}

		content, err := readFileContent(configPath)
		if err != nil {
			t.Errorf("CONFORMANCE VIOLATION: Cannot read %s: %v", configPath, err)
			continue
		}

		// Config must specify exchange-native requirement for microstructure
		if strings.Contains(content, "microstructure") || strings.Contains(content, "depth") || strings.Contains(content, "spread") {
			requiredSettings := []string{
				"exchange_native", "exchange-native", "exchangeNative",
				"native_only", "native-only", "nativeOnly",
			}

			foundRequirement := false
			for _, setting := range requiredSettings {
				if strings.Contains(content, setting) {
					foundRequirement = true
					break
				}
			}

			if !foundRequirement {
				t.Errorf("CONFORMANCE VIOLATION: %s microstructure config lacks exchange-native requirement", filepath.Base(configPath))
			}
		}
	}
}

// Helper functions
func checkExchangeAPIsDirectory(t *testing.T, apisPath string, approvedExchanges []string) {
	entries, err := os.ReadDir(apisPath)
	if err != nil {
		return // Directory doesn't exist or not accessible
	}

	for _, entry := range entries {
		if entry.IsDir() {
			dirName := entry.Name()

			// Check if directory name matches approved exchanges
			isApproved := false
			for _, approved := range approvedExchanges {
				if strings.ToLower(dirName) == strings.ToLower(approved) {
					isApproved = true
					break
				}
			}

			// Special cases for reference implementations
			if strings.Contains(dirName, "reference") || strings.Contains(dirName, "mock") {
				isApproved = true
			}

			if !isApproved {
				t.Errorf("CONFORMANCE VIOLATION: Unapproved exchange API directory '%s' found in %s", dirName, apisPath)
			}
		}
	}
}

func validateExchangeNativeUsage(t *testing.T, filename, content string, approvedExchanges []string) {
	lines := strings.Split(content, "\n")

	for lineNum, line := range lines {
		// Look for API endpoint definitions or client connections
		if strings.Contains(line, "http") && (strings.Contains(line, "depth") || strings.Contains(line, "orderbook")) {

			// Check if URL is from approved exchange
			foundApproved := false
			for _, exchange := range approvedExchanges {
				if strings.Contains(strings.ToLower(line), strings.ToLower(exchange)) {
					foundApproved = true
					break
				}
			}

			if !foundApproved {
				t.Errorf("CONFORMANCE VIOLATION: %s:%d uses non-approved exchange for microstructure: %s",
					filename, lineNum+1, strings.TrimSpace(line))
			}
		}
	}
}

func extractFunctionBlock(lines []string, startLine int) string {
	if startLine >= len(lines) {
		return ""
	}

	var block strings.Builder
	braceCount := 0
	started := false

	for i := startLine; i < len(lines) && i < startLine+50; i++ { // Limit to 50 lines
		line := lines[i]
		block.WriteString(line)
		block.WriteString("\n")

		// Count braces to find function end
		for _, char := range line {
			if char == '{' {
				braceCount++
				started = true
			} else if char == '}' {
				braceCount--
				if started && braceCount == 0 {
					return block.String()
				}
			}
		}
	}

	return block.String()
}
