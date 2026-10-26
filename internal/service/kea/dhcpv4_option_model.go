// Copyright (c) Matthew Mellor
// SPDX-License-Identifier: MPL-2.0

package kea

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

var optionEncodings = []string{"hex", "ipv4-address", "ipv6-address", "uint8", "uint16", "uint32", "int32", "boolean", "string", "fqdn"}

// DHCPv4OptionResourceModel owns one independent option, not its parent links.
type DHCPv4OptionResourceModel struct {
	ID            types.String `tfsdk:"id"`
	Code          types.String `tfsdk:"code"`
	Encoding      types.String `tfsdk:"encoding"`
	Data          types.String `tfsdk:"data"`
	Force         types.Bool   `tfsdk:"force"`
	MatchCode     types.String `tfsdk:"match_code"`
	MatchEncoding types.String `tfsdk:"match_encoding"`
	MatchData     types.String `tfsdk:"match_data"`
	Description   types.String `tfsdk:"description"`
}

type optionPayload map[string]string

func optionContains(values []string, value string) bool {
	for _, v := range values {
		if v == value {
			return true
		}
	}
	return false
}

func optionCodeValid(value string) bool {
	n, err := strconv.Atoi(value)
	return err == nil && n >= 1 && n <= 254 && strconv.Itoa(n) == value
}

func validateOptionPayload(p optionPayload, complete bool) error {
	for key, value := range p {
		switch key {
		case "code":
			if !optionCodeValid(value) {
				return fmt.Errorf("code must be a DHCPv4 selector key from 1 through 254")
			}
		case "data", "description":
			if value == "" {
				return fmt.Errorf("%s must not be empty", key)
			}
		case "encoding":
			if !optionContains(optionEncodings, value) {
				return fmt.Errorf("invalid encoding %q", value)
			}
		case "force":
			if value != "0" && value != "1" {
				return fmt.Errorf("force must be 0 or 1")
			}
		case "match_code":
			if value != "" && !optionCodeValid(value) {
				return fmt.Errorf("invalid match_code")
			}
		case "match_encoding":
			if value != "" && !optionContains(optionEncodings, value) {
				return fmt.Errorf("invalid match_encoding")
			}
		case "match_data":
		default:
			return fmt.Errorf("unexpected option field %q", key)
		}
	}
	for _, key := range []string{"code", "data", "description"} {
		if _, ok := p[key]; !ok {
			return fmt.Errorf("missing required %s", key)
		}
	}
	code, hasCode := p["match_code"]
	data, hasData := p["match_data"]
	encoding, hasEncoding := p["match_encoding"]
	if hasCode && hasData && (code == "") != (data == "") {
		return fmt.Errorf("match_code and match_data must both be populated or both empty")
	}
	if hasData && hasEncoding && data != "" && encoding == "" {
		return fmt.Errorf("match_encoding is required with match_data")
	}
	if complete {
		if len(p) != 8 {
			return fmt.Errorf("incomplete option model")
		}
		if (p["match_code"] == "") != (p["match_data"] == "") {
			return fmt.Errorf("match_code and match_data must both be populated or both empty")
		}
		if p["match_data"] != "" && p["match_encoding"] == "" {
			return fmt.Errorf("match_encoding is required with match_data")
		}
	}
	return nil
}

func (m DHCPv4OptionResourceModel) values() (optionPayload, error) {
	p := optionPayload{}
	for k, v := range map[string]types.String{"code": m.Code, "encoding": m.Encoding, "data": m.Data, "match_code": m.MatchCode, "match_encoding": m.MatchEncoding, "match_data": m.MatchData, "description": m.Description} {
		if v.IsNull() || v.IsUnknown() {
			return nil, fmt.Errorf("%s must be known and non-null before mutation", k)
		}
		p[k] = v.ValueString()
	}
	if m.Force.IsNull() || m.Force.IsUnknown() {
		return nil, fmt.Errorf("force must be known and non-null before mutation")
	}
	p["force"] = "0"
	if m.Force.ValueBool() {
		p["force"] = "1"
	}
	return p, validateOptionPayload(p, true)
}

// wireIntent consults raw configuration: computed/prior values are never an
// instruction to serialize omitted optional fields during an update.
func (m DHCPv4OptionResourceModel) wireIntent(config DHCPv4OptionResourceModel, create bool) (optionPayload, error) {
	p, err := m.values()
	if err != nil {
		return nil, err
	}
	for key, value := range map[string]types.String{"encoding": config.Encoding, "match_code": config.MatchCode, "match_encoding": config.MatchEncoding, "match_data": config.MatchData} {
		if value.IsUnknown() {
			return nil, fmt.Errorf("%s is unresolved", key)
		}
		if !create && value.IsNull() {
			delete(p, key)
		}
	}
	if config.Force.IsUnknown() {
		return nil, fmt.Errorf("force is unresolved")
	}
	if !create && config.Force.IsNull() {
		delete(p, "force")
	}
	return p, nil
}

func decodeOption(fields map[string]json.RawMessage, id string, template bool) (DHCPv4OptionResourceModel, error) {
	m := DHCPv4OptionResourceModel{ID: types.StringValue(id)}
	for key, dest := range map[string]*types.String{"code": &m.Code, "encoding": &m.Encoding, "match_code": &m.MatchCode, "match_encoding": &m.MatchEncoding} {
		value, _, err := optionSelection(fields[key], key, template)
		if err != nil {
			return m, fmt.Errorf("%s: %w", key, err)
		}
		*dest = types.StringValue(value)
	}
	for key, dest := range map[string]*types.String{"data": &m.Data, "match_data": &m.MatchData, "description": &m.Description} {
		var value *string
		if json.Unmarshal(fields[key], &value) != nil || value == nil {
			return m, fmt.Errorf("missing or malformed %s", key)
		}
		*dest = types.StringValue(*value)
	}
	var flag *string
	if json.Unmarshal(fields["force"], &flag) != nil || flag == nil || (*flag != "0" && *flag != "1") {
		return m, fmt.Errorf("missing or malformed force")
	}
	m.Force = types.BoolValue(*flag == "1")
	if !template {
		_, err := m.values()
		return m, err
	}
	return m, nil
}

// Flattened selector entries carry labels/optgroups, but keys are the wire
// values. Optional empty-key None is a real selected sentinel, not absence.
func optionSelection(raw json.RawMessage, field string, template bool) (string, map[string]json.RawMessage, error) {
	var options map[string]json.RawMessage
	if json.Unmarshal(raw, &options) != nil || options == nil {
		return "", nil, fmt.Errorf("expected selected map")
	}
	selected := []string{}
	for key, value := range options {
		valid := optionCodeValid(key)
		if field == "encoding" || field == "match_encoding" {
			valid = optionContains(optionEncodings, key)
		}
		if key == "" && (field == "match_code" || field == "match_encoding") {
			valid = true
		}
		if !valid {
			return "", nil, fmt.Errorf("unsupported selector key %q", key)
		}
		var entry struct {
			Selected *int `json:"selected"`
		}
		if json.Unmarshal(value, &entry) != nil || entry.Selected == nil || (*entry.Selected != 0 && *entry.Selected != 1) {
			return "", nil, fmt.Errorf("invalid selected flag")
		}
		if *entry.Selected == 1 {
			selected = append(selected, key)
		}
	}
	if len(selected) > 1 || (len(selected) == 0 && !template) {
		return "", nil, fmt.Errorf("expected exactly one selected key")
	}
	if len(selected) == 0 {
		return "", options, nil
	}
	return selected[0], options, nil
}

func optionMatches(actual DHCPv4OptionResourceModel, requested optionPayload) bool {
	got, err := actual.values()
	if err != nil {
		return false
	}
	for key, value := range requested {
		if got[key] != value {
			return false
		}
	}
	return true
}

func optionModelFromPayload(p optionPayload, id string) DHCPv4OptionResourceModel {
	return DHCPv4OptionResourceModel{ID: types.StringValue(id), Code: types.StringValue(p["code"]), Encoding: types.StringValue(p["encoding"]), Data: types.StringValue(p["data"]), Force: types.BoolValue(p["force"] == "1"), MatchCode: types.StringValue(p["match_code"]), MatchEncoding: types.StringValue(p["match_encoding"]), MatchData: types.StringValue(p["match_data"]), Description: types.StringValue(p["description"])}
}
