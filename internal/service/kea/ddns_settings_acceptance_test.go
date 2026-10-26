// Copyright (c) Matthew Mellor
// SPDX-License-Identifier: MPL-2.0

package kea_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/matthew-on-git/terraform-provider-opnsense/internal/acctest"
)

// Private oracle protocol: args=[action,evidence_directory]. All actions return
// ok/services_inactive. observe-ddns additionally returns ddns (four API scalar
// strings) and unrelated_kea_equal; drift returns changed. pre-destroy captures
// an independent baseline; verify-state-only compares it before any rollback.
func ddnsOracle(action string, expected map[string]string) error {
	endpoint := ""
	switch os.Getenv("STORY31_0O_ORACLE_URL") {
	case "http://127.0.0.1:19535":
		endpoint = "http://127.0.0.1:19535/action"
	case "http://host.docker.internal:19535":
		endpoint = "http://host.docker.internal:19535/action"
	default:
		return fmt.Errorf("DDNS oracle must use the fixed local acceptance endpoint on 19535")
	}
	payload, err := json.Marshal(map[string][]string{"args": {action, os.Getenv("STORY31_0O_EVIDENCE_DIR")}})
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+os.Getenv("STORY31_0O_ORACLE_TOKEN"))
	client := &http.Client{Timeout: 60 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("DDNS oracle %s transport failed; private evidence retained", action)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("DDNS oracle %s HTTP failure; private evidence retained", action)
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1024*1024))
	if err != nil {
		return err
	}
	return ddnsOracleResponse(action, expected, raw, os.Getenv("STORY31_0O_EXPECTED_ORACLE_DIGEST"))
}

func ddnsOracleResponse(action string, expected map[string]string, raw []byte, expectedSHA string) error {
	var result struct {
		OK            bool              `json:"ok"`
		Inactive      bool              `json:"services_inactive"`
		Unrelated     bool              `json:"unrelated_kea_equal"`
		Changed       bool              `json:"changed"`
		SettingsEqual bool              `json:"settings_equal"`
		ConfigEqual   bool              `json:"config_equal"`
		RoutesEqual   bool              `json:"routes_equal"`
		ServerSHA     string            `json:"server_sha256"`
		RuntimeEqual  bool              `json:"runtime_equal"`
		DDNS          map[string]string `json:"ddns"`
	}
	if json.Unmarshal(raw, &result) != nil || !result.OK || !result.Inactive {
		return fmt.Errorf("DDNS oracle %s missing success/inactive attestation", action)
	}
	if !regexp.MustCompile(`^[0-9a-f]{64}$`).MatchString(expectedSHA) || result.ServerSHA != expectedSHA {
		return fmt.Errorf("DDNS oracle %s source attestation mismatch", action)
	}
	switch action {
	case "observe-ddns":
		if len(expected) != 4 || !result.Unrelated {
			return fmt.Errorf("independent DDNS/unrelated Kea observation missing")
		}
		for k, want := range expected {
			if result.DDNS[k] != want {
				return fmt.Errorf("independent DDNS field %s differs", k)
			}
		}
	case "drift":
		if !result.Changed {
			return fmt.Errorf("external drift was not established")
		}
	case "verify-state-only":
		if !result.SettingsEqual || !result.ConfigEqual || !result.RuntimeEqual || !result.RoutesEqual {
			return fmt.Errorf("destroy changed appliance configuration or runtime before restoration")
		}
	}
	return nil
}

func ddnsAcceptanceConfig(phase string) string {
	optional := "manual_config=true\nserver_ip=\"192.0.2.31\"\nserver_port=53002"
	switch phase {
	case "omitted":
		optional = ""
	case "omitted-update":
		optional = "server_port=53004"
	case "false":
		optional = "manual_config=false\nserver_ip=\"192.0.2.31\"\nserver_port=53002"
	case "any":
		optional = "manual_config=false\nserver_ip=\"any\"\nserver_port=53003"
	case "zero":
		optional = "server_port=0"
	}
	return `provider "opnsense" {}
resource "opnsense_kea_ddns_settings" "test" {
 enabled=false
 ` + optional + `
}
data "opnsense_kea_ddns_settings" "test" { depends_on=[opnsense_kea_ddns_settings.test] }
`
}

func TestAccDDNSSettingsOwner(t *testing.T) {
	name := "opnsense_kea_ddns_settings.test"
	expected := func(phase string) map[string]string {
		p := map[string]string{"enabled": "0", "manual_config": "1", "server_ip": "192.0.2.31", "server_port": "53002"}
		if phase == "omitted-update" {
			p["server_port"] = "53004"
		}
		if phase == "false" || phase == "any" {
			p["manual_config"] = "0"
		}
		if phase == "any" {
			p["server_ip"] = "any"
			p["server_port"] = "53003"
		}
		return p
	}
	observe := func(phase string) resource.TestCheckFunc {
		return func(state *terraform.State) error {
			owner := state.RootModule().Resources[name]
			lookup := state.RootModule().Resources["data.opnsense_kea_ddns_settings.test"]
			if owner == nil || lookup == nil || owner.Primary.ID != "ddns" || lookup.Primary.ID != "ddns" {
				return fmt.Errorf("fixed DDNS owner/lookup missing")
			}
			native := map[string]string{"id": "ddns"}
			for field, value := range expected(phase) {
				if field == "enabled" || field == "manual_config" {
					value = strconv.FormatBool(value == "1")
				}
				native[field] = value
			}
			for field, want := range native {
				if owner.Primary.Attributes[field] != want || lookup.Primary.Attributes[field] != want {
					return fmt.Errorf("Terraform DDNS %s differs from expected native value", field)
				}
			}
			return ddnsOracle("observe-ddns", expected(phase))
		}
	}
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		PreCheck: func() {
			acctest.PreCheck(t)
			for _, key := range []string{"STORY31_0O_UPSTREAM_URI", "STORY31_0O_TLS_SHA256", "STORY31_0O_ORACLE_URL", "STORY31_0O_ORACLE_TOKEN", "STORY31_0O_EVIDENCE_DIR", "STORY31_0O_EXPECTED_ORACLE_DIGEST"} {
				if os.Getenv(key) == "" {
					t.Fatalf("missing guarded acceptance input %s", key)
				}
			}
			if os.Getenv("OPNSENSE_URI") != "http://127.0.0.1:19536" || os.Getenv("STORY31_0O_ATTESTED") != "true" {
				t.Fatal("guarded pinned proxy required")
			}
		},
		CheckDestroy: func(_ *terraform.State) error { return ddnsOracle("verify-state-only", nil) },
		Steps: []resource.TestStep{
			{Config: ddnsAcceptanceConfig("populated"), Check: observe("populated")},
			{Config: ddnsAcceptanceConfig("omitted"), ResourceName: name, ImportState: true, ImportStateId: "ddns", ImportStateVerify: true},
			{Config: ddnsAcceptanceConfig("omitted"), PlanOnly: true},
			{Config: ddnsAcceptanceConfig("omitted"), Check: observe("omitted")},
			{Config: ddnsAcceptanceConfig("omitted-update"), Check: observe("omitted-update")},
			{Config: ddnsAcceptanceConfig("false"), Check: observe("false")},
			{Config: ddnsAcceptanceConfig("false"), PlanOnly: true},
			{Config: ddnsAcceptanceConfig("any"), Check: observe("any")},
			{Config: ddnsAcceptanceConfig("any"), PlanOnly: true, ExpectNonEmptyPlan: true, PreConfig: func() {
				if err := ddnsOracle("drift", nil); err != nil {
					t.Fatal(err)
				}
			}},
			{Config: ddnsAcceptanceConfig("any"), Check: observe("any")},
			{Config: ddnsAcceptanceConfig("any"), PlanOnly: true},
			{Config: ddnsAcceptanceConfig("zero"), ExpectError: regexp.MustCompile(`(?s)(between 1 and 65535|at least 1)`)},
			{Config: ddnsAcceptanceConfig("any"), Check: func(state *terraform.State) error {
				if err := observe("any")(state); err != nil {
					return err
				}
				return ddnsOracle("pre-destroy", nil)
			}},
			{Config: `provider "opnsense" {}`, Check: func(_ *terraform.State) error {
				if err := ddnsOracle("verify-state-only", nil); err != nil {
					return err
				}
				t.Log("KEA_DDNS_CORE create=true import=true no_change=true optional_preservation=true explicit_false=true address_port=true lookup=true drift_repair=true invalid_zero=true state_only_destroy=true")
				return nil
			}},
		},
	})
}

func TestDDNSOracleResponsesRequireSourceAndRouteEquality(t *testing.T) {
	expectedSHA := strings.Repeat("a", 64)
	for _, action := range []string{"observe-ddns", "drift", "pre-destroy", "verify-state-only"} {
		base := map[string]any{"ok": true, "services_inactive": true, "server_sha256": expectedSHA, "unrelated_kea_equal": true, "changed": true, "settings_equal": true, "config_equal": true, "runtime_equal": true, "routes_equal": true, "ddns": map[string]string{"enabled": "0", "manual_config": "1", "server_ip": "any", "server_port": "53004"}}
		expected := base["ddns"].(map[string]string)
		for _, mode := range []string{"valid", "missing source", "wrong source", "routes changed", "missing routes"} {
			t.Run(action+"/"+mode, func(t *testing.T) {
				value := map[string]any{}
				for k, v := range base {
					value[k] = v
				}
				switch mode {
				case "missing source":
					delete(value, "server_sha256")
				case "wrong source":
					value["server_sha256"] = strings.Repeat("b", 64)
				case "routes changed":
					value["routes_equal"] = false
				case "missing routes":
					delete(value, "routes_equal")
				}
				raw, err := json.Marshal(value)
				if err != nil {
					t.Fatal(err)
				}
				valid := mode == "valid" || ((mode == "routes changed" || mode == "missing routes") && action != "verify-state-only")
				if (ddnsOracleResponse(action, expected, raw, expectedSHA) == nil) != valid {
					t.Fatal("oracle response validation disagreed", mode)
				}
			})
		}
	}
}
