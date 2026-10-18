// Copyright (c) Matthew Mellor
// SPDX-License-Identifier: MPL-2.0

package firewall

import (
	"context"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
)

type protocolCasePlanModifier struct{}

func (protocolCasePlanModifier) Description(context.Context) string {
	return "preserves API-normalized protocol casing when configuration differs only by case"
}

func (m protocolCasePlanModifier) MarkdownDescription(ctx context.Context) string {
	return m.Description(ctx)
}

func (protocolCasePlanModifier) PlanModifyString(_ context.Context, req planmodifier.StringRequest, resp *planmodifier.StringResponse) {
	if !req.ConfigValue.IsNull() && !req.ConfigValue.IsUnknown() && !req.StateValue.IsNull() && !req.StateValue.IsUnknown() && strings.EqualFold(req.ConfigValue.ValueString(), req.StateValue.ValueString()) {
		resp.PlanValue = req.StateValue
	}
}
