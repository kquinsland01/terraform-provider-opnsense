// Copyright (c) Matthew Mellor
// SPDX-License-Identifier: MPL-2.0

package wireguard

import (
	"context"
	"errors"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/matthew-on-git/terraform-provider-opnsense/pkg/opnsense"
)

// Ensure serverResource satisfies the resource interfaces.
var (
	_ resource.Resource                = &serverResource{}
	_ resource.ResourceWithImportState = &serverResource{}
)

// serverReqOpts configures the OPNsense API endpoints for WireGuard servers.
var serverReqOpts = opnsense.ReqOpts{
	AddEndpoint:                 "/api/wireguard/server/add_server",
	GetEndpoint:                 "/api/wireguard/server/get_server",
	UpdateEndpoint:              "/api/wireguard/server/set_server",
	DeleteEndpoint:              "/api/wireguard/server/del_server",
	SearchEndpoint:              "/api/wireguard/server/search_server",
	ReconfigureEndpoint:         "/api/wireguard/service/reconfigure",
	StrictDeleteResponse:        true,
	StrictMissingObjectResponse: true,
	StrictReconfigureResult:     true,
	Monad:                       "server",
}

// serverResource implements the opnsense_wireguard_server resource.
type serverResource struct {
	client *opnsense.Client
}

func newServerResource() resource.Resource {
	return &serverResource{}
}

// Metadata sets the resource type name.
func (r *serverResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_wireguard_server"
}

// Configure extracts the OPNsense API client from provider data.
func (r *serverResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

// Create owns discovery, mutation, and recovery under the WireGuard owner lock.
func (r *serverResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan ServerResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resolveOmittedNumber(ctx, req.Config, "port", &plan.Port)...)
	resp.Diagnostics.Append(resolveOmittedNumber(ctx, req.Config, "mtu", &plan.MTU)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(validateConfiguredKey(ctx, req.Config, "public_key", plan.PublicKey)...)
	resp.Diagnostics.Append(validateConfiguredKey(ctx, req.Config, "private_key", plan.PrivateKey)...)
	if resp.Diagnostics.HasError() {
		return
	}
	ownerMu.Lock()
	defer ownerMu.Unlock()
	payload, err := plan.toAPI(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Invalid WireGuard server", err.Error())
		return
	}
	payload.Instance = ""
	id, err := createOwned(ctx, r.client, serverReqOpts, payload)
	if id == "" {
		if err != nil {
			resp.Diagnostics.AddError("Error creating WireGuard server", err.Error())
		}
		return
	}
	plan.ID = types.StringValue(id)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
	if err != nil {
		resp.Diagnostics.Append(setPending(ctx, resp.Private, pendingMutation)...)
		resp.Diagnostics.AddError("Saved WireGuard server needs recovery", err.Error())
		return
	}
	result, err := opnsense.Get[wireguardServerAPIResponse](ctx, r.client, serverReqOpts, id)
	if err == nil {
		err = plan.fromAPI(ctx, result, id)
	}
	if err != nil {
		resp.Diagnostics.Append(setPending(ctx, resp.Private, pendingMutation)...)
		resp.Diagnostics.AddError("Error reading saved WireGuard server", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *serverResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state ServerResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	ownerMu.Lock()
	defer ownerMu.Unlock()
	pending, d := getPending(ctx, req.Private)
	resp.Diagnostics.Append(d...)
	if resp.Diagnostics.HasError() {
		return
	}
	if pending != "" {
		if err := opnsense.RetryReconfigure(ctx, r.client, serverReqOpts); err != nil {
			resp.Diagnostics.AddError("Error activating saved WireGuard server", err.Error())
			return
		}
	}
	result, err := opnsense.Get[wireguardServerAPIResponse](ctx, r.client, serverReqOpts, state.ID.ValueString())
	if err != nil {
		var missing *opnsense.NotFoundError
		if errors.As(err, &missing) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading WireGuard server", err.Error())
		return
	}
	if err := state.fromAPI(ctx, result, state.ID.ValueString()); err != nil {
		resp.Diagnostics.AddError("Malformed WireGuard server", err.Error())
		return
	}
	resp.Diagnostics.Append(reconcileSecret(ctx, resp.Private, result.PrivateKey, &state.PrivateKey)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
	if pending != "" && !resp.Diagnostics.HasError() {
		resp.Diagnostics.Append(setPending(ctx, resp.Private, "")...)
	}
}

func (r *serverResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, prior ServerResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &prior)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resolveOmittedNumber(ctx, req.Config, "port", &plan.Port)...)
	resp.Diagnostics.Append(resolveOmittedNumber(ctx, req.Config, "mtu", &plan.MTU)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(validateConfiguredKey(ctx, req.Config, "public_key", plan.PublicKey)...)
	resp.Diagnostics.Append(validateConfiguredKey(ctx, req.Config, "private_key", plan.PrivateKey)...)
	if resp.Diagnostics.HasError() {
		return
	}
	ownerMu.Lock()
	defer ownerMu.Unlock()
	pending, d := getPending(ctx, req.Private)
	resp.Diagnostics.Append(d...)
	if resp.Diagnostics.HasError() {
		return
	}
	if pending != "" {
		if err := opnsense.RetryReconfigure(ctx, r.client, serverReqOpts); err != nil {
			resp.Diagnostics.AddError("Error activating saved WireGuard server", err.Error())
			return
		}
		result, err := opnsense.Get[wireguardServerAPIResponse](ctx, r.client, serverReqOpts, prior.ID.ValueString())
		if err == nil {
			err = prior.fromAPI(ctx, result, prior.ID.ValueString())
		}
		if err != nil {
			resp.Diagnostics.AddError("Error reading recovered WireGuard server", err.Error())
			return
		}
		resp.Diagnostics.Append(reconcileSecret(ctx, resp.Private, result.PrivateKey, &prior.PrivateKey)...)
		resp.Diagnostics.Append(resp.State.Set(ctx, &prior)...)
		if resp.Diagnostics.HasError() {
			return
		}
		resp.Diagnostics.Append(setPending(ctx, resp.Private, "")...)
		resp.Diagnostics.AddError("WireGuard update recovered; retry required", "The previous saved update was recovered without repeating a mutation. Run a fresh plan and apply to reconcile current intent.")
		return
	}
	if plan.PrivateKey.IsUnknown() {
		plan.PrivateKey = prior.PrivateKey
	}
	plan.Instance = prior.Instance
	if plan.PublicKey.IsUnknown() {
		plan.PublicKey = prior.PublicKey
	}
	payload, err := plan.toAPI(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Invalid WireGuard server", err.Error())
		return
	}
	id := prior.ID.ValueString()
	plan.ID = prior.ID
	err = opnsense.Update(ctx, r.client, serverReqOpts, payload, id)
	if err != nil {
		var validation *opnsense.ValidationError
		if errors.As(err, &validation) {
			resp.Diagnostics.AddError("Error updating WireGuard server", err.Error())
			return
		}
		// A transport/metadata failure may follow a committed save. Preserve identity
		// and retry activation, never repeat the mutation during recovery.
		resp.Diagnostics.Append(setPending(ctx, resp.Private, pendingMutation)...)
		var confirmed *opnsense.MutationReconfigureError
		if errors.As(err, &confirmed) {
			resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
		} else {
			if !plan.PrivateKey.Equal(prior.PrivateKey) {
				resp.Diagnostics.Append(markSecretAmbiguous(ctx, resp.Private, prior.PrivateKey, plan.PrivateKey)...)
				prior.PrivateKey = types.StringNull()
			}
			resp.Diagnostics.Append(resp.State.Set(ctx, &prior)...)
		}
		resp.Diagnostics.AddError("WireGuard server update outcome requires reconciliation", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
	result, err := opnsense.Get[wireguardServerAPIResponse](ctx, r.client, serverReqOpts, id)
	if err == nil {
		err = plan.fromAPI(ctx, result, id)
	}
	if err != nil {
		resp.Diagnostics.Append(setPending(ctx, resp.Private, pendingMutation)...)
		resp.Diagnostics.AddError("Error reading saved WireGuard server", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *serverResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state ServerResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	ownerMu.Lock()
	defer ownerMu.Unlock()
	pending, d := getPending(ctx, req.Private)
	resp.Diagnostics.Append(d...)
	if resp.Diagnostics.HasError() {
		return
	}
	if pending != "" {
		if err := opnsense.RetryReconfigure(ctx, r.client, serverReqOpts); err != nil {
			resp.Diagnostics.AddError("Error activating saved WireGuard mutation", err.Error())
			return
		}
		if pending == pendingDeleteUnknown {
			_, readErr := opnsense.Get[wireguardServerAPIResponse](ctx, r.client, serverReqOpts, state.ID.ValueString())
			var missing *opnsense.NotFoundError
			if errors.As(readErr, &missing) {
				resp.Diagnostics.Append(setPending(ctx, resp.Private, "")...)
				return
			}
			if readErr != nil {
				resp.Diagnostics.AddError("Unable to reconcile ambiguous WireGuard deletion", readErr.Error())
				return
			}
		}
		resp.Diagnostics.Append(setPending(ctx, resp.Private, "")...)
		if pending == pendingDelete {
			return
		}
	}
	err := opnsense.Delete(ctx, r.client, serverReqOpts, state.ID.ValueString())
	if err != nil {
		var activation *opnsense.MutationReconfigureError
		if errors.As(err, &activation) {
			resp.Diagnostics.Append(setPending(ctx, resp.Private, pendingDelete)...)
		} else {
			_, readErr := opnsense.Get[wireguardServerAPIResponse](ctx, r.client, serverReqOpts, state.ID.ValueString())
			var missing *opnsense.NotFoundError
			if errors.As(readErr, &missing) {
				resp.Diagnostics.Append(setPending(ctx, resp.Private, pendingDelete)...)
			} else if readErr != nil {
				resp.Diagnostics.Append(setPending(ctx, resp.Private, pendingDeleteUnknown)...)
			}
		}
		resp.Diagnostics.AddError("Error deleting WireGuard server", err.Error())
	}
}

func (r *serverResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
