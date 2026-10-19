#!/usr/bin/env bash
set -euo pipefail

# Runs Story 31.0d only after an external executable independently attests the
# disposable VM. The acceptance test additionally verifies OPNsense 26.7.7.
repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "${repo_root}"

required_env=(
  OPNSENSE_URI OPNSENSE_API_KEY OPNSENSE_API_SECRET OPNSENSE_ALLOW_INSECURE
  OPNSENSE_DISPOSABLE_VM_NAME OPNSENSE_DISPOSABLE_VM_UUID
  OPNSENSE_UNBOUND_IDENTITY_PROBE OPNSENSE_UNBOUND_EXPECTED_DIGEST
)
for name in "${required_env[@]}"; do
  if [[ -z "${!name:-}" ]]; then
    echo "missing required environment variable: ${name}" >&2
    exit 2
  fi
done

probe="${OPNSENSE_UNBOUND_IDENTITY_PROBE}"
if [[ "${probe}" != /* || ! -x "${probe}" ]]; then
  echo "OPNSENSE_UNBOUND_IDENTITY_PROBE must be an absolute executable path" >&2
  exit 2
fi
identity="$("${probe}")"
observed_name="$(sed -n 's/^name=//p' <<<"${identity}")"
observed_uuid="$(sed -n 's/^uuid=//p' <<<"${identity}")"
if [[ -z "${observed_name}" || -z "${observed_uuid}" || "${observed_name}" != "${OPNSENSE_DISPOSABLE_VM_NAME}" || "${observed_uuid}" != "${OPNSENSE_DISPOSABLE_VM_UUID}" ]]; then
  echo "disposable VM identity mismatch" >&2
  exit 2
fi

digest_before="$(test/scripts/story-31-0d-code-digest.sh)"
if [[ "${digest_before}" != "${OPNSENSE_UNBOUND_EXPECTED_DIGEST}" ]]; then
  echo "candidate digest mismatch: expected ${OPNSENSE_UNBOUND_EXPECTED_DIGEST}, got ${digest_before}" >&2
  exit 2
fi

mkdir -p "${repo_root}/test/acceptance-results"
artifact_dir="$(mktemp -d "${repo_root}/test/acceptance-results/story31d.XXXXXX")"
trap 'rm -rf "${artifact_dir}"' EXIT
runner_log="${artifact_dir}/runner.log"
results_dir="${artifact_dir}/results"
run_id="story31d"
summary_file="${results_dir}/${run_id}/summary.tsv"
: >"${runner_log}"

export OPNSENSE_UNBOUND_PARITY=1
export OPNSENSE_UNBOUND_IDENTITY_ATTESTATION="${observed_name}:${observed_uuid}"
set +e
TEST_REGEX='^TestAccUnboundParity$' \
  TEST_TIMEOUT=90m \
  TEST_VERBOSE=1 \
  RESULTS_DIR="${results_dir}" \
  RUN_ID="${run_id}" \
  test/scripts/run-vagrant-acceptance.sh --package ./internal/service/unbound \
  >"${runner_log}" 2>&1
test_status=$?
set -e

test_log="$(awk -F '\t' '$2 == "./internal/service/unbound" { print $3 }' "${summary_file}" 2>/dev/null || true)"
if [[ -z "${test_log}" || ! -f "${test_log}" ]]; then
  sed -E 's/(OPNSENSE_API_(KEY|SECRET)=)[^[:space:]]+/\1[REDACTED]/g' "${runner_log}"
  echo "Story 31.0d wrapper did not produce the expected package log" >&2
  exit 1
fi
sed -E 's/(OPNSENSE_API_(KEY|SECRET)=)[^[:space:]]+/\1[REDACTED]/g' "${test_log}"
if [[ ${test_status} -ne 0 ]]; then
  echo "Story 31.0d acceptance failed" >&2
  exit "${test_status}"
fi
grep -Eq -- '--- PASS: TestAccUnboundParity( |$)' "${test_log}" || { echo "missing exact PASS line for TestAccUnboundParity" >&2; exit 1; }
grep -Eq -- '--- SKIP: TestAccUnboundParity( |$)' "${test_log}" && { echo "required acceptance test skipped" >&2; exit 1; }
grep -Eq -- 'UNBOUND_SETTINGS_RESTORE equal=true' "${test_log}" || { echo "missing exact settings restore proof" >&2; exit 1; }
grep -Eq -- 'UNBOUND_SINGLETON_DESTROY unchanged=true' "${test_log}" || { echo "missing immediate singleton state-only destroy proof" >&2; exit 1; }
for namespace in forward dot; do
  line="$(grep "UNBOUND_TERRAFORM_DESTROY namespace=${namespace} " "${test_log}" || true)"
  [[ "${line}" =~ total=0($|[[:space:]]) ]] || { echo "Terraform destroy did not prove zero ${namespace} matches: ${line}" >&2; exit 1; }
done
for namespace in dnsbl forward dot; do
  line="$(grep "UNBOUND_PARITY_CLEANUP namespace=${namespace} " "${test_log}" || true)"
  [[ "${line}" =~ total=0($|[[:space:]]) ]] || { echo "cleanup did not prove zero ${namespace} matches: ${line}" >&2; exit 1; }
done

digest_after="$(test/scripts/story-31-0d-code-digest.sh)"
if [[ "${digest_after}" != "${digest_before}" ]]; then
  echo "candidate digest changed during acceptance: before=${digest_before} after=${digest_after}" >&2
  exit 1
fi
echo "UNBOUND_PARITY_EVIDENCE digest=${digest_after} vm_name=${observed_name} vm_uuid=${observed_uuid} version=26.7.7 tests=1 singleton_destroy_unchanged=true restore_equal=true cleanup_dnsbl=0 cleanup_forward=0 cleanup_dot=0"
