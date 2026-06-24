# Fabric

Fabric is a provider-neutral protocol for persistent repository decisions and
causal provenance across agent threads, worktrees, and tools. The `fabric` CLI
is its local reference client.

It lets one thread preserve a correction, finding, or decision once; lets other
threads receive the relevant subset before acting; and lets a later user trace
an agent action back through the records, evidence, review, and human direction
that actually informed it.

Start with the essay [Why Fabric Exists](docs/why-fabric-exists.md) for the
coordination problem, large-project story, and OSS evidence behind the protocol.
Then read the normative [protocol specification](PROTOCOL.md), the reference
[conformance claim](CONFORMANCE.md), and the deliberately deferred [private
service design](SERVICE.md).

## Why We Need This

The bottleneck in multi-agent development is not only model intelligence or
context-window size. Large projects already spend human time deciding ownership,
rejecting plausible approaches, and explaining local constraints. Then the next
person or agent starts without that history and pays for the same decision
again.

This is not always a lack of coordination. Sometimes the meeting happened, the
review was clear, and the project direction was reasonable. The failure is that
later work cannot see what was decided, where it applies, or why a rejected path
should stay rejected. More parallel agents make that failure faster.

We saw the same shape in a bounded public sample of 100 recently merged
`vercel/next.js` PRs: 200 of 234 top-level comments were automated, and only 29
of 311 submitted reviews contained body text. In three detailed PRs, the useful
direction was scattered across descriptions, commits, and inline threads. The
[full essay](docs/why-fabric-exists.md#what-a-small-oss-sample-revealed) explains
the method, examples, and limitations.

### Decisions are trapped in conversations

A human corrects thread A. Thread B, running in a sibling worktree, keeps the old
assumption. A PR reviewer redirects thread C, but thread D sees only the final
diff. A feature area extends an existing abstraction while another thread
quietly invents a duplicate. Every additional parallel context adds execution
capacity and another place for repository direction to diverge.

### Git preserves results, not causality

Commits are excellent at showing what changed. They rarely encode the full
reason an endpoint was extended instead of duplicated, which alternatives were
rejected, what hidden constraint another thread found, or whose direction made
the choice authoritative.

### Transcripts are the wrong memory primitive

Archiving every prompt and reply creates a large, provider-owned, private corpus
without identifying what future work should do differently. Replaying it wastes
context and still leaves causality ambiguous. Fabric stores a sparse set of
curated records and opaque links, never the conversation itself.

### Delivery is not proof of influence

Even when a decision was present in context, that does not prove the agent used
it. Fabric records projections and exposure receipts separately from explicit
`informed_by` and `implements` relations. This makes explanations honest:
"available to the model" and "declared as causal" are different facts.

### Repository memory must outlive providers

Codex, Claude, an IDE agent, CI, and future first-party integrations should be
able to participate in one decision history without any provider owning it.
Fabric keeps the protocol in repository-controlled, inspectable events and gives
providers a small adapter contract.

The intended experience is simple: select a message, action, commit, or PR and
walk backward through decisions, findings, review evidence, and human direction.
The repository becomes better at remembering what matters without becoming a
transcript database.

## What Fabric Does

- Stores immutable decisions, findings, requirements, challenges, and lifecycle
  transitions as versioned protocol events.
- Shares active direction immediately through the Git-common directory used by
  sibling worktrees.
- Projects relevant records by PR, issue, path, configured area, and global
  scope with deterministic ranking and complete-record budgets.
- Tracks exact delivery and provider-confirmed exposure with receipts instead of
  scalar cursors.
- Links records to opaque messages, actions, commits, issues, PRs, and evidence.
- Distinguishes context availability from explicit causal influence.
- Works fully locally with no account, network, daemon, database, or LLM call.

## Core Model

**Events** are immutable protocol mutations. Each event is one JSON file with a
typed UUIDv7 ID such as `evt_...`.

**Records** are concise repository knowledge: direction, decisions, findings,
requirements, challenges, and resolutions. Record IDs use `rec_...`.

**Threads** are scoped agent execution contexts. The registry and receipts are
shared across worktrees; only the current-thread pointer is local.

**Projections** record exactly which event revisions were selected, why they
matched, and whether more were omitted by budget.

**Receipts** record CLI delivery or adapter-confirmed model exposure. An omitted
record remains pending.

**Relations** form the provenance graph: `derived_from`, `informed_by`,
`implements`, `supersedes`, `challenges`, and `resolves`. Availability relations
such as `delivered_to` and `exposed_to` are never presented as causal proof.

## Durability and Trust

| Tier | Meaning | Storage |
|---|---|---|
| `live` | Temporary active-worktree coordination. | Git-common runtime only. |
| `candidate` | Probably reusable, awaiting curation. | Tracked immutable ledger. |
| `durable` | Reviewed long-term repository direction. | Tracked immutable ledger. |

Agents should default uncertain reusable knowledge to candidate. Durable
promotion requires human- or reviewer-confirmed rationale. Lower-trust direction
cannot silently replace stronger direction; it must challenge it or receive
explicit approval.

## Install

```bash
go install ./cmd/fabric
```

Or build locally:

```bash
go build -o fabric ./cmd/fabric
```

## Quick Start

```bash
fabric init
fabric install-agents

fabric thread start --issue VS-123 --area virtual-store/listing \
  --path 'internal/listing/**'

fabric preflight "add listing filters" --issue VS-123 \
  --area virtual-store/listing --path 'internal/listing/**'

fabric note --candidate --reason "The shared endpoint owns pagination semantics" \
  "Extend the existing listing endpoint instead of creating another one."

fabric sync
```

Fabric writes human projections under `.fabric/generated/`. They are views of
the protocol state, not a second ledger.

## Common Operations

```bash
# Discover client and repository capabilities
fabric version --json
fabric capabilities --json
fabric status

# Create or resume scoped work
fabric thread start --issue VS-123 --pr 42 --area listing --path 'internal/listing/**'
fabric continue --pr 42 --budget 700

# Record direction and review findings
fabric note --candidate --issue VS-123 --reason "why this applies" "direction"
fabric review note --pr 42 --issue VS-123 "review direction"

# Synchronize exact pending revisions
fabric preflight "task description" --issue VS-123 --budget 800
fabric sync --budget 300

# Confirm that a native adapter placed a projection in model context
fabric context acknowledge --projection prj_... --state exposed --provider codex

# Add causal provenance
fabric relation add --type informed_by \
  --from action:codex:opaque-action-id --to record:rec_...
fabric relation add --type implements \
  --from commit:abc123 --to record:rec_... --durable --reason "implements decision"

# Traverse explanation
fabric explain --node action:codex:opaque-action-id --direction both --depth 4 --json

# Make disagreement explicit
fabric challenge --direction rec_... --pr 42 --proposal "new path" --reason "new evidence"
fabric challenge resolve rec_... --accepted --reason "scoped exception approved"

# Curate after work completes
fabric consolidate --pr 42
fabric promote rec_... --reason "reusable, human-confirmed repository direction"
fabric expire rec_... --reason "PR completed"
fabric discard rec_... --reason "too specific"

# Validate storage and protocol
fabric doctor
fabric conformance
```

All commands support `--format=human|json`; `--json` is an alias. JSON responses
use a stable protocol envelope and typed error codes so adapters never need to
parse Markdown.

## Repository Layout

```text
.agents/skills/                         discoverable provider workflows
.fabric/config.yaml                    repository matching and budgets
.fabric/ledger/events/<event-id>.json  tracked candidate/durable events
.fabric/active/current-thread          worktree-local pointer, ignored
.fabric/generated/                     human projections, ignored

<git-common-dir>/fabric/events/        shared live event copies
<git-common-dir>/fabric/runtime/        threads, projections, receipts
```

Git hygiene is one sentence: commit project direction, not agent runtime state.

Tracked:

- `AGENTS.md`
- `.agents/skills/**`
- `.fabric/config.yaml`
- `.fabric/ledger/events/**`
- `PROTOCOL.md`, schemas, and conformance fixtures

Ignored:

- `.fabric/active/**`
- `.fabric/generated/**`
- the Git-common Fabric runtime

One event per file makes independent branch additions merge naturally. Lifecycle
changes append child events. Competing children are reported as conflicts, never
resolved by last writer.

## Agent Workflow

Repository skills under `.agents/skills/` adapt agents to the protocol:

- `fabric-session`: discovery, preflight, sync, and continuation.
- `fabric-provenance`: exposure receipts, causal assertions, and explanation.
- `fabric-record-direction`: corrections, rationale, and challenges.
- `fabric-pr-direction`: bounded PR mining with approval before ingestion.
- `fabric-consolidate`: sparse post-task curation.
- `fabric-publish`: explicit external publication only.

Fabric itself does not call GitHub or an LLM. Skills use provider connectors or
authenticated tools to acquire context, then submit only approved records.

## Provider Motivation View

A provider such as Codex can build a native "Why did this happen?" view without
reading Fabric Markdown:

1. Create or resume a Fabric thread and consume a projection.
2. Acknowledge actual model exposure with `fabric context acknowledge`.
3. After an important message, tool action, commit, or PR, create explicit
   `informed_by` and/or `implements` relations to the records it used. The
   relation command accepts actor, provider, and trust claims so the assertion
   can be attributed to the adapter that made it.
4. When the user selects that provider object, call:

   ```bash
   fabric explain --node action:codex:<opaque-id> --direction both --depth 4 --json
   ```

The graph response includes opaque provider nodes, typed edges, and resolved
record details with text, rationale, evidence, scope, creation and lifecycle
actor/trust, lifecycle state, and conflicts. Relation details identify who
asserted every causal or availability edge. Projection and thread details show
when context was delivered or exposed. Provider deep links remain available on
node references.

Fabric never infers motivation from text similarity. If Codex reports only an
exposure receipt, the view can say the record was available in context. It may
say the record informed or was implemented by the action only when Codex writes
the explicit causal relation.

## Code Layout

- `protocol/`: public transport-neutral Go contracts and validation.
- `internal/core/`: direction mapping, relevance, graph traversal, and causal
  materialization.
- `internal/store/`: create-only immutable files and local/shared ledger routing.
- `internal/direction/`: repository operations that compose core and storage.
- `internal/skills/`: provider skill templates and installation sources.
- `internal/cli/`: command parsing plus human/JSON rendering.
- `cmd/fabric/`: the executable entry point.

Protocol and repository behavior do not depend on CLI parsing, and storage does
not own domain semantics. A future encrypted transport can implement the public
store interfaces without moving those boundaries back into the CLI.

## Future Private Service

Local Git-backed operation is permanent. A future optional service may relay
end-to-end encrypted event blobs between devices and repositories. It may know
accounts, opaque repository/thread/event IDs, timestamps, membership, device
public keys, blob sizes, and delivery receipts. It must never receive source
code, patches, plaintext direction, rationale, evidence, prompts, transcripts,
or repository credentials.

Matching, projection, decryption, and explanation remain client-side. The full
design and threat model are preserved in [SERVICE.md](SERVICE.md); no server,
network client, authentication, or encryption is implemented now.

## Development

```bash
go test ./...
go test -race ./...
```

Schemas live in `schemas/v1/`, fixtures in `conformance/`, and the public Go
protocol package in `protocol/`.
