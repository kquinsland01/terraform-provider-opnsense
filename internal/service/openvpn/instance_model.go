// Copyright (c) Matthew Mellor
// SPDX-License-Identifier: MPL-2.0

package openvpn

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/matthew-on-git/terraform-provider-opnsense/pkg/opnsense"
)

// InstanceResourceModel is the Terraform state model for opnsense_openvpn_instance.
// Existing public names and types intentionally remain unchanged. The ordered
// cipher attribute is additive because data_ciphers has historically been a set.
type InstanceResourceModel struct {
	ID                   types.String `tfsdk:"id"`
	VPNID                types.String `tfsdk:"vpnid"`
	Enabled              types.Bool   `tfsdk:"enabled"`
	Role                 types.String `tfsdk:"role"`
	Description          types.String `tfsdk:"description"`
	DevType              types.String `tfsdk:"dev_type"`
	Protocol             types.String `tfsdk:"protocol"`
	Port                 types.String `tfsdk:"port"`
	PortShare            types.String `tfsdk:"port_share"`
	Local                types.String `tfsdk:"local"`
	Remote               types.String `tfsdk:"remote"`
	Server               types.String `tfsdk:"server"`
	ServerIPv6           types.String `tfsdk:"server_ipv6"`
	Topology             types.String `tfsdk:"topology"`
	NoPool               types.Bool   `tfsdk:"no_pool"`
	BridgeGateway        types.String `tfsdk:"bridge_gateway"`
	BridgePool           types.String `tfsdk:"bridge_pool"`
	Route                types.Set    `tfsdk:"route"`
	PushRoute            types.Set    `tfsdk:"push_route"`
	PushExcludedRoutes   types.Set    `tfsdk:"push_excluded_routes"`
	CA                   types.String `tfsdk:"ca"`
	Cert                 types.String `tfsdk:"cert"`
	CRL                  types.String `tfsdk:"crl"`
	CertDepth            types.String `tfsdk:"cert_depth"`
	RemoteCertTLS        types.Bool   `tfsdk:"remote_cert_tls"`
	VerifyClientCert     types.String `tfsdk:"verify_client_cert"`
	UseOCSP              types.Bool   `tfsdk:"use_ocsp"`
	TLSKey               types.String `tfsdk:"tls_key"`
	DataCiphers          types.Set    `tfsdk:"data_ciphers"`
	DataCiphersOrdered   types.List   `tfsdk:"data_ciphers_ordered"`
	DataCiphersFallback  types.String `tfsdk:"data_ciphers_fallback"`
	Auth                 types.String `tfsdk:"auth"`
	AuthMode             types.Set    `tfsdk:"auth_mode"`
	LocalGroup           types.String `tfsdk:"local_group"`
	VariousFlags         types.Set    `tfsdk:"various_flags"`
	VariousPushFlags     types.Set    `tfsdk:"various_push_flags"`
	PushInactive         types.Int64  `tfsdk:"push_inactive"`
	UsernameAsCommonName types.Bool   `tfsdk:"username_as_common_name"`
	StrictUserCN         types.String `tfsdk:"strict_user_cn"`
	Username             types.String `tfsdk:"username"`
	Password             types.String `tfsdk:"password"`
	MaxClients           types.Int64  `tfsdk:"max_clients"`
	KeepaliveInterval    types.Int64  `tfsdk:"keepalive_interval"`
	KeepaliveTimeout     types.Int64  `tfsdk:"keepalive_timeout"`
	RenegSec             types.Int64  `tfsdk:"reneg_sec"`
	AuthGenToken         types.Int64  `tfsdk:"auth_gen_token"`
	AuthGenTokenRenewal  types.Int64  `tfsdk:"auth_gen_token_renewal"`
	AuthGenTokenSecret   types.String `tfsdk:"auth_gen_token_secret"`
	ProvisionExclusive   types.Bool   `tfsdk:"provision_exclusive"`
	RedirectGateway      types.Set    `tfsdk:"redirect_gateway"`
	RouteMetric          types.Int64  `tfsdk:"route_metric"`
	RegisterDNS          types.Bool   `tfsdk:"register_dns"`
	DNSDomain            types.Set    `tfsdk:"dns_domain"`
	DNSDomainSearch      types.Set    `tfsdk:"dns_domain_search"`
	DNSServers           types.Set    `tfsdk:"dns_servers"`
	NTPServers           types.Set    `tfsdk:"ntp_servers"`
	TunMTU               types.Int64  `tfsdk:"tun_mtu"`
	Fragment             types.Int64  `tfsdk:"fragment"`
	MSSFix               types.Bool   `tfsdk:"mss_fix"`
	CARPDependOn         types.String `tfsdk:"carp_depend_on"`
	CompressMigrate      types.Bool   `tfsdk:"compress_migrate"`
	IfConfigPoolPersist  types.Bool   `tfsdk:"ifconfig_pool_persist"`
	HTTPProxy            types.String `tfsdk:"http_proxy"`
	VerifyX509Name       types.String `tfsdk:"verify_x509_name"`
	Verb                 types.String `tfsdk:"verb"`
}

// These owner-local decoders intentionally reject malformed/ambiguous shapes.
// The shared decoders remain permissive for legacy owners.
type (
	strictSelectedMap  string
	strictSelectedList []string
)

func (s *strictSelectedMap) UnmarshalJSON(data []byte) error {
	trimmed := strings.TrimSpace(string(data))
	if trimmed == "null" || trimmed == "[]" || trimmed == `""` {
		*s = ""
		return nil
	}
	if strings.HasPrefix(trimmed, `"`) {
		var value string
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		*s = strictSelectedMap(value)
		return nil
	}
	if strings.HasPrefix(trimmed, "[") {
		var choices []struct {
			Selected json.RawMessage `json:"selected"`
		}
		if err := json.Unmarshal(data, &choices); err != nil {
			return fmt.Errorf("expected indexed-choice array: %w", err)
		}
		selected := ""
		selectedCount := 0
		for index, choice := range choices {
			chosen, err := strictSelected(choice.Selected)
			if err != nil {
				return fmt.Errorf("indexed choice %d: %w", index, err)
			}
			if chosen {
				selectedCount++
				if selectedCount > 1 {
					return fmt.Errorf("ambiguous indexed-choice array contains multiple selected indexes %s and %d", selected, index)
				}
				selected = strconv.Itoa(index)
			}
		}
		*s = strictSelectedMap(selected)
		return nil
	}
	var raw map[string]struct {
		Selected json.RawMessage `json:"selected"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("expected selected-map object: %w", err)
	}
	selected := ""
	selectedCount := 0
	for key, entry := range raw {
		chosen, err := strictSelected(entry.Selected)
		if err != nil {
			return fmt.Errorf("selection %q: %w", key, err)
		}
		if chosen {
			selectedCount++
			if selectedCount > 1 {
				return fmt.Errorf("ambiguous selected-map contains multiple selected values %q and %q", selected, key)
			}
			selected = key
		}
	}
	*s = strictSelectedMap(selected)
	return nil
}

func (s *strictSelectedList) UnmarshalJSON(data []byte) error {
	trimmed := strings.TrimSpace(string(data))
	if trimmed == "null" || trimmed == "[]" || trimmed == `""` {
		*s = []string{}
		return nil
	}
	if strings.HasPrefix(trimmed, `"`) {
		var value string
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		if value == "" {
			*s = []string{}
		} else {
			*s = []string{value}
		}
		return nil
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	if delim, ok := token.(json.Delim); !ok || delim != '{' {
		return fmt.Errorf("expected selected-map object")
	}
	selected := make([]string, 0)
	for decoder.More() {
		keyToken, err := decoder.Token()
		if err != nil {
			return err
		}
		key, ok := keyToken.(string)
		if !ok {
			return fmt.Errorf("selected-map key is not a string")
		}
		var entry struct {
			Selected json.RawMessage `json:"selected"`
		}
		if err := decoder.Decode(&entry); err != nil {
			return err
		}
		chosen, err := strictSelected(entry.Selected)
		if err != nil {
			return fmt.Errorf("selection %q: %w", key, err)
		}
		if chosen {
			selected = append(selected, key)
		}
	}
	if len(selected) == 1 && selected[0] == "" {
		*s = []string{}
		return nil
	}
	if len(selected) > 1 {
		for _, value := range selected {
			if value == "" {
				return fmt.Errorf("ambiguous selected-map contains an empty selection with other selected values")
			}
		}
	}
	*s = selected
	return nil
}

func strictSelected(raw json.RawMessage) (bool, error) {
	if len(raw) == 0 {
		return false, fmt.Errorf("missing selected marker")
	}
	var value any
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err := decoder.Decode(&value); err != nil {
		return false, err
	}
	switch v := value.(type) {
	case string:
		if v == "0" || v == "1" {
			return v == "1", nil
		}
	case json.Number:
		if v.String() == "0" || v.String() == "1" {
			return v.String() == "1", nil
		}
	}
	return false, fmt.Errorf("selected marker must be 0 or 1")
}

type instanceAPIResponse struct {
	VPNID                string             `json:"vpnid"`
	Enabled              string             `json:"enabled"`
	Role                 strictSelectedMap  `json:"role"`
	Description          string             `json:"description"`
	DevType              strictSelectedMap  `json:"dev_type"`
	Protocol             strictSelectedMap  `json:"proto"`
	Port                 string             `json:"port"`
	PortShare            string             `json:"port-share"`
	Local                string             `json:"local"`
	Remote               strictSelectedMap  `json:"remote"`
	Server               string             `json:"server"`
	ServerIPv6           string             `json:"server_ipv6"`
	Topology             strictSelectedMap  `json:"topology"`
	NoPool               string             `json:"nopool"`
	BridgeGateway        string             `json:"bridge_gateway"`
	BridgePool           string             `json:"bridge_pool"`
	Route                strictSelectedList `json:"route"`
	PushRoute            strictSelectedList `json:"push_route"`
	PushExcludedRoutes   strictSelectedList `json:"push_excluded_routes"`
	CA                   strictSelectedMap  `json:"ca"`
	Cert                 strictSelectedMap  `json:"cert"`
	CRL                  strictSelectedMap  `json:"crl"`
	CertDepth            strictSelectedMap  `json:"cert_depth"`
	RemoteCertTLS        string             `json:"remote_cert_tls"`
	VerifyClientCert     strictSelectedMap  `json:"verify_client_cert"`
	UseOCSP              string             `json:"use_ocsp"`
	TLSKey               strictSelectedMap  `json:"tls_key"`
	DataCiphers          strictSelectedList `json:"data-ciphers"`
	DataCiphersFallback  strictSelectedMap  `json:"data-ciphers-fallback"`
	Auth                 strictSelectedMap  `json:"auth"`
	AuthMode             strictSelectedList `json:"authmode"`
	LocalGroup           strictSelectedMap  `json:"local_group"`
	VariousFlags         strictSelectedList `json:"various_flags"`
	VariousPushFlags     strictSelectedList `json:"various_push_flags"`
	PushInactive         string             `json:"push_inactive"`
	UsernameAsCommonName string             `json:"username_as_common_name"`
	StrictUserCN         strictSelectedMap  `json:"strictusercn"`
	Username             string             `json:"username"`
	Password             string             `json:"password"`
	MaxClients           string             `json:"maxclients"`
	KeepaliveInterval    string             `json:"keepalive_interval"`
	KeepaliveTimeout     string             `json:"keepalive_timeout"`
	RenegSec             string             `json:"reneg-sec"`
	AuthGenToken         string             `json:"auth-gen-token"`
	AuthGenTokenRenewal  string             `json:"auth-gen-token-renewal"`
	AuthGenTokenSecret   string             `json:"auth-gen-token-secret"`
	ProvisionExclusive   string             `json:"provision_exclusive"`
	RedirectGateway      strictSelectedList `json:"redirect_gateway"`
	RouteMetric          string             `json:"route_metric"`
	RegisterDNS          string             `json:"register_dns"`
	DNSDomain            strictSelectedList `json:"dns_domain"`
	DNSDomainSearch      strictSelectedList `json:"dns_domain_search"`
	DNSServers           strictSelectedList `json:"dns_servers"`
	NTPServers           strictSelectedList `json:"ntp_servers"`
	TunMTU               string             `json:"tun_mtu"`
	Fragment             string             `json:"fragment"`
	MSSFix               string             `json:"mssfix"`
	CARPDependOn         strictSelectedMap  `json:"carp_depend_on"`
	CompressMigrate      string             `json:"compress_migrate"`
	IfConfigPoolPersist  string             `json:"ifconfig-pool-persist"`
	HTTPProxy            string             `json:"http-proxy"`
	VerifyX509Name       string             `json:"verify-x509-name"`
	Verb                 strictSelectedMap  `json:"verb"`
}

// UnmarshalJSON adds field context for the source-observed indexed choices.
// The local alias prevents recursion while the RawMessage shadows defer these
// two fields to the strict decoder below.
func (a *instanceAPIResponse) UnmarshalJSON(data []byte) error {
	type responseAlias instanceAPIResponse
	decoded := responseAlias(*a)
	var indexed struct {
		*responseAlias
		StrictUserCN json.RawMessage `json:"strictusercn"`
		Verb         json.RawMessage `json:"verb"`
	}
	indexed.responseAlias = &decoded
	if err := json.Unmarshal(data, &indexed); err != nil {
		return err
	}
	if len(indexed.StrictUserCN) > 0 {
		if err := json.Unmarshal(indexed.StrictUserCN, &decoded.StrictUserCN); err != nil {
			return fmt.Errorf("decode OpenVPN field strictusercn: %w", err)
		}
	}
	if len(indexed.Verb) > 0 {
		if err := json.Unmarshal(indexed.Verb, &decoded.Verb); err != nil {
			return fmt.Errorf("decode OpenVPN field verb: %w", err)
		}
	}
	*a = instanceAPIResponse(decoded)
	return nil
}

type instanceAPIRequest struct {
	VPNID                string  `json:"vpnid"`
	Enabled              string  `json:"enabled"`
	Role                 string  `json:"role"`
	Description          string  `json:"description"`
	DevType              string  `json:"dev_type"`
	Protocol             string  `json:"proto"`
	Port                 string  `json:"port"`
	PortShare            *string `json:"port-share,omitempty"`
	Local                string  `json:"local"`
	Remote               string  `json:"remote"`
	Server               string  `json:"server"`
	ServerIPv6           *string `json:"server_ipv6,omitempty"`
	Topology             string  `json:"topology"`
	NoPool               *string `json:"nopool,omitempty"`
	BridgeGateway        *string `json:"bridge_gateway,omitempty"`
	BridgePool           *string `json:"bridge_pool,omitempty"`
	Route                *string `json:"route,omitempty"`
	PushRoute            *string `json:"push_route,omitempty"`
	PushExcludedRoutes   *string `json:"push_excluded_routes,omitempty"`
	CA                   string  `json:"ca"`
	Cert                 string  `json:"cert"`
	CRL                  *string `json:"crl,omitempty"`
	CertDepth            *string `json:"cert_depth,omitempty"`
	RemoteCertTLS        *string `json:"remote_cert_tls,omitempty"`
	VerifyClientCert     string  `json:"verify_client_cert"`
	UseOCSP              *string `json:"use_ocsp,omitempty"`
	TLSKey               string  `json:"tls_key"`
	DataCiphers          *string `json:"data-ciphers,omitempty"`
	DataCiphersFallback  *string `json:"data-ciphers-fallback,omitempty"`
	Auth                 string  `json:"auth"`
	AuthMode             *string `json:"authmode,omitempty"`
	LocalGroup           *string `json:"local_group,omitempty"`
	VariousFlags         *string `json:"various_flags,omitempty"`
	VariousPushFlags     *string `json:"various_push_flags,omitempty"`
	PushInactive         *string `json:"push_inactive,omitempty"`
	UsernameAsCommonName *string `json:"username_as_common_name,omitempty"`
	StrictUserCN         *string `json:"strictusercn,omitempty"`
	Username             *string `json:"username,omitempty"`
	Password             *string `json:"password,omitempty"`
	MaxClients           string  `json:"maxclients"`
	KeepaliveInterval    string  `json:"keepalive_interval"`
	KeepaliveTimeout     string  `json:"keepalive_timeout"`
	RenegSec             *string `json:"reneg-sec,omitempty"`
	AuthGenToken         *string `json:"auth-gen-token,omitempty"`
	AuthGenTokenRenewal  *string `json:"auth-gen-token-renewal,omitempty"`
	AuthGenTokenSecret   *string `json:"auth-gen-token-secret,omitempty"`
	ProvisionExclusive   *string `json:"provision_exclusive,omitempty"`
	RedirectGateway      *string `json:"redirect_gateway,omitempty"`
	RouteMetric          *string `json:"route_metric,omitempty"`
	RegisterDNS          *string `json:"register_dns,omitempty"`
	DNSDomain            *string `json:"dns_domain,omitempty"`
	DNSDomainSearch      *string `json:"dns_domain_search,omitempty"`
	DNSServers           *string `json:"dns_servers,omitempty"`
	NTPServers           *string `json:"ntp_servers,omitempty"`
	TunMTU               *string `json:"tun_mtu,omitempty"`
	Fragment             *string `json:"fragment,omitempty"`
	MSSFix               *string `json:"mssfix,omitempty"`
	CARPDependOn         *string `json:"carp_depend_on,omitempty"`
	CompressMigrate      *string `json:"compress_migrate,omitempty"`
	IfConfigPoolPersist  *string `json:"ifconfig-pool-persist,omitempty"`
	HTTPProxy            *string `json:"http-proxy,omitempty"`
	VerifyX509Name       *string `json:"verify-x509-name,omitempty"`
	Verb                 string  `json:"verb,omitempty"`
}

func ptr(value string) *string { return &value }
func optionalString(value types.String) *string {
	if value.IsNull() || value.IsUnknown() {
		return nil
	}
	return ptr(value.ValueString())
}

func optionalBool(value types.Bool) *string {
	if value.IsNull() || value.IsUnknown() {
		return nil
	}
	return ptr(opnsense.BoolToString(value.ValueBool()))
}

func optionalInt(value types.Int64) *string {
	if value.IsNull() || value.IsUnknown() {
		return nil
	}
	return ptr(int64ToStringOrEmpty(value.ValueInt64()))
}

func collectionCSV(ctx context.Context, value types.Set) (*string, error) {
	if value.IsNull() || value.IsUnknown() {
		return nil, nil
	}
	var items []string
	if diags := value.ElementsAs(ctx, &items, false); diags.HasError() {
		return nil, fmt.Errorf("collection contains an unknown or invalid value: %s", diags.Errors()[0].Detail())
	}
	return ptr(strings.Join(items, ",")), nil
}

func orderedCSV(ctx context.Context, set types.Set, ordered types.List) (*string, error) {
	setValue, err := collectionCSV(ctx, set)
	if err != nil {
		return nil, err
	}
	if ordered.IsNull() || ordered.IsUnknown() {
		return setValue, nil
	}
	var orderedItems []string
	if diags := ordered.ElementsAs(ctx, &orderedItems, false); diags.HasError() {
		return nil, fmt.Errorf("data_ciphers_ordered contains an unknown or invalid value: %s", diags.Errors()[0].Detail())
	}
	if setValue != nil {
		var setItems []string
		if diags := set.ElementsAs(ctx, &setItems, false); diags.HasError() {
			return nil, fmt.Errorf("data_ciphers contains an unknown or invalid value")
		}
		a, b := append([]string(nil), setItems...), append([]string(nil), orderedItems...)
		sort.Strings(a)
		sort.Strings(b)
		if strings.Join(a, "\x00") != strings.Join(b, "\x00") {
			return nil, fmt.Errorf("data_ciphers and data_ciphers_ordered must contain the same cipher values")
		}
	}
	return ptr(strings.Join(orderedItems, ",")), nil
}

func (m *InstanceResourceModel) toAPI(ctx context.Context) (*instanceAPIRequest, error) {
	dataCiphers, err := orderedCSV(ctx, m.DataCiphers, m.DataCiphersOrdered)
	if err != nil {
		return nil, err
	}
	c := make([]*string, 11)
	values := []types.Set{m.Route, m.PushRoute, m.PushExcludedRoutes, m.AuthMode, m.VariousFlags, m.VariousPushFlags, m.RedirectGateway, m.DNSDomain, m.DNSDomainSearch, m.DNSServers, m.NTPServers}
	for i := range values {
		c[i], err = collectionCSV(ctx, values[i])
		if err != nil {
			return nil, err
		}
	}
	return &instanceAPIRequest{
		Enabled: opnsense.BoolToString(m.Enabled.ValueBool()), Role: m.Role.ValueString(), VPNID: m.VPNID.ValueString(), Description: m.Description.ValueString(), DevType: m.DevType.ValueString(), Protocol: m.Protocol.ValueString(), Port: m.Port.ValueString(), PortShare: optionalString(m.PortShare), Local: m.Local.ValueString(), Remote: m.Remote.ValueString(), Server: m.Server.ValueString(), ServerIPv6: optionalString(m.ServerIPv6), Topology: m.Topology.ValueString(), NoPool: optionalBool(m.NoPool), BridgeGateway: optionalString(m.BridgeGateway), BridgePool: optionalString(m.BridgePool), Route: c[0], PushRoute: c[1], PushExcludedRoutes: c[2], CA: m.CA.ValueString(), Cert: m.Cert.ValueString(), CRL: optionalString(m.CRL), CertDepth: optionalString(m.CertDepth), RemoteCertTLS: optionalBool(m.RemoteCertTLS), VerifyClientCert: m.VerifyClientCert.ValueString(), UseOCSP: optionalBool(m.UseOCSP), TLSKey: m.TLSKey.ValueString(), DataCiphers: dataCiphers, DataCiphersFallback: optionalString(m.DataCiphersFallback), Auth: m.Auth.ValueString(), AuthMode: c[3], LocalGroup: optionalString(m.LocalGroup), VariousFlags: c[4], VariousPushFlags: c[5], PushInactive: optionalInt(m.PushInactive), UsernameAsCommonName: optionalBool(m.UsernameAsCommonName), StrictUserCN: optionalString(m.StrictUserCN), Username: optionalString(m.Username), Password: optionalString(m.Password), MaxClients: int64ToStringOrEmpty(m.MaxClients.ValueInt64()), KeepaliveInterval: int64ToStringOrEmpty(m.KeepaliveInterval.ValueInt64()), KeepaliveTimeout: int64ToStringOrEmpty(m.KeepaliveTimeout.ValueInt64()), RenegSec: optionalInt(m.RenegSec), AuthGenToken: optionalInt(m.AuthGenToken), AuthGenTokenRenewal: optionalInt(m.AuthGenTokenRenewal), AuthGenTokenSecret: optionalString(m.AuthGenTokenSecret), ProvisionExclusive: optionalBool(m.ProvisionExclusive), RedirectGateway: c[6], RouteMetric: optionalInt(m.RouteMetric), RegisterDNS: optionalBool(m.RegisterDNS), DNSDomain: c[7], DNSDomainSearch: c[8], DNSServers: c[9], NTPServers: c[10], TunMTU: optionalInt(m.TunMTU), Fragment: optionalInt(m.Fragment), MSSFix: optionalBool(m.MSSFix), CARPDependOn: optionalString(m.CARPDependOn), CompressMigrate: optionalBool(m.CompressMigrate), IfConfigPoolPersist: optionalBool(m.IfConfigPoolPersist), HTTPProxy: optionalString(m.HTTPProxy), VerifyX509Name: optionalString(m.VerifyX509Name), Verb: m.Verb.ValueString(),
	}, nil
}

func int64ToStringOrEmpty(n int64) string {
	if n == 0 {
		return ""
	}
	return strconv.FormatInt(n, 10)
}

func sliceToSet(items []string) types.Set {
	values := make([]attr.Value, len(items))
	for i, item := range items {
		values[i] = types.StringValue(item)
	}
	return types.SetValueMust(types.StringType, values)
}

func sliceToList(items []string) types.List {
	values := make([]attr.Value, len(items))
	for i, item := range items {
		values[i] = types.StringValue(item)
	}
	return types.ListValueMust(types.StringType, values)
}

func strictInt(field, value string) (types.Int64, error) {
	if value == "" {
		return types.Int64Value(0), nil
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return types.Int64Null(), fmt.Errorf("decode OpenVPN field %s: expected integer, got %q", field, value)
	}
	return types.Int64Value(parsed), nil
}

func strictBool(field, value string) (types.Bool, error) {
	switch value {
	case "0", "":
		return types.BoolValue(false), nil
	case "1":
		return types.BoolValue(true), nil
	default:
		return types.BoolNull(), fmt.Errorf("decode OpenVPN field %s: expected 0 or 1, got %q", field, value)
	}
}

func (m *InstanceResourceModel) fromAPI(_ context.Context, a *instanceAPIResponse, uuid string) error {
	priorPassword, priorTokenSecret := m.Password, m.AuthGenTokenSecret
	m.ID, m.VPNID = types.StringValue(uuid), types.StringValue(a.VPNID)
	m.Role, m.Description, m.DevType, m.Protocol = types.StringValue(string(a.Role)), types.StringValue(a.Description), types.StringValue(string(a.DevType)), types.StringValue(string(a.Protocol))
	m.Port, m.PortShare, m.Local, m.Remote = types.StringValue(a.Port), types.StringValue(a.PortShare), types.StringValue(a.Local), types.StringValue(string(a.Remote))
	m.Server, m.ServerIPv6, m.Topology = types.StringValue(a.Server), types.StringValue(a.ServerIPv6), types.StringValue(string(a.Topology))
	m.BridgeGateway, m.BridgePool = types.StringValue(a.BridgeGateway), types.StringValue(a.BridgePool)
	m.CA, m.Cert, m.CRL, m.CertDepth = types.StringValue(string(a.CA)), types.StringValue(string(a.Cert)), types.StringValue(string(a.CRL)), types.StringValue(string(a.CertDepth))
	m.VerifyClientCert, m.TLSKey, m.DataCiphersFallback, m.Auth = types.StringValue(string(a.VerifyClientCert)), types.StringValue(string(a.TLSKey)), types.StringValue(string(a.DataCiphersFallback)), types.StringValue(string(a.Auth))
	m.LocalGroup, m.StrictUserCN = types.StringValue(string(a.LocalGroup)), types.StringValue(string(a.StrictUserCN))
	m.Username = types.StringValue(a.Username)
	if a.Password != "" || priorPassword.IsNull() || priorPassword.IsUnknown() {
		m.Password = types.StringValue(a.Password)
	} else {
		m.Password = priorPassword
	}
	if a.AuthGenTokenSecret != "" || priorTokenSecret.IsNull() || priorTokenSecret.IsUnknown() {
		m.AuthGenTokenSecret = types.StringValue(a.AuthGenTokenSecret)
	} else {
		m.AuthGenTokenSecret = priorTokenSecret
	}
	m.CARPDependOn, m.HTTPProxy, m.VerifyX509Name, m.Verb = types.StringValue(string(a.CARPDependOn)), types.StringValue(a.HTTPProxy), types.StringValue(a.VerifyX509Name), types.StringValue(string(a.Verb))
	m.Route, m.PushRoute, m.PushExcludedRoutes = sliceToSet(a.Route), sliceToSet(a.PushRoute), sliceToSet(a.PushExcludedRoutes)
	m.DataCiphers, m.DataCiphersOrdered = sliceToSet(a.DataCiphers), sliceToList(a.DataCiphers)
	m.AuthMode, m.VariousFlags, m.VariousPushFlags = sliceToSet(a.AuthMode), sliceToSet(a.VariousFlags), sliceToSet(a.VariousPushFlags)
	m.RedirectGateway, m.DNSDomain, m.DNSDomainSearch = sliceToSet(a.RedirectGateway), sliceToSet(a.DNSDomain), sliceToSet(a.DNSDomainSearch)
	m.DNSServers, m.NTPServers = sliceToSet(a.DNSServers), sliceToSet(a.NTPServers)
	boolFields := []struct {
		name, raw string
		dest      *types.Bool
	}{{"enabled", a.Enabled, &m.Enabled}, {"no_pool", a.NoPool, &m.NoPool}, {"remote_cert_tls", a.RemoteCertTLS, &m.RemoteCertTLS}, {"use_ocsp", a.UseOCSP, &m.UseOCSP}, {"username_as_common_name", a.UsernameAsCommonName, &m.UsernameAsCommonName}, {"provision_exclusive", a.ProvisionExclusive, &m.ProvisionExclusive}, {"register_dns", a.RegisterDNS, &m.RegisterDNS}, {"mss_fix", a.MSSFix, &m.MSSFix}, {"compress_migrate", a.CompressMigrate, &m.CompressMigrate}, {"ifconfig_pool_persist", a.IfConfigPoolPersist, &m.IfConfigPoolPersist}}
	for _, field := range boolFields {
		value, err := strictBool(field.name, field.raw)
		if err != nil {
			return err
		}
		*field.dest = value
	}
	intFields := []struct {
		name, raw string
		dest      *types.Int64
	}{{"push_inactive", a.PushInactive, &m.PushInactive}, {"max_clients", a.MaxClients, &m.MaxClients}, {"keepalive_interval", a.KeepaliveInterval, &m.KeepaliveInterval}, {"keepalive_timeout", a.KeepaliveTimeout, &m.KeepaliveTimeout}, {"reneg_sec", a.RenegSec, &m.RenegSec}, {"auth_gen_token", a.AuthGenToken, &m.AuthGenToken}, {"auth_gen_token_renewal", a.AuthGenTokenRenewal, &m.AuthGenTokenRenewal}, {"route_metric", a.RouteMetric, &m.RouteMetric}, {"tun_mtu", a.TunMTU, &m.TunMTU}, {"fragment", a.Fragment, &m.Fragment}}
	for _, field := range intFields {
		value, err := strictInt(field.name, field.raw)
		if err != nil {
			return err
		}
		*field.dest = value
	}
	return nil
}
