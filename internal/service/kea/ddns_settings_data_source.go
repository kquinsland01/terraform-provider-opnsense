// Copyright (c) Matthew Mellor
// SPDX-License-Identifier: MPL-2.0

package kea

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/matthew-on-git/terraform-provider-opnsense/pkg/opnsense"
)

type ddnsSettingsDataSource struct{ client *opnsense.Client }

func newDDNSSettingsDataSource() datasource.DataSource { return &ddnsSettingsDataSource{} }
func (d *ddnsSettingsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_kea_ddns_settings"
}

func (d *ddnsSettingsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *ddnsSettingsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{MarkdownDescription: "Reads all four Kea DDNS general settings on compatible stable OPNsense 26.7.7+. Never mutates or activates configuration.", Attributes: map[string]schema.Attribute{
		"id":            schema.StringAttribute{Computed: true, MarkdownDescription: "Fixed singleton identity ddns."},
		"enabled":       schema.BoolAttribute{Computed: true, MarkdownDescription: "Stored enabled flag."},
		"manual_config": schema.BoolAttribute{Computed: true, MarkdownDescription: "Stored manual configuration flag; no file ownership."},
		"server_ip":     schema.StringAttribute{Computed: true, MarkdownDescription: "Canonical IPv4 address or exact any sentinel."},
		"server_port":   schema.Int64Attribute{Computed: true, MarkdownDescription: "Server port from 1 through 65535."},
	}}
}

func (d *ddnsSettingsDataSource) Read(ctx context.Context, _ datasource.ReadRequest, resp *datasource.ReadResponse) {
	if err := requireDDNS(ctx, d.client); err != nil {
		resp.Diagnostics.AddError("DDNS unavailable", err.Error())
		return
	}
	remote, err := readDDNS(ctx, d.client)
	if err != nil {
		resp.Diagnostics.AddError("Error reading DDNS settings", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &remote)...)
}
