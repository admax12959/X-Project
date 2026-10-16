# Code Review

Every change is reviewed by Codex (plugin `codex@openai-codex`) before its PR opens. The owner of the change runs the review; no one else is assumed to.

## Commands

The review commands are user-invoked only (`disable-model-invocation`): an agent cannot run them. When implementation is done, give the user the exact command and wait for the output.

| Command | Use when |
| --- | --- |
| `/codex:review --base main --scope branch --background` | Always. Standard review of the branch diff. |
| `/codex:adversarial-review --base main --scope branch --background <focus>` | The change touches identity, auth, permissions, data migration, or a public interface. Name the risk in `<focus>`, e.g. `email identity login boundary`. |

## When the user tells you to run it

The user may say to run the review yourself. Then:

- `cd` into the worktree of the branch under review first; the review targets the checked-out branch of the current directory.
- Run `node ~/.claude/plugins/cache/openai-codex/codex/<version>/scripts/codex-companion.mjs adversarial-review --base main --scope branch <focus>` with `run_in_background`. Plain `codex review --base main` takes no prompt, so use the companion for focus text.
- Fix findings in a worktree of their own, merge, rerun the review on the merged branch.

Follow progress with `/codex:status`, fetch the finished review with `/codex:result`, stop a stuck job with `/codex:cancel`.

Uncommitted work: swap `--scope branch` for `--scope working-tree`.

## Handling the output

Codex reviews only. Fixes happen afterwards, in the working session, never inside the review run.

Treat each finding as one of:

- **Fixed**: a commit addresses it.
- **Disputed**: the finding is wrong or out of scope; say why in one line.
- **Deferred**: real but not this PR; open an issue (see `issue-tracker.md`) and link it.

Re-run the review after fixes that touch the flagged code.

## Stop rule

Each round of an adversarial review finds a narrower race than the last. Any security finding blocks the merge: authentication or authorization bypass, access kept after a credential change, secret exposure, data loss. Everything else (availability races, deadlocks that fail one request, style) is Deferred to a follow-up issue so the loop ends.

## Before merging

Marking a PR ready triggers the Codex GitHub review. Read it (`gh api repos/<owner>/<repo>/pulls/<n>/comments`), fix what is security-relevant, defer the rest to the follow-up issue, and list the disposition in the PR description.

## Done

A review is done when every finding is Fixed, Disputed, or Deferred, and the PR description lists that disposition per finding. A PR with an unaddressed finding stays in draft.

## Optional: stop-time gate

`/codex:setup` can enable a Stop hook that has Codex review each Claude Code turn before it ends. Enable it for work on identity and auth; it adds latency to every turn.
