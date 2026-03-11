package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/scottatron/maestron/internal/agents"
	"github.com/scottatron/maestron/internal/discover"
)

// RegisterLaunchCommands discovers installed agents and registers launch subcommands.
// Errors are silently ignored — if mise is unavailable, no launch commands are registered.
func RegisterLaunchCommands(root *cobra.Command) {
	agentList, err := discover.ListAgents(false)
	if err != nil {
		return
	}
	for _, agent := range agentList {
		registerLaunchCmd(root, agent)
	}
}

func registerLaunchCmd(root *cobra.Command, agent discover.AgentInfo) {
	// Don't shadow existing commands
	for _, existing := range root.Commands() {
		if existing.Name() == agent.Name {
			return
		}
	}
	cmd := &cobra.Command{
		Use:                agent.Name,
		Short:              fmt.Sprintf("Launch %s", agent.DisplayName),
		Args:               cobra.ArbitraryArgs,
		DisableFlagParsing: true,
		RunE:               makeLaunchRunE(agent.Name),
	}
	root.AddCommand(cmd)
}

func makeLaunchRunE(agentName string) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		binary, err := exec.LookPath(agentName)
		if err != nil {
			return fmt.Errorf("%s not found on PATH: %w", agentName, err)
		}

		extraArgs := extraArgsFor(agentName)
		finalArgs := append(extraArgs, args...)

		proc := exec.Command(binary, finalArgs...)
		proc.Stdin = os.Stdin
		proc.Stdout = os.Stdout
		proc.Stderr = os.Stderr

		return proc.Run()
	}
}

// extraArgsFor returns any extra args to prepend when launching the given agent.
func extraArgsFor(agentName string) []string {
	switch agentName {
	case "copilot":
		return copilotExtraArgs()
	}
	return nil
}

// copilotExtraArgs returns --additional-mcp-config args if .copilot/mcp-config.json exists.
func copilotExtraArgs() []string {
	root, _, err := agents.FindAgentsConfig()
	if err != nil || root == "" {
		return nil
	}
	mcpConfigPath := filepath.Join(root, ".copilot", "mcp-config.json")
	data, err := os.ReadFile(mcpConfigPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: .copilot/mcp-config.json not found; run `maestron sync` to generate it\n")
		return nil
	}
	return []string{"--additional-mcp-config", string(data)}
}
