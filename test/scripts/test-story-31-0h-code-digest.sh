#!/usr/bin/env bash
# Purpose: Reject partial digest inventories even when find emitted paths.
# Usage: test/scripts/test-story-31-0h-code-digest.sh
# Dependencies: bash, mktemp, find
set -euo pipefail
repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
fixture="$(mktemp -d)"
trap 'rm -rf -- "${fixture}"' EXIT
real_find="$(command -v find)"
mkdir -p "${fixture}/bin"
cat >"${fixture}/bin/find" <<'FIND'
#!/usr/bin/env bash
exec "${FRR_REAL_FIND}" "${FRR_MISSING_INVENTORY_PATH}" "$@"
FIND
chmod +x "${fixture}/bin/find"
if FRR_REAL_FIND="${real_find}" FRR_MISSING_INVENTORY_PATH="${fixture}/missing-source-root" PATH="${fixture}/bin:${PATH}" bash "${repo_root}/test/scripts/story-31-0h-code-digest.sh" >"${fixture}/output" 2>"${fixture}/error"; then
  printf '%s\n' 'partial failed find inventory incorrectly accepted' >&2
  exit 1
fi
[[ ! -s "${fixture}/output" ]]
grep -Fq 'inventory failed' "${fixture}/error"
