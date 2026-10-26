// Copyright (c) Matthew Mellor
// SPDX-License-Identifier: MPL-2.0

package kea

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"strconv"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/matthew-on-git/terraform-provider-opnsense/pkg/opnsense"
)

const ddnsRecoveryKey = "kea_ddns_recovery_v1"

type ddnsRecovery struct {
	ID      string      `json:"id"`
	Kind    string      `json:"kind"`
	Payload ddnsPayload `json:"payload"`
}

func ddnsMark(ctx context.Context, p optionPrivate, kind string, payload ddnsPayload) diag.Diagnostics {
	var d diag.Diagnostics
	if p == nil || reflect.ValueOf(p).IsNil() {
		d.AddError("Missing durable DDNS recovery state", "Private state is required before a DDNS mutation.")
		return d
	}
	if kind == "" {
		return p.SetKey(ctx, ddnsRecoveryKey, nil)
	}
	raw, err := json.Marshal(ddnsRecovery{ID: "ddns", Kind: kind, Payload: payload})
	if err != nil {
		d.AddError("Invalid DDNS recovery state", err.Error())
		return d
	}
	return p.SetKey(ctx, ddnsRecoveryKey, raw)
}

func ddnsPending(ctx context.Context, p optionPrivate, id string) (ddnsRecovery, diag.Diagnostics) {
	var r ddnsRecovery
	var d diag.Diagnostics
	if id != "ddns" {
		d.AddError("Invalid DDNS identity", "State identity must be exactly ddns.")
		return r, d
	}
	if p == nil || reflect.ValueOf(p).IsNil() {
		return r, d
	}
	raw, diags := p.GetKey(ctx, ddnsRecoveryKey)
	d.Append(diags...)
	if len(raw) == 0 {
		return r, d
	}
	err := json.Unmarshal(raw, &r)
	canonical, canonicalErr := json.Marshal(r)
	if err != nil || canonicalErr != nil || !bytes.Equal(raw, canonical) || r.ID != "ddns" || (r.Kind != optionSaved && r.Kind != optionSaveUnknown) || validateDDNSPayload(r.Payload, false) != nil {
		d.AddError("Invalid DDNS recovery state", "Noncanonical, corrupt, unsupported, identity-mismatched or invalid requested payload; no mutation attempted.")
	}
	return r, d
}

func requireDDNS(ctx context.Context, c *opnsense.Client) error {
	if c == nil {
		return errors.New("DDNS client is not configured")
	}
	if err := c.AcquireRead(ctx); err != nil {
		return err
	}
	defer c.ReleaseRead()
	raw, err := optionRequest(ctx, c, http.MethodGet, "/api/core/firmware/status", nil)
	if err != nil {
		return err
	}
	var status struct {
		Product struct {
			Name    string `json:"product_name"`
			ID      string `json:"product_id"`
			Version string `json:"product_version"`
		} `json:"product"`
	}
	if json.Unmarshal(raw, &status) != nil || (status.Product.Name == "" && status.Product.ID == "") || (status.Product.Name != "" && status.Product.Name != "OPNsense") || (status.Product.ID != "" && status.Product.ID != "opnsense") {
		return errors.New("DDNS requires a verified, noncontradictory OPNsense firmware identity")
	}
	parts := optionStableVersion.FindStringSubmatch(status.Product.Version)
	if parts == nil {
		return errors.New("DDNS requires stable OPNsense 26.7.7 or newer")
	}
	version := [3]uint64{}
	for i := range version {
		if parts[i+1] != "" {
			version[i], err = strconv.ParseUint(parts[i+1], 10, 63)
			if err != nil {
				return err
			}
		}
	}
	if revision := strings.Split(status.Product.Version, "_"); len(revision) > 1 {
		if _, err := strconv.ParseUint(revision[1], 10, 63); err != nil {
			return err
		}
	}
	minimum := [3]uint64{26, 7, 7}
	for i := range version {
		if version[i] > minimum[i] {
			return nil
		}
		if version[i] < minimum[i] {
			return errors.New("DDNS requires stable OPNsense 26.7.7 or newer")
		}
	}
	return nil
}

func readDDNS(ctx context.Context, c *opnsense.Client) (DDNSSettingsResourceModel, error) {
	if err := c.AcquireRead(ctx); err != nil {
		return DDNSSettingsResourceModel{}, err
	}
	defer c.ReleaseRead()
	raw, err := optionRequest(ctx, c, http.MethodGet, "/api/kea/ddns/get", nil)
	if err != nil {
		return DDNSSettingsResourceModel{}, err
	}
	return decodeDDNS(raw)
}

// Recovery always observes first and compares only the requested subset.
func recoverDDNS(ctx context.Context, c *opnsense.Client, pending ddnsRecovery, set func(DDNSSettingsResourceModel) diag.Diagnostics, p optionPrivate) diag.Diagnostics {
	var d diag.Diagnostics
	remote, err := readDDNS(ctx, c)
	if err != nil {
		d.AddError("DDNS recovery read failed", err.Error())
		return d
	}
	d.Append(set(remote)...)
	if d.HasError() {
		return d
	}
	if pending.Kind != "" && ddnsMatches(remote, pending.Payload) {
		if err := activateDHCPv4Option(ctx, c); err != nil {
			d.AddError("Saved DDNS settings require activation", err.Error())
			return d
		}
	} else if pending.Kind != "" {
		d.AddWarning("DDNS save reconciled", "Authoritative fields differ from the requested save. No activation or repeated set; review a fresh plan.")
	}
	d.Append(ddnsMark(context.WithoutCancel(ctx), p, "", nil)...)
	return d
}

func saveDDNS(ctx context.Context, c *opnsense.Client, config DDNSSettingsResourceModel, pending ddnsRecovery, set func(DDNSSettingsResourceModel) diag.Diagnostics, p optionPrivate) diag.Diagnostics {
	var d diag.Diagnostics
	payload, err := config.wireIntent()
	if err != nil {
		d.AddError("Invalid DDNS settings", err.Error())
		return d
	}
	if c == nil {
		d.AddError("Unconfigured DDNS client", "The provider client is unavailable.")
		return d
	}
	if err := c.LockMutex(ctx); err != nil {
		d.AddError("DDNS mutation canceled", err.Error())
		return d
	}
	defer c.UnlockMutex()
	if err := requireDDNS(ctx, c); err != nil {
		d.AddError("DDNS unavailable", err.Error())
		return d
	}
	if pending.Kind != "" {
		d.Append(recoverDDNS(ctx, c, pending, set, p)...)
		if !d.HasError() {
			d.AddError("DDNS recovered; fresh plan required", "Recovery did not repeat the saved mutation. Run a fresh plan and apply.")
		}
		return d
	}
	before, err := readDDNS(ctx, c)
	if err != nil {
		d.AddError("Cannot verify DDNS baseline", err.Error())
		return d
	}
	// Prove durable private storage works before issuing a non-idempotent write.
	d.Append(ddnsMark(ctx, p, "", nil)...)
	if d.HasError() {
		return d
	}
	merged := before.values()
	for k, v := range payload {
		merged[k] = v
	}
	raw, err := optionRequest(ctx, c, http.MethodPost, "/api/kea/ddns/set", map[string]any{"ddns": map[string]any{"general": payload}})
	if err == nil {
		_, err = opnsense.ParseMutationResponse(raw)
	}
	persist := context.WithoutCancel(ctx)
	if err != nil {
		var validation *opnsense.ValidationError
		if errors.As(err, &validation) {
			d.AddError("DDNS validation failed", err.Error())
			return d
		}
		d.Append(set(before)...)
		d.Append(ddnsMark(persist, p, optionSaveUnknown, payload)...)
		d.AddError("DDNS save outcome uncertain", fmt.Sprintf("%v; fixed identity and requested fields retained. Refresh to reconcile without repeating set.", err))
		return d
	}
	d.Append(set(ddnsModel(merged))...)
	d.Append(ddnsMark(persist, p, optionSaved, payload)...)
	if d.HasError() {
		return d
	}
	var observed DDNSSettingsResourceModel
	d.Append(recoverDDNS(ctx, c, ddnsRecovery{ID: "ddns", Kind: optionSaved, Payload: payload}, func(remote DDNSSettingsResourceModel) diag.Diagnostics {
		observed = remote
		return set(remote)
	}, p)...)
	if !d.HasError() && !ddnsMatches(observed, payload) {
		d.AddError("Saved DDNS settings require reconciliation", "Authoritative requested fields differ; pending intent was cleared without activation. Review a fresh plan.")
	}
	return d
}
