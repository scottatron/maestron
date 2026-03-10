package agents

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// AgentsConfig represents the schema v3 .agents/agents.json structure.
type AgentsConfig struct {
	SchemaVersion int `json:"schemaVersion"`
	MCP           struct {
		Servers map[string]MCPServerDef `json:"servers"`
	} `json:"mcp"`
	Integrations struct {
		Enabled []string `json:"enabled"`
	} `json:"integrations"`
	SyncMode string `json:"syncMode"`
}

// MCPServerDef is an MCP server definition from agents.json.
type MCPServerDef struct {
	Label       string            `json:"label"`
	Description string            `json:"description"`
	Transport   string            `json:"transport"`
	Command     string            `json:"command"`
	Args        []string          `json:"args"`
	Env         map[string]string `json:"env,omitempty"`
	Targets     []string          `json:"targets"`
	Enabled     bool              `json:"enabled"`
}

// FindAgentsConfig searches for the squad root and parses agents.json.
// Returns ("", nil, nil) if not found — callers handle gracefully.
func FindAgentsConfig() (root string, cfg *AgentsConfig, err error) {
	// 1. $SQUAD_ROOT env var
	if squadRoot := os.Getenv("SQUAD_ROOT"); squadRoot != "" {
		if cfg, err := tryLoadAgentsConfig(squadRoot); cfg != nil || err != nil {
			return squadRoot, cfg, err
		}
	}

	// 2. Walk up from cwd looking for .agents/agents.json
	cwd, err := os.Getwd()
	if err == nil {
		dir := cwd
		for {
			if cfg, err := tryLoadAgentsConfig(dir); cfg != nil || err != nil {
				return dir, cfg, err
			}
			parent := filepath.Dir(dir)
			if parent == dir {
				break
			}
			dir = parent
		}
	}

	// 3. Fallback to ~/src/github.com/scottatron/squad
	home, err := os.UserHomeDir()
	if err != nil {
		return "", nil, err
	}
	fallback := filepath.Join(home, "src", "github.com", "scottatron", "squad")
	if cfg, err := tryLoadAgentsConfig(fallback); cfg != nil || err != nil {
		return fallback, cfg, err
	}

	return "", nil, nil
}

func tryLoadAgentsConfig(root string) (*AgentsConfig, error) {
	path := filepath.Join(root, ".agents", "agents.json")
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var cfg AgentsConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}
