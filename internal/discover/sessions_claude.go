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

// claudeSessionsIndex matches ~/.claude/projects/<encoded>/sessions-index.json
type claudeSessionsIndex struct {
	Version int                  `json:"version"`
	Entries []claudeSessionEntry `json:"entries"`
}

type claudeSessionEntry struct {
	SessionID    string `json:"sessionId"`
	FullPath     string `json:"fullPath"`
	FileMtime    int64  `json:"fileMtime"`
	FirstPrompt  string `json:"firstPrompt"`
	Summary      string `json:"summary"`
	MessageCount int    `json:"messageCount"`
	Created      string `json:"created"`
	Modified     string `json:"modified"`
	GitBranch    string `json:"gitBranch"`
	ProjectPath  string `json:"projectPath"`
	IsSidechain  bool   `json:"isSidechain"`
}

// claudeJSONLLine is used to extract metadata from .jsonl session files.
type claudeJSONLLine struct {
	Type string `json:"type"`
	CWD  string `json:"cwd"`
	Message struct {
		Role    string `json:"role"`
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		Model string `json:"model"`
	} `json:"message"`
	Usage struct {
		InputTokens              int64 `json:"input_tokens"`
		OutputTokens             int64 `json:"output_tokens"`
		CacheReadInputTokens     int64 `json:"cache_read_input_tokens"`
		CacheCreationInputTokens int64 `json:"cache_creation_input_tokens"`
	} `json:"usage"`
}

// listClaudeSessions returns all discovered Claude sessions.
func listClaudeSessions() ([]SessionInfo, error) {
	projectsDir, err := platform.ClaudeProjectsDir()
	if err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(projectsDir)
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
		dirName := entry.Name()
		projectDir := filepath.Join(projectsDir, dirName)
		projectPath := decodeProjectPath(dirName)

		// Try sessions-index.json first
		indexPath := filepath.Join(projectDir, "sessions-index.json")
		if si, err := readClaudeSessionsIndex(indexPath); err == nil {
			for _, e := range si.Entries {
				if e.IsSidechain {
					continue
				}
				pp := e.ProjectPath
				if pp == "" {
					pp = projectPath
				}
				sessions = append(sessions, claudeEntryToSession(e, pp, projectDir))
			}
			continue
		}

		// Fall back: scan *.jsonl files
		jsonlSessions, err := scanClaudeJSONLDir(projectDir, projectPath)
		if err == nil {
			sessions = append(sessions, jsonlSessions...)
		}
	}

	return sessions, nil
}

func readClaudeSessionsIndex(path string) (*claudeSessionsIndex, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var idx claudeSessionsIndex
	if err := json.Unmarshal(data, &idx); err != nil {
		return nil, err
	}
	return &idx, nil
}

func claudeEntryToSession(e claudeSessionEntry, projectPath, projectDir string) SessionInfo {
	title := e.Summary
	if title == "" {
		title = truncate(e.FirstPrompt, 60)
	}

	var startedAt, modifiedAt time.Time
	if t, err := time.Parse(time.RFC3339, e.Created); err == nil {
		startedAt = t
	}
	if t, err := time.Parse(time.RFC3339, e.Modified); err == nil {
		modifiedAt = t
	} else if e.FileMtime > 0 {
		modifiedAt = time.Unix(e.FileMtime/1000, 0)
	}

	return SessionInfo{
		SessionID:      e.SessionID,
		Agent:          "claude",
		Title:          title,
		ProjectPath:    projectPath,
		StartedAt:      startedAt,
		ModifiedAt:     modifiedAt,
		MessageCount:   e.MessageCount,
		TranscriptPath: filepath.Join(projectDir, e.SessionID+".jsonl"),
	}
}

func scanClaudeJSONLDir(dir, projectPath string) ([]SessionInfo, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var sessions []SessionInfo
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".jsonl") {
			continue
		}
		sessionID := strings.TrimSuffix(entry.Name(), ".jsonl")
		transcriptPath := filepath.Join(dir, entry.Name())

		info, err := entry.Info()
		if err != nil {
			continue
		}

		sess := SessionInfo{
			SessionID:      sessionID,
			Agent:          "claude",
			ProjectPath:    projectPath,
			ModifiedAt:     info.ModTime(),
			TranscriptPath: transcriptPath,
		}

		if meta, err := scanJSONLMeta(transcriptPath); err == nil {
			sess.Model = meta.model
			sess.Title = truncate(meta.firstPrompt, 60)
			sess.Tokens = meta.tokens
			if meta.cwd != "" {
				sess.ProjectPath = meta.cwd
			}
		}

		sessions = append(sessions, sess)
	}
	return sessions, nil
}

type jsonlMeta struct {
	cwd         string
	model       string
	firstPrompt string
	tokens      TokenUsage
}

func scanJSONLMeta(path string) (*jsonlMeta, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	meta := &jsonlMeta{}
	scanner := bufio.NewScanner(f)
	// Increase buffer for long lines
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	lineCount := 0
	for scanner.Scan() && lineCount < 50 {
		lineCount++
		var line claudeJSONLLine
		if err := json.Unmarshal(scanner.Bytes(), &line); err != nil {
			continue
		}
		if line.CWD != "" && meta.cwd == "" {
			meta.cwd = line.CWD
		}
		if line.Message.Model != "" && meta.model == "" {
			meta.model = line.Message.Model
		}
		if meta.firstPrompt == "" && line.Message.Role == "user" {
			for _, c := range line.Message.Content {
				if c.Type == "text" && c.Text != "" {
					meta.firstPrompt = c.Text
					break
				}
			}
		}
		meta.tokens.Input += line.Usage.InputTokens
		meta.tokens.Output += line.Usage.OutputTokens
		meta.tokens.CacheRead += line.Usage.CacheReadInputTokens
		meta.tokens.CacheCreate += line.Usage.CacheCreationInputTokens
	}
	return meta, scanner.Err()
}

// decodeProjectPath converts a Claude-encoded directory name back to a filesystem path.
// Claude encodes /Users/foo/bar as -Users-foo-bar (prepend -, replace / with -).
func decodeProjectPath(encoded string) string {
	s := strings.TrimPrefix(encoded, "-")
	s = strings.ReplaceAll(s, "-", "/")
	return "/" + s
}

func truncate(s string, n int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n]) + "..."
}
