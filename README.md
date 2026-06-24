# Fabric

Fabric is a repo-local coordination CLI for disposable agent threads. It records small pieces of project direction and surfaces the relevant subset to each thread at the right time.

The core loop:

```text
Human corrects Thread A
-> Fabric records the correction
-> Thread B is marked stale
-> Thread B syncs
-> Thread B receives a tiny relevant delta
-> Thread B avoids the same wrong path
```

The command is `fabric`. The object it records is direction.

Direction has three durabilities:

- **live** — shared across active worktrees now, not persisted to the durable ledger
- **candidate** — probably important, but needs review before becoming durable
- **durable** — long-term project guidance, committed to `.fabric/ledger/events.jsonl`

Direction also has a lifecycle status:

- **active** — current and actionable
- **expired** — useful during a task/PR, but its window is over
- **discarded** — too specific, noisy, or wrong
- **superseded** — replaced by newer direction

## Demo

Build the CLI:

```bash
go build -o fabric ./cmd/fabric
```

Run the basic sync loop:

```bash
fabric init
fabric thread start --id thread-a --issue VS-123 --area virtual-store/listing
fabric thread start --id thread-b --issue VS-123 --area virtual-store/listing
fabric note --thread thread-a --issue VS-123 --area virtual-store/listing "Don't create a second listing endpoint; extend the existing one or escalate API direction"
fabric sync --thread thread-b --budget 300
```

Run the PR/review continuation loop:

```bash
fabric init
fabric thread start --id thread-review-fix --issue VS-123 --area file-opening
fabric note --issue VS-123 --area file-opening "Do not implement full Office preview unless the task is explicitly rescoped."
fabric review note --pr 123 --issue VS-123 --area file-opening "Reviewer rejected picker-level Office special-casing; move unsupported file handling into the shared file-open resolver."
fabric continue --pr 123 --thread thread-c --budget 700
fabric explain --pr 123
```

When a PR or issue is done, consolidate its direction:

```bash
fabric consolidate --pr 123
# Review .fabric/generated/CONSOLIDATION.md
# Then promote useful lessons or expire temporary ones:
fabric promote evt_000024 --reason "Reusable review-ingest product direction"
fabric expire evt_000025 --reason "PR-local checklist item completed"
```

Or run the demo script from a scratch directory:

```bash
scripts/demo-v0-loop.sh
```

Expected sync delta:

```md
# Sync Delta

Thread:
thread-b

New relevant direction since last sync:

1. Don't create a second listing endpoint; extend the existing one or escalate API direction

Source:
Human note from related thread thread-a.

Why it applies:
- Same issue: VS-123
- Same area: virtual-store/listing

Next action:
Adjust your plan before continuing.
```

## Repo-local Files

`fabric init` creates:

```text
.fabric/
  config.yaml
  ledger/
    events.jsonl
    threads.jsonl
  active/
    issues/
  generated/
    TASK_DIRECTION.md
    SYNC_DELTA.md
    CONTINUATION_CONTEXT.md
    CHALLENGE.md
    PR_REVIEW_INGEST.md
    HANDOFF.md
    CONSOLIDATION.md
  skills/
    preflight/SKILL.md
    sync/SKILL.md
    note/SKILL.md
    continue/SKILL.md
    challenge/SKILL.md
    pr-review-ingest/SKILL.md
    consolidate-after-merge/SKILL.md
```

There is no database, server, daemon, LLM call, transcript storage, dashboard, automatic PR mining, webhook, or GitHub app.

## Commands

```bash
fabric init
fabric install-agents
fabric thread start --id thread-b --issue VS-123 --area virtual-store/listing
fabric note --thread thread-a --issue VS-123 --area virtual-store/listing "Prefer the existing endpoint"
fabric note --candidate "Direction that may matter later"
fabric note --durable "Long-term project guidance"
fabric review note --pr 123 --issue VS-123 --area file-opening "Reviewer rejected picker-level Office special-casing; move unsupported file handling into the shared file-open resolver."
fabric sync --thread thread-b --budget 300
fabric preflight "add filtering to virtual-store listing" --issue VS-123 --area virtual-store/listing --budget 800
fabric continue --pr 123 --thread thread-c --budget 700
fabric challenge --direction evt_000001 --pr 123 --issue VS-123 --proposal "New path" --reason "Why the existing direction may not apply"
fabric challenge resolve evt_000003 --accepted
fabric explain --issue VS-123
fabric explain --pr 123
fabric ingest-pr template --pr 123 --issue VS-123 --area review-ingest
fabric ingest-pr --pr 123 --issue VS-123 --area review-ingest --from-file review.md
fabric handoff --pr 123 --budget 900
fabric consolidate --pr 123
fabric consolidate --issue VS-123
fabric promote evt_000024 --reason "Reusable review-ingest product direction"
fabric expire evt_000025 --reason "PR-local checklist item completed"
fabric discard evt_000027 --reason "too specific to this PR"
fabric keep evt_000026 --candidate
fabric doctor
```

## Test

```bash
go test -cover ./...
```
