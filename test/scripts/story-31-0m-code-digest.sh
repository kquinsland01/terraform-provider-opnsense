#!/usr/bin/env bash
set -euo pipefail
repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "${repo_root}"
# Include all behavior, helpers, fixtures and publication inputs; no private
# runtime files or generated acceptance evidence belongs in the source digest.
{
  find internal pkg tools templates examples docs -type f
  printf '%s\n' main.go go.mod go.sum Makefile .devrail.yml CHANGELOG.md \
    test/scripts/lib/log.sh test/scripts/run-story-31-0m-acceptance.sh \
    test/scripts/story-31-0m-code-digest.sh \
    test/scripts/run-story-31-0m-container.sh \
    test/scripts/gateway_group_pinned_proxy.py \
    test/scripts/gateway_group_pinned_proxy_test.py \
    test/scripts/gateway_group_runner_guard_test.py \
    _bmad-output/implementation-artifacts/spec-31-0m-gateway-group-owner.md \
    _bmad-output/implementation-artifacts/31-0m-gateway-group-resource.md \
    _bmad-output/implementation-artifacts/sprint-status.yaml \
    _bmad-output/implementation-artifacts/epic-31-context.md \
    _bmad-output/planning-artifacts/gateway-group-api-validation.md \
    _bmad-output/planning-artifacts/support-matrix.md \
    _bmad-output/planning-artifacts/provider-ecosystem-coverage-audit.json \
    _bmad-output/planning-artifacts/provider-ecosystem-coverage-audit.md
} | LC_ALL=C sort -u | while IFS= read -r path; do
  printf 'path %s\n' "${path}"
  shasum -a 256 "${path}"
done | shasum -a 256 | awk '{print $1}'
