# Fabric

Fabric is a repo-local coordination CLI for disposable agent threads.

V0 proves one loop:

```text
Human corrects Thread A
-> Fabric records the correction
-> Thread B is marked stale
-> Thread B syncs
-> Thread B receives a tiny relevant delta
-> Thread B avoids the same wrong path
```

The command is `fabric`. The object it records is direction.

V1 adds a narrow PR/review continuation loop:

```text
Reviewer redirects a PR
-> Fabric records that explicit review direction
-> A fresh or follow-up thread runs fabric continue
-> The thread gets a small continuation packet
-> The thread addresses review without replaying the whole PR
```

## Demo

Build the CLI:

```bash
go build -o fabric ./cmd/fabric
```

Run the V0 validation loop:

```bash
fabric init
fabric thread start --id thread-a --issue VS-123 --area virtual-store/listing
fabric thread start --id thread-b --issue VS-123 --area virtual-store/listing
fabric note --thread thread-a --issue VS-123 --area virtual-store/listing "Don't create a second listing endpoint; extend the existing one or escalate API direction"
fabric sync --thread thread-b --budget 300
```

Run the V1 PR/review continuation loop:

```bash
fabric init
fabric thread start --id thread-review-fix --issue VS-123 --area file-opening
fabric note --issue VS-123 --area file-opening "Do not implement full Office preview unless the task is explicitly rescoped."
fabric review note --pr 123 --issue VS-123 --area file-opening "Reviewer rejected picker-level Office special-casing; move unsupported file handling into the shared file-open resolver."
fabric continue --pr 123 --thread thread-c --budget 700
fabric explain --pr 123
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
    AGENTS_SNIPPET.md
  skills/
    preflight/SKILL.md
    sync/SKILL.md
    note/SKILL.md
```

There is no database, server, daemon, LLM call, transcript storage, dashboard, automatic PR mining, webhook, or GitHub app.

## Commands

```bash
fabric init
fabric thread start --id thread-b --issue VS-123 --area virtual-store/listing
fabric note --thread thread-a --issue VS-123 --area virtual-store/listing "Prefer the existing endpoint"
fabric review note --pr 123 --issue VS-123 --area file-opening "Reviewer rejected picker-level Office special-casing; move unsupported file handling into the shared file-open resolver."
fabric sync --thread thread-b --budget 300
fabric preflight "add filtering to virtual-store listing" --issue VS-123 --area virtual-store/listing --budget 800
fabric continue --pr 123 --thread thread-c --budget 700
fabric explain --issue VS-123
fabric explain --pr 123
```

## Test

```bash
go test -cover ./...
```
