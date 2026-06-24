#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
workspace="$(mktemp -d)"
binary="${workspace}/fabric"

echo "Demo workspace: ${workspace}"
echo

go_bin="${GO:-$(command -v go || true)}"
if [[ -z "${go_bin}" && -x /usr/local/go/bin/go ]]; then
  go_bin=/usr/local/go/bin/go
fi
(
  cd "${repo_root}"
  GOCACHE="${workspace}/gocache" "${go_bin}" build -o "${binary}" ./cmd/fabric
)

(
  cd "${workspace}"

  "${binary}" init
  "${binary}" thread start --id thread-c --pr 123 --issue VS-123 --area file-opening
  direction_id="$("${binary}" note --candidate --issue VS-123 --area file-opening --json \
    "Do not implement full Office preview; this is an entry-point consistency issue." | jq -r '.data.id')"
  "${binary}" review note --pr 123 --issue VS-123 --area file-opening "Reviewer rejected picker-level Office special-casing; move unsupported file handling into the shared file-open resolver."
  "${binary}" continue --pr 123 --thread thread-followup --budget 700
  challenge_id="$("${binary}" challenge --direction "${direction_id}" --pr 123 --issue VS-123 --area file-opening --proposal "Implement internal Office preview for supported Office files" --reason "Product explicitly rescoped this from entry-point consistency to preview support." --json | jq -r '.data.id')"
  "${binary}" continue --pr 123 --budget 700
  "${binary}" challenge resolve "${challenge_id}" --accepted
  "${binary}" continue --pr 123 --budget 700
  "${binary}" explain --pr 123

  echo
  echo "Generated challenge:"
  echo
  sed -n '1,120p' .fabric/generated/CHALLENGE.md

  echo
  echo "Generated continuation context:"
  echo
  sed -n '1,160p' .fabric/generated/CONTINUATION_CONTEXT.md
)
