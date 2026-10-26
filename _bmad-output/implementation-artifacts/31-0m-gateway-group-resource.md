# Story 31.0m: Gateway group resource and data source

Status: review — implemented and live-validated; maintainer PR/release pending

## Scope

Implements `opnsense_system_gateway_group` resource and UUID-keyed lookup for the
validated stable OPNsense 26.7.7 Routing API. Public schema owns immutable name,
five gateway-name tier sets, trigger, pool options and description. The stable API
cannot clear a populated first tier; the owner rejects that transition before
saving any fields. Initially empty first tiers remain supported when another
tier is populated. This is bounded ownership, not full all-tier-clear parity. Route
reconfigure does not apply PF and is not a traffic failover test.

## Acceptance contract

Given a supported disposable appliance, when Terraform Core create, read, import,
no-change, tier/trigger/pool updates, explicit clears, lookup, drift/repair and
delete execute, then all fields converge with one stable UUID. Given an in-use
group or malformed response, when delete/read fails, then tracked identity and
valid prior state remain. Given confirmed or uncertain saves/deletes and failed
activation, when returned protocol state/private bytes survive provider restart,
then authoritative recovery avoids duplicated mutations.

## Verification and remaining gates

Implementation and offline verification are recorded separately from live proof.
The identity/TLS/version guarded runner binds source/helper hashes before and
after execution, keeps raw artifacts privately, requires non-skipped Core proof,
and independently checks zero synthetic entries before baseline rollback and
again after exact config/routes restoration. Failed attempts remain distinct.

The repaired 16-step real Terraform lifecycle and independent follow-up reviews
passed. Parent owns the final publication-tree binding and sanitized evidence
linked below before PR handoff. This is validated unreleased bounded ownership,
not universal first-tier-clear completeness. No older-core, OpenTofu, real traffic
failover or production adoption claim is made.

## Offline verification

Executed in cached `ghcr.io/devrail-dev/dev-toolchain:1.12.24`:

- Gateway-group named unit/protocol tests pass with `-count=1 -race -v`, without
  skips. Actual protocol6 provider registration, returned state/private restart,
  mutation counters, cancellation, concurrency and strict missing-object controls
  are exercised. The full system package race run also passes; live acceptance
  tests are intentionally not counted as proof when `TF_ACC` is absent.
- `go test -count=1 ./...` passes.
- Two consecutive `go generate ./...` runs produce identical source/docs/audit
  content hashes. Generated inventory is 110 resources and 96 data sources.
- `make check` passes all six targets, with no suppressions; `git diff --check`,
  focused `golangci-lint`, ShellCheck, shell formatting and Terraform example
  formatting pass.

The strict 404 owner mapping is local to this gated resource; existing shared
CRUD contracts and individual-gateway lifecycle are unchanged. Non-idempotent
group writes use the authenticated single-attempt transport and reconcile
uncertain outcomes instead of inheriting automatic POST retries.

## Runner private interface

`test/scripts/run-story-31-0m-acceptance.sh` requires an absolute private
`STORY31_0M_RUNTIME_ENV` shell environment file and `STORY31_0M_PRIVATE_ROOT`
outside the publication tree. Runtime values include API URI/key/secret, TLS
policy, synthetic gateway interface/addresses, expected source digest, and
absolute guard/oracle executables. `STORY31_0M_EXPECTED_HELPER_DIGEST` is an
independently reviewed SHA256 of the combined `shasum -a 256 GUARD ORACLE`
output (including its path names), hashed again with SHA256. The runner requires
and compares it **before either helper executes**, then proves both executables
unchanged afterward. Missing/mismatched expectations fail closed before helper
execution. Private inputs and raw logs are never public.

The guard emits JSON with `ok`, `identity_verified`, `tls_verified`,
`version_verified`, `isolated` all true and `tls_sha256` (64 lowercase hex). It
must independently attest the exact disposable VM, pinned endpoint, minimum stable
version, isolation and route-reconfigure-safe management path. The acceptance-only
container launcher routes actual provider HTTP requests through a loopback proxy
with one fixed HTTPS upstream. Every upstream TLS connection verifies the exact
attested certificate digest before forwarding HTTP bytes or API credentials;
a changed certificate or any redirect is rejected. A separate one-time pin
preflight is not treated as continuous protection. The proxy and launcher are
reviewed source-digest inputs and add no provider setting or public interface.
The in-Core oracle uses the private loopback HTTP
service via `STORY31_0M_ORACLE_URL` and `STORY31_0M_ORACLE_TOKEN`, posting to
`/action` with JSON `args` matching the executable argument contract.

Oracle invocation is `oracle ACTION PRIVATE_EVIDENCE_DIR [GROUP_UUID GROUP_NAME]`.
Every successful action returns JSON `ok:true`. `snapshot` includes `core_version`,
`config_sha256`, `routes_sha256`, and zero `test_groups`/`test_gateways`.
`verify-zero` and `cleanup` include zero counts. `verify-restored` includes
`config_equal:true` and `routes_equal:true` and is read-only. `restore` performs
parent-owned baseline restoration only after the pre-restoration zero check has
been attempted. `remove-reference` includes `reference_removed:true`; `drift`
includes `changed:true`; `establish-reference` includes `eligible:true` and
`reference_established:true` for the supplied Terraform-tracked UUID. An unmet
dependency fixture returns `ok:false` and a specific private `reason`; it is not a
skip/pass. `cleanup` removes only identity-guarded synthetic test objects through
official APIs, never raw XML. Cleanup attempts all phases even after failures and
cannot promote a failed test to passing proof.

`observe-group` returns `ok:true` and an independent persisted `group` object:
scalar `name`, `descr`, `trigger`, `poolopts` and selected gateway-name arrays
`item`, `item2`, `item3`, `item4`, `item5`. The Core fixture compares every field
against explicit expectations after positive phases, including initial tier2-only
creation/update and populated/cleared tiers 3–5. Descriptions are compared
literally; only gateway selector arrays are compared order-insensitively.

## Publication proof

See [sanitized exact-tree lifecycle evidence](31-0m-gateway-group-live-evidence.md).
The evidence record is outside the executable/source digest to avoid a recursive
self-hash; all runtime source, test helpers and publication claim inputs are bound.
