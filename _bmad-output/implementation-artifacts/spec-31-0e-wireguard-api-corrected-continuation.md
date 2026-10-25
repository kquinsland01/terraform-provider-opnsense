---
title: 'Story 31.0e: WireGuard Field and Settings Parity — API-corrected continuation'
type: 'feature'
created: '2026-10-20'
status: 'done'
baseline_revision: '369737ddb8e4d40caed31ff40905674e678ca422'
review_loop_iteration: 1
followup_review_recommended: true
context:
  - '_bmad-output/implementation-artifacts/epic-31-context.md'
warnings: [oversized]
deferred: []
---

<intent-contract>

## Intent

**Problem:** The existing WireGuard peer/server owners flatten multi-value API fields, omit PSK and material server fields, and provide no owner for daemon settings. Imports can therefore lose authoritative API state, while generic lifecycle handling can orphan saved mutations or accept ambiguous responses.

**Approach:** Harden the existing peer/server owners around the released core WireGuard API, add typed advanced fields and an import-first settings singleton, preserve write-only secrets, and prove lifecycle/drift/restoration on a disposable OPNsense 26.7.7 appliance before promoting exactly six audited rows.

## Boundaries & Constraints

**Always:** Preserve public resource names, UUID import, the global mutation lock, Terraform/OpenTofu compatibility, and existing scalar state through explicit schema migration when `tunnel_address`/relationship fields become sets. Treat OPNsense 24.1+ WireGuard as core. Use strict source-backed missing-object, mutation, delete, selected-map, boolean, and numeric decoding; accept reconfigure success only as `result=ok`. Retain committed identity and a private pending-activation marker across mutation/reconfigure/read-back failure, retrying activation without repeating mutation. Keep peer PSK and server private key resource-only, Sensitive, and prior-state-preserved when API reads omit them. Make server `public_key` Optional+Computed and `instance` Computed; omit the instance wire key on create so the API assigns it, preserve the assigned value through updates, and read authoritative API truth on refresh. Treat server `peers` as the configured association and peer `servers` as its API backreference; serialize relation changes and document that one configuration must not manage both sides independently. Use fixed singleton ID `wireguard_settings`; Create is import-first, Update performs set/read/reconfigure, and Delete is state-only. Snapshot and exactly restore settings during live proof.

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

- `_bmad-output/implementation-artifacts/31-0e-wireguard-live-evidence.md` -- source/read-only sanitized failing-contract and successful omitted-field control; latest candidate base is digest `5eb86d55e95b65e7cf8f17179164de1dd3a9b730aa49bc6fbe90dc168b90fc22`, not promoted parity.
- `internal/service/wireguard/server_model.go:56,120-126` -- explicit empty request serialization is the proven fault; fix create omission without losing assigned instance on refresh/update. Add exact absent-key request assertions and a non-empty API-assigned read-back regression.
- `internal/service/wireguard/wireguard_parity_acceptance_test.go:31-118` -- preserve lifecycle checks and make success-path singleton-destroy assertions failure-aware, so an earlier create failure cannot masquerade as an appliance mutation. Add live singleton Update proof, not solely a mocked/unit Update check.

- `internal/service/wireguard/{peer,server}_{schema,model,resource,data_source}.go` -- baseline owners and then-thin generic CRUD at `peer_resource.go:23-184` / `server_resource.go:23-184`; add strict typed fields, schema upgrades, state preservation, and owner-locked recovery.
- `internal/service/wireguard/exports.go:12-25` -- register one settings resource/data source; expected module totals become 3/3.
- `internal/service/wireguard/*_test.go` -- baseline coverage was basic model/import/update only; add schema migration, exact payload, malformed-response, concurrency, recovery, missing-object, secret, association, and singleton contracts.
- `internal/service/openvpn/{pending_activation.go,instance_resource.go}` and `internal/service/unbound/{create_lock.go,domain_override_resource.go,general_resource.go,settings_contract_test.go}` -- reuse private activation state, exact recovery, owner lock, state-only singleton delete, and handshake-driven concurrency patterns.
- `pkg/opnsense/{crud.go,reconfigure.go,errors.go,reqopts.go,types.go}` -- extend owner-selective affirmative `result=ok` reconfigure behavior without weakening Kea `status=ok`; retain strict non-2xx/mutation/delete contracts.
- `internal/tfconv/tfconv.go` -- deterministic set/CSV conversion; reject unresolved or malformed members instead of emitting clears.
- `templates/{resources,data-sources}/wireguard_*.md.tmpl`, `examples/{resources,data-sources}/opnsense_wireguard_*`, `examples/compositions/wireguard-vpn`, and generated `docs/` -- document core 24.1+, typed fields, imports, secret limits, association ownership, and singleton semantics.
- `internal/service/wireguard/wireguard_parity_acceptance_test.go`, `test/scripts/run-vagrant-acceptance.sh`, `test/scripts/run-story-31-0e-acceptance.sh`, and `test/scripts/story-31-0e-code-digest.sh` -- disposable identity/version preflight, exact digest, full lifecycle/drift, secret scans, Terraform-delete zero proof, rollback, and settings restoration.
- `_bmad-output/planning-artifacts/provider-ecosystem-coverage-audit.json` plus generated Markdown -- promote only six Story 31.0e rows after executed proof; expected inventory becomes 109 resources and 95 data sources.
- `_bmad-output/implementation-artifacts/{31-0e-wireguard-field-and-settings-parity.md,sprint-status.yaml}` and planning support/roadmap files -- synchronize evidence-bound status and remaining-story counts only after final proof.
- OPNsense stable/26.7 WireGuard controllers/models and pinned `browningluke/opnsense` v0.26.0 commit `33c2a03c2bc1a7339cb5aa5b611e15014e27f7ba` -- read-only evidence for endpoints, wire keys, defaults, validators, and singleton behavior; do not vendor.

## Tasks & Acceptance

### Review repair 1 — required behavior and proof

- Omitted Optional+Computed numeric fields must resolve documented API defaults/unset semantics without treating configured unresolved intent as an empty clear. Distinguish Config null from configured unknown; exercise minimal peer/server Core create, unrelated update and no-change, including omitted server public_key. Preserve authoritative generated key/instance on updates.
- On confirmed saved Update followed by reconfigure failure, preserve the newly committed planned PSK/private key in state. For genuinely ambiguous secret outcomes preserve recoverable identity plus explicit private reconciliation metadata; do not silently claim old or new key as confirmed or repeat a possibly saved mutation. On a recovered pending Update whose current plan differs, diagnose a required retry instead of returning false success.
- Any non-validation settings save failure must retain an ambiguous pending marker even if immediate GET fails; subsequent refresh reconciles/activates without another save. Add engine/protocol-backed persistence/retry tests, as well as direct fault-injected peer/server/settings Update and Delete cases.
- Exercise actual registered version0 state callbacks and engine/protocol upgrade of persisted scalar state, stable identities/secrets/CSV/ports/description and no-replacement convergence. Do not require end users to edit state.
- Replace helper-only serialization proof with concurrent resource entry points; handshake must catch removal of an owner lock, show no competing traffic while first mutation/recovery owns lock, and allow progress after release.
- Bind runner API URI TLS certificate SHA256 to a fingerprint independently obtained by the trusted identity probe through the exact VM's QEMU forwarding. Probe returns name, uuid and tls_sha256; runner must reject missing/malformed/mismatched fingerprint before credentialed API traffic. Root's probe verifies live name/UUID/isolation and disposable VM HOST_FORWARD ownership before obtaining certificate on node loopback18443. Add non-mutating preflight negative tests.
- Extend digest to transitive provider build/source files and runner dependencies, including logging library; retained evidence and review patch are excluded to avoid recursive hashing. Verify helper changes alter the digest.
- Add explicit before/after configured peer servers migration to server peers; capture/import and preserve all existing memberships. Clarify core introduction24.1 versus acceptance-validated26.7.7; no invented older-version or OpenTofu execution claims. Independently compare material raw API fields as well as shared resource/data-source state.
- KEEP all successful advanced17-step lifecycle, strict decoding, API-assigned omitted instance, authoritative peer-update association passthrough, exact empty-list sentinel, scalar upgrade behavior, singleton disabled-stateCoreUpdate, exactrestore, state-onlydestroy, immediateTerraform-delete/finalcleanup zeros, secretscan, six-row audit and narrow scanner fingerprints. Preserve PR62 planning content.
- The saved attempted candidate is `31-0e-review-pass1-attempt.patch`; use as a reference for positive preservation while re-deriving corrected implementation. It is not new intent or an approved final implementation. Do not include this reference patch in the published commit.

The prior candidate is already implemented and locally green. Preserve it; this continuation repairs the proven create payload and live harness gaps, completes outstanding proof, then reconciles only evidence-backed claims. Do not restart completed parity work.

**Execution:**
- `internal/service/wireguard/server_model.go` and request/lifecycle tests -- correct create-instance omission from stable source and live evidence; preserve the other verified contracts.
- `internal/service/wireguard/wireguard_parity_acceptance_test.go` plus runner -- exercise real Terraform Core singleton Update without ever enabling the appliance daemon: in a test-only temporary Terraform workspace, import the real disabled singleton, inject stale enabled=true into its prior state fixture, then plan/apply enabled=false with refresh disabled for this one controlled fixture transition. All API writes/read-back/reconfigure must be real; immediately verify API enabled=0, exact settings restoration and state-only destroy with a pre-destroy snapshot. Keep state-fixture manipulation isolated to the harness; users still get normal imports/no manual state-edit requirements. A direct resource.Update unit call alone is not this live proof.
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

- 2026-10-20 review repair1: verified optional-field planning, saved-secret recovery, ambiguous singleton activation, target binding and proof gaps. Amended execution/proof requirements outside the immutable intent. Avoid stale prior secrets, lost pending markers, unknown-as-clear, and unbound endpoint attestation. KEEP the successful advanced lifecycle, strict APIs, association preservation, disabled singleton, exact cleanup/restore and PR62 planning.

- 2026-10-20: This new planning revision supersedes the blocked original generated spec only where stable 26.7.7 disproved explicit-empty create serialization. Original contract remains unchanged in the historical spec. The bounded omitted-instance control returned saved plus API-assigned instance=0; explicit instance="" failed validation. KEEP typed state migration, secret boundaries, owner-locked recovery, import-first singleton, disabled daemon and all six-row proof requirements. This is an API-contract correction under Matthew's approved provider work, not a new production authorization.

## Review Triage Log

### 2026-10-20 — Review pass 1
- verdicts: 25 findings — high 9, medium 11, low 1, false 4, maybe-false 0
- findings:
  - `[high]` `[bad_spec]` Blind1: omitted optional/computed numerics fail normal apply — Schema gives omitted values unknown; optionalRange rejects unknown. Minimal Core fixtures must prove omitted create, unrelated update and no-change.
  - `[false]` `[reject]` Blind2: unknown server public key silently clears configured intent — Optional+Computed omission is intentionally API-computed; Terraform resolves configured references before apply. No reachable unresolved configured-key mutation was demonstrated. Minimal omitted-key proof is added under Blind10, not evidence this claim is true.
  - `[high]` `[bad_spec]` Blind3: confirmed-save key rotation retains old secrets — Update returns MutationReconfigureError after successful ParseMutationResponse; owners store prior for every non-validation error. Blank API reads then preserve the obsolete key.
  - `[medium]` `[patch]` Blind4: pending Update reports success against a different current plan — Both owner pending branches return recovered prior without reconciling plan. A no-refresh changed plan can reach this branch; return actionable reconciliation/retry diagnostic rather than success if current intent differs.
  - `[high]` `[bad_spec]` Blind5: ambiguous settings save plus failed GET loses marker — settings.Update sets pending only when reconciliation GET succeeds and matches; failed GET leaves no marker despite possible committed save.
  - `[medium]` `[bad_spec]` Blind6: digest omits execution inputs — Sourced test/scripts/lib/log.sh is absent from explicit_scope, and compiled provider packages outside listed files can change execution without changing digest. Cover transitive source/build/runner inputs.
  - `[high]` `[bad_spec]` Blind7: target URI is not bound to disposable identity — Runner compares name/UUID from independent probe but never compares its API endpoint. Add TLS fingerprint binding to VM-owned QEMU forwarding; root probe now attests that fingerprint over trusted SSH.
  - `[medium]` `[patch]` Blind8: writable peer servers migration is undocumented — Computed-only schema rejects legacy configured servers; narrative only describes sets and ownership. Add concrete association-preserving before/after import guidance.
  - `[medium]` `[bad_spec]` Blind9: legacy migration bypasses Core upgrade — Direct helper tests do not exercise registered version0 callback; current acceptance imports fresh schema. Add real protocol/Core upgrade no-replacement proof.
  - `[medium]` `[bad_spec]` Blind10: live fixtures omit computed/default paths — Live fixture supplies every numeric and server public_key. Add minimal lifecycle fixtures to exercise documented default/computed behavior; same root as Blind1.
  - `[low]` `[patch]` Blind11: core-introduction date can imply broader validation — Descriptions say manages 24.1+ while proof is 26.7.7. Directly distinguish core introduction from tested contract; do not invent older-version validation.
  - `[medium]` `[patch]` Gap1: upgrade registration unprotected — Pre-verified gap: changing registration map key0 to1 escapes direct-helper and fresh-state tests. Exercise both actual registrations with persisted v0 fixture.
  - `[medium]` `[patch]` Gap2: delete recovery unprotected — Pre-verified gap: both pendingDeleteUnknown branches lack failure-path tests. Exercise surviving remote object, authoritative absence, failed read-back and activation for both owners.
  - `[high]` `[bad_spec]` Edge1: omitted computed numeric fields reject valid plans — Same verified schema/optionalRange defect as Blind1; grouped in coherent optional-field plan repair.
  - `[high]` `[bad_spec]` Edge2: settings save/read failure loses pending activation — Same verified settings.Update branch as Blind5.
  - `[high]` `[bad_spec]` Edge3: rotated saved secret state regresses — Same confirmed-save MutationReconfigureError and prior-state defect as Blind3.
  - `[medium]` `[bad_spec]` IntentA: engine-persisted failure recovery remains unproved — Direct resource Create/Read test manually passes Private objects; no engine-mediated failed mutation/retry proof. Add protocol/Core persistence boundary tests without requiring every malformed response against live firmware.
  - `[high]` `[bad_spec]` IntentB1: saved secret represented by obsolete prior state — Same verified confirmed-save rotation defect as Blind3.
  - `[high]` `[bad_spec]` IntentB2: ambiguous settings save can lose activation marker — Same verified failed reconciliation GET defect as Blind5.
  - `[medium]` `[bad_spec]` IntentC: migration helper is not engine migration — Same verified registered-callback/Core coverage gap as Blind9 and Gap1.
  - `[medium]` `[bad_spec]` IntentD: helper locking test cannot catch missing resource owner lock — Concurrency test acquires ownerMu manually; removing an owner method lock does not change test. Add handler-level interleaving with request handshakes.
  - `[medium]` `[patch]` IntentE: shared resource/data-source decoding is not independent API oracle — Both fromAPI paths share decoder; add direct API fixture checks for material fields and omitted computed-key path. Current Core convergence checks remain useful but not independent oracle.
  - `[false]` `[reject]` IntentF: singleton Update uses stale state rather than enabling daemon — Intent explicitly requires disabled daemon and real Update; isolated stale-state fixture with no-refresh invokes real set/get/reconfigure and satisfies the disabled-state transition. Evidence states this accurately.
  - `[false]` `[reject]` IntentG: compatibility/completion evidence is bounded — Evidence explicitly names Terraform Core and26.7.7, not executed OpenTofu or older-version acceptance. Core24.1 origin is historical, not proof of all versions; clarification recorded under Blind11.
  - `[false]` `[reject]` IntentH: planning and scanner files exceed isolated WireGuard intent — Matthew approved broader release/backlog sequence; PR62 already merged these planning files. Four scanner fingerprints identify deterministic historical Unbound state-key names, not credentials; full-history scanner remains enabled. No additional WireGuard validation claim is made for those files.
- Route: coherent bad_spec repair iteration1; lower patch entries are retained as KEEP/repair requirements, not independently applied. Saved attempted diff: `31-0e-review-pass1-attempt.patch`.


### 2026-10-20 — Review pass 2
- Execution note: native agent-thread limit prevented fresh contexts for all four layers. One fresh independent blind reviewer and the retained independent intent reviewer completed blind/intent first, then edge/verification roles sequentially. Neither was the implementer. All four layer outputs were collected before triage; this is not represented as four fresh reviewers.
- verdicts: 27 findings — high 0, medium 15, low 1, false 11, maybe-false 0
- findings:
  - `[medium]` `[patch]` Blind1: configured unknown server public key is flattened — toAPI skips validation for unknown and ValueString produces empty; add Config-aware unknown rejection while retaining omitted computed API-truth semantics. Applied: bounded correction and targeted regression passed.
  - `[medium]` `[patch]` Blind2: configured unknown secret rotation reuses prior key — Update unconditionally substitutes prior on unknown plan; Config-aware guard can distinguish unresolved configured rotation from computed omission. Applied: bounded correction and targeted regression passed.
  - `[medium]` `[patch]` Blind3: scalar CSV set recovery is order-sensitive — createOwned directly compares scalar strings although strictSelectedMapList accepts CSV and models are sets. Normalize known set-valued wire fields before exact recovery comparison; reject malformed members. Applied: bounded correction and targeted regression passed.
  - `[false]` `[reject]` Blind4: recovered Create cannot confirm secret without echo — Preflight excludes existing same-name objects; serialized Add creates a uniquely new matching identity under owner lock. Atomic validated creation proves its payload, unlike ambiguous Update of an already existing object. No competing writer is allowed by ownership contract; missing echo is expected write-only behavior, not uncertainty about which existing save ran.
  - `[medium]` `[patch]` Blind5: malformed returned secret replaces confirmed state — fromAPI assigns any nonempty API PSK/private key without existing key validation. Add validation before assigning, with value-free diagnostics and regression preserving prior state. Applied: bounded correction and targeted regression passed.
  - `[medium]` `[patch]` Blind6: imported-peer PSK default is not explicit in migration guide — Generated schema says blank disables, but association-import guide does not tell operators to securely retain/configure existing PSK; omission defaults blank and a later update clears it. Add direct migration note. Applied: bounded correction and targeted regression passed.
  - `[low]` `[patch]` Blind7: composition does not show server public-key supply — API may leave optional public key blank. Clarify obtaining matching wg pubkey and show use in example without suggesting automatic derivation. Applied: bounded correction and targeted regression passed.
  - `[medium]` `[patch]` Blind8: scan operational failure can be reported as no match — if grep treats status2 like status1. Distinguish no-match1 from operational errors before printing artifacts or claiming scan pass; test fail-closed status2. Applied: bounded correction and targeted regression passed.
  - `[false]` `[reject]` Blind9: preflight attestation lacks continuous transport pinning — Claim accurately observes separate connections but does not demonstrate a bad outcome in the supported runtime: fixed root-owned loopback/QEMU forwarding, isolated VM, unique disposable credentials and no route/endpoint mutation during proof. Evidence claims preflight identity checks, not general adversarial transport pinning. Adding a continuous TLS policy for hypothetical endpoint replacement is not required by this bounded acceptance contract.
  - `[medium]` `[patch]` Blind10: multi-member association regression is absent — Current live fixture has one association and protocol fixture returns fixed two-member state without validating Update payload. Add HTTP-backed multiple backreference/peer-membership payload assertions; no claim that multiple live appliances are required by intent. Applied: bounded correction and targeted regression passed.
  - `[false]` `[reject]` IntentA: protocol replay differs from all-engine failure proof — Selected layered reading is implemented: actual protocol persistence/restart boundaries plus real Terraform success lifecycle. Intent does not require every injected failure through the CLI; no lost state or repeated save is demonstrated.
  - `[false]` `[reject]` IntentB: constructed v0 protocol upgrade differs from old-provider CLI fixture — Actual registered UpgradeResourceState, exact members/scalars, in-place Apply/Read and no-change/no-replacement Plan exercise the stated upgrade contract. A separately old-provider CLI fixture is not required and no migration failure is shown.
  - `[false]` `[reject]` IntentC: ambiguous key uses null public value with retained private candidates — Confirmed-knowledge preservation is explicit and tested; both candidates/identity remain private, no silent loss or replay. This satisfies retain identity/confirmed state rather than falsely asserting an old or new key.
  - `[false]` `[reject]` IntentD: independent raw API comparison occurs at selected checkpoints — Raw API oracle plus Core convergence and broader resource/data-source comparisons meet layered proof. No incorrect field mapping or missing intended surface is demonstrated.
  - `[false]` `[reject]` IntentE: disabled singleton uses stale-state fixture — Carried: immutable intent requires daemon remain disabled and real Core Update; fixture invokes actual Update with enabled0 and exact restore, as documented.
  - `[false]` `[reject]` IntentF: compatibility/promotion evidence bounded — Carried: documentation distinguishes24.1 origin,26.7.7 proof and unexecuted OpenTofu; root observed actual live proof/order separately. No blanket compatibility or release claim.
  - `[false]` `[reject]` IntentG: planning/scanner changes exceed narrow WG contract — Carried: Matthew authorized broader release/backlog sequence and PR62 merged planning. Exact historical scanner-name fingerprints are noncredential false positives; rules/history retained.
  - `[medium]` `[patch]` Edge1: configured unknown public key — Same verified unknown/config distinction as Blind1; shared fix and regression. Applied: bounded correction and targeted regression passed.
  - `[medium]` `[patch]` Edge2: unknown configured secret rotation — Same verified prior-fallback guard gap as Blind2; shared fix and regression. Applied: bounded correction and targeted regression passed.
  - `[medium]` `[patch]` Edge3: scalar CSV ordering strands recovery — Same accepted CSV/set mismatch as Blind3; shared normalization fix. Applied: bounded correction and targeted regression passed.
  - `[false]` `[reject]` Edge4: missing secret echo makes recovered Create unconfirmed — Same refutation as Blind4: new identity absent before request plus exact readable payload under owner lock establishes atomic creation, unlike unknown Update of existing object.
  - `[medium]` `[patch]` Edge5: malformed nonblank API secrets accepted — Same verified unchecked assignment as Blind5; add existing validator before assignment. Applied: bounded correction and targeted regression passed.
  - `[medium]` `[patch]` Edge6: grep operational errors fail open — Same verified exit-status branch as Blind8; fail closed for any status other than1. Applied: bounded correction and targeted regression passed.
  - `[false]` `[reject]` Edge7: endpoint replaced after preflight — Same bounded-runtime refutation as Blind9. No endpoint or forwarding mutation is part of proof and no authenticated wrong-target write is shown; not a general TLS pinning guarantee.
  - `[medium]` `[patch]` Edge8: imported peer omits existing PSK — Same verified default-empty write and migration-note gap as Blind6. Applied: bounded correction and targeted regression passed.
  - `[medium]` `[patch]` Gap1: multiple exact create matches not tested — Pre-verified gap: changing cardinality guard to reject only zero escapes existing tests. Add zero/multiple-candidate cases asserting no UUID adoption and ambiguity diagnostics. Applied: bounded correction and targeted regression passed.
  - `[medium]` `[patch]` Gap2: authoritative secret journal clear transition untested — Pre-verified gap: removing journal clear escapes blank-only uncertainty tests. Add nonblank confirmation then blank refresh, assert journal cleared/confirmed key retained and no repeated save, for both owners. Applied: bounded correction and targeted regression passed.

## Design Notes

Released WireGuard uses CSV requests and selected-map responses for multi-value fields. Terraform sets provide stable order-insensitive state, but existing scalar state must be upgraded explicitly. The official peer `servers` value is derived from server membership updates; the provider therefore treats server `peers` as the writable association and peer `servers` as the authoritative backreference rather than allowing two competing writers.

General settings are an import-first singleton because the appliance object already exists. Its public identity is `wireguard_settings`; Create rejects with import guidance, Update sets only `enabled`, and Delete is state-only. WireGuard activation differs from Kea: success is `{"result":"ok"}`, so strictness must be owner-selective rather than changing the established Kea `status=ok` contract.

## Verification

Current disposable runtime is root-owned at `/private/tmp/story31e-runtime.7b5ubq6d/acceptance.env` (mode600; do not print values); its executable `identity-probe` reads live Proxmox metadata. Expected VM name is `disposable acceptance guest`, UUID `d630950e-d4dd-4df0-9fcb-fac6b086bded`, core26.7.7. For Docker use the same root-owned tunnel as `https://host.docker.internal:18444`; host-side requests use `https://127.0.0.1:18444`. Root retains provisioning/tunnel/retirement ownership; implementation owns exact-digest test execution and sanitized evidence. If these runtime references stop existing, ask parent for replacement setup rather than targeting production. Ephemeral credentials belong only in the runtime directory, never source/docs/logs.

**Commands:**
- `go test -count=1 ./internal/service/wireguard/... ./pkg/opnsense/... ./internal/tfconv/...` -- expected: schema migration, encoding/decoding, lifecycle/recovery, singleton, concurrency, and secret contracts pass.
- `go generate ./... && go generate ./... && git diff --exit-code` -- expected: generated docs/audit output is deterministic and current.
- `go test -count=1 ./internal/coverageaudit && bash test/scripts/validate-structure.sh && git diff --check` -- expected: exact registration/audit counts, structure, and whitespace pass.
- `OPNSENSE_WIREGUARD_PARITY=1 test/scripts/run-story-31-0e-acceptance.sh` with externally attested disposable credentials -- expected: non-skipped peer/server/settings lifecycle, drift repair, state-only destroy, exact restore, secret scan, and zero leftovers on the expected digest.
- `DEVRAIL_IMAGE=ghcr.io/devrail-dev/dev-toolchain:1.12.24 make check` -- expected: lint, format, tests, security, scan, and docs all pass.

## Auto Run Result

- Implemented typed WireGuard peer/server parity and scalar-state upgrades, strict recovery/decoding, resource-only Sensitive keys, import-first settings, and six audited rows with109 resources/95 data sources. Preserved PR62 release/backlog planning.
- Files: WireGuard models/schemas/resources/data sources/lifecycle/state upgrade and registration; shared owner-selective reconfigure options; controlled/protocol/live tests and digest-bound runners; templates/generated docs/examples; audit and story/support/roadmap metadata. Historical scanner maintenance remains four exact noncredential state-name fingerprints, with no rule/history weakening.
- Review: pass1 logged25 findings and triggered one coherent spec repair outside immutable intent. Pass2 logged27 findings:15medium,1low,11false. Applied9 root-cause patch groups (8medium,1low); no deferred items. Every rejected finding and refutation remains in its per-finding log. No high patch group remains.
- Follow-up review recommended: true. This is the first post-rederivation patch pass and it patched multiple medium groups. Specific unverified review risk: interactions between newly Config-aware unknown-key guards, ambiguous-create set normalization/cardinality, and private secret reconciliation have targeted tests but have not received another fresh-context independent review after the bounded patch. Native thread capacity required independent reviewer reuse; maintainer PR review should focus on those paths. This recommendation is not an assertion of a failing test or production authorization.
- Verification: parent focused tests, double deterministic generation, audit/structure/whitespace and the earlier repaired full6gate/21step proof passed; bounded patch tests, WireGuard lint, runner shellcheck, examples formatting and whitespace passed. Final frozen publication-tree full checks and exact-digest live results are recorded in [the separate evidence](31-0e-wireguard-live-evidence.md) after execution. This spec is included in the digest; do not edit covered metadata after that final proof without rebinding it.
- Matrix audit: seven rows map to executed focused/protocol tests plus non-skipped21step live acceptance. Exact named test output and final command outcomes are retained by the parent; no basic acceptance tests or unexecuted OpenTofu/older-firmware paths are claimed.
- Residual constraints: API-permanently-unreadable ambiguous key rotations cannot auto-confirm; explicit secure same-UUID reconciliation/re-import is documented. WireGuard core introduction24.1 is not blanket earlier-version proof. Source/history reference attempt patch is retained locally only and is intentionally not a published artifact. Root owns disposable retirement after proof. Provider merge/release and production adoption are separate gates.
