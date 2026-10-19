#!/usr/bin/env bash
set -euo pipefail

# Reproducible Story 31.0c candidate digest. The scope includes every source
# that controls provider registration/configuration, the acceptance harness,
# shared HTTP lifecycle behavior, dependency locks, and this script itself.
repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "${repo_root}"

explicit_scope=(
  go.mod
  go.sum
  internal/acctest/acctest.go
  internal/coverageaudit/audit.go
  internal/provider/provider.go
  pkg/opnsense/client.go
  pkg/opnsense/crud.go
  pkg/opnsense/errors.go
  pkg/opnsense/reconfigure.go
  pkg/opnsense/reqopts.go
  pkg/opnsense/search.go
  test/scripts/run-vagrant-acceptance.sh
  test/scripts/run-story-31-0c-acceptance.sh
  test/scripts/story-31-0c-code-digest.sh
)

paths=("${explicit_scope[@]}")
while IFS= read -r -d '' path; do
  paths+=("${path}")
done < <(find internal/service/openvpn -type f -print0)

while IFS= read -r path; do
  printf 'path %s\n' "${path}"
  shasum -a 256 "${path}"
done < <(printf '%s\n' "${paths[@]}" | LC_ALL=C sort -u) \
  | shasum -a 256 \
  | awk '{print $1}'
