package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/scottatron/maestron/internal/agents"
	maestronsync "github.com/scottatron/maestron/internal/sync"
	"github.com/scottatron/maestron/internal/output"
)

var (
	syncDryRun      bool
	syncIntegration string
)

var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Sync MCP config to per-integration config files",
	RunE:  runSync,
}

func init() {
	syncCmd.Flags().BoolVarP(&syncDryRun, "dry-run", "n", false, "show what would be written without writing")
	syncCmd.Flags().StringVar(&syncIntegration, "integration", "", "sync only this integration")
}

func runSync(cmd *cobra.Command, args []string) error {
	root, _, err := agents.FindAgentsConfig()
	if err != nil {
		return fmt.Errorf("finding project root: %w", err)
	}
	if root == "" {
		return fmt.Errorf("no agents.json found; run from a project directory")
	}

	results, err := maestronsync.Sync(root, syncDryRun, syncIntegration)
	if err != nil {
		return err
	}

	output.Print(results, func() {
		renderSyncTable(results)
	})
	return nil
}

func renderSyncTable(results []maestronsync.SyncResult) {
	if len(results) == 0 {
		fmt.Println("No integrations to sync.")
		return
	}

	t := output.NewTable(os.Stdout, []string{"INTEGRATION", "PATH", "STATUS"})
	for _, r := range results {
		status := "written"
		if r.Skipped {
			status = "skipped (no renderer)"
		} else if r.Err != nil {
			status = fmt.Sprintf("error: %v", r.Err)
		} else if !r.Written {
			status = "dry-run"
		}
		t.Row(r.Integration, r.Path, status)
	}
	t.Flush()
}
