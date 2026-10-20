// Copyright (c) Matthew Mellor
// SPDX-License-Identifier: MPL-2.0

package wireguard

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/matthew-on-git/terraform-provider-opnsense/internal/tfconv"
	"github.com/matthew-on-git/terraform-provider-opnsense/pkg/opnsense"
)

func TestLegacyWireGuardStateUpgradePreservesMembers(t *testing.T) {
	for _, kind := range []string{"peer", "server"} {
		t.Run(kind, func(t *testing.T) {
			raw := `{"id":"uuid","enabled":false,"name":"legacy","tunnel_address":"10.0.0.1/24,fd00::1/64","description":"kept","private_key":"","port":"51820"}`
			schema := serverSchemaV1()
			if kind == "peer" {
				raw = `{"id":"uuid","enabled":false,"name":"legacy","public_key":"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=","tunnel_address":"10.0.0.1/24,fd00::1/64","server_address":"","server_port":"51820","keepalive":null,"servers":"first,second"}`
				schema = peerSchemaV1()
			}
			resp := resource.UpgradeStateResponse{State: tfsdk.State{Schema: schema}}
			upgraders := (&serverResource{}).UpgradeState(context.Background())
			if kind == "peer" {
				upgraders = (&peerResource{}).UpgradeState(context.Background())
			}
			upgrader, ok := upgraders[0]
			if !ok {
				t.Fatal("missing registered version0 upgrader")
			}
			upgrader.StateUpgrader(context.Background(), resource.UpgradeStateRequest{RawState: &tfprotov6.RawState{JSON: []byte(raw)}}, &resp)
			if resp.Diagnostics.HasError() {
				t.Fatal(resp.Diagnostics)
			}
			if kind == "peer" {
				var model PeerResourceModel
				if d := resp.State.Get(context.Background(), &model); d.HasError() {
					t.Fatal(d)
				}
				if len(model.TunnelAddress.Elements()) != 2 || len(model.Servers.Elements()) != 2 || model.ServerPort.ValueInt64() != 51820 {
					t.Fatalf("lost legacy state: %#v", model)
				}
			} else {
				var model ServerResourceModel
				if d := resp.State.Get(context.Background(), &model); d.HasError() {
					t.Fatal(d)
				}
				if len(model.TunnelAddress.Elements()) != 2 || model.Port.ValueInt64() != 51820 || model.Description.ValueString() != "kept" {
					t.Fatalf("lost legacy state: %#v", model)
				}
			}
		})
	}
}

func TestLegacyWireGuardMalformedCSVFailsClosed(t *testing.T) {
	for _, value := range []string{"10.0.0.1/24,", "10.0.0.1/24, fd00::1/64", "not-cidr"} {
		raw, _ := json.Marshal(map[string]any{"tunnel_address": value})
		resp := resource.UpgradeStateResponse{State: tfsdk.State{Schema: peerSchemaV1()}}
		upgradeLegacy(context.Background(), resource.UpgradeStateRequest{RawState: &tfprotov6.RawState{JSON: raw}}, &resp, "peer")
		if !resp.Diagnostics.HasError() {
			t.Fatalf("accepted malformed CSV %q", value)
		}
	}
}

func TestStrictWireGuardDecodersRejectAmbiguity(t *testing.T) {
	for _, raw := range []string{`null`, `true`, `"yes"`, `1`, `""`} {
		var b strictBool
		if json.Unmarshal([]byte(raw), &b) == nil {
			t.Errorf("accepted boolean %s", raw)
		}
	}
	for _, raw := range []string{`null`, `"abc"`, `"9223372036854775808"`, `1`} {
		var n strictOptionalInteger
		if json.Unmarshal([]byte(raw), &n) == nil {
			t.Errorf("accepted integer %s", raw)
		}
	}
	for _, raw := range []string{`null`, `["a"]`, `{"a":{}}`, `{"a":{"selected":2}}`, `{"a":{"selected":true}}`, `{"a":{"selected":"1"}}`, `"a,,b"`, `{"":{"selected":1}}`, `{"":{"value":"","selected":0}}`, `{"":{"value":"bad","selected":1}}`, `{"":{"value":"","selected":1},"10.0.0.1":{"selected":1}}`} {
		var s strictSelectedMapList
		if json.Unmarshal([]byte(raw), &s) == nil {
			t.Errorf("accepted selector %s", raw)
		}
	}
	for _, raw := range []string{`{}`, `null`, `{"enabled":"0"}`, `{"enabled":null}`} {
		var p wireguardPeerAPIResponse
		if json.Unmarshal([]byte(raw), &p) == nil {
			t.Errorf("accepted truncated peer %s", raw)
		}
		var s wireguardServerAPIResponse
		if json.Unmarshal([]byte(raw), &s) == nil {
			t.Errorf("accepted truncated server %s", raw)
		}
	}
}

func TestWireGuardPayloadAndSecrets(t *testing.T) {
	ctx := context.Background()
	key := "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA="
	peer := PeerResourceModel{Enabled: types.BoolValue(false), Name: types.StringValue("peer"), PublicKey: types.StringValue(key), PSK: types.StringValue(key), TunnelAddress: tfconv.SliceToSet([]string{"fd00::2/128", "10.0.0.2/32"})}
	payload, err := peer.toAPI(ctx)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(payload)
	if strings.Contains(string(raw), `"servers"`) {
		t.Fatal("peer must not write server associations")
	}
	if payload.TunnelAddress != "10.0.0.2/32,fd00::2/128" {
		t.Fatal("non-deterministic tunnel addresses")
	}
	if err = peer.fromAPI(ctx, &wireguardPeerAPIResponse{PublicKey: key, TunnelAddress: strictSelectedMapList{"10.0.0.2/32"}}, "peer-id"); err != nil {
		t.Fatal(err)
	}
	if peer.PSK.ValueString() != key {
		t.Fatal("blank API PSK erased prior secret")
	}
	server := ServerResourceModel{Enabled: types.BoolValue(false), Name: types.StringValue("server"), PrivateKey: types.StringValue(key), DNS: tfconv.SliceToSet([]string{"fd00::1", "10.0.0.1"}), TunnelAddress: tfconv.SliceToSet([]string{"10.0.0.1/24"}), Peers: tfconv.SliceToSet(nil), MTU: types.Int64Value(9300)}
	request, err := server.toAPI(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if request.Instance != "" || request.DNS != "10.0.0.1,fd00::1" || request.MTU != "9300" {
		t.Fatalf("unexpected payload: %#v", request)
	}
	if err = server.fromAPI(ctx, &wireguardServerAPIResponse{Instance: "3", Name: "server"}, "server-id"); err != nil {
		t.Fatal(err)
	}
	if server.PrivateKey.ValueString() != key || server.Instance.ValueString() != "3" {
		t.Fatal("lost secret or authoritative instance")
	}
	server.MTU = types.Int64Value(9301)
	if _, err = server.toAPI(ctx); err == nil {
		t.Fatal("accepted unsupported MTU")
	}
	server.MTU = types.Int64Null()
	server.Gateway = types.StringUnknown()
	if _, err = server.toAPI(ctx); err == nil {
		t.Fatal("unresolved gateway must not serialize a clear")
	}
}

type memoryPrivate struct{ values map[string][]byte }

func (p *memoryPrivate) GetKey(_ context.Context, key string) ([]byte, diag.Diagnostics) {
	return p.values[key], nil
}

func (p *memoryPrivate) SetKey(_ context.Context, key string, value []byte) diag.Diagnostics {
	if p.values == nil {
		p.values = map[string][]byte{}
	}
	p.values[key] = value
	return nil
}

func TestSettingsImportFirstAndStateOnlyDestroy(t *testing.T) {
	r := &settingsResource{}
	create := resource.CreateResponse{}
	r.Create(context.Background(), resource.CreateRequest{}, &create)
	if !create.Diagnostics.HasError() {
		t.Fatal("settings create must reject")
	}
	imp := resource.ImportStateResponse{State: tfsdk.State{Schema: settingsSchemaForTest(t)}}
	r.ImportState(context.Background(), resource.ImportStateRequest{ID: "wrong"}, &imp)
	if !imp.Diagnostics.HasError() {
		t.Fatal("wrong singleton ID accepted")
	}
	// A nil client and pending activation marker make accidental traffic obvious.
	private := &memoryPrivate{values: map[string][]byte{pendingActivationStateName: []byte(pendingMutation)}}
	del := resource.DeleteResponse{}
	r.Delete(context.Background(), resource.DeleteRequest{Private: nil}, &del)
	if del.Diagnostics.HasError() {
		t.Fatal(del.Diagnostics)
	}
	_ = private
}

func settingsSchemaForTest(t *testing.T) schema.Schema {
	t.Helper()
	var resp resource.SchemaResponse
	(&settingsResource{}).Schema(context.Background(), resource.SchemaRequest{}, &resp)
	return resp.Schema
}

func TestOwnerCreateRecoveryAndSerialization(t *testing.T) {
	ctx := context.Background()
	saved := false
	created := make(chan struct{})
	release := make(chan struct{})
	secondTraffic := make(chan struct{}, 1)
	var mu sync.Mutex
	searches := 0
	key := "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA="
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		switch r.URL.Path {
		case peerReqOpts.SearchEndpoint:
			searches++
			if searches > 2 {
				select {
				case secondTraffic <- struct{}{}:
				default:
				}
			}
			if saved {
				_, _ = w.Write([]byte(`{"rows":[{"uuid":"created-id","name":"peer"}],"total":1,"current":1,"rowCount":1}`))
			} else {
				_, _ = w.Write([]byte(`{"rows":[],"total":0,"current":1,"rowCount":0}`))
			}
		case peerReqOpts.AddEndpoint:
			saved = true
			close(created)
			<-release
			_, _ = w.Write([]byte(`{"result":"saved"}`))
		case peerReqOpts.GetEndpoint + "/created-id":
			_, _ = w.Write([]byte(`{"client":{"enabled":"0","name":"peer","pubkey":"` + key + `","psk":"","tunneladdress":"10.0.0.2/32","serveraddress":"","serverport":"","keepalive":""}}`))
		default:
			t.Errorf("unexpected request %s", r.URL.Path)
		}
	}))
	defer server.Close()
	client := newTestClient(t, server.URL)
	payload := &wireguardPeerAPIRequest{Enabled: "0", Name: "peer", PublicKey: key, TunnelAddress: "10.0.0.2/32"}
	done := make(chan error, 1)
	go func() {
		ownerMu.Lock()
		defer ownerMu.Unlock()
		id, err := createOwned(ctx, client, peerReqOpts, payload)
		if id != "created-id" || err == nil {
			done <- fmt.Errorf("recovery id=%q err=%v", id, err)
			return
		}
		done <- nil
	}()
	<-created
	attempted := make(chan struct{})
	secondDone := make(chan struct{})
	go func() {
		close(attempted)
		ownerMu.Lock()
		defer ownerMu.Unlock()
		defer close(secondDone)
		_, _ = createOwned(ctx, client, peerReqOpts, payload)
	}()
	<-attempted
	select {
	case <-secondTraffic:
		t.Fatal("competing owner emitted traffic before release")
	default:
	}
	close(release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	<-secondDone
}

func TestPeerSavedActivationFailureRetainsIdentityAndDoesNotRepeatMutation(t *testing.T) {
	key := "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA="
	addCalls, activationCalls := 0, 0
	allowActivation := false
	allowRead := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case peerReqOpts.SearchEndpoint:
			_, _ = w.Write([]byte(`{"rows":[],"total":0,"current":1,"rowCount":0}`))
		case peerReqOpts.AddEndpoint:
			addCalls++
			_, _ = w.Write([]byte(`{"result":"saved","uuid":"retained-id"}`))
		case peerReqOpts.ReconfigureEndpoint:
			activationCalls++
			if !allowActivation {
				_, _ = w.Write([]byte(`{"result":"failed"}`))
				return
			}
			_, _ = w.Write([]byte(`{"result":"ok"}`))
		case peerReqOpts.GetEndpoint + "/retained-id":
			if !allowRead {
				_, _ = w.Write([]byte(`{"client":{}}`))
				return
			}
			_, _ = w.Write([]byte(`{"client":{"enabled":"0","name":"peer","pubkey":"` + key + `","tunneladdress":{"10.0.0.2/32":{"selected":1}},"servers":[],"serveraddress":"","serverport":"","keepalive":""}}`))
		default:
			t.Errorf("unexpected request %s", r.URL.Path)
		}
	}))
	defer server.Close()
	owner := &peerResource{client: newTestClient(t, server.URL)}
	model := PeerResourceModel{ID: types.StringUnknown(), Enabled: types.BoolValue(false), Name: types.StringValue("peer"), PublicKey: types.StringValue(key), PSK: types.StringValue(key), TunnelAddress: tfconv.SliceToSet([]string{"10.0.0.2/32"}), Servers: types.SetUnknown(types.StringType)}
	state := tfsdk.State{Schema: peerSchemaV1()}
	if d := state.Set(context.Background(), &model); d.HasError() {
		t.Fatal(d)
	}
	create := resource.CreateResponse{State: tfsdk.State{Schema: state.Schema}}
	initializeWireGuardPrivate(&create)
	owner.Create(context.Background(), resource.CreateRequest{Plan: tfsdk.Plan(state)}, &create)
	if !create.Diagnostics.HasError() {
		t.Fatal("expected activation error")
	}
	var saved PeerResourceModel
	if d := create.State.Get(context.Background(), &saved); d.HasError() {
		t.Fatal(d)
	}
	if saved.ID.ValueString() != "retained-id" {
		t.Fatal("committed UUID lost")
	}
	pending, d := getPending(context.Background(), create.Private)
	if d.HasError() || pending != pendingMutation {
		t.Fatal("pending activation was not recorded")
	}
	allowActivation = true
	failedRead := resource.ReadResponse{State: create.State, Private: create.Private}
	owner.Read(context.Background(), resource.ReadRequest{State: create.State, Private: create.Private}, &failedRead)
	if !failedRead.Diagnostics.HasError() {
		t.Fatal("expected malformed read-back error")
	}
	pending, d = getPending(context.Background(), failedRead.Private)
	if d.HasError() || pending != pendingMutation {
		t.Fatal("read-back failure cleared pending activation")
	}
	allowRead = true
	read := resource.ReadResponse{State: create.State, Private: create.Private}
	owner.Read(context.Background(), resource.ReadRequest{State: create.State, Private: create.Private}, &read)
	if read.Diagnostics.HasError() {
		t.Fatal(read.Diagnostics)
	}
	if addCalls != 1 || activationCalls != 3 {
		t.Fatalf("add=%d activation=%d", addCalls, activationCalls)
	}
	if d := read.State.Get(context.Background(), &saved); d.HasError() {
		t.Fatal(d)
	}
	if saved.PSK.ValueString() != key {
		t.Fatal("refresh erased PSK")
	}
}

func initializeWireGuardPrivate(response any) {
	v := reflect.ValueOf(response).Elem().FieldByName("Private")
	v.Set(reflect.New(v.Type().Elem()))
}

func TestStrictPeerRefreshRetainsStateOnAmbiguity(t *testing.T) {
	for _, body := range []string{`{}`, `null`, `{"client":null}`, `{"client":{}}`, `{"other":[]}`, `{"client":{"enabled":"x"}}`, `[]`} {
		t.Run(body, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(body)) }))
			defer server.Close()
			owner := &peerResource{client: newTestClient(t, server.URL)}
			model := PeerResourceModel{ID: types.StringValue("retained"), TunnelAddress: tfconv.SliceToSet([]string{"10.0.0.2/32"}), Servers: tfconv.SliceToSet(nil)}
			state := tfsdk.State{Schema: peerSchemaV1()}
			if d := state.Set(context.Background(), &model); d.HasError() {
				t.Fatal(d)
			}
			response := resource.ReadResponse{State: state}
			owner.Read(context.Background(), resource.ReadRequest{State: state}, &response)
			if body == `[]` {
				if response.Diagnostics.HasError() || !response.State.Raw.IsNull() {
					t.Fatal("exact empty array must remove missing object")
				}
				return
			}
			if !response.Diagnostics.HasError() || !response.State.Raw.Equal(state.Raw) {
				t.Fatal("ambiguous read changed state or failed to diagnose")
			}
		})
	}
}

func TestSettingsUpdateUsesExactWireContractAndOneActivation(t *testing.T) {
	var paths []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.Method+" "+r.URL.Path)
		switch r.URL.Path {
		case settingsOpts.UpdateEndpoint:
			var body map[string]map[string]string
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Error(err)
			}
			if len(body) != 1 || len(body["general"]) != 1 || body["general"]["enabled"] != "0" {
				t.Error("unexpected settings payload")
			}
			_, _ = w.Write([]byte(`{"result":"saved"}`))
		case settingsOpts.ReconfigureEndpoint:
			_, _ = w.Write([]byte(`{"result":"ok"}`))
		case settingsOpts.GetEndpoint:
			_, _ = w.Write([]byte(`{"general":{"enabled":"0"}}`))
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()
	owner := &settingsResource{client: newTestClient(t, server.URL)}
	schema := settingsSchemaForTest(t)
	state := tfsdk.State{Schema: schema}
	if d := state.Set(context.Background(), &settingsModel{ID: types.StringValue(settingsID), Enabled: types.BoolValue(false)}); d.HasError() {
		t.Fatal(d)
	}
	response := resource.UpdateResponse{State: state}
	initializeWireGuardPrivate(&response)
	owner.Update(context.Background(), resource.UpdateRequest{Plan: tfsdk.Plan(state), State: state}, &response)
	if response.Diagnostics.HasError() {
		t.Fatal(response.Diagnostics)
	}
	want := []string{"POST " + settingsOpts.UpdateEndpoint, "GET " + settingsOpts.GetEndpoint, "POST " + settingsOpts.ReconfigureEndpoint}
	if !reflect.DeepEqual(paths, want) {
		t.Fatalf("calls=%v want=%v", paths, want)
	}
	paths = nil
	owner.Delete(context.Background(), resource.DeleteRequest{State: response.State, Private: response.Private}, &resource.DeleteResponse{})
	if len(paths) != 0 {
		t.Fatal("singleton destroy emitted appliance traffic")
	}
}

func TestPeerUpdatePreservesAuthoritativeServerAssociation(t *testing.T) {
	const key = "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA="
	const serverID = "00000000-0000-0000-0000-000000000001"
	const secondServerID = "00000000-0000-0000-0000-000000000002"
	for _, malformed := range []bool{false, true} {
		t.Run(fmt.Sprintf("malformed=%t", malformed), func(t *testing.T) {
			updates := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case peerReqOpts.GetEndpoint + "/peer-id":
					if malformed {
						_, _ = w.Write([]byte(`{"client":{}}`))
						return
					}
					_, _ = fmt.Fprintf(w, `{"client":{"enabled":"0","name":"peer","pubkey":%q,"tunneladdress":{"10.0.0.2/32":{"selected":1}},"servers":{%q:{"selected":1},%q:{"selected":1}},"serveraddress":"","serverport":"","keepalive":""}}`, key, secondServerID, serverID)
				case peerReqOpts.UpdateEndpoint + "/peer-id":
					updates++
					var body map[string]map[string]string
					if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
						t.Error(err)
					}
					if body["client"]["servers"] != serverID+","+secondServerID {
						t.Error("update did not preserve live API backreference")
					}
					_, _ = w.Write([]byte(`{"result":"saved"}`))
				case peerReqOpts.ReconfigureEndpoint:
					_, _ = w.Write([]byte(`{"result":"ok"}`))
				default:
					t.Errorf("unexpected path %s", r.URL.Path)
				}
			}))
			defer server.Close()
			owner := &peerResource{client: newTestClient(t, server.URL)}
			// Prior state is intentionally stale: the live association, not this empty
			// set, must be preserved during an unrelated peer update.
			model := PeerResourceModel{ID: types.StringValue("peer-id"), Enabled: types.BoolValue(false), Name: types.StringValue("peer"), PublicKey: types.StringValue(key), TunnelAddress: tfconv.SliceToSet([]string{"10.0.0.2/32"}), Servers: tfconv.SliceToSet(nil)}
			state := tfsdk.State{Schema: peerSchemaV1()}
			if d := state.Set(context.Background(), &model); d.HasError() {
				t.Fatal(d)
			}
			response := resource.UpdateResponse{State: state}
			initializeWireGuardPrivate(&response)
			owner.Update(context.Background(), resource.UpdateRequest{Plan: tfsdk.Plan(state), State: state}, &response)
			if malformed {
				if !response.Diagnostics.HasError() || updates != 0 {
					t.Fatal("malformed association read must prevent mutation")
				}
			} else if response.Diagnostics.HasError() || updates != 1 {
				t.Fatalf("updates=%d diagnostics=%v", updates, response.Diagnostics)
			}
		})
	}
}

func TestServerRequestsOmitCreateInstanceAndRetainAssignedUpdateInstance(t *testing.T) {
	const key = "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA="
	const firstPeer = "00000000-0000-0000-0000-000000000001"
	const secondPeer = "00000000-0000-0000-0000-000000000002"
	creates, updates := 0, 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == serverReqOpts.ReconfigureEndpoint {
			_, _ = w.Write([]byte(`{"result":"ok"}`))
			return
		}
		var body map[string]map[string]string
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		if body["server"]["peers"] != firstPeer+","+secondPeer {
			t.Error("server payload truncated peer associations")
		}
		switch r.URL.Path {
		case serverReqOpts.AddEndpoint:
			creates++
			if _, exists := body["server"]["instance"]; exists {
				t.Error("create must omit the API-assigned instance wire key")
			}
			_, _ = w.Write([]byte(`{"result":"saved","uuid":"server-id"}`))
		case serverReqOpts.UpdateEndpoint + "/server-id":
			updates++
			if body["server"]["instance"] != "3" {
				t.Error("update lost the authoritative assigned instance")
			}
			_, _ = w.Write([]byte(`{"result":"saved"}`))
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()
	client := newTestClient(t, server.URL)
	model := ServerResourceModel{Enabled: types.BoolValue(false), Name: types.StringValue("server"), PrivateKey: types.StringValue(key), Peers: tfconv.SliceToSet([]string{secondPeer, firstPeer})}
	payload, err := model.toAPI(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if _, err = opnsense.Add(context.Background(), client, serverReqOpts, payload); err != nil {
		t.Fatal(err)
	}
	if err = model.fromAPI(context.Background(), &wireguardServerAPIResponse{Instance: "3", Name: "server", Peers: strictSelectedMapList{firstPeer, secondPeer}}, "server-id"); err != nil {
		t.Fatal(err)
	}
	payload, err = model.toAPI(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if err = opnsense.Update(context.Background(), client, serverReqOpts, payload, "server-id"); err != nil {
		t.Fatal(err)
	}
	if creates != 1 || updates != 1 {
		t.Fatalf("creates=%d updates=%d", creates, updates)
	}
}

func TestReleasedEmptyAsListSentinel(t *testing.T) {
	var selected strictSelectedMapList
	if err := json.Unmarshal([]byte(`{"":{"value":"","selected":1}}`), &selected); err != nil {
		t.Fatal(err)
	}
	if len(selected) != 0 {
		t.Fatalf("empty released list decoded as %v", selected)
	}
}
