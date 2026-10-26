# Story 31.0o: Kea DDNS Settings Singleton and Data Source

## Status

done — reviewed four-field owner/lookup, fixed import identity, targeted writes
and durable recovery. Full/race Core/protocol tests passed; all 11 bounded review
repairs have zero-finding retained rechecks. Publication readiness requires the
exact final source digest and non-skipped disabled-appliance result in
[the final evidence](31-0o-kea-ddns-live-evidence.md). Merge, release, active DNS
traffic and private adoption remain separate gates.

## Priority and Entry Point

P1, before public completeness/migration positioning. Read AGENTS.md and the
consumer-neutral validation reports in `_bmad-output/planning-artifacts/`.
Use existing resource addresses/ownership conventions; no operator inventory or
private adoption plan belongs in this specification.

## Implementation Scope

Implement `internal/service/kea/ddns_settings_{resource,schema,model,data_source}.go`
and register `opnsense_kea_ddns_settings`. Own only `ddns.general`: enabled,
manual_config, IPv4 server_ip and server_port. Use targeted nested writes through
kea/ddns/set and kea/service/reconfigure. Do not write unrelated Kea subtrees or
assume filtered GET implies broader ownership. Import singleton identity `ddns`;
destroy removes state only. Preserve configured false and unresolved unknown
intent; reject zero/empty ports, invalid addresses and incomplete scalar truth
before mutation. Handle saved-but-unactivated and uncertain-save responses
with private recovery state and actual provider-restart protocol tests.

## Tasks

1. Re-read tagged 26.7.7 model/controller and the validation evidence, then write a
   complete implementation spec with exact fields, request/read shapes, defaults,
   unknown/null/empty semantics, activation and recovery paths before coding.
2. Add strict model/schema/resource/data source files above, exports, matching
   schema/example templates and generated docs. Gate unsupported versions or
   unavailable model endpoints before mutation; document required permissions.
3. Add unit and actual provider-protocol fixtures for malformed responses,
   create/update/delete/save/reconfigure failures, restart/refresh recovery,
   cancellation, imports and data-source parity. Never treat HTTP success alone
   as saved/activated state or turn validation errors into a missing object.
4. Run non-skipped real Terraform lifecycle on a disposable, identity-guarded,
   loopback-accessible target, with services/traffic disabled where applicable.
   Capture independent API/config/runtime observations and exact restoration.
5. Regenerate docs/audit deterministically, run full gates and independent review,
   update support claims only for executed proof, and add the required versioned
   Unreleased changelog entry in the implementation PR.

## Acceptance Criteria

- **Given** a valid configuration, **when** create/import/update/refresh run through
  Terraform Core, **then** fixed singleton identity `ddns` and all readable fields are
  preserved and a subsequent plan has no changes.
- **Given** drift or a populated optional field, **when** repaired or explicitly
  updated, **then** the API and Terraform state converge without losing unrelated
  fields or accepting malformed/unknown intent as an empty clear.
- **Given** a save/read-back/reconfigure failure, **when** the provider restarts and
  retries or refreshes, **then** state accurately retains committed identity and
  requested-subset recovery intent without repeating set or silently removing ownership.
- **Given** completed acceptance, **when** Terraform destroy and harness cleanup
  finish, **then** state-only destruction leaves the saved settings/configuration/runtime unchanged
  before harness restoration; the harness separately restores exact baselines.

## Verification

Focused uncached/race-enabled service tests; actual provider-protocol tests;
real Terraform import/no-change/drift/explicit-false/failure/state-only-destroy proof on disposable
26.7.7; deterministic `go generate ./...`, structure and whitespace checks,
`make check` and complete hosted CI. Legacy appliance/OpenTofu/live traffic or
private adoption are unverified unless separately executed and documented.

## Implementation notes

The sole implementation contract is `spec-31-0o-kea-ddns-settings-owner.md`.
Optional values are observed, not defaulted, including on create. Fixed-ID private
requested-field intent is persisted with known state after confirmed/uncertain
saves. Matching requested fields permit recovery activation; authoritative mismatch
reconciles without activation or another set. Core failed-create taint/replacement
planning remains normal. State-only destruction abandons pending activation and
requires operator review/reimport/manual reconciliation, not implicit rollback.

The guarded public fixture uses fixed oracle port 19535 and per-connection pinned
proxy port 19536. It requires independently supplied source/helper/oracle hashes,
private credentials/runtime and inactive disposable services. Helper hashes are observed immediately before each invocation, including
cleanup/restoration; source/runtime/helper hashes bind observed before/after
inputs during the controlled unchanged-candidate run, not an immutable snapshot.
Evidence is private and excluded
from the source digest. No VM provisioning or production access is authorized by
the fixture. A skip/failure is not live proof.

Focused implementation verification (pinned DevRail 1.12.24): DDNS uncached/race
suite, Kea golangci-lint, helper-integrity negative controls, shellcheck/shfmt,
Terraform example formatting and docs/audit generation passed. Actual Core tests
observe genuine error-returned private requested subsets across fresh servers,
including confirmed/uncertain create/update, failed read/activation, contradictory
truth, post-commit cancellation and pending state-only destroy with refresh
disabled. Normal failed-create taint/replacement planning is asserted. This is
synthetic protocol/Core proof, not appliance lifecycle evidence. Parent owns
whole-tree `make check`, deterministic-generation repeat, independent review,
non-skipped disabled-appliance execution and exact restoration.

Delete itself is zero-API/state-only. Normal Core planning refresh can read,
recover and activate matching pending saves, or fail before Delete. Only a
no-refresh path sends pending recovery directly to Delete; this does not suppress
or weaken normal Read recovery semantics. Focused review regressions also cover
changed-plan optional unknowns/fresh baselines, built-in dependency unknowns,
ambiguous authoritative JSON and expected native Terraform/oracle observations.
