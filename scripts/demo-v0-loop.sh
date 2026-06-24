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
  "${binary}" thread start --id thread-a --issue VS-123 --area virtual-store/listing
  "${binary}" thread start --id thread-b --issue VS-123 --area virtual-store/listing
  "${binary}" note --thread thread-a --issue VS-123 --area virtual-store/listing "Don't create a second listing endpoint; extend the existing one or escalate API direction"
  "${binary}" sync --thread thread-b --budget 300

  echo
  echo "Generated sync delta:"
  echo
  sed -n '1,120p' .fabric/generated/SYNC_DELTA.md
)
