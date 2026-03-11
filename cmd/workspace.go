package cmd

import (
	"fmt"
	"sort"

	"github.com/spf13/cobra"

	"github.com/scottatron/maestron/internal/agents"
	"github.com/scottatron/maestron/internal/discover"
	"github.com/scottatron/maestron/internal/output"
)

var workspaceCmd = &cobra.Command{
	Use:   "workspace",
	Short: "Manage workspace configuration",
}

var workspaceStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Project config state",
	RunE:  runWorkspaceStatus,
}

func runWorkspaceStatus(cmd *cobra.Command, args []string) error {
	root, cfg, err := agents.FindAgentsConfig()
	if err != nil {
		return fmt.Errorf("finding project root: %w", err)
	}
	if cfg == nil {
		return fmt.Errorf("no agents.json found; run from a project directory")
	}

	mcpServers, _ := discover.ListMCPServers()
	// Keep only non-shadowed entries
	var activeMCP []discover.MCPServerInfo
	for _, s := range mcpServers {
		if !s.Shadowed {
			activeMCP = append(activeMCP, s)
		}
	}
	sort.Slice(activeMCP, func(i, j int) bool {
		return activeMCP[i].Name < activeMCP[j].Name
	})

	type workspaceStatus struct {
		Root         string                  `json:"root"`
		Instructions string                  `json:"instructions,omitempty"`
		LastSync     string                  `json:"lastSync,omitempty"`
		Integrations []string                `json:"integrations"`
		MCPServers   []discover.MCPServerInfo `json:"mcpServers"`
	}

	status := workspaceStatus{
		Root:         root,
		Instructions: cfg.Instructions.Path,
		LastSync:     cfg.LastSync,
		Integrations: cfg.Integrations.Enabled,
		MCPServers:   activeMCP,
	}
	if status.Integrations == nil {
		status.Integrations = []string{}
	}
	if status.MCPServers == nil {
		status.MCPServers = []discover.MCPServerInfo{}
	}

	output.Print(status, func() {
		renderWorkspaceStatus(status.Root, cfg, activeMCP)
	})
	return nil
}

func renderWorkspaceStatus(root string, cfg *agents.AgentsConfig, mcpServers []discover.MCPServerInfo) {
	fmt.Printf("Workspace: %s\n", tildeAbbrev(root))
	if cfg.Instructions.Path != "" {
		fmt.Printf("Instructions: %s\n", cfg.Instructions.Path)
	}
	if cfg.LastSync != "" {
		fmt.Printf("Last sync: %s\n", cfg.LastSync)
	}
	fmt.Println()

	fmt.Printf("Integrations (%d enabled):\n", len(cfg.Integrations.Enabled))
	if len(cfg.Integrations.Enabled) == 0 {
		fmt.Println("  (none)")
	}
	for _, name := range cfg.Integrations.Enabled {
		fmt.Printf("  • %s\n", name)
	}

	fmt.Println()
	fmt.Printf("MCP Servers (%d):\n", len(mcpServers))
	if len(mcpServers) == 0 {
		fmt.Println("  (none)")
	}
	for _, s := range mcpServers {
		enabled := "enabled"
		if !agents.IsEnabled(s.Enabled) {
			enabled = "disabled"
		}
		fmt.Printf("  • %-30s  %s  (%s)\n", s.Name, enabled, tildeAbbrev(s.ConfigPath))
	}
}

func init() {
	workspaceCmd.AddCommand(workspaceStatusCmd)
	workspaceCmd.AddCommand(initCmd)
	workspaceCmd.AddCommand(doctorCmd)
	workspaceCmd.AddCommand(syncCmd)
	workspaceCmd.AddCommand(mcpCmd)
}
