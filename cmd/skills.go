package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/scottatron/maestron/internal/discover"
	"github.com/scottatron/maestron/internal/output"
	"github.com/scottatron/maestron/internal/platform"
)

var skillsSource string

var skillsCmd = &cobra.Command{
	Use:   "skills",
	Short: "List installed skills",
	RunE:  runSkills,
}

func init() {
	skillsCmd.Flags().StringVar(&skillsSource, "source", "all", `filter by source: "project", "claude", or "all"`)
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
			case "project":
				if s.Source == "project" {
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
	t := output.NewTable(os.Stdout, []string{"NAME", "DESCRIPTION", "SOURCE"})
	for _, s := range skills {
		desc := s.Description
		if len(desc) > 60 {
			desc = desc[:57] + "..."
		}
		t.Row(s.Name, desc, formatSkillSource(s.Path))
	}
	t.Flush()
}

// formatSkillSource returns a short human-readable source label for a skill path:
//   - ~/.agents/skills/foo/SKILL.md        → ~/.agents/skills/foo
//   - {cacheDir}/registry/plugin/ver/...   → plugin@registry
//   - anything else                        → tilde-abbreviated parent dir
func formatSkillSource(path string) string {
	home, _ := platform.HomeDir()

	// Claude plugin cache: {home}/.claude/plugins/cache/{registry}/{plugin}/{version}/skills/...
	if home != "" {
		cacheDir := filepath.Join(home, ".claude", "plugins", "cache")
		if strings.HasPrefix(path, cacheDir+string(filepath.Separator)) {
			rel := path[len(cacheDir)+1:]
			parts := strings.SplitN(rel, string(filepath.Separator), 3)
			if len(parts) >= 2 {
				registry, plugin := parts[0], parts[1]
				return plugin + "@" + registry
			}
		}
	}

	// For everything else: show the skill's parent directory (tilde-abbreviated)
	return tildeAbbrev(filepath.Dir(path))
}
