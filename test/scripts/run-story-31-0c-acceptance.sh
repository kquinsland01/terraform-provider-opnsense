#!/usr/bin/env bash
set -euo pipefail

# Runs the complete Story 31.0c proof only after an external executable has
# independently identified the disposable VM. The probe must print exactly:
#   name=<hypervisor-observed-name>
#   uuid=<hypervisor-observed-uuid>

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "${repo_root}"

required_env=(
  OPNSENSE_URI OPNSENSE_API_KEY OPNSENSE_API_SECRET OPNSENSE_ALLOW_INSECURE
  OPNSENSE_DISPOSABLE_VM_NAME OPNSENSE_DISPOSABLE_VM_UUID
  OPNSENSE_OPENVPN_IDENTITY_PROBE OPNSENSE_OPENVPN_EXPECTED_DIGEST
  OPNSENSE_OPENVPN_TEST_CA OPNSENSE_OPENVPN_TEST_CERT OPNSENSE_OPENVPN_TEST_CRL
  OPNSENSE_OPENVPN_TEST_STATIC_KEY_UUID OPNSENSE_OPENVPN_TEST_LOCAL_GROUP
  OPNSENSE_OPENVPN_TEST_CARP
)
for name in "${required_env[@]}"; do
  if [[ -z "${!name:-}" ]]; then
    echo "missing required environment variable: ${name}" >&2
    exit 2
  fi
done

probe="${OPNSENSE_OPENVPN_IDENTITY_PROBE}"
if [[ "${probe}" != /* || ! -x "${probe}" ]]; then
  echo "OPNSENSE_OPENVPN_IDENTITY_PROBE must be an absolute executable path" >&2
  exit 2
fi
identity="$("${probe}")"
observed_name="$(sed -n 's/^name=//p' <<<"${identity}")"
observed_uuid="$(sed -n 's/^uuid=//p' <<<"${identity}")"
if [[ -z "${observed_name}" || -z "${observed_uuid}" || "${observed_name}" != "${OPNSENSE_DISPOSABLE_VM_NAME}" || "${observed_uuid}" != "${OPNSENSE_DISPOSABLE_VM_UUID}" ]]; then
  echo "disposable VM identity mismatch: expected name/uuid do not match independent probe" >&2
  exit 2
fi

digest_before="$(test/scripts/story-31-0c-code-digest.sh)"
if [[ "${digest_before}" != "${OPNSENSE_OPENVPN_EXPECTED_DIGEST}" ]]; then
  echo "candidate digest mismatch: expected ${OPNSENSE_OPENVPN_EXPECTED_DIGEST}, got ${digest_before}" >&2
  exit 2
fi

mkdir -p "${repo_root}/test/acceptance-results"
artifact_dir="$(mktemp -d "${repo_root}/test/acceptance-results/story31c.XXXXXX")"
trap 'rm -rf "${artifact_dir}"' EXIT
runner_log="${artifact_dir}/runner.log"
results_dir="${artifact_dir}/results"
run_id="story31c"
summary_file="${results_dir}/${run_id}/summary.tsv"
terraform_log="${artifact_dir}/terraform.log"
terraform_workdir="${artifact_dir}/terraform-core"
: >"${terraform_log}"
mkdir -p "${terraform_workdir}"
: >"${runner_log}"

export OPNSENSE_OPENVPN_PARITY=1
export OPNSENSE_OPENVPN_IDENTITY_ATTESTATION="${observed_name}:${observed_uuid}"
export TF_LOG=INFO
export TF_LOG_PATH="/workspace/${terraform_log#"${repo_root}/"}"
export OPNSENSE_OPENVPN_TF_WORKDIR="/workspace/${terraform_workdir#"${repo_root}/"}"

set +e
TEST_REGEX='^(TestAccOpenVPNAdvancedParity|TestAccOpenVPNStaticKeyLookup|TestAccOpenVPNGenerateKeyEphemeralProtocol)$' \
  TEST_TIMEOUT=90m \
  TEST_VERBOSE=1 \
  RESULTS_DIR="${results_dir}" \
  RUN_ID="${run_id}" \
  test/scripts/run-vagrant-acceptance.sh --package ./internal/service/openvpn \
  >"${runner_log}" 2>&1
test_status=$?
set -e
test_log="$(awk -F '\t' '$2 == "./internal/service/openvpn" { print $3 }' "${summary_file}" 2>/dev/null || true)"
if [[ -z "${test_log}" || ! -f "${test_log}" ]]; then
  sed -E 's/(OPNSENSE_API_(KEY|SECRET)=)[^[:space:]]+/\1[REDACTED]/g' "${runner_log}"
  echo "Story 31.0c acceptance wrapper did not produce the expected package log" >&2
  exit 1
fi
if grep -E -q -- 'BEGIN OpenVPN Static key|story31c-password|story31c-token-secret' "${test_log}" "${terraform_log}"; then
  echo "secret material appeared in acceptance artifacts; output withheld" >&2
  exit 1
fi
sed -E 's/(OPNSENSE_API_(KEY|SECRET)=)[^[:space:]]+/\1[REDACTED]/g' "${test_log}"
if [[ ${test_status} -ne 0 ]]; then
  echo "Story 31.0c acceptance failed" >&2
  exit "${test_status}"
fi

required_passes=(
  TestAccOpenVPNAdvancedParity
  TestAccOpenVPNStaticKeyLookup
  TestAccOpenVPNGenerateKeyEphemeralProtocol
)
for test_name in "${required_passes[@]}"; do
  grep -Eq -- "--- PASS: ${test_name}( |$)" "${test_log}" || {
    echo "missing exact PASS line for ${test_name}" >&2
    exit 1
  }
  if grep -Eq -- "--- SKIP: ${test_name}( |$)" "${test_log}"; then
    echo "required acceptance test skipped: ${test_name}" >&2
    exit 1
  fi
done

cleanup_count="$(grep -c 'OPENVPN_PARITY_CLEANUP owner=' "${test_log}" || true)"
if [[ "${cleanup_count}" != "2" ]]; then
  echo "expected two parsed cleanup lines, found ${cleanup_count}" >&2
  exit 1
fi
for owner in instance client-overwrite; do
  line="$(grep 'OPENVPN_PARITY_CLEANUP owner=' "${test_log}" | grep "owner=${owner} ")"
  total="$(sed -n 's/.* total=\([0-9][0-9]*\).*/\1/p' <<<"${line}")"
  if [[ "${total}" != "0" ]]; then
    echo "cleanup did not prove zero ${owner} matches: ${line}" >&2
    exit 1
  fi
done

digest_after="$(test/scripts/story-31-0c-code-digest.sh)"
if [[ "${digest_after}" != "${digest_before}" ]]; then
  echo "candidate digest changed during acceptance: before=${digest_before} after=${digest_after}" >&2
  exit 1
fi

echo "OPENVPN_PARITY_EVIDENCE digest=${digest_after} vm_name=${observed_name} vm_uuid=${observed_uuid} tests=3 cleanup_instance=0 cleanup_client_overwrite=0 secret_scan=clean"
