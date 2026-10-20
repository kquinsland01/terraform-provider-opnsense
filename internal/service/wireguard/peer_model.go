// Copyright (c) Matthew Mellor
// SPDX-License-Identifier: MPL-2.0

package wireguard

import (
	"context"
	"encoding/base64"
	"fmt"
	"net"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/matthew-on-git/terraform-provider-opnsense/internal/tfconv"
	"github.com/matthew-on-git/terraform-provider-opnsense/pkg/opnsense"
)

// PeerResourceModel represents the WireGuard peer Terraform state.
type PeerResourceModel struct {
	ID            types.String `tfsdk:"id"`
	Enabled       types.Bool   `tfsdk:"enabled"`
	Name          types.String `tfsdk:"name"`
	PublicKey     types.String `tfsdk:"public_key"`
	PSK           types.String `tfsdk:"psk"`
	TunnelAddress types.Set    `tfsdk:"tunnel_address"`
	ServerAddress types.String `tfsdk:"server_address"`
	ServerPort    types.Int64  `tfsdk:"server_port"`
	Keepalive     types.Int64  `tfsdk:"keepalive"`
	Servers       types.Set    `tfsdk:"servers"`
}

type wireguardPeerAPIResponse struct {
	Enabled       strictBool            `json:"enabled"`
	Name          string                `json:"name"`
	PublicKey     string                `json:"pubkey"`
	PSK           string                `json:"psk"`
	TunnelAddress strictSelectedMapList `json:"tunneladdress"`
	ServerAddress string                `json:"serveraddress"`
	ServerPort    strictOptionalInteger `json:"serverport"`
	Keepalive     strictOptionalInteger `json:"keepalive"`
	Servers       strictSelectedMapList `json:"servers"`
}

type wireguardPeerAPIRequest struct {
	Enabled       string `json:"enabled"`
	Name          string `json:"name"`
	PublicKey     string `json:"pubkey"`
	PSK           string `json:"psk"`
	TunnelAddress string `json:"tunneladdress"`
	ServerAddress string `json:"serveraddress"`
	ServerPort    string `json:"serverport"`
	Keepalive     string `json:"keepalive"`
	// Preserve API-owned associations on updates; omission clears them upstream.
	Servers *string `json:"servers,omitempty"`
}

func (m *PeerResourceModel) toAPI(ctx context.Context) (*wireguardPeerAPIRequest, error) {
	if m.Enabled.IsUnknown() || m.PublicKey.IsUnknown() || m.PSK.IsUnknown() || m.ServerAddress.IsUnknown() || m.Name.IsUnknown() {
		return nil, fmt.Errorf("WireGuard peer contains unresolved values")
	}
	if err := validateName(m.Name.ValueString()); err != nil {
		return nil, err
	}
	if m.ServerAddress.ValueString() != "" && !validEndpoint(m.ServerAddress.ValueString()) {
		return nil, fmt.Errorf("server_address must be an IP address or hostname")
	}
	if err := validateWireGuardKey("public_key", m.PublicKey.ValueString(), false); err != nil {
		return nil, err
	}
	if err := validateWireGuardKey("psk", m.PSK.ValueString(), true); err != nil {
		return nil, err
	}
	tunnels, err := setStrings(ctx, "tunnel_address", m.TunnelAddress)
	if err != nil {
		return nil, err
	}
	if len(tunnels) == 0 {
		return nil, fmt.Errorf("tunnel_address must contain at least one CIDR")
	}
	if err := validateCIDRs("tunnel_address", tunnels); err != nil {
		return nil, err
	}
	serverPort, err := optionalRange("server_port", m.ServerPort, 1, 65535)
	if err != nil {
		return nil, err
	}
	keepalive, err := optionalRange("keepalive", m.Keepalive, 1, 65535)
	if err != nil {
		return nil, err
	}
	return &wireguardPeerAPIRequest{
		Enabled: opnsense.BoolToString(m.Enabled.ValueBool()), Name: m.Name.ValueString(),
		PublicKey: m.PublicKey.ValueString(), PSK: m.PSK.ValueString(), TunnelAddress: strings.Join(tunnels, ","),
		ServerAddress: m.ServerAddress.ValueString(), ServerPort: serverPort, Keepalive: keepalive,
	}, nil
}

func (m *PeerResourceModel) fromAPI(_ context.Context, a *wireguardPeerAPIResponse, uuid string) error {
	if err := validateUUIDs("servers", []string(a.Servers)); err != nil {
		return err
	}
	if err := validateWireGuardKey("public_key", a.PublicKey, false); err != nil {
		return fmt.Errorf("malformed WireGuard peer response: %w", err)
	}
	if err := validateCIDRs("tunneladdress", []string(a.TunnelAddress)); err != nil {
		return fmt.Errorf("malformed WireGuard peer response: %w", err)
	}
	if a.ServerPort.Set && (a.ServerPort.Value < 1 || a.ServerPort.Value > 65535) {
		return fmt.Errorf("malformed WireGuard peer response: serverport %d is outside 1..65535", a.ServerPort.Value)
	}
	if a.Keepalive.Set && (a.Keepalive.Value < 1 || a.Keepalive.Value > 65535) {
		return fmt.Errorf("malformed WireGuard peer response: keepalive %d is outside 1..65535", a.Keepalive.Value)
	}
	if err := validateWireGuardKey("psk", a.PSK, true); err != nil {
		return fmt.Errorf("malformed WireGuard peer response: %w", err)
	}
	m.ID, m.Enabled, m.Name = types.StringValue(uuid), types.BoolValue(bool(a.Enabled)), types.StringValue(a.Name)
	m.PublicKey = types.StringValue(a.PublicKey)
	if a.PSK != "" {
		m.PSK = types.StringValue(a.PSK)
	}
	m.TunnelAddress = tfconv.SliceToSet([]string(a.TunnelAddress))
	m.ServerAddress = types.StringValue(a.ServerAddress)
	m.ServerPort, m.Keepalive = optionalIntegerValue(a.ServerPort), optionalIntegerValue(a.Keepalive)
	m.Servers = tfconv.SliceToSet([]string(a.Servers))
	return nil
}

func optionalIntegerValue(value strictOptionalInteger) types.Int64 {
	if !value.Set {
		return types.Int64Null()
	}
	return types.Int64Value(value.Value)
}

func optionalRange(field string, value types.Int64, minimum, maximum int64) (string, error) {
	if value.IsNull() {
		return "", nil
	}
	if value.IsUnknown() {
		return "", fmt.Errorf("%s is unresolved", field)
	}
	n := value.ValueInt64()
	if n < minimum || n > maximum {
		return "", fmt.Errorf("%s must be between %d and %d", field, minimum, maximum)
	}
	return strconv.FormatInt(n, 10), nil
}

func setStrings(ctx context.Context, field string, value types.Set) ([]string, error) {
	if value.IsNull() {
		return []string{}, nil
	}
	if value.IsUnknown() {
		return nil, fmt.Errorf("%s contains unresolved values", field)
	}
	var result []string
	diagnostics := value.ElementsAs(ctx, &result, false)
	if diagnostics.HasError() {
		return nil, fmt.Errorf("%s contains unresolved or non-string values: %v", field, diagnostics)
	}
	for _, item := range result {
		if strings.TrimSpace(item) == "" || item != strings.TrimSpace(item) || strings.Contains(item, ",") {
			return nil, fmt.Errorf("%s contains malformed member %q", field, item)
		}
	}
	sort.Strings(result)
	return result, nil
}

func validateCIDRs(field string, values []string) error {
	for _, value := range values {
		if _, _, err := net.ParseCIDR(value); err != nil {
			return fmt.Errorf("%s contains invalid CIDR %q", field, value)
		}
	}
	return nil
}

func validateIPs(field string, values []string) error {
	for _, value := range values {
		if net.ParseIP(value) == nil {
			return fmt.Errorf("%s contains invalid IP address %q", field, value)
		}
	}
	return nil
}

func validateWireGuardKey(field, value string, allowEmpty bool) error {
	if value == "" && allowEmpty {
		return nil
	}
	decoded, err := base64.StdEncoding.DecodeString(value)
	if err != nil || len(decoded) != 32 {
		return fmt.Errorf("%s must be a base64-encoded 256-bit WireGuard key", field)
	}
	return nil
}

func validateName(value string) error {
	if !regexp.MustCompile(`^[0-9a-zA-Z._-]{1,64}$`).MatchString(value) {
		return fmt.Errorf("name must be 1..64 alphanumeric, dot, dash or underscore characters")
	}
	return nil
}

func validateUUIDs(field string, values []string) error {
	for _, value := range values {
		if !regexp.MustCompile(`^[a-fA-F0-9]{8}-[a-fA-F0-9]{4}-[a-fA-F0-9]{4}-[a-fA-F0-9]{4}-[a-fA-F0-9]{12}$`).MatchString(value) {
			return fmt.Errorf("%s contains an invalid UUID", field)
		}
	}
	return nil
}

func validEndpoint(value string) bool {
	if net.ParseIP(value) != nil {
		return true
	}
	if len(value) > 253 {
		return false
	}
	for _, label := range strings.Split(strings.TrimSuffix(value, "."), ".") {
		if !regexp.MustCompile(`^[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?$`).MatchString(label) {
			return false
		}
	}
	return true
}
