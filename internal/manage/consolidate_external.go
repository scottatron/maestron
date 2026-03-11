package manage

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
	"github.com/scottatron/maestron/internal/agents"
	"github.com/scottatron/maestron/internal/platform"
)

// ExternalSource represents a non-maestron MCP config source that can be consolidated.
type ExternalSource struct {
	Label      string // source label as used in MCPServerInfo.Source
	ConfigPath string // absolute path to the config file
}

// ListExternalSources returns the external MCP config files that exist on this machine.
func ListExternalSources() ([]ExternalSource, error) {
	var sources []ExternalSource

	if settingsFile, err := platform.ClaudeSettingsFile(); err == nil {
		if _, err := os.Stat(settingsFile); err == nil {
			sources = append(sources, ExternalSource{Label: "claude-settings", ConfigPath: settingsFile})
		}
	}

	if home, err := platform.HomeDir(); err == nil {
		copilotPath := filepath.Join(home, ".copilot", "mcp-config.json")
		if _, err := os.Stat(copilotPath); err == nil {
			sources = append(sources, ExternalSource{Label: "copilot-mcp-config", ConfigPath: copilotPath})
		}

		codexPath := filepath.Join(home, ".codex", "config.toml")
		if _, err := os.Stat(codexPath); err == nil {
			sources = append(sources, ExternalSource{Label: "codex-config", ConfigPath: codexPath})
		}
	}

	return sources, nil
}

// ReadExternalServers reads MCP server definitions from an external source.
// Returns a map of server name → MCPServerDef.
func ReadExternalServers(src ExternalSource) (map[string]agents.MCPServerDef, error) {
	switch src.Label {
	case "claude-settings":
		return readClaudeSettingsServers(src.ConfigPath)
	case "copilot-mcp-config":
		return readCopilotConfigServers(src.ConfigPath)
	case "codex-config":
		return readCodexConfigServers(src.ConfigPath)
	default:
		return nil, nil
	}
}

// RemoveExternalServers removes the named servers from an external source config file,
// preserving all other content in the file.
func RemoveExternalServers(src ExternalSource, names []string) error {
	switch src.Label {
	case "claude-settings":
		return removeFromClaudeSettings(src.ConfigPath, names)
	case "copilot-mcp-config":
		return removeFromCopilotConfig(src.ConfigPath, names)
	case "codex-config":
		return removeFromCodexConfig(src.ConfigPath, names)
	default:
		return nil
	}
}

func readClaudeSettingsServers(path string) (map[string]agents.MCPServerDef, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cs struct {
		MCPServers map[string]struct {
			Command string            `json:"command"`
			Args    []string          `json:"args"`
			Env     map[string]string `json:"env,omitempty"`
		} `json:"mcpServers"`
	}
	if err := json.Unmarshal(data, &cs); err != nil {
		return nil, err
	}
	result := make(map[string]agents.MCPServerDef, len(cs.MCPServers))
	for name, srv := range cs.MCPServers {
		result[name] = agents.MCPServerDef{
			Transport: "stdio",
			Command:   srv.Command,
			Args:      srv.Args,
			Env:       srv.Env,
			Enabled:   agents.BoolPtr(true),
		}
	}
	return result, nil
}

func readCopilotConfigServers(path string) (map[string]agents.MCPServerDef, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cc struct {
		MCPServers map[string]struct {
			Type    string            `json:"type"`
			Command string            `json:"command"`
			Args    []string          `json:"args"`
			Env     map[string]string `json:"env,omitempty"`
			URL     string            `json:"url,omitempty"`
			Headers map[string]string `json:"headers,omitempty"`
		} `json:"mcpServers"`
	}
	if err := json.Unmarshal(data, &cc); err != nil {
		return nil, err
	}
	result := make(map[string]agents.MCPServerDef, len(cc.MCPServers))
	for name, srv := range cc.MCPServers {
		transport := "stdio"
		if srv.Type == "http" {
			transport = "http"
		}
		result[name] = agents.MCPServerDef{
			Transport: transport,
			Command:   srv.Command,
			Args:      srv.Args,
			Env:       srv.Env,
			URL:       srv.URL,
			Headers:   srv.Headers,
			Enabled:   agents.BoolPtr(true),
		}
	}
	return result, nil
}

func readCodexConfigServers(path string) (map[string]agents.MCPServerDef, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var dc struct {
		MCPServers map[string]struct {
			Command string   `toml:"command"`
			Args    []string `toml:"args"`
		} `toml:"mcp_servers"`
	}
	if _, err := toml.Decode(string(data), &dc); err != nil {
		return nil, err
	}
	result := make(map[string]agents.MCPServerDef, len(dc.MCPServers))
	for name, srv := range dc.MCPServers {
		result[name] = agents.MCPServerDef{
			Transport: "stdio",
			Command:   srv.Command,
			Args:      srv.Args,
			Enabled:   agents.BoolPtr(true),
		}
	}
	return result, nil
}

// removeFromClaudeSettings removes named servers from ~/.claude/settings.json,
// preserving all other content in the file.
func removeFromClaudeSettings(path string, names []string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var root map[string]json.RawMessage
	if err := json.Unmarshal(data, &root); err != nil {
		return err
	}
	if raw, ok := root["mcpServers"]; ok {
		var servers map[string]json.RawMessage
		if err := json.Unmarshal(raw, &servers); err == nil {
			for _, name := range names {
				delete(servers, name)
			}
			updated, err := json.Marshal(servers)
			if err != nil {
				return err
			}
			root["mcpServers"] = updated
		}
	}
	out, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(out, '\n'), 0644)
}

// removeFromCopilotConfig removes named servers from ~/.copilot/mcp-config.json.
func removeFromCopilotConfig(path string, names []string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var root map[string]json.RawMessage
	if err := json.Unmarshal(data, &root); err != nil {
		return err
	}
	if raw, ok := root["mcpServers"]; ok {
		var servers map[string]json.RawMessage
		if err := json.Unmarshal(raw, &servers); err == nil {
			for _, name := range names {
				delete(servers, name)
			}
			updated, err := json.Marshal(servers)
			if err != nil {
				return err
			}
			root["mcpServers"] = updated
		}
	}
	out, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(out, '\n'), 0644)
}

// removeFromCodexConfig removes named servers from ~/.codex/config.toml.
func removeFromCodexConfig(path string, names []string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var root map[string]interface{}
	if _, err := toml.Decode(string(data), &root); err != nil {
		return err
	}
	if mcpRaw, ok := root["mcp_servers"]; ok {
		if mcpMap, ok := mcpRaw.(map[string]interface{}); ok {
			for _, name := range names {
				delete(mcpMap, name)
			}
			root["mcp_servers"] = mcpMap
		}
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return toml.NewEncoder(f).Encode(root)
}
