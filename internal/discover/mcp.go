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

// ListMCPServers returns all configured MCP servers from all known sources.
// Priority: agents.json > ~/.claude/settings.json > ~/.copilot/mcp-config.json > ~/.codex/config.toml
func ListMCPServers() ([]MCPServerInfo, error) {
	seen := map[string]bool{}
	var result []MCPServerInfo

	// 1. agents.json (highest priority)
	agentsRoot, cfg, err := agents.FindAgentsConfig()
	if err == nil && cfg != nil {
		agentsConfigPath := filepath.Join(agentsRoot, ".agents", "agents.json")
		for name, def := range cfg.MCP.Servers {
			seen[name] = true
			result = append(result, MCPServerInfo{
				Name:       name,
				Label:      def.Label,
				Command:    def.Command,
				Args:       def.Args,
				Env:        def.Env,
				Transport:  def.Transport,
				Targets:    def.Targets,
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
					if seen[name] {
						continue
					}
					seen[name] = true
					result = append(result, MCPServerInfo{
						Name:       name,
						Command:    srv.Command,
						Args:       srv.Args,
						Env:        srv.Env,
						Transport:  "stdio",
						Enabled:    true,
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
					if seen[name] {
						continue
					}
					seen[name] = true
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
						Targets:    []string{"copilot"},
						Enabled:    true,
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
					if seen[name] {
						continue
					}
					seen[name] = true
					result = append(result, MCPServerInfo{
						Name:       name,
						Command:    srv.Command,
						Args:       srv.Args,
						Transport:  "stdio",
						Targets:    []string{"codex"},
						Enabled:    true,
						Source:     "codex-config",
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
					if seen[name] {
						continue
					}
					seen[name] = true
					result = append(result, MCPServerInfo{
						Name:       name,
						Command:    def.Command,
						Args:       def.Args,
						Env:        def.Env,
						URL:        def.URL,
						Transport:  def.Transport,
						Targets:    def.Targets,
						Enabled:    def.Enabled,
						Source:     "agents-global",
						ConfigPath: globalPath,
					})
				}
			}
		}
	}

	return result, nil
}
