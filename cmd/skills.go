package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/term"
	"github.com/spf13/cobra"

	"github.com/scottatron/maestron/internal/discover"
	"github.com/scottatron/maestron/internal/manage"
	"github.com/scottatron/maestron/internal/output"
	"github.com/scottatron/maestron/internal/platform"
)

var (
	styleSource      = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.AdaptiveColor{Light: "#5B21B6", Dark: "#A78BFA"})
	styleCount       = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#6B7280", Dark: "#6B7280"})
	styleName        = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#111827", Dark: "#F9FAFB"})
	styleDesc        = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#6B7280", Dark: "#9CA3AF"})
	styleManagedKey   = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#0E7490", Dark: "#22D3EE"})
	styleManagedVal   = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#374151", Dark: "#A5F3FC"})
	styleAlias        = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#6B7280", Dark: "#6B7280"})
	styleConflict     = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#B45309", Dark: "#FCD34D"})
)

var skillsSource string

var skillsCmd = &cobra.Command{
	Use:   "skills",
	Short: "List installed skills",
	RunE:  runSkills,
}

func init() {
	skillsCmd.Flags().StringVar(&skillsSource, "source", "", `filter by source path (e.g. "claude", "codex", "superpowers")`)
}

func runSkills(cmd *cobra.Command, args []string) error {
	skills, err := discover.ListSkills()
	if err != nil {
		return err
	}

	if skillsSource != "" {
		var filtered []discover.SkillInfo
		for _, s := range skills {
			if strings.Contains(s.Source, skillsSource) {
				filtered = append(filtered, s)
			}
		}
		skills = filtered
	}

	// Load managed skills manifest and update cache for inline annotations (best-effort).
	var manifest *manage.SkillsManifest
	var updateCache *manage.UpdateCheckCache
	home, _ := platform.HomeDir()
	if home != "" {
		manifest, _ = manage.LoadManifest(home)
		updateCache, _ = manage.LoadUpdateCache(home)
	}

	output.Print(skills, func() {
		renderSkillsTable(skills, manifest, updateCache, home)
	})
	return nil
}

func renderSkillsTable(skills []discover.SkillInfo, manifest *manage.SkillsManifest, updateCache *manage.UpdateCheckCache, home string) {
	if len(skills) == 0 {
		fmt.Println("No skills found.")
		return
	}

	if !term.IsTerminal(os.Stdout.Fd()) {
		renderSkillsPlain(skills)
		return
	}

	renderSkillsGrouped(skills, manifest, updateCache, home, ttyWidth())
}

// metaLayout holds the pre-computed column widths for aligned metadata rendering.
type metaLayout struct {
	sourceWidth int // max visible width of source value across all managed records
	shaWidth    int // 7 if any git record has a SHA, else 0
	refWidth    int // max visible width of ref value across git records
}

// computeMetaLayout scans all managed skills to determine column widths for alignment.
func computeMetaLayout(skills []discover.SkillInfo, manifest *manage.SkillsManifest, home string) metaLayout {
	var layout metaLayout
	if manifest == nil {
		return layout
	}
	for _, s := range skills {
		if s.ManagedRelation != discover.ManagedRelationIs {
			continue
		}
		record := manifest.Skills[s.Name]
		if record == nil {
			continue
		}
		sv := managedSourceVal(record, home)
		if len(sv) > layout.sourceWidth {
			layout.sourceWidth = len(sv)
		}
		if record.Source.Type == "git" {
			if shortSHA(record.Source.ResolvedSHA) != "" {
				layout.shaWidth = 7
			}
			if n := len(record.Source.Ref); n > layout.refWidth {
				layout.refWidth = n
			}
		}
	}
	return layout
}

// managedSourceVal returns the raw (unstyled) source value string for a record.
func managedSourceVal(r *manage.SkillRecord, home string) string {
	if r.Source.Type == "git" {
		return stripURLScheme(r.Source.URL)
	}
	srcPath := tildeSubstPath(home, r.Source.Path)
	if r.Source.Hostname != "" {
		return r.Source.Hostname + ":" + srcPath
	}
	return srcPath
}

func renderSkillsGrouped(skills []discover.SkillInfo, manifest *manage.SkillsManifest, updateCache *manage.UpdateCheckCache, home string, width int) {
	// Calculate name column width from the widest name, capped at 40.
	maxName := 0
	for _, s := range skills {
		if n := len(s.Name); n > maxName {
			maxName = n
		}
	}
	if maxName > 40 {
		maxName = 40
	}

	// Description fills the remaining width: indent(2) + name + gap(2).
	const indent = 2
	const gap = 2
	descWidth := width - indent - maxName - gap
	if descWidth < 20 {
		descWidth = 20
	}

	// Pre-compute metadata column widths for cross-group alignment.
	layout := computeMetaLayout(skills, manifest, home)

	// Collect groups in order of first appearance.
	type group struct {
		source string
		skills []discover.SkillInfo
	}
	var groups []group
	index := map[string]int{}
	for _, s := range skills {
		if i, ok := index[s.Source]; ok {
			groups[i].skills = append(groups[i].skills, s)
		} else {
			index[s.Source] = len(groups)
			groups = append(groups, group{source: s.Source, skills: []discover.SkillInfo{s}})
		}
	}

	for i, g := range groups {
		if i > 0 {
			fmt.Println()
		}
		count := styleCount.Render(fmt.Sprintf("(%d)", len(g.skills)))
		fmt.Printf("%s %s\n", styleSource.Render(g.source), count)
		for _, s := range g.skills {
			desc := truncateRunes(s.Description, descWidth)
			name := styleName.Render(fmt.Sprintf("%-*s", maxName, s.Name))
			fmt.Printf("  %s  %s\n", name, styleDesc.Render(desc))
			pad := strings.Repeat(" ", maxName)

			var record *manage.SkillRecord
			if manifest != nil {
				record = manifest.Skills[s.Name]
			}

			// Determine if this occurrence is the local source the managed copy
			// was installed from.
			isSource := record != nil &&
				record.Source.Type == "local" &&
				record.Source.Path == filepath.Dir(s.Path)

			switch {
			case s.ManagedRelation == discover.ManagedRelationIs && record != nil:
				// This IS the managed copy — show its full metadata.
				var uce *manage.UpdateCheckEntry
				if updateCache != nil {
					uce = updateCache.Skills[s.Name]
				}
				fmt.Printf("  %s  %s\n", pad, renderManagedMeta(record, home, layout, uce))

			case isSource:
				// This path is where the managed copy was installed from.
				// Show a compact "source of" line with a sync indicator.
				installPath := tildeSubstPath(home, record.InstallPath)
				var syncLabel string
				if s.ManagedRelation == discover.ManagedRelationDiffers {
					syncLabel = "  " + styleConflict.Render("⚠ out of sync")
				} else {
					syncLabel = "  " + styleAlias.Render("✓ in sync")
				}
				fmt.Printf("  %s  %s%s\n", pad,
					styleManagedKey.Render("↑ source of: ")+styleManagedVal.Render(installPath),
					syncLabel)

			case s.ManagedRelation == discover.ManagedRelationMatches:
				fmt.Printf("  %s  %s\n", pad, styleAlias.Render("✓ matches managed version"))

			case s.ManagedRelation == discover.ManagedRelationDiffers:
				fmt.Printf("  %s  %s\n", pad, styleConflict.Render("⚠ differs from managed version"))
			}
		}
	}
}

func renderSkillsPlain(skills []discover.SkillInfo) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tDESCRIPTION\tSOURCE")
	for _, s := range skills {
		fmt.Fprintf(w, "%s\t%s\t%s\n", s.Name, s.Description, s.Source)
	}
	w.Flush()
}

// ttyWidth returns the terminal width, falling back to $COLUMNS then 120.
func ttyWidth() int {
	if w, _, err := term.GetSize(os.Stdout.Fd()); err == nil && w > 0 {
		return w
	}
	if cols := os.Getenv("COLUMNS"); cols != "" {
		if n, err := strconv.Atoi(cols); err == nil && n > 0 {
			return n
		}
	}
	return 120
}

// renderManagedMeta formats the managed-skill metadata as inline key: value pairs,
// padding each column to the widths in layout so fields align across all skills.
// uce is the cached update check result for this skill (may be nil).
func renderManagedMeta(r *manage.SkillRecord, home string, layout metaLayout, uce *manage.UpdateCheckEntry) string {
	kv := func(key, val string) string {
		return styleManagedKey.Render(key+":") + " " + styleManagedVal.Render(val)
	}
	spacer := func(n int) string { return strings.Repeat(" ", n) }

	var parts []string

	// source — padded to the widest source value across all records.
	sv := managedSourceVal(r, home)
	parts = append(parts, kv("source", fmt.Sprintf("%-*s", layout.sourceWidth, sv)))

	if r.Source.Type == "git" {
		if layout.refWidth > 0 {
			parts = append(parts, kv("ref", fmt.Sprintf("%-*s", layout.refWidth, r.Source.Ref)))
		}
		if layout.shaWidth > 0 {
			sha := shortSHA(r.Source.ResolvedSHA)
			if sha == "" {
				sha = spacer(layout.shaWidth)
			}
			parts = append(parts, kv("sha", sha))
		}
	} else {
		// Local: insert blank spacers so "updated:" aligns with git records.
		if layout.refWidth > 0 {
			parts = append(parts, spacer(len("ref: ")+layout.refWidth))
		}
		if layout.shaWidth > 0 {
			parts = append(parts, spacer(len("sha: ")+layout.shaWidth))
		}
	}

	parts = append(parts, kv("updated", r.UpdatedAt.Format("2006-01-02")))

	// Append cached update status if present.
	if uce != nil {
		if uce.ErrMsg != "" {
			parts = append(parts, styleConflict.Render("⚠ check error"))
		} else if uce.HasUpdate {
			indicator := "↑ update available"
			if uce.RemoteSHA != "" && r.Source.ResolvedSHA != "" {
				indicator = fmt.Sprintf("↑ update available (%s → %s)", shortSHA(r.Source.ResolvedSHA), shortSHA(uce.RemoteSHA))
			}
			parts = append(parts, styleConflict.Render(indicator))
		} else {
			parts = append(parts, styleAlias.Render("✓ up to date"))
		}
	}

	return strings.Join(parts, "  ")
}

func truncateRunes(s string, max int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max-1]) + "…"
}
