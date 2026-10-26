#!/usr/bin/env bash
# Purpose: Run Kea DDNS Core acceptance inside the pinned toolchain.
# Usage: run-story-31-0o-container.sh [--help]
# Dependencies: bash, docker, python3, shasum
# Acceptance-only entry point; invoked inside the pinned dev-toolchain image.
set -euo pipefail
if [[ "${1:-}" = --help ]]; then
  printf '%s\n' "Usage: run-story-31-0o-container.sh [--help]" "Run Kea DDNS Core acceptance inside the pinned toolchain."
  exit 0
fi
export STORY31_0O_UPSTREAM_URI="${OPNSENSE_URI}"
export OPNSENSE_URI=http://127.0.0.1:19536
python3 test/scripts/kea_ddns_pinned_proxy.py >"${STORY31_0O_EVIDENCE_DIR}/proxy.log" 2>&1 &
proxy_pid=$!
cleanup_proxy() {
  local status="$?"
  if kill -0 "${proxy_pid}" 2>/dev/null; then kill "${proxy_pid}"; fi
  exit "${status}"
}
trap cleanup_proxy EXIT
for ((attempt = 0; attempt < 100; attempt++)); do
  [[ -f "${STORY31_0O_EVIDENCE_DIR}/proxy.ready" ]] && break
  kill -0 "${proxy_pid}"
  sleep 0.1
done
[[ -f "${STORY31_0O_EVIDENCE_DIR}/proxy.ready" ]]
go test -count=1 -v -timeout 40m -run '^TestAccDDNSSettingsOwner$' ./internal/service/kea
