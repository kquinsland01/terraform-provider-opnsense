// Copyright (c) Matthew Mellor
// SPDX-License-Identifier: MPL-2.0

package firewall

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestCreatePreservesConfiguredNonFirstCompatibilityScalar(t *testing.T) {
	t.Run("alias", func(t *testing.T) {
		var requestBody, updateBody []byte
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/api/firewall/alias/addItem":
				requestBody, _ = io.ReadAll(r.Body)
				_, _ = io.WriteString(w, `{"result":"saved","uuid":"id"}`)
			case "/api/firewall/alias/reconfigure":
				w.WriteHeader(http.StatusOK)
			case "/api/firewall/alias/setItem/id":
				updateBody, _ = io.ReadAll(r.Body)
				_, _ = io.WriteString(w, `{"result":"saved"}`)
			case "/api/firewall/alias/getItem/id":
				_, _ = io.WriteString(w, `{"alias":{"name":"a","type":{"asn":{"selected":1}},"content":"13335","description":"remote","enabled":"1","proto":{"IPv4":{"selected":1},"IPv6":{"selected":1}},"interface":[],"categories":[],"updatefreq":"","path_expression":"","counters":"0"}}`)
			default:
				t.Fatalf("unexpected endpoint %s", r.URL.Path)
			}
		}))
		defer server.Close()
		r := &aliasResource{}
		configureResource(t, r, server.URL)
		model := normalizeAlias(AliasResourceModel{Name: types.StringValue("a"), Type: types.StringValue("asn"), Content: stringSetValue([]string{"13335"}), Description: types.StringValue("planned"), Enabled: types.BoolValue(true), Proto: types.StringValue("IPv6"), IPProtocol: stringSetValue([]string{"IPv4", "IPv6"}), Interface: types.StringValue(""), UpdateFreq: types.StringValue(""), PathExpression: types.StringValue(""), Stats: types.BoolValue(false)})
		state := resourceState(t, r, model)
		resp := resource.CreateResponse{State: tfsdk.State{Schema: state.Schema}}
		r.Create(context.Background(), resource.CreateRequest{Plan: tfsdk.Plan(state)}, &resp)
		if resp.Diagnostics.HasError() {
			t.Fatal(resp.Diagnostics)
		}
		var got AliasResourceModel
		resp.State.Get(context.Background(), &got)
		if got.Proto.ValueString() != "IPv6" || !bytes.Contains(requestBody, []byte(`"proto":"IPv4,IPv6"`)) {
			t.Fatalf("state proto=%q request=%s", got.Proto.ValueString(), requestBody)
		}
		readResp := resource.ReadResponse{State: resp.State}
		r.Read(context.Background(), resource.ReadRequest{State: resp.State}, &readResp)
		if readResp.Diagnostics.HasError() {
			t.Fatal(readResp.Diagnostics)
		}
		var read AliasResourceModel
		readResp.State.Get(context.Background(), &read)
		if read.Proto.ValueString() != "IPv6" || len(read.IPProtocol.Elements()) != 2 {
			t.Fatalf("read compatibility state = %#v", read)
		}
		updateResp := resource.UpdateResponse{State: tfsdk.State{Schema: state.Schema}}
		r.Update(context.Background(), resource.UpdateRequest{Plan: tfsdk.Plan(state), State: readResp.State}, &updateResp)
		if updateResp.Diagnostics.HasError() {
			t.Fatal(updateResp.Diagnostics)
		}
		var updated AliasResourceModel
		updateResp.State.Get(context.Background(), &updated)
		if updated.Proto.ValueString() != "IPv6" || !bytes.Contains(updateBody, []byte(`"proto":"IPv4,IPv6"`)) {
			t.Fatalf("updated proto=%q request=%s", updated.Proto.ValueString(), updateBody)
		}
	})

	t.Run("NAT", func(t *testing.T) {
		var requestBody, updateBody []byte
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/api/firewall/d_nat/add_rule":
				requestBody, _ = io.ReadAll(r.Body)
				_, _ = io.WriteString(w, `{"result":"saved","uuid":"id"}`)
			case "/api/firewall/d_nat/apply":
				w.WriteHeader(http.StatusOK)
			case "/api/firewall/d_nat/set_rule/id":
				updateBody, _ = io.ReadAll(r.Body)
				_, _ = io.WriteString(w, `{"result":"saved"}`)
			case "/api/firewall/d_nat/get_rule/id":
				_, _ = io.WriteString(w, `{"rule":{"disabled":"1","sequence":"10","interface":{"lan":{"selected":1},"wan":{"selected":1}},"ipprotocol":{"inet":{"selected":1}},"protocol":{"tcp":{"selected":1}},"source":{"network":"any","port":"","not":"0"},"destination":{"network":"any","port":"443","not":"0"},"target":"192.0.2.1","local-port":"443","log":"0","descr":"remote","natreflection":[],"categories":[]}}`)
			default:
				t.Fatalf("unexpected endpoint %s", r.URL.Path)
			}
		}))
		defer server.Close()
		r := &natPortForwardResource{}
		configureResource(t, r, server.URL)
		model := normalizeNAT(NatPortForwardResourceModel{Enabled: types.BoolValue(false), Sequence: types.Int64Value(10), Interface: types.StringValue("wan"), Interfaces: stringSetValue([]string{"lan", "wan"}), IPProtocol: types.StringValue("inet"), Protocol: types.StringValue("tcp"), SourceNet: types.StringValue("any"), SourcePort: types.StringValue(""), SourceNot: types.BoolValue(false), DestinationNet: types.StringValue("any"), DestinationPort: types.StringValue("443"), DestinationNot: types.BoolValue(false), Target: types.StringValue("192.0.2.1"), LocalPort: types.StringValue("443"), Log: types.BoolValue(false), Description: types.StringValue("planned"), Reflection: types.StringValue("")})
		state := resourceState(t, r, model)
		resp := resource.CreateResponse{State: tfsdk.State{Schema: state.Schema}}
		r.Create(context.Background(), resource.CreateRequest{Plan: tfsdk.Plan(state)}, &resp)
		if resp.Diagnostics.HasError() {
			t.Fatal(resp.Diagnostics)
		}
		var got NatPortForwardResourceModel
		resp.State.Get(context.Background(), &got)
		var wrapped struct {
			Rule map[string]any `json:"rule"`
		}
		if err := json.Unmarshal(requestBody, &wrapped); err != nil {
			t.Fatal(err)
		}
		if got.Interface.ValueString() != "wan" || wrapped.Rule["interface"] != "lan,wan" {
			t.Fatalf("state interface=%q request=%s", got.Interface.ValueString(), requestBody)
		}
		readResp := resource.ReadResponse{State: resp.State}
		r.Read(context.Background(), resource.ReadRequest{State: resp.State}, &readResp)
		if readResp.Diagnostics.HasError() {
			t.Fatal(readResp.Diagnostics)
		}
		var read NatPortForwardResourceModel
		readResp.State.Get(context.Background(), &read)
		if read.Interface.ValueString() != "wan" || len(read.Interfaces.Elements()) != 2 {
			t.Fatalf("read compatibility state = %#v", read)
		}
		updateResp := resource.UpdateResponse{State: tfsdk.State{Schema: state.Schema}}
		r.Update(context.Background(), resource.UpdateRequest{Plan: tfsdk.Plan(state), State: readResp.State}, &updateResp)
		if updateResp.Diagnostics.HasError() {
			t.Fatal(updateResp.Diagnostics)
		}
		var updated NatPortForwardResourceModel
		updateResp.State.Get(context.Background(), &updated)
		if err := json.Unmarshal(updateBody, &wrapped); err != nil {
			t.Fatal(err)
		}
		if updated.Interface.ValueString() != "wan" || wrapped.Rule["interface"] != "lan,wan" {
			t.Fatalf("updated interface=%q request=%s", updated.Interface.ValueString(), updateBody)
		}
	})
}

func TestCreateRecoveryReadFailurePreservesOnlyIdentity(t *testing.T) {
	tests := []struct {
		name, add, apply, get, monad string
		run                          func(*testing.T, string) (types.String, types.String)
	}{
		{name: "alias", add: "/api/firewall/alias/addItem", apply: "/api/firewall/alias/reconfigure", get: "/api/firewall/alias/getItem/id", monad: "alias", run: runAliasFailedRecovery},
		{name: "filter", add: "/api/firewall/filter/addRule", apply: "/api/firewall/filter/apply", get: "/api/firewall/filter/getRule/id", monad: "rule", run: runFilterFailedRecovery},
		{name: "NAT", add: "/api/firewall/d_nat/add_rule", apply: "/api/firewall/d_nat/apply", get: "/api/firewall/d_nat/get_rule/id", monad: "rule", run: runNATFailedRecovery},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var paths []string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				paths = append(paths, r.URL.Path)
				switch r.URL.Path {
				case tt.add:
					_, _ = io.WriteString(w, `{"result":"saved","uuid":"id"}`)
				case tt.apply:
					http.Error(w, "apply failed", 500)
				case tt.get:
					http.Error(w, "read failed", 500)
				default:
					t.Fatalf("unexpected endpoint %s", r.URL.Path)
				}
			}))
			defer server.Close()
			id, description := tt.run(t, server.URL)
			if id.ValueString() != "id" {
				t.Fatalf("id=%q", id.ValueString())
			}
			if !description.IsNull() {
				t.Fatalf("unverified description persisted: %#v", description)
			}
			for _, p := range paths {
				if strings.Contains(p, "savepoint") || strings.Contains(p, "cancel") || strings.Contains(p, "rollback") {
					t.Fatalf("legacy endpoint %s", p)
				}
			}
		})
	}
}

func TestCreatePostMutationReadOrValidationFailurePreservesIdentity(t *testing.T) {
	tests := []struct {
		name, add, apply, get, invalidBody string
		run                                func(*testing.T, string) (types.String, types.String)
	}{
		{name: "alias", add: "/api/firewall/alias/addItem", apply: "/api/firewall/alias/reconfigure", get: "/api/firewall/alias/getItem/id", invalidBody: `{"alias":{"updatefreq":"0"}}`, run: runAliasFailedRecovery},
		{name: "filter", add: "/api/firewall/filter/addRule", apply: "/api/firewall/filter/apply", get: "/api/firewall/filter/getRule/id", invalidBody: `{"rule":{"sequence":"0"}}`, run: runFilterFailedRecovery},
		{name: "NAT", add: "/api/firewall/d_nat/add_rule", apply: "/api/firewall/d_nat/apply", get: "/api/firewall/d_nat/get_rule/id", invalidBody: `{"rule":{"sequence":"0"}}`, run: runNATFailedRecovery},
	}
	for _, tt := range tests {
		for _, mode := range []string{"read", "validation"} {
			t.Run(tt.name+"/"+mode, func(t *testing.T) {
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					switch r.URL.Path {
					case tt.add:
						_, _ = io.WriteString(w, `{"result":"saved","uuid":"id"}`)
					case tt.apply:
						w.WriteHeader(http.StatusOK)
					case tt.get:
						if mode == "read" {
							http.Error(w, "read failed", http.StatusBadRequest)
						} else {
							_, _ = io.WriteString(w, tt.invalidBody)
						}
					default:
						t.Fatalf("unexpected endpoint %s", r.URL.Path)
					}
				}))
				defer server.Close()
				id, description := tt.run(t, server.URL)
				if id.ValueString() != "id" || !description.IsNull() {
					t.Fatalf("identity recovery = id %q description %#v", id.ValueString(), description)
				}
			})
		}
	}
}

func TestCreateApplyFailureSuccessfulRecoveryUsesAPIState(t *testing.T) {
	tests := []struct {
		name, add, apply, get, body string
		run                         func(*testing.T, string) (types.String, types.String)
	}{
		{name: "alias", add: "/api/firewall/alias/addItem", apply: "/api/firewall/alias/reconfigure", get: "/api/firewall/alias/getItem/id", body: `{"alias":{"name":"a","type":{"host":{"selected":1}},"content":"192.0.2.1","description":"remote","enabled":"1","proto":[],"interface":[],"categories":[],"updatefreq":"","path_expression":"","counters":"0"}}`, run: runAliasFailedRecovery},
		{name: "filter", add: "/api/firewall/filter/addRule", apply: "/api/firewall/filter/apply", get: "/api/firewall/filter/getRule/id", body: `{"rule":{"enabled":"0","sequence":"10","action":{"pass":{"selected":1}},"quick":"1","interface":[],"direction":{"in":{"selected":1}},"ipprotocol":{"inet":{"selected":1}},"protocol":{"TCP":{"selected":1}},"source_net":"any","source_port":"","source_not":"0","destination_net":"any","destination_port":"","destination_not":"0","description":"remote"}}`, run: runFilterFailedRecovery},
		{name: "NAT", add: "/api/firewall/d_nat/add_rule", apply: "/api/firewall/d_nat/apply", get: "/api/firewall/d_nat/get_rule/id", body: `{"rule":{"disabled":"1","sequence":"10","interface":{"wan":{"selected":1}},"ipprotocol":{"inet":{"selected":1}},"protocol":{"tcp":{"selected":1}},"source":{"network":"any","port":"","not":"0"},"destination":{"network":"any","port":"443","not":"0"},"target":"192.0.2.1","local-port":"443","descr":"remote"}}`, run: runNATFailedRecovery},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case tt.add:
					_, _ = io.WriteString(w, `{"result":"saved","uuid":"id"}`)
				case tt.apply:
					http.Error(w, "apply failed", http.StatusInternalServerError)
				case tt.get:
					_, _ = io.WriteString(w, tt.body)
				default:
					t.Fatalf("unexpected endpoint %s", r.URL.Path)
				}
			}))
			defer server.Close()
			id, description := tt.run(t, server.URL)
			if id.ValueString() != "id" || description.ValueString() != "remote" {
				t.Fatalf("API recovery = id %q description %#v", id.ValueString(), description)
			}
		})
	}
}

func runAliasFailedRecovery(t *testing.T, url string) (types.String, types.String) {
	r := &aliasResource{}
	configureResource(t, r, url)
	m := normalizeAlias(AliasResourceModel{Name: types.StringValue("a"), Type: types.StringValue("host"), Content: stringSetValue([]string{"192.0.2.1"}), Description: types.StringValue("planned"), Enabled: types.BoolValue(true), Proto: types.StringValue(""), Interface: types.StringValue(""), UpdateFreq: types.StringValue(""), PathExpression: types.StringValue(""), Stats: types.BoolValue(false)})
	state := resourceState(t, r, m)
	resp := resource.CreateResponse{State: tfsdk.State{Schema: state.Schema}}
	r.Create(context.Background(), resource.CreateRequest{Plan: tfsdk.Plan(state)}, &resp)
	if !resp.Diagnostics.HasError() {
		t.Fatal("expected failure")
	}
	var got AliasResourceModel
	resp.State.Get(context.Background(), &got)
	return got.ID, got.Description
}

func runFilterFailedRecovery(t *testing.T, url string) (types.String, types.String) {
	r := &filterRuleResource{}
	configureResource(t, r, url)
	m := filterFixture()
	state := resourceState(t, r, m)
	resp := resource.CreateResponse{State: tfsdk.State{Schema: state.Schema}}
	r.Create(context.Background(), resource.CreateRequest{Plan: tfsdk.Plan(state)}, &resp)
	if !resp.Diagnostics.HasError() {
		t.Fatal("expected failure")
	}
	var got FilterRuleResourceModel
	resp.State.Get(context.Background(), &got)
	return got.ID, got.Description
}

func runNATFailedRecovery(t *testing.T, url string) (types.String, types.String) {
	r := &natPortForwardResource{}
	configureResource(t, r, url)
	m := normalizeNAT(NatPortForwardResourceModel{Enabled: types.BoolValue(false), Sequence: types.Int64Value(10), Interface: types.StringValue("wan"), Interfaces: stringSetValue([]string{"wan"}), IPProtocol: types.StringValue("inet"), Protocol: types.StringValue("tcp"), SourceNet: types.StringValue("any"), SourcePort: types.StringValue(""), SourceNot: types.BoolValue(false), DestinationNet: types.StringValue("any"), DestinationPort: types.StringValue("443"), DestinationNot: types.BoolValue(false), Target: types.StringValue("192.0.2.1"), LocalPort: types.StringValue("443"), Log: types.BoolValue(false), Description: types.StringValue("planned"), Reflection: types.StringValue("")})
	state := resourceState(t, r, m)
	resp := resource.CreateResponse{State: tfsdk.State{Schema: state.Schema}}
	r.Create(context.Background(), resource.CreateRequest{Plan: tfsdk.Plan(state)}, &resp)
	if !resp.Diagnostics.HasError() {
		t.Fatal("expected failure")
	}
	var got NatPortForwardResourceModel
	resp.State.Get(context.Background(), &got)
	return got.ID, got.Description
}

func filterFixture() FilterRuleResourceModel {
	return FilterRuleResourceModel{Enabled: types.BoolValue(false), Sequence: types.Int64Value(10), Action: types.StringValue("pass"), Quick: types.BoolValue(true), Interface: stringSetValue(nil), Direction: types.StringValue("in"), IPProtocol: types.StringValue("inet"), Protocol: types.StringValue("tcp"), ICMPType: stringSetValue(nil), SourceNet: types.StringValue("any"), SourcePort: types.StringValue(""), SourceNot: types.BoolValue(false), DestinationNet: types.StringValue("any"), DestinationPort: types.StringValue(""), DestinationNot: types.BoolValue(false), Gateway: types.StringValue(""), TCPFlags: stringSetValue(nil), TCPFlagsOutOf: stringSetValue(nil), Description: types.StringValue("planned"), Categories: stringSetValue(nil)}
}

func resourceState(t *testing.T, r resource.Resource, m any) tfsdk.State {
	t.Helper()
	schema := resourceSchema(t, r)
	s := tfsdk.State{Schema: schema}
	if d := s.Set(context.Background(), m); d.HasError() {
		t.Fatal(d)
	}
	return s
}

func configureResource(t *testing.T, r resource.ResourceWithConfigure, url string) {
	t.Helper()
	var resp resource.ConfigureResponse
	r.Configure(context.Background(), resource.ConfigureRequest{ProviderData: newTestClient(t, url)}, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatal(resp.Diagnostics)
	}
}
