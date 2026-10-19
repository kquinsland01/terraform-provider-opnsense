---
title: 'Story 31.0d: Unbound Settings and Forward Parity'
type: 'feature'
created: '2026-10-19'
status: 'review'
baseline_revision: '0ec00231e0938d7697ffee53c0a60eff730bd631'
review_loop_iteration: 2
followup_review_recommended: false
context:
  - '_bmad-output/implementation-artifacts/epic-31-context.md'
warnings:
  - multiple-goals
  - oversized
deferred: []
---

<intent-contract>

## Intent

**Problem:** The provider cannot losslessly adopt the complete stable Unbound settings surface, and its existing domain-override owner cannot represent query and DNS-over-TLS forwards with their port and certificate-name semantics. The audit therefore leaves four buildable settings/forward rows open and brownfield plans can omit or overwrite real appliance configuration.

**Approach:** Extend the existing non-overlapping owners: make `opnsense_unbound_general` own general plus the previously unowned advanced, ACL-default, and forwarding branches; keep `opnsense_unbound_dnsbl` as the DNSBL owner; add a complete read-only `opnsense_unbound_settings` view; and extend `opnsense_unbound_domain_override` resource/data source additively for query and DoT. Use the exact OPNsense 26.7.7 wire contract, preserve legacy HCL/state, and promote audit rows only after digest-bound disposable-appliance proof.

## Boundaries & Constraints

**Always:** Preserve the existing resource names and established attributes; keep exactly one writable owner per `unbound` branch; submit branch-scoped settings payloads so unrelated hosts, aliases, ACL items, DNSBL blocklists, and forwards survive; map public forward `type=query|dot` to wire/endpoint `forward|dot`, `server_port` to `port`, and `verify_cn` to `verify`; route add/set through the matching forward or dot endpoint because the endpoint overrides body type; treat missing-item `[]` as absent but malformed/ambiguous responses as errors; require affirmative `saved`/`deleted` and `status=ok`; preserve identity and pending activation after committed-but-unreconfigured mutations; make singleton destroy state-only; snapshot and exactly restore every touched singleton value in live tests.

**Never:** Add a second monolithic writable owner over fields already managed by general/DNSBL/domain-override resources; rename `server`, silently repurpose existing state, or copy the baseline provider's obsolete DNSBL wire shape; send full GET payloads back to `/set`; treat HTTP success, object existence, an empty monad, or a failed reconfigure as mutation success; require `verify_cn` for DoT because OPNsense explicitly supports unverified/self-signed TLS; globally version-gate legacy general/forward operations; reset appliance settings on Terraform destroy; promote audit/support claims before exact live proof and zero-leftover cleanup.

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
|----------|---------------|---------------------------|----------------|
| Existing settings import | Non-default general, advanced/cache/log/privacy/serve-expired, ACL-default, forwarding, and DNSBL values | Complete read-only view and writable owners agree with authoritative GET; matching plan is empty and no branch overlaps | Malformed field/branch diagnoses without replacing prior usable state |
| Branch-scoped settings update | One owner changes populated, explicit-empty, or omitted values | POST contains only that owner's branch, reconfigure runs once, siblings are preserved | Unknown values are not clears; saved-but-unreconfigured state remains recoverable |
| Singleton destroy | Imported or configured singleton leaves Terraform | State is removed and upstream settings remain byte/field equivalent | No reset POST is sent |
| Query forward | `type=query`, explicit/default port | Uses `add_forward`/`set_forward`; full resource/data-source state round-trips | Wrong/missing monad or non-affirmative result is diagnostic |
| DoT forward | `type=dot`, port, optional/blank `verify_cn` | Uses `add_dot`/`set_dot`; GET/import/drift preserve type, port, CN | No fallback to query endpoint; blank CN remains valid |
| Forward type change | Existing query becomes DoT or reverse | Endpoint-correct update preserves UUID when upstream permits and read-back proves desired type | Pre-read prevents silent upsert/adoption; ambiguity preserves recovery state |
| Missing/ambiguous forward | GET returns `[]`, `{}`, transport/auth/server error, or malformed object | Only `[]` removes state; all other cases retain identity and diagnose | Never collapse failure into absence |
| Delete/reconfigure split | Delete saves, then reconfigure fails | Next lifecycle retries activation and only then removes state | State is not forgotten on ambiguous delete/GET |

</intent-contract>

## Code Map

- `internal/generate/schemas/unbound.yaml:1` -- generated source for existing general/DNSBL singleton models; extend only branch-safe fields supported by the generator and never edit `.gen.go` outputs directly.
- `internal/generate/main.go:157` -- singleton data sources are intentionally not generated; the complete settings view and any multi-branch owner need handwritten implementations or a deliberate generator extension.
- `internal/service/unbound/general_*` and `dnsbl_*` -- existing fixed-ID singleton owners with state-only Delete; extend general across its four owned branches, preserve public names/import IDs, and document the legacy `no_aaaa_records` spelling as the upstream `noarecords` control without adding a second writable alias.
- `internal/service/unbound/domain_override_{schema,model,resource,data_source}.go` -- existing forward owner; add type/port/CN and source-backed strict lifecycle handling without renaming `server` or dropping local `description`.
- `internal/service/unbound/exports.go:13` and `internal/provider/provider.go:237` -- register the new read-only settings view through the existing package boundary while retaining the current writable owner set.
- `pkg/opnsense/{crud,reqopts,reconfigure}.go` -- reuse strict missing/delete and affirmative-result controls; do not broaden generic behavior without proving unrelated owners.
- `internal/service/openvpn/{pending_activation,instance_resource}.go` -- reuse the proven private-state pending-activation and three-state recovery pattern for settings/forward mutations.
- `internal/service/haproxy/frontend_state_upgrade.go:14` -- only local state-upgrade precedent; avoid cross-type migration by extending existing names additively.
- `internal/service/unbound/*_test.go` and `internal/acctest/` -- add field-complete schemas/models, request/response strictness, no-overlap, recovery, import/no-diff/drift, query/DoT routing, singleton preservation, and cleanup proof.
- `test/scripts/run-story-31-0c-acceptance.sh` and its digest/evidence pattern -- reuse target identity, exact-source digest, required PASS, parsed cleanup, and sanitized evidence mechanics for Story 31.0d.
- `templates/`, `examples/`, and generated `docs/` -- document the split ownership model, complete settings view, state-only singleton destroy, query/DoT behavior, and insecure blank-CN choice.
- `_bmad-output/planning-artifacts/provider-ecosystem-coverage-audit.{json,md}` -- four canonical missing rows: resource/data-source `unbound_forward` and `unbound_settings`; promote only from final row-specific proof.
- `_bmad-output/planning-artifacts/{support-matrix,resource-gap-verification,core-config-gap-analysis,feature-complete-roadmap}.md` and `_bmad-output/implementation-artifacts/sprint-status.yaml` -- reconcile counts/status after evidence; mark merged 31.0c done and 31.0d active/review accurately.
- OPNsense core `f1a83f72b144469fd97839aba2419ad130d19043` `Unbound.xml`, `SettingsController.php`, `ServiceController.php`, `dot.conf`, `advanced.conf`, and `access_lists.conf` -- read-only authority for 26.7.7 fields, roots, endpoint-selected type, defaults, and reconfigure semantics.
- Baseline `browningluke/opnsense` v0.26.0 at `33c2a03c2bc1a7339cb5aa5b611e15014e27f7ba` -- semantic inventory only; its legacy DNSBL shape/defaults are not authoritative for 26.7.7.

## Tasks & Acceptance

**Execution:**
- `internal/generate/schemas/unbound.yaml` and `internal/service/unbound/` -- extend the existing general owner across general/advanced/ACL-default/forwarding, retain DNSBL ownership, add the complete settings view, and implement compatibility-safe decoding, field validation, branch-scoped requests, state-only destroy, and recoverable activation.
- `internal/service/unbound/domain_override_*.go` -- add query/DoT type, port, and CN with endpoint-correct CRUD, strict absence/result handling, import stability, and pending activation.
- `internal/service/unbound/*_test.go`, provider registration tests, and acceptance harness -- prove every matrix row, exact HTTP contracts/call counts, all audited fields at non-default values, pre-existing import, no-diff, drift/repair, recovery, sibling preservation, and both search namespaces cleaned to zero.
- `templates/`, `examples/`, `docs/`, audit/status/roadmap artifacts, and Story 31.0d evidence -- regenerate deterministically and reconcile only what the executed proof establishes.

**Acceptance Criteria:**
- Given pre-existing OPNsense 26.7.7 Unbound settings, when each writable singleton is imported and the complete settings data source is read, then every audited value is represented by exactly one writable owner, authoritative states agree, and a matching plan is empty.
- Given a change in each settings branch, when Terraform applies and refreshes, then only that branch changes, the required reconfigure action succeeds exactly once, unrelated Unbound configuration is preserved, and external drift is detected and repaired.
- Given singleton resources in state, when Terraform destroys them, then state is removed without a settings reset request and the upstream snapshot is unchanged.
- Given query and DoT forwards, when create/read/update/import/no-diff/drift/delete lifecycles run, then type, port, certificate name, and existing fields round-trip through the correct endpoint family and resource/data-source states are field-complete.
- Given a mutation commits but activation/read-back fails, when a subsequent apply runs, then activation is retried without duplicate creation, silent upsert, skipped requested update, or forgotten deletion.
- Given the final digest-bound disposable-appliance run, when required tests and cleanup finish, then settings restoration is exact, both query and DoT searches report zero tagged leftovers, evidence is sanitized, and only the four proven audit rows are promoted.

## Spec Change Log

- `2026-10-19` — Convergence pass 7 revoked digest `450ee309b006f469d5c0a929ac27ecf4b97bd6064440c843590a2c20bbd6ba4f` for publication. The forward and DNSBL serialization tests inferred the second lock attempt from a timed sleep instead of a positive handshake, and the forward test asserted only a request prefix. Proof artifacts were removed and exactly four Story 31.0d rows returned to `missing-buildable` pending Repair 7 proof.
- `2026-10-19` — Convergence pass 6 revoked digest `ec5d2ba170d7b5ace670d3226ba557cd2bd204d9f725ba9e19756813320fa83d` for publication. Missing pagination members and inconsistent page metadata could still be accepted as authoritative zero, the negative-total branch lacked its required direct test, and the concurrency suite did not prove forward recovery or DNSBL recovery remained inside the owner-level critical section. Proof artifacts were removed and exactly four Story 31.0d rows returned to `missing-buildable` pending Repair 6 proof.
- `2026-10-19` — Convergence pass 5 revoked digest `564647060091c60762532d8c6fdf0c7b31ce14e600a4439b000f6c33a42ae3cc` for publication. Its exact-tree and live assertions passed, but malformed pagination could be accepted as authoritative zero, create preflight/mutation/recovery was not atomic across concurrent Terraform creates, the attestation omitted shared-package test inputs, and the older-appliance forward Create test did not inspect its request body. The proof artifacts were removed and exactly four Story 31.0d rows returned to `missing-buildable` pending Repair 5 proof.
- `2026-10-19` — Convergence pass 4 revoked digest `fd82db095c2bcc97660e6081737b0d87bb1a920f4b6eedf3be67beb3776596ef` for publication. The live run's executed lifecycle claims trace correctly, but ambiguous mutation responses were misclassified as deterministic validation failures, rollback cleanup could mask failed Terraform forward deletion, older-appliance Create/Update and rejection branches lacked direct contract tests, and audit/count bookkeeping remained inaccurate. The sanitized proof artifacts were removed and exactly four Story 31.0d rows returned to `missing-buildable` pending Repair 4 proof.
- `2026-10-19` — Repair 4 passed focused/full tests, deterministic generation, diff hygiene, all six DevRail checks, and replacement OPNsense 26.7.7 acceptance. The run non-mutatingly proves Terraform's own query/DoT deletes left zero tagged objects before rollback cleanup. Exactly four audit rows are reconciled; publication remains gated on the promoted-tree rerun and fresh four-layer convergence.
- `2026-10-19` — Convergence pass 3 revoked digest `8f755959915da4dbebdd134ce162a4b53a4de15a955445ee258a82e9b34d7d87` for publication. The exact-digest live run passed its implemented assertions, but those assertions did not execute independent DoT create/delete or a Terraform DNSBL update, while code review found current-port defaulting, legacy-delete option selection, and acceptance setup rollback defects. The sanitized proof artifacts were removed and exactly four Story 31.0d audit rows returned to `missing-buildable` pending Repair 3 proof.
- `2026-10-19` — Repair 3 passed focused/full tests, deterministic generation, diff hygiene, all six DevRail checks, and replacement OPNsense 26.7.7 acceptance. The executed run includes independent DoT add/delete, DNSBL Terraform update/drift/repair, exact restoration, and zero leftovers. Evidence and exactly four audit rows are reconciled; final publication remains gated on a fresh convergence pass.
- `2026-10-19` — Review pass 2 revoked digest `58c5991756acd457e3a3635fcf3c394438562d46529d1ed680a7db35b43d833a` for publication. Although its local gates and disposable-appliance run passed, the proof did not exercise a Terraform mutation in every owned settings branch or compare every aggregate DNSBL field, and the implementation still violated compatibility, branch-scoping, identity, pending-activation, and owner-selective reconfigure contracts. The sanitized proof artifacts were removed and all four Story 31.0d audit rows returned to `missing-buildable` pending replacement proof.
- `2026-10-19` — Repair 2 passed focused/full tests, deterministic generation, diff hygiene, all six DevRail checks, and replacement OPNsense 26.7.7 acceptance with exact settings restoration and zero DNSBL/query/DoT leftovers. The final digest is recorded only in the out-of-scope evidence artifact to avoid self-referential attestation. Evidence and exactly four audit rows were reconciled; final publication remains gated on convergence pass 3.
- `2026-10-19` — Rejected and removed the earlier live evidence because its digest and acceptance scope did not cover the complete Repair-1 contract. The four audit rows remain missing-buildable pending replacement proof.
- `2026-10-19` — Review pass 1 exposed incomplete recovery, strict-decoding, legacy-state, digest, and acceptance-proof contracts. Repair 1 must avoid the known-bad façade that can orphan a committed DNSBL item, persist unknown Optional+Computed values, compare omitted values as zeroes, adopt an arbitrary/mismatched legacy blocklist, accept malformed authoritative fields, or promote evidence broader than executed. KEEP: the 26.7.7 field/endpoint map, non-overlapping public owners, query/DoT endpoint routing and port defaults, typed settings data source, branch-scoped request model, state-only singleton deletion, sanitized digest-bound runner, and exact cleanup/restoration checks.

## Review Triage Log

### 2026-10-19 — Review pass
- verdicts: 29 findings — high 17, medium 5, low 4, false 2, maybe-false 1
- findings:
  - `[medium]` `[patch]` Pending-activation recovery during resource Read is untested for general, DNSBL, forward mutation, and forward deletion — verification-gap evidence found only mutation-method recovery tests; repair must add direct Read/private-state tests proving one reconfigure, marker clearing, refreshed state, and delete removal without a second mutation.
  - `[medium]` `[patch]` Ambiguous committed forward creation recovery has no exact-match/ambiguity test — `recoverDomainOverrideID` is otherwise unexercised; repair must prove the new UUID is adopted only for one exact post-snapshot match and never for zero/multiple matches.
  - `[high]` `[bad_spec]` Retaining the DNSBL public singleton while rebinding it to one blocklist item silently changes backing-object/cardinality semantics — repair must define deterministic migration/adoption from legacy state and never choose arbitrarily.
  - `[high]` `[bad_spec]` Live acceptance writes general settings before importing them — repair must create the complete brownfield fixture outside Terraform, import first, and then prove the matching plan is empty.
  - `[false]` `[reject]` The complete settings view must include unrelated item collections — the intent separates those item owners.
  - `[high]` `[bad_spec]` State-only singleton destroy was observed only after restoration — repair must observe upstream equality immediately after Terraform destroy.
  - `[high]` `[bad_spec]` DNSBL Create can commit then lose usable state on read-back failure — repair must retain identity and confirmed state.
  - `[high]` `[bad_spec]` General and DNSBL recovery paths write raw plans containing possible Optional+Computed unknowns — repair must persist confirmed authoritative state.
  - `[high]` `[bad_spec]` General ambiguous-outcome recovery compares omitted values as zeroes — compare only emitted mutation fields.
  - `[high]` `[bad_spec]` Legacy DNSBL state rejects every multi-item appliance — match prior state to exactly one item or diagnose zero/multiple matches.
  - `[medium]` `[patch]` DNSBL private-state diagnostics are not checked before mutation — add immediate guards.
  - `[high]` `[bad_spec]` DNSBL decoding accepts malformed booleans and selectors — add owner-local strict validation without changing provider-wide compatibility behavior.
  - `[high]` `[bad_spec]` The typed settings view accepts malformed DNSBL selectors — validate every current item field.
  - `[high]` `[bad_spec]` General decoding requires too few fields — require every pinned audited field and preserve prior usable state on malformed responses.
  - `[medium]` `[bad_spec]` General values lack range/choice validation — validate authoritative values to schema bounds and choices.
  - `[low]` `[patch]` `dnsbl_json` canonicalization loses large-number precision — use `UseNumber` and reject trailing JSON.
  - `[high]` `[bad_spec]` The attestation digest omits conversion helpers and promoted artifacts — bind all direct dependencies and claim inputs.
  - `[high]` `[bad_spec]` Evidence claims broader drift/destroy coverage than executed — strengthen tests or narrow claims.
  - `[high]` `[bad_spec]` Query-to-DoT is proven but DoT-to-query is absent — exercise both directions.
  - `[low]` `[patch]` Forward audit rows omit local-only `description` — record it.
  - `[medium]` `[patch]` Forward GET decoding accepts empty required domain/server — reject empty UUID/domain/server.
  - `[low]` `[reject]` External mutation races lack compare-and-swap support; strict pre/post reads remain the practical boundary.
  - `[maybe-false]` `[defer]` Mixed reconfigure success/failure fields need upstream evidence before generic behavior changes.
  - `[false]` `[reject]` General owned-branch concurrency is normal Terraform last-write-wins behavior.
  - `[low]` `[reject]` A concurrent external DNSBL add cannot be safely compensated without risking deletion of user-owned data.

## Repair-1 Requirements

- Preserve public names and passed field/endpoint work, but re-derive lifecycle recovery from authoritative remote state rather than raw plans.
- Add field-complete strict decoders for general, advanced, ACL-default, forwarding, DNSBL items, and forward required fields; validate booleans, integer ranges, choices, populated selected maps, UUIDs, and required strings before state changes.
- Define legacy DNSBL adoption explicitly: use existing Terraform state to match exactly one blocklist item, persist its private UUID, and diagnose zero/multiple matches without mutation. A clean import may adopt a sole item; ordinary create must not adopt existing objects.
- For committed create/update outcomes and post-mutation read failures, retain usable identity and only persist confirmed authoritative values. Omitted/unknown Optional+Computed values must never be converted into clears or zero-value equality comparisons.
- Expand the digest scope to every direct conversion/decoder dependency and all audit/status/evidence inputs whose claims are promoted.
- Strengthen acceptance to start from API-created non-default singleton and DNSBL fixtures, import before Terraform writes, prove immediate post-destroy singleton preservation, cover query→DoT→query, and keep evidence wording no broader than executed assertions.
- Add direct resource-Read recovery tests, ambiguous create exact/zero/multiple-match tests, strict malformed-response tests, legacy multi-item matching tests, and all review-identified direct guards.

### 2026-10-19 — Review pass 2
- verdicts: 14 deduplicated verified findings — high 7, medium 4, low 3; all require Repair 2 before publication
- findings:
  - `[high]` `[bad_spec]` The 24.1+ compatibility promise is not executable: current general, DNSBL, and query-forward reads require 26.7.7-only shapes. Repair must define selective capability gates so established general/query-forward behavior and legacy DNSBL state remain usable while new parity fields, the aggregate view, DoT, and current DNSBL item semantics require 26.7.7.
  - `[high]` `[patch]` General updates replay all four owned branches. Update requests must contain only branches whose configured values changed from prior state, with exact request tests and a live Terraform change in every branch.
  - `[high]` `[bad_spec]` The live proof changed only the advanced branch through Terraform. Repair must exercise general, advanced, ACL-default, and forwarding mutations independently and prove sibling preservation.
  - `[high]` `[patch]` Legacy DNSBL sole-item discovery adopts a mismatched object. Complete prior state may adopt only one exact item-field match; clean import/incomplete state may adopt a sole item; SafeSearch must not participate in item identity.
  - `[high]` `[patch]` Singleton Delete discards private pending activation. General and DNSBL destroy remain state-only but must retry a pending reconfigure before removing Terraform state.
  - `[medium]` `[patch]` Forward Update clears a pending-delete marker and then reads the deleted UUID. Successful pending-delete activation must transition to clean create semantics without losing recovery state.
  - `[low]` `[patch]` Forward private-state decoding accepts unknown markers and the setter is nil-unsafe. Accept only `mutation` or `delete`, diagnose unknown values, and make the setter nil-safe.
  - `[high]` `[bad_spec]` Aggregate settings proof checks four DNSBL fields only for non-emptiness and never compares `dnsbl_json`. Replacement proof must compare every typed DNSBL field against exact API fixture values and compare canonical JSON with the authoritative DNSBL branch.
  - `[medium]` `[evidence]` Forward evidence called a Terraform-created object brownfield. Create the fixture outside Terraform or describe it precisely as an isolated-state import of a persisted Terraform-created appliance object; do not claim external provenance.
  - `[high]` `[patch]` Strict reconfigure semantics were broadened endpoint-globally for unrelated Unbound owners. Add an owner-selective request option used by Story 31.0d owners while preserving existing unrelated caller behavior.
  - `[medium]` `[patch]` Strict general decoding does not validate serve-expired numeric strings or cache-size syntax. Derive exact formats/ranges from pinned OPNsense source and add malformed, negative, overflow, and boundary tests.
  - `[low]` `[patch]` Acceptance cleanup silently ignores invalid selected-map values. Validate every selection as exactly `0` or `1` and require exactly one selected option before restore.
  - `[low]` `[bookkeeping]` Support-matrix remaining-story counts must include 31.0d during repair and move to 31.0e–31.0j only after final promotion.
  - `[medium]` `[evidence]` Final settings/forward claims must be no broader than exact branch mutation, aggregate equality, and import provenance assertions executed by the replacement acceptance run.

## Repair-2 Requirements

- Compatibility matrix: established general attributes and query-forward CRUD/read/import remain supported on the documented practical 24.1+ floor; omitted additive response fields remain null/unmanaged and do not fail established-owner reads. New advanced/cache/logging/privacy/serve-expired parity fields, ACL-default and forwarding branches, the aggregate settings data source, DoT fields/endpoints, and current item-shaped DNSBL semantics require OPNsense 26.7.7. Older DNSBL appliances use the legacy singleton adapter. Older query-forward responses that omit type/port decode as query/default port 53, while current 26.7.7 responses remain strict. Gates are per configured/read surface and never global to unrelated legacy operations.
- General Update computes configured branch deltas against prior authoritative state and sends only changed `general`, `advanced`, `acls`, or `forwarding` branches. Exact unit tests and live assertions prove one-branch payloads, one reconfigure, and sibling preservation for all four branches.
- DNSBL discovery treats item identity independently from global SafeSearch. Complete legacy state adopts exactly one exact item-owned-field match; clean import or explicitly incomplete legacy state may adopt a sole item; zero, mismatch, and multiple matches diagnose without mutation.
- General and DNSBL Delete remain state-only, but a private pending activation must be retried and cleared before state removal. Forward Update distinguishes pending mutation from pending delete and transitions a successfully activated delete into create semantics without rereading the absent UUID. Private markers are closed-set and nil-safe.
- Strict reconfigure interpretation is owner-selective through request options. Existing ACL, host-alias, host-override, and other unrelated Unbound callers retain their prior behavior unless separately proven.
- General numeric-string decoding validates pinned OPNsense formats and bounds, including serve-expired TTL/client-timeout values and cache sizes with digits plus the allowed optional size suffix. Tests cover malformed, negative, overflow, and boundary inputs.
- Replacement acceptance mutates each settings branch through Terraform, compares all aggregate typed DNSBL fields and canonical `dnsbl_json` to exact API fixtures, validates restore selection maps, states forward import provenance precisely, and promotes no audit row until exact-digest local gates, clean live proof, and four-layer convergence all pass.

### 2026-10-19 — Review pass 3
- verdicts: five deduplicated verified findings — high 4, medium 1; all require Repair 3 before publication
- findings:
  - `[high]` `[patch]` Current OPNsense 26.7.7 forward decoding defaults an absent/empty `port` even though only the legacy adapter may default query port 53. Reject missing or empty current ports and retain the default only in the legacy response adapter.
  - `[high]` `[evidence]` The live suite transitions one query object through DoT and back, but never executes independent DoT create or DoT delete while the acceptance contract and audit claim full query/DoT lifecycles.
  - `[high]` `[evidence]` The live suite imports and reads the API-created DNSBL item but never mutates it through Terraform while the settings-resource audit claims update and the branch-change contract covers every owned settings branch.
  - `[high]` `[patch]` Initial legacy query-forward Delete hard-codes current strict request options. Select current/legacy options before the first delete so an established legacy empty reconfigure response does not fail after deletion committed.
  - `[medium]` `[patch]` Acceptance PreCheck mutates singleton/DNSBL state without an immediately registered rollback, and partial DNSBL fixture creation can strand an item if later setup fails. Register idempotent `t.Cleanup` after snapshot capture and compensate partial fixture creation.

## Repair-3 Requirements

- Current 26.7.7 forward responses require a present, non-empty, in-range `port`; only the legacy adapter may default an omitted query port to 53. Add direct current-missing/current-empty and legacy-missing port tests.
- Resolve compatibility-selected request options before the first forward Delete and prove a legacy query delete with an empty legacy reconfigure body succeeds in one lifecycle.
- Register idempotent acceptance rollback immediately after the authoritative settings snapshot is captured; compensate a DNSBL item created before SafeSearch/reconfigure setup failures.
- Execute independent Terraform query and DoT create/read/import/no-diff/drift/delete lifecycles, in addition to bidirectional type transitions, so endpoint-family create/delete claims are direct rather than inferred from zero-search cleanup.
- Mutate the API-created DNSBL item through Terraform, prove exact authoritative update/read/data-source equality and drift repair, then restore/clean it. Promote no settings-resource update claim unless that path executes.
- Regenerate and rerun focused/full tests, deterministic generation, diff hygiene, all six DevRail checks, exact-digest disposable OPNsense proof, and a fresh four-layer convergence pass before recreating evidence or promoting any audit row.

### 2026-10-19 — Review pass 4
- verdicts: six deduplicated verified findings — high 2, medium 2, low 2; all require Repair 4 before publication
- findings:
  - `[high]` `[patch]` `ParseMutationResponse` classifies missing and unknown mutation results as `ValidationError`, causing recovery-aware Unbound create/update paths to skip authoritative recovery after an ambiguous committed mutation.
  - `[high]` `[evidence]` Acceptance rollback deletes tagged forwards before asserting cleanup, so a failed provider Delete could be hidden by the rollback helper.
  - `[medium]` `[verification]` The documented legacy compatibility matrix lacks direct general and query-forward Create/Update request tests and direct old-version rejection tests for parity-only and DoT surfaces.
  - `[medium]` `[bookkeeping]` The core scorecard still reports 92 data sources/docs and a 17-item matching backlog instead of the current 94/94/16 inventory.
  - `[low]` `[audit]` Both forward audit rows omit the local-only `description` attribute.
  - `[low]` `[audit]` Generic promotion edits corrupted the unrelated cron-job and Unbound host-alias baseline-only statements; restore all non-target rows exactly to their reviewed baseline.

## Repair-4 Requirements

- Treat only explicit `result="failed"` mutation responses as deterministic validation errors. Missing, empty, or unknown results are ambiguous ordinary errors so recovery-aware owners perform authoritative reconciliation. Add direct parser tests that prove ambiguous bodies are not `ValidationError`.
- Before rollback cleanup can delete anything, the final acceptance step must non-mutatingly search both query-forward and DoT namespaces and require zero owner-tagged objects left by Terraform. Emit stable per-namespace proof lines and require them in the digest-bound runner.
- Add direct older-appliance HTTP-contract tests for established general and query-forward Create/Update behavior and for rejection of configured parity-only and DoT surfaces. Keep the compatibility promise only where those executable branches pass.
- Reconcile the core scorecard to 94 data sources, 94 data-source docs, and 16 resource-matching gaps; record `description` as local-only for both forward rows; and prove every non-target audit row matches the preserved baseline.
- Regenerate and rerun focused/full tests, deterministic generation, diff hygiene, all six DevRail checks, replacement exact-digest proof on VM 102, and a fresh four-layer convergence pass before recreating evidence or promoting any audit row.

### 2026-10-19 — Review pass 5
- verdicts: seven deduplicated verified findings — medium 5, low 2; all require Repair 5 before publication
- findings:
  - `[medium]` `[patch]` Search accepts an empty/missing page as authoritative zero even when pagination declares remaining objects, allowing ownership and cleanup checks to pass falsely.
  - `[medium]` `[bad_spec]` DNSBL and forward Create separate preflight, mutation, and recovery across different locks; concurrent Terraform creates can both pass preflight or let an ambiguous creator adopt another resource's UUID.
  - `[medium]` `[evidence]` The digest claims tests are bound but omits shared-package `errors_test.go`, `reconfigure_test.go`, and `types_test.go` inputs.
  - `[medium]` `[verification]` The legacy query-forward Create test counts Add calls without decoding or asserting the legacy request envelope and required fields.
  - `[medium]` `[bookkeeping]` The terminal implementation-evidence narrative still names Repair 3, omits revoked digest `fd82db09…`, and says a promoted-tree rerun is pending after the exact promoted digest already passed.
  - `[low]` `[docs]` The `ValidationError` type comment still says every non-`saved` result is validation failure, contradicting the explicit-failed-only parser contract.
  - `[low]` `[docs]` Repair-4 evidence/status prose must leave only fresh convergence pending, not a rerun already executed.

## Repair-5 Requirements

- Treat pagination as authoritative only when it is internally consistent: reject negative totals, page totals that change, pages that overrun total, and empty/missing pages before the declared total is collected. Add direct malformed-page tests.
- Serialize each Unbound Create preflight, mutation, and authoritative recovery as one owner-level critical section within the provider process. DNSBL singleton creation and forward ambiguous-outcome recovery must not race another Terraform resource into duplicate ownership or cross-adoption. Add a direct serialization test or an equivalent deterministic contract test.
- Bind every staged shared-package test that proves Story 31.0d behavior into the digest scope, including mutation parsing, strict/legacy reconfigure handling, selected-map decoding, and search pagination tests.
- Decode and assert the older-appliance query-forward Create and Update request envelopes and required fields; retain explicit rejection tests for parity-only general fields and DoT.
- Correct the terminal evidence narrative and `ValidationError` comment. Regenerate and rerun focused/full tests, deterministic generation, diff hygiene, all six DevRail checks, replacement exact-digest proof, and a fresh four-layer convergence pass before recreating evidence or promoting any audit row.

### 2026-10-19 — Review pass 6
- verdicts: four reviewer findings deduplicated to three verified gaps — medium 2, low 1; all require Repair 6 before publication
- findings:
  - `[medium]` `[patch]` Missing `rows`, `rowCount`, `total`, or `current` members and inconsistent page metadata can decode to zero values and be accepted as authoritative zero, allowing ownership discovery and cleanup proof to succeed on a malformed response.
  - `[medium]` `[verification]` The forward serialization test does not prove ambiguous-outcome recovery stays inside the critical section, and no concurrent DNSBL Create test proves its preflight/mutation/recovery section.
  - `[low]` `[verification]` The implemented negative-total rejection has no direct regression test despite the Repair-5 contract requiring direct malformed-page coverage.

## Repair-6 Requirements

- Decode search pagination with presence-aware fields. Require a non-null array `rows`, present `rowCount`, `total`, and `current`, exact requested-page identity, non-negative metadata, and `rowCount` equal to the number of decoded page rows before any zero-result or ownership conclusion is authoritative.
- Add direct tests for empty/missing/null pagination envelopes, negative totals and row counts, mismatched page and row counts, changing totals, overrun totals, and a premature empty page.
- Deterministically block the first ambiguous forward and DNSBL Create inside its recovery search, start a second creator, and prove the second creator cannot issue preflight or mutation traffic until recovery releases the owner lock. Assert the complete critical-section request ordering for both owners.
- Regenerate and rerun focused/full tests, deterministic generation, diff hygiene, all six DevRail checks, replacement exact-digest proof on VM 102, and a fresh four-layer convergence pass before recreating evidence or promoting any audit row.

### 2026-10-19 — Review pass 7
- verdicts: three reviewers found no product defect; two independently verified one low proof-quality gap, plus one low direct-test omission
- findings:
  - `[low]` `[verification]` Forward and DNSBL create-serialization tests launch the second creator and sleep before sampling traffic, without a positive handshake proving it reached the owner-lock attempt; the forward test also checks only a request prefix instead of its complete critical-section order.
  - `[low]` `[verification]` Null `rowCount`, `total`, and `current` values are rejected by the presence-aware decoder but lack direct cases alongside the existing missing-member tests.

## Repair-7 Requirements

- Inject an owner-local lock test seam while retaining the shared production lock. Signal synchronously when the second creator attempts acquisition, sample traffic only after that signal, and assert the exact complete request order and counts for both forward and DNSBL owners.
- Add direct null-metadata cases for `rowCount`, `total`, and `current` so every required non-null pagination member is covered.
- Regenerate and rerun focused/full tests, deterministic generation, diff hygiene, all six DevRail checks, replacement exact-digest proof on VM 102, and a fresh four-layer convergence pass before recreating evidence or promoting any audit row.

## Design Notes

The stable settings API is one tree with multiple independent branches. Terraform ownership must mirror those branches even though the read-only settings view aggregates them. The general owner and settings view must cover the pinned public advanced fields `hide_identity`, `hide_version`, `enable_prefetch_key`, `dnssec_stripped`, `aggressive_nsec`, `qname_min_strict`, `outgoing_num_tcp`, `incoming_num_tcp`, `num_queries_per_thread`, `outgoing_range`, `jostle_timeout`, `discard_timeout`, `private_domains`, `private_addresses`, and `insecure_domains`; cache fields `enable_prefetch`, `unwanted_reply_threshold`, `msg_cache_size`, `rrset_cache_size`, `max_ttl`, `max_negative_ttl`, `min_ttl`, `host_ttl`, `keep_probing_hosts`, and `num_hosts`; logging fields `extended_statistics`, `log_queries`, `log_replies`, `tag_query_reply`, `log_local_actions`, `log_servfail`, `verbosity_level`, and `validation_level`; serve-expired fields `enabled`, `record_reply_ttl`, `ttl`, `reset_ttl`, and `client_response_timeout`; plus ACL `default_action` and forwarding `enabled`. Existing general and DNSBL fields remain owned by their current resources and appear in the complete view.

Forward items use one stored `dot` monad but endpoint-selected type; body `type` alone cannot change query to DoT. OPNsense 26.7.7 also moved SafeSearch into `general.safesearch` and models DNSBL as `dnsbl.blocklist`, so the pinned baseline's older singleton DNSBL representation must not be used as the wire contract. The existing DNSBL resource remains compatibility-owned and is represented from authoritative current wire data without writing the obsolete aggregate shape.

## Verification

**Commands:**
- `go test ./internal/service/unbound ./internal/provider ./internal/coverageaudit` -- focused schema/model/HTTP/recovery/registration/audit tests pass.
- `go generate ./... && git diff --exit-code` -- generated docs and audit output are deterministic and committed.
- `DEVRAIL_IMAGE=ghcr.io/devrail-dev/dev-toolchain:1.12.24 make check` with isolated Docker config -- lint, format, tests, security, scan, and docs all pass on the current Go baseline.
- Story 31.0d disposable OPNsense runner -- verified VM identity/version and exact candidate digest; all singleton/forward tests pass uncached; upstream settings restore exactly; forward and dot cleanup counts are zero.

## Implementation Evidence

Repair 7 passes focused/full tests, deterministic generation, diff hygiene, all six DevRail checks, promoted-digest OPNsense 26.7.7 acceptance, and clean four-layer convergence. The prior `58c59917…`, `8f755959…`, `fd82db09…`, `56464706…`, `ec5d2ba1…`, and `450ee309…` results remain invalid and are retained only in review history. Exactly four Story 31.0d audit rows are reconciled, and the story is ready for review.
