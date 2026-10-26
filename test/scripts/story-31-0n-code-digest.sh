#!/usr/bin/env bash
# Purpose: Hash immutable implementation and publication inputs, excluding final evidence.
# Usage: story-31-0n-code-digest.sh [--help]
# Dependencies: bash, docker, python3, shasum
set -euo pipefail
if [[ "${1:-}" = --help ]]; then
  printf '%s\n' "Usage: story-31-0n-code-digest.sh [--help]" "Hash immutable implementation and publication inputs, excluding final evidence."
  exit 0
fi
repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "${repo_root}"
# Include all behavior, helpers, fixtures and publication inputs; no private
# runtime files or generated acceptance evidence belongs in the source digest.
{
  find internal pkg tools templates examples docs test .github -type f ! -path '*/__pycache__/*' ! -name '*.pyc'
  printf '%s\n' main.go go.mod go.sum Makefile .devrail.yml CHANGELOG.md AGENTS.md DEVELOPMENT.md README.md .editorconfig \
    test/scripts/lib/log.sh test/scripts/run-story-31-0n-acceptance.sh \
    test/scripts/story-31-0n-code-digest.sh \
    test/scripts/run-story-31-0n-container.sh \
    test/scripts/gateway_group_pinned_proxy.py \
    test/scripts/gateway_group_pinned_proxy_test.py \
    test/scripts/gateway_group_runner_guard_test.py \
    test/scripts/kea_option_pinned_proxy.py \
    test/scripts/kea_option_runner_guard_test.py \
    _bmad-output/implementation-artifacts/31-0p-kea-ordered-option-linkage.md \
    _bmad-output/planning-artifacts/post-release-epics.md \
    _bmad-output/implementation-artifacts/spec-31-0n-kea-generic-option-owner.md \
    _bmad-output/implementation-artifacts/31-0n-kea-generic-option-resource.md \
    _bmad-output/implementation-artifacts/sprint-status.yaml \
    _bmad-output/implementation-artifacts/epic-31-context.md \
    _bmad-output/planning-artifacts/kea-option-ddns-api-validation.md \
    _bmad-output/planning-artifacts/support-matrix.md \
    _bmad-output/planning-artifacts/provider-ecosystem-coverage-audit.json \
    _bmad-output/planning-artifacts/provider-ecosystem-coverage-audit.md
} | LC_ALL=C sort -u | while IFS= read -r path; do
  printf 'path %s\n' "${path}"
  shasum -a 256 "${path}"
done | shasum -a 256 | awk '{print $1}'
