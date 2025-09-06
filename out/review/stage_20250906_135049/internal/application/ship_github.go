package application

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	"github.com/rs/zerolog/log"
)

// GitHubPRManager handles GitHub PR creation and management
type GitHubPRManager struct {
	token      string
	repo       string
	client     *http.Client
	gistEnabled bool
}

// PRCreationRequest contains PR creation parameters
type PRCreationRequest struct {
	Title       string
	Description string
	Results     *ShipResults
	DryRun      bool
}

// PRCreationResponse contains the result of PR creation
type PRCreationResponse struct {
	Success     bool
	PRURL       string
	PRNumber    int
	Message     string
	AttachmentLinks []AttachmentLink
	BlockersFile    string
}

// GitHubPR represents a GitHub pull request
type GitHubPR struct {
	Title string `json:"title"`
	Body  string `json:"body"`
	Head  string `json:"head"`
	Base  string `json:"base"`
}

// NewGitHubPRManager creates a new GitHub PR manager
func NewGitHubPRManager(repo string) *GitHubPRManager {
	token := os.Getenv("GITHUB_TOKEN")
	
	return &GitHubPRManager{
		token:  token,
		repo:   repo,
		client: &http.Client{Timeout: 30 * time.Second},
		gistEnabled: token != "", // Enable gist uploads if token available
	}
}

// HasCredentials checks if GitHub credentials are available
func (gm *GitHubPRManager) HasCredentials() bool {
	return gm.token != ""
}

// CreatePullRequest creates a GitHub PR with comprehensive shipping data
func (gm *GitHubPRManager) CreatePullRequest(req *PRCreationRequest) (*PRCreationResponse, error) {
	response := &PRCreationResponse{}

	// Generate PR body
	prBody, err := gm.generateEnhancedPRBody(req)
	if err != nil {
		return nil, fmt.Errorf("failed to generate PR body: %w", err)
	}

	// Create blockers file if quality gates failed
	if !req.Results.QualityGatesPassed {
		blockersPath, err := gm.generateBlockersFile(req.Results)
		if err != nil {
			log.Warn().Err(err).Msg("Failed to generate blockers file")
		} else {
			response.BlockersFile = blockersPath
		}
	}

	// Handle dry run
	if req.DryRun {
		return gm.handleDryRun(prBody, response)
	}

	// Check credentials
	if !gm.HasCredentials() {
		log.Info().Msg("No GitHub credentials - falling back to dry run")
		return gm.handleDryRun(prBody, response)
	}

	// Upload attachments to gists
	if gm.gistEnabled {
		attachmentLinks, err := gm.createAttachmentGists(req.Results)
		if err != nil {
			log.Warn().Err(err).Msg("Failed to create attachment gists")
		} else {
			response.AttachmentLinks = attachmentLinks
		}
	}

	// Create actual PR
	prURL, prNumber, err := gm.createGitHubPR(req.Title, prBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create GitHub PR: %w", err)
	}

	response.Success = true
	response.PRURL = prURL
	response.PRNumber = prNumber
	response.Message = fmt.Sprintf("✅ Pull request #%d created: %s", prNumber, prURL)

	// Apply release label if permitted
	if req.Results.QualityGatesPassed {
		if err := gm.applyReleaseLabel(prNumber); err != nil {
			log.Warn().Err(err).Int("pr", prNumber).Msg("Failed to apply release label")
		}
	}

	// Update CHANGELOG with ship status
	if err := gm.updateChangelogShipStatus(req.Results, "OPENED"); err != nil {
		log.Warn().Err(err).Msg("Failed to update CHANGELOG with ship status")
	}

	return response, nil
}

// generateEnhancedPRBody creates comprehensive PR body with all required elements
func (gm *GitHubPRManager) generateEnhancedPRBody(req *PRCreationRequest) (string, error) {
	// Load template
	templateData, err := os.ReadFile("templates/PR_BODY.md.tmpl")
	if err != nil {
		return "", fmt.Errorf("failed to read PR template: %w", err)
	}

	tmpl, err := template.New("pr_body").Parse(string(templateData))
	if err != nil {
		return "", fmt.Errorf("failed to parse PR template: %w", err)
	}

	// Prepare template data
	data := struct {
		Title                string
		Description          string
		LatestDryrun        string
		Results             PerformanceKPIs
		MonitorSnapshot     string
		Artifacts           []ArtifactCheck
		CoveragePolicy      string
		AttachmentLinks     []AttachmentLink
		QualityGatesPassed  bool
		BlockersLink        string
		ReleaseLabel        string
		Branch              string
		CommitSHA           string
		ShipStatus          string
	}{
		Title:               req.Title,
		Description:         req.Description,
		LatestDryrun:        req.Results.LatestDryrun,
		Results:             req.Results.PerformanceMetrics,
		MonitorSnapshot:     gm.formatMonitorSnapshot(req.Results.MonitorSnapshot),
		Artifacts:           req.Results.ArtifactChecks,
		CoveragePolicy:      req.Results.CoveragePolicy,
		AttachmentLinks:     []AttachmentLink{}, // Will be populated later
		QualityGatesPassed:  req.Results.QualityGatesPassed,
		BlockersLink:        "out/ship/PR_BLOCKERS.md",
		ReleaseLabel:        gm.determineReleaseLabel(req.Results),
		Branch:              req.Results.Branch,
		CommitSHA:           req.Results.CommitSHA,
		ShipStatus:          "PREPARED",
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("failed to execute PR template: %w", err)
	}

	return buf.String(), nil
}

// formatMonitorSnapshot formats monitor health snapshot for PR body
func (gm *GitHubPRManager) formatMonitorSnapshot(snapshot *MetricsSnapshot) string {
	if snapshot == nil {
		return "❌ Monitor /metrics endpoint unreachable"
	}

	return snapshot.FormatSummary()
}

// determineReleaseLabel determines appropriate release label
func (gm *GitHubPRManager) determineReleaseLabel(results *ShipResults) string {
	if !results.QualityGatesPassed {
		return ""
	}

	// Check if this is a dryrun release
	if strings.Contains(strings.ToLower(results.LatestDryrun), "dryrun") ||
	   strings.Contains(strings.ToLower(results.LatestDryrun), "dry-run") {
		return "release:dryrun"
	}

	return "release:candidate"
}

// generateBlockersFile creates PR_BLOCKERS.md with detailed fix instructions
func (gm *GitHubPRManager) generateBlockersFile(results *ShipResults) (string, error) {
	shipDir := "out/ship"
	if err := os.MkdirAll(shipDir, 0755); err != nil {
		return "", err
	}

	var blockers strings.Builder
	blockers.WriteString("# PR Shipping Blockers\n\n")
	blockers.WriteString(fmt.Sprintf("Generated: %s\n\n", time.Now().Format(time.RFC3339)))
	blockers.WriteString("The following issues must be resolved before shipping:\n\n")

	for _, violation := range results.PolicyViolations {
		if violation.Blocker {
			blockers.WriteString(fmt.Sprintf("## ❌ %s\n\n", violation.Description))
			blockers.WriteString(fmt.Sprintf("**Category:** %s\n", violation.Category))
			
			if violation.Current != nil && violation.Required != nil {
				blockers.WriteString(fmt.Sprintf("**Current:** %v\n", violation.Current))
				blockers.WriteString(fmt.Sprintf("**Required:** %v\n", violation.Required))
			}
			
			blockers.WriteString("\n**Next Actions:**\n")
			blockers.WriteString(gm.generateFixInstructions(violation))
			blockers.WriteString("\n")
		}
	}

	if len(results.PolicyViolations) == 0 {
		blockers.WriteString("✅ No blocking violations found.\n")
	}

	blockersPath := filepath.Join(shipDir, "PR_BLOCKERS.md")
	if err := os.WriteFile(blockersPath, []byte(blockers.String()), 0644); err != nil {
		return "", err
	}

	return blockersPath, nil
}

// generateFixInstructions provides specific fix instructions for violations
func (gm *GitHubPRManager) generateFixInstructions(violation PolicyViolation) string {
	switch violation.Category {
	case "performance":
		if strings.Contains(violation.Description, "precision") {
			return `1. Review scanning parameters and gate thresholds
2. Run additional scan cycles to gather more data
3. Consider adjusting factor weights or orthogonalization
4. Validate signal quality in out/analyst/coverage.json
5. Re-run digest after improvements: ./cryptorun digest`
		}
		
	case "operational":
		if strings.Contains(violation.Description, "endpoint unreachable") {
			return `1. Start monitor server: ./cryptorun monitor
2. Verify port 8080 is accessible
3. Check for firewall or network issues
4. Ensure monitor service is healthy`
		}
		
	case "artifacts":
		if strings.Contains(violation.Description, "missing") {
			return `1. Run complete scan pipeline: ./cryptorun scan
2. Generate analyst coverage: ./cryptorun analyst-coverage
3. Create digest: ./cryptorun digest --date <today>
4. Verify all artifacts in out/ directory structure`
		}
		
	case "coverage":
		return `1. Run analyst coverage analysis: ./cryptorun analyst-coverage
2. Review missed opportunities in coverage report
3. Adjust scoring thresholds if needed
4. Ensure sufficient trading universe coverage`
	}
	
	return "Review the specific issue and consult documentation for resolution steps."
}

// createAttachmentGists uploads artifacts to GitHub gists
func (gm *GitHubPRManager) createAttachmentGists(results *ShipResults) ([]AttachmentLink, error) {
	var links []AttachmentLink
	
	attachmentPaths := []string{
		"out/results/results_report.md",
		"out/digest/latest/digest.md",
		"out/analyst/coverage.json", 
		"out/scanner/latest_candidates.jsonl",
	}

	for _, path := range attachmentPaths {
		if gistURL, err := gm.uploadToGist(path); err == nil {
			links = append(links, AttachmentLink{
				Name: filepath.Base(path),
				URL:  gistURL,
				Path: path,
			})
		} else {
			log.Warn().Err(err).Str("path", path).Msg("Failed to upload to gist")
		}
	}

	return links, nil
}

// uploadToGist uploads a file to GitHub gist
func (gm *GitHubPRManager) uploadToGist(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}

	filename := filepath.Base(path)
	gistData := map[string]interface{}{
		"description": fmt.Sprintf("CryptoRun Ship Artifact: %s", filename),
		"public":      false,
		"files": map[string]interface{}{
			filename: map[string]interface{}{
				"content": string(data),
			},
		},
	}

	jsonData, err := json.Marshal(gistData)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequest("POST", "https://api.github.com/gists", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", err
	}

	req.Header.Set("Authorization", "Bearer "+gm.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := gm.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		return "", fmt.Errorf("gist creation failed: %s", resp.Status)
	}

	var gistResp map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&gistResp); err != nil {
		return "", err
	}

	if htmlURL, ok := gistResp["html_url"].(string); ok {
		return htmlURL, nil
	}

	return "", fmt.Errorf("no HTML URL in gist response")
}

// createGitHubPR creates the actual GitHub pull request
func (gm *GitHubPRManager) createGitHubPR(title, body string) (string, int, error) {
	// Get current branch
	branch, err := gm.getCurrentBranch()
	if err != nil {
		return "", 0, fmt.Errorf("failed to get current branch: %w", err)
	}

	// Create PR data
	prData := GitHubPR{
		Title: title,
		Body:  body,
		Head:  branch,
		Base:  "main", // assuming main branch
	}

	jsonData, err := json.Marshal(prData)
	if err != nil {
		return "", 0, err
	}

	// Create PR via API
	url := fmt.Sprintf("https://api.github.com/repos/%s/pulls", gm.repo)
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", 0, err
	}

	req.Header.Set("Authorization", "Bearer "+gm.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := gm.client.Do(req)
	if err != nil {
		return "", 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return "", 0, fmt.Errorf("PR creation failed: %s - %s", resp.Status, string(body))
	}

	var prResp map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&prResp); err != nil {
		return "", 0, err
	}

	prURL := prResp["html_url"].(string)
	prNumber := int(prResp["number"].(float64))

	return prURL, prNumber, nil
}

// getCurrentBranch gets the current git branch
func (gm *GitHubPRManager) getCurrentBranch() (string, error) {
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	
	return strings.TrimSpace(string(output)), nil
}

// applyReleaseLabel applies appropriate release label to PR
func (gm *GitHubPRManager) applyReleaseLabel(prNumber int) error {
	url := fmt.Sprintf("https://api.github.com/repos/%s/issues/%d/labels", gm.repo, prNumber)
	
	labelData := []string{"release:dryrun"}
	jsonData, err := json.Marshal(labelData)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}

	req.Header.Set("Authorization", "Bearer "+gm.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := gm.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("label application failed: %s", resp.Status)
	}

	return nil
}

// updateChangelogShipStatus appends ship status to CHANGELOG
func (gm *GitHubPRManager) updateChangelogShipStatus(results *ShipResults, status string) error {
	changelogPath := "CHANGELOG.md"
	
	// Read current changelog
	data, err := os.ReadFile(changelogPath)
	if err != nil {
		return err
	}

	// Append ship status entry
	shipEntry := fmt.Sprintf("\n## Ship Status: %s\n", time.Now().Format("2006-01-02 15:04:05"))
	shipEntry += fmt.Sprintf("**SHIP+:** branch=%s sha=%s status=%s\n", 
		results.Branch, results.CommitSHA, status)
	shipEntry += fmt.Sprintf("**Quality Gates:** %v\n", map[bool]string{true: "PASS", false: "FAIL"}[results.QualityGatesPassed])
	
	if len(results.PolicyViolations) > 0 {
		shipEntry += "**Policy Violations:**\n"
		for _, violation := range results.PolicyViolations {
			shipEntry += fmt.Sprintf("- %s: %s\n", violation.Category, violation.Description)
		}
	}

	updatedContent := string(data) + shipEntry

	return os.WriteFile(changelogPath, []byte(updatedContent), 0644)
}

// handleDryRun handles dry run mode by writing PR body to file
func (gm *GitHubPRManager) handleDryRun(prBody string, response *PRCreationResponse) (*PRCreationResponse, error) {
	shipDir := "out/ship"
	if err := os.MkdirAll(shipDir, 0755); err != nil {
		return nil, err
	}

	prBodyPath := filepath.Join(shipDir, "PR_BODY.md")
	if err := os.WriteFile(prBodyPath, []byte(prBody), 0644); err != nil {
		return nil, err
	}

	response.Success = true
	response.Message = fmt.Sprintf("✅ Dry run completed - PR body written to %s", prBodyPath)
	
	// Also write a one-liner for manual PR creation
	oneLinePath := filepath.Join(shipDir, "MANUAL_PR_COMMAND.txt")
	oneLiner := fmt.Sprintf("gh pr create --title \"[MANUAL]\" --body-file \"%s\"", prBodyPath)
	os.WriteFile(oneLinePath, []byte(oneLiner), 0644)

	response.Message += fmt.Sprintf("\nManual PR command: %s", oneLiner)

	return response, nil
}