# Epic 31 Context: Provider Completeness, Positioning, and Migration

<!-- Generated from planning artifacts. Regenerate with compile-epic-context if planning docs change. -->

## Goal

Close meaningful coverage gaps so operators can choose one OPNsense provider for an appliance, then translate verified completeness into clear positioning, dependable brownfield migration guidance, and useful adoption examples. Coverage must mean field depth and correct lifecycle behavior, not merely a larger resource count.

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
- Story 31.1: Complete Provider Positioning README and Registry Pass
- Story 31.2: Existing Provider Migration Guide
- Story 31.3: Example Stacks for Adoption

## Requirements & Constraints

- Compare equivalent provider surfaces by semantics, fields, types, defaults, lifecycle, lookup behavior, and secret handling. Distinguish renamed coverage, broader coverage, deliberate differences, buildable gaps, and upstream limitations. Refresh the deterministic audit after registration or schema changes; do not infer parity from counts.
- Public support claims must match verified support and release availability. Keep stable API presence, provider implementation, published release, and live Terraform lifecycle proof distinct. Development-branch models alone do not establish supported APIs. Interface assignment/IP support requires OPNsense 26.7.6 or later.
- Use official OPNsense APIs only; no legacy-page scraping, direct XML edits, or ownership bypasses. A validation report establishing a buildable contract does not itself ship a resource or prove Terraform behavior. Gateway-group and Kea option/DDNS probes establish contracts on 26.7.7; traffic failover and external DHCP/DDNS integrations remain unproved.
- Prove create/read/update/delete, import, no-change plans, drift, failure consistency, and cleanup on disposable appliances as applicable. Read-only lookups must not mutate configuration. Preserve existing user state and support schema evolution without manual state editing.
- Use one provider owner per object domain in steady state. Migration must account for uncovered fields and explicit blockers before ownership changes; two providers must never manage the same object. Provider development does not authorize production firmware, routing, or ownership cutovers.
- Keep all public artifacts consumer-neutral. Use synthetic disposable compositions, reserved example domains, and documentation address ranges; exclude private inventory, credentials, real topology, and operator deployment plans.

## Technical Decisions

- Preserve accurate plan/apply semantics: read authoritative API state after mutations, update mutable network resources in place, and retain consistent state after partial failures. Serialize mutations globally while permitting bounded concurrent reads; reconfigure only the affected service. Firewall filter changes require savepoint/apply/rollback-confirmation protection.
- Handle OPNsense response semantics explicitly: successful HTTP status may contain validation failures, and missing UUIDs may return blank defaults. Surface field-specific diagnostics; distinguish absent objects from authentication, service, and dependency failures.
- Normalize API selected maps to selected keys, preserving multi-value sets rather than labels or whole response maps. Respect request wrappers, omitted-field preservation, explicit clears, and API-assigned metadata. Ownership must be limited to the actual managed object or settings subtree.
- Treat API write-only secrets as sensitive and preserve prior values when refresh omits them; never invent readable secret parity. Use ephemeral/write-only flows where supported, and do not expose resource-only secrets through data sources. Document unavoidable secret drift-detection limits.
- Singleton settings need durable get/set semantics and non-destructive ownership boundaries, not fabricated CRUD. Kea DDNS owns only its general settings subtree and has state-only destroy; generic options are distinct from subnet option data. Operational interface lookups describe live state rather than durable configuration ownership.
- Expose configurable retries and backoff through the shared client while retaining transient-failure recovery and actionable exhaustion errors.

## UX & Interaction Patterns

The provider's interface is HCL, Terraform diagnostics, README, and Registry documentation. Present broad modern appliance management and brownfield import without unsupported completeness claims or adversarial comparisons. Explain supported versions, plugin/API permissions, authentication precedence, known gaps, and a working quickstart. Migration guidance follows dependency-aware import, matching configuration, and a no-change plan before an explicit apply gate; composition examples demonstrate connected resources and serve as migration target configurations.

## Cross-Story Dependencies

The coverage audit drives parity work; finish verified gaps before positioning and migration claims. Prioritize the FRR compatibility gate before BGP adoption. Gateway-group validation precedes its resource/data source, and Kea option/DDNS validation precedes their separate owners; completed subnet/PXE coverage must not be rebuilt as a generic-option substitute. README/Registry positioning precedes the migration guide, with examples supporting adoption. Keep Epic 30's assignment release evidence current; Epic 32's acceptance-proof work is a prerequisite for credible v1 positioning, not a consequence of broader coverage alone.
