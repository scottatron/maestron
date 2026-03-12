# maestron — agent instructions

## Commits

All commits MUST use a conventional commit prefix:

```
feat:     new feature
fix:      bug fix
docs:     documentation only
refactor: code restructuring (no behaviour change)
test:     add or fix tests
chore:    build, deps, tooling, CI
perf:     performance improvement
ci:       CI/CD changes
```

Breaking changes append `!` after the type/scope: `feat!:` or `feat(mcp)!:`

**No commits without a prefix. No exceptions.**

### Why this matters

The CI release pipeline infers the next version number directly from commit
prefixes. Getting them wrong produces the wrong version bump:

| Commit type | Version bump |
|-------------|-------------|
| `feat!:` / `BREAKING CHANGE:` | major (`v1.0.0`) |
| `feat:` | minor (`v0.5.0`) |
| `fix:`, `chore:`, `docs:`, etc. | patch (`v0.4.3`) |

Every push to `main` builds an edge release tagged `v{next}-edge` using this
logic. The tag tells you exactly what the next stable release will be — so a
`chore:` commit that introduces a new feature will silently produce a patch
release instead of a minor one.

Examples:
```
feat(mcp): add consolidate subcommand
fix(sync): resolve ${PROJECT_ROOT} in env values
chore: add GitHub Actions release workflow
```

## Code style

- Go standard formatting (`gofmt`)
- Keep functions small and focused
- Errors returned, not panicked
- Prefer explicit over clever
