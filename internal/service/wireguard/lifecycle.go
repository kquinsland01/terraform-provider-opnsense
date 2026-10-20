// Copyright (c) Matthew Mellor
// SPDX-License-Identifier: MPL-2.0

package wireguard

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"sync"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/matthew-on-git/terraform-provider-opnsense/pkg/opnsense"
)

var ownerMu sync.Mutex

const (
	pendingActivationStateName = "wireguard_pending_activation_v1"
	pendingMutation            = `"mutation"`
	pendingDelete              = `"delete"`
	pendingDeleteUnknown       = `"delete_unknown"`
)

type privateState interface {
	GetKey(context.Context, string) ([]byte, diag.Diagnostics)
	SetKey(context.Context, string, []byte) diag.Diagnostics
}

func getPending(ctx context.Context, p privateState) (string, diag.Diagnostics) {
	if p == nil || reflect.ValueOf(p).IsNil() {
		return "", nil
	}
	v, d := p.GetKey(ctx, pendingActivationStateName)
	if len(v) > 0 && string(v) != pendingMutation && string(v) != pendingDelete && string(v) != pendingDeleteUnknown {
		d.AddError("Invalid WireGuard pending activation state", "Expected mutation, delete, or delete_unknown marker.")
	}
	return string(v), d
}

func setPending(ctx context.Context, p privateState, value string) diag.Diagnostics {
	if p == nil || reflect.ValueOf(p).IsNil() {
		return nil
	}
	if value == "" {
		return p.SetKey(ctx, pendingActivationStateName, nil)
	}
	return p.SetKey(ctx, pendingActivationStateName, []byte(value))
}

type searchIdentity struct {
	UUID string `json:"uuid"`
	Name string `json:"name"`
}

func createOwned[K any](ctx context.Context, client *opnsense.Client, opts opnsense.ReqOpts, payload *K) (string, error) {
	encoded, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	var fields map[string]string
	if err = json.Unmarshal(encoded, &fields); err != nil {
		return "", err
	}
	before, err := opnsense.Search[searchIdentity](ctx, client, opts, opnsense.SearchParams{})
	if err != nil {
		return "", err
	}
	known := make(map[string]bool)
	for _, row := range before {
		if row.UUID == "" {
			return "", errors.New("WireGuard search returned an empty UUID")
		}
		known[row.UUID] = true
		if row.Name == fields["name"] {
			return "", errors.New("WireGuard name already exists; import the existing object instead")
		}
	}
	id, mutationErr := opnsense.Add(ctx, client, opts, payload)
	if id != "" || mutationErr == nil {
		return id, mutationErr
	}
	var validation *opnsense.ValidationError
	if errors.As(mutationErr, &validation) {
		return "", mutationErr
	}
	after, err := opnsense.Search[searchIdentity](ctx, client, opts, opnsense.SearchParams{})
	if err != nil {
		return "", fmt.Errorf("mutation outcome is ambiguous; recovery search failed: %w", err)
	}
	matches := []string{}
	for _, row := range after {
		if known[row.UUID] || row.Name != fields["name"] {
			continue
		}
		remote, readErr := opnsense.Get[map[string]json.RawMessage](ctx, client, opts, row.UUID)
		if readErr != nil {
			return "", fmt.Errorf("recovery read failed: %w", readErr)
		}
		match := true
		for key, want := range fields {
			if key == "instance" && want == "" {
				continue
			}
			if key == "pubkey" && want == "" {
				continue
			}
			if key == "description" {
				continue
			} // not a released model field
			raw, exists := (*remote)[key]
			if !exists && (key == "psk" || key == "privkey") {
				continue
			}
			if !exists {
				match = false
				break
			}
			if key == "psk" || key == "privkey" {
				var secret string
				if json.Unmarshal(raw, &secret) != nil || (secret != "" && secret != want) {
					match = false
				}
				continue
			}
			switch key {
			case "tunneladdress", "dns", "peers", "servers":
				got, err := normalizeRecoverySet(key, raw)
				expectedJSON, _ := json.Marshal(want)
				expected, wantErr := normalizeRecoverySet(key, expectedJSON)
				if err != nil || wantErr != nil || got != expected {
					match = false
				}
			default:
				var scalar string
				if json.Unmarshal(raw, &scalar) != nil || scalar != want {
					match = false
				}
			}
		}
		if match {
			matches = append(matches, row.UUID)
		}
	}
	if len(matches) != 1 {
		return "", fmt.Errorf("ambiguous WireGuard create: %d exact recovery matches; reconcile appliance objects before retrying", len(matches))
	}
	return matches[0], fmt.Errorf("saved WireGuard object recovered after ambiguous response: %w", mutationErr)
}

// Resolve only omitted Optional+Computed numeric intent. A configured expression
// that remains unknown must still fail closed rather than becoming a clear.
func resolveOmittedNumber(ctx context.Context, config tfsdk.Config, name string, value *types.Int64) diag.Diagnostics {
	if !value.IsUnknown() || config.Schema == nil {
		return nil
	}
	var configured types.Int64
	d := config.GetAttribute(ctx, path.Root(name), &configured)
	if !d.HasError() && configured.IsNull() {
		*value = types.Int64Null()
	}
	return d
}

const secretReconciliationName = "wireguard_secret_reconciliation_v1"

// An ambiguous save cannot confirm either the old or the planned write-only
// secret. Retain both privately, with a null public value until authoritative
// read-back can resolve it; never repeat the possibly committed save.
func markSecretAmbiguous(ctx context.Context, p privateState, old, planned types.String) diag.Diagnostics {
	raw, _ := json.Marshal(map[string]string{"prior": old.ValueString(), "planned": planned.ValueString()})
	if p == nil || reflect.ValueOf(p).IsNil() {
		return nil
	}
	return p.SetKey(ctx, secretReconciliationName, raw)
}

func reconcileSecret(ctx context.Context, p privateState, remote string, value *types.String) diag.Diagnostics {
	if p == nil || reflect.ValueOf(p).IsNil() {
		return nil
	}
	raw, d := p.GetKey(ctx, secretReconciliationName)
	if d.HasError() || len(raw) == 0 {
		return d
	}
	if remote == "" {
		*value = types.StringNull()
		d.AddError("WireGuard secret outcome is unconfirmed", "A prior update may have saved a write-only secret, but API read-back cannot confirm it. Automatic confirmation is impossible when the API omits the key. Independently verify/reconcile the appliance key, securely back up state, record this exact UUID, then use supported state removal and re-import of that same UUID to reset private uncertainty; configure the verified key and review a fresh plan. See the resource recovery guide. The provider will not repeat the ambiguous mutation or claim either key as confirmed.")
		return d
	}
	*value = types.StringValue(remote)
	d.Append(p.SetKey(ctx, secretReconciliationName, nil)...)
	return d
}

// Config must be inspected even when a plan modifier has already reused prior
// state. Explicit unresolved key intent is never an omission or a confirmed key.
func validateConfiguredKey(ctx context.Context, config tfsdk.Config, name string, planned types.String) diag.Diagnostics {
	if config.Schema == nil {
		return nil
	}
	var configured types.String
	d := config.GetAttribute(ctx, path.Root(name), &configured)
	if !d.HasError() && (configured.IsUnknown() || (!configured.IsNull() && planned.IsUnknown())) {
		d.AddError("Unresolved WireGuard key intent", name+" is explicitly configured but unresolved; no appliance request was made. Resolve the configured value before applying.")
	}
	return d
}

func normalizeRecoverySet(field string, raw json.RawMessage) (string, error) {
	var members strictSelectedMapList
	if err := json.Unmarshal(raw, &members); err != nil {
		return "", err
	}
	var err error
	switch field {
	case "tunneladdress":
		err = validateCIDRs(field, []string(members))
	case "dns":
		err = validateIPs(field, []string(members))
	case "peers", "servers":
		err = validateUUIDs(field, []string(members))
	}
	if err != nil {
		return "", err
	}
	unique := map[string]bool{}
	values := []string{}
	for _, member := range members {
		if !unique[member] {
			unique[member] = true
			values = append(values, member)
		}
	}
	sort.Strings(values)
	return strings.Join(values, ","), nil
}
