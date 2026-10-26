# Epic 31 Context: Provider Completeness, Positioning, and Migration

<!-- Generated from planning artifacts. Regenerate with compile-epic-context if planning docs change. -->

## Goal

Make the provider complete enough to be the default choice for an OPNsense appliance, then turn verified completeness into user trust and adoption. Close meaningful field, lifecycle, configuration and read-only surface gaps before refreshing public positioning, migration guidance and composition examples; resource counts alone do not establish completeness.

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

- Assess semantic coverage against the pinned established-provider baseline, including schema depth, naming differences, resource/data-source/ephemeral surfaces, sensitive fields and provider configuration. Classify real gaps separately from equivalent differently named features and intentional differences. Re-run the deterministic audit after schema or registration changes.
- Public claims must track the support matrix: distinguish supported, coming, needs-research and upstream-blocked work. Stable API availability, provider implementation, published release and live Terraform lifecycle proof are separate gates. Do not treat upstream development branches or direct-API probes as shipped Terraform support.
- Use stable, durable OPNsense APIs only; do not scrape legacy pages, edit configuration XML directly or bypass API ownership. Document feature-specific core/plugin version requirements. Assignment/IP configuration requires OPNsense 26.7.6+; PPPoE semantics remain separately unverified. FRR 2.0 compatibility requires explicit model/version and migration validation, not a blanket compatibility claim.
- Prove create/read/update/delete, import, no-change plans, drift detection/repair and cleanup on disposable appliances. Settings mutations must preserve unrelated configuration and restore the baseline. Keep state recoverable when CRUD succeeds but activation fails. Read-only views must not expose write-only secrets or imply ownership of operational state.
- Aim for one provider per appliance/object domain in steady state; never let two providers manage the same object. Migration must establish supported coverage and dependency-ordered import before ownership handoff. Examples use synthetic infrastructure, reserved example domains and documentation address ranges; private inventories, usage plans and production cutovers are outside scope.
- Ship resource documentation and import examples with functionality. README and Registry guidance must accurately cover authentication, configuration precedence, permissions, version requirements, support limitations and a working quickstart. Prefer factual broad-coverage positioning over adversarial comparisons or unsupported complete-appliance claims.

## Technical Decisions

- Preserve the separation between Terraform lifecycle semantics and API transport/model conversion. Shared API infrastructure owns serialized writes, bounded parallel reads, pagination, retries and response validation; feature work must not bypass those safety guarantees.
- Treat mutation and service activation as distinct operations with domain-specific activation behavior. Honor firewall rollback safety and recognize that service reload, firewall activation and daemon restart are not interchangeable evidence.
- Populate readable state from authoritative API read-back rather than request configuration. Normalize selected-map enums, multi-selects and string booleans without losing values. Distinguish omitted fields from explicit clears and surface body-level validation failures even on HTTP success.
- Keep existing configurations/state upgradeable. Preserve write-only values when the API cannot return them, mark secrets sensitive and never log secret-bearing bodies. Ephemeral and write-only functionality needs explicit Terraform feature-version and non-persistence proof; do not infer equivalent OpenTofu support.
- Keep retry controls in shared provider configuration with configurable retry count/backoff. Protect state during transient failures and appliance unavailability. Changes must retain idempotence, useful field/permission diagnostics and safe missing-object refresh behavior.

## UX & Interaction Patterns

The user interface is HCL, Terraform plans/diagnostics and documentation. Brownfield adoption follows matching configuration, dependency-ordered import, refresh and a no-change plan before a deliberate apply gate. Show references between resources in realistic firewall, HAProxy/ACME, routing, VPN and DNS compositions. Explain naming/field differences and unsupported surfaces before users attempt an ownership transfer; recommend protected remote state for sensitive values.

## Cross-Story Dependencies

- The coverage audit drives parity work; finish and reconcile meaningful P0/P1 gaps before positioning, migration and adoption examples. Preserve completed subnet/PXE/DHCPv6 coverage rather than implementing duplicate Kea owners.
- Gateway-group and generic Kea option/DDNS validation establish API contracts; the subsequent owner stories still require real Terraform acceptance. Direct-API validation does not close provider import, no-change, drift, recovery or settings-destroy obligations.
- README/Registry positioning precedes the existing-provider migration guide; example stacks must compose only verified, appropriately version-gated surfaces. Completed assignment API adoption supplies interface coverage, while broader acceptance-proof publication and v1 criteria remain separate Epic 32 work.
