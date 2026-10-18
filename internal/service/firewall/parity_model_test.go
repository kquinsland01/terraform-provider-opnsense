// Copyright (c) Matthew Mellor
// SPDX-License-Identifier: MPL-2.0

package firewall

import (
	"context"
	"encoding/json"
	"math"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	resourceschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestAliasDynamicInterfaceAndAdvancedFieldsRoundTrip(t *testing.T) {
	model := AliasResourceModel{Type: types.StringValue("dynipv6host"), Interface: types.StringValue("wan"), Stats: types.BoolValue(true), PathExpression: types.StringValue(".items[]"), IPProtocol: stringSetValue([]string{"IPv6", "IPv4"}), Content: stringSetValue([]string{"host.example"}), Categories: stringSetValue(nil)}
	request := model.toAPI(context.Background())
	if request.Interface != "wan" || request.Counters != "1" || request.PathExpression != ".items[]" || request.Proto != "IPv4,IPv6" {
		t.Fatalf("advanced alias request = %#v", request)
	}
	var response aliasAPIResponse
	if err := json.Unmarshal([]byte(`{"name":"dynamic","type":{"dynipv6host":{"selected":1}},"content":"host.example","description":"","enabled":"1","proto":{"IPv6":{"selected":1},"IPv4":{"selected":1}},"interface":{"wan":{"selected":1}},"categories":[],"updatefreq":"","path_expression":".items[]","counters":"1"}`), &response); err != nil {
		t.Fatal(err)
	}
	model.fromAPI(context.Background(), &response, "id")
	if model.Interface.ValueString() != "wan" || !model.Stats.ValueBool() || len(model.IPProtocol.Elements()) != 2 {
		t.Fatalf("advanced alias response = %#v", model)
	}
}

func TestFirewallResponseModelsUseFieldSpecificScalarDecoding(t *testing.T) {
	var alias aliasAPIResponse
	if err := json.Unmarshal([]byte(`{"content":"value,with,commas\n192.0.2.2","proto":"IPv4,IPv6","categories":"cat-b,cat-a"}`), &alias); err != nil {
		t.Fatal(err)
	}
	if got := []string(alias.Content); len(got) != 2 || got[1] != "value,with,commas" || strings.Join([]string(alias.Proto), ",") != "IPv4,IPv6" || strings.Join([]string(alias.Categories), ",") != "cat-a,cat-b" {
		t.Fatalf("alias decoding = content %v proto %v categories %v", alias.Content, alias.Proto, alias.Categories)
	}
	var filter filterRuleAPIResponse
	if err := json.Unmarshal([]byte(`{"interface":"wan,lan","icmptype":"echoreq,unreach","tcpflags1":"syn,ack","categories":"cat-b,cat-a"}`), &filter); err != nil {
		t.Fatal(err)
	}
	if strings.Join([]string(filter.Interface), ",") != "lan,wan" || strings.Join([]string(filter.TCPFlags), ",") != "ack,syn" {
		t.Fatalf("filter decoding = interface %v flags %v", filter.Interface, filter.TCPFlags)
	}
	var nat natPortForwardAPIResponse
	if err := json.Unmarshal([]byte(`{"interface":"wan,lan","categories":"cat-b,cat-a"}`), &nat); err != nil {
		t.Fatal(err)
	}
	if strings.Join([]string(nat.Interface), ",") != "lan,wan" || strings.Join([]string(nat.Categories), ",") != "cat-a,cat-b" {
		t.Fatalf("NAT decoding = interface %v categories %v", nat.Interface, nat.Categories)
	}
}

func TestFilterAdvancedFieldsRoundTrip(t *testing.T) {
	var response filterRuleAPIResponse
	if err := json.Unmarshal([]byte(`{"enabled":"1","sequence":"10","action":{"pass":{"selected":1}},"quick":"1","interface":{"wan":{"selected":1}},"interfacenot":"1","nosync":"1","allowopts":"1","direction":{"in":{"selected":1}},"ipprotocol":{"inet":{"selected":1}},"protocol":{"TCP":{"selected":1}},"icmptype":{"echoreq":{"selected":1}},"source_net":"any","source_port":"","source_not":"0","destination_net":"any","destination_port":"","destination_not":"0","gateway":[],"tcpflags1":{"syn":{"selected":1}},"tcpflags2":{"ack":{"selected":1}},"sched":{"schedule-a":{"selected":1}},"statetype":{"keep":{"selected":1}},"state-policy":{"if-bound":{"selected":1}},"statetimeout":"30","adaptivestart":"100","adaptiveend":"200","max":"50","max-src-nodes":"20","max-src-states":"10","max-src-conn":"5","max-src-conn-rate":"3","max-src-conn-rates":"60","overload":{"table-a":{"selected":1}},"nopfsync":"1","shaper1":{"shape-a":{"selected":1}},"shaper2":{"shape-b":{"selected":1}},"disablereplyto":"1","replyto":{"gateway-a":{"selected":1}},"prio":{"1":{"selected":1}},"set-prio":{"2":{"selected":1}},"set-prio-low":{"3":{"selected":1}},"tos":{"lowdelay":{"selected":1}},"tag":"tag-a","tagged":"tag-b","log":"0","description":"","categories":[]}`), &response); err != nil {
		t.Fatal(err)
	}
	var model FilterRuleResourceModel
	model.fromAPI(context.Background(), &response, "id")
	if model.Shaper.ValueString() != "shape-a" || model.ReverseShaper.ValueString() != "shape-b" || !model.InterfaceNot.ValueBool() || !model.NoXMLRPCSync.ValueBool() || !model.AllowOptions.ValueBool() || firstString(setStrings(context.Background(), model.ICMPType)) != "echoreq" || model.Schedule.ValueString() != "schedule-a" || model.OverloadTable.ValueString() != "table-a" || model.ReplyTo.ValueString() != "gateway-a" || model.MaxStates.ValueInt64() != 50 {
		t.Fatalf("advanced response lost fields: %#v", model)
	}
	request := model.toAPI(context.Background())
	if request.Shaper != "shape-a" || request.ReverseShaper != "shape-b" || request.InterfaceNot != "1" || request.NoXMLRPCSync != "1" || request.AllowOptions != "1" || request.ICMPType != "echoreq" || request.Schedule != "schedule-a" || request.OverloadTable != "table-a" || request.ReplyTo != "gateway-a" || request.MatchTOS != "lowdelay" || request.MaxStates != "50" {
		t.Fatalf("advanced request = %#v", request)
	}
}

func TestProtocolCasePlanModifierUsesCanonicalState(t *testing.T) {
	req := planmodifier.StringRequest{ConfigValue: types.StringValue("tcp"), StateValue: types.StringValue("TCP"), PlanValue: types.StringValue("tcp")}
	resp := planmodifier.StringResponse{PlanValue: req.PlanValue}
	protocolCasePlanModifier{}.PlanModifyString(context.Background(), req, &resp)
	if resp.PlanValue.ValueString() != "TCP" {
		t.Fatalf("plan=%q", resp.PlanValue.ValueString())
	}
	if filterOperationProtocol(types.StringValue("tcp"), types.StringValue("TCP")).ValueString() != "tcp" {
		t.Fatal("apply did not retain planned casing")
	}
}

func TestNumericSentinelRoundTrip(t *testing.T) {
	if got := intSentinelToAPI(types.Int64Value(-1)); got != "" {
		t.Fatalf("-1 encoded %q", got)
	}
	if got := apiIntToSentinel("").ValueInt64(); got != -1 {
		t.Fatalf("empty decoded %d", got)
	}
	if got := apiIntToSentinel("30").ValueInt64(); got != 30 {
		t.Fatalf("30 decoded %d", got)
	}
}

func TestAPINumericValidationRequiresSequenceAndIsDeterministic(t *testing.T) {
	if !validateFilterAPIResponse(&filterRuleAPIResponse{}).HasError() {
		t.Fatal("filter accepted empty sequence")
	}
	if !validateNATAPIResponse(&natPortForwardAPIResponse{}).HasError() {
		t.Fatal("NAT accepted empty sequence")
	}
	response := &filterRuleAPIResponse{Sequence: "0", StateTimeout: "-1", AdaptiveStart: "-1", AdaptiveEnd: "-1", MaxStates: "-1", MaxSourceNodes: "-1", MaxSourceStates: "-1", MaxSourceConnections: "-1", MaxNewConnections: "-1", MaxNewConnectionsSeconds: "-1", MatchPriority: "8", SetPriority: "8", SetPriorityLowDelay: "8"}
	diagnostics := validateFilterAPIResponse(response)
	var details string
	for _, diagnostic := range diagnostics {
		details += diagnostic.Detail() + "\n"
	}
	last := -1
	for _, field := range []string{"sequence", "statetimeout", "adaptivestart", "adaptiveend", "max value", "max-src-nodes", "max-src-states", "max-src-conn value", "max-src-conn-rate", "max-src-conn-rates", "prio", "set-prio value", "set-prio-low"} {
		index := strings.Index(details, field)
		if index <= last {
			t.Fatalf("numeric diagnostic order mismatch at %q: %s", field, details)
		}
		last = index
	}
}

func TestReviewRepairValidators(t *testing.T) {
	t.Run("non-finite alias interval", func(t *testing.T) {
		for _, value := range []float64{math.NaN(), math.Inf(1)} {
			m := AliasResourceModel{Type: types.StringValue("urltable"), UpdateFreq: types.StringValue(fmtFloat(value)), PathExpression: types.StringValue(""), Interface: types.StringValue(""), Proto: types.StringValue(""), IPProtocol: types.SetNull(types.StringType)}
			var resp resource.ValidateConfigResponse
			validateAliasModel(context.Background(), m, &resp)
			if !resp.Diagnostics.HasError() {
				t.Fatalf("accepted %v", value)
			}
		}
	})
	t.Run("legacy proto applicability", func(t *testing.T) {
		m := AliasResourceModel{Type: types.StringValue("host"), Proto: types.StringValue("IPv4"), IPProtocol: types.SetNull(types.StringType), UpdateFreq: types.StringValue(""), PathExpression: types.StringValue(""), Interface: types.StringValue("")}
		var resp resource.ValidateConfigResponse
		validateAliasModel(context.Background(), m, &resp)
		if !resp.Diagnostics.HasError() {
			t.Fatal("accepted inapplicable proto")
		}
	})
	t.Run("empty NAT member", func(t *testing.T) {
		m := NatPortForwardResourceModel{Interface: types.StringNull(), Interfaces: stringSetValue([]string{""})}
		var resp resource.ValidateConfigResponse
		validateNATModel(context.Background(), m, &resp)
		if !resp.Diagnostics.HasError() {
			t.Fatal("accepted empty interface")
		}
	})
	t.Run("malformed API numeric", func(t *testing.T) {
		if !validateFilterAPIResponse(&filterRuleAPIResponse{MaxStates: "bogus"}).HasError() {
			t.Fatal("malformed API numeric accepted")
		}
	})
	t.Run("alias protocol domains", func(t *testing.T) {
		for _, aliasType := range []string{"asn", "geoip", "external"} {
			m := AliasResourceModel{Type: types.StringValue(aliasType), Proto: types.StringValue("IPv4"), IPProtocol: types.SetNull(types.StringType), UpdateFreq: types.StringValue(""), PathExpression: types.StringValue(""), Interface: types.StringValue("")}
			var resp resource.ValidateConfigResponse
			validateAliasModel(context.Background(), m, &resp)
			if resp.Diagnostics.HasError() {
				t.Fatalf("%s rejected: %v", aliasType, resp.Diagnostics)
			}
		}
	})
	t.Run("empty canonical NAT set remains invalid with scalar", func(t *testing.T) {
		m := NatPortForwardResourceModel{Interface: types.StringValue("wan"), Interfaces: stringSetValue(nil)}
		var resp resource.ValidateConfigResponse
		validateNATModel(context.Background(), m, &resp)
		if !resp.Diagnostics.HasError() {
			t.Fatal("accepted explicitly empty interfaces")
		}
	})
	t.Run("exact filter domains", func(t *testing.T) {
		for _, protocol := range []string{"not-tcp", "sctp"} {
			m := filterFixture()
			m.Protocol = types.StringValue(protocol)
			m.StateTimeout = types.Int64Value(30)
			var resp resource.ValidateConfigResponse
			validateFilterModel(context.Background(), m, &resp)
			if !resp.Diagnostics.HasError() {
				t.Fatalf("accepted TCP-only state timeout for %q", protocol)
			}
		}
		m := filterFixture()
		m.Protocol = types.StringValue("udp")
		m.MaxSourceNodes = types.Int64Value(10)
		m.MaxSourceStates = types.Int64Value(10)
		var resp resource.ValidateConfigResponse
		validateFilterModel(context.Background(), m, &resp)
		if resp.Diagnostics.HasError() {
			t.Fatalf("rejected generic UDP limits: %v", resp.Diagnostics)
		}
		m.Protocol = types.StringValue("invented")
		resp = resource.ValidateConfigResponse{}
		validateFilterModel(context.Background(), m, &resp)
		if !resp.Diagnostics.HasError() {
			t.Fatal("accepted fabricated protocol")
		}
	})
	t.Run("TOS domain", func(t *testing.T) {
		if !validTOS("lowdelay") || !validTOS("0xff") || validTOS("0x100") || validTOS("bogus") {
			t.Fatal("TOS domain mismatch")
		}
	})
}

func fmtFloat(value float64) string {
	if math.IsNaN(value) {
		return "NaN"
	}
	return "+Inf"
}

func TestCompatibilityPlanningPreservesCanonicalState(t *testing.T) {
	aliasConfig := normalizeAlias(AliasResourceModel{Proto: types.StringValue("IPv6"), IPProtocol: types.SetNull(types.StringType)})
	aliasState := normalizeAlias(AliasResourceModel{Proto: types.StringValue("IPv4"), IPProtocol: stringSetValue([]string{"IPv4", "IPv6"})})
	aliasPlan := aliasConfig
	aliasPlan.IPProtocol = types.SetUnknown(types.StringType)
	gotAlias := runAliasModifyPlan(t, aliasConfig, aliasPlan, aliasState)
	if len(gotAlias.IPProtocol.Elements()) != 2 || gotAlias.Proto.ValueString() != "IPv6" {
		t.Fatalf("alias plan collapsed state: %#v", gotAlias)
	}

	natConfig := normalizeNAT(NatPortForwardResourceModel{Interface: types.StringValue("wan"), Interfaces: types.SetNull(types.StringType)})
	natState := normalizeNAT(NatPortForwardResourceModel{Interface: types.StringValue("opt1"), Interfaces: stringSetValue([]string{"opt1", "wan"})})
	natPlan := natConfig
	natPlan.Interfaces = types.SetUnknown(types.StringType)
	gotNAT := runNATModifyPlan(t, natConfig, natPlan, natState)
	if len(gotNAT.Interfaces.Elements()) != 2 || gotNAT.Interface.ValueString() != "wan" {
		t.Fatalf("NAT plan collapsed state: %#v", gotNAT)
	}
}

func TestCompatibilityPlanningPreservesUnknownCanonicalConfig(t *testing.T) {
	aliasConfig := normalizeAlias(AliasResourceModel{Proto: types.StringValue("IPv6"), IPProtocol: types.SetUnknown(types.StringType)})
	aliasState := normalizeAlias(AliasResourceModel{Proto: types.StringValue("IPv4"), IPProtocol: stringSetValue([]string{"IPv4", "IPv6"})})
	gotAlias := runAliasModifyPlan(t, aliasConfig, aliasConfig, aliasState)
	if !gotAlias.IPProtocol.IsUnknown() {
		t.Fatalf("alias unknown became %#v", gotAlias.IPProtocol)
	}
	natConfig := normalizeNAT(NatPortForwardResourceModel{Interface: types.StringValue("wan"), Interfaces: types.SetUnknown(types.StringType)})
	natState := normalizeNAT(NatPortForwardResourceModel{Interface: types.StringValue("opt1"), Interfaces: stringSetValue([]string{"opt1", "wan"})})
	gotNAT := runNATModifyPlan(t, natConfig, natConfig, natState)
	if !gotNAT.Interfaces.IsUnknown() {
		t.Fatalf("NAT unknown became %#v", gotNAT.Interfaces)
	}
	legacyUnknownConfig := normalizeAlias(AliasResourceModel{Proto: types.StringUnknown(), IPProtocol: types.SetNull(types.StringType)})
	legacyUnknownPlan := legacyUnknownConfig
	legacyUnknownPlan.IPProtocol = types.SetUnknown(types.StringType)
	gotLegacyUnknown := runAliasModifyPlan(t, legacyUnknownConfig, legacyUnknownPlan, aliasState)
	if !gotLegacyUnknown.IPProtocol.IsUnknown() {
		t.Fatalf("unknown legacy proto reused stale canonical state: %#v", gotLegacyUnknown.IPProtocol)
	}
}

func TestCompatibilityPlanningReplacesPriorSingletonOnLegacyChange(t *testing.T) {
	aliasConfig := normalizeAlias(AliasResourceModel{Proto: types.StringValue("IPv6"), IPProtocol: types.SetNull(types.StringType)})
	aliasState := normalizeAlias(AliasResourceModel{Proto: types.StringValue("IPv4"), IPProtocol: stringSetValue([]string{"IPv4"})})
	aliasPlan := aliasConfig
	aliasPlan.IPProtocol = types.SetUnknown(types.StringType)
	gotAlias := runAliasModifyPlan(t, aliasConfig, aliasPlan, aliasState)
	if values := setStrings(context.Background(), gotAlias.IPProtocol); len(values) != 1 || values[0] != "IPv6" {
		t.Fatalf("legacy alias change retained stale canonical state: %v", values)
	}

	natConfig := normalizeNAT(NatPortForwardResourceModel{Interface: types.StringValue("lan"), Interfaces: types.SetNull(types.StringType)})
	natState := normalizeNAT(NatPortForwardResourceModel{Interface: types.StringValue("wan"), Interfaces: stringSetValue([]string{"wan"})})
	natPlan := natConfig
	natPlan.Interfaces = types.SetUnknown(types.StringType)
	gotNAT := runNATModifyPlan(t, natConfig, natPlan, natState)
	if values := setStrings(context.Background(), gotNAT.Interfaces); len(values) != 1 || values[0] != "lan" {
		t.Fatalf("legacy NAT change retained stale canonical state: %v", values)
	}
}

func TestUnknownCanonicalSetsDoNotFallbackInRequests(t *testing.T) {
	alias := normalizeAlias(AliasResourceModel{Proto: types.StringValue("IPv4"), IPProtocol: types.SetUnknown(types.StringType)})
	if got := alias.toAPI(context.Background()).Proto; got != "" {
		t.Fatalf("unknown ip_protocol fell back to %q", got)
	}
	nat := normalizeNAT(NatPortForwardResourceModel{Interface: types.StringValue("wan"), Interfaces: types.SetUnknown(types.StringType)})
	if got := nat.toAPI(context.Background()).Interface; got != "" {
		t.Fatalf("unknown interfaces fell back to %q", got)
	}
}

func TestResourceConfigValidatorsAreRegisteredAndExecute(t *testing.T) {
	tests := []struct {
		name  string
		owner resource.Resource
		model any
	}{
		{name: "alias", owner: &aliasResource{}, model: normalizeAlias(AliasResourceModel{Type: types.StringValue("host"), Proto: types.StringValue("IPv4"), UpdateFreq: types.StringValue(""), PathExpression: types.StringValue(""), Interface: types.StringValue("")})},
		{name: "filter", owner: &filterRuleResource{}, model: func() FilterRuleResourceModel {
			m := filterFixture()
			m.Protocol = types.StringValue("invented")
			return m
		}()},
		{name: "NAT", owner: &natPortForwardResource{}, model: normalizeNAT(NatPortForwardResourceModel{Interface: types.StringValue("wan"), Interfaces: stringSetValue(nil)})},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			owner, ok := tt.owner.(resource.ResourceWithConfigValidators)
			if !ok || len(owner.ConfigValidators(context.Background())) == 0 {
				t.Fatal("resource has no config validators")
			}
			state := resourceState(t, tt.owner, tt.model)
			var resp resource.ValidateConfigResponse
			owner.ConfigValidators(context.Background())[0].ValidateResource(context.Background(), resource.ValidateConfigRequest{Config: tfsdk.Config(state)}, &resp)
			if !resp.Diagnostics.HasError() {
				t.Fatal("registered validator accepted invalid configuration")
			}
		})
	}
}

func TestRegisteredConditionalAndPairedValidators(t *testing.T) {
	aliasBase := func(aliasType string) AliasResourceModel {
		return normalizeAlias(AliasResourceModel{Name: types.StringValue("a"), Type: types.StringValue(aliasType), Proto: types.StringValue(""), Interface: types.StringValue(""), UpdateFreq: types.StringValue(""), PathExpression: types.StringValue("")})
	}
	filterBase := filterFixture

	missingConditional := aliasBase("urltable")
	inapplicableConditional := aliasBase("host")
	inapplicableConditional.PathExpression = types.StringValue(".addresses[]")
	oneSidedRate := filterBase()
	oneSidedRate.MaxNewConnections = types.Int64Value(10)
	oneSidedRate.MaxNewConnectionsSeconds = types.Int64Value(-1)
	oneSidedAdaptive := filterBase()
	oneSidedAdaptive.AdaptiveStart = types.Int64Value(100)
	oneSidedAdaptive.AdaptiveEnd = types.Int64Value(-1)
	reversedAdaptive := filterBase()
	reversedAdaptive.AdaptiveStart = types.Int64Value(200)
	reversedAdaptive.AdaptiveEnd = types.Int64Value(100)
	maxAboveEnd := filterBase()
	maxAboveEnd.MaxStates = types.Int64Value(201)
	maxAboveEnd.AdaptiveStart = types.Int64Value(100)
	maxAboveEnd.AdaptiveEnd = types.Int64Value(200)
	unknownDeferred := filterBase()
	unknownDeferred.MaxNewConnections = types.Int64Value(10)
	unknownDeferred.MaxNewConnectionsSeconds = types.Int64Unknown()
	unknownDeferred.AdaptiveStart = types.Int64Value(100)
	unknownDeferred.AdaptiveEnd = types.Int64Unknown()
	unknownDeferred.MaxStates = types.Int64Value(500)
	unknownNAT := normalizeNAT(NatPortForwardResourceModel{Interface: types.StringUnknown(), Interfaces: types.SetNull(types.StringType)})

	for _, tt := range []struct {
		name      string
		owner     resource.Resource
		model     any
		wantError bool
	}{
		{"missing conditional alias field", &aliasResource{}, missingConditional, true},
		{"inapplicable conditional alias field", &aliasResource{}, inapplicableConditional, true},
		{"one-sided rate", &filterRuleResource{}, oneSidedRate, true},
		{"one-sided adaptive range", &filterRuleResource{}, oneSidedAdaptive, true},
		{"reversed adaptive range", &filterRuleResource{}, reversedAdaptive, true},
		{"max states above adaptive end", &filterRuleResource{}, maxAboveEnd, true},
		{"unknown paired values defer", &filterRuleResource{}, unknownDeferred, false},
		{"unknown legacy NAT interface defers", &natPortForwardResource{}, unknownNAT, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			owner := tt.owner.(resource.ResourceWithConfigValidators)
			state := resourceState(t, tt.owner, tt.model)
			var resp resource.ValidateConfigResponse
			owner.ConfigValidators(context.Background())[0].ValidateResource(context.Background(), resource.ValidateConfigRequest{Config: tfsdk.Config(state)}, &resp)
			if resp.Diagnostics.HasError() != tt.wantError {
				t.Fatalf("diagnostics=%v wantError=%t", resp.Diagnostics, tt.wantError)
			}
		})
	}
}

func normalizeAlias(m AliasResourceModel) AliasResourceModel {
	if m.Content.ElementType(context.Background()) == nil {
		m.Content = types.SetNull(types.StringType)
	}
	if m.IPProtocol.ElementType(context.Background()) == nil {
		m.IPProtocol = types.SetNull(types.StringType)
	}
	if m.Categories.ElementType(context.Background()) == nil {
		m.Categories = types.SetNull(types.StringType)
	}
	return m
}

func normalizeNAT(m NatPortForwardResourceModel) NatPortForwardResourceModel {
	if m.Interfaces.ElementType(context.Background()) == nil {
		m.Interfaces = types.SetNull(types.StringType)
	}
	if m.Categories.ElementType(context.Background()) == nil {
		m.Categories = types.SetNull(types.StringType)
	}
	return m
}

func runAliasModifyPlan(t *testing.T, config, plan, state AliasResourceModel) AliasResourceModel {
	t.Helper()
	r := &aliasResource{}
	schema := resourceSchema(t, r)
	cs, ps, ss := tfsdk.State{Schema: schema}, tfsdk.State{Schema: schema}, tfsdk.State{Schema: schema}
	cs.Set(context.Background(), &config)
	ps.Set(context.Background(), &plan)
	ss.Set(context.Background(), &state)
	req := resource.ModifyPlanRequest{Config: tfsdk.Config(cs), Plan: tfsdk.Plan(ps), State: ss}
	resp := resource.ModifyPlanResponse{Plan: req.Plan}
	r.ModifyPlan(context.Background(), req, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatal(resp.Diagnostics)
	}
	var got AliasResourceModel
	resp.Plan.Get(context.Background(), &got)
	return got
}

func runNATModifyPlan(t *testing.T, config, plan, state NatPortForwardResourceModel) NatPortForwardResourceModel {
	t.Helper()
	r := &natPortForwardResource{}
	schema := resourceSchema(t, r)
	cs, ps, ss := tfsdk.State{Schema: schema}, tfsdk.State{Schema: schema}, tfsdk.State{Schema: schema}
	cs.Set(context.Background(), &config)
	ps.Set(context.Background(), &plan)
	ss.Set(context.Background(), &state)
	req := resource.ModifyPlanRequest{Config: tfsdk.Config(cs), Plan: tfsdk.Plan(ps), State: ss}
	resp := resource.ModifyPlanResponse{Plan: req.Plan}
	r.ModifyPlan(context.Background(), req, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatal(resp.Diagnostics)
	}
	var got NatPortForwardResourceModel
	resp.Plan.Get(context.Background(), &got)
	return got
}

func resourceSchema(t *testing.T, r resource.Resource) resourceschema.Schema {
	t.Helper()
	var resp resource.SchemaResponse
	r.Schema(context.Background(), resource.SchemaRequest{}, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatal(resp.Diagnostics)
	}
	return resp.Schema
}
