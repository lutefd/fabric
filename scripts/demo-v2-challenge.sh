#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
workspace="$(mktemp -d)"
binary="${workspace}/fabric"

echo "Demo workspace: ${workspace}"
echo

GOCACHE="${workspace}/gocache" GOMODCACHE="${workspace}/gomodcache" go build -o "${binary}" "${repo_root}/cmd/fabric"

(
  cd "${workspace}"

  "${binary}" init
  "${binary}" thread start --id thread-c --pr 123 --issue VS-123 --area file-opening
  "${binary}" note --issue VS-123 --area file-opening "Do not implement full Office preview; this is an entry-point consistency issue."
  "${binary}" review note --pr 123 --issue VS-123 --area file-opening "Reviewer rejected picker-level Office special-casing; move unsupported file handling into the shared file-open resolver."
  "${binary}" continue --pr 123 --thread thread-followup --budget 700
  "${binary}" challenge --direction evt_000001 --pr 123 --issue VS-123 --area file-opening --proposal "Implement internal Office preview for supported Office files" --reason "Product explicitly rescoped this from entry-point consistency to preview support."
  "${binary}" continue --pr 123 --budget 700
  "${binary}" challenge resolve evt_000003 --accepted
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
