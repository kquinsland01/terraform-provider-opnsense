// Copyright (c) Matthew Mellor
// SPDX-License-Identifier: MPL-2.0

package kea

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/matthew-on-git/terraform-provider-opnsense/internal/tfconv"
)

// DHCPv6SubnetResourceModel is the Terraform state model for opnsense_kea_dhcpv6_subnet.
type DHCPv6SubnetResourceModel struct {
	ID           types.String `tfsdk:"id"`
	Subnet       types.String `tfsdk:"subnet"`
	Interface    types.String `tfsdk:"interface"`
	Allocator    types.String `tfsdk:"allocator"`
	PDAllocator  types.String `tfsdk:"pd_allocator"`
	Pools        types.String `tfsdk:"pools"`
	DNSServers   types.Set    `tfsdk:"dns_servers"`
	DomainSearch types.Set    `tfsdk:"domain_search"`
	Description  types.String `tfsdk:"description"`
}

type dhcpv6SubnetAPIResponse struct {
	Subnet      string                    `json:"subnet"`
	Interface   strictSelectedMap         `json:"interface"`
	Allocator   strictOptionalSelectedMap `json:"allocator"`
	PDAllocator strictOptionalSelectedMap `json:"pd-allocator"`
	Pools       string                    `json:"pools"`
	OptionData  *struct {
		DNSServers   *keaOptionList `json:"dns_servers"`
		DomainSearch *keaOptionList `json:"domain_search"`
	} `json:"option_data"`
	Description string `json:"description"`
}

type dhcpv6SubnetAPIRequest struct {
	Subnet      *string `json:"subnet,omitempty"`
	Interface   *string `json:"interface,omitempty"`
	Allocator   *string `json:"allocator,omitempty"`
	PDAllocator *string `json:"pd-allocator,omitempty"`
	Pools       *string `json:"pools,omitempty"`
	OptionData  struct {
		DNSServers   *string `json:"dns_servers,omitempty"`
		DomainSearch *string `json:"domain_search,omitempty"`
	} `json:"option_data"`
	Description *string `json:"description,omitempty"`
}

func (m *DHCPv6SubnetResourceModel) toAPI(ctx context.Context) *dhcpv6SubnetAPIRequest {
	request := &dhcpv6SubnetAPIRequest{
		Subnet:      knownString(m.Subnet),
		Interface:   knownString(m.Interface),
		Allocator:   knownString(m.Allocator),
		PDAllocator: knownString(m.PDAllocator),
		Pools:       knownString(m.Pools),
		Description: knownString(m.Description),
	}
	request.OptionData.DNSServers = knownSetCSV(ctx, m.DNSServers)
	request.OptionData.DomainSearch = knownSetCSV(ctx, m.DomainSearch)
	return request
}

func (m *DHCPv6SubnetResourceModel) fromAPI(_ context.Context, a *dhcpv6SubnetAPIResponse, uuid string) error {
	if a.OptionData == nil {
		return fmt.Errorf("malformed Kea subnet response: option_data is absent or null")
	}
	if a.OptionData.DNSServers == nil {
		return fmt.Errorf("malformed Kea subnet response: option_data.dns_servers is absent or null")
	}
	if a.OptionData.DomainSearch == nil {
		return fmt.Errorf("malformed Kea subnet response: option_data.domain_search is absent or null")
	}
	m.ID = types.StringValue(uuid)
	m.Subnet = types.StringValue(a.Subnet)
	m.Interface = types.StringValue(string(a.Interface))
	m.Allocator = types.StringValue(string(a.Allocator))
	m.PDAllocator = types.StringValue(string(a.PDAllocator))
	m.Pools = types.StringValue(a.Pools)
	m.DNSServers = tfconv.SliceToSet([]string(*a.OptionData.DNSServers))
	m.DomainSearch = tfconv.SliceToSet([]string(*a.OptionData.DomainSearch))
	m.Description = types.StringValue(a.Description)
	return validateDHCPv6SubnetResponse(m)
}
