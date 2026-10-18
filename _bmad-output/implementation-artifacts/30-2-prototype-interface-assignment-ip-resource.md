# Story 30.2: Implement Interface Assignment/IP Resource for OPNsense 26.7.6+

## Status

done

## Story

As an OPNsense Terraform user,
I want an experimental interface assignment/settings resource guarded by OPNsense version capability checks,
so that I can manage interface enablement and static/DHCP address settings once upstream ships the expanded assignment API.

## Acceptance Criteria

1. Prototype resource name is selected deliberately, with a preference for `opnsense_interface_assignment` or `opnsense_interface_settings`.
2. Resource can import an existing `wan`, `lan`, or `optN` assignment by interface name or stable upstream identifier.
3. Resource supports enable/disable and IPv4 mode/address/prefix/gateway where upstream supports it.
4. Resource supports IPv6 mode/address/prefix/gateway where upstream supports it.
5. Create/update calls use `api/interfaces/assignment/set_item/{ifname}` followed by `api/interfaces/assignment/reconfigure`.
6. Terraform plan is no-change after apply against a disposable OPNsense `26.7.6` or newer appliance with the expanded API.
7. Provider fails clearly on OPNsense versions that lack the expanded assignment API.
8. Tests prove both the supported path and the "API missing" diagnostic path.

## Dev Notes

- Story 30.1 identified OPNsense `26.7.6` as the first released tag carrying the complete required API surface.
- Avoid overload with API-backed virtual interface resources already supported by the provider, such as VLAN, bridge, GIF, GRE, LAGG, loopback, neighbor, and VXLAN.
- Base assignment/IP config has stricter blast radius than most resources. Acceptance tests must run on disposable interfaces in an OPNsense `26.7.6` or newer QEMU/Vagrant appliance, not production firewall interfaces.
- Preserve global mutation serialization and reconfigure behavior.

## Validation

- Unit tests cover schema/version guard behavior.
- Acceptance test evidence is recorded against a disposable OPNsense `26.7.6` or newer VM.
- `make check` passes.

## Implementation Record

### Implemented

- Added the hand-written `opnsense_interface_assignment` resource for adopting
  an existing `wan`, `lan`, or `optN` assignment by immutable interface name.
- Added firmware and expanded-response capability gates before mutation. Old,
  partial, missing, endpoint-not-found, and unauthorized APIs produce a
  resource-specific OPNsense 26.7.6+ diagnostic rather than a plugin error.
- Added safe enablement, static/DHCP/none IPv4 modes, supported IPv6 modes,
  address/prefix/gateway fields, and the released DHCPv4/DHCPv6 setting subset.
- Update payloads carry current unmanaged assignment fields forward, use the
  provider-wide locked update/reconfigure path, and always read API state back.
- Delete is state-only. No assignment delete or disable operation is called.
- Added HTTP contract/unit coverage, guarded disposable-`optN` acceptance
  coverage with full pre-test restoration, Registry docs, examples, and runner
  environment forwarding.
- Normalized both keyed and indexed OPNsense choice payloads before preserving
  unmanaged fields, and mapped Terraform's explicit `none` modes to the blank
  option values required by the assignment API.

### Verification Evidence

- `go test ./internal/provider ./internal/service/iface ./pkg/opnsense` passes.
- `go vet ./internal/service/iface ./pkg/opnsense` passes.
- `DEVRAIL_IMAGE=ghcr.io/devrail-dev/dev-toolchain:1.12.24 make check`
  passes all lint, format, test, security, scan, and docs gates. The repository
  default 1.12.0 image predates the current Go 1.26 baseline.
- `git diff --check` passes.
- Live acceptance passed against a disposable OPNsense 26.7.7 VM using isolated
  `opt1` and TEST-NET-1 addresses. `TestAccInterfaceAssignment` proved apply,
  an empty second plan, import, update, out-of-band drift detection and repair,
  and exact restoration of the captured pre-test assignment state.
- The live appliance exposed indexed `OptionField` responses and blank
  `none` mode values that the initial fixture did not model. Regression tests
  now cover both wire formats, and the focused live test passed after the
  fixes.
- Four independent build-auto review layers produced 31 findings. Seventeen
  were patched, including unsupported-mode protection, recovery state after
  read-back failure, transport diagnostics, drift proof, and exact cleanup;
  all rejected findings and rationale are recorded in the implementation spec.
