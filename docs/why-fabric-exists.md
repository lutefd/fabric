# Why Fabric Exists

Modern repository work is no longer a single conversation between one engineer
and one tool. It is a swarm of short-lived agent threads, IDE sessions, review
comments, CI checks, and sibling worktrees. That parallelism is useful, but it
creates a new failure mode: the repository can change quickly while the reasons
behind those changes evaporate.

Fabric exists because engineering teams and agent-platform builders need a
small, provider-neutral layer for repository memory: not a transcript archive,
not another issue tracker, and not a replacement for Git, but a protocol for
preserving the decisions and causal provenance that disposable agent contexts
otherwise lose.

The protocol boundary is defined in [PROTOCOL.md](../PROTOCOL.md). The local
reference client and repository behavior are summarized in
[README.md](../README.md), the current Local V1 evidence is stated in
[CONFORMANCE.md](../CONFORMANCE.md), and the optional future private relay is
kept separate in [SERVICE.md](../SERVICE.md).

## The Problem: Work Outlives the Thread

Agent threads are cheap to start and easy to abandon. That is part of their
power. A thread can investigate a bug, draft an implementation, respond to a
review, or test an idea without becoming the permanent place where the project
lives.

But repository direction often appears inside those disposable contexts:

- A human corrects an agent's assumption.
- A reviewer explains why a change belongs in a shared layer.
- One worktree discovers that a path is a dead end.
- Another thread learns that a temporary workaround should expire when a PR
  closes.
- A platform adapter knows exactly which prior direction was included in the
  model context, but the repository does not.

Without a shared protocol, every new thread starts with an incomplete memory of
the work. The code may contain the final result, but it rarely contains the
review rationale, rejected alternatives, source of authority, or coordination
state that made the result correct.

That gap gets worse as teams run more agents in parallel. A human can redirect
thread A while thread B continues in a sibling worktree with the rejected plan.
A useful finding can be rediscovered repeatedly because it never left the
conversation where it first appeared. A later maintainer can see what changed in
Git and still not know which decision the change implemented.

Fabric is designed for that missing layer.

## Why Transcripts Are Not Repository Memory

The obvious answer is to keep every prompt and response. Fabric deliberately
does not do that.

Transcripts are verbose, provider-owned, privacy-sensitive, and poorly shaped
for future work. They contain brainstorming, mistakes, credentials-adjacent
context, stale assumptions, and long passages that should not become durable
repository knowledge. Replaying transcripts into future contexts also wastes
budget and still leaves the important question unresolved: which few facts
should change what the next agent does?

Fabric treats a useful repository memory as a compact record: a decision,
finding, requirement, challenge, review direction, or note with scope, rationale,
confidence, trust, lifecycle state, and evidence references. The protocol
explicitly says record text must not contain source code, patches, complete
prompts, or transcripts. External content stays in the system that owns it;
Fabric keeps opaque references and short summaries.

That makes repository memory sparse by design. The point is not to remember
everything an agent saw. The point is to preserve the small set of facts that
can change future work or explain past work.

## Why Git Alone Is Not Enough

Git is excellent at preserving content history. It can show what changed, when
it changed, and who authored the commit. It is not designed to answer every
question an agent-platform user will ask later:

- Which human correction was available before this action?
- Which review finding explicitly influenced this commit?
- Which rejected path should a new thread avoid reopening?
- Was this direction merely delivered to the model, or did the agent declare it
  as causal?
- Did two worktrees create conflicting lifecycle updates, or did one silently
  win because it was newer?

Fabric does not replace Git. In Local V1, it uses Git-controlled storage as the
local trust and audit boundary. Candidate and durable events are tracked as
immutable ledger files; live coordination is shared through the Git-common
runtime used by sibling worktrees. Generated Markdown files are just human views,
not authoritative state.

That division matters. Git keeps the code and reviewed ledger history. Fabric
adds typed events, scoped records, lifecycle transitions, projections, receipts,
and relations so tools can explain why a repository action happened without
scraping transcripts or guessing from diffs.

## The Coordination Failure Across Worktrees

Parallel worktrees make the problem concrete. Each worktree can have its own
agent thread, local files, and partial plan, but the repository still needs one
shared understanding of active direction.

Fabric models that with scoped threads and deterministic projection. Direction
can match by repository, issue, PR, area, path, or global scope. Before acting, a
thread can receive the relevant subset of active records. During work, live
records can coordinate sibling worktrees without first becoming long-term
policy. After work completes, temporary records can expire, be discarded, or be
promoted through explicit lifecycle events.

The protocol avoids last-writer-wins mutation. Events are immutable. Lifecycle
changes append child events to a parent revision. If two lifecycle changes
compete from the same parent, readers must report a conflict instead of choosing
by timestamp, file order, or ID order. That is exactly the sort of behavior a
multi-agent repository needs: disagreement should become visible, not silently
papered over.

## Availability Is Not Causal Influence

Fabric's most important distinction is also one of the easiest to blur.

There are two separate questions:

1. What direction was available to a thread?
2. What direction did an agent, adapter, commit, PR, or action explicitly
   declare as influential or implemented?

The first question is answered by projections and receipts. A projection records
which event revisions were selected for a context view and why. A receipt can
record delivery, or adapter-confirmed exposure, when a provider integration knows
the projection actually entered model context.

That proves availability. It does not prove motivation.

The second question is answered by causal relations such as `informed_by` and
`implements`. Fabric does not infer causality from textual similarity or from
the mere fact that something was in context. If a provider only records that a
decision was exposed to the model, a truthful UI can say the decision was
available. It may say the decision informed an action only when an actor records
that explicit causal edge.

This distinction is what lets Fabric support honest "why did this happen?"
views. A provider can render delivered or exposed availability paths separately
from `informed_by` and `implements` causal paths, alongside unresolved external
nodes, record rationale, evidence, trust claims, and lifecycle state. That view
can show what was available without pretending that context delivery is mind
reading.

## A Provider-Neutral Protocol, Not a Provider Feature

Fabric is intentionally provider-neutral. Its external node references use
opaque provider-owned identifiers for messages, actions, commits, issues, PRs,
and evidence. The protocol does not require those systems to expose transcript
content, and clients must not infer content from opaque IDs.

That is important for agent-platform builders. Codex, Claude, IDE agents, CI
tools, review systems, and future integrations should be able to participate in
one repository decision graph without one provider owning the memory layer. A
native adapter can create or resume a Fabric thread, consume a projection,
acknowledge actual model exposure, and then record explicit causal relations for
important messages, actions, commits, or PRs.

The protocol is the product boundary. The CLI in this repository is the local
reference client, not the only possible implementation. The conformance claim
covers the Local V1 event model, storage behavior, materialization, projection,
receipt handling, relation graph, trust validation, explanation output, and
machine-response contract for the checked source revision. Other clients can
target the same artifacts and semantics without parsing the reference client's
human Markdown views.

## Local-First by Design

Fabric works locally without an account, daemon, network service, database, or
LLM call. That is not an implementation accident; it is part of the design.

Local V1 uses immutable event files and a shared Git-common runtime so sibling
worktrees can coordinate immediately. The repository can carry candidate and
durable direction through normal Git review. Agents can run preflight, sync,
record notes, create challenges, and traverse explanations while the protocol
state remains inspectable in repository-controlled storage.

The trust boundary is correspondingly modest and explicit. Local V1 trust claims
are claims, not cryptographic identity proofs. Durable promotion requires
human- or reviewer-confirmed rationale, and lower-trust records must not silently
supersede stronger direction. They can challenge it, or a human can explicitly
approve replacement.

That local-first stance also keeps Fabric useful before any platform integration
exists. A team can dogfood the protocol inside a repository, learn what direction
is worth preserving, and keep the history even if providers, tools, or hosted
services change.

## The Future Relay Is Optional

[SERVICE.md](../SERVICE.md) describes a possible private Fabric service, but it
is intentionally not part of Local V1. There is no server, network client,
authentication system, account model, signing layer, or encryption
implementation in the local conformance claim.

The future service exists for a narrower reason: local Git-common sharing is not
enough when repository memory must synchronize across machines, teams, and
providers. The proposed service is an optional end-to-end encrypted relay for
Fabric protocol events, not a source-code host, search index, transcript store,
LLM gateway, provider proxy, or semantic authority.

The privacy line is strict. The service must not receive source code, patches,
plaintext direction, rationale, evidence, prompts, model responses, transcripts,
repository credentials, or decrypted projections. Matching, projection,
conflict materialization, and explanation remain client-side. The relay may see
only routing metadata such as accounts, devices, opaque repository and event
identifiers, membership metadata, timestamps, blob sizes, and delivery receipts,
and even that leakage must be documented honestly.

Most importantly, service adoption must not convert a repository into a
service-only format. Local Git-backed operation remains complete and
first-class. The relay is a transport option for encrypted blobs, not the source
of protocol meaning.

## What Fabric Makes Possible

Fabric gives repositories a way to remember the small things that compound:

- Human corrections that should affect sibling threads immediately.
- Review rationale that should survive the PR conversation.
- Rejected paths that should not be rediscovered every week.
- Temporary coordination that should expire instead of becoming permanent lore.
- Durable direction that should be traceable to evidence and trust claims.
- Provider actions that can be explained by explicit provenance rather than
  inferred after the fact.

For engineers, that means less repeated explanation and fewer regressions into
known-bad approaches. For agent-platform builders, it means there is a clean
adapter contract for repository memory: expose what was available, record what
was actually causal, preserve opaque links to provider objects, and let the
repository own the decision graph.

Fabric exists because parallel agent work needs memory that is smaller than a
transcript, richer than a commit, more portable than a provider feature, and
more honest than a guessed explanation.
