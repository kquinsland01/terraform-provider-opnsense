#!/usr/bin/env bash
set -euo pipefail

# Reproducible Story 31.0e candidate digest. The scope includes every source
# controlling WireGuard registration, schemas, lifecycle behavior, generated
# documentation, coverage claims, and acceptance execution.
repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "${repo_root}"

explicit_scope=(
  .gitleaksignore
  .devrail.yml
  Makefile
  go.mod
  go.sum
  internal/acctest/acctest.go
  internal/coverageaudit/audit.go
  internal/generate/main.go
  internal/provider/provider.go
  pkg/opnsense/types.go
  pkg/opnsense/client.go
  pkg/opnsense/crud.go
  pkg/opnsense/errors.go
  pkg/opnsense/reconfigure.go
  pkg/opnsense/reqopts.go
  pkg/opnsense/search.go
  _bmad-output/implementation-artifacts/epic-31-context.md
  _bmad-output/implementation-artifacts/31-0e-wireguard-field-and-settings-parity.md
  _bmad-output/implementation-artifacts/spec-31-0e-wireguard-api-corrected-continuation.md
  _bmad-output/implementation-artifacts/spec-31-0e-wireguard-field-and-settings-parity.md
  _bmad-output/implementation-artifacts/sprint-status.yaml
  _bmad-output/planning-artifacts/core-config-gap-analysis.md
  _bmad-output/planning-artifacts/post-release-epics.md
  _bmad-output/planning-artifacts/provider-ecosystem-coverage-audit.json
  _bmad-output/planning-artifacts/provider-ecosystem-coverage-audit.md
  _bmad-output/planning-artifacts/provider-management-roadmap-2026-09-28.md
  _bmad-output/planning-artifacts/resource-gap-verification.md
  _bmad-output/planning-artifacts/support-matrix.md
  test/scripts/lib/log.sh
  main.go
  test/scripts/run-vagrant-acceptance.sh
  test/scripts/run-story-31-0e-acceptance.sh
  test/scripts/story-31-0e-code-digest.sh
)

paths=("${explicit_scope[@]}")
while IFS= read -r -d '' path; do
  paths+=("${path}")
done < <(find internal pkg -type f -name '*.go' -print0)
while IFS= read -r -d '' path; do
  paths+=("${path}")
done < <(find internal/service/wireguard -type f -print0)
while IFS= read -r -d '' path; do
  paths+=("${path}")
done < <(find internal/tfconv -type f -print0)
while IFS= read -r -d '' path; do
  paths+=("${path}")
done < <(find templates examples docs -type f -path '*wireguard*' -print0)

while IFS= read -r path; do
  printf 'path %s\n' "${path}"
  shasum -a 256 "${path}"
done < <(printf '%s\n' "${paths[@]}" | LC_ALL=C sort -u) |
  shasum -a 256 |
  awk '{print $1}'
