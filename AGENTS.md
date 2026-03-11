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
