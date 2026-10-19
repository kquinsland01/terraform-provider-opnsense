// Copyright (c) Matthew Mellor
// SPDX-License-Identifier: MPL-2.0

package openvpn

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	dsschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/matthew-on-git/terraform-provider-opnsense/pkg/opnsense"
)

func TestDataSources_schemaIDs(t *testing.T) {
	t.Parallel()

	constructors := DataSources()
	if len(constructors) != 3 {
		t.Fatalf("expected 3 OpenVPN data sources, got %d", len(constructors))
	}

	for _, constructor := range constructors {
		assertRequiredID(t, constructor())
	}
}

func assertRequiredID(t *testing.T, ds datasource.DataSource) {
	t.Helper()

	var resp datasource.SchemaResponse
	ds.Schema(context.Background(), datasource.SchemaRequest{}, &resp)
	id, ok := resp.Schema.Attributes["id"].(dsschema.StringAttribute)
	if !ok {
		t.Fatalf("id attribute is %T, want StringAttribute", resp.Schema.Attributes["id"])
	}
	if !id.Required {
		t.Fatal("id attribute must be required")
	}
}

func TestInstanceDataSource_read(t *testing.T) {
	t.Parallel()

	const id = "instance-uuid"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case openVPNFirmwareStatusEndpoint:
			writeOpenVPNFirmware(w)
		case instanceReqOpts.GetEndpoint + "/" + id:
			_, _ = w.Write([]byte(`{"instance":{"vpnid":"srv1","enabled":"1","role":{"server":{"selected":1}},"description":"roadwarrior","dev_type":{"tun":{"selected":1}},"proto":{"udp":{"selected":1}},"port":"1194","local":"","remote":[],"server":"10.8.0.0/24","topology":{"subnet":{"selected":1}},"ca":{"ca":{"selected":1}},"cert":{"cert":{"selected":1}},"verify_client_cert":{"required":{"selected":1}},"tls_key":{"tls":{"selected":1}},"data-ciphers":{"AES-256-GCM":{"selected":1}},"auth":{"SHA256":{"selected":1}},"dns_servers":{"192.0.2.53":{"selected":1}},"push_route":{"192.0.2.0/24":{"selected":1}},"redirect_gateway":{"def1":{"selected":1}},"maxclients":"50","keepalive_interval":"10","keepalive_timeout":"60","verb":{"3":{"selected":1}}}}`))
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	}))
	t.Cleanup(server.Close)

	ds := newInstanceDataSource()
	configureDataSource(t, ds, newTestClient(t, server.URL))
	schema := dataSourceSchema(t, ds)
	config := InstanceResourceModel{
		ID:                 types.StringValue(id),
		Route:              types.SetNull(types.StringType),
		PushExcludedRoutes: types.SetNull(types.StringType),
		AuthMode:           types.SetNull(types.StringType),
		VariousFlags:       types.SetNull(types.StringType),
		VariousPushFlags:   types.SetNull(types.StringType),
		DNSDomain:          types.SetNull(types.StringType),
		DNSDomainSearch:    types.SetNull(types.StringType),
		NTPServers:         types.SetNull(types.StringType),
		DataCiphersOrdered: types.ListNull(types.StringType),
		DataCiphers:        types.SetNull(types.StringType),
		DNSServers:         types.SetNull(types.StringType),
		PushRoute:          types.SetNull(types.StringType),
		RedirectGateway:    types.SetNull(types.StringType),
	}
	resp := datasource.ReadResponse{State: tfsdk.State{Schema: schema}}

	ds.Read(context.Background(), datasource.ReadRequest{Config: modelConfig(t, schema, &config)}, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("read diagnostics: %v", resp.Diagnostics)
	}

	var state InstanceResourceModel
	if diags := resp.State.Get(context.Background(), &state); diags.HasError() {
		t.Fatalf("state diagnostics: %v", diags)
	}
	if state.VPNID.ValueString() != "srv1" || state.Role.ValueString() != "server" || state.MaxClients.ValueInt64() != 50 {
		t.Fatalf("unexpected state: %#v", state)
	}
}

func TestStaticKeyDataSourceStrictGETOnly(t *testing.T) {
	t.Parallel()
	const id = "123e4567-e89b-12d3-a456-426614174000"
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		requests++
		if request.Method != http.MethodGet {
			t.Fatalf("static-key lookup mutated appliance with %s %s", request.Method, request.URL.Path)
		}
		switch request.URL.Path {
		case "/api/core/firmware/status":
			_, _ = w.Write([]byte(`{"product":{"product_name":"OPNsense","product_id":"opnsense","product_version":"26.7.7"}}`))
		case staticKeyReqOpts.GetEndpoint + "/" + id:
			_, _ = w.Write([]byte(`{"statickey":{"mode":{"crypt":{"selected":1}},"description":"fixture","key":"sensitive-key-material"}}`))
		default:
			t.Fatalf("unexpected request %s %s", request.Method, request.URL.Path)
		}
	}))
	defer server.Close()
	ds := newStaticKeyDataSource()
	configureDataSource(t, ds, newTestClient(t, server.URL))
	schema := dataSourceSchema(t, ds)
	keyAttribute, ok := schema.Attributes["key"].(dsschema.StringAttribute)
	if !ok || !keyAttribute.Sensitive {
		t.Fatal("static-key data source key must be sensitive")
	}
	config := StaticKeyResourceModel{ID: types.StringValue(id)}
	response := datasource.ReadResponse{State: tfsdk.State{Schema: schema}}
	ds.Read(context.Background(), datasource.ReadRequest{Config: modelConfig(t, schema, &config)}, &response)
	if response.Diagnostics.HasError() {
		t.Fatal(response.Diagnostics)
	}
	var state StaticKeyResourceModel
	if diags := response.State.Get(context.Background(), &state); diags.HasError() {
		t.Fatal(diags)
	}
	if state.Mode.ValueString() != "crypt" || state.Description.ValueString() != "fixture" || state.Key.ValueString() == "" || requests != 2 {
		t.Fatalf("unexpected strict lookup mode=%q description=%q key_nonempty=%t requests=%d", state.Mode.ValueString(), state.Description.ValueString(), state.Key.ValueString() != "", requests)
	}
	badConfig := StaticKeyResourceModel{ID: types.StringValue("not-a-uuid")}
	badResponse := datasource.ReadResponse{State: tfsdk.State{Schema: schema}}
	ds.Read(context.Background(), datasource.ReadRequest{Config: modelConfig(t, schema, &badConfig)}, &badResponse)
	if !badResponse.Diagnostics.HasError() || requests != 2 {
		t.Fatalf("malformed UUID must fail before any request: diagnostics=%v requests=%d", badResponse.Diagnostics, requests)
	}
}

func TestStaticKeyDataSourceRejectsIncompleteResults(t *testing.T) {
	t.Parallel()
	const id = "123e4567-e89b-12d3-a456-426614174000"
	for name, body := range map[string]string{
		"blank key":  `{"statickey":{"mode":{"crypt":{"selected":1}},"description":"fixture","key":"  "}}`,
		"blank mode": `{"statickey":{"mode":{},"description":"fixture","key":"sensitive-key-material"}}`,
	} {
		name, body := name, body
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
				switch request.URL.Path {
				case openVPNFirmwareStatusEndpoint:
					writeOpenVPNFirmware(w)
				case staticKeyReqOpts.GetEndpoint + "/" + id:
					_, _ = w.Write([]byte(body))
				default:
					t.Fatalf("unexpected request %s %s", request.Method, request.URL.Path)
				}
			}))
			defer server.Close()
			ds := newStaticKeyDataSource()
			configureDataSource(t, ds, newTestClient(t, server.URL))
			schema := dataSourceSchema(t, ds)
			config := StaticKeyResourceModel{ID: types.StringValue(id)}
			response := datasource.ReadResponse{State: tfsdk.State{Schema: schema}}
			ds.Read(context.Background(), datasource.ReadRequest{Config: modelConfig(t, schema, &config)}, &response)
			if !response.Diagnostics.HasError() {
				t.Fatal("incomplete static-key result was accepted")
			}
		})
	}
}

func newTestClient(t *testing.T, baseURL string) *opnsense.Client {
	t.Helper()
	client, err := opnsense.NewClient(opnsense.ClientConfig{BaseURL: baseURL, APIKey: "key", APISecret: "secret", RetryMax: 1})
	if err != nil {
		t.Fatal(err)
	}
	return client
}

func configureDataSource(t *testing.T, ds datasource.DataSource, client *opnsense.Client) {
	t.Helper()
	configurable, ok := ds.(datasource.DataSourceWithConfigure)
	if !ok {
		t.Fatal("data source does not implement Configure")
	}
	var resp datasource.ConfigureResponse
	configurable.Configure(context.Background(), datasource.ConfigureRequest{ProviderData: client}, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("configure diagnostics: %v", resp.Diagnostics)
	}
}

func dataSourceSchema(t *testing.T, ds datasource.DataSource) dsschema.Schema {
	t.Helper()
	var resp datasource.SchemaResponse
	ds.Schema(context.Background(), datasource.SchemaRequest{}, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("schema diagnostics: %v", resp.Diagnostics)
	}
	return resp.Schema
}

func modelConfig(t *testing.T, schema dsschema.Schema, model any) tfsdk.Config {
	t.Helper()
	state := tfsdk.State{Schema: schema}
	if diags := state.Set(context.Background(), model); diags.HasError() {
		t.Fatalf("config diagnostics: %v", diags)
	}
	return tfsdk.Config{Raw: state.Raw, Schema: schema}
}
