// Copyright (c) Matthew Mellor
// SPDX-License-Identifier: MPL-2.0

package kea

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/matthew-on-git/terraform-provider-opnsense/pkg/opnsense"
)

func TestGeneratedKeaOwnersRejectUnsupportedFirmwareBeforeItemUse(t *testing.T) {
	itemCalls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if request.URL.Path == keaFirmwareStatusEndpoint {
			_, _ = io.WriteString(w, `{"product":{"product_version":"26.7.6"}}`)
			return
		}
		itemCalls++
		http.Error(w, "must not be called", http.StatusInternalServerError)
	}))
	defer server.Close()
	client, err := opnsense.NewClient(opnsense.ClientConfig{BaseURL: server.URL, APIKey: "key", APISecret: "secret", RetryMax: 1})
	if err != nil {
		t.Fatal(err)
	}
	owner := &dhcpv6PeerResource{}
	var configured resource.ConfigureResponse
	owner.Configure(context.Background(), resource.ConfigureRequest{ProviderData: client}, &configured)
	if configured.Diagnostics.HasError() {
		t.Fatal(configured.Diagnostics)
	}
	model := DHCPv6PeerResourceModel{ID: types.StringValue("id"), Name: types.StringValue("peer"), Role: types.StringValue("primary"), URL: types.StringValue("http://[2001:db8::1]:8000/")}
	state := keaResourceState(t, owner, model)
	for _, operation := range []string{"create", "read", "update"} {
		t.Run(operation, func(t *testing.T) {
			switch operation {
			case "create":
				var response resource.CreateResponse
				owner.Create(context.Background(), resource.CreateRequest{Plan: tfsdk.Plan(state)}, &response)
				if !response.Diagnostics.HasError() {
					t.Fatal("expected compatibility diagnostic")
				}
			case "read":
				response := resource.ReadResponse{State: state}
				owner.Read(context.Background(), resource.ReadRequest{State: state}, &response)
				if !response.Diagnostics.HasError() {
					t.Fatal("expected compatibility diagnostic")
				}
			case "update":
				response := resource.UpdateResponse{State: state}
				owner.Update(context.Background(), resource.UpdateRequest{State: state, Plan: tfsdk.Plan(state)}, &response)
				if !response.Diagnostics.HasError() {
					t.Fatal("expected compatibility diagnostic")
				}
			}
		})
	}
	ds := newDHCPv6PeerDataSource()
	configureDataSource(t, ds, client)
	schema := dataSourceSchema(t, ds)
	request := datasource.ReadRequest{Config: modelConfig(t, schema, &DHCPv6PeerResourceModel{ID: types.StringValue("id")})}
	response := datasource.ReadResponse{State: tfsdk.State{Schema: schema}}
	ds.Read(context.Background(), request, &response)
	if !response.Diagnostics.HasError() {
		t.Fatal("data source accepted unsupported firmware")
	}
	if itemCalls != 0 {
		t.Fatalf("unsupported firmware reached item endpoints %d times", itemCalls)
	}
}

func TestDHCPv6PeerUsesSnakeCaseEndpointsAndOneReconfigure(t *testing.T) {
	var requestBody string
	reconfigureCalls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/api/kea/dhcpv6/add_peer":
			body, _ := io.ReadAll(request.Body)
			requestBody = string(body)
			_, _ = io.WriteString(w, `{"result":"saved","uuid":"peer-id"}`)
		case "/api/kea/service/reconfigure":
			reconfigureCalls++
			_, _ = io.WriteString(w, `{"status":"ok"}`)
		case "/api/kea/dhcpv6/get_peer/peer-id":
			_, _ = io.WriteString(w, `{"peer":{"name":"peer6","role":{"standby":{"selected":1}},"url":"http://[2001:db8::6]:8000/"}}`)
		default:
			t.Fatalf("unexpected request %s %s", request.Method, request.URL.Path)
		}
	}))
	t.Cleanup(server.Close)

	owner := &dhcpv6PeerResource{}
	configureKeaResource(t, owner, server.URL)
	plan := keaResourceState(t, owner, DHCPv6PeerResourceModel{Name: types.StringValue("peer6"), Role: types.StringValue("standby"), URL: types.StringValue("http://[2001:db8::6]:8000/")})
	response := resource.CreateResponse{State: tfsdk.State{Schema: plan.Schema}}
	owner.Create(context.Background(), resource.CreateRequest{Plan: tfsdk.Plan(plan)}, &response)
	if response.Diagnostics.HasError() {
		t.Fatal(response.Diagnostics)
	}
	if reconfigureCalls != 1 || !strings.Contains(requestBody, `"role":"standby"`) {
		t.Fatalf("reconfigure calls=%d request=%s", reconfigureCalls, requestBody)
	}
}

func TestDHCPv6PeerCreateFailureRecoversAPITruth(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/api/kea/dhcpv6/add_peer":
			_, _ = io.WriteString(w, `{"result":"saved","uuid":"peer-id"}`)
		case "/api/kea/service/reconfigure":
			http.Error(w, "reconfigure failed", http.StatusInternalServerError)
		case "/api/kea/dhcpv6/get_peer/peer-id":
			_, _ = io.WriteString(w, `{"peer":{"name":"remote-peer","role":{"primary":{"selected":1}},"url":"http://[2001:db8::7]:8000/"}}`)
		default:
			t.Fatalf("unexpected request %s %s", request.Method, request.URL.Path)
		}
	}))
	t.Cleanup(server.Close)

	owner := &dhcpv6PeerResource{}
	configureKeaResource(t, owner, server.URL)
	plan := keaResourceState(t, owner, DHCPv6PeerResourceModel{Name: types.StringValue("planned-peer"), Role: types.StringValue("standby"), URL: types.StringValue("http://[2001:db8::6]:8000/")})
	response := resource.CreateResponse{State: tfsdk.State{Schema: plan.Schema}}
	owner.Create(context.Background(), resource.CreateRequest{Plan: tfsdk.Plan(plan)}, &response)
	if !response.Diagnostics.HasError() {
		t.Fatal("expected reconfigure failure")
	}
	var state DHCPv6PeerResourceModel
	if diagnostics := response.State.Get(context.Background(), &state); diagnostics.HasError() {
		t.Fatal(diagnostics)
	}
	if state.ID.ValueString() != "peer-id" || state.Name.ValueString() != "remote-peer" || state.Role.ValueString() != "primary" {
		t.Fatalf("recoverable API truth not retained: %#v", state)
	}
}

func TestDHCPv6PeerPendingCreateActivationRetriesWithoutRepeatingMutation(t *testing.T) {
	addCalls, reconfigureCalls := 0, 0
	allowActivation := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/api/kea/dhcpv6/add_peer":
			addCalls++
			_, _ = io.WriteString(w, `{"result":"saved","uuid":"peer-id"}`)
		case "/api/kea/service/reconfigure":
			reconfigureCalls++
			if !allowActivation {
				http.Error(w, "failed", http.StatusInternalServerError)
				return
			}
			_, _ = io.WriteString(w, `{"status":"ok"}`)
		case "/api/kea/dhcpv6/get_peer/peer-id":
			_, _ = io.WriteString(w, `{"peer":{"name":"peer6","role":{"primary":{"selected":1}},"url":"http://[2001:db8::6]:8000/"}}`)
		default:
			t.Fatalf("unexpected request %s %s", request.Method, request.URL.Path)
		}
	}))
	t.Cleanup(server.Close)
	owner := &dhcpv6PeerResource{}
	configureKeaResource(t, owner, server.URL)
	plan := keaResourceState(t, owner, DHCPv6PeerResourceModel{Name: types.StringValue("peer6"), Role: types.StringValue("primary"), URL: types.StringValue("http://[2001:db8::6]:8000/")})
	create := resource.CreateResponse{State: tfsdk.State{Schema: plan.Schema}}
	initializePrivateState(t, &create)
	owner.Create(context.Background(), resource.CreateRequest{Plan: tfsdk.Plan(plan)}, &create)
	if !create.Diagnostics.HasError() {
		t.Fatal("expected initial reconfigure failure")
	}
	allowActivation = true
	pending, diags := getKeaPendingActivation(context.Background(), create.Private)
	if diags.HasError() || pending != keaPendingMutation {
		t.Fatalf("pending=%q diagnostics=%v", pending, diags)
	}

	read := resource.ReadResponse{State: create.State, Private: create.Private}
	owner.Read(context.Background(), resource.ReadRequest{State: create.State, Private: create.Private}, &read)
	if read.Diagnostics.HasError() {
		t.Fatal(read.Diagnostics)
	}
	pending, diags = getKeaPendingActivation(context.Background(), read.Private)
	if diags.HasError() || pending != "" {
		t.Fatalf("pending marker not cleared: %q diagnostics=%v", pending, diags)
	}
	if addCalls != 1 || reconfigureCalls != 3 {
		t.Fatalf("add calls=%d reconfigure calls=%d", addCalls, reconfigureCalls)
	}
}

func TestDHCPv6PeerPendingDeleteActivationRetriesWithoutRepeatingDelete(t *testing.T) {
	deleteCalls, reconfigureCalls := 0, 0
	allowActivation := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/api/kea/dhcpv6/del_peer/peer-id":
			deleteCalls++
			_, _ = io.WriteString(w, `{"result":"deleted"}`)
		case "/api/kea/service/reconfigure":
			reconfigureCalls++
			if !allowActivation {
				http.Error(w, "failed", http.StatusInternalServerError)
				return
			}
			_, _ = io.WriteString(w, `{"status":"ok"}`)
		default:
			t.Fatalf("unexpected request %s %s", request.Method, request.URL.Path)
		}
	}))
	t.Cleanup(server.Close)
	owner := &dhcpv6PeerResource{}
	configureKeaResource(t, owner, server.URL)
	state := keaResourceState(t, owner, DHCPv6PeerResourceModel{ID: types.StringValue("peer-id"), Name: types.StringValue("peer6"), Role: types.StringValue("primary"), URL: types.StringValue("http://[2001:db8::6]:8000/")})
	first := resource.DeleteResponse{}
	initializePrivateState(t, &first)
	owner.Delete(context.Background(), resource.DeleteRequest{State: state}, &first)
	if !first.Diagnostics.HasError() {
		t.Fatal("expected initial reconfigure failure")
	}
	allowActivation = true
	second := resource.DeleteResponse{Private: first.Private}
	owner.Delete(context.Background(), resource.DeleteRequest{State: state, Private: first.Private}, &second)
	if second.Diagnostics.HasError() {
		t.Fatal(second.Diagnostics)
	}
	if deleteCalls != 1 || reconfigureCalls != 3 {
		t.Fatalf("delete calls=%d reconfigure calls=%d", deleteCalls, reconfigureCalls)
	}
}

func TestDHCPv6PeerPendingUpdateActivationRetriesWithoutRepeatingMutation(t *testing.T) {
	updateCalls, reconfigureCalls := 0, 0
	allowActivation := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/api/kea/dhcpv6/set_peer/peer-id":
			updateCalls++
			_, _ = io.WriteString(w, `{"result":"saved"}`)
		case "/api/kea/service/reconfigure":
			reconfigureCalls++
			if !allowActivation {
				http.Error(w, "failed", http.StatusInternalServerError)
				return
			}
			_, _ = io.WriteString(w, `{"status":"ok"}`)
		case "/api/kea/dhcpv6/get_peer/peer-id":
			_, _ = io.WriteString(w, `{"peer":{"name":"updated","role":{"standby":{"selected":1}},"url":"http://[2001:db8::7]:8000/"}}`)
		default:
			t.Fatalf("unexpected request %s %s", request.Method, request.URL.Path)
		}
	}))
	t.Cleanup(server.Close)
	owner := &dhcpv6PeerResource{}
	configureKeaResource(t, owner, server.URL)
	state := keaResourceState(t, owner, DHCPv6PeerResourceModel{ID: types.StringValue("peer-id"), Name: types.StringValue("old"), Role: types.StringValue("primary"), URL: types.StringValue("http://[2001:db8::6]:8000/")})
	plan := keaResourceState(t, owner, DHCPv6PeerResourceModel{ID: types.StringValue("peer-id"), Name: types.StringValue("updated"), Role: types.StringValue("standby"), URL: types.StringValue("http://[2001:db8::7]:8000/")})
	first := resource.UpdateResponse{State: tfsdk.State{Schema: state.Schema}}
	initializePrivateState(t, &first)
	owner.Update(context.Background(), resource.UpdateRequest{State: state, Plan: tfsdk.Plan(plan)}, &first)
	if !first.Diagnostics.HasError() {
		t.Fatal("expected initial reconfigure failure")
	}
	allowActivation = true
	read := resource.ReadResponse{State: first.State, Private: first.Private}
	owner.Read(context.Background(), resource.ReadRequest{State: first.State, Private: first.Private}, &read)
	if read.Diagnostics.HasError() {
		t.Fatal(read.Diagnostics)
	}
	if updateCalls != 1 || reconfigureCalls != 3 {
		t.Fatalf("update calls=%d reconfigure calls=%d", updateCalls, reconfigureCalls)
	}
}

func TestAllStoryOwnersRetryPendingActivationBeforeRead(t *testing.T) {
	for _, test := range []struct {
		name, getPath, body string
		run                 func(*testing.T, string, *int)
	}{
		{"v4 subnet", "/api/kea/dhcpv4/get_subnet/id", `{"subnet4":{"subnet":"192.0.2.0/24","allocator":{},"match-client-id":"1","ping_check":"0","option_data_autocollect":"0","option_data":{"domain_name_servers":{},"domain_search":{},"routers":{},"ntp_servers":{},"time_servers":{}}}}`, func(t *testing.T, baseURL string, _ *int) {
			owner := &dhcpv4SubnetResource{}
			configureKeaResource(t, owner, baseURL)
			state := keaResourceState(t, owner, DHCPv4SubnetResourceModel{ID: types.StringValue("id"), DNSServers: types.SetNull(types.StringType), DomainSearch: types.SetNull(types.StringType), Routers: types.SetNull(types.StringType), StaticRoutes: types.SetNull(types.ObjectType{AttrTypes: staticRouteAttrTypes}), NTPServers: types.SetNull(types.StringType), TimeServers: types.SetNull(types.StringType)})
			response := resource.ReadResponse{State: state}
			initializePrivateState(t, &response)
			if diags := setKeaPendingActivation(context.Background(), response.Private, keaPendingMutation); diags.HasError() {
				t.Fatal(diags)
			}
			owner.Read(context.Background(), resource.ReadRequest{State: state, Private: response.Private}, &response)
			if response.Diagnostics.HasError() {
				t.Fatal(response.Diagnostics)
			}
		}},
		{"v6 subnet", "/api/kea/dhcpv6/get_subnet/id", `{"subnet6":{"subnet":"2001:db8::/64","interface":{"opt1":{"selected":1}},"option_data":{"dns_servers":{},"domain_search":{}}}}`, func(t *testing.T, baseURL string, _ *int) {
			owner := &dhcpv6SubnetResource{}
			configureKeaResource(t, owner, baseURL)
			state := keaResourceState(t, owner, DHCPv6SubnetResourceModel{ID: types.StringValue("id"), DNSServers: types.SetNull(types.StringType), DomainSearch: types.SetNull(types.StringType)})
			response := resource.ReadResponse{State: state}
			initializePrivateState(t, &response)
			if diags := setKeaPendingActivation(context.Background(), response.Private, keaPendingMutation); diags.HasError() {
				t.Fatal(diags)
			}
			owner.Read(context.Background(), resource.ReadRequest{State: state, Private: response.Private}, &response)
			if response.Diagnostics.HasError() {
				t.Fatal(response.Diagnostics)
			}
		}},
		{"v6 peer", "/api/kea/dhcpv6/get_peer/id", `{"peer":{"name":"peer","role":{"primary":{"selected":1}},"url":"http://[2001:db8::1]:8000/"}}`, func(t *testing.T, baseURL string, _ *int) {
			owner := &dhcpv6PeerResource{}
			configureKeaResource(t, owner, baseURL)
			state := keaResourceState(t, owner, DHCPv6PeerResourceModel{ID: types.StringValue("id")})
			response := resource.ReadResponse{State: state}
			initializePrivateState(t, &response)
			if diags := setKeaPendingActivation(context.Background(), response.Private, keaPendingMutation); diags.HasError() {
				t.Fatal(diags)
			}
			owner.Read(context.Background(), resource.ReadRequest{State: state, Private: response.Private}, &response)
			if response.Diagnostics.HasError() {
				t.Fatal(response.Diagnostics)
			}
		}},
		{"v6 PD pool", "/api/kea/dhcpv6/get_pd_pool/id", `{"pd_pool":{"subnet":{"parent":{"selected":1}},"prefix":"2001:db8::","prefix_len":"48","delegated_len":"64"}}`, func(t *testing.T, baseURL string, _ *int) {
			owner := &dhcpv6PdPoolResource{}
			configureKeaResource(t, owner, baseURL)
			state := keaResourceState(t, owner, DHCPv6PDPoolResourceModel{ID: types.StringValue("id")})
			response := resource.ReadResponse{State: state}
			initializePrivateState(t, &response)
			if diags := setKeaPendingActivation(context.Background(), response.Private, keaPendingMutation); diags.HasError() {
				t.Fatal(diags)
			}
			owner.Read(context.Background(), resource.ReadRequest{State: state, Private: response.Private}, &response)
			if response.Diagnostics.HasError() {
				t.Fatal(response.Diagnostics)
			}
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			reconfigureCalls, getCalls := 0, 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
				switch request.URL.Path {
				case "/api/kea/service/reconfigure":
					reconfigureCalls++
					_, _ = io.WriteString(w, `{"status":"ok"}`)
				case test.getPath:
					getCalls++
					_, _ = io.WriteString(w, test.body)
				default:
					t.Fatalf("unexpected request %s", request.URL.Path)
				}
			}))
			defer server.Close()
			test.run(t, server.URL, &reconfigureCalls)
			if reconfigureCalls != 1 || getCalls != 1 {
				t.Fatalf("reconfigure=%d get=%d", reconfigureCalls, getCalls)
			}
		})
	}
}

func TestPendingActivationMarkerSurvivesAmbiguousKeaResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/api/kea/service/reconfigure" {
			t.Fatalf("pending failure reached %s", request.URL.Path)
		}
		_, _ = io.WriteString(w, `{}`)
	}))
	defer server.Close()
	owner := &dhcpv4SubnetResource{}
	configureKeaResource(t, owner, server.URL)
	state := keaResourceState(t, owner, DHCPv4SubnetResourceModel{ID: types.StringValue("id"), DNSServers: types.SetNull(types.StringType), DomainSearch: types.SetNull(types.StringType), Routers: types.SetNull(types.StringType), StaticRoutes: types.SetNull(types.ObjectType{AttrTypes: staticRouteAttrTypes}), NTPServers: types.SetNull(types.StringType), TimeServers: types.SetNull(types.StringType)})
	response := resource.ReadResponse{State: state}
	initializePrivateState(t, &response)
	if diags := setKeaPendingActivation(context.Background(), response.Private, keaPendingMutation); diags.HasError() {
		t.Fatal(diags)
	}
	owner.Read(context.Background(), resource.ReadRequest{State: state, Private: response.Private}, &response)
	if !response.Diagnostics.HasError() {
		t.Fatal("ambiguous activation response accepted")
	}
	pending, diags := getKeaPendingActivation(context.Background(), response.Private)
	if diags.HasError() || pending != keaPendingMutation {
		t.Fatalf("pending marker lost: %q %v", pending, diags)
	}
}

func initializePrivateState(t *testing.T, response any) {
	t.Helper()
	value := reflect.ValueOf(response).Elem().FieldByName("Private")
	value.Set(reflect.New(value.Type().Elem()))
}

func TestDHCPv6PeerCreateFailureWithoutReadPreservesOnlyIdentity(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/api/kea/dhcpv6/add_peer":
			_, _ = io.WriteString(w, `{"result":"saved","uuid":"peer-id"}`)
		case "/api/kea/service/reconfigure", "/api/kea/dhcpv6/get_peer/peer-id":
			http.Error(w, "failed", http.StatusInternalServerError)
		default:
			t.Fatalf("unexpected request %s %s", request.Method, request.URL.Path)
		}
	}))
	t.Cleanup(server.Close)

	owner := &dhcpv6PeerResource{}
	configureKeaResource(t, owner, server.URL)
	plan := keaResourceState(t, owner, DHCPv6PeerResourceModel{Name: types.StringValue("planned-peer"), Role: types.StringValue("standby"), URL: types.StringValue("http://[2001:db8::6]:8000/")})
	response := resource.CreateResponse{State: tfsdk.State{Schema: plan.Schema}}
	owner.Create(context.Background(), resource.CreateRequest{Plan: tfsdk.Plan(plan)}, &response)
	if !response.Diagnostics.HasError() {
		t.Fatal("expected reconfigure failure")
	}
	var state DHCPv6PeerResourceModel
	if diagnostics := response.State.Get(context.Background(), &state); diagnostics.HasError() {
		t.Fatal(diagnostics)
	}
	if state.ID.ValueString() != "peer-id" || !state.Name.IsNull() || !state.Role.IsNull() || !state.URL.IsNull() {
		t.Fatalf("planned values were claimed as remote truth: %#v", state)
	}
}

func TestDHCPv4SubnetCreateFailureWithoutReadPreservesOnlyIdentity(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/api/kea/dhcpv4/add_subnet":
			_, _ = io.WriteString(w, `{"result":"saved","uuid":"subnet-id"}`)
		case "/api/kea/service/reconfigure", "/api/kea/dhcpv4/get_subnet/subnet-id":
			http.Error(w, "failed", http.StatusInternalServerError)
		default:
			t.Fatalf("unexpected request %s %s", request.Method, request.URL.Path)
		}
	}))
	t.Cleanup(server.Close)

	owner := &dhcpv4SubnetResource{}
	configureKeaResource(t, owner, server.URL)
	emptyStrings := types.SetValueMust(types.StringType, []attr.Value{})
	emptyRoutes := types.SetValueMust(types.ObjectType{AttrTypes: staticRouteAttrTypes}, []attr.Value{})
	model := DHCPv4SubnetResourceModel{
		Subnet: types.StringValue("192.0.2.0/24"), Allocator: types.StringValue(""), Pools: types.StringValue(""),
		MatchClientID: types.BoolValue(true), PingCheck: types.BoolValue(false), AutoCollect: types.BoolValue(false),
		DNSServers: emptyStrings, DomainName: types.StringValue(""), DomainSearch: emptyStrings, Routers: emptyStrings,
		StaticRoutes: emptyRoutes, NTPServers: emptyStrings, TimeServers: emptyStrings, NextServer: types.StringValue(""),
		TFTPServer: types.StringValue(""), TFTPBootfile: types.StringValue(""), Description: types.StringValue("planned"),
	}
	plan := keaResourceState(t, owner, model)
	response := resource.CreateResponse{State: tfsdk.State{Schema: plan.Schema}}
	owner.Create(context.Background(), resource.CreateRequest{Plan: tfsdk.Plan(plan)}, &response)
	if !response.Diagnostics.HasError() {
		t.Fatal("expected reconfigure failure")
	}
	var state DHCPv4SubnetResourceModel
	if diagnostics := response.State.Get(context.Background(), &state); diagnostics.HasError() {
		t.Fatal(diagnostics)
	}
	if state.ID.ValueString() != "subnet-id" || !state.Subnet.IsNull() || !state.Description.IsNull() {
		t.Fatalf("planned subnet values were claimed as remote truth: %#v", state)
	}
}

func TestDHCPv4SubnetCreateMalformedReadBackPreservesOnlyIdentity(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/api/kea/dhcpv4/add_subnet":
			_, _ = io.WriteString(w, `{"result":"saved","uuid":"subnet-id"}`)
		case "/api/kea/service/reconfigure":
			_, _ = io.WriteString(w, `{"status":"ok"}`)
		case "/api/kea/dhcpv4/get_subnet/subnet-id":
			_, _ = io.WriteString(w, `{"subnet4":{"subnet":"192.0.2.0/24","option_data":{"static_routes":"192.0.2.1"},"description":"remote"}}`)
		default:
			t.Fatalf("unexpected request %s %s", request.Method, request.URL.Path)
		}
	}))
	t.Cleanup(server.Close)

	owner := &dhcpv4SubnetResource{}
	configureKeaResource(t, owner, server.URL)
	emptyStrings := types.SetValueMust(types.StringType, []attr.Value{})
	emptyRoutes := types.SetValueMust(types.ObjectType{AttrTypes: staticRouteAttrTypes}, []attr.Value{})
	model := DHCPv4SubnetResourceModel{
		Subnet: types.StringValue("192.0.2.0/24"), Allocator: types.StringValue(""), Pools: types.StringValue(""),
		MatchClientID: types.BoolValue(true), PingCheck: types.BoolValue(false), AutoCollect: types.BoolValue(false),
		DNSServers: emptyStrings, DomainName: types.StringValue(""), DomainSearch: emptyStrings, Routers: emptyStrings,
		StaticRoutes: emptyRoutes, NTPServers: emptyStrings, TimeServers: emptyStrings, NextServer: types.StringValue(""),
		TFTPServer: types.StringValue(""), TFTPBootfile: types.StringValue(""), Description: types.StringValue("planned"),
	}
	plan := keaResourceState(t, owner, model)
	response := resource.CreateResponse{State: tfsdk.State{Schema: plan.Schema}}
	owner.Create(context.Background(), resource.CreateRequest{Plan: tfsdk.Plan(plan)}, &response)
	if !response.Diagnostics.HasError() {
		t.Fatal("expected malformed read-back diagnostic")
	}
	var state DHCPv4SubnetResourceModel
	if diagnostics := response.State.Get(context.Background(), &state); diagnostics.HasError() {
		t.Fatal(diagnostics)
	}
	if state.ID.ValueString() != "subnet-id" || !state.Subnet.IsNull() || !state.Description.IsNull() {
		t.Fatalf("malformed read-back retained planned values: %#v", state)
	}
}

func TestSubnetResourceReadsRejectMissingOptionListChildren(t *testing.T) {
	t.Run("DHCPv4", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = io.WriteString(w, `{"subnet4":{"subnet":"192.0.2.0/24","allocator":{},"match-client-id":"1","ping_check":"0","option_data_autocollect":"0","option_data":{"domain_name_servers":{},"domain_search":{},"routers":{},"ntp_servers":{}}}}`)
		}))
		defer server.Close()
		owner := &dhcpv4SubnetResource{}
		configureKeaResource(t, owner, server.URL)
		state := keaResourceState(t, owner, DHCPv4SubnetResourceModel{ID: types.StringValue("id"), DNSServers: types.SetNull(types.StringType), DomainSearch: types.SetNull(types.StringType), Routers: types.SetNull(types.StringType), StaticRoutes: types.SetNull(types.ObjectType{AttrTypes: staticRouteAttrTypes}), NTPServers: types.SetNull(types.StringType), TimeServers: types.SetNull(types.StringType)})
		response := resource.ReadResponse{State: state}
		owner.Read(context.Background(), resource.ReadRequest{State: state}, &response)
		if !response.Diagnostics.HasError() || !strings.Contains(response.Diagnostics.Errors()[0].Detail(), "time_servers") {
			t.Fatalf("missing option child diagnostics: %v", response.Diagnostics)
		}
	})

	t.Run("DHCPv6", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = io.WriteString(w, `{"subnet6":{"subnet":"2001:db8::/64","interface":{"lan":{"selected":1}},"option_data":{"dns_servers":{}}}}`)
		}))
		defer server.Close()
		owner := &dhcpv6SubnetResource{}
		configureKeaResource(t, owner, server.URL)
		state := keaResourceState(t, owner, DHCPv6SubnetResourceModel{ID: types.StringValue("id"), DNSServers: types.SetNull(types.StringType), DomainSearch: types.SetNull(types.StringType)})
		response := resource.ReadResponse{State: state}
		owner.Read(context.Background(), resource.ReadRequest{State: state}, &response)
		if !response.Diagnostics.HasError() || !strings.Contains(response.Diagnostics.Errors()[0].Detail(), "domain_search") {
			t.Fatalf("missing option child diagnostics: %v", response.Diagnostics)
		}
	})
}

func TestDHCPv6SubnetCreateFailureWithoutReadPreservesOnlyIdentity(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/api/kea/dhcpv6/add_subnet":
			_, _ = io.WriteString(w, `{"result":"saved","uuid":"subnet6-id"}`)
		case "/api/kea/service/reconfigure", "/api/kea/dhcpv6/get_subnet/subnet6-id":
			http.Error(w, "failed", http.StatusInternalServerError)
		default:
			t.Fatalf("unexpected request %s %s", request.Method, request.URL.Path)
		}
	}))
	t.Cleanup(server.Close)
	owner := &dhcpv6SubnetResource{}
	configureKeaResource(t, owner, server.URL)
	model := DHCPv6SubnetResourceModel{
		Subnet: types.StringValue("2001:db8:31::/48"), Interface: types.StringValue("opt1"), Allocator: types.StringValue(""), PDAllocator: types.StringValue(""), Pools: types.StringValue(""),
		DNSServers: stringSet(), DomainSearch: stringSet(), Description: types.StringValue("planned"),
	}
	plan := keaResourceState(t, owner, model)
	response := resource.CreateResponse{State: tfsdk.State{Schema: plan.Schema}}
	owner.Create(context.Background(), resource.CreateRequest{Plan: tfsdk.Plan(plan)}, &response)
	if !response.Diagnostics.HasError() {
		t.Fatal("expected reconfigure failure")
	}
	var state DHCPv6SubnetResourceModel
	if diagnostics := response.State.Get(context.Background(), &state); diagnostics.HasError() {
		t.Fatal(diagnostics)
	}
	if state.ID.ValueString() != "subnet6-id" || !state.Subnet.IsNull() || !state.Interface.IsNull() || !state.Description.IsNull() {
		t.Fatalf("planned DHCPv6 subnet values were claimed as remote truth: %#v", state)
	}
}

func TestDHCPv6PDPoolCreateFailureWithoutReadPreservesOnlyIdentity(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/api/kea/dhcpv6/add_pd_pool":
			_, _ = io.WriteString(w, `{"result":"saved","uuid":"pd-id"}`)
		case "/api/kea/service/reconfigure", "/api/kea/dhcpv6/get_pd_pool/pd-id":
			http.Error(w, "failed", http.StatusInternalServerError)
		default:
			t.Fatalf("unexpected request %s %s", request.Method, request.URL.Path)
		}
	}))
	t.Cleanup(server.Close)
	owner := &dhcpv6PdPoolResource{}
	configureKeaResource(t, owner, server.URL)
	plan := keaResourceState(t, owner, DHCPv6PDPoolResourceModel{
		SubnetID: types.StringValue("parent"), Prefix: types.StringValue("2001:db8:31::/48"), PrefixLength: types.Int64Value(48), DelegatedLength: types.Int64Value(64), Description: types.StringValue("planned"),
	})
	response := resource.CreateResponse{State: tfsdk.State{Schema: plan.Schema}}
	owner.Create(context.Background(), resource.CreateRequest{Plan: tfsdk.Plan(plan)}, &response)
	if !response.Diagnostics.HasError() {
		t.Fatal("expected reconfigure failure")
	}
	var state DHCPv6PDPoolResourceModel
	if diagnostics := response.State.Get(context.Background(), &state); diagnostics.HasError() {
		t.Fatal(diagnostics)
	}
	if state.ID.ValueString() != "pd-id" || !state.SubnetID.IsNull() || !state.Prefix.IsNull() || !state.Description.IsNull() {
		t.Fatalf("planned PD-pool values were claimed as remote truth: %#v", state)
	}
}

func TestDHCPv6PDPoolResourceReadDecodesValidLengths(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/api/kea/dhcpv6/get_pd_pool/pd-id" {
			t.Fatalf("unexpected request %s %s", request.Method, request.URL.Path)
		}
		_, _ = io.WriteString(w, `{"pd_pool":{"subnet":{"parent":{"selected":1}},"prefix":"2001:db8:31::/48","prefix_len":"48","delegated_len":"64","description":"remote"}}`)
	}))
	t.Cleanup(server.Close)
	owner := &dhcpv6PdPoolResource{}
	configureKeaResource(t, owner, server.URL)
	state := keaResourceState(t, owner, DHCPv6PDPoolResourceModel{ID: types.StringValue("pd-id"), SubnetID: types.StringValue("old"), Prefix: types.StringValue("2001:db8:31::/48"), PrefixLength: types.Int64Value(48), DelegatedLength: types.Int64Value(64), Description: types.StringValue("prior")})
	response := resource.ReadResponse{State: state}
	owner.Read(context.Background(), resource.ReadRequest{State: state}, &response)
	if response.Diagnostics.HasError() {
		t.Fatal(response.Diagnostics)
	}
	var refreshed DHCPv6PDPoolResourceModel
	if diagnostics := response.State.Get(context.Background(), &refreshed); diagnostics.HasError() {
		t.Fatal(diagnostics)
	}
	if refreshed.PrefixLength.ValueInt64() != 48 || refreshed.DelegatedLength.ValueInt64() != 64 || refreshed.SubnetID.ValueString() != "parent" {
		t.Fatalf("unexpected refreshed state: %#v", refreshed)
	}
}

func TestPeerAndPDPoolReadsRejectMalformedSelectedMaps(t *testing.T) {
	for _, test := range []struct {
		name, path, body string
		owner            resource.Resource
	}{
		{"peer missing role", "/api/kea/dhcpv6/get_peer/id", `{"peer":{"name":"peer","role":{},"url":"http://[2001:db8::1]:8000/"}}`, &dhcpv6PeerResource{}},
		{"PD multiple parents", "/api/kea/dhcpv6/get_pd_pool/id", `{"pd_pool":{"subnet":{"one":{"selected":1},"two":{"selected":1}},"prefix":"2001:db8::/48","prefix_len":"48","delegated_len":"64"}}`, &dhcpv6PdPoolResource{}},
	} {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
				if request.URL.Path != test.path {
					t.Fatalf("unexpected path %s", request.URL.Path)
				}
				_, _ = io.WriteString(w, test.body)
			}))
			defer server.Close()
			owner := test.owner.(resource.ResourceWithConfigure)
			configureKeaResource(t, owner, server.URL)
			var state tfsdk.State
			switch typed := owner.(type) {
			case *dhcpv6PeerResource:
				state = keaResourceState(t, typed, DHCPv6PeerResourceModel{ID: types.StringValue("id"), Name: types.StringValue("prior"), Role: types.StringValue("primary"), URL: types.StringValue("http://[2001:db8::1]:8000/")})
				response := resource.ReadResponse{State: state}
				typed.Read(context.Background(), resource.ReadRequest{State: state}, &response)
				if !response.Diagnostics.HasError() {
					t.Fatal("expected malformed peer diagnostic")
				}
			case *dhcpv6PdPoolResource:
				state = keaResourceState(t, typed, DHCPv6PDPoolResourceModel{ID: types.StringValue("id"), SubnetID: types.StringValue("prior"), Prefix: types.StringValue("2001:db8::/48"), PrefixLength: types.Int64Value(48), DelegatedLength: types.Int64Value(64), Description: types.StringValue("")})
				response := resource.ReadResponse{State: state}
				typed.Read(context.Background(), resource.ReadRequest{State: state}, &response)
				if !response.Diagnostics.HasError() {
					t.Fatal("expected malformed PD diagnostic")
				}
			}
		})
	}
}

func TestStoryOwnersRemoveStateForMissingObjects(t *testing.T) {
	t.Run("empty top-level array", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = io.WriteString(w, `[]`) }))
		defer server.Close()
		testStoryOwnersMissingAtURL(t, server.URL)
	})
	t.Run("empty monad array", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
			switch {
			case strings.Contains(request.URL.Path, "/dhcpv4/"):
				_, _ = io.WriteString(w, `{"subnet4":[]}`)
			case strings.Contains(request.URL.Path, "get_subnet"):
				_, _ = io.WriteString(w, `{"subnet6":[]}`)
			case strings.Contains(request.URL.Path, "get_peer"):
				_, _ = io.WriteString(w, `{"peer":[]}`)
			case strings.Contains(request.URL.Path, "get_pd_pool"):
				_, _ = io.WriteString(w, `{"pd_pool":[]}`)
			default:
				t.Fatalf("unexpected missing-object path %s", request.URL.Path)
			}
		}))
		defer server.Close()
		testStoryOwnersMissingAtURL(t, server.URL)
	})
}

func TestStoryOwnerDeletesDiagnoseAmbiguousHTTP404(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { http.NotFound(w, nil) }))
	defer server.Close()
	for _, test := range []struct {
		name string
		run  func(*testing.T)
	}{
		{"v4 subnet", func(t *testing.T) {
			owner := &dhcpv4SubnetResource{}
			configureKeaResource(t, owner, server.URL)
			state := keaResourceState(t, owner, DHCPv4SubnetResourceModel{ID: types.StringValue("missing"), DNSServers: types.SetNull(types.StringType), DomainSearch: types.SetNull(types.StringType), Routers: types.SetNull(types.StringType), StaticRoutes: types.SetNull(types.ObjectType{AttrTypes: staticRouteAttrTypes}), NTPServers: types.SetNull(types.StringType), TimeServers: types.SetNull(types.StringType)})
			var response resource.DeleteResponse
			owner.Delete(context.Background(), resource.DeleteRequest{State: state}, &response)
			if !response.Diagnostics.HasError() {
				t.Fatal("ambiguous HTTP 404 delete was accepted")
			}
		}},
		{"v6 subnet", func(t *testing.T) {
			owner := &dhcpv6SubnetResource{}
			configureKeaResource(t, owner, server.URL)
			state := keaResourceState(t, owner, DHCPv6SubnetResourceModel{ID: types.StringValue("missing"), DNSServers: types.SetNull(types.StringType), DomainSearch: types.SetNull(types.StringType)})
			var response resource.DeleteResponse
			owner.Delete(context.Background(), resource.DeleteRequest{State: state}, &response)
			if !response.Diagnostics.HasError() {
				t.Fatal("ambiguous HTTP 404 delete was accepted")
			}
		}},
		{"v6 peer", func(t *testing.T) {
			owner := &dhcpv6PeerResource{}
			configureKeaResource(t, owner, server.URL)
			state := keaResourceState(t, owner, DHCPv6PeerResourceModel{ID: types.StringValue("missing")})
			var response resource.DeleteResponse
			owner.Delete(context.Background(), resource.DeleteRequest{State: state}, &response)
			if !response.Diagnostics.HasError() {
				t.Fatal("ambiguous HTTP 404 delete was accepted")
			}
		}},
		{"v6 PD pool", func(t *testing.T) {
			owner := &dhcpv6PdPoolResource{}
			configureKeaResource(t, owner, server.URL)
			state := keaResourceState(t, owner, DHCPv6PDPoolResourceModel{ID: types.StringValue("missing")})
			var response resource.DeleteResponse
			owner.Delete(context.Background(), resource.DeleteRequest{State: state}, &response)
			if !response.Diagnostics.HasError() {
				t.Fatal("ambiguous HTTP 404 delete was accepted")
			}
		}},
	} {
		t.Run(test.name, test.run)
	}
}

func TestStoryOwnerResponsePoliciesAreStrict(t *testing.T) {
	for name, opts := range map[string]opnsense.ReqOpts{
		"v4 subnet":  dhcpv4SubnetReqOpts,
		"v6 subnet":  dhcpv6SubnetReqOpts,
		"v6 peer":    dhcpv6PeerReqOpts,
		"v6 PD pool": dhcpv6PdPoolReqOpts,
	} {
		if !opts.StrictDeleteResponse {
			t.Fatalf("%s does not require affirmative delete responses", name)
		}
		if !opts.StrictMissingObjectResponse {
			t.Fatalf("%s does not require source-proven empty-array missing responses", name)
		}
	}
}

func TestStoryOwnersRetainStateForAmbiguousHTTP404(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { http.NotFound(w, nil) }))
	defer server.Close()
	testStoryOwnersAtURL(t, server.URL, func(t *testing.T, state tfsdk.State, hasError bool) {
		if !hasError {
			t.Fatal("ambiguous HTTP 404 did not produce a diagnostic")
		}
		if state.Raw.IsNull() {
			t.Fatal("ambiguous HTTP 404 removed state")
		}
	})
}

func testStoryOwnersMissingAtURL(t *testing.T, baseURL string) {
	testStoryOwnersAtURL(t, baseURL, assertRemovedState)
}

func testStoryOwnersAtURL(t *testing.T, baseURL string, check func(*testing.T, tfsdk.State, bool)) {
	for _, test := range []struct {
		name string
		run  func(*testing.T)
	}{
		{"v4 subnet", func(t *testing.T) {
			owner := &dhcpv4SubnetResource{}
			configureKeaResource(t, owner, baseURL)
			state := keaResourceState(t, owner, DHCPv4SubnetResourceModel{ID: types.StringValue("missing"), DNSServers: types.SetNull(types.StringType), DomainSearch: types.SetNull(types.StringType), Routers: types.SetNull(types.StringType), StaticRoutes: types.SetNull(types.ObjectType{AttrTypes: staticRouteAttrTypes}), NTPServers: types.SetNull(types.StringType), TimeServers: types.SetNull(types.StringType)})
			response := resource.ReadResponse{State: state}
			owner.Read(context.Background(), resource.ReadRequest{State: state}, &response)
			check(t, response.State, response.Diagnostics.HasError())
		}},
		{"v6 subnet", func(t *testing.T) {
			owner := &dhcpv6SubnetResource{}
			configureKeaResource(t, owner, baseURL)
			state := keaResourceState(t, owner, DHCPv6SubnetResourceModel{ID: types.StringValue("missing"), DNSServers: types.SetNull(types.StringType), DomainSearch: types.SetNull(types.StringType)})
			response := resource.ReadResponse{State: state}
			owner.Read(context.Background(), resource.ReadRequest{State: state}, &response)
			check(t, response.State, response.Diagnostics.HasError())
		}},
		{"v6 peer", func(t *testing.T) {
			owner := &dhcpv6PeerResource{}
			configureKeaResource(t, owner, baseURL)
			state := keaResourceState(t, owner, DHCPv6PeerResourceModel{ID: types.StringValue("missing")})
			response := resource.ReadResponse{State: state}
			owner.Read(context.Background(), resource.ReadRequest{State: state}, &response)
			check(t, response.State, response.Diagnostics.HasError())
		}},
		{"v6 PD pool", func(t *testing.T) {
			owner := &dhcpv6PdPoolResource{}
			configureKeaResource(t, owner, baseURL)
			state := keaResourceState(t, owner, DHCPv6PDPoolResourceModel{ID: types.StringValue("missing")})
			response := resource.ReadResponse{State: state}
			owner.Read(context.Background(), resource.ReadRequest{State: state}, &response)
			check(t, response.State, response.Diagnostics.HasError())
		}},
	} {
		t.Run(test.name, test.run)
	}
}

func assertRemovedState(t *testing.T, state tfsdk.State, hasError bool) {
	t.Helper()
	if hasError {
		t.Fatal("missing object produced an error diagnostic")
	}
	if !state.Raw.IsNull() {
		t.Fatalf("missing object retained state: %s", state.Raw)
	}
}

func TestDHCPv6PDPoolResourceReadRejectsMalformedLengths(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/api/kea/dhcpv6/get_pd_pool/pd-id" {
			t.Fatalf("unexpected request %s %s", request.Method, request.URL.Path)
		}
		_, _ = io.WriteString(w, `{"pd_pool":{"subnet":{"subnet-id":{"selected":1}},"prefix":"2001:db8:31::","prefix_len":"not-a-number","delegated_len":"64","description":"remote"}}`)
	}))
	t.Cleanup(server.Close)

	owner := &dhcpv6PdPoolResource{}
	configureKeaResource(t, owner, server.URL)
	state := keaResourceState(t, owner, DHCPv6PDPoolResourceModel{
		ID: types.StringValue("pd-id"), SubnetID: types.StringValue("subnet-id"), Prefix: types.StringValue("2001:db8:31::"),
		PrefixLength: types.Int64Value(48), DelegatedLength: types.Int64Value(64), Description: types.StringValue("prior"),
	})
	response := resource.ReadResponse{State: state}
	owner.Read(context.Background(), resource.ReadRequest{State: state}, &response)
	if !response.Diagnostics.HasError() {
		t.Fatal("expected malformed prefix_len diagnostic")
	}
	var retained DHCPv6PDPoolResourceModel
	if diagnostics := response.State.Get(context.Background(), &retained); diagnostics.HasError() {
		t.Fatal(diagnostics)
	}
	if retained.PrefixLength.ValueInt64() != 48 || retained.Description.ValueString() != "prior" {
		t.Fatalf("malformed response overwrote prior state: %#v", retained)
	}
}

func TestDHCPv6PDPoolDataSourceReadRejectsOutOfRangeLengths(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/api/kea/dhcpv6/get_pd_pool/pd-id" {
			t.Fatalf("unexpected request %s %s", request.Method, request.URL.Path)
		}
		_, _ = io.WriteString(w, `{"pd_pool":{"subnet":{"subnet-id":{"selected":1}},"prefix":"2001:db8:31::","prefix_len":"48","delegated_len":"129","description":"remote"}}`)
	}))
	t.Cleanup(server.Close)

	ds := newDHCPv6PDPoolDataSource()
	configureDataSource(t, ds, testClient(t, server.URL))
	schema := dataSourceSchema(t, ds)
	request := datasource.ReadRequest{Config: modelConfig(t, schema, &DHCPv6PDPoolResourceModel{ID: types.StringValue("pd-id")})}
	response := datasource.ReadResponse{State: tfsdk.State{Schema: schema}}
	ds.Read(context.Background(), request, &response)
	if !response.Diagnostics.HasError() {
		t.Fatal("expected out-of-range delegated_len diagnostic")
	}
}

func configureKeaResource(t *testing.T, owner resource.ResourceWithConfigure, baseURL string) {
	t.Helper()
	var response resource.ConfigureResponse
	owner.Configure(context.Background(), resource.ConfigureRequest{ProviderData: testClient(t, baseURL)}, &response)
	if response.Diagnostics.HasError() {
		t.Fatal(response.Diagnostics)
	}
}
