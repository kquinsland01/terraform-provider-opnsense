# Story 31.0h: Quagga/BGP Field Parity

## Status

backlog

## Priority

P1 — complete before Stories 31.1–31.3.

## Evidence and Scope

Extend the existing BGP neighbor and route-map owners. Neighbor gaps include BFD, authentication, connect timer, default route, multihop, route-reflector, weight, connected-check, attribute preservation, and next-hop-self-all. Route-map gaps include AS-path/community/prefix-list references and route-map ID semantics. Use existing stable `quagga/bgp` item CRUD/toggle/reconfigure endpoints and retain the broader local FRR surface.

## Acceptance Criteria

- **Given** advanced neighbor/route-map objects, **when** imported, **then** all audited fields read back and matching configuration plans no change.
- **Given** linked policy objects, **when** updated or drifted, **then** UUID relationships and ordering remain stable through reconfigure.
- **Given** existing local-only FRR resources, **when** parity fields ship, **then** registrations and state addresses remain backward compatible.

## Verification

Quagga unit/generated tests, disposable-appliance BGP policy lifecycle proof, docs generation, and `make check`.
