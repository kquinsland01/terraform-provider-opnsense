# Story 31.0a: Firewall and NAT Field Parity

## Status

backlog

## Priority

P0 — complete before Stories 31.1–31.3.

## Evidence and Scope

The pinned v0.26.0 audit found material fields missing from the existing stable endpoint owners: alias `interface`/`path_expression`/`stats`; filter-rule tagging, priority, schedules, state limits/timeouts, TCP/ICMP, reply-to, shaping, and HA-sync controls; and multi-interface port forwards. Extend the existing resources/data sources—do not create competing owners. Existing CRUD/apply endpoints under `firewall/alias`, `firewall/filter`, and `firewall/destination_nat` are stable and already used by this provider.

## Acceptance Criteria

- **Given** the pinned schemas and current OPNsense models, **when** the three local schemas are extended, **then** every audited field is either round-tripped or explicitly rejected with source-backed rationale.
- **Given** imported advanced rules, **when** refresh and no-change plan run, **then** state matches API read-back without flattening or losing nested semantics.
- **Given** create/update/delete, **when** mutations finish, **then** the existing savepoint/apply/cancel-rollback lifecycle remains intact and acceptance tests prove drift repair.

## Verification

`go test ./internal/service/firewall/...`, generated-doc drift check, disposable-appliance acceptance, and `make check`.
