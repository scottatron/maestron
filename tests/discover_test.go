package tests

import (
	"testing"

	"github.com/scottatron/maestron/internal/agents"
	"github.com/scottatron/maestron/internal/discover"
)

func TestLookupKnownAgent(t *testing.T) {
	tests := []struct {
		toolName    string
		expectFound bool
		expectName  string
	}{
		{"aqua:anthropics/claude-code", true, "claude"},
		{"claude", true, "claude"},
		{"npm:@openai/codex", true, "codex"},
		{"opencode", true, "opencode"},
		{"kubectl", false, ""},
	}

	for _, tt := range tests {
		t.Run(tt.toolName, func(t *testing.T) {
			ka, ok := agents.LookupAgent(tt.toolName)
			if ok != tt.expectFound {
				t.Errorf("LookupAgent(%q): got found=%v, want %v", tt.toolName, ok, tt.expectFound)
			}
			if ok && ka.Name != tt.expectName {
				t.Errorf("LookupAgent(%q).Name = %q, want %q", tt.toolName, ka.Name, tt.expectName)
			}
		})
	}
}

func TestFindAgentsConfig(t *testing.T) {
	root, cfg, err := agents.FindAgentsConfig()
	if err != nil {
		t.Logf("FindAgentsConfig error (may be expected in CI): %v", err)
	}
	if root != "" && cfg == nil {
		t.Error("got root but nil config")
	}
	if cfg != nil {
		t.Logf("found agents.json at %s, schemaVersion=%d", root, cfg.SchemaVersion)
	}
}

func TestListAgents(t *testing.T) {
	agentList, err := discover.ListAgents(false)
	if err != nil {
		t.Logf("ListAgents error (mise may not be available): %v", err)
		return
	}
	t.Logf("found %d agents", len(agentList))
	for _, a := range agentList {
		t.Logf("  %s %s (active=%v)", a.DisplayName, a.Version, a.Active)
	}
}

func TestListMCPServers(t *testing.T) {
	servers, err := discover.ListMCPServers()
	if err != nil {
		t.Logf("ListMCPServers error: %v", err)
		return
	}
	t.Logf("found %d MCP servers (including duplicates across sources)", len(servers))
	for _, s := range servers {
		shadowed := ""
		if s.Shadowed {
			shadowed = " [shadowed]"
		}
		t.Logf("  %s (source=%s, enabled=%v%s)", s.Name, s.Source, agents.IsEnabled(s.Enabled), shadowed)
	}
}

func TestListSkills(t *testing.T) {
	skills, err := discover.ListSkills()
	if err != nil {
		t.Logf("ListSkills error: %v", err)
		return
	}
	t.Logf("found %d skills", len(skills))
	for _, s := range skills {
		t.Logf("  %s [%s]", s.Name, s.Source)
	}
}
