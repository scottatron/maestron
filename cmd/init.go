package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/charmbracelet/huh"
	"github.com/scottatron/maestron/internal/agents"
	"github.com/scottatron/maestron/internal/discover"
	"github.com/scottatron/maestron/internal/manage"
	"github.com/spf13/cobra"
)

var initForce bool

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialise a project agents.json with MCP server overrides",
	Long: `Creates .agents/agents.json in the current directory.

Reads discovered agents and global MCP servers, then presents interactive
pickers to configure which agents and MCP servers to enable for this project.
All discovered agents are pre-selected; deselect any you don't want.
Non-selected MCP servers are explicitly disabled.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("could not determine current directory: %w", err)
		}

		agentsDir := filepath.Join(cwd, ".agents")
		agentsFile := filepath.Join(agentsDir, "agents.json")

		if _, err := os.Stat(agentsFile); err == nil && !initForce {
			return fmt.Errorf("agents.json already exists at %s\nUse --force to overwrite", agentsFile)
		}

		// Discover installed agents
		discoveredAgents, _ := discover.ListAgents(false)

		// Read global MCP servers
		globalCfg, err := manage.ReadGlobalMcpConfig()
		if err != nil {
			return fmt.Errorf("could not read global config: %w", err)
		}

		// Build agent options — all pre-selected
		var selectedAgents []string
		var agentGroups []*huh.Group

		if len(discoveredAgents) > 0 {
			sort.Slice(discoveredAgents, func(i, j int) bool {
				return discoveredAgents[i].Name < discoveredAgents[j].Name
			})
			agentOptions := make([]huh.Option[string], len(discoveredAgents))
			for i, a := range discoveredAgents {
				label := a.DisplayName
				if a.Version != "" {
					label += " (" + a.Version + ")"
				}
				agentOptions[i] = huh.NewOption(label, a.Name).Selected(true)
			}
			// Pre-select all
			for _, a := range discoveredAgents {
				selectedAgents = append(selectedAgents, a.Name)
			}
			agentGroups = append(agentGroups, huh.NewGroup(
				huh.NewMultiSelect[string]().
					Title("Select agents to enable for this project").
					Description("All discovered agents are pre-selected. Space to toggle, Enter to confirm.").
					Options(agentOptions...).
					Value(&selectedAgents),
			))
		}

		// Build MCP server options
		var selectedMCP []string
		if len(globalCfg.MCPServers) > 0 {
			type serverOption struct {
				name  string
				label string
			}
			var opts []serverOption
			for name, def := range globalCfg.MCPServers {
				display := name
				if def.Label != "" {
					display = def.Label
				}
				opts = append(opts, serverOption{name: name, label: display})
			}
			sort.Slice(opts, func(i, j int) bool { return opts[i].name < opts[j].name })

			mcpOptions := make([]huh.Option[string], len(opts))
			for i, o := range opts {
				mcpOptions[i] = huh.NewOption(o.label, o.name)
			}
			agentGroups = append(agentGroups, huh.NewGroup(
				huh.NewMultiSelect[string]().
					Title("Select MCP servers to enable for this project").
					Description("Space to toggle, Enter to confirm").
					Options(mcpOptions...).
					Value(&selectedMCP),
			))
		}

		if len(agentGroups) > 0 {
			if err := huh.NewForm(agentGroups...).Run(); err != nil {
				return fmt.Errorf("selection cancelled: %w", err)
			}
		} else {
			fmt.Println("No agents or MCP servers found. Creating agents.json with defaults.")
		}

		// Build MCP enable set
		mcpEnabledSet := make(map[string]bool, len(selectedMCP))
		for _, name := range selectedMCP {
			mcpEnabledSet[name] = true
		}

		// Create agents.json
		cfg := &agents.AgentsConfig{}
		cfg.SchemaVersion = 3
		cfg.Instructions.Path = "AGENTS.md"
		cfg.Integrations.Enabled = selectedAgents
		cfg.MCP.Servers = make(map[string]agents.MCPServerDef)
		for name := range globalCfg.MCPServers {
			cfg.MCP.Servers[name] = agents.MCPServerDef{
				Enabled: agents.BoolPtr(mcpEnabledSet[name]),
			}
		}

		if err := os.MkdirAll(agentsDir, 0755); err != nil {
			return fmt.Errorf("could not create .agents directory: %w", err)
		}
		if err := manage.WriteAgentsConfig(cwd, cfg); err != nil {
			return fmt.Errorf("could not write agents.json: %w", err)
		}
		fmt.Printf("Created %s\n", tildeAbbrev(agentsFile))
		if len(discoveredAgents) > 0 {
			fmt.Printf("  %d/%d agents enabled\n", len(selectedAgents), len(discoveredAgents))
		}
		if len(globalCfg.MCPServers) > 0 {
			fmt.Printf("  %d/%d MCP servers enabled\n", len(selectedMCP), len(globalCfg.MCPServers))
		}

		return nil
	},
}

func init() {
	initCmd.Flags().BoolVar(&initForce, "force", false, "overwrite existing agents.json")
}
