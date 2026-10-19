# Epic 31 Context: Provider Completeness, Positioning, and Migration

<!-- Generated from planning artifacts. Regenerate with compile-epic-context if planning docs change. -->

## Goal

Make the provider complete enough to be the default choice for managing an OPNsense appliance, then turn demonstrated completeness into user trust and adoption. Coverage claims, migration guidance, and examples must follow verified field- and surface-level parity so operators can standardize on one provider instead of splitting ownership across providers, Ansible, or the OPNsense UI.

## Stories

- Story 31.0: Complete Provider Coverage Audit
- Story 31.0a: Firewall and NAT Field Parity
- Story 31.0b: Kea Field and Surface Parity
- Story 31.0c: OpenVPN Field and Key Parity
- Story 31.0d: Unbound Settings and Forward Parity
- Story 31.0e: WireGuard Field and Settings Parity
- Story 31.0f: Trust Lifecycle and Settings Parity
- Story 31.0g: IPsec Field Parity
- Story 31.0h: Quagga/BGP Field Parity
- Story 31.0i: Live Interface Data Sources
- Story 31.0j: Provider Retry Settings
- Story 31.1: Complete Provider Positioning README and Registry Pass
- Story 31.2: Existing Provider Migration Guide
- Story 31.3: Example Stacks for Adoption

## Requirements & Constraints

- Use the pinned, deterministic ecosystem audit to close field and surface gaps across firewall/NAT, Kea, OpenVPN, Unbound, WireGuard, trust, IPsec, Quagga/BGP, live interface state, and configurable retry behavior. Measure parity by usable fields and lifecycle semantics, not resource counts alone.
- Treat the support matrix as the release-positioning source of truth. Clearly classify supported, planned, research-needed, and upstream-blocked capabilities; do not conceal gaps in documentation or claim completeness beyond the evidence.
- Use one provider per appliance and object domain in steady state. Migration guidance must prohibit two providers from managing the same OPNsense object and must explain dependency-aware import of existing objects without manual state editing, ending in an accurate no-change plan.
- Preserve the common resource contract across parity work: create, API read-back, in-place update where valid, delete, UUID import, drift detection, missing-object removal from state, correct service reconfigure, and recoverable partial failure. Firewall changes must retain savepoint/apply/rollback safety, and all mutations must remain serialized to protect OPNsense configuration integrity.
- Retry settings must keep transient network, timeout, reboot, and server failures retryable while allowing operators to configure retry count and backoff. Exhausted retries must produce actionable diagnostics without corrupting state.
- Sensitive keys and credentials must never appear in plan or logs and should not be retained in state when write-only handling is available. When an API cannot return a secret, preserve configured state rather than creating false drift.
- Registry-facing documentation must cover configuration, authentication, minimum OPNsense versions, required permissions, the current support matrix, and a complete quickstart. Every resource continues to ship generated argument/attribute documentation, examples, and import instructions.
- Public positioning should emphasize broad, modern core and plugin/service management, brownfield import, and release-gated API support. Avoid adversarial provider comparisons and unsupported claims of total appliance coverage.
- Adoption examples must be realistic, runnable multi-resource compositions that demonstrate dependency links and normal plan/apply behavior. They must not require manual UI steps, external scripts, legacy-page scraping, or direct configuration-file edits.
- All changes must retain Terraform CLI 1.0+ protocol compatibility and core OpenTofu CRUD/import/plan/apply compatibility, pass the full quality gate, and keep tests deterministic. Resource acceptance coverage remains a product requirement rather than documentation-only evidence.

## Technical Decisions

- Stable OPNsense APIs own configuration. Version-gate newly released API surfaces; never treat an unreleased branch as supported or bypass the API with XML edits or legacy UI scraping.
- The shared API client owns authentication, retry, pagination, OPNsense error parsing, and the global mutation lock. Service controllers define typed requests and endpoint metadata; resources and data sources preserve the established module boundaries.
- Search operations must traverse all OPNsense result pages. Validation failures may arrive with HTTP 200 and must be recognized from response content; absent UUID records must be distinguished from blank default responses.
- Plugin-backed capabilities require the corresponding plugin to be installed. Missing plugins, permissions, credentials, and connectivity must yield specific, actionable diagnostics.
- Documentation remains generated through the established documentation pipeline. Narrative templates, per-resource examples, import examples, and dedicated composition examples are the maintained inputs.
- The deterministic ecosystem audit is the parity feedback loop. Registration or schema changes must update its machine-readable and human-readable outputs so positioning is based on current results.

## UX & Interaction Patterns

- Evaluation starts with a concise support view that lets operators confirm their appliance version, required plugins, permissions, and needed domains before adoption.
- Brownfield onboarding proceeds from provider authentication to dependency-ordered imports, configuration capture, and a no-change plan; subsequent plans must expose UI/API drift clearly before apply.
- Examples should model the review experience users adopt in CI: readable resource relationships, predictable diffs, explicit unsupported boundaries, and no hidden post-apply steps.

## Cross-Story Dependencies

- Story 31.0 established the pinned baseline and generated Stories 31.0a–31.0j. Stories 31.0a–31.0d have reached done or review; complete the remaining P0 work in Stories 31.0e–31.0f before P1 Stories 31.0g–31.0j, rerunning the deterministic audit after registration or schema changes.
- Story 31.1 follows the parity work so README and Registry claims match verified coverage. Story 31.2 then publishes safe migration paths, and Story 31.3 builds adoption stacks on that verified surface.
- Epic 30 supplies release-gated interface assignment/IP support for OPNsense 26.7.6 or newer; PPPoE remains separately unverified and must not be implied by Epic 31 positioning.
- Epic 32 supplies broader acceptance-proof and v1-readiness evidence. Epic 31 improves completeness and adoption but does not by itself establish v1 readiness.
