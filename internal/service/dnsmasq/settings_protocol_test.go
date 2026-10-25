// Copyright (c) Matthew Mellor
// SPDX-License-Identifier: MPL-2.0

package dnsmasq_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tftypes"

	"github.com/matthew-on-git/terraform-provider-opnsense/internal/provider"
)

func dnsmasqValue(t *testing.T, s *tfprotov6.Schema, overrides map[string]any) *tfprotov6.DynamicValue {
	t.Helper()
	values := map[string]any{}
	for _, a := range s.Block.Attributes {
		values[a.Name] = nil
	}
	for name, v := range overrides {
		values[name] = v
	}
	raw, err := json.Marshal(values)
	if err != nil {
		t.Fatal(err)
	}
	return &tfprotov6.DynamicValue{JSON: raw}
}

func dnsmasqErrors(ds []*tfprotov6.Diagnostic) bool {
	for _, d := range ds {
		if d.Severity == tfprotov6.DiagnosticSeverityError {
			return true
		}
	}
	return false
}

func dnsmasqServer(t *testing.T, uri string) (tfprotov6.ProviderServer, *tfprotov6.Schema) {
	t.Helper()
	server := providerserver.NewProtocol6(provider.New("test")())()
	schemas, err := server.GetProviderSchema(context.Background(), &tfprotov6.GetProviderSchemaRequest{})
	if err != nil || dnsmasqErrors(schemas.Diagnostics) {
		t.Fatal(err, schemas.Diagnostics)
	}
	config := dnsmasqValue(t, schemas.Provider, map[string]any{"uri": uri, "api_key": "fixture-key", "api_secret": "fixture-secret", "insecure": true})
	configured, err := server.ConfigureProvider(context.Background(), &tfprotov6.ConfigureProviderRequest{Config: config, TerraformVersion: "1.14.0"})
	if err != nil || dnsmasqErrors(configured.Diagnostics) {
		t.Fatal(err, configured.Diagnostics)
	}
	return server, schemas.ResourceSchemas["opnsense_dnsmasq_settings"]
}

const protocolDHCP = `{"no_interface":{"wan":{"value":"WAN","selected":1}},"fqdn":"1","domain":"","local":"0","lease_max":"128","authoritative":"1","default_fw_rules":"1","reply_delay":"0","enable_ra":"1","host_ping":"1","nosync":"0","log_dhcp":"0","log_quiet":"1","%domain":"system.invalid"}`

func TestProtocolDNSMasqUpgradeImportRefreshAndDestroy(t *testing.T) {
	ctx := context.Background()
	calls := 0
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/core/menu/search" {
			_, _ = fmt.Fprint(w, `{}`)
			return
		}
		calls++
		if r.Method != http.MethodGet || r.URL.Path != "/api/dnsmasq/settings/get" {
			t.Errorf("unexpected mutation: %s %s", r.Method, r.URL.Path)
		}
		_, _ = fmt.Fprintf(w, "%s", `{"dnsmasq":{"enable":"1","port":"5353","dhcp":`+protocolDHCP+`}}`)
	}))
	defer api.Close()
	server, schema := dnsmasqServer(t, api.URL)
	if schema.Version != 1 {
		t.Fatal("expected v1 schema")
	}
	upgraded, err := server.UpgradeResourceState(ctx, &tfprotov6.UpgradeResourceStateRequest{TypeName: "opnsense_dnsmasq_settings", Version: 0, RawState: &tfprotov6.RawState{JSON: []byte(`{"id":"dnsmasq","enable":true,"port":"5353","dhcp":[]}`)}})
	if err != nil || dnsmasqErrors(upgraded.Diagnostics) {
		t.Fatal(err, upgraded.Diagnostics)
	}
	read, err := server.ReadResource(ctx, &tfprotov6.ReadResourceRequest{TypeName: "opnsense_dnsmasq_settings", CurrentState: upgraded.UpgradedState})
	if err != nil || dnsmasqErrors(read.Diagnostics) {
		t.Fatal(err, read.Diagnostics)
	}
	value, err := read.NewState.Unmarshal(schema.ValueType())
	if err != nil {
		t.Fatal(err)
	}
	var attrs map[string]tftypes.Value
	if err = value.As(&attrs); err != nil {
		t.Fatal(err)
	}
	var nested map[string]tftypes.Value
	if err = attrs["dhcp"].As(&nested); err != nil {
		t.Fatal(err)
	}
	if len(nested) != 13 || !nested["fqdn"].Equal(tftypes.NewValue(tftypes.Bool, true)) || !nested["domain"].Equal(tftypes.NewValue(tftypes.String, "")) {
		t.Fatalf("bad refreshed object: %v", nested)
	}
	imported, err := server.ImportResourceState(ctx, &tfprotov6.ImportResourceStateRequest{TypeName: "opnsense_dnsmasq_settings", ID: "dnsmasq"})
	if err != nil || dnsmasqErrors(imported.Diagnostics) || len(imported.ImportedResources) != 1 {
		t.Fatal(err, imported)
	}
	reread, err := server.ReadResource(ctx, &tfprotov6.ReadResourceRequest{TypeName: "opnsense_dnsmasq_settings", CurrentState: imported.ImportedResources[0].State})
	if err != nil || dnsmasqErrors(reread.Diagnostics) {
		t.Fatal(err, reread.Diagnostics)
	}
	iv, err := reread.NewState.Unmarshal(schema.ValueType())
	if err != nil || !iv.Equal(value) {
		t.Fatal("import differs from upgraded refresh", err)
	}
	// Singleton destroy is state-only and must issue no HTTP mutation.
	nullState, err := tfprotov6.NewDynamicValue(schema.ValueType(), tftypes.NewValue(schema.ValueType(), nil))
	if err != nil {
		t.Fatal(err)
	}
	before := calls
	destroyed, err := server.ApplyResourceChange(ctx, &tfprotov6.ApplyResourceChangeRequest{TypeName: "opnsense_dnsmasq_settings", PriorState: read.NewState, PlannedState: &nullState, Config: &nullState})
	if err != nil || dnsmasqErrors(destroyed.Diagnostics) || calls != before {
		t.Fatal("destroy mutated appliance", err, destroyed.Diagnostics)
	}
}

func TestProtocolDNSMasqValidation(t *testing.T) {
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/core/menu/search" {
			t.Errorf("validation emitted traffic: %s", r.URL.Path)
		}
		_, _ = fmt.Fprint(w, `{}`)
	}))
	defer api.Close()
	server, schema := dnsmasqServer(t, api.URL)
	for _, tt := range []struct {
		name   string
		values map[string]any
		valid  bool
	}{
		{"partial", map[string]any{"reply_delay": 0}, true},
		{"upper", map[string]any{"reply_delay": 60}, true},
		{"too large", map[string]any{"reply_delay": 61}, false},
		{"negative leases", map[string]any{"lease_max": -1}, false},
		{"logging conflict", map[string]any{"log_dhcp": true, "log_quiet": true}, false},
		{"explicit false", map[string]any{"log_dhcp": false, "log_quiet": true}, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			nested := map[string]any{}
			for _, name := range []string{"no_interface", "fqdn", "domain", "local", "lease_max", "authoritative", "default_fw_rules", "reply_delay", "enable_ra", "host_ping", "nosync", "log_dhcp", "log_quiet"} {
				nested[name] = nil
			}
			for name, v := range tt.values {
				nested[name] = v
			}
			response, err := server.ValidateResourceConfig(context.Background(), &tfprotov6.ValidateResourceConfigRequest{TypeName: "opnsense_dnsmasq_settings", Config: dnsmasqValue(t, schema, map[string]any{"dhcp": nested})})
			if err != nil || dnsmasqErrors(response.Diagnostics) == tt.valid {
				t.Fatal(err, response.Diagnostics)
			}
		})
	}
	// A legacy set in HCL is not silently interpreted as an empty settings object.
	response, err := server.ValidateResourceConfig(context.Background(), &tfprotov6.ValidateResourceConfigRequest{TypeName: "opnsense_dnsmasq_settings", Config: dnsmasqValue(t, schema, map[string]any{"dhcp": []string{}})})
	if err == nil && !dnsmasqErrors(response.Diagnostics) {
		t.Fatal("legacy set configuration accepted")
	}
}

func TestProtocolDNSMasqMalformedReadPreservesPriorState(t *testing.T) {
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprintf(w, "%s", `{"dnsmasq":{"dhcp":{"fqdn":"1"}}}`)
	}))
	defer api.Close()
	server, schema := dnsmasqServer(t, api.URL)
	prior := dnsmasqValue(t, schema, map[string]any{"id": "dnsmasq", "port": "5353"})
	read, err := server.ReadResource(context.Background(), &tfprotov6.ReadResourceRequest{TypeName: "opnsense_dnsmasq_settings", CurrentState: prior})
	if err != nil || !dnsmasqErrors(read.Diagnostics) {
		t.Fatal("malformed read accepted", err, read.Diagnostics)
	}
	found := false
	for _, d := range read.Diagnostics {
		found = found || strings.Contains(d.Detail, "DHCP response is missing")
	}
	if !found {
		t.Fatal("missing actionable DHCP diagnostic", read.Diagnostics)
	}
}

func dnsmasqNestedConfig(changes map[string]any) map[string]any {
	out := map[string]any{}
	for _, name := range []string{"no_interface", "fqdn", "domain", "local", "lease_max", "authoritative", "default_fw_rules", "reply_delay", "enable_ra", "host_ping", "nosync", "log_dhcp", "log_quiet"} {
		out[name] = nil
	}
	for name, v := range changes {
		out[name] = v
	}
	return out
}

func TestProtocolDNSMasqCreatePlanUpdatePreservesUnmanagedSettings(t *testing.T) {
	ctx := context.Background()
	var dhcp map[string]any
	if err := json.Unmarshal([]byte(protocolDHCP), &dhcp); err != nil {
		t.Fatal(err)
	}
	dhcp["future_option"] = "untouched"
	writes := 0
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/core/menu/search":
			_, _ = fmt.Fprint(w, `{}`)
		case "/api/dnsmasq/settings/get":
			_ = json.NewEncoder(w).Encode(map[string]any{"dnsmasq": map[string]any{"enable": "0", "no_ident": "1", "dhcp": dhcp, "hosts": map[string]any{"host-uuid": "unchanged"}}})
		case "/api/dnsmasq/settings/set":
			writes++
			var body struct {
				DNSMasq map[string]json.RawMessage `json:"dnsmasq"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Error(err)
				return
			}
			if _, ok := body.DNSMasq["hosts"]; ok {
				t.Error("wrote independently owned hosts")
			}
			var patch map[string]string
			if err := json.Unmarshal(body.DNSMasq["dhcp"], &patch); err != nil {
				t.Error(err)
				return
			}
			want := map[string]string{"fqdn": "0", "domain": "", "no_interface": "", "reply_delay": "0"}
			if writes == 2 {
				want["reply_delay"] = "60"
			}
			if len(patch) != len(want) {
				t.Errorf("wrote unconfigured children: %v", patch)
			}
			for name, v := range want {
				if actual, ok := patch[name]; !ok || actual != v {
					t.Errorf("wrong patch %s: got %q want %q", name, actual, v)
				}
			}
			for name, v := range patch {
				if name == "no_interface" {
					dhcp[name] = []any{}
				} else {
					dhcp[name] = v
				}
			}
			_, _ = fmt.Fprint(w, `{"result":"saved"}`)
		case "/api/dnsmasq/service/reconfigure":
			_, _ = fmt.Fprint(w, `{"status":"ok"}`)
		default:
			t.Errorf("unexpected endpoint %s", r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	defer api.Close()
	server, schema := dnsmasqServer(t, api.URL)
	nullValue, err := tfprotov6.NewDynamicValue(schema.ValueType(), tftypes.NewValue(schema.ValueType(), nil))
	if err != nil {
		t.Fatal(err)
	}
	config := dnsmasqValue(t, schema, map[string]any{"enable": false, "no_ident": true, "dhcp": dnsmasqNestedConfig(map[string]any{"fqdn": false, "domain": "", "no_interface": []string{}, "reply_delay": 0})})
	planned, err := server.PlanResourceChange(ctx, &tfprotov6.PlanResourceChangeRequest{TypeName: "opnsense_dnsmasq_settings", PriorState: &nullValue, Config: config, ProposedNewState: config})
	if err != nil || dnsmasqErrors(planned.Diagnostics) {
		t.Fatal("create plan", err, planned.Diagnostics)
	}
	created, err := server.ApplyResourceChange(ctx, &tfprotov6.ApplyResourceChangeRequest{TypeName: "opnsense_dnsmasq_settings", PriorState: &nullValue, Config: config, PlannedState: planned.PlannedState, PlannedPrivate: planned.PlannedPrivate})
	if err != nil || dnsmasqErrors(created.Diagnostics) {
		t.Fatal("create", err, created.Diagnostics)
	}
	unchanged, err := server.PlanResourceChange(ctx, &tfprotov6.PlanResourceChangeRequest{TypeName: "opnsense_dnsmasq_settings", PriorState: created.NewState, Config: config, ProposedNewState: created.NewState})
	if err != nil || dnsmasqErrors(unchanged.Diagnostics) {
		t.Fatal("no-change plan", err, unchanged.Diagnostics)
	}
	priorValue, err := created.NewState.Unmarshal(schema.ValueType())
	if err != nil {
		t.Fatal(err)
	}
	unchangedValue, err := unchanged.PlannedState.Unmarshal(schema.ValueType())
	if err != nil || !unchangedValue.Equal(priorValue) {
		t.Fatal("nonempty no-change plan", err)
	}
	var attrs map[string]tftypes.Value
	if err = priorValue.As(&attrs); err != nil {
		t.Fatal(err)
	}
	var nested map[string]tftypes.Value
	if err = attrs["dhcp"].As(&nested); err != nil {
		t.Fatal(err)
	}
	nested["reply_delay"] = tftypes.NewValue(tftypes.Number, 60)
	attrs["dhcp"] = tftypes.NewValue(attrs["dhcp"].Type(), nested)
	proposed, err := tfprotov6.NewDynamicValue(schema.ValueType(), tftypes.NewValue(schema.ValueType(), attrs))
	if err != nil {
		t.Fatal(err)
	}
	updateConfig := dnsmasqValue(t, schema, map[string]any{"enable": false, "no_ident": true, "dhcp": dnsmasqNestedConfig(map[string]any{"fqdn": false, "domain": "", "no_interface": []string{}, "reply_delay": 60})})
	updatePlan, err := server.PlanResourceChange(ctx, &tfprotov6.PlanResourceChangeRequest{TypeName: "opnsense_dnsmasq_settings", PriorState: created.NewState, Config: updateConfig, ProposedNewState: &proposed})
	if err != nil || dnsmasqErrors(updatePlan.Diagnostics) {
		t.Fatal("update plan", err, updatePlan.Diagnostics)
	}
	updated, err := server.ApplyResourceChange(ctx, &tfprotov6.ApplyResourceChangeRequest{TypeName: "opnsense_dnsmasq_settings", PriorState: created.NewState, Config: updateConfig, PlannedState: updatePlan.PlannedState, PlannedPrivate: updatePlan.PlannedPrivate})
	if err != nil || dnsmasqErrors(updated.Diagnostics) {
		t.Fatal("update", err, updated.Diagnostics)
	}
	if writes != 2 || dhcp["future_option"] != "untouched" || dhcp["lease_max"] != "128" || dhcp["enable_ra"] != "1" || dhcp["%domain"] != "system.invalid" {
		t.Fatalf("unmanaged settings changed: %v", dhcp)
	}
}
