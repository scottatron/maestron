package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/charmbracelet/huh"
	"github.com/scottatron/maestron/internal/agents"
	"github.com/scottatron/maestron/internal/manage"
	"github.com/spf13/cobra"
)

var initForce bool

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialise a project agents.json with MCP server overrides",
	Long: `Creates .agents/agents.json in the current directory.

Reads MCP servers from the global config (~/.agents/global.json) and
presents an interactive picker to select which servers to enable for
this project. Non-selected servers are explicitly disabled.`,
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

		// Read global MCP servers
		globalCfg, err := manage.ReadGlobalMcpConfig()
		if err != nil {
			return fmt.Errorf("could not read global config: %w", err)
		}

		var selected []string

		if len(globalCfg.MCPServers) == 0 {
			fmt.Println("No MCP servers found in global config (~/.agents/global.json).")
			fmt.Println("Creating agents.json with no overrides.")
		} else {
			// Build sorted options list
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

			options := make([]huh.Option[string], len(opts))
			for i, o := range opts {
				options[i] = huh.NewOption(o.label, o.name)
			}

			form := huh.NewForm(
				huh.NewGroup(
					huh.NewMultiSelect[string]().
						Title("Select MCP servers to enable for this project").
						Description("Space to toggle, Enter to confirm").
						Options(options...).
						Value(&selected),
				),
			)

			if err := form.Run(); err != nil {
				return fmt.Errorf("selection cancelled: %w", err)
			}
		}

		// Build the enable set for quick lookup
		enabledSet := make(map[string]bool, len(selected))
		for _, name := range selected {
			enabledSet[name] = true
		}

		// Create agents.json with explicit enable/disable overrides
		cfg := &agents.AgentsConfig{}
		cfg.MCP.Servers = make(map[string]agents.MCPServerDef)
		for name := range globalCfg.MCPServers {
			cfg.MCP.Servers[name] = agents.MCPServerDef{
				Enabled: agents.BoolPtr(enabledSet[name]),
			}
		}

		if err := os.MkdirAll(agentsDir, 0755); err != nil {
			return fmt.Errorf("could not create .agents directory: %w", err)
		}

		if err := manage.WriteAgentsConfig(cwd, cfg); err != nil {
			return fmt.Errorf("could not write agents.json: %w", err)
		}

		fmt.Printf("Created %s\n", tildeAbbrev(agentsFile))
		if len(globalCfg.MCPServers) > 0 {
			fmt.Printf("  %d/%d MCP servers enabled\n", len(selected), len(globalCfg.MCPServers))
		}
		return nil
	},
}

func init() {
	initCmd.Flags().BoolVar(&initForce, "force", false, "overwrite existing agents.json")
}
