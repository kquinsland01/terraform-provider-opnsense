---
title: 'Story 31.0c: OpenVPN Field and Key Parity'
type: 'feature'
created: '2026-10-18'
status: 'in-review'
baseline_revision: 'ee8c03309725b355a0ac7e1357f1060a1664b8e4'
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

**Problem:** The provider's OpenVPN instance and client-overwrite owners omit audited configuration fields, the static-key lookup is missing, and generated keys have no ephemeral Terraform surface. This prevents lossless import/migration parity and tempts operators to persist generated secret material.

**Approach:** Extend the existing OpenVPN owners additively, add a sensitive static-key data source, and implement a Terraform 1.10+ ephemeral key generator against the source-proven OPNsense 26.7.6+ contract. Preserve current HCL/state compatibility, use strict owner-specific wire decoding and affirmative lifecycle results, and close audit rows only after digest-bound disposable-appliance proof on 26.7.7.

## Boundaries & Constraints

**Always:** Preserve the existing public names/types (`vpnid`, `ca`, `cert`, `auth`, scalar `remote`, string `port`) and their import behavior; model all audited fields for both resource and data-source reads; distinguish unknown, null, and explicit-empty collections; mark `password`, `auth_gen_token_secret`, and all returned/generated key material sensitive; require HTTP success plus source-proven result/identity semantics; preserve recoverable identity/state and retry activation after CRUD succeeded but OpenVPN reconfigure/read-back failed; runtime-gate the expanded surface to OPNsense `>=26.7.6`; use `POST /api/openvpn/instances/gen_key/{key_type}` with exactly `secret`, `auth-token`, `tls-auth`, `tls-crypt`, and `tls-crypt-v2-server`; never log secret request/response bodies; make live fixtures disabled, collision-tagged, disposable, and cleanup-verifiable.

**Never:** Rename or change existing attributes in place; copy the pinned baseline's broken `GET ...?type=` generator call or its unsupported `tls-crypt-v2-client`; treat HTTP 200, malformed selected maps, empty keys, ambiguous CRUD results, or failed reconfigure as success; silently coerce malformed numeric/selection data; collapse unknown values into clears; persist generated keys in Terraform plan/state/evidence; claim a normal sensitive data-source value is non-persistent; broaden permissive shared decoders in a way that changes unrelated owners; mark audit/support rows covered before final live proof.

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
|----------|---------------|----------------------------|----------------|
| Advanced instance/overwrite import | Existing 26.7.7 object with audited scalar, set/list, IPv6, routing, bridge, token, credential, DNS/NTP and flag fields | Refresh is lossless; matching configuration plans zero change; existing local aliases/types remain valid | Diagnose malformed/ambiguous API values without overwriting usable prior state |
| Collection update/clear | Known populated, explicit-empty, null, or partially unknown collection | Known values round-trip in source-defined order/shape; explicit empty clears; omitted/unknown does not become a clear | Defer or diagnose unknown input; never issue a destructive fallback payload |
| Secret refresh | Instance secret is returned, blanked, or omitted by the appliance | Sensitive value is never logged; source-proven read-back behavior yields stable plans without fabricating secret truth | Preserve recoverable prior/configured value only where the framework contract permits; diagnose contradictory shapes |
| CRUD then reconfigure failure | Add/set/delete is saved, activation or read-back fails | State retains/reconstructs identity and a later apply retries activation without duplicate creation or lost deletion intent | Return a precise diagnostic while preserving recovery state |
| Static-key lookup | Valid static-key UUID | Read-only GET returns mode, description, and sensitive key; no mutation/reconfigure occurs | Missing/malformed object and non-2xx are deterministic errors with no key leakage |
| Ephemeral generation | Valid known `key_type` | One POST to the path-segment endpoint returns exact `result=ok` and non-empty sensitive key only in ephemeral result | Unsupported/unknown type, non-2xx, malformed JSON, failed/ambiguous result, or blank key diagnoses without fallback |
| Ephemeral persistence proof | Each generated key is captured only in memory while a write-only sink consumes it | Every exact generated key is absent from saved plan/state and sanitized evidence/logs; Terraform may retain non-secret resource-address metadata | Fail the acceptance run if any generated key value appears in a persisted artifact |
| Version/safety gate | Appliance older than 26.7.6, wrong VM identity, enabled production-like object, or dirty fixture namespace | No mutation occurs | Fail before mutation and retain cleanup diagnostics |

</intent-contract>

## Code Map

- `internal/service/openvpn/instance_schema.go:21` -- extend the existing resource schema additively; preserve aliases and scalar/string compatibility; apply source-backed domains and cross-field validation.
- `internal/service/openvpn/instance_model.go:16` -- extend state/wire models for all audited fields; replace silent collection/numeric coercion on new paths with diagnostic, field-specific encode/decode while retaining legacy compatibility.
- `internal/service/openvpn/instance_resource.go:22` -- existing endpoint owner; add affirmative OpenVPN reconfigure and partial-create/update/delete recovery without changing unrelated generic CRUD behavior.
- `internal/service/openvpn/instance_data_source.go:27` -- mirror the complete resource read surface through the shared model/decoder.
- `internal/service/openvpn/client_overwrite_schema.go:18` and `client_overwrite_model.go:14` -- add IPv6, DNS/search/NTP/WINS, redirect/register-DNS and route-gateway parity with explicit-clear and strict selected-map behavior.
- `internal/service/openvpn/client_overwrite_resource.go:22` and `client_overwrite_data_source.go:27` -- preserve the stable `cso` lifecycle and shared full-field read model with recovery/activation semantics matching the instance owner.
- `internal/service/openvpn/static_key_model.go:14` and `static_key_resource.go:22` -- reuse the source-proven `statickey` GET model/endpoint; do not use search because it exposes description only.
- `internal/service/openvpn/static_key_data_source.go` -- new UUID lookup with required `id`, computed `mode`/`description`, computed+sensitive `key`, strict missing/malformed handling, and no reconfigure.
- `internal/service/openvpn/generate_key_ephemeral.go` -- new `ephemeral.EphemeralResource`/`WithConfigure`; validate source-backed types and strictly decode `POST /api/openvpn/instances/gen_key/{type}` without logging bodies.
- `internal/service/openvpn/exports.go:12` -- register the static-key data source and OpenVPN ephemeral factory.
- `internal/provider/provider.go:45,105,232` -- assert `ProviderWithEphemeralResources`, pass `EphemeralResourceData`, and expose ephemeral constructors while preserving resource/data-source configuration.
- `internal/coverageaudit/audit.go:48,278` -- inventory/validate ephemeral registrations and update expected local surface counts; do not special-case an unregistered claim.
- `internal/service/openvpn/*_test.go` -- focused schema, model, HTTP, strict-decoder, validation, recovery, registration, and generated-key non-persistence tests; cover every matrix edge.
- `internal/service/openvpn/*_test.go` must drive both resource owners through create/update/read/delete failure injection, not only helper functions: recover committed adds from an owner-specific collision search after malformed success, retain pending deletes after ambiguous success, and prove a successful pending reconfigure is not followed by a repeated mutation. Cover every audited scalar at a non-default value, both cipher attributes, partially unknown credentials, strict UUID lookup, product identity, stable-release parsing, and field-preserving import.
- `internal/acctest/` and OpenVPN acceptance files -- reuse provider/live harness patterns while adding version, independently verified VM name/UUID, collision, complete executable/dependency digest, exact test PASS, parsed cleanup-count, skip/non-skip, and secret-redaction attestation. The digest must include provider/acctest sources and `go.mod`/`go.sum`, and evidence must be derived from the actual required PASS and cleanup lines rather than hard-coded claims.
- The OpenVPN version gate is feature-selective: legacy configurations and state using only the pre-existing owner surface must retain their former refresh/update/delete compatibility on older supported appliances, while any newly audited field, strict owner behavior, static-key data source, or ephemeral key generation must fail before mutation below stable OPNsense 26.7.6. Product identity must be OPNsense, and prerelease/devel suffixes do not satisfy the stable minimum.
- New Optional+Computed audited scalars must not use schema defaults that overwrite imported non-default API truth when omitted. Preserve imported values, make `data_ciphers` and `data_ciphers_ordered` agree or conflict deterministically, and reject known username with null/unknown password rather than encoding an empty secret.
- `internal/service/openvpn/static_key_data_source.go` validates a non-empty UUID before any request.
- Ephemeral acceptance must use Terraform CLI/provider protocol with a real `ephemeral "opnsense_openvpn_generate_key"` block and an ephemeral-compatible/write-only consumption path. It must prove provider configuration handoff, all five key types, and absence of every exact generated key value from saved plan, state, logs, and committed evidence; direct Go-owner invocation is supplemental only.
- `templates/{resources,data-sources,ephemeral-resources}/`, `examples/`, `docs/` -- document complete fields, Terraform 1.10+ ephemeral use, normal data-source state persistence, and the restriction that ephemeral values cannot feed ordinary persisted attributes.
- `_bmad-output/planning-artifacts/provider-ecosystem-coverage-audit.json` -- canonical six affected missing rows; promote only after owner-specific final proof, then regenerate Markdown.
- `_bmad-output/planning-artifacts/support-matrix.md`, `_bmad-output/planning-artifacts/provider-management-roadmap-2026-09-28.md`, and `_bmad-output/implementation-artifacts/sprint-status.yaml` -- synchronize final evidenced status without overstating release/merge state.
- OPNsense core `f1a83f72...` `OpenVPN.xml`, `OpenVPN.php`, `InstancesController.php`, `ClientOverwritesController.php`, `ServiceController.php`, `Router.php`, and `Dispatcher.php` -- read-only contract for field domains, cross-field validation, CRUD roots/results, separate reconfigure, path-argument generator types, and secret read-back.
- Baseline `browningluke/opnsense` v0.26.0 at `33c2a03...` -- read-only semantic inventory only; its query-string generator and key-type validator are known-bad and must not be copied.

## Tasks & Acceptance

**Execution:**
- `internal/service/openvpn/{instance,client_overwrite}_*.go` -- implement the full audited field surface, compatibility-safe conversions, validation, strict reads, and recoverable lifecycle activation.
- `internal/service/openvpn/static_key_data_source.go` and `generate_key_ephemeral.go` -- add the read-only sensitive lookup and non-persistent source-correct key generator.
- `internal/service/openvpn/exports.go` and `internal/provider/provider.go` -- wire the new data-source/ephemeral surfaces and provider data channel.
- `internal/service/openvpn/*_test.go`, provider tests, and acceptance harness -- prove matrix behavior, exact HTTP contracts, state safety, feature-selective version compatibility, every audited field at non-default import/read values, owner-level create/update/delete recovery, malformed-success identity recovery, import/no-diff/update/drift, actual Terraform ephemeral protocol/non-persistence, and dependency-ordered cleanup.
- `templates/`, `examples/`, `docs/`, audit generator/data, roadmap, sprint status, and story evidence -- regenerate and reconcile public/planning artifacts only from final proof.

**Acceptance Criteria:**
- Given an imported advanced instance or client overwrite on OPNsense 26.7.7, when refresh and a matching plan run, then every audited field round-trips and the plan is empty without breaking existing HCL names/types.
- Given supported create/update/delete paths, when mutation succeeds but reconfigure/read-back fails, then a subsequent apply safely recovers/retries without duplicate objects, false zero-diff activation, or lost deletion state.
- Given a static-key UUID, when the data source reads it, then mode/description/sensitive key are returned through GET only and documentation states that sensitive data-source values remain in state.
- Given each source-supported key type, when the ephemeral resource opens, then the path-segment POST contract returns a non-empty sensitive value and no generated key/address/marker persists in Terraform plan, state, logs, or evidence.
- Given a source-unsupported type or malformed/failed API response, when validation/open runs, then it fails deterministically without falling back to `secret` or disclosing body/key content.
- Given the final digest-bound disposable-appliance run, when all live tests and cleanup checks finish, then advanced instance/overwrite, static lookup, all key types, drift/clear/recovery cases and zero matching leftovers are evidenced; only then are all six audit rows marked covered.

## Spec Change Log

### 2026-10-19 — Review repair 2
- Trigger: the follow-up four-layer review found that ambiguous CRUD recovery could adopt or forget unrelated objects, pending activation could skip a newly requested plan, post-refresh computed values made the 26.7.6 gate global, legacy reads had no version-aware decoder path, and the ephemeral proof claimed a compatible consumer and saved-plan/key scan that it did not execute.
- Amendment: require pre/post identity evidence and full-payload confirmation for ambiguous creates; typed existence results that distinguish not-found from transport/decode failures; state-versus-plan reconciliation after pending activation; explicit state removal after a proven pending delete; configuration-presence-aware feature gating and version-aware legacy decoding for resources and data sources; deterministic selected-map counting; client request-slot use and strict single-document response decoding; complete static-key result validation; table-driven five-key routing/failure tests; and a Terraform CLI proof that actually consumes ephemeral values through a test-only write-only/ephemeral-compatible sink while saving and scanning plan/state/log artifacts against the generated values.
- Known-bad state avoided: name-only adoption, bool existence collapse, planned-state writes after unproven mutations, successful reconfigure treated as the requested update, stale state after pending delete, legacy owners blocked by computed defaults, unbounded direct POSTs, trailing JSON acceptance, untested key routes, and attestation prose broader than executed evidence.
- KEEP: preserve the source-correct five path-segment endpoints; strict owner-local decoding; import-safe Optional+Computed semantics; explicit-empty handling; sensitive fields; disabled collision-tagged fixtures; successful live advanced CRUD/import/drift/clear/static-key coverage; exact VM identity/digest/cleanup checks; deterministic generation; six-part DevRail gate; and six-row promotion only after replacement proof.

### 2026-10-18 — Review repair 1
- Trigger: the first four-layer review found that the implementation could break legacy appliances/imports, repeat or lose committed mutations, and claim ephemeral/non-persistence and live-safety evidence without exercising the Terraform protocol or binding evidence to the actual target/run.
- Amendment: expanded the code-map, execution, and verification requirements with feature-selective compatibility, import-safe Optional+Computed semantics, owner-level ambiguous CRUD recovery, cipher/credential/UUID/product/version validation, independently identified disposable targets, complete digests, exact PASS/cleanup evidence, and a real Terraform ephemeral consumer path.
- Known-bad state avoided: globally gating established owners, schema defaults that clear imported API truth, helper-only recovery tests, direct Go-owner ephemeral tests presented as Terraform proof, and hard-coded evidence counts.
- KEEP: preserve the source-correct path-segment generator and five supported key types; strict owner-specific decoding; explicit-empty collection behavior; sensitive fields; disabled collision-tagged live fixtures; successful advanced CRUD/import/drift/clear coverage; deterministic generation; six-part DevRail gate; and the final six-row audit/status synchronization only after fresh proof.

## Review Repair 2 Constraints

- Treat ambiguous mutations as three-state outcomes: confirmed committed, confirmed absent/uncommitted, or unknown. Only a source-proven not-found may establish absence; transport, authorization, server, and decode failures preserve state and return diagnostics.
- Recover create identity only from collision-resistant pre/post evidence plus exact normalized payload equality. A sole pre-existing description/common-name match is never proof of ownership. Both owners must retain the returned UUID in state when post-create read or decode fails.
- Pending update recovery must activate the previously committed mutation, read current remote truth, compare it with both prior state and the current plan, and still apply a newly requested difference. Never overwrite the current plan with remote state and return success. A failed update is pending only when commit is affirmatively proven; object existence alone is insufficient.
- Pending delete recovery must remove Terraform state after affirmative activation/deletion proof. Read and Delete callbacks must not return success with the deleted object retained, and they must not discard state on ambiguous GET failures.
- Determine parity usage from configuration presence, not populated Optional+Computed plan/state mirrors. Gate parity-capable data-source reads too. Preserve established refresh/update/delete behavior on older appliances through a source-backed version-aware legacy decoder path; do not globally switch legacy fields to 26.7.6-only strict shapes.
- Keep `data_ciphers` and `data_ciphers_ordered` additive without treating a computed mirror as user configuration. A user change to either supported alias must be deterministic and must not conflict with the omitted mirror.
- Count selected entries independently of their key text so an empty selected option plus any other selection is always rejected. Require exactly one JSON document, bounded diagnostics, request-slot acquisition/release, and affirmative response semantics for key generation. Static-key reads reject blank/whitespace key material and missing mode.
- Unit/HTTP proof must cover all five key types with exact method/path/call-count/result assertions; non-2xx success-shaped bodies; malformed or trailing JSON; blank keys; empty static keys; and no response-body/key disclosure.
- The ephemeral acceptance proof must use a real Terraform CLI/Core saved-plan workflow and a test-only ephemeral-compatible or write-only consumer. It must capture every generated key value without logging it, scan the actual saved plan, state, Terraform logs, and retained evidence for those exact values, and fail if any appears. Terraform's non-secret resource-address metadata is expected and is not evidence of key persistence. Unreferenced root ephemeral blocks and static-string scans do not satisfy this constraint.
- Advanced live proof must either exercise pre-existing advanced imports and all audited resource/data-source state fields at the Terraform boundary, or state exactly which assertions are owner/model-only and leave affected audit rows unpromoted. Recovery semantics require Terraform Core two-apply proof or equally strong persisted private-state proof; attestation text must never claim broader coverage than executed.
- The prior `20261019-final` evidence is invalidated by this repair. Move all six scoped audit rows back to `missing-buildable` until a fresh digest-bound run on the re-derived tree passes and its evidence is reconciled.

## Review Triage Log

### 2026-10-18 — Review pass 1
- verdicts: 30 findings — high 16, medium 11, low 1, false 2, maybe-false 0
- findings:
  - `[medium]` `[bad_spec]` Provider-to-ephemeral configuration handoff was not exercised; registration tests and direct Configure calls would miss a broken `EphemeralResourceData` path — amended the spec to require Terraform protocol/provider configuration proof.
  - `[high]` `[bad_spec]` Pending-activation recovery was tested only through disconnected helpers, so resource create/update/delete wiring could regress unnoticed — amended the spec to require owner-level failure-injection tests for both owners.
  - `[high]` `[bad_spec]` Numerous newly audited instance fields never appeared with non-default values in import/read tests — amended the spec to require every audited field in the field-level proof matrix.
  - `[high]` `[bad_spec]` The implementation globally required 26.7.6 for established owner refresh/update paths, breaking the additive compatibility requirement — amended the spec with a feature-selective legacy path.
  - `[high]` `[bad_spec]` Concrete defaults on new instance Optional+Computed scalars could clear non-default imported API truth — amended the spec to require import-preserving omitted-field semantics.
  - `[high]` `[bad_spec]` Concrete defaults on new client-overwrite scalars could clear imported IPv6, gateway, and register-DNS truth — amended the same import-preserving requirement for both owners.
  - `[high]` `[bad_spec]` Update retried pending activation and then unconditionally repeated the mutation — amended recovery requirements to reconcile after reconfigure and avoid duplicate mutation.
  - `[medium]` `[bad_spec]` Differing `data_ciphers` and `data_ciphers_ordered` values were accepted while one was silently ignored — amended the spec to require deterministic agreement/conflict behavior.
  - `[medium]` `[bad_spec]` A known username with null/unknown password bypassed pairing validation and could encode an empty secret — amended partial-unknown credential validation.
  - `[medium]` `[bad_spec]` Static-key lookup accepted an empty or malformed identifier — amended the spec to require pre-request UUID validation.
  - `[medium]` `[bad_spec]` Runtime compatibility checked only a version string and not OPNsense product identity — amended product-identity validation.
  - `[medium]` `[bad_spec]` Version parsing treated prerelease/devel suffixes as the stable minimum — amended stable-release semantics.
  - `[high]` `[bad_spec]` A caller-controlled environment flag was the only disposability assertion, so a production appliance could pass preflight — amended the harness to bind an independently verified VM name/UUID.
  - `[high]` `[bad_spec]` Direct Go-owner key generation did not exercise Terraform ephemeral protocol or persisted artifacts — amended the spec to require a real Terraform ephemeral block and compatible sink.
  - `[medium]` `[bad_spec]` The live digest omitted provider/acctest sources and dependency locks that control the run — amended the complete digest input set.
  - `[medium]` `[bad_spec]` The evidence script did not require the pending-activation test's exact PASS line — amended evidence generation to require every claimed test PASS.
  - `[medium]` `[bad_spec]` Cleanup evidence hard-coded zeroes instead of parsing the marker-bound query/count lines — amended evidence generation to derive and verify actual cleanup counts.
  - `[high]` `[bad_spec]` Intent alignment failed specifically at the Terraform consumer surface for ephemeral generation/non-persistence — amended the implementation proof to use Terraform CLI/provider protocol; direct owner calls remain supplemental.
  - `[high]` `[bad_spec]` An instance add that committed but returned malformed success could lose identity and duplicate on retry — amended owner-level collision-search recovery for ambiguous adds.
  - `[high]` `[bad_spec]` A client-overwrite add had the same malformed-success identity-loss path — amended the shared owner recovery requirement.
  - `[high]` `[bad_spec]` A committed instance delete with malformed success could retain runtime configuration and later forget state without reconfigure — amended pending-delete recovery for ambiguous success.
  - `[high]` `[bad_spec]` A committed client-overwrite delete had the same lost-activation path — amended the shared pending-delete recovery requirement.
  - `[false]` `[reject]` Delete lacking a blanket 26.7.6 gate was not itself unsafe: delete is a pre-existing owner path and must remain available for cleanup; the actual defect was globally gating legacy refresh/update paths.
  - `[false]` `[reject]` The equivalent client-overwrite delete gate claim is refuted by the same feature-selective compatibility requirement.
  - `[medium]` `[bad_spec]` Conflicting cipher attributes could produce ignored configuration and perpetual diffs — carried into the explicit cipher agreement amendment.
  - `[low]` `[reject]` Keepalive multiplication can overflow only at impractical int64-scale intervals, and adding special overflow branches is disproportionate to ordinary use.
  - `[medium]` `[bad_spec]` Static-key ID validation was absent — carried into the UUID validation amendment.
  - `[high]` `[bad_spec]` Live safety did not independently prove the disposable VM identity — carried into the target-bound identity amendment.
  - `[high]` `[bad_spec]` Live ephemeral proof did not inspect Terraform artifacts — carried into the Terraform protocol/non-persistence amendment.
  - `[high]` `[bad_spec]` Recovery claims depended on helper-only tests and did not exercise resource failure paths — carried into the owner-level recovery amendment.

### 2026-10-19 — Review pass 2
- verdicts: 41 findings — high 28, medium 10, low 0, false 3, maybe-false 0
- findings:
  - `[high]` `[bad_spec]` Computed `data_ciphers` mirrors can conflict when a user changes only one alias — amended configuration-presence-aware cipher reconciliation.
  - `[high]` `[bad_spec]` Pending activation can overwrite the current plan with remote state and silently skip a newly requested update — amended state-versus-plan recovery sequencing.
  - `[high]` `[bad_spec]` A successful pending delete returns from Read without removing Terraform state — amended explicit pending-delete state removal.
  - `[high]` `[bad_spec]` Boolean existence helpers collapse transport/auth/server/decode failures into absence during delete recovery — amended typed three-state existence handling.
  - `[high]` `[bad_spec]` Update failure can be treated as committed merely because the old object still exists, then return false success — amended affirmative commit proof and retry rules.
  - `[high]` `[bad_spec]` Name-only create recovery can adopt an unrelated sole pre-existing object — amended pre/post collision-resistant identity and payload proof.
  - `[high]` `[bad_spec]` Post-refresh computed parity fields globally trigger the 26.7.6 gate — carried from the prior feature-selective compatibility finding; the code still reads as previously described.
  - `[medium]` `[bad_spec]` Parity data sources expose strict new fields without the runtime compatibility gate — amended data-source gating.
  - `[high]` `[bad_spec]` Established fields use strict 26.7.6 response decoders without a legacy path — amended source-backed version-aware decoding.
  - `[medium]` `[bad_spec]` Empty-key selected maps can accept multiple selections nondeterministically — amended independent selected-count validation.
  - `[medium]` `[bad_spec]` Ephemeral generation bypasses the provider client's request-slot controls — amended slot acquisition/release requirements.
  - `[high]` `[bad_spec]` Root ephemeral blocks remain unconsumed despite the required compatible sink — carried from the prior Terraform-consumer finding; the test still has no consumer.
  - `[high]` `[bad_spec]` The acceptance test never creates or scans a saved plan/state file despite claiming it does — carried from the prior artifact-inspection finding; the code still reads as previously described.
  - `[high]` `[bad_spec]` Secret scanning never captures or searches for the five generated values — amended generated-value-aware artifact scanning.
  - `[high]` `[bad_spec]` Client-overwrite create loses its UUID when post-create decoding fails — amended identity preservation for both owners.
  - `[high]` `[bad_spec]` Instance create recovery can adopt an unrelated description match — grouped into the pre/post identity amendment.
  - `[high]` `[bad_spec]` Client-overwrite create recovery can adopt an unrelated common-name match — grouped into the pre/post identity amendment.
  - `[high]` `[bad_spec]` Instance delete can mark an ambiguous GET failure as committed absence — grouped into typed three-state existence handling.
  - `[high]` `[bad_spec]` Client-overwrite delete has the same ambiguous GET failure path — grouped into typed three-state existence handling.
  - `[high]` `[bad_spec]` Instance pending activation can skip a differing current plan — grouped into state-versus-plan recovery sequencing.
  - `[high]` `[bad_spec]` Client-overwrite pending activation can skip a differing current plan — grouped into state-versus-plan recovery sequencing.
  - `[medium]` `[bad_spec]` Selected-map empty-key ambiguity is nondeterministic under Go map iteration — grouped into independent selected-count validation.
  - `[medium]` `[bad_spec]` Key-generation decoding accepts valid JSON followed by trailing data — amended strict single-document decoding.
  - `[medium]` `[bad_spec]` Static-key lookup accepts whitespace-only keys or a missing mode — amended complete result validation.
  - `[high]` `[bad_spec]` Legacy updates are gated by known computed parity fields — carried from the prior feature-selective compatibility finding; the same defect remains.
  - `[false]` `[reject]` The live evidence does not claim reconfigure-failure recovery ran on the appliance; it separately attributes that proof to owner-level failure-injection tests.
  - `[high]` `[bad_spec]` Ephemeral evidence claims generated-value and saved-plan scans that were not executed — carried from the prior artifact-inspection finding; the evidence remains broader than the test.
  - `[medium]` `[bad_spec]` Four key types lack exact endpoint-routing assertions — amended a five-type method/path/call-count/result matrix.
  - `[medium]` `[bad_spec]` Non-2xx, malformed JSON, and blank-key generator failures lack consumer-path tests — amended the response-failure matrix.
  - `[medium]` `[bad_spec]` Empty static-key result rejection lacks a data-source test — amended malformed static-key coverage.
  - `[high]` `[bad_spec]` Feature-selective compatibility remains global after refresh — carried from the prior compatibility finding; the current code still uses computed plan presence.
  - `[high]` `[bad_spec]` Object existence is still used as proof that a failed update committed — grouped into affirmative commit proof.
  - `[high]` `[bad_spec]` Live advanced import creates with this provider first and asserts only a subset of audited state — amended pre-existing-import/Terraform-boundary proof or row non-promotion.
  - `[high]` `[bad_spec]` Resource/data-source live assertions cover only a small subset while all rows are promoted — amended field-complete boundary proof or row non-promotion.
  - `[medium]` `[bad_spec]` Partially unknown collections are proved only below Terraform Core planning — amended Core-level tri-state proof.
  - `[high]` `[bad_spec]` Secret refresh blank/omitted/returned behavior is not exercised through a real refresh boundary — amended Terraform refresh proof or explicit audit limitation.
  - `[high]` `[bad_spec]` Recovery is tested through direct callback invocations rather than persisted private state across separate Core applies — amended two-apply or equivalent persisted-state proof.
  - `[false]` `[reject]` A live older-appliance run is not required to prove pre-mutation version rejection; deterministic fake-server no-mutation tests can satisfy this contract.
  - `[false]` `[reject]` Static-key malformed/non-2xx cases need deterministic owner tests, not necessarily a live malformed appliance.
  - `[high]` `[bad_spec]` Ephemeral consumer and saved-artifact evidence diverges from the actual unreferenced-block test — carried from the prior Terraform-consumer/artifact findings.
  - `[high]` `[bad_spec]` Six audit rows were promoted from a proof surface narrower than the intent contract — amended row promotion to require boundary-complete replacement evidence.

## Design Notes

OPNsense 26.7.7 is authoritative where the pinned ecosystem provider disagrees. The router binds action arguments from URI path segments, so key generation uses `/gen_key/{key_type}`; the established provider's `?type=` call and `tls-crypt-v2-client` validator are evidence of a baseline defect, not compatibility requirements. `data_ciphers` is priority-ordered upstream, so new canonical ordering support must be additive rather than changing the existing set type in place. A static-key data source is sensitive but stateful; the generator is ephemeral and cannot be used as an ordinary persisted resource argument unless that consumer is write-only/ephemeral-compatible.

## Implementation Evidence

- Final digest-bound disposable run `20261019-final` passed the three exact acceptance tests on OPNsense 26.7.7 with unchanged pre/post digest `5177792c7d4a1cb9ec57ddb293fd0a2498b41c6b6073f63bff32f534e39ddb76`.
- `TestAccOpenVPNAdvancedParity` proved advanced instance and client-overwrite CRUD/read/import, stable no-change plans, out-of-band drift repair, populated-to-explicit-empty collection updates, and exact marker-bound cleanup (`instance=0`, `client-overwrite=0`).
- `TestAccOpenVPNStaticKeyLookup` proved the strict GET-only sensitive UUID lookup without mutation or reconfigure.
- `TestAccOpenVPNGenerateKeyEphemeralProtocol` proved provider-configuration handoff and all five supported key types through Terraform Core; saved plan, state, logs, and retained evidence contained none of the exact generated key values.
- Owner-level focused failure-injection tests prove create/update/delete recovery for both owners, including ambiguous committed add/delete results and reconfigure-only retry without repeating mutation.
- Exactly the six scoped coverage-audit rows are promoted to `covered` with row-specific owner evidence. The implementation remains in review and is not represented as merged or released.
- Sanitized target, digest, exact PASS, cleanup, and non-persistence attestations are retained in `_bmad-output/implementation-artifacts/31-0c-live-evidence.json` and `31-0c-live-output.txt`; endpoint URLs, credentials, fixture dependency identifiers, key material, ephemeral markers, state files, and unsanitized logs are omitted.

## Verification

**Commands:**
- `go test ./internal/service/openvpn ./internal/provider ./internal/coverageaudit` -- focused schema/model/HTTP/recovery/registration/audit tests pass.
- `go generate ./... && git diff --exit-code` -- generated docs/audit output are deterministic and committed.
- `DOCKER_CONFIG=/tmp/docker-empty-story31c DEVRAIL_IMAGE=ghcr.io/devrail-dev/dev-toolchain:1.12.24 make check` -- lint, format, tests, security, scans, and docs pass with Go 1.26 tooling.
- opt-in disposable OPNsense 26.7.7 acceptance command recorded by the implementation -- all required tests run uncached, candidate digest is unchanged before/after, secret scans are clean, and exact cleanup searches return zero.
