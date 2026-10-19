// Copyright (c) Matthew Mellor
// SPDX-License-Identifier: MPL-2.0

package openvpn

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestOpenVPNInstanceOwnerRecovery(t *testing.T) {
	testOpenVPNOwnerRecovery(t, openVPNRecoveryOwner{
		name:       "instance",
		addPath:    "/api/openvpn/instances/add",
		setPath:    "/api/openvpn/instances/set/item-id",
		delPath:    "/api/openvpn/instances/del/item-id",
		getPath:    "/api/openvpn/instances/get/item-id",
		searchPath: "/api/openvpn/instances/search",
		searchBody: `{"rows":[{"uuid":"item-id","description":"collision-tag"}],"total":1,"rowCount":1,"current":1}`,
		getBody:    `{"instance":{"enabled":"0","role":"client","description":"collision-tag","dev_type":"tun","proto":"udp","remote":"vpn.example.test","topology":"subnet","verify_client_cert":"none","data-ciphers":"AES-256-GCM"}}`,
		newResource: func(baseURL string) resource.Resource {
			owner := &instanceResource{}
			configureOpenVPNResource(t, owner, baseURL)
			return owner
		},
		state: func(owner resource.Resource, description string) tfsdk.State {
			return openVPNResourceState(t, owner, &InstanceResourceModel{
				ID: types.StringValue("item-id"), Enabled: types.BoolValue(false), Role: types.StringValue("client"),
				Description: types.StringValue(description), DevType: types.StringValue("tun"), Protocol: types.StringValue("udp"),
				Remote: types.StringValue("vpn.example.test"), Topology: types.StringValue("subnet"), VerifyClientCert: types.StringValue("none"),
				Route: types.SetNull(types.StringType), PushRoute: types.SetNull(types.StringType), PushExcludedRoutes: types.SetNull(types.StringType),
				DataCiphers: openVPNStringSet("AES-256-GCM"), DataCiphersOrdered: types.ListNull(types.StringType), AuthMode: types.SetNull(types.StringType),
				VariousFlags: types.SetNull(types.StringType), VariousPushFlags: types.SetNull(types.StringType), RedirectGateway: types.SetNull(types.StringType),
				DNSDomain: types.SetNull(types.StringType), DNSDomainSearch: types.SetNull(types.StringType), DNSServers: types.SetNull(types.StringType), NTPServers: types.SetNull(types.StringType),
			})
		},
	})
}

func TestOpenVPNClientOverwriteOwnerRecovery(t *testing.T) {
	testOpenVPNOwnerRecovery(t, openVPNRecoveryOwner{
		name:       "client overwrite",
		addPath:    "/api/openvpn/client_overwrites/add",
		setPath:    "/api/openvpn/client_overwrites/set/item-id",
		delPath:    "/api/openvpn/client_overwrites/del/item-id",
		getPath:    "/api/openvpn/client_overwrites/get/item-id",
		searchPath: "/api/openvpn/client_overwrites/search",
		searchBody: `{"rows":[{"uuid":"item-id","common_name":"collision-tag"}],"total":1,"rowCount":1,"current":1}`,
		getBody:    `{"cso":{"enabled":"0","common_name":"collision-tag","description":"collision-tag","block":"0","push_reset":"0","register_dns":"0"}}`,
		newResource: func(baseURL string) resource.Resource {
			owner := &clientOverwriteResource{}
			configureOpenVPNResource(t, owner, baseURL)
			return owner
		},
		state: func(owner resource.Resource, description string) tfsdk.State {
			return openVPNResourceState(t, owner, &ClientOverwriteResourceModel{
				ID: types.StringValue("item-id"), Enabled: types.BoolValue(false), CommonName: types.StringValue("collision-tag"),
				Description: types.StringValue(description), Block: types.BoolValue(false), PushReset: types.BoolValue(false),
				Servers: types.SetNull(types.StringType), LocalNetworks: types.SetNull(types.StringType), RemoteNetworks: types.SetNull(types.StringType),
				RedirectGateway: types.SetNull(types.StringType), DNSDomain: types.SetNull(types.StringType), DNSDomainSearch: types.SetNull(types.StringType),
				DNSServers: types.SetNull(types.StringType), NTPServers: types.SetNull(types.StringType), WINSServers: types.SetNull(types.StringType),
			})
		},
	})
}

type openVPNRecoveryOwner struct {
	name, addPath, setPath, delPath, getPath, searchPath, searchBody, getBody string
	newResource                                                               func(string) resource.Resource
	state                                                                     func(resource.Resource, string) tfsdk.State
}

func testOpenVPNOwnerRecovery(t *testing.T, fixture openVPNRecoveryOwner) {
	t.Helper()
	t.Run("create activation failure retries without duplicate add", func(t *testing.T) {
		addCalls, reconfigureCalls := 0, 0
		allowActivation := false
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
			switch request.URL.Path {
			case openVPNFirmwareStatusEndpoint:
				writeOpenVPNFirmware(w)
			case fixture.searchPath:
				_, _ = io.WriteString(w, `{"rows":[],"total":0,"rowCount":0,"current":1}`)
			case fixture.addPath:
				addCalls++
				_, _ = io.WriteString(w, `{"result":"saved","uuid":"item-id"}`)
			case openVPNReconfigureEndpoint:
				reconfigureCalls++
				if !allowActivation {
					http.Error(w, "injected activation failure", http.StatusInternalServerError)
					return
				}
				_, _ = io.WriteString(w, `{"status":"ok"}`)
			case fixture.getPath:
				_, _ = io.WriteString(w, fixture.getBody)
			default:
				t.Fatalf("unexpected request %s %s", request.Method, request.URL.Path)
			}
		}))
		defer server.Close()
		owner := fixture.newResource(server.URL)
		plan := fixture.state(owner, "collision-tag")
		create := resource.CreateResponse{State: tfsdk.State{Schema: plan.Schema}}
		initializeOpenVPNPrivate(t, &create)
		owner.Create(context.Background(), resource.CreateRequest{Config: tfsdk.Config(plan), Plan: tfsdk.Plan(plan)}, &create)
		if !create.Diagnostics.HasError() {
			t.Fatal("expected injected activation failure")
		}
		allowActivation = true
		read := resource.ReadResponse{State: create.State, Private: create.Private}
		owner.Read(context.Background(), resource.ReadRequest{State: create.State, Private: create.Private}, &read)
		if read.Diagnostics.HasError() {
			t.Fatal(read.Diagnostics)
		}
		if addCalls != 1 || reconfigureCalls != 3 {
			t.Fatalf("add=%d reconfigure=%d", addCalls, reconfigureCalls)
		}
	})

	t.Run("ambiguous committed add recovers identity and retries activation", func(t *testing.T) {
		addCalls, reconfigureCalls, searchCalls := 0, 0, 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
			switch request.URL.Path {
			case openVPNFirmwareStatusEndpoint:
				writeOpenVPNFirmware(w)
			case fixture.addPath:
				addCalls++
				_, _ = io.WriteString(w, `{"result":"saved"}`)
			case fixture.searchPath:
				searchCalls++
				if searchCalls == 1 {
					_, _ = io.WriteString(w, `{"rows":[],"total":0,"rowCount":0,"current":1}`)
				} else {
					_, _ = io.WriteString(w, fixture.searchBody)
				}
			case openVPNReconfigureEndpoint:
				reconfigureCalls++
				_, _ = io.WriteString(w, `{"status":"ok"}`)
			case fixture.getPath:
				_, _ = io.WriteString(w, fixture.getBody)
			default:
				t.Fatalf("unexpected request %s %s", request.Method, request.URL.Path)
			}
		}))
		defer server.Close()
		owner := fixture.newResource(server.URL)
		plan := fixture.state(owner, "collision-tag")
		response := resource.CreateResponse{State: tfsdk.State{Schema: plan.Schema}}
		initializeOpenVPNPrivate(t, &response)
		owner.Create(context.Background(), resource.CreateRequest{Config: tfsdk.Config(plan), Plan: tfsdk.Plan(plan)}, &response)
		if !response.Diagnostics.HasError() {
			t.Fatal("ambiguous add response was accepted")
		}
		pending, diags := getOpenVPNPending(context.Background(), response.Private)
		if diags.HasError() || pending != openVPNPendingMutation {
			t.Fatalf("pending=%q diagnostics=%v", pending, diags)
		}
		read := resource.ReadResponse{State: response.State, Private: response.Private}
		owner.Read(context.Background(), resource.ReadRequest{State: response.State, Private: response.Private}, &read)
		if read.Diagnostics.HasError() {
			t.Fatal(read.Diagnostics)
		}
		if addCalls != 1 || reconfigureCalls != 1 {
			t.Fatalf("add=%d reconfigure=%d", addCalls, reconfigureCalls)
		}
	})

	t.Run("update activation retry does not repeat mutation", func(t *testing.T) {
		updateCalls, reconfigureCalls := 0, 0
		allowActivation := false
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
			switch request.URL.Path {
			case openVPNFirmwareStatusEndpoint:
				writeOpenVPNFirmware(w)
			case fixture.setPath:
				updateCalls++
				_, _ = io.WriteString(w, `{"result":"saved"}`)
			case openVPNReconfigureEndpoint:
				reconfigureCalls++
				if !allowActivation {
					http.Error(w, "injected activation failure", http.StatusInternalServerError)
					return
				}
				_, _ = io.WriteString(w, `{"status":"ok"}`)
			case fixture.getPath:
				body := strings.Replace(fixture.getBody, `"description":"collision-tag"`, `"description":"updated"`, 1)
				_, _ = io.WriteString(w, body)
			default:
				t.Fatalf("unexpected request %s %s", request.Method, request.URL.Path)
			}
		}))
		defer server.Close()
		owner := fixture.newResource(server.URL)
		state, plan := fixture.state(owner, "old"), fixture.state(owner, "updated")
		first := resource.UpdateResponse{State: tfsdk.State{Schema: state.Schema}}
		initializeOpenVPNPrivate(t, &first)
		owner.Update(context.Background(), resource.UpdateRequest{Config: tfsdk.Config(plan), State: state, Plan: tfsdk.Plan(plan)}, &first)
		if !first.Diagnostics.HasError() {
			t.Fatal("expected injected activation failure")
		}
		allowActivation = true
		second := resource.UpdateResponse{State: tfsdk.State{Schema: state.Schema}, Private: first.Private}
		owner.Update(context.Background(), resource.UpdateRequest{Config: tfsdk.Config(plan), State: first.State, Plan: tfsdk.Plan(plan), Private: first.Private}, &second)
		if second.Diagnostics.HasError() {
			t.Fatal(second.Diagnostics)
		}
		// The configured HTTP client retries the injected 500 once; critically,
		// the owner still performs exactly one object mutation.
		if updateCalls != 1 || reconfigureCalls != 3 {
			t.Fatalf("update=%d reconfigure=%d", updateCalls, reconfigureCalls)
		}
	})

	t.Run("ambiguous committed delete retains intent and retries activation", func(t *testing.T) {
		deleteCalls, reconfigureCalls, getCalls := 0, 0, 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
			switch request.URL.Path {
			case fixture.delPath:
				deleteCalls++
				_, _ = io.WriteString(w, `{}`)
			case fixture.getPath:
				getCalls++
				// The mutation committed even though its response was ambiguous.
				if fixture.name == "instance" {
					_, _ = io.WriteString(w, `{"instance":[]}`)
				} else {
					_, _ = io.WriteString(w, `{"cso":[]}`)
				}
			case openVPNReconfigureEndpoint:
				reconfigureCalls++
				_, _ = io.WriteString(w, `{"status":"ok"}`)
			default:
				t.Fatalf("unexpected request %s %s", request.Method, request.URL.Path)
			}
		}))
		defer server.Close()
		owner := fixture.newResource(server.URL)
		state := fixture.state(owner, "old")
		first := resource.DeleteResponse{}
		initializeOpenVPNPrivate(t, &first)
		owner.Delete(context.Background(), resource.DeleteRequest{State: state}, &first)
		if !first.Diagnostics.HasError() {
			t.Fatal("ambiguous delete response was accepted")
		}
		pending, diags := getOpenVPNPending(context.Background(), first.Private)
		if diags.HasError() || pending != openVPNPendingDelete {
			t.Fatalf("pending=%q diagnostics=%v", pending, diags)
		}
		second := resource.DeleteResponse{State: tfsdk.State{Schema: state.Schema}, Private: first.Private}
		owner.Delete(context.Background(), resource.DeleteRequest{State: state, Private: first.Private}, &second)
		if second.Diagnostics.HasError() {
			t.Fatal(second.Diagnostics)
		}
		if deleteCalls != 1 || reconfigureCalls != 1 || getCalls != 1 {
			t.Fatalf("delete=%d reconfigure=%d get=%d", deleteCalls, reconfigureCalls, getCalls)
		}
	})

	t.Run("delete activation failure retries without duplicate delete", func(t *testing.T) {
		deleteCalls, reconfigureCalls := 0, 0
		allowActivation := false
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
			switch request.URL.Path {
			case fixture.delPath:
				deleteCalls++
				_, _ = io.WriteString(w, `{"result":"deleted"}`)
			case openVPNReconfigureEndpoint:
				reconfigureCalls++
				if !allowActivation {
					http.Error(w, "injected activation failure", http.StatusInternalServerError)
					return
				}
				_, _ = io.WriteString(w, `{"status":"ok"}`)
			default:
				t.Fatalf("unexpected request %s %s", request.Method, request.URL.Path)
			}
		}))
		defer server.Close()
		owner := fixture.newResource(server.URL)
		state := fixture.state(owner, "old")
		first := resource.DeleteResponse{}
		initializeOpenVPNPrivate(t, &first)
		owner.Delete(context.Background(), resource.DeleteRequest{State: state}, &first)
		if !first.Diagnostics.HasError() {
			t.Fatal("expected injected activation failure")
		}
		allowActivation = true
		second := resource.DeleteResponse{State: tfsdk.State{Schema: state.Schema}, Private: first.Private}
		owner.Delete(context.Background(), resource.DeleteRequest{State: state, Private: first.Private}, &second)
		if second.Diagnostics.HasError() {
			t.Fatal(second.Diagnostics)
		}
		if deleteCalls != 1 || reconfigureCalls != 3 {
			t.Fatalf("delete=%d reconfigure=%d", deleteCalls, reconfigureCalls)
		}
	})
}

const (
	openVPNFirmwareStatusEndpoint = "/api/core/firmware/status"
	openVPNReconfigureEndpoint    = "/api/openvpn/service/reconfigure"
)

func writeOpenVPNFirmware(w http.ResponseWriter) {
	_, _ = io.WriteString(w, `{"product":{"product_name":"OPNsense","product_id":"opnsense","product_version":"26.7.7"}}`)
}

func configureOpenVPNResource(t *testing.T, owner resource.Resource, baseURL string) {
	t.Helper()
	configured, ok := owner.(resource.ResourceWithConfigure)
	if !ok {
		t.Fatal("owner does not implement Configure")
	}
	var response resource.ConfigureResponse
	configured.Configure(context.Background(), resource.ConfigureRequest{ProviderData: newTestClient(t, baseURL)}, &response)
	if response.Diagnostics.HasError() {
		t.Fatal(response.Diagnostics)
	}
}

func openVPNResourceState(t *testing.T, owner resource.Resource, model any) tfsdk.State {
	t.Helper()
	var response resource.SchemaResponse
	owner.Schema(context.Background(), resource.SchemaRequest{}, &response)
	state := tfsdk.State{Schema: response.Schema}
	if diags := state.Set(context.Background(), model); diags.HasError() {
		t.Fatal(diags)
	}
	return state
}

func initializeOpenVPNPrivate(t *testing.T, response any) {
	t.Helper()
	value := reflect.ValueOf(response).Elem().FieldByName("Private")
	value.Set(reflect.New(value.Type().Elem()))
}

func openVPNStringSet(values ...string) types.Set {
	items := make([]attr.Value, len(values))
	for i, value := range values {
		items[i] = types.StringValue(value)
	}
	return types.SetValueMust(types.StringType, items)
}
