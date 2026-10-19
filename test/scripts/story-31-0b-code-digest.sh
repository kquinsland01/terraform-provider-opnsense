#!/usr/bin/env bash
set -euo pipefail

# Reproducible Story 31.0b implementation/test digest. This hashes sorted scope
# paths and their contents directly, so it is independent of HEAD, staging, and
# dirty-worktree state. The script and acceptance runner are part of the scope.
repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "${repo_root}"

explicit_scope=(
  internal/generate/main.go
  internal/generate/templates.go
  internal/generate/schemas/kea.yaml
  internal/tfconv/tfconv.go
  pkg/opnsense/crud.go
  pkg/opnsense/crud_test.go
  pkg/opnsense/errors.go
  pkg/opnsense/errors_test.go
  pkg/opnsense/reqopts.go
  pkg/opnsense/reconfigure.go
  pkg/opnsense/reconfigure_test.go
  test/scripts/run-vagrant-acceptance.sh
  test/scripts/story-31-0b-code-digest.sh
)

paths=("${explicit_scope[@]}")
while IFS= read -r -d '' path; do
  paths+=("${path}")
done < <(find internal/service/kea -type f -print0)

while IFS= read -r path; do
  printf 'path %s\n' "${path}"
  shasum -a 256 "${path}"
done < <(printf '%s\n' "${paths[@]}" | LC_ALL=C sort -u) \
  | shasum -a 256 \
  | awk '{print $1}'
