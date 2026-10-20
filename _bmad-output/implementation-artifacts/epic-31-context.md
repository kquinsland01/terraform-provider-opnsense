# Epic 31 Context: Provider Completeness, Positioning, and Migration

<!-- Generated from planning artifacts. Regenerate with compile-epic-context if planning docs change. -->

## Goal

Make the provider complete enough to be the default choice for an OPNsense appliance, then convert demonstrated completeness into user trust and adoption. Close meaningful field, surface, and lifecycle gaps before publishing positioning, migration guidance, and adoption stacks that encourage operators to standardize on one provider.

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
- Story 31.0k: Gateway Group Stable API Validation
- Story 31.0l: Kea Option and DDNS Stable API Validation
- Story 31.1: Complete Provider Positioning README and Registry Pass
- Story 31.2: Existing Provider Migration Guide
- Story 31.3: Example Stacks for Adoption

## Requirements & Constraints

- Measure completeness through the pinned deterministic ecosystem audit: usable fields, safe lookup surfaces, import, and lifecycle behavior matter more than resource counts. Reconcile audit outputs after registration or schema changes.
- Keep stable API availability, provider implementation, published provider release, and live lifecycle proof as separate gates. A stable controller or merged implementation alone does not establish released support or production readiness. Support claims must match the support matrix and distinguish supported, coming, research-needed, and upstream-blocked capabilities.
- Prioritize plugin-aware FRR 2.0 compatibility before BGP adoption. Core firmware version alone cannot establish plugin compatibility. Family/state/policy migration and automatic protocol firewall-rule behavior require explicit verification; do not imply blanket FRR 2.0 support from existing BGP resources.
- Stable 26.7.7 gateway-group APIs clear an upstream-source blocker, not provider implementation or lifecycle validation. Generic Kea option/DDNS payload, mutation, reconfigure, and lifecycle semantics remain a separate validation gate; completed subnet PXE/TFTP, peer, and delegated-prefix work is not proof of those surfaces or Foreman integration.
- Preserve CRUD, authoritative API read-back, UUID import where applicable, drift detection, missing-object handling, service-specific reconfigure, and recoverable partial failure. CRUD success with activation failure must retain recoverable state and clearly report pending activation. Firewall mutations retain rollback protection.
- Serialize mutations to protect shared appliance configuration; bound read concurrency. Retry transient network, server, timeout, and reboot failures with configurable retry/backoff and actionable exhaustion diagnostics without corrupting state.
- Protect credentials and keys in output and logs; use write-only attributes where available and preserve non-readable secret state without false drift. Keep TLS verification enabled unless explicitly disabled.
- Use stable, durable OPNsense APIs only: no legacy-page scraping, direct XML edits, or provider-managed firmware/plugin upgrades. These stories authorize no production firmware, routing, or ownership cutover.
- Require deterministic acceptance coverage against real disposable OPNsense targets plus the full quality gate. Preserve core Terraform/OpenTofu compatibility, and document higher minimum versions for optional newer protocol features rather than implying universal feature support.
- Generate resource documentation with arguments, attributes, imports, and realistic composition examples. README/Registry guidance must identify authentication, required permissions/plugins, minimum versions, and unsupported boundaries. Position broad modern coverage without adversarial comparisons or unsupported completeness claims.

## Technical Decisions

- Maintain API ownership and the two-phase configuration/activation lifecycle. Interpret validation bodies even when HTTP status is successful, distinguish absent objects from blank defaults, and traverse all search-result pages.
- Distinguish plugin availability/version from core appliance version. Missing plugins, insufficient permissions, authentication failures, endpoint absence, and connection failures must yield specific diagnostics.
- Keep parity evidence reproducible and tied to the implementation under test. Documentation and generated audit claims must reflect verified behavior rather than simply copying another provider's schema.
- Preserve ordered policy semantics and safe state/schema migration across upstream model changes; successful serialization alone is insufficient evidence of equivalent behavior.

## UX & Interaction Patterns

- Operators should be able to determine support, version/plugin prerequisites, and required permissions before adoption.
- Brownfield onboarding follows authentication, dependency-aware import, configuration capture, and a no-change plan. Subsequent plans expose UI/API drift clearly; avoid competing owners for the same appliance object.
- Adoption stacks demonstrate readable resource relationships, predictable plan/apply behavior, and explicit unsupported boundaries without hidden configuration steps.

## Cross-Story Dependencies

- The coverage audit drives parity work and must remain current. Finish the WireGuard prerequisite before prioritizing the FRR 2.0 compatibility subgate; complete remaining parity and gateway/Kea validation before positioning. A merged prerequisite must still be distinguished from a published release.
- Stories 31.0k and 31.0l validate newly available stable surfaces without recreating completed Kea parity. Do not reopen completed firewall, OpenVPN, Unbound, or interface work absent a regression.
- Stories 31.1–31.3 depend on verified coverage and safe ownership/import semantics. Epic 30 supplies release-gated base assignment/IP support for OPNsense 26.7.6+ with disposable 26.7.7 proof; PPPoE remains separately unverified.
- Epic 32 supplies broader acceptance-proof and v1-readiness work. Epic 31 does not independently establish v1 readiness or authorize appliance adoption/cutover.
