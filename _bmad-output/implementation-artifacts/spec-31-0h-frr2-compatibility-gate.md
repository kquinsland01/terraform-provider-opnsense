---
title: 'Story 31.0h: FRR 2.0 compatibility gate'
type: 'bugfix'
created: '2026-10-20'
status: 'done'
baseline_revision: '154de46ec53a091a27edd27b69172247b214f3dd'
review_loop_iteration: 0
followup_review_recommended: false
context:
  - '_bmad-output/implementation-artifacts/epic-31-context.md'
warnings: [oversized]
deferred: []
---

<intent-contract>

## Intent

**Problem:** Existing BGP owners send removed `multiprotocol` fields and assume individual, multi-select policy relationships. Stable os-frr 2.0 uses explicit address families and grouped policy selectors, so existing configurations risk lost family intent, perpetual UUID drift or incorrect effective policies.

**Approach:** Adapt the existing BGP owners to the installed plugin's proven capabilities, preserve existing Terraform names/identities and legacy configurations, and prove family/state/policy/reload behavior on an isolated disposable appliance before recommending BGP adoption. This is the P0 compatibility subgate of 31.0h, not completion of all advanced BGP field parity.

## Boundaries & Constraints

**Always:** Discover installed os-frr independently of core firmware using read-only package metadata and cross-check the BGP model's read-only capabilities. Validate again before writes; fail closed on unsupported version, missing plugin, contradictory/malformed capability, auth or transport failure. Preserve all existing registrations, UUID addresses, deterministic offline version-0 state upgrades, existing `multi_protocol` configurations and IPv6-only intent. Explicit address families are lowercase `ipv4`/`ipv6`, represented as a typed set. Configured unknown intent is not a default/clear. Preserve coherent linked policy identities and sequence order. Verify semantic group equivalence using authoritative full membership, not UUID guessing. Retain saved identity/state and private pending activation on reload failure; recovery activates without duplicating a committed mutation. Serialize discovery, writes and recovery using existing locks. Strict mutation/read/delete/reload handling must be scoped to owners under test. Existing write-only credentials remain hidden and preserved. Keep legacy live acceptance explicitly unverified unless actually executed. Record exact source digest, core, installed plugin and daemon versions, independent generated-config/PF/routing observations, exact restoration and zero leftovers.

**Never:** Infer plugin semantics from core version; detect capabilities using trial mutations; silently discard unsupported fields; choose arbitrarily among configured policy groups; silently replace user resource addresses or policy UUIDs; edit production firewalls/BGP; enable lab protocol traffic on a production bridge; introduce parallel policy/group resources; claim all of 31.0h or broader parity complete; claim older firmware/OpenTofu/live-version proof from mocks; promote coverage rows or compatibility claims broader than actual non-skipped proof; weaken scanners/checks. No provider-managed plugin upgrade or raw appliance XML editing as provider behavior.

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
| --- | --- | --- | --- |
| Installed capability | Installed os-frr package plus read-only neighbor template | Version/capability-selected legacy or 2.0 encoding, independently recorded core version | Absent/malformed/contradictory/unknown future plugin or permission/network failure diagnoses before writes; plugin change between plan/apply is revalidated |
| Legacy family intent | Existing `multi_protocol` true/false and transport address | Match official migration: true -> both; false + IPv6 address -> ipv6; false + IPv4 -> ipv4; UUID preserved | Unknown/malformed values remain unknown or diagnose, never become false/empty; actual registered version0 upgrade is offline |
| Explicit family | ipv4, ipv6, both, omitted or conflicting legacy alias | 2.0 supports cross-family transport; legacy supports only dual or transport-matched single; imported family is API truth | Empty/invalid families and explicitly conflicting aliases reject; legacy unsupported cross-family-only intent rejects before mutation |
| Grouped policy | Existing linked UUIDs including nonrepresentative sibling, same-name members and ordered sequences | Resolve logical name/number equivalence and preserve existing configured identity when semantically unchanged; import uses authoritative canonical relation; generated policy/order remains correct | Multiple different groups in a single-select field, missing/stale members and ambiguous mixed-family prefix groups diagnose before write; never choose first arbitrary UUID |
| Peer-group AF policy | Family with linked prefix/route-map policy and next-hop/default-route flags | Existing flat API owner emits per-AF effective policy; no accidental default originate/export | Unknown/unsupported configured policy rejects, actual effective config and routing inspected on isolated target |
| Mutation failure | Saved CRUD followed by failed reload/read; ambiguous create/delete | Durable UUID/confirmed state and pending activation survive provider restart/refresh; retry activation without replaying saved mutation | Validation/missing/ambiguous/nonaffirmative bodies produce actionable diagnostics without secret values; ambiguous identity never adopted |
| Live cleanup | Tagged dependencies, disabled bridged NIC, captured settings/config/PF/routing baseline | Real Terraform lifecycle/import/drift/delete converges; immediate pre-rollback zero assertions and exact baseline restoration | Fail closed on identity/isolation/version/digest mismatch; cleanup cannot hide Terraform delete failure; partial artifacts remain private |

</intent-contract>

## Code Map

- `internal/service/quagga/bgp_neighbor_{model,schema,resource,data_source}.go` -- current boolean `multi_protocol`, `multiprotocol` wire payload, flat SelectedMap policy sets; add explicit family compatibility and actual registered state upgrade. Preserve existing defaults by Config-aware alias resolution rather than equating omitted false with explicitly configured false.
- `internal/service/quagga/bgp_peergroup_{model,schema,resource,data_source}.go` -- family exists; correct lowercase values and extend existing flat owner only with policy fields needed for equivalent FRR2 behavior.
- `internal/service/quagga/route_map_*`, `prefix_list_*`, `bgp_aspath_*`, `bgp_communitylist_*`, `bgp_redistribution_*`, `bgp_global_*` -- audit affected relationships and sequence identities; route-map `order` is API sequence `id`, distinct from UUID. Existing `match_prefix` is `match2`; AS-path `match`/community `match3` and grouped policy relations must not silently disappear during compatibility updates. Preserve sibling fields/objects. Guard unsupported intents explicitly rather than pretending full advanced parity shipped.
- `internal/service/quagga/exports.go`, `data_source_helpers.go`, `acceptance_precheck_test.go` -- preserve registrations and extend owner-local capability/data-source/acceptance behavior; no generic core-version assumption.
- `internal/service/openvpn/compatibility.go`, `internal/service/wireguard/{pending_activation,create_lock,peer_resource,server_resource}.go`, `pkg/opnsense/{crud,reconfigure,reqopts,types}.go` -- reuse proven owner-selective decoding/result/pending-state/locking patterns. OpenVPN core detection is structural precedent only, not FRR version authority.
- Pinned plugins `5ac2959c8514b4f60e94d65439be5f629f10521f`, extracted read-only at `/private/tmp/frr2-pinned.qZiTsw/net/frr/`: `Makefile` (2.0 revision1), `pkg-descr`, `src/opnsense/mvc/app/models/OPNsense/Quagga/{BGP.xml,GroupedModelRelationField.php,Migrations/M1_1_3.php}`, API `BgpController.php`/`ServiceController.php`, service templates and `src/etc/inc/plugins.inc.d/frr.inc` are exact authority. Model1.1.5 is not package2.0 or daemon version. Inspect source for actual reload response before choosing strict affirmative contract.
- Core26.7.7 cache `/private/tmp/opnsense-core-26.7.7.H2iTv2/src/opnsense/mvc/app/controllers/OPNsense/Core/Api/FirmwareController.php:801` -- `/api/core/firmware/info` package rows: installed os-frr version, including package `_revision`; local overrides catalog. Read-only `get_neighbor` template cross-checks family versus multiprotocol. Read old stable plugin source for legacy mapping; never fabricate legacy live proof.
- `test/scripts/run-story-31-0e-acceptance.sh`, `story-31-0e-code-digest.sh`, `internal/service/wireguard/wireguard_parity_acceptance_test.go`, `internal/acctest/` -- adapt digest-bound real Terraform Core proof, root-owned identity/TLS preflight, independent raw API oracle, cleanup-before-rollback and fail-closed scan mechanism for FRR.
- `templates/`, `examples/`, generated `docs/`, support/roadmap/audit artifacts and `31-0h-quagga-bgp-field-parity.md` -- accurately document compatibility subgate versus remaining advanced parity and release/adoption boundaries. Do not mark complete missing audit rows without full row-level proof.

## Tasks & Acceptance

**Execution:**
- `internal/service/quagga/compatibility*.go` and affected existing owners -- implement read-only installed-plugin/capability validation, family adaptation and strict owner-local lifecycle; preserve APIs/identities and detect unsupported intent before writes.
- `internal/service/quagga/*_test.go` -- cover every matrix row with named executed tests, exact HTTP contracts, malformed/default/unknown boundaries, real provider-protocol state upgrade and serialized failure/restart/refresh; handshake concurrency through owner methods, not lock helper tests alone.
- Affected policy models/resources/data sources -- implement deterministic semantic grouped identity handling, single-select validation, ordered sequence preservation and peer-group AF policy; avoid unrelated advanced-field expansion.
- `internal/service/quagga/frr_compatibility_acceptance_test.go`, `test/scripts/run-story-31-0h-acceptance.sh`, `story-31-0h-code-digest.sh` -- implement non-skippable digest-bound isolated lifecycle/config/PF/routing proof with independent raw API/guest observations, before-rollback zeros, exact restore and secret scans. Parent owns provisioning/retirement and supplies private runtime path when ready.
- Templates/examples/generated docs, Story31.0h status and `_bmad-output/implementation-artifacts/31-0h-frr2-live-evidence.md` -- reconcile only proven compatibility subgate. Evidence excluded from digest to avoid recursion; code/spec/dependencies included. Preserve prior WireGuard proof as historical, not this candidate's live proof.

**Acceptance Criteria:**
- Given an existing boolean-only Terraform configuration and version0 persisted state, when actual registered Upgrade/Plan/Apply/Refresh run, then equivalent IPv4/IPv6/dual-stack policy converges in-place with preserved UUIDs and exact no-change plans, without user state editing.
- Given FRR2 linked-policy entries with multiple sequence siblings, when importing, applying, refreshing, drifting and repairing neighbors/peer groups/route maps, then semantic UUID normalization causes no perpetual diff or silent identity loss and generated per-AF policy/order matches intended configuration.
- Given unknown/unsupported family/group/version or failed reload after a saved write, when provider operations run and restart, then unsupported writes are absent, diagnostics are actionable, saved identity and activation recovery persist, and secrets never appear in retained output.
- Given isolated core26.7.7/os-frr2, when real Terraform lifecycle completes, then generated FRR/PF rules and effective routing are independently checked, no default-route export or production bridge traffic occurs, Terraform deletes all tagged objects before rollback and baseline settings/config/rules/routes restore exactly.
- Given final frozen candidate, when generation, full/focused tests, full `make check`, independent review and exact-digest live rerun pass, then a reviewable implementation PR is published with precise tested-version evidence and remaining legacy/full-parity/release/adoption gates stated.

## Spec Change Log

## Review Triage Log

### 2026-10-23 — Review pass 1
- verdicts: 25 findings — high 6, medium 14, low 1, false 4, maybe-false 0
- findings:
  - `[high]` `[patch]` B1 Omitted aliases retain stale transport family on address-family change — Transport-derived AF follows a changed known transport; imported cross-family/dual intent remains authoritative. Protocol planning and delayed Apply regressions cover both boundaries. Verified original defect: Parent real SDK overlay regression fails: IPv4 prior state plus IPv6 address plans ipv4. Explicit and imported cross-family intent must remain distinct.
  - `[high]` `[patch]` B2 Imported peer-group/route-map omitted values may be cleared — Generic and singleton updates preserve genuinely omitted prior Optional+Computed fields; explicit unknown expressions still diagnose. Imported peer-group/route-map/global update regressions exercise retained values. Verified original defect: resolveBGPModel replaces unknown omitted fields with null; generic Update has prior state available but does not supply it; peer-group null family defaults ipv4 and null policy sets serialize empty.
  - `[medium]` `[patch]` B3 Null scalar readbacks may become empty clears — Owned string nulls reject before state replacement. Full valid-neighbor control plus null description/timer protocol regressions retain the complete prior state. Verified original defect: JSON null into owned string leaves its zero value, recordFields marks it present, and optional description/timer validation accepts empty values before fromAPI overwrites state.
  - `[medium]` `[patch]` B4 Reordered selector CSV prevents unique ambiguous-create recovery — Ordinary scalar strings compare exactly/non-null; only known selector lists normalize order, with scalar linkedRoutemap single-select. Scalar-comma and reordered-list regressions cover identity. Verified original defect: exactBGPPayload sorts expected members but decodeFRRSelection leaves scalar CSV order unchanged. Fix selector-only canonicalization without treating arbitrary scalar text as a set.
  - `[medium]` `[patch]` B5 Fatal cleanup helper prevents later restoration phases — Independent LIFO cleanup callbacks attempt every restoration phase even after earlier Fatal; subprocess regression verifies continuation while preserving failure. Verified original defect: One cleanup callback calls helpers using t.Fatal; Goexit skips subsequent General restore, disabled sync and filter reload. Independent cleanup phases must still be attempted.
  - `[medium]` `[patch]` B6 Marker-filtered searches do not prove globally empty namespaces — Zero assertions search complete namespaces; marker targeting remains only for bounded deletion. Untagged-leftover regression proves rejection. Verified original defect: frrAssertZeros passes marker to searchPhrase although preflight requires globally empty namespaces; untagged leftovers can evade asserted zeros. Retain marker targeting for deletion only.
  - `[medium]` `[patch]` B7 Active collector does not exercise peer-group membership/inheritance — Active collector now joins the policy peer group without a masking direct outbound policy; active defaults stay false. Separate unattached group exercises default flags; raw/import/data-source membership checks added. Verified original defect: The collector peer selects direct route-map and never sets peer_group. Attach it to policy group with default-originate permanently false; exercise true flags on a separate unused group.
  - `[medium]` `[patch]` B8 Disabled family fixtures lack independent raw API truth checks — Disabled IPv4/IPv6/dual/cross fixtures independently assert raw API family and disabled state; live candidate rerun is required. Verified original defect: frrLiveCheck only reads Terraform family attributes for v4/v6/dual/cross. Add raw family selection checks; disabled peers deliberately have no rendered activation, while active collector/peer-group AF structure is checked separately.
  - `[medium]` `[patch]` B9 Failed find inventory can yield successful partial digest — Checked temporary inventories replace process substitutions; actual find failure regression rejects partial digest output. Verified original defect: find commands execute in process substitutions whose status is not propagated by parent set-e/pipefail. Collect inventories in checked commands and add an injected-inventory-failure regression.
  - `[medium]` `[patch]` B10 Canceled owner request may wait indefinitely for mutex — Cancellation-aware weighted semaphore retains serialization; canceled owner waiter regression proves no discovery/mutation. Verified original defect: All affected owners acquire sync.Mutex before any request-context check; existing owner serialization trace proves waiting behind reload. Use cancellation-aware acquisition and test canceled actual owner never reaches discovery/mutation.
  - `[medium]` `[patch]` G1 Saved generic update failure/restart recovery is not covered — Actual protocol saved-update reload/read failures preserve new values, UUID/private marker across restart and activation retry without another mutation. Verified original defect: Preverified gap: create/delete and singleton-create recovery tests do not execute bgpUpdate saved-state/private branch. Add reload/read-back failure cases with fresh provider, retained UUID/new values and exactly one mutation.
  - `[medium]` `[patch]` G2 No known nonempty peer-group relationship is tested — Active collector now joins the policy peer group without a masking direct outbound policy; active defaults stay false. Separate unattached group exercises default flags; raw/import/data-source membership checks added. Verified original defect: Preverified gap: replacing outgoing peergroup with empty does not affect existing fixtures. Active inherited policy proof and raw/import/data-source membership checks must exercise it.
  - `[high]` `[patch]` E1 Omitted imported policy values clear on unrelated update — Generic and singleton updates preserve genuinely omitted prior Optional+Computed fields; explicit unknown expressions still diagnose. Imported peer-group/route-map/global update regressions exercise retained values. Verified original defect: Same verified resolveBGPModel unknown-to-null path as B2; preserve genuinely omitted prior values for generic updates without replacing explicitly configured unknown expressions.
  - `[high]` `[patch]` E2 Omitted aliases preserve stale family after transport switch — Transport-derived AF follows a changed known transport; imported cross-family/dual intent remains authoritative. Protocol planning and delayed Apply regressions cover both boundaries. Verified original defect: Same real SDK failure as B1; preserve imported cross-transport family truth, dual intent and explicit configured aliases.
  - `[medium]` `[patch]` E3 Valid explicit single family plus false alias fails when address unknown — Only address-dependent alias comparison defers for unknown transport; definite conflicts reject and resolved Apply validates the final family. Verified original defect: ModifyPlan calls legacyFamilies on unknown Address.ValueString()==empty and labels parse failure an alias conflict. Defer only transport-dependent single-family validation; validate again once apply address resolves.
  - `[false]` `[reject]` E4 Selected empty key alleged malformed policy identity — Refuted by actual 26.7.7 get_neighbor: linked policy fields and peergroup return empty-key selected=1. BaseListField.getNodeOptions explicitly emits this placeholder for unset optional single selectors. Rejecting it would break valid API truth.
  - `[medium]` `[patch]` E5 Effective PF oracle permits narrowed endpoints/interfaces — Effective and rendered PF oracles require broad any-to-self/reverse TCP179 for both AFs without interface narrowing; negative fixtures added. Verified original defect: frrBGPFirewallProof marks direction/family from any pass TCP179 line without checking its from/to/on restriction. Actual captured rules are any-to-(self) and reverse; require those effective semantics and negative fixtures.
  - `[medium]` `[patch]` I1 Inherited peer-group routing surface is unexercised — Active collector now joins the policy peer group without a masking direct outbound policy; active defaults stay false. Separate unattached group exercises default flags; raw/import/data-source membership checks added. Verified original defect: Descriptive intent divergence matches B7/G2: direct neighbor policy routing is not inherited peer-group behavior. Keep all active default origination disabled while testing separate unattached flag group.
  - `[medium]` `[patch]` I2 Occurrence counts do not structurally associate policy with AF — Independent parser associates peer-group policy/default flags with the correct address-family block; misplaced-AF negative fixture rejects. Verified original defect: Both AF headings plus two next-hop/default lines could pass with both lines under one AF. Parse relevant address-family blocks and assert expected peer-group policy in each.
  - `[false]` `[reject]` I3 Provider-only effective PF/daemon convergence reading — Intent requires independent PF/routing observations, not provider-managed PF reload or daemon-set restart. Runner marks explicit lab activation and docs explain soft-reload boundary; no provider-only convergence claim is made.
  - `[false]` `[reject]` I4 Former uppercase peer-group documentation alleged lost working compatibility — Pinned legacy 1.55 BGP.xml at09dbb71a4dfbf8e7da5dfdddaa4c8bce6aeb7e2f defines lowercase ipv4/ipv6 keys. BaseListField validators use case-sensitive option-key lookup. Old provider sent uppercase unchanged, so this was a documentation error, not a previously working configuration regression.
  - `[low]` `[patch]` I5 Import oracle verifies group but not exact canonical representative — Fresh imported/data-source references compare exact canonical raw owner selection; managed sibling identity preservation remains semantic. Verified original defect: frrCheckImportedPolicy confirms referenced group name, not raw selected canonical UUID. Add exact independent raw owner-selector comparison for fresh import/data-source references; managed configured sibling preservation remains semantic.
  - `[false]` `[reject]` I6 Offline edge-case coverage versus all-matrix live proof — Intent explicitly retains unexecuted legacy live/OpenTofu boundaries; evidence pins one installed version and does not promote missing coverage rows. Mock/protocol error-path tests plus bounded real live proof do not claim every matrix scenario was live executed.
  - `[high]` `[patch]` P1 Scalar text may be mistaken for unordered selector during identity recovery — Ordinary scalar strings compare exactly/non-null; only known selector lists normalize order, with scalar linkedRoutemap single-select. Scalar-comma and reordered-list regressions cover identity. Verified original defect: Parent real Go overlay regression fails: remote description a,b incorrectly exact-matches wanted b,a. exactBGPPayload must compare ordinary scalars exactly and normalize only known selector-list fields; test both wrong-scalar rejection and reordered real selector recovery.
  - `[high]` `[patch]` P2 Partially unknown configured policy set may serialize as an empty clear — Recursive unknown-value rejection prevents configured partially unknown policy sets from reaching mutation; actual protocol red/green and delayed transport Apply regressions cover the boundary. Verified original defect: Actual protocol Apply with a known set containing an unknown string reached set_neighbor when the recursive guard was disabled (red regression). Resolution now rejects nested unknown intent before mutation; genuine omission alone can preserve prior values. TestReviewPartialUnknownPolicySetCannotMutate and delayed transport Apply regression pass in targeted verification.

All 21 accepted rows map to 16 bounded repair groups (high 4, medium 11, low 1); four false findings were rejected individually. Focused repair tests pass. Follow-up review targets interactions among omitted-state preservation, family resolution and saved-mutation identity.

### 2026-10-23 — Follow-up review pass 2
- verdicts: 19 findings — high 4, medium 8, low 1, false 6, maybe-false 0
- findings:
  - `[high]` `[patch]` FB1 Null collection member can clear configured policy — Reviewed resolveBGPModel only rejects unknown members and setToCSV ignores conversion diagnostics; null string member becomes empty CSV. Reproduce actual owner protocol mutation, then reject before serialization without changing whole-field omission semantics. Applied; targeted verification passed: TestReviewNullElementPolicySetCannotMutate; whole empty sets remain valid.
  - `[medium]` `[patch]` FB2 Valid inherited-AS neighbor cannot be read — Pinned BGP.xml remoteas is optional; validateFRRFields newly makes it required. Allow present empty read as Null and reject malformed nonempty values; public required config remains unchanged. Applied; targeted verification passed: TestFinalNeighborEmptyASRefreshImportDataSource (including Null API truth and no public schema change).
  - `[high]` `[patch]` FB3 Ambiguous singleton save loses pending activation — Actual singleton Update error branches retain marker only for MutationReconfigureError; transport/nonaffirmative save response can follow committed settings. Reconcile authoritative exact owned payload before adopting desired state, keep activation pending without fabricating commit, add restart/no-replay regression. Applied; targeted verification passed: TestProtocolFRRSingletonAmbiguousUpdateRestartRecovery, including actual General option maps and unconfirmed read-failure refresh.
  - `[medium]` `[patch]` FB4 FRR2 read accepts multiple distinct single-select groups — normalizeBGPRead validates policyGroups membership but ignores group count; unlike prepareBGPRequest it accepts two logical groups in linked* fields. Apply cardinality only to FRR2 single-group fields; keep real route-map match multi-selection. Applied; targeted verification passed: TestFinalMalformedOwnedReadRetainsState, including full valid controls and distinct logical groups.
  - `[medium]` `[patch]` FB5 Invalid finite selector keys can overwrite owned state — Peer-group RemoteASMode, redistribution protocol, General Profile/SyslogLevel are owned but only syntactically decoded. Validate exact pinned option domains; optional empty placeholders remain valid. Neighbor remote_as_mode is unowned and is not added. Applied; targeted verification passed: TestFinalMalformedOwnedReadRetainsState against exact pinned option domains.
  - `[medium]` `[patch]` FB6 Malformed router ID or network list can overwrite state — validateFRRFields only validates neighbor Address, not owned global RouterID/Networks. Enforce pinned IPv4-host/strict CIDR contracts before state replacement with full valid-response controls. Applied; targeted verification passed: TestFinalMalformedOwnedReadRetainsState for router ID and strict network CIDRs.
  - `[medium]` `[patch]` FB7 Effective PF proof can accept port1790 — strings.Contains(port = 179) accepts1790 with otherwise broad endpoints/directions. Match an exact port token or bgp alias and add negative fixture. Applied; targeted verification passed: TestFinalFirewallExactPortToken rejects1790 while preserving179/bgp.
  - `[medium]` `[patch]` FB8 Stale peer-group relation can enter strict state — normalizeBGPRead omits PeerGroup from membership validation; scalar decoder only validates selection shape. Require authoritative UUID lookup for nonempty relation; empty optional remains valid. Applied; targeted verification passed: TestFinalPeerGroupRelationReadBoundary rejects stale relations and preserves empty/valid ones.
  - `[medium]` `[patch]` FB9 Actual singleton Update/read-back recovery lacks regression — Existing singleton failure test creates then refreshes/unchanged-updates; it does not exercise a changed saved Update followed by reload/read failure. Add actual General/BGP Update restart retention and exactly-one-save cases. Applied; targeted verification passed: Actual General/BGP Update ambiguous/reload/read failures and fresh refresh/unchanged retry in TestProtocolFRRSingletonAmbiguousUpdateRestartRecovery.
  - `[low]` `[patch]` FB10 External identity/guest probe provenance not recorded — Runner records collector hash but not executable source hashes for two proof-critical host helpers. Direct correction: record before/after SHA256 and reject changes; do not print contents/credentials. Applied; targeted verification passed: Before/after external probe hashes are recorded and compared by the frozen runner; actual outcome is in the excluded live evidence.
  - `[high]` `[patch]` FE1 Null policy element bypasses intent guard — Same demonstrated serializer boundary as FB1; actual protocol red/green required and null collection elements rejected owner-locally. Applied; targeted verification passed: TestReviewNullElementPolicySetCannotMutate, with actual red retained before repair.
  - `[medium]` `[patch]` FG1 Successful legacy multiprotocol encoding lacks wire regression — Preverified gap: current protocol fixture always advertises2.0_1; existing helper rejection test does not exercise successful legacy dual encoding. Add legacy IPv4/IPv6/dual create/update/readback assertions; old-plugin live remains unverified. Applied; targeted verification passed: TestFinalLegacyCreateUpdateWire: IPv4/IPv6/dual create and update emit correct multiprotocol and omit family.
  - `[false]` `[reject]` FI1 Exact reviewed candidate live proof remains pending — This is the explicitly tracked unfinished verification gate, not a code defect or false success claim. Historical digest is labeled historical; final candidate must pass before publication.
  - `[false]` `[reject]` FI2 Disabled family fixtures do not prove every active family transport — Carried I6 evidence-boundary refutation: intent does not require active session for every family matrix; current active dual-AF plus specific IPv4-prefix proof and offline/raw family checks are explicitly bounded.
  - `[false]` `[reject]` FI3 Failure-recovery proof is principally protocol-level — Carried I6: real-appliance injected failure/restart matrix is not claimed; actual provider-protocol failure tests are distinct and valid evidence, without promoting live coverage.
  - `[false]` `[reject]` FI4 Live fixture does not exercise every effective policy combination — Carried I6: compatibility subgate excludes full advanced parity; disabled redistribution and offline AS-path/community combinations remain explicitly distinguished from active inherited route-map proof.
  - `[false]` `[reject]` FI5 Legacy encoding tests are not old-plugin live certification — Carried I6: exact intent retains unexecuted legacy live as unverified; actual v0 provider state migration on FRR2 is labeled separately.
  - `[false]` `[reject]` FI6 Soft reload does not imply provider PF/startup convergence — Carried I3 activation-boundary refutation: explicit isolated-lab PF/sync operations are disclosed; no provider-managed daemon restart/filter activation claim is made.

  - `[high]` `[patch]` FP1 Empty configured policy member can serialize as an empty clear — Parent actual protocol overlay reproduced set_neighbor for a set containing an empty string (frr-parent-empty-policy-red.log). Reject malformed member before serialization while preserving an empty whole set as a legitimate clear; targeted green passed. Applied; targeted verification passed: TestReviewEmptyElementPolicySetCannotMutate and TestReviewWhitespaceElementPolicySetCannotMutate; parent actual red retained.

Four context-free layers completed; fourth launched after the initial concurrency cap released a slot. No reused reviewer was needed. The same implementation owner is applying the bounded repairs. Parent will verify the repaired paths and the final frozen candidate; no historical pass is reused as proof for changed code.

## Design Notes

Official migration maps old `multiprotocol=1` to both AFs, otherwise derives a single AF from the transport address. Explicit FRR2 family may intentionally differ from transport family. Schema upgrades must be deterministic/offline; runtime capability checks happen separately and must not reinterpret imported API family through the old boolean default.

GroupedModelRelationField exposes the first UUID in a logical group with selection merged from siblings. Resolve authoritative group membership to compare semantic identity; preserve a prior/configured member only when it exists in the same selected logical group. Distinct groups in single-select fields and mixed-family prefix groups are not silently collapsed. Keep individual policy-entry resource identities and upstream reference-protected deletion.

Peer-group policy fields remain flat; templates moved them under each activated AF. Automatic routing firewall rules are gated by general enabled+fwrules and include broad inet46 TCP179 to/from self. Observe generated/effective rules in the disconnected lab, explicitly test opt-out/restoration and default-originate behavior; do not infer from neighbor family alone.

## Verification

All generation/lint/tests run inside DevRail1.12.24; use `bash -c`, not login bash. Parent owns a fresh VM102 on pmx-02 with disabled production-bridge net0, QEMU user-network WAN/test NICs and loopback-only forwarding. Expected UUID is `13921598-695f-42cc-9d09-1b06dd2d38b5`; runtime credentials/guest probe will be supplied privately when ready. Do not use retired WireGuard runtime/credentials. Do not provision or target production. Retain all unfinished live work explicitly until parent supplies lab access and proof actually runs.

**Commands:**
- `go test -count=1 -v ./internal/service/quagga/... ./pkg/opnsense/...` -- all compatibility/schema/upgrade/recovery/grouped-policy tests run/pass; acceptance may skip only in this offline command, never as live evidence.
- `go generate ./...` twice plus generated-diff check -- deterministic docs/audit and exact registration counts preserved.
- `go test -count=1 ./...`, `bash test/scripts/validate-structure.sh`, `git diff --check` -- complete suite, structure and whitespace pass.
- `make check` -- all six full DevRail gates pass, no suppression or skipped required target.
- `test/scripts/run-story-31-0h-acceptance.sh` with private disposable environment -- non-skipped version/identity/digest-bound Terraform lifecycle and independent generated/config/PF/routing/default/cleanup/restoration/secret proof. Legacy live row remains unverified if no second compatible legacy appliance exists.

## Auto Run Result

### Implemented change

The FRR2 compatibility subgate adapts the existing owners using read-only installed-plugin/model capabilities, preserves legacy AF/UUID and logical policy identities, and implements strict saved-mutation/activation recovery. Public registrations/resource addresses remain unchanged. The isolated Terraform Core lifecycle independently checks actual state migration, canonical references, active peer-group inheritance, per-AF policy, approved-prefix-only export, PF opt-out/re-enable, real deletion, global zero leftovers and exact restoration.

### Review disposition

Two complete four-layer independent review passes reported 41 findings. Three additional parent regression findings (scalar identity, partial-unknown intent and empty member) are recorded individually in the same logs, for 44 total: 34 accepted patch rows and ten specifically refuted findings. Shared roots retain individual rows; no finding was silently dropped. No items are deferred. The same implementation owner applied bounded repairs; parent ran whole-tree checks and actual red/green boundary tests. A final independent bounded recheck found no remaining correctness failures in the follow-up repairs. The parent empty-member guard correction was independently reproduced before repair and receives an actual protocol regression, scoped lint and whole-tree verification.

Follow-up patched root counts: high 2 (collection intent and ambiguous singleton save), medium 8, low 1. First-pass patched roots: high 4, medium 11, low 1. The arithmetic is per root group, not per duplicate finding. The standard follow-up trigger is true because high roots were patched; no additional unverified correctness risk remains named after the bounded independent recheck, actual protocol regressions and parent verification, so the final recommendation is false. The intentionally unexecuted live-version/failure matrices below remain explicit evidence boundaries, not inferred support.

Rejected findings: optional API empty-selector placeholder is valid on the exact installed version and official BaseListField source; uppercase historical peer-group HCL was already invalid against legacy lowercase option keys; no provider-owned PF/daemon-set activation or all-matrix live proof is claimed. The six descriptive follow-up divergences preserve those bounded evidence surfaces rather than identify an unreported behavior claim.

### Verification performed

- Default `make check`: all six required targets pass, no skips/suppressions. Exact final output is retained under `test/acceptance-results/frr-parent-final-check.log` and summarized in the live-evidence document.
- Fresh focused Quagga/client suite, fresh `go test -count=1 ./...`, generation twice with identical generated-content hashes, structure registration check, failed-inventory regression and whitespace check pass. Actual protocol tests cover v0 upgrade/plan/apply/refresh, committed mutation failure/restart recovery, omitted/unknown/null/empty intent, scalar identity, strict API response retention and successful legacy IPv4/IPv6/dual write/read contracts.
- Two four-layer independent reviews and bounded independent repair recheck completed. Every finding has a verdict, evidence and disposition above.
- Final frozen-candidate non-skipped live run, source digest, exact installed versions, helper provenance, observed routing/PF, restoration and cleanup outcome are recorded in `31-0h-frr2-live-evidence.md`, excluded from the digest to avoid recursion. Publication is conditional on that final run passing without any candidate changes.

### Publication naming correction

The exact publication-file scan identified a deterministic private-state identifier as a generic credential-pattern false positive. The Go constant identifier is now `bgpPendingStateField`; its persisted string value is byte-identical. No scanner rule, ignore entry or required check changed. The exact publication tree receives the full gate, directory secret scan and digest-bound live lifecycle again before publishing.

### Residual boundaries

Only core26.7.7 / os-frr2.0_1 / bgpd10.7.1 has real-appliance proof. Legacy1.46–1.55 wire/state tests are offline; older-plugin live and OpenTofu remain unverified. Active live routing is dual-AF over IPv4 transport with the specific allowed IPv4 prefix; disabled family fixtures additionally prove raw API/state intent, not every active IPv6/cross-family session or policy combination. Failure injection/restart recovery is actual provider-protocol proof, not all failure cases injected into a live appliance. An unconfirmed singleton response plus failed reconciliation retains old state and pending activation; fresh refresh recovery is verified, not direct unrefreshed Update replay avoidance for an unconfirmed save. Explicit isolated-lab filter reload/disabled-config synchronization is not provider-managed PF activation or daemon-set restart. Full advanced31.0h parity, tag/release, ownership cutover and production adoption remain separate gates. Production configuration is unchanged.

### Files changed

- `.github/workflows/acceptance.yml` — Align existing CI/release tooling to the verified Go security baseline and DevRail image.
- `.github/workflows/ci.yml` — Align existing CI/release tooling to the verified Go security baseline and DevRail image.
- `.github/workflows/docs.yml` — Align existing CI/release tooling to the verified Go security baseline and DevRail image.
- `.github/workflows/format.yml` — Align existing CI/release tooling to the verified Go security baseline and DevRail image.
- `.github/workflows/lint.yml` — Align existing CI/release tooling to the verified Go security baseline and DevRail image.
- `.github/workflows/release.yml` — Align existing CI/release tooling to the verified Go security baseline and DevRail image.
- `.github/workflows/scan.yml` — Align existing CI/release tooling to the verified Go security baseline and DevRail image.
- `.github/workflows/security.yml` — Align existing CI/release tooling to the verified Go security baseline and DevRail image.
- `.github/workflows/test.yml` — Align existing CI/release tooling to the verified Go security baseline and DevRail image.
- `.github/workflows/upstream-assignment-api.yml` — Align existing CI/release tooling to the verified Go security baseline and DevRail image.
- `DEVELOPMENT.md` — Align developer workflow, security baseline or acceptance guidance with the verified toolchain.
- `Makefile` — Align developer workflow, security baseline or acceptance guidance with the verified toolchain.
- `README.md` — Align developer workflow, security baseline or acceptance guidance with the verified toolchain.
- `_bmad-output/implementation-artifacts/31-0h-frr2-live-evidence.md` — Track compatibility subgate, immutable intent, triage, proof and residual backlog boundaries.
- `_bmad-output/implementation-artifacts/31-0h-quagga-bgp-field-parity.md` — Track compatibility subgate, immutable intent, triage, proof and residual backlog boundaries.
- `_bmad-output/implementation-artifacts/spec-31-0h-frr2-compatibility-gate.md` — Track compatibility subgate, immutable intent, triage, proof and residual backlog boundaries.
- `_bmad-output/implementation-artifacts/sprint-status.yaml` — Track compatibility subgate, immutable intent, triage, proof and residual backlog boundaries.
- `_bmad-output/planning-artifacts/support-matrix.md` — Track compatibility subgate, immutable intent, triage, proof and residual backlog boundaries.
- `docs/data-sources/quagga_bgp_neighbor.md` — Document the existing owner compatibility contract and bounded tested-version evidence.
- `docs/data-sources/quagga_bgp_peergroup.md` — Document the existing owner compatibility contract and bounded tested-version evidence.
- `docs/data-sources/quagga_route_map.md` — Document the existing owner compatibility contract and bounded tested-version evidence.
- `docs/resources/quagga_bgp_global.md` — Document the existing owner compatibility contract and bounded tested-version evidence.
- `docs/resources/quagga_bgp_neighbor.md` — Document the existing owner compatibility contract and bounded tested-version evidence.
- `docs/resources/quagga_bgp_peergroup.md` — Document the existing owner compatibility contract and bounded tested-version evidence.
- `docs/resources/quagga_general.md` — Document the existing owner compatibility contract and bounded tested-version evidence.
- `docs/resources/quagga_route_map.md` — Document the existing owner compatibility contract and bounded tested-version evidence.
- `examples/resources/opnsense_quagga_bgp_neighbor/resource.tf` — Show compatible existing resource configuration/import usage.
- `examples/resources/opnsense_quagga_bgp_peergroup/import.sh` — Show compatible existing resource configuration/import usage.
- `examples/resources/opnsense_quagga_bgp_peergroup/resource.tf` — Show compatible existing resource configuration/import usage.
- `go.mod` — Align developer workflow, security baseline or acceptance guidance with the verified toolchain.
- `internal/service/quagga/bgp_aspath_data_source.go` — Adapt existing FRR owner/model/lifecycle behavior or exercise its strict compatibility and recovery contract.
- `internal/service/quagga/bgp_aspath_model.go` — Adapt existing FRR owner/model/lifecycle behavior or exercise its strict compatibility and recovery contract.
- `internal/service/quagga/bgp_aspath_resource.go` — Adapt existing FRR owner/model/lifecycle behavior or exercise its strict compatibility and recovery contract.
- `internal/service/quagga/bgp_communitylist_data_source.go` — Adapt existing FRR owner/model/lifecycle behavior or exercise its strict compatibility and recovery contract.
- `internal/service/quagga/bgp_communitylist_model.go` — Adapt existing FRR owner/model/lifecycle behavior or exercise its strict compatibility and recovery contract.
- `internal/service/quagga/bgp_communitylist_resource.go` — Adapt existing FRR owner/model/lifecycle behavior or exercise its strict compatibility and recovery contract.
- `internal/service/quagga/bgp_global_model.go` — Adapt existing FRR owner/model/lifecycle behavior or exercise its strict compatibility and recovery contract.
- `internal/service/quagga/bgp_global_resource.go` — Adapt existing FRR owner/model/lifecycle behavior or exercise its strict compatibility and recovery contract.
- `internal/service/quagga/bgp_global_schema.go` — Adapt existing FRR owner/model/lifecycle behavior or exercise its strict compatibility and recovery contract.
- `internal/service/quagga/bgp_neighbor_data_source.go` — Adapt existing FRR owner/model/lifecycle behavior or exercise its strict compatibility and recovery contract.
- `internal/service/quagga/bgp_neighbor_model.go` — Adapt existing FRR owner/model/lifecycle behavior or exercise its strict compatibility and recovery contract.
- `internal/service/quagga/bgp_neighbor_resource.go` — Adapt existing FRR owner/model/lifecycle behavior or exercise its strict compatibility and recovery contract.
- `internal/service/quagga/bgp_neighbor_schema.go` — Adapt existing FRR owner/model/lifecycle behavior or exercise its strict compatibility and recovery contract.
- `internal/service/quagga/bgp_peergroup_data_source.go` — Adapt existing FRR owner/model/lifecycle behavior or exercise its strict compatibility and recovery contract.
- `internal/service/quagga/bgp_peergroup_model.go` — Adapt existing FRR owner/model/lifecycle behavior or exercise its strict compatibility and recovery contract.
- `internal/service/quagga/bgp_peergroup_resource.go` — Adapt existing FRR owner/model/lifecycle behavior or exercise its strict compatibility and recovery contract.
- `internal/service/quagga/bgp_peergroup_schema.go` — Adapt existing FRR owner/model/lifecycle behavior or exercise its strict compatibility and recovery contract.
- `internal/service/quagga/bgp_redistribution_data_source.go` — Adapt existing FRR owner/model/lifecycle behavior or exercise its strict compatibility and recovery contract.
- `internal/service/quagga/bgp_redistribution_model.go` — Adapt existing FRR owner/model/lifecycle behavior or exercise its strict compatibility and recovery contract.
- `internal/service/quagga/bgp_redistribution_resource.go` — Adapt existing FRR owner/model/lifecycle behavior or exercise its strict compatibility and recovery contract.
- `internal/service/quagga/compatibility.go` — Adapt existing FRR owner/model/lifecycle behavior or exercise its strict compatibility and recovery contract.
- `internal/service/quagga/compatibility_decode.go` — Adapt existing FRR owner/model/lifecycle behavior or exercise its strict compatibility and recovery contract.
- `internal/service/quagga/compatibility_diagnostics.go` — Adapt existing FRR owner/model/lifecycle behavior or exercise its strict compatibility and recovery contract.
- `internal/service/quagga/compatibility_family.go` — Adapt existing FRR owner/model/lifecycle behavior or exercise its strict compatibility and recovery contract.
- `internal/service/quagga/compatibility_final_review_test.go` — Adapt existing FRR owner/model/lifecycle behavior or exercise its strict compatibility and recovery contract.
- `internal/service/quagga/compatibility_lifecycle.go` — Adapt existing FRR owner/model/lifecycle behavior or exercise its strict compatibility and recovery contract.
- `internal/service/quagga/compatibility_policy.go` — Adapt existing FRR owner/model/lifecycle behavior or exercise its strict compatibility and recovery contract.
- `internal/service/quagga/compatibility_presence.go` — Adapt existing FRR owner/model/lifecycle behavior or exercise its strict compatibility and recovery contract.
- `internal/service/quagga/compatibility_protocol_test.go` — Adapt existing FRR owner/model/lifecycle behavior or exercise its strict compatibility and recovery contract.
- `internal/service/quagga/compatibility_recovery.go` — Adapt existing FRR owner/model/lifecycle behavior or exercise its strict compatibility and recovery contract.
- `internal/service/quagga/compatibility_review_test.go` — Adapt existing FRR owner/model/lifecycle behavior or exercise its strict compatibility and recovery contract.
- `internal/service/quagga/compatibility_test.go` — Adapt existing FRR owner/model/lifecycle behavior or exercise its strict compatibility and recovery contract.
- `internal/service/quagga/data_source_schema_test.go` — Adapt existing FRR owner/model/lifecycle behavior or exercise its strict compatibility and recovery contract.
- `internal/service/quagga/frr_compatibility_acceptance_test.go` — Adapt existing FRR owner/model/lifecycle behavior or exercise its strict compatibility and recovery contract.
- `internal/service/quagga/general_model.go` — Adapt existing FRR owner/model/lifecycle behavior or exercise its strict compatibility and recovery contract.
- `internal/service/quagga/general_resource.go` — Adapt existing FRR owner/model/lifecycle behavior or exercise its strict compatibility and recovery contract.
- `internal/service/quagga/general_schema.go` — Adapt existing FRR owner/model/lifecycle behavior or exercise its strict compatibility and recovery contract.
- `internal/service/quagga/prefix_list_data_source.go` — Adapt existing FRR owner/model/lifecycle behavior or exercise its strict compatibility and recovery contract.
- `internal/service/quagga/prefix_list_model.go` — Adapt existing FRR owner/model/lifecycle behavior or exercise its strict compatibility and recovery contract.
- `internal/service/quagga/prefix_list_resource.go` — Adapt existing FRR owner/model/lifecycle behavior or exercise its strict compatibility and recovery contract.
- `internal/service/quagga/route_map_data_source.go` — Adapt existing FRR owner/model/lifecycle behavior or exercise its strict compatibility and recovery contract.
- `internal/service/quagga/route_map_model.go` — Adapt existing FRR owner/model/lifecycle behavior or exercise its strict compatibility and recovery contract.
- `internal/service/quagga/route_map_resource.go` — Adapt existing FRR owner/model/lifecycle behavior or exercise its strict compatibility and recovery contract.
- `internal/service/quagga/route_map_schema.go` — Adapt existing FRR owner/model/lifecycle behavior or exercise its strict compatibility and recovery contract.
- `templates/data-sources/quagga_bgp_neighbor.md.tmpl` — Document the existing owner compatibility contract and bounded tested-version evidence.
- `templates/data-sources/quagga_bgp_peergroup.md.tmpl` — Document the existing owner compatibility contract and bounded tested-version evidence.
- `templates/data-sources/quagga_route_map.md.tmpl` — Document the existing owner compatibility contract and bounded tested-version evidence.
- `templates/resources/quagga_bgp_global.md.tmpl` — Document the existing owner compatibility contract and bounded tested-version evidence.
- `templates/resources/quagga_bgp_neighbor.md.tmpl` — Document the existing owner compatibility contract and bounded tested-version evidence.
- `templates/resources/quagga_bgp_peergroup.md.tmpl` — Document the existing owner compatibility contract and bounded tested-version evidence.
- `templates/resources/quagga_general.md.tmpl` — Document the existing owner compatibility contract and bounded tested-version evidence.
- `templates/resources/quagga_route_map.md.tmpl` — Document the existing owner compatibility contract and bounded tested-version evidence.
- `test/README.md` — Align developer workflow, security baseline or acceptance guidance with the verified toolchain.
- `test/scripts/run-story-31-0h-acceptance.sh` — Provide digest-bound isolated acceptance, provenance or regression tooling.
- `test/scripts/run-vagrant-acceptance.sh` — Provide digest-bound isolated acceptance, provenance or regression tooling.
- `test/scripts/story-31-0h-code-digest.sh` — Provide digest-bound isolated acceptance, provenance or regression tooling.
- `test/scripts/test-story-31-0h-code-digest.sh` — Provide digest-bound isolated acceptance, provenance or regression tooling.
