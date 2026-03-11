package agents

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// AgentsConfig represents the schema v3 .agents/agents.json structure.
type AgentsConfig struct {
	SchemaVersion int `json:"schemaVersion"`
	Instructions  struct {
		Path string `json:"path,omitempty"`
	} `json:"instructions,omitempty"`
	MCP struct {
		Servers map[string]MCPServerDef `json:"servers"`
	} `json:"mcp"`
	Integrations struct {
		Enabled []string               `json:"enabled"`
		Options map[string]interface{} `json:"options,omitempty"`
	} `json:"integrations"`
	SyncMode  string          `json:"syncMode,omitempty"`
	Workspace json.RawMessage `json:"workspace,omitempty"`
	LastSync  string          `json:"lastSync,omitempty"`
}

// MCPServerDef is an MCP server definition from agents.json.
type MCPServerDef struct {
	Label       string            `json:"label,omitempty"`
	Description string            `json:"description,omitempty"`
	Transport   string            `json:"transport,omitempty"`
	Command     string            `json:"command,omitempty"`
	Args        []string          `json:"args,omitempty"`
	Env         map[string]string `json:"env,omitempty"`
	Headers     map[string]string `json:"headers,omitempty"`
	URL         string            `json:"url,omitempty"`
	Enabled     *bool             `json:"enabled,omitempty"`
}

// GlobalMcpConfig represents the ~/.agents/global.json structure.
type GlobalMcpConfig struct {
	MCPServers map[string]MCPServerDef `json:"mcpServers"`
}

// FindAgentsConfig searches for the project root and parses agents.json.
// Returns ("", nil, nil) if not found — callers handle gracefully.
func FindAgentsConfig() (root string, cfg *AgentsConfig, err error) {
	// 1. $AGENTS_ROOT env var
	if agentsRoot := os.Getenv("AGENTS_ROOT"); agentsRoot != "" {
		if cfg, err := tryLoadAgentsConfig(agentsRoot); cfg != nil || err != nil {
			return agentsRoot, cfg, err
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

	return "", nil, nil
}

// BoolPtr returns a pointer to b.
func BoolPtr(b bool) *bool { return &b }

// IsEnabled returns true if b is nil (unset, defaults to enabled) or points to true.
func IsEnabled(b *bool) bool { return b == nil || *b }

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
