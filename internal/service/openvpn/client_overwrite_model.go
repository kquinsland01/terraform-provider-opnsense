// Copyright (c) Matthew Mellor
// SPDX-License-Identifier: MPL-2.0

package openvpn

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/matthew-on-git/terraform-provider-opnsense/pkg/opnsense"
)

// ClientOverwriteResourceModel is the Terraform state model for opnsense_openvpn_client_overwrite.
type ClientOverwriteResourceModel struct {
	ID              types.String `tfsdk:"id"`
	Enabled         types.Bool   `tfsdk:"enabled"`
	CommonName      types.String `tfsdk:"common_name"`
	Description     types.String `tfsdk:"description"`
	Servers         types.Set    `tfsdk:"servers"`
	Block           types.Bool   `tfsdk:"block"`
	PushReset       types.Bool   `tfsdk:"push_reset"`
	TunnelNetwork   types.String `tfsdk:"tunnel_network"`
	TunnelNetworkV6 types.String `tfsdk:"tunnel_network_v6"`
	LocalNetworks   types.Set    `tfsdk:"local_networks"`
	RemoteNetworks  types.Set    `tfsdk:"remote_networks"`
	RouteGateway    types.String `tfsdk:"route_gateway"`
	RedirectGateway types.Set    `tfsdk:"redirect_gateway"`
	RegisterDNS     types.Bool   `tfsdk:"register_dns"`
	DNSDomain       types.Set    `tfsdk:"dns_domain"`
	DNSDomainSearch types.Set    `tfsdk:"dns_domain_search"`
	DNSServers      types.Set    `tfsdk:"dns_servers"`
	NTPServers      types.Set    `tfsdk:"ntp_servers"`
	WINSServers     types.Set    `tfsdk:"wins_servers"`
}

type clientOverwriteAPIResponse struct {
	Enabled         string             `json:"enabled"`
	CommonName      string             `json:"common_name"`
	Description     string             `json:"description"`
	Servers         strictSelectedList `json:"servers"`
	Block           string             `json:"block"`
	PushReset       string             `json:"push_reset"`
	TunnelNetwork   string             `json:"tunnel_network"`
	TunnelNetworkV6 string             `json:"tunnel_networkv6"`
	LocalNetworks   strictSelectedList `json:"local_networks"`
	RemoteNetworks  strictSelectedList `json:"remote_networks"`
	RouteGateway    string             `json:"route_gateway"`
	RedirectGateway strictSelectedList `json:"redirect_gateway"`
	RegisterDNS     string             `json:"register_dns"`
	DNSDomain       strictSelectedList `json:"dns_domain"`
	DNSDomainSearch strictSelectedList `json:"dns_domain_search"`
	DNSServers      strictSelectedList `json:"dns_servers"`
	NTPServers      strictSelectedList `json:"ntp_servers"`
	WINSServers     strictSelectedList `json:"wins_servers"`
}

type clientOverwriteAPIRequest struct {
	Enabled         string  `json:"enabled"`
	CommonName      string  `json:"common_name"`
	Description     string  `json:"description"`
	Servers         *string `json:"servers,omitempty"`
	Block           string  `json:"block"`
	PushReset       string  `json:"push_reset"`
	TunnelNetwork   string  `json:"tunnel_network"`
	TunnelNetworkV6 *string `json:"tunnel_networkv6,omitempty"`
	LocalNetworks   *string `json:"local_networks,omitempty"`
	RemoteNetworks  *string `json:"remote_networks,omitempty"`
	RouteGateway    *string `json:"route_gateway,omitempty"`
	RedirectGateway *string `json:"redirect_gateway,omitempty"`
	RegisterDNS     *string `json:"register_dns,omitempty"`
	DNSDomain       *string `json:"dns_domain,omitempty"`
	DNSDomainSearch *string `json:"dns_domain_search,omitempty"`
	DNSServers      *string `json:"dns_servers,omitempty"`
	NTPServers      *string `json:"ntp_servers,omitempty"`
	WINSServers     *string `json:"wins_servers,omitempty"`
}

func (m *ClientOverwriteResourceModel) toAPI(ctx context.Context) (*clientOverwriteAPIRequest, error) {
	values := []types.Set{m.Servers, m.LocalNetworks, m.RemoteNetworks, m.RedirectGateway, m.DNSDomain, m.DNSDomainSearch, m.DNSServers, m.NTPServers, m.WINSServers}
	encoded := make([]*string, len(values))
	for i := range values {
		var err error
		encoded[i], err = collectionCSV(ctx, values[i])
		if err != nil {
			return nil, fmt.Errorf("encode OpenVPN client overwrite collection: %w", err)
		}
	}
	return &clientOverwriteAPIRequest{Enabled: opnsense.BoolToString(m.Enabled.ValueBool()), CommonName: m.CommonName.ValueString(), Description: m.Description.ValueString(), Servers: encoded[0], Block: opnsense.BoolToString(m.Block.ValueBool()), PushReset: opnsense.BoolToString(m.PushReset.ValueBool()), TunnelNetwork: m.TunnelNetwork.ValueString(), TunnelNetworkV6: optionalString(m.TunnelNetworkV6), LocalNetworks: encoded[1], RemoteNetworks: encoded[2], RouteGateway: optionalString(m.RouteGateway), RedirectGateway: encoded[3], RegisterDNS: optionalBool(m.RegisterDNS), DNSDomain: encoded[4], DNSDomainSearch: encoded[5], DNSServers: encoded[6], NTPServers: encoded[7], WINSServers: encoded[8]}, nil
}

func (m *ClientOverwriteResourceModel) fromAPI(_ context.Context, a *clientOverwriteAPIResponse, uuid string) error {
	m.ID = types.StringValue(uuid)
	m.CommonName = types.StringValue(a.CommonName)
	m.Description = types.StringValue(a.Description)
	m.TunnelNetwork = types.StringValue(a.TunnelNetwork)
	m.TunnelNetworkV6 = types.StringValue(a.TunnelNetworkV6)
	m.RouteGateway = types.StringValue(a.RouteGateway)
	m.Servers = sliceToSet(a.Servers)
	m.LocalNetworks = sliceToSet(a.LocalNetworks)
	m.RemoteNetworks = sliceToSet(a.RemoteNetworks)
	m.RedirectGateway = sliceToSet(a.RedirectGateway)
	m.DNSDomain = sliceToSet(a.DNSDomain)
	m.DNSDomainSearch = sliceToSet(a.DNSDomainSearch)
	m.DNSServers = sliceToSet(a.DNSServers)
	m.NTPServers = sliceToSet(a.NTPServers)
	m.WINSServers = sliceToSet(a.WINSServers)
	boolFields := []struct {
		name, raw string
		dest      *types.Bool
	}{{"enabled", a.Enabled, &m.Enabled}, {"block", a.Block, &m.Block}, {"push_reset", a.PushReset, &m.PushReset}, {"register_dns", a.RegisterDNS, &m.RegisterDNS}}
	for _, field := range boolFields {
		value, err := strictBool(field.name, field.raw)
		if err != nil {
			return err
		}
		*field.dest = value
	}
	return nil
}
