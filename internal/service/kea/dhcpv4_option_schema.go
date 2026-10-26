// Copyright (c) Matthew Mellor
// SPDX-License-Identifier: MPL-2.0

package kea

import (
	"context"
	"regexp"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func (r *dhcpv4OptionResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	attrs := map[string]schema.Attribute{
		"id":          schema.StringAttribute{Computed: true, MarkdownDescription: "Option UUID.", PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
		"code":        schema.StringAttribute{Required: true, MarkdownDescription: "Scalar DHCPv4 option selector key (1–254), checked against the appliance model.", Validators: []validator.String{stringvalidator.RegexMatches(regexp.MustCompile(`^([1-9]|[1-9][0-9]|1[0-9][0-9]|2[0-4][0-9]|25[0-4])$`), "Use an actual DHCPv4 selector key from 1 through 254.")}},
		"data":        schema.StringAttribute{Required: true, MarkdownDescription: "Literal nonempty option data; semantic validation is performed by OPNsense.", Validators: []validator.String{stringvalidator.LengthAtLeast(1)}},
		"description": schema.StringAttribute{Required: true, MarkdownDescription: "Mutable, nonempty unique description.", Validators: []validator.String{stringvalidator.LengthAtLeast(1)}},
		"force":       schema.BoolAttribute{Optional: true, Computed: true, MarkdownDescription: "Stored force flag; defaults to false on create. Omission preserves prior/imported values. Not a packet-delivery guarantee.", PlanModifiers: []planmodifier.Bool{optionOmittedBool{}}},
	}
	for key, desc := range map[string]string{"encoding": "Encoding selector key. Defaults to hex on create.", "match_code": "Matching code selector key. Defaults to empty on create.", "match_encoding": "Matching encoding selector key. Encoding alone with empty matching code/data is supported.", "match_data": "Literal matching data. Code/data must both be populated or both empty; data requires matching encoding."} {
		initial := ""
		if key == "encoding" {
			initial = "hex"
		}
		var validators []validator.String
		if key == "encoding" {
			validators = []validator.String{stringvalidator.OneOf(optionEncodings...)}
		}
		if key == "match_encoding" {
			validators = []validator.String{stringvalidator.OneOf(append([]string{""}, optionEncodings...)...)}
		}
		attrs[key] = schema.StringAttribute{Validators: validators, Optional: true, Computed: true, MarkdownDescription: desc + " Omission preserves prior/imported values; explicit empty clears supported matching fields.", PlanModifiers: []planmodifier.String{optionOmittedString{initial: initial}}}
	}
	resp.Schema = schema.Schema{MarkdownDescription: "Manages an independent generic Kea DHCPv4 option on stable OPNsense 26.7.7+ with the compatible eight-field model. Does not manage ordered subnet/reservation linkage or enable DHCP.", Attributes: attrs}
}

// Only absent configuration inherits state/defaults. Explicit unknown inputs
// must remain unknown, even when a previous value exists.
type optionOmittedString struct{ initial string }

func (m optionOmittedString) Description(context.Context) string {
	return "Preserve omitted values, with create-only defaults."
}

func (m optionOmittedString) MarkdownDescription(ctx context.Context) string {
	return m.Description(ctx)
}

func (m optionOmittedString) PlanModifyString(_ context.Context, req planmodifier.StringRequest, resp *planmodifier.StringResponse) {
	if !req.ConfigValue.IsNull() {
		return
	}
	if !req.StateValue.IsNull() && !req.StateValue.IsUnknown() {
		resp.PlanValue = req.StateValue
		return
	}
	resp.PlanValue = types.StringValue(m.initial)
}

type optionOmittedBool struct{}

func (m optionOmittedBool) Description(context.Context) string {
	return "Preserve omitted values, defaulting to false only on create."
}

func (m optionOmittedBool) MarkdownDescription(ctx context.Context) string { return m.Description(ctx) }

func (m optionOmittedBool) PlanModifyBool(_ context.Context, req planmodifier.BoolRequest, resp *planmodifier.BoolResponse) {
	if !req.ConfigValue.IsNull() {
		return
	}
	if !req.StateValue.IsNull() && !req.StateValue.IsUnknown() {
		resp.PlanValue = req.StateValue
		return
	}
	resp.PlanValue = types.BoolValue(false)
}
