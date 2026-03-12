package manage

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// PreviewInstall compares the SKILL.md at destPath against the upstream source.
// Returns a formatted diff string if they differ, or "" if destPath does not
// exist or the content is identical. An error is returned only if the upstream
// could not be fetched.
func PreviewInstall(destPath string, source SkillSource) (string, error) {
	if _, err := os.Stat(destPath); os.IsNotExist(err) {
		return "", nil
	}

	localLines, err := readSKILLMDLines(destPath, "")
	if err != nil {
		return "", err
	}

	var upstreamLines []string
	switch source.Type {
	case "git":
		upstreamLines, err = gitUpstreamSkillLines(&SkillRecord{Source: source})
		if err != nil {
			return "", err
		}
	case "local":
		upstreamLines, err = readSKILLMDLines(source.Path, "")
		if err != nil {
			return "", err
		}
	default:
		return "", fmt.Errorf("unknown source type %q", source.Type)
	}

	diff := formatDiff(localLines, upstreamLines)
	if !diffHasChanges(diff) {
		return "", nil
	}
	return diff, nil
}

// diffHasChanges reports whether a formatted diff string contains any added or
// removed lines (i.e. lines prefixed with "+ " or "- ").
func diffHasChanges(diff string) bool {
	for _, line := range strings.Split(diff, "\n") {
		if strings.HasPrefix(line, "+ ") || strings.HasPrefix(line, "- ") {
			return true
		}
	}
	return false
}

// DiffSkill returns a line-by-line diff of the installed skill's SKILL.md
// against the upstream version. Lines removed from upstream are prefixed "- ",
// lines added in upstream are prefixed "+ ", and unchanged lines are prefixed "  ".
func DiffSkill(record *SkillRecord) (string, error) {
	localPath := filepath.Join(record.InstallPath, "SKILL.md")
	localData, err := os.ReadFile(localPath)
	if err != nil && !os.IsNotExist(err) {
		return "", fmt.Errorf("read local SKILL.md: %w", err)
	}
	var localLines []string
	if localData != nil {
		localLines = strings.Split(string(localData), "\n")
	}

	var upstreamLines []string
	switch record.Source.Type {
	case "git":
		upstreamLines, err = gitUpstreamSkillLines(record)
		if err != nil {
			return "", err
		}
	case "local":
		upstreamPath := filepath.Join(record.Source.Path, "SKILL.md")
		data, err := os.ReadFile(upstreamPath)
		if err != nil {
			return "", fmt.Errorf("read upstream SKILL.md: %w", err)
		}
		upstreamLines = strings.Split(string(data), "\n")
	default:
		return "", fmt.Errorf("unknown source type %q", record.Source.Type)
	}

	return formatDiff(localLines, upstreamLines), nil
}

// gitUpstreamSkillLines clones the upstream git source and reads its SKILL.md lines.
func gitUpstreamSkillLines(record *SkillRecord) ([]string, error) {
	stagedir, _, cleanup, err := StageGitClone(record.Source.URL, record.Source.Ref)
	if err != nil {
		return nil, err
	}
	defer cleanup()
	return readSKILLMDLines(stagedir, record.Source.Subpath)
}

// DiffStagedSKILL returns a formatted diff between the installed skill at
// destPath and the staged version at filepath.Join(stagedir, subpath).
// Returns "" if the SKILL.md files are identical or both absent.
func DiffStagedSKILL(destPath, stagedir, subpath string) (string, error) {
	existing, err := readSKILLMDLines(destPath, "")
	if err != nil {
		return "", err
	}
	upstream, err := readSKILLMDLines(stagedir, subpath)
	if err != nil {
		return "", err
	}
	diff := formatDiff(existing, upstream)
	if !diffHasChanges(diff) {
		return "", nil
	}
	return diff, nil
}

// readSKILLMDLines reads SKILL.md from filepath.Join(base, subpath).
// Returns nil (not an error) if the file does not exist.
func readSKILLMDLines(base, subpath string) ([]string, error) {
	data, err := os.ReadFile(filepath.Join(base, subpath, "SKILL.md"))
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return strings.Split(string(data), "\n"), nil
}

// formatDiff generates a diff of current vs upstream using LCS.
// current is the installed version; upstream is the source version.
func formatDiff(current, upstream []string) string {
	lcs := computeLCS(current, upstream)

	var sb strings.Builder
	i, j, k := 0, 0, 0
	for k < len(lcs) {
		for i < len(current) && current[i] != lcs[k] {
			sb.WriteString("- " + current[i] + "\n")
			i++
		}
		for j < len(upstream) && upstream[j] != lcs[k] {
			sb.WriteString("+ " + upstream[j] + "\n")
			j++
		}
		sb.WriteString("  " + lcs[k] + "\n")
		i++
		j++
		k++
	}
	// Remaining lines after LCS is exhausted
	for i < len(current) {
		sb.WriteString("- " + current[i] + "\n")
		i++
	}
	for j < len(upstream) {
		sb.WriteString("+ " + upstream[j] + "\n")
		j++
	}
	return sb.String()
}

// computeLCS returns the longest common subsequence of two string slices.
func computeLCS(a, b []string) []string {
	m, n := len(a), len(b)
	dp := make([][]int, m+1)
	for i := range dp {
		dp[i] = make([]int, n+1)
	}
	for i := 1; i <= m; i++ {
		for j := 1; j <= n; j++ {
			if a[i-1] == b[j-1] {
				dp[i][j] = dp[i-1][j-1] + 1
			} else if dp[i-1][j] > dp[i][j-1] {
				dp[i][j] = dp[i-1][j]
			} else {
				dp[i][j] = dp[i][j-1]
			}
		}
	}

	// Backtrack to build LCS
	lcs := make([]string, 0, dp[m][n])
	i, j := m, n
	for i > 0 && j > 0 {
		if a[i-1] == b[j-1] {
			lcs = append(lcs, a[i-1])
			i--
			j--
		} else if dp[i-1][j] > dp[i][j-1] {
			i--
		} else {
			j--
		}
	}

	// Reverse
	for l, r := 0, len(lcs)-1; l < r; l, r = l+1, r-1 {
		lcs[l], lcs[r] = lcs[r], lcs[l]
	}
	return lcs
}
