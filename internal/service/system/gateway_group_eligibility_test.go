// Copyright (c) Matthew Mellor
// SPDX-License-Identifier: MPL-2.0

package system

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/matthew-on-git/terraform-provider-opnsense/pkg/opnsense"
)

func TestGatewayGroupEligibilityProductionBoundWithoutWrite(t *testing.T) {
	reads, writes := 0, 0
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writes++
			http.Error(w, "unexpected mutation", http.StatusBadRequest)
			return
		}
		reads++
		_ = json.NewEncoder(w).Encode(map[string]any{"gateway_group": map[string]any{"name": "", "descr": "", "item": []string{}, "item2": []string{}, "item3": []string{}, "item4": []string{}, "item5": []string{}, "trigger": map[string]any{"down": map[string]int{"selected": 1}}, "poolopts": map[string]any{"": map[string]int{"selected": 1}}}})
	}))
	defer api.Close()
	client := newTestClient(t, api.URL)
	now := time.Now()
	initial := now
	waits := 0
	policy := groupEligibilityPolicy{timeout: groupEligibilityTimeout, interval: groupEligibilityInterval, now: func() time.Time { return now }, wait: func(ctx context.Context, delay time.Duration) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if delay <= 0 || delay > time.Second {
			t.Fatalf("unexpected polling delay %s", delay)
		}
		waits++
		now = now.Add(delay)
		return nil
	}}
	payload, err := groupTestModel().payload(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	err = waitGatewayGroupMembersWithPolicy(context.Background(), client, payload, policy)
	if !errors.Is(err, context.DeadlineExceeded) || now.Sub(initial) != 30*time.Second || reads != 30 || waits != 30 || writes != 0 {
		t.Fatalf("bound err=%v elapsed=%s reads=%d waits=%d writes=%d", err, now.Sub(initial), reads, waits, writes)
	}
}

func TestGatewayGroupEligibilityWaitCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := waitGatewayGroupEligibility(ctx, time.Second); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation not respected: %v", err)
	}
}

func TestGatewayGroupRequestRetriesReadsButNotPostsOrRedirects(t *testing.T) {
	gets, posts, redirected := 0, 0, 0
	destination := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { redirected++; w.WriteHeader(http.StatusOK) }))
	defer destination.Close()
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/redirect" {
			http.Redirect(w, r, destination.URL, http.StatusFound)
			return
		}
		if r.Method == http.MethodGet {
			gets++
			if gets == 1 {
				http.Error(w, "transient", http.StatusServiceUnavailable)
				return
			}
			_, _ = w.Write([]byte(`{"status":"ok"}`))
			return
		}
		posts++
		http.Error(w, "uncertain", http.StatusServiceUnavailable)
	}))
	defer api.Close()
	client, err := opnsense.NewClient(opnsense.ClientConfig{BaseURL: api.URL, APIKey: "fixture", APISecret: "fixture", RetryMax: 1, RetryWaitMin: time.Millisecond, RetryWaitMax: time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := groupRequest(context.Background(), client, http.MethodGet, "/read", nil); err != nil || gets != 2 {
		t.Fatalf("configured GET retry lost: %v gets=%d", err, gets)
	}
	if _, err := groupRequest(context.Background(), client, http.MethodPost, "/save", nil); err == nil || posts != 1 {
		t.Fatalf("POST retried: %v posts=%d", err, posts)
	}
	_, _ = groupRequest(context.Background(), client, http.MethodGet, "/redirect", nil)
	if redirected != 0 {
		t.Fatal("GET followed redirect")
	}
}
