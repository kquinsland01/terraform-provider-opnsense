// Copyright (c) Matthew Mellor
// SPDX-License-Identifier: MPL-2.0

package openvpn

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

const openVPNMinimumVersion = "26.7.6"

var stableOpenVPNVersion = regexp.MustCompile(`^[0-9]+\.[0-9]+(?:\.[0-9]+)?(?:_[0-9]+)?$`)

func requireOpenVPNParityVersion(ctx context.Context, client *opnsense.Client) error {
	supported, version, err := openVPNParitySupport(ctx, client)
	if err != nil {
		return err
	}
	if !supported {
		return fmt.Errorf("OpenVPN parity requires stable OPNsense %s or newer; appliance reports %s", openVPNMinimumVersion, version)
	}
	return nil
}

func openVPNParitySupport(ctx context.Context, client *opnsense.Client) (bool, string, error) {
	if err := client.AcquireRead(ctx); err != nil {
		return false, "", fmt.Errorf("check OpenVPN compatibility: acquire read slot: %w", err)
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
		return false, "", fmt.Errorf("check OpenVPN compatibility: firmware endpoint returned HTTP %d", resp.StatusCode)
	}
	var body struct {
		Product struct {
			ProductName    string `json:"product_name"`
			ProductID      string `json:"product_id"`
			ProductVersion string `json:"product_version"`
		} `json:"product"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return false, "", fmt.Errorf("check OpenVPN compatibility: decode firmware identity: %w", err)
	}
	identity := strings.TrimSpace(body.Product.ProductName + " " + body.Product.ProductID)
	if identity == "" || !strings.Contains(strings.ToLower(identity), "opnsense") {
		return false, "", fmt.Errorf("OpenVPN parity requires verified OPNsense product identity; appliance reported %q", strings.TrimSpace(identity))
	}
	ok, err := stableVersionAtLeast(body.Product.ProductVersion, openVPNMinimumVersion)
	if err != nil {
		return false, "", fmt.Errorf("check OpenVPN compatibility: %w", err)
	}
	return ok, body.Product.ProductVersion, nil
}

func stableVersionAtLeast(actual, minimum string) (bool, error) {
	parse := func(value string) ([3]uint64, error) {
		var out [3]uint64
		if !stableOpenVPNVersion.MatchString(value) {
			return out, fmt.Errorf("malformed or non-stable OPNsense version %q", value)
		}
		base := strings.Split(value, "_")[0]
		parts := strings.Split(base, ".")
		for i, p := range parts {
			n, err := strconv.ParseUint(p, 10, 63)
			if err != nil {
				return out, fmt.Errorf("malformed OPNsense version %q", value)
			}
			out[i] = n
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
	for i := range a {
		if a[i] != m[i] {
			return a[i] > m[i], nil
		}
	}
	return true, nil
}

func instanceUsesParity(m InstanceResourceModel) bool {
	stringsToCheck := []typesStringLike{m.PortShare, m.ServerIPv6, m.BridgeGateway, m.BridgePool, m.CRL, m.CertDepth, m.DataCiphersFallback, m.LocalGroup, m.StrictUserCN, m.Username, m.Password, m.AuthGenTokenSecret, m.CARPDependOn, m.HTTPProxy, m.VerifyX509Name}
	for _, v := range stringsToCheck {
		if stringKnown(v) {
			return true
		}
	}
	bools := []interface {
		IsNull() bool
		IsUnknown() bool
	}{m.NoPool, m.RemoteCertTLS, m.UseOCSP, m.UsernameAsCommonName, m.ProvisionExclusive, m.RegisterDNS, m.MSSFix, m.CompressMigrate, m.IfConfigPoolPersist}
	for _, v := range bools {
		if !v.IsNull() && !v.IsUnknown() {
			return true
		}
	}
	ints := []interface {
		IsNull() bool
		IsUnknown() bool
	}{m.PushInactive, m.RenegSec, m.AuthGenToken, m.AuthGenTokenRenewal, m.RouteMetric, m.TunMTU, m.Fragment}
	for _, v := range ints {
		if !v.IsNull() && !v.IsUnknown() {
			return true
		}
	}
	sets := []interface {
		IsNull() bool
		IsUnknown() bool
	}{m.Route, m.PushExcludedRoutes, m.AuthMode, m.VariousFlags, m.VariousPushFlags, m.DNSDomain, m.DNSDomainSearch, m.NTPServers}
	for _, v := range sets {
		if !v.IsNull() && !v.IsUnknown() {
			return true
		}
	}
	return !m.DataCiphersOrdered.IsNull() && !m.DataCiphersOrdered.IsUnknown()
}

func clientOverwriteUsesParity(m ClientOverwriteResourceModel) bool {
	for _, v := range []typesStringLike{m.TunnelNetworkV6, m.RouteGateway} {
		if stringKnown(v) {
			return true
		}
	}
	for _, v := range []interface {
		IsNull() bool
		IsUnknown() bool
	}{m.RedirectGateway, m.DNSDomain, m.DNSDomainSearch, m.NTPServers, m.WINSServers, m.RegisterDNS} {
		if !v.IsNull() && !v.IsUnknown() {
			return true
		}
	}
	return false
}

type typesStringLike interface {
	IsNull() bool
	IsUnknown() bool
}

func stringKnown(v typesStringLike) bool { return !v.IsNull() && !v.IsUnknown() }
