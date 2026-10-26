// Copyright (c) Matthew Mellor
// SPDX-License-Identifier: MPL-2.0

package system

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func groupTestModel() GatewayGroupResourceModel {
	empty := types.SetValueMust(types.StringType, nil)
	return GatewayGroupResourceModel{ID: types.StringValue("00000000-0000-0000-0000-000000000001"), Name: types.StringValue("synthetic_group"), Tier1: types.SetValueMust(types.StringType, []attr.Value{types.StringValue("synthetic_a")}), Tier2: empty, Tier3: empty, Tier4: empty, Tier5: empty, Trigger: types.StringValue("down"), PoolOptions: types.StringValue(""), Description: types.StringValue("")}
}

func TestGatewayGroupWriteBoundaries(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(*GatewayGroupResourceModel)
		valid  bool
	}{
		{"valid defaults", func(*GatewayGroupResourceModel) {}, true},
		{"all empty", func(m *GatewayGroupResourceModel) { m.Tier1 = m.Tier2 }, false},
		{"unknown set", func(m *GatewayGroupResourceModel) { m.Tier2 = types.SetUnknown(types.StringType) }, false},
		{"null set", func(m *GatewayGroupResourceModel) { m.Tier2 = types.SetNull(types.StringType) }, false},
		{"unknown member", func(m *GatewayGroupResourceModel) {
			m.Tier2 = types.SetValueMust(types.StringType, []attr.Value{types.StringUnknown()})
		}, false},
		{"null member", func(m *GatewayGroupResourceModel) {
			m.Tier2 = types.SetValueMust(types.StringType, []attr.Value{types.StringNull()})
		}, false},
		{"empty member", func(m *GatewayGroupResourceModel) {
			m.Tier2 = types.SetValueMust(types.StringType, []attr.Value{types.StringValue("")})
		}, false},
		{"whitespace member", func(m *GatewayGroupResourceModel) {
			m.Tier2 = types.SetValueMust(types.StringType, []attr.Value{types.StringValue(" synthetic_b")})
		}, false},
		{"csv member", func(m *GatewayGroupResourceModel) {
			m.Tier2 = types.SetValueMust(types.StringType, []attr.Value{types.StringValue("a,b")})
		}, false},
		{"immutable invalid name", func(m *GatewayGroupResourceModel) { m.Name = types.StringValue("invalid name") }, false},
		{"unknown name", func(m *GatewayGroupResourceModel) { m.Name = types.StringUnknown() }, false},
		{"invalid trigger", func(m *GatewayGroupResourceModel) { m.Trigger = types.StringValue("invalid") }, false},
		{"invalid pool", func(m *GatewayGroupResourceModel) { m.PoolOptions = types.StringValue("roundrobin") }, false},
		{"unknown description", func(m *GatewayGroupResourceModel) { m.Description = types.StringUnknown() }, false},
		{"repeated cross tier permitted", func(m *GatewayGroupResourceModel) { m.Tier2 = m.Tier1 }, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := groupTestModel()
			tc.mutate(&m)
			_, err := m.payload(context.Background())
			if (err == nil) != tc.valid {
				t.Fatalf("valid=%v err=%v", tc.valid, err)
			}
		})
	}
}

func TestGatewayGroupStrictRead(t *testing.T) {
	baseline := `{"name":"synthetic_group","descr":"literal,description","item":{"synthetic_a":{"selected":1,"value":"different label"}},"item2":[],"item3":{},"item4":[],"item5":[],"trigger":{"down":{"selected":1}},"poolopts":{"":{"selected":1}}}`
	for _, tc := range []struct {
		name, key, value string
		valid            bool
	}{
		{name: "selected keys and default pool", valid: true},
		{"missing name", "name", "", false},
		{"empty array whitespace", "item2", ` [ ] `, true},
		{"future unselected enum", "trigger", `{"down":{"selected":1},"future":{"selected":0}}`, false},
		{"null description", "descr", "null", false},
		{"numeric name", "name", "5", false},
		{"missing tier", "item2", "", false},
		{"null tier", "item2", "null", false},
		{"string tier", "item2", `""`, false},
		{"null flag", "item", `{"synthetic_a":{"selected":null}}`, false},
		{"string flag", "item", `{"synthetic_a":{"selected":"1"}}`, false},
		{"invalid flag", "item", `{"synthetic_a":{"selected":2}}`, false},
		{"empty gateway key", "item", `{"":{"selected":0}}`, false},
		{"empty union", "item", `[]`, false},
		{"multiple triggers", "trigger", `{"down":{"selected":1},"downloss":{"selected":1}}`, false},
		{"invalid trigger", "trigger", `{"future":{"selected":1}}`, false},
		{"unselected pool", "poolopts", `{"":{"selected":0}}`, false},
		{"invalid pool", "poolopts", `{"future":{"selected":1}}`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var raw map[string]json.RawMessage
			if err := json.Unmarshal([]byte(baseline), &raw); err != nil {
				t.Fatal(err)
			}
			if tc.key != "" {
				if tc.value == "" {
					delete(raw, tc.key)
				} else {
					raw[tc.key] = json.RawMessage(tc.value)
				}
			}
			m, err := decodeGatewayGroup(context.Background(), raw, "00000000-0000-0000-0000-000000000001")
			if (err == nil) != tc.valid {
				t.Fatalf("valid=%v err=%v", tc.valid, err)
			}
			if tc.valid {
				p, e := m.payload(context.Background())
				if e != nil || p["item"] != "synthetic_a" || p["poolopts"] != "" || p["descr"] != "literal,description" {
					t.Fatalf("wrong strict mapping %v %v", p, e)
				}
			}
		})
	}
}
