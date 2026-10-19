// Copyright (c) Matthew Mellor
// SPDX-License-Identifier: MPL-2.0

package unbound

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type domainOverridePortDefaultPlanModifier struct{}

func (domainOverridePortDefaultPlanModifier) Description(context.Context) string {
	return "defaults an omitted port to 53 for query forwarding and 853 for DNS-over-TLS"
}

func (m domainOverridePortDefaultPlanModifier) MarkdownDescription(ctx context.Context) string {
	return m.Description(ctx)
}

func (domainOverridePortDefaultPlanModifier) PlanModifyInt64(ctx context.Context, req planmodifier.Int64Request, resp *planmodifier.Int64Response) {
	if !req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}
	var forwardType types.String
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("type"), &forwardType)...)
	if resp.Diagnostics.HasError() || forwardType.IsUnknown() {
		return
	}
	if !forwardType.IsNull() && forwardType.ValueString() == "dot" {
		resp.PlanValue = types.Int64Value(853)
		return
	}
	resp.PlanValue = types.Int64Value(53)
}
