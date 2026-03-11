package discover

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"

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

// copilotMCPConfig represents ~/.copilot/mcp-config.json
type copilotMCPConfig struct {
	MCPServers map[string]copilotMCPServer `json:"mcpServers"`
}

type copilotMCPServer struct {
	Type    string            `json:"type"` // "local" or "http"
	Command string            `json:"command"`
	Args    []string          `json:"args"`
	Env     map[string]string `json:"env,omitempty"`
	URL     string            `json:"url,omitempty"`
	Headers map[string]string `json:"headers,omitempty"`
	Tools   []string          `json:"tools,omitempty"`
}

// codexConfig represents the MCP-relevant parts of ~/.codex/config.toml
type codexConfig struct {
	MCPServers map[string]codexMCPServer `toml:"mcp_servers"`
}

type codexMCPServer struct {
	Command string   `toml:"command"`
	Args    []string `toml:"args"`
}

// sourceOrder defines the priority of sources (index = priority, lower = higher priority).
var sourceOrder = []string{
	"agents.json",
	"claude-settings",
	"copilot-mcp-config",
	"codex-config",
	"agents-global",
}

// SourcePriority returns the priority index for a source (lower = higher priority).
func SourcePriority(source string) int {
	for i, s := range sourceOrder {
		if s == source {
			return i
		}
	}
	return len(sourceOrder)
}

// ListMCPServers returns all configured MCP servers from all known sources.
// All entries are returned — no deduplication. When the same server name appears
// in multiple sources, the lower-priority entries have Shadowed set to true.
func ListMCPServers() ([]MCPServerInfo, error) {
	var result []MCPServerInfo

	// 1. agents.json (highest priority)
	agentsRoot, cfg, err := agents.FindAgentsConfig()
	if err == nil && cfg != nil {
		agentsConfigPath := filepath.Join(agentsRoot, ".agents", "agents.json")
		for name, def := range cfg.MCP.Servers {
			result = append(result, MCPServerInfo{
				Name:       name,
				Label:      def.Label,
				Command:    def.Command,
				Args:       def.Args,
				Env:        def.Env,
				Transport:  def.Transport,
				Enabled:    def.Enabled,
				Source:     "agents.json",
				ConfigPath: agentsConfigPath,
			})
		}
	}

	// 2. ~/.claude/settings.json
	if settingsFile, err := platform.ClaudeSettingsFile(); err == nil {
		if data, err := os.ReadFile(settingsFile); err == nil {
			var cs claudeSettings
			if json.Unmarshal(data, &cs) == nil {
				for name, srv := range cs.MCPServers {
					result = append(result, MCPServerInfo{
						Name:       name,
						Command:    srv.Command,
						Args:       srv.Args,
						Env:        srv.Env,
						Transport:  "stdio",
						Enabled:    agents.BoolPtr(true),
						Source:     "claude-settings",
						ConfigPath: settingsFile,
					})
				}
			}
		}
	}

	// 3. ~/.copilot/mcp-config.json
	if home, err := platform.HomeDir(); err == nil {
		copilotPath := filepath.Join(home, ".copilot", "mcp-config.json")
		if data, err := os.ReadFile(copilotPath); err == nil {
			var cc copilotMCPConfig
			if json.Unmarshal(data, &cc) == nil {
				for name, srv := range cc.MCPServers {
					transport := "stdio"
					if srv.Type == "http" {
						transport = "http"
					}
					result = append(result, MCPServerInfo{
						Name:       name,
						Command:    srv.Command,
						Args:       srv.Args,
						Env:        srv.Env,
						URL:        srv.URL,
						Transport:  transport,
						Enabled:    agents.BoolPtr(true),
						Source:     "copilot-mcp-config",
						ConfigPath: copilotPath,
					})
				}
			}
		}
	}

	// 4. ~/.codex/config.toml
	if home, err := platform.HomeDir(); err == nil {
		codexPath := filepath.Join(home, ".codex", "config.toml")
		if data, err := os.ReadFile(codexPath); err == nil {
			var dc codexConfig
			if toml.Unmarshal(data, &dc) == nil {
				for name, srv := range dc.MCPServers {
					result = append(result, MCPServerInfo{
						Name:      name,
						Command:   srv.Command,
						Args:      srv.Args,
						Transport: "stdio",
						Enabled:   agents.BoolPtr(true),
						Source:    "codex-config",
						ConfigPath: codexPath,
					})
				}
			}
		}
	}

	// 5. ~/.agents/global.json (lowest priority)
	if home, err := platform.HomeDir(); err == nil {
		globalPath := filepath.Join(home, ".agents", "global.json")
		if data, err := os.ReadFile(globalPath); err == nil {
			var gc agents.GlobalMcpConfig
			if json.Unmarshal(data, &gc) == nil {
				for name, def := range gc.MCPServers {
					result = append(result, MCPServerInfo{
						Name:       name,
						Command:    def.Command,
						Args:       def.Args,
						Env:        def.Env,
						URL:        def.URL,
						Transport:  def.Transport,
						Enabled:    def.Enabled,
						Source:     "agents-global",
						ConfigPath: globalPath,
					})
				}
			}
		}
	}

	// Mark shadowed entries: for each server name, entries from lower-priority
	// sources are shadowed by entries from higher-priority sources.
	markShadowed(result)

	return result, nil
}

// markShadowed sets Shadowed=true on entries that are overridden by a
// higher-priority source with the same server name.
func markShadowed(servers []MCPServerInfo) {
	// Find the highest priority source for each name.
	best := make(map[string]int) // name → best priority seen so far
	for i := range servers {
		p := SourcePriority(servers[i].Source)
		if existing, ok := best[servers[i].Name]; !ok || p < existing {
			best[servers[i].Name] = p
		}
	}
	// Mark entries that are not from the best source as shadowed.
	for i := range servers {
		if SourcePriority(servers[i].Source) > best[servers[i].Name] {
			servers[i].Shadowed = true
		}
	}
}
