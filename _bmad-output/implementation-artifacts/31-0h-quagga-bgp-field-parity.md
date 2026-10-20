# Story 31.0h: Quagga/BGP Field Parity

## Status

backlog

## Priority

P0 compatibility gate after active Story 31.0e; remaining field parity stays P1. Complete before Stories 31.1–31.3 and any production FRR 2.0 provider adoption.

## Evidence and Scope

Extend the existing BGP neighbor and route-map owners. Neighbor gaps include BFD, authentication, connect timer, default route, multihop, route-reflector, weight, connected-check, attribute preservation, and next-hop-self-all. Route-map gaps include AS-path/community/prefix-list references and route-map ID semantics. Use existing stable `quagga/bgp` item CRUD/toggle/reconfigure endpoints and retain the broader local FRR surface.

## FRR 2.0 Compatibility Gate (2026-10-20)

OPNsense 26.7.7 includes `os-frr` 2.0. The pinned [plugin migration notes](https://github.com/opnsense/plugins/blob/5ac2959c8514b4f60e94d65439be5f629f10521f/net/frr/pkg-descr) replace neighbor `multiprotocol` with explicit `family`, move peer-group policies into address-family configuration, group policy references, tighten validation and add automatic routing-protocol firewall rules. Existing `internal/service/quagga/bgp_neighbor_model.go` still sends `multiprotocol`; this is a source-proven compatibility risk, not a reproduced live failure or a claim that every 26.7.7 appliance has the same plugin version.

Entry point: inventory core **and installed os-frr versions** on the disposable target; inspect pinned `net/frr/.../Quagga/BGP.xml`, BGP migrations, `Api/BgpController.php`, service/reconfigure and firewall templates before editing `internal/service/quagga/`. Keep existing Terraform type names and state addresses. Extend the existing neighbor, peer-group, route-map and policy owners rather than adding parallel resources.

1. Establish explicit version/capability selection for old and 2.0 plugin payloads. Do not infer plugin version from core version alone or silently drop unsupported configured fields.
2. Model address-family selection without losing IPv6-only or dual-stack intent; define upgrade/state handling for existing `multi_protocol` configurations, unknowns and imports. Unsupported combinations must fail before writes with actionable diagnostics.
3. Re-audit grouped AS-path/community/prefix-list/route-map relationships, peer-group address-family policy and reference ordering. API route existence alone is not parity proof.
4. Check generated automatic firewall rules and effective routing policy on the isolated target, including no unintended default-route export; no production routing/firewall changes are authorized by this provider story.
5. Preserve linked policy identities and strict reload failure reporting. If old-plugin acceptance is unavailable, leave that support claim explicitly unverified rather than replacing its proof with a mocked 2.0 response.

## Acceptance Criteria

- **Given** advanced neighbor/route-map objects, **when** imported, **then** all audited fields read back and matching configuration plans no change.
- **Given** linked policy objects, **when** updated or drifted, **then** UUID relationships and ordering remain stable through reconfigure.
- **Given** existing local-only FRR resources, **when** parity fields ship, **then** registrations and state addresses remain backward compatible.

- **Given** supported legacy and FRR 2.0 plugin versions, **when** create/import/read/update and external drift run, **then** the same existing Terraform resource addresses converge with correct family and policy semantics and no perpetual diff; record version-specific non-skipped proof.
- **Given** an existing `multi_protocol` state or an unsupported plugin capability, **when** planning upgrades/imports, **then** state handling is deterministic and unsupported intent cannot be silently discarded or written.
- **Given** a 2.0 configuration with linked policies and protocol auto rules, **when** reconfigure runs, **then** generated configuration, route filters, effective firewall rules, ordering and failure diagnostics are verified on a disposable appliance and exact cleanup restores baseline.

## Verification

Quagga unit/schema/generated tests, real Terraform import/no-change/drift/state-upgrade tests, version-specific disposable BGP/policy/reconfigure/cleanup proof, docs generation, and `make check`. Record exact core/plugin versions, source digest and unverified rows; do not claim production compatibility from source inspection alone.
