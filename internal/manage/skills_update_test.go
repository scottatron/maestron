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
	if err := os.WriteFile(skillFile, []byte("# Skill\n"), 0o644); err != nil {
		t.Fatalf("write SKILL.md: %v", err)
	}

	runGit(t, repoDir, "add", "SKILL.md")
	runGit(t, repoDir, "commit", "-m", "initial")
	commitSHA := runGit(t, repoDir, "rev-parse", "HEAD")
	runGit(t, repoDir, "tag", "-a", "v1.0.0", "-m", "release")

	remoteDir := filepath.Join(t.TempDir(), "remote.git")
	runGit(t, "", "clone", "--bare", repoDir, remoteDir)
	installDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(installDir, "SKILL.md"), []byte("# Skill\n"), 0o644); err != nil {
		t.Fatalf("write installed SKILL.md: %v", err)
	}
	installedHash, err := contentHash(installDir)
	if err != nil {
		t.Fatalf("contentHash: %v", err)
	}

	status := CheckUpdate(&SkillRecord{
		Name:        "example",
		InstallPath: installDir,
		ContentHash: installedHash,
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

func TestHashInstallableSnapshotMatchesInstalledContent(t *testing.T) {
	t.Parallel()

	src := t.TempDir()
	dest := filepath.Join(t.TempDir(), "installed")

	mustWriteFile(t, filepath.Join(src, "SKILL.md"), "name: Example\n")
	mustWriteFile(t, filepath.Join(src, "scripts", "run.sh"), "#!/bin/sh\necho hi\n")
	mustWriteFile(t, filepath.Join(src, ".git", "HEAD"), "ref: refs/heads/main\n")

	sourceHash, err := hashInstallableSnapshot(src)
	if err != nil {
		t.Fatalf("hashInstallableSnapshot: %v", err)
	}
	if err := copyDir(src, dest); err != nil {
		t.Fatalf("copyDir: %v", err)
	}
	installedHash, err := contentHash(dest)
	if err != nil {
		t.Fatalf("contentHash: %v", err)
	}

	if sourceHash != installedHash {
		t.Fatalf("canonical source hash %q != installed hash %q", sourceHash, installedHash)
	}
}

func TestCheckUpdateReportsLocalModificationWithoutUpstreamChange(t *testing.T) {
	t.Parallel()

	src := t.TempDir()
	destRoot := t.TempDir()
	dest := filepath.Join(destRoot, "installed")

	mustWriteFile(t, filepath.Join(src, "SKILL.md"), "name: Example\n")
	if err := copyDir(src, dest); err != nil {
		t.Fatalf("copyDir: %v", err)
	}
	hash, err := contentHash(dest)
	if err != nil {
		t.Fatalf("contentHash: %v", err)
	}
	hostname, _ := os.Hostname()
	record := &SkillRecord{
		Name:        "example",
		InstallPath: dest,
		ContentHash: hash,
		Source: SkillSource{
			Type:     "local",
			Path:     src,
			Hostname: hostname,
		},
	}

	mustWriteFile(t, filepath.Join(dest, "SKILL.md"), "name: Example\nmodified: true\n")

	status := CheckUpdate(record)
	if status.Err != nil {
		t.Fatalf("CheckUpdate: %v", status.Err)
	}
	if !status.LocalModified {
		t.Fatalf("expected local modification to be detected")
	}
	if status.HasUpdate {
		t.Fatalf("did not expect upstream update when source is unchanged")
	}
}

func TestCheckUpdateIgnoresVCSNoiseInLocalSource(t *testing.T) {
	t.Parallel()

	home := t.TempDir()
	srcDir := filepath.Join(home, "source-skill")
	if err := os.MkdirAll(filepath.Join(srcDir, ".git"), 0o755); err != nil {
		t.Fatalf("create source dirs: %v", err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "SKILL.md"), []byte("# Skill\n"), 0o644); err != nil {
		t.Fatalf("write SKILL.md: %v", err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, ".git", "HEAD"), []byte("ref: refs/heads/main\n"), 0o644); err != nil {
		t.Fatalf("write .git/HEAD: %v", err)
	}

	record, err := InstallFromLocal(home, srcDir, "example")
	if err != nil {
		t.Fatalf("InstallFromLocal returned error: %v", err)
	}

	status := CheckUpdate(record)
	if status.Err != nil {
		t.Fatalf("CheckUpdate returned error: %v", status.Err)
	}
	if status.HasUpdate {
		t.Fatalf("expected no update when only VCS metadata differs, got %+v", status)
	}
	if status.LocalModified {
		t.Fatalf("did not expect installed content to be marked modified")
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

func mustWriteFile(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll(%q): %v", path, err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("WriteFile(%q): %v", path, err)
	}
}
