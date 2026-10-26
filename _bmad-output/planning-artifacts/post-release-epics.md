---
title: Post-v0.1.0 Epic Plan
date: 2026-09-28
author: BMad PM
status: current
inputs:
  - prd.md
  - feature-complete-roadmap.md
  - core-config-gap-analysis.md
  - support-matrix.md
  - provider-management-roadmap-2026-09-28.md
---

# Post-v0.1.0 Epic Plan

The first provider version is published. Completed post-release work closed the original data-source parity, Registry docs, verified gap, and HAProxy/ACME migration tracks. The next management focus is release-gated OPNsense assignment API adoption, complete coverage positioning, and v1 credibility.

## Epic 25B: Data Source Parity

Goal: close straightforward data-source parity gaps and leave only singleton or sensitive special-case follow-up work. Epic 25B completed the historical move from 34 to 76 supported data sources; subsequent work brings the current inventory to 92 data sources and 17 resource-matching gaps.

| Story | Status | File |
|---|---|---|
| 25B.1 Data Source Parity Inventory and Implementation Batches | done | `_bmad-output/implementation-artifacts/25b-1-data-source-parity-inventory-and-batches.md` |
| 25B.2 High-Reference Data Sources | done | `_bmad-output/implementation-artifacts/25b-2-high-reference-data-sources.md` |
| 25B.3 Routing, DNS, DHCP, ACME, and Trust Data Sources | done | `_bmad-output/implementation-artifacts/25b-3-routing-dns-dhcp-acme-trust-data-sources.md` |

## Epic 26: Registry Docs Hardening

Goal: improve the published Registry experience now that users can install the provider.

| Story | Status | File |
|---|---|---|
| 26.1 Published Registry Documentation Audit | done | `_bmad-output/implementation-artifacts/26-1-registry-docs-audit.md` |
| 26.2 Public Support Matrix in Registry Docs | done | `_bmad-output/implementation-artifacts/26-2-public-support-matrix-registry-docs.md` |
| 26.3 Full-Appliance Migration and Import Guide | done | `_bmad-output/implementation-artifacts/26-3-full-appliance-migration-guide.md` |

## Epic 27: Remaining Buildable Resource Gaps

Goal: verify and implement remaining provider-owned resource gaps without speculating on unconfirmed endpoints.

| Story | Status | File |
|---|---|---|
| 27.1 Verify Remaining Buildable Resource Gaps | done | `_bmad-output/implementation-artifacts/27-1-verify-buildable-resource-gaps.md` |
| 27.2 Implement Highest-Value Verified Resource Gap | done | `_bmad-output/implementation-artifacts/27-2-implement-highest-value-verified-gap.md` |
| 27.3 Implement OSPF Area Resource | done | `_bmad-output/implementation-artifacts/27-3-implement-ospf-area-resource.md` |
| 27.4 HASync Configuration Model Review (no resource shipped) | done | `_bmad-output/implementation-artifacts/27-4-implement-hasync-configuration-resource.md` |

## Epic 28: Upstream-Blocked Tracking

Goal: keep OPNsense-side blockers visible, honest, and periodically reviewed.

| Story | Status | File |
|---|---|---|
| 28.1 Upstream-Blocked Register and Maintenance Workflow | done | `_bmad-output/implementation-artifacts/28-1-upstream-blocked-register-and-maintenance.md` |
| 28.2 Research HASync Status and Actions | done | `_bmad-output/implementation-artifacts/28-2-research-hasync-status-actions.md` |
| 28.3 Research System Tunables and Sysctl API Lifecycle | done | `_bmad-output/implementation-artifacts/28-3-research-system-tunables-sysctl.md` |
| 28.4 Implement System Tunables Resource | done | `_bmad-output/implementation-artifacts/28-4-implement-system-tunables-resource.md` |

## Epic 29: Appliance Migration Blockers — HAProxy Routing, TLS & ACME Issuance

Goal: close the specific gaps that block migrating a real multi-domain OPNsense edge off Ansible onto this provider. Driven by the downstream `consumer automation` appliance, whose `https-in` frontend routes many domains (incl. CNAME aliases) via a domain map + actions, applies internal-only deny rules and an HTTP→HTTPS redirect, and binds an ACME-issued certificate. Verified missing/incomplete against **v0.2.0** source.

| Story | Status | File |
|---|---|---|
| 29.1 HAProxy Action Resource | done | `_bmad-output/implementation-artifacts/29-1-haproxy-action-resource.md` |
| 29.2 HAProxy Map File Resource | done | `_bmad-output/implementation-artifacts/29-2-haproxy-mapfile-resource.md` |
| 29.3 HAProxy Frontend TLS Certificate Binding | done | `_bmad-output/implementation-artifacts/29-3-haproxy-frontend-tls-certificate-binding.md` |
| 29.4 ACME Certificate Issuance & Refid Output | done | `_bmad-output/implementation-artifacts/29-4-acme-certificate-issuance-and-refid.md` |
| 29.5 ddclient Daemon Settings Resource | done | `_bmad-output/implementation-artifacts/29-5-ddclient-daemon-settings-resource.md` |
| 29.6 Multi-Domain Edge Composition & Migration Validation | done | `_bmad-output/implementation-artifacts/29-6-multi-domain-edge-composition-and-migration-validation.md` |

**Notes / corrections to prior tracking:**
- 29.1 / 29.2 / 29.3 were deferred in Story 4.2 ("What NOT to Build") to a Story 4.3 that only shipped ACLs — the action, map file, and frontend cert-binding were never created.
- 29.4 corrects Story 8.2: despite being titled "...with Issuance" and marked done, `certificate_resource.go` is plain CRUD (no `/sign`, no status poll, no refid output).
- 29.5 supersedes the wrongly-cancelled Story 9.5 — daemon settings (`enabled`/`daemon_delay`/`backend`/...) are a distinct `/dyndns/settings` object, not the account-level `service` field.
- **Ownership gate (not an absent-API blocker):** Completed Story 31.0b implements subnet PXE/TFTP fields; Stories 11.3 / 21.4 must reconcile that coverage rather than repeat it. Generic option/DDNS lifecycle remains a separate Story 31.0l validation gate; do not transfer production DHCP ownership without a reviewed site-specific cutover.

**Recommended build order:** 29.1 → 29.2 → 29.4 → 29.3 → 29.5 (independent) → 29.6 (capstone). 29.3 wants 29.4's `cert_ref_id`; 29.6 depends on 29.1–29.4.

## Epic 30: OPNsense Assignment API Adoption

Goal: safely adopt the expanded upstream `api/interfaces/assignment/*` API after it lands in a stable OPNsense release. The September 2026 upstream PR detour proved the separate `address_settings` endpoint is not the right direction; provider work should consume the upstream assignment API instead.

| Story | Status | File |
|---|---|---|
| 30.1 Track OPNsense Assignment API Release | done | `_bmad-output/implementation-artifacts/30-1-track-opnsense-assignment-api-release.md` |
| 30.2 Implement Interface Assignment/IP Resource for OPNsense 26.7.6+ | done | `_bmad-output/implementation-artifacts/30-2-prototype-interface-assignment-ip-resource.md` |
| 30.3 Update Interface Support Matrix and Public Blocker Docs | done | `_bmad-output/implementation-artifacts/30-3-update-interface-support-matrix.md` |

## Epic 31: Provider Completeness, Positioning, and Migration

Goal: make this provider complete enough to be the default provider choice for a given appliance, then convert that completeness into user trust and adoption.

| Story | Status | File |
|---|---|---|
| 31.0 Complete Provider Coverage Audit | done | `_bmad-output/implementation-artifacts/31-0-complete-provider-coverage-audit.md` |
| 31.0a Firewall and NAT Field Parity | done | `_bmad-output/implementation-artifacts/31-0a-firewall-nat-field-parity.md` |
| 31.0b Kea Field and Surface Parity | done | `_bmad-output/implementation-artifacts/31-0b-kea-field-and-surface-parity.md` |
| 31.0c OpenVPN Field and Key Parity | done (repair 2 proved) | `_bmad-output/implementation-artifacts/31-0c-openvpn-field-and-key-parity.md` |
| 31.0d Unbound Settings and Forward Parity | done | `_bmad-output/implementation-artifacts/31-0d-unbound-settings-and-forward-parity.md` |
| 31.0e WireGuard Field and Settings Parity | review | `_bmad-output/implementation-artifacts/31-0e-wireguard-field-and-settings-parity.md` |
| 31.0f Trust Lifecycle and Settings Parity | backlog | `_bmad-output/implementation-artifacts/31-0f-trust-lifecycle-and-settings-parity.md` |
| 31.0g IPsec Field Parity | backlog | `_bmad-output/implementation-artifacts/31-0g-ipsec-field-parity.md` |
| 31.0h Quagga/BGP Field Parity | backlog | `_bmad-output/implementation-artifacts/31-0h-quagga-bgp-field-parity.md` |
| 31.0i Live Interface Data Sources | backlog | `_bmad-output/implementation-artifacts/31-0i-live-interface-data-sources.md` |
| 31.0j Provider Retry Settings | backlog | `_bmad-output/implementation-artifacts/31-0j-provider-retry-settings.md` |
| 31.0k Gateway Group Stable API Validation | review (direct-API report recorded) | `_bmad-output/implementation-artifacts/31-0k-gateway-group-stable-api-validation.md` |
| 31.0l Kea Option and DDNS Stable API Validation | review (direct-API report recorded) | `_bmad-output/implementation-artifacts/31-0l-kea-option-ddns-stable-api-validation.md` |
| 31.0m Gateway Group Resource/Data Source | review / live-validated, unreleased | `_bmad-output/implementation-artifacts/31-0m-gateway-group-resource.md` |
| 31.0n Kea Generic Option Resource/Data Source | review / live-validated locally, unreleased | `_bmad-output/implementation-artifacts/31-0n-kea-generic-option-resource.md` |
| 31.0o Kea DDNS Settings Resource/Data Source | backlog | `_bmad-output/implementation-artifacts/31-0o-kea-ddns-settings-resource.md` |
| 31.0p Kea Ordered Generic-Option Linkage | backlog / ordered-read API gate | `_bmad-output/implementation-artifacts/31-0p-kea-ordered-option-linkage.md` |
| 31.1 Complete Provider Positioning README and Registry Pass | backlog | `_bmad-output/implementation-artifacts/31-1-complete-provider-positioning-docs.md` |
| 31.2 Existing Provider Migration Guide | backlog | `_bmad-output/implementation-artifacts/31-2-existing-provider-migration-guide.md` |
| 31.3 Example Stacks for Adoption | backlog | `_bmad-output/implementation-artifacts/31-3-example-stacks-for-adoption.md` |

## Epic 32: Release Credibility and v1 Readiness

Goal: make releases feel intentionally tested, versioned, and ready for production adoption.

| Story | Status | File |
|---|---|---|
| 32.1 Acceptance Test Proof Artifact | backlog | `_bmad-output/implementation-artifacts/32-1-acceptance-test-proof-artifact.md` |
| 32.2 Provider Preflight and Version Diagnostics | backlog | `_bmad-output/implementation-artifacts/32-2-provider-preflight-version-diagnostics.md` |
| 32.3 v1 Stability Roadmap | backlog | `_bmad-output/implementation-artifacts/32-3-v1-stability-roadmap.md` |

## Recommended Sequence

1. Treat Epics 25B through 29 as completed or historical unless a regression appears.
2. Keep completed Epic 30's weekly probe baseline current; assignment/IP support requires OPNsense 26.7.6+ and has disposable-appliance lifecycle proof.
3. Finish active P0 WireGuard Story 31.0e first; prioritize Story 31.0h's FRR 2.0 compatibility subgate before BGP adoption. Complete remaining parity (31.0f–31.0j), gateway-group validation (31.0k) and generic Kea option/DDNS validation (31.0l) before positioning. Stable API presence, provider implementation, published release and live lifecycle proof are distinct gates. No production firmware, routing or ownership cutover is authorized by these provider stories.
4. Follow with Stories 31.1 and 31.2 to make the Registry/README story match verified coverage.
5. Add Story 32.1 acceptance proof before treating v1 positioning as credible.
