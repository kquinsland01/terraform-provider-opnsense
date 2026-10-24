// Copyright (c) Matthew Mellor
// SPDX-License-Identifier: MPL-2.0

package quagga

import (
	"errors"
	"fmt"
	"strings"

	"github.com/matthew-on-git/terraform-provider-opnsense/pkg/opnsense"
)

// API result/status/validation text is untrusted and can contain credential
// values. Keep owner diagnostics actionable without echoing arbitrary bodies.
func frrDiagnostic(err error) string {
	var auth *opnsense.AuthError
	if errors.As(err, &auth) {
		return auth.Error()
	}
	var saved *opnsense.MutationReconfigureError
	if errors.As(err, &saved) {
		return "The appliance confirmed the save, but FRR activation was not confirmed. Saved identity and pending activation are retained; refresh to retry activation without replaying the mutation."
	}
	var validation *opnsense.ValidationError
	if errors.As(err, &validation) {
		return "The appliance rejected configuration validation. Check address/family, policy membership, required fields and in-use references; no validation body or credential values are retained."
	}
	var server *opnsense.ServerError
	if errors.As(err, &server) {
		return "The appliance API request failed after retries. Verify endpoint connectivity, TLS, service availability and permissions before retrying."
	}
	message := err.Error()
	// These messages are provider-owned compatibility/identity diagnostics, not
	// interpolated result/status/validation bodies from the remote service.
	if strings.HasPrefix(message, "FRR ") || strings.HasPrefix(message, "FRR2 ") || strings.HasPrefix(message, "BGP ") || strings.HasPrefix(message, "legacy os-frr ") {
		return message
	}
	return fmt.Sprintf("The appliance response could not be confirmed (%T). Inspect API/service status and refresh saved state before retrying; no raw response values are retained.", err)
}
