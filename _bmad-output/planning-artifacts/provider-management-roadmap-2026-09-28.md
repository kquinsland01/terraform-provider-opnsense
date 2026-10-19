---
title: Provider Management Roadmap - Complete Provider Coverage and OPNsense Assignment API
date: 2026-09-28
last_updated: 2026-10-19
author: BMad PM
status: in-review
inputs:
  - prd.md
  - post-release-epics.md
  - support-matrix.md
  - docs/upstream-blocked.md
  - Provider ecosystem coverage comparison, 2026-09-28
  - OrgDocs h155 OPNsense provider/API gap note
---

# Provider Management Roadmap

## PM Decision

We should manage `matthew-on-git/opnsense` as the broad, modern, brownfield-friendly OPNsense provider and make it complete enough to be the default provider choice for new OPNsense automation.

Why this matters: this provider now has broad OPNsense surface coverage, but breadth only becomes trustworthy when it is backed by clear support status, release-gated API adoption, acceptance-test proof, and a coverage matrix that proves users can standardize on one provider rather than run multiple providers side-by-side. Migration content matters, but it is secondary to completeness.

## Current Facts

| Signal | `matthew-on-git/opnsense` | Established provider baseline |
|---|---:|---:|
| Current public release checked | `v0.6.0` | `v0.26.0` |
| Registry downloads checked | about 8.1K | about 1.33M |
| Resources / data sources | 108 / 94 | pinned baseline 47 / 45 plus one registered ephemeral resource; Story 31.0d Repair 7 adds and candidate-proves the complete Unbound settings data source |
| Primary advantage | breadth, plugin/service coverage, generated docs | adoption, community recognition, polished positioning |
| Main product gap | audited field/surface parity, proof, and positioning | coverage breadth |

Coverage advantage already exists in HAProxy, ACME, Monit, syslog, traffic shaping, FRR/OSPF, auth/trust, interface types, and many service/plugin domains. The weak spots are not raw resource count; they are provable field-level coverage, release narrative, acceptance proof, v1 criteria, and clear migration guidance.

## OPNsense Assignment API Release Tracking

The September upstream PR detour changed the plan. We closed the duplicate upstream `address_settings` PR because current OPNsense `master` already has the relevant work under the existing assignment API:

- `api/interfaces/assignment/set_item/{ifname}`
- `api/interfaces/assignment/reconfigure`
- expanded `NetworkInterface.xml` fields including `enable`, `type4`, `ipaddr`, `type6`, `ipaddrv6`, gateways, DHCP fields, and hardware flags
- `/tmp/.interfaces.todo` staging and assignment `reconfigure` finalization

Release timing conclusion from the automated 2026-10-17 check:

- OPNsense backported the expanded assignment model/controller to `stable/26.7`.
- `26.7.6` is the first released tag with the complete probed address, gateway, DHCP, `set_item`, and `reconfigure` surface.
- `26.7.5` and `27.1.a` do not have the complete required field set.
- Story 30.1 now commits a machine-readable baseline and checks it weekly for upstream changes.

Product rule: `opnsense_interface_assignment` supports base assignment/IP settings on OPNsense `26.7.6` or newer after Story 30.2 passed lifecycle, drift, repair, no-change-plan, and restoration testing on disposable OPNsense 26.7.7. PPPoE remains outside that implementation pending separate semantic validation.

## Public Positioning Strategy

### Positioning

Use this message in README, Registry docs, and release notes:

> `matthew-on-git/opnsense` focuses on broad, modern OPNsense appliance management: core networking plus the plugin/service areas operators actually use at the edge, with brownfield import guidance and release-gated API support.

Avoid public comparisons that invite maintainer drama. Claim "broad modern coverage" or "complete appliance coverage goal" only where the support matrix backs it.

### Product Principles

1. Version-gate new OPNsense APIs instead of pretending `master` equals stable.
2. Treat acceptance proof as product surface, not private maintainer confidence.
3. Treat established ecosystem coverage as a product gate, including field-level schema depth where providers expose similar concepts.
4. Make migration from existing providers explicit and low-friction after the coverage position is understood.
5. Publish v1 criteria before declaring v1 readiness.
6. Do not ship workaround resources that scrape legacy PHP pages, edit XML directly, or bypass OPNsense API ownership.

## Backlog Epics

## Epic 30: OPNsense Assignment API Adoption

Goal: turn the upstream assignment API work into a safe provider feature as soon as it lands in a supported OPNsense release.

| Story | Priority | Status | File |
|---|---|---|---|
| 30.1 Track OPNsense Assignment API Release | P0 | done | `_bmad-output/implementation-artifacts/30-1-track-opnsense-assignment-api-release.md` |
| 30.2 Implement Interface Assignment/IP Resource for OPNsense 26.7.6+ | P0 | done | `_bmad-output/implementation-artifacts/30-2-prototype-interface-assignment-ip-resource.md` |
| 30.3 Update Interface Support Matrix and Public Blocker Docs | P0 | done | `_bmad-output/implementation-artifacts/30-3-update-interface-support-matrix.md` |

## Epic 31: Provider Completeness, Positioning, and Migration

Goal: make this provider the standard OPNsense provider by closing meaningful coverage gaps, then turning that completeness into user trust and adoption.

| Story | Priority | Status | File |
|---|---|---|---|
| 31.0 Complete Provider Coverage Audit | P0 | done | `_bmad-output/implementation-artifacts/31-0-complete-provider-coverage-audit.md` |
| 31.0a Firewall and NAT Field Parity | P0 | done | `_bmad-output/implementation-artifacts/31-0a-firewall-nat-field-parity.md` |
| 31.0b Kea Field and Surface Parity | P0 | done | `_bmad-output/implementation-artifacts/31-0b-kea-field-and-surface-parity.md` |
| 31.0c OpenVPN Field and Key Parity | P0 | done (repair 2 proved) | `_bmad-output/implementation-artifacts/spec-31-0c-openvpn-field-and-key-parity.md` |
| 31.0d Unbound Settings and Forward Parity | P0 | review | `_bmad-output/implementation-artifacts/31-0d-unbound-settings-and-forward-parity.md` |
| 31.0e WireGuard Field and Settings Parity | P0 | backlog | `_bmad-output/implementation-artifacts/31-0e-wireguard-field-and-settings-parity.md` |
| 31.0f Trust Lifecycle and Settings Parity | P0 | backlog | `_bmad-output/implementation-artifacts/31-0f-trust-lifecycle-and-settings-parity.md` |
| 31.0g IPsec Field Parity | P1 | backlog | `_bmad-output/implementation-artifacts/31-0g-ipsec-field-parity.md` |
| 31.0h Quagga/BGP Field Parity | P1 | backlog | `_bmad-output/implementation-artifacts/31-0h-quagga-bgp-field-parity.md` |
| 31.0i Live Interface Data Sources | P1 | backlog | `_bmad-output/implementation-artifacts/31-0i-live-interface-data-sources.md` |
| 31.0j Provider Retry Settings | P1 | backlog | `_bmad-output/implementation-artifacts/31-0j-provider-retry-settings.md` |
| 31.1 Complete Provider Positioning README and Registry Pass | P1 | backlog | `_bmad-output/implementation-artifacts/31-1-complete-provider-positioning-docs.md` |
| 31.2 Existing Provider Migration Guide | P1 | backlog | `_bmad-output/implementation-artifacts/31-2-existing-provider-migration-guide.md` |
| 31.3 Example Stacks for Adoption | P2 | backlog | `_bmad-output/implementation-artifacts/31-3-example-stacks-for-adoption.md` |

## Epic 32: Release Credibility and v1 Readiness

Goal: make releases feel boring, tested, and intentionally versioned.

| Story | Priority | Status | File |
|---|---|---|---|
| 32.1 Acceptance Test Proof Artifact | P1 | backlog | `_bmad-output/implementation-artifacts/32-1-acceptance-test-proof-artifact.md` |
| 32.2 Provider Preflight and Version Diagnostics | P2 | backlog | `_bmad-output/implementation-artifacts/32-2-provider-preflight-version-diagnostics.md` |
| 32.3 v1 Stability Roadmap | P2 | backlog | `_bmad-output/implementation-artifacts/32-3-v1-stability-roadmap.md` |

## Epic 33: Remaining Ecosystem Gap Mining

Goal: mine broader ecosystem-only areas without distracting from release tracking and the primary complete-provider coverage objective.

Candidates:

- captive portal templates/zones
- firewall groups
- NPTv6
- firewall automation filter/source NAT variants

This should stay P1/P2 until Epic 30 has a working release gate and Epic 31 has fixed the public positioning gap.

## Recommended Sequence

1. Keep completed Epic 30's release probe current; `opnsense_interface_assignment` requires OPNsense 26.7.6+ and has disposable 26.7.7 lifecycle/drift/restoration proof.
2. Complete audit-derived P0 Stories 31.0a–31.0f, then P1 Stories 31.0g–31.0j; re-run the deterministic audit after each registration/schema change.
3. Story 31.1: make README/Registry positioning match verified post-gap coverage.
4. Story 31.2: publish the existing-provider migration guide.
5. Story 32.1: publish repeatable acceptance-test proof.
6. Story 32.3: define v1 criteria before v1 work is declared.
