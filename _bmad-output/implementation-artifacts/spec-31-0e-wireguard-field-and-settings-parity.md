---
title: 'Story 31.0e: WireGuard Field and Settings Parity'
type: 'feature'
created: '2026-10-19'
status: 'blocked'
baseline_revision: '369737ddb8e4d40caed31ff40905674e678ca422'
review_loop_iteration: 0
followup_review_recommended: false
context:
  - '_bmad-output/implementation-artifacts/epic-31-context.md'
warnings: []
deferred: []
---

<intent-contract>

## Intent

**Problem:** The existing WireGuard peer/server owners flatten multi-value API fields, omit PSK and material server fields, and provide no owner for daemon settings. Imports can therefore lose authoritative API state, while generic lifecycle handling can orphan saved mutations or accept ambiguous responses.

**Approach:** Harden the existing peer/server owners around the released core WireGuard API, add typed advanced fields and an import-first settings singleton, preserve write-only secrets, and prove lifecycle/drift/restoration on a disposable OPNsense 26.7.7 appliance before promoting exactly six audited rows.

## Boundaries & Constraints

**Always:** Preserve public resource names, UUID import, the global mutation lock, Terraform/OpenTofu compatibility, and existing scalar state through explicit schema migration when `tunnel_address`/relationship fields become sets. Treat OPNsense 24.1+ WireGuard as core. Use strict source-backed missing-object, mutation, delete, selected-map, boolean, and numeric decoding; accept reconfigure success only as `result=ok`. Retain committed identity and a private pending-activation marker across mutation/reconfigure/read-back failure, retrying activation without repeating mutation. Keep peer PSK and server private key resource-only, Sensitive, and prior-state-preserved when API reads omit them. Make server `public_key` Optional+Computed and `instance` Computed; send an empty instance on create and authoritative API truth on refresh. Treat server `peers` as the configured association and peer `servers` as its API backreference; serialize relation changes and document that one configuration must not manage both sides independently. Use fixed singleton ID `wireguard_settings`; Create is import-first, Update performs set/read/reconfigure, and Delete is state-only. Snapshot and exactly restore settings during live proof.

**Never:** Add a duplicate `wireguard_client` owner, expose write-only secrets through data sources or logs, silently coerce malformed API values to empty state, erase state on `{}`, `null`, missing monads, or ambiguous HTTP errors, repeat a saved mutation during recovery, enable the daemon for acceptance, mutate production, broaden scope to unaudited CARP/debug/endpoint/peer-DNS/allowed-IP fields, or promote audit/status claims before digest-bound live proof and zero-leftover cleanup.

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
|----------|---------------|---------------------------|----------------|
| Peer parity | PSK, multiple CIDR tunnel addresses, endpoint, keepalive, server backreference | Deterministic set/CSV request, strict selected-map read, PSK preserved across blank API reads, stable import/plan | Reject invalid keys/CIDRs/ports, malformed selectors/numbers, and unknown members without mutation |
| Server parity | Multiple tunnel/DNS addresses, MTU, route suppression/gateway, peer bindings, public/private keys | Exact released wire keys; readable values round-trip; instance is API-assigned; private key remains stable and hidden | Reject invalid keys/addresses/MTU/gateway relations and ambiguous relation ownership |
| Schema upgrade | Existing scalar `tunnel_address`/`servers` state | Upgrade to equivalent sets without replacement or state editing | Diagnose malformed legacy CSV; never discard members |
| Partial mutation | Save succeeds, UUID/result/read-back/reconfigure is missing or ambiguous | Recover exact owned object, retain identity/confirmed state, mark pending activation, retry reconfigure only | Diagnose zero/multiple recovery matches; second creator emits no traffic until owner lock releases |
| Missing object | Exact empty-array item response | Remove item resource from state | `{}`, null, missing monad, malformed object, 404, auth/plugin/server failures retain state and diagnose |
| Settings singleton | Import `wireguard_settings`, toggle disabled-state value, refresh, destroy | General get/set is authoritative; one reconfigure; destroy makes zero appliance calls | Wrong import ID or malformed/ambiguous saved/reconfigure bodies diagnose without claiming state |
| Acceptance cleanup | Marker-tagged disabled peer/server and settings snapshot | Terraform deletes leave zero tagged items before rollback; settings restore byte-semantically exactly | Fail closed on identity/version/collision mismatch; cleanup reports remaining UUIDs and cannot mask provider-delete failure |

</intent-contract>

## Code Map

- `internal/service/wireguard/{peer,server}_{schema,model,resource,data_source}.go` -- existing owners and thin generic CRUD at `peer_resource.go:23-184` / `server_resource.go:23-184`; add strict typed fields, schema upgrades, state preservation, and owner-locked recovery.
- `internal/service/wireguard/exports.go:12-25` -- register one settings resource/data source; expected module totals become 3/3.
- `internal/service/wireguard/*_test.go` -- current coverage is basic model/import/update only; add schema migration, exact payload, malformed-response, concurrency, recovery, missing-object, secret, association, and singleton contracts.
- `internal/service/openvpn/{pending_activation.go,instance_resource.go}` and `internal/service/unbound/{create_lock.go,domain_override_resource.go,general_resource.go,settings_contract_test.go}` -- reuse private activation state, exact recovery, owner lock, state-only singleton delete, and handshake-driven concurrency patterns.
- `pkg/opnsense/{crud.go,reconfigure.go,errors.go,reqopts.go,types.go}` -- extend owner-selective affirmative `result=ok` reconfigure behavior without weakening Kea `status=ok`; retain strict non-2xx/mutation/delete contracts.
- `internal/tfconv/tfconv.go` -- deterministic set/CSV conversion; reject unresolved or malformed members instead of emitting clears.
- `templates/{resources,data-sources}/wireguard_*.md.tmpl`, `examples/{resources,data-sources}/opnsense_wireguard_*`, `examples/compositions/wireguard-vpn`, and generated `docs/` -- document core 24.1+, typed fields, imports, secret limits, association ownership, and singleton semantics.
- `internal/service/wireguard/wireguard_parity_acceptance_test.go`, `test/scripts/run-vagrant-acceptance.sh`, `test/scripts/run-story-31-0e-acceptance.sh`, and `test/scripts/story-31-0e-code-digest.sh` -- disposable identity/version preflight, exact digest, full lifecycle/drift, secret scans, Terraform-delete zero proof, rollback, and settings restoration.
- `_bmad-output/planning-artifacts/provider-ecosystem-coverage-audit.json` plus generated Markdown -- promote only six Story 31.0e rows after executed proof; expected inventory becomes 109 resources and 95 data sources.
- `_bmad-output/implementation-artifacts/{31-0e-wireguard-field-and-settings-parity.md,sprint-status.yaml}` and planning support/roadmap files -- synchronize evidence-bound status and remaining-story counts only after final proof.
- OPNsense stable/26.7 WireGuard controllers/models and pinned `browningluke/opnsense` v0.26.0 commit `33c2a03c2bc1a7339cb5aa5b611e15014e27f7ba` -- read-only evidence for endpoints, wire keys, defaults, validators, and singleton behavior; do not vendor.

## Tasks & Acceptance

**Execution:**
- `internal/service/wireguard` -- implement versioned state migration, strict models/schemas, PSK and advanced server fields, association ownership, hardened CRUD recovery, settings singleton/data source, and complete boundary tests.
- `pkg/opnsense` and `internal/tfconv` -- add only the shared owner-selective result/recovery/conversion primitives proved necessary by WireGuard tests, preserving existing owners.
- `templates`, `examples`, `docs`, provider registration tests, and composition example -- generate accurate public schemas/examples and reconcile inventory counts.
- `internal/service/wireguard/wireguard_parity_acceptance_test.go` and `test/scripts` -- implement digest-bound disposable proof with immediate zero checks, exact restore, and credential/key redaction scans.
- Coverage audit, story/spec, sprint status, roadmaps, and support matrix -- update exactly the evidenced six rows and final status after promoted-tree local/live proof.

**Acceptance Criteria:**
- Given existing scalar WireGuard state, when the new schemas load it, then typed tunnel/relationship sets preserve every member with no manual state edit or replacement.
- Given advanced disabled peer/server configurations, when create, read, import, no-change plan, update, populated-to-empty transition, out-of-band drift, repair, and delete execute, then every readable field matches API truth and PSK/private key remain stable and absent from data sources/logs.
- Given concurrent peer/server creation or ambiguous committed mutation outcomes, when recovery runs, then preflight, mutation, and recovery remain inside one owner lock, exactly one object is adopted, and no competing request is emitted before release.
- Given malformed/missing API metadata, selectors, booleans, numbers, result bodies, or objects, when refresh/mutation runs, then actionable diagnostics retain recoverable state and never misclassify ambiguity as absence or success.
- Given imported `wireguard_settings`, when `enabled` is updated and refreshed, then exact general get/set and one `result=ok` reconfigure round-trip; destroy removes state without an appliance mutation.
- Given final disposable OPNsense 26.7.7 acceptance, when Terraform completes, then settings restore exactly, Terraform itself leaves zero tagged peers/servers before rollback, final searches remain zero, secrets appear in no retained artifact, and only the six executed audit rows become covered.
- Given the final promoted digest, when deterministic generation, focused/full tests, structure/audit checks, `make check`, and a fresh exact-digest live rerun complete, then every gate is green and review/status/evidence contain no pending or stale claim.

## Spec Change Log

## Review Triage Log

## Design Notes

Released WireGuard uses CSV requests and selected-map responses for multi-value fields. Terraform sets provide stable order-insensitive state, but existing scalar state must be upgraded explicitly. The official peer `servers` value is derived from server membership updates; the provider therefore treats server `peers` as the writable association and peer `servers` as the authoritative backreference rather than allowing two competing writers.

General settings are an import-first singleton because the appliance object already exists. Its public identity is `wireguard_settings`; Create rejects with import guidance, Update sets only `enabled`, and Delete is state-only. WireGuard activation differs from Kea: success is `{"result":"ok"}`, so strictness must be owner-selective rather than changing the established Kea `status=ok` contract.

## Verification

**Commands:**
- `go test -count=1 ./internal/service/wireguard/... ./pkg/opnsense/... ./internal/tfconv/...` -- expected: schema migration, encoding/decoding, lifecycle/recovery, singleton, concurrency, and secret contracts pass.
- `go generate ./... && go generate ./... && git diff --exit-code` -- expected: generated docs/audit output is deterministic and current.
- `go test -count=1 ./internal/coverageaudit && bash test/scripts/validate-structure.sh && git diff --check` -- expected: exact registration/audit counts, structure, and whitespace pass.
- `OPNSENSE_WIREGUARD_PARITY=1 test/scripts/run-story-31-0e-acceptance.sh` with externally attested disposable credentials -- expected: non-skipped peer/server/settings lifecycle, drift repair, state-only destroy, exact restore, secret scan, and zero leftovers on the expected digest.
- `DEVRAIL_IMAGE=ghcr.io/devrail-dev/dev-toolchain:1.12.24 make check` -- expected: lint, format, tests, security, scan, and docs all pass.

## Auto Run Result — instance payload conflict

Status: blocked
Blocking condition: implementation verification failed because the generated explicit-empty create-instance contract contradicts the released OPNsense 26.7.7 API. Candidate local gates passed, but final lifecycle proof did not. Bounded control omitted the instance key, received result=saved and API-assigned instance=0, then deleted the object and reconfigured successfully. Both namespaces returned zero and settings restored; no audit rows promoted. See `31-0e-wireguard-live-evidence.md`.

The authorized continuation will re-enter planning with this observed API evidence, preserving API-assigned numbering and the remaining contract. This is not approval for production writes.
