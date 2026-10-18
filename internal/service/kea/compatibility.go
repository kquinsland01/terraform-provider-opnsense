// Copyright (c) Matthew Mellor
// SPDX-License-Identifier: MPL-2.0

package kea

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/matthew-on-git/terraform-provider-opnsense/pkg/opnsense"
)

const keaFirmwareStatusEndpoint = "/api/core/firmware/status"

func requireKeaVersion(ctx context.Context, client *opnsense.Client, minimum string) error {
	if err := client.AcquireRead(ctx); err != nil {
		return fmt.Errorf("check Kea compatibility: acquire read slot: %w", err)
	}
	defer client.ReleaseRead()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, client.BaseURL()+keaFirmwareStatusEndpoint, nil)
	if err != nil {
		return fmt.Errorf("check Kea compatibility: %w", err)
	}
	resp, err := client.HTTPClient().Do(req) //nolint:gosec // provider-configured appliance URL
	if err != nil {
		return fmt.Errorf("check Kea compatibility: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("check Kea compatibility: firmware endpoint returned HTTP %d", resp.StatusCode)
	}
	var result struct {
		Product struct {
			ProductVersion string `json:"product_version"`
		} `json:"product"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("check Kea compatibility: decode firmware version: %w", err)
	}
	if result.Product.ProductVersion == "" {
		return fmt.Errorf("check Kea compatibility: firmware status response is missing product.product_version")
	}
	ok, err := versionAtLeast(result.Product.ProductVersion, minimum)
	if err != nil {
		return fmt.Errorf("check Kea compatibility: %w", err)
	}
	if !ok {
		return fmt.Errorf("kea owner requires OPNsense %s or newer; appliance reports %s", minimum, result.Product.ProductVersion)
	}
	return nil
}

func versionAtLeast(actual, minimum string) (bool, error) {
	parse := func(value string) ([3]uint64, error) {
		var parsed [3]uint64
		fields := strings.FieldsFunc(value, func(r rune) bool { return r == '_' || r == '-' })
		if len(fields) == 0 {
			return parsed, fmt.Errorf("malformed OPNsense version %q", value)
		}
		base := fields[0]
		parts := strings.Split(base, ".")
		if len(parts) < 2 || len(parts) > 3 {
			return parsed, fmt.Errorf("malformed OPNsense version %q", value)
		}
		for i, part := range parts {
			if part == "" {
				return parsed, fmt.Errorf("malformed OPNsense version %q", value)
			}
			n, err := strconv.ParseUint(part, 10, 63)
			if err != nil {
				return parsed, fmt.Errorf("malformed OPNsense version %q: %w", value, err)
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
