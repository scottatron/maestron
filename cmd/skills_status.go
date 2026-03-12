package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/scottatron/maestron/internal/manage"
	"github.com/scottatron/maestron/internal/output"
	"github.com/scottatron/maestron/internal/platform"
)

var statusCheck bool

var skillsStatusCmd = &cobra.Command{
	Use:   "status [name]",
	Short: "Show status of managed skills",
	RunE:  runSkillsStatus,
}

func init() {
	skillsStatusCmd.Flags().BoolVar(&statusCheck, "check", false, "check for upstream updates (makes network calls)")
	skillsCmd.AddCommand(skillsStatusCmd)
}

func runSkillsStatus(cmd *cobra.Command, args []string) error {
	home, err := platform.HomeDir()
	if err != nil {
		return err
	}

	manifest, err := manage.LoadManifest(home)
	if err != nil {
		return fmt.Errorf("load manifest: %w", err)
	}

	if len(manifest.Skills) == 0 {
		fmt.Println("No managed skills. Use 'maestron skills install' to install skills.")
		return nil
	}

	updateCache, _ := manage.LoadUpdateCache(home) // best-effort

	if len(args) > 0 {
		record, ok := manifest.Skills[args[0]]
		if !ok {
			return fmt.Errorf("skill %q not found in manifest", args[0])
		}
		var uce *manage.UpdateCheckEntry
		if updateCache != nil {
			uce = updateCache.Skills[args[0]]
		}
		printSkillDetail(home, record, statusCheck, uce)
		return nil
	}

	t := output.NewTable(os.Stdout, []string{"NAME", "SOURCE", "SHA", "HASH", "UPDATED", "STATUS"})
	for _, record := range manifest.Skills {
		source := recordSource(home, record)
		sha := shortSHA(record.Source.ResolvedSHA) // empty for local
		hash := shortContentHash(record.ContentHash)
		updated := record.UpdatedAt.Format("2006-01-02")
		var uce *manage.UpdateCheckEntry
		if updateCache != nil {
			uce = updateCache.Skills[record.Name]
		}
		status := statusForRecord(record, statusCheck, uce)
		t.Row(record.Name, source, sha, hash, updated, status)
	}
	t.Flush()
	return nil
}

// recordSource returns the formatted source string for the status table,
// matching the style used in the skills list managed metadata.
func recordSource(home string, record *manage.SkillRecord) string {
	if record.Source.Type == "git" {
		return stripURLScheme(record.Source.URL)
	}
	srcPath := tildeSubstPath(home, record.Source.Path)
	if record.Source.Hostname != "" {
		return record.Source.Hostname + ":" + srcPath
	}
	return srcPath
}

// shortContentHash returns the first 8 hex characters of a "sha256:..." hash.
func shortContentHash(hash string) string {
	if after, ok := strings.CutPrefix(hash, "sha256:"); ok {
		if len(after) > 8 {
			return after[:8]
		}
		return after
	}
	return hash
}

func statusForRecord(record *manage.SkillRecord, check bool, uce *manage.UpdateCheckEntry) string {
	if check {
		us := manage.CheckUpdate(record)
		if us.Err != nil {
			return "error: " + us.Err.Error()
		}
		if us.HasUpdate {
			return "update available"
		}
		return "up to date"
	}

	// Without --check, do quick local-only checks then fall back to cached result.
	if record.Source.Type == "local" {
		hostname, _ := os.Hostname()
		if record.Source.Hostname != hostname {
			return "foreign host"
		}
		if _, err := os.Stat(record.Source.Path); os.IsNotExist(err) {
			return "source missing"
		}
	}

	if uce != nil {
		if uce.ErrMsg != "" {
			return "check error"
		}
		if uce.HasUpdate {
			return "update available"
		}
		return "up to date"
	}

	return "managed"
}

func printSkillDetail(home string, record *manage.SkillRecord, check bool, uce *manage.UpdateCheckEntry) {
	fmt.Printf("Name:      %s\n", record.Name)
	fmt.Printf("Path:      %s\n", tildeSubstPath(home, record.InstallPath))
	fmt.Printf("Source:    %s\n", recordSource(home, record))
	if record.Source.Type == "git" {
		fmt.Printf("Ref:       %s\n", record.Source.Ref)
		fmt.Printf("SHA:       %s\n", record.Source.ResolvedSHA)
		if record.Source.Subpath != "" {
			fmt.Printf("Subpath:   %s\n", record.Source.Subpath)
		}
	}
	fmt.Printf("Hash:      %s\n", record.ContentHash)
	fmt.Printf("Installed: %s\n", record.InstalledAt.Format("2006-01-02T15:04:05Z"))
	fmt.Printf("Updated:   %s\n", record.UpdatedAt.Format("2006-01-02T15:04:05Z"))

	if check {
		us := manage.CheckUpdate(record)
		if us.Err != nil {
			fmt.Printf("Status:    error: %s\n", us.Err)
		} else if us.HasUpdate {
			fmt.Printf("Status:    update available (%s)\n", shortSHA(us.RemoteSHA))
		} else {
			fmt.Printf("Status:    up to date\n")
		}
	} else if uce != nil {
		if uce.ErrMsg != "" {
			fmt.Printf("Status:    check error: %s (checked %s)\n", uce.ErrMsg, uce.CheckedAt.Format("2006-01-02"))
		} else if uce.HasUpdate {
			if uce.RemoteSHA != "" {
				fmt.Printf("Status:    update available (%s → %s, checked %s)\n", shortSHA(record.Source.ResolvedSHA), shortSHA(uce.RemoteSHA), uce.CheckedAt.Format("2006-01-02"))
			} else {
				fmt.Printf("Status:    update available (checked %s)\n", uce.CheckedAt.Format("2006-01-02"))
			}
		} else {
			fmt.Printf("Status:    up to date (checked %s)\n", uce.CheckedAt.Format("2006-01-02"))
		}
	}
}
