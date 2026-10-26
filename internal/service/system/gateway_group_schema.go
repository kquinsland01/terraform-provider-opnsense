// Copyright (c) Matthew Mellor
// SPDX-License-Identifier: MPL-2.0

package system

import (
	"context"
	"fmt"
	"regexp"

	"github.com/hashicorp/terraform-plugin-framework-validators/setvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/setdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func (r *gatewayGroupResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	attrs := map[string]schema.Attribute{
		"id":           schema.StringAttribute{Computed: true, MarkdownDescription: "Gateway group UUID.", PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
		"name":         schema.StringAttribute{Required: true, MarkdownDescription: "Immutable unique gateway group name.", Validators: []validator.String{stringvalidator.RegexMatches(groupNamePattern, "Use 1–32 ASCII letters, digits, underscores or hyphens.")}, PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}},
		"trigger":      schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString("down"), MarkdownDescription: "Failure trigger.", Validators: []validator.String{stringvalidator.OneOf(groupTriggers...)}},
		"pool_options": schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString(""), MarkdownDescription: "Pool selection; empty means appliance default.", Validators: []validator.String{stringvalidator.OneOf(groupPools...)}},
		"description":  schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString(""), MarkdownDescription: "Description; empty clears it."},
	}
	for i := 1; i <= 5; i++ {
		description := "Gateway names selected at this tier. Empty clears this tier."
		if i == 1 {
			description = "Gateway names selected at tier 1. May be empty initially when another tier is populated. The stable API cannot clear a populated first tier; such updates fail before any fields are saved."
		}
		attrs[fmt.Sprintf("tier_%d", i)] = schema.SetAttribute{Optional: true, Computed: true, ElementType: types.StringType, Validators: []validator.Set{setvalidator.ValueStringsAre(stringvalidator.RegexMatches(regexp.MustCompile(`^[^,\s]+$`), "Gateway names must be nonempty and contain no commas or whitespace."))}, Default: setdefault.StaticValue(types.SetValueMust(types.StringType, nil)), MarkdownDescription: description}
	}
	resp.Schema = schema.Schema{MarkdownDescription: "Manages a gateway group on stable OPNsense 26.7.7 or newer. Activates interface routes only; traffic failover is unverified.", Attributes: attrs}
}

// ValidateConfig rejects known empty unions while allowing unresolved expressions
// to be planned. Apply performs the same validation again before any write.
func (r *gatewayGroupResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var config GatewayGroupResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}
	count := 0
	for _, tier := range []types.Set{config.Tier1, config.Tier2, config.Tier3, config.Tier4, config.Tier5} {
		if tier.IsUnknown() {
			return
		}
		count += len(tier.Elements())
	}
	if count == 0 {
		resp.Diagnostics.AddError("Empty gateway group", "At least one gateway must be selected across tier_1 through tier_5.")
	}
}
