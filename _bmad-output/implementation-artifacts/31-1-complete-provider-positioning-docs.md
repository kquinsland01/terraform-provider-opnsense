# Story 31.1: Complete Provider Positioning README and Registry Pass

## Status

backlog

## Story

As a Terraform Registry visitor,
I want the provider landing docs to quickly explain the provider's complete OPNsense appliance coverage goal,
so that I can decide whether its breadth and release discipline fit my OPNsense automation needs.

## Acceptance Criteria

1. README and Registry provider index surface the current support headline: 106 resources and 90 data sources, or the validator-confirmed counts current when this story executes.
2. Positioning emphasizes broad modern OPNsense API coverage, plugin/service domains, brownfield import, and release-gated API adoption.
3. Docs avoid unverifiable claims such as "most adopted."
4. A concise coverage section explains the provider's complete-appliance direction without naming other maintainers or framing the project around other providers.
5. Links point to support matrix, migration/import guide, upstream-blocked register, and acceptance-test proof once Story 32.1 exists.
6. Positioning does not suggest running multiple providers against the same objects; it frames this provider as the desired single standard once coverage gaps are closed or documented.

## Dev Notes

- Registry readers scan fast. Put the differentiator above the fold.
- Use precise counts only when they match generated docs.
- Do not begin positioning claims until P0/P1 Stories 31.0a–31.0j are done or explicitly reclassified by the audit.

## Validation

- `make docs` updates generated provider docs if templates change.
- `git diff --check` passes.
- `make check` passes if templates/generated docs change.
