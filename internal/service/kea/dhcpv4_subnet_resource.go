// Copyright (c) Matthew Mellor
// SPDX-License-Identifier: MPL-2.0

package kea

import (
	"context"
	"errors"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/matthew-on-git/terraform-provider-opnsense/pkg/opnsense"
)

// Ensure dhcpv4SubnetResource satisfies the resource interfaces.
var (
	_ resource.Resource                = &dhcpv4SubnetResource{}
	_ resource.ResourceWithImportState = &dhcpv4SubnetResource{}
)

// dhcpv4SubnetReqOpts configures the OPNsense API endpoints for Kea DHCPv4 subnets.
var dhcpv4SubnetReqOpts = opnsense.ReqOpts{
	AddEndpoint:                 "/api/kea/dhcpv4/add_subnet",
	GetEndpoint:                 "/api/kea/dhcpv4/get_subnet",
	UpdateEndpoint:              "/api/kea/dhcpv4/set_subnet",
	DeleteEndpoint:              "/api/kea/dhcpv4/del_subnet",
	StrictDeleteResponse:        true,
	StrictMissingObjectResponse: true,
	SearchEndpoint:              "/api/kea/dhcpv4/search_subnet",
	ReconfigureEndpoint:         "/api/kea/service/reconfigure",
	Monad:                       "subnet4",
}

// dhcpv4SubnetResource implements the opnsense_kea_dhcpv4_subnet resource.
type dhcpv4SubnetResource struct {
	client *opnsense.Client
}

func newDHCPv4SubnetResource() resource.Resource {
	return &dhcpv4SubnetResource{}
}

// Metadata sets the resource type name.
func (r *dhcpv4SubnetResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_kea_dhcpv4_subnet"
}

// Configure extracts the OPNsense API client from provider data.
func (r *dhcpv4SubnetResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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
}

// Create creates a new Kea DHCPv4 subnet via the OPNsense API.
func (r *dhcpv4SubnetResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan DHCPv4SubnetResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiReq := plan.toAPI(ctx)

	uuid, err := opnsense.Add(ctx, r.client, dhcpv4SubnetReqOpts, apiReq)
	if err != nil {
		var reconfigureErr *opnsense.MutationReconfigureError
		if errors.As(err, &reconfigureErr) {
			resp.Diagnostics.Append(setKeaPendingActivation(ctx, resp.Private, keaPendingMutation)...)
		}
		if uuid != "" {
			recovered := dhcpv4SubnetIdentity(uuid)
			if result, readErr := opnsense.Get[dhcpv4SubnetAPIResponse](ctx, r.client, dhcpv4SubnetReqOpts, uuid); readErr == nil {
				if plan.fromAPI(ctx, result, uuid) == nil {
					recovered = plan
				}
			}
			resp.Diagnostics.Append(resp.State.Set(ctx, &recovered)...)
		}
		resp.Diagnostics.AddError(
			"Error creating Kea DHCPv4 subnet",
			fmt.Sprintf("Could not create Kea DHCPv4 subnet: %s", err),
		)
		return
	}

	result, err := opnsense.Get[dhcpv4SubnetAPIResponse](ctx, r.client, dhcpv4SubnetReqOpts, uuid)
	if err != nil {
		recovered := dhcpv4SubnetIdentity(uuid)
		resp.Diagnostics.Append(resp.State.Set(ctx, &recovered)...)
		resp.Diagnostics.AddError(
			"Error reading Kea DHCPv4 subnet after create",
			fmt.Sprintf("Created subnet %s but could not read it back: %s", uuid, err),
		)
		return
	}

	if err := plan.fromAPI(ctx, result, uuid); err != nil {
		recovered := dhcpv4SubnetIdentity(uuid)
		resp.Diagnostics.Append(resp.State.Set(ctx, &recovered)...)
		resp.Diagnostics.AddError("Error decoding Kea DHCPv4 subnet after create", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func dhcpv4SubnetIdentity(uuid string) DHCPv4SubnetResourceModel {
	return DHCPv4SubnetResourceModel{
		ID: types.StringValue(uuid), DNSServers: types.SetNull(types.StringType), DomainSearch: types.SetNull(types.StringType),
		Routers: types.SetNull(types.StringType), StaticRoutes: types.SetNull(types.ObjectType{AttrTypes: staticRouteAttrTypes}),
		NTPServers: types.SetNull(types.StringType), TimeServers: types.SetNull(types.StringType),
	}
}

// Read refreshes the Terraform state from the OPNsense API.
func (r *dhcpv4SubnetResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state DHCPv4SubnetResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	pending, privateDiags := getKeaPendingActivation(ctx, req.Private)
	resp.Diagnostics.Append(privateDiags...)
	if resp.Diagnostics.HasError() {
		return
	}
	if pending != "" {
		if err := retryKeaPendingActivation(ctx, r.client, dhcpv4SubnetReqOpts, pending); err != nil {
			resp.Diagnostics.AddError("Error activating pending Kea mutation", err.Error())
			return
		}
		resp.Diagnostics.Append(clearKeaPendingActivation(ctx, resp.Private)...)
		if resp.Diagnostics.HasError() {
			return
		}
	}

	result, err := opnsense.Get[dhcpv4SubnetAPIResponse](ctx, r.client, dhcpv4SubnetReqOpts, state.ID.ValueString())
	if err != nil {
		var notFoundErr *opnsense.NotFoundError
		if errors.As(err, &notFoundErr) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError(
			"Error reading Kea DHCPv4 subnet",
			fmt.Sprintf("Could not read Kea DHCPv4 subnet %s: %s", state.ID.ValueString(), err),
		)
		return
	}

	if err := state.fromAPI(ctx, result, state.ID.ValueString()); err != nil {
		resp.Diagnostics.AddError("Error decoding Kea DHCPv4 subnet", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// Update modifies an existing Kea DHCPv4 subnet via the OPNsense API.
func (r *dhcpv4SubnetResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan DHCPv4SubnetResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var state DHCPv4SubnetResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	pending, privateDiags := getKeaPendingActivation(ctx, req.Private)
	resp.Diagnostics.Append(privateDiags...)
	if resp.Diagnostics.HasError() {
		return
	}
	if pending != "" {
		if err := retryKeaPendingActivation(ctx, r.client, dhcpv4SubnetReqOpts, pending); err != nil {
			resp.Diagnostics.AddError("Error activating pending Kea mutation", err.Error())
			return
		}
		resp.Diagnostics.Append(clearKeaPendingActivation(ctx, resp.Private)...)
		if resp.Diagnostics.HasError() {
			return
		}
	}

	apiReq := plan.toAPI(ctx)
	id := state.ID.ValueString()

	err := opnsense.Update(ctx, r.client, dhcpv4SubnetReqOpts, apiReq, id)
	if err != nil {
		var reconfigureErr *opnsense.MutationReconfigureError
		if errors.As(err, &reconfigureErr) {
			resp.Diagnostics.Append(setKeaPendingActivation(ctx, resp.Private, keaPendingMutation)...)
			recovered := dhcpv4SubnetIdentity(id)
			if result, readErr := opnsense.Get[dhcpv4SubnetAPIResponse](ctx, r.client, dhcpv4SubnetReqOpts, id); readErr == nil && plan.fromAPI(ctx, result, id) == nil {
				recovered = plan
			}
			resp.Diagnostics.Append(resp.State.Set(ctx, &recovered)...)
		}
		resp.Diagnostics.AddError(
			"Error updating Kea DHCPv4 subnet",
			fmt.Sprintf("Could not update Kea DHCPv4 subnet %s: %s", id, err),
		)
		return
	}

	result, err := opnsense.Get[dhcpv4SubnetAPIResponse](ctx, r.client, dhcpv4SubnetReqOpts, id)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error reading Kea DHCPv4 subnet after update",
			fmt.Sprintf("Updated subnet %s but could not read it back: %s", id, err),
		)
		return
	}

	if err := plan.fromAPI(ctx, result, id); err != nil {
		resp.Diagnostics.AddError("Error decoding Kea DHCPv4 subnet after update", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Delete removes a Kea DHCPv4 subnet from the OPNsense API.
func (r *dhcpv4SubnetResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state DHCPv4SubnetResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	pending, privateDiags := getKeaPendingActivation(ctx, req.Private)
	resp.Diagnostics.Append(privateDiags...)
	if resp.Diagnostics.HasError() {
		return
	}
	if pending != "" {
		if err := retryKeaPendingActivation(ctx, r.client, dhcpv4SubnetReqOpts, pending); err != nil {
			resp.Diagnostics.AddError("Error activating pending Kea mutation", err.Error())
			return
		}
		resp.Diagnostics.Append(clearKeaPendingActivation(ctx, resp.Private)...)
		if resp.Diagnostics.HasError() {
			return
		}
		if pending == keaPendingDelete {
			return
		}
	}

	err := opnsense.Delete(ctx, r.client, dhcpv4SubnetReqOpts, state.ID.ValueString())
	if err != nil {
		var notFoundErr *opnsense.NotFoundError
		if errors.As(err, &notFoundErr) {
			return
		}
		var reconfigureErr *opnsense.MutationReconfigureError
		if errors.As(err, &reconfigureErr) {
			resp.Diagnostics.Append(setKeaPendingActivation(ctx, resp.Private, keaPendingDelete)...)
		}
		resp.Diagnostics.AddError(
			"Error deleting Kea DHCPv4 subnet",
			fmt.Sprintf("Could not delete Kea DHCPv4 subnet %s: %s", state.ID.ValueString(), err),
		)
	}
}

// ImportState imports an existing Kea DHCPv4 subnet by UUID.
func (r *dhcpv4SubnetResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
