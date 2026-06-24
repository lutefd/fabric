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
  "${binary}" thread start --id thread-a --issue VS-123 --area virtual-store/listing
  "${binary}" thread start --id thread-b --issue VS-123 --area virtual-store/listing
  "${binary}" note --thread thread-a --issue VS-123 --area virtual-store/listing "Don't create a second listing endpoint; extend the existing one or escalate API direction"
  "${binary}" sync --thread thread-b --budget 300

  echo
  echo "Generated sync delta:"
  echo
  sed -n '1,120p' .fabric/generated/SYNC_DELTA.md
)
