#!/usr/bin/env bash
# Purpose: Hash observed implementation and publication inputs, excluding final evidence.
# Usage: story-31-0o-code-digest.sh [--help]
# Dependencies: bash, docker, python3, shasum
set -euo pipefail
if [[ "${1:-}" = --help ]]; then
  printf '%s\n' "Usage: story-31-0o-code-digest.sh [--help]" "Hash observed implementation and publication inputs, excluding final evidence."
  exit 0
fi
repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "${repo_root}"
# Include all behavior, helpers, fixtures and publication inputs; no private
# runtime files or generated acceptance evidence belongs in the source digest.
# These observations bind reviewed before/after inputs during a controlled run,
# not an adversarial immutable snapshot. Symlink inputs are unsupported.
source_roots=(internal pkg tools templates examples docs test .github)
if [[ -n "$(find "${source_roots[@]}" -type l -print -quit)" ]]; then
  printf '%s\n' 'DDNS source digest rejects symlink inputs' >&2
  exit 1
fi
paths="$(
  {
    find internal pkg tools templates examples docs test .github -type f ! -path '*/__pycache__/*' ! -name '*.pyc'
    printf '%s\n' main.go go.mod go.sum Makefile .devrail.yml CHANGELOG.md AGENTS.md DEVELOPMENT.md README.md .editorconfig \
      test/scripts/lib/log.sh test/scripts/run-story-31-0o-acceptance.sh \
      test/scripts/story-31-0o-code-digest.sh \
      test/scripts/run-story-31-0o-container.sh \
      test/scripts/gateway_group_pinned_proxy.py \
      test/scripts/gateway_group_pinned_proxy_test.py \
      test/scripts/gateway_group_runner_guard_test.py \
      test/scripts/kea_ddns_pinned_proxy.py \
      test/scripts/kea_ddns_runner_guard_test.py \
      _bmad-output/implementation-artifacts/31-0p-kea-ordered-option-linkage.md \
      _bmad-output/planning-artifacts/post-release-epics.md \
      _bmad-output/implementation-artifacts/spec-31-0o-kea-ddns-settings-owner.md \
      _bmad-output/implementation-artifacts/31-0o-kea-ddns-settings-resource.md \
      _bmad-output/implementation-artifacts/sprint-status.yaml \
      _bmad-output/implementation-artifacts/epic-31-context.md \
      _bmad-output/planning-artifacts/kea-option-ddns-api-validation.md \
      _bmad-output/planning-artifacts/support-matrix.md \
      _bmad-output/planning-artifacts/provider-ecosystem-coverage-audit.json \
      _bmad-output/planning-artifacts/provider-ecosystem-coverage-audit.md
  } | LC_ALL=C sort -u
)"
while IFS= read -r path; do
  ancestor="${path}"
  while [[ "${ancestor}" != . ]]; do
    if [[ -L "${ancestor}" ]]; then
      printf '%s\n' 'DDNS source digest rejects symlink inputs' >&2
      exit 1
    fi
    if [[ "${ancestor}" == */* ]]; then ancestor="${ancestor%/*}"; else ancestor=.; fi
  done
done <<<"${paths}"
while IFS= read -r path; do
  printf 'path %s\n' "${path}"
  shasum -a 256 "${path}"
done <<<"${paths}" | shasum -a 256 | awk '{print $1}'
