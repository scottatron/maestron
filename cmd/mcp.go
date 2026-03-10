package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/scottatron/maestron/internal/discover"
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
}

func runMCP(cmd *cobra.Command, args []string) error {
	servers, err := discover.ListMCPServers()
	if err != nil {
		return err
	}

	// Apply filters
	var filtered []discover.MCPServerInfo
	for _, s := range servers {
		if mcpEnabledOnly && !s.Enabled {
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
	t := output.NewTable(os.Stdout, []string{"NAME", "COMMAND", "TRANSPORT", "TARGETS", "ENABLED"})
	for _, s := range servers {
		cmd := s.Command
		if len(s.Args) > 0 {
			cmd += " " + strings.Join(s.Args[:min(len(s.Args), 2)], " ")
		}
		targets := strings.Join(s.Targets, ",")
		if len(targets) > 40 {
			targets = targets[:37] + "..."
		}
		enabled := "yes"
		if !s.Enabled {
			enabled = "no"
		}
		t.Row(s.Name, cmd, s.Transport, targets, enabled)
	}
	t.Flush()
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
