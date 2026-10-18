---
title: 'Story 31.0a: Firewall and NAT Field Parity'
type: 'feature'
created: '2026-10-18'
status: 'done'
baseline_revision: '77bef62d43765d1e37a2a8eed806dad3a5499c21'
review_loop_iteration: 2
followup_review_recommended: false
context:
  - '_bmad-output/implementation-artifacts/epic-31-context.md'
warnings:
  - 'oversized'
deferred: []
---

<intent-contract>

## Intent

**Problem:** Existing firewall alias, filter-rule, and destination-NAT owners use stable OPNsense APIs but flatten or omit material fields, so imported advanced objects can lose semantics or produce inaccurate state.

**Approach:** Extend the existing resources and data sources with symmetric request/read-back models, compatibility-preserving additive attributes, generated documentation, and live disposable-appliance proof; then mark only the proven audit rows covered.

## Boundaries & Constraints

**Always:** Preserve UUID identity/import, missing-object state removal, the global mutation lock, API read-back after create/update, direct alias reconfigure and firewall apply after mutations, deterministic set/CSV conversion, API truth on refresh, and one-provider ownership. Retain existing scalar HCL where a new set-valued compatibility attribute is needed. Use OPNsense 26.7.7+ disposable acceptance objects with unique names/descriptions, disable traffic-affecting rules where possible, and restore/delete every object exactly.

**Never:** Create competing resource types; rename or re-nest existing attributes; reintroduce the removed savepoint, revisioned apply, or cancel-rollback endpoints; echo plans into state; flatten multi-selected API maps; accept unsupported field combinations silently; add pinned-provider-only `sort_order`; scrape UI pages or edit `config.xml`; mutate production firewall objects.

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
|----------|---------------|---------------------------|----------------|
| Advanced alias | `urljson`, `dynipv6host`, counters, or dual IP protocols | Conditional fields and every selected protocol round-trip through API read-back | Reject missing/inapplicable conditional fields before mutation |
| Imported advanced filter | Existing schedule, tags, state limits, TCP/ICMP, routing, shaping, or HA flags | Refresh captures every field; no-change plan is empty | Reject only combinations supported by source-backed validators; retain opaque IDs otherwise |
| Multi-interface port forward | API/config selects two or more interfaces | State and data source retain the full unordered set; update sends sorted CSV | Reject an explicitly configured empty set; never collapse to one member |
| Legacy scalar configuration | Existing alias `proto` or NAT `interface` remains configured | Additive canonical set attributes preserve compatibility and remote multi-value state | Reject conflicting explicit scalar/set values with a specific diagnostic |
| Apply failure | Mutation succeeds but alias reconfigure or firewall apply fails | Terraform reports the partial failure and does not claim normalized success | Preserve recoverable identity/state behavior; do not call unsupported rollback APIs |
| Out-of-band drift | Advanced field or set membership changes in OPNsense | Refresh exposes the drift and apply restores configuration | Missing UUID removes resource; transport/API errors remain diagnostics |

</intent-contract>

## Code Map

- `internal/service/firewall/alias_schema.go` and `alias_model.go` -- add `interface`, `path_expression`, `stats` (`counters` on wire), canonical multi-valued `ip_protocol`, conditional validation, response/request conversion, and deterministic compatibility behavior for deprecated scalar `proto`; retain fractional `update_freq` value semantics.
- `internal/service/firewall/alias_{resource,data_source}.go` and tests -- existing alias CRUD/import/read owner and shared read-back; add schema/model/HTTP/acceptance coverage without changing `/api/firewall/alias/{addItem,getItem,setItem,delItem,reconfigure}` ownership.
- `internal/service/firewall/filter_rule_schema.go` and `filter_rule_model.go` -- extend the flat compatible schema/model for interface inversion/options, HA flags, local tags, priority/TOS, schedule, state policy/limits/rates, ICMP/TCP matching, reply-to, and shaping; map exact API keys and empty-to-`-1` numeric sentinels symmetrically.
- `internal/service/firewall/filter_rule_{resource,data_source}.go` and tests -- preserve UUID CRUD/import and `POST /api/firewall/filter/apply`; prove read-after-write, drift, and apply-failure behavior. Existing lowercase protocol examples/config remain valid alongside appliance-normalized values.
- `internal/service/firewall/nat_port_forward_schema.go` and `nat_port_forward_model.go` -- add canonical `interfaces` set while retaining legacy scalar `interface`; decode GET with `opnsense.SelectedMapList`, send sorted CSV, and diagnose conflicting explicit values.
- `internal/service/firewall/nat_port_forward_{resource,data_source}.go`, `nat_port_forward_{model,schema}_test.go`, and acceptance tests -- preserve direct `/api/firewall/d_nat/apply`, UUID import, and full selected-set read-back. Do not restore the historical savepoint flow removed by `1d95ab3`.
- `pkg/opnsense/types.go` -- reuse `SelectedMapList` and existing bool/string helpers; change shared helpers only if focused tests prove a missing deterministic conversion.
- `/tmp/browningluke-opnsense-0.26.0/` and `/tmp/browningluke-opnsense-0.26.0-schema.json` -- read-only pinned v0.26.0/`33c2a03c2bc1a7339cb5aa5b611e15014e27f7ba` evidence for field types, JSON keys, validators, and lifecycle; never commit these investigation inputs.
- `templates/{resources,data-sources}/firewall_{alias,filter_rule,nat_port_forward}.md.tmpl` and matching `examples/` -- document advanced fields, compatibility attributes, exact limitations, imports, and safe examples; generated `docs/` must remain drift-free.
- `_bmad-output/planning-artifacts/provider-ecosystem-coverage-audit.json` and generated Markdown -- update the six resource/data-source rows only after implementation and disposable proof; retain explicit rationale for any source-backed rejection.
- `_bmad-output/implementation-artifacts/31-0a-firewall-nat-field-parity.md`, `sprint-status.yaml`, and related roadmap/support artifacts -- record completion evidence and remove only the verified Story 31.0a gap claims.
- `internal/acctest/acctest.go`, `test/scripts/run-vagrant-acceptance.sh`, and `test/README.md` -- reuse opt-in secret-backed serial acceptance; target only a disposable OPNsense 26.7.7+ appliance and capture restoration evidence.

## Tasks & Acceptance

**Execution:**
- Alias schema/model/resource/data-source/tests -- add the three audited controls plus lossless multi-protocol state, conditional validation, and compatibility handling for `proto`/`update_freq`.
- Filter schema/model/resource/data-source/tests -- add the audited advanced flat fields with exact wire keys, symmetric conversions, validated numeric/enumerated semantics, and opaque stable identifiers where no list endpoint is evidenced.
- Port-forward schema/model/resource/data-source/tests -- add lossless multi-interface support through additive `interfaces`, preserve legacy `interface`, and cover singleton/multiple/empty/conflict cases.
- Firewall HTTP and acceptance tests -- prove request bodies, read-back, import/no-change, update, data-source parity, out-of-band drift repair, apply ordering/failure, and exact cleanup on a disposable appliance.
- Templates/examples/generated docs and BMad audit/status artifacts -- publish the new contract and change audit classifications only where the proof is complete.

**Acceptance Criteria:**
- Given advanced alias fields and applicable alias types, when create/import/read/update and drift repair run, then interface, JSON path, counters, all selected IP protocols, and fractional refresh intervals round-trip or fail before mutation with a source-backed diagnostic.
- Given an imported advanced filter rule, when Terraform refreshes it, then tags, priorities, schedule, state policy/limits/rates, TCP/ICMP selectors, reply-to, shaping, and HA controls are represented without flattening and a no-change plan is empty.
- Given legacy lowercase protocol values and opaque schedule/gateway/shaper identifiers, when plans run, then compatible existing configuration remains valid and API-normalized read-back does not create perpetual drift.
- Given a port forward spanning multiple interfaces, when resource and data-source reads occur, then every selected interface is present; updating membership sends deterministic CSV and never silently removes an unselected-by-order member.
- Given each create/update/delete, when mutation succeeds, then aliases call direct reconfigure and filters/DNAT call the single firewall apply endpoint under the mutation lock; no savepoint/revision/cancel-rollback path is called.
- Given the disposable OPNsense lifecycle, when advanced objects are created, imported, externally drifted, repaired, and deleted, then second plans are empty at stable points and cleanup restores the pre-test appliance state.
- Given generated docs and the coverage manifest, when generation and validation run, then outputs are deterministic and only proven Story 31.0a rows move out of `missing-buildable`.

## Spec Change Log

### 2026-10-18 — Review repair 1

- Trigger: the first independent review proved that legacy scalar planning could
  overwrite refreshed multi-value state, scalar compatibility views could
  disagree with deterministic API read-back, validation/recovery boundaries
  were incomplete, and parts of the claimed parity lacked direct assertions.
- Amendment: compatibility planning must inspect prior state and seed canonical
  sets only when no canonical state exists; unknown canonical configuration must
  remain unknown. Explicit scalar/set pairs must either use the deterministic
  scalar member or preserve the explicitly configured member without flattening
  the canonical set. Deprecated `proto` follows the same applicability rules as
  `ip_protocol`.
- Amendment: reject empty interface members, non-finite refresh intervals, and
  unsupported ICMP/TCP flag values before mutation. Malformed non-empty numeric
  API values produce diagnostics rather than silently becoming the unset
  sentinel. Plain multi-select decoding must preserve comma- and newline-delimited
  values without regressing other `SelectedMapList` consumers.
- Amendment: when mutation/apply fails and recovery read-back also fails,
  preserve only recoverable identity and never populate unverified planned
  fields as remote truth. Add direct positive assertions for dynamic-alias
  interface and filter shaper conversion. Live acceptance must exercise a
  multi-interface membership update and filter update/drift while retaining
  representative safe disabled objects and exact cleanup checks.
- Known-bad state avoided: a singleton legacy scalar may not silently erase
  additional remote protocols/interfaces; API parse failures may not masquerade
  as unset state; failed recovery may not echo the plan into state.
- KEEP: retain the additive resource/data-source surfaces, direct apply and
  reconfigure endpoints, mutation locking, generated docs, deterministic set
  ordering, OPNsense 26.7.7 opt-in harness, successful alias drift proof,
  protocol-casing no-diff behavior, partial-failure UUID recovery when read-back
  succeeds, and all source-backed field mappings already shown correct.

### 2026-10-18 — Review repair 2

- Trigger: the follow-up review proved that legacy-only scalar changes still
  retained old singleton canonical state, request encoding could fall back from
  unknown canonical values, and several source-backed alias/filter domains and
  create-recovery boundaries remained incomplete.
- Amendment: when only a legacy scalar is configured, a changed scalar replaces
  a prior singleton canonical set, while a prior multi-valued set is preserved
  for unrelated updates. Unknown canonical sets may never fall back to scalar
  request values. An explicitly configured empty canonical set is invalid even
  when its scalar compatibility field is populated.
- Amendment: alias protocol applicability includes `asn`, `geoip`, and
  `external`. Filter protocol, ICMP, TOS, TCP-only, and generic state-limit
  validators must match the pinned source domains exactly; shared selected-list
  decoding may not split field values on ambiguous delimiters.
- Amendment: every successful mutation that yields a UUID but fails read-back or
  response validation must preserve identity-only state. Normal HTTP tests must
  exercise successful and failed recovery, resource-path legacy-scalar
  preservation, framework validator registration, and every non-default
  advanced filter mapping; live acceptance remains an explicit manual gate.
- Known-bad state avoided: legacy scalar updates may not become no-ops, unknown
  sets may not collapse to singleton requests, valid GeoIP/external or generic
  state-limit configurations may not be rejected, invalid protocol domains may
  not reach OPNsense, and successful creates may not orphan remote objects.
- KEEP: preserve every first-repair invariant, the reviewed additive public
  schema, deterministic direct apply/reconfigure lifecycle, generated docs and
  audit tooling, safe disabled acceptance fixtures, the passing 26.7.7 live
  alias/filter/NAT lifecycle, exact post-run zero-match cleanup evidence, and
  the acceptance harness fixes that emit valid HCL and exact scalar mutation
  payloads.

## Review Triage Log

### 2026-10-18 — Review pass
- verdicts: 36 findings — high 11, medium 16, low 6, false 3, maybe-false 0
- findings:
  - `[high]` `[bad_spec]` Legacy alias `proto` overwrites refreshed multi-protocol state — `ModifyPlan` unconditionally seeds a singleton whenever canonical config is omitted; the spec now requires inspecting prior canonical state before seeding.
  - `[high]` `[bad_spec]` Legacy NAT `interface` overwrites refreshed multi-interface state — the same unconditional singleton seeding can remove remote members; the spec now requires prior-state preservation.
  - `[high]` `[bad_spec]` Alias scalar/set pairs can read back a different deterministic scalar — validation accepts any member while `fromAPI` chooses the sorted first; the spec now requires deterministic agreement or explicit-scalar preservation.
  - `[high]` `[bad_spec]` NAT scalar/set pairs can read back a different deterministic scalar — validation accepts any member while API read-back chooses the sorted first; the spec now requires deterministic agreement or explicit-scalar preservation.
  - `[medium]` `[bad_spec]` Deprecated alias `proto` bypasses alias-type applicability checks — only `ip_protocol` was checked; the spec now applies the same rule to both inputs.
  - `[medium]` `[bad_spec]` NAT `interfaces` accepts an empty string member — cardinality alone permits a request that serializes to no interface; the spec now requires non-empty members.
  - `[medium]` `[bad_spec]` Alias refresh accepts `NaN` and infinity — `ParseFloat` succeeds for both and the positive comparison does not reject `NaN`; the spec now requires finite positive values.
  - `[medium]` `[bad_spec]` Global plain-string multi-select splitting can lose newline-delimited alias content — the shared decoder now splits only commas although alias content can be newline-delimited; the spec now requires delimiter-safe decoding plus regression coverage for other consumers.
  - `[medium]` `[bad_spec]` ICMP and TCP flag sets lack element validation — arbitrary values can reach the API despite source-backed enums; the spec now requires element validators.
  - `[low]` `[reject]` Not every advanced filter field is exercised live — representative live owner coverage plus deterministic model/HTTP assertions is sufficient for audit-row parity; exhaustive live combinations add appliance coupling without a distinct user outcome.
  - `[medium]` `[bad_spec]` Live DNAT proof omits membership update/drift — the acceptance contract names deterministic membership updates; the spec now requires a live membership update while preserving exact cleanup.
  - `[medium]` `[bad_spec]` Live filter proof omits update/drift — the acceptance contract includes update/drift behavior; the spec now requires a safe disabled-rule update/drift path.
  - `[medium]` `[bad_spec]` Dynamic IPv6 alias interface lacks positive round-trip proof — only a negative validator existed; the spec now requires direct positive request/read-back assertions.
  - `[false]` `[reject]` Acceptance result logs are not committed — `test/acceptance-results/` is intentionally ignored, the reproducible command and sanitized zero-match evidence are recorded, and committing environment-specific logs would be inappropriate.
  - `[low]` `[patch]` NAT example uses a tab before `interfaces` — formatting is directly correctable and will be regenerated during re-derivation.
  - `[high]` `[bad_spec]` Alias legacy-state collapse was independently confirmed by claims tracing — config-only planning ignores the existing canonical state; covered by the prior-state amendment.
  - `[high]` `[bad_spec]` NAT legacy-state collapse was independently confirmed by claims tracing — config-only planning ignores the existing canonical state; covered by the prior-state amendment.
  - `[medium]` `[bad_spec]` Unknown canonical alias configuration can be replaced by known legacy input — the current branch treats unknown as absent; the spec now requires unknown propagation.
  - `[medium]` `[bad_spec]` Unknown canonical NAT configuration can be replaced by known legacy input — the current branch treats unknown as absent; the spec now requires unknown propagation.
  - `[high]` `[bad_spec]` Alias deterministic scalar mismatch was independently confirmed — a compatible but non-first scalar can yield inconsistent state; covered by the scalar/set amendment.
  - `[high]` `[bad_spec]` NAT deterministic scalar mismatch was independently confirmed — a compatible but non-first scalar can yield inconsistent state; covered by the scalar/set amendment.
  - `[medium]` `[bad_spec]` Legacy alias `proto` applicability gap was independently confirmed — nonempty `proto` on unsupported alias types reaches mutation; covered by the applicability amendment.
  - `[medium]` `[bad_spec]` Non-finite alias refresh interval gap was independently confirmed — finite-value validation is now explicit.
  - `[medium]` `[bad_spec]` Malformed non-empty API numerics silently become `-1` — this erases API truth instead of surfacing a diagnostic; the spec now requires diagnostic conversion.
  - `[high]` `[bad_spec]` Alias failed recovery can persist unread planned fields — when both reconfigure and GET fail the code sets the plan plus UUID; the spec now permits identity-only recovery.
  - `[high]` `[bad_spec]` Filter failed recovery can persist unread planned fields — when both apply and GET fail the code sets the plan plus UUID; the spec now permits identity-only recovery.
  - `[high]` `[bad_spec]` DNAT failed recovery can persist unread planned fields — when both apply and GET fail the code sets the plan plus UUID; the spec now permits identity-only recovery.
  - `[medium]` `[bad_spec]` Dynamic IPv6 alias interface has no positive conversion assertion — repository searches found only the negative validator; re-derivation must add direct request/read-back coverage.
  - `[medium]` `[patch]` Filter shaper fields are present in fixtures but never asserted — the smallest fix is to assert both decoded and serialized shaper identifiers in the existing model test.
  - `[low]` `[reject]` Apply-time protocol casing retains the semantically equivalent configured spelling — ordinary refresh/import still uses API truth, the value is case-insensitively identical, and a custom semantic type would add disproportionate complexity without a functional difference.
  - `[low]` `[reject]` Alias live proof is representative rather than exhaustive — the matrix uses alternative advanced cases and lower-level tests cover conditional branches; no additional bad outcome was established beyond the positive interface assertion already retained.
  - `[low]` `[reject]` Filter live proof does not configure every advanced identifier — model/HTTP tests exercise the full mapping and live proof validates the owner lifecycle; exhaustive live fixture dependencies are not required for the audit-row reading.
  - `[medium]` `[bad_spec]` DNAT live proof does not update membership — this repeats the concrete acceptance gap and is covered by the live-update amendment.
  - `[false]` `[reject]` Apply failures are mocked rather than induced on the appliance — the intent requires observable recovery behavior, not unsafe live fault injection; HTTP lifecycle tests are the appropriate deterministic surface.
  - `[false]` `[reject]` Some preserved invariants rely on existing tests — unchanged missing-object, locking, and ownership paths are not regressions and remain covered by the repository suite.
  - `[low]` `[reject]` Audit rows use mixed unit/live evidence rather than exhaustive live combinations — the audit-row interpretation permits representative live lifecycle plus deterministic field-level evidence, and no unrepresented field remains after the retained assertion fixes.

### 2026-10-18 — Follow-up review pass
- verdicts: 36 findings — high 9, medium 19, low 4, false 4, maybe-false 0
- findings:
  - `[medium]` `[bad_spec]` GeoIP and external aliases reject valid protocol selection — pinned v0.26.0 permits `ip_protocol` for `asn`, `geoip`, and `external`; repair 2 now requires all three source-backed types.
  - `[high]` `[bad_spec]` Changing legacy alias `proto` preserves the old singleton canonical set — the request can remain on the prior protocol indefinitely; repair 2 requires scalar changes to replace singleton state while preserving multi-value state.
  - `[high]` `[bad_spec]` Changing legacy NAT `interface` preserves the old singleton canonical set — the request can remain on the prior interface indefinitely; repair 2 applies the same singleton-versus-multivalue rule.
  - `[medium]` `[bad_spec]` Unknown alias `ip_protocol` falls back to legacy `proto` during request conversion — the model collapses unresolved canonical intent; repair 2 prohibits fallback from unknown sets.
  - `[medium]` `[bad_spec]` Unknown NAT `interfaces` falls back to legacy `interface` during request conversion — the model has the same unresolved-value collapse; repair 2 prohibits it.
  - `[medium]` `[bad_spec]` Explicit empty NAT `interfaces` is accepted when scalar `interface` is set — this violates the explicit-empty-set contract; repair 2 rejects a known empty set regardless of scalar state.
  - `[medium]` `[bad_spec]` Generic `max_source_nodes` and `max_source_states` are classified as TCP-only — pinned evidence permits them on non-TCP stateful rules; repair 2 separates generic and TCP-only limits.
  - `[medium]` `[bad_spec]` TCP-only `state_timeout` lacks protocol applicability validation — pinned evidence labels it TCP-only; repair 2 requires exact source-backed applicability.
  - `[medium]` `[bad_spec]` Substring protocol checks and the absent protocol enum accept fabricated values — values such as `not-tcp` can bypass validation; repair 2 requires exact case-insensitive supported protocol values.
  - `[medium]` `[bad_spec]` ICMP type `any` is not in the pinned enum — the current validator permits an unsupported wire value; repair 2 requires the pinned set exactly.
  - `[medium]` `[bad_spec]` `match_tos` lacks its pinned finite enum — arbitrary values can reach the API; repair 2 requires source-backed validation.
  - `[high]` `[bad_spec]` Shared selected-list decoding splits all plain strings on commas and can corrupt a legitimate alias content value — field delimiters are ambiguous; repair 2 requires field-specific decoding without a global ambiguous split.
  - `[high]` `[bad_spec]` Successful create read-back or validation failure can return without UUID state — all three owners can orphan a created object; repair 2 requires identity-only state on every post-create read/validation failure.
  - `[medium]` `[bad_spec]` API numeric validation checks syntax but not resource domains — impossible sequence, priority, sentinel, or update-frequency values can enter state; repair 2 requires matching schema domains.
  - `[medium]` `[bad_spec]` GeoIP/external protocol rejection was independently confirmed by claims tracing — this shares the source-domain root cause and is covered by repair 2.
  - `[high]` `[bad_spec]` Legacy alias scalar-change no-op was independently confirmed — the current prior-state branch ignores a changed singleton scalar; repair 2 defines the replacement rule.
  - `[high]` `[bad_spec]` Legacy NAT scalar-change no-op was independently confirmed — the current prior-state branch ignores a changed singleton scalar; repair 2 defines the replacement rule.
  - `[medium]` `[bad_spec]` SCTP is treated as TCP-compatible by substring matching — `sctp` contains `tcp`; repair 2 requires exact protocol classification.
  - `[medium]` `[bad_spec]` Generic source-node/state limits are rejected on UDP — pinned descriptions establish they are not TCP-only; repair 2 separates applicability classes.
  - `[high]` `[bad_spec]` Alias successful-create read-back failure can orphan the remote object — no UUID state is written on that path; repair 2 requires identity-only recovery.
  - `[high]` `[bad_spec]` Filter successful-create read-back failure can orphan the remote object — the same missing identity path exists; repair 2 requires identity-only recovery.
  - `[high]` `[bad_spec]` DNAT successful-create read-back failure can orphan the remote object — the same missing identity path exists; repair 2 requires identity-only recovery.
  - `[medium]` `[bad_spec]` Fixed acceptance names and ports can collide across concurrent or stale runs — the intent requires unique disposable objects; repair 2 requires per-run collision-resistant identifiers while retaining safe disabled rules.
  - `[medium]` `[bad_spec]` Repository verification never executes the opt-in live parity suite — the live run is manual and normal tests do not cover all request/read-back boundaries; repair 2 requires deterministic HTTP coverage for those boundaries and explicit documentation of the manual live gate.
  - `[medium]` `[bad_spec]` Non-first legacy scalar preservation lacks resource-path tests — ModifyPlan-only coverage cannot catch read/create/update regressions; repair 2 requires HTTP resource-operation cases for alias and NAT.
  - `[medium]` `[bad_spec]` Successful partial-create recovery is untested — existing tests cover only failed recovery GET; repair 2 requires GET-success recovery cases using API-derived state.
  - `[medium]` `[bad_spec]` Framework validator registration is bypassed by direct helper tests — returning no validators would leave tests green; repair 2 requires registration/execution coverage through each resource.
  - `[medium]` `[bad_spec]` Several non-default filter mappings lack round-trip assertions — interface inversion, ICMP, schedule, overload, and reply-to can regress silently; repair 2 requires exact request and reconstructed-state assertions.
  - `[low]` `[reject]` Literal live proof omits the dynamic IPv6 alias variant — carried: representative live owner coverage plus deterministic field-level tests remains the accepted layered-proof reading.
  - `[low]` `[reject]` Literal live proof does not import an independently pre-existing filter with every advanced field — carried: exhaustive appliance combinations were already rejected in favor of representative lifecycle plus deterministic mapping tests.
  - `[false]` `[reject]` DNAT lacks out-of-band membership drift — repair 1 intentionally required and the live suite proved a deterministic membership update; the intent does not require every drift example to be external.
  - `[low]` `[reject]` Apply failures are mocked rather than induced live — deterministic HTTP failure injection is the safe observable surface and the earlier identical concern was rejected.
  - `[false]` `[reject]` Preserved invariants rely on existing tests — unchanged missing-object, locking, and ownership paths remain covered and no regression was identified.
  - `[false]` `[reject]` Exact cleanup exists only in narrative — the parent independently ran three API zero-match searches after the live suite; environment-specific evidence is intentionally recorded rather than committed.
  - `[false]` `[reject]` Scalar compatibility projections are not always sorted-first API projections — preserving an explicitly configured member is required by the compatibility contract while the canonical set remains complete API truth.
  - `[low]` `[reject]` Audit rows use representative rather than exhaustive live proof — carried: the accepted audit-row reading combines live owner lifecycle with deterministic field/HTTP evidence.

### 2026-10-18 — Final follow-up review pass
- verdicts: 35 findings — high 0, medium 12, low 10, false 13, maybe-false 0
- findings:
  - `[false]` `[reject]` The `url` alias type was removed — current `alias_schema.go` still includes `url`; the claim is disproved by the live schema.
  - `[false]` `[reject]` Newline alias decoding retains blank segments — `strings.FieldsFunc` drops empty runs and the decoder trims each retained value.
  - `[medium]` `[patch]` Unknown legacy NAT `interface` is treated as absent when `interfaces` is null — validation must defer until the computed scalar is known.
  - `[medium]` `[patch]` Unknown paired filter numerics are treated as missing — pair/range checks must defer when either participating value is unknown.
  - `[false]` `[reject]` The TOS validator accepts values outside the pinned finite enum — the pinned enum explicitly contains every hexadecimal byte `0x0` through `0xff` plus the named values implemented.
  - `[false]` `[reject]` Zero is forbidden for every state-limit field — the pinned schema supplies no positive-only validators and uses `-1` only as the unset sentinel, so zero is not disproved as an API domain value.
  - `[false]` `[reject]` The adaptive-end/max-states relationship is reversed — pinned documentation says the state limit should be lower than the adaptive end, which is exactly what the current check enforces.
  - `[false]` `[reject]` Ports require additional protocol rejection — the pinned provider supplies no source-backed port/protocol validator, and the intent says not to invent rejection rules.
  - `[false]` `[reject]` `disable_reply_to` must conflict with `reply_to` — no pinned or target API evidence establishes that combination as invalid.
  - `[false]` `[reject]` `overload_table` requires selected connection-limit fields — no source-backed validator establishes the proposed dependency.
  - `[false]` `[reject]` TCP flag sets require additional relational validation — the pinned source validates each enum member but does not impose the proposed pair relation.
  - `[low]` `[reject]` Live NAT membership proof is configuration-driven rather than out-of-band — carried: repair 1 intentionally required and proved deterministic membership update; every resource need not use the same drift mechanism.
  - `[low]` `[reject]` Only the dual-protocol alias is imported live — representative owner import plus deterministic advanced-field tests is the accepted layered-proof reading.
  - `[low]` `[patch]` New filter data-source attributes lack useful descriptions — populate concise applicability/sentinel/opaque-ID descriptions without changing behavior.
  - `[low]` `[patch]` Acceptance firmware precheck ignores non-2xx status — report bounded status/body diagnostics before decoding JSON.
  - `[low]` `[patch]` Numeric API diagnostics iterate maps nondeterministically — use fixed ordered field slices.
  - `[false]` `[reject]` Explicit empty legacy alias `proto` cannot clear filtering — empty deprecated scalar means unspecified; canonical `ip_protocol` is the lossless control and applicable aliases have a source-backed protocol default.
  - `[medium]` `[patch]` Unknown legacy alias `proto` with omitted canonical state can resolve to stale prior state — preserve unknown through compatibility planning.
  - `[medium]` `[patch]` Unknown legacy NAT `interface` can fail validation — duplicate of the verified unknown-scalar deferral gap and receives the same fix.
  - `[medium]` `[patch]` Unknown paired filter numeric can fail validation — duplicate of the verified pair-deferral gap and receives the same fix.
  - `[medium]` `[patch]` Plain scalar multi-select responses can collapse CSV into one member — add field-specific CSV decoding for selector owners while preserving comma-containing alias content.
  - `[medium]` `[patch]` Empty filter/NAT sequence responses are accepted — require nonempty sequence before conversion so malformed responses cannot silently become zero state.
  - `[medium]` `[patch]` Scalar DNAT interface responses can violate lossless membership — duplicate of the field-specific CSV decoding gap and receives the same fix.
  - `[medium]` `[patch]` Non-first compatibility scalars are tested on create but not refresh/update — add HTTP-backed alias and NAT read/update assertions.
  - `[medium]` `[patch]` Advanced filter `max_states` is present in fixtures but unasserted — assert exact decode and request encoding in the existing round-trip test.
  - `[medium]` `[patch]` Alias conditional-field requirements lack negative-path tests — add registered-validator cases for missing and inapplicable update frequency, JSON path, and dynamic interface.
  - `[medium]` `[patch]` Paired filter constraints lack negative-path tests — add cases for one-sided rate/adaptive inputs, reversed bounds, and max-states above adaptive end, including unknown deferral.
  - `[low]` `[reject]` Live proof omits the dynamic IPv6 alias variant — carried: representative appliance lifecycle plus deterministic model coverage remains the accepted interpretation.
  - `[low]` `[reject]` Live filter does not cover every named field class — carried: the accepted layered proof uses complete model/HTTP evidence plus representative live lifecycle.
  - `[low]` `[reject]` Live import uses an object created by Terraform rather than an independently pre-existing rule — import behavior and API-truth refresh are still exercised without requiring unmanaged fixture setup.
  - `[low]` `[reject]` Advanced-field drift is represented by description drift — the live suite proves owner drift/repair while deterministic tests cover the advanced mappings; no distinct untested runtime owner was shown.
  - `[false]` `[reject]` NAT membership lacks acceptable live proof — the suite passed a two-to-one canonical membership update and stable plan, satisfying the amended acceptance requirement.
  - `[false]` `[reject]` Apply failures must be induced on the appliance — deterministic mocked HTTP failure is the safe and repeatable evidence surface, as previously recorded.
  - `[low]` `[reject]` Data-source parity is asserted selectively live — resource and data source share conversion models, with complete deterministic mapping tests and representative live reads.
  - `[false]` `[reject]` Cleanup is only narrative — the parent independently verified three zero-match API searches after both final live runs; environment-specific proof is intentionally recorded rather than committed.

## Design Notes

Use additive compatibility attributes instead of changing existing public attribute types: alias `ip_protocol` is the lossless set while deprecated `proto` remains a scalar view; NAT `interfaces` is the lossless set while deprecated `interface` remains the scalar input/view. On API read-back, populate the canonical set completely and choose any scalar compatibility value deterministically. When only legacy scalar configuration is supplied, seed the canonical request set from it; once canonical set state exists, preserve all remote selections on unrelated updates. Explicitly configured conflicting values fail planning.

Advanced filter fields remain flat to avoid a breaking state/schema migration. API empty numeric values map to Terraform `-1` and back; booleans use `0`/`1`; multi-selects use sorted sets/CSV. Validators must follow current target API evidence and must not reject existing lowercase protocol aliases merely because the pinned provider documents uppercase values.

The Story 31.0a backlog phrase naming savepoint/apply/cancel-rollback is stale. Current code and pinned evidence both use one direct firewall apply call after mutation, and tests explicitly prohibit the removed endpoints. This spec preserves the actual lifecycle and updates the story evidence accordingly.

Compatibility planning must distinguish absent canonical configuration from
existing canonical state. A legacy scalar seeds a canonical set only for a new
object or genuinely absent canonical state; it never replaces an existing
multi-value state on an unrelated update, and an unknown canonical expression
remains unknown. When scalar and set are both explicit, the scalar compatibility
view must remain stable across read-after-write and refresh.

Recovery state is evidence-based: a successful read-back may populate the full
remote object, while a failed recovery read may preserve only the known UUID.
Validation rejects non-finite numbers, empty set members, and unsupported enum
members before mutation; malformed non-empty API numerics are diagnostics, not
the unset sentinel.

## Verification

**Commands:**
- `go test ./internal/service/firewall/... ./pkg/opnsense/...` -- expected: schema, conversion, HTTP-contract, lifecycle, and state-compatibility tests pass.
- `TF_ACC=1 go test -p 1 -run 'TestAccFirewall(Alias|FilterRule|NatPortForward)' ./internal/service/firewall` with disposable 26.7.7+ credentials -- expected: advanced create, read, import/no-change, update, data-source, drift repair, and exact cleanup pass.
- `go generate ./... && git diff --exit-code` -- expected: generated docs and the coverage report are current.
- `bash test/scripts/validate-structure.sh` -- expected: all constructors remain registered.
- `DEVRAIL_IMAGE=ghcr.io/devrail-dev/dev-toolchain:1.12.24 make check` -- expected: lint, format, tests, security, scan, and docs pass.
- `git diff --check` -- expected: no whitespace errors.

**Prior attempt evidence (superseded by review repair 1):** the first derivation
passed local gates and representative OPNsense 26.7.7 acceptance, but its review
found the compatibility, validation, recovery, and proof gaps recorded above.
The re-derived tree subsequently passed the local and live evidence recorded
below; the superseded first-derivation result alone did not satisfy this iteration.

**Review repair 1 live evidence:** acceptance run
`story-31-0a-live-26.7.7-review1-rerun2` passed on a disposable OPNsense 26.7.7
appliance. The run exercised advanced alias drift and repair, filter drift and
configured update, DNAT interface-membership update, resource/data-source
read-back, imports, and zero-diff stable plans. Independent post-run API
searches returned zero Story 31.0a matches for alias `searchItem`, filter
`searchRule`, and `d_nat/searchRule`. The spec frontmatter status remains for
the parent review transition.

**Review repair 2 local evidence:** `go test -count=1 ./...`, `go generate
./...`, `go test -count=1 ./internal/coverageaudit`, `bash
test/scripts/validate-structure.sh`, `DEVRAIL_IMAGE=ghcr.io/devrail-dev/dev-toolchain:1.12.24
make check`, and `git diff --check` passed on the amended tree.

**Review repair 2 live evidence:** acceptance run
`story-31-0a-live-26.7.7-review2` passed on a disposable OPNsense 26.7.7
appliance. Independent post-run searches returned zero Story31a matches for
alias, filter, and `d_nat`, proving exact cleanup of the repair-2 acceptance
objects.

**Final reviewed live evidence:** acceptance run
`story-31-0a-live-26.7.7-final-rerun1` passed on the disposable OPNsense 26.7.7
appliance after the final follow-up patches. It exercised advanced alias,
filter, and destination-NAT create/read/import/update/drift/repair/delete
lifecycle coverage, stable zero-diff plans, and resource/data-source parity.
Independent post-run API searches returned zero Story31a matches from alias
`searchItem`, filter `searchRule`, and `d_nat/searchRule`.

## Auto Run Result

- **Result:** done; the six Story 31.0a audit rows are covered by the reviewed
  implementation and disposable-appliance evidence.
- **Scope delivered:** additive alias protocol/conditional fields, advanced
  filter fields and source-backed validation, lossless multi-interface DNAT,
  symmetric data-source state, compatibility planning, generated docs, and
  deterministic audit output.
- **Review:** two required spec-repair loops completed. The converged follow-up
  pass recorded 35 findings: 12 medium, 10 low, and 13 false; 12 medium and 3
  low findings were patched, while the remaining findings were rejected with
  rationale in the review triage log. No high finding remained.
- **Verification:** focused/unit/HTTP tests, two deterministic generations,
  coverage-audit tests, structure validation, the complete DevRail
  lint/format/test/security/scan/docs gate, whitespace checks, live OPNsense
  26.7.7 acceptance, and post-run zero-match cleanup all passed.
- **Follow-up review:** not recommended; the final follow-up pass patched no
  high-severity finding and the workflow convergence rule is satisfied.
- **Residual risk:** live proof uses representative safe disabled firewall
  fixtures; complete field combinations remain covered deterministically by
  model, validator, and HTTP-contract tests rather than unsafe exhaustive live
  traffic permutations.
