<!-- fabric:start -->
# Direction Fabric Protocol

Before starting work:
- Run fabric status.
- Run fabric preflight for the task.
- Read .fabric/generated/TASK_DIRECTION.md.

Direction events are shared repo state. Do not delete .fabric/ledger/events.jsonl or treat each worktree as isolated; sibling worktrees should hear relevant notes through Fabric.

Before major checkpoints:
- Run fabric sync.
- Read .fabric/generated/SYNC_DELTA.md.

When corrected by the human:
- Run fabric note "<direction>".

Direction has three tiers:
- live: shared across active worktrees now, not persisted to the durable ledger
- candidate: probably important, but needs review before becoming durable
- durable: long-term project guidance, committed to .fabric/ledger/events.jsonl

For interactive use, run fabric note and choose the durability.
For agent use (non-interactive), prefer:
  fabric note --candidate "<direction>"
unless the human explicitly says it is temporary or durable.

Promote a candidate later:
  fabric promote <event-id>

Check ledger health:
  fabric doctor

Git hygiene rule:
  Commit project direction, not agent runtime state.

Tracked:
- AGENTS.md
- .fabric/config.yaml
- .fabric/skills/**
- .fabric/ledger/events.jsonl (candidate/durable project direction only)

Ignored:
- .fabric/active/**
- .fabric/generated/**
- .fabric/ledger/threads.jsonl
- the git-common shared mirror (.git/fabric/events.jsonl)

When continuing PR/review work:
- Run fabric continue --pr "<pr>".
- Read .fabric/generated/CONTINUATION_CONTEXT.md.

When PR review redirects the implementation, record it explicitly:

fabric review note --pr "<pr>" --issue "<issue>" --area "<area>" "<review direction>"

If your planned approach conflicts with active direction, do not silently proceed.

Choose one:

1. Align with existing direction
2. Ask for a scoped exception
3. Record a challenge

To record a challenge, run:

fabric challenge \
  --direction "<event-id>" \
  --issue "<issue>" \
  --pr "<pr>" \
  --area "<area>" \
  --proposal "<new proposed path>" \
  --reason "<why the existing direction may not apply>"

Read:

.fabric/generated/CHALLENGE.md

Mention the challenge in the PR or handoff.
<!-- fabric:end -->
