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
5. Example stack exists for PXE/VLAN/DHCP foundation, using supported `opnsense_interface_assignment` on OPNsense 26.7.6+ and retaining release-gated caveats for DHCP options and PPPoE.
6. All examples pass formatting and provider schema validation where feasible.
7. README and Registry docs link to the examples.

## Dev Notes

- Prefer runnable compositions over toy snippets.
- Keep secrets and appliance-specific values as variables.
- Depends on completion or explicit audit reclassification of P0/P1 Stories 31.0a–31.0j.

## Validation

- `terraform fmt -check` on examples.
- `make docs` if generated docs link to new examples.
- `make check` before completion.
