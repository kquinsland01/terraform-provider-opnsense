// Copyright (c) Matthew Mellor
// SPDX-License-Identifier: MPL-2.0

package kea

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	dsschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/matthew-on-git/terraform-provider-opnsense/pkg/opnsense"
)

var _ datasource.DataSource = &dhcpv4SubnetDataSource{}

type dhcpv4SubnetDataSource struct{ client *opnsense.Client }

func newDHCPv4SubnetDataSource() datasource.DataSource { return &dhcpv4SubnetDataSource{} }

func (d *dhcpv4SubnetDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_kea_dhcpv4_subnet"
}

func (d *dhcpv4SubnetDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = dsschema.Schema{
		MarkdownDescription: "Reads an existing Kea DHCPv4 subnet on OPNsense by UUID.",
		Attributes: map[string]dsschema.Attribute{
			"id":              dsschema.StringAttribute{Required: true, MarkdownDescription: "UUID to look up."},
			"subnet":          dsschema.StringAttribute{Computed: true, MarkdownDescription: "IPv4 subnet in CIDR notation."},
			"allocator":       dsschema.StringAttribute{Computed: true, MarkdownDescription: "Address allocator."},
			"pools":           dsschema.StringAttribute{Computed: true, MarkdownDescription: "DHCPv4 pool definitions."},
			"match_client_id": dsschema.BoolAttribute{Computed: true, MarkdownDescription: "Match clients by client-id."},
			"ping_check":      dsschema.BoolAttribute{Computed: true, MarkdownDescription: "Ping the address before handing out a lease."},
			"auto_collect":    dsschema.BoolAttribute{Computed: true, MarkdownDescription: "Automatically collect option data from OPNsense."},
			"dns_servers":     computedStringSet("IPv4 DNS servers offered to clients."),
			"domain_name":     dsschema.StringAttribute{Computed: true, MarkdownDescription: "Domain name offered to clients."},
			"domain_search":   computedStringSet("Domain-search suffixes offered to clients."),
			"routers":         computedStringSet("IPv4 default routers offered to clients."),
			"static_routes": dsschema.SetNestedAttribute{Computed: true, MarkdownDescription: "IPv4 static route pairs.", NestedObject: dsschema.NestedAttributeObject{Attributes: map[string]dsschema.Attribute{
				"destination": dsschema.StringAttribute{Computed: true, MarkdownDescription: "IPv4 route destination."},
				"router":      dsschema.StringAttribute{Computed: true, MarkdownDescription: "IPv4 next-hop router."},
			}}},
			"ntp_servers":   computedStringSet("IPv4 NTP servers offered to clients."),
			"time_servers":  computedStringSet("IPv4 RFC 868 time servers offered to clients."),
			"next_server":   dsschema.StringAttribute{Computed: true, MarkdownDescription: "Network-boot next-server address."},
			"tftp_server":   dsschema.StringAttribute{Computed: true, MarkdownDescription: "TFTP server name or address."},
			"tftp_bootfile": dsschema.StringAttribute{Computed: true, MarkdownDescription: "Network boot filename."},
			"description":   dsschema.StringAttribute{Computed: true, MarkdownDescription: "Description of the subnet."},
		},
	}
}

func computedStringSet(description string) dsschema.SetAttribute {
	return dsschema.SetAttribute{ElementType: types.StringType, Computed: true, MarkdownDescription: description}
}

func (d *dhcpv4SubnetDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	client, ok := req.ProviderData.(*opnsense.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected Provider Data", "Expected *opnsense.Client.")
		return
	}
	d.client = client
}

func (d *dhcpv4SubnetDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config DHCPv4SubnetResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}
	id := config.ID.ValueString()
	result, err := opnsense.Get[dhcpv4SubnetAPIResponse](ctx, d.client, dhcpv4SubnetReqOpts, id)
	if err != nil {
		resp.Diagnostics.AddError("Error reading Kea DHCPv4 subnet", fmt.Sprintf("Could not read Kea DHCPv4 subnet %s: %s", id, err))
		return
	}
	if err := config.fromAPI(ctx, result, id); err != nil {
		resp.Diagnostics.AddError("Error decoding Kea DHCPv4 subnet", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &config)...)
}
