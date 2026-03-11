package sync

import (
	"os"
	"regexp"
	"strings"

	"github.com/scottatron/maestron/internal/agents"
	"github.com/scottatron/maestron/internal/manage"
)

// SyncResult describes the outcome of syncing a single integration.
type SyncResult struct {
	Integration string
	Path        string
	Written     bool
	Skipped     bool // integration not enabled or no renderer
	Err         error
}

// Sync generates per-integration config files from the merged MCP config.
// If dryRun is true, files are not written. If integration is non-empty, only
// that integration is synced.
func Sync(projectRoot string, dryRun bool, integration string) ([]SyncResult, error) {
	// Load project config
	cfg, err := manage.ReadAgentsConfig(projectRoot)
	if err != nil {
		return nil, err
	}

	// Load local overrides (non-fatal if missing)
	lo, _ := manage.ReadLocalOverrides(projectRoot)

	// Load global config
	gc, _ := manage.ReadGlobalMcpConfig()

	// Merge: global → project → local
	merged := mergeServers(gc, cfg, lo)

	// Resolve variables
	for name, def := range merged {
		merged[name] = resolveServerVars(def, projectRoot)
	}

	// Determine which integrations to sync
	integrations := cfg.Integrations.Enabled
	if integration != "" {
		integrations = []string{integration}
	}

	var results []SyncResult
	for _, integ := range integrations {
		renderer, ok := integrationRenderers[integ]
		if !ok {
			results = append(results, SyncResult{
				Integration: integ,
				Skipped:     true,
			})
			continue
		}

		// Filter servers for this integration
		servers := filterServersForIntegration(merged, integ)

		out, err := renderer(projectRoot, servers)
		if err != nil {
			results = append(results, SyncResult{
				Integration: integ,
				Path:        out.Path,
				Err:         err,
			})
			continue
		}

		if dryRun {
			results = append(results, SyncResult{
				Integration: integ,
				Path:        out.Path,
				Written:     false,
			})
			continue
		}

		if err := writeFile(out.Path, out.Data); err != nil {
			results = append(results, SyncResult{
				Integration: integ,
				Path:        out.Path,
				Err:         err,
			})
			continue
		}

		results = append(results, SyncResult{
			Integration: integ,
			Path:        out.Path,
			Written:     true,
		})
	}

	return results, nil
}

// mergeServers merges global → project → local (field-level merge, not full replace).
// For each server present in project's agents.json that also exists in global:
// project values override global values only for non-zero/non-nil fields.
// Local overlays env/headers/args on top.
func mergeServers(gc *agents.GlobalMcpConfig, cfg *agents.AgentsConfig, lo *manage.LocalOverrides) map[string]agents.MCPServerDef {
	result := make(map[string]agents.MCPServerDef)

	// Global servers (base definitions)
	if gc != nil {
		for k, v := range gc.MCPServers {
			result[k] = v
		}
	}

	// Project servers: field-level merge over global
	if cfg != nil {
		for k, proj := range cfg.MCP.Servers {
			base, hasGlobal := result[k]
			if !hasGlobal {
				// Not in global — use project definition as-is
				result[k] = proj
				continue
			}
			// Field-level merge: project overrides non-zero fields
			merged := base
			if proj.Label != "" {
				merged.Label = proj.Label
			}
			if proj.Description != "" {
				merged.Description = proj.Description
			}
			if proj.Transport != "" {
				merged.Transport = proj.Transport
			}
			if proj.Command != "" {
				merged.Command = proj.Command
			}
			if len(proj.Args) > 0 {
				merged.Args = proj.Args
			}
			if proj.URL != "" {
				merged.URL = proj.URL
			}
			if len(proj.Env) > 0 {
				if merged.Env == nil {
					merged.Env = make(map[string]string)
				}
				for k2, v2 := range proj.Env {
					merged.Env[k2] = v2
				}
			}
			if len(proj.Headers) > 0 {
				if merged.Headers == nil {
					merged.Headers = make(map[string]string)
				}
				for k2, v2 := range proj.Headers {
					merged.Headers[k2] = v2
				}
			}
			if proj.Enabled != nil {
				merged.Enabled = proj.Enabled
			}
			result[k] = merged
		}
	}

	// Local overrides (highest priority) — overlay specific fields
	if lo != nil {
		for name, override := range lo.MCPServers {
			def, ok := result[name]
			if !ok {
				continue
			}
			if len(override.Env) > 0 {
				if def.Env == nil {
					def.Env = make(map[string]string)
				}
				for k, v := range override.Env {
					def.Env[k] = v
				}
			}
			if len(override.Headers) > 0 {
				if def.Headers == nil {
					def.Headers = make(map[string]string)
				}
				for k, v := range override.Headers {
					def.Headers[k] = v
				}
			}
			if len(override.Args) > 0 {
				def.Args = override.Args
			}
			result[name] = def
		}
	}

	return result
}

// filterServersForIntegration returns all enabled servers.
func filterServersForIntegration(servers map[string]agents.MCPServerDef, _ string) map[string]agents.MCPServerDef {
	result := make(map[string]agents.MCPServerDef)
	for name, def := range servers {
		if agents.IsEnabled(def.Enabled) {
			result[name] = def
		}
	}
	return result
}

var varPattern = regexp.MustCompile(`\$\{([^}]+)\}`)

// resolveServerVars replaces ${PROJECT_ROOT} and ${VAR} in server definition fields.
func resolveServerVars(def agents.MCPServerDef, projectRoot string) agents.MCPServerDef {
	resolve := func(s string) string {
		s = strings.ReplaceAll(s, "${PROJECT_ROOT}", projectRoot)
		return varPattern.ReplaceAllStringFunc(s, func(match string) string {
			varName := match[2 : len(match)-1]
			if val := os.Getenv(varName); val != "" {
				return val
			}
			return match
		})
	}

	def.Command = resolve(def.Command)
	def.URL = resolve(def.URL)

	resolvedArgs := make([]string, len(def.Args))
	for i, a := range def.Args {
		resolvedArgs[i] = resolve(a)
	}
	def.Args = resolvedArgs

	if len(def.Env) > 0 {
		resolvedEnv := make(map[string]string, len(def.Env))
		for k, v := range def.Env {
			resolvedEnv[k] = resolve(v)
		}
		def.Env = resolvedEnv
	}

	return def
}

// writeFile writes data to path, creating parent directories as needed.
func writeFile(path string, data []byte) error {
	if err := os.MkdirAll(dirOf(path), 0755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func dirOf(path string) string {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' || path[i] == '\\' {
			return path[:i]
		}
	}
	return "."
}
