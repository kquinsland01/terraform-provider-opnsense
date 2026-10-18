// Copyright (c) Matthew Mellor
// SPDX-License-Identifier: MPL-2.0

package firewall

import (
	"context"
	"sort"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.ResourceWithModifyPlan = &aliasResource{}
	_ resource.ResourceWithModifyPlan = &natPortForwardResource{}
)

func stringSetValue(values []string) types.Set {
	attrs := make([]attr.Value, len(values))
	for i, value := range values {
		attrs[i] = types.StringValue(value)
	}
	return types.SetValueMust(types.StringType, attrs)
}

func setStrings(ctx context.Context, value types.Set) []string {
	if value.IsNull() || value.IsUnknown() {
		return nil
	}
	var values []string
	value.ElementsAs(ctx, &values, false)
	sort.Strings(values)
	return values
}

func firstString(values []string) string {
	if len(values) == 0 {
		return ""
	}
	return values[0]
}

func setContains(ctx context.Context, value types.Set, needle string) bool {
	for _, candidate := range setStrings(ctx, value) {
		if candidate == needle {
			return true
		}
	}
	return false
}

func (r *aliasResource) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	if req.Plan.Raw.IsNull() {
		return
	}
	var config, plan AliasResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var state AliasResourceModel
	hasState := !req.State.Raw.IsNull()
	if hasState {
		resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
		if resp.Diagnostics.HasError() {
			return
		}
	}
	switch {
	case config.IPProtocol.IsUnknown():
		plan.IPProtocol = types.SetUnknown(types.StringType)
	case config.IPProtocol.IsNull() && config.Proto.IsUnknown():
		plan.IPProtocol = types.SetUnknown(types.StringType)
	case config.IPProtocol.IsNull():
		if hasState && !state.IPProtocol.IsNull() && !state.IPProtocol.IsUnknown() && len(state.IPProtocol.Elements()) > 0 {
			stateValues := setStrings(ctx, state.IPProtocol)
			if len(stateValues) == 1 && config.Proto.ValueString() != "" && config.Proto.ValueString() != state.Proto.ValueString() {
				plan.IPProtocol = stringSetValue([]string{config.Proto.ValueString()})
			} else {
				plan.IPProtocol = state.IPProtocol
			}
			if config.Proto.ValueString() == "" {
				plan.Proto = state.Proto
			}
		} else if config.Proto.ValueString() != "" {
			plan.IPProtocol = stringSetValue([]string{config.Proto.ValueString()})
		}
	default:
		if config.Proto.ValueString() == "" {
			plan.Proto = types.StringValue(firstString(setStrings(ctx, config.IPProtocol)))
		}
	}
	resp.Diagnostics.Append(resp.Plan.Set(ctx, &plan)...)
}

func (r *natPortForwardResource) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	if req.Plan.Raw.IsNull() {
		return
	}
	var config, plan NatPortForwardResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var state NatPortForwardResourceModel
	hasState := !req.State.Raw.IsNull()
	if hasState {
		resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
		if resp.Diagnostics.HasError() {
			return
		}
	}
	switch {
	case config.Interfaces.IsUnknown():
		plan.Interfaces = types.SetUnknown(types.StringType)
	case config.Interfaces.IsNull():
		if hasState && !state.Interfaces.IsNull() && !state.Interfaces.IsUnknown() && len(state.Interfaces.Elements()) > 0 {
			stateValues := setStrings(ctx, state.Interfaces)
			if len(stateValues) == 1 && !config.Interface.IsNull() && config.Interface.ValueString() != "" && config.Interface.ValueString() != state.Interface.ValueString() {
				plan.Interfaces = stringSetValue([]string{config.Interface.ValueString()})
			} else {
				plan.Interfaces = state.Interfaces
			}
			if config.Interface.IsNull() || config.Interface.ValueString() == "" {
				plan.Interface = state.Interface
			}
		} else if !config.Interface.IsNull() && config.Interface.ValueString() != "" {
			plan.Interfaces = stringSetValue([]string{config.Interface.ValueString()})
		}
	default:
		if config.Interface.IsNull() || config.Interface.ValueString() == "" {
			plan.Interface = types.StringValue(firstString(setStrings(ctx, config.Interfaces)))
		}
	}
	resp.Diagnostics.Append(resp.Plan.Set(ctx, &plan)...)
}
