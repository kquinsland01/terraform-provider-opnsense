// Copyright (c) Matthew Mellor
// SPDX-License-Identifier: MPL-2.0

package kea_test

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/matthew-on-git/terraform-provider-opnsense/internal/acctest"
	"github.com/matthew-on-git/terraform-provider-opnsense/pkg/opnsense"
)

func optionOracle(t *testing.T, action, id string, expected ...map[string]any) error {
	t.Helper()
	description := ""
	if len(expected) == 1 {
		if value, ok := expected[0]["description"].(string); ok {
			description = value
		}
	}
	requestBody, err := json.Marshal(map[string][]string{"args": {action, os.Getenv("STORY31_0N_EVIDENCE_DIR"), id, description}})
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	// Constrain the privileged oracle to the fixed local acceptance service;
	// never forward its bearer token to arbitrary URLs or redirects.
	oracleEndpoint := ""
	switch os.Getenv("STORY31_0N_ORACLE_URL") {
	case "http://host.docker.internal:19532":
		oracleEndpoint = "http://host.docker.internal:19532/action"
	case "http://127.0.0.1:19532":
		oracleEndpoint = "http://127.0.0.1:19532/action"
	default:
		return fmt.Errorf("oracle URL must identify the local isolated acceptance service on port 19532")
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, oracleEndpoint, bytes.NewReader(requestBody))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+os.Getenv("STORY31_0N_ORACLE_TOKEN"))
	client := &http.Client{Timeout: 90 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	response, err := client.Do(request)
	if err != nil {
		return fmt.Errorf("Kea option oracle %s transport failed (private evidence retained)", action)
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("Kea option oracle %s failed with HTTP %d (private evidence retained)", action, response.StatusCode)
	}
	raw, err := io.ReadAll(io.LimitReader(response.Body, 1024*1024))
	if err != nil {
		return err
	}
	var result struct {
		OK               bool                       `json:"ok"`
		Changed          bool                       `json:"changed"`
		Eligible         bool                       `json:"eligible"`
		Established      bool                       `json:"reference_established"`
		Inactive         bool                       `json:"services_inactive"`
		Removed          bool                       `json:"reference_removed"`
		TestOptions      *int                       `json:"test_options"`
		TestSubnets      *int                       `json:"test_subnets"`
		TestReservations *int                       `json:"test_reservations"`
		Reason           string                     `json:"reason"`
		Option           map[string]json.RawMessage `json:"option"`
	}
	if err = json.Unmarshal(raw, &result); err != nil {
		return fmt.Errorf("oracle %s returned malformed JSON", action)
	}
	if !result.OK || !result.Inactive {
		return fmt.Errorf("oracle %s gate unmet (private diagnostic): %s", action, result.Reason)
	}
	if action == "verify-zero" {
		if result.TestOptions == nil || result.TestSubnets == nil || result.TestReservations == nil || *result.TestOptions != 0 || *result.TestSubnets != 0 || *result.TestReservations != 0 {
			return fmt.Errorf("independent zero-option/dependency proof failed")
		}
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
	if action == "observe-option" {
		if len(expected) != 1 || result.Option == nil {
			return fmt.Errorf("independent persisted option observation missing")
		}
		for field, want := range expected[0] {
			raw, exists := result.Option[field]
			if !exists {
				return fmt.Errorf("independent option observation lacks %s", field)
			}
			switch value := want.(type) {
			case bool:
				var got bool
				if json.Unmarshal(raw, &got) != nil || got != value {
					return fmt.Errorf("independent saved option %s differs", field)
				}
			case string:
				var got string
				if json.Unmarshal(raw, &got) != nil || got != value {
					return fmt.Errorf("independent saved option %s differs from expected phase", field)
				}
			default:
				return fmt.Errorf("invalid independent observation expectation")
			}
		}
	}
	t.Logf("KEA_OPTION_ORACLE action=%s ok=true", action)
	return nil
}

func TestAccDHCPv4OptionOwner(t *testing.T) {
	var trackedID string
	name := "opnsense_kea_dhcpv4_option.test"
	tc := resource.TestCase{
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		PreCheck: func() {
			acctest.PreCheck(t)
			for _, key := range []string{"STORY31_0N_UPSTREAM_URI", "STORY31_0N_TLS_SHA256", "STORY31_0N_ATTESTED", "STORY31_0N_ORACLE_URL", "STORY31_0N_ORACLE_TOKEN", "STORY31_0N_EVIDENCE_DIR"} {
				if os.Getenv(key) == "" {
					t.Fatalf("missing acceptance gate %s", key)
				}
			}
			if os.Getenv("OPNSENSE_URI") != "http://127.0.0.1:19534" || os.Getenv("STORY31_0N_ATTESTED") != "true" {
				t.Fatal("guard/pinned proxy required")
			}
		},
		CheckDestroy: func(_ *terraform.State) error {
			client := acctest.TestClient(t)
			for _, namespace := range []struct{ endpoint, monad string }{{"/api/kea/dhcpv4/search_option", "option"}, {"/api/kea/dhcpv4/search_subnet", "subnet4"}, {"/api/kea/dhcpv4/search_reservation", "reservation"}} {
				rows, err := opnsense.Search[map[string]any](context.Background(), client, opnsense.ReqOpts{SearchEndpoint: namespace.endpoint, Monad: namespace.monad}, opnsense.SearchParams{})
				if err != nil {
					return err
				}
				for _, row := range rows {
					for _, key := range []string{"description", "hostname"} {
						if value, ok := row[key].(string); ok && regexp.MustCompile(`^tf31n_`).MatchString(value) {
							return fmt.Errorf("test-owned %s remains after Terraform destroy", namespace.monad)
						}
					}
				}
			}
			if err := optionOracle(t, "verify-zero", ""); err != nil {
				return err
			}
			t.Log("KEA_OPTION_TERRAFORM_DESTROY test_options=0 test_subnets=0 test_reservations=0")
			return nil
		},
		Steps: []resource.TestStep{
			{Config: optionAcceptanceConfig("defaults"), Check: func(state *terraform.State) error {
				trackedID = state.RootModule().Resources[name].Primary.ID
				return nil
			}},
			{Config: optionAcceptanceConfig("populated")},
			{Config: optionAcceptanceConfig("omitted"), ResourceName: name, ImportState: true, ImportStateVerify: true},
			{Config: optionAcceptanceConfig("omitted"), PlanOnly: true},
			{Config: optionAcceptanceConfig("omitted-update")},
			{Config: optionAcceptanceConfig("omitted-update"), PlanOnly: true},
			{Config: optionAcceptanceConfig("clear")},
			{Config: optionAcceptanceConfig("encoding-only")},
			{Config: optionAcceptanceConfig("mutable")},
			{Config: optionAcceptanceConfig("mutable"), PlanOnly: true, ExpectNonEmptyPlan: true, PreConfig: func() {
				if err := optionOracle(t, "drift", trackedID); err != nil {
					t.Fatal(err)
				}
			}},
			{Config: optionAcceptanceConfig("mutable")},
			{Config: optionAcceptanceConfig("mutable"), PlanOnly: true},
			{Config: optionAcceptanceCompanion(), PreConfig: func() {
				if err := optionOracle(t, "establish-reference", trackedID); err != nil {
					t.Fatal(err)
				}
			}, ExpectError: regexp.MustCompile(`(?s)Error deleting Kea option`)},
			{Config: optionAcceptanceConfig("mutable"), PreConfig: func() {
				if err := optionDeleteHTTP500(trackedID); err != nil {
					t.Fatal(err)
				}
				if err := optionOracle(t, "observe-option", trackedID, optionAcceptanceExpected("mutable")); err != nil {
					t.Fatal(err)
				}
				if err := optionOracle(t, "remove-reference", trackedID); err != nil {
					t.Fatal(err)
				}
			}, Check: func(state *terraform.State) error {
				if state.RootModule().Resources[name].Primary.ID != trackedID {
					return fmt.Errorf("UUID changed after in-use deletion")
				}
				t.Log("KEA_OPTION_CORE create=true import=true no_change=true lookup=true all_fields=true defaults=true optional_preservation=true explicit_false=true matching_clears=true encoding_only=true mutable_description=true drift_repair=true in_use_identity=true")
				return nil
			}},
		},
	}
	for i := range tc.Steps {
		step := &tc.Steps[i]
		if step.PlanOnly || step.ExpectError != nil || step.Config == "" {
			continue
		}
		phase := regexp.MustCompile(`# acceptance_phase=([a-z-]+)`).FindStringSubmatch(step.Config)
		if len(phase) != 2 {
			continue
		}
		current := phase[1]
		observe := func(state *terraform.State) error {
			observed := state.RootModule().Resources[name].Primary.ID
			if observed != trackedID {
				return fmt.Errorf("phase %s changed tracked UUID", current)
			}
			for _, field := range []string{"id", "code", "encoding", "data", "force", "match_code", "match_encoding", "match_data", "description"} {
				if state.RootModule().Resources[name].Primary.Attributes[field] != state.RootModule().Resources["data.opnsense_kea_dhcpv4_option.test"].Primary.Attributes[field] {
					return fmt.Errorf("lookup %s differs", field)
				}
			}
			if err := optionOracle(t, "observe-option", observed, optionAcceptanceExpected(current)); err != nil {
				return err
			}
			companion := state.RootModule().Resources["opnsense_kea_dhcpv4_option.companion"].Primary.ID
			return optionOracle(t, "observe-option", companion, map[string]any{"code": "15", "encoding": "fqdn", "data": "unrelated.example.org", "force": false, "match_code": "", "match_encoding": "", "match_data": "", "description": "tf31n_companion"})
		}
		if step.Check == nil {
			step.Check = observe
		} else {
			step.Check = resource.ComposeAggregateTestCheckFunc(step.Check, observe)
		}
	}
	resource.Test(t, tc)
}

func optionAcceptanceCompanion() string {
	return `
provider "opnsense" {}
resource "opnsense_kea_dhcpv4_option" "companion" {
 code="15"
 encoding="fqdn"
 data="unrelated.example.org"
 description="tf31n_companion"
}`
}

func optionAcceptanceConfig(phase string) string {
	expected := optionAcceptanceExpected(phase)
	optional := ""
	if phase != "defaults" && phase != "omitted" && phase != "omitted-update" {
		optional = fmt.Sprintf("encoding=%q\n force=%t\n match_code=%q\n match_encoding=%q\n match_data=%q", expected["encoding"], expected["force"], expected["match_code"], expected["match_encoding"], expected["match_data"])
	}
	return fmt.Sprintf("# acceptance_phase=%s\n", phase) + optionAcceptanceCompanion() + fmt.Sprintf(`
resource "opnsense_kea_dhcpv4_option" "test" {
 code=%q
 data=%q
 description=%q
 %s
}
data "opnsense_kea_dhcpv4_option" "test" { id=opnsense_kea_dhcpv4_option.test.id }
`, expected["code"], expected["data"], expected["description"], optional)
}

func optionAcceptanceExpected(phase string) map[string]any {
	result := map[string]any{"code": "15", "encoding": "fqdn", "data": "example.org", "force": true, "match_code": "66", "match_encoding": "string", "match_data": "synthetic.example.org", "description": "tf31n_option"}
	if phase == "defaults" {
		result["code"] = "224"
		result["encoding"] = "hex"
		result["data"] = "0102"
		result["force"] = false
		result["match_code"] = ""
		result["match_encoding"] = ""
		result["match_data"] = ""
	}
	if phase == "omitted-update" {
		result["data"] = "updated.example.org"
	}
	if phase == "clear" || phase == "encoding-only" || phase == "mutable" {
		result["force"] = false
		result["match_code"] = ""
		result["match_encoding"] = ""
		result["match_data"] = ""
	}
	if phase == "encoding-only" || phase == "mutable" {
		result["match_encoding"] = "hex"
	}
	if phase == "mutable" {
		result["description"] = "tf31n_option_renamed"
	}
	return result
}

func optionDeleteHTTP500(id string) error {
	root, err := os.OpenRoot(os.Getenv("STORY31_0N_EVIDENCE_DIR"))
	if err != nil {
		return err
	}
	defer func() { _ = root.Close() }()
	file, err := root.Open("option-delete-status.jsonl")
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()
	count := 0
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		var entry struct {
			Method string `json:"method"`
			Path   string `json:"path"`
			Status int    `json:"status"`
		}
		if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
			return err
		}
		if entry.Path == "/api/kea/dhcpv4/del_option/"+id {
			count++
			if entry.Method != "POST" || entry.Status != http.StatusInternalServerError {
				return fmt.Errorf("tracked delete did not return HTTP500")
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	if count != 1 {
		return fmt.Errorf("expected exactly one tracked HTTP500 delete, got %d", count)
	}
	return nil
}

func TestDHCPv4OptionDeleteStatusEvidence(t *testing.T) {
	for _, tc := range []struct {
		name, records string
		valid         bool
	}{
		{"tracked500", `{"method":"POST","path":"/api/kea/dhcpv4/del_option/` + optionUUID + `","status":500}` + "\n", true},
		{"generic diagnostic no delete", "", false},
		{"wrong status", `{"method":"POST","path":"/api/kea/dhcpv4/del_option/` + optionUUID + `","status":200}` + "\n", false},
		{"wrong method", `{"method":"GET","path":"/api/kea/dhcpv4/del_option/` + optionUUID + `","status":500}` + "\n", false},
		{"wrong UUID", `{"method":"POST","path":"/api/kea/dhcpv4/del_option/00000000-0000-0000-0000-000000000032","status":500}` + "\n", false},
		{"duplicate tracked500", strings.Repeat(`{"method":"POST","path":"/api/kea/dhcpv4/del_option/`+optionUUID+`","status":500}`+"\n", 2), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			directory := t.TempDir()
			t.Setenv("STORY31_0N_EVIDENCE_DIR", directory)
			if err := os.WriteFile(filepath.Join(directory, "option-delete-status.jsonl"), []byte(tc.records), 0o600); err != nil {
				t.Fatal(err)
			}
			if err := optionDeleteHTTP500(optionUUID); (err == nil) != tc.valid {
				t.Fatal("incorrect tracked Core delete proof", err)
			}
		})
	}
}
