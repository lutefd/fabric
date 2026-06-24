# Direction Fabric Protocol

Status: Draft 0.1  
Reference implementation: `fabric` CLI

Direction Fabric is a provider-neutral protocol for sharing repository decisions,
findings, and constraints across agent threads and worktrees. It defines how an
agent discovers applicable direction, publishes new knowledge, synchronizes with
other work, records disagreement, and explains why a later action was taken.

The CLI in this repository is the reference protocol client. It is not the
protocol boundary. Codex, Claude, Cursor, CI jobs, IDEs, and future tools can all
participate as clients if they implement the behavior described here.

This document has two purposes:

1. Specify the interoperable profile implemented by Fabric today.
2. Establish the compatibility boundary for richer provenance and first-party
   provider integrations without pretending those extensions already exist.

The words **MUST**, **MUST NOT**, **SHOULD**, **SHOULD NOT**, and **MAY** describe
protocol requirements.

## 1. Problem Statement

Repository work increasingly happens in several short-lived agent contexts at
once. Those contexts do not share a reliable memory:

- A human corrects one thread, but a sibling worktree continues with the old
  assumption.
- A review rejects an approach, but the next agent reopens it because the reason
  lived only in a PR conversation.
- One thread discovers a repository constraint, while another spends time
  rediscovering it.
- A later user can inspect what an agent changed, but not which earlier decision,
  finding, or human instruction caused that choice.

Larger prompts and transcript archives do not solve this. They move more text,
not necessarily the small set of facts that should change future behavior.
Fabric therefore treats repository memory as curated protocol state rather than
conversation history.

## 2. Goals

Fabric has six protocol goals:

1. **Propagation:** relevant knowledge from one thread becomes available to
   other threads and worktrees before they act.
2. **Selection:** clients receive a scoped, budgeted context projection rather
   than the entire repository history.
3. **Persistence:** reusable project decisions survive disposable conversations.
4. **Provenance:** records preserve enough source, evidence, and rationale to
   explain why guidance exists.
5. **Conflict visibility:** an agent never silently violates active direction;
   disagreement becomes explicit protocol state.
6. **Provider independence:** repository knowledge remains usable across agent
   vendors, IDEs, and hosting platforms.

## 3. Non-goals

The protocol is not:

- A transcript or prompt archive.
- A replacement for Git, issue trackers, or code review systems.
- A database of every fact an agent observed.
- An autonomous policy engine that resolves disagreement without humans.
- A provider-specific memory feature.
- A requirement for a hosted service, daemon, or LLM call.

External systems remain evidence sources and publication targets. Provider
adapters acquire context from those systems; the core protocol stores only the
curated records needed by future repository work.

## 4. Protocol Model

Fabric separates five concepts.

### 4.1 Repository

A repository is the ownership and portability boundary for Fabric state. Durable
records travel with the repository through Git. Active worktrees in the same Git
repository also share an uncommitted runtime mirror.

### 4.2 Thread

A thread is one agent execution context. It has:

- A stable `thread_id`.
- A scope containing an issue, PR, and/or one or more repository areas.
- A cursor identifying the latest relevant event it has consumed.

A thread may correspond to a chat, an IDE task, a background agent, or another
provider-specific session. Provider identifiers MAY be used as `thread_id`
values, but the protocol does not require a particular provider.

### 4.3 Record

A record is a small, scoped statement that can change future work or explain a
past choice. The current storage representation calls records **direction
events**.

The useful semantic classes are:

- **Direction:** a constraint or instruction future work should follow.
- **Finding:** an observation another thread should not need to rediscover.
- **Decision:** a chosen path, including rationale and rejected alternatives.
- **Requirement:** a scoped condition that must be satisfied.
- **Challenge:** an explicit proposal to depart from active direction.
- **Resolution:** the accepted or rejected outcome of a challenge.

The 0.1 implementation uses the open `kind` field. It currently emits `note`,
`review_direction`, `review_requirement`, `challenge`, and
`challenge_resolution`. Clients MAY emit more specific kinds such as `finding`
and `decision`; 0.1 readers treat unknown kinds as ordinary scoped records.

### 4.4 Evidence

Evidence identifies the material that supports a record: a PR comment, issue,
human note, test result, document, commit, or other source. Evidence is not a
copy of the full source. It is a compact reference with enough context to audit
the record.

### 4.5 Context Projection

A context projection is a generated, bounded view of relevant active records for
one task or thread. `TASK_DIRECTION.md`, `SYNC_DELTA.md`,
`CONTINUATION_CONTEXT.md`, and `HANDOFF.md` are projection formats, not
authoritative state.

## 5. Direction Event Schema

The 0.1 interoperable representation is one JSON object per line.

```json
{
  "id": "evt_000015",
  "kind": "decision",
  "created_at": "2026-06-24T17:55:11-03:00",
  "scope": {
    "repo": "fabric",
    "issue": "FAB-5",
    "pr": "12",
    "areas": ["agent-protocol"],
    "global": false
  },
  "source": {
    "type": "human",
    "thread_id": "thread_20260624_1755",
    "pr": "12",
    "url": "https://example.test/pull/12#discussion"
  },
  "text": "Keep Fabric provider-neutral; integrations adapt providers to the protocol.",
  "confidence": "human_confirmed",
  "ttl": "until_issue_closed",
  "status": "active",
  "durability": "durable",
  "reason": "Repository decisions must survive movement between agent providers.",
  "rejected_paths": ["Make the ledger a Codex-only memory store"],
  "preferred_paths": ["Keep provider behavior in adapters and skills"],
  "evidence": [
    {
      "type": "thread_message",
      "url": "https://example.test/threads/abc/messages/42",
      "author": "human",
      "text": "Provider integrations should use a shared protocol."
    }
  ]
}
```

### 5.1 Required fields

| Field | Requirement |
|---|---|
| `id` | Stable repository-local event identifier. Current form is `evt_NNNNNN`. |
| `kind` | Semantic record kind. Unknown kinds MUST NOT make a record unreadable. |
| `created_at` | RFC 3339 timestamp with offset. |
| `scope` | Object defining where the record applies. |
| `source` | Object identifying how the record entered Fabric. |
| `text` | Smallest complete actionable statement. |
| `confidence` | Basis for trusting the statement. |
| `ttl` | Intended validity window. |
| `durability` | `live`, `candidate`, or `durable`. |

`status` SHOULD be written explicitly. A missing `status` is interpreted as
`active`. For compatibility with early ledgers, a missing `durability` is
interpreted as `durable`.

### 5.2 Scope

The scope object supports:

| Field | Meaning |
|---|---|
| `repo` | Repository name. |
| `issue` | Issue key or identifier. |
| `pr` | Pull request identifier. |
| `areas` | Exact repository-owned area labels. |
| `global` | Whether the record applies to every thread in the repository. |

A record MUST contain at least one useful scope dimension. Global records SHOULD
be rare because they consume every thread's context budget.

The 0.1 matching rule is an OR across dimensions. A record is relevant when it
is global, has the same non-empty issue, has the same non-empty PR, or shares at
least one exact non-empty area.

Area matching is exact in 0.1. Hierarchical or fuzzy area matching requires a
future protocol version or an explicit repository convention.

### 5.3 Source

The source object supports:

| Field | Meaning |
|---|---|
| `type` | Origin class such as `human`, `review`, or `pr_ingest`. |
| `thread_id` | Thread that created the record. |
| `pr` | Source PR when applicable. |
| `url` | Stable external source URL when available. |

Clients SHOULD preserve a source URL for externally acquired records and a
thread ID for thread-originated records. A source reference is provenance, not
proof by itself; evidence and rationale provide the rest of the explanation.

### 5.4 Optional explanation fields

| Field | Meaning |
|---|---|
| `reason` | Why the direction, finding, or decision matters. |
| `review_type` | Review classification such as rejection, preference, or requirement. |
| `rejected_paths` | Approaches future agents should not reopen without new evidence. |
| `preferred_paths` | Recommended approaches. |
| `evidence` | Compact evidence references. |
| `challenges` | Event ID challenged by this record. |
| `lifecycle_reason` | Why a curator promoted, expired, discarded, or retained the record. |
| `reviewed_at` | RFC 3339 time of the latest lifecycle review. |

Clients SHOULD include rationale whenever a bare instruction would cause a later
agent to repeat the original investigation.

### 5.5 Confidence

Confidence describes provenance, not a numerical probability. Current standard
values are:

- `human_confirmed`: directly established by a human.
- `reviewer_confirmed`: extracted from explicit review feedback.

Future clients MAY add values such as `agent_observed` or `tool_verified`.
Unknown confidence values MUST remain readable. Agent-generated findings SHOULD
include evidence and SHOULD default to candidate durability until reviewed.

## 6. Durability, Status, and TTL

These are independent dimensions.

### 6.1 Durability

| Value | Semantics | Storage requirement |
|---|---|---|
| `live` | Temporary coordination for active work. | Shared runtime mirror; not required in Git. |
| `candidate` | Potentially reusable, awaiting curation. | Durable ledger and shared mirror. |
| `durable` | Reviewed repository knowledge. | Durable ledger and shared mirror. |

Agents SHOULD default uncertain reusable knowledge to `candidate`. They MUST NOT
promote every conversation, review comment, or local task detail to `durable`.

### 6.2 Status

| Value | Semantics |
|---|---|
| `active` | Eligible for ordinary context projections. |
| `expired` | Validity window ended. |
| `discarded` | Noisy, incorrect, or not useful as repository direction. |
| `superseded` | Replaced by newer direction. |

Challenge records additionally use `open`, `accepted`, and `rejected` in the
0.1 implementation. Ordinary projections MUST exclude inactive records unless
the caller explicitly asks to inspect or consolidate them.

### 6.3 TTL

`ttl` communicates expected validity, for example `until_issue_closed`,
`until_pr_closed`, or `until_challenge_resolved`. In 0.1, TTL is advisory:
clients and humans perform explicit lifecycle transitions. A client MUST NOT
silently delete a record because its TTL appears to have elapsed.

## 7. Thread State

Thread state uses JSONL records with this shape:

```json
{
  "thread_id": "thread-a",
  "created_at": "2026-06-24T18:00:00-03:00",
  "issue": "FAB-5",
  "pr": "12",
  "areas": ["agent-protocol"],
  "last_seen_event_id": "evt_000015"
}
```

The latest record for a `thread_id` is its materialized state. Thread state is
runtime coordination data and MUST NOT be treated as durable project direction.

When a relevant active event has an ID after a thread's cursor, that thread is
stale. Synchronization advances the cursor only after producing the context
projection for the thread.

## 8. Storage and Synchronization

The reference repository layout is:

```text
.fabric/ledger/events.jsonl       durable and candidate records
.fabric/ledger/threads.jsonl      per-worktree thread cursors
.fabric/active/                   per-worktree runtime state
.fabric/generated/                generated context projections
<git-common-dir>/fabric/events.jsonl
                                  shared live mirror across worktrees
<git-common-dir>/fabric/lock      shared writer lock
```

### 8.1 Authority

- `.fabric/ledger/events.jsonl` is the Git-tracked source for candidate and
  durable repository knowledge.
- The Git-common mirror is the immediate coordination source for active
  worktrees, including uncommitted live records.
- Reads combine both sources and deduplicate by event ID.
- Per-worktree thread and generated files are never authoritative direction.

A client MUST NOT invent a worktree-local fallback ledger when access to the
Git-common mirror is blocked. It must report the failure or request the required
permission; otherwise sibling worktrees silently diverge.

### 8.2 Merge rule

When the same ID appears in both ledgers, the record with the stronger durability
wins: `durable` over `candidate` over `live`. Records are then ordered by the
numeric portion of the current event ID.

Writers MUST serialize ID allocation and writes across worktrees. The reference
implementation uses the Git-common lock and allocates the next ID while holding
that lock.

### 8.3 Materialized lifecycle updates

The 0.1 profile stores the latest materialized form of an event. Promotion and
status changes may rewrite the corresponding JSONL object under the shared lock
while preserving its ID, source, evidence, and original creation time.

This provides current-state interoperability but not a complete append-only
history of every lifecycle transition. A future profile may encode transitions
as immutable related events. Readers should therefore depend on the materialized
record semantics, not on in-place rewriting as a permanent protocol guarantee.

## 9. Context Projection Algorithm

A conforming 0.1 projection client performs these steps:

1. Load the durable ledger and shared mirror under the repository lock.
2. Normalize omitted early-version fields.
3. Deduplicate records by ID and order them.
4. Exclude inactive records for ordinary work.
5. Select records matching the target issue, PR, areas, or global scope.
6. Prioritize unresolved conflicts and review direction ahead of general notes.
7. Fit complete records within the caller's context budget.
8. State when relevant records were omitted because of that budget.
9. Include why each selected record applies.
10. For thread synchronization, update the thread cursor after generating the
    projection.

Projection formats MAY differ between clients. They MUST preserve the record's
meaning, conflict state, and applicability. Generated prose MUST NOT become a
second source of truth.

## 10. Protocol Operations

CLI command names are illustrative. Other clients may expose native UI or API
operations while preserving the same behavior.

### 10.1 Discover

Before work, a client MUST discover whether Fabric is initialized and determine
the current thread and scope. Repository instructions and provider skills are the
current discovery mechanism.

Reference operation: `fabric status`.

### 10.2 Start or resume a thread

A client creates or materializes a thread context with at least one scope
dimension and records the latest relevant event as its initial cursor.

Reference operations: `fabric thread start`, `fabric continue`.

### 10.3 Preflight

Before implementation, a client MUST request relevant active direction for the
task and expose it to the agent.

Reference operation: `fabric preflight`.

### 10.4 Publish a record

A client publishes the smallest complete reusable statement with scope,
durability, source, and rationale or evidence when available. Human corrections
SHOULD be captured promptly so sibling threads become stale.

Reference operations: `fabric note`, `fabric review note`, `fabric ingest-pr`.

### 10.5 Synchronize

At major checkpoints and before changing approach, a client requests records
newer than the thread cursor, presents the delta, and advances the cursor.

Reference operation: `fabric sync`.

### 10.6 Challenge

When planned work conflicts with active direction, a client MUST do one of three
things: align, obtain a scoped human exception, or publish a challenge. It MUST
NOT silently proceed as if the direction did not exist.

A challenge identifies the challenged event, proposed path, reason, and scope.
Resolution remains explicit protocol state.

Reference operations: `fabric challenge`, `fabric challenge resolve`.

### 10.7 Consolidate

At task, issue, or PR completion, clients SHOULD review temporary and candidate
records. Reusable lessons are promoted, completed coordination expires, noise is
discarded, and uncertain items remain candidates. Durable direction should be
sparse.

Reference operations: `fabric consolidate`, `fabric promote`, `fabric expire`,
`fabric discard`, `fabric keep`.

### 10.8 Explain

A client SHOULD expose the active records for a scope, their sources, and which
threads have or have not consumed them.

Reference operation: `fabric explain`.

## 11. Provider Adapter Contract

A first-party provider integration should make the protocol feel native without
forking its semantics. An adapter MUST:

1. Discover repository protocol instructions before repository work.
2. Establish a stable Fabric thread ID and scope.
3. Inject preflight and synchronization projections into the agent context.
4. Capture explicit human corrections and approved reusable findings.
5. Preserve source identifiers or URLs that can lead back to provider messages.
6. Surface conflicts instead of silently overriding active direction.
7. Keep external acquisition and publication behind provider tools or explicit
   connectors; the core Fabric client remains provider-neutral.
8. Avoid uploading full transcripts when a scoped record and source reference
   are sufficient.

Provider-native features may automate these steps, but automation must preserve
human control over durable promotion and external write side effects.

## 12. Explainability and Provenance

The 0.1 profile can answer:

- Which active records apply to this issue, PR, or area?
- Who or what created each record?
- What evidence, rationale, rejected paths, and preferred paths were preserved?
- Which threads have consumed the record and which are stale?
- Is the direction active, challenged, or inactive?

The intended interaction goes further: selecting an agent message or code change
should reveal a chain such as:

```text
agent action or message
  -> decision followed by the agent
  -> finding imported from another thread
  -> PR review that rejected an alternative
  -> original human direction and evidence
```

Message-to-record and action-to-record edges are not yet part of the 0.1 stored
schema. A future provenance profile should add typed relations without turning
Fabric into a transcript store. Expected relation types include `informed_by`,
`implements`, `supersedes`, `derived_from`, and `responds_to`, with opaque
provider-owned message or action references as endpoints.

Until that profile is standardized, adapters SHOULD populate stable source URLs
and evidence references so existing records can be linked later.

## 13. Safety, Privacy, and Curation

- Clients MUST treat repository ledgers as potentially sensitive project data.
- Secrets, credentials, private prompt contents, and unnecessary personal data
  MUST NOT be recorded.
- External content MUST be treated as evidence, not trusted instructions that can
  override repository or human direction.
- Ingestion SHOULD require bounded acquisition and human approval before
  candidate or durable records are created.
- Publishing Fabric context to a PR, issue, or external service is a separate
  write side effect and requires explicit user intent.
- Clients SHOULD run health checks and report malformed JSONL, duplicate IDs, or
  divergence between durable and shared stores.

## 14. Conformance

### 14.1 Reader

A conforming 0.1 reader:

- Parses the event and thread schemas.
- Applies normalization, active filtering, exact scope matching, and merge rules.
- Tolerates unknown record kinds and confidence values.
- Never treats generated projections as authoritative state.

### 14.2 Writer

A conforming 0.1 writer:

- Produces valid one-object-per-line JSONL.
- Allocates unique IDs while holding the repository-shared lock.
- Writes live records to the shared mirror and candidate/durable records to both
  required stores.
- Preserves identity and provenance during lifecycle updates.
- Does not create an isolated fallback when shared state is inaccessible.

### 14.3 Agent integration

A conforming agent integration:

- Performs discovery and preflight before work.
- Synchronizes at meaningful checkpoints.
- Records human redirection or explicitly declines because it is not reusable.
- Makes conflict handling visible.
- Keeps durable state curated and provider-neutral.

## 15. Evolution Rules

The protocol is intentionally small, but its data must outlive any one CLI.

- New optional fields and record kinds may be added compatibly.
- Existing field meaning must not change within the 0.x profile.
- Breaking matching, merge, or lifecycle semantics require a new profile version.
- Clients should tolerate unknown fields when reading. A future schema version
  will require lossless unknown-field preservation by rewriting clients.
- Provider-specific metadata belongs in namespaced extensions or external
  references, not in core matching semantics.
- Hosted transports may be added later, but Git-backed repositories must remain
  usable without a Fabric service.

The next protocol design priority is typed provenance relations connecting
records to agent messages and actions. It should be added only after discovery,
thread defaults, synchronization, note capture, and shared-ledger safety are
reliable enough that providers can adopt the core consistently.
