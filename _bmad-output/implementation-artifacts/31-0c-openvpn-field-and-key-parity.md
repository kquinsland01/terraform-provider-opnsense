# Story 31.0c: OpenVPN Field and Key Parity

## Status

backlog

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
