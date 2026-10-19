// Copyright (c) Matthew Mellor
// SPDX-License-Identifier: MPL-2.0

package unbound

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/matthew-on-git/terraform-provider-opnsense/pkg/opnsense"
)

// Ensure domainOverrideResource satisfies the resource interfaces.
var (
	_ resource.Resource                = &domainOverrideResource{}
	_ resource.ResourceWithImportState = &domainOverrideResource{}
)

const (
	domainOverridePendingKey      = "unbound_forward_pending_activation_v1"
	domainOverridePendingMutation = "mutation"
	domainOverridePendingDelete   = "delete"
)

func getDomainOverridePending(ctx context.Context, p unboundPrivateState) (string, diag.Diagnostics) {
	if p == nil {
		return "", nil
	}
	value, diagnostics := p.GetKey(ctx, domainOverridePendingKey)
	if len(value) == 0 || diagnostics.HasError() {
		return "", diagnostics
	}
	var pending string
	if err := json.Unmarshal(value, &pending); err != nil {
		diagnostics.AddError("Invalid private pending state", err.Error())
		return "", diagnostics
	}
	if pending != domainOverridePendingMutation && pending != domainOverridePendingDelete {
		diagnostics.AddError("Invalid private pending state", fmt.Sprintf("unknown Unbound forward pending marker %q", pending))
		return "", diagnostics
	}
	return pending, diagnostics
}

func setDomainOverridePending(ctx context.Context, p unboundPrivateState, value string) diag.Diagnostics {
	if value != "" && value != domainOverridePendingMutation && value != domainOverridePendingDelete {
		return diag.Diagnostics{diag.NewErrorDiagnostic("Invalid private pending state", fmt.Sprintf("unknown Unbound forward pending marker %q", value))}
	}
	if p == nil {
		return nil
	}
	if value == "" {
		return p.SetKey(ctx, domainOverridePendingKey, nil)
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return diag.Diagnostics{diag.NewErrorDiagnostic("Unable to encode private pending state", err.Error())}
	}
	return p.SetKey(ctx, domainOverridePendingKey, encoded)
}

// domainOverrideReqOpts configures the OPNsense API endpoints for Unbound domain overrides.
var domainOverrideReqOpts = opnsense.ReqOpts{
	AddEndpoint:                 "/api/unbound/settings/add_forward",
	GetEndpoint:                 "/api/unbound/settings/get_forward",
	UpdateEndpoint:              "/api/unbound/settings/set_forward",
	DeleteEndpoint:              "/api/unbound/settings/del_forward",
	SearchEndpoint:              "/api/unbound/settings/search_forward",
	ReconfigureEndpoint:         "/api/unbound/service/reconfigure",
	StrictReconfigureResponse:   true,
	Monad:                       "dot",
	StrictDeleteResponse:        true,
	StrictMissingObjectResponse: true,
}

var domainOverrideDotReqOpts = opnsense.ReqOpts{
	AddEndpoint:                 "/api/unbound/settings/add_dot",
	GetEndpoint:                 "/api/unbound/settings/get_forward",
	UpdateEndpoint:              "/api/unbound/settings/set_dot",
	DeleteEndpoint:              "/api/unbound/settings/del_forward",
	SearchEndpoint:              "/api/unbound/settings/search_dot",
	ReconfigureEndpoint:         "/api/unbound/service/reconfigure",
	StrictReconfigureResponse:   true,
	Monad:                       "dot",
	StrictDeleteResponse:        true,
	StrictMissingObjectResponse: true,
}

func domainOverrideMutationOpts(forwardType string) opnsense.ReqOpts {
	if forwardType == "dot" {
		return domainOverrideDotReqOpts
	}
	return domainOverrideReqOpts
}

func legacyDomainOverrideReqOpts() opnsense.ReqOpts {
	opts := domainOverrideReqOpts
	opts.StrictReconfigureResponse = false
	return opts
}

func domainOverrideRuntimeOpts(ctx context.Context, client *opnsense.Client, forwardType string) (opnsense.ReqOpts, bool, string, error) {
	supported, version, err := unboundParitySupport(ctx, client)
	if err != nil {
		return opnsense.ReqOpts{}, false, "", err
	}
	if supported {
		return domainOverrideMutationOpts(forwardType), true, version, nil
	}
	return legacyDomainOverrideReqOpts(), false, version, nil
}

// domainOverrideResource implements the opnsense_unbound_domain_override resource.
type domainOverrideResource struct {
	client     *opnsense.Client
	createLock unboundCreateLocker
}

func domainOverrideUsesParity(model DomainOverrideResourceModel) bool {
	return (!model.Type.IsNull() && !model.Type.IsUnknown() && model.Type.ValueString() == "dot") ||
		(!model.ServerPort.IsNull() && !model.ServerPort.IsUnknown() && model.ServerPort.ValueInt64() != 53) ||
		(!model.VerifyCN.IsNull() && !model.VerifyCN.IsUnknown() && model.VerifyCN.ValueString() != "")
}

func getDomainOverride(ctx context.Context, client *opnsense.Client, id string) (*domainOverrideAPIResponse, error) {
	current, currentErr := opnsense.Get[domainOverrideAPIResponse](ctx, client, domainOverrideReqOpts, id)
	if currentErr == nil {
		return current, nil
	}
	var missing *opnsense.NotFoundError
	if errors.As(currentErr, &missing) {
		return nil, currentErr
	}
	supported, _, compatibilityErr := unboundParitySupport(ctx, client)
	if compatibilityErr != nil {
		return nil, fmt.Errorf("%w (compatibility check also failed: %v)", currentErr, compatibilityErr)
	}
	if supported {
		return nil, currentErr
	}
	legacy, err := opnsense.Get[domainOverrideLegacyAPIResponse](ctx, client, domainOverrideReqOpts, id)
	if err != nil {
		return nil, err
	}
	port := legacy.Port
	if port == "" {
		port = "53"
	}
	return &domainOverrideAPIResponse{
		Enabled: legacy.Enabled, Domain: legacy.Domain, Type: opnsense.SelectedMap("forward"), Server: legacy.Server,
		Port: port, Verify: legacy.Verify, Description: legacy.Description,
	}, nil
}

func newDomainOverrideResource() resource.Resource {
	return &domainOverrideResource{}
}

// Metadata sets the resource type name.
func (r *domainOverrideResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_unbound_domain_override"
}

// Configure extracts the OPNsense API client from provider data.
func (r *domainOverrideResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

// Create creates a new Unbound domain override via the OPNsense API.
func (r *domainOverrideResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan DomainOverrideResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	opts := domainOverrideMutationOpts(plan.Type.ValueString())
	if req.Config.Schema != nil {
		var config DomainOverrideResourceModel
		resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
		if resp.Diagnostics.HasError() {
			return
		}
		var supported bool
		var version string
		var err error
		opts, supported, version, err = domainOverrideRuntimeOpts(ctx, r.client, plan.Type.ValueString())
		if err != nil {
			resp.Diagnostics.AddError("Unable to check OPNsense compatibility", err.Error())
			return
		}
		if domainOverrideUsesParity(config) && !supported {
			resp.Diagnostics.AddError("Unsupported OPNsense version", fmt.Sprintf("Unbound parity fields require stable OPNsense %s or newer; appliance reports %s", unboundParityMinimumVersion, version))
			return
		}
	}
	r.create(ctx, plan, opts, resp.Private, resp.State.Set, &resp.Diagnostics)
}

func (r *domainOverrideResource) create(ctx context.Context, plan DomainOverrideResourceModel, opts opnsense.ReqOpts, private unboundPrivateState, stateSetter func(context.Context, any) diag.Diagnostics, diagnostics *diag.Diagnostics) {
	createLock := ownerCreateLocker(r.createLock)
	createLock.Lock()
	defer createLock.Unlock()

	apiReq := plan.toAPI(ctx)
	before, err := domainOverrideIDsWithOpts(ctx, r.client, plan, opts)
	if err != nil {
		diagnostics.AddError("Error preflighting Unbound domain override create", err.Error())
		return
	}

	uuid, err := opnsense.Add(ctx, r.client, opts, apiReq)
	if err != nil {
		var activationErr *opnsense.MutationReconfigureError
		var validationErr *opnsense.ValidationError
		committed := uuid != "" && errors.As(err, &activationErr)
		if uuid == "" && !errors.As(err, &validationErr) {
			var recoveryErr error
			uuid, recoveryErr = recoverDomainOverrideIDWithOpts(ctx, r.client, plan, before, opts)
			if recoveryErr != nil {
				diagnostics.AddError("Unable to confirm Unbound domain override create outcome", recoveryErr.Error())
			}
			committed = uuid != ""
		}
		if committed {
			confirmed, _ := readDomainOverrideState(ctx, r.client, uuid)
			if confirmed == nil {
				confirmed = &DomainOverrideResourceModel{ID: types.StringValue(uuid)}
			}
			*diagnostics = append(*diagnostics, setDomainOverridePending(ctx, private, domainOverridePendingMutation)...)
			*diagnostics = append(*diagnostics, stateSetter(ctx, confirmed)...)
		}
		diagnostics.AddError(
			"Error creating Unbound domain override",
			fmt.Sprintf("Could not create Unbound domain override: %s", err),
		)
		return
	}

	// Read back from API to populate state (never echo from config).
	result, err := getDomainOverride(ctx, r.client, uuid)
	if err != nil {
		confirmed, _ := readDomainOverrideState(ctx, r.client, uuid)
		if confirmed == nil {
			confirmed = &DomainOverrideResourceModel{ID: types.StringValue(uuid)}
		}
		*diagnostics = append(*diagnostics, stateSetter(ctx, confirmed)...)
		diagnostics.AddError(
			"Error reading Unbound domain override after create",
			fmt.Sprintf("Created domain override %s but could not read it back: %s", uuid, err),
		)
		return
	}

	if err := plan.fromAPI(ctx, result, uuid); err != nil {
		confirmed, _ := readDomainOverrideState(ctx, r.client, uuid)
		if confirmed == nil {
			confirmed = &DomainOverrideResourceModel{ID: types.StringValue(uuid)}
		}
		*diagnostics = append(*diagnostics, stateSetter(ctx, confirmed)...)
		diagnostics.AddError("Error decoding Unbound domain override after create", err.Error())
		return
	}
	*diagnostics = append(*diagnostics, stateSetter(ctx, &plan)...)
}

type domainOverrideSearchRow struct {
	UUID        string `json:"uuid"`
	Enabled     string `json:"enabled"`
	Domain      string `json:"domain"`
	Server      string `json:"server"`
	Port        string `json:"port"`
	Verify      string `json:"verify"`
	Description string `json:"description"`
}

func domainOverrideIDsWithOpts(ctx context.Context, client *opnsense.Client, plan DomainOverrideResourceModel, opts opnsense.ReqOpts) (map[string]struct{}, error) {
	rows, err := opnsense.Search[domainOverrideSearchRow](ctx, client, opts, opnsense.SearchParams{SearchQuery: plan.Domain.ValueString()})
	if err != nil {
		return nil, err
	}
	found := make(map[string]struct{}, len(rows))
	for _, row := range rows {
		if row.UUID == "" {
			return nil, fmt.Errorf("malformed Unbound forward search response: row has no UUID")
		}
		found[row.UUID] = struct{}{}
	}
	return found, nil
}

func recoverDomainOverrideID(ctx context.Context, client *opnsense.Client, plan DomainOverrideResourceModel, before map[string]struct{}) (string, error) {
	return recoverDomainOverrideIDWithOpts(ctx, client, plan, before, domainOverrideMutationOpts(plan.Type.ValueString()))
}

func recoverDomainOverrideIDWithOpts(ctx context.Context, client *opnsense.Client, plan DomainOverrideResourceModel, before map[string]struct{}, opts opnsense.ReqOpts) (string, error) {
	rows, err := opnsense.Search[domainOverrideSearchRow](ctx, client, opts, opnsense.SearchParams{SearchQuery: plan.Domain.ValueString()})
	if err != nil {
		return "", err
	}
	want := plan.toAPI(ctx)
	match := ""
	for _, row := range rows {
		if row.UUID == "" {
			return "", fmt.Errorf("malformed Unbound forward search response: row has no UUID")
		}
		if _, existed := before[row.UUID]; existed {
			continue
		}
		port := row.Port
		if port == "" {
			port = "53"
			if plan.Type.ValueString() == "dot" {
				port = "853"
			}
		}
		if row.Enabled != want.Enabled || row.Domain != want.Domain || row.Server != want.Server || port != want.Port || row.Verify != want.Verify || row.Description != want.Description {
			continue
		}
		if match != "" {
			return "", fmt.Errorf("multiple newly created Unbound forwards match the requested payload")
		}
		match = row.UUID
	}
	return match, nil
}

// Read refreshes the Terraform state from the OPNsense API.
func (r *domainOverrideResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state DomainOverrideResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	pending, privateDiagnostics := getDomainOverridePending(ctx, req.Private)
	resp.Diagnostics.Append(privateDiagnostics...)
	if resp.Diagnostics.HasError() {
		return
	}
	if pending != "" {
		opts, err := unboundCompatibilityOpts(ctx, r.client, domainOverrideReqOpts, legacyDomainOverrideReqOpts())
		if err != nil {
			resp.Diagnostics.AddError("Unable to check OPNsense compatibility", err.Error())
			return
		}
		if err := opnsense.RetryReconfigure(ctx, r.client, opts); err != nil {
			resp.Diagnostics.AddError("Error activating pending Unbound forward mutation", err.Error())
			return
		}
		resp.Diagnostics.Append(setDomainOverridePending(ctx, resp.Private, "")...)
		if pending == domainOverridePendingDelete {
			resp.State.RemoveResource(ctx)
			return
		}
	}

	result, err := getDomainOverride(ctx, r.client, state.ID.ValueString())
	if err != nil {
		var notFoundErr *opnsense.NotFoundError
		if errors.As(err, &notFoundErr) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError(
			"Error reading Unbound domain override",
			fmt.Sprintf("Could not read Unbound domain override %s: %s", state.ID.ValueString(), err),
		)
		return
	}

	if err := state.fromAPI(ctx, result, state.ID.ValueString()); err != nil {
		resp.Diagnostics.AddError("Error decoding Unbound domain override", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// Update modifies an existing Unbound domain override via the OPNsense API.
func (r *domainOverrideResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan DomainOverrideResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	opts := domainOverrideMutationOpts(plan.Type.ValueString())
	if req.Config.Schema != nil {
		var config DomainOverrideResourceModel
		resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
		if resp.Diagnostics.HasError() {
			return
		}
		var supported bool
		var version string
		var err error
		opts, supported, version, err = domainOverrideRuntimeOpts(ctx, r.client, plan.Type.ValueString())
		if err != nil {
			resp.Diagnostics.AddError("Unable to check OPNsense compatibility", err.Error())
			return
		}
		if domainOverrideUsesParity(config) && !supported {
			resp.Diagnostics.AddError("Unsupported OPNsense version", fmt.Sprintf("Unbound parity fields require stable OPNsense %s or newer; appliance reports %s", unboundParityMinimumVersion, version))
			return
		}
	}

	var state DomainOverrideResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	pending, privateDiagnostics := getDomainOverridePending(ctx, req.Private)
	resp.Diagnostics.Append(privateDiagnostics...)
	if resp.Diagnostics.HasError() {
		return
	}
	if pending != "" {
		if err := opnsense.RetryReconfigure(ctx, r.client, opts); err != nil {
			resp.Diagnostics.AddError("Error activating pending Unbound forward mutation", err.Error())
			return
		}
		resp.Diagnostics.Append(setDomainOverridePending(ctx, resp.Private, "")...)
		if pending == domainOverridePendingDelete {
			// The prior delete is now active, so the UUID no longer exists. An
			// Update callback with a new desired object must recreate it rather
			// than read or set the deleted identity.
			r.create(ctx, plan, opts, resp.Private, resp.State.Set, &resp.Diagnostics)
			return
		}
		matches, remote, err := domainOverrideRemoteMatches(ctx, r.client, state.ID.ValueString(), plan)
		if err != nil {
			resp.Diagnostics.AddError("Error reconciling pending Unbound forward update", err.Error())
			return
		}
		if matches {
			resp.Diagnostics.Append(resp.State.Set(ctx, remote)...)
			return
		}
	}

	apiReq := plan.toAPI(ctx)
	id := state.ID.ValueString()
	// A strict pre-read proves this UUID still exists. The set endpoints can
	// otherwise silently create/adopt an item on an ambiguous type transition.
	if _, _, err := domainOverrideRemoteMatches(ctx, r.client, id, plan); err != nil {
		resp.Diagnostics.AddError("Error preflighting Unbound domain override update", fmt.Sprintf("Could not read %s before update: %s", id, err))
		return
	}

	err := opnsense.Update(ctx, r.client, opts, apiReq, id)
	if err != nil {
		var activationErr *opnsense.MutationReconfigureError
		var validationErr *opnsense.ValidationError
		committed := errors.As(err, &activationErr)
		var confirmed *DomainOverrideResourceModel
		if !committed && !errors.As(err, &validationErr) {
			var confirmErr error
			committed, confirmed, confirmErr = domainOverrideRemoteMatches(ctx, r.client, id, plan)
			if confirmErr != nil {
				resp.Diagnostics.AddError("Unable to confirm Unbound domain override update outcome", confirmErr.Error())
			}
		}
		if committed {
			if confirmed == nil {
				confirmed, _ = readDomainOverrideState(ctx, r.client, id)
			}
			if confirmed == nil {
				confirmed = &state
			}
			resp.Diagnostics.Append(setDomainOverridePending(ctx, resp.Private, domainOverridePendingMutation)...)
			resp.Diagnostics.Append(resp.State.Set(ctx, confirmed)...)
		}
		resp.Diagnostics.AddError(
			"Error updating Unbound domain override",
			fmt.Sprintf("Could not update Unbound domain override %s: %s", id, err),
		)
		return
	}

	// Read back from API to populate state (never echo from config).
	result, err := getDomainOverride(ctx, r.client, id)
	if err != nil {
		resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
		resp.Diagnostics.AddError(
			"Error reading Unbound domain override after update",
			fmt.Sprintf("Updated domain override %s but could not read it back: %s", id, err),
		)
		return
	}

	if err := plan.fromAPI(ctx, result, id); err != nil {
		resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
		resp.Diagnostics.AddError("Error decoding Unbound domain override after update", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func domainOverrideRemoteMatches(ctx context.Context, client *opnsense.Client, id string, plan DomainOverrideResourceModel) (bool, *DomainOverrideResourceModel, error) {
	remote, err := readDomainOverrideState(ctx, client, id)
	if err != nil {
		return false, nil, err
	}
	actual := remote.toAPI(ctx)
	expected := plan.toAPI(ctx)
	return *actual == *expected, remote, nil
}

func readDomainOverrideState(ctx context.Context, client *opnsense.Client, id string) (*DomainOverrideResourceModel, error) {
	result, err := getDomainOverride(ctx, client, id)
	if err != nil {
		return nil, err
	}
	var remote DomainOverrideResourceModel
	if err := remote.fromAPI(ctx, result, id); err != nil {
		return nil, err
	}
	return &remote, nil
}

// Delete removes an Unbound domain override from the OPNsense API.
func (r *domainOverrideResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state DomainOverrideResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	pending, privateDiagnostics := getDomainOverridePending(ctx, req.Private)
	resp.Diagnostics.Append(privateDiagnostics...)
	if resp.Diagnostics.HasError() {
		return
	}
	if pending != "" {
		opts, err := unboundCompatibilityOpts(ctx, r.client, domainOverrideReqOpts, legacyDomainOverrideReqOpts())
		if err != nil {
			resp.Diagnostics.AddError("Unable to check OPNsense compatibility", err.Error())
			return
		}
		if err := opnsense.RetryReconfigure(ctx, r.client, opts); err != nil {
			resp.Diagnostics.AddError("Error activating pending Unbound forward mutation", err.Error())
			return
		}
		resp.Diagnostics.Append(setDomainOverridePending(ctx, resp.Private, "")...)
		if pending == domainOverridePendingDelete {
			return
		}
	}

	opts, _, _, err := domainOverrideRuntimeOpts(ctx, r.client, state.Type.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Unable to check OPNsense compatibility", err.Error())
		return
	}
	err = opnsense.Delete(ctx, r.client, opts, state.ID.ValueString())
	if err != nil {
		var notFoundErr *opnsense.NotFoundError
		if errors.As(err, &notFoundErr) {
			return
		}
		var activationErr *opnsense.MutationReconfigureError
		var validationErr *opnsense.ValidationError
		deleted := errors.As(err, &activationErr)
		if !deleted && !errors.As(err, &validationErr) {
			_, getErr := getDomainOverride(ctx, r.client, state.ID.ValueString())
			var missing *opnsense.NotFoundError
			if errors.As(getErr, &missing) {
				deleted = true
			} else if getErr != nil {
				resp.Diagnostics.AddError("Unable to confirm Unbound domain override delete outcome", getErr.Error())
			}
		}
		if deleted {
			resp.Diagnostics.Append(setDomainOverridePending(ctx, resp.Private, domainOverridePendingDelete)...)
		}
		resp.Diagnostics.AddError(
			"Error deleting Unbound domain override",
			fmt.Sprintf("Could not delete Unbound domain override %s: %s", state.ID.ValueString(), err),
		)
	}
}

// ImportState imports an existing Unbound domain override by UUID.
func (r *domainOverrideResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
