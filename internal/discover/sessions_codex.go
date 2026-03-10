package discover

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/scottatron/maestron/internal/platform"
)


type codexSessionMeta struct {
	ID            string `json:"id"`
	Timestamp     string `json:"timestamp"`
	CWD           string `json:"cwd"`
	CLIVersion    string `json:"cli_version"`
	ModelProvider string `json:"model_provider"`
}

type codexJSONLLine struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

type codexResponseItem struct {
	Type    string `json:"type"`
	Role    string `json:"role"`
	Model   string `json:"model"`
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
}

type codexTurnContext struct {
	Model string `json:"model"`
}

// codexIndexEntry represents a line in ~/.codex/session_index.jsonl
type codexIndexEntry struct {
	ID         string `json:"id"`
	ThreadName string `json:"thread_name"`
	UpdatedAt  string `json:"updated_at"`
}

// listCodexSessions returns all discovered Codex sessions.
func listCodexSessions() ([]SessionInfo, error) {
	home, err := platform.HomeDir()
	if err != nil {
		return nil, err
	}
	codexDir := filepath.Join(home, ".codex")

	// Read session_index.jsonl for titles
	titlesByID := readCodexSessionIndex(filepath.Join(codexDir, "session_index.jsonl"))

	sessionsDir := filepath.Join(codexDir, "sessions")
	var sessions []SessionInfo

	err = filepath.Walk(sessionsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() || !strings.HasSuffix(info.Name(), ".jsonl") {
			return nil
		}

		sess, err := parseCodexSessionFile(path, info, titlesByID)
		if err != nil {
			return nil
		}
		sessions = append(sessions, sess)
		return nil
	})

	return sessions, err
}

func readCodexSessionIndex(path string) map[string]codexIndexEntry {
	result := map[string]codexIndexEntry{}
	f, err := os.Open(path)
	if err != nil {
		return result
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var entry codexIndexEntry
		if json.Unmarshal(scanner.Bytes(), &entry) == nil && entry.ID != "" {
			result[entry.ID] = entry
		}
	}
	return result
}

func parseCodexSessionFile(path string, info os.FileInfo, index map[string]codexIndexEntry) (SessionInfo, error) {
	f, err := os.Open(path)
	if err != nil {
		return SessionInfo{}, err
	}
	defer f.Close()

	sess := SessionInfo{
		Agent:          "codex",
		ModifiedAt:     info.ModTime(),
		TranscriptPath: path,
	}

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 64*1024), 4*1024*1024)

	lineCount := 0
	for scanner.Scan() && lineCount < 100 {
		lineCount++
		var line codexJSONLLine
		if err := json.Unmarshal(scanner.Bytes(), &line); err != nil {
			continue
		}

		switch line.Type {
		case "session_meta":
			var meta codexSessionMeta
			if json.Unmarshal(line.Payload, &meta) == nil {
				sess.SessionID = meta.ID
				sess.ProjectPath = meta.CWD
				if t, err := time.Parse(time.RFC3339Nano, meta.Timestamp); err == nil {
					sess.StartedAt = t
				} else if t, err := time.Parse(time.RFC3339, meta.Timestamp); err == nil {
					sess.StartedAt = t
				}
				// Look up title from index
				if entry, ok := index[meta.ID]; ok {
					sess.Title = entry.ThreadName
					if t, err := time.Parse(time.RFC3339Nano, entry.UpdatedAt); err == nil {
						sess.ModifiedAt = t
					}
				}
			}

		case "turn_context":
			var tc codexTurnContext
			if json.Unmarshal(line.Payload, &tc) == nil && tc.Model != "" && sess.Model == "" {
				sess.Model = tc.Model
			}

		case "response_item":
			var item codexResponseItem
			if json.Unmarshal(line.Payload, &item) == nil {
				if item.Model != "" && sess.Model == "" {
					sess.Model = item.Model
				}
				if sess.Title == "" && item.Role == "user" {
					for _, c := range item.Content {
						if c.Type == "input_text" && c.Text != "" &&
							!strings.HasPrefix(c.Text, "<environment_context>") &&
							!strings.HasPrefix(c.Text, "<user_action>") &&
							!strings.HasPrefix(c.Text, "# AGENTS.md") &&
							len(c.Text) < 500 {
							sess.Title = truncate(c.Text, 60)
							break
						}
					}
				}
			}
		}

		// Stop early once we have all needed fields
		if sess.SessionID != "" && sess.Model != "" && (sess.Title != "" || lineCount >= 50) {
			break
		}
	}

	return sess, scanner.Err()
}
