# Epic 31 Context: Provider Completeness, Positioning, and Migration

<!-- Generated from planning artifacts. Regenerate with compile-epic-context if planning docs change. -->

## Goal

Make the provider complete enough to be the default choice for managing an OPNsense appliance, then turn that demonstrated completeness into user trust and adoption. The epic must establish an evidence-backed coverage position before publishing claims, migration guidance, or examples so operators can confidently standardize on one Terraform provider instead of maintaining split configuration across multiple providers, Ansible, or the OPNsense UI.

## Stories

- Story 31.0: Complete Provider Coverage Audit
- Story 31.1: Complete Provider Positioning README and Registry Pass
- Story 31.2: Existing Provider Migration Guide
- Story 31.3: Example Stacks for Adoption

## Requirements & Constraints

- Audit coverage at field level, not only by counting resource types. Compare overlapping concepts with the established provider ecosystem and distinguish genuine product gaps from documentation or positioning gaps.
- Split any confirmed, meaningful provider-owned coverage gaps into implementation stories instead of hiding them inside documentation work. Keep upstream-blocked domains identified honestly and do not fabricate resources through legacy page scraping, direct configuration-file edits, or other API-bypassing workarounds.
- Public claims must be supported by the current support matrix. Prefer a factual position centered on broad, modern appliance management across core networking and edge plugin/service areas, brownfield import, and release-gated API support. Avoid adversarial provider comparisons or unsupported claims of total completeness.
- Registry-facing documentation must explain provider configuration, authentication, supported OPNsense versions, required permissions, support status, and a complete quickstart. It must keep Supported, Coming, and Upstream-blocked areas explicit.
- Migration guidance must support existing appliances and users of other providers. Every manageable resource must remain importable without manual state editing, and the documented flow should lead from import to an accurate no-change plan.
- Migration ordering must account for cross-resource dependencies so referenced objects are imported before dependents. Guidance must preserve Terraform's core guarantees: accurate plan/apply, drift detection, and stable state read-back.
- Adoption examples should be realistic multi-resource compositions rather than isolated snippets. They should demonstrate complete operational workflows such as HAProxy with ACME, BGP peering, firewall foundations, VPN setup, DNS management, and customer onboarding.
- Generated Registry documentation remains part of the shipped product. Examples and import workflows must accompany documentation, and documentation validation must pass before merge.
- Migration content is secondary to verified completeness: coverage evidence and resulting implementation gaps must be resolved or clearly classified before positioning and migration claims are finalized.

## Technical Decisions

- Use the Terraform Plugin Framework and Protocol v6 contract; do not introduce legacy SDK patterns as part of migration compatibility work.
- Preserve native Terraform import, refresh, plan, and drift semantics. State must reflect API read-back rather than configuration assumptions, and imported resources must converge to a no-change plan when configuration matches the appliance.
- Keep public documentation generated through the established tfplugindocs pipeline, with narrative templates, resource examples, and import examples as its inputs. Composition examples belong in dedicated composition directories and should reuse normal provider resources rather than external scripts.
- Treat release-gated API support and acceptance evidence as part of the provider's product surface. Coverage claims must distinguish implemented and proven behavior from planned or upstream-only capability.
- Retain an honest single-provider objective: broad core and plugin/service coverage is the differentiator, but unsupported domains remain visible until stable APIs and lifecycle proof exist.

## Cross-Story Dependencies

- Story 31.0 runs first and may create higher-priority implementation stories. Confirmed product gaps must be handled before the later stories present the provider as complete for those areas.
- Story 31.1 uses the audit and support matrix to align README and Registry positioning with verified coverage.
- Story 31.2 follows the coverage audit so migration instructions do not direct users toward incomplete or unsupported replacement paths.
- Story 31.3 builds on the verified resource surface and migration conventions; its compositions also serve as target-state references for brownfield adoption.
- Epic 30's release-gated interface assignment work must be complete or accurately classified before Epic 31 makes coverage claims. Epic 32's acceptance-proof and v1-readiness work strengthens later release credibility but does not replace this epic's coverage audit.
