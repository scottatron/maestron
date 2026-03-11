package doctor

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"

	"github.com/scottatron/maestron/internal/manage"
)

// Issue describes a problem found during health checks.
type Issue struct {
	Severity string // "error", "warning", "info"
	Message  string
	Fix      string // optional: suggested fix command
}

// Check runs all health checks for the project at projectRoot and returns any issues found.
func Check(projectRoot string) ([]Issue, error) {
	var issues []Issue

	// 1. agents.json exists and is valid JSON
	agentsPath := filepath.Join(projectRoot, ".agents", "agents.json")
	data, err := os.ReadFile(agentsPath)
	if err != nil {
		issues = append(issues, Issue{
			Severity: "error",
			Message:  fmt.Sprintf(".agents/agents.json not found: %v", err),
			Fix:      "Create .agents/agents.json with schemaVersion 3",
		})
		return issues, nil
	}

	var rawCfg map[string]interface{}
	if err := json.Unmarshal(data, &rawCfg); err != nil {
		issues = append(issues, Issue{
			Severity: "error",
			Message:  fmt.Sprintf(".agents/agents.json is not valid JSON: %v", err),
		})
		return issues, nil
	}

	// 2. schemaVersion == 3
	cfg, err := manage.ReadAgentsConfig(projectRoot)
	if err != nil {
		issues = append(issues, Issue{
			Severity: "error",
			Message:  fmt.Sprintf("failed to parse agents.json: %v", err),
		})
		return issues, nil
	}

	if cfg.SchemaVersion != 3 {
		issues = append(issues, Issue{
			Severity: "error",
			Message:  fmt.Sprintf("schemaVersion is %d, expected 3", cfg.SchemaVersion),
			Fix:      `Set "schemaVersion": 3 in agents.json`,
		})
	}

	// 3. AGENTS.md exists
	agentsMdPath := cfg.Instructions.Path
	if agentsMdPath == "" {
		agentsMdPath = "AGENTS.md"
	}
	if !filepath.IsAbs(agentsMdPath) {
		agentsMdPath = filepath.Join(projectRoot, agentsMdPath)
	}
	if _, err := os.Stat(agentsMdPath); err != nil {
		issues = append(issues, Issue{
			Severity: "warning",
			Message:  fmt.Sprintf("instructions file %q not found", cfg.Instructions.Path),
			Fix:      fmt.Sprintf("Create %s with agent instructions", cfg.Instructions.Path),
		})
	}

	// 4. .agents/local.json — if exists, is valid JSON
	localPath := filepath.Join(projectRoot, ".agents", "local.json")
	if localData, err := os.ReadFile(localPath); err == nil {
		var localRaw map[string]interface{}
		if err := json.Unmarshal(localData, &localRaw); err != nil {
			issues = append(issues, Issue{
				Severity: "error",
				Message:  fmt.Sprintf(".agents/local.json is not valid JSON: %v", err),
			})
		}
	}

	// 5. MCP server env/headers have no literal secrets
	secretPatterns := []*regexp.Regexp{
		regexp.MustCompile(`sk-[A-Za-z0-9]{20,}`),
		regexp.MustCompile(`ghp_[A-Za-z0-9]{36}`),
		regexp.MustCompile(`xoxb-[A-Za-z0-9-]+`),
		regexp.MustCompile(`AKIA[A-Z0-9]{16}`),
		regexp.MustCompile(`ya29\.[A-Za-z0-9_-]+`),
	}
	for name, def := range cfg.MCP.Servers {
		for k, v := range def.Env {
			for _, pat := range secretPatterns {
				if pat.MatchString(v) {
					issues = append(issues, Issue{
						Severity: "error",
						Message:  fmt.Sprintf("MCP server %q env %q appears to contain a literal secret", name, k),
						Fix:      fmt.Sprintf(`Use ${%s} placeholder and set the variable in your shell environment`, k),
					})
					break
				}
			}
		}
		for k, v := range def.Headers {
			for _, pat := range secretPatterns {
				if pat.MatchString(v) {
					issues = append(issues, Issue{
						Severity: "error",
						Message:  fmt.Sprintf("MCP server %q header %q appears to contain a literal secret", name, k),
						Fix:      fmt.Sprintf(`Use ${%s} placeholder and set the variable in your shell environment`, k),
					})
					break
				}
			}
		}
	}

	// 6. Env values using ${VAR} — check env var is set
	varPattern := regexp.MustCompile(`\$\{([^}]+)\}`)
	for name, def := range cfg.MCP.Servers {
		allVals := []string{}
		for _, v := range def.Env {
			allVals = append(allVals, v)
		}
		for _, a := range def.Args {
			allVals = append(allVals, a)
		}
		for _, v := range allVals {
			matches := varPattern.FindAllStringSubmatch(v, -1)
			for _, m := range matches {
				varName := m[1]
				if varName == "PROJECT_ROOT" {
					continue
				}
				if os.Getenv(varName) == "" {
					issues = append(issues, Issue{
						Severity: "warning",
						Message:  fmt.Sprintf("MCP server %q references ${%s} but the variable is not set", name, varName),
						Fix:      fmt.Sprintf("export %s=<value>", varName),
					})
				}
			}
		}
	}

	// 7. .agents/generated/ is not git-tracked
	generatedDir := filepath.Join(".agents", "generated")
	if isGitTracked(projectRoot, generatedDir) {
		issues = append(issues, Issue{
			Severity: "warning",
			Message:  ".agents/generated/ is tracked by git — it should be gitignored",
			Fix:      "git rm -r --cached .agents/generated && echo '.agents/generated' >> .gitignore",
		})
	}

	// 8. .agents/local.json is not git-tracked
	if isGitTracked(projectRoot, ".agents/local.json") {
		issues = append(issues, Issue{
			Severity: "warning",
			Message:  ".agents/local.json is tracked by git — it may contain machine-specific secrets",
			Fix:      "git rm --cached .agents/local.json && echo '.agents/local.json' >> .gitignore",
		})
	}

	// 9. Each enabled integration binary exists on PATH
	integrationBinaries := map[string]string{
		"claude":         "claude",
		"codex":          "codex",
		"gemini":         "gemini",
		"copilot":        "copilot",
		"copilot_vscode": "code",
		"cursor":         "cursor",
		"windsurf":       "windsurf",
		"antigravity":    "antigravity",
		"opencode":       "opencode",
	}
	for _, integ := range cfg.Integrations.Enabled {
		binary, ok := integrationBinaries[integ]
		if !ok {
			binary = integ
		}
		if _, err := exec.LookPath(binary); err != nil {
			issues = append(issues, Issue{
				Severity: "info",
				Message:  fmt.Sprintf("Integration %q is enabled but binary %q not found on PATH", integ, binary),
				Fix:      fmt.Sprintf("Install %s or remove it from integrations.enabled", binary),
			})
		}
	}

	return issues, nil
}

// isGitTracked returns true if the given path (relative to projectRoot) is tracked by git.
func isGitTracked(projectRoot, relPath string) bool {
	cmd := exec.Command("git", "ls-files", "--error-unmatch", relPath)
	cmd.Dir = projectRoot
	cmd.Stderr = io.Discard
	err := cmd.Run()
	return err == nil
}
