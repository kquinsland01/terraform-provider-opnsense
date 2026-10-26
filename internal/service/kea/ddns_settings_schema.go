// Copyright (c) Matthew Mellor
// SPDX-License-Identifier: MPL-2.0

package kea

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
)

func (r *ddnsSettingsResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{MarkdownDescription: "Manages the existing Kea DDNS general singleton on compatible stable OPNsense 26.7.7+. Owns only four fields. Delete itself is zero-API and state-only. Normal Terraform planning refresh may read, recover or activate pending saves, or fail before Delete.", Attributes: map[string]schema.Attribute{
		"id":            schema.StringAttribute{Computed: true, MarkdownDescription: "Fixed singleton identity ddns.", PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
		"enabled":       schema.BoolAttribute{Required: true, MarkdownDescription: "Explicit DDNS enabled intent; not proof of active DNS updates."},
		"manual_config": schema.BoolAttribute{Optional: true, Computed: true, MarkdownDescription: "Manual configuration flag only; does not manage manual files. Omission preserves appliance/prior values.", PlanModifiers: []planmodifier.Bool{ddnsOmitted{}}},
		"server_ip":     schema.StringAttribute{Optional: true, Computed: true, MarkdownDescription: "Canonical IPv4 address or exact upstream any sentinel. Omission preserves appliance/prior values.", Validators: []validator.String{ddnsAddressValidator{}}, PlanModifiers: []planmodifier.String{ddnsOmitted{}}},
		"server_port":   schema.Int64Attribute{Optional: true, Computed: true, MarkdownDescription: "Server port 1–65535; zero does not clear. Omission preserves appliance/prior values.", Validators: []validator.Int64{int64validator.Between(1, 65535)}, PlanModifiers: []planmodifier.Int64{ddnsOmitted{}}},
	}}
}

type ddnsOmitted struct{}

func (ddnsOmitted) Description(context.Context) string {
	return "Preserve prior values only when configuration is omitted; unresolved inputs remain unknown."
}
func (m ddnsOmitted) MarkdownDescription(ctx context.Context) string { return m.Description(ctx) }
func (ddnsOmitted) PlanModifyString(_ context.Context, req planmodifier.StringRequest, resp *planmodifier.StringResponse) {
	if req.ConfigValue.IsNull() && !req.PlanValue.IsUnknown() && !req.StateValue.IsNull() && !req.StateValue.IsUnknown() {
		resp.PlanValue = req.StateValue
	}
}

func (ddnsOmitted) PlanModifyBool(_ context.Context, req planmodifier.BoolRequest, resp *planmodifier.BoolResponse) {
	if req.ConfigValue.IsNull() && !req.PlanValue.IsUnknown() && !req.StateValue.IsNull() && !req.StateValue.IsUnknown() {
		resp.PlanValue = req.StateValue
	}
}

func (ddnsOmitted) PlanModifyInt64(_ context.Context, req planmodifier.Int64Request, resp *planmodifier.Int64Response) {
	if req.ConfigValue.IsNull() && !req.PlanValue.IsUnknown() && !req.StateValue.IsNull() && !req.StateValue.IsUnknown() {
		resp.PlanValue = req.StateValue
	}
}

type ddnsAddressValidator struct{}

func (ddnsAddressValidator) Description(context.Context) string {
	return "Use canonical IPv4 or the exact any sentinel."
}

func (v ddnsAddressValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (ddnsAddressValidator) ValidateString(_ context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}
	if !ddnsAddressValid(req.ConfigValue.ValueString()) {
		resp.Diagnostics.AddAttributeError(req.Path, "Invalid DDNS server_ip", "Use canonical IPv4 or the exact any sentinel; empty, IPv6, CIDR and list values are not supported.")
	}
}
