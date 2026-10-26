# Story 31.0k: Gateway Group Stable API Validation

## Status

review — stable source/direct-API probes recorded on 26.7.7; implementation follow-ups registered. Terraform resource/lifecycle support remains pending.

## Priority and Owner

P1 — provider implementation owner; after the active WireGuard and FRR compatibility gate, before Stories 31.1–31.3. This closes the stable-source blocker from historical Story 5.6, not an authorization to change production multi-WAN configuration.

## Entry Point and Evidence

Read historical `5-6-gateway-group-resource.md`, `internal/service/system/gateway_*`, `pkg/opnsense/` request/lifecycle helpers and the tagged [GroupSettingsController](https://github.com/opnsense/core/blob/26.7.7/src/opnsense/mvc/app/controllers/OPNsense/Routing/Api/GroupSettingsController.php), `Routing/GatewayGroups.xml` and `GatewayGroups.php` before code. OPNsense 26.7.7 exposes `/api/routing/group_settings/{get,add,set,del,search,reconfigure}`. Wrapper is `gateway_group`; reconfigure runs `interface routes configure`; deletion refuses references to the group. Do not copy individual-gateway wrapper keys or infer support on older core versions.

## Tasks

1. Record exact core version and source revision and inspect model names/defaults/required fields, tier member representation (`item` through `item5`), trigger/pool options, group-name collisions, rename immutability and deletion dependencies. Save sanitized evidence only in `_bmad-output/planning-artifacts/gateway-group-api-validation.md`.
2. Use a disposable appliance and isolated synthetic gateways/groups only. Prove API create/read/update/import-shaped decoding, tier ordering, reconfigure, external drift repair, in-use deletion rejection and exact cleanup. Preserve default-route and management access; if a safe isolated topology cannot prove a case, record that case unverified and block its support claim.
3. If all required semantics pass, create the implementation follow-up story under `_bmad-output/implementation-artifacts/` and register it in `sprint-status.yaml` and `post-release-epics.md` in the same change. Scope it to `internal/service/system/gateway_group_{resource,schema,model,data_source}.go`, registration in `exports.go`, generated docs/examples and real Terraform lifecycle coverage. Use `opnsense_system_gateway_group` unless a documented schema review chooses otherwise; do not duplicate individual gateways/static routes.
4. If validation fails, keep the domain validation-blocked with exact endpoint/version/error and an owned next action, not falsely upstream-absent or supported. Update `docs/upstream-blocked.md`, support matrix and core gap analysis together.

## Acceptance Criteria

- **Given** the stable 26.7.7 controller/model, **when** the source and live probes are compared, **then** wrapper, types, defaults, tier/trigger semantics and reconfigure behavior have explicit matching evidence.
- **Given** isolated groups with member gateways and references, **when** CRUD/drift/reconfigure/deletion tests run, **then** convergence and dependency failures are deterministic without modifying default routes or unrelated policy, and teardown restores baseline.
- **Given** completed validation, **when** status is updated, **then** either a registered actionable implementation story exists or a concrete validation blocker with owner/next action remains; API existence alone never becomes a provider support claim.

## Verification and Release Boundary

Disposable API/real Terraform proof as applicable, generated-doc checks if support text changes, `git diff --check` and full `make check`. Record unavailable tests explicitly. No production upgrade, Terraform adoption, live ownership transfer or new published support is implied.

## Validation handoff — 2026-10-25

See [direct-API contract and executed/unverified cases](../planning-artifacts/gateway-group-api-validation.md).
Implementation owner: 31.0m. Source availability and these direct-API
probes are not a provider support claim; real Terraform import/no-change/drift,
failure/restart recovery and final restoration still belong to implementation.
