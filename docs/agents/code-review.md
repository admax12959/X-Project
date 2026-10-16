# Code Review

Every change is reviewed by Codex (plugin `codex@openai-codex`) before its PR opens. The owner of the change runs the review; no one else is assumed to.

## Commands

The review commands are user-invoked only (`disable-model-invocation`): an agent cannot run them. When implementation is done, give the user the exact command and wait for the output.

| Command | Use when |
| --- | --- |
| `/codex:review --base main --scope branch --background` | Always. Standard review of the branch diff. |
| `/codex:adversarial-review --base main --scope branch --background <focus>` | The change touches identity, auth, permissions, data migration, or a public interface. Name the risk in `<focus>`, e.g. `email identity login boundary`. |

Follow progress with `/codex:status`, fetch the finished review with `/codex:result`, stop a stuck job with `/codex:cancel`.

Uncommitted work: swap `--scope branch` for `--scope working-tree`.

## Handling the output

Codex reviews only. Fixes happen afterwards, in the working session, never inside the review run.

Treat each finding as one of:

- **Fixed**: a commit addresses it.
- **Disputed**: the finding is wrong or out of scope; say why in one line.
- **Deferred**: real but not this PR; open an issue (see `issue-tracker.md`) and link it.

Re-run the review after fixes that touch the flagged code.

## Done

A review is done when every finding is Fixed, Disputed, or Deferred, and the PR description lists that disposition per finding. A PR with an unaddressed finding stays in draft.

## Optional: stop-time gate

`/codex:setup` can enable a Stop hook that has Codex review each Claude Code turn before it ends. Enable it for work on identity and auth; it adds latency to every turn.
