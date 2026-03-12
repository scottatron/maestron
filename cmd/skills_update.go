package cmd

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/scottatron/maestron/internal/manage"
	"github.com/scottatron/maestron/internal/platform"
)

var (
	updateAll   bool
	updateCheck bool
)

var skillsUpdateCmd = &cobra.Command{
	Use:   "update [name]",
	Short: "Update a managed skill from its source",
	RunE:  runSkillsUpdate,
}

func init() {
	skillsUpdateCmd.Flags().BoolVar(&updateAll, "all", false, "update all managed skills")
	skillsUpdateCmd.Flags().BoolVar(&updateCheck, "check", false, "check for updates without applying them (makes network calls)")
	skillsCmd.AddCommand(skillsUpdateCmd)
}

func runSkillsUpdate(cmd *cobra.Command, args []string) error {
	home, err := platform.HomeDir()
	if err != nil {
		return err
	}

	manifest, err := manage.LoadManifest(home)
	if err != nil {
		return fmt.Errorf("load manifest: %w", err)
	}

	if len(args) == 0 && !updateAll && !updateCheck {
		return fmt.Errorf("specify a skill name, use --all, or use --check to check all skills")
	}

	var names []string
	if updateAll || updateCheck {
		for name := range manifest.Skills {
			names = append(names, name)
		}
	} else {
		names = []string{args[0]}
	}

	if updateCheck {
		return runSkillsUpdateCheck(names, manifest)
	}

	var lastErr error
	for _, name := range names {
		record, ok := manifest.Skills[name]
		if !ok {
			fmt.Printf("✗ Skill %q not found in manifest\n", name)
			continue
		}

		oldSHA := shortSHA(record.Source.ResolvedSHA)

		src := record.Source.URL
		if record.Source.Type == "local" {
			src = record.Source.Path
		}
		label := src
		if record.Source.Ref != "" {
			label = src + "@" + record.Source.Ref
		}
		fmt.Printf("Updating %s from %s...\n", name, label)

		newRecord, err := manage.UpdateSkill(home, record)
		if err != nil {
			fmt.Printf("✗ Failed to update %s: %v\n", name, err)
			lastErr = err
			continue
		}

		manifest.Skills[name] = newRecord
		newSHA := shortSHA(newRecord.Source.ResolvedSHA)
		if oldSHA != "" && newSHA != "" && oldSHA != newSHA {
			fmt.Printf("✓ Updated %s (%s → %s)\n", name, oldSHA, newSHA)
		} else {
			fmt.Printf("✓ Updated %s\n", name)
		}
	}

	if err := manifest.Save(home); err != nil {
		return fmt.Errorf("save manifest: %w", err)
	}
	return lastErr
}

func runSkillsUpdateCheck(names []string, manifest *manage.SkillsManifest) error {
	home, err := platform.HomeDir()
	if err != nil {
		return err
	}

	cache, _ := manage.LoadUpdateCache(home) // best-effort

	var lastErr error
	for _, name := range names {
		record, ok := manifest.Skills[name]
		if !ok {
			fmt.Printf("✗ %s: not found in manifest\n", name)
			continue
		}
		us := manage.CheckUpdate(record)

		entry := &manage.UpdateCheckEntry{
			HasUpdate: us.HasUpdate,
			RemoteSHA: us.RemoteSHA,
			CheckedAt: time.Now().UTC(),
		}
		if us.Err != nil {
			entry.ErrMsg = us.Err.Error()
		}
		cache.Skills[name] = entry

		if us.Err != nil {
			fmt.Printf("✗ %s: %v\n", name, us.Err)
			lastErr = us.Err
			continue
		}
		if us.HasUpdate {
			if us.RemoteSHA != "" {
				fmt.Printf("↑ %s: update available (%s → %s)\n", name, shortSHA(record.Source.ResolvedSHA), shortSHA(us.RemoteSHA))
			} else {
				fmt.Printf("↑ %s: update available\n", name)
			}
		} else {
			fmt.Printf("✓ %s: up to date\n", name)
		}
	}

	// Persist results (best-effort; don't mask the real error)
	cache.Save(home) //nolint:errcheck
	return lastErr
}
