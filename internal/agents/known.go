package agents

// KnownAgent describes a recognized AI coding agent.
type KnownAgent struct {
	Name        string // logical name, e.g. "claude"
	DisplayName string // human display name, e.g. "Claude Code"
}

// knownAgents maps mise tool names to their agent metadata.
var knownAgents = map[string]KnownAgent{
	"aqua:anthropics/claude-code": {Name: "claude", DisplayName: "Claude Code"},
	"claude":                      {Name: "claude", DisplayName: "Claude Code"},
	"npm:@openai/codex":           {Name: "codex", DisplayName: "OpenAI Codex"},
	"codex":                       {Name: "codex", DisplayName: "OpenAI Codex"},
	"npm:@github/copilot":         {Name: "copilot", DisplayName: "GitHub Copilot"},
	"copilot":                     {Name: "copilot", DisplayName: "GitHub Copilot"},
	"npm:@google/gemini-cli":      {Name: "gemini", DisplayName: "Gemini CLI"},
	"gemini":                      {Name: "gemini", DisplayName: "Gemini CLI"},
	"opencode":                    {Name: "opencode", DisplayName: "OpenCode"},
}

// LookupAgent returns KnownAgent info for a mise tool name.
// ok is false if the tool is not a known agent.
func LookupAgent(miseToolName string) (KnownAgent, bool) {
	a, ok := knownAgents[miseToolName]
	return a, ok
}

// IsKnownAgent reports whether the given mise tool name is a known agent.
func IsKnownAgent(miseToolName string) bool {
	_, ok := knownAgents[miseToolName]
	return ok
}
