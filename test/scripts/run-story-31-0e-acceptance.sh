#!/usr/bin/env bash
set -euo pipefail
repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "${repo_root}"
# shellcheck source=test/scripts/lib/log.sh
. test/scripts/lib/log.sh
# Only grep exit 1 proves absence. Matches and operational failures both withhold
# every raw artifact and can never produce secret_scan=pass.
scan_secret_artifacts() {
  local artifact_dir="$1" secret_name scan_status
  for secret_name in OPNSENSE_API_KEY OPNSENSE_API_SECRET OPNSENSE_WIREGUARD_PRIVATE_KEY OPNSENSE_WIREGUARD_PSK; do
    if grep -rFq -- "${!secret_name}" "${artifact_dir}"; then
      die "secret scan failed for ${secret_name}; raw artifacts withheld"
    else
      scan_status=$?
    fi
    [[ ${scan_status} -eq 1 ]] || die "secret scan operational error for ${secret_name}; raw artifacts withheld"
  done
}
required_env=(OPNSENSE_URI OPNSENSE_API_KEY OPNSENSE_API_SECRET OPNSENSE_ALLOW_INSECURE OPNSENSE_DISPOSABLE_VM_NAME OPNSENSE_DISPOSABLE_VM_UUID OPNSENSE_WIREGUARD_IDENTITY_PROBE OPNSENSE_WIREGUARD_EXPECTED_DIGEST)
for name in "${required_env[@]}"; do
  [[ -n "${!name:-}" ]] || die "missing required environment variable: ${name}"
done
probe="${OPNSENSE_WIREGUARD_IDENTITY_PROBE}"
[[ "${probe}" = /* && -x "${probe}" ]] || die "identity probe must be an absolute executable path"
identity="$("${probe}")"
observed_name="$(sed -n 's/^name=//p' <<<"${identity}")"
observed_uuid="$(sed -n 's/^uuid=//p' <<<"${identity}")"
[[ "${observed_name}" = opnsense-acceptance-102 && "${observed_name}" = "${OPNSENSE_DISPOSABLE_VM_NAME}" && -n "${observed_uuid}" && "${observed_uuid}" = "${OPNSENSE_DISPOSABLE_VM_UUID}" ]] || die "disposable VM identity mismatch"
# Independently bind the configured API endpoint to the attested VM before any
# credentialed traffic. The probe gets this certificate through VM-owned QEMU
# forwarding over trusted SSH, not from OPNSENSE_URI.
attested_tls="$(sed -n 's/^tls_sha256=//p' <<<"${identity}")"
[[ "${attested_tls}" =~ ^[0-9a-f]{64}$ ]] || die "missing or malformed attested TLS fingerprint"
endpoint_tls="$(python3 - "${OPNSENSE_URI}" <<'TLS'
import hashlib, socket, ssl, sys, urllib.parse
uri = urllib.parse.urlsplit(sys.argv[1])
if uri.scheme != "https" or not uri.hostname or uri.username or uri.password:
    raise SystemExit("API URI must be HTTPS without credentials")
# Docker Desktop resolves this special hostname only in containers. The host
# leg uses the root-owned localhost tunnel; the Go precheck separately attests
# the exact container endpoint before constructing an authenticated client.
hostname = "127.0.0.1" if uri.hostname == "host.docker.internal" else uri.hostname
context = ssl.SSLContext(ssl.PROTOCOL_TLS_CLIENT)
context.check_hostname = False
context.verify_mode = ssl.CERT_NONE
with socket.create_connection((hostname, uri.port or 443), timeout=15) as sock:
    with context.wrap_socket(sock, server_hostname=uri.hostname) as tls:
        print(hashlib.sha256(tls.getpeercert(binary_form=True)).hexdigest())
TLS
)"
[[ "${endpoint_tls}" = "${attested_tls}" ]] || die "API endpoint TLS fingerprint mismatch"
digest_before="$(test/scripts/story-31-0e-code-digest.sh)"
[[ "${digest_before}" = "${OPNSENSE_WIREGUARD_EXPECTED_DIGEST}" ]] || die "candidate digest mismatch"
# The disposable forwarding is exposed to Docker Desktop through the host
# alias. The exact alias endpoint is checked again, unauthenticated, in Go.
if [[ "${OPNSENSE_URI}" = https://127.0.0.1:18444 ]]; then
  export OPNSENSE_URI=https://host.docker.internal:18444
fi
export OPNSENSE_WIREGUARD_TLS_SHA256="${attested_tls}"
# Keys are ephemeral, never printed, never persisted in the evidence record.
OPNSENSE_WIREGUARD_PRIVATE_KEY="$(openssl rand -base64 32)"
OPNSENSE_WIREGUARD_PSK="$(openssl rand -base64 32)"
OPNSENSE_WIREGUARD_PUBLIC_KEY="$(openssl rand -base64 32)"
export OPNSENSE_WIREGUARD_PRIVATE_KEY OPNSENSE_WIREGUARD_PSK OPNSENSE_WIREGUARD_PUBLIC_KEY
export OPNSENSE_WIREGUARD_PARITY=1
export OPNSENSE_WIREGUARD_IDENTITY_ATTESTATION="${observed_name}:${observed_uuid}"
umask 077
mkdir -p test/acceptance-results
artifact_dir="$(mktemp -d "${repo_root}/test/acceptance-results/story31e.XXXXXX")"
trap 'rm -rf "${artifact_dir}"' EXIT
set +e
TEST_REGEX='^TestAccWireGuardParity$' TEST_TIMEOUT=60m TEST_VERBOSE=1 RESULTS_DIR="${artifact_dir}/results" RUN_ID=story31e \
  test/scripts/run-vagrant-acceptance.sh --package ./internal/service/wireguard >"${artifact_dir}/runner.log" 2>&1
status=$?
set -e
# Fail closed before printing any test output if a credential/key leaked.
scan_secret_artifacts "${artifact_dir}"
summary="${artifact_dir}/results/story31e/summary.tsv"
test_log="$(awk -F '\t' '$2 == "./internal/service/wireguard" {print $3}' "${summary}" 2>/dev/null || true)"
[[ -n "${test_log}" && -f "${test_log}" ]] || die "acceptance produced no package log"
cat "${test_log}"
[[ ${status} -eq 0 ]] || die "WireGuard acceptance failed"
grep -Eq -- '--- PASS: TestAccWireGuardParity( |$)' "${test_log}" || die "missing non-skipped live PASS"
for proof in 'WIREGUARD_SINGLETON_UPDATE terraform_core=true enabled=0 refresh_disabled_fixture=true' 'WIREGUARD_SETTINGS_RESTORE equal=true' 'WIREGUARD_SINGLETON_DESTROY unchanged=true'; do
  grep -Fq "${proof}" "${test_log}" || die "missing ${proof}"
done
for kind in client server; do
  for phase in TERRAFORM_DESTROY PARITY_CLEANUP; do
    grep -Fq "WIREGUARD_${phase} namespace=${kind} total=0" "${test_log}" || die "missing zero-leftover proof"
  done
done
digest_after="$(test/scripts/story-31-0e-code-digest.sh)"
[[ "${digest_after}" = "${digest_before}" ]] || die "candidate changed during acceptance"
log_info "WIREGUARD_PARITY_EVIDENCE digest=${digest_after} version=26.7.7 vm_name=${observed_name} vm_uuid=${observed_uuid} restore_equal=true singleton_destroy_unchanged=true terraform_deleted=0 final_leftovers=0 secret_scan=pass"
