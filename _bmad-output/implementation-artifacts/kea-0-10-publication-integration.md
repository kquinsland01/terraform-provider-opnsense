# Kea 0.10.0 publication integration

## Upstream and release inputs

Gateway PR72 merged at `198f08ffa72087657d9d1fdec5c3a0986d0ece03`.
Successful main CI caused Auto Release Tag run40352410371 to read 0.9.0,
observe no existing tag, create `v0.9.0` at that exact merge commit and dispatch
Release run40352437138. The release workflow passed; all 13 Go-supported
platform archives plus manifest, checksums and signature were present, and
Terraform Registry version0.9.0 was verified independently.

This combined PR moves the two independently verified Kea owners into the next
minor release, `## 0.10.0 (Unreleased)`. Merging it after successful main CI
requests normal automatic tagging/publication; no manual tag is required.
The existing 0.9.0 heading is dated after verified publication. Neither Kea
owner is already released merely because it has a public PR or passed tests.

## Preserved behavioral proof

- [Generic-option evidence](31-0n-kea-option-live-evidence.md): accepted commit
  `da044c934feb18e16184ee738e0a873ef6d13f28`, source digest
  `4e6c1f853ad63b0385a33f33b37c6e224bbf2b8994c8bf1fafc59d467f63b5eb`,
  14 actual Core steps, 17 independent observations, exact tracked in-use
  rejection/deletion and restoration.
- [DDNS evidence](31-0o-kea-ddns-live-evidence.md): accepted commit
  `b6c43cc0149cbccc222161747df0acf3eaa44602`, source digest
  `de2d74f4cfd7ea42a3974eff7f97a9fef133f36827c496cca62786def5ca2ca8`,
  14 actual Core steps, seven independent expected four-field observations,
  state-only deletion boundary and exact restoration.

Merging upstream main into the combined branch introduced **zero content
changes** relative to the accepted DDNS commit, which already contains the
accepted generic-option implementation. The subsequent integration changes only
release notes, current support/status metadata and generated provider index.

A Git blob-manifest comparison verified **1,278 files byte-identical** to the
DDNS checkpoint across implementation, dependencies, fixtures, scripts, examples,
resource/data-source templates/docs, workflows and build/release configuration.
Its sorted `git ls-tree -r` manifest SHA256 is
`ffdb56067c2e41cea9f3626a43dfa3af19226f40ee54ef07c2f90ffbcd3906b9`.
Protected roots are `internal`, `pkg`, `tools`, `test`, `examples`,
`templates/resources`, `templates/data-sources`, `docs/resources`,
`docs/data-sources` and `.github`, plus root implementation/module, Makefile,
DevRail, GoReleaser, Registry manifest, editor and contributor instruction files.

The old whole-tree live digests are retained as historical exact-tree evidence;
they are **not** relabeled as the new release-note revision's digest. Neither
appliance test was rerun for this metadata-only integration. Fresh whole-tree Go
and Kea race tests passed after the clean merge, and final generation, quality,
privacy and secret scans are recorded in the publication handoff. The retired
lab and its credentials were not revived.

## Ownership boundaries

Generic-option ownership is independent from ordered parent option linkage.
DDNS owns only the four general settings, not manual files, hooks or actual DNS
delivery. Its Delete RPC is state-only; normal Core planning refresh can still
recover/activate pending saves or fail before Delete. No production configuration,
state upgrade, service enablement, ownership transfer or operator adoption is
part of this public PR. Consumer inventory and plans remain private.

## Final integration verification

- Fresh full Go suite and whole-Kea race suite: passed.
- Repeated generation: identical, integration source digest
  `dedc9aa83dba9c697d43ddf35bbb9f031268cb508332bb98d24ab326f5f7ebca`.
  This metadata-integration digest is not a new appliance-test digest.
- All six quality targets: passed without skips or suppressions.
- Current inventory: 112 resources, 98 data sources and matching generated docs.
- Publication privacy review and directory/history secret scans cover the final
  combined PR; their final counts are recorded in its handoff.
