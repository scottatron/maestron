package manage

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// UpdateStatus reports the result of checking a skill for upstream changes.
type UpdateStatus struct {
	Name      string
	HasUpdate bool
	RemoteSHA string // populated for git sources
	Err       error
}

// CheckUpdate inspects a skill record against its source to detect available updates.
// For git sources this makes a network call (git ls-remote).
// For local sources it checks the source path on the same host.
func CheckUpdate(record *SkillRecord) UpdateStatus {
	status := UpdateStatus{Name: record.Name}

	switch record.Source.Type {
	case "git":
		remoteSHA, err := resolveRemoteGitSHA(record.Source.URL, record.Source.Ref)
		if err != nil {
			status.Err = err
			return status
		}
		status.RemoteSHA = remoteSHA
		// Compare: either could be a prefix of the other for short SHAs
		if !strings.HasPrefix(remoteSHA, record.Source.ResolvedSHA) &&
			!strings.HasPrefix(record.Source.ResolvedSHA, remoteSHA) {
			status.HasUpdate = true
		}

	case "local":
		hostname, _ := os.Hostname()
		if hostname != record.Source.Hostname {
			// Installed on a different host; not an error, just skip
			return status
		}
		if _, err := os.Stat(record.Source.Path); os.IsNotExist(err) {
			status.Err = fmt.Errorf("source path %q no longer exists", record.Source.Path)
			return status
		}
		hash, err := contentHash(record.Source.Path)
		if err != nil {
			status.Err = fmt.Errorf("hash source dir: %w", err)
			return status
		}
		if hash != record.ContentHash {
			status.HasUpdate = true
		}
	}

	return status
}

func resolveRemoteGitSHA(url, ref string) (string, error) {
	if ref == "" {
		ref = "HEAD"
	}
	if shaPattern.MatchString(ref) {
		// A pinned commit does not move upstream; compare against the pinned SHA.
		return ref, nil
	}

	patterns := []string{ref}
	if ref != "HEAD" {
		patterns = append(patterns, ref+"^{}")
	}

	out, err := exec.Command("git", append([]string{"ls-remote", url}, patterns...)...).Output()
	if err != nil {
		return "", fmt.Errorf("git ls-remote: %w", err)
	}

	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) == 0 || lines[0] == "" {
		return "", fmt.Errorf("no matching ref %q at %s", ref, url)
	}

	var fallbackSHA string
	for _, line := range lines {
		parts := strings.Fields(line)
		if len(parts) < 2 {
			return "", fmt.Errorf("unexpected git ls-remote output")
		}
		sha := parts[0]
		remoteRef := parts[1]
		if strings.HasSuffix(remoteRef, "^{}") {
			return sha, nil
		}
		if fallbackSHA == "" {
			fallbackSHA = sha
		}
	}
	if fallbackSHA == "" {
		return "", fmt.Errorf("unexpected git ls-remote output")
	}
	return fallbackSHA, nil
}

// UpdateSkill re-installs a skill from its source, preserving the original
// install timestamp and updating the updated_at timestamp.
func UpdateSkill(home string, record *SkillRecord) (*SkillRecord, error) {
	var (
		newRecord *SkillRecord
		err       error
	)

	switch record.Source.Type {
	case "git":
		newRecord, err = InstallFromGit(home, record.Source.URL, record.Source.Ref, record.Source.Subpath, record.Name)
	case "local":
		newRecord, err = InstallFromLocal(home, record.Source.Path, record.Name)
	default:
		return nil, fmt.Errorf("unknown source type %q", record.Source.Type)
	}
	if err != nil {
		return nil, err
	}

	// Preserve original install time
	newRecord.InstalledAt = record.InstalledAt
	newRecord.UpdatedAt = time.Now().UTC()
	return newRecord, nil
}
