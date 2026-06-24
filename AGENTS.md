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
