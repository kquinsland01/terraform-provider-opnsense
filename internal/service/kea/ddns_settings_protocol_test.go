// Copyright (c) Matthew Mellor
// SPDX-License-Identifier: MPL-2.0

package kea_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

const ddnsType = "opnsense_kea_ddns_settings"

type ddnsAPI struct {
	mu                                                                                         sync.Mutex
	fields                                                                                     map[string]string
	lastWire                                                                                   map[string]string
	saves, activations, reads                                                                  int
	failActivation, badRead, unknown, mismatch, failReadAfterSave, cancelAfterSave, validation bool
	version, name, productID, raw                                                              string
	status                                                                                     int
	cancel                                                                                     context.CancelFunc
	coreCancel                                                                                 bool
	pendingApplyPrivate                                                                        int
}

func (a *ddnsAPI) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.fields == nil {
		a.fields = map[string]string{"enabled": "0", "manual_config": "1", "server_ip": "any", "server_port": "53002", "unowned": "preserved"}
	}
	switch r.URL.Path {
	case "/api/core/firmware/status":
		version, name, id := a.version, a.name, a.productID
		if version == "" {
			version = "26.7.7_2"
		}
		if name == "" {
			name = "OPNsense"
		}
		if id == "" {
			id = "opnsense"
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"product": map[string]string{"product_name": name, "product_id": id, "product_version": version}})
	case "/api/kea/ddns/get":
		a.reads++
		if a.status != 0 {
			http.Error(w, "unavailable", a.status)
			return
		}
		if a.raw != "" {
			_, _ = fmt.Fprint(w, a.raw)
			return
		}
		if a.badRead {
			_, _ = fmt.Fprint(w, `{"ddns":{"general":{}}}`)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"ddns": map[string]any{"general": a.fields}})
	case "/api/kea/ddns/set":
		a.saves++
		var body map[string]map[string]map[string]string
		if json.NewDecoder(r.Body).Decode(&body) != nil || len(body) != 1 || len(body["ddns"]) != 1 || body["ddns"]["general"] == nil {
			http.Error(w, "wrong shape", 400)
			return
		}
		if a.validation {
			_, _ = fmt.Fprint(w, `{"result":"failed","validations":{"ddns.general.server_ip":"invalid"}}`)
			return
		}
		a.lastWire = body["ddns"]["general"]
		for k, v := range a.lastWire {
			a.fields[k] = v
		}
		if a.mismatch {
			a.fields["server_port"] = "53003"
		}
		if a.failReadAfterSave {
			a.badRead = true
		}
		if a.cancel != nil {
			a.cancel()
		}
		if a.cancelAfterSave {
			conn, _, err := w.(http.Hijacker).Hijack()
			if err == nil {
				_ = conn.Close()
			}
			return
		}
		if a.unknown {
			_, _ = fmt.Fprint(w, `{}`)
			return
		}
		_, _ = fmt.Fprint(w, `{"result":"saved"}`)
	case "/api/kea/service/reconfigure":
		a.activations++
		if a.failActivation {
			_, _ = fmt.Fprint(w, `{"status":"failed"}`)
			return
		}
		_, _ = fmt.Fprint(w, `{"status":"ok"}`)
	default:
		_, _ = fmt.Fprint(w, `{}`)
	}
}

func ddnsApply(t *testing.T, s tfprotov6.ProviderServer, prior, plan, config *tfprotov6.DynamicValue, private []byte) *tfprotov6.ApplyResourceChangeResponse {
	t.Helper()
	result, err := s.ApplyResourceChange(context.Background(), &tfprotov6.ApplyResourceChangeRequest{TypeName: ddnsType, PriorState: prior, PlannedState: plan, Config: config, PlannedPrivate: private})
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func ddnsRead(t *testing.T, s tfprotov6.ProviderServer, state *tfprotov6.DynamicValue, private []byte) *tfprotov6.ReadResourceResponse {
	t.Helper()
	result, err := s.ReadResource(context.Background(), &tfprotov6.ReadResourceRequest{TypeName: ddnsType, CurrentState: state, Private: private})
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func TestDDNSProtocolErrorPersistenceRecoveryAndPendingStateOnlyDestroy(t *testing.T) {
	for _, failure := range []string{"activation", "read", "unknown", "mismatch", "connection lost"} {
		for _, create := range []bool{true, false} {
			t.Run(fmt.Sprintf("%s/create=%t", failure, create), func(t *testing.T) {
				api := &ddnsAPI{}
				httpServer := httptest.NewServer(api)
				defer httpServer.Close()
				server, schemas := optionServer(t, httpServer.URL)
				schema := schemas.ResourceSchemas[ddnsType]
				prior := optionNull(t, schema)
				config := optionValue(t, schema, map[string]any{"enabled": false, "manual_config": false, "server_port": 53004})
				if !create {
					initial := ddnsApply(t, server, prior, config, config, nil)
					if optionErrors(initial.Diagnostics) {
						t.Fatal(initial.Diagnostics)
					}
					prior = initial.NewState
				}
				api.mu.Lock()
				api.failActivation = failure == "activation"
				api.failReadAfterSave = failure == "read"
				api.unknown = failure == "unknown" || failure == "mismatch"
				api.mismatch = failure == "mismatch"
				api.cancelAfterSave = failure == "connection lost"
				api.mu.Unlock()
				failed := ddnsApply(t, server, prior, config, config, nil)
				if !optionErrors(failed.Diagnostics) || len(failed.Private) == 0 {
					t.Fatal("missing durable failed-save state", failed.Diagnostics)
				}
				var id string
				if err := optionAttrs(t, schema, failed.NewState)["id"].As(&id); err != nil || id != "ddns" {
					t.Fatal("fixed identity lost", err)
				}
				api.mu.Lock()
				saves, activations, reads := api.saves, api.activations, api.reads
				api.mu.Unlock()
				fresh, _ := optionServer(t, httpServer.URL)
				// Apply destroy directly with genuine error-returned bytes, with endpoint
				// unavailable, proves destroy does not perform implicit recovery.
				destroyed := ddnsApply(t, fresh, failed.NewState, optionNull(t, schema), optionNull(t, schema), failed.Private)
				api.mu.Lock()
				if optionErrors(destroyed.Diagnostics) || api.saves != saves || api.activations != activations || api.reads != reads {
					t.Fatal("pending destroy touched appliance", destroyed.Diagnostics)
				}
				api.failActivation = false
				api.badRead = false
				api.failReadAfterSave = false
				api.mu.Unlock()
				refreshed := ddnsRead(t, fresh, failed.NewState, failed.Private)
				api.mu.Lock()
				if optionErrors(refreshed.Diagnostics) || api.saves != saves {
					t.Fatal("fresh recovery failed or repeated set", refreshed.Diagnostics)
				}
				want := activations + 1
				if failure == "mismatch" {
					want = activations
				}
				if api.activations != want {
					t.Fatalf("activation=%d want=%d", api.activations, want)
				}
				if len(api.lastWire) != 3 || api.lastWire["manual_config"] != "0" || api.lastWire["enabled"] != "0" {
					t.Fatal("wrong targeted wire shape", api.lastWire)
				}
				api.mu.Unlock()
				lookup, err := fresh.ReadDataSource(context.Background(), &tfprotov6.ReadDataSourceRequest{TypeName: ddnsType, Config: optionValue(t, schemas.DataSourceSchemas[ddnsType], nil)})
				if err != nil || optionErrors(lookup.Diagnostics) {
					t.Fatal(err, lookup.Diagnostics)
				}
				actual := optionAttrs(t, schemas.DataSourceSchemas[ddnsType], lookup.State)
				expected := optionAttrs(t, schema, refreshed.NewState)
				for k, v := range expected {
					if !actual[k].Equal(v) {
						t.Fatal("lookup mismatch", k)
					}
				}
				api.mu.Lock()
				if api.activations != want || api.fields["unowned"] != "preserved" {
					t.Fatal("lookup activated or unowned field changed")
				}
				api.mu.Unlock()
			})
		}
	}
}

func TestDDNSProtocolCapabilityMalformedTruthAndValidation(t *testing.T) {
	api := &ddnsAPI{}
	h := httptest.NewServer(api)
	defer h.Close()
	s, schemas := optionServer(t, h.URL)
	schema := schemas.ResourceSchemas[ddnsType]
	config := optionValue(t, schema, map[string]any{"enabled": false})
	for _, version := range []string{"26.7.6", "unknown", "26.7.7-rc1", "26.7.7-devel", "184467440737095516160.7.7", "26.7.7_184467440737095516160"} {
		api.version = version
		result := ddnsApply(t, s, optionNull(t, schema), config, config, nil)
		if !optionErrors(result.Diagnostics) || api.saves != 0 {
			t.Fatal("unsupported firmware mutated", version)
		}
	}
	api.version = "26.7.7"
	api.productID = "other"
	result := ddnsApply(t, s, optionNull(t, schema), config, config, nil)
	if !optionErrors(result.Diagnostics) || api.saves != 0 {
		t.Fatal("contradictory identity accepted")
	}
	api.productID = "opnsense"
	created := ddnsApply(t, s, optionNull(t, schema), config, config, nil)
	if optionErrors(created.Diagnostics) {
		t.Fatal(created.Diagnostics)
	}
	for _, raw := range []string{`[]`, `{}`, `null`, `{"ddns":[]}`, `{"ddns":{"general":null}}`, `{"ddns":{"general":{}}}`} {
		api.raw = raw
		result := ddnsRead(t, s, created.NewState, nil)
		if !optionErrors(result.Diagnostics) {
			t.Fatal("malformed singleton accepted", raw)
		}
		for key, v := range optionAttrs(t, schema, created.NewState) {
			if !optionAttrs(t, schema, result.NewState)[key].Equal(v) {
				t.Fatal("state lost", key)
			}
		}
	}
	api.raw = ""
	api.validation = true
	failed := ddnsApply(t, s, optionNull(t, schema), config, config, nil)
	if !optionErrors(failed.Diagnostics) {
		t.Fatal("validation rejection accepted")
	}
	value, err := failed.NewState.Unmarshal(schema.ValueType())
	if err != nil || !value.IsNull() {
		t.Fatal("validation fabricated owner", err)
	}
	for _, status := range []int{401, 403, 404, 500} {
		api.status = status
		result := ddnsRead(t, s, created.NewState, nil)
		if !optionErrors(result.Diagnostics) {
			t.Fatal("HTTP failure removed owner", status)
		}
	}
}

func TestDDNSProtocolPostCommitCancellationRetainsActualReturnedIntent(t *testing.T) {
	api := &ddnsAPI{}
	h := httptest.NewServer(api)
	defer h.Close()
	s, schemas := optionServer(t, h.URL)
	schema := schemas.ResourceSchemas[ddnsType]
	config := optionValue(t, schema, map[string]any{"enabled": false, "server_port": 53004})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	api.mu.Lock()
	api.cancel = cancel
	api.mu.Unlock()
	failed, err := s.ApplyResourceChange(ctx, &tfprotov6.ApplyResourceChangeRequest{TypeName: ddnsType, PriorState: optionNull(t, schema), PlannedState: config, Config: config})
	if err != nil || !optionErrors(failed.Diagnostics) || len(failed.Private) == 0 {
		t.Fatal("canceled committed write lost private", err, failed.Diagnostics)
	}
	var id string
	if err := optionAttrs(t, schema, failed.NewState)["id"].As(&id); err != nil || id != "ddns" {
		t.Fatal("canceled committed write lost identity", err)
	}
	api.mu.Lock()
	api.cancel = nil
	api.mu.Unlock()
	fresh, _ := optionServer(t, h.URL)
	recovered := ddnsRead(t, fresh, failed.NewState, failed.Private)
	api.mu.Lock()
	defer api.mu.Unlock()
	if optionErrors(recovered.Diagnostics) || api.saves != 1 || api.activations != 1 {
		t.Fatal("post-cancellation fresh recovery repeated set or failed", recovered.Diagnostics)
	}
}

func TestDDNSProtocolConfirmedContradictionReconcilesWithoutActivation(t *testing.T) {
	api := &ddnsAPI{mismatch: true}
	h := httptest.NewServer(api)
	defer h.Close()
	s, schemas := optionServer(t, h.URL)
	schema := schemas.ResourceSchemas[ddnsType]
	config := optionValue(t, schema, map[string]any{"enabled": false, "server_port": 53004})
	failed := ddnsApply(t, s, optionNull(t, schema), config, config, nil)
	if !optionErrors(failed.Diagnostics) {
		t.Fatal("confirmed contradictory truth claimed success")
	}
	if !optionAttrs(t, schema, failed.NewState)["server_port"].Equal(tftypes.NewValue(tftypes.Number, int64(53003))) {
		t.Fatal("contradictory authoritative state lost")
	}
	api.mu.Lock()
	defer api.mu.Unlock()
	if api.saves != 1 || api.activations != 0 {
		t.Fatal("contradictory truth activated or retried")
	}
	var private map[string][]byte
	if len(failed.Private) > 0 && json.Unmarshal(failed.Private, &private) != nil {
		t.Fatal("malformed returned private")
	}
	if len(private["kea_ddns_recovery_v1"]) != 0 {
		t.Fatal("authoritative contradiction kept pending intent")
	}
}

func TestDDNSProtocolCorruptPrivateFailsClosedBeforeAnyClientAction(t *testing.T) {
	api := &ddnsAPI{}
	h := httptest.NewServer(api)
	defer h.Close()
	s, schemas := optionServer(t, h.URL)
	schema := schemas.ResourceSchemas[ddnsType]
	config := optionValue(t, schema, map[string]any{"enabled": false})
	created := ddnsApply(t, s, optionNull(t, schema), config, config, nil)
	if optionErrors(created.Diagnostics) {
		t.Fatal(created.Diagnostics)
	}
	api.mu.Lock()
	saves, reads, activations := api.saves, api.reads, api.activations
	api.mu.Unlock()
	for _, raw := range []string{
		`{"id":"other","kind":"saved","payload":{"enabled":"0"}}`,
		`{"id":"ddns","kind":"saved","payload":{"enabled":"0","enabled":"1"}}`,
		`{"id":"ddns","Kind":"saved","payload":{"enabled":"0"}}`,
		`{"id":"ddns","kind":"deleted","payload":{"enabled":"0"}}`,
		`{"id":"ddns","kind":"saved","payload":{"enabled":"0","server_port":"0"}}`,
	} {
		private, err := json.Marshal(map[string][]byte{"kea_ddns_recovery_v1": []byte(raw)})
		if err != nil {
			t.Fatal(err)
		}
		read := ddnsRead(t, s, created.NewState, private)
		update := ddnsApply(t, s, created.NewState, config, config, private)
		if !optionErrors(read.Diagnostics) || !optionErrors(update.Diagnostics) {
			t.Fatal("corrupt private accepted")
		}
		destroyed := ddnsApply(t, s, created.NewState, optionNull(t, schema), optionNull(t, schema), private)
		if optionErrors(destroyed.Diagnostics) {
			t.Fatal("state-only delete inspected corrupt recovery bytes")
		}
	}
	api.mu.Lock()
	defer api.mu.Unlock()
	if api.saves != saves || api.reads != reads || api.activations != activations {
		t.Fatal("corrupt private performed client action")
	}
}

func TestDDNSProtocolPendingUpdateRecoversOriginalIntentBeforeChangedDesired(t *testing.T) {
	for _, uncertain := range []bool{false, true} {
		t.Run(fmt.Sprint(uncertain), func(t *testing.T) {
			api := &ddnsAPI{}
			h := httptest.NewServer(api)
			defer h.Close()
			s, schemas := optionServer(t, h.URL)
			schema := schemas.ResourceSchemas[ddnsType]
			initial := optionValue(t, schema, map[string]any{"enabled": false})
			created := ddnsApply(t, s, optionNull(t, schema), initial, initial, nil)
			if optionErrors(created.Diagnostics) {
				t.Fatal(created.Diagnostics)
			}
			api.mu.Lock()
			api.failActivation = !uncertain
			api.unknown = uncertain
			api.mu.Unlock()
			requested := optionValue(t, schema, map[string]any{"enabled": false, "server_port": 53004})
			failed := ddnsApply(t, s, created.NewState, requested, requested, nil)
			if !optionErrors(failed.Diagnostics) || len(failed.Private) == 0 {
				t.Fatal("missing genuine failed-update bytes")
			}
			api.mu.Lock()
			saves, activations := api.saves, api.activations
			api.failActivation = false
			api.unknown = false
			api.fields["server_ip"] = "192.0.2.55"
			api.mu.Unlock()
			fresh, _ := optionServer(t, h.URL)
			changed := optionValue(t, schema, map[string]any{"enabled": false, "server_port": 53005, "server_ip": "192.0.2.99"})
			recovered := ddnsApply(t, fresh, failed.NewState, changed, changed, failed.Private)
			found := false
			for _, d := range recovered.Diagnostics {
				if d.Summary == "DDNS recovered; fresh plan required" {
					found = true
				}
			}
			if !found || !optionErrors(recovered.Diagnostics) {
				t.Fatal("pending update did not demand fresh plan", recovered.Diagnostics)
			}
			attrs := optionAttrs(t, schema, recovered.NewState)
			for key, want := range map[string]tftypes.Value{"id": tftypes.NewValue(tftypes.String, "ddns"), "enabled": tftypes.NewValue(tftypes.Bool, false), "manual_config": tftypes.NewValue(tftypes.Bool, true)} {
				if !attrs[key].Equal(want) {
					t.Fatal("pending update lost authoritative field", key)
				}
			}
			if !attrs["server_port"].Equal(tftypes.NewValue(tftypes.Number, int64(53004))) || !attrs["server_ip"].Equal(tftypes.NewValue(tftypes.String, "192.0.2.55")) {
				t.Fatal("original requested subset was replaced by new desired fields")
			}
			api.mu.Lock()
			defer api.mu.Unlock()
			if api.saves != saves || api.activations != activations+1 {
				t.Fatal("pending update repeated set or failed requested-subset activation")
			}
		})
	}
}

func TestDDNSProtocolAmbiguousTruthRetainsPendingStateWithoutActivation(t *testing.T) {
	api := &ddnsAPI{failActivation: true}
	h := httptest.NewServer(api)
	defer h.Close()
	s, schemas := optionServer(t, h.URL)
	schema := schemas.ResourceSchemas[ddnsType]
	config := optionValue(t, schema, map[string]any{"enabled": false, "server_port": 53004})
	failed := ddnsApply(t, s, optionNull(t, schema), config, config, nil)
	if !optionErrors(failed.Diagnostics) || len(failed.Private) == 0 {
		t.Fatal("no genuine pending state")
	}
	good := `{"ddns":{"general":{"enabled":"0","manual_config":"1","server_ip":"any","server_port":"53004"}}}`
	for _, raw := range []string{
		strings.Replace(good, `"ddns":`, `"DDNS":`, 1),
		strings.Replace(good, `"general":`, `"General":`, 1),
		strings.Replace(good, `"server_port":"53004"`, `"server_port":"53003","server_port":"53004"`, 1),
		strings.Replace(good, `"enabled":"0"`, `"enabled":"0","Enabled":"0"`, 1),
		strings.Replace(good, `"ddns":`, `"ddns":{},"ddns":`, 1),
	} {
		api.mu.Lock()
		api.raw = raw
		activations := api.activations
		api.mu.Unlock()
		fresh, _ := optionServer(t, h.URL)
		read := ddnsRead(t, fresh, failed.NewState, failed.Private)
		if !optionErrors(read.Diagnostics) || !bytes.Equal(read.Private, failed.Private) {
			t.Fatal("ambiguous truth lost pending private")
		}
		for k, v := range optionAttrs(t, schema, failed.NewState) {
			if !optionAttrs(t, schema, read.NewState)[k].Equal(v) {
				t.Fatal("ambiguous truth replaced known state", k)
			}
		}
		api.mu.Lock()
		got := api.activations
		api.mu.Unlock()
		if got != activations {
			t.Fatal("ambiguous truth activated")
		}
	}
}

func TestDDNSProtocolServerIPRejectedAtConfigurationValidation(t *testing.T) {
	api := &ddnsAPI{}
	h := httptest.NewServer(api)
	defer h.Close()
	s, schemas := optionServer(t, h.URL)
	schema := schemas.ResourceSchemas[ddnsType]
	for _, ip := range []string{"", "::1", "192.0.2.1/32", "ANY", "192.0.2.01", "192.0.2.1,192.0.2.2"} {
		resp, err := s.ValidateResourceConfig(context.Background(), &tfprotov6.ValidateResourceConfigRequest{TypeName: ddnsType, Config: optionValue(t, schema, map[string]any{"enabled": false, "server_ip": ip})})
		if err != nil || !optionErrors(resp.Diagnostics) {
			t.Fatal("invalid server_ip passed configuration validation", ip, err)
		}
	}
	api.mu.Lock()
	defer api.mu.Unlock()
	if api.saves != 0 || api.reads != 0 || api.activations != 0 {
		t.Fatal("configuration validation reached DDNS API")
	}
}
