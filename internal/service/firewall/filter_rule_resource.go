// Copyright (c) Matthew Mellor
// SPDX-License-Identifier: MPL-2.0

package firewall

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/matthew-on-git/terraform-provider-opnsense/pkg/opnsense"
)

// Ensure filterRuleResource satisfies the resource interfaces.
var (
	_ resource.Resource                = &filterRuleResource{}
	_ resource.ResourceWithImportState = &filterRuleResource{}
)

// filterRuleResource implements the opnsense_firewall_filter_rule resource.
// Uses instance-level reqOpts because ReconfigureFunc needs the client.
type filterRuleResource struct {
	client  *opnsense.Client
	reqOpts opnsense.ReqOpts
}

func newFilterRuleResource() resource.Resource {
	return &filterRuleResource{}
}

// Metadata sets the resource type name.
func (r *filterRuleResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_firewall_filter_rule"
}

// Configure extracts the OPNsense API client and sets up ReqOpts with the
// single direct filter apply function. It intentionally accepts any HTTP 200
// response body and never calls the removed savepoint/cancel flow.
func (r *filterRuleResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	client, ok := req.ProviderData.(*opnsense.Client)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Provider Data",
			"Expected *opnsense.Client, got something else.",
		)
		return
	}
	r.client = client
	r.reqOpts = opnsense.ReqOpts{
		AddEndpoint:     "/api/firewall/filter/addRule",
		GetEndpoint:     "/api/firewall/filter/getRule",
		UpdateEndpoint:  "/api/firewall/filter/setRule",
		DeleteEndpoint:  "/api/firewall/filter/delRule",
		SearchEndpoint:  "/api/firewall/filter/searchRule",
		ReconfigureFunc: opnsense.FirewallFilterReconfigure(r.client),
		Monad:           "rule",
	}
}

// Create creates a new firewall filter rule via the OPNsense API.
func (r *filterRuleResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan FilterRuleResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiReq := plan.toAPI(ctx)
	plannedProtocol := plan.Protocol

	uuid, err := opnsense.Add(ctx, r.client, r.reqOpts, apiReq)
	if err != nil {
		if uuid != "" {
			recovered := false
			if result, readErr := opnsense.Get[filterRuleAPIResponse](ctx, r.client, r.reqOpts, uuid); readErr == nil {
				apiDiags := validateFilterAPIResponse(result)
				resp.Diagnostics.Append(apiDiags...)
				if !apiDiags.HasError() {
					plan.fromAPI(ctx, result, uuid)
					plan.Protocol = filterOperationProtocol(plannedProtocol, plan.Protocol)
					resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
					recovered = true
				}
			}
			if !recovered {
				identity := FilterRuleResourceModel{ID: types.StringValue(uuid), Interface: types.SetNull(types.StringType), ICMPType: types.SetNull(types.StringType), TCPFlags: types.SetNull(types.StringType), TCPFlagsOutOf: types.SetNull(types.StringType), Categories: types.SetNull(types.StringType)}
				resp.Diagnostics.Append(resp.State.Set(ctx, &identity)...)
			}
		}
		resp.Diagnostics.AddError(
			"Error creating firewall filter rule",
			fmt.Sprintf("Could not create firewall filter rule: %s", err),
		)
		return
	}

	result, err := opnsense.Get[filterRuleAPIResponse](ctx, r.client, r.reqOpts, uuid)
	if err != nil {
		identity := FilterRuleResourceModel{ID: types.StringValue(uuid), Interface: types.SetNull(types.StringType), ICMPType: types.SetNull(types.StringType), TCPFlags: types.SetNull(types.StringType), TCPFlagsOutOf: types.SetNull(types.StringType), Categories: types.SetNull(types.StringType)}
		resp.Diagnostics.Append(resp.State.Set(ctx, &identity)...)
		resp.Diagnostics.AddError(
			"Error reading firewall filter rule after create",
			fmt.Sprintf("Created rule %s but could not read it back: %s", uuid, err),
		)
		return
	}
	apiDiags := validateFilterAPIResponse(result)
	if apiDiags.HasError() {
		identity := FilterRuleResourceModel{ID: types.StringValue(uuid), Interface: types.SetNull(types.StringType), ICMPType: types.SetNull(types.StringType), TCPFlags: types.SetNull(types.StringType), TCPFlagsOutOf: types.SetNull(types.StringType), Categories: types.SetNull(types.StringType)}
		resp.Diagnostics.Append(resp.State.Set(ctx, &identity)...)
		resp.Diagnostics.Append(apiDiags...)
		return
	}

	plan.fromAPI(ctx, result, uuid)
	plan.Protocol = filterOperationProtocol(plannedProtocol, plan.Protocol)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Read refreshes the Terraform state from the OPNsense API.
func (r *filterRuleResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state FilterRuleResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	result, err := opnsense.Get[filterRuleAPIResponse](ctx, r.client, r.reqOpts, state.ID.ValueString())
	if err != nil {
		var notFoundErr *opnsense.NotFoundError
		if errors.As(err, &notFoundErr) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError(
			"Error reading firewall filter rule",
			fmt.Sprintf("Could not read firewall filter rule %s: %s", state.ID.ValueString(), err),
		)
		return
	}
	resp.Diagnostics.Append(validateFilterAPIResponse(result)...)
	if resp.Diagnostics.HasError() {
		return
	}

	state.fromAPI(ctx, result, state.ID.ValueString())
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// Update modifies an existing firewall filter rule via the OPNsense API.
func (r *filterRuleResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan FilterRuleResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var state FilterRuleResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiReq := plan.toAPI(ctx)
	plannedProtocol := plan.Protocol
	id := state.ID.ValueString()

	err := opnsense.Update(ctx, r.client, r.reqOpts, apiReq, id)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error updating firewall filter rule",
			fmt.Sprintf("Could not update firewall filter rule %s: %s", id, err),
		)
		return
	}
	result, err := opnsense.Get[filterRuleAPIResponse](ctx, r.client, r.reqOpts, id)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error reading firewall filter rule after update",
			fmt.Sprintf("Updated rule %s but could not read it back: %s", id, err),
		)
		return
	}
	resp.Diagnostics.Append(validateFilterAPIResponse(result)...)
	if resp.Diagnostics.HasError() {
		return
	}

	plan.fromAPI(ctx, result, id)
	plan.Protocol = filterOperationProtocol(plannedProtocol, plan.Protocol)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func filterOperationProtocol(planned, read types.String) types.String {
	if planned.IsNull() || planned.IsUnknown() || read.IsNull() || read.IsUnknown() {
		return read
	}
	if strings.EqualFold(planned.ValueString(), read.ValueString()) {
		return planned
	}
	return read
}

// Delete removes a firewall filter rule from the OPNsense API.
func (r *filterRuleResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state FilterRuleResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := opnsense.Delete(ctx, r.client, r.reqOpts, state.ID.ValueString())
	if err != nil {
		var notFoundErr *opnsense.NotFoundError
		if errors.As(err, &notFoundErr) {
			return
		}
		resp.Diagnostics.AddError(
			"Error deleting firewall filter rule",
			fmt.Sprintf("Could not delete firewall filter rule %s: %s", state.ID.ValueString(), err),
		)
	}
}

// ImportState imports an existing firewall filter rule by UUID.
func (r *filterRuleResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
