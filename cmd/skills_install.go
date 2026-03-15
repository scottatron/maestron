package cmd

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/x/term"
	"github.com/spf13/cobra"

	"github.com/scottatron/maestron/internal/manage"
	"github.com/scottatron/maestron/internal/platform"
)

var (
	installRef     string
	installSubpath string
	installName    string
)

var skillsInstallCmd = &cobra.Command{
	Use:   "install <url-or-path>",
	Short: "Install a skill from a git repo or local path",
	Args:  cobra.ExactArgs(1),
	RunE:  runSkillsInstall,
}

func init() {
	skillsInstallCmd.Flags().StringVar(&installRef, "ref", "", "git branch, tag, or SHA (default: repo default branch)")
	skillsInstallCmd.Flags().StringVar(&installSubpath, "path", "", "subpath within git repo (skips multi-skill detection)")
	skillsInstallCmd.Flags().StringVar(&installName, "name", "", "override skill name")
	skillsCmd.AddCommand(skillsInstallCmd)
}

func runSkillsInstall(cmd *cobra.Command, args []string) error {
	src := args[0]
	home, err := platform.HomeDir()
	if err != nil {
		return err
	}

	manifest, err := manage.LoadManifest(home)
	if err != nil {
		return fmt.Errorf("load manifest: %w", err)
	}

	if isLocalPath(src) {
		return installLocal(home, manifest, src)
	}
	return installGit(home, manifest, src)
}

func installLocal(home string, manifest *manage.SkillsManifest, src string) error {
	if strings.HasPrefix(src, "~/") {
		src = filepath.Join(home, src[2:])
	}
	name := installName
	if name == "" {
		name = filepath.Base(src)
	}
	dest := filepath.Join(home, ".agents", "skills", name)

	if _, err := os.Stat(dest); err == nil {
		source := manage.SkillSource{Type: "local", Path: src}
		if diff, err := manage.PreviewInstall(dest, source); err != nil {
			fmt.Printf("Warning: could not check for differences: %v\n", err)
		} else if diff != "" {
			fmt.Println()
			fmt.Print(colorDiff(diff))
			fmt.Printf("\n%s already exists and differs from upstream. Overwrite? [y/N] ", name)
			if !readYN() {
				fmt.Println("Aborted.")
				return nil
			}
		}
	}

	fmt.Printf("Installing %s from local path %s...\n", name, src)
	record, err := manage.InstallFromLocal(home, src, installName)
	if err != nil {
		return err
	}
	return saveAndPrint(home, manifest, record)
}

func installGit(home string, manifest *manage.SkillsManifest, url string) error {
	ref := installRef
	refLabel := ref
	if refLabel == "" {
		refLabel = "default branch"
	}

	// If an explicit subpath was given, skip scanning and install directly.
	if installSubpath != "" {
		name := installName
		if name == "" {
			name = filepath.Base(installSubpath)
		}
		fmt.Printf("Installing %s from %s@%s...\n", name, url, refLabel)
		record, err := manage.InstallFromGit(home, url, ref, installSubpath, installName)
		if err != nil {
			return err
		}
		return saveAndPrint(home, manifest, record)
	}

	// Clone once, scan for skills, prompt if multiple.
	fmt.Printf("Cloning %s...\n", url)
	stagedir, resolvedSHA, cleanup, err := manage.StageGitClone(url, ref)
	if err != nil {
		return err
	}
	defer cleanup()

	candidates, err := manage.ScanSkills(stagedir, url)
	if err != nil {
		return fmt.Errorf("scan skills: %w", err)
	}
	if len(candidates) == 0 {
		return fmt.Errorf("no SKILL.md files found in %s", url)
	}

	// Determine which candidates to install.
	selected := candidates
	if len(candidates) > 1 {
		selected, err = promptSkillSelect(candidates, url)
		if err != nil {
			return err
		}
		if len(selected) == 0 {
			fmt.Println("No skills selected.")
			return nil
		}
	}

	var lastErr error
	for _, c := range selected {
		name := c.Name
		if len(selected) == 1 && installName != "" {
			name = installName
		}
		dest := filepath.Join(home, ".agents", "skills", name)

		// Conflict check against the staged tree (no extra clone needed).
		if _, err := os.Stat(dest); err == nil {
			diff, err := manage.DiffStagedSKILL(dest, stagedir, c.Subpath)
			if err != nil {
				fmt.Printf("Warning: could not check for differences in %s: %v\n", name, err)
			} else if diff != "" {
				fmt.Println()
				fmt.Print(colorDiff(diff))
				fmt.Printf("\n%s already exists and differs from upstream. Overwrite? [y/N] ", name)
				if !readYN() {
					fmt.Printf("Skipping %s.\n", name)
					continue
				}
			}
		}

		record, err := manage.InstallFromStaged(home, stagedir, c.Subpath, name, url, ref, resolvedSHA)
		if err != nil {
			fmt.Printf("✗ Failed to install %s: %v\n", name, err)
			lastErr = err
			continue
		}
		manifest.Skills[record.Name] = record
		fmt.Printf("✓ Installed %s to %s\n", record.Name, tildeSubstPath(home, record.InstallPath))
		fmt.Printf("  SHA: %s  Content: %s\n", shortSHA(record.Source.ResolvedSHA), record.ContentHash)
	}

	if err := manifest.Save(home); err != nil {
		return fmt.Errorf("save manifest: %w", err)
	}
	return lastErr
}

// promptSkillSelect shows a multi-select list of skill candidates and returns
// the ones the user chose. Falls back to a numbered text prompt when not on a TTY.
func promptSkillSelect(candidates []manage.SkillCandidate, repoURL string) ([]manage.SkillCandidate, error) {
	repoName := strings.TrimSuffix(filepath.Base(repoURL), ".git")

	if !term.IsTerminal(os.Stdin.Fd()) {
		return promptSkillSelectPlain(candidates)
	}

	options := make([]huh.Option[int], len(candidates))
	for i, c := range candidates {
		label := c.Name
		if c.Subpath != "" {
			label = fmt.Sprintf("%-30s %s", c.Name, styleDesc.Render(c.Subpath))
		}
		options[i] = huh.NewOption(label, i)
	}

	var chosen []int
	err := huh.NewForm(
		huh.NewGroup(
			huh.NewMultiSelect[int]().
				Title(fmt.Sprintf("Select skills to install from %s", repoName)).
				Options(options...).
				Value(&chosen),
		),
	).Run()
	if err != nil {
		return nil, err
	}

	result := make([]manage.SkillCandidate, len(chosen))
	for i, idx := range chosen {
		result[i] = candidates[idx]
	}
	return result, nil
}

// promptSkillSelectPlain handles skill selection for non-TTY environments.
func promptSkillSelectPlain(candidates []manage.SkillCandidate) ([]manage.SkillCandidate, error) {
	fmt.Println("Multiple skills found. Enter numbers to install (comma-separated), or 'all':")
	for i, c := range candidates {
		if c.Subpath != "" {
			fmt.Printf("  %d) %s (%s)\n", i+1, c.Name, c.Subpath)
		} else {
			fmt.Printf("  %d) %s\n", i+1, c.Name)
		}
	}
	fmt.Print("> ")

	reader := bufio.NewReader(os.Stdin)
	line, err := reader.ReadString('\n')
	if err != nil {
		return nil, err
	}
	line = strings.TrimSpace(line)

	if strings.ToLower(line) == "all" {
		return candidates, nil
	}

	var result []manage.SkillCandidate
	for _, part := range strings.Split(line, ",") {
		part = strings.TrimSpace(part)
		var idx int
		if _, err := fmt.Sscanf(part, "%d", &idx); err != nil || idx < 1 || idx > len(candidates) {
			fmt.Printf("Ignoring invalid selection: %q\n", part)
			continue
		}
		result = append(result, candidates[idx-1])
	}
	return result, nil
}

func saveAndPrint(home string, manifest *manage.SkillsManifest, record *manage.SkillRecord) error {
	manifest.Skills[record.Name] = record
	if err := manifest.Save(home); err != nil {
		return fmt.Errorf("save manifest: %w", err)
	}
	fmt.Printf("✓ Installed %s to %s\n", record.Name, tildeSubstPath(home, record.InstallPath))
	if record.Source.Type == "git" {
		fmt.Printf("  SHA: %s  Content: %s\n", shortSHA(record.Source.ResolvedSHA), record.ContentHash)
	} else {
		fmt.Printf("  Content: %s\n", record.ContentHash)
	}
	return nil
}

// readYN reads a y/N confirmation from stdin.
func readYN() bool {
	reader := bufio.NewReader(os.Stdin)
	response, err := reader.ReadString('\n')
	if err != nil {
		return false
	}
	r := strings.TrimSpace(strings.ToLower(response))
	return r == "y" || r == "yes"
}
