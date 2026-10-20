// Copyright (c) Matthew Mellor
// SPDX-License-Identifier: MPL-2.0

package wireguard

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/setdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/matthew-on-git/terraform-provider-opnsense/internal/tfconv"
)

func (r *serverResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = serverSchemaV1()
}

func serverSchemaV1() schema.Schema {
	emptySet := setdefault.StaticValue(tfconv.SliceToSet(nil))
	return schema.Schema{
		Version:             1,
		MarkdownDescription: "Manages a WireGuard local server on OPNsense (core since 24.1; acceptance-validated on 26.7.7). This is the sole writable owner of peer associations through `peers`.",
		Attributes: map[string]schema.Attribute{
			"id":             schema.StringAttribute{Computed: true, MarkdownDescription: "UUID of the WireGuard server.", PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"enabled":        schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(true), MarkdownDescription: "Whether this server is enabled."},
			"name":           schema.StringAttribute{Required: true, MarkdownDescription: "Name of the WireGuard server."},
			"instance":       schema.StringAttribute{Computed: true, MarkdownDescription: "API-assigned WireGuard interface instance.", PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"public_key":     schema.StringAttribute{Optional: true, Computed: true, MarkdownDescription: "Base64-encoded public key returned by OPNsense. Omission follows API truth, which may be blank; the add API does not derive a public key from private_key.", PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"private_key":    schema.StringAttribute{Required: true, Sensitive: true, MarkdownDescription: "Write-only base64-encoded private key.", PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"port":           schema.Int64Attribute{Optional: true, Computed: true, MarkdownDescription: "Listen port. Null lets OPNsense choose its default.", Validators: []validator.Int64{int64validator.Between(1, 65535)}},
			"mtu":            schema.Int64Attribute{Optional: true, Computed: true, MarkdownDescription: "Interface MTU. Null uses the parent interface MTU.", Validators: []validator.Int64{int64validator.Between(1, 9300)}},
			"dns":            schema.SetAttribute{ElementType: types.StringType, Optional: true, Computed: true, Default: emptySet, MarkdownDescription: "Interface-specific DNS server IP addresses."},
			"tunnel_address": schema.SetAttribute{ElementType: types.StringType, Optional: true, Computed: true, Default: emptySet, MarkdownDescription: "Tunnel interface CIDRs."},
			"peers":          schema.SetAttribute{ElementType: types.StringType, Optional: true, Computed: true, Default: emptySet, MarkdownDescription: "Peer UUIDs associated with this server. Do not independently manage the peer backreference."},
			"disable_routes": schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(false), MarkdownDescription: "Suppress automatic route installation."},
			"gateway":        schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString(""), MarkdownDescription: "Gateway IP used when disable_routes is true."},
			"description":    schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString(""), MarkdownDescription: "Legacy state-only description. The released core API does not persist this field."},
		},
	}
}
