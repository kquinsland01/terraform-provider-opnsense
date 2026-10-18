// Copyright (c) Matthew Mellor
// SPDX-License-Identifier: MPL-2.0

package firewall

import (
	"fmt"
	"math"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework/diag"
)

func validateAliasAPIResponse(a *aliasAPIResponse) diag.Diagnostics {
	var result diag.Diagnostics
	if a.UpdateFreq != "" {
		value, err := strconv.ParseFloat(a.UpdateFreq, 64)
		if err != nil || math.IsNaN(value) || math.IsInf(value, 0) || value <= 0 {
			result.AddError("Invalid Alias API Numeric Value", fmt.Sprintf("OPNsense returned non-finite or malformed updatefreq %q.", a.UpdateFreq))
		}
	}
	return result
}

func validateIntegerRange(value, field string, minimum, maximum int64, result *diag.Diagnostics) {
	if value == "" {
		return
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil || parsed < minimum || parsed > maximum {
		result.AddError("Invalid Firewall API Numeric Value", fmt.Sprintf("OPNsense returned malformed %s value %q.", field, value))
	}
}

func validateRequiredIntegerRange(value, field string, minimum, maximum int64, result *diag.Diagnostics) {
	if value == "" {
		result.AddError("Invalid Firewall API Numeric Value", fmt.Sprintf("OPNsense returned empty required %s value.", field))
		return
	}
	validateIntegerRange(value, field, minimum, maximum, result)
}

func validateFilterAPIResponse(a *filterRuleAPIResponse) diag.Diagnostics {
	var result diag.Diagnostics
	validateRequiredIntegerRange(a.Sequence, "sequence", 1, 999999, &result)
	for _, field := range []struct{ name, value string }{
		{"statetimeout", a.StateTimeout},
		{"adaptivestart", a.AdaptiveStart},
		{"adaptiveend", a.AdaptiveEnd},
		{"max", a.MaxStates},
		{"max-src-nodes", a.MaxSourceNodes},
		{"max-src-states", a.MaxSourceStates},
		{"max-src-conn", a.MaxSourceConnections},
		{"max-src-conn-rate", a.MaxNewConnections},
		{"max-src-conn-rates", a.MaxNewConnectionsSeconds},
	} {
		validateIntegerRange(field.value, field.name, 0, math.MaxInt64, &result)
	}
	for _, field := range []struct{ name, value string }{
		{"prio", string(a.MatchPriority)},
		{"set-prio", string(a.SetPriority)},
		{"set-prio-low", string(a.SetPriorityLowDelay)},
	} {
		validateIntegerRange(field.value, field.name, 0, 7, &result)
	}
	return result
}

func validateNATAPIResponse(a *natPortForwardAPIResponse) diag.Diagnostics {
	var result diag.Diagnostics
	validateRequiredIntegerRange(a.Sequence, "sequence", 1, 999999, &result)
	return result
}
