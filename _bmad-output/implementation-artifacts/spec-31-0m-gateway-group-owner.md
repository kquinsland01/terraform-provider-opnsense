---
title: 'Story 31.0m: Gateway group resource and data source'
type: 'feature'
created: '2026-10-25'
status: 'done'
baseline_revision: '295c3a0a8bccf58d4bcb34b82be0dff545957286'
review_loop_iteration: 0
followup_review_recommended: false
context:
  - 'AGENTS.md'
  - '_bmad-output/implementation-artifacts/epic-31-context.md'
  - '_bmad-output/planning-artifacts/gateway-group-api-validation.md'
warnings: [oversized]
deferred:
  - title: Supported UUID-preserving populated first-tier clear
    location: Upstream Routing GatewayGroupItemField
    severity: medium
    note: Stable API ignores empty first-tier updates; remain fail-closed until upstream support and positive live proof.
---

<intent-contract>

## Intent

**Problem:** Stable OPNsense 26.7.7 gateway-group CRUD is validated but has no Terraform owner or lookup. Generic gateway lifecycle patterns can lose committed identity after activation failure.

**Approach:** Add `opnsense_system_gateway_group` resource and UUID-keyed data source, strict typed API mapping and durable failure recovery. Prove real Terraform ownership and cleanup on a disposable appliance before promoting support; actual traffic failover remains unverified.

## Boundaries & Constraints

**Always:** Preserve UUID identities, serialize discover/mutate/recover with existing client locks, gate core/model availability read-only before mutations, retain confirmed writes and recovery intent in state/private bytes across provider restart. Consumer-neutral public artifacts, synthetic examples and sanitized proof only. Existing registered resources and shared CRUD contracts remain unchanged unless a verified defect makes a narrowly tested shared change necessary. Add a versioned Unreleased changelog entry after comparing current tags. Parent owns provision/retirement; implementer provides identity-guarded acceptance runner but never targets production. Read authoritative saved state, distinguish unavailable API from absent item, verify complete strict response shape before replacing state, and never interpret HTTP success as affirmative mutation success.

**Never:** Mutate production, modify individual gateway ownership or silently activate PF, claim actual traffic failover from route reconfigure, use raw XML as provider behavior, suppress failing checks, copy private runtime paths/site metadata into public files, or expand into Kea/completeness stories in this change. Do not publish a support claim from mocks/direct API probes alone. Never blindly retry an uncertain add or discard ambiguous delete identity.

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
| --- | --- | --- | --- |
| Core/model gate | firmware status and group template | OPNsense stable >=26.7.7 and usable model before any mutation | Invalid/unknown/future unsupported shape, auth, transport or unavailable model diagnoses; no mutation |
| Typed group | immutable required name; tier_1 through tier_5 string sets; trigger; pool_options; description | API gateway_group wrapper with name,item..item5 CSV keys,trigger,poolopts,descr; UUID retained | Name mask [A-Za-z0-9_-]{1,32}; exact finite enums; no unknown/null/empty/whitespace collection member becomes clear |
| Defaults and clears | omitted tier sets/description/pool; populated optional clear | Known empty sets/strings, trigger down; clear preserves other tiers; nonempty union required | Defer unknown at plan, reject unresolved at write; all-empty union rejects before write even if upstream returns saved |
| Strict read | valid selected maps, scalar string name/descr, selected enum including pool default empty key | Keys not labels; exactly one valid trigger/pool selection, valid nonempty tier union; imported known defaults | Missing/null/malformed fields, selected flags, invalid enum, empty gateway key, invalid immutable name reject without destroying prior state; explicitly handle actual upstream empty shapes only |
| Save/activation | confirmed add/update saved then read/reconfigure failure | Commit UUID/state and durable pending activation; fresh provider retries activation without duplicating saved mutation | Diagnostic preserves committed values/identity and pending marker; do not report old values as current if new save committed |
| Ambiguous mutation | lost/invalid add/set/delete response | Reconcile by authoritative object/search truth; uncertain add recovers only exactly one new complete payload match versus pre-search; selector comparison order-insensitive only | Zero/multiple candidates or contradictory truth fail closed, no duplicate add or arbitrary adoption; recovered writes still activate |
| Delete/in-use | persisted references, confirmed deletion with failed activation, uncertain deletion | HTTP500 in-use retains UUID; pending confirmed delete is activation-only recovery; ambiguous delete checks object truth first | Do not map dependency/auth/read failures to missing; preserve state and marker until proven deletion plus activation |
| Lookup/import/drift | UUID import and data source; external changes | Same strict gate/read mapping, no data source writes, import/no-change, repair drift and explicit clears | Missing owner removes only on proven actual empty response/404 after model gate; malformed reads retain state |
| Restart/concurrency/cancellation | returned protocol state/private bytes in new server | Pending save/delete survives restart, one committed mutation; global mutation serialization and context cancellation | No silent marker corruption/default, canceled operations fail safely with accurate state |

</intent-contract>

## Code Map

- `internal/service/system/exports.go` registers package constructors; no group files exist. `gateway_resource.go` naming/import patterns only, NOT safe recovery implementation.
- `internal/service/wireguard/lifecycle.go` and `server_resource.go` implement discovery snapshots, strict identity, durable pending markers and committed state. `review_repair_test.go` covers ambiguous-delete and actual marker generation; adapt principles, not WireGuard field exclusions.
- `internal/service/wireguard/protocol_repair_test.go` actual protocol6 server/client dynamic values, restart with private bytes; `openvpn/recovery.go` and owner-recovery tests provide smaller patterns.
- `pkg/opnsense/reqopts.go`, `crud.go`, `errors.go`: StrictDeleteResponse, StrictMissingObjectResponse, StrictReconfigureResponse; ParseMutationResponse, MutationReconfigureError, RetryReconfigure. These helpers already acquire slots/locks: avoid nested-lock deadlock. Strict read distinguishes actual empty array from malformed wrapper.
- `internal/service/unbound/compatibility.go` strict firmware status product/version parsing pattern; do not use permissive suffix stripping or infer existence from version alone.
- Official tagged 26.7.7 Routing model/controller and the validation report are authority. GroupSettingsController uses get/add/set/del/search/reconfigure, `gateway_group`, not old camel-case group endpoints; activation means interface routes configure only.
- `templates/{resources,data-sources}/`, `examples/{resources,data-sources}/`, `docs/`, `templates/index.md.tmpl`, support-matrix and coverage audit form public surface; handwritten system owner needs no generator YAML or Kea-specific version template.

## Tasks & Acceptance

**Execution:**
- `internal/service/system/gateway_group_{resource,schema,model,data_source}.go` plus tightly scoped helpers -- add strict owner/lookup, standard configure/import and exports. Schema: ID computed with UseStateForUnknown (lookup ID required); name required RequiresReplace; tier_1..tier_5 optional string sets with known empty-set defaults; trigger default down, enum down/downloss/downlatency/downlosslatency; pool_options default empty, enum empty/round-robin/round-robin sticky-address; description default empty string. Do not invent cross-tier uniqueness or IP-family restrictions.
- `internal/service/system/gateway_group_*test.go` -- named unit and actual protocol regressions for every matrix row, valid controls, omitted/unknown/null/empty boundaries, selected default pool key, complete read retention, confirmed and ambiguous mutation/reconfigure/restart recovery, imports/lookup, cancellation and concurrency. Test real exported provider registration and save/add/delete counters, not helper state alone.
- `internal/service/system/gateway_group_acceptance_test.go`, `test/scripts/run-story-31-0m-acceptance.sh`, `test/scripts/story-31-0m-code-digest.sh` -- real Terraform Core fixture: two synthetic non-default monitor-disabled gateway prerequisites, group create/read/update/tier swap/trigger/pool/default clearing/import/no-change/data-source/drift/repair/delete; reference/in-use delete check if eligible fixture can be established through official APIs, otherwise report specific unmet gate. Parent supplies runtime environment/guard/oracle paths privately. Runner refuses missing identity/TLS/version preflight and missing acceptance environment rather than counting skipped proof; unchanged source/helper hash before/after, exact config/routes restoration and global zero test entries before rollback AND after cleanup. Existing individual-gateway reconfigure must precede eligible selector membership. Retain failed attempts separately; cleanup attempts every phase even after failure. No output containing credentials/raw config in public evidence.
- `templates/resources/system_gateway_group.md.tmpl`, matching data-source template/examples and generated docs -- document min26.7.7, required read firmware/model and Routing group API permissions, UUID import, gateway-name membership, immutable name, activation boundary, recovery and remaining unverified traffic/OpenTofu/older-version gates. No invented ACL identifier.
- `CHANGELOG.md`, story31.0m, sprint status, support matrix/index and coverage audit -- next appropriate Unreleased heading, deterministic generation, reflect only executed proof. Parent adds final sanitized live evidence and final support promotion after proof; do not mark this story done while live/review pending.

**Acceptance Criteria:**
- Given a supported disposable appliance and valid synthetic HCL, when actual Core create/import/update/refresh/data source run, then all fields converge with stable UUID and exact no-change plan.
- Given failed saves, lost responses or activation/read failures, when returned state/private bytes are persisted into a new protocol server and retried/refreshed, then committed identity and values remain accurate, no duplicate add occurs, and recovery completes exactly once.
- Given an in-use group or malformed API response, when deletion/read is attempted, then diagnostic and unchanged tracked identity prevent orphaning or silent loss.
- Given the exact publication tree, when full offline checks, deterministic generation, independent reviews and non-skipped live acceptance finish, then zero test objects and exact baseline restoration are verified and a consumer-neutral implementation PR is published; no production adoption inferred.

## Spec Change Log

- Live Terraform composition exposed a read-only selector-cache prerequisite:
  tagged 26.7.7 `Routing/GatewayGroupItemField` extends
  `JsonKeyValueStoreField`, using `interface gateways list`; the configd
  `gateways.list` action defines `cache_ttl:20`. Individual-gateway route
  reconfigure does not invalidate that cache. Before add/set, the group owner
  must poll authoritative model option **keys** for each planned tier member,
  bounded by 30 seconds and caller cancellation, with modest read-only waits.
  It must issue no add/set while membership remains unavailable and never poll
  by retrying mutations. Pending saved-operation recovery is unchanged.
- Preserve the first failed live Core attempt separately: selectors rejected
  both newly created synthetic gateway names before group save; teardown zeros
  and exact config/routes restoration passed. This is not passing Core proof.


- Additional exact tagged-source review and parent raw-API negative controls
  expose a separate upstream limitation: `GatewayGroupItemField::setValue`
  delegates only for nonempty scalar values; `BaseListField::setValues` filters
  empty array members and delegates through that same override. The official
  controller exposes no UUID-preserving first-tier clear action. Empty-string
  updates retain a populated first tier while returning saved; array variants
  failed without clearing. No sentinel, raw XML or private-endpoint workaround
  is accepted. The owner therefore rejects populated-to-empty `tier_1` updates
  before any field save, while allowing initially tier2-only groups and updates
  of already-empty first tiers. Protocol negative/control tests explicitly
  preserve this boundary. The unchanged intent-contract's universal tier-clear
  promise is not achievable on this stable API; final support-gate disposition
  remains parent-owned rather than silently weakening the acceptance contract.

- Deferred upstream follow-up: obtain a supported stable API change that permits
  UUID-preserving populated first-tier clears, then add positive direct-API and
  real Core proof before removing the protective diagnostic. Neither deleting
  and recreating the group nor bypassing the official controller closes this
  gap; no complete all-tier-clear claim is made meanwhile.

## Review Triage Log

### 2026-10-25 — Review pass
- verdicts: 27 findings — high 2, medium 16, low 0, false 9, maybe-false 0
- findings:
  - `[medium]` `[patch]` Gap: lookup output assertions — Existing lookup tests check diagnostics/ID only; compare every populated computed field, not provider-produced identity alone.
  - `[medium]` `[patch]` Gap: unsupported firmware on create — Refresh cases do not exercise the mutation entry point; assert create rejects with no identity or mutation.
  - `[high]` `[patch]` Blind: uncertain delete blocks refresh retry — The still-present branch returns an error with the marker unchanged; normal Core refresh prevents destroy from reaching Delete. Reconcile retained truth and clear uncertainty, with Core regression.
  - `[medium]` `[patch]` Blind: uncertain save activates unmatched truth — Only a save_unknown enum survives restart; no attempted payload comparison precedes activation. Persist a bounded expected payload and test match/mismatch reconciliation.
  - `[false]` `[reject]` Blind: automatically resume add after failed recovery search — The contract explicitly requires zero/ambiguous recovery candidates to fail closed and instruct manual reconciliation; code diagnoses the failed search and later name collision instructs UUID import. No arbitrary adoption or duplicate add is performed. Automatic adoption without the original pre-search snapshot would weaken identity safety.
  - `[medium]` `[patch]` Blind: reads lose configured retry behavior — groupRequest bypasses the retry transport for GET as well as POST. Preserve read retries while keeping non-idempotent writes single-attempt.
  - `[medium]` `[patch]` Blind: restored confirmed-delete UUID can be forgotten — groupDeleted Read/Delete paths activate and discard state without checking current authoritative presence. Recheck absence and retain a reappeared identity.
  - `[medium]` `[patch]` Blind: actual acceptance HTTPS requests lack pin enforcement — The preflight validates a different connection; insecure provider connections can accept certificate substitution. Pin each actual upstream acceptance request without adding production provider configuration.
  - `[medium]` `[patch]` Blind: helper hash captured after execution — Runner executes guard/snapshot before its hash baseline. Require independently supplied expected hashes and compare before execution, then compare after.
  - `[medium]` `[patch]` Blind: later-tier and initial empty-first-tier live coverage — The first live fixture exercises tiers1/2 only. Add actual populated/cleared tiers3-5 and initial tier2-only composition.
  - `[medium]` `[patch]` Blind: Core recovery state persistence unproved — Protocol fixtures persist returned bytes but bypass Terraform Core error/refresh handling. Add Core fault-server lifecycle regression with fresh provider servers.
  - `[medium]` `[patch]` Blind: intermediate fields lack independent oracle — Terraform attribute checks share the same decoder. Observe saved group fields independently through the private oracle. Route reconfigure is not a traffic-failover claim; the latter remains unverified.
  - `[high]` `[patch]` Edge: uncertain-delete still-present refresh loop — Same demonstrated defect as the blind delete-unknown finding; grouped repair must prove normal Core refresh-before-destroy retry.
  - `[false]` `[reject]` Alignment: broader five-workstream completion absent — This is the explicitly bounded gateway increment, not a claim that all five workstreams are complete; Kea and completeness remain pending and private adoption is tracked separately.
  - `[false]` `[reject]` Alignment: neutral product notes are public — The user excludes site/use notes from the public project; consumer-neutral product specifications and synthetic test contracts do not disclose site plans. Full publication privacy inspection remains required.
  - `[false]` `[reject]` Alignment: runner is preparation not live proof — Correct for the frozen diff narrative; parent has separately completed 13 real Core steps and exact restoration. Final publication-tree proof and support promotion remain pending, not claimed.
  - `[medium]` `[defer]` Alignment: five-tier clear expectation exceeds upstream API — Verified tagged upstream first-tier setter ignores empty values; safe rejection is disclosed and tested. Upstream UUID-preserving first-tier clearing remains an explicit unmet completeness gate, not a passed universal-clear criterion.
  - `[medium]` `[patch]` Alignment: lookup assertion surface narrower — Same root verification gap as lookup-field findings; compare authoritative populated outputs.
  - `[medium]` `[patch]` Alignment: compatibility assertion surface narrower — Same unsupported-create verification gap; exercise mutation entry gate with zero-write counters.
  - `[false]` `[reject]` Alignment: adjacent FRR release wording lacks gateway proof — This corrects pre-existing stale pending wording using already merged/released FRR evidence; it does not claim gateway tests validate FRR or expand this owner.
  - `[false]` `[reject]` Alignment: synthetic tests do not prove private adoption — Intent explicitly excludes production adoption and site inventory from this public change. No adoption claim is made.
  - `[medium]` `[defer]` Contract alignment: populated first-tier clear deviation — The unchanged contract overpromises a stable API operation verified unavailable. Preserve contract text and record unmet upstream completeness rather than fabricate a workaround or full-parity support claim.
  - `[false]` `[reject]` Contract alignment: negative test differs from universal clear promise — Negative tests deliberately prove safe nonmutation for the documented upstream limitation; they are not counted as positive first-tier-clear proof. The completeness gate remains open.
  - `[false]` `[reject]` Contract alignment: live proof pending in reviewed diff — Parent executed first candidate lifecycle outside the diff; final post-review publication-tree run is still required. No pending gate is represented as passed.
  - `[medium]` `[patch]` Contract alignment: lookup full parity assertions absent — Duplicate root verification gap; same all-field populated authoritative lookup regression.
  - `[medium]` `[patch]` Contract alignment: mutation-gate tests absent — Duplicate root verification gap; same unsupported create zero-write regression.
  - `[false]` `[reject]` Contract alignment: FRR wording beyond central implementation — Public tracking cleanup is authorized and based on prior verified merged/released FRR work; no gateway test is offered as FRR evidence.

Three independent non-implementing reviewers covered four layers; the gap reviewer additionally performed alignment, with a supplemental exact-contract audit. All reports were collected before triage/repairs. Duplicate findings retain individual rows; shared root repairs are grouped. The first-tier limitation predates this owner and remains explicitly upstream-blocked; this build does not satisfy universal clear parity. Parent-directed bounded delivery follows the user's authorization to progress as far as possible without production changes.

## Design Notes

Defaults intentionally make managed optional fields owned: explicit empty sets/strings clear them, but an unknown set member is never an empty clear. GET pool default may select the empty key; gateway-tier keys may not be empty. API map labels may differ without representing changed membership. Do not compare arbitrary description strings as CSV sets. Scope recovery to this resource rather than changing all system owners.

## Verification

**Commands:**
- Run all Go, Terraform, generation and scanner commands inside cached `ghcr.io/devrail-dev/dev-toolchain:1.12.24` with Docker `bash -c`; no host tool installs.
- `go test -count=1 -race -v ./internal/service/system/...` -- all relevant named unit/protocol cases pass with no skips for the matrix.
- `go test -count=1 ./...` and `go generate ./...` twice -- full Go pass; deterministic tracked generated output.
- `make check` -- all six default targets pass, no suppression; `git diff --check` clean.
- Parent executes the digest-bound live runner using private runtime configuration; final 31.0m evidence must record actual versions/hash/outcomes, teardown zeros and restoration. Missing lab is unfinished, never a live pass.
- Parent inspects full publication diff for identifying data and scans exact publication files before push; hosted CI separately verified at PR head.

## Follow-up review — repaired candidate

Two non-implementing reviewers independently rechecked the repaired recovery and
verification/security paths. Neither found a remaining reproducible defect.
No new findings or patches: high 0, medium 0, low 0, false 0, maybe-false 0.
The follow-up recommendation is false: no high-severity patch was needed in this
follow-up, and no specific unresolved risk beyond the disclosed upstream/manual
reconciliation boundaries was identified. Initial review's recommendation was
true because its distinct patched roots included one high and ten medium items;
that recommendation was satisfied by the bounded follow-up, not silently reset.

## Auto Run Result

- Implemented the typed gateway-group owner and UUID lookup, scoped strict API
  decoding and firmware/model checks, read-only member eligibility wait, durable
  save/delete recovery and UUID-bound uncertain-save reconciliation. Shared CRUD
  and existing individual-gateway behavior remain unchanged.
- Resource files own schema/mapping/lifecycle/registration; unit/protocol/Core
  files verify strict fields, state/private replay, failure recovery, gating and
  serialization. Templates/examples/generated docs describe the public contract.
  Acceptance scripts provide per-connection certificate pinning, pre-execution
  helper binding, source immutability and independent saved-field/zero/restore
  observations. Changelog, audit, support/index and historical sprint tracking
  distinguish implemented, validated, released and upstream-limited work.
- All 27 initial observations are individually classified above. Eleven distinct
  repaired root causes (one high, ten medium) were delivered in nine handoff
  items; duplicate findings retain their own log rows. Two medium observations
  share the same pre-existing upstream first-tier-clear limitation and remain
  deferred. Nine false observations were rejected on their recorded evidence:
  uncertain-create automatic adoption is deliberately forbidden without identity
  proof; broader programme completion is not claimed; neutral product notes are
  not private consumer notes; pending narrative is not failed/executed live proof;
  negative first-tier tests are not positive clear proof; synthetic acceptance is
  not production adoption; and adjacent FRR wording reconciles prior release
  evidence rather than borrowing gateway tests.
- Parent verification on repaired digest passed: 30 named gateway root tests with
  race detection, five actual local Terraform Core failure/private-replay cases,
  fresh full Go suite, two helper security regressions, deterministic generation,
  all six default quality targets and whitespace checks. No gate was suppressed.
- Repaired appliance lifecycle passed all 16 steps on core 26.7.7 with Terraform
  1.16.5: initial tier2-only create/update, all later-tier population/clears,
  import/no-change, drift/repair, protected first-tier-clear rejection,
  same-UUID in-use deletion/recovery, real destroy and eleven independent saved
  field observations. Groups/gateways/references were globally zero before and
  after rollback; full config and IPv4/IPv6 routes restored exactly. Source and
  private helper bindings remained unchanged. Parent binds the final publication
  tree and records its separate sanitized proof before push; raw evidence remains
  private and is not a public CI artifact.
- Residual limits: populated first-tier clearing remains impossible through the
  stable API, so universal tier-clear parity is explicitly NOT complete. Unknown
  saves with persistently absent/malformed truth and unresolved uncertain creates
  require manual authoritative reconciliation. Older cores, OpenTofu, traffic
  failover/PF activation and private consumer adoption remain unverified. No
  production configuration or published release is changed by this build.
