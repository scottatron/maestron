package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/scottatron/maestron/internal/manage"
	"github.com/scottatron/maestron/internal/platform"
)

var skillsDiffCmd = &cobra.Command{
	Use:   "diff <name>",
	Short: "Show diff of installed skill against upstream",
	Args:  cobra.ExactArgs(1),
	RunE:  runSkillsDiff,
}

func init() {
	skillsCmd.AddCommand(skillsDiffCmd)
}

func runSkillsDiff(cmd *cobra.Command, args []string) error {
	name := args[0]
	home, err := platform.HomeDir()
	if err != nil {
		return err
	}

	manifest, err := manage.LoadManifest(home)
	if err != nil {
		return fmt.Errorf("load manifest: %w", err)
	}

	record, ok := manifest.Skills[name]
	if !ok {
		return fmt.Errorf("skill %q not found in manifest", name)
	}

	ref := record.Source.Ref
	if ref == "" {
		ref = "HEAD"
	}
	sha := shortSHA(record.Source.ResolvedSHA)
	if sha != "" {
		fmt.Printf("Diffing %s against upstream (%s @ %s)...\n\n", name, ref, sha)
	} else {
		src := record.Source.Path
		fmt.Printf("Diffing %s against upstream (%s)...\n\n", name, src)
	}

	diff, err := manage.DiffSkill(record)
	if err != nil {
		return err
	}

	if strings.TrimSpace(diff) == "" {
		fmt.Println("No differences found.")
		return nil
	}

	fmt.Print(colorDiff(diff))
	return nil
}
