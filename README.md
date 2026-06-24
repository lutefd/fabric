# Fabric

Fabric is a provider-neutral repository decision and provenance protocol, with a small reference CLI for disposable agent threads. It records the decisions, findings, and constraints that should survive a conversation, then surfaces the relevant subset to each thread at the right time.

The command is `fabric`. Its protocol unit is a scoped **record**; the current storage model calls it a **direction event**.

Fabric is agent-first: agents are the primary protocol clients, and the CLI manages the repo-local protocol state. Provider skills adapt Codex and other agents to that protocol while Fabric remains independent from GitHub connectors and LLM providers.

The interoperable data model, matching rules, synchronization behavior, and provider contract are defined in [PROTOCOL.md](PROTOCOL.md).

## Why Fabric

The hard problem is not that an individual agent lacks enough context. It is that repository knowledge is fragmented across contexts that start, stop, and run concurrently.

A human corrects one thread, a reviewer redirects another, and a third thread starts fresh with no memory of either. One worktree discovers an architectural constraint while a sibling worktree continues down the rejected path. The code eventually records *what* was changed, but the reason for the choice remains trapped in a conversation or PR comment.

This creates four recurring failures.

### Decisions do not travel

Agent conversations, code review threads, issue comments, and worktrees are separate memory islands. A decision made in one is usually invisible to the others until a human repeats it. Parallel agent work increases the cost: adding threads creates more execution capacity while also creating more places for direction to diverge.

### Handoffs lose rationale

Ordinary summaries preserve the latest state but often discard why an approach was selected, which alternatives were rejected, and what evidence changed the plan. The next agent can continue the code, but it cannot reliably distinguish an intentional constraint from an accidental implementation detail.

### Repositories repeat mistakes

Without curated memory, each fresh thread can reopen a path already rejected in review, rediscover the same hidden constraint, or ask the human the same question. Transcripts are too large and noisy to use as project memory; they preserve everything without identifying what should change future behavior.

### Agent choices are difficult to explain

Today, a user can inspect a message or diff but often cannot answer: "Why did the agent choose this?" The useful answer is a provenance chain connecting the action to a decision, a finding from another thread, review feedback, and ultimately human direction or evidence.

Fabric makes that knowledge explicit and scoped:

- Record a decision, finding, or constraint once in repository-owned state.
- Propagate it immediately across active threads and sibling worktrees.
- Match it by issue, PR, or repository area instead of replaying all history.
- Preserve source, evidence, rationale, rejected paths, and preferred paths.
- Mark threads stale when new relevant direction arrives.
- Review temporary knowledge after a task and promote only reusable lessons.

The result is selective repository memory: smaller than a transcript, more durable than a chat summary, and portable across agent providers. A Codex thread, a Claude session, an IDE agent, and a future first-party integration should be able to participate in the same decision history without making any one provider the owner of it.

The long-term experience is straightforward: click an agent message or implementation choice and trace it back through the Fabric records that informed it. Fabric 0.1 establishes the shared ledger, scopes, thread cursors, evidence, and conflict model required for that experience; typed message-to-decision relationships are a later protocol extension.

## Protocol and CLI

The protocol is the product boundary. It defines the records, scopes, durability, lifecycle, synchronization, context projection, provenance, and client behavior that let independent agents coordinate.

The `fabric` CLI is the reference client. It keeps the protocol usable today without a server, daemon, database, or LLM call. Provider integrations can wrap the same operations in native hooks and interfaces while leaving the repository state portable and inspectable.

This separation matters: Fabric should not become a GitHub wrapper, a hosted transcript service, or a provider-specific memory feature. Connectors and skills acquire or publish external context; Fabric validates, stores, matches, and explains the curated repository knowledge.

## Core Ideas

### Records

A Fabric record is a short statement that changes what a future agent should do or explains why earlier work chose a path. Records include:

- **Direction:** a constraint future work should follow.
- **Finding:** evidence or repository knowledge another thread should not rediscover.
- **Decision:** a selected path with rationale and rejected alternatives.
- **Requirement:** a scoped condition that must be satisfied.
- **Challenge:** an explicit proposal to depart from active direction.

For example:

- "Don't create a second listing endpoint; extend the existing one."
- "Reviewer rejected picker-level Office special-casing."
- "The shared resolver already owns unsupported file handling."

Fabric is not a transcript store. It is not for chat history, code snippets, or full prompts. A record should be the smallest complete piece of knowledge that changes future work, with its source, rationale, and evidence when available.

### Threads

A thread is a scoped agent session. You start one with an issue, PR, or area:

```bash
fabric thread start --id thread-a --issue VS-123 --area virtual-store/listing
```

Fabric tracks what each thread has seen and marks threads stale when new relevant direction arrives.

### Durability

Records have three durability tiers:

| Tier | What it means | Where it lives |
|------|---------------|----------------|
| **live** | Temporary; shared across active worktrees now. | Git-common shared mirror only. |
| **candidate** | Probably important; needs review before becoming durable. | Durable ledger `.fabric/ledger/events.jsonl`. |
| **durable** | Long-term project guidance; committed to Git. | Durable ledger `.fabric/ledger/events.jsonl`. |

When in doubt, record reusable knowledge as a candidate. Promote it to durable only after review.

### Lifecycle Status

Each record also has a lifecycle status:

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

Initialization installs discoverable repository skills under `.agents/skills/`.

Install the agent protocol into `AGENTS.md` and install the managed Fabric skills globally for the current user under `~/.agents/skills/`:

```bash
fabric install-agents
```

The global installation refreshes only the namespaced `fabric-*` skills and preserves unrelated user skills. The repo-local copies remain checked in so agents can discover the protocol before global installation and so teams can review the exact workflows they adopt.

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

### Mine and Ingest PR Direction

Use `$fabric-pr-direction`. The skill prefers the agent's native GitHub connector and falls back directly to authenticated `gh`; Fabric itself does not wrap GitHub.

The skill presents extracted decisions for approval before it uses the existing template and dry-run workflow:

```bash
fabric ingest-pr template --pr 123 --issue VS-123 --area review-ingest
# Review .fabric/generated/PR_REVIEW_INGEST.md
fabric ingest-pr --pr 123 --issue VS-123 --area review-ingest \
  --from-file .fabric/generated/PR_REVIEW_INGEST.md --dry-run
# After explicit item selection:
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

`fabric init` creates the protocol state and discoverable agent skills:

```text
.agents/
  skills/
    fabric-session/
    fabric-record-direction/
    fabric-pr-direction/
    fabric-consolidate/
    fabric-publish/
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
```

Git hygiene rule: commit project direction, not agent runtime state.

Tracked:

- `AGENTS.md`
- `.agents/skills/**`
- `.fabric/config.yaml`
- `.fabric/ledger/events.jsonl`

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

There is no database, server, daemon, LLM call, transcript storage, dashboard, automatic PR mining, webhook, GitHub app, or Fabric-owned GitHub API layer. Agent skills use an available provider connector first and authenticated `gh` as the fallback. Fabric stays small, local, deterministic, and focused on direction state.

## Test

```bash
go test -cover ./...
```
