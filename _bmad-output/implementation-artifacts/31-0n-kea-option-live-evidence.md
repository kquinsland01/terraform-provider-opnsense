# Story 31.0n — Kea generic-option lifecycle evidence

## Gate status

The final repaired publication tree passed the complete disabled-appliance
Terraform lifecycle, including real deletion and exact restoration. Earlier
initial and repaired runs are retained separately below. Local implementation
and live proof are complete; upstream base/release-input reconciliation, public
PR hosted checks, merge and publication remain gated. No released-support claim
is made here.

## Initial exact-tree run

- Source digest: `3114c7a2465964efa495cd8cb837594ee53e7d3456f02f813735a3ab0314d913`.
- Stable core: OPNsense `26.7.7`, with the actual compatible eight-field option
  model. Terraform `1.16.5`, Go `1.26.10`, toolchain
  `ghcr.io/devrail-dev/dev-toolchain:1.12.24`.
- Actual non-skipped `TestAccDHCPv4OptionOwner`: **PASS**, 145.94 seconds,
  **14 Terraform test steps**. Both runner and outer invocation exited zero.
- **16 independent eight-field observations** compared the tracked main and
  unrelated companion options, rather than relying only on provider state.
- Create defaults, populated values, UUID import/no-change, omission-preserving
  required-field updates, explicit false/matching clears, encoding-only matching,
  mutable unique description, lookup parity and drift repair passed.
- A harness-owned official-API subnet relation targeted the tracked UUID.
  Expected deletion failure retained that identity; unlinking permitted ordinary
  Terraform deletion. Review requires a stronger response-specific dependency
  oracle in the repaired candidate rather than reusing this initial assertion.

## Deletion and restoration

Terraform's own final destroy completed. Its independent check verified zero
synthetic options, subnets and reservations **before harness cleanup**. Separate
before-rollback and after-restoration checks again found all three counts zero.
The unrelated companion remained correct throughout intermediate updates.

The parent independently verified exact saved configuration, full settings,
IPv4/IPv6 routes and runtime-file restoration. DHCPv4, DHCPv6 and DDNS stayed
inactive. Source, standalone guard/oracle helpers, private runtime input and the
independently reviewed six-input remote-helper closure remained unchanged.
The TLS pin was checked on each actual upstream connection before credentials.
Raw evidence, configuration, appliance identity and credentials are private and
are not publication artifacts.

## Offline and review boundaries

Parent fresh full Go tests, uncached Kea race tests and two identical generations
passed. All six quality targets passed without skips or suppression, and the
initial 38-file publication scan found zero secrets. A parent ShellCheck
invocation initially omitted `-x` for sourced-file discovery; the corrected
invocation passed without source changes or rule exclusions.

All four review layers were collected before triage. Three nonimplementing
reviewers started context-free. The runtime rejected a fourth fresh reviewer at
its thread limit; the verification-gap reviewer was reused for descriptive
alignment, not represented as fresh. Every observation has an individual verdict
in the story specification. Repairs, follow-up verification and the final
publication digest will be recorded separately below.

## Repaired immutable candidate

- Digest: `08aecba97996cb5221e1354559ed7fed0c4841993b0c9d5b7336f513f2f94b7d`.
- Actual Core **14/14 PASS**, 149.08 seconds, with both runner and outer exit zero.
- **17 independent eight-field observations** passed, including the immediate
  unchanged option after the exact tracked POST returned HTTP500. The same UUID
  subsequently returned HTTP200 on ordinary deletion after unlinking.
- Real destroy and three namespace counts before cleanup/rollback and after
  restoration passed. Configuration, settings, IPv4/IPv6 routes and all five
  runtime-file baselines restored exactly; all three daemons remained inactive.
- Source, runtime inputs, guard/oracle and the reviewed six-input private helper
  closure stayed unchanged. No private configuration or identity is published.
- Fresh full Go/race tests, deterministic generation and all six quality targets
  passed. Two new harness lint constructions initially failed; bounded-root file
  access and literal executable construction fixed them without suppression.
  The failed result is retained separately. The refreshed 40-file directory scan
  found zero secrets. Two retained nonimplementing reviewers found no remaining
  issues in bounded follow-up source inspection; neither executed the live run.

## Final publication candidate

- Final digest: `4e6c1f853ad63b0385a33f33b37c6e224bbf2b8994c8bf1fafc59d467f63b5eb`.
- Actual Core **14/14 PASS**, **148.05 seconds**, runner and outer invocation both
  exited zero. The publication source was unchanged throughout.
- **17 independent all-eight-field observations** passed. The tracked POST
  delete returned exactly one HTTP500 while the option remained unchanged; after
  unlinking, ordinary same-UUID deletion returned HTTP200.
- Terraform actually destroyed the graph, with independent zero synthetic
  options/subnets/reservations before harness cleanup. Those three counts were
  again zero before rollback and after restoration.
- Full saved configuration, DHCPv4/DHCPv6/DDNS settings, IPv4/IPv6 routes and all
  five runtime-file byte/existence baselines restored exactly. All three daemons
  remained inactive. Source, private runtime, guard/oracle and six-input helper
  closure hashes stayed unchanged. Per-connection pinning preceded credentials.
- Final deterministic generation and all six quality targets passed. The
  container-only final Git whitespace check could not resolve its external
  worktree metadata; the host worktree check passed separately. The failed outer
  check result is retained, not represented as a successful invocation. Final
  publication-directory and committed-history scan results are checked separately.
- Gateway PR #72 remained open at final local verification, with latest published
  tag v0.8.1. Reconcile its eventual merge and the release version before opening
  the dependent public PR; no duplicate-scope merge or manual tag is claimed.

Ordered subnet/reservation option linkage, active packet delivery, packet-level
force behavior, older cores, OpenTofu and operator adoption remain separate.
