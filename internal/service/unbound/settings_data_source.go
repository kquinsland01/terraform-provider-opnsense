// Copyright (c) Matthew Mellor
// SPDX-License-Identifier: MPL-2.0

package unbound

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"sort"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	dsschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/matthew-on-git/terraform-provider-opnsense/pkg/opnsense"
)

var _ datasource.DataSource = &settingsDataSource{}

type settingsDataSource struct{ client *opnsense.Client }

func newSettingsDataSource() datasource.DataSource { return &settingsDataSource{} }

func (d *settingsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_unbound_settings"
}

func (d *settingsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	attrs := map[string]dsschema.Attribute{
		"id":         dsschema.StringAttribute{Computed: true, MarkdownDescription: "Synthetic identifier for the complete Unbound settings view."},
		"dnsbl_json": dsschema.StringAttribute{Computed: true, MarkdownDescription: "Canonical JSON for the authoritative current `dnsbl` branch, including every blocklist item."},
		"dnsbl_items": dsschema.ListAttribute{
			Computed:            true,
			ElementType:         types.ObjectType{AttrTypes: dnsblSettingsItemAttrTypes()},
			MarkdownDescription: "Every current array-backed DNSBL blocklist item, losslessly identified by UUID.",
		},
	}
	bools := []string{
		"enabled", "stats", "dnssec", "dns64", "no_aaaa_records", "register_dhcp", "register_dhcp_static",
		"no_register_link_local6", "no_register_records", "txt_support", "cache_flush", "enable_wpad", "safe_search",
		"hide_identity", "hide_version", "enable_prefetch_key", "dnssec_stripped", "aggressive_nsec", "qname_min_strict",
		"enable_prefetch", "keep_probing_hosts", "extended_statistics", "log_queries", "log_replies", "tag_query_reply",
		"log_local_actions", "log_servfail", "serve_expired_enabled", "serve_expired_reset_ttl", "forwarding_enabled",
	}
	ints := []string{
		"outgoing_num_tcp", "incoming_num_tcp", "num_queries_per_thread", "outgoing_range", "jostle_timeout", "discard_timeout",
		"unwanted_reply_threshold", "max_ttl", "max_negative_ttl", "min_ttl", "host_ttl", "num_hosts", "verbosity_level", "validation_level",
	}
	sets := []string{"active_interface", "outgoing_interface", "private_domains", "private_addresses", "insecure_domains"}
	strings := []string{
		"port", "dns64_prefix", "register_dhcp_domain", "local_zone_type", "msg_cache_size", "rrset_cache_size",
		"serve_expired_record_reply_ttl", "serve_expired_ttl", "serve_expired_client_response_timeout", "acl_default_action",
	}
	for _, name := range bools {
		attrs[name] = dsschema.BoolAttribute{Computed: true, MarkdownDescription: "Authoritative Unbound setting."}
	}
	attrs["no_aaaa_records"] = dsschema.BoolAttribute{Computed: true, MarkdownDescription: "Whether upstream `noarecords` removes A records (legacy Terraform attribute name)."}
	for _, name := range ints {
		attrs[name] = dsschema.Int64Attribute{Computed: true, MarkdownDescription: "Authoritative Unbound setting."}
	}
	for _, name := range sets {
		attrs[name] = dsschema.SetAttribute{Computed: true, ElementType: types.StringType, MarkdownDescription: "Authoritative Unbound setting."}
	}
	for _, name := range strings {
		attrs[name] = dsschema.StringAttribute{Computed: true, MarkdownDescription: "Authoritative Unbound setting."}
	}
	resp.Schema = dsschema.Schema{
		MarkdownDescription: "Reads the complete stable Unbound settings tree without claiming writable ownership. The general, DNSBL, ACL-default, and forwarding owners remain separate.",
		Attributes:          attrs,
	}
}

type dnsblSettingsItemModel struct {
	UUID        types.String `tfsdk:"uuid"`
	Enabled     types.Bool   `tfsdk:"enabled"`
	Type        types.Set    `tfsdk:"type"`
	Lists       types.Set    `tfsdk:"lists"`
	Allowlists  types.Set    `tfsdk:"allowlists"`
	Blocklists  types.Set    `tfsdk:"blocklists"`
	Wildcards   types.Set    `tfsdk:"wildcards"`
	SourceNets  types.Set    `tfsdk:"source_nets"`
	Address     types.String `tfsdk:"address"`
	NXDomain    types.Bool   `tfsdk:"nxdomain"`
	CacheTTL    types.Int64  `tfsdk:"cache_ttl"`
	Description types.String `tfsdk:"description"`
}

func decodeDNSBLItems(raw json.RawMessage) ([]dnsblSettingsItemModel, error) {
	var branch struct {
		Blocklist map[string]json.RawMessage `json:"blocklist"`
	}
	if len(raw) == 0 || string(raw) == "null" {
		return nil, fmt.Errorf("missing dnsbl branch")
	}
	if err := json.Unmarshal(raw, &branch); err != nil {
		return nil, fmt.Errorf("malformed dnsbl branch: %w", err)
	}
	if branch.Blocklist == nil {
		return nil, fmt.Errorf("malformed dnsbl branch: missing blocklist collection")
	}
	uuids := make([]string, 0, len(branch.Blocklist))
	for uuid := range branch.Blocklist {
		uuids = append(uuids, uuid)
	}
	sort.Strings(uuids)
	items := make([]dnsblSettingsItemModel, 0, len(uuids))
	for _, uuid := range uuids {
		if uuid == "" {
			return nil, fmt.Errorf("malformed dnsbl.blocklist: item UUID is empty")
		}
		var wire dnsblItemAPIResponse
		if err := json.Unmarshal(branch.Blocklist[uuid], &wire); err != nil {
			return nil, fmt.Errorf("malformed dnsbl.blocklist.%s: %w", uuid, err)
		}
		if err := validateDNSBLItem(&wire, uuid); err != nil {
			return nil, fmt.Errorf("malformed dnsbl.blocklist.%s: %w", uuid, err)
		}
		cacheTTL, err := strconv.ParseInt(wire.CacheTTL, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("malformed dnsbl.blocklist.%s.cache_ttl: %w", uuid, err)
		}
		items = append(items, dnsblSettingsItemModel{
			UUID: types.StringValue(uuid), Enabled: types.BoolValue(opnsense.StringToBool(wire.Enabled)),
			Type:       types.SetValueMust(types.StringType, stringValues(wire.Type)),
			Lists:      types.SetValueMust(types.StringType, stringValues(wire.Lists)),
			Allowlists: types.SetValueMust(types.StringType, stringValues(wire.Allowlists)),
			Blocklists: types.SetValueMust(types.StringType, stringValues(wire.Blocklists)),
			Wildcards:  types.SetValueMust(types.StringType, stringValues(wire.Wildcards)),
			SourceNets: types.SetValueMust(types.StringType, stringValues(wire.SourceNets)),
			Address:    types.StringValue(wire.Address), NXDomain: types.BoolValue(opnsense.StringToBool(wire.NXDomain)),
			CacheTTL: types.Int64Value(cacheTTL), Description: types.StringValue(wire.Description),
		})
	}
	return items, nil
}

func canonicalJSON(raw json.RawMessage) ([]byte, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return nil, err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return nil, fmt.Errorf("trailing JSON value")
	}
	return json.Marshal(value)
}

func stringValues(values []string) []attr.Value {
	result := make([]attr.Value, len(values))
	for i, value := range values {
		result[i] = types.StringValue(value)
	}
	return result
}

func dnsblSettingsItemAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"uuid": types.StringType, "enabled": types.BoolType, "type": types.SetType{ElemType: types.StringType},
		"lists": types.SetType{ElemType: types.StringType}, "allowlists": types.SetType{ElemType: types.StringType},
		"blocklists": types.SetType{ElemType: types.StringType}, "wildcards": types.SetType{ElemType: types.StringType},
		"source_nets": types.SetType{ElemType: types.StringType}, "address": types.StringType,
		"nxdomain": types.BoolType, "cache_ttl": types.Int64Type, "description": types.StringType,
	}
}

func dnsblSettingsItemsValue(items []dnsblSettingsItemModel) types.List {
	itemType := types.ObjectType{AttrTypes: dnsblSettingsItemAttrTypes()}
	values := make([]attr.Value, 0, len(items))
	for _, item := range items {
		values = append(values, types.ObjectValueMust(itemType.AttrTypes, map[string]attr.Value{
			"uuid": item.UUID, "enabled": item.Enabled, "type": item.Type, "lists": item.Lists,
			"allowlists": item.Allowlists, "blocklists": item.Blocklists, "wildcards": item.Wildcards,
			"source_nets": item.SourceNets, "address": item.Address, "nxdomain": item.NXDomain,
			"cache_ttl": item.CacheTTL, "description": item.Description,
		}))
	}
	return types.ListValueMust(itemType, values)
}

func (d *settingsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

type settingsDataSourceModel struct {
	ID                                types.String `tfsdk:"id"`
	Enabled                           types.Bool   `tfsdk:"enabled"`
	Port                              types.String `tfsdk:"port"`
	Stats                             types.Bool   `tfsdk:"stats"`
	ActiveInterface                   types.Set    `tfsdk:"active_interface"`
	DNSSEC                            types.Bool   `tfsdk:"dnssec"`
	DNS64                             types.Bool   `tfsdk:"dns64"`
	DNS64Prefix                       types.String `tfsdk:"dns64_prefix"`
	NoAAAARecords                     types.Bool   `tfsdk:"no_aaaa_records"`
	RegisterDHCP                      types.Bool   `tfsdk:"register_dhcp"`
	RegisterDHCPDomain                types.String `tfsdk:"register_dhcp_domain"`
	RegisterDHCPStatic                types.Bool   `tfsdk:"register_dhcp_static"`
	NoRegisterLinkLocal6              types.Bool   `tfsdk:"no_register_link_local6"`
	NoRegisterRecords                 types.Bool   `tfsdk:"no_register_records"`
	TXTSupport                        types.Bool   `tfsdk:"txt_support"`
	CacheFlush                        types.Bool   `tfsdk:"cache_flush"`
	LocalZoneType                     types.String `tfsdk:"local_zone_type"`
	OutgoingInterface                 types.Set    `tfsdk:"outgoing_interface"`
	EnableWPAD                        types.Bool   `tfsdk:"enable_wpad"`
	HideIdentity                      types.Bool   `tfsdk:"hide_identity"`
	HideVersion                       types.Bool   `tfsdk:"hide_version"`
	EnablePrefetchKey                 types.Bool   `tfsdk:"enable_prefetch_key"`
	DNSSECStripped                    types.Bool   `tfsdk:"dnssec_stripped"`
	AggressiveNSEC                    types.Bool   `tfsdk:"aggressive_nsec"`
	QnameMinStrict                    types.Bool   `tfsdk:"qname_min_strict"`
	OutgoingNumTCP                    types.Int64  `tfsdk:"outgoing_num_tcp"`
	IncomingNumTCP                    types.Int64  `tfsdk:"incoming_num_tcp"`
	NumQueriesPerThread               types.Int64  `tfsdk:"num_queries_per_thread"`
	OutgoingRange                     types.Int64  `tfsdk:"outgoing_range"`
	JostleTimeout                     types.Int64  `tfsdk:"jostle_timeout"`
	DiscardTimeout                    types.Int64  `tfsdk:"discard_timeout"`
	PrivateDomains                    types.Set    `tfsdk:"private_domains"`
	PrivateAddresses                  types.Set    `tfsdk:"private_addresses"`
	InsecureDomains                   types.Set    `tfsdk:"insecure_domains"`
	EnablePrefetch                    types.Bool   `tfsdk:"enable_prefetch"`
	UnwantedReplyThreshold            types.Int64  `tfsdk:"unwanted_reply_threshold"`
	MsgCacheSize                      types.String `tfsdk:"msg_cache_size"`
	RRSetCacheSize                    types.String `tfsdk:"rrset_cache_size"`
	MaxTTL                            types.Int64  `tfsdk:"max_ttl"`
	MaxNegativeTTL                    types.Int64  `tfsdk:"max_negative_ttl"`
	MinTTL                            types.Int64  `tfsdk:"min_ttl"`
	HostTTL                           types.Int64  `tfsdk:"host_ttl"`
	KeepProbingHosts                  types.Bool   `tfsdk:"keep_probing_hosts"`
	NumHosts                          types.Int64  `tfsdk:"num_hosts"`
	ExtendedStatistics                types.Bool   `tfsdk:"extended_statistics"`
	LogQueries                        types.Bool   `tfsdk:"log_queries"`
	LogReplies                        types.Bool   `tfsdk:"log_replies"`
	TagQueryReply                     types.Bool   `tfsdk:"tag_query_reply"`
	LogLocalActions                   types.Bool   `tfsdk:"log_local_actions"`
	LogServfail                       types.Bool   `tfsdk:"log_servfail"`
	VerbosityLevel                    types.Int64  `tfsdk:"verbosity_level"`
	ValidationLevel                   types.Int64  `tfsdk:"validation_level"`
	ServeExpiredEnabled               types.Bool   `tfsdk:"serve_expired_enabled"`
	ServeExpiredRecordReplyTTL        types.String `tfsdk:"serve_expired_record_reply_ttl"`
	ServeExpiredTTL                   types.String `tfsdk:"serve_expired_ttl"`
	ServeExpiredResetTTL              types.Bool   `tfsdk:"serve_expired_reset_ttl"`
	ServeExpiredClientResponseTimeout types.String `tfsdk:"serve_expired_client_response_timeout"`
	ACLDefaultAction                  types.String `tfsdk:"acl_default_action"`
	ForwardingEnabled                 types.Bool   `tfsdk:"forwarding_enabled"`
	SafeSearch                        types.Bool   `tfsdk:"safe_search"`
	DNSBLJSON                         types.String `tfsdk:"dnsbl_json"`
	DNSBLItems                        types.List   `tfsdk:"dnsbl_items"`
}

func settingsStateFromGeneral(model GeneralResourceModel) settingsDataSourceModel {
	var state settingsDataSourceModel
	source := reflect.ValueOf(model)
	destination := reflect.ValueOf(&state).Elem()
	for i := 0; i < source.NumField(); i++ {
		field := destination.FieldByName(source.Type().Field(i).Name)
		if field.IsValid() && field.CanSet() {
			field.Set(source.Field(i))
		}
	}
	return state
}

func (d *settingsDataSource) Read(ctx context.Context, _ datasource.ReadRequest, resp *datasource.ReadResponse) {
	if err := requireUnboundParityVersion(ctx, d.client); err != nil {
		resp.Diagnostics.AddError("Unsupported OPNsense version", err.Error())
		return
	}
	settings, err := opnsense.GetSingleton[unboundSettingsEnvelope](ctx, d.client, generalReqOpts)
	if err != nil {
		resp.Diagnostics.AddError("Error reading the complete Unbound settings", err.Error())
		return
	}
	general, err := flattenGeneralSettings(settings)
	if err != nil {
		resp.Diagnostics.AddError("Error decoding the complete Unbound settings", err.Error())
		return
	}
	var model GeneralResourceModel
	model.fromAPI(ctx, general, "unbound-settings")
	state := settingsStateFromGeneral(model)
	state.ID = types.StringValue("unbound-settings")
	var generalBranch map[string]json.RawMessage
	if err := json.Unmarshal(settings.General, &generalBranch); err != nil {
		resp.Diagnostics.AddError("Error decoding the complete Unbound settings", fmt.Sprintf("malformed general branch: %s", err))
		return
	}
	var safeSearch string
	if raw, ok := generalBranch["safesearch"]; !ok || json.Unmarshal(raw, &safeSearch) != nil || (safeSearch != "0" && safeSearch != "1") {
		resp.Diagnostics.AddError("Error decoding the complete Unbound settings", "malformed Unbound settings response: general.safesearch is absent or invalid")
		return
	}
	state.SafeSearch = types.BoolValue(opnsense.StringToBool(safeSearch))
	if len(settings.DNSBL) == 0 || string(settings.DNSBL) == "null" {
		resp.Diagnostics.AddError("Error decoding the complete Unbound settings", "malformed Unbound settings response: missing dnsbl branch")
		return
	}
	canonical, err := canonicalJSON(settings.DNSBL)
	if err != nil {
		resp.Diagnostics.AddError("Error decoding the complete Unbound settings", err.Error())
		return
	}
	state.DNSBLJSON = types.StringValue(string(canonical))
	items, err := decodeDNSBLItems(settings.DNSBL)
	if err != nil {
		resp.Diagnostics.AddError("Error decoding the complete Unbound settings", err.Error())
		return
	}
	state.DNSBLItems = dnsblSettingsItemsValue(items)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
