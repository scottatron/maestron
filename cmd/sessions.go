package cmd

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/scottatron/maestron/internal/discover"
	"github.com/scottatron/maestron/internal/output"
)

var (
	sessionsProject string
	sessionsAgent   string
	sessionsLimit   int
	sessionsSince   string
)

var sessionsCmd = &cobra.Command{
	Use:   "sessions",
	Short: "List sessions grouped by project",
	RunE:  runSessions,
}

func init() {
	sessionsCmd.Flags().StringVar(&sessionsProject, "project", "", "filter to one project path")
	sessionsCmd.Flags().StringVar(&sessionsAgent, "agent", "", "filter to one agent")
	sessionsCmd.Flags().IntVar(&sessionsLimit, "limit", 10, "max sessions per project")
	sessionsCmd.Flags().StringVar(&sessionsSince, "since", "", "only sessions modified within duration (e.g. 7d, 24h)")
}

func runSessions(cmd *cobra.Command, args []string) error {
	filter := discover.SessionFilter{
		Project: sessionsProject,
		Agent:   sessionsAgent,
		Limit:   sessionsLimit,
	}

	if sessionsSince != "" {
		d, err := parseDuration(sessionsSince)
		if err != nil {
			return fmt.Errorf("invalid --since value %q: %w", sessionsSince, err)
		}
		filter.Since = d
	}

	groups, err := discover.ListSessions(filter)
	if err != nil {
		return err
	}

	output.Print(groups, func() {
		renderSessionsTable(groups)
	})
	return nil
}

func renderSessionsTable(groups []discover.SessionGroup) {
	if len(groups) == 0 {
		fmt.Println("No sessions found.")
		return
	}

	for _, g := range groups {
		fmt.Printf("\n%s\n", g.ProjectPath)
		t := output.NewTable(os.Stdout, []string{"  ID", "AGENT", "MODEL", "TITLE", "MODIFIED"})
		for _, s := range g.Sessions {
			model := s.Model
			if model == "" {
				model = "-"
			}
			title := s.Title
			if title == "" {
				title = s.SessionID
			}
			modified := "-"
			if !s.ModifiedAt.IsZero() {
				modified = s.ModifiedAt.Format("2006-01-02 15:04")
			}
			id := s.SessionID
			if len(id) > 8 {
				id = id[:8]
			}
			t.Row("  "+id, s.Agent, model, title, modified)
		}
		t.Flush()
	}
}

// parseDuration parses durations like "7d", "24h", "30m".
func parseDuration(s string) (time.Duration, error) {
	if len(s) > 1 && s[len(s)-1] == 'd' {
		days, err := time.ParseDuration(s[:len(s)-1] + "h")
		if err != nil {
			return 0, err
		}
		return days * 24, nil
	}
	return time.ParseDuration(s)
}
