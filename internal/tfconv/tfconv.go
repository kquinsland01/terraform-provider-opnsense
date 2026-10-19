// Copyright (c) Matthew Mellor
// SPDX-License-Identifier: MPL-2.0

// Package tfconv provides shared conversions between Terraform Plugin Framework
// types and the string-based shapes OPNsense expects. These helpers are used by
// hand-written and generated resources alike.
package tfconv

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/matthew-on-git/terraform-provider-opnsense/pkg/opnsense"
)

// SetToCSV joins a Terraform string set into a comma-separated string for the API.
// A null or unknown set yields an empty string.
func SetToCSV(ctx context.Context, s types.Set) string {
	if s.IsNull() || s.IsUnknown() {
		return ""
	}
	var elems []string
	s.ElementsAs(ctx, &elems, false)
	sort.Strings(elems)
	return strings.Join(elems, ",")
}

// SliceToSet converts a string slice into a Terraform string set (never null).
func SliceToSet(items []string) types.Set {
	if len(items) == 0 {
		return types.SetValueMust(types.StringType, []attr.Value{})
	}
	vals := make([]attr.Value, len(items))
	for i, v := range items {
		vals[i] = types.StringValue(v)
	}
	return types.SetValueMust(types.StringType, vals)
}

// IntOrEmpty renders an int64 as a string, returning "" for zero so optional
// integer fields are not forced to "0" on the API side.
func IntOrEmpty(n int64) string {
	if n == 0 {
		return ""
	}
	return opnsense.Int64ToString(n)
}

// IntOrZero parses an OPNsense numeric string, returning 0 when empty or
// unparseable.
func IntOrZero(s string) int64 {
	if s == "" {
		return 0
	}
	if v, err := opnsense.StringToInt64(s); err == nil {
		return v
	}
	return 0
}

// ParseIntInRange decodes an API integer string without hiding malformed or
// schema-invalid remote values.
func ParseIntInRange(field, value string, minValue, maxValue int64) (int64, error) {
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid %s value %q: expected an integer", field, value)
	}
	if parsed < minValue || parsed > maxValue {
		return 0, fmt.Errorf("invalid %s value %q: expected a value between %d and %d", field, value, minValue, maxValue)
	}
	return parsed, nil
}
