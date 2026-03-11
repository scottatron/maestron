package manage

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/scottatron/maestron/internal/agents"
)

// LocalOverrides mirrors .agents/local.json, providing per-machine overrides.
type LocalOverrides struct {
	MCPServers map[string]LocalOverrideServer `json:"mcpServers,omitempty"`
}

// LocalOverrideServer holds local overrides for a single MCP server.
type LocalOverrideServer struct {
	Env     map[string]string `json:"env,omitempty"`
	Headers map[string]string `json:"headers,omitempty"`
	Args    []string          `json:"args,omitempty"`
}

// ReadAgentsConfig reads agents.json from the given project root.
func ReadAgentsConfig(root string) (*agents.AgentsConfig, error) {
	path := filepath.Join(root, ".agents", "agents.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg agents.AgentsConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// WriteAgentsConfig writes agents.json, updating the lastSync timestamp.
func WriteAgentsConfig(root string, cfg *agents.AgentsConfig) error {
	cfg.LastSync = time.Now().UTC().Format("2006-01-02T15:04:05.000Z07:00")
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	path := filepath.Join(root, ".agents", "agents.json")
	return os.WriteFile(path, append(data, '\n'), 0644)
}

// ReadLocalOverrides reads .agents/local.json. Returns empty overrides if not found.
func ReadLocalOverrides(root string) (*LocalOverrides, error) {
	path := filepath.Join(root, ".agents", "local.json")
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return &LocalOverrides{}, nil
	}
	if err != nil {
		return nil, err
	}
	var lo LocalOverrides
	if err := json.Unmarshal(data, &lo); err != nil {
		return nil, err
	}
	return &lo, nil
}

// WriteLocalOverrides writes .agents/local.json.
func WriteLocalOverrides(root string, lo *LocalOverrides) error {
	data, err := json.MarshalIndent(lo, "", "  ")
	if err != nil {
		return err
	}
	path := filepath.Join(root, ".agents", "local.json")
	return os.WriteFile(path, append(data, '\n'), 0644)
}

// AddMCPServer adds or replaces an MCP server in agents.json (or global.json if global is true).
func AddMCPServer(root, name string, def agents.MCPServerDef, local *LocalOverrideServer, global bool) error {
	if global {
		return addGlobalMCPServer(name, def)
	}
	cfg, err := ReadAgentsConfig(root)
	if err != nil {
		return err
	}
	if cfg.MCP.Servers == nil {
		cfg.MCP.Servers = make(map[string]agents.MCPServerDef)
	}
	cfg.MCP.Servers[name] = def
	if err := WriteAgentsConfig(root, cfg); err != nil {
		return err
	}
	if local != nil {
		lo, err := ReadLocalOverrides(root)
		if err != nil {
			return err
		}
		if lo.MCPServers == nil {
			lo.MCPServers = make(map[string]LocalOverrideServer)
		}
		lo.MCPServers[name] = *local
		return WriteLocalOverrides(root, lo)
	}
	return nil
}

// RemoveMCPServer removes an MCP server from agents.json (or global.json if global is true).
func RemoveMCPServer(root, name string, global bool) error {
	if global {
		return removeGlobalMCPServer(name)
	}
	cfg, err := ReadAgentsConfig(root)
	if err != nil {
		return err
	}
	if _, ok := cfg.MCP.Servers[name]; !ok {
		return fmt.Errorf("MCP server %q not found in agents.json", name)
	}
	delete(cfg.MCP.Servers, name)
	if err := WriteAgentsConfig(root, cfg); err != nil {
		return err
	}
	// Also remove from local overrides if present
	lo, err := ReadLocalOverrides(root)
	if err != nil {
		return err
	}
	if _, ok := lo.MCPServers[name]; ok {
		delete(lo.MCPServers, name)
		return WriteLocalOverrides(root, lo)
	}
	return nil
}

// SetMCPServerEnabled enables or disables an MCP server in agents.json.
func SetMCPServerEnabled(root, name string, enabled bool) error {
	cfg, err := ReadAgentsConfig(root)
	if err != nil {
		return err
	}
	def, ok := cfg.MCP.Servers[name]
	if !ok {
		return fmt.Errorf("MCP server %q not found in agents.json", name)
	}
	def.Enabled = enabled
	cfg.MCP.Servers[name] = def
	return WriteAgentsConfig(root, cfg)
}

// globalConfigPath returns the path to ~/.agents/global.json.
func globalConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".agents", "global.json"), nil
}

// ReadGlobalMcpConfig reads ~/.agents/global.json. Returns empty config if not found.
func ReadGlobalMcpConfig() (*agents.GlobalMcpConfig, error) {
	path, err := globalConfigPath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return &agents.GlobalMcpConfig{MCPServers: map[string]agents.MCPServerDef{}}, nil
	}
	if err != nil {
		return nil, err
	}
	var gc agents.GlobalMcpConfig
	if err := json.Unmarshal(data, &gc); err != nil {
		return nil, err
	}
	if gc.MCPServers == nil {
		gc.MCPServers = map[string]agents.MCPServerDef{}
	}
	return &gc, nil
}

// WriteGlobalMcpConfig writes ~/.agents/global.json.
func WriteGlobalMcpConfig(gc *agents.GlobalMcpConfig) error {
	path, err := globalConfigPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(gc, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0644)
}

func addGlobalMCPServer(name string, def agents.MCPServerDef) error {
	gc, err := ReadGlobalMcpConfig()
	if err != nil {
		return err
	}
	gc.MCPServers[name] = def
	return WriteGlobalMcpConfig(gc)
}

func removeGlobalMCPServer(name string) error {
	gc, err := ReadGlobalMcpConfig()
	if err != nil {
		return err
	}
	if _, ok := gc.MCPServers[name]; !ok {
		return fmt.Errorf("MCP server %q not found in global.json", name)
	}
	delete(gc.MCPServers, name)
	return WriteGlobalMcpConfig(gc)
}
