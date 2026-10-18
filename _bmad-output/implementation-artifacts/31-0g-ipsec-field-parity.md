# Story 31.0g: IPsec Field Parity

## Status

backlog

## Priority

P1 — complete before Stories 31.1–31.3.

## Evidence and Scope

Extend the existing connection and child owners for the audited stable swanctl model fields. Connection gaps include DPD, ports, MOBIKE, lifetimes, local addresses, pools, certificate-send, UDP encapsulation, and keying controls. Child gaps include close/DPD actions, install-policies, request ID, SHA256-96, and rekey time. Keep existing `/api/ipsec/swanctl/*` item CRUD and reconfigure ownership; do not add parallel resource types.

## Acceptance Criteria

- **Given** imported advanced connection/child objects, **when** read and plan run, **then** every audited model field round-trips without loss or perpetual diff.
- **Given** update and drift, **when** apply runs, **then** the existing reconfigure lifecycle repairs only the intended object.
- **Given** secret/auth child objects, **when** schemas are reviewed, **then** sensitive/write-only boundaries remain isolated to their owning resources.

## Verification

IPsec unit/schema tests, disposable-appliance connection/child lifecycle proof, docs generation, and `make check`.
