// Copyright (c) Matthew Mellor
// SPDX-License-Identifier: MPL-2.0

package kea_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/matthew-on-git/terraform-provider-opnsense/internal/provider"
)

const (
	optionType = "opnsense_kea_dhcpv4_option"
	optionUUID = "00000000-0000-0000-0000-000000000031"
)

type optionAPI struct {
	readOverride                                                                                         map[string]any
	templateOverride                                                                                     map[string]any
	mu                                                                                                   sync.Mutex
	payload                                                                                              map[string]string
	adds, saves, deletes, activations                                                                    int
	failActivation, malformedRead, ambiguousAdd, ambiguousSave, ambiguousDelete, inUse, modelUnavailable bool
	version                                                                                              string
	loseAdd, multipleAdd, failReadAfterMutation, transportAdd, mismatchSave, preserveDelete              bool
	objectStatus                                                                                         int
	missingRaw                                                                                           string
	malformedAfterDelete                                                                                 bool
	lastWire                                                                                             map[string]string
}

func optionWire(p map[string]string) map[string]any {
	if p == nil {
		p = map[string]string{"encoding": "hex", "force": "0"}
	}
	result := map[string]any{}
	for _, key := range []string{"data", "force", "match_data", "description"} {
		result[key] = p[key]
	}
	encodings := []string{"hex", "ipv4-address", "ipv6-address", "uint8", "uint16", "uint32", "int32", "boolean", "string", "fqdn"}
	for _, key := range []string{"code", "encoding", "match_code", "match_encoding"} {
		choices := encodings
		if key == "code" || key == "match_code" {
			choices = []string{"1", "3", "15", "66", "67", "224"}
		}
		if key == "match_code" || key == "match_encoding" {
			choices = append(append([]string{}, choices...), "")
		}
		options := map[string]any{}
		for _, choice := range choices {
			flag := 0
			if choice == p[key] {
				flag = 1
			}
			options[choice] = map[string]any{"selected": flag, "value": "label differs", "optgroup": "Assigned"}
		}
		result[key] = options
	}
	return result
}

func (g *optionAPI) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	g.mu.Lock()
	defer g.mu.Unlock()
	switch r.URL.Path {
	case "/api/core/firmware/status":
		version := g.version
		if version == "" {
			version = "26.7.7"
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"product": map[string]string{"product_name": "OPNsense", "product_version": version}})
	case "/api/kea/dhcpv4/get_option":
		if g.modelUnavailable {
			_, _ = fmt.Fprint(w, `{}`)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"option": func() map[string]any {
			if g.templateOverride != nil {
				return g.templateOverride
			}
			return optionWire(nil)
		}()})
	case "/api/kea/dhcpv4/search_option":
		rows := []map[string]string{}
		if g.payload != nil {
			rows = append(rows, map[string]string{"uuid": optionUUID, "description": g.payload["description"]})
			if g.multipleAdd {
				rows = append(rows, map[string]string{"uuid": "00000000-0000-0000-0000-000000000032", "description": g.payload["description"]})
			}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"rows": rows, "rowCount": len(rows), "total": len(rows), "current": 1})
	case "/api/kea/dhcpv4/get_option/" + optionUUID, "/api/kea/dhcpv4/get_option/00000000-0000-0000-0000-000000000032":
		if g.objectStatus != 0 {
			http.Error(w, "fixture unavailable", g.objectStatus)
			return
		}
		if g.malformedRead {
			_, _ = fmt.Fprint(w, `{"option":{"code":null}}`)
			return
		}
		if g.payload == nil {
			if g.missingRaw != "" {
				_, _ = fmt.Fprint(w, g.missingRaw)
			} else {
				_, _ = fmt.Fprint(w, `[]`)
			}
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"option": func() map[string]any {
			if g.readOverride != nil {
				return g.readOverride
			}
			return optionWire(g.payload)
		}()})
	case "/api/kea/dhcpv4/add_option", "/api/kea/dhcpv4/set_option/" + optionUUID:
		var body struct {
			Option map[string]string `json:"option"`
		}
		if json.NewDecoder(r.Body).Decode(&body) != nil {
			http.Error(w, "invalid request", 400)
			return
		}
		g.lastWire = body.Option
		if g.payload == nil {
			g.payload = map[string]string{}
		}
		for k, v := range body.Option {
			g.payload[k] = v
		}
		if g.failReadAfterMutation {
			g.malformedRead = true
		}
		if strings.HasSuffix(r.URL.Path, "add_option") {
			g.adds++
			if g.loseAdd {
				g.payload = nil
				_, _ = fmt.Fprint(w, `{}`)
				return
			}
			if g.transportAdd {
				http.Error(w, "lost committed add", 500)
				return
			}
			if g.ambiguousAdd {
				_, _ = fmt.Fprint(w, `{}`)
				return
			}
			_, _ = fmt.Fprint(w, `{"result":"saved","uuid":"`+optionUUID+`"}`)
		} else {
			g.saves++
			if g.mismatchSave {
				g.payload["description"] = "authoritative other description"
				_, _ = fmt.Fprint(w, `{}`)
				return
			}
			if g.ambiguousSave {
				_, _ = fmt.Fprint(w, `{}`)
				return
			}
			_, _ = fmt.Fprint(w, `{"result":"saved"}`)
		}
	case "/api/kea/dhcpv4/del_option/" + optionUUID:
		g.deletes++
		if g.malformedAfterDelete {
			g.malformedRead = true
		}
		if g.inUse {
			http.Error(w, "option in use", 500)
			return
		}
		if !g.preserveDelete {
			g.payload = nil
		}
		if g.ambiguousDelete {
			_, _ = fmt.Fprint(w, `{}`)
			return
		}
		_, _ = fmt.Fprint(w, `{"result":"deleted"}`)
	case "/api/kea/service/reconfigure":
		g.activations++
		if g.failActivation {
			_, _ = fmt.Fprint(w, `{"status":"failed"}`)
		} else {
			_, _ = fmt.Fprint(w, `{"status":"ok"}`)
		}
	default:
		_, _ = fmt.Fprint(w, `{}`)
	}
}

func optionValues() map[string]any {
	return map[string]any{"id": optionUUID, "code": "15", "encoding": "fqdn", "data": "example.org", "force": true, "match_code": "66", "match_encoding": "string", "match_data": "literal,matching,data", "description": "synthetic_option"}
}

func optionServer(t *testing.T, uri string) (tfprotov6.ProviderServer, *tfprotov6.GetProviderSchemaResponse) {
	t.Helper()
	server := providerserver.NewProtocol6(provider.New("test")())()
	schemas, err := server.GetProviderSchema(context.Background(), &tfprotov6.GetProviderSchemaRequest{})
	if err != nil {
		t.Fatal(err)
	}
	config := optionValue(t, schemas.Provider, map[string]any{"uri": uri, "api_key": "fixture-key", "api_secret": "fixture-secret", "insecure": true})
	result, err := server.ConfigureProvider(context.Background(), &tfprotov6.ConfigureProviderRequest{Config: config, TerraformVersion: "1.14.0"})
	if err != nil || optionErrors(result.Diagnostics) {
		t.Fatalf("configure: %v %+v", err, result.Diagnostics[0])
	}
	if schemas.ResourceSchemas[optionType] == nil || schemas.DataSourceSchemas[optionType] == nil {
		t.Fatal("missing exported registration")
	}
	return server, schemas
}

func optionValue(t *testing.T, schema *tfprotov6.Schema, fields map[string]any) *tfprotov6.DynamicValue {
	t.Helper()
	values := map[string]any{}
	for _, attr := range schema.Block.Attributes {
		values[attr.Name] = nil
	}
	for key, value := range fields {
		values[key] = value
	}
	raw, err := json.Marshal(values)
	if err != nil {
		t.Fatal(err)
	}
	return &tfprotov6.DynamicValue{JSON: raw}
}

func optionNull(t *testing.T, schema *tfprotov6.Schema) *tfprotov6.DynamicValue {
	t.Helper()
	value, err := tfprotov6.NewDynamicValue(schema.ValueType(), tftypes.NewValue(schema.ValueType(), nil))
	if err != nil {
		t.Fatal(err)
	}
	return &value
}

func optionErrors(diagnostics []*tfprotov6.Diagnostic) bool {
	for _, d := range diagnostics {
		if d.Severity == tfprotov6.DiagnosticSeverityError {
			return true
		}
	}
	return false
}

func optionAttrs(t *testing.T, schema *tfprotov6.Schema, state *tfprotov6.DynamicValue) map[string]tftypes.Value {
	t.Helper()
	value, err := state.Unmarshal(schema.ValueType())
	if err != nil {
		t.Fatal(err)
	}
	var attrs map[string]tftypes.Value
	if err = value.As(&attrs); err != nil {
		t.Fatal(err)
	}
	return attrs
}

func optionApply(t *testing.T, server tfprotov6.ProviderServer, _ *tfprotov6.Schema, prior, planned *tfprotov6.DynamicValue, private []byte) *tfprotov6.ApplyResourceChangeResponse {
	t.Helper()
	response, err := server.ApplyResourceChange(context.Background(), &tfprotov6.ApplyResourceChangeRequest{TypeName: optionType, PriorState: prior, PlannedState: planned, Config: planned, PlannedPrivate: private})
	if err != nil {
		t.Fatal(err)
	}
	return response
}

func TestDHCPv4OptionProtocolCreateRestartRecovery(t *testing.T) {
	for _, ambiguous := range []bool{false, true} {
		t.Run(fmt.Sprintf("ambiguous=%v", ambiguous), func(t *testing.T) {
			api := &optionAPI{failActivation: true, ambiguousAdd: ambiguous}
			httpServer := httptest.NewServer(api)
			defer httpServer.Close()
			server, schemas := optionServer(t, httpServer.URL)
			schema := schemas.ResourceSchemas[optionType]
			values := optionValues()
			values["id"] = nil
			plan := optionValue(t, schema, values)
			failed := optionApply(t, server, schema, optionNull(t, schema), plan, nil)
			if !optionErrors(failed.Diagnostics) || len(failed.Private) == 0 {
				t.Fatal("missing failed activation state", failed.Diagnostics)
			}
			var id string
			if err := optionAttrs(t, schema, failed.NewState)["id"].As(&id); err != nil || id != optionUUID {
				t.Fatal("lost UUID", err)
			}
			// Persist exactly protocol-returned dynamic state/private, then restart server.
			stored, err := json.Marshal(failed)
			if err != nil {
				t.Fatal(err)
			}
			var persisted tfprotov6.ApplyResourceChangeResponse
			if err = json.Unmarshal(stored, &persisted); err != nil {
				t.Fatal(err)
			}
			api.failActivation = false
			fresh, _ := optionServer(t, httpServer.URL)
			recovered, err := fresh.ReadResource(context.Background(), &tfprotov6.ReadResourceRequest{TypeName: optionType, CurrentState: persisted.NewState, Private: persisted.Private})
			if err != nil || optionErrors(recovered.Diagnostics) {
				t.Fatal(err, recovered.Diagnostics)
			}
			if api.adds != 1 || api.activations != 2 {
				t.Fatalf("add=%d activations=%d", api.adds, api.activations)
			}
			lookup, err := fresh.ReadDataSource(context.Background(), &tfprotov6.ReadDataSourceRequest{TypeName: optionType, Config: optionValue(t, schemas.DataSourceSchemas[optionType], map[string]any{"id": optionUUID})})
			if err != nil || optionErrors(lookup.Diagnostics) {
				t.Fatal(err, lookup.Diagnostics)
			}
			got := optionAttrs(t, schemas.DataSourceSchemas[optionType], lookup.State)
			want := optionAttrs(t, schema, recovered.NewState)
			for field, expected := range want {
				if !got[field].Equal(expected) {
					t.Fatalf("lookup computed %s differs from authoritative populated object", field)
				}
			}
			if api.adds != 1 || api.activations != 2 {
				t.Fatal("lookup mutated")
			}
		})
	}
}

func TestDHCPv4OptionProtocolStrictEveryFieldAndAbsence(t *testing.T) {
	api := &optionAPI{}
	httpServer := httptest.NewServer(api)
	defer httpServer.Close()
	server, schemas := optionServer(t, httpServer.URL)
	schema := schemas.ResourceSchemas[optionType]
	created := optionApply(t, server, schema, optionNull(t, schema), optionValue(t, schema, optionValues()), nil)
	if optionErrors(created.Diagnostics) {
		t.Fatal(created.Diagnostics)
	}
	for _, field := range []string{"code", "encoding", "data", "force", "match_code", "match_encoding", "match_data", "description"} {
		for _, mode := range []string{"missing", "null", "wrong-type"} {
			t.Run(field+"/"+mode, func(t *testing.T) {
				override := optionWire(api.payload)
				switch mode {
				case "missing":
					delete(override, field)
				case "null":
					override[field] = nil
				case "wrong-type":
					override[field] = 123
				}
				api.readOverride = override
				response, err := server.ReadResource(context.Background(), &tfprotov6.ReadResourceRequest{TypeName: optionType, CurrentState: created.NewState})
				if err != nil || !optionErrors(response.Diagnostics) {
					t.Fatal("invalid authoritative field accepted", err, response.Diagnostics)
				}
				{
					got := optionAttrs(t, schema, response.NewState)
					want := optionAttrs(t, schema, created.NewState)
					for key, value := range want {
						if !got[key].Equal(value) {
							t.Fatalf("prior %s lost", key)
						}
					}
				}
			})
		}
	}
	api.readOverride = nil
	for _, shape := range []string{`{}`, `null`, `{"option":[]}`, `{"option":null}`, `{"option":{}}`} {
		api.payload = nil
		api.missingRaw = shape
		response, err := server.ReadResource(context.Background(), &tfprotov6.ReadResourceRequest{TypeName: optionType, CurrentState: created.NewState})
		if err != nil || !optionErrors(response.Diagnostics) {
			t.Fatal("wrapper corruption treated as absence", shape, err)
		}
	}
	api.missingRaw = "[]"
	response, err := server.ReadResource(context.Background(), &tfprotov6.ReadResourceRequest{TypeName: optionType, CurrentState: created.NewState})
	if err != nil || optionErrors(response.Diagnostics) {
		t.Fatal(err, response.Diagnostics)
	}
	value, err := response.NewState.Unmarshal(schema.ValueType())
	if err != nil || !value.IsNull() {
		t.Fatal("actual [] did not remove missing object", err)
	}
}

func TestDHCPv4OptionProtocolUnsupportedCreateZeroWrites(t *testing.T) {
	for _, version := range []string{"26.7.6", "26.7.7-devel", "unknown", "26.7.7-rc1"} {
		t.Run(version, func(t *testing.T) {
			api := &optionAPI{version: version}
			httpServer := httptest.NewServer(api)
			defer httpServer.Close()
			server, schemas := optionServer(t, httpServer.URL)
			schema := schemas.ResourceSchemas[optionType]
			response := optionApply(t, server, schema, optionNull(t, schema), optionValue(t, schema, optionValues()), nil)
			if !optionErrors(response.Diagnostics) || api.adds+api.saves+api.deletes+api.activations != 0 {
				t.Fatal("unsupported create mutated", response.Diagnostics)
			}
		})
	}
}

func TestDHCPv4OptionProtocolAbsentUpdateNoUpsert(t *testing.T) {
	api := &optionAPI{}
	httpServer := httptest.NewServer(api)
	defer httpServer.Close()
	server, schemas := optionServer(t, httpServer.URL)
	schema := schemas.ResourceSchemas[optionType]
	values := optionValues()
	prior := optionValue(t, schema, values)
	values["data"] = "new.example.org"
	response := optionApply(t, server, schema, prior, optionValue(t, schema, values), nil)
	if !optionErrors(response.Diagnostics) || api.saves != 0 || api.activations != 0 {
		t.Fatal("absent UUID upserted", response.Diagnostics)
	}
}

func TestDHCPv4OptionProtocolCorruptUUIDPayloadRecoveryFailsBeforeMutation(t *testing.T) {
	api := &optionAPI{failActivation: true}
	httpServer := httptest.NewServer(api)
	defer httpServer.Close()
	server, schemas := optionServer(t, httpServer.URL)
	schema := schemas.ResourceSchemas[optionType]
	failed := optionApply(t, server, schema, optionNull(t, schema), optionValue(t, schema, optionValues()), nil)
	if !optionErrors(failed.Diagnostics) || len(failed.Private) == 0 {
		t.Fatal("actual marker generation failed", failed.Diagnostics)
	}
	var original map[string][]byte
	if err := json.Unmarshal(failed.Private, &original); err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"bytes", "uuid", "kind", "missing-required", "extra-field", "invalid-field", "trailing", "duplicate", "case-alias", "payload-duplicate", "whitespace"} {
		t.Run(mode, func(t *testing.T) {
			data := map[string][]byte{}
			for k, v := range original {
				data[k] = v
			}
			var marker map[string]any
			if err := json.Unmarshal(data["kea_option_recovery_v1"], &marker); err != nil {
				t.Fatal(err)
			}
			switch mode {
			case "uuid":
				marker["id"] = "00000000-0000-0000-0000-000000000032"
			case "kind":
				marker["kind"] = "future"
			case "missing-required":
				delete(marker["payload"].(map[string]any), "data")
			case "extra-field":
				marker["unexpected"] = "corrupt"
			case "invalid-field":
				marker["payload"].(map[string]any)["force"] = "maybe"
			}
			raw, err := json.Marshal(marker)
			if err != nil {
				t.Fatal(err)
			}
			if mode == "bytes" {
				raw = []byte(`broken`)
			}
			if mode == "duplicate" {
				raw = []byte(`{"id":"` + optionUUID + `","id":"` + optionUUID + `","kind":"saved","payload":{"code":"15","data":"example.org","description":"synthetic_option"}}`)
			}
			if mode == "case-alias" {
				raw = []byte(`{"ID":"` + optionUUID + `","Kind":"saved","Payload":{"code":"15","data":"example.org","description":"synthetic_option"}}`)
			}
			if mode == "payload-duplicate" {
				raw = []byte(`{"id":"` + optionUUID + `","kind":"saved","payload":{"code":"15","data":"bad","data":"example.org","description":"synthetic_option"}}`)
			}
			if mode == "whitespace" {
				raw = append([]byte(" "), raw...)
			}
			if mode == "trailing" {
				raw = append(raw, []byte(` {}`)...)
			}
			data["kea_option_recovery_v1"] = raw
			private, err := json.Marshal(data)
			if err != nil {
				t.Fatal(err)
			}
			response, err := server.ReadResource(context.Background(), &tfprotov6.ReadResourceRequest{TypeName: optionType, CurrentState: failed.NewState, Private: private})
			if err != nil || !optionErrors(response.Diagnostics) || api.activations != 1 || api.adds != 1 {
				t.Fatal("corrupt recovery mutated", err, response.Diagnostics)
			}
		})
	}
}

func TestDHCPv4OptionProtocolUnknownRequiredOptionalZeroWrites(t *testing.T) {
	for _, key := range []string{"code", "data", "description", "encoding", "force", "match_code", "match_encoding", "match_data"} {
		t.Run(key, func(t *testing.T) {
			api := &optionAPI{}
			httpServer := httptest.NewServer(api)
			defer httpServer.Close()
			server, schemas := optionServer(t, httpServer.URL)
			schema := schemas.ResourceSchemas[optionType]
			attrs := optionAttrs(t, schema, optionValue(t, schema, optionValues()))
			kind := tftypes.Type(tftypes.String)
			if key == "force" {
				kind = tftypes.Bool
			}
			attrs[key] = tftypes.NewValue(kind, tftypes.UnknownValue)
			dynamic, err := tfprotov6.NewDynamicValue(schema.ValueType(), tftypes.NewValue(schema.ValueType(), attrs))
			if err != nil {
				t.Fatal(err)
			}
			response := optionApply(t, server, schema, optionNull(t, schema), &dynamic, nil)
			if !optionErrors(response.Diagnostics) || api.adds+api.saves+api.activations != 0 {
				t.Fatal("unknown became default/write", response.Diagnostics)
			}
		})
	}
}

func TestDHCPv4OptionProtocolConcurrentDiscoverySerializedAndCanceled(t *testing.T) {
	api := &optionAPI{}
	httpServer := httptest.NewServer(api)
	defer httpServer.Close()
	server, schemas := optionServer(t, httpServer.URL)
	schema := schemas.ResourceSchemas[optionType]
	plan := optionValue(t, schema, optionValues())
	prior := optionNull(t, schema)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	canceled, err := server.ApplyResourceChange(ctx, &tfprotov6.ApplyResourceChangeRequest{TypeName: optionType, PriorState: prior, PlannedState: plan, Config: plan})
	if err == nil && !optionErrors(canceled.Diagnostics) {
		t.Fatal("canceled operation accepted")
	}
	if api.adds+api.saves+api.activations != 0 {
		t.Fatal("canceled operation wrote")
	}
	var wg sync.WaitGroup
	responses := make(chan *tfprotov6.ApplyResourceChangeResponse, 2)
	errs := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			response, err := server.ApplyResourceChange(context.Background(), &tfprotov6.ApplyResourceChangeRequest{TypeName: optionType, PriorState: prior, PlannedState: plan, Config: plan})
			responses <- response
			errs <- err
		}()
	}
	wg.Wait()
	close(responses)
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	success := 0
	for response := range responses {
		if !optionErrors(response.Diagnostics) {
			success++
		}
	}
	if success != 1 || api.adds != 1 {
		t.Fatalf("discover/add not serialized: success=%d adds=%d", success, api.adds)
	}
}

func TestDHCPv4OptionProtocolUncertainAddMatchesAndNoAutomaticRetry(t *testing.T) {
	for _, mode := range []string{"zero", "multiple", "http500"} {
		t.Run(mode, func(t *testing.T) {
			api := &optionAPI{ambiguousAdd: true, loseAdd: mode == "zero", multipleAdd: mode == "multiple", transportAdd: mode == "http500"}
			httpServer := httptest.NewServer(api)
			defer httpServer.Close()
			server, schemas := optionServer(t, httpServer.URL)
			schema := schemas.ResourceSchemas[optionType]
			response := optionApply(t, server, schema, optionNull(t, schema), optionValue(t, schema, optionValues()), nil)
			if mode == "http500" {
				if optionErrors(response.Diagnostics) {
					t.Fatal(response.Diagnostics)
				}
			} else if !optionErrors(response.Diagnostics) {
				t.Fatal("ambiguous adoption did not fail closed")
			}
			if api.adds != 1 {
				t.Fatalf("automatic add retry occurred: %d", api.adds)
			}
			if mode == "multiple" {
				retry := optionApply(t, server, schema, optionNull(t, schema), optionValue(t, schema, optionValues()), nil)
				if !optionErrors(retry.Diagnostics) || api.adds != 1 {
					t.Fatal("blind duplicate add")
				}
			}
		})
	}
}

func TestDHCPv4OptionProtocolUnknownSaveRestartBindsAttempt(t *testing.T) {
	for _, mode := range []string{"match", "mismatch", "malformed", "missing"} {
		t.Run(mode, func(t *testing.T) {
			api := &optionAPI{}
			httpServer := httptest.NewServer(api)
			defer httpServer.Close()
			server, schemas := optionServer(t, httpServer.URL)
			schema := schemas.ResourceSchemas[optionType]
			values := optionValues()
			created := optionApply(t, server, schema, optionNull(t, schema), optionValue(t, schema, values), nil)
			if optionErrors(created.Diagnostics) {
				t.Fatal(created.Diagnostics)
			}
			api.ambiguousSave = true
			api.failReadAfterMutation = true
			values["description"] = "attempted changed description"
			failed := optionApply(t, server, schema, created.NewState, optionValue(t, schema, values), created.Private)
			if !optionErrors(failed.Diagnostics) || len(failed.Private) == 0 {
				t.Fatal("uncertain attempt not persisted", failed.Diagnostics)
			}
			api.failReadAfterMutation = false
			api.malformedRead = mode == "malformed"
			if mode == "mismatch" {
				api.payload["description"] = "contradictory authoritative value"
			}
			if mode == "missing" {
				api.payload = nil
			}
			fresh, _ := optionServer(t, httpServer.URL)
			read, err := fresh.ReadResource(context.Background(), &tfprotov6.ReadResourceRequest{TypeName: optionType, CurrentState: failed.NewState, Private: failed.Private})
			if err != nil {
				t.Fatal(err)
			}
			if mode == "malformed" || mode == "missing" {
				if !optionErrors(read.Diagnostics) || api.activations != 1 || len(read.Private) == 0 {
					t.Fatal("malformed recovery lost uncertainty or activated", read.Diagnostics)
				}
				return
			}
			if optionErrors(read.Diagnostics) || api.saves != 1 {
				t.Fatal("recovery repeated set or failed", read.Diagnostics)
			}
			expectedActivation := 1
			if mode == "match" {
				expectedActivation = 2
			}
			if api.activations != expectedActivation {
				t.Fatal("activated nonmatching attempted payload", api.activations)
			}
			var description string
			if err := optionAttrs(t, schema, read.NewState)["description"].As(&description); err != nil || description != api.payload["description"] {
				t.Fatal("recovery did not retain authoritative values", err)
			}
		})
	}
}

func TestDHCPv4OptionProtocolDeletedMarkerReappearedUUIDRetained(t *testing.T) {
	for _, operation := range []string{"read", "delete"} {
		t.Run(operation, func(t *testing.T) {
			api := &optionAPI{}
			httpServer := httptest.NewServer(api)
			defer httpServer.Close()
			server, schemas := optionServer(t, httpServer.URL)
			schema := schemas.ResourceSchemas[optionType]
			created := optionApply(t, server, schema, optionNull(t, schema), optionValue(t, schema, optionValues()), nil)
			saved := api.payload
			api.failActivation = true
			failed := optionApply(t, server, schema, created.NewState, optionNull(t, schema), created.Private)
			if !optionErrors(failed.Diagnostics) {
				t.Fatal("delete activation must fail")
			}
			api.payload = saved
			api.failActivation = false
			fresh, _ := optionServer(t, httpServer.URL)
			var state *tfprotov6.DynamicValue
			if operation == "read" {
				response, err := fresh.ReadResource(context.Background(), &tfprotov6.ReadResourceRequest{TypeName: optionType, CurrentState: failed.NewState, Private: failed.Private})
				if err != nil || optionErrors(response.Diagnostics) {
					t.Fatal(err, response.Diagnostics)
				}
				state = response.NewState
			} else {
				response := optionApply(t, fresh, schema, failed.NewState, optionNull(t, schema), failed.Private)
				if !optionErrors(response.Diagnostics) {
					t.Fatal("reappeared delete must retain identity with retry diagnostic")
				}
				state = response.NewState
			}
			var id string
			if err := optionAttrs(t, schema, state)["id"].As(&id); err != nil || id != optionUUID || api.deletes != 1 || api.activations != 2 {
				t.Fatal("restored UUID was removed or activated", err, api.deletes, api.activations)
			}
		})
	}
}

func TestDHCPv4OptionProtocolAmbiguousDeleteUnknownRestartChecksTruthFirst(t *testing.T) {
	for _, stillPresent := range []bool{false, true} {
		t.Run(fmt.Sprintf("still_present=%v", stillPresent), func(t *testing.T) {
			api := &optionAPI{}
			httpServer := httptest.NewServer(api)
			defer httpServer.Close()
			server, schemas := optionServer(t, httpServer.URL)
			schema := schemas.ResourceSchemas[optionType]
			created := optionApply(t, server, schema, optionNull(t, schema), optionValue(t, schema, optionValues()), nil)
			if optionErrors(created.Diagnostics) {
				t.Fatal(created.Diagnostics)
			}
			api.ambiguousDelete = true
			api.preserveDelete = stillPresent
			api.malformedAfterDelete = true
			failed := optionApply(t, server, schema, created.NewState, optionNull(t, schema), created.Private)
			if !optionErrors(failed.Diagnostics) || len(failed.Private) == 0 {
				t.Fatal("unknown deletion not retained", failed.Diagnostics)
			}
			api.malformedRead = false
			api.malformedAfterDelete = false
			fresh, _ := optionServer(t, httpServer.URL)
			read, err := fresh.ReadResource(context.Background(), &tfprotov6.ReadResourceRequest{TypeName: optionType, CurrentState: failed.NewState, Private: failed.Private})
			if err != nil {
				t.Fatal(err)
			}
			if stillPresent {
				if optionErrors(read.Diagnostics) || api.activations != 1 || api.deletes != 1 {
					t.Fatal("uncertain delete activated before checking present truth", read.Diagnostics)
				}
				api.preserveDelete = false
				removed := optionApply(t, fresh, schema, read.NewState, optionNull(t, schema), read.Private)
				if optionErrors(removed.Diagnostics) || api.deletes != 2 {
					t.Fatal("verified still-present destroy failed", removed.Diagnostics)
				}
			} else {
				value, err := read.NewState.Unmarshal(schema.ValueType())
				if err != nil || !value.IsNull() || optionErrors(read.Diagnostics) || api.deletes != 1 || api.activations != 2 {
					t.Fatal("missing truth recovery failed", err, read.Diagnostics)
				}
			}
		})
	}
}

func TestDHCPv4OptionProtocolUnknownSaveComparesOnlyRequestedWireFields(t *testing.T) {
	api := &optionAPI{}
	httpServer := httptest.NewServer(api)
	defer httpServer.Close()
	server, schemas := optionServer(t, httpServer.URL)
	schema := schemas.ResourceSchemas[optionType]
	values := optionValues()
	created := optionApply(t, server, schema, optionNull(t, schema), optionValue(t, schema, values), nil)
	if optionErrors(created.Diagnostics) {
		t.Fatal(created.Diagnostics)
	}
	values["data"] = "updated.example.org"
	config := map[string]any{"id": optionUUID, "code": "15", "data": "updated.example.org", "description": "synthetic_option"}
	api.ambiguousSave = true
	api.failReadAfterMutation = true
	response, err := server.ApplyResourceChange(context.Background(), &tfprotov6.ApplyResourceChangeRequest{TypeName: optionType, PriorState: created.NewState, PlannedState: optionValue(t, schema, values), Config: optionValue(t, schema, config)})
	if err != nil || !optionErrors(response.Diagnostics) || len(response.Private) == 0 {
		t.Fatal("uncertain partial save missing", err, response.Diagnostics)
	}
	for _, key := range []string{"encoding", "force", "match_code", "match_encoding", "match_data"} {
		if _, ok := api.lastWire[key]; ok {
			t.Fatal("omitted key serialized", key)
		}
	}
	// Unrelated valid omitted-field drift must not veto an exact requested save.
	api.payload["force"] = "0"
	api.payload["match_code"] = ""
	api.payload["match_data"] = ""
	api.malformedRead = false
	api.failReadAfterMutation = false
	fresh, _ := optionServer(t, httpServer.URL)
	read, err := fresh.ReadResource(context.Background(), &tfprotov6.ReadResourceRequest{TypeName: optionType, CurrentState: response.NewState, Private: response.Private})
	if err != nil || optionErrors(read.Diagnostics) || api.activations != 2 || api.saves != 1 {
		t.Fatal("omitted unrelated fields entered save comparison", err, read.Diagnostics)
	}
}

func TestDHCPv4OptionProtocolMalformedCatalogZeroWrites(t *testing.T) {
	for _, mode := range []string{"missing field", "missing encoding", "invalid flag", "multiple selections", "unknown code"} {
		t.Run(mode, func(t *testing.T) {
			api := &optionAPI{}
			model := optionWire(nil)
			switch mode {
			case "missing field":
				delete(model, "data")
			case "missing encoding":
				delete(model["encoding"].(map[string]any), "fqdn")
			case "invalid flag":
				model["code"].(map[string]any)["15"] = map[string]any{"selected": "1"}
			case "multiple selections":
				model["code"].(map[string]any)["15"] = map[string]any{"selected": 1}
				model["code"].(map[string]any)["66"] = map[string]any{"selected": 1}
			case "unknown code":
				delete(model["code"].(map[string]any), "15")
			}
			api.templateOverride = model
			httpServer := httptest.NewServer(api)
			defer httpServer.Close()
			server, schemas := optionServer(t, httpServer.URL)
			schema := schemas.ResourceSchemas[optionType]
			response := optionApply(t, server, schema, optionNull(t, schema), optionValue(t, schema, optionValues()), nil)
			if !optionErrors(response.Diagnostics) || api.adds+api.activations != 0 {
				t.Fatal("unusable template mutated", response.Diagnostics)
			}
		})
	}
}

func TestDHCPv4OptionProtocolPlanExplicitUnknownNeverInheritsPrior(t *testing.T) {
	api := &optionAPI{}
	httpServer := httptest.NewServer(api)
	defer httpServer.Close()
	server, schemas := optionServer(t, httpServer.URL)
	schema := schemas.ResourceSchemas[optionType]
	prior := optionValue(t, schema, optionValues())
	for _, key := range []string{"encoding", "force", "match_code", "match_encoding", "match_data"} {
		t.Run(key, func(t *testing.T) {
			attrs := optionAttrs(t, schema, prior)
			kind := tftypes.Type(tftypes.String)
			if key == "force" {
				kind = tftypes.Bool
			}
			attrs[key] = tftypes.NewValue(kind, tftypes.UnknownValue)
			proposed, err := tfprotov6.NewDynamicValue(schema.ValueType(), tftypes.NewValue(schema.ValueType(), attrs))
			if err != nil {
				t.Fatal(err)
			}
			response, err := server.PlanResourceChange(context.Background(), &tfprotov6.PlanResourceChangeRequest{TypeName: optionType, PriorState: prior, ProposedNewState: &proposed, Config: &proposed})
			if err != nil || optionErrors(response.Diagnostics) {
				t.Fatal(err, response.Diagnostics)
			}
			if optionAttrs(t, schema, response.PlannedState)[key].IsKnown() {
				t.Fatal("explicit unknown replaced by prior/default", key)
			}
		})
	}
	if api.adds+api.saves+api.deletes+api.activations != 0 {
		t.Fatal("planning mutated")
	}
}

func TestDHCPv4OptionProtocolAPIFieldValidationRetainedWithoutActivation(t *testing.T) {
	for _, operation := range []string{"create", "update"} {
		t.Run(operation, func(t *testing.T) {
			api := &optionAPI{}
			reject := false
			attempts := 0
			httpServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if reject && (r.URL.Path == "/api/kea/dhcpv4/add_option" || strings.HasPrefix(r.URL.Path, "/api/kea/dhcpv4/set_option/")) {
					attempts++
					_, _ = fmt.Fprint(w, `{"result":"failed","validations":{"option.data":"invalid encoded option data"}}`)
					return
				}
				api.ServeHTTP(w, r)
			}))
			defer httpServer.Close()
			server, schemas := optionServer(t, httpServer.URL)
			schema := schemas.ResourceSchemas[optionType]
			prior := optionNull(t, schema)
			beforeActivation := 0
			if operation == "update" {
				created := optionApply(t, server, schema, prior, optionValue(t, schema, optionValues()), nil)
				if optionErrors(created.Diagnostics) {
					t.Fatal(created.Diagnostics)
				}
				prior = created.NewState
				beforeActivation = 1
			}
			reject = true
			response := optionApply(t, server, schema, prior, optionValue(t, schema, optionValues()), nil)
			if !optionErrors(response.Diagnostics) || attempts != 1 || api.activations != beforeActivation {
				t.Fatal("API rejection was retried or activated", response.Diagnostics)
			}
			found := false
			for _, diagnostic := range response.Diagnostics {
				if strings.Contains(diagnostic.Detail, "option.data") {
					found = true
				}
			}
			if !found {
				t.Fatal("API field-specific validation lost", response.Diagnostics)
			}
		})
	}
}

func TestDHCPv4OptionProtocolPreexistingAddReplyNeverOwnsUnrelatedUUID(t *testing.T) {
	const unrelated = "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"
	for _, committed := range []bool{false, true} {
		t.Run(fmt.Sprint(committed), func(t *testing.T) {
			api := &optionAPI{}
			unrelatedDeletes := 0
			httpServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/api/kea/dhcpv4/search_option":
					rows := []map[string]string{{"uuid": unrelated, "description": "unrelated"}}
					if api.payload != nil {
						rows = append(rows, map[string]string{"uuid": optionUUID, "description": api.payload["description"]})
					}
					_ = json.NewEncoder(w).Encode(map[string]any{"rows": rows, "rowCount": len(rows), "total": len(rows), "current": 1})
				case "/api/kea/dhcpv4/add_option":
					api.adds++
					if committed {
						var body struct {
							Option map[string]string `json:"option"`
						}
						if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
							t.Error(err)
						}
						api.payload = body.Option
					}
					_, _ = fmt.Fprint(w, `{"result":"saved","uuid":"`+strings.ToUpper(unrelated)+`"}`)
				case "/api/kea/dhcpv4/del_option/" + unrelated:
					unrelatedDeletes++
					http.Error(w, "unrelated must remain", 500)
				default:
					api.ServeHTTP(w, r)
				}
			}))
			defer httpServer.Close()
			server, schemas := optionServer(t, httpServer.URL)
			schema := schemas.ResourceSchemas[optionType]
			response := optionApply(t, server, schema, optionNull(t, schema), optionValue(t, schema, optionValues()), nil)
			if committed {
				if optionErrors(response.Diagnostics) {
					t.Fatal(response.Diagnostics)
				}
				var id string
				if err := optionAttrs(t, schema, response.NewState)["id"].As(&id); err != nil || id != optionUUID {
					t.Fatal("preexisting reply adopted", id, err)
				}
				deleted := optionApply(t, server, schema, response.NewState, optionNull(t, schema), response.Private)
				if optionErrors(deleted.Diagnostics) || api.deletes != 1 {
					t.Fatal(deleted.Diagnostics)
				}
			} else {
				value, err := response.NewState.Unmarshal(schema.ValueType())
				if err != nil || !value.IsNull() || !optionErrors(response.Diagnostics) {
					t.Fatal("unrelated reply manufactured tracked identity", err, response.Diagnostics)
				}
			}
			if unrelatedDeletes != 0 {
				t.Fatal("unrelated object deleted")
			}
		})
	}
}

func TestDHCPv4OptionProtocolSearchRedirectNeverReachesCredentialSink(t *testing.T) {
	requests := 0
	credentials := false
	sink := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		credentials = credentials || r.Header.Get("Authorization") != ""
		_, _ = fmt.Fprint(w, `{}`)
	}))
	defer sink.Close()
	api := &optionAPI{}
	source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/kea/dhcpv4/search_option" {
			http.Redirect(w, r, sink.URL, http.StatusTemporaryRedirect)
			return
		}
		api.ServeHTTP(w, r)
	}))
	defer source.Close()
	server, schemas := optionServer(t, source.URL)
	schema := schemas.ResourceSchemas[optionType]
	response := optionApply(t, server, schema, optionNull(t, schema), optionValue(t, schema, optionValues()), nil)
	if !optionErrors(response.Diagnostics) || requests != 0 || credentials || api.adds+api.saves+api.deletes+api.activations != 0 {
		t.Fatal("redirect leaked credentials/requests or mutated", requests, credentials, response.Diagnostics)
	}
}

func TestDHCPv4OptionProtocolConfirmedCreateBadReadRetainsIntentAndRecovers(t *testing.T) {
	api := &optionAPI{failReadAfterMutation: true}
	httpServer := httptest.NewServer(api)
	defer httpServer.Close()
	server, schemas := optionServer(t, httpServer.URL)
	schema := schemas.ResourceSchemas[optionType]
	failed := optionApply(t, server, schema, optionNull(t, schema), optionValue(t, schema, optionValues()), nil)
	if !optionErrors(failed.Diagnostics) || len(failed.Private) == 0 || api.adds != 1 || api.activations != 0 {
		t.Fatal("unverified create activated or lost private intent", failed.Diagnostics)
	}
	var id string
	if err := optionAttrs(t, schema, failed.NewState)["id"].As(&id); err != nil || id != optionUUID {
		t.Fatal("confirmed identity lost", err)
	}
	var private map[string][]byte
	var recovery struct {
		ID      string            `json:"id"`
		Kind    string            `json:"kind"`
		Payload map[string]string `json:"payload"`
	}
	if json.Unmarshal(failed.Private, &private) != nil || json.Unmarshal(private["kea_option_recovery_v1"], &recovery) != nil || recovery.ID != optionUUID || recovery.Kind != "saved" || recovery.Payload["data"] != "example.org" || len(recovery.Payload) != 8 {
		t.Fatal("confirmed create requested intent lost")
	}
	api.malformedRead = false
	api.failReadAfterMutation = false
	fresh, _ := optionServer(t, httpServer.URL)
	read, err := fresh.ReadResource(context.Background(), &tfprotov6.ReadResourceRequest{TypeName: optionType, CurrentState: failed.NewState, Private: failed.Private})
	if err != nil || optionErrors(read.Diagnostics) || api.adds != 1 || api.activations != 1 {
		t.Fatal("fresh recovery repeated add or failed", err, read.Diagnostics)
	}
	deleted := optionApply(t, fresh, schema, read.NewState, optionNull(t, schema), read.Private)
	if optionErrors(deleted.Diagnostics) || api.deletes != 1 || api.payload != nil {
		t.Fatal("ordinary destroy orphaned confirmed create", deleted.Diagnostics)
	}
}

func TestDHCPv4OptionProtocolPostCommitCancellationRecovery(t *testing.T) {
	for _, operation := range []string{"save", "delete"} {
		t.Run(operation, func(t *testing.T) {
			api := &optionAPI{}
			var cancel context.CancelFunc
			interrupt := false
			httpServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				target := "/api/kea/dhcpv4/set_option/" + optionUUID
				if operation == "delete" {
					target = "/api/kea/dhcpv4/del_option/" + optionUUID
				}
				if interrupt && r.URL.Path == target {
					api.ServeHTTP(httptest.NewRecorder(), r)
					cancel()
					<-r.Context().Done()
					return
				}
				api.ServeHTTP(w, r)
			}))
			defer httpServer.Close()
			server, schemas := optionServer(t, httpServer.URL)
			schema := schemas.ResourceSchemas[optionType]
			created := optionApply(t, server, schema, optionNull(t, schema), optionValue(t, schema, optionValues()), nil)
			if optionErrors(created.Diagnostics) {
				t.Fatal(created.Diagnostics)
			}
			values := optionValues()
			values["data"] = "committed.example.org"
			planned := optionValue(t, schema, values)
			if operation == "delete" {
				planned = optionNull(t, schema)
			}
			ctx, stop := context.WithCancel(context.Background())
			defer stop()
			cancel = stop
			interrupt = true
			failed, err := server.ApplyResourceChange(ctx, &tfprotov6.ApplyResourceChangeRequest{TypeName: optionType, PriorState: created.NewState, PlannedState: planned, Config: optionValue(t, schema, values)})
			if err != nil || !optionErrors(failed.Diagnostics) || len(failed.Private) == 0 {
				t.Fatal("postcommit cancellation lost failure/private", err, failed.Diagnostics)
			}
			var id string
			if err := optionAttrs(t, schema, failed.NewState)["id"].As(&id); err != nil || id != optionUUID {
				t.Fatal("postcommit cancellation lost identity", err)
			}
			var outer map[string][]byte
			var marker struct {
				ID      string            `json:"id"`
				Kind    string            `json:"kind"`
				Payload map[string]string `json:"payload"`
			}
			expectedKind := "save_unknown"
			if operation == "delete" {
				expectedKind = "delete_unknown"
			}
			if json.Unmarshal(failed.Private, &outer) != nil || json.Unmarshal(outer["kea_option_recovery_v1"], &marker) != nil || marker.ID != optionUUID || marker.Kind != expectedKind {
				t.Fatal("UUID-bound postcommit recovery intent lost")
			}
			if operation == "save" && marker.Payload["data"] != "committed.example.org" {
				t.Fatal("requested wire payload lost")
			}

			interrupt = false
			fresh, _ := optionServer(t, httpServer.URL)
			read, err := fresh.ReadResource(context.Background(), &tfprotov6.ReadResourceRequest{TypeName: optionType, CurrentState: failed.NewState, Private: failed.Private})
			if err != nil || optionErrors(read.Diagnostics) || api.adds != 1 || api.activations != 2 {
				t.Fatal("canceled committed operation did not recover", err, read.Diagnostics)
			}
			if operation == "save" {
				var data string
				if err := optionAttrs(t, schema, read.NewState)["data"].As(&data); err != nil || data != "committed.example.org" {
					t.Fatal("recovery lost committed value", err)
				}
			} else {
				value, err := read.NewState.Unmarshal(schema.ValueType())
				if err != nil || !value.IsNull() {
					t.Fatal("confirmed deletion remained tracked", err)
				}
			}
			if operation == "save" && api.saves != 1 {
				t.Fatal("save repeated")
			}
			if operation == "delete" && api.deletes != 1 {
				t.Fatal("delete repeated")
			}
		})
	}
}
