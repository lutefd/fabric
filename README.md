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

The command is `fabric`. The thing it records is direction.

## Install locally

```bash
go build -o fabric ./cmd/fabric
```

## Quickstart

```bash
fabric init

fabric thread start \
  --id thread-a \
  --issue VS-123 \
  --area virtual-store/listing

fabric thread start \
  --id thread-b \
  --issue VS-123 \
  --area virtual-store/listing

fabric note \
  --thread thread-a \
  --issue VS-123 \
  --area virtual-store/listing \
  "Don't create a second listing endpoint; extend the existing one or escalate API direction"

fabric sync --thread thread-b --budget 300
```

## Repo-local files

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
    AGENTS_SNIPPET.md
  skills/
    preflight/SKILL.md
    sync/SKILL.md
    note/SKILL.md
```

There is no database, server, daemon, LLM call, transcript storage, dashboard, PR mining, or GitHub app in V0.

## Commands

```bash
fabric init
fabric thread start --id thread-b --issue VS-123 --area virtual-store/listing
fabric note --thread thread-a --issue VS-123 --area virtual-store/listing "Prefer the existing endpoint"
fabric sync --thread thread-b --budget 300
fabric preflight "add filtering to virtual-store listing" --issue VS-123 --area virtual-store/listing --budget 800
fabric explain --issue VS-123
```

## Test

```bash
go test ./...
```
