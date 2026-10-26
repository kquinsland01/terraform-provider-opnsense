# Epic 31 Context: Provider Completeness, Positioning, and Migration

<!-- Generated from planning artifacts. Regenerate with compile-epic-context if planning docs change. -->

## Goal

Make the provider complete enough to be the default choice for an OPNsense appliance, then translate verified coverage into trust and adoption. Close meaningful field, lifecycle, and lookup gaps before promoting broad appliance management; support dependency-aware migration from existing providers without competing ownership or unexpected changes.

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
- Story 31.0m: Gateway Group Resource/Data Source
- Story 31.0n: Kea Generic Option Resource/Data Source
- Story 31.0o: Kea DDNS Settings Resource/Data Source
- Story 31.0p: Kea Ordered Generic-Option Linkage
- Story 31.1: Complete Provider Positioning README and Registry Pass
- Story 31.2: Existing Provider Migration Guide
- Story 31.3: Example Stacks for Adoption

## Requirements & Constraints

- Establish completeness through semantic field and lifecycle coverage, not resource counts or matching names. Compare against a pinned ecosystem baseline; distinguish covered, renamed/broader, buildable, upstream-blocked, and intentionally different surfaces. Refresh the deterministic audit after schema or registration changes.
- Keep supported, unreleased, research-only, and upstream-blocked capabilities distinct. Stable API availability, provider implementation, published release, and live lifecycle proof are separate gates. README, Registry documentation, and migration claims must match the verified support matrix rather than older roadmap snapshots.
- Preserve Terraform semantics: accurate refresh and drift detection, in-place updates where valid, API read-back after writes, consistent state after partial failures, and clear reconfigure diagnostics. Verify import and a no-change plan, not merely successful API calls or mocked CRUD.
- Maintain one owner per appliance/object domain in steady state; never let two providers manage the same object. Migration must identify gaps and ownership boundaries before handoff. Synthetic examples require an explicit manual apply gate; provider work does not authorize production firmware, routing, DHCP, or ownership changes.
- Publish consumer-neutral examples using documentation addresses and reserved domains. Exclude operator inventory, allocations, hostnames, operational plans, credentials, and local workspace paths. Record only sanitized test versions, behavior, and results.

## Technical Decisions

- Consume durable OPNsense APIs with explicit core/plugin compatibility gates. Do not scrape legacy pages, edit configuration XML directly, or invent private endpoints to bypass upstream ownership. Interface assignment/IP support requires OPNsense 26.7.6+; PPPoE remains separately unverified. Gateway-group and generic-option/DDNS contracts have stable 26.7.7 evidence, not proof for older targets.
- Respect each ownership model: UUID-backed items, read-only lookups, settings singletons, and sensitive/ephemeral operations are not interchangeable. Limit singleton writes to the owned subtree; a filtered GET does not establish ownership of surrounding configuration. Kea DDNS owns only its general settings subtree and uses state-only destroy.
- Preserve omission versus explicit clearing, selected-key semantics, and ordering where behavior depends on it. Generic options are independent objects, not proof of parent linkage. Label-sorted relation choices do not establish stored order; ordered linkage remains gated on an adequate read contract. Stored force flags do not establish packet-level behavior.
- Honor observed API limitations without silently reporting success: gateway-group populated first-tier clearing is upstream-gated and must be rejected before other fields are saved. In-use deletion failures retain identity/state and must not be treated as successful deletion or not-found.
- Follow shared lifecycle safety: serialize mutations, reconfigure only the affected service, and retain firewall rollback protection. Evidence should exercise real Terraform lifecycle, import/no-change, drift, failure recovery, deletion or state-only removal as appropriate, and exact disposable-appliance restoration. Direct-API validation alone proves none of those Terraform behaviors.

## UX & Interaction Patterns

- Lead README and Registry pages with broad modern core/plugin coverage backed by explicit support status; avoid unsupported completeness claims or adversarial provider comparisons. Explain authentication, permissions, compatibility requirements, and a working quickstart.
- Present migration as matching configuration, importing identities in dependency order, and inspecting a no-change plan before ownership handoff. Explain semantic name/field differences and unsupported cases; do not imply provider state is automatically interchangeable.
- Supply realistic multi-resource compositions and complete import examples alongside generated argument/attribute references. Show how resources connect so examples also serve as migration targets.

## Cross-Story Dependencies

- The coverage audit drives parity follow-ups; reconcile completed work rather than rebuilding it. Finish remaining parity and compatibility gates before positioning, migration guidance, and adoption examples. FRR 2.0 compatibility is a separate prerequisite to BGP adoption; compatibility alone does not complete advanced parity.
- Gateway-group validation precedes its owner/lookup. Kea option/DDNS validation precedes their separate owners; ordered parent linkage is an additional gate, not completion conferred by standalone generic-option ownership. Existing subnet/PXE/DHCPv6 work is not repeated.
- Retain Epic 30's assignment API release gate. Epic 32's repeatable acceptance proof and explicit stability criteria remain prerequisites for credible v1 positioning.

## Story 31.0o implementation boundary

The compatible stable 26.7.7+ DDNS general singleton now has a four-field owner
and read-only lookup in the working candidate. It preserves omitted optional
values, writes only explicit configured fields and persists fixed-ID requested-
subset save recovery. Delete itself is zero-API/state-only, even with pending
recovery; normal Core planning refresh may read/recover/activate first or fail
before Delete. Delete does not roll back saved settings or activate them. Actual Core failed-create
taint remains normal. Full/race verification and bounded review rechecks passed. The final digest-bound
disabled-appliance evidence gates publication; merge/release remain separate.
Active DNS delivery remains unverified.
