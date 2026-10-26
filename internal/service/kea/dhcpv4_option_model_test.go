// Copyright (c) Matthew Mellor
// SPDX-License-Identifier: MPL-2.0

package kea

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/matthew-on-git/terraform-provider-opnsense/pkg/opnsense"
)

func TestDHCPv4OptionOmittedPlanPreservesStateAndExplicitUnknown(t *testing.T) {
	modifier := optionOmittedString{initial: "hex"}
	for _, tc := range []struct {
		name                string
		config, state, want types.String
	}{
		{"create", types.StringNull(), types.StringNull(), types.StringValue("hex")},
		{"import omission", types.StringNull(), types.StringValue("fqdn"), types.StringValue("fqdn")},
		{"removed configuration", types.StringNull(), types.StringValue("string"), types.StringValue("string")},
		{"explicit unknown", types.StringUnknown(), types.StringValue("fqdn"), types.StringUnknown()},
		{"explicit clear", types.StringValue(""), types.StringValue("fqdn"), types.StringValue("")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			response := planmodifier.StringResponse{PlanValue: tc.config}
			modifier.PlanModifyString(context.Background(), planmodifier.StringRequest{ConfigValue: tc.config, StateValue: tc.state}, &response)
			if !response.PlanValue.Equal(tc.want) {
				t.Fatalf("got %s want %s", response.PlanValue, tc.want)
			}
		})
	}
	for _, tc := range []struct{ config, state, want types.Bool }{
		{types.BoolNull(), types.BoolNull(), types.BoolValue(false)},
		{types.BoolNull(), types.BoolValue(true), types.BoolValue(true)},
		{types.BoolUnknown(), types.BoolValue(true), types.BoolUnknown()},
		{types.BoolValue(false), types.BoolValue(true), types.BoolValue(false)},
	} {
		response := planmodifier.BoolResponse{PlanValue: tc.config}
		optionOmittedBool{}.PlanModifyBool(context.Background(), planmodifier.BoolRequest{ConfigValue: tc.config, StateValue: tc.state}, &response)
		if !response.PlanValue.Equal(tc.want) {
			t.Fatal("bool omitted/unknown intent lost")
		}
	}
}

func TestDHCPv4OptionSelectorSentinelsFlagsAndFiniteCatalog(t *testing.T) {
	for _, raw := range []string{`null`, `[]`, `{}`, `{"15":{}}`, `{"15":{"selected":"1"}}`, `{"15":{"selected":true}}`, `{"15":{"selected":2}}`, `{"15":{"selected":1},"66":{"selected":1}}`, `{"0":{"selected":1}}`, `{"255":{"selected":1}}`} {
		if _, _, err := optionSelection(json.RawMessage(raw), "code", false); err == nil {
			t.Fatalf("accepted malformed selector %s", raw)
		}
	}
	selected, _, err := optionSelection(json.RawMessage(`{"":{"selected":1,"value":"None"},"15":{"selected":0}}`), "match_code", false)
	if err != nil || selected != "" {
		t.Fatal("optional None sentinel rejected", err)
	}
	selected, _, err = optionSelection(json.RawMessage(`{"15":{"selected":1,"value":"label differs","optgroup":"Assigned"}}`), "code", false)
	if err != nil || selected != "15" {
		t.Fatal("selector label used instead of key", err)
	}
}

func TestDHCPv4OptionConfiguredReadRetriesAndSinglePOST(t *testing.T) {
	gets, posts := 0, 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			gets++
		} else {
			posts++
		}
		http.Error(w, "unavailable", 500)
	}))
	defer server.Close()
	client, err := opnsense.NewClient(opnsense.ClientConfig{BaseURL: server.URL, APIKey: "fixture", APISecret: "fixture", RetryMax: 2, RetryWaitMin: time.Millisecond, RetryWaitMax: time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = optionRequest(context.Background(), client, http.MethodGet, "/api/fixture", nil); err == nil {
		t.Fatal("expected unavailable read")
	}
	if _, err = optionRequest(context.Background(), client, http.MethodPost, "/api/fixture", nil); err == nil {
		t.Fatal("expected unavailable write")
	}
	if gets != 3 || posts != 1 {
		t.Fatalf("configured GET policy / single POST violated: gets=%d posts=%d", gets, posts)
	}
	// The copied requester must not modify the original retry policy.
	_, _ = optionRequest(context.Background(), client, http.MethodGet, "/api/fixture", nil)
	if gets != 6 {
		t.Fatal("shared retry configuration changed")
	}
	_, _ = optionRequest(context.Background(), client, http.MethodPost, dhcpv4OptionReqOpts.SearchEndpoint, nil)
	if posts != 4 {
		t.Fatal("read-only search POST lost configured retry policy", posts)
	}
}

func TestDHCPv4OptionMatchingPairAndEncodingOnly(t *testing.T) {
	base := optionPayload{"code": "15", "encoding": "fqdn", "data": "literal, data", "force": "0", "match_code": "", "match_encoding": "", "match_data": "", "description": "literal, description"}
	for _, tc := range []struct {
		code, encoding, data string
		valid                bool
	}{{"", "", "", true}, {"", "hex", "", true}, {"66", "string", "example.org", true}, {"66", "string", "", false}, {"", "string", "example.org", false}, {"66", "", "example.org", false}} {
		t.Run(fmt.Sprintf("%s/%s/%s", tc.code, tc.encoding, tc.data), func(t *testing.T) {
			payload := optionPayload{}
			for k, v := range base {
				payload[k] = v
			}
			payload["match_code"] = tc.code
			payload["match_encoding"] = tc.encoding
			payload["match_data"] = tc.data
			if err := validateOptionPayload(payload, true); (err == nil) != tc.valid {
				t.Fatal("incorrect matching validation", err)
			}
		})
	}
}

func TestDHCPv4OptionSnapshotStrictCompletePagination(t *testing.T) {
	for _, mode := range []string{"complete", "missing rows", "null rows", "missing metadata", "wrong count", "wrong current", "negative total", "changing total", "empty page", "duplicate UUID"} {
		t.Run(mode, func(t *testing.T) {
			pages := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				pages++
				var request struct {
					Current int `json:"current"`
				}
				if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
					t.Error(err)
				}
				row := map[string]string{"uuid": fmt.Sprintf("00000000-0000-0000-0000-%012d", pages), "description": "synthetic"}
				if mode == "duplicate UUID" {
					row["uuid"] = "00000000-0000-0000-0000-000000000001"
				}
				response := map[string]any{"rows": []any{row}, "current": request.Current, "rowCount": 1, "total": 2}
				switch mode {
				case "missing rows":
					delete(response, "rows")
				case "null rows":
					response["rows"] = nil
				case "missing metadata":
					delete(response, "rowCount")
				case "wrong count":
					response["rowCount"] = 0
				case "wrong current":
					response["current"] = 0
				case "negative total":
					response["total"] = -1
				case "changing total":
					if pages == 2 {
						response["total"] = 3
					}
				case "empty page":
					response["rows"] = []any{}
					response["rowCount"] = 0
				}
				_ = json.NewEncoder(w).Encode(response)
			}))
			defer server.Close()
			client, err := opnsense.NewClient(opnsense.ClientConfig{BaseURL: server.URL, APIKey: "fixture", APISecret: "fixture"})
			if err != nil {
				t.Fatal(err)
			}
			result, err := optionSnapshot(context.Background(), client)
			if mode == "complete" {
				if err != nil || pages != 2 || len(result) != 2 {
					t.Fatal("complete discovery lost pages", err, pages, result)
				}
			} else if err == nil {
				t.Fatal("corrupt/incomplete discovery accepted", mode, result)
			}
		})
	}
}
