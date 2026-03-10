package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/scottatron/maestron/internal/discover"
	"github.com/scottatron/maestron/internal/output"
)

var skillsSource string

var skillsCmd = &cobra.Command{
	Use:   "skills",
	Short: "List installed skills",
	RunE:  runSkills,
}

func init() {
	skillsCmd.Flags().StringVar(&skillsSource, "source", "all", `filter by source: "squad", "claude", or "all"`)
}

func runSkills(cmd *cobra.Command, args []string) error {
	skills, err := discover.ListSkills()
	if err != nil {
		return err
	}

	// Apply source filter
	if skillsSource != "all" && skillsSource != "" {
		var filtered []discover.SkillInfo
		for _, s := range skills {
			switch skillsSource {
			case "squad":
				if s.Source == "squad" {
					filtered = append(filtered, s)
				}
			case "claude":
				if s.Source == "claude-native" {
					filtered = append(filtered, s)
				}
			}
		}
		skills = filtered
	}

	output.Print(skills, func() {
		renderSkillsTable(skills)
	})
	return nil
}

func renderSkillsTable(skills []discover.SkillInfo) {
	if len(skills) == 0 {
		fmt.Println("No skills found.")
		return
	}
	t := output.NewTable(os.Stdout, []string{"NAME", "SOURCE", "DESCRIPTION"})
	for _, s := range skills {
		desc := s.Description
		if len(desc) > 80 {
			desc = desc[:77] + "..."
		}
		t.Row(s.Name, s.Source, desc)
	}
	t.Flush()
}
