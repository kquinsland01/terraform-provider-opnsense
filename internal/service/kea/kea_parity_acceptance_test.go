// Copyright (c) Matthew Mellor
// SPDX-License-Identifier: MPL-2.0

package kea_test

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/hashicorp/go-retryablehttp"
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"

	"github.com/matthew-on-git/terraform-provider-opnsense/internal/acctest"
	"github.com/matthew-on-git/terraform-provider-opnsense/pkg/opnsense"
)

func TestAccKeaFieldAndSurfaceParity(t *testing.T) {
	suffix := keaParityRunMarker(t)
	iface := os.Getenv("OPNSENSE_KEA_TEST_INTERFACE")
	config := testAccKeaParityConfig(suffix, iface, "standby")
	updated := testAccKeaParityConfig(suffix, iface, "primary")

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		PreCheck:                 keaParityPreCheck(t, suffix),
		CheckDestroy:             checkKeaParityCleanup(t, suffix),
		Steps: []resource.TestStep{
			{Config: config, Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttr("opnsense_kea_dhcpv4_subnet.parity", "dns_servers.#", "2"),
				resource.TestCheckResourceAttr("opnsense_kea_dhcpv4_subnet.parity", "static_routes.#", "2"),
				resource.TestCheckResourceAttr("opnsense_kea_dhcpv6_subnet.parity", "domain_search.#", "2"),
				resource.TestCheckResourceAttr("opnsense_kea_dhcpv6_peer.parity", "role", "standby"),
				resource.TestCheckResourceAttr("opnsense_kea_dhcpv6_peer.parity", "name", "story31b-"+suffix),
				resource.TestCheckResourceAttr("opnsense_kea_dhcpv6_peer.parity", "url", "http://[2001:db8::31]:8000/"),
				resource.TestCheckResourceAttrPair("opnsense_kea_dhcpv6_pd_pool.parity", "subnet_id", "opnsense_kea_dhcpv6_subnet.parity", "id"),
				resource.TestCheckResourceAttrPair("data.opnsense_kea_dhcpv6_peer.parity", "id", "opnsense_kea_dhcpv6_peer.parity", "id"),
				resource.TestCheckResourceAttrPair("data.opnsense_kea_dhcpv4_subnet.parity", "id", "opnsense_kea_dhcpv4_subnet.parity", "id"),
				resource.TestCheckResourceAttr("data.opnsense_kea_dhcpv4_subnet.parity", "dns_servers.#", "2"),
				resource.TestCheckTypeSetElemAttr("data.opnsense_kea_dhcpv4_subnet.parity", "dns_servers.*", "192.0.2.53"),
				resource.TestCheckTypeSetElemAttr("data.opnsense_kea_dhcpv4_subnet.parity", "dns_servers.*", "192.0.2.54"),
				resource.TestCheckResourceAttr("data.opnsense_kea_dhcpv4_subnet.parity", "domain_name", "story31b.example.test"),
				resource.TestCheckResourceAttr("data.opnsense_kea_dhcpv4_subnet.parity", "domain_search.#", "2"),
				resource.TestCheckTypeSetElemAttr("data.opnsense_kea_dhcpv4_subnet.parity", "domain_search.*", "story31b.example.test"),
				resource.TestCheckTypeSetElemAttr("data.opnsense_kea_dhcpv4_subnet.parity", "domain_search.*", "services.example.test"),
				resource.TestCheckResourceAttr("data.opnsense_kea_dhcpv4_subnet.parity", "routers.#", "1"),
				resource.TestCheckTypeSetElemAttr("data.opnsense_kea_dhcpv4_subnet.parity", "routers.*", fmt.Sprintf("198.18.%d.1", keaTestSegment(suffix))),
				resource.TestCheckResourceAttr("data.opnsense_kea_dhcpv4_subnet.parity", "static_routes.#", "2"),
				checkKeaStaticRoutesExact("data.opnsense_kea_dhcpv4_subnet.parity", []keaStaticRoute{
					{destination: "198.51.100.1", router: fmt.Sprintf("198.18.%d.1", keaTestSegment(suffix))},
					{destination: "203.0.113.1", router: fmt.Sprintf("198.18.%d.1", keaTestSegment(suffix))},
				}),
				resource.TestCheckResourceAttr("data.opnsense_kea_dhcpv4_subnet.parity", "ntp_servers.#", "1"),
				resource.TestCheckTypeSetElemAttr("data.opnsense_kea_dhcpv4_subnet.parity", "ntp_servers.*", "192.0.2.20"),
				resource.TestCheckResourceAttr("data.opnsense_kea_dhcpv4_subnet.parity", "time_servers.#", "1"),
				resource.TestCheckTypeSetElemAttr("data.opnsense_kea_dhcpv4_subnet.parity", "time_servers.*", "192.0.2.21"),
				resource.TestCheckResourceAttr("data.opnsense_kea_dhcpv4_subnet.parity", "next_server", fmt.Sprintf("198.18.%d.22", keaTestSegment(suffix))),
				resource.TestCheckResourceAttr("data.opnsense_kea_dhcpv4_subnet.parity", "tftp_server", "tftp.example.test"),
				resource.TestCheckResourceAttr("data.opnsense_kea_dhcpv4_subnet.parity", "tftp_bootfile", "bootx64.efi"),
				resource.TestCheckResourceAttrPair("data.opnsense_kea_dhcpv6_subnet.parity", "id", "opnsense_kea_dhcpv6_subnet.parity", "id"),
				resource.TestCheckResourceAttr("data.opnsense_kea_dhcpv6_subnet.parity", "dns_servers.#", "2"),
				resource.TestCheckTypeSetElemAttr("data.opnsense_kea_dhcpv6_subnet.parity", "dns_servers.*", "2001:db8::53"),
				resource.TestCheckTypeSetElemAttr("data.opnsense_kea_dhcpv6_subnet.parity", "dns_servers.*", "2001:db8::54"),
				resource.TestCheckResourceAttr("data.opnsense_kea_dhcpv6_subnet.parity", "domain_search.#", "2"),
				resource.TestCheckTypeSetElemAttr("data.opnsense_kea_dhcpv6_subnet.parity", "domain_search.*", "story31b.example.test"),
				resource.TestCheckTypeSetElemAttr("data.opnsense_kea_dhcpv6_subnet.parity", "domain_search.*", "services.example.test"),
				resource.TestCheckResourceAttr("data.opnsense_kea_dhcpv6_peer.parity", "role", "standby"),
				resource.TestCheckResourceAttr("data.opnsense_kea_dhcpv6_peer.parity", "name", "story31b-"+suffix),
				resource.TestCheckResourceAttr("data.opnsense_kea_dhcpv6_peer.parity", "url", "http://[2001:db8::31]:8000/"),
				resource.TestCheckResourceAttrPair("data.opnsense_kea_dhcpv6_pd_pool.parity", "id", "opnsense_kea_dhcpv6_pd_pool.parity", "id"),
				resource.TestCheckResourceAttrPair("data.opnsense_kea_dhcpv6_pd_pool.parity", "subnet_id", "opnsense_kea_dhcpv6_subnet.parity", "id"),
				resource.TestCheckResourceAttr("data.opnsense_kea_dhcpv6_pd_pool.parity", "prefix_len", "48"),
				resource.TestCheckResourceAttr("data.opnsense_kea_dhcpv6_pd_pool.parity", "delegated_len", "64"),
				resource.TestCheckResourceAttr("data.opnsense_kea_dhcpv6_pd_pool.parity", "prefix", fmt.Sprintf("fd31:%x::", keaTestSegment(suffix))),
				resource.TestCheckResourceAttr("data.opnsense_kea_dhcpv6_pd_pool.parity", "description", "Story31b-"+suffix+"-pd"),
			)},
			{ResourceName: "opnsense_kea_dhcpv4_subnet.parity", ImportState: true, ImportStateVerify: true},
			{ResourceName: "opnsense_kea_dhcpv6_subnet.parity", ImportState: true, ImportStateVerify: true},
			{ResourceName: "opnsense_kea_dhcpv6_peer.parity", ImportState: true, ImportStateVerify: true},
			{ResourceName: "opnsense_kea_dhcpv6_pd_pool.parity", ImportState: true, ImportStateVerify: true},
			{Config: config, PlanOnly: true},
			{Config: config, Check: mutateKeaV4Description(t, suffix), ExpectNonEmptyPlan: true},
			{Config: config, PlanOnly: true, ExpectNonEmptyPlan: true},
			{Config: config, Check: resource.TestCheckResourceAttr("opnsense_kea_dhcpv4_subnet.parity", "description", "Story31b-"+suffix+"-v4")},
			{Config: config, PlanOnly: true},
			{Config: config, Check: mutateKeaV6PeerRole(t), ExpectNonEmptyPlan: true},
			{Config: config, PlanOnly: true, ExpectNonEmptyPlan: true},
			{Config: config, Check: resource.TestCheckResourceAttr("opnsense_kea_dhcpv6_peer.parity", "role", "standby")},
			{Config: config, PlanOnly: true},
			{Config: config, Check: mutateKeaV6PDDelegatedLength(t), ExpectNonEmptyPlan: true},
			{Config: config, PlanOnly: true, ExpectNonEmptyPlan: true},
			{Config: config, Check: resource.TestCheckResourceAttr("opnsense_kea_dhcpv6_pd_pool.parity", "delegated_len", "64")},
			{Config: config, PlanOnly: true},
			{Config: updated, Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttr("opnsense_kea_dhcpv6_peer.parity", "role", "primary"),
				resource.TestCheckResourceAttr("opnsense_kea_dhcpv4_subnet.parity", "pools", ""),
				resource.TestCheckResourceAttr("opnsense_kea_dhcpv4_subnet.parity", "dns_servers.#", "0"),
				resource.TestCheckResourceAttr("opnsense_kea_dhcpv4_subnet.parity", "domain_name", ""),
				resource.TestCheckResourceAttr("opnsense_kea_dhcpv4_subnet.parity", "domain_search.#", "0"),
				resource.TestCheckResourceAttr("opnsense_kea_dhcpv4_subnet.parity", "routers.#", "0"),
				resource.TestCheckResourceAttr("opnsense_kea_dhcpv4_subnet.parity", "static_routes.#", "0"),
				resource.TestCheckResourceAttr("opnsense_kea_dhcpv4_subnet.parity", "ntp_servers.#", "0"),
				resource.TestCheckResourceAttr("opnsense_kea_dhcpv4_subnet.parity", "time_servers.#", "0"),
				resource.TestCheckResourceAttr("opnsense_kea_dhcpv4_subnet.parity", "next_server", ""),
				resource.TestCheckResourceAttr("opnsense_kea_dhcpv4_subnet.parity", "tftp_server", ""),
				resource.TestCheckResourceAttr("opnsense_kea_dhcpv4_subnet.parity", "tftp_bootfile", ""),
				resource.TestCheckResourceAttr("opnsense_kea_dhcpv6_subnet.parity", "dns_servers.#", "0"),
				resource.TestCheckResourceAttr("opnsense_kea_dhcpv6_subnet.parity", "domain_search.#", "0"),
				resource.TestCheckResourceAttr("opnsense_kea_dhcpv6_pd_pool.parity", "prefix", fmt.Sprintf("fd31:%x::/48", keaTestSegment(suffix))),
				resource.TestCheckResourceAttr("data.opnsense_kea_dhcpv4_subnet.parity", "dns_servers.#", "0"),
				resource.TestCheckResourceAttr("data.opnsense_kea_dhcpv4_subnet.parity", "pools", ""),
				resource.TestCheckResourceAttr("data.opnsense_kea_dhcpv4_subnet.parity", "domain_name", ""),
				resource.TestCheckResourceAttr("data.opnsense_kea_dhcpv4_subnet.parity", "domain_search.#", "0"),
				resource.TestCheckResourceAttr("data.opnsense_kea_dhcpv4_subnet.parity", "routers.#", "0"),
				resource.TestCheckResourceAttr("data.opnsense_kea_dhcpv4_subnet.parity", "static_routes.#", "0"),
				resource.TestCheckResourceAttr("data.opnsense_kea_dhcpv4_subnet.parity", "ntp_servers.#", "0"),
				resource.TestCheckResourceAttr("data.opnsense_kea_dhcpv4_subnet.parity", "time_servers.#", "0"),
				resource.TestCheckResourceAttr("data.opnsense_kea_dhcpv4_subnet.parity", "next_server", ""),
				resource.TestCheckResourceAttr("data.opnsense_kea_dhcpv4_subnet.parity", "tftp_server", ""),
				resource.TestCheckResourceAttr("data.opnsense_kea_dhcpv4_subnet.parity", "tftp_bootfile", ""),
				resource.TestCheckResourceAttr("data.opnsense_kea_dhcpv6_subnet.parity", "dns_servers.#", "0"),
				resource.TestCheckResourceAttr("data.opnsense_kea_dhcpv6_subnet.parity", "domain_search.#", "0"),
				resource.TestCheckResourceAttr("data.opnsense_kea_dhcpv6_peer.parity", "role", "primary"),
				resource.TestCheckResourceAttr("data.opnsense_kea_dhcpv6_peer.parity", "name", "story31b-"+suffix),
				resource.TestCheckResourceAttr("data.opnsense_kea_dhcpv6_peer.parity", "url", "http://[2001:db8::31]:8000/"),
				resource.TestCheckResourceAttr("data.opnsense_kea_dhcpv6_pd_pool.parity", "prefix", fmt.Sprintf("fd31:%x::", keaTestSegment(suffix))),
				resource.TestCheckResourceAttr("data.opnsense_kea_dhcpv6_pd_pool.parity", "prefix_len", "48"),
				resource.TestCheckResourceAttr("data.opnsense_kea_dhcpv6_pd_pool.parity", "delegated_len", "64"),
				resource.TestCheckResourceAttr("data.opnsense_kea_dhcpv6_pd_pool.parity", "description", "Story31b-"+suffix+"-pd"),
			)},
			{Config: updated, PlanOnly: true},
		},
	})
}

type keaStaticRoute struct {
	destination string
	router      string
}

func checkKeaStaticRoutesExact(resourceName string, expected []keaStaticRoute) resource.TestCheckFunc {
	return func(state *terraform.State) error {
		entry := state.RootModule().Resources[resourceName]
		if entry == nil || entry.Primary == nil {
			return fmt.Errorf("%s missing from state", resourceName)
		}
		actual, err := keaStaticRoutesFromAttributes(entry.Primary.Attributes)
		if err != nil {
			return fmt.Errorf("%s: %w", resourceName, err)
		}
		want := append([]keaStaticRoute(nil), expected...)
		sort.Slice(want, func(i, j int) bool { return staticRouteKey(want[i]) < staticRouteKey(want[j]) })
		if len(actual) != len(want) {
			return fmt.Errorf("static_routes = %#v, want %#v", actual, want)
		}
		for i := range want {
			if actual[i] != want[i] {
				return fmt.Errorf("static_routes = %#v, want %#v", actual, want)
			}
		}
		return nil
	}
}

func keaStaticRoutesFromAttributes(attributes map[string]string) ([]keaStaticRoute, error) {
	count, err := strconv.Atoi(attributes["static_routes.#"])
	if err != nil {
		return nil, fmt.Errorf("invalid static_routes count %q", attributes["static_routes.#"])
	}
	byHash := make(map[string]*keaStaticRoute, count)
	seen := make(map[string]map[string]bool, count)
	for key, value := range attributes {
		if !strings.HasPrefix(key, "static_routes.") || key == "static_routes.#" {
			continue
		}
		parts := strings.Split(strings.TrimPrefix(key, "static_routes."), ".")
		if len(parts) != 2 || (parts[1] != "destination" && parts[1] != "router") {
			continue
		}
		route := byHash[parts[0]]
		if route == nil {
			route = &keaStaticRoute{}
			byHash[parts[0]] = route
			seen[parts[0]] = make(map[string]bool, 2)
		}
		if parts[1] == "destination" {
			route.destination = value
		} else {
			route.router = value
		}
		seen[parts[0]][parts[1]] = true
	}
	if len(byHash) != count {
		return nil, fmt.Errorf("decoded %d static_routes, state count is %d", len(byHash), count)
	}
	routes := make([]keaStaticRoute, 0, count)
	for hash, route := range byHash {
		if !seen[hash]["destination"] || !seen[hash]["router"] {
			return nil, fmt.Errorf("static_routes.%s is missing destination or router", hash)
		}
		routes = append(routes, *route)
	}
	sort.Slice(routes, func(i, j int) bool { return staticRouteKey(routes[i]) < staticRouteKey(routes[j]) })
	return routes, nil
}

func staticRouteKey(route keaStaticRoute) string {
	return route.destination + "\x00" + route.router
}

func TestKeaStaticRoutesFromAttributes(t *testing.T) {
	attributes := map[string]string{
		"static_routes.#":               "2",
		"static_routes.482.destination": "203.0.113.1",
		"static_routes.482.router":      "198.18.31.1",
		"static_routes.917.destination": "198.51.100.1",
		"static_routes.917.router":      "198.18.31.1",
	}
	routes, err := keaStaticRoutesFromAttributes(attributes)
	if err != nil {
		t.Fatal(err)
	}
	want := []keaStaticRoute{{destination: "198.51.100.1", router: "198.18.31.1"}, {destination: "203.0.113.1", router: "198.18.31.1"}}
	for i := range want {
		if routes[i] != want[i] {
			t.Fatalf("routes = %#v, want %#v", routes, want)
		}
	}
	delete(attributes, "static_routes.482.router")
	if _, err := keaStaticRoutesFromAttributes(attributes); err == nil {
		t.Fatal("expected incomplete set element to fail")
	}
}

type keaGeneratedCleanupCheck struct {
	resourceType string
	opts         opnsense.ReqOpts
}

func keaGeneratedPeerCleanupChecks() []keaGeneratedCleanupCheck {
	return []keaGeneratedCleanupCheck{
		{"opnsense_kea_dhcpv6_peer", opnsense.ReqOpts{GetEndpoint: "/api/kea/dhcpv6/get_peer", Monad: "peer", StrictMissingObjectResponse: true}},
	}
}

func keaGeneratedPDPoolCleanupChecks() []keaGeneratedCleanupCheck {
	return []keaGeneratedCleanupCheck{
		{"opnsense_kea_dhcpv6_pd_pool", opnsense.ReqOpts{GetEndpoint: "/api/kea/dhcpv6/get_pd_pool", Monad: "pd_pool", StrictMissingObjectResponse: true}},
		{"opnsense_kea_dhcpv6_subnet", opnsense.ReqOpts{GetEndpoint: "/api/kea/dhcpv6/get_subnet", Monad: "subnet6", StrictMissingObjectResponse: true}},
	}
}

func checkKeaGeneratedPeerCleanup(t *testing.T) resource.TestCheckFunc {
	return checkKeaGeneratedCleanup(t, keaGeneratedPeerCleanupChecks())
}

func checkKeaGeneratedPDPoolCleanup(t *testing.T) resource.TestCheckFunc {
	return checkKeaGeneratedCleanup(t, keaGeneratedPDPoolCleanupChecks())
}

func checkKeaGeneratedCleanup(t *testing.T, checks []keaGeneratedCleanupCheck) resource.TestCheckFunc {
	return func(state *terraform.State) error {
		client := acctest.TestClient(t)
		for _, check := range checks {
			for _, instance := range state.RootModule().Resources {
				if instance.Type != check.resourceType {
					continue
				}
				if err := verifyKeaGeneratedCleanup(context.Background(), client, check.opts, instance.Primary.ID); err != nil {
					return fmt.Errorf("strict cleanup %s id=%s: %w", check.resourceType, instance.Primary.ID, err)
				}
			}
		}
		return nil
	}
}

func verifyKeaGeneratedCleanup(ctx context.Context, client *opnsense.Client, opts opnsense.ReqOpts, id string) error {
	_, err := opnsense.Get[map[string]any](ctx, client, opts, id)
	var notFound *opnsense.NotFoundError
	if errors.As(err, &notFound) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("GET %s/%s: %w", opts.GetEndpoint, id, err)
	}
	return fmt.Errorf("GET %s/%s found the resource still present", opts.GetEndpoint, id)
}

func TestGeneratedKeaCleanupRequiresSourceBackedEmptyArrayAbsence(t *testing.T) {
	for _, test := range []struct {
		name   string
		status int
		body   string
		ok     bool
	}{
		{name: "monad empty array", status: http.StatusOK, body: `{"peer":[]}`, ok: true},
		{name: "top-level empty array", status: http.StatusOK, body: `[]`, ok: true},
		{name: "empty object", status: http.StatusOK, body: `{"peer":{}}`},
		{name: "null monad", status: http.StatusOK, body: `{"peer":null}`},
		{name: "missing monad", status: http.StatusOK, body: `{"other":[]}`},
		{name: "present object", status: http.StatusOK, body: `{"peer":{"name":"still-present"}}`},
		{name: "ambiguous 404", status: http.StatusNotFound, body: `[]`},
		{name: "non-success status", status: http.StatusConflict, body: `[]`},
	} {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(test.status)
				_, _ = io.WriteString(w, test.body)
			}))
			defer server.Close()
			client, err := opnsense.NewClient(opnsense.ClientConfig{BaseURL: server.URL, APIKey: "key", APISecret: "secret", RetryMax: 1})
			if err != nil {
				t.Fatal(err)
			}
			err = verifyKeaGeneratedCleanup(context.Background(), client, keaGeneratedPeerCleanupChecks()[0].opts, "peer-id")
			if (err == nil) != test.ok {
				t.Fatalf("ok=%v err=%v", test.ok, err)
			}
		})
	}
}

func TestGeneratedKeaCleanupPropagatesTransportErrors(t *testing.T) {
	client, err := opnsense.NewClient(opnsense.ClientConfig{BaseURL: "https://appliance.invalid", APIKey: "key", APISecret: "secret", RetryMax: 1})
	if err != nil {
		t.Fatal(err)
	}
	client.HTTPClient().Transport = failingRoundTripper{}
	if err := verifyKeaGeneratedCleanup(context.Background(), client, keaGeneratedPeerCleanupChecks()[0].opts, "peer-id"); err == nil {
		t.Fatal("transport error was accepted as successful cleanup")
	}
}

type failingRoundTripper struct{}

func (failingRoundTripper) RoundTrip(*http.Request) (*http.Response, error) {
	return nil, errors.New("synthetic transport failure")
}

func TestGeneratedKeaCleanupPoliciesUseStrictMissingResponses(t *testing.T) {
	checks := append(keaGeneratedPeerCleanupChecks(), keaGeneratedPDPoolCleanupChecks()...)
	if len(checks) != 3 {
		t.Fatalf("cleanup checks=%d, want peer, PD pool, and parent subnet", len(checks))
	}
	for _, check := range checks {
		if !check.opts.StrictMissingObjectResponse {
			t.Fatalf("%s cleanup does not require empty-array absence", check.resourceType)
		}
	}
}

func keaParityRunMarker(t *testing.T) string {
	t.Helper()
	marker := make([]byte, 8)
	if _, err := rand.Read(marker); err != nil {
		t.Fatalf("generate Kea parity run marker: %v", err)
	}
	return fmt.Sprintf("%x", marker)
}

func TestKeaParityAcceptanceConfigParses(t *testing.T) {
	config := testAccKeaParityConfig("parse123", "opt3", "standby")
	_, diagnostics := hclsyntax.ParseConfig([]byte(config), "kea-parity.tf", hcl.Pos{Line: 1, Column: 1})
	if diagnostics.HasErrors() {
		t.Fatalf("invalid Kea parity acceptance config: %s", diagnostics.Error())
	}
}

func TestGeneratedKeaParityConfigsUseUniqueIsolatedFixtures(t *testing.T) {
	for name, config := range map[string]string{
		"peer":    fmt.Sprintf(testAccDHCPv6PeerConfig, "unique123", "opt1", 42),
		"PD pool": fmt.Sprintf(testAccDHCPv6PDPoolConfig, "unique123", "opt1", 42),
	} {
		if !strings.Contains(config, "unique123") {
			t.Fatalf("%s config lacks unique run marker: %s", name, config)
		}
		if strings.Contains(config, `interface  = "lan"`) || strings.Contains(config, `interface  = "wan"`) {
			t.Fatalf("%s config uses a management interface: %s", name, config)
		}
		_, diagnostics := hclsyntax.ParseConfig([]byte(config), name+".tf", hcl.Pos{Line: 1, Column: 1})
		if diagnostics.HasErrors() {
			t.Fatalf("invalid generated %s config: %s", name, diagnostics.Error())
		}
	}
}

func TestValidateKeaStatusResponseRequiresHTTPHierarchyDisabledServicesAndInterface(t *testing.T) {
	valid := []byte(`{"dhcpv6":{"general":{"enabled":"0","interfaces":{"opt1":{"selected":1}}},"ha":{"enabled":"0"}}}`)
	if err := validateKeaStatusResponse(http.StatusOK, valid, "dhcpv6", "opt1"); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name   string
		status int
		body   []byte
		iface  string
	}{
		{"HTTP error", http.StatusServiceUnavailable, []byte(`{"error":"down"}`), "opt1"},
		{"missing root", http.StatusOK, []byte(`{}`), "opt1"},
		{"missing general", http.StatusOK, []byte(`{"dhcpv6":{"ha":{"enabled":"0"}}}`), "opt1"},
		{"missing HA", http.StatusOK, []byte(`{"dhcpv6":{"general":{"enabled":"0"}}}`), "opt1"},
		{"enabled service", http.StatusOK, []byte(`{"dhcpv6":{"general":{"enabled":"1","interfaces":{"opt1":{"selected":1}}},"ha":{"enabled":"0"}}}`), "opt1"},
		{"missing interfaces", http.StatusOK, []byte(`{"dhcpv6":{"general":{"enabled":"0"},"ha":{"enabled":"0"}}}`), "opt1"},
		{"absent interface", http.StatusOK, valid, "opt2"},
		{"unselected interface", http.StatusOK, []byte(`{"dhcpv6":{"general":{"enabled":"0","interfaces":{"opt1":{"selected":0}}},"ha":{"enabled":"0"}}}`), "opt1"},
	} {
		t.Run(test.name, func(t *testing.T) {
			if err := validateKeaStatusResponse(test.status, test.body, "dhcpv6", test.iface); err == nil {
				t.Fatal("expected preflight rejection")
			}
		})
	}
}

func TestValidateKeaStatusResponseRejectsNonExactBinaryFlags(t *testing.T) {
	for _, flag := range []string{`true`, `null`, `[]`, `{}`, `0.0`, `1.5`, `2`, `"00"`, `"true"`} {
		body := []byte(fmt.Sprintf(`{"dhcpv6":{"general":{"enabled":%s,"interfaces":{"opt1":{"selected":1}}},"ha":{"enabled":"0"}}}`, flag))
		if err := validateKeaStatusResponse(http.StatusOK, body, "dhcpv6", "opt1"); err == nil {
			t.Fatalf("accepted non-exact service flag %s", flag)
		}
		body = []byte(fmt.Sprintf(`{"dhcpv6":{"general":{"enabled":"0","interfaces":{"opt1":{"selected":%s}}},"ha":{"enabled":"0"}}}`, flag))
		if err := validateKeaStatusResponse(http.StatusOK, body, "dhcpv6", "opt1"); err == nil {
			t.Fatalf("accepted non-exact selected flag %s", flag)
		}
	}
}

func TestKeaVersionAtLeastAcceptsOPNsenseTwoAndThreePartVersions(t *testing.T) {
	for _, version := range []string{"26.7.7", "26.7.8_1", "27.1"} {
		if !keaVersionAtLeast(version, 26, 7, 7) {
			t.Fatalf("newer supported version rejected: %s", version)
		}
	}
	for _, version := range []string{"26.6.9", "25.7", "nonsense"} {
		if keaVersionAtLeast(version, 26, 7, 7) {
			t.Fatalf("unsupported version accepted: %s", version)
		}
	}
}

func TestEncodeKeaV4SubnetMutationProjectsProviderOwnedWireFields(t *testing.T) {
	readBack := []byte(`{
  "subnet4": {
    "subnet_id": "17",
    "subnet": "198.18.42.0/24",
    "allocator": {"random":{"value":"Random","selected":1},"iterative":{"selected":0}},
    "pools": "198.18.42.100-198.18.42.150",
    "match-client-id": "1",
    "ping_check": "0",
    "option_data_autocollect": "0",
    "next_server": "198.18.42.22",
    "description": "old",
    "valid_lifetime": "7200",
    "ddns_send_updates": "1",
    "classless_static_route": "unsupported",
    "v6_only_preferred": "unsupported",
    "v4_dnr": "unsupported",
    "option": {"unsupported":{"selected":1}},
    "option_data": {
      "domain_name_servers": {"192.0.2.54":{"selected":"1"},"192.0.2.53":{"selected":1}},
      "domain_name": "story31b.example.test",
      "domain_search": {"services.example.test":{"selected":1},"story31b.example.test":{"selected":1}},
      "routers": {"198.18.42.1":{"selected":1}},
      "static_routes": "198.51.100.1,198.18.42.1,203.0.113.1,198.18.42.1",
      "ntp_servers": {"192.0.2.20":{"selected":1}},
      "time_servers": {"192.0.2.21":{"selected":1}},
      "tftp_server_name": "tftp.example.test",
      "boot_file_name": "bootx64.efi",
      "unsupported_option": "drop-me"
    }
  }
}`)
	payload, err := encodeKeaV4SubnetMutation(readBack, "out-of-band")
	if err != nil {
		t.Fatal(err)
	}
	want := `{"subnet4":{"subnet":"198.18.42.0/24","allocator":"random","pools":"198.18.42.100-198.18.42.150","match-client-id":"1","ping_check":"0","option_data_autocollect":"0","next_server":"198.18.42.22","option_data":{"domain_name_servers":"192.0.2.53,192.0.2.54","domain_name":"story31b.example.test","domain_search":"services.example.test,story31b.example.test","routers":"198.18.42.1","static_routes":"198.51.100.1,198.18.42.1,203.0.113.1,198.18.42.1","ntp_servers":"192.0.2.20","time_servers":"192.0.2.21","tftp_server_name":"tftp.example.test","boot_file_name":"bootx64.efi"},"description":"out-of-band"}}`
	if string(payload) != want {
		t.Fatalf("unexpected set_subnet payload\n got: %s\nwant: %s", payload, want)
	}
	for _, forbidden := range []string{"subnet_id", "valid_lifetime", "ddns_send_updates", "classless_static_route", "v6_only_preferred", "v4_dnr", "unsupported_option"} {
		if bytes.Contains(payload, []byte(forbidden)) {
			t.Fatalf("read-only field %q leaked into payload: %s", forbidden, payload)
		}
	}
}

func TestPostKeaMutationPreservesBoundedHTTPDiagnostics(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		calls++
		username, password, ok := request.BasicAuth()
		if !ok || username != "key" || password != "secret" {
			t.Error("diagnostic client did not preserve API authentication")
		}
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = io.WriteString(w, strings.Repeat("x", 5000))
	}))
	t.Cleanup(server.Close)
	client, err := opnsense.NewClient(opnsense.ClientConfig{BaseURL: server.URL, APIKey: "key", APISecret: "secret", RetryMax: 1})
	if err != nil {
		t.Fatal(err)
	}
	err = postKeaMutation(client.HTTPClient(), server.URL+"/api/kea/dhcpv4/set_subnet/test-id", []byte(`{"subnet4":{"description":"drift"}}`))
	if err == nil {
		t.Fatal("expected HTTP diagnostic")
	}
	diagnostic := err.Error()
	if calls != 1 {
		t.Fatalf("diagnostic request retried %d times", calls)
	}
	for _, fragment := range []string{"HTTP 500", "...[truncated]", `request: {"subnet4":{"description":"drift"}}`} {
		if !strings.Contains(diagnostic, fragment) {
			t.Fatalf("diagnostic %q omitted %q", diagnostic, fragment)
		}
	}
	if len(diagnostic) > 4600 {
		t.Fatalf("diagnostic was not bounded: %d bytes", len(diagnostic))
	}
}

func TestPostKeaReconfigureSendsJSONObject(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost || request.URL.Path != "/api/kea/service/reconfigure" {
			t.Errorf("unexpected request %s %s", request.Method, request.URL.Path)
		}
		if got := request.Header.Get("Content-Type"); got != "application/json" {
			t.Errorf("content type = %q, want application/json", got)
		}
		body, err := io.ReadAll(request.Body)
		if err != nil {
			t.Errorf("read request body: %v", err)
		}
		if string(body) != `{}` {
			t.Errorf("reconfigure body = %q, want {}", body)
		}
		_, _ = io.WriteString(w, `{"status":"ok"}`)
	}))
	t.Cleanup(server.Close)

	if err := postKeaReconfigure(server.Client(), server.URL+"/api/kea/service/reconfigure"); err != nil {
		t.Fatal(err)
	}
}

func TestPostKeaHelpersRequireAffirmativeBodies(t *testing.T) {
	for _, test := range []struct {
		name        string
		body        string
		reconfigure bool
	}{
		{name: "mutation empty", body: `{}`},
		{name: "mutation unknown", body: `{"result":"unknown"}`},
		{name: "reconfigure empty", body: `{}`, reconfigure: true},
		{name: "reconfigure unknown", body: `{"status":"unknown"}`, reconfigure: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = io.WriteString(w, test.body) }))
			defer server.Close()
			var err error
			if test.reconfigure {
				err = postKeaReconfigure(server.Client(), server.URL)
			} else {
				err = postKeaMutation(server.Client(), server.URL, []byte(`{}`))
			}
			if err == nil {
				t.Fatalf("ambiguous HTTP 200 body accepted: %s", test.body)
			}
		})
	}
}

func TestPostKeaMutationAcceptsExactSavedResult(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"result":"saved"}`)
	}))
	defer server.Close()
	if err := postKeaMutation(server.Client(), server.URL, []byte(`{}`)); err != nil {
		t.Fatal(err)
	}
}

func keaParityGeneratedPreCheck(t *testing.T, suffix string) {
	t.Helper()
	keaParityStatusPreCheck(t)()
	client := acctest.TestClient(t)
	segment := keaTestSegment(suffix)
	for _, search := range []struct{ endpoint, query string }{
		{"/api/kea/dhcpv6/search_peer", "tfaccpeer6-" + suffix},
		{"/api/kea/dhcpv6/search_pd_pool", "Story31b-generated-" + suffix},
		{"/api/kea/dhcpv6/search_subnet", fmt.Sprintf("fd32:%x::/48", segment)},
	} {
		total, _, err := keaSearchMatches(client.HTTPClient(), client.BaseURL()+search.endpoint, search.query)
		if err != nil {
			t.Fatalf("Kea generated-fixture collision preflight %s query %q: %v", search.endpoint, search.query, err)
		}
		if total != 0 {
			t.Fatalf("Kea generated-fixture collision preflight %s found %d matches for %q", search.endpoint, total, search.query)
		}
	}
}

func keaParityPreCheck(t *testing.T, suffix string) func() {
	return func() {
		keaParityStatusPreCheck(t)()
		t.Logf("Kea parity fixture marker=%s", suffix)
		client := acctest.TestClient(t)
		segment := keaTestSegment(suffix)
		for _, search := range []struct{ endpoint, query string }{
			{"/api/kea/dhcpv4/search_subnet", fmt.Sprintf("198.18.%d.0/24", segment)},
			{"/api/kea/dhcpv6/search_subnet", fmt.Sprintf("fd31:%x::/48", segment)},
			{"/api/kea/dhcpv6/search_peer", "story31b-" + suffix},
			{"/api/kea/dhcpv6/search_pd_pool", "Story31b-" + suffix + "-pd"},
		} {
			total, err := keaSearchTotal(client.HTTPClient(), client.BaseURL()+search.endpoint, search.query)
			if err != nil {
				t.Fatalf("Kea collision preflight %s: %v", search.endpoint, err)
			}
			if total != 0 {
				t.Fatalf("Kea collision preflight %s found %d matches for %q", search.endpoint, total, search.query)
			}
		}
	}
}

func keaParityStatusPreCheck(t *testing.T) func() {
	return func() {
		acctest.PreCheck(t)
		if os.Getenv("OPNSENSE_KEA_PARITY") != "1" {
			t.Skip("set OPNSENSE_KEA_PARITY=1 only for a disposable OPNsense 26.7.7+ appliance")
		}
		if os.Getenv("OPNSENSE_KEA_TEST_INTERFACE") == "" {
			t.Fatal("OPNSENSE_KEA_TEST_INTERFACE must name an isolated disposable interface")
		}
		iface := os.Getenv("OPNSENSE_KEA_TEST_INTERFACE")
		if strings.EqualFold(iface, "lan") || strings.EqualFold(iface, "wan") {
			t.Fatalf("OPNSENSE_KEA_TEST_INTERFACE must not be a management interface: %q", iface)
		}
		client := acctest.TestClient(t)
		version := readKeaFirmwareVersion(t, client.HTTPClient(), client.BaseURL()+"/api/core/firmware/info")
		if !keaVersionAtLeast(version, 26, 7, 7) {
			t.Fatalf("requires OPNsense 26.7.7+, got %s", version)
		}
		for _, family := range []string{"dhcpv4", "dhcpv6"} {
			endpoint := client.BaseURL() + "/api/kea/" + family + "/get"
			resp, err := client.HTTPClient().Get(endpoint)
			if err != nil {
				t.Fatal(err)
			}
			bodyBytes, readErr := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
			_ = resp.Body.Close()
			if readErr != nil {
				t.Fatalf("GET %s: read response: %v", endpoint, readErr)
			}
			if err := validateKeaStatusResponse(resp.StatusCode, bodyBytes, family, iface); err != nil {
				t.Fatalf("GET %s: %v", endpoint, err)
			}
		}
		for _, endpoint := range []string{"search_peer", "search_pd_pool", "search_subnet"} {
			family := "dhcpv6"
			if endpoint == "search_subnet" {
				for _, searchFamily := range []string{"dhcpv4", "dhcpv6"} {
					preflightKeaSearch(t, client.HTTPClient(), client.BaseURL()+"/api/kea/"+searchFamily+"/"+endpoint)
				}
				continue
			}
			preflightKeaSearch(t, client.HTTPClient(), client.BaseURL()+"/api/kea/"+family+"/"+endpoint)
		}
	}
}

func validateKeaStatusResponse(status int, bodyBytes []byte, family, iface string) error {
	if status < 200 || status >= 300 {
		return fmt.Errorf("returned HTTP %d: %s", status, boundedKeaDiagnostic(bodyBytes, 4096))
	}
	var body map[string]any
	decoder := json.NewDecoder(bytes.NewReader(bodyBytes))
	decoder.UseNumber()
	if err := decoder.Decode(&body); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	root, ok := body[family].(map[string]any)
	if !ok {
		return fmt.Errorf("response omitted object %q", family)
	}
	general, ok := root["general"].(map[string]any)
	if !ok {
		return fmt.Errorf("response omitted %s.general", family)
	}
	ha, ok := root["ha"].(map[string]any)
	if !ok {
		return fmt.Errorf("response omitted %s.ha", family)
	}
	generalEnabled, ok := general["enabled"]
	if !ok {
		return fmt.Errorf("response omitted %s.general.enabled", family)
	}
	haEnabled, ok := ha["enabled"]
	if !ok {
		return fmt.Errorf("response omitted %s.ha.enabled", family)
	}
	generalFlag, generalErr := exactBinaryFlag(generalEnabled)
	haFlag, haErr := exactBinaryFlag(haEnabled)
	if generalErr != nil || haErr != nil || generalFlag != "0" || haFlag != "0" {
		return fmt.Errorf("%s and its HA service must remain disabled", family)
	}
	if family != "dhcpv6" {
		return nil
	}
	interfaces, ok := general["interfaces"].(map[string]any)
	if !ok {
		return fmt.Errorf("response omitted dhcpv6.general.interfaces selected map")
	}
	entry, ok := interfaces[iface].(map[string]any)
	if !ok {
		return fmt.Errorf("response does not contain test interface %q", iface)
	}
	selected, ok := entry["selected"]
	selectedFlag, selectedErr := exactBinaryFlag(selected)
	if !ok || selectedErr != nil || selectedFlag != "1" {
		return fmt.Errorf("interface %q is not selected", iface)
	}
	return nil
}

func preflightKeaSearch(t *testing.T, client *http.Client, endpoint string) {
	t.Helper()
	if _, err := keaSearchTotal(client, endpoint, "story31b-status-preflight"); err != nil {
		t.Fatalf("Kea search preflight %s: %v", endpoint, err)
	}
}

func testAccKeaParityConfig(suffix, iface, role string) string {
	segment := keaTestSegment(suffix)
	pools := fmt.Sprintf("198.18.%d.100-198.18.%d.150", segment, segment)
	dns4, domainName, domainSearch4 := `["192.0.2.53", "192.0.2.54"]`, `"story31b.example.test"`, `["story31b.example.test", "services.example.test"]`
	routers, routes, ntp, times := fmt.Sprintf(`["198.18.%d.1"]`, segment), fmt.Sprintf(`[{ destination = "198.51.100.1", router = "198.18.%d.1" }, { destination = "203.0.113.1", router = "198.18.%d.1" }]`, segment, segment), `["192.0.2.20"]`, `["192.0.2.21"]`
	nextServer, tftpServer, bootfile := fmt.Sprintf("198.18.%d.22", segment), "tftp.example.test", "bootx64.efi"
	dns6, domainSearch6, pdPrefix := `["2001:db8::53", "2001:db8::54"]`, `["story31b.example.test", "services.example.test"]`, fmt.Sprintf("fd31:%x::/48", segment)
	if role == "primary" {
		pools, dns4, domainName, domainSearch4, routers, routes, ntp, times = "", `[]`, `""`, `[]`, `[]`, `[]`, `[]`, `[]`
		nextServer, tftpServer, bootfile, dns6, domainSearch6 = "", "", "", `[]`, `[]`
	}
	return fmt.Sprintf(`
resource "opnsense_kea_dhcpv4_subnet" "parity" {
  subnet        = "198.18.%[1]d.0/24"
  pools         = %[5]q
  auto_collect  = false
  dns_servers   = %[6]s
  domain_name   = %[7]s
  domain_search = %[8]s
  routers       = %[9]s
  static_routes = %[10]s
  ntp_servers   = %[11]s
  time_servers  = %[12]s
  next_server   = %[13]q
  tftp_server   = %[14]q
  tftp_bootfile = %[15]q
  description   = "Story31b-%[2]s-v4"
}

resource "opnsense_kea_dhcpv6_subnet" "parity" {
  subnet        = "fd31:%[1]x::/48"
  interface     = %[3]q
  dns_servers   = %[16]s
  domain_search = %[17]s
  description   = "Story31b-%[2]s-v6"
}

resource "opnsense_kea_dhcpv6_peer" "parity" {
  name = "story31b-%[2]s"
  role = %[4]q
  url  = "http://[2001:db8::31]:8000/"
}

resource "opnsense_kea_dhcpv6_pd_pool" "parity" {
  subnet_id        = opnsense_kea_dhcpv6_subnet.parity.id
  prefix           = %[18]q
  prefix_len       = 48
  delegated_len    = 64
  description      = "Story31b-%[2]s-pd"
}

data "opnsense_kea_dhcpv4_subnet" "parity" { id = opnsense_kea_dhcpv4_subnet.parity.id }
data "opnsense_kea_dhcpv6_subnet" "parity" { id = opnsense_kea_dhcpv6_subnet.parity.id }
data "opnsense_kea_dhcpv6_peer" "parity" { id = opnsense_kea_dhcpv6_peer.parity.id }
data "opnsense_kea_dhcpv6_pd_pool" "parity" { id = opnsense_kea_dhcpv6_pd_pool.parity.id }
`, segment, suffix, iface, role, pools, dns4, domainName, domainSearch4, routers, routes, ntp, times, nextServer, tftpServer, bootfile, dns6, domainSearch6, pdPrefix)
}

func mutateKeaV4Description(t *testing.T, suffix string) resource.TestCheckFunc {
	return func(state *terraform.State) error {
		entry := state.RootModule().Resources["opnsense_kea_dhcpv4_subnet.parity"]
		if entry == nil {
			return fmt.Errorf("DHCPv4 subnet missing from state")
		}
		client := acctest.TestClient(t)
		endpoint := client.BaseURL() + "/api/kea/dhcpv4/get_subnet/" + entry.Primary.ID
		get, err := keaDiagnosticHTTPClient(client.HTTPClient()).Get(endpoint)
		if err != nil {
			return fmt.Errorf("GET %s: %w", endpoint, err)
		}
		body, readErr := io.ReadAll(io.LimitReader(get.Body, 64*1024))
		_ = get.Body.Close()
		if readErr != nil {
			return fmt.Errorf("GET %s: read response: %w", endpoint, readErr)
		}
		if get.StatusCode < 200 || get.StatusCode >= 300 {
			return fmt.Errorf("GET %s returned HTTP %d: %s", endpoint, get.StatusCode, boundedKeaDiagnostic(body, 4096))
		}
		payload, err := encodeKeaV4SubnetMutation(body, "out-of-band-Story31b-"+suffix)
		if err != nil {
			return fmt.Errorf("encode DHCPv4 subnet mutation: %w", err)
		}
		if err := postKeaMutation(client.HTTPClient(), client.BaseURL()+"/api/kea/dhcpv4/set_subnet/"+entry.Primary.ID, payload); err != nil {
			return err
		}
		return postKeaReconfigure(client.HTTPClient(), client.BaseURL()+"/api/kea/service/reconfigure")
	}
}

type keaV4SubnetReadBack struct {
	Subnet        string               `json:"subnet"`
	Allocator     opnsense.SelectedMap `json:"allocator"`
	Pools         string               `json:"pools"`
	MatchClientID string               `json:"match-client-id"`
	PingCheck     string               `json:"ping_check"`
	AutoCollect   string               `json:"option_data_autocollect"`
	NextServer    string               `json:"next_server"`
	OptionData    struct {
		DNSServers   opnsense.CSVSelectedMapList `json:"domain_name_servers"`
		DomainName   string                      `json:"domain_name"`
		DomainSearch opnsense.CSVSelectedMapList `json:"domain_search"`
		Routers      opnsense.CSVSelectedMapList `json:"routers"`
		StaticRoutes string                      `json:"static_routes"`
		NTPServers   opnsense.CSVSelectedMapList `json:"ntp_servers"`
		TimeServers  opnsense.CSVSelectedMapList `json:"time_servers"`
		TFTPServer   string                      `json:"tftp_server_name"`
		TFTPBootfile string                      `json:"boot_file_name"`
	} `json:"option_data"`
	Description string `json:"description"`
}

type keaV4SubnetMutationRequest struct {
	Subnet        string `json:"subnet"`
	Allocator     string `json:"allocator,omitempty"`
	Pools         string `json:"pools,omitempty"`
	MatchClientID string `json:"match-client-id"`
	PingCheck     string `json:"ping_check"`
	AutoCollect   string `json:"option_data_autocollect"`
	NextServer    string `json:"next_server,omitempty"`
	OptionData    struct {
		DNSServers   string `json:"domain_name_servers,omitempty"`
		DomainName   string `json:"domain_name,omitempty"`
		DomainSearch string `json:"domain_search,omitempty"`
		Routers      string `json:"routers,omitempty"`
		StaticRoutes string `json:"static_routes,omitempty"`
		NTPServers   string `json:"ntp_servers,omitempty"`
		TimeServers  string `json:"time_servers,omitempty"`
		TFTPServer   string `json:"tftp_server_name,omitempty"`
		TFTPBootfile string `json:"boot_file_name,omitempty"`
	} `json:"option_data"`
	Description string `json:"description,omitempty"`
}

func encodeKeaV4SubnetMutation(body []byte, description string) ([]byte, error) {
	var readBack struct {
		Subnet keaV4SubnetReadBack `json:"subnet4"`
	}
	if err := json.Unmarshal(body, &readBack); err != nil {
		return nil, fmt.Errorf("decode get_subnet response: %w", err)
	}
	if readBack.Subnet.Subnet == "" {
		return nil, fmt.Errorf("decode get_subnet response: subnet4.subnet is empty")
	}
	request := keaV4SubnetMutationRequest{
		Subnet: readBack.Subnet.Subnet, Allocator: string(readBack.Subnet.Allocator), Pools: readBack.Subnet.Pools,
		MatchClientID: readBack.Subnet.MatchClientID, PingCheck: readBack.Subnet.PingCheck,
		AutoCollect: readBack.Subnet.AutoCollect, NextServer: readBack.Subnet.NextServer, Description: description,
	}
	request.OptionData.DNSServers = strings.Join(readBack.Subnet.OptionData.DNSServers, ",")
	request.OptionData.DomainName = readBack.Subnet.OptionData.DomainName
	request.OptionData.DomainSearch = strings.Join(readBack.Subnet.OptionData.DomainSearch, ",")
	request.OptionData.Routers = strings.Join(readBack.Subnet.OptionData.Routers, ",")
	request.OptionData.StaticRoutes = readBack.Subnet.OptionData.StaticRoutes
	request.OptionData.NTPServers = strings.Join(readBack.Subnet.OptionData.NTPServers, ",")
	request.OptionData.TimeServers = strings.Join(readBack.Subnet.OptionData.TimeServers, ",")
	request.OptionData.TFTPServer = readBack.Subnet.OptionData.TFTPServer
	request.OptionData.TFTPBootfile = readBack.Subnet.OptionData.TFTPBootfile
	return json.Marshal(struct {
		Subnet keaV4SubnetMutationRequest `json:"subnet4"`
	}{Subnet: request})
}

func mutateKeaV6PeerRole(t *testing.T) resource.TestCheckFunc {
	return mutateKeaParityItem(t, "opnsense_kea_dhcpv6_peer.parity", "peer", "/api/kea/dhcpv6/get_peer/", "/api/kea/dhcpv6/set_peer/", func(item map[string]any) {
		item["role"] = "primary"
	})
}

func mutateKeaV6PDDelegatedLength(t *testing.T) resource.TestCheckFunc {
	return mutateKeaParityItem(t, "opnsense_kea_dhcpv6_pd_pool.parity", "pd_pool", "/api/kea/dhcpv6/get_pd_pool/", "/api/kea/dhcpv6/set_pd_pool/", func(item map[string]any) {
		item["delegated_len"] = "80"
	})
}

func mutateKeaParityItem(t *testing.T, address, monad, getEndpoint, setEndpoint string, mutate func(map[string]any)) resource.TestCheckFunc {
	return func(state *terraform.State) error {
		entry := state.RootModule().Resources[address]
		if entry == nil {
			return fmt.Errorf("%s missing from state", address)
		}
		client := acctest.TestClient(t)
		get, err := client.HTTPClient().Get(client.BaseURL() + getEndpoint + entry.Primary.ID)
		if err != nil {
			return err
		}
		defer func() { _ = get.Body.Close() }()
		if get.StatusCode < 200 || get.StatusCode >= 300 {
			body, _ := io.ReadAll(io.LimitReader(get.Body, 4096))
			return fmt.Errorf("GET %s returned HTTP %d: %s", getEndpoint, get.StatusCode, strings.TrimSpace(string(body)))
		}
		var wrapper map[string]map[string]any
		if err := json.NewDecoder(get.Body).Decode(&wrapper); err != nil {
			return err
		}
		item, ok := wrapper[monad]
		if !ok {
			return fmt.Errorf("GET %s response omitted %q", getEndpoint, monad)
		}
		mutate(item)
		normalizeKeaSelectors(item)
		payload, err := json.Marshal(map[string]any{monad: item})
		if err != nil {
			return err
		}
		if err := postKeaMutation(client.HTTPClient(), client.BaseURL()+setEndpoint+entry.Primary.ID, payload); err != nil {
			return err
		}
		return postKeaReconfigure(client.HTTPClient(), client.BaseURL()+"/api/kea/service/reconfigure")
	}
}

func normalizeKeaSelectors(value map[string]any) {
	for key, raw := range value {
		nested, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		selected := make([]string, 0)
		selector := true
		for option, optionRaw := range nested {
			entry, ok := optionRaw.(map[string]any)
			if !ok {
				selector = false
				break
			}
			if _, ok := entry["selected"]; !ok {
				selector = false
				break
			}
			if scalar(entry["selected"]) == "1" {
				selected = append(selected, option)
			}
		}
		if selector {
			value[key] = strings.Join(selected, ",")
		} else {
			normalizeKeaSelectors(nested)
		}
	}
}

func postKeaMutation(client *http.Client, endpoint string, payload []byte) error {
	return postKeaAffirmative(client, endpoint, payload, "result", "saved")
}

func postKeaReconfigure(client *http.Client, endpoint string) error {
	return postKeaAffirmative(client, endpoint, []byte(`{}`), "status", "ok")
}

func postKeaAffirmative(client *http.Client, endpoint string, payload []byte, field, expected string) error {
	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := keaDiagnosticHTTPClient(client).Do(req)
	if err != nil {
		return fmt.Errorf("POST %s failed: %w; request: %s", endpoint, err, boundedKeaDiagnostic(payload, 4096))
	}
	defer func() { _ = resp.Body.Close() }()
	body, readErr := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	if readErr != nil {
		return fmt.Errorf("POST %s: read response: %w", endpoint, readErr)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("POST %s returned HTTP %d: %s; request: %s", endpoint, resp.StatusCode, boundedKeaDiagnostic(body, 4096), boundedKeaDiagnostic(payload, 4096))
	}
	var result map[string]json.RawMessage
	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("POST %s returned malformed success body: %w", endpoint, err)
	}
	var actual string
	if raw, ok := result[field]; ok {
		if err := json.Unmarshal(raw, &actual); err != nil {
			return fmt.Errorf("POST %s returned malformed %s: %w", endpoint, field, err)
		}
	}
	if actual != expected {
		return fmt.Errorf("POST %s did not affirm success: %s=%q, want %q; response: %s", endpoint, field, actual, expected, boundedKeaDiagnostic(body, 4096))
	}
	return nil
}

func keaDiagnosticHTTPClient(client *http.Client) *http.Client {
	retrying, ok := client.Transport.(*retryablehttp.RoundTripper)
	if !ok || retrying.Client == nil || retrying.Client.HTTPClient == nil {
		return client
	}
	direct := *retrying.Client.HTTPClient
	return &direct
}

func boundedKeaDiagnostic(value []byte, limit int) string {
	trimmed := strings.TrimSpace(string(value))
	if len(trimmed) <= limit {
		return trimmed
	}
	return trimmed[:limit] + "...[truncated]"
}

func checkKeaParityCleanup(t *testing.T, suffix string) resource.TestCheckFunc {
	return func(*terraform.State) error {
		keaParityStatusPreCheck(t)()
		client := acctest.TestClient(t)
		segment := keaTestSegment(suffix)
		for _, search := range []struct{ endpoint, query string }{
			{"/api/kea/dhcpv6/search_pd_pool", "Story31b-" + suffix},
			{"/api/kea/dhcpv6/search_subnet", fmt.Sprintf("fd31:%x::/48", segment)},
			{"/api/kea/dhcpv6/search_peer", "story31b-" + suffix},
			{"/api/kea/dhcpv4/search_subnet", fmt.Sprintf("198.18.%d.0/24", segment)},
		} {
			total, identifiers, err := keaSearchMatches(client.HTTPClient(), client.BaseURL()+search.endpoint, search.query)
			if err != nil {
				return fmt.Errorf("cleanup search endpoint=%s query=%q: %w", search.endpoint, search.query, err)
			}
			if total != 0 {
				if len(identifiers) < total {
					return fmt.Errorf("cleanup search endpoint=%s query=%q found %d matches but returned only %d identifiable rows; available identifiers=%v", search.endpoint, search.query, total, len(identifiers), identifiers)
				}
				return fmt.Errorf("cleanup search endpoint=%s query=%q found %d matches; remaining identifiers=%v", search.endpoint, search.query, total, identifiers)
			}
			t.Logf("Kea parity cleanup endpoint=%s query=%q total=0", search.endpoint, search.query)
		}
		return nil
	}
}

func keaSearchTotal(client *http.Client, endpoint, query string) (int, error) {
	total, _, err := keaSearchMatches(client, endpoint, query)
	return total, err
}

func keaSearchMatches(client *http.Client, endpoint, query string) (int, []string, error) {
	requestBody, err := json.Marshal(map[string]any{"current": 1, "rowCount": 500, "searchPhrase": query})
	if err != nil {
		return 0, nil, err
	}
	resp, err := client.Post(endpoint, "application/json", bytes.NewReader(requestBody))
	if err != nil {
		return 0, nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	if err != nil {
		return 0, nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return 0, nil, fmt.Errorf("POST %s returned HTTP %d: %s", endpoint, resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var result struct {
		Total json.RawMessage  `json:"total"`
		Rows  []map[string]any `json:"rows"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return 0, nil, fmt.Errorf("decode response: %w", err)
	}
	if len(result.Total) == 0 {
		return 0, nil, fmt.Errorf("decode response: missing total")
	}
	var totalText string
	if result.Total[0] == '"' {
		if err := json.Unmarshal(result.Total, &totalText); err != nil {
			return 0, nil, fmt.Errorf("decode total: %w", err)
		}
	} else {
		totalText = string(result.Total)
	}
	total, err := strconv.Atoi(totalText)
	if err != nil || total < 0 {
		return 0, nil, fmt.Errorf("decode total: invalid value %q", totalText)
	}
	if total == 0 && len(result.Rows) != 0 {
		return 0, nil, fmt.Errorf("decode response: total is zero but rows contains %d entries", len(result.Rows))
	}
	identifiers := make([]string, 0, len(result.Rows))
	for index, row := range result.Rows {
		identifier := ""
		for _, key := range []string{"uuid", "id"} {
			if value, ok := row[key]; ok && scalar(value) != "" {
				identifier = scalar(value)
				break
			}
		}
		if identifier == "" {
			encoded, _ := json.Marshal(row)
			identifier = fmt.Sprintf("row[%d]=%s", index, boundedKeaDiagnostic(encoded, 1024))
		}
		identifiers = append(identifiers, identifier)
	}
	return total, identifiers, nil
}

func TestKeaSearchTotalHandlesOPNsenseShape(t *testing.T) {
	for _, response := range []string{`{"total":2,"rows":[]}`, `{"total":"2","rows":[]}`} {
		t.Run(response, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				_, _ = io.WriteString(w, response)
			}))
			t.Cleanup(server.Close)
			total, err := keaSearchTotal(server.Client(), server.URL, "marker")
			if err != nil || total != 2 {
				t.Fatalf("total=%d err=%v", total, err)
			}
		})
	}
}

func TestKeaSearchTotalRejectsHTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "search unavailable", http.StatusServiceUnavailable)
	}))
	t.Cleanup(server.Close)
	if _, err := keaSearchTotal(server.Client(), server.URL, "marker"); err == nil || !strings.Contains(err.Error(), "HTTP 503") {
		t.Fatalf("expected HTTP status error, got %v", err)
	}
}

func keaTestSegment(suffix string) int {
	value := 0
	for _, character := range suffix {
		value = (value*33 + int(character)) % 200
	}
	return value + 20
}

func readKeaFirmwareVersion(t *testing.T, client *http.Client, endpoint string) string {
	t.Helper()
	resp, err := client.Get(endpoint)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	var body struct {
		ProductVersion string `json:"product_version"`
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		t.Fatalf("firmware preflight returned HTTP %d", resp.StatusCode)
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	return body.ProductVersion
}

func keaVersionAtLeast(version string, major, minor, patch int) bool {
	ok, err := keaTestVersionAtLeast(version, fmt.Sprintf("%d.%d.%d", major, minor, patch))
	return err == nil && ok
}

func keaTestVersionAtLeast(actual, minimum string) (bool, error) {
	parse := func(value string) ([3]uint64, error) {
		var parsed [3]uint64
		fields := strings.FieldsFunc(value, func(r rune) bool { return r == '_' || r == '-' })
		if len(fields) == 0 {
			return parsed, fmt.Errorf("malformed version %q", value)
		}
		parts := strings.Split(fields[0], ".")
		if len(parts) < 2 || len(parts) > 3 {
			return parsed, fmt.Errorf("malformed version %q", value)
		}
		for i, part := range parts {
			n, err := strconv.ParseUint(part, 10, 63)
			if err != nil {
				return parsed, err
			}
			parsed[i] = n
		}
		return parsed, nil
	}
	a, err := parse(actual)
	if err != nil {
		return false, err
	}
	m, err := parse(minimum)
	if err != nil {
		return false, err
	}
	for i := range a {
		if a[i] != m[i] {
			return a[i] > m[i], nil
		}
	}
	return true, nil
}

func exactBinaryFlag(value any) (string, error) {
	var text string
	switch typed := value.(type) {
	case string:
		text = typed
	case json.Number:
		text = typed.String()
	default:
		return "", fmt.Errorf("expected exact binary flag, got %T", value)
	}
	if text != "0" && text != "1" {
		return "", fmt.Errorf("expected exact binary flag 0 or 1, got %q", text)
	}
	return text, nil
}

func scalar(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case json.Number:
		return typed.String()
	case float64:
		return strconv.Itoa(int(typed))
	default:
		return fmt.Sprint(typed)
	}
}
