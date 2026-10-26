#!/usr/bin/env bash
# Acceptance-only entry point; invoked inside the pinned dev-toolchain image.
set -euo pipefail
export STORY31_0M_UPSTREAM_URI="${OPNSENSE_URI}"
export OPNSENSE_URI=http://127.0.0.1:19532
python3 test/scripts/gateway_group_pinned_proxy.py >"${STORY31_0M_EVIDENCE_DIR}/proxy.log" 2>&1 &
proxy_pid=$!
cleanup_proxy() {
  local status="$?"
  if kill -0 "${proxy_pid}" 2>/dev/null; then kill "${proxy_pid}"; fi
  exit "${status}"
}
trap cleanup_proxy EXIT
for ((attempt = 0; attempt < 100; attempt++)); do
  [[ -f "${STORY31_0M_EVIDENCE_DIR}/proxy.ready" ]] && break
  kill -0 "${proxy_pid}"
  sleep 0.1
done
[[ -f "${STORY31_0M_EVIDENCE_DIR}/proxy.ready" ]]
go test -count=1 -v -timeout 40m -run '^TestAccGatewayGroupOwner$' ./internal/service/system
