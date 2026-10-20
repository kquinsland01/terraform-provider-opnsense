// Copyright (c) Matthew Mellor
// SPDX-License-Identifier: MPL-2.0

package wireguard

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	dsschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/matthew-on-git/terraform-provider-opnsense/pkg/opnsense"
)

type (
	serverDataSource      struct{ client *opnsense.Client }
	serverDataSourceModel struct {
		ID            types.String `tfsdk:"id"`
		Enabled       types.Bool   `tfsdk:"enabled"`
		Name          types.String `tfsdk:"name"`
		Instance      types.String `tfsdk:"instance"`
		PublicKey     types.String `tfsdk:"public_key"`
		Port          types.Int64  `tfsdk:"port"`
		MTU           types.Int64  `tfsdk:"mtu"`
		DNS           types.Set    `tfsdk:"dns"`
		TunnelAddress types.Set    `tfsdk:"tunnel_address"`
		Peers         types.Set    `tfsdk:"peers"`
		DisableRoutes types.Bool   `tfsdk:"disable_routes"`
		Gateway       types.String `tfsdk:"gateway"`
		Description   types.String `tfsdk:"description"`
	}
)

func newServerDataSource() datasource.DataSource { return &serverDataSource{} }
func (d *serverDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_wireguard_server"
}

func (d *serverDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = dsschema.Schema{MarkdownDescription: "Reads a core WireGuard server by UUID on OPNsense 24.1+. Secrets are never exposed.", Attributes: map[string]dsschema.Attribute{"id": dsschema.StringAttribute{Required: true, MarkdownDescription: "id."}, "enabled": dsschema.BoolAttribute{Computed: true, MarkdownDescription: "enabled."}, "name": dsschema.StringAttribute{Computed: true, MarkdownDescription: "name."}, "instance": dsschema.StringAttribute{Computed: true, MarkdownDescription: "instance."}, "public_key": dsschema.StringAttribute{Computed: true, MarkdownDescription: "public key."}, "port": dsschema.Int64Attribute{Computed: true, MarkdownDescription: "port."}, "mtu": dsschema.Int64Attribute{Computed: true, MarkdownDescription: "mtu."}, "dns": dsschema.SetAttribute{Computed: true, ElementType: types.StringType, MarkdownDescription: "dns."}, "tunnel_address": dsschema.SetAttribute{Computed: true, ElementType: types.StringType, MarkdownDescription: "tunnel address."}, "peers": dsschema.SetAttribute{Computed: true, ElementType: types.StringType, MarkdownDescription: "peers."}, "disable_routes": dsschema.BoolAttribute{Computed: true, MarkdownDescription: "disable routes."}, "gateway": dsschema.StringAttribute{Computed: true, MarkdownDescription: "gateway."}, "description": dsschema.StringAttribute{Computed: true, MarkdownDescription: "description."}}}
}

func (d *serverDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *serverDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config serverDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}
	result, err := opnsense.Get[wireguardServerAPIResponse](ctx, d.client, serverReqOpts, config.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error reading WireGuard server", err.Error())
		return
	}
	var remote ServerResourceModel
	if err = remote.fromAPI(ctx, result, config.ID.ValueString()); err != nil {
		resp.Diagnostics.AddError("Malformed WireGuard server", err.Error())
		return
	}
	config.ID = remote.ID
	config.Enabled = remote.Enabled
	config.Name = remote.Name
	config.Instance = remote.Instance
	config.PublicKey = remote.PublicKey
	config.Port = remote.Port
	config.MTU = remote.MTU
	config.DNS = remote.DNS
	config.TunnelAddress = remote.TunnelAddress
	config.Peers = remote.Peers
	config.DisableRoutes = remote.DisableRoutes
	config.Gateway = remote.Gateway
	config.Description = remote.Description
	resp.Diagnostics.Append(resp.State.Set(ctx, &config)...)
}
