# Story 31.3: Example Stacks for Adoption

## Status

backlog

## Story

As a new user,
I want complete example stacks for common OPNsense jobs,
so that I can start from proven compositions rather than isolated resource snippets.

## Acceptance Criteria

1. Example stack exists for home firewall baseline.
2. Example stack exists for HAProxy plus ACME edge routing.
3. Example stack exists for FRR/BGP edge routing.
4. Example stack exists for brownfield import workflow.
5. Example stack exists for PXE/VLAN/DHCP foundation, with caveats where DHCP options remain release-gated or base interface assignment remains provider-implementation pending.
6. All examples pass formatting and provider schema validation where feasible.
7. README and Registry docs link to the examples.

## Dev Notes

- Prefer runnable compositions over toy snippets.
- Keep secrets and appliance-specific values as variables.

## Validation

- `terraform fmt -check` on examples.
- `make docs` if generated docs link to new examples.
- `make check` before completion.
