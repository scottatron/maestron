package discover

import (
	"encoding/json"
	"fmt"
	"os/exec"

	"github.com/scottatron/maestron/internal/agents"
)

type miseEntry struct {
	Version          string `json:"version"`
	RequestedVersion string `json:"requested_version"`
	InstallPath      string `json:"install_path"`
	Source           struct {
		Type string `json:"type"`
		Path string `json:"path"`
	} `json:"source"`
	Installed bool `json:"installed"`
	Active    bool `json:"active"`
}

// ListAgents returns installed AI coding agents detected via `mise ls --json`.
// If allTools is true, all mise tools are returned (not just known agents).
// If mise is not on PATH, returns a graceful error.
func ListAgents(allTools bool) ([]AgentInfo, error) {
	cmd := exec.Command("mise", "ls", "--json")
	out, err := cmd.Output()
	if err != nil {
		if execErr, ok := err.(*exec.Error); ok && execErr.Err == exec.ErrNotFound {
			return nil, fmt.Errorf("mise not found on PATH; install mise to enable agent detection")
		}
		return nil, fmt.Errorf("running mise ls --json: %w", err)
	}

	var raw map[string][]miseEntry
	if err := json.Unmarshal(out, &raw); err != nil {
		return nil, fmt.Errorf("parsing mise ls output: %w", err)
	}

	// Deduplicate by logical agent name (keep the active/most recent entry)
	seen := map[string]bool{}
	var result []AgentInfo

	for toolName, entries := range raw {
		ka, isAgent := agents.LookupAgent(toolName)
		if !allTools && !isAgent {
			continue
		}

		// Prefer active entries; fall back to first installed
		var best *miseEntry
		for i := range entries {
			e := &entries[i]
			if !e.Installed {
				continue
			}
			if best == nil || (e.Active && !best.Active) {
				best = e
			}
		}
		if best == nil {
			continue
		}

		name := toolName
		displayName := toolName
		if isAgent {
			// Deduplicate by logical name
			if seen[ka.Name] {
				continue
			}
			seen[ka.Name] = true
			name = ka.Name
			displayName = ka.DisplayName
		}

		result = append(result, AgentInfo{
			Name:             name,
			DisplayName:      displayName,
			Version:          best.Version,
			RequestedVersion: best.RequestedVersion,
			Source:           best.Source.Path,
			InstallPath:      best.InstallPath,
			Active:           best.Active,
			MiseToolName:     toolName,
		})
	}

	return result, nil
}
