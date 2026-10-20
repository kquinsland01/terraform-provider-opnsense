# Story 31.0l: Kea Option and DDNS Stable API Validation

## Status

backlog

## Priority and Owner

P1 — provider implementation owner; after active parity work, before Stories 31.1–31.3. Reconcile Stories 11.3/21.4 against completed 31.0b; do not repeat subnet PXE/TFTP, DHCPv6 peer or delegated-prefix pool implementation.

## Entry Point and Evidence

Read completed `31-0b-kea-field-and-surface-parity.md`, `internal/service/kea/dhcpv4_subnet_{model,schema}.go`, `kea_parity_acceptance_test.go`, `exports.go`, and tagged core 26.7.7 `Kea/Api/Dhcpv4Controller.php`, `DdnsController.php`, `KeaDdns.xml`, the base mutable controller and Kea service/config templates. Stable DHCPv4 has `/api/kea/dhcpv4/{get,add,set,del,search}_option`. DDNS has an explicit `/api/kea/ddns/get` response filtered to `ddns.general`; inspect inherited set/save semantics instead of assuming the filtered read response owns every DDNS model field. Stable source availability does not prove lifecycle or installed target availability.

## Tasks

1. Write `_bmad-output/planning-artifacts/kea-option-ddns-api-validation.md` with exact core/plugin versions, source refs, wrapper/default/required-field semantics, service reconfigure path, selected-map shapes and each verified or unverified capability.
2. On a disposable appliance with DHCP/DDNS disabled, probe generic option CRUD/search independently from subnet `option_data`. Verify safe linkage/scope, import shape, ordering, update/clear semantics, reconfigure, external drift repair and teardown. Demonstrate subnet PXE/TFTP fields are already covered, not a prerequisite for a second owner of the same object.
3. Separately prove DDNS inherited write behavior, strict readable/write-only boundaries, targeted update preservation, reconfigure and singleton state-only destroy semantics without enabling live DNS changes. If read filtering prevents safe complete ownership, document the precise unsupported subset rather than replacing the entire model.
4. For each proven missing owner, create and register an implementation follow-up story with concrete `internal/service/kea/` files, exports, docs/examples and lifecycle gates; if a surface fails validation, record its version/endpoint-specific blocker and owned next action. Reconcile old story/public support text in the same change.
5. Keep Foreman Kea Control Agent, `host_cmds`/`lease_cmds` hooks and the operational Kea-to-Unbound integration as separate compatibility/ownership decisions. Do not remove plugins, Ansible or production DHCP ownership as part of this validation.

## Acceptance Criteria

- **Given** completed 31.0b parity, **when** the audit is reconciled, **then** supported subnet PXE/TFTP fields stay supported and only genuinely missing generic option/DDNS owners become follow-up work.
- **Given** stable option/DDNS APIs on a disposable target, **when** writes/import/drift/reconfigure/cleanup are exercised, **then** the report identifies every proven lifecycle and every unverified or unsafe behavior with exact core/plugin versions and no broad configuration overwrite.
- **Given** the validation outcome, **when** planning/support docs change, **then** each buildable gap has a registered implementation story and each remaining block has a concrete next action; neither stable API presence nor native DDNS implies Foreman hook support or a production cutover.

## Verification

Source/payload audit, disposable non-skipped lifecycle proof, existing Kea regression tests, generated-doc checks when relevant, `git diff --check` and full `make check`. Never label unavailable proof passed.
