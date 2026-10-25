// Copyright (c) Matthew Mellor
// SPDX-License-Identifier: MPL-2.0

package dnsmasq

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"

	"github.com/matthew-on-git/terraform-provider-opnsense/internal/tfconv"
	"github.com/matthew-on-git/terraform-provider-opnsense/pkg/opnsense"
)

// The nested branch is explicit: scalar configuration and selector option maps
// must not be confused, and response-only %domain never enters the request.
type settingsDHCPResponse struct {
	NoInterface    opnsense.SelectedMapList `json:"no_interface"`
	FQDN           string                   `json:"fqdn"`
	Domain         string                   `json:"domain"`
	Local          string                   `json:"local"`
	LeaseMax       string                   `json:"lease_max"`
	Authoritative  string                   `json:"authoritative"`
	DefaultFWRules string                   `json:"default_fw_rules"`
	ReplyDelay     string                   `json:"reply_delay"`
	EnableRA       string                   `json:"enable_ra"`
	HostPing       string                   `json:"host_ping"`
	NoSync         string                   `json:"nosync"`
	LogDHCP        string                   `json:"log_dhcp"`
	LogQuiet       string                   `json:"log_quiet"`
}

type settingsDHCPRequest struct {
	NoInterface    *string `json:"no_interface,omitempty"`
	FQDN           *string `json:"fqdn,omitempty"`
	Domain         *string `json:"domain,omitempty"`
	Local          *string `json:"local,omitempty"`
	LeaseMax       *string `json:"lease_max,omitempty"`
	Authoritative  *string `json:"authoritative,omitempty"`
	DefaultFWRules *string `json:"default_fw_rules,omitempty"`
	ReplyDelay     *string `json:"reply_delay,omitempty"`
	EnableRA       *string `json:"enable_ra,omitempty"`
	HostPing       *string `json:"host_ping,omitempty"`
	NoSync         *string `json:"nosync,omitempty"`
	LogDHCP        *string `json:"log_dhcp,omitempty"`
	LogQuiet       *string `json:"log_quiet,omitempty"`
}

func dhcpSchema() schema.SingleNestedAttribute {
	return schema.SingleNestedAttribute{Optional: true, Computed: true, MarkdownDescription: "Typed DHCP/RA settings. Omitted fields preserve appliance configuration; explicit false, zero, empty domain and empty interface set are intentional updates.", Attributes: map[string]schema.Attribute{
		"no_interface":     schema.SetAttribute{Optional: true, Computed: true, MarkdownDescription: "Interfaces excluded from DHCP and router advertisements (empty = none).", ElementType: types.StringType},
		"fqdn":             schema.BoolAttribute{Optional: true, Computed: true, MarkdownDescription: "Register DHCP client names as fully qualified names."},
		"domain":           schema.StringAttribute{Optional: true, Computed: true, MarkdownDescription: "DHCP domain; empty uses the appliance default. This is not response metadata %domain."},
		"local":            schema.BoolAttribute{Optional: true, Computed: true, MarkdownDescription: "Do not forward queries for the DHCP domain."},
		"lease_max":        schema.Int64Attribute{Optional: true, Computed: true, MarkdownDescription: "Maximum DHCP leases; null preserves the appliance setting.", Validators: []validator.Int64{int64validator.AtLeast(0)}},
		"authoritative":    schema.BoolAttribute{Optional: true, Computed: true, MarkdownDescription: "Act as the authoritative DHCP server."},
		"default_fw_rules": schema.BoolAttribute{Optional: true, Computed: true, MarkdownDescription: "Generate default DHCP firewall rules."},
		"reply_delay":      schema.Int64Attribute{Optional: true, Computed: true, MarkdownDescription: "Delay DHCP replies by 0–60 seconds.", Validators: []validator.Int64{int64validator.Between(0, 60)}},
		"enable_ra":        schema.BoolAttribute{Optional: true, Computed: true, MarkdownDescription: "Enable IPv6 router advertisements."},
		"host_ping":        schema.BoolAttribute{Optional: true, Computed: true, MarkdownDescription: "Check address availability before allocating a lease."},
		"nosync":           schema.BoolAttribute{Optional: true, Computed: true, MarkdownDescription: "Disable configuration synchronization for DHCP settings."},
		"log_dhcp":         schema.BoolAttribute{Optional: true, Computed: true, MarkdownDescription: "Log DHCP options and tags; cannot be enabled with log_quiet."},
		"log_quiet":        schema.BoolAttribute{Optional: true, Computed: true, MarkdownDescription: "Suppress routine DHCP logging; cannot be enabled with log_dhcp."},
	}}
}

func dhcpAttributeTypes() map[string]attr.Type {
	return dhcpSchema().GetType().(types.ObjectType).AttrTypes
}

func (a settingsDHCPResponse) toTerraform() types.Object {
	values := map[string]attr.Value{
		"no_interface":     tfconv.SliceToSet(a.NoInterface),
		"fqdn":             types.BoolValue(a.FQDN == "1"),
		"domain":           types.StringValue(a.Domain),
		"local":            types.BoolValue(a.Local == "1"),
		"lease_max":        dhcpInteger(a.LeaseMax),
		"authoritative":    types.BoolValue(a.Authoritative == "1"),
		"default_fw_rules": types.BoolValue(a.DefaultFWRules == "1"),
		"reply_delay":      dhcpInteger(a.ReplyDelay),
		"enable_ra":        types.BoolValue(a.EnableRA == "1"),
		"host_ping":        types.BoolValue(a.HostPing == "1"),
		"nosync":           types.BoolValue(a.NoSync == "1"),
		"log_dhcp":         types.BoolValue(a.LogDHCP == "1"),
		"log_quiet":        types.BoolValue(a.LogQuiet == "1"),
	}
	return types.ObjectValueMust(dhcpAttributeTypes(), values)
}

// Optional integers are null for the API empty-string sentinel, never zero.
func dhcpInteger(s string) types.Int64 {
	if s == "" {
		return types.Int64Null()
	}
	n, _ := strconv.ParseInt(s, 10, 64) // validated by UnmarshalJSON
	return types.Int64Value(n)
}

func dhcpRequest(ctx context.Context, value types.Object) *settingsDHCPRequest {
	if value.IsNull() || value.IsUnknown() {
		return nil
	}
	attrs := value.Attributes()
	out := &settingsDHCPRequest{}
	if v, ok := attrs["no_interface"].(types.Set); ok && !v.IsNull() && !v.IsUnknown() {
		// A known set may still contain unresolved elements. Never convert a
		// failed/partial collection conversion into an intentional clear.
		known := true
		for _, element := range v.Elements() {
			known = known && !element.IsNull() && !element.IsUnknown()
		}
		if known {
			s := tfconv.SetToCSV(ctx, v)
			out.NoInterface = &s
		}
	}
	if v, ok := attrs["fqdn"].(types.Bool); ok && !v.IsNull() && !v.IsUnknown() {
		s := opnsense.BoolToString(v.ValueBool())
		out.FQDN = &s
	}
	if v, ok := attrs["domain"].(types.String); ok && !v.IsNull() && !v.IsUnknown() {
		s := v.ValueString()
		out.Domain = &s
	}
	if v, ok := attrs["local"].(types.Bool); ok && !v.IsNull() && !v.IsUnknown() {
		s := opnsense.BoolToString(v.ValueBool())
		out.Local = &s
	}
	if v, ok := attrs["lease_max"].(types.Int64); ok && !v.IsNull() && !v.IsUnknown() {
		s := strconv.FormatInt(v.ValueInt64(), 10)
		out.LeaseMax = &s
	}
	if v, ok := attrs["authoritative"].(types.Bool); ok && !v.IsNull() && !v.IsUnknown() {
		s := opnsense.BoolToString(v.ValueBool())
		out.Authoritative = &s
	}
	if v, ok := attrs["default_fw_rules"].(types.Bool); ok && !v.IsNull() && !v.IsUnknown() {
		s := opnsense.BoolToString(v.ValueBool())
		out.DefaultFWRules = &s
	}
	if v, ok := attrs["reply_delay"].(types.Int64); ok && !v.IsNull() && !v.IsUnknown() {
		s := strconv.FormatInt(v.ValueInt64(), 10)
		out.ReplyDelay = &s
	}
	if v, ok := attrs["enable_ra"].(types.Bool); ok && !v.IsNull() && !v.IsUnknown() {
		s := opnsense.BoolToString(v.ValueBool())
		out.EnableRA = &s
	}
	if v, ok := attrs["host_ping"].(types.Bool); ok && !v.IsNull() && !v.IsUnknown() {
		s := opnsense.BoolToString(v.ValueBool())
		out.HostPing = &s
	}
	if v, ok := attrs["nosync"].(types.Bool); ok && !v.IsNull() && !v.IsUnknown() {
		s := opnsense.BoolToString(v.ValueBool())
		out.NoSync = &s
	}
	if v, ok := attrs["log_dhcp"].(types.Bool); ok && !v.IsNull() && !v.IsUnknown() {
		s := opnsense.BoolToString(v.ValueBool())
		out.LogDHCP = &s
	}
	if v, ok := attrs["log_quiet"].(types.Bool); ok && !v.IsNull() && !v.IsUnknown() {
		s := opnsense.BoolToString(v.ValueBool())
		out.LogQuiet = &s
	}
	return out
}

// Fail closed for missing or malformed DHCP branches rather than publishing an
// authoritative empty state. Unknown response metadata remains harmless.
func (a *settingsAPIResponse) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	if v, ok := raw["dhcp"]; !ok || bytes.Equal(bytes.TrimSpace(v), []byte("null")) {
		return fmt.Errorf("dnsmasq response is missing nested dhcp settings")
	}
	type plain settingsAPIResponse
	return json.Unmarshal(data, (*plain)(a))
}

func (a *settingsDHCPResponse) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("invalid nested DHCP settings: %w", err)
	}
	if v, ok := raw["no_interface"]; !ok || bytes.Equal(bytes.TrimSpace(v), []byte("null")) {
		return fmt.Errorf("DHCP response is missing no_interface")
	}
	if v, ok := raw["fqdn"]; !ok || bytes.Equal(bytes.TrimSpace(v), []byte("null")) {
		return fmt.Errorf("DHCP response is missing fqdn")
	}
	if v, ok := raw["domain"]; !ok || bytes.Equal(bytes.TrimSpace(v), []byte("null")) {
		return fmt.Errorf("DHCP response is missing domain")
	}
	if v, ok := raw["local"]; !ok || bytes.Equal(bytes.TrimSpace(v), []byte("null")) {
		return fmt.Errorf("DHCP response is missing local")
	}
	if v, ok := raw["lease_max"]; !ok || bytes.Equal(bytes.TrimSpace(v), []byte("null")) {
		return fmt.Errorf("DHCP response is missing lease_max")
	}
	if v, ok := raw["authoritative"]; !ok || bytes.Equal(bytes.TrimSpace(v), []byte("null")) {
		return fmt.Errorf("DHCP response is missing authoritative")
	}
	if v, ok := raw["default_fw_rules"]; !ok || bytes.Equal(bytes.TrimSpace(v), []byte("null")) {
		return fmt.Errorf("DHCP response is missing default_fw_rules")
	}
	if v, ok := raw["reply_delay"]; !ok || bytes.Equal(bytes.TrimSpace(v), []byte("null")) {
		return fmt.Errorf("DHCP response is missing reply_delay")
	}
	if v, ok := raw["enable_ra"]; !ok || bytes.Equal(bytes.TrimSpace(v), []byte("null")) {
		return fmt.Errorf("DHCP response is missing enable_ra")
	}
	if v, ok := raw["host_ping"]; !ok || bytes.Equal(bytes.TrimSpace(v), []byte("null")) {
		return fmt.Errorf("DHCP response is missing host_ping")
	}
	if v, ok := raw["nosync"]; !ok || bytes.Equal(bytes.TrimSpace(v), []byte("null")) {
		return fmt.Errorf("DHCP response is missing nosync")
	}
	if v, ok := raw["log_dhcp"]; !ok || bytes.Equal(bytes.TrimSpace(v), []byte("null")) {
		return fmt.Errorf("DHCP response is missing log_dhcp")
	}
	if v, ok := raw["log_quiet"]; !ok || bytes.Equal(bytes.TrimSpace(v), []byte("null")) {
		return fmt.Errorf("DHCP response is missing log_quiet")
	}
	if err := validateDHCPSelector(raw["no_interface"]); err != nil {
		return err
	}
	type plain settingsDHCPResponse
	var decoded plain
	if err := json.Unmarshal(data, &decoded); err != nil {
		return fmt.Errorf("invalid DHCP setting: %w", err)
	}
	if decoded.FQDN != "0" && decoded.FQDN != "1" {
		return fmt.Errorf("invalid DHCP boolean fqdn: %q", decoded.FQDN)
	}
	if decoded.Local != "0" && decoded.Local != "1" {
		return fmt.Errorf("invalid DHCP boolean local: %q", decoded.Local)
	}
	if decoded.LeaseMax != "" {
		n, err := strconv.ParseInt(decoded.LeaseMax, 10, 64)
		if err != nil || n < 0 {
			return fmt.Errorf("invalid DHCP integer lease_max: %q", decoded.LeaseMax)
		}
	}
	if decoded.Authoritative != "0" && decoded.Authoritative != "1" {
		return fmt.Errorf("invalid DHCP boolean authoritative: %q", decoded.Authoritative)
	}
	if decoded.DefaultFWRules != "0" && decoded.DefaultFWRules != "1" {
		return fmt.Errorf("invalid DHCP boolean default_fw_rules: %q", decoded.DefaultFWRules)
	}
	if decoded.ReplyDelay != "" {
		n, err := strconv.ParseInt(decoded.ReplyDelay, 10, 64)
		if err != nil || n < 0 || n > 60 {
			return fmt.Errorf("invalid DHCP integer reply_delay: %q", decoded.ReplyDelay)
		}
	}
	if decoded.EnableRA != "0" && decoded.EnableRA != "1" {
		return fmt.Errorf("invalid DHCP boolean enable_ra: %q", decoded.EnableRA)
	}
	if decoded.HostPing != "0" && decoded.HostPing != "1" {
		return fmt.Errorf("invalid DHCP boolean host_ping: %q", decoded.HostPing)
	}
	if decoded.NoSync != "0" && decoded.NoSync != "1" {
		return fmt.Errorf("invalid DHCP boolean nosync: %q", decoded.NoSync)
	}
	if decoded.LogDHCP != "0" && decoded.LogDHCP != "1" {
		return fmt.Errorf("invalid DHCP boolean log_dhcp: %q", decoded.LogDHCP)
	}
	if decoded.LogQuiet != "0" && decoded.LogQuiet != "1" {
		return fmt.Errorf("invalid DHCP boolean log_quiet: %q", decoded.LogQuiet)
	}
	if decoded.LogDHCP == "1" && decoded.LogQuiet == "1" {
		return fmt.Errorf("DHCP log_dhcp and log_quiet cannot both be enabled")
	}
	*a = settingsDHCPResponse(decoded)
	return nil
}

func validateDHCPSelector(data []byte) error {
	trimmed := bytes.TrimSpace(data)
	if bytes.Equal(trimmed, []byte("[]")) {
		return nil
	}
	var options map[string]struct {
		Value    *string         `json:"value"`
		Selected json.RawMessage `json:"selected"`
	}
	if err := json.Unmarshal(data, &options); err != nil {
		return fmt.Errorf("invalid DHCP no_interface selector: %w", err)
	}
	for key, option := range options {
		selected := string(bytes.TrimSpace(option.Selected))
		if key == "" || option.Value == nil || (selected != "0" && selected != "1" && selected != `"0"` && selected != `"1"`) {
			return fmt.Errorf("invalid DHCP no_interface option %q", key)
		}
	}
	return nil
}

var (
	_ resource.ResourceWithUpgradeState   = &settingsResource{}
	_ resource.ResourceWithValidateConfig = &settingsResource{}
)

func (r *settingsResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var value types.Object
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("dhcp"), &value)...)
	if resp.Diagnostics.HasError() || value.IsNull() || value.IsUnknown() {
		return
	}
	attrs := value.Attributes()
	logDHCP, _ := attrs["log_dhcp"].(types.Bool)
	logQuiet, _ := attrs["log_quiet"].(types.Bool)
	if logDHCP.ValueBool() && logQuiet.ValueBool() {
		resp.Diagnostics.AddAttributeError(path.Root("dhcp").AtName("log_quiet"), "Conflicting DHCP logging settings", "log_dhcp and log_quiet cannot both be enabled.")
	}
}

// No network access or guessed configuration is used during state migration.
// Empty legacy sets contained no DHCP values; refresh fills the null object.
func (r *settingsResource) UpgradeState(_ context.Context) map[int64]resource.StateUpgrader {
	return map[int64]resource.StateUpgrader{0: {StateUpgrader: upgradeDHCPState}}
}

func upgradeDHCPState(ctx context.Context, req resource.UpgradeStateRequest, resp *resource.UpgradeStateResponse) {
	if req.RawState == nil {
		resp.Diagnostics.AddError("Missing legacy Dnsmasq state", "Cannot upgrade absent state.")
		return
	}
	var state map[string]json.RawMessage
	if err := json.Unmarshal(req.RawState.JSON, &state); err != nil || state == nil {
		resp.Diagnostics.AddError("Invalid legacy Dnsmasq state", "Expected a JSON state object.")
		return
	}
	if raw, ok := state["dhcp"]; ok {
		var flags []string
		if err := json.Unmarshal(raw, &flags); err != nil || len(flags) != 0 {
			resp.Diagnostics.AddError("Cannot infer legacy DHCP settings", "The old dhcp set was not a valid model of DHCP configuration. Back up state and appliance configuration, remove the legacy dhcp argument, remove only this singleton from state and import it again as dnsmasq; review the refreshed typed object before applying. Import and state removal do not change appliance settings.")
			return
		}
	}
	state["dhcp"] = json.RawMessage("null")
	var schemaResp resource.SchemaResponse
	(&settingsResource{}).Schema(ctx, resource.SchemaRequest{}, &schemaResp)
	for name := range schemaResp.Schema.Attributes {
		if _, ok := state[name]; !ok {
			state[name] = json.RawMessage("null")
		}
	}
	data, err := json.Marshal(state)
	if err != nil {
		resp.Diagnostics.AddError("Error upgrading Dnsmasq state", err.Error())
		return
	}
	value, err := (&tfprotov6.DynamicValue{JSON: data}).Unmarshal(resp.State.Schema.Type().TerraformType(ctx))
	if err != nil {
		resp.Diagnostics.AddError("Error upgrading Dnsmasq state", err.Error())
		return
	}
	resp.State.Raw = value
}
