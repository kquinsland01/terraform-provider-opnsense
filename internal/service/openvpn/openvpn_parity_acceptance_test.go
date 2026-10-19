// Copyright (c) Matthew Mellor
// SPDX-License-Identifier: MPL-2.0

package openvpn_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"

	"github.com/matthew-on-git/terraform-provider-opnsense/internal/acctest"
)

// TestAccOpenVPNAdvancedParity is the digest-bound disposable-appliance proof
// entry point. Fixtures are disabled and collision tagged. It covers advanced
// create/read, data-source parity, import/no-diff, update, out-of-band drift,
// explicit collection clears, and exact post-destroy namespace cleanup.
func TestAccOpenVPNAdvancedParity(t *testing.T) {
	suffix := strconv.FormatInt(time.Now().UnixNano(), 36)
	marker := "tf-story31c-" + suffix
	initial := openVPNAdvancedConfig(marker, false)
	cleared := openVPNAdvancedConfig(marker, true)
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		PreCheck:                 openVPNParityPreCheck(t, marker),
		CheckDestroy:             openVPNParityCleanup(t, marker),
		Steps: []resource.TestStep{
			{Config: initial, Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttr("opnsense_openvpn_instance.server", "enabled", "false"),
				resource.TestCheckResourceAttr("opnsense_openvpn_instance.server", "server_ipv6", "2001:db8:31::/64"),
				resource.TestCheckResourceAttr("opnsense_openvpn_instance.server", "data_ciphers_ordered.#", "2"),
				resource.TestCheckResourceAttr("opnsense_openvpn_instance.nopool", "no_pool", "true"),
				resource.TestCheckResourceAttr("opnsense_openvpn_instance.client", "username", marker+"-user"),
				resource.TestCheckResourceAttr("opnsense_openvpn_client_overwrite.advanced", "tunnel_network_v6", "2001:db8:31::9/128"),
				resource.TestCheckResourceAttr("data.opnsense_openvpn_instance.server", "route_metric", "31"),
				resource.TestCheckResourceAttr("data.opnsense_openvpn_client_overwrite.advanced", "wins_servers.#", "1"),
				compareOpenVPNState("opnsense_openvpn_instance.server", "data.opnsense_openvpn_instance.server", "password", "auth_gen_token_secret"),
				compareOpenVPNState("opnsense_openvpn_instance.nopool", "data.opnsense_openvpn_instance.nopool", "password", "auth_gen_token_secret"),
				compareOpenVPNState("opnsense_openvpn_instance.client", "data.opnsense_openvpn_instance.client", "password", "auth_gen_token_secret"),
				compareOpenVPNState("opnsense_openvpn_client_overwrite.advanced", "data.opnsense_openvpn_client_overwrite.advanced"),
			)},
			{ResourceName: "opnsense_openvpn_instance.server", ImportState: true, ImportStateVerify: true},
			{ResourceName: "opnsense_openvpn_instance.client", ImportState: true, ImportStateVerify: true, ImportStateVerifyIgnore: []string{"password", "auth_gen_token_secret"}},
			{ResourceName: "opnsense_openvpn_client_overwrite.advanced", ImportState: true, ImportStateVerify: true},
			{Config: initial, PlanOnly: true},
			{Config: initial, Check: mutateOpenVPNDescription(t, "opnsense_openvpn_instance.server", "instance", "/api/openvpn/instances/set", marker+"-drift"), ExpectNonEmptyPlan: true},
			{Config: initial, PlanOnly: true, ExpectNonEmptyPlan: true},
			{Config: initial, Check: resource.TestCheckResourceAttr("opnsense_openvpn_instance.server", "description", marker+"-server")},
			{Config: initial, PlanOnly: true},
			{Config: cleared, Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttr("opnsense_openvpn_instance.server", "dns_domain.#", "0"),
				resource.TestCheckResourceAttr("opnsense_openvpn_instance.server", "push_excluded_routes.#", "0"),
				resource.TestCheckResourceAttr("opnsense_openvpn_client_overwrite.advanced", "dns_domain_search.#", "0"),
				resource.TestCheckResourceAttr("opnsense_openvpn_client_overwrite.advanced", "wins_servers.#", "0"),
			)},
			{Config: cleared, PlanOnly: true},
		},
	})
}

// TestAccOpenVPNStaticKeyLookup proves the strict GET-only, stateful sensitive
// lookup independently from key generation. The fixture UUID is operator-owned
// and the test performs no mutation.
func TestAccOpenVPNStaticKeyLookup(t *testing.T) {
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		PreCheck:                 openVPNParityPreCheck(t, "static-key-lookup"),
		Steps: []resource.TestStep{{
			Config: fmt.Sprintf(`data "opnsense_openvpn_static_key" "existing" { id = %q }`, os.Getenv("OPNSENSE_OPENVPN_TEST_STATIC_KEY_UUID")),
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttrSet("data.opnsense_openvpn_static_key.existing", "mode"),
				resource.TestCheckResourceAttrSet("data.opnsense_openvpn_static_key.existing", "description"),
				resource.TestCheckResourceAttrSet("data.opnsense_openvpn_static_key.existing", "key"),
			),
		}},
	})
}

func TestOpenVPNAdvancedAcceptanceFixtureCoversAuditedFields(t *testing.T) {
	for _, name := range []string{"OPNSENSE_OPENVPN_TEST_CA", "OPNSENSE_OPENVPN_TEST_CERT", "OPNSENSE_OPENVPN_TEST_CRL", "OPNSENSE_OPENVPN_TEST_STATIC_KEY_UUID", "OPNSENSE_OPENVPN_TEST_LOCAL_GROUP", "OPNSENSE_OPENVPN_TEST_CARP"} {
		t.Setenv(name, name+"-value")
	}
	config := openVPNAdvancedConfig("fixture-marker", false)
	if _, diagnostics := hclsyntax.ParseConfig([]byte(config), "story31c.tf", hcl.Pos{Line: 1, Column: 1}); diagnostics.HasErrors() {
		t.Fatal(diagnostics.Error())
	}
	for _, field := range []string{
		"port_share", "server_ipv6", "no_pool", "bridge_gateway", "bridge_pool", "route", "push_excluded_routes", "crl", "cert_depth", "remote_cert_tls", "use_ocsp",
		"data_ciphers_ordered", "data_ciphers_fallback", "auth_mode", "local_group", "various_flags", "various_push_flags", "push_inactive", "username_as_common_name", "strict_user_cn",
		"username", "password", "reneg_sec", "auth_gen_token", "auth_gen_token_renewal", "auth_gen_token_secret", "provision_exclusive", "route_metric", "register_dns", "dns_domain",
		"dns_domain_search", "ntp_servers", "tun_mtu", "fragment", "mss_fix", "carp_depend_on", "compress_migrate", "ifconfig_pool_persist", "http_proxy", "verify_x509_name",
		"tunnel_network_v6", "route_gateway", "wins_servers",
	} {
		if !strings.Contains(config, field+" =") {
			t.Errorf("advanced acceptance fixture does not configure %s", field)
		}
	}
	cleared := openVPNAdvancedConfig("fixture-marker", true)
	if _, diagnostics := hclsyntax.ParseConfig([]byte(cleared), "story31c-clear.tf", hcl.Pos{Line: 1, Column: 1}); diagnostics.HasErrors() {
		t.Fatal(diagnostics.Error())
	}
	for _, field := range []string{"route", "push_route", "push_excluded_routes", "auth_mode", "various_flags", "various_push_flags", "redirect_gateway", "dns_domain", "dns_domain_search", "dns_servers", "ntp_servers", "wins_servers"} {
		if !strings.Contains(cleared, field+" = []") {
			t.Errorf("explicit-clear fixture does not clear %s", field)
		}
	}
	for _, required := range []string{`bridge_gateway = "10.31.0.1/24"`, `various_flags = ["persist-remote-ip"]`, "description = \"fixture-marker-server-nopool\"\n  protocol = \"udp4\"\n  server = \"10.31.1.0/24\"\n  no_pool = true\n  ca =", "verify_client_cert = \"none\"\n  auth_mode = [\"Local Database\"]"} {
		if !strings.Contains(config, required) {
			t.Errorf("acceptance fixture omitted source-valid requirement %q", required)
		}
	}
	if strings.Contains(config, "persist-tun") {
		t.Error("acceptance fixture uses unsupported various_flags value persist-tun")
	}
	for _, required := range []string{`auth_mode = []`, `verify_client_cert = "require"`, `username_as_common_name = false`, `cert = "OPNSENSE_OPENVPN_TEST_CERT-value"`} {
		if !strings.Contains(cleared, required) {
			t.Errorf("explicit-clear fixture omitted certificate-auth requirement %q", required)
		}
	}
	if strings.Contains(cleared, "username_as_common_name = true") {
		t.Error("explicit-clear server retained username authentication while clearing auth_mode")
	}
}

func openVPNParityPreCheck(t *testing.T, marker string) func() {
	return func() {
		acctest.PreCheck(t)
		if os.Getenv("OPNSENSE_OPENVPN_PARITY") != "1" {
			t.Skip("set OPNSENSE_OPENVPN_PARITY=1 only through the Story 31.0c disposable runner")
		}
		for _, name := range []string{"OPNSENSE_DISPOSABLE_VM_NAME", "OPNSENSE_DISPOSABLE_VM_UUID", "OPNSENSE_OPENVPN_IDENTITY_ATTESTATION", "OPNSENSE_OPENVPN_TEST_CA", "OPNSENSE_OPENVPN_TEST_CERT", "OPNSENSE_OPENVPN_TEST_CRL", "OPNSENSE_OPENVPN_TEST_STATIC_KEY_UUID", "OPNSENSE_OPENVPN_TEST_LOCAL_GROUP", "OPNSENSE_OPENVPN_TEST_CARP"} {
			requiredOpenVPNEnv(t, name)
		}
		wantAttestation := os.Getenv("OPNSENSE_DISPOSABLE_VM_NAME") + ":" + os.Getenv("OPNSENSE_DISPOSABLE_VM_UUID")
		if os.Getenv("OPNSENSE_OPENVPN_IDENTITY_ATTESTATION") != wantAttestation {
			t.Fatal("identity attestation does not match the expected disposable VM name and UUID")
		}
		client := acctest.TestClient(t)
		resp, err := client.HTTPClient().Get(client.BaseURL() + "/api/core/firmware/status")
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = resp.Body.Close() }()
		var status struct {
			Product struct {
				Name    string `json:"product_name"`
				ID      string `json:"product_id"`
				Version string `json:"product_version"`
			} `json:"product"`
		}
		if resp.StatusCode != http.StatusOK || json.NewDecoder(resp.Body).Decode(&status) != nil || !strings.Contains(strings.ToLower(status.Product.Name+" "+status.Product.ID), "opnsense") || status.Product.Version != "26.7.7" {
			t.Fatalf("Story 31.0c live proof requires verified OPNsense 26.7.7; got HTTP %d identity=%q version=%q", resp.StatusCode, status.Product.Name+" "+status.Product.ID, status.Product.Version)
		}
		if marker != "static-key-lookup" {
			for _, endpoint := range []string{"/api/openvpn/instances/search", "/api/openvpn/client_overwrites/search"} {
				total, _, err := openVPNSearch(client.HTTPClient(), client.BaseURL()+endpoint, marker)
				if err != nil {
					t.Fatal(err)
				}
				if total != 0 {
					t.Fatalf("dirty Story 31.0c fixture namespace endpoint=%s marker=%q total=%d", endpoint, marker, total)
				}
			}
		}
	}
}

func requiredOpenVPNEnv(t *testing.T, name string) string {
	t.Helper()
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		t.Fatalf("%s is required by the disposable Story 31.0c harness", name)
	}
	return value
}

func openVPNAdvancedConfig(marker string, clearCollections bool) string {
	collections := map[string]string{
		"route": `["192.0.2.0/24"]`, "push_route": `["198.51.100.0/24"]`, "push_excluded_routes": `["203.0.113.0/24"]`,
		"auth_mode": `["Local Database"]`, "various_flags": `["persist-remote-ip"]`, "various_push_flags": `["block-outside-dns"]`,
		"redirect_gateway": `["def1"]`, "dns_domain": `["example.test"]`, "dns_domain_search": `["search.example.test"]`,
		"dns_servers": `["192.0.2.53"]`, "ntp_servers": `["192.0.2.123"]`, "wins_servers": `["192.0.2.44"]`,
	}
	verifyClientCert := "none"
	usernameAsCommonName := "true"
	if clearCollections {
		for key := range collections {
			collections[key] = "[]"
		}
		verifyClientCert = "require"
		usernameAsCommonName = "false"
	}
	return fmt.Sprintf(`
resource "opnsense_openvpn_instance" "server" {
  enabled = false
  role = "server"
  description = %[1]q
  dev_type = "tap"
  protocol = "tcp4"
  port = "1195"
  port_share = "127.0.0.1:8443"
  local = "192.0.2.10"
  server = "10.31.0.0/24"
  server_ipv6 = "2001:db8:31::/64"
  topology = "p2p"
  bridge_gateway = "10.31.0.1/24"
  bridge_pool = "10.31.0.10-10.31.0.20"
  route = %[2]s
  push_route = %[3]s
  push_excluded_routes = %[4]s
  ca = %[5]q
  cert = %[6]q
  crl = %[7]q
  cert_depth = "3"
  remote_cert_tls = true
  verify_client_cert = %[24]q
  use_ocsp = true
  tls_key = %[8]q
  data_ciphers = ["AES-256-GCM", "CHACHA20-POLY1305"]
  data_ciphers_ordered = ["AES-256-GCM", "CHACHA20-POLY1305"]
  data_ciphers_fallback = "AES-256-CBC"
  auth = "SHA512"
  auth_mode = %[9]s
  local_group = %[10]q
  various_flags = %[11]s
  various_push_flags = %[12]s
  push_inactive = 120
  username_as_common_name = %[25]s
  strict_user_cn = "2"
  max_clients = 37
  keepalive_interval = 11
  keepalive_timeout = 61
  reneg_sec = 3601
  auth_gen_token = 7201
  auth_gen_token_renewal = 1801
  auth_gen_token_secret = "story31c-token-secret"
  provision_exclusive = true
  redirect_gateway = %[13]s
  route_metric = 31
  register_dns = true
  dns_domain = %[14]s
  dns_domain_search = %[15]s
  dns_servers = %[16]s
  ntp_servers = %[17]s
  tun_mtu = 1401
  fragment = 1301
  mss_fix = true
  carp_depend_on = %[18]q
  compress_migrate = true
  ifconfig_pool_persist = true
  verb = "7"
}

resource "opnsense_openvpn_instance" "nopool" {
  enabled = false
  role = "server"
  description = "%[1]s-nopool"
  protocol = "udp4"
  server = "10.31.1.0/24"
  no_pool = true
  ca = %[5]q
  cert = %[6]q
  verify_client_cert = "none"
  auth_mode = ["Local Database"]
  data_ciphers = ["AES-256-GCM"]
  data_ciphers_ordered = ["AES-256-GCM"]
}

resource "opnsense_openvpn_instance" "client" {
  enabled = false
  role = "client"
  description = %[19]q
  remote = "vpn.example.test"
  protocol = "udp4"
  ca = %[5]q
  cert = %[6]q
  verify_client_cert = "none"
  tls_key = %[8]q
  data_ciphers = ["AES-256-GCM"]
  data_ciphers_ordered = ["AES-256-GCM"]
  username = %[20]q
  password = "story31c-password"
  http_proxy = "proxy.example.test:3128"
  verify_x509_name = "vpn.example.test"
}

resource "opnsense_openvpn_client_overwrite" "advanced" {
  enabled = false
  common_name = %[21]q
  description = %[22]q
  servers = [opnsense_openvpn_instance.server.id]
  block = true
  push_reset = true
  tunnel_network = "10.31.0.9/32"
  tunnel_network_v6 = "2001:db8:31::9/128"
  local_networks = %[2]s
  remote_networks = %[3]s
  route_gateway = "10.31.0.1"
  redirect_gateway = %[13]s
  register_dns = true
  dns_domain = %[14]s
  dns_domain_search = %[15]s
  dns_servers = %[16]s
  ntp_servers = %[17]s
  wins_servers = %[23]s
}

data "opnsense_openvpn_instance" "server" { id = opnsense_openvpn_instance.server.id }
data "opnsense_openvpn_instance" "nopool" { id = opnsense_openvpn_instance.nopool.id }
data "opnsense_openvpn_instance" "client" { id = opnsense_openvpn_instance.client.id }
data "opnsense_openvpn_client_overwrite" "advanced" { id = opnsense_openvpn_client_overwrite.advanced.id }
data "opnsense_openvpn_static_key" "existing" { id = %[8]q }
`, marker+"-server", collections["route"], collections["push_route"], collections["push_excluded_routes"], os.Getenv("OPNSENSE_OPENVPN_TEST_CA"), os.Getenv("OPNSENSE_OPENVPN_TEST_CERT"), os.Getenv("OPNSENSE_OPENVPN_TEST_CRL"), os.Getenv("OPNSENSE_OPENVPN_TEST_STATIC_KEY_UUID"), collections["auth_mode"], os.Getenv("OPNSENSE_OPENVPN_TEST_LOCAL_GROUP"), collections["various_flags"], collections["various_push_flags"], collections["redirect_gateway"], collections["dns_domain"], collections["dns_domain_search"], collections["dns_servers"], collections["ntp_servers"], os.Getenv("OPNSENSE_OPENVPN_TEST_CARP"), marker+"-client", marker+"-user", marker+"-cn", marker+"-overwrite", collections["wins_servers"], verifyClientCert, usernameAsCommonName)
}

func compareOpenVPNState(resourceAddress, dataSourceAddress string, ignored ...string) resource.TestCheckFunc {
	return func(state *terraform.State) error {
		resourceState, ok := state.RootModule().Resources[resourceAddress]
		if !ok {
			return fmt.Errorf("resource %s not found", resourceAddress)
		}
		dataSourceState, ok := state.RootModule().Resources[dataSourceAddress]
		if !ok {
			return fmt.Errorf("data source %s not found", dataSourceAddress)
		}
		ignore := make(map[string]struct{}, len(ignored))
		for _, name := range ignored {
			ignore[name] = struct{}{}
		}
		for name, want := range resourceState.Primary.Attributes {
			if _, skip := ignore[name]; skip {
				continue
			}
			got, exists := dataSourceState.Primary.Attributes[name]
			if !exists {
				return fmt.Errorf("data source %s omitted resource attribute %s", dataSourceAddress, name)
			}
			if got != want {
				return fmt.Errorf("data source %s attribute %s=%q, want resource value %q", dataSourceAddress, name, got, want)
			}
		}
		return nil
	}
}

func mutateOpenVPNDescription(t *testing.T, address, monad, endpoint, description string) resource.TestCheckFunc {
	return func(state *terraform.State) error {
		instance, ok := state.RootModule().Resources[address]
		if !ok {
			return fmt.Errorf("resource %s not found", address)
		}
		client := acctest.TestClient(t)
		payload, _ := json.Marshal(map[string]any{monad: map[string]string{"description": description}})
		request, err := http.NewRequest(http.MethodPost, client.BaseURL()+endpoint+"/"+instance.Primary.ID, bytes.NewReader(payload))
		if err != nil {
			return err
		}
		request.Header.Set("Content-Type", "application/json")
		if err := openVPNPOST(client.HTTPClient(), request, "drift mutation"); err != nil {
			return err
		}
		apply, _ := http.NewRequest(http.MethodPost, client.BaseURL()+"/api/openvpn/service/reconfigure", nil)
		return openVPNPOST(client.HTTPClient(), apply, "drift reconfigure")
	}
}

func openVPNPOST(client *http.Client, request *http.Request, operation string) error {
	response, err := client.Do(request)
	if err != nil {
		return fmt.Errorf("%s: %w", operation, err)
	}
	defer func() { _ = response.Body.Close() }()
	body, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("%s HTTP %d: %s", operation, response.StatusCode, strings.TrimSpace(string(body)))
	}
	return nil
}

func openVPNParityCleanup(t *testing.T, marker string) resource.TestCheckFunc {
	return func(_ *terraform.State) error {
		client := acctest.TestClient(t)
		for _, owner := range []struct{ name, endpoint string }{{"instance", "/api/openvpn/instances/search"}, {"client-overwrite", "/api/openvpn/client_overwrites/search"}} {
			total, ids, err := openVPNSearch(client.HTTPClient(), client.BaseURL()+owner.endpoint, marker)
			if err != nil {
				return err
			}
			if total != 0 {
				return fmt.Errorf("OpenVPN parity cleanup owner=%s marker=%q total=%d ids=%v", owner.name, marker, total, ids)
			}
			t.Logf("OPENVPN_PARITY_CLEANUP owner=%s marker=%s total=%d", owner.name, marker, total)
		}
		return nil
	}
}

func openVPNSearch(client *http.Client, endpoint, marker string) (int, []string, error) {
	payload, _ := json.Marshal(map[string]any{"current": 1, "rowCount": 500, "searchPhrase": marker})
	request, _ := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(payload))
	request.Header.Set("Content-Type", "application/json")
	response, err := client.Do(request)
	if err != nil {
		return 0, nil, err
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return 0, nil, fmt.Errorf("cleanup search %s returned HTTP %d", endpoint, response.StatusCode)
	}
	var body struct {
		Total int `json:"total"`
		Rows  []struct {
			UUID string `json:"uuid"`
		} `json:"rows"`
	}
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		return 0, nil, err
	}
	ids := make([]string, 0, len(body.Rows))
	for _, row := range body.Rows {
		if row.UUID != "" {
			ids = append(ids, row.UUID)
		}
	}
	return body.Total, ids, nil
}
