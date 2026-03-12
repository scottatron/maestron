package manage

import (
	"crypto/sha256"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

// shaPattern matches a short or full git SHA.
var shaPattern = regexp.MustCompile(`^[0-9a-f]{7,40}$`)

// SkillCandidate describes a skill found during a repo scan.
type SkillCandidate struct {
	// Subpath is the path of the skill directory relative to the repo root.
	// An empty string means the skill lives at the repo root.
	Subpath string
	// Name is derived from the subpath basename, or the repo name for root skills.
	Name string
}

// StageGitClone clones url@ref into a temp directory and returns the directory
// path, the resolved HEAD SHA, and a cleanup function the caller must invoke.
func StageGitClone(url, ref string) (tmpdir, resolvedSHA string, cleanup func(), err error) {
	tmpdir, err = os.MkdirTemp("", "maestron-skill-*")
	if err != nil {
		return "", "", nil, fmt.Errorf("create temp dir: %w", err)
	}
	cleanup = func() { os.RemoveAll(tmpdir) }

	var cloneArgs []string
	if shaPattern.MatchString(ref) {
		cloneArgs = []string{"clone", "--depth=1", url, tmpdir}
	} else if ref != "" {
		cloneArgs = []string{"clone", "--depth=1", "--branch", ref, url, tmpdir}
	} else {
		cloneArgs = []string{"clone", "--depth=1", url, tmpdir}
	}

	if out, runErr := exec.Command("git", cloneArgs...).CombinedOutput(); runErr != nil {
		cleanup()
		return "", "", nil, fmt.Errorf("git clone: %w\n%s", runErr, out)
	}

	if shaPattern.MatchString(ref) {
		if out, runErr := exec.Command("git", "-C", tmpdir, "checkout", ref).CombinedOutput(); runErr != nil {
			cleanup()
			return "", "", nil, fmt.Errorf("git checkout %s: %w\n%s", ref, runErr, out)
		}
	}

	shaOut, runErr := exec.Command("git", "-C", tmpdir, "rev-parse", "HEAD").Output()
	if runErr != nil {
		cleanup()
		return "", "", nil, fmt.Errorf("git rev-parse HEAD: %w", runErr)
	}
	resolvedSHA = strings.TrimSpace(string(shaOut))
	return tmpdir, resolvedSHA, cleanup, nil
}

// ScanSkills walks dir and returns a SkillCandidate for each subdirectory (or
// the root) that contains a SKILL.md file. Hidden directories are skipped.
func ScanSkills(dir, repoURL string) ([]SkillCandidate, error) {
	var candidates []SkillCandidate
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && strings.HasPrefix(d.Name(), ".") {
			return filepath.SkipDir
		}
		if !d.IsDir() && d.Name() == "SKILL.md" {
			skillDir := filepath.Dir(path)
			rel, _ := filepath.Rel(dir, skillDir)
			if rel == "." {
				rel = ""
			}
			name := filepath.Base(rel)
			if rel == "" {
				name = strings.TrimSuffix(filepath.Base(repoURL), ".git")
			}
			candidates = append(candidates, SkillCandidate{Subpath: rel, Name: name})
		}
		return nil
	})
	return candidates, err
}

// InstallFromStaged copies a skill from an already-cloned stagedir into the
// skills directory. url, ref, and resolvedSHA describe the source for the manifest.
func InstallFromStaged(home, stagedir, subpath, name, url, ref, resolvedSHA string) (*SkillRecord, error) {
	srcDir := stagedir
	if subpath != "" {
		srcDir = filepath.Join(stagedir, subpath)
	}

	dest := filepath.Join(home, ".agents", "skills", name)
	if err := copyDir(srcDir, dest); err != nil {
		return nil, fmt.Errorf("copy skill: %w", err)
	}

	hash, err := contentHash(dest)
	if err != nil {
		return nil, fmt.Errorf("compute content hash: %w", err)
	}

	now := time.Now().UTC()
	return &SkillRecord{
		Name:        name,
		InstallPath: dest,
		Source: SkillSource{
			Type:        "git",
			URL:         url,
			Ref:         ref,
			ResolvedSHA: resolvedSHA,
			Subpath:     subpath,
		},
		InstalledAt: now,
		UpdatedAt:   now,
		ContentHash: hash,
	}, nil
}

// InstallFromGit clones a git repo and installs a skill from it.
// For multi-skill repos use StageGitClone + ScanSkills + InstallFromStaged instead.
func InstallFromGit(home, url, ref, subpath, name string) (*SkillRecord, error) {
	stagedir, resolvedSHA, cleanup, err := StageGitClone(url, ref)
	if err != nil {
		return nil, err
	}
	defer cleanup()

	if name == "" {
		if subpath != "" {
			name = filepath.Base(subpath)
		} else {
			name = strings.TrimSuffix(filepath.Base(url), ".git")
		}
	}

	return InstallFromStaged(home, stagedir, subpath, name, url, ref, resolvedSHA)
}

// InstallFromLocal copies a skill from a local directory.
// name overrides the default skill name (derived from the base of srcPath).
func InstallFromLocal(home, srcPath, name string) (*SkillRecord, error) {
	if strings.HasPrefix(srcPath, "~/") {
		srcPath = filepath.Join(home, srcPath[2:])
	}

	if name == "" {
		name = filepath.Base(srcPath)
	}

	dest := filepath.Join(home, ".agents", "skills", name)
	if err := copyDir(srcPath, dest); err != nil {
		return nil, fmt.Errorf("copy skill: %w", err)
	}

	hash, err := contentHash(dest)
	if err != nil {
		return nil, fmt.Errorf("compute content hash: %w", err)
	}

	hostname, _ := os.Hostname()
	now := time.Now().UTC()
	return &SkillRecord{
		Name:        name,
		InstallPath: dest,
		Source: SkillSource{
			Type:     "local",
			Path:     srcPath,
			Hostname: hostname,
		},
		InstalledAt: now,
		UpdatedAt:   now,
		ContentHash: hash,
	}, nil
}

// contentHash returns a sha256 hash of all files in dir, sorted by relative path.
func contentHash(dir string) (string, error) {
	var files []string
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			rel, _ := filepath.Rel(dir, path)
			files = append(files, rel)
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	sort.Strings(files)

	h := sha256.New()
	for _, rel := range files {
		fmt.Fprintf(h, "%s\n", rel)
		data, err := os.ReadFile(filepath.Join(dir, rel))
		if err != nil {
			return "", err
		}
		h.Write(data)
	}
	return fmt.Sprintf("sha256:%x", h.Sum(nil)), nil
}

// copyDir copies src directory to dst, replacing dst if it exists.
func copyDir(src, dst string) error {
	if err := os.RemoveAll(dst); err != nil {
		return err
	}
	if err := os.MkdirAll(dst, 0755); err != nil {
		return err
	}
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, path)
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0755)
		}
		return copyFile(path, target)
	})
}

// copyFile copies a single file preserving its mode.
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	info, err := in.Stat()
	if err != nil {
		return err
	}

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, info.Mode())
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}
