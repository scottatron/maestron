package discover

import (
	"encoding/json"
	"os"

	"github.com/scottatron/maestron/internal/agents"
	"github.com/scottatron/maestron/internal/platform"
)

// claudeSettings represents the relevant parts of ~/.claude/settings.json
type claudeSettings struct {
	MCPServers map[string]claudeMCPServer `json:"mcpServers"`
}

type claudeMCPServer struct {
	Command string            `json:"command"`
	Args    []string          `json:"args"`
	Env     map[string]string `json:"env,omitempty"`
}

// ListMCPServers returns all configured MCP servers from agents.json and Claude settings.
func ListMCPServers() ([]MCPServerInfo, error) {
	seen := map[string]bool{}
	var result []MCPServerInfo

	// 1. agents.json (takes priority)
	_, cfg, err := agents.FindAgentsConfig()
	if err == nil && cfg != nil {
		for name, def := range cfg.MCP.Servers {
			seen[name] = true
			result = append(result, MCPServerInfo{
				Name:      name,
				Label:     def.Label,
				Command:   def.Command,
				Args:      def.Args,
				Env:       def.Env,
				Transport: def.Transport,
				Targets:   def.Targets,
				Enabled:   def.Enabled,
				Source:    "agents.json",
			})
		}
	}

	// 2. ~/.claude/settings.json
	settingsFile, err := platform.ClaudeSettingsFile()
	if err == nil {
		if data, err := os.ReadFile(settingsFile); err == nil {
			var cs claudeSettings
			if json.Unmarshal(data, &cs) == nil {
				for name, srv := range cs.MCPServers {
					if seen[name] {
						continue
					}
					result = append(result, MCPServerInfo{
						Name:      name,
						Command:   srv.Command,
						Args:      srv.Args,
						Env:       srv.Env,
						Transport: "stdio",
						Enabled:   true,
						Source:    "claude-settings",
					})
				}
			}
		}
	}

	return result, nil
}
