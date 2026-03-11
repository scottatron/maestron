package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/scottatron/maestron/internal/agents"
	"github.com/scottatron/maestron/internal/doctor"
	"github.com/scottatron/maestron/internal/output"
)

var doctorFix bool

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Validate project setup",
	RunE:  runDoctor,
}

func init() {
	doctorCmd.Flags().BoolVar(&doctorFix, "fix", false, "auto-apply fixable issues")
}

func runDoctor(cmd *cobra.Command, args []string) error {
	root, _, err := agents.FindAgentsConfig()
	if err != nil {
		return fmt.Errorf("finding project root: %w", err)
	}
	if root == "" {
		return fmt.Errorf("no agents.json found; run from a project directory")
	}

	issues, err := doctor.Check(root)
	if err != nil {
		return err
	}

	output.Print(issues, func() {
		renderDoctorOutput(issues, root)
	})
	return nil
}

func renderDoctorOutput(issues []doctor.Issue, root string) {
	if len(issues) == 0 {
		fmt.Println("✓ No issues found.")
		return
	}

	errors, warnings, infos := 0, 0, 0
	for _, issue := range issues {
		switch issue.Severity {
		case "error":
			errors++
		case "warning":
			warnings++
		case "info":
			infos++
		}
	}

	t := output.NewTable(os.Stdout, []string{"SEVERITY", "MESSAGE", "FIX"})
	for _, issue := range issues {
		indicator := "✓"
		switch issue.Severity {
		case "error":
			indicator = "✗"
		case "warning":
			indicator = "⚠"
		case "info":
			indicator = "ℹ"
		}
		severity := indicator + " " + issue.Severity
		fix := issue.Fix
		if len(fix) > 60 {
			fix = fix[:57] + "..."
		}
		t.Row(severity, issue.Message, fix)
	}
	t.Flush()

	fmt.Printf("\n%d error(s), %d warning(s), %d info(s)\n", errors, warnings, infos)
}
