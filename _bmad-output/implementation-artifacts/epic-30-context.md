# Epic 30 Context: OPNsense Assignment API Adoption

<!-- Generated from planning artifacts. Regenerate with compile-epic-context if planning docs change. -->

## Goal

Safely turn OPNsense's expanded `interfaces/assignment` API into provider support for base interface assignment and IP configuration. The feature must target a released, complete API rather than unreleased upstream code or a duplicate `address_settings` design, and it must be proven on a disposable appliance before users are told the domain is supported.

## Stories

- Story 30.1: Track OPNsense Assignment API Release
- Story 30.2: Implement Interface Assignment/IP Resource for OPNsense 26.7.6+
- Story 30.3: Update Interface Support Matrix and Public Blocker Docs

## Requirements & Constraints

- Target OPNsense `26.7.6` or newer. It is the first released tag with the complete required assignment surface; `26.7.5` and `27.1.a` are incomplete and must not be treated as compatible solely because some controller actions exist.
- Cover the released assignment/IP surface: interface enablement, IPv4 and IPv6 address modes and values, IPv4 and IPv6 gateways, and the available DHCPv4/DHCPv6 settings. Hardware flags exposed by the expanded model may be included where their semantics are safe and verified.
- Keep PPPoE outside the initial implementation. Its behavior remains unverified and requires separate semantic proof.
- Do not claim public support when code merely exists. Support requires disposable-appliance acceptance evidence for the intended lifecycle, drift/read-back behavior, and a second plan that reports no changes.
- Preserve Terraform safety expectations: operations must be idempotent, out-of-band changes must appear as drift, partial failures must leave recoverable state, and reconfigure failures must clearly warn that persisted changes may not yet be active.
- Continue maintaining machine-readable release evidence as upstream branches and tags change. Public support and blocker documentation must match the verified release gate and current acceptance status.
- Do not implement fallbacks that scrape legacy PHP pages, edit `config.xml` directly, or otherwise bypass OPNsense API ownership.

## Technical Decisions

- Use the existing assignment API, specifically `api/interfaces/assignment/set_item/{ifname}` followed by `api/interfaces/assignment/reconfigure`. The abandoned separate `address_settings` endpoint is not the implementation direction.
- Treat the interface name as the API item identity. Account for OPNsense staging assignment changes through `/tmp/.interfaces.todo`, with assignment reconfigure finalizing them.
- Add version and capability guardrails so incomplete appliances fail with a clear compatibility diagnostic instead of silently accepting a partial field set. A controller exposing `set_item` and `reconfigure` is insufficient unless the required address, gateway, and DHCP fields are also present.
- Serialize assignment mutations with the provider's global mutation lock. OPNsense stores configuration in a shared `config.xml`, so parallel writes can lose changes or corrupt configuration.
- After each successful mutation, call the assignment-specific reconfigure operation and then read API state back into Terraform. Never populate state by echoing the request.
- Parse mutation response bodies even on HTTP 200 and surface field validation failures as Terraform diagnostics. OPNsense may return validation failure without an error HTTP status.
- Ensure request payloads use the assignment API's expected wrapper/model shape and conversions for OPNsense representations such as string booleans, selected values, and integer-as-string fields.

## Cross-Story Dependencies

- Story 30.1 establishes and maintains the minimum-version and capability evidence consumed by Story 30.2; implementation must be revisited if the release probe's required surface changes.
- Story 30.2 supplies the lifecycle and no-change-plan evidence required before Story 30.3 can move interface assignment/IP settings from pending to Supported.
- Story 30.3 must distinguish released API availability from provider support until Story 30.2's acceptance gate passes, and must keep PPPoE listed as separate research.
