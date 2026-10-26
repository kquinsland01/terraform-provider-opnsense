// Copyright (c) Matthew Mellor
// SPDX-License-Identifier: MPL-2.0

package kea

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/matthew-on-git/terraform-provider-opnsense/pkg/opnsense"
)

var _ resource.ResourceWithImportState = &dhcpv4OptionResource{}

type dhcpv4OptionResource struct{ client *opnsense.Client }

func newDHCPv4OptionResource() resource.Resource { return &dhcpv4OptionResource{} }
func (r *dhcpv4OptionResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_kea_dhcpv4_option"
}

func (r *dhcpv4OptionResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *dhcpv4OptionResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	if !optionIDValid(req.ID) {
		resp.Diagnostics.AddError("Invalid Kea option UUID", "Import requires an exact UUID.")
		return
	}
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *dhcpv4OptionResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan DHCPv4OptionResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var config DHCPv4OptionResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}
	payload, err := plan.wireIntent(config, true)
	if err != nil {
		resp.Diagnostics.AddError("Invalid Kea option", err.Error())
		return
	}
	if r.client == nil {
		resp.Diagnostics.AddError("Unconfigured client", "Kea option client is not configured.")
		return
	}
	if err = r.client.LockMutex(ctx); err != nil {
		resp.Diagnostics.AddError("Kea option canceled", err.Error())
		return
	}
	defer r.client.UnlockMutex()
	if err = requireDHCPv4Option(ctx, r.client); err != nil {
		resp.Diagnostics.AddError("Kea option unavailable", err.Error())
		return
	}
	id, err := createDHCPv4Option(ctx, r.client, payload)
	if err != nil {
		resp.Diagnostics.AddError("Error creating Kea option", err.Error())
		return
	}
	if !optionIDValid(id) {
		resp.Diagnostics.AddError("Invalid saved Kea option UUID", "The add response supplied an invalid identity; reconcile the appliance before retrying.")
		return
	}
	plan.ID = types.StringValue(id)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
	resp.Diagnostics.Append(optionMark(ctx, resp.Private, id, optionSaved, payload)...)
	if resp.Diagnostics.HasError() {
		return
	}
	remote, err := readDHCPv4Option(ctx, r.client, id)
	if err != nil {
		resp.Diagnostics.AddError("Saved Kea option requires recovery", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &remote)...)
	if !optionMatches(remote, payload) {
		resp.Diagnostics.AddError("Saved Kea option requires reconciliation", "Authoritative values do not match requested wire fields; no activation attempted")
		return
	}
	if err = activateDHCPv4Option(ctx, r.client); err != nil {
		resp.Diagnostics.AddError("Saved Kea option requires activation", err.Error())
		return
	}
	resp.Diagnostics.Append(optionMark(ctx, resp.Private, "", "", nil)...)
}

func (r *dhcpv4OptionResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state DHCPv4OptionResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	pending, d := optionPending(ctx, req.Private, state.ID.ValueString())
	resp.Diagnostics.Append(d...)
	if resp.Diagnostics.HasError() {
		return
	}
	if !optionIDValid(state.ID.ValueString()) {
		resp.Diagnostics.AddError("Invalid Kea option UUID", "State must contain an exact UUID.")
		return
	}
	if r.client == nil {
		resp.Diagnostics.AddError("Unconfigured client", "Kea option client is not configured.")
		return
	}
	if err := r.client.LockMutex(ctx); err != nil {
		resp.Diagnostics.AddError("Kea option canceled", err.Error())
		return
	}
	defer r.client.UnlockMutex()
	if err := requireDHCPv4Option(ctx, r.client); err != nil {
		resp.Diagnostics.AddError("Kea option unavailable", err.Error())
		return
	}
	remote, removed, notice, err := recoverDHCPv4Option(ctx, r.client, state.ID.ValueString(), pending)
	if err != nil {
		resp.Diagnostics.AddError("Error reading or recovering Kea option", err.Error())
		return
	}
	if removed {
		resp.State.RemoveResource(ctx)
	} else {
		resp.Diagnostics.Append(resp.State.Set(ctx, &remote)...)
	}
	resp.Diagnostics.Append(optionMark(ctx, resp.Private, "", "", nil)...)
	if notice != "" {
		resp.Diagnostics.AddWarning("Kea option outcome reconciled", notice)
	}
}

func (r *dhcpv4OptionResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, prior DHCPv4OptionResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &prior)...)
	pending, d := optionPending(ctx, req.Private, prior.ID.ValueString())
	resp.Diagnostics.Append(d...)
	if resp.Diagnostics.HasError() {
		return
	}
	if !optionIDValid(prior.ID.ValueString()) {
		resp.Diagnostics.AddError("Invalid Kea option UUID", "State must contain an exact UUID.")
		return
	}
	plan.ID = prior.ID
	var config DHCPv4OptionResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}
	payload, err := plan.wireIntent(config, false)
	if err != nil {
		resp.Diagnostics.AddError("Invalid Kea option", err.Error())
		return
	}
	if r.client == nil {
		resp.Diagnostics.AddError("Unconfigured client", "Kea option client is not configured.")
		return
	}
	if err = r.client.LockMutex(ctx); err != nil {
		resp.Diagnostics.AddError("Kea option canceled", err.Error())
		return
	}
	defer r.client.UnlockMutex()
	if err = requireDHCPv4Option(ctx, r.client); err != nil {
		resp.Diagnostics.AddError("Kea option unavailable", err.Error())
		return
	}
	if pending.Kind != "" {
		remote, removed, _, e := recoverDHCPv4Option(ctx, r.client, prior.ID.ValueString(), pending)
		if e != nil {
			resp.Diagnostics.AddError("Kea option recovery failed", e.Error())
			return
		}
		if removed {
			resp.State.RemoveResource(ctx)
		} else {
			resp.Diagnostics.Append(resp.State.Set(ctx, &remote)...)
		}
		resp.Diagnostics.Append(optionMark(ctx, resp.Private, "", "", nil)...)
		resp.Diagnostics.AddError("Kea option recovered; fresh plan required", "Recovery completed without repeating the saved mutation. Run a fresh plan and apply.")
		return
	}
	// setBase is an upsert: never set a UUID proven absent at preflight.
	remoteBefore, err := readDHCPv4Option(ctx, r.client, prior.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Cannot verify Kea option before update", err.Error()+"; no set issued; run a fresh plan")
		return
	}
	merged, _ := remoteBefore.values()
	for k, v := range payload {
		merged[k] = v
	}
	if err = validateOptionPayload(merged, true); err != nil {
		resp.Diagnostics.AddError("Invalid Kea option matching", err.Error())
		return
	}
	if err = optionValidateCatalog(ctx, r.client, payload); err != nil {
		resp.Diagnostics.AddError("Invalid Kea option selector", err.Error())
		return
	}
	raw, err := optionRequest(ctx, r.client, http.MethodPost, dhcpv4OptionReqOpts.UpdateEndpoint+"/"+prior.ID.ValueString(), map[string]any{"option": payload})
	if err == nil {
		_, err = opnsense.ParseMutationResponse(raw)
	}
	if err != nil {
		var validation *opnsense.ValidationError
		if errors.As(err, &validation) {
			resp.Diagnostics.AddError("Kea option validation failed", err.Error())
			return
		}
		resp.Diagnostics.Append(optionMark(ctx, resp.Private, prior.ID.ValueString(), optionSaveUnknown, payload)...)
		if remote, e := readDHCPv4Option(ctx, r.client, prior.ID.ValueString()); e == nil {
			resp.Diagnostics.Append(resp.State.Set(ctx, &remote)...)
		}
		resp.Diagnostics.AddError("Kea option save requires reconciliation", err.Error())
		return
	}
	committed := optionModelFromPayload(merged, prior.ID.ValueString())
	resp.Diagnostics.Append(resp.State.Set(ctx, &committed)...)
	resp.Diagnostics.Append(optionMark(ctx, resp.Private, prior.ID.ValueString(), optionSaved, payload)...)
	if resp.Diagnostics.HasError() {
		return
	}
	remote, err := readDHCPv4Option(ctx, r.client, prior.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Saved Kea option requires recovery", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &remote)...)
	if !optionMatches(remote, payload) {
		resp.Diagnostics.AddError("Saved Kea option requires reconciliation", "Authoritative values do not match requested wire fields; no activation attempted")
		return
	}
	if err = activateDHCPv4Option(ctx, r.client); err != nil {
		resp.Diagnostics.AddError("Saved Kea option requires activation", err.Error())
		return
	}
	resp.Diagnostics.Append(optionMark(ctx, resp.Private, "", "", nil)...)
}

func (r *dhcpv4OptionResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state DHCPv4OptionResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	pending, d := optionPending(ctx, req.Private, state.ID.ValueString())
	resp.Diagnostics.Append(d...)
	if resp.Diagnostics.HasError() {
		return
	}
	id := state.ID.ValueString()
	if !optionIDValid(id) {
		resp.Diagnostics.AddError("Invalid Kea option UUID", "State must contain an exact UUID.")
		return
	}
	if r.client == nil {
		resp.Diagnostics.AddError("Unconfigured client", "Kea option client is not configured.")
		return
	}
	if err := r.client.LockMutex(ctx); err != nil {
		resp.Diagnostics.AddError("Kea option canceled", err.Error())
		return
	}
	defer r.client.UnlockMutex()
	if err := requireDHCPv4Option(ctx, r.client); err != nil {
		resp.Diagnostics.AddError("Kea option unavailable", err.Error())
		return
	}
	if pending.Kind != "" {
		remote, removed, _, err := recoverDHCPv4Option(ctx, r.client, id, pending)
		if err != nil {
			resp.Diagnostics.AddError("Cannot reconcile Kea option deletion", err.Error())
			return
		}
		resp.Diagnostics.Append(optionMark(ctx, resp.Private, "", "", nil)...)
		if removed {
			return
		}
		resp.Diagnostics.Append(resp.State.Set(ctx, &remote)...)
		resp.Diagnostics.AddError("Kea option deletion reconciled; fresh destroy required", "Recovery reconciled the still-present object and retained its UUID and authoritative fields. Run a fresh normal destroy.")
		return
	}

	// Always establish object truth before deleting or retrying an uncertain delete.
	_, err := readDHCPv4Option(ctx, r.client, id)
	if err != nil && !optionMissing(err) {
		resp.Diagnostics.AddError("Cannot verify Kea option before delete", err.Error())
		return
	}
	if err == nil {
		raw, deleteErr := optionRequest(ctx, r.client, http.MethodPost, dhcpv4OptionReqOpts.DeleteEndpoint+"/"+id, nil)
		if deleteErr == nil {
			deleteErr = opnsense.ParseStrictDeleteResponse(raw)
		}
		if deleteErr != nil {
			_, readErr := readDHCPv4Option(ctx, r.client, id)
			if !optionMissing(readErr) {
				if readErr != nil {
					resp.Diagnostics.Append(optionMark(ctx, resp.Private, id, optionDeleteUnknown, nil)...)
				}
				resp.Diagnostics.AddError("Error deleting Kea option", fmt.Sprintf("Delete was not confirmed: %v; authoritative read: %v", deleteErr, readErr))
				return
			}
		}
	}
	_, absenceErr := readDHCPv4Option(ctx, r.client, id)
	if !optionMissing(absenceErr) {
		resp.Diagnostics.Append(optionMark(ctx, resp.Private, id, optionDeleteUnknown, nil)...)
		resp.Diagnostics.AddError("Cannot confirm Kea option deletion", "UUID still present or authoritative truth unavailable; no activation attempted")
		return
	}
	resp.Diagnostics.Append(optionMark(ctx, resp.Private, id, optionDeleted, nil)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err = activateDHCPv4Option(ctx, r.client); err != nil {
		resp.Diagnostics.AddError("Deleted Kea option requires activation", err.Error())
		return
	}
	_, err = readDHCPv4Option(ctx, r.client, id)
	if !optionMissing(err) {
		resp.Diagnostics.AddError("Cannot confirm Kea option deletion", "UUID reappeared or authoritative truth is unavailable; identity retained")
		return
	}
	resp.Diagnostics.Append(optionMark(ctx, resp.Private, "", "", nil)...)
}
