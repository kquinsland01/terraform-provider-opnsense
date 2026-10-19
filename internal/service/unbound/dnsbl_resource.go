// Copyright (c) Matthew Mellor
// SPDX-License-Identifier: MPL-2.0

package unbound

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/matthew-on-git/terraform-provider-opnsense/pkg/opnsense"
)

var (
	_ resource.Resource                = &dnsblResource{}
	_ resource.ResourceWithImportState = &dnsblResource{}
)

const (
	dnsblID                   = "unbound-dnsbl"
	dnsblPendingActivationKey = "unbound_dnsbl_pending_activation_v1"
	dnsblItemUUIDKey          = "unbound_dnsbl_item_uuid_v1"
)

var errLegacyDNSBL = errors.New("legacy Unbound DNSBL singleton wire contract")

var dnsblReqOpts = opnsense.ReqOpts{
	AddEndpoint:                 "/api/unbound/settings/add_dnsbl",
	GetEndpoint:                 "/api/unbound/settings/get_dnsbl",
	UpdateEndpoint:              "/api/unbound/settings/set_dnsbl",
	DeleteEndpoint:              "/api/unbound/settings/del_dnsbl",
	SearchEndpoint:              "/api/unbound/settings/search_dnsbl",
	ReconfigureEndpoint:         "/api/unbound/service/reconfigure",
	StrictReconfigureResponse:   true,
	Monad:                       "blocklist",
	StrictDeleteResponse:        true,
	StrictMissingObjectResponse: true,
}

var dnsblSettingsReqOpts = opnsense.ReqOpts{
	GetEndpoint:               "/api/unbound/settings/get",
	UpdateEndpoint:            "/api/unbound/settings/set",
	ReconfigureEndpoint:       "/api/unbound/service/reconfigure",
	StrictReconfigureResponse: true,
	Monad:                     "unbound",
}

var legacyDNSBLReqOpts = opnsense.ReqOpts{
	GetEndpoint:         "/api/unbound/settings/get",
	UpdateEndpoint:      "/api/unbound/settings/set",
	ReconfigureEndpoint: "/api/unbound/service/reconfigure",
	Monad:               "unbound.dnsbl",
}

func withoutReconfigure(options opnsense.ReqOpts) opnsense.ReqOpts {
	options.ReconfigureEndpoint = ""
	return options
}

type dnsblResource struct {
	client     *opnsense.Client
	createLock unboundCreateLocker
}

func newDNSBLResource() resource.Resource { return &dnsblResource{} }

func (r *dnsblResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_unbound_dnsbl"
}

func (r *dnsblResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

// dnsblItemAPIResponse is the current 26.7.7 blocklist item wire shape.
// Selector fields are decoded by SelectedMapList; mutation requests use CSV.
type dnsblItemAPIResponse struct {
	UUID        string                   `json:"uuid"`
	Enabled     string                   `json:"enabled"`
	Type        opnsense.SelectedMapList `json:"type"`
	Lists       opnsense.SelectedMapList `json:"lists"`
	Allowlists  opnsense.SelectedMapList `json:"allowlists"`
	Blocklists  opnsense.SelectedMapList `json:"blocklists"`
	Wildcards   opnsense.SelectedMapList `json:"wildcards"`
	SourceNets  opnsense.SelectedMapList `json:"source_nets"`
	Address     string                   `json:"address"`
	NXDomain    string                   `json:"nxdomain"`
	CacheTTL    string                   `json:"cache_ttl"`
	Description string                   `json:"description"`
}

// UnmarshalJSON keeps provider-wide selector compatibility unchanged while
// enforcing the pinned current-wire contract for this owner.
func (item *dnsblItemAPIResponse) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	decodeString := func(name string, target *string) error {
		value, ok := raw[name]
		if !ok {
			return fmt.Errorf("missing %s", name)
		}
		if err := json.Unmarshal(value, target); err != nil {
			return fmt.Errorf("decode %s: %w", name, err)
		}
		return nil
	}
	for name, target := range map[string]*string{
		"enabled": &item.Enabled, "address": &item.Address, "nxdomain": &item.NXDomain,
		"cache_ttl": &item.CacheTTL, "description": &item.Description,
	} {
		if err := decodeString(name, target); err != nil {
			return err
		}
	}
	if value, ok := raw["uuid"]; ok {
		if err := json.Unmarshal(value, &item.UUID); err != nil {
			return fmt.Errorf("decode uuid: %w", err)
		}
	}
	for name, target := range map[string]*opnsense.SelectedMapList{
		"type": &item.Type, "lists": &item.Lists, "allowlists": &item.Allowlists,
		"blocklists": &item.Blocklists, "wildcards": &item.Wildcards, "source_nets": &item.SourceNets,
	} {
		value, ok := raw[name]
		if !ok {
			return fmt.Errorf("missing %s", name)
		}
		if err := validateSelectedMap(value, false); err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
		if err := json.Unmarshal(value, target); err != nil {
			return fmt.Errorf("decode %s: %w", name, err)
		}
	}
	return nil
}

func validateDNSBLItem(item *dnsblItemAPIResponse, uuid string) error {
	if uuid == "" {
		return errors.New("DNSBL item UUID is empty")
	}
	for name, value := range map[string]string{"enabled": item.Enabled, "nxdomain": item.NXDomain} {
		if value != "0" && value != "1" {
			return fmt.Errorf("DNSBL item %s has invalid boolean %q", name, value)
		}
	}
	ttl, err := strconv.ParseInt(item.CacheTTL, 10, 64)
	if err != nil || ttl < 0 {
		return fmt.Errorf("DNSBL item cache_ttl %q is not a non-negative integer", item.CacheTTL)
	}
	return nil
}

type dnsblItemAPIRequest struct {
	Enabled     string `json:"enabled"`
	Type        string `json:"type"`
	Lists       string `json:"lists"`
	Allowlists  string `json:"allowlists"`
	Blocklists  string `json:"blocklists"`
	Wildcards   string `json:"wildcards"`
	SourceNets  string `json:"source_nets"`
	Address     string `json:"address"`
	NXDomain    string `json:"nxdomain"`
	CacheTTL    string `json:"cache_ttl"`
	Description string `json:"description"`
}

type dnsblSearchRow struct {
	UUID string `json:"uuid"`
}

func dnsblItemRequest(ctx context.Context, model DNSBLResourceModel, existing *dnsblItemAPIResponse) *dnsblItemAPIRequest {
	wire := model.toAPI(ctx)
	request := &dnsblItemAPIRequest{
		Enabled:     wire.Enabled,
		Type:        wire.Type,
		Lists:       wire.Lists,
		Allowlists:  wire.Whitelists,
		Blocklists:  wire.Blocklists,
		Wildcards:   wire.Wildcards,
		Address:     wire.Address,
		NXDomain:    wire.NXDomain,
		CacheTTL:    "72000",
		Description: "Terraform compatibility DNSBL",
	}
	if existing != nil {
		request.SourceNets = opnsense.SliceToCSV(existing.SourceNets)
		request.CacheTTL = existing.CacheTTL
		request.Description = existing.Description
	}
	return request
}

func dnsblModelFromItem(ctx context.Context, item *dnsblItemAPIResponse, uuid, safeSearch string) (DNSBLResourceModel, error) {
	if err := validateDNSBLItem(item, uuid); err != nil {
		return DNSBLResourceModel{}, err
	}
	response := &dnsblAPIResponse{
		Enabled: item.Enabled, SafeSearch: safeSearch, Type: item.Type, Lists: item.Lists,
		Whitelists: item.Allowlists, Blocklists: item.Blocklists, Wildcards: item.Wildcards,
		Address: item.Address, NXDomain: item.NXDomain,
	}
	var model DNSBLResourceModel
	model.fromAPI(ctx, response, dnsblID)
	return model, nil
}

func readDNSBLSafeSearch(ctx context.Context, client *opnsense.Client) (string, error) {
	settings, err := opnsense.GetSingleton[unboundSettingsEnvelope](ctx, client, dnsblSettingsReqOpts)
	if err != nil {
		return "", err
	}
	var general map[string]json.RawMessage
	if err := json.Unmarshal(settings.General, &general); err != nil {
		return "", fmt.Errorf("decode general branch: %w", err)
	}
	var value string
	if raw, ok := general["safesearch"]; !ok || json.Unmarshal(raw, &value) != nil || (value != "0" && value != "1") {
		return "", fmt.Errorf("malformed Unbound settings response: general.safesearch is absent or invalid")
	}
	return value, nil
}

func dnsblModelComplete(model *DNSBLResourceModel) bool {
	if model == nil {
		return false
	}
	return !model.Enabled.IsNull() && !model.Enabled.IsUnknown() && !model.SafeSearch.IsNull() && !model.SafeSearch.IsUnknown() &&
		!model.Type.IsNull() && !model.Type.IsUnknown() && !model.Lists.IsNull() && !model.Lists.IsUnknown() &&
		!model.Whitelists.IsNull() && !model.Whitelists.IsUnknown() && !model.Blocklists.IsNull() && !model.Blocklists.IsUnknown() &&
		!model.Wildcards.IsNull() && !model.Wildcards.IsUnknown() && !model.Address.IsNull() && !model.Address.IsUnknown() &&
		!model.NXDomain.IsNull() && !model.NXDomain.IsUnknown()
}

func discoverDNSBLUUID(ctx context.Context, client *opnsense.Client, prior *DNSBLResourceModel) (string, error) {
	rows, err := searchDNSBL(ctx, client)
	if err != nil {
		return "", err
	}
	switch len(rows) {
	case 0:
		return "", nil
	default:
		if len(rows) > 1 && !dnsblModelComplete(prior) {
			return "", fmt.Errorf("ambiguous DNSBL compatibility ownership: found %d blocklist items and prior state is incomplete", len(rows))
		}
		if len(rows) == 1 && !dnsblModelComplete(prior) {
			if rows[0].UUID == "" {
				return "", fmt.Errorf("malformed DNSBL search response: sole item has no UUID")
			}
			return rows[0].UUID, nil
		}
		matches := make([]string, 0, 1)
		for _, row := range rows {
			if row.UUID == "" {
				return "", errors.New("malformed DNSBL search response: item has no UUID")
			}
			item, err := opnsense.Get[dnsblItemAPIResponse](ctx, client, dnsblReqOpts, row.UUID)
			if err != nil {
				return "", err
			}
			if err := validateDNSBLItem(item, row.UUID); err != nil {
				return "", err
			}
			// SafeSearch is a global general-branch setting, not part of the
			// item identity. Its drift is reconciled independently after the
			// owned blocklist item has been selected.
			if dnsblItemMatchesPlan(ctx, item, *prior) {
				matches = append(matches, row.UUID)
			}
		}
		if len(matches) != 1 {
			return "", fmt.Errorf("ambiguous DNSBL compatibility ownership: %d of %d blocklist items match prior state; expected exactly one", len(matches), len(rows))
		}
		return matches[0], nil
	}
}

func searchDNSBL(ctx context.Context, client *opnsense.Client) ([]dnsblSearchRow, error) {
	rows, err := opnsense.Search[dnsblSearchRow](ctx, client, dnsblReqOpts, opnsense.SearchParams{})
	if err == nil {
		return rows, nil
	}
	var missing *opnsense.NotFoundError
	var missingPlugin *opnsense.PluginNotFoundError
	if !errors.As(err, &missing) && !errors.As(err, &missingPlugin) {
		return nil, err
	}
	supported, _, compatibilityErr := unboundParitySupport(ctx, client)
	if compatibilityErr != nil {
		return nil, fmt.Errorf("%w (compatibility check also failed: %v)", err, compatibilityErr)
	}
	if supported {
		return nil, err
	}
	return nil, errLegacyDNSBL
}

func legacyDNSBLState(ctx context.Context, client *opnsense.Client) (*DNSBLResourceModel, error) {
	result, err := opnsense.GetSingleton[dnsblAPIResponse](ctx, client, legacyDNSBLReqOpts)
	if err != nil {
		return nil, err
	}
	var state DNSBLResourceModel
	state.fromAPI(ctx, result, dnsblID)
	return &state, nil
}

func (r *dnsblResource) applyLegacy(ctx context.Context, plan *DNSBLResourceModel, prior *DNSBLResourceModel, private unboundPrivateState, stateSetter func(context.Context, any) diag.Diagnostics, diagnostics *diag.Diagnostics, action string) {
	err := opnsense.UpdateSingleton(ctx, r.client, legacyDNSBLReqOpts, plan.toAPI(ctx))
	if err != nil {
		var activationErr *opnsense.MutationReconfigureError
		if errors.As(err, &activationErr) {
			*diagnostics = append(*diagnostics, setDNSBLPending(ctx, private, true)...)
			if confirmed, readErr := legacyDNSBLState(ctx, r.client); readErr == nil {
				*diagnostics = append(*diagnostics, stateSetter(ctx, confirmed)...)
			} else if prior != nil {
				*diagnostics = append(*diagnostics, stateSetter(ctx, prior)...)
			}
		}
		diagnostics.AddError("Error "+action+" the legacy DNSBL settings", err.Error())
		return
	}
	state, err := legacyDNSBLState(ctx, r.client)
	if err != nil {
		if prior != nil {
			*diagnostics = append(*diagnostics, stateSetter(ctx, prior)...)
		}
		diagnostics.AddError("Error reading the legacy DNSBL settings after "+action, err.Error())
		return
	}
	*diagnostics = append(*diagnostics, setDNSBLPending(ctx, private, false)...)
	*diagnostics = append(*diagnostics, stateSetter(ctx, state)...)
}

func dnsblItemMatchesPlan(ctx context.Context, item *dnsblItemAPIResponse, plan DNSBLResourceModel) bool {
	want := plan.toAPI(ctx)
	return item.Enabled == want.Enabled && opnsense.SliceToCSV(item.Type) == want.Type &&
		opnsense.SliceToCSV(item.Lists) == want.Lists && opnsense.SliceToCSV(item.Allowlists) == want.Whitelists &&
		opnsense.SliceToCSV(item.Blocklists) == want.Blocklists && opnsense.SliceToCSV(item.Wildcards) == want.Wildcards &&
		item.Address == want.Address && item.NXDomain == want.NXDomain
}

func getDNSBLUUID(ctx context.Context, private unboundPrivateState) (string, diag.Diagnostics) {
	if private == nil {
		return "", nil
	}
	value, diagnostics := private.GetKey(ctx, dnsblItemUUIDKey)
	if len(value) == 0 || diagnostics.HasError() {
		return "", diagnostics
	}
	var uuid string
	if err := json.Unmarshal(value, &uuid); err != nil {
		diagnostics.AddError("Invalid private DNSBL item identity", err.Error())
	}
	return uuid, diagnostics
}

func setDNSBLUUID(ctx context.Context, private unboundPrivateState, uuid string) diag.Diagnostics {
	if private == nil {
		return nil
	}
	encoded, err := json.Marshal(uuid)
	if err != nil {
		return diag.Diagnostics{diag.NewErrorDiagnostic("Unable to encode private DNSBL item identity", err.Error())}
	}
	return private.SetKey(ctx, dnsblItemUUIDKey, encoded)
}

func setDNSBLPending(ctx context.Context, private unboundPrivateState, pending bool) diag.Diagnostics {
	if private == nil {
		return nil
	}
	if pending {
		return private.SetKey(ctx, dnsblPendingActivationKey, []byte("true"))
	}
	return private.SetKey(ctx, dnsblPendingActivationKey, nil)
}

func getDNSBLPending(ctx context.Context, private unboundPrivateState) (bool, diag.Diagnostics) {
	if private == nil {
		return false, nil
	}
	value, diagnostics := private.GetKey(ctx, dnsblPendingActivationKey)
	if len(value) == 0 || diagnostics.HasError() {
		return false, diagnostics
	}
	var pending bool
	if err := json.Unmarshal(value, &pending); err != nil {
		diagnostics.AddError("Invalid private DNSBL pending state", err.Error())
	}
	return pending, diagnostics
}

func updateDNSBLSafeSearch(ctx context.Context, client *opnsense.Client, desired string) error {
	current, err := readDNSBLSafeSearch(ctx, client)
	if err != nil {
		return err
	}
	if current == desired {
		return nil
	}
	payload := map[string]any{"general": map[string]any{"safesearch": desired}}
	settingsMutationOpts := withoutReconfigure(dnsblSettingsReqOpts)
	return opnsense.UpdateSingleton(ctx, client, settingsMutationOpts, &payload)
}

func (r *dnsblResource) readOwned(ctx context.Context, private unboundPrivateState, prior *DNSBLResourceModel) (*dnsblItemAPIResponse, string, string, diag.Diagnostics, error) {
	uuid, diagnostics := getDNSBLUUID(ctx, private)
	if diagnostics.HasError() {
		return nil, "", "", diagnostics, nil
	}
	if uuid == "" {
		var err error
		uuid, err = discoverDNSBLUUID(ctx, r.client, prior)
		if err != nil {
			return nil, "", "", diagnostics, err
		}
		if uuid == "" {
			return nil, "", "", diagnostics, &opnsense.NotFoundError{Message: "DNSBL compatibility item not found"}
		}
		diagnostics.Append(setDNSBLUUID(ctx, private, uuid)...)
		if diagnostics.HasError() {
			return nil, uuid, "", diagnostics, nil
		}
	}
	item, err := opnsense.Get[dnsblItemAPIResponse](ctx, r.client, dnsblReqOpts, uuid)
	if err != nil {
		return nil, uuid, "", diagnostics, err
	}
	if err := validateDNSBLItem(item, uuid); err != nil {
		return nil, uuid, "", diagnostics, err
	}
	safeSearch, err := readDNSBLSafeSearch(ctx, r.client)
	return item, uuid, safeSearch, diagnostics, err
}

func minimalDNSBLState() DNSBLResourceModel {
	return DNSBLResourceModel{ID: types.StringValue(dnsblID), Type: types.SetNull(types.StringType), Lists: types.SetNull(types.StringType), Whitelists: types.SetNull(types.StringType), Blocklists: types.SetNull(types.StringType), Wildcards: types.SetNull(types.StringType)}
}

func (r *dnsblResource) authoritativeState(ctx context.Context, uuid string) (*DNSBLResourceModel, error) {
	item, err := opnsense.Get[dnsblItemAPIResponse](ctx, r.client, dnsblReqOpts, uuid)
	if err != nil {
		return nil, err
	}
	safeSearch, err := readDNSBLSafeSearch(ctx, r.client)
	if err != nil {
		return nil, err
	}
	state, err := dnsblModelFromItem(ctx, item, uuid, safeSearch)
	return &state, err
}

func setDNSBLConfirmedState(ctx context.Context, setter func(context.Context, any) diag.Diagnostics, confirmed, fallback *DNSBLResourceModel) diag.Diagnostics {
	if confirmed != nil {
		return setter(ctx, confirmed)
	}
	if fallback != nil {
		return setter(ctx, fallback)
	}
	minimal := minimalDNSBLState()
	return setter(ctx, &minimal)
}

func (r *dnsblResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan DNSBLResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	createLock := ownerCreateLocker(r.createLock)
	createLock.Lock()
	defer createLock.Unlock()

	before, err := searchDNSBL(ctx, r.client)
	if err != nil {
		if errors.Is(err, errLegacyDNSBL) {
			r.applyLegacy(ctx, &plan, nil, resp.Private, resp.State.Set, &resp.Diagnostics, "applying")
			return
		}
		resp.Diagnostics.AddError("Error discovering DNSBL compatibility item", err.Error())
		return
	}
	if len(before) > 0 {
		resp.Diagnostics.AddError(
			"Existing DNSBL item requires import",
			fmt.Sprintf("found %d existing blocklist item(s); import %q to explicitly adopt a sole existing item, or remove the conflicting items before create", len(before), dnsblID),
		)
		return
	}
	uuid, err := opnsense.Add(ctx, r.client, withoutReconfigure(dnsblReqOpts), dnsblItemRequest(ctx, plan, nil))
	if uuid != "" {
		resp.Diagnostics.Append(setDNSBLUUID(ctx, resp.Private, uuid)...)
		if resp.Diagnostics.HasError() {
			minimal := minimalDNSBLState()
			resp.Diagnostics.Append(resp.State.Set(ctx, &minimal)...)
			return
		}
	}
	if err != nil {
		var activationErr *opnsense.MutationReconfigureError
		var validationErr *opnsense.ValidationError
		committed := errors.As(err, &activationErr)
		if !committed && !errors.As(err, &validationErr) {
			if uuid == "" {
				after, recoveryErr := searchDNSBL(ctx, r.client)
				if recoveryErr != nil {
					resp.Diagnostics.AddError("Unable to confirm DNSBL create outcome", recoveryErr.Error())
				} else if len(after) == 1 && after[0].UUID != "" {
					readBack, readErr := opnsense.Get[dnsblItemAPIResponse](ctx, r.client, dnsblReqOpts, after[0].UUID)
					if readErr == nil && validateDNSBLItem(readBack, after[0].UUID) == nil && dnsblItemMatchesPlan(ctx, readBack, plan) {
						uuid = after[0].UUID
						committed = true
					}
				}
			} else if readBack, recoveryErr := opnsense.Get[dnsblItemAPIResponse](ctx, r.client, dnsblReqOpts, uuid); recoveryErr == nil {
				committed = dnsblItemMatchesPlan(ctx, readBack, plan)
			}
		}
		if committed {
			resp.Diagnostics.Append(setDNSBLUUID(ctx, resp.Private, uuid)...)
			if resp.Diagnostics.HasError() {
				minimal := minimalDNSBLState()
				resp.Diagnostics.Append(resp.State.Set(ctx, &minimal)...)
				return
			}
			confirmed, confirmErr := r.authoritativeState(ctx, uuid)
			if confirmErr != nil {
				resp.Diagnostics.AddError("Unable to read committed DNSBL create state", confirmErr.Error())
			}
			resp.Diagnostics.Append(setDNSBLPending(ctx, resp.Private, true)...)
			resp.Diagnostics.Append(setDNSBLConfirmedState(ctx, resp.State.Set, confirmed, nil)...)
		}
		resp.Diagnostics.AddError("Error applying DNSBL item", err.Error())
		return
	}
	if err := updateDNSBLSafeSearch(ctx, r.client, plan.toAPI(ctx).SafeSearch); err != nil {
		confirmed, _ := r.authoritativeState(ctx, uuid)
		resp.Diagnostics.Append(setDNSBLPending(ctx, resp.Private, true)...)
		resp.Diagnostics.Append(setDNSBLConfirmedState(ctx, resp.State.Set, confirmed, nil)...)
		resp.Diagnostics.AddError("Error applying DNSBL SafeSearch setting", err.Error())
		return
	}
	if err := opnsense.RetryReconfigure(ctx, r.client, dnsblReqOpts); err != nil {
		confirmed, _ := r.authoritativeState(ctx, uuid)
		resp.Diagnostics.Append(setDNSBLPending(ctx, resp.Private, true)...)
		resp.Diagnostics.Append(setDNSBLConfirmedState(ctx, resp.State.Set, confirmed, nil)...)
		resp.Diagnostics.AddError("Error activating DNSBL settings", err.Error())
		return
	}
	state, err := r.authoritativeState(ctx, uuid)
	if err != nil {
		resp.Diagnostics.Append(setDNSBLConfirmedState(ctx, resp.State.Set, nil, nil)...)
		resp.Diagnostics.AddError("Error reading DNSBL settings after apply", err.Error())
		return
	}
	resp.Diagnostics.Append(setDNSBLPending(ctx, resp.Private, false)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (r *dnsblResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var prior DNSBLResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &prior)...)
	if resp.Diagnostics.HasError() {
		return
	}
	pending, privateDiagnostics := getDNSBLPending(ctx, req.Private)
	resp.Diagnostics.Append(privateDiagnostics...)
	if resp.Diagnostics.HasError() {
		return
	}
	if pending {
		opts, err := unboundCompatibilityOpts(ctx, r.client, dnsblReqOpts, legacyDNSBLReqOpts)
		if err != nil {
			resp.Diagnostics.AddError("Unable to check OPNsense compatibility", err.Error())
			return
		}
		if err := opnsense.RetryReconfigure(ctx, r.client, opts); err != nil {
			resp.Diagnostics.AddError("Error activating pending DNSBL settings mutation", err.Error())
			return
		}
		resp.Diagnostics.Append(setDNSBLPending(ctx, resp.Private, false)...)
	}
	item, uuid, safeSearch, diagnostics, err := r.readOwned(ctx, resp.Private, &prior)
	resp.Diagnostics.Append(diagnostics...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err != nil {
		if errors.Is(err, errLegacyDNSBL) {
			state, legacyErr := legacyDNSBLState(ctx, r.client)
			if legacyErr != nil {
				resp.Diagnostics.AddError("Error reading legacy DNSBL settings", legacyErr.Error())
				return
			}
			resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
			return
		}
		var notFound *opnsense.NotFoundError
		if errors.As(err, &notFound) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading DNSBL settings", err.Error())
		return
	}
	state, err := dnsblModelFromItem(ctx, item, uuid, safeSearch)
	if err != nil {
		resp.Diagnostics.AddError("Error decoding DNSBL settings", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *dnsblResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan DNSBLResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var prior DNSBLResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &prior)...)
	if resp.Diagnostics.HasError() {
		return
	}
	pending, privateDiagnostics := getDNSBLPending(ctx, req.Private)
	resp.Diagnostics.Append(privateDiagnostics...)
	if resp.Diagnostics.HasError() {
		return
	}
	if pending {
		opts, err := unboundCompatibilityOpts(ctx, r.client, dnsblReqOpts, legacyDNSBLReqOpts)
		if err != nil {
			resp.Diagnostics.AddError("Unable to check OPNsense compatibility", err.Error())
			return
		}
		if err := opnsense.RetryReconfigure(ctx, r.client, opts); err != nil {
			resp.Diagnostics.AddError("Error activating pending DNSBL settings mutation", err.Error())
			return
		}
		resp.Diagnostics.Append(setDNSBLPending(ctx, resp.Private, false)...)
	}
	item, uuid, safeSearch, diagnostics, err := r.readOwned(ctx, req.Private, &prior)
	resp.Diagnostics.Append(diagnostics...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err != nil {
		if errors.Is(err, errLegacyDNSBL) {
			r.applyLegacy(ctx, &plan, &prior, resp.Private, resp.State.Set, &resp.Diagnostics, "updating")
			return
		}
		resp.Diagnostics.AddError("Error reading DNSBL settings before update", err.Error())
		return
	}
	itemMatches := dnsblItemMatchesPlan(ctx, item, plan)
	if !itemMatches {
		err = opnsense.Update(ctx, r.client, withoutReconfigure(dnsblReqOpts), dnsblItemRequest(ctx, plan, item), uuid)
	}
	if err != nil {
		var activationErr *opnsense.MutationReconfigureError
		var validationErr *opnsense.ValidationError
		committed := errors.As(err, &activationErr)
		if !committed && !errors.As(err, &validationErr) {
			if readBack, recoveryErr := opnsense.Get[dnsblItemAPIResponse](ctx, r.client, dnsblReqOpts, uuid); recoveryErr == nil {
				committed = dnsblItemMatchesPlan(ctx, readBack, plan)
			} else {
				resp.Diagnostics.AddError("Unable to confirm DNSBL update outcome", recoveryErr.Error())
			}
		}
		if committed {
			confirmed, _ := r.authoritativeState(ctx, uuid)
			resp.Diagnostics.Append(setDNSBLPending(ctx, resp.Private, true)...)
			resp.Diagnostics.Append(setDNSBLConfirmedState(ctx, resp.State.Set, confirmed, &prior)...)
		}
		resp.Diagnostics.AddError("Error updating DNSBL item", err.Error())
		return
	}
	if safeSearch != plan.toAPI(ctx).SafeSearch {
		if err := updateDNSBLSafeSearch(ctx, r.client, plan.toAPI(ctx).SafeSearch); err != nil {
			confirmed, _ := r.authoritativeState(ctx, uuid)
			resp.Diagnostics.Append(setDNSBLPending(ctx, resp.Private, true)...)
			resp.Diagnostics.Append(setDNSBLConfirmedState(ctx, resp.State.Set, confirmed, &prior)...)
			resp.Diagnostics.AddError("Error updating DNSBL SafeSearch setting", err.Error())
			return
		}
	}
	if !itemMatches || safeSearch != plan.toAPI(ctx).SafeSearch {
		if err := opnsense.RetryReconfigure(ctx, r.client, dnsblReqOpts); err != nil {
			confirmed, _ := r.authoritativeState(ctx, uuid)
			resp.Diagnostics.Append(setDNSBLPending(ctx, resp.Private, true)...)
			resp.Diagnostics.Append(setDNSBLConfirmedState(ctx, resp.State.Set, confirmed, &prior)...)
			resp.Diagnostics.AddError("Error activating DNSBL settings update", err.Error())
			return
		}
	}
	state, err := r.authoritativeState(ctx, uuid)
	if err != nil {
		resp.Diagnostics.Append(resp.State.Set(ctx, &prior)...)
		resp.Diagnostics.AddError("Error reading DNSBL settings after update", err.Error())
		return
	}
	resp.Diagnostics.Append(setDNSBLPending(ctx, resp.Private, false)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

// Delete is intentionally state-only: this compatibility singleton never
// removes appliance settings, including the private item UUID it owns.
func (r *dnsblResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	pending, diagnostics := getDNSBLPending(ctx, req.Private)
	resp.Diagnostics.Append(diagnostics...)
	if resp.Diagnostics.HasError() || !pending {
		return
	}
	opts, err := unboundCompatibilityOpts(ctx, r.client, dnsblReqOpts, legacyDNSBLReqOpts)
	if err != nil {
		resp.Diagnostics.AddError("Unable to check OPNsense compatibility", err.Error())
		return
	}
	if err := opnsense.RetryReconfigure(ctx, r.client, opts); err != nil {
		resp.Diagnostics.AddError("Error activating pending DNSBL settings mutation before state removal", err.Error())
		return
	}
	resp.Diagnostics.Append(setDNSBLPending(ctx, resp.Private, false)...)
}

func (r *dnsblResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	if req.ID != dnsblID {
		resp.Diagnostics.AddError("Invalid import identifier", fmt.Sprintf("use %q for this singleton, got %q", dnsblID, req.ID))
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), types.StringValue(dnsblID))...)
}
