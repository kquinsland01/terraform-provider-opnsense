# Story 31.0i: Live Interface Data Sources

## Status

backlog

## Priority

P1 — complete before Stories 31.1–31.3.

## Evidence and Scope

Add read-only equivalents for the pinned `interface`, `interface_all`, `interfaces_overview`, and `interfaces_overview_all` data sources. These do not replace `opnsense_interface_assignment`, which owns durable assignment/IP configuration.

Exact pinned endpoint/client evidence:

- `interface` calls `Diagnostics().GetInterface(ctx, device)`, which filters `GET /api/diagnostics/interface/getInterfaceConfig`; pinned provider source: [`interface_data_source.go`](https://github.com/browningluke/terraform-provider-opnsense/blob/33c2a03c2bc1a7339cb5aa5b611e15014e27f7ba/internal/service/diagnostics/interface_data_source.go), pinned client source: [`diagnostics/interface.go`](https://github.com/browningluke/opnsense-go/blob/9cd77265c81e329ba6d4399defcbf884c61ac75f/pkg/diagnostics/interface.go).
- `interface_all` calls `Diagnostics().GetInterfaceAll(ctx)` against the same `GET /api/diagnostics/interface/getInterfaceConfig` endpoint without a device filter; pinned provider source: [`interface_all_data_source.go`](https://github.com/browningluke/terraform-provider-opnsense/blob/33c2a03c2bc1a7339cb5aa5b611e15014e27f7ba/internal/service/diagnostics/interface_all_data_source.go), pinned client source: [`diagnostics/interface.go`](https://github.com/browningluke/opnsense-go/blob/9cd77265c81e329ba6d4399defcbf884c61ac75f/pkg/diagnostics/interface.go).
- `interfaces_overview` calls `Interfaces().OverviewGet(ctx)`, then selects the requested device from `GET /api/interfaces/overview/interfaces_info`; pinned provider source: [`overview_data_source.go`](https://github.com/browningluke/terraform-provider-opnsense/blob/33c2a03c2bc1a7339cb5aa5b611e15014e27f7ba/internal/service/interfaces/overview_data_source.go), pinned client source: [`interfaces/overview.go`](https://github.com/browningluke/opnsense-go/blob/9cd77265c81e329ba6d4399defcbf884c61ac75f/pkg/interfaces/overview.go).
- `interfaces_overview_all` calls the same `Interfaces().OverviewGet(ctx)` client and returns every row from `GET /api/interfaces/overview/interfaces_info`; pinned provider source: [`overview_all_data_source.go`](https://github.com/browningluke/terraform-provider-opnsense/blob/33c2a03c2bc1a7339cb5aa5b611e15014e27f7ba/internal/service/interfaces/overview_all_data_source.go), pinned client source: [`interfaces/overview.go`](https://github.com/browningluke/opnsense-go/blob/9cd77265c81e329ba6d4399defcbf884c61ac75f/pkg/interfaces/overview.go).

The pinned models cover device status, flags, media, capabilities, addresses, MTU, gateways/routes, VLAN/LAGG details, and DHCP-learned values.

## Acceptance Criteria

- **Given** an interface device or identifier, **when** the single-item data sources read, **then** runtime and overview fields match the API without claiming configuration ownership.
- **Given** multiple interfaces, **when** aggregate data sources read, **then** ordering and nested values are deterministic and unknown/disappearing devices produce actionable diagnostics.
- **Given** `interface_assignment`, **when** both surfaces are configured, **then** only the resource mutates configuration and the data sources remain read-only.

## Verification

Interface data-source unit tests, live read-only validation on a supported appliance, docs generation, and `make check`.
