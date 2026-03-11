package cmd

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/scottatron/maestron/internal/agents"
	"github.com/scottatron/maestron/internal/discover"
	"github.com/scottatron/maestron/internal/manage"
	"github.com/scottatron/maestron/internal/output"
)

var (
	mcpEnabledOnly bool
	mcpTarget      string
)

var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "List configured MCP servers",
	RunE:  runMCP,
}

func init() {
	mcpCmd.Flags().BoolVar(&mcpEnabledOnly, "enabled-only", false, "skip disabled servers")
	mcpCmd.Flags().StringVar(&mcpTarget, "target", "", "filter by target agent")

	mcpCmd.AddCommand(mcpAddCmd)
	mcpCmd.AddCommand(mcpRemoveCmd)
	mcpCmd.AddCommand(mcpEnableCmd)
	mcpCmd.AddCommand(mcpDisableCmd)
	mcpCmd.AddCommand(mcpConsolidateCmd)
}

func runMCP(cmd *cobra.Command, args []string) error {
	servers, err := discover.ListMCPServers()
	if err != nil {
		return err
	}

	// Apply filters
	var filtered []discover.MCPServerInfo
	for _, s := range servers {
		if mcpEnabledOnly && !agents.IsEnabled(s.Enabled) {
			continue
		}
		if mcpTarget != "" {
			found := false
			for _, t := range s.Targets {
				if t == mcpTarget {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}
		filtered = append(filtered, s)
	}

	// Sort: by name asc, then by source priority asc (so active entry comes first)
	sort.Slice(filtered, func(i, j int) bool {
		if filtered[i].Name != filtered[j].Name {
			return filtered[i].Name < filtered[j].Name
		}
		return discover.SourcePriority(filtered[i].Source) < discover.SourcePriority(filtered[j].Source)
	})

	output.Print(filtered, func() {
		renderMCPTable(filtered)
	})
	return nil
}

func renderMCPTable(servers []discover.MCPServerInfo) {
	if len(servers) == 0 {
		fmt.Println("No MCP servers found.")
		return
	}
	t := output.NewTable(os.Stdout, []string{"NAME", "COMMAND/URL", "TRANSPORT", "TARGETS", "ENABLED", "SOURCE"})
	for _, s := range servers {
		name := s.Name
		if s.Shadowed {
			name = "~ " + name
		}

		cmdStr := s.Command
		if len(s.Args) > 0 {
			cmdStr += " " + strings.Join(s.Args[:min(len(s.Args), 2)], " ")
		}
		if cmdStr == "" && s.URL != "" {
			cmdStr = s.URL
		}
		targets := strings.Join(s.Targets, ",")
		if len(targets) > 40 {
			targets = targets[:37] + "..."
		}
		enabled := "yes"
		if !agents.IsEnabled(s.Enabled) {
			enabled = "no"
		}
		source := s.Source
		if s.Shadowed {
			source = source + " (shadowed)"
		}
		t.Row(name, cmdStr, s.Transport, targets, enabled, source)
	}
	t.Flush()
}

// --- mcp add ---

var (
	addCommand     string
	addArgs        []string
	addURL         string
	addEnv         []string
	addTransport   string
	addTargets     []string
	addDescription string
	addDisabled    bool
	addGlobal      bool
)

var mcpAddCmd = &cobra.Command{
	Use:   "add <name>",
	Short: "Add an MCP server to agents.json",
	Args:  cobra.ExactArgs(1),
	RunE:  runMCPAdd,
}

func init() {
	mcpAddCmd.Flags().StringVar(&addCommand, "command", "", "command to run (stdio transport)")
	mcpAddCmd.Flags().StringArrayVar(&addArgs, "arg", nil, "argument (repeatable)")
	mcpAddCmd.Flags().StringVar(&addURL, "url", "", "URL (http/sse transport)")
	mcpAddCmd.Flags().StringArrayVar(&addEnv, "env", nil, "environment variable KEY=VALUE (repeatable)")
	mcpAddCmd.Flags().StringVar(&addTransport, "transport", "", "transport: stdio, http, sse (auto-detected if omitted)")
	mcpAddCmd.Flags().StringArrayVar(&addTargets, "target", nil, "target integration (repeatable)")
	mcpAddCmd.Flags().StringVar(&addDescription, "description", "", "server description")
	mcpAddCmd.Flags().BoolVar(&addDisabled, "disabled", false, "add server in disabled state")
	mcpAddCmd.Flags().BoolVar(&addGlobal, "global", false, "add to ~/.agents/global.json instead of agents.json")
}

func runMCPAdd(cmd *cobra.Command, args []string) error {
	name := args[0]

	// Auto-detect transport
	transport := addTransport
	if transport == "" {
		if addURL != "" {
			transport = "http"
		} else {
			transport = "stdio"
		}
	}

	// Parse env vars
	env := map[string]string{}
	for _, kv := range addEnv {
		parts := strings.SplitN(kv, "=", 2)
		if len(parts) != 2 {
			return fmt.Errorf("invalid --env value %q: expected KEY=VALUE", kv)
		}
		env[parts[0]] = parts[1]
	}

	def := agents.MCPServerDef{
		Description: addDescription,
		Transport:   transport,
		Command:     addCommand,
		Args:        addArgs,
		URL:         addURL,
		Targets:     addTargets,
		Enabled:     agents.BoolPtr(!addDisabled),
	}
	if len(env) > 0 {
		def.Env = env
	}

	root, _, err := agents.FindAgentsConfig()
	if err != nil {
		return fmt.Errorf("finding project root: %w", err)
	}
	if root == "" && !addGlobal {
		return fmt.Errorf("no agents.json found; run from a project directory or use --global")
	}

	if err := manage.AddMCPServer(root, name, def, nil, addGlobal); err != nil {
		return err
	}

	target := "agents.json"
	if addGlobal {
		target = "~/.agents/global.json"
	}
	fmt.Printf("Added MCP server %q to %s\n", name, target)
	fmt.Println("Run `maestron sync` to apply changes.")
	return nil
}

// --- mcp remove ---

var removeGlobal bool

var mcpRemoveCmd = &cobra.Command{
	Use:   "remove <name>",
	Short: "Remove an MCP server from agents.json",
	Args:  cobra.ExactArgs(1),
	RunE:  runMCPRemove,
}

func init() {
	mcpRemoveCmd.Flags().BoolVar(&removeGlobal, "global", false, "remove from ~/.agents/global.json")
}

func runMCPRemove(cmd *cobra.Command, args []string) error {
	name := args[0]

	root, _, err := agents.FindAgentsConfig()
	if err != nil {
		return fmt.Errorf("finding project root: %w", err)
	}
	if root == "" && !removeGlobal {
		return fmt.Errorf("no agents.json found; run from a project directory or use --global")
	}

	if err := manage.RemoveMCPServer(root, name, removeGlobal); err != nil {
		return err
	}

	target := "agents.json"
	if removeGlobal {
		target = "~/.agents/global.json"
	}
	fmt.Printf("Removed MCP server %q from %s\n", name, target)
	fmt.Println("Run `maestron sync` to apply changes.")
	return nil
}

// --- mcp enable ---

var mcpEnableCmd = &cobra.Command{
	Use:   "enable <name>",
	Short: "Enable an MCP server in agents.json",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runMCPSetEnabled(args[0], true)
	},
}

var mcpDisableCmd = &cobra.Command{
	Use:   "disable <name>",
	Short: "Disable an MCP server in agents.json",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runMCPSetEnabled(args[0], false)
	},
}

func runMCPSetEnabled(name string, enabled bool) error {
	root, _, err := agents.FindAgentsConfig()
	if err != nil {
		return fmt.Errorf("finding project root: %w", err)
	}
	if root == "" {
		return fmt.Errorf("no agents.json found; run from a project directory")
	}

	if err := manage.SetMCPServerEnabled(root, name, enabled); err != nil {
		return err
	}

	state := "enabled"
	if !enabled {
		state = "disabled"
	}
	fmt.Printf("MCP server %q %s\n", name, state)
	fmt.Println("Run `maestron sync` to apply changes.")
	return nil
}

// --- mcp consolidate ---

var (
	consolidateDryRun bool
	consolidateYes    bool
)

var mcpConsolidateCmd = &cobra.Command{
	Use:   "consolidate",
	Short: "Move MCP server definitions from agents.json into global.json",
	Long: `Consolidate moves all MCP server definitions from the project's agents.json
into ~/.agents/global.json, leaving only enable/disable overrides in agents.json.

This supports a workflow where server definitions are managed globally and
projects only opt in/out of individual servers.`,
	RunE: runMCPConsolidate,
}

func init() {
	mcpConsolidateCmd.Flags().BoolVar(&consolidateDryRun, "dry-run", false, "show what would be done without making changes")
	mcpConsolidateCmd.Flags().BoolVarP(&consolidateYes, "yes", "y", false, "skip confirmation prompt")
}

func runMCPConsolidate(cmd *cobra.Command, args []string) error {
	root, cfg, err := agents.FindAgentsConfig()
	if err != nil {
		return fmt.Errorf("finding project root: %w", err)
	}
	if root == "" {
		return fmt.Errorf("no agents.json found; run from a project directory")
	}

	if len(cfg.MCP.Servers) == 0 {
		fmt.Println("No MCP servers in agents.json to consolidate.")
		return nil
	}

	// Show preview
	fmt.Printf("Will consolidate %d server(s) from agents.json → ~/.agents/global.json:\n\n", len(cfg.MCP.Servers))
	names := make([]string, 0, len(cfg.MCP.Servers))
	for name := range cfg.MCP.Servers {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		def := cfg.MCP.Servers[name]
		enabled := "enabled"
		if !agents.IsEnabled(def.Enabled) {
			enabled = "disabled"
		}
		fmt.Printf("  %-30s  (%s)\n", name, enabled)
	}
	fmt.Println()

	if consolidateDryRun {
		fmt.Println("Dry run — no changes made.")
		return nil
	}

	if !consolidateYes {
		fmt.Print("Proceed? [y/N] ")
		var answer string
		fmt.Scanln(&answer)
		if strings.ToLower(strings.TrimSpace(answer)) != "y" {
			fmt.Println("Aborted.")
			return nil
		}
	}

	results, err := manage.ConsolidateMCPServers(root, false)
	if err != nil {
		return err
	}

	for _, r := range results {
		switch r.Action {
		case "moved-to-global":
			fmt.Printf("  moved   %s → global.json\n", r.Name)
		case "merged":
			fmt.Printf("  merged  %s → global.json (merged with existing entry)\n", r.Name)
		}
	}

	fmt.Println("\nDone. agents.json now contains only enable/disable overrides.")
	fmt.Println("Run `maestron sync` to apply changes.")
	return nil
}
