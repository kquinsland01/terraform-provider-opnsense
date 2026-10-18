# Story 31.0a: Firewall and NAT Field Parity

## Status

done

## Priority

P0 — complete before Stories 31.1–31.3.

## Evidence and Scope

The pinned v0.26.0 audit found material fields missing from the existing stable endpoint owners: alias `interface`/`path_expression`/`stats`; filter-rule tagging, priority, schedules, state limits/timeouts, TCP/ICMP, reply-to, shaping, and HA-sync controls; and multi-interface port forwards. Extend the existing resources/data sources—do not create competing owners. Existing CRUD/apply endpoints under `firewall/alias`, `firewall/filter`, and `firewall/destination_nat` are stable and already used by this provider.

## Acceptance Criteria

- **Given** the pinned schemas and current OPNsense models, **when** the three local schemas are extended, **then** every audited field is either round-tripped or explicitly rejected with source-backed rationale.
- **Given** imported advanced rules, **when** refresh and no-change plan run, **then** state matches API read-back without flattening or losing nested semantics.
- **Given** create/update/delete, **when** mutations finish, **then** aliases call direct reconfigure and filter/DNAT rules call the single direct firewall apply endpoint under the global mutation lock; no removed savepoint/revision/cancel-rollback endpoint is called.

## Verification

Review-repair implementation verification completed on 2026-10-18:

- `go test -count=1 ./...`
- `go generate ./...`
- `bash test/scripts/validate-structure.sh`
- `DEVRAIL_IMAGE=ghcr.io/devrail-dev/dev-toolchain:1.12.24 make check`
- `git diff --check`

Review repair 2 reran the same local gates after tightening legacy-scalar
planning, unknown-value encoding, source-backed alias/filter validation,
field-specific selected-list decoding, and post-create recovery. The focused
HTTP/framework suite additionally covers singleton scalar changes, non-first
scalar preservation, successful and failed recovery reads, validation failures,
and registration of all three resource-level validators.

Review-2 acceptance run `story-31-0a-live-26.7.7-review2` passed on a
disposable OPNsense 26.7.7 appliance. Independent post-run searches returned
zero Story31a matches for alias, filter, and `d_nat`, confirming cleanup of all
acceptance objects.

Final reviewed acceptance run `story-31-0a-live-26.7.7-final-rerun1` passed on
the disposable OPNsense 26.7.7 appliance after the converged follow-up review.
The complete local DevRail gate and deterministic generation checks passed,
and independent post-run searches again returned zero Story31a matches for
alias, filter, and `d_nat`.

Review-iteration acceptance run
`story-31-0a-live-26.7.7-review1-rerun2` passed on a disposable OPNsense
26.7.7 appliance. It covered advanced alias drift detection and repair, filter
drift plus a configured update, DNAT interface-membership update,
resource/data-source parity, imports, and zero-diff plans at stable points.
Independent post-run API searches returned zero matching rows from alias
`searchItem`, filter `searchRule`, and `d_nat/searchRule`. The six Story 31.0a
resource/data-source audit rows are therefore classified `covered`.
