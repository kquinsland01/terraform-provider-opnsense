# Story 31.2: Existing Provider Migration Guide

## Status

backlog

## Story

As a user migrating from another OPNsense Terraform provider,
I want a clear migration guide based on the coverage audit,
so that I can move to this provider as the single standard for an appliance instead of running both providers against overlapping OPNsense objects.

## Acceptance Criteria

1. A public guide maps audited resources and data sources from established OPNsense Terraform providers to this provider's equivalents or documented gaps.
2. Guide includes known naming differences, such as `interfaces_vlan` vs `system_vlan`, `route` vs `system_route`, and firewall rule naming.
3. Guide identifies non-equivalent areas and advises whether to keep existing state, import, or redesign.
4. Import examples show brownfield migration patterns and no-change plan verification.
5. Guide links from README, Registry docs, and `docs/migration-import.md`.
6. Guide clearly says the recommended steady state is one provider per appliance/object domain; it does not recommend running both providers against the same objects.

## Dev Notes

- This is a product/adoption story backed by Story 31.0, not a substitute for coverage work.
- Include evidence from the coverage audit when making coverage claims.
- Depends on completion or explicit audit reclassification of P0/P1 Stories 31.0a–31.0j.

## Validation

- Markdown links resolve locally.
- `git diff --check` passes.
- Docs generation runs if Registry templates change.
