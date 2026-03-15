package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/scottatron/maestron/internal/manage"
	"github.com/scottatron/maestron/internal/platform"
)

var keepFiles bool

var skillsRemoveCmd = &cobra.Command{
	Use:   "remove <name>",
	Short: "Remove a managed skill",
	Args:  cobra.ExactArgs(1),
	RunE:  runSkillsRemove,
}

func init() {
	skillsRemoveCmd.Flags().BoolVar(&keepFiles, "keep-files", false, "remove from manifest only, don't delete files")
	skillsCmd.AddCommand(skillsRemoveCmd)
}

func runSkillsRemove(cmd *cobra.Command, args []string) error {
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

	installPath := tildeSubstPath(home, record.InstallPath)
	fmt.Printf("Remove %s from %s? [y/N] ", name, installPath)

	reader := bufio.NewReader(os.Stdin)
	response, err := reader.ReadString('\n')
	if err != nil {
		return err
	}
	response = strings.TrimSpace(strings.ToLower(response))
	if response != "y" && response != "yes" {
		fmt.Println("Aborted.")
		return nil
	}

	if !keepFiles {
		if err := os.RemoveAll(record.InstallPath); err != nil {
			return fmt.Errorf("remove skill files: %w", err)
		}
	}

	delete(manifest.Skills, name)
	if err := manifest.Save(home); err != nil {
		return fmt.Errorf("save manifest: %w", err)
	}

	fmt.Printf("✓ Removed %s\n", name)
	return nil
}
