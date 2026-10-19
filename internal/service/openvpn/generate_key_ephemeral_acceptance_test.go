// Copyright (c) Matthew Mellor
// SPDX-License-Identifier: MPL-2.0

package openvpn_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	frameworkprovider "github.com/hashicorp/terraform-plugin-framework/provider"
	providerschema "github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	frameworkresource "github.com/hashicorp/terraform-plugin-framework/resource"
	resourceschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/hashicorp/terraform-plugin-testing/tfversion"

	"github.com/matthew-on-git/terraform-provider-opnsense/internal/acctest"
)

// TestAccOpenVPNGenerateKeyEphemeralProtocol exercises provider configuration
// handoff and all source-supported key types through Terraform Core protocol.
// A test-only managed resource consumes each key through a write-only
// attribute, forcing Terraform Core to open all five ephemeral resources. The
// check rejects any saved-plan/state/log copy of the generated key material.
func TestAccOpenVPNGenerateKeyEphemeralProtocol(t *testing.T) {
	if os.Getenv("OPNSENSE_OPENVPN_PARITY") != "1" {
		t.Skip("set OPNSENSE_OPENVPN_PARITY=1 only for the independently verified disposable OPNsense target")
	}
	sinkValues.reset()
	factories := make(map[string]func() (tfprotov6.ProviderServer, error), len(acctest.ProtoV6ProviderFactories)+1)
	for name, factory := range acctest.ProtoV6ProviderFactories {
		factories[name] = factory
	}
	factories["story31c"] = providerserver.NewProtocol6WithError(&story31cSinkProvider{})
	workingDir := os.Getenv("OPNSENSE_OPENVPN_TF_WORKDIR")
	if workingDir == "" {
		t.Fatal("OPNSENSE_OPENVPN_TF_WORKDIR is required for saved-plan/state artifact proof")
	}
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		WorkingDir:               workingDir,
		TerraformVersionChecks:   []tfversion.TerraformVersionCheck{tfversion.SkipBelow(tfversion.Version1_10_0)},
		PreCheck: func() {
			acctest.PreCheck(t)
			if os.Getenv("OPNSENSE_OPENVPN_PARITY") != "1" {
				t.Skip("set OPNSENSE_OPENVPN_PARITY=1 only for the independently verified disposable OPNsense target")
			}
			if os.Getenv("OPNSENSE_DISPOSABLE_VM_NAME") == "" || os.Getenv("OPNSENSE_DISPOSABLE_VM_UUID") == "" || os.Getenv("OPNSENSE_OPENVPN_IDENTITY_ATTESTATION") == "" {
				t.Fatal("disposable VM name/UUID and runner-produced identity attestation are required")
			}
			if os.Getenv("OPNSENSE_OPENVPN_IDENTITY_ATTESTATION") != os.Getenv("OPNSENSE_DISPOSABLE_VM_NAME")+":"+os.Getenv("OPNSENSE_DISPOSABLE_VM_UUID") {
				t.Fatal("identity attestation does not match the disposable VM name and UUID")
			}
		},
		Steps: []resource.TestStep{{Config: openVPNEphemeralProtocolConfig(), Check: func(state *terraform.State) error {
			if len(state.RootModule().Resources) != 5 {
				return fmt.Errorf("ephemeral protocol test persisted %d resources, want five test-only sinks", len(state.RootModule().Resources))
			}
			encoded, err := json.Marshal(state)
			if err != nil {
				return err
			}
			lower := strings.ToLower(string(encoded))
			for _, marker := range []string{"begin openvpn static key"} {
				if strings.Contains(lower, marker) {
					return fmt.Errorf("ephemeral marker %q persisted in Terraform state", marker)
				}
			}
			values := sinkValues.snapshot()
			if len(values) != 5 {
				return fmt.Errorf("test-only write-only sinks consumed %d generated keys, want five", len(values))
			}
			for _, value := range values {
				if strings.Contains(string(encoded), value) {
					return fmt.Errorf("generated key material persisted in Terraform state")
				}
			}
			if err := scanEphemeralArtifacts(workingDir, values); err != nil {
				return err
			}
			return scanEphemeralLog(os.Getenv("TF_LOG_PATH"), values)
		}}},
	})
}

func openVPNEphemeralProtocolConfig() string {
	var config strings.Builder
	for index, keyType := range []string{"secret", "auth-token", "tls-auth", "tls-crypt", "tls-crypt-v2-server"} {
		name := strings.ReplaceAll(keyType, "-", "_")
		fmt.Fprintf(&config, "ephemeral \"opnsense_openvpn_generate_key\" %q {\n  key_type = %q\n}\nresource \"story31c_sink\" \"k%d\" {\n  value = ephemeral.opnsense_openvpn_generate_key.%s.key\n}\n", name, keyType, index, name)
	}
	return config.String()
}

func TestOpenVPNEphemeralProtocolConfigUsesFiveWriteOnlyConsumers(t *testing.T) {
	t.Parallel()
	config := openVPNEphemeralProtocolConfig()
	if strings.Count(config, `ephemeral "opnsense_openvpn_generate_key"`) != 5 {
		t.Fatalf("expected five ephemeral blocks:\n%s", config)
	}
	if strings.Contains(config, "output ") {
		t.Fatalf("root ephemeral outputs are invalid and must not be generated:\n%s", config)
	}
	if strings.Count(config, `resource "story31c_sink"`) != 5 || strings.Count(config, "value = ephemeral.opnsense_openvpn_generate_key.") != 5 {
		t.Fatalf("expected five write-only ephemeral consumers:\n%s", config)
	}
	for _, keyType := range []string{"secret", "auth-token", "tls-auth", "tls-crypt", "tls-crypt-v2-server"} {
		if !strings.Contains(config, fmt.Sprintf("key_type = %q", keyType)) {
			t.Errorf("missing ephemeral key type %q", keyType)
		}
	}
}

type story31cSinkProvider struct{}

func (*story31cSinkProvider) Metadata(_ context.Context, _ frameworkprovider.MetadataRequest, response *frameworkprovider.MetadataResponse) {
	response.TypeName = "story31c"
}

func (*story31cSinkProvider) Schema(_ context.Context, _ frameworkprovider.SchemaRequest, response *frameworkprovider.SchemaResponse) {
	response.Schema = providerschema.Schema{}
}

func (*story31cSinkProvider) Configure(context.Context, frameworkprovider.ConfigureRequest, *frameworkprovider.ConfigureResponse) {
}

func (*story31cSinkProvider) DataSources(context.Context) []func() datasource.DataSource { return nil }

func (*story31cSinkProvider) Resources(context.Context) []func() frameworkresource.Resource {
	return []func() frameworkresource.Resource{func() frameworkresource.Resource { return &story31cSinkResource{} }}
}

type (
	story31cSinkResource struct{}
	story31cSinkModel    struct {
		ID    types.String `tfsdk:"id"`
		Value types.String `tfsdk:"value"`
	}
)

func (*story31cSinkResource) Metadata(_ context.Context, request frameworkresource.MetadataRequest, response *frameworkresource.MetadataResponse) {
	response.TypeName = request.ProviderTypeName + "_sink"
}

func (*story31cSinkResource) Schema(_ context.Context, _ frameworkresource.SchemaRequest, response *frameworkresource.SchemaResponse) {
	response.Schema = resourceschema.Schema{Attributes: map[string]resourceschema.Attribute{
		"id":    resourceschema.StringAttribute{Computed: true},
		"value": resourceschema.StringAttribute{Required: true, Sensitive: true, WriteOnly: true},
	}}
}

func (*story31cSinkResource) Create(ctx context.Context, request frameworkresource.CreateRequest, response *frameworkresource.CreateResponse) {
	consumeStory31cValue(ctx, request.Config, &response.State, &response.Diagnostics)
}

func (*story31cSinkResource) Read(context.Context, frameworkresource.ReadRequest, *frameworkresource.ReadResponse) {
}

func (*story31cSinkResource) Update(ctx context.Context, request frameworkresource.UpdateRequest, response *frameworkresource.UpdateResponse) {
	consumeStory31cValue(ctx, request.Config, &response.State, &response.Diagnostics)
}

func (*story31cSinkResource) Delete(context.Context, frameworkresource.DeleteRequest, *frameworkresource.DeleteResponse) {
}

func consumeStory31cValue(ctx context.Context, config tfsdk.Config, state *tfsdk.State, diagnostics *diag.Diagnostics) {
	var model story31cSinkModel
	diagnostics.Append(config.Get(ctx, &model)...)
	if diagnostics.HasError() {
		return
	}
	value := model.Value.ValueString()
	if strings.TrimSpace(value) == "" {
		diagnostics.AddError("Empty generated key", "The write-only sink received an empty generated key.")
		return
	}
	sinkValues.add(value)
	model.ID = types.StringValue("accepted")
	model.Value = types.StringNull()
	diagnostics.Append(state.Set(ctx, &model)...)
}

var sinkValues = &story31cValueSet{values: make(map[string]struct{})}

type story31cValueSet struct {
	sync.Mutex
	values map[string]struct{}
}

func (s *story31cValueSet) reset() {
	s.Lock()
	defer s.Unlock()
	s.values = make(map[string]struct{})
}

func (s *story31cValueSet) add(value string) {
	s.Lock()
	defer s.Unlock()
	s.values[value] = struct{}{}
}

func (s *story31cValueSet) snapshot() []string {
	s.Lock()
	defer s.Unlock()
	values := make([]string, 0, len(s.values))
	for value := range s.values {
		values = append(values, value)
	}
	return values
}

func scanEphemeralArtifacts(root string, generatedValues []string) error {
	rooted, err := os.OpenRoot(root)
	if err != nil {
		return err
	}
	defer func() { _ = rooted.Close() }()
	found := map[string]bool{"terraform.tfstate": false, "tfplan": false}
	err = fs.WalkDir(rooted.FS(), ".", func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		if _, required := found[entry.Name()]; required {
			info, err := entry.Info()
			if err != nil {
				return err
			}
			if !info.Mode().IsRegular() {
				return fmt.Errorf("required Terraform persistence artifact %s is not a regular file", entry.Name())
			}
			found[entry.Name()] = true
		}
		contents, err := rooted.ReadFile(path)
		if err != nil {
			return err
		}
		lower := strings.ToLower(string(contents))
		if strings.Contains(lower, "begin openvpn static key") {
			return fmt.Errorf("ephemeral marker persisted in Terraform artifact %s", filepath.Base(path))
		}
		for _, value := range generatedValues {
			if strings.Contains(string(contents), value) {
				return fmt.Errorf("generated key material persisted in Terraform artifact %s", filepath.Base(path))
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	for _, required := range []string{"terraform.tfstate", "tfplan"} {
		if !found[required] {
			return fmt.Errorf("required Terraform persistence artifact %s is unavailable beneath the isolated working directory", required)
		}
	}
	return nil
}

func scanEphemeralLog(path string, generatedValues []string) error {
	if !filepath.IsAbs(path) {
		return fmt.Errorf("TF_LOG_PATH must be an absolute path for persistence proof")
	}
	rooted, err := os.OpenRoot("/")
	if err != nil {
		return err
	}
	defer func() { _ = rooted.Close() }()
	contents, err := rooted.ReadFile(strings.TrimPrefix(filepath.Clean(path), "/"))
	if err != nil {
		return fmt.Errorf("Terraform log is unavailable for persistence proof: %w", err)
	}
	lower := strings.ToLower(string(contents))
	if strings.Contains(lower, "begin openvpn static key") {
		return fmt.Errorf("ephemeral key marker persisted in Terraform log")
	}
	for _, value := range generatedValues {
		if strings.Contains(string(contents), value) {
			return fmt.Errorf("generated key material persisted in Terraform log")
		}
	}
	return nil
}
