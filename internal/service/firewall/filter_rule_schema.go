// Copyright (c) Matthew Mellor
// SPDX-License-Identifier: MPL-2.0

package firewall

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/setvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/setplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Schema defines the Terraform schema for opnsense_firewall_filter_rule.
func (r *filterRuleResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a firewall filter rule on OPNsense.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "UUID of the firewall filter rule in OPNsense.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"enabled": schema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(true),
				MarkdownDescription: "Whether this rule is enabled. Defaults to `true`.",
			},
			"sequence": schema.Int64Attribute{
				Optional:            true,
				Computed:            true,
				Default:             int64default.StaticInt64(1),
				MarkdownDescription: "Rule sequence number (1-999999). Controls evaluation order.",
				Validators: []validator.Int64{
					int64validator.Between(1, 999999),
				},
			},
			"action": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Rule action: `pass`, `block`, or `reject`.",
				Validators: []validator.String{
					stringvalidator.OneOf("pass", "block", "reject"),
				},
			},
			"quick": schema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(true),
				MarkdownDescription: "First match wins. Defaults to `true`.",
			},
			"interface": schema.SetAttribute{
				ElementType:         types.StringType,
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Network interfaces this rule applies to.",
			},
			"interface_not":  boolAttr(false, "Invert selected interfaces."),
			"no_xmlrpc_sync": boolAttr(false, "Exclude this rule from HA XMLRPC synchronization."),
			"allow_options":  boolAttr(false, "Allow packets with IP options."),
			"direction": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString("in"),
				MarkdownDescription: "Traffic direction: `in`, `out`, or `any`.",
				Validators: []validator.String{
					stringvalidator.OneOf("in", "out", "any"),
				},
			},
			"ip_protocol": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString("inet"),
				MarkdownDescription: "IP version: `inet` (IPv4), `inet6` (IPv6), or `inet46` (both).",
				Validators: []validator.String{
					stringvalidator.OneOf("inet", "inet6", "inet46"),
				},
			},
			"protocol": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString("any"),
				MarkdownDescription: "Protocol (e.g., `any`, `tcp`, `udp`, `icmp`, `TCP/UDP`).",
				PlanModifiers:       []planmodifier.String{protocolCasePlanModifier{}},
			},
			"icmp_type": validatedSetAttr("ICMP message types.", "echoreq", "echorep", "unreach", "squench", "redir", "althost", "routeradv", "routersol", "timex", "paramprob", "timereq", "timerep", "inforeq", "inforep", "maskreq", "maskrep"),
			"source_net": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString("any"),
				MarkdownDescription: "Source network (`any`, interface name, CIDR, or alias).",
			},
			"source_port": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString(""),
				MarkdownDescription: "Source port or range (e.g., `80`, `80:443`). Empty for any.",
			},
			"source_not": schema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(false),
				MarkdownDescription: "Invert source match. Defaults to `false`.",
			},
			"destination_net": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString("any"),
				MarkdownDescription: "Destination network (`any`, interface name, CIDR, or alias).",
			},
			"destination_port": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString(""),
				MarkdownDescription: "Destination port or range (e.g., `443`, `8080:8090`). Empty for any.",
			},
			"destination_not": schema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(false),
				MarkdownDescription: "Invert destination match. Defaults to `false`.",
			},
			"gateway": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString(""),
				MarkdownDescription: "Gateway for policy routing. Empty for default.",
			},
			"tcp_flags":                   validatedSetAttr("TCP flags that must be set.", "fin", "syn", "rst", "psh", "ack", "urg", "ece", "cwr"),
			"tcp_flags_out_of":            validatedSetAttr("TCP flags considered when matching.", "fin", "syn", "rst", "psh", "ack", "urg", "ece", "cwr"),
			"schedule":                    stringAttr("", "Opaque OPNsense schedule identifier."),
			"state_type":                  validatedStringAttr("keep", "State tracking mechanism.", "keep", "modulate", "none", "sloppy", "synproxy"),
			"state_policy":                validatedStringAttr("", "State policy.", "", "floating", "if-bound"),
			"state_timeout":               sentinelIntAttr("State timeout in seconds."),
			"adaptive_timeout_start":      sentinelIntAttr("State count where adaptive timeouts begin."),
			"adaptive_timeout_end":        sentinelIntAttr("State count where adaptive timeouts reach zero."),
			"max_states":                  sentinelIntAttr("Maximum concurrent states."),
			"max_source_nodes":            sentinelIntAttr("Maximum source nodes."),
			"max_source_states":           sentinelIntAttr("Maximum states per source."),
			"max_source_connections":      sentinelIntAttr("Maximum established connections per source."),
			"max_new_connections":         sentinelIntAttr("Maximum new connections per interval."),
			"max_new_connections_seconds": sentinelIntAttr("New connection interval seconds."),
			"overload_table":              stringAttr("", "Opaque overload table identifier."),
			"no_pfsync":                   boolAttr(false, "Prevent state synchronization by pfsync."),
			"shaper":                      stringAttr("", "Traffic shaper identifier."),
			"reverse_shaper":              stringAttr("", "Reverse traffic shaper identifier."),
			"disable_reply_to":            boolAttr(false, "Disable reply-to."),
			"reply_to":                    stringAttr("", "Opaque reply-to gateway identifier."),
			"match_priority":              priorityAttr("Priority to match."),
			"set_priority":                priorityAttr("Priority to assign."),
			"set_priority_low_delay":      priorityAttr("Low-delay priority to assign."),
			"match_tos":                   stringAttr("", "Type-of-service value to match."),
			"set_local_tag":               stringAttr("", "Local packet tag to set."),
			"match_local_tag":             stringAttr("", "Local packet tag required."),
			"log": schema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(false),
				MarkdownDescription: "Log matching packets. Defaults to `false`.",
			},
			"description": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString(""),
				MarkdownDescription: "Description of the rule.",
			},
			"categories": schema.SetAttribute{
				ElementType:         types.StringType,
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Set of category UUIDs assigned to this rule.",
				PlanModifiers:       []planmodifier.Set{setplanmodifier.UseStateForUnknown()},
			},
		},
	}
}

func boolAttr(value bool, description string) schema.BoolAttribute {
	return schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(value), MarkdownDescription: description}
}

func stringAttr(value, description string) schema.StringAttribute {
	return schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString(value), MarkdownDescription: description}
}

func validatedStringAttr(value, description string, values ...string) schema.StringAttribute {
	a := stringAttr(value, description)
	a.Validators = []validator.String{stringvalidator.OneOf(values...)}
	return a
}

func sentinelIntAttr(description string) schema.Int64Attribute {
	return schema.Int64Attribute{Optional: true, Computed: true, Default: int64default.StaticInt64(-1), MarkdownDescription: description, Validators: []validator.Int64{int64validator.AtLeast(-1)}}
}

func priorityAttr(description string) schema.Int64Attribute {
	return schema.Int64Attribute{Optional: true, Computed: true, Default: int64default.StaticInt64(-1), MarkdownDescription: description, Validators: []validator.Int64{int64validator.Between(-1, 7)}}
}

func validatedSetAttr(description string, values ...string) schema.SetAttribute {
	return schema.SetAttribute{ElementType: types.StringType, Optional: true, Computed: true, MarkdownDescription: description, Validators: []validator.Set{setvalidator.ValueStringsAre(stringvalidator.OneOf(values...))}, PlanModifiers: []planmodifier.Set{setplanmodifier.UseStateForUnknown()}}
}
