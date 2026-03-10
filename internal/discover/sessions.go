package discover

import (
	"sort"
	"strings"
	"time"
)

// ListSessions discovers all sessions, applying the given filter.
// Results are grouped by project path.
func ListSessions(filter SessionFilter) ([]SessionGroup, error) {
	var all []SessionInfo

	if sessions, err := listClaudeSessions(); err == nil {
		all = append(all, sessions...)
	}
	if sessions, err := listCodexSessions(); err == nil {
		all = append(all, sessions...)
	}
	if sessions, err := listGeminiSessions(); err == nil {
		all = append(all, sessions...)
	}
	if sessions, err := listOpenCodeSessions(); err == nil {
		all = append(all, sessions...)
	}
	if sessions, err := listCopilotSessions(); err == nil {
		all = append(all, sessions...)
	}

	// Apply filters
	cutoff := time.Time{}
	if filter.Since > 0 {
		cutoff = time.Now().Add(-filter.Since)
	}

	var filtered []SessionInfo
	for _, s := range all {
		if filter.Agent != "" && s.Agent != filter.Agent {
			continue
		}
		if filter.Project != "" && !strings.HasPrefix(s.ProjectPath, filter.Project) {
			continue
		}
		if !cutoff.IsZero() && s.ModifiedAt.Before(cutoff) {
			continue
		}
		filtered = append(filtered, s)
	}

	// Group by project path
	groupMap := map[string]*SessionGroup{}
	var order []string
	for _, s := range filtered {
		pp := s.ProjectPath
		if _, ok := groupMap[pp]; !ok {
			groupMap[pp] = &SessionGroup{ProjectPath: pp}
			order = append(order, pp)
		}
		groupMap[pp].Sessions = append(groupMap[pp].Sessions, s)
	}

	// Sort sessions within each group by ModifiedAt descending
	for _, g := range groupMap {
		sort.Slice(g.Sessions, func(i, j int) bool {
			return g.Sessions[i].ModifiedAt.After(g.Sessions[j].ModifiedAt)
		})
		if filter.Limit > 0 && len(g.Sessions) > filter.Limit {
			g.Sessions = g.Sessions[:filter.Limit]
		}
	}

	result := make([]SessionGroup, 0, len(order))
	for _, pp := range order {
		result = append(result, *groupMap[pp])
	}
	return result, nil
}
