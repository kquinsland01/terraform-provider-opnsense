---
title: 'Story 30.2: Implement Interface Assignment/IP Resource for OPNsense 26.7.6+'
type: 'feature'
created: '2026-10-17'
status: 'done'
baseline_revision: '1cea3c099a86d6a133fc41f52fb3666537023c1a'
review_loop_iteration: 1
followup_review_recommended: true
context:
  - 'AGENTS.md'
  - 'DEVELOPMENT.md'
  - '_bmad-output/implementation-artifacts/epic-30-context.md'
warnings:
  - 'oversized'
deferred: []
---

<intent-contract>

## Intent

**Problem:** OPNsense 26.7.6 released the complete `interfaces/assignment` API, but the provider cannot yet manage base interface enablement and IPv4/IPv6 settings or explain incompatibility with older appliances.

**Approach:** Add a hand-written `opnsense_interface_assignment` resource that adopts an existing `wan`, `lan`, or `optN` assignment by name, applies the safe released address-setting subset through `set_item` plus `reconfigure`, reads API state back, and proves lifecycle/idempotence on a disposable 26.7.6+ appliance.

## Boundaries & Constraints

**Always:** Use the interface name as immutable identity and state ID; target OPNsense 26.7.6+; serialize mutation through the existing client lock; parse HTTP-200 validation bodies; reconfigure before read-back; preserve unmanaged upstream fields; emit a specific compatibility diagnostic for old, partial, missing, or unauthorized assignment APIs; test only a disposable non-management interface; treat Terraform destroy as state-only and document that the appliance assignment is retained.

**Never:** Create an unknown assignment through `set_item`; call destructive `del_item`; mutate WAN/LAN management during acceptance; echo planned/request values into state; support PPP/PPPoE, 6rd, 6to4, identity-association, track-interface, hardware tuning, MTU/MSS, or advanced DHCP options in this first resource; scrape PHP pages or edit `config.xml`.

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
|----------|---------------|----------------------------|----------------|
| Adopt/update | Existing `wan`, `lan`, or `optN`; safe IPv4/IPv6 mode, CIDR, gateway, enabled flag | POST wrapped `interface` payload to `set_item/{name}`, POST `reconfigure`, GET `get_item/{name}`, and store normalized API values | Surface upstream field validation and reconfigure failure, including persisted-but-not-active warning |
| Static addressing | `staticv4`/`staticv6` with address and prefix | Join Terraform address/prefix into API CIDR; split API CIDR back into stable state | Reject missing/invalid address-prefix combinations before mutation |
| Dynamic/none | IPv4 `none`/`dhcp`; IPv6 `none`/`dhcp6`/`linklocal`/`slaac` | Clear static CIDR fields while preserving unrelated assignment fields | Reject static-only values with a non-static mode |
| Unsupported appliance | Firmware below 26.7.6, missing endpoint, partial response shape, or endpoint-not-found body | No mutation; clear diagnostic names the required expanded assignment API and minimum version | Do not misreport the core API as a missing plugin |
| Import/read | Import ID is an existing interface name | Set ID/name, GET upstream state, and produce no-change plan for represented fields | Remove state only when the assignment genuinely no longer exists |
| Destroy | Resource removed from configuration | Remove Terraform state without deleting or disabling the appliance interface | Document retained upstream object and brownfield re-import behavior |

</intent-contract>

## Code Map

- `internal/service/iface/assignment_resource.go` -- new hand-written resource lifecycle, schema, immutable name identity, state-only delete, import, compatibility gate, update/reconfigure/read-back.
- `internal/service/iface/assignment_model.go` -- wrapped API model and explicit CIDR/selected-value/string-boolean conversions; preserve required/unmanaged fields returned by GET when constructing partial updates.
- `internal/service/iface/assignment_resource_unit_test.go` -- httptest coverage for payload/path, read-back normalization, invalid mode/value combinations, HTTP-200 validation failure, compatibility diagnostics, no-op delete, and import.
- `internal/service/iface/assignment_resource_test.go` -- serialized acceptance lifecycle on an operator-supplied disposable `26.7.6+` `optN`: apply, empty second plan, import, update, and cleanup restoration.
- `internal/service/iface/exports.go:12` -- register `newAssignmentResource`; provider already aggregates `iface.Resources()`.
- `pkg/opnsense/crud.go:65` and `pkg/opnsense/crud.go:99` -- reuse `Get` and `Update`; they unwrap the monad, acquire read/write synchronization, parse mutation responses, and call reconfigure while locked.
- `pkg/opnsense/errors.go:102` -- mutation-body parsing to retain; resource-specific 404/endpoint diagnostics must bypass the misleading generic plugin message.
- `pkg/opnsense/types.go:14` -- reuse OPNsense string booleans and `SelectedMap`; add only narrowly reusable capability/version helpers under `pkg/opnsense/` if resource-local logic would duplicate client behavior.
- `internal/service/system/system_info_data_source.go:82` -- firmware-info request/response reference; do not couple the new resource to the private data-source implementation.
- `internal/acctest/acctest.go:28` and `test/scripts/run-vagrant-acceptance.sh:62` -- reuse credentials, endpoint prechecks, serial execution, and result capture; require an explicit disposable interface variable and never default to management interfaces.
- `test/Vagrantfile:16` -- read-only evidence: current VirtualBox box is 25.7 and cannot satisfy this story; acceptance may use the existing secret-backed runner only when its target is verified disposable and 26.7.6+.
- `templates/resources/interface_assignment.md.tmpl`, `examples/resources/opnsense_interface_assignment/resource.tf`, and generated `docs/resources/interface_assignment.md` -- document safe modes, minimum version, retained-on-destroy semantics, import, and blast-radius warning.
- `_bmad-output/implementation-artifacts/30-2-prototype-interface-assignment-ip-resource.md` and `_bmad-output/implementation-artifacts/sprint-status.yaml` -- record implementation/evidence and move 30.1/30.2/30.3/Epic 30 to truthful final states only after gates pass.

## Tasks & Acceptance

**Execution:**
- `internal/service/iface/assignment_model.go` -- model the safe assignment/IP subset and lossless conversions while carrying required current values -- avoid clobbering shared interface configuration.
- `internal/service/iface/assignment_resource.go` and `internal/service/iface/exports.go` -- implement/register guarded adopt-update-read lifecycle, state-only destroy, and import -- expose the released API safely.
- `internal/service/iface/assignment_resource_unit_test.go` -- exhaustively test the I/O matrix with an HTTP fixture -- prove validation and diagnostics without requiring a legacy appliance.
- `internal/service/iface/assignment_resource_test.go` and acceptance runner docs/config -- add a serial, opt-in disposable-interface lifecycle test with restoration -- prove apply, drift/read-back, empty second plan, import, and update on 26.7.6+.
- Registry template/example/generated docs -- publish schema, version/safety limits, retained-on-destroy behavior, and import syntax -- prevent users from assuming assignment creation or destructive deletion.
- BMad/support artifacts -- record exact live evidence and synchronize statuses/support claims -- do not label the feature Supported without the disposable acceptance proof.

**Acceptance Criteria:**
- Given an existing disposable `optN` on OPNsense 26.7.6+, when Terraform applies supported enablement and IPv4/IPv6 settings, then the assignment API reflects them and a second plan is empty.
- Given imported `wan`, `lan`, or `optN` state, when refresh completes, then ID/name and represented settings come from `get_item` and out-of-band changes appear as drift.
- Given an incompatible, partial, missing, or unauthorized assignment API, when planning or applying the resource, then no mutation occurs and the diagnostic identifies OPNsense 26.7.6+ expanded assignment API requirements.
- Given the resource is removed, when destroy runs, then Terraform forgets it without calling `del_item`, disabling the interface, or removing firewall references.
- Given all generated docs and tests are present, when repository validation runs, then `make check` and `git diff --check` pass.

## Spec Change Log

- 2026-10-17: Live OPNsense 26.7.7 acceptance identified two released API
  wire-format details: indexed choice arrays must be reduced to their selected
  numeric value, and Terraform `none` modes must be sent as the API's blank
  option. Added regression coverage and verified the complete disposable
  `opt1` lifecycle, including state restoration.

## Review Triage Log

### 2026-10-18 — Build-auto review pass 1

Verdicts: 31 total — 2 high, 13 medium, 5 low, 11 false-positive.
Seventeen findings were patched (2 high, 13 medium, 2 low); fourteen were
rejected with the rationale recorded below. Duplicate findings remain separate
rows so each review layer has a complete disposition.

| # | Layer | Verdict | Route | Finding | Disposition / evidence |
|---:|---|---|---|---|---|
| 1 | Verification | medium | patch | Resource validator wiring was tested only through helpers. | Added tests that obtain and execute the resource `ConfigValidators` for both address families. |
| 2 | Verification | medium | patch | Reconfigure-failure persisted-state recovery was untested. | Added an HTTP-contract test covering the persisted-but-not-active diagnostic and recovery state. |
| 3 | Verification | medium | patch | Non-default DHCP mappings and read-back were untested. | Added conversion and read-back cases for the represented DHCPv4/DHCPv6 fields. |
| 4 | Verification | medium | patch | Read removal for a vanished assignment versus a compatibility error was untested. | Added distinct empty-item and compatibility-response tests. |
| 5 | Blind | false-positive | reject | The capability gate does not preflight `set_item` and `reconfigure`. | The 26.7.6+ firmware and complete GET shape are the non-mutating release gate; mutation endpoints cannot be safely preflighted, and their failures already stop or report persisted-but-not-active state. |
| 6 | Blind | false-positive | reject | Treating HTTP 404 as compatibility could prevent normal not-found removal. | Live 26.7.7 returns HTTP 200 with an empty item for a missing assignment; 404 identifies an absent endpoint on the gated API. |
| 7 | Blind | medium | patch | Required upstream `if` and `enable` fields were not included in payload completeness checks. | Added both fields to the required-shape guard and regression tests. |
| 8 | Blind | high | patch | Updating an assignment in an unsupported active mode could clear PPP, track, or tunnel settings. | Added a pre-mutation supported-mode guard and tests; unsupported active modes now fail without a write. |
| 9 | Blind | medium | patch | Static address values with an omitted mode could be silently ignored. | Added cross-field validation that rejects static-only values when the corresponding mode is null. |
| 10 | Blind | low | reject | Name-only adoption always writes and reconfigures. | This is the documented adopt/update contract; redesigning Create as a conditionally read-only operation adds lifecycle ambiguity for an uncommon case. |
| 11 | Blind | false-positive | reject | DHCP attributes are allowed when DHCP is not the active mode. | OPNsense stores these settings independently for later mode transitions; the story does not require destructive clearing or DHCP-only coupling. |
| 12 | Blind | low | reject | `dhcp_reject_from` lacks a Terraform-side IPv4 element validator. | Upstream validates the field, and adding custom set-element validation is disproportionate to the low-risk diagnostic improvement. |
| 13 | Blind | low | patch | Acceptance restoration serialized map-backed choices nondeterministically. | Sorted choice keys before normalization so snapshot comparisons are deterministic. |
| 14 | Blind | medium | patch | Acceptance cleanup did not compare the final assignment with its captured snapshot. | Added normalized exact post-cleanup comparison. |
| 15 | Blind | medium | patch | Acceptance did not prove out-of-band drift detection. | Added a safe out-of-band enablement change and asserted a non-empty Terraform plan before repair. |
| 16 | Blind | false-positive | reject | Live acceptance covered static IPv4 rather than every IPv4/IPv6/DHCP combination. | The story requires one supported live lifecycle; the full mode and DHCP matrix is covered by unit and HTTP-contract tests. |
| 17 | Intent | false-positive | reject | The user-level intent includes ordered orchestration while the diff is a product change. | This diff is the first intentionally sequential story output; orchestration remains the parent workflow, not Story 30.2 code. |
| 18 | Intent | false-positive | reject | Product tests do not exercise the build-auto skill orchestration. | Repository tests appropriately verify provider behavior; skill execution is workflow evidence outside the product test surface. |
| 19 | Intent | false-positive | reject | The diff does not prove the skill was installed or invoked. | Skill rendering and invocation are session/workflow evidence and should not be encoded in provider source. |
| 20 | Intent | false-positive | reject | The diff does not enumerate every open story. | The backlog inventory was completed before this iteration; maintaining the entire queue is not a responsibility of this feature diff. |
| 21 | Intent | false-positive | reject | Only Story 30.2, not the full multi-story request, is implemented. | Stories are being executed in dependency order; this review intentionally covers the first implementation iteration. |
| 22 | Edge | medium | patch | Required upstream `enable` and `if` values were not enforced. | Same underlying issue as finding 7; the required-shape guard and tests now cover both fields. |
| 23 | Edge | low | reject | A concurrent unmanaged-field change between pre-read and locked update could be overwritten. | The race is real but narrow; eliminating it requires a client lock/read API redesign disproportionate to this story and is retained as residual risk. |
| 24 | Edge | high | patch | Mandatory post-mutation read-back failure could leave Terraform without recovery state. | The resource now records the valid pre-update state, including immutable identity, before returning the read-back diagnostic; a regression test covers it. |
| 25 | Edge | medium | patch | Transient GET transport failures were labeled as API incompatibility. | Assignment and firmware transport failures now use normal server diagnostics; only response-shape/endpoint/auth failures use compatibility diagnostics. |
| 26 | Edge | false-positive | reject | HTTP 404 for a vanished assignment might be misclassified. | Same live evidence as finding 6: item absence is HTTP 200 plus an empty payload; endpoint 404 is an API compatibility failure. |
| 27 | Edge | low | patch | Acceptance could run with identical initial and updated addresses. | Added an explicit distinct-address precondition. |
| 28 | Edge | medium | patch | Acceptance cleanup used an unbounded background context. | Added a 30-second cleanup context and retained best-effort restoration diagnostics. |
| 29 | Edge | false-positive | reject | Empty-object option normalization could lose an unmanaged value. | OPNsense uses an empty option object for no selection and expects a blank scalar on write; no retained unmanaged value exists in that representation. |
| 30 | Edge | medium | patch | The verification matrix omitted direct reconfigure, read-back, removal, and validator coverage. | Same gaps as findings 1–4; added focused HTTP-contract and framework-validator tests for every branch. |
| 31 | Edge | medium | patch | The stated drift guarantee lacked acceptance evidence. | Same underlying issue as finding 15; live acceptance now mutates the disposable assignment out of band and proves Terraform detects and repairs it. |

## Design Notes

`opnsense_interface_assignment` matches the upstream controller and existing provider namespace. Create is adoption: it updates an assignment that must already exist. Safe mode scope is IPv4 `none|staticv4|dhcp` and IPv6 `none|staticv6|dhcp6|linklocal|slaac`; modes needing additional tunnel/track/PPP fields are intentionally deferred. State-only destroy is required because upstream `del_item` can remove firewall references and sever connectivity.

## Verification

**Commands:**
- `go test ./internal/service/iface ./pkg/opnsense` -- expected: unit and HTTP-contract tests pass.
- `TF_ACC=1 go test -p 1 -run TestAccInterfaceAssignment ./internal/service/iface` with disposable 26.7.6+ credentials/interface -- expected: apply, empty plan, import, update, and restoration pass.
- `make docs` -- expected: generated Registry documentation matches templates/examples.
- `make check` -- expected: lint, format, test, security, scan, generated-code, and docs gates pass in DevRail.
- `git diff --check` -- expected: no whitespace errors.

**Live evidence:** `TestAccInterfaceAssignment` passed on a disposable OPNsense
26.7.7 VM with isolated `opt1`, proving apply, empty plan, import, update,
out-of-band drift detection and repair, and exact post-test restoration.

## Auto Run Result

- **Result:** Story 30.2 implemented and reviewed against baseline
  `1cea3c099a86d6a133fc41f52fb3666537023c1a`.
- **Review:** Four independent layers produced 31 findings. Seventeen were
  patched; fourteen were rejected with explicit rationale in the triage log.
  Because the first pass patched multiple medium/high findings, a follow-up
  review is recommended for the PR even though all local gates pass.
- **Verification:** Disposable OPNsense 26.7.7 acceptance passed after review;
  DevRail `make check` passed lint, format, tests, security, scan, and docs;
  `git diff --check` passed.
- **Files:** Added the assignment model/resource, unit and acceptance tests,
  Registry template/generated docs/examples, resource registration, acceptance
  runner inputs, and synchronized BMad artifacts.
- **Residual risk:** A narrow external-concurrency window remains between the
  pre-read and serialized mutation, and the live lifecycle intentionally proves
  one safe static-IPv4 path while the broader mode matrix is fixture-tested.
