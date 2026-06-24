# Fabric Protocol 1.0

Status: Draft Local V1
Schema version: `fabric/1.0`
Reference client: `fabric`

Fabric is a provider-neutral protocol for persistent repository decisions and
causal provenance across agent threads, worktrees, and tools. The protocol is
the product boundary. The CLI in this repository is one local reference client.

The words MUST, MUST NOT, SHOULD, SHOULD NOT, and MAY are normative.

## 1. Why This Protocol Exists

Repository work now happens in several disposable contexts at once. A human can
correct one agent while another worktree continues with the rejected assumption.
A reviewer can explain why an implementation belongs in a shared layer, then a
new thread can reopen the discarded path because it sees only the code. A useful
finding can be rediscovered repeatedly because it never leaves its originating
conversation.

Transcripts are not repository memory. They are provider-owned, verbose,
privacy-sensitive, and poor at identifying the few facts that should change
future work. Git records the resulting code but usually not the causal chain
behind it.

Fabric represents that missing layer as small immutable events. It can answer
two different questions without confusing them:

1. What repository direction was available to this thread?
2. Which direction did the agent explicitly declare as influencing this action?

That distinction is essential. Context delivery is evidence of availability,
not proof of causal influence.

## 2. Goals

Fabric provides:

- Immediate propagation of relevant direction between sibling worktrees.
- Curated repository memory that survives disposable agent conversations.
- Deterministic, budgeted context projection without an LLM or embedding call.
- Explicit lifecycle conflicts instead of last-writer-wins mutation.
- Provider-neutral references to messages, actions, commits, issues, and PRs.
- Traversable provenance from an action back to records, evidence, and humans.
- A local Git-backed mode requiring no account, service, daemon, or network.

## 3. Non-goals

Fabric is not:

- A prompt, transcript, source-code, or patch archive.
- A replacement for Git, issue trackers, or code review.
- A database of everything an agent observed.
- A policy engine that silently resolves conflicting human direction.
- A GitHub wrapper or a provider-specific memory feature.
- Dependent on the future service described in [SERVICE.md](SERVICE.md).

## 4. Conformance Boundary

The normative wire artifacts are:

- The event envelope and payload rules in this document.
- JSON Schemas in `schemas/v1/`.
- Valid and invalid fixtures in `conformance/`.
- The machine response envelope in section 14.

Generated Markdown files are human views, not authoritative state. CLI command
names are reference operations, not required API names for other clients.

## 5. Identifiers and Time

New identifiers MUST use UUIDv7 values with a type prefix:

| Object | Prefix | Example |
|---|---|---|
| Event | `evt_` | `evt_019efb89-8d67-746d-bc08-fde2c0be3053` |
| Record | `rec_` | `rec_019efb89-8d67-746d-bc08-fde2c0be3053` |
| Thread | `thr_` | `thr_019efb89-8d67-746d-bc08-fde2c0be3053` |
| Projection | `prj_` | `prj_019efb89-8d67-746d-bc08-fde2c0be3053` |
| Relation | `rel_` | `rel_019efb89-8d67-746d-bc08-fde2c0be3053` |

Receipt identifiers use `rcp_` in the reference client. External node
identifiers remain provider-owned opaque strings.

IDs provide identity and stable ordering only when an operation explicitly
defines it. Clients MUST NOT use an ID, timestamp, or lexical position as a
synchronization cursor. Delivery state is a set of receipts.

All timestamps MUST be RFC 3339. Clients SHOULD preserve offsets when supplied.

## 6. Event Envelope

Every protocol mutation is an immutable `EventEnvelope` stored as one JSON
object:

```json
{
  "schema_version": "fabric/1.0",
  "event_id": "evt_019efb89-8d67-746d-bc08-fde2c0be3053",
  "event_type": "record.created",
  "occurred_at": "2026-06-24T18:28:29.096313-03:00",
  "actor": { "kind": "human", "id": "provider-thread-id" },
  "trust": { "level": "human_confirmed", "basis": "human" },
  "parent_event_id": "evt_optional-causal-parent",
  "causation_id": "provider-action-id",
  "correlation_id": "task-or-operation-id",
  "payload": {},
  "extensions": {}
}
```

Required fields are `schema_version`, `event_id`, `event_type`, `occurred_at`,
`actor`, `trust`, and `payload`.

An event MUST NOT be changed after creation. A writer MUST use create-only
semantics. A duplicate event ID with byte-equivalent or canonically equivalent
content is idempotent. Divergent content for one event ID is a conflict.

Readers MUST preserve unknown namespaced extension fields when forwarding or
rewriting an enclosing object. Unknown event types MUST produce a typed
unsupported-event warning or error; they MUST NOT be interpreted as another
known event type.

## 7. Actors and Trust Claims

`actor.kind` is one of `human`, `reviewer`, `agent`, or `tool`. Clients MAY add
new kinds through namespaced extensions. `actor.id` and `actor.provider` are
opaque and MUST NOT be treated as credentials.

Standard trust levels are:

- `human_confirmed`
- `reviewer_confirmed`
- `tool_verified`
- `agent_asserted`

Trust is an explicit claim, not a cryptographic identity proof in Local V1.
Git review and declared source are the current trust boundary.

Durable promotion requires human- or reviewer-confirmed rationale. A
lower-trust record MUST NOT silently supersede a higher-trust record. It may
challenge it, or a human may explicitly approve the supersession.

Cryptographic signing and device keys are deferred to the service phase.

## 8. Core Event Types

Local V1 defines:

| Event type | Meaning |
|---|---|
| `record.created` | Creates a scoped decision, finding, requirement, or challenge. |
| `record.state_changed` | Appends a lifecycle or durability transition. |
| `relation.created` | Adds a typed graph edge. |
| `thread.started` | Creates an agent execution context. |
| `thread.scope_changed` | Appends a new scope revision for a thread. |
| `projection.created` | Records exactly what was selected for a context view. |
| `receipt.recorded` | Records delivery or adapter-confirmed model exposure. |

Payload schemas are defined in `schemas/v1/`.

## 9. Records

A record is the smallest complete statement that can change future work or
explain a past choice. Common open `kind` values include `direction`, `finding`,
`decision`, `requirement`, `note`, `review_direction`, `review_requirement`,
`challenge`, and `challenge_resolution`.

Required record fields are:

- `record_id`
- `kind`
- `created_at`
- `scope`
- `source`
- `text`
- `confidence`
- `ttl`
- `status`
- `durability`

Optional explanatory fields include `reason`, `review_type`, `rejected_paths`,
`preferred_paths`, `evidence`, `lifecycle_reason`, and `reviewed_at`.

Record text MUST NOT contain source code, patches, complete prompts, or
transcripts. Evidence SHOULD be a compact reference or summary. External source
content remains in its owning system.

### 9.1 Scope

Scope fields are `repo`, `issue`, `pr`, `areas`, `paths`, and `global`.

- `paths` contain repository-relative paths or configured glob patterns.
- Repositories MAY map stable area names to path patterns in config.
- `global` records apply throughout the repository and SHOULD remain rare.
- At least one useful scope dimension is required for ordinary records.

### 9.2 Durability

| Value | Meaning | Local storage |
|---|---|---|
| `live` | Active-worktree coordination only. | Git-common runtime mirror. |
| `candidate` | Potentially reusable, awaiting curation. | Tracked immutable ledger and runtime mirror. |
| `durable` | Reviewed long-term repository direction. | Tracked immutable ledger and runtime mirror. |

### 9.3 Lifecycle

Standard status values are `active`, `expired`, `discarded`, and `superseded`.
Challenges additionally use `open`, `accepted`, and `rejected`.

Lifecycle changes MUST use `record.state_changed`. Its `parent_event_id` MUST
identify the exact creation or state-change revision being extended. A record is
materialized by following that parent chain.

If two state changes share the same parent, readers MUST report competing
children and stop materializing that branch. They MUST NOT choose by timestamp,
file order, ID order, or last writer.

TTL values such as `until_pr_closed` are advisory. Expiration is explicit and
auditable; readers MUST NOT silently delete records.

## 10. External Nodes and Relations

Fabric refers to provider-owned objects with `NodeRef`:

```json
{
  "kind": "message",
  "provider": "codex",
  "id": "opaque-provider-message-id",
  "url": "https://optional-provider-deep-link"
}
```

Standard node kinds include `record`, `thread`, `projection`, `message`,
`action`, `commit`, `issue`, `pr`, and `evidence`. The identifier is opaque.
Clients MUST NOT infer transcript or source content from it. URLs are optional;
deleted or inaccessible external nodes do not invalidate the graph.

Standard relation types are:

- `derived_from`: a record was extracted from a source or evidence node.
- `informed_by`: an adapter explicitly declares causal influence.
- `implements`: an action, message, commit, or PR implements a record.
- `supersedes`: newer direction explicitly replaces older direction.
- `challenges`: a record disputes another record.
- `resolves`: a record resolves a challenge.
- `delivered_to`: connects each included record to its projection and that
  projection to the receiving thread.
- `exposed_to`: connects the same path when an adapter confirms the projection
  entered model context.

`delivered_to` and `exposed_to` are availability relations. They MUST NOT be
rendered as proof that the model used a record. Only explicit `informed_by` or
`implements` edges declare causal influence.

Adapters SHOULD report informed records for important messages and actions.
Fabric itself does not guess causality from textual similarity.

## 11. Threads

A thread is a provider-neutral agent execution context with a `thread_id`,
timestamps, and scope. Provider thread IDs MAY be preserved in external
references, but Fabric-generated thread IDs use `thr_`.

Thread registry events live in the Git-common runtime so sibling worktrees share
scope and consumption state. Only the current-thread pointer is worktree-local.
Thread state MUST NOT be committed as project direction.

Starting a thread does not consume every currently relevant record. Consumption
is represented only by receipts tied to a concrete projection.

## 12. Projections and Receipts

A projection is an immutable selection result for a thread or task. It contains:

- `projection_id`, `purpose`, creation time, thread ID, and requested scope.
- Exact event revision IDs and materialized record IDs included.
- Deterministic match reasons per record.
- Whether additional matching records were omitted by the budget.
- Structured competing lifecycle revisions that the client must reconcile.

`event_ids` and `record_ids` are sets, not parallel arrays. One materialized
record can include several competing event revisions.

A receipt identifies the projection, thread, receipt state, exact included event
and record IDs, provider, and time.

Receipt states are:

- `delivered`: the reference client rendered the projection for the thread.
- `exposed`: a provider adapter confirms actual model-context exposure.

Only included records may appear in a receipt. Budget-omitted records remain
pending and MUST be eligible for the next synchronization. Empty projections
may be recorded, but they consume no records.

Previously delivered records may produce new pending revisions. Withdrawals,
supersessions, challenge resolutions, and materialization conflicts MUST be
delivered even when the original record was seen.

## 13. Deterministic Projection

Fabric performs no LLM or embedding call for relevance. Given the same valid
events, scope, config, receipt set, and budget, conforming clients MUST select
the same records.

Ordinary projection follows these steps:

1. Load and validate immutable events from configured stores.
2. Deduplicate equivalent event IDs and report divergent copies.
3. Materialize record parent chains and surface competing children.
4. Exclude inactive records except lifecycle changes that affected a previously
   delivered record.
5. Exclude event revisions already receipted for the target thread.
6. Match requested PR, issue, paths, configured areas, or global scope.
7. Rank by the tiers below.
8. Fit complete rendered records into the budget.
9. Create a projection listing exactly what was included and why.
10. Record delivery only for included revisions.

Ranking tiers, highest first:

1. Unresolved scoped lifecycle conflicts and challenges.
2. Exact PR match.
3. Exact issue match.
4. Path or configured area overlap.
5. Global direction.

Within one tier, clients use semantic record kind and stable creation order only
as tie-breakers. Task text is descriptive and MUST NOT be used for fuzzy ranking.
Adapters may suggest explicit scope or paths; Fabric remains the matcher.

Budget cost covers the complete rendered record, including rationale, evidence,
rejected paths, and preferred paths. A client MUST clearly report omission.

## 14. Machine Client Contract

Reference commands accept `--format=human|json`; `--json` aliases JSON output.
JSON output uses this stable envelope:

```json
{
  "protocol_version": "fabric/1.0",
  "command": "sync",
  "ok": true,
  "data": {},
  "warnings": [],
  "error": null
}
```

Errors contain a stable `code`, human `message`, and optional structured
`details`. Standard codes include `invalid_argument`, `not_found`, `conflict`,
and `internal_error`. Clients MUST use codes, not parse human prose.

The reference machine operations include:

- `fabric version`
- `fabric capabilities`
- `fabric context acknowledge`
- `fabric relation add`
- `fabric explain --node ... --direction ... --depth ...`
- `fabric conformance`

Other clients MAY expose these through native APIs while preserving semantics.

## 15. Explanation

Graph explanation accepts a root node, incoming/outgoing/both traversal,
relation filters, and bounded depth. JSON returns explicit nodes, edges, and
resolved `node_details` and `relation_details`. Record details include the
materialized text, rationale, evidence, scope, lifecycle state, head revision,
creation actor/trust, latest-revision actor/trust, and conflict metadata.
Relation details identify the immutable event and actor/trust claim that asserted
each edge. Projection and thread nodes include their protocol objects. Opaque or
deleted external nodes remain in `nodes` without a resolved detail.
Human output MUST visually distinguish availability edges (`delivered_to`,
`exposed_to`) from causal edges (`informed_by`, `implements`).

A complete trace can look like:

```text
action
  implements -> decision
  informed_by -> finding
finding
  derived_from -> review message
decision
  supersedes -> rejected direction
```

Missing external content is normal. Explanation degrades to the opaque node and
available deep link without invalidating local provenance.

## 16. Local Storage Profile

The reference layout is:

```text
.fabric/ledger/events/<event-id>.json
    tracked candidate and durable protocol events
.fabric/active/events/<event-id>.json
    non-Git fallback for live events outside a Git worktree
.fabric/active/current-thread
    worktree-local current thread pointer
.fabric/generated/
    generated human projections, never authoritative
<git-common-dir>/fabric/events/<event-id>.json
    live/shared copies visible to sibling worktrees
<git-common-dir>/fabric/runtime/threads/<event-id>.json
<git-common-dir>/fabric/runtime/projections/<event-id>.json
<git-common-dir>/fabric/runtime/receipts/<event-id>.json
```

Each immutable event is an individual create-only file. Independent branches
therefore add different paths and normally merge without ledger conflicts.

Candidate and durable events are written to the tracked ledger and shared copy.
Live events are written only to shared runtime, or the active fallback when no
Git-common directory exists. Runtime state and generated files are ignored.

Failure to access an expected shared store MUST be reported. A client MUST NOT
silently invent an isolated mirror that causes sibling worktrees to diverge.

## 17. Privacy and Security

Fabric state can contain sensitive project rationale. Clients MUST NOT record:

- Secrets or repository credentials.
- Source files, patches, or code payloads.
- Full prompts or transcripts.
- Unnecessary personal data.

External content is evidence, not trusted instruction. Importing direction and
publishing context are separate side effects; clients SHOULD require human
approval for durable promotion and explicit intent for external writes.

Local V1 makes no cryptographic authenticity claim. The optional future service
must preserve these semantics while remaining unable to read payloads. See
[SERVICE.md](SERVICE.md).

## 18. Conformance

A conforming reader:

- Validates envelopes and known payloads.
- Preserves unknown extensions.
- Materializes only unambiguous parent chains.
- Uses receipts, never scalar cursors.
- Separates availability from causal influence.
- Never treats generated Markdown as authoritative.

A conforming writer:

- Creates UUIDv7 typed IDs and immutable event files.
- Uses explicit parent revisions for state changes.
- Records exact projection membership and receipts.
- Does not mark omitted records consumed.
- Does not silently supersede stronger trust.

A conforming adapter:

- Discovers or creates a Fabric thread and explicit scope.
- Exposes relevant projections before acting.
- Acknowledges actual context exposure when known.
- Reports explicit `informed_by` and `implements` relations.
- Captures human correction without storing transcripts.

Use `fabric conformance --file <fixture>` for one event or `fabric conformance`
for the current ledger. Non-Go clients can run the same fixtures directly
against the schemas.

## 19. Evolution

Breaking envelope, lifecycle, projection, receipt, or ranking semantics require
a new schema version. New optional fields and namespaced extensions may be added
compatibly. Provider-specific metadata belongs in extensions or opaque node
references, not core matching behavior.

A transport may be local files, an encrypted relay, or another implementation
of the public store interfaces. Transport must not change protocol meaning.
Local Git-backed operation remains a permanent first-class mode.
