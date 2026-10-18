// Copyright (c) Matthew Mellor
// SPDX-License-Identifier: MPL-2.0

package firewall

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	dsschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/matthew-on-git/terraform-provider-opnsense/pkg/opnsense"
)

var _ datasource.DataSource = &filterRuleDataSource{}

type filterRuleDataSource struct{ client *opnsense.Client }

var filterRuleDataSourceReqOpts = opnsense.ReqOpts{
	GetEndpoint:    "/api/firewall/filter/getRule",
	SearchEndpoint: "/api/firewall/filter/searchRule",
	Monad:          "rule",
}

func newFilterRuleDataSource() datasource.DataSource { return &filterRuleDataSource{} }

func (d *filterRuleDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_firewall_filter_rule"
}

func (d *filterRuleDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = dsschema.Schema{
		MarkdownDescription: "Reads an existing firewall filter rule on OPNsense by UUID.",
		Attributes: map[string]dsschema.Attribute{
			"id": dsschema.StringAttribute{
				Required:            true,
				MarkdownDescription: "UUID to look up.",
			},
			"enabled":                     dsschema.BoolAttribute{Computed: true, MarkdownDescription: "Whether this rule is enabled. Defaults to 'true'."},
			"sequence":                    dsschema.Int64Attribute{Computed: true, MarkdownDescription: "Rule sequence number (1-999999). Controls evaluation order."},
			"action":                      dsschema.StringAttribute{Computed: true, MarkdownDescription: "Rule action: 'pass', 'block', or 'reject'."},
			"quick":                       dsschema.BoolAttribute{Computed: true, MarkdownDescription: "First match wins. Defaults to 'true'."},
			"interface":                   dsschema.SetAttribute{ElementType: types.StringType, Computed: true, MarkdownDescription: "Network interfaces this rule applies to."},
			"interface_not":               dsschema.BoolAttribute{Computed: true, MarkdownDescription: "Whether the selected interfaces are inverted."},
			"no_xmlrpc_sync":              dsschema.BoolAttribute{Computed: true, MarkdownDescription: "Whether HA XMLRPC synchronization is disabled for this rule."},
			"allow_options":               dsschema.BoolAttribute{Computed: true, MarkdownDescription: "Whether packets carrying IP options are allowed."},
			"direction":                   dsschema.StringAttribute{Computed: true, MarkdownDescription: "Traffic direction: 'in', 'out', or 'any'."},
			"ip_protocol":                 dsschema.StringAttribute{Computed: true, MarkdownDescription: "IP version: 'inet' (IPv4), 'inet6' (IPv6), or 'inet46' (both)."},
			"protocol":                    dsschema.StringAttribute{Computed: true, MarkdownDescription: "Protocol (e.g., 'any', 'tcp', 'udp', 'icmp', 'TCP/UDP')."},
			"icmp_type":                   dsschema.SetAttribute{ElementType: types.StringType, Computed: true, MarkdownDescription: "ICMP message types matched by ICMP protocols."},
			"source_net":                  dsschema.StringAttribute{Computed: true, MarkdownDescription: "Source network ('any', interface name, CIDR, or alias)."},
			"source_port":                 dsschema.StringAttribute{Computed: true, MarkdownDescription: "Source port or range (e.g., '80', '80:443'). Empty for any."},
			"source_not":                  dsschema.BoolAttribute{Computed: true, MarkdownDescription: "Invert source match. Defaults to 'false'."},
			"destination_net":             dsschema.StringAttribute{Computed: true, MarkdownDescription: "Destination network ('any', interface name, CIDR, or alias)."},
			"destination_port":            dsschema.StringAttribute{Computed: true, MarkdownDescription: "Destination port or range (e.g., '443', '8080:8090'). Empty for any."},
			"destination_not":             dsschema.BoolAttribute{Computed: true, MarkdownDescription: "Invert destination match. Defaults to 'false'."},
			"gateway":                     dsschema.StringAttribute{Computed: true, MarkdownDescription: "Gateway for policy routing. Empty for default."},
			"tcp_flags":                   dsschema.SetAttribute{ElementType: types.StringType, Computed: true, MarkdownDescription: "TCP flags that must be set; applicable to TCP-compatible protocols."},
			"tcp_flags_out_of":            dsschema.SetAttribute{ElementType: types.StringType, Computed: true, MarkdownDescription: "TCP flags considered by the match; applicable to TCP-compatible protocols."},
			"schedule":                    dsschema.StringAttribute{Computed: true, MarkdownDescription: "Opaque OPNsense schedule identifier."},
			"state_type":                  dsschema.StringAttribute{Computed: true, MarkdownDescription: "State tracking mechanism."},
			"state_policy":                dsschema.StringAttribute{Computed: true, MarkdownDescription: "State policy, such as floating or interface-bound."},
			"state_timeout":               dsschema.Int64Attribute{Computed: true, MarkdownDescription: "TCP state timeout in seconds; -1 means unset."},
			"adaptive_timeout_start":      dsschema.Int64Attribute{Computed: true, MarkdownDescription: "State count where adaptive timeouts begin; -1 means unset."},
			"adaptive_timeout_end":        dsschema.Int64Attribute{Computed: true, MarkdownDescription: "State count where adaptive timeouts reach zero; -1 means unset."},
			"max_states":                  dsschema.Int64Attribute{Computed: true, MarkdownDescription: "Maximum concurrent states; -1 means unset."},
			"max_source_nodes":            dsschema.Int64Attribute{Computed: true, MarkdownDescription: "Maximum source nodes; -1 means unset."},
			"max_source_states":           dsschema.Int64Attribute{Computed: true, MarkdownDescription: "Maximum states per source; -1 means unset."},
			"max_source_connections":      dsschema.Int64Attribute{Computed: true, MarkdownDescription: "Maximum established TCP connections per source; -1 means unset."},
			"max_new_connections":         dsschema.Int64Attribute{Computed: true, MarkdownDescription: "Maximum new TCP connections per interval; -1 means unset."},
			"max_new_connections_seconds": dsschema.Int64Attribute{Computed: true, MarkdownDescription: "Connection-rate interval in seconds; -1 means unset."},
			"overload_table":              dsschema.StringAttribute{Computed: true, MarkdownDescription: "Opaque overload-table identifier."},
			"no_pfsync":                   dsschema.BoolAttribute{Computed: true, MarkdownDescription: "Whether state synchronization through pfsync is disabled."},
			"shaper":                      dsschema.StringAttribute{Computed: true, MarkdownDescription: "Opaque traffic-shaper identifier."},
			"reverse_shaper":              dsschema.StringAttribute{Computed: true, MarkdownDescription: "Opaque reverse-traffic-shaper identifier."},
			"disable_reply_to":            dsschema.BoolAttribute{Computed: true, MarkdownDescription: "Whether automatic reply-to is disabled."},
			"reply_to":                    dsschema.StringAttribute{Computed: true, MarkdownDescription: "Opaque reply-to gateway identifier."},
			"match_priority":              dsschema.Int64Attribute{Computed: true, MarkdownDescription: "Queue priority to match; -1 means unset."},
			"set_priority":                dsschema.Int64Attribute{Computed: true, MarkdownDescription: "Queue priority to assign; -1 means unset."},
			"set_priority_low_delay":      dsschema.Int64Attribute{Computed: true, MarkdownDescription: "Low-delay queue priority to assign; -1 means unset."},
			"match_tos":                   dsschema.StringAttribute{Computed: true, MarkdownDescription: "Type-of-service value to match."},
			"set_local_tag":               dsschema.StringAttribute{Computed: true, MarkdownDescription: "Local packet tag assigned by this rule."},
			"match_local_tag":             dsschema.StringAttribute{Computed: true, MarkdownDescription: "Local packet tag required by this rule."},
			"log":                         dsschema.BoolAttribute{Computed: true, MarkdownDescription: "Log matching packets. Defaults to 'false'."},
			"description":                 dsschema.StringAttribute{Computed: true, MarkdownDescription: "Description of the rule."},
			"categories":                  dsschema.SetAttribute{ElementType: types.StringType, Computed: true, MarkdownDescription: "Set of category UUIDs assigned to this rule."},
		},
	}
}

func (d *filterRuleDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	client, ok := req.ProviderData.(*opnsense.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected Provider Data", "Expected *opnsense.Client.")
		return
	}
	d.client = client
}

func (d *filterRuleDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config FilterRuleResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}
	id := config.ID.ValueString()
	result, err := opnsense.Get[filterRuleAPIResponse](ctx, d.client, filterRuleDataSourceReqOpts, id)
	if err != nil {
		resp.Diagnostics.AddError("Error reading firewall filter rule", fmt.Sprintf("Could not read firewall filter rule %s: %s", id, err))
		return
	}
	resp.Diagnostics.Append(validateFilterAPIResponse(result)...)
	if resp.Diagnostics.HasError() {
		return
	}
	config.fromAPI(ctx, result, id)
	resp.Diagnostics.Append(resp.State.Set(ctx, &config)...)
}
