#!/usr/bin/env bash
# Purpose: Run parent-guarded disposable Kea option acceptance with private evidence.
# Usage: run-story-31-0n-acceptance.sh [--help]
# Dependencies: bash, docker, python3, shasum
set -euo pipefail
if [[ "${1:-}" = --help ]]; then
  printf '%s\n' "Usage: run-story-31-0n-acceptance.sh [--help]" "Run parent-guarded disposable Kea option acceptance with private evidence."
  exit 0
fi
repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "${repo_root}"
# shellcheck source=test/scripts/lib/log.sh
. test/scripts/lib/log.sh
: "${STORY31_0N_RUNTIME_ENV:?private runtime environment file required}"
[[ -f "${STORY31_0N_RUNTIME_ENV}" && "${STORY31_0N_RUNTIME_ENV}" = /* ]] || die "runtime environment must be an absolute private file"
# Runtime inputs are supplied by the parent/operator, never committed.
runtime_digest_before="$(shasum -a 256 "${STORY31_0N_RUNTIME_ENV}" | awk '{print $1}')"
set -a
# shellcheck source=/dev/null
. "${STORY31_0N_RUNTIME_ENV}"
set +a
required=(STORY31_0N_EXPECTED_ORACLE_DIGEST STORY31_0N_EXPECTED_HELPER_DIGEST STORY31_0N_ORACLE_URL STORY31_0N_ORACLE_TOKEN STORY31_0N_GUARD STORY31_0N_ORACLE STORY31_0N_PRIVATE_ROOT STORY31_0N_EXPECTED_DIGEST OPNSENSE_URI OPNSENSE_API_KEY OPNSENSE_API_SECRET OPNSENSE_ALLOW_INSECURE)
for name in "${required[@]}"; do [[ -n "${!name:-}" ]] || die "missing acceptance environment ${name}"; done
for name in STORY31_0N_GUARD STORY31_0N_ORACLE; do [[ "${!name}" = /* && -x "${!name}" ]] || die "${name} must be an absolute executable"; done
[[ "${STORY31_0N_PRIVATE_ROOT}" = /* && -d "${STORY31_0N_PRIVATE_ROOT}" && "${STORY31_0N_PRIVATE_ROOT}" != "${repo_root}"* ]] || die "private root must exist outside publication tree"
helper_digest_before="$(shasum -a 256 "${STORY31_0N_GUARD}" "${STORY31_0N_ORACLE}" | shasum -a 256 | awk '{print $1}')"
[[ "${STORY31_0N_EXPECTED_HELPER_DIGEST}" =~ ^[0-9a-f]{64}$ && "${helper_digest_before}" = "${STORY31_0N_EXPECTED_HELPER_DIGEST}" ]] || die "independently expected helper digest mismatch; no helper executed"
verify_helper_integrity() {
  local current
  current="$(shasum -a 256 "${STORY31_0N_GUARD}" "${STORY31_0N_ORACLE}" | shasum -a 256 | awk '{print $1}')" || return 1
  [[ "${current}" = "${STORY31_0N_EXPECTED_HELPER_DIGEST}" ]] || {
    log_error "helper integrity mismatch; changed helper not executed"
    return 1
  }
}
umask 077
STORY31_0N_EVIDENCE_DIR="$(mktemp -d "${STORY31_0N_PRIVATE_ROOT}/kea-option-attempt.XXXXXX")"
export STORY31_0N_EVIDENCE_DIR
image=ghcr.io/devrail-dev/dev-toolchain:1.12.24
# Validate required attestations without printing potentially private JSON.
validate_json() {
  python3 - "$1" "$2" <<'PY'
import json, os, re, sys
with open(sys.argv[2], encoding="utf-8") as stream:
    value = json.load(stream)
def require(condition, message):
    if not condition:
        raise SystemExit(message)
action = sys.argv[1]
require(value.get("ok") is True, "oracle action failed")
if action == "guard":
    for key in ("identity_verified", "tls_verified", "version_verified", "isolated"):
        require(value.get(key) is True, key + " missing")
    require(re.fullmatch(r"[0-9a-f]{64}", value.get("tls_sha256", "")), "missing TLS pin")
if action in ("snapshot", "verify-restored"):
    expected = os.environ["STORY31_0N_EXPECTED_ORACLE_DIGEST"]
    require(re.fullmatch(r"[0-9a-f]{64}", expected) and value.get("server_sha256") == expected, "oracle source attestation mismatch")
if action == "snapshot":
    version = value.get("core_version", "")
    require(re.fullmatch(r"[0-9]+\.[0-9]+\.[0-9]+(?:_[0-9]+)?", version), "missing stable core version")
    require(tuple(map(int, version.split("_")[0].split("."))) >= (26, 7, 7), "unsupported core version")
    for key in ("config_sha256", "routes_sha256", "runtime_sha256"):
        require(re.fullmatch(r"[0-9a-f]{64}", value.get(key, "")), key + " missing")
if action in ("snapshot", "verify-zero", "cleanup", "verify-restored"):
    require(value.get("services_inactive") is True, "services must remain inactive")
if action in ("snapshot", "verify-zero", "cleanup"):
    for key in ("test_options", "test_subnets", "test_reservations"):
        require(type(value.get(key)) is int and value[key] == 0, "test leftovers or invalid counts")
if action == "verify-restored":
    require(value.get("config_equal") is True and value.get("routes_equal") is True and value.get("runtime_equal") is True, "baseline not restored")
if action == "remove-reference":
    require(value.get("reference_removed") is True, "reference remains")
PY
}
verify_helper_integrity || die "guard integrity gate failed"
"${STORY31_0N_GUARD}" >"${STORY31_0N_EVIDENCE_DIR}/guard.json"
validate_json guard "${STORY31_0N_EVIDENCE_DIR}/guard.json"
STORY31_0N_TLS_SHA256="$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1]))["tls_sha256"])' "${STORY31_0N_EVIDENCE_DIR}/guard.json")"
export STORY31_0N_TLS_SHA256
STORY31_0N_ATTESTED=true
export STORY31_0N_ATTESTED
run_oracle() {
  local action="$1"
  verify_helper_integrity || return 1
  "${STORY31_0N_ORACLE}" "${action}" "${STORY31_0N_EVIDENCE_DIR}" >"${STORY31_0N_EVIDENCE_DIR}/${action}.json" 2>"${STORY31_0N_EVIDENCE_DIR}/${action}.stderr" && validate_json "${action}" "${STORY31_0N_EVIDENCE_DIR}/${action}.json"
}
run_oracle snapshot || die "baseline snapshot gate failed; private attempt retained"
digest_before="$(docker run --rm -v "${repo_root}:/workspace" -w /workspace "${image}" bash -c 'test/scripts/story-31-0n-code-digest.sh')"
[[ "${digest_before}" = "${STORY31_0N_EXPECTED_DIGEST}" ]] || die "source digest does not match reviewed candidate"
printf '%s\n' "${helper_digest_before}" >"${STORY31_0N_EVIDENCE_DIR}/helper-sha256.before"
cleanup_done=false
cleanup() {
  local status="$?" cleanup_status=0 action
  [[ "${cleanup_done}" = false ]] || return "${status}"
  cleanup_done=true
  if [[ -n "${env_file:-}" ]]; then rm -f "${env_file}"; fi
  # Every phase is attempted and every failed gate retained, even after a
  # failed Core apply. Restoration never turns a failed ownership check green.
  for action in remove-reference cleanup verify-zero restore verify-restored; do
    if ! run_oracle "${action}"; then
      cleanup_status=1
      log_error "acceptance cleanup gate failed: ${action}; private attempt retained"
    fi
    if [[ "${action}" = verify-zero ]]; then
      if ! cp "${STORY31_0N_EVIDENCE_DIR}/verify-zero.json" "${STORY31_0N_EVIDENCE_DIR}/before-restore-zero.json"; then cleanup_status=1; fi
    fi
  done
  if ! run_oracle verify-zero; then
    cleanup_status=1
    log_error "post-restoration zero gate failed"
  fi
  digest_after="$(docker run --rm -v "${repo_root}:/workspace" -w /workspace "${image}" bash -c 'test/scripts/story-31-0n-code-digest.sh')"
  helper_digest_after="$(shasum -a 256 "${STORY31_0N_GUARD}" "${STORY31_0N_ORACLE}" | shasum -a 256 | awk '{print $1}')"
  printf '%s\n' "${helper_digest_after}" >"${STORY31_0N_EVIDENCE_DIR}/helper-sha256.after"
  runtime_digest_after="$(shasum -a 256 "${STORY31_0N_RUNTIME_ENV}" | awk '{print $1}')"
  if [[ "${runtime_digest_after}" != "${runtime_digest_before}" ]]; then
    cleanup_status=1
    log_error "private runtime inputs changed during acceptance"
  fi
  if [[ "${helper_digest_after}" != "${helper_digest_before}" ]]; then
    cleanup_status=1
    log_error "guard/oracle helper changed during acceptance"
  fi
  if [[ "${digest_after}" != "${digest_before}" ]]; then
    cleanup_status=1
    log_error "source changed during acceptance"
  fi
  if [[ ${status} -eq 0 && ${cleanup_status} -eq 0 ]]; then
    log_info "KEA_OPTION_ACCEPTANCE digest=${digest_after} terraform_core=true before_restore_options=0 before_restore_subnets=0 before_restore_reservations=0 final_options=0 final_subnets=0 final_reservations=0 config_equal=true routes_equal=true runtime_equal=true services_inactive=true packet_delivery=unverified"
  else
    log_error "Kea option acceptance incomplete; failed attempt and cleanup evidence retained privately"
  fi
  if [[ ${status} -eq 0 && ${cleanup_status} -eq 0 ]]; then exit 0; else exit 1; fi
}
trap cleanup EXIT
# Env-file lives only in the private attempt and contains ephemeral credentials.
env_file="${STORY31_0N_EVIDENCE_DIR}/container.env"
for name in "${required[@]}" STORY31_0N_EVIDENCE_DIR STORY31_0N_TLS_SHA256 STORY31_0N_ATTESTED; do
  [[ "${!name}" != *$'\n'* ]] || die "multiline runtime values are unsupported"
  printf '%s=%s\n' "${name}" "${!name}" >>"${env_file}"
done
printf 'TF_ACC=1\n' >>"${env_file}"

set +e
docker run --rm -v "${repo_root}:/workspace" -v "${STORY31_0N_PRIVATE_ROOT}:${STORY31_0N_PRIVATE_ROOT}" \
  -v gateway-owner-gomod:/root/go/pkg/mod -v gateway-owner-gocache:/root/.cache/go-build \
  -w /workspace --env-file "${env_file}" "${image}" bash -c \
  'test/scripts/run-story-31-0n-container.sh' \
  >"${STORY31_0N_EVIDENCE_DIR}/core.log" 2>&1
status=$?
set -e
rm -f "${env_file}"
[[ ${status} -eq 0 ]] || die "Core acceptance failed; private attempt retained"
# Only fixed sanitized proof is emitted; never echo credentials/raw config/logs.
for proof in '--- PASS: TestAccDHCPv4OptionOwner ' 'KEA_OPTION_CORE create=true import=true no_change=true lookup=true all_fields=true defaults=true optional_preservation=true explicit_false=true matching_clears=true encoding_only=true mutable_description=true drift_repair=true in_use_identity=true' 'KEA_OPTION_TERRAFORM_DESTROY test_options=0 test_subnets=0 test_reservations=0'; do
  grep -Fq -- "${proof}" "${STORY31_0N_EVIDENCE_DIR}/core.log" || die "required non-skipped Core proof missing"
done
