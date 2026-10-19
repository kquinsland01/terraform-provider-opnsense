// Copyright (c) Matthew Mellor
// SPDX-License-Identifier: MPL-2.0

package openvpn

import (
	"encoding/json"
	"reflect"
)

// normalizedPayloadEqual compares API payloads after treating an omitted
// optional string and an explicitly empty optional string as the same wire
// value. OPNsense commonly returns empty strings for omitted form fields.
func normalizedPayloadEqual(left, right any) bool {
	decode := func(value any) map[string]any {
		encoded, err := json.Marshal(value)
		if err != nil {
			return nil
		}
		var result map[string]any
		if err := json.Unmarshal(encoded, &result); err != nil {
			return nil
		}
		return result
	}
	a, b := decode(left), decode(right)
	if a == nil || b == nil {
		return false
	}
	isWireZero := func(value any) bool { return value == "" || value == "0" }
	for key, value := range a {
		other, ok := b[key]
		if !ok && isWireZero(value) {
			continue
		}
		if !ok || !reflect.DeepEqual(value, other) {
			return false
		}
	}
	for key, value := range b {
		other, ok := a[key]
		if !ok && isWireZero(value) {
			continue
		}
		if !ok || !reflect.DeepEqual(value, other) {
			return false
		}
	}
	return true
}
