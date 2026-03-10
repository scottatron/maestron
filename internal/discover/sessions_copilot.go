package discover

import (
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/scottatron/maestron/internal/platform"
)

type copilotWorkspace struct {
	ID           string `yaml:"id"`
	CWD          string `yaml:"cwd"`
	Summary      string `yaml:"summary"`
	SummaryCount int    `yaml:"summary_count"`
	CreatedAt    string `yaml:"created_at"`
	UpdatedAt    string `yaml:"updated_at"`
	GitRoot      string `yaml:"git_root"`
	Repository   string `yaml:"repository"`
	Branch       string `yaml:"branch"`
}

// listCopilotSessions returns all discovered GitHub Copilot sessions.
func listCopilotSessions() ([]SessionInfo, error) {
	home, err := platform.HomeDir()
	if err != nil {
		return nil, err
	}

	sessionStateDir := filepath.Join(home, ".copilot", "session-state")
	entries, err := os.ReadDir(sessionStateDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var sessions []SessionInfo
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		wsPath := filepath.Join(sessionStateDir, entry.Name(), "workspace.yaml")
		data, err := os.ReadFile(wsPath)
		if err != nil {
			continue
		}

		var ws copilotWorkspace
		if err := yaml.Unmarshal(data, &ws); err != nil {
			continue
		}

		id := ws.ID
		if id == "" {
			id = entry.Name()
		}

		sess := SessionInfo{
			SessionID:      id,
			Agent:          "copilot",
			ProjectPath:    ws.CWD,
			Title:          truncate(ws.Summary, 60),
			TranscriptPath: wsPath,
		}

		if t, err := time.Parse(time.RFC3339Nano, ws.CreatedAt); err == nil {
			sess.StartedAt = t
		} else if t, err := time.Parse(time.RFC3339, ws.CreatedAt); err == nil {
			sess.StartedAt = t
		}

		if t, err := time.Parse(time.RFC3339Nano, ws.UpdatedAt); err == nil {
			sess.ModifiedAt = t
		} else if t, err := time.Parse(time.RFC3339, ws.UpdatedAt); err == nil {
			sess.ModifiedAt = t
		}

		if sess.ProjectPath == "" {
			continue // skip sessions with no project path
		}

		sessions = append(sessions, sess)
	}

	return sessions, nil
}
