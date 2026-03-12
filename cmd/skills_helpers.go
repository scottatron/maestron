package cmd

import (
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/term"
)

var (
	styleDiffAdded   = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#15803D", Dark: "#4ADE80"})
	styleDiffRemoved = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#B91C1C", Dark: "#F87171"})
)

// stripURLScheme removes the scheme (e.g. "https://", "git://") from a URL,
// returning just the host and path portion.
func stripURLScheme(url string) string {
	if i := strings.Index(url, "://"); i >= 0 {
		return url[i+3:]
	}
	return url
}

// isLocalPath returns true if s looks like a local filesystem path.
func isLocalPath(s string) bool {
	if strings.HasPrefix(s, "/") || strings.HasPrefix(s, "./") || strings.HasPrefix(s, "~/") {
		return true
	}
	if _, err := os.Stat(s); err == nil {
		return true
	}
	return false
}

// shortSHA returns the first 7 characters of a git SHA, or the full string if shorter.
func shortSHA(sha string) string {
	if len(sha) > 7 {
		return sha[:7]
	}
	return sha
}

// tildeSubstPath replaces the home directory prefix with "~".
func tildeSubstPath(home, path string) string {
	if strings.HasPrefix(path, home) {
		return "~" + path[len(home):]
	}
	return path
}

// colorDiff adds colors to diff output lines. Uses lipgloss when on a TTY.
func colorDiff(diff string) string {
	isTTY := term.IsTerminal(os.Stdout.Fd())
	lines := strings.Split(diff, "\n")
	var sb strings.Builder
	for _, line := range lines {
		switch {
		case strings.HasPrefix(line, "- "):
			if isTTY {
				sb.WriteString(styleDiffRemoved.Render(line) + "\n")
			} else {
				sb.WriteString(line + "\n")
			}
		case strings.HasPrefix(line, "+ "):
			if isTTY {
				sb.WriteString(styleDiffAdded.Render(line) + "\n")
			} else {
				sb.WriteString(line + "\n")
			}
		default:
			sb.WriteString(line + "\n")
		}
	}
	return sb.String()
}
