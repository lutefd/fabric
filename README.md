# Fabric

Fabric is a repo-local coordination CLI for disposable agent threads. It records small pieces of project direction and surfaces the relevant subset to each thread at the right time, so agents do not have to rediscover the same constraints, rejected paths, or review feedback in every new conversation.

The command is `fabric`. The object it records is **direction**.

## Why Fabric

Agent threads are often short-lived. A human corrects one thread, a reviewer redirects another, and a third thread starts fresh with no memory of either. Fabric solves this by:

- Recording direction once, in a repo-local JSONL ledger.
- Matching direction to threads by issue, PR, or area.
- Surfacing only the relevant, recently-changed direction to each thread.
- Letting you review and curate direction after a task or PR is done.

## Core Ideas

### Direction

Direction is a short statement that changes what a future agent should do:

- "Don't create a second listing endpoint; extend the existing one."
- "Reviewer rejected picker-level Office special-casing."
- "Do not ingest every review comment as direction."

Fabric is not a transcript store. It is not for chat history, code snippets, or full prompts. It is for durable, reusable guidance.

### Threads

A thread is a scoped agent session. You start one with an issue, PR, or area:

```bash
fabric thread start --id thread-a --issue VS-123 --area virtual-store/listing
```

Fabric tracks what each thread has seen and marks threads stale when new relevant direction arrives.

### Durability

Direction has three tiers:

| Tier | What it means | Where it lives |
|------|---------------|----------------|
| **live** | Temporary; shared across active worktrees now. | Git-common shared mirror only. |
| **candidate** | Probably important; needs review before becoming durable. | Durable ledger `.fabric/ledger/events.jsonl`. |
| **durable** | Long-term project guidance; committed to Git. | Durable ledger `.fabric/ledger/events.jsonl`. |

When in doubt, record direction as a candidate. Promote it to durable only after review.

### Lifecycle Status

Direction also has a status:

| Status | Meaning |
|--------|---------|
| **active** | Current and actionable. |
| **expired** | Useful during a task or PR, but its window is over. |
| **discarded** | Too specific, noisy, or wrong. Kept for audit. |
| **superseded** | Replaced by newer direction. |

Inactive direction is hidden from normal sync, preflight, continue, handoff, and explain output. Use `fabric consolidate --include-inactive` to review it.

## The Full Loop

```text
Thread A receives human direction
-> Thread B syncs and avoids the wrong path
-> PR review redirects implementation
-> Thread C continues with review context
-> Thread C generates a handoff
-> PR is done
-> fabric consolidate --pr 123 proposes:
     promote reusable review direction
     expire temporary test checklist
     discard noisy comment
-> Human promotes one lesson
-> Next thread preflight sees durable direction
```

## Installation

Build the CLI:

```bash
go build -o fabric ./cmd/fabric
```

Install to `$GOPATH/bin`:

```bash
go install ./cmd/fabric
```

## Quick Start

Initialize Fabric in a repo:

```bash
fabric init
```

Install the agent protocol into `AGENTS.md`:

```bash
fabric install-agents
```

Start threads and record direction:

```bash
fabric thread start --id thread-a --issue VS-123 --area virtual-store/listing
fabric note --thread thread-a "Don't create a second listing endpoint; extend the existing one or escalate API direction"

fabric thread start --id thread-b --issue VS-123 --area virtual-store/listing
fabric sync --thread thread-b --budget 300
```

Read `.fabric/generated/SYNC_DELTA.md` to see what thread-b missed.

## Common Workflows

### Record Human Direction

```bash
fabric note --candidate "Direction that may matter later"
fabric note --durable "Long-term project guidance"
```

Interactive use without flags prompts for durability.

### Sync a Thread

```bash
fabric sync --thread thread-b --budget 300
```

Sync writes `.fabric/generated/SYNC_DELTA.md` with direction the thread has not yet seen.

### Preflight Before Starting Work

```bash
fabric preflight "add filtering to virtual-store listing" --issue VS-123 --area virtual-store/listing --budget 800
```

Preflight writes `.fabric/generated/TASK_DIRECTION.md` with relevant active direction for the task.

### Continue a PR or Issue

```bash
fabric continue --pr 123 --thread thread-c --budget 700
```

Continue writes `.fabric/generated/CONTINUATION_CONTEXT.md` with open challenges, review direction, live requirements, and issue direction.

### Record Review Direction

```bash
fabric review note --pr 123 --issue VS-123 --area file-opening \
  "Reviewer rejected picker-level Office special-casing; move unsupported file handling into the shared file-open resolver."
```

### Ingest PR Review from a Template

```bash
fabric ingest-pr template --pr 123 --issue VS-123 --area review-ingest
# Edit .fabric/generated/PR_REVIEW_INGEST.md
fabric ingest-pr --pr 123 --issue VS-123 --area review-ingest --from-file .fabric/generated/PR_REVIEW_INGEST.md
```

### Hand Off a PR

```bash
fabric handoff --pr 123 --budget 900
```

Handoff writes `.fabric/generated/HANDOFF.md` with current review direction, live requirements, open challenges, and rejected paths.

### Challenge Existing Direction

If a planned approach conflicts with active direction, record the dispute:

```bash
fabric challenge --direction evt_000001 --pr 123 --issue VS-123 \
  --proposal "New proposed path" \
  --reason "Why the existing direction may not apply"
```

Read `.fabric/generated/CHALLENGE.md`. Resolve later:

```bash
fabric challenge resolve evt_000003 --accepted
```

### Consolidate After a PR or Issue

When work finishes, review what should survive:

```bash
fabric consolidate --pr 123
# Read .fabric/generated/CONSOLIDATION.md
```

Then act on its suggestions:

```bash
fabric promote evt_000024 --reason "Reusable review-ingest product direction"
fabric expire evt_000025 --reason "PR-local checklist item completed"
fabric discard evt_000027 --reason "too specific to this PR"
fabric keep evt_000026 --candidate
```

### Inspect Ledger Health

```bash
fabric doctor
```

## Repo-local Files

`fabric init` creates:

```text
.fabric/
  config.yaml              # repo name, budgets, paths
  ledger/
    events.jsonl           # durable/candidate direction (commit this)
    threads.jsonl          # per-thread state (ignored by Git)
  active/                  # per-worktree runtime state (ignored by Git)
    issues/
    current-thread
  generated/               # checkpoint files for agents (ignored by Git)
    TASK_DIRECTION.md
    SYNC_DELTA.md
    CONTINUATION_CONTEXT.md
    CHALLENGE.md
    PR_REVIEW_INGEST.md
    HANDOFF.md
    CONSOLIDATION.md
  skills/                  # agent skill files (commit these)
    preflight/SKILL.md
    sync/SKILL.md
    note/SKILL.md
    continue/SKILL.md
    challenge/SKILL.md
    pr-review-ingest/SKILL.md
    consolidate-after-merge/SKILL.md
```

Git hygiene rule: commit project direction, not agent runtime state.

Tracked:

- `AGENTS.md`
- `.fabric/config.yaml`
- `.fabric/ledger/events.jsonl`
- `.fabric/skills/**/SKILL.md`

Ignored:

- `.fabric/active/**`
- `.fabric/generated/**`
- `.fabric/ledger/threads.jsonl`
- the git-common shared mirror (`.git/fabric/events.jsonl`)

## Commands

```bash
# Setup
fabric init
fabric install-agents

# Threads
fabric thread start --id thread-b --issue VS-123 --area virtual-store/listing

# Direction
fabric note "Prefer the existing endpoint"
fabric note --candidate "Direction that may matter later"
fabric note --durable "Long-term project guidance"
fabric review note --pr 123 --issue VS-123 --area file-opening "Reviewer direction"

# Context
fabric sync --thread thread-b --budget 300
fabric preflight "task text" --issue VS-123 --area virtual-store/listing --budget 800
fabric continue --pr 123 --thread thread-c --budget 700
fabric explain --issue VS-123
fabric explain --pr 123

# Review ingestion
fabric ingest-pr template --pr 123 --issue VS-123 --area review-ingest
fabric ingest-pr --pr 123 --issue VS-123 --area review-ingest --from-file review.md
fabric handoff --pr 123 --budget 900

# Challenges
fabric challenge --direction evt_000001 --pr 123 --issue VS-123 --proposal "New path" --reason "Why"
fabric challenge resolve evt_000003 --accepted

# Lifecycle consolidation
fabric consolidate --pr 123
fabric consolidate --issue VS-123
fabric promote evt_000024 --reason "Reusable review-ingest product direction"
fabric expire evt_000025 --reason "PR-local checklist item completed"
fabric discard evt_000027 --reason "too specific to this PR"
fabric keep evt_000026 --candidate

# Health
fabric doctor
```

## What Fabric Is Not

There is no database, server, daemon, LLM call, transcript storage, dashboard, automatic PR mining, webhook, or GitHub app. Fabric is intentionally small, local, and deterministic.

## Test

```bash
go test -cover ./...
```
