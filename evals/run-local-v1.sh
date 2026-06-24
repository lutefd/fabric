#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
workspace="$(mktemp -d)"
binary="${workspace}/fabric"
trap 'rm -rf "${workspace}"' EXIT

go_bin="${GO:-$(command -v go || true)}"
if [[ -z "${go_bin}" && -x /usr/local/go/bin/go ]]; then
  go_bin=/usr/local/go/bin/go
fi
if [[ -z "${go_bin}" ]]; then
  echo 'go is required' >&2
  exit 1
fi

(
  cd "${repo_root}"
  GOCACHE="${workspace}/gocache" GOMODCACHE="${workspace}/gomodcache" \
    "${go_bin}" build -o "${binary}" ./cmd/fabric
)

cd "${workspace}"
git init -q
"${binary}" init >/dev/null

"${binary}" thread start --id thread-a --issue FAB-1 --area protocol >/dev/null
"${binary}" thread start --id thread-b --issue FAB-1 --area protocol >/dev/null
"${binary}" status | rg -q 'Current thread:'
"${binary}" preflight 'implement Local V1' --issue FAB-1 --area protocol >/dev/null

"${binary}" note --candidate --thread thread-a --issue FAB-1 --area protocol \
  --reason 'The repository needs one immutable protocol boundary.' \
  'Use immutable events for repository direction.' >/dev/null
"${binary}" note --candidate --thread thread-a --issue FAB-1 --area protocol \
  --reason 'Complete records must fit as a unit.' \
  'This intentionally long direction should remain pending when a tiny synchronization budget omits the complete rendered record.' >/dev/null

"${binary}" sync --thread thread-b --budget 40 >/dev/null
rg -q 'Use immutable events' .fabric/generated/SYNC_DELTA.md
rg -q 'budget was reached' .fabric/generated/SYNC_DELTA.md

record_id="$(jq -r 'select(.event_type == "record.created") | .payload.record.record_id' .fabric/ledger/events/*.json | head -n 1)"
"${binary}" challenge --direction "${record_id}" --issue FAB-1 \
  --proposal 'Use a mutable database' --reason 'Evaluate an explicit exception.' >/dev/null
rg -q 'Direction Challenge' .fabric/generated/CHALLENGE.md

"${binary}" relation add --type informed_by \
  --from action:codex:eval-action --to "record:${record_id}" \
  --actor-provider codex --actor-id eval-action >/dev/null
"${binary}" explain --node action:codex:eval-action | rg -q '\[causal\]'
"${binary}" explain --node action:codex:eval-action --json | jq -e --arg rid "${record_id}" '
  any(.data.node_details[]?;
    .ref.kind == "record" and
    .ref.id == $rid and
    .record.record.reason == "The repository needs one immutable protocol boundary.")
  and any(.data.relation_details[]?;
    .actor.kind == "agent" and
    .actor.id == "eval-action" and
    .actor.provider == "codex")
' >/dev/null

projection_id="$(jq -r --arg rid "${record_id}" '
  select(.event_type == "projection.created") |
  select(any(.payload.projection.record_ids[]?; . == $rid)) |
  .payload.projection.projection_id
' .git/fabric/runtime/projections/*.json | head -n 1)"
"${binary}" context acknowledge --projection "${projection_id}" --state exposed --provider codex >/dev/null
"${binary}" explain --node "record:${record_id}" --depth 4 --json | jq -e '
  any(.data.relations[]?; .type == "exposed_to") and
  any(.data.node_details[]?; .projection != null) and
  any(.data.node_details[]?; .thread != null)
' >/dev/null
"${binary}" expire "${record_id}" --reason 'Evaluation lifecycle completed.' >/dev/null
"${binary}" explain --node "record:${record_id}" --json | jq -e --arg rid "${record_id}" '
  any(.data.node_details[]?;
    .ref.id == $rid and
    .record.record.status == "expired" and
    .record.actor.kind == "human" and
    .record.head_actor.kind == "tool")
' >/dev/null
"${binary}" capabilities --json | jq -e '.ok and .protocol_version == "fabric/1.0"' >/dev/null
"${binary}" conformance >/dev/null

cat > review.md <<'EOF'
# PR Review Ingest
PR: 42
Issue: FAB-1
Areas:
- protocol

## Review directions
### 1
Type: preference
Durability: candidate
Review says: Keep provider logic outside the protocol core.
Reason: Provider-neutral state must remain portable.
EOF

before="$(find .fabric/ledger/events -name '*.json' | wc -l | tr -d ' ')"
"${binary}" ingest-pr --pr 42 --issue FAB-1 --area protocol --from-file review.md --dry-run >/dev/null
after="$(find .fabric/ledger/events -name '*.json' | wc -l | tr -d ' ')"
test "${before}" = "${after}"

git config user.name 'Fabric Eval'
git config user.email 'fabric-eval@example.invalid'
git add .
git commit -qm 'eval baseline'
baseline="$(git rev-parse HEAD)"

git switch -qc branch-a
"${binary}" note --candidate --global 'Branch A direction' >/dev/null
git add .fabric/ledger/events
git commit -qm 'branch A direction'

git switch -qc branch-b "${baseline}"
"${binary}" note --candidate --global 'Branch B direction' >/dev/null
git add .fabric/ledger/events
git commit -qm 'branch B direction'
git merge -q --no-edit branch-a
rg -q 'Branch A direction' .fabric/ledger/events
rg -q 'Branch B direction' .fabric/ledger/events

echo 'Fabric Local V1 executable evaluations passed.'
