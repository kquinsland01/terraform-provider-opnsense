// Copyright (c) Matthew Mellor
// SPDX-License-Identifier: MPL-2.0

package kea

import (
	"context"
	"encoding/json"
	"fmt"
	"net/netip"
	"net/url"
	"regexp"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// strictSelectedMap is used only for Kea fields whose tagged API contract is
// exactly one selected entry. The shared SelectedMap remains permissive for
// older OPNsense surfaces with inconsistent empty/scalar shapes.
type strictSelectedMap string

func (s *strictSelectedMap) UnmarshalJSON(data []byte) error {
	var values map[string]struct {
		Selected json.Number `json:"selected"`
	}
	if err := json.Unmarshal(data, &values); err != nil {
		return fmt.Errorf("expected a selected-map object: %w", err)
	}
	selected := ""
	selectedCount := 0
	for key, entry := range values {
		switch entry.Selected.String() {
		case "0":
		case "1":
			selectedCount++
			selected = key
		default:
			return fmt.Errorf("selected-map value %q has invalid selected flag %q: expected 0 or 1", key, entry.Selected.String())
		}
	}
	if selectedCount != 1 {
		return fmt.Errorf("selected-map must contain exactly one selected value")
	}
	*s = strictSelectedMap(selected)
	return nil
}

type strictOptionalSelectedMap string

func (s *strictOptionalSelectedMap) UnmarshalJSON(data []byte) error {
	trimmed := strings.TrimSpace(string(data))
	if trimmed == "null" || trimmed == "[]" || trimmed == `{}` || trimmed == `""` {
		*s = ""
		return nil
	}
	var values map[string]struct {
		Selected json.Number `json:"selected"`
	}
	if err := json.Unmarshal(data, &values); err != nil {
		return fmt.Errorf("expected an optional selected-map object: %w", err)
	}
	selected := ""
	selectedCount := 0
	for key, entry := range values {
		switch entry.Selected.String() {
		case "0":
		case "1":
			selectedCount++
			selected = key
		default:
			return fmt.Errorf("selected-map value %q has invalid selected flag %q: expected 0 or 1", key, entry.Selected.String())
		}
	}
	if selectedCount > 1 {
		return fmt.Errorf("optional selected-map has %d selected values; expected at most one", selectedCount)
	}
	*s = strictOptionalSelectedMap(selected)
	return nil
}

var (
	_ resource.ResourceWithConfigValidators = &dhcpv4SubnetResource{}
	_ resource.ResourceWithConfigValidators = &dhcpv6SubnetResource{}
	_ resource.ResourceWithConfigValidators = &dhcpv6PeerResource{}
	_ resource.ResourceWithConfigValidators = &dhcpv6PdPoolResource{}
)

type keaConfigValidator[T any] struct {
	description string
	validate    func(context.Context, T, *resource.ValidateConfigResponse)
}

func (v keaConfigValidator[T]) Description(context.Context) string { return v.description }
func (v keaConfigValidator[T]) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (v keaConfigValidator[T]) ValidateResource(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var model T
	resp.Diagnostics.Append(req.Config.Get(ctx, &model)...)
	if !resp.Diagnostics.HasError() {
		v.validate(ctx, model, resp)
	}
}

func (r *dhcpv4SubnetResource) ConfigValidators(context.Context) []resource.ConfigValidator {
	return []resource.ConfigValidator{keaConfigValidator[DHCPv4SubnetResourceModel]{description: "validates advanced DHCPv4 subnet option values", validate: validateDHCPv4Subnet}}
}

func (r *dhcpv6SubnetResource) ConfigValidators(context.Context) []resource.ConfigValidator {
	return []resource.ConfigValidator{keaConfigValidator[DHCPv6SubnetResourceModel]{description: "validates DHCPv6 subnet option values", validate: validateDHCPv6Subnet}}
}

func (r *dhcpv6PeerResource) ConfigValidators(context.Context) []resource.ConfigValidator {
	return []resource.ConfigValidator{keaConfigValidator[DHCPv6PeerResourceModel]{description: "validates the DHCPv6 peer URL", validate: validateDHCPv6Peer}}
}

func (r *dhcpv6PdPoolResource) ConfigValidators(context.Context) []resource.ConfigValidator {
	return []resource.ConfigValidator{keaConfigValidator[DHCPv6PDPoolResourceModel]{description: "validates the DHCPv6 delegated-prefix pool", validate: validateDHCPv6PDPool}}
}

func validateDHCPv4Subnet(ctx context.Context, model DHCPv4SubnetResourceModel, resp *resource.ValidateConfigResponse) {
	validatePrefix(resp, path.Root("subnet"), model.Subnet, false)
	validateAllocator(resp, path.Root("allocator"), model.Allocator, "iterative", "random", "flq")
	validateAddressSet(ctx, resp, path.Root("dns_servers"), model.DNSServers, false)
	validateAddressSet(ctx, resp, path.Root("routers"), model.Routers, false)
	validateAddressSet(ctx, resp, path.Root("ntp_servers"), model.NTPServers, false)
	validateAddressSet(ctx, resp, path.Root("time_servers"), model.TimeServers, false)
	validateDomains(ctx, resp, path.Root("domain_search"), model.DomainSearch)
	validateDomain(resp, path.Root("domain_name"), model.DomainName)
	validateOptionalAddress(resp, path.Root("next_server"), model.NextServer, false)
	validateKeaText(resp, path.Root("tftp_server"), model.TFTPServer)
	validateKeaText(resp, path.Root("tftp_bootfile"), model.TFTPBootfile)
	if !model.AutoCollect.IsNull() && !model.AutoCollect.IsUnknown() && model.AutoCollect.ValueBool() &&
		(isManagedSet(model.DNSServers) || isManagedSet(model.Routers) || isManagedSet(model.NTPServers)) {
		resp.Diagnostics.AddAttributeError(path.Root("auto_collect"), "Conflicting automatic option collection", "set auto_collect to false when managing dns_servers, routers, or ntp_servers")
	}
	if !model.StaticRoutes.IsNull() && !model.StaticRoutes.IsUnknown() {
		for _, element := range model.StaticRoutes.Elements() {
			if element.IsNull() || element.IsUnknown() {
				continue
			}
			object, ok := element.(types.Object)
			if !ok {
				continue
			}
			destination, _ := object.Attributes()["destination"].(types.String)
			router, _ := object.Attributes()["router"].(types.String)
			validateRequiredAddress(resp, path.Root("static_routes"), "destination", destination, false)
			validateRequiredAddress(resp, path.Root("static_routes"), "router", router, false)
		}
	}
}

func isManagedSet(value types.Set) bool {
	if value.IsNull() || value.IsUnknown() {
		return false
	}
	for _, element := range value.Elements() {
		if element.IsNull() || element.IsUnknown() {
			return false
		}
	}
	return true
}

func validateDHCPv4SubnetResponse(model *DHCPv4SubnetResourceModel) error {
	if allocator := model.Allocator.ValueString(); allocator != "" && allocator != "iterative" && allocator != "random" && allocator != "flq" {
		return fmt.Errorf("invalid allocator response %q: expected iterative, random, flq, or empty", allocator)
	}
	if err := validateResponsePrefix(model.Subnet.ValueString(), false); err != nil {
		return fmt.Errorf("invalid subnet response: %w", err)
	}
	for field, values := range map[string]types.Set{"dns_servers": model.DNSServers, "routers": model.Routers, "ntp_servers": model.NTPServers, "time_servers": model.TimeServers} {
		for _, element := range values.Elements() {
			value := element.(types.String).ValueString()
			address, err := netip.ParseAddr(value)
			if err != nil || !address.Is4() {
				return fmt.Errorf("invalid %s response member %q", field, value)
			}
		}
	}
	for _, element := range model.StaticRoutes.Elements() {
		object, ok := element.(types.Object)
		if !ok || object.IsNull() || object.IsUnknown() {
			return fmt.Errorf("invalid static_routes response element")
		}
		for _, field := range []string{"destination", "router"} {
			value, ok := object.Attributes()[field].(types.String)
			if !ok || value.IsNull() || value.IsUnknown() || strings.TrimSpace(value.ValueString()) == "" {
				return fmt.Errorf("invalid static_routes response %s: expected a non-empty IPv4 address", field)
			}
			address, err := netip.ParseAddr(value.ValueString())
			if err != nil || !address.Is4() {
				return fmt.Errorf("invalid static_routes response %s %q", field, value.ValueString())
			}
		}
	}
	for _, element := range model.DomainSearch.Elements() {
		value := element.(types.String).ValueString()
		if len(value) > 253 || !domainPattern.MatchString(value) {
			return fmt.Errorf("invalid domain_search response member %q", value)
		}
	}
	if value := model.DomainName.ValueString(); value != "" && (len(value) > 253 || !domainPattern.MatchString(value)) {
		return fmt.Errorf("invalid domain_name response %q", value)
	}
	if value := model.NextServer.ValueString(); value != "" {
		address, err := netip.ParseAddr(value)
		if err != nil || !address.Is4() {
			return fmt.Errorf("invalid next_server response %q", value)
		}
	}
	for field, value := range map[string]string{"tftp_server": model.TFTPServer.ValueString(), "tftp_bootfile": model.TFTPBootfile.ValueString()} {
		if strings.ContainsAny(value, "\r\n\"") {
			return fmt.Errorf("invalid %s response: quotes and newlines are not allowed", field)
		}
	}
	return nil
}

func validateDHCPv6SubnetResponse(model *DHCPv6SubnetResourceModel) error {
	if strings.TrimSpace(model.Interface.ValueString()) == "" {
		return fmt.Errorf("invalid interface response: expected one selected interface")
	}
	if err := validateAllocatorResponse("allocator", model.Allocator.ValueString(), "iterative", "random"); err != nil {
		return err
	}
	if err := validateAllocatorResponse("pd_allocator", model.PDAllocator.ValueString(), "iterative", "random", "flq"); err != nil {
		return err
	}
	if err := validateResponsePrefix(model.Subnet.ValueString(), true); err != nil {
		return fmt.Errorf("invalid subnet response: %w", err)
	}
	for _, element := range model.DNSServers.Elements() {
		value := element.(types.String).ValueString()
		address, err := netip.ParseAddr(value)
		if err != nil || !address.Is6() {
			return fmt.Errorf("invalid dns_servers response member %q", value)
		}
	}
	for _, element := range model.DomainSearch.Elements() {
		value := element.(types.String).ValueString()
		if len(value) > 253 || !domainPattern.MatchString(value) {
			return fmt.Errorf("invalid domain_search response member %q", value)
		}
	}
	return nil
}

func validateResponsePrefix(value string, ipv6 bool) error {
	prefix, err := netip.ParsePrefix(value)
	if err != nil || prefix.Addr().Is6() != ipv6 || prefix != prefix.Masked() {
		return fmt.Errorf("invalid or unaligned CIDR %q", value)
	}
	return nil
}

func validateDHCPv6Subnet(ctx context.Context, model DHCPv6SubnetResourceModel, resp *resource.ValidateConfigResponse) {
	validatePrefix(resp, path.Root("subnet"), model.Subnet, true)
	if !model.Interface.IsNull() && !model.Interface.IsUnknown() && strings.TrimSpace(model.Interface.ValueString()) == "" {
		resp.Diagnostics.AddAttributeError(path.Root("interface"), "Invalid DHCPv6 interface", "interface must identify one OPNsense interface")
	}
	validateAllocator(resp, path.Root("allocator"), model.Allocator, "iterative", "random")
	validateAllocator(resp, path.Root("pd_allocator"), model.PDAllocator, "iterative", "random", "flq")
	validateAddressSet(ctx, resp, path.Root("dns_servers"), model.DNSServers, true)
	validateDomains(ctx, resp, path.Root("domain_search"), model.DomainSearch)
}

func validateDHCPv6Peer(_ context.Context, model DHCPv6PeerResourceModel, resp *resource.ValidateConfigResponse) {
	if model.URL.IsNull() || model.URL.IsUnknown() {
		return
	}
	parsed, err := url.ParseRequestURI(model.URL.ValueString())
	if err != nil || parsed.Scheme == "" || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		resp.Diagnostics.AddAttributeError(path.Root("url"), "Invalid DHCPv6 peer URL", "url must be an absolute HTTP or HTTPS control-agent URL")
	}
}

func validateDHCPv6PeerResponse(model *DHCPv6PeerResourceModel) error {
	if strings.TrimSpace(model.Name.ValueString()) == "" {
		return fmt.Errorf("invalid name response: expected a non-empty peer name")
	}
	role := model.Role.ValueString()
	if role != "primary" && role != "standby" {
		return fmt.Errorf("invalid role response %q: expected primary or standby", role)
	}
	parsed, err := url.ParseRequestURI(model.URL.ValueString())
	if err != nil || parsed.Scheme == "" || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return fmt.Errorf("invalid URL response %q: expected an absolute HTTP or HTTPS URL", model.URL.ValueString())
	}
	return nil
}

func validateDHCPv6PDPool(_ context.Context, model DHCPv6PDPoolResourceModel, resp *resource.ValidateConfigResponse) {
	if !model.SubnetID.IsNull() && !model.SubnetID.IsUnknown() && strings.TrimSpace(model.SubnetID.ValueString()) == "" {
		resp.Diagnostics.AddAttributeError(path.Root("subnet_id"), "Invalid parent subnet", "subnet_id must identify an existing DHCPv6 subnet")
	}
	if model.Prefix.IsNull() || model.Prefix.IsUnknown() || model.PrefixLength.IsNull() || model.PrefixLength.IsUnknown() {
		return
	}
	prefixLength := int(model.PrefixLength.ValueInt64())
	if prefixLength < 1 || prefixLength > 128 {
		resp.Diagnostics.AddAttributeError(path.Root("prefix_len"), "Invalid aggregate prefix length", "prefix_len must be between 1 and 128")
		return
	}
	if err := validatePDPoolPrefix(model.Prefix.ValueString(), int64(prefixLength)); err != nil {
		resp.Diagnostics.AddAttributeError(path.Root("prefix"), "Invalid delegated-prefix pool", err.Error())
	}
	if !model.DelegatedLength.IsNull() && !model.DelegatedLength.IsUnknown() {
		delegatedLength := model.DelegatedLength.ValueInt64()
		if delegatedLength < 1 || delegatedLength > 128 {
			resp.Diagnostics.AddAttributeError(path.Root("delegated_len"), "Invalid delegated prefix length", "delegated_len must be between 1 and 128")
		} else if delegatedLength <= model.PrefixLength.ValueInt64() {
			resp.Diagnostics.AddAttributeError(path.Root("delegated_len"), "Invalid delegated prefix length", "delegated_len must be greater than prefix_len")
		}
	}
}

func validateDHCPv6PDPoolResponse(model *DHCPv6PDPoolResourceModel) error {
	if strings.TrimSpace(model.SubnetID.ValueString()) == "" {
		return fmt.Errorf("invalid subnet_id response: expected one selected parent subnet")
	}
	if err := validatePDPoolPrefix(model.Prefix.ValueString(), model.PrefixLength.ValueInt64()); err != nil {
		return fmt.Errorf("invalid prefix response %q: %w", model.Prefix.ValueString(), err)
	}
	if model.DelegatedLength.ValueInt64() <= model.PrefixLength.ValueInt64() {
		return fmt.Errorf("invalid delegated_len response %d: must be greater than prefix_len %d", model.DelegatedLength.ValueInt64(), model.PrefixLength.ValueInt64())
	}
	return nil
}

func validatePDPoolPrefix(value string, prefixLength int64) error {
	if address, err := netip.ParseAddr(value); err == nil {
		if !address.Is6() {
			return fmt.Errorf("prefix must be an IPv6 address or CIDR prefix")
		}
		prefix := netip.PrefixFrom(address, int(prefixLength))
		if prefix.Masked().Addr() != address {
			return fmt.Errorf("prefix must be aligned to prefix_len %d", prefixLength)
		}
		return nil
	}
	prefix, err := netip.ParsePrefix(value)
	if err != nil || !prefix.Addr().Is6() {
		return fmt.Errorf("prefix must be an IPv6 address or CIDR prefix")
	}
	if int64(prefix.Bits()) != prefixLength {
		return fmt.Errorf("CIDR prefix length %d must match prefix_len %d", prefix.Bits(), prefixLength)
	}
	if prefix != prefix.Masked() {
		return fmt.Errorf("prefix must be aligned to prefix_len %d", prefixLength)
	}
	return nil
}

func normalizeCIDRToAddress(value string) string {
	prefix, err := netip.ParsePrefix(value)
	if err != nil {
		return value
	}
	return prefix.Addr().String()
}

func preserveEquivalentCIDR(current types.String, apiValue string) string {
	if current.IsNull() || current.IsUnknown() {
		return apiValue
	}
	prefix, err := netip.ParsePrefix(current.ValueString())
	if err == nil && prefix.Addr().String() == apiValue {
		return current.ValueString()
	}
	return apiValue
}

// fromAPIResource preserves configured spelling during ordinary refreshes and
// reconstructs the canonical CIDR during import, where no prior prefix exists.
// The data source deliberately continues to call fromAPI and expose API truth.
func (m *DHCPv6PDPoolResourceModel) fromAPIResource(ctx context.Context, response *dhcpv6PdPoolAPIResponse, id string) error {
	hadConfiguredPrefix := !m.Prefix.IsNull() && !m.Prefix.IsUnknown()
	if err := m.fromAPI(ctx, response, id); err != nil {
		return err
	}
	if hadConfiguredPrefix {
		return nil
	}
	if _, err := netip.ParsePrefix(m.Prefix.ValueString()); err == nil {
		return nil
	}
	address, err := netip.ParseAddr(m.Prefix.ValueString())
	if err != nil {
		return fmt.Errorf("prefix: reconstruct CIDR: %w", err)
	}
	m.Prefix = types.StringValue(netip.PrefixFrom(address, int(m.PrefixLength.ValueInt64())).String())
	return nil
}

func validateKeaText(resp *resource.ValidateConfigResponse, attributePath path.Path, value types.String) {
	if value.IsNull() || value.IsUnknown() {
		return
	}
	if strings.ContainsAny(value.ValueString(), "\r\n\"") {
		resp.Diagnostics.AddAttributeError(attributePath, "Invalid Kea text value", "value must not contain quotes or newlines")
	}
}

func validatePrefix(resp *resource.ValidateConfigResponse, attributePath path.Path, value types.String, ipv6 bool) {
	if value.IsNull() || value.IsUnknown() {
		return
	}
	prefix, err := netip.ParsePrefix(value.ValueString())
	if err != nil || prefix.Addr().Is6() != ipv6 {
		family := "IPv4"
		if ipv6 {
			family = "IPv6"
		}
		resp.Diagnostics.AddAttributeError(attributePath, "Invalid subnet", "value must be a valid "+family+" CIDR prefix")
		return
	}
	if prefix != prefix.Masked() {
		resp.Diagnostics.AddAttributeError(attributePath, "Unaligned subnet", "subnet must use its network address")
	}
}

func validateAddressSet(_ context.Context, resp *resource.ValidateConfigResponse, attributePath path.Path, value types.Set, ipv6 bool) {
	if value.IsNull() || value.IsUnknown() {
		return
	}
	for _, element := range value.Elements() {
		addressValue, ok := element.(types.String)
		if !ok || addressValue.IsNull() || addressValue.IsUnknown() {
			continue
		}
		address := addressValue.ValueString()
		parsed, err := netip.ParseAddr(address)
		if err != nil || parsed.Is6() != ipv6 {
			resp.Diagnostics.AddAttributeError(attributePath, "Invalid address", fmt.Sprintf("%q is not a valid address for this field", address))
		}
	}
}

func validateRequiredAddress(resp *resource.ValidateConfigResponse, attributePath path.Path, field string, value types.String, ipv6 bool) {
	if value.IsNull() || value.IsUnknown() {
		return
	}
	if strings.TrimSpace(value.ValueString()) == "" {
		resp.Diagnostics.AddAttributeError(attributePath, "Invalid static route", field+" must be a non-empty IPv4 address")
		return
	}
	validateOptionalAddress(resp, attributePath, value, ipv6)
}

func validateAllocator(resp *resource.ValidateConfigResponse, attributePath path.Path, value types.String, allowed ...string) {
	if value.IsNull() || value.IsUnknown() {
		return
	}
	if err := validateAllocatorResponse(attributePath.String(), value.ValueString(), allowed...); err != nil {
		resp.Diagnostics.AddAttributeError(attributePath, "Invalid allocator", err.Error())
	}
}

func validateAllocatorResponse(field, value string, allowed ...string) error {
	if value == "" {
		return nil
	}
	for _, candidate := range allowed {
		if value == candidate {
			return nil
		}
	}
	return fmt.Errorf("invalid %s %q: expected %s, or empty", field, value, strings.Join(allowed, ", "))
}

func validateOptionalAddress(resp *resource.ValidateConfigResponse, attributePath path.Path, value types.String, ipv6 bool) {
	if value.IsNull() || value.IsUnknown() || strings.TrimSpace(value.ValueString()) == "" {
		return
	}
	parsed, err := netip.ParseAddr(value.ValueString())
	if err != nil || parsed.Is6() != ipv6 {
		resp.Diagnostics.AddAttributeError(attributePath, "Invalid address", fmt.Sprintf("%q is not a valid address for this field", value.ValueString()))
	}
}

var domainPattern = regexp.MustCompile(`(?i)^(?:[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?\.)*[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?\.?$`)

func validateDomain(resp *resource.ValidateConfigResponse, attributePath path.Path, value types.String) {
	if value.IsNull() || value.IsUnknown() || value.ValueString() == "" {
		return
	}
	if len(value.ValueString()) > 253 || !domainPattern.MatchString(value.ValueString()) {
		resp.Diagnostics.AddAttributeError(attributePath, "Invalid domain name", fmt.Sprintf("%q is not a valid domain name", value.ValueString()))
	}
}

func validateDomains(_ context.Context, resp *resource.ValidateConfigResponse, attributePath path.Path, value types.Set) {
	if value.IsNull() || value.IsUnknown() {
		return
	}
	for _, element := range value.Elements() {
		domain, ok := element.(types.String)
		if !ok || domain.IsNull() || domain.IsUnknown() {
			continue
		}
		validateDomain(resp, attributePath, domain)
	}
}
