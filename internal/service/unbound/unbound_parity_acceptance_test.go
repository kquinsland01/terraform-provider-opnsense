// Copyright (c) Matthew Mellor
// SPDX-License-Identifier: MPL-2.0

package unbound_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"

	"github.com/matthew-on-git/terraform-provider-opnsense/internal/acctest"
	"github.com/matthew-on-git/terraform-provider-opnsense/pkg/opnsense"
)

type unboundParitySnapshot struct {
	canonical         json.RawMessage
	dnsblUUID         string
	restored          bool
	cleanupRegistered bool
}

var generalParityFields = []string{
	"enabled", "port", "stats", "active_interface", "dnssec", "dns64", "dns64_prefix", "no_aaaa_records",
	"register_dhcp", "register_dhcp_domain", "register_dhcp_static", "no_register_link_local6", "no_register_records",
	"txt_support", "cache_flush", "local_zone_type", "outgoing_interface", "enable_wpad", "hide_identity", "hide_version",
	"enable_prefetch_key", "dnssec_stripped", "aggressive_nsec", "qname_min_strict", "outgoing_num_tcp", "incoming_num_tcp",
	"num_queries_per_thread", "outgoing_range", "jostle_timeout", "discard_timeout", "private_domains", "private_addresses",
	"insecure_domains", "enable_prefetch", "unwanted_reply_threshold", "msg_cache_size", "rrset_cache_size", "max_ttl",
	"max_negative_ttl", "min_ttl", "host_ttl", "keep_probing_hosts", "num_hosts", "extended_statistics", "log_queries",
	"log_replies", "tag_query_reply", "log_local_actions", "log_servfail", "verbosity_level", "validation_level",
	"serve_expired_enabled", "serve_expired_record_reply_ttl", "serve_expired_ttl", "serve_expired_reset_ttl",
	"serve_expired_client_response_timeout", "acl_default_action", "forwarding_enabled",
}

var domainParityFields = []string{"id", "enabled", "domain", "type", "server", "server_port", "verify_cn", "description"}

// TestAccUnboundParity is executed only by the digest-bound disposable runner.
// It starts with an API-created DNSBL item so fixed-ID import is genuinely
// pre-existing, covers every settings field and both forward endpoint families,
// then restores only request-shaped owned fields and proves full-tree equality.
func TestAccUnboundParity(t *testing.T) {
	marker := "tf-story31d-" + strconv.FormatInt(time.Now().UnixNano(), 36)
	var snapshot unboundParitySnapshot
	var singletonBeforeDestroy json.RawMessage
	dnsblOnly := unboundDNSBLImportConfig(marker)
	singletons := unboundSingletonConfig(marker)
	query := unboundParityConfig(marker, "query", "192.0.2.53", 0, "")
	generalChanged := strings.Replace(query, `port = "53535"`, `port = "53536"`, 1)
	advancedChanged := strings.Replace(query, `hide_identity = true`, `hide_identity = false`, 1)
	aclChanged := strings.Replace(query, `acl_default_action = "refuse"`, `acl_default_action = "allow"`, 1)
	forwardingChanged := strings.Replace(query, `forwarding_enabled = true`, `forwarding_enabled = false`, 1)
	dnsblChanged := strings.Replace(query, `address = "192.0.2.31"`, `address = "192.0.2.32"`, 1)
	dot := unboundParityConfig(marker, "dot", "192.0.2.54", 0, "dns.example.test")
	dotIndependent := unboundIndependentDotConfig(marker)
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		PreCheck: func() {
			unboundParityPreCheck(t)
			client := acctest.TestClient(t)
			var err error
			snapshot.canonical, err = getUnboundSnapshot(client)
			if err != nil {
				t.Fatal(err)
			}
			if !snapshot.cleanupRegistered {
				snapshot.cleanupRegistered = true
				t.Cleanup(func() {
					if cleanupErr := restoreUnboundSnapshot(t, marker, &snapshot)(nil); cleanupErr != nil {
						t.Errorf("rollback Story 31.0d acceptance fixture: %v", cleanupErr)
					}
				})
			}
			if err = applyUnboundGeneralFixture(client); err != nil {
				t.Fatal(err)
			}
			snapshot.dnsblUUID, err = createUnboundDNSBLFixture(client, marker)
			if err != nil {
				t.Fatal(err)
			}
		},
		CheckDestroy: restoreUnboundSnapshot(t, marker, &snapshot),
		Steps: []resource.TestStep{
			dnsblPreexistingImportStep(dnsblOnly, marker),
			{Config: dnsblOnly, PlanOnly: true},
			{Config: singletons, ResourceName: "opnsense_unbound_general.parity", ImportState: true, ImportStateId: "unbound-general", ImportStatePersist: true},
			{Config: singletons, PlanOnly: true},
			{Config: query, Check: resource.ComposeAggregateTestCheckFunc(
				compareResourceDataFields("opnsense_unbound_general.parity", "data.opnsense_unbound_settings.parity", generalParityFields, nil),
				compareResourceDataFields("opnsense_unbound_domain_override.parity", "data.opnsense_unbound_domain_override.parity", domainParityFields, nil),
				compareDNSBLSettings(t, marker, "192.0.2.31", &snapshot),
				resource.TestCheckResourceAttr("opnsense_unbound_domain_override.parity", "server_port", "53"),
			)},
			// Exercise every owned singleton branch through Terraform. Returning
			// to the base configuration between mutations ensures each Update
			// changes exactly one branch and makes sibling preservation observable.
			{Config: generalChanged, Check: compareResourceDataFields("opnsense_unbound_general.parity", "data.opnsense_unbound_settings.parity", generalParityFields, nil)},
			{Config: query, Check: compareResourceDataFields("opnsense_unbound_general.parity", "data.opnsense_unbound_settings.parity", generalParityFields, nil)},
			{Config: advancedChanged, Check: compareResourceDataFields("opnsense_unbound_general.parity", "data.opnsense_unbound_settings.parity", generalParityFields, nil)},
			{Config: query, Check: compareResourceDataFields("opnsense_unbound_general.parity", "data.opnsense_unbound_settings.parity", generalParityFields, nil)},
			{Config: aclChanged, Check: compareResourceDataFields("opnsense_unbound_general.parity", "data.opnsense_unbound_settings.parity", generalParityFields, nil)},
			{Config: query, Check: compareResourceDataFields("opnsense_unbound_general.parity", "data.opnsense_unbound_settings.parity", generalParityFields, nil)},
			{Config: forwardingChanged, Check: compareResourceDataFields("opnsense_unbound_general.parity", "data.opnsense_unbound_settings.parity", generalParityFields, nil)},
			{Config: query, Check: compareResourceDataFields("opnsense_unbound_general.parity", "data.opnsense_unbound_settings.parity", generalParityFields, nil)},
			// Mutate the API-created DNSBL item through Terraform, then inject
			// out-of-band drift and prove Terraform detects and repairs it.
			{Config: dnsblChanged, Check: compareDNSBLSettings(t, marker, "192.0.2.32", &snapshot)},
			{Config: dnsblChanged, Check: driftUnboundDNSBLAddress(t, &snapshot, "192.0.2.31"), ExpectNonEmptyPlan: true},
			{Config: dnsblChanged, PlanOnly: true, ExpectNonEmptyPlan: true},
			{Config: dnsblChanged, Check: compareDNSBLSettings(t, marker, "192.0.2.32", &snapshot)},
			{Config: dnsblChanged, PlanOnly: true},
			{Config: query, Check: compareDNSBLSettings(t, marker, "192.0.2.31", &snapshot)},
			{Config: query, PlanOnly: true},
			{ResourceName: "opnsense_unbound_general.parity", ImportState: true, ImportStateId: "unbound-general", ImportStateVerify: true},
			{ResourceName: "opnsense_unbound_dnsbl.parity", ImportState: true, ImportStateId: "unbound-dnsbl", ImportStateVerify: true},
			{ResourceName: "opnsense_unbound_domain_override.parity", ImportState: true, ImportStateVerify: true},
			{Config: query, PlanOnly: true},
			{Config: query, Check: resource.ComposeAggregateTestCheckFunc(driftUnboundHideIdentity(t, false), driftUnboundForward(t, "query", "drifted")), ExpectNonEmptyPlan: true},
			{Config: query, PlanOnly: true, ExpectNonEmptyPlan: true},
			{Config: query, Check: resource.ComposeAggregateTestCheckFunc(
				compareResourceDataFields("opnsense_unbound_general.parity", "data.opnsense_unbound_settings.parity", generalParityFields, nil),
				compareResourceDataFields("opnsense_unbound_domain_override.parity", "data.opnsense_unbound_domain_override.parity", domainParityFields, nil),
			)},
			{Config: query, PlanOnly: true},
			{Config: dot, Check: resource.ComposeAggregateTestCheckFunc(
				compareResourceDataFields("opnsense_unbound_domain_override.parity", "data.opnsense_unbound_domain_override.parity", domainParityFields, nil),
				resource.TestCheckResourceAttr("opnsense_unbound_domain_override.parity", "server_port", "853"),
				resource.TestCheckResourceAttr("opnsense_unbound_domain_override.parity", "verify_cn", "dns.example.test"),
			)},
			{ResourceName: "opnsense_unbound_domain_override.parity", ImportState: true, ImportStateVerify: true},
			{Config: dot, PlanOnly: true},
			{Config: dot, Check: driftUnboundForward(t, "dot", "dot-drifted"), ExpectNonEmptyPlan: true},
			{Config: dot, PlanOnly: true, ExpectNonEmptyPlan: true},
			{Config: dot, Check: compareResourceDataFields("opnsense_unbound_domain_override.parity", "data.opnsense_unbound_domain_override.parity", domainParityFields, nil)},
			{Config: dot, PlanOnly: true},
			{Config: query, Check: resource.ComposeAggregateTestCheckFunc(
				compareResourceDataFields("opnsense_unbound_domain_override.parity", "data.opnsense_unbound_domain_override.parity", domainParityFields, nil),
				resource.TestCheckResourceAttr("opnsense_unbound_domain_override.parity", "type", "query"),
				resource.TestCheckResourceAttr("opnsense_unbound_domain_override.parity", "server_port", "53"),
				resource.TestCheckResourceAttr("opnsense_unbound_domain_override.parity", "verify_cn", ""),
				captureSingletonSnapshot(t, &singletonBeforeDestroy),
			)},
			{ResourceName: "opnsense_unbound_domain_override.parity", ImportState: true, ImportStateVerify: true},
			{Config: query, PlanOnly: true},
			// Use a distinct Terraform address after the bidirectional transition
			// coverage so DoT add/delete endpoints execute independently.
			{Config: dotIndependent, Check: resource.ComposeAggregateTestCheckFunc(
				compareResourceDataFields("opnsense_unbound_domain_override.dot_independent", "data.opnsense_unbound_domain_override.dot_independent", domainParityFields, nil),
				resource.TestCheckResourceAttr("opnsense_unbound_domain_override.dot_independent", "server_port", "853"),
				resource.TestCheckResourceAttr("opnsense_unbound_domain_override.dot_independent", "verify_cn", "independent.example.test"),
			)},
			{ResourceName: "opnsense_unbound_domain_override.dot_independent", ImportState: true, ImportStateVerify: true},
			{Config: dotIndependent, PlanOnly: true},
			{Config: dotIndependent, Check: driftUnboundForwardAt(t, "opnsense_unbound_domain_override.dot_independent", "dot", "independent-drifted"), ExpectNonEmptyPlan: true},
			{Config: dotIndependent, PlanOnly: true, ExpectNonEmptyPlan: true},
			{Config: dotIndependent, Check: compareResourceDataFields("opnsense_unbound_domain_override.dot_independent", "data.opnsense_unbound_domain_override.dot_independent", domainParityFields, nil)},
			{Config: dotIndependent, PlanOnly: true},
			{Config: "terraform {}", Check: resource.ComposeAggregateTestCheckFunc(
				checkSingletonSnapshotUnchanged(t, &singletonBeforeDestroy),
				checkTaggedForwardsAbsent(t, marker),
				restoreUnboundSnapshot(t, marker, &snapshot),
			)},
		},
	})
}

func applyUnboundGeneralFixture(client *opnsense.Client) error {
	payload := map[string]any{
		"general": map[string]any{
			"enabled": "0", "port": "53535", "stats": "1", "active_interface": "", "dnssec": "0", "dns64": "1", "dns64prefix": "64:ff9b:1::/96",
			"noarecords": "1", "regdhcp": "1", "regdhcpdomain": "story31d.example.test", "regdhcpstatic": "1", "noreglladdr6": "1", "noregrecords": "1",
			"txtsupport": "1", "cacheflush": "1", "local_zone_type": "refuse", "outgoing_interface": "", "enable_wpad": "1",
		},
		"advanced": map[string]any{
			"hideidentity": "1", "hideversion": "1", "prefetchkey": "1", "dnssecstripped": "1", "aggressivensec": "0", "qnameminstrict": "1",
			"outgoingnumtcp": "31", "incomingnumtcp": "32", "numqueriesperthread": "1025", "outgoingrange": "4097", "jostletimeout": "201", "discardtimeout": "1901",
			"privatedomain": "private.story31d.example.test", "privateaddress": "192.0.2.0/24", "insecuredomain": "insecure.story31d.example.test",
			"prefetch": "1", "unwantedreplythreshold": "10000001", "msgcachesize": "17m", "rrsetcachesize": "33m", "cachemaxttl": "86399",
			"cachemaxnegativettl": "3599", "cacheminttl": "31", "infrahostttl": "901", "infrakeepprobing": "1", "infracachenumhosts": "10001",
			"extendedstatistics": "1", "logqueries": "1", "logreplies": "1", "logtagqueryreply": "1", "loglocalactions": "1", "logservfail": "1",
			"logverbosity": "2", "valloglevel": "1", "serveexpired": "1", "serveexpiredreplyttl": "31", "serveexpiredttl": "86431",
			"serveexpiredttlreset": "1", "serveexpiredclienttimeout": "181",
		},
		"acls": map[string]any{"default_action": "refuse"}, "forwarding": map[string]any{"enabled": "1"},
	}
	return opnsense.UpdateSingleton(context.Background(), client, generalSettingsOpts(), &payload)
}

func unboundSingletonConfig(marker string) string {
	config := unboundParityConfig(marker, "query", "192.0.2.53", 0, "")
	index := strings.Index(config, `resource "opnsense_unbound_domain_override" "parity"`)
	if index < 0 {
		panic("Unbound parity fixture lacks forward resource")
	}
	return config[:index] + `
data "opnsense_unbound_settings" "parity" {
  depends_on = [opnsense_unbound_general.parity, opnsense_unbound_dnsbl.parity]
}
`
}

func captureSingletonSnapshot(t *testing.T, target *json.RawMessage) resource.TestCheckFunc {
	return func(*terraform.State) error {
		value, err := getOwnedSingletonSnapshot(acctest.TestClient(t))
		if err == nil {
			*target = value
		}
		return err
	}
}

func checkSingletonSnapshotUnchanged(t *testing.T, before *json.RawMessage) resource.TestCheckFunc {
	return func(*terraform.State) error {
		if len(*before) == 0 {
			return fmt.Errorf("singleton pre-destroy snapshot is empty")
		}
		after, err := getOwnedSingletonSnapshot(acctest.TestClient(t))
		if err != nil {
			return err
		}
		if string(after) != string(*before) {
			return fmt.Errorf("state-only singleton destroy changed authoritative settings")
		}
		t.Log("UNBOUND_SINGLETON_DESTROY unchanged=true")
		return nil
	}
}

func getOwnedSingletonSnapshot(client *opnsense.Client) (json.RawMessage, error) {
	full, err := getUnboundSnapshot(client)
	if err != nil {
		return nil, err
	}
	var tree map[string]json.RawMessage
	if err := json.Unmarshal(full, &tree); err != nil {
		return nil, err
	}
	owned := make(map[string]json.RawMessage)
	for _, branch := range []string{"general", "advanced", "forwarding", "dnsbl"} {
		owned[branch] = tree[branch]
	}
	var acls map[string]json.RawMessage
	if err := json.Unmarshal(tree["acls"], &acls); err != nil {
		return nil, err
	}
	aclDefault, err := json.Marshal(map[string]json.RawMessage{"default_action": acls["default_action"]})
	if err != nil {
		return nil, err
	}
	owned["acls"] = aclDefault
	return json.Marshal(owned)
}

func dnsblPreexistingImportStep(config, marker string) resource.TestStep {
	return resource.TestStep{
		Config:             config,
		ResourceName:       "opnsense_unbound_dnsbl.parity",
		ImportState:        true,
		ImportStateId:      "unbound-dnsbl",
		ImportStatePersist: true,
		ImportStateCheck:   checkImportedDNSBL(marker),
	}
}

func checkImportedDNSBL(marker string) resource.ImportStateCheckFunc {
	return func(states []*terraform.InstanceState) error {
		if len(states) != 1 {
			return fmt.Errorf("DNSBL import returned %d states, want 1", len(states))
		}
		state := states[0]
		if state.ID != "unbound-dnsbl" || state.Attributes["id"] != "unbound-dnsbl" {
			return fmt.Errorf("DNSBL import ID=%q attribute=%q", state.ID, state.Attributes["id"])
		}
		for field, want := range map[string]string{"enabled": "false", "safe_search": "true", "address": "192.0.2.31", "nxdomain": "true"} {
			if got := state.Attributes[field]; got != want {
				return fmt.Errorf("imported DNSBL %s=%q, want %q", field, got, want)
			}
		}
		for field, want := range map[string]string{
			"type": "ag", "lists": "https://example.invalid/" + marker,
			"whitelists": "allow." + marker + ".example.test", "blocklists": "block." + marker + ".example.test",
			"wildcards": "wild." + marker + ".example.test",
		} {
			values := collectionValues(state.Attributes, field)
			if len(values) != 1 || values[0] != want {
				return fmt.Errorf("imported DNSBL %s=%v, want [%s]", field, values, want)
			}
		}
		return nil
	}
}

func TestDNSBLPreexistingImportStepPersistsAndChecksState(t *testing.T) {
	step := dnsblPreexistingImportStep("config", "fixture")
	if !step.ImportState || !step.ImportStatePersist || step.ImportStateVerify || step.ImportStateCheck == nil {
		t.Fatalf("pre-existing import step is not persistence-safe: %#v", step)
	}
	state := &terraform.InstanceState{ID: "unbound-dnsbl", Attributes: map[string]string{
		"id": "unbound-dnsbl", "enabled": "false", "safe_search": "true", "address": "192.0.2.31", "nxdomain": "true",
		"type.#": "1", "type.1": "ag", "lists.#": "1", "lists.1": "https://example.invalid/fixture",
		"whitelists.#": "1", "whitelists.1": "allow.fixture.example.test", "blocklists.#": "1", "blocklists.1": "block.fixture.example.test",
		"wildcards.#": "1", "wildcards.1": "wild.fixture.example.test",
	}}
	if err := step.ImportStateCheck([]*terraform.InstanceState{state}); err != nil {
		t.Fatal(err)
	}
}

func TestUnboundParityAcceptanceFixtureIsFieldComplete(t *testing.T) {
	config := unboundParityConfig("fixture", "dot", "192.0.2.54", 0, "dns.example.test")
	if _, diagnostics := hclsyntax.ParseConfig([]byte(config), "story31d.tf", hcl.Pos{Line: 1, Column: 1}); diagnostics.HasErrors() {
		t.Fatal(diagnostics.Error())
	}
	for _, field := range generalParityFields {
		if !strings.Contains(config, field+" =") {
			t.Errorf("field-complete acceptance fixture omits %s", field)
		}
	}
	for _, required := range []string{"safe_search =", "type = [\"ag\"]", `type = "dot"`, `verify_cn = "dns.example.test"`, `data "opnsense_unbound_settings"`, `data "opnsense_unbound_domain_override"`} {
		if !strings.Contains(config, required) {
			t.Errorf("acceptance fixture omits %q", required)
		}
	}
	if strings.Contains(config, "server_port =") {
		t.Error("DoT acceptance fixture must exercise the omitted-port default")
	}
}

func TestIndependentDotAcceptanceFixtureUsesDistinctAddress(t *testing.T) {
	config := unboundIndependentDotConfig("fixture")
	if _, diagnostics := hclsyntax.ParseConfig([]byte(config), "story31d-dot.tf", hcl.Pos{Line: 1, Column: 1}); diagnostics.HasErrors() {
		t.Fatal(diagnostics.Error())
	}
	for _, required := range []string{
		`resource "opnsense_unbound_domain_override" "dot_independent"`,
		`data "opnsense_unbound_domain_override" "dot_independent"`,
		`id = opnsense_unbound_domain_override.dot_independent.id`,
		`type = "dot"`,
		`verify_cn = "independent.example.test"`,
	} {
		if !strings.Contains(config, required) {
			t.Errorf("independent DoT fixture omits %q", required)
		}
	}
	if strings.Contains(config, `resource "opnsense_unbound_domain_override" "parity"`) {
		t.Fatal("independent DoT fixture still uses the transition resource address")
	}
}

func TestDNSBLFixtureRollbackDeletesPartialItem(t *testing.T) {
	t.Parallel()
	deleteCalls, reconfigureCalls := 0, 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case dnsblItemOpts().DeleteEndpoint + "/partial":
			deleteCalls++
			_, _ = w.Write([]byte(`{"result":"deleted"}`))
		case dnsblItemOpts().ReconfigureEndpoint:
			reconfigureCalls++
			_, _ = w.Write([]byte(`{"status":"ok"}`))
		default:
			t.Errorf("unexpected rollback route %s", request.URL.Path)
		}
	}))
	t.Cleanup(server.Close)
	client, err := opnsense.NewClient(opnsense.ClientConfig{BaseURL: server.URL, APIKey: "key", APISecret: "secret", RetryMax: 1})
	if err != nil {
		t.Fatal(err)
	}
	fixtureErr := errors.New("fixture setup failed")
	result := joinDNSBLFixtureRollbackError(client, "partial", fixtureErr)
	if !errors.Is(result, fixtureErr) {
		t.Fatalf("rollback result=%v, want original fixture error", result)
	}
	if deleteCalls != 1 || reconfigureCalls != 1 {
		t.Fatalf("rollback delete=%d reconfigure=%d, want 1/1", deleteCalls, reconfigureCalls)
	}
}

func TestSettingsRestorePayloadIsRequestShapedAndBranchScoped(t *testing.T) {
	snapshot := json.RawMessage(`{"general":{"enabled":"1","active_interface":{"lan":{"value":"LAN","selected":1}},"local_zone_type":{"transparent":{"value":"transparent","selected":1}},"safesearch":"0"},"advanced":{"hideidentity":"1"},"acls":{"default_action":{"refuse":{"value":"refuse","selected":1}},"acl":{"unrelated":{"description":"keep"}}},"forwarding":{"enabled":"0"},"dnsbl":{"blocklist":{"unrelated":{}}},"dots":{"unrelated":{}},"hosts":{"unrelated":{}}}`)
	payload, err := settingsRestorePayload(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"dnsbl", "dots", "hosts"} {
		if _, ok := payload[forbidden]; ok {
			t.Fatalf("restore payload replays sibling branch %s: %#v", forbidden, payload)
		}
	}
	if got := payload["general"].(map[string]any)["active_interface"]; got != "lan" {
		t.Fatalf("selected GET object was not converted to request CSV: %#v", got)
	}
	if got := payload["acls"].(map[string]any)["default_action"]; got != "refuse" {
		t.Fatalf("ACL default restore = %#v", got)
	}
	if _, ok := payload["acls"].(map[string]any)["acl"]; ok {
		t.Fatal("restore payload replays unrelated ACL array")
	}
}

func unboundParityPreCheck(t *testing.T) {
	t.Helper()
	acctest.PreCheck(t)
	if os.Getenv("OPNSENSE_UNBOUND_PARITY") != "1" {
		t.Skip("set OPNSENSE_UNBOUND_PARITY=1 only through the Story 31.0d disposable runner")
	}
	want := os.Getenv("OPNSENSE_DISPOSABLE_VM_NAME") + ":" + os.Getenv("OPNSENSE_DISPOSABLE_VM_UUID")
	if want == ":" || os.Getenv("OPNSENSE_UNBOUND_IDENTITY_ATTESTATION") != want {
		t.Fatal("identity attestation does not match the disposable VM name and UUID")
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
	if response.StatusCode != http.StatusOK || json.NewDecoder(response.Body).Decode(&status) != nil ||
		!strings.Contains(strings.ToLower(status.Product.Name+" "+status.Product.ID), "opnsense") || status.Product.Version != "26.7.7" {
		t.Fatalf("Story 31.0d requires verified OPNsense 26.7.7; got HTTP %d identity=%q version=%q", response.StatusCode, status.Product.Name+" "+status.Product.ID, status.Product.Version)
	}
}

func getUnboundSnapshot(client *opnsense.Client) (json.RawMessage, error) {
	response, err := client.HTTPClient().Get(client.BaseURL() + "/api/unbound/settings/get")
	if err != nil {
		return nil, err
	}
	defer func() { _ = response.Body.Close() }()
	var envelope map[string]json.RawMessage
	if response.StatusCode != http.StatusOK || json.NewDecoder(response.Body).Decode(&envelope) != nil {
		return nil, fmt.Errorf("unable to snapshot complete Unbound settings: HTTP %d", response.StatusCode)
	}
	unbound, ok := envelope["unbound"]
	if !ok {
		return nil, fmt.Errorf("snapshot lacks unbound monad")
	}
	var canonical any
	if err := json.Unmarshal(unbound, &canonical); err != nil {
		return nil, err
	}
	return json.Marshal(canonical)
}

func createUnboundDNSBLFixture(client *opnsense.Client, marker string) (string, error) {
	rows, err := opnsense.Search[struct {
		UUID string `json:"uuid"`
	}](context.Background(), client, dnsblItemOpts(), opnsense.SearchParams{})
	if err != nil {
		return "", err
	}
	if len(rows) != 0 {
		return "", fmt.Errorf("Story 31.0d disposable fixture requires zero pre-existing DNSBL items; found %d", len(rows))
	}
	request := struct {
		Enabled     string `json:"enabled"`
		Type        string `json:"type"`
		Lists       string `json:"lists"`
		Allowlists  string `json:"allowlists"`
		Blocklists  string `json:"blocklists"`
		Wildcards   string `json:"wildcards"`
		SourceNets  string `json:"source_nets"`
		Address     string `json:"address"`
		NXDomain    string `json:"nxdomain"`
		CacheTTL    string `json:"cache_ttl"`
		Description string `json:"description"`
	}{"0", "ag", "https://example.invalid/" + marker, "allow." + marker + ".example.test", "block." + marker + ".example.test", "wild." + marker + ".example.test", "192.0.2.0/24", "192.0.2.31", "1", "73131", marker}
	uuid, err := opnsense.Add(context.Background(), client, dnsblItemOpts(), &request)
	if err != nil {
		return uuid, joinDNSBLFixtureRollbackError(client, uuid, err)
	}
	payload := map[string]any{"general": map[string]any{"safesearch": "1"}}
	if err := opnsense.UpdateSingleton(context.Background(), client, settingsOpts(), &payload); err != nil {
		return uuid, joinDNSBLFixtureRollbackError(client, uuid, err)
	}
	if err := opnsense.RetryReconfigure(context.Background(), client, dnsblItemOpts()); err != nil {
		return uuid, joinDNSBLFixtureRollbackError(client, uuid, err)
	}
	return uuid, nil
}

func joinDNSBLFixtureRollbackError(client *opnsense.Client, uuid string, fixtureErr error) error {
	if uuid == "" {
		return fixtureErr
	}
	rollbackErr := opnsense.Delete(context.Background(), client, dnsblItemOpts(), uuid)
	var notFound *opnsense.NotFoundError
	if rollbackErr == nil || errors.As(rollbackErr, &notFound) {
		return fixtureErr
	}
	return fmt.Errorf("%w (DNSBL fixture rollback also failed: %v)", fixtureErr, rollbackErr)
}

// checkTaggedForwardsAbsent proves that Terraform's provider delete callbacks
// removed both endpoint families before rollback cleanup is allowed to mutate
// the appliance. restoreUnboundSnapshot retains active deletion only as an
// idempotent failure-safety path.
func checkTaggedForwardsAbsent(t *testing.T, marker string) resource.TestCheckFunc {
	return func(_ *terraform.State) error {
		client := acctest.TestClient(t)
		for _, namespace := range []struct {
			name string
			opts opnsense.ReqOpts
		}{{"forward", forwardOpts(false)}, {"dot", forwardOpts(true)}} {
			rows, err := opnsense.Search[struct {
				UUID string `json:"uuid"`
			}](context.Background(), client, namespace.opts, opnsense.SearchParams{SearchQuery: marker})
			if err != nil {
				return err
			}
			t.Logf("UNBOUND_TERRAFORM_DESTROY namespace=%s marker=%s total=%d", namespace.name, marker, len(rows))
			if len(rows) != 0 {
				return fmt.Errorf("Terraform destroy left %d owner-tagged %s forwards", len(rows), namespace.name)
			}
		}
		return nil
	}
}

func restoreUnboundSnapshot(t *testing.T, marker string, snapshot *unboundParitySnapshot) resource.TestCheckFunc {
	return func(_ *terraform.State) error {
		if snapshot.restored {
			return nil
		}
		client := acctest.TestClient(t)
		if len(snapshot.canonical) == 0 {
			return fmt.Errorf("complete Unbound snapshot is unavailable")
		}
		dnsblRows, err := opnsense.Search[struct {
			UUID string `json:"uuid"`
		}](context.Background(), client, dnsblItemOpts(), opnsense.SearchParams{SearchQuery: marker})
		if err != nil {
			return err
		}
		for _, row := range dnsblRows {
			if err := opnsense.Delete(context.Background(), client, dnsblItemOpts(), row.UUID); err != nil {
				return err
			}
		}
		dnsblRows, err = opnsense.Search[struct {
			UUID string `json:"uuid"`
		}](context.Background(), client, dnsblItemOpts(), opnsense.SearchParams{SearchQuery: marker})
		if err != nil {
			return err
		}
		t.Logf("UNBOUND_PARITY_CLEANUP namespace=dnsbl marker=%s total=%d", marker, len(dnsblRows))
		if len(dnsblRows) != 0 {
			return fmt.Errorf("DNSBL cleanup found %d marker matches", len(dnsblRows))
		}
		for namespace, options := range map[string]opnsense.ReqOpts{"forward": forwardOpts(false), "dot": forwardOpts(true)} {
			if err := deleteTaggedForwards(client, marker, options); err != nil {
				return err
			}
			rows, err := opnsense.Search[struct {
				UUID string `json:"uuid"`
			}](context.Background(), client, options, opnsense.SearchParams{SearchQuery: marker})
			if err != nil {
				return err
			}
			t.Logf("UNBOUND_PARITY_CLEANUP namespace=%s marker=%s total=%d", namespace, marker, len(rows))
			if len(rows) != 0 {
				return fmt.Errorf("%s cleanup found %d marker matches", namespace, len(rows))
			}
		}
		payload, err := settingsRestorePayload(snapshot.canonical)
		if err != nil {
			return err
		}
		if err := opnsense.UpdateSingleton(context.Background(), client, generalSettingsOpts(), &payload); err != nil {
			return fmt.Errorf("restore request-shaped Unbound owner fields: %w", err)
		}
		if err := opnsense.Reconfigure(context.Background(), client, opnsense.ReqOpts{ReconfigureEndpoint: "/api/unbound/service/reconfigure", StrictReconfigureResponse: true}); err != nil {
			return fmt.Errorf("activate restored SafeSearch/forwarding settings: %w", err)
		}
		after, err := getUnboundSnapshot(client)
		if err != nil {
			return err
		}
		equal := string(after) == string(snapshot.canonical)
		t.Logf("UNBOUND_SETTINGS_RESTORE equal=%t", equal)
		if !equal {
			return fmt.Errorf("complete Unbound settings differ after request-shaped restore")
		}
		snapshot.restored = true
		return nil
	}
}

func settingsRestorePayload(snapshot json.RawMessage) (map[string]any, error) {
	var tree map[string]json.RawMessage
	if err := json.Unmarshal(snapshot, &tree); err != nil {
		return nil, err
	}
	keys := map[string][]string{
		"general":  {"enabled", "port", "stats", "active_interface", "dnssec", "dns64", "dns64prefix", "noarecords", "regdhcp", "regdhcpdomain", "regdhcpstatic", "noreglladdr6", "noregrecords", "txtsupport", "cacheflush", "safesearch", "local_zone_type", "outgoing_interface", "enable_wpad"},
		"advanced": {"hideidentity", "hideversion", "prefetchkey", "dnssecstripped", "aggressivensec", "qnameminstrict", "outgoingnumtcp", "incomingnumtcp", "numqueriesperthread", "outgoingrange", "jostletimeout", "discardtimeout", "privatedomain", "privateaddress", "insecuredomain", "prefetch", "unwantedreplythreshold", "msgcachesize", "rrsetcachesize", "cachemaxttl", "cachemaxnegativettl", "cacheminttl", "infrahostttl", "infrakeepprobing", "infracachenumhosts", "extendedstatistics", "logqueries", "logreplies", "logtagqueryreply", "loglocalactions", "logservfail", "logverbosity", "valloglevel", "serveexpired", "serveexpiredreplyttl", "serveexpiredttl", "serveexpiredttlreset", "serveexpiredclienttimeout"},
		"acls":     {"default_action"}, "forwarding": {"enabled"},
	}
	payload := make(map[string]any, len(keys))
	for branch, fields := range keys {
		var source map[string]json.RawMessage
		if err := json.Unmarshal(tree[branch], &source); err != nil {
			return nil, fmt.Errorf("decode snapshot branch %s: %w", branch, err)
		}
		destination := make(map[string]any, len(fields))
		for _, field := range fields {
			requireOne := (branch == "general" && field == "local_zone_type") ||
				(branch == "acls" && field == "default_action")
			value, err := requestScalar(source[field], requireOne)
			if err != nil {
				return nil, fmt.Errorf("decode snapshot %s.%s: %w", branch, field, err)
			}
			destination[field] = value
		}
		payload[branch] = destination
	}
	return payload, nil
}

func requestScalar(raw json.RawMessage, requireOne bool) (string, error) {
	if len(raw) == 0 || string(raw) == "null" || string(raw) == "[]" {
		if requireOne {
			return "", fmt.Errorf("selected field contains no selection")
		}
		return "", nil
	}
	var scalar string
	if err := json.Unmarshal(raw, &scalar); err == nil {
		if requireOne && scalar == "" {
			return "", fmt.Errorf("selected field is empty")
		}
		return scalar, nil
	}
	var indexed []struct {
		Selected json.Number `json:"selected"`
	}
	if err := json.Unmarshal(raw, &indexed); err == nil {
		selected := -1
		for index, option := range indexed {
			switch option.Selected.String() {
			case "0":
			case "1":
				if selected >= 0 {
					return "", fmt.Errorf("selected-option array contains multiple selections")
				}
				selected = index
			default:
				return "", fmt.Errorf("selected-option array has invalid selection %q", option.Selected.String())
			}
		}
		if selected < 0 {
			return "", fmt.Errorf("selected-option array contains no selection")
		}
		return strconv.Itoa(selected), nil
	}
	var options map[string]struct {
		Value    string      `json:"value"`
		Selected json.Number `json:"selected"`
	}
	if err := json.Unmarshal(raw, &options); err != nil {
		return "", err
	}
	selected := make([]string, 0)
	for key, option := range options {
		switch option.Selected.String() {
		case "0":
		case "1":
			selected = append(selected, key)
		default:
			return "", fmt.Errorf("selected-option map has invalid selection %q", option.Selected.String())
		}
	}
	if requireOne && len(selected) != 1 {
		return "", fmt.Errorf("selected-option map contains %d selections; expected exactly one", len(selected))
	}
	sort.Strings(selected)
	return strings.Join(selected, ","), nil
}

func deleteTaggedForwards(client *opnsense.Client, marker string, options opnsense.ReqOpts) error {
	rows, err := opnsense.Search[struct {
		UUID string `json:"uuid"`
	}](context.Background(), client, options, opnsense.SearchParams{SearchQuery: marker})
	if err != nil {
		return err
	}
	for _, row := range rows {
		if err := opnsense.Delete(context.Background(), client, options, row.UUID); err != nil {
			return err
		}
	}
	return nil
}

func dnsblItemOpts() opnsense.ReqOpts {
	return opnsense.ReqOpts{AddEndpoint: "/api/unbound/settings/add_dnsbl", GetEndpoint: "/api/unbound/settings/get_dnsbl", UpdateEndpoint: "/api/unbound/settings/set_dnsbl", DeleteEndpoint: "/api/unbound/settings/del_dnsbl", SearchEndpoint: "/api/unbound/settings/search_dnsbl", ReconfigureEndpoint: "/api/unbound/service/reconfigure", StrictReconfigureResponse: true, Monad: "blocklist", StrictDeleteResponse: true, StrictMissingObjectResponse: true}
}

func settingsOpts() opnsense.ReqOpts {
	return opnsense.ReqOpts{GetEndpoint: "/api/unbound/settings/get", UpdateEndpoint: "/api/unbound/settings/set", ReconfigureEndpoint: "/api/unbound/service/reconfigure", StrictReconfigureResponse: true, Monad: "unbound"}
}

func generalSettingsOpts() opnsense.ReqOpts {
	options := settingsOpts()
	options.ReconfigureEndpoint = "/api/unbound/service/reconfigure_general"
	return options
}

func forwardOpts(dot bool) opnsense.ReqOpts {
	prefix := "forward"
	if dot {
		prefix = "dot"
	}
	return opnsense.ReqOpts{AddEndpoint: "/api/unbound/settings/add_" + prefix, GetEndpoint: "/api/unbound/settings/get_forward", UpdateEndpoint: "/api/unbound/settings/set_" + prefix, DeleteEndpoint: "/api/unbound/settings/del_forward", SearchEndpoint: "/api/unbound/settings/search_" + prefix, ReconfigureEndpoint: "/api/unbound/service/reconfigure", StrictReconfigureResponse: true, Monad: "dot", StrictDeleteResponse: true, StrictMissingObjectResponse: true}
}

func driftUnboundHideIdentity(t *testing.T, value bool) resource.TestCheckFunc {
	return func(_ *terraform.State) error {
		payload := map[string]any{"advanced": map[string]any{"hideidentity": opnsense.BoolToString(value)}}
		return opnsense.UpdateSingleton(context.Background(), acctest.TestClient(t), generalSettingsOpts(), &payload)
	}
}

func driftUnboundForward(t *testing.T, forwardType, description string) resource.TestCheckFunc {
	return driftUnboundForwardAt(t, "opnsense_unbound_domain_override.parity", forwardType, description)
}

func driftUnboundForwardAt(t *testing.T, resourceAddress, forwardType, description string) resource.TestCheckFunc {
	return func(state *terraform.State) error {
		attributes := state.RootModule().Resources[resourceAddress].Primary.Attributes
		port, err := strconv.ParseInt(attributes["server_port"], 10, 64)
		if err != nil {
			return err
		}
		request := struct {
			Enabled     string `json:"enabled"`
			Type        string `json:"type"`
			Domain      string `json:"domain"`
			Server      string `json:"server"`
			Port        string `json:"port"`
			Verify      string `json:"verify"`
			Description string `json:"description"`
		}{opnsense.BoolToString(attributes["enabled"] == "true"), map[string]string{"query": "forward", "dot": "dot"}[forwardType], attributes["domain"], attributes["server"], strconv.FormatInt(port, 10), attributes["verify_cn"], description}
		return opnsense.Update(context.Background(), acctest.TestClient(t), forwardOpts(forwardType == "dot"), &request, attributes["id"])
	}
}

func driftUnboundDNSBLAddress(t *testing.T, snapshot *unboundParitySnapshot, address string) resource.TestCheckFunc {
	return func(_ *terraform.State) error {
		if snapshot.dnsblUUID == "" {
			return fmt.Errorf("DNSBL fixture UUID is unavailable")
		}
		client := acctest.TestClient(t)
		item, err := opnsense.Get[struct {
			Enabled     string                   `json:"enabled"`
			Type        opnsense.SelectedMapList `json:"type"`
			Lists       opnsense.SelectedMapList `json:"lists"`
			Allowlists  opnsense.SelectedMapList `json:"allowlists"`
			Blocklists  opnsense.SelectedMapList `json:"blocklists"`
			Wildcards   opnsense.SelectedMapList `json:"wildcards"`
			SourceNets  opnsense.SelectedMapList `json:"source_nets"`
			Address     string                   `json:"address"`
			NXDomain    string                   `json:"nxdomain"`
			CacheTTL    string                   `json:"cache_ttl"`
			Description string                   `json:"description"`
		}](context.Background(), client, dnsblItemOpts(), snapshot.dnsblUUID)
		if err != nil {
			return err
		}
		request := struct {
			Enabled     string `json:"enabled"`
			Type        string `json:"type"`
			Lists       string `json:"lists"`
			Allowlists  string `json:"allowlists"`
			Blocklists  string `json:"blocklists"`
			Wildcards   string `json:"wildcards"`
			SourceNets  string `json:"source_nets"`
			Address     string `json:"address"`
			NXDomain    string `json:"nxdomain"`
			CacheTTL    string `json:"cache_ttl"`
			Description string `json:"description"`
		}{
			Enabled: item.Enabled, Type: strings.Join(item.Type, ","), Lists: strings.Join(item.Lists, ","),
			Allowlists: strings.Join(item.Allowlists, ","), Blocklists: strings.Join(item.Blocklists, ","),
			Wildcards: strings.Join(item.Wildcards, ","), SourceNets: strings.Join(item.SourceNets, ","),
			Address: address, NXDomain: item.NXDomain, CacheTTL: item.CacheTTL, Description: item.Description,
		}
		return opnsense.Update(context.Background(), client, dnsblItemOpts(), &request, snapshot.dnsblUUID)
	}
}

func compareResourceDataFields(resourceAddress, dataAddress string, fields []string, rename map[string]string) resource.TestCheckFunc {
	return func(state *terraform.State) error {
		managed := state.RootModule().Resources[resourceAddress].Primary.Attributes
		view := state.RootModule().Resources[dataAddress].Primary.Attributes
		for _, field := range fields {
			viewField := field
			if replacement := rename[field]; replacement != "" {
				viewField = replacement
			}
			if err := compareTerraformAttribute(managed, field, view, viewField); err != nil {
				return fmt.Errorf("%s ↔ %s: %w", resourceAddress, dataAddress, err)
			}
		}
		return nil
	}
}

func compareTerraformAttribute(left map[string]string, leftName string, right map[string]string, rightName string) error {
	if left[leftName] != "" || right[rightName] != "" {
		if left[leftName] != right[rightName] {
			return fmt.Errorf("%s=%q differs from %s=%q", leftName, left[leftName], rightName, right[rightName])
		}
		return nil
	}
	leftValues, rightValues := collectionValues(left, leftName), collectionValues(right, rightName)
	if strings.Join(leftValues, "\x00") != strings.Join(rightValues, "\x00") {
		return fmt.Errorf("%s=%v differs from %s=%v", leftName, leftValues, rightName, rightValues)
	}
	return nil
}

func collectionValues(attributes map[string]string, name string) []string {
	values := make([]string, 0)
	for key, value := range attributes {
		if strings.HasPrefix(key, name+".") && key != name+".#" && key != name+".%" {
			values = append(values, value)
		}
	}
	sort.Strings(values)
	return values
}

func compareDNSBLSettings(t *testing.T, marker, expectedAddress string, snapshot *unboundParitySnapshot) resource.TestCheckFunc {
	return func(state *terraform.State) error {
		managed := state.RootModule().Resources["opnsense_unbound_dnsbl.parity"].Primary.Attributes
		view := state.RootModule().Resources["data.opnsense_unbound_settings.parity"].Primary.Attributes
		if view["dnsbl_items.#"] != "1" {
			return fmt.Errorf("expected one typed DNSBL item, got %q", view["dnsbl_items.#"])
		}
		if managed["safe_search"] != view["safe_search"] {
			return fmt.Errorf("safe_search resource=%q data=%q", managed["safe_search"], view["safe_search"])
		}
		for _, field := range []string{"enabled", "type", "lists", "whitelists", "blocklists", "wildcards", "address", "nxdomain"} {
			viewField := "dnsbl_items.0." + field
			if field == "whitelists" {
				viewField = "dnsbl_items.0.allowlists"
			}
			if err := compareTerraformAttribute(managed, field, view, viewField); err != nil {
				return err
			}
		}
		if managed["address"] != expectedAddress {
			return fmt.Errorf("DNSBL address=%q, want %q", managed["address"], expectedAddress)
		}
		if view["dnsbl_items.0.uuid"] != snapshot.dnsblUUID {
			return fmt.Errorf("typed DNSBL uuid=%q, want API-created %q", view["dnsbl_items.0.uuid"], snapshot.dnsblUUID)
		}
		if got := collectionValues(view, "dnsbl_items.0.source_nets"); len(got) != 1 || got[0] != "192.0.2.0/24" {
			return fmt.Errorf("typed DNSBL source_nets=%v, want [192.0.2.0/24]", got)
		}
		if view["dnsbl_items.0.cache_ttl"] != "73131" || view["dnsbl_items.0.description"] != marker {
			return fmt.Errorf("typed DNSBL cache_ttl=%q description=%q", view["dnsbl_items.0.cache_ttl"], view["dnsbl_items.0.description"])
		}
		authoritative, err := getUnboundSnapshot(acctest.TestClient(t))
		if err != nil {
			return err
		}
		var tree map[string]json.RawMessage
		if err := json.Unmarshal(authoritative, &tree); err != nil {
			return err
		}
		var dnsbl any
		if err := json.Unmarshal(tree["dnsbl"], &dnsbl); err != nil {
			return err
		}
		canonical, err := json.Marshal(dnsbl)
		if err != nil {
			return err
		}
		if view["dnsbl_json"] != string(canonical) {
			return fmt.Errorf("dnsbl_json differs from authoritative canonical DNSBL branch")
		}
		return nil
	}
}

func unboundDNSBLImportConfig(marker string) string {
	return fmt.Sprintf(`
resource "opnsense_unbound_dnsbl" "parity" {
  enabled = false
  safe_search = true
  type = ["ag"]
  lists = [%q]
  whitelists = [%q]
  blocklists = [%q]
  wildcards = [%q]
  address = "192.0.2.31"
  nxdomain = true
}
`, "https://example.invalid/"+marker, "allow."+marker+".example.test", "block."+marker+".example.test", "wild."+marker+".example.test")
}

func unboundParityConfig(marker, forwardType, server string, port int, verify string) string {
	portLine := ""
	if port != 0 {
		portLine = fmt.Sprintf("  server_port = %d\n", port)
	}
	return fmt.Sprintf(`
resource "opnsense_unbound_general" "parity" {
  enabled = false
  port = "53535"
  stats = true
  active_interface = []
  dnssec = false
  dns64 = true
  dns64_prefix = "64:ff9b:1::/96"
  no_aaaa_records = true
  register_dhcp = true
  register_dhcp_domain = "story31d.example.test"
  register_dhcp_static = true
  no_register_link_local6 = true
  no_register_records = true
  txt_support = true
  cache_flush = true
  local_zone_type = "refuse"
  outgoing_interface = []
  enable_wpad = true
  hide_identity = true
  hide_version = true
  enable_prefetch_key = true
  dnssec_stripped = true
  aggressive_nsec = false
  qname_min_strict = true
  outgoing_num_tcp = 31
  incoming_num_tcp = 32
  num_queries_per_thread = 1025
  outgoing_range = 4097
  jostle_timeout = 201
  discard_timeout = 1901
  private_domains = ["private.story31d.example.test"]
  private_addresses = ["192.0.2.0/24"]
  insecure_domains = ["insecure.story31d.example.test"]
  enable_prefetch = true
  unwanted_reply_threshold = 10000001
  msg_cache_size = "17m"
  rrset_cache_size = "33m"
  max_ttl = 86399
  max_negative_ttl = 3599
  min_ttl = 31
  host_ttl = 901
  keep_probing_hosts = true
  num_hosts = 10001
  extended_statistics = true
  log_queries = true
  log_replies = true
  tag_query_reply = true
  log_local_actions = true
  log_servfail = true
  verbosity_level = 2
  validation_level = 1
  serve_expired_enabled = true
  serve_expired_record_reply_ttl = "31"
  serve_expired_ttl = "86431"
  serve_expired_reset_ttl = true
  serve_expired_client_response_timeout = "181"
  acl_default_action = "refuse"
  forwarding_enabled = true
}
%s
resource "opnsense_unbound_domain_override" "parity" {
  enabled = true
  domain = %q
  type = %q
  server = %q
%s  verify_cn = %q
  description = %q
}
data "opnsense_unbound_settings" "parity" {
  depends_on = [opnsense_unbound_general.parity, opnsense_unbound_dnsbl.parity]
}
data "opnsense_unbound_domain_override" "parity" {
  id = opnsense_unbound_domain_override.parity.id
}
`, unboundDNSBLImportConfig(marker), marker+".example.test", forwardType, server, portLine, verify, marker)
}

func unboundIndependentDotConfig(marker string) string {
	config := unboundParityConfig(marker, "dot", "192.0.2.55", 0, "independent.example.test")
	config = strings.Replace(config, `resource "opnsense_unbound_domain_override" "parity"`, `resource "opnsense_unbound_domain_override" "dot_independent"`, 1)
	config = strings.Replace(config, `data "opnsense_unbound_domain_override" "parity"`, `data "opnsense_unbound_domain_override" "dot_independent"`, 1)
	return strings.Replace(config, "opnsense_unbound_domain_override.parity.id", "opnsense_unbound_domain_override.dot_independent.id", 1)
}
