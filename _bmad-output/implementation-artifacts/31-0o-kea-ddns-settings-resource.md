# Story 31.0o: Kea DDNS Settings Singleton and Data Source

## Status

backlog — stable-source/direct-API contract validated; implementation and real
Terraform acceptance remain required. This story does not confer provider support.

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
destroy removes state only. Preserve configured false/zero/unknown intent with
model-valid validation. Handle saved-but-unactivated and uncertain-save responses
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
  Terraform Core, **then** the UUID/singleton identity and all readable fields are
  preserved and a subsequent plan has no changes.
- **Given** drift or a populated optional field, **when** repaired or explicitly
  cleared, **then** the API and Terraform state converge without losing unrelated
  fields or accepting malformed/unknown intent as an empty clear.
- **Given** a save/reconfigure/delete failure, **when** the provider restarts and
  retries or refreshes, **then** state accurately retains committed identity and
  recovery intent without duplication or silent deletion of an in-use owner.
- **Given** completed acceptance, **when** Terraform destroy and harness cleanup
  finish, **then** no test entries remain and singleton/settings/runtime baselines
  are restored exactly; state-only singleton destruction must not alter services.

## Verification

Focused uncached/race-enabled service tests; actual provider-protocol tests;
real Terraform import/no-change/drift/clear/failure/cleanup proof on disposable
26.7.7; deterministic `go generate ./...`, structure and whitespace checks,
`make check` and complete hosted CI. Legacy appliance/OpenTofu/live traffic or
private adoption are unverified unless separately executed and documented.
