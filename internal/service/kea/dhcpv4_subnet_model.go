// Copyright (c) Matthew Mellor
// SPDX-License-Identifier: MPL-2.0

package kea

import (
	"context"
	"encoding/json"
	"fmt"
	"net/netip"
	"sort"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/matthew-on-git/terraform-provider-opnsense/internal/tfconv"
	"github.com/matthew-on-git/terraform-provider-opnsense/pkg/opnsense"
)

// DHCPv4SubnetResourceModel is the Terraform state model for opnsense_kea_dhcpv4_subnet.
type DHCPv4SubnetResourceModel struct {
	ID            types.String `tfsdk:"id"`
	Subnet        types.String `tfsdk:"subnet"`
	Allocator     types.String `tfsdk:"allocator"`
	Pools         types.String `tfsdk:"pools"`
	MatchClientID types.Bool   `tfsdk:"match_client_id"`
	PingCheck     types.Bool   `tfsdk:"ping_check"`
	AutoCollect   types.Bool   `tfsdk:"auto_collect"`
	DNSServers    types.Set    `tfsdk:"dns_servers"`
	DomainName    types.String `tfsdk:"domain_name"`
	DomainSearch  types.Set    `tfsdk:"domain_search"`
	Routers       types.Set    `tfsdk:"routers"`
	StaticRoutes  types.Set    `tfsdk:"static_routes"`
	NTPServers    types.Set    `tfsdk:"ntp_servers"`
	TimeServers   types.Set    `tfsdk:"time_servers"`
	NextServer    types.String `tfsdk:"next_server"`
	TFTPServer    types.String `tfsdk:"tftp_server"`
	TFTPBootfile  types.String `tfsdk:"tftp_bootfile"`
	Description   types.String `tfsdk:"description"`
}

type staticRouteModel struct {
	Destination types.String `tfsdk:"destination"`
	Router      types.String `tfsdk:"router"`
}

var staticRouteAttrTypes = map[string]attr.Type{
	"destination": types.StringType,
	"router":      types.StringType,
}

type dhcpv4OptionDataAPIResponse struct {
	DNSServers   *keaOptionList `json:"domain_name_servers"`
	DomainName   string         `json:"domain_name"`
	DomainSearch *keaOptionList `json:"domain_search"`
	Routers      *keaOptionList `json:"routers"`
	StaticRoutes string         `json:"static_routes"`
	NTPServers   *keaOptionList `json:"ntp_servers"`
	TimeServers  *keaOptionList `json:"time_servers"`
	TFTPServer   string         `json:"tftp_server_name"`
	TFTPBootfile string         `json:"boot_file_name"`
}

type keaOptionList []string

func (values *keaOptionList) UnmarshalJSON(data []byte) error {
	trimmed := strings.TrimSpace(string(data))
	if !strings.HasPrefix(trimmed, "{") {
		return fmt.Errorf("expected selected-map list object, got %q", trimmed)
	}
	var selected map[string]struct {
		Selected json.Number `json:"selected"`
	}
	if err := json.Unmarshal(data, &selected); err != nil {
		return fmt.Errorf("expected selected-map list object: %w", err)
	}
	result := make([]string, 0, len(selected))
	emptySelected := false
	for key, entry := range selected {
		switch entry.Selected.String() {
		case "1":
			if key == "" {
				emptySelected = true
				continue
			}
			result = append(result, key)
		case "0":
		default:
			return fmt.Errorf("invalid selected flag %q for %q: expected 0 or 1", entry.Selected.String(), key)
		}
	}
	if emptySelected && len(result) > 0 {
		return fmt.Errorf("selected-map list contains both the empty sentinel and selected values")
	}
	sort.Strings(result)
	*values = result
	return nil
}

type dhcpv4OptionDataAPIRequest struct {
	DNSServers   *string `json:"domain_name_servers,omitempty"`
	DomainName   *string `json:"domain_name,omitempty"`
	DomainSearch *string `json:"domain_search,omitempty"`
	Routers      *string `json:"routers,omitempty"`
	StaticRoutes *string `json:"static_routes,omitempty"`
	NTPServers   *string `json:"ntp_servers,omitempty"`
	TimeServers  *string `json:"time_servers,omitempty"`
	TFTPServer   *string `json:"tftp_server_name,omitempty"`
	TFTPBootfile *string `json:"boot_file_name,omitempty"`
}

type dhcpv4SubnetAPIResponse struct {
	Subnet        string                       `json:"subnet"`
	Allocator     strictOptionalSelectedMap    `json:"allocator"`
	Pools         string                       `json:"pools"`
	MatchClientID string                       `json:"match-client-id"`
	PingCheck     string                       `json:"ping_check"`
	AutoCollect   string                       `json:"option_data_autocollect"`
	NextServer    string                       `json:"next_server"`
	OptionData    *dhcpv4OptionDataAPIResponse `json:"option_data"`
	Description   string                       `json:"description"`
}

type dhcpv4SubnetAPIRequest struct {
	Subnet        *string                    `json:"subnet,omitempty"`
	Allocator     *string                    `json:"allocator,omitempty"`
	Pools         *string                    `json:"pools,omitempty"`
	MatchClientID *string                    `json:"match-client-id,omitempty"`
	PingCheck     *string                    `json:"ping_check,omitempty"`
	AutoCollect   *string                    `json:"option_data_autocollect,omitempty"`
	NextServer    *string                    `json:"next_server,omitempty"`
	OptionData    dhcpv4OptionDataAPIRequest `json:"option_data"`
	Description   *string                    `json:"description,omitempty"`
}

func (m *DHCPv4SubnetResourceModel) toAPI(ctx context.Context) *dhcpv4SubnetAPIRequest {
	return &dhcpv4SubnetAPIRequest{
		Subnet:        knownString(m.Subnet),
		Allocator:     knownString(m.Allocator),
		Pools:         knownString(m.Pools),
		MatchClientID: knownBool(m.MatchClientID),
		PingCheck:     knownBool(m.PingCheck),
		AutoCollect:   knownBool(m.AutoCollect),
		NextServer:    knownString(m.NextServer),
		OptionData: dhcpv4OptionDataAPIRequest{
			DNSServers: knownSetCSV(ctx, m.DNSServers), DomainName: knownString(m.DomainName),
			DomainSearch: knownSetCSV(ctx, m.DomainSearch), Routers: knownSetCSV(ctx, m.Routers),
			StaticRoutes: knownStaticRoutes(ctx, m.StaticRoutes), NTPServers: knownSetCSV(ctx, m.NTPServers),
			TimeServers: knownSetCSV(ctx, m.TimeServers), TFTPServer: knownString(m.TFTPServer),
			TFTPBootfile: knownString(m.TFTPBootfile),
		},
		Description: knownString(m.Description),
	}
}

func (m *DHCPv4SubnetResourceModel) fromAPI(ctx context.Context, a *dhcpv4SubnetAPIResponse, uuid string) error {
	if a.OptionData == nil {
		return fmt.Errorf("malformed Kea subnet response: option_data is absent or null")
	}
	for _, field := range []struct {
		name   string
		values *keaOptionList
	}{
		{"domain_name_servers", a.OptionData.DNSServers},
		{"domain_search", a.OptionData.DomainSearch},
		{"routers", a.OptionData.Routers},
		{"ntp_servers", a.OptionData.NTPServers},
		{"time_servers", a.OptionData.TimeServers},
	} {
		if field.values == nil {
			return fmt.Errorf("malformed Kea subnet response: option_data.%s is absent or null", field.name)
		}
	}
	routes, err := decodeStaticRoutes(ctx, a.OptionData.StaticRoutes)
	if err != nil {
		return err
	}
	m.ID = types.StringValue(uuid)
	m.Subnet = types.StringValue(a.Subnet)
	m.Allocator = types.StringValue(string(a.Allocator))
	m.Pools = types.StringValue(a.Pools)
	matchClientID, err := parseKeaBinaryFlag("match-client-id", a.MatchClientID)
	if err != nil {
		return err
	}
	pingCheck, err := parseKeaBinaryFlag("ping_check", a.PingCheck)
	if err != nil {
		return err
	}
	autoCollect, err := parseKeaBinaryFlag("option_data_autocollect", a.AutoCollect)
	if err != nil {
		return err
	}
	m.MatchClientID = types.BoolValue(matchClientID)
	m.PingCheck = types.BoolValue(pingCheck)
	m.AutoCollect = types.BoolValue(autoCollect)
	m.DNSServers = tfconv.SliceToSet([]string(*a.OptionData.DNSServers))
	m.DomainName = types.StringValue(a.OptionData.DomainName)
	m.DomainSearch = tfconv.SliceToSet([]string(*a.OptionData.DomainSearch))
	m.Routers = tfconv.SliceToSet([]string(*a.OptionData.Routers))
	m.StaticRoutes = routes
	m.NTPServers = tfconv.SliceToSet([]string(*a.OptionData.NTPServers))
	m.TimeServers = tfconv.SliceToSet([]string(*a.OptionData.TimeServers))
	m.NextServer = types.StringValue(a.NextServer)
	m.TFTPServer = types.StringValue(a.OptionData.TFTPServer)
	m.TFTPBootfile = types.StringValue(a.OptionData.TFTPBootfile)
	m.Description = types.StringValue(a.Description)
	return validateDHCPv4SubnetResponse(m)
}

func knownBool(value types.Bool) *string {
	if value.IsNull() || value.IsUnknown() {
		return nil
	}
	result := opnsense.BoolToString(value.ValueBool())
	return &result
}

func knownString(value types.String) *string {
	if value.IsNull() || value.IsUnknown() {
		return nil
	}
	result := value.ValueString()
	return &result
}

func knownSetCSV(ctx context.Context, value types.Set) *string {
	if value.IsNull() || value.IsUnknown() {
		return nil
	}
	for _, element := range value.Elements() {
		if element.IsNull() || element.IsUnknown() {
			return nil
		}
	}
	result := tfconv.SetToCSV(ctx, value)
	return &result
}

func knownStaticRoutes(ctx context.Context, value types.Set) *string {
	if value.IsNull() || value.IsUnknown() {
		return nil
	}
	var values []staticRouteModel
	if value.ElementsAs(ctx, &values, false).HasError() {
		return nil
	}
	for _, route := range values {
		if route.Destination.IsNull() || route.Destination.IsUnknown() || route.Router.IsNull() || route.Router.IsUnknown() {
			return nil
		}
	}
	pairs := make([]string, 0, len(values))
	for _, route := range values {
		pairs = append(pairs, route.Destination.ValueString()+","+route.Router.ValueString())
	}
	sort.Strings(pairs)
	result := strings.Join(pairs, ",")
	return &result
}

func parseKeaBinaryFlag(field, value string) (bool, error) {
	switch value {
	case "0":
		return false, nil
	case "1":
		return true, nil
	default:
		return false, fmt.Errorf("invalid %s value %q: expected 0 or 1", field, value)
	}
}

func decodeStaticRoutes(ctx context.Context, raw string) (types.Set, error) {
	if strings.TrimSpace(raw) == "" {
		return types.SetValueMust(types.ObjectType{AttrTypes: staticRouteAttrTypes}, []attr.Value{}), nil
	}
	parts := opnsense.CSVToSlice(raw)
	if len(parts)%2 != 0 {
		return types.SetNull(types.ObjectType{AttrTypes: staticRouteAttrTypes}), fmt.Errorf("malformed Kea static_routes response %q: expected destination/router pairs", raw)
	}
	routes := make([]staticRouteModel, 0, len(parts)/2)
	for i := 0; i < len(parts); i += 2 {
		if destination, err := netip.ParseAddr(parts[i]); err != nil || !destination.Is4() {
			return types.SetNull(types.ObjectType{AttrTypes: staticRouteAttrTypes}), fmt.Errorf("malformed Kea static_routes destination %q", parts[i])
		}
		if router, err := netip.ParseAddr(parts[i+1]); err != nil || !router.Is4() {
			return types.SetNull(types.ObjectType{AttrTypes: staticRouteAttrTypes}), fmt.Errorf("malformed Kea static_routes router %q", parts[i+1])
		}
		routes = append(routes, staticRouteModel{Destination: types.StringValue(parts[i]), Router: types.StringValue(parts[i+1])})
	}
	value, diagnostics := types.SetValueFrom(ctx, types.ObjectType{AttrTypes: staticRouteAttrTypes}, routes)
	if diagnostics.HasError() {
		return types.SetNull(types.ObjectType{AttrTypes: staticRouteAttrTypes}), fmt.Errorf("decode Kea static_routes response: %s", diagnostics.Errors()[0].Detail())
	}
	return value, nil
}
