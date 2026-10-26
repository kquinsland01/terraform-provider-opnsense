# Story 31.0n: Kea generic DHCPv4 option owner and lookup

Status: review / implemented and live-validated locally; upstream merge, hosted
checks and release publication remain separate. No active-packet claim is made.

## Implementation

- Registered `opnsense_kea_dhcpv4_option` resource and UUID lookup for eight fields.
- Stable core >=26.7.7 and compatible model/catalog gate, strict selected-key and
  string-boolean mapping; only successful actual empty controller item responses
  establish absence. Required code/data/description remain literal/required;
  description is mutable and unique in the appliance model.
- Optional omission preserves imported/prior values; create-only defaults,
  explicit false/matching clears and unresolved unknown fail-closed behavior.
  Partial update wire maps contain required plus configured optional fields.
- Single-attempt writes with copied authenticated transport and retained GET
  retry policy; shared client serialization; preflight prevents known-absent
  UUID upsert without claiming atomicity against outside edits.
- UUID-bound private save/delete intent, requested-field-only comparison,
  strict recovery-byte validation, confirmed identity retention, authoritative
  deletion checks and normal refresh-before-destroy recovery paths.
- Public resource/lookup templates, examples and generated docs; guarded private
  live runner reusing the per-connection pin primitive; independent saved-field
  observations and full config/routes/runtime restoration interface.

## Verification gates

Verification results are reported to the parent separately while work proceeds.
The final sanitized exact-tree live evidence is a separate artifact and excluded
from the source digest to avoid recursive self-hashing. A skipped/failed or
rollback-only attempt is not a passing owner proof. Parent controls private
runtime inputs and disposable VM actions; implementation never provisions or
accesses production.

## Boundaries and follow-up

The owner does not manage option_data, singleton/DDNS settings, parent links,
hooks or operator integration. The tagged generator does not consume stored
force; no packet-level force behavior is claimed. Parent relation selection maps
are label-sorted and cannot reconstruct persisted order. Story 31.0p owns ordered
linkage research/implementation through existing parent owners, not a duplicate
subnet owner. Older-core/OpenTofu/production adoption remain unverified.

Current release input is appended to 0.9.0 (Unreleased). The parent must reconcile
the stacked gateway change and actual published tags before publication so an
existing tag cannot skip the automatic release; no manual tag is created here.

## Offline matrix audit

All named tests below are registered in ordinary Go test execution. Local Core
uses Terraform 1.16.5 and fresh protocol servers with persisted state/private
bytes; it contacts only synthetic loopback APIs. The toolchain is
`ghcr.io/devrail-dev/dev-toolchain:1.12.24`, Go 1.26.10.

| Contract row | Named passing offline coverage / separate live gate |
|---|---|
| Core/model gate | `TestDHCPv4OptionProtocolUnsupportedCreateZeroWrites`, `TestDHCPv4OptionProtocolMalformedCatalogZeroWrites`; no writes for unsupported versions/templates/catalogs. |
| Stored fields | `TestDHCPv4OptionProtocolCreateRestartRecovery` compares all lookup/state fields; `TestDHCPv4OptionProtocolAPIFieldValidationRetainedWithoutActivation` retains field-specific upstream rejection without retry/activation. Local Core updates description under the same UUID. |
| Omitted/unknown intent | `TestDHCPv4OptionCoreImportOmissionClearsAndNoChange`, `TestDHCPv4OptionOmittedPlanPreservesStateAndExplicitUnknown`, `TestDHCPv4OptionProtocolUnknownRequiredOptionalZeroWrites`, `TestDHCPv4OptionProtocolPlanExplicitUnknownNeverInheritsPrior`; omitted update keys, populated imports, explicit false/clear and unresolved writes are checked. |
| Matching | `TestDHCPv4OptionMatchingPairAndEncodingOnly` checks valid/invalid pairs and encoding-only acceptance; actual Core explicit matching clears converge. |
| Strict read/lookup | `TestDHCPv4OptionProtocolStrictEveryFieldAndAbsence`, `TestDHCPv4OptionSelectorSentinelsFlagsAndFiniteCatalog`, all computed-field comparison in create recovery; malformed/null fields and wrappers retain state, only actual controller `[]` establishes absence. |
| Create identity | `TestDHCPv4OptionProtocolUncertainAddMatchesAndNoAutomaticRetry`, `TestDHCPv4OptionProtocolCreateRestartRecovery`; zero/multiple/existing descriptions fail closed, one exact new match recovers, HTTP500 add is not blindly retried. Same-code distinct descriptions have a separate live companion fixture. |
| Update/upsert/recovery | `TestDHCPv4OptionProtocolAbsentUpdateNoUpsert`, `TestDHCPv4OptionProtocolUnknownSaveRestartBindsAttempt`, `TestDHCPv4OptionProtocolUnknownSaveComparesOnlyRequestedWireFields`, `TestDHCPv4OptionProtocolCorruptUUIDPayloadRecoveryFailsBeforeMutation`; actual local Core save activation/match/mismatch failures persist error-returned state/private across fresh servers. |
| Delete/restart | `TestDHCPv4OptionProtocolAmbiguousDeleteUnknownRestartChecksTruthFirst`, `TestDHCPv4OptionProtocolDeletedMarkerReappearedUUIDRetained`; actual local Core delete activation and still-present uncertain-delete failures recover through ordinary refresh-before-destroy. HTTP500 in-use rejection on the tracked UUID remains a separate live fixture gate. |
| Import/drift/concurrency | `TestDHCPv4OptionCoreImportOmissionClearsAndNoChange`, `TestDHCPv4OptionProtocolConcurrentDiscoverySerializedAndCanceled`, `TestDHCPv4OptionConfiguredReadRetriesAndSinglePOST`; local Core contradictory authoritative drift reconciles then explicitly repairs on a fresh plan. |
| Appliance acceptance | `TestAccDHCPv4OptionOwner` passed all 14 Core steps on the disabled isolated 26.7.7 appliance, including 17 independent eight-field observations, tracked-UUID HTTP500 in-use rejection, real deletion and exact restoration. Immutable publication-tree result is recorded separately in `31-0n-kea-option-live-evidence.md`; it is not merge/release proof. |

Normal-path `TestDHCPv4OptionAcceptanceHelperSecurity` also runs the shared
per-connection changed-certificate-before-credentials test and both helper-hash
fail-before-execution controls. `TestDHCPv4OptionAcceptanceShellAndExampleChecks` registers shellcheck, shfmt
and Terraform example formatting in the ordinary Go gate. Repaired local Core
failed-create/taint, confirmed-create bad-read, committed cancellation, pre-existing
UUID, redirect and canonical-marker regressions also pass; see the spec triage.
