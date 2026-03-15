package manage

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// PreviewInstall compares the skill directory at destPath against the upstream
// source. Returns a formatted diff string if they differ, or "" if destPath does
// not exist or the content is identical. An error is returned only if the
// upstream could not be fetched.
func PreviewInstall(destPath string, source SkillSource) (string, error) {
	if _, err := os.Stat(destPath); os.IsNotExist(err) {
		return "", nil
	}

	var diff string
	var err error
	switch source.Type {
	case "git":
		stagedir, _, cleanup, cloneErr := StageGitClone(source.URL, source.Ref)
		if cloneErr != nil {
			return "", cloneErr
		}
		defer cleanup()
		srcDir := stagedir
		if source.Subpath != "" {
			srcDir = filepath.Join(stagedir, source.Subpath)
		}
		diff, err = DiffSkillDir(destPath, srcDir)
	case "local":
		diff, err = DiffSkillDir(destPath, source.Path)
	default:
		return "", fmt.Errorf("unknown source type %q", source.Type)
	}
	if err != nil {
		return "", err
	}
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

// DiffSkill returns a sectioned diff of the installed skill against the upstream
// version, covering all files in the skill directory (SKILL.md, scripts/,
// references/, assets/, etc.).
func DiffSkill(record *SkillRecord) (string, error) {
	var (
		upstreamDir string
		cleanup     func()
	)

	switch record.Source.Type {
	case "git":
		stagedir, _, cl, err := StageGitClone(record.Source.URL, record.Source.Ref)
		if err != nil {
			return "", err
		}
		cleanup = cl
		upstreamDir = stagedir
		if record.Source.Subpath != "" {
			upstreamDir = filepath.Join(stagedir, record.Source.Subpath)
		}
	case "local":
		upstreamDir = record.Source.Path
	default:
		return "", fmt.Errorf("unknown source type %q", record.Source.Type)
	}
	if cleanup != nil {
		defer cleanup()
	}

	return DiffSkillDir(record.InstallPath, upstreamDir)
}

// DiffStagedSKILL returns a formatted diff between the installed skill at
// destPath and the staged version at filepath.Join(stagedir, subpath).
// Returns "" if the skill directories are identical or both absent.
func DiffStagedSKILL(destPath, stagedir, subpath string) (string, error) {
	srcDir := stagedir
	if subpath != "" {
		srcDir = filepath.Join(stagedir, subpath)
	}
	diff, err := DiffSkillDir(destPath, srcDir)
	if err != nil {
		return "", err
	}
	if !diffHasChanges(diff) {
		return "", nil
	}
	return diff, nil
}

// DiffSkillDir computes a sectioned, file-by-file diff between two skill
// directories. Each file that differs gets a "=== <path>" header followed by
// line-diff output (+ / - /   prefixes). Files present in only one side are
// marked "(added)" or "(removed)". Binary files that differ show a one-line
// marker instead of a line diff.
func DiffSkillDir(localDir, upstreamDir string) (string, error) {
	localFiles, err := listSkillFiles(localDir)
	if err != nil && !os.IsNotExist(err) {
		return "", fmt.Errorf("list local skill files: %w", err)
	}
	upstreamFiles, err := listSkillFiles(upstreamDir)
	if err != nil && !os.IsNotExist(err) {
		return "", fmt.Errorf("list upstream skill files: %w", err)
	}

	localSet := toStringSet(localFiles)
	upstreamSet := toStringSet(upstreamFiles)
	allFiles := unionSorted(localFiles, upstreamFiles)

	var sb strings.Builder
	for _, f := range allFiles {
		inLocal := localSet[f]
		inUpstream := upstreamSet[f]

		var localData, upstreamData []byte
		if inLocal {
			localData, err = os.ReadFile(filepath.Join(localDir, f))
			if err != nil {
				return "", fmt.Errorf("read %s: %w", f, err)
			}
		}
		if inUpstream {
			upstreamData, err = os.ReadFile(filepath.Join(upstreamDir, f))
			if err != nil {
				return "", fmt.Errorf("read upstream %s: %w", f, err)
			}
		}

		// Skip unchanged files.
		if inLocal && inUpstream && bytes.Equal(localData, upstreamData) {
			continue
		}

		// File section header.
		switch {
		case !inLocal:
			sb.WriteString(fmt.Sprintf("=== %s (added)\n", f))
		case !inUpstream:
			sb.WriteString(fmt.Sprintf("=== %s (removed)\n", f))
		default:
			sb.WriteString(fmt.Sprintf("=== %s\n", f))
		}

		// Binary files: just note they differ.
		if isBinary(localData) || isBinary(upstreamData) {
			sb.WriteString("~ binary file differs\n")
			continue
		}

		// Text diff.
		var localLines, upstreamLines []string
		if localData != nil {
			localLines = strings.Split(string(localData), "\n")
		}
		if upstreamData != nil {
			upstreamLines = strings.Split(string(upstreamData), "\n")
		}
		sb.WriteString(formatDiff(localLines, upstreamLines))
	}
	return sb.String(), nil
}

// listSkillFiles returns sorted relative paths of all non-hidden files under dir.
// Returns nil (not an error) if dir does not exist.
func listSkillFiles(dir string) ([]string, error) {
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return nil, nil
	}
	var files []string
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // skip unreadable entries
		}
		if d.IsDir() {
			if strings.HasPrefix(d.Name(), ".") {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasPrefix(d.Name(), ".") {
			return nil
		}
		rel, _ := filepath.Rel(dir, path)
		files = append(files, rel)
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(files)
	return files, nil
}

// isBinary reports whether data appears to be binary (contains a null byte).
func isBinary(data []byte) bool {
	return bytes.IndexByte(data, 0) >= 0
}

// toStringSet converts a string slice to a set map.
func toStringSet(ss []string) map[string]bool {
	m := make(map[string]bool, len(ss))
	for _, s := range ss {
		m[s] = true
	}
	return m
}

// unionSorted returns a sorted slice of all unique strings from a and b,
// assuming both inputs are already sorted.
func unionSorted(a, b []string) []string {
	seen := make(map[string]bool, len(a)+len(b))
	for _, s := range a {
		seen[s] = true
	}
	for _, s := range b {
		seen[s] = true
	}
	result := make([]string, 0, len(seen))
	for s := range seen {
		result = append(result, s)
	}
	sort.Strings(result)
	return result
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
