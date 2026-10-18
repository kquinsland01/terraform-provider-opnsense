---
title: 'Story 31.0: Complete Provider Coverage Audit'
type: 'chore'
created: '2026-10-18'
status: 'done'
baseline_revision: '3c05a54d9902512738658fef27e99377fe6d8550'
review_loop_iteration: 0
followup_review_recommended: true
context:
  - 'AGENTS.md'
  - '_bmad-output/implementation-artifacts/epic-31-context.md'
warnings:
  - 'oversized'
deferred: []
---

<intent-contract>

## Intent

**Problem:** Provider breadth is currently described by stale counts and name-level comparisons, so maintainers cannot tell whether an established-provider capability is materially covered, deliberately different, buildable, or upstream-blocked.

**Approach:** Pin the established ecosystem baseline, commit a machine-readable semantic mapping for every published baseline surface, validate it against this repository's registrations/docs, publish a human audit, and turn every confirmed buildable gap into a prioritized implementation story before positioning or migration work continues.

## Boundaries & Constraints

**Always:** Pin `browningluke/opnsense` v0.26.0 at commit `33c2a03c2bc1a7339cb5aa5b611e15014e27f7ba`; cover all 47 resources, 45 data sources, the OpenVPN key-generation ephemeral resource, and provider retry settings exactly once; compare semantic fields, lifecycle, endpoint ownership, sensitive/write-only behavior, and differently named equivalents; use only evidence available from pinned schemas/source, this repository, and stable supported OPNsense APIs; keep the one-provider-per-appliance/object-domain direction explicit.

**Never:** Treat equal names or counts as proof of coverage; create duplicate stories for deprecated Kea aliases; classify a gap as buildable without stable endpoint/model/reconfigure evidence; recommend two providers manage the same object; turn Story 31.0 into positioning copy, migration guidance, or product implementation.

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
|----------|---------------|---------------------------|----------------|
| Complete baseline | Pinned 47/45/1 baseline plus provider configuration | Every surface occurs once with local equivalent(s), allowed classification, field/lifecycle evidence, and source links | Validator fails on missing, duplicate, unknown, or stale rows |
| Different naming | Equivalent concept under another local type or several broader types | Record explicit mapping and material field differences | Never infer coverage from filename similarity alone |
| Deprecated alias | Baseline generic Kea v4 alias duplicates its typed successor | Classify `intentionally-different` with deprecation evidence | Do not create duplicate work |
| Candidate gap | Baseline-only field or surface | Classify buildable, blocked, intentionally different, or retain `needs-field-audit` with exact unresolved proof | Buildable rows require a linked prioritized story; blocked rows require a blocker-register link |
| Local drift | Registered types/docs change after the audit | Repository test reports count/name/mapping drift | No network access is required in CI |

</intent-contract>

## Code Map

- `_bmad-output/planning-artifacts/provider-ecosystem-coverage-audit.json` -- canonical pinned manifest: baseline metadata, allowed classifications, all baseline rows, explicit local mappings, field/lifecycle evidence, source links, and follow-up IDs.
- `_bmad-output/planning-artifacts/provider-ecosystem-coverage-audit.md` -- human-readable generated audit and domain summaries; report current local totals of 106 resources and 90 data sources.
- `internal/coverageaudit/audit.go` and `audit_test.go` -- parse/validate the committed manifest, enumerate generated local docs, enforce uniqueness/completeness/classification/follow-up rules, and render deterministic Markdown without network access.
- `internal/service/*/exports.go`, `docs/resources/*.md`, `docs/data-sources/*.md` -- read-only local inventory evidence; `test/scripts/validate-structure.sh` remains the constructor-registration guard.
- `/tmp/browningluke-opnsense-0.26.0-schema.json` and pinned `browningluke/opnsense` v0.26.0 source/docs -- read-only investigation inputs; do not commit temporary clones or downloaded artifacts.
- `_bmad-output/implementation-artifacts/31-0-complete-provider-coverage-audit.md` -- record final classifications, generated follow-up stories, evidence, and verification.
- `_bmad-output/implementation-artifacts/sprint-status.yaml` plus new `31-*-*.md` stories -- insert each confirmed P0/P1 buildable gap ahead of 31.1–31.3 with endpoints, lifecycle, Given/When/Then acceptance, and priority.
- `_bmad-output/planning-artifacts/{support-matrix.md,core-config-gap-analysis.md,data-source-parity-plan.md,resource-gap-verification.md,provider-management-roadmap-2026-09-28.md,post-release-epics.md}` -- reconcile stale 102/88 counts, 15-gap claims, Epic 30 state, and the dependency sequence.
- `docs/upstream-blocked.md` -- add/link only confirmed stable-API blockers; preserve the canonical blocker-register format.
- `_bmad-output/implementation-artifacts/{31-1-complete-provider-positioning-docs.md,31-2-existing-provider-migration-guide.md,31-3-example-stacks-for-adoption.md` -- align downstream dependencies and remove stale assignment/count assumptions without implementing those stories.

## Tasks & Acceptance

**Execution:**
- Audit manifest/report and `internal/coverageaudit/` -- inventory the pinned baseline, semantically compare overlapping domains, encode evidence, generate the report, and add deterministic validation.
- BMad planning/support artifacts -- reconcile current facts and classify every candidate, including live-interface data, Unbound settings/forwarding, Kea peers/PD pools, WireGuard/trust settings, OpenVPN key generation, and provider retry knobs.
- Follow-up story files and sprint status -- create one resumable implementation story per confirmed buildable gap and order P0/P1 gaps before positioning/migration/adoption work.
- Blocker register and downstream stories -- link blocked rows and synchronize dependencies/current facts without expanding into later-story deliverables.

**Acceptance Criteria:**
- Given the pinned v0.26.0 schema/source, when the audit validator runs, then all 47 resources, 45 data sources, one ephemeral resource, and provider retry settings appear exactly once with an allowed classification, local mapping, field/lifecycle evidence, and source reference.
- Given overlapping firewall/NAT, Unbound, Kea, WireGuard, OpenVPN, IPsec, trust, cron, Quagga/BGP, route, VLAN, VIP, and interface domains, when the report is rendered, then material baseline-only/local-only fields and lifecycle differences are visible rather than hidden by counts.
- Given a `missing-buildable` row, when validation runs, then a prioritized implementation story with stable endpoint evidence and Given/When/Then criteria exists in sprint status before Story 31.1.
- Given a `missing-upstream-blocked` row, when validation runs, then it links to a current `docs/upstream-blocked.md` entry.
- Given the current repository, when inventory is recomputed, then 106 resources, 90 data sources, 106/90 generated docs, 17 resource-only same-name data-source gaps, and `system_info` as data-only are reflected consistently.
- Given audit guidance, when maintainers read the report, then it rejects overlapping multi-provider ownership and states the single-provider steady-state direction.

## Spec Change Log

## Review Triage Log

### 2026-10-18 — Review pass
- verdicts: 26 findings — high 1, medium 19, low 2, false 4, maybe-false 0
- findings:
  - `[medium]` `[patch]` The generated report had only snapshot coverage, so a renderer regression could omit manifest rows and bless the same omission on regeneration — added semantic tests that require every manifest surface, classification, and evidence block exactly once.
  - `[medium]` `[patch]` Manifest documentation counts were not compared with the generated-doc inventory — added resource and data-source document-count assertions against the filesystem.
  - `[medium]` `[patch]` A nonempty nonexistent follow-up path had no regression test — added an invalid-matrix case that proves the validator rejects it.
  - `[medium]` `[patch]` Buildable follow-ups were checked only for file existence, not an actionable prioritized contract — validation now requires a P0/P1 story with Given/When/Then criteria, sprint-status membership, and ordering before Story 31.1.
  - `[medium]` `[patch]` The provider roadmap omitted Story 31.0i — restored the 31.0i row exactly once while retaining 31.0j.
  - `[false]` `[reject]` The spec being `in-review` while the implementation story said `done` was a defect — `in-review` was the required transient build-auto state and is finalized to `done` only after triage and verification.
  - `[medium]` `[patch]` Unknown or empty surface kinds could pass validation and disappear from rendering — validation now rejects every kind outside the pinned resource, data-source, ephemeral-resource, and provider-config set.
  - `[medium]` `[patch]` Covered rows could name nonexistent local equivalents — validation now requires each mapped equivalent to exist in local registrations and generated docs.
  - `[low]` `[reject]` Free-form semantic field comparisons are not automatically checked against both providers' full schemas — the immutable pinned source evidence and committed manual semantic audit satisfy this story; a schema crawler would add substantial machinery beyond a direct correction.
  - `[medium]` `[patch]` Generated-document count fields were not checked against actual docs — the shared document-count fix now enforces both totals.
  - `[medium]` `[patch]` Follow-up stories were accepted without priority, acceptance, sprint, or ordering checks — the shared actionable-follow-up validator now enforces all four requirements.
  - `[medium]` `[patch]` Upstream source references could point to mutable HTTPS locations — validator and tests now require pinned commit identifiers in upstream evidence URLs.
  - `[false]` `[reject]` The audit should commit an upstream schema/source snapshot — the intent explicitly pins an immutable commit and treats clones/schema exports as noncommitted investigation inputs; the manifest and commit-addressed references are the durable evidence.
  - `[medium]` `[patch]` Blocked rows could satisfy validation with any local path — blocker validation now requires the current blocker register and a matching registered entry.
  - `[medium]` `[patch]` Classification invariants allowed missing mappings and contradictory follow-up/blocker states — added classification-specific mapping, follow-up, blocker, rationale, and exclusivity checks.
  - `[medium]` `[patch]` The generated audit promised domain summaries but rendered only classification and flat surface sections — added deterministic domain rollups and regression coverage.
  - `[false]` `[reject]` The generic lifecycle sentence claimed every domain was an operational interface lookup — the sentence already separates the general read-only lookup rule from the explicitly qualified operational-interface rows.
  - `[medium]` `[patch]` Story 31.0i lacked concrete endpoint and pinned client/source locations — added exact diagnostics/interface and interfaces/overview endpoint, client, provider, and commit evidence for all four data sources.
  - `[medium]` `[patch]` Document-count drift was not guarded — the shared count assertions now fail on resource or data-source documentation drift.
  - `[medium]` `[patch]` Covered classifications could omit a real local equivalent — the shared registration-and-doc mapping checks now reject that state.
  - `[medium]` `[patch]` Follow-up contract validation stopped at filesystem existence — the shared story-contract and sprint-order checks now enforce actionable follow-ups.
  - `[medium]` `[patch]` Blocked rows did not have to reference an actual blocker entry — the shared blocker-register validation now checks both canonical path and matching entry.
  - `[low]` `[reject]` `needs-field-audit` evidence could still be semantically weak despite rejecting empty placeholders — no current row uses that classification, and deciding proof quality mechanically would add speculative complexity without a demonstrated user-facing defect.
  - `[high]` `[patch]` The `ipsec_psk` audit incorrectly said sensitive/write-only behavior was `none` despite `pre_shared_key` — corrected the manifest and generated report to state that the secret is sensitive, write-only, and must preserve configured state.
  - `[medium]` `[patch]` The roadmap omitted Story 31.0i — the shared roadmap repair restores it exactly once.
  - `[false]` `[reject]` The one-word `merged` intent required repository code to verify merge ancestry or CI — the preceding explicit handoff resolved it as the continuation signal after PR #56, whose merge and post-merge checks were verified before Story 31.0 began.

## Design Notes

The JSON manifest is the maintained source of truth; Markdown is deterministic output. Classification is restricted to `covered`, `covered-different-name`, `covered-broader`, `missing-buildable`, `missing-upstream-blocked`, `intentionally-different`, and `needs-field-audit`. The last value requires a concrete missing proof and must not satisfy completion for a P0/P1 candidate.

## Verification

**Commands:**
- `go test ./internal/coverageaudit` -- expected: manifest completeness, mapping, follow-up, local inventory, and deterministic rendering tests pass.
- `go generate ./... && git diff --exit-code` -- expected: generated audit/docs are current.
- `bash test/scripts/validate-structure.sh` -- expected: all constructors remain registered.
- `DEVRAIL_IMAGE=ghcr.io/devrail-dev/dev-toolchain:1.12.24 make check` -- expected: lint, format, tests, security, scan, and docs pass.
- `git diff --check` -- expected: no whitespace errors.

## Auto Run Result

- **Summary:** Completed the pinned provider-ecosystem coverage audit, generated its human report, reconciled planning/support artifacts, and created prioritized Stories 31.0a–31.0j for every confirmed buildable gap.
- **Files changed:**
  - `_bmad-output/planning-artifacts/provider-ecosystem-coverage-audit.{json,md}` — canonical 94-surface semantic matrix and deterministic report with classification/domain summaries.
  - `internal/coverageaudit/` — offline manifest validator, renderer, generator, and regression tests for pinned inventory, mappings, evidence, follow-ups, blockers, docs, and render completeness.
  - `_bmad-output/implementation-artifacts/31-0[a-j]-*.md` and `sprint-status.yaml` — actionable P0/P1 gap stories ordered before positioning, migration, and adoption work.
  - `_bmad-output/planning-artifacts/*.md`, downstream 31.1–31.3 stories, and `docs/upstream-blocked.md` — reconciled counts, dependencies, Epic 30 state, coverage conclusions, and blocker language.
  - `.gitleaksignore` — exact historical scanner fingerprint for the nonsecret API model field in the merged Story 30.2 test fixture.
- **Review findings:** 26 findings were logged individually above. Twenty were patched (1 high, 19 medium findings) and none were deferred. Six were rejected: the transient `in-review` state, committing an upstream snapshot, the already-qualified lifecycle wording, and merge-ancestry interpretation were false; automatic full-schema crawling and speculative semantic grading of unused `needs-field-audit` evidence were low-value complexity beyond direct corrections.
- **Follow-up review recommendation:** `true`. Patched entry counts were 1 high and 11 medium; the remaining unverified risk is whether the broadened validator rules interact correctly with future blocked and `needs-field-audit` rows not present in this manifest.
- **Verification:** `go test ./internal/coverageaudit` passed; `go generate ./... && git diff --exit-code` passed; `bash test/scripts/validate-structure.sh` passed; the complete DevRail `make check` passed lint, format, tests, security, scan, and docs after the reviewed patch; `git diff --check` passed.
- **Residual risks:** The baseline is intentionally frozen at `browningluke/opnsense` v0.26.0/`33c2a03c2bc1a7339cb5aa5b611e15014e27f7ba`; future upstream releases require an explicit baseline refresh rather than silent drift.
