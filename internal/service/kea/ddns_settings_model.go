// Copyright (c) Matthew Mellor
// SPDX-License-Identifier: MPL-2.0

package kea

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/netip"
	"strconv"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

// DDNSSettingsResourceModel owns only the four ddns.general scalar settings.
type DDNSSettingsResourceModel struct {
	ID           types.String `tfsdk:"id"`
	Enabled      types.Bool   `tfsdk:"enabled"`
	ManualConfig types.Bool   `tfsdk:"manual_config"`
	ServerIP     types.String `tfsdk:"server_ip"`
	ServerPort   types.Int64  `tfsdk:"server_port"`
}

type ddnsPayload map[string]string

func validateDDNSPayload(p ddnsPayload, complete bool) error {
	if len(p) == 0 {
		return fmt.Errorf("empty DDNS requested payload")
	}
	for k, v := range p {
		switch k {
		case "enabled", "manual_config":
			if v != "0" && v != "1" {
				return fmt.Errorf("%s must be exactly 0 or 1", k)
			}
		case "server_ip":
			if !ddnsAddressValid(v) {
				return fmt.Errorf("server_ip must be canonical IPv4 or the exact any sentinel")
			}
		case "server_port":
			n, err := strconv.ParseUint(v, 10, 16)
			if err != nil || n == 0 || strconv.FormatUint(n, 10) != v {
				return fmt.Errorf("server_port must be an integer from 1 through 65535")
			}
		default:
			return fmt.Errorf("unexpected DDNS field %q", k)
		}
	}
	if _, ok := p["enabled"]; !ok {
		return fmt.Errorf("enabled intent is required")
	}
	if complete && len(p) != 4 {
		return fmt.Errorf("incomplete DDNS model")
	}
	return nil
}

// Consult configuration, never computed plan values, to determine wire ownership.
func (m DDNSSettingsResourceModel) wireIntent() (ddnsPayload, error) {
	p := ddnsPayload{}
	for k, v := range map[string]types.Bool{"enabled": m.Enabled, "manual_config": m.ManualConfig} {
		if v.IsUnknown() || (k == "enabled" && v.IsNull()) {
			return nil, fmt.Errorf("%s must be known before mutation", k)
		}
		if !v.IsNull() {
			p[k] = "0"
			if v.ValueBool() {
				p[k] = "1"
			}
		}
	}
	if m.ServerIP.IsUnknown() || m.ServerPort.IsUnknown() {
		return nil, fmt.Errorf("DDNS endpoint must be known before mutation")
	}
	if !m.ServerIP.IsNull() {
		p["server_ip"] = m.ServerIP.ValueString()
	}
	if !m.ServerPort.IsNull() {
		p["server_port"] = strconv.FormatInt(m.ServerPort.ValueInt64(), 10)
	}
	return p, validateDDNSPayload(p, false)
}

func ddnsModel(p ddnsPayload) DDNSSettingsResourceModel {
	port, _ := strconv.ParseInt(p["server_port"], 10, 64)
	return DDNSSettingsResourceModel{ID: types.StringValue("ddns"), Enabled: types.BoolValue(p["enabled"] == "1"), ManualConfig: types.BoolValue(p["manual_config"] == "1"), ServerIP: types.StringValue(p["server_ip"]), ServerPort: types.Int64Value(port)}
}

func (m DDNSSettingsResourceModel) values() ddnsPayload {
	p, _ := m.wireIntent()
	return p
}

func decodeDDNS(raw []byte) (DDNSSettingsResourceModel, error) {
	envelope, err := ddnsObject(raw, []string{"ddns"})
	if err != nil {
		return DDNSSettingsResourceModel{}, err
	}
	ddns, err := ddnsObject(envelope["ddns"], []string{"general"})
	if err != nil {
		return DDNSSettingsResourceModel{}, err
	}
	general, err := ddnsObject(ddns["general"], []string{"enabled", "manual_config", "server_ip", "server_port"})
	if err != nil {
		return DDNSSettingsResourceModel{}, err
	}
	p := ddnsPayload{}
	for _, k := range []string{"enabled", "manual_config", "server_ip", "server_port"} {
		var value *string
		if err := json.Unmarshal(general[k], &value); err != nil || value == nil {
			return DDNSSettingsResourceModel{}, fmt.Errorf("missing or malformed DDNS %s", k)
		}
		p[k] = *value
	}
	if err := validateDDNSPayload(p, true); err != nil {
		return DDNSSettingsResourceModel{}, err
	}
	return ddnsModel(p), nil
}

func ddnsMatches(m DDNSSettingsResourceModel, p ddnsPayload) bool {
	actual := m.values()
	for k, v := range p {
		if actual[k] != v {
			return false
		}
	}
	return true
}

func ddnsAddressValid(value string) bool {
	if value == "any" {
		return true
	}
	addr, err := netip.ParseAddr(value)
	return err == nil && addr.Is4() && addr.String() == value
}

// Check only owned/envelope keys. Extra unowned values remain opaque and are
// neither adopted nor rejected merely for carrying a different representation.
func ddnsObject(raw []byte, owned []string) (map[string]json.RawMessage, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	token, err := decoder.Token()
	if err != nil || token != json.Delim('{') {
		return nil, fmt.Errorf("expected DDNS JSON object")
	}
	result := map[string]json.RawMessage{}
	for decoder.More() {
		token, err := decoder.Token()
		if err != nil {
			return nil, err
		}
		key, ok := token.(string)
		if !ok {
			return nil, fmt.Errorf("invalid DDNS object key")
		}
		for _, name := range owned {
			if strings.EqualFold(key, name) {
				if key != name {
					return nil, fmt.Errorf("case-alias DDNS key %q", key)
				}
				if _, exists := result[key]; exists {
					return nil, fmt.Errorf("duplicate DDNS key %q", key)
				}
			}
		}
		var value json.RawMessage
		if err := decoder.Decode(&value); err != nil {
			return nil, err
		}
		result[key] = value
	}
	if _, err := decoder.Token(); err != nil {
		return nil, err
	}
	var extra any
	if decoder.Decode(&extra) != io.EOF {
		return nil, fmt.Errorf("trailing DDNS JSON data")
	}
	return result, nil
}
