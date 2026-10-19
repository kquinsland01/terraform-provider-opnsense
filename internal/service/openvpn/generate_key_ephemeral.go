// Copyright (c) Matthew Mellor
// SPDX-License-Identifier: MPL-2.0

package openvpn

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/ephemeral"
	"github.com/hashicorp/terraform-plugin-framework/ephemeral/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/matthew-on-git/terraform-provider-opnsense/pkg/opnsense"
)

var (
	_ ephemeral.EphemeralResource              = &generateKeyEphemeral{}
	_ ephemeral.EphemeralResourceWithConfigure = &generateKeyEphemeral{}
)

type (
	generateKeyEphemeral      struct{ client *opnsense.Client }
	generateKeyEphemeralModel struct {
		KeyType types.String `tfsdk:"key_type"`
		Key     types.String `tfsdk:"key"`
	}
)

func newGenerateKeyEphemeral() ephemeral.EphemeralResource { return &generateKeyEphemeral{} }
func (r *generateKeyEphemeral) Metadata(_ context.Context, req ephemeral.MetadataRequest, resp *ephemeral.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_openvpn_generate_key"
}

func (r *generateKeyEphemeral) Schema(_ context.Context, _ ephemeral.SchemaRequest, resp *ephemeral.SchemaResponse) {
	resp.Schema = schema.Schema{MarkdownDescription: "Generates OpenVPN key material in memory using OPNsense 26.7.6 or newer. Ephemeral results are not stored in Terraform state and may only feed ephemeral or write-only consumers.", Attributes: map[string]schema.Attribute{"key_type": schema.StringAttribute{Optional: true, MarkdownDescription: "Key type: `secret`, `auth-token`, `tls-auth`, `tls-crypt`, or `tls-crypt-v2-server`. Defaults to `secret`.", Validators: []validator.String{stringvalidator.OneOf("secret", "auth-token", "tls-auth", "tls-crypt", "tls-crypt-v2-server")}}, "key": schema.StringAttribute{Computed: true, Sensitive: true, MarkdownDescription: "Fresh generated key material, available only for the current operation."}}}
}

func (r *generateKeyEphemeral) Configure(_ context.Context, req ephemeral.ConfigureRequest, resp *ephemeral.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	client, ok := req.ProviderData.(*opnsense.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected Provider Data", fmt.Sprintf("Expected *opnsense.Client, got %T.", req.ProviderData))
		return
	}
	r.client = client
}

func (r *generateKeyEphemeral) Open(ctx context.Context, req ephemeral.OpenRequest, resp *ephemeral.OpenResponse) {
	var data generateKeyEphemeralModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if data.KeyType.IsUnknown() {
		resp.Diagnostics.AddError("Invalid OpenVPN key type", "key_type must be a known supported value.")
		return
	}
	keyType := "secret"
	if !data.KeyType.IsNull() {
		keyType = data.KeyType.ValueString()
	} else {
		data.KeyType = types.StringValue(keyType)
	}
	if err := requireOpenVPNParityVersion(ctx, r.client); err != nil {
		resp.Diagnostics.AddError("Unsupported OpenVPN parity surface", err.Error())
		return
	}
	endpoint := "/api/openvpn/instances/gen_key/" + keyType
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, r.client.BaseURL()+endpoint, nil)
	if err != nil {
		resp.Diagnostics.AddError("Error generating OpenVPN key", "Unable to construct the key-generation request.")
		return
	}
	if err := r.client.LockMutex(ctx); err != nil {
		resp.Diagnostics.AddError("Error generating OpenVPN key", "Unable to acquire the provider request slot.")
		return
	}
	defer r.client.UnlockMutex()
	httpResp, err := r.client.HTTPClient().Do(request)
	if err != nil {
		resp.Diagnostics.AddError("Error generating OpenVPN key", fmt.Sprintf("Request to %s failed: %s", endpoint, err))
		return
	}
	defer func() { _ = httpResp.Body.Close() }()
	if httpResp.StatusCode < 200 || httpResp.StatusCode >= 300 {
		resp.Diagnostics.AddError("Error generating OpenVPN key", fmt.Sprintf("%s returned HTTP %d", endpoint, httpResp.StatusCode))
		return
	}
	var result struct {
		Result string `json:"result"`
		Key    string `json:"key"`
	}
	decoder := json.NewDecoder(httpResp.Body)
	if err := decoder.Decode(&result); err != nil {
		resp.Diagnostics.AddError("Malformed OpenVPN key response", "The appliance returned malformed JSON; response content was not logged.")
		return
	}
	var trailing any
	if err := decoder.Decode(&trailing); err == nil {
		resp.Diagnostics.AddError("Malformed OpenVPN key response", "The appliance returned more than one JSON document; response content was not logged.")
		return
	} else if err != io.EOF {
		resp.Diagnostics.AddError("Malformed OpenVPN key response", "The appliance returned trailing non-JSON data; response content was not logged.")
		return
	}
	if result.Result != "ok" {
		resp.Diagnostics.AddError("OpenVPN key generation failed", fmt.Sprintf("The appliance returned result %q for key type %q; response content was not logged.", result.Result, keyType))
		return
	}
	if strings.TrimSpace(result.Key) == "" {
		resp.Diagnostics.AddError("Malformed OpenVPN key response", "The appliance returned an empty key.")
		return
	}
	data.Key = types.StringValue(result.Key)
	resp.Diagnostics.Append(resp.Result.Set(ctx, &data)...)
}
