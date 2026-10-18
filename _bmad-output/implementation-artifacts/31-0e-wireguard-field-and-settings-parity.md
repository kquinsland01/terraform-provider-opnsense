# Story 31.0e: WireGuard Field and Settings Parity

## Status

backlog

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
