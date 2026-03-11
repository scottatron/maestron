package cmd

import (
	"fmt"
	"runtime/debug"

	"github.com/spf13/cobra"

	"github.com/scottatron/maestron/internal/output"
)

var version = "dev"

var rootCmd = &cobra.Command{
	Use:   "maestron",
	Short: "Node-level agent introspection tool",
	Long: `maestron reports the full state of an AI agent node:
what agents, sessions, skills, and MCP servers exist on this machine.`,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		jsonFlag, _ := cmd.Flags().GetBool("json")
		output.SetJSONMode(jsonFlag)
	},
}

// Execute runs the root command.
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	if version == "dev" {
		if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
			version = info.Main.Version
		}
	}
	rootCmd.Version = version
	rootCmd.SetVersionTemplate(fmt.Sprintf("maestron version %s\n", version))

	rootCmd.PersistentFlags().BoolP("json", "j", false, "output as JSON")

	rootCmd.AddCommand(workspaceCmd)
	rootCmd.AddCommand(nodeCmd)

	RegisterLaunchCommands(rootCmd)
}
