# Story 31.0o — Kea DDNS publication-tree proof

## Result and binding

The non-skipped `TestAccDDNSSettingsOwner` passed all **14 Terraform Core
steps in 87.20 seconds** on isolated stable **OPNsense 26.7.7**. The exact
publication source digest was:

```text
de2d74f4cfd7ea42a3974eff7f97a9fef133f36827c496cca62786def5ca2ca8
```

The pinned DevRail `1.12.24` container used actual Terraform Core `1.16.5`
and the repository-selected Go `1.26.10` toolchain. Source/runtime/helper
hashes matched before and after the controlled unchanged-candidate run.
Every consumed oracle response matched the independently reviewed server-source
attestation; the parent's six-input private helper closure stayed unchanged.
These are observed hash bindings, not a claim of adversarial immutable execution.
This evidence artifact is deliberately excluded from the source digest to avoid
a recursive hash; implementation, fixtures, helpers, templates, generated docs,
changelog, support tracking and the final specification are included.

The initial lifecycle also passed 14 steps in 88.02 seconds at digest
`eea64e5f2414d7b6fbfa4ccd25de9252ae047e8066a7d60feb20931c56d8799c`.
That earlier result is retained separately and does not validate the repairs or
the publication digest above. The final run independently passed all gates.

## Exercised outer surfaces

- Real Core created ownership of the pre-existing `ddns` singleton, imported
  fixed identity, verified no-change plans and preserved omitted populated
  settings. Explicit true-to-false manual-config intent and valid documentation
  IPv4/`any`/port changes converged. DDNS enabled remained false throughout.
- Both resource and lookup were checked against exact expected native Terraform
  values for all four attributes and fixed ID, not merely against each other.
- Seven independent expected four-field appliance observations passed while
  DHCPv4/DHCPv6 and unowned DDNS fields remained unchanged.
- External port drift was observed and repaired by Terraform. A zero-port
  configuration was rejected; a skip or failed attempt was not accepted as proof.
- The final Core step removed resource/lookup ownership. A pre-rollback receipt
  compared the changed singleton's saved settings, complete configuration, routes
  and runtime files/metadata before and after removal. All were unchanged; this
  was not a vacuous comparison against the original restored baseline.
- Exactly one explicit state-only receipt was required and observed. Core's
  testing helper skips its redundant post-test destroy callback once the final
  step has already emptied state. No second receipt is claimed.

**Delete boundary:** the provider Delete RPC is zero-API/state-only, including
pending recovery, unconfigured clients and cancellation. An ordinary Core
planning refresh can read/recover/activate a verified pending save or fail before
Delete; the entire `terraform destroy` command is not a zero-call guarantee.
Actual Core tests separately prove normal-refresh behavior and no-refresh
pending deletion. The live success path had no pending activation at removal.

## Independent restoration and isolation

| Gate | Observed result |
|---|---|
| Original configuration/routes/settings/runtime snapshot attestations | Independently recomputed and matched |
| Settings, complete configuration and IPv4/IPv6 routes | Exact original baseline restored |
| Five runtime files | Exact byte/existence and mode/UID/GID restoration |
| Unrelated Kea configuration | Preserved throughout observations and cleanup |
| DHCPv4, DHCPv6 and DDNS daemons | Disabled/inactive throughout |
| Global option/subnet/reservation counts | All three zero at snapshot, before rollback and after restoration |
| Source/runtime/helper inputs | Observed before/after bindings unchanged |
| Oracle server-source identity | Expected attestation on every consumed response |
| Test infrastructure | Exact disposable VM/config/disk, three oracle processes and tunnel retired; forwarding/listening endpoints absent |
| Temporary access material | Only generated lab credentials and runtime access copies retired; operator keys and source image untouched |

Raw observations, exact appliance identity, private helper implementation and
credentials remain outside this public repository. No production configuration,
upgrade, traffic or adoption action occurred.

## Offline and review proof

Fresh whole-tree Go tests and whole-Kea race tests passed. Actual protocol/Core
coverage includes confirmed/uncertain saved-state persistence through fresh
servers, genuine pending-update retry without another set, post-commit
cancellation, failed-create taint, default-refresh versus no-refresh destroy,
built-in dependency unknowns, and an out-of-band optional-field change between
plan and apply. Strict owned/envelope JSON rejects duplicate/case aliases without
losing pending state or activating malformed truth.

Plan-time shared address validation, exact Terraform expected-value checks,
missing/false route attestations, per-response source-hash negatives, helper
mutation negatives and source-symlink rejection run under ordinary Go tests.
Two generation passes produced identical digest-bound output. All six quality
targets passed with no skips or new scanner exclusions. The exact 36-file
publication directory scan found zero secrets before this evidence was added;
this artifact and the committed history are scanned again before fork backup.

Four fresh independent layers produced 17 individually classified observations:
10 medium, four low and three false. Eleven bounded repair roots (seven medium,
four low) resolved 14 observations; all three rejected claims retain their
individual refutations in the specification. Two retained bounded rechecks then
reported no remaining failures; they are not represented as fresh review layers.
Nothing was deferred.

Failed quality attempts remain distinct: the initial formatter failure and the
reviewed validator's two formatting-only lines were corrected using container
formatters. The default Trivy database mirror returned HTTP404; using its official
database source allowed the complete enabled scan to pass. No failing check was
suppressed and no scanner exception was introduced.

## Limits and handoff

This proves stored general-settings lifecycle on the named version, not actual
DNS packet delivery, daemon enablement, manual configuration files, hooks,
Control Agent, ordered option linkage, older cores or OpenTofu. Shared Kea
reconfigure is not a DDNS-only process claim. Merge, hosted checks, automatic
release/Registry publication and private adoption remain separate gates.
The upstream base and executable release-version input must be reconciled before
opening a public implementation PR; an unreleased local branch is not published
provider support.
