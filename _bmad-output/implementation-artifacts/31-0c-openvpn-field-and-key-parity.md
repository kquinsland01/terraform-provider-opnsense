# Story 31.0c: OpenVPN Field and Key Parity

## Status

review

## Priority

P0 — complete before Stories 31.1–31.3.

## Evidence and Scope

Extend the existing OpenVPN instance/client-overwrite owners for the audited token, bridge, CRL/OCSP, IPv6, credential, routing, MTU/MSS, DNS/search/NTP, and advanced-flag fields. Add the read-only static-key data source and an `openvpn_generate_key` ephemeral resource. Pinned source proves `POST /api/openvpn/instances/gen_key`, with `key_type` input and sensitive in-memory `key` output, plus existing stable OpenVPN item CRUD/get endpoints.

## Acceptance Criteria

- **Given** an imported advanced instance or client overwrite, **when** refresh runs, **then** all audited fields round-trip and a matching configuration plans no change.
- **Given** `openvpn_generate_key`, **when** apply runs, **then** it calls `gen_key`, validates the result, marks key material sensitive, and never persists it in Terraform state.
- **Given** a static-key UUID, **when** the data source reads it, **then** mode/description/key are returned with key marked sensitive and endpoint limitations documented.

## Verification

OpenVPN schema/unit tests, ephemeral state-safety test, disposable-appliance lifecycle tests, docs generation, and `make check`.

## Repair-2 Notes

- Added complete audited OpenVPN instance and client-overwrite fields with strict owner-local decoding, explicit-clear semantics, import-safe Optional+Computed handling, and recoverable reconfigure activation.
- Added the sensitive stateful static-key data source and Terraform 1.10+ ephemeral generator for the five source-supported key types.
- Fresh repair-2 proof now exercises complete resource/data-source state equality for the live server, no-pool server, client, and client-overwrite fixtures; only the two write-only credential values are excluded from equality checks.
- The static-key data source and ephemeral generator are live-proven. Instance/client-overwrite resource imports use persisted appliance objects and isolated Terraform import state, while field-complete resource/data-source comparisons cover the authoritative OPNsense read boundary.
- All six scoped audit rows are promoted after repair-2 convergence; the exact-tree repository gate remains the final pre-PR check.

## Evidence

- Repair-2 candidate digest: `0a9077a69f9048582dd92e1de06b708fd7c6de70027461a6ef3a449ef484c3e4` before and after the field-complete run.
- Disposable target: `opnsense-acceptance-102` (`aa90e347-371e-4e3b-8ba7-af7218b8a92b`), OPNsense 26.7.7.
- Fixture namespace: `tf-story31c-dm8xu0a7821f`; cleanup `instance=0`, `client-overwrite=0`; saved-plan/state/log exact-value scan clean.
- Sanitized repair-2 evidence: `31-0c-live-evidence.json` and `31-0c-live-output.txt`.
