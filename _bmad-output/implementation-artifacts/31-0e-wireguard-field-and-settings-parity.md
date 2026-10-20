# Story 31.0e: WireGuard Field and Settings Parity

## Status

review

## Priority

P0 — complete before Stories 31.1–31.3.

## Evidence and Scope

Extend existing peer/server owners for PSK, typed multi-address fields, route suppression/gateway, DNS, MTU, peer bindings, public key, and instance number. Add the settings singleton resource/data source using stable `GET /api/wireguard/general/get` and `POST /api/wireguard/general/set`; preserve the existing service reconfigure lifecycle. Private keys/PSKs remain sensitive and API write-only behavior must be explicit.

## Acceptance Criteria

- **Given** advanced peer/server configurations, **when** import and refresh run, **then** all API-readable fields produce a no-change plan and write-only secrets remain stable.
- **Given** the daemon singleton, **when** imported and updated, **then** `enabled` round-trips through general get/set and service reconfigure.
- **Given** singleton destroy, **when** applied, **then** it removes state only and does not disable or delete upstream configuration.

## Verification

WireGuard unit/schema tests, disposable-appliance lifecycle/drift proof, docs generation, and `make check`.

## Implementation Progress (2026-10-20)

Implemented typed peer/server parity, versioned scalar-state upgrades, strict decoding and owner-locked lifecycle recovery, Sensitive resource-only keys, and import-first daemon settings with state-only destroy. Added generated documentation, examples, registration inventory (109 resources/95 data sources), boundary tests and the digest-bound disposable runner.

The corrected continuation proves omitted create `instance`, authoritative assigned-instance refresh/update, and peer Update passthrough of the live `servers` backreference under the owner lock (the released controller otherwise detaches associations). The exact released empty AsList sentinel is normalized without accepting malformed/mixed selectors. Unknown values diagnose before mutation.

The complete 21-step disposable OPNsense 26.7.7 lifecycle passed: real Terraform Core singleton Update using an isolated stale-state fixture (appliance always disabled), create/read/import/no-change/update, populated-to-empty, drift/repair, resource/data-source comparisons, state-only singleton destroy, immediate Terraform-delete zeros, final cleanup zeros, exact settings restoration, and artifact secret scans. Exactly six audited WireGuard rows are promoted; inventory is 109 resources / 95 data sources. No duplicate client owner or unaudited fields were added.

Focused/full Go tests, full DevRail 1.12.24 six-gate validation, changed-runner shellcheck, structure/whitespace checks, and deterministic generation form the local gate. Final promoted-tree digest and executed verification results are recorded in [the live evidence](31-0e-wireguard-live-evidence.md). Four historical scanner fingerprints remain narrowly classified as deterministic Unbound private-state identifiers under the existing repository policy; scanner rules/full-history coverage are unchanged.

Implementation is in **review**, not merged, released or independently approved. The governing corrected build is locally complete; maintainer PR review and merge remain separate gates. Disposable provisioning/tunnel/credential retirement remains root-owned.


### Review repair 1

Repaired omitted Optional+Computed numeric planning using Config-null-only unset
resolution, retained confirmed saved key rotations on activation failure, and
recorded explicit private uncertainty for genuinely ambiguous unreadable secrets.
The recovery guide now describes same-UUID supported release/re-import after
independent key verification; refresh never claims an unreadable key confirmed.
Pending singleton saves survive failed read-back and protocol restart; recovered
pending updates require a fresh plan rather than falsely accepting changed intent.

Added registered version0 callback/protocol upgrade assertions, exact legacy
members/ports/private-key/description preservation, in-place default normalization
and no-replacement/no-change convergence,
serialized protocol-private failure/restart/refresh tests, initial and retry Delete
faults for both owners, resource-entry-point concurrency handshakes, omitted-field
Core lifecycle, generated key/instance stability and independent raw API checks.
Isolated negative controls prove missing owner locks and wrong upgrade registration
keys fail tests. Runner attestation binds host and actual container TLS fingerprints
before authentication; missing/malformed/mismatch tests emit no HTTP requests.
Digest sensitivity covers all transitive Go source plus the runner logging library.
No OpenTofu or earlier-version acceptance execution is claimed.
