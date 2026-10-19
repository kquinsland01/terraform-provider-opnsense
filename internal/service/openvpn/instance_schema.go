// Copyright (c) Matthew Mellor
// SPDX-License-Identifier: MPL-2.0

package openvpn

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func (r *instanceResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages an OpenVPN instance (server or client) on OPNsense using the modern instances API.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true, MarkdownDescription: "UUID of the OpenVPN instance.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"vpnid": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Numeric VPN instance id, auto-assigned by OPNsense.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"enabled": schema.BoolAttribute{
				Optional: true, Computed: true, Default: booldefault.StaticBool(true),
				MarkdownDescription: "Whether the instance is enabled. Defaults to `true`.",
			},
			"role": schema.StringAttribute{
				Required: true, MarkdownDescription: "Instance role: `server` or `client`.",
				Validators: []validator.String{stringvalidator.OneOf("server", "client")},
			},
			"description": schema.StringAttribute{
				Optional: true, Computed: true, Default: stringdefault.StaticString(""),
				MarkdownDescription: "Description.",
			},
			"dev_type": schema.StringAttribute{
				Optional: true, Computed: true, Default: stringdefault.StaticString("tun"),
				MarkdownDescription: "Device type: `tun`, `tap`, or `ovpn`. Defaults to `tun`.",
				Validators:          []validator.String{stringvalidator.OneOf("tun", "tap", "ovpn")},
			},
			"protocol": schema.StringAttribute{
				Optional: true, Computed: true, Default: stringdefault.StaticString("udp"),
				MarkdownDescription: "Protocol: `udp`, `udp4`, `udp6`, `tcp`, `tcp4`, `tcp6`. Defaults to `udp`.",
				Validators:          []validator.String{stringvalidator.OneOf("udp", "udp4", "udp6", "tcp", "tcp4", "tcp6")},
			},
			"port": schema.StringAttribute{
				Optional: true, Computed: true, Default: stringdefault.StaticString(""),
				MarkdownDescription: "Listen/connect port.",
			},
			"port_share": schema.StringAttribute{Optional: true, Computed: true, MarkdownDescription: "Share the OpenVPN port with another service using `host:port`."},
			"local": schema.StringAttribute{
				Optional: true, Computed: true, Default: stringdefault.StaticString(""),
				MarkdownDescription: "Local interface address to bind.",
			},
			"remote": schema.StringAttribute{
				Optional: true, Computed: true, Default: stringdefault.StaticString(""),
				MarkdownDescription: "Remote server address (client role).",
			},
			"server": schema.StringAttribute{
				Optional: true, Computed: true, Default: stringdefault.StaticString(""),
				MarkdownDescription: "IPv4 tunnel network (CIDR) for server role.",
			},
			"server_ipv6": schema.StringAttribute{Optional: true, Computed: true, MarkdownDescription: "IPv6 tunnel network in CIDR notation."},
			"topology": schema.StringAttribute{
				Optional: true, Computed: true, Default: stringdefault.StaticString("subnet"),
				MarkdownDescription: "Topology: `subnet`, `p2p`, or `net30`. Defaults to `subnet`.",
				Validators:          []validator.String{stringvalidator.OneOf("subnet", "p2p", "net30")},
			},
			"no_pool":        schema.BoolAttribute{Optional: true, Computed: true, MarkdownDescription: "Disable the dynamic address pool."},
			"bridge_gateway": schema.StringAttribute{Optional: true, Computed: true, MarkdownDescription: "Bridge gateway address for TAP mode."},
			"bridge_pool":    schema.StringAttribute{Optional: true, Computed: true, MarkdownDescription: "Bridge pool range for TAP mode."},
			"route":          schema.SetAttribute{ElementType: types.StringType, Optional: true, Computed: true, MarkdownDescription: "Local routes installed by OpenVPN."},
			"ca": schema.StringAttribute{
				Optional: true, Computed: true, Default: stringdefault.StaticString(""),
				MarkdownDescription: "Certificate Authority reference (UUID/refid).",
			},
			"cert": schema.StringAttribute{
				Optional: true, Computed: true, Default: stringdefault.StaticString(""),
				MarkdownDescription: "Server/client certificate reference (UUID/refid).",
			},
			"crl":             schema.StringAttribute{Optional: true, Computed: true, MarkdownDescription: "Certificate revocation list reference."},
			"cert_depth":      schema.StringAttribute{Optional: true, Computed: true, MarkdownDescription: "Maximum certificate chain depth.", Validators: []validator.String{stringvalidator.OneOf("", "1", "2", "3", "4", "5")}},
			"remote_cert_tls": schema.BoolAttribute{Optional: true, Computed: true, MarkdownDescription: "Require the peer certificate extended key usage."},
			"verify_client_cert": schema.StringAttribute{
				Optional: true, Computed: true, Default: stringdefault.StaticString("require"),
				MarkdownDescription: "Client certificate verification: `require` or `none`. Defaults to `require`.",
				Validators:          []validator.String{stringvalidator.OneOf("require", "none")},
			},
			"tls_key": schema.StringAttribute{
				Optional: true, Computed: true, Default: stringdefault.StaticString(""),
				MarkdownDescription: "TLS static key reference (UUID of an opnsense_openvpn_static_key).",
			},
			"use_ocsp": schema.BoolAttribute{Optional: true, Computed: true, MarkdownDescription: "Verify peer certificates using OCSP."},
			"data_ciphers": schema.SetAttribute{
				ElementType: types.StringType, Optional: true, Computed: true,
				MarkdownDescription: "Allowed data ciphers (e.g. `AES-256-GCM`).",
			},
			"data_ciphers_ordered":  schema.ListAttribute{ElementType: types.StringType, Optional: true, Computed: true, MarkdownDescription: "Priority-ordered data ciphers. When both cipher attributes are configured they must contain identical values."},
			"data_ciphers_fallback": schema.StringAttribute{Optional: true, Computed: true, MarkdownDescription: "Fallback data cipher for legacy clients."},
			"auth": schema.StringAttribute{
				Optional: true, Computed: true, Default: stringdefault.StaticString(""),
				MarkdownDescription: "Auth digest algorithm (e.g. `SHA256`).",
			},
			"auth_mode":               schema.SetAttribute{ElementType: types.StringType, Optional: true, Computed: true, MarkdownDescription: "Authentication backend identifiers."},
			"local_group":             schema.StringAttribute{Optional: true, Computed: true, MarkdownDescription: "Required local authentication group."},
			"various_flags":           schema.SetAttribute{ElementType: types.StringType, Optional: true, Computed: true, MarkdownDescription: "Additional OpenVPN instance flags."},
			"various_push_flags":      schema.SetAttribute{ElementType: types.StringType, Optional: true, Computed: true, MarkdownDescription: "Additional flags pushed to clients."},
			"push_inactive":           schema.Int64Attribute{Optional: true, Computed: true, MarkdownDescription: "Disconnect clients after this many inactive seconds.", Validators: []validator.Int64{int64validator.AtLeast(0)}},
			"username_as_common_name": schema.BoolAttribute{Optional: true, Computed: true, MarkdownDescription: "Use the authenticated username as common name."},
			"strict_user_cn":          schema.StringAttribute{Optional: true, Computed: true, MarkdownDescription: "Username/common-name matching mode.", Validators: []validator.String{stringvalidator.OneOf("0", "1", "2")}},
			"username":                schema.StringAttribute{Optional: true, Computed: true, MarkdownDescription: "Client-mode username."},
			"password":                schema.StringAttribute{Optional: true, Computed: true, Sensitive: true, MarkdownDescription: "Client-mode password. The prior configured value is preserved when OPNsense omits it on read."},
			"dns_servers": schema.SetAttribute{
				ElementType: types.StringType, Optional: true, Computed: true,
				MarkdownDescription: "DNS servers pushed to clients.",
			},
			"push_route": schema.SetAttribute{
				ElementType: types.StringType, Optional: true, Computed: true,
				MarkdownDescription: "Networks (CIDR) pushed as routes to clients.",
			},
			"redirect_gateway": schema.SetAttribute{
				ElementType: types.StringType, Optional: true, Computed: true,
				MarkdownDescription: "redirect-gateway flags (e.g. `def1`, `bypass-dhcp`).",
			},
			"max_clients": schema.Int64Attribute{
				Optional: true, Computed: true, Default: int64default.StaticInt64(0),
				MarkdownDescription: "Maximum number of connected clients (0 = unset).",
			},
			"keepalive_interval": schema.Int64Attribute{
				Optional: true, Computed: true, Default: int64default.StaticInt64(0),
				MarkdownDescription: "Keepalive ping interval in seconds (0 = unset).",
			},
			"keepalive_timeout": schema.Int64Attribute{
				Optional: true, Computed: true, Default: int64default.StaticInt64(0),
				MarkdownDescription: "Keepalive timeout in seconds (0 = unset).",
			},
			"reneg_sec":              schema.Int64Attribute{Optional: true, Computed: true, MarkdownDescription: "Data-channel key renegotiation interval.", Validators: []validator.Int64{int64validator.AtLeast(0)}},
			"auth_gen_token":         schema.Int64Attribute{Optional: true, Computed: true, MarkdownDescription: "Generated authentication token lifetime.", Validators: []validator.Int64{int64validator.AtLeast(0)}},
			"auth_gen_token_renewal": schema.Int64Attribute{Optional: true, Computed: true, MarkdownDescription: "Generated authentication token renewal interval.", Validators: []validator.Int64{int64validator.AtLeast(0)}},
			"auth_gen_token_secret":  schema.StringAttribute{Optional: true, Computed: true, Sensitive: true, MarkdownDescription: "Secret used to sign generated authentication tokens."},
			"provision_exclusive":    schema.BoolAttribute{Optional: true, Computed: true, MarkdownDescription: "Allow only the newest session for a common name."},
			"push_excluded_routes":   schema.SetAttribute{ElementType: types.StringType, Optional: true, Computed: true, MarkdownDescription: "Routes excluded from client route pushes."},
			"route_metric":           schema.Int64Attribute{Optional: true, Computed: true, MarkdownDescription: "Installed route metric.", Validators: []validator.Int64{int64validator.Between(0, 65535)}},
			"register_dns":           schema.BoolAttribute{Optional: true, Computed: true, MarkdownDescription: "Request Windows clients to register DNS."},
			"dns_domain":             schema.SetAttribute{ElementType: types.StringType, Optional: true, Computed: true, MarkdownDescription: "DNS domains pushed to clients."},
			"dns_domain_search":      schema.SetAttribute{ElementType: types.StringType, Optional: true, Computed: true, MarkdownDescription: "DNS search domains pushed to clients."},
			"verb": schema.StringAttribute{
				Optional: true, Computed: true,
				MarkdownDescription: "Log verbosity level (0-11). Defaults to the OPNsense value when unset.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"ntp_servers":           schema.SetAttribute{ElementType: types.StringType, Optional: true, Computed: true, MarkdownDescription: "NTP servers pushed to clients."},
			"tun_mtu":               schema.Int64Attribute{Optional: true, Computed: true, MarkdownDescription: "Tunnel MTU.", Validators: []validator.Int64{int64validator.Between(60, 65535)}},
			"fragment":              schema.Int64Attribute{Optional: true, Computed: true, MarkdownDescription: "Internal datagram fragmentation size.", Validators: []validator.Int64{int64validator.Between(0, 65528)}},
			"mss_fix":               schema.BoolAttribute{Optional: true, Computed: true, MarkdownDescription: "Clamp TCP MSS to the tunnel MTU."},
			"carp_depend_on":        schema.StringAttribute{Optional: true, Computed: true, MarkdownDescription: "CARP virtual IP this instance depends on."},
			"compress_migrate":      schema.BoolAttribute{Optional: true, Computed: true, MarkdownDescription: "Enable legacy compression migration."},
			"ifconfig_pool_persist": schema.BoolAttribute{Optional: true, Computed: true, MarkdownDescription: "Persist client address assignments."},
			"http_proxy":            schema.StringAttribute{Optional: true, Computed: true, MarkdownDescription: "Client HTTP proxy in `host:port` form."},
			"verify_x509_name":      schema.StringAttribute{Optional: true, Computed: true, MarkdownDescription: "Expected peer certificate X.509 name."},
		},
	}
}
