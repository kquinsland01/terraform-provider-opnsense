// Copyright (c) Matthew Mellor
// SPDX-License-Identifier: MPL-2.0

package kea

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/matthew-on-git/terraform-provider-opnsense/pkg/opnsense"
)

func TestVersionAtLeastStrictAndOverflowSafe(t *testing.T) {
	for _, version := range []string{"26.7.7", "26.7.8_1", "27.1"} {
		ok, err := versionAtLeast(version, "26.7.7")
		if err != nil || !ok {
			t.Fatalf("version %q: ok=%t err=%v", version, ok, err)
		}
	}
	for _, version := range []string{"26.7", "bad", "999999999999999999999999.1"} {
		ok, err := versionAtLeast(version, "26.7.7")
		if err == nil && ok {
			t.Fatalf("unsupported/malformed version accepted: %q", version)
		}
	}
}

func TestRequireKeaVersionUsesFirmwareStatusNestedVersion(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if request.URL.Path != keaFirmwareStatusEndpoint {
			t.Fatalf("compatibility used unexpected endpoint: %s", request.URL.Path)
		}
		_, _ = w.Write([]byte(`{"product":{"product_version":"26.7.7"}}`))
	}))
	defer server.Close()
	client, err := opnsense.NewClient(opnsense.ClientConfig{BaseURL: server.URL, APIKey: "key", APISecret: "secret", RetryMax: 1})
	if err != nil {
		t.Fatal(err)
	}
	if err := requireKeaVersion(context.Background(), client, "26.7.7"); err != nil {
		t.Fatalf("expected compatible firmware, got %v", err)
	}
}

func TestRequireKeaVersionRejectsInvalidFirmwareStatus(t *testing.T) {
	tests := map[string]struct {
		status int
		body   string
		want   string
	}{
		"old version":        {status: http.StatusOK, body: `{"product":{"product_version":"26.7.6"}}`, want: "requires"},
		"malformed version":  {status: http.StatusOK, body: `{"product":{"product_version":"malformed"}}`, want: "malformed OPNsense version"},
		"missing version":    {status: http.StatusOK, body: `{"product":{}}`, want: "missing product.product_version"},
		"missing product":    {status: http.StatusOK, body: `{}`, want: "missing product.product_version"},
		"wrong version type": {status: http.StatusOK, body: `{"product":{"product_version":2677}}`, want: "decode firmware version"},
		"non-2xx":            {status: http.StatusForbidden, body: `{"error":"denied"}`, want: "HTTP 403"},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
				if request.URL.Path != keaFirmwareStatusEndpoint {
					t.Fatalf("compatibility used unexpected endpoint: %s", request.URL.Path)
				}
				w.WriteHeader(test.status)
				_, _ = w.Write([]byte(test.body))
			}))
			defer server.Close()
			client, err := opnsense.NewClient(opnsense.ClientConfig{BaseURL: server.URL, APIKey: "key", APISecret: "secret", RetryMax: 1})
			if err != nil {
				t.Fatal(err)
			}
			err = requireKeaVersion(context.Background(), client, "26.7.7")
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want substring %q", err, test.want)
			}
		})
	}
}

func TestRequireKeaVersionUsesAndReleasesReadSemaphore(t *testing.T) {
	var requests atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		_, _ = w.Write([]byte(`{"product":{"product_version":"26.7.7"}}`))
	}))
	defer server.Close()
	client, err := opnsense.NewClient(opnsense.ClientConfig{BaseURL: server.URL, APIKey: "key", APISecret: "secret", RetryMax: 1, MaxReadConcurrency: 1})
	if err != nil {
		t.Fatal(err)
	}
	if err := client.AcquireRead(context.Background()); err != nil {
		t.Fatal(err)
	}
	blockedContext, cancel := context.WithTimeout(context.Background(), 25*time.Millisecond)
	defer cancel()
	if err := requireKeaVersion(blockedContext, client, "26.7.7"); err == nil || !strings.Contains(err.Error(), "acquire read slot") {
		t.Fatalf("compatibility probe did not respect occupied semaphore: %v", err)
	}
	if requests.Load() != 0 {
		t.Fatalf("compatibility made %d requests without a read slot", requests.Load())
	}
	client.ReleaseRead()
	if err := requireKeaVersion(context.Background(), client, "26.7.7"); err != nil {
		t.Fatal(err)
	}
	if err := client.AcquireRead(context.Background()); err != nil {
		t.Fatalf("compatibility did not release read slot: %v", err)
	}
	client.ReleaseRead()
}
