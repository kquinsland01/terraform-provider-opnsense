// Copyright (c) Matthew Mellor
// SPDX-License-Identifier: MPL-2.0

package system

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/matthew-on-git/terraform-provider-opnsense/pkg/opnsense"
)

type gatewayGroupDataSource struct{ client *opnsense.Client }

func newGatewayGroupDataSource() datasource.DataSource { return &gatewayGroupDataSource{} }
func (d *gatewayGroupDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_system_gateway_group"
}

func (d *gatewayGroupDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *gatewayGroupDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	attrs := map[string]schema.Attribute{"id": schema.StringAttribute{Required: true, MarkdownDescription: "Exact gateway group UUID to look up."}}
	for _, key := range []string{"name", "trigger", "pool_options", "description"} {
		attrs[key] = schema.StringAttribute{Computed: true, MarkdownDescription: "Gateway group " + key + "."}
	}
	for i := 1; i <= 5; i++ {
		attrs[fmt.Sprintf("tier_%d", i)] = schema.SetAttribute{Computed: true, ElementType: types.StringType, MarkdownDescription: "Selected gateway names at this tier."}
	}
	resp.Schema = schema.Schema{MarkdownDescription: "Reads a gateway group by UUID on stable OPNsense 26.7.7 or newer. This lookup never mutates or activates configuration.", Attributes: attrs}
}

func (d *gatewayGroupDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var model GatewayGroupResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &model)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if !groupIDValid(model.ID.ValueString()) {
		resp.Diagnostics.AddError("Invalid gateway group UUID", "Lookup requires an exact known UUID.")
		return
	}
	if err := requireGatewayGroup(ctx, d.client); err != nil {
		resp.Diagnostics.AddError("Gateway group unavailable", err.Error())
		return
	}
	remote, err := readGatewayGroup(ctx, d.client, model.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error reading gateway group", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &remote)...)
}
