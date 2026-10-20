// Copyright (c) Matthew Mellor
// SPDX-License-Identifier: MPL-2.0

package wireguard

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/matthew-on-git/terraform-provider-opnsense/internal/tfconv"
)

const (
	repairKey  = "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA="
	rotatedKey = "AQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQE="
)

func repairState(t *testing.T, kind, secret string) tfsdk.State {
	t.Helper()
	state := tfsdk.State{Schema: peerSchemaV1()}
	var model any = &PeerResourceModel{ID: types.StringValue("owned"), Enabled: types.BoolValue(false), Name: types.StringValue("peer"), PublicKey: types.StringValue(repairKey), PSK: types.StringValue(secret), TunnelAddress: tfconv.SliceToSet([]string{"10.0.0.2/32"}), Servers: tfconv.SliceToSet(nil)}
	if kind == "server" {
		state.Schema = serverSchemaV1()
		model = &ServerResourceModel{ID: types.StringValue("owned"), Enabled: types.BoolValue(false), Name: types.StringValue("server"), Instance: types.StringValue("4"), PublicKey: types.StringValue(repairKey), PrivateKey: types.StringValue(secret), TunnelAddress: tfconv.SliceToSet(nil), DNS: tfconv.SliceToSet(nil), Peers: tfconv.SliceToSet(nil)}
	}
	if d := state.Set(context.Background(), model); d.HasError() {
		t.Fatal(d)
	}
	return state
}

func repairBody(kind string) string {
	if kind == "peer" {
		return `{"client":{"enabled":"0","name":"peer","pubkey":"` + repairKey + `","psk":"","tunneladdress":{"10.0.0.2/32":{"selected":1}},"servers":[],"serveraddress":"","serverport":"","keepalive":""}}`
	}
	return `{"server":{"enabled":"0","name":"server","pubkey":"` + repairKey + `","privkey":"","instance":"4","port":"","mtu":"","dns":[],"tunneladdress":[],"peers":[],"disableroutes":"0","gateway":""}}`
}

func TestOwnerUpdateSecretFailureRecovery(t *testing.T) {
	for _, kind := range []string{"peer", "server"} {
		for _, confirmed := range []bool{true, false} {
			t.Run(fmt.Sprintf("%s/confirmed=%t", kind, confirmed), func(t *testing.T) {
				var updates atomic.Int32
				allowActivation := false
				api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					switch {
					case strings.Contains(r.URL.Path, "set_"):
						updates.Add(1)
						if confirmed {
							_, _ = fmt.Fprint(w, `{"result":"saved"}`)
						} else {
							_, _ = fmt.Fprint(w, `{}`)
						}
					case strings.Contains(r.URL.Path, "reconfigure"):
						if allowActivation {
							_, _ = fmt.Fprint(w, `{"result":"ok"}`)
						} else {
							_, _ = fmt.Fprint(w, `{"result":"failed"}`)
						}
					default:
						_, _ = fmt.Fprint(w, repairBody(kind))
					}
				}))
				defer api.Close()
				var owner resource.Resource = &peerResource{client: newTestClient(t, api.URL)}
				if kind == "server" {
					owner = &serverResource{client: newTestClient(t, api.URL)}
				}
				prior, planned := repairState(t, kind, repairKey), repairState(t, kind, rotatedKey)
				response := resource.UpdateResponse{State: prior}
				initializeWireGuardPrivate(&response)
				owner.Update(context.Background(), resource.UpdateRequest{State: prior, Plan: tfsdk.Plan(planned)}, &response)
				if !response.Diagnostics.HasError() {
					t.Fatal("expected failure")
				}
				field := "psk"
				if kind == "server" {
					field = "private_key"
				}
				var secret types.String
				if d := response.State.GetAttribute(context.Background(), path.Root(field), &secret); d.HasError() {
					t.Fatal(d)
				}
				if confirmed && secret.ValueString() != rotatedKey {
					t.Fatal("confirmed saved key rotation reverted")
				}
				if !confirmed && !secret.IsNull() {
					t.Fatal("ambiguous secret claimed as confirmed")
				}
				allowActivation = true
				refreshed := resource.ReadResponse{State: response.State, Private: response.Private}
				owner.Read(context.Background(), resource.ReadRequest{State: response.State, Private: response.Private}, &refreshed)
				if confirmed && refreshed.Diagnostics.HasError() {
					t.Fatal(refreshed.Diagnostics)
				}
				if !confirmed && !refreshed.Diagnostics.HasError() {
					t.Fatal("unreadable ambiguous key silently resolved")
				}
				if updates.Load() != 1 {
					t.Fatal("recovery repeated update")
				}
				if confirmed {
					retry := resource.UpdateResponse{State: refreshed.State, Private: refreshed.Private}
					// Explicit pending recovery must not return false success for changed intent.
					setPending(context.Background(), retry.Private, pendingMutation)
					owner.Update(context.Background(), resource.UpdateRequest{State: refreshed.State, Plan: tfsdk.Plan(prior), Private: retry.Private}, &retry)
					if !retry.Diagnostics.HasError() || updates.Load() != 1 {
						t.Fatal("pending changed plan returned success or mutated")
					}
				}
			})
		}
	}
}

func TestOwnerAmbiguousDeleteRecovery(t *testing.T) {
	for _, kind := range []string{"peer", "server"} {
		for _, outcome := range []string{"survives", "absent", "read_failure", "activation_failure"} {
			t.Run(kind+"/"+outcome, func(t *testing.T) {
				deletes := 0
				api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					switch {
					case strings.Contains(r.URL.Path, "reconfigure"):
						if outcome == "activation_failure" {
							_, _ = fmt.Fprint(w, `{}`)
						} else {
							_, _ = fmt.Fprint(w, `{"result":"ok"}`)
						}
					case strings.Contains(r.URL.Path, "del_"):
						deletes++
						_, _ = fmt.Fprint(w, `{"result":"deleted"}`)
					default:
						switch outcome {
						case "absent":
							_, _ = fmt.Fprint(w, `[]`)
						case "read_failure":
							_, _ = fmt.Fprint(w, `{}`)
						default:
							_, _ = fmt.Fprint(w, repairBody(kind))
						}
					}
				}))
				defer api.Close()
				var owner resource.Resource = &peerResource{client: newTestClient(t, api.URL)}
				if kind == "server" {
					owner = &serverResource{client: newTestClient(t, api.URL)}
				}
				response := resource.DeleteResponse{State: repairState(t, kind, repairKey)}
				initializeWireGuardPrivate(&response)
				setPending(context.Background(), response.Private, pendingDeleteUnknown)
				owner.Delete(context.Background(), resource.DeleteRequest{State: response.State, Private: response.Private}, &response)
				want := 0
				if outcome == "survives" {
					want = 1
				}
				if deletes != want {
					t.Fatalf("deletes=%d want=%d", deletes, want)
				}
				if (outcome == "read_failure" || outcome == "activation_failure") != response.Diagnostics.HasError() {
					t.Fatal(response.Diagnostics)
				}
			})
		}
	}
}

func TestSettingsAmbiguousSaveFailedReadRetainsPending(t *testing.T) {
	saves := 0
	failing := true
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case settingsOpts.UpdateEndpoint:
			saves++
			_, _ = fmt.Fprint(w, `{}`)
		case settingsOpts.GetEndpoint:
			if failing {
				_, _ = fmt.Fprint(w, `{}`)
			} else {
				_, _ = fmt.Fprint(w, `{"general":{"enabled":"0"}}`)
			}
		default:
			_, _ = fmt.Fprint(w, `{"result":"ok"}`)
		}
	}))
	defer api.Close()
	owner := &settingsResource{client: newTestClient(t, api.URL)}
	state := tfsdk.State{Schema: settingsSchemaForTest(t)}
	state.Set(context.Background(), &settingsModel{ID: types.StringValue(settingsID), Enabled: types.BoolValue(false)})
	response := resource.UpdateResponse{State: state}
	initializeWireGuardPrivate(&response)
	owner.Update(context.Background(), resource.UpdateRequest{State: state, Plan: tfsdk.Plan(state)}, &response)
	pending, d := getPending(context.Background(), response.Private)
	if !response.Diagnostics.HasError() || d.HasError() || pending != pendingMutation {
		t.Fatal("ambiguous save lost marker")
	}
	failing = false
	read := resource.ReadResponse{State: response.State, Private: response.Private}
	owner.Read(context.Background(), resource.ReadRequest{State: response.State, Private: response.Private}, &read)
	if read.Diagnostics.HasError() || saves != 1 {
		t.Fatal("refresh failed or repeated save", read.Diagnostics)
	}
}

func TestResourceEntryPointsSerializeRecovery(t *testing.T) {
	for _, kind := range []string{"peer", "server"} {
		t.Run(kind, func(t *testing.T) {
			entered := make(chan struct{})
			release := make(chan struct{})
			traffic := make(chan struct{}, 8)
			var searches atomic.Int32
			api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch {
				case strings.Contains(r.URL.Path, "search_"):
					count := searches.Add(1)
					if count == 1 {
						_, _ = fmt.Fprint(w, `{"rows":[],"total":0,"current":1,"rowCount":0}`)
						return
					}
					if count == 2 {
						close(entered)
						<-release
					} else {
						traffic <- struct{}{}
					}
					_, _ = fmt.Fprintf(w, `{"rows":[{"uuid":"owned","name":%q}],"total":1,"current":1,"rowCount":1}`, kind)
				case strings.Contains(r.URL.Path, "add_"):
					_, _ = fmt.Fprint(w, `{"result":"saved"}`)
				default:
					_, _ = fmt.Fprint(w, repairBody(kind))
				}
			}))
			defer api.Close()
			var owner resource.Resource = &peerResource{client: newTestClient(t, api.URL)}
			if kind == "server" {
				owner = &serverResource{client: newTestClient(t, api.URL)}
			}
			state := repairState(t, kind, repairKey)
			// Match recovery payload exactly, including blank optional fields.
			done := make(chan struct{})
			go func() {
				defer close(done)
				resp := resource.CreateResponse{State: state}
				initializeWireGuardPrivate(&resp)
				owner.Create(context.Background(), resource.CreateRequest{Plan: tfsdk.Plan(state)}, &resp)
			}()
			<-entered
			secondDone := make(chan struct{})
			started := make(chan struct{})
			go func() {
				defer close(secondDone)
				close(started)
				resp := resource.CreateResponse{State: state}
				owner.Create(context.Background(), resource.CreateRequest{Plan: tfsdk.Plan(state)}, &resp)
			}()
			<-started
			select {
			case <-traffic:
				close(release)
				t.Fatal("competing resource traffic while recovery owns lock")
			case <-time.After(100 * time.Millisecond):
			}
			close(release)
			<-done
			select {
			case <-secondDone:
			case <-time.After(3 * time.Second):
				t.Fatal("second resource did not progress")
			}
			if searches.Load() < 3 {
				t.Fatal("second owner did not reach API after release")
			}
		})
	}
}

func TestOptionalComputedNumbersDistinguishOmittedFromUnknown(t *testing.T) {
	for _, omitted := range []bool{true, false} {
		state := repairState(t, "peer", repairKey)
		configured := types.Int64Unknown()
		if omitted {
			configured = types.Int64Null()
		}
		if d := state.SetAttribute(context.Background(), path.Root("server_port"), configured); d.HasError() {
			t.Fatal(d)
		}
		planned := types.Int64Unknown()
		if d := resolveOmittedNumber(context.Background(), tfsdk.Config(state), "server_port", &planned); d.HasError() {
			t.Fatal(d)
		}
		if omitted && !planned.IsNull() {
			t.Fatal("omission did not resolve unset semantics")
		}
		if !omitted && !planned.IsUnknown() {
			t.Fatal("configured unknown became clear")
		}
		_, err := optionalRange("server_port", planned, 1, 65535)
		if omitted && err != nil {
			t.Fatal(err)
		}
		if !omitted && err == nil {
			t.Fatal("configured unresolved intent accepted")
		}
	}
}

func TestDeleteFailureGeneratesRecoveryMarkers(t *testing.T) {
	for _, kind := range []string{"peer", "server"} {
		for _, confirmed := range []bool{true, false} {
			t.Run(fmt.Sprintf("%s/confirmed=%t", kind, confirmed), func(t *testing.T) {
				recovery := false
				deletes := 0
				api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					switch {
					case strings.Contains(r.URL.Path, "del_"):
						deletes++
						if confirmed {
							_, _ = fmt.Fprint(w, `{"result":"deleted"}`)
						} else {
							_, _ = fmt.Fprint(w, `{}`)
						}
					case strings.Contains(r.URL.Path, "reconfigure"):
						if recovery {
							_, _ = fmt.Fprint(w, `{"result":"ok"}`)
						} else {
							_, _ = fmt.Fprint(w, `{}`)
						}
					default:
						if recovery {
							_, _ = fmt.Fprint(w, `[]`)
						} else {
							_, _ = fmt.Fprint(w, `{}`)
						}
					}
				}))
				defer api.Close()
				var owner resource.Resource = &peerResource{client: newTestClient(t, api.URL)}
				if kind == "server" {
					owner = &serverResource{client: newTestClient(t, api.URL)}
				}
				prior := repairState(t, kind, repairKey)
				initial := resource.DeleteResponse{State: prior}
				initializeWireGuardPrivate(&initial)
				owner.Delete(context.Background(), resource.DeleteRequest{State: prior}, &initial)
				marker, d := getPending(context.Background(), initial.Private)
				want := pendingDeleteUnknown
				if confirmed {
					want = pendingDelete
				}
				if !initial.Diagnostics.HasError() || d.HasError() || marker != want || !initial.State.Raw.Equal(prior.Raw) {
					t.Fatal("failed deletion lost state/private marker", initial.Diagnostics)
				}
				recovery = true
				retry := resource.DeleteResponse{State: initial.State, Private: initial.Private}
				owner.Delete(context.Background(), resource.DeleteRequest{State: initial.State, Private: initial.Private}, &retry)
				if retry.Diagnostics.HasError() || deletes != 1 {
					t.Fatal("delete recovery repeated mutation", retry.Diagnostics)
				}
				marker, d = getPending(context.Background(), retry.Private)
				if d.HasError() || marker != "" {
					t.Fatal("delete recovery marker was not cleared")
				}
			})
		}
	}
}

func TestConfiguredUnknownKeysPreventOwnerTraffic(t *testing.T) {
	for _, kind := range []string{"peer", "server"} {
		fields := []string{"public_key", "psk"}
		if kind == "server" {
			fields = []string{"public_key", "private_key"}
		}
		for _, field := range fields {
			for _, operation := range []string{"Create", "Update"} {
				t.Run(kind+"/"+field+"/"+operation, func(t *testing.T) {
					calls := 0
					api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { calls++; _, _ = fmt.Fprint(w, `{}`) }))
					defer api.Close()
					var owner resource.Resource = &peerResource{client: newTestClient(t, api.URL)}
					if kind == "server" {
						owner = &serverResource{client: newTestClient(t, api.URL)}
					}
					prior := repairState(t, kind, repairKey)
					planned := repairState(t, kind, repairKey)
					configured := repairState(t, kind, repairKey)
					if d := configured.SetAttribute(context.Background(), path.Root(field), types.StringUnknown()); d.HasError() {
						t.Fatal(d)
					}
					// Exercise a modifier-reused known prior value as well as an unknown plan.
					for _, unknownPlan := range []bool{false, true} {
						if unknownPlan {
							if d := planned.SetAttribute(context.Background(), path.Root(field), types.StringUnknown()); d.HasError() {
								t.Fatal(d)
							}
						}
						if operation == "Create" {
							response := resource.CreateResponse{State: prior}
							owner.Create(context.Background(), resource.CreateRequest{Plan: tfsdk.Plan(planned), Config: tfsdk.Config(configured)}, &response)
							if !response.Diagnostics.HasError() {
								t.Fatal("configured unknown key accepted")
							}
						} else {
							response := resource.UpdateResponse{State: prior}
							owner.Update(context.Background(), resource.UpdateRequest{State: prior, Plan: tfsdk.Plan(planned), Config: tfsdk.Config(configured)}, &response)
							if !response.Diagnostics.HasError() {
								t.Fatal("configured unknown key reused/cleared")
							}
						}
					}
					if calls != 0 {
						t.Fatal("unresolved configured key emitted API traffic")
					}
				})
			}
		}
	}
}

func TestMalformedAPISecretsRetainConfirmedState(t *testing.T) {
	for _, kind := range []string{"peer", "server"} {
		t.Run(kind, func(t *testing.T) {
			field, wire := "psk", "psk"
			if kind == "server" {
				field, wire = "private_key", "privkey"
			}
			malformed := "invalid-secret-must-not-appear"
			body := strings.Replace(repairBody(kind), `"`+wire+`":""`, `"`+wire+`":"`+malformed+`"`, 1)
			api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = fmt.Fprint(w, body) }))
			defer api.Close()
			var owner resource.Resource = &peerResource{client: newTestClient(t, api.URL)}
			if kind == "server" {
				owner = &serverResource{client: newTestClient(t, api.URL)}
			}
			prior := repairState(t, kind, repairKey)
			response := resource.ReadResponse{State: prior}
			owner.Read(context.Background(), resource.ReadRequest{State: prior}, &response)
			if !response.Diagnostics.HasError() || !response.State.Raw.Equal(prior.Raw) {
				t.Fatal("malformed secret changed confirmed state")
			}
			if strings.Contains(fmt.Sprint(response.Diagnostics), malformed) {
				t.Fatal("diagnostic exposed API secret")
			}
			var key types.String
			if d := response.State.GetAttribute(context.Background(), path.Root(field), &key); d.HasError() {
				t.Fatal(d)
			}
			if key.ValueString() != repairKey {
				t.Fatal("confirmed key overwritten")
			}
		})
	}
}

func TestAmbiguousSecretAuthoritativeConfirmation(t *testing.T) {
	for _, kind := range []string{"peer", "server"} {
		t.Run(kind, func(t *testing.T) {
			field, wire := "psk", "psk"
			if kind == "server" {
				field, wire = "private_key", "privkey"
			}
			saves, activations := 0, 0
			confirm := false
			api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch {
				case strings.Contains(r.URL.Path, "set_"):
					saves++
					_, _ = fmt.Fprint(w, `{}`)
				case strings.Contains(r.URL.Path, "reconfigure"):
					activations++
					_, _ = fmt.Fprint(w, `{"result":"ok"}`)
				default:
					body := repairBody(kind)
					if confirm {
						body = strings.Replace(body, `"`+wire+`":""`, `"`+wire+`":"`+rotatedKey+`"`, 1)
					}
					_, _ = fmt.Fprint(w, body)
				}
			}))
			defer api.Close()
			var owner resource.Resource = &peerResource{client: newTestClient(t, api.URL)}
			if kind == "server" {
				owner = &serverResource{client: newTestClient(t, api.URL)}
			}
			prior, planned := repairState(t, kind, repairKey), repairState(t, kind, rotatedKey)
			failed := resource.UpdateResponse{State: prior}
			initializeWireGuardPrivate(&failed)
			owner.Update(context.Background(), resource.UpdateRequest{State: prior, Plan: tfsdk.Plan(planned)}, &failed)
			if !failed.Diagnostics.HasError() {
				t.Fatal("expected ambiguous save")
			}
			journal, d := failed.Private.GetKey(context.Background(), secretReconciliationName)
			if d.HasError() || len(journal) == 0 {
				t.Fatal("missing reconciliation journal")
			}
			confirm = true
			read := resource.ReadResponse{State: failed.State, Private: failed.Private}
			owner.Read(context.Background(), resource.ReadRequest{State: failed.State, Private: failed.Private}, &read)
			if read.Diagnostics.HasError() {
				t.Fatal(read.Diagnostics)
			}
			journal, d = read.Private.GetKey(context.Background(), secretReconciliationName)
			if d.HasError() || len(journal) != 0 {
				t.Fatal("confirmation retained journal")
			}
			pending, d := getPending(context.Background(), read.Private)
			if d.HasError() || pending != "" {
				t.Fatal("confirmation retained pending marker")
			}
			confirm = false
			refreshed := resource.ReadResponse{State: read.State, Private: read.Private}
			owner.Read(context.Background(), resource.ReadRequest{State: read.State, Private: read.Private}, &refreshed)
			if refreshed.Diagnostics.HasError() {
				t.Fatal(refreshed.Diagnostics)
			}
			var key types.String
			if d := refreshed.State.GetAttribute(context.Background(), path.Root(field), &key); d.HasError() {
				t.Fatal(d)
			}
			if key.ValueString() != rotatedKey || saves != 1 || activations != 1 {
				t.Fatal("blank refresh lost confirmed key or repeated mutation/activation")
			}
		})
	}
}

func TestCreateRecoveryNormalizesSetsAndRejectsAmbiguousCandidates(t *testing.T) {
	const first = "00000000-0000-0000-0000-000000000001"
	const second = "00000000-0000-0000-0000-000000000002"
	for _, scenario := range []string{"reordered", "zero", "multiple", "malformed", "invalid_member"} {
		t.Run(scenario, func(t *testing.T) {
			searches := 0
			saves := 0
			api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case serverReqOpts.SearchEndpoint:
					searches++
					if searches == 1 {
						_, _ = fmt.Fprint(w, `{"rows":[],"total":0,"current":1,"rowCount":0}`)
						return
					}
					if scenario == "multiple" {
						_, _ = fmt.Fprint(w, `{"rows":[{"uuid":"new1","name":"created"},{"uuid":"new2","name":"created"}],"total":2,"current":1,"rowCount":2}`)
					} else {
						_, _ = fmt.Fprint(w, `{"rows":[{"uuid":"new1","name":"created"}],"total":1,"current":1,"rowCount":1}`)
					}
				case serverReqOpts.AddEndpoint:
					saves++
					_, _ = fmt.Fprint(w, `{"result":"saved"}`)
				default:
					fields := map[string]string{"name": "created", "enabled": "0", "tunneladdress": "fd00::1/64,10.0.0.1/24", "dns": "2001:db8::53,192.0.2.53", "peers": second + "," + first, "servers": second + "," + first}
					switch scenario {
					case "zero":
						fields["enabled"] = "1"
					case "malformed":
						fields["dns"] = "192.0.2.53,,2001:db8::53"
					case "invalid_member":
						fields["tunneladdress"] = "not-cidr"
					}
					if err := json.NewEncoder(w).Encode(map[string]any{"server": fields}); err != nil {
						t.Error(err)
					}
				}
			}))
			defer api.Close()
			payload := map[string]string{"name": "created", "enabled": "0", "tunneladdress": "10.0.0.1/24,fd00::1/64", "dns": "192.0.2.53,2001:db8::53", "peers": first + "," + second, "servers": first + "," + second}
			id, err := createOwned(context.Background(), newTestClient(t, api.URL), serverReqOpts, &payload)
			if err == nil || saves != 1 {
				t.Fatal("ambiguous response missing diagnostic or mutation repeated")
			}
			if scenario == "reordered" {
				if id != "new1" {
					t.Fatal("equivalent reordered sets were not recovered")
				}
			} else if id != "" {
				t.Fatal("zero/multiple/malformed candidates adopted a UUID")
			}
		})
	}
}

func TestOmittedComputedPublicKeyPreservesPrior(t *testing.T) {
	updates := 0
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "set_"):
			updates++
			var body map[string]map[string]string
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Error(err)
			}
			if body["server"]["pubkey"] != repairKey || body["server"]["instance"] != "4" {
				t.Error("omitted computed key/instance lost prior API value")
			}
			_, _ = fmt.Fprint(w, `{"result":"saved"}`)
		case strings.Contains(r.URL.Path, "reconfigure"):
			_, _ = fmt.Fprint(w, `{"result":"ok"}`)
		default:
			_, _ = fmt.Fprint(w, repairBody("server"))
		}
	}))
	defer api.Close()
	owner := &serverResource{client: newTestClient(t, api.URL)}
	prior, planned, configured := repairState(t, "server", repairKey), repairState(t, "server", repairKey), repairState(t, "server", repairKey)
	if d := planned.SetAttribute(context.Background(), path.Root("public_key"), types.StringUnknown()); d.HasError() {
		t.Fatal(d)
	}
	if d := configured.SetAttribute(context.Background(), path.Root("public_key"), types.StringNull()); d.HasError() {
		t.Fatal(d)
	}
	response := resource.UpdateResponse{State: prior}
	owner.Update(context.Background(), resource.UpdateRequest{State: prior, Plan: tfsdk.Plan(planned), Config: tfsdk.Config(configured)}, &response)
	if response.Diagnostics.HasError() || updates != 1 {
		t.Fatal("legitimate omitted computed key rejected", response.Diagnostics)
	}
}
