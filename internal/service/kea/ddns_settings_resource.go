// Copyright (c) Matthew Mellor
// SPDX-License-Identifier: MPL-2.0

package kea

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/matthew-on-git/terraform-provider-opnsense/pkg/opnsense"
)

var _ resource.ResourceWithImportState = &ddnsSettingsResource{}

type ddnsSettingsResource struct{ client *opnsense.Client }

func newDDNSSettingsResource() resource.Resource { return &ddnsSettingsResource{} }
func (r *ddnsSettingsResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_kea_ddns_settings"
}

func (r *ddnsSettingsResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *ddnsSettingsResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	if req.ID != "ddns" {
		resp.Diagnostics.AddError("Invalid DDNS import identity", "Import requires exactly ddns.")
		return
	}
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *ddnsSettingsResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var config DDNSSettingsResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(saveDDNS(ctx, r.client, config, ddnsRecovery{}, func(m DDNSSettingsResourceModel) diag.Diagnostics {
		return resp.State.Set(context.WithoutCancel(ctx), &m)
	}, resp.Private)...)
}

func (r *ddnsSettingsResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var config, prior DDNSSettingsResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &prior)...)
	pending, d := ddnsPending(ctx, req.Private, prior.ID.ValueString())
	resp.Diagnostics.Append(d...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(saveDDNS(ctx, r.client, config, pending, func(m DDNSSettingsResourceModel) diag.Diagnostics {
		return resp.State.Set(context.WithoutCancel(ctx), &m)
	}, resp.Private)...)
}

func (r *ddnsSettingsResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state DDNSSettingsResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	pending, d := ddnsPending(ctx, req.Private, state.ID.ValueString())
	resp.Diagnostics.Append(d...)
	if resp.Diagnostics.HasError() {
		return
	}
	if r.client == nil {
		resp.Diagnostics.AddError("Unconfigured DDNS client", "The provider client is unavailable.")
		return
	}
	if err := r.client.LockMutex(ctx); err != nil {
		resp.Diagnostics.AddError("DDNS refresh canceled", err.Error())
		return
	}
	defer r.client.UnlockMutex()
	if err := requireDDNS(ctx, r.client); err != nil {
		resp.Diagnostics.AddError("DDNS unavailable", err.Error())
		return
	}
	resp.Diagnostics.Append(recoverDDNS(ctx, r.client, pending, func(m DDNSSettingsResourceModel) diag.Diagnostics {
		return resp.State.Set(context.WithoutCancel(ctx), &m)
	}, resp.Private)...)
}

// Destroy intentionally abandons even pending activation. No client access,
// capability check, recovery, stop, reset or reconfigure belongs here.
func (r *ddnsSettingsResource) Delete(ctx context.Context, _ resource.DeleteRequest, resp *resource.DeleteResponse) {
	resp.State.RemoveResource(context.WithoutCancel(ctx))
}
