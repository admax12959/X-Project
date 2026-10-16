## Agent skills

### Issue tracker

Issues are tracked in GitHub Issues via the `gh` CLI. See `docs/agents/issue-tracker.md`.

### Triage labels

Uses the default five triage labels (`needs-triage`, `needs-info`, `ready-for-agent`, `ready-for-human`, `wontfix`). See `docs/agents/triage-labels.md`.

### Domain docs

Single-context: one `GLOSSARY.md` and `docs/adr/` at the repo root. See `docs/agents/domain.md`.

### Code review

Every change gets a Codex review before its PR opens: `/codex:review`, plus `/codex:adversarial-review` for identity, auth, permissions, migrations, and public interfaces. These are user-typed commands, so when implementation is done, hand the user the exact command and wait for the findings. Flags, finding disposition, and the done criterion: `docs/agents/code-review.md`.

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

## Token budget
Every turn re-reads the whole context, so cost is turns × context size.
Keep context small; output length barely matters.

Tool output
- Never `cat` whole files. Find the spot with `rg -n`, then read only
  that range (`Read` with offset/limit, or `sed -n A,Bp`). One file per call.
- Pipe noisy commands through a filter: `make check 2>&1 | tail -40`,
  `go test ./... 2>&1 | grep -E 'FAIL|panic|---' | head -40`. On failure,
  hand the full log to the log-reader subagent instead of reading it here.
- Cap listings: `git log --oneline -15`, `git diff --stat` before
  `git diff`, `gh ... --json <fields>` instead of full bodies.
- Never grep or read session transcripts (`~/.claude/projects/**.jsonl`).

Browser checks
- `preview_snapshot` without `includeImage` by default; one screenshot
  per screen, only when the layout itself is in question. Never `Read`
  the screenshot file again.
- Do UI verification once per ticket, at the end, not after each edit.

Subagents
- Each subagent starts with ~60k tokens of tools and instructions before
  it reads anything, unless its definition restricts `tools:` (then ~6k;
  measured). Always spawn implementers as `subagent_type: implementer`
  (`.claude/agents/implementer.md`), never `general-purpose`.
- Spawn one only for a whole ticket, not small steps. Start from
  `docs/agents/codemap.md`; ticket work spent about 60 of 110 turns
  re-reading the same files it describes.
- Running two at once does not lower the total, only the burst. The total
  is what hits the session limit (one ticket ≈ 12–17M cache-read tokens).
- Run at most 2 implementer subagents at once.
- The brief carries what the subagent would otherwise rediscover:
  the ticket number, which dependencies are already merged, the files and
  seams to touch, prior art to copy, and the verify command. Tell it not
  to re-read the spec beyond its own ticket.
- A subagent that passes ~250k context or ~120 turns stops, commits WIP,
  and reports where it is, so it can be restarted with a fresh brief.
- Commit WIP at every green step, so a usage-limit interruption loses
  nothing.

Main session
- Coordinate, don't implement: merge, run checks, update the PR.
- Compact after each merged ticket (above ~200k), keeping branch tips,
  open tickets and worktree paths.
