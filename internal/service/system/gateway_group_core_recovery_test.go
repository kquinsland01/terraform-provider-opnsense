// Copyright (c) Matthew Mellor
// SPDX-License-Identifier: MPL-2.0

package system_test

import (
	"context"
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
func TestGatewayGroupCoreErrorPrivatePersistenceAndRefreshBeforeDestroy(t *testing.T) {
	binary, err := exec.LookPath("terraform")
	if err != nil {
		t.Fatal("local Terraform Core binary is required", err)
	}
	t.Setenv("TF_ACC_TERRAFORM_PATH", binary)
	for _, failure := range []string{"save activation", "save unknown match", "save unknown mismatch", "delete activation", "delete unknown present"} {
		t.Run(failure, func(t *testing.T) {
			api := &groupAPI{}
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
					return &groupCoreObservedServer{ProviderServer: base, observed: &privateRefresh}, nil
				}}
			}
			config := func(description string) string {
				return fmt.Sprintf(`provider "opnsense" {
 uri = %q
 api_key = "fixture-key"
 api_secret = "fixture-secret"
}
resource "opnsense_system_gateway_group" "test" {
 name="synthetic_group"
 tier_1=["synthetic_a"]
 description=%q
}`, httpServer.URL, description)
			}
			providerOnly := fmt.Sprintf(`provider "opnsense" {
 uri=%q
 api_key="fixture-key"
 api_secret="fixture-secret"
}`, httpServer.URL)
			desired := config("saved update")
			expected := regexp.MustCompile(`(?s)Saved gateway group requires activation`)
			if failure == "save unknown match" || failure == "save unknown mismatch" {
				expected = regexp.MustCompile(`(?s)Gateway group save requires reconciliation`)
			}
			if failure != "save activation" && failure != "save unknown match" && failure != "save unknown mismatch" {
				desired = providerOnly
				expected = regexp.MustCompile(`(?s)(Deleted gateway group requires activation|Error deleting gateway group)`)
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
						if api.saves != expectedSaves || api.activations != expectedActivations || state.RootModule().Resources["opnsense_system_gateway_group.test"].Primary.Attributes["description"] != "saved update" {
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

type groupCoreObservedServer struct {
	tfprotov6.ProviderServer
	observed *atomic.Int64
}

func (s *groupCoreObservedServer) ReadResource(ctx context.Context, req *tfprotov6.ReadResourceRequest) (*tfprotov6.ReadResourceResponse, error) {
	if req.TypeName == groupType && len(req.Private) > 0 {
		s.observed.Add(1)
	}
	return s.ProviderServer.ReadResource(ctx, req)
}
