// Copyright (c) Matthew Mellor
// SPDX-License-Identifier: MPL-2.0

package openvpn

import (
	"context"
	"sort"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
)

var _ resource.ResourceWithConfigValidators = &instanceResource{}

type instanceConfigValidator struct{}

func (instanceConfigValidator) Description(context.Context) string {
	return "validates OpenVPN credentials and additive ordered ciphers"
}

func (v instanceConfigValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (instanceConfigValidator) ValidateResource(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var model InstanceResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &model)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if !model.Username.IsNull() && !model.Username.IsUnknown() && model.Username.ValueString() != "" && (model.Password.IsNull() || model.Password.IsUnknown()) {
		resp.Diagnostics.AddAttributeError(path.Root("password"), "Password required with username", "A known non-empty username requires a known password; null or unknown must not be encoded as an empty secret.")
	}
	if model.DataCiphers.IsNull() || model.DataCiphers.IsUnknown() || model.DataCiphersOrdered.IsNull() || model.DataCiphersOrdered.IsUnknown() {
		return
	}
	var setItems, orderedItems []string
	setDiags := model.DataCiphers.ElementsAs(ctx, &setItems, false)
	orderedDiags := model.DataCiphersOrdered.ElementsAs(ctx, &orderedItems, false)
	resp.Diagnostics.Append(setDiags...)
	resp.Diagnostics.Append(orderedDiags...)
	if setDiags.HasError() || orderedDiags.HasError() {
		return
	}
	sort.Strings(setItems)
	sortedOrdered := append([]string(nil), orderedItems...)
	sort.Strings(sortedOrdered)
	if strings.Join(setItems, "\x00") != strings.Join(sortedOrdered, "\x00") {
		resp.Diagnostics.AddAttributeError(path.Root("data_ciphers_ordered"), "Conflicting cipher attributes", "data_ciphers and data_ciphers_ordered must contain the same values when both are configured.")
	}
}

func (r *instanceResource) ConfigValidators(context.Context) []resource.ConfigValidator {
	return []resource.ConfigValidator{instanceConfigValidator{}}
}
