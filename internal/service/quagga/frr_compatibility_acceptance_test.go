// Copyright (c) Matthew Mellor
// SPDX-License-Identifier: MPL-2.0

package quagga_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/matthew-on-git/terraform-provider-opnsense/internal/acctest"
)

var (
	frrObservationCounter atomic.Uint64
	frrCollectorSHA       atomic.Value
)

const frrOracleBase = "http://host.docker.internal:18445"

type frrObservation struct {
	Identity             struct{ Name, UUID string } `json:"identity"`
	Core, Plugin, Daemon frrCommand
	Config               frrCommand `json:"frr_config"`
	PF                   frrCommand `json:"pf_rules"`
	AllPF                frrCommand `json:"pf_all_rules"`
	Rendered             frrCommand `json:"pf_rendered"`
	Routes               frrCommand `json:"routes"`
	Summary              frrCommand `json:"bgp_summary"`
	Advertised4          frrCommand `json:"advertised_ipv4"`
	Advertised6          frrCommand `json:"advertised_ipv6"`
	Collector            struct {
		Active          bool     `json:"active"`
		Prefixes        []string `json:"prefixes"`
		EverPrefixes    []string `json:"ever_prefixes"`
		DefaultObserved bool     `json:"default_observed"`
		Sessions        int      `json:"sessions"`
	} `json:"collector"`
	CollectorSHA string `json:"collector_sha256"`
}
type frrCommand struct {
	Status         int
	Stdout, Stderr string
}

func frrObserve(t *testing.T) frrObservation {
	t.Helper()
	var result frrObservation
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, frrOracleBase+"/snapshot", nil)
	if err != nil {
		t.Fatal("invalid observation URI")
	}
	req.Header.Set("Authorization", "Bearer "+os.Getenv("OPNSENSE_FRR_ORACLE_TOKEN"))
	client := &http.Client{Timeout: 30 * time.Second}
	response, err := client.Do(req)
	if err != nil {
		t.Fatal("independent observation transport failed")
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != 200 || json.NewDecoder(response.Body).Decode(&result) != nil {
		t.Fatal("independent observation failed")
	}
	if result.Identity.Name != "opnsense-acceptance-102" || result.Identity.UUID != "13921598-695f-42cc-9d09-1b06dd2d38b5" {
		t.Fatal("observation VM identity mismatch")
	}
	if result.Core.Status != 0 || !strings.Contains(result.Core.Stdout, "OPNsense 26.7.7") || result.Plugin.Status != 0 || strings.TrimSpace(result.Plugin.Stdout) != "2.0_1" || result.Daemon.Status != 0 || !strings.Contains(result.Daemon.Stdout, "bgpd version 10.7.1") {
		t.Fatal("independent core/plugin/daemon version mismatch")
	}
	if initial := frrCollectorSHA.Load(); initial == nil {
		frrCollectorSHA.Store(result.CollectorSHA)
	} else if initial.(string) != result.CollectorSHA {
		t.Fatal("independent collector source changed during proof")
	}
	if len(result.CollectorSHA) != 64 || result.Config.Status != 0 || result.PF.Status != 0 || result.AllPF.Status != 0 || result.Rendered.Status != 0 || result.Routes.Status != 0 {
		t.Fatal("independent generated/config/PF/routing observation incomplete")
	}
	if dir := os.Getenv("OPNSENSE_FRR_ARTIFACT_DIR"); dir != "" {
		root, err := os.OpenRoot(dir)
		if err != nil {
			t.Fatal("cannot retain independent observations")
		}
		defer func() { _ = root.Close() }()
		encoded, err := json.Marshal(result)
		if err != nil {
			t.Fatal("cannot encode independent observations")
		}
		if root.WriteFile("observation-"+strconv.FormatUint(frrObservationCounter.Add(1), 10)+".json", encoded, 0o600) != nil {
			t.Fatal("cannot retain independent observations")
		}
	}
	return result
}

func frrLabOperation(t *testing.T, operation string) {
	t.Helper()
	uri := frrOracleBase + "/filter-reload"
	if operation != "filter-reload" {
		t.Fatal("unsupported lab operation")
	}
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, uri, nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+os.Getenv("OPNSENSE_FRR_ORACLE_TOKEN"))
	response, err := (&http.Client{Timeout: 30 * time.Second}).Do(req)
	if err != nil {
		t.Fatal("explicit lab filter activation failed")
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != 200 {
		t.Fatal("explicit lab filter activation rejected")
	}
	t.Logf("FRR_LAB_OPERATION operation=%s source=explicit_lab_not_provider", operation)
}

func frrLabFilterReload(t *testing.T) {
	t.Helper()
	frrLabOperation(t, "filter-reload")
	t.Log("FRR_FIREWALL_ACTIVATION source=explicit_lab_filter_reload provider_reconfigure=FRR_only")
}

func frrLabDisabledConfigSync(t *testing.T) {
	t.Helper()
	for _, kind := range []string{"general", "bgp"} {
		if string(frrRawSettings(t, kind)["enabled"]) != `"0"` {
			t.Fatal("lab disabled-config sync refuses enabled settings")
		}
	}
	root, err := os.OpenRoot(os.Getenv("OPNSENSE_FRR_ARTIFACT_DIR"))
	if err != nil {
		t.Fatal("cannot open private host handshake directory")
	}
	defer func() { _ = root.Close() }()
	if root.WriteFile("disabled-config-sync.request", []byte("13921598-695f-42cc-9d09-1b06dd2d38b5"), 0o600) != nil {
		t.Fatal("cannot request host-side disabled config sync")
	}
	deadline := time.Now().Add(30 * time.Second)
	for {
		raw, err := root.ReadFile("disabled-config-sync.done")
		if err == nil {
			if string(raw) != "ok" {
				t.Fatal("host-side disabled config sync failed")
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("host-side disabled config sync timed out")
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Log("FRR_LAB_OPERATION operation=disabled-config-sync source=explicit_host_probe_not_provider")
}

func frrRaw(t *testing.T, method, path string, body any) map[string]json.RawMessage {
	t.Helper()
	var payload bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&payload).Encode(body); err != nil {
			t.Fatal(err)
		}
	}
	client := acctest.TestClient(t)
	req, err := http.NewRequestWithContext(context.Background(), method, client.BaseURL()+path, &payload)
	if err != nil {
		t.Fatal(err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	response, err := client.HTTPClient().Do(req)
	if err != nil {
		t.Fatal("raw API oracle transport failed")
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != 200 {
		t.Fatalf("raw API oracle %s HTTP %d", path, response.StatusCode)
	}
	var result map[string]json.RawMessage
	if json.NewDecoder(response.Body).Decode(&result) != nil {
		t.Fatalf("raw API oracle %s malformed response", path)
	}
	return result
}

func frrRawSettings(t *testing.T, kind string) map[string]json.RawMessage {
	t.Helper()
	outer := frrRaw(t, "GET", "/api/quagga/"+kind+"/get", nil)
	var result map[string]json.RawMessage
	if json.Unmarshal(outer[kind], &result) != nil || len(result) == 0 {
		t.Fatalf("raw %s settings absent", kind)
	}
	return result
}

func frrRawSelection(raw json.RawMessage) (string, error) {
	var scalar *string
	if json.Unmarshal(raw, &scalar) == nil {
		if scalar == nil {
			return "", fmt.Errorf("null raw selector")
		}
		return *scalar, nil
	}
	if string(raw) == "[]" {
		return "", nil
	}
	var options map[string]struct {
		Selected json.RawMessage `json:"selected"`
	}
	if json.Unmarshal(raw, &options) != nil || options == nil {
		return "", fmt.Errorf("malformed raw selector")
	}
	values := []string{}
	for id, entry := range options {
		flag := strings.Trim(string(entry.Selected), "\"")
		if flag != "0" && flag != "1" {
			return "", fmt.Errorf("malformed raw selected flag")
		}
		if flag == "1" && id != "" {
			values = append(values, id)
		}
	}
	sort.Strings(values)
	return strings.Join(values, ","), nil
}

func frrRestoreSettings(t *testing.T, kind string, snapshot map[string]json.RawMessage) {
	t.Helper()
	payload := map[string]string{}
	for key, raw := range snapshot {
		if strings.HasPrefix(key, "neighbors") || key == "aspaths" || key == "prefixlists" || key == "communitylists" || key == "routemaps" || key == "peergroups" || key == "redistributions" {
			continue
		}
		scalar, err := frrRawSelection(raw)
		if err != nil {
			t.Fatalf("cannot restore raw %s field %s", kind, key)
		}
		payload[key] = scalar
	}
	result := frrRaw(t, "POST", "/api/quagga/"+kind+"/set", map[string]any{kind: payload})
	if string(result["result"]) != `"saved"` {
		t.Fatalf("restoration save for %s rejected", kind)
	}
}

func frrTagged(t *testing.T, kind, marker string) []string {
	t.Helper()
	var ids []string
	total := -1
	for page := 1; total < 0 || len(ids) < total; page++ {
		body := frrRaw(t, "POST", "/api/quagga/bgp/search_"+kind, map[string]any{"current": page, "rowCount": 500, "searchPhrase": marker})
		var rows []struct {
			UUID string `json:"uuid"`
		}
		var count int
		if json.Unmarshal(body["rows"], &rows) != nil || json.Unmarshal(body["total"], &count) != nil || count < 0 {
			t.Fatal("raw tagged search malformed")
		}
		if total >= 0 && count != total {
			t.Fatal("raw tagged search changed during pagination")
		}
		total = count
		if len(rows) == 0 && len(ids) < total {
			t.Fatal("raw tagged search incomplete")
		}
		for _, row := range rows {
			if row.UUID == "" {
				t.Fatal("raw tagged search missing identity")
			}
			ids = append(ids, row.UUID)
		}
	}
	return ids
}

// Separate standard testing cleanups keep later phases running after Fatal/Goexit.
func frrRegisterCleanup(t interface{ Cleanup(func()) }, phases ...func()) {
	for i := len(phases) - 1; i >= 0; i-- {
		t.Cleanup(phases[i])
	}
}

func frrAssertNamespaceZero(t *testing.T, kind, phase string) {
	t.Helper()
	ids := frrTagged(t, kind, "")
	if len(ids) != 0 {
		t.Errorf("%s %s has %d global leftovers before rollback", phase, kind, len(ids))
	}
	t.Logf("FRR_%s namespace=%s total=%d", phase, kind, len(ids))
}

func frrAssertZeros(t *testing.T, _, phase string) {
	t.Helper()
	for _, kind := range []string{"neighbor", "peergroup", "routemap", "prefixlist", "redistribution", "aspath", "communitylist"} {
		frrAssertNamespaceZero(t, kind, phase)
	}
}

func frrPreflight(t *testing.T) {
	t.Helper()
	acctest.PreCheck(t)
	if os.Getenv("OPNSENSE_URI") != "https://host.docker.internal:18444" {
		t.Fatal("FRR acceptance requires the exact isolated container forwarding endpoint")
	}
	if os.Getenv("OPNSENSE_FRR_ORACLE_URI") != frrOracleBase+"/snapshot" {
		t.Fatal("requires the fixed private host observation oracle")
	}
	if os.Getenv("OPNSENSE_DISPOSABLE_VM_NAME") != "opnsense-acceptance-102" || os.Getenv("OPNSENSE_DISPOSABLE_VM_UUID") != "13921598-695f-42cc-9d09-1b06dd2d38b5" || os.Getenv("OPNSENSE_FRR_IDENTITY_ATTESTATION") != "opnsense-acceptance-102:13921598-695f-42cc-9d09-1b06dd2d38b5" {
		t.Fatal("requires exact disposable identity")
	}
	endpointCheck := exec.Command("python3", "-c", `
import hashlib,re,socket,ssl,sys,urllib.parse
uri=urllib.parse.urlsplit(sys.stdin.readline().strip()); expected=sys.stdin.readline().strip()
if uri.scheme!="https" or not uri.hostname or not re.fullmatch(r"[0-9a-f]{64}",expected): raise SystemExit(1)
ctx=ssl.SSLContext(ssl.PROTOCOL_TLS_CLIENT);ctx.check_hostname=False;ctx.verify_mode=ssl.CERT_NONE
with socket.create_connection((uri.hostname,uri.port or 443),timeout=15) as sock:
 with ctx.wrap_socket(sock,server_hostname=uri.hostname) as tls:
  if hashlib.sha256(tls.getpeercert(binary_form=True)).hexdigest()!=expected: raise SystemExit(1)
`)
	endpointCheck.Stdin = strings.NewReader(os.Getenv("OPNSENSE_URI") + "\n" + os.Getenv("OPNSENSE_FRR_TLS_SHA256") + "\n")
	if endpointCheck.Run() != nil {
		t.Fatal("exact container API endpoint TLS attestation failed")
	}
	if os.Getenv("OPNSENSE_FRR_IPV6_READY_ATTESTATION") != "13921598-695f-42cc-9d09-1b06dd2d38b5:vtnet0:2001:db8:2::15:linklocal:ready" {
		t.Fatal("requires independently attested isolated WAN IPv6 next-hop readiness")
	}
	t.Log("FRR_LAB_IPV6_READY interface=vtnet0 global=2001:db8:2::15 linklocal=true provisioning=parent_lab_not_provider")
	observed := frrObserve(t)
	if observed.Collector.DefaultObserved {
		t.Fatal("collector already observed default before fixture")
	}
	t.Logf("FRR_IDENTITY core=26.7.7 plugin=2.0_1 daemon=10.7.1 collector_sha256=%s vm_uuid=%s", observed.CollectorSHA, observed.Identity.UUID)
}

func TestAccFRRCompatibility(t *testing.T) {
	if os.Getenv("OPNSENSE_FRR_COMPATIBILITY") != "1" {
		t.Skip("FRR compatibility requires the digest-bound isolated runner")
	}
	frrPreflight(t)
	marker := "tf31h_" + fmt.Sprint(time.Now().UnixNano())
	general, bgp := frrRawSettings(t, "general"), frrRawSettings(t, "bgp")
	if string(general["enabled"]) != `"0"` || string(bgp["enabled"]) != `"0"` {
		t.Fatal("acceptance requires disabled baseline FRR and BGP")
	}
	baseline := frrObserve(t)
	if strings.Contains(baseline.Config.Stdout, "router bgp") {
		t.Fatal("requires parent-restored canonical disabled generated-config baseline")
	}
	if baseline.Collector.Active || len(baseline.Collector.Prefixes) > 0 {
		t.Fatal("collector baseline must have no active session or current prefixes")
	}
	workingDir, err := os.MkdirTemp(os.Getenv("OPNSENSE_FRR_ARTIFACT_DIR"), "terraform-core-")
	if err != nil {
		t.Fatal(err)
	}
	kinds := []string{"neighbor", "peergroup", "redistribution", "routemap", "prefixlist", "aspath", "communitylist"}
	for _, kind := range kinds {
		if len(frrTagged(t, kind, "")) != 0 {
			t.Fatal("isolated fixture requires every BGP object namespace initially empty")
		}
	}
	phases := []func(){func() { frrAssertZeros(t, marker, "BEFORE_ROLLBACK") }}
	for _, kind := range kinds {
		phases = append(phases, func() {
			for _, id := range frrTagged(t, kind, marker) {
				result := frrRaw(t, "POST", "/api/quagga/bgp/del_"+kind+"/"+id, nil)
				if string(result["result"]) != `"deleted"` && string(result["result"]) != `"saved"` {
					t.Errorf("best-effort cleanup %s rejected", kind)
				}
			}
		})
	}
	phases = append(phases,
		func() { frrRestoreSettings(t, "bgp", bgp) },
		func() { frrRestoreSettings(t, "general", general) },
		func() {
			if string(frrRaw(t, "POST", "/api/quagga/service/reconfigure", nil)["status"]) != `"ok"` {
				t.Error("baseline FRR reload failed")
			}
		},
		func() { frrLabDisabledConfigSync(t) },
		func() { frrLabFilterReload(t) },
		func() {
			restored := frrObserve(t)
			if !reflect.DeepEqual(general, frrRawSettings(t, "general")) || !reflect.DeepEqual(bgp, frrRawSettings(t, "bgp")) || baseline.Config.Stdout != restored.Config.Stdout || baseline.PF.Stdout != restored.PF.Stdout || baseline.AllPF.Stdout != restored.AllPF.Stdout || baseline.Rendered.Stdout != restored.Rendered.Stdout || baseline.Routes.Stdout != restored.Routes.Stdout {
				t.Error("exact settings/generated config/PF/routing baseline restoration failed")
			} else {
				t.Log("FRR_RESTORE settings_equal=true config_equal=true pf_equal=true rendered_pf_equal=true routes_equal=true")
			}
		},
	)
	for _, kind := range kinds {
		phases = append(phases, func() { frrAssertNamespaceZero(t, kind, "FINAL_CLEANUP") })
	}
	phases = append(phases, func() {
		if frrObserve(t).Collector.DefaultObserved {
			t.Error("collector observed forbidden default during fixture")
		}
	})
	frrRegisterCleanup(t, phases...)

	frrLegacyCoreUpgrade(t, marker)
	config := frrCompatibilityConfig(marker, true, false)
	check := func(s *terraform.State) error { return frrLiveCheck(t, s, marker, true, baseline.Collector.Sessions) }
	steps := []resource.TestStep{{Config: config, Check: check}, {Config: config, ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}}}
	for _, name := range []string{"opnsense_quagga_prefix_list.a", "opnsense_quagga_prefix_list.b", "opnsense_quagga_route_map.a", "opnsense_quagga_route_map.b", "opnsense_quagga_bgp_peergroup.policy", "opnsense_quagga_bgp_neighbor.peer", "opnsense_quagga_bgp_redistribution.policy"} {
		steps = append(steps, resource.TestStep{ResourceName: name, ImportState: true, ImportStateVerify: true, ImportStateVerifyIgnore: []string{"linked_prefixlist_out", "linked_routemap_out", "match_prefix", "route_map"}, ImportStateCheck: frrCheckImportedPolicy(t, marker, name)})
	}
	steps = append(steps, resource.TestStep{Config: config, ExpectNonEmptyPlan: true, Check: func(s *terraform.State) error {
		owner := s.RootModule().Resources["opnsense_quagga_bgp_neighbor.peer"]
		result := frrRaw(t, "POST", "/api/quagga/bgp/set_neighbor/"+owner.Primary.ID, map[string]any{"neighbor": map[string]string{"description": marker + "_drift", "family": "ipv6"}})
		if string(result["result"]) != `"saved"` {
			return fmt.Errorf("raw drift mutation rejected")
		}
		return nil
	}}, resource.TestStep{Config: config, ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction("opnsense_quagga_bgp_neighbor.peer", plancheck.ResourceActionUpdate)}}, Check: check}, resource.TestStep{Config: frrCompatibilityConfig(marker, false, true), Check: func(s *terraform.State) error { return frrLiveCheck(t, s, marker, false, baseline.Collector.Sessions) }}, resource.TestStep{Config: config, Check: check})
	resource.Test(t, resource.TestCase{ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories, WorkingDir: workingDir, CheckDestroy: func(_ *terraform.State) error { frrAssertZeros(t, marker, "TERRAFORM_DESTROY"); return nil }, Steps: steps})
}

// Imports may canonicalize a sibling UUID within a name/number group. Verify
// every ignored reference independently through the authoritative raw API.
func frrCheckImportedPolicy(t *testing.T, marker, name string) resource.ImportStateCheckFunc {
	return func(states []*terraform.InstanceState) error {
		if len(states) != 1 || states[0].ID == "" {
			return fmt.Errorf("import did not return one identified object")
		}
		expected := map[string]string{
			"opnsense_quagga_route_map.a":               "match_prefix.#",
			"opnsense_quagga_bgp_peergroup.policy":      "linked_routemap_out.#",
			"opnsense_quagga_bgp_neighbor.peer":         "peer_group",
			"opnsense_quagga_bgp_redistribution.policy": "route_map",
		}[name]
		if expected != "" && (states[0].Attributes[expected] == "" || states[0].Attributes[expected] == "0") {
			return fmt.Errorf("import lost configured policy reference")
		}
		kind := map[string]string{"opnsense_quagga_route_map.a": "routemap", "opnsense_quagga_bgp_peergroup.policy": "peergroup", "opnsense_quagga_bgp_neighbor.peer": "neighbor", "opnsense_quagga_bgp_redistribution.policy": "redistribution"}[name]
		if kind != "" {
			raw := frrRaw(t, "GET", "/api/quagga/bgp/get_"+kind+"/"+states[0].ID, nil)
			var owner map[string]json.RawMessage
			if json.Unmarshal(raw[kind], &owner) != nil {
				return fmt.Errorf("canonical owner read failed")
			}
			if err := frrCanonicalReferences(states[0].Attributes, owner); err != nil {
				return err
			}
		}
		for key, id := range states[0].Attributes {
			kind, group := "", ""
			switch {
			case strings.HasPrefix(key, "match_prefix.") && key != "match_prefix.#", strings.HasPrefix(key, "linked_prefixlist_out.") && key != "linked_prefixlist_out.#":
				kind, group = "prefixlist", marker+"_pl"
			case strings.HasPrefix(key, "linked_routemap_out.") && key != "linked_routemap_out.#", key == "route_map":
				kind, group = "routemap", marker+"_rm"
			}
			if kind == "" || id == "" {
				continue
			}
			raw := frrRaw(t, "GET", "/api/quagga/bgp/get_"+kind+"/"+id, nil)
			var object struct {
				Name string `json:"name"`
			}
			if json.Unmarshal(raw[kind], &object) != nil || object.Name != group {
				return fmt.Errorf("imported policy reference lost authoritative group identity")
			}
		}
		return nil
	}
}

func frrLiveCheck(t *testing.T, s *terraform.State, marker string, firewall bool, baselineSessions int) error {
	t.Helper()
	frrLabFilterReload(t)
	deadline := time.Now().Add(180 * time.Second)
	var observation frrObservation
	for {
		observation = frrObserve(t)
		if observation.Collector.Active && observation.Collector.Sessions > baselineSessions && strings.Contains(strings.Join(observation.Collector.Prefixes, ","), "198.18.31.0/24") {
			break
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("collector did not receive positive approved prefix")
		}
		time.Sleep(time.Second)
	}
	for _, prefix := range observation.Collector.Prefixes {
		if prefix != "198.18.31.0/24" {
			return fmt.Errorf("collector observed a non-allowlisted prefix")
		}
	}
	if observation.Collector.DefaultObserved {
		return fmt.Errorf("collector observed forbidden default")
	}
	config := observation.Config.Stdout
	for _, fragment := range []string{"address-family ipv4 unicast", "address-family ipv6 unicast", "route-map " + marker + "_rm permit 10", "route-map " + marker + "_rm deny 20", "neighbor " + marker + "_pg next-hop-self", "neighbor " + marker + "_pg route-map " + marker + "_rm out"} {
		if !strings.Contains(config, fragment) {
			return fmt.Errorf("independent generated configuration missing %s", fragment)
		}
	}
	if err := frrAFPolicyProof(config, marker, !firewall); err != nil {
		return err
	}
	if !strings.Contains(config, "neighbor 10.0.2.2 peer-group "+marker+"_pg") || strings.Contains(config, "neighbor 10.0.2.2 route-map ") {
		return fmt.Errorf("active peer does not exclusively inherit group policy")
	}
	if err := frrBGPFirewallProof(observation.AllPF.Stdout, observation.Rendered.Stdout, firewall); err != nil {
		return err
	}
	if strings.Contains(config, "neighbor 10.0.2.2 default-originate") {
		return fmt.Errorf("active collector peer unexpectedly originates default")
	}
	// Data sources may return canonical sibling UUIDs; all other state must
	// agree with the managed owner and both reference forms must resolve to the
	// configured authoritative group, not merely be nonempty.
	for _, name := range []string{"opnsense_quagga_bgp_neighbor.peer", "opnsense_quagga_bgp_peergroup.policy", "opnsense_quagga_route_map.a"} {
		managed, lookup := s.RootModule().Resources[name], s.RootModule().Resources["data."+name]
		if managed == nil || lookup == nil {
			return fmt.Errorf("resource/data-source fixture missing")
		}
		for key, value := range managed.Primary.Attributes {
			if strings.HasPrefix(key, "linked_routemap_out.") || strings.HasPrefix(key, "match_prefix.") {
				continue
			}
			if lookup.Primary.Attributes[key] != value {
				return fmt.Errorf("resource/data-source disagree on %s", key)
			}
		}
		if err := frrCheckImportedPolicy(t, marker, name)([]*terraform.InstanceState{lookup.Primary}); err != nil {
			return err
		}
	}
	owner := s.RootModule().Resources["opnsense_quagga_bgp_neighbor.peer"]
	raw := frrRaw(t, "GET", "/api/quagga/bgp/get_neighbor/"+owner.Primary.ID, nil)
	var neighbor map[string]json.RawMessage
	if json.Unmarshal(raw["neighbor"], &neighbor) != nil {
		return fmt.Errorf("raw neighbor read missing")
	}
	membership, err := frrRawSelection(neighbor["peergroup"])
	if err != nil {
		return err
	}
	group := s.RootModule().Resources["opnsense_quagga_bgp_peergroup.policy"]
	if membership == "" || membership != group.Primary.ID || owner.Primary.Attributes["peer_group"] != membership {
		return fmt.Errorf("raw active peer-group membership differs")
	}
	var description string
	_ = json.Unmarshal(neighbor["description"], &description)
	if description != marker {
		return fmt.Errorf("drift repair did not restore description")
	}
	for _, name := range []string{"v4", "v6", "dual", "cross"} {
		owner := s.RootModule().Resources["opnsense_quagga_bgp_neighbor."+name]
		if owner == nil {
			return fmt.Errorf("legacy/explicit neighbor fixture missing")
		}
		expected := map[string]string{"v4": "ipv4", "v6": "ipv6", "dual": "ipv4,ipv6", "cross": "ipv6"}[name]
		family := []string{}
		for k, v := range owner.Primary.Attributes {
			if strings.HasPrefix(k, "family.") && k != "family.#" {
				family = append(family, v)
			}
		}
		sort.Strings(family)
		fixture := frrRaw(t, "GET", "/api/quagga/bgp/get_neighbor/"+owner.Primary.ID, nil)
		var rawFixture map[string]json.RawMessage
		if json.Unmarshal(fixture["neighbor"], &rawFixture) != nil {
			return fmt.Errorf("raw disabled family fixture missing")
		}
		selected, err := frrRawSelection(rawFixture["family"])
		if err != nil {
			return err
		}
		selectedFamilies := strings.Split(selected, ",")
		sort.Strings(selectedFamilies)
		if strings.Join(selectedFamilies, ",") != expected || string(rawFixture["enabled"]) != `"0"` {
			return fmt.Errorf("raw disabled family fixture differs for %s", name)
		}
		if strings.Join(family, ",") != expected {
			return fmt.Errorf("legacy or explicit family state wrong for %s", name)
		}
	}
	t.Logf("FRR_EFFECTIVE_CONFIG policy_order=true peer_group_per_af=true positive_prefix=198.18.31.0/24 default_export=false firewall_rules=%t activation=explicit_lab_filter_reload", firewall)
	return nil
}

func frrCompatibilityConfig(marker string, firewall, defaultOriginate bool) string {
	return fmt.Sprintf(`
resource "opnsense_quagga_general" "lab" {
 enabled = true
 firewall_rules = %[2]t
 depends_on = [opnsense_quagga_bgp_global.lab]
}
resource "opnsense_quagga_bgp_global" "lab" {
 enabled = true
 as_number = 65111
 router_id = "198.18.31.1"
 networks = ["198.18.31.0/24", "0.0.0.0/0"]
}
resource "opnsense_quagga_prefix_list" "a" {
 name = "%[1]s_pl"
 version = "IPv4"
 sequence = 10
 action = "permit"
 network = "198.18.31.0/24"
}
resource "opnsense_quagga_prefix_list" "b" {
 name = "%[1]s_pl"
 version = "IPv4"
 sequence = 20
 action = "deny"
 network = "0.0.0.0/0 le 32"
 depends_on = [opnsense_quagga_prefix_list.a]
}
resource "opnsense_quagga_route_map" "a" {
 name = "%[1]s_rm"
 action = "permit"
 order = 10
 match_prefix = [opnsense_quagga_prefix_list.b.id]
}
resource "opnsense_quagga_route_map" "b" {
 name = "%[1]s_rm"
 action = "deny"
 order = 20
 depends_on = [opnsense_quagga_route_map.a]
}
resource "opnsense_quagga_bgp_redistribution" "policy" {
 enabled = false
 description = "%[1]s_redistribution"
 redistribute = "connected"
 route_map = opnsense_quagga_route_map.b.id
}
resource "opnsense_quagga_bgp_peergroup" "policy" {
 name = "%[1]s_pg"
 remote_as = 65222
 family = ["ipv4","ipv6"]
 next_hop_self = true
 default_originate = false
 linked_routemap_out = [opnsense_quagga_route_map.b.id]
}
resource "opnsense_quagga_bgp_peergroup" "unused" {
 name = "%[1]s_unused"
 remote_as = 65222
 family = ["ipv4","ipv6"]
 default_originate = %[3]t
}
resource "opnsense_quagga_bgp_neighbor" "peer" {
 address = "10.0.2.2"
 remote_as = 65222
 description = "%[1]s"
 family = ["ipv4","ipv6"]
 peer_group = opnsense_quagga_bgp_peergroup.policy.id
 depends_on = [opnsense_quagga_general.lab]
}
resource "opnsense_quagga_bgp_neighbor" "v4" {
 enabled=false
 address="192.0.2.31"
 remote_as=65222
 multi_protocol=false
 description="%[1]s_v4"
}
resource "opnsense_quagga_bgp_neighbor" "v6" {
 enabled=false
 address="2001:db8::31"
 remote_as=65222
 multi_protocol=false
 description="%[1]s_v6"
}
resource "opnsense_quagga_bgp_neighbor" "dual" {
 enabled=false
 address="192.0.2.32"
 remote_as=65222
 multi_protocol=true
 description="%[1]s_dual"
}
resource "opnsense_quagga_bgp_neighbor" "cross" {
 enabled=false
 address="192.0.2.33"
 remote_as=65222
 family=["ipv6"]
 description="%[1]s_cross"
}
data "opnsense_quagga_bgp_neighbor" "peer" {id=opnsense_quagga_bgp_neighbor.peer.id}
data "opnsense_quagga_bgp_peergroup" "policy" {id=opnsense_quagga_bgp_peergroup.policy.id}
data "opnsense_quagga_route_map" "a" {id=opnsense_quagga_route_map.a.id}
`, marker, firewall, defaultOriginate)
}

// Real Terraform Core obtains version-0 state through the unmodified baseline
// provider's supported import, then upgrades with the frozen candidate. No
// manual state editing or replacement of a user's resource address is involved.
func frrLegacyCoreUpgrade(t *testing.T, marker string) {
	t.Helper()
	dir, err := os.MkdirTemp(os.Getenv("OPNSENSE_FRR_ARTIFACT_DIR"), "core-v0-")
	if err != nil {
		t.Fatal(err)
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = root.Close() }()
	config := fmt.Sprintf(`terraform {
 required_providers { opnsense = { source = "matthew-on-git/opnsense" } }
}
provider "opnsense" {}
resource "opnsense_quagga_bgp_neighbor" "legacy" {
 enabled = false
 address = "2001:db8::40"
 remote_as = 65222
 multi_protocol = false
 description = %q
}
`, marker+"_core_v0")
	if root.WriteFile("main.tf", []byte(config), 0o600) != nil {
		t.Fatal("cannot write Core migration fixture")
	}
	artifactDir := os.Getenv("OPNSENSE_FRR_ARTIFACT_DIR")
	for _, kind := range []string{"legacy", "current"} {
		cli := fmt.Sprintf("provider_installation {\n dev_overrides {\n  \"registry.terraform.io/matthew-on-git/opnsense\" = %q\n }\n direct {}\n}\n", artifactDir+"/bin/"+kind)
		if root.WriteFile(kind+".tfrc", []byte(cli), 0o600) != nil {
			t.Fatal("cannot write Core provider override")
		}
	}
	var id string
	run := func(kind, action string) {
		t.Helper()
		var cmd *exec.Cmd
		switch action {
		case "import":
			cmd = exec.Command("terraform", "import", "-no-color", "opnsense_quagga_bgp_neighbor.legacy")
			cmd.Args = append(cmd.Args, id)
		case "refresh":
			cmd = exec.Command("terraform", "apply", "-refresh-only", "-auto-approve", "-no-color")
		case "saved-plan":
			cmd = exec.Command("terraform", "plan", "-detailed-exitcode", "-out=candidate.tfplan", "-no-color")
		case "apply":
			cmd = exec.Command("terraform", "apply", "-auto-approve", "-no-color", "candidate.tfplan")
		case "plan":
			cmd = exec.Command("terraform", "plan", "-detailed-exitcode", "-no-color")
		case "destroy":
			cmd = exec.Command("terraform", "destroy", "-auto-approve", "-no-color")
		default:
			t.Fatal("unsupported Core fixture command")
		}
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "TF_CLI_CONFIG_FILE="+dir+"/"+kind+".tfrc", "TF_IN_AUTOMATION=1")
		log, err := root.OpenFile("core-migration.log", os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o600)
		if err != nil {
			t.Fatal("cannot retain Core log")
		}
		cmd.Stdout = log
		cmd.Stderr = log
		runErr := cmd.Run()
		_ = log.Close()
		if runErr != nil {
			t.Fatalf("Terraform Core %s %s failed; private scanned log retained", kind, action)
		}
	}
	added := frrRaw(t, "POST", "/api/quagga/bgp/add_neighbor", map[string]any{"neighbor": map[string]string{"enabled": "0", "address": "2001:db8::40", "remoteas": "65222", "family": "ipv6", "description": marker + "_core_v0"}})
	if string(added["result"]) != `"saved"` || json.Unmarshal(added["uuid"], &id) != nil || !regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`).MatchString(id) {
		t.Fatal("legacy import fixture create did not affirm identity")
	}
	run("legacy", "import")
	type instance struct {
		SchemaVersion int                        `json:"schema_version"`
		Attributes    map[string]json.RawMessage `json:"attributes"`
	}
	type state struct {
		Resources []struct {
			Type, Name string
			Instances  []instance
		} `json:"resources"`
	}
	readState := func() instance {
		raw, err := root.ReadFile("terraform.tfstate")
		if err != nil {
			t.Fatal("Core state unavailable")
		}
		var decoded state
		if json.Unmarshal(raw, &decoded) != nil {
			t.Fatal("Core state malformed")
		}
		for _, resource := range decoded.Resources {
			if resource.Type == "opnsense_quagga_bgp_neighbor" && resource.Name == "legacy" && len(resource.Instances) == 1 {
				return resource.Instances[0]
			}
		}
		t.Fatal("Core migration resource address or identity lost")
		return instance{}
	}
	before := readState()
	if before.SchemaVersion != 0 || string(before.Attributes["id"]) != fmt.Sprintf("%q", id) {
		t.Fatal("baseline provider did not persist actual version-0 identity")
	}
	run("current", "refresh")
	after := readState()
	var upgradedFamilies []string
	familyErr := json.Unmarshal(after.Attributes["family"], &upgradedFamilies)
	if after.SchemaVersion != 1 || string(after.Attributes["id"]) != fmt.Sprintf("%q", id) || (familyErr != nil || len(upgradedFamilies) != 1 || upgradedFamilies[0] != "ipv6") {
		t.Fatal("real Core migration did not preserve IPv6 intent and UUID")
	}
	run("current", "saved-plan")
	run("current", "apply")
	run("current", "plan")
	run("current", "destroy")
	t.Log("FRR_CORE_V0_UPGRADE actual_legacy_import=true offline_upgrade=true uuid_preserved=true ipv6_only=true no_state_edit=true no_change_plan=true terraform_delete=true")
}

func frrBGPFirewallProof(rules, rendered string, enabled bool) error {
	in4, out4, in6, out6 := false, false, false, false
	count := 0
	for _, line := range strings.Split(rules, "\n") {
		if !strings.Contains(line, "proto tcp") || (!regexp.MustCompile(`\bport = (bgp|179)(\s|$)`).MatchString(line)) {
			continue
		}
		count++
		ipv4, ipv6 := strings.Contains(line, " inet "), strings.Contains(line, " inet6 ")
		if !ipv4 && !ipv6 {
			ipv4 = true
			ipv6 = true
		}
		if strings.Contains(line, " on ") {
			continue
		}
		toSelf := strings.Contains(line, "from any to (self) port = ") || strings.Contains(line, "from any to self port = ")
		fromSelf := strings.Contains(line, "from (self) to any port = ") || strings.Contains(line, "from self to any port = ")
		if strings.HasPrefix(line, "pass in ") && toSelf {
			in4 = in4 || ipv4
			in6 = in6 || ipv6
		}
		if strings.HasPrefix(line, "pass out ") && fromSelf {
			out4 = out4 || ipv4
			out6 = out6 || ipv6
		}
	}
	if !enabled {
		if count != 0 || strings.Contains(rendered, "# Pass BGP") {
			return fmt.Errorf("effective/rendered BGP firewall opt-out failed")
		}
		return nil
	}
	broadIn4, broadOut4, broadIn6, broadOut6 := false, false, false, false
	for _, line := range strings.Split(rendered, "\n") {
		if !strings.Contains(line, "# Pass BGP") || !strings.Contains(line, "proto tcp") || !strings.Contains(line, "179") {
			continue
		}
		if strings.Contains(line, " on ") {
			continue
		}
		// The core renderer may expand source inet46 into separate inet/inet6
		// rules. Require the complete union in both directions, not one syntax.
		ipv4, ipv6 := !strings.Contains(line, " inet6 "), !strings.Contains(line, " inet ")
		if strings.HasPrefix(line, "pass in ") && strings.Contains(line, "from {any} to {(self)} port {179}") {
			broadIn4 = broadIn4 || ipv4
			broadIn6 = broadIn6 || ipv6
		}
		if strings.HasPrefix(line, "pass out ") && strings.Contains(line, "from {(self)} to {any} port {179}") {
			broadOut4 = broadOut4 || ipv4
			broadOut6 = broadOut6 || ipv6
		}
	}
	if !in4 || !out4 || !in6 || !out6 || !broadIn4 || !broadOut4 || !broadIn6 || !broadOut6 {
		return fmt.Errorf("independent generated/effective inet46 TCP179 to/from-self firewall proof incomplete")
	}
	return nil
}

func frrCanonicalReferences(attrs map[string]string, raw map[string]json.RawMessage) error {
	for field, wire := range map[string]string{"linked_routemap_out": "linkedRoutemapOut", "match_prefix": "match2", "route_map": "linkedRoutemap", "peer_group": "peergroup"} {
		if _, present := attrs[field]; !present {
			if _, present = attrs[field+".#"]; !present {
				continue
			}
		}
		selected, err := frrRawSelection(raw[wire])
		if err != nil {
			return err
		}
		want := []string{}
		if value, scalar := attrs[field]; scalar {
			if value != "" {
				want = append(want, value)
			}
		} else {
			for key, value := range attrs {
				if strings.HasPrefix(key, field+".") && key != field+".#" {
					want = append(want, value)
				}
			}
		}
		got := []string{}
		if selected != "" {
			got = strings.Split(selected, ",")
		}
		sort.Strings(got)
		sort.Strings(want)
		if !reflect.DeepEqual(want, got) {
			return fmt.Errorf("fresh import/data-source reference %s is not raw canonical selection", field)
		}
	}
	return nil
}

func frrAFPolicyProof(config, marker string, unusedDefault bool) error {
	blocks := map[string][]string{}
	af := ""
	for _, line := range strings.Split(config, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "address-family ") {
			af = strings.TrimPrefix(line, "address-family ")
			continue
		}
		if line == "exit-address-family" {
			af = ""
			continue
		}
		if line == "neighbor "+marker+"_pg default-originate" {
			return fmt.Errorf("active group may not originate default")
		}
		if af == "" && (line == "neighbor "+marker+"_pg next-hop-self" || strings.HasPrefix(line, "neighbor "+marker+"_pg route-map ") || line == "neighbor "+marker+"_unused default-originate") {
			return fmt.Errorf("group AF policy outside address-family block")
		}
		if af != "" {
			blocks[af] = append(blocks[af], line)
		}
	}
	for _, family := range []string{"ipv4 unicast", "ipv6 unicast"} {
		lines := blocks[family]
		for _, expected := range []string{"neighbor " + marker + "_pg next-hop-self", "neighbor " + marker + "_pg route-map " + marker + "_rm out"} {
			count := 0
			for _, line := range lines {
				if line == expected {
					count++
				}
			}
			if count != 1 {
				return fmt.Errorf("group policy missing or duplicated in %s", family)
			}
		}
		active, unused := 0, 0
		for _, line := range lines {
			if line == "neighbor "+marker+"_pg default-originate" {
				active++
			}
			if line == "neighbor "+marker+"_unused default-originate" {
				unused++
			}
		}
		if active != 0 || unused != map[bool]int{true: 1, false: 0}[unusedDefault] {
			return fmt.Errorf("per-AF default-originate mismatch in %s", family)
		}
	}
	return nil
}
