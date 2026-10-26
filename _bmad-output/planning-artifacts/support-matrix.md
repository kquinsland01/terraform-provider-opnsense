---
title: Provider Support Matrix
date: 2026-09-28
author: BMad PM
status: in-review
inputs:
  - prd.md
  - core-config-gap-analysis.md
  - feature-complete-roadmap.md
  - provider-management-roadmap-2026-09-28.md
  - repository implementation inventory
---

# Provider Support Matrix

This matrix is the current source of truth for release positioning. It reconciles the expanded PRD/roadmap with the implementation present in the repository.

## Summary

| Area | Count | Status |
|---|---:|---|
| Resources | 111 | Implementation inventory includes unreleased gateway-group and generic-option owners |
| Data sources | 97 | Registered; Story 31.0e adds disposable-proved WireGuard settings alongside the existing Unbound settings view |
| Resource docs | 111 | Includes unreleased implementation |
| Data source docs | 97 | Generated |
| Ephemeral resources / docs | 1 / 1 | Terraform 1.10+ OpenVPN key generator passed final protocol/non-persistence proof; implementation in review |
| Remaining field/surface gaps | 5 audit-derived P0/P1 story groups (Stories 31.0f-31.0j), with separate research/upstream blockers | Coming, needs research, or upstream-blocked |
| Remaining resource-matching data-source gaps | 16 | Coming |

Data-source parity is tracked in [data-source-parity-plan.md](data-source-parity-plan.md). Remaining resource-gap verification is tracked in [resource-gap-verification.md](resource-gap-verification.md).

## Supported Resources

| Domain | Supported resources |
|---|---|
| ACME | account, certificate, challenge |
| Auth | group, user |
| Cron | job |
| DHCPv4 | reservation, subnet |
| Dnsmasq | boot, domain, host, option, range, settings, tag |
| Dynamic DNS | ddclient account, ddclient settings |
| Firewall | alias, category, filter rule, NAT one-to-one, NAT outbound, NAT port forward |
| HAProxy | ACL, action, backend, frontend, health check, map file, server |
| Interfaces | assignment/IP settings, bridge, GIF, GRE, LAGG, loopback, neighbor, VXLAN |
| IPsec | child, connection, key pair, local, manual SPD, pool, PSK, remote, VTI |
| Kea | control agent, DHCPv4 reservation/settings/subnet, DHCPv6 peer/PD pool/reservation/settings/subnet, HA peer |
| Monit | alert, service, test |
| OpenVPN | client overwrite, instance, static key |
| Quagga / FRR | BGP AS path, BGP community list, BGP global, BGP neighbor, BGP peer group, BGP redistribution, general, OSPF area/general/interface/neighbor/network/prefix list/redistribution/route map, OSPFv3 general/interface/network/prefix list/redistribution/route map, prefix list, RIP, route map, static general, static route |
| Syslog | destination |
| System | gateway, route, tunable, VIP, VLAN |
| Traffic shaper | pipe, queue, rule |
| Trust | CA, certificate |
| Unbound | ACL, DNSBL, domain override, general, host alias, host override |
| WireGuard | peer, server, settings |

## Supported Data Sources

| Domain | Supported data sources |
|---|---|
| Auth | group, user |
| Cron | job |
| ACME | account, certificate, challenge |
| DHCPv4 | reservation, subnet |
| Dnsmasq | boot, domain, host, option, range, tag |
| Dynamic DNS | ddclient account, ddclient settings |
| Firewall | alias, category, filter rule, NAT one-to-one, NAT outbound, NAT port forward |
| HAProxy | ACL, action, backend, frontend, health check, map file, server |
| Interfaces | bridge, GIF, GRE, LAGG, loopback, neighbor, VXLAN |
| IPsec | child, connection, local, manual SPD, pool, remote, VTI |
| Kea | DHCPv4 reservation/subnet, DHCPv6 peer/PD pool/reservation/subnet, HA peer |
| Monit | alert, service, test |
| Quagga / FRR | BGP AS path, BGP community list, BGP neighbor, BGP peer group, BGP redistribution, OSPF area/interface/neighbor/network/prefix list/redistribution/route map, OSPFv3 interface/network/prefix list/redistribution/route map, prefix list, route map, static route |
| Syslog | destination |
| OpenVPN | client overwrite, instance, static key |
| System | gateway, route, system info, tunable, VIP, VLAN |
| Traffic shaper | pipe, queue, rule |
| Trust | CA |
| Unbound | ACL, domain override, host alias, host override, settings |
| WireGuard | peer, server, settings |

## Coming: Buildable Provider Work

| Domain | Work |
|---|---|
| Data-source parity | Add read-only data sources for the 16 supported singleton/sensitive special-case resources that do not yet have same-name data-source counterparts. |
| Gateway groups | Stable 26.7.7 source and isolated direct-API CRUD/tier/trigger/pool/clear/rename/dependency-delete/route-restoration probes are recorded in `gateway-group-api-validation.md`. Story 31.0m adds a typed owner and UUID lookup: its repaired 16-step real Terraform lifecycle, five local Core recovery/private-replay cases, independent saved-field observations, exact restoration and follow-up reviews passed. This implementation is unreleased for 0.9.0; the final publication-tree proof is recorded in `31-0m-gateway-group-live-evidence.md`. Populated first-tier clearing is upstream-gated: empty updates are rejected before save, not advertised as full tier-clear completeness. No older-core, OpenTofu or actual multi-WAN failover claim. |
| Kea generic options | Story 31.0n implements the independent eight-field owner and UUID lookup on stable 26.7.7+, omission-preserving intent and durable single-write recovery. Full/race tests and a disabled-appliance 14-step Core lifecycle passed with 17 independent observations, tracked HTTP500 rejection, actual deletion and exact restoration; see the digest-bound evidence. Merge, hosted checks and publication remain separate gates. Generic parent linkage/order remains Story 31.0p; force is stored, not packet-proven. DDNS remains Story 31.0o. |
| FRR 2.0 compatibility | Story 31.0h compatibility subgate adds installed-plugin/model validation, family/state migration and grouped-policy preservation to existing owners. Exact non-skipped versions/digest are recorded in the FRR live evidence; legacy live, remaining advanced parity, release and adoption remain gated. Soft FRR reload is not PF activation or a daemon-set restart. |
| Documentation | Fill missing resource templates for generated docs, expand composition examples, and keep the provider index support matrix current. |
| Release hardening | Keep release workflow, Registry manifest, changelog, and provider docs verified for subsequent patch/minor releases. |
| Audited field/surface parity | Complete Stories 31.0f–31.0j for trust, IPsec, Quagga/BGP, live interface state, and provider retry controls before positioning/migration/adoption work. |
| Competitive positioning | Refresh README/Registry positioning and adoption examples only after the P0/P1 audit follow-ups complete. |

## Needs Research

| Domain | Research needed |
|---|---|
| Kea | Stable 26.7.7 source and disabled-service direct-API option/DDNS probes are recorded in `kea-option-ddns-api-validation.md`: targeted updates/preservation/false clear, reconfigure, required-field negatives and exact settings restoration. Story 31.0n implements generic-option ownership with a passing disabled-appliance Terraform lifecycle; merge and publication remain separately gated; Story 31.0o owns DDNS. Ordered parent linkage (31.0p), older targets and operator integrations remain separate; do not duplicate completed subnet/PXE/DHCPv6 parity. |
| System / HA | HASync configuration needs request/response shape research because current `Hasync.xml` uses dynamic `JsonKeyValueStoreField` `syncitems`; HASync status `services`/`version` are data-source candidates after live validation, while service operations are action candidates only after product/framework decision. |
| Interfaces | Base assignment/IP settings are supported by `opnsense_interface_assignment` on OPNsense 26.7.6+ after disposable 26.7.7 lifecycle/drift/restoration proof. PPPoE semantics remain separate research. |

Source NAT is supported as `opnsense_firewall_nat_outbound`. Story 31.0d Repair 7 implements and digest-bound proves Unbound query and DNS-over-TLS forwarding plus complete read-only settings; exactly four audited parity rows are reconciled and four-layer convergence is clean.

## Upstream-Blocked

Confirmed blockers and the maintenance workflow are documented publicly in [`docs/upstream-blocked.md`](../../docs/upstream-blocked.md).

| Resource/domain | Reason | Action |
|---|---|---|
| System general settings | No stable target-release durable settings API; `core/system` is action/status-only and `core/initial_setup` is wizard-only with broad side effects. | Watch OPNsense System Settings MVC roadmap, generated API docs, controllers/models, ACL/menu entries, and stable get/set semantics. |

## Release Readiness Snapshot

| Item | Status | Evidence / gap |
|---|---|---|
| GoReleaser config | Ready | `.goreleaser.yml` cross-compiles, archives ZIPs, emits checksums, signs checksum artifact. |
| Release workflow | Ready with secrets | `.github/workflows/release.yml` runs on `v*` tags; requires `GPG_PRIVATE_KEY` and `PASSPHRASE` secrets. |
| Terraform Registry manifest | Ready | `terraform-registry-manifest.json` declares protocol `6.0`. |
| Changelog | Published baseline | `CHANGELOG.md` lists 90 resources and 34 data sources for `0.1.0`; current post-release work has added more data sources. |
| Provider index | Mostly ready | Covers auth, minimum version, permissions, and quickstart; support matrix now added to planning artifacts and should stay reflected in Registry docs. |
| Migration/import guidance | Partial | Per-resource import examples exist; broader dependency-order migration guide remains useful follow-up. |

Story 31.0e proves exactly six WireGuard audit rows on disposable OPNsense 26.7.7: typed peer/server field parity and import-first daemon settings. WireGuard is core on 24.1+; server `peers` owns associations, peer `servers` is its authoritative API backreference. PSK/private keys stay resource-only Sensitive values. The daemon remained disabled, Terraform deletes left zero tagged items, and settings restored exactly. Implementation is in review, not yet merged/released; see [digest-bound evidence](../implementation-artifacts/31-0e-wireguard-live-evidence.md). Stories 31.0k/31.0l remain separate stable-API validation work; the FRR2 compatibility gate is unchanged.
