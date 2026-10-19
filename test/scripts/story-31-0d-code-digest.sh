#!/usr/bin/env bash
set -euo pipefail

# Reproducible Story 31.0d candidate digest. The scope includes every source
# controlling Unbound registration, schemas, lifecycle behavior, generated
# documentation, coverage claims, and acceptance execution.
repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "${repo_root}"

explicit_scope=(
  go.mod
  go.sum
  internal/acctest/acctest.go
  internal/coverageaudit/audit.go
  internal/generate/main.go
  internal/generate/schemas/unbound.yaml
  internal/provider/provider.go
  pkg/opnsense/types.go
  pkg/opnsense/client.go
  pkg/opnsense/crud.go
  pkg/opnsense/errors.go
  pkg/opnsense/reconfigure.go
  pkg/opnsense/reqopts.go
  pkg/opnsense/search.go
  _bmad-output/implementation-artifacts/31-0d-unbound-settings-and-forward-parity.md
  _bmad-output/implementation-artifacts/spec-31-0d-unbound-settings-and-forward-parity.md
  _bmad-output/implementation-artifacts/sprint-status.yaml
  _bmad-output/planning-artifacts/core-config-gap-analysis.md
  _bmad-output/planning-artifacts/post-release-epics.md
  _bmad-output/planning-artifacts/provider-ecosystem-coverage-audit.json
  _bmad-output/planning-artifacts/provider-ecosystem-coverage-audit.md
  _bmad-output/planning-artifacts/provider-management-roadmap-2026-09-28.md
  _bmad-output/planning-artifacts/resource-gap-verification.md
  _bmad-output/planning-artifacts/support-matrix.md
  test/scripts/run-vagrant-acceptance.sh
  test/scripts/run-story-31-0d-acceptance.sh
  test/scripts/story-31-0d-code-digest.sh
)

paths=("${explicit_scope[@]}")
while IFS= read -r -d '' path; do
  paths+=("${path}")
done < <(find pkg/opnsense -maxdepth 1 -type f -name '*_test.go' -print0)
while IFS= read -r -d '' path; do
  paths+=("${path}")
done < <(find internal/service/unbound -type f -print0)
while IFS= read -r -d '' path; do
  paths+=("${path}")
done < <(find internal/tfconv -type f -print0)
while IFS= read -r -d '' path; do
  paths+=("${path}")
done < <(find templates examples docs -type f -path '*unbound*' -print0)

while IFS= read -r path; do
  printf 'path %s\n' "${path}"
  shasum -a 256 "${path}"
done < <(printf '%s\n' "${paths[@]}" | LC_ALL=C sort -u) \
  | shasum -a 256 \
  | awk '{print $1}'
