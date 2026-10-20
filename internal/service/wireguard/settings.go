// Copyright (c) Matthew Mellor
// SPDX-License-Identifier: MPL-2.0

package wireguard

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	ds "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/matthew-on-git/terraform-provider-opnsense/pkg/opnsense"
)

const settingsID = "wireguard_settings"

var settingsOpts = opnsense.ReqOpts{GetEndpoint: "/api/wireguard/general/get", UpdateEndpoint: "/api/wireguard/general/set", ReconfigureEndpoint: "/api/wireguard/service/reconfigure", Monad: "general", StrictReconfigureResult: true, StrictMissingObjectResponse: true}

type (
	settingsModel struct {
		ID      types.String `tfsdk:"id"`
		Enabled types.Bool   `tfsdk:"enabled"`
	}
	settingsAPI struct {
		Enabled strictBool `json:"enabled"`
	}
)

func (s *settingsAPI) UnmarshalJSON(data []byte) error {
	type alias settingsAPI
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	if _, ok := raw["enabled"]; !ok {
		return fmt.Errorf("WireGuard settings response is missing enabled")
	}
	return json.Unmarshal(data, (*alias)(s))
}

type settingsResource struct{ client *opnsense.Client }

func newSettingsResource() resource.Resource { return &settingsResource{} }
func (r *settingsResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_wireguard_settings"
}

func (r *settingsResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{MarkdownDescription: "Import-first core WireGuard daemon settings singleton. Destroy only removes Terraform state.", Attributes: map[string]schema.Attribute{"id": schema.StringAttribute{Computed: true, MarkdownDescription: "Fixed identity wireguard_settings."}, "enabled": schema.BoolAttribute{Required: true, MarkdownDescription: "Whether the WireGuard daemon is enabled."}}}
}

func (r *settingsResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	var ok bool
	r.client, ok = req.ProviderData.(*opnsense.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected Provider Data", "Expected *opnsense.Client.")
	}
}

func (r *settingsResource) Create(_ context.Context, _ resource.CreateRequest, resp *resource.CreateResponse) {
	resp.Diagnostics.AddError("Import required", "WireGuard settings already exist. Import using ID wireguard_settings before managing enabled.")
}

func (r *settingsResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	ownerMu.Lock()
	defer ownerMu.Unlock()
	pending, d := getPending(ctx, req.Private)
	resp.Diagnostics.Append(d...)
	if resp.Diagnostics.HasError() {
		return
	}
	if pending != "" {
		if err := opnsense.RetryReconfigure(ctx, r.client, settingsOpts); err != nil {
			resp.Diagnostics.AddError("Error activating WireGuard settings", err.Error())
			return
		}
	}
	result, err := opnsense.GetSingleton[settingsAPI](ctx, r.client, settingsOpts)
	if err != nil {
		resp.Diagnostics.AddError("Error reading WireGuard settings", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &settingsModel{ID: types.StringValue(settingsID), Enabled: types.BoolValue(bool(result.Enabled))})...)
	if pending != "" {
		resp.Diagnostics.Append(setPending(ctx, resp.Private, "")...)
	}
}

func (r *settingsResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan settingsModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if plan.Enabled.IsUnknown() || plan.Enabled.IsNull() {
		resp.Diagnostics.AddError("Unresolved WireGuard settings", "enabled must be known before mutation")
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
		if err := opnsense.RetryReconfigure(ctx, r.client, settingsOpts); err != nil {
			resp.Diagnostics.AddError("Error activating saved WireGuard settings", err.Error())
			return
		}
	} else {
		payload := map[string]string{"enabled": opnsense.BoolToString(plan.Enabled.ValueBool())}
		saveOpts := settingsOpts
		saveOpts.ReconfigureEndpoint = ""
		if err := opnsense.UpdateSingleton(ctx, r.client, saveOpts, &payload); err != nil {
			var validation *opnsense.ValidationError
			if !errors.As(err, &validation) {
				resp.Diagnostics.Append(setPending(ctx, resp.Private, pendingMutation)...)
			}
			resp.Diagnostics.AddError("Error saving WireGuard settings", err.Error())
			return
		}
		resp.Diagnostics.Append(setPending(ctx, resp.Private, pendingMutation)...)
	}
	result, err := opnsense.GetSingleton[settingsAPI](ctx, r.client, settingsOpts)
	if err != nil {
		resp.Diagnostics.AddError("Error reading saved WireGuard settings", err.Error())
		return
	}
	if pending != "" && bool(result.Enabled) != plan.Enabled.ValueBool() {
		resp.Diagnostics.Append(setPending(ctx, resp.Private, "")...)
		resp.Diagnostics.AddError("WireGuard settings recovered; retry required", "The recovered saved value differs from the current plan. Run a fresh plan and apply.")
		return
	}
	plan.ID = types.StringValue(settingsID)
	plan.Enabled = types.BoolValue(bool(result.Enabled))
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
	if pending == "" {
		if err := opnsense.RetryReconfigure(ctx, r.client, settingsOpts); err != nil {
			resp.Diagnostics.AddError("Error activating saved WireGuard settings", err.Error())
			return
		}
	}
	resp.Diagnostics.Append(setPending(ctx, resp.Private, "")...)
}

func (r *settingsResource) Delete(_ context.Context, _ resource.DeleteRequest, _ *resource.DeleteResponse) {
}

func (r *settingsResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	if req.ID != settingsID {
		resp.Diagnostics.AddError("Invalid import identifier", "Use wireguard_settings for this singleton.")
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), settingsID)...)
}

type settingsDataSource struct{ client *opnsense.Client }

func newSettingsDataSource() datasource.DataSource { return &settingsDataSource{} }
func (d *settingsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_wireguard_settings"
}

func (d *settingsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = ds.Schema{MarkdownDescription: "Reads the core WireGuard daemon settings.", Attributes: map[string]ds.Attribute{"id": ds.StringAttribute{Computed: true, MarkdownDescription: "Fixed singleton identity."}, "enabled": ds.BoolAttribute{Computed: true, MarkdownDescription: "Whether the daemon is enabled."}}}
}

func (d *settingsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	var ok bool
	d.client, ok = req.ProviderData.(*opnsense.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected Provider Data", "Expected *opnsense.Client.")
	}
}

func (d *settingsDataSource) Read(ctx context.Context, _ datasource.ReadRequest, resp *datasource.ReadResponse) {
	result, err := opnsense.GetSingleton[settingsAPI](ctx, d.client, settingsOpts)
	if err != nil {
		resp.Diagnostics.AddError("Error reading WireGuard settings", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &settingsModel{ID: types.StringValue(settingsID), Enabled: types.BoolValue(bool(result.Enabled))})...)
}
