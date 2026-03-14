package cmd

import (
	"path/filepath"
	"reflect"
	"slices"
	"testing"

	"github.com/scottatron/maestron/internal/discover"
	"github.com/scottatron/maestron/internal/manage"
)

func TestSkillUpdateNames(t *testing.T) {
	manifest := &manage.SkillsManifest{
		Skills: map[string]*manage.SkillRecord{
			"alpha": {Name: "alpha"},
			"beta":  {Name: "beta"},
		},
	}

	tests := []struct {
		name        string
		args        []string
		updateAll   bool
		updateCheck bool
		want        []string
	}{
		{
			name:        "check named skill only checks requested skill",
			args:        []string{"alpha"},
			updateCheck: true,
			want:        []string{"alpha"},
		},
		{
			name:      "all updates every managed skill",
			updateAll: true,
			want:      []string{"alpha", "beta"},
		},
		{
			name:        "check without name inspects all managed skills",
			updateCheck: true,
			want:        []string{"alpha", "beta"},
		},
		{
			name: "named update only updates requested skill",
			args: []string{"beta"},
			want: []string{"beta"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := skillUpdateNames(tt.args, manifest, tt.updateAll, tt.updateCheck)
			slices.Sort(got)
			slices.Sort(tt.want)

			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("skillUpdateNames() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSkillsCommandsRejectExtraArgs(t *testing.T) {
	t.Run("update", func(t *testing.T) {
		if err := skillsUpdateCmd.Args(skillsUpdateCmd, []string{"one", "two"}); err == nil {
			t.Fatal("expected update command to reject extra args")
		}
	})

	t.Run("status", func(t *testing.T) {
		if err := skillsStatusCmd.Args(skillsStatusCmd, []string{"one", "two"}); err == nil {
			t.Fatal("expected status command to reject extra args")
		}
	})
}

func TestRecordForDiscoveredSkillUsesDirKey(t *testing.T) {
	manifest := &manage.SkillsManifest{
		Skills: map[string]*manage.SkillRecord{
			"repo-skill": {
				Name:        "repo-skill",
				InstallPath: "/tmp/home/.agents/skills/repo-skill",
			},
		},
	}

	skill := discover.SkillInfo{
		Name:            "frontmatter-name",
		Path:            "/tmp/home/.agents/skills/repo-skill/SKILL.md",
		ManagedRelation: discover.ManagedRelationIs,
	}

	record := recordForDiscoveredSkill(manifest, skill)
	if record == nil {
		t.Fatal("expected managed record lookup by skill dir name")
	}
	if record.Name != "repo-skill" {
		t.Fatalf("record.Name = %q, want %q", record.Name, "repo-skill")
	}
}

func TestLocalSourceRecordForSkillMatchesSourcePath(t *testing.T) {
	srcDir := filepath.Join("/tmp", "repo-root")
	manifest := &manage.SkillsManifest{
		Skills: map[string]*manage.SkillRecord{
			"repo-skill": {
				Name:        "repo-skill",
				InstallPath: "/tmp/home/.agents/skills/repo-skill",
				Source: manage.SkillSource{
					Type: "local",
					Path: srcDir,
				},
			},
		},
	}

	skill := discover.SkillInfo{
		Name: "frontmatter-name",
		Path: filepath.Join(srcDir, "SKILL.md"),
	}

	record := localSourceRecordForSkill(manifest, skill)
	if record == nil {
		t.Fatal("expected local source record lookup by source path")
	}
	if record.Name != "repo-skill" {
		t.Fatalf("record.Name = %q, want %q", record.Name, "repo-skill")
	}
}
