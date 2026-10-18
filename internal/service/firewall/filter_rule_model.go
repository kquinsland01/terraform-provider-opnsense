// Copyright (c) Matthew Mellor
// SPDX-License-Identifier: MPL-2.0

package firewall

import (
	"context"
	"sort"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/matthew-on-git/terraform-provider-opnsense/pkg/opnsense"
)

// FilterRuleResourceModel is the Terraform state model for opnsense_firewall_filter_rule.
type FilterRuleResourceModel struct {
	ID                       types.String `tfsdk:"id"`
	Enabled                  types.Bool   `tfsdk:"enabled"`
	Sequence                 types.Int64  `tfsdk:"sequence"`
	Action                   types.String `tfsdk:"action"`
	Quick                    types.Bool   `tfsdk:"quick"`
	Interface                types.Set    `tfsdk:"interface"`
	InterfaceNot             types.Bool   `tfsdk:"interface_not"`
	NoXMLRPCSync             types.Bool   `tfsdk:"no_xmlrpc_sync"`
	AllowOptions             types.Bool   `tfsdk:"allow_options"`
	Direction                types.String `tfsdk:"direction"`
	IPProtocol               types.String `tfsdk:"ip_protocol"`
	Protocol                 types.String `tfsdk:"protocol"`
	ICMPType                 types.Set    `tfsdk:"icmp_type"`
	SourceNet                types.String `tfsdk:"source_net"`
	SourcePort               types.String `tfsdk:"source_port"`
	SourceNot                types.Bool   `tfsdk:"source_not"`
	DestinationNet           types.String `tfsdk:"destination_net"`
	DestinationPort          types.String `tfsdk:"destination_port"`
	DestinationNot           types.Bool   `tfsdk:"destination_not"`
	Gateway                  types.String `tfsdk:"gateway"`
	TCPFlags                 types.Set    `tfsdk:"tcp_flags"`
	TCPFlagsOutOf            types.Set    `tfsdk:"tcp_flags_out_of"`
	Schedule                 types.String `tfsdk:"schedule"`
	StateType                types.String `tfsdk:"state_type"`
	StatePolicy              types.String `tfsdk:"state_policy"`
	StateTimeout             types.Int64  `tfsdk:"state_timeout"`
	AdaptiveStart            types.Int64  `tfsdk:"adaptive_timeout_start"`
	AdaptiveEnd              types.Int64  `tfsdk:"adaptive_timeout_end"`
	MaxStates                types.Int64  `tfsdk:"max_states"`
	MaxSourceNodes           types.Int64  `tfsdk:"max_source_nodes"`
	MaxSourceStates          types.Int64  `tfsdk:"max_source_states"`
	MaxSourceConnections     types.Int64  `tfsdk:"max_source_connections"`
	MaxNewConnections        types.Int64  `tfsdk:"max_new_connections"`
	MaxNewConnectionsSeconds types.Int64  `tfsdk:"max_new_connections_seconds"`
	OverloadTable            types.String `tfsdk:"overload_table"`
	NoPFSync                 types.Bool   `tfsdk:"no_pfsync"`
	Shaper                   types.String `tfsdk:"shaper"`
	ReverseShaper            types.String `tfsdk:"reverse_shaper"`
	DisableReplyTo           types.Bool   `tfsdk:"disable_reply_to"`
	ReplyTo                  types.String `tfsdk:"reply_to"`
	MatchPriority            types.Int64  `tfsdk:"match_priority"`
	SetPriority              types.Int64  `tfsdk:"set_priority"`
	SetPriorityLowDelay      types.Int64  `tfsdk:"set_priority_low_delay"`
	MatchTOS                 types.String `tfsdk:"match_tos"`
	SetLocalTag              types.String `tfsdk:"set_local_tag"`
	MatchLocalTag            types.String `tfsdk:"match_local_tag"`
	Log                      types.Bool   `tfsdk:"log"`
	Description              types.String `tfsdk:"description"`
	Categories               types.Set    `tfsdk:"categories"`
}

// filterRuleAPIResponse is the struct for unmarshaling OPNsense GET responses.
type filterRuleAPIResponse struct {
	Enabled                  string                      `json:"enabled"`
	Sequence                 string                      `json:"sequence"`
	Action                   opnsense.SelectedMap        `json:"action"`
	Quick                    string                      `json:"quick"`
	Interface                opnsense.CSVSelectedMapList `json:"interface"`
	InterfaceNot             string                      `json:"interfacenot"`
	NoXMLRPCSync             string                      `json:"nosync"`
	AllowOptions             string                      `json:"allowopts"`
	Direction                opnsense.SelectedMap        `json:"direction"`
	IPProtocol               opnsense.SelectedMap        `json:"ipprotocol"`
	Protocol                 opnsense.SelectedMap        `json:"protocol"`
	ICMPType                 opnsense.CSVSelectedMapList `json:"icmptype"`
	SourceNet                string                      `json:"source_net"`
	SourcePort               string                      `json:"source_port"`
	SourceNot                string                      `json:"source_not"`
	DestinationNet           string                      `json:"destination_net"`
	DestinationPort          string                      `json:"destination_port"`
	DestinationNot           string                      `json:"destination_not"`
	Gateway                  opnsense.SelectedMap        `json:"gateway"`
	TCPFlags                 opnsense.CSVSelectedMapList `json:"tcpflags1"`
	TCPFlagsOutOf            opnsense.CSVSelectedMapList `json:"tcpflags2"`
	Schedule                 opnsense.SelectedMap        `json:"sched"`
	StateType                opnsense.SelectedMap        `json:"statetype"`
	StatePolicy              opnsense.SelectedMap        `json:"state-policy"`
	StateTimeout             string                      `json:"statetimeout"`
	AdaptiveStart            string                      `json:"adaptivestart"`
	AdaptiveEnd              string                      `json:"adaptiveend"`
	MaxStates                string                      `json:"max"`
	MaxSourceNodes           string                      `json:"max-src-nodes"`
	MaxSourceStates          string                      `json:"max-src-states"`
	MaxSourceConnections     string                      `json:"max-src-conn"`
	MaxNewConnections        string                      `json:"max-src-conn-rate"`
	MaxNewConnectionsSeconds string                      `json:"max-src-conn-rates"`
	OverloadTable            opnsense.SelectedMap        `json:"overload"`
	NoPFSync                 string                      `json:"nopfsync"`
	Shaper                   opnsense.SelectedMap        `json:"shaper1"`
	ReverseShaper            opnsense.SelectedMap        `json:"shaper2"`
	DisableReplyTo           string                      `json:"disablereplyto"`
	ReplyTo                  opnsense.SelectedMap        `json:"replyto"`
	MatchPriority            opnsense.SelectedMap        `json:"prio"`
	SetPriority              opnsense.SelectedMap        `json:"set-prio"`
	SetPriorityLowDelay      opnsense.SelectedMap        `json:"set-prio-low"`
	MatchTOS                 opnsense.SelectedMap        `json:"tos"`
	SetLocalTag              string                      `json:"tag"`
	MatchLocalTag            string                      `json:"tagged"`
	Log                      string                      `json:"log"`
	Description              string                      `json:"description"`
	Categories               opnsense.CSVSelectedMapList `json:"categories"`
}

// filterRuleAPIRequest is the struct for marshaling OPNsense POST requests.
type filterRuleAPIRequest struct {
	Enabled                  string `json:"enabled"`
	Sequence                 string `json:"sequence"`
	Action                   string `json:"action"`
	Quick                    string `json:"quick"`
	Interface                string `json:"interface"`
	InterfaceNot             string `json:"interfacenot"`
	NoXMLRPCSync             string `json:"nosync"`
	AllowOptions             string `json:"allowopts"`
	Direction                string `json:"direction"`
	IPProtocol               string `json:"ipprotocol"`
	Protocol                 string `json:"protocol"`
	ICMPType                 string `json:"icmptype"`
	SourceNet                string `json:"source_net"`
	SourcePort               string `json:"source_port"`
	SourceNot                string `json:"source_not"`
	DestinationNet           string `json:"destination_net"`
	DestinationPort          string `json:"destination_port"`
	DestinationNot           string `json:"destination_not"`
	Gateway                  string `json:"gateway"`
	TCPFlags                 string `json:"tcpflags1"`
	TCPFlagsOutOf            string `json:"tcpflags2"`
	Schedule                 string `json:"sched"`
	StateType                string `json:"statetype"`
	StatePolicy              string `json:"state-policy"`
	StateTimeout             string `json:"statetimeout"`
	AdaptiveStart            string `json:"adaptivestart"`
	AdaptiveEnd              string `json:"adaptiveend"`
	MaxStates                string `json:"max"`
	MaxSourceNodes           string `json:"max-src-nodes"`
	MaxSourceStates          string `json:"max-src-states"`
	MaxSourceConnections     string `json:"max-src-conn"`
	MaxNewConnections        string `json:"max-src-conn-rate"`
	MaxNewConnectionsSeconds string `json:"max-src-conn-rates"`
	OverloadTable            string `json:"overload"`
	NoPFSync                 string `json:"nopfsync"`
	Shaper                   string `json:"shaper1"`
	ReverseShaper            string `json:"shaper2"`
	DisableReplyTo           string `json:"disablereplyto"`
	ReplyTo                  string `json:"replyto"`
	MatchPriority            string `json:"prio"`
	SetPriority              string `json:"set-prio"`
	SetPriorityLowDelay      string `json:"set-prio-low"`
	MatchTOS                 string `json:"tos"`
	SetLocalTag              string `json:"tag"`
	MatchLocalTag            string `json:"tagged"`
	Log                      string `json:"log"`
	Description              string `json:"description"`
	Categories               string `json:"categories"`
}

// toAPI converts the Terraform model to an API request struct.
func (m *FilterRuleResourceModel) toAPI(ctx context.Context) *filterRuleAPIRequest {
	var interfaceStr string
	if !m.Interface.IsNull() && !m.Interface.IsUnknown() {
		var elements []string
		m.Interface.ElementsAs(ctx, &elements, false)
		sort.Strings(elements)
		interfaceStr = strings.Join(elements, ",")
	}

	var categoriesStr string
	if !m.Categories.IsNull() && !m.Categories.IsUnknown() {
		var elements []string
		m.Categories.ElementsAs(ctx, &elements, false)
		sort.Strings(elements)
		categoriesStr = strings.Join(elements, ",")
	}
	setCSV := func(value types.Set) string { return strings.Join(setStrings(ctx, value), ",") }

	return &filterRuleAPIRequest{
		Enabled:                  opnsense.BoolToString(m.Enabled.ValueBool()),
		Sequence:                 opnsense.Int64ToString(m.Sequence.ValueInt64()),
		Action:                   m.Action.ValueString(),
		Quick:                    opnsense.BoolToString(m.Quick.ValueBool()),
		Interface:                interfaceStr,
		InterfaceNot:             opnsense.BoolToString(m.InterfaceNot.ValueBool()),
		NoXMLRPCSync:             opnsense.BoolToString(m.NoXMLRPCSync.ValueBool()),
		AllowOptions:             opnsense.BoolToString(m.AllowOptions.ValueBool()),
		Direction:                m.Direction.ValueString(),
		IPProtocol:               m.IPProtocol.ValueString(),
		Protocol:                 m.Protocol.ValueString(),
		ICMPType:                 setCSV(m.ICMPType),
		SourceNet:                m.SourceNet.ValueString(),
		SourcePort:               m.SourcePort.ValueString(),
		SourceNot:                opnsense.BoolToString(m.SourceNot.ValueBool()),
		DestinationNet:           m.DestinationNet.ValueString(),
		DestinationPort:          m.DestinationPort.ValueString(),
		DestinationNot:           opnsense.BoolToString(m.DestinationNot.ValueBool()),
		Gateway:                  m.Gateway.ValueString(),
		TCPFlags:                 setCSV(m.TCPFlags),
		TCPFlagsOutOf:            setCSV(m.TCPFlagsOutOf),
		Schedule:                 m.Schedule.ValueString(),
		StateType:                m.StateType.ValueString(),
		StatePolicy:              m.StatePolicy.ValueString(),
		StateTimeout:             intSentinelToAPI(m.StateTimeout),
		AdaptiveStart:            intSentinelToAPI(m.AdaptiveStart),
		AdaptiveEnd:              intSentinelToAPI(m.AdaptiveEnd),
		MaxStates:                intSentinelToAPI(m.MaxStates),
		MaxSourceNodes:           intSentinelToAPI(m.MaxSourceNodes),
		MaxSourceStates:          intSentinelToAPI(m.MaxSourceStates),
		MaxSourceConnections:     intSentinelToAPI(m.MaxSourceConnections),
		MaxNewConnections:        intSentinelToAPI(m.MaxNewConnections),
		MaxNewConnectionsSeconds: intSentinelToAPI(m.MaxNewConnectionsSeconds),
		OverloadTable:            m.OverloadTable.ValueString(),
		NoPFSync:                 opnsense.BoolToString(m.NoPFSync.ValueBool()),
		Shaper:                   m.Shaper.ValueString(),
		ReverseShaper:            m.ReverseShaper.ValueString(),
		DisableReplyTo:           opnsense.BoolToString(m.DisableReplyTo.ValueBool()),
		ReplyTo:                  m.ReplyTo.ValueString(),
		MatchPriority:            intSentinelToAPI(m.MatchPriority),
		SetPriority:              intSentinelToAPI(m.SetPriority),
		SetPriorityLowDelay:      intSentinelToAPI(m.SetPriorityLowDelay),
		MatchTOS:                 m.MatchTOS.ValueString(),
		SetLocalTag:              m.SetLocalTag.ValueString(),
		MatchLocalTag:            m.MatchLocalTag.ValueString(),
		Log:                      opnsense.BoolToString(m.Log.ValueBool()),
		Description:              m.Description.ValueString(),
		Categories:               categoriesStr,
	}
}

// fromAPI populates the Terraform model from an API response struct.
func (m *FilterRuleResourceModel) fromAPI(_ context.Context, a *filterRuleAPIResponse, uuid string) {
	m.ID = types.StringValue(uuid)
	m.Enabled = types.BoolValue(opnsense.StringToBool(a.Enabled))
	m.Action = types.StringValue(string(a.Action))
	m.Quick = types.BoolValue(opnsense.StringToBool(a.Quick))
	m.Direction = types.StringValue(string(a.Direction))
	m.IPProtocol = types.StringValue(string(a.IPProtocol))
	m.Protocol = types.StringValue(string(a.Protocol))
	m.InterfaceNot = types.BoolValue(opnsense.StringToBool(a.InterfaceNot))
	m.NoXMLRPCSync = types.BoolValue(opnsense.StringToBool(a.NoXMLRPCSync))
	m.AllowOptions = types.BoolValue(opnsense.StringToBool(a.AllowOptions))
	m.ICMPType = stringSetValue([]string(a.ICMPType))
	m.SourceNet = types.StringValue(a.SourceNet)
	m.SourcePort = types.StringValue(a.SourcePort)
	m.SourceNot = types.BoolValue(opnsense.StringToBool(a.SourceNot))
	m.DestinationNet = types.StringValue(a.DestinationNet)
	m.DestinationPort = types.StringValue(a.DestinationPort)
	m.DestinationNot = types.BoolValue(opnsense.StringToBool(a.DestinationNot))
	m.Gateway = types.StringValue(string(a.Gateway))
	m.TCPFlags = stringSetValue([]string(a.TCPFlags))
	m.TCPFlagsOutOf = stringSetValue([]string(a.TCPFlagsOutOf))
	m.Schedule = types.StringValue(string(a.Schedule))
	m.StateType = types.StringValue(string(a.StateType))
	m.StatePolicy = types.StringValue(string(a.StatePolicy))
	m.StateTimeout = apiIntToSentinel(a.StateTimeout)
	m.AdaptiveStart = apiIntToSentinel(a.AdaptiveStart)
	m.AdaptiveEnd = apiIntToSentinel(a.AdaptiveEnd)
	m.MaxStates = apiIntToSentinel(a.MaxStates)
	m.MaxSourceNodes = apiIntToSentinel(a.MaxSourceNodes)
	m.MaxSourceStates = apiIntToSentinel(a.MaxSourceStates)
	m.MaxSourceConnections = apiIntToSentinel(a.MaxSourceConnections)
	m.MaxNewConnections = apiIntToSentinel(a.MaxNewConnections)
	m.MaxNewConnectionsSeconds = apiIntToSentinel(a.MaxNewConnectionsSeconds)
	m.OverloadTable = types.StringValue(string(a.OverloadTable))
	m.NoPFSync = types.BoolValue(opnsense.StringToBool(a.NoPFSync))
	m.Shaper = types.StringValue(string(a.Shaper))
	m.ReverseShaper = types.StringValue(string(a.ReverseShaper))
	m.DisableReplyTo = types.BoolValue(opnsense.StringToBool(a.DisableReplyTo))
	m.ReplyTo = types.StringValue(string(a.ReplyTo))
	m.MatchPriority = apiIntToSentinel(string(a.MatchPriority))
	m.SetPriority = apiIntToSentinel(string(a.SetPriority))
	m.SetPriorityLowDelay = apiIntToSentinel(string(a.SetPriorityLowDelay))
	m.MatchTOS = types.StringValue(string(a.MatchTOS))
	m.SetLocalTag = types.StringValue(a.SetLocalTag)
	m.MatchLocalTag = types.StringValue(a.MatchLocalTag)
	m.Log = types.BoolValue(opnsense.StringToBool(a.Log))
	m.Description = types.StringValue(a.Description)

	// Sequence (required — always has a value).
	if a.Sequence != "" {
		seqVal, err := opnsense.StringToInt64(a.Sequence)
		if err == nil {
			m.Sequence = types.Int64Value(seqVal)
		}
	}

	// Interface — SelectedMapList → types.Set.
	if len(a.Interface) == 0 {
		m.Interface = types.SetValueMust(types.StringType, []attr.Value{})
	} else {
		vals := make([]attr.Value, len(a.Interface))
		for i, v := range a.Interface {
			vals[i] = types.StringValue(v)
		}
		m.Interface = types.SetValueMust(types.StringType, vals)
	}

	// Categories — SelectedMapList → types.Set.
	if len(a.Categories) == 0 {
		m.Categories = types.SetValueMust(types.StringType, []attr.Value{})
	} else {
		vals := make([]attr.Value, len(a.Categories))
		for i, v := range a.Categories {
			vals[i] = types.StringValue(v)
		}
		m.Categories = types.SetValueMust(types.StringType, vals)
	}
}

func intSentinelToAPI(value types.Int64) string {
	if value.IsNull() || value.IsUnknown() || value.ValueInt64() < 0 {
		return ""
	}
	return opnsense.Int64ToString(value.ValueInt64())
}

func apiIntToSentinel(value string) types.Int64 {
	if value == "" {
		return types.Int64Value(-1)
	}
	parsed, err := opnsense.StringToInt64(value)
	if err != nil {
		return types.Int64Value(-1)
	}
	return types.Int64Value(parsed)
}
