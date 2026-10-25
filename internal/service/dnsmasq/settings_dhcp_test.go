// Copyright (c) Matthew Mellor
// SPDX-License-Identifier: MPL-2.0

package dnsmasq

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
)

// Tagged OPNsense 26.7.7 GET shape; labels and %domain are not configuration.
const dhcpFixture = `{"no_interface":{"wan":{"value":"WAN label","selected":0},"lan":{"value":"LAN label","selected":1}},"fqdn":"1","domain":"","local":"0","lease_max":"","authoritative":"1","default_fw_rules":"1","reply_delay":"0","enable_ra":"1","host_ping":"1","nosync":"0","log_dhcp":"0","log_quiet":"1","%domain":"system.example.invalid"}`

func TestDHCPNestedResponse(t *testing.T) {
	var response settingsAPIResponse
	if err := json.Unmarshal([]byte(`{"dhcp":`+dhcpFixture+`}`), &response); err != nil {
		t.Fatal(err)
	}
	var model SettingsResourceModel
	model.fromAPI(context.Background(), &response, "dnsmasq")
	attrs := model.DHCP.Attributes()
	if len(attrs) != 13 || !attrs["fqdn"].Equal(types.BoolValue(true)) || !attrs["local"].Equal(types.BoolValue(false)) || !attrs["domain"].Equal(types.StringValue("")) || !attrs["lease_max"].IsNull() || !attrs["reply_delay"].Equal(types.Int64Value(0)) {
		t.Fatalf("lost typed settings: %v", attrs)
	}
	set := attrs["no_interface"].(types.Set)
	var interfaces []string
	if d := set.ElementsAs(context.Background(), &interfaces, false); d.HasError() {
		t.Fatal(d)
	}
	if !reflect.DeepEqual(interfaces, []string{"lan"}) {
		t.Fatalf("selected labels instead of keys: %v", interfaces)
	}
	payload, err := json.Marshal(model.toAPI(context.Background()))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(payload), "%domain") || strings.Contains(string(payload), "label") {
		t.Fatalf("metadata leaked into request: %s", payload)
	}
	var request map[string]json.RawMessage
	if err = json.Unmarshal(payload, &request); err != nil {
		t.Fatal(err)
	}
	var nested map[string]string
	if err = json.Unmarshal(request["dhcp"], &nested); err != nil {
		t.Fatal(err)
	}
	if nested["no_interface"] != "lan" || nested["fqdn"] != "1" || nested["reply_delay"] != "0" || nested["domain"] != "" {
		t.Fatalf("wrong nested payload: %v", nested)
	}
	if _, ok := nested["lease_max"]; ok {
		t.Fatal("empty integer must remain omitted, not become zero")
	}
}

func TestDHCPPartialPayloadPreservesOmissionsAndSendsExplicitClears(t *testing.T) {
	attrs := map[string]attr.Value{}
	for name, typ := range dhcpAttributeTypes() {
		switch {
		case typ.Equal(types.BoolType):
			attrs[name] = types.BoolUnknown()
		case typ.Equal(types.Int64Type):
			attrs[name] = types.Int64Null()
		case typ.Equal(types.StringType):
			attrs[name] = types.StringUnknown()
		case typ.Equal(types.SetType{ElemType: types.StringType}):
			attrs[name] = types.SetNull(types.StringType)
		}
	}
	attrs["fqdn"] = types.BoolValue(false)
	attrs["domain"] = types.StringValue("")
	attrs["reply_delay"] = types.Int64Value(0)
	attrs["no_interface"] = types.SetValueMust(types.StringType, []attr.Value{})
	value := types.ObjectValueMust(dhcpAttributeTypes(), attrs)
	encoded, err := json.Marshal(dhcpRequest(context.Background(), value))
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]string
	if err = json.Unmarshal(encoded, &got); err != nil {
		t.Fatal(err)
	}
	want := map[string]string{"fqdn": "0", "domain": "", "reply_delay": "0", "no_interface": ""}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("partial write: got %v want %v", got, want)
	}
	for _, element := range []attr.Value{types.StringUnknown(), types.StringNull()} {
		attrs["no_interface"] = types.SetValueMust(types.StringType, []attr.Value{element})
		partial := dhcpRequest(context.Background(), types.ObjectValueMust(dhcpAttributeTypes(), attrs))
		if partial.NoInterface != nil {
			t.Fatal("unresolved interface element became an intentional clear")
		}
	}
	for _, v := range []types.Object{types.ObjectNull(dhcpAttributeTypes()), types.ObjectUnknown(dhcpAttributeTypes())} {
		m := SettingsResourceModel{DHCP: v}
		data, err := json.Marshal(m.toAPI(context.Background()))
		if err != nil {
			t.Fatal(err)
		}
		var top map[string]json.RawMessage
		if err = json.Unmarshal(data, &top); err != nil {
			t.Fatal(err)
		}
		if _, ok := top["dhcp"]; ok {
			t.Fatalf("omitted object was written: %s", data)
		}
	}
}

func TestDHCPRejectsMalformedResponses(t *testing.T) {
	var original map[string]json.RawMessage
	if err := json.Unmarshal([]byte(dhcpFixture), &original); err != nil {
		t.Fatal(err)
	}
	for field := range original {
		if field == "%domain" {
			continue
		}
		for _, missing := range []bool{false, true} {
			data := map[string]json.RawMessage{}
			for k, v := range original {
				data[k] = v
			}
			if missing {
				delete(data, field)
			} else {
				data[field] = json.RawMessage("null")
			}
			raw, _ := json.Marshal(data)
			var response settingsDHCPResponse
			if err := json.Unmarshal(raw, &response); err == nil {
				t.Fatalf("accepted missing/null %s", field)
			}
		}
	}
	cases := map[string][]string{
		"reply_delay":  {`"-1"`, `"61"`, `"one"`, `1`, `"9223372036854775808"`},
		"lease_max":    {`"-1"`, `"one"`},
		"fqdn":         {`"true"`, `true`, `"2"`},
		"no_interface": {`"lan"`, `["lan"]`, `{"lan":{"value":"LAN"}}`, `{"lan":{"value":"LAN","selected":2}}`, `{"lan":"1"}`},
	}
	for field, values := range cases {
		for _, value := range values {
			data := map[string]json.RawMessage{}
			for k, v := range original {
				data[k] = v
			}
			data[field] = json.RawMessage(value)
			raw, _ := json.Marshal(data)
			var response settingsDHCPResponse
			if err := json.Unmarshal(raw, &response); err == nil {
				t.Fatalf("accepted %s=%s", field, value)
			}
		}
	}
	for _, raw := range []string{`{}`, `{"dhcp":null}`, `{"dhcp":[]}`, `{"dhcp":"fqdn"}`} {
		var response settingsAPIResponse
		if err := json.Unmarshal([]byte(raw), &response); err == nil {
			t.Fatalf("accepted malformed branch %s", raw)
		}
	}
	for _, value := range []string{`{}`, `[]`, `{"wan":{"value":"WAN","selected":"0"}}`} {
		data := map[string]json.RawMessage{}
		for k, v := range original {
			data[k] = v
		}
		data["no_interface"] = json.RawMessage(value)
		raw, _ := json.Marshal(data)
		var response settingsDHCPResponse
		if err := json.Unmarshal(raw, &response); err != nil {
			t.Fatalf("rejected valid empty selector: %v", err)
		}
	}
}

func TestDHCPStateUpgrade(t *testing.T) {
	ctx := context.Background()
	var schemaResponse resource.SchemaResponse
	r := &settingsResource{}
	r.Schema(ctx, resource.SchemaRequest{}, &schemaResponse)
	if schemaResponse.Schema.Version != 1 {
		t.Fatal("missing schema version bump")
	}
	for _, legacy := range []string{`[]`, `null`, `["fqdn"]`, `"lan"`, `{}`, `[null]`} {
		t.Run(legacy, func(t *testing.T) {
			raw := []byte(`{"id":"dnsmasq","enable":true,"port":"5353","dhcp":` + legacy + `}`)
			response := resource.UpgradeStateResponse{State: tfsdk.State{Schema: schemaResponse.Schema}}
			r.UpgradeState(ctx)[0].StateUpgrader(ctx, resource.UpgradeStateRequest{RawState: &tfprotov6.RawState{JSON: raw}}, &response)
			valid := legacy == `[]` || legacy == `null`
			if response.Diagnostics.HasError() == valid {
				t.Fatalf("unexpected diagnostics: %v", response.Diagnostics)
			}
			if !valid {
				return
			}
			var model SettingsResourceModel
			if d := response.State.Get(ctx, &model); d.HasError() {
				t.Fatal(d)
			}
			if model.ID.ValueString() != "dnsmasq" || !model.Enable.ValueBool() || model.Port.ValueString() != "5353" || !model.DHCP.IsNull() {
				t.Fatalf("migration changed unrelated settings: %+v", model)
			}
		})
	}
}
