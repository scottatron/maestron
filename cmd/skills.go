package cmd

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/charmbracelet/x/term"
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

	if !term.IsTerminal(os.Stdout.Fd()) {
		renderSkillsPlain(skills)
		return
	}

	renderSkillsGrouped(skills, ttyWidth())
}

func renderSkillsGrouped(skills []discover.SkillInfo, width int) {
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
		count := fmt.Sprintf("(%d)", len(g.skills))
		fmt.Printf("%s %s\n", g.source, count)
		for _, s := range g.skills {
			desc := truncateRunes(s.Description, descWidth)
			fmt.Printf("  %-*s  %s\n", maxName, s.Name, desc)
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

func truncateRunes(s string, max int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max-1]) + "…"
}
