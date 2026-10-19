// Copyright (c) Matthew Mellor
// SPDX-License-Identifier: MPL-2.0

package kea

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/setplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func (r *dhcpv4SubnetResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a Kea DHCPv4 subnet on OPNsense. The subnet's interface must be enabled in the Kea DHCPv4 general settings (`opnsense_kea_dhcpv4_settings`) first.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true, MarkdownDescription: "UUID of the subnet.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"subnet": schema.StringAttribute{
				Required: true, MarkdownDescription: "Subnet in CIDR notation (e.g. `192.168.40.0/24`).",
			},
			"allocator": schema.StringAttribute{
				Optional: true, Computed: true,
				Default:             stringdefault.StaticString(""),
				MarkdownDescription: "Lease allocator strategy (empty = default, `iterative`, `random`, or `flq`).",
			},
			"pools": schema.StringAttribute{
				Optional: true, Computed: true,
				Default:             stringdefault.StaticString(""),
				MarkdownDescription: "Address pools for the subnet (newline-separated ranges).",
			},
			"match_client_id": schema.BoolAttribute{
				Optional: true, Computed: true,
				Default:             booldefault.StaticBool(true),
				MarkdownDescription: "Match clients by client-id in addition to hardware address. Defaults to `true`.",
			},
			"ping_check": schema.BoolAttribute{
				Optional: true, Computed: true,
				Default:             booldefault.StaticBool(false),
				MarkdownDescription: "Ping the address before handing out a lease. Defaults to `false`.",
			},
			"auto_collect": schema.BoolAttribute{
				Optional: true, Computed: true,
				Default:             booldefault.StaticBool(true),
				MarkdownDescription: "Automatically collect router, DNS, and NTP option data from OPNsense. Set false to manage those values explicitly.",
			},
			"dns_servers": optionalComputedStringSet("IPv4 DNS servers offered to clients."),
			"domain_name": schema.StringAttribute{
				Optional: true, Computed: true,
				Default:             stringdefault.StaticString(""),
				MarkdownDescription: "Domain name offered to clients.",
			},
			"domain_search": optionalComputedStringSet("Domain-search suffixes offered to clients."),
			"routers":       optionalComputedStringSet("IPv4 default routers offered to clients."),
			"static_routes": schema.SetNestedAttribute{
				Optional: true, Computed: true,
				MarkdownDescription: "IPv4 destination/router pairs encoded by OPNsense as one flat comma-separated sequence.",
				PlanModifiers:       []planmodifier.Set{setplanmodifier.UseStateForUnknown()},
				NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
					"destination": schema.StringAttribute{Required: true, MarkdownDescription: "IPv4 route destination."},
					"router":      schema.StringAttribute{Required: true, MarkdownDescription: "IPv4 next-hop router."},
				}},
			},
			"ntp_servers":  optionalComputedStringSet("IPv4 NTP servers offered to clients."),
			"time_servers": optionalComputedStringSet("IPv4 RFC 868 time servers offered to clients."),
			"next_server": schema.StringAttribute{
				Optional: true, Computed: true,
				Default:             stringdefault.StaticString(""),
				MarkdownDescription: "IPv4 next-server address used for network boot.",
			},
			"tftp_server": schema.StringAttribute{
				Optional: true, Computed: true,
				Default:             stringdefault.StaticString(""),
				MarkdownDescription: "TFTP server name or address offered to clients.",
			},
			"tftp_bootfile": schema.StringAttribute{
				Optional: true, Computed: true,
				Default:             stringdefault.StaticString(""),
				MarkdownDescription: "Network boot filename offered to clients.",
			},
			"description": schema.StringAttribute{
				Optional: true, Computed: true,
				Default:             stringdefault.StaticString(""),
				MarkdownDescription: "Description of the subnet.",
			},
		},
	}
}

func optionalComputedStringSet(description string) schema.SetAttribute {
	return schema.SetAttribute{
		ElementType: types.StringType, Optional: true, Computed: true,
		MarkdownDescription: description,
		PlanModifiers:       []planmodifier.Set{setplanmodifier.UseStateForUnknown()},
	}
}
