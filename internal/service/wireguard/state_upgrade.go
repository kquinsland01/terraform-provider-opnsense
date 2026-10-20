// Copyright (c) Matthew Mellor
// SPDX-License-Identifier: MPL-2.0

package wireguard

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
)

func upgradeLegacy(ctx context.Context, req resource.UpgradeStateRequest, resp *resource.UpgradeStateResponse, kind string) {
	if req.RawState == nil {
		resp.Diagnostics.AddError("Missing legacy WireGuard state", "Cannot upgrade absent state.")
		return
	}
	var state map[string]json.RawMessage
	if err := json.Unmarshal(req.RawState.JSON, &state); err != nil {
		resp.Diagnostics.AddError("Malformed legacy WireGuard state", err.Error())
		return
	}
	for _, field := range []string{"tunnel_address", "servers"} {
		if kind == "server" && field == "servers" {
			continue
		}
		raw, ok := state[field]
		if !ok {
			continue
		}
		var csv *string
		if err := json.Unmarshal(raw, &csv); err != nil {
			resp.Diagnostics.AddError("Malformed legacy WireGuard state", fmt.Sprintf("%s must be a string", field))
			return
		}
		members := []string{}
		if csv != nil && *csv != "" {
			members = strings.Split(*csv, ",")
			for _, member := range members {
				if member == "" || strings.TrimSpace(member) != member {
					resp.Diagnostics.AddError("Malformed legacy WireGuard CSV", field+" contains an empty or whitespace-padded member")
					return
				}
			}
		}
		if field == "tunnel_address" {
			if err := validateCIDRs(field, members); err != nil {
				resp.Diagnostics.AddError("Malformed legacy WireGuard CSV", err.Error())
				return
			}
		}
		state[field], _ = json.Marshal(members)
	}
	number := "server_port"
	if kind == "server" {
		number = "port"
	}
	if raw, ok := state[number]; ok {
		var value *string
		if err := json.Unmarshal(raw, &value); err != nil {
			resp.Diagnostics.AddError("Malformed legacy WireGuard port", err.Error())
			return
		}
		state[number] = json.RawMessage("null")
		if value != nil && *value != "" {
			n, err := strconv.ParseInt(*value, 10, 64)
			if err != nil || n < 1 || n > 65535 {
				resp.Diagnostics.AddError("Malformed legacy WireGuard port", "Port must be in 1..65535")
				return
			}
			state[number], _ = json.Marshal(n)
		}
	}
	// Newly introduced fields begin null; refresh obtains authoritative defaults.
	attributes := peerSchemaV1().Attributes
	if kind == "server" {
		attributes = serverSchemaV1().Attributes
	}
	for key := range attributes {
		if _, ok := state[key]; !ok {
			state[key] = json.RawMessage("null")
		}
	}
	encoded, err := json.Marshal(state)
	if err != nil {
		resp.Diagnostics.AddError("Error upgrading WireGuard state", err.Error())
		return
	}
	dynamic := tfprotov6.DynamicValue{JSON: encoded}
	value, err := dynamic.Unmarshal(resp.State.Schema.Type().TerraformType(ctx))
	if err != nil {
		resp.Diagnostics.AddError("Error upgrading WireGuard state", err.Error())
		return
	}
	resp.State.Raw = value
}

func (r *peerResource) UpgradeState(_ context.Context) map[int64]resource.StateUpgrader {
	return map[int64]resource.StateUpgrader{0: {StateUpgrader: func(ctx context.Context, req resource.UpgradeStateRequest, resp *resource.UpgradeStateResponse) {
		upgradeLegacy(ctx, req, resp, "peer")
	}}}
}

func (r *serverResource) UpgradeState(_ context.Context) map[int64]resource.StateUpgrader {
	return map[int64]resource.StateUpgrader{0: {StateUpgrader: func(ctx context.Context, req resource.UpgradeStateRequest, resp *resource.UpgradeStateResponse) {
		upgradeLegacy(ctx, req, resp, "server")
	}}}
}
