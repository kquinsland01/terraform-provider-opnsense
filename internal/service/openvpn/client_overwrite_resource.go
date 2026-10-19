// Copyright (c) Matthew Mellor
// SPDX-License-Identifier: MPL-2.0

package openvpn

import (
	"context"
	"errors"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/matthew-on-git/terraform-provider-opnsense/pkg/opnsense"
)

var (
	_ resource.Resource                = &clientOverwriteResource{}
	_ resource.ResourceWithImportState = &clientOverwriteResource{}
)
var clientOverwriteReqOpts = opnsense.ReqOpts{AddEndpoint: "/api/openvpn/client_overwrites/add", GetEndpoint: "/api/openvpn/client_overwrites/get", UpdateEndpoint: "/api/openvpn/client_overwrites/set", DeleteEndpoint: "/api/openvpn/client_overwrites/del", SearchEndpoint: "/api/openvpn/client_overwrites/search", StrictDeleteResponse: true, ReconfigureEndpoint: "/api/openvpn/service/reconfigure", Monad: "cso"}

type clientOverwriteResource struct{ client *opnsense.Client }

func newClientOverwriteResource() resource.Resource { return &clientOverwriteResource{} }
func (r *clientOverwriteResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_openvpn_client_overwrite"
}

func (r *clientOverwriteResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	client, ok := req.ProviderData.(*opnsense.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected Provider Data", "Expected *opnsense.Client.")
		return
	}
	r.client = client
}

func (r *clientOverwriteResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan, config ClientOverwriteResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	config = plan
	if req.Config.Schema != nil {
		resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	}
	if resp.Diagnostics.HasError() {
		return
	}
	if clientOverwriteUsesParity(config) {
		if err := requireOpenVPNParityVersion(ctx, r.client); err != nil {
			resp.Diagnostics.AddError("Unsupported OpenVPN parity surface", err.Error())
			return
		}
	}
	payload, err := plan.toAPI(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Invalid OpenVPN client overwrite configuration", err.Error())
		return
	}
	before, err := clientOverwriteIDsByCommonName(ctx, r.client, plan.CommonName.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error preflighting OpenVPN client overwrite create", err.Error())
		return
	}
	uuid, err := opnsense.Add(ctx, r.client, clientOverwriteReqOpts, payload)
	if err != nil {
		var reconfigureErr *opnsense.MutationReconfigureError
		if errors.As(err, &reconfigureErr) {
			uuid = reconfigureErr.ResourceID
			resp.Diagnostics.Append(setOpenVPNPending(ctx, resp.Private, openVPNPendingMutation)...)
		}
		if uuid == "" {
			var recoverErr error
			uuid, recoverErr = recoverClientOverwriteID(ctx, r.client, plan, before)
			if recoverErr != nil {
				resp.Diagnostics.AddError("Unable to confirm OpenVPN client overwrite create outcome", recoverErr.Error())
			}
			if uuid != "" {
				resp.Diagnostics.Append(setOpenVPNPending(ctx, resp.Private, openVPNPendingMutation)...)
			}
		}
		if uuid != "" {
			plan.ID = types.StringValue(uuid)
			resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
		}
		resp.Diagnostics.AddError("Error creating OpenVPN client overwrite", err.Error())
		return
	}
	result, err := opnsense.Get[clientOverwriteAPIResponse](ctx, r.client, clientOverwriteReqOpts, uuid)
	if err != nil {
		plan.ID = types.StringValue(uuid)
		resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
		resp.Diagnostics.AddError("Error reading OpenVPN client overwrite after create", err.Error())
		return
	}
	if err := plan.fromAPI(ctx, result, uuid); err != nil {
		plan.ID = types.StringValue(uuid)
		resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
		resp.Diagnostics.AddError("Error decoding OpenVPN client overwrite after create", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *clientOverwriteResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state ClientOverwriteResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	pending, diags := getOpenVPNPending(ctx, req.Private)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	if pending != "" {
		if err := retryOpenVPNPending(ctx, r.client, clientOverwriteReqOpts); err != nil {
			resp.Diagnostics.AddError("Error activating pending OpenVPN mutation", err.Error())
			return
		}
		resp.Diagnostics.Append(clearOpenVPNPending(ctx, resp.Private)...)
		if pending == openVPNPendingDelete {
			resp.State.RemoveResource(ctx)
			return
		}
	}
	result, err := opnsense.Get[clientOverwriteAPIResponse](ctx, r.client, clientOverwriteReqOpts, state.ID.ValueString())
	if err != nil {
		var nf *opnsense.NotFoundError
		if errors.As(err, &nf) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading OpenVPN client overwrite", err.Error())
		return
	}
	if err := state.fromAPI(ctx, result, state.ID.ValueString()); err != nil {
		resp.Diagnostics.AddError("Error decoding OpenVPN client overwrite", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *clientOverwriteResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state, config ClientOverwriteResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	config = plan
	if req.Config.Schema != nil {
		resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	}
	if resp.Diagnostics.HasError() {
		return
	}
	if clientOverwriteUsesParity(config) {
		if err := requireOpenVPNParityVersion(ctx, r.client); err != nil {
			resp.Diagnostics.AddError("Unsupported OpenVPN parity surface", err.Error())
			return
		}
	}
	pending, diags := getOpenVPNPending(ctx, req.Private)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	if pending != "" {
		if err := retryOpenVPNPending(ctx, r.client, clientOverwriteReqOpts); err != nil {
			resp.Diagnostics.AddError("Error activating pending OpenVPN mutation", err.Error())
			return
		}
		resp.Diagnostics.Append(clearOpenVPNPending(ctx, resp.Private)...)
		matches, readErr := clientOverwriteRemoteMatchesPlan(ctx, r.client, state.ID.ValueString(), plan)
		if readErr != nil {
			resp.Diagnostics.AddError("Error reconciling pending OpenVPN client overwrite update", readErr.Error())
			return
		}
		if matches {
			plan.ID = types.StringValue(state.ID.ValueString())
			resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
			return
		}
	}
	payload, err := plan.toAPI(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Invalid OpenVPN client overwrite configuration", err.Error())
		return
	}
	id := state.ID.ValueString()
	if err := opnsense.Update(ctx, r.client, clientOverwriteReqOpts, payload, id); err != nil {
		var reconfigureErr *opnsense.MutationReconfigureError
		var validationErr *opnsense.ValidationError
		committed := errors.As(err, &reconfigureErr)
		if !committed && !errors.As(err, &validationErr) {
			var confirmErr error
			committed, confirmErr = clientOverwriteRemoteMatchesPlan(ctx, r.client, id, plan)
			if confirmErr != nil {
				resp.Diagnostics.AddError("Unable to confirm OpenVPN client overwrite update outcome", confirmErr.Error())
			}
		}
		if committed {
			resp.Diagnostics.Append(setOpenVPNPending(ctx, resp.Private, openVPNPendingMutation)...)
			plan.ID = types.StringValue(id)
			resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
		}
		resp.Diagnostics.AddError("Error updating OpenVPN client overwrite", err.Error())
		return
	}
	result, err := opnsense.Get[clientOverwriteAPIResponse](ctx, r.client, clientOverwriteReqOpts, id)
	if err != nil {
		resp.Diagnostics.AddError("Error reading OpenVPN client overwrite after update", err.Error())
		return
	}
	if err := plan.fromAPI(ctx, result, id); err != nil {
		resp.Diagnostics.AddError("Error decoding OpenVPN client overwrite after update", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *clientOverwriteResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state ClientOverwriteResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	pending, diags := getOpenVPNPending(ctx, req.Private)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	if pending != "" {
		if err := retryOpenVPNPending(ctx, r.client, clientOverwriteReqOpts); err != nil {
			resp.Diagnostics.AddError("Error activating pending OpenVPN mutation", err.Error())
			return
		}
		resp.Diagnostics.Append(clearOpenVPNPending(ctx, resp.Private)...)
		if pending == openVPNPendingDelete {
			resp.State.RemoveResource(ctx)
			return
		}
	}
	id := state.ID.ValueString()
	if err := opnsense.Delete(ctx, r.client, clientOverwriteReqOpts, id); err != nil {
		var nf *opnsense.NotFoundError
		if errors.As(err, &nf) {
			return
		}
		var reconfigureErr *opnsense.MutationReconfigureError
		var validationErr *opnsense.ValidationError
		deleted := errors.As(err, &reconfigureErr)
		if !deleted && !errors.As(err, &validationErr) {
			exists, confirmErr := clientOverwriteExistence(ctx, r.client, id)
			if confirmErr != nil {
				resp.Diagnostics.AddError("Unable to confirm OpenVPN client overwrite delete outcome", confirmErr.Error())
			} else {
				deleted = !exists
			}
		}
		if deleted {
			resp.Diagnostics.Append(setOpenVPNPending(ctx, resp.Private, openVPNPendingDelete)...)
		}
		resp.Diagnostics.AddError("Error deleting OpenVPN client overwrite", err.Error())
	}
}

func (r *clientOverwriteResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

type clientOverwriteSearchRow struct {
	UUID       string `json:"uuid"`
	CommonName string `json:"common_name"`
}

func clientOverwriteIDsByCommonName(ctx context.Context, client *opnsense.Client, commonName string) (map[string]struct{}, error) {
	if commonName == "" {
		return map[string]struct{}{}, nil
	}
	rows, err := opnsense.Search[clientOverwriteSearchRow](ctx, client, clientOverwriteReqOpts, opnsense.SearchParams{SearchQuery: commonName})
	if err != nil {
		return nil, err
	}
	found := make(map[string]struct{})
	for _, row := range rows {
		if row.CommonName == commonName {
			found[row.UUID] = struct{}{}
		}
	}
	return found, nil
}

func recoverClientOverwriteID(ctx context.Context, client *opnsense.Client, plan ClientOverwriteResourceModel, before map[string]struct{}) (string, error) {
	after, err := clientOverwriteIDsByCommonName(ctx, client, plan.CommonName.ValueString())
	if err != nil {
		return "", err
	}
	match := ""
	for id := range after {
		if _, existed := before[id]; existed {
			continue
		}
		equal, compareErr := clientOverwriteRemoteMatchesPlan(ctx, client, id, plan)
		if compareErr != nil {
			return "", compareErr
		}
		if equal {
			if match != "" {
				return "", fmt.Errorf("multiple newly created OpenVPN client overwrites match the requested payload")
			}
			match = id
		}
	}
	return match, nil
}

func clientOverwriteRemoteMatchesPlan(ctx context.Context, client *opnsense.Client, id string, plan ClientOverwriteResourceModel) (bool, error) {
	result, err := opnsense.Get[clientOverwriteAPIResponse](ctx, client, clientOverwriteReqOpts, id)
	if err != nil {
		var nf *opnsense.NotFoundError
		if errors.As(err, &nf) {
			return false, nil
		}
		return false, err
	}
	var remote ClientOverwriteResourceModel
	if err := remote.fromAPI(ctx, result, id); err != nil {
		return false, err
	}
	remotePayload, err := remote.toAPI(ctx)
	if err != nil {
		return false, err
	}
	plannedPayload, err := plan.toAPI(ctx)
	if err != nil {
		return false, err
	}
	return normalizedPayloadEqual(remotePayload, plannedPayload), nil
}

func clientOverwriteExistence(ctx context.Context, client *opnsense.Client, id string) (bool, error) {
	_, err := opnsense.Get[clientOverwriteAPIResponse](ctx, client, clientOverwriteReqOpts, id)
	if err == nil {
		return true, nil
	}
	var nf *opnsense.NotFoundError
	if errors.As(err, &nf) {
		return false, nil
	}
	return false, err
}
