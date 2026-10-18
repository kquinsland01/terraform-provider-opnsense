// Copyright (c) Matthew Mellor
// SPDX-License-Identifier: MPL-2.0

package firewall_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/go-retryablehttp"
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"

	"github.com/matthew-on-git/terraform-provider-opnsense/internal/acctest"
)

func TestAccFirewallAlias_advancedParity(t *testing.T) {
	const address = "opnsense_firewall_alias.dual"
	suffix := acceptanceSuffix()
	description := "managed by Story 31.0a " + suffix
	config := testAccFirewallAliasAdvancedConfig(suffix, description)
	resource.Test(t, resource.TestCase{ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories, PreCheck: parityPreCheck(t), Steps: []resource.TestStep{
		{Config: config, Check: resource.ComposeAggregateTestCheckFunc(
			resource.TestCheckResourceAttr(address, "ip_protocol.#", "2"),
			resource.TestCheckResourceAttr("opnsense_firewall_alias.json", "path_expression", ".addresses[]"),
			resource.TestCheckResourceAttr("opnsense_firewall_alias.table", "update_freq", "1.25"),
			resource.TestCheckResourceAttr("data.opnsense_firewall_alias.dual", "ip_protocol.#", "2"),
		)},
		{ResourceName: address, ImportState: true, ImportStateVerify: true},
		{Config: config, PlanOnly: true},
		{Config: config, Check: mutateDescription(t, address, "alias", "out-of-band Story 31.0a drift "+suffix), ExpectNonEmptyPlan: true},
		{Config: config, PlanOnly: true, ExpectNonEmptyPlan: true},
		{Config: config, Check: resource.TestCheckResourceAttr(address, "description", description)},
		{Config: config, PlanOnly: true},
	}})
}

func TestAccFirewallFilterRule_advancedParity(t *testing.T) {
	iface := os.Getenv("OPNSENSE_FIREWALL_TEST_INTERFACE")
	if iface == "" {
		t.Skip("OPNSENSE_FIREWALL_TEST_INTERFACE is required")
	}
	const address = "opnsense_firewall_filter_rule.advanced"
	suffix := acceptanceSuffix()
	port := strconv.Itoa(20000 + int(time.Now().UnixNano()%20000))
	config := testAccFirewallFilterAdvancedConfig(iface, port, "Story 31.0a advanced filter "+suffix)
	resource.Test(t, resource.TestCase{ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories, PreCheck: parityPreCheck(t), Steps: []resource.TestStep{
		{Config: config, Check: resource.ComposeAggregateTestCheckFunc(
			resource.TestCheckResourceAttr(address, "protocol", "tcp"),
			resource.TestCheckResourceAttr(address, "tcp_flags.#", "1"),
			resource.TestCheckResourceAttr(address, "set_local_tag", "story31a"),
			resource.TestCheckResourceAttr("data.opnsense_firewall_filter_rule.advanced", "protocol", "TCP"),
		)},
		{ResourceName: address, ImportState: true, ImportStateVerify: true, ImportStateVerifyIgnore: []string{"protocol"}},
		{Config: config, PlanOnly: true},
		{Config: config, Check: mutateDescription(t, address, "rule", "out-of-band Story 31.0a filter drift "+suffix), ExpectNonEmptyPlan: true},
		{Config: config, PlanOnly: true, ExpectNonEmptyPlan: true},
		{Config: testAccFirewallFilterAdvancedConfig(iface, port, "Story 31.0a updated filter "+suffix), Check: resource.TestCheckResourceAttr(address, "description", "Story 31.0a updated filter "+suffix)},
		{Config: testAccFirewallFilterAdvancedConfig(iface, port, "Story 31.0a updated filter "+suffix), PlanOnly: true},
	}})
}

func TestAccFirewallNatPortForward_advancedParity(t *testing.T) {
	interfaces := splitNonEmpty(os.Getenv("OPNSENSE_FIREWALL_TEST_INTERFACES"))
	if len(interfaces) < 2 {
		t.Skip("OPNSENSE_FIREWALL_TEST_INTERFACES requires two disposable interfaces")
	}
	const address = "opnsense_firewall_nat_port_forward.advanced"
	suffix := acceptanceSuffix()
	port := strconv.Itoa(40000 + int(time.Now().UnixNano()%20000))
	initial := testAccFirewallNATAdvancedConfig(interfaces[:2], suffix, port)
	updated := testAccFirewallNATAdvancedConfig(interfaces[1:2], suffix, port)
	resource.Test(t, resource.TestCase{ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories, PreCheck: parityPreCheck(t), Steps: []resource.TestStep{
		{Config: initial, Check: resource.ComposeAggregateTestCheckFunc(resource.TestCheckResourceAttr(address, "interfaces.#", "2"), resource.TestCheckResourceAttr("data.opnsense_firewall_nat_port_forward.advanced", "interfaces.#", "2"))},
		{ResourceName: address, ImportState: true, ImportStateVerify: true},
		{Config: initial, PlanOnly: true},
		{Config: updated, Check: resource.TestCheckResourceAttr(address, "interfaces.#", "1")},
		{Config: updated, PlanOnly: true},
	}})
}

func parityPreCheck(t *testing.T) func() {
	return func() {
		acctest.PreCheck(t)
		if os.Getenv("OPNSENSE_FIREWALL_PARITY") != "1" {
			t.Skip("set OPNSENSE_FIREWALL_PARITY=1 only for disposable OPNsense 26.7.7+")
		}
		client := acctest.TestClient(t)
		resp, err := client.HTTPClient().Get(client.BaseURL() + "/api/core/firmware/info")
		if err != nil {
			t.Fatal(err)
		}
		version, err := firmwareVersion(resp)
		if err != nil {
			t.Fatal(err)
		}
		if !acceptanceVersionAtLeast(version, 26, 7, 7) {
			t.Skipf("requires OPNsense 26.7.7+, got %s", version)
		}
	}
}

func firmwareVersion(resp *http.Response) (string, error) {
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		body, err := io.ReadAll(io.LimitReader(resp.Body, 4096))
		if err != nil {
			return "", fmt.Errorf("read firmware response: %w", err)
		}
		return "", fmt.Errorf("firmware precheck HTTP %d: %q", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var version struct {
		ProductVersion string `json:"product_version"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&version); err != nil {
		return "", err
	}
	return version.ProductVersion, nil
}

func TestFirmwareVersionDecodesLargeSuccessBody(t *testing.T) {
	body := fmt.Sprintf(`{"padding":%q,"product_version":"26.7.7"}`, strings.Repeat("x", 8192))
	resp := &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body))}
	version, err := firmwareVersion(resp)
	if err != nil {
		t.Fatal(err)
	}
	if version != "26.7.7" {
		t.Fatalf("unexpected version: %q", version)
	}
}

func TestFirmwareVersionRejectsNonSuccessBeforeDecode(t *testing.T) {
	body := "upstream unavailable " + strings.Repeat("x", 4096) + " unbounded-tail"
	resp := &http.Response{StatusCode: http.StatusBadGateway, Body: io.NopCloser(strings.NewReader(body))}
	_, err := firmwareVersion(resp)
	if err == nil || !strings.Contains(err.Error(), "HTTP 502") || !strings.Contains(err.Error(), "upstream unavailable") {
		t.Fatalf("unexpected diagnostic: %v", err)
	}
	if strings.Contains(err.Error(), "unbounded-tail") {
		t.Fatalf("non-success diagnostic was not bounded: %v", err)
	}
}

func testAccFirewallAliasAdvancedConfig(suffix, description string) string {
	return fmt.Sprintf(`
resource "opnsense_firewall_alias" "dual" {
  name        = "tf_story31a_dual_%[1]s"
  type        = "asn"
  content     = ["13335"]
  proto       = "IPv4"
  ip_protocol = ["IPv4", "IPv6"]
  description = %[2]q
}

resource "opnsense_firewall_alias" "json" {
  name            = "tf_story31a_json_%[1]s"
  type            = "urljson"
  content         = ["https://example.invalid/story.json"]
  path_expression = ".addresses[]"
  stats           = true
  description     = "Story 31.0a JSON %[1]s"
}

resource "opnsense_firewall_alias" "table" {
  name        = "tf_story31a_table_%[1]s"
  type        = "urltable"
  content     = ["https://example.invalid/story.txt"]
  update_freq = "1.25"
  description = "Story 31.0a table %[1]s"
}

data "opnsense_firewall_alias" "dual" {
  id = opnsense_firewall_alias.dual.id
}
`, suffix, description)
}

func testAccFirewallFilterAdvancedConfig(iface, port, description string) string {
	return fmt.Sprintf(`
resource "opnsense_firewall_filter_rule" "advanced" {
  enabled                     = false
  sequence                    = 987600
  action                      = "pass"
  quick                       = true
  interface                   = [%[1]q]
  no_xmlrpc_sync              = true
  allow_options               = true
  direction                   = "in"
  ip_protocol                 = "inet"
  protocol                    = "tcp"
  source_net                  = "any"
  destination_net             = "any"
  destination_port            = %[2]q
  tcp_flags                   = ["syn"]
  tcp_flags_out_of            = ["ack", "syn"]
  state_type                  = "keep"
  state_policy                = "if-bound"
  state_timeout               = 30
  adaptive_timeout_start      = 1000
  adaptive_timeout_end        = 2000
  max_states                  = 100
  max_source_nodes            = 20
  max_source_states           = 30
  max_source_connections      = 40
  max_new_connections         = 10
  max_new_connections_seconds = 60
  no_pfsync                   = true
  disable_reply_to            = true
  match_priority              = 1
  set_priority                = 2
  set_priority_low_delay      = 3
  match_tos                   = "lowdelay"
  set_local_tag               = "story31a"
  match_local_tag             = "story31a-input"
  description                 = %[3]q
}

data "opnsense_firewall_filter_rule" "advanced" {
  id = opnsense_firewall_filter_rule.advanced.id
}
`, iface, port, description)
}

func testAccFirewallNATAdvancedConfig(interfaces []string, suffix, port string) string {
	sort.Strings(interfaces)
	quoted := make([]string, len(interfaces))
	for i, v := range interfaces {
		quoted[i] = strconv.Quote(v)
	}
	return fmt.Sprintf(`
resource "opnsense_firewall_nat_port_forward" "advanced" {
  enabled          = false
  sequence         = 987600
  interfaces       = [%s]
  ip_protocol      = "inet"
  protocol         = "tcp"
  source_net       = "any"
  destination_net  = "any"
  destination_port = %[2]q
  target            = "192.0.2.31"
  local_port        = "8443"
  description       = "Story 31.0a disabled DNAT %[3]s"
}

data "opnsense_firewall_nat_port_forward" "advanced" {
  id = opnsense_firewall_nat_port_forward.advanced.id
}
`, strings.Join(quoted, ","), port, suffix)
}

func TestFirewallParityAcceptanceConfigsParse(t *testing.T) {
	tests := map[string]string{
		"alias":  testAccFirewallAliasAdvancedConfig("test", "test"),
		"filter": testAccFirewallFilterAdvancedConfig("lan", "18443", "test"),
		"nat":    testAccFirewallNATAdvancedConfig([]string{"wan", "lan"}, "test", "18443"),
	}

	for name, config := range tests {
		t.Run(name, func(t *testing.T) {
			_, diags := hclsyntax.ParseConfig([]byte(config), name+".tf", hcl.Pos{Line: 1, Column: 1})
			if diags.HasErrors() {
				t.Fatalf("invalid acceptance config: %s", diags.Error())
			}
		})
	}
}

func acceptanceSuffix() string {
	value := strconv.FormatInt(time.Now().UnixNano(), 36)
	if len(value) > 8 {
		return value[len(value)-8:]
	}
	return value
}

func mutateDescription(t *testing.T, address, monad, description string) resource.TestCheckFunc {
	t.Helper()
	return func(state *terraform.State) error {
		entry, ok := state.RootModule().Resources[address]
		if !ok {
			return fmt.Errorf("resource %s not found", address)
		}
		client := acctest.TestClient(t)
		family, getName, setName, applyName := "filter", "getRule", "setRule", "apply"
		if monad == "alias" {
			family, getName, setName, applyName = "alias", "getItem", "setItem", "reconfigure"
		}
		resp, err := client.HTTPClient().Get(fmt.Sprintf("%s/api/firewall/%s/%s/%s", client.BaseURL(), family, getName, entry.Primary.ID))
		if err != nil {
			return err
		}
		body, err := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if err != nil {
			return err
		}
		payload, err := normalizeMutationPayload(body, monad, description)
		if err != nil {
			return err
		}
		req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("%s/api/firewall/%s/%s/%s", client.BaseURL(), family, setName, entry.Primary.ID), bytes.NewReader(payload))
		if err != nil {
			return err
		}
		req.Header.Set("Content-Type", "application/json")
		if err := postMutation(client.HTTPClient(), req, "drift mutation"); err != nil {
			return err
		}
		applyReq, _ := http.NewRequest(http.MethodPost, fmt.Sprintf("%s/api/firewall/%s/%s", client.BaseURL(), family, applyName), nil)
		return postMutation(client.HTTPClient(), applyReq, "drift apply")
	}
}

func normalizeMutationPayload(body []byte, monad, description string) ([]byte, error) {
	var wrapper map[string]map[string]any
	if err := json.Unmarshal(body, &wrapper); err != nil {
		return nil, err
	}
	item := wrapper[monad]
	if item == nil {
		return nil, fmt.Errorf("missing %s", monad)
	}
	var scalarFields, singleFields, multiFields []string
	delimiters := map[string]string{}
	if monad == "alias" {
		scalarFields = []string{"name", "enabled", "updatefreq", "path_expression", "counters"}
		singleFields = []string{"type", "interface"}
		multiFields = []string{"content", "proto", "categories"}
		delimiters["content"] = "\n"
	} else {
		scalarFields = []string{
			"enabled", "sequence", "quick", "interfacenot", "nosync", "allowopts",
			"source_net", "source_port", "source_not", "destination_net", "destination_port", "destination_not",
			"statetimeout", "adaptivestart", "adaptiveend", "max", "max-src-nodes", "max-src-states",
			"max-src-conn", "max-src-conn-rate", "max-src-conn-rates", "nopfsync", "disablereplyto",
			"tag", "tagged", "log",
		}
		singleFields = []string{
			"action", "direction", "ipprotocol", "protocol", "gateway", "sched", "statetype", "state-policy",
			"overload", "shaper1", "shaper2", "replyto", "prio", "set-prio", "set-prio-low", "tos",
		}
		multiFields = []string{"interface", "icmptype", "tcpflags1", "tcpflags2", "categories"}
	}

	payload := make(map[string]string, len(scalarFields)+len(singleFields)+len(multiFields)+1)
	for _, key := range scalarFields {
		payload[key] = scalarString(item[key])
	}
	for _, key := range singleFields {
		payload[key] = selectedSingle(item[key])
	}
	for _, key := range multiFields {
		delimiter := delimiters[key]
		if delimiter == "" {
			delimiter = ","
		}
		payload[key] = strings.Join(selectedMany(item[key]), delimiter)
	}
	payload["description"] = description
	return json.Marshal(map[string]any{monad: payload})
}

func TestNormalizeMutationPayload(t *testing.T) {
	for _, test := range []struct {
		name, monad, body string
		fields            map[string]string
	}{
		{name: "alias", monad: "alias", body: `{"alias":{"name":"a","type":{"asn":{"value":"Autonomous System Numbers","selected":1},"host":{"value":"Host(s)","selected":0}},"content":{"64512":{"value":"64512","selected":"1"},"13335":{"value":"13335","selected":1}},"description":"before","enabled":"1","proto":{"IPv6":{"value":"IPv6","selected":1},"IPv4":{"value":"IPv4","selected":1}},"interface":[],"categories":[],"updatefreq":"","path_expression":"","counters":"0","validationMessage":"response-only"}}`, fields: map[string]string{"name": "a", "type": "asn", "content": "13335\n64512", "enabled": "1", "proto": "IPv4,IPv6", "interface": "", "categories": "", "updatefreq": "", "path_expression": "", "counters": "0", "description": "after"}},
		{name: "filter", monad: "rule", body: `{"rule":{"enabled":"0","sequence":"987600","action":{"pass":{"value":"Pass","selected":1},"block":{"value":"Block","selected":0}},"quick":"1","interface":{"wan":{"value":"WAN","selected":1},"lan":{"value":"LAN","selected":"1"}},"interfacenot":"0","nosync":"1","allowopts":"1","direction":{"in":{"selected":1}},"ipprotocol":{"inet":{"selected":1}},"protocol":{"TCP":{"value":"TCP","selected":1}},"icmptype":[],"source_net":"any","source_port":"","source_not":"0","destination_net":"any","destination_port":"8443","destination_not":"0","gateway":[],"tcpflags1":{"syn":{"selected":1}},"tcpflags2":{"ack":{"selected":1},"syn":{"selected":1}},"sched":[],"statetype":{"keep":{"selected":1}},"state-policy":{"if-bound":{"selected":1}},"statetimeout":"30","adaptivestart":"1000","adaptiveend":"2000","max":"100","max-src-nodes":"20","max-src-states":"30","max-src-conn":"40","max-src-conn-rate":"10","max-src-conn-rates":"60","overload":[],"nopfsync":"1","shaper1":[],"shaper2":[],"disablereplyto":"1","replyto":[],"prio":{"1":{"selected":1}},"set-prio":{"2":{"selected":1}},"set-prio-low":{"3":{"selected":1}},"tos":{"lowdelay":{"selected":1}},"tag":"story31a","tagged":"story31a-input","log":"0","description":"before","categories":[],"validationMessage":"response-only"}}`, fields: map[string]string{"enabled": "0", "sequence": "987600", "action": "pass", "interface": "lan,wan", "protocol": "TCP", "tcpflags1": "syn", "tcpflags2": "ack,syn", "state-policy": "if-bound", "max-src-conn-rates": "60", "description": "after"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			payload, err := normalizeMutationPayload([]byte(test.body), test.monad, "after")
			if err != nil {
				t.Fatal(err)
			}
			var wrapper map[string]map[string]any
			if err := json.Unmarshal(payload, &wrapper); err != nil {
				t.Fatal(err)
			}
			if _, ok := wrapper[test.monad]["validationMessage"]; ok {
				t.Fatal("response-only field was included in mutation payload")
			}
			for key, value := range wrapper[test.monad] {
				if _, ok := value.(string); !ok {
					t.Fatalf("%s=%#v is not a scalar string", key, value)
				}
			}
			for key, want := range test.fields {
				if got, ok := wrapper[test.monad][key].(string); !ok || got != want {
					t.Fatalf("%s=%#v want %q", key, wrapper[test.monad][key], want)
				}
			}
		})
	}
}

func scalarString(value any) string {
	if value == nil {
		return ""
	}
	if result, ok := value.(string); ok {
		return result
	}
	return fmt.Sprint(value)
}

func postMutation(client *http.Client, req *http.Request, operation string) error {
	if transport, ok := client.Transport.(*retryablehttp.RoundTripper); ok {
		source := transport.Client
		retryClient := retryablehttp.NewClient()
		retryClient.HTTPClient = source.HTTPClient
		retryClient.Logger = source.Logger
		retryClient.RetryWaitMin = source.RetryWaitMin
		retryClient.RetryWaitMax = source.RetryWaitMax
		retryClient.RetryMax = source.RetryMax
		retryClient.CheckRetry = source.CheckRetry
		retryClient.Backoff = source.Backoff
		retryClient.PrepareRetry = source.PrepareRetry
		retryClient.RequestLogHook = source.RequestLogHook
		retryClient.ResponseLogHook = source.ResponseLogHook
		retryClient.ErrorHandler = retryablehttp.PassthroughErrorHandler
		client = retryClient.StandardClient()
	}

	resp, requestErr := client.Do(req)
	if resp == nil {
		return fmt.Errorf("%s request failed: %w", operation, requestErr)
	}
	defer func() { _ = resp.Body.Close() }()
	body, readErr := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if readErr != nil {
		return fmt.Errorf("%s HTTP %d: failed reading response: %w", operation, resp.StatusCode, readErr)
	}
	if requestErr != nil || resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("%s HTTP %d: %q", operation, resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return nil
}

func TestPostMutationSurfacesSafeResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = w.Write([]byte(`{"result":"field validation failed"}`))
	}))
	defer server.Close()

	req, err := http.NewRequest(http.MethodPost, server.URL, strings.NewReader(`{"rule":{}}`))
	if err != nil {
		t.Fatal(err)
	}
	req.SetBasicAuth("api-key", "api-secret")
	err = postMutation(server.Client(), req, "drift mutation")
	if err == nil || !strings.Contains(err.Error(), "HTTP 422") || !strings.Contains(err.Error(), "field validation failed") {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(err.Error(), "api-key") || strings.Contains(err.Error(), "api-secret") {
		t.Fatalf("error leaked request credentials: %v", err)
	}
}

func selectedMany(value any) []string {
	if s, ok := value.(string); ok {
		return []string{s}
	}
	raw, ok := value.(map[string]any)
	if !ok {
		return nil
	}
	var out []string
	for key, v := range raw {
		if choice, ok := v.(map[string]any); ok && fmt.Sprint(choice["selected"]) == "1" {
			out = append(out, key)
		}
	}
	sort.Strings(out)
	return out
}

func selectedSingle(value any) string {
	values := selectedMany(value)
	if len(values) > 0 {
		return values[0]
	}
	if s, ok := value.(string); ok {
		return s
	}
	return ""
}

func splitNonEmpty(csv string) []string {
	var out []string
	for _, v := range strings.Split(csv, ",") {
		if v = strings.TrimSpace(v); v != "" {
			out = append(out, v)
		}
	}
	return out
}

func acceptanceVersionAtLeast(version string, required ...int) bool {
	parts := strings.FieldsFunc(version, func(r rune) bool { return r < '0' || r > '9' })
	for i, want := range required {
		if i >= len(parts) {
			return false
		}
		got, err := strconv.Atoi(parts[i])
		if err != nil {
			return false
		}
		if got != want {
			return got > want
		}
	}
	return true
}
