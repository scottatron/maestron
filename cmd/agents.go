package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/scottatron/maestron/internal/discover"
	"github.com/scottatron/maestron/internal/output"
)

var agentsAll bool

var agentsCmd = &cobra.Command{
	Use:   "agents",
	Short: "List agents installed via mise",
	RunE:  runAgents,
}

func init() {
	agentsCmd.Flags().BoolVar(&agentsAll, "all", false, "include non-agent mise tools")
}

func runAgents(cmd *cobra.Command, args []string) error {
	agentList, err := discover.ListAgents(agentsAll)
	if err != nil {
		return err
	}

	output.Print(agentList, func() {
		renderAgentsTable(agentList)
	})
	return nil
}

func renderAgentsTable(agentList []discover.AgentInfo) {
	if len(agentList) == 0 {
		fmt.Println("No agents found.")
		return
	}
	t := output.NewTable(os.Stdout, []string{"NAME", "VERSION", "SOURCE", "STATUS"})
	for _, a := range agentList {
		status := "inactive"
		if a.Active {
			status = "active"
		}
		source := a.Source
		if source == "" {
			source = "-"
		}
		t.Row(a.DisplayName, a.Version, source, status)
	}
	t.Flush()
}
