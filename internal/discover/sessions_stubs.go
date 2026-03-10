package discover

// listGeminiSessions returns Gemini sessions (stub — future implementation).
func listGeminiSessions() ([]SessionInfo, error) {
	// TODO: implement Gemini session discovery from ~/.gemini/projects.json + ~/.gemini/tmp/
	return nil, nil
}

// listOpenCodeSessions returns OpenCode sessions (stub — future implementation).
func listOpenCodeSessions() ([]SessionInfo, error) {
	// TODO: implement OpenCode session discovery from ~/Library/Application Support/opencode/ (macOS)
	return nil, nil
}

// listCopilotSessions returns Copilot sessions (stub — future implementation).
// GitHub Copilot CLI does not appear to persist session transcripts locally.
func listCopilotSessions() ([]SessionInfo, error) {
	return nil, nil
}
