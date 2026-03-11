package discover

import "time"

// NodeInfo is the top-level summary of a node.
type NodeInfo struct {
	Hostname     string      `json:"hostname"`
	OS           string      `json:"os"`
	Agents       []AgentInfo `json:"agents"`
	SessionCount int         `json:"session_count"`
	SkillCount   int         `json:"skill_count"`
	MCPCount     int         `json:"mcp_count"`
	GeneratedAt  time.Time   `json:"generated_at"`
}

// AgentInfo describes a detected AI coding agent.
type AgentInfo struct {
	Name             string `json:"name"`
	DisplayName      string `json:"display_name"`
	Version          string `json:"version"`
	RequestedVersion string `json:"requested_version"`
	Source           string `json:"source"`
	InstallPath      string `json:"install_path"`
	Active           bool   `json:"active"`
	MiseToolName     string `json:"mise_tool_name"`
}

// SessionInfo describes a single agent session.
type SessionInfo struct {
	SessionID      string     `json:"session_id"`
	Agent          string     `json:"agent"`
	Model          string     `json:"model"`
	Title          string     `json:"title"`
	ProjectPath    string     `json:"project_path"`
	StartedAt      time.Time  `json:"started_at"`
	ModifiedAt     time.Time  `json:"modified_at"`
	MessageCount   int        `json:"message_count"`
	Turns          int        `json:"turns"`
	Tokens         TokenUsage `json:"tokens"`
	TranscriptPath string     `json:"transcript_path"`
}

// TokenUsage holds token counts for a session.
type TokenUsage struct {
	Input       int64 `json:"input"`
	Output      int64 `json:"output"`
	CacheRead   int64 `json:"cache_read"`
	CacheCreate int64 `json:"cache_create"`
}

// SessionGroup groups sessions by project path.
type SessionGroup struct {
	ProjectPath string        `json:"project_path"`
	Sessions    []SessionInfo `json:"sessions"`
}

// SessionFilter filters session results.
type SessionFilter struct {
	Project string
	Agent   string
	Limit   int
	Since   time.Duration
}

// SkillInfo describes a discovered skill.
type SkillInfo struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Source      string `json:"source"`
	Path        string `json:"path"`
}

// MCPServerInfo describes a configured MCP server.
type MCPServerInfo struct {
	Name       string            `json:"name"`
	Label      string            `json:"label"`
	Command    string            `json:"command"`
	Args       []string          `json:"args"`
	Env        map[string]string `json:"env,omitempty"`
	URL        string            `json:"url,omitempty"`
	Transport  string            `json:"transport"`
	Targets    []string          `json:"targets"`
	Enabled    *bool             `json:"enabled"`
	Source     string            `json:"source"`
	ConfigPath string            `json:"config_path,omitempty"`
	Shadowed   bool              `json:"shadowed,omitempty"`
}
