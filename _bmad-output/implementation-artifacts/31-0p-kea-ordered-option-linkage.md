# Story 31.0p: Kea Ordered Generic-Option Linkage

## Status

backlog — concrete gap discovered while implementing31.0n. Generic option CRUD
is separate from attaching those options to existing subnet/reservation owners.

## Scope and API research gate

Tagged core26.7.7 has a multi-valued `option` relation on subnet and reservation
models. Stored CSV order drives order-sensitive flex-option generation: the last
matching same-code action wins, and reservation actions follow subnet actions.
Current provider parent owners expose neither relation. GET choices are
label-sorted rather than a reliable ordered representation. Do not model this as
an unordered set or infer order from JSON map keys.

1. Determine whether a stable supported read API can expose authoritative stored
   order. If unavailable, document the exact missing capability and an upstream
   owned proposal; never use raw XML reads/writes as provider behavior.
2. Once an order-preserving API contract is proven, extend the existing
   `opnsense_kea_dhcpv4_subnet` and reservation owners/lookup counterparts with
   typed ordered option UUID references, preserving omitted fields and existing
   option_data/PXE ownership. Do not create a competing parent owner.
3. Prove order/reorder/import/no-change/drift/clear and dependency deletion through
   real Core plus independent installed-model observations on disabled disposable
   services. Protect already configured links across unknowns/restarts.
4. Update release notes, docs, support/audit and complete full/review/hosted gates.

## Acceptance Criteria

- Given two same-code options, when linked/reordered in supported HCL, then the
  actual authoritative order and generated precedence match with stable parent
  UUIDs and exact no-change import.
- Given unavailable ordered reads, when planning support, then ownership remains
  explicitly research-gated rather than falsely supported through sorted maps.
- Given omission, unknown intent or failed write/activation, when recovering,
  then existing relations and unrelated parent settings are not lost.

Active packet delivery and private adoption remain separate validation gates.
