// Copyright (c) Matthew Mellor
// SPDX-License-Identifier: MPL-2.0

package system_test

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
	"time"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/matthew-on-git/terraform-provider-opnsense/internal/provider"
)

const (
	groupType = "opnsense_system_gateway_group"
	groupUUID = "00000000-0000-0000-0000-000000000031"
)

type groupAPI struct {
	mu                                                                                                   sync.Mutex
	payload                                                                                              map[string]string
	adds, saves, deletes, activations                                                                    int
	failActivation, malformedRead, ambiguousAdd, ambiguousSave, ambiguousDelete, inUse, modelUnavailable bool
	version                                                                                              string
	loseAdd, multipleAdd, failReadAfterMutation, transportAdd, mismatchSave, preserveDelete              bool
	objectStatus                                                                                         int
	missingRaw                                                                                           string
	malformedAfterDelete                                                                                 bool
	modelReads, eligibleAfter                                                                            int
	membersAbsent                                                                                        bool
	earlyMutations                                                                                       int
	preserveFirstTier                                                                                    bool
}

func (g *groupAPI) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	g.mu.Lock()
	defer g.mu.Unlock()
	switch r.URL.Path {
	case "/api/core/firmware/status":
		version := g.version
		if version == "" {
			version = "26.7.7"
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"product": map[string]string{"product_name": "OPNsense", "product_version": version}})
	case "/api/routing/group_settings/get":
		if g.modelUnavailable {
			_, _ = fmt.Fprint(w, `{}`)
			return
		}
		g.modelReads++
		model := groupWire(nil)
		if !g.membersAbsent && g.modelReads >= g.eligibleAfter {
			for _, field := range []string{"item", "item2", "item3", "item4", "item5"} {
				model[field] = map[string]any{"synthetic_a": map[string]any{"selected": 0, "value": "unrelated label a"}, "synthetic_b": map[string]any{"selected": 0, "value": "unrelated label b"}, "synthetic_c": map[string]any{"selected": 0, "value": "unrelated label c"}}
			}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"gateway_group": model})
	case "/api/routing/group_settings/search":
		rows := []map[string]string{}
		if g.payload != nil {
			rows = append(rows, map[string]string{"uuid": groupUUID, "name": g.payload["name"]})
			if g.multipleAdd {
				rows = append(rows, map[string]string{"uuid": "00000000-0000-0000-0000-000000000032", "name": g.payload["name"]})
			}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"rows": rows, "rowCount": len(rows), "total": len(rows), "current": 1})
	case "/api/routing/group_settings/get/" + groupUUID, "/api/routing/group_settings/get/00000000-0000-0000-0000-000000000032":
		if g.objectStatus != 0 {
			http.Error(w, "fixture unavailable object", g.objectStatus)
			return
		}
		if g.malformedRead {
			_, _ = fmt.Fprint(w, `{"gateway_group":{"name":"synthetic_group"}}`)
			return
		}
		if g.payload == nil {
			if g.missingRaw != "" {
				_, _ = fmt.Fprint(w, g.missingRaw)
				return
			}
			_, _ = fmt.Fprint(w, `[]`)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"gateway_group": groupWire(g.payload)})
	case "/api/routing/group_settings/add", "/api/routing/group_settings/set/" + groupUUID:
		if g.membersAbsent || g.modelReads < g.eligibleAfter {
			g.earlyMutations++
			_, _ = fmt.Fprint(w, `{"result":"failed"}`)
			return
		}
		var body struct {
			Group map[string]string `json:"gateway_group"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "invalid fixture request", 400)
			return
		}
		if g.preserveFirstTier && !strings.HasSuffix(r.URL.Path, "add") && body.Group["item"] == "" && g.payload != nil {
			// Tagged GatewayGroupItemField ignores empty updates but still returns saved.
			body.Group["item"] = g.payload["item"]
		}
		g.payload = body.Group
		if g.failReadAfterMutation {
			g.malformedRead = true
		}
		if strings.HasSuffix(r.URL.Path, "add") {
			g.adds++
			if g.loseAdd {
				g.payload = nil
				_, _ = fmt.Fprint(w, `{}`)
				return
			}
			if g.transportAdd {
				http.Error(w, "uncertain committed add", 500)
				return
			}
			if g.ambiguousAdd {
				_, _ = fmt.Fprint(w, `{}`)
				return
			}
			_, _ = fmt.Fprint(w, `{"result":"saved","uuid":"`+groupUUID+`"}`)
		} else {
			g.saves++
			if g.mismatchSave {
				g.payload["descr"] = "authoritative other description"
				_, _ = fmt.Fprint(w, `{}`)
				return
			}
			if g.ambiguousSave {
				_, _ = fmt.Fprint(w, `{}`)
				return
			}
			_, _ = fmt.Fprint(w, `{"result":"saved"}`)
		}
	case "/api/routing/group_settings/del/" + groupUUID:
		g.deletes++
		if g.malformedAfterDelete {
			g.malformedRead = true
		}
		if g.inUse {
			http.Error(w, "group is in use", 500)
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
	case "/api/routing/group_settings/reconfigure":
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

func groupWire(payload map[string]string) map[string]any {
	result := map[string]any{"name": "", "descr": ""}
	if payload != nil {
		result["name"] = payload["name"]
		result["descr"] = payload["descr"]
	}
	for _, key := range []string{"item", "item2", "item3", "item4", "item5"} {
		selected := map[string]any{}
		if payload != nil && payload[key] != "" {
			for _, name := range strings.Split(payload[key], ",") {
				selected[name] = map[string]any{"value": "label differs", "selected": 1}
			}
		}
		result[key] = selected
	}
	for key, values := range map[string][]string{"trigger": {"down", "downloss", "downlatency", "downlosslatency"}, "poolopts": {"", "round-robin", "round-robin sticky-address"}} {
		selected := ""
		if key == "trigger" {
			selected = "down"
		}
		if payload != nil {
			selected = payload[key]
		}
		options := map[string]any{}
		for _, value := range values {
			flag := 0
			if value == selected {
				flag = 1
			}
			options[value] = map[string]any{"selected": flag}
		}
		result[key] = options
	}
	return result
}

func groupServer(t *testing.T, uri string) (tfprotov6.ProviderServer, *tfprotov6.GetProviderSchemaResponse) {
	t.Helper()
	server := providerserver.NewProtocol6(provider.New("test")())()
	schemas, err := server.GetProviderSchema(context.Background(), &tfprotov6.GetProviderSchemaRequest{})
	if err != nil {
		t.Fatal(err)
	}
	config := groupValue(t, schemas.Provider, map[string]any{"uri": uri, "api_key": "fixture-key", "api_secret": "fixture-secret", "insecure": true})
	result, err := server.ConfigureProvider(context.Background(), &tfprotov6.ConfigureProviderRequest{Config: config, TerraformVersion: "1.14.0"})
	if err != nil || groupErrors(result.Diagnostics) {
		t.Fatalf("configure: %v %+v", err, result.Diagnostics[0])
	}
	if schemas.ResourceSchemas[groupType] == nil || schemas.DataSourceSchemas[groupType] == nil {
		t.Fatal("missing exported registration")
	}
	return server, schemas
}

func groupValue(t *testing.T, schema *tfprotov6.Schema, fields map[string]any) *tfprotov6.DynamicValue {
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

func groupNull(t *testing.T, schema *tfprotov6.Schema) *tfprotov6.DynamicValue {
	t.Helper()
	value, err := tfprotov6.NewDynamicValue(schema.ValueType(), tftypes.NewValue(schema.ValueType(), nil))
	if err != nil {
		t.Fatal(err)
	}
	return &value
}

func groupErrors(diagnostics []*tfprotov6.Diagnostic) bool {
	for _, d := range diagnostics {
		if d.Severity == tfprotov6.DiagnosticSeverityError {
			return true
		}
	}
	return false
}

func groupValues() map[string]any {
	return map[string]any{"id": groupUUID, "name": "synthetic_group", "tier_1": []string{"synthetic_a", "synthetic_b"}, "tier_2": []string{}, "tier_3": []string{}, "tier_4": []string{}, "tier_5": []string{}, "trigger": "down", "pool_options": "", "description": "literal,description"}
}

func groupAttrs(t *testing.T, schema *tfprotov6.Schema, state *tfprotov6.DynamicValue) map[string]tftypes.Value {
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

func groupApply(t *testing.T, server tfprotov6.ProviderServer, _ *tfprotov6.Schema, prior, planned *tfprotov6.DynamicValue, private []byte) *tfprotov6.ApplyResourceChangeResponse {
	t.Helper()
	response, err := server.ApplyResourceChange(context.Background(), &tfprotov6.ApplyResourceChangeRequest{TypeName: groupType, PriorState: prior, PlannedState: planned, Config: planned, PlannedPrivate: private})
	if err != nil {
		t.Fatal(err)
	}
	return response
}

func TestGatewayGroupProtocolCreateRestartRecovery(t *testing.T) {
	for _, ambiguous := range []bool{false, true} {
		t.Run(fmt.Sprintf("ambiguous=%v", ambiguous), func(t *testing.T) {
			api := &groupAPI{failActivation: true, ambiguousAdd: ambiguous}
			httpServer := httptest.NewServer(api)
			defer httpServer.Close()
			server, schemas := groupServer(t, httpServer.URL)
			schema := schemas.ResourceSchemas[groupType]
			values := groupValues()
			values["id"] = nil
			plan := groupValue(t, schema, values)
			failed := groupApply(t, server, schema, groupNull(t, schema), plan, nil)
			if !groupErrors(failed.Diagnostics) || len(failed.Private) == 0 {
				t.Fatal("missing failed activation state", failed.Diagnostics)
			}
			var id string
			if err := groupAttrs(t, schema, failed.NewState)["id"].As(&id); err != nil || id != groupUUID {
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
			fresh, _ := groupServer(t, httpServer.URL)
			recovered, err := fresh.ReadResource(context.Background(), &tfprotov6.ReadResourceRequest{TypeName: groupType, CurrentState: persisted.NewState, Private: persisted.Private})
			if err != nil || groupErrors(recovered.Diagnostics) {
				t.Fatal(err, recovered.Diagnostics)
			}
			if api.adds != 1 || api.activations != 2 {
				t.Fatalf("add=%d activations=%d", api.adds, api.activations)
			}
			lookup, err := fresh.ReadDataSource(context.Background(), &tfprotov6.ReadDataSourceRequest{TypeName: groupType, Config: groupValue(t, schemas.DataSourceSchemas[groupType], map[string]any{"id": groupUUID})})
			if err != nil || groupErrors(lookup.Diagnostics) {
				t.Fatal(err, lookup.Diagnostics)
			}
			got := groupAttrs(t, schemas.DataSourceSchemas[groupType], lookup.State)
			want := groupAttrs(t, schema, recovered.NewState)
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

func TestGatewayGroupProtocolSavedUpdateRestartRecovery(t *testing.T) {
	for _, ambiguous := range []bool{false, true} {
		t.Run(fmt.Sprintf("ambiguous=%v", ambiguous), func(t *testing.T) {
			api := &groupAPI{}
			httpServer := httptest.NewServer(api)
			defer httpServer.Close()
			server, schemas := groupServer(t, httpServer.URL)
			schema := schemas.ResourceSchemas[groupType]
			values := groupValues()
			created := groupApply(t, server, schema, groupNull(t, schema), groupValue(t, schema, values), nil)
			if groupErrors(created.Diagnostics) {
				t.Fatal(created.Diagnostics)
			}
			values["description"] = "new,ordered,description"
			values["tier_2"] = []string{"synthetic_c"}
			values["trigger"] = "downlosslatency"
			values["pool_options"] = "round-robin sticky-address"
			api.failActivation = true
			api.ambiguousSave = ambiguous
			failed := groupApply(t, server, schema, created.NewState, groupValue(t, schema, values), created.Private)
			if !groupErrors(failed.Diagnostics) || len(failed.Private) == 0 {
				t.Fatal(failed.Diagnostics)
			}
			var description string
			if err := groupAttrs(t, schema, failed.NewState)["description"].As(&description); err != nil || description != values["description"] {
				t.Fatal("reverted committed values", description, err)
			}
			api.failActivation = false
			fresh, _ := groupServer(t, httpServer.URL)
			recovered, err := fresh.ReadResource(context.Background(), &tfprotov6.ReadResourceRequest{TypeName: groupType, CurrentState: failed.NewState, Private: failed.Private})
			if err != nil || groupErrors(recovered.Diagnostics) || api.saves != 1 {
				t.Fatal(err, recovered.Diagnostics, api.saves)
			}
			values["tier_2"] = []string{}
			values["description"] = ""
			values["pool_options"] = ""
			cleared := groupApply(t, fresh, schema, recovered.NewState, groupValue(t, schema, values), recovered.Private)
			if groupErrors(cleared.Diagnostics) || api.payload["item2"] != "" || api.payload["item"] != "synthetic_a,synthetic_b" || api.payload["descr"] != "" || api.payload["poolopts"] != "" {
				t.Fatal("clear failed", cleared.Diagnostics, api.payload)
			}
		})
	}
}

func TestGatewayGroupProtocolDeleteRestartAndInUse(t *testing.T) {
	for _, mode := range []string{"confirmed", "ambiguous", "in-use"} {
		t.Run(mode, func(t *testing.T) {
			api := &groupAPI{}
			httpServer := httptest.NewServer(api)
			defer httpServer.Close()
			server, schemas := groupServer(t, httpServer.URL)
			schema := schemas.ResourceSchemas[groupType]
			created := groupApply(t, server, schema, groupNull(t, schema), groupValue(t, schema, groupValues()), nil)
			if groupErrors(created.Diagnostics) {
				t.Fatal(created.Diagnostics)
			}
			api.failActivation = true
			api.ambiguousDelete = mode == "ambiguous"
			api.inUse = mode == "in-use"
			failed := groupApply(t, server, schema, created.NewState, groupNull(t, schema), created.Private)
			if !groupErrors(failed.Diagnostics) {
				t.Fatal("expected delete diagnostic")
			}
			attrs := groupAttrs(t, schema, failed.NewState)
			var id string
			if err := attrs["id"].As(&id); err != nil || id != groupUUID {
				t.Fatal("delete lost identity", err)
			}
			if mode == "in-use" {
				if api.payload == nil || api.activations != 1 {
					t.Fatal("in-use delete misclassified")
				}
				return
			}
			if len(failed.Private) == 0 {
				t.Fatal("delete recovery marker absent")
			}
			api.failActivation = false
			fresh, _ := groupServer(t, httpServer.URL)
			recovered := groupApply(t, fresh, schema, failed.NewState, groupNull(t, schema), failed.Private)
			if groupErrors(recovered.Diagnostics) || api.deletes != 1 {
				t.Fatal("replayed deletion", recovered.Diagnostics, api.deletes)
			}
			value, err := recovered.NewState.Unmarshal(schema.ValueType())
			if err != nil || !value.IsNull() {
				t.Fatal("delete did not converge", err)
			}
		})
	}
}

func TestGatewayGroupProtocolStrictReadRetainsStateAndGate(t *testing.T) {
	api := &groupAPI{}
	httpServer := httptest.NewServer(api)
	defer httpServer.Close()
	server, schemas := groupServer(t, httpServer.URL)
	schema := schemas.ResourceSchemas[groupType]
	created := groupApply(t, server, schema, groupNull(t, schema), groupValue(t, schema, groupValues()), nil)
	if groupErrors(created.Diagnostics) {
		t.Fatal(created.Diagnostics)
	}
	for _, mode := range []string{"malformed", "model unavailable", "older", "prerelease", "unknown"} {
		t.Run(mode, func(t *testing.T) {
			api.malformedRead = mode == "malformed"
			api.modelUnavailable = mode == "model unavailable"
			api.version = ""
			switch mode {
			case "older":
				api.version = "26.7.6"
			case "prerelease":
				api.version = "26.7.7-devel"
			case "unknown":
				api.version = "future"
			}
			read, err := server.ReadResource(context.Background(), &tfprotov6.ReadResourceRequest{TypeName: groupType, CurrentState: created.NewState})
			if err != nil || !groupErrors(read.Diagnostics) {
				t.Fatal("expected strict diagnostic", err, read.Diagnostics)
			}
			if !reflect.DeepEqual(groupAttrs(t, schema, created.NewState), groupAttrs(t, schema, read.NewState)) {
				t.Fatal("failed read changed prior state")
			}
			if api.adds != 1 || api.activations != 1 {
				t.Fatal("gate mutated")
			}
		})
	}
}

func TestGatewayGroupProtocolDefaultsImportNoChangeAndCancellation(t *testing.T) {
	api := &groupAPI{}
	httpServer := httptest.NewServer(api)
	defer httpServer.Close()
	server, schemas := groupServer(t, httpServer.URL)
	schema := schemas.ResourceSchemas[groupType]
	config := groupValue(t, schema, map[string]any{"name": "synthetic_group", "tier_1": []string{"synthetic_a"}})
	planned, err := server.PlanResourceChange(context.Background(), &tfprotov6.PlanResourceChangeRequest{TypeName: groupType, PriorState: groupNull(t, schema), ProposedNewState: config, Config: config})
	if err != nil || groupErrors(planned.Diagnostics) {
		t.Fatal(err, planned.Diagnostics)
	}
	attrs := groupAttrs(t, schema, planned.PlannedState)
	for _, field := range []string{"tier_2", "tier_3", "tier_4", "tier_5", "description", "pool_options", "trigger"} {
		if attrs[field].IsNull() || !attrs[field].IsKnown() {
			t.Fatalf("omitted %s lacks known default", field)
		}
	}
	applied := groupApply(t, server, schema, groupNull(t, schema), planned.PlannedState, planned.PlannedPrivate)
	if groupErrors(applied.Diagnostics) {
		t.Fatal(applied.Diagnostics)
	}
	imported, err := server.ImportResourceState(context.Background(), &tfprotov6.ImportResourceStateRequest{TypeName: groupType, ID: groupUUID})
	if err != nil || groupErrors(imported.Diagnostics) || len(imported.ImportedResources) != 1 {
		t.Fatal(err, imported.Diagnostics)
	}
	read, err := server.ReadResource(context.Background(), &tfprotov6.ReadResourceRequest{TypeName: groupType, CurrentState: imported.ImportedResources[0].State})
	if err != nil || groupErrors(read.Diagnostics) {
		t.Fatal(err, read.Diagnostics)
	}
	noChange, err := server.PlanResourceChange(context.Background(), &tfprotov6.PlanResourceChangeRequest{TypeName: groupType, PriorState: read.NewState, ProposedNewState: read.NewState, Config: config})
	if err != nil || groupErrors(noChange.Diagnostics) {
		t.Fatal(err, noChange.Diagnostics)
	}
	stateValue, err := read.NewState.Unmarshal(schema.ValueType())
	if err != nil {
		t.Fatal(err)
	}
	planValue, err := noChange.PlannedState.Unmarshal(schema.ValueType())
	if err != nil || !stateValue.Equal(planValue) {
		t.Fatal("not a no-change import", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	canceled, err := server.ReadResource(ctx, &tfprotov6.ReadResourceRequest{TypeName: groupType, CurrentState: read.NewState})
	if err == nil && !groupErrors(canceled.Diagnostics) {
		t.Fatal("cancellation ignored")
	}
	if api.adds != 1 || api.saves != 0 {
		t.Fatal("canceled operation mutated")
	}
}

func TestGatewayGroupProtocolUncertainAddMatchesAndNoAutomaticRetry(t *testing.T) {
	for _, mode := range []string{"zero", "multiple", "http500"} {
		t.Run(mode, func(t *testing.T) {
			api := &groupAPI{ambiguousAdd: true, loseAdd: mode == "zero", multipleAdd: mode == "multiple", transportAdd: mode == "http500"}
			httpServer := httptest.NewServer(api)
			defer httpServer.Close()
			server, schemas := groupServer(t, httpServer.URL)
			schema := schemas.ResourceSchemas[groupType]
			response := groupApply(t, server, schema, groupNull(t, schema), groupValue(t, schema, groupValues()), nil)
			if mode == "http500" {
				if groupErrors(response.Diagnostics) {
					t.Fatal(response.Diagnostics)
				}
			} else if !groupErrors(response.Diagnostics) {
				t.Fatal("ambiguous adoption did not fail closed")
			}
			if api.adds != 1 {
				t.Fatalf("automatic add retry occurred: %d", api.adds)
			}
			if mode == "multiple" {
				retry := groupApply(t, server, schema, groupNull(t, schema), groupValue(t, schema, groupValues()), nil)
				if !groupErrors(retry.Diagnostics) || api.adds != 1 {
					t.Fatal("blind duplicate add")
				}
			}
		})
	}
}

func TestGatewayGroupProtocolCommittedReadFailureAndAmbiguousContradiction(t *testing.T) {
	for _, mode := range []string{"create read failure", "update read failure", "contradictory save", "delete still present"} {
		t.Run(mode, func(t *testing.T) {
			api := &groupAPI{}
			httpServer := httptest.NewServer(api)
			defer httpServer.Close()
			server, schemas := groupServer(t, httpServer.URL)
			schema := schemas.ResourceSchemas[groupType]
			values := groupValues()
			if mode == "create read failure" {
				api.failReadAfterMutation = true
			}
			first := groupApply(t, server, schema, groupNull(t, schema), groupValue(t, schema, values), nil)
			failed := first
			switch mode {
			case "update read failure":
				api.failReadAfterMutation = true
				values["description"] = "confirmed changed value"
				failed = groupApply(t, server, schema, first.NewState, groupValue(t, schema, values), first.Private)
			case "contradictory save":
				api.mismatchSave = true
				values["description"] = "desired changed value"
				failed = groupApply(t, server, schema, first.NewState, groupValue(t, schema, values), first.Private)
			case "delete still present":
				api.ambiguousDelete = true
				api.preserveDelete = true
				failed = groupApply(t, server, schema, first.NewState, groupNull(t, schema), first.Private)
			}
			if !groupErrors(failed.Diagnostics) {
				t.Fatal("fault produced no diagnostic")
			}
			attrs := groupAttrs(t, schema, failed.NewState)
			var id, description string
			if err := attrs["id"].As(&id); err != nil || id != groupUUID {
				t.Fatal("lost identity", err)
			}
			if err := attrs["description"].As(&description); err != nil {
				t.Fatal(err)
			}
			switch mode {
			case "create read failure":
				if description != "literal,description" {
					t.Fatal("lost committed create values")
				}
			case "update read failure":
				if description != "confirmed changed value" {
					t.Fatal("lost committed update values")
				}
			case "contradictory save":
				if description != "authoritative other description" {
					t.Fatal("did not persist authoritative saved truth")
				}
			case "delete still present":
				if api.payload == nil || api.deletes != 1 || api.activations != 1 {
					t.Fatal("uncertain present delete lost truth")
				}
				return
			}
			if len(failed.Private) == 0 {
				t.Fatal("no durable marker")
			}
			api.malformedRead = false
			fresh, _ := groupServer(t, httpServer.URL)
			read, err := fresh.ReadResource(context.Background(), &tfprotov6.ReadResourceRequest{TypeName: groupType, CurrentState: failed.NewState, Private: failed.Private})
			if err != nil || groupErrors(read.Diagnostics) {
				t.Fatal(err, read.Diagnostics)
			}
			if api.adds != 1 || api.saves > 1 {
				t.Fatal("recovery repeated save")
			}
		})
	}
}

func TestGatewayGroupProtocolConcurrentCreateSerializesDiscovery(t *testing.T) {
	api := &groupAPI{}
	httpServer := httptest.NewServer(api)
	defer httpServer.Close()
	server, schemas := groupServer(t, httpServer.URL)
	schema := schemas.ResourceSchemas[groupType]
	prior := groupNull(t, schema)
	planned := groupValue(t, schema, groupValues())
	responses := make(chan *tfprotov6.ApplyResourceChangeResponse, 2)
	errs := make(chan error, 2)
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			response, err := server.ApplyResourceChange(context.Background(), &tfprotov6.ApplyResourceChangeRequest{TypeName: groupType, PriorState: prior, PlannedState: planned, Config: planned})
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
		if !groupErrors(response.Diagnostics) {
			success++
		}
	}
	if success != 1 || api.adds != 1 {
		t.Fatalf("success=%d adds=%d; discovery was not serialized", success, api.adds)
	}
}

func TestGatewayGroupProtocolCorruptPrivateFailsClosed(t *testing.T) {
	api := &groupAPI{failActivation: true}
	httpServer := httptest.NewServer(api)
	defer httpServer.Close()
	server, schemas := groupServer(t, httpServer.URL)
	schema := schemas.ResourceSchemas[groupType]
	failed := groupApply(t, server, schema, groupNull(t, schema), groupValue(t, schema, groupValues()), nil)
	if !groupErrors(failed.Diagnostics) {
		t.Fatal("expected failed activation")
	}
	var data map[string][]byte
	if err := json.Unmarshal(failed.Private, &data); err != nil {
		t.Fatal(err)
	}
	data["gateway_group_pending_v1"] = []byte(`{"unsupported":"marker"}`)
	corrupt, err := json.Marshal(data)
	if err != nil {
		t.Fatal(err)
	}
	api.failActivation = false
	fresh, _ := groupServer(t, httpServer.URL)
	read, err := fresh.ReadResource(context.Background(), &tfprotov6.ReadResourceRequest{TypeName: groupType, CurrentState: failed.NewState, Private: corrupt})
	if err != nil || !groupErrors(read.Diagnostics) || api.activations != 1 {
		t.Fatal("corrupt marker defaulted or activated", err, read.Diagnostics)
	}
	if !reflect.DeepEqual(groupAttrs(t, schema, failed.NewState), groupAttrs(t, schema, read.NewState)) {
		t.Fatal("corrupt marker lost state")
	}
}

func TestGatewayGroupProtocolUnresolvedAndEmptyRejectBeforeWrite(t *testing.T) {
	for _, mode := range []string{"unknown set", "unknown member", "null member", "empty member", "whitespace member", "all empty"} {
		t.Run(mode, func(t *testing.T) {
			api := &groupAPI{}
			httpServer := httptest.NewServer(api)
			defer httpServer.Close()
			server, schemas := groupServer(t, httpServer.URL)
			schema := schemas.ResourceSchemas[groupType]
			values := groupValues()
			values["id"] = nil
			switch mode {
			case "null member":
				values["tier_2"] = []any{nil}
			case "empty member":
				values["tier_2"] = []string{""}
			case "whitespace member":
				values["tier_2"] = []string{" "}
			case "all empty":
				values["tier_1"] = []string{}
			}
			plan := groupValue(t, schema, values)
			if mode == "unknown set" || mode == "unknown member" {
				attrs := groupAttrs(t, schema, plan)
				value := tftypes.NewValue(tftypes.Set{ElementType: tftypes.String}, tftypes.UnknownValue)
				if mode == "unknown member" {
					value = tftypes.NewValue(tftypes.Set{ElementType: tftypes.String}, []tftypes.Value{tftypes.NewValue(tftypes.String, tftypes.UnknownValue)})
				}
				attrs["tier_2"] = value
				dynamic, err := tfprotov6.NewDynamicValue(schema.ValueType(), tftypes.NewValue(schema.ValueType(), attrs))
				if err != nil {
					t.Fatal(err)
				}
				plan = &dynamic
			}
			response := groupApply(t, server, schema, groupNull(t, schema), plan, nil)
			if !groupErrors(response.Diagnostics) || api.adds != 0 || api.saves != 0 || api.activations != 0 {
				t.Fatal("invalid member reached write", response.Diagnostics)
			}
		})
	}
}

func TestGatewayGroupProtocolValidateConfigAndImmutableName(t *testing.T) {
	api := &groupAPI{}
	httpServer := httptest.NewServer(api)
	defer httpServer.Close()
	server, schemas := groupServer(t, httpServer.URL)
	schema := schemas.ResourceSchemas[groupType]
	for _, mode := range []string{"valid", "empty union", "whitespace", "empty member"} {
		t.Run(mode, func(t *testing.T) {
			values := map[string]any{"name": "synthetic_group", "tier_1": []string{"synthetic_a"}}
			switch mode {
			case "empty union":
				values["tier_1"] = []string{}
			case "whitespace":
				values["tier_1"] = []string{"synthetic a"}
			case "empty member":
				values["tier_1"] = []string{""}
			}
			validated, err := server.ValidateResourceConfig(context.Background(), &tfprotov6.ValidateResourceConfigRequest{TypeName: groupType, Config: groupValue(t, schema, values)})
			if err != nil || groupErrors(validated.Diagnostics) != (mode != "valid") {
				t.Fatal("config validation", err, validated.Diagnostics)
			}
		})
	}
	values := groupValues()
	prior := groupValue(t, schema, values)
	values["name"] = "replacement_group"
	config := groupValue(t, schema, values)
	plan, err := server.PlanResourceChange(context.Background(), &tfprotov6.PlanResourceChangeRequest{TypeName: groupType, PriorState: prior, ProposedNewState: config, Config: config})
	if err != nil || groupErrors(plan.Diagnostics) || len(plan.RequiresReplace) != 1 {
		t.Fatal("name did not require replacement", err, plan.Diagnostics, plan.RequiresReplace)
	}
	if api.adds != 0 || api.saves != 0 {
		t.Fatal("planning mutated")
	}
}

func TestGatewayGroupProtocolMissingRequiresActualEvidence(t *testing.T) {
	for _, mode := range []string{"empty array", "wrapped empty array", "404", "empty object", "null", "missing wrapper", "403"} {
		t.Run(mode, func(t *testing.T) {
			api := &groupAPI{}
			httpServer := httptest.NewServer(api)
			defer httpServer.Close()
			server, schemas := groupServer(t, httpServer.URL)
			schema := schemas.ResourceSchemas[groupType]
			created := groupApply(t, server, schema, groupNull(t, schema), groupValue(t, schema, groupValues()), nil)
			if groupErrors(created.Diagnostics) {
				t.Fatal(created.Diagnostics)
			}
			api.payload = nil
			missing := false
			switch mode {
			case "empty array":
				missing = true
			case "wrapped empty array":
				api.missingRaw = `{"gateway_group":[]}`
				missing = true
			case "404":
				api.objectStatus = 404
				missing = true
			case "empty object":
				api.missingRaw = `{}`
			case "null":
				api.missingRaw = `null`
			case "missing wrapper":
				api.missingRaw = `{"other":[]}`
			case "403":
				api.objectStatus = 403
			}
			read, err := server.ReadResource(context.Background(), &tfprotov6.ReadResourceRequest{TypeName: groupType, CurrentState: created.NewState})
			if err != nil {
				t.Fatal(err)
			}
			value, err := read.NewState.Unmarshal(schema.ValueType())
			if err != nil {
				t.Fatal(err)
			}
			if missing {
				if groupErrors(read.Diagnostics) || !value.IsNull() {
					t.Fatal("actual missing not removed", read.Diagnostics)
				}
			} else if !groupErrors(read.Diagnostics) || value.IsNull() {
				t.Fatal("malformed/unavailable object removed", read.Diagnostics)
			}
		})
	}
}

func TestGatewayGroupProtocolAmbiguousDeleteUnknownRestartChecksTruthFirst(t *testing.T) {
	for _, stillPresent := range []bool{false, true} {
		t.Run(fmt.Sprintf("still_present=%v", stillPresent), func(t *testing.T) {
			api := &groupAPI{}
			httpServer := httptest.NewServer(api)
			defer httpServer.Close()
			server, schemas := groupServer(t, httpServer.URL)
			schema := schemas.ResourceSchemas[groupType]
			created := groupApply(t, server, schema, groupNull(t, schema), groupValue(t, schema, groupValues()), nil)
			if groupErrors(created.Diagnostics) {
				t.Fatal(created.Diagnostics)
			}
			api.ambiguousDelete = true
			api.preserveDelete = stillPresent
			api.malformedAfterDelete = true
			failed := groupApply(t, server, schema, created.NewState, groupNull(t, schema), created.Private)
			if !groupErrors(failed.Diagnostics) || len(failed.Private) == 0 {
				t.Fatal("unknown deletion not retained", failed.Diagnostics)
			}
			api.malformedRead = false
			api.malformedAfterDelete = false
			fresh, _ := groupServer(t, httpServer.URL)
			read, err := fresh.ReadResource(context.Background(), &tfprotov6.ReadResourceRequest{TypeName: groupType, CurrentState: failed.NewState, Private: failed.Private})
			if err != nil {
				t.Fatal(err)
			}
			if stillPresent {
				if groupErrors(read.Diagnostics) || api.activations != 1 || api.deletes != 1 {
					t.Fatal("uncertain delete activated before checking present truth", read.Diagnostics)
				}
				api.preserveDelete = false
				removed := groupApply(t, fresh, schema, read.NewState, groupNull(t, schema), read.Private)
				if groupErrors(removed.Diagnostics) || api.deletes != 2 {
					t.Fatal("verified still-present destroy failed", removed.Diagnostics)
				}
			} else {
				value, err := read.NewState.Unmarshal(schema.ValueType())
				if err != nil || !value.IsNull() || groupErrors(read.Diagnostics) || api.deletes != 1 || api.activations != 2 {
					t.Fatal("missing truth recovery failed", err, read.Diagnostics)
				}
			}
		})
	}
}

func TestGatewayGroupProtocolModelEligibilityStaleThenEligible(t *testing.T) {
	for _, operation := range []string{"create", "update"} {
		t.Run(operation, func(t *testing.T) {
			api := &groupAPI{}
			httpServer := httptest.NewServer(api)
			defer httpServer.Close()
			server, schemas := groupServer(t, httpServer.URL)
			schema := schemas.ResourceSchemas[groupType]
			values := groupValues()
			prior := groupNull(t, schema)
			if operation == "update" {
				created := groupApply(t, server, schema, prior, groupValue(t, schema, values), nil)
				if groupErrors(created.Diagnostics) {
					t.Fatal(created.Diagnostics)
				}
				prior = created.NewState
				values["tier_2"] = []string{"synthetic_c"}
			}
			api.eligibleAfter = api.modelReads + 3
			response := groupApply(t, server, schema, prior, groupValue(t, schema, values), nil)
			if groupErrors(response.Diagnostics) || api.modelReads != api.eligibleAfter || api.adds != 1 || api.earlyMutations != 0 {
				t.Fatal("stale selector did not converge with one mutation", response.Diagnostics, api.modelReads, api.adds)
			}
			if operation == "update" && api.saves != 1 {
				t.Fatal("update was not saved exactly once")
			}
		})
	}
}

func TestGatewayGroupProtocolModelEligibilityAbsentDeadlineCancellationNoWrite(t *testing.T) {
	for _, operation := range []string{"create", "update"} {
		for _, cancellation := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/cancel=%v", operation, cancellation), func(t *testing.T) {
				api := &groupAPI{}
				httpServer := httptest.NewServer(api)
				defer httpServer.Close()
				server, schemas := groupServer(t, httpServer.URL)
				schema := schemas.ResourceSchemas[groupType]
				values := groupValues()
				prior := groupNull(t, schema)
				if operation == "update" {
					created := groupApply(t, server, schema, prior, groupValue(t, schema, values), nil)
					if groupErrors(created.Diagnostics) {
						t.Fatal(created.Diagnostics)
					}
					prior = created.NewState
					values["tier_2"] = []string{"synthetic_c"}
				}
				api.membersAbsent = true
				beforeAdds, beforeSaves, beforeActivations := api.adds, api.saves, api.activations
				ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
				defer cancel()
				if cancellation {
					timer := time.AfterFunc(40*time.Millisecond, cancel)
					defer timer.Stop()
				}
				response, err := server.ApplyResourceChange(ctx, &tfprotov6.ApplyResourceChangeRequest{TypeName: groupType, PriorState: prior, PlannedState: groupValue(t, schema, values), Config: groupValue(t, schema, values)})
				if err != nil || !groupErrors(response.Diagnostics) {
					t.Fatal("absent selector must diagnose", err, response.Diagnostics)
				}
				if api.adds != beforeAdds || api.saves != beforeSaves || api.activations != beforeActivations {
					t.Fatal("ineligible member caused a mutation")
				}
				if len(response.Private) != 0 {
					t.Fatal("read-only eligibility failure manufactured uncertain mutation recovery")
				}
				if operation == "update" && !reflect.DeepEqual(groupAttrs(t, schema, prior), groupAttrs(t, schema, response.NewState)) {
					t.Fatal("preflight failure changed saved state")
				}
			})
		}
	}
}

func TestGatewayGroupProtocolModelEligibilityValidControlNoWait(t *testing.T) {
	api := &groupAPI{}
	httpServer := httptest.NewServer(api)
	defer httpServer.Close()
	server, schemas := groupServer(t, httpServer.URL)
	schema := schemas.ResourceSchemas[groupType]
	response := groupApply(t, server, schema, groupNull(t, schema), groupValue(t, schema, groupValues()), nil)
	if groupErrors(response.Diagnostics) || api.adds != 1 || api.modelReads != 2 {
		t.Fatal("valid model options should save immediately", response.Diagnostics, api.modelReads)
	}
}

func TestGatewayGroupProtocolPendingRecoveryIgnoresSelectorCache(t *testing.T) {
	api := &groupAPI{failActivation: true}
	httpServer := httptest.NewServer(api)
	defer httpServer.Close()
	server, schemas := groupServer(t, httpServer.URL)
	schema := schemas.ResourceSchemas[groupType]
	failed := groupApply(t, server, schema, groupNull(t, schema), groupValue(t, schema, groupValues()), nil)
	if !groupErrors(failed.Diagnostics) || len(failed.Private) == 0 {
		t.Fatal("expected durable saved-operation marker", failed.Diagnostics)
	}
	api.failActivation = false
	api.membersAbsent = true
	fresh, _ := groupServer(t, httpServer.URL)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	recovered, err := fresh.ReadResource(ctx, &tfprotov6.ReadResourceRequest{TypeName: groupType, CurrentState: failed.NewState, Private: failed.Private})
	if err != nil || groupErrors(recovered.Diagnostics) || api.adds != 1 || api.activations != 2 {
		t.Fatal("saved recovery incorrectly waited for selector eligibility or repeated save", err, recovered.Diagnostics)
	}
}

func TestGatewayGroupProtocolPopulatedFirstTierClearFailsBeforeAnyWrite(t *testing.T) {
	api := &groupAPI{preserveFirstTier: true}
	httpServer := httptest.NewServer(api)
	defer httpServer.Close()
	server, schemas := groupServer(t, httpServer.URL)
	schema := schemas.ResourceSchemas[groupType]
	values := groupValues()
	values["tier_2"] = []string{"synthetic_c"}
	created := groupApply(t, server, schema, groupNull(t, schema), groupValue(t, schema, values), nil)
	if groupErrors(created.Diagnostics) {
		t.Fatal(created.Diagnostics)
	}
	values["tier_1"] = []string{}
	values["description"] = "must not partially save"
	values["trigger"] = "downlosslatency"
	refused := groupApply(t, server, schema, created.NewState, groupValue(t, schema, values), created.Private)
	if !groupErrors(refused.Diagnostics) || api.saves != 0 || api.activations != 1 || len(refused.Private) != 0 {
		t.Fatal("unsupported first-tier clear mutated or claimed success", refused.Diagnostics, api.saves, api.activations)
	}
	if !reflect.DeepEqual(groupAttrs(t, schema, created.NewState), groupAttrs(t, schema, refused.NewState)) {
		t.Fatal("first-tier clear lost UUID or changed other saved fields")
	}
	if api.payload["item"] != "synthetic_a,synthetic_b" || api.payload["item2"] != "synthetic_c" || api.payload["descr"] != "literal,description" {
		t.Fatal("first-tier clear partially changed remote fields")
	}
}

func TestGatewayGroupProtocolInitiallyEmptyFirstTierRemainsValid(t *testing.T) {
	api := &groupAPI{preserveFirstTier: true}
	httpServer := httptest.NewServer(api)
	defer httpServer.Close()
	server, schemas := groupServer(t, httpServer.URL)
	schema := schemas.ResourceSchemas[groupType]
	values := groupValues()
	values["tier_1"] = []string{}
	values["tier_2"] = []string{"synthetic_c"}
	created := groupApply(t, server, schema, groupNull(t, schema), groupValue(t, schema, values), nil)
	if groupErrors(created.Diagnostics) {
		t.Fatal("initial tier2-only group rejected", created.Diagnostics)
	}
	values["description"] = "updated tier2-only group"
	updated := groupApply(t, server, schema, created.NewState, groupValue(t, schema, values), created.Private)
	if groupErrors(updated.Diagnostics) || api.saves != 1 || api.payload["item"] != "" || api.payload["item2"] != "synthetic_c" {
		t.Fatal("already-empty tier1 update rejected", updated.Diagnostics)
	}
}

func TestGatewayGroupProtocolFirstTierClearUsesAuthoritativeTruthNotPriorState(t *testing.T) {
	for _, remotePopulated := range []bool{false, true} {
		t.Run(fmt.Sprintf("remote_populated=%v", remotePopulated), func(t *testing.T) {
			api := &groupAPI{preserveFirstTier: true}
			httpServer := httptest.NewServer(api)
			defer httpServer.Close()
			server, schemas := groupServer(t, httpServer.URL)
			schema := schemas.ResourceSchemas[groupType]
			values := groupValues()
			values["tier_2"] = []string{"synthetic_c"}
			if remotePopulated {
				values["tier_1"] = []string{}
			}
			created := groupApply(t, server, schema, groupNull(t, schema), groupValue(t, schema, values), nil)
			if groupErrors(created.Diagnostics) {
				t.Fatal(created.Diagnostics)
			}
			if remotePopulated {
				api.payload["item"] = "synthetic_a"
			} else {
				api.payload["item"] = ""
			}
			values["tier_1"] = []string{}
			values["description"] = "new intent"
			result := groupApply(t, server, schema, created.NewState, groupValue(t, schema, values), created.Private)
			if remotePopulated {
				if !groupErrors(result.Diagnostics) || api.saves != 0 || api.payload["descr"] != "literal,description" {
					t.Fatal("stale empty TF state bypassed actual populated first-tier guard", result.Diagnostics)
				}
				attrs := groupAttrs(t, schema, result.NewState)
				var members []tftypes.Value
				if err := attrs["tier_1"].As(&members); err != nil || len(members) != 1 {
					t.Fatal("authoritative populated first tier not retained", err)
				}
			} else if groupErrors(result.Diagnostics) || api.saves != 1 || api.payload["item"] != "" || api.payload["descr"] != "new intent" {
				t.Fatal("stale populated TF state incorrectly blocked actual empty first tier", result.Diagnostics)
			}
		})
	}
}

func TestGatewayGroupProtocolUnsupportedCreateHasNoIdentityOrMutation(t *testing.T) {
	for _, version := range []string{"26.7.6", "26.7.7-devel", "unknown"} {
		t.Run(version, func(t *testing.T) {
			api := &groupAPI{version: version}
			httpServer := httptest.NewServer(api)
			defer httpServer.Close()
			server, schemas := groupServer(t, httpServer.URL)
			schema := schemas.ResourceSchemas[groupType]
			response := groupApply(t, server, schema, groupNull(t, schema), groupValue(t, schema, groupValues()), nil)
			if !groupErrors(response.Diagnostics) || api.adds != 0 || api.saves != 0 || api.deletes != 0 || api.activations != 0 {
				t.Fatal("unsupported create mutated", response.Diagnostics)
			}
			value, err := response.NewState.Unmarshal(schema.ValueType())
			if err != nil || !value.IsNull() {
				t.Fatal("unsupported create manufactured identity", err)
			}
		})
	}
}

func TestGatewayGroupProtocolUnknownSaveRestartBindsAttempt(t *testing.T) {
	for _, mode := range []string{"match", "mismatch", "malformed", "missing"} {
		t.Run(mode, func(t *testing.T) {
			api := &groupAPI{}
			httpServer := httptest.NewServer(api)
			defer httpServer.Close()
			server, schemas := groupServer(t, httpServer.URL)
			schema := schemas.ResourceSchemas[groupType]
			values := groupValues()
			created := groupApply(t, server, schema, groupNull(t, schema), groupValue(t, schema, values), nil)
			if groupErrors(created.Diagnostics) {
				t.Fatal(created.Diagnostics)
			}
			api.ambiguousSave = true
			api.failReadAfterMutation = true
			values["description"] = "attempted changed description"
			failed := groupApply(t, server, schema, created.NewState, groupValue(t, schema, values), created.Private)
			if !groupErrors(failed.Diagnostics) || len(failed.Private) == 0 {
				t.Fatal("uncertain attempt not persisted", failed.Diagnostics)
			}
			api.failReadAfterMutation = false
			api.malformedRead = mode == "malformed"
			if mode == "mismatch" {
				api.payload["descr"] = "contradictory authoritative value"
			}
			if mode == "missing" {
				api.payload = nil
			}
			fresh, _ := groupServer(t, httpServer.URL)
			read, err := fresh.ReadResource(context.Background(), &tfprotov6.ReadResourceRequest{TypeName: groupType, CurrentState: failed.NewState, Private: failed.Private})
			if err != nil {
				t.Fatal(err)
			}
			if mode == "malformed" || mode == "missing" {
				if !groupErrors(read.Diagnostics) || api.activations != 1 || len(read.Private) == 0 {
					t.Fatal("malformed recovery lost uncertainty or activated", read.Diagnostics)
				}
				return
			}
			if groupErrors(read.Diagnostics) || api.saves != 1 {
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
			if err := groupAttrs(t, schema, read.NewState)["description"].As(&description); err != nil || description != api.payload["descr"] {
				t.Fatal("recovery did not retain authoritative values", err)
			}
		})
	}
}

func TestGatewayGroupProtocolDeletedMarkerReappearedUUIDRetained(t *testing.T) {
	for _, operation := range []string{"read", "delete"} {
		t.Run(operation, func(t *testing.T) {
			api := &groupAPI{}
			httpServer := httptest.NewServer(api)
			defer httpServer.Close()
			server, schemas := groupServer(t, httpServer.URL)
			schema := schemas.ResourceSchemas[groupType]
			created := groupApply(t, server, schema, groupNull(t, schema), groupValue(t, schema, groupValues()), nil)
			saved := api.payload
			api.failActivation = true
			failed := groupApply(t, server, schema, created.NewState, groupNull(t, schema), created.Private)
			if !groupErrors(failed.Diagnostics) {
				t.Fatal("delete activation must fail")
			}
			api.payload = saved
			api.failActivation = false
			fresh, _ := groupServer(t, httpServer.URL)
			var state *tfprotov6.DynamicValue
			if operation == "read" {
				response, err := fresh.ReadResource(context.Background(), &tfprotov6.ReadResourceRequest{TypeName: groupType, CurrentState: failed.NewState, Private: failed.Private})
				if err != nil || groupErrors(response.Diagnostics) {
					t.Fatal(err, response.Diagnostics)
				}
				state = response.NewState
			} else {
				response := groupApply(t, fresh, schema, failed.NewState, groupNull(t, schema), failed.Private)
				if !groupErrors(response.Diagnostics) {
					t.Fatal("reappeared delete must retain identity with retry diagnostic")
				}
				state = response.NewState
			}
			var id string
			if err := groupAttrs(t, schema, state)["id"].As(&id); err != nil || id != groupUUID || api.deletes != 1 || api.activations != 2 {
				t.Fatal("restored UUID was removed or activated", err, api.deletes, api.activations)
			}
		})
	}
}

func TestGatewayGroupProtocolLookupEveryPopulatedComputedField(t *testing.T) {
	api := &groupAPI{payload: map[string]string{"name": "synthetic_group", "item": "synthetic_a", "item2": "synthetic_b", "item3": "synthetic_c", "item4": "synthetic_a,synthetic_b", "item5": "synthetic_b,synthetic_c", "trigger": "downlatency", "poolopts": "round-robin sticky-address", "descr": "literal,ordered,description"}}
	httpServer := httptest.NewServer(api)
	defer httpServer.Close()
	server, schemas := groupServer(t, httpServer.URL)
	schema := schemas.DataSourceSchemas[groupType]
	response, err := server.ReadDataSource(context.Background(), &tfprotov6.ReadDataSourceRequest{TypeName: groupType, Config: groupValue(t, schema, map[string]any{"id": groupUUID})})
	if err != nil || groupErrors(response.Diagnostics) {
		t.Fatal(err, response.Diagnostics)
	}
	expected := map[string]any{"id": groupUUID, "name": "synthetic_group", "tier_1": []string{"synthetic_a"}, "tier_2": []string{"synthetic_b"}, "tier_3": []string{"synthetic_c"}, "tier_4": []string{"synthetic_a", "synthetic_b"}, "tier_5": []string{"synthetic_b", "synthetic_c"}, "trigger": "downlatency", "pool_options": "round-robin sticky-address", "description": "literal,ordered,description"}
	got, want := groupAttrs(t, schema, response.State), groupAttrs(t, schema, groupValue(t, schema, expected))
	for field, value := range want {
		if !got[field].Equal(value) {
			t.Fatalf("computed lookup field %s differs from authoritative fixture", field)
		}
	}
	if api.adds != 0 || api.saves != 0 || api.deletes != 0 || api.activations != 0 {
		t.Fatal("lookup mutated configuration")
	}
}
