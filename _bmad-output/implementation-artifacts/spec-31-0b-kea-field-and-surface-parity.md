---
title: 'Story 31.0b: Kea Field and Surface Parity'
type: 'feature'
created: '2026-10-18'
status: 'done'
baseline_revision: '4c9bd5f51a5e8a8da085bd7652b298ed5a49c5e9'
review_loop_iteration: 6
followup_review_recommended: false
context:
  - '_bmad-output/implementation-artifacts/epic-31-context.md'
warnings:
  - 'oversized'
deferred:
  - summary: >-
      Standard repository acceptance CI does not provision the disposable Kea parity gates.
    evidence: |-
      The live Story 31.0b run proves the new surfaces, but .github/workflows/acceptance.yml does not set OPNSENSE_KEA_PARITY or OPNSENSE_KEA_TEST_INTERFACE and therefore skips the gated Kea lifecycle tests. Story 32.1 already owns durable acceptance-proof automation.
    location: >-
      .github/workflows/acceptance.yml
    severity: medium
---

<intent-contract>

## Intent

**Problem:** The provider lacks stable DHCPv6 peer and delegated-prefix-pool owners, while existing DHCPv4 and DHCPv6 subnet owners omit material option fields. Imported Kea configuration can therefore be unmanaged or flattened even though OPNsense 26.7.7 exposes stable APIs.

**Approach:** Add paired resource/data-source owners for DHCPv6 peers and PD pools, extend the existing subnet owners with symmetric field mappings, and prove lifecycle/read-back behavior on a disposable OPNsense appliance before moving the eight audited rows to covered.

## Boundaries & Constraints

**Always:** Preserve the current global mutation lock, `/api/kea/service/reconfigure`, UUID import, API read-back after mutation, source-backed missing-object state removal, recoverable identity on partial create failure, deterministic collection encoding, current local subnet fields, and API truth on refresh. A mutation saved before reconfigure fails must retain a private pending-activation marker and retry reconfigure on the next safe lifecycle operation so a refreshed zero-diff plan cannot strand unapplied configuration; clear the marker only after the source-backed affirmative Kea response `{"status":"ok"}`, and retry delete activation before forgetting an already-removed object. Add responses must include a non-empty UUID, Kea delete/reconfigure responses must be affirmative, and ambiguous HTTP 404s must remain diagnostics rather than being treated as object absence; the tagged Kea API proves missing UUIDs through an empty item/monad response. The new peer/PD owners must verify OPNsense 26.7.7+ at runtime before create/update/read. Use the tagged OPNsense `26.7.7` controller/model sources and pinned provider `v0.26.0` commit `33c2a03c2bc1a7339cb5aa5b611e15014e27f7ba` as evidence. Keep Kea v4, v6, and HA services disabled during serial disposable-appliance acceptance.

**Never:** Register deprecated generic `opnsense_kea_peer`, `opnsense_kea_reservation`, or `opnsense_kea_subnet` aliases; copy the pinned provider's semicolon static-route encoding; change existing subnet `pools` from its newline-compatible string type; use the camelCase DHCPv4 HA endpoints as a DHCPv6 template; emit empty scalars for unknown Terraform values; edit generated files directly; claim the audit rows covered without live lifecycle and cleanup proof; mutate a production appliance.

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
|----------|---------------|---------------------------|----------------|
| DHCPv6 peer | Unique name, control-agent URL, omitted/default `primary` or explicit `primary`/`standby` role | Snake-case peer endpoints round-trip exactly one selected role and reconfigure once after each mutation | Reject unsupported roles before mutation; diagnose missing/multiple selected roles; preserve returned UUID and pending activation if save/reconfigure/read-back partially fails |
| PD pool | Parent subnet UUID, aligned IPv6 address or CIDR prefix, matching prefix length, delegated length | Request sends scalar `subnet`; read maps exactly one selected `subnet`; address and CIDR inputs remain plan-stable; import/update/drift retain exact relationship | Reject invalid IPv6, CIDR/prefix-length disagreement, unaligned prefix, malformed/multiple parent selection, out-of-range lengths, or delegation not longer than the aggregate prefix |
| Advanced DHCPv4 subnet | DNS/domain/search, routers/static routes, NTP/time, next-server and TFTP fields with auto-collect disabled | Lists round-trip losslessly; explicit empty collections/scalars clear remote values; unknown/null scalars, booleans, and sets are omitted; static routes encode as comma-separated destination/router pairs; stable plan is empty | Reject absent/null option-data objects, malformed route pairs, non-binary flags/selectors, semantically invalid addresses/domains/text, and explicitly managed empty or populated auto-collected sets before mutation; malformed API shapes are diagnostics |
| Advanced DHCPv6 subnet | DNS servers and search domains on an isolated interface | CSV request and strict selected-map-list response preserve every member; explicit empty sets clear remote values | Unknown/null planned sets remain unresolved and are omitted rather than clearing remote values; absent option-data, invalid members, malformed/multiple required interface selections, and malformed optional allocator selections are diagnostics |
| Reconfigure recovery | Add/update/delete mutation succeeds but Kea reconfigure fails | Preserve identity plus a private pending marker; a later lifecycle operation retries exactly one reconfigure and clears the marker only after exact Kea `status: ok`, including when delete already removed the object | Reject empty/unknown/conflicting non-empty success bodies; never let read-back suppress activation or forget a pending deleted object |
| Runtime compatibility | Peer/PD resource or data source on an older/malformed appliance | Read firmware first and proceed only on OPNsense 26.7.7+ | Return an actionable compatibility diagnostic before item endpoint use or mutation |
| Dependency cleanup | PD pool references a DHCPv6 subnet | Delete PD pool before parent subnet; post-run searches find no story objects | Cleanup failure reports the exact endpoint, query, and remaining object identifiers and never claims coverage |
| Deprecated aliases | Provider registration and generated docs | Only typed Kea names are exposed | Structure/audit tests fail if a generic duplicate appears |
| Acceptance safety | Explicit disposable interface and disabled Kea v4/v6/HA | Preflight requires an exact supported OPNsense version, exact binary flags, 2xx responses, complete enabled/HA/interface hierarchy, a selected non-`lan`/non-`wan` interface, and collision-free unique fixtures; generated peer/PD tests use the same isolated interface and unique names | Any missing/malformed/fractional safety evidence or unsupported firmware fails, not skips, after explicit opt-in; final checks re-prove disabled services and zero matching objects |
| Malformed mutation/missing object | Add without UUID, ambiguous delete body, HTTP 404, or exact empty item/monad response | Add requires a recoverable UUID; delete and reconfigure require affirmative success; only the source-backed empty item/monad shape removes state | Diagnose missing UUID, empty/unknown delete result, and ambiguous 404 without reconfiguring or forgetting state; converge only the proven empty item/monad shape |

</intent-contract>

## Code Map

- `internal/generate/schemas/kea.yaml` -- generator source for item/singleton owners; add DHCPv6 peer and PD-pool definitions using tagged snake-case endpoints, monads `peer`/`pd_pool`, and Kea reconfigure.
- `internal/generate/templates/` and generated Kea item outputs -- reuse the established generated item contract, but ensure create/update recovery and read-back preserve UUID/API truth; generated outputs are regenerated, never hand-edited.
- `internal/service/kea/dhcpv{4,6}_subnet_{schema,model,resource,data_source}.go` -- existing hand-written subnet owners; retain local allocator/ping/interface/pools behavior while adding audited option fields and symmetric request/response conversion.
- `internal/service/kea/exports.go` and `data_source_schema_test.go` -- register exactly two new resources and two new data sources; constructor expectations become 10 resources and 7 data sources, with no generic aliases.
- `pkg/opnsense/{crud.go,reqopts.go,types.go}` and `internal/tfconv/tfconv.go` -- shared mutation/reconfigure and selected-map/CSV primitives; reuse unless focused tests prove a missing field-specific conversion.
- `internal/service/kea/*_test.go` -- add schema, validator, model, HTTP-contract, recovery, import/missing-object, unknown/null, static-route, and registration coverage at the public resource/data-source boundary.
- `internal/service/kea/kea_parity_acceptance_test.go`, `test/scripts/run-vagrant-acceptance.sh`, and `test/README.md` -- opt-in serial 26.7.7 acceptance with collision-resistant objects, firmware/search preflight, drift repair, dependency-ordered cleanup, and four zero-match searches.
- `templates/{resources,data-sources}/kea_*.md.tmpl`, `examples/`, and generated `docs/` -- document new owners, advanced subnet fields, minimum verified version, import syntax, disabled-service safety, and intentional omission of deprecated aliases.
- `_bmad-output/planning-artifacts/provider-ecosystem-coverage-audit.json` and generated Markdown -- update only the eight 31.0b resource/data-source rows after deterministic and live proof; provider totals become 108 resources and 92 data sources.
- `_bmad-output/implementation-artifacts/{31-0b-kea-field-and-surface-parity.md,sprint-status.yaml}` -- record exact verification evidence and completion state.
- OPNsense core tag `26.7.7` commit `f1a83f72b144469fd97839aba2419ad130d19043` -- read-only source for `Dhcpv6Controller.php`, `KeaDhcpv{4,6}.xml`, and `ServiceController.php`; do not commit vendored upstream files.

## Tasks & Acceptance

**Execution:**
- `internal/generate/schemas/kea.yaml` plus generator/templates -- define typed DHCPv6 peer and PD-pool owners and generate symmetric models, schemas, CRUD resources, UUID data sources, and focused generated tests.
- `internal/service/kea/dhcpv4_subnet_{schema,model,resource,data_source}.go` -- add auto-collect, DNS/domain/search, routers/static routes, NTP/time, next-server, and TFTP fields without changing existing field types or lifecycle.
- `internal/service/kea/dhcpv6_subnet_{schema,model,resource,data_source}.go` -- add DNS/search collections with lossless unknown/null and selected-map handling.
- `internal/service/kea/exports.go` and tests -- register only the four new typed surfaces and prove constructor/type-name counts and deprecated-alias absence.
- `internal/service/kea/*_test.go` -- prove the edge-case matrix, exact wire keys/shapes, semantic API read-back, pending-reconfigure recovery, import/no-change, missing-object removal, unknown/null scalar/boolean/set handling, explicit clears, exact data-source equality, and deterministic collection ordering.
- `internal/service/kea/kea_parity_acceptance_test.go`, `test/scripts/run-vagrant-acceptance.sh`, and `test/README.md` -- prove safe disposable lifecycle and exact cleanup behind explicit opt-in guards.
- `templates/{resources,data-sources}/kea_*.md.tmpl`, `examples/`, `_bmad-output/planning-artifacts/provider-ecosystem-coverage-audit.json`, and `_bmad-output/implementation-artifacts/{31-0b-kea-field-and-surface-parity.md,sprint-status.yaml}` -- regenerate public docs/audit output, then classify only the eight evidenced rows covered and record exact completion evidence.

**Acceptance Criteria:**
- Given OPNsense 26.7.7 and disabled Kea services, when DHCPv6 peer and PD-pool resources are created, read, imported, updated, drifted, repaired, and deleted, then stable snake-case item endpoints and one Kea reconfigure per mutation produce API-truth state and zero-diff stable plans.
- Given a PD pool referencing a DHCPv6 subnet, when planning and applying valid and invalid prefix combinations, then valid aligned delegations round-trip and invalid prefix/length/parent combinations fail before mutation with actionable diagnostics.
- Given existing advanced DHCPv4 and DHCPv6 subnets, when imported and refreshed, then every audited field and collection is preserved without flattening, auto-collection surprises, or perpetual drift.
- Given mutation success followed by reconfigure or read-back failure, when Terraform records diagnostics, then the recoverable UUID/remote evidence remains available and planned values are not claimed as remote truth.
- Given a saved add/update/delete followed by reconfigure failure, when Terraform refreshes or retries, then private pending activation forces exactly one later reconfigure before state becomes stable or an already-deleted object is forgotten.
- Given the new peer/PD owners on an appliance older than 26.7.7 or with malformed version evidence, when create/update/read is requested, then an actionable compatibility diagnostic is returned before item endpoint use or mutation.
- Given provider registration and generated documentation, when structure and audit validation run, then 108 resources and 92 data sources are represented, the eight 31.0b rows are covered only after live proof, and no deprecated generic Kea alias is exposed.
- Given the disposable acceptance run, when cleanup completes, then PD pools are removed before parent subnets and peer, PD-pool, v4-subnet, and v6-subnet searches each return zero Story31b matches.
- Given explicit empty DHCPv4/DHCPv6 option collections, when Terraform updates an existing subnet, then the API receives intentional empty values and refresh remains stable; null or unknown planned collections are omitted and never clear remote data.
- Given the live parity configuration, when advanced v4/v6 values are changed from populated to explicitly empty and a PD prefix is exercised in CIDR form, then resource state preserves the configured CIDR spelling, the UUID-only data source reports the appliance's equivalent address-only API truth, empty fields round-trip exactly, and the following resource plan is empty.
- Given malformed selected-map, numeric, auto-collect, or option-list API responses, when a resource or data source refreshes, then it returns an actionable diagnostic instead of silently flattening state.
- Given the generated peer and PD-pool acceptance tests, when they run on a disposable target, then they use collision-resistant values and only the verified isolated interface; `lan`, `wan`, absent interfaces, non-2xx prechecks, and missing response hierarchy fail before mutation.
- Given explicit parity opt-in, when firmware is malformed/unsupported or enabled/interface flags are fractional, boolean, null, array, or object values, then acceptance fails before mutation rather than skipping or coercing evidence.
- Given partially unknown advanced collections or optional scalar clears, when Terraform validates and encodes a request, then unresolved values remain deferred/omitted while known empty values intentionally clear remote state, including PD descriptions.
- Unit verification includes positive PD-pool integer decoding, negative validator tables for every new validator, and partial-create identity recovery for DHCPv6 subnet and PD pool in addition to peer and DHCPv4.

## Spec Change Log

### 2026-10-18 — Review repair 1

- Trigger: the first four-layer review found that the initial derivation collapsed explicit empty and unknown collections, trusted malformed selected maps and option flags, rejected the pinned provider's documented CIDR prefix form, and left generated acceptance fixtures able to use management interfaces or fixed names.
- Amended: the edge-case matrix and acceptance criteria now distinguish explicit empty from null/unknown, require strict response decoding and relational validation, accept address/CIDR PD prefixes with stable semantics, strengthen disposable-interface/status/hierarchy/collision guards, require identifier-rich cleanup diagnostics, and name the missing positive/negative/recovery tests.
- Known-bad state avoided: no re-derived implementation may omit explicit clears, silently coerce malformed API truth, hard-code `lan`, use fixed peer/PD fixtures, or claim disabled-service safety from a missing/error response.
- KEEP: preserve the passing typed peer/PD owners, snake-case endpoints, global lock and single reconfigure lifecycle, UUID recovery, advanced subnet mappings, deterministic flat comma-pair static routes, typed registration/docs/audit generation, 20-step live lifecycle structure, collision-resistant custom suite, bounded mutation diagnostics, and exact four-search cleanup proof from reviewed attempt `/tmp/story-31-0b-reviewed-attempt1.patch`.

### 2026-10-18 — Review repair 2

- Trigger: the convergence review found that a saved mutation could become zero-diff after reconfigure failure and never activate, while response strictness and final live-proof evidence remained incomplete.
- Amended: require durable private pending-activation recovery for add/update/delete, strict semantic response validation, omission of unknown scalars/booleans, pinned-default peer-role compatibility, exact acceptance safety flags/version failures, explicit-clear and CIDR live proof, exact data-source parity, and final-run audit evidence.
- Known-bad state avoided: no re-derived implementation may strand saved-but-unapplied Kea configuration, silently flatten absent option objects or malformed flags/selections, skip an explicitly opted-in unsupported appliance, or promote audit rows using a pre-repair run.
- KEEP: preserve the typed surfaces, source-backed endpoints, deterministic generation/docs/audit work, repaired explicit-empty encoders, strict peer/PD decoding, relational PD validation, collision-resistant lifecycle structure, and identifier-rich cleanup proof from reviewed attempt `/tmp/story-31-0b-reviewed-attempt2.patch`.

### 2026-10-18 — Review repair 3

- Trigger: pass 3 found ambiguous HTTP-200 reconfigure bodies could clear pending activation, the evidence digest was self-referential, and the new 26.7.7-only owners lacked a runtime compatibility gate.
- Amended: require exact Kea `status: ok`, runtime 26.7.7+ compatibility diagnostics, a reproducible code/test-only attestation scope, retained sanitized non-cached output, strict remaining response/unknown handling, exact live member assertions, and factual support/status synchronization.
- Known-bad state avoided: no implementation may clear pending activation on `{}` or unknown status, mutate an unsupported appliance, turn partially unknown values into clears, accept missing interfaces after explicit opt-in, or promote coverage against an unreproducible digest.
- KEEP: preserve repair 2's pending private state machine, CIDR normalization/resource spelling, address-only data-source truth, strict safety decoder, explicit-empty live lifecycle, eight-row boundary, and final-run cleanup structure from `/tmp/story-31-0b-reviewed-attempt3.patch`.

### 2026-10-18 — Review repair 4

- Trigger: final convergence review found ambiguous 404/delete/add handling could forget state or lose identity, validation still accepted malformed/partially unknown values, the attestation digest depended on a dirty worktree, and the final live command omitted the generated peer lifecycle.
- Amended: require non-empty add UUIDs, affirmative Kea delete/reconfigure bodies, source-backed empty item/monad missing-object detection with ambiguous 404 diagnostics, complete allocator/relationship/static-route validation, deferred partially unknown collections, read-semaphore compatibility probes, a content-based commit-stable digest, row-specific audit evidence, and both generated peer/PD live lifecycles.
- Known-bad state avoided: no implementation may erase state on an endpoint/plugin 404, accept an identity-less add or ambiguous delete, reject unknown collection members that should remain deferred, attest only a dirty diff, or claim generated-owner acceptance without running both generated tests.
- KEEP: preserve repair 3's strict activation/version gates, typed surfaces, pending activation state, CIDR semantics, exact data-source assertions, disabled-service safety, deterministic generation, and zero-match cleanup structure.

### 2026-10-18 — Review repair 5

- Trigger: pass 5 found that non-2xx responses could still be accepted, strict Kea absence had accidentally replaced generic legacy absence behavior, selected-map/list decoders retained malformed/nondeterministic cases, and retained support/evidence text was incomplete.
- Amended: reject every non-2xx status, make strict missing-object decoding opt-in for the four source-proven Kea owners, preserve legacy generic empty-object absence, count optional selections deterministically, require selected-map option-list response shapes and children, distinguish PD resource/data-source prefix descriptions, and emit the fixture marker plus exact cleanup queries from the acceptance test itself.
- Known-bad state avoided: no implementation may accept affirmative-looking 4xx bodies, erase state from ambiguous Kea responses, break existing non-Kea empty-object refresh behavior, flatten malformed option lists, or claim cleanup without replayable endpoint/query evidence.
- KEEP: preserve repair 4's UUID/delete/reconfigure strictness, content-stable digest, validation, runtime gate, generated lifecycles, disabled-service safety, and eight-row evidence boundary.

## Review Triage Log

### 2026-10-18 — Review pass 1
- verdicts: 26 findings — high 4, medium 18, low 1, false 3, maybe-false 0
- findings:
  - `[medium]` `[bad_spec]` DHCPv4 explicit empty option values were omitted and could not clear remote state — amended the spec to distinguish explicit empty from null/unknown while preserving intentional clears.
  - `[medium]` `[bad_spec]` DHCPv6 explicit empty DNS/search sets were omitted and could not clear remote state — amended the same explicit-empty contract for DHCPv6.
  - `[medium]` `[bad_spec]` TFTP server/bootfile accepted newline and quote values rejected by the tagged OPNsense model — added source-backed text validation to the matrix.
  - `[medium]` `[bad_spec]` DHCPv6 peer refresh silently accepted missing or multiply selected roles — strict exactly-one selected-map decoding is now required.
  - `[medium]` `[bad_spec]` PD-pool refresh silently accepted missing or multiply selected parents — strict exactly-one parent decoding is now required.
  - `[medium]` `[bad_spec]` PD-pool refresh did not revalidate IPv6 alignment or delegated-length ordering — the response contract now requires relational validation.
  - `[false]` `[reject]` Local validation allegedly had to prove the parent UUID exists — the intent requires a scalar relation and OPNsense's ModelRelationField validates existence; no local inventory lookup was specified.
  - `[medium]` `[bad_spec]` The implementation rejected CIDR-form PD prefixes documented by the pinned provider — the spec now accepts address or CIDR form with matching, plan-stable prefix semantics.
  - `[high]` `[bad_spec]` Acceptance safety treated non-2xx or missing enabled/HA hierarchy as disabled — preflight must now require 2xx and complete response structure.
  - `[high]` `[bad_spec]` Acceptance accepted `lan`, `wan`, or an absent test interface — the spec now requires a selected non-management interface before mutation.
  - `[high]` `[bad_spec]` The generated PD-pool test hard-coded `lan` despite the isolated-interface gate — generated tests must consume the verified isolated interface.
  - `[medium]` `[bad_spec]` Generated peer/PD fixtures used fixed collision-prone values — collision-resistant generated fixtures and preflight are now required.
  - `[false]` `[reject]` The disabled-service claim lacked any final evidence — the live run was followed by explicit v4/v6 general and HA reads, and the exercised object endpoints cannot enable those settings.
  - `[low]` `[patch]` DHCPv4 docs overstated that every advanced field conflicts with auto-collection — narrow the sentence to router, DNS, and NTP values during re-derivation.
  - `[false]` `[reject]` PD parent containment/existence was said to be missing from the local validator — containment is not an OPNsense relation rule and parent existence is enforced by the API relation field.
  - `[medium]` `[bad_spec]` PD-pool response validation omitted delegated-length ordering — same relational-response root cause as the sixth finding; amended once and carried here.
  - `[medium]` `[bad_spec]` Peer role selected-map decoding was not strict — same strict peer response root cause as the fourth finding; amended once and carried here.
  - `[medium]` `[bad_spec]` PD parent selected-map decoding was not strict — same strict parent response root cause as the fifth finding; amended once and carried here.
  - `[medium]` `[bad_spec]` DHCPv4 option-list responses silently dropped missing/non-binary selected flags — malformed option selectors must now diagnose rather than flatten state.
  - `[medium]` `[bad_spec]` DHCPv4 auto-collect responses silently coerced values other than `0`/`1` — strict boolean response validation is now required.
  - `[high]` `[bad_spec]` Acceptance preflight omitted HTTP-status and hierarchy checks — same safety root cause as the ninth finding; amended once and carried here.
  - `[medium]` `[bad_spec]` Cleanup errors did not identify remaining object UUIDs — cleanup diagnostics must now include remaining identifiers.
  - `[medium]` `[defer]` Standard repository acceptance CI does not provision the disposable Kea gates — live proof exists for this story; durable CI acceptance ownership remains with Story 32.1.
  - `[medium]` `[patch]` Valid PD integer read-back had only negative unit coverage — add a successful `48`/`64` resource or data-source read test.
  - `[medium]` `[patch]` DHCPv4 subnet, DHCPv6 subnet, and peer negative validators lacked focused tests — add source-backed table-driven invalid cases.
  - `[medium]` `[patch]` DHCPv6 subnet and PD-pool partial-create recovery branches lacked tests — add HTTP-backed UUID-preservation tests without claiming planned values.

### 2026-10-18 — Review pass 2
- verdicts: 18 unique findings — high 3, medium 13, low 2, false 0, maybe-false 0
- findings:
  - `[high]` `[bad_spec]` Saved add/update/delete mutations could become zero-diff after reconfigure failure and never activate — require a private pending-activation marker and lifecycle retry before stability/removal.
  - `[high]` `[bad_spec]` Fractional or typed safety flags were truncated and could pass disposable preflight — require exact binary decoding and fail malformed evidence before mutation.
  - `[high]` `[patch]` Eight covered audit rows cited the pre-repair live run — update canonical evidence to the final repaired run and regenerate Markdown.
  - `[medium]` `[bad_spec]` Explicit-empty clearing was proven only at marshal level — require live populated-to-empty update, read-back, data-source equality, and zero-diff proof.
  - `[medium]` `[patch]` Explicit opt-in skipped unsupported or unparseable firmware and rejected valid newer two-part versions — fail gated runs and use a tested OPNsense-aware comparator.
  - `[medium]` `[patch]` Live data-source checks compared counts rather than exact values — compare all advanced subnet, peer, and PD attributes and members.
  - `[medium]` `[patch]` Shared mutation tests proved at least one rather than exactly one reconfigure — assert exact counts and zero calls after failed mutations.
  - `[medium]` `[patch]` Public missing-object removal lacked focused tests — cover HTTP 404 and empty monad for all four Story 31.0b owners.
  - `[medium]` `[bad_spec]` Missing or null option-data objects silently flattened state — require presence and diagnostic failure for v4/v6 resource and data-source refresh.
  - `[medium]` `[bad_spec]` Unknown subnet scalars and booleans serialized zero values — omit unknown/null request values or return apply-time diagnostics without remote mutation.
  - `[medium]` `[patch]` CIDR-form PD prefixes were not included in the cited live command — exercise CIDR form in the main parity suite or an explicitly recorded generated test.
  - `[low]` `[patch]` Generated PD cleanup did not prove parent subnet deletion — add composite parent/pool cleanup evidence.
  - `[medium]` `[bad_spec]` Known empty auto-collected sets bypassed conflict validation — treat configured empty and populated sets as explicit management when auto-collect is enabled.
  - `[medium]` `[bad_spec]` Advanced response values were shape-checked but not semantically validated — reject invalid addresses, domains, text, and malformed option members on refresh.
  - `[medium]` `[bad_spec]` Match-client-id and ping-check responses silently coerced non-binary values — decode all binary fields strictly.
  - `[medium]` `[bad_spec]` DHCPv6 required and optional selected maps remained permissive — use strict exactly-one interface and strict optional-single allocator decoding.
  - `[medium]` `[bad_spec]` Peer role was required despite the pinned provider and upstream default — support omitted role with default `primary` while retaining strict read-back.
  - `[low]` `[patch]` Implementation evidence still said review had not started — synchronize status and evidence after convergence.

### 2026-10-18 — Review pass 3
- verdicts: 17 unique findings — high 2, medium 13, low 2, false 0, maybe-false 0
- findings:
  - `[high]` `[bad_spec]` Ambiguous HTTP-200 reconfigure bodies cleared pending activation — require exact source-backed Kea `status: ok` and retain the marker for malformed/unknown responses.
  - `[high]` `[bad_spec]` Live evidence hashed a self-containing staged diff and could not identify the tested artifact — define and retain a reproducible code/test-only digest scope verified before and after live execution.
  - `[medium]` `[bad_spec]` New 26.7.7-only peer/PD owners lacked a runtime compatibility gate — verify firmware before create/update/read and return actionable diagnostics.
  - `[medium]` `[patch]` Support matrix, roadmaps, registry docs, and README retained stale counts/status — synchronize factual inventory and Story 31.0a/31.0b state without preempting Story 31.1 positioning work.
  - `[medium]` `[patch]` Live data-source checks still omitted exact members/scalars — assert every audited populated and cleared value, including static-route objects.
  - `[medium]` `[patch]` Generated PD cleanup used a permissive shared helper and the generated test was not run — make deletion proof strict and execute the generated PD lifecycle.
  - `[medium]` `[patch]` Retained live evidence lacked sanitized non-cached command output — retain a sanitized `-count=1` result artifact with firmware, non-skip, safety, and cleanup proof.
  - `[medium]` `[patch]` Pending activation recovery tests covered only the peer — table-drive add/update/delete retry semantics across v4 subnet, v6 subnet, peer, and PD pool.
  - `[medium]` `[bad_spec]` DHCPv4 allocator refresh remained permissive — require strict optional-single decoding plus supported-value validation.
  - `[medium]` `[patch]` Shared delete translated HTTP 404 to the wrong error type — converge already-absent item deletion and cover public owners.
  - `[medium]` `[patch]` Acceptance version parsing ignored integer overflow — fail oversized/malformed version components.
  - `[medium]` `[bad_spec]` Peer response validation omitted name and URL semantics — share response-side absolute HTTP/HTTPS and nonempty-name checks.
  - `[medium]` `[bad_spec]` PD description could not be explicitly cleared because generated request strings used `omitempty` — distinguish known empty from null/unknown for optional generated scalars.
  - `[medium]` `[patch]` Explicit parity opt-in skipped when the isolated interface was absent — fail once opt-in is set; only absent opt-in may skip.
  - `[medium]` `[bad_spec]` Partially unknown sets/object members could collapse into explicit clears — defer/omit unresolved members and never discard conversion diagnostics into an empty payload.
  - `[low]` `[patch]` Cleanup accepted `total=0` with nonempty rows — reject inconsistent search responses.
  - `[low]` `[patch]` PD data-source docs implied CIDR output despite address-only API truth — document resource-only CIDR spelling preservation.

### 2026-10-18 — Review pass 4
- verdicts: 15 unique findings — high 3, medium 9, low 3, false 0, maybe-false 0
- findings:
  - `[high]` `[bad_spec]` An add response without a non-empty UUID could save remote configuration without recoverable identity — require a non-empty UUID before reconfigure and retain an actionable diagnostic.
  - `[high]` `[bad_spec]` Ambiguous HTTP 404 responses could erase Terraform state even when the endpoint or plugin was missing — only the tagged empty item/monad response proves object absence.
  - `[high]` `[bad_spec]` The live attestation digest depended on the dirty index/worktree and was not commit-stable — hash a sorted, content-based code/test scope including the digest script and acceptance runner.
  - `[medium]` `[patch]` Required interface and parent relationships still accepted malformed selections — enforce exactly one selected relationship where required.
  - `[medium]` `[patch]` Allocator response values were shape-checked but not constrained to source-backed domains — validate supported DHCPv4/DHCPv6 allocator values.
  - `[medium]` `[patch]` Partially unknown advanced collections could be rejected or flattened instead of deferred — omit unresolved collections and known members from mutation payloads until fully known.
  - `[medium]` `[patch]` Static-route entries accepted empty destination/router members — require nonempty IPv4 destination/router pairs before mutation.
  - `[medium]` `[patch]` Delete responses could be treated as success without an affirmative Kea result — require strict endpoint-specific success bodies and preserve state on ambiguity.
  - `[medium]` `[patch]` The final live command omitted the generated peer lifecycle — run both generated peer and PD-pool tests with the parity suite.
  - `[medium]` `[patch]` Coverage rows reused generic lifecycle claims — retain row-specific resource/data-source lifecycle and read-back evidence.
  - `[medium]` `[patch]` Support summaries still counted Story 31.0b among remaining gaps — synchronize the remaining set to Stories 31.0c–31.0j after proof.
  - `[medium]` `[patch]` Interface assignment remained marked pending after Story 30.2 merged — synchronize the support matrix to the shipped owner.
  - `[low]` `[patch]` Live mutation helpers did not require affirmative saved/reconfigure bodies — validate both before advancing acceptance steps.
  - `[low]` `[patch]` The runtime compatibility probe bypassed the shared read semaphore — acquire and release the standard read limiter around firmware status.
  - `[low]` `[patch]` Acceptance documentation named fewer opt-in flags than the runner requires — document all four Kea acceptance flags.

### 2026-10-18 — Review pass 5
- verdicts: 9 unique findings — high 2, medium 4, low 3, false 0, maybe-false 0
- findings:
  - `[high]` `[bad_spec]` Non-2xx statuses outside 401/403/404 could be accepted as CRUD success or absence — reject every non-2xx status before decoding response semantics.
  - `[high]` `[bad_spec]` Strict Kea missing-object handling was applied globally and broke legacy empty-object absence for unrelated owners — retain legacy generic behavior and opt the four Story 31.0b owners into strict source-backed decoding.
  - `[medium]` `[patch]` Mixed empty-sentinel and real optional selections were map-order dependent — count every selected entry independently and reject multiples deterministically.
  - `[medium]` `[bad_spec]` Kea option-list decoders accepted null, array, string, CSV, or missing child shapes — require selected-map objects and every source-backed option child.
  - `[medium]` `[patch]` The current data-source inventory was incomplete despite correct totals — point the planning page at authoritative registration/generated inventories instead of a stale partial list.
  - `[medium]` `[patch]` The Kea roadmap omitted the new DHCPv6 peer and PD-pool surfaces — synchronize the implemented surface list.
  - `[low]` `[patch]` PD-pool data-source prefix docs contradicted address-only API truth — retain address/CIDR input only on the resource and document address-only data-source output.
  - `[low]` `[patch]` The support matrix still named Stories 31.0a–31.0j as remaining — narrow the remaining range to 31.0c–31.0j.
  - `[low]` `[patch]` Retained cleanup evidence omitted the random fixture marker and exact queries — emit both from the acceptance test and retain the verbose lines.

### 2026-10-18 — Review pass 6
- verdicts: 5 unique findings — high 0, medium 3, low 2, false 0, maybe-false 0
- findings:
  - `[medium]` `[patch]` Required selected-map decoding remained map-order dependent for a selected empty key plus selected real key — count required selections independently and reject every multiple-selection shape deterministically.
  - `[medium]` `[patch]` Generated peer cleanup used a permissive generic helper that ignored transport failures and did not inspect HTTP-200 bodies — generate a strict Kea cleanup helper and prove malformed, present-object, non-2xx, and transport failure behavior.
  - `[medium]` `[patch]` Generated PD-pool and parent cleanup omitted strict missing-object response handling — require the source-backed HTTP-200 empty-array shape for all generated cleanup owners.
  - `[low]` `[patch]` The remaining-domain support row still included firewall/NAT completed by Story 31.0a — start the remaining 31.0c–31.0j list with OpenVPN.
  - `[low]` `[patch]` Sprint status still named repair 4 after the repair-5 run — synchronize the comment to the final convergence state.

### 2026-10-18 — Review pass 7
- verdicts: 0 findings — high 0, medium 0, low 0, false 0, maybe-false 0
- result: publication GO. Independent blind, edge-case, and verification reviewers confirmed deterministic required-selection decoding, strict generated peer/PD-pool/parent cleanup, compatible generic versus strict missing-object behavior, reproducible live evidence, exactly eight covered audit rows, synchronized 108/92 inventory, and no stale Story 31.0b run or digest references.

## Design Notes

The tagged target API uses asymmetric wire forms: peer `role` and PD-pool `subnet` are scalar requests but selected-map responses; subnet option lists are CSV requests but selected-map-list responses. DHCPv4 static routes are a flat comma sequence of destination/router pairs, despite the pinned provider's semicolon implementation. Field-specific converters must preserve these distinctions rather than broadening shared decoding in ways that corrupt unrelated values.

The new owners require OPNsense `26.7.7+` unless an earlier stable tag is separately proven. Runtime compatibility uses the low-privilege `/api/core/firmware/status` response and its nested `product.product_version`; this avoids concurrent reads of the much larger firmware-info payload. DHCPv6 peer routes are `/api/kea/dhcpv6/{add,get,set,del,search}_peer`; PD-pool routes use the same verbs with `_pd_pool`. Both mutate under the shared lock and call `/api/kea/service/reconfigure`.

## Implementation Evidence

- Local generated/model/schema/validator/HTTP-contract tests pass, including strict malformed-response diagnostics, identity-only partial-create recovery, exact acceptance mutation encoding, and bounded HTTP diagnostics.
- `TestAccKeaFieldAndSurfaceParity` passed all 20 steps on disposable OPNsense 26.7.7 after pass-6 cleanup patching in final run `20261018T233224Z`, covering create/read/import/update, exact data-source parity, populated-to-explicit-empty updates, CIDR resource stability with address-only API data-source truth, stable plans, drift repair, and dependency-ordered cleanup.
- `TestAccDHCPv6Peer_basic` and `TestAccDHCPv6PDPool_basic` passed generated create, update, import, and strict cleanup in the same uncached verbose run.
- Post-run searches returned `total=0` for `dhcpv4/search_subnet`, `dhcpv6/search_subnet`, `dhcpv6/search_peer`, and `dhcpv6/search_pd_pool`.
- DHCPv4 general and HA remained disabled, DHCPv6 general and HA remained disabled, and isolated interface `opt1` remained selected throughout the live proof.
- Exactly the eight scoped coverage-audit rows are `covered` with row-specific final run `20261018T233224Z` evidence in the canonical JSON and generated Markdown; review pass 7 returned publication GO with no findings.
- Sanitized final-run metadata, matching pre/post content digest `2db677cf3ca78f0ff680ca738ed75973c0635fde5c5159abff2802db3cc3d19f`, verbose non-skip output, safety state, fixture marker, and four exact zero-match cleanup queries are retained in `_bmad-output/implementation-artifacts/31-0b-live-evidence.json` and `31-0b-live-output.txt`.

## Verification

**Commands:**
- `go test -count=1 ./internal/service/kea/... ./pkg/opnsense/... ./internal/tfconv/...` -- expected: schemas, conversions, lifecycle/recovery, and HTTP contracts pass.
- `go generate ./... && go generate ./... && git diff --exit-code` -- expected: docs, generated owners, and coverage report are deterministic and current.
- `go test -count=1 ./internal/coverageaudit && bash test/scripts/validate-structure.sh` -- expected: audit invariants and exact registration pass.
- `TF_ACC=1 go test -p 1 -run 'TestAcc(Kea.*Parity|DHCPv6(Peer|PDPool)_basic)' ./internal/service/kea` with disposable 26.7.7 credentials -- expected: v4/v6 subnet, generated peer/PD-pool, drift/import/data-source, and cleanup proof passes.
- `DEVRAIL_IMAGE=ghcr.io/devrail-dev/dev-toolchain:1.12.24 make check && git diff --check` -- expected: lint, format, tests, security, scan, docs, and whitespace checks pass.
