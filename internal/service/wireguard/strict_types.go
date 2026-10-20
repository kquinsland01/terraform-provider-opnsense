// Copyright (c) Matthew Mellor
// SPDX-License-Identifier: MPL-2.0

package wireguard

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

type strictBool bool

func (b *strictBool) UnmarshalJSON(data []byte) error {
	var value string
	if err := json.Unmarshal(data, &value); err != nil {
		return fmt.Errorf("expected boolean string: %w", err)
	}
	switch value {
	case "0":
		*b = false
	case "1":
		*b = true
	default:
		return fmt.Errorf("invalid boolean %q; expected 0 or 1", value)
	}
	return nil
}

type strictOptionalInteger struct {
	Value int64
	Set   bool
}

func (i *strictOptionalInteger) UnmarshalJSON(data []byte) error {
	if bytes.Equal(bytes.TrimSpace(data), []byte("null")) {
		return fmt.Errorf("null is not an authoritative integer")
	}
	var value string
	if err := json.Unmarshal(data, &value); err != nil {
		return fmt.Errorf("expected integer string: %w", err)
	}
	if value == "" {
		i.Set = false
		return nil
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid integer %q", value)
	}
	i.Value, i.Set = parsed, true
	return nil
}

type strictSelectedMapList []string

func (s *strictSelectedMapList) UnmarshalJSON(data []byte) error {
	trimmed := bytes.TrimSpace(data)
	if bytes.Equal(trimmed, []byte("null")) {
		return fmt.Errorf("null is not an authoritative selector")
	}
	if bytes.Equal(trimmed, []byte("[]")) || bytes.Equal(trimmed, []byte(`""`)) {
		*s = []string{}
		return nil
	}
	if len(trimmed) > 0 && trimmed[0] == '"' {
		var csv string
		if err := json.Unmarshal(trimmed, &csv); err != nil {
			return err
		}
		if csv == "" {
			*s = []string{}
			return nil
		}
		values := strings.Split(csv, ",")
		for _, value := range values {
			if value == "" || strings.TrimSpace(value) != value {
				return fmt.Errorf("malformed comma-separated selector %q", csv)
			}
		}
		sort.Strings(values)
		*s = values
		return nil
	}
	var raw map[string]struct {
		Selected json.RawMessage `json:"selected"`
		Value    *string         `json:"value"`
	}
	decoder := json.NewDecoder(bytes.NewReader(trimmed))
	decoder.UseNumber()
	if err := decoder.Decode(&raw); err != nil {
		return fmt.Errorf("expected selected-option map: %w", err)
	}
	// BaseSetField::getNodeData() returns this exact sentinel for an empty
	// AsList field: explode(",", "") yields one empty, selected entry.
	if entry, ok := raw[""]; ok {
		if len(raw) == 1 && string(bytes.TrimSpace(entry.Selected)) == "1" && entry.Value != nil && *entry.Value == "" {
			*s = []string{}
			return nil
		}
		return fmt.Errorf("malformed empty selected-option sentinel")
	}
	selected := make([]string, 0)
	for key, entry := range raw {
		switch string(bytes.TrimSpace(entry.Selected)) {
		case "0":
		case "1":
			selected = append(selected, key)
		default:
			return fmt.Errorf("selector %q has invalid selected value %q", key, string(bytes.TrimSpace(entry.Selected)))
		}
	}
	sort.Strings(selected)
	*s = selected
	return nil
}

// requireFields distinguishes an explicit false/empty value from a truncated
// API object. Missing metadata must never silently clear Terraform state.
func requireFields(data []byte, fields ...string) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	for _, field := range fields {
		value, ok := raw[field]
		if !ok || bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
			return fmt.Errorf("WireGuard response is missing required field %q", field)
		}
	}
	return nil
}

func (a *wireguardPeerAPIResponse) UnmarshalJSON(data []byte) error {
	if err := requireFields(data, "enabled", "name", "pubkey", "tunneladdress", "servers", "serveraddress", "serverport", "keepalive"); err != nil {
		return err
	}
	type alias wireguardPeerAPIResponse
	return json.Unmarshal(data, (*alias)(a))
}

func (a *wireguardServerAPIResponse) UnmarshalJSON(data []byte) error {
	if err := requireFields(data, "enabled", "name", "instance", "pubkey", "port", "mtu", "dns", "tunneladdress", "disableroutes", "gateway", "peers"); err != nil {
		return err
	}
	type alias wireguardServerAPIResponse
	return json.Unmarshal(data, (*alias)(a))
}
