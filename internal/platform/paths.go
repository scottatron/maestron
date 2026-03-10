package platform

import (
	"os"
	"path/filepath"
	"runtime"
)

// HomeDir returns the current user's home directory.
func HomeDir() (string, error) {
	return os.UserHomeDir()
}

// ClaudeDir returns ~/.claude
func ClaudeDir() (string, error) {
	home, err := HomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".claude"), nil
}

// ClaudeProjectsDir returns ~/.claude/projects
func ClaudeProjectsDir() (string, error) {
	claude, err := ClaudeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(claude, "projects"), nil
}

// ClaudeSettingsFile returns ~/.claude/settings.json
func ClaudeSettingsFile() (string, error) {
	claude, err := ClaudeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(claude, "settings.json"), nil
}

// ClaudePluginsCacheDir returns ~/.claude/plugins/cache
func ClaudePluginsCacheDir() (string, error) {
	claude, err := ClaudeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(claude, "plugins", "cache"), nil
}

// UserDataDir returns the platform-specific user data directory.
// macOS: ~/Library/Application Support
// Linux: ~/.local/share (XDG_DATA_HOME)
func UserDataDir() (string, error) {
	home, err := HomeDir()
	if err != nil {
		return "", err
	}
	switch runtime.GOOS {
	case "darwin":
		return filepath.Join(home, "Library", "Application Support"), nil
	default:
		if xdg := os.Getenv("XDG_DATA_HOME"); xdg != "" {
			return xdg, nil
		}
		return filepath.Join(home, ".local", "share"), nil
	}
}
