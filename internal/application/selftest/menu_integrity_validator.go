package selftest

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// MenuIntegrityValidator validates CLI menu structure and command integrity
type MenuIntegrityValidator struct{}

// NewMenuIntegrityValidator creates a new menu integrity validator
func NewMenuIntegrityValidator() *MenuIntegrityValidator {
	return &MenuIntegrityValidator{}
}

// Name returns the validator name
func (miv *MenuIntegrityValidator) Name() string {
	return "Menu Integrity Validation"
}

// CommandInfo represents information about a CLI command
type CommandInfo struct {
	Name         string
	File         string
	HasHelp      bool
	HasExample   bool
	IsComplete   bool
	Dependencies []string
}

// Validate checks CLI menu integrity and command completeness
func (miv *MenuIntegrityValidator) Validate() TestResult {
	start := time.Now()
	result := TestResult{
		Name:      miv.Name(),
		Timestamp: start,
		Details:   []string{},
	}

	// Check 1: Scan for command files
	commands, err := miv.scanCommandFiles()
	if err != nil {
		result.Status = "FAIL"
		result.Message = fmt.Sprintf("Failed to scan command files: %v", err)
		result.Duration = time.Since(start)
		return result
	}

	result.Details = append(result.Details, fmt.Sprintf("Found %d command files", len(commands)))

	// Check 2: Validate each command
	validCommands := 0
	for _, cmd := range commands {
		if miv.validateCommand(cmd) {
			validCommands++
			result.Details = append(result.Details, fmt.Sprintf("✅ Command %s: valid", cmd.Name))
		} else {
			result.Details = append(result.Details, fmt.Sprintf("❌ Command %s: issues found", cmd.Name))
		}

		// Add detailed checks
		if cmd.HasHelp {
			result.Details = append(result.Details, fmt.Sprintf("   Help text: present"))
		} else {
			result.Details = append(result.Details, fmt.Sprintf("   Help text: missing"))
		}

		if cmd.HasExample {
			result.Details = append(result.Details, fmt.Sprintf("   Examples: present"))
		} else {
			result.Details = append(result.Details, fmt.Sprintf("   Examples: missing"))
		}

		if len(cmd.Dependencies) > 0 {
			result.Details = append(result.Details, fmt.Sprintf("   Dependencies: %s", strings.Join(cmd.Dependencies, ", ")))
		}
	}

	// Check 3: Verify core commands exist
	requiredCommands := []string{"scan", "monitor", "health", "optimize", "selftest", "digest"}
	missingCommands := []string{}

	commandMap := make(map[string]bool)
	for _, cmd := range commands {
		commandMap[cmd.Name] = true
	}

	for _, required := range requiredCommands {
		if !commandMap[required] {
			missingCommands = append(missingCommands, required)
		}
	}

	if len(missingCommands) > 0 {
		result.Status = "FAIL"
		result.Message = fmt.Sprintf("Missing required commands: %s", strings.Join(missingCommands, ", "))
		result.Details = append(result.Details, "Missing required commands:")
		for _, cmd := range missingCommands {
			result.Details = append(result.Details, fmt.Sprintf("  - %s", cmd))
		}
	} else {
		result.Details = append(result.Details, "All required commands present")
	}

	// Check 4: Validate main.go integration
	mainIntegration := miv.validateMainIntegration(commands)
	if !mainIntegration {
		result.Status = "FAIL"
		result.Message = "Main.go integration issues detected"
		result.Details = append(result.Details, "❌ Main.go integration: failed")
	} else {
		result.Details = append(result.Details, "✅ Main.go integration: valid")
	}

	// Check 5: Validate help system
	helpSystem := miv.validateHelpSystem()
	if !helpSystem {
		result.Status = "FAIL"
		result.Message = "Help system issues detected"
		result.Details = append(result.Details, "❌ Help system: failed")
	} else {
		result.Details = append(result.Details, "✅ Help system: valid")
	}

	// Overall result
	if result.Status == "" {
		if validCommands == len(commands) && len(missingCommands) == 0 {
			result.Status = "PASS"
			result.Message = fmt.Sprintf("Menu integrity validation passed: %d/%d commands valid", validCommands, len(commands))
		} else {
			result.Status = "FAIL"
			result.Message = fmt.Sprintf("Menu integrity issues: %d/%d commands valid", validCommands, len(commands))
		}
	}

	result.Duration = time.Since(start)
	return result
}

// scanCommandFiles scans for CLI command files
func (miv *MenuIntegrityValidator) scanCommandFiles() ([]CommandInfo, error) {
	commands := []CommandInfo{}

	// Look for command files in expected locations
	searchPaths := []string{
		"src/cmd/cryptorun",
		"cmd/cryptorun",
	}

	for _, searchPath := range searchPaths {
		if _, err := os.Stat(searchPath); os.IsNotExist(err) {
			continue
		}

		entries, err := os.ReadDir(searchPath)
		if err != nil {
			continue
		}

		for _, entry := range entries {
			if entry.IsDir() || !strings.HasPrefix(entry.Name(), "cmd_") || !strings.HasSuffix(entry.Name(), ".go") {
				continue
			}

			// Extract command name from filename (cmd_scan.go -> scan)
			cmdName := strings.TrimPrefix(entry.Name(), "cmd_")
			cmdName = strings.TrimSuffix(cmdName, ".go")

			filePath := filepath.Join(searchPath, entry.Name())
			cmd := miv.analyzeCommandFile(cmdName, filePath)
			commands = append(commands, cmd)
		}
	}

	return commands, nil
}

// analyzeCommandFile analyzes a command file for completeness
func (miv *MenuIntegrityValidator) analyzeCommandFile(name, filePath string) CommandInfo {
	cmd := CommandInfo{
		Name:         name,
		File:         filePath,
		Dependencies: []string{},
	}

	content, err := os.ReadFile(filePath)
	if err != nil {
		return cmd
	}

	fileContent := string(content)

	// Check for help text
	if strings.Contains(fileContent, "Short:") && strings.Contains(fileContent, "Long:") {
		cmd.HasHelp = true
	}

	// Check for examples
	if strings.Contains(fileContent, "Example") || strings.Contains(fileContent, "example") ||
		strings.Contains(fileContent, "Usage:") {
		cmd.HasExample = true
	}

	// Check for dependencies (imports from internal packages)
	lines := strings.Split(fileContent, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "\"cryptorun/internal/") {
			// Extract package name
			pkg := strings.Trim(line, "\"")
			parts := strings.Split(pkg, "/")
			if len(parts) > 0 {
				cmd.Dependencies = append(cmd.Dependencies, parts[len(parts)-1])
			}
		}
	}

	// Basic completeness check
	cmd.IsComplete = cmd.HasHelp && len(cmd.Dependencies) > 0 &&
		strings.Contains(fileContent, "RunE:") &&
		strings.Contains(fileContent, "cobra.Command")

	return cmd
}

// validateCommand validates individual command structure
func (miv *MenuIntegrityValidator) validateCommand(cmd CommandInfo) bool {
	// Command is valid if:
	// 1. Has help text
	// 2. Is properly structured (has RunE function)
	// 3. Has reasonable dependencies

	if !cmd.HasHelp {
		return false
	}

	if !cmd.IsComplete {
		return false
	}

	// Special validation for core commands
	switch cmd.Name {
	case "scan":
		// Scan should have scanner dependencies
		return miv.hasAnyDependency(cmd, []string{"scanner", "application"})
	case "monitor":
		// Monitor should have server/http dependencies
		return miv.hasAnyDependency(cmd, []string{"interfaces", "application"})
	case "optimize":
		// Optimize should have optimization dependencies
		return miv.hasAnyDependency(cmd, []string{"optimization"})
	case "selftest":
		// Selftest should have selftest dependencies
		return miv.hasAnyDependency(cmd, []string{"selftest"})
	default:
		return true // Other commands pass basic validation
	}
}

// hasAnyDependency checks if command has any of the specified dependencies
func (miv *MenuIntegrityValidator) hasAnyDependency(cmd CommandInfo, required []string) bool {
	for _, dep := range cmd.Dependencies {
		for _, req := range required {
			if strings.Contains(dep, req) {
				return true
			}
		}
	}
	return false
}

// validateMainIntegration checks if commands are properly integrated in main.go
func (miv *MenuIntegrityValidator) validateMainIntegration(commands []CommandInfo) bool {
	// Look for main.go or root.go
	mainFiles := []string{
		"src/cmd/cryptorun/main.go",
		"src/cmd/cryptorun/root.go",
		"cmd/cryptorun/main.go",
		"cmd/cryptorun/root.go",
	}

	for _, mainFile := range mainFiles {
		if content, err := os.ReadFile(mainFile); err == nil {
			mainContent := string(content)

			// Check that commands are registered
			registeredCommands := 0
			for _, cmd := range commands {
				// Look for command registration patterns
				patterns := []string{
					fmt.Sprintf("new%sCmd", strings.Title(cmd.Name)),
					fmt.Sprintf("cmd_%s", cmd.Name),
					fmt.Sprintf("AddCommand(%s", cmd.Name),
				}

				for _, pattern := range patterns {
					if strings.Contains(mainContent, pattern) {
						registeredCommands++
						break
					}
				}
			}

			// At least 80% of commands should be registered
			return registeredCommands >= len(commands)*4/5
		}
	}

	return false
}

// validateHelpSystem checks if help system is working
func (miv *MenuIntegrityValidator) validateHelpSystem() bool {
	// Check if root command has proper help setup
	rootFiles := []string{
		"src/cmd/cryptorun/root.go",
		"src/cmd/cryptorun/main.go",
		"cmd/cryptorun/root.go",
		"cmd/cryptorun/main.go",
	}

	for _, rootFile := range rootFiles {
		if content, err := os.ReadFile(rootFile); err == nil {
			rootContent := string(content)

			// Look for cobra setup patterns
			hasCobraRoot := strings.Contains(rootContent, "cobra.Command") &&
				strings.Contains(rootContent, "Use:")

			hasHelpSetup := strings.Contains(rootContent, "Short:") ||
				strings.Contains(rootContent, "Long:")

			if hasCobraRoot && hasHelpSetup {
				return true
			}
		}
	}

	return false
}
