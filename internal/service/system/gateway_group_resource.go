// Copyright (c) Matthew Mellor
// SPDX-License-Identifier: MPL-2.0

package system

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"reflect"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/matthew-on-git/terraform-provider-opnsense/pkg/opnsense"
)

var _ resource.ResourceWithImportState = &gatewayGroupResource{}

type gatewayGroupResource struct{ client *opnsense.Client }

func newGatewayGroupResource() resource.Resource { return &gatewayGroupResource{} }
func (r *gatewayGroupResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_system_gateway_group"
}

func (r *gatewayGroupResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*opnsense.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected Provider Data", "Expected *opnsense.Client.")
		return
	}
	r.client = c
}

func (r *gatewayGroupResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	if !groupIDValid(req.ID) {
		resp.Diagnostics.AddError("Invalid gateway group UUID", "Import requires an exact UUID.")
		return
	}
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *gatewayGroupResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan GatewayGroupResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	payload, err := plan.payload(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Invalid gateway group", err.Error())
		return
	}
	if r.client == nil {
		resp.Diagnostics.AddError("Unconfigured client", "Gateway group client is not configured.")
		return
	}
	if err = r.client.LockMutex(ctx); err != nil {
		resp.Diagnostics.AddError("Gateway group canceled", err.Error())
		return
	}
	defer r.client.UnlockMutex()
	if err = requireGatewayGroup(ctx, r.client); err != nil {
		resp.Diagnostics.AddError("Gateway group unavailable", err.Error())
		return
	}
	id, err := createGatewayGroup(ctx, r.client, payload)
	if err != nil {
		resp.Diagnostics.AddError("Error creating gateway group", err.Error())
		return
	}
	if !groupIDValid(id) {
		resp.Diagnostics.AddError("Invalid saved gateway group UUID", "The add response supplied an invalid identity; reconcile the appliance before retrying.")
		return
	}
	plan.ID = types.StringValue(id)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
	resp.Diagnostics.Append(groupMark(ctx, resp.Private, groupSaved)...)
	remote, err := readGatewayGroup(ctx, r.client, id)
	if err != nil {
		resp.Diagnostics.AddError("Saved gateway group requires recovery", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &remote)...)
	if err = opnsense.Reconfigure(ctx, r.client, gatewayGroupReqOpts); err != nil {
		resp.Diagnostics.AddError("Saved gateway group requires activation", err.Error())
		return
	}
	resp.Diagnostics.Append(groupMark(ctx, resp.Private, "")...)
}

// recoverGatewayGroup reads authoritative truth before any recovery activation.
// In particular, an uncertain deletion must never be treated as confirmed.
func recoverGatewayGroup(ctx context.Context, c *opnsense.Client, id, pending string, attempt gatewayGroupPayload) (GatewayGroupResourceModel, bool, string, error) {
	remote, err := readGatewayGroup(ctx, c, id)
	if err != nil {
		if !groupMissing(err) {
			return remote, false, "", err
		}
		if pending == groupSaveUnknown {
			return remote, false, "", errors.New("uncertain saved gateway group is absent; attempted payload and identity retained until authoritative reconciliation")
		}
		if pending == groupDeleted || pending == groupDeleteUnknown || pending == groupSaved {
			if err := opnsense.Reconfigure(ctx, c, gatewayGroupReqOpts); err != nil {
				return remote, false, "", err
			}
		}
		return remote, true, "", nil
	}
	if pending == groupDeleteUnknown || pending == groupDeleted {
		return remote, false, "The tracked UUID is still present. Delete uncertainty was cleared without activation; a fresh normal destroy may remove it.", nil
	}
	if pending == groupSaveUnknown {
		got, err := remote.payload(ctx)
		if err != nil {
			return remote, false, "", err
		}
		if !reflect.DeepEqual(got, attempt) {
			return remote, false, "The authoritative object does not match the attempted save. Current values were retained without activation or another set; review a fresh plan.", nil
		}
	}
	if pending != "" {
		if err := opnsense.Reconfigure(ctx, c, gatewayGroupReqOpts); err != nil {
			return remote, false, "", err
		}
	}
	return remote, false, "", nil
}

func (r *gatewayGroupResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state GatewayGroupResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	pending, d := groupPending(ctx, req.Private)
	resp.Diagnostics.Append(d...)
	if resp.Diagnostics.HasError() {
		return
	}
	if !groupIDValid(state.ID.ValueString()) {
		resp.Diagnostics.AddError("Invalid gateway group UUID", "State must contain an exact UUID.")
		return
	}
	if r.client == nil {
		resp.Diagnostics.AddError("Unconfigured client", "Gateway group client is not configured.")
		return
	}
	if err := r.client.LockMutex(ctx); err != nil {
		resp.Diagnostics.AddError("Gateway group canceled", err.Error())
		return
	}
	defer r.client.UnlockMutex()
	if err := requireGatewayGroup(ctx, r.client); err != nil {
		resp.Diagnostics.AddError("Gateway group unavailable", err.Error())
		return
	}
	var attempt gatewayGroupPayload
	if pending == groupSaveUnknown {
		var d diag.Diagnostics
		attempt, d = groupAttempt(ctx, req.Private, state.ID.ValueString())
		resp.Diagnostics.Append(d...)
		if resp.Diagnostics.HasError() {
			return
		}
	}
	remote, removed, notice, err := recoverGatewayGroup(ctx, r.client, state.ID.ValueString(), pending, attempt)
	if err != nil {
		resp.Diagnostics.AddError("Error reading or recovering gateway group", err.Error())
		return
	}
	if removed {
		resp.State.RemoveResource(ctx)
	} else {
		resp.Diagnostics.Append(resp.State.Set(ctx, &remote)...)
	}
	resp.Diagnostics.Append(groupMark(ctx, resp.Private, "")...)
	if notice != "" {
		resp.Diagnostics.AddWarning("Gateway group outcome reconciled", notice)
	}
}

func (r *gatewayGroupResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, prior GatewayGroupResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &prior)...)
	pending, d := groupPending(ctx, req.Private)
	resp.Diagnostics.Append(d...)
	if resp.Diagnostics.HasError() {
		return
	}
	if !groupIDValid(prior.ID.ValueString()) {
		resp.Diagnostics.AddError("Invalid gateway group UUID", "State must contain an exact UUID.")
		return
	}
	plan.ID = prior.ID
	payload, err := plan.payload(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Invalid gateway group", err.Error())
		return
	}
	if !plan.Name.Equal(prior.Name) {
		resp.Diagnostics.AddError("Immutable gateway group name", "Changing name requires replacement.")
		return
	}
	if r.client == nil {
		resp.Diagnostics.AddError("Unconfigured client", "Gateway group client is not configured.")
		return
	}
	if err = r.client.LockMutex(ctx); err != nil {
		resp.Diagnostics.AddError("Gateway group canceled", err.Error())
		return
	}
	defer r.client.UnlockMutex()
	if err = requireGatewayGroup(ctx, r.client); err != nil {
		resp.Diagnostics.AddError("Gateway group unavailable", err.Error())
		return
	}
	if pending != "" {
		var attempt gatewayGroupPayload
		if pending == groupSaveUnknown {
			var d diag.Diagnostics
			attempt, d = groupAttempt(ctx, req.Private, prior.ID.ValueString())
			resp.Diagnostics.Append(d...)
			if resp.Diagnostics.HasError() {
				return
			}
		}
		remote, removed, _, e := recoverGatewayGroup(ctx, r.client, prior.ID.ValueString(), pending, attempt)
		if e != nil {
			resp.Diagnostics.AddError("Gateway group recovery failed", e.Error())
			return
		}
		if removed {
			resp.State.RemoveResource(ctx)
		} else {
			resp.Diagnostics.Append(resp.State.Set(ctx, &remote)...)
		}
		resp.Diagnostics.Append(groupMark(ctx, resp.Private, "")...)
		resp.Diagnostics.AddError("Gateway group recovered; fresh plan required", "Recovery completed without repeating the saved mutation. Run a fresh plan and apply.")
		return
	}
	// The stable item field ignores empty values instead of clearing a populated
	// first tier. Check authoritative saved truth, not potentially stale TF state,
	// and reject before changing any other fields under this same UUID.
	if len(plan.Tier1.Elements()) == 0 {
		remote, readErr := readGatewayGroup(ctx, r.client, prior.ID.ValueString())
		if readErr != nil {
			resp.Diagnostics.AddError("Cannot verify first-tier clear", readErr.Error())
			return
		}
		if len(remote.Tier1.Elements()) != 0 {
			resp.Diagnostics.Append(resp.State.Set(ctx, &remote)...)
			resp.Diagnostics.AddAttributeError(path.Root("tier_1"), "Unsupported first-tier clear", "The stable OPNsense gateway-group API ignores an empty update to populated tier_1. No fields were saved or activated; the same UUID and authoritative values are retained. Keep at least one first-tier member, or separately plan an explicit replacement after reviewing dependencies. Groups created with an empty tier_1 and nonempty later tiers remain supported.")
			return
		}
	}
	confirmed, err := saveGatewayGroup(ctx, r.client, payload, prior.ID.ValueString())
	if !confirmed {
		var eligibility *groupEligibilityError
		if errors.As(err, &eligibility) {
			resp.Diagnostics.AddError("Gateway group members unavailable", err.Error())
			return
		}
		var validation *opnsense.ValidationError
		if errors.As(err, &validation) {
			resp.Diagnostics.AddError("Gateway group validation failed", err.Error())
			return
		}
		resp.Diagnostics.Append(groupMarkAttempt(ctx, resp.Private, prior.ID.ValueString(), payload)...)
		if remote, e := readGatewayGroup(ctx, r.client, prior.ID.ValueString()); e == nil {
			resp.Diagnostics.Append(resp.State.Set(ctx, &remote)...)
		}
		resp.Diagnostics.AddError("Gateway group save requires reconciliation", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
	resp.Diagnostics.Append(groupMark(ctx, resp.Private, groupSaved)...)
	remote, err := readGatewayGroup(ctx, r.client, prior.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Saved gateway group requires recovery", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &remote)...)
	if err = opnsense.Reconfigure(ctx, r.client, gatewayGroupReqOpts); err != nil {
		resp.Diagnostics.AddError("Saved gateway group requires activation", err.Error())
		return
	}
	resp.Diagnostics.Append(groupMark(ctx, resp.Private, "")...)
}

func (r *gatewayGroupResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state GatewayGroupResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	pending, d := groupPending(ctx, req.Private)
	resp.Diagnostics.Append(d...)
	if resp.Diagnostics.HasError() {
		return
	}
	id := state.ID.ValueString()
	if !groupIDValid(id) {
		resp.Diagnostics.AddError("Invalid gateway group UUID", "State must contain an exact UUID.")
		return
	}
	if r.client == nil {
		resp.Diagnostics.AddError("Unconfigured client", "Gateway group client is not configured.")
		return
	}
	if err := r.client.LockMutex(ctx); err != nil {
		resp.Diagnostics.AddError("Gateway group canceled", err.Error())
		return
	}
	defer r.client.UnlockMutex()
	if err := requireGatewayGroup(ctx, r.client); err != nil {
		resp.Diagnostics.AddError("Gateway group unavailable", err.Error())
		return
	}
	if pending == groupDeleted || pending == groupDeleteUnknown || pending == groupSaveUnknown {
		var attempt gatewayGroupPayload
		if pending == groupSaveUnknown {
			var d diag.Diagnostics
			attempt, d = groupAttempt(ctx, req.Private, id)
			resp.Diagnostics.Append(d...)
			if resp.Diagnostics.HasError() {
				return
			}
		}
		remote, removed, _, err := recoverGatewayGroup(ctx, r.client, id, pending, attempt)
		if err != nil {
			resp.Diagnostics.AddError("Cannot reconcile gateway group deletion", err.Error())
			return
		}
		resp.Diagnostics.Append(groupMark(ctx, resp.Private, "")...)
		if removed {
			return
		}
		resp.Diagnostics.Append(resp.State.Set(ctx, &remote)...)
		resp.Diagnostics.AddError("Gateway group deletion reconciled; fresh destroy required", "Recovery reconciled the still-present object and retained its UUID and authoritative fields. Run a fresh normal destroy.")
		return
	}

	// Always establish object truth before deleting or retrying an uncertain delete.
	_, err := readGatewayGroup(ctx, r.client, id)
	if err != nil && !groupMissing(err) {
		resp.Diagnostics.AddError("Cannot verify gateway group before delete", err.Error())
		return
	}
	if err == nil {
		raw, deleteErr := groupRequest(ctx, r.client, http.MethodPost, gatewayGroupReqOpts.DeleteEndpoint+"/"+id, nil)
		if deleteErr == nil {
			deleteErr = opnsense.ParseStrictDeleteResponse(raw)
		}
		if deleteErr != nil {
			_, readErr := readGatewayGroup(ctx, r.client, id)
			if !groupMissing(readErr) {
				if readErr != nil {
					resp.Diagnostics.Append(groupMark(ctx, resp.Private, groupDeleteUnknown)...)
				}
				resp.Diagnostics.AddError("Error deleting gateway group", fmt.Sprintf("Delete was not confirmed: %v; authoritative read: %v", deleteErr, readErr))
				return
			}
		}
	}
	resp.Diagnostics.Append(groupMark(ctx, resp.Private, groupDeleted)...)
	if err = opnsense.Reconfigure(ctx, r.client, gatewayGroupReqOpts); err != nil {
		resp.Diagnostics.AddError("Deleted gateway group requires activation", err.Error())
		return
	}
	resp.Diagnostics.Append(groupMark(ctx, resp.Private, "")...)
}
