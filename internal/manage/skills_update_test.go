package manage

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestCheckUpdateAnnotatedTagUsesPeeledCommit(t *testing.T) {
	t.Parallel()

	repoDir := t.TempDir()
	runGit(t, repoDir, "init")
	runGit(t, repoDir, "config", "user.name", "Test User")
	runGit(t, repoDir, "config", "user.email", "test@example.com")

	skillFile := filepath.Join(repoDir, "SKILL.md")
	if err := os.WriteFile(skillFile, []byte("# Skill\n"), 0644); err != nil {
		t.Fatalf("write SKILL.md: %v", err)
	}

	runGit(t, repoDir, "add", "SKILL.md")
	runGit(t, repoDir, "commit", "-m", "initial")
	commitSHA := runGit(t, repoDir, "rev-parse", "HEAD")
	runGit(t, repoDir, "tag", "-a", "v1.0.0", "-m", "release")

	remoteDir := filepath.Join(t.TempDir(), "remote.git")
	runGit(t, "", "clone", "--bare", repoDir, remoteDir)

	status := CheckUpdate(&SkillRecord{
		Name: "example",
		Source: SkillSource{
			Type:        "git",
			URL:         remoteDir,
			Ref:         "v1.0.0",
			ResolvedSHA: commitSHA,
		},
	})
	if status.Err != nil {
		t.Fatalf("CheckUpdate returned error: %v", status.Err)
	}
	if status.HasUpdate {
		t.Fatalf("expected no update for unchanged annotated tag, got %+v", status)
	}
	if status.RemoteSHA != commitSHA {
		t.Fatalf("expected peeled commit %q, got %q", commitSHA, status.RemoteSHA)
	}
}

func runGit(t *testing.T, dir string, args ...string) string {
	t.Helper()

	cmd := exec.Command("git", args...)
	if dir != "" {
		cmd.Dir = dir
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s failed: %v\n%s", strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out))
}
