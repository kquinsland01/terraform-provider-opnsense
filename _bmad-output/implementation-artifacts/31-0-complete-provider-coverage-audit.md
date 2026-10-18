# Story 31.0: Complete Provider Coverage Audit

## Status

done

## Story

As provider maintainers,
I want a field-level coverage audit against the established OPNsense Terraform provider ecosystem,
so that `matthew-on-git/opnsense` can become the standard complete provider without requiring users to run multiple providers on the same appliance.

## Acceptance Criteria

1. A maintained coverage matrix compares every published resource and data source from the leading established OPNsense provider baseline against this provider's equivalent, including same-name resources and differently named equivalents.
2. The matrix classifies each row as `covered`, `covered-different-name`, `covered-broader`, `missing-buildable`, `missing-upstream-blocked`, `intentionally-different`, or `needs-field-audit`.
3. Field-depth coverage is checked for shared domains instead of relying on resource counts alone, especially firewall rules/NAT, Unbound, Kea, WireGuard, OpenVPN, IPsec, trust, cron, Quagga/BGP, routes, VLANs, VIPs, and live interface data.
4. Known ecosystem-only candidates are explicitly researched and classified:
   - live interface state data sources: `interface`, `interface_all`, `interfaces_overview`, `interfaces_overview_all`
   - Unbound singleton/settings and DoT/query forwarding: `unbound_settings`, `unbound_forward`
   - Kea peers and DHCPv6 prefix delegation pools: `kea_dhcpv4_peer`, `kea_dhcpv6_peer`, `kea_dhcpv6_pd_pool`
   - singleton settings resources such as `wireguard_settings` and `trust_settings`
   - OpenVPN static-key generation behavior
   - any provider-level retry/configuration knobs that materially affect user experience
5. Every true `missing-buildable` gap becomes a follow-up implementation story with evidence, endpoint notes, acceptance criteria, and priority.
6. Every `missing-upstream-blocked` gap links to `docs/upstream-blocked.md` or adds the missing blocker entry.
7. The audit states that running multiple providers against the same objects is not a supported strategy; the product direction is to make this provider complete enough to be the single provider for a given appliance.

## Dev Notes

- This story is the product completeness gate. Do not let migration documentation substitute for missing provider coverage.
- Resource/data-source counts are only a starting point. Use Registry docs, source schemas, generated docs, and OPNsense endpoint evidence.
- Treat the established provider ecosystem as a coverage baseline, not as a public comparison target. Public docs should emphasize complete OPNsense appliance coverage and clear support status.
- Migration support should flow from this audit: once equivalent coverage is known, Story 31.2 can map state/import paths honestly.

## Validation

- Coverage matrix is committed under `_bmad-output/planning-artifacts/` or another BMad-owned planning path.
- Follow-up stories are added to `_bmad-output/implementation-artifacts/` and `sprint-status.yaml`.
- `git diff --check` passes.
- `make check` passes if docs, generated docs, templates, or source files change.

## Completion Evidence

- Pinned `browningluke/opnsense` v0.26.0 at commit `33c2a03c2bc1a7339cb5aa5b611e15014e27f7ba`; schema counts are 47 resources, 45 data sources, one ephemeral resource, plus provider retry configuration.
- Added the canonical machine-readable manifest and deterministic Markdown report under `_bmad-output/planning-artifacts/`.
- Added `internal/coverageaudit` validation for baseline completeness, classification/follow-up rules, registered local names, 106/90 generated docs, 17 resource-only gaps, `system_info` data-only status, and render drift.
- Classified deprecated generic Kea v4 aliases as intentionally different rather than duplicate work.
- Created P0/P1 Stories 31.0a–31.0j for every confirmed buildable field or surface family and ordered them before Stories 31.1–31.3.
- Confirmed no baseline row is upstream-blocked; gateway groups and system general settings remain in the separate public blocker register because they are not published baseline surfaces.
- Reconciled support/planning/downstream story facts and retained the one-provider-per-appliance/object-domain direction.

## Verification Results

- `go test ./internal/coverageaudit`
- `go generate ./... && git diff --exit-code` (run after generation artifacts are committed/staged in CI; local generated output was checked deterministically)
- `bash test/scripts/validate-structure.sh`
- `DEVRAIL_IMAGE=ghcr.io/devrail-dev/dev-toolchain:1.12.24 make check`
- `git diff --check`
