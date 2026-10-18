// Copyright (c) Matthew Mellor
// SPDX-License-Identifier: MPL-2.0

package firewall

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.ResourceWithConfigValidators = &aliasResource{}
	_ resource.ResourceWithConfigValidators = &filterRuleResource{}
	_ resource.ResourceWithConfigValidators = &natPortForwardResource{}
)

type resourceModelValidator[T any] struct {
	description string
	validate    func(context.Context, T, *resource.ValidateConfigResponse)
}

func (v resourceModelValidator[T]) Description(context.Context) string { return v.description }
func (v resourceModelValidator[T]) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (v resourceModelValidator[T]) ValidateResource(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var model T
	resp.Diagnostics.Append(req.Config.Get(ctx, &model)...)
	if !resp.Diagnostics.HasError() {
		v.validate(ctx, model, resp)
	}
}

func (r *aliasResource) ConfigValidators(context.Context) []resource.ConfigValidator {
	return []resource.ConfigValidator{resourceModelValidator[AliasResourceModel]{"validates alias conditional fields and protocol compatibility", validateAliasModel}}
}

func validateAliasModel(ctx context.Context, m AliasResourceModel, resp *resource.ValidateConfigResponse) {
	if m.Type.IsNull() || m.Type.IsUnknown() {
		return
	}
	typeName := m.Type.ValueString()
	require := func(value types.String, field, wantType string) {
		if typeName == wantType && !value.IsUnknown() && (value.IsNull() || value.ValueString() == "") {
			resp.Diagnostics.AddAttributeError(path.Root(field), "Missing Alias Field", fmt.Sprintf("%s is required for alias type %s.", field, wantType))
		}
		if typeName != wantType && !value.IsNull() && !value.IsUnknown() && value.ValueString() != "" {
			resp.Diagnostics.AddAttributeError(path.Root(field), "Inapplicable Alias Field", fmt.Sprintf("%s may be set only for alias type %s.", field, wantType))
		}
	}
	require(m.UpdateFreq, "update_freq", "urltable")
	require(m.PathExpression, "path_expression", "urljson")
	require(m.Interface, "interface", "dynipv6host")
	if typeName == "urltable" && !m.UpdateFreq.IsUnknown() && m.UpdateFreq.ValueString() != "" {
		value, err := strconv.ParseFloat(m.UpdateFreq.ValueString(), 64)
		if err != nil || math.IsNaN(value) || math.IsInf(value, 0) || value <= 0 {
			resp.Diagnostics.AddAttributeError(path.Root("update_freq"), "Invalid Alias Update Frequency", "update_freq must be a finite positive number of days.")
		}
	}
	protocolConfigured := (!m.Proto.IsNull() && !m.Proto.IsUnknown() && m.Proto.ValueString() != "") || (!m.IPProtocol.IsNull() && !m.IPProtocol.IsUnknown() && len(m.IPProtocol.Elements()) > 0)
	if protocolConfigured && typeName != "asn" && typeName != "geoip" && typeName != "external" {
		resp.Diagnostics.AddAttributeError(path.Root("ip_protocol"), "Inapplicable Alias Protocol", "proto and ip_protocol may be set only for ASN, GeoIP, or external aliases.")
	}
	if !m.Proto.IsNull() && !m.Proto.IsUnknown() && m.Proto.ValueString() != "" && !m.IPProtocol.IsNull() && !m.IPProtocol.IsUnknown() && !setContains(ctx, m.IPProtocol, m.Proto.ValueString()) {
		resp.Diagnostics.AddAttributeError(path.Root("proto"), "Conflicting Alias Protocols", "proto must be a member of ip_protocol when both are configured.")
	}
}

func (r *natPortForwardResource) ConfigValidators(context.Context) []resource.ConfigValidator {
	return []resource.ConfigValidator{resourceModelValidator[NatPortForwardResourceModel]{"validates NAT interface compatibility", validateNATModel}}
}

func validateNATModel(ctx context.Context, m NatPortForwardResourceModel, resp *resource.ValidateConfigResponse) {
	scalarUnknown := m.Interface.IsUnknown()
	scalar := !m.Interface.IsNull() && !m.Interface.IsUnknown() && strings.TrimSpace(m.Interface.ValueString()) != ""
	setKnown := !m.Interfaces.IsNull() && !m.Interfaces.IsUnknown()
	values := setStrings(ctx, m.Interfaces)
	if setKnown && len(values) == 0 {
		resp.Diagnostics.AddAttributeError(path.Root("interfaces"), "Missing NAT Interface", "interfaces must contain at least one non-empty interface.")
	}
	for _, value := range values {
		if strings.TrimSpace(value) == "" {
			resp.Diagnostics.AddAttributeError(path.Root("interfaces"), "Invalid NAT Interface", "interfaces must not contain empty values.")
		}
	}
	if !scalarUnknown && !scalar && (m.Interfaces.IsNull() || (setKnown && len(values) == 0)) {
		resp.Diagnostics.AddAttributeError(path.Root("interface"), "Missing NAT Interface", "configure interface or interfaces.")
	}
	if scalar && setKnown && len(values) > 0 && !setContains(ctx, m.Interfaces, m.Interface.ValueString()) {
		resp.Diagnostics.AddAttributeError(path.Root("interface"), "Conflicting NAT Interfaces", "interface must be a member of interfaces when both are configured.")
	}
}

func (r *filterRuleResource) ConfigValidators(context.Context) []resource.ConfigValidator {
	return []resource.ConfigValidator{resourceModelValidator[FilterRuleResourceModel]{"validates protocol-specific filter fields and state limits", validateFilterModel}}
}

func configuredSet(v types.Set) bool { return !v.IsNull() && !v.IsUnknown() && len(v.Elements()) > 0 }

func configuredInt(v types.Int64) bool { return !v.IsNull() && !v.IsUnknown() && v.ValueInt64() >= 0 }

func knownInt(v types.Int64) bool { return !v.IsUnknown() }

func validateFilterModel(_ context.Context, m FilterRuleResourceModel, resp *resource.ValidateConfigResponse) {
	protocol := strings.ToLower(m.Protocol.ValueString())
	protocolKnown := !m.Protocol.IsNull() && !m.Protocol.IsUnknown()
	if protocolKnown && !validFilterProtocol(protocol) {
		resp.Diagnostics.AddAttributeError(path.Root("protocol"), "Invalid Filter Protocol", fmt.Sprintf("protocol %q is not supported by OPNsense.", m.Protocol.ValueString()))
	}
	if protocolKnown && configuredSet(m.ICMPType) && protocol != "icmp" && protocol != "ipv6-icmp" {
		resp.Diagnostics.AddAttributeError(path.Root("icmp_type"), "Invalid ICMP Selector", "icmp_type requires an ICMP protocol.")
	}
	tcpOnly := configuredSet(m.TCPFlags) || configuredSet(m.TCPFlagsOutOf) || configuredInt(m.StateTimeout) || configuredInt(m.MaxSourceConnections) || configuredInt(m.MaxNewConnections) || configuredInt(m.MaxNewConnectionsSeconds)
	if protocolKnown && tcpOnly && protocol != "tcp" && protocol != "tcp/udp" {
		resp.Diagnostics.AddAttributeError(path.Root("protocol"), "Invalid TCP State Configuration", "TCP flags, state_timeout, and connection limits require TCP or TCP/UDP.")
	}
	rate, seconds := configuredInt(m.MaxNewConnections), configuredInt(m.MaxNewConnectionsSeconds)
	if knownInt(m.MaxNewConnections) && knownInt(m.MaxNewConnectionsSeconds) && rate != seconds {
		resp.Diagnostics.AddAttributeError(path.Root("max_new_connections"), "Incomplete Connection Rate", "max_new_connections and max_new_connections_seconds must be configured together.")
	}
	start, end := configuredInt(m.AdaptiveStart), configuredInt(m.AdaptiveEnd)
	if knownInt(m.AdaptiveStart) && knownInt(m.AdaptiveEnd) && (start != end || (start && m.AdaptiveStart.ValueInt64() > m.AdaptiveEnd.ValueInt64())) {
		resp.Diagnostics.AddAttributeError(path.Root("adaptive_timeout_start"), "Invalid Adaptive Timeout Range", "adaptive timeout start/end must be configured together and start must not exceed end.")
	}
	if knownInt(m.MaxStates) && knownInt(m.AdaptiveEnd) && configuredInt(m.MaxStates) && end && m.MaxStates.ValueInt64() > m.AdaptiveEnd.ValueInt64() {
		resp.Diagnostics.AddAttributeError(path.Root("adaptive_timeout_end"), "Invalid Adaptive Timeout End", "adaptive_timeout_end must not be lower than max_states.")
	}
	if !m.MatchTOS.IsNull() && !m.MatchTOS.IsUnknown() && !validTOS(m.MatchTOS.ValueString()) {
		resp.Diagnostics.AddAttributeError(path.Root("match_tos"), "Invalid Type of Service", "match_tos must be empty, a hexadecimal byte from 0x0 through 0xff, or a supported OPNsense TOS name.")
	}
}

func validFilterProtocol(value string) bool {
	for _, allowed := range []string{
		"3pc", "a/n", "ah", "argus", "aris", "ax.25", "bbn-rcc", "bna", "br-sat-mon", "carp", "cbt", "cftp", "chaos", "compaq-peer", "cphb", "cpnx", "crtp", "crudp", "dccp", "dcn", "ddp", "ddx", "dgp", "divert", "dsr", "egp", "eigrp", "emcon", "encap", "esp", "etherip", "fc", "ggp", "gmtp", "gre", "hip", "hmp", "i-nlsp", "iatp", "icmp", "idpr", "idpr-cmtp", "idrp", "ifmp", "igmp", "igp", "il", "ipcomp", "ipcv", "ipencap", "ipip", "ippc", "ipv6", "ipv6-icmp", "ipx-in-ip", "irtp", "isis", "iso-ip", "iso-tp4", "kryptolan", "l2tp", "larp", "leaf-1", "leaf-2", "manet", "merit-inp", "mfe-nsp", "micp", "mobile", "mpls-in-ip", "mtp", "mux", "narp", "netblt", "nsfnet-igp", "nvp", "ospf", "pfsync", "pgm", "pim", "pipe", "pnni", "prm", "ptp", "pup", "pvp", "qnx", "rdp", "rohc", "rsvp", "rsvp-e2e-ignore", "rvd", "sat-expak", "sat-mon", "scc-sp", "scps", "sctp", "sdrp", "secure-vmtp", "shim6", "skip", "sm", "smp", "snp", "sprite-rpc", "sps", "srp", "st2", "stp", "sun-nd", "swipe", "tcf", "tcp", "tcp/udp", "tlsp", "tp++", "trunk-1", "trunk-2", "ttp", "udp", "udplite", "uti", "vines", "visa", "vmtp", "wb-expak", "wb-mon", "wesp", "wsn", "xnet", "xns-idp", "xtp", "any",
	} {
		if value == allowed {
			return true
		}
	}
	return false
}

func validTOS(value string) bool {
	if value == "" {
		return true
	}
	for _, named := range []string{"af11", "af12", "af13", "af21", "af22", "af23", "af31", "af32", "af33", "af41", "af42", "af43", "critical", "cs0", "cs1", "cs2", "cs3", "cs4", "cs5", "cs6", "cs7", "ef", "inetcontrol", "lowdelay", "netcontrol", "reliability", "throughput"} {
		if value == named {
			return true
		}
	}
	if !strings.HasPrefix(value, "0x") || len(value) < 3 || len(value) > 4 {
		return false
	}
	parsed, err := strconv.ParseUint(value[2:], 16, 8)
	return err == nil && parsed <= 0xff
}
