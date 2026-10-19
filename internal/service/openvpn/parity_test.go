// Copyright (c) Matthew Mellor
// SPDX-License-Identifier: MPL-2.0

package openvpn

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/ephemeral"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestStableVersionAtLeast(t *testing.T) {
	t.Parallel()
	for _, value := range []string{"26.7.6", "26.7.7", "26.7.7_1", "27.1"} {
		ok, err := stableVersionAtLeast(value, "26.7.6")
		if err != nil || !ok {
			t.Fatalf("%s: ok=%t err=%v", value, ok, err)
		}
	}
	for _, value := range []string{"26.7.5", "26.7.r", "26.7.6-devel", "bad"} {
		ok, err := stableVersionAtLeast(value, "26.7.6")
		if value == "26.7.5" {
			if err != nil || ok {
				t.Fatalf("%s: ok=%t err=%v", value, ok, err)
			}
		} else if err == nil || ok {
			t.Fatalf("non-stable %s accepted", value)
		}
	}
}

func TestRequireOpenVPNParityVersionIdentity(t *testing.T) {
	t.Parallel()
	for name, body := range map[string]string{"supported": `{"product":{"product_name":"OPNsense","product_id":"opnsense","product_version":"26.7.7"}}`, "wrong product": `{"product":{"product_name":"OtherSense","product_id":"other","product_version":"26.7.7"}}`, "prerelease": `{"product":{"product_name":"OPNsense","product_id":"opnsense-devel","product_version":"26.7.6-devel"}}`} {
		t.Run(name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/api/core/firmware/status" {
					t.Fatalf("path=%s", r.URL.Path)
				}
				_, _ = w.Write([]byte(body))
			}))
			defer server.Close()
			err := requireOpenVPNParityVersion(context.Background(), newTestClient(t, server.URL))
			if name == "supported" && err != nil {
				t.Fatal(err)
			}
			if name != "supported" && err == nil {
				t.Fatal("expected compatibility error")
			}
		})
	}
}

func TestInstanceModelStrictDecodeAndSecretPreservation(t *testing.T) {
	t.Parallel()
	var response instanceAPIResponse
	if err := json.Unmarshal([]byte(`{"role":{"server":{"selected":1},"client":{"selected":1}}}`), &response); err == nil {
		t.Fatal("ambiguous selection accepted")
	}
	model := InstanceResourceModel{Password: types.StringValue("configured-secret"), AuthGenTokenSecret: types.StringValue("token-secret")}
	response = instanceAPIResponse{Enabled: "1", NoPool: "0", RemoteCertTLS: "0", UseOCSP: "0", UsernameAsCommonName: "0", ProvisionExclusive: "0", RegisterDNS: "0", MSSFix: "0", CompressMigrate: "0", IfConfigPoolPersist: "0"}
	if err := model.fromAPI(context.Background(), &response, "id"); err != nil {
		t.Fatal(err)
	}
	if model.Password.ValueString() != "configured-secret" || model.AuthGenTokenSecret.ValueString() != "token-secret" {
		t.Fatal("blank appliance response erased configured secret")
	}
	response.MaxClients = "not-a-number"
	if err := model.fromAPI(context.Background(), &response, "id"); err == nil {
		t.Fatal("malformed numeric value accepted")
	}
}

func TestInstanceIndexedChoiceDecodeIsStrictAndFieldIdentifying(t *testing.T) {
	t.Parallel()
	var response instanceAPIResponse
	if err := json.Unmarshal([]byte(`{"verb":[{"value":"zero","selected":0},{"value":"one","selected":"0"},{"value":"two","selected":1}],"strictusercn":[{"value":"No","selected":0},{"value":"Yes","selected":1}]}`), &response); err != nil {
		t.Fatal(err)
	}
	if response.Verb != "2" || response.StrictUserCN != "1" {
		t.Fatalf("indexed choices decoded as verb=%q strictusercn=%q", response.Verb, response.StrictUserCN)
	}
	for name, body := range map[string]string{
		"verb malformed marker":  `{"verb":[{"value":"zero","selected":2}]}`,
		"strictusercn ambiguous": `{"strictusercn":[{"value":"No","selected":1},{"value":"Yes","selected":1}]}`,
	} {
		err := json.Unmarshal([]byte(body), &response)
		field := strings.Fields(name)[0]
		if err == nil || !strings.Contains(strings.ToLower(err.Error()), field) {
			t.Fatalf("%s error must identify field: %v", name, err)
		}
	}
}

func TestStrictSelectedListNormalizesSoleSelectedEmptyKey(t *testing.T) {
	t.Parallel()
	var selected strictSelectedList
	if err := json.Unmarshal([]byte(`{"":{"selected":1}}`), &selected); err != nil {
		t.Fatal(err)
	}
	if len(selected) != 0 {
		t.Fatalf("sole selected empty key decoded as %#v, want empty list", selected)
	}
	var response instanceAPIResponse
	if err := json.Unmarshal([]byte(`{"route":{"":{"selected":1}},"push_route":{"":{"selected":1}},"push_excluded_routes":{"":{"selected":1}},"dns_domain":{"":{"selected":1}},"dns_domain_search":{"":{"selected":1}},"dns_servers":{"":{"selected":1}},"ntp_servers":{"":{"selected":1}}}`), &response); err != nil {
		t.Fatal(err)
	}
	for field, values := range map[string]strictSelectedList{
		"route": response.Route, "push_route": response.PushRoute, "push_excluded_routes": response.PushExcludedRoutes,
		"dns_domain": response.DNSDomain, "dns_domain_search": response.DNSDomainSearch, "dns_servers": response.DNSServers, "ntp_servers": response.NTPServers,
	} {
		if len(values) != 0 {
			t.Errorf("%s decoded as %#v, want empty list", field, values)
		}
	}
	for name, body := range map[string]string{
		"empty plus value": `{"":{"selected":1},"192.0.2.0/24":{"selected":1}}`,
		"duplicate empty":  `{"":{"selected":1},"":{"selected":1}}`,
	} {
		selected = nil
		if err := json.Unmarshal([]byte(body), &selected); err == nil || !strings.Contains(err.Error(), "ambiguous selected-map") {
			t.Errorf("%s must fail as ambiguous: %v", name, err)
		}
	}
}

func TestInstanceCreateAndReadResolveUnknownsFromAPITruth(t *testing.T) {
	t.Parallel()
	const id = "instance-id"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case instanceReqOpts.SearchEndpoint:
			_, _ = w.Write([]byte(`{"rows":[],"total":0,"rowCount":0,"current":1}`))
		case instanceReqOpts.AddEndpoint:
			_, _ = w.Write([]byte(`{"result":"saved","uuid":"` + id + `"}`))
		case instanceReqOpts.ReconfigureEndpoint:
			_, _ = w.Write([]byte(`{"status":"ok"}`))
		case instanceReqOpts.GetEndpoint + "/" + id:
			_, _ = w.Write([]byte(`{"instance":{"vpnid":"7","enabled":"0","role":"client","description":"api truth","dev_type":"tun","proto":"udp4","port":"1194","port-share":"api.example.test:443","remote":"vpn.example.test","topology":"subnet","ca":"","cert":"","crl":"","cert_depth":"","verify_client_cert":"none","tls_key":"","data-ciphers":{"AES-256-GCM":{"selected":1}},"data-ciphers-fallback":"","auth":"SHA256","authmode":{"Local Database":{"selected":1}},"local_group":"","strictusercn":[{"value":"No","selected":1}],"verb":[{"value":"0","selected":0},{"value":"1","selected":1}]}}`))
		default:
			t.Fatalf("unexpected request %s %s", request.Method, request.URL.Path)
		}
	}))
	defer server.Close()
	owner := &instanceResource{}
	configureOpenVPNResource(t, owner, server.URL)
	planModel := InstanceResourceModel{
		Enabled: types.BoolValue(false), Role: types.StringValue("client"), Description: types.StringValue("planned"), DevType: types.StringValue("tun"), Protocol: types.StringValue("udp4"),
		Port: types.StringValue("1194"), Remote: types.StringValue("vpn.example.test"), Topology: types.StringValue("subnet"), VerifyClientCert: types.StringValue("none"),
		PortShare: types.StringUnknown(), ServerIPv6: types.StringUnknown(), NoPool: types.BoolUnknown(), BridgeGateway: types.StringUnknown(), BridgePool: types.StringUnknown(),
		Route: types.SetUnknown(types.StringType), PushRoute: types.SetUnknown(types.StringType), PushExcludedRoutes: types.SetUnknown(types.StringType), CRL: types.StringUnknown(), CertDepth: types.StringUnknown(),
		RemoteCertTLS: types.BoolUnknown(), UseOCSP: types.BoolUnknown(), DataCiphers: openVPNStringSet("AES-256-GCM"), DataCiphersOrdered: types.ListUnknown(types.StringType),
		DataCiphersFallback: types.StringUnknown(), AuthMode: types.SetUnknown(types.StringType), LocalGroup: types.StringUnknown(), VariousFlags: types.SetUnknown(types.StringType),
		VariousPushFlags: types.SetUnknown(types.StringType), PushInactive: types.Int64Unknown(), UsernameAsCommonName: types.BoolUnknown(), StrictUserCN: types.StringUnknown(),
		Username: types.StringUnknown(), Password: types.StringUnknown(), RenegSec: types.Int64Unknown(), AuthGenToken: types.Int64Unknown(), AuthGenTokenRenewal: types.Int64Unknown(),
		AuthGenTokenSecret: types.StringUnknown(), ProvisionExclusive: types.BoolUnknown(), RedirectGateway: types.SetUnknown(types.StringType), RouteMetric: types.Int64Unknown(),
		RegisterDNS: types.BoolUnknown(), DNSDomain: types.SetUnknown(types.StringType), DNSDomainSearch: types.SetUnknown(types.StringType), DNSServers: types.SetUnknown(types.StringType),
		NTPServers: types.SetUnknown(types.StringType), TunMTU: types.Int64Unknown(), Fragment: types.Int64Unknown(), MSSFix: types.BoolUnknown(), CARPDependOn: types.StringUnknown(),
		CompressMigrate: types.BoolUnknown(), IfConfigPoolPersist: types.BoolUnknown(), HTTPProxy: types.StringUnknown(), VerifyX509Name: types.StringUnknown(), Verb: types.StringUnknown(),
	}
	plan := openVPNResourceState(t, owner, &planModel)
	create := resource.CreateResponse{State: tfsdk.State{Schema: plan.Schema}}
	owner.Create(context.Background(), resource.CreateRequest{Plan: tfsdk.Plan(plan)}, &create)
	if create.Diagnostics.HasError() {
		t.Fatal(create.Diagnostics)
	}
	var created InstanceResourceModel
	if diags := create.State.Get(context.Background(), &created); diags.HasError() {
		t.Fatal(diags)
	}
	assertNoUnknownInstanceValues(t, created)
	if created.PortShare.ValueString() != "api.example.test:443" || created.Description.ValueString() != "api truth" || created.Verb.ValueString() != "1" {
		t.Fatalf("create did not adopt API truth: port_share=%q description=%q verb=%q", created.PortShare.ValueString(), created.Description.ValueString(), created.Verb.ValueString())
	}
	created.Password = types.StringValue("configured-password")
	created.AuthGenTokenSecret = types.StringValue("configured-token")
	state := openVPNResourceState(t, owner, &created)
	read := resource.ReadResponse{State: state}
	owner.Read(context.Background(), resource.ReadRequest{State: state}, &read)
	if read.Diagnostics.HasError() {
		t.Fatal(read.Diagnostics)
	}
	var refreshed InstanceResourceModel
	if diags := read.State.Get(context.Background(), &refreshed); diags.HasError() {
		t.Fatal(diags)
	}
	if refreshed.Password.ValueString() != "configured-password" || refreshed.AuthGenTokenSecret.ValueString() != "configured-token" {
		t.Fatal("blank API secrets did not preserve configured state")
	}
}

func assertNoUnknownInstanceValues(t *testing.T, model InstanceResourceModel) {
	t.Helper()
	value := reflect.ValueOf(model)
	typeOf := value.Type()
	for index := 0; index < value.NumField(); index++ {
		candidate, ok := value.Field(index).Interface().(interface{ IsUnknown() bool })
		if ok && candidate.IsUnknown() {
			t.Errorf("instance field %s remained unknown after API read-back", typeOf.Field(index).Name)
		}
	}
}

func TestCollectionsUnknownAndExplicitEmpty(t *testing.T) {
	t.Parallel()
	unknown, err := collectionCSV(context.Background(), types.SetUnknown(types.StringType))
	if err != nil || unknown != nil {
		t.Fatalf("unknown=%v err=%v", unknown, err)
	}
	empty, err := collectionCSV(context.Background(), types.SetValueMust(types.StringType, nil))
	if err != nil || empty == nil || *empty != "" {
		t.Fatalf("empty=%v err=%v", empty, err)
	}
}

func TestInstanceValidatorRejectsPartialCredentialsAndCipherConflict(t *testing.T) {
	t.Parallel()
	owner := &instanceResource{}
	base := InstanceResourceModel{
		Role: types.StringValue("client"), Username: types.StringValue("known-user"), Password: types.StringUnknown(),
		Route: types.SetNull(types.StringType), PushRoute: types.SetNull(types.StringType), PushExcludedRoutes: types.SetNull(types.StringType),
		DataCiphers: openVPNStringSet("AES-256-GCM"), DataCiphersOrdered: types.ListValueMust(types.StringType, []attr.Value{types.StringValue("CHACHA20-POLY1305")}),
		AuthMode: types.SetNull(types.StringType), VariousFlags: types.SetNull(types.StringType), VariousPushFlags: types.SetNull(types.StringType),
		RedirectGateway: types.SetNull(types.StringType), DNSDomain: types.SetNull(types.StringType), DNSDomainSearch: types.SetNull(types.StringType),
		DNSServers: types.SetNull(types.StringType), NTPServers: types.SetNull(types.StringType),
	}
	state := openVPNResourceState(t, owner, &base)
	var response resource.ValidateConfigResponse
	owner.ConfigValidators(context.Background())[0].ValidateResource(context.Background(), resource.ValidateConfigRequest{Config: tfsdk.Config(state)}, &response)
	if !response.Diagnostics.HasError() || len(response.Diagnostics.Errors()) < 2 {
		t.Fatalf("partial credentials and conflicting cipher aliases must both be rejected: %v", response.Diagnostics)
	}
}

func TestAllAuditedInstanceFieldsRoundTripNonDefault(t *testing.T) {
	t.Parallel()
	const wire = `{"vpnid":"31","enabled":"0","role":"server","description":"story31c","dev_type":"tap","proto":"tcp4","port":"1195","port-share":"127.0.0.1:8443","local":"192.0.2.10","remote":"vpn.example.test","server":"10.31.0.0/24","server_ipv6":"2001:db8:31::/64","topology":"p2p","nopool":"1","bridge_gateway":"10.31.0.1","bridge_pool":"10.31.0.10-10.31.0.20","route":{"192.0.2.0/24":{"selected":1}},"push_route":{"198.51.100.0/24":{"selected":1}},"push_excluded_routes":{"203.0.113.0/24":{"selected":1}},"ca":"ca-ref","cert":"cert-ref","crl":"crl-ref","cert_depth":"3","remote_cert_tls":"1","verify_client_cert":"none","use_ocsp":"1","tls_key":"key-ref","data-ciphers":{"AES-256-GCM":{"selected":1},"CHACHA20-POLY1305":{"selected":1}},"data-ciphers-fallback":"AES-256-CBC","auth":"SHA512","authmode":{"Local Database":{"selected":1}},"local_group":"group-ref","various_flags":{"client-to-client":{"selected":1}},"various_push_flags":{"block-outside-dns":{"selected":1}},"push_inactive":"120","username_as_common_name":"1","strictusercn":"2","username":"story-user","password":"story-password","maxclients":"37","keepalive_interval":"11","keepalive_timeout":"61","reneg-sec":"3601","auth-gen-token":"7201","auth-gen-token-renewal":"1801","auth-gen-token-secret":"token-secret","provision_exclusive":"1","redirect_gateway":{"def1":{"selected":1}},"route_metric":"31","register_dns":"1","dns_domain":{"example.test":{"selected":1}},"dns_domain_search":{"search.example.test":{"selected":1}},"dns_servers":{"192.0.2.53":{"selected":1}},"ntp_servers":{"192.0.2.123":{"selected":1}},"tun_mtu":"1401","fragment":"1301","mssfix":"1","carp_depend_on":"carp-ref","compress_migrate":"1","ifconfig-pool-persist":"1","http-proxy":"proxy.example.test:3128","verify-x509-name":"vpn.example.test","verb":"7"}`
	var api instanceAPIResponse
	if err := json.Unmarshal([]byte(wire), &api); err != nil {
		t.Fatal(err)
	}
	var model InstanceResourceModel
	if err := model.fromAPI(context.Background(), &api, "instance-id"); err != nil {
		t.Fatal(err)
	}
	request, err := model.toAPI(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	assertRequestValue := func(field string, got *string, want string) {
		t.Helper()
		if got == nil || *got != want {
			t.Errorf("round-trip request field %s = %v, want %q", field, got, want)
		}
	}
	for _, check := range []struct {
		field string
		got   *string
		want  string
	}{
		{"port_share", request.PortShare, "127.0.0.1:8443"},
		{"server_ipv6", request.ServerIPv6, "2001:db8:31::/64"},
		{"no_pool", request.NoPool, "1"},
		{"bridge_gateway", request.BridgeGateway, "10.31.0.1"},
		{"bridge_pool", request.BridgePool, "10.31.0.10-10.31.0.20"},
		{"route", request.Route, "192.0.2.0/24"},
		{"push_excluded_routes", request.PushExcludedRoutes, "203.0.113.0/24"},
		{"crl", request.CRL, "crl-ref"},
		{"cert_depth", request.CertDepth, "3"},
		{"remote_cert_tls", request.RemoteCertTLS, "1"},
		{"use_ocsp", request.UseOCSP, "1"},
		{"data_ciphers", request.DataCiphers, "AES-256-GCM,CHACHA20-POLY1305"},
		{"data_ciphers_fallback", request.DataCiphersFallback, "AES-256-CBC"},
		{"auth_mode", request.AuthMode, "Local Database"},
		{"local_group", request.LocalGroup, "group-ref"},
		{"various_flags", request.VariousFlags, "client-to-client"},
		{"various_push_flags", request.VariousPushFlags, "block-outside-dns"},
		{"push_inactive", request.PushInactive, "120"},
		{"username_as_common_name", request.UsernameAsCommonName, "1"},
		{"strict_user_cn", request.StrictUserCN, "2"},
		{"username", request.Username, "story-user"},
		{"reneg_sec", request.RenegSec, "3601"},
		{"auth_gen_token", request.AuthGenToken, "7201"},
		{"auth_gen_token_renewal", request.AuthGenTokenRenewal, "1801"},
		{"provision_exclusive", request.ProvisionExclusive, "1"},
		{"redirect_gateway", request.RedirectGateway, "def1"},
		{"route_metric", request.RouteMetric, "31"},
		{"register_dns", request.RegisterDNS, "1"},
		{"dns_domain", request.DNSDomain, "example.test"},
		{"dns_domain_search", request.DNSDomainSearch, "search.example.test"},
		{"dns_servers", request.DNSServers, "192.0.2.53"},
		{"ntp_servers", request.NTPServers, "192.0.2.123"},
		{"tun_mtu", request.TunMTU, "1401"},
		{"fragment", request.Fragment, "1301"},
		{"mss_fix", request.MSSFix, "1"},
		{"carp_depend_on", request.CARPDependOn, "carp-ref"},
		{"compress_migrate", request.CompressMigrate, "1"},
		{"ifconfig_pool_persist", request.IfConfigPoolPersist, "1"},
		{"http_proxy", request.HTTPProxy, "proxy.example.test:3128"},
		{"verify_x509_name", request.VerifyX509Name, "vpn.example.test"},
	} {
		assertRequestValue(check.field, check.got, check.want)
	}
	if request.Password == nil || *request.Password != "story-password" {
		t.Error("password did not round-trip into the API request")
	}
	if request.AuthGenTokenSecret == nil || *request.AuthGenTokenSecret != "token-secret" {
		t.Error("authentication token secret did not round-trip into the API request")
	}
	if model.DataCiphersOrdered.IsNull() || len(model.DataCiphersOrdered.Elements()) != 2 {
		t.Fatal("ordered ciphers did not round-trip")
	}
}

func TestAllAuditedClientOverwriteFieldsRoundTripNonDefaultAndClear(t *testing.T) {
	t.Parallel()
	const wire = `{"enabled":"0","common_name":"story-client","description":"story31c","servers":{"server-ref":{"selected":1}},"block":"1","push_reset":"1","tunnel_network":"10.31.0.9/32","tunnel_networkv6":"2001:db8:31::9/128","local_networks":{"192.0.2.0/24":{"selected":1}},"remote_networks":{"198.51.100.0/24":{"selected":1}},"route_gateway":"10.31.0.1","redirect_gateway":{"def1":{"selected":1}},"register_dns":"1","dns_domain":{"example.test":{"selected":1}},"dns_domain_search":{"search.example.test":{"selected":1}},"dns_servers":{"192.0.2.53":{"selected":1}},"ntp_servers":{"192.0.2.123":{"selected":1}},"wins_servers":{"192.0.2.44":{"selected":1}}}`
	var api clientOverwriteAPIResponse
	if err := json.Unmarshal([]byte(wire), &api); err != nil {
		t.Fatal(err)
	}
	var model ClientOverwriteResourceModel
	if err := model.fromAPI(context.Background(), &api, "overwrite-id"); err != nil {
		t.Fatal(err)
	}
	request, err := model.toAPI(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	encoded, _ := json.Marshal(request)
	for _, want := range []string{`"tunnel_networkv6":"2001:db8:31::9/128"`, `"route_gateway":"10.31.0.1"`, `"redirect_gateway":"def1"`, `"register_dns":"1"`, `"dns_domain":"example.test"`, `"dns_domain_search":"search.example.test"`, `"dns_servers":"192.0.2.53"`, `"ntp_servers":"192.0.2.123"`, `"wins_servers":"192.0.2.44"`} {
		if !strings.Contains(string(encoded), want) {
			t.Errorf("round-trip request omitted or changed %s: %s", want, encoded)
		}
	}
	model.DNSDomain = types.SetValueMust(types.StringType, nil)
	cleared, err := model.toAPI(context.Background())
	if err != nil || cleared.DNSDomain == nil || *cleared.DNSDomain != "" {
		t.Fatalf("explicit clear not encoded: value=%v err=%v", cleared.DNSDomain, err)
	}
}

func TestGenerateKeyEphemeralPathAndResult(t *testing.T) {
	t.Parallel()
	const key = "generated-secret-marker"
	var generationCalls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/core/firmware/status":
			_, _ = w.Write([]byte(`{"product":{"product_name":"OPNsense","product_id":"opnsense","product_version":"26.7.7"}}`))
		case "/api/openvpn/instances/gen_key/tls-crypt-v2-server":
			generationCalls++
			if r.Method != http.MethodPost {
				t.Fatalf("method=%s", r.Method)
			}
			_, _ = w.Write([]byte(`{"result":"ok","key":"` + key + `"}`))
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()
	owner := &generateKeyEphemeral{}
	var configure ephemeral.ConfigureResponse
	owner.Configure(context.Background(), ephemeral.ConfigureRequest{ProviderData: newTestClient(t, server.URL)}, &configure)
	if configure.Diagnostics.HasError() {
		t.Fatal(configure.Diagnostics)
	}
	var schemaResp ephemeral.SchemaResponse
	owner.Schema(context.Background(), ephemeral.SchemaRequest{}, &schemaResp)
	state := tfsdk.State{Schema: schemaResp.Schema}
	model := generateKeyEphemeralModel{KeyType: types.StringValue("tls-crypt-v2-server"), Key: types.StringUnknown()}
	if diags := state.Set(context.Background(), &model); diags.HasError() {
		t.Fatal(diags)
	}
	response := ephemeral.OpenResponse{Result: tfsdk.EphemeralResultData{Schema: schemaResp.Schema}}
	owner.Open(context.Background(), ephemeral.OpenRequest{Config: tfsdk.Config{Raw: state.Raw, Schema: schemaResp.Schema}}, &response)
	if response.Diagnostics.HasError() {
		t.Fatal(response.Diagnostics)
	}
	if generationCalls != 1 {
		t.Fatalf("generation calls=%d", generationCalls)
	}
	var got generateKeyEphemeralModel
	if diags := response.Result.Get(context.Background(), &got); diags.HasError() {
		t.Fatal(diags)
	}
	if got.Key.ValueString() != key {
		t.Fatalf("key=%q", got.Key.ValueString())
	}
}

func TestGenerateKeyEphemeralDoesNotLeakBody(t *testing.T) {
	t.Parallel()
	const secret = "must-not-appear"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/core/firmware/status" {
			_, _ = w.Write([]byte(`{"product":{"product_name":"OPNsense","product_id":"opnsense","product_version":"26.7.7"}}`))
			return
		}
		_, _ = w.Write([]byte(`{"result":"failed","key":"` + secret + `"}`))
	}))
	defer server.Close()
	owner := &generateKeyEphemeral{}
	var configure ephemeral.ConfigureResponse
	owner.Configure(context.Background(), ephemeral.ConfigureRequest{ProviderData: newTestClient(t, server.URL)}, &configure)
	var schemaResp ephemeral.SchemaResponse
	owner.Schema(context.Background(), ephemeral.SchemaRequest{}, &schemaResp)
	state := tfsdk.State{Schema: schemaResp.Schema}
	_ = state.Set(context.Background(), &generateKeyEphemeralModel{KeyType: types.StringValue("secret"), Key: types.StringUnknown()})
	response := ephemeral.OpenResponse{Result: tfsdk.EphemeralResultData{Schema: schemaResp.Schema}}
	owner.Open(context.Background(), ephemeral.OpenRequest{Config: tfsdk.Config{Raw: state.Raw, Schema: schemaResp.Schema}}, &response)
	if !response.Diagnostics.HasError() {
		t.Fatal("failed result accepted")
	}
	for _, diag := range response.Diagnostics.Errors() {
		if strings.Contains(diag.Detail(), secret) {
			t.Fatal("secret leaked in diagnostic")
		}
	}
}

func TestGenerateKeyEphemeralRouteMatrix(t *testing.T) {
	t.Parallel()
	for _, keyType := range []string{"secret", "auth-token", "tls-auth", "tls-crypt", "tls-crypt-v2-server"} {
		keyType := keyType
		t.Run(keyType, func(t *testing.T) {
			t.Parallel()
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
				switch request.URL.Path {
				case openVPNFirmwareStatusEndpoint:
					writeOpenVPNFirmware(w)
				case "/api/openvpn/instances/gen_key/" + keyType:
					calls++
					if request.Method != http.MethodPost {
						t.Fatalf("method=%s", request.Method)
					}
					_, _ = w.Write([]byte(`{"result":"ok","key":"generated-` + keyType + `"}`))
				default:
					t.Fatalf("unexpected request %s %s", request.Method, request.URL.Path)
				}
			}))
			defer server.Close()
			response := openGeneratedKey(t, server.URL, keyType)
			if response.Diagnostics.HasError() || calls != 1 {
				t.Fatalf("diagnostics=%v calls=%d", response.Diagnostics, calls)
			}
			var result generateKeyEphemeralModel
			if diags := response.Result.Get(context.Background(), &result); diags.HasError() || result.Key.ValueString() != "generated-"+keyType {
				t.Fatalf("result=%#v diagnostics=%v", result, diags)
			}
		})
	}
}

func TestGenerateKeyEphemeralRejectsInvalidResponses(t *testing.T) {
	t.Parallel()
	tests := map[string]struct {
		status int
		body   string
	}{
		"non-2xx":       {status: http.StatusBadRequest, body: `{"result":"ok","key":"must-not-appear"}`},
		"malformed":     {status: http.StatusOK, body: `{"result":`},
		"trailing-json": {status: http.StatusOK, body: `{"result":"ok","key":"must-not-appear"}{}`},
		"trailing-data": {status: http.StatusOK, body: `{"result":"ok","key":"must-not-appear"} trailing`},
		"blank-key":     {status: http.StatusOK, body: `{"result":"ok","key":"  "}`},
		"failed-result": {status: http.StatusOK, body: `{"result":"failed","key":"must-not-appear"}`},
	}
	for name, test := range tests {
		name, test := name, test
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
				if request.URL.Path == openVPNFirmwareStatusEndpoint {
					writeOpenVPNFirmware(w)
					return
				}
				calls++
				w.WriteHeader(test.status)
				_, _ = w.Write([]byte(test.body))
			}))
			defer server.Close()
			response := openGeneratedKey(t, server.URL, "secret")
			if !response.Diagnostics.HasError() || calls != 1 {
				t.Fatalf("diagnostics=%v calls=%d", response.Diagnostics, calls)
			}
			for _, diagnostic := range response.Diagnostics.Errors() {
				if strings.Contains(diagnostic.Detail(), "must-not-appear") {
					t.Fatal("generated key leaked in diagnostics")
				}
			}
		})
	}
}

func openGeneratedKey(t *testing.T, baseURL, keyType string) ephemeral.OpenResponse {
	t.Helper()
	owner := &generateKeyEphemeral{}
	var configure ephemeral.ConfigureResponse
	owner.Configure(context.Background(), ephemeral.ConfigureRequest{ProviderData: newTestClient(t, baseURL)}, &configure)
	if configure.Diagnostics.HasError() {
		t.Fatal(configure.Diagnostics)
	}
	var schemaResponse ephemeral.SchemaResponse
	owner.Schema(context.Background(), ephemeral.SchemaRequest{}, &schemaResponse)
	state := tfsdk.State{Schema: schemaResponse.Schema}
	if diags := state.Set(context.Background(), &generateKeyEphemeralModel{KeyType: types.StringValue(keyType), Key: types.StringUnknown()}); diags.HasError() {
		t.Fatal(diags)
	}
	response := ephemeral.OpenResponse{Result: tfsdk.EphemeralResultData{Schema: schemaResponse.Schema}}
	owner.Open(context.Background(), ephemeral.OpenRequest{Config: tfsdk.Config{Raw: state.Raw, Schema: schemaResponse.Schema}}, &response)
	return response
}
