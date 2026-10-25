# Story 31.0e — API-corrected disposable proof (2026-10-20)

Status: **implementation proven; in review**. Independent review/PR publication is not implied. The governing corrected local build is finalized; maintainer PR review/merge and release remain separate gates. Follow-up review is recommended as recorded in that spec.

## Final promoted-tree identity

- Digest: `514e0d9a632f39fd098903e9ee9b6994e9e521c9e284b9f12f2399f9c89cc214`.
- Core: **OPNsense 26.7.7**.
- Disposable VM: `disposable acceptance guest`, UUID `d630950e-d4dd-4df0-9fcb-fac6b086bded`.
- The runner executed the root-owned live Proxmox identity probe and checked live name/UUID, isolation and exact QEMU HOST_FORWARD ownership before obtaining TLS SHA256 over trusted SSH from node-loopback18443. The host runner and the exact container API endpoint both matched that independent fingerprint before credentialed HTTP traffic. Credentials came from the mode-600 temporary runtime env file; no values are retained here.
- Container traffic used the root-owned tunnel at `host.docker.internal:18444`. No production appliance or production bridge was used. Provisioning/tunnel/credential retirement remains root-owned.
- Origin/main PR62's planning/public-document changes were integrated before the final digest. FRR2 compatibility gating, Stories 31.0k/31.0l and Unbound completion are preserved.

The digest covers the governing corrected spec, its loaded Epic31 context, implementation, all transitive provider Go source/build inputs, shared client/conversion code, WireGuard tests, templates/examples/generated docs, audit/status/roadmaps and runners including their sourced logging library. This evidence record is outside the digest to avoid a self-referential hash; updating the record did not change the verified digest.

## Executed local gates on the promoted tree

All executed in DevRail **1.12.24**:

- Fresh `go test -count=1 ./internal/service/wireguard/... ./pkg/opnsense/... ./internal/tfconv/...`: pass.
- Fresh `go test -count=1 ./...`: pass.
- Repeated `go generate ./...`: deterministic; consecutive generation digests equal the final digest above.
- `go test -count=1 ./internal/coverageaudit`: pass; exact comparison against the branch baseline changes only the six resource/data-source WireGuard rows.
- `bash test/scripts/validate-structure.sh`, changed-runner `shellcheck -x`, and `git diff --check`: pass.
- `DEVRAIL_IMAGE=ghcr.io/devrail-dev/dev-toolchain:1.12.24 make check`: **all six gates pass**, 51.139 seconds (lint, format, tests, security, scan, docs; none skipped).
- Explicit WireGuard working-tree `gitleaks dir --redact`: zero findings. A final exact publication-file snapshot (71 changed/untracked intended files, excluding the retained review patch) also has zero findings. An additional whole-tree directory scan reports 57 inherited matches outside the changed WireGuard candidate: vendored BMAD examples/manifests, historical research, unchanged ddclient/iface examples and existing Kea/Unbound private-state identifiers. Those out-of-scope matches were not suppressed or represented as a clean whole-tree scan. The existing full-history scan remains enabled. Four inherited exact Unbound private-state-name fingerprints were classified under the existing `.gitleaksignore` false-positive policy; no rules, coverage or Git history were weakened.

## Executed exact-digest live proof

`OPNSENSE_WIREGUARD_PARITY=1 test/scripts/run-story-31-0e-acceptance.sh` passed the full **21-step** Terraform Core test, non-skipped, in **72.21 seconds** on the final digest. The preserved advanced 17-step lifecycle remains intact; four minimal create/no-change/unrelated-update/no-change steps exercise omitted numeric and public-key paths. Earlier successful digests are historical, not final-candidate evidence.

Executed contracts:

- Import real disabled `wireguard_settings`, verify import/no-change plan, and perform a real Terraform Core Update. Only the isolated test workspace's prior-state fixture was changed to `enabled=true`; plan/apply for this one controlled transition disabled refresh and required an Update action. The real API request was always `enabled=0`, followed by real read-back/reconfigure. No daemon-enable request occurred.
- Minimal disabled peer/server Core create with omitted endpoint port/keepalive, server port/MTU and server public key; unrelated name updates and no-change plans preserve assigned instance and authoritative public-key truth. The released add API may return blank public key and does not derive one from private key; no unsupported nonblank-key expectation is claimed.
- Independently compare raw API scalars and selected-map fields against resource state, separately from shared resource/data-source decoding.
- Create disabled marker-tagged peer/server with generated ephemeral keys, typed multiple IPv4/IPv6 addresses, endpoint/keepalive, DNS/MTU, route suppression/gateway and server-owned peer binding.
- Read/import/no-change plans, complete readable resource/data-source comparisons, stable resource-only PSK/private key handling, out-of-band peer/server name/numeric drift, detected drift and repair.
- Populated-to-empty server DNS/tunnel/peer-binding/gateway transitions, empty-state no-change plan, repopulation and refresh.
- Snapshot settings immediately before successful final destruction. Singleton destroy leaves settings byte-semantically unchanged.
- Terraform's own deletes leave **zero tagged servers and clients before rollback**. Final cleanup searches also find zero; rollback cannot mask a provider-delete failure.
- Restore the exact general-settings snapshot and compare equal. Generated secret/credential exact-value artifact scans pass; raw runner artifacts are removed. Temporary Terraform state is removed with the test workspace. Public docs accurately state that Sensitive resource keys are stored in normal resource state and need a secure backend.

Sanitized proof lines:

```text
WIREGUARD_SINGLETON_UPDATE terraform_core=true enabled=0 refresh_disabled_fixture=true
WIREGUARD_TERRAFORM_DESTROY namespace=server total=0
WIREGUARD_TERRAFORM_DESTROY namespace=client total=0
WIREGUARD_SINGLETON_DESTROY unchanged=true
WIREGUARD_PARITY_CLEANUP namespace=server total=0
WIREGUARD_PARITY_CLEANUP namespace=client total=0
WIREGUARD_SETTINGS_RESTORE equal=true
--- PASS: TestAccWireGuardParity (72.21s)
WIREGUARD_PARITY_EVIDENCE digest=514e0d9a632f39fd098903e9ee9b6994e9e521c9e284b9f12f2399f9c89cc214 version=26.7.7 vm_name=disposable acceptance guest vm_uuid=d630950e-d4dd-4df0-9fcb-fac6b086bded restore_equal=true singleton_destroy_unchanged=true terraform_deleted=0 final_leftovers=0 secret_scan=pass
```

Exactly six audited WireGuard rows are now covered (client/peer uses `covered-different-name`); inventory is **109 resources / 95 data sources**. No duplicate client owner, unaudited field or production change was introduced.

## Released-API corrections and failing-contract history

1. **Create instance must be omitted.** The historical explicit `instance=""` request failed with `server.instance: A value is required`. The bounded omitted-key control saved and returned authoritative `instance="0"`, then deleted cleanly. Released [AutoNumberField](https://github.com/opnsense/core/blob/stable/26.7/src/opnsense/mvc/app/models/OPNsense/Base/FieldTypes/AutoNumberField.php) assigns the default; an explicit empty post overrides it. The corrected implementation omits the create wire key, reads the assigned instance and retains it on update. Exact HTTP request tests cover omission and non-empty assigned-instance retention. The original blocked spec remains historical; the corrected continuation governs.
2. **Peer updates must preserve live server associations.** Released [ClientController::setClientAction](https://github.com/opnsense/core/blob/stable/26.7/src/opnsense/mvc/app/controllers/OPNsense/Wireguard/Api/ClientController.php) treats an omitted `servers` key as an empty selection and detaches the peer from all servers. Live drift repair exposed this. Update now reads and passes through the authoritative API backreference under the WireGuard owner lock; malformed reads prevent mutation. This is preservation, not independent configured ownership: server `peers` remains authoritative. Create still omits the association key.
3. **Exact empty-list sentinel is authoritative.** Released [BaseSetField::getNodeData](https://github.com/opnsense/core/blob/stable/26.7/src/opnsense/mvc/app/models/OPNsense/Base/FieldTypes/BaseSetField.php) serializes an empty AsList field as `{"":{"value":"","selected":1}}`. The populated-to-empty live test exposed the old decoder's invalid empty-IP result. Only this exact sole-entry sentinel normalizes to an empty set; malformed/mixed sentinels, missing selector metadata, quoted/invalid selector numbers and unresolved configured members diagnose.
4. **Harness corrections do not weaken field comparisons.** Terraform's synthetic `%` attribute-count metadata differs because resources contain secret fields and data sources intentionally do not; it is excluded from field comparison, not an API field. The state fixture locates the framework's isolated `work*` directory and uses rooted filesystem access. Destroy comparison is failure-aware when an earlier lifecycle failure prevented the success-path snapshot, and the runner still requires the affirmative singleton-destroy proof for a passing run.

Earlier failing runs retained zero tagged objects and restored disabled settings exactly. The historical secondary singleton-destroy diagnostic reflected an absent success-path snapshot, not an appliance mutation. All of those implementation/harness failures are resolved by the final executed proof above.


## Review repair 1 — executed boundary proof

- Config-null-only resolution distinguishes omitted Optional+Computed numerics
  from configured unknown intent. The minimal live Core steps pass both omitted
  creates, unrelated updates and no-change plans, including omitted server key.
- Confirmed saves retain newly rotated PSK/private keys across failed activation.
  Direct owner tests and protocol ApplyResourceChange→serialized state/private→
  fresh provider ReadResource tests prove blank API reads retain the saved key and
  recovery does not repeat the save. Genuinely ambiguous unreadable keys instead
  retain explicit private reconciliation metadata and an unconfirmed null public
  secret with diagnostics; no old/new key is silently claimed confirmed.
- Pending changed-plan Update recovery diagnoses a required fresh plan/retry.
  Singleton ambiguous-save failure persists a pending marker through protocol
  serialization/provider restart even when read-back cannot establish the result;
  refresh performs activation/read reconciliation without another save.
- Both owners' direct Delete tests cover initial ambiguous response/failed read
  and confirmed delete/failed activation, retained state/private markers, and
  retries for surviving object, authoritative absence, read failure and activation
  failure. Singleton destroy remains state-only, including pending private state.
- Registered v0 callbacks and protocol UpgradeResourceState(v0) preserve exact
  identity, all CIDR/association members, numeric ports/keepalive, server private
  key and legacy description. Realistic Config omits computed-only values. The
  protocol then plans/applies in-place default normalization, refreshes, and
  reaches an exact no-change/no-replacement plan without editing user state.
- Concurrent resource Create entry points are handshake-tested through ambiguous
  recovery. Isolated negative controls removing each actual owner lock fail both
  cases; changing the version0 registration keys likewise fails callback/protocol
  upgrade tests. No production source was changed for these negative controls.
- Missing/malformed/mismatched independent TLS fingerprints reject before HTTP
  requests. Digest tests verify transitive provider helper and logging-library
  changes alter the hash. No scanner rules/history coverage were weakened.
- Public docs provide explicit association-preserving before/after migration,
  core24.1 introduction versus validated26.7.7, and a workable same-UUID supported
  release/re-import workflow after independent reconciliation of unreadable keys.
  No OpenTofu or earlier-firmware execution is claimed.

During this repair, a first harness run used the host-loopback URI inside Docker
and failed before mutations; the runner now selects the host alias and attests its
exact TLS certificate inside the container before authentication. A later stricter
harness assertion incorrectly expected addServer to derive a public key; released
source and API truth show blank is valid. That unsupported assertion was removed,
while independent raw API equality and before/after key/instance stability remain.
The URI failure occurred before any mutation; the failing derived-key assertion run explicitly proved immediate/final zeros and exact settings restoration.

The retained `31-0e-review-pass1-attempt.patch` is reference-only, outside the digest,
and must not be published. All PR62 planning content remains preserved. Review,
commit/PR publication and root-owned disposable VM/tunnel/credential retirement
remain parent-owned; this record does not claim those steps completed.

## Final publication candidate — parent verification

After review pass2 bounded patches, the parent froze all covered source and metadata at the digest above. Parent directly executed named verbose focused tests, double generation with no unstaged diff, audit/structure/whitespace, fresh `go test -count=1 ./...`, full6 DevRail gates (51.139s), publication snapshot scan(0 findings/71files), and the exact-digest21step live runner(72.21s). Six controlled/protocol matrix rows had affirmative named PASS lines; the seventh cleanup row had the non-skipped live PASS and every required restoration/zero proof.

Final patches diagnose configured unknown keys before traffic, compare recovery CSV fields as sets, validate nonblank returned secrets without echoing them, clear authoritative secret uncertainty journals, fail artifact scans closed on grep errors, and assert multiple associations. Zero/multiple exact recovery candidates diagnose without adopting a UUID. Migration docs explicitly preserve existing PSKs and show matching server public-key provisioning.

The native agent-thread limit required independent reviewer reuse for some pass2 layers. Follow-up maintainer review is recommended for the Config-aware key guards, normalized recovery/cardinality and secret-journal interactions; this record does not claim four fresh reviewer contexts or an unexecuted post-patch fresh-context review. No provider release or production change occurred.

## Disposable retirement and publication

After final proof, root reverified the exact VM name/UUID, isolated net0 and sole disposable VM disk before shutdown/destruction. Cluster inventory and local-lvm content confirm disposable VM/config/disk absence. The exact recorded SSH forwarding process was terminated; temporary credentials/runtime moved to private macOS Trash with mode700 preserved. Official image cache was retained. No production VM or firewall was changed.

Implementation [PR63](https://github.com/matthew-on-git/terraform-provider-opnsense/pull/63) is open for maintainer review. Merge/release remain separate gates. Reconciliation with merged PR62 ancestry preserved the final verified digest exactly. This retirement/publication note is outside digest scope and does not change implementation proof.
