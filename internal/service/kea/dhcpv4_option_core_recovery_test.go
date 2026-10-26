// Copyright (c) Matthew Mellor
// SPDX-License-Identifier: MPL-2.0

package kea_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"os/exec"
	"regexp"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/matthew-on-git/terraform-provider-opnsense/internal/provider"
)

// UnitTest runs actual Terraform Core against only a loopback synthetic API.
// Each step supplies a fresh protocol server, while Core persists state/private
// bytes in its real local backend across failed applies and normal refreshes.
func TestDHCPv4OptionCoreErrorPrivatePersistenceAndRefreshBeforeDestroy(t *testing.T) {
	binary, err := exec.LookPath("terraform")
	if err != nil {
		t.Fatal("local Terraform Core binary is required", err)
	}
	t.Setenv("TF_ACC_TERRAFORM_PATH", binary)
	for _, failure := range []string{"save activation", "save unknown match", "save unknown mismatch", "delete activation", "delete unknown present"} {
		t.Run(failure, func(t *testing.T) {
			api := &optionAPI{}
			httpServer := httptest.NewServer(api)
			defer httpServer.Close()
			var servers atomic.Int64
			var privateRefresh atomic.Int64
			factories := func() map[string]func() (tfprotov6.ProviderServer, error) {
				return map[string]func() (tfprotov6.ProviderServer, error){"opnsense": func() (tfprotov6.ProviderServer, error) {
					servers.Add(1)
					base, err := providerserver.NewProtocol6WithError(provider.New("local-fault-test")())()
					if err != nil {
						return nil, err
					}
					return &optionCoreObservedServer{ProviderServer: base, observed: &privateRefresh}, nil
				}}
			}
			config := func(description string) string {
				return fmt.Sprintf(`provider "opnsense" {
 uri = %q
 api_key = "fixture-key"
 api_secret = "fixture-secret"
}
resource "opnsense_kea_dhcpv4_option" "test" {
 code="15"
 encoding="fqdn"
 data="example.org"
 description=%q
}`, httpServer.URL, description)
			}
			providerOnly := fmt.Sprintf(`provider "opnsense" {
 uri=%q
 api_key="fixture-key"
 api_secret="fixture-secret"
}`, httpServer.URL)
			desired := config("saved update")
			expected := regexp.MustCompile(`(?s)Saved Kea option requires activation`)
			if failure == "save unknown match" || failure == "save unknown mismatch" {
				expected = regexp.MustCompile(`(?s)Kea option save requires reconciliation`)
			}
			if failure != "save activation" && failure != "save unknown match" && failure != "save unknown mismatch" {
				desired = providerOnly
				expected = regexp.MustCompile(`(?s)(Deleted Kea option requires activation|Error deleting Kea option)`)
			}
			resource.UnitTest(t, resource.TestCase{Steps: []resource.TestStep{
				{ProtoV6ProviderFactories: factories(), Config: config("initial")},
				{ProtoV6ProviderFactories: factories(), Config: desired, PreConfig: func() {
					api.mu.Lock()
					defer api.mu.Unlock()
					switch failure {
					case "save unknown match":
						api.ambiguousSave = true
						api.failReadAfterMutation = true
					case "save unknown mismatch":
						api.mismatchSave = true
					case "delete unknown present":
						api.ambiguousDelete = true
						api.preserveDelete = true
						api.malformedAfterDelete = true
					default:
						api.failActivation = true
					}
				}, ExpectError: expected},
				{ProtoV6ProviderFactories: factories(), Config: desired, PreConfig: func() {
					api.mu.Lock()
					defer api.mu.Unlock()
					api.failActivation = false
					if failure == "save unknown mismatch" && api.activations != 1 {
						t.Error("contradictory failed save was activated before fresh plan")
					}
					api.mismatchSave = false
					api.ambiguousSave = false
					api.failReadAfterMutation = false
					api.preserveDelete = false
					api.malformedRead = false
					api.malformedAfterDelete = false
				}, Check: func(state *terraform.State) error {
					api.mu.Lock()
					defer api.mu.Unlock()
					if api.adds != 1 {
						return fmt.Errorf("Core recovery duplicated add: %d", api.adds)
					}
					if strings.HasPrefix(failure, "save ") {
						expectedSaves := 1
						expectedActivations := 2
						if failure == "save unknown mismatch" {
							expectedSaves = 2
						}
						if failure == "save activation" {
							expectedActivations = 3
						}
						if api.saves != expectedSaves || api.activations != expectedActivations || state.RootModule().Resources["opnsense_kea_dhcpv4_option.test"].Primary.Attributes["description"] != "saved update" {
							return fmt.Errorf("Core lost error-returned save/private or repeated set: saves=%d activation=%d", api.saves, api.activations)
						}
					} else {
						expectedDeletes, expectedActivations := 1, 3
						if failure == "delete unknown present" {
							expectedDeletes = 2
							expectedActivations = 2
						}
						if api.deletes != expectedDeletes || api.activations != expectedActivations || api.payload != nil {
							return fmt.Errorf("Core refresh-before-destroy failed deletes=%d activation=%d", api.deletes, api.activations)
						}
					}
					if privateRefresh.Load() == 0 {
						return fmt.Errorf("Core never replayed persisted private bytes into a new server refresh")
					}

					return nil
				}},
				{ProtoV6ProviderFactories: factories(), Config: desired, PlanOnly: true},
			}})
			if servers.Load() < 3 {
				t.Fatal("provider servers were not restarted across persisted failed apply")
			}
		})
	}
}

type optionCoreObservedServer struct {
	tfprotov6.ProviderServer
	observed *atomic.Int64
	inspect  func(*tfprotov6.ReadResourceRequest)
}

func (s *optionCoreObservedServer) ReadResource(ctx context.Context, req *tfprotov6.ReadResourceRequest) (*tfprotov6.ReadResourceResponse, error) {
	if s.inspect != nil {
		s.inspect(req)
	}
	if req.TypeName == optionType && len(req.Private) > 0 {
		s.observed.Add(1)
	}
	return s.ProviderServer.ReadResource(ctx, req)
}

func TestDHCPv4OptionCoreImportOmissionClearsAndNoChange(t *testing.T) {
	binary, err := exec.LookPath("terraform")
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("TF_ACC_TERRAFORM_PATH", binary)
	api := &optionAPI{}
	httpServer := httptest.NewServer(api)
	defer httpServer.Close()
	factories := map[string]func() (tfprotov6.ProviderServer, error){"opnsense": providerserver.NewProtocol6WithError(provider.New("local-lifecycle-test")())}
	config := func(phase string) string {
		optional := `encoding="fqdn"
 force=true
 match_code="66"
 match_encoding="string"
 match_data="synthetic.example.org"`
		data := "example.org"
		if phase == "omitted" || phase == "omitted-update" {
			optional = ""
		}
		if phase == "omitted-update" {
			data = "updated.example.org"
		}
		if phase == "clear" {
			optional = `encoding="fqdn"
 force=false
 match_code=""
 match_encoding=""
 match_data=""`
		}
		return fmt.Sprintf(`provider "opnsense" {
 uri=%q
 api_key="fixture-key"
 api_secret="fixture-secret"
}
resource "opnsense_kea_dhcpv4_option" "test" {
 code="15"
 data=%q
 description="synthetic_option"
 %s
}
data "opnsense_kea_dhcpv4_option" "test" {id=opnsense_kea_dhcpv4_option.test.id}
`, httpServer.URL, data, optional)
	}
	resource.UnitTest(t, resource.TestCase{ProtoV6ProviderFactories: factories, Steps: []resource.TestStep{
		{Config: config("populated")},
		{Config: config("omitted"), ResourceName: "opnsense_kea_dhcpv4_option.test", ImportState: true, ImportStateVerify: true},
		{Config: config("omitted"), PlanOnly: true},
		{Config: config("omitted-update"), Check: func(_ *terraform.State) error {
			api.mu.Lock()
			defer api.mu.Unlock()
			for _, key := range []string{"encoding", "force", "match_code", "match_encoding", "match_data"} {
				if _, ok := api.lastWire[key]; ok {
					return fmt.Errorf("omitted %s serialized on update", key)
				}
			}
			if api.payload["force"] != "1" || api.payload["match_data"] != "synthetic.example.org" {
				return fmt.Errorf("omitted values lost")
			}
			return nil
		}},
		{Config: config("omitted-update"), PlanOnly: true},
		{Config: config("clear"), Check: resource.ComposeAggregateTestCheckFunc(resource.TestCheckResourceAttr("opnsense_kea_dhcpv4_option.test", "force", "false"), resource.TestCheckResourceAttr("opnsense_kea_dhcpv4_option.test", "match_data", ""))},
		{Config: config("clear"), PlanOnly: true},
	}})
}

func TestDHCPv4OptionCoreFailedCreateTaintRefreshAndDestroy(t *testing.T) {
	binary, err := exec.LookPath("terraform")
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("TF_ACC_TERRAFORM_PATH", binary)
	for _, failure := range []string{"bad read", "activation"} {
		t.Run(failure, func(t *testing.T) {
			api := &optionAPI{failReadAfterMutation: failure == "bad read", failActivation: failure == "activation"}
			httpServer := httptest.NewServer(api)
			defer httpServer.Close()
			var privateRefresh, servers, intentRefresh atomic.Int64
			factories := func() map[string]func() (tfprotov6.ProviderServer, error) {
				return map[string]func() (tfprotov6.ProviderServer, error){"opnsense": func() (tfprotov6.ProviderServer, error) {
					servers.Add(1)
					base, err := providerserver.NewProtocol6WithError(provider.New("failed-create-test")())()
					if err != nil {
						return nil, err
					}
					return &optionCoreObservedServer{ProviderServer: base, observed: &privateRefresh, inspect: func(req *tfprotov6.ReadResourceRequest) {
						if req.TypeName != optionType || len(req.Private) == 0 {
							return
						}
						var outer map[string][]byte
						if json.Unmarshal(req.Private, &outer) != nil {
							t.Error("Core replayed corrupt private wrapper")
							return
						}
						raw := outer["kea_option_recovery_v1"]
						if len(raw) == 0 {
							return
						}
						var marker struct {
							ID      string            `json:"id"`
							Kind    string            `json:"kind"`
							Payload map[string]string `json:"payload"`
						}
						if json.Unmarshal(raw, &marker) != nil || marker.ID != optionUUID || marker.Kind != "saved" || len(marker.Payload) != 8 || marker.Payload["code"] != "15" || marker.Payload["encoding"] != "fqdn" || marker.Payload["data"] != "example.org" || marker.Payload["description"] != "synthetic_option" {
							t.Error("Core failed create did not persist requested UUID-bound intent")
							return
						}
						intentRefresh.Add(1)
					}}, nil
				}}
			}
			providerOnly := fmt.Sprintf(`provider "opnsense" {
 uri=%q
 api_key="fixture-key"
 api_secret="fixture-secret"
}`, httpServer.URL)
			config := providerOnly + `
resource "opnsense_kea_dhcpv4_option" "test" {
 code="15"
 encoding="fqdn"
 data="example.org"
 description="synthetic_option"
}`
			resource.UnitTest(t, resource.TestCase{Steps: []resource.TestStep{
				{ProtoV6ProviderFactories: factories(), Config: config, ExpectError: regexp.MustCompile(`(?s)Saved Kea option requires (recovery|activation)`)},
				// Core taints failed creates. A same-config plan must remain a replacement
				// plan even after authoritative recovery; recovery itself never adds again.
				{ProtoV6ProviderFactories: factories(), Config: config, PlanOnly: true, ExpectNonEmptyPlan: true, PreConfig: func() {
					api.mu.Lock()
					defer api.mu.Unlock()
					expectedActivation := 0
					if failure == "activation" {
						expectedActivation = 1
					}
					if api.adds != 1 || api.activations != expectedActivation || api.payload == nil {
						t.Fatal("failed create lost committed object or activated unverified read-back")
					}
					api.malformedRead = false
					api.failReadAfterMutation = false
					api.failActivation = false
				}},
				{ProtoV6ProviderFactories: factories(), Config: providerOnly, Check: func(_ *terraform.State) error {
					api.mu.Lock()
					defer api.mu.Unlock()
					if api.adds != 1 || api.deletes != 1 || api.payload != nil || privateRefresh.Load() == 0 || intentRefresh.Load() == 0 {
						return fmt.Errorf("Core failed-create recovery/destroy orphaned or repeated add: adds=%d deletes=%d private=%d", api.adds, api.deletes, privateRefresh.Load())
					}
					return nil
				}},
			}})
			if servers.Load() < 3 {
				t.Fatal("fresh provider servers not exercised")
			}
		})
	}
}
