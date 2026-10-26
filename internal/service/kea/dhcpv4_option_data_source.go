// Copyright (c) Matthew Mellor
// SPDX-License-Identifier: MPL-2.0

package kea

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/matthew-on-git/terraform-provider-opnsense/pkg/opnsense"
)

type dhcpv4OptionDataSource struct{ client *opnsense.Client }

func newDHCPv4OptionDataSource() datasource.DataSource { return &dhcpv4OptionDataSource{} }
func (d *dhcpv4OptionDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_kea_dhcpv4_option"
}

func (d *dhcpv4OptionDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*opnsense.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected Provider Data", "Expected *opnsense.Client.")
		return
	}
	d.client = c
}

func (d *dhcpv4OptionDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	attrs := map[string]schema.Attribute{"id": schema.StringAttribute{Required: true, MarkdownDescription: "Exact Kea DHCPv4 option UUID to look up."}}
	for _, key := range []string{"code", "encoding", "data", "match_code", "match_encoding", "match_data", "description"} {
		attrs[key] = schema.StringAttribute{Computed: true, MarkdownDescription: "Saved option " + key + "."}
	}
	attrs["force"] = schema.BoolAttribute{Computed: true, MarkdownDescription: "Stored force flag; not a packet-level force guarantee."}
	resp.Schema = schema.Schema{MarkdownDescription: "Reads a Kea DHCPv4 option by UUID on stable OPNsense 26.7.7 or newer. This lookup never mutates or activates configuration.", Attributes: attrs}
}

func (d *dhcpv4OptionDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var model DHCPv4OptionResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &model)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if !optionIDValid(model.ID.ValueString()) {
		resp.Diagnostics.AddError("Invalid Kea DHCPv4 option UUID", "Lookup requires an exact known UUID.")
		return
	}
	if err := requireDHCPv4Option(ctx, d.client); err != nil {
		resp.Diagnostics.AddError("Kea DHCPv4 option unavailable", err.Error())
		return
	}
	remote, err := readDHCPv4Option(ctx, d.client, model.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error reading Kea DHCPv4 option", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &remote)...)
}
