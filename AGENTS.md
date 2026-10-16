## Agent skills

### Issue tracker

Issues are tracked in GitHub Issues via the `gh` CLI. See `docs/agents/issue-tracker.md`.

### Triage labels

Uses the default five triage labels (`needs-triage`, `needs-info`, `ready-for-agent`, `ready-for-human`, `wontfix`). See `docs/agents/triage-labels.md`.

### Domain docs

Single-context: one `GLOSSARY.md` and `docs/adr/` at the repo root. See `docs/agents/domain.md`.

### Code review

Every change gets a Codex review before its PR opens: `/codex:review`, plus `/codex:adversarial-review` for identity, auth, permissions, migrations, and public interfaces. These are user-typed commands: when implementation is done, hand the user the exact command and wait for the findings, unless the user tells you to run the review yourself (how: `docs/agents/code-review.md`). Flags, finding disposition, stop rule, and the done criterion are there too. Before merging a PR, read the review comments Codex posts on it.

Coding rules that need judgement (for the reviewer): `CODING_STANDARDS.md`. Mechanical rules are enforced by `make check` (`scripts/check-repo.sh`); enable the hook once per clone with `git config core.hooksPath .githooks`.

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
- A cache miss rewrites the whole context at 1.25× input price, about
  what 12 cached turns cost. Two things cause one: switching models
  (`/model`), and a pause longer than the cache lifetime. Measured here:
  a 130k rewrite after `/model`, and three 120k rewrites after an
  overnight pause. Pick the model when the session starts.
- After a long pause, or once a phase of work is done, start a new
  session from a short handoff note (branch tips, open tickets, worktree
  paths) rather than resuming a large one.

## Token budget
Every turn re-reads the whole context, so cost is turns × context size.
Keep context small; output length barely matters.

Tool output
- Never `cat` whole files. Find the spot with `rg -n`, then read only
  that range (`Read` with offset/limit, or `sed -n A,Bp`). One file per call.
- Send noisy commands to a log and read the exit code, not a pipe's:
  `make check >/tmp/c.log 2>&1; echo $?`, then `tail -40 /tmp/c.log` only on
  failure (a trailing `| tail` would report tail's status). Hand a long
  failing log to the log-reader subagent instead of reading it here.
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
