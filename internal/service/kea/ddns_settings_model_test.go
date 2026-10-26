// Copyright (c) Matthew Mellor
// SPDX-License-Identifier: MPL-2.0

package kea

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/matthew-on-git/terraform-provider-opnsense/pkg/opnsense"
)

type ddnsPrivateFixture struct{ raw []byte }

func (p *ddnsPrivateFixture) GetKey(context.Context, string) ([]byte, diag.Diagnostics) {
	return p.raw, nil
}

func (p *ddnsPrivateFixture) SetKey(_ context.Context, _ string, value []byte) diag.Diagnostics {
	p.raw = value
	return nil
}

func TestDDNSStrictModelEveryField(t *testing.T) {
	base := map[string]any{"enabled": "0", "manual_config": "1", "server_ip": "any", "server_port": "65535", "unowned": map[string]any{"opaque": true}}
	for _, field := range []string{"enabled", "manual_config", "server_ip", "server_port"} {
		for _, mode := range []string{"missing", "null", "number", "bool", "array", "wrapper", "invalid"} {
			t.Run(field+"/"+mode, func(t *testing.T) {
				fields := map[string]any{}
				for k, v := range base {
					fields[k] = v
				}
				switch mode {
				case "missing":
					delete(fields, field)
				case "null":
					fields[field] = nil
				case "number":
					fields[field] = 1
				case "bool":
					fields[field] = false
				case "array":
					fields[field] = []string{"1"}
				case "wrapper":
					fields[field] = map[string]int{"selected": 1}
				case "invalid":
					fields[field] = "invalid"
				}
				raw, _ := json.Marshal(map[string]any{"ddns": map[string]any{"general": fields}})
				if _, err := decodeDDNS(raw); err == nil {
					t.Fatal("invalid authoritative field accepted")
				}
			})
		}
	}
	for _, ip := range []string{"any", "192.0.2.1", "0.0.0.0"} {
		p := ddnsPayload{"enabled": "0", "server_ip": ip}
		if err := validateDDNSPayload(p, false); err != nil {
			t.Fatal(err)
		}
	}
	for _, ip := range []string{"", "ANY", "::1", "192.0.2.1/32", "192.0.2.01", "192.0.2.1,192.0.2.2", "192.0.2.1 "} {
		if validateDDNSPayload(ddnsPayload{"enabled": "0", "server_ip": ip}, false) == nil {
			t.Fatal("invalid address accepted", ip)
		}
	}
	for _, port := range []string{"0", "", "65536", "-1", "domain", "53-54", "053", "1.0"} {
		if validateDDNSPayload(ddnsPayload{"enabled": "0", "server_port": port}, false) == nil {
			t.Fatal("invalid port accepted", port)
		}
	}
	for _, port := range []string{"1", "53001", "65535"} {
		if err := validateDDNSPayload(ddnsPayload{"enabled": "0", "server_port": port}, false); err != nil {
			t.Fatal(err)
		}
	}
	raw, _ := json.Marshal(map[string]any{"ddns": map[string]any{"general": base}})
	if _, err := decodeDDNS(raw); err != nil {
		t.Fatal("valid model rejected", err)
	}
}

func TestDDNSOmittedPlansUnknownAndExplicitFalse(t *testing.T) {
	for _, config := range []types.String{types.StringNull(), types.StringUnknown(), types.StringValue("any")} {
		resp := planmodifier.StringResponse{PlanValue: config}
		ddnsOmitted{}.PlanModifyString(context.Background(), planmodifier.StringRequest{ConfigValue: config, StateValue: types.StringValue("192.0.2.1")}, &resp)
		want := config
		if config.IsNull() {
			want = types.StringValue("192.0.2.1")
		}
		if !resp.PlanValue.Equal(want) {
			t.Fatal("string intent changed")
		}
	}
	for _, config := range []types.Bool{types.BoolNull(), types.BoolUnknown(), types.BoolValue(false)} {
		resp := planmodifier.BoolResponse{PlanValue: config}
		ddnsOmitted{}.PlanModifyBool(context.Background(), planmodifier.BoolRequest{ConfigValue: config, StateValue: types.BoolValue(true)}, &resp)
		want := config
		if config.IsNull() {
			want = types.BoolValue(true)
		}
		if !resp.PlanValue.Equal(want) {
			t.Fatal("bool intent changed")
		}
	}
	for _, config := range []types.Int64{types.Int64Null(), types.Int64Unknown(), types.Int64Value(53002)} {
		resp := planmodifier.Int64Response{PlanValue: config}
		ddnsOmitted{}.PlanModifyInt64(context.Background(), planmodifier.Int64Request{ConfigValue: config, StateValue: types.Int64Value(53001)}, &resp)
		want := config
		if config.IsNull() {
			want = types.Int64Value(53001)
		}
		if !resp.PlanValue.Equal(want) {
			t.Fatal("port intent changed")
		}
	}
	m := DDNSSettingsResourceModel{Enabled: types.BoolValue(false)}
	p, err := m.wireIntent()
	if err != nil || len(p) != 1 || p["enabled"] != "0" {
		t.Fatal("omitted fields or false lost", p, err)
	}
	for _, field := range []string{"enabled", "manual_config", "server_ip", "server_port"} {
		unknown := m
		switch field {
		case "enabled":
			unknown.Enabled = types.BoolUnknown()
		case "manual_config":
			unknown.ManualConfig = types.BoolUnknown()
		case "server_ip":
			unknown.ServerIP = types.StringUnknown()
		case "server_port":
			unknown.ServerPort = types.Int64Unknown()
		}
		if _, err := unknown.wireIntent(); err == nil {
			t.Fatal("unknown became known", field)
		}
	}
}

func TestDDNSCanonicalPrivateNegativesAndIdentity(t *testing.T) {
	ctx := context.Background()
	p := &ddnsPrivateFixture{}
	if d := ddnsMark(ctx, p, optionSaved, ddnsPayload{"enabled": "0"}); d.HasError() {
		t.Fatal(d)
	}
	if _, d := ddnsPending(ctx, p, "ddns"); d.HasError() {
		t.Fatal(d)
	}
	canonical := string(p.raw)
	for _, raw := range []string{`{}`, `null`, `[]`, `{"id":"ddns","ID":"ddns","kind":"saved","payload":{"enabled":"0"}}`, `{"id":"ddns","kind":"saved","payload":{"enabled":"0","enabled":"1"}}`, `{"id":"other","kind":"saved","payload":{"enabled":"0"}}`, `{"id":"ddns","kind":"delete_unknown","payload":{"enabled":"0"}}`, `{"id":"ddns","kind":"saved","payload":{"server_port":"0"}}`, `{"id":"ddns","kind":"saved","payload":{"enabled":"0","unowned":"x"}}`, canonical + " ", canonical + canonical} {
		p.raw = []byte(raw)
		if _, d := ddnsPending(ctx, p, "ddns"); !d.HasError() {
			t.Fatal("noncanonical or invalid private accepted", raw)
		}
	}
	p.raw = []byte(canonical)
	if _, d := ddnsPending(ctx, p, "DDNS"); !d.HasError() {
		t.Fatal("wrong identity accepted")
	}
}

func TestDDNSConfiguredRetriesSingleWriteRedirectAndCanceledLock(t *testing.T) {
	gets, posts, redirects := 0, 0, 0
	h := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/redirect" {
			http.Redirect(w, r, "/target", http.StatusTemporaryRedirect)
			return
		}
		if r.URL.Path == "/target" {
			redirects++
		}
		if r.Method == http.MethodGet {
			gets++
		} else {
			posts++
		}
		http.Error(w, "unavailable", 500)
	}))
	defer h.Close()
	c, err := opnsense.NewClient(opnsense.ClientConfig{BaseURL: h.URL, APIKey: "fixture", APISecret: "fixture", RetryMax: 2, RetryWaitMin: time.Millisecond, RetryWaitMax: time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	_, _ = readDDNS(context.Background(), c)
	_, _ = optionRequest(context.Background(), c, http.MethodPost, "/api/kea/ddns/set", map[string]any{})
	_, _ = optionRequest(context.Background(), c, http.MethodPost, "/redirect", map[string]any{})
	_, _ = optionRequest(context.Background(), c, http.MethodGet, "/redirect", nil)
	if gets != 3 || posts != 1 || redirects != 0 {
		t.Fatalf("GET retry/write once/no redirect: %d/%d/%d", gets, posts, redirects)
	}
	if err := c.LockMutex(context.Background()); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	d := saveDDNS(ctx, c, DDNSSettingsResourceModel{Enabled: types.BoolValue(false)}, ddnsRecovery{}, func(DDNSSettingsResourceModel) diag.Diagnostics { return nil }, &ddnsPrivateFixture{})
	c.UnlockMutex()
	if !d.HasError() || posts != 1 {
		t.Fatal("canceled lock mutated")
	}
	// Delete must work with no client and canceled context; private corruption is
	// irrelevant because destruction intentionally discards ownership only.
	var schemaResp resource.SchemaResponse
	(&ddnsSettingsResource{}).Schema(context.Background(), resource.SchemaRequest{}, &schemaResp)
	resp := resource.DeleteResponse{}
	resp.State.Schema = schemaResp.Schema
	(&ddnsSettingsResource{}).Delete(ctx, resource.DeleteRequest{}, &resp)
	if resp.Diagnostics.HasError() || posts != 1 || gets != 3 {
		t.Fatal("delete required live client")
	}
}

func TestDDNSRequestedSubsetDoesNotVetoUnrelatedFields(t *testing.T) {
	actual := ddnsModel(ddnsPayload{"enabled": "0", "manual_config": "1", "server_ip": "any", "server_port": "53003"})
	if !ddnsMatches(actual, ddnsPayload{"enabled": "0"}) {
		t.Fatal("omitted endpoint vetoed save")
	}
	if ddnsMatches(actual, ddnsPayload{"enabled": "0", "manual_config": "0"}) {
		t.Fatal("explicit false ignored")
	}
	for _, kind := range []string{optionSaved, optionSaveUnknown} {
		t.Run(fmt.Sprint(kind), func(t *testing.T) {
			p := &ddnsPrivateFixture{}
			if d := ddnsMark(context.Background(), p, kind, ddnsPayload{"enabled": "0"}); d.HasError() {
				t.Fatal(d)
			}
		})
	}
}

func TestDDNSChangedOmittedPlansKeepFrameworkUnknown(t *testing.T) {
	s := planmodifier.StringResponse{PlanValue: types.StringUnknown()}
	ddnsOmitted{}.PlanModifyString(context.Background(), planmodifier.StringRequest{ConfigValue: types.StringNull(), StateValue: types.StringValue("any"), PlanValue: types.StringUnknown()}, &s)
	b := planmodifier.BoolResponse{PlanValue: types.BoolUnknown()}
	ddnsOmitted{}.PlanModifyBool(context.Background(), planmodifier.BoolRequest{ConfigValue: types.BoolNull(), StateValue: types.BoolValue(true), PlanValue: types.BoolUnknown()}, &b)
	n := planmodifier.Int64Response{PlanValue: types.Int64Unknown()}
	ddnsOmitted{}.PlanModifyInt64(context.Background(), planmodifier.Int64Request{ConfigValue: types.Int64Null(), StateValue: types.Int64Value(53001), PlanValue: types.Int64Unknown()}, &n)
	if !s.PlanValue.IsUnknown() || !b.PlanValue.IsUnknown() || !n.PlanValue.IsUnknown() {
		t.Fatal("changed omitted fields were forced known")
	}
}

func TestDDNSAddressConfigurationValidation(t *testing.T) {
	for _, value := range []types.String{types.StringNull(), types.StringUnknown(), types.StringValue("any"), types.StringValue("192.0.2.31"), types.StringValue(""), types.StringValue("ANY"), types.StringValue("::1"), types.StringValue("192.0.2.1/32"), types.StringValue("192.0.2.01"), types.StringValue("192.0.2.1,192.0.2.2")} {
		resp := validator.StringResponse{}
		ddnsAddressValidator{}.ValidateString(context.Background(), validator.StringRequest{ConfigValue: value}, &resp)
		valid := value.IsNull() || value.IsUnknown() || ddnsAddressValid(value.ValueString())
		if resp.Diagnostics.HasError() == valid {
			t.Fatalf("incorrect configuration validation for %s", value)
		}
	}
}

func TestDDNSAmbiguousOwnedJSON(t *testing.T) {
	good := `{"ddns":{"general":{"enabled":"0","manual_config":"1","server_ip":"any","server_port":"53002","extra":{"opaque":null}}}}`
	for _, raw := range []string{
		strings.Replace(good, `"ddns":`, `"DDNS":`, 1),
		strings.Replace(good, `"general":`, `"General":`, 1),
		strings.Replace(good, `"enabled":"0"`, `"enabled":"0","Enabled":"0"`, 1),
		strings.Replace(good, `"enabled":"0"`, `"enabled":"1","enabled":"0"`, 1),
		strings.Replace(good, `"ddns":`, `"ddns":{},"ddns":`, 1),
		strings.Replace(good, `"general":`, `"general":{},"general":`, 1),
	} {
		if _, err := decodeDDNS([]byte(raw)); err == nil {
			t.Fatal("ambiguous owned JSON accepted", raw)
		}
	}
	if _, err := decodeDDNS([]byte(good)); err != nil {
		t.Fatal("unowned values rejected", err)
	}
}
