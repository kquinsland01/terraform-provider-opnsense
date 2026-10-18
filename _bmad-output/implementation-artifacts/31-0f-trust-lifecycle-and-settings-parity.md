# Story 31.0f: Trust Lifecycle and Settings Parity

## Status

backlog

## Priority

P0 — complete before Stories 31.1–31.3.

## Evidence and Scope

Extend the existing CA/certificate owners for import, internal generation, intermediate/OCSP, external CSR/sign/import/reissue actions, subject/SAN/EKU, digest/key type/lifetime, and operational metadata. Add the certificate data source and settings singleton resource/data source. Stable settings evidence is `GET /api/trust/settings/get`, `POST /api/trust/settings/set`, and `POST /api/trust/settings/reconfigure`; existing trust CA/cert endpoints own object lifecycles. Private-key values are sensitive and local import inputs are currently write-only.

## Acceptance Criteria

- **Given** each supported CA/certificate action, **when** create/import/read/update/delete or state-only lifecycle applies, **then** the action-specific contract is validated and no secret is exposed without `Sensitive`.
- **Given** a certificate UUID, **when** the data source reads it, **then** public metadata and API-returned sensitive fields match documented state semantics.
- **Given** imported trust settings, **when** updated, **then** all six audited settings round-trip and reconfigure; destroy removes state only.

## Verification

Trust unit/schema tests, action-matrix disposable-appliance acceptance, docs generation, and `make check`.
