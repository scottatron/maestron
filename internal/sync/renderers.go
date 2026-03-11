package sync

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"

	"github.com/scottatron/maestron/internal/agents"
)

// RenderOutput holds the target path and content for a rendered config.
type RenderOutput struct {
	Path string
	Data []byte
}

// integrationRenderers maps integration names to their renderer functions.
var integrationRenderers = map[string]func(root string, servers map[string]agents.MCPServerDef) (RenderOutput, error){
	"claude":         RenderClaude,
	"codex":          RenderCodex,
	"gemini":         RenderGemini,
	"copilot":        RenderCopilotCLI,
	"copilot_vscode": RenderVSCode,
	"vscode":         RenderVSCode,
	"cursor":         RenderCursor,
	"windsurf":       RenderWindsurf,
	"antigravity":    RenderAntigravity,
	"opencode":       RenderOpenCode,
}

// RenderClaude merges mcpServers into .mcp.json at the project root.
func RenderClaude(root string, servers map[string]agents.MCPServerDef) (RenderOutput, error) {
	type claudeServer struct {
		Type    string            `json:"type,omitempty"`
		Command string            `json:"command,omitempty"`
		Args    []string          `json:"args,omitempty"`
		Env     map[string]string `json:"env,omitempty"`
		URL     string            `json:"url,omitempty"`
		Headers map[string]string `json:"headers,omitempty"`
	}

	outPath := filepath.Join(root, ".mcp.json")

	// Read existing file to preserve non-mcpServers keys
	existing := map[string]json.RawMessage{}
	if data, err := os.ReadFile(outPath); err == nil {
		_ = json.Unmarshal(data, &existing)
	}

	// Build mcpServers map
	mcpServers := map[string]claudeServer{}
	for name, def := range servers {
		srv := claudeServer{
			Command: def.Command,
			Args:    def.Args,
			Env:     def.Env,
			URL:     def.URL,
			Headers: def.Headers,
		}
		if def.Transport == "http" || def.Transport == "sse" {
			srv.Type = def.Transport
		} else if def.Command != "" {
			srv.Type = "stdio"
		}
		mcpServers[name] = srv
	}

	mcpData, err := json.Marshal(mcpServers)
	if err != nil {
		return RenderOutput{}, err
	}
	existing["mcpServers"] = mcpData

	data, err := marshalJSON(existing)
	if err != nil {
		return RenderOutput{}, err
	}
	return RenderOutput{Path: outPath, Data: data}, nil
}

// RenderCodex generates .codex/config.toml
func RenderCodex(root string, servers map[string]agents.MCPServerDef) (RenderOutput, error) {
	type codexServer struct {
		Command string   `toml:"command"`
		Args    []string `toml:"args,omitempty"`
		Enabled bool     `toml:"enabled"`
	}
	type codexConfig struct {
		MCPServers map[string]codexServer `toml:"mcp_servers"`
	}

	cfg := codexConfig{MCPServers: map[string]codexServer{}}
	for name, def := range servers {
		cfg.MCPServers[name] = codexServer{
			Command: def.Command,
			Args:    def.Args,
			Enabled: true,
		}
	}

	var buf bytes.Buffer
	if err := toml.NewEncoder(&buf).Encode(cfg); err != nil {
		return RenderOutput{}, err
	}
	return RenderOutput{Path: filepath.Join(root, ".codex", "config.toml"), Data: buf.Bytes()}, nil
}

// RenderGemini generates .gemini/settings.json (merges with existing).
func RenderGemini(root string, servers map[string]agents.MCPServerDef) (RenderOutput, error) {
	type geminiServer struct {
		Command string            `json:"command,omitempty"`
		Args    []string          `json:"args,omitempty"`
		Env     map[string]string `json:"env,omitempty"`
		URL     string            `json:"url,omitempty"`
	}

	path := filepath.Join(root, ".gemini", "settings.json")
	existing := map[string]interface{}{}
	if data, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(data, &existing)
	}

	mcpServers := map[string]geminiServer{}
	for name, def := range servers {
		srv := geminiServer{
			Command: def.Command,
			Args:    def.Args,
			Env:     def.Env,
			URL:     def.URL,
		}
		mcpServers[name] = srv
	}
	existing["mcpServers"] = mcpServers

	data, err := marshalJSON(existing)
	if err != nil {
		return RenderOutput{}, err
	}
	return RenderOutput{Path: path, Data: data}, nil
}

// RenderCopilotCLI generates .copilot/mcp-config.json
func RenderCopilotCLI(root string, servers map[string]agents.MCPServerDef) (RenderOutput, error) {
	type copilotServer struct {
		Type    string            `json:"type"`
		Command string            `json:"command,omitempty"`
		Args    []string          `json:"args,omitempty"`
		Env     map[string]string `json:"env,omitempty"`
		URL     string            `json:"url,omitempty"`
		Headers map[string]string `json:"headers,omitempty"`
		Tools   []string          `json:"tools"`
	}
	type copilotConfig struct {
		MCPServers map[string]copilotServer `json:"mcpServers"`
	}

	cfg := copilotConfig{MCPServers: map[string]copilotServer{}}
	for name, def := range servers {
		srvType := "local"
		if def.Transport == "http" || def.Transport == "sse" {
			srvType = "http"
		}
		cfg.MCPServers[name] = copilotServer{
			Type:    srvType,
			Command: def.Command,
			Args:    def.Args,
			Env:     def.Env,
			URL:     def.URL,
			Headers: def.Headers,
			Tools:   []string{"*"},
		}
	}

	data, err := marshalJSON(cfg)
	if err != nil {
		return RenderOutput{}, err
	}
	return RenderOutput{Path: filepath.Join(root, ".copilot", "mcp-config.json"), Data: data}, nil
}

// RenderVSCode generates .vscode/mcp.json
func RenderVSCode(root string, servers map[string]agents.MCPServerDef) (RenderOutput, error) {
	type vscodeServer struct {
		Type    string            `json:"type,omitempty"`
		Command string            `json:"command,omitempty"`
		Args    []string          `json:"args,omitempty"`
		Env     map[string]string `json:"env,omitempty"`
		URL     string            `json:"url,omitempty"`
	}
	type vscodeConfig struct {
		Servers map[string]vscodeServer `json:"servers"`
	}

	cfg := vscodeConfig{Servers: map[string]vscodeServer{}}
	for name, def := range servers {
		srv := vscodeServer{
			Command: def.Command,
			Args:    def.Args,
			Env:     def.Env,
			URL:     def.URL,
		}
		if def.Transport == "http" || def.Transport == "sse" {
			srv.Type = def.Transport
		} else {
			srv.Type = "stdio"
		}
		cfg.Servers[name] = srv
	}

	data, err := marshalJSON(cfg)
	if err != nil {
		return RenderOutput{}, err
	}
	return RenderOutput{Path: filepath.Join(root, ".vscode", "mcp.json"), Data: data}, nil
}

// RenderOpenCode generates opencode.json (merges with existing).
func RenderOpenCode(root string, servers map[string]agents.MCPServerDef) (RenderOutput, error) {
	type opencodeServer struct {
		Type    string   `json:"type"`
		Command []string `json:"command,omitempty"`
		URL     string   `json:"url,omitempty"`
		Enabled bool     `json:"enabled"`
	}

	path := filepath.Join(root, "opencode.json")
	existing := map[string]interface{}{}
	if data, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(data, &existing)
	}

	mcp := map[string]opencodeServer{}
	for name, def := range servers {
		srv := opencodeServer{Enabled: true}
		if def.Transport == "http" || def.Transport == "sse" {
			srv.Type = "remote"
			srv.URL = def.URL
		} else {
			srv.Type = "local"
			cmd := []string{def.Command}
			cmd = append(cmd, def.Args...)
			srv.Command = cmd
		}
		mcp[name] = srv
	}
	existing["mcp"] = mcp

	data, err := marshalJSON(existing)
	if err != nil {
		return RenderOutput{}, err
	}
	return RenderOutput{Path: path, Data: data}, nil
}

// RenderCursor generates .cursor/mcp.json
func RenderCursor(root string, servers map[string]agents.MCPServerDef) (RenderOutput, error) {
	type cursorServer struct {
		Type    string            `json:"type,omitempty"`
		Command string            `json:"command,omitempty"`
		Args    []string          `json:"args,omitempty"`
		Env     map[string]string `json:"env,omitempty"`
		URL     string            `json:"url,omitempty"`
	}
	type cursorConfig struct {
		MCPServers map[string]cursorServer `json:"mcpServers"`
	}

	cfg := cursorConfig{MCPServers: map[string]cursorServer{}}
	for name, def := range servers {
		srv := cursorServer{
			Command: def.Command,
			Args:    def.Args,
			Env:     def.Env,
			URL:     def.URL,
		}
		if def.Transport == "http" || def.Transport == "sse" {
			srv.Type = def.Transport
		} else {
			srv.Type = "stdio"
		}
		cfg.MCPServers[name] = srv
	}

	data, err := marshalJSON(cfg)
	if err != nil {
		return RenderOutput{}, err
	}
	return RenderOutput{Path: filepath.Join(root, ".cursor", "mcp.json"), Data: data}, nil
}

// RenderWindsurf generates ~/.windsurf/mcp.json
func RenderWindsurf(root string, servers map[string]agents.MCPServerDef) (RenderOutput, error) {
	type windsurfServer struct {
		Command string            `json:"command,omitempty"`
		Args    []string          `json:"args,omitempty"`
		Env     map[string]string `json:"env,omitempty"`
		URL     string            `json:"url,omitempty"`
	}
	type windsurfConfig struct {
		MCPServers map[string]windsurfServer `json:"mcpServers"`
	}

	cfg := windsurfConfig{MCPServers: map[string]windsurfServer{}}
	for name, def := range servers {
		cfg.MCPServers[name] = windsurfServer{
			Command: def.Command,
			Args:    def.Args,
			Env:     def.Env,
			URL:     def.URL,
		}
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return RenderOutput{}, err
	}
	path := filepath.Join(home, ".windsurf", "mcp.json")
	data, err := marshalJSON(cfg)
	if err != nil {
		return RenderOutput{}, err
	}
	return RenderOutput{Path: path, Data: data}, nil
}

// RenderAntigravity generates ~/.config/antigravity/mcp.json
func RenderAntigravity(root string, servers map[string]agents.MCPServerDef) (RenderOutput, error) {
	type agServer struct {
		Command string            `json:"command,omitempty"`
		Args    []string          `json:"args,omitempty"`
		Env     map[string]string `json:"env,omitempty"`
		URL     string            `json:"url,omitempty"`
	}
	type agConfig struct {
		Servers    map[string]agServer `json:"servers"`
		MCPServers map[string]agServer `json:"mcpServers"`
	}

	cfg := agConfig{
		Servers:    map[string]agServer{},
		MCPServers: map[string]agServer{},
	}
	for name, def := range servers {
		srv := agServer{
			Command: def.Command,
			Args:    def.Args,
			Env:     def.Env,
			URL:     def.URL,
		}
		cfg.Servers[name] = srv
		cfg.MCPServers[name] = srv
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return RenderOutput{}, err
	}
	path := filepath.Join(home, ".config", "antigravity", "mcp.json")
	data, err := marshalJSON(cfg)
	if err != nil {
		return RenderOutput{}, err
	}
	return RenderOutput{Path: path, Data: data}, nil
}

func marshalJSON(v interface{}) ([]byte, error) {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}
