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
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/matthew-on-git/terraform-provider-opnsense/internal/provider"
)

type ddnsCoreServer struct {
	tfprotov6.ProviderServer
	observed *atomic.Int64
	api      *ddnsAPI
}

func (s *ddnsCoreServer) ApplyResourceChange(ctx context.Context, req *tfprotov6.ApplyResourceChangeRequest) (*tfprotov6.ApplyResourceChangeResponse, error) {
	s.api.mu.Lock()
	var outer map[string][]byte
	if req.TypeName == ddnsType && json.Unmarshal(req.PlannedPrivate, &outer) == nil && len(outer["kea_ddns_recovery_v1"]) > 0 {
		s.api.pendingApplyPrivate++
	}
	enabled := s.api.coreCancel
	var cancel context.CancelFunc
	if enabled {
		ctx, cancel = context.WithCancel(ctx)
		s.api.cancel = cancel
	}
	s.api.mu.Unlock()
	if cancel != nil {
		defer cancel()
	}
	return s.ProviderServer.ApplyResourceChange(ctx, req)
}

func (s *ddnsCoreServer) ReadResource(ctx context.Context, req *tfprotov6.ReadResourceRequest) (*tfprotov6.ReadResourceResponse, error) {
	if req.TypeName == ddnsType && len(req.Private) > 0 {
		var outer map[string][]byte
		if json.Unmarshal(req.Private, &outer) == nil && len(outer["kea_ddns_recovery_v1"]) > 0 {
			var marker struct {
				ID      string            `json:"id"`
				Kind    string            `json:"kind"`
				Payload map[string]string `json:"payload"`
			}
			if json.Unmarshal(outer["kea_ddns_recovery_v1"], &marker) == nil && marker.ID == "ddns" && (marker.Kind == "saved" || marker.Kind == "save_unknown") && len(marker.Payload) == 2 && marker.Payload["enabled"] == "0" && marker.Payload["server_port"] == "53004" {
				s.observed.Add(1)
			}
		}
	}
	return s.ProviderServer.ReadResource(ctx, req)
}

func ddnsCoreSetup(t *testing.T, api *ddnsAPI) (string, func() map[string]func() (tfprotov6.ProviderServer, error), *atomic.Int64) {
	t.Helper()
	binary, err := exec.LookPath("terraform")
	if err != nil {
		t.Fatal("actual Terraform Core is required", err)
	}
	t.Setenv("TF_ACC_TERRAFORM_PATH", binary)
	h := httptest.NewServer(api)
	t.Cleanup(h.Close)
	var observed atomic.Int64
	factories := func() map[string]func() (tfprotov6.ProviderServer, error) {
		return map[string]func() (tfprotov6.ProviderServer, error){"opnsense": func() (tfprotov6.ProviderServer, error) {
			base, err := providerserver.NewProtocol6WithError(provider.New("ddns-core-test")())()
			if err != nil {
				return nil, err
			}
			return &ddnsCoreServer{ProviderServer: base, observed: &observed, api: api}, nil
		}}
	}
	return fmt.Sprintf(`provider "opnsense" {
 uri=%q
 api_key="fixture-key"
 api_secret="fixture-secret"
}
`, h.URL), factories, &observed
}

func TestDDNSCoreErrorPrivatePersistenceFreshRefreshAndDestroy(t *testing.T) {
	for _, create := range []bool{true, false} {
		for _, failure := range []string{"activation", "read", "unknown", "mismatch", "connection lost", "cancellation"} {
			t.Run(fmt.Sprintf("create=%t/%s", create, failure), func(t *testing.T) {
				api := &ddnsAPI{}
				providerConfig, factories, observed := ddnsCoreSetup(t, api)
				config := func(port int) string {
					return providerConfig + fmt.Sprintf(`resource "opnsense_kea_ddns_settings" "test" {
 enabled=false
 server_port=%d
}
`, port)
				}
				setupFailure := func() {
					api.mu.Lock()
					defer api.mu.Unlock()
					api.failActivation = failure == "activation"
					api.failReadAfterSave = failure == "read"
					api.unknown = failure == "unknown" || failure == "mismatch"
					api.mismatch = failure == "mismatch"
					api.cancelAfterSave = failure == "connection lost"
					api.coreCancel = failure == "cancellation"
				}
				clearFailure := func() {
					api.mu.Lock()
					defer api.mu.Unlock()
					api.failActivation = false
					api.badRead = false
					api.failReadAfterSave = false
					api.unknown = false
					api.mismatch = false
					api.cancelAfterSave = false
					api.coreCancel = false
					api.cancel = nil
				}
				steps := []resource.TestStep{}
				if !create {
					steps = append(steps, resource.TestStep{ProtoV6ProviderFactories: factories(), Config: config(53002)})
				}
				steps = append(steps, resource.TestStep{ProtoV6ProviderFactories: factories(), Config: config(53004), PreConfig: setupFailure, ExpectError: regexp.MustCompile(`(?s)(Saved DDNS settings require activation|DDNS recovery read failed|DDNS save outcome uncertain)`)})
				if create {
					steps = append(steps, resource.TestStep{ProtoV6ProviderFactories: factories(), Config: config(53004), PlanOnly: true, ExpectNonEmptyPlan: true, PreConfig: clearFailure})
				} else {
					steps = append(steps, resource.TestStep{ProtoV6ProviderFactories: factories(), Config: config(53004), PreConfig: clearFailure, Check: resource.TestCheckResourceAttr("opnsense_kea_ddns_settings.test", "server_port", "53004")})
					steps = append(steps, resource.TestStep{ProtoV6ProviderFactories: factories(), Config: config(53004), PlanOnly: true})
				}
				steps = append(steps, resource.TestStep{ProtoV6ProviderFactories: factories(), Config: providerConfig, Check: func(_ *terraform.State) error {
					api.mu.Lock()
					defer api.mu.Unlock()
					want := 1
					if !create {
						want = 2
					}
					if !create && failure == "mismatch" {
						want++
					}
					if api.saves != want || observed.Load() == 0 || api.fields["unowned"] != "preserved" {
						return fmt.Errorf("Core lost private recovery or repeated set: saves=%d want=%d private=%d", api.saves, want, observed.Load())
					}
					return nil
				}})
				resource.UnitTest(t, resource.TestCase{Steps: steps})
			})
		}
	}
}

func TestDDNSCoreImportOmissionFalseLookupAndNoChange(t *testing.T) {
	api := &ddnsAPI{}
	providerConfig, factories, _ := ddnsCoreSetup(t, api)
	config := func(optional string) string {
		return providerConfig + `resource "opnsense_kea_ddns_settings" "test" {
 enabled=false
 ` + optional + `
}
data "opnsense_kea_ddns_settings" "test" { depends_on=[opnsense_kea_ddns_settings.test] }
`
	}
	check := func(_ *terraform.State) error {
		api.mu.Lock()
		defer api.mu.Unlock()
		if api.fields["manual_config"] != "1" || api.fields["server_ip"] != "any" || api.fields["server_port"] != "53002" || len(api.lastWire) != 1 {
			return fmt.Errorf("omitted imported fields reset or serialized: %v", api.lastWire)
		}
		return nil
	}
	resource.UnitTest(t, resource.TestCase{Steps: []resource.TestStep{
		{ProtoV6ProviderFactories: factories(), Config: config(""), Check: check},
		{ProtoV6ProviderFactories: factories(), Config: config(""), ResourceName: "opnsense_kea_ddns_settings.test", ImportState: true, ImportStateId: "ddns", ImportStateVerify: true},
		{ProtoV6ProviderFactories: factories(), Config: config(""), PlanOnly: true},
		{ProtoV6ProviderFactories: factories(), Config: strings.Replace(config(""), "enabled=false", "enabled=true", 1), Check: check},
		{ProtoV6ProviderFactories: factories(), Config: strings.Replace(config(""), "enabled=false", "enabled=true", 1), PlanOnly: true},
		{ProtoV6ProviderFactories: factories(), Config: config("manual_config=false\nserver_ip=\"192.0.2.31\"\nserver_port=53004"), Check: resource.ComposeAggregateTestCheckFunc(resource.TestCheckResourceAttr("opnsense_kea_ddns_settings.test", "manual_config", "false"), resource.TestCheckResourceAttr("data.opnsense_kea_ddns_settings.test", "server_ip", "192.0.2.31"))},
		{ProtoV6ProviderFactories: factories(), Config: config(""), PlanOnly: true},
		{ProtoV6ProviderFactories: factories(), Config: config(""), Check: resource.TestCheckResourceAttr("opnsense_kea_ddns_settings.test", "server_port", "53004")},
	}})
}

// Core applies a genuine error-returned state/private snapshot with refresh
// disabled, so pending recovery reaches Delete rather than Read first. This is
// intentionally distinct from normal Core refresh-before-destroy behavior.
func TestDDNSCorePendingStateOnlyDestroyWithoutRefresh(t *testing.T) {
	for _, create := range []bool{true, false} {
		t.Run(fmt.Sprint(create), func(t *testing.T) {
			api := &ddnsAPI{}
			config, factories, _ := ddnsCoreSetup(t, api)
			desired := config + `resource "opnsense_kea_ddns_settings" "test" {
 enabled=false
 server_port=53004
}`
			steps := []resource.TestStep{}
			if !create {
				steps = append(steps, resource.TestStep{ProtoV6ProviderFactories: factories(), Config: config + `resource "opnsense_kea_ddns_settings" "test" {enabled=false}`})
			}
			steps = append(steps, resource.TestStep{ProtoV6ProviderFactories: factories(), Config: desired, PreConfig: func() { api.mu.Lock(); api.failReadAfterSave = true; api.mu.Unlock() }, ExpectError: regexp.MustCompile(`DDNS recovery read failed`)})
			var saves, reads, activations int
			steps = append(steps, resource.TestStep{ProtoV6ProviderFactories: factories(), Config: config, PreConfig: func() {
				api.mu.Lock()
				saves, reads, activations = api.saves, api.reads, api.activations
				api.mu.Unlock()
			}, Check: func(_ *terraform.State) error {
				api.mu.Lock()
				defer api.mu.Unlock()
				if api.saves != saves || api.reads != reads || api.activations != activations || api.pendingApplyPrivate == 0 {
					return fmt.Errorf("pending Core destroy touched DDNS: saves=%d/%d reads=%d/%d activations=%d/%d", api.saves, saves, api.reads, reads, api.activations, activations)
				}
				return nil
			}})
			resource.UnitTest(t, resource.TestCase{AdditionalCLIOptions: &resource.AdditionalCLIOptions{Plan: resource.PlanOptions{NoRefresh: true}}, Steps: steps})
		})
	}
}

type ddnsPlanCheck func(plancheck.CheckPlanRequest) error

func (f ddnsPlanCheck) CheckPlan(_ context.Context, req plancheck.CheckPlanRequest, resp *plancheck.CheckPlanResponse) {
	resp.Error = f(req)
}

func TestDDNSCoreChangedPlanPreservesFreshOutOfBandOptionalBaseline(t *testing.T) {
	api := &ddnsAPI{}
	config, factories, _ := ddnsCoreSetup(t, api)
	desired := config + `resource "opnsense_kea_ddns_settings" "test" { enabled=false }`
	changed := strings.Replace(desired, "enabled=false", "enabled=true", 1)
	resource.UnitTest(t, resource.TestCase{Steps: []resource.TestStep{
		{ProtoV6ProviderFactories: factories(), Config: desired},
		{ProtoV6ProviderFactories: factories(), Config: changed, ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{ddnsPlanCheck(func(req plancheck.CheckPlanRequest) error {
			for _, change := range req.Plan.ResourceChanges {
				if change.Address == "opnsense_kea_ddns_settings.test" {
					unknown, _ := change.Change.AfterUnknown.(map[string]interface{})
					for _, key := range []string{"manual_config", "server_ip", "server_port"} {
						if unknown[key] != true {
							return fmt.Errorf("changed omitted %s was forced known", key)
						}
					}
				}
			}
			api.mu.Lock()
			defer api.mu.Unlock()
			api.fields["manual_config"] = "0"
			api.fields["server_ip"] = "192.0.2.50"
			api.fields["server_port"] = "53005"
			return nil
		})}}, Check: resource.ComposeAggregateTestCheckFunc(resource.TestCheckResourceAttr("opnsense_kea_ddns_settings.test", "manual_config", "false"), resource.TestCheckResourceAttr("opnsense_kea_ddns_settings.test", "server_ip", "192.0.2.50"), resource.TestCheckResourceAttr("opnsense_kea_ddns_settings.test", "server_port", "53005"), func(_ *terraform.State) error {
			api.mu.Lock()
			defer api.mu.Unlock()
			if len(api.lastWire) != 1 || api.lastWire["enabled"] != "1" {
				return fmt.Errorf("omitted baseline serialized: %v", api.lastWire)
			}
			return nil
		})},
		{ProtoV6ProviderFactories: factories(), Config: changed, PlanOnly: true},
	}})
}

func TestDDNSCoreBuiltinDependencyUnknownPlanThenKnownApply(t *testing.T) {
	api := &ddnsAPI{}
	config, factories, _ := ddnsCoreSetup(t, api)
	desired := config + `resource "terraform_data" "intent" {
 input={enabled=true,manual_config=false,server_ip="192.0.2.32",server_port=53004}
}
resource "opnsense_kea_ddns_settings" "test" {
 enabled=terraform_data.intent.output.enabled
 manual_config=terraform_data.intent.output.manual_config
 server_ip=terraform_data.intent.output.server_ip
 server_port=terraform_data.intent.output.server_port
}`
	check := ddnsPlanCheck(func(req plancheck.CheckPlanRequest) error {
		found := false
		for _, change := range req.Plan.ResourceChanges {
			if change.Address == "opnsense_kea_ddns_settings.test" {
				found = true
				unknown, _ := change.Change.AfterUnknown.(map[string]interface{})
				for _, key := range []string{"enabled", "manual_config", "server_ip", "server_port"} {
					if unknown[key] != true {
						return fmt.Errorf("dependency %s collapsed to known", key)
					}
				}
			}
		}
		api.mu.Lock()
		defer api.mu.Unlock()
		if !found || api.saves != 0 {
			return fmt.Errorf("DDNS mutated before built-in dependency resolved")
		}
		return nil
	})
	resource.UnitTest(t, resource.TestCase{Steps: []resource.TestStep{
		{ProtoV6ProviderFactories: factories(), Config: desired, ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{check}}, Check: resource.ComposeAggregateTestCheckFunc(resource.TestCheckResourceAttr("opnsense_kea_ddns_settings.test", "enabled", "true"), resource.TestCheckResourceAttr("opnsense_kea_ddns_settings.test", "manual_config", "false"), resource.TestCheckResourceAttr("opnsense_kea_ddns_settings.test", "server_ip", "192.0.2.32"), resource.TestCheckResourceAttr("opnsense_kea_ddns_settings.test", "server_port", "53004"))},
		{ProtoV6ProviderFactories: factories(), Config: desired, PlanOnly: true},
	}})
}

func TestDDNSCoreDefaultRefreshBeforePendingDestroy(t *testing.T) {
	for _, malformed := range []bool{false, true} {
		t.Run(fmt.Sprint(malformed), func(t *testing.T) {
			api := &ddnsAPI{}
			config, factories, _ := ddnsCoreSetup(t, api)
			desired := config + `resource "opnsense_kea_ddns_settings" "test" {
 enabled=false
 server_port=53004
}`
			var saves, reads, activations int
			steps := []resource.TestStep{{ProtoV6ProviderFactories: factories(), Config: desired, PreConfig: func() { api.mu.Lock(); api.failReadAfterSave = true; api.mu.Unlock() }, ExpectError: regexp.MustCompile(`DDNS recovery read failed`)}}
			if malformed {
				steps = append(steps, resource.TestStep{ProtoV6ProviderFactories: factories(), Config: config, PreConfig: func() {
					api.mu.Lock()
					saves, reads, activations = api.saves, api.reads, api.activations
					api.mu.Unlock()
				}, ExpectError: regexp.MustCompile(`DDNS recovery read failed`)})
			}
			steps = append(steps, resource.TestStep{ProtoV6ProviderFactories: factories(), Config: config, PreConfig: func() {
				api.mu.Lock()
				defer api.mu.Unlock()
				if malformed && (api.saves != saves || api.reads != reads+1 || api.activations != activations || api.pendingApplyPrivate != 0) {
					t.Fatal("failed planning refresh reached Delete or changed settings")
				}
				saves, reads, activations = api.saves, api.reads, api.activations
				api.badRead = false
				api.failReadAfterSave = false
			}, Check: func(_ *terraform.State) error {
				api.mu.Lock()
				defer api.mu.Unlock()
				if api.saves != saves || api.reads != reads+1 || api.activations != activations+1 || api.pendingApplyPrivate != 0 {
					return fmt.Errorf("default planning refresh calls: saves=%d/%d reads=%d/%d activation=%d/%d", api.saves, saves, api.reads, reads+1, api.activations, activations+1)
				}
				return nil
			}})
			resource.UnitTest(t, resource.TestCase{Steps: steps})
		})
	}
}
