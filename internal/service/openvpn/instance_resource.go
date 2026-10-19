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
	_ resource.Resource                = &instanceResource{}
	_ resource.ResourceWithImportState = &instanceResource{}
)
var instanceReqOpts = opnsense.ReqOpts{AddEndpoint: "/api/openvpn/instances/add", GetEndpoint: "/api/openvpn/instances/get", UpdateEndpoint: "/api/openvpn/instances/set", DeleteEndpoint: "/api/openvpn/instances/del", SearchEndpoint: "/api/openvpn/instances/search", StrictDeleteResponse: true, ReconfigureEndpoint: "/api/openvpn/service/reconfigure", Monad: "instance"}

type instanceResource struct{ client *opnsense.Client }

func newInstanceResource() resource.Resource { return &instanceResource{} }
func (r *instanceResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_openvpn_instance"
}

func (r *instanceResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *instanceResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan, config InstanceResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	config = plan
	if req.Config.Schema != nil {
		resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	}
	if resp.Diagnostics.HasError() {
		return
	}
	reconcileInstanceCipherConfig(&plan, config)
	if instanceUsesParity(config) {
		if err := requireOpenVPNParityVersion(ctx, r.client); err != nil {
			resp.Diagnostics.AddError("Unsupported OpenVPN parity surface", err.Error())
			return
		}
	}
	payload, err := plan.toAPI(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Invalid OpenVPN instance configuration", err.Error())
		return
	}
	before, err := instanceIDsByDescription(ctx, r.client, plan.Description.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error preflighting OpenVPN instance create", err.Error())
		return
	}
	uuid, err := opnsense.Add(ctx, r.client, instanceReqOpts, payload)
	if err != nil {
		var reconfigureErr *opnsense.MutationReconfigureError
		if errors.As(err, &reconfigureErr) {
			uuid = reconfigureErr.ResourceID
			resp.Diagnostics.Append(setOpenVPNPending(ctx, resp.Private, openVPNPendingMutation)...)
		}
		if uuid == "" {
			var recoverErr error
			uuid, recoverErr = recoverInstanceID(ctx, r.client, plan, before)
			if recoverErr != nil {
				resp.Diagnostics.AddError("Unable to confirm OpenVPN instance create outcome", recoverErr.Error())
			}
			if uuid != "" {
				resp.Diagnostics.Append(setOpenVPNPending(ctx, resp.Private, openVPNPendingMutation)...)
			}
		}
		if uuid != "" {
			plan.ID = types.StringValue(uuid)
			resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
		}
		resp.Diagnostics.AddError("Error creating OpenVPN instance", err.Error())
		return
	}
	result, err := opnsense.Get[instanceAPIResponse](ctx, r.client, instanceReqOpts, uuid)
	if err != nil {
		plan.ID = types.StringValue(uuid)
		resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
		resp.Diagnostics.AddError("Error reading OpenVPN instance after create", err.Error())
		return
	}
	if err := plan.fromAPI(ctx, result, uuid); err != nil {
		plan.ID = types.StringValue(uuid)
		resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
		resp.Diagnostics.AddError("Error decoding OpenVPN instance after create", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *instanceResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state InstanceResourceModel
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
		if err := retryOpenVPNPending(ctx, r.client, instanceReqOpts); err != nil {
			resp.Diagnostics.AddError("Error activating pending OpenVPN mutation", err.Error())
			return
		}
		resp.Diagnostics.Append(clearOpenVPNPending(ctx, resp.Private)...)
		if pending == openVPNPendingDelete {
			resp.State.RemoveResource(ctx)
			return
		}
	}
	result, err := opnsense.Get[instanceAPIResponse](ctx, r.client, instanceReqOpts, state.ID.ValueString())
	if err != nil {
		var nf *opnsense.NotFoundError
		if errors.As(err, &nf) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading OpenVPN instance", err.Error())
		return
	}
	if err := state.fromAPI(ctx, result, state.ID.ValueString()); err != nil {
		resp.Diagnostics.AddError("Error decoding OpenVPN instance", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *instanceResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state, config InstanceResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	config = plan
	if req.Config.Schema != nil {
		resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	}
	if resp.Diagnostics.HasError() {
		return
	}
	reconcileInstanceCipherConfig(&plan, config)
	if instanceUsesParity(config) {
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
		if err := retryOpenVPNPending(ctx, r.client, instanceReqOpts); err != nil {
			resp.Diagnostics.AddError("Error activating pending OpenVPN mutation", err.Error())
			return
		}
		resp.Diagnostics.Append(clearOpenVPNPending(ctx, resp.Private)...)
		matches, readErr := instanceRemoteMatchesPlan(ctx, r.client, state.ID.ValueString(), plan)
		if readErr != nil {
			resp.Diagnostics.AddError("Error reconciling pending OpenVPN instance update", readErr.Error())
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
		resp.Diagnostics.AddError("Invalid OpenVPN instance configuration", err.Error())
		return
	}
	id := state.ID.ValueString()
	if err := opnsense.Update(ctx, r.client, instanceReqOpts, payload, id); err != nil {
		var reconfigureErr *opnsense.MutationReconfigureError
		var validationErr *opnsense.ValidationError
		committed := errors.As(err, &reconfigureErr)
		if !committed && !errors.As(err, &validationErr) {
			var confirmErr error
			committed, confirmErr = instanceRemoteMatchesPlan(ctx, r.client, id, plan)
			if confirmErr != nil {
				resp.Diagnostics.AddError("Unable to confirm OpenVPN instance update outcome", confirmErr.Error())
			}
		}
		if committed {
			resp.Diagnostics.Append(setOpenVPNPending(ctx, resp.Private, openVPNPendingMutation)...)
			plan.ID = types.StringValue(id)
			resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
		}
		resp.Diagnostics.AddError("Error updating OpenVPN instance", err.Error())
		return
	}
	result, err := opnsense.Get[instanceAPIResponse](ctx, r.client, instanceReqOpts, id)
	if err != nil {
		resp.Diagnostics.AddError("Error reading OpenVPN instance after update", err.Error())
		return
	}
	if err := plan.fromAPI(ctx, result, id); err != nil {
		resp.Diagnostics.AddError("Error decoding OpenVPN instance after update", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *instanceResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state InstanceResourceModel
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
		if err := retryOpenVPNPending(ctx, r.client, instanceReqOpts); err != nil {
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
	if err := opnsense.Delete(ctx, r.client, instanceReqOpts, id); err != nil {
		var nf *opnsense.NotFoundError
		if errors.As(err, &nf) {
			return
		}
		var reconfigureErr *opnsense.MutationReconfigureError
		var validationErr *opnsense.ValidationError
		deleted := errors.As(err, &reconfigureErr)
		if !deleted && !errors.As(err, &validationErr) {
			exists, confirmErr := instanceExistence(ctx, r.client, id)
			if confirmErr != nil {
				resp.Diagnostics.AddError("Unable to confirm OpenVPN instance delete outcome", confirmErr.Error())
			} else {
				deleted = !exists
			}
		}
		if deleted {
			resp.Diagnostics.Append(setOpenVPNPending(ctx, resp.Private, openVPNPendingDelete)...)
		}
		resp.Diagnostics.AddError("Error deleting OpenVPN instance", err.Error())
	}
}

func (r *instanceResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

type instanceSearchRow struct {
	UUID        string `json:"uuid"`
	Description string `json:"description"`
}

func instanceIDsByDescription(ctx context.Context, client *opnsense.Client, description string) (map[string]struct{}, error) {
	if description == "" {
		return map[string]struct{}{}, nil
	}
	rows, err := opnsense.Search[instanceSearchRow](ctx, client, instanceReqOpts, opnsense.SearchParams{SearchQuery: description})
	if err != nil {
		return nil, err
	}
	found := make(map[string]struct{})
	for _, row := range rows {
		if row.Description == description {
			found[row.UUID] = struct{}{}
		}
	}
	return found, nil
}

func recoverInstanceID(ctx context.Context, client *opnsense.Client, plan InstanceResourceModel, before map[string]struct{}) (string, error) {
	after, err := instanceIDsByDescription(ctx, client, plan.Description.ValueString())
	if err != nil {
		return "", err
	}
	match := ""
	for id := range after {
		if _, existed := before[id]; existed {
			continue
		}
		equal, compareErr := instanceRemoteMatchesPlan(ctx, client, id, plan)
		if compareErr != nil {
			return "", compareErr
		}
		if equal {
			if match != "" {
				return "", fmt.Errorf("multiple newly created OpenVPN instances match the requested payload")
			}
			match = id
		}
	}
	return match, nil
}

func instanceRemoteMatchesPlan(ctx context.Context, client *opnsense.Client, id string, plan InstanceResourceModel) (bool, error) {
	result, err := opnsense.Get[instanceAPIResponse](ctx, client, instanceReqOpts, id)
	if err != nil {
		var nf *opnsense.NotFoundError
		if errors.As(err, &nf) {
			return false, nil
		}
		return false, err
	}
	var remote InstanceResourceModel
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

func instanceExistence(ctx context.Context, client *opnsense.Client, id string) (bool, error) {
	_, err := opnsense.Get[instanceAPIResponse](ctx, client, instanceReqOpts, id)
	if err == nil {
		return true, nil
	}
	var nf *opnsense.NotFoundError
	if errors.As(err, &nf) {
		return false, nil
	}
	return false, err
}

func reconcileInstanceCipherConfig(plan *InstanceResourceModel, config InstanceResourceModel) {
	if config.DataCiphers.IsNull() {
		plan.DataCiphers = config.DataCiphers
	}
	if config.DataCiphersOrdered.IsNull() {
		plan.DataCiphersOrdered = config.DataCiphersOrdered
	}
}
