# Local V1 Conformance Claim

This document states the evidence behind the reference client's claim of
conformance with [Fabric Protocol 1.0](PROTOCOL.md). It is not a third-party
certification and does not cover the future service.

## Claim

The current `fabric` CLI and public Go `protocol` package conform to the Local V1
event, storage, materialization, projection, receipt, relation, trust,
explanation, and machine-response requirements.

The claim is bounded to the checked source revision. It must be re-established
after a protocol or behavior change by running all verification commands below.

## Evidence Matrix

| Protocol surface | Reference implementation and evidence |
|---|---|
| Event envelope and typed UUIDv7 IDs | `protocol/`, strict `DecodeEvent`, ID and fuzz tests |
| Versioned payload contracts | `schemas/v1/`, valid/invalid `conformance/` fixtures |
| Unknown extension preservation | event/record extension round-trip tests |
| Immutable create-only storage | `internal/store`, idempotency, crash-temp, race, and multi-process tests |
| Lifecycle materialization | explicit parent chains, structured competing-child conflicts, no last-writer-wins |
| Shared worktree state | Git-common event/runtime stores and executable branch-merge evaluation |
| Threads, projections, receipts | filesystem `RuntimeStore`, exact revision membership, omission and exposure tests |
| Deterministic relevance | PR, issue, path/area mapping, global tiers, stable tie-break tests |
| Trust boundary | actor/trust validation, durable-promotion rationale, lower-trust supersession rejection |
| Provenance graph | causal and availability relations, bounded traversal, resolved node details |
| Machine contract | JSON envelope, capabilities/version, typed errors, conformance failure details |
| Local independence | all executable evaluations run with no account, network, service, or provider dependency |

## Provider Motivation View

The protocol is ready for a provider to render motivations behind a message or
action when the provider supplies stable opaque node IDs and reports causal
relations.

`fabric explain --node ... --json` returns:

- The selected provider node and bounded relation graph.
- Explicit `informed_by` and `implements` causal edges.
- Separate `delivered_to` and `exposed_to` availability paths through the exact
  projection and thread.
- Materialized record text, rationale, evidence, scope, lifecycle status,
  creation actor/trust, latest-revision actor/trust, head revision, and conflict
  metadata.
- Relation assertion event, actor, and trust metadata for each returned edge.
- Opaque unresolved external nodes and optional deep links without requiring
  transcripts or source code.

Fabric cannot truthfully infer that a record motivated an action merely because
it was exposed to the model. A Codex integration must create the explicit causal
edge after the message/action. Without that adapter report, the view is ready to
show availability only.

## Verification

```bash
go vet ./...
go test ./...
go test -race ./...
./evals/run-local-v1.sh
fabric conformance --json
fabric doctor --json
```

The verification suite includes correction capture, cross-thread sync, budget
omission, challenge handling, causal explanation, exposure acknowledgement,
resolved motivation details, dry-run write safety, multi-process writes, and
independent Git branch merges.

## Exclusions

- No server, account, network client, encryption, signing, or key management.
- No claim that opaque provider objects still exist or remain accessible.
- No claim of causal influence unless a provider or actor records an explicit
  `informed_by` or `implements` relation.
- No cryptographic proof of actor/trust claims in Local V1.
