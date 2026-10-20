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
	peerDataSource      struct{ client *opnsense.Client }
	peerDataSourceModel struct {
		ID            types.String `tfsdk:"id"`
		Enabled       types.Bool   `tfsdk:"enabled"`
		Name          types.String `tfsdk:"name"`
		PublicKey     types.String `tfsdk:"public_key"`
		TunnelAddress types.Set    `tfsdk:"tunnel_address"`
		ServerAddress types.String `tfsdk:"server_address"`
		ServerPort    types.Int64  `tfsdk:"server_port"`
		Keepalive     types.Int64  `tfsdk:"keepalive"`
		Servers       types.Set    `tfsdk:"servers"`
	}
)

func newPeerDataSource() datasource.DataSource { return &peerDataSource{} }
func (d *peerDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_wireguard_peer"
}

func (d *peerDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = dsschema.Schema{MarkdownDescription: "Reads a core WireGuard peer by UUID on OPNsense 24.1+. Secrets are never exposed.", Attributes: map[string]dsschema.Attribute{"id": dsschema.StringAttribute{Required: true, MarkdownDescription: "id."}, "enabled": dsschema.BoolAttribute{Computed: true, MarkdownDescription: "enabled."}, "name": dsschema.StringAttribute{Computed: true, MarkdownDescription: "name."}, "public_key": dsschema.StringAttribute{Computed: true, MarkdownDescription: "public key."}, "tunnel_address": dsschema.SetAttribute{Computed: true, ElementType: types.StringType, MarkdownDescription: "tunnel address."}, "server_address": dsschema.StringAttribute{Computed: true, MarkdownDescription: "server address."}, "server_port": dsschema.Int64Attribute{Computed: true, MarkdownDescription: "server port."}, "keepalive": dsschema.Int64Attribute{Computed: true, MarkdownDescription: "keepalive."}, "servers": dsschema.SetAttribute{Computed: true, ElementType: types.StringType, MarkdownDescription: "servers."}}}
}

func (d *peerDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *peerDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config peerDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}
	result, err := opnsense.Get[wireguardPeerAPIResponse](ctx, d.client, peerReqOpts, config.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error reading WireGuard peer", err.Error())
		return
	}
	var remote PeerResourceModel
	if err = remote.fromAPI(ctx, result, config.ID.ValueString()); err != nil {
		resp.Diagnostics.AddError("Malformed WireGuard peer", err.Error())
		return
	}
	config.ID = remote.ID
	config.Enabled = remote.Enabled
	config.Name = remote.Name
	config.PublicKey = remote.PublicKey
	config.TunnelAddress = remote.TunnelAddress
	config.ServerAddress = remote.ServerAddress
	config.ServerPort = remote.ServerPort
	config.Keepalive = remote.Keepalive
	config.Servers = remote.Servers
	resp.Diagnostics.Append(resp.State.Set(ctx, &config)...)
}
