# Story 31.0b: Kea Field and Surface Parity

## Status

backlog

## Priority

P0 — complete before Stories 31.1–31.3.

## Evidence and Scope

Add DHCPv6 peers and delegated-prefix pools, and fill audited subnet field gaps. Stable endpoint evidence in the pinned client is `/api/kea/dhcpv6/{add,get,set,del}_peer`, `/api/kea/dhcpv6/{add,get,set,del}_pd_pool`, and the existing Kea reconfigure action. PD-pool fields are `subnet_id`, `prefix`, `prefix_len`, `delegated_len`, and `description`; peer fields are `name`, `url`, and `role`. DHCPv4 subnet parity also includes DNS/domain/search, routers/static routes, NTP/time, and PXE/TFTP fields. Deprecated generic Kea v4 aliases remain intentionally omitted.

## Acceptance Criteria

- **Given** a DHCPv6 subnet, **when** peer and PD-pool resources/data sources are applied and read, **then** full CRUD/import/drift semantics use the stable item endpoints and reconfigure path.
- **Given** v4/v6 subnet fields present upstream, **when** imported, **then** no-change plans preserve every audited field.
- **Given** deprecated `kea_peer`, `kea_reservation`, or `kea_subnet` names, **when** docs are generated, **then** no duplicate alias types are registered.

## Verification

Kea unit/generated tests, disposable-appliance v4/v6 lifecycle tests, docs generation, and `make check`.
