package cmd

import (
	"fmt"
	"os"

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

	if len(args) > 0 {
		record, ok := manifest.Skills[args[0]]
		if !ok {
			return fmt.Errorf("skill %q not found in manifest", args[0])
		}
		printSkillDetail(home, record, statusCheck)
		return nil
	}

	t := output.NewTable(os.Stdout, []string{"NAME", "SOURCE", "VERSION", "UPDATED", "STATUS"})
	for _, record := range manifest.Skills {
		source := record.Source.URL
		if record.Source.Type == "local" {
			source = record.Source.Path
		}
		version := shortSHA(record.Source.ResolvedSHA)
		if record.Source.Type == "local" {
			version = record.Source.Hostname
		}
		updated := record.UpdatedAt.Format("2006-01-02")

		status := statusForRecord(record, statusCheck)
		t.Row(record.Name, tildeSubstPath(home, source), version, updated, status)
	}
	t.Flush()
	return nil
}

func statusForRecord(record *manage.SkillRecord, check bool) string {
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

	// Without --check, do quick local-only checks
	if record.Source.Type == "local" {
		hostname, _ := os.Hostname()
		if record.Source.Hostname != hostname {
			return "foreign host"
		}
		if _, err := os.Stat(record.Source.Path); os.IsNotExist(err) {
			return "source missing"
		}
	}
	return "managed"
}

func printSkillDetail(home string, record *manage.SkillRecord, check bool) {
	fmt.Printf("Name:      %s\n", record.Name)
	fmt.Printf("Path:      %s\n", tildeSubstPath(home, record.InstallPath))
	fmt.Printf("Source:    %s\n", record.Source.Type)
	if record.Source.Type == "git" {
		fmt.Printf("URL:       %s\n", record.Source.URL)
		fmt.Printf("Ref:       %s\n", record.Source.Ref)
		fmt.Printf("SHA:       %s\n", record.Source.ResolvedSHA)
		if record.Source.Subpath != "" {
			fmt.Printf("Subpath:   %s\n", record.Source.Subpath)
		}
	} else {
		fmt.Printf("SrcPath:   %s\n", record.Source.Path)
		fmt.Printf("Hostname:  %s\n", record.Source.Hostname)
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
	}
}
