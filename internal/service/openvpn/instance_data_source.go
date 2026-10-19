// Copyright (c) Matthew Mellor
// SPDX-License-Identifier: MPL-2.0

package openvpn

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	dsschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/matthew-on-git/terraform-provider-opnsense/pkg/opnsense"
)

var _ datasource.DataSource = &instanceDataSource{}

type instanceDataSource struct{ client *opnsense.Client }

func newInstanceDataSource() datasource.DataSource { return &instanceDataSource{} }

func (d *instanceDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_openvpn_instance"
}

func (d *instanceDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = dsschema.Schema{
		MarkdownDescription: "Reads an existing OpenVPN instance on OPNsense by UUID.",
		Attributes: map[string]dsschema.Attribute{
			"id": dsschema.StringAttribute{
				Required:            true,
				MarkdownDescription: "UUID to look up.",
			},
			"vpnid":                   dsschema.StringAttribute{Computed: true, MarkdownDescription: "Numeric VPN instance id, auto-assigned by OPNsense."},
			"enabled":                 dsschema.BoolAttribute{Computed: true, MarkdownDescription: "Whether the instance is enabled. Defaults to 'true'."},
			"role":                    dsschema.StringAttribute{Computed: true, MarkdownDescription: "Instance role: 'server' or 'client'."},
			"description":             dsschema.StringAttribute{Computed: true, MarkdownDescription: "Description."},
			"dev_type":                dsschema.StringAttribute{Computed: true, MarkdownDescription: "Device type: 'tun', 'tap', or 'ovpn'. Defaults to 'tun'."},
			"protocol":                dsschema.StringAttribute{Computed: true, MarkdownDescription: "Protocol: 'udp', 'udp4', 'udp6', 'tcp', 'tcp4', 'tcp6'. Defaults to 'udp'."},
			"port":                    dsschema.StringAttribute{Computed: true, MarkdownDescription: "Listen/connect port."},
			"port_share":              dsschema.StringAttribute{Computed: true, MarkdownDescription: "Shared service host and port."},
			"local":                   dsschema.StringAttribute{Computed: true, MarkdownDescription: "Local interface address to bind."},
			"remote":                  dsschema.StringAttribute{Computed: true, MarkdownDescription: "Remote server address (client role)."},
			"server":                  dsschema.StringAttribute{Computed: true, MarkdownDescription: "IPv4 tunnel network (CIDR) for server role."},
			"server_ipv6":             dsschema.StringAttribute{Computed: true, MarkdownDescription: "IPv6 tunnel network."},
			"topology":                dsschema.StringAttribute{Computed: true, MarkdownDescription: "Topology: 'subnet', 'p2p', or 'net30'. Defaults to 'subnet'."},
			"no_pool":                 dsschema.BoolAttribute{Computed: true, MarkdownDescription: "Whether the dynamic pool is disabled."},
			"bridge_gateway":          dsschema.StringAttribute{Computed: true, MarkdownDescription: "TAP bridge gateway."},
			"bridge_pool":             dsschema.StringAttribute{Computed: true, MarkdownDescription: "TAP bridge pool."},
			"route":                   dsschema.SetAttribute{ElementType: types.StringType, Computed: true, MarkdownDescription: "Installed routes."},
			"ca":                      dsschema.StringAttribute{Computed: true, MarkdownDescription: "Certificate Authority reference (UUID/refid)."},
			"cert":                    dsschema.StringAttribute{Computed: true, MarkdownDescription: "Server/client certificate reference (UUID/refid)."},
			"crl":                     dsschema.StringAttribute{Computed: true, MarkdownDescription: "Certificate revocation list reference."},
			"cert_depth":              dsschema.StringAttribute{Computed: true, MarkdownDescription: "Certificate chain depth."},
			"remote_cert_tls":         dsschema.BoolAttribute{Computed: true, MarkdownDescription: "Whether peer extended key usage is required."},
			"verify_client_cert":      dsschema.StringAttribute{Computed: true, MarkdownDescription: "Client certificate verification: 'require' or 'none'. Defaults to 'require'."},
			"tls_key":                 dsschema.StringAttribute{Computed: true, MarkdownDescription: "TLS static key reference (UUID of an opnsense_openvpn_static_key)."},
			"use_ocsp":                dsschema.BoolAttribute{Computed: true, MarkdownDescription: "Whether OCSP verification is enabled."},
			"data_ciphers":            dsschema.SetAttribute{ElementType: types.StringType, Computed: true, MarkdownDescription: "Allowed data ciphers (e.g. 'AES-256-GCM')."},
			"data_ciphers_ordered":    dsschema.ListAttribute{ElementType: types.StringType, Computed: true, MarkdownDescription: "Allowed data ciphers in priority order."},
			"data_ciphers_fallback":   dsschema.StringAttribute{Computed: true, MarkdownDescription: "Legacy data cipher fallback."},
			"auth":                    dsschema.StringAttribute{Computed: true, MarkdownDescription: "Auth digest algorithm (e.g. 'SHA256')."},
			"auth_mode":               dsschema.SetAttribute{ElementType: types.StringType, Computed: true, MarkdownDescription: "Authentication backends."},
			"local_group":             dsschema.StringAttribute{Computed: true, MarkdownDescription: "Required local group."},
			"various_flags":           dsschema.SetAttribute{ElementType: types.StringType, Computed: true, MarkdownDescription: "Additional instance flags."},
			"various_push_flags":      dsschema.SetAttribute{ElementType: types.StringType, Computed: true, MarkdownDescription: "Additional pushed flags."},
			"push_inactive":           dsschema.Int64Attribute{Computed: true, MarkdownDescription: "Inactive disconnect interval."},
			"username_as_common_name": dsschema.BoolAttribute{Computed: true, MarkdownDescription: "Whether username is used as common name."},
			"strict_user_cn":          dsschema.StringAttribute{Computed: true, MarkdownDescription: "Username/common-name comparison mode."},
			"username":                dsschema.StringAttribute{Computed: true, MarkdownDescription: "Client username."},
			"password":                dsschema.StringAttribute{Computed: true, Sensitive: true, MarkdownDescription: "Client password. Sensitive data-source values remain in Terraform state."},
			"dns_servers":             dsschema.SetAttribute{ElementType: types.StringType, Computed: true, MarkdownDescription: "DNS servers pushed to clients."},
			"push_route":              dsschema.SetAttribute{ElementType: types.StringType, Computed: true, MarkdownDescription: "Networks (CIDR) pushed as routes to clients."},
			"redirect_gateway":        dsschema.SetAttribute{ElementType: types.StringType, Computed: true, MarkdownDescription: "redirect-gateway flags (e.g. 'def1', 'bypass-dhcp')."},
			"max_clients":             dsschema.Int64Attribute{Computed: true, MarkdownDescription: "Maximum number of connected clients (0 = unset)."},
			"keepalive_interval":      dsschema.Int64Attribute{Computed: true, MarkdownDescription: "Keepalive ping interval in seconds (0 = unset)."},
			"keepalive_timeout":       dsschema.Int64Attribute{Computed: true, MarkdownDescription: "Keepalive timeout in seconds (0 = unset)."},
			"reneg_sec":               dsschema.Int64Attribute{Computed: true, MarkdownDescription: "Key renegotiation interval."},
			"auth_gen_token":          dsschema.Int64Attribute{Computed: true, MarkdownDescription: "Authentication token lifetime."},
			"auth_gen_token_renewal":  dsschema.Int64Attribute{Computed: true, MarkdownDescription: "Authentication token renewal interval."},
			"auth_gen_token_secret":   dsschema.StringAttribute{Computed: true, Sensitive: true, MarkdownDescription: "Authentication token secret. Sensitive data-source values remain in Terraform state."},
			"provision_exclusive":     dsschema.BoolAttribute{Computed: true, MarkdownDescription: "Whether sessions are exclusive."},
			"push_excluded_routes":    dsschema.SetAttribute{ElementType: types.StringType, Computed: true, MarkdownDescription: "Excluded pushed routes."},
			"route_metric":            dsschema.Int64Attribute{Computed: true, MarkdownDescription: "Route metric."},
			"register_dns":            dsschema.BoolAttribute{Computed: true, MarkdownDescription: "Whether register-dns is pushed."},
			"dns_domain":              dsschema.SetAttribute{ElementType: types.StringType, Computed: true, MarkdownDescription: "Pushed DNS domains."},
			"dns_domain_search":       dsschema.SetAttribute{ElementType: types.StringType, Computed: true, MarkdownDescription: "Pushed DNS search domains."},
			"verb":                    dsschema.StringAttribute{Computed: true, MarkdownDescription: "Log verbosity level (0-11). Defaults to the OPNsense value when unset."},
			"ntp_servers":             dsschema.SetAttribute{ElementType: types.StringType, Computed: true, MarkdownDescription: "Pushed NTP servers."},
			"tun_mtu":                 dsschema.Int64Attribute{Computed: true, MarkdownDescription: "Tunnel MTU."},
			"fragment":                dsschema.Int64Attribute{Computed: true, MarkdownDescription: "Fragment size."},
			"mss_fix":                 dsschema.BoolAttribute{Computed: true, MarkdownDescription: "Whether MSS clamping is enabled."},
			"carp_depend_on":          dsschema.StringAttribute{Computed: true, MarkdownDescription: "CARP dependency."},
			"compress_migrate":        dsschema.BoolAttribute{Computed: true, MarkdownDescription: "Whether compression migration is enabled."},
			"ifconfig_pool_persist":   dsschema.BoolAttribute{Computed: true, MarkdownDescription: "Whether address assignments persist."},
			"http_proxy":              dsschema.StringAttribute{Computed: true, MarkdownDescription: "HTTP proxy."},
			"verify_x509_name":        dsschema.StringAttribute{Computed: true, MarkdownDescription: "Expected peer X.509 name."},
		},
	}
}

func (d *instanceDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *instanceDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config InstanceResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}
	id := config.ID.ValueString()
	supported, _, err := openVPNParitySupport(ctx, d.client)
	if err != nil {
		resp.Diagnostics.AddError("Unable to determine OpenVPN response contract", err.Error())
		return
	}
	result, err := opnsense.Get[instanceAPIResponse](ctx, d.client, instanceReqOpts, id)
	if err != nil {
		resp.Diagnostics.AddError("Error reading OpenVPN instance", fmt.Sprintf("Could not read OpenVPN instance %s: %s", id, err))
		return
	}
	if err := config.fromAPI(ctx, result, id); err != nil {
		resp.Diagnostics.AddError("Error decoding OpenVPN instance", err.Error())
		return
	}
	if !supported {
		// The strict response type deliberately accepts the established selected-map
		// shapes and absent additive fields used before 26.7.6.
		config.DataCiphersOrdered = types.ListNull(types.StringType)
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &config)...)
}
