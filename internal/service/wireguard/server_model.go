// Copyright (c) Matthew Mellor
// SPDX-License-Identifier: MPL-2.0

package wireguard

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/matthew-on-git/terraform-provider-opnsense/internal/tfconv"
	"github.com/matthew-on-git/terraform-provider-opnsense/pkg/opnsense"
)

// ServerResourceModel represents the WireGuard server Terraform state.
type ServerResourceModel struct {
	ID            types.String `tfsdk:"id"`
	Enabled       types.Bool   `tfsdk:"enabled"`
	Name          types.String `tfsdk:"name"`
	Instance      types.String `tfsdk:"instance"`
	PublicKey     types.String `tfsdk:"public_key"`
	PrivateKey    types.String `tfsdk:"private_key"`
	Port          types.Int64  `tfsdk:"port"`
	MTU           types.Int64  `tfsdk:"mtu"`
	DNS           types.Set    `tfsdk:"dns"`
	TunnelAddress types.Set    `tfsdk:"tunnel_address"`
	Peers         types.Set    `tfsdk:"peers"`
	DisableRoutes types.Bool   `tfsdk:"disable_routes"`
	Gateway       types.String `tfsdk:"gateway"`
	Description   types.String `tfsdk:"description"`
}

type wireguardServerAPIResponse struct {
	Enabled       strictBool            `json:"enabled"`
	Name          string                `json:"name"`
	Instance      string                `json:"instance"`
	PublicKey     string                `json:"pubkey"`
	PrivateKey    string                `json:"privkey"`
	Port          strictOptionalInteger `json:"port"`
	MTU           strictOptionalInteger `json:"mtu"`
	DNS           strictSelectedMapList `json:"dns"`
	TunnelAddress strictSelectedMapList `json:"tunneladdress"`
	DisableRoutes strictBool            `json:"disableroutes"`
	Gateway       string                `json:"gateway"`
	Peers         strictSelectedMapList `json:"peers"`
	Description   string                `json:"description"`
}

type wireguardServerAPIRequest struct {
	Enabled       string `json:"enabled"`
	Name          string `json:"name"`
	Instance      string `json:"instance,omitempty"`
	PublicKey     string `json:"pubkey"`
	PrivateKey    string `json:"privkey"`
	Port          string `json:"port"`
	MTU           string `json:"mtu"`
	DNS           string `json:"dns"`
	TunnelAddress string `json:"tunneladdress"`
	DisableRoutes string `json:"disableroutes"`
	Gateway       string `json:"gateway"`
	Peers         string `json:"peers"`
	Description   string `json:"-"`
}

func (m *ServerResourceModel) toAPI(ctx context.Context) (*wireguardServerAPIRequest, error) {
	if err := validateWireGuardKey("private_key", m.PrivateKey.ValueString(), false); err != nil {
		return nil, err
	}
	if !m.PublicKey.IsNull() && !m.PublicKey.IsUnknown() {
		if err := validateWireGuardKey("public_key", m.PublicKey.ValueString(), true); err != nil {
			return nil, err
		}
	}
	port, err := optionalRange("port", m.Port, 1, 65535)
	if err != nil {
		return nil, err
	}
	mtu, err := optionalRange("mtu", m.MTU, 1, 9300)
	if err != nil {
		return nil, err
	}
	dns, err := setStrings(ctx, "dns", m.DNS)
	if err != nil {
		return nil, err
	}
	if err := validateIPs("dns", dns); err != nil {
		return nil, err
	}
	tunnels, err := setStrings(ctx, "tunnel_address", m.TunnelAddress)
	if err != nil {
		return nil, err
	}
	if err := validateCIDRs("tunnel_address", tunnels); err != nil {
		return nil, err
	}
	peers, err := setStrings(ctx, "peers", m.Peers)
	if err != nil {
		return nil, err
	}
	if m.Enabled.IsUnknown() || m.DisableRoutes.IsUnknown() || m.PrivateKey.IsUnknown() || m.Name.IsUnknown() || m.Gateway.IsUnknown() {
		return nil, fmt.Errorf("WireGuard server contains unresolved required values")
	}
	if err := validateName(m.Name.ValueString()); err != nil {
		return nil, err
	}
	if err := validateUUIDs("peers", peers); err != nil {
		return nil, err
	}
	if m.DisableRoutes.ValueBool() {
		if m.Gateway.ValueString() != "" && net.ParseIP(m.Gateway.ValueString()) == nil {
			return nil, fmt.Errorf("gateway must be a valid IP address when disable_routes is true")
		}
	} else if m.Gateway.ValueString() != "" {
		return nil, fmt.Errorf("gateway must be empty when disable_routes is false")
	}
	instance := ""
	if !m.Instance.IsNull() && !m.Instance.IsUnknown() {
		instance = m.Instance.ValueString()
	}
	return &wireguardServerAPIRequest{
		Enabled: opnsense.BoolToString(m.Enabled.ValueBool()), Name: m.Name.ValueString(), Instance: instance,
		PublicKey: m.PublicKey.ValueString(), PrivateKey: m.PrivateKey.ValueString(), Port: port, MTU: mtu,
		DNS: strings.Join(dns, ","), TunnelAddress: strings.Join(tunnels, ","),
		DisableRoutes: opnsense.BoolToString(m.DisableRoutes.ValueBool()), Gateway: m.Gateway.ValueString(),
		Peers: strings.Join(peers, ","), Description: m.Description.ValueString(),
	}, nil
}

func (m *ServerResourceModel) fromAPI(_ context.Context, a *wireguardServerAPIResponse, uuid string) error {
	if n, err := strconv.ParseInt(a.Instance, 10, 64); err != nil || n < 0 {
		return fmt.Errorf("malformed WireGuard server instance: expected nonnegative integer")
	}
	if err := validateUUIDs("peers", []string(a.Peers)); err != nil {
		return err
	}
	if err := validateWireGuardKey("public_key", a.PublicKey, true); err != nil {
		return fmt.Errorf("malformed WireGuard server response: %w", err)
	}
	if a.Port.Set && (a.Port.Value < 1 || a.Port.Value > 65535) {
		return fmt.Errorf("malformed WireGuard server response: port %d is outside 1..65535", a.Port.Value)
	}
	if a.MTU.Set && (a.MTU.Value < 1 || a.MTU.Value > 9300) {
		return fmt.Errorf("malformed WireGuard server response: mtu %d is outside 1..9300", a.MTU.Value)
	}
	if err := validateIPs("dns", []string(a.DNS)); err != nil {
		return fmt.Errorf("malformed WireGuard server response: %w", err)
	}
	if err := validateCIDRs("tunneladdress", []string(a.TunnelAddress)); err != nil {
		return fmt.Errorf("malformed WireGuard server response: %w", err)
	}
	if a.Gateway != "" && (!bool(a.DisableRoutes) || net.ParseIP(a.Gateway) == nil) {
		return fmt.Errorf("malformed WireGuard server response: gateway %q is invalid while routes are disabled", a.Gateway)
	}
	if err := validateWireGuardKey("private_key", a.PrivateKey, true); err != nil {
		return fmt.Errorf("malformed WireGuard server response: %w", err)
	}
	m.ID, m.Enabled, m.Name = types.StringValue(uuid), types.BoolValue(bool(a.Enabled)), types.StringValue(a.Name)
	m.Instance, m.PublicKey = types.StringValue(a.Instance), types.StringValue(a.PublicKey)
	if a.PrivateKey != "" {
		m.PrivateKey = types.StringValue(a.PrivateKey)
	}
	m.Port, m.MTU = optionalIntegerValue(a.Port), optionalIntegerValue(a.MTU)
	m.DNS, m.TunnelAddress, m.Peers = tfconv.SliceToSet([]string(a.DNS)), tfconv.SliceToSet([]string(a.TunnelAddress)), tfconv.SliceToSet([]string(a.Peers))
	m.DisableRoutes, m.Gateway = types.BoolValue(bool(a.DisableRoutes)), types.StringValue(a.Gateway)
	if a.Description != "" || m.Description.IsNull() || m.Description.IsUnknown() {
		m.Description = types.StringValue(a.Description)
	}
	return nil
}
