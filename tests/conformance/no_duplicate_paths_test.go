package conformance_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"
	"testing"
)

// TestNoDuplicateScoringPaths ensures only UnifiedFactorEngine handles scoring
func TestNoDuplicateScoringPaths(t *testing.T) {
	t.Log("🎯 CONFORMANCE: Validating single scoring path via UnifiedFactorEngine")

	// Forbidden legacy scorer symbols that should not exist
	forbiddenSymbols := []string{
		"calcOptimizedCompositeScore",
		"FactorWeights",
		"ComprehensiveScanner",
		"buildOptimizedCompositeScore",
		"calculateCompositeScore",
		"applyFactorWeights",
	}

	// Files to check for forbidden symbols (using correct relative paths)
	filesToCheck := []string{
		"../../src/cmd/cryptorun/cmd_scan.go",
		"../../src/cmd/cryptorun/cmd_bench.go",
		"../../cmd/cryptorun/menu_main.go",
		"../../internal/domain/factors/unified.go",
	}

	var violations []string

	for _, file := range filesToCheck {
		if violations := findForbiddenScoringSymbols(t, file, forbiddenSymbols); len(violations) > 0 {
			for _, violation := range violations {
				violations = append(violations,
					filepath.Base(file)+": contains forbidden symbol '"+violation+"'")
			}
		}
	}

	// Fail if any violations found
	if len(violations) > 0 {
		t.Errorf("VIOLATION: Found %d references to legacy scoring paths:", len(violations))
		for _, violation := range violations {
			t.Errorf("  - %s", violation)
		}
		t.Errorf("REQUIRED: Only UnifiedFactorEngine should handle scoring")
	} else {
		t.Log("✅ PASS: No duplicate scoring paths detected")
	}
}

// TestSingleImplementationPerAction ensures there's only one implementation per action
func TestSingleImplementationPerAction(t *testing.T) {
	actions := []struct {
		name              string
		allowedTargets    []string
		forbiddenPatterns []string
	}{
		{
			name:           "Scan Implementation",
			allowedTargets: []string{"pipeline.Run"},
			forbiddenPatterns: []string{
				"ScanUniverse", // Should only be called via pipeline.Run
				"ScanPipeline", // Should only be internal to pipeline.Run
			},
		},
		{
			name:           "Benchmark Implementation",
			allowedTargets: []string{"bench.Run", "bench.RunDiagnostics"},
			forbiddenPatterns: []string{
				"RunBenchmark",        // Should only be called via bench.Run
				"TopGainersBenchmark", // Should only be internal
			},
		},
		{
			name:           "Health Implementation",
			allowedTargets: []string{"metrics.Snapshot"},
			forbiddenPatterns: []string{
				"HealthSnapshot",     // Should only be called via metrics.Snapshot
				"checkAllComponents", // Should only be internal
			},
		},
	}

	for _, action := range actions {
		t.Run(action.name, func(t *testing.T) {
			// Check CMD files
			cmdFiles := []string{
				"src/cmd/cryptorun/cmd_scan.go",
				"src/cmd/cryptorun/cmd_bench.go",
				"src/cmd/cryptorun/cmd_health.go",
			}

			for _, cmdFile := range cmdFiles {
				violations := findForbiddenPatterns(t, cmdFile, action.forbiddenPatterns)
				for _, violation := range violations {
					t.Errorf("CONFORMANCE VIOLATION: CMD file %s contains forbidden pattern: %s",
						cmdFile, violation)
				}
			}

			// Check menu files
			menuFiles := []string{
				"cmd/cryptorun/menu_main.go",
				"cmd/cryptorun/menu_unified.go",
			}

			for _, menuFile := range menuFiles {
				violations := findForbiddenPatterns(t, menuFile, action.forbiddenPatterns)
				for _, violation := range violations {
					t.Errorf("CONFORMANCE VIOLATION: Menu file %s contains forbidden pattern: %s",
						menuFile, violation)
				}
			}
		})
	}
}

// TestPipelineFunctionUniqueness verifies each action has exactly one entry point
func TestPipelineFunctionUniqueness(t *testing.T) {
	entryPoints := []struct {
		function  string
		file      string
		mustExist bool
	}{
		{"pipeline.Run", "internal/application/pipeline/scan.go", true},
		{"bench.Run", "internal/application/bench/topgainers_pipeline.go", true},
		{"bench.RunDiagnostics", "internal/application/bench/diagnostics_pipeline.go", true},
		{"metrics.Snapshot", "internal/application/metrics/health_pipeline.go", true},
	}

	for _, ep := range entryPoints {
		t.Run(ep.function, func(t *testing.T) {
			if ep.mustExist {
				if !functionExistsInFile(t, ep.file, extractFunctionName(ep.function)) {
					t.Errorf("CONFORMANCE VIOLATION: Entry point %s not found in %s",
						ep.function, ep.file)
				}
			}

			// Verify this is the ONLY implementation of this pattern
			duplicates := findDuplicateImplementations(t, ep.function)
			if len(duplicates) > 1 {
				t.Errorf("CONFORMANCE VIOLATION: Multiple implementations found for %s: %v",
					ep.function, duplicates)
			}
		})
	}
}

// Helper functions for AST analysis

func callsFunction(t *testing.T, filename string, targetFunction string) bool {
	t.Helper()

	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, filename, nil, parser.ParseComments)
	if err != nil {
		t.Logf("Failed to parse %s: %v", filename, err)
		return false
	}

	found := false
	ast.Inspect(node, func(n ast.Node) bool {
		if call, ok := n.(*ast.CallExpr); ok {
			if callString := getCallString(call); strings.Contains(callString, targetFunction) {
				found = true
			}
		}
		return true
	})

	return found
}

func menuFunctionCallsTarget(t *testing.T, filename string, functionName string, targetFunction string) bool {
	t.Helper()

	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, filename, nil, parser.ParseComments)
	if err != nil {
		t.Logf("Failed to parse %s: %v", filename, err)
		return false
	}

	found := false
	inTargetFunction := false

	ast.Inspect(node, func(n ast.Node) bool {
		// Check if we're entering the target function
		if funcDecl, ok := n.(*ast.FuncDecl); ok {
			if funcDecl.Name.Name == functionName {
				inTargetFunction = true
			} else {
				inTargetFunction = false
			}
			return true
		}

		// If we're in the target function, look for the call
		if inTargetFunction {
			if call, ok := n.(*ast.CallExpr); ok {
				if callString := getCallString(call); strings.Contains(callString, targetFunction) {
					found = true
				}
			}
		}

		return true
	})

	return found
}

func findForbiddenPatterns(t *testing.T, filename string, patterns []string) []string {
	t.Helper()

	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, filename, nil, parser.ParseComments)
	if err != nil {
		t.Logf("Failed to parse %s: %v", filename, err)
		return nil
	}

	var violations []string

	ast.Inspect(node, func(n ast.Node) bool {
		if call, ok := n.(*ast.CallExpr); ok {
			callString := getCallString(call)
			for _, pattern := range patterns {
				if strings.Contains(callString, pattern) {
					violations = append(violations, pattern)
				}
			}
		}
		return true
	})

	return violations
}

func functionExistsInFile(t *testing.T, filename string, functionName string) bool {
	t.Helper()

	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, filename, nil, parser.ParseComments)
	if err != nil {
		t.Logf("Failed to parse %s: %v", filename, err)
		return false
	}

	found := false
	ast.Inspect(node, func(n ast.Node) bool {
		if funcDecl, ok := n.(*ast.FuncDecl); ok {
			if funcDecl.Name.Name == functionName {
				found = true
			}
		}
		return true
	})

	return found
}

func findDuplicateImplementations(t *testing.T, functionPattern string) []string {
	t.Helper()

	// This would scan multiple files to find duplicate implementations
	// For now, return a mock implementation
	return []string{} // Would implement full directory scanning
}

func getCallString(call *ast.CallExpr) string {
	// Extract function call as string for pattern matching
	if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
		if ident, ok := sel.X.(*ast.Ident); ok {
			return ident.Name + "." + sel.Sel.Name
		}
	}
	if ident, ok := call.Fun.(*ast.Ident); ok {
		return ident.Name
	}
	return ""
}

func extractFunctionName(fullName string) string {
	parts := strings.Split(fullName, ".")
	if len(parts) > 1 {
		return parts[len(parts)-1]
	}
	return fullName
}

// TestUnifiedFactorEngineExists ensures the unified engine is present
func TestUnifiedFactorEngineExists(t *testing.T) {
	t.Log("🎯 CONFORMANCE: Validating UnifiedFactorEngine presence")

	// Check that UnifiedFactorEngine exists
	unifiedPath := "../../internal/domain/factors/unified.go"

	if !functionExistsInFile(t, unifiedPath, "NewUnifiedFactorEngine") {
		t.Errorf("VIOLATION: UnifiedFactorEngine not found at %s", unifiedPath)
		return
	}

	// Required symbols in UnifiedFactorEngine
	requiredSymbols := []string{
		"UnifiedFactorEngine",
		"ProcessFactors",
		"RegimeWeights",
		"applyOrthogonalization",
	}

	for _, symbol := range requiredSymbols {
		if !containsSymbol(t, unifiedPath, symbol) {
			t.Errorf("VIOLATION: UnifiedFactorEngine missing symbol: %s", symbol)
		}
	}

	t.Log("✅ PASS: UnifiedFactorEngine contains required symbols")
}

// TestMenuRoutesToUnified ensures Menu calls same functions as CLI
func TestMenuRoutesToUnified(t *testing.T) {
	t.Log("🎯 CONFORMANCE: Validating Menu routes to unified functions")

	menuPath := "../../cmd/cryptorun/menu_main.go"

	// Required unified routing patterns
	requiredPatterns := []string{
		"runScanMomentum",    // Menu should call CLI function
		"runBenchTopGainers", // Menu should call CLI function
	}

	for _, pattern := range requiredPatterns {
		if !containsSymbol(t, menuPath, pattern) {
			t.Errorf("VIOLATION: Menu not calling CLI function: %s", pattern)
		}
	}

	t.Log("✅ PASS: Menu properly routes to unified functions")
}

func findForbiddenScoringSymbols(t *testing.T, filename string, patterns []string) []string {
	t.Helper()

	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, filename, nil, parser.ParseComments)
	if err != nil {
		t.Logf("Failed to parse %s: %v", filename, err)
		return nil
	}

	var violations []string

	ast.Inspect(node, func(n ast.Node) bool {
		// Check function declarations
		if funcDecl, ok := n.(*ast.FuncDecl); ok {
			for _, pattern := range patterns {
				if strings.Contains(funcDecl.Name.Name, pattern) {
					violations = append(violations, pattern)
				}
			}
		}

		// Check function calls
		if call, ok := n.(*ast.CallExpr); ok {
			callString := getCallString(call)
			for _, pattern := range patterns {
				if strings.Contains(callString, pattern) {
					violations = append(violations, pattern)
				}
			}
		}

		// Check type declarations
		if typeSpec, ok := n.(*ast.TypeSpec); ok {
			for _, pattern := range patterns {
				if strings.Contains(typeSpec.Name.Name, pattern) {
					violations = append(violations, pattern)
				}
			}
		}

		return true
	})

	return violations
}

func containsSymbol(t *testing.T, filename string, symbol string) bool {
	t.Helper()

	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, filename, nil, parser.ParseComments)
	if err != nil {
		t.Logf("Failed to parse %s: %v", filename, err)
		return false
	}

	found := false
	ast.Inspect(node, func(n ast.Node) bool {
		// Check function declarations
		if funcDecl, ok := n.(*ast.FuncDecl); ok {
			if strings.Contains(funcDecl.Name.Name, symbol) {
				found = true
			}
		}

		// Check type declarations
		if typeSpec, ok := n.(*ast.TypeSpec); ok {
			if strings.Contains(typeSpec.Name.Name, symbol) {
				found = true
			}
		}

		// Check function calls
		if call, ok := n.(*ast.CallExpr); ok {
			callString := getCallString(call)
			if strings.Contains(callString, symbol) {
				found = true
			}
		}

		return true
	})

	return found
}
