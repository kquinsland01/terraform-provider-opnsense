# Story 32.1: Acceptance Test Proof Artifact

## Status

backlog

## Story

As a cautious infrastructure operator,
I want visible acceptance-test proof for provider releases,
so that I can trust that documented resources were exercised against a real OPNsense appliance.

## Acceptance Criteria

1. Release or nightly process produces a package-level acceptance summary.
2. Summary separates unit tests from live appliance acceptance tests.
3. Summary reports tested OPNsense version, provider version/commit, domains covered, skipped domains, and known blockers.
4. Artifact is linked from README or Registry docs once stable.
5. Acceptance runner remains serial for mutations unless the test appliance and provider support parallelism safely.

## Dev Notes

- The provider already has local/live validation history. This story makes it visible and repeatable.
- QEMU/Vagrant disposable appliance proof is enough; do not depend on production or shared lab firewall interfaces.

## Validation

- A sample artifact is generated from the current test run.
- CI or documented local command can reproduce it.
- `make check` passes.
