## Agent skills

### Issue tracker

Issues are tracked in GitHub Issues via the `gh` CLI. See `docs/agents/issue-tracker.md`.

### Triage labels

Uses the default five triage labels (`needs-triage`, `needs-info`, `ready-for-agent`, `ready-for-human`, `wontfix`). See `docs/agents/triage-labels.md`.

### Domain docs

Single-context: one `GLOSSARY.md` and `docs/adr/` at the repo root. See `docs/agents/domain.md`.

## Model routing
- Plan, refactor, cross-layer debugging: Opus 5.5
- Well-scoped changes: Sonnet 5.5
- Search, logs, test output, summaries: delegate to
  the log-reader / repo-search subagents (Haiku 5.5)
- Never let a Haiku subagent edit files
- Same failure twice at high effort: stop and ask
  before switching to Fable 5.1

## Cache
- Do not connect MCP servers or switch models mid-task
- Compact only at a natural break, and say what to keep
