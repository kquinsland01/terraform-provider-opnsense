---
title: 'Story 31.0n: Kea generic DHCPv4 option owner and lookup'
type: 'feature'
created: '2026-10-25'
status: 'done'
baseline_revision: '8472daf63808a56894e2c388f0e576d3bb2b7e1a'
review_loop_iteration: 0
followup_review_recommended: false
context:
  - 'AGENTS.md'
  - 'DEVELOPMENT.md'
  - '_bmad-output/implementation-artifacts/epic-31-context.md'
  - '_bmad-output/planning-artifacts/kea-option-ddns-api-validation.md'
warnings: [oversized]
deferred: []
---

<intent-contract>

## Intent

**Problem:** Stable core 26.7.7 exposes independently identified generic DHCPv4 options, but the provider has no Terraform owner or lookup. These objects are distinct from completed subnet option_data/PXE fields; API presence alone does not prove safe lifecycle behavior.

**Approach:** Add `opnsense_kea_dhcpv4_option` resource and UUID lookup for the eight stored option fields, preserving omitted intent and durable failure recovery. Prove real Terraform lifecycle, independent saved-state observations and exact cleanup on a disabled disposable appliance. Record parent-link ordering and force limitations rather than inventing packet-delivery or completeness claims.

## Boundaries & Constraints

**Always:** Use the official option CRUD/search and Kea service reconfigure APIs only. Gate a known stable compatible core/model before writes, serialize discover/mutate/recover with the existing client lock, preserve configured read retries and issue non-idempotent writes once. Read strict authoritative truth; keep UUID/committed values and recovery intent across failure/restart, and distinguish unavailable/malformed responses from actual absence. Omitted optional configuration preserves imported/prior values; create-only API defaults apply without turning unknown values into empty/default values. Update only required fields plus explicitly configured optional fields. Public files remain consumer-neutral; raw evidence, credentials, appliance identity and operational inputs remain private. Add the correct versioned Unreleased release input and keep live proof separate from implementation and release publication.

**Never:** Enable production or lab DHCP/DDNS traffic, copy private consumer metadata, bypass API ownership with XML writes, adopt an arbitrary same-code object, blindly retry uncertain writes, issue set for a UUID already proven absent at its preflight, discard ambiguous deletion identity or activate an unverified attempted save. Do not manage subnet/reservation links, singleton settings, option_data, DDNS settings, hooks or private integrations in this owner. Do not infer link order from selected-map key order or expose an invented generic-option position. The stored force flag is not a packet-level force guarantee. No older-core, OpenTofu, packet-delivery or production-adoption claim without separately executed proof.

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
| --- | --- | --- | --- |
| Core/model gate | Product status and unsaved option template | Strict stable core >=26.7.7 plus compatible eight-field model before mutation; actual code selector keys and finite encodings | Unknown/prerelease/old version, auth/transport or unusable template diagnoses with zero writes; template required fields may be blank, item reads may not |
| Stored fields | Required code/data/description; optional encoding/force/match fields | Code is a scalar selector key, encoding defaults to hex on create, force to false, matching to empty; data/description remain literal strings; description mutable and unique | Code must identify an actual non-pad/end template option; invalid encoding/empty required value rejected, API semantic code/data validation retained; no invented immutable description |
| Omitted/unknown intent | Import/prior optional values then omit them; unknown dependency input; explicit false/empty | Omitted values survive no-change plans and subsequent required-field updates; create defaults only for omitted attributes; explicit false/empty is written; unknown stays unknown until resolved | Never clear unknowns, never guess prior for explicit unknowns; unresolved writes fail before mutation. Do not serialize omitted optional keys on update |
| Matching | Populated code/data pair with encoding; explicit clear; encoding-only configuration | Match code/data both populated or both empty; encoding required with match data; encoding alone while code/data empty is valid. Explicit requested clears converge in place | Partial invalid pair/unknowns fail closed; do not invent all-three-or-none. Code/encoding semantic validation comes from actual stable API |
| Strict read/lookup | Eight fields, selected maps including optional empty sentinel, scalar string booleans | Selected keys, not labels; exact readable values in resource and every lookup field; no lookup activation | Missing/null/malformed fields/flags, multiple selections or invalid bools retain prior owner state; only proven actual empty item response establishes absence, not wrapper corruption/auth/in-use errors |
| Create identity | Unique description; lost/invalid add reply | Pre-search snapshot plus exactly one new UUID and complete payload match can recover a committed add; retain UUID before read/reconfigure | Same code with another description is valid. Existing description instructs import; zero/multiple/failed reconciliation fails closed with manual authoritative reconciliation, never automatic arbitrary adoption |
| Update/upsert/recovery | Existing UUID; partial wire payload; lost set response or activation failure | Authoritative presence checked before set; a proven absent UUID is diagnosed without set and requires a fresh plan. Confirmed writes retain actual state/pending activation. Unknown save persists UUID-bound requested wire fields; compare exactly those requested fields, not omitted unrelated values | Matching requested fields can activate; contradictory valid truth reconciles without another set or unverified activation and permits fresh plan. Missing/malformed truth retains uncertainty/manual reconciliation. Corrupt recovery bytes fail before mutation |
| Delete/restart | Linked option; uncertain delete; deletion saved but activation failed; restored UUID | In-use response retains UUID; present uncertain delete reconciles successfully so normal Core refresh-before-destroy retry can proceed; confirmed delete rechecks absence before dropping state | HTTP500 dependency is not missing. Reappeared UUID retained; prove deletion plus required activation. No repeated blind delete or permanent refresh-error retry loop |
| Import/drift/concurrency | UUID import, outside field changes, overlapping writes, cancellation | Same complete strict mapping, exact no-change import, explicit drift repair, globally serialized mutation and canceled-before-write zero mutation | Preserve unrelated options and omitted fields; valid authoritative drift is not malformed input; no shared transport mutation or nested-lock deadlock |
| Appliance acceptance | Disabled isolated Kea, synthetic options and harness-owned dependencies | Real Core create/import/no-change/updates/lookup/drift/delete, independent saved-field observations and dependency rejection on tracked UUID; exact config/routes/runtime baseline restore and zero test entries before/after rollback | Never count skipped, failed or rollback-only proof as success. Installed-model generation to a private artifact can observe order with daemon inactive; it is not ordinary disabled-service generation or packet delivery |

</intent-contract>

## Code Map

- Repository root is the directory enclosing `_bmad-output`; context paths and paths below are repository-relative. `internal/service/kea/exports.go` registers ten existing resources; add one resource and matching data source, not a duplicate subnet/settings owner.
- `internal/service/kea/dhcpv4_subnet_{resource,model,schema}.go` and `dhcpv4_reservation_model.go` show existing names/import patterns and targeted nested ownership, but neither exposes the upstream generic `option` relation. Their shared retrying CRUD and loose `pending_activation.go` marker are insufficient for this story's single-write/uncertain-save requirements.
- `internal/service/system/gateway_group_lifecycle.go:70-112,303-373` and `gateway_group_resource.go:100-184,186-389` are continuity references for copied authenticated transport/policy, unique-key snapshots, strict response parsing, attempted payload binding, delete truth and fresh-plan boundaries. Adapt principles locally: option updates send a subset and compare requested fields; do not copy a nine-field/full-map assumption or gateway tier eligibility poll. Do not alter gateway behavior.
- `internal/service/system/gateway_group_core_recovery_test.go` demonstrates actual Terraform UnitTest, fresh protocol servers and a private-byte replay observer. `gateway_group_protocol_test.go` covers wrong/missing/malformed controls, all lookup fields and unsupported-create zero writes. Reuse coverage principles, not unchecked helper-only state.
- Tagged core [Dhcpv4Controller](https://github.com/opnsense/core/blob/26.7.7/src/opnsense/mvc/app/controllers/OPNsense/Kea/Api/Dhcpv4Controller.php): wrapper `option`, `options.option`, `get/add/set/del/search_option`; service reconfigure `/api/kea/service/reconfigure`. Inherited setBase is UUID-preserving upsert; safe-delete checks model references. Actual missing get returns PHP `[]`, while an in-use delete was HTTP500.
- Tagged [KeaDhcpv4 model](https://github.com/opnsense/core/blob/26.7.7/src/opnsense/mvc/app/models/OPNsense/Kea/KeaDhcpv4.xml): eight fields, code keys1-254 from configd catalog, ten encodings, mutable unique required description. Actual template has flattened code entries with optgroup/value metadata and numeric selected flags; optional match selectors include the selected empty-key None sentinel.
- Tagged [KeaOptionDataField](https://github.com/opnsense/core/blob/26.7.7/src/opnsense/mvc/app/models/OPNsense/Kea/FieldTypes/KeaOptionDataField.php): known codes constrain typed encoding, hex is fallback; code15 accepts fqdn/hex, not string. Preserve opaque textual data and ordering: runtime hex/IP/integer encoding does not normalize GET values. Match constraints permit encoding-only. Preserve API validation instead of duplicating the entire encoding engine or strengthening its acceptance accidentally.
- Tagged [KeaDhcpv4 generator](https://github.com/opnsense/core/blob/26.7.7/src/opnsense/mvc/app/models/OPNsense/Kea/KeaDhcpv4.php): same-code flex options are last-match-wins; subnet actions precede reservations. `ModelRelationField` GET choices are label-sorted, not persisted CSV order. The generator does not consume force. `kea.inc` skips ordinary config generation while DHCP is disabled; parent observed installed-model generation to a private disposable file without enabling services.
- `templates/{resources,data-sources}/`, matching examples, generated docs, support/index, coverage audit, story/sprint/epic and versioned changelog are public inputs. `test/scripts/gateway_group_pinned_proxy.py` is an already-tested acceptance-only fixed-upstream TLS-pin primitive; reuse without weakening existing runner or exposing a new provider setting.

## Tasks & Acceptance

**Execution:**
- `internal/service/kea/dhcpv4_option_{resource,schema,model,data_source,lifecycle}.go`, `exports.go`, schema tests -- add strict typed owner/lookup, import and mutable unique description. Optional/computed semantics must preserve omissions after import/update and provide known create-only defaults, without unknown-to-empty coercion. Required code/data/description plus configured optional fields form update wire intent; strict pre-read prevents silent absent-UUID upsert. Validate field-specific failures and all recovery data locally; do not broaden shared CRUD.
- `internal/service/kea/dhcpv4_option_*test.go` -- named unit/protocol controls for every matrix row, every computed lookup field, actual marker generation, mismatching UUID/payload/corrupt private bytes, configured GET retries versus single POST, canceled operations and serialization. Add actual local Core failure/state/private persistence and normal refresh/plan/apply/destroy tests; direct protocol replay alone does not prove Core recovery. Test removing optional config after populated values and importing omitted fields without changes, explicit false and matching clears, unknown inputs and omitted wire keys. Ensure acceptance-helper security regressions are registered in the normal test path, not only orphaned manual test files.
- `internal/service/kea/dhcpv4_option_acceptance_test.go`, `test/scripts/run-story-31-0n-acceptance.sh`, `test/scripts/story-31-0n-code-digest.sh` and minimal runner helpers -- supply real Core fixture and parent guard/oracle interface. Require pre-execution independently expected helper hashes, per-actual-connection certificate pin before credentials, immutable source/helper inputs, full default/appliance/runtime baseline, independent all-field observations, exact no-change/import, optional-preservation/matching clears, drift/repair and same-UUID in-use delete. Hash every executable/helper and publication claim input; keep final evidence outside digest to avoid recursive self-hash. Parent supplies all runtime secrets/VM actions privately; implementer does not provision, access production or put raw evidence into the tree.
- `templates/resources/kea_dhcpv4_option.md.tmpl`, `templates/data-sources/kea_dhcpv4_option.md.tmpl`, `examples/resources/opnsense_kea_dhcpv4_option/{resource.tf,import.sh}`, `examples/data-sources/opnsense_kea_dhcpv4_option/data-source.tf`, generated `docs/{resources,data-sources}/kea_dhcpv4_option.md` -- exact permissions/minimum model/core gate, mutable unique description, scalar code/encoding/data semantics, defaults/omission/clear rules, recovery/manual reconciliation, force limitation and unmodeled ordered parent linkage. No invented ACL identifier, traffic/Foreman hook or older/OpenTofu claim.
- `CHANGELOG.md`, `_bmad-output/implementation-artifacts/31-0n-kea-generic-option-resource.md`, `_bmad-output/implementation-artifacts/sprint-status.yaml`, `_bmad-output/implementation-artifacts/epic-31-context.md`, `templates/index.md.tmpl`, generated `docs/index.md`, `_bmad-output/planning-artifacts/support-matrix.md`, generated `_bmad-output/planning-artifacts/provider-ecosystem-coverage-audit.{json,md}` and separate final sanitized evidence -- deterministic generation and correct counts; only promote executed proof. Append to current unreleased heading initially; parent must reconcile pending gateway PR merge and actual published tags before final publication so an existing tag cannot skip release. Do not manually tag or count an unmerged base as shipped. Register discovered ordered parent-linkage follow-up31.0p without implementing that separate owner here.

**Acceptance Criteria:**
- Given valid synthetic HCL on a supported disabled disposable appliance, when real Core creates, imports, updates, reads lookup and refreshes, then every owned field and UUID converge and subsequent plans have no changes.
- Given omitted versus explicitly false/empty/unknown optional intent, when plans and mutations execute through Core, then omitted values and unrelated objects survive, explicit supported clears converge, and unresolved unknowns cause no writes.
- Given saved/uncertain mutations and activation/read failures, when actual returned state/private bytes reach fresh provider servers through Core, then committed identity remains accurate, requested-field reconciliation avoids unintended activation/duplication and normal destroy retry remains possible. Document unresolved identity/truth manual paths, not fictitious automatic recovery.
- Given completed reviewed publication candidate, when all offline/hosted and non-skipped real Core gates plus real deletion/independent cleanup finish, then the provider change is reviewable with sanitized exact-tree proof and zero test options/dependencies plus exact configuration/routes/runtime restoration. Production remains untouched; parent-link ownership and active packet delivery remain separate.

## Spec Change Log

## Review Triage Log

### 2026-10-25 — Review pass
- verdicts: 18 findings — high 3, medium 10, low 2, false 3, maybe-false 0
- review execution: three context-free nonimplementing reviewers completed blind/edge/gap layers. Fourth fresh spawn was rejected at the runtime thread limit; the gap reviewer was reused for descriptive alignment only. All four layers were collected before classification or edits. No reviewer wrote implementation.
- findings:
  - `[medium]` `[patch]` G1 create read-back failure regression absent — Pre-verified gap: existing protocol create case fails only activation after successful read-back; add confirmed-create bad-read restart coverage together with actual Core failed-create persistence tests. Applied: confirmed-create bad-read protocol regression and actual Core failed-create private persistence/refresh/destroy cases pass.
  - `[high]` `[patch]` B1 add reply can adopt a pre-existing UUID — createDHCPv4Option returns any syntactically valid reply UUID before consulting its pre-add snapshot; Create persists that identity before mismatch detection. Reject pre-existing reply IDs and reconcile only new complete matches. Applied: pre-existing response UUIDs, including case aliases, cannot become owned; only a new full-payload match reconciles. Named unrelated-UUID regression passes.
  - `[high]` `[patch]` B2 discovery follows redirects and re-injects credentials — optionSnapshot calls shared Search, whose default HTTP client follows redirects; apiKeyTransport injects Basic Auth on every redirected request. Use the already redirect-rejecting copied requester locally without weakening pagination or changing shared transports. Applied: local strict paginated snapshots use the copied redirect-rejecting requester, preserving read retries; the credential-sink test observes zero requests and zero writes.
  - `[medium]` `[patch]` B3 ambiguous recovery JSON encodings are accepted — The standard struct decoder accepts duplicate keys and case aliases despite DisallowUnknownFields. The producer emits canonical JSON; validate that internal canonical representation and cover ambiguous corrupt bytes before activation. Applied: exact canonical recovery bytes are required; duplicate keys, case aliases, duplicate payload keys and whitespace corruption fail before mutation.
  - `[low]` `[reject]` B4 invalid matching literals are diagnosed at apply rather than planning — Known invalid code/pair/encoding is rejected by values/merged validation before any write; plan-time rejection was not promised. Additional cross-field validation complexity for invalid configurations is not justified by this negligible timing-only outcome.
  - `[medium]` `[patch]` B5 changed helper can execute cleanup before late hash check — run_oracle executes the path repeatedly during cleanup and hashes only afterward. Revalidate independently expected helper integrity immediately before every invocation and add a mid-run mutation security regression. Applied: expected hashes are rechecked before each guard/oracle call including cleanup; mid-Core helper mutation never executes changed cleanup.
  - `[medium]` `[patch]` B6 in-use proof accepts any matching delete diagnostic — ExpectError checks generic deletion text; the subsequent check preserves identity but does not capture actual Core delete status before unlinking. Record exact synthetic DELETE response method/path/status in the pinned acceptance proxy and assert HTTP500 plus immediate independent same-UUID/all-field truth before removing the reference. Applied: exact tracked POST HTTP500 evidence and immediate independent eight-field unchanged truth are required before unlink; repaired live lifecycle passes.
  - `[medium]` `[patch]` B7 failed-create behavior is missing from actual Core persistence matrix — The Core failure matrix starts with successful create. Extend it to confirmed-create bad-read and activation failures with fresh servers/private replay and ordinary refresh/destroy, without overriding Terraform taint/replacement semantics. Shared root with G1. Applied: actual Core bad-read/activation failed-create cases retain UUID/private payload, preserve normal taint planning, refresh through fresh servers and destroy with one add total.
  - `[medium]` `[patch]` B8 cancellation after a committed write is untested — Existing cancellation test cancels before start only. Add update/delete committed-before-response cancellation, then fresh-server recovery checking retained state/private intent and no blind repeated mutation. Applied: committed save/delete cancellation before response retains UUID-bound attempted intent; fresh-server recovery performs no repeated mutation.
  - `[medium]` `[patch]` B9 CLI checks for new acceptance files are manual only — Go-only DevRail currently registers Python security regressions but does not enforce shellcheck/shfmt/example fmt for these added files. Register the existing small CLI checks in the normal Go test gate. Applied: normal Go tests enforce literal shellcheck/shfmt/Terraform example checks; full gate passes. Lint construction corrections use bounded root file access and literal commands, without suppression.
  - `[false]` `[reject]` B10 live acceptance must become an executable CI release gate — No releasable merge/publication has occurred; parent already executed the exact-tree 14-step Core run, real destroy, zero-before/after and restoration on digest3114c7a...4d913. Final repaired proof/public evidence remains a pre-publication gate. The intent does not require public CI to provision a private appliance.
  - `[high]` `[patch]` E1 discovery cross-origin redirect leaks credentials — Same new Search call and Basic Auth injection path as B2; verified by reading shared searchPage and apiKeyTransport callers. Keep separate verdict row, shared redirect fix root. Applied: same local redirect-rejecting snapshot fix and zero-request credential-sink regression as B2.
  - `[medium]` `[patch]` A1 installed-appliance truth is not yet in the public diff — Alignment observation is accurate about the reviewed artifact. Parent has privately verified the initial exact-tree 14-step proof; add sanitized final candidate evidence before publication rather than substituting synthetic tests. Applied: sanitized initial/repaired evidence records actual installed-model lifecycle and exact cleanup; final publication digest result will be appended separately.
  - `[medium]` `[patch]` A2 independent oracle results are only an interface in the diff — The initial private run contains16 independent eight-field observations, six zero counts before/after and exact settings/config/routes/runtime restoration plus unchanged six-input private closure. Publish sanitized final digest-bound evidence, never raw configuration or private inputs. Same documentation handoff root as A1. Applied: repaired lifecycle has 17 independent eight-field observations, exact tracked HTTP500 then successful delete, three-namespace zeros before cleanup/rollback and after restoration, and unchanged private helper closure.
  - `[medium]` `[patch]` A3 story still records the live gate as open — This was honest implementation-stage wording. Update final reviewed story/support/claims after the repaired frozen candidate passes, retaining earlier run identities separately. Same final-evidence root as A1. Applied: story/support/index now record independently executed lifecycle while retaining merge, hosted-CI, release and adoption gates.
  - `[false]` `[reject]` A4 stored ownership versus effective DHCP delivery — Reviewer confirms agreement: parent order/linkage and packet-level force/delivery remain explicitly separate, not a defect.
  - `[false]` `[reject]` A5 release-input placement is not publication proof — Reviewer confirms the deliberate separation. Current top0.9.0 heading has no published tag; gateway PR72 remains unmerged. Actual base/tag reconciliation is an explicit parent pre-publication dependency, not a claim that release shipped.
  - `[low]` `[patch]` P1 copied gateway oracle log prefix misidentifies Kea proof — The new acceptance helper emits GATEWAY_GROUP_ORACLE for Kea actions. Directly correct the literal prefix, with no new branch or public setting. Applied: oracle proof uses KEA_OPTION_ORACLE prefix.
- grouping: 10 bounded roots (2 high,7 medium,1 low). The same implementation worker owns9 focused code/test/helper fixes; parent owns the sanitized final-evidence/metadata root. No intent change, new provider setting, scanner suppression or production action.

## Design Notes

The generic owner is not effective delivery by itself: existing subnet/reservation
resources cannot yet express generic option links, and their ordered behavior is
not recoverable from selected-map key order. Preserve these relations, prove the
in-use guard with harness-owned official-API fixtures, and expose the remaining
gap rather than manufacturing an option position or a second subnet owner.

Omitted optional attributes are not an instruction to clear a prior/imported
value. Explicit unknown input is not permission to substitute a previous value.
On update, uncertain-write matching covers only requested wire fields; unrelated
omitted-field drift must not falsely reject an otherwise matching committed save.
Data/description compare literally, never as CSV sets or runtime-encoded bytes.

## Verification

- All tests, Terraform, linters, generation and scanners run inside cached `ghcr.io/devrail-dev/dev-toolchain:1.12.24` with Docker `bash -c`; no host installs or host test execution.
- Focused uncached/race Kea tests including named protocol and local Core recovery cases; parent fresh full `go test -count=1 ./...`, `go generate ./...` twice with identical digest, `make check` all six targets, `git diff --check`.
- Matrix audit requires a passing non-skipped test for each row at its stated surface. Parent executes identity/version/TLS/digest-bound live Core proof and independently verifies fields, services inactive, actual deletion, zero tagged entries before and after rollback, full config/routes/runtime baseline and unchanged sources/helpers. Failed attempts remain separate.
- Parent inspects exact publication diff/PR body for private information and scans final files/history. Reconcile the stacked base/release input, then verify hosted checks on the actual PR head; local success is not hosted or release success.

The upstream controller has no conditional-write transaction: the presence
preflight must not be described as an atomic lock against out-of-band appliance
edits. The provider serializes its own client mutations and never knowingly
sets a UUID its preflight found absent.

### 2026-10-25 — Bounded follow-up review
- verdicts: 0 findings — high 0, medium 0, low 0, false 0, maybe-false 0
- Retained nonimplementing edge/gap reviewers independently inspected the repaired ownership, transport, recovery, Core persistence and acceptance-helper paths. Both returned no findings; source inspection is not represented as independent test execution. Reviewer reuse was disclosed after the runtime's fresh-thread limit. No new forced-floor review or scope expansion.

## Auto Run Result

Implemented the independent eight-field generic DHCPv4 option owner and UUID lookup, strict stable-model gates, omission-preserving planning, single-write reconciliation and durable UUID-bound recovery. No production, parent-link or packet-delivery ownership was added.

### Review disposition

Initial 18 individual verdicts and their evidence remain above: 14 patched observations grouped into 10 roots (2 high, 7 medium, 1 low); no deferred entries. Rejected B4 is a negligible diagnostic-timing distinction requiring extra complexity; B10 incorrectly requires private appliance provisioning in public CI; A4 and A5 confirm intentionally separate delivery/publication gates. The follow-up patched counts are high 0, medium 0, low 0; `followup_review_recommended: false` because bounded independent rechecks found no remaining unverified repair risk.

### Verification and remaining boundaries

Parent fresh full Go tests, uncached Kea race tests, two identical generations, shellcheck/shfmt/example formatting, six quality targets and the 40-file directory secret scan passed. The initial repaired full gate exposed two harness lint constructions; bounded-root file access and literal commands corrected them without suppression. Its failed result remains separate. The repaired immutable lifecycle passed all 14 Core steps, 17 independent eight-field observations, exact tracked HTTP500, real destroy, zero test objects and exact config/settings/routes/runtime restoration. See `31-0n-kea-option-live-evidence.md` for immutable run digests and final publication-tree proof.

Gateway PR #72 is an unmerged local base dependency at this checkpoint. The top 0.9.0 release input has no published tag; before public PR handoff, reconcile the actual upstream merge and current published tags, then verify hosted checks on that head. This completed local story is not a claim that the owner is merged, released or adopted. Ordered linkage (31.0p), DDNS (31.0o), active packet delivery, older-core/OpenTofu coverage and operator migration remain separate.

### Files changed

- `CHANGELOG.md` — Versioned unreleased feature and compatibility/limitation notes.
- `_bmad-output/implementation-artifacts/31-0n-kea-generic-option-resource.md` — Consumer-neutral implementation, verification, inventory or follow-up tracking.
- `_bmad-output/implementation-artifacts/31-0n-kea-option-live-evidence.md` — Consumer-neutral implementation, verification, inventory or follow-up tracking.
- `_bmad-output/implementation-artifacts/31-0p-kea-ordered-option-linkage.md` — Consumer-neutral implementation, verification, inventory or follow-up tracking.
- `_bmad-output/implementation-artifacts/epic-31-context.md` — Consumer-neutral implementation, verification, inventory or follow-up tracking.
- `_bmad-output/implementation-artifacts/spec-31-0n-kea-generic-option-owner.md` — Consumer-neutral implementation, verification, inventory or follow-up tracking.
- `_bmad-output/implementation-artifacts/sprint-status.yaml` — Consumer-neutral implementation, verification, inventory or follow-up tracking.
- `_bmad-output/planning-artifacts/kea-option-ddns-api-validation.md` — Consumer-neutral implementation, verification, inventory or follow-up tracking.
- `_bmad-output/planning-artifacts/post-release-epics.md` — Consumer-neutral implementation, verification, inventory or follow-up tracking.
- `_bmad-output/planning-artifacts/provider-ecosystem-coverage-audit.json` — Consumer-neutral implementation, verification, inventory or follow-up tracking.
- `_bmad-output/planning-artifacts/provider-ecosystem-coverage-audit.md` — Consumer-neutral implementation, verification, inventory or follow-up tracking.
- `_bmad-output/planning-artifacts/support-matrix.md` — Consumer-neutral implementation, verification, inventory or follow-up tracking.
- `docs/data-sources/kea_dhcpv4_option.md` — Public consumer-neutral schema, example, recovery/compatibility guidance or generated documentation.
- `docs/index.md` — Public consumer-neutral schema, example, recovery/compatibility guidance or generated documentation.
- `docs/resources/kea_dhcpv4_option.md` — Public consumer-neutral schema, example, recovery/compatibility guidance or generated documentation.
- `examples/data-sources/opnsense_kea_dhcpv4_option/data-source.tf` — Public consumer-neutral schema, example, recovery/compatibility guidance or generated documentation.
- `examples/resources/opnsense_kea_dhcpv4_option/import.sh` — Public consumer-neutral schema, example, recovery/compatibility guidance or generated documentation.
- `examples/resources/opnsense_kea_dhcpv4_option/resource.tf` — Public consumer-neutral schema, example, recovery/compatibility guidance or generated documentation.
- `internal/service/kea/data_source_schema_test.go` — Owner/lookup lifecycle, schema, registration or named protocol/Core/acceptance regression coverage.
- `internal/service/kea/dhcpv4_option_acceptance_test.go` — Owner/lookup lifecycle, schema, registration or named protocol/Core/acceptance regression coverage.
- `internal/service/kea/dhcpv4_option_core_recovery_test.go` — Owner/lookup lifecycle, schema, registration or named protocol/Core/acceptance regression coverage.
- `internal/service/kea/dhcpv4_option_data_source.go` — Owner/lookup lifecycle, schema, registration or named protocol/Core/acceptance regression coverage.
- `internal/service/kea/dhcpv4_option_helper_security_test.go` — Owner/lookup lifecycle, schema, registration or named protocol/Core/acceptance regression coverage.
- `internal/service/kea/dhcpv4_option_lifecycle.go` — Owner/lookup lifecycle, schema, registration or named protocol/Core/acceptance regression coverage.
- `internal/service/kea/dhcpv4_option_model.go` — Owner/lookup lifecycle, schema, registration or named protocol/Core/acceptance regression coverage.
- `internal/service/kea/dhcpv4_option_model_test.go` — Owner/lookup lifecycle, schema, registration or named protocol/Core/acceptance regression coverage.
- `internal/service/kea/dhcpv4_option_protocol_test.go` — Owner/lookup lifecycle, schema, registration or named protocol/Core/acceptance regression coverage.
- `internal/service/kea/dhcpv4_option_resource.go` — Owner/lookup lifecycle, schema, registration or named protocol/Core/acceptance regression coverage.
- `internal/service/kea/dhcpv4_option_schema.go` — Owner/lookup lifecycle, schema, registration or named protocol/Core/acceptance regression coverage.
- `internal/service/kea/exports.go` — Owner/lookup lifecycle, schema, registration or named protocol/Core/acceptance regression coverage.
- `internal/service/kea/parity_model_test.go` — Owner/lookup lifecycle, schema, registration or named protocol/Core/acceptance regression coverage.
- `templates/data-sources/kea_dhcpv4_option.md.tmpl` — Public consumer-neutral schema, example, recovery/compatibility guidance or generated documentation.
- `templates/index.md.tmpl` — Public consumer-neutral schema, example, recovery/compatibility guidance or generated documentation.
- `templates/resources/kea_dhcpv4_option.md.tmpl` — Public consumer-neutral schema, example, recovery/compatibility guidance or generated documentation.
- `test/scripts/kea_option_pinned_proxy.py` — Guarded digest-bound acceptance, pinned transport/status evidence or helper-security regression.
- `test/scripts/kea_option_pinned_proxy_test.py` — Guarded digest-bound acceptance, pinned transport/status evidence or helper-security regression.
- `test/scripts/kea_option_runner_guard_test.py` — Guarded digest-bound acceptance, pinned transport/status evidence or helper-security regression.
- `test/scripts/run-story-31-0n-acceptance.sh` — Guarded digest-bound acceptance, pinned transport/status evidence or helper-security regression.
- `test/scripts/run-story-31-0n-container.sh` — Guarded digest-bound acceptance, pinned transport/status evidence or helper-security regression.
- `test/scripts/story-31-0n-code-digest.sh` — Guarded digest-bound acceptance, pinned transport/status evidence or helper-security regression.
