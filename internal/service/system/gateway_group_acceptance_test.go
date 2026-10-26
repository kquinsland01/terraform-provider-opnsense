// Copyright (c) Matthew Mellor
// SPDX-License-Identifier: MPL-2.0

package system_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"reflect"
	"regexp"
	"sort"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/matthew-on-git/terraform-provider-opnsense/internal/acctest"
	"github.com/matthew-on-git/terraform-provider-opnsense/pkg/opnsense"
)

func groupOracle(t *testing.T, action, id string, expected ...map[string]any) error {
	t.Helper()
	requestBody, err := json.Marshal(map[string][]string{"args": {action, os.Getenv("STORY31_0M_EVIDENCE_DIR"), id, "tf31m_group"}})
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	// Constrain the privileged oracle to the fixed local acceptance service;
	// never forward its bearer token to arbitrary URLs or redirects.
	oracleEndpoint := ""
	switch os.Getenv("STORY31_0M_ORACLE_URL") {
	case "http://host.docker.internal:19531":
		oracleEndpoint = "http://host.docker.internal:19531/action"
	case "http://127.0.0.1:19531":
		oracleEndpoint = "http://127.0.0.1:19531/action"
	default:
		return fmt.Errorf("oracle URL must identify the local isolated acceptance service on port 19531")
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, oracleEndpoint, bytes.NewReader(requestBody))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+os.Getenv("STORY31_0M_ORACLE_TOKEN"))
	client := &http.Client{Timeout: 90 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	response, err := client.Do(request)
	if err != nil {
		return fmt.Errorf("gateway group oracle %s transport failed (private evidence retained)", action)
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("gateway group oracle %s failed with HTTP %d (private evidence retained)", action, response.StatusCode)
	}
	raw, err := io.ReadAll(io.LimitReader(response.Body, 1024*1024))
	if err != nil {
		return err
	}
	var result struct {
		OK          bool                       `json:"ok"`
		Changed     bool                       `json:"changed"`
		Eligible    bool                       `json:"eligible"`
		Established bool                       `json:"reference_established"`
		Removed     bool                       `json:"reference_removed"`
		Reason      string                     `json:"reason"`
		Group       map[string]json.RawMessage `json:"group"`
	}
	if err = json.Unmarshal(raw, &result); err != nil {
		return fmt.Errorf("oracle %s returned malformed JSON", action)
	}
	if !result.OK {
		return fmt.Errorf("oracle %s gate unmet (private diagnostic): %s", action, result.Reason)
	}
	if action == "drift" && !result.Changed {
		return fmt.Errorf("oracle did not attest actual drift")
	}
	if action == "establish-reference" && (!result.Eligible || !result.Established) {
		return fmt.Errorf("eligible official-API dependency fixture unavailable")
	}
	if action == "remove-reference" && !result.Removed {
		return fmt.Errorf("oracle did not attest reference removal")
	}
	if action == "observe-group" {
		if len(expected) != 1 || result.Group == nil {
			return fmt.Errorf("independent persisted group observation missing")
		}
		for field, want := range expected[0] {
			raw, exists := result.Group[field]
			if !exists {
				return fmt.Errorf("independent group observation lacks %s", field)
			}
			switch value := want.(type) {
			case []string:
				var got []string
				if json.Unmarshal(raw, &got) != nil || got == nil {
					return fmt.Errorf("independent group %s is not a selected-key array", field)
				}
				sort.Strings(got)
				sorted := append([]string{}, value...)
				sort.Strings(sorted)
				if !reflect.DeepEqual(got, sorted) {
					return fmt.Errorf("independent saved group %s differs from expected phase", field)
				}
			case string:
				var got string
				if json.Unmarshal(raw, &got) != nil || got != value {
					return fmt.Errorf("independent saved group %s differs from expected phase", field)
				}
			default:
				return fmt.Errorf("invalid independent observation expectation")
			}
		}
	}
	t.Logf("GATEWAY_GROUP_ORACLE action=%s ok=true", action)
	return nil
}

func TestAccGatewayGroupOwner(t *testing.T) {
	var trackedID string
	testCase := resource.TestCase{
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		PreCheck: func() {
			acctest.PreCheck(t)
			for _, key := range []string{"STORY31_0M_UPSTREAM_URI", "STORY31_0M_TLS_SHA256", "STORY31_0M_ATTESTED", "STORY31_0M_ORACLE_URL", "STORY31_0M_ORACLE_TOKEN", "STORY31_0M_EVIDENCE_DIR", "OPNSENSE_GATEWAY_GROUP_INTERFACE", "OPNSENSE_GATEWAY_GROUP_ADDRESS_A", "OPNSENSE_GATEWAY_GROUP_ADDRESS_B"} {
				if os.Getenv(key) == "" {
					t.Fatalf("missing acceptance gate %s", key)
				}
			}
			if os.Getenv("OPNSENSE_URI") != "http://127.0.0.1:19532" {
				t.Fatal("acceptance provider requests must use the per-connection pinned local proxy")
			}
			if os.Getenv("STORY31_0M_ATTESTED") != "true" {
				t.Fatal("identity/TLS/version guard did not attest disposable appliance")
			}
		},
		CheckDestroy: func(_ *terraform.State) error {
			client := acctest.TestClient(t)
			for _, namespace := range []struct{ endpoint, monad string }{{"/api/routing/group_settings/search", "gateway_group"}, {"/api/routing/settings/search_gateway", "gateway_item"}} {
				rows, err := opnsense.Search[map[string]any](context.Background(), client, opnsense.ReqOpts{SearchEndpoint: namespace.endpoint, Monad: namespace.monad}, opnsense.SearchParams{})
				if err != nil {
					return err
				}
				for _, row := range rows {
					if name, ok := row["name"].(string); ok && regexp.MustCompile(`^tf31m_`).MatchString(name) {
						return fmt.Errorf("test-owned %s remains after Terraform destroy", namespace.monad)
					}
				}
			}
			t.Log("GATEWAY_GROUP_TERRAFORM_DESTROY test_groups=0 test_gateways=0")
			return nil
		},
		Steps: []resource.TestStep{
			{Config: groupAcceptanceConfig("tier2-only"), Check: func(state *terraform.State) error {
				trackedID = state.RootModule().Resources["opnsense_system_gateway_group.test"].Primary.ID
				return nil
			}},
			{Config: groupAcceptanceConfig("tier2-only-update"), Check: resource.TestCheckResourceAttr("opnsense_system_gateway_group.test", "tier_1.#", "0")},
			{Config: groupAcceptanceConfig("initial"), Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttr("opnsense_system_gateway_group.test", "name", "tf31m_group"),
				resource.TestCheckResourceAttr("opnsense_system_gateway_group.test", "tier_1.#", "1"),
				resource.TestCheckResourceAttr("opnsense_system_gateway_group.test", "tier_2.#", "1"),
				resource.TestCheckResourceAttrPair("data.opnsense_system_gateway_group.test", "id", "opnsense_system_gateway_group.test", "id"),
				func(state *terraform.State) error {
					observedID := state.RootModule().Resources["opnsense_system_gateway_group.test"].Primary.ID
					if trackedID != "" && trackedID != observedID {
						return fmt.Errorf("initial tier2-only adoption changed UUID")
					}
					trackedID = observedID
					if !regexp.MustCompile(`^[0-9a-f-]{36}$`).MatchString(trackedID) {
						return fmt.Errorf("invalid tracked UUID")
					}
					t.Log("GATEWAY_GROUP_CORE create=true lookup=true")
					return nil
				},
			)},
			{ResourceName: "opnsense_system_gateway_group.test", ImportState: true, ImportStateVerify: true},
			{Config: groupAcceptanceConfig("initial"), PlanOnly: true},
			{Config: groupAcceptanceConfig("swap"), Check: resource.ComposeAggregateTestCheckFunc(resource.TestCheckTypeSetElemAttr("opnsense_system_gateway_group.test", "tier_1.*", "tf31m_gateway_b"), resource.TestCheckResourceAttr("opnsense_system_gateway_group.test", "trigger", "downlosslatency"), resource.TestCheckResourceAttr("opnsense_system_gateway_group.test", "pool_options", "round-robin sticky-address"))},
			{Config: groupAcceptanceConfig("later-tiers"), Check: resource.ComposeAggregateTestCheckFunc(resource.TestCheckResourceAttr("opnsense_system_gateway_group.test", "tier_3.#", "1"), resource.TestCheckResourceAttr("opnsense_system_gateway_group.test", "tier_4.#", "1"), resource.TestCheckResourceAttr("opnsense_system_gateway_group.test", "tier_5.#", "2"))},
			{Config: groupAcceptanceConfig("round-robin"), Check: resource.TestCheckResourceAttr("opnsense_system_gateway_group.test", "pool_options", "round-robin")},
			{Config: groupAcceptanceConfig("clear"), Check: resource.ComposeAggregateTestCheckFunc(resource.TestCheckResourceAttr("opnsense_system_gateway_group.test", "tier_2.#", "0"), resource.TestCheckResourceAttr("opnsense_system_gateway_group.test", "description", ""), resource.TestCheckResourceAttr("opnsense_system_gateway_group.test", "pool_options", ""))},
			{Config: groupAcceptanceConfig("tier1-clear"), ExpectError: regexp.MustCompile(`(?s)Unsupported first-tier clear`)},
			{Config: groupAcceptanceConfig("clear"), PreConfig: func() {
				if err := groupOracle(t, "observe-group", trackedID, groupAcceptanceExpected("clear")); err != nil {
					t.Fatal(err)
				}
			}, Check: func(state *terraform.State) error {
				if state.RootModule().Resources["opnsense_system_gateway_group.test"].Primary.ID != trackedID {
					return fmt.Errorf("first-tier clear guard changed tracked UUID")
				}
				t.Log("GATEWAY_GROUP_CORE first_tier_clear_rejected=true same_uuid=true")
				return nil
			}},
			{Config: groupAcceptanceConfig("clear"), PlanOnly: true, ExpectNonEmptyPlan: true, PreConfig: func() {
				if err := groupOracle(t, "drift", trackedID); err != nil {
					t.Fatal(err)
				}
			}},
			{Config: groupAcceptanceConfig("clear"), Check: resource.TestCheckResourceAttr("opnsense_system_gateway_group.test", "description", "")},
			{Config: groupAcceptanceConfig("clear"), PlanOnly: true},
			{Config: groupAcceptanceGateways(), PreConfig: func() {
				if err := groupOracle(t, "establish-reference", trackedID); err != nil {
					t.Fatal(err)
				}
			}, ExpectError: regexp.MustCompile(`(?s)Error deleting gateway group`)},
			{Config: groupAcceptanceConfig("clear"), PreConfig: func() {
				if err := groupOracle(t, "remove-reference", trackedID); err != nil {
					t.Fatal(err)
				}
			}, Check: func(state *terraform.State) error {
				if state.RootModule().Resources["opnsense_system_gateway_group.test"].Primary.ID != trackedID {
					return fmt.Errorf("UUID changed after in-use deletion")
				}
				t.Log("GATEWAY_GROUP_CORE import=true no_change=true tiers=true trigger=true pool=true tier2_clear=true later_tiers=true later_tier_clears=true initial_tier2_only=true first_tier_clear_rejected=true drift_repair=true in_use_identity=true")
				return nil
			}},
		},
	}
	for i := range testCase.Steps {
		step := &testCase.Steps[i]
		if step.PlanOnly || step.ExpectError != nil || step.Config == "" {
			continue
		}
		matches := regexp.MustCompile(`# acceptance_phase=([a-z0-9-]+)`).FindStringSubmatch(step.Config)
		if len(matches) != 2 {
			continue
		}
		phase := matches[1]
		observe := func(state *terraform.State) error {
			id := state.RootModule().Resources["opnsense_system_gateway_group.test"].Primary.ID
			if trackedID != "" && trackedID != id {
				return fmt.Errorf("phase %s changed tracked UUID", phase)
			}
			return groupOracle(t, "observe-group", id, groupAcceptanceExpected(phase))
		}
		if step.Check == nil {
			step.Check = observe
		} else {
			step.Check = resource.ComposeAggregateTestCheckFunc(step.Check, observe)
		}
	}
	resource.Test(t, testCase)
}

func groupAcceptanceGateways() string {
	return fmt.Sprintf(`
provider "opnsense" {}
resource "opnsense_system_gateway" "a" {
 name = "tf31m_gateway_a"
 interface = %q
 gateway = %q
 default_gateway = false
 monitor_disable = true
}
resource "opnsense_system_gateway" "b" {
 name = "tf31m_gateway_b"
 interface = %q
 gateway = %q
 default_gateway = false
 monitor_disable = true
}
`, os.Getenv("OPNSENSE_GATEWAY_GROUP_INTERFACE"), os.Getenv("OPNSENSE_GATEWAY_GROUP_ADDRESS_A"), os.Getenv("OPNSENSE_GATEWAY_GROUP_INTERFACE"), os.Getenv("OPNSENSE_GATEWAY_GROUP_ADDRESS_B"))
}

func groupAcceptanceConfig(phase string) string {
	tier1, tier2 := "a", "b"
	trigger, pool, description := "down", "", "synthetic gateway group"
	if phase != "initial" {
		tier1, tier2 = "b", "a"
		trigger = "downlosslatency"
		pool = "round-robin sticky-address"
	}
	if phase == "round-robin" {
		pool = "round-robin"
	}
	first := fmt.Sprintf("[opnsense_system_gateway.%s.name]", tier1)
	second := fmt.Sprintf("[opnsense_system_gateway.%s.name]", tier2)
	if phase == "tier1-clear" || phase == "tier2-only" || phase == "tier2-only-update" {
		first = "[]"
	}
	if phase == "clear" {
		second = "[]"
		description = ""
		pool = ""
	}
	later := ""
	if phase == "later-tiers" {
		later = `tier_3 = [opnsense_system_gateway.a.name]
 tier_4 = [opnsense_system_gateway.b.name]
 tier_5 = [opnsense_system_gateway.a.name, opnsense_system_gateway.b.name]`
	}
	if phase == "tier2-only-update" {
		description = "updated empty first tier"
	}
	return fmt.Sprintf("# acceptance_phase=%s\n", phase) + groupAcceptanceGateways() + fmt.Sprintf(`
resource "opnsense_system_gateway_group" "test" {
 name = "tf31m_group"
 tier_1 = %s
 tier_2 = %s
 %s
 trigger = %q
 pool_options = %q
 description = %q
 depends_on = [opnsense_system_gateway.a, opnsense_system_gateway.b]
}
data "opnsense_system_gateway_group" "test" {
 id = opnsense_system_gateway_group.test.id
}
`, first, second, later, trigger, pool, description)
}

func groupAcceptanceExpected(phase string) map[string]any {
	first, second := "tf31m_gateway_b", "tf31m_gateway_a"
	trigger, pool, description := "downlosslatency", "round-robin sticky-address", "synthetic gateway group"
	if phase == "initial" {
		first, second = "tf31m_gateway_a", "tf31m_gateway_b"
		trigger = "down"
		pool = ""
	}
	tiers := map[string]any{"name": "tf31m_group", "item": []string{first}, "item2": []string{second}, "item3": []string{}, "item4": []string{}, "item5": []string{}}
	if phase == "tier2-only" || phase == "tier2-only-update" {
		tiers["item"] = []string{}
	}
	if phase == "tier2-only-update" {
		description = "updated empty first tier"
	}
	if phase == "later-tiers" {
		tiers["item3"] = []string{"tf31m_gateway_a"}
		tiers["item4"] = []string{"tf31m_gateway_b"}
		tiers["item5"] = []string{"tf31m_gateway_a", "tf31m_gateway_b"}
	}
	if phase == "round-robin" {
		pool = "round-robin"
	}
	if phase == "clear" {
		tiers["item2"] = []string{}
		description = ""
		pool = ""
	}
	tiers["trigger"] = trigger
	tiers["poolopts"] = pool
	tiers["descr"] = description
	return tiers
}
