package cmd

import (
	"fmt"
	"os"
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

	// Load managed skills manifest for inline annotations (best-effort).
	var manifest *manage.SkillsManifest
	if home, err := platform.HomeDir(); err == nil {
		manifest, _ = manage.LoadManifest(home)
	}

	output.Print(skills, func() {
		renderSkillsTable(skills, manifest)
	})
	return nil
}

func renderSkillsTable(skills []discover.SkillInfo, manifest *manage.SkillsManifest) {
	if len(skills) == 0 {
		fmt.Println("No skills found.")
		return
	}

	if !term.IsTerminal(os.Stdout.Fd()) {
		renderSkillsPlain(skills)
		return
	}

	renderSkillsGrouped(skills, manifest, ttyWidth())
}

func renderSkillsGrouped(skills []discover.SkillInfo, manifest *manage.SkillsManifest, width int) {
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
			if manifest != nil {
				if record, ok := manifest.Skills[s.Name]; ok {
					fmt.Printf("  %s  %s\n", pad, renderManagedMeta(record))
				}
			}
			switch s.ManagedRelation {
			case discover.ManagedRelationMatches:
				fmt.Printf("  %s  %s\n", pad, styleAlias.Render("✓ matches managed version"))
			case discover.ManagedRelationDiffers:
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

// renderManagedMeta formats the managed-skill metadata as inline key: value pairs.
func renderManagedMeta(r *manage.SkillRecord) string {
	kv := func(key, val string) string {
		return styleManagedKey.Render(key+":") + " " + styleManagedVal.Render(val)
	}

	var parts []string
	if r.Source.Type == "git" {
		parts = append(parts, kv("source", stripURLScheme(r.Source.URL)))
		if r.Source.Ref != "" {
			parts = append(parts, kv("ref", r.Source.Ref))
		}
		if sha := shortSHA(r.Source.ResolvedSHA); sha != "" {
			parts = append(parts, kv("sha", sha))
		}
	} else {
		sourceVal := r.Source.Path
		if r.Source.Hostname != "" {
			sourceVal = r.Source.Hostname + ":" + r.Source.Path
		}
		parts = append(parts, kv("source", sourceVal))
	}
	parts = append(parts, kv("updated", r.UpdatedAt.Format("2006-01-02")))

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
