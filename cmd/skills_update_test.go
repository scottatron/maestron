package cmd

import (
	"reflect"
	"slices"
	"testing"

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
