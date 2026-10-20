// Copyright (c) Matthew Mellor
// SPDX-License-Identifier: MPL-2.0

package wireguard

import (
	"context"
	"errors"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/matthew-on-git/terraform-provider-opnsense/pkg/opnsense"
)

// Ensure peerResource satisfies the resource interfaces.
var (
	_ resource.Resource                = &peerResource{}
	_ resource.ResourceWithImportState = &peerResource{}
)

// peerReqOpts configures the OPNsense API endpoints for WireGuard peers (clients).
var peerReqOpts = opnsense.ReqOpts{
	AddEndpoint:                 "/api/wireguard/client/add_client",
	GetEndpoint:                 "/api/wireguard/client/get_client",
	UpdateEndpoint:              "/api/wireguard/client/set_client",
	DeleteEndpoint:              "/api/wireguard/client/del_client",
	SearchEndpoint:              "/api/wireguard/client/search_client",
	ReconfigureEndpoint:         "/api/wireguard/service/reconfigure",
	StrictDeleteResponse:        true,
	StrictMissingObjectResponse: true,
	StrictReconfigureResult:     true,
	Monad:                       "client",
}

// peerResource implements the opnsense_wireguard_peer resource.
type peerResource struct {
	client *opnsense.Client
}

func newPeerResource() resource.Resource {
	return &peerResource{}
}

// Metadata sets the resource type name.
func (r *peerResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_wireguard_peer"
}

// Configure extracts the OPNsense API client from provider data.
func (r *peerResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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
func (r *peerResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan PeerResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resolveOmittedNumber(ctx, req.Config, "server_port", &plan.ServerPort)...)
	resp.Diagnostics.Append(resolveOmittedNumber(ctx, req.Config, "keepalive", &plan.Keepalive)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(validateConfiguredKey(ctx, req.Config, "public_key", plan.PublicKey)...)
	resp.Diagnostics.Append(validateConfiguredKey(ctx, req.Config, "psk", plan.PSK)...)
	if resp.Diagnostics.HasError() {
		return
	}
	ownerMu.Lock()
	defer ownerMu.Unlock()
	payload, err := plan.toAPI(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Invalid WireGuard peer", err.Error())
		return
	}
	id, err := createOwned(ctx, r.client, peerReqOpts, payload)
	if id == "" {
		if err != nil {
			resp.Diagnostics.AddError("Error creating WireGuard peer", err.Error())
		}
		return
	}
	plan.ID = types.StringValue(id)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
	if err != nil {
		resp.Diagnostics.Append(setPending(ctx, resp.Private, pendingMutation)...)
		resp.Diagnostics.AddError("Saved WireGuard peer needs recovery", err.Error())
		return
	}
	result, err := opnsense.Get[wireguardPeerAPIResponse](ctx, r.client, peerReqOpts, id)
	if err == nil {
		err = plan.fromAPI(ctx, result, id)
	}
	if err != nil {
		resp.Diagnostics.Append(setPending(ctx, resp.Private, pendingMutation)...)
		resp.Diagnostics.AddError("Error reading saved WireGuard peer", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *peerResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state PeerResourceModel
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
		if err := opnsense.RetryReconfigure(ctx, r.client, peerReqOpts); err != nil {
			resp.Diagnostics.AddError("Error activating saved WireGuard peer", err.Error())
			return
		}
	}
	result, err := opnsense.Get[wireguardPeerAPIResponse](ctx, r.client, peerReqOpts, state.ID.ValueString())
	if err != nil {
		var missing *opnsense.NotFoundError
		if errors.As(err, &missing) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading WireGuard peer", err.Error())
		return
	}
	if err := state.fromAPI(ctx, result, state.ID.ValueString()); err != nil {
		resp.Diagnostics.AddError("Malformed WireGuard peer", err.Error())
		return
	}
	resp.Diagnostics.Append(reconcileSecret(ctx, resp.Private, result.PSK, &state.PSK)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
	if pending != "" && !resp.Diagnostics.HasError() {
		resp.Diagnostics.Append(setPending(ctx, resp.Private, "")...)
	}
}

func (r *peerResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, prior PeerResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &prior)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resolveOmittedNumber(ctx, req.Config, "server_port", &plan.ServerPort)...)
	resp.Diagnostics.Append(resolveOmittedNumber(ctx, req.Config, "keepalive", &plan.Keepalive)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(validateConfiguredKey(ctx, req.Config, "public_key", plan.PublicKey)...)
	resp.Diagnostics.Append(validateConfiguredKey(ctx, req.Config, "psk", plan.PSK)...)
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
		if err := opnsense.RetryReconfigure(ctx, r.client, peerReqOpts); err != nil {
			resp.Diagnostics.AddError("Error activating saved WireGuard peer", err.Error())
			return
		}
		result, err := opnsense.Get[wireguardPeerAPIResponse](ctx, r.client, peerReqOpts, prior.ID.ValueString())
		if err == nil {
			err = prior.fromAPI(ctx, result, prior.ID.ValueString())
		}
		if err != nil {
			resp.Diagnostics.AddError("Error reading recovered WireGuard peer", err.Error())
			return
		}
		resp.Diagnostics.Append(reconcileSecret(ctx, resp.Private, result.PSK, &prior.PSK)...)
		resp.Diagnostics.Append(resp.State.Set(ctx, &prior)...)
		if resp.Diagnostics.HasError() {
			return
		}
		resp.Diagnostics.Append(setPending(ctx, resp.Private, "")...)
		resp.Diagnostics.AddError("WireGuard update recovered; retry required", "The previous saved update was recovered without repeating a mutation. Run a fresh plan and apply to reconcile current intent.")
		return
	}
	if plan.PSK.IsUnknown() {
		plan.PSK = prior.PSK
	}
	payload, err := plan.toAPI(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Invalid WireGuard peer", err.Error())
		return
	}
	id := prior.ID.ValueString()
	// Released setClientAction interprets an absent servers key as a request
	// to detach this peer from every server. Preserve the authoritative current
	// backreference under ownerMu; never accept independent configured ownership.
	current, readErr := opnsense.Get[wireguardPeerAPIResponse](ctx, r.client, peerReqOpts, id)
	if readErr != nil {
		resp.Diagnostics.AddError("Error preserving WireGuard server associations", readErr.Error())
		return
	}
	servers := strings.Join([]string(current.Servers), ",")
	payload.Servers = &servers
	plan.ID = prior.ID
	err = opnsense.Update(ctx, r.client, peerReqOpts, payload, id)
	if err != nil {
		var validation *opnsense.ValidationError
		if errors.As(err, &validation) {
			resp.Diagnostics.AddError("Error updating WireGuard peer", err.Error())
			return
		}
		// A transport/metadata failure may follow a committed save. Preserve identity
		// and retry activation, never repeat the mutation during recovery.
		resp.Diagnostics.Append(setPending(ctx, resp.Private, pendingMutation)...)
		var confirmed *opnsense.MutationReconfigureError
		if errors.As(err, &confirmed) {
			resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
		} else {
			if !plan.PSK.Equal(prior.PSK) {
				resp.Diagnostics.Append(markSecretAmbiguous(ctx, resp.Private, prior.PSK, plan.PSK)...)
				prior.PSK = types.StringNull()
			}
			resp.Diagnostics.Append(resp.State.Set(ctx, &prior)...)
		}
		resp.Diagnostics.AddError("WireGuard peer update outcome requires reconciliation", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
	result, err := opnsense.Get[wireguardPeerAPIResponse](ctx, r.client, peerReqOpts, id)
	if err == nil {
		err = plan.fromAPI(ctx, result, id)
	}
	if err != nil {
		resp.Diagnostics.Append(setPending(ctx, resp.Private, pendingMutation)...)
		resp.Diagnostics.AddError("Error reading saved WireGuard peer", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *peerResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state PeerResourceModel
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
		if err := opnsense.RetryReconfigure(ctx, r.client, peerReqOpts); err != nil {
			resp.Diagnostics.AddError("Error activating saved WireGuard mutation", err.Error())
			return
		}
		if pending == pendingDeleteUnknown {
			_, readErr := opnsense.Get[wireguardPeerAPIResponse](ctx, r.client, peerReqOpts, state.ID.ValueString())
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
	err := opnsense.Delete(ctx, r.client, peerReqOpts, state.ID.ValueString())
	if err != nil {
		var activation *opnsense.MutationReconfigureError
		if errors.As(err, &activation) {
			resp.Diagnostics.Append(setPending(ctx, resp.Private, pendingDelete)...)
		} else {
			_, readErr := opnsense.Get[wireguardPeerAPIResponse](ctx, r.client, peerReqOpts, state.ID.ValueString())
			var missing *opnsense.NotFoundError
			if errors.As(readErr, &missing) {
				resp.Diagnostics.Append(setPending(ctx, resp.Private, pendingDelete)...)
			} else if readErr != nil {
				resp.Diagnostics.Append(setPending(ctx, resp.Private, pendingDeleteUnknown)...)
			}
		}
		resp.Diagnostics.AddError("Error deleting WireGuard peer", err.Error())
	}
}

func (r *peerResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
