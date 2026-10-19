// Copyright (c) Matthew Mellor
// SPDX-License-Identifier: MPL-2.0

package unbound

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/matthew-on-git/terraform-provider-opnsense/pkg/opnsense"
)

const unboundParityMinimumVersion = "26.7.7"

var stableUnboundVersion = regexp.MustCompile(`^[0-9]+\.[0-9]+(?:\.[0-9]+)?(?:_[0-9]+)?$`)

func requireUnboundParityVersion(ctx context.Context, client *opnsense.Client) error {
	supported, version, err := unboundParitySupport(ctx, client)
	if err != nil {
		return err
	}
	if !supported {
		return fmt.Errorf("unbound parity fields require stable OPNsense %s or newer; appliance reports %s", unboundParityMinimumVersion, version)
	}
	return nil
}

func unboundCompatibilityOpts(ctx context.Context, client *opnsense.Client, current, legacy opnsense.ReqOpts) (opnsense.ReqOpts, error) {
	supported, _, err := unboundParitySupport(ctx, client)
	if err != nil {
		return opnsense.ReqOpts{}, err
	}
	if supported {
		return current, nil
	}
	return legacy, nil
}

func unboundParitySupport(ctx context.Context, client *opnsense.Client) (bool, string, error) {
	if err := client.AcquireRead(ctx); err != nil {
		return false, "", fmt.Errorf("check Unbound compatibility: acquire read slot: %w", err)
	}
	defer client.ReleaseRead()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, client.BaseURL()+"/api/core/firmware/status", nil)
	if err != nil {
		return false, "", err
	}
	resp, err := client.HTTPClient().Do(req)
	if err != nil {
		return false, "", opnsense.NewServerError("/api/core/firmware/status", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return false, "", fmt.Errorf("check Unbound compatibility: firmware endpoint returned HTTP %d", resp.StatusCode)
	}
	var body struct {
		Product struct {
			ProductName    string `json:"product_name"`
			ProductID      string `json:"product_id"`
			ProductVersion string `json:"product_version"`
		} `json:"product"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return false, "", fmt.Errorf("check Unbound compatibility: decode firmware identity: %w", err)
	}
	identity := strings.TrimSpace(body.Product.ProductName + " " + body.Product.ProductID)
	if identity == "" || !strings.Contains(strings.ToLower(identity), "opnsense") {
		return false, "", fmt.Errorf("unbound parity requires verified OPNsense product identity; appliance reported %q", identity)
	}
	ok, err := unboundVersionAtLeast(body.Product.ProductVersion, unboundParityMinimumVersion)
	if err != nil {
		return false, "", fmt.Errorf("check Unbound compatibility: %w", err)
	}
	return ok, body.Product.ProductVersion, nil
}

func unboundVersionAtLeast(actual, minimum string) (bool, error) {
	parse := func(value string) ([3]uint64, error) {
		var out [3]uint64
		if !stableUnboundVersion.MatchString(value) {
			return out, fmt.Errorf("malformed or non-stable OPNsense version %q", value)
		}
		parts := strings.Split(strings.Split(value, "_")[0], ".")
		for index, part := range parts {
			number, err := strconv.ParseUint(part, 10, 63)
			if err != nil {
				return out, fmt.Errorf("malformed OPNsense version %q", value)
			}
			out[index] = number
		}
		return out, nil
	}
	a, err := parse(actual)
	if err != nil {
		return false, err
	}
	m, err := parse(minimum)
	if err != nil {
		return false, err
	}
	for index := range a {
		if a[index] != m[index] {
			return a[index] > m[index], nil
		}
	}
	return true, nil
}
