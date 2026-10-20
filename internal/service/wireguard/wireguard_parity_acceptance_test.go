// Copyright (c) Matthew Mellor
// SPDX-License-Identifier: MPL-2.0

package wireguard_test

import (
	"context"
	"crypto/ecdh"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/matthew-on-git/terraform-provider-opnsense/internal/acctest"
	"github.com/matthew-on-git/terraform-provider-opnsense/pkg/opnsense"
)

var wgSettingsOpts = opnsense.ReqOpts{GetEndpoint: "/api/wireguard/general/get", UpdateEndpoint: "/api/wireguard/general/set", Monad: "general", ReconfigureEndpoint: "/api/wireguard/service/reconfigure", StrictReconfigureResult: true}

func wgItemOpts(kind string) opnsense.ReqOpts {
	return opnsense.ReqOpts{GetEndpoint: "/api/wireguard/" + kind + "/get_" + kind, SearchEndpoint: "/api/wireguard/" + kind + "/search_" + kind, UpdateEndpoint: "/api/wireguard/" + kind + "/set_" + kind, DeleteEndpoint: "/api/wireguard/" + kind + "/del_" + kind, Monad: kind, ReconfigureEndpoint: "/api/wireguard/service/reconfigure", StrictReconfigureResult: true, StrictDeleteResponse: true, StrictMissingObjectResponse: true}
}

func TestAccWireGuardParity(t *testing.T) {
	marker := "tf_story31e_" + strconv.FormatInt(time.Now().UnixNano(), 36)
	initial := wgParityConfig(marker, false)
	cleared := wgParityConfig(marker, true)
	var snapshot []byte
	var beforeDestroy []byte
	initialized := false
	workingDir := t.TempDir()
	cliOptions := &resource.AdditionalCLIOptions{}
	var minimalInstance, minimalPublic string
	minimalCheck := func(s *terraform.State) error {
		if err := wgRawAPICheck(t)(s); err != nil {
			return err
		}
		server := s.RootModule().Resources["opnsense_wireguard_server.parity"]
		instance, public := server.Primary.Attributes["instance"], server.Primary.Attributes["public_key"]
		if instance == "" {
			return fmt.Errorf("minimal server API-assigned instance is missing")
		}
		if minimalInstance == "" {
			minimalInstance, minimalPublic = instance, public
		} else if instance != minimalInstance || public != minimalPublic {
			return fmt.Errorf("unrelated minimal update changed generated key/instance")
		}
		return nil
	}

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		WorkingDir:               workingDir,
		AdditionalCLIOptions:     cliOptions,
		PreCheck: func() {
			wgPreCheck(t)
			if initialized {
				return
			}
			initialized = true
			client := acctest.TestClient(t)
			var err error
			snapshot, err = wgSettingsSnapshot(client)
			if err != nil {
				t.Fatal(err)
			}
			for _, kind := range []string{"server", "client"} {
				rows, searchErr := wgTagged(client, kind, marker)
				if searchErr != nil || len(rows) != 0 {
					t.Fatalf("fixture preflight failed for %s", kind)
				}
			}
			t.Cleanup(func() {
				// Rollback never masks Terraform-delete failure: report the original
				// tagged leftovers before any best-effort deletion.
				for _, kind := range []string{"server", "client"} {
					rows, searchErr := wgTagged(client, kind, marker)
					if searchErr != nil {
						t.Errorf("cleanup search %s: %v", kind, searchErr)
						continue
					}
					if len(rows) > 0 {
						t.Errorf("provider left %d tagged %s objects before rollback", len(rows), kind)
					}
					for _, id := range rows {
						if deleteErr := opnsense.Delete(context.Background(), client, wgItemOpts(kind), id); deleteErr != nil {
							t.Errorf("rollback %s UUID %s: %v", kind, id, deleteErr)
						}
					}
					remaining, searchErr := wgTagged(client, kind, marker)
					if searchErr != nil || len(remaining) > 0 {
						t.Errorf("final cleanup %s has UUIDs %v: %v", kind, remaining, searchErr)
					} else {
						t.Logf("WIREGUARD_PARITY_CLEANUP namespace=%s total=0", kind)
					}
				}
				var original map[string]string
				if err := json.Unmarshal(snapshot, &original); err != nil {
					t.Error(err)
					return
				}
				if err := opnsense.UpdateSingleton(context.Background(), client, wgSettingsOpts, &original); err != nil {
					t.Errorf("restore settings: %v", err)
					return
				}
				after, err := wgSettingsSnapshot(client)
				if err != nil || string(after) != string(snapshot) {
					t.Errorf("settings restoration mismatch: %v", err)
					return
				}
				t.Log("WIREGUARD_SETTINGS_RESTORE equal=true")
			})
		},
		CheckDestroy: func(*terraform.State) error {
			client := acctest.TestClient(t)
			for _, kind := range []string{"server", "client"} {
				rows, err := wgTagged(client, kind, marker)
				if err != nil {
					return err
				}
				if len(rows) != 0 {
					return fmt.Errorf("Terraform delete left %s UUIDs %v", kind, rows)
				}
				t.Logf("WIREGUARD_TERRAFORM_DESTROY namespace=%s total=0", kind)
			}
			after, err := wgSettingsSnapshot(client)
			if err != nil {
				return err
			}
			if len(beforeDestroy) == 0 {
				// A prior lifecycle failure has no success-path snapshot. Cleanup still
				// verifies exact restoration, but cannot claim a destroy comparison.
				return nil
			}
			if string(after) != string(beforeDestroy) {
				return fmt.Errorf("singleton destroy changed WireGuard settings")
			}
			t.Log("WIREGUARD_SINGLETON_DESTROY unchanged=true")
			return nil
		},
		Steps: []resource.TestStep{
			{Config: wgSettingsConfig(), ResourceName: "opnsense_wireguard_settings.parity", ImportState: true, ImportStateId: "wireguard_settings", ImportStatePersist: true},
			{Config: wgSettingsConfig(), PreConfig: func() {
				wgInjectStaleEnabled(t, workingDir)
				cliOptions.Plan.NoRefresh = true
			}, ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction("opnsense_wireguard_settings.parity", plancheck.ResourceActionUpdate)}}, Check: func(s *terraform.State) error {
				if err := resource.TestCheckResourceAttr("opnsense_wireguard_settings.parity", "enabled", "false")(s); err != nil {
					return err
				}
				after, err := wgSettingsSnapshot(acctest.TestClient(t))
				if err != nil {
					return err
				}
				if string(after) != string(snapshot) {
					return fmt.Errorf("singleton Update changed disabled settings snapshot")
				}
				beforeDestroy = after
				t.Log("WIREGUARD_SINGLETON_UPDATE terraform_core=true enabled=0 refresh_disabled_fixture=true")
				return nil
			}},
			{Config: wgSettingsConfig(), PreConfig: func() { cliOptions.Plan.NoRefresh = false }, PlanOnly: true},
			{Config: wgMinimalConfig(marker), Check: minimalCheck},
			{Config: wgMinimalConfig(marker), PlanOnly: true},
			{Config: wgMinimalConfig(marker + "_renamed"), Check: minimalCheck},
			{Config: wgMinimalConfig(marker + "_renamed"), PlanOnly: true},
			{Config: initial, Check: resource.TestCheckResourceAttr("data.opnsense_wireguard_peer.parity", "servers.#", "1")},
			{Config: initial, Check: resource.ComposeAggregateTestCheckFunc(wgCompareState, wgRawAPICheck(t))},
			{ResourceName: "opnsense_wireguard_peer.parity", ImportState: true, ImportStateVerify: true, ImportStateVerifyIgnore: []string{"psk"}},
			{ResourceName: "opnsense_wireguard_server.parity", ImportState: true, ImportStateVerify: true, ImportStateVerifyIgnore: []string{"private_key"}},
			{Config: initial, PlanOnly: true},
			{Config: initial, Check: wgDrift(t, marker), ExpectNonEmptyPlan: true},
			{Config: initial, PlanOnly: true, ExpectNonEmptyPlan: true},
			{Config: initial, Check: resource.ComposeAggregateTestCheckFunc(wgCompareState, wgRawAPICheck(t))},
			{Config: initial, PlanOnly: true},
			{Config: cleared, Check: resource.ComposeAggregateTestCheckFunc(resource.TestCheckResourceAttr("opnsense_wireguard_server.parity", "peers.#", "0"), resource.TestCheckResourceAttr("opnsense_wireguard_server.parity", "dns.#", "0"), resource.TestCheckResourceAttr("opnsense_wireguard_server.parity", "tunnel_address.#", "0"))},
			{Config: cleared, Check: wgCompareState},
			{Config: cleared, PlanOnly: true},
			{Config: initial},
			{Config: initial, Check: func(s *terraform.State) error {
				if err := wgCompareState(s); err != nil {
					return err
				}
				var err error
				beforeDestroy, err = wgSettingsSnapshot(acctest.TestClient(t))
				return err
			}},
		},
	})
}

func wgPreCheck(t *testing.T) {
	t.Helper()
	acctest.PreCheck(t)
	if os.Getenv("OPNSENSE_WIREGUARD_PARITY") != "1" {
		t.Skip("requires digest-bound disposable WireGuard runner")
	}
	name, uuid := os.Getenv("OPNSENSE_DISPOSABLE_VM_NAME"), os.Getenv("OPNSENSE_DISPOSABLE_VM_UUID")
	if name != "opnsense-acceptance-102" || uuid == "" || os.Getenv("OPNSENSE_WIREGUARD_IDENTITY_ATTESTATION") != name+":"+uuid {
		t.Fatal("disposable identity attestation mismatch")
	}
	for _, key := range []string{"OPNSENSE_WIREGUARD_PRIVATE_KEY", "OPNSENSE_WIREGUARD_PSK", "OPNSENSE_WIREGUARD_PUBLIC_KEY"} {
		if os.Getenv(key) == "" {
			t.Fatalf("missing %s", key)
		}
	}
	// Independently bind the exact URI used inside the test container before
	// constructing any credentialed client or making firmware/API requests.
	endpointCheck := exec.Command("python3", "-c", `
import hashlib, re, socket, ssl, sys, urllib.parse
uri = urllib.parse.urlsplit(sys.stdin.readline().strip())
expected = sys.stdin.readline().strip()
if uri.scheme != "https" or not uri.hostname or not re.fullmatch(r"[0-9a-f]{64}", expected):
    raise SystemExit("invalid endpoint attestation")
context = ssl.SSLContext(ssl.PROTOCOL_TLS_CLIENT)
context.check_hostname = False
context.verify_mode = ssl.CERT_NONE
with socket.create_connection((uri.hostname, uri.port or 443), timeout=15) as sock:
    with context.wrap_socket(sock, server_hostname=uri.hostname) as tls:
        if hashlib.sha256(tls.getpeercert(binary_form=True)).hexdigest() != expected:
            raise SystemExit("endpoint TLS fingerprint mismatch")
`)
	endpointCheck.Stdin = strings.NewReader(os.Getenv("OPNSENSE_URI") + "\n" + os.Getenv("OPNSENSE_WIREGUARD_TLS_SHA256") + "\n")
	if err := endpointCheck.Run(); err != nil {
		t.Fatal("attested endpoint TLS handshake failed")
	}
	client := acctest.TestClient(t)
	response, err := client.HTTPClient().Get(client.BaseURL() + "/api/core/firmware/status")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = response.Body.Close() }()
	var status struct {
		Product struct {
			Name    string `json:"product_name"`
			ID      string `json:"product_id"`
			Version string `json:"product_version"`
		} `json:"product"`
	}
	if response.StatusCode != http.StatusOK || json.NewDecoder(response.Body).Decode(&status) != nil || !strings.Contains(strings.ToLower(status.Product.Name+" "+status.Product.ID), "opnsense") || status.Product.Version != "26.7.7" {
		t.Fatal("requires verified OPNsense 26.7.7")
	}
	settings, err := opnsense.GetSingleton[map[string]string](context.Background(), client, wgSettingsOpts)
	if err != nil {
		t.Fatal(err)
	}
	if (*settings)["enabled"] != "0" {
		t.Fatal("acceptance requires WireGuard daemon already disabled; refusing to enable or alter active daemon")
	}
}

func wgSettingsSnapshot(client *opnsense.Client) ([]byte, error) {
	value, err := opnsense.GetSingleton[map[string]string](context.Background(), client, wgSettingsOpts)
	if err != nil {
		return nil, err
	}
	return json.Marshal(value)
}

func wgTagged(client *opnsense.Client, kind, marker string) ([]string, error) {
	rows, err := opnsense.Search[struct {
		UUID string `json:"uuid"`
		Name string `json:"name"`
	}](context.Background(), client, wgItemOpts(kind), opnsense.SearchParams{})
	if err != nil {
		return nil, err
	}
	var matches []string
	for _, row := range rows {
		if strings.HasPrefix(row.Name, marker) {
			if row.UUID == "" {
				return nil, fmt.Errorf("tagged %s has empty UUID", kind)
			}
			matches = append(matches, row.UUID)
		}
	}
	return matches, nil
}

func wgCompareState(s *terraform.State) error {
	for _, kind := range []string{"peer", "server", "settings"} {
		name := "opnsense_wireguard_" + kind + ".parity"
		owner, ok := s.RootModule().Resources[name]
		if !ok {
			return fmt.Errorf("missing resource %s", name)
		}
		lookup, ok := s.RootModule().Resources["data."+name]
		if !ok {
			return fmt.Errorf("missing data source %s", name)
		}
		for key, value := range owner.Primary.Attributes {
			if key == "%" || key == "psk" || key == "private_key" || key == "description" {
				continue
			}
			if lookup.Primary.Attributes[key] != value {
				return fmt.Errorf("%s field %s differs between resource and data source", kind, key)
			}
		}
		for _, secret := range []string{"psk", "private_key"} {
			if _, exists := lookup.Primary.Attributes[secret]; exists {
				return fmt.Errorf("data source exposed %s", secret)
			}
		}
	}
	return nil
}

func wgDrift(t *testing.T, marker string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		client := acctest.TestClient(t)
		for _, item := range []struct {
			kind, resource string
			payload        map[string]string
		}{{"client", "peer", map[string]string{"name": marker + "_drift_peer", "keepalive": "31"}}, {"server", "server", map[string]string{"name": marker + "_drift_server", "mtu": "1430"}}} {
			owner := s.RootModule().Resources["opnsense_wireguard_"+item.resource+".parity"]
			if owner == nil {
				return fmt.Errorf("missing drift owner")
			}
			if item.kind == "client" {
				remote, err := opnsense.Get[struct {
					Servers map[string]struct {
						Selected int `json:"selected"`
					} `json:"servers"`
				}](context.Background(), client, wgItemOpts("client"), owner.Primary.ID)
				if err != nil {
					return err
				}
				var servers []string
				for id, selection := range remote.Servers {
					if selection.Selected == 1 {
						servers = append(servers, id)
					}
				}
				sort.Strings(servers)
				item.payload["servers"] = strings.Join(servers, ",")
			}
			if err := opnsense.Update(context.Background(), client, wgItemOpts(item.kind), &item.payload, owner.Primary.ID); err != nil {
				return err
			}
		}
		return nil
	}
}

func wgSettingsConfig() string {
	return `resource "opnsense_wireguard_settings" "parity" { enabled = false }
data "opnsense_wireguard_settings" "parity" { depends_on = [opnsense_wireguard_settings.parity] }
`
}

func wgParityConfig(marker string, cleared bool) string {
	dns, tunnel, peers, gateway, disable := `["192.0.2.53", "2001:db8::53"]`, `["192.0.2.1/24", "2001:db8::1/64"]`, `[opnsense_wireguard_peer.parity.id]`, `"192.0.2.254"`, `true`
	if cleared {
		dns, tunnel, peers, gateway, disable = `[]`, `[]`, `[]`, `""`, `false`
	}
	return wgSettingsConfig() + fmt.Sprintf(`
resource "opnsense_wireguard_peer" "parity" {
 enabled = false
 name = %q
 public_key = %q
 psk = %q
 tunnel_address = ["192.0.2.2/32", "2001:db8::2/128"]
 server_address = "endpoint.example.test"
 server_port = 51831
 keepalive = 25
}
resource "opnsense_wireguard_server" "parity" {
 enabled = false
 name = %q
 private_key = %q
 public_key = %q
 port = 51832
 mtu = 1420
 dns = %s
 tunnel_address = %s
 peers = %s
 disable_routes = %s
 gateway = %s
}
data "opnsense_wireguard_peer" "parity" {
 id = opnsense_wireguard_peer.parity.id
 depends_on = [opnsense_wireguard_server.parity]
}
data "opnsense_wireguard_server" "parity" { id = opnsense_wireguard_server.parity.id }
`, marker+"_peer", os.Getenv("OPNSENSE_WIREGUARD_PUBLIC_KEY"), os.Getenv("OPNSENSE_WIREGUARD_PSK"), marker+"_server", os.Getenv("OPNSENSE_WIREGUARD_PRIVATE_KEY"), wgServerPublicKey(), dns, tunnel, peers, disable, gateway)
}

func wgServerPublicKey() string {
	raw, err := base64.StdEncoding.DecodeString(os.Getenv("OPNSENSE_WIREGUARD_PRIVATE_KEY"))
	if err != nil {
		return ""
	}
	key, err := ecdh.X25519().NewPrivateKey(raw)
	if err != nil {
		return ""
	}
	return base64.StdEncoding.EncodeToString(key.PublicKey().Bytes())
}

// wgInjectStaleEnabled changes only the isolated test workspace's prior state.
// The appliance remains disabled throughout; Terraform Core must call Update
// to reconcile this stale true fixture to the configured false value.
func wgInjectStaleEnabled(t *testing.T, workingDir string) {
	t.Helper()
	paths, err := filepath.Glob(filepath.Join(workingDir, "work*", "terraform.tfstate"))
	if err != nil || len(paths) != 1 {
		t.Fatalf("expected one isolated Terraform workspace state: matches=%d error=%v", len(paths), err)
	}
	path := paths[0]
	root, err := os.OpenRoot(workingDir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = root.Close() }()
	relative, err := filepath.Rel(workingDir, path)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := root.ReadFile(relative)
	if err != nil {
		t.Fatal(err)
	}
	var state map[string]json.RawMessage
	if err = json.Unmarshal(raw, &state); err != nil {
		t.Fatal(err)
	}
	var resources []struct {
		Mode      string `json:"mode"`
		Type      string `json:"type"`
		Name      string `json:"name"`
		Instances []struct {
			Attributes map[string]json.RawMessage `json:"attributes"`
		} `json:"instances"`
	}
	if err = json.Unmarshal(state["resources"], &resources); err != nil {
		t.Fatal(err)
	}
	// Preserve the complete state envelope, resource metadata and provider-private
	// fields: edit the attribute in raw maps rather than round-tripping a subset.
	var rawResources []map[string]json.RawMessage
	if err = json.Unmarshal(state["resources"], &rawResources); err != nil {
		t.Fatal(err)
	}
	matches := 0
	for i, resource := range resources {
		if resource.Mode != "managed" || resource.Type != "opnsense_wireguard_settings" || resource.Name != "parity" {
			continue
		}
		if len(resource.Instances) != 1 || string(resource.Instances[0].Attributes["enabled"]) != "false" {
			t.Fatal("expected one imported disabled singleton")
		}
		var instances []map[string]json.RawMessage
		if err = json.Unmarshal(rawResources[i]["instances"], &instances); err != nil {
			t.Fatal(err)
		}
		resource.Instances[0].Attributes["enabled"] = json.RawMessage("true")
		instances[0]["attributes"], err = json.Marshal(resource.Instances[0].Attributes)
		if err != nil {
			t.Fatal(err)
		}
		rawResources[i]["instances"], err = json.Marshal(instances)
		if err != nil {
			t.Fatal(err)
		}
		matches++
	}
	if matches != 1 {
		t.Fatal("expected exactly one imported settings fixture")
	}
	state["resources"], err = json.Marshal(rawResources)
	if err != nil {
		t.Fatal(err)
	}
	raw, err = json.Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	if err = root.WriteFile(relative, raw, 0o600); err != nil {
		t.Fatal(err)
	}
}

func wgMinimalConfig(marker string) string {
	return wgSettingsConfig() + fmt.Sprintf(`
resource "opnsense_wireguard_peer" "parity" {
 enabled = false
 name = %q
 public_key = %q
 tunnel_address = ["192.0.2.2/32"]
}
resource "opnsense_wireguard_server" "parity" {
 enabled = false
 name = %q
 private_key = %q
}
data "opnsense_wireguard_peer" "parity" { id = opnsense_wireguard_peer.parity.id }
data "opnsense_wireguard_server" "parity" { id = opnsense_wireguard_server.parity.id }
`, marker+"_peer", os.Getenv("OPNSENSE_WIREGUARD_PUBLIC_KEY"), marker+"_server", os.Getenv("OPNSENSE_WIREGUARD_PRIVATE_KEY"))
}

// Independent oracle: do not use either owner's shared model/decoder. Compare
// material raw wire scalars and selectors directly to Terraform's persisted state.
func wgRawAPICheck(t *testing.T) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		client := acctest.TestClient(t)
		for _, kind := range []string{"peer", "server"} {
			wire := "client"
			fields := map[string]string{"enabled": "enabled", "name": "name", "pubkey": "public_key", "serveraddress": "server_address", "serverport": "server_port", "keepalive": "keepalive", "tunneladdress": "tunnel_address", "servers": "servers"}
			if kind == "server" {
				wire = "server"
				fields = map[string]string{"enabled": "enabled", "name": "name", "instance": "instance", "pubkey": "public_key", "port": "port", "mtu": "mtu", "dns": "dns", "tunneladdress": "tunnel_address", "peers": "peers", "disableroutes": "disable_routes", "gateway": "gateway"}
			}
			state := s.RootModule().Resources["opnsense_wireguard_"+kind+".parity"]
			if state == nil {
				return fmt.Errorf("missing raw API owner")
			}
			remote, err := opnsense.Get[map[string]json.RawMessage](context.Background(), client, wgItemOpts(wire), state.Primary.ID)
			if err != nil {
				return err
			}
			for key, attr := range fields {
				raw, ok := (*remote)[key]
				if !ok {
					return fmt.Errorf("raw API missing %s.%s", kind, key)
				}
				if attr == "dns" || attr == "tunnel_address" || attr == "peers" || attr == "servers" {
					got := []string{}
					var selections map[string]struct {
						Value    string `json:"value"`
						Selected int    `json:"selected"`
					}
					if string(raw) != "[]" {
						if err := json.Unmarshal(raw, &selections); err != nil {
							return err
						}
						for member, item := range selections {
							if item.Selected == 1 && (member != "" || item.Value != "") {
								got = append(got, member)
							}
						}
					}
					want := []string{}
					for k, v := range state.Primary.Attributes {
						if strings.HasPrefix(k, attr+".") && k != attr+".#" {
							want = append(want, v)
						}
					}
					sort.Strings(got)
					sort.Strings(want)
					if strings.Join(got, ",") != strings.Join(want, ",") {
						return fmt.Errorf("raw API %s.%s differs", kind, key)
					}
				} else {
					var got string
					if err := json.Unmarshal(raw, &got); err != nil {
						return err
					}
					if attr == "enabled" || attr == "disable_routes" {
						got = strconv.FormatBool(got == "1")
					}
					if got != state.Primary.Attributes[attr] {
						return fmt.Errorf("raw API %s.%s differs", kind, key)
					}
				}
			}
		}
		return nil
	}
}
