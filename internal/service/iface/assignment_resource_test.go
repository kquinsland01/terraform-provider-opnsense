// Copyright (c) Matthew Mellor
// SPDX-License-Identifier: MPL-2.0

package iface_test

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
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"

	"github.com/matthew-on-git/terraform-provider-opnsense/internal/acctest"
	"github.com/matthew-on-git/terraform-provider-opnsense/pkg/opnsense"
)

func TestAccInterfaceAssignment(t *testing.T) {
	name := os.Getenv("OPNSENSE_INTERFACE_ASSIGNMENT")
	address := os.Getenv("OPNSENSE_INTERFACE_ASSIGNMENT_IPV4_ADDRESS")
	updatedAddress := os.Getenv("OPNSENSE_INTERFACE_ASSIGNMENT_UPDATED_IPV4_ADDRESS")
	prefix := os.Getenv("OPNSENSE_INTERFACE_ASSIGNMENT_IPV4_PREFIX_LENGTH")
	if name == "" || address == "" || updatedAddress == "" || prefix == "" {
		t.Skip("set OPNSENSE_INTERFACE_ASSIGNMENT and both disposable IPv4 addresses/prefix length to run this destructive acceptance test")
	}
	if !regexp.MustCompile(`^opt[0-9]+$`).MatchString(name) {
		t.Fatalf("OPNSENSE_INTERFACE_ASSIGNMENT must be a disposable optN assignment; refusing %q", name)
	}
	if address == updatedAddress {
		t.Fatal("initial and updated interface-assignment test addresses must be different")
	}
	if _, err := strconv.ParseInt(prefix, 10, 64); err != nil {
		t.Fatalf("OPNSENSE_INTERFACE_ASSIGNMENT_IPV4_PREFIX_LENGTH must be numeric: %v", err)
	}

	var original map[string]json.RawMessage
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		PreCheck: func() {
			acctest.PreCheck(t)
			original = captureAssignment(t, name)
		},
		CheckDestroy: restoreAssignment(t, name, &original),
		Steps: []resource.TestStep{
			{
				Config: testAccInterfaceAssignmentConfig(name, address, prefix),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("opnsense_interface_assignment.test", "id", name),
					resource.TestCheckResourceAttr("opnsense_interface_assignment.test", "name", name),
					resource.TestCheckResourceAttr("opnsense_interface_assignment.test", "enabled", "true"),
					resource.TestCheckResourceAttr("opnsense_interface_assignment.test", "ipv4_mode", "staticv4"),
					resource.TestCheckResourceAttr("opnsense_interface_assignment.test", "ipv4_address", address),
					resource.TestCheckResourceAttr("opnsense_interface_assignment.test", "ipv4_prefix_length", prefix),
				),
			},
			{Config: testAccInterfaceAssignmentConfig(name, address, prefix), PlanOnly: true, ExpectNonEmptyPlan: false},
			{ResourceName: "opnsense_interface_assignment.test", ImportState: true, ImportStateId: name, ImportStateVerify: true},
			{
				PreConfig: func() { mutateAssignmentField(t, name, "enable", "0") },
				Config:    testAccInterfaceAssignmentConfig(name, address, prefix), PlanOnly: true, ExpectNonEmptyPlan: true,
			},
			{
				Config: testAccInterfaceAssignmentConfig(name, updatedAddress, prefix),
				Check:  resource.TestCheckResourceAttr("opnsense_interface_assignment.test", "ipv4_address", updatedAddress),
			},
		},
	})
}

func captureAssignment(t *testing.T, name string) map[string]json.RawMessage {
	t.Helper()
	client := acctest.TestClient(t)
	var firmware struct {
		ProductVersion string `json:"product_version"`
	}
	getJSON(t, client, "/api/core/firmware/info", &firmware)
	if !versionAtLeastForAcceptance(firmware.ProductVersion, 26, 7, 6) {
		t.Fatalf("interface assignment acceptance requires OPNsense 26.7.6+, got %q", firmware.ProductVersion)
	}
	var envelope struct {
		Interface map[string]json.RawMessage `json:"interface"`
	}
	getJSON(t, client, "/api/interfaces/assignment/get_item/"+name, &envelope)
	if len(envelope.Interface) == 0 {
		t.Fatalf("disposable interface assignment %q does not exist", name)
	}
	for _, field := range []string{"type4", "ipaddr", "type6", "ipaddrv6", "gateway", "gatewayv6", "dhcphostname", "dhcprejectfrom", "dhcphonourmtu", "dhcpvlanprio", "dhcp_send_options", "dhcp_request_options", "dhcp6vlanprio", "dhcp6_request_dns", "dhcp6-information-only", "dhcp6_rapid_commit", "dhcp6_send_options", "dhcp6_request_options"} {
		if _, ok := envelope.Interface[field]; !ok {
			t.Fatalf("assignment API is incomplete: missing %s", field)
		}
	}
	return envelope.Interface
}

func restoreAssignment(t *testing.T, name string, original *map[string]json.RawMessage) func(*terraform.State) error {
	t.Helper()
	return func(_ *terraform.State) error {
		if original == nil || len(*original) == 0 {
			return fmt.Errorf("original assignment state was not captured")
		}
		payload, err := normalizeAssignmentSnapshot(*original)
		if err != nil {
			return err
		}
		client := acctest.TestClient(t)
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := setAssignment(ctx, client, name, payload); err != nil {
			return fmt.Errorf("restore assignment: %w", err)
		}
		restored, err := fetchAssignment(ctx, client, name)
		if err != nil {
			return fmt.Errorf("verify restored assignment: %w", err)
		}
		normalizedRestored, err := normalizeAssignmentSnapshot(restored)
		if err != nil {
			return fmt.Errorf("normalize restored assignment: %w", err)
		}
		if !reflect.DeepEqual(normalizedRestored, payload) {
			return fmt.Errorf("restored assignment does not match captured snapshot: got %#v, want %#v", normalizedRestored, payload)
		}
		return nil
	}
}

func mutateAssignmentField(t *testing.T, name, field string, value any) {
	t.Helper()
	client := acctest.TestClient(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	current, err := fetchAssignment(ctx, client, name)
	if err != nil {
		t.Fatalf("capture assignment before out-of-band mutation: %v", err)
	}
	payload, err := normalizeAssignmentSnapshot(current)
	if err != nil {
		t.Fatalf("normalize assignment before out-of-band mutation: %v", err)
	}
	payload[field] = value
	if err := setAssignment(ctx, client, name, payload); err != nil {
		t.Fatalf("out-of-band assignment mutation: %v", err)
	}
}

func setAssignment(ctx context.Context, client *opnsense.Client, name string, payload map[string]any) error {
	body, err := json.Marshal(map[string]any{"interface": payload})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, client.BaseURL()+"/api/interfaces/assignment/set_item/"+name, bytes.NewReader(body)) //nolint:gosec // Acceptance base URL is provider-configured and name is restricted to optN.
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.HTTPClient().Do(req) //nolint:gosec // Request URL is constructed from validated acceptance inputs.
	if err != nil {
		return err
	}
	responseBody, readErr := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if readErr != nil {
		return readErr
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("set_item returned HTTP %d", resp.StatusCode)
	}
	if _, err := opnsense.ParseMutationResponse(responseBody); err != nil {
		return err
	}
	return opnsense.Reconfigure(ctx, client, opnsense.ReqOpts{ReconfigureEndpoint: "/api/interfaces/assignment/reconfigure"})
}

func fetchAssignment(ctx context.Context, client *opnsense.Client, name string) (map[string]json.RawMessage, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, client.BaseURL()+"/api/interfaces/assignment/get_item/"+name, nil) //nolint:gosec // Acceptance base URL is provider-configured and name is restricted to optN.
	if err != nil {
		return nil, err
	}
	resp, err := client.HTTPClient().Do(req) //nolint:gosec // Request URL is constructed from validated acceptance inputs.
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("get_item returned HTTP %d", resp.StatusCode)
	}
	var envelope struct {
		Interface map[string]json.RawMessage `json:"interface"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		return nil, err
	}
	return envelope.Interface, nil
}

func normalizeAssignmentSnapshot(snapshot map[string]json.RawMessage) (map[string]any, error) {
	normalized := make(map[string]any, len(snapshot))
	for key, raw := range snapshot {
		switch key {
		case "identifier", "pending_action", "icon", "optgroup":
			continue
		}
		value, err := normalizeAcceptanceValue(raw)
		if err != nil {
			return nil, fmt.Errorf("normalize assignment field %s: %w", key, err)
		}
		normalized[key] = value
	}
	return normalized, nil
}

func getJSON(t *testing.T, client *opnsense.Client, endpoint string, target any) {
	t.Helper()
	resp, err := client.HTTPClient().Get(client.BaseURL() + endpoint) //nolint:gosec // Endpoint is selected only by this acceptance test.
	if err != nil {
		t.Fatalf("GET %s: %v", endpoint, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET %s returned HTTP %d", endpoint, resp.StatusCode)
	}
	if err := json.NewDecoder(resp.Body).Decode(target); err != nil {
		t.Fatalf("decode %s: %v", endpoint, err)
	}
}

func normalizeAcceptanceValue(raw json.RawMessage) (any, error) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return "", nil
	}
	if trimmed[0] == '[' {
		var choices []map[string]json.RawMessage
		if err := json.Unmarshal(trimmed, &choices); err == nil && len(choices) > 0 {
			selected := make([]string, 0)
			isChoiceList := true
			for index, choice := range choices {
				rawSelected, ok := choice["selected"]
				if !ok {
					isChoiceList = false
					break
				}
				var selectedValue any
				if err := json.Unmarshal(rawSelected, &selectedValue); err != nil {
					return nil, err
				}
				if fmt.Sprint(selectedValue) == "1" {
					selected = append(selected, strconv.Itoa(index))
				}
			}
			if isChoiceList {
				return strings.Join(selected, ","), nil
			}
		}
	}
	if trimmed[0] != '{' {
		var value any
		if err := json.Unmarshal(trimmed, &value); err != nil {
			return nil, err
		}
		if list, ok := value.([]any); ok && len(list) == 0 {
			return "", nil
		}
		return value, nil
	}
	var rawChoices map[string]json.RawMessage
	if err := json.Unmarshal(trimmed, &rawChoices); err == nil && len(rawChoices) > 0 {
		selected := make([]string, 0)
		isChoiceMap := true
		for key, rawChoice := range rawChoices {
			var fields map[string]json.RawMessage
			if err := json.Unmarshal(rawChoice, &fields); err != nil {
				isChoiceMap = false
				break
			}
			rawSelected, ok := fields["selected"]
			if !ok {
				isChoiceMap = false
				break
			}
			var selectedValue any
			if err := json.Unmarshal(rawSelected, &selectedValue); err != nil {
				return nil, err
			}
			if fmt.Sprint(selectedValue) == "1" {
				selected = append(selected, key)
			}
		}
		if isChoiceMap {
			sort.Strings(selected)
			return strings.Join(selected, ","), nil
		}
	}
	var value any
	return value, json.Unmarshal(trimmed, &value)
}

func TestNormalizeAcceptanceValueIndexedChoice(t *testing.T) {
	t.Parallel()
	value, err := normalizeAcceptanceValue(json.RawMessage(`[{"value":"enabled","selected":1},{"value":"disabled","selected":0}]`))
	if err != nil {
		t.Fatal(err)
	}
	if value != "0" {
		t.Fatalf("normalize indexed choice = %#v, want %q", value, "0")
	}
}

func versionAtLeastForAcceptance(version string, major, minor, patch int) bool {
	parts := regexp.MustCompile(`[0-9]+`).FindAllString(version, 3)
	if len(parts) < 2 {
		return false
	}
	got := []int{0, 0, 0}
	for i := range parts {
		got[i], _ = strconv.Atoi(parts[i])
	}
	want := []int{major, minor, patch}
	for i := range want {
		if got[i] != want[i] {
			return got[i] > want[i]
		}
	}
	return true
}

func testAccInterfaceAssignmentConfig(name, address, prefix string) string {
	return fmt.Sprintf(`
resource "opnsense_interface_assignment" "test" {
  name               = %[1]q
  enabled            = true
  ipv4_mode          = "staticv4"
  ipv4_address       = %[2]q
  ipv4_prefix_length = %[3]s
  ipv4_gateway       = ""
  ipv6_mode          = "none"
}
`, name, address, prefix)
}
