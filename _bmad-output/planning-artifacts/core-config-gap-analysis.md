---
title: OPNsense Core Config — Terraform Provider Gap Analysis
date: 2026-06-02
author: BMad PM (reconciliation)
status: current
purpose: Define the feature-complete finish line and classify every core-config domain as Supported, Coming, Needs research, Upstream-blocked, or Not planned.
---

# Core Config Gap Analysis

## Goal

A feature-complete Terraform provider covers all OPNsense core configuration that has a stable usable API, and clearly labels anything that is not yet buildable because OPNsense itself lacks the required endpoint.

## Status Legend

- **Supported** — resource/data source exists in the provider and has generated docs.
- **Coming** — OPNsense API appears buildable; provider work remains.
- **Needs research** — published or suspected endpoint evidence exists, but target-version availability, wrapper shape, durable Terraform semantics, or product priority are not yet clear.
- **Upstream-blocked** — no usable API in the target OPNsense release; requires OPNsense work first.
- **Not planned** — redundant or intentionally out of scope for the current product definition.

## Current Scorecard

| Area | Count | Status |
|---|---:|---|
| Resources | 108 | Supported |
| Data sources | 94 | Supported |
| Resource docs | 108 | Supported |
| Data source docs | 94 | Supported |
| Resource-matching data-source backlog | 16 | Coming |
| Verified provider-owned resource candidates | 0 plus research follow-ups | Needs research |

## Domain-by-Domain Status

### Interfaces

| Item | Status | Notes |
|---|---|---|
| VLAN | Supported | `opnsense_system_vlan` |
| Virtual IP | Supported | `opnsense_system_vip` |
| Bridge | Supported | Resource + data source |
| GRE | Supported | Resource + data source |
| GIF | Supported | Resource + data source |
| VXLAN | Supported | Resource + data source |
| Loopback | Supported | Resource + data source |
| Neighbor / static ARP/NDP | Supported | Resource + data source |
| LAGG | Supported | Resource + data source; live-validated against Vagrant with dedicated `em4`/`em5` member interfaces. |
| Base assignment / IP config | Supported | `opnsense_interface_assignment` shipped after disposable OPNsense 26.7.7 lifecycle, drift, repair, and restoration proof; requires OPNsense 26.7.6+. |
| PPPoE | Needs research | The assignment implementation deliberately excluded PPPoE until model, secret, and lifecycle semantics are proven. |

### Firewall & NAT

| Item | Status | Notes |
|---|---|---|
| Alias | Supported | Resource + data source |
| Category | Supported | Resource only |
| Filter rule | Supported | Resource only; savepoint/apply/cancelRollback path implemented. |
| NAT port forward | Supported | Resource only |
| NAT outbound | Supported | Resource only |
| NAT one-to-one | Supported | Resource + data source |
| Source NAT | Supported | Already shipped as `opnsense_firewall_nat_outbound` using `/api/firewall/source_nat/*_rule`. |
| Schedules | Coming | Verify API and product priority before adding. |

### Routing & Gateways

| Item | Status | Notes |
|---|---|---|
| Static route | Supported | Resource only |
| Gateway | Supported | Resource only |
| Gateway group | Coming | Stable 26.7.7 `routing/group_settings` CRUD/search/reconfigure is source-confirmed. Story 31.0k must prove tier/default/wrapper semantics, in-use dependency refusal and convergence before implementation/support. No claim for older core versions. |

### Dynamic Routing (FRR / Quagga)

| Item | Status | Notes |
|---|---|---|
| FRR general | Supported | Singleton resource |
| BGP global config | Supported | Singleton resource |
| BGP neighbor | Supported | Resource |
| BGP prefix list | Supported | Resource |
| BGP route map | Supported | Resource |
| BGP AS path | Supported | Resource |
| BGP community list | Supported | Resource |
| BGP peer group | Supported | Resource |
| BGP redistribution | Supported | Resource |
| OSPF general/area/interface/neighbor/network/prefix list/redistribution/route map | Supported | Resources; sub-resource data sources exist. |
| OSPFv3 general/interface/network/prefix list/redistribution/route map | Supported | Resources; sub-resource data sources exist. |
| RIP | Supported | Singleton resource |
| FRR static general + route | Supported | Resources |

FRR support above is not a blanket os-frr 2.0 compatibility claim. Story 31.0h now gates address-family migration, grouped policy references and automatic protocol rules against separately identified core/plugin versions before adoption.

### VPN

| Item | Status | Notes |
|---|---|---|
| WireGuard server | Supported | Resource only |
| WireGuard peer | Supported | Resource only |
| OpenVPN instance | Supported | Resource only |
| OpenVPN client overwrite | Supported | Resource only |
| OpenVPN static key | Supported | Resource only |
| IPsec connection | Supported | Resource only |
| IPsec child | Supported | Resource only |
| IPsec PSK | Supported | Resource only |
| IPsec local | Supported | Resource only |
| IPsec remote | Supported | Resource only |
| IPsec pool | Supported | Resource + data source |
| IPsec VTI | Supported | Resource + data source |
| IPsec manual SPD | Supported | Resource + data source |
| IPsec key pair | Supported | Resource only |

### DHCP and Dynamic DNS

| Item | Status | Notes |
|---|---|---|
| DHCPv4 subnet | Supported | Resource only |
| DHCPv4 reservation | Supported | Resource only |
| Kea control agent | Supported | Singleton resource |
| Kea DHCPv6 settings | Supported | Resource |
| Kea DHCPv6 subnet | Supported | Resource |
| Kea DHCPv6 reservation | Supported | Resource |
| Kea HA peer | Supported | Resource + data source |
| DHCPv4 option | Needs research | Stable 26.7.7 `kea/dhcpv4` option CRUD/search exists. Story 31.0l validates generic options separately from completed 31.0b subnet PXE/TFTP fields; no absent-stable-API blocker remains. |
| Kea DDNS | Needs research | Stable 26.7.7 `Kea/Api/DdnsController.php` exists. Story 31.0l must prove inherited set semantics, wrapper/default handling, reconfigure and non-destructive singleton lifecycle before implementation. |
| Dynamic DNS account | Supported | Resource only |
| DDNS provider | Not planned | Redundant with provider/service field on account unless a separate endpoint emerges. |

### DNS

| Item | Status | Notes |
|---|---|---|
| Unbound host override | Supported | Resource only |
| Unbound host alias | Supported | Resource only |
| Unbound domain override | Supported | Resource only |
| Unbound ACL | Supported | Resource only |
| Unbound general | Supported | Singleton resource |
| Unbound DNSBL/blocklist | Supported | Resource |
| Unbound forward | Covered | Story 31.0d Repair 7 implements and candidate-proves strict, endpoint-correct independent query and DNS-over-TLS lifecycles including non-mutating post-destroy zero-object checks. |
| Dnsmasq settings | Supported | Resource only |
| Dnsmasq host/domain/range/option/tag/boot | Supported | UUID item resources and data sources using `dnsmasq/settings` item endpoints plus `dnsmasq/service/reconfigure`. |

### System, Access, and Trust

| Item | Status | Notes |
|---|---|---|
| System info | Supported | Data source |
| Users | Supported | Resource + data source |
| Groups | Supported | Resource + data source |
| Certificate authority | Supported | Resource only |
| Certificate | Supported | Resource only |
| Cron job | Supported | Resource + data source |
| System general settings | Upstream-blocked | Story 5.7 created on 2026-06-14: published core API docs list no durable system general settings endpoint; `core/system` is action/status-only; no `Core/Api/SettingsController.php`, `Core/System.xml`, or `Core/Settings.xml` was found; `core/initial_setup` is wizard-only and unsafe for day-2 Terraform management. |
| Tunables / sysctl | Coming with safety/live-validation gate | Story 28.3 confirmed persistent `core/tunables` item CRUD/search and `reconfigure`; implement only after live validation and safety documentation for kernel/network tunables. |
| High availability / HASync config | Needs research | Source recheck on 2026-06-18 confirms singleton `core/hasync` get/set/reconfigure exists on `stable/25.7`, but `Hasync.xml` uses dynamic `JsonKeyValueStoreField` `syncitems`; verify live API shape and safe Terraform representation before implementation. |
| High availability / HASync status/actions | Needs research | Story 28.2 classified `services`/`version` as data-source candidates after live response validation and service operations as action candidates only after product/framework decision; no durable resource semantics. |

### Services and Shaping

| Item | Status | Notes |
|---|---|---|
| HAProxy server/backend/frontend/ACL/health check | Supported | Resources + data sources |
| ACME account/certificate/challenge | Supported | Resources + data sources |
| Traffic shaper pipe/queue/rule | Supported | Resources + data sources |
| Monit service/test/alert | Supported | Resources + data sources |
| Syslog destination | Supported | Resource + data source |

## Cross-Cutting Gaps

| Item | Status | Notes |
|---|---|---|
| Data-source parity | Coming | 17 singleton, assignment, or sensitive special-case resources still lack matching data sources. |
| Provider index | Supported with follow-up | Auth, version, permissions, and quickstart exist; support matrix should remain visible. |
| Migration/import guidance | Supported | Full-appliance dependency-order migration guide exists in `docs/migration-import.md`. |
| Registry release workflow | Supported with preflight | GoReleaser, signed checksums, Registry manifest, changelog, and tag workflow exist; confirm secrets and dry-run before first tag. |

## Upstream-Blocked Register

The public register and maintenance workflow live in [`docs/upstream-blocked.md`](../../docs/upstream-blocked.md).

| Resource/domain | Upstream item | Action |
|---|---|---|
| System general settings | OPNsense System Settings MVC roadmap; current `core/initial_setup` evidence is wizard-only, not a durable singleton API | Watch release notes, API docs, controllers/models, ACL/menu entries, and adopt only after stable target-release get/set semantics are available. |

## Release Matrix

- **Supported:** 108 resources and 92 data sources listed in `support-matrix.md` and generated docs.
- **Coming:** data-source parity for singleton and sensitive special cases; gateway-group lifecycle validation for OPNsense 26.7.7 (Story 31.0k).
- **Needs research:** Kea DHCPv4 option, Kea DDNS, HASync configuration, HASync status/actions, and PPPoE semantics.
- **Upstream-blocked:** system general settings. Gateway groups moved to Coming for the verified 26.7.7 target; see Story 31.0k.
