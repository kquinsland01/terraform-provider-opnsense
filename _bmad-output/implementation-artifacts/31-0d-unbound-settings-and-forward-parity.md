# Story 31.0d: Unbound Settings and Forward Parity

## Status

backlog

## Priority

P0 — complete before Stories 31.1–31.3.

## Evidence and Scope

Keep `opnsense_unbound_general`, `opnsense_unbound_dnsbl`, and `opnsense_unbound_domain_override` as the object owners. Extend them (or consolidate with state-migration support) to cover pinned `unbound_settings` advanced cache/logging/privacy/serve-expired, ACL-default, forwarding, and general fields, plus forward `server_port`, `type=query|dot`, and `verify_cn`. Stable evidence is `GET/POST /api/unbound/settings/{get,set}`, reconfigure actions, and existing `*_forward` item CRUD.

## Acceptance Criteria

- **Given** existing Unbound settings, **when** they are imported/read, **then** all audited settings round-trip with no overlapping local resources managing the same field.
- **Given** query and DNS-over-TLS forwards, **when** CRUD/import/drift tests run, **then** port/type/CN verification are preserved and reconfigure applies the change.
- **Given** singleton deletion, **when** Terraform destroys it, **then** only state is removed and upstream settings are not reset.

## Verification

Unbound unit/schema tests, singleton and forward disposable-appliance acceptance, docs generation, and `make check`.
