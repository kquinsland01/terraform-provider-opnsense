// Copyright (c) Matthew Mellor
// SPDX-License-Identifier: MPL-2.0

package openvpn

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	dsschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"

	"github.com/matthew-on-git/terraform-provider-opnsense/pkg/opnsense"
)

var _ datasource.DataSource = &staticKeyDataSource{}

type staticKeyDataSource struct{ client *opnsense.Client }

func newStaticKeyDataSource() datasource.DataSource { return &staticKeyDataSource{} }
func (d *staticKeyDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_openvpn_static_key"
}

func (d *staticKeyDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = dsschema.Schema{MarkdownDescription: "Reads an OpenVPN static key by UUID. The sensitive key value is stored in Terraform state; use the ephemeral generator when persistence is not acceptable.", Attributes: map[string]dsschema.Attribute{"id": dsschema.StringAttribute{Required: true, MarkdownDescription: "Static-key UUID."}, "mode": dsschema.StringAttribute{Computed: true, MarkdownDescription: "Key usage mode."}, "description": dsschema.StringAttribute{Computed: true, MarkdownDescription: "Description."}, "key": dsschema.StringAttribute{Computed: true, Sensitive: true, MarkdownDescription: "Static key material. Sensitive values from data sources remain in Terraform state."}}}
}

func (d *staticKeyDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *staticKeyDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data StaticKeyResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	id := data.ID.ValueString()
	if _, err := uuid.Parse(id); err != nil {
		resp.Diagnostics.AddError("Invalid OpenVPN static key UUID", fmt.Sprintf("id must be a non-empty UUID: %s", err))
		return
	}
	if err := requireOpenVPNParityVersion(ctx, d.client); err != nil {
		resp.Diagnostics.AddError("Unsupported OpenVPN parity surface", err.Error())
		return
	}
	lookupOpts := staticKeyReqOpts
	lookupOpts.StrictMissingObjectResponse = true
	result, err := opnsense.Get[staticKeyAPIResponse](ctx, d.client, lookupOpts, id)
	if err != nil {
		resp.Diagnostics.AddError("Error reading OpenVPN static key", fmt.Sprintf("Could not read OpenVPN static key %s: %s", id, err))
		return
	}
	if strings.TrimSpace(result.Key) == "" {
		resp.Diagnostics.AddError("Malformed OpenVPN static key response", "The appliance returned an empty key.")
		return
	}
	if strings.TrimSpace(string(result.Mode)) == "" {
		resp.Diagnostics.AddError("Malformed OpenVPN static key response", "The appliance returned an empty mode.")
		return
	}
	data.fromAPI(ctx, result, id)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
