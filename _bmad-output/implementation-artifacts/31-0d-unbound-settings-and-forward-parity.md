# Story 31.0d: Unbound Settings and Forward Parity

## Status

done

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

## Implementation Evidence

Repair 7 passes exact-tree and promoted-digest OPNsense 26.7.7 proof, including handshake-driven serialization tests, strict null-metadata coverage, state-only singleton destroy, exact restore, and zero leftovers. Exactly four Story 31.0d audit rows are reconciled, and four-layer convergence is clean. The story is ready for review.

### Merge reconciliation — 2026-10-20

PR #61 merged as `f888efac1b129196847d3f836b6e1c621a4b1c1a` on 2026-10-19 after all eight checks passed. This status-only reconciliation does not rerun or replace the recorded Repair 7 lifecycle proof.
